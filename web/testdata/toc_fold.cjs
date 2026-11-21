// 목차 접기(itda-hyve#985 — share.js tocHTML 의 접기 꼴·tocTree·tocFold*·tocFold·paintTocFold·tocBulkHead·paintTocBulk) 시험. Go 시험
// (toc_fold_test.go)이 pager.js·slides.js·share.js 경로를 넘긴다. 화면 안 동작(누르기·굴림·서랍)은 보지 않는다 — 가짜 DOM 은 단추·ul 만 흉내 낸다.
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
const plain = m => JSON.parse(JSON.stringify(m));

// 목차: 1(하위 1.1(하위 1.1.1)·1.2) · 2(하위 없음) · id 없는 3(하위 3.1(하위 3.1.1))
const items = [
  { ID: "s1", Number: "1", Text: "하나", Children: [
    { ID: "s11", Number: "1.1", Text: "하나 하나", Children: [{ ID: "s111", Text: "깊은 절", Children: [] }] },
    { ID: "s12", Number: "1.2", Text: "하나 둘", Children: null }] },
  { ID: "s2", Number: "2", Text: "둘", Children: [] },
  { ID: "", Text: "id 없음", Children: [{ ID: "s31", Text: "셋 하나", Children: [{ ID: "s311", Text: "셋 하나 하나" }] }] },
];

// 1. tocTree: 부모 표·하위 있는 항목(문서 차례)·id 없는 항목 건너뜀·같은 id 는 앞의 것
const tr = S.tocTree(items);
eq("트리: 부모", [...tr.parent].map(([k, v]) => k + "<" + v), ["s1<", "s11<s1", "s111<s11", "s12<s1", "s2<", "s31<", "s311<s31"]);
eq("트리: 하위 있는 항목(문서 차례)", tr.order, ["s1", "s11", "s31"]);
eq("트리: 빈·틀린 목차", [S.tocTree(null).order, S.tocTree([null, { ID: "x" }]).order, [...S.tocTree([{ ID: "x", Children: [{ ID: "x" }] }]).parent]], [[], [], [["x", ""]]]);
eq("조상: 가까운 것부터, 목차에 없으면 []", [S.tocAncestors("s111", tr), S.tocAncestors("s1", tr), S.tocAncestors("없음", tr)], [["s11", "s1"], [], []]);

// 2. 펼침 판정: 기억이 자동보다 앞선다
const none = Object.create(null);
eq("판정: 기억 없음 — 자동만", [S.tocFoldIsOpen("s1", none, new Set()), S.tocFoldIsOpen("s1", none, new Set(["s1"]))], [false, true]);
eq("판정: 기억 closed 는 자동을 이긴다·open 은 자동 없이도", [S.tocFoldIsOpen("s1", { s1: "closed" }, new Set(["s1"])), S.tocFoldIsOpen("s1", { s1: "open" }, new Set())], [false, true]);
eq("판정: 원형의 이름(constructor·__proto__)은 기억이 아니다", [S.tocFoldIsOpen("constructor", {}, new Set()), S.tocFoldIsOpen("__proto__", {}, new Set())], [false, false]);

// 3. 따라 펼치기: 사람이 접은 조상은 펼치지 않고, 보이는 가장 가까운 조상에 강조
{
  const auto = new Set();
  S.tocFoldFollow("s111", tr, none, auto);
  eq("따라 펼치기: 조상 모두", [...auto].sort(), ["s1", "s11"]);
  eq("보이는 강조: 다 펼치면 그 항목", S.tocFoldShown("s111", tr, none, auto), "s111");
  const memo = { s11: "closed" }, a2 = new Set();
  S.tocFoldFollow("s111", tr, memo, a2);
  eq("따라 펼치기: 사람이 접은 s11 은 빼고 s1 만", [...a2], ["s1"]);
  eq("보이는 강조: 접힌 s11", S.tocFoldShown("s111", tr, memo, a2), "s11");
  eq("보이는 강조: 맨 위 조상부터 — s1 도 접혔으면 s1", S.tocFoldShown("s111", tr, { s1: "closed", s11: "closed" }, new Set()), "s1");
  eq("보이는 강조: 최상위·목차에 없는 id·빈 id 는 그대로", [S.tocFoldShown("s2", tr, none, new Set()), S.tocFoldShown("x", tr, none, new Set()), S.tocFoldShown("", tr, none, new Set())], ["s2", "x", ""]);
}

