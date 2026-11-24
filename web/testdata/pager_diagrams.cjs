// pager.js 의 sizeDiagrams(그림 자리 — itda-hyve#475·#1074)를 공개된 adoptWidths 로 최소 DOM 대역에서 돌린다(pager_test.go). 인자: pager.js 경로.
// 속성 → SVG 에 거는 CSSOM 글자(aspect-ratio·width)만 본다. CSS calc 가 실제로 풀리는 레이아웃은 selftest·화면 판정이 본다.
"use strict";
const fs = require("fs");
const vm = require("vm");

const ctx = { console, performance: { now: () => 0 } };
ctx.window = ctx;
ctx.document = {};
ctx.getComputedStyle = () => ({});
vm.createContext(ctx);
vm.runInContext(fs.readFileSync(process.argv[2], "utf8"), ctx);
const { adoptWidths } = ctx.ItdaPager;

// figure.diagram 대역 — data-diagram-* 속성과 그 안의 svg.diagram-svg(style 은 CSSOM 처럼 글자를 담는 빈 객체)
function fig(attrs) {
  const svg = { style: {} };
  return { svg, getAttribute: k => (k in attrs ? attrs[k] : null), querySelector: sel => (sel === ":scope > svg.diagram-svg" ? svg : null) };
}
const root = (...figs) => ({ querySelectorAll: sel => (/figure\.diagram/.test(sel) ? figs : []) });

let cases = 0, failed = 0;
function ok(name, cond, got) { cases++; if (!cond) { failed++; console.log("FAIL", name, got === undefined ? "" : JSON.stringify(got)); } }

const CAP = "(var(--body-h, 200mm) - 60px)";
// 받는 값 — aspect-ratio 와 너비 상한 계산식에 비율이 그대로(소수 포함) 들어간다
for (const [v, rw, rh] of [["16:9", "16", "9"], ["1.91:1", "1.91", "1"], ["2.35:1", "2.35", "1"], ["0.5:1", "0.5", "1"], ["0.1:1", "0.1", "1"], ["99.99:10", "99.99", "10"], ["1:10", "1", "10"]]) {
  const f = fig({ "data-diagram-ratio": v });
  adoptWidths(root(f));
  ok(v + ": aspect-ratio", f.svg.style.aspectRatio === rw + " / " + rh, f.svg.style);
  ok(v + ": width", f.svg.style.width === "min(100%, calc(" + CAP + " * " + rw + " / " + rh + "))", f.svg.style.width);
  ok(v + ": max-height none", f.svg.style.maxHeight === "none", f.svg.style);
}
// 소수 ratio + height — 너비 계산식에 소수 비율이 그대로 들어간다
{
  const f = fig({ "data-diagram-ratio": "1.91:1", "data-diagram-height": "60%" });
  adoptWidths(root(f));
  ok("1.91:1 + height: width", f.svg.style.width === "min(100%, calc(var(--body-h, 200mm) * 60 / 100 * 1.91 / 1), calc(" + CAP + " * 1.91 / 1))", f.svg.style.width);
}
// 받지 않는 값 — 상자를 걸지 않는다(수 하나 꼴은 mdoc 이 R:1 로 정규화해 내므로 속성으로는 받지 않는다)
for (const v of ["2", "0:1", "1:0", "0.0:1", "10.01:1", "1:10.01", "100:1", "1.234:1", ".5:1", "1.:1", "05:1", "-1:1", "1e2:1", "\"1.5:1\"", "1.5 : 1"]) {
  const f = fig({ "data-diagram-ratio": v });
  adoptWidths(root(f));
  ok(JSON.stringify(v) + ": 상자 없음", Object.keys(f.svg.style).length === 0, f.svg.style);
}

console.log("PAGER DONE cases=" + cases + " failed=" + failed);
if (failed) process.exit(1);
