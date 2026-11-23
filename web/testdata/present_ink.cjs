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
  // 자손 결합자(빈칸)도 받는다 — 마지막 조각은 이 요소, 앞 조각들은 조상에서 차례로 찾는다(present.js NEXT_COUNTERS 의 ".page-body figcaption")
  matches(s) {
    return s.split(",").some(x => {
      const parts = x.trim().split(/\s+/);
      if (!matchOne(this, parts.pop())) return false;
      let e = this.parentNode;
      while (parts.length) { while (e && !(e.classList && matchOne(e, parts[parts.length - 1]))) e = e.parentNode; if (!e) return false; parts.pop(); e = e.parentNode; }
      return true;
    });
  }
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
  win.document = doc;
  doc.createElement = t => new El(t.toLowerCase());
  doc.createElementNS = (ns, t) => new El(t);
  const raf = [];
  Object.assign(win, { devicePixelRatio: 2, navigator: { platform: "MacIntel" }, matchMedia: () => ({ matches: false }), requestAnimationFrame: f => { const g = () => f(); g.id = ++world.seq; raf.push(g); return g.id; }, cancelAnimationFrame: id => { const i = raf.findIndex(g => g.id === id); if (i >= 0) raf.splice(i, 1); },
    ItdaSlides: { SIZE: { "16:9": [1280, 720], "4:3": [960, 720] } }, getComputedStyle: () => ({ backgroundColor: "rgb(255, 255, 255)", width: "0px", height: "0px" }) });
  win.Path2D = class { constructor(d) { this.d = d; } };
  const ctx = vm.createContext({ window: win, document: doc, URL, performance: { now: () => world.clock }, setTimeout: (f, ms) => { const id = ++world.seq; world.timers.set(id, f); return id; }, clearTimeout: id => world.timers.delete(id), setInterval: () => 0, clearInterval() {}, requestAnimationFrame: win.requestAnimationFrame, console, Math, parseFloat, Number, String, Map, WeakMap, Set, Infinity });
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
// pointer 꼴이 아닌 이벤트(mousedown 등 — 실제 MouseEvent)에는 pointerType·pointerId·pressure 가 없다: 넘겨도 뺀다.
function fire(target, type, ev) {
  const e = Object.assign({ type, target, button: 0, buttons: 0, clientX: 0, clientY: 0, pointerId: 1, pointerType: "mouse", pressure: 0.5, defaultPrevented: false,
    preventDefault() { this.defaultPrevented = true; }, stopPropagation() { this.stopped = true; } }, ev);
  if (!/^pointer|^lostpointercapture$/.test(type)) { delete e.pointerType; delete e.pointerId; delete e.pressure; }
  for (let n = target; n && !e.stopped; n = n.parentNode) for (const f of [...(n.on[type] || [])]) f(e);
  return e;
}
// click 은 뒤따르는 click 이 창 붙잡기 단계에서 먹히는가를 돌려준다(먹히면 문서의 링크 처리까지 가지 않는다).
function clickEaten(w, target) { const e = { type: "click", target, button: 0, preventDefault() { this.dp = true; }, stopPropagation() {} }; for (const f of [...(w.win.on.click || [])]) f(e); return !!e.dp; }
// 기본 자리는 무대 오른쪽 가장자리(x 1200 — 무대 1280px 의 7% = 89.6px 안)다: 보통 누르기는 다음(present.js edgeAt). 가운데 대상을 누를 때는 clientX 를 준다
function tap(w, target, ev) {
  const a = Object.assign({ clientX: 1200, clientY: 100, button: 0 }, ev);
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
  tap(w, P.slides[1].querySelector("img"), { clientX: 640 });
  ok("크게 보기 부품 없음: 가운데 그림 클릭은 아무것도 하지 않는다(넘기지 않는다)", P.idx === 1 && !P.lightbox, { idx: P.idx, lb: P.lightbox });
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
  tap(w, P.slides[0].querySelector("a"), { clientX: 640 });
  ok("ownLink: 호스트 전용 앵커는 링크로 다루지 않는다 — 가운데 누르기는 빈 곳과 같다(넘기지 않는다)", P.idx === 0 && opened.length === 0, { idx: P.idx, opened });
  H.next();
  const eaten = tap(w, P.slides[1].querySelector("a"), { clientX: 640 });
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
  tap(w, P.slides[1].querySelector("img"), { clientX: 640 });
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
  ok("onState: 열 때 첫 상태를 한 번 보낸다", states.length === 1 && JSON.stringify(last()) === JSON.stringify({ i: 0, step: 0, steps: 0, total: 3, next: 1, black: false, grid: false, lightbox: false, on: true }), states);
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
  tap(w, P.slides[1].querySelector("img"), { clientX: 640 });
  ok("onState: 크게 보기 열기에 한 번(lightbox 참)", states.length === 9 && last().lightbox === true && P.idx === 1, { n: states.length, s: last() });
  H.next();
  ok("onState: 크게 보기가 열린 채 넘기면 닫힘·장 바뀜을 한 번에 보낸다", states.length === 10 && last().lightbox === false && last().i === 2, { n: states.length, s: last() });
  H.jump(2); H.show({ i: 2, step: 0, black: false });
  ok("onState: 바뀐 것 없는 jump·show 는 부르지 않는다", states.length === 10, states.length);
  H.stop();
  ok("onState: 끝낼 때 on:false 를 한 번", states.length === 11 && JSON.stringify(last()) === JSON.stringify({ i: 2, step: 0, steps: 0, total: 3, next: -1, black: false, grid: false, lightbox: false, on: false }), last());
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
  A.set({ services: ["claude"], mcp: "https://docs.example/mcp", doc_id: "01" });
  w.doc.addEventListener("click", A.handler({ open: u => opened.push(u) })); // 호스트가 문서에 한 번 거는 위임 처리기(거품 단계)
  const asked = () => {
    const p = slide([]), title = p.appendChild(new El("h2"));
    title.id = "intro";
    A.armSections(p);
    p.appendChild(new El("a")).setAttribute("href", "https://example.com/");
    return p;
  };
  // dispatch 는 click 하나를 창 붙잡기 단계 → 문서 거품 단계 차례로 보낸다(앞에서 stopPropagation 하면 멈춘다). detail 0 은 키보드 활성화(Enter·Space)다.
  const dispatch = (t, detail) => { const e = { type: "click", target: t, button: 0, detail, dp: false, sp: false, preventDefault() { this.dp = true; }, stopPropagation() { this.sp = true; } };
    for (const f of [...(w.win.on.click || [])]) { f(e); if (e.sp) return e; }
    for (const f of [...(w.doc.on.click || [])]) { f(e); if (e.sp) return e; }
    return e; };
  const src = [asked(), asked()];
  H.start({ pages: src, notes: [], index: 0, ratio: "16:9", onExit() {} });
  dispatch(P.slides[0].querySelector("button.itda-section-ask"), 1);
  ok("묻기 위임(대조): 발표 사본에서는 새 대화를 열지 않는다", opened.length === 0, opened.length);
  H.stop();
  H.start({ pages: src, notes: [], index: 0, ratio: "16:9", onExit() {}, audience: true });
  const deck = rootOf(w).querySelector(".pr-deck"), btn = P.slides[0].querySelector("button.itda-section-ask"), link = P.slides[0].querySelector("a");
  ok("audience: 덱에 inert 를 단다", "inert" in deck.attrs && deck.inert === true, deck.attrs);
  ok("audience: 덱 사본의 단추·링크는 포커스 차례에서 빠지고 원본은 그대로다", btn.getAttribute("tabindex") === "-1" && link.getAttribute("tabindex") === "-1" && src[0].querySelector("button.itda-section-ask").getAttribute("tabindex") === undefined);
  let e = dispatch(btn, 1);
  ok("audience: 복제 묻기 단추의 위임 클릭은 ask 로 가지 않는다(붙잡기 단계에서 먹는다)", opened.length === 0 && e.dp && e.sp, { n: opened.length, dp: e.dp });
  e = dispatch(btn, 0);
  ok("audience: 키보드 활성화(detail 0 click)도 ask 로 가지 않는다", opened.length === 0 && e.dp && e.sp && P.idx === 0, opened.length);
  e = dispatch(P.slides[0], 1);
  ok("audience: 덱의 여느 누르기도 넘김이 아니고 먹는다", P.idx === 0 && e.dp, P.idx);
  H.stop();
  dispatch(src[0].querySelector("button.itda-section-ask"), 1);
  ok("audience 끝: 발표 밖(원본 단추)의 위임 클릭은 지금처럼 ask 로 간다", opened.length === 1, opened.length);
}

