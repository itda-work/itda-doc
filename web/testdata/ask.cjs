// 묻기 단추(itda-hyve#896, web/ask.js) 시험 — ask_test.go 가 ask.js 경로를 넘긴다. 가짜 DOM 위에서 돈다.
// 켬 설정 정리(normalize)·OS 두 출처(blockOS — title·언어·짝 블록, browserOS — userAgentData → userAgent)·질문 글 두 틀(코드 블록·슬라이드)·
// 화면 글자(slideText — 노트 칸·단추·장 바닥 제외)·노트는 실은 렌더일 때만(창 렌더처럼 호스트가 노트를 들고 있어도)·공개 묶음의 슬라이드 주소·
// 주소 넘침 문턱(24,000자 경계)·넘침의 클립보드 길·슬라이드 막대 달기(멱등·끄기·장 수 그대로·글자 없음)·위임 클릭을 본다.
// 실제 화면(CSS·새 탭·클립보드)은 호스트 앱 selftest 가 본다.
"use strict";
const fs = require("fs");
const vm = require("vm");
const [askPath] = process.argv.slice(2);
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
    this._html = "";
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
  get innerHTML() { return this._html; }
  set innerHTML(h) { this._html = h; } // 단추 SVG 만 — 글자 자식을 만들지 않는다(실제 SVG 에도 글자가 없다)
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
  cloneNode(deep) { const c = new El(this.tag); c.attrs = { ...this.attrs }; c._html = this._html; if (deep) for (const k of this.childNodes) c.appendChild(k.cloneNode(true)); return c; }
  addEventListener(t, f) { (this.on[t] = this.on[t] || []).push(f); }
  blur() { this.blurred = true; }
}
const h = (tag, attrs, ...kids) => { const e = new El(tag); for (const k in attrs || {}) e.setAttribute(k, attrs[k]); e.append(...kids); return e; };
// parseHTML 은 시험용 아주 작은 HTML 풀이다(여는·닫는 태그와 글자 — 속성은 class 만).
function parseHTML(html) {
  const body = new El("body"), stack = [body];
  const re = /<\/?([a-zA-Z0-9]+)([^>]*)>|([^<]+)/g;
  let m;
  while ((m = re.exec(html))) {
    if (m[3] !== undefined) { stack[stack.length - 1].append(m[3]); continue; }
    if (m[0][1] === "/") { if (stack.length > 1) stack.pop(); continue; }
    const e = new El(m[1]);
    const c = /class="([^"]*)"/.exec(m[2]);
    if (c) e.className = c[1];
    stack[stack.length - 1].appendChild(e);
    if (!/^(br|img|hr)$/i.test(m[1])) stack.push(e);
  }
  return body;
}

function world(o) {
  o = o || {};
  const doc = new El("html");
  doc.body = doc.appendChild(new El("body"));
  doc.createElement = t => new El(t);
  const opened = [];
  const win = { document: doc, location: { href: o.href || "https://docs.example/camp/@abc/main/?x=1#old" },
    navigator: o.navigator || { userAgentData: { platform: "macOS" }, userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)" },
    open: (u, t, f) => { opened.push([u, t, f]); return null; },
    DOMParser: class { parseFromString(s) { return { body: parseHTML(s.replace(/^<body>|<\/body>$/g, "")) }; } } };
  const ctx = vm.createContext({ window: win, console, Promise, encodeURIComponent, parseInt, String, Array, Object, JSON, setTimeout });
  vm.runInContext(fs.readFileSync(askPath, "utf8"), ctx);
  return { win, doc, A: win.ItdaAsk, opened };
}
const tick = () => new Promise(r => setTimeout(r, 0));

