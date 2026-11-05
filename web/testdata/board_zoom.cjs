// dashboard.js 의 크게 보기(zoomHit·zoomStep·zoomKey·zoomer)를 node 로 돌린다(board_test.go TestBoardZoom). 인자: dashboard.js.
// zoomer 는 작은 가짜 DOM 위에서 돈다 — 선택자는 쉼표로 나눈 조각마다 마지막 겹(태그·클래스·[속성])만 본다(실제 화면 동작은 문서 창 selftest 가 본다).
"use strict";
const fs = require("fs"), vm = require("vm");

// ---- 가짜 DOM
// 선택자: 쉼표 목록, 겹(태그·.클래스·[속성], :not(...) 는 무시)과 결합자(자손 공백·자식 >), :scope, :is(a, b) 는 펼친다.
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
function steps(p) { // 오른쪽부터: [{c, comb}] — comb 는 왼쪽 겹과의 결합자
  const toks = p.replace(/\s*>\s*/g, " > ").trim().split(/\s+/);
  const out = [];
  let comb = " ";
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
  constructor(tag, cls) { this.dataset = {}; this.tagName = tag.toUpperCase(); this.cls = new Set((cls || "").split(" ").filter(Boolean)); this.children = []; this.parentElement = null; this.attrs = {}; this.style = { vars: {}, setProperty(k, v) { this.vars[k] = v; }, removeProperty(k) { delete this.vars[k]; } }; this.textContent = ""; }
  get classList() { const s = this.cls; return { add: c => s.add(c), remove: c => s.delete(c), contains: c => s.has(c), toggle: (c, on) => (on ? s.add(c) : s.delete(c)) }; }
  set className(v) { this.cls = new Set(v.split(" ").filter(Boolean)); }
  get isConnected() { let e = this; while (e.parentElement) e = e.parentElement; return e === ROOT_TOP; }
  get firstElementChild() { return this.children[0] || null; }
  appendChild(c) { if (c.parentElement) c.remove(); c.parentElement = this; this.children.push(c); return c; }
  remove() { const p = this.parentElement; if (p) p.children.splice(p.children.indexOf(this), 1); this.parentElement = null; }
  setAttribute(k, v) { this.attrs[k] = String(v); }
  getAttribute(k) { return k in this.attrs ? this.attrs[k] : null; }
  removeAttribute(k) { delete this.attrs[k]; if (k === "tabindex") delete this._ti; }
  set tabIndex(v) { this._ti = v; this.attrs.tabindex = String(v); }
  get tabIndex() { return this._ti === undefined ? -1 : this._ti; }
  set type(v) { this.attrs.type = v; }
  set title(v) { this.attrs.title = v; }
  matches(sel, scope) { return pieces(sel).some(p => matchSteps(this, steps(p), 0, scope)); }
  closest(sel) { for (let e = this; e; e = e.parentElement) if (e.matches(sel)) return e; return null; }
  contains(x) { for (let e = x; e; e = e.parentElement) if (e === this) return true; return false; }
  all() { return this.children.flatMap(c => [c, ...c.all()]); }
  querySelectorAll(sel) { return this.all().filter(e => e.matches(sel, this)); }
  querySelector(sel) { return this.querySelectorAll(sel)[0] || null; }
  getClientRects() { return this.cls.has("tile-zoom-btn") && this.closest(".tile-zoomed") ? [] : [1]; }
  focus() { DOC.activeElement = this; }
}
const listeners = { win: {}, root: {} };
const on = (box, type, fn) => { (box[type] = box[type] || []).push(fn); };
const WIN = { addEventListener: (t, fn) => on(listeners.win, t, fn), getSelection: () => SEL };
const DOC = { activeElement: null, defaultView: WIN, body: null, createElement: tag => new El(tag) };
let SEL = { isCollapsed: true, anchorNode: null, toString: () => "" };
const ROOT_TOP = new El("body");
DOC.body = ROOT_TOP;
const root = ROOT_TOP.appendChild(new El("div", "pages"));
root.ownerDocument = DOC;
root.addEventListener = (t, fn) => on(listeners.root, t, fn);

