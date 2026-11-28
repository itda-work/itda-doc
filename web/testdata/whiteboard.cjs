// 화이트보드(wbcore.js·whiteboard.js)와 발표의 W 키(present.js) 시험 — whiteboard_test.go 가 web/ 폴더를 넘긴다. 가짜 DOM 위에서 돈다.
// 순수 함수(좌표·외곽선·curve·색인·타일)·파일 형식(왕복·검증·모르는 키·큰 v), 좌표 지킴이, 입력 규칙, 자동 저장, 판 목록의 store 호출 차례,
// 열린 동안 아래 화면이 키·포인터를 받지 않음, Esc 한 겹씩, 발표의 W(계약 있음·없음·청중·보드 발표)를 본다.
// 실제 그리기(canvas 픽셀)·CSS·실기 입력은 보지 않는다 — 가짜 canvas 는 명령을 버린다.
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
const near = (a, b, e) => Math.abs(a - b) <= (e == null ? 1e-9 : e);

// ---- 가짜 DOM: 선택자는 tag·.class·[attr] 와 쉼표 목록·자손 결합자만. 사건은 붙잡기(창 → 대상) → 대상 → 거품(대상 → 창) 차례로 돈다.
function matchOne(e, sel) {
  const m = /^([a-z0-9]*)((?:\.[\w-]+)*)((?:\[[\w-]+\])*)$/i.exec(sel.trim());
  if (!m) return false;
  if (m[1] && e.tag !== m[1].toLowerCase()) return false;
  for (const c of m[2].split(".").filter(Boolean)) if (!e.classList || !e.classList.contains(c)) return false;
  for (const a of m[3].replace(/[\[\]]/g, " ").trim().split(/\s+/).filter(Boolean)) if (!(a in e.attrs) && !(a.startsWith("data-") && a.slice(5) in e.dataset)) return false;
  return true;
}
class El {
  constructor(tag) {
    this.tag = tag; this.kids = []; this.parentNode = null; this.attrs = {}; this.dataset = {}; this.on = {}; this._cls = new Set(); this._text = "";
    this.style = { setProperty(k, v) { this[k] = v; }, getPropertyValue(k) { return this[k] || ""; }, removeProperty(k) { delete this[k]; } };
    const c = this._cls;
    this.classList = { add: (...a) => a.forEach(x => c.add(x)), remove: (...a) => a.forEach(x => c.delete(x)), contains: x => c.has(x), toggle: (x, f) => { const v = f === undefined ? !c.has(x) : !!f; if (v) c.add(x); else c.delete(x); return v; } };
    this.clientWidth = 1280; this.clientHeight = 720; this.offsetWidth = 300; this.offsetHeight = 200; this.title = ""; this.disabled = false; this.hidden = false; this.nodeType = 1;
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
  // 포커스가 있던 요소(또는 그 조상)를 떼면 포커스는 body 로 간다(브라우저와 같다)
  remove() { const p = this.parentNode; if (p) { const d = world.cur && world.cur.doc; if (d && this.contains(d.activeElement)) d.activeElement = d.body; p.kids.splice(p.kids.indexOf(this), 1); this.parentNode = null; } }
  contains(x) { for (let e = x; e; e = e.parentNode) if (e === this) return true; return false; }
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
  hasAttribute(k) { return k in this.attrs; }
  removeAttribute(k) { delete this.attrs[k]; }
  addEventListener(t, f, o) { const capture = o === true || !!(o && o.capture); (this.on[t] = this.on[t] || []).push({ f, capture }); }
  removeEventListener(t, f, o) { const capture = o === true || !!(o && o.capture); this.on[t] = (this.on[t] || []).filter(x => !(x.f === f && x.capture === capture)); }
  getBoundingClientRect() { return { left: 0, top: 0, right: 1280, bottom: 720, width: 1280, height: 720 }; }
  setPointerCapture() { throw new Error("합성 입력"); }
  releasePointerCapture() {}
  focus() { world.cur.doc.activeElement = this; }
  blur() {}
  select() {}
  getContext() { if (this.tag !== "canvas") return null; this.ctx = this.ctx || ctx2d(); return this.ctx; }
  // 그림 시험용: canvas 의 toDataURL 은 그린 그림의 크기·투명을 머리 「IMG 폭 높이 투명」 에 싣고 world.imgK × 화소 수 만큼 채운 바이트의 base64 다.
  // file input 의 click 은 연 수를 센다(고른 파일은 시험이 files 에 넣고 change 를 보낸다).
  toDataURL(type) {
    const src = this.ctx && this.ctx._src, alpha = !!(src && src.alpha), k = world.cur.imgK == null ? 0.05 : world.cur.imgK;
    const head = "IMG " + this.width + " " + this.height + " " + (alpha ? 1 : 0) + " ";
    const body = head + "x".repeat(Math.max(0, Math.floor(this.width * this.height * k) - head.length));
    const mime = type === "image/jpeg" ? "image/jpeg" : "image/png";
    (world.cur.encodes = world.cur.encodes || []).push({ w: this.width, h: this.height, mime, bytes: body.length });
    return "data:" + mime + ";base64," + Buffer.from(body, "latin1").toString("base64");
  }
  click() { this.clicked = (this.clicked || 0) + 1; }
  cloneNode(deep) {
    const c = new El(this.tag); c.className = this.className; c.attrs = { ...this.attrs }; c.dataset = { ...this.dataset }; c._text = this._text;
    if (deep) for (const k of this.kids) c.appendChild(k.cloneNode(true));
    return c;
  }
  listeners(t, capture) { return (this.on[t] || []).filter(x => capture === undefined || x.capture === capture).length; }
}
// ctx2d 는 그리기 명령을 버린다(세는 것만 — fills·strokes). ops 는 clearRect·drawImage·fill·fillRect 의 차례다(그림이 획 아래인가 — 최근 것만 든다).
// drawImage 는 그린 원천을 _src 로 둔다(가짜 toDataURL·getImageData 가 그 투명을 본다).
// draws 는 drawImage 의 원천과 그때 setTransform 의 평행 이동 tx 다({ src, tx } — 그림 층의 그리기 차례·자리를 본다, 최근 것만).
function ctx2d() {
  const t = { fills: 0, strokes: 0, ops: [], draws: [], _src: null, _t: null };
  const op = (o, x) => { o.ops.push(x); if (o.ops.length > 4000) o.ops.splice(0, 2000); };
  return new Proxy(t, { get(o, k) {
    if (k in o) return o[k];
    if (k === "fill") return () => { o.fills++; op(o, "fill"); };
    if (k === "stroke") return () => { o.strokes++; };
    if (k === "clearRect") return () => op(o, "clear");
    if (k === "fillRect") return () => op(o, "rect");
    if (k === "setTransform") return (...a) => { o._t = a; };
    if (k === "drawImage") return src => {
      o._src = src; op(o, src && src.tag === "canvas" ? "canvas" : "img");
      o.draws.push({ src, tx: o._t ? o._t[4] : 0 }); if (o.draws.length > 4000) o.draws.splice(0, 2000);
    };
    if (k === "getImageData") return () => ({ data: new Uint8ClampedArray([0, 0, 0, o._src && o._src.alpha ? 0 : 255, 0, 0, 0, 255]) });
    return () => {};
  }, set(o, k, v) { o[k] = v; return true; } });
}
class Path2D { constructor(d) { this.d = d; this.n = 0; } moveTo() { this.n++; } lineTo() { this.n++; } closePath() {} arc() { this.n++; } }

// dispatch 는 사건 하나를 붙잡기 → 대상 → 거품 차례로 돌린다. 돌려준 e 의 dp·stopped 는 preventDefault·stopPropagation 을 불렀는가다.
function dispatch(w, target, type, init) {
  const e = Object.assign({ type, target, button: 0, buttons: 0, clientX: 0, clientY: 0, timeStamp: w.clock, dp: false, stopped: false,
    preventDefault() { this.dp = true; }, stopPropagation() { this.stopped = true; } }, init);
  const chain = [];
  for (let n = target; n; n = n.parentNode) chain.push(n);
  if (chain[chain.length - 1] === w.doc) chain.push(w.win);
  else if (target !== w.win && chain[chain.length - 1] !== w.win) { /* 문서에 붙지 않은 요소 */ }
  const run = (n, capture) => { for (const l of [...(n.on[type] || [])]) { if (capture !== undefined && l.capture !== capture) continue; l.f.call(n, e); } };
  for (let i = chain.length - 1; i > 0 && !e.stopped; i--) { e.currentTarget = chain[i]; run(chain[i], true); }
  if (!e.stopped) { e.currentTarget = target; run(target, undefined); }
  if (type !== "pointerleave") for (let i = 1; i < chain.length && !e.stopped; i++) { e.currentTarget = chain[i]; run(chain[i], false); }
  return e;
}

function world(files) {
  const doc = new El("#document"), win = new El("window"), body = new El("body");
  doc.body = body; body.parentNode = doc; doc.activeElement = body; doc.visibilityState = "visible";
  win.document = doc;
  const w = { doc, win, body, raf: [], timers: new Map(), clock: 1000, seq: 0, bitmaps: [], canvases: [], imgK: 0.05, encodes: [] };
  doc.createElement = t => { const e = new El(t.toLowerCase()); if (e.tag === "canvas") w.canvases.push(e); return e; };
  doc.createElementNS = (ns, t) => new El(t);
  // 가짜 createImageBitmap: 시험이 만든 파일({ type, _img: { w, h, alpha, bad } })은 그 크기로, Blob(판 본문의 data 를 푼 것)은 바이트의 머리
  // 「IMG 폭 높이 투명」 으로, canvas 는 그 폭·높이로 디코드한다(머리가 없거나 bad 면 거부 — 디코드 실패). 만든 bitmap 은 w.bitmaps 에 남는다(close 했는가를 본다).
  win.Blob = Blob;
  win.createImageBitmap = async src => {
    let info = null;
    if (src && src.tag === "canvas") info = { w: src.width, h: src.height, alpha: !!(src.ctx && src.ctx._src && src.ctx._src.alpha), from: "canvas" }; // 줄인 canvas(fitBitmap)
    else if (src && src._img) info = src._img;
    else if (src && typeof src.arrayBuffer === "function") {
      const m = /^IMG (\d+) (\d+) (\d)/.exec(Buffer.from(await src.arrayBuffer()).toString("latin1"));
      if (m) info = { w: +m[1], h: +m[2], alpha: m[3] === "1" };
    }
    if (!info || info.bad) throw new Error("디코드 실패");
    const b = { width: info.w, height: info.h, alpha: !!info.alpha, closed: false, close() { this.closed = true; } };
    w.bitmaps.push(b);
    return b;
  };
  world.cur = w;
  Object.assign(win, {
    devicePixelRatio: 2, navigator: { platform: "MacIntel" }, matchMedia: () => ({ matches: false }), TextEncoder,
    requestAnimationFrame: f => { const id = ++w.seq; w.raf.push({ id, f }); return id; },
    cancelAnimationFrame: id => { const i = w.raf.findIndex(g => g.id === id); if (i >= 0) w.raf.splice(i, 1); },
    ItdaSlides: { SIZE: { "16:9": [1280, 720], "4:3": [960, 720] } },
    getComputedStyle: () => ({ backgroundColor: "rgb(255, 255, 255)", width: "0px", height: "0px" }),
    Path2D, performance: { now: () => w.clock },
  });
  const setTimeout_ = (f, ms) => { const id = ++w.seq; w.timers.set(id, { at: w.clock + (ms || 0), f }); return id; };
  const clearTimeout_ = id => { w.timers.delete(id); };
  win.setTimeout = setTimeout_; win.clearTimeout = clearTimeout_;
  const ctx = vm.createContext({ window: win, document: doc, URL, performance: win.performance, setTimeout: setTimeout_, clearTimeout: clearTimeout_,
    setInterval: () => 0, clearInterval() {}, requestAnimationFrame: win.requestAnimationFrame, console, TextEncoder });
  w.ctx = ctx;
  for (const f of files || ["wbcore.js", "whiteboard.js"]) vm.runInContext(fs.readFileSync(path.join(webDir, f), "utf8"), ctx, { filename: f });
  w.W = win.ItdaWhiteboard;
  w.C = win.ItdaWbCore;
  return w;
}
// advance 는 시계를 ms 만큼 옮기며 때가 된 타이머를 차례로 부른다.
function advance(w, ms) {
  const end = w.clock + ms;
  for (;;) {
    let next = null;
    for (const [id, t] of w.timers) if (t.at <= end && (!next || t.at < next[1].at)) next = [id, t];
    if (!next) break;
    w.timers.delete(next[0]);
    w.clock = Math.max(w.clock, next[1].at);
    next[1].f();
  }
  w.clock = end;
}
function frames(w, n) { for (let k = 0; k < (n || 1); k++) { const q = w.raf.splice(0); for (const g of q) g.f(w.clock); } }
const tick = () => new Promise(r => setImmediate(r));
async function settle(n) { for (let i = 0; i < (n || 8); i++) await tick(); }

// logged 는 store 를 감싸 부른 차례를 적는다. ctl.fail[name] 이 참이면 그 함수는 거부한다. ctl.hold.save 가 참이면 save 를 손으로 풀 때까지 붙든다(ctl.release()).
function logged(store, ctl) {
  ctl.log = []; ctl.fail = ctl.fail || {}; ctl.hold = ctl.hold || {}; ctl.waiting = [];
  ctl.release = () => { const q = ctl.waiting.splice(0); q.forEach(f => f()); };
  const out = {};
  for (const k of Object.keys(store)) out[k] = (...a) => {
    ctl.log.push(k + (a.length && k !== "save" ? ":" + a[0] : k === "save" ? ":" + a[0] : ""));
    if (k === "save") ctl.lastSave = a[1];
    if (ctl.fail[k]) return Promise.reject(new Error("실패 " + k));
    if (ctl.hold[k]) return new Promise(r => ctl.waiting.push(() => r(store[k](...a))));
    return store[k](...a);
  };
  return out;
}
// ops 는 store 차례에서 목록 미리보기가 부른 load 를 뺀 것이다(목록이 열린 동안 미리보기는 다른 판의 본문을 읽는다)
const ops = ctl => ctl.log.filter(x => !/^load:/.test(x)).join();
const surfaceOf = w => w.body.kids.find(k => k.classList.contains("itda-wb")) || null;
async function openWb(w, store, o) {
  w.win.ItdaWhiteboardHost = { store };
  const p = w.W.open(o || {});
  await settle();
  const root = surfaceOf(w);
  frames(w, 2);
  return { p, root, stage: root && root.querySelector(".itda-wb-stage"), D: w.W.debug() };
}
const btn = (root, act, extra) => root.querySelectorAll("button").find(b => b.dataset.act === act && (!extra || Object.keys(extra).every(k => b.dataset[k] === extra[k])));
const click = (w, t) => dispatch(w, t, "click", { button: 0 });
function ptr(w, t, type, o) { return dispatch(w, t, type, Object.assign({ pointerId: 1, pointerType: "mouse", pressure: 0.5, buttons: type === "pointerup" ? 0 : 1 }, o)); }
function stroke(w, t, pts, o) {
  const a = Object.assign({ pointerId: 7, pointerType: "pen" }, o);
  ptr(w, t, "pointerdown", { ...a, clientX: pts[0][0], clientY: pts[0][1], pressure: pts[0][2] == null ? 0.5 : pts[0][2] });
  for (const p of pts.slice(1)) { w.clock += 4; ptr(w, t, "pointermove", { ...a, clientX: p[0], clientY: p[1], pressure: p[2] == null ? 0.5 : p[2] }); }
  const l = pts[pts.length - 1];
  ptr(w, t, "pointerup", { ...a, clientX: l[0], clientY: l[1], pressure: 0 });
  frames(w, 1);
}
const line = (x0, y0, x1, y1, n) => Array.from({ length: n + 1 }, (_, i) => [x0 + ((x1 - x0) * i) / n, y0 + ((y1 - y0) * i) / n]);
// 대상을 주지 않으면 포커스가 있는 요소다 — 문서에서 떨어졌으면 body(브라우저와 같다)
const focused = w => (w.doc.activeElement && w.doc.contains(w.doc.activeElement) ? w.doc.activeElement : w.body);
function key(w, target, k, code, mods) { return dispatch(w, target || focused(w), "keydown", Object.assign({ key: k, code: code || "", metaKey: false, ctrlKey: false, altKey: false, shiftKey: false, isComposing: false }, mods)); }
const lastStroke = eng => { let m = null; for (const s of eng.strokes.values()) if (!m || s.id > m.id) m = s; return m; };
// boardText 는 획 n 개짜리 파일 본문이다.
function boardText(C, n, o) {
  const strokes = Array.from({ length: n }, (_, i) => ({ tool: "pen", color: 0, w: 4, pts: [i * 10, 0, 0.5, i * 10 + 5, 5, 0.5] }));
  return C.serialize(Object.assign({ name: "판", bg: "dark", view: { x: 0, y: 0, z: 1 }, strokes }, o));
}

(async () => {
  // ---- 1. 순수 함수(시제품 test/wbcore.test.js 13건 중 측정용 함수를 뺀 11건) + curve
  {
    const { C } = world();
    {
      let good = true;
      for (const v of [{ x: 0, y: 0, z: 1 }, { x: -1234.5, y: 99, z: 0.1 }, { x: 1e7, y: -3e6, z: 8 }]) {
        const p = C.toWorld(v, 321.25, 77.5), s = C.toScreen(v, p.x, p.y);
        good = good && near(s.x, 321.25, 1e-6) && near(s.y, 77.5, 1e-6);
      }
      ok("core: toWorld·toScreen 은 서로 거꾸로다, 먼 좌표도", good);
    }
    {
      const v = { x: 10, y: 20, z: 1 }, w0 = C.toWorld(v, 400, 300), v2 = C.zoomAt(v, 400, 300, 2.5), w1 = C.toWorld(v2, 400, 300);
      ok("core: zoomAt 커서 아래 판의 점이 그대로다, 배율은 0.1~8", near(w0.x, w1.x) && near(w0.y, w1.y) && v2.z === 2.5 && C.zoomAt(v, 0, 0, 100).z === 8 && C.zoomAt(v, 0, 0, 0.001).z === 0.1);
    }
    {
      const v = { x: 0, y: 0, z: 4 }, p = C.toWorld(v, 100, 100), s = C.toScreen(C.panBy(v, 30, -10), p.x, p.y);
      ok("core: panBy 화면 px 만큼 판이 따라온다", near(s.x, 130) && near(s.y, 90));
    }
    {
      const v = C.snap({ x: 12.3456, y: -7.891, z: 1.7 }, 2);
      ok("core: snap 원점이 장치 픽셀에 맞는다", near((v.x * 1.7 * 2) % 1, 0, 1e-6) && near(Math.abs((v.y * 1.7 * 2) % 1), 0, 1e-6));
    }
    {
      const { poly, joins } = C.outline([0, 0, 0.2, 10, 0, 0.5, 20, 0, 1], 4, 8);
      let inside = true;
      for (let i = 1; i < poly.length; i += 2) inside = inside && Math.abs(poly[i]) <= C.penRadius(4, 1) + 1e-9;
      ok("core: outline 곧은 획의 굵기가 필압을 따른다", joins.length === 0 && near(poly[1], C.penRadius(4, 0.2)) && near(poly[3], C.penRadius(4, 0.5)) && near(poly[5], C.penRadius(4, 1))
        && C.penRadius(4, 1) > C.penRadius(4, 0.2) * 2 && poly.length / 2 === 20 && inside);
    }
    {
      const a = C.outline([5, 5, 0.5], 4, 6), b = C.outline([5, 5, 0.5, 5, 5, 0.5], 4, 6);
      ok("core: outline 점 하나는 원, 같은 점 되풀이는 하나로", a.poly.length / 2 === 12 && JSON.stringify(a.poly) === JSON.stringify(b.poly) && C.outline([], 4).poly.length === 0);
    }
    {
      const r = C.penRadius(2, 0.5), right = C.outline([0, 0, 0.5, 10, 0, 0.5, 10, 10, 0.5], 2, 8);
      const has = (x, y) => { for (let i = 0; i < right.poly.length; i += 2) if (Math.hypot(right.poly[i] - x, right.poly[i + 1] - y) < 1e-9) return true; return false; };
      const pin = C.outline([0, 0, 0.5, 10, 0, 0.5, 0, 0.5, 0.5], 2, 8);
      ok("core: outline 꺾인 곳 마이터가 굵기를 지키고 머리핀은 join 원으로 메운다", right.joins.length === 0 && has(10 - r, r) && has(10 + r, -r)
        && pin.joins.length === 3 && near(pin.joins[0], 10) && near(pin.joins[2], r) && pin.poly.every(Number.isFinite));
    }
    {
      const s = { tool: "pen", w: 4, pts: [0, 0, 0.5, 100, 0, 0.5] }, h = { tool: "hl", w: 20, pts: [0, 0, 0.5] };
      ok("core: strokeBox·hitStroke", JSON.stringify(C.strokeBox(s.pts, 3)) === "[-3,-3,103,3]" && C.hitStroke(s, 50, 4.7, 3) && !C.hitStroke(s, 50, 5, 3) && C.hitStroke(h, 0, 12, 3) && !C.hitStroke(h, 0, 14, 3));
    }
    {
      const g = new C.Grid(100), mk = (id, box) => ({ id, box });
      const a = mk(3, [0, 0, 50, 50]), b = mk(1, [150, 0, 260, 40]), c = mk(2, [-1e6, -1e6, 1e6, 1e6]), d = mk(4, [5000, 5000, 5010, 5010]);
      for (const s of [a, b, c, d]) g.insert(s);
      const ids = q => g.query(q).map(s => s.id).join();
      let good = g.big.size === 1 && ids([0, 0, 99, 99]) === "2,3" && ids([0, 0, 200, 60]) === "1,2,3" && ids([-1e5, -1e5, 1e5, 1e5]) === "1,2,3,4";
      g.remove(a); g.remove(c);
      good = good && ids([0, 0, 200, 60]) === "1" && g.size === 2;
      g.insert(mk(5, [0, 0, 350, 350]));
      ok("core: Grid 상자에 걸친 획만, 쓴 차례로, 지우면 빠진다", good && ids([0, 0, 400, 400]) === "1,5");
    }
    {
      const dpr = 2, z = 1.5, T = C.tileWorld(z, dpr, 512), v = { x: -300, y: 1000, z };
      const tiles = C.tilesFor(v, 1366, 1024, T), xs = tiles.map(t => t[0]), ys = tiles.map(t => t[1]);
      const c = C.toWorld(v, 683, 512), first = C.tileBox(tiles[0][0], tiles[0][1], T);
      ok("core: 타일 보이는 타일이 화면을 덮고 가운데부터", near(T * z * dpr, 512) && Math.min(...xs) * T <= v.x && (Math.max(...xs) + 1) * T >= v.x + 1366 / z
        && Math.min(...ys) * T <= v.y && (Math.max(...ys) + 1) * T >= v.y + 1024 / z && tiles.length === new Set(tiles.map(t => t.join())).size
        && C.boxHit(first, [c.x - T, c.y - T, c.x + T, c.y + T]));
    }
    {
      const slow = C.speedPressure(null, 0.1), fast = C.speedPressure(null, 5);
      ok("core: speedPressure 빠를수록 가늘고 범위 안", slow > fast && fast >= 0.18 && slow <= 0.8);
    }
    // curve(조정 세션 추가 지시 — 시제품 2026-11-27 판)
    {
      const pts = [0, 0, 0.2, 30, 40, 0.5, 90, 10, 0.9, 120, 80, 0.4, 121, 80, 0.4];
      const out = C.curve(pts, 2);
      let at = 0, all = true;
      for (let i = 0; i < pts.length; i += 3) {
        while (at < out.length && !(out[at] === pts[i] && out[at + 1] === pts[i + 1] && out[at + 2] === pts[i + 2])) at += 3;
        if (at >= out.length) { all = false; break; }
        at += 3;
      }
      ok("curve: 받은 점을 차례대로 전부 그대로 지난다", all && out.length > pts.length && out.length % 3 === 0, out.length);
      ok("curve: 끝점이 마지막 받은 점이다", out[out.length - 3] === 121 && out[out.length - 2] === 80 && out[out.length - 1] === 0.4);
    }
    {
      const out = C.curve([0, 0, 0.5, 100, 0, 0.5, 100, 100, 0.5], 2);
      const turn = o => { let mx = 0; for (let i = 3; i + 5 < o.length; i += 3) { const a = Math.atan2(o[i + 1] - o[i - 2], o[i] - o[i - 3]), b = Math.atan2(o[i + 4] - o[i + 1], o[i + 3] - o[i]); let d = Math.abs(b - a); if (d > Math.PI) d = 2 * Math.PI - d; mx = Math.max(mx, d); } return mx * 180 / Math.PI; };
      const mx = turn(out);
      ok("curve: 직각으로 꺾인 성긴 점에서 조각당 꺾임이 작다(< 12°, 원래 90°)", mx < 12 && turn([0, 0, 0.5, 100, 0, 0.5, 100, 100, 0.5]) > 89, mx);
      ok("curve: 조각 수는 구간마다 24 를 넘지 않는다", out.length / 3 <= 2 * 24 + 1, out.length / 3);
    }
    {
      const two = [0, 0, 0.5, 9, 9, 0.5], dup = C.curve([5, 5, 0.5, 5, 5, 0.5, 5, 5, 0.5, 10, 10, 0.5, 10, 10, 0.5], 1);
      ok("curve: 점 둘은 그대로 돌려준다", C.curve(two, 1) === two);
      ok("curve: 겹친 점이 있어도 모두 유한하다", dup.every(Number.isFinite) && C.outline(dup, 4).poly.every(Number.isFinite), dup);
      ok("curve: 점 하나·빈 점열도 그대로", C.curve([1, 2, 0.5], 1).length === 3 && C.curve([], 1).length === 0);
    }
  }

  // ---- 2. 파일 형식
  {
    const { C } = world();
    const b = { name: "수업 1", bg: "grid", view: { x: 12.3456, y: -7.891, z: 1.23456 }, strokes: [
      { tool: "pen", color: 5, w: 2.5, pts: [1.23456, 2.34567, 0.12345, 3, 4, 1] },
      { tool: "hl", color: 3, w: 24, pts: [-1e6 + 0.004, 7.005, 0.5] }] };
    const text = C.serialize(b), j = JSON.parse(text), r = C.parse(text);
    ok("형식: 머리 키", j.format === "itda-whiteboard" && j.v === 1 && j.name === "수업 1" && j.bg === "grid", j);
    ok("형식: 좌표는 둘째 자리, 필압은 셋째 자리로 반올림", JSON.stringify(j.strokes[0].p) === "[1.23,2.35,0.123,3,4,1]" && j.view.x === 12.35 && j.view.y === -7.89, j.strokes[0].p);
    ok("형식: 획은 t·c·w·p 꼴", j.strokes[1].t === "hl" && j.strokes[1].c === 3 && j.strokes[1].w === 24 && Object.keys(j.strokes[0]).join() === "t,c,w,p");
    ok("형식: 왕복 — 읽은 판을 다시 쓰면 같은 본문", r.ok && !r.readOnly && C.serialize(r.board) === text && r.board.strokes.length === 2 && r.board.strokes[1].tool === "hl", r);
    const base = () => JSON.parse(text);
    const rej = (name, mut, re) => { const o = base(); mut(o); const p = C.parse(JSON.stringify(o)); ok("검증: " + name, !p.ok && (!re || re.test(p.error)), p); };
    ok("검증: JSON 이 아니면 거부", !C.parse("{ 깨짐").ok && !C.parse("[]").ok && !C.parse("null").ok);
    rej("format 이 다르면 거부", o => { o.format = "excalidraw"; }, /format/);
    rej("v 가 정수가 아니면 거부", o => { o.v = 1.5; }, /v /);
    rej("v 가 0 이면 거부", o => { o.v = 0; });
    rej("strokes 가 배열이 아니면 거부", o => { o.strokes = {}; }, /strokes/);
    rej("색 번호가 범위 밖이면 거부", o => { o.strokes[0].c = 8; }, /c /);
    rej("색 번호가 음수면 거부", o => { o.strokes[0].c = -1; });
    rej("도구가 모르는 값이면 거부", o => { o.strokes[0].t = "laser"; }, /t /);
    rej("굵기가 0 이면 거부", o => { o.strokes[0].w = 0; }, /w /);
    rej("점 배열이 3 의 배수가 아니면 거부", o => { o.strokes[0].p.push(1); }, /p /);
    rej("점에 숫자가 아닌 값이 있으면 거부", o => { o.strokes[0].p[1] = "2"; }, /유한수/);
    rej("view 가 유한하지 않으면 거부", o => { o.view = { x: 0, y: 0, z: "1" }; }, /view/);
    rej("name 이 문자열이 아니면 거부", o => { o.name = 3; }, /name/);
    {
      const inf = text.replace('"p":[1.23', '"p":[1e999');
      const p = C.parse(inf);
      ok("검증: 무한대(1e999)는 거부", !p.ok && /유한수/.test(p.error), p);
    }
    ok("검증: 상한 상수는 획 5만·점 300만", C.LIMIT.strokes === 50000 && C.LIMIT.points === 3000000);
    ok("검증: 획 수가 상한을 넘으면 거부", !C.parse(boardText(C, 4), { strokes: 3, points: 100 }).ok && C.parse(boardText(C, 3), { strokes: 3, points: 100 }).ok);
    ok("검증: 점 수가 상한을 넘으면 거부", !C.parse(boardText(C, 4), { strokes: 10, points: 7 }).ok && C.parse(boardText(C, 4), { strokes: 10, points: 8 }).ok);
    {
      const o = base(); o.layers = [{ id: "a" }]; o.meta = { by: "뒤 판" }; o.strokes[0].future = 1;
      const p = C.parse(JSON.stringify(o)), again = JSON.parse(C.serialize(p.board));
      ok("형식: 모르는 최상위 키는 다시 쓸 때 남는다", p.ok && JSON.stringify(again.layers) === '[{"id":"a"}]' && again.meta.by === "뒤 판", again);
      ok("형식: 획 안의 모르는 키는 버린다", !("future" in again.strokes[0]));
      ok("형식: 아는 키는 extra 가 덮지 못한다", JSON.parse(C.serialize(Object.assign({}, p.board, { extra: { v: 9, format: "x", note: 1 } }))).v === 1);
    }
    {
      const o = base(); o.v = 2; o.newThing = true;
      const p = C.parse(JSON.stringify(o));
      ok("형식: v 가 더 크면 읽기 전용으로 연다", p.ok && p.readOnly && p.v === 2, p);
    }
    {
      const o = base(); o.bg = "dots"; delete o.view; o.strokes[0].p[2] = 1.7;
      const p = C.parse(JSON.stringify(o));
      ok("형식: 모르는 bg 는 dark, view 가 없으면 원점, 필압은 0~1 로 자른다", p.ok && p.board.bg === "dark" && p.board.view.z === 1 && p.board.strokes[0].pts[2] === 1, p.board);
      const z = base(); z.view.z = 50;
      ok("형식: view.z 는 0.1~8 로 자른다", C.parse(JSON.stringify(z)).board.view.z === 8);
    }
  }

  // ---- 3. 열기·표면 기본
  {
    const w = world();
    let threw = null;
    await w.W.open({}).catch(e => { threw = e; });
    ok("열기: 저장 계약이 없으면 거부하고 표면을 만들지 않는다", threw && !surfaceOf(w) && !w.W.isOpen());
    const ctl = {}, store = logged(w.W.memoryStore(), ctl);
    let opened = 0;
    w.win.ItdaWhiteboardHost = { store, onOpen() { opened++; }, onClose() { opened += 10; } };
    const id = await w.W.open();
    const root = surfaceOf(w);
    ok("열기: 목록이 비었으면 새 판을 만들고 곧바로 빈 본문을 쓴다", id === "wb1" && ctl.log.join() === "list,create:화이트보드 1,save:wb1,list", ctl.log);
    ok("열기: 쓴 본문은 읽을 수 있는 빈 판", w.C.parse(ctl.lastSave).ok && w.C.parse(ctl.lastSave).board.strokes.length === 0);
    ok("열기: 표면은 body 의 itda-wb·겹침 층 표지·onOpen 한 번", root && root.hasAttribute("data-pr-overlay") && opened === 1 && w.W.isOpen());
    ok("열기: 다시 열면 같은 Promise", w.W.open() === w.W.open());
    ok("표면: style 속성을 쓰지 않는다", root.querySelectorAll("*").every(e => !("style" in e.attrs)));
    const bar = root.querySelector(".itda-wb-bar");
    const acts = bar.kids.map(b => b.dataset.act + (b.dataset.tool || b.dataset.pop || ""));
    ok("도구 막대: 펜·형광펜·지우개·고르기·그림·색·굵기(색과 따로)·되돌리기·다시·배경·목록·닫기", acts.join() === "toolpen,toolhl,toolerase,toolselect,image,popcolor,popwidth,undo,redo,popbg,list,close", acts);
    click(w, btn(root, "pop", { pop: "color" }));
    ok("색: 팝오버에 8개", !root.querySelector(".itda-wb-pop-color").hidden && root.querySelectorAll(".itda-wb-pop-color button").length === 8);
    click(w, btn(root, "pop", { pop: "width" }));
    const wpop = root.querySelector(".itda-wb-pop-width");
    ok("굵기: 색 팝오버를 닫고 따로 연다 — 단계 3개 + 막대(펜 1~20)", root.querySelector(".itda-wb-pop-color").hidden && !wpop.hidden && wpop.querySelectorAll(".itda-wb-step").length === 3
      && wpop.querySelector(".itda-wb-slider").min === "1" && wpop.querySelector(".itda-wb-slider").max === "20");
    key(w, null, "h", "KeyH");
    ok("굵기: 형광펜은 6~80 이고 도구마다 따로 기억한다", wpop.querySelector(".itda-wb-slider").min === "6" && wpop.querySelector(".itda-wb-slider").max === "80" && wpop.querySelector(".itda-wb-slider").value === "24");
    const sl = wpop.querySelector(".itda-wb-slider");
    sl.value = "50"; dispatch(w, sl, "input", {});
    key(w, null, "p", "KeyP");
    ok("굵기: 막대로 고른 형광펜 굵기는 펜 굵기를 바꾸지 않는다", w.W.debug().prefs.width.hl === 50 && w.W.debug().prefs.width.pen === 4 && sl.value === "4");
    click(w, wpop.querySelectorAll(".itda-wb-step")[2]);
    ok("굵기: 단계 단추", w.W.debug().prefs.width.pen === 8);
    key(w, null, "Escape", "Escape");
    ok("Esc: 팝오버 먼저 닫는다(표면은 그대로)", wpop.hidden && w.W.isOpen());
    click(w, btn(root, "pop", { pop: "bg" }));
    click(w, btn(root, "bg", { bg: "light" }));
    ok("배경: 흰색으로 바꾸면 판 바탕 클래스", root.querySelector(".itda-wb-stage").classList.contains("bg-light") && w.W.debug().engine.bg() === "light");
    advance(w, 2100); await settle();
    ok("배경: 판마다 저장된다", w.C.parse(ctl.lastSave).board.bg === "light", ctl.lastSave);
    key(w, null, "Escape", "Escape");
    key(w, null, "Escape", "Escape");
    await settle();
    ok("Esc: 팝오버가 없으면 닫는다 — 표면을 떼고 onClose", !w.W.isOpen() && !surfaceOf(w) && opened === 11);
    ctl.log = [];
    await w.W.open(); await settle();
    ok("다시 열기: id 가 없으면 마지막에 쓴 판", w.W.debug().state.id === "wb1" && ctl.log.join() === "list,load:wb1", ctl.log);
    ok("다시 열기: 배경을 기억한다", w.W.debug().engine.bg() === "light");
    await w.W.close();
  }

  // ---- 4. 좌표 지킴이·입력 규칙
  {
    const w = world();
    const store = w.W.memoryStore();
    const { stage, D } = await openWb(w, store);
    const eng = D.engine;
    // coalesced 에 엉뚱한 점(0,0)
    ptr(w, stage, "pointerdown", { pointerId: 3, pointerType: "pen", clientX: 300, clientY: 300, pressure: 0.4 });
    const co = [{ clientX: 305, clientY: 300, pressure: 0.4, timeStamp: 1 }, { clientX: 0, clientY: 0, pressure: 0.4, timeStamp: 2 }, { clientX: 310, clientY: 300, pressure: 0.4, timeStamp: 3 }];
    ptr(w, stage, "pointermove", { pointerId: 3, pointerType: "pen", clientX: 310, clientY: 300, pressure: 0.4, getCoalescedEvents: () => co,
      getPredictedEvents: () => [{ clientX: 315, clientY: 300 }, { clientX: 2000, clientY: 2000 }] });
    const m = eng.mode;
    ok("지킴이: 엉뚱한 coalesced 점은 버린다", m && m.s.pts.length / 3 === 3 && !m.s.pts.some((v, i) => i % 3 < 2 && v === 0) && eng.guard.coal === 1, m && m.s.pts);
    ok("지킴이: 멀리 뛴 예측 점 앞까지만 꼬리로 쓴다", m.pred && m.pred.length === 3 && eng.guard.pred === 1, m.pred);
    ptr(w, stage, "pointerup", { pointerId: 3, pointerType: "pen", clientX: 310, clientY: 300, pressure: 0 });
    const s1 = lastStroke(eng);
    ok("지킴이: 예측 점은 획 자료에 들어가지 않는다", s1 && s1.pts.length / 3 === 3 && !s1.pts.includes(315), s1 && s1.pts);
    ok("펜: 필압을 그대로 쓴다(쓴 그대로 — 평활 없음)", s1.pts[2] === 0.4 && s1.pts[0] === 300 && s1.pts[3] === 305);
    // 마우스는 예측을 쓰지 않는다
    ptr(w, stage, "pointerdown", { pointerId: 4, pointerType: "mouse", clientX: 100, clientY: 100 });
    ptr(w, stage, "pointermove", { pointerId: 4, pointerType: "mouse", clientX: 120, clientY: 100, getPredictedEvents: () => [{ clientX: 125, clientY: 100 }] });
    ok("지킴이: 마우스는 예측 꼬리를 쓰지 않는다", eng.mode && eng.mode.pred === null && eng.mode.kind === "draw");
    ptr(w, stage, "pointerup", { pointerId: 4, pointerType: "mouse", clientX: 120, clientY: 100 });
    const s2 = lastStroke(eng);
    ok("마우스: 끌기는 그리기(속도 굵기)", eng.strokes.size === 2 && s2.pts[2] !== 0.5 && s2.pts[2] >= 0.18 && s2.pts[2] <= 0.8, s2.pts);
    // 손가락 하나 = 이동
    const v0 = eng.view;
    stroke(w, stage, line(400, 400, 460, 420, 6), { pointerId: 11, pointerType: "touch" });
    ok("손가락 하나: 이동(획 없음)", eng.strokes.size === 2 && near(eng.view.x, v0.x - 60) && near(eng.view.y, v0.y - 20), eng.view);
    // 손가락 둘 = 오므리기
    const v1 = eng.view, mid = C0 => C0.toWorld(v1, 500, 400);
    const wMid = mid(w.C);
    ptr(w, stage, "pointerdown", { pointerId: 21, pointerType: "touch", clientX: 450, clientY: 400 });
    ptr(w, stage, "pointerdown", { pointerId: 22, pointerType: "touch", clientX: 550, clientY: 400 });
    ptr(w, stage, "pointermove", { pointerId: 22, pointerType: "touch", clientX: 650, clientY: 400 });
    ptr(w, stage, "pointermove", { pointerId: 21, pointerType: "touch", clientX: 350, clientY: 400 });
    const after = w.C.toWorld(eng.view, 500, 400);
    ok("손가락 둘: 오므려 확대 — 가운데 판 점이 그대로, 배율 3", near(eng.view.z, 3, 1e-9) && near(after.x, wMid.x, 1e-6) && near(after.y, wMid.y, 1e-6) && eng.strokes.size === 2, eng.view);
    ptr(w, stage, "pointerup", { pointerId: 21, pointerType: "touch", clientX: 350, clientY: 400 });
    ok("손가락 둘: 하나를 떼면 남은 손가락으로 이동을 잇는다", eng.mode && eng.mode.kind === "pan");
    ptr(w, stage, "pointerup", { pointerId: 22, pointerType: "touch", clientX: 650, clientY: 400 });
    // 손바닥 무시
    const zv = eng.view;
    ptr(w, stage, "pointerdown", { pointerId: 30, pointerType: "pen", clientX: 200, clientY: 200 });
    ptr(w, stage, "pointerdown", { pointerId: 31, pointerType: "touch", clientX: 600, clientY: 600 });
    ptr(w, stage, "pointermove", { pointerId: 31, pointerType: "touch", clientX: 700, clientY: 650 });
    ptr(w, stage, "pointermove", { pointerId: 30, pointerType: "pen", clientX: 240, clientY: 200 });
    ptr(w, stage, "pointerup", { pointerId: 31, pointerType: "touch", clientX: 700, clientY: 650 });
    ptr(w, stage, "pointermove", { pointerId: 30, pointerType: "pen", clientX: 280, clientY: 200 });
    ptr(w, stage, "pointerup", { pointerId: 30, pointerType: "pen", clientX: 280, clientY: 200 });
    const s3 = lastStroke(eng);
    ok("손바닥: 펜으로 쓰는 중 닿은 손가락은 획을 끊지도 판을 움직이지도 않는다", eng.strokes.size === 3 && s3.pts.length / 3 === 3 && eng.view === zv, s3.pts);
    ptr(w, stage, "pointerdown", { pointerId: 40, pointerType: "touch", clientX: 600, clientY: 600 });
    ptr(w, stage, "pointerdown", { pointerId: 41, pointerType: "pen", clientX: 200, clientY: 500 });
    ptr(w, stage, "pointermove", { pointerId: 40, pointerType: "touch", clientX: 680, clientY: 600 });
    ptr(w, stage, "pointermove", { pointerId: 41, pointerType: "pen", clientX: 240, clientY: 500 });
    ptr(w, stage, "pointerup", { pointerId: 41, pointerType: "pen", clientX: 240, clientY: 500 });
    ok("손바닥: 펜이 닿은 순간 이미 닿아 있던 손가락은 뗄 때까지 무시", eng.strokes.size === 4 && eng.view === zv);
    ptr(w, stage, "pointerdown", { pointerId: 42, pointerType: "touch", clientX: 300, clientY: 300 });
    ok("손바닥: 무시한 손가락은 새 손가락과 오므리기를 만들지 않는다(한 손가락 이동)", eng.mode && eng.mode.kind === "pan", eng.mode && eng.mode.kind);
    ptr(w, stage, "pointerup", { pointerId: 42, pointerType: "touch", clientX: 300, clientY: 300 });
    ptr(w, stage, "pointerup", { pointerId: 40, pointerType: "touch", clientX: 680, clientY: 600 });
    // 지우개·되돌리기
    key(w, null, "e", "KeyE");
    const target = lastStroke(eng), sp = w.C.toScreen(eng.view, target.pts[0], target.pts[1]);
    stroke(w, stage, [[sp.x, sp.y], [sp.x + 2, sp.y]], { pointerId: 50, pointerType: "pen" });
    ok("지우개: 스친 획만 획 단위로 지운다", eng.strokes.size === 3 && !eng.strokes.has(target.id));
    key(w, null, "z", "KeyZ", { metaKey: true });
    ok("⌘Z: 지운 획을 되돌린다", eng.strokes.size === 4 && eng.strokes.has(target.id));
    key(w, null, "z", "KeyZ", { metaKey: true, shiftKey: true });
    ok("⇧⌘Z: 다시 하기", eng.strokes.size === 3);
    key(w, null, "ㅔ", "KeyP");
    ok("키: 한글 입력 상태에서도 e.code 로 도구를 바꾼다", w.W.debug().prefs.tool === "pen");
    // 휠·확대
    const vb = eng.view;
    dispatch(w, stage, "wheel", { deltaX: 0, deltaY: 30, deltaMode: 0, clientX: 100, clientY: 100 });
    ok("휠: 이동", near(eng.view.y, vb.y + 30 / vb.z) && eng.view.z === vb.z);
    const wz = w.C.toWorld(eng.view, 640, 360);
    const we = dispatch(w, stage, "wheel", { deltaX: 0, deltaY: -50, deltaMode: 0, clientX: 640, clientY: 360, ctrlKey: true });
    const wz2 = w.C.toWorld(eng.view, 640, 360);
    ok("⌃휠: 커서 기준 확대(기본 동작 막음)", eng.view.z > vb.z && near(wz.x, wz2.x, 1e-6) && we.dp);
    // Space+끌기
    key(w, null, " ", "Space");
    const vs = eng.view, n0 = eng.strokes.size;
    stroke(w, stage, line(100, 100, 150, 100, 5), { pointerId: 60, pointerType: "mouse" });
    dispatch(w, w.doc.activeElement, "keyup", { key: " ", code: "Space" });
    ok("Space+끌기: 이동(획 없음)", eng.strokes.size === n0 && eng.view.x < vs.x);
    await w.W.close();
  }

  // ---- 5. 자동 저장
  {
    const w = world();
    const ctl = {}, store = logged(w.W.memoryStore(), ctl);
    const { stage, D } = await openWb(w, store);
    const eng = D.engine;
    ctl.log = [];
    stroke(w, stage, line(100, 100, 200, 100, 5));
    advance(w, 1900); await settle();
    ok("자동 저장: 바뀐 뒤 2초 전에는 저장하지 않는다", ctl.log.length === 0, ctl.log);
    stroke(w, stage, line(100, 200, 200, 200, 5));
    advance(w, 1900); await settle();
    ok("자동 저장: 다시 바뀌면 2초를 다시 센다", ctl.log.length === 0, ctl.log);
    advance(w, 200); await settle();
    ok("자동 저장: 조용한 2초 뒤 한 번", ctl.log.join() === "save:wb1" && w.C.parse(ctl.lastSave).board.strokes.length === 2, ctl.log);
    const it = D.state.list.find(x => x.id === "wb1");
    ok("자동 저장: 목록의 고친 때를 save 가 돌려준 값으로 고친다", it && it.modified > 0 && it.bytes === ctl.lastSave.length);
    // 보기만 바뀌면 저장하지 않는다
    ctl.log = [];
    dispatch(w, stage, "wheel", { deltaX: 0, deltaY: 30, deltaMode: 0, clientX: 1, clientY: 1 });
    advance(w, 5000); await settle();
    ok("자동 저장: 보기(이동)만 바뀐 것은 저장을 부르지 않는다", ctl.log.length === 0, ctl.log);
    // 저장 중 또 바뀜
    ctl.hold.save = true;
    stroke(w, stage, line(100, 300, 200, 300, 5));
    advance(w, 2100); await settle();
    ok("저장 중: 첫 저장이 붙들려 있다", ctl.log.join() === "save:wb1" && ctl.waiting.length === 1, ctl.log);
    const firstText = ctl.lastSave;
    stroke(w, stage, line(100, 400, 200, 400, 5));
    ctl.hold.save = false;
    ctl.release(); await settle();
    ok("저장 중: 끝난 뒤 곧바로 한 번 더(2초를 기다리지 않는다)", ctl.log.join() === "save:wb1,save:wb1" && w.C.parse(ctl.lastSave).board.strokes.length === 4 && w.C.parse(firstText).board.strokes.length === 3, ctl.log);
    ok("저장 중: 처음 보기 변경도 함께 실렸다", w.C.parse(firstText).board.view.y !== 0);
    advance(w, 2100); await settle();
    ok("저장 중: 남은 타이머는 할 일이 없다", ctl.log.length === 2, ctl.log);
    // 실패 뒤 재시도
    ctl.log = []; ctl.fail.save = true;
    stroke(w, stage, line(100, 500, 200, 500, 5));
    advance(w, 2100); await settle();
    const note = D.root.querySelector(".itda-wb-note");
    ok("실패: 표면 위 한 줄 안내, 판은 그대로", ctl.log.join() === "save:wb1" && !note.hidden && /저장하지 못했습니다/.test(note.textContent) && eng.strokes.size === 5, ctl.log);
    advance(w, 10000); await settle();
    ok("실패: 바뀐 것이 없으면 스스로 되풀이하지 않는다", ctl.log.length === 1, ctl.log);
    ctl.fail.save = false;
    stroke(w, stage, line(100, 600, 200, 600, 5));
    advance(w, 2100); await settle();
    ok("실패: 다음 변경 때 다시 저장하고 안내를 거둔다", ctl.log.join() === "save:wb1,save:wb1" && w.C.parse(ctl.lastSave).board.strokes.length === 6 && note.hidden, ctl.log);
    // visibilitychange·pagehide
    ctl.log = [];
    stroke(w, stage, line(300, 100, 400, 100, 5));
    w.doc.visibilityState = "hidden";
    dispatch(w, w.doc, "visibilitychange", {});
    await settle();
    ok("숨김: visibilitychange hidden 이면 곧바로 저장", ctl.log.join() === "save:wb1", ctl.log);
    w.doc.visibilityState = "visible";
    stroke(w, stage, line(300, 200, 400, 200, 5));
    dispatch(w, w.win, "pagehide", {});
    await settle();
    ok("숨김: pagehide 때도 저장", ctl.log.join() === "save:wb1,save:wb1", ctl.log);
    // 닫기: 저장 실패 → 처음은 닫지 않음 → 두 번째는 메모리에 남기고 닫음 → 다시 열면 그 본문
    ctl.log = []; ctl.fail.save = true;
    stroke(w, stage, line(300, 300, 400, 300, 5));
    const r1 = await w.W.close();
    ok("닫기: 저장이 실패하면 처음 한 번은 닫지 않고 안내한다", r1 === false && w.W.isOpen() && /닫지 않았습니다/.test(note.textContent), ctl.log);
    const r2 = await w.W.close();
    ok("닫기: 한 번 더 닫으면 메모리에 남긴 채 닫는다", r2 === true && !w.W.isOpen() && ctl.log.join() === "save:wb1,save:wb1", ctl.log);
    ctl.fail.save = false; ctl.log = [];
    await w.W.open(); await settle();
    ok("다시 열기: 메모리의 본문으로 열고 곧바로 다시 저장한다", w.W.debug().engine.strokes.size === 9 && ctl.log.join() === "list,save:wb1", ctl.log);
    await w.W.close();
  }

  // ---- 6. 판 목록: 전환·새 판·복제·이름·삭제의 store 차례
  {
    const w = world();
    const mem = w.W.memoryStore();
    await mem.create("가"); await mem.save("wb1", boardText(w.C, 1, { name: "가" }));
    await mem.create("나"); await mem.save("wb2", boardText(w.C, 2, { name: "나" }));
    const ctl = {}, store = logged(mem, ctl);
    const { root, stage, D } = await openWb(w, store);
    const st = D.state, eng = D.engine;
    ok("목록: 고친 때가 가장 늦은 판을 연다", st.id === "wb2" && eng.strokes.size === 2 && ctl.log.join() === "list,load:wb2", ctl.log);
    click(w, btn(root, "list"));
    await settle();
    const rows = () => root.querySelectorAll(".itda-wb-item");
    ok("목록: 판마다 미리보기 canvas·이름·고친 때, 고친 때 내림차순", rows().length === 2 && rows()[0].dataset.id === "wb2" && rows()[0].querySelector(".itda-wb-thumb") && rows()[1].querySelector(".itda-wb-iname").textContent === "가"
      && /월 .*일/.test(rows()[0].querySelector(".itda-wb-itime").textContent), rows().map(r => r.dataset.id));
    ok("목록: 지금 판 표시", rows()[0].classList.contains("on"));
    ok("목록: 다른 판의 미리보기는 그 본문을 읽어 그린다", ctl.log.includes("load:wb1"), ctl.log);
    // 전환
    stroke(w, stage, line(100, 100, 200, 100, 5));
    ctl.log = [];
    click(w, rows()[1].querySelector(".itda-wb-iname"));
    await settle();
    ok("전환: 지금 판을 저장한 뒤 고른 판을 읽는다", ctl.log.join() === "save:wb2,load:wb1" && st.id === "wb1" && eng.strokes.size === 1 && root.querySelector(".itda-wb-list").hidden, ctl.log);
    ok("전환: 저장한 본문에 새 획", w.C.parse(ctl.lastSave).board.strokes.length === 3);
    ok("전환: 되돌리기는 판마다 비운다", !eng.canUndo());
    // [ ] 이전·다음
    ctl.log = [];
    key(w, null, "]", "BracketRight");
    await settle();
    ok("] : 목록 차례의 다음 판(바뀐 것 없으면 저장 없이 읽기만)", st.id === "wb2" && ctl.log.join() === "load:wb2", ctl.log);
    key(w, null, "[", "BracketLeft");
    await settle();
    ok("[ : 이전 판", st.id === "wb1");
    // 새 판
    click(w, btn(root, "list")); await settle();
    ctl.log = [];
    click(w, btn(root, "new"));
    await settle();
    ok("새 판: 목록 → create → 빈 본문 save → 목록", ctl.log.join() === "list,create:화이트보드 3,save:wb3,list" && st.id === "wb3" && eng.strokes.size === 0, ctl.log);
    // 복제(지금 판, 바뀐 것 있음)
    stroke(w, stage, line(100, 100, 200, 100, 5));
    ctl.log = [];
    const dupBtn = () => rows().find(r => r.dataset.id === "wb3").querySelectorAll("button").find(b => b.dataset.act === "dup");
    click(w, dupBtn());
    await settle();
    ok("복제: 지금 판이면 저장 → duplicate → 목록", ops(ctl) === "save:wb3,duplicate:wb3,list" && st.id === "wb3", ctl.log);
    ok("복제: 사본이 목록에 있다", rows().some(r => r.querySelector(".itda-wb-iname") && r.querySelector(".itda-wb-iname").textContent === "화이트보드 3 사본"));
    // 이름 바꾸기
    ctl.log = [];
    const row1 = () => rows().find(r => r.dataset.id === "wb1");
    click(w, row1().querySelectorAll("button").find(b => b.dataset.act === "rename"));
    const inp = row1().querySelector(".itda-wb-rename");
    ok("이름: 입력 칸이 열리고 포커스를 받는다", inp && w.doc.activeElement === inp);
    const kd = key(w, inp, "a", "KeyA");
    ok("이름: 입력 칸의 글자 키는 기본 동작 그대로(막지 않음)", !kd.dp && w.W.debug().prefs.tool === "pen");
    key(w, inp, "Escape", "Escape");
    ok("이름: Esc 는 고치기만 거두고 목록은 그대로", !row1().querySelector(".itda-wb-rename") && !root.querySelector(".itda-wb-list").hidden);
    click(w, row1().querySelectorAll("button").find(b => b.dataset.act === "rename"));
    const inp2 = row1().querySelector(".itda-wb-rename");
    inp2.value = "  새 이름  ";
    key(w, inp2, "Enter", "Enter");
    await settle();
    ok("이름: 다른 판이면 rename → 목록(저장 없음)", ops(ctl) === "rename:wb1,list" && row1().querySelector(".itda-wb-iname").textContent === "새 이름", ctl.log);
    ctl.log = [];
    const row3 = () => rows().find(r => r.dataset.id === "wb3");
    click(w, row3().querySelectorAll("button").find(b => b.dataset.act === "rename"));
    const inp3 = row3().querySelector(".itda-wb-rename");
    inp3.value = "지금 판";
    key(w, inp3, "Enter", "Enter");
    await settle();
    advance(w, 2100); await settle();
    ok("이름: 지금 판이면 rename → 본문 name 을 곧바로 저장 → 목록", ops(ctl) === "rename:wb3,save:wb3,list" && w.C.parse(ctl.lastSave).board.name === "지금 판", ctl.log);
    // 삭제(확인 한 번)
    ctl.log = [];
    const del = () => row1().querySelectorAll("button").find(b => b.dataset.act === "trash");
    click(w, del());
    ok("삭제: 처음 누르기는 확인만 묻는다", ctl.log.length === 0 && del().textContent === "정말 삭제");
    key(w, null, "Escape", "Escape");
    ok("삭제: Esc 는 확인을 거둔다(목록은 그대로)", del().textContent === "삭제" && !root.querySelector(".itda-wb-list").hidden);
    click(w, del()); click(w, del());
    await settle();
    ok("삭제: 확인 뒤 trash → 목록", ops(ctl) === "trash:wb1,list" && !row1(), ctl.log);
    // 지금 판 삭제가 실패하면 판은 그대로이고 바뀐 것은 다시 저장한다
    const d3 = () => row3().querySelectorAll("button").find(b => b.dataset.act === "trash");
    stroke(w, stage, line(100, 250, 200, 250, 5));
    ctl.log = []; ctl.fail.trash = true;
    click(w, d3()); click(w, d3());
    await settle();
    advance(w, 2100); await settle();
    ok("삭제 실패: 안내, 판 그대로, 바뀐 것은 저장한다", st.id === "wb3" && /삭제하지 못했습니다/.test(root.querySelector(".itda-wb-note").textContent) && ops(ctl) === "trash:wb3,save:wb3", ctl.log);
    ctl.fail.trash = false;
    // 지금 판 삭제 → 저장하지 않고 다음 판을 연다(지우는 동안 그린 것도 지운 판에 저장하지 않는다)
    stroke(w, stage, line(100, 300, 200, 300, 5));
    ctl.log = []; ctl.hold.trash = true;
    click(w, d3()); click(w, d3());
    await settle();
    stroke(w, stage, line(100, 350, 200, 350, 5));
    ctl.hold.trash = false; ctl.hold.list = true; ctl.release();
    await settle();
    advance(w, 2100); await settle(); // 목록을 다시 읽는 동안 저장 타이머가 돈다
    ctl.hold.list = false; ctl.release();
    await settle();
    advance(w, 3000); await settle();
    ok("삭제: 지우는 동안 그린 것도 지운 판에 저장하지 않는다", !ctl.log.includes("save:wb3") && st.id !== "wb3", ctl.log);
    ok("삭제: 지금 판이면 저장하지 않고 trash → 목록 → 남은 판 중 가장 늦은 판", ctl.log[0] === "trash:wb3" && ctl.log[1] === "list" && /^load:/.test(ctl.log[2]) && !ctl.log.includes("save:wb3") && st.id !== "wb3", ctl.log);
    // 목록 Esc
    key(w, null, "Escape", "Escape");
    ok("Esc: 목록이 열렸으면 목록만 닫는다", root.querySelector(".itda-wb-list").hidden && w.W.isOpen());
    // 마지막 판 삭제 → 새 판
    ctl.log = [];
    for (const it of [...st.list]) if (it.id !== st.id) await mem.trash(it.id);
    click(w, btn(root, "list")); await settle();
    const only = rows()[0], dl = () => only.querySelectorAll("button").find(b => b.dataset.act === "trash");
    click(w, dl()); click(w, root.querySelectorAll(".itda-wb-item")[0].querySelectorAll("button").find(b => b.dataset.act === "trash"));
    await settle();
    ok("삭제: 마지막 판을 지우면 새 판을 만든다", ctl.log.some(x => /^create:/.test(x)) && st.list.length === 1 && eng.strokes.size === 0, ctl.log);
    // 읽기 전용(큰 v)
    const v2 = JSON.parse(boardText(w.C, 2)); v2.v = 3;
    const r = await mem.create("미래"); await mem.save(r.id, JSON.stringify(v2));
    ctl.log = [];
    await w.W.close();
    await w.W.open({ id: r.id }); await settle();
    const D2 = w.W.debug(), st2 = D2.root.querySelector(".itda-wb-stage");
    stroke(w, st2, line(100, 100, 200, 100, 5));
    advance(w, 3000); await settle();
    ok("읽기 전용: 큰 v 는 안내를 보이고 그리지 않는다(끌기는 이동)", D2.engine.readOnly() && D2.engine.strokes.size === 2 && /읽기만/.test(D2.root.querySelector(".itda-wb-note").textContent) && D2.engine.view.x !== 0);
    await w.W.close();
    ok("읽기 전용: 닫아도 저장하지 않는다", !ctl.log.some(x => x === "save:" + r.id), ctl.log);
    // 읽지 못하는 본문
    const bad = await mem.create("깨짐"); await mem.save(bad.id, "{ 깨진 본문");
    ctl.log = [];
    await w.W.open({ id: bad.id }); await settle();
    ok("읽기 실패: 읽기 전용으로 열고 고쳐 쓰지 않는다", w.W.debug().engine.readOnly() && /읽지 못해/.test(w.W.debug().root.querySelector(".itda-wb-note").textContent));
    await w.W.close();
    ok("읽기 실패: 닫아도 저장하지 않는다", !ctl.log.some(x => x === "save:" + bad.id), ctl.log);
  }

  // ---- 7. 열린 동안 아래 화면은 키·포인터를 받지 않는다
  {
    const w = world();
    const below = { key: 0, keyCap: 0, down: 0, click: 0, win: 0 };
    w.doc.addEventListener("keydown", () => below.key++);
    w.doc.addEventListener("keydown", () => below.keyCap++, true);
    w.doc.addEventListener("pointerdown", () => below.down++);
    w.doc.addEventListener("click", () => below.click++, true);
    w.win.addEventListener("keydown", () => below.win++);
    const page = w.body.appendChild(new El("div"));
    page.addEventListener("keydown", () => below.key++);
    const { root, stage } = await openWb(w, w.W.memoryStore());
    for (const [k, c] of [["ArrowRight", "ArrowRight"], ["p", "KeyP"], [" ", "Space"], ["z", "KeyZ"], ["Escape", "Escape"]]) {
      if (k === "Escape") break;
      key(w, page, k, c);
    }
    key(w, page, "q", "KeyQ");
    stroke(w, stage, line(100, 100, 200, 100, 4));
    click(w, btn(root, "pop", { pop: "color" }));
    dispatch(w, w.doc.activeElement, "keyup", { key: " ", code: "Space" });
    ok("아래 화면: 키(붙잡기·거품, 문서·창·페이지 요소)를 받지 않는다", below.key === 0 && below.keyCap === 0 && below.win === 0, below);
    ok("아래 화면: 표면 안 포인터·클릭을 받지 않는다", below.down === 0 && below.click === 0, below);
    ok("표면: 창 붙잡기 단계에 키 처리기를 단다", w.win.listeners("keydown", true) === 1);
    const pk = key(w, page, "ArrowDown", "ArrowDown");
    ok("아래 화면: 굴림 키의 기본 동작도 막는다", pk.dp && pk.stopped);
    await w.W.close();
    key(w, page, "ArrowRight", "ArrowRight");
    ok("닫은 뒤: 아래 화면이 다시 키를 받고 표면의 처리기는 모두 떨어진다", below.key === 2 && below.keyCap === 1 && w.win.listeners("keydown") === 1 && w.win.listeners("pointerdown") === 0 && w.doc.listeners("visibilitychange") === 0, below);
    // W 로 닫기
    await openWb(w, w.W.memoryStore());
    key(w, null, "w", "KeyW"); await settle();
    ok("W: 닫는다", !w.W.isOpen());
    await openWb(w, w.W.memoryStore());
    key(w, null, "ㅈ", "KeyW"); await settle();
    ok("W: 한글 입력 상태(ㅈ)에서도 닫는다", !w.W.isOpen());
    await openWb(w, w.W.memoryStore());
    key(w, null, "w", "KeyW", { metaKey: true }); await settle();
    ok("W: ⌘W 는 닫지 않는다", w.W.isOpen());
    await w.W.toggle();
    ok("toggle: 열렸으면 닫는다", !w.W.isOpen());
    await w.W.toggle({ host: { store: w.W.memoryStore() } }); await settle();
    ok("toggle: 닫혔으면 연다(host 를 직접 줄 수 있다)", w.W.isOpen());
    await w.W.close();
    // host.mount
    const box = w.body.appendChild(new El("div"));
    w.win.ItdaWhiteboardHost = { store: w.W.memoryStore(), mount: box };
    await w.W.open(); await settle();
    ok("mount: 호스트가 준 요소에 붙인다", box.kids.some(k => k.classList.contains("itda-wb")) && !surfaceOf(w));
    await w.W.close();
  }

  // ---- 8. 발표의 W(present.js)
  {
    const files = ["inkshape.js", "ink.js", "present.js", "wbcore.js", "whiteboard.js"];
    const slide = () => { const p = new El("div"); p.className = "page"; p.dataset.kind = "content"; p.appendChild(new El("div")).className = "page-body"; return p; };
    const start = (w, o) => w.win.ItdaPresent.start(Object.assign({ pages: [slide(), slide(), slide()], notes: [], index: 0, ratio: "16:9", onExit() {} }, o));
    {
      const w = world(files), H = w.win.ItdaPresent;
      start(w);
      key(w, null, "w", "KeyW");
      ok("발표 W: 계약이 없으면 아무 일도 없다(표면 없음·장 그대로)", !w.W.isOpen() && !surfaceOf(w) && H.state.idx === 0);
      ok("발표 W: whiteboard() 도 false", H.whiteboard() === false);
      const ctl = {};
      w.win.ItdaWhiteboardHost = { store: logged(w.W.memoryStore(), ctl) };
      key(w, null, "p", "KeyP");
      ok("발표 W: (준비) 판서 펜을 켠다", H.ink().tool() === "pen" && H.ink().takes({ button: 0 }) === "draw");
      const sl0 = H.state.slides[0];
      ptr(w, sl0, "pointerdown", { pointerId: 90, pointerType: "mouse", clientX: 300, clientY: 300 });
      ptr(w, sl0, "pointermove", { pointerId: 90, pointerType: "mouse", clientX: 360, clientY: 320 });
      ok("발표 W: (준비) 판서 획을 그리는 중", H.ink().drawing());
      const e = key(w, null, "w", "KeyW");
      ok("발표 W: 여는 순간 그리던 판서 획은 거기까지 확정한다", !H.ink().drawing() && w.win.ItdaInk.pageOf(H.ink().book, 0).list.length === 1);
      await settle(); frames(w, 2);
      const root = surfaceOf(w);
      ok("발표 W: 계약이 있으면 화이트보드를 연다", w.W.isOpen() && root && e.dp && ctl.log[0] === "list", ctl.log);
      ok("발표 W: 표면은 겹침 층(발표 중에도 보인다)", root.hasAttribute("data-pr-overlay") && w.body.kids.some(k => k.id === "present"));
      key(w, null, "ArrowRight", "ArrowRight");
      key(w, null, "p", "KeyP");
      ok("발표 W: 열린 동안 발표의 넘김·판서 키는 쉰다(P 가 판서 펜을 끄지 않는다)", H.state.idx === 0 && H.ink().tool() === "pen", H.ink().tool());
      const pres = w.body.kids.find(k => k.id === "present");
      let presDown = 0;
      pres.addEventListener("mousedown", () => presDown++);
      dispatch(w, root.querySelector(".itda-wb-stage"), "mousedown", { button: 0, clientX: 1200, clientY: 100 });
      ok("발표 W: 열린 동안 표면의 누르기는 발표에 가지 않는다", presDown === 0 && H.state.idx === 0);
      ok("발표 W: 열린 동안 판서는 쉬는 화면으로 본다", H.ink().takes({ button: 0 }) === "");
      key(w, null, "w", "KeyW"); await settle();
      ok("발표 W: 다시 W 는 화이트보드를 닫고 발표는 그대로", !w.W.isOpen() && H.state.on && H.state.idx === 0);
      ok("발표 W: 닫은 뒤 판서가 다시 누르기를 받는다", H.ink().takes({ button: 0 }) === "draw");
      key(w, null, "p", "KeyP");
      key(w, null, "ArrowRight", "ArrowRight");
      ok("발표 W: 닫은 뒤 발표 키가 다시 돈다", H.state.idx === 1);
      key(w, null, "b", "KeyB");
      key(w, null, "w", "KeyW");
      ok("발표 W: 검은 화면에서는 열지 않는다", !w.W.isOpen());
      key(w, null, "b", "KeyB");
      key(w, null, "w", "KeyW", { metaKey: true });
      ok("발표 W: 수식키가 있으면 열지 않는다", !w.W.isOpen());
      key(w, null, "ㅈ", "KeyW");
      await settle();
      ok("발표 W: 한글 입력 상태(ㅈ)에서도 연다", w.W.isOpen());
      await w.W.close();
      ok("발표 W: 공개 함수 whiteboard() 로 연다", H.whiteboard() === true && w.W.isOpen());
      await settle();
      await w.W.close();
      H.stop();
      ok("발표 W: 발표 밖에서는 whiteboard() 가 false", H.whiteboard() === false && !w.W.isOpen());
    }
    {
      const w = world(files), H = w.win.ItdaPresent;
      w.win.ItdaWhiteboardHost = { store: w.W.memoryStore() };
      start(w, { audience: true });
      key(w, null, "w", "KeyW");
      ok("발표 W: 청중 화면에서는 열지 않는다", !w.W.isOpen() && H.whiteboard() === false);
      H.stop();
    }
    {
      const w = world(files), H = w.win.ItdaPresent;
      w.win.ItdaWhiteboardHost = { store: w.W.memoryStore() };
      const pg = new El("div"); pg.className = "page board"; pg.appendChild(new El("div")).className = "page-body";
      H.startBoard({ page: pg, ratio: "16:9", onExit() {} });
      key(w, null, "w", "KeyW");
      await settle();
      ok("발표 W: 보드 발표에서도 연다", w.W.isOpen() && H.state.board);
      await w.W.close();
      H.stop();
    }
    {
      const w = world(["inkshape.js", "ink.js", "present.js"]), H = w.win.ItdaPresent;
      w.win.ItdaWhiteboardHost = { store: {} };
      start(w);
      key(w, null, "w", "KeyW");
      ok("발표 W: whiteboard.js 가 없으면 아무 일도 없다", H.state.on && H.whiteboard() === false);
      H.stop();
    }
  }

  // ---- 9. 리뷰 1회차 반영: 잠금·진행 중 획 확정·저장소 구분·복제·기다림 끝·수치 범위·굵기 왕복·발표 조건·미리보기 캐시·획 점 상한
  const strokesIn = async (w, mem, id) => w.C.parse(await mem.load(id)).board.strokes.length;
  const penDown = (w, stage, x, y) => { ptr(w, stage, "pointerdown", { pointerId: 7, pointerType: "pen", clientX: x, clientY: y }); w.clock += 4; ptr(w, stage, "pointermove", { pointerId: 7, pointerType: "pen", clientX: x + 30, clientY: y + 10 }); };
  async function two(w) { // 판 둘(wb1 1획·wb2 2획 — wb2 가 더 늦다)
    const mem = w.W.memoryStore();
    await mem.create("가"); await mem.save("wb1", boardText(w.C, 1, { name: "가" }));
    await mem.create("나"); await mem.save("wb2", boardText(w.C, 2, { name: "나" }));
    return mem;
  }
  // [1] 전환 중에는 잠근다 — load 를 기다리는 동안 그은 획은 처음부터 그려지지 않는다(이동은 된다)
  {
    const w = world(), mem = await two(w), ctl = {}, store = logged(mem, ctl);
    const { stage, root, D } = await openWb(w, store, { id: "wb1" });
    const eng = D.engine, st = D.state, busy = root.querySelector(".itda-wb-busy");
    ok("잠금: 처음 판이 뜨면 풀려 있다", !eng.locked() && busy.hidden);
    stroke(w, stage, line(100, 500, 200, 500, 4)); // 되돌릴 것이 있게
    ctl.hold.load = true;
    key(w, null, "]", "BracketRight"); await settle();
    ok("잠금: 전환이 load 를 기다리는 동안 잠그고 「여는 중」 을 보인다", eng.locked() && !busy.hidden && /여는 중/.test(busy.textContent) && st.id === "wb1", ctl.log);
    const v0 = eng.view.x;
    stroke(w, stage, [[100, 100], [200, 150], [260, 200]]);
    ok("잠금: 그은 펜은 그리지 않고 이동한다", eng.strokes.size === 2 && eng.view.x !== v0, eng.strokes.size);
    ptr(w, stage, "pointerdown", { pointerId: 2, pointerType: "mouse", clientX: 300, clientY: 300 });
    ptr(w, stage, "pointermove", { pointerId: 2, pointerType: "mouse", clientX: 340, clientY: 330 });
    ptr(w, stage, "pointerup", { pointerId: 2, pointerType: "mouse", clientX: 340, clientY: 330 });
    ok("잠금: 마우스 끌기도 그리지 않는다", eng.strokes.size === 2);
    click(w, btn(root, "pop", { pop: "bg" })); click(w, btn(root, "bg", { bg: "light" }));
    key(w, null, "z", "KeyZ", { metaKey: true });
    ok("잠금: 배경 바꾸기·되돌리기를 받지 않는다", eng.bg() === "dark" && eng.strokes.size === 2 && btn(root, "undo").disabled && btn(root, "pop", { pop: "bg" }).disabled);
    ctl.hold.load = false; ctl.release(); await settle();
    ok("잠금: 새 판이 뜨면 푼다", st.id === "wb2" && eng.strokes.size === 2 && !eng.locked() && busy.hidden && !btn(root, "pop", { pop: "bg" }).disabled);
    ok("잠금: 떠난 판에는 잠그기 전 획만 있다(그리다 사라지는 획이 없다)", await strokesIn(w, mem, "wb1") === 2 && !D.dirty("wb1"));
    // load 실패 → 원래 판으로 돌아온다
    stroke(w, stage, line(100, 300, 200, 300, 4));
    ctl.fail.load = true;
    key(w, null, "[", "BracketLeft"); await settle();
    ok("잠금: load 가 실패하면 원래 판으로 돌아와 잠금을 풀고 안내한다", st.id === "wb2" && eng.strokes.size === 3 && !eng.locked() && busy.hidden
      && /원래 판으로/.test(root.querySelector(".itda-wb-note").textContent), ctl.log);
    ctl.fail.load = false;
    // load 가 끝나지 않으면 8초 뒤 원래 판으로
    ctl.hold.load = true;
    key(w, null, "[", "BracketLeft"); await settle();
    advance(w, 7900); await settle();
    ok("잠금: 끝나지 않는 load — 8초 전에는 잠근 채 기다린다", eng.locked() && st.id === "wb2");
    advance(w, 200); await settle();
    ok("잠금: 끝나지 않는 load — 8초가 지나면 원래 판으로 돌아와 안내한다", !eng.locked() && st.id === "wb2" && eng.strokes.size === 3 && /원래 판으로/.test(root.querySelector(".itda-wb-note").textContent));
    stroke(w, stage, line(100, 400, 200, 400, 4));
    ctl.hold.load = false; ctl.release(); await settle();
    ok("잠금: 버린 load 가 늦게 끝나도 판을 바꾸지 않는다", st.id === "wb2" && eng.strokes.size === 4);
    // 새 판 — create 를 기다리는 동안 잠그고, 실패하면 원래 판
    ctl.hold.create = true;
    click(w, btn(root, "list")); await settle();
    click(w, btn(root, "new")); await settle();
    ok("잠금: 새 판 create 를 기다리는 동안도 잠근다", eng.locked() && st.id === "wb2");
    ctl.hold.create = false; ctl.release(); await settle();
    ok("잠금: create 가 끝나면 새 판을 보이고 푼다", st.id === "wb3" && !eng.locked());
    ctl.fail.create = true;
    click(w, btn(root, "new")); await settle();
    ok("잠금: create 가 실패하면 원래 판으로 돌아와 푼다", st.id === "wb3" && !eng.locked() && /원래 판으로/.test(root.querySelector(".itda-wb-note").textContent));
    ctl.fail.create = false;
    // 지금 판 삭제 — trash 를 기다리는 동안 잠근다(지울 판에 그리지 않는다)
    const n3 = eng.strokes.size;
    ctl.hold.trash = true;
    const d3 = () => root.querySelectorAll(".itda-wb-item").find(x => x.dataset.id === "wb3").querySelectorAll("button").find(b => b.dataset.act === "trash");
    click(w, d3()); click(w, d3()); await settle();
    stroke(w, stage, line(100, 100, 200, 100, 4));
    ok("잠금: 지금 판 trash 를 기다리는 동안 잠그고 그리지 않는다", eng.locked() && eng.strokes.size === n3);
    ctl.hold.trash = false; ctl.release(); await settle();
    ok("잠금: 지운 뒤 남은 판이 뜨면 푼다", st.id !== "wb3" && !eng.locked());
    await w.W.close();
    // 처음 열기 — 첫 판이 뜰 때까지 잠근다
    ctl.hold.load = true;
    w.W.open({ id: "wb1" }); await settle();
    const D1 = w.W.debug();
    ok("잠금: 처음 열 때 첫 판의 load 를 기다리는 동안 잠그고 「여는 중」", D1.engine.locked() && !D1.root.querySelector(".itda-wb-busy").hidden);
    ctl.hold.load = false; ctl.release(); await settle();
    ok("잠금: 첫 판이 뜨면 푼다", !D1.engine.locked() && D1.state.id === "wb1");
    await w.W.close();
  }
  // [3] 저장하고 떠나는 모든 길에서 진행 중 그리기·지우기를 먼저 확정한다
  {
    const w = world(), mem = await two(w);
    let r = await openWb(w, mem, { id: "wb1" });
    penDown(w, r.stage, 300, 300);
    await w.W.close();
    ok("확정: 펜을 댄 채 닫으면 그 획까지 저장한다", await strokesIn(w, mem, "wb1") === 2);
    r = await openWb(w, mem, { id: "wb1" });
    penDown(w, r.stage, 300, 400);
    key(w, null, "]", "BracketRight"); await settle();
    ok("확정: 펜을 댄 채 전환하면 떠난 판에 그 획까지 저장한다", r.D.state.id === "wb2" && await strokesIn(w, mem, "wb1") === 3);
    penDown(w, r.stage, 300, 300);
    w.doc.visibilityState = "hidden"; dispatch(w, w.doc, "visibilitychange", {}); await settle();
    w.doc.visibilityState = "visible";
    ok("확정: 펜을 댄 채 숨기면 그 획까지 저장한다", await strokesIn(w, mem, "wb2") === 3);
    penDown(w, r.stage, 300, 500);
    click(w, btn(r.root, "list")); await settle();
    click(w, btn(r.root, "new")); await settle();
    ok("확정: 펜을 댄 채 새 판을 만들면 떠난 판에 그 획까지 저장한다", r.D.state.id === "wb3" && await strokesIn(w, mem, "wb2") === 4);
    penDown(w, r.stage, 300, 300);
    const dupOf3 = r.root.querySelectorAll(".itda-wb-item").find(x => x.dataset.id === "wb3").querySelectorAll("button").find(b => b.dataset.act === "dup");
    click(w, dupOf3); await settle();
    ok("확정: 펜을 댄 채 복제하면 사본에 그 획이 있다", await strokesIn(w, mem, "wb4") === 1);
    key(w, null, "Escape", "Escape");
    // 지우개 중 닫기
    key(w, null, "e", "KeyE");
    const s = lastStroke(r.D.engine), at = w.C.toScreen(r.D.engine.view, s.pts[0], s.pts[1]);
    ptr(w, r.stage, "pointerdown", { pointerId: 7, pointerType: "pen", clientX: at.x, clientY: at.y });
    ok("확정: (준비) 지우는 중", r.D.engine.mode && r.D.engine.mode.kind === "erase" && r.D.engine.strokes.size === 0);
    await w.W.close();
    ok("확정: 지우는 중 닫으면 지운 것까지 저장한다", await strokesIn(w, mem, "wb3") === 0);
  }
  // [2] 저장하지 못한 본문·마지막 판·미리보기는 store 마다 따로다
  {
    const w = world(), memA = w.W.memoryStore(), ctl = {}, A = logged(memA, ctl);
    const r = await openWb(w, A);
    stroke(w, r.stage, line(10, 10, 60, 60, 4));
    ctl.fail.save = true;
    await w.W.close(); await w.W.close();
    ok("저장소: (준비) A 의 wb1 본문을 메모리에 두고 닫았다", !w.W.isOpen());
    const memB = w.W.memoryStore();
    await memB.create("B1"); await memB.save("wb1", boardText(w.C, 2));
    await memB.create("B2"); await memB.save("wb2", boardText(w.C, 3));
    const ctlB = {}, B = logged(memB, ctlB);
    w.win.ItdaWhiteboardHost = { store: B };
    await w.W.open({ id: "wb1" }); await settle();
    advance(w, 3000); await settle();
    ok("저장소: 다른 store 의 같은 id 는 그 store 의 본문으로 연다(A 의 본문으로 덮지 않는다)", w.W.debug().engine.strokes.size === 2 && !ctlB.log.some(x => /^save:/.test(x)) && await strokesIn(w, memB, "wb1") === 2, ctlB.log);
    await w.W.close();
    ctl.fail.save = false; ctl.log = [];
    w.win.ItdaWhiteboardHost = { store: A };
    await w.W.open(); await settle();
    ok("저장소: A 로 다시 열면 A 의 메모리 본문으로 열고 다시 저장한다", w.W.debug().engine.strokes.size === 1 && ctl.log.includes("save:wb1") && await strokesIn(w, memA, "wb1") === 1, ctl.log);
    await w.W.close();
    w.win.ItdaWhiteboardHost = { store: B };
    await w.W.open(); await settle();
    ok("저장소: 마지막 판도 store 마다다(B 는 B 에서 마지막에 쓴 판)", w.W.debug().state.id === "wb1");
    await w.W.close();
    const memC = w.W.memoryStore();
    await memC.create("C1"); await memC.save("wb1", boardText(w.C, 1));
    await memC.create("C2"); await memC.save("wb2", boardText(w.C, 1));
    w.win.ItdaWhiteboardHost = { store: memC };
    await w.W.open(); await settle();
    ok("저장소: 처음 여는 store 는 다른 store 의 마지막 판(wb1)을 따르지 않는다", w.W.debug().state.id === "wb2");
    await w.W.close();
  }
  // [4] 복제 — 가장 새 본문을 저장하지 못하면 복제하지 않는다
  {
    const w = world(), mem = await two(w), ctl = {}, store = logged(mem, ctl);
    const { stage, root, D } = await openWb(w, store, { id: "wb1" });
    stroke(w, stage, line(10, 10, 60, 60, 4));
    ctl.fail.save = true; ctl.log = [];
    click(w, btn(root, "list")); await settle();
    const dupBtn = id => root.querySelectorAll(".itda-wb-item").find(x => x.dataset.id === id).querySelectorAll("button").find(b => b.dataset.act === "dup");
    click(w, dupBtn("wb1")); await settle();
    ok("복제: 지금 판 저장이 실패하면 duplicate 를 부르지 않고 안내한다", !ctl.log.some(x => /^duplicate/.test(x)) && (await mem.list()).length === 2
      && /복제하지 않았습니다/.test(root.querySelector(".itda-wb-note").textContent), ctl.log);
    ctl.fail.save = false; ctl.log = [];
    click(w, dupBtn("wb1")); await settle();
    ok("복제: 저장되면 저장 → duplicate, 사본은 화면의 최신 판", ops(ctl) === "save:wb1,duplicate:wb1,list" && await strokesIn(w, mem, "wb3") === 2, ctl.log);
    // 다른 판에 저장하지 못한 본문이 있으면 먼저 그것을 저장해 본다
    ctl.fail.save = true;
    D.put("wb2", boardText(w.C, 5)); await settle(); // 다른 판의 save 가 거부돼 기록기에 남았다
    ctl.log = [];
    click(w, dupBtn("wb2")); await settle();
    ok("복제: 다른 판의 저장하지 못한 본문을 저장하지 못하면 복제하지 않는다", ops(ctl) === "save:wb2" && D.dirty("wb2") && /복제하지 않았습니다/.test(root.querySelector(".itda-wb-note").textContent), ctl.log);
    ctl.fail.save = false; ctl.log = [];
    click(w, dupBtn("wb2")); await settle();
    const copies = (await mem.list()).map(x => x.id);
    ok("복제: 다른 판의 저장하지 못한 본문을 저장한 뒤 복제한다(사본도 그 본문)", ops(ctl) === "save:wb2,duplicate:wb2,list" && !D.dirty("wb2") && await strokesIn(w, mem, "wb2") === 5 && await strokesIn(w, mem, copies[copies.length - 1]) === 5, ctl.log);
    ctl.log = [];
    click(w, dupBtn("wb2")); await settle();
    ok("복제: 저장하지 못한 본문이 없는 다른 판은 곧바로 duplicate", ops(ctl) === "duplicate:wb2,list", ctl.log);
    await w.W.close();
  }
  // [5] 끝나지 않는 store — 닫기는 8초 뒤 끝나고, 늦은 완료는 DOM·상태를 건드리지 않는다, 한 판의 save 는 한 번에 하나
  {
    const w = world(), mem = await two(w), ctl = {}, store = logged(mem, ctl);
    let closedArg = null;
    w.win.ItdaWhiteboardHost = { store, onClose(a) { closedArg = a; } };
    await w.W.open({ id: "wb1" }); await settle();
    const D = w.W.debug();
    ctl.hold.load = true;
    key(w, null, "]", "BracketRight"); await settle();
    let closed = null;
    w.W.close().then(v => { closed = v; });
    await settle();
    ok("기다림: 끝나지 않는 load 뒤 닫기는 그 load 를 기다리지 않고 곧바로 닫는다(떠난 판은 저장할 것이 없다 — onClose 에 unsaved 없음)",
      closed === true && !w.W.isOpen() && !surfaceOf(w) && closedArg && closedArg.unsaved === false, closedArg);
    ctl.hold.load = false; ctl.release(); await settle(); frames(w, 2);
    ok("기다림: 닫힌 세션의 늦은 load 는 표면을 다시 붙이거나 판을 바꾸지 않는다", !surfaceOf(w) && !w.W.isOpen() && D.state.id === "wb1");
    // 끝나지 않는 save 뒤 첫 닫기 — 8초 뒤 메모리에 남기고 닫는다
    closedArg = null;
    await w.W.open({ id: "wb1" }); await settle();
    const r = { stage: surfaceOf(w).querySelector(".itda-wb-stage") };
    stroke(w, r.stage, line(10, 10, 60, 60, 4));
    ctl.hold.save = true; ctl.log = [];
    closed = null;
    w.W.close().then(v => { closed = v; });
    await settle(); advance(w, 8100); await settle();
    ok("기다림: 끝나지 않는 save 뒤 첫 닫기도 8초 뒤 닫고 onClose({ unsaved: true })", closed === true && !w.W.isOpen() && closedArg && closedArg.unsaved === true, closedArg);
    const recAt = D.rec("wb1");
    ok("기다림: 본문은 그 store 의 기록기에 남는다", recAt && w.C.parse(D.latest("wb1")).board.strokes.length === 2 && D.dirty("wb1") && recAt.running);
    ctl.log = [];
    await w.W.open({ id: "wb1" }); await settle();
    ok("기다림: 같은 store 로 다시 열면 같은 기록기를 쓴다(그 본문으로 연다)", w.W.debug().recs === D.recs && w.W.debug().rec("wb1") === recAt && w.W.debug().engine.strokes.size === 2);
    stroke(w, surfaceOf(w).querySelector(".itda-wb-stage"), line(10, 100, 60, 160, 4));
    advance(w, 2100); await settle();
    ok("기다림: 버린 save 가 아직 돌면 그 판의 새 save 를 내지 않는다", !ctl.log.some(x => /^save:/.test(x)), ctl.log);
    advance(w, 8000); await settle();
    ok("기다림: 앞 save 를 기다리다 8초를 넘기면 내지 않고 실패로 본다", !ctl.log.some(x => /^save:/.test(x)) && /저장하지 못했습니다/.test(surfaceOf(w).querySelector(".itda-wb-note").textContent), ctl.log);
    stroke(w, surfaceOf(w).querySelector(".itda-wb-stage"), line(10, 200, 60, 260, 4));
    advance(w, 2100); await settle();
    ctl.hold.save = false; ctl.release(); await settle();
    ok("기다림: 앞 save 가 끝나면 그 뒤에 가장 새 본문을 한 번 낸다", ctl.log.filter(x => x === "save:wb1").length === 1 && await strokesIn(w, mem, "wb1") === 4, ctl.log);
    await w.W.close();
    // 닫힌 세션에 늦게 온 load — DOM·상태를 건드리지 않는다
    let late = null;
    const m2 = await two(w), lazy = Object.assign({}, m2, { load: id => new Promise(res => { late = () => res(m2.load(id)); }) });
    w.win.ItdaWhiteboardHost = { store: lazy };
    w.W.open({ id: "wb2" }); await settle();
    const D2 = w.W.debug();
    w.W.close(); await settle(); advance(w, 8100); await settle();
    ok("기다림: 처음 load 가 끝나지 않아도 닫힌다", !w.W.isOpen() && !surfaceOf(w));
    late(); await settle(); frames(w, 2);
    ok("기다림: 늦게 온 첫 load 는 판을 열지 않는다", !surfaceOf(w) && D2.state.id !== "wb2" && D2.engine.strokes.size === 0, D2.state.id);
    // 처음 열기 중의 닫기 — list 를 기다리는 동안 닫으면 곧바로 닫고, 늦게 온 list 뒤에 load 를 부르지 않는다(판을 열지 않는다)
    {
      const m3 = await two(w), c3 = {}, s3 = logged(m3, c3);
      c3.hold.list = true;
      w.win.ItdaWhiteboardHost = { store: s3 };
      w.W.open({ id: "wb2" }); await settle();
      const D3 = w.W.debug();
      let shut = null;
      w.W.close().then(v => { shut = v; }); await settle();
      ok("기다림: (준비) 처음 열기 중의 닫기는 앞 일을 기다리지 않고 곧바로 닫는다", shut === true && !w.W.isOpen(), c3.log);
      c3.hold.list = false; c3.release(); await settle(); frames(w, 2);
      ok("기다림: 닫힌 세션의 늦은 list 는 load 를 부르거나 판을 열지 않는다", !surfaceOf(w) && !c3.log.includes("load:wb2") && D3.state.id == null && D3.engine.strokes.size === 0, c3.log);
    }
    // 닫힌 뒤 늦게 성공한 save — 표면의 상태를 건드리지 않는다(기록기만 깨끗해진다)
    {
      const m4 = await two(w), c4 = {}, s4 = logged(m4, c4);
      let arg = null;
      w.win.ItdaWhiteboardHost = { store: s4, onClose(a) { arg = a; } };
      await w.W.open({ id: "wb1" }); await settle();
      const D4 = w.W.debug();
      c4.hold.save = true;
      stroke(w, D4.root.querySelector(".itda-wb-stage"), line(10, 10, 60, 60, 4));
      let shut = null;
      w.W.close().then(v => { shut = v; }); await settle();
      advance(w, 8100); await settle(); // 닫기 8초
      const before = D4.state.list.find(x => x.id === "wb1").modified, r4 = D4.rec("wb1");
      ok("기다림: (준비) 닫기가 8초에 닫았고 save 는 아직 돈다", shut === true && c4.log.includes("save:wb1") && arg && arg.unsaved === true && r4 && r4.running, [shut, c4.log, arg]);
      c4.hold.save = false; c4.release(); await settle();
      ok("기다림: 닫힌 세션에 늦게 성공한 save 는 표면의 상태를 건드리지 않고 기록기만 깨끗해진다",
        D4.state.list.find(x => x.id === "wb1").modified === before && !surfaceOf(w) && !r4.running && r4.done >= 1 && !D4.recs.has("wb1") && await strokesIn(w, m4, "wb1") === 2);
    }
  }
  // closeFailed — 저장이 한 번 되거나 판을 바꾸면 첫 닫기 안내를 다시 센다
  {
    const w = world(), mem = await two(w), ctl = {}, store = logged(mem, ctl);
    const { stage, D } = await openWb(w, store, { id: "wb1" });
    stroke(w, stage, line(10, 10, 60, 60, 4));
    ctl.fail.save = true;
    ok("닫기 안내: (준비) 첫 닫기는 닫지 않는다", await w.W.close() === false);
    ctl.fail.save = false;
    stroke(w, stage, line(10, 100, 60, 160, 4));
    advance(w, 2100); await settle();
    ctl.fail.save = true;
    stroke(w, stage, line(10, 200, 60, 260, 4));
    ok("닫기 안내: 저장이 한 번 된 뒤 다시 실패하면 첫 닫기는 또 닫지 않는다", await w.W.close() === false && w.W.isOpen());
    key(w, null, "]", "BracketRight"); await settle();
    ok("전환: 떠나는 판의 저장이 거부되면 판을 바꾸지 않는다(원래 판에 남아 잠금을 풀고 안내, 대상을 읽지 않는다)", D.state.id === "wb1" && D.dirty("wb1") && !D.engine.locked()
      && /판을 바꾸지 않았습니다/.test(D.root.querySelector(".itda-wb-note").textContent) && !ctl.log.includes("load:wb2"), ctl.log);
    ctl.fail.save = false;
    key(w, null, "]", "BracketRight"); await settle();
    ok("닫기 안내: (준비) 저장한 뒤 판을 바꿨다", D.state.id === "wb2" && !D.dirty("wb1"));
    ctl.fail.save = true;
    stroke(w, stage, line(10, 300, 60, 360, 4));
    ok("닫기 안내: 판을 바꾼 뒤의 첫 닫기도 닫지 않는다", await w.W.close() === false && w.W.isOpen());
    let arg = null;
    w.win.ItdaWhiteboardHost.onClose = a => { arg = a; };
    ok("닫기 안내: 두 번째 닫기는 기록기에 남기고 닫는다(onClose({ unsaved: true }))", await w.W.close() === true && !w.W.isOpen() && D.dirty("wb2") && arg && arg.unsaved === true, arg);
    // 메모리 본문으로 다시 열고 그 저장이 실패한 뒤, 고쳐서 저장이 되면 낡은 메모리 본문은 거둔다(다음에 열 때 낡은 본문으로 열지 않는다)
    await w.W.open({ id: "wb2" }); await settle();
    const s2 = surfaceOf(w).querySelector(".itda-wb-stage"), n2 = w.W.debug().engine.strokes.size;
    ctl.fail.save = false;
    stroke(w, s2, line(10, 400, 60, 460, 4));
    advance(w, 2100); await settle();
    await w.W.close();
    await w.W.open({ id: "wb2" }); await settle();
    ok("닫기 안내: 저장이 되면 기록기가 깨끗해진다(다음에 열 때 낡은 본문으로 열지 않는다)", !w.W.debug().dirty("wb2") && w.W.debug().engine.strokes.size === n2 + 1 && await strokesIn(w, mem, "wb2") === n2 + 1, [n2, w.W.debug().engine.strokes.size]);
    await w.W.close();
  }
  // [6] 수치 범위 — parse 는 범위 밖 좌표·굵기를 거부하고, 색인·타일은 범위 끝에서도 반복이 끝난다
  {
    const { C } = world();
    const one = (o) => { const j = JSON.parse(boardText(C, 1)); Object.assign(j.strokes[0], o.s || {}); if (o.view) j.view = o.view; return C.parse(JSON.stringify(j)); };
    ok("범위: 상수 — 좌표 1e9, 굵기 0.01~10000, 배율 0.1~8", C.COORD_MAX === 1e9 && C.W_MIN === 0.01 && C.W_MAX === 10000 && C.ZMIN === 0.1 && C.ZMAX === 8);
    ok("범위: 좌표 끝(±1e9)은 읽는다", one({ s: { p: [1e9, -1e9, 0.5, -1e9, 1e9, 0.5] } }).ok);
    ok("범위: 좌표가 범위 밖이면 거부(x·y)", !one({ s: { p: [1e9 + 1, 0, 0.5] } }).ok && !one({ s: { p: [0, -1e9 - 1, 0.5] } }).ok && /좌표 범위/.test(one({ s: { p: [1e20, 0, 0.5] } }).error));
    ok("범위: 굵기 끝(0.01·10000)은 읽고 밖은 거부", one({ s: { w: 0.01 } }).ok && one({ s: { w: 10000 } }).ok && !one({ s: { w: 0.0099 } }).ok && !one({ s: { w: 10000.5 } }).ok && !one({ s: { w: 1e-6 } }).ok);
    const vb = one({ view: { x: 1e12, y: -1e12, z: 0.01 } });
    ok("범위: 보기는 끌어 맞춘다(x·y 좌표 범위, z 0.1~8)", vb.ok && vb.board.view.x === 1e9 && vb.board.view.y === -1e9 && vb.board.view.z === 0.1, vb.board && vb.board.view);
    const run = src => { try { vm.runInNewContext(src, { C }, { timeout: 2000 }); return true; } catch (e) { return e.code || String(e); } };
    ok("범위: Grid — 범위 끝 상자의 넣기·찾기·빼기가 끝난다", run(`const g = new C.Grid(512), s = { id: 1, box: [1e9 - 100, -1e9 - 15000, 1e9 + 15000, -1e9 + 50] }; g.insert(s);
      if (g.query([1e9 - 10, -1e9, 1e9, -1e9 + 10]).length !== 1) throw new Error("못 찾음"); g.remove(s); if (g.query([1e9 - 10, -1e9, 1e9, -1e9 + 10]).length) throw new Error("남음");`) === true);
    ok("범위: Grid — 범위 밖(1e20) 상자도 끝난다(끝 칸에 모인다)", run(`const g = new C.Grid(512), s = { id: 1, box: [1e20, 0, 1e20, 0] }; g.insert(s); g.query([0, 0, 10, 10]); g.remove(s);`) === true);
    ok("범위: tilesFor — 범위 끝 보기(배율 0.1·8)에서 화면만큼 타일을 낸다", run(`for (const z of [0.1, 8]) for (const x of [1e9, -1e9]) { const v = C.boundView({ x, y: -x, z }, 1280, 720), T = C.tileWorld(z, 3, 512), t = C.tilesFor(v, 1280, 720, T);
      if (!t.length || t.length > 200 || !t.every(([i, j]) => Number.isSafeInteger(i) && Number.isSafeInteger(j))) throw new Error("타일 " + t.length); }`) === true);
    ok("범위: tilesFor — 범위 밖(1e20) 보기도 끝난다", run(`C.tilesFor({ x: 1e20, y: -1e20, z: 1 }, 1280, 720, 256);`) === true);
    const bv = C.boundView({ x: 5e9, y: -5e9, z: 2 }, 1000, 800);
    ok("범위: boundView 는 화면 전체를 좌표 범위 안에 묶는다(오른쪽 끝 = +1e9, 위 끝 = -1e9)", near(bv.x + 1000 / 2, 1e9, 1e-3) && near(bv.y, -1e9, 1e-3) && bv.z === 2, bv);
    const bw = C.boundView({ x: -5e9, y: 5e9, z: 0.1 }, 1280, 720);
    ok("범위: boundView — 최소 배율에서도 왼쪽 끝 = -1e9, 아래 끝 = +1e9", near(bw.x, -1e9, 1e-3) && near(bw.y + 720 / 0.1, 1e9, 1e-3), bw);
    const inside = C.boundView({ x: 12.5, y: -7, z: 3 }, 1280, 720);
    ok("범위: boundView — 범위 안 보기는 그대로", inside.x === 12.5 && inside.y === -7 && inside.z === 3, inside);
    ok("범위: inRange — 끝은 안, 넘거나 유한하지 않으면 밖", C.inRange(1e9, -1e9) && !C.inRange(1e9 + 1, 0) && !C.inRange(0, -1e9 - 0.01) && !C.inRange(NaN, 0) && !C.inRange(0, Infinity));
    // 엔진의 이동도 묶인다 — 네 모서리·두 배율 끝에서 그은 획은 저장 전후 좌표가 같다(r2 [2])
    const w = world(), mem = w.W.memoryStore();
    const r = await openWb(w, mem);
    const eng = r.D.engine, size = eng.size;
    let corners = true;
    const why = [];
    for (const z of [0.1, 8]) for (const [vx, vy] of [[1e12, 1e12], [-1e12, -1e12], [1e12, -1e12], [-1e12, 1e12]]) {
      eng.view = { x: vx, y: vy, z };
      const v = eng.view;
      const okView = v.x >= -1e9 && v.y >= -1e9 && v.x + size.W / v.z <= 1e9 + 1e-3 && v.y + size.H / v.z <= 1e9 + 1e-3;
      stroke(w, r.stage, [[1, 1], [640, 360], [1279, 719]]);
      const s = lastStroke(eng);
      const back = w.C.parse(w.C.serialize({ strokes: [{ tool: s.tool, color: s.color, w: s.w, pts: s.pts }] }));
      const same = back.ok && back.board.strokes[0].pts.length === s.pts.length && back.board.strokes[0].pts.every((x, i) => near(x, s.pts[i], 0.006));
      if (!okView || !same) { corners = false; why.push([z, vx, vy, v, back.error]); }
    }
    ok("범위: 네 모서리·배율 끝(0.1·8)에서 화면 전체가 범위 안이고, 끝에서 그은 획은 저장 전후 좌표가 같다(반올림 허용)", corners, why);
    eng.view = { x: 1e12, y: 0, z: 1 };
    stroke(w, r.stage, line(1200, 10, 1270, 700, 6));
    await w.W.close();
    const back = w.C.parse(await mem.load("wb1"));
    ok("범위: 범위 끝에서 그은 획도 저장한 파일을 다시 읽는다(점은 범위 안)", back.ok && back.board.strokes.length === 9 && back.board.strokes.every(s => s.pts.every((x, i) => i % 3 === 2 || Math.abs(x) <= 1e9)), back.error);
  }
  // r2 [2] — 범위 밖 점: 획을 만들지 않거나 거기서 끝낸다(나갔다 들어와도 잇지 않는다), 예측 꼬리도 범위 밖은 그리지 않는다, serialize 는 던진다
  {
    const w = world(), mem = w.W.memoryStore();
    await mem.create("경계"); await mem.save("wb1", boardText(w.C, 0, { view: { x: 1e9, y: 0, z: 1 } }));
    const r = await openWb(w, mem, { id: "wb1" });
    const eng = r.D.engine;
    ok("경계: 판을 읽을 때 보기를 화면 전체가 범위 안이도록 묶는다", near(eng.view.x + eng.size.W / eng.view.z, 1e9, 1e-3), eng.view);
    stroke(w, r.stage, line(800, 100, 1000, 100, 4));
    const s1 = lastStroke(eng), xs = s1.pts.filter((_, i) => i % 3 === 0);
    await r.D.save();
    const saved = w.C.parse(await mem.load("wb1")).board.strokes[0].pts.filter((_, i) => i % 3 === 0);
    ok("경계: 허용 영역에서 그은 200 단위 가로획은 저장 전후 x 가 같다(r2 재현)", near(xs[xs.length - 1] - xs[0], 200, 0.02) && saved.length === xs.length && saved.every((x, i) => near(x, xs[i], 0.006)), [xs[0], xs[xs.length - 1], saved[0], saved[saved.length - 1]]);
    // 화면 밖(오른쪽 — 범위 밖)에서 시작한 획은 만들지 않는다
    const n0 = eng.strokes.size;
    stroke(w, r.stage, line(1400, 100, 1500, 100, 4));
    ok("경계: 첫 점부터 범위 밖이면 획을 만들지 않는다(빈 획·끝 점 참조 없음)", eng.strokes.size === n0 && !eng.mode && /판의 끝/.test(r.root.querySelector(".itda-wb-note").textContent));
    // 안에서 시작해 밖으로 나갔다가 다시 들어오는 획은 나간 자리에서 끝나고 이어지지 않는다
    stroke(w, r.stage, [[1000, 300], [1100, 300], [1200, 300], [1500, 300], [1200, 400], [1000, 400]]);
    const s2 = lastStroke(eng), xs2 = s2.pts.filter((_, i) => i % 3 === 0), ys2 = s2.pts.filter((_, i) => i % 3 === 1);
    ok("경계: 나갔다 들어오는 입력은 나간 자리에서 획을 끝낸다(다시 들어온 점을 긴 직선으로 잇지 않는다)", eng.strokes.size === n0 + 1 && xs2.every(x => x <= 1e9) && ys2.every(y => near(y, ys2[0], 1e-6)) && !eng.mode, [xs2, ys2]);
    // coalesced 의 범위 밖 점도 같은 길이다
    ptr(w, r.stage, "pointerdown", { pointerId: 7, pointerType: "pen", clientX: 900, clientY: 500 });
    w.clock += 4;
    ptr(w, r.stage, "pointermove", { pointerId: 7, pointerType: "pen", clientX: 1270, clientY: 500, getCoalescedEvents: () => [{ clientX: 1000, clientY: 500, pressure: 0.5, timeStamp: w.clock }, { clientX: 1290, clientY: 500, pressure: 0.5, timeStamp: w.clock }, { clientX: 1270, clientY: 500, pressure: 0.5, timeStamp: w.clock }] });
    ptr(w, r.stage, "pointerup", { pointerId: 7, pointerType: "pen", clientX: 1270, clientY: 500 });
    const s3 = lastStroke(eng);
    ok("경계: coalesced 의 범위 밖 점에서도 획을 끝낸다", eng.strokes.size === n0 + 2 && s3.pts.every((x, i) => i % 3 !== 0 || x <= 1e9) && s3.pts.length === 6, s3.pts);
    // 떼기 때 거른(skipped) 끝점이 범위 밖이면 넣지 않는다
    ptr(w, r.stage, "pointerdown", { pointerId: 7, pointerType: "pen", clientX: 1279.5, clientY: 600 });
    w.clock += 4;
    ptr(w, r.stage, "pointermove", { pointerId: 7, pointerType: "pen", clientX: 1279.9, clientY: 600 }); // 1px 안 — 거른다(skipped)
    r.stage.clientWidth = 640; dispatch(w, w.win, "resize", {}); // 그리는 중 화면이 좁아졌다 — 거른 자리는 이제 화면(범위) 밖이다
    const before = eng.strokes.size;
    ptr(w, r.stage, "pointerup", { pointerId: 7, pointerType: "pen", clientX: 1279.9, clientY: 600 });
    const s4 = lastStroke(eng);
    ok("경계: 뗀 자리(skipped) 점도 범위 검사를 거친다", eng.strokes.size === before + 1 && s4.pts.length === 3 && s4.pts.every((x, i) => i % 3 !== 0 || x <= 1e9), s4.pts);
    r.stage.clientWidth = 1280; dispatch(w, w.win, "resize", {});
    eng.view = { x: 1e12, y: 0, z: 1 };
    // 예측 꼬리 — 범위 밖 점은 그리지 않는다
    ptr(w, r.stage, "pointerdown", { pointerId: 7, pointerType: "pen", clientX: 1200, clientY: 650 });
    w.clock += 4;
    ptr(w, r.stage, "pointermove", { pointerId: 7, pointerType: "pen", clientX: 1270, clientY: 650, getPredictedEvents: () => [{ clientX: 1278, clientY: 650 }, { clientX: 1300, clientY: 650 }] });
    ok("경계: 범위 밖 예측 꼬리는 그리지 않는다(범위 안 예측 점까지만)", eng.mode && eng.mode.pred && eng.mode.pred.length === 3 && eng.mode.pred[0] <= 1e9, eng.mode && eng.mode.pred);
    ptr(w, r.stage, "pointerup", { pointerId: 7, pointerType: "pen", clientX: 1270, clientY: 650 });
    // serialize 는 범위 밖에서 던지고, 표면은 저장을 멈추고 본문을 잃지 않는다(닫기도 하지 않는다)
    ok("경계: serialize 는 범위 밖 점 좌표를 끌어 맞추지 않고 던진다", (() => { try { w.C.serialize({ strokes: [{ tool: "pen", color: 0, w: 1, pts: [1e9 + 5, 0, 0.5] }] }); return false; } catch (e) { return e.name === "RangeError" && /좌표 범위/.test(e.message); } })());
    const bad = lastStroke(eng);
    bad.pts[0] = 2e9; // 내부 불변식이 깨진 판(시험으로만 만든다)
    const nBefore = eng.strokes.size, savedBefore = await mem.load("wb1");
    const shut = await w.W.close();
    ok("경계: 본문을 만들지 못하면 닫지 않고(false) 판을 엔진에 둔 채 안내한다(저장소 본문도 그대로)", shut === false && w.W.isOpen() && eng.strokes.size === nBefore && /좌표 범위 밖/.test(r.root.querySelector(".itda-wb-note").textContent) && await mem.load("wb1") === savedBefore && !eng.locked());
    bad.pts[0] = 1e9 - 10;
    ok("경계: 고치면 닫는다", await w.W.close() === true && w.C.parse(await mem.load("wb1")).board.strokes.length === nBefore);
  }
  // [7] 굵기 왕복 — serialize 는 셋째 자리로 반올림하되 0.01 아래로 내려가지 않는다
  {
    const { C } = world();
    const trip = w0 => { const t1 = C.serialize({ strokes: [{ tool: "pen", color: 0, w: w0, pts: [0, 0, 0.5] }] }), p1 = C.parse(t1), t2 = p1.ok && C.serialize(p1.board), p2 = t2 && C.parse(t2);
      return { w1: JSON.parse(t1).strokes[0].w, ok: p1.ok && p2 && p2.ok && t1 === t2 }; };
    const a = trip(0.0104), b = trip(0.0149), c = trip(10000), d = trip(0.01), e = trip(2.34567);
    ok("굵기 왕복: 셋째 자리 반올림(0.0104→0.01·0.0149→0.015·2.34567→2.346), 끝 값 그대로, parse→serialize→parse 가 같다",
      a.w1 === 0.01 && b.w1 === 0.015 && e.w1 === 2.346 && c.w1 === 10000 && d.w1 === 0.01 && a.ok && b.ok && c.ok && d.ok && e.ok, [a, b, c, d, e]);
    const f = trip(0.001), g = trip(1e7);
    ok("굵기 왕복: 범위 밖 굵기를 쓰면 끝 값(0.01·10000)으로 끌어 맞춰 다시 읽힌다", f.w1 === 0.01 && g.w1 === 10000 && f.ok && g.ok, [f, g]);
    const far = C.parse(C.serialize({ view: { x: 5e9, y: 0, z: 99 }, strokes: [{ tool: "hl", color: 1, w: 3, pts: [1e9, -1e9, 0.5] }] }));
    ok("굵기 왕복: 범위 밖 보기는 끌어 맞춰 다시 읽힌다(끝 좌표의 점은 그대로)", far.ok && far.board.strokes[0].pts[0] === 1e9 && far.board.strokes[0].pts[1] === -1e9 && far.board.view.x === 1e9 && far.board.view.z === 8, far.error);
    let threw = 0;
    for (const p of [[2e9, 0, 0.5], [0, -1e9 - 1, 0.5], [NaN, 0, 0.5], [0, Infinity, 0.5]]) { try { C.serialize({ strokes: [{ tool: "pen", color: 0, w: 1, pts: p }] }); } catch (e) { threw++; } }
    ok("굵기 왕복: 범위 밖·유한하지 않은 점 좌표는 끌어 맞추지 않고 던진다", threw === 4, threw);
  }
  // 의견 — 미리보기 캐시(그린 결과만·같은 판 요청 하나로·최대 40개), 진행 중 획 점 상한
  {
    const w = world(), mem = await two(w), ctl = {}, store = logged(mem, ctl);
    const { root, D } = await openWb(w, store, { id: "wb2" });
    ctl.log = [];
    click(w, btn(root, "list")); await settle(8);
    ok("미리보기: 목록을 열 때 두 번 칠해도 같은 판 본문은 한 번만 읽는다", ctl.log.filter(x => x === "load:wb1").length === 1, ctl.log);
    const c = D.thumbs.get("wb1");
    ok("미리보기: 캐시는 그린 canvas 만 든다(본문 없음)", c && c.canvas && c.canvas.tag === "canvas" && !("board" in c) && Object.keys(c).sort().join() === "canvas,modified", c && Object.keys(c));
    key(w, null, "Escape", "Escape");
    ctl.log = [];
    click(w, btn(root, "list")); await settle(8);
    ok("미리보기: 다시 열면 캐시로 그린다(읽지 않는다)", !ctl.log.some(x => x === "load:wb1"), ctl.log);
    key(w, null, "Escape", "Escape");
    for (let i = 0; i < 44; i++) { const n = await mem.create("판 " + i); await mem.save(n.id, boardText(w.C, 1)); }
    click(w, btn(root, "list")); await settle(60);
    ok("미리보기: 캐시는 store 마다 최대 40개", D.thumbs.size === 40, D.thumbs.size);
    key(w, null, "Escape", "Escape");
    await w.W.close();
  }
  {
    const w = world(), mem = w.W.memoryStore();
    const { stage, root, D } = await openWb(w, mem);
    const eng = D.engine;
    ptr(w, stage, "pointerdown", { pointerId: 7, pointerType: "pen", clientX: 100, clientY: 100 });
    const at = i => ({ clientX: 100 + (i % 500) * 2, clientY: 100 + Math.floor(i / 500) * 2, pressure: 0.5, timeStamp: i });
    for (let i = 1; i < 19999; i++) ptr(w, stage, "pointermove", { pointerId: 7, pointerType: "pen", ...at(i) });
    // 마지막 사건은 coalesced 점 넷 — 상한(2만)은 묶음 가운데서 닿는다(나머지 점은 버린다)
    const co = [at(19999), at(20000), at(20001), at(20002)];
    ptr(w, stage, "pointermove", { pointerId: 7, pointerType: "pen", ...at(20002), getCoalescedEvents: () => co });
    ok("획 점 상한: 2만 점에 닿으면 그 획을 끝내고 안내한다", !eng.mode && eng.strokes.size === 1 && lastStroke(eng).pts.length === 20000 * 3 && /2.?0000개|20000개/.test(root.querySelector(".itda-wb-note").textContent), [eng.strokes.size, lastStroke(eng) && lastStroke(eng).pts.length / 3]);
    ptr(w, stage, "pointermove", { pointerId: 7, pointerType: "pen", clientX: 500, clientY: 500 });
    ptr(w, stage, "pointerup", { pointerId: 7, pointerType: "pen", clientX: 500, clientY: 500 });
    ok("획 점 상한: 떼기 전 움직임은 새 획으로 잇지 않는다", eng.strokes.size === 1 && !eng.mode);
    await w.W.close();
  }
  // ---- 10. 접근 방식 리뷰 「필수 순서 시험 8개」 — 가짜 시계와 손으로 푸는 store 로 실제 호출 차례·같은 판 save 최대 동시 수·최종 저장 본문·기록기 상태·표면을 본다
  // manual 은 store 를 감싸 부를 때마다 calls 에 { k, a, res(), rej() } 를 쌓는다(auto 에 든 함수는 곧바로 부른다). 실제 쓰기는 res() 때 한다.
  function manual(mem) {
    const m = { calls: [], live: new Map(), max: 0, auto: new Set(), texts: [], counts: {} };
    m.store = {};
    for (const k of Object.keys(mem)) m.store[k] = (...a) => {
      m.counts[k] = (m.counts[k] || 0) + 1;
      if (k === "save") { m.texts.push(a[1]); const n = (m.live.get(a[0]) || 0) + 1; m.live.set(a[0], n); m.max = Math.max(m.max, n); }
      const fin = () => { if (k === "save") m.live.set(a[0], m.live.get(a[0]) - 1); };
      if (m.auto.has(k)) { const p = mem[k](...a); p.then(fin, fin); return p; }
      return new Promise((resolve, reject) => {
        const c = { k, a, done: false };
        c.res = () => { c.done = true; fin(); resolve(mem[k](...a)); };
        c.rej = () => { c.done = true; fin(); reject(new Error("거부 " + k)); };
        m.calls.push(c);
      });
    };
    m.pending = k => m.calls.filter(c => c.k === k && !c.done);
    m.count = k => m.counts[k] || 0;
    m.saves = () => m.texts.length;
    return m;
  }
  const dirty0 = r => !r.gone && (r.want ? r.want.n : r.running ? r.running.n : r.done) > r.done;
  async function openHost(w, host, o) {
    w.W.open(Object.assign({ host }, o)); await settle(); frames(w, 2);
    const root = surfaceOf(w);
    return { root, stage: root && root.querySelector(".itda-wb-stage"), D: w.W.debug() };
  }
  const nStrokes = text => w0C.parse(text).board.strokes.length;
  let w0C = null;
  const renameTo = async (w, root, id, name) => {
    if (root.querySelector(".itda-wb-list").hidden) { click(w, btn(root, "list")); await settle(); }
    const row = root.querySelectorAll(".itda-wb-item").find(x => x.dataset.id === id);
    click(w, row.querySelectorAll("button").find(b => b.dataset.act === "rename"));
    const inp = root.querySelector(".itda-wb-rename");
    inp.value = name; key(w, inp, "Enter", "Enter"); await settle();
  };
  const rowBtn = (root, id, act) => root.querySelectorAll(".itda-wb-item").find(x => x.dataset.id === id).querySelectorAll("button").find(b => b.dataset.act === act);
  // 1. r2 [1] 그대로 — 끝나지 않는 첫 save → rename 대기 중 close → 1초 뒤 rename 완료 → close 8초 → 같은 store 로 다시 열기 → 새 획 → 첫 save 풀기
  {
    const w = world(), mem = w.W.memoryStore(); w0C = w.C;
    await mem.create("A"); await mem.save("wb1", boardText(w.C, 0, { name: "A" }));
    const m = manual(mem); m.auto.add("list"); m.auto.add("load");
    let arg = null;
    const host = { store: m.store, onClose(a) { arg = a; } };
    let a = await openHost(w, host, { id: "wb1" });
    stroke(w, a.stage, line(10, 10, 50, 50, 4)); a.D.save(); await settle();
    advance(w, 8100); await settle(); // 첫 save 는 지켜보기만 시간을 넘겼다 — 요청은 살아 있다
    await renameTo(w, a.root, "wb1", "새 이름");
    let shut = null;
    w.W.close().then(v => { shut = v; }); await settle();
    advance(w, 1000); m.pending("rename")[0].res(); await settle();
    advance(w, 7000); await settle();
    ok("순서1: 닫기는 요청부터 8초에 닫고 본문은 기록기에 남는다(onClose unsaved)", shut === true && !w.W.isOpen() && arg && arg.unsaved === true && m.saves() === 1, [shut, m.saves(), arg]);
    const r = a.D.rec("wb1");
    ok("순서1: rename 완료가 닫는 세션의 이름을 본문에 넣었다(want)", r && r.want && w.C.parse(r.want.text).board.name === "새 이름" && r.running, r && r.want);
    const b = await openHost(w, host, { id: "wb1" });
    ok("순서1: 다시 열면 기록기의 본문으로 열고(새 save 를 내지 않는다 — 첫 save 가 돈다)", b.D.engine.strokes.size === 1 && m.saves() === 1 && b.D.state.name === "새 이름");
    stroke(w, b.stage, line(20, 80, 100, 80, 4));
    advance(w, 2100); await settle();
    ok("순서1: 새 획도 첫 save 가 끝날 때까지 내지 않는다", m.saves() === 1 && m.live.get("wb1") === 1);
    m.pending("save")[0].res(); await settle();
    ok("순서1: 첫 save 가 끝나면 가장 새 본문 하나를 낸다(같은 판 save 최대 동시 수 1)", m.saves() === 2 && m.max === 1 && nStrokes(m.texts[1]) === 2 && w.C.parse(m.texts[1]).board.name === "새 이름", [m.saves(), m.max]);
    m.pending("save")[0].res(); await settle();
    advance(w, 20000); await settle();
    const fin = w.C.parse(await mem.load("wb1")).board;
    ok("순서1: 최종 저장 본문은 새 획·새 이름을 담고, 닫힌 세션의 늦은 put 이 없다(save 2번뿐)", fin.strokes.length === 2 && fin.name === "새 이름" && m.saves() === 2 && !b.D.dirty("wb1"), [fin.strokes.length, fin.name, m.saves()]);
    await w.W.close();
  }
  // 2. 합치기·다시 열 때의 번호·보기 — r1 도는 중 r2·r3 → r1 끝 → r3 저장, r2 관찰자도 성공. 닫고 다시 열어 보기만 바꾼 저장·첫 편집도 새 번호
  {
    const w = world(), mem = await two(w);
    const m = manual(mem); m.auto.add("list"); m.auto.add("load");
    const host = { store: m.store };
    const a = await openHost(w, host, { id: "wb1" }), D = a.D;
    stroke(w, a.stage, line(10, 10, 50, 50, 4)); advance(w, 2100); await settle();
    const r = D.rec("wb1"), n1 = r.running && r.running.n;
    stroke(w, a.stage, line(10, 60, 50, 90, 4)); D.save();
    const n2 = r.want && r.want.n;
    stroke(w, a.stage, line(10, 100, 50, 130, 4)); D.save();
    const n3 = r.want && r.want.n;
    ok("순서2: (준비) r1 도는 중 r2·r3 — want 는 하나(r3)로 합친다", n1 > 0 && n2 > n1 && n3 > n2 && nStrokes(r.want.text) === 4 && m.saves() === 1, [n1, n2, n3]);
    let w2 = null;
    D.watch("wb1", n2, 30000).then(v => { w2 = v; });
    m.pending("save")[0].res(); await settle();
    ok("순서2: r1 이 끝나면 r3 를 낸다 — r3 확인 전에는 미저장이고 r2 관찰자도 기다린다", m.saves() === 2 && nStrokes(m.texts[1]) === 4 && D.dirty("wb1") && w2 === null && r.done === n1, [m.saves(), w2, r.done]);
    m.pending("save")[0].res(); await settle();
    ok("순서2: r3 가 끝나면 r2 관찰자도 성공한다(done ≥ n) — 중간 r2 는 따로 쓰지 않는다", w2 === "ok" && !D.dirty("wb1") && m.saves() === 2 && r.done === n3, [w2, m.saves()]);
    let w1 = null;
    D.watch("wb1", n1, 30000).then(v => { w1 = v; }); await settle();
    ok("순서2: 이미 지난 번호(r1 < done)를 나중에 지켜봐도 곧바로 성공이다", w1 === "ok", w1);
    await w.W.close(); await settle();
    const top = n3;
    let c = await openHost(w, host, { id: "wb1" });
    dispatch(w, c.stage, "wheel", { deltaX: 0, deltaY: 40, deltaMode: 0, clientX: 5, clientY: 5 });
    let shut = null;
    w.W.close().then(v => { shut = v; }); await settle();
    const r2 = c.D.rec("wb1");
    ok("순서2: 다시 연 뒤 보기만 바꾸고 닫아도 새 번호로 save 를 낸다(옛 done 에 묻히지 않는다)", m.saves() === 3 && r2 && r2.running && r2.running.n > top && shut === null, [m.saves(), r2 && r2.running]);
    m.pending("save")[0].res(); await settle();
    ok("순서2: 그 save 가 끝나면 닫는다(본문의 보기가 바뀌었다)", shut === true && w.C.parse(await mem.load("wb1")).board.view.y !== 0);
    c = await openHost(w, host, { id: "wb1" });
    stroke(w, c.stage, line(200, 10, 260, 50, 4)); advance(w, 2100); await settle();
    const r3 = c.D.rec("wb1");
    ok("순서2: 다시 연 뒤 첫 편집도 새 번호로 save 를 낸다", m.saves() === 4 && r3.running && r3.running.n > top, r3.running);
    m.pending("save")[0].res(); await settle();
    ok("순서2: 최종 본문은 다섯 획", await strokesIn(w, mem, "wb1") === 5 && m.max === 1);
    await w.W.close();
  }
  // 3. 거부와 재시도 — r1 도는 중 r2 → r1 거부 → 닫기·다시 열기. 같은 본문 retry 가 실제로 나가고, 옛 실패가 새 관찰자를 곧바로 실패시키지 않는다. 동기 throw 도 같다
  {
    const w = world(), mem = await two(w);
    const m = manual(mem); m.auto.add("list"); m.auto.add("load");
    let arg = null;
    const host = { store: m.store, onClose(a) { arg = a; } };
    const a = await openHost(w, host, { id: "wb1" }), D = a.D;
    stroke(w, a.stage, line(10, 10, 50, 50, 4)); advance(w, 2100); await settle();
    stroke(w, a.stage, line(10, 60, 50, 90, 4)); D.save();
    const r = D.rec("wb1"), n2 = r.want.n;
    m.pending("save")[0].rej(); await settle();
    ok("순서3: r1 이 거부되면 want(r2)를 둔 채 멈춘다 — 저장됐다고 보지 않고 스스로 되풀이하지 않는다", r.halted && r.want && r.want.n === n2 && D.dirty("wb1") && m.saves() === 1
      && /저장하지 못했습니다/.test(a.root.querySelector(".itda-wb-note").textContent), [r.halted, m.saves()]);
    let wv = null;
    D.watch("wb1", n2, 30000).then(v => { wv = v; }); await settle();
    ok("순서3: 옛 실패가 새 관찰자를 곧바로 실패시키지 않는다", wv === null);
    let shut = null;
    w.W.close().then(v => { shut = v; }); await settle();
    ok("순서3: 닫기의 retry 가 같은 본문을 실제로 낸다", m.saves() === 2 && m.texts[1] === r.running.text && nStrokes(m.texts[1]) === 3, m.saves());
    m.pending("save")[0].rej(); await settle();
    ok("순서3: 다시 거부되면 첫 닫기는 닫지 않는다(관찰자도 실패)", shut === false && w.W.isOpen() && wv === "fail");
    w.W.close().then(v => { shut = v; }); await settle();
    m.pending("save")[0].rej(); await settle();
    ok("순서3: 두 번째 닫기도 거부면 기록기에 남긴 채 닫는다", shut === true && arg && arg.unsaved === true && D.dirty("wb1"));
    const b = await openHost(w, host, { id: "wb1" });
    ok("순서3: 다시 열면 want 본문으로 열고 retry 로 다시 낸다", b.D.engine.strokes.size === 3 && m.saves() === 4 && m.pending("save").length === 1);
    m.pending("save")[0].res(); await settle();
    ok("순서3: 그 save 가 되면 깨끗하다(저장 본문 세 획)", !b.D.dirty("wb1") && await strokesIn(w, mem, "wb1") === 3 && m.max === 1);
    await w.W.close();
    // 동기 throw
    const w3 = world(), mem3 = await two(w3);
    let throwOnce = true;
    const s3 = Object.assign({}, mem3, { save: (id, t) => { if (throwOnce) { throwOnce = false; throw new Error("동기 실패"); } return mem3.save(id, t); } });
    const c = await openHost(w3, { store: s3 }, { id: "wb1" });
    stroke(w3, c.stage, line(10, 10, 50, 50, 4)); advance(w3, 2100); await settle();
    const r3 = c.D.rec("wb1");
    ok("순서3: 동기 throw 도 거부와 같다(want 를 두고 멈춤·안내)", r3.halted && r3.want && !r3.running && c.D.dirty("wb1") && /저장하지 못했습니다/.test(c.root.querySelector(".itda-wb-note").textContent));
    ok("순서3: 동기 throw 뒤 닫기의 retry 가 저장하고 닫는다", await w3.W.close() === true && await strokesIn(w3, mem3, "wb1") === 2);
  }
  // 4. 떠나기·늦은 load/create — 진행 중 획·지우기를 확정해 넣고, 기한을 넘기면 원래 판에 남으며, 늦은 load·create 는 새 편집을 없애지 않는다
  {
    const w = world(), mem = await two(w);
    const m = manual(mem); m.auto.add("list"); m.auto.add("save");
    const host = { store: m.store };
    w.W.open({ host, id: "wb1" }); await settle();
    m.pending("load")[0].res(); await settle(); frames(w, 2);
    const root = surfaceOf(w), stage = root.querySelector(".itda-wb-stage"), D = w.W.debug(), eng = D.engine;
    penDown(w, stage, 300, 300);
    key(w, null, "]", "BracketRight"); await settle();
    ok("순서4: 펜을 댄 채 전환 — 그 획까지 넣어 저장이 확인된 뒤에만 대상 load 를 낸다", await strokesIn(w, mem, "wb1") === 2 && m.pending("load").length === 1 && eng.locked() && D.state.id === "wb1");
    stroke(w, stage, line(100, 100, 200, 150, 4));
    ok("순서4: load 를 기다리는 동안 그은 것은 판을 바꾸지 않는다(잠금)", eng.strokes.size === 2);
    advance(w, 8100); await settle();
    ok("순서4: 요청부터 8초가 지나면 원래 판에 남아 잠금을 푼다", D.state.id === "wb1" && !eng.locked() && /원래 판으로/.test(root.querySelector(".itda-wb-note").textContent));
    stroke(w, stage, line(100, 400, 200, 400, 4));
    m.pending("load")[0].res(); await settle(); frames(w, 2);
    ok("순서4: 늦게 온 load 는 버린다 — 새 편집이 그대로고 잠기지 않는다", D.state.id === "wb1" && eng.strokes.size === 3 && !eng.locked());
    // 지우는 중 새 판 → create 가 늦다
    key(w, null, "e", "KeyE");
    const s = lastStroke(eng), at = w.C.toScreen(eng.view, s.pts[0], s.pts[1]);
    ptr(w, stage, "pointerdown", { pointerId: 7, pointerType: "pen", clientX: at.x, clientY: at.y });
    click(w, btn(root, "list")); await settle();
    click(w, btn(root, "new")); await settle();
    ok("순서4: 지우는 중 새 판 — 지운 것까지 저장한 뒤 create 를 낸다", await strokesIn(w, mem, "wb1") === 2 && m.pending("create").length === 1 && eng.locked());
    advance(w, 8100); await settle();
    key(w, null, "Escape", "Escape"); key(w, null, "p", "KeyP");
    stroke(w, stage, line(300, 500, 400, 500, 4));
    m.pending("create")[0].res(); await settle(); frames(w, 2);
    ok("순서4: 기한 뒤 늦게 온 create 는 판을 바꾸지 않는다(새 편집 그대로)", D.state.id === "wb1" && eng.strokes.size === 3 && !eng.locked());
    // 전환 load 를 기다리는 중의 닫기 — 앞 시도를 기다리지 않는다
    key(w, null, "]", "BracketRight"); await settle();
    advance(w, 1000);
    let shut = null;
    w.W.close().then(v => { shut = v; }); await settle();
    ok("순서4: 전환 도중 닫기는 앞 시도(load)를 기다리지 않고 곧바로 닫는다", shut === true && !w.W.isOpen());
    m.pending("load")[0].res(); await settle(); frames(w, 2);
    ok("순서4: 밀려난 시도의 늦은 load 는 표면을 붙이거나 판을 바꾸지 않는다", !surfaceOf(w) && D.state.id === "wb1" && await strokesIn(w, mem, "wb1") === 3);
  }
  // 5. 복제 — 지금 판·닫아 둔 미저장 판의 save 가 거부·미완료면 기한에 철회(duplicate 0번), 그 뒤 save 성공만으로 되살지 않는다. 다시 하면 최신 본문 저장 뒤 한 번
  {
    const w = world(), mem = await two(w);
    const m = manual(mem); m.auto.add("list"); m.auto.add("load"); m.auto.add("duplicate");
    const a = await openHost(w, { store: m.store }, { id: "wb1" }), D = a.D;
    stroke(w, a.stage, line(10, 10, 50, 50, 4)); advance(w, 2100); await settle(); // save#1 붙듦
    click(w, btn(a.root, "list")); await settle();
    click(w, rowBtn(a.root, "wb1", "dup")); await settle();
    ok("순서5: 지금 판 save 가 돌면 복제는 기다린다(잠금, duplicate 없음)", D.engine.locked() && m.count("duplicate") === 0);
    advance(w, 8100); await settle();
    ok("순서5: 기한까지 깨끗해지지 않으면 철회한다(duplicate 0번, 잠금 풂, 안내)", m.count("duplicate") === 0 && !D.engine.locked() && /복제하지 않았습니다/.test(a.root.querySelector(".itda-wb-note").textContent));
    m.pending("save")[0].res(); await settle(); advance(w, 20000); await settle();
    ok("순서5: 그 뒤 save 가 끝나도 철회한 복제는 되살지 않는다", m.count("duplicate") === 0 && (await mem.list()).length === 2);
    click(w, rowBtn(a.root, "wb1", "dup")); await settle();
    ok("순서5: 다시 복제하면 깨끗한 판을 곧바로 한 번 복제한다(사본은 최신 본문)", m.count("duplicate") === 1 && await strokesIn(w, mem, "wb3") === 2);
    // 닫아 둔 미저장 판(wb2) — 거부
    D.put("wb2", boardText(w.C, 6)); await settle();
    m.pending("save")[0].rej(); await settle();
    ok("순서5: (준비) wb2 는 거부돼 기록기에 미저장으로 남았다", D.dirty("wb2") && D.rec("wb2").halted);
    click(w, rowBtn(a.root, "wb2", "dup")); await settle();
    ok("순서5: 미저장 판 복제는 먼저 retry 로 저장해 본다", m.pending("save").length === 1 && m.count("duplicate") === 1);
    m.pending("save")[0].rej(); await settle();
    ok("순서5: 그 저장이 거부되면 곧바로 철회한다(duplicate 0번)", m.count("duplicate") === 1 && /복제하지 않았습니다/.test(a.root.querySelector(".itda-wb-note").textContent));
    click(w, rowBtn(a.root, "wb2", "dup")); await settle();
    advance(w, 8100); await settle();
    m.pending("save")[0].res(); await settle(); advance(w, 20000); await settle();
    ok("순서5: 미완료로 기한을 넘긴 복제도 철회되고 뒤늦은 save 성공으로 되살지 않는다", m.count("duplicate") === 1 && !D.dirty("wb2"));
    click(w, rowBtn(a.root, "wb2", "dup")); await settle();
    const ids = (await mem.list()).map(x => x.id);
    ok("순서5: 다시 하면 정확히 한 번 복제하고 사본은 최신 본문이다", m.count("duplicate") === 2 && await strokesIn(w, mem, ids[ids.length - 1]) === 6 && m.max === 1);
    key(w, null, "Escape", "Escape");
    await w.W.close();
  }
  // 6. 삭제·이름 순서 — save 미완료 뒤 삭제는 기한에 철회(뒤늦게 지우지 않는다), 시작한 삭제가 기한을 넘기면 결과 미확정·save 를 앞지르지 않음,
  // 삭제 성공 뒤 옛 기록기의 put·retry 는 실행되지 않음, 이름 바꾸기 뒤 저장·다시 열기에서 이름이 맞음
  {
    const w = world(), mem = await two(w);
    const m = manual(mem); m.auto.add("list"); m.auto.add("load");
    const a = await openHost(w, { store: m.store }, { id: "wb1" }), D = a.D;
    stroke(w, a.stage, line(10, 10, 50, 50, 4)); advance(w, 2100); await settle(); // save#1 붙듦
    click(w, btn(a.root, "list")); await settle();
    click(w, rowBtn(a.root, "wb1", "trash")); click(w, rowBtn(a.root, "wb1", "trash")); await settle();
    ok("순서6: 도는 save 가 있으면 삭제는 시작하지 않고 기다린다(잠금)", m.count("trash") === 0 && D.engine.locked());
    advance(w, 8100); await settle();
    ok("순서6: 기한까지 시작하지 못하면 철회한다(trash 0번·잠금 풂·안내)", m.count("trash") === 0 && !D.engine.locked() && /삭제하지 않았습니다/.test(a.root.querySelector(".itda-wb-note").textContent));
    key(w, null, "Escape", "Escape");
    stroke(w, a.stage, line(10, 100, 50, 150, 4)); advance(w, 2100); await settle();
    m.pending("save")[0].res(); await settle();
    ok("순서6: 편집을 이어 가면 save 가 끝난 뒤 새 본문을 낸다 — 철회한 삭제는 뒤늦게 부르지 않는다", m.count("trash") === 0 && m.saves() === 2);
    m.pending("save")[0].res(); await settle(); advance(w, 20000); await settle();
    ok("순서6: (확인) 판은 남고 본문은 세 획", m.count("trash") === 0 && await strokesIn(w, mem, "wb1") === 3);
    // 시작한 삭제가 기한을 넘긴다
    click(w, btn(a.root, "list")); await settle();
    click(w, rowBtn(a.root, "wb1", "trash")); click(w, rowBtn(a.root, "wb1", "trash")); await settle();
    ok("순서6: 깨끗하면 곧바로 trash 를 낸다", m.pending("trash").length === 1);
    advance(w, 8100); await settle();
    ok("순서6: 시작한 삭제가 기한을 넘기면 결과 미확정으로 안내하고 잠금을 푼다", /결과를 확인하지 못했습니다/.test(a.root.querySelector(".itda-wb-note").textContent) && !D.engine.locked() && D.state.id === "wb1");
    key(w, null, "Escape", "Escape");
    const oldRec = D.rec("wb1"), saves0 = m.saves();
    stroke(w, a.stage, line(10, 200, 50, 250, 4)); advance(w, 2100); await settle();
    ok("순서6: 그동안의 편집은 기록기에 두고 save 를 내지 않는다(삭제를 앞지르지 않는다)", m.saves() === saves0 && oldRec.want && oldRec.blocked === 1);
    m.pending("trash")[0].res(); await settle(); frames(w, 2);
    advance(w, 20000); await settle();
    ok("순서6: 삭제가 늦게 되면 기록기를 폐기하고(옛 put·retry 실행 없음) 남은 판으로 옮긴다", oldRec.gone && !oldRec.want && m.saves() === saves0 && D.state.id === "wb2" && !(await mem.list()).some(x => x.id === "wb1") && !D.engine.locked(), [m.saves(), saves0, D.state.id]);
    // 이름 바꾸기 — 지금 판
    await renameTo(w, a.root, "wb2", "둘째");
    ok("순서6: 이름 바꾸기를 기다리는 동안 지금 판을 잠그고 save 를 내지 않는다", D.engine.locked() && m.pending("rename").length === 1);
    m.pending("rename")[0].res(); await settle();
    ok("순서6: 이름이 바뀌면 곧바로 본문 이름을 넣어 저장한다", m.pending("save").length === 1 && w.C.parse(m.pending("save")[0].a[1]).board.name === "둘째" && !D.engine.locked());
    m.pending("save")[0].res(); await settle();
    stroke(w, a.stage, line(300, 300, 350, 350, 4)); advance(w, 2100); await settle();
    m.pending("save")[0].res(); await settle();
    ok("순서6: 뒤이은 save 도 새 이름이다", w.C.parse(await mem.load("wb2")).board.name === "둘째");
    key(w, null, "Escape", "Escape");
    await w.W.close();
    const b = await openHost(w, { store: m.store }, { id: "wb2" });
    ok("순서6: 다시 열어도 이름이 맞는다(제목·목록·본문)", b.D.state.name === "둘째" && b.D.state.list.find(x => x.id === "wb2").name === "둘째");
    // 이름 바꾸기 — 닫힌 판의 미저장 본문
    const nb = await mem.create("셋"); await mem.save(nb.id, boardText(w.C, 1, { name: "셋" }));
    b.D.put(nb.id, boardText(w.C, 4, { name: "셋" })); await settle();
    m.pending("save")[0].rej(); await settle();
    await renameTo(w, b.root, nb.id, "셋째");
    m.pending("rename")[0].res(); await settle();
    ok("순서6: 닫힌 판의 미저장 본문은 이름을 바꿔 다시 넣는다(멈춘 기록기 — 아직 내지 않는다)", w.C.parse(b.D.latest(nb.id)).board.name === "셋째" && b.D.dirty(nb.id) && m.pending("save").length === 0);
    key(w, null, "Escape", "Escape");
    click(w, btn(b.root, "list")); await settle();
    click(w, b.root.querySelectorAll(".itda-wb-item").find(x => x.dataset.id === nb.id).querySelector(".itda-wb-iname")); await settle();
    m.pending("save").forEach(c => c.res()); await settle();
    m.pending("save").forEach(c => c.res()); await settle();
    ok("순서6: 그 판을 열면 미저장 본문으로 열고 다시 저장한다(이름·획이 맞다)", b.D.state.id === nb.id && b.D.state.name === "셋째" && w.C.parse(await mem.load(nb.id)).board.name === "셋째" && await strokesIn(w, mem, nb.id) === 4 && m.max === 1);
    // 이름 바꾸기 — 닫힌 깨끗한 판: 본문 이름은 그대로지만, 열 때 목록(호스트)의 이름을 쓰고 다음 저장에서 본문도 맞춘다
    const nd = await mem.create("넷"); await mem.save(nd.id, boardText(w.C, 1, { name: "넷" }));
    await renameTo(w, b.root, nd.id, "넷째");
    m.pending("rename")[0].res(); await settle();
    click(w, b.root.querySelectorAll(".itda-wb-item").find(x => x.dataset.id === nd.id).querySelector(".itda-wb-iname")); await settle();
    stroke(w, b.stage, line(10, 10, 50, 50, 4)); advance(w, 2100); await settle();
    m.pending("save").forEach(c => c.res()); await settle();
    ok("순서6: 닫힌 깨끗한 판의 이름을 바꾼 뒤 열면 목록의 이름을 쓰고, 다음 저장에서 본문 이름도 맞춘다", b.D.state.name === "넷째" && w.C.parse(await mem.load(nd.id)).board.name === "넷째", [b.D.state.name]);
    await w.W.close();
  }
  // 7. 저장소 경계 — 다른 store 의 같은 id 는 섞이지 않는다, 같은 객체로 다시 열면 기록기를 잇는다, store 객체를 새 래퍼로 바꾸면 잇지 않는다
  {
    const w = world(), memA = await two(w), memB = await two(w);
    const mA = manual(memA); mA.auto.add("list"); mA.auto.add("load");
    const hostA = { store: mA.store };
    const a = await openHost(w, hostA, { id: "wb1" });
    stroke(w, a.stage, line(10, 10, 50, 50, 4)); advance(w, 2100); await settle(); // A 의 save 붙듦
    const recA = a.D.rec("wb1");
    w.W.close(); await settle(); advance(w, 8100); await settle();
    ok("저장소7: (준비) A 의 wb1 은 save 가 돌고 미저장으로 닫혔다", !w.W.isOpen() && recA.running && a.D.dirty("wb1"));
    const b = await openHost(w, { store: memB }, { id: "wb1" });
    stroke(w, b.stage, line(10, 300, 50, 350, 4)); stroke(w, b.stage, line(60, 300, 90, 350, 4));
    await w.W.close();
    ok("저장소7: 다른 store 의 같은 id 는 그 store 의 본문·기록기로 열고 저장한다", b.D.rec("wb1") !== recA && await strokesIn(w, memB, "wb1") === 3 && await strokesIn(w, memA, "wb1") === 1);
    const a2 = await openHost(w, hostA, { id: "wb1" });
    ok("저장소7: 같은 store 객체로 다시 열면 기록기(want·running)를 잇는다", a2.D.rec("wb1") === recA && a2.D.engine.strokes.size === 2 && recA.running && mA.saves() === 1);
    w.W.close(); await settle(); advance(w, 8100); await settle();
    // store 객체 교체 — 같은 백엔드(memA)를 새 객체로 감싸 열면 잇지 않는다(지원하지 않는 사용 — 머리 주석 「호스트가 지킬 것」)
    const wrap = Object.assign({}, memA);
    const c = await openHost(w, { store: wrap }, { id: "wb1" });
    ok("저장소7: store 객체를 바꾸면 앞 객체의 미저장 본문을 잇지 않는다(새 객체는 저장소의 본문 — 1획 — 으로 연다)", c.D.rec("wb1") !== recA && c.D.engine.strokes.size === 1 && !c.D.dirty("wb1") && recA.running);
    await w.W.close();
    mA.pending("save")[0].res(); await settle();
    ok("저장소7: (한계) 앞 객체의 기록기는 저 혼자 끝까지 저장한다", !dirty0(recA) && await strokesIn(w, memA, "wb1") === 2);
  }
  // 8. 숨김 — 진행 획 + 도는 save + 앞 일(다른 판 이름 바꾸기) 대기 중 hidden·pagehide. 처리기가 돌아온 그 자리에서 최신 획이 want 에 있고 새 동시 save 는 없다
  {
    const w = world(), mem = await two(w);
    const m = manual(mem); m.auto.add("list"); m.auto.add("load");
    const host = { store: m.store };
    const a = await openHost(w, host, { id: "wb1" }), D = a.D;
    stroke(w, a.stage, line(10, 10, 50, 50, 4)); advance(w, 2100); await settle(); // save#1 붙듦
    await renameTo(w, a.root, "wb2", "다른 판"); // 다른 판의 이름 바꾸기가 붙들려 있다(지금 판은 잠그지 않는다)
    key(w, null, "Escape", "Escape");
    penDown(w, a.stage, 300, 300);
    w.doc.visibilityState = "hidden"; dispatch(w, w.doc, "visibilitychange", {});
    const r = D.rec("wb1");
    ok("숨김8: hidden 처리기가 돌아온 그 자리에서 진행 획까지 want 에 있고 새 save 는 없다", r.want && nStrokes(r.want.text) === 3 && m.saves() === 1 && m.live.get("wb1") === 1 && !D.engine.mode, [r.want && nStrokes(r.want.text), m.saves()]);
    w.doc.visibilityState = "visible";
    penDown(w, a.stage, 300, 400);
    dispatch(w, w.win, "pagehide", {});
    ok("숨김8: pagehide 도 그 자리에서 넣는다", r.want && nStrokes(r.want.text) === 4 && m.saves() === 1);
    let shut = null;
    w.W.close().then(v => { shut = v; }); await settle(); advance(w, 8100); await settle();
    ok("숨김8: (준비) 닫기는 8초에 닫았다(본문은 기록기에)", shut === true && D.dirty("wb1"));
    const b = await openHost(w, host, { id: "wb1" });
    ok("숨김8: 다시 열면 최신 본문으로 연다", b.D.engine.strokes.size === 4 && m.saves() === 1);
    m.pending("save")[0].res(); await settle();
    ok("숨김8: 앞 save 가 끝난 뒤 최신 want 를 낸다(동시 save 없음)", m.saves() === 2 && nStrokes(m.texts[1]) === 4 && m.max === 1);
    m.pending("save")[0].res(); m.pending("rename").forEach(c => c.res()); await settle();
    ok("숨김8: 최종 저장 본문은 네 획", await strokesIn(w, mem, "wb1") === 4 && !b.D.dirty("wb1"));
    await w.W.close();
  }

  // [8] 발표 — 열 수 있는 조건은 whiteboard() 한 곳이다
  {
    const files = ["inkshape.js", "ink.js", "present.js", "wbcore.js", "whiteboard.js"];
    const slide = () => { const p = new El("div"); p.className = "page"; p.dataset.kind = "content"; p.appendChild(new El("div")).className = "page-body"; return p; };
    const w = world(files), H = w.win.ItdaPresent;
    w.win.ItdaWhiteboardHost = { store: w.W.memoryStore() };
    H.start({ pages: [slide(), slide(), slide()], notes: [], index: 0, ratio: "16:9", onExit() {} });
    H.setBlack(true);
    ok("발표 조건: 검은 화면이면 whiteboard() 도 열지 않는다", H.whiteboard() === false && !w.W.isOpen());
    H.setBlack(false);
    H.toggleGrid(true);
    ok("발표 조건: 한눈에 보기면 whiteboard() 도 열지 않는다", H.state.grid && H.whiteboard() === false && !w.W.isOpen());
    H.toggleGrid(false);
    key(w, null, "2", "Digit2");
    ok("발표 조건: 번호 입력 중이면 whiteboard() 도 열지 않는다", H.whiteboard() === false && !w.W.isOpen());
    key(w, null, "Escape", "Escape");
    H.state.lightbox = "image";
    ok("발표 조건: 크게 보기면 whiteboard() 도 열지 않는다", H.whiteboard() === false && !w.W.isOpen());
    H.state.lightbox = null;
    ok("발표 조건: 가린 것이 없으면 연다", H.whiteboard() === true && w.W.isOpen());
    await settle(); await w.W.close();
    H.stop();
  }

  // ---- 11. 리뷰 3회차 반영: [1] 늦은 삭제 성공은 그 판을 지금 보고 있는 세션(요청한 세션이 아니어도)이 받아 잠그고 안내한 뒤 다음 판으로 옮긴다,
  // [2] 처음 열기·삭제 뒤 다음 판 열기도 요청부터 8초 하나의 기한이다(list 7초 뒤 load 지연)
  const noteOf = root => root.querySelector(".itda-wb-note").textContent;
  const busyOf = root => root.querySelector(".itda-wb-busy").textContent;
  // trashLate 는 판 둘의 store 로 wb1 을 열어 삭제를 낸 뒤 기한을 넘기고 닫는다(trash 는 붙든 채) — 리뷰어 재현의 앞부분
  async function trashLate(w, host, m) {
    const a = await openHost(w, host, { id: "wb1" });
    click(w, btn(a.root, "list")); await settle();
    click(w, rowBtn(a.root, "wb1", "trash")); click(w, rowBtn(a.root, "wb1", "trash")); await settle();
    advance(w, 8100); await settle();
    const unsure = /결과를 확인하지 못했습니다/.test(noteOf(a.root)) && m.pending("trash").length === 1;
    await w.W.close();
    return unsure;
  }
  // 폐기1 — 리뷰어 재현 차례: 삭제 기한 넘김 → 닫기 → 같은 store·판 다시 열기 → 삭제 성공 → 잠김·안내·다음 판, 그 뒤 입력은 그려지지 않는다
  {
    const w = world(), mem = await two(w);
    const m = manual(mem); for (const k of ["list", "load", "save"]) m.auto.add(k);
    let arg = null;
    const host = { store: m.store, onClose(x) { arg = x; } };
    ok("폐기1: (준비) 삭제가 기한을 넘겨 결과 미확정인 채 닫았다", await trashLate(w, host, m));
    const b = await openHost(w, host, { id: "wb1" }), D = b.D, r = D.rec("wb1");
    ok("폐기1: (준비) 같은 store 로 같은 판을 다시 열면 막힌 그 기록기를 들고 편집할 수 있다", D.state.id === "wb1" && D.state.rec === r && r.blocked === 1 && !D.engine.locked());
    stroke(w, b.stage, line(20, 200, 100, 250, 4)); // 삭제 전에 그은 획 — 막혀 있어 저장하지 못한다
    m.auto.delete("list"); // 다음 판 열기의 list 를 붙든다
    penDown(w, b.stage, 300, 400); // 삭제가 될 때 그리는 중
    const saves0 = m.saves(), n0 = D.engine.strokes.size;
    m.pending("trash")[0].res(); await settle(); frames(w, 2);
    ok("폐기1: 늦은 삭제 성공을 지금 세션이 곧바로 받는다 — 기록기 폐기·잠금(다음 판 「여는 중」)·「삭제되었습니다」 안내", r.gone && D.engine.locked() && /삭제되었습니다/.test(noteOf(b.root)) && /여는 중/.test(busyOf(b.root)), [r.gone, D.engine.locked(), noteOf(b.root), busyOf(b.root)]);
    ok("폐기1: 그리던 획은 확정되지만 저장하지 않는다(삭제 전 미저장 획은 삭제와 함께 버려진다)", D.engine.strokes.size === n0 + 1 && m.saves() === saves0 && !D.dirty("wb1"));
    ok("폐기1: 시도가 없었으니 곧바로 다음 판 열기를 새 시도로 시작한다(list 를 기다린다)", D.state.task && D.state.task.kind === "next" && m.pending("list").length === 1);
    ptr(w, b.stage, "pointerup", { pointerId: 7, pointerType: "pen", clientX: 330, clientY: 410 });
    const v0 = D.engine.view.x;
    stroke(w, b.stage, line(20, 300, 100, 350, 4));
    key(w, null, "z", "KeyZ", { metaKey: true });
    click(w, btn(b.root, "pop", { pop: "bg" })); click(w, btn(b.root, "bg", { bg: "light" }));
    advance(w, 2100); await settle();
    ok("폐기1: 잠긴 동안 그은 펜은 그려지지 않고(이동), 되돌리기·배경도 받지 않으며 save 도 없다", D.engine.strokes.size === n0 + 1 && D.engine.view.x !== v0 && D.engine.bg() === "dark" && m.saves() === saves0, [D.engine.strokes.size, m.saves()]);
    m.pending("list")[0].res(); await settle(); frames(w, 2);
    ok("폐기1: list 가 오면 남은 판(wb2)을 열고 잠금을 풀며 삭제를 안내한다", D.state.id === "wb2" && !D.engine.locked() && /삭제되었습니다 — 다음 판을 열었습니다/.test(noteOf(b.root)) && D.engine.strokes.size === 2, [D.state.id, noteOf(b.root)]);
    m.auto.add("list");
    stroke(w, b.stage, line(20, 300, 100, 350, 4)); advance(w, 2100); await settle();
    ok("폐기1: 그 뒤 획은 새 판에 그려지고 저장된다(조용히 버려지지 않는다)", await strokesIn(w, mem, "wb2") === 3 && !(await mem.list()).some(x => x.id === "wb1"));
    await w.W.close();
    ok("폐기1: 닫기 onClose 는 미저장 없음(failed 없음)", arg && arg.unsaved === false && !arg.failed, arg);
  }
  // 폐기2 — 시도가 도는 중에 폐기가 온다: 판 바꾸기의 저장 확인을 기다리는 중 → 잠긴 채(전환 실패가 잠금을 풀지 않는다) 안내하고, 그 시도가 끝난 뒤 옮긴다
  {
    const w = world(), mem = await two(w);
    const m = manual(mem); for (const k of ["list", "load", "save"]) m.auto.add(k);
    const host = { store: m.store };
    ok("폐기2: (준비) 삭제가 기한을 넘겨 결과 미확정인 채 닫았다", await trashLate(w, host, m));
    const b = await openHost(w, host, { id: "wb1" }), D = b.D, r = D.rec("wb1");
    stroke(w, b.stage, line(20, 200, 100, 250, 4)); // 막힌 기록기 — 떠나기 저장이 확인되지 않는다
    key(w, null, "]", "BracketRight"); await settle();
    ok("폐기2: (준비) 판 바꾸기가 떠나는 판의 저장을 기다린다(잠금)", D.state.task && D.state.task.kind === "switch" && D.engine.locked());
    m.auto.delete("list");
    m.pending("trash")[0].res(); await settle(); frames(w, 2);
    ok("폐기2: 시도 중에 온 폐기 — 그 시도가 끝나도 잠금이 풀리지 않고 실패 까닭 대신 삭제를 안내한다", r.gone && D.engine.locked() && /삭제되었습니다/.test(noteOf(b.root)) && !/저장하지 못해/.test(noteOf(b.root)), noteOf(b.root));
    ok("폐기2: 그 시도가 끝난 뒤 다음 판 열기를 새 시도로 시작한다", D.state.task && D.state.task.kind === "next" && m.pending("list").length === 1 && D.state.rec === null);
    const n0 = D.engine.strokes.size;
    stroke(w, b.stage, line(20, 300, 100, 350, 4));
    ok("폐기2: 그동안 그은 펜은 그려지지 않는다", D.engine.strokes.size === n0);
    m.pending("list")[0].res(); await settle(); frames(w, 2);
    ok("폐기2: 남은 판을 열고 잠금을 푼다", D.state.id === "wb2" && !D.engine.locked() && /다음 판을 열었습니다/.test(noteOf(b.root)));
    await w.W.close();
  }
  // 폐기3 — 관리 작업(다른 판 이름 바꾸기)이 도는 중에 폐기가 온다: 곧바로 잠그고, 그 작업이 끝난 뒤 옮긴다
  {
    const w = world(), mem = await two(w);
    const m = manual(mem); for (const k of ["list", "load", "save"]) m.auto.add(k);
    const host = { store: m.store };
    await trashLate(w, host, m);
    const b = await openHost(w, host, { id: "wb1" }), D = b.D;
    await renameTo(w, b.root, "wb2", "둘째");
    ok("폐기3: (준비) 다른 판 이름 바꾸기가 돈다(지금 판은 잠그지 않는다)", m.pending("rename").length === 1 && !D.engine.locked());
    m.pending("trash")[0].res(); await settle(); frames(w, 2);
    const n0 = D.engine.strokes.size;
    stroke(w, b.stage, line(20, 300, 100, 350, 4));
    ok("폐기3: 곧바로 잠그고 안내하며 시도가 끝날 때까지 옮기지 않는다", D.engine.locked() && D.engine.strokes.size === n0 && D.state.id === "wb1" && D.state.task.kind === "rename" && /삭제되었습니다/.test(noteOf(b.root)));
    m.pending("rename")[0].res(); await settle(); frames(w, 2);
    ok("폐기3: 작업이 끝나면 남은 판으로 옮긴다(이름도 맞다)", D.state.id === "wb2" && D.state.name === "둘째" && !D.engine.locked() && /다음 판을 열었습니다/.test(noteOf(b.root)), [D.state.id, D.state.name]);
    await w.W.close();
  }
  // 기한1 — 처음 열기: list 7초 뒤 load 지연 → 요청부터 8초에 표면을 떼고 거부(onClose failed), 늦은 load 는 판을 열지 않는다
  {
    const w = world(), mem = await two(w), m = manual(mem);
    let opened = null, rejected = null, arg = null, n = 0;
    w.W.open({ host: { store: m.store, onClose(x) { arg = x; n++; } }, id: "wb1" }).then(v => { opened = v; }, e => { rejected = e; }); await settle();
    advance(w, 7000); m.pending("list")[0].res(); await settle();
    ok("기한1: (준비) list 가 7초에 와서 load 를 기다린다(잠금)", m.pending("load").length === 1 && w.W.isOpen() && w.W.debug().engine.locked());
    advance(w, 1100); await settle();
    ok("기한1: 처음 열기는 list·load 를 합쳐 요청부터 8초에 표면을 떼고 open 을 거부한다", rejected && rejected.timeout && opened === null && !w.W.isOpen() && !surfaceOf(w), [opened, rejected && rejected.message]);
    ok("기한1: onClose({ unsaved: false, failed: true }) 를 한 번 부른다(안내는 호스트 몫)", n === 1 && arg && arg.unsaved === false && arg.failed === true, arg);
    m.pending("load")[0].res(); await settle(); frames(w, 2);
    ok("기한1: 늦은 load 는 표면을 붙이거나 판을 열지 않는다", !w.W.isOpen() && !surfaceOf(w) && opened === null);
    // 목록이 비었으면 create 를 기다린다 — list 7초 뒤 create 지연
    const w2 = world(), m2 = manual(w2.W.memoryStore());
    let rej2 = null, arg2 = null;
    w2.W.open({ host: { store: m2.store, onClose(x) { arg2 = x; } } }).catch(e => { rej2 = e; }); await settle();
    advance(w2, 7000); m2.pending("list")[0].res(); await settle();
    ok("기한1: (준비) 빈 목록이면 create 를 기다린다", m2.pending("create").length === 1);
    advance(w2, 1100); await settle();
    ok("기한1: create 도 같은 기한이다 — 8초에 떼고 거부, 늦은 create 는 판을 보이지 않는다", rej2 && rej2.timeout && !w2.W.isOpen() && arg2 && arg2.failed === true && arg2.unsaved === false, arg2);
    m2.pending("create")[0].res(); await settle();
    ok("기한1: (확인) 늦은 create 뒤 save 를 내지 않는다", m2.saves() === 0 && !w2.W.isOpen());
    // 새 판의 첫 save 도 같은 기한이다 — list 7초·create 7.5초 뒤 save 가 끝나지 않는다
    const w4 = world(), m4 = manual(w4.W.memoryStore());
    let rej4 = null, v4 = null, arg4 = null;
    w4.W.open({ host: { store: m4.store, onClose(x) { arg4 = x; } } }).then(v => { v4 = v; }, e => { rej4 = e; }); await settle();
    advance(w4, 7000); m4.pending("list")[0].res(); await settle();
    advance(w4, 500); m4.pending("create")[0].res(); await settle();
    ok("기한1: (준비) 만든 판의 첫 save 를 기다린다", m4.pending("save").length === 1 && w4.W.isOpen());
    advance(w4, 600); await settle();
    ok("기한1: 첫 save 를 합쳐 요청부터 8초에 떼고 거부한다(빈 본문은 기록기에 — unsaved 는 그 사실 그대로)", rej4 && rej4.timeout && v4 === null && !w4.W.isOpen() && arg4 && arg4.failed === true && arg4.unsaved === true, [v4, arg4]);
    // 기한 안이면 예전처럼 연다(list 7초·load 0.5초)
    const w3 = world(), m3 = manual(await two(w3));
    let v3 = null;
    w3.W.open({ host: { store: m3.store }, id: "wb1" }).then(v => { v3 = v; }); await settle();
    advance(w3, 7000); m3.pending("list")[0].res(); await settle();
    advance(w3, 500); m3.pending("load")[0].res(); await settle();
    ok("기한1: 8초 안에 다 오면 연다", v3 === "wb1" && w3.W.isOpen() && !w3.W.debug().engine.locked());
    await w3.W.close();
  }
  // 기한2 — 삭제 뒤 다음 판 열기: list 7초 뒤 load 지연 → 8초에 잠근 채 「판을 열지 못했습니다」·목록, 늦은 load 는 버리고, 고르면 연다
  {
    const w = world(), mem = await two(w);
    const m = manual(mem); for (const k of ["list", "load", "save", "trash"]) m.auto.add(k);
    const a = await openHost(w, { store: m.store }, { id: "wb1" }), D = a.D;
    m.auto.delete("list"); m.auto.delete("load");
    click(w, btn(a.root, "list")); await settle(); m.pending("list").forEach(c => c.res()); await settle();
    click(w, rowBtn(a.root, "wb1", "trash")); click(w, rowBtn(a.root, "wb1", "trash")); await settle();
    ok("기한2: (준비) 기한 안의 삭제 뒤 다음 판 열기가 list 를 기다린다(잠금·안내 없음)", D.state.task && D.state.task.kind === "next" && D.engine.locked() && m.pending("list").length === 1 && !/삭제되었습니다/.test(noteOf(a.root)));
    key(w, null, "Escape", "Escape"); // 목록을 닫아 둔다 — 기한을 넘기면 다시 띄워야 한다
    ok("기한2: (준비) 목록을 닫았다", a.root.querySelector(".itda-wb-list").hidden);
    advance(w, 7000); m.pending("list")[0].res(); await settle();
    ok("기한2: (준비) list 7초 뒤 wb2 load 를 기다린다", m.pending("load").some(c => c.a[0] === "wb2"));
    advance(w, 1100); await settle(); frames(w, 2);
    ok("기한2: 요청부터 8초에 잠근 채 「판을 열지 못했습니다」 와 목록을 띄운다", D.engine.locked() && /판을 열지 못했습니다/.test(noteOf(a.root)) && !a.root.querySelector(".itda-wb-list").hidden && D.state.task === null && D.state.id == null && D.engine.strokes.size === 0, [D.engine.locked(), noteOf(a.root), D.state.task]);
    const n0 = D.engine.strokes.size;
    stroke(w, a.stage, line(20, 300, 100, 350, 4));
    ok("기한2: 그 사이 그은 펜은 그려지지 않는다", D.engine.strokes.size === n0);
    m.pending("load").forEach(c => c.res()); m.pending("list").forEach(c => c.res()); await settle(); frames(w, 2);
    ok("기한2: 늦은 load 는 판을 열지 않는다(잠긴 채)", D.state.id == null && D.engine.locked());
    const row = a.root.querySelectorAll(".itda-wb-item").find(x => x.dataset.id === "wb2");
    click(w, row); await settle();
    m.pending("load").forEach(c => c.res()); await settle(); frames(w, 2);
    ok("기한2: 목록에서 고르면 그 판을 열고 잠금을 푼다", D.state.id === "wb2" && !D.engine.locked() && D.engine.strokes.size === 2, [D.state.id, D.engine.locked()]);
    await w.W.close();
  }
  // 기한2 — 남은 판이 없으면 새 판: list 7초·create 7.5초 뒤 첫 save 가 끝나지 않는다 → 8초에 잠근 채 「판을 열지 못했습니다」·목록
  {
    const w = world(), mem = w.W.memoryStore();
    await mem.create("가"); await mem.save("wb1", boardText(w.C, 1, { name: "가" }));
    const m = manual(mem); for (const k of ["list", "load", "save", "trash"]) m.auto.add(k);
    const a = await openHost(w, { store: m.store }, { id: "wb1" }), D = a.D;
    click(w, btn(a.root, "list")); await settle();
    m.auto.delete("list"); m.auto.delete("save");
    click(w, rowBtn(a.root, "wb1", "trash")); click(w, rowBtn(a.root, "wb1", "trash")); await settle();
    advance(w, 7000); m.pending("list")[0].res(); await settle();
    advance(w, 500); m.pending("create")[0].res(); await settle();
    ok("기한2: (준비) 새 판을 보이고 첫 save 를 기다린다(잠금)", m.pending("save").length === 1 && D.engine.locked() && D.state.task && D.state.task.kind === "next");
    advance(w, 600); await settle(); frames(w, 2);
    ok("기한2: 첫 save 를 합쳐 요청부터 8초에 잠근 채 「판을 열지 못했습니다」·목록", D.engine.locked() && /판을 열지 못했습니다/.test(noteOf(a.root)) && !a.root.querySelector(".itda-wb-list").hidden && D.state.task === null, [D.engine.locked(), noteOf(a.root)]);
    m.calls.filter(c => !c.done).forEach(c => c.res()); await settle();
    await w.W.close();
  }

  // ---- 12. 그림(itda-hyve#699 ④-1a 추가): 형식(images 왕복·검증 거부·없는 옛 본문), 넣기(paste·파일 고르기·안 넣는 경우·줄이기·상한),
  // 고르기 도구(고르기·풀기·겹침·격자 옮기기·크기·손가락·Esc·지우기), 되돌리기, 지우개, 그리기 차례, 디코드 실패 보존, bitmap close, 미리보기
  {
    const imgFile = (wd, ht, alpha, extra) => ({ type: alpha ? "image/png" : "image/jpeg", _img: Object.assign({ w: wd, h: ht, alpha: !!alpha }, extra) });
    const paste = (w, list, target) => dispatch(w, target || focused(w), "paste", { clipboardData: {
      items: list.map(f => (f.kind ? f : { kind: "file", type: f.type, getAsFile: () => f })), files: [] } });
    const settleImg = async w => { await settle(30); frames(w, 3); };
    const b64 = s => Buffer.from(s, "latin1").toString("base64");
    const imgData = (wd, ht, alpha) => b64("IMG " + wd + " " + ht + " " + (alpha ? 1 : 0) + " xxxx");
    const imgBoard = (C, images, o) => C.serialize(Object.assign({ name: "그림판", bg: "dark", view: { x: 0, y: 0, z: 1 }, strokes: [], images }, o));
    const imgs = eng => [...eng.images.values()].sort((a, b) => a.id - b.id);
    const noteTxt = root => root.querySelector(".itda-wb-note").hidden ? "" : root.querySelector(".itda-wb-note").textContent;
    const mouse = (w, t, type, x, y, o) => ptr(w, t, type, Object.assign({ pointerId: 1, pointerType: "mouse", clientX: x, clientY: y }, o));
    const drag = (w, t, from, to, o) => { mouse(w, t, "pointerdown", from[0], from[1], o); w.clock += 4; mouse(w, t, "pointermove", (from[0] + to[0]) / 2, (from[1] + to[1]) / 2, o); w.clock += 4; mouse(w, t, "pointermove", to[0], to[1], o); mouse(w, t, "pointerup", to[0], to[1], o); frames(w, 1); };

    // 형식
    {
      const { C } = world();
      const d1 = imgData(200, 100, false), d2 = imgData(50, 50, true);
      const text = imgBoard(C, [{ x: 1.234, y: -5, w: 320, h: 160.005, mime: "image/jpeg", data: d1 }, { x: 64, y: 64, w: 32, h: 32, mime: "image/png", data: d2 }]);
      const j = JSON.parse(text), r = C.parse(text);
      ok("그림 형식: images 는 x·y·w·h·mime·data 꼴, 넣은 차례, 좌표는 둘째 자리", Array.isArray(j.images) && Object.keys(j.images[0]).join() === "x,y,w,h,mime,data" && j.images[0].x === 1.23 && j.images[0].h === 160.01 && j.images[1].mime === "image/png", j.images);
      ok("그림 형식: 왕복 — 읽은 판을 다시 쓰면 같은 본문", r.ok && r.board.images.length === 2 && r.board.images[0].data === d1 && C.serialize(r.board) === text, r);
      ok("그림 형식: 그림이 없으면 images 키를 쓰지 않는다(옛 본문과 같다)", !("images" in JSON.parse(boardText(C, 2))) && boardText(C, 2) === C.serialize(Object.assign(C.parse(boardText(C, 2)).board, { images: [] })));
      ok("그림 형식: images 가 없는 옛 본문도 읽는다(빈 배열)", C.parse(boardText(C, 1)).ok && C.parse(boardText(C, 1)).board.images.length === 0);
      const base = () => JSON.parse(text);
      const rej = (name, mut, re, lim) => { const o = base(); mut(o); const p = C.parse(JSON.stringify(o), lim); ok("그림 검증: " + name, !p.ok && (!re || re.test(p.error)), p); };
      rej("mime 이 png·jpeg 가 아니면 거부", o => { o.images[0].mime = "image/svg+xml"; }, /mime/);
      rej("data 에 base64 가 아닌 글자가 있으면 거부", o => { o.images[0].data = "AAA!"; }, /base64/);
      rej("data 길이가 4 의 배수가 아니면 거부", o => { o.images[0].data = "AAAAA"; }, /base64/);
      rej("data 가 비면 거부", o => { o.images[0].data = ""; }, /base64/);
      rej("data 가 data URL 이면 거부", o => { o.images[0].data = "data:image/png;base64," + d2; }, /base64/);
      rej("w 가 0 이면 거부", o => { o.images[0].w = 0; }, /w·h/);
      rej("h 가 유한하지 않으면 거부", o => { o.images[0].h = "1"; }, /w·h/);
      rej("x 가 좌표 범위 밖이면 거부", o => { o.images[0].x = 2e9; }, /x·y/);
      rej("images 가 배열이 아니면 거부", o => { o.images = {}; }, /images/);
      rej("그림 수가 상한을 넘으면 거부", () => {}, /그림이 상한/, { strokes: 10, points: 10, images: 1 });
      rej("그림 바이트 합이 상한을 넘으면 거부", () => {}, /바이트/, { strokes: 10, points: 10, images: 10, imageBytes: C.b64Bytes(d1) + C.b64Bytes(d2) - 1 });
      ok("그림 검증: 상한 상수는 그림 200·24MB, 바이트 합이 딱 상한이면 읽는다", C.LIMIT.images === 200 && C.LIMIT.imageBytes === 24 * 1024 * 1024
        && C.parse(text, { strokes: 10, points: 10, images: 2, imageBytes: C.b64Bytes(d1) + C.b64Bytes(d2) }).ok);
      {
        const raw = Buffer.from([0, 1, 2, 250, 251, 252, 253, 254, 255, 7]);
        const okAll = [raw.subarray(0, 10), raw.subarray(0, 9), raw.subarray(0, 8)].every(b => { const s = b.toString("base64"); return C.b64Ok(s) && C.b64Bytes(s) === b.length && Buffer.from(C.b64Decode(s)).equals(b); });
        ok("base64: 패딩 0·1·2 개 모두 바이트 수·풀기가 맞다", okAll);
      }
      ok("그림 형식: 쓸 때 w·h 는 0.01 아래로 내려가지 않는다", JSON.parse(imgBoard(C, [{ x: 0, y: 0, w: 0.001, h: 5, mime: "image/png", data: d2 }])).images[0].w === 0.01);
      let threw = null;
      try { imgBoard(C, [{ x: 1e10, y: 0, w: 5, h: 5, mime: "image/png", data: d2 }]); } catch (e) { threw = e; }
      ok("그림 형식: 쓸 때 x 가 범위 밖이면 RangeError", threw instanceof RangeError || (threw && threw.name === "RangeError"), threw && threw.message);
    }

    // 넣기 — paste 로 화면 가운데에, 격자에 맞춰, 고르기 도구로
    {
      const w = world(), ctl = {}, store = logged(w.W.memoryStore(), ctl);
      const { root, stage, D } = await openWb(w, store);
      const eng = D.engine;
      w.W.debug().prefs.tool = "pen";
      const kv = key(w, null, "v", "KeyV", { metaKey: true }), kc = key(w, null, "v", "KeyV", { ctrlKey: true });
      ok("넣기: ⌘V·Ctrl+V 키는 기본 동작을 막지 않는다(막으면 paste 사건이 오지 않는다) — 아래 화면으로는 보내지 않고 도구도 바꾸지 않는다",
        !kv.dp && !kc.dp && kv.stopped && D.prefs.tool === "pen");
      const e = paste(w, [{ kind: "string", type: "text/plain", getAsFile: () => null }]);
      await settleImg(w);
      ok("넣기: 클립보드에 그림이 없으면 아무 일도 없다(아래 화면으로 보내지도 않는다)", eng.images.size === 0 && !e.dp && e.stopped && noteTxt(root) === "");
      const e2 = paste(w, [{ kind: "string", type: "text/plain", getAsFile: () => null }, imgFile(1000, 500, false)]);
      await settleImg(w);
      const im = imgs(eng)[0];
      ok("넣기: paste 의 image/* 항목 하나를 넣는다(기본 동작을 막는다)", e2.dp && eng.images.size === 1 && im && im.mime === "image/jpeg", eng.stats());
      ok("넣기: 화면 가운데, 긴 변 480 화면 px, 왼쪽 위와 폭이 격자(32)에 맞고 높이는 비율대로", im.x === 416 && im.y === 256 && im.w === 480 && im.h === 240, [im.x, im.y, im.w, im.h]);
      ok("넣기: 넣은 뒤 고르기 도구로 바뀌고 그 그림이 골라진다", D.prefs.tool === "select" && eng.selected() === im && stage.classList.contains("selecting"));
      ok("넣기: 디코드된 bitmap 을 든다", im.bmp && im.state === "ok" && !im.bmp.closed);
      ok("넣기: 고른 그림 옆에 「지우기」 단추", !root.querySelector(".itda-wb-imgdel").hidden && root.querySelector(".itda-wb-imgdel").style.left === "904px", root.querySelector(".itda-wb-imgdel").style.left);
      advance(w, 2100); await settle();
      const saved = w.C.parse(ctl.lastSave);
      ok("넣기: 판 파일 안에 들어가 저장된다(mime·data·자리)", saved.ok && saved.board.images.length === 1 && saved.board.images[0].data === im.data && saved.board.images[0].x === 416, saved.error);
      // 작은 그림은 원래 크기
      paste(w, [imgFile(100, 50, true)]);
      await settleImg(w);
      const small = imgs(eng)[1];
      ok("넣기: 원래 크기가 480 보다 작으면 원래 크기(폭은 격자 배수), 투명이 있으면 PNG", small && small.w === 96 && small.h === 48 && small.mime === "image/png", small && [small.w, small.h, small.mime]);
      // 확대한 채 넣으면 화면 px 기준
      eng.view = { x: eng.view.x, y: eng.view.y, z: 2 };
      paste(w, [imgFile(1000, 500, false)]);
      await settleImg(w);
      const z2 = imgs(eng)[2];
      ok("넣기: 처음 크기는 화면 px 기준(배율 2 면 판 단위 240 → 격자 배수 256)", z2 && z2.w === 256 && z2.h === 128 && z2.x % 32 === 0 && z2.y % 32 === 0, z2 && [z2.x, z2.y, z2.w]);
      await w.W.close();
    }
    // 넣지 않는 경우 — 입력 칸 초점·잠금·읽기 전용
    {
      const w = world(), mem = w.W.memoryStore();
      await mem.create("가"); await mem.save("wb1", boardText(w.C, 1, { name: "가" }));
      const { root, D } = await openWb(w, mem, { id: "wb1" });
      const eng = D.engine;
      click(w, btn(root, "list")); await settle();
      click(w, root.querySelectorAll(".itda-wb-item")[0].querySelectorAll("button").find(b => b.dataset.act === "rename")); await settle();
      const inp = root.querySelector(".itda-wb-rename");
      const e = paste(w, [imgFile(300, 300, false)]);
      await settleImg(w);
      ok("안 넣음: 이름 칸에 초점이 있으면 글자 붙여넣기로 두고 그림을 넣지 않는다", inp && w.doc.activeElement === inp && eng.images.size === 0 && !e.dp && !e.stopped);
      key(w, null, "Escape", "Escape"); key(w, null, "Escape", "Escape");
      eng.lock(true);
      paste(w, [imgFile(300, 300, false)]);
      await settleImg(w);
      ok("안 넣음: 잠긴 동안", eng.images.size === 0);
      eng.lock(false);
      // 준비하는 사이 잠기면 버린다
      paste(w, [imgFile(300, 300, false)]);
      eng.lock(true);
      await settleImg(w);
      ok("안 넣음: 준비하는 사이 잠기면 버린다", eng.images.size === 0);
      eng.lock(false);
      await w.W.close();
      const w2 = world(), m2 = w2.W.memoryStore();
      await m2.create("새"); await m2.save("wb1", JSON.stringify(Object.assign(JSON.parse(imgBoard(w2.C, [{ x: 0, y: 0, w: 320, h: 160, mime: "image/jpeg", data: imgData(200, 100) }])), { v: 2 })));
      const b = await openWb(w2, m2, { id: "wb1" });
      await settleImg(w2);
      paste(w2, [imgFile(300, 300, false)]);
      await settleImg(w2);
      ok("안 넣음: 읽기 전용 판(안내한다) — 그 판의 그림은 보인다", b.D.engine.readOnly() && b.D.engine.images.size === 1 && imgs(b.D.engine)[0].state === "ok" && /읽기 전용/.test(noteTxt(b.root)), noteTxt(b.root));
      key(w2, null, "v", "KeyV");
      const v0 = b.D.engine.view.x;
      drag(w2, b.stage, [100, 100], [150, 100]);
      ok("안 넣음: 읽기 전용이면 고르기 도구의 끌기도 이동이다(그림은 그대로)", imgs(b.D.engine)[0].x === 0 && b.D.engine.view.x !== v0 && b.D.engine.selected() === null);
      ok("안 넣음: 읽기 전용이면 「그림」 단추가 꺼진다", btn(b.root, "image").disabled);
      await w2.W.close();
    }
    // 파일 고르기·줄이기·상한
    {
      const w = world();
      const { root, D } = await openWb(w, w.W.memoryStore());
      const eng = D.engine, fileIn = root.querySelector(".itda-wb-file");
      click(w, btn(root, "image"));
      ok("파일 고르기: 「그림」 단추가 accept=image/* 파일 칸을 연다", fileIn && fileIn.type === "file" && fileIn.accept === "image/*" && fileIn.clicked === 1 && fileIn.hidden);
      fileIn.files = [imgFile(4000, 3000, false)];
      w.encodes.length = 0;
      dispatch(w, fileIn, "change", {});
      await settleImg(w);
      const a = imgs(eng)[0];
      ok("파일 고르기: 고른 그림을 넣는다", eng.images.size === 1 && a);
      ok("줄이기: 긴 변이 2000 을 넘으면 2000 으로 줄여 JPEG 로 인코드", w.encodes.length === 1 && w.encodes[0].w === 2000 && w.encodes[0].h === 1500 && w.encodes[0].mime === "image/jpeg", w.encodes);
      ok("줄이기: 판 본문에 든 그림이 줄인 크기다", /^IMG 2000 1500 0/.test(Buffer.from(a.data, "base64").toString("latin1")) && a.w === 480 && a.h === 360);
      w.imgK = 1; w.encodes.length = 0;
      paste(w, [imgFile(4000, 3000, false)]);
      await settleImg(w);
      const sizes = w.encodes.map(x => x.w + "x" + x.h).join();
      ok("줄이기: 1.5MB 를 넘으면 긴 변을 0.75 배씩 줄여 다시 인코드", sizes === "2000x1500,1500x1125,1125x844" && eng.images.size === 2 && imgs(eng)[1].bytes <= 1.5 * 1024 * 1024, sizes);
      w.imgK = 100; w.encodes.length = 0;
      paste(w, [imgFile(4000, 3000, false)]);
      await settleImg(w);
      ok("상한: 4번 줄여도 1.5MB 를 넘으면 넣지 않고 안내(인코드 5번)", eng.images.size === 2 && w.encodes.length === 5 && /1\.5MB/.test(noteTxt(root)), [w.encodes.length, noteTxt(root)]);
      w.imgK = 0.05;
      paste(w, [imgFile(10, 10, false, { bad: true })]);
      await settleImg(w);
      ok("상한: 디코드하지 못하는 그림은 넣지 않고 안내", eng.images.size === 2 && /읽지 못해/.test(noteTxt(root)), noteTxt(root));
      w.C.LIMIT.images = 2;
      paste(w, [imgFile(100, 100, false)]);
      await settleImg(w);
      ok("상한: 그림 수 상한이면 넣지 않고 안내", eng.images.size === 2 && /가득/.test(noteTxt(root)), noteTxt(root));
      w.C.LIMIT.images = 200; w.C.LIMIT.imageBytes = eng.stats().imageBytes + 10;
      paste(w, [imgFile(100, 100, false)]);
      await settleImg(w);
      ok("상한: 그림 바이트 합 상한이면 넣지 않고 안내", eng.images.size === 2 && /가득/.test(noteTxt(root)));
      w.C.LIMIT.imageBytes = 24 * 1024 * 1024;
      await w.W.close();
    }
    // 고르기 도구 — 고르기·풀기·겹침·격자 옮기기·크기·되돌리기·지우기·Esc
    {
      const w = world(), ctl = {}, store = logged(w.W.memoryStore(), ctl);
      const { root, stage, D } = await openWb(w, store);
      const eng = D.engine;
      paste(w, [imgFile(1000, 500, false)]);
      await settleImg(w);
      const im = imgs(eng)[0];
      eng.select(null); frames(w, 1);
      ok("고르기: 풀면 「지우기」 단추를 숨긴다", root.querySelector(".itda-wb-imgdel").hidden);
      mouse(w, stage, "pointerdown", 500, 300); mouse(w, stage, "pointerup", 500, 300); frames(w, 1);
      ok("고르기: 그림을 누르면 골라진다(움직이지 않으면 되돌리기 단계가 없다)", eng.selected() === im && eng.stats().undo === 1);
      mouse(w, stage, "pointerdown", 100, 100); mouse(w, stage, "pointerup", 100, 100);
      ok("고르기: 빈 곳을 누르면 풀린다", eng.selected() === null);
      const v0 = eng.view.x;
      drag(w, stage, [100, 100], [160, 100]);
      ok("고르기: 빈 곳을 마우스로 끌면 판 이동", eng.view.x === v0 - 60 && im.x === 416, [eng.view.x, v0]);
      drag(w, stage, [160, 100], [100, 100]);
      // 옮기기
      ctl.log = [];
      mouse(w, stage, "pointerdown", 500, 300); w.clock += 4;
      mouse(w, stage, "pointermove", 530, 310);
      for (const c of w.canvases) if (c.ctx) c.ctx.ops.length = 0; // 이 프레임에 그린 것만 본다
      frames(w, 1);
      ok("옮기기: 끄는 동안 옅은 격자(바탕이 격자가 아니어도)·조작 중 자리는 mode.g(그림은 뗄 때까지 제자리)", eng.snapping() && stage.classList.contains("snapping") && im.x === 416 && eng.mode.g.x === 448, eng.mode && eng.mode.g);
      {
        const rebaked = w.canvases.filter(c => c.width === 512 && c.ctx && c.ctx.ops.includes("clear")), liveC = stage.querySelector(".itda-wb-live"), imgC = stage.querySelector(".itda-wb-img");
        const last = imgC.ctx.draws[imgC.ctx.draws.length - 1];
        ok("옮기기: 끄는 그림은 그림 층에 지금 자리로 그리고, live 층에는 그림이 없고, 획 타일은 다시 굽지 않는다",
          rebaked.length === 0 && !liveC.ctx.ops.includes("img") && last && last.src === im.bmp && last.tx === 448 * 2 + Math.round(-eng.view.x * 2), [rebaked.length, liveC.ctx.ops, last && last.tx]);
      }
      mouse(w, stage, "pointermove", 550, 320);
      advance(w, 2100); await settle();
      ok("옮기기: 끄는 동안은 저장하지 않는다", ctl.log.filter(x => /^save/.test(x)).length === 0, ctl.log);
      mouse(w, stage, "pointerup", 550, 320); frames(w, 1);
      ok("옮기기: 왼쪽 위가 격자에 붙는다(+50,+20 → 480,288)", im.x === 480 && im.y === 288 && im.w === 480 && im.h === 240 && !im.live && !eng.snapping() && !stage.classList.contains("snapping"), [im.x, im.y]);
      advance(w, 2100); await settle();
      ok("옮기기: 뗀 뒤 자동 저장", w.C.parse(ctl.lastSave).board.images[0].x === 480);
      // 크기 — 오른쪽 아래 귀
      mouse(w, stage, "pointerdown", 962, 530); w.clock += 4; // 귀(960, 528) 옆
      mouse(w, stage, "pointermove", 1080, 600); mouse(w, stage, "pointerup", 1080, 600); frames(w, 1);
      ok("크기: 귀를 끌면 비율을 지키고 폭이 격자 배수, 맞은편 귀(왼쪽 위) 고정", im.x === 480 && im.y === 288 && im.w === 608 && im.h === 304, [im.x, im.y, im.w, im.h]);
      mouse(w, stage, "pointerdown", 1088, 592); w.clock += 4;
      mouse(w, stage, "pointermove", 300, 100); mouse(w, stage, "pointerup", 300, 100); frames(w, 1);
      ok("크기: 최소 폭은 격자 한 칸", im.w === 32 && im.h === 16 && im.x === 480 && im.y === 288, [im.w, im.h]);
      // 왼쪽 위 귀 — 오른쪽 아래 고정
      mouse(w, stage, "pointerdown", 481, 289); w.clock += 4;
      mouse(w, stage, "pointermove", 300, 200); mouse(w, stage, "pointerup", 300, 200); frames(w, 1);
      ok("크기: 왼쪽 위 귀를 끌면 오른쪽 아래가 고정, 움직인 귀의 x 가 격자에", im.x + im.w === 512 && near(im.y + im.h, 304) && im.x % 32 === 0 && im.w === 224 && near(im.h, 112), [im.x, im.y, im.w, im.h]);
      // 되돌리기·다시 하기 — 크기·옮기기·넣기
      const geo = () => [im.x, im.y, im.w, im.h].join();
      key(w, null, "z", "KeyZ", { metaKey: true }); frames(w, 1);
      ok("되돌리기: 크기 한 단계", geo() === "480,288,32,16", geo());
      key(w, null, "z", "KeyZ", { metaKey: true }); key(w, null, "z", "KeyZ", { metaKey: true }); key(w, null, "z", "KeyZ", { metaKey: true });
      ok("되돌리기: 옮기기 한 단계", geo() === "416,256,480,240", geo());
      key(w, null, "z", "KeyZ", { metaKey: true });
      ok("되돌리기: 넣기 한 단계(그림이 빠지고 고르기가 풀린다)", eng.images.size === 0 && eng.selected() === null);
      key(w, null, "z", "KeyZ", { metaKey: true, shiftKey: true }); key(w, null, "z", "KeyZ", { metaKey: true, shiftKey: true });
      ok("다시 하기: 넣기·옮기기", eng.images.size === 1 && geo() === "480,288,480,240", geo());
      // 지우기 — Delete 키·되돌리기·Backspace·단추
      eng.select(im); key(w, null, "Delete", "Delete");
      ok("지우기: Delete 로 고른 그림을 지운다", eng.images.size === 0 && eng.selected() === null);
      key(w, null, "z", "KeyZ", { metaKey: true });
      ok("되돌리기: 지우기 한 단계(되살린 그림은 bitmap 을 그대로 든다)", eng.images.size === 1 && im.bmp && !im.bmp.closed);
      eng.select(im); key(w, null, "Backspace", "Backspace");
      ok("지우기: Backspace", eng.images.size === 0);
      key(w, null, "z", "KeyZ", { metaKey: true });
      eng.select(im); frames(w, 1);
      click(w, root.querySelector(".itda-wb-imgdel"));
      ok("지우기: 고른 그림 옆 「지우기」 단추", eng.images.size === 0);
      key(w, null, "z", "KeyZ", { metaKey: true });
      // 겹침 — 나중에 넣은 것이 잡힌다
      paste(w, [imgFile(1000, 500, false)]);
      await settleImg(w);
      const top = imgs(eng)[1];
      eng.select(null);
      mouse(w, stage, "pointerdown", 600, 330); mouse(w, stage, "pointerup", 600, 330);
      ok("겹침: 겹친 곳을 누르면 나중에 넣은 그림이 골라진다", top && eng.selected() === top && top.id > im.id);
      // Esc 차례
      key(w, null, "Escape", "Escape");
      ok("Esc: 고르기 풀기가 먼저(표면은 열린 채)", eng.selected() === null && w.W.isOpen());
      // 지우개는 그림을 지우지 않는다, 펜은 그림 위에 그린다
      key(w, null, "e", "KeyE");
      drag(w, stage, [420, 260], [900, 500]);
      ok("지우개: 그림을 지우지 않는다", eng.images.size === 2);
      key(w, null, "p", "KeyP");
      const n0 = eng.strokes.size;
      stroke(w, stage, line(500, 300, 700, 400, 6));
      ok("펜: 그림 위에 그대로 그린다(고르기 도구가 아니면 그림을 잡지 않는다)", eng.strokes.size === n0 + 1 && imgs(eng)[1].x === top.x);
      await w.W.close();
    }
    // 손가락 — 고르기 도구에서 그림 위에서 시작할 때만 조작
    {
      const w = world();
      const { stage, D } = await openWb(w, w.W.memoryStore());
      const eng = D.engine;
      paste(w, [imgFile(1000, 500, false)]);
      await settleImg(w);
      const im = imgs(eng)[0];
      const touch = (type, id, x, y) => ptr(w, stage, type, { pointerId: id, pointerType: "touch", clientX: x, clientY: y, pressure: 0 });
      const v0 = eng.view.x;
      touch("pointerdown", 21, 100, 100); w.clock += 4; touch("pointermove", 21, 160, 100); touch("pointerup", 21, 160, 100);
      ok("손가락: 고르기 도구에서도 빈 곳에서 시작하면 이동(그림은 그대로·고르기도 그대로)", eng.view.x === v0 - 60 && im.x === 416 && eng.selected() === im);
      touch("pointerdown", 22, 160, 100); touch("pointerup", 22, 160, 100);
      ok("손가락: 빈 곳을 톡 치면 고르기를 푼다", eng.selected() === null);
      eng.view = { x: v0, y: eng.view.y, z: 1 };
      touch("pointerdown", 23, 500, 300); w.clock += 4; touch("pointermove", 23, 600, 300); touch("pointerup", 23, 600, 300);
      ok("손가락: 고르기 도구에서 그림 위에서 시작하면 그림을 옮긴다", im.x === 512 && eng.view.x === v0, [im.x, eng.view.x]);
      touch("pointerdown", 24, 600, 300); w.clock += 4; touch("pointermove", 24, 700, 300);
      touch("pointerdown", 25, 800, 500);
      ok("손가락: 두 손가락이 되면 하던 그림 조작은 제자리로, 오므리기", im.x === 512 && eng.mode && eng.mode.kind === "pinch" && eng.stats().undo === 2);
      touch("pointerup", 24, 700, 300); touch("pointerup", 25, 800, 500);
      key(w, null, "p", "KeyP");
      const v1 = eng.view.x;
      touch("pointerdown", 26, 600, 300); w.clock += 4; touch("pointermove", 26, 650, 300); touch("pointerup", 26, 650, 300);
      ok("손가락: 펜 도구에서는 그림 위에서 시작해도 이동", im.x === 512 && eng.view.x === v1 - 50);
      key(w, null, "v", "KeyV");
      ptr(w, stage, "pointerdown", { pointerId: 27, pointerType: "pen", clientX: 100, clientY: 600, pressure: 0.5 }); w.clock += 4;
      ptr(w, stage, "pointermove", { pointerId: 27, pointerType: "pen", clientX: 130, clientY: 600, pressure: 0.5 });
      ptr(w, stage, "pointerup", { pointerId: 27, pointerType: "pen", clientX: 130, clientY: 600, pressure: 0 });
      ok("펜: 고르기 도구에서 빈 곳을 끌면 판 이동", eng.view.x === v1 - 80 && eng.strokes.size === 0);
      await w.W.close();
    }
    // 그리기 차례·디코드 실패 보존·bitmap close·미리보기
    {
      const w = world(), ctl = {}, mem = w.W.memoryStore(), store = logged(mem, ctl);
      await mem.create("가"); await mem.create("나");
      const bad = b64("깨진 그림 바이트".replace(/[^\x00-\x7f]/g, "?"));
      await mem.save("wb1", imgBoard(w.C, [{ x: 0, y: 0, w: 320, h: 160, mime: "image/jpeg", data: imgData(200, 100) }, { x: 400, y: 0, w: 64, h: 64, mime: "image/png", data: bad }], { name: "가" }));
      await mem.save("wb2", imgBoard(w.C, [{ x: 0, y: 0, w: 320, h: 160, mime: "image/png", data: imgData(80, 40, true) }], { name: "나" }));
      const { root, stage, D } = await openWb(w, store, { id: "wb1" });
      await settleImg(w);
      const eng = D.engine, [good, broken] = imgs(eng);
      ok("열기: 판을 열면 그림을 디코드해 든다", good.state === "ok" && good.bmp && !good.bmp.closed);
      ok("디코드 실패: 자리 표시로 남는다(판에서 지우지 않는다)", broken.state === "bad" && !broken.bmp && eng.images.size === 2);
      stroke(w, stage, line(10, 10, 300, 150, 6));
      advance(w, 2100); await settle();
      const sv = w.C.parse(ctl.lastSave);
      ok("디코드 실패: 다시 저장해도 그 그림의 data 를 잃지 않는다", sv.ok && sv.board.images.length === 2 && sv.board.images[1].data === bad && sv.board.strokes.length === 1);
      // 그리기 차례 — 그림은 그림 층(획 층 아래)에, 획 타일에는 굽지 않는다. 그림을 넣어도 획 타일을 버리지 않는다
      const imgC = stage.querySelector(".itda-wb-img");
      ok("층 차례: stage 안 canvas 는 아래부터 그림 → 획 타일(base) → live", stage.kids.filter(k => k.tag === "canvas").map(k => k.className).join() === "itda-wb-img,itda-wb-base,itda-wb-live");
      for (const c of w.canvases) if (c.ctx) { c.ctx.ops.length = 0; c.ctx.draws.length = 0; }
      paste(w, [imgFile(1000, 1000, false)]);
      await settleImg(w);
      const tilesNow = w.canvases.filter(c => c.width === 512 && c.ctx && c.ctx.ops.includes("clear"));
      const newest = imgs(eng)[2];
      ok("그리기 차례: 그림을 넣으면 그림 층만 다시 그린다(획 타일을 다시 굽지 않는다)", tilesNow.length === 0 && imgC.ctx.draws.some(d => d.src === newest.bmp), tilesNow.length);
      ok("그리기 차례: 획 타일에는 그림이 없다", w.canvases.filter(c => c.width === 512 && c.ctx).every(c => !c.ctx.ops.includes("img")));
      imgC.ctx.ops.length = 0;
      eng.setBg("light"); frames(w, 1);
      ok("그리기 차례: 바탕을 바꾸면 그림 층도 다시 그린다(자리 표시 색이 바탕을 따른다)", imgC.ctx.ops.includes("clear") && imgC.ctx.ops.includes("rect"), imgC.ctx.ops);
      eng.setBg("dark"); frames(w, 1);
      const before = w.bitmaps.length;
      key(w, null, "]", "BracketRight"); await settleImg(w);
      ok("떠나기: 판을 바꾸면 앞 판의 bitmap 을 모두 close 한다", D.state.id === "wb2" && good.bmp === null && w.bitmaps.slice(0, before).every(b => b.closed), w.bitmaps.map(b => b.closed));
      const cur2 = imgs(eng)[0];
      ok("떠나기: 새 판의 그림을 디코드한다", cur2 && cur2.state === "ok" && !cur2.bmp.closed);
      // 미리보기 — 다른 판(wb1)의 그림을 디코드해 그리고 곧바로 close
      const nb = w.bitmaps.length;
      click(w, btn(root, "list")); await settleImg(w);
      const fresh = w.bitmaps.slice(nb);
      const thumbs = root.querySelectorAll(".itda-wb-thumb");
      ok("미리보기: 다른 판의 그림을 디코드해 그리고 곧바로 close 한다", fresh.length >= 2 && fresh.every(b => b.closed) && thumbs.some(c => c.ctx && c.ctx.ops.includes("canvas")), [fresh.length, fresh.map(b => b.closed)]);
      ok("미리보기: 지금 판은 엔진이 든 bitmap 으로 그린다", thumbs.some(c => c.ctx && c.ctx.ops.includes("img")) && !cur2.bmp.closed);
      key(w, null, "Escape", "Escape");
      await w.W.close();
      ok("닫기: 표면을 떼면 든 bitmap 을 모두 close 한다", w.bitmaps.every(b => b.closed), w.bitmaps.map(b => b.closed));
    }
    // 겹침 차례(리뷰 1회차 [3]) — 겹친 두 그림 A(먼저)·B 와 획: 조작 전·중·후 그림 층의 그리기 차례가 넣은 차례 그대로(층 차례로 획 아래),
    // 끄는 그림은 그림 층의 제 차례에 지금 자리로, live 층에는 그림이 없고, 그림만 옮기면 획 타일을 다시 굽지 않는다. 획 타일 굽기에 그림이 없다.
    {
      const w = world(), mem = w.W.memoryStore();
      await mem.create("겹침");
      await mem.save("wb1", imgBoard(w.C, [{ x: 0, y: 0, w: 320, h: 320, mime: "image/jpeg", data: imgData(100, 100) }, { x: 160, y: 160, w: 320, h: 320, mime: "image/jpeg", data: imgData(100, 100) }],
        { name: "겹침", strokes: [{ tool: "pen", color: 0, w: 4, pts: [10, 10, 0.5, 200, 200, 0.5, 400, 400, 0.5] }] }));
      const { stage, D } = await openWb(w, mem, { id: "wb1" });
      await settleImg(w);
      const eng = D.engine, [A, B] = imgs(eng), imgC = stage.querySelector(".itda-wb-img"), liveC = stage.querySelector(".itda-wb-live");
      const reset = () => { for (const c of w.canvases) if (c.ctx) { c.ctx.ops.length = 0; c.ctx.draws.length = 0; } };
      const drawn = () => imgC.ctx.draws.map(d => (d.src === A.bmp ? "A" : d.src === B.bmp ? "B" : "?")).join();
      const txOf = im => { const d = imgC.ctx.draws.find(x => x.src === im.bmp); return d ? d.tx : NaN; };
      const rebaked = () => w.canvases.filter(c => c.width === 512 && c.ctx && c.ctx.ops.includes("clear")).length;
      const ox = () => Math.round(-eng.view.x * 2);
      key(w, null, "v", "KeyV");
      reset(); eng.view = { x: 1, y: 0, z: 1 }; frames(w, 1);
      const before = drawn();
      reset();
      mouse(w, stage, "pointerdown", 100, 100); w.clock += 4; mouse(w, stage, "pointermove", 300, 300); frames(w, 1);
      const during = drawn(), duringTx = txOf(A), liveImg = liveC.ctx.ops.includes("img"), rbDuring = rebaked();
      reset();
      mouse(w, stage, "pointerup", 300, 300); frames(w, 1);
      const after = drawn(), afterTx = txOf(A), rbAfter = rebaked();
      ok("겹침 차례: 조작 전·중·후 그림 층의 그리기 차례가 넣은 차례(A → B) 그대로", before === "A,B" && during === "A,B" && after === "A,B", [before, during, after]);
      ok("겹침 차례: 끄는 동안 A 는 그림 층의 제 차례에 지금 자리(192)로, 뗀 뒤 그 자리에 남는다", eng.selected() === A && duringTx === 192 * 2 + ox() && A.x === 192 && afterTx === duringTx, [duringTx, afterTx, A.x]);
      ok("겹침 차례: 끄는 동안 live 층에는 그림이 없다(테두리·손잡이만)", !liveImg && eng.strokes.size === 1);
      ok("겹침 차례: 그림만 옮기면 획 타일을 다시 굽지 않는다(끄는 동안·뗀 뒤)", rbDuring === 0 && rbAfter === 0, [rbDuring, rbAfter]);
      reset(); eng.view = { x: 1, y: 0, z: 1.5 }; advance(w, 400); frames(w, 3);
      const baked = w.canvases.filter(c => c.width === 512 && c.ctx && c.ctx.ops.includes("clear") && c.ctx.ops.includes("fill"));
      ok("겹침 차례: 획 타일 굽기에 그림이 없다(배율이 바뀌어 다시 구워도)", baked.length > 0 && baked.every(c => !c.ctx.ops.includes("img")) && drawn() === "A,B", [baked.length, drawn()]);
      ok("층 차례: 그림 층·획 층·live 층이 stage 안에 그 차례로(뒤의 것이 위)", stage.kids.filter(k => k.tag === "canvas").map(k => k.className).join() === "itda-wb-img,itda-wb-base,itda-wb-live");
      // 끄는 중 휠로 판을 옮겨 처음 자리가 화면 밖이 되어도, 지금 자리가 화면 안이면 그 그림을 그린다(색인은 처음 자리를 든다)
      eng.view = { x: 0, y: 0, z: 1 }; advance(w, 400); frames(w, 3);
      mouse(w, stage, "pointerdown", 500, 500); w.clock += 4;
      dispatch(w, stage, "wheel", { deltaX: 3000, deltaY: 0, deltaMode: 0, clientX: 500, clientY: 500 });
      reset();
      mouse(w, stage, "pointermove", 600, 500); frames(w, 1);
      const offTx = txOf(A);
      mouse(w, stage, "pointerup", 600, 500); frames(w, 1);
      ok("겹침 차례: 끄는 그림의 처음 자리가 화면 밖이어도 지금 자리(화면 안)에 그린다", eng.view.x === 3000 && offTx === 3296 * 2 - 6000 && A.x === 3296, [eng.view.x, offTx, A.x]);
      await w.W.close();
    }
    // 크기 조절의 유효성(리뷰 1회차 [1]) — 네 귀 × 좌표 양끝 × 작은 그림(16×16)·극단 비율에서 조절 뒤 serialize 가 던지지 않고 닫힌다.
    {
      const M = 1e9;
      const inside = im => [im.x, im.y, im.w, im.h].every(Number.isFinite) && im.w >= 0.01 && im.h >= 0.01 && im.w <= M && im.h <= M && im.x >= -M && im.y >= -M && im.x + im.w <= M && im.y + im.h <= M;
      const cases = [
        { name: "왼쪽 위 끝 16×16", g: { x: -M, y: -M, w: 16, h: 16 }, view: { x: -M, y: -M, z: 8 } },
        { name: "오른쪽 아래 끝 16×16", g: { x: M - 16, y: M - 16, w: 16, h: 16 }, view: { x: M, y: M, z: 8 } },
        { name: "왼쪽 끝 16×16(리뷰어 재현 자리)", g: { x: -M, y: 64, w: 16, h: 16 }, view: { x: -M, y: 0, z: 8 } },
        { name: "오른쪽 위 끝 16×16", g: { x: M - 16, y: -M, w: 16, h: 16 }, view: { x: M, y: -M, z: 8 } },
        { name: "납작한 그림 4096×0.02 왼쪽 끝", g: { x: -M, y: 0, w: 4096, h: 0.02 }, view: { x: -M, y: -10, z: 0.25 } },
        { name: "긴 그림 0.02×4096 아래 끝", g: { x: 0, y: M - 4096, w: 0.02, h: 4096 }, view: { x: -10, y: M, z: 0.25 } },
        { name: "아주 긴 그림 0.01×1e6 위 끝", g: { x: 0, y: -M, w: 0.01, h: 1e6 }, view: { x: -10, y: -M, z: 1 } },
      ];
      const moves = [[-1, -1], [1, 1], [-300, -300], [300, 300], [-300, 300], [300, -300]];
      for (const cs of cases) {
        const w = world(), mem = w.W.memoryStore();
        const text = imgBoard(w.C, [Object.assign({ mime: "image/png", data: imgData(16, 16, true) }, cs.g)], { name: "경계", view: cs.view });
        await mem.create("경계"); await mem.save("wb1", text);
        const { D, stage } = await openWb(w, mem, { id: "wb1" });
        await settleImg(w);
        const eng = D.engine, im = imgs(eng)[0], bad = [];
        key(w, null, "v", "KeyV");
        for (let c = 0; c < 4; c++) for (const [dx, dy] of moves) {
          eng.select(im); frames(w, 1);
          const a = w.C.toScreen(eng.view, c === 1 || c === 2 ? im.x + im.w : im.x, c === 2 || c === 3 ? im.y + im.h : im.y);
          mouse(w, stage, "pointerdown", a.x, a.y); w.clock += 4;
          mouse(w, stage, "pointermove", a.x + dx / 2, a.y + dy / 2); w.clock += 4;
          if (eng.mode && eng.mode.g && !inside(eng.mode.g)) bad.push({ c, dx, dy, during: eng.mode.g }); // 조작 중 자리도 늘 유효하다
          mouse(w, stage, "pointermove", a.x + dx, a.y + dy);
          if (eng.mode && eng.mode.g && !inside(eng.mode.g)) bad.push({ c, dx, dy, during: eng.mode.g });
          mouse(w, stage, "pointerup", a.x + dx, a.y + dy); frames(w, 1);
          let err = null;
          try { w.C.serialize(eng.board()); } catch (e) { err = e.message; }
          if (err || !inside(im)) bad.push({ c, dx, dy, x: im.x, y: im.y, w: im.w, h: im.h, err });
        }
        const closed = await w.W.close();
        ok("크기 유효성: " + cs.name + " — 네 귀·여러 방향 조절 뒤 serialize 가 던지지 않고 그림이 범위 안, 닫힌다", bad.length === 0 && closed === true, bad.slice(0, 3));
      }
      // 리뷰어 재현 값 그대로: x=-1e9, y=64, w=h=16, z=8, 왼쪽 위 귀에서 1px
      {
        const w = world(), mem = w.W.memoryStore();
        await mem.create("경계"); await mem.save("wb1", imgBoard(w.C, [{ x: -M, y: 64, w: 16, h: 16, mime: "image/png", data: imgData(16, 16) }], { name: "경계", view: { x: -M, y: 0, z: 8 } }));
        const { D, stage } = await openWb(w, mem, { id: "wb1" });
        await settleImg(w);
        const eng = D.engine, im = imgs(eng)[0];
        eng.setTool("select"); eng.select(im); frames(w, 2);
        const a = w.C.toScreen(eng.view, im.x, im.y);
        ptr(w, stage, "pointerdown", { pointerId: 91, clientX: a.x, clientY: a.y });
        ptr(w, stage, "pointermove", { pointerId: 91, clientX: a.x + 1, clientY: a.y + 1 });
        ptr(w, stage, "pointermove", { pointerId: 91, clientX: a.x - 1, clientY: a.y - 1 });
        ptr(w, stage, "pointerup", { pointerId: 91, clientX: a.x - 1, clientY: a.y - 1 });
        let err = null;
        try { w.C.serialize(eng.board()); } catch (e) { err = e.message; }
        ok("크기 유효성: 리뷰어 재현(x=-1e9·16×16·z=8·왼쪽 위 귀 1px) — 범위 밖을 만들지 않고 그 움직임을 적용하지 않는다", !err && im.x === -M && im.w === 16 && im.h === 16 && eng.stats().undo === 0, [err, im.x, im.w]);
        ok("크기 유효성: 리뷰어 재현 뒤 닫기가 된다", await w.W.close() === true);
      }
      // 납작한 그림의 높이 하한(IMG_MIN)·범위를 넘어 걸친 옛 그림·뗄 때 다시 보기
      {
        const w = world(), mem = w.W.memoryStore();
        await mem.create("경계");
        await mem.save("wb1", imgBoard(w.C, [{ x: 0, y: 0, w: 4096, h: 0.02, mime: "image/png", data: imgData(16, 16) },
          { x: M - 10, y: 200, w: 100, h: 100, mime: "image/png", data: imgData(16, 16) }, { x: 0, y: 400, w: 9344, h: 0.02, mime: "image/png", data: imgData(16, 16) }],
          { name: "경계", view: { x: -10, y: -10, z: 0.25 } }));
        const { D, stage } = await openWb(w, mem, { id: "wb1" });
        await settleImg(w);
        const eng = D.engine, [flat, over, flat2] = imgs(eng);
        key(w, null, "v", "KeyV");
        eng.select(flat); frames(w, 1);
        const a = w.C.toScreen(eng.view, flat.x + flat.w, flat.y + flat.h);
        mouse(w, stage, "pointerdown", a.x, a.y); w.clock += 4; mouse(w, stage, "pointermove", a.x - 1000, a.y); mouse(w, stage, "pointerup", a.x - 1000, a.y); frames(w, 1);
        ok("크기 유효성: 아주 납작한 그림은 높이가 IMG_MIN 아래로 내려가지 않는 격자 배수 폭(2048)에서 멈춘다", flat.w === 2048 && near(flat.h, 0.01) && flat.h >= 0.01 && flat.x === 0, [flat.w, flat.h]);
        // 부동소수 경계: 9344×0.02 는 4672 폭이면 높이가 0.00999… 라 한 칸 더(4704)에서 멈춘다
        eng.select(flat2); frames(w, 1);
        const a2 = w.C.toScreen(eng.view, flat2.x + flat2.w, flat2.y + flat2.h);
        mouse(w, stage, "pointerdown", a2.x, a2.y); w.clock += 4; mouse(w, stage, "pointermove", a2.x - 3000, a2.y); mouse(w, stage, "pointerup", a2.x - 3000, a2.y); frames(w, 1);
        ok("크기 유효성: 높이 하한의 부동소수 경계(9344×0.02)에서도 멈춘 폭의 높이가 IMG_MIN 이상(4704)", flat2.w === 4704 && flat2.h >= 0.01, [flat2.w, flat2.h]);
        eng.view = { x: M - 1200, y: 0, z: 1 }; frames(w, 1);
        eng.select(over); frames(w, 1);
        const geo = () => [over.x, over.y, over.w, over.h].join(), g0 = geo(), u0 = eng.stats().undo;
        const tl = w.C.toScreen(eng.view, over.x, over.y);
        mouse(w, stage, "pointerdown", tl.x, tl.y); w.clock += 4; mouse(w, stage, "pointermove", tl.x + 40, tl.y + 40); mouse(w, stage, "pointerup", tl.x + 40, tl.y + 40); frames(w, 1);
        ok("크기 유효성: 범위를 넘어 걸친 옛 그림(x+w > 1e9)의 맞은편 귀가 범위 밖이면 크기 조절을 적용하지 않는다", geo() === g0 && eng.stats().undo === u0, geo());
        const body = w.C.toScreen(eng.view, over.x + 5, over.y + 50);
        mouse(w, stage, "pointerdown", body.x, body.y); w.clock += 4; mouse(w, stage, "pointermove", body.x - 3, body.y); mouse(w, stage, "pointerup", body.x - 3, body.y); frames(w, 1);
        ok("크기 유효성: 그 그림을 옮기면 범위 안으로 들어온다(x+w ≤ 1e9)", over.x + over.w <= M && over.w === 100 && eng.stats().undo === u0 + 1, geo());
        // 뗄 때 다시 본다 — 조작 중 자리가 (어떤 까닭으로든) 유효하지 않으면 제자리로(되돌리기 단계 없음)
        const g1 = geo(), u1 = eng.stats().undo, c = w.C.toScreen(eng.view, over.x + 50, over.y + 50);
        mouse(w, stage, "pointerdown", c.x, c.y); w.clock += 4; mouse(w, stage, "pointermove", c.x - 64, c.y);
        const ok1 = eng.mode && eng.mode.g && eng.mode.g.x !== over.x;
        eng.mode.g = { x: M - 50, y: over.y, w: over.w, h: over.h };
        mouse(w, stage, "pointerup", c.x - 64, c.y); frames(w, 1);
        ok("크기 유효성: 뗄 때 한 번 더 보아 유효하지 않으면 제자리로 둔다", ok1 && geo() === g1 && eng.stats().undo === u1, geo());
        ok("크기 유효성: 그 뒤에도 본문을 만들 수 있고 닫힌다", await w.W.close() === true);
      }
    }
    // bitmap 회수(리뷰 1회차 [2]) — 지금 판에도 되돌리기·다시 하기 더미에도 닿지 않는 그림의 bitmap 은 close 한다(늦게 끝난 디코드도).
    {
      const w = world(), { D } = await openWb(w, w.W.memoryStore()), eng = D.engine;
      const data = imgData(64, 64, false), p = { mime: "image/jpeg", data, bytes: w.C.b64Bytes(data), nat: 64 };
      const openN = () => w.bitmaps.filter(b => !b.closed).length, last = () => imgs(eng)[imgs(eng).length - 1];
      for (let i = 0; i < 205; i++) { eng.insertImage(p, 64, 64); await settle(); eng.undo(); }
      ok("bitmap 회수: 넣기 → 되돌리기 → 다른 그림 넣기 205번 뒤 열린 bitmap 은 닿는 그림 수(다시 하기 더미의 하나)와 같다",
        w.bitmaps.length === 205 && eng.images.size === 0 && eng.stats().undo === 0 && eng.stats().redo === 1 && openN() === 1, [w.bitmaps.length, eng.stats(), openN()]);
      eng.redo();
      const A = last();
      eng.select(A); eng.deleteSelected();
      eng.insertImage(p, 64, 64); await settle();
      const B = last();
      ok("bitmap 회수: 되돌리기 더미에서 닿는 지운 그림은 그대로 든다", A.bmp && !A.bmp.closed && B.bmp && !B.bmp.closed && openN() === 2, openN());
      eng.undo();
      eng.insertImage(p, 64, 64); await settle();
      const Cc = last();
      ok("bitmap 회수: 새 작업이 버린 다시 하기의 그림은 close, 되돌리기 더미의 지운 그림은 든다", B.bmp === null && A.bmp && !A.bmp.closed && Cc.bmp && openN() === 2, openN());
      const n0 = w.bitmaps.length;
      eng.insertImage(p, 64, 64);
      const X = last();
      eng.undo(); eng.insertImage(p, 64, 64); // X 는 디코드가 끝나기 전에 닿지 않게 됐다
      await settle();
      const fresh = w.bitmaps.slice(n0);
      ok("bitmap 회수: 늦게 끝난 디코드도 닿지 않는 그림이면 곧바로 close(든 수 = 지금 그림 + 더미에서 닿는 그림)", X.bmp === null && fresh.length === 2 && fresh[0].closed && !fresh[1].closed && openN() === 3 && eng.images.size === 2,
        [fresh.map(b => b.closed), openN(), eng.images.size]);
      // 늦게 끝난 디코드: 지웠지만 되돌리기 더미에서 닿는 그림은 든다(되돌리면 그대로 보인다)
      eng.insertImage(p, 64, 64);
      const Z = last();
      eng.select(Z); eng.deleteSelected(); // 디코드가 끝나기 전에 지웠다
      await settle();
      ok("bitmap 회수: 늦게 끝난 디코드라도 되돌리기 더미에서 닿는 그림이면 든다", Z.bmp && !Z.bmp.closed && !eng.images.has(Z.id));
      eng.undo();
      ok("bitmap 회수: 되돌리면 그 그림이 든 bitmap 으로 돌아온다", eng.images.has(Z.id) && Z.bmp && !Z.bmp.closed);
      await w.W.close();
      ok("bitmap 회수: 닫으면 모두 close", openN() === 0);
    }
    // bitmap 회수: 파일에서 읽은 그림(더미에 없다)은 지금 판에 있으므로 회수하지 않는다
    {
      const w = world(), mem = w.W.memoryStore();
      await mem.create("가"); await mem.save("wb1", imgBoard(w.C, [{ x: 0, y: 0, w: 320, h: 160, mime: "image/jpeg", data: imgData(200, 100) }], { name: "가" }));
      const { D } = await openWb(w, mem, { id: "wb1" });
      await settleImg(w);
      const eng = D.engine, L = imgs(eng)[0], data = imgData(64, 64, false), p = { mime: "image/jpeg", data, bytes: w.C.b64Bytes(data), nat: 64 };
      eng.insertImage(p, 64, 64); await settle(); eng.undo();
      eng.insertImage(p, 64, 64); await settle(); // 버린 다시 하기 — reap
      ok("bitmap 회수: 파일에서 읽은 지금 판의 그림은 그대로 든다(버린 다시 하기의 그림만 close)", L.bmp && !L.bmp.closed && w.bitmaps.filter(b => !b.closed).length === 2);
      await w.W.close();
    }
    // 큰 그림(리뷰 1회차 의견) — 파일에서 읽은 긴 변 2000 초과 그림은 디코드할 때 줄여 들고 큰 것은 close, 본문의 data·w·h 는 그대로. 미리보기도 같다.
    {
      const w = world(), ctl = {}, mem = w.W.memoryStore(), store = logged(mem, ctl);
      await mem.create("큰 그림"); await mem.create("다른 판");
      const big = imgData(3000, 1500, false);
      await mem.save("wb1", imgBoard(w.C, [{ x: 0, y: 0, w: 600, h: 300, mime: "image/jpeg", data: big }], { name: "큰 그림" }));
      await mem.save("wb2", imgBoard(w.C, [{ x: 0, y: 0, w: 600, h: 300, mime: "image/jpeg", data: big }], { name: "다른 판" }));
      const { root, stage, D } = await openWb(w, store, { id: "wb1" });
      await settleImg(w);
      const eng = D.engine, im = imgs(eng)[0], bigB = w.bitmaps.filter(b => b.width === 3000);
      ok("큰 그림: 3000×1500 그림은 2000×1000 bitmap 으로 줄여 들고 큰 bitmap 은 close", im.bmp && im.bmp.width === 2000 && im.bmp.height === 1000 && !im.bmp.closed
        && bigB.length === 1 && bigB[0].closed, w.bitmaps.map(b => b.width + "x" + b.height + (b.closed ? "c" : "")));
      stroke(w, stage, line(700, 400, 800, 450, 6)); advance(w, 2100); await settle();
      const sv = w.C.parse(ctl.lastSave);
      ok("큰 그림: 다시 저장한 본문의 data·w·h 는 그대로", sv.ok && sv.board.images[0].data === big && sv.board.images[0].w === 600 && sv.board.images[0].h === 300 && sv.board.strokes.length === 1);
      const nb = w.bitmaps.length;
      click(w, btn(root, "list")); await settleImg(w);
      const fresh = w.bitmaps.slice(nb);
      ok("큰 그림: 미리보기 디코드도 줄여 그리고 모두 close", fresh.some(b => b.width === 3000) && fresh.some(b => b.width === 2000 && b.height === 1000) && fresh.every(b => b.closed),
        fresh.map(b => b.width + "x" + b.height + (b.closed ? "c" : "")));
      key(w, null, "Escape", "Escape");
      await w.W.close();
    }
  }

  // ---- 13. 쓰는 중 자동 저장이 획을 끊지 않는다(조정 세션 추가 지시): 2초 타이머·save 성공 뒤 「곧바로 한 번 더」 는 그리기·지우기 중이면 미루고,
  // 뗀 뒤 저장한다. 숨김은 여전히 그 자리에서 확정한다.
  {
    const pen = (w, t, type, x, y) => ptr(w, t, type, { pointerId: 31, pointerType: "pen", clientX: x, clientY: y, pressure: type === "pointerup" ? 0 : 0.5 });
    const saves = ctl => ctl.log.filter(x => /^save/.test(x)).length;
    const last = (w, ctl) => w.C.parse(ctl.lastSave).board;
    // 앞 획의 2초가 긴 획 도중에 온다
    {
      const w = world(), ctl = {}, store = logged(w.W.memoryStore(), ctl);
      const { stage, D } = await openWb(w, store);
      const eng = D.engine;
      advance(w, 2100); await settle(); ctl.log = [];
      stroke(w, stage, line(100, 100, 200, 100, 4));
      advance(w, 1000);
      pen(w, stage, "pointerdown", 100, 300);
      for (let i = 1; i <= 10; i++) { w.clock += 4; pen(w, stage, "pointermove", 100 + i * 20, 300); }
      advance(w, 1500); await settle(); // 앞 획의 타이머(2초)가 이 획 도중에 온다
      ok("쓰는 중: 앞 획의 자동 저장이 와도 save 를 부르지 않고 획을 끊지 않는다", saves(ctl) === 0 && eng.mode && eng.mode.kind === "draw" && eng.strokes.size === 1, [saves(ctl), eng.mode && eng.mode.kind]);
      advance(w, 2100); await settle();
      ok("쓰는 중: 미룬 타이머가 다시 와도 쥐고 있는 동안은 미룬다", saves(ctl) === 0 && eng.mode && eng.mode.kind === "draw");
      for (let i = 11; i <= 20; i++) { w.clock += 4; pen(w, stage, "pointermove", 100 + i * 20, 300); }
      pen(w, stage, "pointerup", 500, 300); frames(w, 1);
      const s2 = lastStroke(eng);
      ok("쓰는 중: 뗀 뒤 한 획으로 남는다(점 21개)", eng.strokes.size === 2 && s2.pts.length / 3 === 21, s2 && s2.pts.length / 3);
      advance(w, 2100); await settle();
      ok("쓰는 중: 뗀 뒤 자동 저장이 두 획을 한 번에 쓴다", saves(ctl) === 1 && last(w, ctl).strokes.length === 2 && last(w, ctl).strokes[1].pts.length === 63, ctl.log);
      await w.W.close();
    }
    // 저장이 도는 중 긋기 시작한 획 — save 성공 뒤 「곧바로 한 번 더」 가 끊지 않는다
    {
      const w = world(), ctl = {}, store = logged(w.W.memoryStore(), ctl);
      const { stage, D } = await openWb(w, store);
      const eng = D.engine;
      advance(w, 2100); await settle(); ctl.log = [];
      stroke(w, stage, line(100, 100, 200, 100, 4));
      ctl.hold.save = true;
      advance(w, 2100); await settle();
      ok("저장 중: (준비) 첫 획의 save 가 돌고 있다", saves(ctl) === 1 && ctl.waiting.length === 1);
      stroke(w, stage, line(100, 200, 200, 200, 4)); // 저장 중 또 바뀜 — 자동 저장 대기(st.timer)
      pen(w, stage, "pointerdown", 100, 300);
      for (let i = 1; i <= 5; i++) { w.clock += 4; pen(w, stage, "pointermove", 100 + i * 20, 300); }
      ctl.hold.save = false; ctl.release(); await settle();
      ok("저장 중: save 성공 때 쓰는 중이면 곧바로 한 번 더 하지 않는다(획이 끊기지 않는다)", saves(ctl) === 1 && eng.mode && eng.mode.kind === "draw" && eng.strokes.size === 2, [ctl.log, eng.mode && eng.mode.kind]);
      for (let i = 6; i <= 10; i++) { w.clock += 4; pen(w, stage, "pointermove", 100 + i * 20, 300); }
      pen(w, stage, "pointerup", 300, 300); frames(w, 1);
      ok("저장 중: 뗀 뒤 한 획으로 남는다(점 11개)", eng.strokes.size === 3 && lastStroke(eng).pts.length / 3 === 11);
      advance(w, 2100); await settle();
      ok("저장 중: 뗀 뒤 자동 저장이 세 획을 쓴다", saves(ctl) === 2 && last(w, ctl).strokes.length === 3, ctl.log);
      await w.W.close();
    }
    // 지우는 중도 같다
    {
      const w = world(), ctl = {}, store = logged(w.W.memoryStore(), ctl);
      const { stage, D } = await openWb(w, store);
      const eng = D.engine;
      stroke(w, stage, line(100, 100, 300, 100, 4));
      stroke(w, stage, line(100, 200, 300, 200, 4));
      advance(w, 2100); await settle();
      stroke(w, stage, line(100, 400, 300, 400, 4)); // 자동 저장 대기
      ctl.log = [];
      key(w, null, "e", "KeyE");
      const er = (type, x, y) => ptr(w, stage, type, { pointerId: 32, pointerType: "pen", clientX: x, clientY: y, pressure: 0.5 });
      er("pointerdown", 200, 80); w.clock += 4; er("pointermove", 200, 110);
      advance(w, 2100); await settle();
      ok("지우는 중: 자동 저장이 와도 미루고 지우기를 끊지 않는다", saves(ctl) === 0 && eng.mode && eng.mode.kind === "erase" && eng.strokes.size === 2, [saves(ctl), eng.mode && eng.mode.kind]);
      w.clock += 4; er("pointermove", 200, 160); w.clock += 4; er("pointermove", 200, 210); er("pointerup", 200, 210); frames(w, 1);
      ok("지우는 중: 뗀 뒤 한 번의 지우기(되돌리기 한 단계에 두 획)", eng.strokes.size === 1 && eng.stats().undo === 4);
      key(w, null, "z", "KeyZ", { metaKey: true });
      ok("지우는 중: 되돌리면 두 획이 함께 돌아온다", eng.strokes.size === 3);
      key(w, null, "z", "KeyZ", { metaKey: true, shiftKey: true });
      advance(w, 2100); await settle();
      ok("지우는 중: 뗀 뒤 저장한다", saves(ctl) === 1 && last(w, ctl).strokes.length === 1, ctl.log);
      await w.W.close();
    }
    // 이동(손가락)은 쥐고 있어도 미루지 않는다 — 판을 바꾸지 않으니 끊을 획이 없다
    {
      const w = world(), ctl = {}, store = logged(w.W.memoryStore(), ctl);
      const { stage, D } = await openWb(w, store);
      advance(w, 2100); await settle(); ctl.log = [];
      stroke(w, stage, line(100, 100, 200, 100, 4));
      const touch = (type, x) => ptr(w, stage, type, { pointerId: 33, pointerType: "touch", clientX: x, clientY: 400, pressure: 0 });
      touch("pointerdown", 400); w.clock += 4; touch("pointermove", 450);
      advance(w, 2100); await settle();
      ok("이동 중: 자동 저장을 미루지 않는다(이동은 그대로 잇는다)", saves(ctl) === 1 && D.engine.mode && D.engine.mode.kind === "pan", [ctl.log, D.engine.mode && D.engine.mode.kind]);
      touch("pointerup", 450);
      await w.W.close();
    }
    // 숨김은 여전히 그 자리에서 확정한다
    {
      const w = world(), ctl = {}, store = logged(w.W.memoryStore(), ctl);
      const { stage, D } = await openWb(w, store);
      const eng = D.engine;
      advance(w, 2100); await settle(); ctl.log = [];
      pen(w, stage, "pointerdown", 100, 300);
      for (let i = 1; i <= 5; i++) { w.clock += 4; pen(w, stage, "pointermove", 100 + i * 20, 300); }
      w.doc.visibilityState = "hidden"; dispatch(w, w.doc, "visibilitychange", {}); await settle();
      ok("숨김: 쓰는 중이어도 미루지 않고 그 자리에서 획을 확정해 저장한다", !eng.mode && eng.strokes.size === 1 && saves(ctl) === 1 && last(w, ctl).strokes.length === 1, [eng.mode, ctl.log]);
      w.doc.visibilityState = "visible";
      await w.W.close();
    }
  }

  // ---- 14. 미저장 판 조회 ItdaWhiteboard.unsaved(store)(itda-hyve#699 ④-1b): 기록기의 dirty 그대로 — 닫힌 표면·도는 save·늦은 성공·삭제·
  // 두 store·쓴 적 없는 store·잘못된 인자·조회가 상태를 바꾸지 않음·저장 실패 뒤 다시 열기의 세 실패 길을 실제 표면으로 본다.
  {
    const ids = (w, s) => w.W.unsaved(s).slice().sort().join();
    const snap = r => r ? JSON.stringify([r.want && r.want.n, r.running && r.running.n, r.done, r.halted, r.blocked, r.editors, r.gone, r.waiters.size]) : "없음";
    // 저장이 거부된 채 두 번 닫으면 닫힌 store 에서도 그 판이 나온다 — 조회는 store 를 부르지 않고 기록기를 바꾸지 않는다
    {
      const w = world(), ctl = {}, store = logged(w.W.memoryStore(), ctl);
      const { stage, D } = await openWb(w, store);
      const id = D.state.id;
      stroke(w, stage, line(100, 100, 200, 100, 4));
      ctl.fail.save = true;
      const c1 = await w.W.close(), c2 = await w.W.close();
      await settle();
      ok("unsaved: 저장 거부 뒤 닫힌 store 에서도 그 판이 나온다", c1 === false && c2 === true && !w.W.isOpen() && ids(w, store) === id, [c1, c2, w.W.unsaved(store)]);
      const before = snap(D.rec(id)), n = ctl.log.length, size = D.recs.size;
      const a = w.W.unsaved(store), b = w.W.unsaved(store);
      ok("unsaved: 조회 전후 그 판 기록기의 번호 필드·store 호출 수·기록기 수가 그대로이고 매번 새 배열이다", snap(D.rec(id)) === before && ctl.log.length === n && D.recs.size === size && a !== b && Array.isArray(a), [before, snap(D.rec(id)), ctl.log.slice(n)]);
      a.push("x");
      ok("unsaved: 돌려준 배열을 고쳐도 다음 조회에 번지지 않는다", ids(w, store) === id);
      ok("unsaved: 쓴 적 없는 store 는 빈 배열", w.W.unsaved(w.W.memoryStore()).length === 0 && w.W.unsaved({}).length === 0 && w.W.unsaved(() => {}).length === 0);
      const bad = [undefined, null, "wb1", 1, true].map(v => { try { w.W.unsaved(v); return "통과"; } catch (e) { return e && e.name === "TypeError" && e.constructor && e.constructor.name === "TypeError" ? "TypeError" : String(e); } });
      ok("unsaved: store 가 객체가 아니면 TypeError(빈 배열로 뭉개지 않는다)", bad.every(x => x === "TypeError"), bad);
    }
    // 도는 save 만 있어도 미저장, 그 뒤 더 새 want 가 있으면 옛 번호의 성공이 지우지 않는다, 늦은 성공 뒤에는 빠진다
    {
      const w = world(), ctl = {}, store = logged(w.W.memoryStore(), ctl);
      const { stage, D } = await openWb(w, store);
      const id = D.state.id;
      advance(w, 2100); await settle();
      ok("unsaved: (준비) 첫 판 저장 뒤 비어 있다", w.W.unsaved(store).length === 0, w.W.unsaved(store));
      ctl.hold.save = true;
      stroke(w, stage, line(100, 100, 200, 100, 4));
      advance(w, 2100); await settle();
      const r = D.rec(id);
      ok("unsaved: running 만 있어도 미저장", r && r.running && !r.want && ids(w, store) === id, snap(r));
      stroke(w, stage, line(100, 200, 200, 200, 4));
      D.save();
      ok("unsaved: running 뒤 더 새 want 가 있어도 미저장", r.running && r.want && r.want.n > r.running.n && ids(w, store) === id, snap(r));
      const newer = r.want.n;
      ctl.fail.save = true; ctl.hold.save = false;
      ctl.release(); await settle();
      ok("unsaved: 오래된 번호의 성공이 최신 want 를 지우지 않는다", r.done < newer && ids(w, store) === id, snap(r));
      ctl.fail.save = false; ctl.hold.save = true;
      const closing = w.W.close(); await settle();
      advance(w, 8100); await settle();
      const closed = await closing;
      ok("unsaved: (준비) 기한을 넘겨 닫혔고 save 가 돈다", !w.W.isOpen() && r.running && ids(w, store) === id, [closed, snap(r)]);
      ctl.hold.save = false; ctl.release(); await settle();
      ok("unsaved: 늦은 성공 뒤에는 빠진다(표면이 닫혀 있어도)", r.done >= newer && w.W.unsaved(store).length === 0, snap(r));
    }
    // 삭제(trash)가 된 판은 빠진다 · 두 store 가 같은 판 id 를 써도 섞이지 않는다
    {
      const w = world(), mem = w.W.memoryStore();
      await mem.create("가"); await mem.save("wb1", boardText(w.C, 1, { name: "가" }));
      await mem.create("나"); await mem.save("wb2", boardText(w.C, 2, { name: "나" }));
      const ctl = {}, store = logged(mem, ctl);
      let o = await openWb(w, store, { id: "wb1" });
      stroke(w, o.stage, line(100, 100, 200, 100, 4));
      ctl.fail.save = true;
      await w.W.close(); await w.W.close(); await settle();
      ok("unsaved: (준비) wb1 이 미저장으로 닫혔다", ids(w, store) === "wb1");
      // 같은 id 를 쓰는 다른 store
      const mem2 = w.W.memoryStore();
      await mem2.create("다"); await mem2.save("wb1", boardText(w.C, 1));
      const ctl2 = {}, store2 = logged(mem2, ctl2);
      const o2 = await openWb(w, store2, { id: "wb1" });
      ok("unsaved: 두 store 가 같은 판 id — 깨끗한 store2 는 비고 store 는 그대로", w.W.unsaved(store2).length === 0 && ids(w, store) === "wb1" && o2.D.state.id === "wb1", [w.W.unsaved(store2), w.W.unsaved(store)]);
      stroke(w, o2.stage, line(100, 100, 200, 100, 4));
      ctl2.fail.save = true;
      await w.W.close(); await w.W.close(); await settle();
      ok("unsaved: 두 store 가 같은 판 id — 둘 다 미저장이면 각자 나온다", ids(w, store2) === "wb1" && ids(w, store) === "wb1");
      // store 로 wb2 를 열어 목록에서 wb1 을 삭제
      ctl.fail.save = false;
      o = await openWb(w, store, { id: "wb2" });
      click(w, btn(o.root, "list")); await settle();
      const del = () => o.root.querySelectorAll(".itda-wb-item").find(x => x.dataset.id === "wb1").querySelectorAll("button").find(b => b.dataset.act === "trash");
      click(w, del()); click(w, del()); await settle();
      ok("unsaved: trash 가 된 판(gone)은 빠진다 — 다른 store 의 같은 id 는 그대로", ctl.log.includes("trash:wb1") && !ids(w, store).split(",").includes("wb1") && ids(w, store2) === "wb1", [ctl.log, w.W.unsaved(store)]);
      await w.W.close();
    }
    // 저장 실패 뒤 같은 store 를 다시 열다 list 가 시간을 넘기거나·곧바로 거부되거나·여는 중 닫혀도 이전 판이 계속 나온다
    for (const mode of ["시간 초과", "즉시 거부", "여는 중 닫기"]) {
      const w = world(), ctl = {}, store = logged(w.W.memoryStore(), ctl);
      const { stage, D } = await openWb(w, store);
      const id = D.state.id;
      stroke(w, stage, line(100, 100, 200, 100, 4));
      ctl.fail.save = true;
      await w.W.close(); await w.W.close(); await settle();
      const before = ids(w, store);
      if (mode === "즉시 거부") ctl.fail.list = true; else ctl.hold.list = true;
      w.win.ItdaWhiteboardHost = { store };
      const opened = w.W.open().then(() => "열림", e => "거부"); await settle();
      if (mode === "시간 초과") { advance(w, 8100); await settle(); }
      else if (mode === "즉시 거부") { frames(w, 2); await w.W.close(); await w.W.close(); }
      else { await w.W.close(); await w.W.close(); }
      advance(w, 8100); await settle();
      ctl.hold.list = false; ctl.release(); await settle();
      const res = await opened;
      ok("unsaved: 저장 실패 뒤 다시 열기 — " + mode + " 에도 이전 판이 계속 나온다", before === id && ids(w, store) === id && !w.W.isOpen(), [before, res, w.W.unsaved(store), ctl.log]);
    }
  }

  // ---- 14. 중계 출구·보기 전용 화면(itda-hyve#699 ④-3a): 묶음 형식(sha-256·검증·반올림), 출구(off·reset·증분·긋는 중 안 보냄·보기 합치기·
  // send 차례·full·실패 뒤 다음 변화에 reset·image 먼저·같은 해시 한 번·큰 판 조각 reset·긴 획 조각·닫힘·판 전환·이름·저장과 무관),
  // 왕복(보내는 쪽 board() 와 뷰어의 판이 같다 — 획 차례·점·그림 자리·배경), 뷰어(검증·resync·상한·이름 textContent·따라가기·함께 열지 않음).
  // 보내는 쪽과 뷰어는 서로 다른 world 다(한 페이지에 함께 뜨지 않는다 — whiteboard.js 머리 주석).
  {
    const nodeCrypto = require("crypto");
    const sha = b => nodeCrypto.createHash("sha256").update(b).digest("hex");
    const imgFile = (wd, ht, alpha) => ({ type: alpha ? "image/png" : "image/jpeg", _img: { w: wd, h: ht, alpha: !!alpha } });
    const paste = (w, list) => dispatch(w, focused(w), "paste", { clipboardData: { items: list.map(f => ({ kind: "file", type: f.type, getAsFile: () => f })), files: [] } });
    const mouse = (w, t, type, x, y) => ptr(w, t, type, { pointerId: 1, pointerType: "mouse", clientX: x, clientY: y });
    const drag = (w, t, from, to) => { mouse(w, t, "pointerdown", from[0], from[1]); w.clock += 4; mouse(w, t, "pointermove", (from[0] + to[0]) / 2, (from[1] + to[1]) / 2); w.clock += 4; mouse(w, t, "pointermove", to[0], to[1]); mouse(w, t, "pointerup", to[0], to[1]); frames(w, 1); };
    const undoKey = w => key(w, null, "z", "KeyZ", { metaKey: true }), redoKey = w => key(w, null, "z", "KeyZ", { metaKey: true, shiftKey: true });
    // pumpR 는 출구의 타이머(100ms)와 약속을 번갈아 돌려 낼 묶음이 다 나가게 한다
    const pumpR = async (w, n) => { for (let i = 0; i < (n || 8); i++) { advance(w, 120); await settle(4); } };
    const kinds = list => list.map(t => JSON.parse(t).k);
    const opsOf = list => list.flatMap(t => JSON.parse(t).ops);
    // hub 는 출구와 뷰어 사이다 — got 보낸 묶음 · imgs 넘긴 그림(해시 → 바이트) · log image·send 차례. o.send 가 있으면 그것이 send 의 값을 정한다(뷰어에 넘기지 않는다).
    function hub(w, o) {
      o = o || {};
      const H = { got: [], imgs: new Map(), log: [], applied: [], viewer: null, busy: 0, maxBusy: 0 };
      H.h = w.W.relay({
        send: t => {
          H.got.push(t); H.log.push("send:" + JSON.parse(t).k);
          if (o.send) return o.send(t, H);
          if (H.viewer) H.applied.push(H.viewer.apply(t));
          return "";
        },
        image: (hash, bytes, mime) => { H.log.push("image:" + hash); H.imgs.set(hash, { bytes: Buffer.from(bytes), mime }); return o.image ? o.image(hash) : undefined; },
      });
      return H;
    }
    // viewerOf 는 다른 world(vw)에 뷰어를 연다 — 그림은 hub 가 받은 바이트를 Blob 으로 준다
    function viewerOf(vw, H, o) {
      const m = vw.doc.createElement("div");
      vw.body.appendChild(m);
      const v = vw.W.viewer(Object.assign({ mount: m, image: hash => { const g = H && H.imgs.get(hash); return g ? new Blob([g.bytes], { type: g.mime }) : null; } }, o));
      if (H) H.viewer = v;
      return v;
    }
    // same 은 보내는 쪽 판(파일로 쓴 꼴 — 반올림)과 뷰어의 판이 같은가다: 획(차례·도구·색·굵기·점), 그림(차례·자리·mime·바이트 sha-256), 배경
    function same(w, D, V) {
      const C = w.C, exp = C.parse(C.serialize(Object.assign(D.engine.board(), { name: "" }))).board, got = V.debug().board();
      const es = JSON.stringify(exp.strokes.map(s => [s.tool, s.color, s.w, s.pts])), gs = JSON.stringify(got.strokes.map(s => [s.tool, s.color, s.w, s.pts]));
      const ei = JSON.stringify(exp.images.map(i => [i.x, i.y, i.w, i.h, i.mime, sha(Buffer.from(i.data, "base64"))]));
      const gi = JSON.stringify(got.images.map(i => [i.x, i.y, i.w, i.h, i.mime, i.hash]));
      return { ok: es === gs && ei === gi && exp.bg === got.bg, n: [exp.strokes.length, got.strokes.length, exp.images.length, got.images.length], bg: [exp.bg, got.bg], es: es.slice(0, 200), gs: gs.slice(0, 200), ei, gi };
    }
    const sameOk = (name, w, D, V) => { const r = same(w, D, V); ok(name, r.ok, r); return r.ok; };

    // 형식 — sha-256 은 node crypto 와 같다(길이 경계 포함)
    {
      const { C } = world();
      const good = [0, 1, 3, 55, 56, 57, 63, 64, 65, 119, 120, 1000, 70000].every(n => { const b = nodeCrypto.randomBytes(n); return C.sha256Hex(new Uint8Array(b)) === sha(b); });
      ok("중계 형식: sha256Hex 가 node crypto 와 같다(빈 것·블록 경계·큰 것)", good && C.sha256Hex(new Uint8Array(0)) === "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855");
      const s = { tool: "pen", color: 2, w: 3.14159, pts: [1.234567, -2.345678, 0.55555, 10.005, 20.004, 1] };
      const im = { x: 1.234, y: -5, w: 320, h: 160.005, mime: "image/png", data: "AAAA" };
      const file = JSON.parse(C.serialize({ name: "", bg: "dark", view: { x: 0, y: 0, z: 1 }, strokes: [s], images: [im] }));
      const ws = C.wireStroke(7, s), wi = C.wireImage(3, im, "a".repeat(64));
      ok("중계 형식: 획·그림의 수 반올림이 파일 형식과 같다", JSON.stringify(ws.slice(2)) === JSON.stringify([file.strokes[0].t, file.strokes[0].c, file.strokes[0].w, file.strokes[0].p])
        && JSON.stringify(wi.slice(2, 6)) === JSON.stringify([file.images[0].x, file.images[0].y, file.images[0].w, file.images[0].h]), [ws, wi, file]);
      const text = C.batchText("reset", [C.wireHead("판", "grid"), ws, ["a", 7, [1, 2, 0.5]], ["d", [1, 2]], wi, C.wireGeom(3, im), ["x", [3]], C.wireView({ x: 1, y: 2, z: 1.5 }, 800, 600), ["zz", 1]].map(o => JSON.stringify(o)));
      const p = C.parseBatch(text);
      ok("중계 형식: batchText 는 JSON.stringify({v,k,ops}) 와 같은 글자, parseBatch 는 모든 연산을 읽고 모르는 연산은 건너뛴다",
        text === JSON.stringify({ v: 1, k: "reset", ops: JSON.parse(text).ops }) && p.ok && p.k === "reset" && p.ops.map(o => o.op).join() === "h,s,a,d,i,g,x,v" && p.ops[1].pts[5] === 1, p);
      const bad = (name, ops, k, v) => { const r = C.parseBatch(JSON.stringify({ v: v == null ? 1 : v, k: k == null ? "" : k, ops })); ok("중계 검증: " + name, !r.ok, r); };
      bad("ops 가 배열이 아니면 거부", null);
      ok("중계 검증: JSON 이 아니거나 문자열이 아니면 거부", !C.parseBatch("{").ok && !C.parseBatch(5).ok && !C.parseBatch("x".repeat(C.BATCH_READ + 1)).ok);
      bad("v 가 1 이 아니면 거부", [], "", 2);
      bad("k 를 모르면 거부", [], "clear");
      bad("off 에 연산이 있으면 거부", [["d", [1]]], "off");
      bad("획 색 번호가 범위 밖이면 거부", [["s", 1, "pen", 8, 4, [0, 0, 0.5]]]);
      bad("획 굵기가 범위 밖이면 거부", [["s", 1, "pen", 0, 0, [0, 0, 0.5]]]);
      bad("획 점 좌표가 범위 밖이면 거부", [["s", 1, "pen", 0, 4, [2e9, 0, 0.5]]]);
      bad("획 점이 3 의 배수가 아니면 거부", [["s", 1, "pen", 0, 4, [0, 0]]]);
      bad("id 가 0·소수·문자열이면 거부", [["s", 0, "pen", 0, 4, [0, 0, 0.5]]]);
      bad("잇기 점이 유한하지 않으면 거부", [["a", 1, [0, "1", 0.5]]]);
      bad("빼기 목록이 정수 배열이 아니면 거부", [["d", [1.5]]]);
      bad("그림 해시가 sha-256 hex 가 아니면 거부", [["i", 1, 0, 0, 10, 10, "image/png", "ABC"]]);
      bad("그림 mime 이 png·jpeg 가 아니면 거부", [["i", 1, 0, 0, 10, 10, "image/svg+xml", "a".repeat(64)]]);
      bad("그림 w 가 0 이면 거부", [["g", 1, 0, 0, 0, 10]]);
      bad("보기 z 가 0 이면 거부", [["v", 0, 0, 0, 800, 600]]);
      bad("이름이 200 글자를 넘으면 거부", [["h", "가".repeat(201), "dark"]]);
      bad("연산이 배열이 아니면 거부", [{ op: "s" }]);
    }

    // 출구 — 닫힌 채 걸면 off, 열면 reset(머리·보기), 긋는 중은 보내지 않고 뗀 뒤 확정된 획만, 보기는 합친다
    {
      const w = world(), store = w.W.memoryStore();
      const H = hub(w);
      await pumpR(w);
      ok("출구: 표면이 닫힌 채 걸면 off 하나", kinds(H.got).join() === "off" && JSON.parse(H.got[0]).ops.length === 0, H.got);
      const vw = world(), states = [], V = viewerOf(vw, H, { onState: s => states.push(s) });
      const { root, stage, D } = await openWb(w, store);
      await pumpR(w);
      const first = JSON.parse(H.got[1]);
      ok("출구: 판이 보이면 reset — 머리(이름·배경)가 먼저, 보기(x·y·z·화면 크기)가 끝", H.got.length === 2 && first.k === "reset" && JSON.stringify(first.ops[0]) === JSON.stringify(["h", "화이트보드 1", "dark"])
        && JSON.stringify(first.ops[first.ops.length - 1]) === JSON.stringify(["v", 0, 0, 1, 1280, 720]), H.got);
      ok("뷰어: reset 을 받으면 켜지고 판 이름표를 보인다(onState)", V.debug().on() && states.length === 1 && states[0].on && states[0].name === "화이트보드 1"
        && !V.debug().root.querySelector(".itda-wb-title").hidden && V.debug().root.querySelector(".itda-wb-title").textContent === "화이트보드 1", states);
      // 긋는 중에는 아무것도 나가지 않는다
      const a = { pointerId: 7, pointerType: "pen" };
      ptr(w, stage, "pointerdown", { ...a, clientX: 100, clientY: 300, pressure: 0.5 });
      for (let i = 1; i <= 6; i++) { w.clock += 4; ptr(w, stage, "pointermove", { ...a, clientX: 100 + i * 20, clientY: 300 + i * 3, pressure: 0.5 }); }
      const n0 = H.got.length;
      await pumpR(w);
      ok("출구: 긋는 중의 점은 보내지 않는다", H.got.length === n0, H.got.slice(n0));
      ptr(w, stage, "pointerup", { ...a, clientX: 220, clientY: 318, pressure: 0 }); frames(w, 1);
      await pumpR(w);
      const inc = H.got.slice(n0).map(t => JSON.parse(t));
      ok("출구: 손을 떼면 그 획 하나가 증분 묶음의 s 로 나간다", inc.length === 1 && inc[0].k === "" && inc[0].ops.length === 1 && inc[0].ops[0][0] === "s" && inc[0].ops[0][1] === lastStroke(D.engine).id, inc);
      sameOk("왕복: 획 하나", w, D, V);
      // 보기는 합친다 — 잦은 이동은 마지막 값 하나
      const n1 = H.got.length;
      for (let i = 0; i < 6; i++) { dispatch(w, stage, "wheel", { deltaX: 0, deltaY: 10, deltaMode: 0, clientX: 600, clientY: 300 }); w.clock += 5; }
      await pumpR(w);
      const vb = H.got.slice(n1).map(t => JSON.parse(t)), ev = D.engine.view;
      ok("출구: 잦은 보기 이동은 묶음 하나·보기 연산 하나(마지막 값)", vb.length === 1 && vb[0].ops.length === 1 && JSON.stringify(vb[0].ops[0]) === JSON.stringify(w.C.wireView(ev, 1280, 720)), vb);
      ok("뷰어: 처음엔 따라간다 — 화면 크기가 같으면 강사의 보기와 같다", V.following() && near(V.debug().engine.view.y, ev.y, 0.01) && near(V.debug().engine.view.z, ev.z, 1e-6), [V.debug().engine.view, ev]);
      // 획 여러 개·지우개·되돌리기·다시 하기 — 겹침 차례
      stroke(w, stage, line(100, 400, 300, 400, 6));
      stroke(w, stage, line(100, 450, 300, 450, 6));
      stroke(w, stage, line(100, 500, 300, 500, 6));
      await pumpR(w);
      const ids0 = V.debug().board().strokes.length;
      sameOk("왕복: 획 여러 개", w, D, V);
      key(w, null, "e", "KeyE");
      stroke(w, stage, line(200, 430, 200, 470, 4)); // 가운데 획(450)만 지운다
      key(w, null, "p", "KeyP");
      await pumpR(w);
      ok("왕복: 지우개 — 빼기(d)로 나가고 뷰어에서도 빠진다", D.engine.strokes.size === ids0 - 1 && opsOf(H.got.slice(-1)).some(o => o[0] === "d") && same(w, D, V).ok, same(w, D, V));
      stroke(w, stage, line(100, 550, 300, 550, 6));
      await pumpR(w);
      undoKey(w); await pumpR(w); // 마지막 획 되돌리기
      undoKey(w); await pumpR(w); // 지우기 되돌리기 — 가운데 획이 원래 차례로
      const order = V.debug().board().strokes.map(s => s.pts[1]), dy = Math.round(D.engine.view.y); // 화면 y + 보기 y = 판 y
      ok("왕복: 되돌리기로 돌아온 지운 획이 원래 겹침 차례(id 차례)에 든다", same(w, D, V).ok && JSON.stringify(order.slice(-3).map(Math.round)) === JSON.stringify([400 + dy, 450 + dy, 500 + dy]), [order, dy, same(w, D, V)]);
      redoKey(w); await pumpR(w);
      sameOk("왕복: 다시 하기(지우기)", w, D, V);
      redoKey(w); await pumpR(w);
      sameOk("왕복: 다시 하기(획)", w, D, V);
      undoKey(w); undoKey(w); redoKey(w); await pumpR(w); // 한 묶음 안에 섞인 되돌리기·다시 하기
      sameOk("왕복: 한 묶음 안에 섞인 되돌리기·다시 하기", w, D, V);
      // 배경
      click(w, btn(root, "bg", { bg: "light" }));
      await pumpR(w);
      ok("왕복: 배경 — 머리 연산(h)으로 나간다", opsOf(H.got.slice(-1)).some(o => o[0] === "h" && o[2] === "light") && same(w, D, V).ok && V.debug().engine.bg() === "light", same(w, D, V));
      // 그림 — 넣기·옮기기·크기·지우기·되돌리기
      paste(w, [imgFile(1000, 500, false)]);
      await settle(30); frames(w, 2);
      const nl = H.log.length;
      await pumpR(w);
      const im = [...D.engine.images.values()][0], hash = sha(Buffer.from(im.data, "base64"));
      const seq = H.log.slice(nl);
      ok("출구: 그림 바이트(image)는 그 그림을 가리키는 묶음보다 먼저 한 번 넘긴다", seq[0] === "image:" + hash && seq[1] === "send:" && H.imgs.get(hash).bytes.equals(Buffer.from(im.data, "base64")) && H.imgs.get(hash).mime === "image/jpeg", seq);
      const iop = opsOf(H.got.slice(-1)).find(o => o[0] === "i");
      ok("출구: 그림 연산은 자리·mime·해시만 싣는다(바이트 없음)", iop && iop.length === 8 && iop[7] === hash && !H.got[H.got.length - 1].includes(im.data.slice(0, 40)), iop);
      await settle(20);
      const vim = [...V.debug().engine.images.values()][0];
      ok("왕복: 그림 넣기 — 뷰어는 호스트의 바이트로 디코드한다", same(w, D, V).ok && vim && vim.state === "ok" && vim.bmp, [same(w, D, V), vim && vim.state]);
      drag(w, stage, [640, 380], [704, 412]); // 몸통 끌기 = 옮기기
      await pumpR(w);
      ok("왕복: 그림 옮기기 — g 연산", opsOf(H.got.slice(-1)).some(o => o[0] === "g") && same(w, D, V).ok, same(w, D, V));
      const g = [...D.engine.images.values()][0], corner = w.C.toScreen(D.engine.view, g.x + g.w, g.y + g.h);
      drag(w, stage, [corner.x, corner.y], [corner.x + 64, corner.y + 32]); // 오른쪽 아래 귀 = 크기
      await pumpR(w);
      ok("왕복: 그림 크기", g.w > 480 && same(w, D, V).ok, [g.w, same(w, D, V)]);
      key(w, null, "Delete", "Delete");
      await pumpR(w);
      ok("왕복: 그림 지우기 — x 연산", D.engine.images.size === 0 && opsOf(H.got.slice(-1)).some(o => o[0] === "x") && same(w, D, V).ok, same(w, D, V));
      undoKey(w); await pumpR(w);
      sameOk("왕복: 그림 지우기 되돌리기", w, D, V);
      undoKey(w); await pumpR(w);
      sameOk("왕복: 그림 크기 되돌리기", w, D, V);
      const imgCalls = H.log.filter(x => x.startsWith("image:")).length;
      paste(w, [imgFile(1000, 500, false)]);
      await settle(30); frames(w, 2);
      await pumpR(w); await settle(20);
      const both = [...D.engine.images.values()];
      ok("출구: 같은 해시의 그림은 바이트를 다시 넘기지 않는다", both.length === 2 && both[0].data === both[1].data && H.log.filter(x => x.startsWith("image:")).length === imgCalls && same(w, D, V).ok, H.log.slice(-4));
      ok("뷰어: 받은 묶음은 모두 적용됐다(true)", H.applied.every(x => x === true), H.applied);
      // 이름 바꾸기(지금 판) — 머리 연산
      click(w, btn(root, "list")); await settle();
      const row = root.querySelectorAll(".itda-wb-item").find(r => r.dataset.id === D.state.id);
      click(w, row.querySelectorAll("button").find(b => b.dataset.act === "rename"));
      const inp = row.querySelector(".itda-wb-rename") || root.querySelector(".itda-wb-rename");
      inp.value = "수업 판";
      key(w, inp, "Enter", "Enter"); await settle(12);
      await pumpR(w);
      ok("출구: 이름 바꾸기는 머리 연산 — 뷰어의 이름표가 바뀐다", opsOf(H.got.slice(-2)).some(o => o[0] === "h" && o[1] === "수업 판") && V.debug().name() === "수업 판", opsOf(H.got.slice(-2)));
      dispatch(w, focused(w), "keydown", { key: "Escape", code: "Escape" }); // 목록 닫기
      // 판 전환 — 새 판·되돌아가기
      const firstId = D.state.id, beforeNew = H.got.length;
      click(w, btn(root, "list")); await settle();
      click(w, btn(root, "new")); await settle(20); frames(w, 2);
      await pumpR(w);
      const afterNew = H.got.slice(beforeNew).map(t => JSON.parse(t));
      ok("출구: 새 판을 열면 reset(빈 판·새 이름)", D.state.id !== firstId && afterNew.some(b => b.k === "reset" && b.ops[0][0] === "h" && b.ops[0][1] === D.state.name) && same(w, D, V).ok && V.debug().board().strokes.length === 0,
        [afterNew.map(b => b.k), D.state.name, same(w, D, V)]);
      stroke(w, stage, line(400, 200, 500, 260, 5));
      await pumpR(w);
      sameOk("왕복: 새 판에 그은 획", w, D, V);
      click(w, btn(root, "listclose"));
      key(w, null, "[", "BracketLeft"); await settle(20); frames(w, 2);
      await pumpR(w);
      ok("출구: 판을 바꾸면 reset — 뷰어는 바꾼 판과 같다", D.state.id === firstId && kinds(H.got.slice(-2)).includes("reset") && same(w, D, V).ok, same(w, D, V));
      // 중간에 붙는 뷰어 — 호스트가 reset() 을 부른다
      const vw2 = world(), V2 = viewerOf(vw2, null);
      H.viewer = V2;
      H.h.reset();
      await pumpR(w); await settle(20);
      ok("출구: reset() — 중간에 붙은 뷰어도 판 전체를 받는다", kinds(H.got.slice(-1))[0] === "reset" && same(w, D, V2).ok, same(w, D, V2));
      // 저장은 그대로
      advance(w, 2100); await settle();
      ok("출구: 걸려 있어도 저장 기록기는 그대로(미저장 없음)", w.W.unsaved(store).length === 0, w.W.unsaved(store));
      // 닫기 — off
      await w.W.close(); await settle();
      await pumpR(w);
      ok("출구: 표면이 닫히면 off — 뷰어는 꺼지고 판을 비운다", kinds(H.got.slice(-1))[0] === "off" && !V2.debug().on() && V2.debug().board().strokes.length === 0
        && V2.debug().root.classList.contains("off"), kinds(H.got.slice(-2)));
      const nOff = H.got.length;
      await pumpR(w);
      ok("출구: off 는 한 번만", H.got.length === nOff);
      H.viewer = V;
      await openWb(w, store); await pumpR(w); await settle(20);
      const D2 = w.W.debug();
      ok("출구: 다시 열면 reset", kinds(H.got.slice(nOff))[0] === "reset" && same(w, D2, V).ok, same(w, D2, V));
      // stop — 더 내지 않는다
      H.h.stop();
      const nStop = H.got.length;
      stroke(w, surfaceOf(w).querySelector(".itda-wb-stage"), line(100, 100, 300, 120, 4));
      await pumpR(w);
      ok("출구: stop() 뒤에는 보내지 않는다", H.got.length === nStop);
      await w.W.close(); await settle();
    }

    // 출구 — send 차례(앞 호출이 끝나야 다음, 기다리는 동안의 연산은 합친다)·full·실패·던짐·image 실패. 그리기·저장은 막히지 않는다
    {
      const w = world(), ctl = {}, store = logged(w.W.memoryStore(), ctl);
      const { stage, D } = await openWb(w, store);
      const vw = world();
      let mode = "", held = [], busy = 0, maxBusy = 0;
      const H = hub(w, { send: (t, H) => {
        if (mode === "full") { mode = ""; return "full"; }
        if (mode === "reject") return Promise.reject(new Error("끊김"));
        if (mode === "throw") throw new Error("끊김");
        if (H.viewer) H.applied.push(H.viewer.apply(t));
        if (mode === "hold") { busy++; maxBusy = Math.max(maxBusy, busy); return new Promise(r => held.push(() => { busy--; r(""); })); }
        return "";
      } });
      const V = viewerOf(vw, H);
      await pumpR(w);
      mode = "hold";
      stroke(w, stage, line(100, 100, 200, 100, 4));
      await pumpR(w);
      const n0 = H.got.length;
      stroke(w, stage, line(100, 150, 200, 150, 4));
      stroke(w, stage, line(100, 200, 200, 200, 4));
      stroke(w, stage, line(100, 250, 200, 250, 4));
      await pumpR(w);
      ok("출구: 앞 send 가 끝나기 전에는 다음을 부르지 않는다", H.got.length === n0 && maxBusy === 1, [H.got.length, n0, maxBusy]);
      mode = ""; held.splice(0).forEach(f => f());
      await pumpR(w);
      const merged = H.got.slice(n0).map(t => JSON.parse(t));
      ok("출구: 기다리는 동안의 연산은 다음 묶음 하나로 합친다", merged.length === 1 && merged[0].ops.filter(o => o[0] === "s").length === 3 && same(w, D, V).ok, merged.map(b => b.ops.map(o => o[0])));
      // full — 다음에 reset
      mode = "full";
      stroke(w, stage, line(100, 300, 200, 300, 4));
      await pumpR(w);
      ok("출구: send 가 full 이면 다음 묶음은 reset", kinds(H.got.slice(-2)).join() === ",reset" && same(w, D, V).ok, [kinds(H.got.slice(-3)), same(w, D, V)]);
      // 거부 — 삼키고, 조용하면 다시 보내지 않으며, 다음 변화 때 reset 부터
      for (const m of ["reject", "throw"]) {
        mode = m;
        stroke(w, stage, line(300, 100, 400, 100 + (m === "throw" ? 40 : 0), 4));
        await pumpR(w);
        const nf = H.got.length;
        mode = "";
        await pumpR(w, 12);
        ok("출구: send " + (m === "reject" ? "거부" : "던짐") + " — 삼키고 조용한 동안 다시 보내지 않는다", H.got.length === nf, H.got.length - nf);
        stroke(w, stage, line(300, 200, 400, 220, 4));
        await pumpR(w);
        ok("출구: send " + (m === "reject" ? "거부" : "던짐") + " 뒤 다음 변화 때 reset 부터 — 뷰어가 다시 같다", kinds(H.got.slice(nf))[0] === "reset" && same(w, D, V).ok, [kinds(H.got.slice(nf)), same(w, D, V)]);
      }
      // 중계가 줄곧 실패해도 그리기·저장은 그대로
      mode = "reject";
      stroke(w, stage, line(500, 100, 600, 160, 4));
      advance(w, 2100); await settle();
      const nNow = D.engine.strokes.size;
      ok("출구: 중계가 실패해도 그리기·자동 저장은 그대로", nNow === 10 && w.C.parse(ctl.lastSave).board.strokes.length === nNow && w.W.unsaved(store).length === 0, [nNow, w.W.unsaved(store)]);
      mode = "";
      ok("출구: 닫기도 그대로(true)", await w.W.close() === true);
    }
    // image 실패 — 그 묶음을 보내지 않고, 다음 변화 때 reset 과 함께 바이트를 다시 넘긴다
    {
      const w = world(), store = w.W.memoryStore();
      const { stage, D } = await openWb(w, store);
      let fail = true;
      const vw = world(), H = hub(w, { image: () => (fail ? Promise.reject(new Error("못 받음")) : undefined) });
      const V = viewerOf(vw, H);
      await pumpR(w);
      paste(w, [imgFile(300, 300, false)]); await settle(30); frames(w, 2);
      const n0 = H.got.length;
      await pumpR(w);
      ok("출구: image 가 실패하면 그 묶음을 보내지 않는다", H.got.length === n0 && H.log.filter(x => x.startsWith("image:")).length === 1, H.log);
      fail = false;
      stroke(w, stage, line(100, 100, 200, 100, 4));
      await pumpR(w); await settle(20);
      ok("출구: image 실패 뒤 다음 변화 때 reset 과 함께 바이트를 다시 넘긴다", kinds(H.got.slice(n0))[0] === "reset" && H.log.filter(x => x.startsWith("image:")).length === 2 && same(w, D, V).ok
        && [...V.debug().engine.images.values()][0].state === "ok", [H.log, same(w, D, V)]);
      await w.W.close();
    }

    // 큰 판 — reset 을 조각으로(첫 조각만 reset), 묶음마다 192KB 안. 긴 획 하나는 s 와 a 로 나뉜다
    {
      const w = world(), store = w.W.memoryStore(), C = w.C;
      const big = await store.create("큰 판");
      await store.save(big.id, boardText(C, 30000));
      const n = 20000, pts = [];
      for (let i = 0; i < n; i++) pts.push(100000 + i * 1.37, -100000 - i * 1.11, 0.123);
      const long = await store.create("긴 획");
      await store.save(long.id, C.serialize({ name: "긴 획", bg: "grid", view: { x: 100000, y: -100100, z: 1 }, strokes: [{ tool: "hl", color: 3, w: 12.5, pts }] }));
      const { D } = await openWb(w, store, { id: big.id });
      const vw = world(), H = hub(w), V = viewerOf(vw, H);
      await pumpR(w, 30);
      const sizes = H.got.map(t => Buffer.byteLength(t, "utf8"));
      ok("출구: 큰 판의 reset 은 여러 묶음 — 첫 묶음만 reset, 나머지는 증분", H.got.length >= 5 && kinds(H.got)[0] === "reset" && kinds(H.got).slice(1).every(k => k === ""), [kinds(H.got), sizes]);
      ok("출구: 묶음마다 192KB 안", sizes.every(s => s <= 192 * 1024), sizes);
      ok("왕복: 조각난 reset 을 이어 받은 뷰어가 판 전체와 같다(획 30000)", same(w, D, V).ok && V.debug().board().strokes.length === 30000, same(w, D, V).n);
      const n1 = H.got.length;
      key(w, null, "]", "BracketRight"); await settle(20); frames(w, 2);
      await pumpR(w, 30);
      const parts = H.got.slice(n1), ps = parts.map(t => Buffer.byteLength(t, "utf8")), lo = opsOf(parts);
      ok("출구: 한 묶음에 들지 않는 긴 획은 s 와 a 여럿으로 나뉘고 묶음마다 192KB 안", D.state.id === long.id && parts.length >= 3 && ps.every(s => s <= 192 * 1024)
        && lo.filter(o => o[0] === "s").length === 1 && lo.filter(o => o[0] === "a").length >= 2, [kinds(parts), ps, lo.map(o => o[0])]);
      ok("왕복: 나뉜 긴 획을 이어 받은 뷰어가 같다(점 20000)", same(w, D, V).ok && V.debug().board().strokes[0].pts.length === n * 3, same(w, D, V).n);
      await w.W.close();
    }

    // 읽기 전용으로 열린 판도 보이는 그대로 — 못 읽은 판은 빈 판, 새 판 형식(v2)은 획을 그대로
    {
      const w = world(), store = w.W.memoryStore(), C = w.C;
      const a = await store.create("깨진 판"); await store.save(a.id, "{not json");
      const b = await store.create("새 형식"); const t = JSON.parse(boardText(C, 3)); t.v = 2; await store.save(b.id, JSON.stringify(t));
      const { D } = await openWb(w, store, { id: a.id });
      const vw = world(), H = hub(w), V = viewerOf(vw, H);
      await pumpR(w);
      ok("출구: 본문을 못 읽어 읽기 전용으로 연 판은 빈 판으로 나간다", D.engine.readOnly() && kinds(H.got).join() === "reset" && V.debug().on() && V.debug().board().strokes.length === 0, H.got);
      key(w, null, "]", "BracketRight"); await settle(20); frames(w, 2);
      await pumpR(w);
      ok("출구: 새 판 형식(읽기 전용)도 보이는 그대로 나간다", D.engine.readOnly() && D.state.id === b.id && same(w, D, V).ok && V.debug().board().strokes.length === 3, same(w, D, V));
      await w.W.close();
    }

    // 뷰어 — 받은 값을 믿지 않는다
    {
      const w = world(), C = w.C, V = viewerOf(w, null);
      const B = (k, ops) => JSON.stringify({ v: 1, k, ops });
      const s1 = ["s", 1, "pen", 0, 4, [0, 0, 0.5, 10, 10, 0.5]], s2 = ["s", 2, "hl", 3, 20, [5, 5, 0.5]];
      ok("뷰어: reset 전의 증분은 resync", V.apply(B("", [s1])) === "resync" && V.debug().board().strokes.length === 0);
      ok("뷰어: reset 을 받으면 적용(true)", V.apply(B("reset", [["h", "판", "grid"], s1, s2])) === true && V.debug().board().strokes.length === 2 && V.debug().engine.bg() === "grid");
      const snap = JSON.stringify(V.debug().board());
      ok("뷰어: JSON 이 아닌 묶음은 false, 판은 그대로, 예외를 던지지 않는다", V.apply("{oops") === false && JSON.stringify(V.debug().board()) === snap);
      ok("뷰어: 꼴이 틀린 묶음 뒤의 증분은 reset 까지 resync", V.apply(B("", [["d", [1]]])) === "resync" && JSON.stringify(V.debug().board()) === snap);
      V.apply(B("reset", [["h", "판", "grid"], s1, s2]));
      ok("뷰어: 범위 밖 좌표가 하나라도 있으면 그 묶음 전체를 버린다(false)", V.apply(B("", [["d", [2]], ["s", 3, "pen", 0, 4, [0, 0, 0.5, 2e9, 0, 0.5]]])) === false
        && V.debug().board().strokes.length === 2, V.debug().board().strokes.length);
      V.apply(B("reset", [["h", "판", "dark"], s1]));
      ok("뷰어: 없는 획에 잇기는 resync", V.apply(B("", [["a", 9, [1, 1, 0.5]]])) === "resync");
      V.apply(B("reset", [["h", "판", "dark"], s1]));
      ok("뷰어: 없는 그림 옮기기는 resync", V.apply(B("", [["g", 9, 0, 0, 10, 10]])) === "resync");
      V.apply(B("reset", [["h", "판", "dark"], s1]));
      ok("뷰어: 없는 id 빼기는 건너뛴다(true)", V.apply(B("", [["d", [42]], ["x", [42]]])) === true && V.debug().board().strokes.length === 1);
      // 그림 상한 — 넘는 넣기는 버린다
      const imgs = Array.from({ length: C.LIMIT.images + 1 }, (_, i) => ["i", i + 1, i * 10, 0, 8, 8, "image/png", "b".repeat(64)]);
      const r = V.apply(B("reset", [["h", "판", "dark"], ...imgs]));
      await settle(10);
      const vi = V.debug().board().images;
      ok("뷰어: 판 상한(그림 " + C.LIMIT.images + ")을 넘는 넣기는 버린다", r === true && vi.length === C.LIMIT.images && !vi.some(i => i.id === C.LIMIT.images + 1), vi.length);
      ok("뷰어: 호스트가 그림을 주지 않으면(null) 자리 표시(디코드 실패)", [...V.debug().engine.images.values()].every(im => im.state === "bad" && !im.bmp));
      // 이름은 textContent 로만
      V.apply(B("reset", [["h", "<img src=x onerror=alert(1)>", "dark"]]));
      const title = V.debug().root.querySelector(".itda-wb-title");
      ok("뷰어: 판 이름은 textContent 로만 넣는다", title.textContent === "<img src=x onerror=alert(1)>" && title.children.length === 0);
      // 따라가기
      const views = [];
      const vw = world(), V3 = viewerOf(vw, null, { onView: f => views.push(f) });
      const vst = V3.debug().root.querySelector(".itda-wb-stage");
      vst.clientWidth = 640; vst.clientHeight = 720; dispatch(vw, vw.win, "resize", {});
      V3.apply(B("reset", [["h", "판", "dark"], ["v", 100, 50, 2, 1280, 720]]));
      const v3 = V3.debug().engine.view, sz = V3.debug().engine.size;
      const r0 = [v3.x, v3.y, v3.x + sz.W / v3.z, v3.y + sz.H / v3.z], want = [100, 50, 100 + 640, 50 + 360];
      ok("뷰어: 따라가기 — 화면 비율이 달라도 강사가 보는 영역이 다 들어온다(잘리지 않는다)", V3.following() && sz.W === 640 && r0[0] <= want[0] + 1e-6 && r0[1] <= want[1] + 1e-6 && r0[2] >= want[2] - 1e-6 && r0[3] >= want[3] - 1e-6
        && near(v3.z, 1, 1e-9), [v3, sz]);
      dispatch(vw, vst, "wheel", { deltaX: 30, deltaY: 0, deltaMode: 0, clientX: 100, clientY: 100 });
      const moved = { ...V3.debug().engine.view };
      V3.apply(B("", [["v", 500, 500, 1, 1280, 720]]));
      ok("뷰어: 내가 옮기면 따라가기가 풀리고(onView false) 강사 보기가 와도 움직이지 않는다", !V3.following() && views.join() === "false" && V3.debug().engine.view.x === moved.x && V3.debug().engine.view.y === moved.y, [views, moved, V3.debug().engine.view]);
      V3.setFollow(true);
      const v4 = V3.debug().engine.view;
      ok("뷰어: setFollow(true) 는 곧바로 강사 보기에 맞춘다(onView true)", V3.following() && views.join() === "false,true" && v4.x <= 500 + 1e-6 && v4.x + 640 / v4.z >= 500 + 1280 - 1e-6, [views, v4]);
      // 표면과 함께 열지 않는다
      let threw = null;
      try { vw.W.viewer({ mount: vw.body }); } catch (e) { threw = e; }
      vw.win.ItdaWhiteboardHost = { store: vw.W.memoryStore() };
      const rej = await vw.W.open().then(() => "열림", () => "거부");
      ok("뷰어: 뷰어가 열린 동안 표면 open 은 거부한다", rej === "거부" && !vw.W.isOpen() && threw === null, [rej, threw && threw.message]);
      V3.close();
      ok("뷰어: close 뒤에는 떼어지고 apply 는 false", !vw.body.contains(V3.debug().root) && V3.apply(B("reset", [])) === false);
      const w2 = world();
      await openWb(w2, w2.W.memoryStore());
      let threw2 = null;
      try { w2.W.viewer({ mount: w2.body }); } catch (e) { threw2 = e; }
      ok("뷰어: 표면이 열린 동안 viewer() 는 던진다", !!threw2 && /표면/.test(threw2.message), threw2 && threw2.message);
      await w2.W.close();
    }

    // ---- 중계 리뷰 1회차 반영
    // [1] 중계를 걸지 않은 처음 열기(미저장 기록 없음)에서 list 가 거부돼도 표면은 열린 채 읽기 전용 빈 판과 안내를 보인다
    {
      const w = world(), ctl = {}, store = logged(w.W.memoryStore(), ctl);
      ctl.fail.list = true;
      w.win.ItdaWhiteboardHost = { store };
      const res = await w.W.open().then(() => "열림", e => "거부:" + e.message);
      await settle(); frames(w, 2);
      const root = surfaceOf(w), D = w.W.debug(), note = root && root.querySelector(".itda-wb-note");
      ok("처음 열기: list 거부 — 표면이 열린 채 읽기 전용 빈 판과 안내(중계 없음)", w.W.isOpen() && !!D && D.engine.readOnly() && D.engine.strokes.size === 0
        && !!note && !note.hidden && /판 목록을 읽지 못했습니다/.test(note.textContent), [res, w.W.isOpen(), note && note.textContent]);
      await w.W.close();
    }
    // [2] 상한에 찬 판에서 한 구간(100ms) 안에 지우고 넣기·되돌리기 — 빼기가 넣기보다 먼저 나가 뷰어가 보내는 쪽과 같다(양쪽 상한을 같게 줄인다)
    {
      const limited = async (name, lim, make, drive) => {
        const w = world(), vw = world();
        Object.assign(w.C.LIMIT, lim); Object.assign(vw.C.LIMIT, lim);
        const store = w.W.memoryStore(), b = await store.create("상한");
        await store.save(b.id, make(w.C));
        const { stage, D } = await openWb(w, store, { id: b.id });
        const H = hub(w), V = viewerOf(vw, H);
        await pumpR(w);
        const n0 = H.got.length, okSame0 = same(w, D, V).ok;
        await drive(w, stage, D, H);
        await pumpR(w);
        const inc = H.got.slice(n0), r = same(w, D, V);
        const order = inc.map(t => JSON.parse(t).ops.map(o => o[0]));
        const delFirst = order.every(names => { const firstIns = names.findIndex(x => x === "s" || x === "i"); return firstIns < 0 || Math.max(names.lastIndexOf("d"), names.lastIndexOf("x")) < firstIns; });
        ok("중계 상한: " + name + " — 묶음마다 빼기가 넣기보다 먼저, 뷰어가 보내는 쪽과 같다", okSame0 && r.ok && H.applied.every(a => a === true) && delFirst, [order, r.n, H.applied]);
        await w.W.close();
      };
      const eraseAll = (w, stage, pts) => { key(w, null, "e", "KeyE"); stroke(w, stage, pts); key(w, null, "p", "KeyP"); };
      // 획 수 상한: 획 2(상한 2)를 다 지우고 같은 구간에 새 획
      await limited("획 수 — 지우기→넣기", { strokes: 2 }, C => boardText(C, 2), async (w, stage, D) => {
        eraseAll(w, stage, line(0, 0, 20, 5, 4)); stroke(w, stage, line(300, 300, 400, 300, 4));
        ok("중계 상한: 획 수 — 보내는 쪽은 새 획 하나", D.engine.strokes.size === 1, D.engine.strokes.size);
      });
      // 되돌리기 조합: 지우기·새 획을 보낸 뒤, 한 구간 안에 새 획 되돌리기 → 지우기 되돌리기(옛 획 둘이 같은 id 로 돌아온다), 다시 한 구간 안에 다시 하기 둘
      await limited("획 수 — 되돌리기·다시 하기", { strokes: 2 }, C => boardText(C, 2), async (w, stage, D, H) => {
        eraseAll(w, stage, line(0, 0, 20, 5, 4)); stroke(w, stage, line(300, 300, 400, 300, 4));
        await pumpR(w);
        const V = H.viewer;
        undoKey(w); undoKey(w); await pumpR(w);
        ok("중계 상한: 획 수 — 되돌리기 둘(새 획 빼기 → 옛 획 둘)", D.engine.strokes.size === 2 && same(w, D, V).ok, same(w, D, V).n);
        redoKey(w); redoKey(w);
      });
      // 점 수 상한: 점 200 개짜리 획(상한 210)을 지우고 같은 구간에 짧은 획(점 11 개 이상)
      const bigStroke = C => C.serialize({ name: "판", bg: "dark", view: { x: 0, y: 0, z: 1 }, strokes: [{ tool: "pen", color: 0, w: 4, pts: Array.from({ length: 200 }, (_, i) => [i, 10, 0.5]).flat() }] });
      await limited("점 수 — 지우기→넣기", { points: 210 }, bigStroke, async (w, stage, D) => {
        eraseAll(w, stage, line(50, 0, 50, 20, 4)); stroke(w, stage, line(300, 300, 500, 360, 20));
        const s = lastStroke(D.engine);
        ok("중계 상한: 점 수 — 보내는 쪽은 새 획(점 11 개 이상) 하나", D.engine.strokes.size === 1 && s.pts.length / 3 > 10, [D.engine.strokes.size, s && s.pts.length / 3]);
      });
      await limited("점 수 — 되돌리기·다시 하기", { points: 210 }, bigStroke, async (w, stage, D, H) => {
        eraseAll(w, stage, line(50, 0, 50, 20, 4)); stroke(w, stage, line(300, 300, 500, 360, 20));
        await pumpR(w);
        undoKey(w); undoKey(w); await pumpR(w);
        ok("중계 상한: 점 수 — 되돌리기 둘(새 획 빼기 → 큰 획)", D.engine.strokes.size === 1 && lastStroke(D.engine).pts.length === 600 && same(w, D, H.viewer).ok, same(w, D, H.viewer).n);
        redoKey(w); redoKey(w);
      });
      // 그림 수 상한: 그림 1(상한 1)을 지우고 같은 구간에 새 그림, 그다음 되돌리기 둘을 한 구간에
      await limited("그림 수 — 지우기→넣기·되돌리기", { images: 1 }, C => boardText(C, 0), async (w, stage, D, H) => {
        paste(w, [imgFile(400, 300, false)]); await settle(30); frames(w, 2);
        await pumpR(w);
        key(w, null, "Delete", "Delete");
        paste(w, [imgFile(200, 600, true)]); await settle(30); frames(w, 2);
        ok("중계 상한: 그림 수 — 보내는 쪽은 새 그림 하나", D.engine.images.size === 1 && [...D.engine.images.values()][0].mime === "image/png", D.engine.images.size);
        await pumpR(w); await settle(20);
        ok("중계 상한: 그림 수 — 지우기→넣기 뒤 뷰어도 새 그림 하나", same(w, D, H.viewer).ok, same(w, D, H.viewer));
        undoKey(w); undoKey(w);
      });
    }
    // [3] 5만 획(기본 상한) 일괄 지우기 — d 가 여러 연산·여러 묶음으로 나뉘고 묶음마다 192KB 안, 뷰어는 빈 판
    {
      const w = world(), store = w.W.memoryStore(), C = w.C;
      const b = await store.create("큰 판");
      await store.save(b.id, C.serialize({ name: "큰 판", bg: "dark", view: { x: 0, y: 0, z: 1 }, strokes: Array.from({ length: C.LIMIT.strokes }, () => ({ tool: "pen", color: 0, w: 4, pts: [100, 100, 0.5] })) }));
      const { stage, D } = await openWb(w, store, { id: b.id });
      const vw = world(), H = hub(w), V = viewerOf(vw, H);
      await pumpR(w, 40);
      const n0 = H.got.length, all0 = H.got.map(t => Buffer.byteLength(t, "utf8"));
      ok("중계 대량: 5만 획 판의 reset 이 묶음마다 192KB 안·뷰어가 같다", C.LIMIT.strokes === 50000 && all0.every(s => s <= C.BATCH_MAX) && V.debug().board().strokes.length === 50000, [all0.length, Math.max(...all0)]);
      key(w, null, "e", "KeyE"); stroke(w, stage, [[100, 100], [100, 101]]);
      await pumpR(w, 20);
      const parts = H.got.slice(n0), sizes = parts.map(t => Buffer.byteLength(t, "utf8")), ds = opsOf(parts).filter(o => o[0] === "d");
      ok("중계 대량: 5만 획 일괄 지우기 — d 가 여럿으로 나뉘고 묶음마다 192KB 안", D.engine.strokes.size === 0 && parts.length >= 2 && sizes.every(s => s <= C.BATCH_MAX)
        && ds.length >= 2 && ds.reduce((n, o) => n + o[1].length, 0) === 50000 && kinds(parts).every(k => k === ""), [sizes, ds.length]);
      ok("중계 대량: 뷰어도 빈 판(왕복)", same(w, D, V).ok && V.debug().board().strokes.length === 0 && H.applied.every(a => a === true), same(w, D, V).n);
      await w.W.close();
    }
    // 크기 단언 — 묶음이 BATCH_MAX 를 넘는다고 재면 아무것도 내지 않고, 다음 바뀜 때 reset 부터 다시 한다
    {
      const w = world(), store = w.W.memoryStore(), C = w.C;
      const { stage, D } = await openWb(w, store);
      const vw = world(), H = hub(w), V = viewerOf(vw, H);
      await pumpR(w);
      const real = C.utf8Len, n0 = H.got.length;
      C.utf8Len = s => (s.startsWith('{"v":') ? C.BATCH_MAX + 1 : real(s)); // 완성된 묶음만 넘는다고 잰다
      stroke(w, stage, line(100, 100, 200, 100, 4));
      await pumpR(w);
      ok("중계 크기 단언: 넘는다고 잰 묶음은 내지 않는다", H.got.length === n0, H.got.length - n0);
      C.utf8Len = real;
      stroke(w, stage, line(100, 200, 200, 200, 4));
      await pumpR(w);
      ok("중계 크기 단언: 다음 바뀜 때 reset 부터 — 뷰어가 다시 같다", kinds(H.got.slice(n0))[0] === "reset" && same(w, D, V).ok && V.debug().board().strokes.length === 2, [kinds(H.got.slice(n0)), same(w, D, V).n]);
      await w.W.close();
    }
  }

  console.log("WHITEBOARD DONE cases=" + cases + " failed=" + failed);
})().catch(e => { console.log("예외 " + (e && e.stack || e)); console.log("WHITEBOARD DONE cases=" + cases + " failed=" + (failed + 1)); });
