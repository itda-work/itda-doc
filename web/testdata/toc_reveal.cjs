// 목차 따라 굴리기(itda-hyve#991 — share.js tocCover·tocRevealTop·tocScroller·tocReveal) 시험. Go 시험(toc_reveal_test.go)이
// pager.js·slides.js·share.js 경로를 넘긴다. 사각형은 가짜 DOM 이 준 값으로 계산만 본다 — 실제 배치·굴림은 브라우저로 본다.
"use strict";
const fs = require("fs");
const vm = require("vm");
const files = process.argv.slice(2);
const ctx = { console };
ctx.window = ctx;
vm.createContext(ctx);
for (const f of files) vm.runInContext(fs.readFileSync(f, "utf8"), ctx);
const S = ctx.window.ItdaShare;
let cases = 0, failed = 0;
function ok(name, cond, detail) {
  cases++;
  if (!cond) { failed++; console.log("실패 " + name + " — " + JSON.stringify(detail === undefined ? null : detail)); } else console.log("통과 " + name);
}
const eq = (name, got, want) => ok(name, JSON.stringify(got) === JSON.stringify(want), { got, want });
const plain = x => JSON.parse(JSON.stringify(x));

// 상자: 화면 y 100~600 (높이 500)
const box = { top: 100, bottom: 600 };
const none = { top: 0, bottom: 0 };

// 1. tocCover — 아래 반이면 아래, 위 반이면 위, 상자 밖·높이 없음은 빼고, 여럿이면 큰 것
eq("가림: 없음", plain(S.tocCover(box, [])), { top: 0, bottom: 0 });
eq("가림: 바닥 저작권 줄(560~600) → 아래 40", plain(S.tocCover(box, [{ top: 560, bottom: 600 }])), { top: 0, bottom: 40 });
eq("가림: 머리(100~130) → 위 30", plain(S.tocCover(box, [{ top: 100, bottom: 130 }])), { top: 30, bottom: 0 });
eq("가림: 상자 밖·높이 0 은 세지 않는다", plain(S.tocCover(box, [{ top: 600, bottom: 640 }, { top: 40, bottom: 100 }, { top: 300, bottom: 300 }, null])), { top: 0, bottom: 0 });
eq("가림: 상자 밖으로 삐져나간 것은 상자 안 몫만", plain(S.tocCover(box, [{ top: 570, bottom: 624 }])), { top: 0, bottom: 30 });
eq("가림: 위·아래 둘 다, 같은 쪽은 큰 것", plain(S.tocCover(box, [{ top: 100, bottom: 120 }, { top: 550, bottom: 600 }, { top: 580, bottom: 600 }])), { top: 20, bottom: 50 });

// 2. tocRevealTop — 이미 들면 그대로, 위·아래로 넘치면 맞춤, 가림을 뺀다, 큰 항목은 위에, 0 아래로 내리지 않음
eq("굴림: 보이는 안이면 그대로", S.tocRevealTop({ top: 300, bottom: 320 }, box, none, 50), 50);
eq("굴림: 여유 24px 경계 안쪽 끝도 그대로", [S.tocRevealTop({ top: 124, bottom: 144 }, box, none, 50), S.tocRevealTop({ top: 556, bottom: 576 }, box, none, 50)], [50, 50]);
eq("굴림: 아래로 넘침 → 항목 아래가 영역 아래 - 24", S.tocRevealTop({ top: 700, bottom: 720 }, box, none, 50), 50 + 720 - 576);
eq("굴림: 위로 넘침 → 항목 위가 영역 위 + 24", S.tocRevealTop({ top: 20, bottom: 40 }, box, none, 300), 300 - (124 - 20));
eq("굴림: 저작권 줄 40 에 가린 항목(540~560 — 가림이 없으면 보이는 자리)도 굴린다", S.tocRevealTop({ top: 540, bottom: 560 }, box, { top: 0, bottom: 40 }, 0), 560 - 536);
eq("굴림: 저작권 줄 위로 들면 그대로", S.tocRevealTop({ top: 500, bottom: 520 }, box, { top: 0, bottom: 40 }, 10), 10);
eq("굴림: sticky 머리 30 아래에 가린 항목", S.tocRevealTop({ top: 130, bottom: 150 }, box, { top: 30, bottom: 0 }, 200), 200 - (154 - 130));
eq("굴림: 영역보다 높은 항목은 위에 맞춘다", S.tocRevealTop({ top: 700, bottom: 1300 }, box, none, 0), 700 - 124);
eq("굴림: 0 아래로는 내리지 않는다", S.tocRevealTop({ top: 20, bottom: 40 }, box, none, 30), 0);
eq("굴림: 여유를 따로 줄 수 있다", S.tocRevealTop({ top: 590, bottom: 610 }, box, none, 0, 0), 10);
eq("여유 기본값 24(문서 창 tocReveal 과 같다)", S.TOC_REVEAL_PAD, 24);

