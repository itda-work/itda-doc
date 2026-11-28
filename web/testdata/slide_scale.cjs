// 장 배율·장 맞춤(itda-hyve#1116 — slide_scale_test.go): 슬라이드를 여는 제목의 data-scale(mdoc 이 검사한 {scale=})·class fit({.fit})으로 slides.js 가
// 그 장의 쪽에만 --slide-body-scale 을 걸고, .fit 이면 한 장이 되는 가장 큰 배율을 고르는지 가짜 DOM 위 실제 build 를 node 로 돌려 본다.
// 쪽 나눔 대역: 받은 흐름의 블록마다 data-u(배율 1 에서의 높이)를 더해 본문 배율 s(나눔 상자 target 에 걸린 --slide-body-scale, 없으면 문서 배율 0.9)를
// 곱하고 쪽 본문 높이 CAP 으로 나눠 쪽 수를 정한다(실제 줄 바꿈·여백은 흉내 내지 않는다 — 실제 배치는 hyve selftest·브라우저로 본다).
// 인자: slides.js·dashboard.js·incr.js 경로.
"use strict";
const fs = require("fs");
const vm = require("vm");
const [slidesPath, boardPath, incrPath] = process.argv.slice(2);
let cases = 0, failed = 0;
function ok(name, cond, detail) {
  cases++;
  if (!cond) { failed++; console.log("실패 " + name + " — " + JSON.stringify(detail === undefined ? null : detail)); } else console.log("통과 " + name);
}

