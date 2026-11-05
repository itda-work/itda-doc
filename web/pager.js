// 쪽 나눔기 — 문서 흐름의 블록을 A4 상자(.page-body)에 차례로 담고, 넘치면 나눈다(설계 4장).
// A4 인쇄 실측(docs/DESIGN-markdown-docs.md 4장)의 쪽 나눔 코드를 제품 수준으로 옮겼다. 실측이 남긴 한계를 메운다:
//   표는 행 단위(새 쪽마다 thead 반복), 목록·정의 목록·각주·중첩 컨테이너는 자식 단위(재귀),
//   문단은 글자 단위(공백에서 끊고 앞뒤 조각 최소 2줄 — 고아·과부 줄. 4줄 미만은 쪼개지 않는다), pre 는 줄 단위,
//   한 쪽보다 큰 그림은 줄여서, 한 쪽보다 큰 통짜 블록(다단·keep)은 풀어서 나눈다(잘리지 않게).
//   .page-break 는 새 쪽, .keep·그림·알림·결재란은 들어갈 쪽으로 통째로, 제목은 다음 블록과 함께.
// 조각마다 data-line 을 유지한다(3단계 스크롤 맞춤). requestAnimationFrame 을 쓰지 않는다 —
// 숨긴 창(내보내기)에서는 rAF 가 돌지 않는다(실측). 재기는 모두 getBoundingClientRect 로 동기다.
(function () {
  "use strict";

  const BLOCK_TAGS = new Set(["P", "UL", "OL", "DL", "TABLE", "PRE", "BLOCKQUOTE", "DIV", "FIGURE", "SECTION", "NAV", "HEADER", "H1", "H2", "H3", "H4", "H5", "H6", "HR"]);
  const isHeading = n => n.nodeType === 1 && /^H[1-6]$/.test(n.tagName);
  // 다음 것과 함께 다녀야 하는 것: 제목, 정의 목록의 용어, 목차 제목
  const keepWithNext = n => n.nodeType === 1 && (isHeading(n) || n.tagName === "DT" || n.classList.contains("toc-title"));
  // 들어갈 수 있는 쪽으로 통째로 옮길 것(그래도 한 쪽보다 크면 나눈다)
  const softKeep = n => n.nodeType === 1 && (n.classList.contains("keep") || n.tagName === "FIGURE" ||
    n.classList.contains("markdown-alert") || n.classList.contains("approval") || n.classList.contains("signature") ||
    n.classList.contains("title-block") || n.tagName === "BLOCKQUOTE" || n.classList.contains("box") || n.classList.contains("columns"));
  // 나누지 않는 것(쪽에 안 들어가면 통째로 다음 쪽)
  // 수식(1.10)은 블록(.math-display)이든 <math> 하나든 나누지 않는다 — MathML 의 자식을 쪽마다 나누면 반쪽 수식이 된다
  const atomic = n => n.nodeType !== 1 || isHeading(n) || ["FIGURE", "IMG", "HR", "TR", "SVG", "INPUT", "CAPTION"].includes(n.tagName) ||
    n.localName === "math" || n.classList.contains("math-display") ||
    n.classList.contains("columns") || n.classList.contains("approval") || n.classList.contains("signature") || n.classList.contains("cover");

  let pages, root, body, bodyBottom, opts, overflow, bodyHeight;

  function newPage(cover) {
    const pg = document.createElement("div");
    pg.className = "page" + (cover ? " page-cover" : "") + (opts && opts.pageClass ? " " + opts.pageClass : "");
    const hd = document.createElement("div"); hd.className = "page-header";
    const bd = document.createElement("div"); bd.className = "page-body";
    const ft = document.createElement("div"); ft.className = "page-footer";
    pg.append(hd, bd, ft);
    root.appendChild(pg);
    pages.push(pg);
    body = bd;
    const r = bd.getBoundingClientRect();
    bodyBottom = r.top + bd.clientHeight;
    bodyHeight = bd.clientHeight;
    // 한 쪽보다 큰 그림은 줄인다(캡션 자리를 남긴다). 쪽마다 같은 값이지만 용지 방향에 따라 다르다. 슬라이드의 캡션 없는 사진 그림·.fit
    // 그림은 base.css 가 이 값 대신 --body-h 로 상한을 정한다(itda-hyve#550 — slides.js markFigures).
    root.style.setProperty("--img-max-h", Math.max(80, bodyHeight - 60) + "px");
    // 그림 자리(itda-hyve#475)의 퍼센트 기준이 되는 본문 높이다. --img-max-h 는 임시 상자(root)에 걸어 쪽이 옮겨지면 사라지지만,
    // 이것은 쪽마다 쪽 요소에 건다 — 쪽이 무대(#stage)에서 보기 영역·출력 상자로 옮겨져도 그림 상자의 크기가 바뀌지 않게.
    pg.style.setProperty("--body-h", bodyHeight + "px");
    return pg;
  }

  const hasContent = el => el.children.length > 0 || (el.textContent || "").trim() !== "";

  // fits 는 지금 쪽 본문이 넘치지 않는지다. 마지막 블록의 아래 끝으로 본다 — scrollHeight 는 마지막 블록의
  // 아래 여백까지 세어 쪽 끝에서 여백 때문에 일찍 넘긴다.
  function fits() {
    const last = body.lastElementChild;
    if (!last) return true;
    return deepBottom(last) <= bodyBottom + 0.5;
  }
  function deepBottom(el) {
    // 표의 caption-side·음수 여백 등으로 자식이 부모 밖으로 나올 수 있어 마지막 자식까지 본다
    let b = el.getBoundingClientRect().bottom;
    const lc = el.lastElementChild;
    if (lc && el.tagName !== "TABLE") b = Math.max(b, lc.getBoundingClientRect().bottom);
    return b;
  }

  // 사본을 만들 때 쪽 나눔기가 알아야 할 것: id 는 앞 조각에만 남긴다(목차 링크가 한 곳을 가리키게).
  function shallow(el, cont) {
    const c = el.cloneNode(false);
    if (cont) { c.removeAttribute("id"); c.classList.add("cont"); }
    applyWidth(c);
    return c;
  }

  // mdoc 은 column·그림의 width 를 style="width:…" 로 낸다. 문서 창의 CSP(style-src 'self')는 style 속성을 막는다 —
  // 값을 data-w 로 옮겨 검사한 뒤 CSSOM 으로 다시 건다(스크립트가 거는 인라인 스타일은 CSP 가 막지 않는다).
  const widthRe = /^\s*width\s*:\s*(\d+(?:\.\d+)?(?:%|mm|cm|in|pt|pc|px|em|rem|ch)?)\s*;?\s*$/i;
  function adoptWidths(rootEl) {
    for (const el of rootEl.querySelectorAll("[style]")) {
      const m = widthRe.exec(el.getAttribute("style") || "");
      el.removeAttribute("style");
      if (m) { el.setAttribute("data-w", /\d$/.test(m[1]) ? m[1] + "px" : m[1]); }
    }
    for (const el of rootEl.querySelectorAll("[data-w]")) applyWidth(el);
    sizeDiagrams(rootEl);
  }

  // 그림 자리(itda-hyve#475): ```mermaid 펜스 줄의 ratio·width·height 를 mdoc 이 검사해 data-diagram-* 로 낸 값을
  // 화면이 다시 검사해 상자로 건다(위 data-w 와 같은 CSP 선례 — mdoc 의 Go 시험이 값 규칙을 본다). 상자는 SVG
  // 요소 자체다: viewBox 의 기본 preserveAspectRatio(xMidYMid meet)가 상자 안에 비율을 지켜 최대한 크게·가운데
  // 그린다(contain — 상자보다 작은 그림도 커진다). 백분율·비율 계산은 CSS calc 로 늦게 풀린다 — 여기서는 본문
  // 높이를 모르고, 쪽 나눔이 값을 재는 한참 뒤에 newPage 가 쪽마다 거는 --body-h 로 풀린다. 너비 % 는 CSS % 가
  // 풀리는 대로 그림이 놓인 자리(SVG 를 담은 figure 의 칸 — 최상위면 본문, 목록·인용·다단 안이면 그 칸) 기준이고
  // 높이 % 는 --body-h(본문 높이) 기준이라 두 축의 기준이 다르다(안내서 6장 값 규칙).
  //   cap = 본문 높이 − 60px(캡션·여백 자리 — --img-max-h 와 같은 60px): 높이가 정해지는 모든 조합의 상한이다.
  //   ratio 만·ratio+width  너비 min(놓인 자리의 100% 또는 N%, cap×비율), 높이는 aspect-ratio
  //   ratio+height          너비 min(100%, 본문높이×N%×비율, cap×비율) — 너비가 100% 에서 잘리면 높이도 비율대로 줄고,
  //                         본문 높이를 넘지 않게 cap 이 다시 자른다
  //   width 만              너비 N%(높이는 그림 제 비율, 높이 상한 --img-max-h 도 그대로 — max-height 를 안 건다)
  //   height 만             높이 min(본문높이×N%, cap), 너비 100%
  //   width+height          너비 N%, 높이 min(본문높이×N%, cap)(셋 다면 mdoc 이 ratio 를 뗏다 — 여기서도 지킨다)
  // 상자를 정한 그림은 base.css 의 max-height(--img-max-h) 대신 위 cap 을 쓴다(백분율 약속을 세는 본문 높이와
  // 같은 출처여야 한다) — max-height: none 을 건 뒤 min() 안에서 자른다.
  const diagramRatioRe = /^([1-9][0-9]?):([1-9][0-9]?)$/;
  const diagramPctRe = /^(100|[1-9][0-9])%$/;
  function sizeDiagrams(rootEl) {
    const BODY = "var(--body-h, 200mm)", CAP = "(" + BODY + " - 60px)";
    const hPct = n => "min(calc(" + BODY + " * " + n + " / 100), calc(" + CAP + "))";
    for (const fig of rootEl.querySelectorAll("figure.diagram[data-diagram-ratio], figure.diagram[data-diagram-width], figure.diagram[data-diagram-height]")) {
      const svg = fig.querySelector(":scope > svg.diagram-svg");
      if (!svg) continue;
      let w = 0, h = 0, rw = 0, rh = 0;
      const r = diagramRatioRe.exec(fig.getAttribute("data-diagram-ratio") || "");
      if (r && +r[1] * 10 >= +r[2] && +r[2] * 10 >= +r[1]) { rw = +r[1]; rh = +r[2]; }
      const wp = diagramPctRe.exec(fig.getAttribute("data-diagram-width") || "");
      if (wp) w = +wp[1]; // 그룹은 % 앞까지다(% 는 정규식 바깥)
      const hp = diagramPctRe.exec(fig.getAttribute("data-diagram-height") || "");
      if (hp) h = +hp[1];
      if (rw && w && h) rw = rh = 0; // 셋 다 — mdoc 이 경고하고 ratio 를 뗏다(옛 문서를 지키는 재검사)
      if (rw) {
        svg.style.maxHeight = "none";
        svg.style.aspectRatio = rw + " / " + rh;
        svg.style.width = h ? "min(100%, calc(" + BODY + " * " + h + " / 100 * " + rw + " / " + rh + "), calc(" + CAP + " * " + rw + " / " + rh + "))"
          : "min(" + (w ? w : 100) + "%, calc(" + CAP + " * " + rw + " / " + rh + "))";
      } else if (w || h) {
        if (w) svg.style.width = w + "%";
        if (h) { svg.style.height = hPct(h); svg.style.maxHeight = "none"; svg.style.width = svg.style.width || "100%"; }
      }
    }
  }
  function applyWidth(el) {
    if (el.nodeType === 1 && el.hasAttribute("data-w")) el.style.width = el.getAttribute("data-w");
  }
  function deepClone(el) {
    const c = el.cloneNode(true);
    if (c.nodeType === 1) { applyWidth(c); for (const x of c.querySelectorAll("[data-w]")) applyWidth(x); }
    return c;
  }

  // fit 은 node 를 parent 에 넣어 본다. 다 들어가면 {head: node, tail: null}, 일부만 들어가면 {head, tail}
  // (head 는 parent 안에, tail 은 밖에), 아무것도 안 들어가면 null.
  function fit(node, parent) {
    parent.appendChild(node);
    if (fits()) return { head: node, tail: null };
    parent.removeChild(node);
    return split(node, parent);
  }

  function split(node, parent) {
    if (atomic(node)) return null;
    if (node.tagName === "TABLE") return splitTable(node, parent);
    if (node.tagName === "PRE") return splitPre(node, parent);
    if (isTextBlock(node)) return splitText(node, parent);
    return splitChildren(node, parent);
  }

  function isTextBlock(n) {
    if (!["P", "LI", "DD", "DT", "TD", "FIGCAPTION", "BLOCKQUOTE"].includes(n.tagName)) return false;
    for (const c of n.children) if (BLOCK_TAGS.has(c.tagName)) return false;
    return true;
  }

  // ---- 컨테이너: 자식 단위로 채우고, 걸리는 자식은 다시 나눈다(목록·정의 목록·각주·div 컨테이너·인용).
  // 앞 조각은 사본으로만 채운다 — 나누기에 실패하면 원본이 그대로 남아야 다음 쪽에서 통째로 다시 시도한다
  // (원본의 자식을 옮겨 담다가 실패하면 글자가 사라졌다: 1차 실측에서 번호 목록 항목 글자 유실).
  function splitChildren(node, parent) {
    const head = shallow(node, false);
    parent.appendChild(head);
    const kids = [...node.childNodes].filter(k => k.nodeType !== 8);
    let i = 0, partialTail = null, placedItems = 0;
    for (; i < kids.length; i++) {
      const k = kids[i];
      if (k.nodeType === 3 && !k.data.trim()) { head.appendChild(k.cloneNode(false)); continue; }
      const r = fit(deepClone(k), head);
      if (r && !r.tail) { if (k.tagName === "LI") placedItems++; continue; }
      if (r && r.tail) { partialTail = r.tail; if (k.tagName === "LI") placedItems++; i++; break; }
      break; // 이 자식은 하나도 안 들어간다
    }
    let rest = kids.slice(i);
    if (i >= kids.length && !partialTail) {
      // 자식은 다 들어갔는데 전체는 안 들어갔다(아래 여백·테두리). 마지막 요소 자식부터 넘긴다.
      let j = kids.length - 1;
      while (j >= 0 && kids[j].nodeType !== 1) j--;
      if (j < 0) { parent.removeChild(head); return null; }
      const cut = head.childNodes.length - (kids.length - j);
      while (head.childNodes.length > cut) head.removeChild(head.lastChild);
      if (kids[j].tagName === "LI") placedItems--;
      rest = kids.slice(j);
    }
    // 다음 것과 함께 다닐 것(제목·dt)이 앞 조각 끝에 남지 않게 뒤로 보낸다
    if (!partialTail) {
      while (head.lastElementChild && keepWithNext(head.lastElementChild)) {
        const idx = [...head.childNodes].indexOf(head.lastElementChild);
        const moved = head.childNodes.length - idx;
        // head 의 뒤쪽 노드들은 kids 의 같은 자리 사본이다 — 원본으로 되돌려 rest 앞에 붙인다
        const startIdx = kids.length - rest.length - moved;
        while (head.childNodes.length > idx) head.removeChild(head.lastChild);
        rest = kids.slice(startIdx, kids.length - rest.length).concat(rest);
        if (rest[0] && rest[0].tagName === "LI") placedItems--;
      }
    }
    if (!hasContent(head)) { parent.removeChild(head); return null; }
    const tail = shallow(node, true);
    if (partialTail) tail.appendChild(partialTail);
    for (const k of rest) tail.appendChild(k);
    if (!hasContent(tail)) return { head, tail: null };
    if (node.tagName === "OL") {
      // 번호를 이어 간다. 반쯤 들어간 항목의 뒷조각(li.cont)은 번호 없이 보이고 다음 항목이 다음 번호를 받는다.
      // start="0"·음수도 그 값에서 잇는다 — 숫자가 아닌 값만 1(HTML 기본값)로 둔다(itda-hyve#584: `|| 1` 은 0 을 1 로 읽었다).
      const n = parseInt(node.getAttribute("start") || "1", 10);
      const start = Number.isNaN(n) ? 1 : n;
      tail.setAttribute("start", String(start + placedItems - (partialTail ? 1 : 0)));
    }
    return { head, tail };
  }

  // ---- 표: 행 단위. 새 쪽의 조각에 thead 를 다시 넣고, caption 은 첫 조각에만, tfoot 은 마지막 조각에만 둔다.
  function splitTable(node, parent) {
    const tb = node.tBodies[0];
    if (!tb) return null;
    const head = shallow(node, false);
    for (const c of node.children) if (c.tagName === "CAPTION" || c.tagName === "COLGROUP" || c.tagName === "THEAD") head.appendChild(deepClone(c));
    const hb = document.createElement("tbody");
    head.appendChild(hb);
    parent.appendChild(head);
    if (!fits()) { parent.removeChild(head); return null; }
    const rows = [...tb.rows];
    let k = 0;
    for (; k < rows.length; k++) {
      const row = deepClone(rows[k]); // 사본으로 채운다 — 실패하면 원본 표가 그대로 남아야 한다
      hb.appendChild(row);
      if (!fits()) { hb.removeChild(row); break; }
    }
    if (k >= rows.length) { k = rows.length - 1; hb.removeChild(hb.lastChild); } // tfoot 이 넘쳤다 — 마지막 행을 데려간다
    if (k <= 0) { parent.removeChild(head); return null; }
    const tail = shallow(node, true);
    const th = node.tHead;
    if (th) tail.appendChild(deepClone(th));
    const tbb = document.createElement("tbody");
    for (const r of rows.slice(k)) tbb.appendChild(r);
    tail.appendChild(tbb);
    if (node.tFoot) tail.appendChild(deepClone(node.tFoot));
    return { head, tail };
  }

  // ---- pre: 줄 단위. 뒷조각의 data-line 은 원문에서 이어지는 줄을 가리킨다.
  // 내용의 첫 줄(base): 코드 블록은 여는 펜스 다음 줄(data-line + 1), 원문 블록(.raw-source — mdoc 안전 모드)은 data-line 그 줄.
  // 나눈 조각에는 그 조각의 첫 내용 줄을 data-body-line 으로 적어, 뒷조각을 다시 나눌 때 펜스 몫을 또 더하지 않는다.
  // 원문 블록은 줄마다 span[data-line] 이라 조각에서도 줄마다 다시 만든다(편집기 줄 맞춤).
  function splitPre(node, parent) {
    const code = node.querySelector("code");
    const text = (code || node).textContent;
    const lines = text.replace(/\n$/, "").split("\n");
    if (lines.length < 2) return null;
    const dl = parseInt(node.getAttribute("data-line") || "", 10);
    const raw = node.classList.contains("raw-source");
    const bodyAttr = parseInt(node.getAttribute("data-body-line") || "", 10);
    const base = bodyAttr || (dl ? (raw ? dl : dl + 1) : 0);
    // 강조 span 이 줄바꿈을 품을 수 있다. 글자 오프셋으로 DOM 을 잘라 클래스 구조를 보존한다.
    // 일반 코드와 raw-source 는 기존 줄 생성 경로를 그대로 쓴다.
    const rich = !raw && code && Array.from(code.childNodes).some(n => n.nodeType === 1);
    const offsets = [0];
    for (const line of lines) offsets.push(offsets[offsets.length - 1] + line.length + 1);
    const cloneRange = (source, from, to, target) => {
      let pos = 0;
      const copy = (n) => {
        if (n.nodeType === 3) {
          const start = pos, end = pos + n.textContent.length;
          pos = end;
          if (end <= from || start >= to) return null;
          return document.createTextNode(n.textContent.slice(Math.max(0, from - start), Math.min(end, to) - start));
        }
        const out = n.cloneNode(false);
        for (const child of n.childNodes) { const part = copy(child); if (part) out.appendChild(part); }
        return out.childNodes.length ? out : null;
      };
      for (const child of source.childNodes) { const part = copy(child); if (part) target.appendChild(part); }
    };
    const fill = (el, from, to) => {
      if (rich) { cloneRange(code, offsets[from], offsets[to], el); return; }
      if (!raw || !base) { el.textContent = lines.slice(from, to).join("\n") + "\n"; return; }
      for (let i = from; i < to; i++) {
        const s = document.createElement("span");
        s.setAttribute("data-line", String(base + i));
        s.textContent = lines[i];
        el.appendChild(s);
        el.appendChild(document.createTextNode("\n"));
      }
    };
    const make = (from, to) => {
      const p = shallow(node, from > 0);
      if (code) { const c = code.cloneNode(false); fill(c, from, to); p.appendChild(c); } else fill(p, from, to);
      if (base) {
        p.setAttribute("data-body-line", String(base + from));
        if (from > 0) p.setAttribute("data-line", String(base + from));
      }
      return p;
    };
    let lo = 1, hi = lines.length - 1, best = 0;
    while (lo <= hi) {
      const mid = (lo + hi) >> 1;
      const h = make(0, mid);
      parent.appendChild(h);
      const ok = fits();
      parent.removeChild(h);
      if (ok) { best = mid; lo = mid + 1; } else hi = mid - 1;
    }
    if (best < 2 && lines.length > 3) return null; // 한두 줄만 남기지 않는다
    if (best === 0) return null;
    if (lines.length - best < 2 && best > 2) best = lines.length - 2; // 뒤에 한 줄만 넘기지 않는다
    const head = make(0, best);
    parent.appendChild(head);
    return { head, tail: make(best, lines.length) };
  }

  // ---- 문단: 글자 단위 이분 탐색. 공백에서 끊고(낱말 중간에서 자르지 않는다), 앞뒤 조각이 모두 2줄 이상이게 한다.
  function textNodes(el) {
    const w = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
    const a = [];
    while (w.nextNode()) a.push(w.currentNode);
    return a;
  }
  // 자르는 단위: 글자 노드, 그리고 수식(<math>, 설계 1.10) 하나는 통째로 — 수식 안 글자에서 자르면 반쪽 수식이 두 쪽에 나뉜다.
  // 자르는 자리가 수식 안이면 수식은 뒤 조각으로 간다.
  function cutUnits(el) {
    const w = document.createTreeWalker(el, NodeFilter.SHOW_TEXT | NodeFilter.SHOW_ELEMENT, {
      acceptNode: n => n.nodeType === 1 ? (n.localName === "math" ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_SKIP)
        : (n.parentNode && n.parentNode.closest && n.parentNode.closest("math") ? NodeFilter.FILTER_REJECT : NodeFilter.FILTER_ACCEPT),
    });
    const a = [];
    while (w.nextNode()) a.push(w.currentNode);
    return a;
  }
  function cut(el, n, tailPart) {
    const c = el.cloneNode(true);
    let left = n;
    for (const t of cutUnits(c)) {
      if (t.nodeType === 1) {
        const len = t.textContent.length;
        if (!tailPart) { if (left >= len) left -= len; else { t.remove(); left = 0; } }
        else { if (left >= len) { left -= len; t.remove(); } else left = 0; }
        continue;
      }
      const len = t.data.length;
      if (!tailPart) { if (left >= len) left -= len; else { t.data = t.data.slice(0, left); left = 0; } }
      else { if (left >= len) { left -= len; t.data = ""; } else { t.data = t.data.slice(left); left = 0; } }
    }
    if (tailPart) { c.removeAttribute("id"); c.classList.add("cont"); trimStart(c); }
    for (const x of [c, ...c.querySelectorAll("[data-w]")]) applyWidth(x);
    return c;
  }
  function trimStart(el) {
    for (const t of textNodes(el)) { if (t.data === "") continue; t.data = t.data.replace(/^\s+/, ""); if (t.data) break; }
  }
  function lineHeightOf(el) {
    const cs = getComputedStyle(el);
    const lh = parseFloat(cs.lineHeight);
    return isNaN(lh) ? parseFloat(cs.fontSize) * 1.2 : lh;
  }
  function splitText(node, parent) {
    const full = node.textContent;
    if (full.trim().length < 2) return null;
    parent.appendChild(node);
    const lh = lineHeightOf(node);
    const totalLines = Math.round(node.getBoundingClientRect().height / lh);
    parent.removeChild(node);
    // 4줄 미만은 쪼개지 않고 통째로 넘긴다 — 2~3줄 블록을 1줄 + 1줄로 나누면 쪽 맨 위에 한 줄만 남는다(실측: "달." 한 줄).
    if (totalLines < 4) return null;
    const minLines = 2;
    const linesOf = el => Math.round(el.getBoundingClientRect().height / lh);
    const search = maxLines => {
      let lo = 1, hi = full.length - 1, best = 0;
      while (lo <= hi) {
        const mid = (lo + hi) >> 1;
        const a = cut(node, mid, false);
        parent.appendChild(a);
        const ok = fits() && linesOf(a) <= maxLines;
        parent.removeChild(a);
        if (ok) { best = mid; lo = mid + 1; } else hi = mid - 1;
      }
      // 공백에서 끊는다. 공백이 없는 긴 글자(주소 등)는 그 자리에서 끊는다.
      let b = best;
      while (b > 0 && !/\s/.test(full[b - 1])) b--;
      return b > 0 ? b : best;
    };
    let n = search(totalLines - minLines);
    if (n <= 0) return null;
    let head = cut(node, n, false);
    parent.appendChild(head);
    const hl = linesOf(head);
    if (hl < minLines) { parent.removeChild(head); return null; } // 쪽 끝에 한 줄만 남기지 않는다(고아 줄)
    head.classList.add("split-head");
    const tail = cut(node, n, true);
    return { head, tail };
  }

  // ---- 한 쪽에도 안 들어가는 것을 빈 쪽에 담는다: 그림은 줄이고, 통짜 블록은 풀어서 나눈다. 그래도 안 되면 잘린 채 표시한다.
  function forceFit(node, parent) {
    if (node.nodeType === 1) {
      const imgs = node.tagName === "IMG" ? [node] : [...node.querySelectorAll("img")];
      if (imgs.length) {
        parent.appendChild(node);
        for (let f = 0.85; f > 0.2 && !fits(); f -= 0.15) {
          for (const im of imgs) im.style.maxHeight = Math.floor(bodyHeight * f) + "px";
        }
        if (fits()) return { head: node, tail: null };
        parent.removeChild(node);
      }
      // mermaid 그림 figure(itda-hyve#475): 캡션이 캡션 자리 예약(60px)보다 길면 상자+캡션이 쪽보다 커진다 — 아래 자식
      // 분할로 넘기면 캡션만 다음 쪽에 홀로 남으므로, 상자를 줄여 캡션과 함께 한 쪽에 통째로 넣는다(위 img 와 같은 축소).
      if (node.tagName === "FIGURE" && node.classList.contains("diagram")) {
        const dsvg = node.querySelector(":scope > svg.diagram-svg");
        if (dsvg) {
          const maxH0 = dsvg.style.maxHeight; // 축소 전 inline 값(sizeDiagrams 의 "none" 포함) — 실패 때 이 값으로 돌린다
          parent.appendChild(node);
          const cap = node.querySelector(":scope > figcaption");
          const reserve = (cap ? cap.getBoundingClientRect().height : 0) + 30; // 캡션 + figure 위아래·캡션 여백
          for (let f = 0.95; f > 0.2 && !fits(); f -= 0.05) {
            dsvg.style.maxHeight = Math.max(80, Math.floor((bodyHeight - reserve) * f)) + "px";
          }
          if (fits()) return { head: node, tail: null };
          parent.removeChild(node);
          dsvg.style.maxHeight = maxH0; // 여기까지 못 맞추면 원래 상자(max-height)로 돌려놓고 아래 분할 경로로 간다
        }
      }
      // 단 나눈 코드 블록(itda-hyve#487 — base.css pre.has-cols[data-cols]): 빈 쪽에서도 단 상태로 못 넣은 조각(줄 span 이 break-inside: avoid
      // 라 단 너비에서 쪽보다 높은 원문 줄, splitPre 의 최소 줄 규칙 등)은 단을 끄고(data-cols → data-cols-off, CSS 가 안 걸려 cols
      // 없는 렌더) cols 없는 블록이 받는 처리 전체를 그대로 받는다 — fit(splitPre), 그다음 forceFit(split-forced 자식 분할, 그래도
      // 안 되면 잘린 채 표시). 그래서 최상위 cols 블록은 cols 가 없을 때보다 더 잘리지 않는다(컨테이너 안 블록은 splitChildren 이 fit 만 불러 이 분기에 닿지 않는다 — itda-hyve#491). 앞조각(잘린 경우 포함)은 끈 상태 그대로,
      // 뒷조각만 data-cols 를 되돌려 다시 단으로 시작한다(또 못 넣으면 거기서 또 끈다). flow 는 렌더마다 HTML 에서 새로 만들어 원본에 안 샌다.
      if (node.tagName === "PRE" && node.hasAttribute("data-cols")) {
        const cols = node.getAttribute("data-cols");
        node.removeAttribute("data-cols");
        node.setAttribute("data-cols-off", cols);
        const r = fit(node, parent) || forceFit(node, parent); // node 에 data-cols 가 없으니 재귀는 이 분기를 다시 타지 않는다
        if (r.tail && r.tail.nodeType === 1) { r.tail.removeAttribute("data-cols-off"); r.tail.setAttribute("data-cols", cols); }
        return r;
      }
      if (node.children.length && !isHeading(node) && node.tagName !== "TR") {
        node.classList.add("split-forced");
        const r = splitChildren(node, parent);
        if (r) return r;
        node.classList.remove("split-forced");
      }
    }
    parent.appendChild(node);
    if (node.nodeType === 1) node.classList.add("overflow-clipped");
    overflow++;
    return { head: node, tail: null };
  }

  // ---- .fit 그림(itda-hyve#550 — 슬라이드만: slides.js 가 opts.fitFill 과 figure.fig-fit 을 준다). 맨 위 블록의 .fit 그림은 놓이는 자리부터
  // 본문 아래까지의 높이를 그림 상자(figure)의 높이로 건다 — base.css 가 fit-sized figure 를 세로 flex 로 두어 img 가 캡션을 뺀 나머지를
  // 비율 유지(object-fit: contain)로 채운다. 높이는 이렇게 고른다(MIN = 본문 높이의 30%):
  //   1. 뒤에 남은 블록(이 묶음의 나머지 — slides.js 는 묶음마다 paginate 를 부른다)을 잰 높이만큼 남기고도 MIN 이상이면 그만큼
  //   2. 아니면 본문 아래까지 전부가 MIN 이상이면 그만큼(뒤 블록은 다음 장 — 「(계속)」)
  //   3. 아니면 빈 쪽이 아닐 때 false — 부르는 쪽이 새 쪽에서 다시 놓는다. 빈 쪽이면 남은 높이(최소 80px)로 놓는다
  // 높이는 쪽 상자 안 px 이라 쪽이 보기 영역·출력 상자로 옮겨져도 그대로다(쪽 크기는 슬라이드 크기로 고정).
  // 가로 배치 문단(p.fit-row, itda-hyve#569 — mdoc 이 .fit 그림만 든 맨 위 문단에 단다)도 같은 규칙으로 높이 h 를 고르고, figure 높이 대신
  // sizeRow 가 그림들을 같은 높이로 맞춘다(문단 높이는 그림 줄 + 캡션 줄 — 폭이 모자라면 h 보다 낮다).
  function placeFit(fig, queue, empty) {
    const row = fig.tagName === "P";
    const apply = row ? hh => sizeRow(fig, hh) : hh => { fig.style.height = hh + "px"; };
    fig.classList.add("fit-sized");
    fig.style.height = "0px";
    body.appendChild(fig);
    const cs = getComputedStyle(fig);
    // 놓이는 자리의 위 끝은 앞 블록의 아래 끝(없으면 본문 위 끝) + 위 여백이다 — 그림 자신의 위치로 재면 가운데 맞춤 본문(k-single)에서
    // 높이 0 상자가 가운데에 놓여 절반만 잰다
    const prev = fig.previousElementSibling;
    const top = (prev ? deepBottom(prev) : bodyBottom - bodyHeight) + (parseFloat(cs.marginTop) || 0);
    const room = bodyBottom - top - (parseFloat(cs.marginBottom) || 0);
    let reserve = 0;
    for (const q of queue) { // 뒤 블록 높이 + 위아래 여백(여백 겹침은 세지 않아 조금 넉넉하다)
      if (q.classList.contains("page-break")) break;
      body.appendChild(q);
      const qs = getComputedStyle(q);
      reserve += q.getBoundingClientRect().height + (parseFloat(qs.marginTop) || 0) + (parseFloat(qs.marginBottom) || 0);
      body.removeChild(q);
    }
    const MIN = bodyHeight * 0.3;
    let h = room - reserve >= MIN ? room - reserve : room >= MIN ? room : empty ? Math.max(80, room) : 0;
    if (!h) {
      body.removeChild(fig); fig.classList.remove("fit-sized"); fig.style.height = "";
      if (row) for (const im of rowImages(fig)) { im.style.width = ""; im.style.height = ""; }
      return false;
    }
    if (row) fig.style.height = "";
    h = Math.floor(h);
    apply(h);
    while (!fits() && h > 80) { h -= 2; apply(h); } // 반올림·여백 겹침으로 넘치면 조금씩 줄인다
    return true;
  }

  // rowImages 는 가로 배치 문단의 그림이다 — 문단의 직접 자식 img(막은 링크는 링크 없이 그림만 남는다)와 그림 하나를 감싼 링크의 img.
  function rowImages(p) { return [...p.querySelectorAll(":scope > img, :scope > a > img")]; }

  // sizeRow 는 가로 배치 문단 p 의 그림들을 같은 높이 H 로 맞춘다(itda-hyve#569). 너비는 그림마다 H × 원래 비율(naturalWidth/naturalHeight)이다 —
  // 비율을 못 읽은 그림(아직 안 읽힘·깨짐)은 너비를 비워 두어(CSS width: auto) 나중에 읽히면 제 비율로 서고, 폭 계산에만 4:3 으로 친다. H 는 문단 높이 h 에서 캡션 줄(slides.js rowCaptions 가 단 span.fr-cap 가운데 가장 높은 것 + 줄 사이)을 뺀
  // 높이와, 문단 폭에서 그림 사이 틈을 빼고 비율 합으로 나눈 높이 가운데 작은 쪽이다 — 그래서 그림 줄은 본문 폭 안이다. 캡션은
  // 그림 폭으로 줄이 바뀌므로 그림을 맞춘 뒤 한 번 더 재어 맞춘다. 크기는 CSSOM(style 속성 아님 — CSP 와 무관)으로 건다.
  function sizeRow(p, h) {
    const imgs = rowImages(p);
    const cs = getComputedStyle(p);
    const gap = parseFloat(cs.columnGap) || 0, rgap = parseFloat(cs.rowGap) || 0;
    const known = imgs.map(im => im.naturalWidth > 0 && im.naturalHeight > 0);
    const ratios = imgs.map((im, i) => known[i] ? im.naturalWidth / im.naturalHeight : 4 / 3);
    const sum = ratios.reduce((a, r) => a + r, 0) || 1;
    const wMax = Math.max(1, p.clientWidth - gap * Math.max(0, imgs.length - 1));
    const caps = [...p.querySelectorAll(":scope > .fr-cap")];
    const capH = () => caps.length ? Math.max(...caps.map(c => c.getBoundingClientRect().height)) + rgap : 0;
    const set = H => imgs.forEach((im, i) => { im.style.height = H + "px"; im.style.width = known[i] ? Math.floor(H * ratios[i]) + "px" : ""; });
    let H = Math.max(1, Math.floor(Math.min(h - capH(), wMax / sum)));
    set(H);
    if (caps.length) { H = Math.max(1, Math.floor(Math.min(h - capH(), wMax / sum))); set(H); }
    return H;
  }

  function measureHeight(node) {
    body.appendChild(node);
    const h = node.getBoundingClientRect().height;
    body.removeChild(node);
    return h;
  }

  // paginate 는 flow 의 블록을 target 안의 쪽 상자로 나눠 담는다. flow 는 비워진다.
  function paginate(flow, target, o) {
    const t0 = performance.now();
    opts = o || {};
    root = target; pages = []; overflow = 0;
    adoptWidths(flow);
    const queue = [...flow.childNodes].filter(n => n.nodeType === 1);
    newPage(false);
    let guard = 0;
    while (queue.length && guard++ < 200000) {
      const n = queue.shift();
      const empty = !hasContent(body);
      if (n.classList.contains("page-break")) { if (!empty) newPage(false); continue; }
      if (n.classList.contains("cover")) {
        if (!empty) newPage(false);
        pages[pages.length - 1].classList.add("page-cover");
        refreshBody();
        body.appendChild(n);
        newPage(false);
        continue;
      }
      // 장(h1) 앞에서 새 쪽(테마 설정). 쪽이 1/3 도 안 찼으면(요약문 한 덩이 등) 나누지 않는다 — 거의 빈 쪽이 생긴다.
      // 테마가 "always" 면(spine) 그래도 나눈다 — 장은 늘 새 쪽에서 시작한다.
      if (opts.h1Break && n.tagName === "H1" && !empty && !onlyBreakables() && (opts.h1Always || usedHeight() > bodyHeight * 0.35)) { queue.unshift(n); newPage(false); continue; }
      if (opts.fitFill && (n.tagName === "FIGURE" && n.classList.contains("fig-fit") || n.tagName === "P" && n.classList.contains("fit-row"))) {
        if (placeFit(n, queue, empty)) continue;
        queue.unshift(...carryHeadings(), n); newPage(false); continue;
      }
      if (softKeep(n) && !empty) {
        body.appendChild(n);
        if (fits()) continue;
        body.removeChild(n);
        if (measureHeight(n) <= bodyHeight && !onlyKeepNext()) { queue.unshift(...carryHeadings(), n); newPage(false); continue; }
      }
      const r = fit(n, body);
      if (r) { if (r.tail) { queue.unshift(r.tail); newPage(false); } continue; }
      // 쪽에 제목뿐이면 다음 쪽으로 보내도 같다(이 블록은 한 쪽보다 크다) — 제목 아래에서 줄이거나 풀어 나눈다
      if (!empty && !onlyKeepNext()) { queue.unshift(...carryHeadings(), n); newPage(false); continue; }
      const f = forceFit(n, body);
      if (f.tail) { queue.unshift(f.tail); newPage(false); }
    }
    if (!hasContent(body) && pages.length > 1) pages.pop().remove();
    decorate();
    return { pages: pages.length, contentPages: pages.filter(p => !p.classList.contains("page-cover")).length, overflow, ms: Math.round(performance.now() - t0) };
  }

  // 표지로 바뀐 쪽은 본문 상자 크기가 다르다
  function refreshBody() {
    const r = body.getBoundingClientRect();
    bodyBottom = r.top + body.clientHeight;
    bodyHeight = body.clientHeight;
  }
  function usedHeight() {
    const last = body.lastElementChild;
    return last ? deepBottom(last) - (bodyBottom - bodyHeight) : 0;
  }
  function onlyKeepNext() { return body.children.length > 0 && [...body.children].every(keepWithNext); }
  function onlyBreakables() { return [...body.children].every(c => c.classList.contains("page-break")); }

  // carryHeadings 는 쪽 끝에 남은 제목(들)을 떼어 돌려준다. 쪽이 제목뿐이면 그대로 둔다.
  function carryHeadings() {
    const carried = [];
    while (body.lastElementChild && keepWithNext(body.lastElementChild) && body.children.length > 1) {
      carried.unshift(body.removeChild(body.lastElementChild));
    }
    return carried;
  }

  // decorate 는 머리글·바닥글·쪽번호와 목차의 쪽번호를 채운다. 쪽번호는 표지를 빼고 센다.
  function decorate() {
    const content = pages.filter(p => !p.classList.contains("page-cover"));
    const total = content.length;
    const numOf = new Map();
    content.forEach((p, i) => numOf.set(p, i + 1));
    for (const p of pages) {
      p.dataset.page = String(numOf.get(p) || 0);
      if (p.classList.contains("page-cover")) continue;
      fillSlots(p.querySelector(".page-header"), opts.header, numOf.get(p), total);
      fillSlots(p.querySelector(".page-footer"), opts.footer, numOf.get(p), total);
    }
    // 목차의 쪽번호
    for (const a of root.querySelectorAll("nav.toc a[href^='#']")) {
      const id = decodeURIComponent(a.getAttribute("href").slice(1));
      const target = id && root.querySelector("#" + CSS.escape(id));
      const pg = target && target.closest(".page");
      const span = a.querySelector(".toc-pno");
      if (span) span.textContent = pg && numOf.get(pg) ? String(numOf.get(pg)) : "";
    }
  }

  // 머리글·바닥글: 조각은 글자(textContent)로만 넣는다 — frontmatter 값이 들어간다.
  function fillSlots(el, slots, n, total) {
    el.textContent = "";
    if (!slots) return;
    let any = false;
    ["left", "center", "right"].forEach((pos, i) => {
      const s = document.createElement("div");
      s.className = "slot " + pos;
      for (const part of slots[i] || []) {
        if (part.field === "page") s.append(String(n));
        else if (part.field === "pages") s.append(String(total));
        else s.append(part.text || "");
      }
      if (s.textContent.trim()) any = true;
      el.appendChild(s);
    });
    if (!any) { el.textContent = ""; el.classList.add("blank"); }
  }

  // 목차 항목에 쪽번호 자리를 미리 만든다(나눈 뒤 숫자가 들어가도 줄이 바뀌지 않게 자리를 잡아 둔다).
  function prepareTOC(flow) {
    for (const a of flow.querySelectorAll("nav.toc a[href^='#']")) {
      if (a.querySelector(".toc-pno")) continue;
      const s = document.createElement("span");
      s.className = "toc-pno";
      s.textContent = "00";
      a.appendChild(s);
    }
  }

  // numberBlocks 는 나누기 전 문서 흐름의 최상위 블록마다 data-n(0부터)을 붙이고 그 수를 돌려준다(itda-hyve#346 ⑧ 내용 앵커). 호스트 앱과
  // 보는 화면(share.js)이 같은 view HTML 에 같은 규칙으로 붙이므로, 창 폭·배율·쪽 수·보기 꼴이 달라도 같은 번호는 같은 내용이다.
  // 쪽 나눔이 블록을 쪼개도 조각(shallow·cut 의 cloneNode)마다 같은 data-n 이 남고, 슬라이드 나눔(slides.js groups)은 같은 요소를 옮긴다.
  // 표지·제목 블록·목차·쪽 나눔 표시·발표 단계(.pause)·노트는 번호에서 뺀다(쪽·이어보기에 내용으로 그려지지 않는다).
  const UNNUMBERED = "section.cover, header.title-block, section.toc-page, nav.toc, .page-break, .pause, .notes";
  function numberBlocks(flow) {
    let n = 0;
    for (const el of flow.children) {
      if (el.matches(UNNUMBERED)) { el.removeAttribute("data-n"); continue; }
      el.setAttribute("data-n", String(n++));
    }
    return n;
  }

  // ---- 쪽 넘김 키(itda-hyve#346 ⑨) — 보는 화면(share.js)과 호스트 앱의 보기 모드가 같은 규칙을 쓴다.

  // navKey 는 쪽 넘김 키의 방향이다: →·PageDown 은 1, ←·PageUp 은 -1, Home 은 "first", End 는 "last", 그 밖은 "". space 면 Space 는 1,
  // Shift+Space 는 -1(보는 화면만 — 문서 창은 Space 를 브라우저 굴림 그대로 둔다). 단추·링크에 포커스가 있으면 Space 는 그것을 누르는 키라
  // 넘기지 않는다. ⌘·Ctrl·⌥ 가 눌렸으면 넘기지 않는다(문서 창의 단축키 몫).
  function navKey(e, space) {
    if (e.altKey || e.ctrlKey || e.metaKey) return "";
    switch (e.key) {
      case "ArrowRight": case "PageDown": return 1;
      case "ArrowLeft": case "PageUp": return -1;
      case "Home": return "first";
      case "End": return "last";
      case " ":
        if (!space || (e.target && e.target.closest && e.target.closest("button, a[href], summary"))) return "";
        return e.shiftKey ? -1 : 1;
    }
    return "";
  }

  // typingTarget 은 el 이 글을 넣는 곳(input·textarea·select·contenteditable)인가다 — 그러면 넘김 키를 그것에 둔다.
  function typingTarget(el) {
    return !!(el && el.closest && el.closest("input, textarea, select, [contenteditable]"));
  }

  // hostNavKey 는 문서 창에서 이 키로 쪽을 넘길지다(navKey 의 방향, 아니면 ""). o: mode·presenting·hasDoc·modal(열린 대화상자가 있음)·
  // active(document.activeElement). 보기 모드(mode !== "edit" — 편집 모드의 ←/→ 는 커서 이동)·발표 아님(발표 키는 present.js)·문서
  // 있음·대화상자 없음이고 포커스가 편집기·입력란·단추·리본 탭·메뉴·대화상자 안이 아닐 때(본문·보기 영역)만. Space 는 넘기지 않는다.
  const HOST_BUSY = "input, textarea, select, button, [contenteditable], [role=menu], [role=menuitem], [role=tab], [role=dialog], .menu, .rb-tab";
  function hostNavKey(e, o) {
    if (o.mode === "edit" || o.presenting || !o.hasDoc || o.modal || e.defaultPrevented || e.isComposing) return "";
    if (o.active && o.active.closest && o.active.closest(HOST_BUSY)) return "";
    return navKey(e, false);
  }

  // pageNav 는 넘긴 뒤의 scrollTop 이다. dir 은 navKey 의 방향, look 은 보기 꼴("doc" 쪽·"slides" 슬라이드·"flow" 이어보기), st 는 지금
  // scrollTop, viewH 는 보기 영역 높이, pages 는 쪽(슬라이드)들의 {top, height}(scrollTop 좌표 — 배율이 걸린 그대로), maxTop 은 끝의 scrollTop.
  //  - 쪽: 보기 영역 맨 위 + 1px(⑧ 기준선)에 걸친 쪽 — 맨 위 여백·쪽 사이 여백이면 그 다음 쪽(⑧ blockSpot 과 같다), 모든 쪽 아래면 마지막
  //    쪽 — 의 이전·다음 쪽 맨 위. 첫 쪽 앞은 0, 마지막 쪽 뒤는 끝. Home 은 0, End 는 끝.
  //  - 슬라이드: 보기 영역 가운데에 걸친(사이 여백이면 그 위) 슬라이드의 이전·다음 슬라이드를 가운데로. 처음·끝에서 멈춘다. Home·End 는 첫·끝 장.
  //  - 이어보기(그리고 쪽이 없을 때): ±(보기 영역 높이 − 40px). Home 은 0, End 는 끝.
  // 늘 0 ~ maxTop 안으로 자른다.
  function pageNav(dir, look, st, viewH, pages, maxTop) {
    const end = Math.max(0, maxTop);
    const clamp = y => Math.max(0, Math.min(end, y));
    const n = pages ? pages.length : 0;
    if (look === "slides" && n) {
      const mid = st + viewH / 2;
      let cur = 0;
      pages.forEach((p, i) => { if (p.top <= mid) cur = i; });
      const i = dir === "first" ? 0 : dir === "last" ? n - 1 : Math.max(0, Math.min(n - 1, cur + dir));
      return clamp(pages[i].top + pages[i].height / 2 - viewH / 2);
    }
    if (dir === "first") return 0;
    if (dir === "last") return end;
    if (look === "doc" && n) {
      let cur = pages.findIndex(p => p.top + p.height > st + 1);
      if (cur < 0) cur = n - 1;
      const i = cur + dir;
      if (i < 0) return 0;
      if (i >= n) return end;
      return clamp(pages[i].top);
    }
    return clamp(st + dir * Math.max(1, viewH - 40));
  }

  // 쪽의 첫 새 블록을 원문 줄에 대응시킨다. 이어진 문단·표 머리는 건너뛰고,
  // splitPre 가 이어지는 실제 내용 줄을 적은 pre 는 경계로 쓴다.
  // pageBandEl 은 이 대응의 대상 요소(쪽의 본문 첫 새 블록 — data-line 이 달린 것)를 골라 돌려준다: pageBands 가
  // 띠를 만드는 규칙 그 자체다. 없으면 null.
  function pageBandEl(page, look) {
    const body = page.querySelector(".page-body");
    let hit = null;
    const visit = node => {
      if (!node || hit) return;
      const tag = node.tagName;
      if (node.matches && node.matches("section.toc-page, nav.toc")) return; // 생성된 차례
      if (tag === "THEAD" && node.closest && node.closest("table.cont")) return;
      const continued = node.classList && node.classList.contains("cont");
      const n = Number(node.getAttribute && node.getAttribute("data-line"));
      if ((!continued || tag === "PRE" && node.getAttribute("data-body-line")) && Number.isInteger(n) && n > 0) { hit = node; return; }
      for (const child of node.children || []) visit(child);
    };
    // 「(계속)」 제목은 원래 제목 줄의 사본이다 — 뒤로 돌아가는 경계로 쓰지 않는다.
    if (look === "slides" && !page.querySelector(".slide-head .slide-cont")) visit(page.querySelector(".slide-head"));
    if (body) for (const child of body.children) visit(child);
    return hit;
  }
  function pageBands(pageNodes, look) {
    if (look === "flow" || look === "dashboard") return []; // 이어보기·보드(긴 한 장, itda-work/itda-hyve#362)는 쪽 띠가 없다
    const list = [...pageNodes];
    const total = list.filter(p => p.dataset.page !== "0").length;
    let lastLine = 0;
    return list.flatMap(page => {
      if (page.dataset.page === "0") return []; // 표지는 원문 본문 줄이 없는 생성 쪽이다.
      const hit = pageBandEl(page, look);
      const line = hit ? Number(hit.getAttribute("data-line")) : 0;
      // 표지·차례·빈 쪽처럼 새 원문 줄이 없는 쪽은 띠 없이 번호 셈에만 둔다.
      if (!line || line <= lastLine) return [];
      lastLine = line;
      return [{ page: Number(page.dataset.page), total, line }];
    });
  }

  // 현재 장 미리보기(itda-hyve#443): 커서 줄이 든 띠의 쪽 범위. bands 는 호스트 편집기가 준 띠({page, line} …), line 은 커서 줄,
  // pages 는 쪽 번호 목록(미리보기 쪽 순서 — 호스트 앱이 그린 쪽의 data-page). 커서 줄 이하의 마지막 띠 k 를 찾아
  // 띠 k 의 쪽부터 띠 k+1 의 쪽 앞까지(띠 없는 이어진 쪽 — 「(계속)」 넘침 쪽 — 은 앞 띠에 딸린다)를 돌려준다:
  // {first, last}(pages 인덱스, 닫힌 구간). 커서가 첫 띠 앞(머리말·표지)이면 첫 띠 앞의 모든 쪽(표지·차례)인데 그런 쪽이 없으면
  // (표지 없는 문서) 첫 띠 구간으로 본다. 띠가 없거나(pages 와 짝이 안 되는 낡은 띠 포함) 쪽 목록이 비었으면 null — 모든 쪽.
  // 입력 중 여러 띠가 같은 줄로 합쳐지면 첫 경계만 세고 나머지는 건너뛴다(높이 0 구간 — scrollMap 과 같은 규칙).
  function currentBandRange(bands, line, pages) {
    if (!bands || !bands.length || !pages || !pages.length) return null;
    const at = new Map(pages.map((p, i) => [p, i]));
    const flat = [];
    let lastLine = -Infinity;
    for (const b of bands) {
      if (!Number.isFinite(b.line) || !at.has(b.page) || b.line <= lastLine) continue;
      lastLine = b.line;
      flat.push(b);
    }
    if (!flat.length) return null;
    let k = -1;
    for (let i = 0; i < flat.length; i++) if (flat[i].line <= line) k = i;
    if (k < 0 && at.get(flat[0].page) === 0) k = 0; // 첫 띠 앞에 쪽이 없다(표지·차례 없음) — 첫 띠 구간으로 본다
    const first = k < 0 ? 0 : at.get(flat[k].page);
    const last = k < 0 ? at.get(flat[0].page) - 1 : k + 1 < flat.length ? at.get(flat[k + 1].page) - 1 : pages.length - 1;
    if (!(first <= last)) return null;
    return { first, last };
  }

  // DOM 없는 스크롤 매핑. 경계 없는 쪽은 다음 경계까지의 구간에 자연히 포함된다.
  // 입력 중 여러 띠가 같은 줄로 합쳐지면 첫 경계를 남긴다(높이 0 구간의 나눗셈·역방향 점프 방지).
  function scrollMap(boundaries, editorEnd, viewerEnd) {
    const endE = Math.max(0, editorEnd), endV = Math.max(0, viewerEnd);
    const points = [{ editor: 0, viewer: 0 }];
    let startBand = false;
    for (const b of boundaries) {
      const prev = points[points.length - 1];
      if (!Number.isFinite(b.editor) || !Number.isFinite(b.viewer)) continue;
      // 첫 줄 띠는 편집기 y=0 이다. 미리보기의 위 여백은 이 경계에 붙인다.
      if (!startBand && points.length === 1 && b.editor === 0 && b.viewer >= 0 && b.viewer < endV) { prev.viewer = b.viewer; startBand = true; continue; }
      if (b.editor <= prev.editor || b.viewer <= prev.viewer || b.editor >= endE || b.viewer >= endV) continue;
      points.push({ editor: b.editor, viewer: b.viewer });
    }
    points.push({ editor: endE, viewer: endV });
    return points;
  }
  function mapScroll(points, y, reverse = false) {
    const from = reverse ? "viewer" : "editor", to = reverse ? "editor" : "viewer";
    if (!points.length) return 0;
    if (y <= points[0][from]) return points[0][to];
    for (let i = 1; i < points.length; i++) {
      const a = points[i - 1], b = points[i];
      if (y <= b[from]) {
        const h = b[from] - a[from];
        return h > 0 ? a[to] + (b[to] - a[to]) * (y - a[from]) / h : b[to];
      }
    }
    return points[points.length - 1][to];
  }

  // 정식 이름은 window.ItdaPager 다. window.HyvePager 는 옛 이름(호환용 별칭) — 같은 객체를 한 번에 대입해 둘이 갈라지지 않는다.
  window.ItdaPager = window.HyvePager = { paginate, prepareTOC, adoptWidths, numberBlocks, navKey, typingTarget, hostNavKey, pageNav, pageBandEl, pageBands, currentBandRange, scrollMap, mapScroll };
})();
