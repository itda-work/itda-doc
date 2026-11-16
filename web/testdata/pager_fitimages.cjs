// pager.js 의 containSize·fitImages(itda-doc#97)를 최소 DOM 대역으로 돌린다(pager_test.go). 인자: pager.js 경로.
// sizeRow(가로 배치)도 그 부분을 떼어 같이 본다. 대역의 첫 CSS 상자는 아래 img() 주석대로 브라우저를 흉내 낸다(테두리만큼 부모를 넘는 content-box).
// 실제 레이아웃(브라우저의 flex·object-fit)은 selftest·화면 판정이 본다.
"use strict";
const fs = require("fs");
const vm = require("vm");

class Style {
  constructor() { this.props = {}; this.prio = {}; }
  setProperty(k, v, p) { this.props[k] = String(v); this.prio[k] = p || ""; }
  removeProperty(k) { delete this.props[k]; delete this.prio[k]; }
  getPropertyValue(k) { return this.props[k] || ""; }
  getPropertyPriority(k) { return this.prio[k] || ""; }
}
const box = (display, pad = 0, bd = 0, extra = {}) => ({
  display, boxSizing: extra.bb ? "border-box" : "content-box", maxHeight: typeof extra.maxH === "string" ? extra.maxH : extra.maxH ? extra.maxH + "px" : "none",
  paddingLeft: pad + "px", paddingRight: pad + "px", paddingTop: pad + "px", paddingBottom: pad + "px",
  borderLeftWidth: bd + "px", borderRightWidth: bd + "px", borderTopWidth: bd + "px", borderBottomWidth: bd + "px",
});
// 부모 대역: display, 내용 폭(clientWidth — 안쪽 여백 0), 그 위 부모
function parent(display, width, up = null) { return { cs: box(display), clientWidth: display === "inline" ? 0 : width, parentElement: up }; }
// img 대역 — 브라우저의 첫 CSS 상자를 그대로 흉내 낸다. content-box 에서 CSS 의 width: 100% 는 「내용」 폭이 부모 폭(fillW)이라 바깥 상자는
// 테두리·안쪽 여백만큼 부모를 넘는다(리뷰 R1 의 642px). 높이는 flex 부모면 flex 가 정한 바깥 높이(flexH), 아니면 height: auto 의 내용 높이
// (fillW × 비율, max-height 가 내용 상자를 누른다) + 바깥 몫이다. 인라인으로 건 width·height 가 있으면 그 값이 상자다.
function img({ par, fillW, flexH = 0, maxH = 0, nw, nh, pad = 0, bd = 1, bb = false, complete = true, inside = null }) {
  const im = { style: new Style(), naturalWidth: nw, naturalHeight: nh, complete, listeners: {}, parentElement: par };
  im.cs = box("inline", pad, bd, { bb, maxH });
  const ex = 2 * (pad + bd);
  const set = k => im.style.props[k] ? parseFloat(im.style.props[k]) + (bb ? 0 : ex) : null;
  Object.defineProperty(im, "offsetWidth", { get: () => set("width") ?? (bb ? fillW : fillW + ex) });
  Object.defineProperty(im, "offsetHeight", { get: () => {
    const v = set("height"); if (v !== null) return v;
    if (flexH || !fillW) return flexH;
    const ch = im.naturalWidth > 0 ? fillW * im.naturalHeight / im.naturalWidth : 0;
    const lim = typeof maxH === "number" && maxH ? maxH : Infinity; // 문자열 max-height 사례는 flexH 로 그 결과 높이를 준다
    return bb ? Math.min(ch + ex, lim) : Math.min(ch, lim) + ex;
  } });
  im.closest = sel => (inside && sel.split(",").map(x => x.trim()).includes(inside) ? {} : null);
  im.addEventListener = (ev, f) => { (im.listeners[ev] = im.listeners[ev] || []).push(f); };
  im.ex = ex; im.bb = bb;
  return im;
}
const content = (im, k) => parseFloat(im.style.props[k]) - (im.bb ? im.ex : 0);
const outer = (im, k) => parseFloat(im.style.props[k]) + (im.bb ? 0 : im.ex);

