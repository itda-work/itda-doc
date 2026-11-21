// 표지 id 옮기기(itda-hyve#979): 슬라이드 꼴(slides.js titleSlide)·보드 꼴(dashboard.js board-head)이 새로 만든 표지가
// 원본 표지(section.cover·header.title-block)의 id 를 갖는지 가짜 DOM 위 실제 build 로 본다(cover_id_test.go). 인자: slides.js·dashboard.js 경로.
// 가짜 DOM 은 ask.cjs 와 같은 꼴이다(removeAttribute 를 더했다). 배치·쪽 나눔은 보지 않는다(쪽 나눔은 대역).
"use strict";
const fs = require("fs");
const vm = require("vm");
const [slidesPath, boardPath] = process.argv.slice(2);
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
    const pg = target.appendChild(h("div", { class: "page " + opts.pageClass }));
    if (!flow.classList.contains("page-body")) flow.className = "page-body";
    pg.appendChild(flow);
    return { overflow: 0 };
  },
  fitImages() {},
  fitMath() {},
};
const ids = root => [root, ...root.querySelectorAll("[id]")].filter(e => e.id).map(e => e.id);
const cover = attrs => h("section", { class: "cover", ...attrs }, h("h1", { class: "cover-title" }, "표지 제목"));
const titleBlock = attrs => h("header", { class: "title-block", ...attrs }, h("h1", { class: "title" }, "제목 블록"));

// 슬라이드 꼴
for (const [name, src] of [["section.cover", cover], ["header.title-block", titleBlock]]) {
  {
    const flow = h("div", {}, src({ id: "hyve-cover" }), h("h2", { id: "a" }, "내용"), h("p", {}, "본문"));
    const target = h("div");
    win.ItdaSlides.build(flow, target, { level: 2 });
    const box = target.querySelector(".slide-cover");
    ok("슬라이드 " + name + ": 새 표지 상자가 있다", !!box);
    eq("슬라이드 " + name + ": 원본 표지 id 가 새 표지 상자에", box && box.id, "hyve-cover");
    eq("슬라이드 " + name + ": 그린 장 안에 그 id 는 하나", ids(target).filter(x => x === "hyve-cover").length, 1);
    const pg = box.closest(".page");
    // 발표 사본은 쪽의 자손 id 만 지운다 — 그 꼴을 실제 present.js 에 넘기는 시험은 present_ink.cjs 14절
    ok("슬라이드 " + name + ": id 는 쪽(.page) 자신이 아니라 자손에", !!pg && pg !== box && !pg.hasAttribute("id"));
  }
  {
    const flow = h("div", {}, src({}), h("h2", { id: "a" }, "내용"));
    const target = h("div");
    win.ItdaSlides.build(flow, target, { level: 2 });
    const box = target.querySelector(".slide-cover");
    eq("슬라이드 " + name + ": 원본에 id 가 없으면 새 표지에도 없음", !!box && box.hasAttribute("id"), false);
  }
}

// 보드 꼴
for (const [name, src] of [["section.cover", cover], ["header.title-block", titleBlock]]) {
  {
    const flow = h("div", {}, src({ id: "hyve-cover" }), h("h1", { id: "s" }, "구역"));
    const target = h("div");
    win.ItdaBoard.build(flow, target, {});
    const head = target.querySelector(".board-head");
    ok("보드 " + name + ": 머리가 있다", !!head);
    eq("보드 " + name + ": 원본 표지 id 가 머리에", head && head.id, "hyve-cover");
    eq("보드 " + name + ": 그린 보드 안에 그 id 는 하나", ids(target).filter(x => x === "hyve-cover").length, 1);
    ok("보드 " + name + ": 머리는 보드(.page)의 자손", !!head && head.parentNode.classList.contains("page") && !head.classList.contains("page"));
  }
  {
    const flow = h("div", {}, src({}), h("h1", { id: "s" }, "구역"));
    const target = h("div");
    win.ItdaBoard.build(flow, target, {});
    eq("보드 " + name + ": 원본에 id 가 없으면 머리에도 없음", target.querySelector(".board-head").hasAttribute("id"), false);
  }
}
{
  const flow = h("div", {}, h("h1", { id: "s" }, "구역"));
  const target = h("div");
  win.ItdaBoard.build(flow, target, {});
  eq("보드: 표지가 없으면 머리에 id 없음", target.querySelector(".board-head").hasAttribute("id"), false);
}
console.log("COVERID DONE cases=" + cases + " failed=" + failed);
process.exitCode = failed ? 1 : 0;