// ---- 10. 펜만 판서(itda-doc#100 — present.js 머리 주석 「펜만 판서」): penOnly 면 판서는 pointerType "pen" 만 받는다. 판서 도구가 켜진 동안 펜이 아닌
// 누르기(손가락·마우스, 그 호환 mousedown·click)는 장을 넘기지 않고 먹고, 도구를 끄면 누르기 = 다음이다. 손가락 밀기는 호스트 몫이라 발표가 판정하지 않고,
// 호스트는 inkHeld() 로 펜 입력 진행을 묻는다. 옵션이 없으면 지금 그대로다. mousedown 은 실제 MouseEvent 꼴(pointerType 없음 — fire)이다.
// swipeBy 는 손가락 하나의 밀기다 — 실제 웹뷰처럼 밀기 뒤에는 mousedown 을 보내지 않는다.
function swipeBy(target, x0, x1, ev) {
  const a = Object.assign({ pointerType: "touch", pointerId: 2, button: 0 }, ev);
  fire(target, "pointerdown", { ...a, buttons: 1, clientX: x0, clientY: 300 });
  for (let k = 1; k <= 4; k++) fire(target, "pointermove", { ...a, buttons: 1, clientX: x0 + (x1 - x0) * k / 4, clientY: 300 });
  fire(target, "pointerup", { ...a, clientX: x1, clientY: 300 });
}
{
  const w = world(), { P, H, I } = w;
  // 대조: 옵션이 없으면 손가락도 그린다
  ok("inkHeld: 발표 밖에서는 거짓", H.inkHeld() === false);
  open(w);
  key(w, "p", "KeyP");
  drag(w, P.slides[0], line(100, 300, 6), { pointerType: "touch", pointerId: 2 });
  ok("penOnly 없음: 손가락 끌기는 지금처럼 획이다", page(w, 0).length === 1 && P.idx === 0 && !rootOf(w).classList.contains("pr-pen-only"), { n: page(w, 0).length, idx: P.idx });
  H.stop();
  // penOnly: 손가락·마우스는 판서가 아니다
  const lasers = [];
  I.relay("pen-only", { ink: () => "", laser: (...a) => lasers.push(a) });
  open(w, { penOnly: true, inkKey: "pen-only" });
  const root = rootOf(w), s0 = () => P.slides[P.idx];
  ok("penOnly: 덮개에 pr-pen-only 가 붙는다", root.classList.contains("pr-pen-only"));
  key(w, "p", "KeyP");
  const touchDraw = drag(w, s0(), line(100, 300, 6), { pointerType: "touch", pointerId: 2 });
  ok("penOnly + 펜 도구: 손가락 끌기는 획을 만들지 않는다", page(w, 0).length === 0 && !w.H.ink().held(), page(w, 0).length);
  ok("penOnly + 펜 도구: 손가락 끌기의 mousedown 은 넘기지 않고 click 을 먹는다", P.idx === 0 && touchDraw, { idx: P.idx, eaten: touchDraw });
  let eaten = tap(w, s0(), { pointerType: "touch", pointerId: 2 });
  ok("penOnly + 펜 도구: 손가락 누르기는 장을 넘기지 않고 click 을 먹는다", P.idx === 0 && eaten && page(w, 0).length === 0, { idx: P.idx, eaten });
  eaten = tap(w, s0(), { pointerType: "mouse" });
  ok("penOnly + 펜 도구: 마우스 누르기도 획이 아니고 넘기지도 않는다", P.idx === 0 && eaten && page(w, 0).length === 0, { idx: P.idx, n: page(w, 0).length });
  eaten = tap(w, s0(), { pointerType: "mouse", button: 2 });
  ok("penOnly + 펜 도구: 오른쪽 누르기도 이전으로 가지 않는다", P.idx === 0, P.idx);
  swipeBy(s0(), 600, 200);
  ok("penOnly: 손가락 밀기는 획이 아니고, 발표가 넘기지도 않는다(밀기는 호스트 몫)", P.idx === 0 && page(w, 0).length === 0 && !H.inkHeld(), { idx: P.idx, n: page(w, 0).length });
  // 도구를 끄면 보통 누르기다 — 오른쪽 가장자리 = 다음
  key(w, "p", "KeyP");
  eaten = tap(w, s0(), { pointerType: "touch", pointerId: 2, clientX: 1200 });
  ok("penOnly + 도구 끔: 손가락 누르기(오른쪽 가장자리)는 다음이다", P.idx === 1 && !eaten, { idx: P.idx, eaten });
  tap(w, s0(), { pointerType: "mouse", clientX: 1200 });
  ok("penOnly + 도구 끔: 마우스 누르기(오른쪽 가장자리)도 다음이다", P.idx === 2, P.idx);
  key(w, "p", "KeyP");
  H.jump(0);
  // 지우개: 손가락은 지우지 않는다
  drag(w, s0(), line(100, 100, 6), { pointerType: "pen", pointerId: 1 });
  ok("penOnly + 펜 도구: 펜 끌기는 획이다", page(w, 0).length === 1, page(w, 0).length);
  key(w, "e", "KeyE");
  fire(s0(), "pointerdown", { pointerType: "touch", pointerId: 2, buttons: 1, clientX: 120, clientY: 100 });
  fire(s0(), "pointermove", { pointerType: "touch", pointerId: 2, buttons: 1, clientX: 140, clientY: 100 });
  fire(s0(), "pointerup", { pointerType: "touch", pointerId: 2, clientX: 140, clientY: 100 });
  ok("penOnly + 지우개: 손가락은 지우지 않는다", page(w, 0).length === 1 && !w.H.ink().drawing(), page(w, 0).length);
  // 레이저: 손가락은 레이저를 움직이지 않는다, 펜은 움직인다
  key(w, "l", "KeyL");
  runTimers(); lasers.length = 0;
  fire(s0(), "pointerdown", { pointerType: "touch", pointerId: 2, buttons: 1, clientX: 300, clientY: 300 });
  fire(s0(), "pointermove", { pointerType: "touch", pointerId: 2, buttons: 1, clientX: 340, clientY: 300 });
  runTimers();
  ok("penOnly + 레이저: 손가락은 레이저 누르기·자리를 만들지 않는다", lasers.length === 0 && !w.H.ink().held(), lasers);
  fire(s0(), "pointerup", { pointerType: "touch", pointerId: 2, clientX: 340, clientY: 300 });
  fire(s0(), "pointermove", { pointerType: "pen", pointerId: 1, buttons: 0, clientX: 300, clientY: 300 });
  runTimers();
  ok("penOnly + 레이저: 펜은 레이저를 움직인다", lasers.length === 1 && lasers[0][0] === 0 && lasers[0][1] === 300, lasers);
  eaten = tap(w, s0(), { pointerType: "mouse" });
  ok("penOnly + 레이저: 마우스 누르기는 넘기지 않는다(레이저가 켜진 동안)", P.idx === 0 && eaten, P.idx);
  eaten = tap(w, s0(), { pointerType: "pen", pointerId: 1, clientX: 1200 });
  ok("penOnly + 레이저: 펜 누르기(오른쪽 가장자리)는 지금처럼 레이저 클릭 — 한 번만 넘긴다", P.idx === 1, P.idx);
  H.jump(0);
  // 펜 필압은 지금처럼 담는다
  key(w, "p", "KeyP");
  drag(w, s0(), line(100, 500, 6), { pointerType: "pen", pointerId: 1, pressure: 0.8 });
  const last = page(w, 0)[page(w, 0).length - 1];
  ok("penOnly: 펜 획은 필압을 담는다", !!last.press && last.press.length === last.pts.length / 2 && last.press.every(v => v === 0.8), last.press);
  // 펜 획 진행 중 손가락: 획이 끊기지 않고 장이 넘어가지 않는다
  const n0 = page(w, 0).length, pen = { pointerType: "pen", pointerId: 1 }, fin = { pointerType: "touch", pointerId: 2 };
  fire(s0(), "pointerdown", { ...pen, buttons: 1, clientX: 100, clientY: 600 });
  fire(s0(), "pointermove", { ...pen, buttons: 1, clientX: 130, clientY: 600 });
  // 실제 순서: 손가락 down → up → (뗀 뒤) 호환 mousedown — pointerType 이 없는 MouseEvent 다 → click
  fire(s0(), "pointerdown", { ...fin, buttons: 1, clientX: 700, clientY: 300 });
  fire(s0(), "pointerup", { ...fin, clientX: 700, clientY: 300 });
  fire(s0(), "mousedown", { buttons: 1, clientX: 700, clientY: 300 });
  const finEaten = clickEaten(w, s0());
  ok("penOnly: 펜 획 중 손가락 누르기는 넘기지 않는다(뒤따르는 click 도 먹는다)", P.idx === 0 && finEaten && w.H.ink().drawing(), { idx: P.idx, eaten: finEaten });
  ok("inkHeld: 펜 획 진행 중에는 참 — 호스트가 이때의 손가락 밀기를 버린다", H.inkHeld() === true);
  fire(s0(), "pointermove", { ...pen, buttons: 1, clientX: 160, clientY: 600 });
  fire(s0(), "pointerup", { ...pen, clientX: 160, clientY: 600 });
  const one = page(w, 0)[page(w, 0).length - 1];
  // 흔들림 보정(SMOOTH)으로 셋째 점은 160 이 아니라 둘째 점과 160 사이다
  ok("penOnly: 펜 획 중 손가락이 닿아도 획은 하나로 이어진다", page(w, 0).length === n0 + 1 && one.pts.length === 6 && one.pts[4] > one.pts[2], { n: page(w, 0).length, pts: one.pts });
  ok("inkHeld: 펜을 떼면 거짓", H.inkHeld() === false);
  // 리뷰 1회차 [1] 순서: 펜 down(+ 호환 mousedown) → 손가락 down → 펜 up → 손가락 up → 손가락 호환 mousedown → click — 장 유지·click 먹음
  fire(s0(), "pointerdown", { ...pen, buttons: 1, clientX: 100, clientY: 650 });
  fire(s0(), "mousedown", { buttons: 1, clientX: 100, clientY: 650 });
  fire(s0(), "pointermove", { ...pen, buttons: 1, clientX: 140, clientY: 650 });
  fire(s0(), "pointerdown", { ...fin, buttons: 1, clientX: 500, clientY: 300 });
  fire(s0(), "pointerup", { ...pen, clientX: 140, clientY: 650 });
  fire(s0(), "pointerup", { ...fin, clientX: 500, clientY: 300 });
  fire(s0(), "mousedown", { buttons: 1, clientX: 500, clientY: 300 });
  eaten = clickEaten(w, s0());
  ok("리뷰 1회차 순서: 펜을 먼저 떼도 겹친 손가락의 호환 mousedown 은 넘기지 않고 click 도 먹는다", P.idx === 0 && eaten, { idx: P.idx, eaten });
  // 리뷰 2회차 [1] 순서: … 손가락 A up 뒤 A 의 호환 mousedown 앞에 다른 손가락 B 가 닿는다 → A 의 호환 mousedown·click — 장 유지·click 먹음
  fire(s0(), "pointerdown", { pointerType: "pen", pointerId: 11, buttons: 1, clientX: 100, clientY: 100 });
  fire(s0(), "mousedown", { buttons: 1, clientX: 100, clientY: 100 });
  fire(s0(), "pointerdown", { pointerType: "touch", pointerId: 22, buttons: 1, clientX: 700, clientY: 300 });
  fire(s0(), "pointerup", { pointerType: "pen", pointerId: 11, clientX: 100, clientY: 100 });
  fire(s0(), "pointerup", { pointerType: "touch", pointerId: 22, clientX: 700, clientY: 300 });
  world.clock += 50;
  fire(s0(), "pointerdown", { pointerType: "touch", pointerId: 33, buttons: 1, clientX: 900, clientY: 400 });
  fire(s0(), "mousedown", { buttons: 1, clientX: 700, clientY: 300 });
  eaten = clickEaten(w, s0());
  ok("리뷰 2회차 순서: 다른 손가락이 끼어도 겹친 손가락의 늦은 호환 mousedown 은 넘기지 않고 click 도 먹는다", P.idx === 0 && eaten, { idx: P.idx, eaten });
  fire(s0(), "pointerup", { pointerType: "touch", pointerId: 33, clientX: 900, clientY: 400 });
  // 시각·자리를 짐작하지 않는다: 오래 지나 다른 자리에 온 mousedown 도 도구가 켜져 있으면 먹는다
  world.clock += 5000;
  const n1 = page(w, 0).length;
  eaten = tap(w, s0(), { pointerType: "touch", pointerId: 4, clientX: 1000, clientY: 50 });
  ok("penOnly + 도구 켜짐: 펜과 겹치지 않은 새 손가락 누르기도 넘기지 않는다(획도 없다)", P.idx === 0 && eaten && page(w, 0).length === n1, { idx: P.idx, eaten });
  // 도구를 끈 직후 누르기는 다음 — 겹친 접촉의 흔적이 남지 않는다
  fire(s0(), "pointerdown", { ...pen, buttons: 1, clientX: 100, clientY: 690 });
  fire(s0(), "pointerdown", { ...fin, buttons: 1, clientX: 500, clientY: 300 });
  fire(s0(), "pointerup", { ...pen, clientX: 100, clientY: 690 });
  fire(s0(), "pointerup", { ...fin, clientX: 500, clientY: 300 });
  key(w, "p", "KeyP");
  fire(s0(), "mousedown", { buttons: 1, clientX: 1200, clientY: 300 });
  eaten = clickEaten(w, s0());
  ok("penOnly + 도구 끈 직후: 누르기(오른쪽 가장자리)는 다음이다", P.idx === 1 && !eaten && !toolOf(w), { idx: P.idx, eaten });
  H.stop();
  ok("inkHeld: 발표가 끝나면 거짓(판서가 떨어진다)", H.inkHeld() === false);
  // 청중 열기에는 판서가 없다 — penOnly 를 받지 않는다
  open(w, { penOnly: true, audience: true });
  ok("penOnly + audience: 받지 않는다(pr-pen-only 없음)", !rootOf(w).classList.contains("pr-pen-only") && !P.penOnly);
  ok("setPenOnly: 청중 열기에는 듣지 않는다(false)", H.setPenOnly(true) === false && !P.penOnly && !rootOf(w).classList.contains("pr-pen-only"));
  H.stop();
}

// ---- 10-2. 발표 중 펜만 판서 바꾸기(present.js 머리 주석 「펜만 판서」 setPenOnly): 발표를 다시 열지 않고 판정·클래스가 바뀌고, 장·판서·도구는 그대로다.
// 진행 중인 획은 바뀌기 전 판정으로 끝까지 가고, 바뀐 판정은 다음 누르기부터다.
{
  const w = world(), { P, H } = w;
  ok("setPenOnly: 발표 밖에서는 듣지 않는다(false)", H.setPenOnly(true) === false && !P.penOnly);
  open(w, { penOnly: true, inkKey: "live-pen" });
  const root = rootOf(w), s0 = () => P.slides[P.idx], fin = { pointerType: "touch", pointerId: 2 }, pen = { pointerType: "pen", pointerId: 1 };
  H.jump(2);
  key(w, "p", "KeyP");
  drag(w, s0(), line(100, 100, 6), fin);
  ok("setPenOnly 앞: 펜만이면 손가락 끌기는 획이 아니다", page(w, 2).length === 0, page(w, 2).length);
  ok("setPenOnly(false): 적용하면 true, 클래스·P.penOnly·판서 판정이 함께 바뀐다",
    H.setPenOnly(false) === true && !P.penOnly && !root.classList.contains("pr-pen-only") && w.H.ink().penOnly() === false && rootOf(w) === root);
  ok("setPenOnly: 장·도구는 그대로다(발표를 다시 열지 않는다)", P.idx === 2 && toolOf(w) === "pen" && P.on);
  const eaten = drag(w, s0(), line(100, 200, 6), fin);
  ok("손가락 모드: 손가락 끌기는 획이고 장이 넘어가지 않는다", page(w, 2).length === 1 && P.idx === 2 && eaten, { n: page(w, 2).length, idx: P.idx });
  // 두 손가락: 첫 손가락이 쥔 동안 둘째 포인터는 획을 만들지 않는다
  fire(s0(), "pointerdown", { ...fin, buttons: 1, clientX: 100, clientY: 300 });
  fire(s0(), "pointerdown", { pointerType: "touch", pointerId: 3, buttons: 1, clientX: 600, clientY: 300 });
  fire(s0(), "pointermove", { pointerType: "touch", pointerId: 3, buttons: 1, clientX: 650, clientY: 300 });
  fire(s0(), "pointermove", { ...fin, buttons: 1, clientX: 140, clientY: 300 });
  fire(s0(), "pointerup", { pointerType: "touch", pointerId: 3, clientX: 650, clientY: 300 });
  fire(s0(), "pointerup", { ...fin, clientX: 140, clientY: 300 });
  ok("손가락 모드: 둘째 손가락은 획을 더 만들지 않는다", page(w, 2).length === 2, page(w, 2).length);
  // 손가락 획 도중 펜만으로 바꿔도 그 획은 떼기까지 이어진다
  fire(s0(), "pointerdown", { ...fin, buttons: 1, clientX: 100, clientY: 400 });
  fire(s0(), "pointermove", { ...fin, buttons: 1, clientX: 140, clientY: 400 });
  H.setPenOnly(true);
  fire(s0(), "pointermove", { ...fin, buttons: 1, clientX: 180, clientY: 400 });
  ok("획 도중 펜만으로: 쥔 손가락 획은 이어진다(inkHeld 참)", H.inkHeld() === true && w.H.ink().drawing());
  fire(s0(), "pointerup", { ...fin, clientX: 180, clientY: 400 });
  const mid = page(w, 2)[page(w, 2).length - 1];
  ok("획 도중 펜만으로: 떼면 한 획으로 확정된다(점 셋)", page(w, 2).length === 3 && mid.pts.length === 6 && H.inkHeld() === false, { n: page(w, 2).length, pts: mid && mid.pts });
  drag(w, s0(), line(100, 500, 6), fin);
  ok("바뀐 뒤 다음 누르기부터 펜만: 손가락 끌기는 획이 아니다", page(w, 2).length === 3, page(w, 2).length);
  // 리뷰 1회차 [1]: 손가락 획 도중 펜만으로 바꾼 뒤 그 떼기를 놓치고 같은 id 로 새 손가락이 닿는다 — 앞 획만 확정하고 새 접촉은 거부한다
  H.setPenOnly(false);
  fire(s0(), "pointerdown", { ...fin, buttons: 1, clientX: 100, clientY: 450 });
  fire(s0(), "pointermove", { ...fin, buttons: 1, clientX: 140, clientY: 450 });
  H.setPenOnly(true);
  fire(s0(), "pointerdown", { ...fin, buttons: 1, clientX: 300, clientY: 450 }); // 떼기(up·cancel·lostpointercapture)를 놓쳤다
  ok("떼기 놓친 같은 id 의 새 손가락 down: 앞 획은 확정되고 새 접촉은 쥐지 않는다", page(w, 2).length === 4 && !H.inkHeld() && !w.H.ink().drawing(), { n: page(w, 2).length, held: H.inkHeld() });
  fire(s0(), "pointermove", { ...fin, buttons: 1, clientX: 340, clientY: 450 });
  fire(s0(), "pointerup", { ...fin, clientX: 340, clientY: 450 });
  ok("떼기 놓친 같은 id 의 새 손가락: 펜만이라 획이 늘지 않는다", page(w, 2).length === 4, page(w, 2).length);
  // 지우개·레이저도 같은 onDown 경계 — 손가락 지우개 도중 펜만으로, 떼기를 놓친 같은 id 의 새 down 은 지우지 않는다
  H.setPenOnly(false);
  key(w, "e", "KeyE");
  fire(s0(), "pointerdown", { ...fin, buttons: 1, clientX: 900, clientY: 900 }); // 획이 없는 자리 — 지우개 누르기만 쥔다
  ok("손가락 지우개 누르기를 쥔다(시험 전제)", H.inkHeld() && w.H.ink().drawing());
  H.setPenOnly(true);
  const nE = page(w, 2).length;
  fire(s0(), "pointerdown", { ...fin, buttons: 1, clientX: 120, clientY: 450 }); // 떼기를 놓친 채 위 획(y 450) 위에 닿는다
  ok("떼기 놓친 같은 id 의 새 손가락 지우개 down: 앞 입력만 정리되고 새 누르기는 쥐지 않는다", !H.inkHeld() && !w.H.ink().drawing(), { held: H.inkHeld() });
  fire(s0(), "pointermove", { ...fin, buttons: 1, clientX: 140, clientY: 450 });
  fire(s0(), "pointerup", { ...fin, clientX: 140, clientY: 450 });
  ok("떼기 놓친 같은 id 의 새 손가락 지우개: 펜만이라 지우지 않는다", page(w, 2).length === nE && nE === 4, { n: page(w, 2).length, nE });
  H.setPenOnly(false);
  key(w, "l", "KeyL");
  fire(s0(), "pointerdown", { ...fin, buttons: 1, clientX: 300, clientY: 300 });
  ok("손가락 레이저 누르기를 쥔다(시험 전제)", H.inkHeld());
  H.setPenOnly(true);
  fire(s0(), "pointerdown", { ...fin, buttons: 1, clientX: 320, clientY: 300 });
  ok("떼기 놓친 같은 id 의 새 손가락 레이저 down: 앞 누르기는 거두고 새 누르기는 쥐지 않는다", !H.inkHeld() && P.idx === 2, { held: H.inkHeld(), idx: P.idx });
  fire(s0(), "pointerup", { ...fin, clientX: 320, clientY: 300 });
  ok("레이저: 그 떼기도 클릭(넘김)이 되지 않는다", P.idx === 2, P.idx);
  key(w, "p", "KeyP");
  ok("펜 도구로 돌아온다(시험 전제)", toolOf(w) === "pen");
  // 다른 id 의 거부된 손가락은 진행 중인 펜 획을 정리하지 않는다(이미 위 「펜 획 중 손가락」 이 보지만, 같은 id 회수 뒤에도 그대로인지)
  fire(s0(), "pointerdown", { ...pen, buttons: 1, clientX: 100, clientY: 650 });
  fire(s0(), "pointermove", { ...pen, buttons: 1, clientX: 140, clientY: 650 });
  fire(s0(), "pointerdown", { ...fin, buttons: 1, clientX: 600, clientY: 650 });
  ok("펜만: 다른 id 의 손가락 down 은 진행 중인 펜 획을 끊지 않는다", H.inkHeld() && w.H.ink().drawing() && page(w, 2).length === 4);
  fire(s0(), "pointerup", { ...fin, clientX: 600, clientY: 650 });
  fire(s0(), "pointermove", { ...pen, buttons: 1, clientX: 180, clientY: 650 });
  fire(s0(), "pointerup", { ...pen, clientX: 180, clientY: 650 });
  const penOne = page(w, 2)[page(w, 2).length - 1];
  ok("펜만: 그 펜 획은 하나로 확정된다", page(w, 2).length === 5 && penOne.pts.length === 6, { n: page(w, 2).length, pts: penOne && penOne.pts });
  // 펜 획 도중 손가락 모드로 바꿔도 그 펜 획은 이어지고, 겹친 손가락은 끼어들지 않는다
  fire(s0(), "pointerdown", { ...pen, buttons: 1, clientX: 100, clientY: 600 });
  fire(s0(), "pointermove", { ...pen, buttons: 1, clientX: 140, clientY: 600 });
  H.setPenOnly(false);
  fire(s0(), "pointerdown", { ...fin, buttons: 1, clientX: 700, clientY: 300 });
  fire(s0(), "pointermove", { ...fin, buttons: 1, clientX: 720, clientY: 300 });
  fire(s0(), "pointerup", { ...fin, clientX: 720, clientY: 300 });
  fire(s0(), "pointermove", { ...pen, buttons: 1, clientX: 180, clientY: 600 });
  fire(s0(), "pointerup", { ...pen, clientX: 180, clientY: 600 });
  const penLine = page(w, 2)[page(w, 2).length - 1];
  ok("펜 획 도중 손가락 모드로: 펜 획은 하나로 이어지고 손가락은 끼어들지 않는다", page(w, 2).length === 6 && penLine.pts.length === 6, { n: page(w, 2).length, pts: penLine && penLine.pts });
  ok("바꾸기를 오가도 장·판서가 남는다", P.idx === 2 && page(w, 2).length === 6);
  H.stop();
  open(w, { inkKey: "live-pen" });
  ok("다음 발표는 setPenOnly 를 물려받지 않는다(start 의 o.penOnly)", !P.penOnly && w.H.ink().penOnly() === false && page(w, 2).length === 6);
  H.stop();
  open(w, { penOnly: true, inkKey: "live-pen" });
  ok("다음 발표가 penOnly 로 열면 펜만이다", P.penOnly && w.H.ink().penOnly() === true);
  H.stop();
  // 보드 발표에는 판서가 없다
  H.startBoard({ page: slide([]), ratio: "16:9", onExit() {} });
  ok("setPenOnly: 보드 발표에는 듣지 않는다(false)", H.setPenOnly(true) === false && !P.penOnly);
  H.stop();
}

