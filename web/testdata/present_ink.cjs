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
  // ask.js 는 묻기 단추의 위임 처리기(handler)를 청중 덱에서 시험하려고 싣는다(8절) — 켜기(set) 전에는 아무것도 하지 않는다
  for (const f of ["inkshape.js", "ink.js", "present.js", "ask.js"]) vm.runInContext(fs.readFileSync(path.join(webDir, f), "utf8"), ctx);
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
// key 는 키 하나를 document 붙잡기 단계에 보낸다. 돌려주는 e 의 dp·sp 는 preventDefault·stopPropagation 을 불렀는가다.
function key(w, k, code, mods) {
  const e = Object.assign({ key: k, code: code || "", metaKey: false, ctrlKey: false, altKey: false, shiftKey: false, isComposing: false, preventDefault() { this.dp = true; }, stopPropagation() { this.sp = true; } }, mods);
  for (const f of [...(w.doc.on.keydown || [])]) f(e);
  return e;
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

// ---- 4. 호스트 계약: 키 양보(ownsKey)·겹침 층([data-pr-overlay])
// 크게 보기 부품 흉내 — 열고 닫을 때 present.js 의 onOpen·onClose 를 부른다(가림 클래스 전이를 보려고)
function fakeLightbox(w) {
  w.win.ItdaLightbox = { SEL: "img", create(root, o) {
    const el = root.appendChild(new El("div")); el.className = "pr-lightbox";
    let open = false;
    return { el, open() { open = true; o.onOpen("img", 1); }, close() { if (open) { open = false; o.onClose(); } }, onScrollbar: () => false, relayout() {} };
  } };
}
const overlayOf = w => { const o = w.body.appendChild(new El("div")); o.dataset.prOverlay = ""; o.attrs["data-pr-overlay"] = ""; return o; };
const hidden = w => w.body.classList.contains("pr-overlay-hidden");
{
  const w = world(), { P, H } = w;
  let owns = true;
  w.win.ItdaPresentHost = { ownsKey: () => owns };
  const ov = overlayOf(w);
  open(w);
  let e = key(w, "ArrowRight", "ArrowRight");
  ok("ownsKey 참: 넘기지 않고 preventDefault·stopPropagation 도 하지 않는다", P.idx === 0 && !e.dp && !e.sp, { idx: P.idx, dp: e.dp, sp: e.sp });
  key(w, "l", "KeyL");
  ok("ownsKey 참: 판서 키(L 레이저)도 발표가 받지 않는다", !toolOf(w), toolOf(w));
  key(w, "Escape", "Escape");
  ok("ownsKey 참: Esc 로도 끝나지 않는다", P.on);
  owns = false;
  e = key(w, "ArrowRight", "ArrowRight");
  ok("ownsKey 거짓: 지금처럼 넘기고 키를 삼킨다", P.idx === 1 && e.dp && e.sp, { idx: P.idx, dp: e.dp, sp: e.sp });
  key(w, "l", "KeyL");
  ok("ownsKey 거짓: 판서 키는 지금처럼", toolOf(w) === "laser", toolOf(w));
  key(w, "l", "KeyL");
  // 겹침 층의 click 은 지난 누르기의 깃발(lbEat)로 먹지 않는다 — 펜으로 그리고 click 없이 놓은 뒤(밖으로 끌어 놓음)
  key(w, "p", "KeyP");
  const s1 = P.slides[1];
  fire(s1, "pointerdown", { clientX: 100, clientY: 100, buttons: 1 });
  fire(s1, "mousedown", { clientX: 100, clientY: 100, buttons: 1 });
  fire(s1, "pointerup", { clientX: 130, clientY: 100 });
  ok("겹침 층: 그 안의 click 은 발표가 먹지 않는다(지난 누르기의 깃발이 서 있어도)", !clickEaten(w, ov));
  ok("겹침 층: 깃발은 층 밖 다음 click 의 몫으로 남는다", clickEaten(w, s1));
  // #present 안에 놓인 층도 누르기·판서 자리가 아니다
  const inner = rootOf(w).appendChild(new El("div")); inner.attrs["data-pr-overlay"] = "";
  const before = page(w, 1).length;
  drag(w, inner, line(100, 100, 5));
  ok("겹침 층(#present 안): 펜이 켜져도 그리지 않는다(ink BLOCKED)", page(w, 1).length === before, page(w, 1).length);
  key(w, "p", "KeyP");
  const at = P.idx;
  tap(w, inner);
  ok("겹침 층(#present 안): 누르기는 넘김이 아니다(onMouse)", P.idx === at, P.idx);
  ok("겹침 층(#present 안): 누르기는 커서 숨김 타이머를 다시 걸지 않는다", (rootOf(w).classList.add("pr-idle"), tap(w, inner), rootOf(w).classList.contains("pr-idle")));
  inner.remove();
  // 가림 전이: 검은 화면·한눈에 보기·크게 보기
  ok("가림: 평소에는 가림 클래스가 없다", !hidden(w));
  owns = true; // 가린 동안에도 ownsKey 를 먼저 묻는다 — 양보할지는 호스트 몫이다
  H.setBlack(true);
  ok("가림: 검은 화면 동안 겹침 층을 가린다", hidden(w));
  e = key(w, "b", "KeyB");
  const d = key(w, "5", "Digit5");
  ok("가림 + ownsKey 참: 발표는 아무것도 하지 않는다 — 검은 화면 그대로, dp·sp 없음, 번호 입력 없음",
    P.black && hidden(w) && !e.dp && !e.sp && !d.dp && !rootOf(w).classList.contains("pr-goto-on"), { black: P.black, dp: e.dp, sp: e.sp, goto: rootOf(w).classList.contains("pr-goto-on") });
  owns = false;
  e = key(w, "b", "KeyB");
  ok("가림 + ownsKey 거짓: 지금처럼 B 로 검은 화면이 걷히고 층이 돌아온다", !P.black && !hidden(w) && e.dp && e.sp, { black: P.black, hidden: hidden(w) });
  H.toggleGrid(true);
  ok("가림: 한눈에 보기 동안 가린다", hidden(w));
  owns = true;
  e = key(w, "Escape", "Escape");
  ok("가림 + ownsKey 참: 한눈에 보기도 Esc 를 받지 않는다", P.grid && hidden(w) && !e.dp, { grid: P.grid });
  owns = false;
  key(w, "Escape", "Escape");
  ok("가림 + ownsKey 거짓: 한눈에 보기를 Esc 로 닫으면 되돌린다", !P.grid && !hidden(w) && P.on, { grid: P.grid, on: P.on });
  // 권하는 호스트: 훅 안에서 가림 클래스를 읽어 가린 동안은 양보하지 않는다
  w.win.ItdaPresentHost = { ownsKey: () => !w.body.classList.contains("pr-overlay-hidden") };
  H.setBlack(true);
  key(w, "b", "KeyB");
  ok("가림 클래스를 읽는 호스트: 가린 동안의 B 는 발표가 받아 검은 화면을 걷는다", !P.black && !hidden(w));
  e = key(w, "ArrowRight", "ArrowRight");
  ok("가림 클래스를 읽는 호스트: 평소 키는 호스트가 가져간다", !e.dp && !e.sp);
  H.setBlack(true);
  H.stop();
  ok("끝: 겹침 층 요소는 남고 가림·presenting 클래스는 떨어진다", w.body.kids.includes(ov) && !hidden(w) && !w.body.classList.contains("presenting"));
}
{
  const w = world(), { P, H } = w;
  fakeLightbox(w);
  let owns = true;
  w.win.ItdaPresentHost = { ownsKey: () => owns };
  open(w, { index: 1 });
  tap(w, P.slides[1].querySelector("img"));
  ok("가림: 크게 보기 동안 가린다", P.lightbox === "img" && hidden(w), P.lightbox);
  let e = key(w, "Escape", "Escape");
  ok("가림 + ownsKey 참: 크게 보기도 Esc 를 받지 않는다", P.lightbox === "img" && hidden(w) && !e.dp && !e.sp, P.lightbox);
  owns = false;
  key(w, "Escape", "Escape");
  ok("가림 + ownsKey 거짓: 크게 보기를 Esc 로 닫으면 되돌린다", !P.lightbox && !hidden(w) && P.on, { lb: P.lightbox, on: P.on });
  H.stop();
}
{
  // 보드 발표: 넘김·판서·한눈에 보기가 없다 — 키 양보·검은 화면·크게 보기 가림·끝 뒤 층 유지를 본다
  const w = world(), { P, H } = w;
  fakeLightbox(w);
  let owns = true;
  w.win.ItdaPresentHost = { ownsKey: () => owns };
  const ov = overlayOf(w);
  const board = new El("div"); board.className = "page"; const bb = board.appendChild(new El("div")); bb.className = "page-body"; bb.appendChild(img());
  H.startBoard({ page: board, ratio: "16:9", onExit() {} });
  let e = key(w, "Escape", "Escape");
  ok("보드 ownsKey 참: Esc 로 끝나지 않고 키를 삼키지 않는다", P.on && P.board && !e.dp && !e.sp, { on: P.on, dp: e.dp });
  key(w, "b", "KeyB");
  ok("보드 ownsKey 참: B 도 발표가 받지 않는다", !P.black && !hidden(w));
  owns = false;
  key(w, "b", "KeyB");
  ok("보드 가림: 검은 화면 동안 가린다", P.black && hidden(w));
  owns = true;
  e = key(w, "b", "KeyB");
  ok("보드 가림 + ownsKey 참: 검은 화면 그대로, dp·sp 없음", P.black && hidden(w) && !e.dp && !e.sp);
  owns = false;
  key(w, "b", "KeyB");
  ok("보드 가림 + ownsKey 거짓: B 로 되돌린다", !P.black && !hidden(w));
  tap(w, P.slides[0].querySelector("img"));
  ok("보드 가림: 크게 보기 동안 가린다", P.lightbox === "img" && hidden(w), P.lightbox);
  ok("보드 겹침 층: 그 안의 click 은 먹지 않는다(크게 보기를 연 누르기의 깃발이 서 있어도)", !clickEaten(w, ov));
  owns = true;
  e = key(w, "Escape", "Escape");
  ok("보드 가림 + ownsKey 참: 크게 보기도 Esc 를 받지 않는다", P.lightbox === "img" && hidden(w) && !e.dp);
  owns = false;
  key(w, "Escape", "Escape");
  ok("보드 가림: 크게 보기를 닫으면 되돌린다", !P.lightbox && !hidden(w) && P.on);
  H.setBlack(true);
  H.stop();
  ok("보드 끝: 검은 화면에서 끝내도 겹침 층 요소는 남고 가림 클래스는 떨어진다", !P.on && w.body.kids.includes(ov) && !hidden(w));
}

// ---- 5. 슬라이드 묻기 막대(itda-hyve#896 — ask.js armSlides 가 장의 마지막 자식으로 단다): 덱 사본에 함께 오고, 그 위 누르기는 넘김이 아니며
// 펜·레이저 판서도 받지 않는다(present.js onMouse·ink.js BLOCKED). 보이기(마우스 올림)는 present.css — 호스트 앱 selftest 가 본다.
{
  const w = world(), { P } = w;
  const withBar = body => { const p = slide(body); const bar = p.appendChild(new El("div")); bar.className = "itda-askbar"; const b = bar.appendChild(new El("button")); b.className = "itda-ask"; b.attrs["data-ask"] = "claude"; return p; };
  w.H.start({ pages: [withBar([]), withBar([]), withBar([])], notes: [], index: 0, ratio: "16:9", onExit() {} });
  const btnOf = i => P.slides[i].querySelector("button.itda-ask");
  ok("묻기 막대: 덱 사본에 함께 온다", !!btnOf(0) && !!btnOf(1), P.slides.length);
  tap(w, btnOf(0));
  ok("묻기 막대: 단추 누르기는 다음 장이 아니다", P.idx === 0, P.idx);
  fire(btnOf(0), "mousedown", { button: 2, buttons: 2 });
  ok("묻기 막대: 오른쪽 누르기도 이전 장이 아니다", P.idx === 0, P.idx);
  tap(w, P.slides[0]);
  ok("묻기 막대: 막대 밖 누르기는 지금처럼 다음 장", P.idx === 1, P.idx);
  key(w, "p", "KeyP");
  const before = page(w, 1).length;
  drag(w, btnOf(1), line(100, 100, 5));
  ok("묻기 막대: 펜이 켜져도 단추 위에서는 그리지 않는다(ink BLOCKED)", page(w, 1).length === before && P.idx === 1, { strokes: page(w, 1).length, idx: P.idx });
  key(w, "p", "KeyP");
  key(w, "l", "KeyL");
  tap(w, btnOf(1));
  ok("묻기 막대: 레이저가 켜져도 단추 누르기는 넘김이 아니다", P.idx === 1, P.idx);
  key(w, "l", "KeyL");
  w.H.stop();
}

// ---- 6. 묻기 넘침 안내 상자(itda-hyve#896 A6 — ask.js NOTE, body 의 자식 .itda-asknote[data-pr-overlay]): 그 안에 포커스가 있으면 발표는 Esc 밖의 키를
// 양보한다(넘김·판서·preventDefault 없음). 상자 밖은 지금처럼, 가린 동안(검은 화면)은 양보하지 않는다.
{
  const w = world(), { P, H } = w;
  w.H.start({ pages: [slide([]), slide([]), slide([])], notes: [], index: 0, ratio: "16:9", onExit() {} });
  const box = overlayOf(w);
  box.className = "itda-asknote";
  const ta = box.appendChild(new El("textarea")), btn = box.appendChild(new El("button"));
  const yielded = (k, code, mods) => { const e = key(w, k, code, mods); return !e.dp && !e.sp; };
  ok("안내 상자: 글 칸 포커스의 ⌘C·Ctrl+C 는 발표가 먹지 않는다(기본 복사)", yielded("c", "KeyC", { metaKey: true, target: ta }) && yielded("c", "KeyC", { ctrlKey: true, target: ta }) && P.idx === 0, P.idx);
  ok("안내 상자: 단추 포커스의 Tab·Enter·Space 는 넘김이 아니다", yielded("Tab", "Tab", { target: btn }) && yielded("Enter", "Enter", { target: btn }) && yielded(" ", "Space", { target: btn }) && P.idx === 0, P.idx);
  ok("안내 상자: 판서 키(L)도 받지 않는다", yielded("l", "KeyL", { target: ta }) && !toolOf(w), toolOf(w));
  w.doc.activeElement = btn;
  ok("안내 상자: 이벤트 대상이 없으면 document.activeElement 로 본다", yielded("Enter", "Enter") && P.idx === 0, P.idx);
  w.doc.activeElement = null;
  let e = key(w, "ArrowRight", "ArrowRight", { target: w.body });
  ok("안내 상자: 상자 밖 포커스는 지금처럼 넘기고 키를 삼킨다", P.idx === 1 && e.dp && e.sp, { idx: P.idx, dp: e.dp });
  H.setBlack(true);
  e = key(w, "Enter", "Enter", { target: btn });
  ok("안내 상자: 가린 동안(검은 화면)은 양보하지 않는다", e.dp && e.sp, { dp: e.dp });
  H.setBlack(false);
  e = key(w, "Escape", "Escape", { target: ta });
  ok("안내 상자: Esc 는 양보하지 않는다(발표 끝내기)", !P.on && e.dp, { on: P.on });
}

// ---- 7. 두 화면 발표(itda-doc#95 — present.js 머리 주석 「두 화면 발표」): onState·audience 열기·show 덮어쓰기
const chartFig = n => { const f = new El("figure"); f.className = "chart"; if (n) f.dataset.step = String(n); return f; };
// snap 은 덮개 아래 모든 요소의 클래스·데이터를 한 글로 적는다 — 같은 show 를 두 번 불러 DOM 이 바뀌지 않았는지 본다.
const snap = r => { const out = []; const walk = e => { out.push(e.tag + "." + e.className + JSON.stringify(e.dataset) + e._text); e.kids.forEach(walk); }; walk(r); return out.join("|"); };
const playing = s => s.querySelectorAll("figure.chart").filter(f => f.classList.contains("chart-play")).length;
{
  const w = world(), { P, H } = w, states = [], slides = [];
  fakeLightbox(w);
  H.start({ pages: [slide([]), slide([stepped(1), stepped(2), img()], 2), slide([])], notes: [], index: 0, ratio: "16:9", onExit() {},
    onSlide: (i, n) => slides.push(i), onState: s => states.push(s) });
  const last = () => states[states.length - 1];
  ok("onState: 열 때 첫 상태를 한 번 보낸다", states.length === 1 && JSON.stringify(last()) === JSON.stringify({ i: 0, step: 0, steps: 0, total: 3, black: false, grid: false, lightbox: false, on: true }), states);
  H.next();
  ok("onState: 장이 바뀌면 한 번(steps 는 그 장의 단계 수)", states.length === 2 && last().i === 1 && last().step === 0 && last().steps === 2, states.length);
  H.next();
  ok("onState: 단계만 바뀌어도 한 번 — onSlide 는 지금처럼 부르지 않는다", states.length === 3 && last().step === 1 && slides.join() === "0,1", { n: states.length, slides });
  H.setBlack(true);
  ok("onState: 검은 화면 켜기에 한 번", states.length === 4 && last().black === true, states.length);
  H.setBlack(true);
  ok("onState: 같은 값이면 부르지 않는다(검은 화면 다시 켜기)", states.length === 4, states.length);
  H.setBlack(false);
  H.toggleGrid(true);
  ok("onState: 한눈에 보기 켜기에 한 번", states.length === 6 && last().grid === true && last().black === false, states.length);
  H.toggleGrid(false);
  ok("onState: 한눈에 보기 끄기에 한 번", states.length === 7 && last().grid === false, states.length);
  H.next(); // 단계 2 — 그림이 보인다
  tap(w, P.slides[1].querySelector("img"));
  ok("onState: 크게 보기 열기에 한 번(lightbox 참)", states.length === 9 && last().lightbox === true && P.idx === 1, { n: states.length, s: last() });
  H.next();
  ok("onState: 크게 보기가 열린 채 넘기면 닫힘·장 바뀜을 한 번에 보낸다", states.length === 10 && last().lightbox === false && last().i === 2, { n: states.length, s: last() });
  H.jump(2); H.show({ i: 2, step: 0, black: false });
  ok("onState: 바뀐 것 없는 jump·show 는 부르지 않는다", states.length === 10, states.length);
  H.stop();
  ok("onState: 끝낼 때 on:false 를 한 번", states.length === 11 && JSON.stringify(last()) === JSON.stringify({ i: 2, step: 0, steps: 0, total: 3, black: false, grid: false, lightbox: false, on: false }), last());
  open(w); H.next(); H.stop();
  ok("onState: 다음 발표(onState 없음)는 지난 onState 를 부르지 않는다", states.length === 11, states.length);
}
{
  const w = world(), { P, H } = w, calls = [], states = [];
  fakeLightbox(w);
  w.win.ItdaPresentHost = { fullscreen: on => calls.push(on) };
  const a = new El("a"); a.setAttribute("href", "https://example.com/x");
  H.start({ pages: [slide([img()]), slide([a]), slide([])], notes: ["<p>노트</p>", "", ""], index: 0, ratio: "16:9", onExit() {}, audience: true, onState: s => states.push(s) });
  const root = rootOf(w);
  const parts = [".pr-notes", ".pr-notes-grip", ".pr-toc", ".pr-toc-grip", ".pr-hud", ".pr-goto", ".pr-grid", ".pr-lightbox", ".pr-ink-bar"].filter(s => root.querySelector(s));
  ok("audience: 덱·검은 화면 층만 — 노트·목차·아래 줄·한눈에·크게 보기·판서 도구가 없다", parts.length === 0 && !!root.querySelector(".pr-deck") && !!root.querySelector(".pr-black") && H.ink() === null && root.classList.contains("pr-audience"), parts);
  ok("audience: 호스트 전체 화면 훅은 그대로 부르고 첫 상태를 보낸다", calls.join() === "true" && P.fsCalls === 1 && states.length === 1 && states[0].i === 0, { calls, n: states.length });
  const e = key(w, "ArrowRight", "ArrowRight");
  key(w, "g", "KeyG"); key(w, "n", "KeyN"); key(w, "l", "KeyL"); key(w, "Escape", "Escape");
  ok("audience: 키로 넘기지·끝내지 않고 키를 삼키지도 않는다", P.on && P.idx === 0 && !P.grid && !P.showNotes && !e.dp && !e.sp && !(w.doc.on.keydown || []).length, { idx: P.idx, dp: e.dp });
  tap(w, P.slides[0].querySelector("img"));
  fire(P.slides[0], "mousedown", { button: 2 });
  ok("audience: 마우스로 넘기거나 크게 보지 않고 커서 깨우기도 없다", P.idx === 0 && !P.lightbox && !(root.on.mousedown || []).length && !(root.on.mousemove || []).length && !root.classList.contains("pr-idle"), { idx: P.idx, lb: P.lightbox });
  H.toggleGrid(true);
  ok("audience: 한눈에 보기는 켜지지 않는다", !P.grid && states.length === 1, P.grid);
  H.show({ i: 1 });
  ok("audience: show 로만 넘어가고 onState 를 보낸다", P.idx === 1 && states.length === 2 && states[1].i === 1, { idx: P.idx, n: states.length });
  ok("audience: 덱 안 링크 click 은 먹어 창이 이동하지 않는다", clickEaten(w, P.slides[1].querySelector("a")));
  H.stop();
  ok("audience: 끝낼 때 전체 화면 끄기·on:false", !P.on && calls.join() === "true,false" && states.length === 3 && states[2].on === false, { calls, n: states.length });
  open(w);
  ok("audience: 다음 보통 발표는 노트·아래 줄을 다시 만든다", !!rootOf(w).querySelector(".pr-notes") && !!rootOf(w).querySelector(".pr-hud") && !rootOf(w).classList.contains("pr-audience"));
  H.stop();
}
{
  const w = world(), { P, H } = w, states = [];
  H.start({ pages: [slide([]), slide([stepped(1), chartFig(2)], 2), slide([chartFig()])], notes: [], index: 0, ratio: "16:9", onExit() {}, audience: true, onState: s => states.push(s) });
  const root = rootOf(w);
  H.show({ i: 2, step: 0 });
  ok("show: 앞으로 간 장은 차트가 움직인다(go 규칙)", P.idx === 2 && P.step === 0 && playing(P.slides[2]) === 1, playing(P.slides[2]));
  for (const f of root.querySelectorAll("figure.chart")) f.classList.remove("chart-play");
  let before = snap(root), n0 = states.length;
  H.show({ i: 2, step: 0 });
  ok("show 멱등: 같은 값 두 번이면 DOM·차트 재생·onState 변화 없음", snap(root) === before && playing(P.slides[2]) === 0 && states.length === n0, { same: snap(root) === before, n: states.length - n0 });
  H.show({ i: 1, step: 1 });
  ok("show: 뒤로 간 장은 그 단계까지 보이고 차트는 움직이지 않는다", P.idx === 1 && P.step === 1 && shown(P) === "10" && playing(P.slides[1]) === 0 && states.length === n0 + 1, { step: P.step, s: shown(P) });
  before = snap(root);
  H.show({ i: 1, step: 1, black: false });
  ok("show 멱등: 뒤로 간 장에서도 두 번째는 변화 없음", snap(root) === before && states.length === n0 + 1);
  H.show({ step: 2 });
  ok("show: 같은 장의 단계 앞으로 — 새로 보인 차트만 움직인다(next 와 같다)", P.idx === 1 && P.step === 2 && shown(P) === "11" && playing(P.slides[1]) === 1, { s: shown(P), p: playing(P.slides[1]) });
  H.show({ i: 99, step: -5 });
  ok("show: 범위 밖 장·단계는 잘라낸다(위)", P.idx === 2 && P.step === 0, { idx: P.idx, step: P.step });
  H.show({ i: -3 });
  H.show({ i: 1, step: 99 });
  ok("show: 범위 밖 단계는 그 장의 단계 수로", P.idx === 1 && P.step === 2, { idx: P.idx, step: P.step });
  H.show({ i: "x", step: null });
  ok("show: 숫자가 아닌 값은 그대로 둔다", P.idx === 1 && P.step === 2);
  const n1 = states.length;
  H.show({ i: 0, black: true });
  ok("show: 장·검은 화면을 함께 바꿔도 onState 는 한 번", P.idx === 0 && P.black && root.classList.contains("pr-black-on") && states.length === n1 + 1 && states[n1].black && states[n1].i === 0, states.length - n1);
  H.show({ black: false });
  ok("show: 검은 화면 끄기(setBlack 과 같은 길)", !P.black && !root.classList.contains("pr-black-on") && states.length === n1 + 2);
  H.stop();
  H.show({ i: 1 });
  ok("show: 발표 밖에서는 아무것도 하지 않는다", !P.on && states.length === n1 + 3);
}

// ---- 8. 끝 콜백 안에서 새 발표 열기(리뷰 [1]): 옛 발표는 끝 on:false 를 받고, 새 발표는 옛 끝 알림을 받지 않으며 이후 알림·audience 표식이 이어진다
for (const via of ["onExit", "onSlide"]) {
  const w = world(), { P, H } = w, recA = [], recB = [];
  const three = () => [slide([]), slide([]), slide([])];
  const openB = () => H.start({ pages: three(), notes: [], index: 0, ratio: "16:9", onExit() {}, audience: true, onState: s => recB.push(s) });
  H.start({ pages: three(), notes: [], index: 0, ratio: "16:9", onState: s => recA.push(s),
    onExit: () => { if (via === "onExit") openB(); }, onSlide: i => { if (via === "onSlide" && i === -1) openB(); } });
  H.next();
  H.stop();
  ok(via + " 안에서 새 발표: 옛 발표는 끝 on:false 를 받는다", recA.length === 3 && recA[2].on === false && recA[2].i === 1, recA);
  ok(via + " 안에서 새 발표: 새 발표는 첫 상태만 받고 옛 끝 알림은 받지 않는다", recB.length === 1 && recB[0].on === true && recB[0].i === 0, recB);
  H.show({ i: 1 });
  ok(via + " 안에서 새 발표: 새 발표의 알림이 이어지고 audience 표식이 남는다", P.on && P.audience && rootOf(w).classList.contains("pr-audience") && !rootOf(w).querySelector(".pr-hud") && recB.length === 2 && recB[1].i === 1, { on: P.on, aud: P.audience, n: recB.length });
  H.stop();
  ok(via + " 안에서 새 발표: 새 발표를 끝내면 그 발표에 on:false, 옛 발표에는 더 없다", recB.length === 3 && recB[2].on === false && recA.length === 3, { a: recA.length, b: recB.length });
}

// ---- 9. 청중 덱의 입력 부품(리뷰 [2]): 덱 사본의 묻기 단추·링크는 포커스 차례에서 빠지고, 누르기·키보드 활성화의 click 이 문서의 위임 처리기
// (ask.js handler)까지 가지 않는다. 실제 WebKit 의 inert·Tab 순서는 미검증(가짜 DOM 은 inert 속성을 다는지만 본다).
{
  const w = world(), { P, H } = w, A = w.win.ItdaAsk, opened = [];
  A.set({ services: ["claude"], course: "과정", doc: "교안" });
  w.doc.addEventListener("click", A.handler({ open: u => opened.push(u) })); // 호스트가 문서에 한 번 거는 위임 처리기(거품 단계)
  const asked = () => { const p = slide([]); p.className = "page slide"; const bar = p.appendChild(new El("div")); bar.className = "itda-askbar";
    const b = bar.appendChild(new El("button")); b.className = "itda-ask"; b.setAttribute("data-ask", "claude"); const a = p.appendChild(new El("a")); a.setAttribute("href", "https://example.com/"); return p; };
  // dispatch 는 click 하나를 창 붙잡기 단계 → 문서 거품 단계 차례로 보낸다(앞에서 stopPropagation 하면 멈춘다). detail 0 은 키보드 활성화(Enter·Space)다.
  const dispatch = (t, detail) => { const e = { type: "click", target: t, button: 0, detail, dp: false, sp: false, preventDefault() { this.dp = true; }, stopPropagation() { this.sp = true; } };
    for (const f of [...(w.win.on.click || [])]) { f(e); if (e.sp) return e; }
    for (const f of [...(w.doc.on.click || [])]) { f(e); if (e.sp) return e; }
    return e; };
  const src = [asked(), asked()];
  H.start({ pages: src, notes: [], index: 0, ratio: "16:9", onExit() {} });
  dispatch(P.slides[0].querySelector("button.itda-ask"), 1);
  ok("묻기 위임(대조): 보통 발표의 덱 단추는 지금처럼 ask 로 간다", opened.length === 1, opened.length);
  H.stop();
  H.start({ pages: src, notes: [], index: 0, ratio: "16:9", onExit() {}, audience: true });
  const deck = rootOf(w).querySelector(".pr-deck"), btn = P.slides[0].querySelector("button.itda-ask"), link = P.slides[0].querySelector("a");
  ok("audience: 덱에 inert 를 단다", "inert" in deck.attrs && deck.inert === true, deck.attrs);
  ok("audience: 덱 사본의 단추·링크는 포커스 차례에서 빠지고 원본은 그대로다", btn.getAttribute("tabindex") === "-1" && link.getAttribute("tabindex") === "-1" && src[0].querySelector("button.itda-ask").getAttribute("tabindex") === undefined);
  let e = dispatch(btn, 1);
  ok("audience: 복제 묻기 단추의 위임 클릭은 ask 로 가지 않는다(붙잡기 단계에서 먹는다)", opened.length === 1 && e.dp && e.sp, { n: opened.length, dp: e.dp });
  e = dispatch(btn, 0);
  ok("audience: 키보드 활성화(detail 0 click)도 ask 로 가지 않는다", opened.length === 1 && e.dp && e.sp && P.idx === 0, opened.length);
  e = dispatch(P.slides[0], 1);
  ok("audience: 덱의 여느 누르기도 넘김이 아니고 먹는다", P.idx === 0 && e.dp, P.idx);
  H.stop();
  dispatch(src[0].querySelector("button.itda-ask"), 1);
  ok("audience 끝: 발표 밖(원본 단추)의 위임 클릭은 지금처럼 ask 로 간다", opened.length === 2, opened.length);
}

console.log("PRESENT INK DONE cases=" + cases + " failed=" + failed);