const ctx = { console, performance: { now: () => 0 } };
ctx.window = ctx;
ctx.document = {};
ctx.getComputedStyle = el => el.cs;
vm.createContext(ctx);
const src = fs.readFileSync(process.argv[2], "utf8");
vm.runInContext(src, ctx);
const { containSize, fitImages } = ctx.ItdaPager;
// sizeRow 는 공개하지 않는다 — 그 부분(rowImages ~ availWidth)을 그대로 떼어 같은 맥락에서 돌린다
{
  const a = src.indexOf("  function rowImages("), z = src.indexOf("  // ---- 그림 상자를 그림에 붙이기");
  if (a < 0 || z < a) { console.log("FAIL pager.js 에서 sizeRow 부분을 찾지 못함"); process.exit(1); }
  vm.runInContext(src.slice(a, z), ctx);
}
let lastSel = "";
const root = (...xs) => ({ querySelectorAll: sel => { lastSel = sel; return xs; } });

let cases = 0, failed = 0;
function ok(name, cond, got) { cases++; if (!cond) { failed++; console.log("FAIL", name, got === undefined ? "" : JSON.stringify(got)); } }

// 「상자 = 그림」: 건 내용 크기가 그림 비율이고(내림 1px 이내), 바깥 상자(테두리·안쪽 여백 포함)가 예산 outW×outH 안이며, 한 축은 예산을 꽉 채운다
// (빈 띠 없음 — 남는 축은 margin 으로 가운데)
function hugs(name, im, outW, outH) {
  const w = content(im, "width"), h = content(im, "height"), r = im.naturalWidth / im.naturalHeight;
  const ow = outer(im, "width"), oh = outer(im, "height");
  ok(name + ": 비율 그대로", Math.abs(w - h * r) <= 1 || Math.abs(h - w / r) <= 1, [w, h]);
  ok(name + ": 바깥 상자가 예산 안", ow <= outW && oh <= outH, [ow, oh, outW, outH]);
  ok(name + ": 한 축은 꽉", ow >= Math.floor(outW) - 1 || oh >= Math.floor(outH) - 1, [ow, oh, outW, outH]);
  ok(name + ": important 로 건다", im.style.prio.width === "important" && im.style.prio.height === "important", im.style.prio);
  ok(name + ": flex none·margin auto", im.style.props.flex === "none" && im.style.props.margin === "auto", im.style.props);
}

// containSize — 순수 계산
ok("containSize 세로 긴 그림·정사각 상자", JSON.stringify(containSize(640, 720, 600, 1400)) === JSON.stringify({ w: 308, h: 720 }), containSize(640, 720, 600, 1400));
ok("containSize 가로 긴 그림", JSON.stringify(containSize(640, 720, 1600, 500)) === JSON.stringify({ w: 640, h: 200 }), containSize(640, 720, 1600, 500));
ok("containSize 같은 비율", JSON.stringify(containSize(400, 300, 800, 600)) === JSON.stringify({ w: 400, h: 300 }), containSize(400, 300, 800, 600));
ok("containSize 비율 모름 → null", containSize(640, 720, 0, 0) === null);
ok("containSize 상자 0 → null", containSize(0, 720, 600, 1400) === null);

// 패널(1280×720 슬라이드 — 1:1 은 640 폭, 2:1 은 853 폭)의 세 블록 꼴(figure·p > a·p > img — 모두 세로 flex 부모)과
// .fit(fit-sized figure 1100×400 — flex, 링크 단 줄의 .fit — 인라인 a 위 블록 문단 1100 폭, max-height 500)
// × 세로·가로 긴 그림 × 테두리 1px(기본)·6px(두꺼운 테마)
const TALL = [600, 1400], WIDE = [1600, 500];
const layouts = [
  ["패널 1:1 figure", () => parent("flex", 640), 640, 720, 0],
  ["패널 1:1 p > a", () => parent("flex", 640, parent("flex", 640)), 640, 720, 0],
  ["패널 2:1 p > img", () => parent("flex", 853), 853, 720, 0],
  [".fit fit-sized", () => parent("flex", 1100), 1100, 400, 0],
  [".fit 링크 단 줄(인라인 a)", () => parent("inline", 0, parent("block", 1100)), 1100, 0, 500],
];
for (const [tag, mk, W, H, maxH] of layouts) {
  for (const bd of [1, 6]) {
    for (const [nm, [nw, nh]] of [["세로 긴", TALL], ["가로 긴", WIDE]]) {
      const name = tag + " 테두리 " + bd + " " + nm;
      const im = img({ par: mk(), fillW: W, flexH: H, maxH, nw, nh, bd });
      ok(name + ": 첫 CSS 상자는 부모를 넘는다(대역 확인)", im.offsetWidth > W, im.offsetWidth);
      const n = fitImages(root(im));
      ok(name + ": 맞춘 수 1", n === 1, n);
      hugs(name, im, W, H || maxH);
      ok(name + ": flex 부모가 아니면 display block", (im.style.props.display === "block") === (im.parentElement.cs.display !== "flex"), im.style.props.display);
      const before = JSON.stringify(im.style.props);
      fitImages(root(im));
      ok(name + ": 다시 불러도 같음", JSON.stringify(im.style.props) === before, [before, im.style.props]);
    }
  }
}

