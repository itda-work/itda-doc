// 웹 뷰어의 로더(web/viewer/itda-doc.js)와 뷰어 원천(web/viewer/viewer.js viewerSource)을 node 에서 돌린다(viewer_test.go).
//   node viewer.cjs <wasm_exec.js> <temml.min.js> <itda-doc.wasm> <web 폴더> <samples.json> <out.json>
// 브라우저처럼 전역 스크립트로 싣는다(wasm_exec.js → temml.min.js → itda-doc.js → share.js → viewer.js — document 가 없어 화면은 시작하지
// 않고 부품만 내보낸다). wasm 은 ItdaDocLoader.load 에 가짜 fetch(파일을 읽어 application/octet-stream 으로 — instantiate 길)로 올린다.
// 결과는 {checks: [{name, ok, got}], samples: [{name, view}]} 이고 판정은 Go 가 한다.
"use strict";
const nodeFS = require("fs");
const path = require("path");
const vm = require("vm");
const [wasmExec, temmlJS, wasmPath, webDir, samplesPath, outPath] = process.argv.slice(2);

globalThis.TextEncoder ??= require("util").TextEncoder;
globalThis.TextDecoder ??= require("util").TextDecoder;
globalThis.performance ??= require("perf_hooks").performance;
globalThis.crypto ??= require("crypto");
require(wasmExec);
const script = (p, name) => vm.runInThisContext(nodeFS.readFileSync(p, "utf8"), { filename: name });
script(temmlJS, "temml.min.js");
globalThis.window = globalThis; // share.js·viewer.js 는 window.* 로 내보낸다
script(path.join(webDir, "viewer", "itda-doc.js"), "itda-doc.js");
script(path.join(webDir, "share.js"), "share.js");
script(path.join(webDir, "viewer", "viewer.js"), "viewer.js");

const checks = [];
const check = (name, ok, got) => checks.push({ name, ok: !!ok, got: got === undefined ? "" : String(got) });
const throwsType = (name, f) => {
  try { f(); check(name, false, "던지지 않음"); } catch (e) { check(name, e instanceof TypeError, e && e.message); }
};