// 4. 일괄 단추
eq("일괄: 하나라도 접혔으면 펼치기, 다 펼쳤으면 접기, 하위 없으면 숨김",
  [S.tocFoldBulk(tr, none, new Set()), S.tocFoldBulk(tr, { s1: "open", s11: "open", s31: "open" }, new Set()), S.tocFoldBulk(tr, none, new Set(["s1", "s11", "s31"])), S.tocFoldBulk(S.tocTree([{ ID: "a" }]), none, new Set())],
  ["expand", "collapse", "collapse", ""]);

// 5. 기억 맵 고르기·저장 글
eq("고르기: 값·지금 목차", plain(S.tocFoldClean({ s1: "open", s11: "closed", s2: "open", gone: "open", s31: 1 }, tr)), { s1: "open", s11: "closed" });
eq("고르기: 트리 없으면 값만·틀린 꼴은 빈 맵", [plain(S.tocFoldClean({ gone: "open", x: "yes" })), plain(S.tocFoldClean(null)), plain(S.tocFoldClean(["open"])), plain(S.tocFoldClean("x"))], [{ gone: "open" }, {}, {}, {}]);
{
  const big = {};
  for (let i = 0; i < S.TOC_FOLD_IDS + 5; i++) big["h" + i] = "open";
  const c = S.tocFoldClean(big);
  ok("고르기: 문서 하나는 TOC_FOLD_IDS 개까지(앞에서부터)", Object.keys(c).length === S.TOC_FOLD_IDS && c.h0 === "open" && !("h" + S.TOC_FOLD_IDS in c), Object.keys(c).length);
  const p = S.tocFoldClean({ ["__proto__"]: "open" });
  ok("고르기: __proto__ 도 그냥 열쇠(원형 없는 객체)", Object.getPrototypeOf(p) === null && p["__proto__"] === "open" && Object.keys(p).length === 1, Object.keys(p));
}
eq("저장 글: 깨짐·틀린 꼴은 없음", [S.tocFoldList(null), S.tocFoldList("{"), S.tocFoldList('{"a":1}'), S.tocFoldList('[["",{"a":"open"}],[1,{}],"x",["k",{"a":"bad"}]]')], [[], [], [], []]);
eq("저장 글: 같은 열쇠는 뒤의 것", S.tocFoldList('[["k",{"a":"open"}],["j",{"b":"open"}],["k",{"a":"closed"}]]').map(e => [e[0], plain(e[1])]), [["j", { b: "open" }], ["k", { a: "closed" }]]);
{
  let raw = S.tocFoldPut(null, "/a/", { s1: "open" });
  raw = S.tocFoldPut(raw, "/b/", { s1: "closed" });
  eq("넣기: 다른 문서와 섞이지 않는다", [plain(S.tocFoldGet(raw, "/a/")), plain(S.tocFoldGet(raw, "/b/")), plain(S.tocFoldGet(raw, "/c/"))], [{ s1: "open" }, { s1: "closed" }, {}]);
  raw = S.tocFoldPut(raw, "/a/", { s1: "closed" });
  eq("넣기: 쓴 문서는 맨 뒤(가장 최근)", JSON.parse(raw).map(e => e[0]), ["/b/", "/a/"]);
  eq("넣기: 빈 맵이면 그 문서를 지운다", JSON.parse(S.tocFoldPut(raw, "/a/", {})).map(e => e[0]), ["/b/"]);
  eq("넣기: 빈 열쇠·너무 긴 열쇠는 그대로", [S.tocFoldPut(raw, "", { s1: "open" }) === raw, S.tocFoldPut(raw, "/" + "x".repeat(2048), { s1: "open" }) === raw], [true, true]);
  let r = null;
  for (let i = 0; i < S.TOC_FOLD_DOCS + 3; i++) r = S.tocFoldPut(r, "/d" + i + "/", { s1: "open" });
  const keys = JSON.parse(r).map(e => e[0]);
  ok("넣기: 문서 수 상한 — 오래 전에 쓴 것부터 버린다", keys.length === S.TOC_FOLD_DOCS && keys[0] === "/d3/" && keys[keys.length - 1] === "/d" + (S.TOC_FOLD_DOCS + 2) + "/", [keys.length, keys[0]]);
  eq("넣기: cap 을 주면 그만큼", JSON.parse(S.tocFoldPut(r, "/z/", { s1: "open" }, 2)).map(e => e[0]), ["/d" + (S.TOC_FOLD_DOCS + 2) + "/", "/z/"]);
}
eq("열쇠: 경로(index.html 뗌) + 폴더 공유 문서", [S.tocFoldKey("/s/tok/", ""), S.tocFoldKey("/s/tok/", "a/b.md"), S.tocFoldKey("/lec/v1/intro/index.html"), S.tocFoldKey("", undefined)],
  ["/s/tok/", "/s/tok/?doc=a/b.md", "/lec/v1/intro/", "/"]);

