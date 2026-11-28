// 코드 블록 배율·맞춤(itda-hyve#1116 — pager_codefit_test.go)을 실제 paginate 로 돌린다. 인자: pager.js 경로.
// 레이아웃 대역: 쪽 본문은 너비 W·높이 H, 블록은 위 끝 0 부터 쌓는다(여백 없음). 코드 블록(pre > code)의 글자는 배율 s(pre 의 --code-scale, 없으면 1)에서
// 한 글자 폭 CW·s, 줄 높이 LH·s 이고 줄은 본문 폭에서 접힌다(white-space: pre 로 재는 동안은 접지 않는다). code 를 max-content 로 두면 폭이 가장 긴 줄이다.
// 그 밖의 블록은 data-h 높이다. 실제 글꼴·최소 글자 크기·안쪽 여백은 흉내 내지 않는다 — 실제 배치는 hyve selftest·브라우저로 본다.
"use strict";
const fs = require("fs");
const vm = require("vm");

const W = 400, H = 200, CW = 10, LH = 20;
class Text {
  constructor(t) { this.nodeType = 3; this.data = t; this.parentNode = null; }
  get textContent() { return this.data; }
  set textContent(t) { this.data = t; }
  cloneNode() { return new Text(this.data); }
}
function parseCompound(s) {
  const c = { tag: "", cls: [], attrs: [] };
  const re = /^([a-zA-Z][\w-]*|\*)|\.([\w-]+)|\[([\w-]+)(?:="([^"]*)")?\]/y;
  let i = 0;
  while (i < s.length) {
    re.lastIndex = i;
    const m = re.exec(s);
    if (!m || m.index !== i) return null; // 모르는 선택자 — 이 대역의 흐름에는 해당 요소가 없다(빈 목록)
    if (m[1]) c.tag = m[1] === "*" ? "" : m[1].toUpperCase();
    else if (m[2]) c.cls.push(m[2]);
    else c.attrs.push([m[3], m[4]]);
    i = re.lastIndex;
  }
  return c;
}
function matchCompound(e, c) {
  if (!e || e.nodeType !== 1 || !c) return false;
  if (c.tag && e.tagName !== c.tag) return false;
  for (const k of c.cls) if (!e.classList.contains(k)) return false;
  for (const [a, v] of c.attrs) { const g = e.getAttribute(a); if (g === null || (v !== undefined && g !== v)) return false; }
  return true;
}
// 선택자: 쉼표 목록, 「A > B」「:scope > B」「A B」 와 단순 선택자 겹침
function matchSel(e, sel, scope) {
  const parts = sel.trim().replace(/\s*>\s*/g, " > ").split(/\s+/);
  const last = parseCompound(parts[parts.length - 1]);
  if (!matchCompound(e, last)) return false;
  if (parts.length === 1) return true;
  const comb = parts[parts.length - 2], up = parts[parts.length - 3];
  if (comb === ">") return up === ":scope" ? e.parentNode === scope : matchCompound(e.parentNode, parseCompound(up));
  const anc = parseCompound(comb);
  for (let n = e.parentNode; n; n = n.parentNode) if (matchCompound(n, anc)) return true;
  return false;
}
class Elem {
  constructor(tag) {
    this.nodeType = 1; this.tagName = tag.toUpperCase(); this.attrs = {}; this.childNodes = []; this.classes = new Set(); this.parentNode = null;
    this.props = new Map(); const self = this;
    this.style = { setProperty: (k, v) => self.props.set(k, String(v)), getPropertyValue: k => self.props.get(k) || "", removeProperty: k => self.props.delete(k) };
    this.dataset = new Proxy({}, { get: (_, k) => self.getAttribute("data-" + String(k)) ?? undefined, set: (_, k, v) => { self.setAttribute("data-" + String(k), v); return true; } });
  }
  get localName() { return this.tagName.toLowerCase(); }
  get className() { return [...this.classes].join(" "); }
  set className(v) { this.classes = new Set(String(v).split(" ").filter(Boolean)); }
  get classList() { const s = this.classes; return { contains: c => s.has(c), add: (...c) => c.forEach(x => s.add(x)), remove: (...c) => c.forEach(x => s.delete(x)) }; }
  get children() { return this.childNodes.filter(c => c.nodeType === 1); }
  get firstChild() { return this.childNodes[0] || null; }
  get lastChild() { return this.childNodes[this.childNodes.length - 1] || null; }
  get lastElementChild() { const c = this.children; return c[c.length - 1] || null; }
  get firstElementChild() { return this.children[0] || null; }
  get parentElement() { return this.parentNode; }
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
    const c = new Elem(this.tagName); c.attrs = { ...this.attrs }; c.classes = new Set(this.classes); c.props = new Map(this.props);
    if (deep) for (const x of this.childNodes) c.appendChild(x.cloneNode(true));
    return c;
  }
  matches(sel) { return sel.split(",").some(s => matchSel(this, s, null)); }
  closest(sel) { for (let e = this; e && e.nodeType === 1; e = e.parentNode) if (e.matches(sel)) return e; return null; }
  querySelectorAll(sel) {
    const out = [], list = sel.split(",");
    const walk = n => { for (const c of n.children) { if (list.some(s => matchSel(c, s, this))) out.push(c); walk(c); } };
    walk(this);
    return out;
  }
  querySelector(sel) { return this.querySelectorAll(sel)[0] || null; }
  get clientHeight() { return this.classes.has("page-body") ? H : heightOf(this); }
  get clientWidth() { return W; }
  get offsetWidth() { return this.tagName === "CODE" && this.style.getPropertyValue("width") === "max-content" ? longest(this) * CW * scaleOf(this) : W; }
  getBoundingClientRect() { const top = topOf(this), h = heightOf(this); return { top, bottom: top + h, height: h, width: W }; }
}
const preOf = el => { for (let e = el; e; e = e.parentNode) if (e.tagName === "PRE") return e; return null; };
const scaleOf = el => { const p = preOf(el); return (p && Number(p.style.getPropertyValue("--code-scale"))) || 1; };
const lines = el => el.textContent.replace(/\n$/, "").split("\n");
const longest = el => Math.max(...lines(el).map(l => l.length));
function heightOf(el) {
  if (el.classes.has("page-body")) return H;
  if (el.hasAttribute("data-h")) return Number(el.getAttribute("data-h"));
  if (el.tagName === "PRE") {
    const s = scaleOf(el), nowrap = el.style.getPropertyValue("white-space") === "pre";
    return lines(el).reduce((a, l) => a + (nowrap ? 1 : Math.max(1, Math.ceil(l.length * CW * s / W - 1e-9))), 0) * LH * s;
  }
  return el.children.reduce((a, c) => a + heightOf(c), 0);
}
function topOf(el) {
  const p = el.parentNode;
  if (!p || el.classes.has("page-body")) return 0;
  let t = p.classes.has("page-body") ? 0 : topOf(p);
  for (const s of p.children) { if (s === el) break; t += heightOf(s); }
  return t;
}

