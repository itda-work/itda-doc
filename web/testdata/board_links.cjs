// dashboard.js 의 타일 잇기(links·linkTargets·linkSides·linkEnds·linkRoute·linkGeometry)를 node 로 돌린다(board_test.go TestBoardLinks). 인자: dashboard.js.
// 순수 함수(가장자리 고르기·꺾는 꼴·한 가장자리에 여럿 벌리기·곧게 맞추기)와, 작은 가짜 DOM 위에서 build 를 끝까지 돌린 선 층(svg.board-links)의 꼴·data 속성·
// 읽는 프로그램용 글·to 없는 문서의 마크업 불변·크게 보기 동안 숨김·발표 복제(id 를 지운 복제본에 선이 따라가고 id 가 없음)를 본다.
// 가짜 DOM 의 타일 자리는 인라인 grid-column·grid-row 에서 셈한다(열 100px·행 80px·틈 10px) — 실제 화면은 Chrome 헤드리스 캡처로 봤다(명세 dashboard.md 8장).
"use strict";
const fs = require("fs"), vm = require("vm");

// ---- 가짜 DOM(board_motion.cjs 와 같은 꼴 + createElementNS·cloneNode·offset*·속성 dump)
function expandIs(sel) {
  const m = sel.match(/:is\(([^)]*)\)/);
  if (!m) return [sel];
  return m[1].split(",").flatMap(alt => expandIs(sel.replace(m[0], alt.trim())));
}
function pieces(sel) {
  const out = [];
  let depth = 0, cur = "";
  for (const ch of sel) {
    if (ch === "(") depth++;
    if (ch === ")") depth--;
    if (ch === "," && !depth) { out.push(cur); cur = ""; continue; }
    cur += ch;
  }
  out.push(cur);
  return out.flatMap(p => expandIs(p.trim()));
}
function compound(c) {
  c = c.replace(/:not\([^)]*\)/g, "");
  const tag = (c.match(/^[a-z][a-z0-9]*/i) || [""])[0].toUpperCase();
  return { scope: c.includes(":scope"), tag, cls: [...c.matchAll(/\.([\w-]+)/g)].map(m => m[1]), attrs: [...c.matchAll(/\[([\w-]+)[^\]]*\]/g)].map(m => m[1]) };
}
function steps(p) {
  const toks = p.replace(/\s*>\s*/g, " > ").trim().split(/\s+/);
  const out = [];
  for (let i = toks.length - 1; i >= 0; i--) {
    if (toks[i] === ">") { out[out.length - 1].comb = ">"; continue; }
    out.push({ c: compound(toks[i]), comb: " " });
  }
  return out;
}
function one(e, c, scope) {
  if (c.scope) return e === scope;
  if (c.tag && c.tag !== e.tagName.toUpperCase()) return false;
  return c.cls.every(x => e.cls.has(x)) && c.attrs.every(a => a in e.attrs);
}
function matchSteps(e, st, i, scope) {
  if (!one(e, st[i].c, scope)) return false;
  if (i === st.length - 1) return true;
  if (st[i].comb === ">") return !!e.parentElement && matchSteps(e.parentElement, st, i + 1, scope);
  for (let a = e.parentElement; a; a = a.parentElement) if (matchSteps(a, st, i + 1, scope)) return true;
  return false;
}
const COL = 100, ROW = 80, GAP = 10;
const span = (v, n) => { const m = /^(\d+)(?: \/ span (\d+))?$/.exec(String(v || "")); return m ? [(+m[1] - 1) * (n + GAP), (+(m[2] || 1)) * n + (+(m[2] || 1) - 1) * GAP] : [0, 0]; };
class El {
  constructor(tag, cls, ns) {
    this.tagName = ns ? tag : tag.toUpperCase(); this.ns = ns || null; this.cls = new Set((cls || "").split(" ").filter(Boolean)); this.children = []; this.parentElement = null;
    this.attrs = {}; this.dataset = {}; this._text = ""; this.on = {};
    this.style = { vars: {}, setProperty(k, v) { this.vars[k] = String(v); }, removeProperty(k) { delete this.vars[k]; } };
  }
  get classList() {
    const s = this.cls;
    return { add: (...c) => c.forEach(x => s.add(x)), remove: (...c) => c.forEach(x => s.delete(x)), contains: c => s.has(c), toggle: (c, on) => (on ? s.add(c) : s.delete(c)) };
  }
  set className(v) { this.cls = new Set(v.split(" ").filter(Boolean)); }
  get className() { return [...this.cls].join(" "); }
  get id() { return this.attrs.id || ""; }
  get isConnected() { return true; }
  get textContent() { return this._text + this.children.map(c => c.textContent).join(""); }
  set textContent(v) { this._text = String(v); for (const c of [...this.children]) c.remove(); }
  get firstChild() { return this.children[0] || null; }
  get firstElementChild() { return this.children[0] || null; }
  get lastElementChild() { return this.children[this.children.length - 1] || null; }
  // 자리: 타일은 인라인 grid-column·grid-row, 격자는 열 수(data-board-cols)·행 수(grid-template-rows)
  get offsetLeft() { return this.cls.has("tile") ? span(this.style.gridColumn, COL)[0] : 0; }
  get offsetTop() { return this.cls.has("tile") ? span(this.style.gridRow, ROW)[0] : 0; }
  get offsetWidth() {
    if (this.cls.has("tile")) return span(this.style.gridColumn, COL)[1];
    if (this.cls.has("board-grid")) { const n = +this.dataset.boardCols; return n * COL + (n - 1) * GAP; }
    return 0;
  }
  get offsetHeight() {
    if (this.cls.has("tile")) return span(this.style.gridRow, ROW)[1];
    if (this.cls.has("board-grid")) { const n = (this.style.gridTemplateRows || "").split(/ (?=auto|var)/).filter(Boolean).length; return n * ROW + (n - 1) * GAP; }
    return 0;
  }
  appendChild(c) { if (c.parentElement) c.remove(); c.parentElement = this; this.children.push(c); return c; }
  insertBefore(c, ref) {
    if (c.parentElement) c.remove();
    const i = ref ? this.children.indexOf(ref) : -1;
    c.parentElement = this;
    if (i < 0) this.children.push(c); else this.children.splice(i, 0, c);
    return c;
  }
  remove() { const p = this.parentElement; if (p) p.children.splice(p.children.indexOf(this), 1); this.parentElement = null; }
  setAttribute(k, v) { if (k === "class") this.className = String(v); else this.attrs[k] = String(v); }
  getAttribute(k) { return k === "class" ? this.className : k in this.attrs ? this.attrs[k] : null; }
  removeAttribute(k) { delete this.attrs[k]; }
  focus() {}
  matches(sel, scope) { return pieces(sel).some(p => matchSteps(this, steps(p), 0, scope)); }
  closest(sel) { for (let e = this; e; e = e.parentElement) if (e.matches(sel)) return e; return null; }
  all() { return this.children.flatMap(c => [c, ...c.all()]); }
  querySelectorAll(sel) { return this.all().filter(e => e.matches(sel, this)); }
  querySelector(sel) { return this.querySelectorAll(sel)[0] || null; }
  addEventListener(t, fn) { (this.on[t] = this.on[t] || []).push(fn); }
  removeEventListener(t, fn) { this.on[t] = (this.on[t] || []).filter(f => f !== fn); }
  cloneNode() {
    const c = new El(this.ns ? this.tagName : this.tagName.toLowerCase(), this.className, this.ns);
    Object.assign(c.attrs, this.attrs); Object.assign(c.dataset, this.dataset); c._text = this._text;
    Object.assign(c.style.vars, this.style.vars);
    for (const k of ["gridColumn", "gridRow", "gridTemplateColumns", "gridTemplateRows"]) if (this.style[k]) c.style[k] = this.style[k];
    for (const ch of this.children) c.appendChild(ch.cloneNode(true));
    return c;
  }
  // 마크업 비교용 꼴: 태그.클래스[data]<속성>{style 변수} (자식…) 글
  dump() {
    const d = Object.keys(this.dataset).sort().map(k => k + "=" + this.dataset[k]).join(",");
    const a = Object.keys(this.attrs).sort().map(k => k + "=" + this.attrs[k]).join(",");
    const v = Object.keys(this.style.vars).sort().map(k => k + "=" + this.style.vars[k]).join(",");
    const extra = ["gridColumn", "gridRow", "gridTemplateColumns", "gridTemplateRows"].filter(k => this.style[k]).map(k => k + "=" + this.style[k]).join(",");
    return this.tagName.toLowerCase() + [...this.cls].sort().map(c => "." + c).join("") + (d ? "[" + d + "]" : "") + (a ? "<" + a + ">" : "") +
      (v || extra ? "{" + [v, extra].filter(Boolean).join(",") + "}" : "") +
      (this._text ? JSON.stringify(this._text) : "") + (this.children.length ? "(" + this.children.map(c => c.dump()).join(" ") + ")" : "");
  }
}
const SVGNS = "http://www.w3.org/2000/svg";
const DOC = { createElement: tag => new El(tag), createElementNS: (ns, tag) => new El(tag, "", ns) };
const ctx = {
  document: DOC, performance: { now: () => 0 },
  getComputedStyle: () => ({ fontSize: "12px", columnGap: GAP + "px" }),
};
ctx.window = ctx;
ctx.window.ItdaSlides = { SIZE: { "16:9": [1280, 720], "4:3": [960, 720] } };
ctx.window.ItdaPager = { fitMath() {} };
vm.createContext(ctx);
vm.runInContext(fs.readFileSync(process.argv[2], "utf8"), ctx);
const Bd = ctx.window.ItdaBoard;