// 6. localStorage 기억(tocFoldPrefs): 읽기·쓰기가 던지면 메모리만
{
  const m = {};
  const st = { getItem: k => (k in m ? m[k] : null), setItem: (k, v) => { m[k] = String(v); } };
  const p = S.tocFoldPrefs(() => st);
  p.save("/a/", { s1: "open" });
  eq("기억: 쓰기·다시 읽기(다른 화면)", [plain(S.tocFoldPrefs(() => st).load("/a/")), Object.keys(m)], [{ s1: "open" }, ["hs-toc-fold"]]);
  m["hs-toc-fold"] = S.tocFoldPut(m["hs-toc-fold"], "/b/", { s1: "closed" }); // 다른 탭이 그사이 씀
  p.save("/a/", { s1: "closed" });
  eq("기억: 쓸 때 다시 읽어 다른 탭의 문서를 지우지 않는다", JSON.parse(m["hs-toc-fold"]).map(e => e[0]), ["/b/", "/a/"]);
  const thrower = () => { throw new Error("막힘"); };
  const q = S.tocFoldPrefs(thrower);
  eq("기억: 못 읽으면 빈 맵", plain(q.load("/a/")), {});
  q.save("/a/", { s1: "open" });
  eq("기억: 못 쓰면 이 화면 메모리", plain(q.load("/a/")), { s1: "open" });
  const ro = { getItem: () => null, setItem: () => { throw new Error("가득"); } };
  const r = S.tocFoldPrefs(() => ro);
  r.save("/a/", { s1: "open" });
  r.save("/b/", { s1: "closed" });
  eq("기억: 쓰기만 막혀도 메모리로 이어 쓴다", [plain(r.load("/a/")), plain(r.load("/b/"))], [{ s1: "open" }, { s1: "closed" }]);
}

// 7. tocFold — 인수 조건 2·3·5 의 규칙
{
  const saved = [];
  const f = S.tocFold(null, m => saved.push(plain(m)));
  f.setTree(items);
  eq("처음: 기억 없으면 모두 접힘·일괄은 펼치기", [f.open("s1"), f.open("s11"), f.open("s31"), f.bulk()], [false, false, false, "expand"]);
  ok("처음: 지금 위치가 생기면 조상만 펼친다(참을 돌려준다)", f.follow("s111") === true && f.open("s1") && f.open("s11") && !f.open("s31"), [f.open("s1"), f.open("s11"), f.open("s31")]);
  eq("같은 항목이면 다시 하지 않는다", f.follow("s111"), false);
  f.follow("s311");
  eq("굴려서 다른 절: 새 조상 펼침, 앞서 펼친 것은 그대로", [f.open("s1"), f.open("s11"), f.open("s31")], [true, true, true]);
  eq("자동 펼침은 저장하지 않는다", saved, []);
  ok("단추: 자동으로 펼친 s11 을 접으면 closed 로 기억", f.toggle("s11") === true && !f.open("s11"), saved);
  eq("단추: 저장은 지금 목차의 하위 있는 항목만", saved, [{ s11: "closed" }]);
  f.follow("s1");
  f.follow("s111");
  eq("사람이 접은 가지 안으로 굴리면 접힌 채 부모에 강조", [f.open("s11"), f.shown("s111")], [false, "s11"]);
  eq("단추: 하위 없는·없는 id 는 아무것도 하지 않는다", [f.toggle("s2"), f.toggle("없음"), saved.length], [false, false, 1]);
  f.toggle("s11");
  eq("단추: 다시 누르면 open", [f.open("s11"), saved[1]], [true, { s11: "open" }]);
  eq("일괄: 모두 펼쳤으면 접기", f.bulk(), "collapse");
  f.all(false);
  eq("모두 접기: 맵 전체를 closed 로·일괄은 펼치기", [saved[2], f.bulk(), f.shown("s111")], [{ s1: "closed", s11: "closed", s31: "closed" }, "expand", "s1"]);
  f.follow("s311");
  eq("모두 접은 뒤 굴려도 펼치지 않는다", [f.open("s31"), f.shown("s311")], [false, "s31"]);
  f.all(true);
  eq("모두 펼치기", [saved[3], f.bulk(), f.shown("s311")], [{ s1: "open", s11: "open", s31: "open" }, "collapse", "s311"]);
  const g = S.tocFold({ s11: "closed", gone: "open", s1: "x" }, null);
  g.setTree(items);
  g.follow("s111");
  eq("기억에서 시작: 기억이 자동보다 앞선다·save 없어도 된다", [g.open("s1"), g.open("s11"), g.shown("s111"), g.toggle("s1")], [true, false, "s11", true]);
  g.setTree([{ ID: "s1", Children: [{ ID: "s11" }] }]);
  eq("새 목차: 기억·자동은 그대로", [g.open("s1"), g.bulk()], [false, "expand"]);
}

