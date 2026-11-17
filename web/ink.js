// 판서 — 슬라이드 발표(present.js) 위의 레이저·펜·형광펜·지우개·되돌리기·전부 지우기·도구 막대.
// 판서는 메모리에만 있고 md 원문·저장 상태를 건드리지 않는다. 발표 슬라이드는 복제본이라 모델은 DOM 밖에(슬라이드 번호 + 설계 px) 둔다 — 공유 중계(relay)가 그대로 쓴다.
// 획 모양(경로·색·바탕 밝기)은 inkshape.js 에 있다 — 공유 보는 화면과 같은 규칙이다.
// 중계: relay 로 나가는 길을 이으면 모델을 바꿀 때마다 연산을 낸다. 공유·DOM 을 모른다 — 잇지 않으면 연산을 만들지 않는다(공유가 꺼져 있으면 비용이 없다).
//
// 층(슬라이드 .page 의 자식 — .page 는 contain: paint 로 쌓임 맥락이다):
//   .ink-hl   끝난 형광펜 SVG. 혼합(곱하기, 어두운 바탕은 스크린)·투명도를 경로가 아니라 층에 건다 — 층이 .page 맥락에서 글자·바탕과 섞이고,
//             층에 쌓임 맥락이 생겨도 혼합이 격리되지 않으며, 형광펜이 몇 개든 합성은 한 번이다(스파이크의 비용은 경로마다 곱하기를 건 값이다).
//   .ink-pen  끝난 펜 SVG. 형광펜 위에 온다.
//   .ink-live 진행 중인 획 canvas(뒷버퍼 설계×배율×dpr) 하나. 지금 슬라이드로 옮겨 다닌다. 끝난 획과 같은 경로(pathOf)를 프레임마다 그리고, 획이 끝나면
//             같은 태스크에서 SVG 로 옮기고 지운다.
// 레이저는 슬라이드 밖 창 전체 canvas(.ink-laser)다 — 판서가 아니라 남지 않는다.
// 스타일은 CSP(style-src 'self') 때문에 style 속성을 쓰지 않는다 — 클래스·CSSOM·SVG 표현 속성만 쓴다.
(function () {
  "use strict";

  const LIMIT = { strokes: 2000, points: 200000 }; // 문서 합계 상한(잠정) — 닿으면 새 획을 받지 않고 막대에 알린다
  const { COLORS, HL_SKIP, WIDTHS, LASER_TAIL, LASER_LINE, r1, widthOf, filled, pathOf, dress, parseColor, luminance, slideDark } = window.ItdaInkShape;
  const SMOOTH = 0.5;      // 흔들림 보정 — 새 입력을 따라가는 비율(1 이면 보정 없음)
  const MIN_STEP = 0.6;    // 이보다 가까운 점은 버린다(설계 px)
  const ERASE_R = 8;       // 지우개 반지름(설계 px)
  const UNDO_MAX = 200;    // 슬라이드마다 되돌리기 칸 수
  const LASER_DRAG = 6;    // 레이저: 이만큼(화면 px) 움직이면 클릭이 아니라 선이다
  const RELAY_MS = 33;     // 중계 묶음 간격(ms)
  const SVG_NS = "http://www.w3.org/2000/svg";
  // 판서가 받지 않는 자리 — 노트·목차·막대·코드 단추·슬라이드 묻기 단추(.itda-askbar — ask.js) 등은 present.js onMouse 의 제외 목록과 같고, YouTube 재생(a.yt·.yt-play)은 호스트가 mousedown 을 창 단계에서 가로채므로 여기서 뺀다.
  // 호스트의 겹침 층([data-pr-overlay] — present.js 머리 주석 「호스트 계약」)도 판서 자리가 아니다
  const BLOCKED = ".pr-notes, .pr-hud, .pr-notes-grip, .pr-toc, .pr-toc-grip, .pr-ink-bar, .pr-lightbox, .itda-codecopy, .itda-codetitle, .itda-askbar, a.yt, .yt-play, [data-pr-overlay]";
  const LIMIT_NOTE = "판서 상한(2천 획·20만 점)에 닿았습니다 — 지우면 다시 그릴 수 있습니다";
  const RELAY_NOTE = "판서가 공유 상한을 넘어 보는 사람에게 보내지 못합니다 — 공유 패널의 「판서 함께 보이기」 를 껐다 켜면 다시 시도합니다";

  // ---- 모델: 슬라이드 번호 → 획 목록. 획은 {id(그 판서 안 번호), tool: "pen"|"hl", color(팔레트 번호), width(0~2), pts[x,y,…], press[0~1,…]|null, box[x0,y0,x1,y1]}.
  // 되돌리기는 조작 전후의 목록(획 참조 — 복사가 아니다)을 쥔다. 추가·지우개·전부 지우기가 같은 꼴이다.
  // live·wiped 는 지금 그리는 획·지우개다(attach — reset 이 본다), out 은 나가는 길(relay), notice·paint 는 도구 막대에 남기는 중계 알림이다.
  function newBook() { return { pages: new Map(), strokes: 0, points: 0, nextId: 1, live: null, wiped: null, out: null, notice: "", paint: null }; }
  function pageOf(book, i) {
    let p = book.pages.get(i);
    if (!p) { p = { list: [], undo: [], redo: [] }; book.pages.set(i, p); }
    return p;
  }
  const pointsIn = list => list.reduce((n, s) => n + s.pts.length / 2, 0);
  function room(book, strokes, points) { return book.strokes + strokes <= LIMIT.strokes && book.points + points <= LIMIT.points; }
  function swapIn(book, p, list) {
    book.strokes += list.length - p.list.length;
    book.points += pointsIn(list) - pointsIn(p.list);
    p.list = list;
  }
  function commit(book, i, list) {
    const p = pageOf(book, i);
    p.undo.push({ before: p.list, after: list });
    if (p.undo.length > UNDO_MAX) p.undo.shift();
    p.redo = [];
    swapIn(book, p, list);
  }
  // step 은 되돌리기·다시 실행이다. 할 것이 없으면 "", 상한을 넘게 되면 "limit"(그대로 둔다), 했으면 "ok".
  function step(book, i, from, to, side) {
    const p = pageOf(book, i), op = p[from].pop();
    if (!op) return "";
    if (!room(book, op[side].length - p.list.length, pointsIn(op[side]) - pointsIn(p.list))) { p[from].push(op); return "limit"; }
    p[to].push(op);
    swapIn(book, p, op[side]);
    return "ok";
  }
  const undo = (book, i) => step(book, i, "undo", "redo", "before");
  const redo = (book, i) => step(book, i, "redo", "undo", "after");

  // bookFor 는 그 탭의 판서를 준다 — 같은 탭에서 발표를 다시 시작해도 남는다. key 가 없으면 이번 발표만의 판서다.
  // drop 은 닫힌 탭의 판서를 놓는다(호스트가 탭을 닫을 때 부른다). 탭 id 는 다시 쓰이지 않는다고 본다(id 를 매기는 것은 호스트다).
  const books = new Map();
  function bookFor(key) {
    if (!key) return newBook();
    if (!books.has(key)) books.set(key, newBook());
    return books.get(key);
  }
  function drop(key) {
    const b = books.get(key);
    if (b) closeOut(b);
    books.delete(key);
    if (shownKey === key) preview("", []);
  }

  // ---- 미리보기: 그 탭의 판서를 슬라이드 보기 미리보기 위에 읽기 전용 층으로 그린다 — 입력·되돌리기가 없고 그리기는 발표(attach)에서만이다.
  // pages 는 미리보기의 슬라이드 .page 들이다 — 발표 P.slides 와 같은 목록·차례(호스트가 같은 목록을 넘긴다)라 장 번호가 그대로 맞는다. 빈 목록이면
  // 지우기만 한다(쪽·이어보기·보드 꼴). 없는 번호의 판서는 그리지 않고 모델에 남긴다. 판서가 없는 탭은 층을 만들지 않는다(bookFor 를 부르지 않는다 — 책도 만들지 않는다).
  // 층에는 .ink-preview 를 함께 단다 — 발표 복제(present.js start)가 떼어 발표 화면에 두 겹이 되지 않고, 출력에서는 호스트가 숨긴다.
  // 부를 때마다 앞서 붙인 층을 모두 떼고 다시 그린다 — 이어 만들기(바뀌지 않은 장을 그대로 두는 다시 그리기)가 그대로 쓴 장에 남은 옛 층도 여기서 떨어진다. 돌려주는 값은 그린 획 수다.
  let shown = [], shownKey = "";
  function svgLayer(cls, W, H) {
    const s = document.createElementNS(SVG_NS, "svg");
    s.setAttribute("class", cls);
    s.setAttribute("viewBox", "0 0 " + W + " " + H);
    s.setAttribute("aria-hidden", "true");
    return s;
  }
  function preview(key, pages, size) {
    for (const e of shown) e.remove();
    shown = [];
    shownKey = "";
    const book = key ? books.get(key) : null;
    if (!book || !book.strokes || !pages || !pages.length) return 0;
    const [W, H] = size;
    let n = 0;
    for (const [i, p] of book.pages) {
      const pg = pages[i];
      if (!pg || !p.list.length) continue;
      const hl = svgLayer("ink-hl ink-preview" + (slideDark(pg, W, H) ? " ink-dark" : ""), W, H), pen = svgLayer("ink-pen ink-preview", W, H);
      for (const s of p.list) {
        const e = document.createElementNS(SVG_NS, "path");
        dress(e, s);
        (s.tool === "hl" ? hl : pen).appendChild(e);
        n++;
      }
      for (const l of [hl, pen]) if (l.children.length) { pg.appendChild(l); shown.push(l); }
    }
    if (shown.length) shownKey = key;
    return n;
  }

  // ---- 획 기하
  function boxOf(pts) {
    let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity;
    for (let i = 0; i < pts.length; i += 2) {
      x0 = Math.min(x0, pts[i]); x1 = Math.max(x1, pts[i]);
      y0 = Math.min(y0, pts[i + 1]); y1 = Math.max(y1, pts[i + 1]);
    }
    return [x0, y0, x1, y1];
  }
  function segDist(px, py, ax, ay, bx, by) {
    const dx = bx - ax, dy = by - ay, l = dx * dx + dy * dy;
    const t = l ? Math.max(0, Math.min(1, ((px - ax) * dx + (py - ay) * dy) / l)) : 0;
    return Math.hypot(px - ax - t * dx, py - ay - t * dy);
  }
  // hits 는 (x, y) 가 획 s 에 닿는가다 — 지우개는 획 단위다.
  function hits(s, x, y, r) {
    const w = r + widthOf(s) / 2, b = s.box, pts = s.pts;
    if (x < b[0] - w || x > b[2] + w || y < b[1] - w || y > b[3] + w) return false;
    if (pts.length === 2) return Math.hypot(x - pts[0], y - pts[1]) <= w;
    for (let i = 0; i + 3 < pts.length; i += 2) if (segDist(x, y, pts[i], pts[i + 1], pts[i + 2], pts[i + 3]) <= w) return true;
    return false;
  }

  // ---- 중계. 책 하나에 나가는 길 하나다(book.out). 연산은 JSON 배열이고 슬라이드 번호·획 id 로 모델을 가리킨다:
  //   ["b", 장, id, 도구, 색, 굵기, 필압 0·1] 획 시작 · ["a", id, [x,y,…], [필압,…]] 점 추가 · ["e", id] 획 끝 · ["d", 장, [id,…]] 삭제 · ["r", 장, [id,…]] 그 장의 목록을 이 차례로(복구)
  //   ["c", 장] 장 지우기 · ["s", 장, id, 도구, 색, 굵기, [x,y,…], [필압,…]|0, 끝남 0·1] 획 전체(reset 안에서만)
  // 묶음은 {"k": "reset"|"clear"|"", "ops": […]} 한 줄 JSON 이다. k 가 허브가 보는 전부다 — reset 은 지금 상태 전체로 로그를 바꾸고, clear 는 기록에 전부 지우기를 남긴다.
  // 호스트의 보내기 호출이 순서를 보장한다고 보지 않으므로 묶음은 앞 호출이 끝난 뒤에야 보낸다 — 그 사이의 연산은 다음 묶음에 합쳐진다. 레이저도 같다(마지막 값 하나).
  // known 은 그 reset 뒤로 보는 쪽이 받은 획이다 — 그 밖의 획을 되살리면(reset 이 뺀 되돌리기 칸) id 로 가리킬 수 없어 reset 을 다시 보낸다.
  function resetOps(book, o) {
    const ops = [], full = (i, s, done) => { o.known.add(s); ops.push(["s", i, s.id, s.tool, s.color, s.width, s.pts, s.press || 0, done]); };
    o.known.clear();
    for (const [i, p] of book.pages) for (const s of book.wiped && book.wiped.at === i ? book.wiped.list : p.list) full(i, s, 1);
    if (book.live) full(book.live.at, book.live, 0);
    return ops;
  }
  function emit(book, op) {
    const o = book.out;
    if (!o) return;
    o.q.push(op);
    wakeOut(book, o);
  }
  function emitPoint(book, s, x, y, press) {
    const o = book.out;
    if (!o) return;
    let a = o.q[o.q.length - 1];
    if (!a || a[0] !== "a" || a[1] !== s.id) o.q.push(a = s.press ? ["a", s.id, [], []] : ["a", s.id, []]);
    a[2].push(x, y);
    if (s.press) a[3].push(press);
    wakeOut(book, o);
  }
  // emitList 는 장 i 의 목록이 was 에서 now 로 바뀐 것을 낸다(되돌리기·다시 실행·지우개 취소).
  function emitList(book, i, was, now) {
    const o = book.out;
    if (!o) return;
    if (was.length === now.length && was.every((s, k) => s === now[k])) return;
    const keep = new Set(now), left = was.filter(s => keep.has(s));
    if (left.length === now.length && left.every((s, k) => s === now[k])) emit(book, ["d", i, was.filter(s => !keep.has(s)).map(s => s.id)]); // 빠지기만 했다
    else if (now.every(s => o.known.has(s))) emit(book, ["r", i, now.map(s => s.id)]);
    else { o.stale = true; wakeOut(book, o); }
  }
  // emitLaser 는 레이저의 마지막 값을 고친다. at 이 null 이면 레이저 없음이다.
  function emitLaser(book, at) {
    const o = book.out;
    if (!o || (!at && o.spotOff)) return;
    o.spot = at || [-1, 0, 0, false];
    o.spotOff = !at;
    wakeOut(book, o);
  }
  function wakeOut(book, o) {
    if (o.timer || o.closed) return;
    if ((o.q.length || o.stale) && !o.busy || (o.spot && !o.spotBusy)) o.timer = setTimeout(() => flushOut(book, o), RELAY_MS);
  }
  const called = f => new Promise(ok => ok(f()));
  function flushOut(book, o) {
    o.timer = 0;
    if (o.closed) return;
    if (o.spot && !o.spotBusy) {
      const v = o.spot, done = () => { o.spotBusy = false; wakeOut(book, o); };
      o.spot = null;
      o.spotBusy = true;
      called(() => o.send.laser(v[0], v[1], v[2], v[3])).then(done, done);
    }
    if (o.busy || (!o.q.length && !o.stale)) return;
    const reset = o.stale, ops = reset ? resetOps(book, o) : o.q;
    const kind = reset ? "reset" : ops.some(op => op[0] === "c") ? "clear" : "";
    o.q = [];
    o.stale = false;
    o.busy = true;
    called(() => o.send.ink(JSON.stringify({ k: kind, ops }))).then(st => {
      o.busy = false;
      if (o.closed) return;
      if (st === "stopped") return halt(book, o);
      if (st === "full") o.stale = true;
      wakeOut(book, o);
    }, () => {
      o.busy = false;
      if (o.closed) return;
      if (reset) return halt(book, o); // reset 까지 거부되면 다시 보내도 같다
      o.stale = true; // 이 묶음이 빠졌다 — 지금 상태로 다시 잡는다
      wakeOut(book, o);
    });
  }
  function halt(book, o) {
    closeOut(book);
    book.notice = RELAY_NOTE;
    if (book.paint) book.paint();
    if (o.send.stopped) o.send.stopped();
  }
  function closeOut(book) {
    const o = book.out;
    if (!o) return;
    o.closed = true;
    if (o.timer) clearTimeout(o.timer);
    o.timer = 0;
    book.out = null;
  }
  // relay 는 key 탭의 판서를 send 로 잇는다 — send.ink(묶음 JSON) 은 ""·"full"·"stopped"·"off" 의 Promise, send.laser(장, x, y, 선), send.stopped()(전달이 멈췄다). 잇자마자 지금 상태를
  // reset 으로 보낸다(판서한 뒤 공유 시작). 돌려준 close 로 끊는다.
  function relay(key, send) {
    const book = bookFor(key);
    closeOut(book);
    const o = { send, q: [], known: new Set(), stale: true, busy: false, timer: 0, closed: false, spot: null, spotBusy: false, spotOff: true };
    book.out = o;
    book.notice = "";
    if (book.paint) book.paint();
    wakeOut(book, o);
    return { close: () => { if (book.out === o) closeOut(book); } };
  }

  // ---- 발표에 붙이기. o = { root, deck, size: [w, h](설계 px), book, slides()(복제된 .page 들), idx(), scale(), paused()(라이트박스·한눈에 보기·검은 화면),
  //   click(target, button)(레이저의 클릭 — 넘김·크게 보기), eatClick()(뒤따르는 click 을 먹는다), wake()(도구가 바뀌면 커서 숨김을 다시 판단),
  //   penOnly(참이면 pointerType "pen" 만 판서 — itda-doc#100) }
  function attach(o) {
    const { root, deck, book } = o, [W, H] = o.size;
    const mac = /Mac/.test(window.navigator ? window.navigator.platform : "");
    const pref = { pen: { color: 0, width: 1 }, hl: { color: 2, width: 1 } }; // 도구마다 색·굵기를 따로 기억한다(발표 하나 동안)
    const layers = new Map(), paths = new Map();
    let tool = "", stroke = null, smoothed = null, wiped = null, press = null, slideAt = -1, note = "", pointer = null, liveRaf = 0;
    let barEl = null, noteEl = null, toolBtns = {}, swatches = [], widthBtns = [], undoBtn = null, redoBtn = null, clearBtn = null;
    let raf = 0, cursor = null, trail = [];

    const live = el("canvas", "ink-live");
    const ctx = live.getContext ? live.getContext("2d") : null;
    const laser = el("canvas", "ink-laser");
    const lctx = laser.getContext ? laser.getContext("2d") : null;
    root.appendChild(laser);
    buildBar();

    function el(tag, cls, text) {
      const e = document.createElement(tag);
      if (cls) e.className = cls;
      if (text != null) e.textContent = text;
      return e;
    }
    const pageNow = () => o.slides()[o.idx()];
    const dpr = () => window.devicePixelRatio || 1;

    // ---- 끝난 획: SVG 층
    const layerSvg = cls => svgLayer(cls, W, H);
    function layerOf(i) {
      let l = layers.get(i);
      if (l) return l;
      const pg = o.slides()[i];
      const dark = slideDark(pg, W, H);
      l = { hl: layerSvg("ink-hl" + (dark ? " ink-dark" : "")), pen: layerSvg("ink-pen"), dark };
      pg.appendChild(l.hl);
      pg.appendChild(l.pen);
      layers.set(i, l);
      return l;
    }
    function pathFor(s) {
      let p = paths.get(s);
      if (p) return p;
      p = document.createElementNS(SVG_NS, "path");
      dress(p, s);
      paths.set(s, p);
      return p;
    }
    // sync 는 슬라이드 i 의 층을 모델에 맞춘다(되돌리기·전부 지우기·시작). 경로는 획마다 하나를 다시 쓴다 — 획을 끝낼 때는 부르지 않는다(appendChild 하나).
    function sync(i) {
      if (!o.slides()[i]) return; // 문서가 줄어 없는 장의 판서는 모델에만 남는다
      const l = layerOf(i), list = pageOf(book, i).list;
      l.hl.textContent = "";
      l.pen.textContent = "";
      for (const s of list) (s.tool === "hl" ? l.hl : l.pen).appendChild(pathFor(s));
    }

    // ---- 진행 중인 획: canvas
    function fitLive() {
      if (!ctx) return;
      const k = (o.scale() || 1) * dpr(), bw = Math.round(W * k), bh = Math.round(H * k);
      if (live.width !== bw || live.height !== bh) { live.width = bw; live.height = bh; }
      ctx.setTransform(k, 0, 0, k, 0, 0);
      ctx.lineCap = "round";
      ctx.lineJoin = "round";
    }
    function fitLaser() {
      if (!lctx) return;
      const k = dpr(), bw = Math.round(root.clientWidth * k), bh = Math.round(root.clientHeight * k);
      if (laser.width !== bw || laser.height !== bh) { laser.width = bw; laser.height = bh; }
      lctx.setTransform(k, 0, 0, k, 0, 0);
    }
    function clearLive() {
      if (liveRaf && window.cancelAnimationFrame) window.cancelAnimationFrame(liveRaf);
      liveRaf = 0;
      if (ctx) ctx.clearRect(0, 0, W, H);
    }
    // paintLive 는 진행 중인 획 전체를 끝난 획과 같은 경로로 다시 그린다 — 조각을 덧그리면 끝부분이 끝난 획과 달라진다. 입력이 아니라 프레임마다 한 번이다
    // (스파이크: 2천 획 위 긴 획의 전체 다시 그리기가 p50 17ms). 형광펜은 불투명하게 그리고 투명도·혼합은 canvas 요소(present.css)에 있다.
    function paintLive() {
      clearLive();
      const s = stroke;
      if (!ctx || !s) return;
      const p = new window.Path2D(pathOf(s));
      ctx.strokeStyle = ctx.fillStyle = COLORS[s.color];
      if (filled(s)) ctx.fill(p);
      else { ctx.lineWidth = widthOf(s); ctx.stroke(p); }
    }
    function queueLive() {
      if (liveRaf) return;
      if (window.requestAnimationFrame) liveRaf = window.requestAnimationFrame(paintLive);
      else paintLive();
    }
    function placeLive(hl) {
      const pg = pageNow();
      if (!pg) return;
      const l = layerOf(o.idx());
      live.className = "ink-live" + (hl ? " ink-live-hl" : "") + (hl && l.dark ? " ink-dark" : "");
      if (live.parentNode !== pg) pg.appendChild(live);
      fitLive();
    }

    // ---- 그리기
    function toDesign(e) {
      const r = deck.getBoundingClientRect(), s = o.scale() || 1;
      return { x: (e.clientX - r.left) / s, y: (e.clientY - r.top) / s };
    }
    const inSlide = p => p.x >= 0 && p.y >= 0 && p.x <= W && p.y <= H;
    function addPoint(p, pressure) {
      const x = r1(Math.max(0, Math.min(W, p.x))), y = r1(Math.max(0, Math.min(H, p.y))), s = stroke, n = s.pts.length;
      if (n && Math.hypot(x - s.pts[n - 2], y - s.pts[n - 1]) < MIN_STEP) return;
      if (!room(book, 1, n / 2 + 1)) { setNote(LIMIT_NOTE); finish(); return; }
      s.pts.push(x, y);
      if (s.press) s.press.push(Math.round(pressure * 100) / 100);
      emitPoint(book, s, x, y, s.press ? s.press[s.press.length - 1] : 0);
      queueLive();
    }
    function begin(e, p) {
      if (!room(book, 1, 1)) { setNote(LIMIT_NOTE); return; }
      placeLive(tool === "hl");
      const pen = e.pointerType === "pen";
      stroke = book.live = { id: book.nextId++, tool, color: pref[tool].color, width: pref[tool].width, pts: [], press: pen ? [] : null, box: null, at: o.idx() };
      if (book.out) book.out.known.add(stroke);
      emit(book, ["b", stroke.at, stroke.id, tool, stroke.color, stroke.width, pen ? 1 : 0]);
      smoothed = { x: p.x, y: p.y };
      addPoint(p, e.pressure || 0.5);
    }
    function moveTo(e) {
      for (const ev of (e.getCoalescedEvents && e.getCoalescedEvents().length ? e.getCoalescedEvents() : [e])) {
        if (!stroke) return;
        const p = toDesign(ev);
        smoothed.x += (p.x - smoothed.x) * SMOOTH;
        smoothed.y += (p.y - smoothed.y) * SMOOTH;
        addPoint(smoothed, ev.pressure || 0.5);
      }
    }
    // finish 는 진행 중인 획을 모델과 SVG 로 옮긴다 — 같은 태스크에서 canvas 를 지우므로 두 층이 함께 그려지는 틈이나 빈 틈이 없다.
    function finish() {
      const s = stroke;
      if (!s) return;
      stroke = book.live = null;
      const i = s.at;
      delete s.at;
      emit(book, ["e", s.id]);
      s.box = boxOf(s.pts);
      commit(book, i, pageOf(book, i).list.concat([s]));
      const l = layerOf(i);
      (s.tool === "hl" ? l.hl : l.pen).appendChild(pathFor(s));
      clearLive();
      paintBar();
    }
    function cancel() {
      if (stroke) {
        emit(book, ["d", stroke.at, [stroke.id]]);
        stroke = book.live = null;
        clearLive();
        return true;
      }
      if (wiped) {
        const w = wiped;
        wiped = book.wiped = null;
        sync(o.idx());
        emitList(book, w.at, w.list, pageOf(book, w.at).list);
        return true;
      }
      return false;
    }
    function eraseAt(p) {
      const hit = wiped.list.filter(s => hits(s, p.x, p.y, ERASE_R));
      if (!hit.length) return;
      wiped.list = wiped.list.filter(s => !hit.includes(s));
      for (const s of hit) { const e = paths.get(s); if (e) e.remove(); }
      emit(book, ["d", wiped.at, hit.map(s => s.id)]);
    }
    function endErase() {
      const w = wiped;
      wiped = book.wiped = null;
      if (w && w.list.length !== pageOf(book, w.at).list.length) commit(book, w.at, w.list);
      paintBar();
    }

    // ---- 레이저
    function wakeLaser() { if (!raf && window.requestAnimationFrame) raf = window.requestAnimationFrame(drawLaser); }
    function drawLaser() {
      raf = 0;
      if (!lctx) return;
      fitLaser();
      lctx.clearRect(0, 0, root.clientWidth, root.clientHeight);
      if (o.paused() || tool !== "laser") { trail = []; return; }
      const now = performance.now(), r = deck.getBoundingClientRect(), base = root.getBoundingClientRect(), s = o.scale() || 1;
      trail = trail.filter(t => now - t.t < t.life);
      const sx = x => r.left - base.left + x * s, sy = y => r.top - base.top + y * s;
      lctx.lineCap = "round";
      for (let i = 1; i < trail.length; i++) {
        const a = trail[i - 1], b = trail[i];
        if (b.brk) continue;
        lctx.strokeStyle = "rgba(255, 40, 40, " + (0.9 * (1 - (now - b.t) / b.life)).toFixed(3) + ")";
        lctx.lineWidth = b.life === LASER_LINE ? 4 : 3;
        lctx.beginPath();
        lctx.moveTo(sx(a.x), sy(a.y));
        lctx.lineTo(sx(b.x), sy(b.y));
        lctx.stroke();
      }
      if (cursor) {
        const cx = sx(cursor.x), cy = sy(cursor.y);
        lctx.fillStyle = "rgba(255, 30, 30, 0.28)";
        lctx.beginPath(); lctx.arc(cx, cy, 13, 0, 7); lctx.fill();
        lctx.fillStyle = "rgba(255, 30, 30, 0.95)";
        lctx.beginPath(); lctx.arc(cx, cy, 5.5, 0, 7); lctx.fill();
      }
      if (trail.length) wakeLaser();
    }
    function laserMove(e, p) {
      const line = !!(press && press.line);
      trail.push({ x: p.x, y: p.y, t: performance.now(), life: line ? LASER_LINE : LASER_TAIL, brk: !cursor });
      cursor = e.target && e.target.closest && e.target.closest(BLOCKED) ? null : p;
      emitLaser(book, cursor && inSlide(p) ? [o.idx(), r1(p.x), r1(p.y), line] : null);
      wakeLaser();
    }

    // ---- 입력(Pointer Events — 펜 필압·getCoalescedEvents 가 여기서 온다. 뒤따르는 mousedown 은 present.js onMouse 가 takes 로 받는다)
    function canStart(e) { return !o.paused() && !(e.target && e.target.closest && e.target.closest(BLOCKED)); }
    // penOnly(itda-doc#100)면 펜이 아닌 입력(손가락·마우스)은 판서가 아니다 — 획·지우개·레이저를 만들지도 움직이지도 않는다.
    // 그 누르기가 발표 입력이 되는가는 takes 가 정한다(도구가 켜진 동안은 먹는다).
    const notPen = e => !!o.penOnly && e.pointerType !== "pen";
    function onDown(e) {
      if (notPen(e)) return; // 진행 중인 펜 획은 그대로 — 손바닥·손가락이 끊지 않는다
      if (pointer === e.pointerId) settle(); // 떼기를 놓쳤다
      if (!tool || pointer !== null || !canStart(e)) return; // 진행 중인 입력이 있으면 둘째 손가락은 받지 않는다
      if (tool === "laser") {
        if (e.button !== 0 && e.button !== 2) return;
        press = { x: e.clientX, y: e.clientY, target: e.target, button: e.button, line: false };
      } else {
        if (e.button !== 0) return;
        const p = toDesign(e);
        if (tool === "erase") { wiped = book.wiped = { list: pageOf(book, o.idx()).list.slice(), at: o.idx() }; eraseAt(p); }
        else if (inSlide(p)) begin(e, p);
        else return;
      }
      if (!stroke && !wiped && !press) return; // 상한에 닿아 시작하지 못했다
      pointer = e.pointerId;
      try { root.setPointerCapture(e.pointerId); } catch (err) { /* 합성 입력 — 끌기는 root 로 그대로 온다 */ }
    }
    const other = e => pointer !== null && e.pointerId !== pointer;
    function onMove(e) {
      if (other(e) || notPen(e)) return;
      if (pointer !== null && (e.buttons === 0 || o.paused())) settle(); // 창 밖에서 놓아 pointerup 을 놓쳤거나 쉬는 화면으로 들어갔다 — 클릭으로 치지 않는다
      if (stroke) moveTo(e);
      else if (wiped) eraseAt(toDesign(e));
      if (tool === "laser" && !o.paused()) {
        if (press && !press.line && Math.hypot(e.clientX - press.x, e.clientY - press.y) >= LASER_DRAG) press.line = true;
        laserMove(e, toDesign(e));
      }
    }
    function onUp(e) {
      if (other(e) || notPen(e)) return;
      const p = press;
      if (!p || e.type !== "pointerup" || o.paused()) { settle(); return; }
      press = null;
      settle();
      if (p.line) o.eatClick(); else o.click(p.target, p.button);
    }
    // settle 은 진행 중인 입력을 끝내는 한 곳이다 — 슬라이드가 바뀌거나 쉬는 화면으로 들어가거나 도구가 바뀌거나 포인터를 잃으면 부른다. 획과 지우개는 그때까지 한
    // 만큼 확정하고(그린 것을 버리지 않는다), 레이저 누르기는 클릭이 되지 않게 거두고 뒤따르는 click 도 먹는다. 포인터를 놓으므로 그 뒤 움직임·떼기는 아무것도 하지 않는다.
    function settle() {
      if (stroke) finish();
      if (wiped) endErase();
      if (press) { press = null; o.eatClick(); }
      const id = pointer;
      pointer = null;
      if (id !== null) try { root.releasePointerCapture(id); } catch (err) { /* 이미 놓였다 */ }
    }
    // pause 는 한눈에 보기·검은 화면으로 들어갈 때 부른다 — 레이저 자취는 drawLaser 가 쉬는 동안 지운다. 라이트박스는 누르기로만 열려 진행 중인 입력이 없다.
    function pause() {
      settle();
      emitLaser(book, null);
      wakeLaser();
    }
    const on = [["pointerdown", onDown], ["pointermove", onMove], ["pointerup", onUp], ["pointercancel", onUp], ["lostpointercapture", onUp]];
    for (const [t, f] of on) root.addEventListener(t, f);
    window.addEventListener("blur", settle);

    // ---- 도구·색·되돌리기
    const canColor = i => !(tool === "hl" && HL_SKIP.includes(i));
    function setTool(t) {
      settle();
      tool = t === tool ? "" : t;
      cursor = null;
      emitLaser(book, null);
      if (tool !== "laser") { trail = []; }
      setNote("");
      paintBar();
      root.classList.toggle("pr-ink-on", !!tool);
      for (const n of ["laser", "pen", "hl", "erase"]) root.classList.toggle("pr-ink-" + n, tool === n);
      if (tool === "laser") fitLaser();
      wakeLaser();
      o.wake();
    }
    function setColor(i) { if ((tool === "pen" || tool === "hl") && canColor(i)) { pref[tool].color = i; paintBar(); } }
    function setWidth(i) { if (tool === "pen" || tool === "hl") { pref[tool].width = i; paintBar(); } }
    function stepColor(dir) {
      if (tool !== "pen" && tool !== "hl") return;
      let i = pref[tool].color;
      do { i = (i + dir + COLORS.length) % COLORS.length; } while (!canColor(i));
      setColor(i);
    }
    function history(fn) {
      settle();
      const i = o.idx(), was = pageOf(book, i).list;
      const r = fn(book, i);
      if (r === "limit") setNote(LIMIT_NOTE);
      else if (r === "ok") { setNote(""); sync(i); emitList(book, i, was, pageOf(book, i).list); }
      paintBar();
    }
    function clearPage() {
      settle();
      const i = o.idx();
      if (!pageOf(book, i).list.length) return;
      commit(book, i, []);
      emit(book, ["c", i]);
      setNote("");
      sync(i);
      paintBar();
    }
    function setNote(t) {
      note = t = t || book.notice; // 중계 알림은 다른 알림이 없는 동안 남는다
      if (noteEl) noteEl.textContent = t;
      if (barEl) barEl.classList.toggle("has-note", !!t);
    }

    // key 는 판서 키다. 받았으면 참. 새 키는 수식키가 없을 때만 받고(⌘L·⌘C·⌘P·⌘W 가 도구를 바꾸지 않게), 한글 입력 상태에서도 되게 e.code 로 본다.
    // 도구가 꺼져 있으면 도구를 켜는 키(L·P·H·E)만 받는다 — 나머지는 지금처럼 삼켜진다.
    function key(e) {
      if (e.key === "Escape") {
        if (cancel()) return true;
        if (tool) { setTool(tool); return true; }
        return false;
      }
      const cmd = mac ? e.metaKey && !e.ctrlKey : e.ctrlKey && !e.metaKey;
      if (cmd && !e.altKey && e.code === "KeyZ") {
        if (!tool) return false;
        history(e.shiftKey ? redo : undo);
        return true;
      }
      if (e.metaKey || e.ctrlKey || e.altKey || e.shiftKey) return false;
      const t = { KeyL: "laser", KeyP: "pen", KeyH: "hl", KeyE: "erase" }[e.code];
      if (t) { setTool(t); return true; }
      if (!tool) return false;
      switch (e.code) {
        case "KeyX": clearPage(); return true;
        case "KeyC": case "BracketRight": stepColor(1); return true;
        case "BracketLeft": stepColor(-1); return true;
      }
      return false;
    }

    // ---- 도구 막대
    function buildBar() {
      barEl = root.appendChild(el("div", "pr-ink-bar"));
      const btn = (cls, text, title, fn) => {
        const b = barEl.appendChild(el("button", cls, text));
        b.type = "button";
        b.title = title;
        b.addEventListener("click", ev => { ev.stopPropagation(); fn(); if (b.blur) b.blur(); });
        return b;
      };
      for (const [t, label, k] of [["laser", "레이저", "L"], ["pen", "펜", "P"], ["hl", "형광펜", "H"], ["erase", "지우개", "E"]]) toolBtns[t] = btn("pr-ink-tool", label, label + "(" + k + ")", () => setTool(t));
      barEl.appendChild(el("span", "pr-ink-gap"));
      COLORS.forEach((c, i) => {
        const b = btn("pr-ink-sw", null, "색 바꾸기(C·[·])", () => setColor(i));
        b.style.backgroundColor = c;
        swatches.push(b);
      });
      barEl.appendChild(el("span", "pr-ink-gap"));
      for (let i = 0; i < 3; i++) {
        const b = btn("pr-ink-w w" + i, null, "굵기 " + (i + 1), () => setWidth(i));
        b.appendChild(el("i"));
        widthBtns.push(b);
      }
      barEl.appendChild(el("span", "pr-ink-gap"));
      undoBtn = btn("pr-ink-act", "되돌리기", mac ? "되돌리기(⌘Z)" : "되돌리기(Ctrl+Z)", () => history(undo));
      redoBtn = btn("pr-ink-act", "다시", mac ? "다시 실행(⇧⌘Z)" : "다시 실행(Ctrl+Shift+Z)", () => history(redo));
      clearBtn = btn("pr-ink-act", "전부 지우기", "이 장 전부 지우기(X) — 되돌리기로 복구", clearPage);
      btn("pr-ink-act", "끄기", "도구 끄기(Esc)", () => setTool(tool));
      noteEl = barEl.appendChild(el("span", "pr-ink-note"));
    }
    function paintBar() {
      for (const t in toolBtns) toolBtns[t].classList.toggle("on", t === tool);
      const inks = tool === "pen" || tool === "hl";
      swatches.forEach((b, i) => { b.disabled = !inks || !canColor(i); b.classList.toggle("on", inks && pref[tool].color === i); });
      widthBtns.forEach((b, i) => { b.disabled = !inks; b.classList.toggle("on", inks && pref[tool].width === i); });
      const p = book.pages.get(o.idx());
      undoBtn.disabled = !p || !p.undo.length;
      redoBtn.disabled = !p || !p.redo.length;
      clearBtn.disabled = !p || !p.list.length;
    }

    // ---- present.js 가 부르는 것
    // takes 는 onMouse 가 이 누르기를 넘김·크게 보기로 쓰면 안 되는가다 — "draw"(펜·형광펜·지우개의 왼쪽 단추) · "laser"(떼는 때로 미룬다) · ""(지금처럼).
    // penOnly 면 도구가 켜진 동안 누르기는 출처(손가락·마우스·펜)를 가리지 않고 "draw"(먹는다 — 장이 넘어가지 않고 뒤따르는 click 도 먹는다)다.
    // 누르기(mousedown — MouseEvent)에는 pointerType 이 없어 출처를 알 수 없다 — 시각·자리로 짐작하지 않는다. 펜의 호환 mousedown 도 원래 판서 몫이라
    // 넘기지 않는다(레이저의 펜 클릭은 onUp 의 o.click 이 넘긴다). 도구가 꺼졌거나 쉬는 화면(검은 화면)이면 지금처럼 ""(누르기 = 다음)다.
    function takes(e) {
      if (!tool || o.paused()) return "";
      if (o.penOnly) return "draw";
      if (tool === "laser") return e.button === 0 || e.button === 2 ? "laser" : "";
      return e.button === 0 ? "draw" : "";
    }
    // slide 는 슬라이드가 바뀔 때 부른다(go). 그리던 획은 그 슬라이드에 남기고, 레이저 자취는 지운다.
    function slide(i) {
      if (i === slideAt) return;
      slideAt = i;
      settle();
      trail = [];
      emitLaser(book, null);
      setNote("");
      paintBar();
    }
    function resize() {
      if (live.parentNode) { fitLive(); paintLive(); }
      fitLaser();
      wakeLaser();
    }
    function detach() {
      cancel();
      emitLaser(book, null);
      book.paint = null;
      press = null;
      pointer = null;
      for (const [t, f] of on) root.removeEventListener(t, f);
      window.removeEventListener("blur", settle);
      if (raf && window.cancelAnimationFrame) window.cancelAnimationFrame(raf);
      raf = 0;
    }

    for (const i of book.pages.keys()) if (pageOf(book, i).list.length) sync(i);
    book.paint = () => setNote(note === RELAY_NOTE ? "" : note);
    setNote("");
    paintBar();
    return { tool: () => tool, setTool, setColor, setWidth, undo: () => history(undo), redo: () => history(redo), clearPage, key, takes, slide, pause, resize, detach,
      drawing: () => !!(stroke || wiped), held: () => pointer !== null, note: () => note, book };
  }

  window.ItdaInk = { LIMIT, COLORS, WIDTHS, HL_SKIP, newBook, pageOf, room, commit, undo, redo, bookFor, drop, preview, relay, pathOf, filled, hits, parseColor, luminance, slideDark, attach };
})();