let cases = 0, failed = 0;
function eq(name, got, want) {
  cases++;
  const g = JSON.stringify(got), w = JSON.stringify(want);
  if (g !== w) { failed++; console.log("FAIL " + name + "\n  got  " + g + "\n  want " + w); }
}
function ok(name, cond) { eq(name, !!cond, true); }
// 제목에 문서가 단 class tile-links-label(자유 어휘)도 넣는다 — links 가 만든 숨긴 글이 아니므로 지우지 않는다(리뷰 1회차 [1])
const NOLINK = [["h1", "구역", "", null, "구역"], ["h2", "결제", "ok", { at: "1,1" }, "pay"], ["p", "글"], ["h2", "덧", "tile-links-label", null, "덧"], ["p", "x"],
  ["hr", ""], ["h2", "정산", "", { tone: "up" }, "settle"], ["table", "칸"]];
// dump: 아래 BEFORE 를 만드는 길 — 이 기능 전 dashboard.js 로 NOLINK 를 그린 꼴을 찍고 끝낸다(옛 파일에는 link* 함수가 없다)
if (process.argv[3] === "dump") {
  const T = new El("div");
  Bd.build(flow(NOLINK), T, { level: 2, cols: 4, info: { title: "보드" }, motion: false });
  console.log(T.dump());
  process.exit(0);
}

