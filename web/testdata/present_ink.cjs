// 발표(present.js)·판서(ink.js·inkshape.js) 시험 — present_test.go 가 web/ 폴더를 넘긴다. 가짜 DOM 위에서 돈다.
// 판서 모형(획 추가·되돌리기·상한·탭 수명)·기하·바탕 밝기, 발표의 넘김·차례로 나타내기, 호스트 훅 계약(present.js 머리 주석)을 본다.
// 실제 창 안 동작(CSS·층·혼합·전체 화면)은 호스트 앱의 시험이 본다.
"use strict";
const fs = require("fs");
const vm = require("vm");
const path = require("path");
const [webDir] = process.argv.slice(2);
let cases = 0, failed = 0;
function ok(name, cond, detail) {
  cases++;
  if (!cond) { failed++; console.log("실패 " + name + " — " + JSON.stringify(detail === undefined ? null : detail)); } else console.log("통과 " + name);
}

// ---- 가짜 DOM: 선택자는 tag·.class·[attr] 와 쉼표 목록만
function matchOne(e, sel) {
  const m = /^([a-z0-9]*)((?:\.[\w-]+)*)((?:\[[\w-]+\])*)$/i.exec(sel.trim());
  if (!m) return false;
  if (m[1] && e.tag !== m[1].toLowerCase()) return false;
  for (const c of m[2].split(".").filter(Boolean)) if (!e.classList.contains(c)) return false;
  for (const a of m[3].replace(/[\[\]]/g, " ").trim().split(/\s+/).filter(Boolean)) if (!(a in e.attrs) && !(a.startsWith("data-") && a.slice(5) in e.dataset)) return false;
  return true;
}
class El {
  constructor(tag) {
    this.tag = tag; this.kids = []; this.parentNode = null; this.attrs = {}; this.dataset = {}; this.on = {}; this._cls = new Set(); this._text = "";
    this.style = { setProperty(k, v) { this[k] = v; }, getPropertyValue(k) { return this[k] || ""; } };
    const c = this._cls;
    this.classList = { add: (...a) => a.forEach(x => c.add(x)), remove: (...a) => a.forEach(x => c.delete(x)), contains: x => c.has(x), toggle: (x, f) => { const v = f === undefined ? !c.has(x) : !!f; if (v) c.add(x); else c.delete(x); return v; } };
    this.clientWidth = 1280; this.clientHeight = 720; this.offsetWidth = 300; this.offsetHeight = 200; this.clientLeft = 0; this.clientTop = 0; this.title = ""; this.disabled = false;
  }
  get tagName() { return this.tag.toUpperCase(); }
  get className() { return [...this._cls].join(" "); }
  set className(v) { this._cls.clear(); for (const x of String(v).split(/\s+/).filter(Boolean)) this._cls.add(x); }
  get children() { return this.kids; }
  get firstElementChild() { return this.kids[0] || null; }
  get parentElement() { return this.parentNode; }
  get textContent() { return this._text + this.kids.map(k => k.textContent).join(""); }
  set textContent(v) { for (const k of this.kids) k.parentNode = null; this.kids = []; this._text = String(v); }
  set innerHTML(v) { this.textContent = v; }
  appendChild(k) { if (k.parentNode) k.remove(); k.parentNode = this; this.kids.push(k); return k; }
  remove() { const p = this.parentNode; if (p) { p.kids.splice(p.kids.indexOf(this), 1); this.parentNode = null; } }
  contains(x) { for (let e = x; e; e = e.parentNode) if (e === this) return true; return false; }
  matches(s) { return s.split(",").some(x => matchOne(this, x)); }
  closest(s) { for (let e = this; e; e = e.parentNode) if (e.matches && e.matches(s)) return e; return null; }
  querySelectorAll(s) { const out = []; const walk = e => { for (const k of e.kids) { if (k.matches(s)) out.push(k); walk(k); } }; walk(this); return out; }
  querySelector(s) { return this.querySelectorAll(s)[0] || null; }
  setAttribute(k, v) { this.attrs[k] = String(v); if (k === "class") this.className = v; }
  getAttribute(k) { return this.attrs[k]; }
  removeAttribute(k) { delete this.attrs[k]; }
  addEventListener(t, f) { (this.on[t] = this.on[t] || []).push(f); }
  removeEventListener(t, f) { this.on[t] = (this.on[t] || []).filter(x => x !== f); }
  getBoundingClientRect() { return { left: 0, top: 0, right: 1280, bottom: 720, width: 1280, height: 720 }; }
  setPointerCapture(id) { if (!El.capture) throw new Error("합성 입력"); El.capture.add(id); }
  releasePointerCapture(id) { if (!El.capture) throw new Error("합성 입력"); El.capture.delete(id); }
  blur() {}
  getContext() { if (this.tag !== "canvas") return null; this.ctx = this.ctx || ctx2d(); return this.ctx; }
  cloneNode(deep) {
    const c = new El(this.tag); c.className = this.className; c.attrs = { ...this.attrs }; c.dataset = { ...this.dataset }; c._text = this._text;
    if (deep) for (const k of this.kids) c.appendChild(k.cloneNode(true));
    return c;
  }
}
// ctx2d 는 그리기 명령을 적는다 — paints 는 지운 뒤 그린 경로들이다({op, d, color, width}).
function ctx2d() {
  const t = { paints: [], lineCap: "", lineJoin: "" };
  const draw = op => p => t.paints.push({ op, d: p && p.d, color: op === "fill" ? t.fillStyle : t.strokeStyle, width: op === "fill" ? null : t.lineWidth, cap: t.lineCap, join: t.lineJoin });
  return new Proxy(t, { get(o, k) { if (k in o) return o[k]; if (k === "stroke" || k === "fill") return draw(k); if (k === "clearRect") return () => { o.paints = []; }; return () => {}; }, set(o, k, v) { o[k] = v; return true; } });
}

