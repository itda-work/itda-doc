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
  check("경고 목록에 상대 경로 그림 한 줄", lines.some(l => l.code === "viewer-local-image" && l.text.includes("1개")), JSON.stringify(lines));
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
  check("경고 줄: 상대·원격", lw.length === 2 && lw[0].code === "viewer-local-image" && lw[1].code === "viewer-remote-image", JSON.stringify(lw));

  // 견본: 빌드가 싣는 견본이 오류 없이 그려진다
  const samples = JSON.parse(nodeFS.readFileSync(samplesPath, "utf8"));
  const out = [];
  for (const s of samples) out.push({ name: s.name, view: doc.render(nodeFS.readFileSync(s.path, "utf8")) });
  nodeFS.writeFileSync(outPath, JSON.stringify({ checks, samples: out }));
  process.exit(0);
})().catch(e => { console.error(e); process.exit(1); });