// flow 만들기: [["h2", "제목", "cls", {to: "b"}, "id"], ["p", "글"], …]
function flow(spec) {
  const f = new El("div", "page-body doc-flow");
  for (const [tag, text, cls, data, id] of spec) {
    const e = f.appendChild(new El(tag, cls || ""));
    e.textContent = text;
    Object.assign(e.dataset, data || {});
    if (id) e.attrs.id = id;
  }
  return f;
}
const build = (spec, target, o) => Bd.build(flow(spec), target, Object.assign({ level: 2, cols: 4, info: { title: "보드" }, motion: false }, o || {}));
const R = (x, y, w, h) => ({ x, y, w, h });

// ---- linkTargets: 쉼표로 나눔·앞뒤 공백 뗌·빈 항목과 겹침 뺌
{
  const h = to => { const e = new El("h2"); if (to != null) e.dataset.to = to; return e; };
  eq("linkTargets", [h(null), h(""), h("a"), h("a,b"), h(" a , b ,,a,"), h("b,a")].map(Bd.linkTargets), [[], [], ["a"], ["a", "b"], ["a", "b"], ["b", "a"]]);
  eq("linkTargets: 제목 없음", Bd.linkTargets(null), []);
}

// ---- linkSides: 온전히 오른쪽·왼쪽이면 좌우, 열이 겹치면 위아래
{
  const a = R(110, 90, 100, 80);
  eq("linkSides", [
    Bd.linkSides(a, R(220, 90, 100, 80)), // 오른쪽 이웃
    Bd.linkSides(a, R(330, 270, 100, 80)), // 오른쪽 아래(열이 안 겹침)
    Bd.linkSides(a, R(0, 0, 100, 80)), // 왼쪽 위
    Bd.linkSides(a, R(110, 180, 100, 80)), // 바로 아래
    Bd.linkSides(a, R(60, 180, 100, 80)), // 열이 겹치는 아래
    Bd.linkSides(a, R(160, 0, 210, 80)), // 열이 겹치는 위
    Bd.linkSides(a, R(210, 90, 100, 80)), // 틈 없이 붙은 오른쪽(경계 같음)
  ], [["right", "left"], ["right", "left"], ["left", "right"], ["bottom", "top"], ["bottom", "top"], ["top", "bottom"], ["right", "left"]]);
}