(async () => {
  const L = globalThis.ItdaDocLoader;
  // load 의 옵션 오류는 거부(TypeError)
  await L.load({ wasm: 5 }).then(() => check("load 옵션 오류", false, "거부하지 않음"), e => check("load 옵션 오류", e instanceof TypeError, e.message));
  const fakeFetch = url => Promise.resolve({ ok: true, status: 200, headers: { get: () => "application/octet-stream" },
    arrayBuffer: () => Promise.resolve(nodeFS.readFileSync(url)) });
  const doc = await L.load({ wasm: wasmPath, fetch: fakeFetch });
  check("load → doc", doc && typeof doc.render === "function" && doc.version && doc.version.go, doc && JSON.stringify(doc.version));

  // 옵션 오류는 throw(2a 열린 결정 4 (b)) — itdaDoc 이라면 View.error·null 이 될 것들
  throwsType("render 원문이 글자 아님", () => doc.render(1));
  throwsType("render 모르는 키", () => doc.render("x", { thme: "mono" }));
  throwsType("render 대소문자 다른 키", () => doc.render("x", { THEME: "mono" }));
  throwsType("render 꼴 틀림", () => doc.render("x", { theme: 5 }));
  throwsType("render JSON 글자 옵션", () => doc.render("x", '{"theme":"mono"}'));
  throwsType("render 배열 옵션", () => doc.render("x", []));
  throwsType("themeCSS 모르는 키", () => doc.themeCSS("classic", { bad: 1 }));
  throwsType("themeCSS 꼴 틀림", () => doc.themeCSS("classic", { unscoped: "yes" }));
  throwsType("themeCSS 이름이 글자 아님", () => doc.themeCSS(5));
  // 던지지 않는 것
  const v0 = doc.render("# 제목\n", { theme: "mono", allowRemoteImages: false, dropNotes: false, theme: "mono" });
  check("render 옵션 정상", !v0.error && v0.theme === "mono" && v0.html.includes("<h1"), v0.error || v0.theme);
  check("render undefined 값은 없는 키", !doc.render("x", { theme: undefined }).error);
  check("themeCSS 없는 테마는 null", doc.themeCSS("nope") === null);
  check("themeCSS 글자", typeof doc.themeCSS("classic", { unscoped: true }) === "string");
  check("themes 배열", Array.isArray(doc.themes()) && doc.themes().length >= 5, doc.themes().length);
  // 그리다 생긴 오류는 throw 가 아니라 View.error 다 — 여기서는 없는 경우만 본다(옵션은 로더가 거른다)

  // 뷰어 원천: 가짜 화면 api·<style> 로 attach → start → open
  const V = globalThis.ItdaViewer;
  const shown = [], failed = [], warnLines = [];
  let themeCSS = null, opens = 0;
  const api = { show: v => { shown.push(v); return Promise.resolve(); }, failed: m => failed.push(m), toast() {} };
  const src = V.viewerSource({ load: () => Promise.resolve(doc), setThemeCSS: css => { themeCSS = css; return Promise.resolve(); },
    unscoped: () => false, onOpen: () => opens++, onView: (v, lines) => warnLines.push(lines) });
  check("뷰어 원천은 탭 제목을 고정", src.tabTitle === "itda-doc 뷰어" && V.TAB_TITLE === src.tabTitle, src.tabTitle);
  const pick = globalThis.ItdaShare.pickSource({ ItdaViewSource: src }, { dataset: {} });
  check("pickSource 가 뷰어 원천을 고름", pick === src);
  const linked = src.attach(api);
  linked.start();
  for (let i = 0; i < 100 && !src.doc(); i++) await new Promise(r => setTimeout(r, 5));
  check("start 가 wasm 을 올림", src.doc() === doc && failed.length === 0, failed.join(";"));
  const report = nodeFS.readFileSync(path.join(webDir, "..", "mdoc", "testdata", "report.md"), "utf8");
  await src.open("보고서", report);
  const v1 = shown[shown.length - 1];
  check("open → show", v1 && !v1.error && v1.html.length > 0, v1 && v1.error);
  check("open 은 doc.render 와 같은 View(html 의 상대 그림만 다름)",
    JSON.stringify({ ...v1, html: "", version: 0 }) === JSON.stringify({ ...doc.render(report, { allowRemoteImages: false }), html: "", version: 0 }));
  check("상대 경로 그림은 요청하지 않음", !/<img\b[^>]*\ssrc="images\//.test(v1.html) && v1.html.includes('data-local-src="images/org.png"'));
  const lines = warnLines[warnLines.length - 1];
  check("경고 목록에 상대 경로 그림 한 줄(파일 하나 — 폴더로 열면 그린다)", lines.some(l => l.code === "viewer-local-image" && l.text.includes("1개") && l.text.includes("폴더로 열면 그린다")), JSON.stringify(lines));
  check("경고 목록에 View.warnings(줄 번호)", lines.some(l => l.line > 0 && l.code === "remote-image"), JSON.stringify(lines));
  check("open 은 onOpen 을 한 번(새 문서 — 굴린 자리·경고 칸 되돌리기)", opens === 1, opens);
  check("applyTheme → themeCSS 를 건다", (await linked.applyTheme(v1)) === true && themeCSS === doc.themeCSS(v1.theme, { unscoped: false }));
  await src.setTheme("mono");
  const v2 = shown[shown.length - 1];
  await linked.applyTheme(v2);
  check("테마 고르기", v2.theme === "mono" && themeCSS === doc.themeCSS("mono", { unscoped: false }), v2.theme);
  check("테마를 바꿔도 onOpen 은 그대로", opens === 1, opens);
  check("원격 그림은 늘 끔", !/\ssrc="https:\/\/example\.com\/logo\.png"/.test(v2.html) && v2.html.includes('data-remote-src="https://example.com/logo.png"') && v2.remote_allowed === false);
  check("원격 스위치 없음", typeof src.setRemote === "undefined");
  await src.setTheme("");
  check("문서의 테마로 되돌림", shown[shown.length - 1].theme === v1.theme);
  // 원격 그림이 src 로 남은 원문(mdoc 이 그대로 두는 길은 없다 — 원문 HTML 은 속성 없이만 남는다)도 그려 본다
  await src.open("그림들", "![a](https://example.com/a.png) ![b](//example.com/b.png) ![c](pic.png) ![d](data:image/png;base64,AA)\n");
  check("두 번째 open 도 onOpen", opens === 2, opens);
  // 상대·// 링크가 섞인 원문 — 뷰어는 같은 출처로 이동하는 href 를 남기지 않는다(#·http(s)·mailto·tel 은 그대로)
  await src.open("링크들", "# 제목\n\n[계획](plans/x.md) [위](../up.md) [절대](/root.md) [호스트](//evil.example/a) [조각](#제목) [웹](https://example.com/) [메일](mailto:a@example.com) [전화](tel:123)\n");
  const vl = shown[shown.length - 1].html;
  check("상대·// 링크는 data-local-href 로", ['plans/x.md', '../up.md', '/root.md', '//evil.example/a'].every(h => vl.includes('data-local-href="' + h + '"')) &&
    ['href="https://example.com/"', 'href="mailto:a@example.com"', 'href="tel:123"'].every(h => vl.includes(h)) && /\shref="#/.test(vl), vl);
  check("localLinks 의 갈래", V.localLinks('<a href="a.md">a</a><a class="x" href="//h/b">b</a><a href="JavaScript:x">c</a><a href="#f">d</a><a href="HTTPS://h/">e</a><abbr title="t">f</abbr>') ===
    '<a data-local-href="a.md">a</a><a class="x" data-local-href="//h/b">b</a><a data-local-href="JavaScript:x">c</a><a href="#f">d</a><a href="HTTPS://h/">e</a><abbr title="t">f</abbr>');

  // show 가 받은 모든 html 에 data:·blob: 이 아닌 src 가 없다(원격·상대 그림을 요청하지 않는다) — 화면이 그리기 직전의 글
  const leaks = shown.filter(v => /<img\b[^>]*\ssrc="(?!data:|blob:)/i.test(v.html)).length;
  check("show 직전 html 에 원격·상대 src 없음", shown.length >= 4 && leaks === 0, "show " + shown.length + " 번 가운데 " + leaks + " 번");
  // show 가 받은 모든 html 에 #·http(s)·mailto·tel 이 아닌 href 가 없다(문서 안 링크가 같은 출처로 이동하지 않는다)
  const hrefLeaks = shown.filter(v => /<a\b[^>]*\shref="(?!#|https?:|mailto:|tel:)/i.test(v.html)).length;
  check("show 직전 html 에 #·http(s)·mailto·tel 이 아닌 href 없음", shown.length >= 5 && hrefLeaks === 0, "show " + shown.length + " 번 가운데 " + hrefLeaks + " 번");
  // localImages 의 갈래
  const li = V.localImages('<p><img src="a.png"><img alt="x" src="data:image/png;base64,AA"><img src="https://x.y/b.png"><img src="http://x.y/c.png"><img src="//x.y/d.png"></p>');
  check("localImages", li.local === 1 && li.remote === 3 && /\ssrc="data:/.test(li.html) && !/\ssrc="(https?:|\/\/|a\.png)/.test(li.html), JSON.stringify(li));
  const lw = V.warningLines({ warnings: [] }, li);
  // 상대 경로 그림 문구는 연 길마다 다르다(파일 하나·붙여넣기·견본) — 부품과 원천(open 의 kind) 둘 다 본다
  const why = { file: "파일 하나로 열었다(폴더로 열면 그린다)", paste: "붙여넣은 문서라 폴더가 없다", sample: "견본 글에는 그림 파일이 없다" };
  for (const kind of Object.keys(why)) {
    const t = (V.warningLines({ warnings: [] }, li, kind).find(l => l.code === "viewer-local-image") || {}).text || "";
    await src.open("그림 " + kind, "![c](pic.png)\n", kind);
    const t2 = (warnLines[warnLines.length - 1].find(l => l.code === "viewer-local-image") || {}).text || "";
    const others = Object.keys(why).filter(k => k !== kind).map(k => why[k]);
    check("상대 경로 그림 문구: " + kind, t === "상대 경로 그림 1개는 그리지 않음 — " + why[kind] && t2 === "상대 경로 그림 1개는 그리지 않음 — " + why[kind] &&
      !others.some(o => t.includes(o) || t2.includes(o)), t + " | " + t2);
  }
  check("경고 줄: 상대·원격", lw.length === 2 && lw[0].code === "viewer-local-image" && lw[1].code === "viewer-remote-image", JSON.stringify(lw));

  await folderChecks(V, doc, api, shown);

  // 견본: 빌드가 싣는 견본이 오류 없이 그려진다
  const samples = JSON.parse(nodeFS.readFileSync(samplesPath, "utf8"));
  const out = [];
  for (const s of samples) out.push({ name: s.name, view: doc.render(nodeFS.readFileSync(s.path, "utf8")) });
  nodeFS.writeFileSync(outPath, JSON.stringify({ checks, samples: out }));
  process.exit(0);
})().catch(e => { console.error(e); process.exit(1); });

// ---- 「지금 여는 일」 의 세대(viewerOpener — 리뷰 1회차 [1]·[3])와 droppedDir([6])·대소문자 실마리(열린 결정 3).
// 가짜 원천은 openFolder·open 을 기록하고 바로 그린 것으로 친다. 가짜 화면 ui 는 고르기 칸·토스트를 기록한다.
function deferred() { let res, rej; const p = new Promise((a, b) => { res = a; rej = b; }); return { p, res, rej }; }
function fakeOpener(V) {
  let cur = null;
  const log = [];
  const source = {
    openFolder(folder, path) { log.push("openFolder:" + path); cur = { kind: "folder", folder, path }; return Promise.resolve({ ok: true }); },
    open(name, t, kind) { log.push("open:" + kind + ":" + name); cur = { kind, name }; return Promise.resolve(); },
    current: () => cur,
  };
  const ui = { picked: null, list: null, shown: false, toasts: [],
    setFolder(list) { log.push("setFolder:" + list.join(",")); this.list = list; this.shown = true; },
    clearFolder() { log.push("clearFolder"); this.list = null; this.shown = false; },
    pick(p) { this.picked = p; }, toast(m) { this.toasts.push(m); } };
  return { o: V.viewerOpener(source, ui), ui, log, source };
}
const tableOf = (name, names) => ({ name, files: new Map(names.map(n => [n, new FakeFile(n.split("/").pop(), "# " + n)])), stopped: "" });
async function openerChecks(V) {
  const A = tableOf("A", ["a.md", "a2.md"]), B = tableOf("B", ["b.md"]);
  {
    const { o, ui, log } = fakeOpener(V);
    const wA = deferred();
    const pA = o.folder(wA.p);
    const rB = await o.folder(Promise.resolve(B));
    wA.res(A);
    const rA = await pA;
    check("세대: A 걷기 중 B 열기 → A 가 이기지 않음", rB.ok && rA.stale && o.folderTable() === B && JSON.stringify(ui.list) === '["b.md"]' && ui.picked === "b.md" &&
      !log.some(l => l.startsWith("setFolder:a") || l === "openFolder:a.md"), JSON.stringify({ rA, log }));
  }
  for (const kind of ["file", "paste", "sample"]) {
    const { o, ui, log, source } = fakeOpener(V);
    const wA = deferred();
    const pA = o.folder(wA.p);
    const rT = await o.text("새 " + kind, Promise.resolve("# t\n"), kind);
    const toastsBefore = ui.toasts.length;
    wA.res(A);
    const rA = await pA;
    check("세대: A 걷기 중 " + kind + " 열기 → A 가 이기지 않음(폴더 상태·고르기 칸 그대로)", rT.ok && rA.stale && source.current().kind === kind && o.folderTable() === null &&
      ui.shown === false && !log.some(l => l.startsWith("setFolder")) && ui.toasts.length === toastsBefore, JSON.stringify({ rA, log, t: ui.toasts }));
  }
  {
    const { o, ui, log, source } = fakeOpener(V);
    const rF = deferred();
    const pF = o.text("느린 파일", rF.p, "file", () => "파일을 읽지 못했습니다");
    const rB = await o.folder(Promise.resolve(B));
    rF.res("# 늦음\n");
    const r = await pF;
    check("세대: 파일 읽기 중 폴더 열기 → 늦은 파일이 이기지 않음(고르기 칸 그대로)", rB.ok && r.stale && source.current().path === "b.md" && o.folderTable() === B && ui.shown &&
      !log.includes("clearFolder") && !log.some(l => l.startsWith("open:")), JSON.stringify({ r, log }));
    // 늦게 실패한 읽기도 토스트를 내지 않는다
    const rF2 = deferred();
    const pF2 = o.text("실패", rF2.p, "file", () => "파일을 읽지 못했습니다");
    const n = ui.toasts.length;
    await o.text("붙임", "# p\n", "paste");
    rF2.rej(new Error("x"));
    const r2 = await pF2;
    check("세대: 늦게 실패한 읽기는 토스트 없음", r2.stale && ui.toasts.length === n, JSON.stringify(ui.toasts.slice(n)));
    // 세대가 같으면 실패를 알리고, 앞 화면·폴더는 그대로
    await o.folder(Promise.resolve(A));
    const before = source.current();
    const r3 = await o.text("실패", Promise.reject(new Error("x")), "file", () => "파일을 읽지 못했습니다");
    check("원문 읽기 실패는 알리고 폴더 상태를 비우지 않음", !r3.ok && !r3.stale && ui.toasts[ui.toasts.length - 1] === "파일을 읽지 못했습니다" && o.folderTable() === A && ui.shown && before === source.current(), JSON.stringify(r3));
  }
  {
    // [3] 고르기 칸: 열 수 없는 문서를 고르면 지금 문서로 되돌리고, 세대를 올리지 않는다
    const { o, ui } = fakeOpener(V);
    const C = { name: "C", files: new Map([["a.md", new FakeFile("a.md", "# a")], ["big.md", new FakeFile("big.md", "x".repeat((8 << 20) + 1))]]), stopped: "" };
    await o.folder(Promise.resolve(C));
    ui.picked = "big.md"; // 사용자가 고르기 칸에서 고른 값(브라우저가 먼저 바꾼다)
    const rBig = await o.doc("big.md");
    check("고르기 실패 → 고르기 칸을 지금 문서로 되돌림", !rBig.ok && ui.picked === "a.md" && /너무 큽니다/.test(ui.toasts[ui.toasts.length - 1]), JSON.stringify({ rBig, p: ui.picked }));
    // 폴더 걷기 중 열 수 없는 문서를 골라도 그 걷기를 버리지 않는다(세대를 올리지 않음)
    const wB = deferred();
    const pB = o.folder(wB.p);
    await o.doc("big.md");
    wB.res(B);
    const rB = await pB;
    check("열 수 없는 문서 고르기는 진행 중인 폴더 열기를 버리지 않음", rB.ok && o.folderTable() === B, JSON.stringify(rB));
    // 원천이 실패를 돌려주면(읽기 실패 등) 되돌린다
    const { o: o2, ui: ui2, source: s2 } = fakeOpener(V);
    await o2.folder(Promise.resolve(A));
    s2.openFolder = () => Promise.resolve({ ok: false, msg: "파일을 읽지 못했습니다" });
    await o2.doc("a2.md");
    check("원천 실패 → 고르기 칸 되돌림", ui2.picked === "a.md" && ui2.toasts[ui2.toasts.length - 1] === "파일을 읽지 못했습니다", ui2.picked);
    // 늦게 실패한 문서 열기는 그 뒤에 연 것을 건드리지 않는다(되돌리기·토스트 없음)
    const late = deferred();
    s2.openFolder = () => late.p;
    const pLate = o2.doc("a2.md");
    await o2.text("붙임", "# p\n", "paste");
    const nt = ui2.toasts.length;
    ui2.picked = "그대로";
    late.res({ ok: false, msg: "파일을 읽지 못했습니다" });
    const rLate = await pLate;
    check("세대: 늦게 실패한 문서 열기는 고르기 칸·토스트를 건드리지 않음", rLate.stale && ui2.toasts.length === nt && ui2.picked === "그대로", JSON.stringify({ rLate, t: ui2.toasts.slice(nt), p: ui2.picked }));
    await o2.folder(Promise.resolve(A));
    // md 가 없는 폴더는 알리기만 하고 앞 폴더 그대로
    const before = o2.folderTable();
    const rN = await o2.folder(Promise.resolve(tableOf("N", ["x.txt"])));
    check("md 없는 폴더는 앞 폴더 그대로", !rN.ok && o2.folderTable() === before && /md 파일이 없습니다/.test(ui2.toasts[ui2.toasts.length - 1]));
  }

  // 리뷰 2회차 [6]: 폴더를 연 「뒤」 파일·붙여넣기·견본을 열면 고르기 칸이 숨고 폴더 상태가 빈다
  for (const kind of ["file", "paste", "sample"]) {
    const { o, ui, log, source } = fakeOpener(V);
    await o.folder(Promise.resolve(A));
    const was = ui.shown && o.folderTable() === A;
    const r = await o.text("새 " + kind, Promise.resolve("# t\n"), kind);
    check("폴더를 연 뒤 " + kind + " 열기 → 고르기 칸 숨김·폴더 상태 비움", was && r.ok && ui.shown === false && ui.list === null && o.folderTable() === null &&
      log.includes("clearFolder") && source.current().kind === kind, JSON.stringify({ was, r, log }));
  }
  // 리뷰 2회차 [2]·[3]: 읽는 중 고르기(가짜 원천이 늦게 푼다), 첫 문서만 못 연 폴더
  {
    const { o, ui, source, log } = fakeOpener(V);
    const C = { name: "C", files: new Map([["big.md", new FakeFile("big.md", "x".repeat((8 << 20) + 1))], ["p0.md", new FakeFile("p0.md", "# 0")], ["p1.md", new FakeFile("p1.md", "# 1")]]), stopped: "" };
    const before = source.current();
    const rC = await o.folder(Promise.resolve(C)); // 첫 문서(경로 순 big.md)가 너무 큼
    check("첫 문서만 못 연 폴더: 폴더는 열린 채 「다른 문서를 고르세요」", !rC.ok && o.folderTable() === C && ui.shown && ui.picked === "" && source.current() === before &&
      /너무 큽니다.* — 다른 문서를 고르세요$/.test(ui.toasts[ui.toasts.length - 1]), JSON.stringify({ rC, t: ui.toasts, p: ui.picked }));
    await o.doc("p0.md");
    const late = deferred();
    const realOpen = source.openFolder;
    source.openFolder = (folder, path) => late.p.then(r => { if (r.ok) realOpen(folder, path); return r; });
    const pP1 = o.doc("p1.md");
    ui.picked = "big.md"; // 사용자가 고르기 칸에서 고른 값
    await o.doc("big.md");
    const mid = ui.picked;
    late.res({ ok: true });
    const rP1 = await pP1;
    check("읽는 중 열 수 없는 문서를 고르면 진행 중인 문서로 되돌림", mid === "p1.md" && rP1.ok && source.current().path === "p1.md" && ui.picked === "p1.md", JSON.stringify({ mid, rP1, cur: source.current().path, log }));
    // 진행 중인 것이 끝나면 다시 지금 문서 기준
    source.openFolder = realOpen;
    ui.picked = "big.md";
    await o.doc("big.md");
    check("진행 중인 열기가 없으면 지금 문서로 되돌림", ui.picked === "p1.md", ui.picked);
    // 문서 고르기 실패는 「다른 문서를 고르세요」 를 붙이지 않는다(첫 문서만)
    check("고르기 실패 토스트에는 꼬리 없음", !/다른 문서를 고르세요/.test(ui.toasts[ui.toasts.length - 1]), ui.toasts[ui.toasts.length - 1]);
  }

  // droppedDir: webkitGetAsEntry 없음·여러 항목·폴더 없음
  const fileEn = { isFile: true, isDirectory: false, name: "a.md" };
  const dirEn = { isFile: false, isDirectory: true, name: "d" }, dirEn2 = { isFile: false, isDirectory: true, name: "e" };
  const item = en => ({ kind: "file", webkitGetAsEntry: () => en });
  let noEntry;
  try { noEntry = V.droppedDir({ items: [{ kind: "file" }] }); } catch (e) { noEntry = "throw: " + e.message; }
  check("droppedDir: webkitGetAsEntry 없음 → null", noEntry === null, noEntry);
  check("droppedDir: 여러 항목 → 첫 폴더", V.droppedDir({ items: [item(fileEn), { kind: "string" }, item(dirEn), item(dirEn2)] }) === dirEn);
  check("droppedDir: 폴더 없음 → null", V.droppedDir({ items: [item(fileEn), item(null)] }) === null);
  check("droppedDir: items 없음 → null", V.droppedDir({}) === null && V.droppedDir(null) === null);

  // 대소문자 실마리: 표에 대소문자만 다른 파일이 꼭 하나 있으면 경고에 덧붙인다(그리지는 않는다)
  const cf = new Map([["img/Logo.png", new FakeFile("Logo.png", "p")], ["A.png", new FakeFile("A.png", "p")], ["a.PNG", new FakeFile("a.PNG", "p")]]);
  const ci = V.folderImages('<img src="img/logo.png"><img src="a.png">', "d.md", cf, () => "blob:x");
  // caseTwin 색인은 표마다다 — 두 표가 서로의 색인을 쓰지 않는다
  const cfA = new Map([["img/Logo.png", new FakeFile("Logo.png", "p")]]), cfB = new Map([["img/LOGO.png", new FakeFile("LOGO.png", "p")]]);
  const twA = V.folderImages('<img src="img/logo.png">', "d.md", cfA, () => "blob:x").missing.concat({})[0].caseTwin;
  const twB = V.folderImages('<img src="img/logo.png">', "d.md", cfB, () => "blob:x").missing.concat({})[0].caseTwin;
  const twA2 = V.folderImages('<img src="img/logo.png">', "d.md", cfA, () => "blob:x").missing.concat({})[0].caseTwin;
  check("caseTwin 색인은 표마다", twA === "img/Logo.png" && twB === "img/LOGO.png" && twA2 === "img/Logo.png", JSON.stringify([twA, twB, twA2]));
  const cw = V.warningLines({ warnings: [] }, ci);
  check("대소문자 실마리(하나일 때만, 그리지 않음)", ci.drawn === 0 && ci.missing[0].caseTwin === "img/Logo.png" && !ci.missing[1].caseTwin &&
    cw[0].text.includes("img/logo.png(대소문자만 다른 img/Logo.png 가 있음)") && cw[0].text.includes("a.png") && !cw[0].text.includes("a.png(대소문자"), JSON.stringify({ m: ci.missing, t: cw[0].text }));
}

// ---- 폴더 열기(itda-doc#27): 경로 풀기·표 만들기·메타 찾기·그림 바꾸기·entry 걷기·경합. 브라우저 대신 가짜 File·entry·handle 을 쓴다.

// FakeFile 은 File 의 쓰는 부분(name·size·text·webkitRelativePath)이다. text 는 reads 를 센다(표를 만들 때 내용을 읽지 않는지).
let reads = 0;
class FakeFile {
  constructor(name, content, rel) { this.name = name; this.content = content == null ? "" : content; this.size = Buffer.byteLength(this.content); this.webkitRelativePath = rel || ""; }
  text() { reads++; return this.fail ? Promise.reject(new Error("읽기 실패")) : this.gate ? this.gate.then(() => this.content) : Promise.resolve(this.content); }
}
// tree 는 {이름: 글 | 하위 tree} 이다. entryOf 는 FileSystemEntry 흉내 — readEntries 는 한 번에 batch 개까지만 주고 다 주면 빈 배열이다.
function entryOf(name, node, stat, batch) {
  if (typeof node === "string") return { name, isFile: true, isDirectory: false, file: (ok) => ok(new FakeFile(name, node)) };
  return { name, isFile: false, isDirectory: true, createReader() {
    const kids = Object.keys(node).map(k => entryOf(k, node[k], stat, batch));
    let at = 0;
    return { readEntries(ok) { stat.calls++; const b = kids.slice(at, at + batch); at += b.length; setTimeout(() => ok(b), 0); } };
  } };
}
function handleOf(name, node) {
  if (typeof node === "string") return { kind: "file", name, getFile: () => Promise.resolve(new FakeFile(name, node)) };
  return { kind: "directory", name, async *entries() { for (const k of Object.keys(node)) yield [k, handleOf(k, node[k])]; } };
}
function mapOf(obj) { return new Map(Object.entries(obj).map(([k, v]) => [k, v instanceof FakeFile ? v : new FakeFile(k.split("/").pop(), v)])); }
const sameKeys = (m, want) => JSON.stringify([...m.keys()].sort()) === JSON.stringify([...want].sort());

async function folderChecks(V, doc, api, shown) {
  // 로더: folderMeta 는 [{rel, yaml, err}] 만 받고(throw), 받은 것은 그대로 itdaDoc 으로 넘긴다
  throwsType("render folderMeta 가 배열 아님", () => doc.render("x", { folderMeta: { rel: "a" } }));
  throwsType("render folderMeta 항목의 모르는 키", () => doc.render("x", { folderMeta: [{ rel: "_metadata.yml", yml: "a" }] }));
  throwsType("render folderMeta 항목에 rel 없음", () => doc.render("x", { folderMeta: [{ yaml: "a" }] }));
  throwsType("render folderMeta 항목 꼴 틀림", () => doc.render("x", { folderMeta: [{ rel: "_metadata.yml", yaml: 5 }] }));
  throwsType("render folderMeta 항목이 null", () => doc.render("x", { folderMeta: [null] }));
  let vf = {};
  try { vf = doc.render("# 제목\n", { folderMeta: [{ rel: "../_metadata.yml", yaml: "theme: mono\n" }, { rel: "_metadata.yml", err: "읽지 못함" }] }); } catch (e) { vf = { error: "throw: " + e.message }; }
  check("로더가 folderMeta 를 넘김", !vf.error && vf.theme === "mono" && JSON.stringify(vf.folder_meta) === '["../_metadata.yml"]' && vf.doc_theme_from === "../_metadata.yml" &&
    (vf.warnings || []).some(w => w.Message === "_metadata.yml: 읽지 못함 — 건너뜀"), JSON.stringify({ e: vf.error, t: vf.theme, f: vf.folder_meta, w: vf.warnings }));

  // resolveImgPath — mdoc 이 낸 src(HTML 이스케이프·퍼센트 인코딩)를 문서 위치 기준으로 푼다
  const R = (src, doc) => JSON.stringify(V.resolveImgPath(src, doc));
  const rs = [
    ["%EA%B3%B5%EB%B0%B1%20%ED%8C%8C%EC%9D%BC.png", "docs/a.md", { path: "docs/공백 파일.png" }],
    ["./sub/a&amp;b.png?x=1#y", "a.md", { path: "sub/a&b.png" }],
    ["../up%20x.svg", "docs/a.md", { path: "up x.svg" }],
    ["a%23b.png", "a.md", { path: "a#b.png" }],
    ["%E0%A4%A.png", "a.md", { path: "%E0%A4%A.png" }],
    ["x/../../y.png", "d/a.md", { path: "y.png" }],
    ["../../x.png", "docs/a.md", { why: "outside" }],
    ["/abs.png", "docs/a.md", { why: "absolute" }],
    ["https://example.com/a.png", "a.md", { why: "remote" }],
    ["//example.com/a.png", "a.md", { why: "remote" }],
  ];
  for (const [src, d, want] of rs) {
    const got = V.resolveImgPath(src, d);
    check("resolveImgPath " + src, Object.keys(want).every(k => got[k] === want[k]), R(src, d));
  }
  // NFC: macOS 이름은 NFD 일 수 있다 — 표의 열쇠도 src 도 NFC 로 맞춰 비교한다
  const nfd = "그림 한글.png".normalize("NFD");
  check("NFD 와 NFC 는 다른 글자(시험 전제)", nfd !== "그림 한글.png");
  const tl = V.tableFromList([new FakeFile(nfd, "png", "뿌리/" + nfd), new FakeFile("a.md", "# a", "뿌리/a.md")]);
  check("tableFromList 는 열쇠를 NFC 로", tl.name === "뿌리" && tl.files.has("그림 한글.png") && tl.files.size === 2, [...tl.files.keys()].join("|"));
  let made = 0;
  const urlFor = (p, f, type) => "blob:fake/" + (++made) + "/" + type;
  for (const [label, src] of [["NFC src", encodeURIComponent("그림 한글.png")], ["NFD src", encodeURIComponent(nfd)]]) {
    const r = V.folderImages('<img src="' + src + '" alt="">', "a.md", tl.files, urlFor);
    check("folderImages " + label + " 를 찾음", r.drawn === 1 && /src="blob:fake\/\d+\/image\/png"/.test(r.html), JSON.stringify(r));
  }

  // folderImages 의 갈래: 그림(blob)·SVG 형식·허용 목록 밖·없음·뿌리 밖·/ 로 시작·원격·data:
  const files = mapOf({ "d/a.png": "p", "logo.SVG": "<svg/>", "d/x.tiff": "t" });
  const html = '<img src="a.png" alt=""><img src="../logo.SVG"><img src="x.tiff"><img src="%EC%97%86%EC%9D%8C.png"><img src="../../o.png"><img src="/r.png"><img src="https://e.x/a.png"><img src="data:image/png;base64,AA">';
  const fi = V.folderImages(html, "d/doc.md", files, urlFor);
  check("folderImages 갈래", fi.drawn === 2 && fi.remote === 1 && fi.local === 4 && fi.html.includes("/image/svg+xml") &&
    JSON.stringify(fi.missing.map(m => m.why)) === '["type","missing","outside","absolute"]' && fi.missing[1].shown === "d/없음.png" &&
    fi.html.includes('data-local-src="x.tiff"') && fi.html.includes('data-local-src="https://e.x/a.png"') && fi.html.includes('src="data:image/png;base64,AA"') &&
    !/<img\b[^>]*\ssrc="(?!data:|blob:)/.test(fi.html), JSON.stringify(fi));
  const fw = V.warningLines({ warnings: [] }, fi);
  check("폴더 경고 줄: 없는 그림 수와 경로·형식", fw.length === 3 && fw[0].code === "viewer-local-image" && fw[0].text.startsWith("폴더에 없는 그림 3개") &&
    fw[0].text.includes("d/없음.png") && fw[0].text.includes("(연 폴더 밖)") && fw[1].code === "viewer-image-type" && fw[1].text.includes("d/x.tiff") && fw[2].code === "viewer-remote-image", JSON.stringify(fw));
  const many = V.warningLines({ warnings: [] }, V.folderImages('<img src="1.png"><img src="2.png"><img src="3.png"><img src="4.png"><img src="5.png">', "a.md", new Map(), urlFor));
  check("폴더 경고 줄: 앞 세 개 뒤는 수만", many[0].text.includes("1.png, 2.png, 3.png 외 2개"), many[0].text);

  // tableFromList: . 폴더·node_modules 제외(. 파일은 넣는다), 깊이·항목 상한
  const deepRel = "r/" + Array.from({ length: 65 }, (_, i) => "d" + i).join("/") + "/deep.md";
  const tl2 = V.tableFromList([["r/a.md"], ["r/.git/x.md"], ["r/s/.cache/y.md"], ["r/node_modules/z.md"], ["r/s/node_modules2/ok.md"], ["r/.env"], [deepRel], ["r/" + Array.from({ length: 64 }, (_, i) => "d" + i).join("/") + "/at64.md"]]
    .map(([p]) => new FakeFile(p.split("/").pop(), "x", p)));
  check("tableFromList 제외·깊이", sameKeys(tl2.files, ["a.md", "s/node_modules2/ok.md", ".env", Array.from({ length: 64 }, (_, i) => "d" + i).join("/") + "/at64.md"]) && tl2.stopped === "depth", [...tl2.files.keys()].join("|") + " " + tl2.stopped);
  const tl3 = V.tableFromList(["1.md", "2.md", "3.md", "4.md"].map(n => new FakeFile(n, "x", "r/" + n)), { entries: 3 });
  check("tableFromList 항목 상한", tl3.files.size === 3 && tl3.stopped === "entries", tl3.files.size + " " + tl3.stopped);

  // walkEntries: readEntries 는 100개씩만 준다 — 빈 배열까지 되풀이해야 250개를 모두 본다
  const big = {};
  for (let i = 0; i < 250; i++) big["f" + String(i).padStart(3, "0") + ".md"] = "# " + i;
  const tree = { "index.md": "# i", big, ".git": { "HEAD.md": "x" }, node_modules: { "m.md": "x" }, sub: { "_metadata.yml": "theme: mono\n", deeper: { "c.md": "c" } } };
  const stat = { calls: 0 };
  reads = 0;
  const we = await V.walkEntries(entryOf("뿌리", tree, stat, 100));
  check("walkEntries 가 readEntries 를 빈 배열까지 되풀이(100개 단위)", we.files.size === 253 && we.files.has("big/f249.md") && we.files.has("sub/deeper/c.md") && !we.files.has(".git/HEAD.md") && !we.files.has("node_modules/m.md") && we.stopped === "" && we.name === "뿌리",
    we.files.size + " " + stat.calls + " " + we.stopped);
  // 뿌리 1+빈 1, big 100·100·50+빈 1, sub 1+빈 1, deeper 1+빈 1 — .git·node_modules 에는 들어가지 않는다
  check("walkEntries readEntries 호출 수", stat.calls === 10, stat.calls);
  check("표를 만드는 동안 파일 내용을 읽지 않음", reads === 0, reads);
  const we2 = await V.walkEntries(entryOf("r", { a: { b: { c: { "x.md": "x" } } }, "y.md": "y" }, { calls: 0 }, 100), { depth: 2 });
  check("walkEntries 깊이 상한은 그 갈래만 건너뛰고 알림", sameKeys(we2.files, ["y.md"]) && we2.stopped === "depth", [...we2.files.keys()].join("|") + " " + we2.stopped);
  const we3 = await V.walkEntries(entryOf("r", big, { calls: 0 }, 100), { entries: 120 });
  check("walkEntries 항목 상한은 멈추고 알림", we3.files.size === 120 && we3.stopped === "entries", we3.files.size + " " + we3.stopped);
  const wh = await V.walkHandle(handleOf("뿌리", tree));
  check("walkHandle 이 같은 표", sameKeys(wh.files, [...we.files.keys()]) && wh.stopped === "", wh.files.size);
  const wh2 = await V.walkHandle(handleOf("r", { a: { b: { c: { "x.md": "x" } } }, "y.md": "y" }), { depth: 2 });
  check("walkHandle 깊이 상한", sameKeys(wh2.files, ["y.md"]) && wh2.stopped === "depth", [...wh2.files.keys()].join("|"));

  // docList·firstDoc
  const dl = V.docList(mapOf({ "b.md": "", "README.md": "", "a/index.md": "", "x.markdown": "", "z.txt": "", "c.MD": "" }));
  check("docList 는 md·markdown 을 경로 순으로", JSON.stringify(dl) === '["README.md","a/index.md","b.md","c.MD","x.markdown"]', JSON.stringify(dl));
  check("firstDoc: 뿌리 index.md 가 없으면 README.md(하위 폴더 index.md 는 아님)", V.firstDoc(dl) === "README.md", V.firstDoc(dl));
  check("firstDoc: 뿌리 Index.MD 가 먼저", V.firstDoc(["Index.MD", "readme.md", "a.md"]) === "Index.MD");
  check("firstDoc: 없으면 첫 항목", V.firstDoc(["a/x.md", "b.md"]) === "a/x.md" && V.firstDoc([]) === "");

  // metaChain: 문서 폴더에서 뿌리까지, 먼 것부터, rel 은 문서 기준
  const mf = mapOf({ "_metadata.yml": "", "a/_metadata.yml": "", "a/b/c/_metadata.yml": "", "x/_metadata.yml/y": "" });
  const mc = V.metaChain("a/b/doc.md", mf);
  check("metaChain 먼 것부터·rel", JSON.stringify(mc) === JSON.stringify([{ rel: "../../_metadata.yml", path: "_metadata.yml" }, { rel: "../_metadata.yml", path: "a/_metadata.yml" }]), JSON.stringify(mc));
  check("metaChain 뿌리 문서", JSON.stringify(V.metaChain("doc.md", mf)) === JSON.stringify([{ rel: "_metadata.yml", path: "_metadata.yml" }]));
  check("metaChain 폴더 이름 _metadata.yml 은 파일이 아님", JSON.stringify(V.metaChain("x/d.md", mf)) === JSON.stringify([{ rel: "../_metadata.yml", path: "_metadata.yml" }]));

  // readFolderMeta: 64KB 상한·읽기 실패는 err 만(page.ReadMetaFile 문구)
  const bigMeta = new FakeFile("_metadata.yml", "a".repeat(64 * 1024 + 1));
  const okMeta = new FakeFile("_metadata.yml", "a".repeat(64 * 1024));
  const badMeta = new FakeFile("_metadata.yml", "theme: mono\n");
  badMeta.fail = true;
  const rm = await V.readFolderMeta([{ rel: "../../_metadata.yml", path: "big" }, { rel: "../_metadata.yml", path: "bad" }, { rel: "_metadata.yml", path: "ok" }],
    new Map([["big", bigMeta], ["bad", badMeta], ["ok", okMeta]]));
  check("readFolderMeta 상한·실패·순서", rm[0].err === "파일이 너무 큼(64KB 넘음)" && rm[0].yaml === "" && rm[1].err === "읽지 못함" && rm[2].err === "" && rm[2].yaml.length === 65536 &&
    rm.map(m => m.rel).join() === "../../_metadata.yml,../_metadata.yml,_metadata.yml", JSON.stringify(rm.map(m => [m.rel, m.err, m.yaml.length])));

  // 뷰어 원천의 폴더 열기: _metadata.yml 이 네이티브 규칙(=wasm render 의 folderMeta)대로 얹히고, 그림이 blob: 로 바뀐다
  let urlMade = 0;
  const revoked = [];
  const views = [];
  const fsrc = V.viewerSource({ load: () => Promise.resolve(doc), setThemeCSS: () => Promise.resolve(), unscoped: () => false,
    onView: (v, lines, cur) => views.push({ v, lines, kind: cur.kind }), objectURL: (f, type) => "blob:fake/" + (++urlMade) + "/" + type, revokeURL: u => revoked.push({ u, shown: fshown.length }) });
  const fshown = [];
  const fl = fsrc.attach({ show: v => { fshown.push(v); return Promise.resolve(); }, failed() {}, toast() {} });
  fl.start();
  for (let i = 0; i < 100 && !fsrc.doc(); i++) await new Promise(r => setTimeout(r, 5));
  const docSrc = "---\ntitle: 보고\n---\n\n# 보고\n\n![로고](../logo.svg) ![사진](<" + nfd + ">) ![없음](없음.png) ![그림 둘](../logo.svg)\n";
  const folder = { name: "뿌리", files: mapOf({ "_metadata.yml": "theme: mono\norganization: 잇다\n", "sub/_metadata.yml": "author: 홍길동\nlist-style: gaejosik\n",
    "sub/doc.md": docSrc, "sub/other.md": "# 다른\n\n![로고](../logo.svg)\n", "logo.svg": "<svg xmlns=\"http://www.w3.org/2000/svg\"/>", ["sub/" + nfd.normalize("NFC")]: "png", "top.md": "# 위\n" }) };
  const r1 = await fsrc.openFolder(folder, "sub/doc.md");
  const fv = views[views.length - 1];
  const want = doc.render(docSrc, { folderMeta: [{ rel: "../_metadata.yml", yaml: "theme: mono\norganization: 잇다\n", err: "" }, { rel: "_metadata.yml", yaml: "author: 홍길동\nlist-style: gaejosik\n", err: "" }] });
  check("openFolder → 폴더 기본값이 얹힌 View(render 의 folderMeta 와 같음, html 의 그림만 다름)", r1.ok && fv.kind === "folder" && fv.v.theme === "mono" && fv.v.list_style === "gaejosik" &&
    JSON.stringify(fv.v.folder_meta) === '["_metadata.yml","../_metadata.yml"]' && JSON.stringify({ ...fv.v, html: "", version: 0 }) === JSON.stringify({ ...want, html: "", version: 0 }),
    JSON.stringify({ r1, t: fv.v.theme, fm: fv.v.folder_meta, e: fv.v.error }));
  check("openFolder 그림: 있는 것 blob(같은 그림은 URL 하나)·없는 것 경고", urlMade === 2 && (fv.v.html.match(/src="blob:fake\/1\/image\/svg\+xml"/g) || []).length === 2 && fv.v.html.includes("/image/png") &&
    fv.lines.some(l => l.code === "viewer-local-image" && l.text.startsWith("폴더에 없는 그림 1개") && l.text.includes("sub/없음.png")), urlMade + " " + JSON.stringify(fv.lines));
  await fsrc.setTheme("spine");
  check("같은 문서 다시 그리기(테마)는 그림 URL 재사용·거두지 않음, 폴더 설정은 그대로", urlMade === 2 && revoked.length === 0 && fshown[fshown.length - 1].theme === "spine" &&
    JSON.stringify(fshown[fshown.length - 1].folder_meta) === '["_metadata.yml","../_metadata.yml"]', urlMade + " " + revoked.length);
  await fsrc.setTheme("");
  await fsrc.openFolder(folder, "sub/other.md");
  // 거두기는 새 문서를 그린(show) 뒤다 — 그리는 동안 앞 화면의 그림이 깨지지 않게
  check("문서를 바꾸면 앞 그림 URL 을 새 문서를 그린 뒤 거둠", revoked.length === 2 && urlMade === 3 && revoked.every(r => r.shown === fshown.length && /^blob:fake\/[12]\//.test(r.u)), JSON.stringify(revoked) + " " + fshown.length);
  await fsrc.openFolder(folder, "top.md");
  check("뿌리 문서는 뿌리 _metadata.yml 만", JSON.stringify(views[views.length - 1].v.folder_meta) === '["_metadata.yml"]' && revoked.length === 3, JSON.stringify(views[views.length - 1].v.folder_meta));
  await fsrc.open("하나", docSrc, "file");
  const sv = views[views.length - 1];
  check("파일 하나로 열면 폴더 상태를 비움(폴더 설정·그림 없음)", sv.kind === "file" && !sv.v.folder_meta && sv.v.theme !== "mono" && !/src="blob:/.test(sv.v.html) &&
    sv.lines.some(l => l.code === "viewer-local-image" && l.text.includes("4개") && l.text.includes("폴더로 열면 그린다")), JSON.stringify(sv.lines));
  const rbig = await fsrc.openFolder({ files: new Map([["big.md", new FakeFile("big.md", "x".repeat((8 << 20) + 1))]]) }, "big.md");
  check("폴더 문서도 크기 상한(8 MB)", !rbig.ok && /너무 큽니다/.test(rbig.msg), JSON.stringify(rbig));

  // 경합: 앞선 문서 읽기가 나중에 끝나도 이기지 않는다(openFolder 끼리, openFolder 뒤 open)
  let release;
  const slow = new FakeFile("slow.md", "# 느림\n");
  slow.gate = new Promise(r => { release = r; });
  const race = { name: "r", files: new Map([["slow.md", slow], ["fast.md", new FakeFile("fast.md", "# 빠름\n")]]) };
  const pSlow = fsrc.openFolder(race, "slow.md");
  const rFast = await fsrc.openFolder(race, "fast.md");
  release();
  const rSlow = await pSlow;
  check("경합: 나중에 연 문서가 이김", rFast.ok && rSlow.stale && fsrc.current().path === "fast.md" && fshown[fshown.length - 1].title === "fast", JSON.stringify({ rSlow, cur: fsrc.current().path, t: fshown[fshown.length - 1].title }));
  const slow2 = new FakeFile("slow.md", "# 느림\n");
  let release2;
  slow2.gate = new Promise(r => { release2 = r; });
  const pSlow2 = fsrc.openFolder({ files: new Map([["slow.md", slow2]]) }, "slow.md");
  await fsrc.open("붙여넣은 문서", "# 붙임\n", "paste");
  release2();
  const rSlow2 = await pSlow2;
  check("경합: 폴더 읽기 중 붙여넣기가 이김", rSlow2.stale && fsrc.current().kind === "paste" && fshown[fshown.length - 1].title === "붙여넣은 문서", JSON.stringify(rSlow2));
  // [3] 열 수 없는 문서(크기·없음)는 seq 를 올리지 않는다 — 진행 중인 다른 문서 열기를 버리지 않는다
  const slow3 = new FakeFile("slow.md", "# 느림\n");
  let release3;
  slow3.gate = new Promise(r => { release3 = r; });
  const f3 = { files: new Map([["slow.md", slow3], ["big.md", new FakeFile("big.md", "x".repeat((8 << 20) + 1))]]) };
  const pSlow3 = fsrc.openFolder(f3, "slow.md");
  const rBig3 = await fsrc.openFolder(f3, "big.md");
  const rNone3 = await fsrc.openFolder(f3, "none.md");
  release3();
  const rSlow3 = await pSlow3;
  check("열 수 없는 문서는 진행 중인 열기를 버리지 않음", !rBig3.ok && !rNone3.ok && /찾지 못했습니다/.test(rNone3.msg) && rSlow3.ok && fsrc.current().path === "slow.md", JSON.stringify({ rBig3, rNone3, rSlow3 }));
  check("folderDocError", V.folderDocError(f3, "big.md").includes("너무 큽니다") && V.folderDocError(f3, "x.md") === "문서를 찾지 못했습니다" && V.folderDocError(f3, "slow.md") === "");

  await openerChecks(V);

  // 폴더 열기로 그린 html 에도 data:·blob: 이 아닌 src 가 없다
  const fleaks = fshown.filter(v => /<img\b[^>]*\ssrc="(?!data:|blob:)/i.test(v.html)).length;
  check("폴더 show 직전 html 에 원격·상대 src 없음", fshown.length >= 6 && fleaks === 0, fshown.length + " " + fleaks);
}
