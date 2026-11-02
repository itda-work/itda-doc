// pager.js 의 splitPre 를 최소 DOM 대역으로 돌린다(pager_test.go). 인자: splitPre 함수 본문을 담은 파일.
// 한 쪽에 3줄만 들어가게 해 12줄 블록을 거듭 나누고, 조각마다 data-line(첫 내용 줄)·줄마다 span 의 data-line 을 본다.
"use strict";
const fs = require("fs");
const body = fs.readFileSync(process.argv[2], "utf8");

class Text {
  constructor(t) { this.nodeType = 3; this.data = t; }
  get textContent() { return this.data; }
}
class Elem {
  constructor(tag) { this.nodeType = 1; this.tagName = tag.toUpperCase(); this.attrs = {}; this.children = []; this.classes = new Set(); }
  get childNodes() { return this.children; }
  get classList() { const s = this.classes; return { contains: c => s.has(c), add: c => s.add(c) }; }
  get textContent() { return this.children.map(c => c.textContent).join(""); }
  set textContent(t) { this.children = [new Text(t)]; }
  getAttribute(k) { return k in this.attrs ? this.attrs[k] : null; }
  setAttribute(k, v) { this.attrs[k] = String(v); }
  removeAttribute(k) { delete this.attrs[k]; }
  hasAttribute(k) { return k in this.attrs; }
  appendChild(c) { this.children.push(c); return c; }
  removeChild(c) { this.children.splice(this.children.indexOf(c), 1); return c; }
  querySelector(sel) { return this.children.find(c => c.nodeType === 1 && c.tagName === sel.toUpperCase()) || null; }
  cloneNode(deep) {
    const c = new Elem(this.tagName); c.attrs = { ...this.attrs }; c.classes = new Set(this.classes);
    if (deep) c.children = this.children.map(x => x.nodeType === 3 ? new Text(x.data) : x.cloneNode(true));
    return c;
  }
}
global.document = { createElement: t => new Elem(t), createTextNode: t => new Text(t) };

const page = new Elem("div");
const lineCount = el => el.textContent.replace(/\n$/, "").split("\n").length;
const fits = () => lineCount(page.children[page.children.length - 1]) <= 3;
const shallow = (el, cont) => { const c = el.cloneNode(false); if (cont) { c.removeAttribute("id"); c.classList.add("cont"); } return c; };
const splitPre = Function("shallow", "fits", "parent", body + "\nreturn splitPre;")(shallow, fits);

let failed = 0, cases = 0;
function check(name, ok, msg) { cases++; if (!ok) { failed++; console.log("FAIL", name, msg); } }

// pieces 는 블록을 끝까지 나눈 조각들이다(앞 조각은 쪽에 넣고 뒷조각을 다시 나눈다)
function pieces(pre) {
  const out = [];
  for (let n = pre; n;) {
    page.children = [];
    const r = splitPre(n, page);
    if (!r) { out.push(n); break; }
    out.push(r.head);
    n = r.tail;
  }
  return out;
}
const lines = Array.from({ length: 12 }, (_, i) => "원문 " + (i + 1));

// 원문 블록(.raw-source): data-line 은 첫 내용 줄, 줄마다 span
{
  const pre = new Elem("pre"); pre.classList.add("raw-source"); pre.setAttribute("data-line", "5");
  const code = pre.appendChild(new Elem("code"));
  lines.forEach((t, i) => { const s = code.appendChild(new Elem("span")); s.setAttribute("data-line", String(5 + i)); s.textContent = t; code.appendChild(new Text("\n")); });
  const ps = pieces(pre);
  check("raw 조각 수", ps.length >= 4, ps.length);
  for (const p of ps) {
    const first = lines.indexOf(p.textContent.split("\n")[0]);
    check("raw data-line", p.getAttribute("data-line") === String(5 + first), `${p.textContent.split("\n")[0]} → ${p.getAttribute("data-line")}`);
    const spans = p.querySelector("code").children.filter(c => c.nodeType === 1);
    check("raw span 수", spans.length === lineCount(p), spans.length);
    for (const s of spans) check("raw span", s.getAttribute("data-line") === String(5 + lines.indexOf(s.textContent)), `${s.textContent} → ${s.getAttribute("data-line")}`);
  }
}
// 코드 블록: data-line 은 여는 펜스 줄, 내용은 그 다음 줄부터 — 뒷조각을 다시 나눠도 펜스 몫을 또 더하지 않는다
{
  const pre = new Elem("pre"); pre.setAttribute("data-line", "10");
  const code = pre.appendChild(new Elem("code")); code.textContent = lines.join("\n") + "\n";
  const ps = pieces(pre);
  check("code 조각 수", ps.length >= 4, ps.length);
  ps.forEach((p, k) => {
    const first = lines.indexOf(p.textContent.split("\n")[0]);
    const want = k === 0 ? "10" : String(11 + first);
    check("code data-line", p.getAttribute("data-line") === want, `${p.textContent.split("\n")[0]} → ${p.getAttribute("data-line")} (기대 ${want})`);
  });
}
// 강조 span 이 줄 경계를 품어도 양쪽 조각에서 클래스와 글자 순서를 지킨다.
{
  const pre = new Elem("pre"); pre.setAttribute("data-line", "20");
  const code = pre.appendChild(new Elem("code"));
  const kw = code.appendChild(new Elem("span")); kw.setAttribute("class", "hl-k"); kw.textContent = "func ";
  const body = code.appendChild(new Elem("span")); body.setAttribute("class", "hl-nx"); body.textContent = lines.join("\n") + "\n";
  const original = code.textContent;
  const ps = pieces(pre);
  check("highlight 조각 수", ps.length >= 4, ps.length);
  check("highlight 글 보존", ps.map(p => p.textContent).join("") === original, ps.map(p => p.textContent).join(""));
  ps.forEach((p, i) => {
    const spans = p.querySelector("code").children.filter(c => c.nodeType === 1 && c.getAttribute("class")?.startsWith("hl-"));
    check("highlight 양쪽 span " + i, spans.length > 0, spans.length);
  });
}
console.log(`PAGER DONE cases=${cases} failed=${failed}`);