// ---- linkRoute: 한 줄이면 곧은 선, 아니면 출발 옆 틈에서 두 번 꺾는다(ㄹ자)
eq("linkRoute: 가로 곧은 선", Bd.linkRoute([100, 40], "right", [110, 40], 3), [[100, 40], [110, 40]]);
eq("linkRoute: 0.5px 안이면 출발 줄에 맞춘 곧은 선", Bd.linkRoute([100, 40], "right", [110, 40.4], 3), [[100, 40], [110, 40]]);
eq("linkRoute: 오른쪽으로 꺾음", Bd.linkRoute([100, 40], "right", [320, 130], 3), [[100, 40], [103, 40], [103, 130], [320, 130]]);
eq("linkRoute: 왼쪽으로 꺾음", Bd.linkRoute([220, 40], "left", [100, 130], 3), [[220, 40], [217, 40], [217, 130], [100, 130]]);
eq("linkRoute: 세로 곧은 선", Bd.linkRoute([50, 80], "bottom", [50, 90], 3), [[50, 80], [50, 90]]);
eq("linkRoute: 아래로 꺾음", Bd.linkRoute([50, 80], "bottom", [160, 180], 3), [[50, 80], [50, 83], [160, 83], [160, 180]]);
eq("linkRoute: 위로 꺾음", Bd.linkRoute([160, 180], "top", [50, 80], 3), [[160, 180], [160, 177], [50, 177], [50, 80]]);

// ---- linkEnds: 한 가장자리에 여럿이면 상대 가운데 차례로 벌리고, 혼자 닿는 끝은 맞은편 줄에 맞춘다
{
  // A(0,0) 에서 오른쪽 셋: 같은 줄 B, 아래 줄 C, 그 아래 D — A 의 오른쪽 가장자리(높이 170)에 끝 셋
  const rects = [R(0, 0, 100, 170), R(110, 0, 100, 170), R(220, 180, 100, 80), R(330, 270, 100, 80)];
  const e = Bd.linkEnds(rects, [[0, 3], [0, 1], [0, 2]]); // 적은 차례와 상관없이 상대 가운데(y) 차례로 놓인다
  eq("linkEnds: 가장자리", e.map(x => x.sides), [["right", "left"], ["right", "left"], ["right", "left"]]);
  eq("linkEnds: A 쪽 끝은 상대 가운데 차례(B 위·C 가운데·D 아래)로 벌림 — 간격 14", e.map(x => x.p[0]), [[100, 99], [100, 71], [100, 85]]);
  eq("linkEnds: 출발 가장자리의 차례·수", e.map(x => [x.rank, x.of]), [[2, 3], [0, 3], [1, 3]]);
  eq("linkEnds: 혼자 닿는 B 의 끝은 A 쪽 끝의 줄로(곧은 선)", e[1].p[1], [110, 71]);
  eq("linkEnds: 마주 보는 구간이 없는 C·D 는 가운데", [e[2].p[1], e[0].p[1]], [[220, 220], [330, 310]]);
  // 짧은 가장자리: 간격은 길이 / (수 + 1) 을 넘지 않는다
  const s = Bd.linkEnds([R(0, 0, 100, 30), R(110, 0, 100, 30), R(110, 40, 100, 30), R(110, 80, 100, 30)], [[0, 1], [0, 2], [0, 3]]);
  eq("linkEnds: 짧은 가장자리의 간격 30/4", s.map(x => x.p[0][1]), [7.5, 15, 22.5]);
  // 둘 다 혼자면 겹침 구간의 가운데 — 높이가 다른 이웃도 곧은 선
  const t = Bd.linkEnds([R(0, 0, 100, 170), R(110, 0, 100, 80)], [[0, 1]]);
  eq("linkEnds: 둘 다 혼자 — 겹침 가운데", t[0].p, [[100, 40], [110, 40]]);
  // 마주 보는 연결(a→b, b→a): 양쪽 가장자리에 둘씩 — 같은 차례로 벌려 나란한 곧은 선 둘
  const m = Bd.linkEnds([R(0, 0, 100, 80), R(110, 0, 100, 80)], [[0, 1], [1, 0]]);
  eq("linkEnds: 마주 보는 연결", m.map(x => x.p), [[[100, 33], [110, 33]], [[110, 47], [100, 47]]]);
  eq("linkEnds: 같은 입력이면 같은 자리", JSON.stringify(Bd.linkEnds(rects, [[0, 3], [0, 1], [0, 2]])), JSON.stringify(e));
}