function world() {
  const doc = new El("html"), win = new El("window"), body = new El("body");
  doc.body = body;
  doc.createElement = t => new El(t.toLowerCase());
  doc.createElementNS = (ns, t) => new El(t);
  const raf = [];
  Object.assign(win, { devicePixelRatio: 2, navigator: { platform: "MacIntel" }, matchMedia: () => ({ matches: false }), requestAnimationFrame: f => { const g = () => f(); g.id = ++world.seq; raf.push(g); return g.id; }, cancelAnimationFrame: id => { const i = raf.findIndex(g => g.id === id); if (i >= 0) raf.splice(i, 1); },
    ItdaSlides: { SIZE: { "16:9": [1280, 720], "4:3": [960, 720] } }, getComputedStyle: () => ({ backgroundColor: "rgb(255, 255, 255)", width: "0px", height: "0px" }) });
  win.Path2D = class { constructor(d) { this.d = d; } };
  const ctx = vm.createContext({ window: win, document: doc, performance: { now: () => world.clock }, setTimeout: (f, ms) => { const id = ++world.seq; world.timers.set(id, f); return id; }, clearTimeout: id => world.timers.delete(id), setInterval: () => 0, clearInterval() {}, requestAnimationFrame: win.requestAnimationFrame, console, Math, parseFloat, Number, String, Map, WeakMap, Set, Infinity });
  for (const f of ["inkshape.js", "ink.js", "present.js"]) vm.runInContext(fs.readFileSync(path.join(webDir, f), "utf8"), ctx);
  return { win, doc, body, H: win.ItdaPresent, I: win.ItdaInk, P: win.ItdaPresent.state, raf };
}
world.clock = 0;
world.seq = 0;
world.timers = new Map();
const runTimers = () => { const t = [...world.timers.values()]; world.timers.clear(); t.forEach(f => f()); };