// 안쪽 여백(spine 의 figure img 처럼)·border-box 테마 — 바깥 상자가 패널 안
{
  const im = img({ par: parent("flex", 640), fillW: 640, flexH: 720, nw: 1600, nh: 500, pad: 6, bd: 1 });
  fitImages(root(im));
  hugs("안쪽 여백 6px 가로 긴", im, 640, 720);
  const t = img({ par: parent("flex", 640), fillW: 640, flexH: 720, nw: 600, nh: 1400, pad: 6, bd: 1 });
  fitImages(root(t));
  hugs("안쪽 여백 6px 세로 긴", t, 640, 720);
  const b = img({ par: parent("flex", 640), fillW: 640, flexH: 720, nw: 1600, nh: 500, pad: 6, bd: 1, bb: true });
  fitImages(root(b));
  hugs("border-box", b, 640, 720);
}

// 계산 스타일 max-height 의 단위(리뷰 R2): px 로 끝나는 값만 상한이다. 백분율·calc 는 브라우저가 문자열 그대로 돌려줄 수 있어 수로 읽으면
// "100%" 가 100px 가 된다 — 그 값의 효과는 CSS 상자 높이(offsetHeight — 여기서는 flexH 로 준 결과)에 이미 들어 있으므로 상한으로 쓰지 않는다.
for (const [css, resultH, budgetH] of [["100%", 719, 719], ["50%", 361.5, 361.5], ["calc(100% - 10px)", 709, 709], ["calc(50% + 2em)", 390, 390],
  ["300px", 719, 300], ["none", 719, 719]]) {
  for (const [nm, [nw, nh]] of [["세로 긴", TALL], ["가로 긴", WIDE]]) {
    const name = "max-height " + css + " " + nm;
    const im = img({ par: parent("flex", 640), fillW: 640, flexH: resultH, maxH: css, nw, nh });
    ok(name + ": 맞춘 수 1", fitImages(root(im)) === 1);
    hugs(name, im, 640, budgetH);
  }
}

