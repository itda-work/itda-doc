// pager.js 의 표 행 나눔(itda-hyve#795 — paginate 옵션 splitTallRows 의 splitTable·splitRow)을 레이아웃을 흉내 낸 작은 DOM 대역으로 돌린다(pager_test.go).
// 인자: pager.js 의 「const BLOCK_TAGS」 부터 「.fit 그림」 앞까지를 담은 파일. 실제 레이아웃(글꼴·표 테두리)은 hyve 의 공유 PDF 시험이 본다.
// 흉내 내는 레이아웃: 칸 글은 CPL 글자마다 한 줄(LH px), 칸 안 큰 수식·블록(data-h)은 그 높이, 칸은 위아래 PAD, 행은 칸 가운데 가장 높은 것,
// 표·tbody 는 자식 높이의 합, 쪽 본문은 위 끝 0 부터 자식을 쌓는다.
"use strict";
const fs = require("fs");
const part = fs.readFileSync(process.argv[2], "utf8");

const CPL = 30, LH = 10, PAD = 2;
class Text {
  constructor(t) { this.nodeType = 3; this.data = t; this.parentNode = null; }
  get textContent() { return this.data; }
  cloneNode() { return new Text(this.data); }
  remove() { if (this.parentNode) this.parentNode.removeChild(this); }
}
class Elem {
  constructor(tag) { this.nodeType = 1; this.tagName = tag.toUpperCase(); this.attrs = {}; this.childNodes = []; this.classes = new Set(); this.parentNode = null; this.style = {}; }
  get localName() { return this.tagName.toLowerCase(); }
  get children() { return this.childNodes.filter(c => c.nodeType === 1); }
  get firstChild() { return this.childNodes[0] || null; }
  get lastChild() { return this.childNodes[this.childNodes.length - 1] || null; }
  get lastElementChild() { const c = this.children; return c[c.length - 1] || null; }
  get previousElementSibling() {
    if (!this.parentNode) return null;
    const s = this.parentNode.children;
    return s[s.indexOf(this) - 1] || null;
  }
  get classList() { const s = this.classes; return { contains: c => s.has(c), add: c => s.add(c), remove: c => s.delete(c) }; }
  get textContent() { return this.childNodes.map(c => c.textContent).join(""); }
  set textContent(t) { this.childNodes.forEach(c => { c.parentNode = null; }); this.childNodes = []; if (t) this.appendChild(new Text(t)); }
  getAttribute(k) { return k in this.attrs ? this.attrs[k] : null; }
  setAttribute(k, v) { this.attrs[k] = String(v); }
  removeAttribute(k) { delete this.attrs[k]; }
  hasAttribute(k) { return k in this.attrs; }
  get id() { return this.attrs.id || ""; }
  appendChild(c) { if (c.parentNode) c.parentNode.removeChild(c); this.childNodes.push(c); c.parentNode = this; return c; }
  removeChild(c) { const i = this.childNodes.indexOf(c); if (i < 0) throw new Error("removeChild: 자식 아님"); this.childNodes.splice(i, 1); c.parentNode = null; return c; }
  replaceChild(n, o) { if (n.parentNode) n.parentNode.removeChild(n); const i = this.childNodes.indexOf(o); if (i < 0) throw new Error("replaceChild: 자식 아님"); this.childNodes[i] = n; n.parentNode = this; o.parentNode = null; return o; }
  insertBefore(n, ref) { if (n.parentNode) n.parentNode.removeChild(n); const i = this.childNodes.indexOf(ref); this.childNodes.splice(i < 0 ? this.childNodes.length : i, 0, n); n.parentNode = this; return n; }
  remove() { if (this.parentNode) this.parentNode.removeChild(this); }
  closest(sel) { for (let n = this; n && n.nodeType === 1; n = n.parentNode) if (n.localName === sel) return n; return null; }
  querySelectorAll(sel) {
    const out = [];
    const walk = n => { for (const c of n.children) { if ((sel === "[data-w]" && c.hasAttribute("data-w")) || c.localName === sel) out.push(c); walk(c); } };
    walk(this);
    return out;
  }
  querySelector(sel) { return this.querySelectorAll(sel)[0] || null; }
  cloneNode(deep) {
    const c = new Elem(this.tagName); c.attrs = { ...this.attrs }; c.classes = new Set(this.classes);
    if (deep) for (const x of this.childNodes) c.appendChild(x.cloneNode(true));
    return c;
  }
  // 표
  get tBodies() { return this.children.filter(c => c.tagName === "TBODY"); }
  get tHead() { return this.children.find(c => c.tagName === "THEAD") || null; }
  get tFoot() { return this.children.find(c => c.tagName === "TFOOT") || null; }
  get caption() { return this.children.find(c => c.tagName === "CAPTION") || null; }
  get rows() { return this.tagName === "TABLE" ? this.children.flatMap(s => s.children.filter(r => r.tagName === "TR")) : this.children.filter(r => r.tagName === "TR"); }
  get cells() { return this.children.filter(c => c.tagName === "TD" || c.tagName === "TH"); }
  get rowSpan() { return parseInt(this.attrs.rowspan || "1", 10); }
  getBoundingClientRect() { const top = topOf(this), h = heightOf(this); return { top, bottom: top + h, height: h }; }
}