function slide(body, steps) {
  const p = new El("div"); p.className = "page"; p.dataset.kind = "content"; if (steps) p.dataset.steps = String(steps);
  const b = p.appendChild(new El("div")); b.className = "page-body";
  for (const x of body) b.appendChild(x);
  return p;
}
function img() { const i = new El("img"); return i; }
const rootOf = w => w.body.kids.find(k => k.classList.contains("present"));
// open 은 슬라이드 셋(둘째는 이미지)짜리 발표를 연다.
function open(w, o) {
  const pages = [slide([]), slide([img()]), slide([])];
  w.H.start(Object.assign({ pages, notes: [], index: 0, ratio: "16:9", onExit() {} }, o));
  const root = rootOf(w);
  return { root, deck: root.querySelector(".pr-deck"), slides: w.P.slides };
}
function fire(target, type, ev) {
  const e = Object.assign({ type, target, button: 0, buttons: 0, clientX: 0, clientY: 0, pointerId: 1, pointerType: "mouse", pressure: 0.5, defaultPrevented: false,
    preventDefault() { this.defaultPrevented = true; }, stopPropagation() { this.stopped = true; } }, ev);
  for (let n = target; n && !e.stopped; n = n.parentNode) for (const f of [...(n.on[type] || [])]) f(e);
  return e;
}
// click 은 뒤따르는 click 이 창 붙잡기 단계에서 먹히는가를 돌려준다(먹히면 문서의 링크 처리까지 가지 않는다).
function clickEaten(w, target) { const e = { type: "click", target, button: 0, preventDefault() { this.dp = true; }, stopPropagation() {} }; for (const f of [...(w.win.on.click || [])]) f(e); return !!e.dp; }
function tap(w, target, ev) {
  const a = Object.assign({ clientX: 100, clientY: 100, button: 0 }, ev);
  fire(target, "pointerdown", { ...a, buttons: 1 });
  fire(target, "mousedown", { ...a, buttons: 1 });
  fire(target, "pointerup", a);
  fire(target, "mouseup", a);
  return clickEaten(w, target);
}
function drag(w, target, pts, ev) {
  const a = Object.assign({ button: 0 }, ev), [x0, y0] = pts[0];
  fire(target, "pointerdown", { ...a, buttons: 1, clientX: x0, clientY: y0 });
  fire(target, "mousedown", { ...a, buttons: 1, clientX: x0, clientY: y0 });
  for (const [x, y] of pts.slice(1)) fire(target, "pointermove", { ...a, buttons: 1, clientX: x, clientY: y });
  const [xe, ye] = pts[pts.length - 1];
  fire(target, "pointerup", { ...a, clientX: xe, clientY: ye });
  fire(target, "mouseup", { ...a, clientX: xe, clientY: ye });
  return clickEaten(w, target);
}
function key(w, k, code, mods) {
  const e = Object.assign({ key: k, code: code || "", metaKey: false, ctrlKey: false, altKey: false, shiftKey: false, isComposing: false, preventDefault() {}, stopPropagation() {} }, mods);
  for (const f of [...(w.doc.on.keydown || [])]) f(e);
}
const line = (x0, y0, n) => Array.from({ length: n }, (_, i) => [x0 + i * 10, y0]);
const page = (w, i) => w.I.pageOf(w.H.ink().book, i).list;
const toolOf = w => w.H.ink().tool();