const ctx = { console, performance: { now: () => 0 }, CSS: { escape: s => s }, document: { createElement: t => new Elem(t), createTextNode: t => new Text(t) },
  getComputedStyle: () => ({ lineHeight: String(LH), fontSize: "8", marginTop: "0", marginBottom: "0", paddingLeft: "0", paddingRight: "0", paddingTop: "0", paddingBottom: "0", borderTopWidth: "0", borderBottomWidth: "0", display: "block" }) };
ctx.window = ctx;
vm.createContext(ctx);
vm.runInContext(fs.readFileSync(process.argv[2], "utf8"), ctx);
const P = ctx.window.ItdaPager;

let cases = 0, failed = 0;
function ok(name, cond, detail) { cases++; if (!cond) { failed++; console.log("실패 " + name + " — " + JSON.stringify(detail)); } else console.log("통과 " + name); }

// 코드 블록: n 줄, 줄마다 chars 글자(첫 줄만 first 글자 — 가장 긴 줄). attrs 는 pre 속성(data-fit·data-scale·data-cols)
function code(n, chars, attrs, first) {
  const pre = new Elem("pre"); pre.setAttribute("data-line", "1");
  for (const k in attrs || {}) pre.setAttribute(k, attrs[k]);
  const c = new Elem("code");
  c.textContent = Array.from({ length: n }, (_, i) => "x".repeat(i === 0 && first ? first : chars)).join("\n") + "\n";
  pre.appendChild(c);
  return pre;
}
const B = (tag, h) => { const e = new Elem(tag); e.setAttribute("data-h", h); return e; };
// run 은 블록을 나눠 쪽마다 [태그(pre 는 배율·줄 수)] 목록을 돌려준다
function run(blocks, slide) {
  const flow = new Elem("div"); for (const b of blocks) flow.appendChild(b);
  const target = new Elem("div");
  P.paginate(flow, target, slide ? { pageClass: "slide", fitFill: true } : {});
  return target.children.map(pg => pg.querySelector(".page-body").children.map(x => x.tagName === "PRE" ? { s: Number(x.style.getPropertyValue("--code-scale")) || 1, n: lines(x).length } : x.tagName));
}
const pres = pages => pages.flat().filter(x => typeof x === "object");
const near = (a, b, d) => Math.abs(a - b) <= d;

