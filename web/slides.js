// 슬라이드 나눔기 — 문서 흐름(A4 와 같은 HTML)을 16:9·4:3 슬라이드로 나눈다(설계 1.12·5장).
// 같은 md 가 A4 문서도 되고 발표도 된다. 나누는 규칙(Pandoc):
//   - 표지·제목 블록 → 제목 슬라이드
//   - 슬라이드 단계(level)보다 높은 제목 → 구역 슬라이드(제목만 크게. 바로 뒤의 내용은 그 슬라이드에 함께)
//   - 슬라이드 단계의 제목 → 새 슬라이드(그 제목이 슬라이드 제목)
//   - 가로줄(---)·쪽 나눔 → 그 자리에서 새 슬라이드(제목 없이)
//   - 넘치면 쪽 나눔기(pager.js)가 나눠 같은 제목 + "(계속 k/n)" 슬라이드로 잇는다 — 잘리는 슬라이드가 없다
//   - 그림·차트·표·요약문 하나만 있는 슬라이드는 그것을 크게(k-single)
//   - 분할 그림(mdoc 이 패널로 표시한 맨 위 블록 — data-slide-panel, itda-hyve#550)은 묶음의 패널로 빼 슬라이드 한쪽을 위아래 끝까지 채운다(takeSlot)
//   - 배경 그림(mdoc 이 배경으로 표시한 맨 위 블록 — data-slide-bg, itda-hyve#569)은 묶음의 배경으로 빼 슬라이드 전체(제목 띠 포함)를 채운다(takeSlot)
//   - .fit 그림만 든 문단(mdoc 이 class fit-row 를 단 맨 위 문단, itda-hyve#569)은 같은 높이로 가로에 늘어놓는다(rowCaptions·pager.js placeFit)
// 목차(nav.toc)는 슬라이드에 넣지 않는다. :::notes 는 슬라이드에서 빼 발표자 노트로 모은다(슬라이드·PDF 에는 안 보인다 —
// 미리보기의 노트 칸·발표 N 칸만 쓴다, itda-hyve#344). 노트는 원문에서 놓인 자리의 장에 붙는다(groups·notePage, itda-hyve#518).
// 차례로 나타내기: ". . ."(div.pause) 뒤의 블록, :::incremental(또는 slides.incremental 이면 모든 목록, :::nonincremental 안은 빼고)의
// 목록 항목에 data-step(1, 2, …)을 단다. 미리보기·PDF 는 모두 보이고(정적), 발표(present.js)가 단계별로 보인다.
(function () {
  "use strict";

  const SIZE = { "16:9": [1280, 720], "4:3": [960, 720] };
  const isHeading = n => /^H[1-6]$/.test(n.tagName);
  const level = n => Number(n.tagName.slice(1));
  const SINGLE = n => n.matches("figure, table, .lead, blockquote");

  function lang() { return (document.documentElement.lang || "ko").startsWith("ko"); }

  // groups 는 흐름의 맨 위 블록을 슬라이드 묶음으로 나눈다.
  // 노트(:::notes)는 묶음의 notes 에 { el, at } 로 모은다(itda-hyve#518). at 은 build 가 노트를 붙일 장을 고르는 법이다(notePage):
  //   "line" — 원문 자리(data-line)로: 노트 바로 앞 내용이 그려진 장. 묶음 안 노트는 모두 이것이다.
  //   "first" — 묶음의 첫 장. 제목 슬라이드(표지·제목 블록) 바로 뒤의 노트(제목 슬라이드의 노트, itda-hyve#179)와, 앞 슬라이드가 없어 다음
  //             슬라이드로 넘긴 노트.
  //   "last" — 묶음의 마지막 장. 걸러지는 묶음(제목도 의미 있는 블록도 없는 것 — 가로줄·쪽 나눔·각주 뒤에 노트와 멈춤 ". . ." 만
  //            있고 곧 새 슬라이드나 문서 끝이 오는 것)의 노트는 앞의 남는 묶음 끝에 붙인다(itda-hyve#179 — 예전에는 묶음과 함께 버려졌다).
  // 가로줄 뒤 노트 다음에 내용이 이어지면 노트와 내용이 한 묶음이라 그 새 슬라이드의 노트다("line" — 노트가 첫 블록 앞이라 첫 장).
  // 묶음 notes 의 차례는 원문 차례다(앞 묶음으로 넘긴 "last" 는 뒤에, 다음 묶음으로 넘긴 "first" 는 앞에 붙인다).
  function groups(flow, lvl) {
    const out = [];
    let cur = null, titleG = null;
    const open = (kind, title) => { cur = { kind, title, blocks: [], notes: [] }; out.push(cur); return cur; };
    for (const el of [...flow.children]) {
      if (el.matches("section.toc-page, nav.toc")) continue;
      // .notes 는 태그를 가리지 않지만 mdoc 은 제목의 notes class 를 버리므로(headings.go dropHeadingNotes) 결과적으로 :::notes 컨테이너만 걸린다
      if (el.matches(".notes")) {
        if (!cur && titleG) titleG.notes.push({ el, at: "first" });
        else (cur || open("content", null)).notes.push({ el, at: "line" });
        continue;
      }
      titleG = null;
      if (el.matches("section.cover, header.title-block")) {
        titleG = { kind: "title", src: el, blocks: [], notes: [] };
        out.push(titleG);
        cur = null;
        continue;
      }
      if (el.matches(".page-break, hr")) { cur = null; continue; }
      if (isHeading(el) && lvl > 0 && level(el) < lvl) { open("section", el); continue; }
      if (isHeading(el) && level(el) === lvl) { open("content", el); continue; }
      if (el.matches("section.footnotes") || el.matches(".footnotes")) {
        const h = document.createElement("h2");
        h.textContent = lang() ? "주석" : "Notes";
        open("content", h).blocks.push(el);
        cur = null;
        continue;
      }
      (cur || open("content", null)).blocks.push(el);
    }
    // 분할 그림(itda-hyve#550)·배경 그림(itda-hyve#569)을 패널·배경으로 뺀다 — mdoc 이 표시한 맨 위 블록만. 의미 있는 블록 하나 판정보다 먼저(빼고 남은 것으로 본다)
    for (const g of out) {
      if (g.kind === "title") continue;
      const s = takeSlot(g);
      g.panel = s && s.kind === "panel" ? s : null;
      g.bg = s && s.kind === "bg" ? s : null;
    }
    // 내용 슬라이드 가운데 의미 있는 블록이 하나(그림·차트·표·요약문)뿐인 것
    for (const g of out) {
      if (g.kind !== "content") continue;
      const real = g.blocks.filter(b => !b.matches(".pause"));
      if (real.length === 1 && SINGLE(real[0])) g.kind = "single";
    }
    // 걸러지는 묶음의 노트를 이웃 남는 묶음으로 옮기고 거른다 — 앞 묶음 마지막 장, 앞 묶음이 없으면 다음 묶음 첫 장
    const kept = g => g.kind === "title" || g.kind === "section" || g.title || g.panel || g.bg || g.blocks.some(b => !b.matches(".pause"));
    const res = [];
    let carry = [];
    for (const g of out) {
      if (kept(g)) {
        g.notes.unshift(...carry.map(n => ({ el: n.el, at: "first" })));
        carry = [];
        res.push(g);
      } else if (res.length) {
        for (const n of g.notes) res[res.length - 1].notes.push({ el: n.el, at: "last" });
      } else {
        carry.push(...g.notes);
      }
    }
    return res;
  }

  // ---- 분할 그림(itda-hyve#550)·배경 그림(itda-hyve#569). 판정은 mdoc(transform.go slideSplits)이 한다: 맨 위 블록이 분할(.left·.right)·배경(.full)
  // 그림 하나뿐인 줄(그림 figure, 그림 하나만 감싼 링크 문단)이면 그 블록에 data-slide-panel·data-slide-bg 를 달고(둘을 합쳐 슬라이드당
  // 하나 — 먼저 적은 쪽), 그 밖의 자리는 제자리 .fit 으로 바꿔 낸다. 그림 배치는 img 의 class(img-left·img-right·img-full·img-fill·
  // img-fit)와 data-split 이다. 슬라이드에서만 뜻이 있고 문서(A4·이어보기)에는 규칙이 없어 보통 그림이다.
  const SPLITS = new Set(["1:1", "1:2", "1:3", "2:1", "3:1"]);
  const SLOT = b => b.hasAttribute && (b.hasAttribute("data-slide-panel") || b.hasAttribute("data-slide-bg"));
  // takeSlot 은 묶음 g 의 맨 위 블록 가운데 data-slide-panel·data-slide-bg 가 달린 첫 블록을 그대로, 통째로 떼어 돌려준다(없으면 null):
  // 분할이면 { kind: "panel", block, side, split }, 배경이면 { kind: "bg", block, fill }. 블록을 다시 짜지 않는다 — 통째로 옮기므로
  // 블록 안의 무엇도 잃지 않는다. 블록은 mdoc(slotImage)이 고른 단독 그림 figure(Figure: 캡션이 있으면 캡션의 글·각주 참조 등도 함께)
  // 또는 그림 하나를 감싼 링크 문단(막은 링크면 링크 없이 그림만)이다. 자리·cover·비율은 블록 안 img 의 class·data-split 에서 읽기만 한다.
  // mdoc 이 이미 슬라이드당 하나로 줄여 내지만, 표시가 둘 이상이면 두 번째부터 표시를 떼고 그림을 .fit 으로 둔다(화면 묶음 규칙으로 한 번 더).
  function takeSlot(g) {
    const marked = g.blocks.filter(SLOT);
    for (const b of marked.slice(1)) {
      b.removeAttribute("data-slide-panel");
      b.removeAttribute("data-slide-bg");
      const im = b.querySelector("img");
      if (im) { im.classList.remove("img-left", "img-right", "img-full", "img-fill"); im.removeAttribute("data-split"); im.classList.add("img-fit"); }
    }
    const block = marked[0];
    if (!block) return null;
    block.remove();
    g.blocks = g.blocks.filter(b => b !== block);
    const im = block.querySelector("img");
    if (block.hasAttribute("data-slide-bg")) return { kind: "bg", block, fill: !!im && im.classList.contains("img-fill") };
    const side = im && im.classList.contains("img-left") ? "left" : "right";
    const sp = im && im.getAttribute("data-split");
    const split = SPLITS.has(sp) ? sp : "1:1";
    return { kind: "panel", block, side, split };
  }

  // notePage 는 노트 하나가 붙을 장(묶음 안 순번, 0부터)이다(itda-hyve#518). at·line 은 노트의 자리(groups 의 at, 노트 요소의 data-line),
  // starts[k] 는 묶음의 k 번째 장 본문(.page-body)에서 가장 앞선 data-line(없으면 Infinity)이다. 쪽 나눔기는 쪼갠 조각에도 data-line
  // 을 남기므로(pager.js — 문단·목록·표 뒷조각은 블록 첫 줄, 코드 뒷조각은 그 조각의 첫 줄) 「노트 줄보다 앞에서 시작하는 블록(조각)이
  // 있는 장」 가운데 마지막 장이 노트 바로 앞 내용이 그려진 장이다 — 두 장에 걸친 블록 뒤 노트는 끝 조각의 장. 그런 장이 없으면(노트가
  // 첫 블록보다 앞 — 제목 바로 뒤) 첫 장, data-line 이 없는 노트도 첫 장.
  function notePage(at, line, starts) {
    if (at === "last") return Math.max(0, starts.length - 1);
    if (at === "first" || !(line > 0)) return 0;
    let k = 0;
    starts.forEach((s, i) => { if (s < line) k = i; });
    return k;
  }

  // firstLine 은 쪽 본문에서 가장 앞선 data-line 이다(없으면 Infinity). 머리(.slide-head — 제목 사본)는 본문 밖이라 세지 않는다.
  function firstLine(body) {
    let m = Infinity;
    if (!body) return m;
    for (const x of body.querySelectorAll("[data-line]")) { const v = Number(x.getAttribute("data-line")); if (v > 0 && v < m) m = v; }
    return m;
  }

  // headClone 은 제목의 사본이다(번호 span 포함). id 는 첫 슬라이드에만 둔다(목차 링크가 한 곳을 가리키게).
  // cont 는 넘쳐 여러 장이 된 묶음의 표지 — 없거나 false 면 표지 없음(한 장 묶음), { k, n } 이면 「(계속 k/n)」(영어 「(cont. k/n)」,
  // itda-hyve#482). k 는 그 묶음 안 순번(첫 장이 1 — 첫 장에도 「(계속 1/n)」을 단다), n 은 묶음의 장 수다. 표지 글자는 contLabel 하나에서
  // 만든다. 첫 장 표지는 class 가 slide-seq, 이어진 장(k ≥ 2)만 slide-cont 다 — .slide-cont 는 「이어진 장」 표식으로 쓰인다(pager.js
  // pageBands, itda-hyve present.js 목차 묶기). 첫 장은 id·내용 앵커를 그대로 갖는다.
  function contLabel(k, n) { return (lang() ? "(계속 " : "(cont. ") + k + "/" + n + ")"; }
  // headHeight 는 넘치는 묶음의 머리 높이(--slide-head-h)를 고른다(itda-hyve#482). 재기·나누기는 넘겨받는 순수 함수다(selftest 가 주입해 본다).
  //   plain: 표지 없는 머리 높이. tagged(d): 순번·장 수가 d 자리인 표지를 단 머리 높이 — 「(계속 9…9/9…9)」(k 와 n 을 d 개의 9 로.
  //   숫자 폭이 글꼴마다 달라도 같은 자릿수 안에서 9 를 가장 넓은 쪽으로 친다 — 실제 표지는 k ≤ n 이고 자릿수가 같거나 작다).
  //   count(h): 머리 높이 h 로 나눴을 때의 장 수.
  // plain 으로 나눠 n 장이면, n ≥ 2 이고 tagged(n 의 자릿수) 가 지금 높이보다 클 때만 그 높이로 다시 나눈다. 다시 나눈 장 수의
  // 자릿수가 늘어 표지가 더 높아지면 한 번 더 — 높이가 더 늘지 않을 때까지 하되 다시 나누기는 HEAD_ROUNDS(3)번까지다. 높이는
  // 늘기만 하므로(tagged 는 자릿수에 따라 줄지 않는다) 끝난다. 3번이면 자릿수 1→2→3→4 를 따라간다 — 한 묶음이 그보다 더
  // 불어나는 일은 머리가 한 줄 늘어 생기는 장 수 변화로는 사실상 없다. 상한에 닿으면 마지막 높이를 쓴다(그 높이의 장 수 자릿수가
  // 더 늘었어도 다시 재지 않는다). 다시 나눈 결과가 한 장이 되어도 높이는 그대로다(되돌리면 다시 넘쳐 오간다).
  // 돌려주는 값: { h, n, rounds } — n 은 마지막에 센 장 수(h 로 나눈 것), rounds 는 다시 나눈 횟수.
  const HEAD_ROUNDS = 3;
  // DIGITS_MAX 는 묶음 장 수의 자릿수 상한이다 — paginate 는 블록 처리 반복을 200000 번에서 끊으므로(pager.js) 한 번에 만드는 쪽은
  // 그보다 많지 않다(6 자리). 표지가 이 자릿수로도 줄을 바꾸지 않으면 어떤 장 수로도 바꾸지 않는다.
  const DIGITS_MAX = 6;
  function headHeight(plain, tagged, count) {
    let h = plain, n = count(plain), rounds = 0;
    while (n >= 2 && rounds < HEAD_ROUNDS) {
      const t = tagged(String(n).length);
      if (t <= h) break;
      h = t;
      rounds++;
      n = count(h);
    }
    return { h, n, rounds };
  }

  function headClone(h, cont) {
    const t = document.createElement("div");
    t.className = "slide-title";
    t.innerHTML = h.innerHTML;
    const first = !cont || cont.k === 1;
    if (h.id && first) t.id = h.id;
    if (h.dataset.line) t.dataset.line = h.dataset.line;
    if (h.dataset.n && first) t.dataset.n = h.dataset.n; // 내용 앵커(itda-hyve#346) — 제목 블록은 첫 슬라이드의 머리로만 그려진다
    if (cont) { const c = document.createElement("span"); c.className = cont.k === 1 ? "slide-seq" : "slide-cont"; c.textContent = contLabel(cont.k, cont.n); t.append(" ", c); }
    return t;
  }

  function el(tag, cls, text) {
    const e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  }

  // rowCaptions 는 가로 배치 문단(mdoc 이 class fit-row 를 단 맨 위 문단, itda-hyve#569)의 그림마다 그 아래 캡션 자리(span.fr-cap)를 단다 — 글은
  // 그림의 title(mdoc 이 글자로 낸 값, textContent 로만 넣는다), 없으면 빈 자리(그림과 캡션이 같은 칸에 짝지어 놓이게). 그림(또는 그림을
  // 감싼 링크) 바로 뒤에 더하기만 하고 블록을 다시 짜지 않는다. base.css 가 문단을 그림 줄·캡션 줄 두 줄의 격자로 놓고, 높이는 pager.js
  // placeFit 이 건다. 캡션이 하나도 없으면 캡션 자리를 달지 않는다.
  function rowCaptions(f) {
    for (const p of f.querySelectorAll(":scope > p.fit-row")) {
      const items = [...p.children].filter(c => c.matches("img.img-fit, a"));
      if (!items.some(c => (c.matches("img") ? c : c.querySelector("img") || {}).title)) continue;
      p.classList.add("fr-has-cap");
      for (const c of items) {
        const im = c.matches("img") ? c : c.querySelector("img");
        c.after(el("span", "fr-cap", (im && im.title) || ""));
      }
    }
  }

  // markFigures 는 쪽을 나누기 전 슬라이드 본문의 그림에 슬라이드 전용 class 를 단다(itda-hyve#550 — base.css 가 .page.slide 안에서만 고른다):
  //   no-cap  캡션(figcaption) 없는 사진 그림(img 하나) — 높이 상한이 본문 높이(캡션 자리 60px 를 남기지 않는다)
  //   fig-fit .fit 그림 하나뿐인 figure — 맨 위 블록이면 쪽 나눔기가 남은 높이를 채운다(pager.js placeFit), 다단 칸·목록 안이면 CSS 만
  // .fit 그림의 width 는 쓰지 않는다(자리를 채우는 것이 뜻이다). 차트·mermaid 그림(figure.chart·diagram)은 제 규칙 그대로다.
  function markFigures(f) {
    for (const im of f.querySelectorAll("img.img-fit")) im.removeAttribute("style");
    for (const fig of f.querySelectorAll("figure")) {
      if (fig.matches(".chart, .diagram")) continue;
      const imgs = fig.querySelectorAll("img");
      if (imgs.length !== 1 || imgs[0].parentElement !== fig) continue;
      if (!fig.querySelector(":scope > figcaption")) fig.classList.add("no-cap");
      if (imgs[0].classList.contains("img-fit")) fig.classList.add("fig-fit");
    }
  }

  // slotBox 는 분할 그림 패널(.slide-panel, itda-hyve#550)·배경(.slide-bg, itda-hyve#569) 하나다. 첫 장은 표시된 블록을 그대로, 넘쳐 생긴 장은 그 블록의
  // 사본(cloneNode, id 없이)을 담는다. 크기·자리는 base.css 가 쪽 class(panel-left·panel-right·split-*·has-bg·bg-fill)로 정한다 — style 을
  // 걸지 않는다.
  function slotBox(p, copy) {
    // 배경 cover 는 상자에도 fill 을 단다 — 테마 CSS 는 @scope (.page) 안이라 쪽 자신의 bg-fill 을 고르지 못하고 형제(.slide-bg.fill ~ …)로 고른다
    const box = el("div", p.kind === "bg" ? "slide-bg" + (p.fill ? " fill" : "") : "slide-panel");
    const blk = copy ? p.block.cloneNode(true) : p.block;
    if (copy) for (const x of [blk, ...blk.querySelectorAll("[id]")]) x.removeAttribute("id");
    box.appendChild(blk);
    return box;
  }

  // titleSlide 는 표지·제목 블록에서 조직·제목·부제·날짜·작성자를 골라 제목 슬라이드를 만든다(구조는 앱이 정한 한 가지).
  // 값은 Go 가 준 문서 정보(o.info — frontmatter 와 테마 defaults)이고, 없으면 표지·제목 블록에서 고른다. 글자로만 넣는다.
  function titleSlide(page, src, o) {
    const body = page.querySelector(".page-body");
    const box = el("div", "slide-cover");
    const info = o.info || {};
    const pick = (sel, key) => { if (info[key]) return info[key]; const x = src.querySelector(sel); return x ? x.textContent.trim() : ""; };
    const org = pick(".cover-org, .title-org", "organization");
    const top = el("div", "sc-top");
    if (o.logo) { const im = document.createElement("img"); im.className = "sc-logo"; im.src = o.logo; im.alt = ""; top.appendChild(im); }
    if (org) top.appendChild(el("div", "sc-org", org));
    const cls = pick(".cover-classification", "classification");
    if (cls) top.appendChild(el("div", "sc-class", cls));
    box.appendChild(top);
    const main = el("div", "sc-main");
    const h = el("h1", "sc-title", pick(".cover-title, .title", "title"));
    if (src.dataset.line) h.dataset.line = src.dataset.line;
    main.appendChild(h);
    const sub = pick(".cover-subtitle, .subtitle", "subtitle");
    if (sub) main.appendChild(el("p", "sc-sub", sub));
    box.appendChild(main);
    const meta = el("div", "sc-meta");
    for (const key of ["author", "date", "doc-no"]) {
      const v = pick('.meta-row[data-key="' + key + '"] dd', key);
      if (v) meta.appendChild(el("span", "sc-" + key, v));
    }
    if (meta.children.length) box.appendChild(meta);
    body.appendChild(box);
  }

  // sectionHead 는 구역 슬라이드의 머리다: 큰 번호 + 제목. 문서가 제목 번호를 쓰지 않으면 구역 차례(1, 2, …)를 번호 자리에 둔다
  // (테마가 그 자리를 꾸민다 — 보고서의 왼쪽 면, 매뉴얼의 "제 n 장"). 바로 뒤의 내용은 아래에.
  function sectionHead(h, nth) {
    const head = el("div", "slide-section");
    const num = h.querySelector(".header-section-number");
    head.appendChild(el("div", "ss-num" + (num ? "" : " auto"), num ? num.textContent : String(nth)));
    const t = el("h2", "ss-title");
    for (const n of h.childNodes) if (!(n.nodeType === 1 && n.classList.contains("header-section-number"))) t.appendChild(n.cloneNode(true));
    if (h.id) t.id = h.id;
    if (h.dataset.line) t.dataset.line = h.dataset.line;
    if (h.dataset.n) t.dataset.n = h.dataset.n; // 내용 앵커(itda-hyve#346) — 구역 제목은 사본으로만 그려진다
    head.appendChild(t);
    return head;
  }

  // steps 는 쪽(슬라이드) 하나의 차례로 나타낼 블록에 data-step 을 단다. 단계 수를 돌려준다.
  // ". . ." 뒤의 블록은 다음 단계에, 차례로 나타낼 목록은 항목마다 한 단계씩. 다단·상자·:::incremental 안은 들어가 본다.
  function steps(body, incAll) {
    let s = 0;
    const walk = (parent, inc) => {
      for (const c of [...parent.children]) {
        if (c.classList.contains("pause")) { s++; continue; }
        if (s > 0) c.dataset.step = String(s);
        const here = c.classList.contains("nonincremental") ? false : inc || c.classList.contains("incremental");
        if (c.matches("ul, ol") && here) {
          for (const li of c.children) { s++; li.dataset.step = String(s); }
          continue;
        }
        if (c.matches(".incremental, .nonincremental, .columns, .column, .box")) walk(c, here);
      }
    };
    walk(body, !!incAll);
    return s;
  }

  // build 는 flow(문서 흐름 HTML 이 든 요소)를 target 안의 슬라이드(.page.slide)로 나눈다.
  // o = { ratio, level, incremental, fontScale(slides.font-scale — 0 이면 기본), logo, org, footer(글자) }. 돌려주는 값: { slides, notes[], overflow, ms }
  // notes[i] 는 슬라이드 i 에 붙은 노트 HTML 이다(없으면 "") — 노트는 원문에서 놓인 자리의 장에 붙는다(notePage, itda-hyve#518). 넘쳐 생긴
  // 「(계속)」 장도 제 노트만 갖고, 노트가 붙지 않은 장은 미리보기 노트 칸·발표 N 칸·공유 보는 화면 모두 「노트 없음」이다.
  function build(flow, target, o) {
    const t0 = performance.now();
    const [w, h] = SIZE[o.ratio] || SIZE["16:9"];
    target.classList.add("slides", o.ratio === "4:3" ? "r43" : "r169");
    target.style.setProperty("--page-w", w + "px");
    target.style.setProperty("--page-h", h + "px");
    // 글자 배율(slides.font-scale, itda-hyve#430) — 쪽을 나누기 전에 걸어 잰 크기와 그린 크기가 같다. 쪽마다도 건다(쪽은 target 밖
    // 보기 영역·발표 사본·출력으로 옮겨진다). 없거나 범위 밖이면 걸지 않는다 — base.css 의 기본(--slide-scale 0.9)
    const fs = Number(o.fontScale);
    const scale = fs >= 0.6 && fs <= 1.5 ? String(fs) : "";
    if (scale) target.style.setProperty("--slide-scale", scale);
    const gs = groups(flow, o.level || 0);
    const notes = [];
    let overflow = 0, sections = 0;
    for (const g of gs) {
      const sub = document.createElement("div");
      target.appendChild(sub);
      if (g.kind === "title") {
        const r = window.HyvePager.paginate(document.createElement("div"), sub, { pageClass: "slide k-title" });
        overflow += r.overflow;
        titleSlide(sub.querySelector(".page"), g.src, o);
      } else {
        const f = document.createElement("div");
        f.className = "page-body doc-flow";
        // 구역 슬라이드는 제목을 본문 첫 블록으로 넣는다(뒤의 내용과 함께 나뉜다). 내용 슬라이드의 제목은 머리 자리(slide-head)에.
        if (g.kind === "section") f.appendChild(sectionHead(g.title, ++sections));
        for (const b of g.blocks) f.appendChild(b);
        // 하나만 있는 슬라이드의 그 블록은 크게 — 쪽을 나누기 전에 붙여야 잰 크기와 그린 크기가 같다(뒤에 붙였더니 표가 넘쳐 잘렸다)
        if (g.kind === "single") for (const b of g.blocks) if (!b.matches(".pause")) b.classList.add("slide-only");
        markFigures(f);
        rowCaptions(f);
        sub.appendChild(f);
        // 분할 그림이 있는 묶음(itda-hyve#550)은 쪽 class 로 글 영역을 반대쪽으로 좁힌다(base.css has-panel·panel-*·split-*) — 쪽을 나누기 전에
        // 걸어야 글 영역 폭으로 잰다(머리 높이 재기 probe 도 같은 class). 패널 자체는 나눈 뒤 장마다 붙인다(넘친 장도 반복).
        const cls = "slide k-" + g.kind + (g.title && g.kind !== "section" ? " has-head" : "") +
          (g.panel ? " has-panel panel-" + g.panel.side + " split-" + g.panel.split.replace(":", "-") : "") +
          (g.bg ? " has-bg " + (g.bg.fill ? "bg-fill" : "bg-contain") : "");
        // 머리(제목) 높이를 재어 본문 위치를 정한다 — 두 줄 제목도 본문을 가리지 않게.
        // 넘쳐 여러 장이 된 묶음의 머리에는(첫 장 포함) 「(계속 k/n)」 표지가 붙어 긴 제목은 한 줄 더 내려갈 수 있다(itda-hyve#482). 묶음의 쪽마다 같은 높이를 써야
        // 나눌 때 잰 본문 높이와 그린 높이가 같으므로, 표지가 실제로 줄을 바꾸는 묶음만 첫 장 포함 전체를 표지 단 높이로 나눈다.
        // 화면에 들어가는 블록은 늘 원본 f 이고 원본은 한 번만 나눈다 — 장 수를 미리 알아야 할 때는 사본(f.cloneNode)으로 센다.
        //   1. plain(표지 없는 머리)과 tagged(DIGITS_MAX 자리 표지)를 잰다. tagged ≤ plain 이면 어떤 장 수의 표지도 줄을 바꾸지
        //      않는다(표지 높이는 자릿수가 늘수록 줄지 않는다) — plain 으로 원본을 한 번 나눈다. 이전과 같은 길이다.
        //   2. 표지가 줄을 바꿀 수 있는 제목이면 headHeight 가 사본으로 장 수를 세며 실제 장 수의 자릿수 표지로 높이를 고른다.
        //      그 높이로 원본을 한 번 나눈다. 한 장이거나 실제 표지가 줄을 바꾸지 않으면 plain 이 남아 나눈 결과는 이전과 같다
        //      — 다만 사본 나눔이 한 번 더 든다(넘치지 않는 장도 이런 제목이면 한 쪽짜리 사본 나눔 한 번).
        let headH = 0;
        if (g.title && g.kind !== "section") {
          const probe = document.createElement("div");
          probe.className = "page " + cls;
          const hd = probe.appendChild(el("div", "slide-head"));
          hd.appendChild(headClone(g.title, false));
          sub.appendChild(probe);
          const plainH = Math.ceil(hd.getBoundingClientRect().height);
          const tagCache = {};
          const tagged = d => {
            if (!(d in tagCache)) {
              const nines = "9".repeat(d);
              hd.replaceChildren(headClone(g.title, { k: nines, n: nines }));
              tagCache[d] = Math.ceil(hd.getBoundingClientRect().height);
            }
            return tagCache[d];
          };
          headH = plainH;
          sub.style.setProperty("--slide-head-h", plainH + "px");
          f.remove();
          if (tagged(DIGITS_MAX) > plainH) {
            const count = h => {
              const dry = document.createElement("div");
              dry.style.setProperty("--slide-head-h", h + "px");
              target.appendChild(dry);
              window.HyvePager.paginate(f.cloneNode(true), dry, { pageClass: cls, fitFill: true }); // 실제 나눔과 같은 옵션(.fit 높이 채우기)
              const n = dry.querySelectorAll(":scope > .page").length;
              dry.remove();
              return n;
            };
            headH = headHeight(plainH, tagged, count).h;
            sub.style.setProperty("--slide-head-h", headH + "px");
          }
          probe.remove();
        }
        f.remove();
        const r = window.HyvePager.paginate(f, sub, { pageClass: cls, fitFill: true });
        overflow += r.overflow;
        const pages = [...sub.querySelectorAll(":scope > .page")];
        pages.forEach((pg, i) => {
          if (g.title && g.kind !== "section") {
            const hd = el("div", "slide-head");
            hd.appendChild(headClone(g.title, pages.length > 1 && { k: i + 1, n: pages.length }));
            pg.insertBefore(hd, pg.firstChild);
          }
          if (g.panel) pg.appendChild(slotBox(g.panel, i > 0));
          if (g.bg) pg.insertBefore(slotBox(g.bg, i > 0), pg.firstChild); // 배경은 쪽의 첫 자식 — 머리·본문·바닥이 그 위에 그려진다
        });
      }
      const made = [...sub.querySelectorAll(":scope > .page")];
      const starts = made.map(pg => firstLine(pg.querySelector(".page-body")));
      const per = made.map(() => []);
      for (const n of g.notes) per[notePage(n.at, Number(n.el.getAttribute("data-line")), starts)].push(n.el);
      for (const [j, pg] of made.entries()) {
        // 잴 때 임시 상자(sub)에 걸린 값(그림 최대 높이·제목 높이)을 쪽으로 옮긴다 — sub 를 지우면 사라져 차트가 커지고 잘렸다
        for (const k of ["--img-max-h", "--slide-head-h"]) { const v = sub.style.getPropertyValue(k); if (v) pg.style.setProperty(k, v); }
        if (scale) pg.style.setProperty("--slide-scale", scale);
        pg.dataset.kind = g.kind;
        notes.push(noteHTML(per[j]));
        target.insertBefore(pg, sub);
      }
      sub.remove();
    }
    // 꾸밈: 단계, 바닥(로고·조직·번호), 장식 자리(테마가 CSS 로 칠한다), 종류 클래스를 안쪽 요소에도(테마 CSS 는 .page 자신을 고르지 못한다)
    const all = [...target.querySelectorAll(":scope > .page")];
    all.forEach((pg, i) => {
      const kind = pg.dataset.kind;
      const body = pg.querySelector(".page-body");
      pg.dataset.steps = String(kind === "title" ? 0 : steps(body, o.incremental));
      pg.dataset.slide = String(i + 1);
      pg.dataset.page = String(i + 1);
      // 맨 구역 장(bare — 테마가 장식 면 위에 큰 제목을 놓는다)은 배경 그림 장이 아닐 때만. 배경 장에는 장식이 없어(아래) 그 모양을 쓰지 않는다(itda-hyve#569)
      const hasBg = !!pg.querySelector(":scope > .slide-bg");
      if (kind === "section" && body.children.length === 1 && !hasBg) { body.classList.add("bare"); pg.classList.add("bare"); }
      for (const x of [pg.querySelector(".page-header"), pg.querySelector(".page-footer")]) if (x) x.remove();
      // 배경 그림 장(itda-hyve#569)에는 장식을 넣지 않는다 — 넓게 칠하는 장식이 그림을 가리고, DOM 에 두면 테마의 형제 선택자
      // (.slide-deco.k-section ~ .page-body 들여쓰기 등)가 보이지 않는 장식 폭만큼 본문을 민다. 종류 class(k-*)는 그대로 단다.
      if (!hasBg)
        pg.insertBefore(el("div", "slide-deco k-" + kind + (pg.classList.contains("bare") ? " bare" : "")), pg.firstChild);
      body.classList.add("k-" + kind);
      const head = pg.querySelector(".slide-head");
      if (head) head.classList.add("k-" + kind);
      if (kind !== "title") {
        const foot = el("div", "slide-foot k-" + kind);
        const left = el("div", "sf-left");
        if (o.logo) { const im = document.createElement("img"); im.className = "sf-logo"; im.src = o.logo; im.alt = ""; left.appendChild(im); }
        if (o.footer) left.appendChild(el("span", "sf-text", o.footer));
        foot.appendChild(left);
        foot.appendChild(el("div", "sf-num", (i + 1) + " / " + all.length));
        pg.appendChild(foot);
      }
    });
    return { slides: all.length, notes, overflow, ms: Math.round(performance.now() - t0) };
  }

  // noteHTML 은 한 장에 붙은 노트 요소들의 HTML 이다(원문 차례로 잇는다) — 빈 노트(공유가 「노트 함께 보이기」 꺼짐에서 남긴 빈 자리 — mdoc transform.go
  // notes)는 「노트 없음」 으로 친다. 노트가 하나도 없거나 모두 비면 "".
  function noteHTML(els) {
    return els.map(n => n.innerHTML).filter(h => h.trim() !== "").join("\n");
  }

  // ---- 노트 칸(itda-hyve#344) — 문서 창의 슬라이드 미리보기(app.js)와 공유 보는 화면(share.js)이 함께 쓴다.
  // 노트 칸(aside.slide-note)은 슬라이드(.page) 밖의 형제라 슬라이드 수·발표 사본·쪽 넘김(.page 만 센다)에 섞이지 않는다.

  // placeNotes 는 box(무대의 target 또는 #pages) 안의 슬라이드마다 노트 칸을 붙인다. o = { notes[](build 의 값), on(붙일지),
  // grip(오른쪽 배치에서 끌어 너비를 바꾸는 손잡이 .sn-grip 을 칸 왼쪽에 둘지) }. on 이 거짓이면 지우기만 한다.
  // 노트가 없는 슬라이드는 넘쳐 생긴 「(계속)」 장이든 아니든 「노트 없음」(오른쪽 배치에서만 보이고 아래 배치에서는 접힌다, itda-hyve#518).
  function placeNotes(box, o) {
    for (const x of box.querySelectorAll(":scope > .slide-note")) x.remove();
    if (!o.on) return;
    box.querySelectorAll(":scope > .page").forEach((pg, i) => {
      const n = el("aside", "slide-note");
      n.dataset.slide = String(i + 1);
      if (o.grip) {
        const g = n.appendChild(el("div", "sn-grip"));
        g.title = "끌어 노트 칸 너비 바꾸기 — 두 번 누르면 기본 너비";
      }
      const html = o.notes[i] || "";
      if (!html) { n.classList.add("sn-empty"); n.appendChild(el("p", "sn-none", "노트 없음")); }
      else {
        const body = el("div", "sn-body");
        body.innerHTML = html; // mdoc 이 만든 HTML(스크립트 없음, CSP 가 2차로 막는다) — 발표 N 칸과 같은 글
        for (const x of body.querySelectorAll("[id]")) x.removeAttribute("id"); // 목차·링크가 노트 칸을 가리키지 않게(발표 사본과 같다)
        n.appendChild(body);
      }
      pg.after(n);
    });
  }

  // 노트 글자 크기 단계(px — Go 의 noteFontSizes 와 같다). 미리보기 노트 칸·발표 N 칸·공유 보는 화면이 함께 쓴다.
  // itda-hyve#541 에서 28~64 를 더했다(최대 24→64 — 강의장에서 청중과 함께 읽는다).
  const NOTE_FONTS = [12, 13, 14, 16, 18, 20, 22, 24, 28, 32, 36, 40, 48, 56, 64];
  // noteFontStep 은 지금 크기 cur(px — 단계 밖의 기본값 17 도 된다)에서 dir(1 크게, -1 작게) 쪽 다음 단계다. 끝이면 그대로.
  function noteFontStep(cur, dir) {
    if (dir > 0) return NOTE_FONTS.find(x => x > cur) || cur;
    const below = NOTE_FONTS.filter(x => x < cur);
    return below.length ? below[below.length - 1] : cur;
  }

  window.HyveSlides = { build, groups, notePage, headHeight, SIZE, placeNotes, noteHTML, NOTE_FONTS, noteFontStep };
})();