// ---- 1. 판서 모형
{
  const w = world(), I = w.I, b = I.newBook();
  const st = (n, tool) => ({ tool: tool || "pen", color: 0, width: 1, pts: Array.from({ length: n * 2 }, (_, i) => i), press: null, box: [0, 0, n, n] });
  const a = st(10), c = st(20);
  I.commit(b, 0, [a]);
  I.commit(b, 0, [a, c]);
  ok("모델: 추가하면 획·점 합계가 는다", b.strokes === 2 && b.points === 30, [b.strokes, b.points]);
  ok("모델: 되돌리기 → 앞 목록, 합계도", I.undo(b, 0) === "ok" && I.pageOf(b, 0).list.length === 1 && b.points === 10, b.points);
  ok("모델: 다시 실행", I.redo(b, 0) === "ok" && b.strokes === 2, b.strokes);
  I.commit(b, 0, []);
  ok("모델: 전부 지우기를 되돌리면 전부 돌아온다", b.strokes === 0 && I.undo(b, 0) === "ok" && b.strokes === 2, b.strokes);
  ok("모델: 슬라이드마다 따로 — 다른 장의 되돌리기는 이 장을 건드리지 않는다", I.undo(b, 5) === "" && b.strokes === 2);
  I.commit(b, 1, [st(1)]);
  ok("모델: 새 획은 다시 실행을 비운다", (I.undo(b, 1), I.commit(b, 1, [st(1)]), I.redo(b, 1) === ""));
  const lim = I.newBook();
  ok("모델: 상한 점", I.room(lim, 1, I.LIMIT.points) && !I.room(lim, 1, I.LIMIT.points + 1));
  ok("모델: 상한 획", I.room(lim, I.LIMIT.strokes, 0) && !I.room(lim, I.LIMIT.strokes + 1, 0));
  I.commit(lim, 0, [st(I.LIMIT.points)]);
  I.commit(lim, 0, []);
  I.commit(lim, 1, [st(I.LIMIT.points)]);
  ok("모델: 되돌려도 상한을 넘지 않는다", I.undo(lim, 0) === "limit" && lim.points === I.LIMIT.points, lim.points);
  const k1 = I.bookFor("t1"), k2 = I.bookFor("t2");
  ok("모델: 탭마다 판서가 따로이고 같은 탭이면 같다", k1 !== k2 && I.bookFor("t1") === k1);
  I.drop("t1");
  ok("모델: 닫힌 탭의 판서는 놓고 다른 탭은 그대로다", I.bookFor("t1") !== k1 && I.bookFor("t2") === k2);
  ok("모델: 키가 없으면 그 발표만의 판서다", I.bookFor(undefined) !== I.bookFor(undefined));
}
{
  const { I } = world();
  const s = { tool: "pen", color: 0, width: 0, pts: [0, 0, 10, 0, 20, 10], press: null, box: [0, 0, 20, 10] };
  ok("기하: 경로는 점 사이 가운데를 잇는다", I.pathOf(s) === "M0 0Q10 0 15 5L20 10", I.pathOf(s));
  ok("기하: 점 하나는 채운 원이다", I.filled({ ...s, pts: [3, 4] }) && /Z$/.test(I.pathOf({ ...s, pts: [3, 4] })) && !I.filled(s), I.pathOf({ ...s, pts: [3, 4] }));
  ok("기하: 지우개는 굵기까지 닿는다", I.hits(s, 10, 3, 1) && !I.hits(s, 10, 30, 1) && !I.hits(s, 10, 3, 0.1));
  const p = I.pathOf({ ...s, press: [0.2, 0.5, 0.9] });
  ok("기하: 필압 획은 채운 조각이고 굵기가 필압을 따른다", /^M/.test(p) && (p.match(/Z/g) || []).length === 5, p);
  const thin = { ...s, press: [0.1, 0.1, 0.1] }, fat = { ...s, press: [1, 1, 1] };
  ok("기하: 필압이 크면 더 굵다", I.pathOf(fat).length && I.pathOf(fat) !== I.pathOf(thin));
}
{
  const { I } = world();
  const dark = (bg, deco) => { const pg = { querySelector: () => (deco ? {} : null) }; const w = world(); w.win.getComputedStyle = (e, part) => part ? (part === "::before" ? deco : { backgroundColor: "rgba(0, 0, 0, 0)", width: "0px", height: "0px" }) : { backgroundColor: bg }; return w.I.slideDark(pg, 1280, 720); };
  ok("바탕: 흰 쪽은 밝다", !dark("rgb(255, 255, 255)"));
  ok("바탕: 어두운 쪽 배경", dark("rgb(5, 20, 31)"));
  ok("바탕: 전체를 덮는 장식 면이 쪽 배경을 이긴다(kia 제목 장)", dark("rgb(255, 255, 255)", { backgroundColor: "rgb(5, 20, 31)", width: "1280px", height: "720px" }));
  ok("바탕: 작은 장식 띠는 바탕이 아니다", !dark("rgb(255, 255, 255)", { backgroundColor: "rgb(5, 20, 31)", width: "61px", height: "7px" }));
  ok("바탕: 어두운 쪽 위의 밝은 전체 면", !dark("rgb(5, 20, 31)", { backgroundColor: "rgb(250, 250, 250)", width: "1280px", height: "720px" }));
  ok("바탕: color() 표기", dark("color(srgb 0.02 0.08 0.12)"));
  ok("바탕: 읽지 못하면 밝은 쪽(곱하기)", !dark("oklch(0 0 0)"));
  ok("바탕: 색 읽기", I.parseColor("rgba(10, 20, 30, 0.5)").a === 0.5 && I.parseColor("transparent") === null);
}


