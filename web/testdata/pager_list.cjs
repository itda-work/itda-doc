// pager.js 의 splitChildren 을 최소 DOM 대역으로 돌려 번호 목록(OL) 뒷조각의 start 를 본다(pager_test.go). 인자: splitChildren 선언(pager.js 에서 표 절 앞까지 자른 조각)을 담은 파일.
// fit 대역은 한 쪽에 항목을 cap 개만 넣는다. partial 이면 넘치는 첫 항목을 반쯤 넣고 나머지를 li.cont 로 돌려준다.
"use strict";
const fs = require("fs");
const body = fs.readFileSync(process.argv[2], "utf8");

class Text {
  constructor(t) { this.nodeType = 3; this.data = t; }
  get textContent() { return this.data; }
  cloneNode() { return new Text(this.data); }
}
class Elem {
  constructor(tag) { this.nodeType = 1; this.tagName = tag.toUpperCase(); this.attrs = {}; this.nodes = []; this.classes = new Set(); }
  get childNodes() { return this.nodes; }
  get children() { return this.nodes.filter(n => n.nodeType === 1); }
  get lastChild() { return this.nodes[this.nodes.length - 1] || null; }
  get lastElementChild() { const c = this.children; return c[c.length - 1] || null; }
  get classList() { const s = this.classes; return { contains: c => s.has(c), add: c => s.add(c) }; }
  get textContent() { return this.nodes.map(c => c.textContent).join(""); }
  set textContent(t) { this.nodes = [new Text(t)]; }
  getAttribute(k) { return k in this.attrs ? this.attrs[k] : null; }
  setAttribute(k, v) { this.attrs[k] = String(v); }
  removeAttribute(k) { delete this.attrs[k]; }
  appendChild(c) { this.nodes.push(c); return c; }
  removeChild(c) { this.nodes.splice(this.nodes.indexOf(c), 1); return c; }
  cloneNode(deep) {
    const c = new Elem(this.tagName); c.attrs = { ...this.attrs }; c.classes = new Set(this.classes);
    if (deep) c.nodes = this.nodes.map(x => x.cloneNode(true));
    return c;
  }
}

let cap = 0, partial = false;
const shallow = (el, cont) => { const c = el.cloneNode(false); if (cont) { c.removeAttribute("id"); c.classList.add("cont"); } return c; };
const deepClone = el => el.cloneNode(true);
const hasContent = el => el.children.length > 0 || (el.textContent || "").trim() !== "";
const keepWithNext = () => false;
function fit(node, parent) {
  if (parent.children.length < cap) { parent.appendChild(node); return { head: node, tail: null }; }
  if (!partial) return null;
  partial = false;
  const h = node.cloneNode(false); h.textContent = "앞"; parent.appendChild(h);
  const t = node.cloneNode(false); t.classList.add("cont"); t.textContent = "뒤";
  return { head: h, tail: t };
}
const splitChildren = Function("shallow", "fit", "deepClone", "hasContent", "keepWithNext",
  body + "\nreturn splitChildren;")(shallow, fit, deepClone, hasContent, keepWithNext);

let failed = 0, cases = 0;
function check(name, ok, msg) { cases++; if (!ok) { failed++; console.log("FAIL", name, msg); } }

function list(n, start) {
  const ol = new Elem("ol");
  if (start !== undefined) ol.setAttribute("start", start);
  for (let i = 0; i < n; i++) { const li = ol.appendChild(new Elem("li")); li.textContent = "항목 " + (i + 1); }
  return ol;
}
// 목록을 끝까지 나눠 조각마다 start 를 모은다(앞 조각은 쪽에 넣고 뒷조각을 다시 나눈다). 첫 조각은 원래 값이다.
function starts(ol, opts) {
  const out = [ol.getAttribute("start")];
  for (let n = ol, first = true; n; first = false) {
    cap = opts.cap; partial = first && !!opts.partial;
    const page = new Elem("div");
    const r = splitChildren(n, page);
    if (!r || !r.tail) break;
    out.push(r.tail.getAttribute("start"));
    n = r.tail;
  }
  return out;
}
function want(name, got, exp) { check(name, JSON.stringify(got) === JSON.stringify(exp), `${JSON.stringify(got)} (기대 ${JSON.stringify(exp)})`); }

want("start 없음", starts(list(10), { cap: 3 }), [null, "4", "7", "10"]);
want("start=3", starts(list(10, "3"), { cap: 3 }), ["3", "6", "9", "12"]);
want("start=0", starts(list(10, "0"), { cap: 3 }), ["0", "3", "6", "9"]);
// 반쯤 들어간 넷째 항목: 앞 조각에 4 개(넷째는 반쪽)가 놓이고 뒷조각의 li.cont 는 번호 없이 보인다 — 다음 항목이 start 를 받는다
want("start 없음·반쪽 항목", starts(list(10), { cap: 3, partial: true }), [null, "4", "7", "10"]);
want("start=0·반쪽 항목", starts(list(10, "0"), { cap: 3, partial: true }), ["0", "3", "6", "9"]);
// 음수는 HTML 이 허용한다 — 그대로 이어 간다. 숫자가 아닌 값만 1 로 둔다.
want("start=-2", starts(list(7, "-2"), { cap: 3 }), ["-2", "1", "4"]);
want("start=숫자 아님", starts(list(7, "가"), { cap: 3 }), ["가", "4", "7"]);
console.log(`PAGERLIST DONE cases=${cases} failed=${failed}`);
