// 노트 칸 배치(slides.js placeNotes·notePlacement·noteAfter·layoutNotes·watchNotes, itda-hyve#752)를 작은 가짜 DOM 으로 돌린다. 쪽 rect 는 쪽 높이 × 배율에 앞 쪽들의
// --sn-after(× 배율)를 더한 값이라, 여백을 걸기 전에 위치를 재면 다음 쪽 칸이 어긋난다. 실제 레이아웃(rect·겹침·또렷함)은 itda-hyve selftest 와 눈 확인이 본다.
const fs = require('fs');
const vm = require('vm');
const assert = require('assert');
const path = require('path');

const ctx = { console };
ctx.window = ctx;
vm.createContext(ctx);
vm.runInContext(fs.readFileSync(path.join(__dirname, '..', 'slides.js'), 'utf8'), ctx);
const S = ctx.window.ItdaSlides;
let cases = 0;
const eq = (name, got, want) => { cases++; assert.deepStrictEqual(JSON.parse(JSON.stringify({ v: got })), JSON.parse(JSON.stringify({ v: want })), name); };

function style() {
  const vars = {}, writes = [];
  const t = {
    getPropertyValue: k => vars[k] || "", setProperty: (k, v) => { vars[k] = v; writes.push(k); }, removeProperty: k => { delete vars[k]; writes.push(k); }, vars, writes,
  };
  return new Proxy(t, { get: (o, k) => (k in o ? o[k] : ""), set: (o, k, v) => { o[k] = v; writes.push(k); return true; } });
}
function classes(...init) { const c = new Set(init); return { contains: x => c.has(x), add: x => c.add(x), remove: x => c.delete(x) }; }

// rig 는 쪽 count 장(hidden 번호는 숨김)과 칸(note.h — 내용 높이)을 만든다. 쪽은 왼쪽 100, 첫 위 50, 사이 20, 크기 1000×500(화면 px).
function rig(count, { hidden = [], empty = [], h = () => 80, zoom = 1 } = {}) {
  const box = { getBoundingClientRect: () => ({ left: 10, top: 5 }) };
  const pages = { style: style(), list: [] };
  const rectOf = i => {
    let top = 50;
    for (let k = 0; k < i; k++) {
      if (hidden.includes(k)) continue;
      top += 500 + 20 + (parseFloat(pages.list[k].style.vars["--sn-after"]) || 0) * zoom;
    }
    return { left: 100, right: 1100, top, bottom: top + 500, width: 1000, height: 500 };
  };
  for (let i = 0; i < count; i++) pages.list.push({ style: style(), offsetParent: hidden.includes(i) ? null : pages, getBoundingClientRect: () => rectOf(i) });
  pages.querySelectorAll = () => pages.list;
  const notes = { parentElement: box, children: [] };
  for (let i = 0; i < count; i++) {
    notes.children.push({ style: style(), dataset: { slide: String(i + 1) }, classList: classes(...(empty.includes(i) ? ["sn-empty"] : [])), hidden: false, offsetHeight: h(i) });
  }
  return { pages, notes, zoom };
}
const spot = n => ({ hidden: n.hidden, left: n.style.left, top: n.style.top, width: n.style.width, height: n.style.height });

// 순수 계산: rect 는 transform 이 반영된 화면 좌표, 원점은 .pages-box
const R = { left: 100.4, right: 1100.4, top: 50.2, bottom: 550.2, width: 1000, height: 500 };
eq("오른쪽: 쪽 오른쪽 + 사이, 같은 위·높이, 칸 너비", S.notePlacement([R], { left: 10, top: 5 }, { layout: "right", noteW: 320, gap: 20 }),
  [{ hidden: false, left: 1110, top: 45, width: 320, height: 500 }]);
eq("아래: 쪽 왼쪽, 쪽 아래 + 10, 쪽 너비, 높이는 내용", S.notePlacement([R], { left: 10, top: 5 }, { layout: "below", noteW: 320, gap: 20 }),
  [{ hidden: false, left: 90, top: 555, width: 1000, height: null }]);
eq("숨은 쪽의 칸은 숨긴다", S.notePlacement([{ hidden: true }], { left: 0, top: 0 }, { layout: "right", noteW: 320, gap: 20 }), [{ hidden: true }]);
eq("여백은 배치 px", S.noteAfter(90, 1.5), 100 / 1.5);

// 오른쪽: #pages 오른쪽 여백(side / zoom), 빈 칸도 보인다, 숨은 쪽의 칸은 숨긴다, 쪽을 미는 여백은 지운다
{
  const r = rig(3, { hidden: [1], empty: [2], zoom: 2 });
  r.pages.list[0].style.setProperty("--sn-after", "99px");
  S.layoutNotes(r.pages, r.notes, { layout: "right", zoom: 2, noteW: 320, gap: 20 });
  eq("오른쪽: #pages 여백", r.pages.style.paddingRight, "170px");
  eq("오른쪽: 첫 칸", spot(r.notes.children[0]), { hidden: false, left: "1110px", top: "45px", width: "320px", height: "500px" });
  eq("오른쪽: 숨은 쪽의 칸", r.notes.children[1].hidden, true);
  eq("오른쪽: 빈 칸도 보임", r.notes.children[2].hidden, false);
  eq("오른쪽: 옛 여백을 지움", r.pages.list[0].style.vars["--sn-after"], undefined);
  const before = r.notes.children[0].style.writes.length, pw = r.pages.style.writes.length;
  S.layoutNotes(r.pages, r.notes, { layout: "right", zoom: 2, noteW: 320, gap: 20 });
  eq("같은 값이면 쓰지 않음", [r.notes.children[0].style.writes.length, r.pages.style.writes.length], [before, pw]);
}