// ---- 2. 발표: 넘김·차례로 나타내기
const stepped = n => { const e = new El("p"); e.dataset.step = String(n); return e; };
const shown = P => P.slides[1].querySelectorAll("[data-step]").map(e => (e.classList.contains("pr-pending") ? "0" : "1")).join("");
{
  const w = world(), { P, H } = w, calls = [];
  w.win.ItdaPresentHost = { fullscreen: on => calls.push(on) };
  H.start({ pages: [slide([]), slide([stepped(1), stepped(2)], 2), slide([])], notes: [], index: 0, ratio: "16:9", onExit() {} });
  ok("열기: 첫 슬라이드에서 시작하고 호스트에 전체 화면을 청한다", P.on && P.idx === 0 && P.step === 0 && calls.join() === "true" && P.fsCalls === 1, { calls, fs: P.fsCalls });
  ok("열기: 덮개는 body 의 자식이고 body 에 presenting 이 붙는다", !!rootOf(w) && w.body.classList.contains("presenting"));
  key(w, "ArrowRight", "ArrowRight");
  ok("넘김: 다음 슬라이드는 단계 0 — 차례 항목이 모두 숨어 있다", P.idx === 1 && P.step === 0 && shown(P) === "00", { idx: P.idx, step: P.step, s: shown(P) });
  key(w, "ArrowRight", "ArrowRight");
  ok("차례: 다음은 슬라이드가 아니라 첫 항목", P.idx === 1 && P.step === 1 && shown(P) === "10", shown(P));
  key(w, " ", "Space");
  ok("차례: Space 도 다음 항목", P.idx === 1 && P.step === 2 && shown(P) === "11", shown(P));
  key(w, "ArrowRight", "ArrowRight");
  ok("넘김: 항목을 다 보인 뒤에야 다음 슬라이드", P.idx === 2 && P.step === 0);
  key(w, "ArrowLeft", "ArrowLeft");
  ok("이전: 앞 슬라이드는 다 나타난 모습", P.idx === 1 && P.step === 2 && shown(P) === "11", { idx: P.idx, step: P.step, s: shown(P) });
  key(w, "ArrowLeft", "ArrowLeft");
  ok("이전: 같은 슬라이드 안에서는 항목 하나씩 거둔다", P.idx === 1 && P.step === 1 && shown(P) === "10", shown(P));
  H.prev(); H.prev();
  ok("이전: 단계 0 을 지나 앞 슬라이드로", P.idx === 0 && P.step === 0);
  H.prev();
  ok("이전: 첫 슬라이드에서는 그대로", P.idx === 0);
  key(w, "End", "End");
  ok("End: 마지막 슬라이드", P.idx === 2);
  H.next();
  ok("다음: 마지막 슬라이드에서는 그대로", P.idx === 2 && P.on);
  key(w, "Home", "Home");
  ok("Home: 첫 슬라이드", P.idx === 0);
  H.jump(1);
  ok("jump: 뒤로 가면 단계 0 부터", P.idx === 1 && P.step === 0 && shown(P) === "00");
  H.jump(2); H.jump(1);
  ok("jump: 앞으로 돌아오면 다 나타난 모습", P.idx === 1 && P.step === 2 && shown(P) === "11");
  key(w, "Escape", "Escape");
  ok("끝: Esc 로 닫고 호스트에 전체 화면 끄기를 청한다", !P.on && calls.join() === "true,false" && !rootOf(w) && !w.body.classList.contains("presenting"), calls);
}

// ---- 3. 호스트 훅 계약
{
  const w = world(), { P, H } = w;
  open(w);
  ok("훅 없음: 전체 화면 없이 열리고 부른 횟수는 센다", P.on && P.fsCalls === 1);
  key(w, "ArrowRight", "ArrowRight");
  tap(w, P.slides[1].querySelector("img"));
  ok("크게 보기 부품 없음: 그림 클릭은 다음", P.idx === 2 && !P.lightbox, { idx: P.idx, lb: P.lightbox });
  key(w, "Escape", "Escape");
  ok("훅 없음: 닫힌다", !P.on);
}
{
  const w = world(), { P } = w;
  w.win.ItdaPresentHost = { fullscreen() { throw new Error("창 없음"); } };
  open(w);
  ok("훅 예외: 삼키고 발표는 열린다", P.on && P.idx === 0);
  key(w, "Escape", "Escape");
  ok("훅 예외: 닫을 때도 삼킨다", !P.on);
}
{
  const w = world(), { P, H } = w, opened = [];
  w.win.ItdaPresentHost = { ownLink: h => h.startsWith("#own:") };
  const own = new El("a"); own.setAttribute("href", "#own:topic");
  const ext = new El("a"); ext.setAttribute("href", "https://example.com/x");
  const s0 = slide([own]), s1 = slide([ext]);
  H.start({ pages: [s0, s1, slide([])], notes: [], index: 0, ratio: "16:9", onExit() {}, onLink: h => opened.push(h) });
  tap(w, P.slides[0].querySelector("a"));
  ok("ownLink: 호스트 전용 앵커는 링크로 다루지 않는다 — 누르기는 다음", P.idx === 1 && opened.length === 0, { idx: P.idx, opened });
  const eaten = tap(w, P.slides[1].querySelector("a"));
  ok("링크: 외부 링크 클릭은 onLink 로 열고 넘기지 않는다(크게 보기 부품이 없어도 click 을 먹는다)", P.idx === 1 && opened.join() === "https://example.com/x" && eaten, { idx: P.idx, opened, eaten });
  H.stop();
}
{
  const w = world(), { P, H } = w, armed = [];
  w.win.ItdaCodeCopy = { arm: el => armed.push(el) };
  H.start({ pages: [slide([]), slide([])], notes: ["<p>노트</p>", ""], index: 0, ratio: "16:9", onExit() {} });
  H.next();
  ok("ItdaCodeCopy: 있으면 슬라이드마다 노트 칸에 붙인다", armed.length === 2 && armed[0] === armed[1], armed.length);
  H.stop();
}

console.log("PRESENT INK DONE cases=" + cases + " failed=" + failed);
