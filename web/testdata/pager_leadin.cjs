// 제목 앞 머리표(.eyebrow — leadIns, itda-doc#92)의 쪽 나눔을 실제 paginate 로 돌린다(leadin_test.go). 머리표는 나누지 않고(atomic) 통째로
// 제목과 함께 다음 쪽으로 간다. 인자: pager.js 경로. 레이아웃 대역: 요소 높이는 data-h(없으면 자식 높이의 합), 쪽 본문은 위 끝 0 부터
// 자식을 쌓고 높이는 H. 여백·글자 나눔은 흉내 내지 않는다 — 안쪽 블록은 자식 없는 div(나눌 수 없음)로 둔다. 실제 배치는 브라우저 PNG 로 본다.
"use strict";
const fs = require("fs");
const vm = require("vm");

let H = 100;
class Text {
  constructor(t) { this.nodeType = 3; this.data = t; this.parentNode = null; }
  get textContent() { return this.data; }
  cloneNode() { return new Text(this.data); }
}
class Elem {
  constructor(tag) { this.nodeType = 1; this.tagName = tag.toUpperCase(); this.attrs = {}; this.childNodes = []; this.classes = new Set(); this.parentNode = null;
    const st = {}; st.setProperty = (k, v) => { st[k] = v; }; this.style = st; this.dataset = {}; }
  get localName() { return this.tagName.toLowerCase(); }
  get className() { return [...this.classes].join(" "); }
  set className(v) { this.classes = new Set(String(v).split(" ").filter(Boolean)); }
  get classList() { const s = this.classes; return { contains: c => s.has(c), add: c => s.add(c), remove: c => s.delete(c) }; }
  get children() { return this.childNodes.filter(c => c.nodeType === 1); }
  get firstChild() { return this.childNodes[0] || null; }
  get lastChild() { return this.childNodes[this.childNodes.length - 1] || null; }
  get lastElementChild() { const c = this.children; return c[c.length - 1] || null; }
  sib(d) { if (!this.parentNode) return null; const s = this.parentNode.children; return s[s.indexOf(this) + d] || null; }
  get previousElementSibling() { return this.sib(-1); }
  get nextElementSibling() { return this.sib(1); }
  get textContent() { return this.childNodes.map(c => c.textContent).join(""); }
  set textContent(t) { this.childNodes.forEach(c => { c.parentNode = null; }); this.childNodes = []; if (t) this.appendChild(new Text(t)); }
  getAttribute(k) { return k in this.attrs ? this.attrs[k] : null; }
  setAttribute(k, v) { this.attrs[k] = String(v); }
  removeAttribute(k) { delete this.attrs[k]; }
  hasAttribute(k) { return k in this.attrs; }
  appendChild(c) { if (c.parentNode) c.parentNode.removeChild(c); this.childNodes.push(c); c.parentNode = this; return c; }
  append(...xs) { for (const x of xs) this.appendChild(typeof x === "string" ? new Text(x) : x); }
  removeChild(c) { const i = this.childNodes.indexOf(c); if (i < 0) throw new Error("removeChild: 자식 아님"); this.childNodes.splice(i, 1); c.parentNode = null; return c; }
  insertBefore(n, ref) { if (n.parentNode) n.parentNode.removeChild(n); const i = ref ? this.childNodes.indexOf(ref) : -1; this.childNodes.splice(i < 0 ? this.childNodes.length : i, 0, n); n.parentNode = this; return n; }
  remove() { if (this.parentNode) this.parentNode.removeChild(this); }
  cloneNode(deep) {
    const c = new Elem(this.tagName); c.attrs = { ...this.attrs }; c.classes = new Set(this.classes);
    if (deep) for (const x of this.childNodes) c.appendChild(x.cloneNode(true));
    return c;
  }
  // 선택자: 「.이름」「태그」「[속성]」 하나만 안다. 그 밖(공백·:scope·[a^=…])은 빈 목록 — 이 시험의 흐름에는 해당 요소가 없다.
  querySelectorAll(sel) {
    let m;
    const test = (m = /^\.([\w-]+)$/.exec(sel)) ? c => c.classes.has(m[1]) : (m = /^\[([\w-]+)\]$/.exec(sel)) ? c => c.hasAttribute(m[1]) :
      /^[a-z]+$/.test(sel) ? c => c.localName === sel : null;
    const out = [];
    if (!test) return out;
    const walk = n => { for (const c of n.children) { if (test(c)) out.push(c); walk(c); } };
    walk(this);
    return out;
  }
  querySelector(sel) { return this.querySelectorAll(sel)[0] || null; }
  get clientHeight() { return this.classes.has("page-body") ? H : heightOf(this); }
  getBoundingClientRect() { const top = topOf(this), h = heightOf(this); return { top, bottom: top + h, height: h, width: 100 }; }
}
function heightOf(el) {
  if (el.classes.has("page-body")) return H;
  if (el.hasAttribute("data-h")) return Number(el.getAttribute("data-h"));
  return el.children.reduce((a, c) => a + heightOf(c), 0);
}
function topOf(el) {
  const p = el.parentNode;
  if (!p || el.classes.has("page-body")) return 0;
  let t = p.classes.has("page-body") ? 0 : topOf(p);
  for (const s of p.children) { if (s === el) break; t += heightOf(s); }
  return t;
}