// 아래: 높이를 재 여백을 건 뒤 위치 — 다음 칸의 위는 앞 칸의 여백만큼 밀린 쪽 아래 + 10. 빈 칸은 숨기고 여백도 없다
{
  const r = rig(3, { empty: [1], h: i => [80, 0, 140][i] });
  r.pages.style.paddingRight = "170px";
  S.layoutNotes(r.pages, r.notes, { layout: "below", zoom: 1, noteW: 320, gap: 20 });
  eq("아래: #pages 여백을 지움", r.pages.style.paddingRight, "");
  eq("아래: 첫 칸", spot(r.notes.children[0]), { hidden: false, left: "90px", top: "555px", width: "1000px", height: "" });
  eq("아래: 빈 칸은 숨김", r.notes.children[1].hidden, true);
  eq("아래: 여백", r.pages.list.map(p => p.style.vars["--sn-after"]), ["90px", null, "150px"]);
  // 쪽 3 위 = 50 + (500+20+90) + (500+20) = 1180, 아래 1680, + 10 − 원점 5
  eq("아래: 앞 쪽들이 밀린 자리", r.notes.children[2].style.top, "1685px");
  // 그림이 늦게 와 칸이 높아지면 다시 잰다
  r.notes.children[0].offsetHeight = 180;
  S.layoutNotes(r.pages, r.notes, { layout: "below", zoom: 1, noteW: 320, gap: 20 });
  eq("아래: 높아진 칸의 여백", r.pages.list[0].style.vars["--sn-after"], "190px");
  eq("아래: 뒤 칸이 밀림", r.notes.children[2].style.top, "1785px");
  // 끔: 여백·padding 만 지운다
  S.layoutNotes(r.pages, r.notes, { layout: "off" });
  eq("끔: 쪽 여백이 없다", r.pages.list.map(p => p.style.vars["--sn-after"]), [null, null, null]);
}

// placeNotes: into 가 있으면 칸을 그 상자에, 지울 때도 그 상자에서
{
  const made = [];
  const elem = tag => {
    const e = { tag, children: [], dataset: {}, classList: classes(), appendChild(c) { this.children.push(c); return c; }, querySelectorAll: () => [], remove() { made.push("remove"); } };
    return e;
  };
  ctx.document = { createElement: elem };
  const into = elem("div");
  into.querySelectorAll = sel => (sel === ":scope > .slide-note" ? into.children.slice() : []);
  const pg = { after() { throw new Error("into 가 있으면 쪽 뒤에 붙이지 않는다"); } };
  const box = { querySelectorAll: sel => (sel === ":scope > .page" ? [pg, pg] : []) };
  S.placeNotes(box, { notes: ["", ""], on: true, into });
  eq("into: 칸이 상자에", into.children.map(c => c.dataset.slide), ["1", "2"]);
  into.children.forEach(c => { c.remove = () => { into.children.splice(into.children.indexOf(c), 1); }; });
  S.placeNotes(box, { notes: [], on: false, into });
  eq("into: 끄면 상자에서 지움", into.children.length, 0);
}

// watchNotes: 잰 높이와 다르면 relayout, 같으면 부르지 않는다. 칸을 다시 만들면 다시 건다
{
  let fire = null, calls = 0;
  const watched = new Set(); // 지금 관찰 중인 노드 — disconnect 하면 비고 observe 하면 든다
  ctx.ResizeObserver = class { constructor(f) { fire = f; } observe(n) { watched.add(n); } disconnect() { watched.clear(); } };
  const n = { snH: 80, offsetHeight: 80 }, plain = { offsetHeight: 300 }, notes = { children: [n, plain] };
  const rearm = S.watchNotes(notes, () => { calls++; n.snH = n.offsetHeight; });
  rearm();
  cases++;
  assert.ok(watched.size === 2 && watched.has(n) && watched.has(plain), "칸마다 감시");
  fire();
  eq("같은 높이면 부르지 않음(되돌이 없음) — 아래 배치가 아닌 칸(snH 없음)도", calls, 0);
  n.offsetHeight = 200;
  fire();
  eq("높이가 바뀌면 다시 놓음", calls, 1);
  fire();
  eq("다시 놓은 뒤에는 부르지 않음", calls, 1);
  // 칸을 다시 만들면(children 교체) 다시 걸어 옛 칸을 놓고 새 칸을 본다
  const fresh = { snH: 50, offsetHeight: 50 };
  notes.children = [fresh];
  rearm();
  cases++;
  assert.ok(watched.size === 1 && watched.has(fresh) && !watched.has(n) && !watched.has(plain), "다시 걸면 옛 칸 해제·새 칸 관찰");
  fresh.offsetHeight = 90;
  fire();
  eq("새 칸의 높이 변화도 다시 놓음", calls, 2);
}

console.log(`NOTES DONE cases=${cases} failed=0`);