// A. 폭 맞춤(A4 — 높이 맞춤 없음): 가장 긴 줄 60자(600px) → 400/600, 넘치는 높이는 지금처럼 나뉘고 조각마다 같은 배율
{
  const r = run([code(30, 20, { "data-fit": "on" }, 60)], false);
  const ps = pres(r), s = ps[0] && ps[0].s;
  ok("A4 폭 맞춤: 가장 긴 줄이 폭에 들어가는 배율(≤ 2/3, 3% 안)", s <= 400 / 600 + 1e-9 && s >= 400 / 600 * 0.97, s);
  ok("A4 폭 맞춤: 높이는 맞추지 않고 쪽을 나눈다 — 조각마다 같은 배율", r.length >= 2 && ps.every(p => p.s === s) && ps.reduce((a, p) => a + p.n, 0) === 30, r);
}
// B. 폭이 넉넉하면 배율을 걸지 않는다(1) — scale= 이 있으면 그 값이 상한
{
  ok("폭 맞춤: 짧은 줄은 배율 없음", pres(run([code(3, 20, { "data-fit": "on" })], false))[0].s === 1, null);
  ok("폭 맞춤 + scale=0.8: 짧은 줄은 0.8 그대로", pres(run([code(3, 20, { "data-fit": "on", "data-scale": "0.8" })], false))[0].s === 0.8, null);
  const s = pres(run([code(3, 20, { "data-fit": "on", "data-scale": "0.8" }, 60)], false))[0].s;
  ok("폭 맞춤 + scale=0.8: 긴 줄은 0.8 에서 더 줄인다", s <= 400 / 600 + 1e-9 && s > 0.6, s);
  const lo = pres(run([code(3, 20, { "data-fit": "on" }, 200)], false))[0].s;
  ok("폭 맞춤: 하한 0.55 에서 멈춘다(그때는 줄이 바뀐다)", lo === 0.55, lo);
}
// C. scale= 만: 그 블록에만, 맞추지 않는다
{
  const r = run([code(3, 20, { "data-scale": "0.8" }, 60), code(3, 20, {})], false);
  const ps = pres(r);
  ok("scale=0.8: 그 블록에만 0.8, 다른 블록은 1", ps[0].s === 0.8 && ps[1].s === 1, ps);
  ok("scale 범위 밖(1.6)은 무시", pres(run([code(3, 20, { "data-scale": "1.6" })], false))[0].s === 1, null);
}
// D. 슬라이드 높이 맞춤: 15줄(300px) → 빈 쪽 맨 위에서 200/300 으로 한 장
{
  const r = run([code(15, 20, { "data-fit": "on" })], true);
  const ps = pres(r);
  ok("슬라이드 높이 맞춤: 한 장에 통째로", r.length === 1 && ps.length === 1 && ps[0].n === 15, r);
  ok("슬라이드 높이 맞춤: 배율 ≤ 2/3 이고 0.01 안", ps[0].s <= 200 / 300 + 1e-9 && ps[0].s > 200 / 300 - 0.01, ps[0].s);
  const a4 = run([code(15, 20, { "data-fit": "on" })], false);
  ok("A4 에서는 같은 블록이 높이를 맞추지 않아 두 쪽", a4.length === 2 && pres(a4).every(p => p.s === 1), a4);
}
// E. 하한에서도 넘치면 하한 배율로 지금처럼 나뉜다(40줄 × 20 × 0.55 = 440px)
{
  const r = run([code(40, 20, { "data-fit": "on" })], true);
  const ps = pres(r);
  ok("하한 넘침: 조각마다 0.55 로 나뉜다", r.length >= 3 && ps.every(p => p.s === 0.55) && ps.reduce((a, p) => a + p.n, 0) === 40, r);
}
// F. 놓인 자리에 본문 높이 30% 도 안 남으면 새 장에서 시작(앞 블록 150px — 남은 50px < 60px)
{
  const r = run([B("p", 150), code(12, 20, { "data-fit": "on" })], true);
  ok("30% 규칙: 앞 장에는 앞 블록만", r.length === 2 && r[0].length === 1 && r[0][0] === "P", r);
  // 들어가는지는 fits() 처럼 0.5px 를 봐준다 — 높이 12줄 × 20 × s ≤ 200.5
  ok("30% 규칙: 새 장에서 통째로(200/240)", r[1].length === 1 && r[1][0].n === 12 && near(r[1][0].s, 200 / 240, 0.01) && 240 * r[1][0].s <= H + 0.5, r);
}
// G. 30% 는 남았지만 하한에서도 안 들어가면 새 장에서 다시(남은 100px, 15줄 × 0.55 = 165px)
{
  const r = run([B("p", 100), code(15, 20, { "data-fit": "on" })], true);
  ok("자리에 하한도 안 들어감: 새 장에서 통째로 맞춘다", r.length === 2 && r[1].length === 1 && r[1][0].n === 15 && near(r[1][0].s, 200 / 300, 0.01), r);
}
// H. 자리에 들어가게 줄여 같은 장에(남은 150px, 10줄 → 0.75)
{
  const r = run([B("p", 50), code(10, 20, { "data-fit": "on" })], true);
  ok("자리에서 줄임: 같은 장에 0.75 근처", r.length === 1 && r[0].length === 2 && near(r[0][1].s, 0.75, 0.01) && 50 + 200 * r[0][1].s <= H + 0.5, r);
}
// I. 제목만 있는 장은 빈 장으로 친다 — 하한에서도 넘치면 되돌지 않고 그 장에서 나뉜다(무한 되풀이 없음)
{
  const hd = B("h2", 30);
  const r = run([hd, code(40, 20, { "data-fit": "on" })], true);
  const ps = pres(r);
  ok("제목 뒤 하한 넘침: 첫 장은 제목 + 조각, 모두 0.55", r[0][0] === "H2" && typeof r[0][1] === "object" && ps.every(p => p.s === 0.55) && ps.reduce((a, p) => a + p.n, 0) === 40, r);
}
// J. 다단(cols=) 블록은 폭을 맞추지 않는다(높이는 맞춘다)
{
  const r = run([code(3, 20, { "data-fit": "on", "data-cols": "2" }, 60)], false);
  ok("cols= 블록: 폭 맞춤 없음", pres(r)[0].s === 1, r);
}
// K. 목록 안 블록도 폭은 맞춘다(높이 맞춤은 맨 위 블록만)
{
  const li = new Elem("li"); li.appendChild(code(3, 20, { "data-fit": "on" }, 60));
  const ul = new Elem("ul"); ul.appendChild(li);
  run([ul], true);
  const s = Number(li.querySelector("pre").style.getPropertyValue("--code-scale"));
  ok("목록 안 fit 블록: 폭 맞춤", s > 0 && s <= 400 / 600 + 1e-9, s);
}
console.log("CODEFIT DONE cases=" + cases + " failed=" + failed);
process.exitCode = failed ? 1 : 0;