// 보드 한 장: 머리, 타일 셋(머리 있음·있음·없음), 구역 줄, 타일 하나
const PG = root.appendChild(new El("div", "page board"));
const HEAD = PG.appendChild(new El("header", "board-head"));
const GRID = PG.appendChild(new El("div", "page-body board-grid"));
const revealed = [];
function tileEl() { const t = GRID.appendChild(new El("section", "tile")); t.scrollIntoView = () => {}; return t; }
const rect = (left, top, right, bottom) => ({ left, top, right, bottom, width: right - left, height: bottom - top });
PG.getBoundingClientRect = () => rect(100, -300 - scrollerEl.top, 1100, 2000 - scrollerEl.top);
GRID.getBoundingClientRect = () => rect(130, -220 - scrollerEl.top, 1070, 1970 - scrollerEl.top);
PG.offsetWidth = 1000;
const scrollerEl = { top: 0, on: {}, addEventListener(t, fn, o) { this.on[t] = { fn, o }; }, getBoundingClientRect: () => rect(0, 50, 1200, 650) };
function head(t, title) { const h = t.appendChild(new El("header", "tile-head")); const h2 = h.appendChild(new El("h2")); h2.textContent = title; t.children.unshift(t.children.pop()); return h; }
function body(t) { return t.appendChild(new El("div", "tile-body")); }
const A = tileEl(), B = tileEl(), C = tileEl(), SECT = GRID.appendChild(new El("div", "board-section")), D = tileEl();
const hA = head(A, "매출"), bA = body(A), pA = bA.appendChild(new El("p")), linkA = pA.appendChild(new El("a")); linkA.setAttribute("href", "#x");
const hB = head(B, "이익"), linkHB = hB.children[0].appendChild(new El("a")); linkHB.setAttribute("href", "#note"); const bB = body(B); const tblB = bB.appendChild(new El("table")); const thB = tblB.appendChild(new El("th", "sortable"));
const bC = body(C); const svgC = bC.appendChild(new El("figure")).appendChild(new El("svg"));
head(D, "주요 수주"); body(D);

const ctx = { window: WIN, document: DOC, performance: { now: () => 0 } };
vm.createContext(ctx);
vm.runInContext(fs.readFileSync(process.argv[2], "utf8"), ctx);
const Bd = ctx.window.ItdaBoard;

let cases = 0, failed = 0;
function eq(name, got, want) {
  cases++;
  const g = JSON.stringify(got), w = JSON.stringify(want);
  if (g !== w) { failed++; console.log("FAIL " + name + "\n  got  " + g + "\n  want " + w); }
}

// ---- 순수 판단
eq("zoomStep: 앞", Bd.zoomStep(2, 1, 4), 3);
eq("zoomStep: 뒤", Bd.zoomStep(2, -1, 4), 1);
eq("zoomStep: 끝에서 멈춤", Bd.zoomStep(3, 1, 4), 3);
eq("zoomStep: 처음에서 멈춤", Bd.zoomStep(0, -1, 4), 0);
eq("zoomStep: 모름(-1)은 그대로", Bd.zoomStep(-1, 1, 4), -1);
const k = (key, o) => Bd.zoomKey(Object.assign({ key }, o || {}));
eq("zoomKey", [k("Escape"), k("ArrowLeft"), k("ArrowRight"), k("Tab"), k("Tab", { shiftKey: true }), k("PageDown"), k(" "), k("ArrowDown"), k("a"), k("Enter"),
  k("ArrowRight", { metaKey: true }), k("ArrowLeft", { altKey: true }), k("Escape", { isComposing: true })],
["close", "prev", "next", "trap", "trap", "hold", "hold", "hold", null, null, null, null, null]);

// ---- zoomer(가짜 DOM)
let blocked = false;
const z = Bd.zoomer(root, { blocked: () => blocked, scroller: scrollerEl, reveal: el => revealed.push({ el, zoomed: el.cls.has("tile-zoomed") }) });
z.arm();
z.arm(); // 두 번 불러도 단추는 하나
eq("arm: 타일마다 단추 하나", [A, B, C, D].map(t => t.querySelectorAll(":scope > .tile-zoom-btn").length), [1, 1, 1, 1]);
eq("arm: 단추 이름은 타일 제목(없으면 「크게 보기」)", [A, C].map(t => t.querySelector(":scope > .tile-zoom-btn").attrs["aria-label"]), ["크게 보기: 매출", "크게 보기"]);

function ev(type, target, o) {
  return Object.assign({ type, target, button: 0, clientX: 10, clientY: 10, defaultPrevented: false, stopped: false,
    preventDefault() { this.defaultPrevented = true; }, stopPropagation() { this.stopped = true; } }, o || {});
}
// click: 창 잡기 → (멈추지 않았으면) root 거품 — 실제 차례와 같다
function click(target, o) {
  const e = ev("click", target, o);
  if (!o || !o.noDown) for (const fn of listeners.root.pointerdown || []) fn(ev("pointerdown", target, { clientX: 10, clientY: 10 }));
  for (const fn of listeners.win.click || []) fn(e);
  if (!e.stopped) for (const fn of listeners.root.click || []) fn(e);
  return e;
}
function key(k, o) {
  const e = ev("keydown", DOC.activeElement, Object.assign({ key: k }, o || {}));
  for (const fn of listeners.win.keydown || []) fn(e);
  return e;
}
const NAMES = new Map([[A, "A"], [B, "B"], [C, "C"], [D, "D"]]);
const zoomed = () => [A, B, C, D].filter(t => t.cls.has("tile-zoomed")).map(t => NAMES.get(t));
const under = () => [A, B, C, D].filter(t => t.cls.has("tile-under")).map(t => NAMES.get(t));

