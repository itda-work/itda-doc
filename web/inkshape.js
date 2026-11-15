// 판서의 모양 — 슬라이드 발표(ink.js)와 공유 보는 화면(shareink-view.js)이 함께 쓰는 순수 함수와 값이다. 두 쪽이 같은 경로·색·혼합 판정을 쓰므로 획 모양의 규칙은 여기 하나뿐이다.
// 획은 {tool: "pen"|"hl", color(팔레트 번호), width(0~2), pts[x,y,…], press[0~1,…]|null} 이다(설계 px). 모델·입력·공유는 모른다.
(function () {
  "use strict";

  const COLORS = ["#e5342b", "#f28c1e", "#ffd400", "#2fa84f", "#1f6fe0", "#8a3fd1", "#111111", "#ffffff"];
  const HL_SKIP = [6, 7]; // 형광펜에 없는 색 — 검정은 스크린에서, 흰색은 곱하기에서 보이지 않는다
  const WIDTHS = { pen: [2.5, 5, 9], hl: [14, 22, 34] }; // 굵기 3단(설계 px)
  const LASER_TAIL = 300, LASER_LINE = 1500; // 레이저 꼬리·끈 선이 사라지는 시간(ms)
  const DARK_LUM = 0.25;   // 슬라이드 바탕 상대 휘도가 이보다 낮으면 어두운 바탕이다

  // ---- 획 기하
  const r1 = v => Math.round(v * 10) / 10;
  const pressWidth = (w, p) => w * (0.35 + 1.1 * p); // 필압 0.59 쯤이 원래 굵기다
  const widthOf = s => WIDTHS[s.tool][s.width];
  const dot = (cx, cy, c) => "M" + +(cx - c).toFixed(2) + " " + cy + "a" + c + " " + c + " 0 1 0 " + 2 * c + " 0a" + c + " " + c + " 0 1 0 " + -2 * c + " 0Z";
  // linePath 는 고른 굵기 획이다 — 점 사이 가운데를 잇는 2차 곡선.
  function linePath(pts) {
    const n = pts.length / 2;
    let d = "M" + pts[0] + " " + pts[1];
    for (let i = 1; i < n - 1; i++) {
      const x = pts[2 * i], y = pts[2 * i + 1];
      d += "Q" + x + " " + y + " " + r1((x + pts[2 * i + 2]) / 2) + " " + r1((y + pts[2 * i + 3]) / 2);
    }
    return d + "L" + pts[2 * n - 2] + " " + pts[2 * n - 1];
  }
  // pressPiece 는 필압 획의 k 번째 조각(k 째 점의 원 + 다음 점까지의 사다리꼴)이다. 조각이 모두 같은 돌림이라 nonzero 채우기로 겹쳐도 구멍이 나지 않는다.
  function pressPiece(s, k) {
    const pts = s.pts, x = pts[2 * k], y = pts[2 * k + 1], r = r1(pressWidth(widthOf(s), s.press[k]) / 2);
    let d = dot(x, y, r);
    if (2 * k + 2 >= pts.length) return d;
    const x2 = pts[2 * k + 2], y2 = pts[2 * k + 3], r2 = r1(pressWidth(widthOf(s), s.press[k + 1]) / 2), len = Math.hypot(x2 - x, y2 - y) || 1;
    const nx = -(y2 - y) / len, ny = (x2 - x) / len;
    d += "M" + r1(x + nx * r) + " " + r1(y + ny * r) + "L" + r1(x2 + nx * r2) + " " + r1(y2 + ny * r2) + "L" + r1(x2 - nx * r2) + " " + r1(y2 - ny * r2) + "L" + r1(x - nx * r) + " " + r1(y - ny * r) + "Z";
    return d;
  }
  // filled·pathOf 가 획 모양의 하나뿐인 규칙이다 — SVG 도 canvas 도 이 경로를 그대로 쓰므로 획을 끝내도 모양이 바뀌지 않는다. 점 하나는 채운 원이다(canvas 는
  // 길이 0 인 선을 그리지 않는다), 필압 획은 채운 조각이다. 그 밖은 둥근 끝의 선이다.
  const filled = s => !!s.press || s.pts.length === 2;
  function pathOf(s) {
    if (!s.press) return s.pts.length === 2 ? dot(s.pts[0], s.pts[1], widthOf(s) / 2) : linePath(s.pts);
    let d = "";
    for (let k = 0; k < s.pts.length / 2; k++) d += pressPiece(s, k);
    return d;
  }
  // dress 는 SVG path 에 획 s 의 모양을 입힌다 — 표현 속성만 쓴다(CSP style-src 'self').
  function dress(p, s) {
    p.setAttribute("d", pathOf(s));
    if (filled(s)) {
      p.setAttribute("fill", COLORS[s.color]);
      p.setAttribute("stroke", "none");
    } else {
      p.setAttribute("fill", "none");
      p.setAttribute("stroke", COLORS[s.color]);
      p.setAttribute("stroke-width", String(widthOf(s)));
      p.setAttribute("stroke-linecap", "round");
      p.setAttribute("stroke-linejoin", "round");
    }
  }

  // ---- 슬라이드 바탕의 밝기: 형광펜 혼합을 고른다. 판서 색이 아니라 슬라이드 바탕이 기준이다. kia 의 제목·맨 구역 장처럼 면이 장식(.slide-deco 의
  // 가상 요소)으로 칠해지는 슬라이드가 있어 쪽 배경색만으로는 모른다. 슬라이드 절반 이상을 덮는 불투명한 장식 면이 있으면 그 색을 본다.
  function parseColor(c) {
    let m = /^rgba?\(([\d.]+)[, ]+([\d.]+)[, ]+([\d.]+)(?:[,/ ]+([\d.]+))?/.exec(c || "");
    if (m) return { r: m[1] / 255, g: m[2] / 255, b: m[3] / 255, a: m[4] === undefined ? 1 : +m[4] };
    m = /^color\(srgb ([\d.]+) ([\d.]+) ([\d.]+)(?: *\/ *([\d.]+))?/.exec(c || "");
    return m ? { r: +m[1], g: +m[2], b: +m[3], a: m[4] === undefined ? 1 : +m[4] } : null;
  }
  function luminance(c) {
    const f = v => v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
    return 0.2126 * f(c.r) + 0.7152 * f(c.g) + 0.0722 * f(c.b);
  }
  function slideDark(pg, w, h) {
    const cs = window.getComputedStyle;
    if (!cs) return false;
    let c = parseColor(cs(pg).backgroundColor);
    const deco = pg.querySelector(".slide-deco");
    if (deco) for (const part of ["::before", "::after"]) {
      const s = cs(deco, part), face = parseColor(s.backgroundColor);
      if (face && face.a > 0.5 && parseFloat(s.width) >= w / 2 && parseFloat(s.height) >= h / 2) c = face;
    }
    return !!c && c.a > 0.5 && luminance(c) < DARK_LUM;
  }

  window.ItdaInkShape = { COLORS, HL_SKIP, WIDTHS, LASER_TAIL, LASER_LINE, r1, widthOf, filled, pathOf, dress, parseColor, luminance, slideDark };
})();
