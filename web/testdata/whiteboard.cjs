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
  cloneNode(deep) {
    const c = new El(this.tag); c.className = this.className; c.attrs = { ...this.attrs }; c.dataset = { ...this.dataset }; c._text = this._text;
    if (deep) for (const k of this.kids) c.appendChild(k.cloneNode(true));
    return c;
  }
  listeners(t, capture) { return (this.on[t] || []).filter(x => capture === undefined || x.capture === capture).length; }
}
// ctx2d 는 그리기 명령을 버린다(세는 것만 — fills·strokes).
function ctx2d() {
  const t = { fills: 0, strokes: 0 };
  return new Proxy(t, { get(o, k) { if (k in o) return o[k]; if (k === "fill") return () => { o.fills++; }; if (k === "stroke") return () => { o.strokes++; }; return () => {}; }, set(o, k, v) { o[k] = v; return true; } });
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
  doc.createElement = t => new El(t.toLowerCase());
  doc.createElementNS = (ns, t) => new El(t);
  const w = { doc, win, body, raf: [], timers: new Map(), clock: 1000, seq: 0 };
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
    ok("도구 막대: 펜·형광펜·지우개·색·굵기(색과 따로)·되돌리기·다시·배경·목록·닫기", acts.join() === "toolpen,toolhl,toolerase,popcolor,popwidth,undo,redo,popbg,list,close", acts);
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

  console.log("WHITEBOARD DONE cases=" + cases + " failed=" + failed);
})().catch(e => { console.log("예외 " + (e && e.stack || e)); console.log("WHITEBOARD DONE cases=" + cases + " failed=" + (failed + 1)); });