// ---- 11. 장 넘김 전환(itda-doc#99 — present.css .t-*·present.js TRANSITIONS·OUT_MS): 가짜 DOM 은 CSS 를 그리지 않으므로 보이는 장을
// 클래스로 본다(present.css: pr-cur·pr-out 이면 보인다). 어느 전환이든 넘긴 직후에는 지금 장(pr-cur 하나) + 바로 앞 장(pr-out) 둘 이하,
// 정리 타이머 뒤에는 지금 장 하나다 — 겹쳐 남은 장·빈 화면이 없다. 움직임 길이(지연 포함 ≤ OUT_MS)는 present_test.go 가 CSS 글로 본다.
{
  const vis = P => P.slides.map((s, i) => (s.classList.contains("pr-cur") || s.classList.contains("pr-out") ? i : -1)).filter(i => i >= 0).join(",");
  const cls = (P, i) => ["pr-cur", "pr-in", "pr-out", "pr-back"].filter(c => P.slides[i].classList.contains(c)).join(" ");
  const curOk = P => P.slides.filter(s => s.classList.contains("pr-cur")).length === 1 && P.slides[P.idx].classList.contains("pr-cur") && !P.slides[P.idx].classList.contains("pr-out");
  const pages = () => [slide([]), slide([stepped(1), stepped(2)], 2), slide([]), slide([])];
  for (const t of ["fade", "slide", "zoom", "push", "cover"]) {
    const w = world(), { P, H } = w;
    runTimers();
    H.start({ pages: pages(), notes: [], index: 0, ratio: "16:9", onExit() {}, transition: t });
    const root = rootOf(w);
    ok(t + ": 덮개에 t-" + t + " 이 붙는다", root.classList.contains("t-" + t) && P.transition === t, root.className);
    // 앞으로
    H.next();
    ok(t + " 앞으로: 새 장 pr-cur pr-in, 옛 장 pr-out(pr-back 없음)", cls(P, 1) === "pr-cur pr-in" && cls(P, 0) === "pr-out" && vis(P) === "0,1" && curOk(P), { a: cls(P, 0), b: cls(P, 1) });
    runTimers();
    ok(t + " 앞으로: 정리 뒤 지금 장만 보인다", vis(P) === "1" && curOk(P), vis(P));
    // 단계 넘김: 같은 장 안에서는 다른 장을 건드리지 않는다
    H.next(); H.next();
    ok(t + " 단계: 같은 장의 단계 넘김은 나가는 장을 만들지 않는다", P.idx === 1 && P.step === 2 && vis(P) === "1" && cls(P, 1) === "pr-cur pr-in", { v: vis(P), c: cls(P, 1) });
    H.prev();
    ok(t + " 단계: 같은 장의 단계 거두기도", P.idx === 1 && P.step === 1 && vis(P) === "1", vis(P));
    // 뒤로: 단계를 다 거두고 앞 장으로
    H.prev(); H.prev();
    ok(t + " 뒤로: 새 장 pr-cur pr-in pr-back, 옛 장 pr-out pr-back", P.idx === 0 && cls(P, 0) === "pr-cur pr-in pr-back" && cls(P, 1) === "pr-out pr-back" && vis(P) === "0,1" && curOk(P), { a: cls(P, 0), b: cls(P, 1) });
    runTimers();
    ok(t + " 뒤로: 정리 뒤 지금 장만", vis(P) === "0" && curOk(P), vis(P));
    // 빠른 연달아 넘김: 정리 타이머가 돌기 전에 세 번 넘기고 한 번 돌아온다
    H.jump(1); H.jump(2); H.jump(3);
    ok(t + " 연달아 앞으로: 보이는 장은 지금·바로 앞 둘뿐", vis(P) === "2,3" && cls(P, 3) === "pr-cur pr-in" && cls(P, 2) === "pr-out" && curOk(P), { v: vis(P) });
    H.prev();
    ok(t + " 연달아 되돌아옴: 나가던 장이 다시 지금 장(pr-out 이 떨어진다)", P.idx === 2 && vis(P) === "2,3" && cls(P, 2) === "pr-cur pr-in pr-back" && cls(P, 3) === "pr-out pr-back" && curOk(P), { c2: cls(P, 2), c3: cls(P, 3) });
    runTimers();
    ok(t + " 연달아: 앞 넘김의 정리 타이머가 지금 장을 숨기지 않는다", vis(P) === "2" && curOk(P), vis(P));
    // 장마다 정리 타이머 하나: A→B 뒤 B→A→B 로 다시 나가는 A 의 움직임을 첫 넘김의 타이머가 끊지 않는다
    H.jump(0); runTimers();
    H.jump(1);
    const first = [...world.timers.keys()];
    H.jump(0); H.jump(1);
    for (const id of first) { const f = world.timers.get(id); if (f) { world.timers.delete(id); f(); } }
    ok(t + " 다시 나가는 장: 지난 넘김의 정리 타이머는 지워져 지금 나가는 움직임을 끊지 않는다", cls(P, 0) === "pr-out" && vis(P) === "0,1", { c0: cls(P, 0) });
    runTimers();
    ok(t + " 다시 나가는 장: 새 타이머 뒤에는 숨는다", vis(P) === "1" && curOk(P), vis(P));
    // 검은 화면 동안 넘기고 걷기
    H.setBlack(true);
    H.next(); H.next(); H.next();
    ok(t + " 검은 화면 동안 넘김: 보이는 장은 둘 이하", P.black && vis(P).split(",").length <= 2 && curOk(P), vis(P));
    H.setBlack(false);
    runTimers();
    ok(t + " 검은 화면 걷기: 지금 장 하나만", !P.black && vis(P) === String(P.idx) && curOk(P), { v: vis(P), idx: P.idx });
    H.stop();
    // 청중 열기의 show 덮어쓰기: 같은 전환 클래스, 같은 정리 길
    const a = world();
    runTimers();
    a.H.start({ pages: pages(), notes: [], index: 0, ratio: "16:9", onExit() {}, transition: t, audience: true, onState() {} });
    ok(t + " 청중: 같은 전환 클래스", rootOf(a).classList.contains("t-" + t));
    a.H.show({ i: 3 }); a.H.show({ i: 1, step: 2 }); a.H.show({ i: 2 });
    ok(t + " 청중 show 연달아: 보이는 장은 둘 이하, 지금 장 하나", a.P.idx === 2 && vis(a.P) === "1,2" && curOk(a.P), vis(a.P));
    a.H.show({ i: 2, black: true }); a.H.show({ black: false });
    runTimers();
    ok(t + " 청중 검은 화면 들고 남: 지금 장 하나만", vis(a.P) === "2" && curOk(a.P), vis(a.P));
    a.H.stop();
  }
  // 동작 줄이기·모르는 값·없는 값은 none
  {
    const w = world(), { P, H } = w;
    w.win.matchMedia = q => ({ matches: /prefers-reduced-motion: reduce/.test(q) });
    open(w, { transition: "push" });
    ok("동작 줄이기: push 를 넘겨도 t-none", P.transition === "none" && rootOf(w).classList.contains("t-none") && !rootOf(w).classList.contains("t-push"), rootOf(w).className);
    H.stop();
    w.win.matchMedia = () => ({ matches: false });
    // 발표 중에 동작 줄이기를 켠 뒤 넘김(리뷰 1회차 [1]): present.js 는 시작 때만 읽어 루트의 t-* 와 나가는 장(pr-out)이 남는다.
    // 화면은 present.css 동작 줄이기 규칙(pr-out 숨김 — present_test.go 가 글로 본다)을 얹어 본다: 보이는 장 = pr-cur 이고 pr-out 아닌 것.
    // 그 화면이 none 과 같게 지금 장 하나여야 한다 — cover 뒤로·push 앞으로·청중 show 를 포함해 다섯 전환 모두
    const visReduce = P => P.slides.map((s, i) => (s.classList.contains("pr-cur") && !s.classList.contains("pr-out") ? i : -1)).filter(i => i >= 0).join(",");
    for (const t of ["fade", "slide", "zoom", "push", "cover"]) {
      for (const aud of [false, true]) {
        w.win.matchMedia = () => ({ matches: false });
        runTimers();
        H.start({ pages: [slide([]), slide([]), slide([])], notes: [], index: 1, ratio: "16:9", onExit() {}, transition: t, audience: aud, onState() {} });
        const c = rootOf(w).classList.contains("t-" + t);
        w.win.matchMedia = q => ({ matches: /prefers-reduced-motion: reduce/.test(q) });
        if (aud) H.show({ i: 0 }); else H.prev();
        const back = visReduce(P), backOut = P.slides[1].classList.contains("pr-out") && P.slides[1].classList.contains("pr-back");
        if (aud) H.show({ i: 1 }); else H.next();
        const fwd = visReduce(P);
        ok(t + (aud ? " 청중 show" : "") + ": 발표 중 동작 줄이기 뒤 뒤로·앞으로 넘김도 지금 장 하나만(나가는 장은 CSS 가 숨긴다)",
          c && backOut && back === "0" && fwd === "1" && P.idx === 1, { c, backOut, back, fwd });
        H.stop();
      }
    }
    w.win.matchMedia = () => ({ matches: false });
    for (const v of ["flip", "zoom pr-dock", "", undefined, 3]) {
      open(w, { transition: v });
      const c = rootOf(w).className;
      ok("모르는 전환 " + JSON.stringify(v) + ": t-none 이고 다른 클래스가 새지 않는다", P.transition === "none" && rootOf(w).classList.contains("t-none") && !rootOf(w).classList.contains("pr-dock") && !/t-(?!none)/.test(c), c);
      H.stop();
    }
  }
}

