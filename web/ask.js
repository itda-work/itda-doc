// 「Claude에게 묻기」「ChatGPT에게 묻기」(itda-hyve#896) — 교안의 코드 블록·슬라이드를 설명해 달라는 질문 글을 만들어, 그 글을 채운 새 대화를
// 새 탭으로 연다. 호스트 앱의 문서 창(app.js)·공유 보기 화면·md pack 묶음(share.js)이 같이 쓴다. 이 파일은 아무것도 보내지 않는다 — 사람이
// 단추를 누를 때만 브라우저(문서 창은 Go 의 OpenLink — 기본 브라우저)가 주소를 연다. 두 서비스 모두 그 주소로 입력칸만 채우고 보내지 않는다
// (실측 2026-11-16, itda-hyve docs/field-test/ask-ai-url.md). 공식 보장은 없다.
//
// 켬(set): 호스트가 렌더마다 set(cfg, host) 를 부른다. cfg 는 view 의 ask(Go — lecture.yml 의 ask: 를 따른다)
//   { services: ["claude", "chatgpt"], course(lecture.yml title), doc(문서 항목 title — 비면 view 제목), notes(노트를 실은 렌더인가 — 문서 항목의
//   notes: true), public(공개 묶음인가 — 질문 글에 슬라이드 주소를 붙인다) }. cfg 가 없거나 아는 서비스가 하나도 없으면 끈다(단추 없음).
//   host = { notes(i) — i 번(0부터) 슬라이드에 붙은 노트 HTML(slides.js build 의 notes), title — 문서 제목(cfg.doc 이 비었을 때) }.
// 코드 블록 단추는 codecopy.js(호스트 부품)가 config() 를 보고 복사 단추 옆에 단다(button.itda-codecopy.itda-codeask). 슬라이드 단추는
// armSlides(box) 가 장(.page.slide)마다 오른쪽 위에 단다(div.itda-askbar > button.itda-ask). 단추 안 글자는 없다 — 이름은 data-label 을 CSS
// ::after 로 그리고(app.css), 읽는 이름은 aria-label 이다. 그래서 pre.textContent·슬라이드의 textContent·복사·검색에 들어가지 않는다.
//
// 질문 글(틀 하나 — 처음 배우는 사람 기준, 결정 A3):
//   코드 블록: 머리 「{과정} · {교안} · {슬라이드 제목}」(빈 칸은 뺀다), 블록 title(없으면 뺀다), OS 줄(blockOS·browserOS — 둘 다 모르면 줄을 뺀다), --- 뒤 코드.
//   슬라이드: 「{과정} · {교안}」 의 슬라이드 「{제목}」, --- 뒤 화면 글자(slideText — 노트 칸·단추·장 바닥·장식은 뺀다), 노트(cfg.notes 일 때만 —
//   host.notes 의 HTML 을 글자로), 공개 묶음이면 슬라이드 주소(slideURL). cfg.notes 가 거짓이면 각주 목록(FOOTNOTES — 노트 안 각주도 이리로
//   옮겨진다)을 통째로 뺀다(결정 A5). 각주뿐인 장(「주석」 장)에는 그때 막대를 달지 않는다(armSlides — 보낼 본문이 없다).
//   화면 글자의 범위(nodeText): 보이는 글만 — [hidden]·aria-hidden="true"·인라인 style 의 display:none 은 빼고, 닫힌 <details> 는 <summary> 만.
//   테마 CSS 로 숨긴 글(계산된 스타일)은 보지 않는다(넣는다).
// 주소(askURL): 질문 글을 encodeURIComponent 로 붙인 주소가 LIMIT(24,000자)을 넘으면 넘침 — 질문 글을 클립보드에 넣고 빈 새 대화를 열고
// 오래 남는 안내 상자(NOTE — 발표 중에도 보이는 겹침 층)에 「붙여 넣으세요」 를, 쓰기에 실패하면 「복사 실패」 와 다시 복사·직접 고를 글을 보인다(ask).
// 문턱은 두 서비스 모두 32K 를 통과한 실측에서 여유를 둔 값이다.
(function (root) {
  "use strict";

  const SERVICES = {
    claude: { label: "Claude", name: "Claude에게 묻기", base: "https://claude.ai/new", q: "https://claude.ai/new?q=" },
    chatgpt: { label: "ChatGPT", name: "ChatGPT에게 묻기", base: "https://chatgpt.com/", q: "https://chatgpt.com/?q=" },
  };
  const ORDER = ["claude", "chatgpt"];
  const LIMIT = 24000; // 인코딩한 주소 전체 길이의 상한(이 값까지는 주소로, 넘으면 클립보드로)
  const BAR = "itda-askbar", BTN = "itda-ask";
  // Lucide message-circle-question 꼴(LICENSE-Lucide.txt)
  const SVG_ASK = '<svg class="ic" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M7.9 20A9 9 0 1 0 4 16.1L2 22Z"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><path d="M12 17h.01"/></svg>';

  let cur = null; // set 으로 받은 켬 설정(정리한 것) — null 이면 끔
  let host = {};

  // normalize 는 view 의 ask 를 정리한다 — 아는 서비스만 정해진 차례로(겹치면 하나), 하나도 없으면 null.
  function normalize(cfg) {
    if (!cfg || typeof cfg !== "object" || !Array.isArray(cfg.services)) return null;
    const services = ORDER.filter(s => cfg.services.includes(s));
    if (!services.length) return null;
    const str = v => (typeof v === "string" ? v.trim() : "");
    return { services, course: str(cfg.course), doc: str(cfg.doc), notes: cfg.notes === true, public: cfg.public === true };
  }
  function set(cfg, h) { cur = normalize(cfg); host = h || {}; return cur; }
  function config() { return cur; }

  // ---- OS 두 출처(결정 7): 블록에 적힌 OS(title·언어·OS 별 짝 블록)와 브라우저가 알려 주는 OS(「(브라우저 추정)」)

  // osOfTitle 은 블록 title 글자에 적힌 OS 들이다(macOS·Windows·Linux 차례, 없으면 []). camp 교안의 꼴: 「macOS · 리눅스 — …」「윈도우 파워쉘 7 — …」.
  function osOfTitle(t) {
    const s = String(t || "");
    const out = [];
    if (/mac\s?os|\bmac\b|\bosx\b|맥\s?os|맥에서|맥북|맥용/i.test(s)) out.push("macOS");
    if (/windows|윈도우|윈도|파워\s?쉘|powershell|\bpwsh\b|\bcmd\b|명령\s?프롬프트/i.test(s)) out.push("Windows");
    if (/linux|리눅스|ubuntu|우분투|debian|데비안/i.test(s)) out.push("Linux");
    return out;
  }
  const WIN_LANGS = ["powershell", "ps1", "pwsh", "bat", "batch", "cmd"];
  const UNIX_LANGS = ["sh", "bash", "zsh", "shell", "console", "fish"];
  function langOf(pre) {
    const code = pre && pre.querySelector && pre.querySelector("code");
    const m = code && /(?:^|\s)language-([^\s]+)/.exec(code.getAttribute("class") || "");
    return m ? m[1].toLowerCase() : "";
  }
  // partners 는 pre 의 OS 짝 블록 후보다 — 단(.column) 안이면 같은 단 묶음(.columns)의 다른 단에 든 pre, 아니면 바로 앞뒤 형제 pre.
  function partners(pre) {
    const col = pre.closest && pre.closest(".column");
    const cols = col && col.parentElement;
    if (col && cols && cols.classList && cols.classList.contains("columns")) {
      const out = [];
      for (const c of cols.children || []) if (c !== col) for (const p of c.querySelectorAll("pre")) out.push(p);
      return out;
    }
    const out = [];
    for (const s of [pre.previousElementSibling, pre.nextElementSibling]) if (s && s.tagName === "PRE") out.push(s);
    return out;
  }
  // blockOS 는 블록에 적힌 OS 다(모르면 ""): ① title 글자, ② 언어가 Windows 셸(powershell·cmd …)이면 Windows, ③ 짝 블록의 title 이 Windows 뿐이고
  // 이 블록 언어가 유닉스 셸(sh·zsh …)이면 「macOS · Linux」. 짝 블록이 macOS·Linux 라고 이 블록을 Windows 로 짐작하지는 않는다(②가 맡는다).
  function blockOS(pre) {
    if (!pre || !pre.getAttribute) return "";
    const own = osOfTitle(pre.getAttribute("data-title"));
    if (own.length) return own.join(" · ");
    const lang = langOf(pre);
    if (WIN_LANGS.includes(lang)) return "Windows";
    if (UNIX_LANGS.includes(lang)) {
      for (const p of partners(pre)) {
        const o = osOfTitle(p.getAttribute("data-title"));
        if (o.length === 1 && o[0] === "Windows") return "macOS · Linux";
      }
    }
    return "";
  }
  // browserOS 는 브라우저가 알려 주는 OS 다(navigator.userAgentData.platform → navigator.userAgent, 모르면 ""). 추정이라 질문 글에 「(브라우저 추정)」 을 붙인다.
  function browserOS(nav) {
    const n = nav === undefined ? root.navigator : nav;
    if (!n) return "";
    const p = n.userAgentData && typeof n.userAgentData.platform === "string" ? n.userAgentData.platform.trim() : "";
    const known = { macos: "macOS", windows: "Windows", linux: "Linux", android: "Android", "chrome os": "ChromeOS", chromeos: "ChromeOS", ios: "iOS" };
    if (p && known[p.toLowerCase()]) return known[p.toLowerCase()];
    const ua = String(n.userAgent || "");
    if (/Windows/.test(ua)) return "Windows";
    if (/iPhone|iPad|iPod/.test(ua)) return "iOS";
    if (/Android/.test(ua)) return "Android";
    if (/CrOS/.test(ua)) return "ChromeOS";
    if (/Mac OS X|Macintosh/.test(ua)) return "macOS";
    if (/Linux/.test(ua)) return "Linux";
    return p;
  }

  // ---- 화면 글자

  const clean = s => String(s || "").replace(/\s+/g, " ").trim();
  // SKIP 은 질문 글에 넣지 않는 것: 노트 칸(.slide-note·.sn-body·.pr-notes-body·.notes·#notes)·단추·장 바닥·장식·판서 층·축소 표지·스크립트·스타일.
  const SKIP = ".slide-note, .sn-body, .pr-notes-body, .notes, #notes, button, ." + BAR + ", a.footnote-backref, .slide-foot, .slide-deco, .ink-preview, svg, script, style, template, [hidden], [aria-hidden=\"true\"]";
  // FOOTNOTES 는 각주 목록이다(mdoc render.go footnoteList 의 div.footnotes — goldmark 기본 꼴 section.footnotes 도 같은 class, slides.js 가
  // 「주석」 장으로 옮긴다). goldmark 은 노트 안에 적은 각주 정의도 노트 칸 밖의 이 목록으로 옮기므로, 노트를 넣지 않는 질문 글(cfg.notes 거짓)은
  // 각주 목록을 통째로 뺀다(결정 A5 — 각주마다 노트 출처를 가리지 않는다. 공개 각주도 빠진다). 본문의 각주 참조 번호(sup)는 공개 글이라 남는다.
  const FOOTNOTES = ".footnotes";
  const skipFor = c => (c && c.notes) ? SKIP : SKIP + ", " + FOOTNOTES;
  const BLOCK = /^(P|DIV|SECTION|ARTICLE|ASIDE|HEADER|FOOTER|BLOCKQUOTE|FIGURE|FIGCAPTION|UL|OL|DL|DT|DD|TABLE|THEAD|TBODY|TFOOT|H[1-6]|NAV|DETAILS|SUMMARY|HR)$/;
  // nodeText 는 요소의 화면 글자다(블록마다 줄을 바꾸고, 목록 항목은 「- 」, 표는 칸을 「 | 」 로, 코드 블록은 글자 그대로, 그림은 alt). skip(기본 SKIP —
  // 노트를 넣지 않는 질문 글은 skipFor 로 각주 목록 FOOTNOTES 까지)은 건너뛴다.
  function nodeText(el, skip) {
    skip = skip || SKIP;
    const lines = [];
    let line = "";
    // 목록 머리 「- 」 만 남은 줄은 내지 않고 둔다 — li 안의 첫 블록(각주·느슨한 목록의 p)이 그 뒤에 이어 붙는다
    const flush = () => { const t = line.replace(/[ \t]+/g, " ").trim(); if (t === "-") return; if (t) lines.push(t); line = ""; };
    const walk = n => {
      if (n.nodeType === 3) { line += n.data.replace(/\s+/g, " "); return; }
      if (n.nodeType !== 1) return;
      if (n.matches && n.matches(skip)) return;
      if (/(^|;)\s*display\s*:\s*none\b/i.test(n.getAttribute ? n.getAttribute("style") || "" : "")) return;
      const tag = n.tagName;
      if (tag === "DETAILS" && !(n.hasAttribute ? n.hasAttribute("open") : n.open)) {
        // 닫힌 접기는 제목(summary)만 보인다
        for (const k of n.children) if (k.tagName === "SUMMARY") { flush(); walk(k); flush(); }
        return;
      }
      if (tag === "BR") { flush(); return; }
      if (tag === "IMG") { const a = clean(n.getAttribute("alt")); if (a) line += " [그림: " + a + "] "; return; }
      if (tag === "PRE") {
        flush();
        const code = n.querySelector("code") || n;
        const t = String(code.textContent || "").replace(/\n$/, "");
        if (t) lines.push(t);
        return;
      }
      if (tag === "TR") {
        flush();
        const cells = [];
        for (const c of n.children) if (c.tagName === "TD" || c.tagName === "TH") cells.push(clean(nodeText(c, skip).replace(/\n/g, " ")));
        if (cells.some(Boolean)) lines.push("| " + cells.join(" | ") + " |");
        return;
      }
      if (tag === "LI") {
        flush();
        line = "- ";
        for (const k of n.childNodes) walk(k);
        flush();
        line = ""; // 빈 항목의 머리는 버린다
        return;
      }
      const block = BLOCK.test(tag);
      if (block) flush();
      for (const k of n.childNodes) walk(k);
      if (block) flush();
    };
    walk(el);
    flush();
    return lines.join("\n");
  }
  // htmlText 는 노트 HTML 의 글자다 — DOMParser 문서(inert — 그림을 받지 않고 스크립트를 돌리지 않는다)에서 nodeText 로.
  function htmlText(html) {
    if (!html || !root.DOMParser) return "";
    const d = new root.DOMParser().parseFromString("<body>" + html + "</body>", "text/html");
    return nodeText(d.body);
  }

  // slideTitle 은 장의 제목이다(present.js slideTitle 과 같은 판정 — 「(계속 k/n)」 표지는 뺀다). 없으면 "".
  function slideTitle(pg) {
    if (!pg || !pg.querySelector) return "";
    const kind = pg.dataset ? pg.dataset.kind : "";
    if (kind === "title") return clean((pg.querySelector(".sc-title") || {}).textContent);
    if (kind === "section") {
      const sec = pg.querySelector(".slide-section");
      if (!sec) return "";
      const num = clean((sec.querySelector(".ss-num:not(.auto)") || {}).textContent);
      const t = clean((sec.querySelector(".ss-title") || {}).textContent);
      return num && t ? num + " " + t : t;
    }
    const h = pg.querySelector(".slide-head .slide-title");
    if (!h) return "";
    const c = h.cloneNode(true);
    for (const x of c.querySelectorAll(".slide-cont, .slide-seq")) x.remove();
    return clean(c.textContent);
  }
  // slideText 는 장의 화면 글자다 — 본문(.page-body)과 패널·배경 칸(.slide-panel·.slide-bg)의 글자. 제목 머리(.slide-head)는 제목 줄로 따로 쓴다.
  // c(켬 설정 — 기본 지금 것)의 notes 가 거짓이면 각주 목록(FOOTNOTES)도 통째로 뺀다.
  function slideText(pg, c) {
    if (!pg || !pg.querySelectorAll) return "";
    const skip = skipFor(c === undefined ? cur : c);
    const parts = [];
    for (const x of pg.querySelectorAll(":scope > .slide-panel, :scope > .page-body, :scope > .slide-bg")) {
      const t = nodeText(x, skip);
      if (t) parts.push(t);
    }
    return parts.join("\n");
  }
  // pageOf 는 el 이 든 슬라이드 장(.page.slide)이다(없으면 null).
  function pageOf(el) {
    for (let e = el; e; e = e.parentElement) if (e.classList && e.classList.contains("page") && e.classList.contains("slide")) return e;
    return null;
  }
  function slideIndex(pg) { const n = parseInt(pg && pg.dataset ? pg.dataset.slide : "", 10); return n > 0 ? n - 1 : -1; }
  // slideURL 은 공개 묶음의 슬라이드 주소다 — 지금 주소(# 앞)에 그 장 또는 앞 장의 제목 id(#…)를 붙인다. 발표 덱의 장은 id 를 뗀 사본이라
  // 미리보기(#pages)의 같은 번호 장에서 찾는다. 제목 id 가 없으면 지금 주소만.
  function slideURL(pg) {
    const loc = root.location;
    if (!loc || !loc.href) return "";
    const base = String(loc.href).split("#")[0];
    const doc = root.document;
    const i = slideIndex(pg);
    let src = pg;
    if (doc && doc.querySelector && i >= 0) {
      const p = doc.querySelector('#pages > .page[data-slide="' + (i + 1) + '"]');
      if (p) src = p;
    }
    for (let p = src; p; p = p.previousElementSibling) {
      if (!p.classList || !p.classList.contains("page")) continue;
      const t = p.querySelector(".slide-title[id], .slide-section [id], h1[id], h2[id], h3[id]");
      if (t && t.id) return base + "#" + encodeURIComponent(t.id);
    }
    return base;
  }

  // ---- 질문 글

  function head(c, extra) {
    return [c.course, c.doc, extra].map(clean).filter(Boolean).join(" · ");
  }
  function docName(c) { return c.doc || clean(host.title); }
  // osLine 은 OS 줄이다 — 블록 OS·브라우저 OS 가 둘 다 있으면 「이 명령 블록: … / 내 PC: … (브라우저 추정)」, 하나면 그것만, 없으면 "".
  function osLine(block, browser) {
    const parts = [];
    if (block) parts.push("이 명령 블록: " + block);
    if (browser) parts.push("내 PC: " + browser + " (브라우저 추정)");
    return parts.join(" / ");
  }
  // codeQuestion 은 코드 블록 질문 글이다. o = { code, title(블록 title), blockOS, browserOS, slide(슬라이드 제목) }, c = 켬 설정.
  function codeQuestion(o, c) {
    c = c || cur || {};
    const h = head({ course: c.course, doc: docName(c) }, o.slide);
    const lines = [];
    lines.push((h ? "「" + h + "」 " : "") + "실습 중인 수강생입니다.");
    const t = clean(o.title);
    lines.push("아래 명령" + (t ? "(" + t + ")" : "") + "이 무엇을 하는지 한 줄씩 설명해 주세요.");
    lines.push("옵션의 뜻, 실행하면 보일 출력, 흔한 오류와 확인 방법도 알려 주세요.");
    const os = osLine(o.blockOS, o.browserOS);
    if (os) lines.push(os);
    lines.push("---");
    lines.push(String(o.code || "").replace(/\n$/, ""));
    return lines.join("\n");
  }
  // slideQuestion 은 슬라이드 질문 글이다. o = { title, text(화면 글자), notes(노트 글 — 실은 렌더일 때만 호출자가 준다), url(공개 묶음일 때만) }.
  function slideQuestion(o, c) {
    c = c || cur || {};
    const h = head({ course: c.course, doc: docName(c) });
    const t = clean(o.title);
    const lines = [];
    lines.push((h ? "「" + h + "」의 " : "") + "슬라이드" + (t ? " 「" + t + "」" : "") + "입니다. 처음 배우는 사람 기준으로 풀어 설명하고,");
    lines.push("이해했는지 확인할 질문 2개를 내 주세요.");
    lines.push("---");
    if (o.text) lines.push(o.text);
    if (o.notes) { lines.push(""); lines.push("[발표자 노트]"); lines.push(o.notes); }
    if (o.url) { lines.push(""); lines.push("슬라이드 주소: " + o.url); }
    return lines.join("\n");
  }
  // forSlide 는 장 pg 의 질문 글이다 — 노트는 cfg.notes 일 때만(host.notes), 주소는 공개 묶음일 때만.
  function forSlide(pg, c) {
    c = c || cur || {};
    let notes = "";
    if (c.notes && typeof host.notes === "function") {
      const i = slideIndex(pg);
      if (i >= 0) notes = htmlText(host.notes(i) || "");
    }
    return slideQuestion({ title: slideTitle(pg), text: slideText(pg, c), notes, url: c.public ? slideURL(pg) : "" }, c);
  }
  // forCode 는 코드 블록 pre(조각이면 첫 조각)와 그 블록 글자(code — 조각을 모은 것)의 질문 글이다.
  function forCode(pre, code, c) {
    return codeQuestion({ code, title: pre.getAttribute("data-title") || "", blockOS: blockOS(pre), browserOS: browserOS(), slide: slideTitle(pageOf(pre)) }, c);
  }

  // askURL 은 서비스 svc 의 새 대화 주소다 — { url, overflow }. 넘치면 url 은 빈 새 대화 주소다.
  function askURL(svc, q) {
    const s = SERVICES[svc];
    if (!s) return null;
    const u = s.q + encodeURIComponent(q);
    return u.length > LIMIT ? { url: s.base, overflow: true } : { url: u, overflow: false };
  }

  // ---- 넘침 안내(NOTE) — 오래 남는 상자(사람이 닫을 때까지). 발표 중에도 보이게 겹침 층(data-pr-overlay — present.js 머리 주석 「호스트 계약」:
  // 발표의 숨김에서 빠지고, 그 안의 누르기는 넘김·판서가 아니다)으로 body 의 자식에 단다. 안: 알림 글(.itda-asknote-msg), 실패면 직접 골라 복사할
  // 질문 글 칸(textarea.itda-asknote-text — readonly), 「다시 복사」(.itda-asknote-copy — 누르기 안에서 다시 쓴다)·「닫기」(.itda-asknote-close).
  // 단추 누르기는 handler 가 받는다(위임). 모양은 호스트 CSS(app.css). document.body 가 없으면(시험 대역 등) deps.toast 로 알린다.
  const NOTE = "itda-asknote";
  let pending = null; // 지금 안내의 { q, label, write } — 「다시 복사」 가 쓴다

  function okMsg(label) { return "질문 글이 길어 주소에 담지 못했습니다. 클립보드에 복사했으니 " + label + " 새 대화 입력칸에 붙여 넣으세요."; }
  function failMsg(err) {
    return "복사 실패 — 질문 글이 길어 주소에 담지 못했고 클립보드에도 넣지 못했습니다(" + String(err && err.message || err || "까닭 모름") +
      "). 「다시 복사」 를 누르거나 아래 글을 골라 복사해 새 대화 입력칸에 붙여 넣으세요.";
  }
  function noteBox() {
    const doc = root.document;
    return doc && doc.body ? doc.body.querySelector(":scope > ." + NOTE) : null;
  }
  // showNote 는 안내 상자를 그린다(있으면 고쳐 쓴다). failed 면 질문 글 칸을 보이고 고른다. 상자가 없고 못 만들면 거짓.
  function showNote(msg, failed) {
    const doc = root.document;
    if (!doc || !doc.body || typeof doc.createElement !== "function") return false;
    let box = noteBox();
    if (!box) {
      box = doc.createElement("div");
      box.classList.add(NOTE);
      box.setAttribute("data-pr-overlay", "");
      box.setAttribute("role", "alert");
      const m = doc.createElement("p");
      m.classList.add(NOTE + "-msg");
      const ta = doc.createElement("textarea");
      ta.classList.add(NOTE + "-text");
      ta.setAttribute("readonly", "");
      ta.setAttribute("aria-label", "질문 글");
      const row = doc.createElement("div");
      row.classList.add(NOTE + "-row");
      const copy = doc.createElement("button");
      copy.type = "button";
      copy.classList.add(NOTE + "-copy");
      copy.textContent = "다시 복사";
      const close = doc.createElement("button");
      close.type = "button";
      close.classList.add(NOTE + "-close");
      close.textContent = "닫기";
      row.appendChild(copy);
      row.appendChild(close);
      box.appendChild(m);
      box.appendChild(ta);
      box.appendChild(row);
      doc.body.appendChild(box);
    }
    box.querySelector("." + NOTE + "-msg").textContent = msg;
    box.classList.toggle("failed", !!failed);
    const ta = box.querySelector("." + NOTE + "-text");
    ta.hidden = !failed;
    ta.value = failed && pending ? pending.q : "";
    if (failed && typeof ta.select === "function") { try { ta.select(); } catch (e) { /* 고르기는 편의 */ } }
    return true;
  }
  function closeNote() {
    const box = noteBox();
    if (box) box.remove();
    pending = null;
  }
  // copyNow 는 pending 의 질문 글을 클립보드에 쓰고 결과를 안내한다(Promise). 안내 상자가 없으면 toast 로.
  function copyNow(toast) {
    const pd = pending;
    if (!pd) return Promise.resolve(false);
    let p;
    try { p = Promise.resolve(pd.write(pd.q)); } catch (err) { p = Promise.reject(err); }
    return p.then(() => {
      if (pending === pd && !showNote(okMsg(pd.label), false) && toast) toast(okMsg(pd.label));
      return true;
    }, err => {
      if (pending === pd && !showNote(failMsg(err), true) && toast) toast(failMsg(err));
      return false;
    });
  }

  // ask 는 질문 글 q 로 svc 의 새 대화를 연다. deps = { open(url)(기본 window.open(url, "_blank", "noopener")), write(text)(클립보드 — Promise,
  // 기본 navigator.clipboard.writeText), toast(글) }. 넘침이면 클립보드 쓰기를 먼저 시작한 뒤(사람의 누르기 안에서) 빈 새 대화를 열고, 결과를 오래
  // 남는 안내 상자(NOTE)에 보인다 — 실패면 「복사 실패」 와 다시 복사·직접 고를 글. 돌려주는 값은 askURL 의 것(넘침이면 promise 에 쓰기 결과).
  function ask(svc, q, deps) {
    deps = deps || {};
    const r = askURL(svc, q);
    if (!r) return null;
    const s = SERVICES[svc];
    const open = deps.open || (u => { if (root.open) root.open(u, "_blank", "noopener"); });
    if (r.overflow) {
      const w = deps.write || (t => root.navigator && root.navigator.clipboard ? root.navigator.clipboard.writeText(t) : Promise.reject(new Error("클립보드를 쓸 수 없음")));
      pending = { q, label: s.label, write: w };
      const p = copyNow(deps.toast);
      open(r.url);
      return Object.assign({}, r, { promise: p });
    }
    open(r.url);
    return r;
  }

  // ---- 슬라이드 단추

  function button(doc, svc, cls) {
    const s = SERVICES[svc];
    const b = doc.createElement("button");
    b.type = "button";
    b.classList.add(...cls);
    b.setAttribute("data-ask", svc);
    b.setAttribute("data-label", s.label);
    b.setAttribute("aria-label", s.name);
    b.title = s.name + " — 질문 글을 채운 새 대화를 새 탭으로 엽니다(보내기는 그 화면에서 직접)";
    b.innerHTML = SVG_ASK;
    return b;
  }
  // armSlides 는 box 의 장(:scope > .page.slide)마다 오른쪽 위 단추 막대를 단다(켜져 있을 때). 꺼졌으면 단 막대를 뗀다. 서비스 목록이 바뀌었으면
  // 다시 만든다(멱등 — 같으면 그대로). 막대는 장의 마지막 자식(본문 밖)이라 쪽 나눔·장 수에 끼지 않는다 — 장을 다 나눈 뒤에 부른다.
  function armSlides(box) {
    const doc = root.document;
    if (!box || !box.querySelectorAll || !doc) return 0;
    const want = cur ? cur.services.join(",") : "";
    let n = 0;
    for (const pg of box.querySelectorAll(":scope > .page.slide")) {
      let bar = null;
      for (const k of pg.children) if (k.classList && k.classList.contains(BAR)) { bar = k; break; }
      const skip = !want || footnoteOnly(pg, cur);
      if (bar && !skip && bar.getAttribute("data-services") === want) { n++; continue; }
      if (bar) bar.remove();
      if (skip) continue;
      bar = doc.createElement("div");
      bar.classList.add(BAR);
      bar.setAttribute("data-services", want);
      bar.setAttribute("role", "group");
      bar.setAttribute("aria-label", "AI 에게 이 슬라이드 묻기");
      for (const svc of cur.services) bar.appendChild(button(doc, svc, [BTN]));
      pg.appendChild(bar);
      n++;
    }
    return n;
  }
  // footnoteOnly 는 장 pg 가 각주뿐인 장(「주석」 장 — 각주를 빼면 화면 글자가 없다)이고 c 가 각주를 넣지 않는가다(cfg.notes 거짓). 그 장에는
  // 막대를 달지 않는다 — 누르면 제목뿐인 질문 글이 된다(A5).
  function footnoteOnly(pg, c) {
    return !!c && !c.notes && !!pg.querySelector(FOOTNOTES) && !slideText(pg, c);
  }
  // codeButtons 는 코드 블록용 단추들이다(codecopy.js 가 복사 단추 옆에 단다 — 클래스는 호출자가 준다).
  function codeButtons(doc, cls) {
    return cur ? cur.services.map(svc => button(doc, svc, cls)) : [];
  }

  // handler 는 슬라이드 단추와 넘침 안내 상자 단추의 위임 클릭 처리기다(문서에 한 번). deps 는 ask 의 것. 발표 화면(조상 .present)에서는 누른 뒤
  // 포커스를 단추에 남기지 않는다. 안내 상자의 「다시 복사」 는 이 누르기 안에서 다시 쓰고, 「닫기」 는 상자를 뗀다.
  function handler(deps) {
    return function (e) {
      const nb = e.target && e.target.closest ? e.target.closest("." + NOTE + " button") : null;
      if (nb) {
        e.preventDefault();
        e.stopPropagation();
        if (nb.classList.contains(NOTE + "-close")) closeNote();
        else if (nb.classList.contains(NOTE + "-copy")) copyNow(deps && deps.toast);
        return;
      }
      const b = e.target && e.target.closest ? e.target.closest("button." + BTN) : null;
      if (!b) return;
      const pg = pageOf(b);
      e.preventDefault();
      e.stopPropagation();
      if (b.closest(".present")) b.blur();
      if (!cur || !pg) return;
      ask(b.getAttribute("data-ask"), forSlide(pg), deps);
    };
  }

  root.ItdaAsk = { SERVICES, ORDER, LIMIT, BAR, BTN, SVG_ASK, normalize, set, config, osOfTitle, blockOS, browserOS, partners, nodeText, htmlText,
    slideTitle, slideText, slideURL, pageOf, codeQuestion, slideQuestion, forSlide, forCode, askURL, ask, armSlides, codeButtons, handler,
    NOTE, FOOTNOTES, closeNote, copyNow, footnoteOnly };
})(typeof window !== "undefined" ? window : globalThis);
