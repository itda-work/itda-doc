// 보드 나눔기 — 문서 흐름(A4·슬라이드와 같은 HTML)을 격자 한 장(.page.board)으로 놓는다(명세 spec/dashboard.md, itda-work/itda-hyve#362).
// 같은 md 가 A4 문서·발표에 이어 보드도 된다. 보드는 판으로 나뉘지 않는다 — 머리 하나 아래 타일이 이어진다. 놓는 규칙(slides.js 의 묶음과 같은 판단):
//   - 타일 단계(level — dashboard.level, 없으면 슬라이드 단계)의 제목 → 새 타일(그 제목이 타일 머리)
//   - 그보다 높은 제목 → 격자 전체 폭의 구역 제목 줄(.board-section). 가로줄(---)·쪽 나눔 → 다음 줄부터 시작(이미 줄 머리면 무시)
//   - 표지·제목 블록은 머리의 제목·조직·날짜가 된다(제목이 없으면 타일 앞의 첫 높은 제목). 목차·각주·발표자 노트·멈춤(. . .)은 보드에 넣지 않는다
//   - 타일 크기는 내용 종류가 정한다(크기를 적는 문법은 없다): 본문이 그림·차트·흐름도(figure)나 표 하나면 2×1,
//     :::kpi 하나면 항목이 셋 이상일 때 2×1 아니면 1×1, 그 밖(글·목록·알림·상자·여럿)은 1×1
//   - 읽는 차례대로 행 우선으로 놓는다 — 남은 칸에 안 들어가면 다음 줄(빈 칸은 그대로, 당겨 채우지 않는다).
//     같은 입력이면 같은 배치다(pack — testdata/board_pack.cjs 가 node 로 돈다, board_test.go)
//   - 타일 행 높이는 일정하다 — rows·ratio 의 상자(16:9 면 1280×720)에 머리와 rows 줄이 꼭 차는 높이(base.css --board-row-h)
// 타일 안이 넘치면 「(계속)」 으로 잇지 않는다(보드는 한눈에 보는 것): 표는 들어가는 행까지만 보이고 「… n행 더」, 그림·차트는 칸에 맞춰 줄이고,
// KPI 값은 --kpi-min-size 까지 줄인 뒤 공백에서 줄을 바꾸고, 그래도 한 낱말이 칸보다 넓으면(또는 아래로 넘치면) 보조 글을 숨기고
// 타일 머리에 「…」 를 달고 잘린다. 글은 잘리고 타일 머리에 「…」. 화면·PDF 가 같은 규칙이다(PDF 는 화면의 보드를 찍는다).
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

  // groups 는 흐름의 맨 위 블록을 줄 나눔(break)·구역 제목(section)·타일(tile)의 차례로 나눈다. head 는 표지·제목 블록(없으면 null)이다.
  function groups(flow, lvl) {
    const items = [];
    let head = null, cur = null;
    for (const b of [...flow.children]) {
      if (b.matches("section.cover, header.title-block")) { head = b; continue; }
      if (b.matches("section.toc-page, nav.toc, nav.todos, .notes, .pause, .footnotes")) continue;
      if (b.matches(".page-break, hr")) { items.push({ kind: "break" }); cur = null; continue; }
      if (isHeading(b) && lvl > 0 && level(b) < lvl) { items.push({ kind: "section", title: b }); cur = null; continue; }
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

  // pack 은 차례대로 놓는다. items = [{brk: true} | {sec: true} | {w}], 돌려주는 place[i] 는 타일이면 {r, c, w}, 구역 제목 줄이면 {r, sec: true}(한 줄을 다
  // 쓴다), 줄 나눔이면 {r}(놓이는 것 없음). 줄 나눔·구역 줄은 지금 줄에 타일이 있을 때만 다음 줄로 간다. 타일 너비는 cols 를 넘지 않는다.
  // rows 는 쓴 줄 수, sections 는 구역 제목 줄의 번호들이다(격자 행 높이를 정한다).
  function pack(items, cols) {
    cols = Math.max(1, cols | 0);
    let r = 0, c = 0;
    const place = [], sections = [];
    for (const it of items) {
      if (it.brk || it.sec) {
        if (c > 0) { r++; c = 0; }
        if (it.brk) { place.push({ r }); continue; }
        place.push({ r, sec: true });
        sections.push(r);
        r++;
        continue;
      }
      const w = Math.min(Math.max(1, it.w | 0), cols);
      if (c + w > cols) { r++; c = 0; }
      place.push({ r, c, w });
      c += w;
    }
    return { place, rows: c > 0 ? r + 1 : r, sections };
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

  // build 는 flow(문서 흐름 HTML 이 든 요소)를 target 안의 보드 한 장(.page.board)으로 놓는다. target 은 무대(잴 수 있는 자리)에 있어야 한다.
  // o = { ratio, cols, rows, level, info, logo }. 돌려주는 값: { overflow(「…」 로 잘린 타일 수), ms }
  function build(flow, target, o) {
    const t0 = performance.now();
    const SIZE = window.ItdaSlides.SIZE;
    const [w, h] = SIZE[o.ratio] || SIZE["16:9"];
    const cols = o.cols || 4, rows = o.rows || 3;
    target.classList.add("boards", o.ratio === "4:3" ? "r43" : "r169");
    target.style.setProperty("--page-w", w + "px");
    target.style.setProperty("--page-h", h + "px");
    const g = groups(flow, o.level || 0);
    const docTitle = headText(g.head, o, ".cover-title, .title", "title");
    let items = g.items.filter(it => it.kind !== "tile" || it.title || it.blocks.length);
    // 머리 제목: 문서 제목, 없으면 타일 앞의 첫 높은 제목(그 제목 요소 그대로 — id·data-line·내용 앵커 data-n 이 남는다)
    const firstTile = items.findIndex(it => it.kind === "tile");
    const before = firstTile < 0 ? items : items.slice(0, firstTile);
    const headSec = docTitle ? null : before.find(it => it.kind === "section") || null;
    if (headSec) items = items.filter(it => it !== headSec);
    const { place, rows: used, sections } = pack(items.map(it => (it.kind === "break" ? { brk: true } : it.kind === "section" ? { sec: true } : { w: tileWidth(it.blocks) })), cols);
    const meta = [headText(g.head, o, ".cover-org, .title-org", "organization"), headText(g.head, o, '.meta-row[data-key="date"] dd', "date")].filter(Boolean).join(" · ");
    const pg = el("div", "page board");
    pg.dataset.kind = "board";
    pg.dataset.steps = "0";
    pg.dataset.slide = pg.dataset.page = "1";
    pg.style.setProperty("--board-rows", String(rows));
    pg.appendChild(el("div", "board-deco"));
    const head = pg.appendChild(el("header", "board-head"));
    if (o.logo) { const im = head.appendChild(el("img", "bh-logo")); im.src = o.logo; im.alt = ""; }
    const title = head.appendChild(el("div", "bh-title", headSec ? null : docTitle));
    if (headSec) title.appendChild(headSec.title);
    if (meta) head.appendChild(el("div", "bh-meta", meta));
    const grid = pg.appendChild(el("div", "page-body board-grid"));
    grid.style.gridTemplateColumns = "repeat(" + cols + ", minmax(0, 1fr))";
    // 행 높이를 줄마다 적는다 — 타일 줄은 고정, 구역 제목 줄은 자기 높이. 크게 보기가 타일을 띄워도 줄이 그대로 남는다
    const isSec = new Set(sections);
    grid.style.gridTemplateRows = Array.from({ length: used }, (_, r) => (isSec.has(r) ? "auto" : "var(--board-row-h)")).join(" ");
    target.appendChild(pg);
    items.forEach((it, i) => {
      const p = place[i];
      if (it.kind === "break") return;
      if (it.kind === "section") {
        // 구역 제목은 제목 요소 그대로(id·data-line·내용 앵커 data-n 이 남는다 — 목차·링크가 닿는다)
        const row = grid.appendChild(el("div", "board-section"));
        row.style.gridColumn = "1 / -1";
        row.style.gridRow = String(p.r + 1);
        row.appendChild(it.title);
        return;
      }
      const tile = el("section", "tile" + (p.w > 1 ? " w2" : ""));
      tile.style.gridColumn = (p.c + 1) + " / span " + p.w;
      tile.style.gridRow = String(p.r + 1);
      if (it.title) tile.appendChild(el("header", "tile-head")).appendChild(it.title);
      const body = tile.appendChild(el("div", "tile-body"));
      for (const b of it.blocks) body.appendChild(b);
      grid.appendChild(tile);
    });
    // 넓은 수식을 타일 너비에 맞춘 뒤에 넘침을 잰다 — 줄어든 높이로 재야 수식이 있는 타일이 필요 없이 잘리지 않는다(보드는 쪽 나눔을 지나지 않는다, itda-hyve#752)
    window.ItdaPager.fitMath(pg);
    let overflow = 0;
    for (const t of target.querySelectorAll(".page.board .tile")) if (fitTile(t)) overflow++;
    return { overflow, ms: Math.round(performance.now() - t0) };
  }

  // ---- 크게 보기(itda-work/itda-hyve#362 — 보는 쪽 화면 동작이며 형식 약속이 아니다, 명세 dashboard.md 「크게 보기」)
  // 타일을 복제하지 않고 그 자리 타일을 띄운다: 타일에 tile-zoomed 를 달면 CSS(base.css 보드 절, @media screen)가 그 타일을 격자 안 절대 위치로
  // 올리고, 위치·크기는 zoomBox 가 굴림 칸의 보이는 영역에서 잰다(--tz-*). 다른 타일·구역 줄·머리(tile-under)는 visibility: hidden 이다. 그래서
  //   - 테마 꾸밈(@scope (.page) 안의 .page-body.board-grid .tile …)이 그대로 닿는다 — 조상이 바뀌지 않는다
  //   - 제목 id·차트 id 가 겹치지 않는다(복제가 없다)
  //   - 보드와 함께 확대·축소된다 — 창 확대(zoom)·transform 조상 아래 position: fixed 를 쓰지 않는다
  //   - 넘침 측정(fitTile)과 배치(pack)를 다시 하지 않는다. 숨긴 행·「… n행 더」·「…」·--img-max-h·--kpi-fit 은 DOM 에 그대로 두고
  //     CSS 가 크게 보기 동안만 덮는다 — 격자 행은 고정이라 타일이 빠져도 자리가 그대로고, 닫으면 클래스·--tz-* 만 떼므로 타일이 전과 같다
  //   - 인쇄·PDF 에는 크게 보기 규칙이 없다(@media screen 안) — 단추는 @media print 에서 숨는다

  // zoomBox 는 크게 보기 타일의 자리다 — grid 의 좌표(확대 전 px)로 돌려준다. view = 굴림 칸의 사각형, page·grid = 보드·격자의 사각형(모두 화면 px),
  // scale = 화면 px / 확대 전 px, gap = 둘레 여백(확대 전 px). 보이는 영역은 굴림 칸과 보드의 겹침이다.
  function zoomBox(view, page, grid, scale, gap) {
    const k = scale > 0 ? scale : 1;
    const left = Math.max(view.left, page.left), right = Math.min(view.right, page.right);
    const top = Math.max(view.top, page.top), bottom = Math.min(view.bottom, page.bottom);
    return { left: (left - grid.left) / k + gap, top: (top - grid.top) / k + gap,
      width: Math.max(0, (right - left) / k - 2 * gap), height: Math.max(0, (bottom - top) / k - 2 * gap) };
  }

  // zoomHit 는 타일 안 누르기가 크게 보기를 여는가다. t = 누른 요소, tile = 그 타일, selected = 고른 글자가 있음, moved = 누르고 끌었음.
  // 여는 곳: 크게 보기 단추, 타일 머리(링크·단추 밖), 타일·타일 본문의 빈 곳, 「… n행 더」. 본문 내용(글·표·차트·KPI)은 열지 않는다 —
  // 글자 고르기·링크·차트 값 표시·표 정렬·복사 조작을 가로채지 않게.
  function zoomHit(t, tile, selected, moved) {
    if (!t || !tile || selected || moved) return false;
    if (t.closest(".tile-zoom-btn")) return true;
    if (t.closest("a[href], button, input, select, textarea, summary, label, [contenteditable]")) return false;
    if (t.closest(".tile-head")) return true;
    return t === tile || t.matches(".tile-body, .tile-more");
  }

  // zoomStep 은 앞뒤 타일 번호다. 끝이면 그 자리(i)에 멈춘다.
  function zoomStep(i, dir, n) {
    const j = i + (dir < 0 ? -1 : 1);
    return i < 0 || j < 0 || j >= n ? i : j;
  }

  // zoomKey 는 크게 보기가 열린 동안의 키가 할 일이다: "close"(Esc)·"prev"·"next"(←·→, 조합키 없음)·"trap"(Tab — 밖으로 나가지 않게)·
  // "hold"(굴림 키 — 보드를 굴리지 않게 막는다, 크게 본 타일 본문 안이면 그 안 굴림만)·null(그 밖 — 그대로 둔다).
  function zoomKey(e) {
    if (e.isComposing) return null;
    if (e.key === "Escape") return "close";
    if (e.key === "Tab") return "trap";
    const mod = e.altKey || e.ctrlKey || e.metaKey || e.shiftKey;
    if (!mod && e.key === "ArrowLeft") return "prev";
    if (!mod && e.key === "ArrowRight") return "next";
    if (/^(ArrowUp|ArrowDown|PageUp|PageDown|Home|End| )$/.test(e.key)) return "hold";
    return null;
  }

  const FOCUSABLE = "a[href], button:not([disabled]), input, select, textarea, [tabindex]:not([tabindex='-1'])";

  // zoomer 는 root(보드가 든 요소) 안 보드 타일의 크게 보기를 다룬다. o = { blocked()(참이면 열지 않는다 — 발표 중·발표 따라가기),
  // scroller(보드를 굴리는 칸 — 크게 보기가 그 보이는 영역을 채운다), scale()(화면 px / 확대 전 px — 없으면 잰다),
  // reveal(el)(el 이 보이게 굴린다 — 없으면 el 의 scrollIntoView nearest) }. arm(root) 는 그린 뒤마다 부른다(타일마다 단추를 단다 — 이미 달린 타일은
  // 건너뛴다). 다시 그려 타일이 떨어지면(isConnected 아님) 열린 상태를 버린다. 인쇄 전(beforeprint)에는 닫는다.
  function zoomer(root, o) {
    o = o || {};
    const doc = root.ownerDocument, win = doc.defaultView;
    const GAP = 12;
    let cur = null, down = null, closedBy = null, mathW = null;
    // 수식은 글자 크기(크게 보기는 1.3배)와 타일 너비로 맞춘 값이다(pager.js fitMath) — 크게 보기를 열고 닫을 때와 타일 너비가 바뀔 때 다시 맞춘다. 안 그러면 작은 타일의
    // 높이에 큰 글자가 잘린다. 굴리지 않는다 — 타일이 #pages 안이라 굴림 상자가 쪽 전체를 흐리게 한다(itda-hyve#752).
    const fitTileMath = tile => { if (win.ItdaPager) win.ItdaPager.fitMath(tile); };
    const tiles = () => [...root.querySelectorAll(".page.board .page-body.board-grid > .tile")];
    const titleOf = tile => { const h = tile.querySelector(":scope > .tile-head > :is(h1, h2, h3, h4, h5, h6)"); return h ? h.textContent.trim() : ""; };
    const reveal = el => {
      if (o.reveal) { o.reveal(el); return; }
      if (el.scrollIntoView) el.scrollIntoView({ block: "nearest" });
    };
    // fit 은 열린 타일을 지금 보이는 영역에 맞춘다(열 때·창 크기·확대·굴림). 칸이 없으면 CSS 값(격자 전체)을 그대로 둔다.
    function fit() {
      if (stale() || !o.scroller) return;
      const grid = cur.parentElement, page = grid.closest(".page");
      const rp = page.getBoundingClientRect();
      const k = o.scale ? o.scale() : (page.offsetWidth ? rp.width / page.offsetWidth : 1);
      const b = zoomBox(o.scroller.getBoundingClientRect(), rp, grid.getBoundingClientRect(), k, GAP);
      for (const n of ["left", "top", "width", "height"]) cur.style.setProperty("--tz-" + n, b[n] + "px");
      if (b.width !== mathW) { mathW = b.width; fitTileMath(cur); } // 굴림마다 부르는 길이라 너비가 그대로면 재지 않는다
    }
    // under 는 크게 보기 동안 숨기는 것들이다 — 같은 격자의 타일·구역 줄과 보드 머리
    const under = tile => {
      const head = tile.closest(".page").querySelector(":scope > .board-head");
      return [...tile.parentElement.children].filter(t => t !== tile).concat(head ? [head] : []);
    };
    // 바깥 굴림 막기: 타일 본문이 실제로 굴러갈 때만 그대로 둔다(본문 끝에서 바깥으로 번지는 것은 CSS overscroll-behavior 가 막는다)
    function holdScroll(e) {
      if (stale()) return; // 다시 그려 타일이 떨어졌으면 열린 상태를 버리고 굴림을 돌려준다
      const body = cur.querySelector(":scope > .tile-body");
      if (!(body && body.contains(e.target) && body.scrollHeight > body.clientHeight + 1)) e.preventDefault();
    }
    if (o.scroller) {
      o.scroller.addEventListener("wheel", holdScroll, { passive: false });
      o.scroller.addEventListener("touchmove", holdScroll, { passive: false });
      // 스크롤 막대를 끌면 굴러간다 — 타일이 보이는 영역을 따라간다
      o.scroller.addEventListener("scroll", fit, { passive: true });
      win.addEventListener("resize", fit);
      if (win.ResizeObserver) { const ro = new win.ResizeObserver(fit); ro.observe(o.scroller); ro.observe(root); }
    }
    function arm() {
      for (const tile of tiles()) {
        if (tile.querySelector(":scope > .tile-zoom-btn")) continue;
        const b = el("button", "tile-zoom-btn");
        b.type = "button";
        const name = titleOf(tile);
        b.setAttribute("aria-label", name ? "크게 보기: " + name : "크게 보기");
        b.title = "크게 보기";
        tile.appendChild(b);
      }
    }
    // stale 은 열린 크게 보기가 없는가다 — 다시 그려 타일이 떨어졌으면 버리고, 열린 동안 발표가 시작됐으면(blocked) 닫는다.
    function stale() {
      if (cur && !cur.isConnected) cur = null;
      if (cur && o.blocked && o.blocked()) shut(false);
      return !cur;
    }
    function open(tile) {
      if (o.blocked && o.blocked()) return false;
      if (cur) shut(false);
      // 재생 중 YouTube 플레이어(호스트 부품)가 다른 타일에 있으면 그 타일이 가려져 소리만 남는다 — 멈춘다. 이 타일 안이면 그대로 크게 본다
      const yt = win.ItdaYT || win.HyveYT, player = root.querySelector(".yt-play");
      if (yt && player && !tile.contains(player)) yt.stop();
      reveal(tile); // 제자리에서 보이게 굴린 뒤 띄운다 — 닫으면 그 타일이 화면에 있다
      cur = tile;
      tile.classList.add("tile-zoomed");
      for (const t of under(tile)) t.classList.add("tile-under");
      mathW = null;
      fit();
      if (mathW === null) fitTileMath(tile); // 칸이 없으면 fit 이 재지 않는다
      tile.setAttribute("role", "dialog");
      tile.setAttribute("aria-modal", "true");
      tile.setAttribute("aria-label", titleOf(tile) || "타일 크게 보기");
      const body = tile.querySelector(":scope > .tile-body");
      if (body) body.tabIndex = 0; // 넘치는 표·글을 키로 굴릴 수 있게(크게 보기 동안만)
      const x = tile.appendChild(el("button", "tile-zoom-close"));
      x.type = "button";
      x.setAttribute("aria-label", "닫기");
      x.title = "닫기 (Esc)";
      x.focus({ preventScroll: true });
      return true;
    }
    function shut(focus) {
      if (cur && !cur.isConnected) cur = null;
      if (!cur) return;
      const tile = cur;
      cur = null;
      tile.classList.remove("tile-zoomed");
      for (const t of tile.parentElement ? under(tile) : []) t.classList.remove("tile-under");
      for (const n of ["left", "top", "width", "height"]) tile.style.removeProperty("--tz-" + n);
      fitTileMath(tile);
      tile.removeAttribute("role");
      tile.removeAttribute("aria-modal");
      tile.removeAttribute("aria-label");
      const body = tile.querySelector(":scope > .tile-body");
      if (body) body.removeAttribute("tabindex");
      const x = tile.querySelector(":scope > .tile-zoom-close");
      if (x) x.remove();
      if (focus) { const b = tile.querySelector(":scope > .tile-zoom-btn"); if (b) b.focus({ preventScroll: true }); }
    }
    function step(dir) {
      const list = tiles(), i = list.indexOf(cur), j = zoomStep(i, dir, list.length);
      if (j !== i) open(list[j]);
    }
    root.addEventListener("pointerdown", e => { down = { x: e.clientX, y: e.clientY }; });
    root.addEventListener("click", e => {
      const moved = !!down && Math.hypot(e.clientX - down.x, e.clientY - down.y) > 4;
      down = null;
      // 열려 있는 동안의 누르기는 아래 바깥 누르기가 다룬다 — 그 누르기로 닫혔으면(closedBy) 다른 타일을 열지 않는다.
      // 단추를 키(Enter·Space)로 누르면 브라우저가 click 을 보낸다(끌기 없음) — 키로 여는 길도 여기다
      if (e.button || e === closedBy || !stale()) return;
      const t = e.target && e.target.closest ? e.target : null;
      const tile = t && t.closest(".page.board .page-body.board-grid > .tile");
      if (!tile || !root.contains(tile)) return;
      const s = win.getSelection && win.getSelection();
      const selected = !!s && !s.isCollapsed && tile.contains(s.anchorNode) && String(s).trim() !== "";
      if (zoomHit(t, tile, selected, moved) && open(tile)) e.preventDefault();
    });
    // 닫기 단추·바깥 누르기 — 창 전체에서 잡는다(보드 밖·머리띠·보기 영역 빈 곳)
    win.addEventListener("click", e => {
      if (stale()) return;
      const t = e.target;
      if (t && t.closest && t.closest(".tile-zoom-close")) { closedBy = e; shut(true); return; }
      if (t && cur.contains(t)) return;
      closedBy = e;
      // 바깥을 눌러 초점이 다른 곳(목차·단추)으로 갔으면 그대로 두고, 아니면(빈 곳·타일 안) 원래 타일로 돌린다
      const a = doc.activeElement;
      shut(!a || a === doc.body || cur.contains(a));
    }, true);
    // 열린 동안의 키는 창(window) 잡기 단계에서 먼저 받아 호스트의 전역 키(쪽 넘김·따라가기 풀기·Esc)로 보내지 않는다
    win.addEventListener("keydown", e => {
      if (stale()) return;
      const act = zoomKey(e);
      if (!act) return;
      if (act === "trap") {
        const f = [...cur.querySelectorAll(FOCUSABLE)].filter(x => x.getClientRects().length && !x.matches(".tile-zoom-btn"));
        if (!f.length) { e.preventDefault(); return; }
        const i = f.indexOf(doc.activeElement);
        if (i < 0 || (e.shiftKey && i === 0) || (!e.shiftKey && i === f.length - 1)) {
          e.preventDefault();
          f[e.shiftKey ? f.length - 1 : 0].focus({ preventScroll: true });
        }
        e.stopPropagation();
        return;
      }
      e.stopPropagation();
      if (act === "hold") {
        const a = doc.activeElement, body = cur.querySelector(":scope > .tile-body");
        if (e.key === " " && a && cur.contains(a) && a.matches("button, a[href]")) return; // Space 는 단추 누르기 그대로
        if (!(body && body.contains(a))) e.preventDefault(); // 본문 안이면 그 안 굴림(기본 동작)만
        return;
      }
      e.preventDefault();
      if (act === "close") shut(true);
      else step(act === "prev" ? -1 : 1);
    }, true);
    win.addEventListener("beforeprint", () => shut(false));
    return { arm, open: tile => open(tile), close: () => shut(false), step, fit, current: () => (stale() ? null : cur) };
  }

  // 정식 이름은 window.ItdaBoard 다(새 전역이라 옛 Hyve* 별칭은 없다). 기준 상자 크기는 slides.js(window.ItdaSlides.SIZE)를, 수식 맞춤은 pager.js(window.ItdaPager.fitMath)를 쓴다 — 그 뒤에 싣는다.
  window.ItdaBoard = { build, groups, pack, tileWidth, zoomer, zoomBox, zoomHit, zoomStep, zoomKey };
})();
