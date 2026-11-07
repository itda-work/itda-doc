// pager.js 의 fitMath 를 최소 DOM 대역으로 돌린다(pager_test.go). 인자: pager.js 경로. 실제 레이아웃은 selftest 가 본다.
"use strict";
const fs = require("fs");
const vm = require("vm");

class Style {
  constructor(el) { this.el = el; this.props = {}; }
  sync() { const s = Object.entries(this.props).map(([k, v]) => k + ": " + v).join("; "); if (s) this.el.attrs.style = s; else if ("style" in this.el.attrs) this.el.attrs.style = ""; }
  setProp(k, v) { this.props[k] = v; this.sync(); }
  removeProperty(k) { delete this.props[k]; this.sync(); }
  set width(v) { this.setProp("width", v); }
  get width() { return this.props.width || ""; }
  set height(v) { this.setProp("height", v); }
  get height() { return this.props.height || ""; }
  set transform(v) { this.setProp("transform", v); }
  get transform() { return this.props.transform || ""; }
  set transformOrigin(v) { this.setProp("transform-origin", v); }
  get transformOrigin() { return this.props["transform-origin"] || ""; }
}
class Elem {
  constructor() { this.attrs = {}; this.style = new Style(this); }
  getAttribute(k) { return k in this.attrs ? this.attrs[k] : null; }
  removeAttribute(k) { delete this.attrs[k]; }
}
// 수식 하나: boxW 는 본문 너비, natural 은 수식 제 너비, h 는 제 높이. wideBox 면 WebKit 처럼 상자가 내용만큼 늘고, 아니면 상자는 본문 너비이고 내용만 넘친다.
function mathBox(boxW, natural, h, wideBox) {
  const m = new Elem();
  const box = new Elem();
  box.clientWidth = boxW;
  box.querySelector = () => m;
  // 확대된 조상의 배율(화면 px / 배치 px) — rect 만 이 값을 곱는다. clientWidth·scrollWidth·style 은 배치 px 다
  box.zoom = 1;
  Object.defineProperty(box, "offsetWidth", { get: () => box.clientWidth });
  box.getBoundingClientRect = () => ({ width: box.clientWidth * box.zoom });
  Object.defineProperty(m, "scrollWidth", { configurable: true, get: () => (wideBox ? Math.max(boxW, natural) : boxW) });
  m.getBoundingClientRect = () => {
    const k = /scale\(([^)]+)\)/.exec(m.style.transform);
    return { width: (parseFloat(m.style.width) || boxW) * (k ? Number(k[1]) : 1) * box.zoom, height: h * (k ? Number(k[1]) : 1) * box.zoom };
  };
  m.natural = () => natural * box.zoom;
  return { box, m };
}
const ctx = { console, performance: { now: () => 0 } };
ctx.window = ctx;
let ranged = null;
ctx.document = { createRange: () => ({ selectNodeContents: m => { ranged = m; }, getBoundingClientRect: () => ({ width: ranged.natural() }) }) };
vm.createContext(ctx);
vm.runInContext(fs.readFileSync(process.argv[2], "utf8"), ctx);
const fit = ctx.ItdaPager.fitMath;
const root = (...xs) => ({ querySelectorAll: () => xs.map(x => x.box) });

let cases = 0, failed = 0;
function ok(name, cond, got) { cases++; if (!cond) { failed++; console.log("FAIL", name, got === undefined ? "" : JSON.stringify(got)); } }

for (const wideBox of [true, false]) {
  const tag = wideBox ? "상자가 느는 엔진" : "내용만 넘치는 엔진";
  const w = mathBox(600, 1000, 50, wideBox);
  fit(root(w));
  ok(tag + ": 넓은 수식이 scale(폭 비율)", w.m.style.transform === "scale(0.6)", w.m.style.transform);
  ok(tag + ": 상자 너비를 제 너비로 못 박음", w.m.style.width === "1000px", w.m.style.width);
  ok(tag + ": 왼쪽 위 기준", w.m.style.transformOrigin === "0 0", w.m.style.transformOrigin);
  ok(tag + ": 블록 높이 = 제 높이 × 비율", w.box.style.height === "30px", w.box.style.height);
  ok(tag + ": 줄인 너비가 본문 너비", w.m.getBoundingClientRect().width === 600, w.m.getBoundingClientRect());

  // 다시 불러도 같고(멱등), 본문이 넓어지면 다시 재서 풀린다
  fit(root(w));
  ok(tag + ": 다시 불러도 같은 값", w.m.style.transform === "scale(0.6)" && w.box.style.height === "30px", [w.m.style.transform, w.box.style.height]);
  w.box.clientWidth = 800;
  fit(root(w));
  ok(tag + ": 본문이 달라지면 새 비율", w.m.style.transform === "scale(0.8)" && w.box.style.height === "40px", [w.m.style.transform, w.box.style.height]);
  w.box.clientWidth = 1200;
  Object.defineProperty(w.m, "scrollWidth", { get: () => 1200 });
  fit(root(w));
  ok(tag + ": 넉넉해지면 값을 모두 지우고 style 도 남기지 않음", !w.m.style.transform && !w.box.style.height && !w.m.style.width &&
    !("style" in w.m.attrs) && !("style" in w.box.attrs), [w.m.attrs, w.box.attrs]);
}

// 확대된 조상 안(크게 보기): 화면 px 인 rect 를 배치 px 로 맞춘다 — 배율이 달라도 값이 같다
for (const wideBox of [true, false]) {
  const z = mathBox(600, 1000, 50, wideBox);
  z.box.zoom = 1.54;
  fit(root(z));
  ok("배율 1.54 에서도 같은 비율·높이·너비(" + wideBox + ")", z.m.style.transform === "scale(0.6)" && z.box.style.height === "30px" && z.m.style.width === "1000px",
    [z.m.style.transform, z.box.style.height, z.m.style.width]);
}

const n = mathBox(600, 200, 50, false);
fit(root(n));
ok("좁은 수식은 건드리지 않음(style 없음)", !n.m.style.transform && !n.box.style.height && !("style" in n.m.attrs) && !("style" in n.box.attrs), [n.m.attrs, n.box.attrs]);

const edge = mathBox(600, 600.4, 50, false);
fit(root(edge));
ok("반 픽셀 안의 차이는 넘침이 아님", !edge.m.style.transform, edge.m.style.transform);

const hidden = mathBox(0, 1000, 0, false);
fit(root(hidden));
ok("숨은 자리(너비 0)는 줄이지 않음", !hidden.m.style.transform, hidden.m.style.transform);

const noMath = new Elem();
noMath.querySelector = () => null;
fit({ querySelectorAll: () => [noMath] });
ok("math 없는 블록은 넘어감", !("style" in noMath.attrs));

console.log("PAGER DONE cases=" + cases + " failed=" + failed);
process.exit(failed ? 1 : 0);