// 글 높이: 이어진 글자는 CPL 글자마다 한 줄, data-h 가 있는 요소(큰 수식·블록)는 그 높이로 줄을 끊는다.
function flowHeight(el) {
  let h = 0, chars = 0;
  const flush = () => { if (chars > 0) h += Math.ceil(chars / CPL) * LH; chars = 0; };
  const walk = n => {
    for (const c of n.childNodes) {
      if (c.nodeType === 3) { chars += c.data.length; continue; }
      if (c.hasAttribute("data-h")) { flush(); h += Number(c.getAttribute("data-h")); continue; }
      walk(c);
    }
  };
  walk(el);
  flush();
  return h;
}
function heightOf(el) {
  switch (el.tagName) {
    case "TABLE": case "THEAD": case "TBODY": case "TFOOT": case "BODY": return el.children.reduce((a, c) => a + heightOf(c), 0);
    case "TR": return Math.max(LH + 2 * PAD, ...el.cells.map(c => flowHeight(c) + 2 * PAD));
    case "TD": case "TH": return heightOf(el.parentNode);
    default: return el.hasAttribute("data-h") ? Number(el.getAttribute("data-h")) : flowHeight(el);
  }
}
function topOf(el) {
  const p = el.parentNode;
  if (!p) return 0;
  if (el.tagName === "TD" || el.tagName === "TH") return topOf(p);
  if (p.tagName === "TD" || p.tagName === "TH") return topOf(p) + PAD; // 칸 안(cellContent 의 감싸개)
  let t = p.isPage ? 0 : topOf(p);
  for (const s of p.children) { if (s === el) break; t += heightOf(s); }
  return t;
}

global.document = {
  createElement: t => new Elem(t),
  createTextNode: t => new Text(t),
  createTreeWalker(root, what, filter) {
    const list = [];
    const visit = n => {
      for (const c of n.childNodes) {
        if (c.nodeType === 3) {
          if (!(what & 4)) continue;
          const r = filter ? filter.acceptNode(c) : 1;
          if (r === 1) list.push(c);
          continue;
        }
        if (what & 1) {
          const r = filter ? filter.acceptNode(c) : 3;
          if (r === 1) list.push(c);
          if (r === 2) continue;
        }
        visit(c);
      }
    };
    visit(root);
    let i = -1;
    return { nextNode() { i++; this.currentNode = list[i]; return i < list.length ? list[i] : null; }, currentNode: null };
  },
};
global.NodeFilter = { SHOW_TEXT: 4, SHOW_ELEMENT: 1, FILTER_ACCEPT: 1, FILTER_REJECT: 2, FILTER_SKIP: 3 };
global.getComputedStyle = el => ({ lineHeight: String(LH), fontSize: "8", paddingTop: el.tagName === "TD" || el.tagName === "TH" ? String(PAD) : "0", paddingBottom: el.tagName === "TD" || el.tagName === "TH" ? String(PAD) : "0", borderTopWidth: "0", borderBottomWidth: "0" });
global.performance = { now: () => 0 };

const P = Function(part + `
return { fit, forceFit,
  set(b, h) { body = b; bodyBottom = h; bodyHeight = h; },
  setOpts(o) { opts = o; },
  reset() { overflow = 0; },
  get overflow() { return overflow; } };`)();

let failed = 0, cases = 0;
function check(name, ok, msg) { cases++; if (!ok) { failed++; console.log("FAIL", name, msg === undefined ? "" : msg); } }