// 열지 않는 누르기: 본문 글·링크·표 머리 칸·차트, 글자를 고른 채, 끌기
for (const [name, t, o] of [["본문 글", pA], ["링크", linkA], ["머리 안 링크", linkHB], ["표 머리 칸", thB], ["차트", svgC]]) {
  click(t, o);
  eq("안 열림: " + name, zoomed().length, 0);
}
SEL = { isCollapsed: false, anchorNode: pA, toString: () => "매출 글" };
click(bA);
eq("안 열림: 글자를 고른 채 빈 곳", zoomed().length, 0);
SEL = { isCollapsed: true, anchorNode: null, toString: () => "" };
{ // 끌기: 누른 자리와 뗀 자리가 4px 넘게 다르다
  for (const fn of listeners.root.pointerdown) fn(ev("pointerdown", bA, { clientX: 0, clientY: 0 }));
  const e = ev("click", bA, { clientX: 30, clientY: 0 });
  for (const fn of listeners.win.click) fn(e);
  for (const fn of listeners.root.click) fn(e);
  eq("안 열림: 끌기", zoomed().length, 0);
}

// 머리 누르기 → 열림(단추 초점·다른 타일·구역 줄·머리 숨김·대화 상자 속성·타일 보이기)
revealed.length = 0;
click(hA.children[0]);
eq("열림: 머리", zoomed(), ["A"]);
eq("열림: 다른 타일 모두 숨김", under(), ["B", "C", "D"]);
eq("열림: 구역 줄·머리도 숨김", [SECT.cls.has("tile-under"), HEAD.cls.has("tile-under")], [true, true]);
// 자리: 굴림 칸(y 50..650)과 보드(x 100..1100)의 겹침 — 격자 원점(130, -220)에서 여백 12, 확대 1(재서 얻음)
const box = () => ["left", "top", "width", "height"].map(n => A.style.vars["--tz-" + n]);
eq("열림: 보이는 영역 크기", box(), ["-18px", "282px", "976px", "576px"]);
eq("열림: 대화 상자·초점은 닫기 단추", [A.attrs.role, A.attrs["aria-modal"], A.attrs["aria-label"], DOC.activeElement && [...DOC.activeElement.cls][0]], ["dialog", "true", "매출", "tile-zoom-close"]);
eq("열림: 띄우기 전에 그 타일을 보이게 굴림", revealed.map(r => [r.el === A, r.zoomed]), [[true, false]]);
// 칸이 굴러가면(막대 끌기) 타일이 보이는 영역을 따라간다, 창 크기가 바뀌어도 다시 맞춘다
scrollerEl.top = 100;
scrollerEl.on.scroll.fn();
eq("굴림: 따라감", box()[1], "382px");
scrollerEl.top = 0;
for (const fn of listeners.win.resize) fn();
eq("창 크기: 다시 맞춤", box()[1], "282px");
// 바깥 굴림은 막고, 본문이 굴러갈 때 본문 안 굴림만 둔다
const wheel = target => { const e = ev("wheel", target); scrollerEl.on.wheel.fn(e); return e.defaultPrevented; };
eq("굴림 막기: 잡기는 passive 아님", scrollerEl.on.wheel.o.passive, false);
eq("굴림: 바깥은 막음", wheel(B), true);
eq("굴림: 본문이 안 넘치면 막음", wheel(pA), true);
bA.scrollHeight = 900; bA.clientHeight = 300;
eq("굴림: 본문이 넘치면 그대로", wheel(pA), false);
eq("굴림: 머리 위 굴림은 막음", wheel(hA), true);
// 열린 동안 안쪽 누르기는 닫지 않는다
click(pA);
eq("열린 동안 안쪽 누르기는 그대로", zoomed(), ["A"]);
// Esc → 닫힘·초점은 원래 타일 단추·속성 원래대로
const esc = key("Escape");
eq("Esc: 닫힘", [zoomed().length, under().length, esc.defaultPrevented, esc.stopped], [0, 0, true, true]);
eq("Esc: 구역 줄·머리·자리 값이 돌아옴", [SECT.cls.has("tile-under"), HEAD.cls.has("tile-under"), Object.keys(A.style.vars)], [false, false, []]);
{ const e = ev("wheel", B); scrollerEl.on.wheel.fn(e); eq("닫힌 동안 굴림은 그대로", e.defaultPrevented, false); }
eq("Esc: 속성이 전과 같다", [A.attrs.role, A.attrs["aria-modal"], A.attrs["aria-label"], bA.attrs.tabindex, A.querySelectorAll(":scope > .tile-zoom-close").length], [undefined, undefined, undefined, undefined, 0]);
eq("Esc: 초점은 원래 타일", DOC.activeElement === A.querySelector(":scope > .tile-zoom-btn"), true);
// 닫힌 동안의 키는 건드리지 않는다
const k0 = key("ArrowRight");
eq("닫힌 동안 → 는 호스트 것", [k0.defaultPrevented, k0.stopped], [false, false]);

