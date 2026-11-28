// 보드 나눔기 — 문서 흐름(A4·슬라이드와 같은 HTML)을 격자 한 장(.page.board)으로 놓는다(명세 spec/dashboard.md, itda-work/itda-hyve#362).
// 같은 md 가 A4 문서·발표에 이어 보드도 된다. 보드는 판으로 나뉘지 않는다 — 머리 하나 아래 타일이 이어진다. 놓는 규칙(slides.js 의 묶음과 같은 판단):
//   - 타일 단계(level — dashboard.level, 없으면 슬라이드 단계)의 제목 → 새 타일(그 제목이 타일 머리)
//   - 그보다 높은 제목 → 격자 전체 폭의 구역 제목 줄(.board-section). 가로줄(---)·쪽 나눔 → 다음 줄부터 시작(이미 줄 머리면 무시)
//   - 표지·제목 블록은 머리의 제목·조직·날짜가 된다(제목이 없으면 타일 앞의 첫 높은 제목). 목차·각주·발표자 노트·멈춤(. . .)은 보드에 넣지 않는다
//   - 타일 크기는 적지 않으면 내용 종류가 정한다: 본문이 그림·차트·흐름도(figure)나 표 하나면 너비 2, :::kpi 하나면 항목이 셋 이상일 때 2 아니면 1,
//     그 밖(글·목록·알림·상자·여럿)은 1(tileWidth). 자리(at)도 크기(span)도 없는 타일은 촘촘한 격자에서 커진다 — 너비 × floor(cols/4)(열 수까지),
//     높이 floor(rows/3)(배율은 둘 다 최소 1 — autoScale). cols 7 이하·rows 5 이하면 배율 1 이라 내용 너비 × 1 그대로다.
//     at 만 적은 타일은 배율 없이 내용 너비 × 1 이다(mdoc boardplace.go 가 span 없는 고정 타일을 1×1 로 재는 것과 더 어긋나지 않게)
//   - 타일 제목 속성 at=열,행·span=너비x높이(itda-work/itda-hyve#1038)가 자리·크기를 정한다 — 행은 구역(구역 제목 줄·가로줄·쪽 나눔 사이) 안에서 센다.
//     틀린 값·격자 밖·겹침은 mdoc(boardplace.go)이 경고하고 속성을 떼어 보낸다. 여기서는 받은 값을 다시 확인하고(tileAttrs·pack), 내용 너비 때문에야
//     드러나는 겹침은 뒤 타일을 자동 배치로 돌린다(pack 의 moved)
//   - 적지 않은 타일은 읽는 차례대로 행 우선으로 놓는다 — 남은 칸에 안 들어가면 다음 줄(빈 칸은 그대로, 당겨 채우지 않는다), 찬 칸은 건너뛴다.
//     같은 입력이면 같은 배치다(pack — testdata/board_pack.cjs 가 node 로 돈다, board_test.go)
//   - 타일마다 놓인 자리를 data-board-seg·data-board-at·data-board-span·data-board-fixed 로, 격자에 열 수를 data-board-cols 로 남긴다(slots — 호스트의 끌기가 읽는다)
//   - 타일 행 높이는 일정하다 — rows·ratio 의 상자(16:9 면 1280×720)에 머리와 rows 줄이 꼭 차는 높이(base.css --board-row-h)
// 타일 안이 넘치면 「(계속)」 으로 잇지 않는다(보드는 한눈에 보는 것): 표는 들어가는 행까지만 보이고 「… n행 더」, 그림·차트는 칸에 맞춰 줄이고,
// KPI 값은 --kpi-min-size 까지 줄인 뒤 공백에서 줄을 바꾸고, 그래도 한 낱말이 칸보다 넓으면(또는 아래로 넘치면) 보조 글을 숨기고
// 타일 머리에 「…」 를 달고 잘린다. 글은 잘리고 타일 머리에 「…」. 화면·PDF 가 같은 규칙이다(PDF 는 화면의 보드를 찍는다).
// 타일을 여는 제목의 class ok·warn·bad 는 타일 상태다 — 타일에 tile-ok·tile-warn·tile-bad, 머리에 표식 자리(tileStatus·markStatus, itda-hyve#1038 3단계 B).
// 타일을 여는 제목의 to=도착 id[,…] 는 타일 잇기다 — 격자 맨 끝 선 층(svg.board-links)에 출발 → 도착 화살표 선, 출발 타일 머리에 읽는 프로그램용 글
// (links·linkGeometry, 3단계 D). 선은 타일 밑에 깔리고 배치(pack)를 바꾸지 않는다. to 가 없으면 선 층도 글도 없다.
// 같은 대상에 처음 그리면 타일 등장, 다시 그리면 본문이 바뀐 타일 강조(motion — o.motion === false 면 없음, 3단계 C). 움직임은 base.css 의 CSS animation 만이다.
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

  // autoScale 은 자리도 크기도 적지 않은 타일의 배율 [열 배율, 행 배율] = [max(1, floor(cols/4)), max(1, floor(rows/3))] 이다
  // (itda-work/itda-hyve#1038 3단계 — 기본 격자 4×3 의 한 칸이 촘촘한 격자에서도 비슷한 넓이가 되게). 이상한 값은 배율 1.
  function autoScale(cols, rows) {
    return [Math.max(1, Math.floor((cols | 0) / 4)), Math.max(1, Math.floor((rows | 0) / 3))];
  }

  // tileItem 은 타일 하나를 pack 의 항목으로 만든다 — w 는 내용 너비(tileWidth), attrs 는 tileAttrs, scale 은 autoScale.
  // 자리도 크기도 적지 않은 타일만 배율을 받는다({w: w × 열 배율, h: 행 배율} — 행 배율이 1 이면 h 를 싣지 않는다). at·span 이 하나라도 있으면
  // {w, at?, span?} 그대로다. mdoc 이 틀린 at·span 을 뗀 타일은 HTML 에 속성이 없어 적지 않은 타일과 같다(배율을 받는다).
  function tileItem(w, attrs, scale) {
    if (attrs.at || attrs.span) return Object.assign({ w }, attrs);
    const [sx, sy] = scale || [1, 1];
    return sy > 1 ? { w: w * sx, h: sy } : { w: w * sx };
  }

  // MAX_ROW 는 구역 안 행 번호(at 의 행 + 높이 − 1)의 상한이다 — mdoc boardplace.go boardMaxRow 와 같다(격자가 끝없이 늘지 않게).
  const MAX_ROW = 99;

  // tileAttrs 는 타일 제목의 data-at·data-span(mdoc 이 제목 속성 at=·span= 을 옮긴 것)을 읽는다. at = [열, 행](1부터), span = [너비, 높이].
  // 꼴이 틀리면 그 값은 없다(mdoc 이 이미 경고하고 뗀다 — 여기는 손으로 만든 HTML 에 대한 방어다).
  function tileAttrs(title) {
    const out = {};
    const d = title && title.dataset ? title.dataset : null;
    if (!d) return out;
    const a = /^\s*(\d{1,3})\s*,\s*(\d{1,3})\s*$/.exec(d.at || "");
    if (a && +a[1] >= 1 && +a[2] >= 1) out.at = [+a[1], +a[2]];
    const s = /^\s*(\d{1,3})x(\d{1,3})\s*$/.exec(d.span || "");
    if (s && +s[1] >= 1 && +s[2] >= 1) out.span = [+s[1], +s[2]];
    return out;
  }

  // pack 은 차례대로 놓는다. items = [{brk: true} | {sec: true} | {w, h?, at?, span?}] — w·h 는 span 이 없을 때의 너비·높이다(h 는 at 이 없을 때만 쓰고
  // 없으면 1 — build 는 자리·크기를 적지 않은 타일에만 autoScale 을 곱해 싣는다). 돌려주는 place[i] 는 타일이면 {r, c, w}(높이가 2 이상이면 h 도),
  // 구역 제목 줄이면 {r, sec: true}(한 줄을 다 쓴다), 줄 나눔이면 {r}(놓이는 것 없음). rows 는 쓴 줄 수, sections 는 구역 제목 줄의 번호들이다(격자 행 높이를 정한다).
  // 구역(줄 나눔·구역 줄 사이)마다 따로 놓고 위에서 아래로 쌓는다 — 한 구역의 높이는 그 안 타일이 쓴 마지막 행까지다(packSeg).
  // 자리를 지키지 못한 고정 타일(격자 밖·겹침)의 번호는 moved 에 싣는다 — 없으면 moved 키가 없다(속성 없는 문서의 결과는 예전과 같다).
  function pack(items, cols) {
    cols = Math.max(1, cols | 0);
    let r = 0;
    const place = [], sections = [], moved = [];
    for (let i = 0; i < items.length;) {
      const it = items[i];
      if (it.brk) { place[i++] = { r }; continue; }
      if (it.sec) { place[i++] = { r, sec: true }; sections.push(r); r++; continue; }
      let j = i;
      while (j < items.length && !items[j].brk && !items[j].sec) j++;
      r += packSeg(items, i, j, cols, r, place, moved);
      i = j;
    }
    const out = { place, rows: r, sections };
    if (moved.length) out.moved = moved;
    return out;
  }

  // packSeg 는 한 구역(items[a..b))의 타일을 base 줄부터 놓고 쓴 줄 수를 돌려준다.
  //   1) at 이 있는 타일을 읽는 차례로 먼저 놓는다. 크기는 span, 없으면 w × 1(그 열에서 격자 끝까지로 줄인다). 격자 밖이거나 앞선 고정 타일과
  //      겹치면 자동 배치로 돌린다(moved).
  //   2) 나머지는 읽는 차례대로 커서(행 우선)로 놓는다: 지금 줄 남은 칸에 너비가 안 들어가면 다음 줄 머리로, 들어가도 찬 칸과 겹치면 한 칸 오른쪽으로.
  //      커서는 뒤로 가지 않는다(앞의 빈 칸을 당겨 채우지 않는다).
  function packSeg(items, a, b, cols, base, place, moved) {
    const used = new Set();
    const free = (r, c, w, h) => {
      for (let y = r; y < r + h; y++) for (let x = c; x < c + w; x++) if (used.has(y * cols + x)) return false;
      return true;
    };
    const take = (k, r, c, w, h) => {
      for (let y = r; y < r + h; y++) for (let x = c; x < c + w; x++) used.add(y * cols + x);
      const p = { r: base + r, c, w };
      if (h > 1) p.h = h;
      place[k] = p;
      bottom = Math.max(bottom, r + h);
    };
    const size = it => {
      const s = it.span;
      if (s && s[0] >= 1 && s[0] <= cols && s[1] >= 1 && s[1] <= MAX_ROW) return [s[0], s[1], true];
      // h 는 자동 자리 타일만 쓴다 — at 이 있는 타일(자리를 못 지켜 자동으로 돈 것도)은 높이 1
      const h = it.at ? 1 : Math.min(Math.max(1, it.h | 0), MAX_ROW);
      return [Math.min(Math.max(1, it.w | 0), cols), h, false];
    };
    let bottom = 0;
    const fixed = new Set();
    for (let k = a; k < b; k++) {
      const at = items[k].at;
      if (!at) continue;
      const [C, R] = at;
      let [w, h, given] = size(items[k]);
      if (!given && C >= 1 && C <= cols) w = Math.min(w, cols - C + 1);
      if (!(C >= 1 && R >= 1 && C - 1 + w <= cols && R - 1 + h <= MAX_ROW) || !free(R - 1, C - 1, w, h)) { moved.push(k); continue; }
      take(k, R - 1, C - 1, w, h);
      fixed.add(k);
    }
    let cr = 0, cc = 0;
    for (let k = a; k < b; k++) {
      if (fixed.has(k)) continue;
      const [w, h] = size(items[k]);
      for (;;) {
        if (cc + w > cols) { cr++; cc = 0; continue; }
        if (free(cr, cc, w, h)) break;
        cc++;
      }
      take(k, cr, cc, w, h);
      cc += w;
    }
    return bottom;
  }

  // slots 는 pack 결과를 타일마다 구역 안 자리로 옮긴다 — 호스트(문서 창 끌기, itda-work/itda-hyve#1038 2단계)가 읽는 값이다.
  // 돌려주는 out[i] 는 타일이면 { seg(구역 번호 — 0부터, 줄 나눔·구역 줄마다 하나씩 는다), c·r(구역 안 열·행 — 1부터, at 과 같은 셈), w·h(놓인 너비·높이),
  // fixed(적은 at 대로 놓였는가) }, 타일이 아니면 null 이다. 빈 구역(줄 머리의 줄 나눔)도 번호를 하나 쓴다.
  function slots(items, place, moved) {
    const mv = new Set(moved || []);
    let seg = 0, base = 0;
    return items.map((it, i) => {
      const p = place[i];
      if (it.brk) { seg++; base = p.r; return null; }
      if (it.sec) { seg++; base = p.r + 1; return null; }
      return { seg, c: p.c + 1, r: p.r - base + 1, w: p.w, h: p.h || 1, fixed: !!it.at && !mv.has(i) };
    });
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

  // ---- 타일 상태(itda-work/itda-hyve#1038 3단계 B): 타일을 여는 제목의 class ok·warn·bad. 둘 이상이면 가장 무거운 것(bad > warn > ok).
  // build 가 타일에 tile-ok·tile-warn·tile-bad 를 달고 타일 머리 맨 앞에 표식 자리(span.tile-status)를 넣는다 — 표식 글자는 CSS(::before)가,
  // 읽는 프로그램이 읽을 글(「정상」·「주의」·「위험」)은 그 안의 숨긴 글(span.tile-status-label)이 맡는다(색만으로 뜻을 전하지 않는다).
  // 타일을 여는 제목이 아닌 곳(구역 제목 줄·타일 안 낮은 제목·쪽/이어보기/슬라이드)의 같은 class 는 문서의 자유 어휘라 아무것도 하지 않는다.
  const STATUS = ["bad", "warn", "ok"]; // 무거운 차례
  const STATUS_LABEL = { ok: "정상", warn: "주의", bad: "위험" };

  // tileStatus 는 타일 제목의 상태("ok"·"warn"·"bad", 없으면 "")다.
  function tileStatus(title) {
    const c = title && title.classList;
    if (!c) return "";
    return STATUS.find(s => c.contains(s)) || "";
  }

  // markStatus 는 tile 에 상태를 옮겨 적는다(head 는 타일 머리). 상태가 없으면 아무것도 하지 않는다 — 상태 없는 문서의 마크업은 전과 같다.
  function markStatus(tile, head, st) {
    if (!st) return;
    tile.classList.add("tile-" + st);
    const s = el("span", "tile-status tile-status-" + st);
    s.appendChild(el("span", "tile-status-label", STATUS_LABEL[st]));
    head.insertBefore(s, head.firstChild);
  }

  // ---- 타일 잇기(itda-work/itda-hyve#1038 3단계 D): 타일을 여는 제목의 to=도착 id[,…](mdoc boardlink.go 가 검사해 data-to 로 보낸다).
  // links 가 보드 한 장의 격자(.board-grid) 맨 끝에 선 층 하나(svg.board-links — aria-hidden)를 넣는다. 선 층은 z-index -1 이라 타일·구역 줄 밑,
  // 보드 바탕 위에 깔린다(쪽 상자 .page 가 contain: paint 로 쌓임 맥락이다) — 타일 사이 틈에서 보이고, 사이에 낀 타일 밑으로 지나 가려진다(돌아가는 길은 찾지 않는다).
  // 좌표는 격자 안 CSS px(타일의 offsetLeft·offsetTop·offsetWidth·offsetHeight — transform·확대에 흔들리지 않는다)이고 선 층도 격자에 붙어 보드와 함께 확대·축소·이동한다.
  // 선마다 g.board-link 에 data-board-from·data-board-to(출발·도착 제목 id)를 남긴다. 화살촉은 marker 가 아니라 채운 삼각형 path 다 — svg 에 id 가 없어
  // 발표가 보드를 복제해도(present.js startBoard — 복제의 id 를 지운다) 겹치거나 끊길 id 참조가 없다. 읽는 프로그램에는 출발 타일 머리의 숨긴 글
  // (span.tile-links-label 「→ 도착 제목」)이 간다. to 가 하나도 없으면 아무것도 만들지 않는다(마크업이 이 기능 전과 같다).
  const SVGNS = "http://www.w3.org/2000/svg";
  const LINK_STEP = 14; // 한 가장자리에 여럿 닿을 때 벌리는 간격(px)의 상한 — 가장자리 길이 / (선 수 + 1) 이 더 작으면 그것

  // linkTargets 는 타일 제목의 data-to 를 도착 id 목록으로 읽는다(쉼표로 나눔, 항목 앞뒤 공백 뗌, 빈 항목·겹침 뺌 — mdoc 이 이미 뗀 것에 대한 방어).
  function linkTargets(title) {
    const d = title && title.dataset ? title.dataset.to : "";
    const out = [];
    for (const s of String(d || "").split(",")) { const id = s.trim(); if (id && !out.includes(id)) out.push(id); }
    return out;
  }

  // linkSides 는 a 에서 b 로 가는 선이 나가는 a 의 가장자리와 들어가는 b 의 가장자리다. a·b = {x, y, w, h}(격자 안 px).
  // b 가 a 보다 온전히 오른쪽이면 오른쪽 → 왼쪽, 온전히 왼쪽이면 왼쪽 → 오른쪽. 열이 겹치면 b 가 아래면 아래 → 위, 아니면 위 → 아래.
  function linkSides(a, b) {
    if (b.x >= a.x + a.w) return ["right", "left"];
    if (b.x + b.w <= a.x) return ["left", "right"];
    return b.y >= a.y + a.h ? ["bottom", "top"] : ["top", "bottom"];
  }

  // linkEnds 는 선마다 두 끝의 자리를 정한다. rects[i] = 타일 i 의 사각형, links = [[출발, 도착], …](타일 번호).
  // 돌려주는 out[k] = { sides: [나가는, 들어가는 가장자리], p: [[x, y], [x, y]], rank, of } — rank·of 는 출발 가장자리에서의 차례·그 가장자리에 닿는 끝 수(꺾는 자리를 나눈다).
  // 한 타일의 한 가장자리에 닿는 끝(나가는 것·들어오는 것)은 그 가장자리를 따라 벌린다 — 상대 타일의 가운데(세로 가장자리는 y, 가로 가장자리는 x) 차례로,
  // 같으면 선 차례·끝 차례로. 간격은 min(LINK_STEP, 가장자리 길이 / (끝 수 + 1)), 가운데를 기준으로 고르게 놓는다. 그 뒤 혼자 닿는 끝은 맞은편 끝의 줄로
  // 맞춰 곧은 선이 되게 한다(아래 「곧게 맞추기」). 같은 입력이면 같은 자리다.
  function linkEnds(rects, links) {
    const out = links.map(([f, t]) => ({ sides: linkSides(rects[f], rects[t]), p: [null, null], n: [1, 1], rank: 0, of: 1 }));
    const groups = new Map();
    links.forEach(([f, t], k) => {
      for (const [me, other, end] of [[f, t, 0], [t, f, 1]]) {
        const key = me + ":" + out[k].sides[end];
        if (!groups.has(key)) groups.set(key, []);
        groups.get(key).push({ k, end, me, other });
      }
    });
    for (const g of groups.values()) {
      const r = rects[g[0].me], side = out[g[0].k].sides[g[0].end];
      const vert = side === "left" || side === "right"; // 세로 가장자리 — y 를 따라 벌린다
      const mid = q => (vert ? q.y + q.h / 2 : q.x + q.w / 2);
      g.sort((a, b) => mid(rects[a.other]) - mid(rects[b.other]) || a.k - b.k || a.end - b.end);
      const n = g.length, step = Math.min(LINK_STEP, (vert ? r.h : r.w) / (n + 1)), c = mid(r);
      const edge = side === "left" ? r.x : side === "right" ? r.x + r.w : side === "top" ? r.y : r.y + r.h;
      g.forEach((e, i) => {
        const v = c + (i - (n - 1) / 2) * step;
        out[e.k].p[e.end] = vert ? [edge, v] : [v, edge];
        out[e.k].n[e.end] = n;
        if (e.end === 0) { out[e.k].rank = i; out[e.k].of = n; }
      });
    }
    // 곧게 맞추기: 두 가장자리가 마주 보는 구간(세로 가장자리면 y, 가로면 x 의 겹침)이 있으면, 혼자 닿는 끝을 맞은편 끝의 줄로 옮겨 곧은 선이 되게 한다 —
    // 둘 다 혼자면 겹침의 가운데로, 한쪽만 혼자면 벌린 쪽의 자리가 겹침 안(양 끝 2px 안쪽)일 때 그 자리로. 둘 다 여럿이면 벌린 자리 그대로다.
    links.forEach(([f, t], k) => {
      const e = out[k], a = rects[f], b = rects[t], vert = e.sides[0] === "left" || e.sides[0] === "right", ax = vert ? 1 : 0;
      const lo = vert ? Math.max(a.y, b.y) : Math.max(a.x, b.x), hi = vert ? Math.min(a.y + a.h, b.y + b.h) : Math.min(a.x + a.w, b.x + b.w);
      if (!(hi - lo > 0)) return;
      const inside = v => v >= lo + 2 && v <= hi - 2;
      if (e.n[0] === 1 && e.n[1] === 1) e.p[0][ax] = e.p[1][ax] = (lo + hi) / 2;
      else if (e.n[1] === 1 && inside(e.p[0][ax])) e.p[1][ax] = e.p[0][ax];
      else if (e.n[0] === 1 && inside(e.p[1][ax])) e.p[0][ax] = e.p[1][ax];
    });
    return out.map(e => ({ sides: e.sides, p: e.p, rank: e.rank, of: e.of }));
  }

  // linkRoute 는 p0(나가는 가장자리 side 위)에서 p1 로 가는 직각 꺾은선의 점들이다. 두 끝이 한 줄이면(0.5px 안) 곧은 선 [p0, p1'] —
  // p1' 은 p0 와 같은 줄로 맞춘 끝이다. 아니면 출발 옆 틈에서 두 번 꺾는다: 나가는 방향으로 off 만큼 간 자리에서 꺾어 p1 의 줄로 간 뒤 p1 로.
  function linkRoute(p0, side, p1, off) {
    const [x0, y0] = p0, [x1, y1] = p1;
    if (side === "left" || side === "right") {
      if (Math.abs(y0 - y1) < 0.5) return [p0, [x1, y0]];
      const bx = x0 + (side === "right" ? off : -off);
      return [p0, [bx, y0], [bx, y1], p1];
    }
    if (Math.abs(x0 - x1) < 0.5) return [p0, [x0, y1]];
    const by = y0 + (side === "bottom" ? off : -off);
    return [p0, [x0, by], [x1, by], p1];
  }

  // linkHead 는 꺾은선 pts 의 끝에 붙는 화살촉이다 — { line: 촉 밑동까지 줄인 점들, head: 삼각형 세 점 }. len = 촉 길이(틈 안에 보이게 — linkSize).
  function linkHead(pts, len) {
    const [ex, ey] = pts[pts.length - 1], [px, py] = pts[pts.length - 2];
    const d = Math.hypot(ex - px, ey - py) || 1, ux = (ex - px) / d, uy = (ey - py) / d, w = len * 0.45;
    const bx = ex - ux * len, by = ey - uy * len;
    return { line: pts.slice(0, -1).concat([[bx + ux * 0.5, by + uy * 0.5]]), head: [[ex, ey], [bx - uy * w, by + ux * w], [bx + uy * w, by - ux * w]] };
  }

  // linkSize 는 틈 폭(px)에 맞춘 화살촉 길이다 — 틈의 0.75 배를 5px 와 10px 사이로 자른 값(틈보다 길지 않다 — 촉이 타일 밑에 숨지 않게).
  function linkSize(gap) { return Math.min(gap, Math.max(5, Math.min(10, gap * 0.75))); }

  // linkGeometry 는 순수한 그림 계산이다: rects·links·gap(격자 틈 px) → 선마다 { line, head }(점 좌표는 0.1px 로 반올림). 꺾는 자리는 출발 옆 틈의 앞부분
  // ((틈 − 촉 길이) 안)을 그 가장자리 끝 수로 나눠 정한다 — 같은 가장자리에서 나가는 선의 꺾는 줄이 겹치지 않고, 꺾은 뒤 도착까지 촉 길이 넘게 남는다.
  // gap 은 잰 틈 그대로다 — 0 이면 0 이고, 잴 수 없을 때(NaN·음수·무한)만 12px 로 본다. 틈이 0 이면(타일이 맞붙음) 촉을 둘 자리가 없어 촉은 없고(head 가 빈 목록)
  // 선은 줄이지 않고 도착 가장자리에서 끝난다. 꺾는 자리는 출발 가장자리 위다(타일 밑에 깔려 빈 칸을 지나는 부분만 보인다). 같은 점이 잇따르면 하나로 줄인다.
  function linkGeometry(rects, links, gap) {
    const g = Number.isFinite(gap) && gap >= 0 ? gap : 12, len = linkSize(g), r1 = v => Math.round(v * 10) / 10;
    const dedupe = ps => ps.filter((p, i) => !i || p[0] !== ps[i - 1][0] || p[1] !== ps[i - 1][1]);
    return linkEnds(rects, links).map(e => {
      const off = (g - len) * (e.rank + 1) / (e.of + 1);
      const route = linkRoute(e.p[0], e.sides[0], e.p[1], off);
      if (!(len > 0)) return { line: dedupe(route.map(p => p.map(r1))), head: [], sides: e.sides };
      const h = linkHead(route, len);
      return { line: h.line.map(p => p.map(r1)), head: h.head.map(p => p.map(r1)), sides: e.sides };
    });
  }

  // links 는 보드 한 장(page — .page.board)의 선 층을 (다시) 그린다. 돌려주는 값은 그린 선 수다. build 가 넘침 처리 뒤에 부른다.
  // 옛 선 층·읽는 프로그램용 글을 먼저 지우므로 거듭 불러도 같다. 도착은 같은 보드의 타일 제목 id 로 찾는다 — 없는 id·자기 자신·겹침은 버린다.
  // 좌표는 격자 안 px 라 보드를 옮기거나(공유 보는 화면의 무대 → #pages) 확대해도 다시 그릴 일이 없다. 격자를 잴 수 없으면(display: none — 폭 0)
  // 선 층을 만들지 않는다(읽는 프로그램용 글은 단다) — 그런 호스트는 보인 뒤 ItdaBoard.links(page) 를 부른다. 발표의 복제본은 id 가 지워져 다시 그릴 수 없다 —
  // 복제할 때 따라간 선 층이 그대로 맞다(폭이 같다).
  function links(page) {
    // 자식만 훑는다(선택자 해석 없이) — to 가 없는 보드에서는 지울 것도 그릴 것도 없이 끝난다
    const kids = e => (e ? [...e.children] : []);
    const grid = kids(page).find(c => c.classList.contains("board-grid"));
    if (!grid) return 0;
    const tiles = kids(grid).filter(c => c.classList.contains("tile"));
    const thead = t => kids(t).find(c => c.classList.contains("tile-head"));
    for (const x of kids(grid)) if (x.classList.contains("board-links")) x.remove();
    // 지우는 것은 links 가 만든 직계 span.tile-links-label 뿐이다 — 문서가 제목에 같은 class 를 달아도(자유 어휘) 제목은 건드리지 않는다
    const ours = x => x.tagName === "SPAN" && x.classList.contains("tile-links-label");
    for (const t of tiles) for (const x of kids(thead(t))) if (ours(x)) x.remove();
    const heads = tiles.map(t => kids(thead(t)).find(isHeading) || null);
    // 같은 id 의 타일이 둘이면(직접 적은 id 의 겹침 — mdoc 은 duplicate-id 경고와 함께 둘 다 남긴다) 도착은 읽는 차례로 첫 타일이다.
    // 자기 id 와 같은 도착은 버린다(mdoc boardLinks 의 「자기 자신」 과 같은 판정 — 겹친 id 의 둘째 타일이 첫 타일로 잇지 않는다)
    const byId = new Map();
    heads.forEach((h, i) => { if (h && h.id && !byId.has(h.id)) byId.set(h.id, i); });
    const pairs = [];
    heads.forEach((h, i) => {
      for (const id of linkTargets(h)) {
        const j = byId.get(id);
        if (j != null && j !== i && id !== h.id) pairs.push([i, j]);
      }
    });
    if (!pairs.length) return 0;
    // 읽는 프로그램용 글: 출발 타일 머리 끝에 「→ 도착 제목」(여럿이면 쉼표로) — 눈에는 안 보인다(base.css .tile-links-label)
    const name = j => squash(heads[j].textContent);
    tiles.forEach((t, i) => {
      const to = pairs.filter(p => p[0] === i).map(p => "→ " + name(p[1]));
      if (to.length) thead(t).appendChild(el("span", "tile-links-label", to.join(", ")));
    });
    const W = grid.offsetWidth, H = grid.offsetHeight;
    if (!(W > 0 && H > 0)) return 0;
    const rects = tiles.map(t => ({ x: t.offsetLeft, y: t.offsetTop, w: t.offsetWidth, h: t.offsetHeight }));
    const cs = typeof getComputedStyle === "function" ? getComputedStyle(grid) : null;
    const gap = cs ? parseFloat(cs.columnGap) : NaN;
    const geo = linkGeometry(rects, pairs, gap);
    const svg = document.createElementNS(SVGNS, "svg");
    svg.setAttribute("class", "board-links");
    svg.setAttribute("aria-hidden", "true");
    svg.setAttribute("focusable", "false");
    svg.setAttribute("width", String(W));
    svg.setAttribute("height", String(H));
    svg.setAttribute("viewBox", "0 0 " + W + " " + H);
    const pts = ps => ps.map((p, i) => (i ? "L" : "M") + p[0] + " " + p[1]).join(" ");
    geo.forEach((g, k) => {
      const a = svg.appendChild(document.createElementNS(SVGNS, "g"));
      a.setAttribute("class", "board-link");
      a.setAttribute("data-board-from", heads[pairs[k][0]].id);
      a.setAttribute("data-board-to", heads[pairs[k][1]].id);
      const line = a.appendChild(document.createElementNS(SVGNS, "path"));
      line.setAttribute("class", "board-link-line");
      line.setAttribute("d", pts(g.line));
      if (!g.head.length) return; // 틈 0 — 촉 없음(linkGeometry)
      const head = a.appendChild(document.createElementNS(SVGNS, "path"));
      head.setAttribute("class", "board-link-head");
      head.setAttribute("d", pts(g.head) + " Z");
    });
    grid.appendChild(svg);
    return pairs.length;
  }

  // ---- 움직임(3단계 C): 처음 그릴 때 등장, 다시 그릴 때 본문이 바뀐 타일 강조. 움직임은 base.css 의 CSS animation 만이다 —
  // JS 는 class(tile-enter·tile-changed)와 늦춤(--tile-enter-delay)만 달고, 끝나면(animationend) 뗀다. 그 이벤트가 오지 않을 때의 늦은 끝은 lifeMs(계산된 시간 + 늦춤 + 0.5초, 최소 SETTLE_MS)이고,
  // 늦은 끝에 타일에서 아직 도는 애니메이션이 있으면 그 끝까지 다시 기다린다(rest). 그래서 build 를 문서 밖·animation 을 끈 무대에서 불러 lifeMs 가 최소값을 읽어도,
  // 무대에서 그린 뒤 타일을 다른 부모로 옮기다 애니메이션이 새 자리에서 다시 시작해도 테마가 늘린 시간이 잘리지 않는다.
  // 끝 모양이 기본 모양이라 애니메이션이 돌지 않아도(인쇄·움직임 줄이기·모르는 엔진) 타일은 다 보인다.
  const ENTER_STEP = 40, ENTER_SPREAD = 300; // 타일 사이 늦춤(ms)·늦춤의 상한 — 등장 시간(base.css --board-enter-time 0.3초)과 합쳐 0.6초 안
  const SETTLE_MS = { "tile-enter": 1200, "tile-changed": 2000 }; // animationend 가 오지 않을 때(돌지 않음·화면 밖) class 를 떼는 늦은 끝의 최소값 — 실제 값은 lifeMs
  const SETTLE_WAITS = 4; // 늦은 끝에 아직 도는 애니메이션을 다시 기다리는 횟수의 상한 — 호스트가 타일을 거듭 떼었다 붙여 애니메이션이 거듭 다시 시작해도 표지가 끝내 떨어진다
  const memo = new WeakMap(); // 대상(또는 o.motionScope) → 앞 그림의 { doc: 문서 열쇠(o.motionDoc), tiles: 짝 열쇠 → 본문 글 } — 대상이 사라지면 함께 사라진다

  // enterDelays 는 타일 n 개의 등장 늦춤(ms)이다 — 읽는 차례로 ENTER_STEP 씩, 마지막이 ENTER_SPREAD 를 넘지 않게 간격을 줄인다.
  function enterDelays(n) {
    const step = n > 1 ? Math.min(ENTER_STEP, ENTER_SPREAD / (n - 1)) : 0;
    return Array.from({ length: Math.max(0, n) }, (_, i) => Math.round(i * step));
  }

  // tileKeys 는 타일마다 짝 맞추기 열쇠다: 제목 글(공백을 한 칸으로) + "#" + 같은 제목 글 안의 차례(0부터). 제목 없는 타일은 빈 제목으로 센다.
  // 자리(at·span·배치)는 열쇠에 넣지 않는다 — 자리만 바뀐 타일은 같은 타일이다.
  const squash = s => String(s || "").replace(/\s+/g, " ").trim();
  function tileKeys(titles) {
    const seen = new Map();
    return titles.map(t => {
      const k = squash(t);
      const n = seen.get(k) || 0;
      seen.set(k, n + 1);
      return k + "#" + n;
    });
  }

  // settle 은 움직임 class 를 끝나면 뗀다 — 크게 보기·호스트의 끌기 transform 과 부딪치지 않게, 화면 밖에 있다가 나중에 다시 돌지 않게.
  // animationcancel 로는 떼지 않는다 — 타일을 떼었다 붙이면 애니메이션이 취소된 뒤 새 자리에서 다시 돌 수 있고, 늦게 온 취소로 떼면 그 등장이 끊긴다.
  // 취소된 채 끝난 것(도중에 움직임 줄이기로 바뀜 등)은 늦은 끝이 뗀다. 늦은 끝(late)은 아직 도는 애니메이션이 있으면 그 끝 + 0.5초까지 다시 기다린다(SETTLE_WAITS 번까지).
  function settle(tile, cls) {
    let done = false, timer = 0, waits = 0;
    const off = e => {
      if (done || (e && e.target !== tile)) return;
      done = true;
      tile.classList.remove(cls);
      if (cls === "tile-enter") tile.style.removeProperty("--tile-enter-delay");
      if (tile.removeEventListener) tile.removeEventListener("animationend", off);
      if (timer) clearTimeout(timer);
    };
    const late = () => {
      timer = 0;
      const left = done ? 0 : rest(tile);
      if (left > 0 && waits++ < SETTLE_WAITS) timer = setTimeout(late, left + 500);
      else off();
    };
    // 이벤트·타이머가 없는 자리(화면 없이 HTML 만 만드는 호스트·시험의 가짜 DOM)에서도 build 가 멈추지 않게 있는 것만 쓴다 — class 가 남아도 끝 모양이 기본 모양이다
    if (tile.addEventListener) tile.addEventListener("animationend", off);
    if (typeof setTimeout === "function") timer = setTimeout(late, lifeMs(tile, cls));
  }
  // rest 는 타일 자신에 걸려 지금 도는(running·pending) CSS animation 이 끝날 때까지 남은 시간(ms, 가장 긴 것)이다. 없거나 셀 수 없으면 0 —
  // 끝없는 것(infinite)·멈춘 것(paused)·끝난 것·transition 은 세지 않는다(기다려도 끝나지 않거나 표지와 상관없다). 문서에서 떨어진 타일은 애니메이션이 없어 0 이다.
  function rest(tile) {
    if (typeof tile.getAnimations !== "function") return 0;
    let left = 0;
    for (const a of tile.getAnimations()) {
      if (typeof a.animationName !== "string" || (a.playState !== "running" && a.playState !== "pending")) continue;
      const t = a.effect && typeof a.effect.getComputedTiming === "function" ? a.effect.getComputedTiming() : null;
      if (!t || !isFinite(t.endTime)) continue;
      left = Math.max(left, t.endTime - (Number(a.currentTime) || 0));
    }
    return left > 0 ? Math.ceil(left) : 0;
  }
  // lifeMs 는 늦은 끝 타이머의 처음 시간이다 — 테마가 움직임 토큰(--board-enter-time·--board-enter-stagger·--board-flash-time)을 늘려도 도는 애니메이션을
  // 자르지 않게, 계산된 animation-duration + animation-delay(쉼표 목록이면 가장 긴 것)에 여유 0.5초를 더한다. 그 값이 SETTLE_MS 보다 짧거나 읽을 수 없으면
  // (움직임 줄이기 — animation 없음, 문서에서 떨어진 타일 — 계산 스타일이 비어 있다, animation 을 끈 무대) SETTLE_MS 다 — 그때도 늦은 끝의 rest 가 자르지 않게 한다.
  // visibility: hidden·display: none 인 무대(문서에 붙음)에서는 계산 스타일을 그대로 읽는다(Chrome·WebKit 재현).
  function lifeMs(tile, cls) {
    const cs = typeof getComputedStyle === "function" ? getComputedStyle(tile) : null;
    const ms = v => String(v || "").split(",").map(s => { s = s.trim(); const n = parseFloat(s); return isFinite(n) ? (/ms$/.test(s) ? n : n * 1000) : 0; });
    const delay = cs ? ms(cs.animationDelay) : [];
    const end = cs ? Math.max(0, ...ms(cs.animationDuration).map((d, i) => d + (delay[i] || 0))) : 0;
    return Math.max(SETTLE_MS[cls], Math.ceil(end) + 500);
  }

  // enter 는 page(보드 한 장) 의 타일에 등장을 건다 — 읽는 차례로 늦춘다. 남은 움직임 표지는 먼저 지운다(발표가 복제한 보드 — present.js startBoard).
  // 돌려주는 값은 등장을 건 타일 수다.
  function enter(page) {
    const tiles = [...page.querySelectorAll(".page-body.board-grid > .tile")];
    const delays = enterDelays(tiles.length);
    tiles.forEach((t, i) => {
      t.classList.remove("tile-enter", "tile-changed");
      t.style.setProperty("--tile-enter-delay", delays[i] + "ms");
      t.classList.add("tile-enter");
      settle(t, "tile-enter");
    });
    return tiles.length;
  }

  // motion 은 build 의 끝에서 앞 그림과 견준다. scope = 기억을 묶을 요소, tiles = [{tile, title(제목 글), sig(본문 글)}], on = 움직임을 쓰는가.
  // 처음 그림이면 등장, 다시 그림이면 같은 열쇠인데 본문 글이 달라진 타일만 강조. 기억은 on 이 아니어도 늘 새로 적는다(끈 동안 그린 것도 앞 그림이다).
  // doc 은 그리는 문서의 열쇠다(o.motionDoc — 없으면 undefined). 앞 그림과 열쇠가 다르면(다른 문서) 앞 그림이 없는 것으로 본다 — 처음 그림처럼 등장하고,
  // 다른 문서의 같은 제목 타일을 「바뀐 타일」로 강조하지 않는다. 열쇠는 === 로 견준다. ent 가 거짓이면 처음 그림이어도 등장하지 않는다(o.enter === false — 강조는 그대로).
  function motion(scope, page, tiles, on, doc, ent) {
    const last = memo.get(scope);
    const prev = last && last.doc === doc ? last.tiles : null;
    const keys = tileKeys(tiles.map(t => t.title));
    const now = new Map(keys.map((k, i) => [k, tiles[i].sig]));
    memo.set(scope, { doc, tiles: now });
    const out = { entered: 0, changed: 0 };
    if (!on) return out;
    if (!prev) { if (ent) out.entered = enter(page); return out; }
    tiles.forEach((t, i) => {
      if (!prev.has(keys[i]) || prev.get(keys[i]) === t.sig) return;
      t.tile.classList.add("tile-changed");
      settle(t.tile, "tile-changed");
      out.changed++;
    });
    return out;
  }

  // forget 은 scope(대상 또는 motionScope)에 묶인 앞 그림의 기억을 버린다 — 다음 build 는 처음 그림이다. 돌려주는 값은 버린 것이 있었는가.
  function forget(scope) { return memo.delete(scope); }

  // headText 는 표지·제목 블록에서 고른 글자다(o.info 가 앞선다 — slides.js titleSlide 와 같다).
  function headText(src, o, sel, key) {
    const info = o.info || {};
    if (info[key]) return info[key];
    const x = src && src.querySelector(sel);
    return x ? x.textContent.trim() : "";
  }

  // build 는 flow(문서 흐름 HTML 이 든 요소)를 target 안의 보드 한 장(.page.board)으로 놓는다. target 은 무대(잴 수 있는 자리)에 있어야 한다.
  // o = { ratio, cols, rows, level, info, logo, motion, enter, motionScope, motionDoc }. motion === false 면 등장·강조가 없다(편집 미리보기처럼 자주 다시 그리는 화면 — 없으면 켜짐).
  // motionScope 는 앞 그림의 기억을 묶을 요소다(없으면 target — 그릴 때마다 새 target 을 만드는 호스트는 늘 남는 요소를 준다).
  // motionDoc 은 문서의 열쇠다 — 앞 그림과 다르면(=== 아님) 처음 그림으로 다룬다(한 자리에서 여러 문서를 바꿔 그리는 호스트). 기억을 아예 버리려면 forget(scope).
  // enter === false 면 등장만 끈다(강조는 그대로 — 그린 직후 타일 안 블록 자리를 재서 굴리는 화면, 예: 공유 보는 화면의 발표 따라가기).
  // 돌려주는 값: { overflow(「…」 로 잘린 타일 수), ms, moved(자리를 지키지 못한 고정 타일 수), links(그린 선 수 — 타일 잇기), entered(등장을 건 타일 수), changed(강조한 타일 수) — 뒤 넷은 있을 때만 }
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
    const scale = autoScale(cols, rows);
    const packed = items.map(it => (it.kind === "break" ? { brk: true } : it.kind === "section" ? { sec: true }
      : tileItem(tileWidth(it.blocks), tileAttrs(it.title), scale)));
    const { place, rows: used, sections, moved } = pack(packed, cols);
    const slot = slots(packed, place, moved);
    const meta = [headText(g.head, o, ".cover-org, .title-org", "organization"), headText(g.head, o, '.meta-row[data-key="date"] dd', "date")].filter(Boolean).join(" · ");
    const pg = el("div", "page board");
    pg.dataset.kind = "board";
    pg.dataset.steps = "0";
    pg.dataset.slide = pg.dataset.page = "1";
    pg.style.setProperty("--board-rows", String(rows));
    pg.appendChild(el("div", "board-deco"));
    const head = pg.appendChild(el("header", "board-head"));
    // 원본 표지의 id 를 머리가 갖는다(itda-hyve#979 — slides.js titleSlide 와 같다). 원본 표지는 보드로 옮기지 않아 흐름과 함께 버려진다.
    if (g.head && g.head.id) head.id = g.head.id;
    if (o.logo) { const im = head.appendChild(el("img", "bh-logo")); im.src = o.logo; im.alt = ""; }
    const title = head.appendChild(el("div", "bh-title", headSec ? null : docTitle));
    if (headSec) title.appendChild(headSec.title);
    if (meta) head.appendChild(el("div", "bh-meta", meta));
    const grid = pg.appendChild(el("div", "page-body board-grid"));
    grid.style.gridTemplateColumns = "repeat(" + cols + ", minmax(0, 1fr))";
    grid.dataset.boardCols = String(cols); // 호스트 끌기가 읽는 열 수(slots 와 함께 — itda-work/itda-hyve#1038 2단계)
    // 행 높이를 줄마다 적는다 — 타일 줄은 고정, 구역 제목 줄은 자기 높이. 크게 보기가 타일을 띄워도 줄이 그대로 남는다
    const isSec = new Set(sections);
    grid.style.gridTemplateRows = Array.from({ length: used }, (_, r) => (isSec.has(r) ? "auto" : "var(--board-row-h)")).join(" ");
    target.appendChild(pg);
    const seen = [];
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
      tile.style.gridRow = p.h > 1 ? (p.r + 1) + " / span " + p.h : String(p.r + 1);
      // 놓인 자리(slots): 구역 번호·구역 안 열,행·너비x높이·적은 자리대로인가. 화면 부품(호스트)이 읽기만 한다 — 배치·테마는 이 값을 쓰지 않는다
      const sl = slot[i];
      tile.dataset.boardSeg = String(sl.seg);
      tile.dataset.boardAt = sl.c + "," + sl.r;
      tile.dataset.boardSpan = sl.w + "x" + sl.h;
      if (sl.fixed) tile.dataset.boardFixed = "";
      if (it.title) {
        const th = tile.appendChild(el("header", "tile-head"));
        th.appendChild(it.title);
        markStatus(tile, th, tileStatus(it.title));
      }
      const body = tile.appendChild(el("div", "tile-body"));
      for (const b of it.blocks) body.appendChild(b);
      grid.appendChild(tile);
      // 짝 맞추기용 제목 글·본문 글은 넘침 처리(「… n행 더」·숨긴 행) 전에 잰다 — 칸 크기가 바뀌어도 본문이 같으면 같은 글이다
      seen.push({ tile, title: it.title ? it.title.textContent : "", sig: squash(body.textContent) });
    });
    // 넓은 수식을 타일 너비에 맞춘 뒤에 넘침을 잰다 — 줄어든 높이로 재야 수식이 있는 타일이 필요 없이 잘리지 않는다(보드는 쪽 나눔을 지나지 않는다, itda-hyve#752)
    window.ItdaPager.fitMath(pg);
    let overflow = 0;
    for (const t of target.querySelectorAll(".page.board .tile")) if (fitTile(t)) overflow++;
    // 타일 잇기(to): 타일 크기가 정해진 뒤(넘침 처리 뒤)에 선 층을 그린다. to 가 없으면 아무것도 만들지 않는다
    const linked = links(pg);
    // moved: 적은 자리를 지키지 못해 자동 배치로 돌린 타일 수(내용 너비 때문에야 드러난 겹침 — mdoc 경고가 없는 경우). 없으면 키가 없다
    // 움직임: 이 대상(o.motionScope 가 있으면 그것)에 처음 그리면 등장, 다시 그리면 본문이 바뀐 타일 강조. o.motion === false 면 둘 다 없다
    const mv = motion(o.motionScope || target, pg, seen, o.motion !== false, o.motionDoc, o.enter !== false);
    const out = { overflow, ms: Math.round(performance.now() - t0) };
    if (moved) out.moved = moved.length;
    if (linked) out.links = linked;
    if (mv.entered) out.entered = mv.entered;
    if (mv.changed) out.changed = mv.changed;
    return out;
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
  window.ItdaBoard = { build, groups, pack, slots, tileWidth, tileAttrs, autoScale, tileItem, tileStatus, tileKeys, enterDelays, enter, forget, zoomer, zoomBox, zoomHit, zoomStep, zoomKey,
    links, linkTargets, linkSides, linkEnds, linkRoute, linkGeometry };
})();