const ctx = { console, performance: { now: () => 0 }, document: { createElement: t => new Elem(t), createTextNode: t => new Text(t) },
  getComputedStyle: () => ({ lineHeight: "10", fontSize: "8", paddingTop: "0", paddingBottom: "0", borderTopWidth: "0", borderBottomWidth: "0" }) };
ctx.window = ctx;
vm.createContext(ctx);
vm.runInContext(fs.readFileSync(process.argv[2], "utf8"), ctx);
const P = ctx.window.ItdaPager;

let cases = 0, failed = 0;
function ok(name, cond, detail) { cases++; if (!cond) { failed++; console.log("실패 " + name + " — " + JSON.stringify(detail)); } }

// 블록 만들기: B(태그, 높이, 이름표, class) — 자식 없는 블록. EB(이름표, ...자식 높이) — 머리표(자식 div 들).
const B = (tag, h, id, cls) => { const e = new Elem(tag); e.setAttribute("data-h", h); e.setAttribute("data-id", id); if (cls) e.className = cls; return e; };
function EB(id, ...hs) {
  const e = new Elem("div"); e.className = "eyebrow"; e.setAttribute("data-id", id);
  hs.forEach((h, i) => e.appendChild(B("div", h, id + "." + (i + 1))));
  return e;
}
// run 은 블록들을 쪽 나눔해 쪽마다 [이름표(조각이면 +cont, 넘침이면 !)] 목록을 돌려준다. 머리표는 자식 수도 붙인다(나눴는지 보려고).
function run(blocks, o, h) {
  H = h || 100;
  const flow = new Elem("div"); for (const b of blocks) flow.appendChild(b);
  const root = new Elem("div");
  const r = P.paginate(flow, root, o || {});
  const pages = root.children.map(pg => pg.children.find(c => c.classes.has("page-body")).children.map(c =>
    c.getAttribute("data-id") + (c.classes.has("eyebrow") ? "[" + c.children.length + "]" : "") + (c.classes.has("cont") ? "+cont" : "") + (c.classes.has("overflow-clipped") ? "!" : "")));
  // leaves 는 모든 쪽의 잎 블록(자식 없는 블록 — 머리표는 그 자식) 이름표를 차례대로 모은 것이다(유실·중복 확인용).
  const leaves = [];
  const walk = n => { for (const c of n.children) { if (c.children.length) walk(c); else if (c.hasAttribute("data-id")) leaves.push(c.getAttribute("data-id")); } };
  for (const pg of root.children) walk(pg.children.find(c => c.classes.has("page-body")));
  return { pages, r, leaves };
}
const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);

