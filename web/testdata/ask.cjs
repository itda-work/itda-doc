// 강의 MCP 섹션 단추의 설정·렌더·누르기를 가짜 DOM 에서 확인한다.
"use strict";
const fs = require("fs");
const vm = require("vm");
const [askPath, slidesPath] = process.argv.slice(2);
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
// parseComplex 는 「a b > c」 를 [{c, comb}] 로 — comb 는 앞 마디와의 관계(" "·">"). :scope 는 마디 하나(scope 자신).
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
    this.nodeType = 1; this.tag = tag.toLowerCase(); this.childNodes = []; this.parentNode = null; this.attrs = {}; this.on = {}; this.blurred = false;
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
  get title() { return this.attrs.title || ""; }
  set title(v) { this.attrs.title = v; }
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
  addEventListener(t, f) { (this.on[t] = this.on[t] || []).push(f); }
  blur() { this.blurred = true; }
}
const h = (tag, attrs, ...kids) => { const e = new El(tag); for (const k in attrs || {}) e.setAttribute(k, attrs[k]); e.append(...kids); return e; };
// docOn 은 문서에 걸린 리스너 목록이다 — 묻기 단추는 문서 리스너를 걸지 않는다(아래 「문서 리스너 없음」).
const docOn = {};
const doc = { documentElement: { lang: "ko" }, createElement: tag => new El(tag), addEventListener: (t, f) => (docOn[t] = docOn[t] || []).push(f) }, opened = [];
const win = { document: doc, open: (...args) => opened.push(args) };
vm.runInNewContext(fs.readFileSync(askPath, "utf8"), { window: win, URL });
const A = win.ItdaAsk;
const cfg = { services: ["chatgpt", "claude", "claude"], mcp: "https://docs.example/course/mcp",
  doc_id: '01-"a"', course: "첫 과정", doc: "첫 교안", guide: "https://docs.example/connect" };
const heading = title => h("h2", { id: "a&b" }, title);
const first = heading("시작"), second = heading("복습");
const box = h("div", {}, h("p", {}, "머리표식"), h("section", { class: "cover" }, heading("표지표식")),
  first, h("p", {}, "본문표식"), h("pre", {}, "코드표식"), h("aside", { class: "notes" }, heading("노트표식")),
  h("div", { class: "slide-notes" }, heading("노트칸표식")), h("h3", {}, "주소없음"), second);
for (const patch of [{ mcp: "" }, { doc_id: " " }, { mcp: "http://example.com" }, { mcp: "https://" }, { services: ["기타"] }]) {
  A.set({ ...cfg, ...patch });
  A.armSections(box);
  eq("설정 누락·오류면 끔 " + JSON.stringify(patch), box.querySelectorAll("button").length, 0);
}
A.set(cfg);
A.armSections(box);
A.armSections(box);
eq("두 제목에 서비스마다 한 번", box.querySelectorAll("button").map(b => b.parentElement.textContent), ["시작", "시작", "복습", "복습"]);
// 단추는 제목 글자 앞(첫 자식들)에 서비스 차례대로 든다 — CSS가 float으로 제목 오른쪽 끝에 띄운다. 단추 안에는 글자 노드가 없다.
eq("단추는 제목 글자 앞에 서비스 차례로·글자 없음", [first, second].map(t => t.childNodes.map(n => n.nodeType === 1 ? n.dataset.service + ":" + n.childNodes.length : n.data)),
  [["claude:0", "chatgpt:0", "시작"], ["claude:0", "chatgpt:0", "복습"]]);