// ---- linkGeometry: 촉은 도착 앞 틈 안, 꺾는 자리는 출발 옆 틈 안, 0.1px 반올림
{
  const rects = [R(0, 0, 100, 80), R(110, 0, 100, 80), R(220, 90, 100, 80), R(0, 90, 100, 80)];
  const g = Bd.linkGeometry(rects, [[0, 1], [0, 2], [3, 0]], 10);
  // 틈 10 → 촉 길이 7.5(밑동 x 102.5, 반너비 7.5 × 0.45), 선은 밑동보다 0.5 더 들어가 촉과 이어진다
  eq("linkGeometry: 곧은 선 — 촉 밑동까지(+0.5) 줄인 선과 틈 안의 촉", g[0], { line: [[100, 33], [103, 33]], head: [[110, 33], [102.5, 36.4], [102.5, 29.6]], sides: ["right", "left"] });
  const xs = g[1].line.map(p => p[0]);
  ok("linkGeometry: 꺾는 자리는 출발 옆 틈(100–110) 안", xs[1] > 100 && xs[1] < 110 && xs[1] === xs[2]);
  ok("linkGeometry: 촉 끝은 도착 가장자리, 밑동은 그 앞 틈(210–220) 안", g[1].head[0][0] === 220 && g[1].head[1][0] >= 210);
  eq("linkGeometry: 아래 타일에서 위로 곧은 선", [g[2].sides, g[2].head[0]], [["top", "bottom"], [50, 80]]);
  eq("linkGeometry: 결정적", JSON.stringify(Bd.linkGeometry(rects, [[0, 1], [0, 2], [3, 0]], 10)), JSON.stringify(g));
  const big = Bd.linkGeometry([R(0, 0, 100, 80), R(140, 0, 100, 80)], [[0, 1]], 40), small = Bd.linkGeometry([R(0, 0, 100, 80), R(104, 0, 100, 80)], [[0, 1]], 4);
  eq("linkGeometry: 촉 길이는 틈에 맞춤(상한 10·틈보다 길지 않음)", [140 - big[0].head[1][0], 104 - small[0].head[1][0]], [10, 4]);
  // 틈 0(타일이 맞붙음 — 리뷰 1회차 [2]): 0 은 0 이다. 촉 없음, 선은 줄이지 않고 도착 가장자리에서 끝, 꺾는 자리는 출발 가장자리 위(도착 안으로 들어가지 않음)
  const z = Bd.linkGeometry([R(0, 0, 100, 80), R(100, 80, 100, 80)], [[0, 1]], 0);
  eq("linkGeometry: 틈 0 — 촉 없음·꺾는 자리는 출발 가장자리·같은 점은 하나로", z[0], { line: [[100, 40], [100, 120]], head: [], sides: ["right", "left"] });
  const z2 = Bd.linkGeometry([R(0, 0, 100, 80), R(100, 0, 100, 80), R(0, 80, 100, 80)], [[0, 1], [0, 2]], 0);
  eq("linkGeometry: 틈 0 — 맞붙은 이웃은 길이 0 선(점 하나)·촉 없음", z2.map(g => [g.line, g.head]), [[[[100, 40]], []], [[[50, 80]], []]]);
  ok("linkGeometry: 틈 0 — 어느 점도 도착 타일 안으로 들어가지 않음", z[0].line.every(p => p[0] <= 100));
  // 잴 수 없는 틈(NaN·음수·무한)만 기본 12px
  const at12 = JSON.stringify(Bd.linkGeometry(rects, [[0, 1], [0, 2]], 12));
  eq("linkGeometry: 잴 수 없는 틈은 12px", [NaN, -1, Infinity, undefined].map(v => JSON.stringify(Bd.linkGeometry(rects, [[0, 1], [0, 2]], v)) === at12), [true, true, true, true]);
}