// 3. 가짜 DOM — tocScroller·tocReveal
function el(o) {
  const e = Object.assign({ hidden: false, children: [], scrollTop: 0, style: {}, rect: { top: 0, bottom: 0 }, shown: true }, o);
  e.getBoundingClientRect = () => {
    const r = typeof e.rect === "function" ? e.rect() : e.rect;
    return { top: r.top, bottom: r.bottom };
  };
  e.getClientRects = () => (e.shown ? [e.getBoundingClientRect()] : []);
  e.querySelector = sel => (e.q && e.q[sel]) || null;
  return e;
}
const win = { getComputedStyle: e => ({ overflowY: e.style.overflowY || "visible", position: e.style.position || "static" }) };
const style = e => win.getComputedStyle(e);

function scene(opts) {
  const o = opts || {};
  const side = el({ rect: { top: 100, bottom: 600 } });
  const nav = el({ rect: { top: 100, bottom: 2000 } });
  // 항목은 문서 좌표 base 에 있고 굴린 만큼 위로 간다(굴리는 상자의 scrollTop)
  const mk = (base, h) => el({ rect: () => { const s = o.navScrolls ? nav.scrollTop : side.scrollTop; return { top: base - s, bottom: base - s + h }; } });
  const on = mk(900, 20), within = mk(400, 20);
  nav.q = { "a.on": on };
  if (o.within) nav.q["a.on-within"] = within;
  if (o.navScrolls) { nav.style.overflowY = "auto"; nav.rect = { top: 100, bottom: 460 }; }
  const foot = el({ style: { position: "sticky" }, rect: { top: 560, bottom: 624 } });
  side.children = [el({ rect: { top: 100, bottom: 120 } }), nav].concat(o.foot ? [foot] : []);
  return { side, nav, on, within, foot };
}
eq("상자: 목차가 굴러가지 않으면 사이드바, 굴러가면(첨부가 있는 사이드바) 목차", (() => {
  const a = scene(), b = scene({ navScrolls: true });
  return [S.tocScroller(a.nav, a.side, style) === a.side, S.tocScroller(b.nav, b.side, style) === b.nav, S.tocScroller(null, a.side, style) === a.side];
})(), [true, true, true]);
{
  const s = scene();
  ok("굴림: 아래로 넘친 .on 으로 사이드바만", S.tocReveal(s.nav, s.side, win) === true && s.side.scrollTop === 920 - 576 && s.nav.scrollTop === 0, [s.side.scrollTop, s.nav.scrollTop]);
  ok("굴림: 이미 보이면 다시 불러도 굴리지 않는다", S.tocReveal(s.nav, s.side, win) === false && s.side.scrollTop === 344, s.side.scrollTop);
}
{
  const s = scene({ foot: true });
  S.tocReveal(s.nav, s.side, win);
  const r = s.on.getBoundingClientRect();
  ok("굴림: sticky 저작권 줄(560~ — 상자 안 40)을 피해 그 위 24px 에", s.side.scrollTop === 920 - (600 - 40 - 24) && r.bottom === 536, [s.side.scrollTop, r]);
}
{
  const s = scene({ within: true });
  ok("굴림: 접힌 가지 안이면 보이는 조상(.on-within — 400 은 이미 보인다)으로 — 굴리지 않는다", S.tocReveal(s.nav, s.side, win) === false && s.side.scrollTop === 0, s.side.scrollTop);
  const t = scene({ within: true });
  t.within.rect = () => ({ top: 1500 - t.side.scrollTop, bottom: 1520 - t.side.scrollTop });
  S.tocReveal(t.nav, t.side, win);
  ok("굴림: .on-within 이 넘치면 그것에 맞춘다(.on 이 아니라)", t.side.scrollTop === 1520 - 576, t.side.scrollTop);
}
{
  const s = scene({ navScrolls: true });
  S.tocReveal(s.nav, s.side, win);
  ok("굴림: 목차가 굴러가는 꼴이면 목차만(사이드바 그대로)", s.nav.scrollTop === 920 - (460 - 24) && s.side.scrollTop === 0, [s.nav.scrollTop, s.side.scrollTop]);
}
{
  const a = scene(); a.side.hidden = true;
  const b = scene(); b.on.shown = false; // 서랍이 닫힘·no-toc(display none)
  const c = scene(); c.nav.q = {};
  eq("굴림: 사이드바 숨김·항목이 안 보임·강조 없음·nav 없음이면 아무것도 하지 않는다",
    [S.tocReveal(a.nav, a.side, win), a.side.scrollTop, S.tocReveal(b.nav, b.side, win), b.side.scrollTop, S.tocReveal(c.nav, c.side, win), S.tocReveal(null, c.side, win)],
    [false, 0, false, 0, false, false]);
}