// 7-1. 목차가 바뀌고 지금 항목 id 가 같아도 새 조상을 다시 본다(리뷰 r1 [2]) — 사람이 접은 새 부모는 접힌 채
{
  const f = S.tocFold(null, null);
  f.setTree([{ ID: "a", Children: [{ ID: "x" }] }, { ID: "b" }]);
  f.follow("x");
  f.setTree([{ ID: "a" }, { ID: "b", Children: [{ ID: "x" }] }]);
  eq("새 목차·같은 id: 새 부모를 펼친다", [f.follow("x"), f.open("b"), f.shown("x"), f.follow("x")], [true, true, "x", false]);
  const g = S.tocFold({ b: "closed" }, null);
  g.setTree([{ ID: "a", Children: [{ ID: "x" }] }, { ID: "b" }]);
  g.follow("x");
  g.setTree([{ ID: "a" }, { ID: "b", Children: [{ ID: "x" }] }]);
  eq("새 목차·같은 id: 사람이 접은 새 부모는 그대로·그 부모를 강조", [g.follow("x"), g.open("b"), g.shown("x")], [false, false, "b"]);
}

// 8. 접기 꼴 HTML
{
  const f = S.tocFold({ s11: "closed" }, null);
  f.setTree(items);
  f.follow("s12");
  const h = S.tocHTML(items, id => f.open(id));
  ok("HTML: 하위 있는 항목에 단추(펼침 ▾ aria-expanded=true)",
    h.startsWith('<ul><li><div class="toc-row"><button type="button" class="toc-tg" data-fold="s1" aria-expanded="true" aria-label="하위 항목 접기" title="하위 항목 접기">▾</button><a href="#s1" data-id="s1"><span class="num">1</span><span>하나</span></a></div><ul><li>'), h);
  ok("HTML: 접힌 항목은 ▸·하위 ul hidden", h.includes('data-fold="s11" aria-expanded="false" aria-label="하위 항목 펼치기" title="하위 항목 펼치기">▸</button><a href="#s11" data-id="s11"><span class="num">1.1</span><span>하나 하나</span></a></div><ul hidden><li>'), h);
  ok("HTML: 하위 없는 항목은 빈 자리", h.includes('<li><div class="toc-row"><span class="toc-tg-pad"></span><a href="#s2" data-id="s2">'), h);
  ok("HTML: id 없는 항목은 단추 없이 하위를 보인다", h.includes('<span class="toc-tg-pad"></span><a href="#" data-id=""><span>id 없음</span></a></div><ul><li><div class="toc-row"><button type="button" class="toc-tg" data-fold="s31" aria-expanded="false"'), h);
  eq("HTML: 항목마다 a 하나", (h.match(/<a /g) || []).length, 8);
  const x = S.tocHTML([{ ID: 'q"<', Text: "t", Children: [{ ID: "c" }] }], () => true);
  ok("HTML: 단추 data-fold 도 이스케이프", x.includes('data-fold="q&quot;&lt;"') && !x.includes('q"<'), x);
  eq("HTML: open 이 없으면 옛 꼴 그대로", S.tocHTML([{ ID: "a", Text: "A", Children: [{ ID: "b", Text: "B" }] }]), '<ul><li><a href="#a" data-id="a"><span>A</span></a><ul><li><a href="#b" data-id="b"><span>B</span></a></li></ul></li></ul>');
}