// 빈 곳(본문 상자) → 열림, → → → 는 구역 줄을 건너 D 에서 멈춤, ← 로 돌아감
click(bB);
eq("열림: 본문 빈 곳", zoomed(), ["B"]);
key("ArrowRight");
eq("→: 다음 타일(머리 없는 타일)", zoomed(), ["C"]);
revealed.length = 0;
key("ArrowRight");
eq("→: 구역 줄을 건너감·그 타일을 보이게 굴림(닫으면 그 자리)", [zoomed(), under(), revealed.map(r => [r.el === D, r.zoomed])], [["D"], ["A", "B", "C"], [[true, false]]]);
const kEnd = key("ArrowRight");
eq("→: 끝에서 멈춤", [zoomed(), kEnd.defaultPrevented], [["D"], true]);
key("ArrowLeft");
eq("←: 앞 타일", zoomed(), ["C"]);
// 굴림 키는 보드를 굴리지 않게 막는다(초점이 본문 밖)
const pd = key("PageDown");
eq("PageDown: 막힘", [pd.defaultPrevented, pd.stopped, zoomed()], [true, true, ["C"]]);
// Tab: 마지막에서 처음으로(밖으로 나가지 않음) — C 의 초점 갈 곳은 본문(tabindex 0)·닫기 단추
const x = C.querySelector(":scope > .tile-zoom-close");
x.focus();
const tab = key("Tab");
eq("Tab: 안에서 돎", [tab.defaultPrevented, DOC.activeElement === bC], [true, true]);
// 닫기 단추 → 닫힘
click(C.querySelector(":scope > .tile-zoom-close"));
eq("닫기 단추: 닫힘·초점 복귀", [zoomed().length, DOC.activeElement === C.querySelector(":scope > .tile-zoom-btn")], [0, true]);

// 바깥 누르기 → 닫힘, 그 누르기로 다른 타일이 열리지 않는다
click(A.querySelector(":scope > .tile-zoom-btn"));
eq("단추: 열림", zoomed(), ["A"]);
click(D.querySelector(":scope > .tile-body"));
eq("바깥 누르기: 닫기만(다른 타일을 열지 않음)", zoomed().length, 0);

// 발표 중(blocked)에는 열지 않고, 열린 동안 발표가 시작되면 다음 확인(current·키)에서 닫는다
blocked = true;
click(hA.children[0]);
eq("발표 중: 안 열림", zoomed().length, 0);
blocked = false;
click(hA.children[0]);
blocked = true;
eq("발표 시작: current() 가 닫음", [z.current(), zoomed().length], [null, 0]);
blocked = false;

// 다시 그리기: 열린 타일이 떨어지면 열린 상태를 버린다(이후 키는 호스트 것)
click(hA.children[0]);
A.remove();
const kr = key("ArrowRight");
eq("다시 그림: 떨어진 타일은 버림", [z.current(), kr.stopped], [null, false]);
// 다시 그린 뒤 키·누르기가 오기 전에 휠이 와도 굴림이 막히지 않는다(열린 상태를 그 자리에서 버린다) — 타일을 되돌려 뒤 시험에 영향이 없게
{
  const wheel = () => { const e = { target: GRID, prevented: false, preventDefault() { this.prevented = true; } }; scrollerEl.on.wheel.fn(e); return e.prevented; };
  GRID.appendChild(A);
  click(hA.children[0]);
  const open = wheel();
  A.remove();
  eq("다시 그림: 열린 동안은 막고, 떨어지면 다음 휠부터 굴러간다", [open, wheel(), z.current()], [true, false, null]);
  GRID.appendChild(A);
  click(hA.children[0]);
  A.remove();
  scrollerEl.on.scroll.fn();
  eq("다시 그림: 떨어진 뒤 굴림(fit)도 열린 상태를 버린다", z.current(), null);
}

