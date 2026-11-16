// 제목 앞 머리표(:::eyebrow, itda-doc#92): slides.js 의 groups 가 제목 바로 앞 형제인 .eyebrow 를 그 제목의 묶음(lead)으로 넘기는지,
// pager.js 의 leadIn 이 그 꼴만 고르는지 node 로 돌린다(leadin_test.go). 인자: slides.js·pager.js 경로. 실제 배치(머리 높이·쪽 경계)는 브라우저 PNG 로 본다.
"use strict";
const fs = require("fs");
const vm = require("vm");

// 최소 DOM 대역: 태그·class·속성과 형제만. matches 는 「tag」「.cls」「tag.cls」 를 쉼표로 이은 선택자만 안다.
class El {
  constructor(tag, cls) { this.nodeType = 1; this.tagName = tag.toUpperCase(); this.cls = new Set((cls || "").split(" ").filter(Boolean)); this.attrs = {}; this.parent = null; }
  get classList() { const s = this.cls; return { contains: c => s.has(c) }; }
  hasAttribute(k) { return k in this.attrs; }
  matches(sel) {
    return sel.split(",").some(one => {
      const m = one.trim().match(/^([a-z0-9]*)((?:\.[a-z0-9-]+)*)$/i);
      if (!m) throw new Error("모르는 선택자 " + one);
      if (m[1] && m[1].toUpperCase() !== this.tagName) return false;
      return m[2].split(".").filter(Boolean).every(c => this.cls.has(c));
    });
  }
  get previousElementSibling() { const k = this.parent.kids; const i = k.indexOf(this); return i > 0 ? k[i - 1] : null; }
  get nextElementSibling() { const k = this.parent.kids; const i = k.indexOf(this); return i >= 0 && i < k.length - 1 ? k[i + 1] : null; }
  remove() { const k = this.parent.kids; k.splice(k.indexOf(this), 1); }
}
function flow(...kids) { const f = { kids }; for (const k of kids) k.parent = f; Object.defineProperty(f, "children", { get: () => f.kids }); return f; }
const E = (tag, cls) => new El(tag, cls);

const ctx = { console, document: { documentElement: { lang: "ko" } } };
ctx.window = ctx;
vm.createContext(ctx);
vm.runInContext(fs.readFileSync(process.argv[2], "utf8"), ctx);
const S = ctx.window.ItdaSlides;

let cases = 0, failed = 0;
function ok(name, cond, detail) { cases++; if (!cond) { failed++; console.log("실패 " + name + " — " + JSON.stringify(detail)); } }

// 1. 내용 장: 앞 장 본문 끝의 머리표가 다음 제목의 장으로 넘어간다
{
  const h1 = E("h2"), p1 = E("p"), eb = E("div", "eyebrow"), h2 = E("h2"), p2 = E("p");
  const gs = S.groups(flow(h1, p1, eb, h2, p2), 2);
  ok("내용 장 둘", gs.length === 2, gs.length);
  ok("앞 장에 머리표가 남지 않음", gs[0].blocks.length === 1 && gs[0].blocks[0] === p1, gs[0].blocks.length);
  ok("다음 장의 lead", gs[1].lead === eb && gs[1].title === h2, null);
  ok("머리표 없는 장의 lead 는 null", gs[0].lead === null, null);
}
// 2. 구역 장(슬라이드 단계보다 높은 제목)도 같다
{
  const eb = E("div", "eyebrow"), h1 = E("h1"), h2 = E("h2"), p = E("p");
  const gs = S.groups(flow(eb, h1, h2, p), 2);
  ok("머리표뿐인 앞 묶음은 걸러짐", gs.length === 2 && gs[0].kind === "section", gs.map(g => g.kind));
  ok("구역 장의 lead", gs[0].lead === eb, null);
}
// 3. 제목 바로 앞이 아닌 머리표·제목 아닌 다음 블록 앞의 머리표는 보통 블록
{
  const h1 = E("h2"), eb = E("div", "eyebrow"), p = E("p"), h2 = E("h2"), q = E("p");
  const gs = S.groups(flow(h1, eb, p, h2, q), 2);
  ok("문단 앞 머리표는 본문에 남음", gs[0].blocks[0] === eb && gs[1].lead === null, null);
}
// 4. 가로줄 뒤 머리표 → 제목: 가로줄이 연 묶음 없이 제목 장으로
{
  const h1 = E("h2"), p = E("p"), hr = E("hr"), eb = E("div", "eyebrow"), h2 = E("h2"), q = E("p");
  const gs = S.groups(flow(h1, p, hr, eb, h2, q), 2);
  ok("가로줄 뒤 머리표도 제목 장으로", gs.length === 2 && gs[1].lead === eb, gs.length);
}
// 5. 슬라이드 단계 밖 제목(h3) 앞 머리표는 그대로 본문
{
  const h2 = E("h2"), eb = E("div", "eyebrow"), h3 = E("h3"), p = E("p");
  const gs = S.groups(flow(h2, eb, h3, p), 2);
  ok("h3 앞 머리표는 본문", gs.length === 1 && gs[0].blocks[0] === eb && gs[0].lead === null, null);
}
// 6. 다른 class 의 div 는 넘기지 않는다
{
  const h1 = E("h2"), d = E("div", "box"), h2 = E("h2"), p = E("p");
  const gs = S.groups(flow(h1, d, h2, p), 2);
  ok("box 는 앞 장에", gs[0].blocks[0] === d && gs[1].lead === null, null);
}

// pager.js leadIn — 함수만 떼어 본다
const psrc = fs.readFileSync(process.argv[3], "utf8");
const m = psrc.match(/\n  function leadIn\(n\) \{[\s\S]*?\n  \}\n/);
ok("pager.js 에 leadIn 이 있음", !!m, null);
if (m) {
  const leadIn = Function(m[0] + "\nreturn leadIn;")();
  const a = E("div", "eyebrow"), h = E("h2"), b = E("div", "eyebrow"), p = E("p"), c = E("div", "box"), h3 = E("h3");
  flow(a, h, b, p, c, h3);
  ok("leadIn: 제목 앞 머리표", leadIn(a) === true, null);
  ok("leadIn: 문단 앞 머리표는 아님", leadIn(b) === false, null);
  ok("leadIn: 제목 앞 다른 div 는 아님", leadIn(c) === false, null);
  ok("leadIn: 끝의 제목은 아님", leadIn(h3) === false, null);
  ok("keepWithNext 가 leadIns 를 봄", /const keepWithNext = [^\n]*leadIns\.has\(n\)/.test(psrc), null);
  ok("paginate 가 맨 위 흐름에서 고름", /leadIns = new WeakSet\(queue\.filter\(leadIn\)\)/.test(psrc), null);
}
console.log("LEADIN DONE cases=" + cases + " failed=" + failed);