(async () => {
  // ---- 1. 켬 설정
  {
    const { A } = world();
    eq("켬: 두 서비스(차례는 claude·chatgpt 로 정한다)", A.normalize({ services: ["chatgpt", "claude"], course: " 과정 ", doc: "교안" }).services, ["claude", "chatgpt"]);
    eq("켬: 과정·교안 글자는 앞뒤 공백을 뗀다", A.normalize({ services: ["claude"], course: " 과정 ", doc: " 교안 " }), { services: ["claude"], course: "과정", doc: "교안", notes: false, public: false });
    ok("켬: ask 없음(null)·빈 목록·모르는 이름만 → 끔", A.normalize(null) === null && A.normalize({ services: [] }) === null && A.normalize({ services: ["gemini"] }) === null && A.normalize({}) === null);
    ok("켬: notes·public 은 참(true)일 때만", A.normalize({ services: ["claude"], notes: "true", public: 1 }).notes === false && A.normalize({ services: ["claude"], notes: true, public: true }).public === true);
    A.set({ services: ["claude"] });
    ok("켬: set 뒤 config, set(null) 로 끔", A.config().services.join() === "claude" && A.set(null) === null && A.config() === null);
  }

  // ---- 2. OS 두 출처
  {
    const { A } = world();
    eq("블록 OS: camp 교안의 title 꼴", ["macOS · 리눅스 — 버전 관리 켜기", "윈도우 파워쉘 7 — 종료 코드", "macOS에서 gh 설치", "윈도우에서 gh 설치", "Ubuntu 서버", "내 PC 에서", "Windows / macOS"].map(A.osOfTitle),
      [["macOS", "Linux"], ["Windows"], ["macOS"], ["Windows"], ["Linux"], [], ["macOS", "Windows"]]);
    const code = lang => h("code", lang ? { class: "language-" + lang } : {}, "echo 1\n");
    const col = (...pres) => h("div", { class: "column" }, ...pres);
    const win = h("pre", { "data-title": "윈도우에서 gh 설치" }, code("powershell"));
    const mac = h("pre", {}, code("zsh"));
    h("div", { class: "columns" }, col(win), col(mac));
    eq("블록 OS: title 이 먼저", A.blockOS(win), "Windows");
    eq("블록 OS: 단 묶음의 짝이 Windows 뿐이고 유닉스 셸이면 macOS · Linux", A.blockOS(mac), "macOS · Linux");
    const ps = h("pre", {}, code("powershell"));
    eq("블록 OS: title 이 없어도 Windows 셸 언어면 Windows", A.blockOS(ps), "Windows");
    const lone = h("pre", {}, code("sh"));
    h("div", {}, h("pre", { "data-title": "macOS 터미널" }, code("zsh")), lone);
    eq("블록 OS: 짝이 macOS 면 짐작하지 않는다(모름)", A.blockOS(lone), "");
    const sib = h("pre", {}, code("bash"));
    h("div", {}, h("pre", { "data-title": "윈도우 파워쉘 7 — 종료 코드" }, code("powershell")), sib);
    eq("블록 OS: 바로 앞 형제 pre 가 짝이 된다", A.blockOS(sib), "macOS · Linux");
    eq("블록 OS: 아무 단서 없음", A.blockOS(h("pre", {}, code("python"))), "");
    eq("브라우저 OS: userAgentData.platform 이 먼저", A.browserOS({ userAgentData: { platform: "Windows" }, userAgent: "Macintosh" }), "Windows");
    eq("브라우저 OS: 없으면 userAgent(Windows·macOS·iOS·Android·Linux)", ["Mozilla/5.0 (Windows NT 10.0; Win64; x64)", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)", "Mozilla/5.0 (Linux; Android 14)", "Mozilla/5.0 (X11; Linux x86_64)"].map(ua => A.browserOS({ userAgent: ua })),
      ["Windows", "macOS", "iOS", "Android", "Linux"]);
    eq("브라우저 OS: 모르면 빈 글자", A.browserOS({ userAgent: "Bot/1.0" }), "");
  }

  // ---- 3. 질문 글 — 코드 블록
  {
    const { A } = world();
    const c = A.normalize({ services: ["claude", "chatgpt"], course: "클로드 과정", doc: "1일차" });
    const both = A.codeQuestion({ code: "brew install gh\n", title: "macOS에서 gh 설치", blockOS: "macOS", browserOS: "Windows", slide: "gh 설치" }, c);
    eq("코드 질문: 틀(머리·title·OS 두 줄 규칙·--- 뒤 코드, 끝 줄바꿈 뗌)", both.split("\n"), [
      "「클로드 과정 · 1일차 · gh 설치」 실습 중인 수강생입니다.",
      "아래 명령(macOS에서 gh 설치)이 무엇을 하는지 한 줄씩 설명해 주세요.",
      "옵션의 뜻, 실행하면 보일 출력, 흔한 오류와 확인 방법도 알려 주세요.",
      "이 명령 블록: macOS / 내 PC: Windows (브라우저 추정)",
      "---",
      "brew install gh"]);
    const noBlock = A.codeQuestion({ code: "ls\n", blockOS: "", browserOS: "macOS" }, c).split("\n");
    eq("코드 질문: 블록 OS 를 모르면 그 칸을 빼고 브라우저 OS 만, title 없으면 괄호 없음", [noBlock[0], noBlock[1], noBlock[3]], ["「클로드 과정 · 1일차」 실습 중인 수강생입니다.", "아래 명령이 무엇을 하는지 한 줄씩 설명해 주세요.", "내 PC: macOS (브라우저 추정)"]);
    const none = A.codeQuestion({ code: "ls\n", blockOS: "", browserOS: "" }, c).split("\n");
    eq("코드 질문: 둘 다 모르면 OS 줄이 없다", none[3], "---");
    A.set({ services: ["claude"], course: "과정" }, { title: "문서 제목" });
    ok("코드 질문: 교안 제목이 비면 호스트의 문서 제목", A.codeQuestion({ code: "x" }).startsWith("「과정 · 문서 제목」"), A.codeQuestion({ code: "x" }));
    // forCode — 슬라이드 안 블록의 제목·title·OS 를 읽는다
    const pre = h("pre", { "data-title": "윈도우 파워쉘 7 — 종료 코드" }, h("code", { class: "language-powershell" }, "$LASTEXITCODE\n"));
    h("div", { class: "page slide", "data-kind": "content" }, h("div", { class: "slide-head" }, h("div", { class: "slide-title" }, "종료 코드 ", h("span", { class: "slide-cont" }, "(계속 2/3)"))),
      h("div", { class: "page-body" }, pre));
    const fq = A.forCode(pre, "$LASTEXITCODE\n").split("\n");
    eq("forCode: 슬라이드 제목(「(계속)」 표지 뺌)·블록 OS·브라우저 OS", [fq[0], fq[3]], ["「과정 · 문서 제목 · 종료 코드」 실습 중인 수강생입니다.", "이 명령 블록: Windows / 내 PC: macOS (브라우저 추정)"]);
  }

  // ---- 4. 주소 — 24,000자 문턱(경계값)과 넘침
  {
    const { A } = world();
    const P = "https://claude.ai/new?q=".length, G = "https://chatgpt.com/?q=".length;
    const at = A.askURL("claude", "a".repeat(A.LIMIT - P)), over = A.askURL("claude", "a".repeat(A.LIMIT - P + 1));
    ok("주소: 인코딩한 주소가 정확히 24,000자면 주소로", A.LIMIT === 24000 && !at.overflow && at.url.length === 24000, { len: at.url.length });
    ok("주소: 24,001자면 넘침 — 빈 새 대화 주소", over.overflow && over.url === "https://claude.ai/new", over.url.slice(0, 40));
    const g = A.askURL("chatgpt", "a".repeat(A.LIMIT - G));
    ok("주소: chatgpt 도 같은 문턱(24,000 은 주소로)", !g.overflow && g.url.startsWith("https://chatgpt.com/?q=aaa") && A.askURL("chatgpt", "a".repeat(A.LIMIT - G + 1)).url === "https://chatgpt.com/", g.url.length);
    const ko = A.askURL("claude", "가");
    ok("주소: 한글은 퍼센트 인코딩(한 글자 9자)으로 센다", ko.url === "https://claude.ai/new?q=%EA%B0%80" && A.askURL("claude", "가".repeat(Math.floor((A.LIMIT - P) / 9))).overflow === false && A.askURL("claude", "가".repeat(Math.floor((A.LIMIT - P) / 9) + 1)).overflow === true);
    ok("주소: 줄바꿈·& 도 인코딩(주소를 깨지 않는다)", A.askURL("chatgpt", "a&b\nc").url === "https://chatgpt.com/?q=a%26b%0Ac");
    ok("주소: 모르는 서비스는 null", A.askURL("gemini", "x") === null);
  }

  // ---- 5. ask — 새 탭·넘침의 클립보드 길
  {
    const { A, opened, doc } = world();
    const log = [];
    const r = A.ask("claude", "질문", { toast: t => log.push(["toast", t]), write: t => { log.push(["write", t]); return Promise.resolve(); } });
    ok("ask: 짧으면 기본 window.open(주소, _blank, noopener) — 클립보드는 쓰지 않는다", !r.overflow && opened.length === 1 && opened[0][0] === "https://claude.ai/new?q=%EC%A7%88%EB%AC%B8" && opened[0][1] === "_blank" && opened[0][2] === "noopener" && log.length === 0, { opened, log });
    const big = "x".repeat(30000), seq = [];
    A.ask("chatgpt", big, { open: u => seq.push(["open", u]), toast: t => seq.push(["toast", t]), write: t => { seq.push(["write", t.length]); return Promise.resolve(); } });
    await tick();
    const box = () => doc.body.querySelector(":scope > .itda-asknote");
    const msg = () => (box() && box().querySelector(".itda-asknote-msg").textContent) || "";
    ok("ask: 넘치면 클립보드에 먼저 쓰고 빈 새 대화를 연 뒤 안내 상자에 「붙여 넣으세요」(토스트가 아니다)", seq[0][0] === "write" && seq[0][1] === 30000 && seq[1][0] === "open" &&
      seq[1][1] === "https://chatgpt.com/" && seq.length === 2 && /ChatGPT 새 대화 입력칸에 붙여 넣으세요/.test(msg()), { seq, msg: msg() });
    ok("ask: 안내 상자는 발표의 겹침 층(data-pr-overlay — 발표 중에도 보이고 누르기가 넘김·판서가 아니다), body 의 자식, 글 칸은 숨김",
      !!box() && box().hasAttribute("data-pr-overlay") && box().parentNode === doc.body && box().querySelector(".itda-asknote-text").hidden === true &&
      !box().classList.contains("failed"), {});
    await new Promise(r => setTimeout(r, 30));
    ok("ask: 안내 상자는 시간이 지나도 남는다(사람이 닫을 때까지)", !!box());
    let tries = 0;
    const flaky = t => { tries++; return tries === 1 ? Promise.reject(new Error("막힘")) : Promise.resolve(t); };
    const fail = [];
    const r2 = A.ask("claude", big, { open: u => fail.push(["open", u]), toast: t => fail.push(["toast", t]), write: flaky });
    await r2.promise;
    const ta = box().querySelector(".itda-asknote-text");
    ok("ask: 클립보드 쓰기가 실패하면 「복사 실패」 와 까닭, 직접 고를 질문 글 칸(같은 상자 하나 — 새 대화는 연다)",
      fail.length === 1 && fail[0][1] === "https://claude.ai/new" && /^복사 실패/.test(msg()) && /막힘/.test(msg()) && box().classList.contains("failed") &&
      ta.hidden === false && ta.value === big && ta.hasAttribute("readonly") && doc.body.querySelectorAll(":scope > .itda-asknote").length === 1, { fail, msg: msg() });
    const H = A.handler({ toast: t => fail.push(["toast", t]) });
    const ev = t => ({ target: t, dp: false, preventDefault() { this.dp = true; }, stopPropagation() {} });
    const e = ev(box().querySelector(".itda-asknote-copy"));
    H(e);
    await tick(); await tick();
    ok("ask: 「다시 복사」 는 그 누르기 안에서 다시 쓰고, 되면 「붙여 넣으세요」 로 바꾸고 글 칸을 숨긴다", e.dp && tries === 2 && /Claude 새 대화 입력칸에 붙여 넣으세요/.test(msg()) &&
      box().querySelector(".itda-asknote-text").hidden === true && fail.length === 1, { tries, msg: msg() });
    H(ev(box().querySelector(".itda-asknote-close")));
    ok("ask: 「닫기」 는 상자를 뗀다", !box());
    // document.body 가 없는 호스트(대역)는 토스트로 알린다
    const bare = world();
    bare.doc.body = null;
    const tl = [];
    await bare.A.ask("claude", big, { open: () => {}, toast: t => tl.push(t), write: () => Promise.reject(new Error("없음")) }).promise;
    ok("ask: 안내 상자를 못 달면 toast 로 「복사 실패」", tl.length === 1 && /^복사 실패/.test(tl[0]), tl);
  }

  // ---- 6. 슬라이드 — 화면 글자·노트(실은 렌더만)·주소
  {
    const { A, doc, win } = world();
    // 창 렌더 꼴: 슬라이드는 #pages 안, 노트 칸은 #notes 안(장 밖)에 있고 호스트(state.slideNotes)가 노트 HTML 을 들고 있다
    const mk = (n, title, id) => h("div", { class: "page slide", "data-kind": "content", "data-slide": String(n) },
      h("div", { class: "slide-deco" }, "장식"),
      h("div", { class: "slide-head" }, h("div", id ? { class: "slide-title", id } : { class: "slide-title" }, ...[].concat(title))),
      h("div", { class: "page-body" },
        h("p", {}, "본문 문장 하나"),
        h("ul", {}, h("li", {}, "첫 항목"), h("li", {}, "둘째 ", h("strong", {}, "항목"))),
        h("pre", {}, h("code", {}, "git status\n")),
        h("table", {}, h("tr", {}, h("th", {}, "열"), h("th", {}, "값")), h("tr", {}, h("td", {}, "가"), h("td", {}, "1"))),
        h("div", { class: "notes" }, "본문 안 노트 자리"),
        h("img", { alt: "흐름도" })),
      h("div", { class: "slide-foot" }, h("div", { class: "sf-num" }, n + " / 2")));
    const pages = h("div", { id: "pages", class: "pages" }, mk(1, "첫 장", "first"), mk(2, ["둘째 장 ", h("span", { class: "slide-cont" }, "(계속 2/2)")]));
    const notes = h("div", { id: "notes" }, h("aside", { class: "slide-note", "data-slide": "1" }, h("div", { class: "sn-body" }, h("p", {}, "노트전용문장"))));
    doc.body.append(pages, notes);
    const noteHTML = ["<p>노트전용문장</p><ul><li>노트 항목</li></ul>", ""];
    A.set({ services: ["claude", "chatgpt"], course: "과정", doc: "교안", notes: false }, { notes: i => noteHTML[i] });
    const p1 = pages.children[0];
    const text = A.slideText(p1);
    eq("화면 글자: 본문·목록·코드·표·그림 alt(장식·바닥 번호·노트 자리는 뺀다)", text.split("\n"), ["본문 문장 하나", "- 첫 항목", "- 둘째 항목", "git status", "| 열 | 값 |", "| 가 | 1 |", "[그림: 흐름도]"]);
    const q0 = A.forSlide(p1);
    ok("슬라이드 질문: notes: true 가 아니면 창 렌더(호스트가 노트를 들고 있다)에서도 노트 글이 없다", !q0.includes("노트전용문장") && !q0.includes("노트 항목") && !q0.includes("[발표자 노트]"), q0);
    eq("슬라이드 질문: 틀(머리·제목·--- 뒤 화면 글자), 공개 묶음이 아니면 주소 없음", q0.split("\n").slice(0, 4), ["「과정 · 교안」의 슬라이드 「첫 장」입니다. 처음 배우는 사람 기준으로 풀어 설명하고,", "이해했는지 확인할 질문 2개를 내 주세요.", "---", "본문 문장 하나"]);
    ok("슬라이드 질문: 주소 줄 없음(공개 묶음 아님)", !q0.includes("슬라이드 주소"), q0);
    // 노트를 뺀 렌더(묶음·공유 — notes: false): 호스트의 노트가 비어 있다
    A.set({ services: ["claude"], course: "과정", doc: "교안", notes: false }, { notes: () => "" });
    ok("슬라이드 질문: 노트를 뺀 렌더에도 노트 글이 없다", !A.forSlide(p1).includes("노트전용문장"));
    A.set({ services: ["claude"], course: "과정", doc: "교안", notes: true, public: true }, { notes: i => noteHTML[i] });
    const q1 = A.forSlide(p1);
    ok("슬라이드 질문: notes: true 면 그 장의 노트 글을 붙인다", q1.includes("[발표자 노트]\n노트전용문장\n- 노트 항목"), q1);
    ok("슬라이드 질문: 공개 묶음은 지금 주소(# 앞) + 제목 id", q1.endsWith("슬라이드 주소: https://docs.example/camp/@abc/main/?x=1#first"), q1.slice(-80));
    const q2 = A.forSlide(pages.children[1]);
    ok("슬라이드 질문: 노트 없는 장은 노트 줄이 없고, 제목 id 가 없으면 앞 장의 id, 「(계속)」 표지는 제목에서 뺀다", !q2.includes("[발표자 노트]") && q2.endsWith("#first") && q2.startsWith("「과정 · 교안」의 슬라이드 「둘째 장」입니다."), q2.slice(0, 60));
    // 발표 덱의 장은 id 를 뗀 사본 — 미리보기(#pages)의 같은 번호 장에서 id 를 찾는다
    const clone = p1.cloneNode(true);
    clone.querySelector(".slide-title").attrs = { class: "slide-title" };
    h("div", { class: "pr-deck" }, clone);
    ok("슬라이드 주소: 발표 덱 사본은 미리보기의 같은 장에서 id 를 찾는다", A.slideURL(clone).endsWith("#first"), A.slideURL(clone));
    win.location.href = "https://docs.example/x/";
    ok("슬라이드 주소: 제목 id 가 하나도 없으면 지금 주소만", A.slideURL(h("div", { class: "page slide", "data-slide": "9" })) === "https://docs.example/x/");
  }
  {
    const { A } = world();
    const title = h("div", { class: "page slide", "data-kind": "title" }, h("div", { class: "sc-title" }, " 표지  제목 "));
    const sec = h("div", { class: "page slide", "data-kind": "section" }, h("div", { class: "slide-section" }, h("span", { class: "ss-num" }, "2"), h("span", { class: "ss-title" }, "구역")));
    const autoSec = h("div", { class: "page slide", "data-kind": "section" }, h("div", { class: "slide-section" }, h("span", { class: "ss-num auto" }, "3"), h("span", { class: "ss-title" }, "자동 번호")));
    eq("슬라이드 제목: 제목 장·구역(번호)·자동 번호 구역", [A.slideTitle(title), A.slideTitle(sec), A.slideTitle(autoSec)], ["표지 제목", "2 구역", "자동 번호"]);
    // 공백 정리: 연속 공백·탭은 하나로
    const sp = h("div", { class: "page slide" }, h("div", { class: "page-body" }, h("p", {}, "  여러   칸\t글  ")));
    eq("화면 글자: 공백 정리", A.slideText(sp), "여러 칸 글");
  }

  // ---- 6b. 각주(결정 A5): notes 거짓이면 각주 목록(.footnotes — 노트 안 각주 정의도 goldmark 이 이리로 옮긴다)을 통째로 뺀다. 각주뿐인 장(「주석」)에는
  // 막대를 달지 않는다. 화면 글자 범위.
  {
    const { A } = world();
    const fnList = (...lis) => h("div", { class: "footnotes", role: "doc-endnotes" }, h("hr", {}), h("ol", {}, ...lis));
    const fnSlide = (n, ...lis) => h("div", { class: "page slide", "data-kind": "content", "data-slide": String(n) },
      h("div", { class: "slide-head" }, h("div", { class: "slide-title" }, "주석")), h("div", { class: "page-body" }, fnList(...lis)));
    const li = (n, t) => h("li", { id: "fn:" + n }, h("p", {}, t + " ", h("a", { href: "#fnref:" + n, class: "footnote-backref" }, "↩")));
    const body = h("div", { class: "page slide", "data-kind": "content", "data-slide": "1" }, h("div", { class: "slide-head" }, h("div", { class: "slide-title" }, "장")),
      h("div", { class: "page-body" }, h("p", {}, "본문", h("sup", { class: "footnote-ref" }, h("a", { href: "#fn:1" }, "1"))), fnList(li(1, "PUBLIC-FOOTNOTE"), li(2, "PRIVATE-NOTE-FOOTNOTE"))));
    const only = fnSlide(2, li(1, "PUBLIC-FOOTNOTE"), li(2, "PRIVATE-NOTE-FOOTNOTE"));
    A.set({ services: ["claude"], course: "과정", doc: "교안", notes: false }, { notes: () => "" });
    const q = A.forSlide(body);
    ok("각주: notes 거짓이면 각주 목록을 통째로 뺀다(공개 각주도) — 본문의 참조 번호는 남는다", !q.includes("PRIVATE") && !q.includes("PUBLIC-FOOTNOTE") && q.includes("본문1"), q);
    ok("각주: 「주석」 장(각주뿐)은 본문이 빈다", A.slideText(only) === "" && A.footnoteOnly(only, A.config()) === true && A.footnoteOnly(body, A.config()) === false);
    const box = h("div", { id: "pages" }, body, only);
    A.armSlides(box);
    ok("각주: notes 거짓이면 각주뿐인 장에는 막대가 없고 다른 장에는 있다", !!body.querySelector(".itda-askbar") && !only.querySelector(".itda-askbar"));
    A.set({ services: ["claude"], course: "과정", doc: "교안", notes: true }, { notes: () => "" });
    A.armSlides(box);
    const qt = A.forSlide(only);
    ok("각주: notes 참이면 각주를 넣고(되돌림 링크 글자는 빼고) 각주뿐인 장에도 막대", qt.includes("- PUBLIC-FOOTNOTE\n- PRIVATE-NOTE-FOOTNOTE") && !qt.includes("↩") && !!only.querySelector(".itda-askbar"), qt);
    A.set({ services: ["claude"], course: "과정", doc: "교안", notes: false }, { notes: () => "" });
    A.armSlides(box);
    ok("각주: 다시 notes 거짓이면 다음 armSlides 가 각주뿐인 장의 막대를 뗀다", !only.querySelector(".itda-askbar") && !!body.querySelector(".itda-askbar"));
    ok("각주: slideText 의 켬 설정 인자가 지금 설정보다 먼저", !A.slideText(body, { notes: false }).includes("PUBLIC") && A.slideText(body, { notes: true }).includes("PUBLIC"));
    // 화면 글자 범위 — 보이는 글만
    const vis = h("div", { class: "page slide", "data-slide": "4" }, h("div", { class: "page-body" },
      h("details", {}, h("summary", {}, "접은 제목"), h("p", {}, "접힌 본문")),
      h("details", { open: "" }, h("summary", {}, "편 제목"), h("p", {}, "편 본문")),
      h("p", { style: "color: red; display: none" }, "인라인 숨김"),
      h("p", { hidden: "" }, "hidden 숨김"),
      h("span", { "aria-hidden": "true" }, "aria 숨김"),
      h("p", { style: "display:block" }, "보이는 글")));
    eq("화면 글자 범위: 닫힌 details 는 summary 만, 인라인 display:none·hidden·aria-hidden 은 뺀다", A.slideText(vis).split("\n"), ["접은 제목", "편 제목", "편 본문", "보이는 글"]);
  }

  // ---- 7. 슬라이드 막대 — 달기·멱등·끄기·장 수·글자
  {
    const { A, doc } = world();
    const box = h("div", { id: "pages" });
    for (let i = 1; i <= 3; i++) box.appendChild(h("div", { class: "page slide", "data-slide": String(i) }, h("div", { class: "page-body" }, h("p", {}, "글" + i))));
    box.appendChild(h("div", { class: "page" }, h("div", { class: "page-body" }, "쪽")));
    doc.body.appendChild(box);
    const txt = box.textContent, kids = box.children.length;
    A.set(null);
    ok("막대: 꺼져 있으면 달지 않는다", A.armSlides(box) === 0 && !box.querySelector(".itda-askbar"));
    A.set({ services: ["chatgpt", "claude"], course: "과정" });
    A.armSlides(box);
    const bars = box.querySelectorAll(".itda-askbar");
    ok("막대: 슬라이드 장(.page.slide)마다 하나 — 장의 마지막 자식, 쪽 장에는 없다", bars.length === 3 && bars.every(b => b.parentNode.classList.contains("slide") && b.parentNode.children[b.parentNode.children.length - 1] === b), bars.length);
    eq("막대: 단추는 정해진 차례·data-ask·aria-label·data-label", bars[0].children.map(b => [b.getAttribute("data-ask"), b.getAttribute("aria-label"), b.getAttribute("data-label"), b.type]),
      [["claude", "Claude에게 묻기", "Claude", "button"], ["chatgpt", "ChatGPT에게 묻기", "ChatGPT", "button"]]);
    ok("막대: 장 수·장의 글자(textContent)가 그대로 — 단추 안 글자는 없다", box.children.length === kids && box.textContent === txt, { kids: box.children.length, txt: box.textContent });
    const first = bars[0];
    A.armSlides(box);
    ok("막대: 다시 불러도 그대로(멱등)", box.querySelectorAll(".itda-askbar").length === 3 && box.querySelector(".itda-askbar") === first);
    A.set({ services: ["claude"] });
    A.armSlides(box);
    ok("막대: 서비스가 바뀌면 다시 만든다", box.querySelectorAll(".itda-askbar").length === 3 && box.querySelectorAll("button.itda-ask").length === 3);
    A.set(null);
    A.armSlides(box);
    ok("막대: 끄면 단 막대를 뗀다", !box.querySelector(".itda-askbar") && box.textContent === txt);
  }

  // ---- 8. 위임 클릭
  {
    const { A, doc } = world();
    const pg = h("div", { class: "page slide", "data-kind": "content", "data-slide": "1" }, h("div", { class: "slide-head" }, h("div", { class: "slide-title" }, "장")), h("div", { class: "page-body" }, h("p", {}, "클릭 본문")));
    const box = h("div", { id: "pages" }, pg);
    doc.body.appendChild(box);
    A.set({ services: ["claude", "chatgpt"], course: "과정", doc: "교안" });
    A.armSlides(box);
    const opened = [];
    const H = A.handler({ open: u => opened.push(u) });
    const click = t => { const e = { target: t, dp: false, sp: false, preventDefault() { this.dp = true; }, stopPropagation() { this.sp = true; } }; H(e); return e; };
    const b = pg.querySelector('button.itda-ask[data-ask="chatgpt"]');
    const e1 = click(b);
    ok("클릭: 그 장의 질문 글로 그 서비스의 새 대화를 연다(기본 누르기·전파 막음)", opened.length === 1 && opened[0].startsWith("https://chatgpt.com/?q=") && decodeURIComponent(opened[0].split("?q=")[1]).includes("클릭 본문") && e1.dp && e1.sp, opened);
    const e2 = click(pg.querySelector(".page-body p"));
    ok("클릭: 단추가 아닌 곳은 지나친다", opened.length === 1 && !e2.dp);
    const deck = h("div", { class: "present" }, pg.cloneNode(true));
    const db = deck.querySelector("button.itda-ask");
    click(db);
    ok("클릭: 발표 화면(.present)에서는 누른 뒤 포커스를 남기지 않는다", db.blurred && opened.length === 2);
    A.set(null);
    click(b);
    ok("클릭: 꺼진 뒤 남은 단추는 아무것도 열지 않는다", opened.length === 2);
  }

  console.log("R896 ASK DONE cases=" + cases + " failed=" + failed);
})().catch(e => { console.log("예외 " + (e && e.stack || e)); process.exit(1); });
