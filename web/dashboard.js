// 보드 나눔기 — 문서 흐름(A4·슬라이드와 같은 HTML)을 격자 판(.page.board)으로 나눈다(명세 spec/dashboard.md, itda-work/itda-hyve#362).
// 같은 md 가 A4 문서·발표에 이어 보드도 된다. 나누는 규칙(slides.js 의 묶음과 같은 판단):
//   - 타일 단계(level — dashboard.level, 없으면 슬라이드 단계)의 제목 → 새 타일(그 제목이 타일 머리)
//   - 그보다 높은 제목 → 새 판(그 제목이 판 머리띠의 제목). 가로줄(---)·쪽 나눔 → 새 판(제목 없이)
//   - 표지·제목 블록은 판 머리띠의 제목·조직·날짜가 된다. 목차·각주·발표자 노트·멈춤(. . .)은 보드에 넣지 않는다
//   - 타일 크기는 내용 종류가 정한다(크기를 적는 문법은 없다): 본문이 그림·차트·흐름도(figure)나 표 하나면 2×1,
//     :::kpi 하나면 항목이 셋 이상일 때 2×1 아니면 1×1, 그 밖(글·목록·알림·상자·여럿)은 1×1
//   - 격자(cols × rows)에 읽는 차례대로 행 우선으로 놓는다 — 남은 칸에 안 들어가면 다음 줄, 판이 차면 다음 판(빈 칸은 그대로, 당겨 채우지 않는다).
//     같은 입력이면 같은 배치다(pack — testdata/board_pack.cjs 가 node 로 돈다, board_test.go)
// 타일 안이 넘치면 「(계속)」 으로 잇지 않는다(보드는 한눈에 보는 것): 표는 들어가는 행까지만 보이고 「… n행 더」, 그림·차트는 칸에 맞춰 줄이고,
// KPI 값은 --kpi-min-size 까지 줄인 뒤 공백에서 줄을 바꾸고, 그래도 한 낱말이 칸보다 넓으면(또는 아래로 넘치면) 보조 글을 숨기고
// 타일 머리에 「…」 를 달고 잘린다. 글은 잘리고 타일 머리에 「…」. 화면·PDF 가 같은 규칙이다(PDF 는 화면의 판을 찍는다).
(function () {
  "use strict";

  const isHeading = n => /^H[1-6]$/.test(n.tagName);
  const level = n => Number(n.tagName.slice(1));

  function el(tag, cls, text) {
    const e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  }

  // groups 는 흐름의 맨 위 블록을 판 나눔(break)과 타일(tile)의 차례로 나눈다. head 는 표지·제목 블록(없으면 null)이다.
  function groups(flow, lvl) {
    const items = [];
    let head = null, cur = null;
    for (const b of [...flow.children]) {
      if (b.matches("section.cover, header.title-block")) { head = b; continue; }
      if (b.matches("section.toc-page, nav.toc, .notes, .pause, .footnotes")) continue;
      if (b.matches(".page-break, hr")) { items.push({ kind: "break", title: null }); cur = null; continue; }
      if (isHeading(b) && lvl > 0 && level(b) < lvl) { items.push({ kind: "break", title: b }); cur = null; continue; }
      if (isHeading(b) && level(b) === lvl) { cur = { kind: "tile", title: b, blocks: [] }; items.push(cur); continue; }
      if (!cur) { cur = { kind: "tile", title: null, blocks: [] }; items.push(cur); }
      cur.blocks.push(b);
    }
    return { head, items };
  }

  // tileWidth 는 타일이 차지하는 열 수(1·2)다 — 내용 종류만 본다.
  function tileWidth(blocks) {
    if (blocks.length !== 1) return 1;
    const b = blocks[0];
    // KPI 는 mdoc 의 구조 게이트를 지난 것(.kpi-tiles — mdoc/kpi.go)만 — 구조 없는 회사 어휘 .kpi 는 글 타일과 같다
    if (b.matches(".kpi-tiles")) return b.querySelectorAll(".kpi-item").length >= 3 ? 2 : 1;
    return b.matches("figure, table") ? 2 : 1;
  }

  // pack 은 차례대로 놓는다. items = [{brk: true} | {w}], 돌려주는 place[i] 는 나눔이면 {b}(그 제목이 붙을 판), 타일이면 {b, r, c, w}.
  // 나눔은 지금 판에 타일이 있을 때만 새 판을 연다(빈 판에 나눔이 겹치면 그 판을 쓴다). 타일 너비는 cols 를 넘지 않는다.
  function pack(items, cols, rows) {
    cols = Math.max(1, cols | 0);
    rows = Math.max(1, rows | 0);
    let b = 0, r = 0, c = 0, used = false;
    const place = [];
    for (const it of items) {
      if (it.brk) {
        if (used) { b++; r = 0; c = 0; used = false; }
        place.push({ b });
        continue;
      }
      const w = Math.min(Math.max(1, it.w | 0), cols);
      if (c + w > cols) { r++; c = 0; }
      if (r >= rows) { b++; r = 0; c = 0; }
      place.push({ b, r, c, w });
      c += w;
      used = true;
    }
    return { place, boards: place.length ? Math.max(...place.map(p => p.b)) + 1 : 0 };
  }

  // cssPx 는 box 안에서 토큰 값(글자 크기)을 px 로 잰다.
  function cssPx(box, token) {
    const p = box.appendChild(el("span"));
    p.style.fontSize = "var(" + token + ")";
    const v = parseFloat(getComputedStyle(p).fontSize);
    p.remove();
    return v;
  }

  // clip 은 타일이 잘렸다고 머리에 「…」 를 단다(글자라 PDF 에도 남는다).
  function clip(tile) {
    tile.classList.add("tile-clipped");
    let head = tile.querySelector(":scope > .tile-head");
    if (!head) { head = el("header", "tile-head"); tile.insertBefore(head, tile.firstChild); }
    const mark = head.appendChild(el("span", "tile-clip", "…"));
    mark.title = "타일에 다 들어가지 않아 잘림";
    return true;
  }

  // fitKPI: 값 글자를 하한까지 줄이고 → 공백에서 줄을 바꾸고 → 그래도 한 낱말이 넓거나 아래로 넘치면 보조 글을 숨기고 「…」 + 잘림.
  function fitKPI(tile, body, kpi) {
    const vals = [...kpi.querySelectorAll(".kpi-item > dd:first-of-type")];
    const over = () => body.scrollHeight > body.clientHeight + 1;
    const wide = () => vals.some(v => v.scrollWidth > v.clientWidth + 1);
    if (!vals.length || (!over() && !wide())) return false;
    const min = cssPx(tile, "--kpi-min-size");
    let size = parseFloat(getComputedStyle(vals[0]).fontSize);
    while ((over() || wide()) && size > min) {
      size = Math.max(min, size - 1);
      kpi.style.setProperty("--kpi-fit", size + "px");
    }
    if (wide()) kpi.classList.add("kpi-wrap");
    if (!over() && !wide()) return false;
    kpi.classList.add("kpi-noaux");
    return clip(tile);
  }

  // fitTile 은 타일 본문이 칸을 넘칠 때의 규칙이다. 잘렸으면(「…」) 참.
  function fitTile(tile) {
    const body = tile.querySelector(":scope > .tile-body");
    if (!body) return false;
    const over = () => body.scrollHeight > body.clientHeight + 1;
    const only = body.children.length === 1 ? body.firstElementChild : null;
    // 구조 없는 .kpi(표식 없음)는 값이 없으니 아래 일반 글 타일의 넘침 처리로 간다
    if (only && only.matches(".kpi-tiles")) return fitKPI(tile, body, only);
    if (!over()) return false;
    const pic = only && only.matches("figure") ? only.querySelector("svg, img") : null;
    if (pic) {
      // 차트·그림은 칸에 맞춰 줄인다(문서의 --img-max-h 와 같은 길 — 너비는 그대로, 높이 상한만)
      for (let i = 0; i < 4 && over(); i++) {
        const h = pic.getBoundingClientRect().height - (body.scrollHeight - body.clientHeight) - 2;
        body.style.setProperty("--img-max-h", Math.max(24, Math.floor(h)) + "px");
      }
      if (!over()) return false;
    }
    if (only && only.matches("table")) {
      const rows = [...only.querySelectorAll(":scope > tbody > tr")];
      const more = body.appendChild(el("p", "tile-more"));
      let hidden = 0;
      while (over() && hidden < rows.length) {
        rows[rows.length - 1 - hidden].hidden = true;
        hidden++;
        more.textContent = "… " + hidden + "행 더";
      }
      if (!hidden) more.remove();
      if (!over()) return false;
    }
    return clip(tile);
  }

  // headText 는 표지·제목 블록에서 고른 글자다(o.info 가 앞선다 — slides.js titleSlide 와 같다).
  function headText(src, o, sel, key) {
    const info = o.info || {};
    if (info[key]) return info[key];
    const x = src && src.querySelector(sel);
    return x ? x.textContent.trim() : "";
  }

  // build 는 flow(문서 흐름 HTML 이 든 요소)를 target 안의 판(.page.board)으로 나눈다. target 은 무대(잴 수 있는 자리)에 있어야 한다.
  // o = { ratio, cols, rows, level, info, logo }. 돌려주는 값: { boards, overflow(「…」 로 잘린 타일 수), ms }
  function build(flow, target, o) {
    const t0 = performance.now();
    const SIZE = window.ItdaSlides.SIZE;
    const [w, h] = SIZE[o.ratio] || SIZE["16:9"];
    const cols = o.cols || 4, rows = o.rows || 3;
    target.classList.add("boards", o.ratio === "4:3" ? "r43" : "r169");
    target.style.setProperty("--page-w", w + "px");
    target.style.setProperty("--page-h", h + "px");
    const g = groups(flow, o.level || 0);
    const items = g.items.filter(it => it.kind === "break" || it.title || it.blocks.length);
    const { place, boards } = pack(items.map(it => it.kind === "break" ? { brk: true } : { w: tileWidth(it.blocks) }), cols, rows);
    const docTitle = headText(g.head, o, ".cover-title, .title", "title");
    const meta = [headText(g.head, o, ".cover-org, .title-org", "organization"), headText(g.head, o, '.meta-row[data-key="date"] dd', "date")].filter(Boolean).join(" · ");
    const pages = [];
    for (let i = 0; i < boards; i++) {
      const pg = el("div", "page board");
      pg.dataset.kind = "board";
      pg.dataset.steps = "0";
      pg.dataset.slide = pg.dataset.page = String(i + 1);
      pg.appendChild(el("div", "board-deco"));
      const head = pg.appendChild(el("header", "board-head"));
      if (o.logo) { const im = head.appendChild(el("img", "bh-logo")); im.src = o.logo; im.alt = ""; }
      head.appendChild(el("div", "bh-title", docTitle));
      if (meta) head.appendChild(el("div", "bh-meta", meta));
      head.appendChild(el("div", "bh-num", (i + 1) + " / " + boards));
      const grid = pg.appendChild(el("div", "page-body board-grid"));
      grid.style.gridTemplateColumns = "repeat(" + cols + ", minmax(0, 1fr))";
      grid.style.gridTemplateRows = "repeat(" + rows + ", minmax(0, 1fr))";
      target.appendChild(pg);
      pages.push(pg);
    }
    items.forEach((it, i) => {
      const p = place[i], pg = pages[p.b];
      if (it.kind === "break") {
        if (it.title) {
          // 판 제목은 제목 요소 그대로(id·data-line·내용 앵커 data-n 이 남는다)
          const t = pg.querySelector(".bh-title");
          t.textContent = "";
          t.appendChild(it.title);
        }
        return;
      }
      const tile = el("section", "tile" + (p.w > 1 ? " w2" : ""));
      tile.style.gridColumn = (p.c + 1) + " / span " + p.w;
      tile.style.gridRow = String(p.r + 1);
      if (it.title) tile.appendChild(el("header", "tile-head")).appendChild(it.title);
      const body = tile.appendChild(el("div", "tile-body"));
      for (const b of it.blocks) body.appendChild(b);
      pg.querySelector(".board-grid").appendChild(tile);
    });
    let overflow = 0;
    for (const t of target.querySelectorAll(".page.board .tile")) if (fitTile(t)) overflow++;
    return { boards, overflow, ms: Math.round(performance.now() - t0) };
  }

  // 정식 이름은 window.ItdaBoard 다(새 전역이라 옛 Hyve* 별칭은 없다). 판 크기는 slides.js(window.ItdaSlides.SIZE)를 쓴다 — 그 뒤에 싣는다.
  window.ItdaBoard = { build, groups, pack, tileWidth };
})();