// ---- build: to 없는 문서는 마크업이 이 기능 전과 같다(아래 꼴은 이 변경 전 dashboard.js(main ba85877)로 같은 입력을 그린 결과)
const BEFORE = "div.boards.r169{--page-h=720px,--page-w=1280px}(div.board.page[kind=board,page=1,slide=1,steps=0]{--board-rows=3}(div.board-deco header.board-head(div.bh-title\"보드\") div.board-grid.page-body[boardCols=4]{gridTemplateColumns=repeat(4, minmax(0, 1fr)),gridTemplateRows=auto var(--board-row-h) var(--board-row-h)}(div.board-section{gridColumn=1 / -1,gridRow=1}(h1<id=구역>\"구역\") section.tile.tile-ok[boardAt=1,1,boardFixed=,boardSeg=1,boardSpan=1x1]{gridColumn=1 / span 1,gridRow=2}(header.tile-head(span.tile-status.tile-status-ok(span.tile-status-label\"정상\") h2.ok[at=1,1]<id=pay>\"결제\") div.tile-body(p\"글\")) section.tile[boardAt=2,1,boardSeg=1,boardSpan=1x1]{gridColumn=2 / span 1,gridRow=2}(header.tile-head(h2.tile-links-label<id=덧>\"덧\") div.tile-body(p\"x\")) section.tile.w2[boardAt=1,1,boardSeg=2,boardSpan=2x1]{gridColumn=1 / span 2,gridRow=3}(header.tile-head(h2[tone=up]<id=settle>\"정산\") div.tile-body(table\"칸\")))))";
{
  const T = new El("div");
  const r = build(NOLINK, T);
  eq("to 없음: 보드 마크업이 전과 같음", T.dump(), BEFORE);
  eq("to 없음: 돌려주는 값에 links 없음", "links" in r, false);
  eq("to 없음: links 를 불러도 아무것도 만들지 않음", [Bd.links(T.querySelector(".page.board")), T.dump() === BEFORE], [0, true]);
}