// ---- 11. 전환 중 판서 입력(itda-doc#105 — present.js settleTransition): 펜·형광펜·지우개·레이저 누름이 오면 진행 중인 전환을 곧바로 끝낸 뒤
// 좌표를 잰다. 가짜 DOM 의 getBoundingClientRect 는 고정값이라, 덱 상자를 재는 순간에 아직 움직이는 장(pr-in·pr-out)이 있었는가를 적어 본다.
// 실제 WebKit 의 화면 좌표 비교는 저장소 밖 탐침이 했다(시험에 넣지 않는다 — 외부 런타임).
{
  const w = world(), { P, H } = w;
  const moving = () => P.slides.filter(s => ["pr-in", "pr-out", "pr-back"].some(c => s.classList.contains(c))).length;
  const visible = () => P.slides.map((s, i) => (s.classList.contains("pr-cur") || s.classList.contains("pr-out") ? i : -1)).filter(i => i >= 0).join(",");
  // spy 는 덱 상자를 잴 때(toDesign) 움직이는 장의 수를 적는다
  function spy(deck) { const seen = []; deck.getBoundingClientRect = () => { seen.push(moving()); return { left: 0, top: 0, right: 1280, bottom: 720, width: 1280, height: 720 }; }; return seen; }
  // outIds 는 넘김(go) 동안 새로 걸린 타이머다 — 나가는 장 정리 타이머가 들어 있다
  function step(fn) { const before = new Set(world.timers.keys()); fn(); return [...world.timers.keys()].filter(k => !before.has(k)); }
  const pen = { pointerType: "pen", pointerId: 1 };
  for (const t of ["push", "cover", "zoom", "slide", "fade"]) {
    for (const [tk, tool] of [["p", "펜"], ["h", "형광펜"], ["e", "지우개"], ["l", "레이저"]]) {
      for (const dir of [1, -1]) {
        runTimers();
        const { deck } = open(w, { transition: t, index: dir > 0 ? 0 : 1, inkKey: "tr-" + t });
        key(w, tk, tk === "p" ? "KeyP" : tk === "h" ? "KeyH" : tk === "e" ? "KeyE" : "KeyL");
        const outIds = step(() => { if (dir > 0) H.next(); else H.prev(); });
        const was = moving(), out = P.slides[dir > 0 ? 0 : 1], cur = P.slides[P.idx];
        const seen = spy(deck);
        // cover 뒤로는 나가는 장이 위 층이다 — 그 장 위를 눌러도 같다
        const target = t === "cover" && dir < 0 ? out : cur;
        fire(target, "pointerdown", { ...pen, buttons: 1, clientX: 320, clientY: 300 });
        fire(target, "pointermove", { ...pen, buttons: 1, clientX: 360, clientY: 300 });
        const name = t + (dir > 0 ? " 앞으로" : " 뒤로") + " + " + tool + ": 전환 중 펜 누름";
        ok(name + " — 움직이는 장 클래스가 정리되고 지금 장 하나만 보인다", was === 2 && moving() === 0 && visible() === String(P.idx) && cur.classList.contains("pr-cur"), { was, now: moving(), vis: visible() });
        ok(name + " — 덱 상자를 재기 전에 전환이 끝났다", (tk === "l" || seen.length > 0) && seen.every(n => n === 0), seen);
        ok(name + " — 나가는 장 정리 타이머가 지워졌다", outIds.length > 0 && outIds.some(id => !world.timers.has(id)), { outIds, left: outIds.filter(id => world.timers.has(id)) });
        fire(target, "pointerup", { ...pen, clientX: 360, clientY: 300 });
        if (tk === "p") {
          const s = page(w, P.idx)[0];
          ok(name + " — 획 첫 점은 덱 기준 좌표다", !!s && s.pts[0] === Math.round(320 / P.scale * 10) / 10, s && s.pts);
        }
        // 뒤늦은 타이머가 돌아도 장을 건드리지 않는다
        const snap = P.slides.map(s => s.className).join("|");
        runTimers();
        ok(name + " — 뒤늦은 타이머가 장을 건드리지 않는다", P.slides.map(s => s.className).join("|") === snap && visible() === String(P.idx), { snap, now: P.slides.map(s => s.className) });
        H.stop();
      }
    }
  }
  // penOnly 의 펜 아닌 입력·도구 끔은 판서가 아니다 — 전환을 끊지 않는다
  for (const [label, o, tk, ev] of [["penOnly + 펜 도구: 손가락 누름", { penOnly: true }, "p", { pointerType: "touch", pointerId: 2 }],
    ["도구 끔: 펜 누름", {}, "", pen]]) {
    runTimers();
    open(w, Object.assign({ transition: "push", inkKey: "tr-none" }, o));
    if (tk) key(w, tk, "KeyP");
    H.next();
    fire(P.slides[1], "pointerdown", { ...ev, buttons: 1, clientX: 320, clientY: 300 });
    ok(label + "은 전환을 끊지 않는다", P.slides[1].classList.contains("pr-in") && P.slides[0].classList.contains("pr-out"), P.slides.map(s => s.className));
    fire(P.slides[1], "pointerup", { ...ev, clientX: 320, clientY: 300 });
    H.stop();
  }
  // 청중 열기에는 판서가 없다 — 전환은 그대로다
  runTimers();
  H.start({ pages: [slide([]), slide([]), slide([])], notes: [], index: 0, ratio: "16:9", onExit() {}, transition: "push", audience: true, onState() {} });
  H.show({ i: 1 });
  fire(P.slides[1], "pointerdown", { ...pen, buttons: 1, clientX: 320, clientY: 300 });
  ok("청중 열기: 판서가 없고 펜 누름이 전환을 끊지 않는다", H.ink() === null && P.slides[1].classList.contains("pr-in") && P.slides[0].classList.contains("pr-out"), P.slides.map(s => s.className));
  H.stop();
}