// ---- 가짜 DOM(slide_bg_cap.cjs 와 같은 꼴에 outerHTML·style 사본을 더했다)
class Text {
  constructor(t) { this.nodeType = 3; this.data = t; this.parentNode = null; }
  get textContent() { return this.data; }
  get outerHTML() { return this.data.replace(/&/g, "&amp;").replace(/</g, "&lt;"); }
  cloneNode() { return new Text(this.data); }
}
function parseCompound(s) {
  const c = { tag: "", id: "", cls: [], attrs: [], not: [] };
  const re = /^([a-zA-Z][\w-]*|\*)|#([\w-]+)|\.([\w-]+)|\[([\w-]+)(?:="([^"]*)")?\]|:not\(([^)]*)\)/y;
  let i = 0;
  while (i < s.length) {
    re.lastIndex = i;
    const m = re.exec(s);
    if (!m || m.index !== i) throw new Error("선택자 모름: " + s);
    if (m[1]) c.tag = m[1] === "*" ? "" : m[1].toUpperCase();
    else if (m[2]) c.id = m[2];
    else if (m[3]) c.cls.push(m[3]);
    else if (m[4]) c.attrs.push([m[4], m[5]]);
    else if (m[6] !== undefined) c.not.push(parseCompound(m[6].trim()));
    i = re.lastIndex;
  }
  return c;
}
function matchCompound(e, c) {
  if (!e || e.nodeType !== 1) return false;
  if (c.tag && e.tagName !== c.tag) return false;
  if (c.id && e.id !== c.id) return false;
  for (const k of c.cls) if (!e.classList.contains(k)) return false;
  for (const [a, v] of c.attrs) { const g = e.getAttribute(a); if (g === null || (v !== undefined && g !== v)) return false; }
  for (const n of c.not) if (matchCompound(e, n)) return false;
  return true;
}
function parseComplex(s) {
  const toks = s.trim().replace(/\s*>\s*/g, " > ").split(/\s+/);
  const out = [];
  let comb = " ";
  for (const t of toks) {
    if (t === ">") { comb = ">"; continue; }
    out.push({ c: t === ":scope" ? null : parseCompound(t), comb, scope: t === ":scope" });
    comb = " ";
  }
  return out;
}
function matchComplex(e, parts, scope) {
  const last = parts[parts.length - 1];
  if (last.scope ? e !== scope : !matchCompound(e, last.c)) return false;
  const rest = parts.slice(0, -1);
  if (!rest.length) return true;
  const up = (node, idx) => {
    const p = rest[idx], combo = parts[idx + 1].comb;
    const test = n => (p.scope ? n === scope : matchCompound(n, p.c)) && (idx === 0 || up(n, idx - 1));
    if (combo === ">") return !!node.parentNode && test(node.parentNode);
    for (let n = node.parentNode; n; n = n.parentNode) if (test(n)) return true;
    return false;
  };
  return up(e, rest.length - 1);
}
const splitList = s => s.split(",").map(x => x.trim()).filter(Boolean);
class El {
  constructor(tag) {
    this.nodeType = 1; this.tag = tag.toLowerCase(); this.childNodes = []; this.parentNode = null; this.attrs = {};
    const self = this;
    this.classList = {
      contains: x => self.cls().includes(x),
      add: (...a) => { const c = self.cls(); for (const x of a) if (!c.includes(x)) c.push(x); self.attrs.class = c.join(" "); },
      remove: (...a) => { self.attrs.class = self.cls().filter(x => !a.includes(x)).join(" "); },
      toggle: (x, f) => { const on = f === undefined ? !self.cls().includes(x) : !!f; if (on) self.classList.add(x); else self.classList.remove(x); return on; },
    };
    this.dataset = new Proxy({}, {
      get: (_, k) => self.getAttribute("data-" + String(k).replace(/[A-Z]/g, m => "-" + m.toLowerCase())) ?? undefined,
      set: (_, k, v) => { self.setAttribute("data-" + String(k).replace(/[A-Z]/g, m => "-" + m.toLowerCase()), String(v)); return true; },
    });
    this.props = new Map();
    this.style = { setProperty: (k, v) => self.props.set(k, String(v)), getPropertyValue: k => self.props.get(k) || "", removeProperty: k => self.props.delete(k) };
  }
  cls() { return String(this.attrs.class || "").split(/\s+/).filter(Boolean); }
  get tagName() { return this.tag.toUpperCase(); }
  get id() { return this.attrs.id || ""; }
  set id(v) { this.attrs.id = v; }
  get className() { return this.attrs.class || ""; }
  set className(v) { this.attrs.class = v; }
  get children() { return this.childNodes.filter(k => k.nodeType === 1); }
  get parentElement() { return this.parentNode; }
  get firstChild() { return this.childNodes[0] || null; }
  sib(d) { const p = this.parentNode; if (!p) return null; const l = p.children; return l[l.indexOf(this) + d] || null; }
  get previousElementSibling() { return this.sib(-1); }
  get nextElementSibling() { return this.sib(1); }
  get textContent() { return this.childNodes.map(k => k.textContent).join(""); }
  set textContent(t) { this.childNodes = []; if (t !== "") this.append(String(t)); }
  get outerHTML() {
    const a = Object.entries(this.attrs).map(([k, v]) => " " + k + '="' + String(v).replace(/"/g, "&quot;") + '"').join("");
    return "<" + this.tag + a + ">" + this.childNodes.map(k => k.outerHTML).join("") + "</" + this.tag + ">";
  }
  get innerHTML() { return this.childNodes.map(k => k.outerHTML).join(""); }
  replaceChildren(...kids) { this.textContent = ""; this.append(...kids); }
  insertBefore(k, ref) {
    k.remove();
    const index = ref ? this.childNodes.indexOf(ref) : this.childNodes.length;
    this.childNodes.splice(index, 0, k);
    k.parentNode = this;
    return k;
  }
  getBoundingClientRect() { return { height: 20 }; }
  getAttribute(k) { return k in this.attrs ? String(this.attrs[k]) : null; }
  hasAttribute(k) { return k in this.attrs; }
  setAttribute(k, v) { this.attrs[k] = String(v); }
  removeAttribute(k) { delete this.attrs[k]; }
  append(...ks) { for (const k of ks) this.appendChild(typeof k === "string" ? new Text(k) : k); return this; }
  appendChild(k) { if (k.parentNode) k.parentNode.childNodes.splice(k.parentNode.childNodes.indexOf(k), 1); k.parentNode = this; this.childNodes.push(k); return k; }
  remove() { const p = this.parentNode; if (p) { p.childNodes.splice(p.childNodes.indexOf(this), 1); this.parentNode = null; } }
  matches(s) { return splitList(s).some(x => matchComplex(this, parseComplex(x), null)); }
  closest(s) { for (let e = this; e && e.nodeType === 1; e = e.parentNode) if (e.matches(s)) return e; return null; }
  querySelectorAll(s) {
    const lists = splitList(s).map(parseComplex), out = [];
    const walk = e => { for (const k of e.children) { if (lists.some(p => matchComplex(k, p, this))) out.push(k); walk(k); } };
    walk(this);
    return out;
  }
  querySelector(s) { return this.querySelectorAll(s)[0] || null; }
  cloneNode(deep) { const c = new El(this.tag); c.attrs = { ...this.attrs }; c.props = new Map(this.props); if (deep) for (const k of this.childNodes) c.appendChild(k.cloneNode(true)); return c; }
  addEventListener() {}
}
const h = (tag, attrs, ...kids) => { const e = new El(tag); for (const k in attrs || {}) e.setAttribute(k, attrs[k]); e.append(...kids); return e; };
const doc = { documentElement: { lang: "ko" }, createElement: tag => new El(tag) };
const win = { document: doc };
const ctx = { window: win, document: doc, performance, getComputedStyle: () => ({ fontSize: "16px" }) };
vm.createContext(ctx);
vm.runInContext(fs.readFileSync(slidesPath, "utf8"), ctx);
vm.runInContext(fs.readFileSync(boardPath, "utf8"), ctx);

// ---- 쪽 나눔 대역. CAP 은 쪽 본문 높이(배율 1 의 data-u 단위). 쪽 수 = ceil(Σ data-u × s / CAP) — 첫 쪽에 흐름을 넣고 나머지 쪽은 빈 본문이다.
// data-atomic 블록은 나눌 수 없는 블록이다(pager.js forceFit 흉내): 높이 × s 가 CAP 을 넘으면 한 쪽만 차지한 채 잘리고(overflow +1), 넘지 않으면 보통 블록처럼 센다.
const CAP = 100, DOC = 0.9;
const calls = [];
win.ItdaPager = {
  paginate(flow, target, opts) {
    const s = Number(target.style.getPropertyValue("--slide-body-scale")) || DOC;
    let overflow = 0;
    const units = flow.children.reduce((a, b) => {
      const u = Number(b.getAttribute("data-u")) || 0;
      if (b.hasAttribute("data-atomic") && u * s > CAP) { overflow++; return a + CAP / s; }
      return a + u;
    }, 0);
    const n = Math.max(1, Math.ceil(units * s / CAP - 1e-9));
    calls.push({ s, n, overflow });
    for (let i = 0; i < n; i++) {
      const pg = target.appendChild(h("div", { class: "page " + opts.pageClass }));
      if (i === 0) { flow.className = "page-body " + (flow.className || ""); pg.appendChild(flow); } else pg.appendChild(h("div", { class: "page-body" }));
    }
    return { overflow };
  },
  fitImages() {},
  fitMath() {},
};
const P = u => h("p", { "data-u": String(u) }, "글");
const pagesOf = (t, title) => t.querySelectorAll(":scope > .page").filter(p => { const x = p.querySelector(".slide-title"); return x && x.textContent.startsWith(title); });
const bodyScale = pg => pg.style.getPropertyValue("--slide-body-scale");
function run(blocks, o) {
  calls.length = 0;
  const flow = h("div", {}, ...blocks);
  const target = h("div");
  win.ItdaSlides.build(flow, target, Object.assign({ level: 2, fontScale: DOC }, o || {}));
  return target;
}

// 1. scale= — 그 장의 쪽에만 본문 배율, 다른 장·쪽의 문서 배율(--slide-scale)은 그대로
{
  const t = run([h("h2", { "data-scale": "0.7" }, "배율 장"), P(50), h("h2", {}, "보통 장"), P(50)]);
  const a = pagesOf(t, "배율 장"), b = pagesOf(t, "보통 장");
  ok("scale=0.7: 그 장 쪽에 --slide-body-scale 0.7", a.length === 1 && bodyScale(a[0]) === "0.7", a.map(bodyScale));
  ok("scale=0.7: 다른 장에는 본문 배율이 없다", b.length === 1 && bodyScale(b[0]) === "", b.map(bodyScale));
  ok("scale=0.7: 문서 배율(--slide-scale)은 두 장 모두 0.9 그대로 — 머리(.slide-title)는 그것을 쓴다",
    [...a, ...b].every(p => p.style.getPropertyValue("--slide-scale") === "0.9"), [...a, ...b].map(p => p.style.getPropertyValue("--slide-scale")));
  ok("scale=0.7: 머리(.slide-head)에는 배율을 걸지 않는다", a[0].querySelector(".slide-head").style.getPropertyValue("--slide-body-scale") === "", null);
  ok("scale=0.7: 나눔이 그 배율로 쟀다", calls.some(c => c.s === 0.7) && calls.some(c => c.s === DOC), calls);
}
// 2. scale= 이 「(계속)」 장에도 — 0.7 × 300 = 210 → 3장, 세 장 모두 0.7
{
  const t = run([h("h2", { "data-scale": "0.7" }, "긴 장"), P(300)]);
  const a = pagesOf(t, "긴 장");
  ok("scale=0.7 긴 장: (계속) 장까지 모두 0.7", a.length === 3 && a.every(p => bodyScale(p) === "0.7"), a.map(bodyScale));
}
// 3. .fit — 기본 배율로는 2장(150 × 0.9 = 135)인 내용을 한 장에. 배율은 한 장이 되는 쪽(≤ 100/150)이고 하한 0.6 이상
{
  const t = run([h("h2", { class: "fit" }, "맞춤 장"), P(150), h("h2", {}, "뒤 장"), P(150)]);
  const a = pagesOf(t, "맞춤 장"), b = pagesOf(t, "뒤 장");
  const s = Number(bodyScale(a[0] || h("div")));
  ok(".fit: 기본 배율로 2장인 내용이 한 장", a.length === 1, a.length);
  ok(".fit: 배율이 한 장 상한(100/150) 이하·하한 0.6 이상·상한 근처(0.02 안)", s >= 0.6 && s <= 100 / 150 + 1e-9 && s > 100 / 150 - 0.02, s);
  ok(".fit: 「(계속)」 표지 없음", !a[0].querySelector(".slide-seq, .slide-cont"), null);
  ok(".fit: 뒤 장은 그대로 2장·배율 없음", b.length === 2 && b.every(p => bodyScale(p) === ""), b.map(bodyScale));
  // 맞춤 장의 마른 나눔(상한·하한 + 6번) + 두 장의 실제 나눔 — 대역의 머리 높이는 표지와 상관없이 같아 표지 높이 재기 나눔은 없다
  ok(".fit: 나눔 수는 마른 나눔 8번 + 실제 나눔 2번", calls.length === 10, calls.length);
}
// 4. .fit — 하한에서도 넘치면 하한 배율로 「(계속)」 (250 × 0.6 = 150 → 2장)
{
  const t = run([h("h2", { class: "fit" }, "넘치는 맞춤"), P(250)]);
  const a = pagesOf(t, "넘치는 맞춤");
  ok(".fit 하한 넘침: 하한 0.6 으로 2장", a.length === 2 && a.every(p => bodyScale(p) === "0.6"), a.map(bodyScale));
  ok(".fit 하한 넘침: 「(계속 k/n)」 표지", !!a[0].querySelector(".slide-seq") && !!a[1].querySelector(".slide-cont"), null);
}
// 5. .fit — 문서 배율로 이미 한 장이면 배율을 걸지 않는다
{
  const t = run([h("h2", { class: "fit" }, "짧은 맞춤"), P(50)]);
  const a = pagesOf(t, "짧은 맞춤");
  ok(".fit 짧은 장: 배율 없음(문서 배율 그대로)", a.length === 1 && bodyScale(a[0]) === "", a.map(bodyScale));
}
// 6. .fit + scale= — 상한이 scale 값. 짧으면 그 값, 길면 그 아래에서 한 장
{
  const t = run([h("h2", { class: "fit", "data-scale": "1.2" }, "큰 맞춤"), P(50), h("h2", { class: "fit", "data-scale": "0.8" }, "작은 맞춤"), P(150)]);
  const a = pagesOf(t, "큰 맞춤"), b = pagesOf(t, "작은 맞춤");
  ok(".fit scale=1.2 짧은 장: 상한 1.2 그대로", a.length === 1 && bodyScale(a[0]) === "1.2", a.map(bodyScale));
  const s = Number(bodyScale(b[0] || h("div")));
  ok(".fit scale=0.8 긴 장: 0.8 아래에서 한 장", b.length === 1 && s < 0.8 && s <= 100 / 150 + 1e-9 && s >= 0.6, [b.length, s]);
}
// 6b. .fit — 상한에선 한 장이지만 나눌 수 없는 블록이 잘리고(150 × 0.9 = 135 > 100), 줄이면 드는 내용: 잘림 없는 배율(≤ 100/150)로 줄인다
//     (itda-hyve#1116 리뷰 1회차 [1] — 장 수만 보던 판정은 상한 0.9 를 골라 잘린 채 두었다)
{
  const t = run([h("h2", { class: "fit" }, "잘림 맞춤"), h("h3", { "data-u": "150", "data-atomic": "" }, "긴 제목")]);
  const a = pagesOf(t, "잘림 맞춤");
  const s = Number(bodyScale(a[0] || h("div")));
  ok(".fit 잘림: 상한 0.9 에서 마른 나눔이 한 장+잘림이었다", calls.some(c => c.s === 0.9 && c.n === 1 && c.overflow === 1), calls);
  ok(".fit 잘림: 잘리지 않는 배율(≤ 100/150, 0.02 안)로 한 장", a.length === 1 && s <= 100 / 150 + 1e-9 && s > 100 / 150 - 0.02, [a.length, s]);
  ok(".fit 잘림: 실제 나눔은 잘림 없음", calls[calls.length - 1].overflow === 0, calls[calls.length - 1]);
}
// 6c. .fit — 하한에서도 잘리면(200 × 0.6 = 120 > 100) 하한 배율 그대로 잘린 채 한 장(나눌 수 없는 블록이라 「(계속)」 없음)
{
  const t = run([h("h2", { class: "fit" }, "하한 잘림"), h("h3", { "data-u": "200", "data-atomic": "" }, "아주 긴 제목")]);
  const a = pagesOf(t, "하한 잘림");
  ok(".fit 하한 잘림: 하한 0.6 으로 한 장", a.length === 1 && bodyScale(a[0]) === "0.6", a.map(bodyScale));
  ok(".fit 하한 잘림: 실제 나눔은 잘린 채(overflow 1)", calls[calls.length - 1].overflow === 1 && calls[calls.length - 1].s === 0.6, calls[calls.length - 1]);
}
// 7. 뜻이 없는 자리 — 구역 장(# 제목)·범위 밖 값·제목 없는 장
{
  const t = run([h("h1", { "data-scale": "0.7", class: "fit" }, "구역"), P(150), h("h2", { "data-scale": "2" }, "틀린 값"), P(50), h("hr"), P(50)]);
  const pgs = t.querySelectorAll(":scope > .page");
  ok("구역 장·범위 밖 값·제목 없는 장: 본문 배율 없음", pgs.length > 0 && pgs.every(p => bodyScale(p) === ""), pgs.map(bodyScale));
}
// 8. 이어 만들기(itda-hyve#658): 배율 속성이 바뀌면 그 장을 다시 만들고, 같으면 그대로 쓴다
{
  vm.runInContext(fs.readFileSync(incrPath, "utf8"), ctx);
  const memo = {};
  const once = attrs => { const flow = h("div", {}, h("h2", Object.assign({ "data-line": "3" }, attrs), "이어 장"), P(50), h("h2", { "data-line": "9" }, "고정 장"), P(50));
    return win.ItdaSlides.build(flow, h("div"), { level: 2, fontScale: DOC, memo }); };
  once({ "data-scale": "0.7" });
  const same = once({ "data-scale": "0.7" });
  ok("이어 만들기: 같은 배율이면 두 장 모두 그대로 씀", same.made === 0 && same.reused === 2, [same.made, same.reused]);
  const changed = once({ "data-scale": "0.8" });
  ok("이어 만들기: scale 이 바뀐 장만 다시 만듦", changed.made === 1 && changed.reused === 1 && bodyScale(changed.pages[0]) === "0.8", [changed.made, changed.reused]);
  const fit = once({ "data-scale": "0.8", class: "fit" });
  ok("이어 만들기: .fit 이 붙은 장도 다시 만듦", fit.made === 1 && fit.reused === 1, [fit.made, fit.reused]);
}
// 9. fitBodyScale(DOM 없음)
{
  const F = win.ItdaSlides.fitBodyScale;
  const byCap = u => s => ({ pages: Math.max(1, Math.ceil(u * s / CAP - 1e-9)), overflow: 0 });
  // clipAt: 한 장 그대로지만 u × s 가 CAP 을 넘으면 잘림(나눌 수 없는 블록 하나)
  const clipAt = u => s => ({ pages: 1, overflow: u * s > CAP ? 1 : 0 });
  ok("fitBodyScale: 상한에서 한 장+잘림, 하한에서 듦 → 줄인 배율(≤ 0.8, 0.015 안)", (x => x <= 0.8 && x > 0.785)(F(1.5, 0.6, clipAt(125))), F(1.5, 0.6, clipAt(125)));
  ok("fitBodyScale: 하한에서도 잘리면 하한", F(0.9, 0.6, clipAt(200)) === 0.6, F(0.9, 0.6, clipAt(200)));
  ok("fitBodyScale: 상한에서 한 장이면 상한", F(0.9, 0.6, byCap(100)) === 0.9, F(0.9, 0.6, byCap(100)));
  ok("fitBodyScale: 하한에서도 넘치면 하한", F(0.9, 0.6, byCap(200)) === 0.6, F(0.9, 0.6, byCap(200)));
  const s = F(1.5, 0.6, byCap(125));
  ok("fitBodyScale: 사이는 한 장이 되는 쪽(≤ 0.8)이고 0.8 에서 0.015 안", s <= 0.8 && s > 0.785, s);
  let n = 0;
  F(1.5, 0.6, x => { n++; return byCap(125)(x); });
  ok("fitBodyScale: 재기는 상한·하한 + 6번", n === 8, n);
  ok("fitBodyScale: 상한이 하한보다 작으면 상한", F(0.5, 0.6, byCap(1000)) === 0.5, F(0.5, 0.6, byCap(1000)));
}
console.log("SLIDESCALE DONE cases=" + cases + " failed=" + failed);
process.exitCode = failed ? 1 : 0;