const el = (tag, attrs, ...kids) => {
  const e = new Elem(tag);
  for (const [k, v] of Object.entries(attrs || {})) { if (k === "class") v.split(" ").forEach(x => e.classes.add(x)); else e.setAttribute(k, v); }
  for (const k of kids) e.appendChild(typeof k === "string" ? new Text(k) : k);
  return e;
};
// table(머리, 행들[, caption]) — 행은 칸 글(문자열) 또는 Elem 의 배열
function table(headCells, rows, caption) {
  const t = el("table", { "data-line": "1" });
  if (caption) t.appendChild(el("caption", null, caption));
  t.appendChild(el("thead", null, el("tr", null, ...headCells.map(h => el("th", null, h)))));
  const tb = t.appendChild(el("tbody"));
  rows.forEach((r, i) => tb.appendChild(el("tr", { id: "r" + i }, ...r.map(c => el("td", null, c)))));
  return t;
}
const words = (n, tag) => Array.from({ length: n }, (_, i) => tag + i).join(" ");

// run 은 paginate 의 차례(fit → 빈 쪽이 아니면 다음 쪽 → 빈 쪽이면 forceFit)를 흉내 내 쪽 본문들을 돌려준다. pre 는 첫 쪽에 먼저 놓을 블록.
// o 는 paginate 옵션이다 — 주지 않으면 공유 PDF 노트·메모 쪽처럼 splitTallRows 를 켠다(묶음 1~8), 묶음 10 은 {} 로 끈다.
function run(node, H, pre, o) {
  P.setOpts(o || { splitTallRows: true });
  const pages = [];
  const page = () => { const b = new Elem("body"); b.isPage = true; pages.push(b); P.set(b, H); return b; };
  P.reset();
  let body = page();
  if (pre) body.appendChild(pre);
  const queue = [node];
  let guard = 0;
  while (queue.length && guard++ < 2000) {
    const n = queue.shift();
    const empty = body.children.length === 0;
    const r = P.fit(n, body);
    if (r) { if (r.tail) { queue.unshift(r.tail); body = page(); } continue; }
    if (!empty) { queue.unshift(n); body = page(); continue; }
    const f = P.forceFit(n, body);
    if (f.tail) { queue.unshift(f.tail); body = page(); }
  }
  check("끝남", guard < 2000, guard);
  return pages;
}
const tablesOf = pages => pages.flatMap(p => p.querySelectorAll("table"));
const rowsOf = t => t.tBodies.flatMap(b => b.rows);
const norm = s => s.split(/\s+/).filter(Boolean);
const same = (a, b) => a.length === b.length && a.every((x, i) => x === b[i]);
const fitsAll = (pages, H) => pages.every(p => p.children.every(c => c.getBoundingClientRect().bottom <= H + 0.5));