const click = b => A.handler()({ target: b, preventDefault() {}, stopPropagation() {} });
for (const [index, title] of [first, second].entries()) {
  const prompts = [];
  for (const [i, b] of title.children.entries()) {
    const name = ["Claude", "ChatGPT"][i] + "에 묻기";
    eq("읽는 이름", b.getAttribute("aria-label"), name);
    // 툴팁은 브라우저 기본 title 두 줄(이름 + 기록 안내 원문)이다. aria-description 은 title 과 겹쳐 읽히지 않게 두지 않는다.
    ok("화면 이름과 기록 안내", b.dataset.label === name && !b.hasAttribute("aria-description") &&
      b.title === name + "\n질문은 강의 커넥터로 처리되고 기록될 수 있습니다. 기록 여부는 커넥터 안내를 확인하세요.");
    click(b);
    const [url, target, features] = opened.at(-1);
    const q = new URL(url).searchParams.get("q");
    eq("서비스 주소와 인코딩", url, ["https://claude.ai/new?q=", "https://chatgpt.com/?q="][i] + encodeURIComponent(q));
    eq("새 탭", [target, features], ["_blank", "noopener"]);
    ok("섹션·하위 읽기", q.includes('read_section(doc=' + JSON.stringify(cfg.doc_id) + ', section="a&b", nth=' + (index + 1) + ', subtree=true)'));
    ok("지침·주소·제목", [cfg.course, cfg.doc, title.textContent, cfg.mcp, cfg.guide, "record_qa", "내 생각"].every(t => q.includes(t)));
    ok("교안 글과 화면 장식 제외", !q.includes("표식"));
    prompts.push(q);
  }
  eq("서비스별 지침 동일", prompts[0], prompts[1]);
}
A.set({ ...cfg, guide: "", doc: "" }, { title: "대체 교안" });
A.armSections(box);
click(first.children[0]);
const fallback = new URL(opened.at(-1)[0]).searchParams.get("q");
ok("안내 주소 생략·교안 제목 대체", fallback.includes("연결 방법") && !fallback.includes(cfg.guide) && fallback.includes("대체 교안"));
{
  const note = h("h2", { id: "dup", class: "teacher notes" }, "비공개 제목");
  const title = h("h3", { id: "dup" }, "첫 ", h("em", {}, "번째", h("span", { class: "notes" }, h("b", {}, "비공개 본문"))), " 제목");
  const next = h("h4", { id: "dup", "data-note": "true" }, "다음 제목");
  const flow = h("div", {}, note, h("h1", { id: "" }), h("h1", { id: "  \t" }), title, next);
  A.set({ ...cfg, services: ["claude"] });
  A.armSections(flow);
  eq("노트 제목과 빈·공백 id는 건너뛰고 같은 id의 순번을 센다",
    flow.querySelectorAll("button").map(b => [b.dataset.title, b.dataset.nth]), [["첫 번째 제목", "1"], ["다음 제목", "2"]]);
  click(title.querySelector("button"));
  const q = new URL(opened.at(-1)[0]).searchParams.get("q");
  ok("제목 안의 중첩 노트 글은 지침에서 제외", !q.includes("비공개") && q.includes("「첫 번째 제목」"));
  eq("화면의 노트 원문은 보존", title.textContent, "첫 번째비공개 본문 제목");
  title.classList.add("notes");
  A.armSections(flow);
  eq("재설정 때 노트로 바뀐 제목의 단추를 떼고 다음 순번을 갱신", flow.querySelectorAll("button").map(b => b.dataset.nth), ["1"]);
}
{
  const section = " " + "긴-id".repeat(40) + " ", docID = "문서-id".repeat(50);
  const mcp = "https://docs.example/" + "m".repeat(160) + "/mcp", guide = "https://docs.example/" + "g".repeat(160);
  const title = h("h2", { id: section }, "🌱".repeat(81));
  const flow = h("div", {}, title);
  const longCfg = { ...cfg, services: ["chatgpt"], mcp, guide, doc_id: docID, course: "가".repeat(81), doc: "나".repeat(81) };
  A.set(longCfg);
  A.armSections(flow);
  click(title.querySelector("button"));
  const q = new URL(opened.at(-1)[0]).searchParams.get("q");
  eq("과정·교안·제목은 각각 생략 부호 포함 최대 80 코드 포인트", q.split(" 섹션")[0],
    "「" + "가".repeat(79) + "… · " + "나".repeat(79) + "…」의 「" + "🌱".repeat(79) + "…」");
  ok("긴 문서·섹션 id와 두 주소는 끝까지 보존", [JSON.stringify(docID), JSON.stringify(section), mcp, guide].every(s => q.includes(s)));
  eq("화면에 보이는 원래 제목 길이 보존", title.textContent, "🌱".repeat(81));
  A.set({ ...longCfg, doc: "", course: "가".repeat(80) }, { title: "교".repeat(100) });
  A.armSections(flow);
  click(title.querySelector("button"));
  const fallback = new URL(opened.at(-1)[0]).searchParams.get("q");
  ok("상한과 같은 과정은 보존하고 호스트 대체 제목에도 상한 적용", fallback.startsWith("「" + "가".repeat(80) + " · " + "교".repeat(79) + "…」"));
}
// 단추 자리(itda-hyve#988 리뷰 [3]): 유효 id의 빈 제목은 끝에 붙이고(append 길), 첫 자식이 요소(번호 span·링크·노트)인 제목은
// 그 요소 앞에 서비스 차례로 넣는다. 다시 불러도 단추 수·차례·data-title·제목 글자가 그대로다.
{
  const empty = h("h2", { id: "empty" });
  const num = h("h2", { id: "num" }, h("span", { class: "header-section-number" }, "1.2"), " 제목 ", h("a", { href: "#x" }, "링크"));
  const link = h("h3", { id: "link" }, h("a", { href: "#y" }, "링크로 시작"), " 뒤");
  const noted = h("h3", { id: "noted" }, h("span", { class: "notes" }, "비공개"), "보이는 제목");
  const flow = h("div", {}, empty, num, link, noted);
  const shape = () => [empty, num, link, noted].map(t => t.childNodes.map(n => n.nodeType !== 1 ? n.data : n.tag === "button" ? n.dataset.service + "|" + n.dataset.title : n.tag));
  const want = [["claude|", "chatgpt|"], ["claude|1.2 제목 링크", "chatgpt|1.2 제목 링크", "span", " 제목 ", "a"],
    ["claude|링크로 시작 뒤", "chatgpt|링크로 시작 뒤", "a", " 뒤"], ["claude|보이는 제목", "chatgpt|보이는 제목", "span", "보이는 제목"]];
  A.set({ ...cfg, services: ["claude", "chatgpt"] });
  A.armSections(flow);
  eq("빈 제목은 끝에, 요소로 시작하는 제목은 첫 요소 앞에 서비스 차례로", shape(), want);
  A.armSections(flow);
  A.armSections(flow);
  eq("다시 불러도 단추 수·차례·data-title 보존", shape(), want);
  eq("제목 글자 보존", [empty, num, link, noted].map(t => t.textContent), ["", "1.2 제목 링크", "링크로 시작 뒤", "비공개보이는 제목"]);
}
// 툴팁은 브라우저 기본 title 이라 set·armSections 는 문서에 리스너를 걸지 않는다(itda-hyve#988 접근 방식 변경).
eq("문서 리스너 없음", Object.keys(docOn), []);
// 실제 slides.build가 구역·내용 제목을 복제하고, 더 낮은 제목은 본문으로 옮긴다.
// 쪽 나눔 대역은 내용 묶음만 두 장으로 나눠 이어지는 머리의 단추 중복을 검사한다.
vm.runInNewContext(fs.readFileSync(slidesPath, "utf8"), { window: win, document: doc, performance });
win.ItdaPager = {
  paginate(flow, target, opts) {
    const page = () => target.appendChild(h("div", { class: "page " + opts.pageClass }));
    page().appendChild(flow);
    if (opts.pageClass.includes("has-head")) page().appendChild(h("div", { class: "page-body" }));
    return { overflow: 0 };
  },
  fitImages() {},
};
const flow = h("div", {}, h("h1", { id: "dup" }, "구역"), h("h2", { id: "dup" }, "내용"), h("h3", { id: "dup" }, "하위"));
A.set({ ...cfg, services: ["claude"] });
A.armSections(flow);
const target = h("div");
win.ItdaSlides.build(flow, target, { level: 2 });
eq("구역·내용·이어지는 장", target.children.length, 3);
eq("구역·내용·본문 제목이 원문 순번을 보존", target.querySelectorAll("button").map(b => [b.dataset.title, b.dataset.nth]),
  [["구역", "1"], ["내용", "2"], ["하위", "3"]]);
ok("이어지는 머리는 단추 없음", target.children[2].querySelector(".slide-cont") && !target.children[2].querySelector("button"));
A.set(null);
A.armSections(box);
eq("다시 끄면 단추 없음", box.querySelectorAll("button").length, 0);
console.log("R965 ASK DONE cases=" + cases + " failed=" + failed);
process.exitCode = failed ? 1 : 0;