// YouTube 플레이어(호스트 부품)가 다른 타일에 있으면 크게 보기를 열 때 멈춘다 — 가려진 타일에서 소리만 남지 않게. 그 타일 안이면 그대로
{
  let stops = 0;
  GRID.appendChild(A); // 위 다시 그리기 시험이 떼어 둔 타일
  WIN.ItdaYT = { stop() { stops++; } };
  const box = bA.appendChild(new El("span", "yt-play"));
  click(hA.children[0]);
  eq("YouTube: 재생 타일을 크게 보면 멈추지 않는다", [z.current() === A, stops], [true, 0]);
  key("Escape");
  click(hB.children[0]);
  eq("YouTube: 다른 타일을 크게 보면 멈춘다", [z.current() === B, stops], [true, 1]);
  key("Escape");
  box.remove();
  click(hA.children[0]);
  eq("YouTube: 플레이어가 없으면 부르지 않는다", stops, 1);
  key("Escape");
  delete WIN.ItdaYT;
}

// ---- 빌드(build): 판 없이 한 장 — 구역 줄·줄 나눔·머리 제목은 groups·pack 을 지나 격자 자리로 나타난다
{
  ctx.window.ItdaSlides = { SIZE: { "16:9": [1280, 720], "4:3": [960, 720] } };
  const mk = (tag, text, id) => { const e = new El(tag); e.textContent = text; if (id) e.id = id; return e; };
  const tiles = t => t.querySelectorAll(".tile").map(x => [x.querySelector("h2").textContent, x.style.gridRow, x.style.gridColumn]);
  const flow = new El("div");
  [mk("h1", "첫 구역", "first"), mk("h2", "매출"), mk("p", "글"), new El("hr"), mk("h2", "이익"), mk("h1", "둘째 구역", "second"), mk("h2", "비용"), mk("h2", "인력"), new El("hr"), new El("hr"), mk("h2", "일정")].forEach(e => flow.appendChild(e));
  const t = new El("div");
  const r = Bd.build(flow, t, { ratio: "16:9", cols: 4, rows: 2, level: 2 });
  eq("빌드: 보드는 한 장이고 돌려주는 값에 판 수가 없다", [t.querySelectorAll(".page.board").length, Object.keys(r).sort()], [1, ["ms", "overflow"]]);
  eq("빌드: 판 번호·rows 는 줄 높이 기준으로", [t.querySelectorAll(".bh-num").length, t.querySelector(".page.board").style.vars["--board-rows"]], [0, "2"]);
  eq("빌드: 타일 앞 첫 높은 제목은 머리 제목(제목 요소 그대로 — id 유지), 구역 줄이 되지 않는다", [t.querySelector(".bh-title h1").id, t.querySelectorAll(".board-section").length], ["first", 1]);
  const sec = t.querySelector(".board-section");
  eq("빌드: 그 밖의 높은 제목은 전체 폭 구역 줄(id 유지)", [sec.querySelector("h1").id, sec.style.gridColumn, sec.style.gridRow], ["second", "1 / -1", "3"]);
  eq("빌드: --- 는 다음 줄부터, 구역 줄은 줄 하나, 연달은 --- 는 줄을 더 만들지 않는다", tiles(t), [["매출", "1", "1 / span 1"], ["이익", "2", "1 / span 1"], ["비용", "4", "1 / span 1"], ["인력", "4", "2 / span 1"], ["일정", "5", "1 / span 1"]]);
  eq("빌드: 타일 줄은 고정 높이, 구역 줄은 auto", t.querySelector(".board-grid").style.gridTemplateRows, ["var(--board-row-h)", "var(--board-row-h)", "auto", "var(--board-row-h)", "var(--board-row-h)"].join(" "));
  const f2 = new El("div"); [mk("h1", "문서 제목 앞 구역", "a"), mk("h2", "타일")].forEach(e => f2.appendChild(e));
  const t2 = new El("div");
  Bd.build(f2, t2, { ratio: "16:9", cols: 4, rows: 3, level: 2, info: { title: "문서 제목" } });
  eq("빌드: 문서 제목이 있으면 머리는 그것, 첫 # 는 구역 줄", [t2.querySelector(".bh-title").textContent, t2.querySelectorAll(".board-section").length], ["문서 제목", 1]);
}

console.log("ZOOM DONE cases=" + cases + " failed=" + failed);
