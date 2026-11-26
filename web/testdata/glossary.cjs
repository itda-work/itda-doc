// 게시 강의 용어사전(glossary.js — itda-doc#135, itda-hyve#1084)의 맞춤 규칙·긋는 곳·팝업·여는 곳을 가짜 DOM 에서 확인한다.
"use strict";
const fs = require("fs");
const vm = require("vm");
const [glossPath, askPath, ...sharePaths] = process.argv.slice(2); // sharePaths: pager.js·share.js — 팝업 안 키가 실제 전역 keyHandler 로 가지 않는지
let cases = 0, failed = 0;
function ok(name, cond, detail) {
  cases++;
  if (!cond) { failed++; console.log("실패 " + name + " — " + JSON.stringify(detail === undefined ? null : detail)); } else console.log("통과 " + name);
}
const eq = (name, got, want) => ok(name, JSON.stringify(got) === JSON.stringify(want), { got, want });

// ---- 가짜 DOM(필요한 만큼): 요소·글 마디, 간단한 HTML 읽기·쓰기, closest·contains 는 .클래스·#id·태그 단순 선택자 쉼표 목록
class Text {
  constructor(t) { this.nodeType = 3; this.data = t; this.parentNode = null; }
  get textContent() { return this.data; }
}
const VOID = new Set(["br", "hr", "img", "input"]);
function simpleMatch(e, sel) {
  return sel.split(",").some(s => {
    s = s.trim();
    if (s.startsWith(".")) return (e.getAttribute("class") || "").split(/\s+/).includes(s.slice(1));
    if (s.startsWith("#")) return e.id === s.slice(1);
    return e.tagName === s.toUpperCase();
  });
}
class El {
  constructor(tag, doc) { this.nodeType = 1; this.tagName = tag.toUpperCase(); this.attrs = new Map(); this.childNodes = []; this.parentNode = null; this.ownerDocument = doc; this.style = {}; this.focused = 0; this.clicks = 0; }
  get children() { return this.childNodes.filter(n => n.nodeType === 1); }
  get id() { return this.attrs.get("id") || ""; }
  set id(v) { this.attrs.set("id", v); }
  get type() { return this.attrs.get("type") || ""; }
  set type(v) { this.attrs.set("type", v); }
  get hidden() { return this.attrs.has("hidden"); }
  set hidden(v) { if (v) this.attrs.set("hidden", ""); else this.attrs.delete("hidden"); }
  get isConnected() { let n = this; while (n.parentNode) n = n.parentNode; return n === this.ownerDocument.root; }
  get offsetWidth() { return this.hidden ? 0 : 300; }
  // 팝업 높이는 메뉴가 펴졌으면 +140(메뉴 셋) — 창 높이(doc.vh)가 있으면 CSS max-height(창 - 16)처럼 자른다
  get offsetHeight() {
    if (this.hidden) return 0;
    let h = 200;
    if ((this.getAttribute("class") || "") === "itda-gloss-pop") { const m = this.children.find(c => c.getAttribute("class") === "ig-ask"); const menu = m && m.children.find(c => c.getAttribute("class") === "ig-ask-menu"); if (menu && !menu.hidden) h += 140; }
    return this.ownerDocument.vh ? Math.min(h, this.ownerDocument.vh - 16) : h;
  }
  getAttribute(k) { return this.attrs.has(k) ? this.attrs.get(k) : null; }
  setAttribute(k, v) { this.attrs.set(k, String(v)); }
  removeAttribute(k) { this.attrs.delete(k); }
  hasAttribute(k) { return this.attrs.has(k); }
  appendChild(n) { if (n.parentNode) n.parentNode.removeChild(n); n.parentNode = this; this.childNodes.push(n); return n; }
  insertBefore(n, ref) { if (n.parentNode) n.parentNode.removeChild(n); const i = this.childNodes.indexOf(ref); n.parentNode = this; if (i < 0) this.childNodes.push(n); else this.childNodes.splice(i, 0, n); return n; }
  removeChild(n) { const i = this.childNodes.indexOf(n); if (i >= 0) this.childNodes.splice(i, 1); n.parentNode = null; return n; }
  remove() { if (this.parentNode) this.parentNode.removeChild(this); }
  set textContent(t) { this.childNodes = []; if (t) this.appendChild(new Text(t)); }
  get textContent() { return this.childNodes.map(n => n.textContent).join(""); }
  closest(sel) { let e = this; while (e && e.nodeType === 1) { if (simpleMatch(e, sel)) return e; e = e.parentNode; } return null; }
  contains(n) { while (n) { if (n === this) return true; n = n.parentNode; } return false; }
  focus() { this.focused++; this.ownerDocument.activeElement = this; }
  click() { this.clicks++; if (this.ownerDocument.onclick) this.ownerDocument.onclick({ target: this, preventDefault() { this.prevented = true; }, stopPropagation() {} }); }
  getBoundingClientRect() { return this.rect || { left: 100, top: 100, right: 160, bottom: 120, width: 60, height: 20 }; }
  get innerHTML() { return this.childNodes.map(ser).join(""); }
  set innerHTML(h) { this.childNodes = []; parseInto(this, h, this.ownerDocument); }
}
function ser(n) {
  if (n.nodeType === 3) return n.data.replace(/&/g, "&amp;").replace(/</g, "&lt;");
  const t = n.tagName.toLowerCase();
  const a = [...n.attrs].map(([k, v]) => v === "" ? " " + k : " " + k + '="' + v + '"').join("");
  return "<" + t + a + ">" + (VOID.has(t) ? "" : n.childNodes.map(ser).join("") + "</" + t + ">");
}
function parseInto(rootEl, html, doc) {
  const re = /<\/([a-zA-Z][\w-]*)\s*>|<([a-zA-Z][\w-]*)((?:\s+[\w:-]+(?:="[^"]*")?)*)\s*\/?>|([^<]+)/g;
  const stack = [rootEl];
  let m;
  while ((m = re.exec(html))) {
    const top = stack[stack.length - 1];
    if (m[1]) { stack.pop(); continue; }
    if (m[2]) {
      const e = new El(m[2], doc);
      for (const am of (m[3] || "").matchAll(/([\w:-]+)(?:="([^"]*)")?/g)) e.attrs.set(am[1], am[2] === undefined ? "" : am[2]);
      top.appendChild(e);
      if (!VOID.has(m[2].toLowerCase())) stack.push(e);
      continue;
    }
    top.appendChild(new Text(m[4].replace(/&lt;/g, "<").replace(/&amp;/g, "&")));
  }
}
function makeDoc() {
  const doc = { activeElement: null, onclick: null };
  doc.root = new El("html", doc);
  doc.body = doc.root.appendChild(new El("body", doc));
  doc.createElement = t => new El(t, doc);
  doc.createTextNode = t => new Text(t);
  return doc;
}

const ctx = { window: undefined, globalThis: {}, console, URL };
ctx.globalThis = ctx;
vm.createContext(ctx);
vm.runInContext(fs.readFileSync(askPath, "utf8"), ctx);
vm.runInContext(fs.readFileSync(glossPath, "utf8"), ctx);
const G = ctx.ItdaGlossary, A = ctx.ItdaAsk;
ok("내보냄", !!G && !!A && typeof A.termQuestion === "function");

const DATA = { format: 1, terms: [
  { term: "대화", aliases: [], def: "사람과 Claude 가 주고받는 글." },
  { term: "Claude", aliases: [], def: "Anthropic 의 AI." },
  { term: "Claude Desktop", aliases: ["데스크톱 앱"], def: "PC 에 설치하는 Claude 앱.", see: { doc: "02-setup", hash: "설치" } },
  { term: "스킬", aliases: ["Skill"], def: "Claude 가 일을 하는 방법을 적은 묶음.\n둘째 줄.", see: { doc: "01-intro", hash: "" } },
  { term: "MCP", aliases: [], def: "도구를 잇는 규약 <b>&</b>." },
] };
const g = G.index(DATA);
ok("색인", !!g && g.terms.length === 5);

// ---- 맞춤 규칙(scan) — 표
const scanCases = [
  // [이름, 글, 앞, 뒤, 기대 표기들]
  ["조사가 붙은 한글 표기", "대화를 시작한다", "", "", ["대화"]],
  ["한글로 끝나는 표기는 뒤 경계 없음(받아들인 한계)", "대화상자를 연다", "", "", ["대화"]],
  ["앞 글자가 한글이면 아님", "전화대화", "", "", []],
  ["긴 표기 먼저", "Claude Desktop 을 켠다", "", "", ["Claude Desktop"]],
  ["짧은 표기도 따로", "Claude 와 Claude Desktop", "", "", ["Claude", "Claude Desktop"]],
  ["영문 표기 뒤가 영문이면 아님", "Skills 폴더", "", "", []],
  ["영문 표기 뒤 조사", "Skill을 만든다", "", "", ["Skill"]],
  ["영문 표기 뒤 숫자면 아님", "MCP2 는 없다", "", "", []],
  ["영문 표기 앞이 영문이면 아님", "xMCP", "", "", []],
  ["괄호 안", "(MCP)", "", "", ["MCP"]],
  ["대소문자 구분", "skill 과 mcp", "", "", []],
  ["앞 글자가 숫자면 아님", "3대화", "", "", []],
  ["이웃 마디의 앞 글자(한글)", "대화", "전", "", []],
  ["이웃 마디의 뒤 글자(영문)", "Skill", "", "s", []],
  ["이웃 마디의 뒤 글자(한글 조사)", "Skill", "", "을", ["Skill"]],
  ["다른 표기", "데스크톱 앱을 연다", "", "", ["데스크톱 앱"]],
  ["여러 번 나오면 모두(쪽 처음 거르기는 markTree)", "스킬과 스킬", "", "", ["스킬", "스킬"]],
];
for (const [name, text, b, a, want] of scanCases) eq("scan: " + name, G.scan(text, g, b, a).map(h => text.slice(h.start, h.end)), want);

// ---- 긋는 곳(markPages) — 표: [이름, 꼴, #pages 안 HTML, #notes 안 HTML, 기대 밑줄 글자들]
const markCases = [
  ["코드·pre·kbd 는 뺀다", "doc", '<div class="page"><div class="page-body"><p><code>스킬</code> <kbd>MCP</kbd></p><pre><code>스킬</code></pre></div></div>', "", []],
  ["제목은 뺀다", "doc", '<div class="page"><div class="page-body"><h2 id="a">스킬</h2></div></div>', "", []],
  ["링크는 뺀다", "doc", '<div class="page"><div class="page-body"><p><a href="#x">스킬</a></p></div></div>', "", []],
  ["단추·입력·라벨은 뺀다", "doc", '<div class="page"><div class="page-body"><p><button>스킬</button><label>스킬</label></p></div></div>', "", []],
  ["수식·그림은 뺀다", "doc", '<div class="page"><div class="page-body"><p><math><mi>MCP</mi></math></p><div class="mermaid-fig"><svg><text>스킬</text></svg></div><p class="math-display">MCP</p></div></div>', "", []],
  ["이미 밑줄 친 곳은 뺀다", "doc", '<div class="page"><div class="page-body"><p><u>스킬</u> <abbr>MCP</abbr></p></div></div>', "", []],
  ["쪽 머리·꼬리말·숨긴 것은 뺀다", "doc", '<div class="page"><div class="page-header">스킬</div><div class="page-body"><p><span aria-hidden="true">MCP</span></p></div><div class="page-footer">대화</div></div>', "", []],
  ["구역마다 처음 한 번", "doc", '<div class="page"><div class="page-body"><p>스킬 하나</p><p>스킬 둘</p><h2 id="b">다음</h2><p>스킬 셋</p></div></div>', "", ["스킬", "스킬"]],
  ["쪽을 넘어도 구역이 같으면 한 번", "doc", '<div class="page"><div class="page-body"><p>스킬</p></div></div><div class="page"><div class="page-body"><p>스킬</p></div></div>', "", ["스킬"]],
  ["다른 표기도 같은 용어로 센다", "doc", '<div class="page"><div class="page-body"><p>스킬과 Skill</p></div></div>', "", ["스킬"]],
  ["강조 속 + 뒤 조사", "doc", '<div class="page"><div class="page-body"><p><strong>스킬</strong>을 쓴다</p></div></div>', "", ["스킬"]],
  ["앞 마디가 영문이면 아님", "doc", '<div class="page"><div class="page-body"><p>A<strong>스킬</strong></p></div></div>', "", []],
  ["뒤 마디가 영문이면 아님", "doc", '<div class="page"><div class="page-body"><p><b>Skill</b>s</p></div></div>', "", []],
  ["덩어리가 다르면 이웃이 아님", "doc", '<div class="page"><div class="page-body"><p>A</p><p>스킬</p></div></div>', "", ["스킬"]],
  ["슬라이드마다 처음 한 번", "slides", '<div class="page"><div class="slide-head"><h2>스킬</h2></div><p>스킬 스킬</p></div><div class="page"><p>스킬</p></div>', "", ["스킬", "스킬"]],
  ["슬라이드의 제목은 구역을 나누지 않는다", "slides", '<div class="page"><p>MCP</p><h3>소제목</h3><p>MCP</p></div>', "", ["MCP"]],
  ["노트는 그 장에 함께 센다", "slides", '<div class="page"><p>스킬</p></div><div class="page"><p>없음</p></div>',
    '<aside class="slide-note" data-slide="1"><div class="sn-body"><p>스킬 MCP</p></div></aside><aside class="slide-note" data-slide="2"><div class="sn-body"><p>스킬</p></div></aside>', ["스킬", "MCP", "스킬"]],
  ["노트 없음 칸은 뺀다", "slides", '<div class="page"><p>x</p></div>', '<aside class="slide-note sn-empty" data-slide="1"><p class="sn-none">스킬</p></aside>', []],
  ["슬라이드 꼬리·표지는 뺀다", "slides", '<div class="page"><div class="slide-cover"><p>스킬</p></div><div class="slide-foot k-a"><span>MCP</span></div></div>', "", []],
];
function terms(el) {
  const out = [];
  (function walk(n) { for (const c of n.childNodes) if (c.nodeType === 1) { if ((c.getAttribute("class") || "").split(" ").includes("itda-term")) out.push(c.textContent); walk(c); } })(el);
  return out;
}
for (const [name, look, pagesHTML, notesHTML, want] of markCases) {
  const doc = makeDoc();
  const pages = doc.body.appendChild(doc.createElement("div")), notes = doc.body.appendChild(doc.createElement("div"));
  pages.innerHTML = pagesHTML;
  notes.innerHTML = notesHTML;
  const before = pages.textContent + "|" + notes.textContent;
  const n = G.markPages(pages, notes, g, doc, look);
  eq("mark: " + name, terms(pages).concat(terms(notes)), want);
  ok("mark: " + name + " — 단 개수", n === want.length, n);
  ok("mark: " + name + " — 글자 그대로", pages.textContent + "|" + notes.textContent === before);
  const once = pages.innerHTML + notes.innerHTML;
  const n2 = G.markPages(pages, notes, g, doc, look);
  ok("mark: " + name + " — 다시 그어도 한 번", n2 === 0 && pages.innerHTML + notes.innerHTML === once, { n2 });
}

// 밑줄의 꼴: 단추 역할·키보드·용어 번호
{
  const doc = makeDoc();
  const pages = doc.body.appendChild(doc.createElement("div"));
  pages.innerHTML = '<div class="page"><p>Skill을</p></div>';
  G.markPages(pages, null, g, doc, "doc");
  eq("밑줄 꼴", pages.innerHTML, '<div class="page"><p><span class="itda-term" data-term="3" role="button" tabindex="0" aria-haspopup="dialog" aria-expanded="false">Skill</span>을</p></div>');
  // 노트 칸을 다시 만든 뒤(placeNotes) 다시 그어도 본문의 밑줄은 겹치지 않는다
  const notes = doc.body.appendChild(doc.createElement("div"));
  notes.innerHTML = '<aside class="slide-note" data-slide="1"><div class="sn-body"><p>스킬 대화</p></div></aside>';
  G.markPages(pages, notes, g, doc, "slides");
  eq("노트만 다시 그음", terms(pages).concat(terms(notes)), ["Skill", "대화"]);
}

// ---- 색인 꼴(index)
const bad = [
  ["형식 번호", { format: 2, terms: DATA.terms }],
  ["용어 없음", { format: 1, terms: [] }],
  ["term 없음", { format: 1, terms: [{ def: "x" }] }],
  ["def 가 글이 아님", { format: 1, terms: [{ term: "a", def: 3 }] }],
  ["aliases 가 목록이 아님", { format: 1, terms: [{ term: "a", aliases: "b", def: "x" }] }],
  ["see 꼴", { format: 1, terms: [{ term: "a", def: "x", see: { doc: "", hash: "" } }] }],
];
for (const [name, d] of bad) ok("index 거부: " + name, G.index(d) === null);
ok("index 거부: null", G.index(null) === null);
{
  const g2 = G.index({ format: 1, terms: [{ term: "a", def: "x" }, { term: "b", aliases: ["a"], def: "y" }] });
  eq("index: 같은 표기는 앞 용어", G.scan("a", g2, "", "").map(h => h.t), [0]);
}

// ---- 자리(place): 아래, 자리가 없으면 위, 화면 밖으로 나가지 않는다
eq("place 아래", G.place({ left: 100, top: 100, bottom: 120 }, 300, 200, 1000, 800), { left: 100, top: 126 });
eq("place 위", G.place({ left: 100, top: 700, bottom: 720 }, 300, 200, 1000, 800), { left: 100, top: 494 });
eq("place 오른쪽 끝", G.place({ left: 900, top: 100, bottom: 120 }, 300, 200, 1000, 800), { left: 692, top: 126 });
eq("place 좁은 화면", G.place({ left: 10, top: 150, bottom: 170 }, 344, 500, 360, 640), { left: 8, top: 132 });

// ---- 여는 곳(이 브라우저의 localStorage 하나)
function mem() { const m = new Map(); return { getItem: k => m.has(k) ? m.get(k) : null, setItem: (k, v) => m.set(k, String(v)), m }; }
{
  const s = mem();
  ok("여는 곳 기본 웹", G.getMode(s) === "web");
  G.setMode(s, "app");
  ok("여는 곳 기억", G.getMode(s) === "app" && s.m.get("itda-glossary-open") === "app" && s.m.size === 1);
  G.setMode(s, "chatgpt");
  ok("모르는 값은 안 적음", G.getMode(s) === "app");
  s.m.set("itda-glossary-open", "zzz");
  ok("틀린 값은 웹", G.getMode(s) === "web");
  const thrower = { getItem() { throw new Error("x"); }, setItem() { throw new Error("x"); } };
  ok("못 쓰는 저장소", G.getMode(thrower) === "web" && (G.setMode(thrower, "app"), true));
}
eq("주소 웹", G.askURL("web", "가 b&c"), "https://claude.ai/new?q=" + encodeURIComponent("가 b&c"));
eq("주소 앱", G.askURL("app", "가"), "claude://claude.ai/new?q=%EA%B0%80");
ok("주소 복사는 없음", G.askURL("copy", "가") === "");

// ---- 지침 글(ask.js termQuestion) — 강의 이름·용어·MCP 주소·연결 안내만
const cfgBoth = A.normalize({ services: ["claude", "chatgpt"], course: "Claude 입문", doc: "1장", mcp: "https://docs.itda.work/claude-igm/mcp", doc_id: "01", guide: "https://docs.itda.work/claude-igm/mcp" });
const cfgGPT = A.normalize({ services: ["chatgpt"], course: "c", mcp: "https://docs.itda.work/x/mcp", doc_id: "01" });
ok("claudeOn", G.claudeOn(cfgBoth) && !G.claudeOn(cfgGPT) && !G.claudeOn(null));
{
  const q = A.termQuestion(cfgBoth, "스킬");
  eq("지침 글", q, "「Claude 입문」 강의 MCP(https://docs.itda.work/claude-igm/mcp)로 용어 '스킬'의 뜻을 처음 배우는 사람에게 비유와 함께 설명하고, 이해했는지 질문 하나로 확인해줘.\n" +
    "커넥터가 연결돼 있지 않으면 이 안내를 보여 주세요. https://docs.itda.work/claude-igm/mcp");
  ok("지침 글에 풀이 없음", !q.includes("묶음"));
}

// ---- 팝업
function popupRig(cfg, storage, size, over) {
  const doc = makeDoc();
  const pages = doc.body.appendChild(doc.createElement("div"));
  pages.id = "pages";
  pages.innerHTML = '<div class="page"><p>Skill을 쓰고 Claude Desktop 을 켠다. MCP 도.</p></div>';
  G.markPages(pages, null, g, doc, "doc");
  const log = { jump: [], copy: [], toast: [] };
  const win = { innerWidth: (size || {}).w || 1000, innerHeight: (size || {}).h || 800, getSelection: () => ({ isCollapsed: true }) };
  const p = G.popup({ doc, win, g, ask: () => cfg, question: A.termQuestion, storage: () => storage,
    seeHref: see => see.doc === "01-intro" ? { href: "#" + encodeURIComponent(see.hash), same: true } : { href: "../" + see.doc + "/#" + encodeURIComponent(see.hash), same: false },
    jump: h => log.jump.push(h), copy: t => { log.copy.push(t); return Promise.resolve(true); }, toast: m => log.toast.push(m), ...(over || {}) });
  doc.onclick = e => p.onClick(e); // 메뉴 링크를 대신 누르면(click()) 같은 처리기로 온다
  const spans = []; (function walk(n) { for (const c of n.childNodes) if (c.nodeType === 1) { if (c.getAttribute("class") === "itda-term") spans.push(c); walk(c); } })(pages);
  const click = t => { const e = { target: t, prevented: false, preventDefault() { this.prevented = true; }, stopPropagation() {} }; const r = p.onClick(e); return { r, e }; };
  const find = cls => { let f = null; (function walk(n) { for (const c of n.childNodes) if (c.nodeType === 1) { if (!f && (c.getAttribute("class") || "").split(" ").includes(cls)) f = c; walk(c); } })(doc.body); return f; };
  return { doc, p, spans, click, find, log };
}
{
  const s = mem();
  const R = popupRig(cfgBoth, s);
  eq("팝업: 밑줄 셋", R.spans.map(x => x.textContent), ["Skill", "Claude Desktop", "MCP"]);
  ok("팝업: 처음엔 없음", !R.p.isOpen() && !R.find("itda-gloss-pop"));
  ok("팝업: 밑줄 누름을 다룸", R.click(R.spans[0]).r === true);
  const pop = R.find("itda-gloss-pop");
  ok("팝업: 열림", R.p.isOpen() && !!pop && !pop.hidden && pop.getAttribute("role") === "dialog");
  eq("팝업: 다른 표기로 눌러도 대표 용어", R.find("ig-term").textContent, "스킬");
  eq("팝업: 다른 표기 줄", R.find("ig-alias").textContent, "다른 표기: Skill");
  eq("팝업: 풀이(줄바꿈 그대로)", R.find("ig-def").textContent, "Claude 가 일을 하는 방법을 적은 묶음.\n둘째 줄.");
  ok("팝업: aria-expanded", R.spans[0].getAttribute("aria-expanded") === "true");
  eq("팝업: 자리", [R.p.el.style.left, R.p.el.style.top], ["100px", "126px"]);
  ok("팝업: 초점이 팝업으로", R.doc.activeElement === R.p.el);
  const web = R.find("ig-ask-item");
  const q = A.termQuestion(cfgBoth, "스킬");
  eq("팝업: 웹 링크", [web.tagName, web.getAttribute("href"), web.getAttribute("target"), web.getAttribute("rel")], ["A", "https://claude.ai/new?q=" + encodeURIComponent(q), "_blank", "noopener noreferrer"]);
  const items = R.find("ig-ask-menu").children;
  eq("팝업: 메뉴 셋", items.map(x => [x.textContent, x.getAttribute("data-mode")]), [["웹에서 열기", "web"], ["데스크톱 앱에서 열기", "app"], ["질문만 복사", "copy"]]);
  eq("팝업: 앱 링크", items[1].getAttribute("href"), "claude://claude.ai/new?q=" + encodeURIComponent(q));
  ok("팝업: 처음 기본값 웹 표시", items[0].getAttribute("aria-current") === "true" && items[1].getAttribute("aria-current") == null && R.find("ig-ask-go").getAttribute("data-mode") === "web");
  ok("팝업: 메뉴 처음엔 닫힘", R.find("ig-ask-menu").hidden);
  R.click(R.find("ig-ask-more"));
  ok("팝업: ▾ 로 메뉴 열기", !R.find("ig-ask-menu").hidden && R.find("ig-ask-more").getAttribute("aria-expanded") === "true");
  // Esc 는 메뉴를 먼저 닫고, 다시 Esc 면 팝업을 닫고 밑줄로 초점
  ok("Esc 1(메뉴)", R.p.onKey({ key: "Escape", target: R.p.el }) && R.find("ig-ask-menu").hidden && R.p.isOpen());
  R.click(R.find("ig-ask-go"));
  ok("단추: 기본(웹) 링크를 대신 누름", web.clicks === 1 && s.m.get("itda-glossary-open") === "web");
  R.click(R.find("ig-ask-more"));
  R.click(items[2]);
  ok("메뉴: 질문만 복사", R.log.copy.length === 1 && R.log.copy[0] === q && s.m.get("itda-glossary-open") === "copy");
  ok("메뉴: 고르면 메뉴 닫힘", R.find("ig-ask-menu").hidden);
  R.click(R.find("ig-ask-go"));
  ok("단추: 마지막에 고른 방식(복사)으로 바로", R.log.copy.length === 2 && web.clicks === 1);
  R.click(items[1]);
  ok("메뉴: 앱을 고르면 기억(링크 기본 동작은 둔다)", s.m.get("itda-glossary-open") === "app" && items[1].getAttribute("aria-current") === "true" && items[0].getAttribute("aria-current") == null);
  R.click(R.find("ig-ask-go"));
  ok("단추: 앱 링크를 대신 누름", items[1].clicks === 1);
  ok("Esc 2(팝업)", R.p.onKey({ key: "Escape", target: R.p.el }) && !R.p.isOpen() && R.doc.activeElement === R.spans[0] && R.spans[0].getAttribute("aria-expanded") === "false");
  // 키보드: Enter·Space 로 여닫기
  ok("Enter 로 열기", R.p.onKey({ key: "Enter", target: R.spans[2] }) && R.p.isOpen());
  eq("다른 용어", R.find("ig-term").textContent, "MCP");
  ok("자세히 없음이면 줄 없음", R.find("ig-see").hidden);
  eq("풀이의 HTML 기호는 글자 그대로", R.find("ig-def").textContent, "도구를 잇는 규약 <b>&</b>.");
  ok("같은 밑줄 Space 로 닫기", R.p.onKey({ key: " ", target: R.spans[2] }) && !R.p.isOpen());
  ok("다른 키는 다루지 않음", R.p.onKey({ key: "ArrowRight", target: R.spans[2] }) === "");
  // 자세히 다룬 곳 — 다른 교안은 링크, 같은 교안은 제자리 옮김
  R.click(R.spans[1]);
  eq("다른 교안 링크", R.find("ig-see-a").getAttribute("href"), "../02-setup/#" + encodeURIComponent("설치"));
  const other = R.click(R.find("ig-see-a"));
  ok("다른 교안: 기본 동작 두고 닫기", !other.e.prevented && !R.p.isOpen());
  R.click(R.spans[0]);
  const same = R.click(R.find("ig-see-a"));
  ok("같은 교안: 제자리로", same.e.prevented && R.log.jump.length === 1 && R.log.jump[0] === "#" && !R.p.isOpen());
  // 바깥 누름·초점 이동
  R.click(R.spans[0]);
  R.p.onPointer({ target: R.p.el });
  ok("팝업 안 누름은 그대로", R.p.isOpen());
  R.p.onPointer({ target: R.doc.body });
  ok("바깥 누름에 닫힘", !R.p.isOpen());
  R.click(R.spans[0]);
  R.p.onFocus({ target: R.doc.body });
  ok("초점이 밖으로 가면 닫힘", !R.p.isOpen());
  R.click(R.spans[0]);
  R.click(R.spans[0]);
  ok("같은 밑줄 다시 누르면 닫힘", !R.p.isOpen());
  R.click(R.spans[0]);
  R.click(R.find("ig-close"));
  ok("닫기 단추", !R.p.isOpen());
  ok("밑줄 밖 누름은 다루지 않음", R.click(R.doc.body).r === false);
}
{
  const R = popupRig(cfgGPT, mem());
  R.click(R.spans[0]);
  ok("ChatGPT 만 켠 강의: Claude 단추 숨김", R.find("ig-ask").hidden && !R.find("ig-def").hidden);
  const R2 = popupRig(null, mem());
  R2.click(R2.spans[0]);
  ok("ask 끈 강의: 풀이와 자세히만", R2.find("ig-ask").hidden && !R2.find("ig-see").hidden);
}
{
  // 발표 덱(#present) 안 사본은 누르지 않는다
  const R = popupRig(cfgBoth, mem());
  const deck = R.doc.body.appendChild(R.doc.createElement("div"));
  deck.id = "present";
  deck.innerHTML = '<p><span class="itda-term" data-term="3">스킬</span></p>';
  ok("발표 덱의 밑줄은 다루지 않음", R.click(deck.children[0].children[0]).r === false && !R.p.isOpen());
  ok("발표 덱의 밑줄 키도", R.p.onKey({ key: "Enter", target: deck.children[0].children[0] }) === "");
}

{
  // install: 연 뒤 보기 영역이 문턱(SCROLL_SHUT) 넘게 굴러야 닫는다 — 누르기 직전 굴림의 늦은 scroll 은 닫지 않는다
  const R = popupRig(cfgBoth, mem());
  const H = {};
  const tgt = name => ({ addEventListener: (t, f) => { H[name + ":" + t] = f; }, removeEventListener: (t) => { delete H[name + ":" + t]; } });
  const win = tgt("win"), doc = tgt("doc"), viewer = Object.assign(tgt("viewer"), { scrollTop: 500 });
  const off = G.install(R.p, win, doc, viewer);
  H["win:click"]({ target: R.spans[0], preventDefault() {}, stopPropagation() {} });
  ok("install: 누르면 열림", R.p.isOpen());
  viewer.scrollTop = 500 + G.SCROLL_SHUT;
  H["viewer:scroll"]();
  ok("install: 문턱 안 굴림은 그대로", R.p.isOpen());
  viewer.scrollTop = 500 + G.SCROLL_SHUT + 1;
  H["viewer:scroll"]();
  ok("install: 문턱 넘게 굴리면 닫힘(쪽 넘김)", !R.p.isOpen());
  H["win:keydown"]({ key: "Enter", target: R.spans[1], preventDefault() {}, stopPropagation() {} });
  viewer.scrollTop -= 10;
  H["viewer:scroll"]();
  ok("install: 키로 연 뒤 기준도 다시 잰다", R.p.isOpen());
  R.p.el.style.top = "1px";
  H["win:resize"]();
  ok("install: 창 크기가 바뀌면 닫지 않고 다시 놓음", R.p.isOpen() && R.p.el.style.top !== "1px");
  off();
  ok("install: 떼기", Object.keys(H).length === 0);
}

{
  // 화면 아래 끝의 용어 — 메뉴를 펴고 접을 때·창 크기가 바뀔 때 다시 놓아 늘 창 안(검수 1차 1)
  const cases = [["넓은 창 1711×1070", 1711, 1070], ["좁은 창 360×640", 360, 640], ["낮은 창 360×300(위아래 다 모자람)", 360, 300]];
  for (const [name, vw, vh] of cases) {
    const R = popupRig(cfgBoth, mem(), { w: vw, h: vh });
    R.doc.vh = vh;
    R.spans[0].rect = { left: vw - 70, top: vh - 30, right: vw - 10, bottom: vh - 8, width: 60, height: 22 };
    const inside = () => { const t = parseInt(R.p.el.style.top, 10), l = parseInt(R.p.el.style.left, 10); return t >= 8 && t + R.p.el.offsetHeight <= vh - 8 && l >= 8 && l + R.p.el.offsetWidth <= vw - 8; };
    R.click(R.spans[0]);
    ok(name + ": 연 자리가 창 안", inside(), [R.p.el.style.left, R.p.el.style.top, R.p.el.offsetHeight]);
    R.click(R.find("ig-ask-more"));
    ok(name + ": 메뉴를 편 높이로도 창 안", !R.find("ig-ask-menu").hidden && inside(), [R.p.el.style.top, R.p.el.offsetHeight]);
    R.click(R.find("ig-ask-more"));
    ok(name + ": 메뉴를 접어도 창 안", R.find("ig-ask-menu").hidden && inside(), [R.p.el.style.top, R.p.el.offsetHeight]);
  }
}

{
  // 팝업 안의 키(리뷰 1회차 [1]) — install 의 window 붙잡기 처리기 → (전파가 멈추지 않았으면) 실제 share.js keyHandler(pager.js navKey)로 이어 배경 교안이
  // 넘어가지 않는지, 팝업 안 기본 동작(굴리기·Tab·누르기)은 살리는지, Tab 이 팝업 밖으로 나가면 닫고 밑줄로 돌아가는지 본다.
  const sc = { console };
  sc.window = sc;
  vm.createContext(sc);
  for (const f of sharePaths) vm.runInContext(fs.readFileSync(f, "utf8"), sc);
  const S = sc.window.ItdaShare;
  ok("전역 키: share.js keyHandler 를 읽음", !!(S && S.keyHandler && sc.window.ItdaPager), sharePaths);
  const R = popupRig(cfgBoth, mem());
  const H = {};
  const tgt = name => ({ addEventListener: (t, f) => { H[name + ":" + t] = f; }, removeEventListener: (t) => { delete H[name + ":" + t]; } });
  const viewer = Object.assign(tgt("viewer"), { scrollTop: 0 });
  G.install(R.p, tgt("win"), tgt("doc"), viewer);
  const turns = [], moved = [];
  const kh = S.keyHandler({ pager: sc.window.ItdaPager, fol: { moved: () => moved.push(1) }, jumper: { clear() {} }, closeDrawer() {}, ended: () => false, turn: d => turns.push(d) });
  const press = (k, target, extra) => {
    const e = Object.assign({ key: k, target, altKey: false, ctrlKey: false, metaKey: false, shiftKey: false, isComposing: false, defaultPrevented: false, stopped: false,
      preventDefault() { this.defaultPrevented = true; }, stopPropagation() { this.stopped = true; } }, extra || {});
    H["win:keydown"](e);
    if (!e.stopped) kh(e);
    return e;
  };
  const body = R.spans[0].parentNode;
  press("PageDown", body);
  ok("전역 키 대조: 팝업 없이 본문의 PageDown 은 쪽을 넘긴다", turns.length === 1, turns);
  turns.length = 0; moved.length = 0;
  const eo = press("Enter", R.spans[0]);
  ok("전역 키: 밑줄 Enter 로 열기(기본 동작 막음)", R.p.isOpen() && eo.defaultPrevented && eo.stopped);
  for (const [k, where] of [["PageDown", "dialog"], ["PageUp", "dialog"], ["Home", "dialog"], ["End", "dialog"], [" ", "dialog"], ["ArrowDown", "dialog"], [" ", "go"], ["PageDown", "go"], ["End", "more"]]) {
    const t = where === "dialog" ? R.p.el : R.find("ig-ask-" + where);
    const e = press(k, t);
    ok("전역 키: 팝업 안(" + where + ") " + JSON.stringify(k) + " 은 쪽을 넘기지 않고 팝업도 열린 채 — 기본 동작은 둔다",
      turns.length === 0 && moved.length === 0 && R.p.isOpen() && e.stopped && !e.defaultPrevented, { turns, moved, open: R.p.isOpen(), e: [e.stopped, e.defaultPrevented] });
  }
  // Tab: 안에서는 그대로(기본 이동), 끝에서 밖으로 나가면 닫고 밑줄로
  let e = press("Tab", R.find("ig-close"));
  ok("Tab: 팝업 안(닫기 → 다음)은 그대로", R.p.isOpen() && e.stopped && !e.defaultPrevented);
  e = press("Tab", R.find("ig-ask-more"));
  ok("Tab: 마지막(▾, 목록 접힘)에서 밖으로 → 닫고 밑줄로", !R.p.isOpen() && e.defaultPrevented && R.doc.activeElement === R.spans[0]);
  press("Enter", R.spans[0]);
  R.click(R.find("ig-ask-more"));
  e = press("Tab", R.find("ig-ask-more"));
  ok("Tab: 목록을 펴면 ▾ 다음은 목록(그대로)", R.p.isOpen() && !e.defaultPrevented);
  e = press("Tab", R.find("ig-ask-menu").childNodes[2]);
  ok("Tab: 펴진 목록의 마지막(질문만 복사)에서 밖으로 → 닫고 밑줄로", !R.p.isOpen() && e.defaultPrevented && R.doc.activeElement === R.spans[0]);
  press("Enter", R.spans[0]);
  e = press("Tab", R.p.el, { shiftKey: true });
  ok("Shift+Tab: 팝업 자체에서 뒤로 → 닫고 밑줄로", !R.p.isOpen() && e.defaultPrevented && R.doc.activeElement === R.spans[0]);
  press("Enter", R.spans[0]);
  e = press("Tab", R.find("ig-close"), { shiftKey: true });
  ok("Shift+Tab: 처음(닫기)에서 뒤로 → 닫고 밑줄로", !R.p.isOpen() && e.defaultPrevented);
  press("Enter", R.spans[0]);
  e = press("PageDown", R.p.el, { isComposing: true });
  ok("전역 키: 조합 중인 키도 팝업 안이면 전역으로 가지 않는다", turns.length === 0 && R.p.isOpen() && e.stopped && !e.defaultPrevented);
  R.p.close(false);
  press("PageDown", body);
  ok("전역 키: 닫은 뒤 본문의 PageDown 은 다시 넘긴다", turns.length === 1, turns);
  const menu = R.find("ig-ask-menu");
  ok("여는 곳 목록: 메뉴 역할이 아니라 묶음(role=group)·▾ 가 aria-controls 로 가리킴", menu.getAttribute("role") === "group" && R.find("ig-ask-more").getAttribute("aria-controls") === menu.id && R.find("ig-ask-more").getAttribute("aria-haspopup") == null &&
    menu.childNodes.every(c => c.getAttribute("role") == null));
}

{
  // 호스트가 see 주소를 못 만들면(seeHref → null — 꼴 틀린 문서 id, 리뷰 1회차 [3]) 「자세히」 줄을 숨긴다
  const R = popupRig(cfgBoth, mem(), null, { seeHref: () => null });
  R.click(R.spans[0]);
  ok("자세히: 호스트가 주소를 못 만들면 줄을 숨김", R.p.isOpen() && R.find("ig-see").hidden && R.find("ig-see-a").getAttribute("href") == null);
}

console.log("R135 GLOSSARY DONE cases=" + cases + " failed=" + failed);
process.exit(failed ? 1 : 0);