// ---- build: 선 층·data 속성·읽는 프로그램용 글. 틀린 도착(없는 id·자기 자신·겹침·구역 제목 줄)은 화면에서도 버린다(mdoc 이 뗀 뒤의 방어)
const SPEC = [
  ["h2", "결제", "", { to: "settle, 알림,없음,pay,settle,구역" }, "pay"], ["p", "a"],
  ["h2", "정산", "", null, "settle"], ["p", "b"],
  ["h2", "알림", "", { to: "pay" }, "알림"], ["p", "c"],
  ["h1", "구역", "", null, "구역"],
  ["h2", "요약", "", { to: "pay" }, "요약"], ["p", "d"],
];
const linksOf = T => T.querySelectorAll(".page-body.board-grid > svg.board-links");
{
  const T = new El("div");
  const r = build(SPEC, T);
  const grid = T.querySelector(".page-body.board-grid"), svg = linksOf(T)[0];
  eq("선 층: 격자 맨 끝에 하나", [linksOf(T).length, grid.lastElementChild === svg, svg && svg.ns], [1, true, SVGNS]);
  eq("선 층: 속성(읽는 프로그램에서 숨김·격자 크기)", svg && [svg.attrs["aria-hidden"], svg.attrs.focusable, svg.attrs.width, svg.attrs.height, svg.attrs.viewBox], ["true", "false", "430", "260", "0 0 430 260"]);
  eq("선 층: 선마다 출발·도착 id", svg.querySelectorAll("g.board-link").map(g => [g.attrs["data-board-from"], g.attrs["data-board-to"]]),
    [["pay", "settle"], ["pay", "알림"], ["알림", "pay"], ["요약", "pay"]]);
  eq("선 층: 선마다 선·촉 path", svg.querySelectorAll("g.board-link").map(g => g.children.map(c => c.className)), Array(4).fill(["board-link-line", "board-link-head"]));
  ok("선 층: 촉은 닫힌 path(marker 를 쓰지 않음)", svg.querySelectorAll(".board-link-head").every(p => / Z$/.test(p.attrs.d)) && !svg.querySelectorAll("marker").length);
  eq("선 층: id 가 없음(발표 복제에서 겹칠 것이 없다)", svg.all().filter(e => e.attrs.id).length + (svg.attrs.id ? 1 : 0), 0);
  eq("돌려주는 값: links", r.links, 4);
  // 타일: 결제 x0–100·정산 x110–210·알림 x220–320(y0–80), 요약 x0–100·y180–260(구역 줄 y90–170 아래). 결제 오른쪽 끝 셋(y 26·40·54 — 간격 14),
  // 알림 왼쪽 끝 둘(y 33·47). 결제 → 정산: 정산 쪽은 혼자라 26 에 맞춘 곧은 선. 결제 ⇄ 알림: 양쪽 모두 여럿이라 벌린 자리 그대로 — 출발 옆 틈에서 꺾고
  // 정산 밑을 지난다. 요약 → 결제: 구역을 넘어 위로 곧게(둘 다 혼자 — 겹침 가운데 x 50)
  const d = svg.querySelectorAll(".board-link-line").map(p => p.attrs.d);
  eq("선 층: 좌표(격자 안 px)", d, ["M100 26 L103 26", "M100 40 L101.3 40 L101.3 33 L213 33", "M220 47 L218.3 47 L218.3 54 L107 54", "M50 180 L50 87"]);
  const lbl = T.querySelectorAll(".tile-links-label").map(s => [s.parentElement.children[0].textContent, s.textContent]);
  eq("읽는 프로그램용 글: 출발 타일 머리에 「→ 도착 제목」", lbl, [["결제", "→ 정산, → 알림"], ["알림", "→ 결제"], ["요약", "→ 결제"]]);
  ok("읽는 프로그램용 글: 타일 머리 안(제목 뒤)", T.querySelectorAll(".tile-links-label").every(s => s.parentElement.cls.has("tile-head")));
  // 강조 짝 맞추기·자리는 to 와 상관없다 — slots 그대로
  eq("slots 그대로", T.querySelectorAll(".page-body.board-grid > .tile").map(t => t.dataset.boardAt), ["1,1", "2,1", "3,1", "1,1"]);
  // 같은 입력이면 같은 그림, 거듭 불러도 하나
  const T2 = new El("div");
  build(SPEC, T2);
  eq("결정적: 같은 입력에 같은 선 층", linksOf(T2)[0].dump(), svg.dump());
  const n = Bd.links(T.querySelector(".page.board"));
  eq("links 다시 부름: 선 층·글이 하나씩", [n, linksOf(T).length, T.querySelectorAll(".tile-links-label").length, linksOf(T)[0].dump() === svg.dump()], [4, 1, 3, true]);
}

// ---- 문서가 제목에 단 class tile-links-label 은 links 가 만든 글이 아니다(리뷰 1회차 [1]) — 연결이 있어도 제목은 남고 숨긴 글만 하나씩 붙었다 떨어진다
{
  const T = new El("div");
  build([["h2", "가", "tile-links-label", { to: "나" }, "가"], ["p", "a"], ["h2", "나", "tile-links-label", null, "나"], ["p", "b"]], T);
  const heads = () => T.querySelectorAll(".page-body.board-grid > .tile > .tile-head").map(h => h.children.map(c => c.tagName.toLowerCase() + ":" + c.textContent));
  eq("자유 class 제목: 제목이 남고 숨긴 글은 출발 타일에 하나", heads(), [["h2:가", "span:→ 나"], ["h2:나"]]);
  Bd.links(T.querySelector(".page.board"));
  eq("자유 class 제목: links 다시 불러도 제목은 그대로", heads(), [["h2:가", "span:→ 나"], ["h2:나"]]);
}