// 9. DOM 고치기(paintTocFold·tocBulkHead·paintTocBulk) — 가짜 요소
{
  const el = (tag, cls) => {
    const e = { tagName: tag.toUpperCase(), className: cls || "", hidden: false, children: [], parentNode: null, attrs: {}, dataset: {}, title: "", textContent: "", innerHTML: "",
      classList: { contains: c => (e.className || "").split(" ").includes(c) },
      setAttribute: (k, v) => { e.attrs[k] = String(v); }, getAttribute: k => e.attrs[k],
      appendChild: c => { if (c.parentNode) c.parentNode.children.splice(c.parentNode.children.indexOf(c), 1); c.parentNode = e; e.children.push(c); return c; },
      insertBefore: (c, ref) => { c.parentNode = e; e.children.splice(e.children.indexOf(ref), 0, c); return c; },
      querySelector: sel => (sel === ".toc-fold-all" ? e.children.find(c => c.classList.contains("toc-fold-all")) || null : null) };
    Object.defineProperty(e, "nextElementSibling", { get: () => { const p = e.parentNode; return p ? p.children[p.children.indexOf(e) + 1] || null : null; } });
    return e;
  };
  const li = el("li"), row = li.appendChild(el("div", "toc-row")), btn = row.appendChild(el("button", "toc-tg")), ul = li.appendChild(el("ul"));
  btn.dataset.fold = "s1";
  const nav = { querySelectorAll: () => [btn] };
  const f = S.tocFold(null, null);
  f.setTree(items);
  S.paintTocFold(nav, f);
  eq("고치기: 접힘", [btn.attrs["aria-expanded"], btn.textContent, btn.title, ul.hidden], ["false", "▸", "하위 항목 펼치기", true]);
  f.toggle("s1");
  S.paintTocFold(nav, f);
  eq("고치기: 펼침", [btn.attrs["aria-expanded"], btn.attrs["aria-label"], btn.textContent, ul.hidden], ["true", "하위 항목 접기", "▾", false]);
  const side = el("aside"), title = side.appendChild(el("div", "side-title")), tnav = side.appendChild(el("nav"));
  title.textContent = "목차";
  const doc = { createElement: t => el(t) };
  const b = S.tocBulkHead(doc, title);
  const head = side.children[0];
  ok("머리: .side-title 을 div.toc-head 로 싸고 끝에 단추(숨김)", head.className === "toc-head" && head.children[0] === title && head.children[1] === b && b.className === "toc-fold-all" && b.type === "button" && b.hidden && side.children[1] === tnav, side.children.map(c => c.className));
  eq("머리: 두 번 불러도 단추 하나", [S.tocBulkHead(doc, title) === b, head.children.length], [true, 2]);
  eq("머리: 자리가 없으면 null", [S.tocBulkHead(doc, null), S.tocBulkHead(doc, el("div"))], [null, null]);
  S.paintTocBulk(b, f, false);
  ok("일괄 단추: 접힌 게 있으면 「모두 펼치기」", !b.hidden && b.title === "모두 펼치기" && b.attrs["aria-label"] === "모두 펼치기" && b.dataset.next === "expand" && b.innerHTML.includes("<svg"), [b.hidden, b.title]);
  f.all(true);
  S.paintTocBulk(b, f, false);
  eq("일괄 단추: 다 펼쳤으면 「모두 접기」", [b.hidden, b.title, b.dataset.next], [false, "모두 접기", "collapse"]);
  S.paintTocBulk(b, f, true);
  eq("일괄 단추: 숨김(목차 없음)", b.hidden, true);
  const leaf = S.tocFold(null, null);
  leaf.setTree([{ ID: "a" }, { ID: "b" }]);
  S.paintTocBulk(b, leaf, false);
  eq("일괄 단추: 하위 있는 항목이 없으면 숨김", b.hidden, true);
  ok("일괄 단추: 단추가 없는 화면(null)도 던지지 않는다", (S.paintTocBulk(null, f, false), true));
}

console.log("TOCFOLD DONE cases=" + cases + " failed=" + failed);