// 1. 두 문단짜리 머리표 — 첫 문단만 들어갈 자리(85 + 10 ≤ 100 < 85 + 20): 쪼개지 않고 제목·본문과 함께 다음 쪽으로
{
  const { pages } = run([B("p", 85, "a"), EB("e", 10, 10), B("h2", 10, "h"), B("p", 20, "b")]);
  ok("1 두 문단 머리표는 통째로 다음 쪽", same(pages, [["a"], ["e[2]", "h", "b"]]), pages);
}
// 2. 뒤 제목이 H1 이고 h1Always — 머리표가 장과 함께 새 쪽으로, 쪽에 머리표 하나만 남는 빈 쪽 없음
{
  const o = { h1Break: true, h1Always: true };
  const a = run([B("p", 50, "a"), EB("e", 10, 10), B("h1", 10, "h"), B("p", 10, "b")], o).pages;
  ok("2a 자리 있음: 머리표+H1 새 쪽", same(a, [["a"], ["e[2]", "h", "b"]]), a);
  const b = run([B("p", 85, "a"), EB("e", 10, 10), B("h1", 10, "h"), B("p", 10, "b")], o).pages;
  ok("2b 첫 문단만 들어갈 자리: 머리표+H1 새 쪽", same(b, [["a"], ["e[2]", "h", "b"]]), b);
  const c = run([EB("e", 10, 10), B("h1", 10, "h"), B("p", 10, "b")], o).pages;
  ok("2c 문서 첫 머리표+H1: 나누지 않음(빈 쪽 없음)", same(c, [["e[2]", "h", "b"]]), c);
}
// 3. 뒤 제목이 H2 이고 본문이 온다 — 머리표·제목은 들어가고 본문이 안 들어가면 셋이 함께 다음 쪽으로
{
  const a = run([B("p", 60, "a"), EB("e", 10, 10), B("h2", 10, "h"), B("p", 30, "b")]).pages;
  ok("3a 본문이 안 들어감: 머리표·제목을 데려감", same(a, [["a"], ["e[2]", "h", "b"]]), a);
  const b = run([B("p", 60, "a"), EB("e", 10, 10), B("h2", 10, "h"), B("p", 10, "b")]).pages;
  ok("3b 다 들어감: 한 쪽", same(b, [["a", "e[2]", "h", "b"]]), b);
  const c = run([B("p", 75, "a"), EB("e", 10, 10), B("h2", 10, "h"), B("p", 10, "b")]).pages;
  ok("3c 머리표만 들어감: 제목과 함께 다음 쪽", same(c, [["a"], ["e[2]", "h", "b"]]), c);
}
// 4. 한 쪽보다 긴 머리표 — 예외 규칙 없이 atomic 넘침 경로(forceFit). 끝나고, 빈 쪽이 없다
{
  const { pages, r, leaves } = run([B("p", 50, "a"), EB("e", 60, 60), B("h2", 10, "h"), B("p", 10, "b")]);
  ok("4 긴 머리표: 끝남·빈 쪽 없음", pages.length >= 2 && pages.length <= 4 && pages.every(p => p.length > 0), { pages, r });
  ok("4 긴 머리표: 첫 쪽에는 앞 문단만", same(pages[0], ["a"]), pages);
  // 넘침 경로(forceFit 의 split-forced 자식 나눔)를 타도 머리표 두 자식·뒤 제목·본문이 차례대로 한 번씩 남는다(유실·중복 없음)
  ok("4 긴 머리표: 머리표 자식·제목·본문 유실 없음", same(leaves, ["a", "e.1", "e.2", "h", "b"]), { leaves, pages });
}
// 5. 대조: 제목 앞이 아닌 .eyebrow(뒤가 문단)는 보통 컨테이너처럼 나뉜다 — 특별 취급은 제목 앞 하나뿐
{
  const { pages } = run([B("p", 85, "a"), EB("e", 10, 10), B("p", 20, "b")]);
  ok("5 문단 앞 머리표는 나뉨", same(pages, [["a", "e[1]"], ["e[1]+cont", "b"]]), pages);
}
console.log("PAGERLEADIN DONE cases=" + cases + " failed=" + failed);
