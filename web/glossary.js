// 게시 강의 화면의 용어사전(itda-work/itda-hyve#1084 갈래 A 2~3, itda-doc#135). 호스트(itda-hyve md pack 의 packglossary.js)가 판의 glossary.json 을
// 받아 index(data) 로 색인을 만들고, 본문을 다 그린 뒤 markPages 로 사전의 표기에 점선 밑줄(span.itda-term)을 단다. 밑줄을 누르면 popup 이 풀이·자세히 다룬 곳·
// 「Claude 로 추가 설명받기 ▾」 를 띄운다. 이 파일은 받아 오지도 보내지도 않는다 — 여는 곳 선택만 그 브라우저의 localStorage(PREF_KEY)에 둔다.
//
// 맞춤 규칙(scan — 시험 testdata/glossary.cjs 가 표로 묶는다):
//   - 표기(term·aliases)와 글자가 정확히 같을 때만(대소문자 구분). 한 자리에서 긴 표기를 먼저 본다(「Claude Desktop」 안의 「Claude」 에 따로 긋지 않는다).
//   - 앞 경계: 바로 앞 글자가 한글·영문·숫자가 아니다(「전화대화」 의 「대화」 는 아니다). 뒤 경계: 표기가 영문·숫자로 끝나면 바로 뒤 글자가 영문·숫자가 아니다
//     (「Skills」 의 「Skill」 은 아니다). 한글로 끝나는 표기는 조사가 붙으므로 뒤 경계를 두지 않는다 — 「대화상자」 의 「대화」 에도 붙는다(받아들인 한계).
//   - 앞뒤 글자는 같은 덩어리(문단·목록 항목·표 칸 따위 — BLOCK) 안의 이웃 글자 마디까지 본다(「<b>스킬</b>을」·「A<b>스킬</b>」). 표기 하나가 여러 글자 마디에
//     걸치면(「Claude <b>Code</b>」) 맞추지 않는다.
// 긋는 곳(markPages): 「쪽」 마다 같은 용어는 처음 한 번 — 슬라이드 보기는 슬라이드 한 장(과 그 장의 노트 칸 — 노트는 수강생도 읽는다), 그 밖의 보기는 제목(h1~h6)으로
// 나뉜 구역(문서 차례로 이어 가다 제목을 만나면 새 구역). 빼는 곳(skipEl): 코드(code·pre·kbd·samp)·제목·링크·단추·입력·수식(math)·그림(svg·canvas·img·
// mermaid)·이미 밑줄 친 곳(u·ins·abbr·span.itda-term)·쪽 머리·꼬리말·슬라이드 머리·꼬리·표지·장 표지·목차·숨긴 것(aria-hidden)·단추 역할(role=button).
// 다시 그려도 겹쳐 긋지 않는다 — 이미 단 밑줄(span.itda-term)을 「본 용어」 로 세고 그 속은 건너뛴다. 밑줄은 글자 마디를 쪼개 span 으로 감쌀 뿐이라
// 덩어리의 textContent·블록 번호(data-n)는 그대로다.
(function (root) {
  "use strict";

  const PREF_KEY = "itda-glossary-open";
  const MODES = ["web", "app", "copy"];
  const URLS = { web: "https://claude.ai/new?q=", app: "claude://claude.ai/new?q=" };
  const MODE_NAME = { web: "웹(claude.ai)", app: "데스크톱 앱", copy: "질문만 복사" };
  const ALNUM = /[A-Za-z0-9]/;
  const HANGUL = /[ᄀ-ᇿ㄰-㆏ꥠ-꥿가-힯ힰ-퟿]/;
  const wordChar = c => !!c && (ALNUM.test(c) || HANGUL.test(c));

  // index 는 glossary.json(pack_glossary.go glossaryOut — {format:1, terms:[{term, aliases, def, see?:{doc, hash}}]})을 색인으로 바꾼다. 꼴이 틀리면 null(화면은 조용히 끈다).
  // 같은 표기가 둘이면 앞 용어가 갖는다(hyve 는 묶을 때 막는다 — 여기서는 기대지 않는다).
  function index(data) {
    if (!data || data.format !== 1 || !Array.isArray(data.terms) || !data.terms.length) return null;
    const terms = [], forms = [], seen = new Set();
    for (const t of data.terms) {
      if (!t || typeof t.term !== "string" || !t.term.trim() || typeof t.def !== "string") return null;
      const aliases = t.aliases === undefined ? [] : t.aliases;
      if (!Array.isArray(aliases) || !aliases.every(a => typeof a === "string" && a.trim())) return null;
      let see = null;
      if (t.see !== undefined && t.see !== null) {
        if (typeof t.see !== "object" || typeof t.see.doc !== "string" || !t.see.doc || typeof t.see.hash !== "string") return null;
        see = { doc: t.see.doc, hash: t.see.hash };
      }
      const i = terms.length;
      terms.push({ term: t.term, aliases: aliases.slice(), def: t.def, see });
      for (const f of [t.term, ...aliases]) {
        if (seen.has(f)) continue;
        seen.add(f);
        forms.push({ text: f, t: i });
      }
    }
    forms.sort((a, b) => b.text.length - a.text.length); // 안정 정렬 — 길이가 같으면 사전 차례
    const byFirst = new Map();
    for (const f of forms) {
      const k = f.text[0];
      if (!byFirst.has(k)) byFirst.set(k, []);
      byFirst.get(k).push(f);
    }
    return { terms, forms, byFirst };
  }

  // scan 은 글 text 안에서 표기가 맞는 자리들이다 — [{start, end, t(용어 번호)}], 겹치지 않고 왼쪽부터. before·after 는 text 바로 앞·뒤 글자(같은 덩어리의 이웃 글자 마디 — 없으면 "").
  function scan(text, g, before, after) {
    const out = [];
    let i = 0;
    while (i < text.length) {
      const list = g.byFirst.get(text[i]);
      let hit = null;
      if (list && !wordChar(i > 0 ? text[i - 1] : before || "")) {
        for (const f of list) {
          if (!text.startsWith(f.text, i)) continue;
          const end = i + f.text.length;
          if (ALNUM.test(f.text[f.text.length - 1]) && ALNUM.test(end < text.length ? text[end] : after || "")) continue;
          hit = { start: i, end, t: f.t };
          break;
        }
      }
      if (hit) { out.push(hit); i = hit.end; } else i++;
    }
    return out;
  }

  const SKIP_TAGS = new Set(["CODE", "PRE", "KBD", "SAMP", "TT", "VAR", "H1", "H2", "H3", "H4", "H5", "H6", "A", "BUTTON", "INPUT", "TEXTAREA", "SELECT", "OPTION",
    "LABEL", "SUMMARY", "MATH", "SVG", "CANVAS", "IMG", "PICTURE", "VIDEO", "AUDIO", "IFRAME", "SCRIPT", "STYLE", "NOSCRIPT", "TEMPLATE", "U", "INS", "ABBR", "NAV"]);
  const SKIP_CLASSES = ["itda-term", "page-header", "page-footer", "slide-head", "slide-foot", "slide-cover", "slide-section", "slide-deco", "toc", "sn-none", "sn-grip",
    "memo-mark", "itda-codecopy", "itda-codetitle", "itda-section-ask", "math-error", "math-display", "math-inline"];
  const BLOCK = new Set(["P", "LI", "DT", "DD", "TD", "TH", "TR", "TABLE", "THEAD", "TBODY", "UL", "OL", "DL", "DIV", "BLOCKQUOTE", "FIGURE", "FIGCAPTION", "SECTION",
    "ARTICLE", "ASIDE", "HEADER", "FOOTER", "MAIN", "DETAILS", "BR", "HR", "CAPTION", "H1", "H2", "H3", "H4", "H5", "H6", "PRE"]);
  const HEADING = /^H[1-6]$/;
  const tagOf = el => String(el.tagName || "").toUpperCase();
  const classesOf = el => String(el.getAttribute && el.getAttribute("class") || "").split(/\s+/).filter(Boolean);

  // skipEl 은 그 요소의 속을 밑줄 후보에서 빼는가다(머리 주석 「빼는 곳」).
  function skipEl(el) {
    if (SKIP_TAGS.has(tagOf(el))) return true;
    const cls = classesOf(el);
    if (cls.some(c => SKIP_CLASSES.includes(c) || c.includes("mermaid"))) return true;
    const ga = n => el.getAttribute ? el.getAttribute(n) : null;
    return ga("role") === "button" || ga("aria-hidden") === "true" || ga("contenteditable") !== null && ga("contenteditable") !== "false" || ga("data-mermaid") !== null;
  }

  // segments 는 el 아래를 문서 차례로 훑어 [{kind:"text", node, ok(후보인가), block} | {kind:"seen", t} | {kind:"reset"}] 을 out 에 더한다.
  // block 은 덩어리 번호다 — 덩어리 요소(BLOCK)에 들고 날 때마다 바뀐다. reset 은 제목을 만난 자리다(구역 나눔). seen 은 이미 단 밑줄이다.
  function segments(el, out, st, skipped) {
    for (const n of Array.from(el.childNodes || [])) {
      if (n.nodeType === 3) {
        if (n.data) out.push({ kind: "text", node: n, ok: !skipped, block: st.block });
        continue;
      }
      if (n.nodeType !== 1) continue;
      const tag = tagOf(n), blk = BLOCK.has(tag);
      if (blk) st.block++;
      if (HEADING.test(tag)) out.push({ kind: "reset" });
      const cls = classesOf(n);
      if (cls.includes("itda-term")) {
        const t = Number(n.getAttribute("data-term"));
        if (Number.isInteger(t)) out.push({ kind: "seen", t });
      }
      segments(n, out, st, skipped || skipEl(n));
      if (blk) st.block++;
    }
  }

  // markTree 는 el 아래 글에 밑줄을 단다. ctx = { seen: Set(이 쪽에서 본 용어), sections: 제목에서 seen 을 비울지 }. 단 개수를 돌려준다.
  function markTree(el, g, doc, ctx) {
    const segs = [];
    segments(el, segs, { block: 0 }, skipEl(el));
    let count = 0;
    for (let k = 0; k < segs.length; k++) {
      const s = segs[k];
      if (s.kind === "reset") { if (ctx.sections) ctx.seen.clear(); continue; }
      if (s.kind === "seen") { ctx.seen.add(s.t); continue; }
      if (!s.ok) continue;
      const text = s.node.data;
      const nb = j => { const o = segs[j]; return o && o.kind === "text" && o.block === s.block ? o.node.data : null; };
      let before = "", after = "";
      for (let j = k - 1; j >= 0 && segs[j].kind === "text"; j--) { const v = nb(j); if (v === null) break; if (v) { before = v[v.length - 1]; break; } }
      for (let j = k + 1; j < segs.length && segs[j].kind === "text"; j++) { const v = nb(j); if (v === null) break; if (v) { after = v[0]; break; } }
      const hits = scan(text, g, before, after).filter(h => {
        if (ctx.seen.has(h.t)) return false;
        ctx.seen.add(h.t);
        return true;
      });
      if (!hits.length) continue;
      const parent = s.node.parentNode;
      let at = 0;
      for (const h of hits) {
        if (h.start > at) parent.insertBefore(doc.createTextNode(text.slice(at, h.start)), s.node);
        parent.insertBefore(termSpan(doc, text.slice(h.start, h.end), h.t), s.node);
        at = h.end;
        count++;
      }
      if (at < text.length) parent.insertBefore(doc.createTextNode(text.slice(at)), s.node);
      parent.removeChild(s.node);
    }
    return count;
  }

  function termSpan(doc, text, t) {
    const s = doc.createElement("span");
    s.setAttribute("class", "itda-term");
    s.setAttribute("data-term", String(t));
    s.setAttribute("role", "button");
    s.setAttribute("tabindex", "0");
    s.setAttribute("aria-haspopup", "dialog");
    s.setAttribute("aria-expanded", "false");
    s.appendChild(doc.createTextNode(text));
    return s;
  }

  const kids = (el, cls) => Array.from(el && el.children || []).filter(x => classesOf(x).includes(cls));

  // markPages 는 그린 본문(#pages)과 노트 칸(#notes)에 밑줄을 단다(머리 주석 「긋는 곳」). look 은 share.js 의 보기 꼴(slides·doc·flow·dashboard). 단 개수를 돌려준다.
  function markPages(pages, notes, g, doc, look) {
    if (!pages || !g) return 0;
    let count = 0;
    const list = kids(pages, "page");
    if (look === "slides") {
      const nts = kids(notes, "slide-note");
      list.forEach((pg, i) => {
        const ctx = { seen: new Set(), sections: false };
        count += markTree(pg, g, doc, ctx);
        const nt = nts.find(x => x.getAttribute("data-slide") === String(i + 1));
        if (nt) count += markTree(nt, g, doc, ctx);
      });
      return count;
    }
    const ctx = { seen: new Set(), sections: true };
    for (const pg of list) count += markTree(pg, g, doc, ctx);
    return count;
  }

  // ---- 여는 곳 선택(이 브라우저의 localStorage 에만 — 강의와 무관한 키 하나)

  function getMode(storage) {
    try { const m = storage && storage.getItem(PREF_KEY); return MODES.includes(m) ? m : "web"; } catch (e) { return "web"; }
  }
  function setMode(storage, m) {
    if (!MODES.includes(m)) return;
    try { if (storage) storage.setItem(PREF_KEY, m); } catch (e) { /* 이 화면에서만 */ }
  }
  // askURL 은 여는 곳 mode(web·app)로 질문 q 를 미리 채운 새 대화 주소다 — 입력칸만 채우고 보내지 않는다(itda-hyve docs/field-test/ask-ai-url.md).
  function askURL(mode, q) { return URLS[mode] ? URLS[mode] + encodeURIComponent(q) : ""; }
  // claudeOn 은 ask 설정(ItdaAsk.normalize 가 돌려준 것)에 Claude 가 켜졌는가다 — ChatGPT 만 켠 강의는 이 단추를 숨긴다(#1084 열린 질문).
  function claudeOn(cfg) { return !!(cfg && Array.isArray(cfg.services) && cfg.services.includes("claude") && cfg.mcp); }

  // place 는 팝업의 화면 자리다(DOM 없음): r 은 밑줄의 화면 사각형, w·h 는 팝업 크기, vw·vh 는 창 크기. 아래에 두되 자리가 없으면 위로, 어느 쪽도 다 들어가지
  // 않으면 창 안으로 민다 — 좁은 화면에서도 화면 밖으로 나가지 않는다(여백 8px).
  function place(r, w, h, vw, vh) {
    const M = 8, G = 6;
    const left = Math.max(M, Math.min(r.left, vw - w - M));
    let top = r.bottom + G;
    if (top + h > vh - M) top = r.top - G - h >= M ? r.top - G - h : Math.max(M, vh - h - M);
    return { left: Math.round(left), top: Math.round(top) };
  }

  // popup 은 풀이 팝업이다. o = { doc, win, g(index 결과), ask()(지금 ask 설정 — ItdaAsk.normalize 결과 또는 null), question(cfg, term)(ItdaAsk.termQuestion),
  // seeHref(see)({href, same} — same 이면 이 문서 안 조각), jump(href)(같은 문서 #조각으로), copy(text)(Promise<bool>), toast(msg), storage()(localStorage — 못 쓰면 null) }.
  // { open(span), close(focusBack), isOpen(), toggle(span), el, onClick(e), onKey(e), onPointer(e), onFocus(e) } 를 돌려준다 — 사건 배선은 install.
  // 팝업은 body 끝의 div 하나(role=dialog, 처음 열 때 만든다)라 본문(#pages)·발표 덱 사본·쪽 나눔에 들지 않는다.
  function popup(o) {
    const { doc, win } = o;
    let el = null, cur = null, menuOpen = false;
    const E = (tag, cls, text) => { const x = doc.createElement(tag); if (cls) x.setAttribute("class", cls); if (text) x.textContent = text; return x; };
    const parts = {};
    function build() {
      el = E("div", "itda-gloss-pop");
      el.setAttribute("role", "dialog");
      el.setAttribute("aria-labelledby", "itda-gloss-term");
      el.setAttribute("tabindex", "-1");
      el.hidden = true;
      const close = E("button", "ig-close", "×");
      close.type = "button";
      close.setAttribute("aria-label", "닫기");
      close.setAttribute("title", "닫기(Esc)");
      parts.term = E("p", "ig-term");
      parts.term.id = "itda-gloss-term";
      parts.alias = E("p", "ig-alias");
      parts.def = E("p", "ig-def");
      parts.see = E("p", "ig-see");
      parts.seeLink = E("a", "ig-see-a", "자세히 다룬 곳 보기 →");
      parts.see.appendChild(parts.seeLink);
      parts.ask = E("div", "ig-ask");
      parts.go = E("button", "ig-ask-go", "Claude 로 추가 설명받기");
      parts.go.type = "button";
      parts.more = E("button", "ig-ask-more", "▾");
      parts.more.type = "button";
      parts.more.setAttribute("aria-expanded", "false");
      parts.more.setAttribute("aria-controls", "itda-gloss-open");
      parts.more.setAttribute("aria-label", "여는 곳 고르기");
      parts.more.setAttribute("title", "여는 곳 고르기");
      // 여는 곳 목록은 메뉴(role=menu)가 아니라 링크·단추의 묶음이다 — 메뉴 역할은 화살표·Home·End·글자 찾기와 초점 관리를 약속하는데, 묶음이면 Tab 으로
      // 차례로 닿고 Enter·Space 는 링크·단추의 기본 동작이라 따로 다룰 키가 없다(더 단순). 지금 고른 것은 aria-current 와 ✓(glossary.css)로 보인다.
      parts.menu = E("div", "ig-ask-menu");
      parts.menu.id = "itda-gloss-open";
      parts.menu.setAttribute("role", "group");
      parts.menu.setAttribute("aria-label", "여는 곳");
      parts.menu.hidden = true;
      parts.items = {};
      for (const [m, label] of [["web", "웹에서 열기"], ["app", "데스크톱 앱에서 열기"], ["copy", "질문만 복사"]]) {
        const it = E(m === "copy" ? "button" : "a", "ig-ask-item", label);
        if (m === "copy") it.type = "button";
        if (m === "web") { it.setAttribute("target", "_blank"); it.setAttribute("rel", "noopener noreferrer"); }
        it.setAttribute("data-mode", m);
        parts.items[m] = it;
        parts.menu.appendChild(it);
      }
      parts.note = E("p", "ig-ask-note", "질문은 Claude 의 새 대화 입력칸에만 채워집니다 — 보내기는 직접 누르세요.");
      const row = E("div", "ig-ask-row");
      row.appendChild(parts.go);
      row.appendChild(parts.more);
      parts.ask.appendChild(row);
      parts.ask.appendChild(parts.menu);
      parts.ask.appendChild(parts.note);
      for (const x of [close, parts.term, parts.alias, parts.def, parts.see, parts.ask]) el.appendChild(x);
      parts.close = close;
      doc.body.appendChild(el);
    }
    function setMenu(on) {
      menuOpen = !!on && !!el && !parts.ask.hidden;
      if (!el) return;
      parts.menu.hidden = !menuOpen;
      parts.more.setAttribute("aria-expanded", String(menuOpen));
      reposition(); // 메뉴를 펴고 접으면 높이가 바뀐다 — 아래에 못 들면 위로, 위아래 다 모자라면 창 안에 맞추고 팝업 안에서 굴린다(CSS max-height)
    }
    // reposition 은 열린 팝업을 지금 밑줄·지금 높이·지금 창 크기로 다시 놓는다(place). 닫혔으면 아무것도 하지 않는다.
    function reposition() {
      if (!el || el.hidden || !cur) return;
      const p = place(cur.getBoundingClientRect(), el.offsetWidth, el.offsetHeight, win.innerWidth, win.innerHeight);
      el.style.left = p.left + "px"; // CSSOM 대입 — style 속성 글이 아니다(CSP style-src 'self')
      el.style.top = p.top + "px";
    }
    function paintMode() {
      const mode = getMode(o.storage());
      for (const m of MODES) { if (m === mode) parts.items[m].setAttribute("aria-current", "true"); else parts.items[m].removeAttribute("aria-current"); }
      parts.go.setAttribute("title", "누르면 " + MODE_NAME[mode] + (mode === "copy" ? "" : "에서 엽니다") + " — ▾ 로 바꾸기");
      parts.go.setAttribute("data-mode", mode);
    }
    function fill(t) {
      const e = o.g.terms[t];
      parts.term.textContent = e.term;
      parts.alias.textContent = e.aliases.length ? "다른 표기: " + e.aliases.join(", ") : "";
      parts.alias.hidden = !e.aliases.length;
      parts.def.textContent = e.def;
      const see = e.see ? o.seeHref(e.see) : null;
      parts.see.hidden = !see;
      if (see) { parts.seeLink.setAttribute("href", see.href); parts.seeLink.setAttribute("data-same", see.same ? "1" : ""); }
      else parts.seeLink.removeAttribute("href");
      const cfg = o.ask();
      const on = claudeOn(cfg);
      parts.ask.hidden = !on;
      if (on) {
        const q = o.question(cfg, e.term);
        parts.q = q;
        parts.items.web.setAttribute("href", askURL("web", q));
        parts.items.app.setAttribute("href", askURL("app", q));
        paintMode();
      } else parts.q = "";
    }
    function open(span) {
      if (!el) build();
      if (cur && cur !== span) cur.setAttribute("aria-expanded", "false");
      const t = Number(span.getAttribute("data-term"));
      if (!o.g.terms[t]) return;
      cur = span;
      fill(t);
      setMenu(false);
      el.hidden = false;
      span.setAttribute("aria-expanded", "true");
      reposition();
      try { el.focus({ preventScroll: true }); } catch (e) { /* 초점 없음 */ }
    }
    function close(focusBack) {
      if (!el || el.hidden) return;
      setMenu(false);
      el.hidden = true;
      const s = cur;
      cur = null;
      if (s) {
        s.setAttribute("aria-expanded", "false");
        if (focusBack && s.isConnected !== false) { try { s.focus({ preventScroll: true }); } catch (e) { /* 없음 */ } }
      }
    }
    const isOpen = () => !!el && !el.hidden;
    function toggle(span) { if (isOpen() && cur === span) close(true); else open(span); }
    function act(mode, viaItem) {
      setMode(o.storage(), mode);
      paintMode();
      setMenu(false);
      if (mode === "copy") {
        Promise.resolve(o.copy(parts.q)).then(ok => o.toast(ok ? "질문을 복사했습니다 — Claude 새 대화에 붙여 넣으세요" : "복사하지 못했습니다"));
        return;
      }
      if (!viaItem) parts.items[mode].click(); // 단추는 고른 방식의 메뉴 링크를 대신 누른다 — 주소는 그 링크의 href 하나다
    }
    // onClick 은 window 붙잡기 단계의 click 이다. 팝업 안이면 여기서 다루고(링크의 기본 동작은 두고) 참, 밑줄이면 여닫고 참, 그 밖이면 거짓.
    function onClick(e) {
      const t = e.target;
      if (el && el.contains(t)) {
        const c = x => t.closest ? t.closest(x) : null;
        if (c(".ig-close")) { close(true); return true; }
        if (c(".ig-ask-more")) { setMenu(!menuOpen); return true; }
        if (c(".ig-ask-go")) { act(getMode(o.storage()), false); return true; }
        const it = c(".ig-ask-item");
        if (it) { act(it.getAttribute("data-mode"), true); if (it.getAttribute("data-mode") === "copy") e.preventDefault(); return true; }
        const see = c(".ig-see-a");
        if (see && see.getAttribute("data-same") === "1") { e.preventDefault(); const h = see.getAttribute("href"); close(false); o.jump(h.slice(h.indexOf("#"))); return true; }
        if (see) { close(false); return true; } // 다른 교안 — 브라우저가 같은 탭에서 옮긴다
        if (!c(".ig-ask")) setMenu(false);
        return true;
      }
      const span = t && t.closest ? t.closest(".itda-term") : null;
      if (!span || (span.closest && span.closest("#present"))) return false;
      const sel = win.getSelection && win.getSelection();
      if (sel && !sel.isCollapsed && String(sel).trim()) return false; // 글을 고르는 중
      toggle(span);
      return true;
    }
    // focusables 는 팝업 안에서 Tab 이 닿는 것들의 차례다(DOM 차례 — 닫기·자세히·Claude 단추·▾·펴진 목록의 셋).
    function focusables() {
      const f = [parts.close];
      if (!parts.see.hidden) f.push(parts.seeLink);
      if (!parts.ask.hidden) { f.push(parts.go, parts.more); if (menuOpen) f.push(parts.items.web, parts.items.app, parts.items.copy); }
      return f;
    }
    // onKey 는 window 붙잡기 단계의 keydown 이다. 돌려주는 값(호출자 install 이 나눠 다룬다):
    //   "own"    — 여기서 다룬 키: 기본 동작을 막고 전파를 멈춘다(Esc, 밑줄의 Enter·Space, 팝업 끝에서 밖으로 나가는 Tab).
    //   "inside" — 팝업 안의 그 밖의 키: 전파만 멈춘다 — 본문 전역 단축키(share.js keyHandler·pager navKey·발표 키)로 가지 않게(PageDown·Home·End·Space 가
    //              배경 교안을 넘기지 않는다). 팝업 안 굴리기·Tab 이동·단추와 링크 누르기 같은 기본 동작은 그대로 둔다.
    //   ""       — 본문의 키(건드리지 않는다).
    // Tab 이 팝업 밖으로 나가려 하면(마지막에서 Tab, 처음·팝업 자체에서 Shift+Tab) 가두지 않고 닫으며 초점을 연 밑줄로 돌린다 — 다음 Tab 은 본문의 다음 자리로 간다.
    function onKey(e) {
      const inPop = isOpen() && !!e.target && el.contains(e.target);
      if (e.isComposing) return inPop ? "inside" : "";
      if (e.key === "Escape" && isOpen()) {
        if (menuOpen) { setMenu(false); try { parts.more.focus(); } catch (x) { /* 없음 */ } } else close(true);
        return "own";
      }
      if (inPop) {
        if (e.key === "Tab" && !e.altKey && !e.ctrlKey && !e.metaKey) {
          const f = focusables();
          const out = e.shiftKey ? (e.target === el || e.target === f[0]) : e.target === f[f.length - 1];
          if (out) { close(true); return "own"; }
        }
        return "inside";
      }
      const span = e.target && e.target.closest ? e.target.closest(".itda-term") : null;
      if (span && (e.key === "Enter" || e.key === " ") && !e.metaKey && !e.ctrlKey && !e.altKey && !(span.closest && span.closest("#present"))) { toggle(span); return "own"; }
      return "";
    }
    // onPointer 는 붙잡기 단계의 pointerdown 이다 — 팝업·밑줄 밖을 누르면 닫는다(막지 않는다).
    function onPointer(e) {
      if (!isOpen()) return;
      const t = e.target;
      if (el.contains(t) || (t && t.closest && t.closest(".itda-term"))) return;
      close(false);
    }
    // onFocus 는 focusin 이다 — 초점이 팝업·지금 밑줄 밖의 요소로 옮겨 가면 닫는다.
    function onFocus(e) {
      if (!isOpen()) return;
      const t = e.target;
      if (el.contains(t) || t === cur) return;
      close(false);
    }
    return { open, close, isOpen, toggle, reposition, onClick, onKey, onPointer, onFocus, get el() { return el; } };
  }

  // SCROLL_SHUT 은 팝업을 연 뒤 보기 영역이 이만큼(px) 넘게 굴러야 닫는 문턱이다 — 누르기 직전의 굴림(눌러 보이게 하는 자동 굴림·관성)이 늦게 낸
  // scroll 사건이 방금 연 팝업을 닫지 않게.
  const SCROLL_SHUT = 24;

  // install 은 팝업의 사건을 단다(win·doc·viewer — 연 뒤 보기 영역을 SCROLL_SHUT 넘게 굴리면 닫는다 — 쪽 넘김, 창 크기가 바뀌면 다시 놓는다). 떼는 함수를 돌려준다.
  function install(p, win, doc, viewer) {
    let base = 0;
    const mark = () => { base = viewer ? viewer.scrollTop : 0; };
    const click = e => { if (p.onClick(e)) { e.stopPropagation(); mark(); } };
    // 키: "own" 은 기본 동작까지 막고, "inside"(팝업 안의 키)는 전파만 멈춰 팝업 안 굴리기·Tab·누르기를 살린다(popup onKey).
    const key = e => {
      const r = p.onKey(e);
      if (!r) return;
      e.stopPropagation();
      if (r === "own") { e.preventDefault(); mark(); }
    };
    const fit = () => p.reposition();
    const scrolled = () => { if (p.isOpen() && Math.abs(viewer.scrollTop - base) > SCROLL_SHUT) p.close(false); };
    win.addEventListener("click", click, true);
    win.addEventListener("keydown", key, true);
    doc.addEventListener("pointerdown", p.onPointer, true);
    doc.addEventListener("focusin", p.onFocus);
    win.addEventListener("resize", fit); // 창 크기가 바뀌면 다시 놓는다(본문을 다시 그리면 호스트가 닫는다)
    if (viewer) viewer.addEventListener("scroll", scrolled, { passive: true });
    return () => {
      win.removeEventListener("click", click, true);
      win.removeEventListener("keydown", key, true);
      doc.removeEventListener("pointerdown", p.onPointer, true);
      doc.removeEventListener("focusin", p.onFocus);
      win.removeEventListener("resize", fit);
      if (viewer) viewer.removeEventListener("scroll", scrolled);
    };
  }

  // copyText 는 글을 클립보드에 쓴다 — navigator.clipboard(보안 컨텍스트), 없으면 숨긴 textarea + execCommand. Promise<bool>.
  function copyText(win, doc, text) {
    const nav = win.navigator;
    if (nav && nav.clipboard && typeof nav.clipboard.writeText === "function") return nav.clipboard.writeText(text).then(() => true, () => legacy());
    return Promise.resolve(legacy());
    function legacy() {
      try {
        const ta = doc.createElement("textarea");
        ta.value = text;
        ta.setAttribute("readonly", "");
        ta.setAttribute("class", "ig-copy-tmp");
        doc.body.appendChild(ta);
        ta.select();
        const ok = doc.execCommand("copy");
        ta.remove();
        return !!ok;
      } catch (e) { return false; }
    }
  }

  root.ItdaGlossary = { PREF_KEY, SCROLL_SHUT, index, scan, skipEl, markTree, markPages, getMode, setMode, askURL, claudeOn, place, popup, install, copyText };
})(typeof window !== "undefined" ? window : globalThis);
