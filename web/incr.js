// 이어 만들기 공통 부품(itda-hyve#658) — 바뀐 묶음만 다시 만들고 나머지는 그대로 쓰는 쪽(슬라이드 slides.js build 의 memo)이 쓴다.
// 쪽 보기(itda-hyve#659)도 같은 부품을 쓸 수 있게 슬라이드 규칙을 넣지 않는다. 하는 일은 넷이다:
//   sig     블록 묶음의 서명 — outerHTML 에서 위치 속성(POS)을 묶음 첫 값에 대한 상대값으로 바꾼 글. 묶음 안 줄 간격·블록 수가 같고
//           내용이 같으면 같다. 위치가 통째로 밀린 것(앞에서 줄·블록이 늘거나 줄었다)은 같은 서명이다.
//   shift   그대로 쓰는 쪽의 위치 속성을 차이만큼 민다(줄은 dl, 블록 번호는 dn).
//   place   상자의 자식을 새 차례(nodes)로 맞춘다 — 남는 자식은 떼고, 새 노드만 끼운다. 그대로 쓰는 노드는 옮기지 않는다
//           (차례가 같다는 것이 이어 만들기의 전제다 — 어긋나면 끼우는 쪽이 옮긴다).
//   clear   기억(memo)을 비운다 — 옛 쪽을 붙잡지 않게.
(function () {
  "use strict";

  // 위치 속성 목록 — 이 한 곳에서만 정한다. 줄 속성(원문 줄 번호, 1부터)과 블록 번호(numberBlocks 의 data-n, 0부터)다.
  // 코드 줄 span(class 첫 낱말이 ln — 코드 줄 번호 감싸기, itda-hyve codelines.js)의 data-n 은 코드 안 줄 순번이라 위치가 아니다 — 빼고 본다.
  const LINE_ATTRS = ["data-line", "data-body-line", "data-table-line"];
  const N_ATTR = "data-n";
  const POS = [...LINE_ATTRS, N_ATTR];
  const SEL = POS.map(a => "[" + a + "]").join(",");
  const RE = new RegExp(' (class="ln(?: [^"]*)?" )?(' + POS.join("|") + ')="(-?\\d+)"', "g");
  // TAG 는 serialize 된 HTML 의 여는 태그다 — 따옴표 안의 > 도 태그 안이다. 글 노드의 < 는 &lt; 로 나오므로 글 안에서 시작하지 않는다.
  const TAG = /<[a-zA-Z](?:[^>"]|"[^"]*")*>/g;
  const codeLine = x => x.classList ? x.classList.contains("ln") : / ln /.test(" " + (x.getAttribute("class") || "") + " ");

  // base 는 블록들(과 그 안)에서 가장 작은 줄 번호(양수)·블록 번호다. 없으면 0.
  function base(els) {
    let line = Infinity, n = Infinity;
    const see = x => {
      for (const a of LINE_ATTRS) { const v = Number(x.getAttribute(a)); if (v > 0 && v < line) line = v; }
      if (x.hasAttribute(N_ATTR) && !codeLine(x)) { const v = Number(x.getAttribute(N_ATTR)); if (v >= 0 && v < n) n = v; }
    };
    for (const el of els) {
      if (!el || el.nodeType !== 1) continue;
      see(el);
      for (const x of el.querySelectorAll(SEL)) see(x);
    }
    return { line: line === Infinity ? 0 : line, n: n === Infinity ? 0 : n };
  }

  // rel 은 HTML 글의 태그 안 위치 속성을 b 에 대한 상대값으로 바꾼다. 글 노드는 그대로 둔다 — 코드 블록 글자의 data-line="12" 는 내용이다.
  function rel(html, b) {
    const attr = (m, ln, a, v) => ln && a === N_ATTR ? m : " " + (ln || "") + a + '="' + (Number(v) - (a === N_ATTR ? b.n : b.line)) + '"';
    return html.replace(TAG, t => t.replace(RE, attr));
  }

  // sig 는 블록들의 서명이다. parts 는 블록(요소)·글(문자열 — 묶음 종류 같은 서명에 함께 넣을 값)을 섞어 받는다. b 를 주지 않으면 블록들로 잰다.
  function sig(parts, b) {
    const els = parts.filter(p => p && p.nodeType === 1);
    const bb = b || base(els);
    let s = "";
    for (const p of parts) s += (p && p.nodeType === 1 ? rel(p.outerHTML, bb) : String(p)) + "\u0001";
    return s;
  }

  // shift 는 root 와 그 안의 위치 속성을 민다. 값이 바뀔 때만 쓴다(같은 값을 다시 쓰지 않는다).
  function shift(root, dl, dn) {
    if (!dl && !dn) return;
    const one = x => {
      if (dl) for (const a of LINE_ATTRS) {
        if (!x.hasAttribute(a)) continue;
        const v = Number(x.getAttribute(a));
        if (v > 0) x.setAttribute(a, String(v + dl));
      }
      if (dn && x.hasAttribute(N_ATTR) && !codeLine(x)) x.setAttribute(N_ATTR, String(Number(x.getAttribute(N_ATTR)) + dn));
    };
    one(root);
    for (const x of root.querySelectorAll(SEL)) one(x);
  }

  // place 는 box 의 자식을 nodes 차례로 맞춘다. 돌려주는 값: { removed, added } — 뗀 자식 수·끼운(옮긴) 노드 수.
  function place(box, nodes) {
    const keep = new Set(nodes);
    let removed = 0, added = 0;
    for (const c of [...box.childNodes]) if (!keep.has(c)) { c.remove(); removed++; }
    let cur = box.firstChild;
    for (const x of nodes) {
      if (x === cur) { cur = cur.nextSibling; continue; }
      box.insertBefore(x, cur);
      added++;
    }
    return { removed, added };
  }

  // clear 는 기억을 비운다 — 쥔 노드 목록·서명을 놓아 옛 쪽이 메모리에 남지 않게 한다.
  function clear(memo) {
    if (!memo) return;
    for (const k of Object.keys(memo)) delete memo[k];
  }

  window.ItdaIncr = { POS, LINE_ATTRS, N_ATTR, base, rel, sig, shift, place, clear };
})();