// ---- 12. 판서 받아 그리기(itda-doc#108 — ink.js view·present.js 머리 주석 「판서 받기」): relay 묶음·레이저를 받아 다른 표면의 장 위에
// 발표자와 같은 층·경로로 그린다. 그리기는 rAF 에 모아 한 번이다. 가짜 DOM 은 그리지 않으므로 층·경로 속성·canvas 명령을 본다(실제 그림은 저장소 밖 탐침).
const runRaf = w => { const r = w.raf.splice(0); r.forEach(g => g()); return r.length; };
const layerKids = (pg, cls) => { const l = pg.kids.find(k => k.classList.contains(cls)); return l ? l.kids : []; };
// inkOf 는 장의 끝난 획 SVG 를 [층, d, fill, stroke, 굵기] 로 적는다 — 두 표면의 모양을 견준다.
const inkOf = pg => ["ink-hl", "ink-pen"].flatMap(c => layerKids(pg, c).map(p => [c, p.attrs.d, p.attrs.fill, p.attrs.stroke || "", p.attrs["stroke-width"] || ""].join(" ")));
const liveOf = pg => pg.kids.find(k => k.classList.contains("ink-live"));
const spotOf = pg => pg.kids.find(k => k.classList.contains("ink-laser-page"));
const B = (k, ops) => JSON.stringify({ k, ops });
const inkKids = pg => pg.kids.filter(k => /(^| )ink-/.test(k.className)).length;
{
  const w = world(), { I } = w, pages = [slide([]), slide([]), slide([])];
  let cur = 0, black = false;
  const v = I.view();
  const at = { slides: () => pages, size: [1280, 720], scale: () => 2, idx: () => cur, paused: () => black };
  // 붙이기 전에 온 연산은 모델에 남는다
  v.apply(B("reset", [["s", 0, 1, "pen", 0, 1, [10, 10, 50, 10, 90, 30], 0, 1]]));
  ok("view: 붙이기 전에는 층을 만들지 않고 모델에 남긴다", inkKids(pages[0]) === 0 && v.ids(0).join() === "1" && w.raf.length === 0, inkKids(pages[0]));
  v.mount(at);
  runRaf(w);
  const s1 = { tool: "pen", color: 0, width: 1, pts: [10, 10, 50, 10, 90, 30], press: null };
  ok("view: 붙이면 받아 둔 획을 그 장 .ink-pen 에 발표자와 같은 경로로 그린다", inkOf(pages[0]).join() === "ink-pen " + I.pathOf(s1) + " none #e5342b 5", inkOf(pages[0]));
  // 진행 중인 획: 묶음 여러 개 → 프레임 한 번
  v.apply(B("", [["b", 0, 2, "pen", 4, 0, 0], ["a", 2, [100, 100, 110, 100]]]));
  v.apply(B("", [["a", 2, [120, 104]]]));
  v.apply(B("", [["a", 2, [130, 110]]]));
  ok("view: 묶음이 여럿 와도 프레임 요청은 하나다", w.raf.length === 1, w.raf.length);
  const live0 = liveOf(pages[0]);
  ok("view: 프레임 전에는 진행 중 획을 그리지 않는다", !live0, !!live0);
  runRaf(w);
  const live = liveOf(pages[0]), s2 = { tool: "pen", color: 4, width: 0, pts: [100, 100, 110, 100, 120, 104, 130, 110], press: null };
  ok("view: 진행 중 획은 그 장의 .ink-live canvas 에 발표자 pathOf 경로로 그린다", live && live.ctx.paints.length === 1 && live.ctx.paints[0].d === I.pathOf(s2) && live.ctx.paints[0].color === "#1f6fe0" && live.ctx.paints[0].width === 2.5, live && live.ctx.paints);
  ok("view: canvas 뒷버퍼는 설계×배율×dpr 이다", live.width === 1280 * 2 * 2 && live.height === 720 * 2 * 2, [live.width, live.height]);
  ok("view: 진행 중 획은 SVG 에 아직 없다", inkOf(pages[0]).length === 1, inkOf(pages[0]).length);
  // 같은 id 의 b 가 다시 와도 획을 새로 만들지 않는다(a 는 순서·한 번 전달을 전제한다 — 다시 온 a 는 겹친다, 머리 주석 「전달 계약」)
  ok("view: 같은 b 가 다시 와도 획이 늘지 않는다", v.apply(B("", [["b", 0, 2, "pen", 4, 0, 0]])) === true && v.ids(0).join() === "1,2", v.ids(0));
  runRaf(w);
  ok("view: 다시 온 b 는 진행 중 획을 비우지 않는다", live.ctx.paints.length === 1 && live.ctx.paints[0].d === I.pathOf(s2), live.ctx.paints);
  v.apply(B("", [["e", 2]]));
  ok("view: 끝(e)도 프레임에 그린다", inkOf(pages[0]).length === 1);
  runRaf(w);
  ok("view: 끝난 획은 같은 프레임에 SVG 로 옮기고 canvas 를 지운다", inkOf(pages[0]).length === 2 && inkOf(pages[0])[1] === "ink-pen " + I.pathOf(s2) + " none #1f6fe0 2.5" && live.ctx.paints.length === 0, { svg: inkOf(pages[0]), live: live.ctx.paints });
  v.apply(B("", [["e", 2], ["b", 0, 1, "hl", 2, 0, 0]]));
  runRaf(w);
  ok("view: 끝난 획의 e·b 가 다시 와도 겹쳐 그리지 않는다", inkOf(pages[0]).length === 2 && v.ids(0).join() === "1,2", { svg: inkOf(pages[0]), ids: v.ids(0) });
  // 형광펜: 층·혼합 클래스, 어두운 바탕
  w.win.getComputedStyle = e => ({ backgroundColor: e === pages[1] ? "rgb(5, 20, 31)" : "rgb(255, 255, 255)", width: "0px", height: "0px" });
  v.apply(B("", [["b", 1, 3, "hl", 2, 1, 0], ["a", 3, [200, 200, 260, 200]]]));
  runRaf(w);
  const live1 = liveOf(pages[1]);
  ok("view: 진행 중 형광펜은 canvas 가 그 장으로 옮겨 ink-live-hl·어두운 바탕 ink-dark 를 단다", live1 === live && live.className === "ink-live ink-live-hl ink-dark" && !liveOf(pages[0]), live.className);
  v.apply(B("", [["a", 3, [300, 210]], ["e", 3]]));
  runRaf(w);
  const hl1 = pages[1].kids.find(k => k.classList.contains("ink-hl"));
  ok("view: 끝난 형광펜은 .ink-hl 층(어두운 바탕이면 ink-dark)이다", hl1 && hl1.className === "ink-hl ink-dark" && inkOf(pages[1]).length === 1 && inkOf(pages[1])[0].startsWith("ink-hl "), { cls: hl1 && hl1.className, svg: inkOf(pages[1]) });
  ok("view: 밝은 바탕의 형광펜 층은 ink-dark 가 없다", pages[0].kids.find(k => k.classList.contains("ink-hl")).className === "ink-hl");
  // 필압 획
  v.apply(B("", [["b", 0, 4, "pen", 6, 1, 1], ["a", 4, [400, 400, 420, 410], [0.3, 0.8]], ["e", 4]]));
  runRaf(w);
  const s4 = { tool: "pen", color: 6, width: 1, pts: [400, 400, 420, 410], press: [0.3, 0.8] };
  ok("view: 필압 획은 채운 조각 경로다(발표자와 같다)", inkOf(pages[0])[2] === "ink-pen " + I.pathOf(s4) + " #111111 none ", inkOf(pages[0])[2]);
  // 삭제·복구·장 지우기
  v.apply(B("", [["d", 0, [2]]]));
  runRaf(w);
  ok("view: d 는 그 장 목록에서 뺀다", v.ids(0).join() === "1,4" && inkOf(pages[0]).length === 2, v.ids(0));
  v.apply(B("", [["r", 0, [1, 2, 4]]]));
  runRaf(w);
  ok("view: r 은 지운 획을 id 로 되살려 그 차례로 그린다", v.ids(0).join() === "1,2,4" && inkOf(pages[0])[1] === "ink-pen " + I.pathOf(s2) + " none #1f6fe0 2.5", inkOf(pages[0]));
  ok("view: r 은 겹친 id·다른 장의 id 를 뺀다(신호 없음)", v.apply(B("", [["r", 0, [1, 2, 4, 4, 3]]])) === true && v.ids(0).join() === "1,2,4", v.ids(0));
  v.apply(B("clear", [["c", 0]]));
  v.apply(B("clear", [["c", 0]]));
  runRaf(w);
  ok("view: c 는 그 장만 비운다(두 번 와도 같다)", v.ids(0).length === 0 && inkOf(pages[0]).length === 0 && inkOf(pages[1]).length === 1);
  v.apply(B("", [["r", 0, [1, 2, 4]]]));
  runRaf(w);
  ok("view: 전부 지우기 뒤 r(되돌리기)로 다시 보인다", inkOf(pages[0]).length === 3);
  // 그리다 취소(진행 중 획의 d)는 버린다 — 뒤의 a 는 버린다
  v.apply(B("", [["b", 0, 5, "pen", 0, 0, 0], ["a", 5, [1, 1, 30, 30]]]));
  runRaf(w);
  ok("view: 진행 중 획의 d(취소)는 신호 없이 받는다", v.apply(B("", [["d", 0, [5]]])) === true);
  runRaf(w);
  ok("view: 진행 중 획의 d(취소)는 canvas 에서 지우고 모델에서 버린다", v.ids(0).join() === "1,2,4" && live.ctx.paints.length === 0 && inkOf(pages[0]).length === 3 && v.held().live === 0, { ids: v.ids(0), live: live.ctx.paints });
  // 틀린 연산·묶음
  ok("view: 읽지 못하는 묶음은 거짓", v.apply("{") === false && v.apply({ k: "" }) === false && v.apply(B("", [])) === true);
  v.apply(B("", [["b", 0, 6, "pen", "red", 0, 0], ["b", 0, 7, "pen", 0, 9, 0], ["b", -1, 8, "pen", 0, 0, 0], ["b", 0, 9, "brush", 0, 0, 0], ["s", 0, 10, "pen", 0, 0, [-5, 1], 0, 1],
    ["s", 0, 11, "pen", 0, 0, [1, 2, 3], 0, 1], ["s", 0, 12, "pen", 0, 0, [1, 2, 3, 4], [0.5], 1], ["x", 0], "b", null]));
  ok("view: 틀린 색·굵기·장·도구·좌표·필압의 연산은 버린다", v.ids(0).join() === "1,2,4", v.ids(0));
  // 없는 장: 모델에만 남고 그 장이 생기면 그린다
  v.apply(B("", [["s", 5, 20, "pen", 0, 0, [5, 5, 9, 9], 0, 1]]));
  runRaf(w);
  ok("view: 덱에 없는 장의 판서는 모델에만 남는다", v.ids(5).join() === "20" && !pages[0].kids.some(k => k.attrs.d), v.ids(5));
  // 레이저
  v.laser(0, 300, 200, false);
  runRaf(w);
  const sp = spotOf(pages[0]);
  ok("view: 레이저는 지금 장의 .ink-laser-page canvas 에 점(두 원)을 그린다", sp && sp.ctx.paints.filter(p => p.op === "fill").length === 2, sp && sp.ctx.paints);
  v.laser(0, 320, 210, true);
  runRaf(w);
  ok("view: 레이저가 움직이면 꼬리 선(끈 선 굵기 4 를 배율로 나눈 값)을 그린다", sp.ctx.paints.some(p => p.op === "stroke" && p.width === 2), sp.ctx.paints);
  cur = 1; v.wake(); runRaf(w);
  ok("view: 청중 화면이 다른 장이면 레이저를 지운다", sp.ctx.paints.length === 0, sp.ctx.paints);
  cur = 0; black = true; v.wake(); runRaf(w);
  ok("view: 쉬는 화면(검은 화면)이면 레이저를 지운다", sp.ctx.paints.length === 0);
  black = false; v.laser(-1, 0, 0, false); runRaf(w);
  ok("view: 꺼진 레이저(i = -1)는 점을 그리지 않는다", !sp.ctx.paints.some(p => p.op === "fill"), sp.ctx.paints);
  // reset: 전체를 바꿔 그린다 — 진행 중 획도 이어 받는다
  v.apply(B("reset", [["s", 2, 30, "pen", 1, 2, [10, 10, 20, 20], 0, 1], ["s", 0, 31, "hl", 3, 0, [5, 5, 15, 5], 0, 0]]));
  runRaf(w);
  ok("view: reset 은 앞의 판서를 모두 버리고 새 상태로 그린다", v.ids(0).join() === "31" && v.ids(1).length === 0 && v.ids(5).length === 0 && inkOf(pages[0]).length === 0 && inkOf(pages[1]).length === 0 && inkOf(pages[2]).length === 1, { a: inkOf(pages[0]), b: inkOf(pages[1]), c: inkOf(pages[2]) });
  ok("view: reset 의 진행 중 획(끝남 0)은 canvas 로 그린다", liveOf(pages[0]) === live && live.ctx.paints.length === 1 && live.className === "ink-live ink-live-hl", { cls: live.className, p: live.ctx.paints });
  v.apply(B("", [["a", 31, [25, 5]], ["e", 31]]));
  runRaf(w);
  ok("view: reset 뒤 이어 온 a·e 로 그 획을 마친다", inkOf(pages[0]).length === 1 && inkOf(pages[0])[0] === "ink-hl " + I.pathOf({ tool: "hl", color: 3, width: 0, pts: [5, 5, 15, 5, 25, 5], press: null }) + " none #2fa84f 14", inkOf(pages[0]));
  // 끝난 획이 그 장 목록의 맨 끝이 아니면(reset 이 진행 중 획 뒤에 끝난 획을 실었다) 그 장을 다시 맞춰 차례를 지킨다
  v.apply(B("reset", [["s", 0, 50, "pen", 0, 0, [1, 1, 9, 9], 0, 0], ["s", 0, 51, "pen", 1, 0, [2, 2, 8, 8], 0, 1]]));
  runRaf(w);
  v.apply(B("", [["e", 50]]));
  runRaf(w);
  ok("view: 맨 끝이 아닌 획이 끝나면 목록 차례로 다시 그린다", inkOf(pages[0]).map(x => x.match(/#\w+/)[0]).join() === "#e5342b,#f28c1e", inkOf(pages[0]));
  // 배율이 바뀌면(resize) canvas 뒷버퍼를 맞춘다
  let k2 = 2;
  v.mount(Object.assign({}, at, { scale: () => k2 }));
  v.apply(B("", [["b", 0, 52, "pen", 0, 0, 0], ["a", 52, [3, 3, 30, 3]]]));
  runRaf(w);
  k2 = 3; v.resize();
  ok("view: resize 는 진행 중 canvas 를 새 배율로 다시 맞춘다", live.width === 1280 * 3 * 2 && live.ctx.paints.length === 1, [live.width, live.ctx.paints.length]);
  v.apply(B("", [["e", 52]]));
  runRaf(w);
  // 떼기·다시 붙이기
  v.unmount();
  ok("view: 떼면 층·canvas 를 장에서 뗀다", pages.every(p => inkKids(p) === 0), pages.map(inkKids));
  v.apply(B("", [["s", 1, 40, "pen", 0, 0, [1, 1, 2, 2], 0, 1]]));
  ok("view: 뗀 동안에는 프레임을 요청하지 않는다", w.raf.length === 0, w.raf.length);
  v.mount(at);
  runRaf(w);
  ok("view: 다시 붙이면 모든 장을 다시 그린다", inkOf(pages[0]).length === 3 && inkOf(pages[1]).length === 1 && inkOf(pages[2]).length === 0, pages.map(p => inkOf(p).length));
  v.reset();
  runRaf(w);
  ok("view: reset() 은 모두 비운다", pages.every(p => inkOf(p).length === 0) && v.ids(0).length === 0);
}
// present.js 청중 열기: inkView 를 덱에 붙이고, 장 넘김·검은 화면·닫기·다시 열기를 따른다
{
  const w = world(), { P, H } = w;
  const v = H.inkView();
  ok("청중: inkView 는 하나다(발표 밖에서도 받는다)", v && H.inkView() === v && v.mounted() === false);
  v.apply(B("reset", [["s", 0, 1, "pen", 0, 1, [10, 10, 50, 50], 0, 1], ["s", 1, 2, "hl", 2, 1, [10, 10, 90, 10], 0, 1]]));
  H.start({ pages: [slide([]), slide([]), slide([])], notes: [], index: 0, ratio: "16:9", onExit() {}, audience: true, onState() {} });
  runRaf(w);
  ok("청중: 열기 전에 받은 판서를 덱의 장에 그린다", v.mounted() && inkOf(P.slides[0]).length === 1 && inkOf(P.slides[1]).length === 1 && H.ink() === null, [inkOf(P.slides[0]), inkOf(P.slides[1])]);
  v.laser(1, 100, 100, false);
  runRaf(w);
  ok("청중: 레이저의 장이 지금 장이 아니면 그리지 않는다", !spotOf(P.slides[1]) && !spotOf(P.slides[0]));
  H.show({ i: 1 });
  runRaf(w);
  ok("청중: 그 장으로 넘기면(show) 레이저가 보인다", spotOf(P.slides[1]) && spotOf(P.slides[1]).ctx.paints.filter(p => p.op === "fill").length === 2);
  ok("청중: 넘긴 장의 판서는 그 장에 남는다", inkOf(P.slides[0]).length === 1 && inkOf(P.slides[1]).length === 1);
  H.show({ black: true });
  runRaf(w);
  ok("청중: 검은 화면이면 레이저를 지운다", spotOf(P.slides[1]).ctx.paints.length === 0);
  H.show({ black: false, i: 0 });
  v.apply(B("", [["b", 0, 3, "pen", 0, 0, 0], ["a", 3, [200, 200, 210, 220]]]));
  runRaf(w);
  ok("청중: 진행 중 획을 그 장 canvas 에 그린다", liveOf(P.slides[0]) && liveOf(P.slides[0]).ctx.paints.length === 1);
  const stage = rootOf(w).querySelector(".pr-stage");
  stage.clientWidth = 2560; stage.clientHeight = 1440;
  for (const f of w.win.on.resize || []) f();
  ok("청중: 창 크기가 바뀌면(fit) canvas 뒷버퍼를 새 배율에 맞춘다", P.scale === 2 && liveOf(P.slides[0]).width === 1280 * 2 * 2, [P.scale, liveOf(P.slides[0]).width]);
  const old = P.slides.slice();
  H.stop();
  ok("청중: 닫으면 층을 떼고 모델은 남긴다", !v.mounted() && old.every(s => !s.kids.some(k => /ink-/.test(k.className))) && v.ids(0).join() === "1,3");
  v.apply(B("", [["e", 3]]));
  H.start({ pages: [slide([]), slide([]), slide([])], notes: [], index: 1, ratio: "16:9", onExit() {}, audience: true, onState() {} });
  runRaf(w);
  ok("청중: 다시 열면 닫은 동안 끝난 획까지 그린다", inkOf(P.slides[0]).length === 2 && inkOf(P.slides[1]).length === 1, inkOf(P.slides[0]));
  H.stop();
  open(w);
  runRaf(w);
  ok("청중: 발표자 열기에는 inkView 를 붙이지 않는다(받은 판서를 발표자 장에 그리지 않는다)", !v.mounted() && P.slides.every(s => inkOf(s).length === 0));
  H.stop();
}
// 받는 경계(itda-doc#108 1회차 리뷰 [1]·[2]·[4]·[5]): 큰 묶음·상한·지운 획 예산·레이저 꼬리. 상한에 닿으면 조용히 버리지 않고 apply 가 "reset" 을 돌려준다.
{
  const w = world(), { I } = w;
  const pts = n => { const a = new Array(n * 2); for (let j = 0; j < n; j++) { a[2 * j] = j % 1000; a[2 * j + 1] = Math.floor(j / 1000) % 700; } return a; };
  const mountOn = (v, pg) => { v.mount({ slides: () => [pg], size: [1280, 720], scale: () => 1, idx: () => 0, paused: () => false }); v.flush(); };
  // [1] 상한 안의 큰 a(20만 점)·필압 a 는 예외 없이 받는다
  {
    const v = I.view(), pg = slide([]);
    let r, err = null;
    try { r = v.apply({ k: "", ops: [["b", 0, 1, "pen", 0, 0, 0], ["a", 1, pts(I.LIMIT.points)], ["e", 1]] }); } catch (e) { err = String(e); }
    mountOn(v, pg);
    ok("경계[1]: 20만 점 a 는 예외 없이 받고 뒤의 e·그리기까지 간다", err === null && r === true && v.held().points === I.LIMIT.points && inkOf(pg).length === 1, { err, r, held: v.held() });
    const press = new Array(1000).fill(0.5);
    let r2;
    err = null;
    try { r2 = v.apply({ k: "", ops: [["b", 0, 2, "pen", 0, 0, 1], ["a", 2, pts(1000), press], ["e", 2]] }); } catch (e) { err = String(e); }
    ok("경계[1]: 필압 a 도 펼치기 없이 붙인다", err === null && r2 === true && v.ids(0).join() === "1,2" && v.held().points === I.LIMIT.points + 1000, { err, r2, ids: v.ids(0) });
  }
  {
    const v = I.view();
    v.apply(B("", [["s", 0, 1, "pen", 0, 0, [1, 1], 0, 1]]));
    // 입력 예산(2회차 리뷰 [2]): 연산 수 + 연산 안 배열 원소 수 합계 하나(LIMIT.points×4 = 80만)
    const r = v.apply({ k: "", ops: [["s", 0, 2, "pen", 0, 0, [2, 2], 0, 1], ["b", 0, 3, "pen", 0, 0, 0], ["a", 3, pts(I.LIMIT.points * 2)]] });
    ok("예산: 원소 합계가 예산을 넘으면 그 묶음을 하나도 적용하지 않고 \"reset\"", r === "reset" && v.ids(0).join() === "1" && v.needsReset(), { r, ids: v.ids(0) });
    ok("예산: 다시 맞춤 전까지는 다음 묶음도 \"reset\"", v.apply(B("", [["c", 0]])) === "reset");
    ok("예산: reset 묶음을 받으면 신호가 풀린다", v.apply(B("reset", [["s", 0, 9, "pen", 0, 0, [3, 3], 0, 1]])) === true && !v.needsReset() && v.ids(0).join() === "9");
    const many = Array.from({ length: 270000 }, () => ["c", 0]);
    ok("예산: 작은 연산도 연산 수가 예산에 들어간다", v.apply({ k: "", ops: many }) === "reset" && v.ids(0).join() === "9");
    v.apply(B("reset", [["s", 0, 1, "pen", 0, 0, [1, 1], 0, 1]]));
    const ids1M = new Array(1000000).fill(1);
    ok("예산: 100만 id 의 r 은 모델을 바꾸기 전에 \"reset\"", v.apply({ k: "", ops: [["c", 0], ["r", 0, ids1M]] }) === "reset" && v.ids(0).join() === "1", v.ids(0));
    v.apply(B("reset", [["s", 0, 1, "pen", 0, 0, [1, 1], 0, 1]]));
    ok("예산: 100만 id 의 d 도 같다(서로 다른 id)", v.apply({ k: "", ops: [["d", 0, Array.from({ length: 1000000 }, (_, i) => i)]] }) === "reset" && v.ids(0).join() === "1");
    v.apply(B("reset", [["s", 0, 1, "pen", 0, 0, [1, 1], 0, 1]]));
    ok("예산: 칸이 아주 많은 연산도 같다", v.apply({ k: "", ops: [new Array(900000).fill(0)] }) === "reset");
    v.apply(B("reset", [["s", 0, 1, "pen", 0, 0, [1, 1], 0, 1]]));
    const huge = JSON.stringify({ k: "", ops: [["c", 0]] }) + " ".repeat(8 << 20);
    ok("예산: 글자 수 상한(8Mi)을 넘는 문자열은 읽기 전에 \"reset\"", v.apply(huge) === "reset" && v.ids(0).join() === "1");
    // 발표자 상태 전체(2천 획·20만 점·필압)를 담은 reset 은 예산 안이다
    const full = [];
    for (let i = 0; i < I.LIMIT.strokes; i++) full.push(["s", 0, i + 1, "pen", 0, 0, pts(100), new Array(100).fill(0.5), 1]);
    const fullJ = JSON.stringify({ k: "reset", ops: full });
    ok("예산: 발표자 상한 꽉 찬 reset(필압 포함)은 받는다", fullJ.length <= 8 << 20 && v.apply(fullJ) === true && v.ids(0).length === I.LIMIT.strokes && v.held().points === I.LIMIT.points, { len: fullJ.length, held: v.held() });
  }
  // [2] 점 없는 b·빈 획·보관 획 수 상한
  {
    const v = I.view();
    const r = v.apply({ k: "", ops: Array.from({ length: 20000 }, (_, i) => ["b", 0, i, "pen", 0, 0, 0]) });
    ok("경계[2]: 진행 중 획은 상한(4)까지만 받고 넘으면 \"reset\"", r === "reset" && v.held().live === 4 && v.ids(0).length === 4, { r, held: v.held() });
    v.apply(B("reset", []));
    let all = true;
    for (let i = 0; i < 3000; i++) all = v.apply(B("", [["b", 0, 100 + i, "pen", 0, 0, 0], ["e", 100 + i]])) === true && all;
    ok("경계[2]: 점 없이 끝난 획(b·e)은 e 에서 버려 쌓이지 않는다", all && v.held().strokes === 0 && v.ids(0).length === 0, v.held());
    const sOps = from => Array.from({ length: 2001 }, (_, i) => ["s", 0, from + i, "pen", 0, 0, [1, 1], 0, 1]);
    const r1 = v.apply({ k: "", ops: sOps(10000) }), r2 = v.apply({ k: "", ops: sOps(20000) });
    ok("경계[2]: 보관 획 수는 상한(4천)을 넘지 않고, 넘치면 \"reset\"", r1 === true && r2 === "reset" && v.held().strokes === I.LIMIT.strokes * 2, { r1, r2, held: v.held() });
    v.apply(B("reset", []));
    const rs = v.apply({ k: "reset", ops: Array.from({ length: 5 }, (_, i) => ["s", 0, i, "pen", 0, 0, [1, 1], 0, 0]) });
    ok("경계[2]: s 의 진행 중 획도 같은 상한이다", rs === "reset" && v.held().live === 4, { rs, held: v.held() });
  }
  // [4] 그리기·지우기를 되풀이해도 새 획이 보인다 — 지운 획을 오래된 것부터 버려 자리를 만든다. 버린 획을 r 이 가리키면 "reset"
  {
    const v = I.view(), pg = slide([]), rets = [];
    for (let i = 1; i <= 4; i++) {
      rets.push(v.apply(B("", [["b", 0, i, "pen", 0, 0, 0]])));
      for (let j = 0; j < 20; j++) rets.push(v.apply({ k: "", ops: [["a", i, pts(10000)]] }));
      rets.push(v.apply(B("clear", [["e", i], ["c", 0]])));
    }
    const r = v.apply(B("", [["b", 0, 5, "pen", 0, 0, 0], ["a", 5, [10, 10, 20, 20]], ["e", 5]]));
    mountOn(v, pg);
    ok("경계[4]: 20만 점 그리기·지우기 네 번 뒤에도 새 획을 받고 보인다", rets.every(x => x === true) && r === true && v.ids(0).join() === "5" && inkOf(pg).length === 1 && v.held().points <= I.LIMIT.points * 2, { bad: rets.filter(x => x !== true).length, r, held: v.held(), svg: inkOf(pg).length });
    ok("경계[4]: 남은 지운 획은 r 로 되살아난다", v.apply(B("", [["r", 0, [4, 5]]])) === true && v.ids(0).join() === "4,5");
    ok("경계[4]: 버린 지운 획을 r 이 가리키면 \"reset\"(남는 획만 그린다)", v.apply(B("", [["r", 0, [1, 4, 5]]])) === "reset" && v.ids(0).join() === "4,5", v.ids(0));
    ok("경계[4]: 모르는 id 의 a·e 도 \"reset\"", (v.apply(B("reset", [])), v.apply(B("", [["a", 77, [1, 1]]])) === "reset") && (v.apply(B("reset", [])), v.apply(B("", [["e", 77]])) === "reset"));
    v.apply(B("reset", []));
    // 지운 획이 없는데 보관 점이 모자라면(목록에 남은 획만으로 넘친다) 그 a 를 버리고 "reset"
    v.apply(B("", [["s", 0, 1, "pen", 0, 0, pts(150000), 0, 1]]));
    v.apply(B("", [["s", 0, 2, "pen", 0, 0, pts(150000), 0, 1]]));
    const rr = v.apply(B("", [["b", 0, 3, "pen", 0, 0, 0], ["a", 3, pts(150000)]]));
    ok("경계[4]: 지울 획이 없어 모자라면 그 a 를 버리고 \"reset\"", rr === "reset" && v.held().points === 300000, { rr, held: v.held() });
  }
  // [5] 레이저 꼬리: 받을 때 만료·개수 상한, 붙지 않은 동안은 마지막 위치만, reset·떼기가 꼬리를 비운다
  {
    const v = I.view(), pg = slide([]);
    for (let i = 0; i < 1000; i++) { world.clock += 10; v.laser(0, i % 500, 10, false); }
    ok("경계[5]: 붙지 않은 동안은 꼬리를 쌓지 않는다", v.held().trail === 0, v.held());
    v.mount({ slides: () => [pg], size: [1280, 720], scale: () => 1, idx: () => 0, paused: () => false });
    for (let i = 0; i < 1000; i++) { world.clock += 10; v.laser(0, i % 500, 10, false); }
    ok("경계[5]: 붙은 동안도 받을 때 수명 지난 점을 뺀다(꼬리 0.3초)", v.held().trail <= 31, v.held());
    for (let i = 0; i < 1000; i++) v.laser(0, i % 500, 10, true);
    ok("경계[5]: 같은 순간에 많이 와도 개수 상한(240)을 넘지 않는다", v.held().trail === 240, v.held());
    v.reset();
    ok("경계[5]: reset() 은 꼬리를 비운다", v.held().trail === 0);
    v.laser(0, 1, 1, true);
    v.unmount();
    ok("경계[5]: 떼면 꼬리를 비운다", v.held().trail === 0);
    world.clock = 0;
    w.raf.length = 0;
  }
}
// ---- 13. 발표자 보조(itda-hyve-ios#24 — present.js 머리 주석 「발표자 보조」): 다음 장 미리보기(nextPreview·setNextPreview)·현재 시각(clock·setClock)·
// onState 의 next. 기본 꺼짐, 다음 장이 바뀔 때만 사본을 바꾸고(연달아 넘김에도 곧바로), 사본은 판서 층·입력 부품·id·움직임을 떼며, 청중·보드에는 없다.
const marked = (n, kids, steps) => { const p = slide(kids || [], steps); p.dataset.n = String(n); return p; };
const nextBoxOf = w => rootOf(w).querySelector(".pr-next-box");
const nextShown = w => { const b = nextBoxOf(w), c = b && b.firstElementChild; return !c ? "비었음" : c.classList.contains("page") ? "장" + c.dataset.n : c.textContent; };
{
  const w = world(), { P, H } = w;
  H.start({ pages: [marked(0), marked(1), marked(2)], notes: [], index: 0, ratio: "16:9", onExit() {} });
  const root = rootOf(w);
  ok("보조: 기본은 미리보기·시각 꺼짐(칸은 있으나 비고 클래스가 없다)", !P.nextPreview && !P.clock && !root.classList.contains("pr-next-on") && !root.classList.contains("pr-clock-on")
    && nextShown(w) === "비었음" && root.querySelector(".pr-clock").textContent === "", { n: nextShown(w) });
  H.next();
  ok("보조: 꺼진 동안 넘겨도 사본을 만들지 않는다", nextShown(w) === "비었음", nextShown(w));
  ok("보조: setNextPreview(true) 는 발표 중 켜고 지금의 다음 장을 담는다", H.setNextPreview(true) === true && root.classList.contains("pr-next-on") && nextShown(w) === "장2", nextShown(w));
  ok("보조: setClock(true) 는 HH:mm 을 곧바로 적는다", H.setClock(true) === true && root.classList.contains("pr-clock-on") && /^\d\d:\d\d$/.test(root.querySelector(".pr-clock").textContent), root.querySelector(".pr-clock").textContent);
  H.setNextPreview(false); H.setClock(false);
  ok("보조: 끄면 클래스를 떼고 사본·시각 글을 비운다", !root.classList.contains("pr-next-on") && !root.classList.contains("pr-clock-on") && nextShown(w) === "비었음" && root.querySelector(".pr-clock").textContent === "", nextShown(w));
  H.stop();
  ok("보조: 발표 밖에서는 setNextPreview·setClock 이 false", H.setNextPreview(true) === false && H.setClock(true) === false && !P.nextPreview && !P.clock);
}
{
  const w = world(), { P, H } = w, states = [];
  H.start({ pages: [marked(0), marked(1, [stepped(1), stepped(2)], 2), marked(2), marked(3)], notes: [], index: 0, ratio: "16:9", onExit() {},
    nextPreview: true, clock: true, onState: s => states.push(s) });
  const root = rootOf(w), last = () => states[states.length - 1];
  ok("보조: start 옵션으로 켜면 첫 장의 다음 장(1)을 담고 시각을 적는다", P.nextPreview && P.clock && root.classList.contains("pr-next-on") && root.classList.contains("pr-clock-on")
    && nextShown(w) === "장1" && /^\d\d:\d\d$/.test(root.querySelector(".pr-clock").textContent), { n: nextShown(w), c: root.querySelector(".pr-clock").textContent });
  ok("보조: 미리보기 칸은 노트 칸 안 머리 다음·노트 위이고 aria-hidden·inert 다", (() => { const n = root.querySelector(".pr-notes"), k = n.kids.map(x => x.className); const e = root.querySelector(".pr-next");
    return k.indexOf("pr-next") === 1 && k.indexOf("pr-notes-body") === 2 && e.attrs["aria-hidden"] === "true" && "inert" in e.attrs && e.inert === true; })(), root.querySelector(".pr-notes").kids.map(x => x.className));
  ok("onState next: 첫 상태의 next 는 1", last().next === 1, last());
  H.next();
  const c1 = nextBoxOf(w).firstElementChild;
  ok("보조: 단계가 있는 장(1)으로 가면 다음 장(2)", nextShown(w) === "장2" && last().i === 1 && last().next === 2, { n: nextShown(w), s: last() });
  H.next(); H.next();
  ok("보조: 장 안 단계를 넘겨도 「다음 단계」 가 아니라 다음 장이고 사본을 다시 만들지 않는다", P.idx === 1 && P.step === 2 && nextShown(w) === "장2" && nextBoxOf(w).firstElementChild === c1 && last().next === 2, { idx: P.idx, step: P.step, same: nextBoxOf(w).firstElementChild === c1 });
  H.jump(3);
  ok("보조: 마지막 장이면 「끝」, onState next 는 -1", nextShown(w) === "끝" && nextBoxOf(w).classList.contains("pr-next-end") && last().next === -1, { n: nextShown(w), s: last() });
  // 빠른 연달아 넘김: 넘김마다 곧바로(동기) 맞는다 — 늦게 따라와 앞 장의 다음 장을 보이지 않는다
  const seen = [];
  for (const f of [() => H.jump(0), () => H.next(), () => H.next(), () => H.next(), () => H.next(), () => H.prev(), () => H.jump(2), () => H.prev()]) { f(); seen.push(P.idx + ">" + nextShown(w)); }
  ok("보조: 연달아 넘겨도 넘김마다 그 장의 다음 장(타이머·프레임을 기다리지 않는다)", seen.every(x => { const [i, n] = x.split(">"); return Number(i) === 3 ? n === "끝" : n === "장" + (Number(i) + 1); }), seen);
  ok("보조: 사본은 하나뿐", nextBoxOf(w).kids.length === 1, nextBoxOf(w).kids.length);
  H.stop();
  ok("보조: 끝내면 깃발을 내리고 다음 발표는 물려받지 않는다", !P.nextPreview && !P.clock);
  open(w);
  ok("보조: 다음 발표(옵션 없음)는 꺼진 채", !P.nextPreview && !P.clock && !rootOf(w).classList.contains("pr-next-on") && nextShown(w) === "비었음", nextShown(w));
  H.stop();
}
{
  // 사본 정리: 덱 장(P.slides)에 발표 중 붙은 판서 층·섹션 묻기 단추·코드 복사 단추·id·차트 재생·단계 표식이 사본에 남지 않고, 덱 장은 그대로다
  const w = world(), { P, H } = w;
  const btn = new El("button"), a = new El("a"); a.setAttribute("href", "https://example.com/");
  H.start({ pages: [marked(0), marked(1, [stepped(1), chartFig(), btn, a], 1), marked(2)], notes: [], index: 2, ratio: "16:9", onExit() {}, nextPreview: true });
  const s1 = P.slides[1];
  const layer = c => { const e = new El("svg"); e.className = c; return e; };
  for (const c of ["ink-hl", "ink-pen", "ink-live", "ink-laser-page", "itda-section-ask", "itda-codecopy"]) s1.appendChild(layer(c));
  const body = s1.querySelector(".page-body"), tagged = body.appendChild(new El("div")); tagged.setAttribute("id", "dup");
  body.querySelector("figure").classList.add("chart-play");
  body.querySelector("p").classList.add("pr-pending");
  H.jump(0);
  const c = nextBoxOf(w).firstElementChild;
  const left = [".ink-hl", ".ink-pen", ".ink-live", ".ink-laser-page", ".itda-section-ask", ".itda-codecopy", "[id]", ".chart-play", ".pr-pending", ".pr-shown"].filter(s => c.querySelector(s));
  ok("보조 사본: 판서 층·입력 부품·id·차트 재생·단계 표식이 없다(다 나타난 모습)", nextShown(w) === "장1" && left.length === 0 && c.classList.contains("pr-static") && !c.classList.contains("pr-cur"), left);
  ok("보조 사본: 단추·링크는 포커스 차례에서 빠진다", c.querySelector("button").attrs.tabindex === "-1" && c.querySelector("a").attrs.tabindex === "-1");
  ok("보조 사본: 덱의 장은 건드리지 않는다", !!s1.querySelector(".ink-pen") && !!s1.querySelector("[id]") && !s1.querySelector("button").attrs.tabindex, Object.keys(s1.querySelector("button").attrs));
  // 크기: 노트 칸이 숨어 있으면 맞추지 않고, 보이면 칸 너비에 맞춰 줄인다(가짜 DOM 의 clientWidth 를 칸 너비로 쓴다)
  ok("보조 크기: 노트를 접은 동안은 배율을 걸지 않는다", !c.style.transform, c.style.transform);
  nextBoxOf(w).clientWidth = 320;
  H.setNotes(true);
  ok("보조 크기: 노트를 펴면 칸 너비에 맞춘다(320/1280 — 높이 180px·scale(0.25))", nextBoxOf(w).style.height === "180px" && c.style.transform === "scale(0.25)", { h: nextBoxOf(w).style.height, t: c.style.transform });
  nextBoxOf(w).clientWidth = 640;
  H.setDock(true);
  ok("보조 크기: 칸 너비가 바뀌면(relayout) 다시 맞춘다", nextBoxOf(w).style.height === "360px" && c.style.transform === "scale(0.5)", { h: nextBoxOf(w).style.height, t: c.style.transform });
  H.next(); // 장 1 — 다음 장 2 의 새 사본도 지금 칸 너비로 곧바로 맞춘다
  ok("보조 크기: 새 사본도 곧바로 맞춘다", nextShown(w) === "장2" && nextBoxOf(w).firstElementChild.style.transform === "scale(0.5)", nextBoxOf(w).firstElementChild.style.transform);
  H.stop();
}
{
  // 그림·표 번호(1회차 리뷰 [1]): 미리보기 상자는 독립 카운터 범위이고, 덱에서 다음 장 앞까지의 캡션 수로 시작한다(base.css 의 증가 선택자와 같은 것만 센다).
  // 가짜 DOM 은 CSS 카운터를 계산하지 않는다 — 상자의 counter-reset 글을 본다(실제 번호는 앱 WebKit E2E 몫)
  const w = world(), { P, H } = w;
  const cap = (tag, inner) => { const f = new El(tag); f.appendChild(new El(inner)); return f; };
  const fig = () => cap("figure", "figcaption"), tbl = () => cap("table", "caption");
  const s0 = marked(0, [fig(), fig(), tbl()]);
  const stray = s0.appendChild(new El("div")); stray.appendChild(new El("figcaption")); // 장 본문(.page-body) 밖 — base.css 규칙이 세지 않는다
  const hidden = fig(); hidden.dataset.step = "1"; // 아직 나타나지 않은 단계(visibility hidden)도 카운터는 오른다
  H.start({ pages: [s0, marked(1, [hidden], 1), marked(2, [tbl()]), marked(3, [tbl(), fig()])], notes: [], index: 0, ratio: "16:9", onExit() {}, nextPreview: true });
  const cr = () => nextBoxOf(w).style["counter-reset"];
  ok("보조 번호: 첫 장 → 다음 장(1) 앞 캡션 수(본문 밖 figcaption 은 빼고)", nextShown(w) === "장1" && cr() === "fig 2 tbl 1", cr());
  H.next();
  ok("보조 번호: 장 1 → 다음 장(2) 앞 — 단계 장의 아직 숨은 캡션도 센다", P.idx === 1 && P.step === 0 && cr() === "fig 3 tbl 1", cr());
  H.next();
  ok("보조 번호: 단계만 넘기면 그대로", P.step === 1 && cr() === "fig 3 tbl 1", cr());
  H.next();
  ok("보조 번호: 장 2 → 다음 장(3) 앞", P.idx === 2 && cr() === "fig 3 tbl 2", cr());
  H.jump(0);
  ok("보조 번호: 되돌아가면 다시 맞춘다", cr() === "fig 2 tbl 1", cr());
  ok("보조 번호: 덱·장의 카운터는 건드리지 않는다", P.slides.every(s => !s.style["counter-reset"]) && !rootOf(w).querySelector(".pr-deck").style["counter-reset"]);
  H.stop();
}
{
  // 2회차 리뷰 [1]: 배치되지 않은 캡션(display: none — 예: 결재란 캡션)은 카운터를 올리지 않으므로 세지 않는다. 판정은 브라우저의 getClientRects 다
  // (가짜 DOM 에서는 그 함수를 단 요소만 판정한다 — 없으면 센다). 장별 수는 발표 동안 한 번만 잰다(같은 장을 다시 재지 않는다)
  const w = world(), { H } = w;
  // data-boxes 를 단 요소에만 getClientRects 를 준다(복제에도 따라간다 — 덱은 사본이다). 다른 시험의 요소는 지금처럼 그 함수가 없다
  let asked = 0;
  Object.defineProperty(El.prototype, "getClientRects", { configurable: true, get() { return "boxes" in this.dataset ? () => { asked++; return new Array(Number(this.dataset.boxes)); } : undefined; } });
  const cap = (tag, inner, boxes) => { const f = new El(tag), c = f.appendChild(new El(inner)); c.dataset.boxes = String(boxes); return f; };
  const s0 = marked(0, [cap("table", "caption", 0), cap("table", "caption", 1), cap("figure", "figcaption", 0)]);
  H.start({ pages: [s0, marked(1, [cap("table", "caption", 1)]), marked(2), marked(3)], notes: [], index: 0, ratio: "16:9", onExit() {}, nextPreview: true });
  const cr = () => nextBoxOf(w).style["counter-reset"];
  ok("보조 번호: 상자가 없는 캡션(display: none)은 세지 않고 상자가 있는 것만 센다", cr() === "fig 0 tbl 1", cr());
  H.next(); H.next();
  ok("보조 번호: 장 2 앞까지 — 앞 장들의 배치된 캡션만", cr() === "fig 0 tbl 2", cr());
  const n0 = asked;
  H.jump(0); H.jump(2);
  ok("보조 번호: 이미 잰 장은 다시 재지 않는다(장별 캐시)", asked === n0 && cr() === "fig 0 tbl 2", { asked, n0 });
  H.stop();
  H.start({ pages: [s0, marked(1), marked(2)], notes: [], index: 0, ratio: "16:9", onExit() {}, nextPreview: true });
  ok("보조 번호: 새 발표는 캐시를 버리고 다시 잰다", asked > n0 && cr() === "fig 0 tbl 1", { asked, n0 });
  H.stop();
}
{
  // 청중 열기·보드 발표에는 없다
  const w = world(), { P, H } = w;
  H.start({ pages: [marked(0), marked(1)], notes: [], index: 0, ratio: "16:9", onExit() {}, audience: true, nextPreview: true, clock: true });
  const root = rootOf(w);
  ok("보조: 청중 열기는 옵션을 줘도 미리보기·시각이 없고 set* 은 false", !P.nextPreview && !P.clock && !root.querySelector(".pr-next") && !root.querySelector(".pr-clock")
    && !root.classList.contains("pr-next-on") && H.setNextPreview(true) === false && H.setClock(true) === false);
  H.show({ i: 1 });
  ok("보조: 청중 넘김에도 미리보기가 생기지 않는다", !root.querySelector(".pr-next-box"));
  H.stop();
  const pg = new El("div"); pg.className = "page";
  w.H.startBoard({ page: pg, ratio: "16:9", onExit() {} });
  ok("보조: 보드 발표에는 없고 set* 은 false", !rootOf(w).querySelector(".pr-next") && !rootOf(w).querySelector(".pr-clock") && H.setNextPreview(true) === false && H.setClock(true) === false && !P.nextPreview);
  H.stop();
}
// ---- 14. 표지 id(itda-hyve#979): slides.js titleSlide 는 원본 표지 id 를 쪽의 자손 div.slide-cover 에, dashboard.js 는 header.board-head 에 단다
// (cover_id.cjs 가 실제 build 로 본다). 그 꼴의 장을 실제 발표에 넘겨 덱 사본·다음 장 미리보기·한눈에 보기 썸네일·보드 사본에 그 id 가 남지 않고
// 원본 장에는 남는지 본다. 장은 그 꼴을 손으로 만든다 — 이 가짜 DOM 은 cover_id.cjs 의 것과 달라 slides.js 출력을 그대로 넘기지 않는다.
{
  const hasCoverID = n => [n, ...n.querySelectorAll("[id]")].some(e => e.attrs.id === "hyve-cover");
  const coverPage = () => {
    const p = slide([]); p.classList.add("k-title"); p.dataset.n = "1";
    const box = p.querySelector(".page-body").appendChild(new El("div")); box.className = "slide-cover"; box.setAttribute("id", "hyve-cover");
    return p;
  };
  const w = world(), { P, H } = w;
  const pages = [marked(0), coverPage(), marked(2)];
  H.start({ pages, notes: [], index: 0, ratio: "16:9", onExit() {}, nextPreview: true });
  const root = rootOf(w);
  ok("표지 id: 다음 장 미리보기가 표지 장을 담는다", nextShown(w) === "장1", nextShown(w));
  ok("표지 id: 덱 사본·미리보기에 원본 표지 id 가 남지 않는다", !hasCoverID(root), P.slides.map(hasCoverID));
  H.toggleGrid(true);
  ok("표지 id: 한눈에 보기 썸네일에도 남지 않는다", root.querySelectorAll(".pr-thumb").filter(t => !t.classList.contains("pr-next-box")).length === 3 && !hasCoverID(root));
  H.toggleGrid(false);
  ok("표지 id: 원본 장(보기 화면)에는 그대로", hasCoverID(pages[1]));
  H.stop();
  const board = new El("div"); board.className = "page board";
  const bh = board.appendChild(new El("header")); bh.className = "board-head"; bh.setAttribute("id", "hyve-cover");
  board.appendChild(new El("div")).className = "page-body board-grid";
  H.startBoard({ page: board, ratio: "16:9", onExit() {} });
  ok("표지 id: 보드 발표 사본에 머리 id 가 남지 않고 원본 보드에는 그대로", !!rootOf(w).querySelector(".board-head") && !hasCoverID(rootOf(w)) && hasCoverID(board));
  H.stop();
}
// 발표자 화면과 청중 화면의 끝난 획 SVG 경로가 같다 — 같은 relay 연산에서 같은 path d(펜·형광펜·필압·지우개·되돌리기·다시 실행·전부 지우기).
// relay 는 앞 묶음의 Promise 가 풀린 뒤 다음을 보내므로 마이크로태스크를 돌린다(pump).
async function mirrorCases() {
  const tick = () => new Promise(r => setImmediate(r));
  const pump = async () => { for (let k = 0; k < 8; k++) { runTimers(); await tick(); } };
  const pw = world(), { P } = pw, sent = [], lasers = [];
  open(pw, { inkKey: "mirror" });
  pw.I.relay("mirror", { ink: j => { sent.push(j); return ""; }, laser: (...a) => lasers.push(a) });
  await pump();
  const aw = world(), av = aw.H;
  av.start({ pages: [slide([]), slide([]), slide([])], notes: [], index: 0, ratio: "16:9", onExit() {}, audience: true, onState() {} });
  const feed = () => { for (const j of sent.splice(0)) av.inkView().apply(j); runRaf(aw); };
  const same = (name) => {
    feed();
    const a = P.slides.map(inkOf), b = aw.P.slides.map(inkOf);
    ok("거울: " + name + " — 끝난 획 SVG 가 같다", JSON.stringify(a) === JSON.stringify(b) && a.flat().length > 0, { presenter: a, audience: b });
  };
  key(pw, "p", "KeyP");
  drag(pw, P.slides[0], line(100, 300, 8));
  await pump();
  same("펜");
  key(pw, "h", "KeyH");
  drag(pw, P.slides[0], line(100, 340, 6));
  await pump();
  same("형광펜");
  key(pw, "p", "KeyP");
  drag(pw, P.slides[0], [[400, 100], [420, 130], [450, 170], [470, 220]], { pointerType: "pen", pressure: 0.8 });
  await pump();
  same("필압 펜");
  // 그리는 중(끝나기 전)의 점 스트림: 청중 canvas 가 발표자와 같은 경로를 그린다
  fire(P.slides[0], "pointerdown", { buttons: 1, clientX: 600, clientY: 500 });
  fire(P.slides[0], "pointermove", { buttons: 1, clientX: 640, clientY: 520 });
  fire(P.slides[0], "pointermove", { buttons: 1, clientX: 690, clientY: 530 });
  await pump();
  feed();
  const pl = liveOf(P.slides[0]), al = liveOf(aw.P.slides[0]);
  runRaf(pw);
  ok("거울: 그리는 중인 획의 canvas 경로가 같다", pl && al && pl.ctx.paints.length === 1 && JSON.stringify(pl.ctx.paints) === JSON.stringify(al.ctx.paints), { p: pl && pl.ctx.paints, a: al && al.ctx.paints });
  fire(P.slides[0], "pointerup", { clientX: 690, clientY: 530 });
  await pump();
  same("그리던 획 끝");
  key(pw, "z", "KeyZ", { metaKey: true });
  await pump();
  same("되돌리기");
  key(pw, "z", "KeyZ", { metaKey: true, shiftKey: true });
  await pump();
  same("다시 실행");
  key(pw, "e", "KeyE");
  drag(pw, P.slides[0], [[100, 300], [130, 300], [160, 300]]);
  await pump();
  same("지우개");
  key(pw, "p", "KeyP");
  pw.H.next();
  drag(pw, P.slides[1], line(200, 200, 5));
  await pump();
  same("다음 장에 그리기");
  pw.H.prev();
  key(pw, "x", "KeyX");
  await pump();
  feed();
  ok("거울: 전부 지우기 — 두 쪽 모두 그 장이 빈다", inkOf(P.slides[0]).length === 0 && inkOf(aw.P.slides[0]).length === 0 && inkOf(aw.P.slides[1]).length === 1);
  key(pw, "z", "KeyZ", { metaKey: true });
  await pump();
  same("전부 지우기 되돌리기");
  // 늦게 붙은 청중: relay 를 다시 이으면 reset 하나로 같은 상태가 된다
  const lw = world();
  lw.H.start({ pages: [slide([]), slide([]), slide([])], notes: [], index: 0, ratio: "16:9", onExit() {}, audience: true, onState() {} });
  const late = [];
  pw.I.relay("mirror", { ink: j => { late.push(j); return ""; }, laser: (...a) => lasers.push(a) });
  await pump();
  for (const j of late) lw.H.inkView().apply(j);
  runRaf(lw);
  ok("거울: 늦게 붙은 청중은 reset 하나로 같은 상태를 받는다", late.length === 1 && JSON.parse(late[0]).k === "reset" && JSON.stringify(P.slides.map(inkOf)) === JSON.stringify(lw.P.slides.map(inkOf)), { n: late.length });
  // 레이저 값이 그대로 넘어온다
  key(pw, "l", "KeyL");
  fire(P.slides[0], "pointermove", { clientX: 300, clientY: 200 });
  await pump();
  const lv = lasers[lasers.length - 1];
  ok("거울: 레이저는 (장, x, y, 선) 으로 넘어온다", lv && lv[0] === 0 && lv[1] === 300 && lv[2] === 200 && lv[3] === false, lasers);
  av.inkView().laser(...lv);
  runRaf(aw);
  ok("거울: 넘어온 레이저를 청중 화면 지금 장에 그린다", spotOf(aw.P.slides[0]) && spotOf(aw.P.slides[0]).ctx.paints.some(p => p.op === "fill"));
  pw.H.stop();
  av.stop();
  lw.H.stop();
  await repeatCases(pump);
  await laserTurnCases(pump);
}
// 2회차 리뷰 [1]: 레이저가 꺼짐(-1)을 거쳐 장을 넘긴 뒤 새 장의 첫 점에, 앞 장의 꼬리를 잇지 않는다 — 실제 attach → relay → view.
async function laserTurnCases(pump) {
  const pw = world(), aw = world();
  open(pw, { inkKey: "laser-turn" });
  aw.H.start({ pages: [slide([]), slide([]), slide([])], notes: [], index: 0, ratio: "16:9", onExit() {}, audience: true, onState() {} });
  const v = aw.H.inkView(), sent = [];
  pw.I.relay("laser-turn", { ink: j => { v.apply(j); return ""; }, laser: (...a) => { sent.push(a); v.laser(...a); } });
  await pump();
  key(pw, "l", "KeyL");
  fire(pw.P.slides[0], "pointermove", { clientX: 100, clientY: 100 });
  await pump();
  fire(pw.P.slides[0], "pointermove", { clientX: 120, clientY: 120 });
  await pump();
  runRaf(aw);
  const before = spotOf(aw.P.slides[0]).ctx.paints.filter(p => p.op === "stroke").length;
  pw.H.next();
  aw.H.show({ i: 1 });
  await pump();
  runRaf(aw);
  fire(pw.P.slides[1], "pointermove", { clientX: 300, clientY: 300 });
  await pump();
  runRaf(pw);
  runRaf(aw);
  const turn = sent.map(a => a[0]).join(), ap = spotOf(aw.P.slides[1]).ctx.paints, pp = rootOf(pw).querySelector(".ink-laser").ctx.paints;
  ok("레이저 넘김: 전달이 앞 장 → 꺼짐 → 새 장이다(시험 전제)", /^0,0,-1,1$/.test(turn) && before === 1, { sent, before });
  ok("레이저 넘김: 새 장에는 앞 장의 꼬리를 잇지 않고 점만 그린다(발표자와 같다)", !ap.some(p => p.op === "stroke") && ap.filter(p => p.op === "fill").length === 2 && !pp.some(p => p.op === "stroke") && pp.filter(p => p.op === "fill").length === 2, { audience: ap, presenter: pp });
  pw.H.stop();
  aw.H.stop();
  // 그릴 때도 점마다 장을 본다: 꼬리에 다른 장의 점이 섞여 있어도 지금 장의 점만 잇는다
  const w = world(), vv = w.I.view(), pages = [slide([]), slide([])];
  let cur = 0;
  vv.mount({ slides: () => pages, size: [1280, 720], scale: () => 1, idx: () => cur, paused: () => false });
  vv.laser(0, 10, 10, false); vv.laser(0, 20, 20, false); vv.laser(-1, 0, 0, false);
  cur = 1; vv.wake(); runRaf(w);
  ok("레이저 넘김: 꺼진 뒤 다른 장으로 넘기면 앞 장 꼬리 때문에 새 장에 레이저 층을 만들지 않는다", !spotOf(pages[1]), pages[1].kids.map(k => k.className));
  vv.laser(1, 300, 300, false);
  ok("레이저 넘김: 꺼짐 뒤 다른 장의 첫 점은 앞 장 꼬리를 비운다(꼬리 = 새 점 하나)", vv.held().trail === 1, vv.held());
  w.raf.length = 0;
}
// 1회차 리뷰 [3]: 되돌아와 같은 좌표를 다시 지나는 정상 입력(두 묶음의 a 가 같은 점들)을 청중이 버리지 않는다. 같은 묶음을 다시 넘기는 것은 계약 밖이고,
// 그때는 relay 를 다시 불러 reset 으로 맞춘다.
async function repeatCases(pump) {
  const pw = world(), aw = world(), P = pw.P;
  open(pw, { inkKey: "repeat" });
  aw.H.start({ pages: [slide([]), slide([]), slide([])], notes: [], index: 0, ratio: "16:9", onExit() {}, audience: true, onState() {} });
  const av = aw.H.inkView(), batches = [], rets = [];
  pw.I.relay("repeat", { ink: j => { batches.push(j); rets.push(av.apply(j)); return ""; }, laser() {} });
  await pump();
  key(pw, "p", "KeyP");
  const pg = P.slides[0];
  fire(pg, "pointerdown", { buttons: 1, clientX: 100, clientY: 100 });
  fire(pg, "pointermove", { buttons: 1, clientX: 120, clientY: 100 });
  fire(pg, "pointermove", { buttons: 1, clientX: 110, clientY: 120 });
  await pump();
  fire(pg, "pointermove", { buttons: 1, clientX: 90, clientY: 90 });
  fire(pg, "pointermove", { buttons: 1, clientX: 120, clientY: 100 });
  fire(pg, "pointermove", { buttons: 1, clientX: 110, clientY: 120 });
  await pump();
  fire(pg, "pointerup", { clientX: 120, clientY: 100 });
  await pump();
  av.flush();
  const as = batches.map(j => JSON.parse(j).ops.filter(o => o[0] === "a").map(o => JSON.stringify(o[2]))).flat();
  ok("되풀이: 두 묶음의 a 가 같은 점들이다(시험 전제)", as.length >= 2 && as[0] === as[1], as);
  ok("되풀이: 같은 점을 다시 지나는 정상 입력도 청중 경로가 발표자와 같다", inkOf(pg).length === 1 && JSON.stringify(inkOf(pg)) === JSON.stringify(inkOf(aw.P.slides[0])) && rets.every(r => r === true), { p: inkOf(pg), a: inkOf(aw.P.slides[0]), rets });
  // 계약 밖: 같은 a 묶음을 다시 넘기면 점이 겹친다 — relay 를 다시 부르면 reset 으로 맞는다
  key(pw, "p", "KeyP"); key(pw, "p", "KeyP");
  fire(pg, "pointerdown", { buttons: 1, clientX: 300, clientY: 300 });
  fire(pg, "pointermove", { buttons: 1, clientX: 340, clientY: 300 });
  await pump();
  av.apply(batches[batches.length - 1]); // 호스트가 잘못 다시 넘겼다
  fire(pg, "pointerup", { clientX: 340, clientY: 300 });
  await pump();
  av.flush();
  const differ = JSON.stringify(inkOf(pg)) !== JSON.stringify(inkOf(aw.P.slides[0]));
  pw.I.relay("repeat", { ink: j => { av.apply(j); return ""; }, laser() {} });
  await pump();
  av.flush();
  ok("되풀이: 다시 넘긴 묶음은 겹치고(계약 밖), relay 를 다시 부르면 reset 으로 같아진다", differ && JSON.stringify(inkOf(pg)) === JSON.stringify(inkOf(aw.P.slides[0])), { differ, p: inkOf(pg), a: inkOf(aw.P.slides[0]) });
  pw.H.stop();
  aw.H.stop();
}

// ---- 15. 누르기 규칙(present.js 머리 주석 「조작」 누르기·edgeAt·zoomTarget): 무대 가장자리 = 이전·다음(링크·크게 보기보다 먼저), 가운데는 링크·크게 보기
// (본문 그림·분할 그림·배경 그림)·빈 곳 = 아무것도 아님, 오른쪽 단추 = 이전, 검은 화면 = 어디든 다음, 레이저 클릭도 같은 규칙, 보드 발표는 그대로
{
  const w = world(), { P, H } = w, hits = [], opened = [];
  w.win.ItdaLightbox = { SEL: "img, table", create(root, o) {
    const el = root.appendChild(new El("div")); el.className = "pr-lightbox";
    let on = false;
    return { el, open(h) { on = true; hits.push(h); o.onOpen("img", 1); }, close() { if (on) { on = false; o.onClose(); } }, onScrollbar: () => false, relayout() {} };
  } };
  const mk = (tag, cls, kids) => { const e = new El(tag); if (cls) e.className = cls; for (const k of kids || []) e.appendChild(k); return e; };
  const a = href => { const x = new El("a"); x.setAttribute("href", href); return x; };
  // slot 은 slides.js slotBox 꼴 — 쪽의 직접 자식 .slide-panel·.slide-bg 안 블록(figure > img, 또는 링크로 감싼 그림)
  const slotted = (cls, block, body) => { const pg = slide(body); pg.kids.unshift(mk("div", cls, [block])); pg.kids[0].parentNode = pg; return pg; };
  const pages = [
    slide([mk("p")]),                                                        // 0 보통 장(그림 없음)
    slotted("slide-panel", mk("figure", "", [img()]), [mk("p")]),            // 1 분할 그림
    slotted("slide-bg", mk("figure", "", [img()]), [mk("p"), a("https://example.com/bg"), a("notes.pdf")]), // 2 배경 그림(본문에 글·링크·파일 링크)
    slotted("slide-bg", mk("p", "", [mk("a", "", [img()])]), [mk("p")]),     // 3 링크로 감싼 배경 그림
    slide([mk("p")]),
  ];
  pages[3].kids[0].kids[0].kids[0].setAttribute("href", "https://example.com/wrap");
  const bgLogo = pages[2].appendChild(mk("div", "slide-foot", [mk("img", "sf-logo")])); // 배경 장 바닥 로고
  H.start({ pages, notes: [], index: 0, ratio: "16:9", onExit() {}, onLink: h => opened.push(h) });
  const root = rootOf(w), stage = root.querySelector(".pr-stage"), cur = () => P.slides[P.idx];
  const C = { clientX: 640 };
  ok("가장자리: 무대에 ‹ › 표시 상자 둘(aria-hidden)", stage.querySelectorAll(".pr-edge").length === 2 && !!stage.querySelector(".pr-edge-prev") && !!stage.querySelector(".pr-edge-next")
    && stage.querySelectorAll(".pr-edge").every(x => x.getAttribute("aria-hidden") === "true"));
  tap(w, cur().querySelector("p"), C);
  ok("가운데 빈 곳(보통 장): 넘기지 않고 크게 보지도 않는다", P.idx === 0 && !P.lightbox && hits.length === 0, { idx: P.idx, lb: P.lightbox });
  tap(w, cur().querySelector("p"), { clientX: 1200 });
  ok("오른쪽 가장자리: 다음", P.idx === 1, P.idx);
  tap(w, cur().querySelector(".page-body p"), { clientX: 40 });
  ok("왼쪽 가장자리: 이전", P.idx === 0, P.idx);
  tap(w, cur().querySelector("p"), { clientX: 1200 });
  // 1 분할 그림
  const pimg = cur().querySelector(".slide-panel img");
  tap(w, pimg, C);
  ok("분할 그림: 가운데 누르기는 크게 보기(대상은 패널 img)", P.lightbox === "img" && hits[hits.length - 1] === pimg && P.idx === 1, { lb: P.lightbox, n: hits.length });
  key(w, "Escape", "Escape");
  ok("분할 그림: Esc 는 크게 보기만 닫는다", !P.lightbox && P.on && P.idx === 1);
  const n1 = hits.length;
  tap(w, pimg, { clientX: 1200 });
  ok("가장자리가 먼저: 가장자리 안 분할 그림 누르기는 다음(크게 보지 않는다)", P.idx === 2 && hits.length === n1 && !P.lightbox, { idx: P.idx, n: hits.length });
  // 2 배경 그림
  const bimg = cur().querySelector(".slide-bg img");
  tap(w, cur().querySelector(".page-body p"), C);
  ok("배경 그림: 본문 글자 위 가운데 누르기는 배경 그림 크게 보기", P.lightbox === "img" && hits[hits.length - 1] === bimg && P.idx === 2, { lb: P.lightbox });
  key(w, "Escape", "Escape");
  tap(w, bimg, C);
  ok("배경 그림: 그림 자체 누르기도 크게 보기", P.lightbox === "img" && hits[hits.length - 1] === bimg);
  key(w, "Escape", "Escape");
  const n2 = hits.length;
  const eaten = tap(w, cur().querySelector(".page-body a"), C);
  ok("배경 그림 위 링크: 링크 동작(크게 보지 않는다)", opened.join() === "https://example.com/bg" && eaten && hits.length === n2 && !P.lightbox && P.idx === 2, { opened, n: hits.length });
  tap(w, cur().querySelector(".sf-logo"), C);
  ok("배경 그림 장의 로고: 가운데 누르기는 아무것도 하지 않는다(배경을 열지 않는다)", hits.length === n2 && !P.lightbox && P.idx === 2, { n: hits.length, idx: P.idx });
  tap(w, cur().querySelector('.page-body a[href="notes.pdf"]'), C);
  ok("배경 그림 장의 파일 링크: 가운데 누르기는 아무것도 하지 않는다(링크도 배경도 아니다)", hits.length === n2 && !P.lightbox && P.idx === 2 && opened.length === 1, { n: hits.length, idx: P.idx, opened });
  tap(w, cur().querySelector(".page-body a"), { clientX: 40 });
  ok("가장자리가 먼저: 가장자리 안 링크 누르기는 이전(링크를 열지 않는다)", P.idx === 1 && opened.length === 1, { idx: P.idx, opened });
  H.jump(3);
  // 3 링크로 감싼 배경 그림 — 그 밖의 곳은 크게 보지 않는다
  const n3 = hits.length;
  tap(w, cur().querySelector(".page-body p"), C);
  ok("링크로 감싼 배경 그림: 본문 위 가운데 누르기는 크게 보지 않고 넘기지도 않는다", hits.length === n3 && !P.lightbox && P.idx === 3, { n: hits.length, idx: P.idx });
  // 오른쪽 단추·검은 화면
  tap(w, cur().querySelector(".page-body p"), { clientX: 640, button: 2 });
  ok("오른쪽 단추: 가운데에서도 이전", P.idx === 2, P.idx);
  H.setBlack(true);
  tap(w, cur().querySelector(".page-body p"), C);
  ok("검은 화면: 가운데 누르기도 다음(크게 보지 않는다)", P.idx === 3 && !P.lightbox, { idx: P.idx, lb: P.lightbox });
  H.setBlack(true);
  tap(w, cur().querySelector(".page-body p"), { clientX: 40 });
  ok("검은 화면: 왼쪽 가장자리 누르기도 다음", P.idx === 4, P.idx);
  H.setBlack(false);
  // 레이저 클릭(ink.js onUp → inkClick)도 같은 규칙
  key(w, "l", "KeyL");
  tap(w, cur().querySelector("p"), C);
  ok("레이저: 가운데 빈 곳 클릭은 넘기지 않는다", P.idx === 4 && toolOf(w) === "laser", P.idx);
  tap(w, cur().querySelector("p"), { clientX: 40 });
  ok("레이저: 왼쪽 가장자리 클릭은 이전", P.idx === 3, P.idx);
  H.jump(1);
  const n4 = hits.length;
  tap(w, cur().querySelector(".slide-panel img"), C);
  ok("레이저: 가운데 분할 그림 클릭은 크게 보기", hits.length === n4 + 1 && P.lightbox === "img", hits.length);
  key(w, "Escape", "Escape");
  tap(w, cur().querySelector(".slide-panel img"), { clientX: 1200 });
  ok("레이저: 가장자리 안 분할 그림 클릭은 다음", P.idx === 2 && hits.length === n4 + 1, { idx: P.idx, n: hits.length });
  H.jump(2);
  tap(w, cur().querySelector(".page-body a"), C);
  ok("레이저: 가운데 링크 클릭은 링크", opened.length === 2 && P.idx === 2, opened);
  key(w, "l", "KeyL");
  // 가장자리 폭 — 무대 너비의 7% 를 48~96px 로 묶는다. 무대(붙인 칸을 뺀 자리)의 왼쪽을 기준으로 잰다
  H.jump(0);
  const p0 = () => cur().querySelector("p");
  stage.getBoundingClientRect = () => ({ left: 0, top: 0, width: 400, height: 720, right: 400, bottom: 720 });
  tap(w, p0(), { clientX: 50 });
  ok("가장자리 폭 하한 48px: 좁은 무대(400px — 7% 는 28px)의 x 50 은 가운데", P.idx === 0, P.idx);
  tap(w, p0(), { clientX: 360 });
  ok("가장자리 폭 하한 48px: x 360(오른쪽에서 40) 은 다음", P.idx === 1, P.idx);
  H.jump(0);
  stage.getBoundingClientRect = () => ({ left: 0, top: 0, width: 2000, height: 1000, right: 2000, bottom: 1000 });
  tap(w, p0(), { clientX: 1880 });
  ok("가장자리 폭 상한 96px: 넓은 무대(2000px — 7% 는 140px)의 x 1880(오른쪽에서 120) 은 가운데", P.idx === 0, P.idx);
  tap(w, p0(), { clientX: 1920 });
  ok("가장자리 폭 상한 96px: x 1920(오른쪽에서 80) 은 다음", P.idx === 1, P.idx);
  stage.getBoundingClientRect = () => ({ left: 300, top: 0, width: 980, height: 720, right: 1280, bottom: 720 });
  tap(w, p0(), { clientX: 350 });
  ok("무대 기준: 왼쪽 칸을 붙여 무대가 x 300 부터면 x 350 이 왼쪽 가장자리(이전)", P.idx === 0, P.idx);
  tap(w, p0(), { clientX: 100 });
  ok("무대 기준: 무대 밖 x 100 은 가장자리가 아니다", P.idx === 0, P.idx);
  H.stop();
  // 청중 열기·보드 발표에는 가장자리가 없다. 보드 발표 누르기는 그대로(그림 = 크게 보기, 그 밖 = 아무것도 아님)
  open(w, { audience: true });
  ok("청중 열기: 가장자리 표시가 없다", rootOf(w).querySelectorAll(".pr-edge").length === 0);
  H.stop();
  const bp = slide([mk("p"), img()]); bp.classList.add("board");
  H.startBoard({ page: bp, ratio: "16:9", onExit() {} });
  const broot = rootOf(w), bpg = P.slides[0];
  ok("보드 발표: 가장자리 표시가 없다", broot.querySelectorAll(".pr-edge").length === 0);
  const n5 = hits.length;
  tap(w, bpg.querySelector("p"), { clientX: 1200 });
  ok("보드 발표: 오른쪽 끝 누르기도 아무것도 하지 않는다", P.on && P.board && !P.lightbox && hits.length === n5);
  tap(w, bpg.querySelector("img"), { clientX: 1200 });
  ok("보드 발표: 그림 누르기는 자리와 상관없이 크게 보기(그대로)", P.lightbox === "img" && hits.length === n5 + 1);
  H.stop();
}

mirrorCases().then(() => console.log("PRESENT INK DONE cases=" + cases + " failed=" + failed), e => { console.log("실패 거울 시험 예외 — " + (e && e.stack)); process.exit(1); });
