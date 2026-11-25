// 캡션 있는 배경 장의 쪽 class(itda-doc#128): 배경 블록(data-slide-bg)에 figcaption 이 있으면 slides.js 가 쪽에 bg-cap 을 달고, 그 class 가
// 쪽 나눔(paginate 의 pageClass)에 이미 들어 있어 나눔이 base.css 의 줄어든 본문 높이로 잰다. 캡션 없는 배경 장·일반 장에는 없다.
// 가짜 DOM·쪽 나눔 대역은 cover_id.cjs 와 같은 꼴이다(대역이 받은 pageClass 를 적어 둔다). 실제 높이·겹침은 이 시험이 보지 않는다(class 와 나눔에 넘긴 pageClass 만).
"use strict";
const fs = require("fs");
const vm = require("vm");
const [slidesPath, boardPath] = process.argv.slice(2);
const seen = [];
let cases = 0, failed = 0;
function ok(name, cond, detail) {
  cases++;
  if (!cond) { failed++; console.log("실패 " + name + " — " + JSON.stringify(detail === undefined ? null : detail)); } else console.log("통과 " + name);
}
const eq = (name, got, want) => ok(name, JSON.stringify(got) === JSON.stringify(want), { got, want });

// ---- 가짜 DOM. 선택자: 쉼표 목록, 조합자(공백·>), :scope, 겹친 단순 선택자(tag·#id·.class·[attr]·[attr="v"]·:not(단순))
class Text {
  constructor(t) { this.nodeType = 3; this.data = t; this.parentNode = null; }
  get textContent() { return this.data; }
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
    const props = new Map();
    this.style = { setProperty: (k, v) => props.set(k, v), getPropertyValue: k => props.get(k) || "" };
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
  cloneNode(deep) { const c = new El(this.tag); c.attrs = { ...this.attrs }; if (deep) for (const k of this.childNodes) c.appendChild(k.cloneNode(true)); return c; }
  addEventListener() {}
}
const h = (tag, attrs, ...kids) => { const e = new El(tag); for (const k in attrs || {}) e.setAttribute(k, attrs[k]); e.append(...kids); return e; };
const doc = { documentElement: { lang: "ko" }, createElement: tag => new El(tag) };
const win = { document: doc };
const ctx = { window: win, document: doc, performance, getComputedStyle: () => ({ fontSize: "16px" }) };
vm.createContext(ctx);
vm.runInContext(fs.readFileSync(slidesPath, "utf8"), ctx);
vm.runInContext(fs.readFileSync(boardPath, "utf8"), ctx);
// 쪽 나눔 대역: 쪽 하나에 받은 흐름을 본문(.page-body)으로 넣는다
win.ItdaPager = {
  paginate(flow, target, opts) {
    seen.push(opts.pageClass);
    const pg = target.appendChild(h("div", { class: "page " + opts.pageClass }));
    if (!flow.classList.contains("page-body")) flow.className = "page-body";
    pg.appendChild(flow);
    return { overflow: 0 };
  },
  fitImages() {},
  fitMath() {},
};
const bgFig = (cap, fill) => h("figure", { "data-slide-bg": "" }, h("img", { class: "img-full" + (fill ? " img-fill" : ""), src: "x.png" }), ...(cap ? [h("figcaption", {}, "캡션")] : []));
const run = blocks => { seen.length = 0; const flow = h("div", {}, ...blocks); const target = h("div"); win.ItdaSlides.build(flow, target, { level: 2 }); return target; };
{
  const t = run([h("h2", {}, "캡션 배경"), bgFig(true), h("ul", {}, h("li", {}, "본문"))]);
  const pg = t.querySelectorAll(".page").find(p => p.querySelector(":scope > .slide-bg"));
  ok("캡션 있는 배경 장: 쪽에 bg-cap", !!pg && pg.classList.contains("bg-cap") && pg.classList.contains("has-bg"), pg && pg.className);
  ok("캡션 있는 배경 장: 나눔이 받은 pageClass 에 bg-cap", seen.some(c => /(^| )has-bg( |$)/.test(c) && /(^| )bg-cap( |$)/.test(c)), seen);
}
{
  const t = run([h("h2", {}, "캡션 cover 배경"), bgFig(true, true), h("p", {}, "본문")]);
  const pg = t.querySelectorAll(".page").find(p => p.querySelector(":scope > .slide-bg"));
  ok("캡션 있는 cover 배경 장: bg-fill 과 bg-cap", !!pg && pg.classList.contains("bg-fill") && pg.classList.contains("bg-cap"), pg && pg.className);
}
{
  const t = run([h("h2", {}, "캡션 없는 배경"), bgFig(false), h("p", {}, "본문")]);
  const pg = t.querySelectorAll(".page").find(p => p.querySelector(":scope > .slide-bg"));
  ok("캡션 없는 배경 장: 배경은 있고 bg-cap 은 없다", !!pg && pg.classList.contains("has-bg") && !pg.classList.contains("bg-cap"), pg && pg.className);
  ok("캡션 없는 배경 장: 나눔 pageClass 에 bg-cap 없음", seen.length > 0 && seen.every(c => !/(^| )bg-cap( |$)/.test(c)), seen);
}
{
  const t = run([h("h2", {}, "일반 장"), h("figure", {}, h("img", { src: "y.png" }), h("figcaption", {}, "보통 그림 캡션")), h("p", {}, "본문")]);
  const pgs = t.querySelectorAll(".page");
  ok("일반 장(배경 아닌 캡션 그림): bg-cap 없음", pgs.length > 0 && pgs.every(p => !p.classList.contains("bg-cap") && !p.classList.contains("has-bg")), pgs.map(p => p.className));
}
console.log("BGCAP DONE cases=" + cases + " failed=" + failed);
process.exitCode = failed ? 1 : 0;