// 아직 안 읽힌 그림: 아무것도 걸지 않고 load 를 듣는다 — 읽히면 그 그림만 맞춘다
{
  const im = img({ par: parent("flex", 640), fillW: 640, flexH: 720, nw: 0, nh: 0, complete: false });
  const n = fitImages(root(im));
  ok("안 읽힘: 맞춘 수 0", n === 0, n);
  ok("안 읽힘: style 없음", Object.keys(im.style.props).length === 0, im.style.props);
  ok("안 읽힘: load 를 듣는다", (im.listeners.load || []).length === 1, Object.keys(im.listeners));
  im.naturalWidth = 600; im.naturalHeight = 1400; im.complete = true;
  im.listeners.load[0]();
  hugs("load 뒤", im, 640, 720);
}
// 깨진 그림(읽기는 끝났는데 비율 0): 걸지 않고 듣지도 않는다. 쪽이 화면 밖(상자 0)이면 걸지 않는다
{
  const im = img({ par: parent("flex", 640), fillW: 640, flexH: 720, nw: 0, nh: 0, complete: true });
  fitImages(root(im));
  ok("깨짐: style 없음", Object.keys(im.style.props).length === 0, im.style.props);
  ok("깨짐: 듣지 않음", !im.listeners.load, Object.keys(im.listeners));
  const hid = img({ par: parent("flex", 0), fillW: 0, flexH: 0, nw: 600, nh: 1400, bd: 0 });
  ok("상자 0: 맞추지 않음", fitImages(root(hid)) === 0 && !hid.style.props.width, hid.style.props);
}
// YouTube 대표 그림·가로 배치는 건드리지 않는다
{
  const yt = img({ par: parent("flex", 640), fillW: 640, flexH: 720, nw: 1280, nh: 720, inside: "a.yt" });
  yt.style.setProperty("width", "100%");
  const row = img({ par: parent("grid", 300), fillW: 300, flexH: 200, nw: 1600, nh: 500, inside: "p.fit-row" });
  row.style.setProperty("width", "300px");
  ok("a.yt·p.fit-row: 맞춘 수 0", fitImages(root(yt, row)) === 0);
  ok("a.yt: 그대로", yt.style.props.width === "100%" && !yt.style.props.height, yt.style.props);
  ok("p.fit-row: sizeRow 값 그대로", row.style.props.width === "300px", row.style.props);
}
// 가로 배치(sizeRow): 그림마다 붙은 테두리·안쪽 여백까지 바깥 상자 줄이 문단 폭 안이고 그림 줄 + 캡션 줄이 높이 h 안이다(리뷰 R1 — 1600×500 두 장이
// 본문 양쪽으로 1px 씩 넘었다). 기본 선(1px)·두꺼운 선(6px)·border-box·캡션 있음/없음. 테두리가 없으면 예전 계산과 같다.
function rowImg(nw, nh, bd, pad = 0, bb = false) {
  return { style: {}, naturalWidth: nw, naturalHeight: nh, cs: box("inline", pad, bd, { bb }), ex: 2 * (pad + bd), bb };
}
function rowP(width, imgs, capH) {
  const caps = capH ? imgs.map(() => ({ getBoundingClientRect: () => ({ height: capH }) })) : [];
  return { clientWidth: width, cs: { columnGap: "18.6667px", rowGap: "5.33333px" }, querySelectorAll: sel => (/fr-cap/.test(sel) ? caps : imgs) };
}
const rowOuterW = im => parseFloat(im.style.width) + (im.bb ? 0 : im.ex);
const rowOuterH = im => parseFloat(im.style.height) + (im.bb ? 0 : im.ex);
for (const bd of [0, 1, 6]) {
  for (const [bb, pad] of [[false, 0], [true, 4], [false, 4]]) {
    for (const capH of [0, 20]) {
      for (const [nm, specs] of [["가로 긴 둘", [WIDE, WIDE]], ["세로·가로", [TALL, WIDE]], ["세로 긴 셋", [TALL, TALL, TALL]]]) {
        const imgs = specs.map(([w, h]) => rowImg(w, h, bd, pad, bb));
        const W = 506.671875, h = 300;
        const p = rowP(W, imgs, capH);
        const H = ctx.sizeRow(p, h);
        const name = "sizeRow " + nm + " 선 " + bd + " 여백 " + pad + (bb ? " border-box" : "") + (capH ? " 캡션" : "");
        const total = imgs.reduce((a, im) => a + rowOuterW(im), 0) + 18.6667 * (imgs.length - 1);
        ok(name + ": 바깥 줄이 문단 폭 안", total <= W + 1e-6, [total, W]);
        const capRow = capH ? capH + 5.33333 : 0;
        ok(name + ": 바깥 높이 + 캡션 줄이 h 안", Math.max(...imgs.map(rowOuterH)) + capRow <= h + 1e-6, [imgs.map(rowOuterH), capRow, h]);
        ok(name + ": 같은 내용 높이", imgs.every(im => parseFloat(im.style.height) - (im.bb ? im.ex : 0) === H), imgs.map(im => im.style.height));
        // H 를 정수로 내리므로 폭은 비율 합(px)만큼, 그림마다 폭 내림 1px 만큼 덜 찰 수 있다(예전 계산과 같은 오차)
        const slack = specs.reduce((a, [w, hh]) => a + w / hh, 0) + imgs.length;
        ok(name + ": 폭이나 높이 한쪽은 꽉(내림 오차 안)", total >= W - slack || Math.max(...imgs.map(rowOuterH)) + capRow >= h - 1, [total, W, slack]);
      }
    }
  }
}
{
  // 테두리가 없으면 예전 식 그대로: H = floor(min(h - 캡션, (폭 - 틈) / 비율 합))
  const imgs = [rowImg(1600, 500, 0), rowImg(1600, 500, 0)];
  const H = ctx.sizeRow(rowP(506.671875, imgs, 0), 300);
  ok("sizeRow 테두리 없음: 예전 값", H === Math.floor((506.671875 - 18.6667) / 6.4), H);
}
// 고르는 그림: 쪽의 패널(직접 자식 — 본문 안 사용자 :::slide-panel 은 아니다, cover 는 뺀다)과 본문의 .fit 그림
ok("선택자", lastSel === ":scope > .slide-panel img:not(.img-fill), .page-body img.img-fit", lastSel);

console.log("PAGER DONE cases=" + cases + " failed=" + failed);