// ---- 같은 id 의 타일이 둘이면(직접 적은 id 의 겹침 — mdoc 은 duplicate-id 경고와 함께 둘 다 남긴다) 도착은 읽는 차례로 첫 타일이다.
// 둘째 타일이 같은 id 로 to 를 적으면 자기 id 와 같은 도착이라 버린다(mdoc boardLinks 의 「자기 자신」 과 같다)
{
  const T = new El("div");
  build([["h2", "가", "", null, "same"], ["p", "a"], ["h2", "나", "", { to: "same" }, "same"], ["p", "b"], ["h2", "다", "", { to: "same" }, "다"], ["p", "c"]], T);
  eq("같은 id 둘: 다 → 첫 타일(가)만, 나 → same 은 버림", linksOf(T)[0].querySelectorAll("g.board-link").map(g => [g.attrs["data-board-from"], g.attrs["data-board-to"]]), [["다", "same"]]);
  eq("같은 id 둘: 화살촉 끝은 첫 타일(가, x 0–100)의 오른쪽 가장자리", linksOf(T)[0].querySelector(".board-link-head").attrs.d.startsWith("M100 "), true);
}

// ---- 틈 0 인 격자: 선만 있고 촉 path 가 없다
{
  const keep = ctx.getComputedStyle;
  ctx.getComputedStyle = () => ({ fontSize: "12px", columnGap: "0px" });
  const T = new El("div");
  build([["h2", "가", "", { to: "다" }, "가"], ["p", "a"], ["h2", "나", "", null, "나"], ["p", "b"], ["h2", "다", "", null, "다"], ["p", "c"]], T);
  ctx.getComputedStyle = keep;
  eq("틈 0: 선 path 만(촉 없음)", linksOf(T)[0].querySelectorAll("g.board-link").map(g => g.children.map(c => c.className)), [["board-link-line"]]);
}

// ---- 크게 보기 동안 선 층은 숨는다(다른 타일과 같은 tile-under — base.css @media screen), 닫으면 돌아온다
{
  const T = new El("div");
  build(SPEC, T);
  const win = { addEventListener() {}, getSelection: () => null };
  T.ownerDocument = { defaultView: win };
  const z = Bd.zoomer(T, {});
  const tile = T.querySelectorAll(".page-body.board-grid > .tile")[1];
  z.open(tile);
  eq("크게 보기: 선 층에 tile-under", linksOf(T)[0].cls.has("tile-under"), true);
  z.close();
  eq("크게 보기 닫음: tile-under 떨어짐", linksOf(T)[0].cls.has("tile-under"), false);
}

// ---- 발표 복제(present.js startBoard 와 같은 차례 — 복제하고 [id] 를 모두 지운다): 선 층이 따라가고 id 가 없다
{
  const T = new El("div");
  build(SPEC, T);
  const page = T.querySelector(".page.board"), c = page.cloneNode(true);
  for (const x of c.querySelectorAll("[id]")) x.removeAttribute("id");
  const cs = c.querySelector(".page-body.board-grid > svg.board-links");
  eq("발표 복제: 선 층이 따라감(같은 그림)", cs && cs.dump(), linksOf(T)[0].dump());
  eq("발표 복제: 복제본에 id 가 없음", c.all().filter(e => e.attrs.id).length, 0);
  eq("발표 복제: 원본 선 층에도 id·id 참조가 없음(겹칠 것이 없다)", linksOf(T)[0].all().concat(linksOf(T)).filter(e => e.attrs.id || Object.values(e.attrs).some(v => /url\(#/.test(v))).length, 0);
  eq("발표 복제: 원본은 그대로", page.querySelectorAll("[id]").length > 0, true);
}

console.log("LINKS DONE cases=" + cases + " failed=" + failed);
