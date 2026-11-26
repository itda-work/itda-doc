// dashboard.js 의 타일 상태(tileStatus·markStatus)와 움직임(motion·enter·enterDelays·tileKeys)을 node 로 돌린다(board_test.go TestBoardMotion). 인자: dashboard.js.
// build 를 작은 가짜 DOM 위에서 끝까지 돌린다 — 넘침 측정(scrollHeight)은 없어서 fitTile 은 아무것도 하지 않고, 수식 맞춤(ItdaPager.fitMath)은 빈 함수다.
// 실제 화면의 움직임(CSS animation)은 Chrome 헤드리스 캡처로 봤다(명세 dashboard.md 8장) — 여기서는 class·늦춤·기억만 본다.
"use strict";
const fs = require("fs"), vm = require("vm");

// ---- 가짜 DOM(선택자 해석은 board_zoom.cjs 와 같은 작은 꼴: 쉼표 목록, 겹 = 태그·.클래스·[속성], 결합자 자손·자식, :scope, :is() 펼침)
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
  if (c.tag && c.tag !== e.tagName) return false;
  return c.cls.every(x => e.cls.has(x)) && c.attrs.every(a => a in e.attrs);
}
function matchSteps(e, st, i, scope) {
  if (!one(e, st[i].c, scope)) return false;
  if (i === st.length - 1) return true;
  if (st[i].comb === ">") return !!e.parentElement && matchSteps(e.parentElement, st, i + 1, scope);
  for (let a = e.parentElement; a; a = a.parentElement) if (matchSteps(a, st, i + 1, scope)) return true;
  return false;
}
class El {
  constructor(tag, cls) {
    this.tagName = tag.toUpperCase(); this.cls = new Set((cls || "").split(" ").filter(Boolean)); this.children = []; this.parentElement = null;
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
  get textContent() { return this._text + this.children.map(c => c.textContent).join(""); }
  set textContent(v) { this._text = String(v); for (const c of [...this.children]) c.remove(); }
  get firstChild() { return this.children[0] || null; }
  get firstElementChild() { return this.children[0] || null; }
  appendChild(c) { if (c.parentElement) c.remove(); c.parentElement = this; this.children.push(c); return c; }
  insertBefore(c, ref) {
    if (c.parentElement) c.remove();
    const i = ref ? this.children.indexOf(ref) : -1;
    c.parentElement = this;
    if (i < 0) this.children.push(c); else this.children.splice(i, 0, c);
    return c;
  }
  remove() { const p = this.parentElement; if (p) p.children.splice(p.children.indexOf(this), 1); this.parentElement = null; }
  setAttribute(k, v) { this.attrs[k] = String(v); }
  getAttribute(k) { return k in this.attrs ? this.attrs[k] : null; }
  removeAttribute(k) { delete this.attrs[k]; }
  matches(sel, scope) { return pieces(sel).some(p => matchSteps(this, steps(p), 0, scope)); }
  closest(sel) { for (let e = this; e; e = e.parentElement) if (e.matches(sel)) return e; return null; }
  all() { return this.children.flatMap(c => [c, ...c.all()]); }
  querySelectorAll(sel) { return this.all().filter(e => e.matches(sel, this)); }
  querySelector(sel) { return this.querySelectorAll(sel)[0] || null; }
  addEventListener(t, fn) { (this.on[t] = this.on[t] || []).push(fn); }
  removeEventListener(t, fn) { this.on[t] = (this.on[t] || []).filter(f => f !== fn); }
  fire(t, target) { for (const fn of [...(this.on[t] || [])]) fn({ type: t, target: target || this }); }
  // 마크업 비교용 꼴: 태그.클래스[data·style 변수] (자식…) 글
  dump() {
    const d = Object.keys(this.dataset).sort().map(k => k + "=" + this.dataset[k]).join(",");
    const v = Object.keys(this.style.vars).sort().map(k => k + "=" + this.style.vars[k]).join(",");
    const extra = ["gridColumn", "gridRow", "gridTemplateColumns", "gridTemplateRows"].filter(k => this.style[k]).map(k => k + "=" + this.style[k]).join(",");
    return this.tagName.toLowerCase() + [...this.cls].sort().map(c => "." + c).join("") + (d ? "[" + d + "]" : "") + (v || extra ? "{" + [v, extra].filter(Boolean).join(",") + "}" : "") +
      (this._text ? JSON.stringify(this._text) : "") + (this.children.length ? "(" + this.children.map(c => c.dump()).join(" ") + ")" : "");
  }
}
const DOC = { createElement: tag => new El(tag) };
const timers = [];
let animStyle = null; // 계산 스타일의 animation-duration·delay 흉내(늦은 끝 타이머 시험) — 없으면 비움
const ctx = {
  document: DOC, performance: { now: () => 0 },
  setTimeout: (fn, ms) => { timers.push({ fn, ms, live: true }); return timers.length; },
  clearTimeout: id => { if (timers[id - 1]) timers[id - 1].live = false; },
  getComputedStyle: el => Object.assign({ fontSize: "12px" }, animStyle ? animStyle(el) : {}),
};
ctx.window = ctx;
ctx.window.ItdaSlides = { SIZE: { "16:9": [1280, 720], "4:3": [960, 720] } };
ctx.window.ItdaPager = { fitMath() {} };
vm.createContext(ctx);
vm.runInContext(fs.readFileSync(process.argv[2], "utf8"), ctx);
const Bd = ctx.window.ItdaBoard;
const runTimers = () => { for (const t of timers.splice(0)) if (t.live) t.fn(); };

let cases = 0, failed = 0;
function eq(name, got, want) {
  cases++;
  const g = JSON.stringify(got), w = JSON.stringify(want);
  if (g !== w) { failed++; console.log("FAIL " + name + "\n  got  " + g + "\n  want " + w); }
}

// flow 만들기: [["h1", "글", "cls"], ["h2", "제목", "cls", {at: "1,1"}], ["p", "글"], …]
function flow(spec) {
  const f = new El("div", "page-body doc-flow");
  for (const [tag, text, cls, data] of spec) {
    const e = f.appendChild(new El(tag, cls || ""));
    e.textContent = text;
    Object.assign(e.dataset, data || {});
  }
  return f;
}
// BEFORE 는 아래 「상태 없음」 입력을 이 변경 전 dashboard.js(main 26d80f8)로 그린 보드의 꼴(El.dump)이다 — motion: false
const BEFORE = "div.boards.r169{--page-h=720px,--page-w=1280px}(div.board.page[kind=board,page=1,slide=1,steps=0]{--board-rows=3}(div.board-deco header.board-head(div.bh-title\"보드\") div.board-grid.page-body[boardCols=4]{gridTemplateColumns=repeat(4, minmax(0, 1fr)),gridTemplateRows=auto var(--board-row-h) var(--board-row-h)}(div.board-section{gridColumn=1 / -1,gridRow=1}(h1.x\"구역\") section.tile[boardAt=2,1,boardFixed=,boardSeg=1,boardSpan=1x1]{gridColumn=2 / span 1,gridRow=2}(header.tile-head(h2.kpi-note[at=2,1]\"매출\") div.tile-body(p\"글\")) section.tile.w2[boardAt=1,1,boardSeg=2,boardSpan=2x1]{gridColumn=1 / span 2,gridRow=3}(header.tile-head(h2\"표\") div.tile-body(table\"칸\")) section.tile[boardAt=3,1,boardSeg=2,boardSpan=1x1]{gridColumn=3 / span 1,gridRow=3}(header.tile-head(h2\"글2\") div.tile-body(p\"끝\")))))";
const tiles = target => target.querySelectorAll(".page-body.board-grid > .tile");
const titleOf = t => { const h = t.querySelector(":scope > .tile-head > :is(h1, h2, h3, h4, h5, h6)"); return h ? h.textContent : ""; };
const build = (spec, target, o) => Bd.build(flow(spec), target, Object.assign({ level: 2, info: { title: "보드" } }, o || {}));

// ---- 상태: class 하나씩, 둘 이상이면 무거운 쪽, 타일 제목 아닌 곳은 무시, 읽는 프로그램용 글
{
  const T = new El("div");
  build([
    ["h1", "구역", "bad"],
    ["h2", "결제", "ok"], ["p", "a"],
    ["h2", "정산", "warn"], ["p", "b"],
    ["h2", "알림", "bad"], ["p", "c"],
    ["h2", "둘", "ok bad"], ["p", "d"],
    ["h2", "셋", "warn ok"], ["p", "e"],
    ["h2", "없음", "note"], ["p", "f"], ["h3", "낮은", "bad"], ["p", "g"],
  ], T, { motion: false });
  const ts = tiles(T);
  const st = t => ["ok", "warn", "bad"].filter(s => t.cls.has("tile-" + s));
  eq("상태: 타일마다 옮겨 적은 class", ts.map(st), [["ok"], ["warn"], ["bad"], ["bad"], ["warn"], []]);
  const mark = t => { const s = t.querySelector(":scope > .tile-head > .tile-status"); return s ? [s.className, s.textContent, t.querySelector(":scope > .tile-head").children[0] === s] : null; };
  eq("상태: 머리 맨 앞 표식 자리와 숨긴 글(정상·주의·위험)", ts.map(mark), [
    ["tile-status tile-status-ok", "정상", true], ["tile-status tile-status-warn", "주의", true], ["tile-status tile-status-bad", "위험", true],
    ["tile-status tile-status-bad", "위험", true], ["tile-status tile-status-warn", "주의", true], null]);
  eq("상태: 숨긴 글은 .tile-status-label 하나", ts.slice(0, 3).map(t => t.querySelectorAll(".tile-status-label").map(x => x.textContent)), [["정상"], ["주의"], ["위험"]]);
  eq("상태: 구역 제목 줄의 class 는 아무것도 하지 않음", [T.querySelectorAll(".board-section .tile-status").length, T.querySelectorAll(".board-section").length], [0, 1]);
  eq("상태: 타일 안 낮은 제목의 class 는 아무것도 하지 않음", [st(ts[5]), ts[5].querySelectorAll(".tile-status").length], [[], 0]);
  eq("상태: 제목의 원래 class 는 그대로(문서의 자유 어휘)", ts.slice(0, 4).map(t => t.querySelector(":scope > .tile-head > h2").className), ["ok", "warn", "bad", "ok bad"]);
  eq("tileStatus", [Bd.tileStatus(null), Bd.tileStatus(new El("h2", "")), Bd.tileStatus(new El("h2", "OK")), Bd.tileStatus(new El("h2", "ok warn bad"))], ["", "", "", "bad"]);
  // 상태가 붙어도 놓인 자리 data 속성은 그대로
  eq("상태: slots data 속성", ts.slice(0, 2).map(t => [t.dataset.boardSeg, t.dataset.boardAt, t.dataset.boardSpan]), [["1", "1,1", "1x1"], ["1", "2,1", "1x1"]]);
}

// ---- 상태 없는 문서의 마크업은 전과 같다 — 아래 꼴은 이 변경 전 dashboard.js(main 26d80f8)로 같은 입력을 그린 결과다
{
  const T = new El("div");
  build([["h1", "구역", "x"], ["h2", "매출", "kpi-note", { at: "2,1" }], ["p", "글"], ["hr", ""], ["h2", "표"], ["table", "칸"], ["h2", "글2"], ["p", "끝"]], T, { motion: false });
  eq("상태 없음: 보드 마크업이 전과 같음", T.dump(), BEFORE);
}

// ---- 움직임: 처음 그림에만 등장
const spec1 = [["h2", "매출"], ["p", "44억"], ["h2", "정산"], ["p", "지연 12분"], ["h2", "메모"], ["p", "하나"], ["h2", "메모"], ["p", "둘"], ["h2", "표", "", { at: "1,3" }], ["p", "칸"]];
const T1 = new El("div");
const r1 = build(spec1, T1);
const enterOf = T => tiles(T).map(t => [t.cls.has("tile-enter"), t.style.vars["--tile-enter-delay"] || null]);
eq("등장: 처음 그림 — 모든 타일에 등장 표지와 읽는 차례 늦춤", enterOf(T1), [[true, "0ms"], [true, "40ms"], [true, "80ms"], [true, "120ms"], [true, "160ms"]]);
eq("등장: 돌려주는 값", [r1.entered, r1.changed], [5, undefined]);
eq("등장: 강조 표지 없음", tiles(T1).filter(t => t.cls.has("tile-changed")).length, 0);

// 다시 그림: 정산 본문 바뀜, 표는 자리만 바뀜(at), 두 번째 메모 본문 바뀜, 새 타일 하나
T1.textContent = "";
const spec2 = [["h2", "매출"], ["p", "44억"], ["h2", "새 타일"], ["p", "새"], ["h2", "정산"], ["p", "지연 30분"], ["h2", "메모"], ["p", "하나"], ["h2", "메모"], ["p", "셋"], ["h2", "표", "", { at: "2,3" }], ["p", "칸"]];
const r2 = build(spec2, T1);
eq("다시 그림: 등장 없음", tiles(T1).filter(t => t.cls.has("tile-enter")).length, 0);
eq("다시 그림: 본문이 바뀐 타일만 강조(새 타일·자리만 바뀐 타일·같은 본문은 없음)", tiles(T1).map(t => [titleOf(t), t.cls.has("tile-changed")]),
  [["매출", false], ["새 타일", false], ["정산", true], ["메모", false], ["메모", true], ["표", false]]);
eq("다시 그림: 돌려주는 값", [r2.entered, r2.changed], [undefined, 2]);
// 넘침 처리가 본문을 바꿔도(「… n행 더」) 짝 맞추기 글은 넘침 전이다 — 같은 본문이면 강조 없음
T1.textContent = "";
const r3 = build(spec2, T1);
eq("같은 문서를 다시 그림: 강조 없음", [r3.entered, r3.changed, tiles(T1).filter(t => t.cls.has("tile-changed") || t.cls.has("tile-enter")).length], [undefined, undefined, 0]);

// motion: false — 표지 없음(처음 그림이어도), 기억은 남는다
{
  const T = new El("div");
  const r = build(spec1, T, { motion: false });
  eq("motion false: 처음 그림에 표지 없음", [r.entered, tiles(T).filter(t => t.cls.has("tile-enter") || "--tile-enter-delay" in t.style.vars).length], [undefined, 0]);
  T.textContent = "";
  const rr = build(spec2, T, { motion: false });
  eq("motion false: 다시 그림에 강조 없음", [rr.changed, tiles(T).filter(t => t.cls.has("tile-changed")).length], [undefined, 0]);
  T.textContent = "";
  const r4 = build(spec1, T);
  eq("motion false 로 그린 뒤 켜서 다시 그림: 앞 그림으로 보고 바뀐 타일만 강조", [r4.entered, r4.changed], [undefined, 2]);
}

// motionScope: 그릴 때마다 새 target 을 만드는 호스트 — 늘 남는 요소에 기억을 묶는다
{
  const keep = new El("div");
  const ra = build(spec1, new El("div"), { motionScope: keep });
  const rb = build(spec2, new El("div"), { motionScope: keep });
  const rc = build(spec1, new El("div"));
  eq("motionScope: 둘째 그림은 다시 그림", [ra.entered, rb.entered, rb.changed, rc.entered], [5, undefined, 2, 5]);
}

// motionDoc: 한 자리에서 문서를 바꿔 그리는 호스트(공유 보는 화면) — 문서 열쇠가 바뀌면 처음 그림(등장), 다른 문서의 같은 제목 타일은 강조하지 않는다
{
  const keep = new El("div");
  const ra = build(spec1, new El("div"), { motionScope: keep, motionDoc: "a.md" });
  const rb = build(spec2, new El("div"), { motionScope: keep, motionDoc: "b.md" }); // 정산·메모 본문이 다르지만 다른 문서
  const Tb = new El("div");
  const rb2 = build(spec1, Tb, { motionScope: keep, motionDoc: "b.md" }); // 같은 문서 b 를 다시 — 앞 그림(spec2)과 견준다
  eq("motionDoc: 다른 문서는 처음 그림 — 등장, 강조 없음", [ra.entered, rb.entered, rb.changed], [5, 6, undefined]);
  eq("motionDoc: 같은 문서를 다시 그리면 바뀐 타일만 강조", [rb2.entered, rb2.changed, tiles(Tb).filter(t => t.cls.has("tile-changed")).map(titleOf)], [undefined, 2, ["정산", "메모"]]);
  const rc = build(spec1, new El("div"), { motionScope: keep, motionDoc: "a.md" });
  eq("motionDoc: 앞 문서로 돌아와도 처음 그림(기억은 마지막 그림 하나)", [rc.entered, rc.changed], [5, undefined]);
  const rd = build(spec2, new El("div"), { motionScope: keep }); // 열쇠 없음(undefined) ≠ "a.md"
  eq("motionDoc: 열쇠를 주던 자리에 열쇠 없이 그리면 다른 문서", [rd.entered, rd.changed], [6, undefined]);
  // enter: false — 처음 그림이어도 등장 표지 없음, 기억은 적고 다음 다시 그림의 강조는 그대로
  const kq = new El("div"), Tq = new El("div");
  const rq = build(spec1, Tq, { motionScope: kq, enter: false });
  const Tq2 = new El("div");
  const rq2 = build(spec2, Tq2, { motionScope: kq, enter: false });
  eq("enter: false — 등장만 끔, 강조는 그대로", [rq.entered, tiles(Tq).some(t => t.cls.has("tile-enter")), rq2.changed, tiles(Tq2).filter(t => t.cls.has("tile-changed")).map(titleOf)],
    [undefined, false, 2, ["정산", "메모"]]);
  eq("forget: 버린 것이 있으면 참, 다음 그림은 처음 그림", [Bd.forget(keep), Bd.forget(keep), build(spec2, new El("div"), { motionScope: keep }).entered], [true, false, 6]);
}

// ---- 표지가 끝난 뒤 떼어짐: animationend(그 타일이 대상일 때만)·animationcancel·늦은 끝(타이머)
{
  timers.length = 0;
  const T = new El("div");
  build(spec1, T);
  const [a, b, c] = tiles(T);
  a.fire("animationend", a.querySelector(".tile-body")); // 자식에서 올라온 끝은 무시
  eq("떼기: 자식의 animationend 는 무시", a.cls.has("tile-enter"), true);
  a.fire("animationend");
  b.fire("animationcancel");
  eq("떼기: animationend·animationcancel 뒤 class·늦춤 변수가 없다", [a, b].map(t => [t.cls.has("tile-enter"), "--tile-enter-delay" in t.style.vars]), [[false, false], [false, false]]);
  eq("떼기: 끝난 타일의 리스너도 뗀다", [(a.on.animationend || []).length, (a.on.animationcancel || []).length], [0, 0]);
  eq("떼기: 아직 안 끝난 타일은 그대로", c.cls.has("tile-enter"), true);
  runTimers();
  eq("떼기: 애니메이션이 돌지 않아도 늦은 끝에 모두 뗀다", tiles(T).filter(t => t.cls.has("tile-enter") || "--tile-enter-delay" in t.style.vars).length, 0);
  T.textContent = "";
  build(spec2, T);
  runTimers();
  eq("떼기: 강조 표지도 늦은 끝에 뗀다", tiles(T).filter(t => t.cls.has("tile-changed")).length, 0);
}
{
  timers.length = 0;
  const K = new El("div");
  build(spec1, new El("div"), { motionScope: K });
  build(spec2, new El("div"), { motionScope: K });
  eq("늦은 끝: 계산 스타일에 animation 이 없으면 최소값(등장 1.2초·강조 2초)", [...new Set(timers.map(t => t.ms))].sort(), [1200, 2000]);
  timers.length = 0;
}

// ---- 늦은 끝은 계산된 animation-duration + animation-delay 를 기다린다 — 테마가 움직임 토큰을 늘려도 도는 애니메이션을 자르지 않는다
{
  timers.length = 0;
  // 등장: --board-enter-time 2s·--board-enter-stagger 10(늦춤 0·0.4·0.8…초), 강조: --board-flash-time 3s, 쉼표 목록·ms 표기도 읽는다
  animStyle = el => el.cls.has("tile-changed") ? { animationDuration: "3s", animationDelay: "0s" }
    : { animationDuration: "2s, 250ms", animationDelay: (el.style.vars["--tile-enter-delay"] ? parseInt(el.style.vars["--tile-enter-delay"]) * 10 : 0) + "ms, 0s" };
  const K = new El("div");
  build(spec1, new El("div"), { motionScope: K });
  eq("늦은 끝: 등장 타이머 = 시간 + 늦춤 + 0.5초", timers.map(t => t.ms), [2500, 2900, 3300, 3700, 4100]);
  timers.length = 0;
  build(spec2, new El("div"), { motionScope: K });
  eq("늦은 끝: 강조 타이머 = 3초 + 0.5초", timers.map(t => t.ms), [3500, 3500]);
  timers.length = 0;
  animStyle = () => ({ animationDuration: "0.1s", animationDelay: "0s" });
  build(spec1, new El("div"));
  eq("늦은 끝: 계산 값이 짧으면 최소값", [...new Set(timers.map(t => t.ms))], [1200]);
  animStyle = null;
  timers.length = 0;
}

// ---- enter: 발표가 복제한 보드 — 남은 표지를 지우고 다시 건다
{
  const T = new El("div");
  build(spec1, T, { motion: false });
  const pg = T.querySelector(".page.board");
  const [a, b] = tiles(T);
  a.classList.add("tile-changed");
  b.classList.add("tile-enter"); b.style.setProperty("--tile-enter-delay", "999ms");
  eq("enter: 돌려주는 값은 타일 수", Bd.enter(pg), 5);
  eq("enter: 강조 표지를 지우고 등장을 다시 건다", enterOf(T).slice(0, 2).concat([[a.cls.has("tile-changed")]]), [[true, "0ms"], [true, "40ms"], [false]]);
}

// ---- 늦춤: 상한(0.3초)을 넘지 않는다 — 타일 100개
{
  const d = Bd.enterDelays(100);
  eq("enterDelays(100): 개수·첫 값·마지막 값 ≤ 300·늘기만 함", [d.length, d[0], Math.max(...d) <= 300, d.every((x, i) => !i || x >= d[i - 1])], [100, 0, true, true]);
  eq("enterDelays 작은 수", [Bd.enterDelays(0), Bd.enterDelays(1), Bd.enterDelays(3), Bd.enterDelays(9).at(-1)], [[], [0], [0, 40, 80], 300]);
  const spec = [];
  for (let i = 0; i < 100; i++) spec.push(["h2", "타일 " + i], ["p", "글"]);
  const T = new El("div");
  build(spec, T, { cols: 12, rows: 6 });
  const ms = tiles(T).map(t => parseFloat(t.style.vars["--tile-enter-delay"]));
  eq("build 타일 100개: 모두 등장, 가장 늦은 늦춤 ≤ 300ms", [ms.length, Math.max(...ms) <= 300], [100, true]);
}

// ---- 짝 맞추기 열쇠
eq("tileKeys: 제목 글 + 같은 제목 안 차례, 공백은 한 칸", Bd.tileKeys(["매출", " 매출 ", "", "메모  하나", ""]), ["매출#0", "매출#1", "#0", "메모 하나#0", "#1"]);

console.log("MOTION DONE cases=" + cases + " failed=" + failed);
