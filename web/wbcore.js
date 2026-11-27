// wbcore.js — 화이트보드(whiteboard.js)의 순수 함수. 전역은 window.ItdaWbCore 다(node 에서는 module.exports).
// DOM·canvas 를 모른다: 보기 좌표 변환·필압 외곽선·맞히기·공간 색인·타일 계산과 파일 형식(직렬화·검증)만 있다.
// 엔진 부분은 시제품(화이트보드 기획 ⓪ 스파이크 wbcore.js — curve 를 더한 판)에서 측정용(통계·부하 획 생성)만 뺀 것이다 — 동작은 바꾸지 않았다
// (더한 것: boundView·inRange, Grid.range·tilesFor 의 칸 번호 끌어 맞춤 — 좌표 범위 안에서는 결과가 같다).
//
// 파일 형식 v1 — JSON 한 덩어리(정본은 이 주석과 parse·serialize):
//   { "format": "itda-whiteboard", "v": 1, "name": 문자열, "bg": "dark"|"light"|"grid", "view": { "x", "y", "z" },
//     "strokes": [ { "t": "pen"|"hl", "c": 색 번호(0~COLORS-1), "w": 굵기(판 단위, W_MIN~W_MAX), "p": [x, y, 필압, …] } ] }
//   · 수치 범위(상수): 좌표 |x|·|y| ≤ COORD_MAX(1e9), 굵기 W_MIN(0.01) ≤ w ≤ W_MAX(10000), 보기 z 는 ZMIN~ZMAX(0.1~8), 보기 x·y 는 좌표 범위.
//     이 범위 안에서는 색인(Grid)·타일(tilesFor)의 칸 번호가 안전한 정수라 반복이 끝난다(범위 밖 값이 와도 두 함수는 끌어 맞춰 끝낸다).
//   · 쓸 때 좌표(x·y, view.x·y)는 소수 둘째 자리, 필압과 굵기는 셋째 자리로 반올림한다(serialize). 굵기는 W_MIN 아래로 내려가지 않고,
//     굵기·보기는 범위 안으로 끌어 맞춘다. 획의 점 좌표는 끌어 맞추지 않는다 — 유한하지 않거나 범위 밖이면 RangeError 를 던진다(획 모양을
//     조용히 바꾸지 않게. 표면은 범위 밖 점을 받지 않으므로 — whiteboard.js addPoint — 던지는 것은 내부 불변식이 깨졌을 때다).
//     serialize 가 돌려준 결과는 상한(LIMIT) 안이면 늘 parse 를 통과한다. 필압은 0~1 이다.
//   · 읽을 때 검증한다(parse): format·v(1 이상 정수), 배열·숫자 유한성, 색 번호 범위, 점 배열 길이(3 의 배수, 3 이상), 좌표·굵기 범위,
//     획 수·점 수 상한(LIMIT — 넘으면 읽기를 거부한다). 틀리면 { ok: false, error } 이고 판을 열지 않는다.
//   · 모르는 최상위 키는 버리지 않고 extra 에 담아 다시 쓴다(뒤 판이 더한 키를 옛 판이 고쳐 써도 잃지 않게). 획 안의 모르는 키는 버린다.
//   · v 가 VERSION 보다 크면 readOnly 로 연다(읽을 수 있으면 보여 주되 고쳐 쓰지 않는다 — 쓰는 쪽은 whiteboard.js).
//   · bg 가 모르는 값이면 "dark" 로 읽는다. view 가 없으면 {0, 0, 1}, z 는 ZMIN~ZMAX 로, x·y 는 좌표 범위로 끌어 맞춘다(거부하지 않는다).
(function (root, factory) {
  const m = factory();
  if (typeof module === "object" && module.exports) module.exports = m;
  else root.ItdaWbCore = m;
})(typeof window !== "undefined" ? window : this, function () {
  "use strict";

  const ZMIN = 0.1, ZMAX = 8;
  const COORD_MAX = 1e9, W_MIN = 0.01, W_MAX = 10000; // 좌표·굵기 범위(머리 주석 「수치 범위」)
  const INDEX_MAX = 4 * COORD_MAX; // 색인·타일이 칸 번호를 셀 때 끌어 맞추는 끝(좌표 범위에 굵기·화면 여유를 더해도 넉넉하다)
  const clamp = (v, a, b) => (v < a ? a : v > b ? b : v);

  // ---- 보기(view): { x, y, z } — 화면 점(CSS px, 판 왼쪽 위 기준) s = (w - (x, y)) * z. x·y 는 화면 왼쪽 위의 판 좌표다(엔진은 boundView 로 묶는다).
  const toWorld = (v, sx, sy) => ({ x: v.x + sx / v.z, y: v.y + sy / v.z });
  const toScreen = (v, wx, wy) => ({ x: (wx - v.x) * v.z, y: (wy - v.y) * v.z });
  // anchor 는 판의 점 w 가 화면 (sx, sy) 에 오도록 배율 z 의 보기를 만든다(오므리기·커서 기준 확대가 모두 이것이다).
  function anchor(w, sx, sy, z) {
    z = clamp(z, ZMIN, ZMAX);
    return { x: w.x - sx / z, y: w.y - sy / z, z };
  }
  const zoomAt = (v, sx, sy, z) => anchor(toWorld(v, sx, sy), sx, sy, z);
  const panBy = (v, dsx, dsy) => ({ x: v.x - dsx / v.z, y: v.y - dsy / v.z, z: v.z });
  // boundView 는 cssW × cssH 화면 전체가 좌표 범위(±COORD_MAX) 안에 오도록 보기를 묶는다(엔진의 보기 바꾸기·크기 바꾸기·판 읽기가 모두 거친다).
  // 왼쪽 위 원점 x 는 [-COORD_MAX, COORD_MAX - cssW/z], y 는 [-COORD_MAX, COORD_MAX - cssH/z] 안이다 — 화면 안의 점은 늘 좌표 범위 안이다.
  // 화면이 좌표 세계보다 넓으면(배율 하한 0.1 에서 2억 px 넘는 화면 — 생기지 않는 크기) 그 축은 가운데(0)에 둔다.
  function boundView(v, cssW, cssH) {
    const span = (o, len) => (len > 2 * COORD_MAX ? -len / 2 : clamp(o, -COORD_MAX, COORD_MAX - len));
    return { x: span(v.x, (cssW || 0) / v.z), y: span(v.y, (cssH || 0) / v.z), z: v.z };
  }
  // inRange 는 판의 점 (x, y) 가 유한하고 좌표 범위 안인가다(표면이 받는 모든 점 — 쓴 점·coalesced·뗀 자리·예측 꼬리 — 이 거친다).
  const inRange = (x, y) => Number.isFinite(x) && Number.isFinite(y) && Math.abs(x) <= COORD_MAX && Math.abs(y) <= COORD_MAX;
  // snap 은 그리기용 보기다 — 원점이 장치 픽셀에 맞아 타일을 옮겨 그려도 흐려지지 않는다. 입력은 맞추기 전 보기를 쓴다(작은 이동이 반올림에 먹히지 않게).
  function snap(v, dpr) {
    const k = v.z * dpr;
    return { x: Math.round(v.x * k) / k, y: Math.round(v.y * k) / k, z: v.z };
  }

  // ---- 획 굵기: 펜은 반지름이 필압(0~1)을 따른다. p 0.59 쯤이 기본 굵기다.
  const penRadius = (w, p) => (w * (0.35 + 1.1 * p)) / 2;
  // speedPressure 는 필압이 없는 입력(마우스·손가락)의 가짜 필압이다 — 빠를수록 가늘다. 점이 아니라 굵기만 이전 값과 섞는다.
  function speedPressure(prev, pxPerMs) {
    const target = clamp(0.8 - pxPerMs * 0.22, 0.18, 0.8);
    return prev == null ? target : prev * 0.65 + target * 0.35;
  }

  // ---- 외곽선: pts = [x, y, p, x, y, p, …](판 좌표), w = 기준 굵기(판 단위).
  // 돌려주는 poly 는 평평한 [x, y, …] 다각형(왼쪽 가장자리 → 끝 둥근 마개 → 오른쪽 가장자리 거꾸로 → 시작 둥근 마개)이다.
  // 꼭짓점 법선은 들어오는·나가는 방향의 가운데이고 마이터 길이(최대 2배)로 굵기를 지킨다. 120° 넘게 꺾인 곳은 joins 에
  // 원(x, y, r)을 더한다 — 다각형이 접히는 곳을 원으로 메운다(같은 색으로 따로 채운다).
  function outline(pts, w, seg) {
    seg = seg || 12;
    const P = [];
    for (let i = 0; i + 2 < pts.length; i += 3) {
      const n = P.length;
      if (n && Math.abs(pts[i] - P[n - 3]) < 1e-9 && Math.abs(pts[i + 1] - P[n - 2]) < 1e-9) continue;
      P.push(pts[i], pts[i + 1], pts[i + 2]);
    }
    const n = P.length / 3, poly = [], joins = [];
    if (!n) return { poly, joins };
    const r = i => penRadius(w, P[3 * i + 2]);
    if (n === 1) {
      const R = r(0);
      for (let k = 0; k < 2 * seg; k++) {
        const a = (k / (2 * seg)) * Math.PI * 2;
        poly.push(P[0] + Math.cos(a) * R, P[1] + Math.sin(a) * R);
      }
      return { poly, joins };
    }
    const L = [], Rt = [];
    let t0 = null, t1 = null;
    for (let i = 0; i < n; i++) {
      const x = P[3 * i], y = P[3 * i + 1];
      let inx = 0, iny = 0, outx = 0, outy = 0, hasIn = false, hasOut = false;
      if (i > 0) { const dx = x - P[3 * i - 3], dy = y - P[3 * i - 2], l = Math.hypot(dx, dy); inx = dx / l; iny = dy / l; hasIn = true; }
      if (i < n - 1) { const dx = P[3 * i + 3] - x, dy = P[3 * i + 4] - y, l = Math.hypot(dx, dy); outx = dx / l; outy = dy / l; hasOut = true; }
      let tx, ty, scale = 1;
      if (hasIn && hasOut) {
        tx = inx + outx; ty = iny + outy;
        const l = Math.hypot(tx, ty);
        const cosHalf = Math.sqrt(Math.max(0, (1 + inx * outx + iny * outy) / 2));
        if (l < 1e-6) { tx = outx; ty = outy; } else { tx /= l; ty /= l; }
        if (cosHalf >= 0.5) scale = 1 / cosHalf;
        else joins.push(x, y, r(i));
      } else if (hasOut) { tx = outx; ty = outy; } else { tx = inx; ty = iny; }
      if (i === 0) t0 = [tx, ty];
      if (i === n - 1) t1 = [tx, ty];
      const R = r(i) * scale, nx = -ty * R, ny = tx * R;
      L.push(x + nx, y + ny);
      Rt.push(x - nx, y - ny);
    }
    for (let i = 0; i < L.length; i++) poly.push(L[i]);
    // 끝 마개: +법선 → 진행 방향 → -법선(각이 줄어드는 쪽)
    const ex = P[3 * n - 3], ey = P[3 * n - 2], er = r(n - 1), ea = Math.atan2(t1[1], t1[0]);
    for (let k = 1; k < seg; k++) {
      const a = ea + Math.PI / 2 - (k / seg) * Math.PI;
      poly.push(ex + Math.cos(a) * er, ey + Math.sin(a) * er);
    }
    for (let i = Rt.length - 2; i >= 0; i -= 2) poly.push(Rt[i], Rt[i + 1]);
    // 시작 마개: -법선 → 거꾸로 방향 → +법선
    const sx = P[0], sy = P[1], sr = r(0), sa = Math.atan2(t0[1], t0[0]);
    for (let k = 1; k < seg; k++) {
      const a = sa - Math.PI / 2 - (k / seg) * Math.PI;
      poly.push(sx + Math.cos(a) * sr, sy + Math.sin(a) * sr);
    }
    return { poly, joins };
  }

  // curve 는 점 사이를 centripetal Catmull-Rom 곡선으로 잇는 그리기용 점열이다 — 받은 점은 전부 그대로 지나고(쓴 그대로),
  // 점이 성긴 곳(빠른 마우스·손가락)에서 꺾인 직선 대신 둥근 선이 된다. 획 자료(pts)는 바꾸지 않는다. step 은 조각 길이(판 단위)다.
  function curve(pts, step) {
    const n = pts.length / 3;
    if (n < 3) return pts;
    const out = [], MAXSUB = 24;
    const X = i => pts[3 * Math.max(0, Math.min(n - 1, i))], Y = i => pts[3 * Math.max(0, Math.min(n - 1, i)) + 1];
    for (let i = 0; i < n - 1; i++) {
      const x1 = X(i), y1 = Y(i), x2 = X(i + 1), y2 = Y(i + 1), p1 = pts[3 * i + 2], p2 = pts[3 * i + 5];
      out.push(x1, y1, p1);
      const len = Math.hypot(x2 - x1, y2 - y1), sub = Math.min(MAXSUB, Math.floor(len / step));
      if (sub < 2) continue;
      let x0 = X(i - 1), y0 = Y(i - 1), x3 = X(i + 2), y3 = Y(i + 2);
      if (i === 0) { x0 = 2 * x1 - x2; y0 = 2 * y1 - y2; }
      if (i === n - 2) { x3 = 2 * x2 - x1; y3 = 2 * y2 - y1; }
      const d01 = Math.sqrt(Math.hypot(x1 - x0, y1 - y0)) || 1e-6, d12 = Math.sqrt(len) || 1e-6, d23 = Math.sqrt(Math.hypot(x3 - x2, y3 - y2)) || 1e-6;
      // 접선(centripetal 매개변수) → Hermite
      const m1x = (x1 - x0) / d01 - (x2 - x0) / (d01 + d12) + (x2 - x1) / d12, m1y = (y1 - y0) / d01 - (y2 - y0) / (d01 + d12) + (y2 - y1) / d12;
      const m2x = (x2 - x1) / d12 - (x3 - x1) / (d12 + d23) + (x3 - x2) / d23, m2y = (y2 - y1) / d12 - (y3 - y1) / (d12 + d23) + (y3 - y2) / d23;
      const t1x = m1x * d12, t1y = m1y * d12, t2x = m2x * d12, t2y = m2y * d12;
      for (let k = 1; k < sub; k++) {
        const t = k / sub, t2 = t * t, t3 = t2 * t;
        const h1 = 2 * t3 - 3 * t2 + 1, h2 = t3 - 2 * t2 + t, h3 = -2 * t3 + 3 * t2, h4 = t3 - t2;
        out.push(h1 * x1 + h2 * t1x + h3 * x2 + h4 * t2x, h1 * y1 + h2 * t1y + h3 * y2 + h4 * t2y, p1 + (p2 - p1) * t);
      }
    }
    out.push(pts[3 * n - 3], pts[3 * n - 2], pts[3 * n - 1]);
    return out;
  }

  // ---- 상자·맞히기
  function strokeBox(pts, pad) {
    let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity;
    for (let i = 0; i + 2 < pts.length; i += 3) {
      const x = pts[i], y = pts[i + 1];
      if (x < x0) x0 = x; if (x > x1) x1 = x;
      if (y < y0) y0 = y; if (y > y1) y1 = y;
    }
    return [x0 - pad, y0 - pad, x1 + pad, y1 + pad];
  }
  const boxHit = (a, b) => a[0] <= b[2] && a[2] >= b[0] && a[1] <= b[3] && a[3] >= b[1];
  function segDist(px, py, ax, ay, bx, by) {
    const dx = bx - ax, dy = by - ay, l2 = dx * dx + dy * dy;
    let t = l2 ? ((px - ax) * dx + (py - ay) * dy) / l2 : 0;
    t = clamp(t, 0, 1);
    return Math.hypot(px - (ax + t * dx), py - (ay + t * dy));
  }
  // strokeRadius 는 획의 i 번째 점 굵기(반지름)다 — 펜은 필압, 형광펜은 일정.
  const strokeRadius = (s, i) => (s.tool === "hl" ? s.w / 2 : penRadius(s.w, s.pts[3 * i + 2]));
  // hitStroke 는 판의 점 (x, y) 에서 반지름 r 인 지우개 원이 획 s 에 닿는가다.
  function hitStroke(s, x, y, r) {
    const p = s.pts, n = p.length / 3;
    if (n === 1) return Math.hypot(x - p[0], y - p[1]) <= r + strokeRadius(s, 0);
    for (let i = 0; i < n - 1; i++) {
      const rr = Math.max(strokeRadius(s, i), strokeRadius(s, i + 1));
      if (segDist(x, y, p[3 * i], p[3 * i + 1], p[3 * i + 3], p[3 * i + 4]) <= r + rr) return true;
    }
    return false;
  }

  // ---- 공간 색인: 칸(cell 판 단위) 격자 해시. 너무 큰 획(칸 64개 넘게)은 big 에 두고 늘 후보로 본다.
  class Grid {
    constructor(cell) { this.cell = cell || 512; this.map = new Map(); this.big = new Set(); this.stamp = 0; this.size = 0; }
    // range 는 상자 b 가 걸친 칸 번호다 — 좌표를 ±INDEX_MAX 로 끌어 맞춰 칸 번호가 늘 안전한 정수다(i + 1 !== i — 반복이 끝난다).
    // 범위 밖 상자는 끝 칸에 모이고, 후보를 고른 뒤 boxHit 로 다시 보므로 결과는 맞다.
    range(b) {
      const c = this.cell, f = x => Math.floor(clamp(x, -INDEX_MAX, INDEX_MAX) / c);
      return [f(b[0]), f(b[1]), f(b[2]), f(b[3])];
    }
    insert(s) {
      const [i0, j0, i1, j1] = this.range(s.box);
      this.size++;
      if ((i1 - i0 + 1) * (j1 - j0 + 1) > 64) { this.big.add(s); s._cells = null; return; }
      s._cells = [];
      for (let i = i0; i <= i1; i++) for (let j = j0; j <= j1; j++) {
        const k = i + "," + j;
        let a = this.map.get(k);
        if (!a) this.map.set(k, (a = new Set()));
        a.add(s);
        s._cells.push(k);
      }
    }
    remove(s) {
      this.size--;
      if (!s._cells) { this.big.delete(s); return; }
      for (const k of s._cells) {
        const a = this.map.get(k);
        if (a) { a.delete(s); if (!a.size) this.map.delete(k); }
      }
      s._cells = null;
    }
    clear() { this.map.clear(); this.big.clear(); this.size = 0; }
    // query 는 상자 b 에 걸친 획을 id 차례(쓴 차례)로 돌려준다.
    query(b) {
      const st = ++this.stamp, out = [];
      const take = s => { if (s._q !== st && boxHit(s.box, b)) { s._q = st; out.push(s); } };
      const [i0, j0, i1, j1] = this.range(b);
      const cells = (i1 - i0 + 1) * (j1 - j0 + 1);
      if (cells > this.map.size) {
        for (const [k, a] of this.map) {
          const c = k.indexOf(","), i = +k.slice(0, c), j = +k.slice(c + 1);
          if (i >= i0 && i <= i1 && j >= j0 && j <= j1) for (const s of a) take(s);
        }
      } else {
        for (let i = i0; i <= i1; i++) for (let j = j0; j <= j1; j++) {
          const a = this.map.get(i + "," + j);
          if (a) for (const s of a) take(s);
        }
      }
      for (const s of this.big) take(s);
      out.sort((a, b) => a.id - b.id);
      return out;
    }
  }

  // ---- 타일: 한 변 px 장치 픽셀. 배율 z·dpr 에서 타일 하나는 판 단위 T = px / (z * dpr) 이다. 타일 (i, j) 는 판 [iT, (i+1)T) × [jT, (j+1)T).
  const tileWorld = (z, dpr, px) => px / (z * dpr);
  const tileBox = (i, j, T) => [i * T, j * T, (i + 1) * T, (j + 1) * T];
  // tilesFor 는 보기 v 로 cssW × cssH 화면을 덮는 타일 (i, j) 들이다 — 가운데에서 가까운 차례(먼저 구울 것).
  // 보기 원점은 ±INDEX_MAX 로 끌어 맞춘다 — 타일 번호가 안전한 정수라 반복이 끝난다(엔진의 보기는 boundView 로 이미 좌표 범위 안이다).
  function tilesFor(v, cssW, cssH, T) {
    if (!(T > 0) || !(v.z > 0)) return [];
    const x0 = clamp(v.x, -INDEX_MAX, INDEX_MAX), y0 = clamp(v.y, -INDEX_MAX, INDEX_MAX), x1 = x0 + cssW / v.z, y1 = y0 + cssH / v.z;
    const i0 = Math.floor(x0 / T), j0 = Math.floor(y0 / T), i1 = Math.floor((x1 - 1e-9) / T), j1 = Math.floor((y1 - 1e-9) / T);
    const ci = (x0 + x1) / 2 / T - 0.5, cj = (y0 + y1) / 2 / T - 0.5, out = [];
    for (let j = j0; j <= j1; j++) for (let i = i0; i <= i1; i++) out.push([i, j, (i - ci) * (i - ci) + (j - cj) * (j - cj)]);
    out.sort((a, b) => a[2] - b[2]);
    return out.map(t => [t[0], t[1]]);
  }

  // ---- 파일 형식(머리 주석 「파일 형식 v1」)
  const FORMAT = "itda-whiteboard", VERSION = 1;
  const COLORS = 8; // 색 번호 개수(whiteboard.js 의 팔레트 길이와 같다)
  const BGS = ["dark", "light", "grid"];
  // LIMIT 은 판 하나의 상한이다(잠정 — 화이트보드 기획 ④). 넘는 파일은 읽지 않고, 표면은 넘게 그리지 않는다(whiteboard.js room).
  const LIMIT = { strokes: 50000, points: 3000000 };
  const KNOWN = ["format", "v", "name", "bg", "view", "strokes"];
  const r2 = x => Math.round(x * 100) / 100, r3 = x => Math.round(x * 1000) / 1000, r4 = x => Math.round(x * 10000) / 10000;
  const rv = x => r2(clamp(x, -COORD_MAX, COORD_MAX)), rw = w => clamp(r3(w), W_MIN, W_MAX); // 보기·굵기 — 범위 안으로 끌어 맞춘 반올림(serialize)
  // rp 는 획의 점 좌표다 — 끌어 맞추지 않는다(범위 밖이면 던진다 — 머리 주석 「쓸 때」).
  const rp = x => { if (!(Number.isFinite(x) && Math.abs(x) <= COORD_MAX)) throw new RangeError("획의 점 좌표 " + x + " 가 좌표 범위(±" + COORD_MAX + ") 밖이다"); return r2(x); };
  const fin = x => typeof x === "number" && Number.isFinite(x);

  // serialize 는 판 b = { name, bg, view: {x, y, z}, strokes: [{ tool, color, w, pts }], extra } 를 파일 본문(JSON 문자열)으로 만든다.
  // extra(모르는 최상위 키)는 아는 키 뒤에 그대로 싣는다 — 아는 키와 이름이 겹치면 아는 키가 이긴다.
  // 획의 점 좌표가 범위 밖이면 RangeError 를 던진다(부르는 쪽은 저장을 멈추고 판을 메모리에 둔다 — whiteboard.js snapshot).
  function serialize(b) {
    const v = b.view || { x: 0, y: 0, z: 1 };
    const out = { format: FORMAT, v: VERSION, name: String(b.name || ""), bg: BGS.includes(b.bg) ? b.bg : "dark",
      view: { x: rv(v.x), y: rv(v.y), z: r4(clamp(v.z, ZMIN, ZMAX)) }, strokes: [] };
    for (const s of b.strokes || []) {
      const p = new Array(s.pts.length);
      for (let i = 0; i < s.pts.length; i += 3) { p[i] = rp(s.pts[i]); p[i + 1] = rp(s.pts[i + 1]); p[i + 2] = r3(s.pts[i + 2]); }
      out.strokes.push({ t: s.tool === "hl" ? "hl" : "pen", c: s.color, w: rw(s.w), p });
    }
    const extra = b.extra || {};
    for (const k of Object.keys(extra)) if (!KNOWN.includes(k)) out[k] = extra[k];
    return JSON.stringify(out);
  }

  // parse 는 파일 본문을 검증해 { ok: true, board, readOnly, v } 또는 { ok: false, error } 를 돌려준다. lim 은 시험용 상한(없으면 LIMIT).
  // board = { name, bg, view, strokes: [{ tool, color, w, pts }], extra }.
  function parse(text, lim) {
    lim = lim || LIMIT;
    let j;
    try { j = JSON.parse(text); } catch (e) { return { ok: false, error: "JSON 이 아니다" }; }
    if (!j || typeof j !== "object" || Array.isArray(j)) return { ok: false, error: "최상위가 객체가 아니다" };
    if (j.format !== FORMAT) return { ok: false, error: "format 이 " + FORMAT + " 가 아니다" };
    if (!Number.isInteger(j.v) || j.v < 1) return { ok: false, error: "v 가 1 이상 정수가 아니다" };
    if (j.name != null && typeof j.name !== "string") return { ok: false, error: "name 이 문자열이 아니다" };
    if (j.bg != null && typeof j.bg !== "string") return { ok: false, error: "bg 가 문자열이 아니다" };
    let view = { x: 0, y: 0, z: 1 };
    if (j.view != null) {
      const v = j.view;
      if (typeof v !== "object" || !fin(v.x) || !fin(v.y) || !fin(v.z) || v.z <= 0) return { ok: false, error: "view 가 유한한 {x, y, z} 가 아니다" };
      view = { x: clamp(v.x, -COORD_MAX, COORD_MAX), y: clamp(v.y, -COORD_MAX, COORD_MAX), z: clamp(v.z, ZMIN, ZMAX) };
    }
    if (!Array.isArray(j.strokes)) return { ok: false, error: "strokes 가 배열이 아니다" };
    if (j.strokes.length > lim.strokes) return { ok: false, error: "획이 상한(" + lim.strokes + ")을 넘는다" };
    const strokes = [];
    let points = 0;
    for (let k = 0; k < j.strokes.length; k++) {
      const s = j.strokes[k], bad = why => ({ ok: false, error: "획 " + k + ": " + why });
      if (!s || typeof s !== "object") return bad("객체가 아니다");
      if (s.t !== "pen" && s.t !== "hl") return bad("t 가 pen·hl 이 아니다");
      if (!Number.isInteger(s.c) || s.c < 0 || s.c >= COLORS) return bad("c 가 0~" + (COLORS - 1) + " 정수가 아니다");
      if (!fin(s.w) || s.w < W_MIN || s.w > W_MAX) return bad("w 가 " + W_MIN + "~" + W_MAX + " 의 유한수가 아니다");
      if (!Array.isArray(s.p) || s.p.length < 3 || s.p.length % 3) return bad("p 가 [x, y, 필압, …] 꼴이 아니다");
      points += s.p.length / 3;
      if (points > lim.points) return { ok: false, error: "점이 상한(" + lim.points + ")을 넘는다" };
      const pts = new Array(s.p.length);
      for (let i = 0; i < s.p.length; i++) {
        const x = s.p[i];
        if (!fin(x)) return bad("p[" + i + "] 가 유한수가 아니다");
        if (i % 3 !== 2 && Math.abs(x) > COORD_MAX) return bad("p[" + i + "] 가 좌표 범위(±" + COORD_MAX + ") 밖이다");
        pts[i] = i % 3 === 2 ? clamp(x, 0, 1) : x;
      }
      strokes.push({ tool: s.t, color: s.c, w: s.w, pts });
    }
    const extra = {};
    for (const key of Object.keys(j)) if (!KNOWN.includes(key)) extra[key] = j[key];
    const board = { name: j.name || "", bg: BGS.includes(j.bg) ? j.bg : "dark", view, strokes, extra };
    return { ok: true, board, readOnly: j.v > VERSION, v: j.v };
  }

  return {
    ZMIN, ZMAX, COORD_MAX, W_MIN, W_MAX, clamp, toWorld, toScreen, anchor, zoomAt, panBy, boundView, inRange, snap,
    penRadius, speedPressure, outline, curve, strokeBox, boxHit, segDist, strokeRadius, hitStroke,
    Grid, tileWorld, tileBox, tilesFor,
    FORMAT, VERSION, COLORS, BGS, LIMIT, serialize, parse,
  };
});