// 1. 한 쪽보다 높은 칸: 쪽마다 나뉘고, 칸 글을 이으면 원래 글(낱말 유실·중복·중간 자름 없음), 이어지는 쪽마다 thead, 뒤 조각 행에 id 없음
{
  const long = words(400, "w") + " 끝표식";
  const t = table(["시간", "멘트"], [["1분", long], ["2분", "짧음"]]);
  const H = 300;
  const pages = run(t, H);
  const ts = tablesOf(pages);
  check("1 쪽 수", pages.length >= 3, pages.length);
  check("1 잘림 없음", P.overflow === 0, P.overflow);
  check("1 쪽 안", fitsAll(pages, H));
  check("1 thead 반복", ts.every(x => x.tHead && x.tHead.textContent === "시간멘트"), ts.map(x => !!x.tHead));
  const pieces = ts.flatMap(x => rowsOf(x)).filter(r => r.cells[0].textContent !== "2분");
  check("1 칸 글 보존", same(norm(pieces.map(r => r.cells[1].textContent).join(" ")), norm(long)), pieces.map(r => r.cells[1].textContent.slice(0, 20)));
  check("1 첫 칸은 앞 조각에만", pieces.map(r => r.cells[0].textContent).join("|") === "1분" + "|".repeat(pieces.length - 1), pieces.map(r => r.cells[0].textContent));
  check("1 뒤 조각 id 없음·cont", pieces.slice(1).every(r => !r.hasAttribute("id") && r.classList.contains("cont") && r.cells.every(c => c.classList.contains("cont"))));
  check("1 앞 조각 id", pieces[0].getAttribute("id") === "r0");
  check("1 짧은 행은 통째로 마지막에", rowsOf(ts[ts.length - 1]).some(r => r.textContent === "2분짧음"));
  check("1 자른 칸마다 2줄 이상", pieces.every(r => flowHeight(r.cells[1]) >= 2 * LH), pieces.map(r => flowHeight(r.cells[1])));
}
// 2. 쪽 중간(앞 블록 뒤, 앞 행 뒤)에서도 앞 조각을 그 쪽에 남긴다
{
  const long = words(300, "m");
  const t = table(["a", "b"], [["짧", "짧은 행"], ["길", long]]);
  const H = 300;
  const pages = run(t, H, el("p", null, words(20, "p")));
  const first = pages[0].querySelector("table");
  check("2 첫 쪽에 표", !!first);
  check("2 첫 쪽에 앞 행과 긴 행의 앞 조각", first && rowsOf(first).length === 2 && rowsOf(first)[1].textContent.startsWith("길m0 "), first && rowsOf(first).map(r => r.textContent.slice(0, 10)));
  check("2 잘림 없음", P.overflow === 0, P.overflow);
  check("2 쪽 안", fitsAll(pages, H));
  const pieces = tablesOf(pages).flatMap(rowsOf).filter(r => r.textContent !== "짧짧은 행");
  check("2 글 보존", same(norm(pieces.map(r => r.cells[1].textContent).join(" ")), norm(long)));
}
// 3. 한 쪽에 들어가는 행만 있는 표: 행 단위 그대로(나뉜 행 없음), 쪽마다 들어가는 만큼
{
  const rows = Array.from({ length: 30 }, (_, i) => ["행" + i, words(12, "c" + i + "_")]); // 칸마다 몇 줄
  const t = table(["a", "b"], rows);
  const H = 200;
  const pages = run(t, H);
  const ts = tablesOf(pages);
  check("3 나뉜 행 없음", ts.every(x => rowsOf(x).every(r => !r.classList.contains("cont"))));
  check("3 행 수 보존", ts.reduce((a, x) => a + rowsOf(x).length, 0) === 30);
  const rh = heightOf(t.tBodies[0] ? t.tBodies[0].rows[0] : rowsOf(ts[0])[0]);
  const per = Math.floor((H - (LH + 2 * PAD)) / heightOf(rowsOf(ts[0])[0]));
  check("3 쪽마다 들어가는 만큼(욕심)", ts.slice(0, -1).every(x => rowsOf(x).length === per), [per, rh, ts.map(x => rowsOf(x).length)]);
  check("3 잘림 없음", P.overflow === 0);
}
// 4. rowspan 이 걸린 높은 행은 지금처럼 잘린 채(overflow-clipped, overflow 증가), 글은 그 요소에 그대로
{
  const long = words(400, "s");
  const t = table(["a", "b"], [["x", long], ["y", "z"]]);
  t.tBodies[0].rows[0].cells[0].setAttribute("rowspan", "2");
  t.tBodies[0].rows[1].removeChild(t.tBodies[0].rows[1].cells[0]);
  const pages = run(t, 300);
  const all = pages.map(p => p.textContent).join("");
  check("4 잘림", P.overflow > 0, P.overflow);
  check("4 잘린 표시", pages.some(p => p.querySelectorAll("table").concat(p.children).some(x => x.classList.contains("overflow-clipped"))));
  check("4 글 보존", all.includes(long));
  check("4 나뉜 행 없음", tablesOf(pages).every(x => rowsOf(x).every(r => !r.classList.contains("cont"))));
}
// 5. 글 단위로 못 자르는 칸(한 쪽보다 큰 수식 하나, 블록을 품은 칸)은 지금처럼 잘린 채
for (const [name, cell] of [["수식", () => el("math", { "data-h": "1000" }, "x")], ["블록", () => el("div", { "data-h": "1000" }, "큰블록")]]) {
  const t = table(["a", "b"], [["x", ""], ["y", "z"]]);
  const td = t.tBodies[0].rows[0].cells[1];
  td.appendChild(cell());
  const pages = run(t, 300);
  check("5 " + name + " 잘림", P.overflow > 0, P.overflow);
  check("5 " + name + " 잘린 표시", pages.some(p => [...p.children, ...p.querySelectorAll("table")].some(x => x.classList.contains("overflow-clipped"))));
  check("5 " + name + " 나뉜 행 없음", tablesOf(pages).every(x => rowsOf(x).every(r => !r.classList.contains("cont"))));
}
// 6. 높은 칸이 둘: 둘 다 나뉘고 글 보존, 칸 수 유지
{
  const a = words(200, "a"), b = words(350, "b");
  const t = table(["a", "b", "c"], [[a, "가운데", b]]);
  const H = 250;
  const pages = run(t, H);
  const rs = tablesOf(pages).flatMap(rowsOf);
  check("6 잘림 없음", P.overflow === 0, P.overflow);
  check("6 쪽 안", fitsAll(pages, H));
  check("6 칸 수", rs.every(r => r.cells.length === 3));
  check("6 왼쪽 글 보존", same(norm(rs.map(r => r.cells[0].textContent).join(" ")), norm(a)));
  check("6 오른쪽 글 보존", same(norm(rs.map(r => r.cells[2].textContent).join(" ")), norm(b)));
  check("6 가운데는 앞 조각에만", rs.map(r => r.cells[1].textContent).join("|") === "가운데" + "|".repeat(rs.length - 1));
}
// 7. 캡션은 첫 조각에만, 공백 없는 긴 글자는 그 자리에서 끊되 글자 보존
{
  const long = "가".repeat(3000);
  const t = table(["a"], [[long]], "표 제목");
  const pages = run(t, 300);
  const ts = tablesOf(pages);
  check("7 캡션은 첫 조각에만", ts[0].caption && ts.slice(1).every(x => !x.caption));
  check("7 글자 보존", ts.flatMap(rowsOf).map(r => r.cells[0].textContent).join("") === long);
  check("7 잘림 없음", P.overflow === 0, P.overflow);
}
// 8. 행 둘의 합만 한 쪽을 넘고 둘째 행은 빈 쪽에 통째로 들어간다(머리 14 + 둘째 행 144 ≤ 200) — 첫 쪽에 앞 조각 두 줄 이상이 들어갈 자리가 있어도
//    나누지 않고 변경 전처럼 둘째 행을 통째로 다음 쪽에 둔다(1회차 리뷰 발견 1)
{
  const t = table(["h"], [["a".repeat(270)], ["b".repeat(420)]]);
  const pages = run(t, 200);
  const ts = tablesOf(pages);
  check("8 나뉜 행 없음", ts.every(x => rowsOf(x).every(r => !r.classList.contains("cont"))), ts.map(x => rowsOf(x).map(r => r.textContent.length)));
  check("8 첫 쪽은 첫 행만, 다음 쪽은 둘째 행 통째로", ts.length === 2 && rowsOf(ts[0]).map(r => r.textContent.length).join() === "270" && rowsOf(ts[1]).map(r => r.textContent.length).join() === "420",
    ts.map(x => rowsOf(x).map(r => r.textContent.length)));
  check("8 잘림 없음", P.overflow === 0, P.overflow);
}
// 10. 옵션(splitTallRows) 없이 부르면 높은 행을 나누지 않는다 — 변경 전(itda-doc 884c311)과 같이 넘친 행은 잘린 채(overflow-clipped), .cont 행 없음,
//     긴 칸 글은 그 잘린 요소에 그대로 있다. 묶음 1·2·7 과 같은 입력이다.
for (const [name, mk, pre] of [
  ["긴 칸", () => table(["시간", "멘트"], [["1분", words(400, "w") + " 끝표식"], ["2분", "짧음"]])],
  ["쪽 중간", () => table(["a", "b"], [["짧", "짧은 행"], ["길", words(300, "m")]]), () => el("p", null, words(20, "p"))],
  ["공백 없음", () => table(["a"], [["가".repeat(3000)]], "표 제목")],
]) {
  const t = mk();
  const all = t.textContent;
  const pages = run(t, 300, pre && pre(), {});
  check("10 " + name + " 나뉜 행 없음", tablesOf(pages).every(x => rowsOf(x).every(r => !r.classList.contains("cont"))), tablesOf(pages).map(x => rowsOf(x).map(r => r.textContent.length)));
  check("10 " + name + " 잘림", P.overflow > 0, P.overflow);
  check("10 " + name + " 잘린 표시", pages.some(p => [...p.children, ...p.querySelectorAll("table")].some(x => x.classList.contains("overflow-clipped"))));
  check("10 " + name + " 긴 칸 글 그대로", pages.map(p => p.textContent).join("").replace(/\s+/g, "").includes(all.replace(/\s+/g, "").slice(-200)));
}
console.log(`PAGERROW DONE cases=${cases} failed=${failed}`);