ok("tocReveal 은 scrollIntoView 를 쓰지 않는다", !/scrollIntoView/.test(S.tocReveal.toString()) && /box\.scrollTop = next/.test(S.tocReveal.toString()));

// 4. tocRevealer — 언제 굴리나(상태 전이). 가짜 rAF 큐: flush() 가 다음 그림 차례를 돌린다.
function rig() {
  let next = 1;
  const q = new Map(), runs = [];
  const raf = f => { const id = next++; q.set(id, f); return id; };
  const caf = id => q.delete(id);
  const flush = () => { const fs = [...q.values()]; q.clear(); for (const f of fs) f(); };
  let tag = "";
  const r = S.tocRevealer(raf, caf, () => runs.push(tag));
  return { r, runs, flush, q, set: t => { tag = t; } };
}
{
  const t = rig();
  t.r.doc("A"); t.set("a1"); t.r.see("a1");
  eq("때: 처음 지금 항목 — 예약 하나", [t.r.pending(), t.q.size], [true, 1]);
  t.r.see("a1"); t.r.see("a2");
  eq("때: 예약이 남은 동안 또 바뀌어도 예약은 하나", t.q.size, 1);
  t.set("a2"); t.flush();
  eq("때: 다음 그림 차례에 한 번 굴린다", [t.runs, t.r.pending()], [["a2"], false]);
  t.r.see("a2"); t.r.see("a2"); t.flush();
  eq("때: 지금 항목이 그대로면 굴리지 않는다(본문이 조금 움직임)", t.runs, ["a2"]);
  t.r.see(""); t.flush();
  eq("때: 빈 항목(목차 없음)은 굴리지 않는다", t.runs, ["a2"]);
  t.set("a3"); t.r.see("a3"); t.flush();
  eq("때: 다시 바뀌면 굴린다", t.runs, ["a2", "a3"]);
}
{
  // 리뷰 1회차 [1]: A 의 지금 항목 id 와 B 의 지금 항목 id 가 같아도 B 로 옮기면 맞춘다. 같은 문서 다시 그리기는 그대로.
  const t = rig();
  t.r.doc("A"); t.set("A"); t.r.see("same"); t.flush();
  t.r.doc("A"); t.r.see("same"); t.flush();
  eq("문서: 같은 문서를 다시 그리면(같은 열쇠) 같은 항목은 굴리지 않는다", t.runs, ["A"]);
  t.r.doc("B"); t.set("B"); t.r.see("same");
  eq("문서: 다른 문서로 옮기면 같은 id 여도 첫 항목에서 예약", t.r.pending(), true);
  t.flush();
  t.r.see("same"); t.flush();
  eq("문서: 옮긴 뒤 한 번 맞추고, 같은 제목 안에서는 다시 굴리지 않는다", t.runs, ["A", "B"]);
  t.r.doc("C"); t.r.see(""); t.r.doc("B"); t.set("B2"); t.r.see("same"); t.flush();
  eq("문서: 오류·제목 없는 문서(C)를 거쳐 돌아와도 맞춘다", t.runs, ["A", "B", "B2"]);
  const v = rig(), o1 = {}, o2 = {};
  v.r.doc(o1); v.r.see("x"); v.flush(); v.r.doc(o1); v.r.see("x"); v.flush(); v.r.doc(o2); v.r.see("x"); v.flush();
  eq("문서: 열쇠가 객체(웹 뷰어의 current())여도 같은 객체는 같은 문서", v.runs.length, 2);
}
{
  // 리뷰 1회차 [2]: 예약 → 사람이 목차를 만짐 → 예약 취소. 그 다음 지금 항목이 바뀌면 다시 굴린다.
  const t = rig();
  t.r.doc("A"); t.set("x"); t.r.see("x");
  t.r.cancel();
  t.flush();
  eq("사람: 예약 뒤 목차를 만지면 그 예약은 돌지 않는다", [t.runs, t.r.pending(), t.q.size], [[], false, 0]);
  t.r.see("x"); t.flush();
  eq("사람: 취소 뒤 같은 항목이면 다시 예약하지 않는다(사람이 굴린 자리 그대로)", t.runs, []);
  t.set("y"); t.r.see("y"); t.flush();
  eq("사람: 그 다음 지금 항목이 바뀌면 굴린다", t.runs, ["y"]);
  t.r.cancel();
  eq("사람: 예약이 없을 때 취소해도 아무 일 없다", [t.r.pending(), t.runs], [false, ["y"]]);
}
{
  const t = rig();
  t.r.doc("A"); t.r.see("x"); t.flush();
  t.set("open"); t.r.open(); t.r.open(); t.flush();
  eq("열기: 목차를 열면 같은 항목이어도 한 번 맞춘다(두 번 눌러도 예약 하나)", t.runs, ["", "open"]);
  t.r.open(); t.r.cancel(); t.flush();
  eq("열기: 연 직후 만지면 그 맞춤도 버린다", t.runs, ["", "open"]);
}
eq("사람 조작 이벤트: 휠·터치·누름·키(scroll 은 빼다 — 자동 굴림도 낸다)", [...S.TOC_TOUCH], ["wheel", "touchstart", "pointerdown", "keydown"]);

// 5. tocDocKey — 원천에 따라 문서 열쇠
{
  const share = { foldKey: v => "/s/t/" + (v && v.doc ? "?doc=" + v.doc : "") };
  const cur = {};
  const viewer = { current: () => cur };
  eq("열쇠: 공유·게시는 foldKey(폴더 공유는 문서마다 다르다)", [S.tocDocKey(share, { doc: "a.md" }), S.tocDocKey(share, { doc: "b.md" }), S.tocDocKey(share, {})], ["/s/t/?doc=a.md", "/s/t/?doc=b.md", "/s/t/"]);
  ok("열쇠: 웹 뷰어는 current() 객체", S.tocDocKey(viewer, {}) === cur);
  eq("열쇠: 둘 다 없으면 v.doc", [S.tocDocKey({}, { doc: "z" }), S.tocDocKey(null, null) === undefined], ["z", true]);
}

console.log("TOCREVEAL DONE cases=" + cases + " failed=" + failed);
