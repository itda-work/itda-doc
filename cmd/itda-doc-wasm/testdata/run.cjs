// wasm 렌더 입구를 node 에서 돌린다(wasm_test.go). 브라우저처럼 temml.min.js 를 전역 스크립트로 올린 뒤 itda-doc.wasm 을 올리고,
// 사례 파일의 입력마다 itdaDoc 을 불러 결과를 JSON 으로 쓴다.
//   node run.cjs <wasm_exec.js> <temml.min.js> <itda-doc.wasm> <cases.json> <out.json>
//
// wasm 에 node 의 진짜 fs 를 주지 않는다 — wasm_exec.js 의 기본 받침(쓰기는 console, 나머지는 ENOSYS — 브라우저와 같다)을 쓴다.
// 그래서 wasm 이 파일을 읽으려 하면 node 에서도 실패해 「파일 접근 없음」이 이 시험에 묶인다. 이 스크립트 자신의 파일 읽기·쓰기는
// require("fs") 를 지역 변수로만 쓴다.
"use strict";
const nodeFS = require("fs");
const vm = require("vm");
const [wasmExec, temmlJS, wasmPath, casesPath, outPath] = process.argv.slice(2);

if (globalThis.fs) throw new Error("globalThis.fs 가 이미 있음 — wasm 이 진짜 파일 시스템을 보게 된다");
globalThis.TextEncoder ??= require("util").TextEncoder;
globalThis.TextDecoder ??= require("util").TextDecoder;
globalThis.performance ??= require("perf_hooks").performance;
globalThis.crypto ??= require("crypto");
require(wasmExec);

// <script src="temml.min.js"> 와 같은 꼴: 맨 위 var temml 이 전역이 된다
vm.runInThisContext(nodeFS.readFileSync(temmlJS, "utf8"), { filename: "temml.min.js" });
const temml = globalThis.temml;

// 사례의 temml 모드: 그 렌더 동안만 globalThis.temml 을 바꾼다(var 로 만든 전역은 delete 되지 않아 값을 바꾼다)
const temmlModes = {
  none: () => undefined,
  notFunc: () => ({ renderToString: 1 }),
  nonString: () => ({ renderToString: () => 42 }),
  throwString: () => ({ renderToString: () => { throw "Undefined control sequence: \\zzthrown"; } }),
  throwNumber: () => ({ renderToString: () => { throw 42; } }),
  throwBare: () => ({ renderToString: () => { throw Object.create(null); } }),
  throwError: () => ({ renderToString: () => { throw new Error("Temml loading"); } }),
  throwParse: () => ({ renderToString: () => { const e = new Error("ParseError: Undefined control sequence: \\zzparse"); e.name = "ParseError"; throw e; } }),
};

(async () => {
  const go = new Go();
  const { instance } = await WebAssembly.instantiate(nodeFS.readFileSync(wasmPath), go.importObject);
  go.run(instance); // main 은 select{} 로 머문다 — 기다리지 않는다
  for (let i = 0; i < 100 && !globalThis.itdaDoc; i++) await new Promise((r) => setTimeout(r, 10));
  if (!globalThis.itdaDoc) throw new Error("itdaDoc 이 올라오지 않음");
  const doc = globalThis.itdaDoc;
  const cases = JSON.parse(nodeFS.readFileSync(casesPath, "utf8"));
  const out = { renders: [], css: [], themes: doc.themes(), version: doc.version };
  for (const c of cases.renders) {
    if (c.temml) globalThis.temml = temmlModes[c.temml]();
    const t0 = performance.now();
    const view = "rawSrc" in c ? doc.render(c.rawSrc, c.opts) : doc.render(c.src, c.opts);
    const ms = performance.now() - t0;
    globalThis.temml = temml;
    out.renders.push({ name: c.name, view, ms });
  }
  for (const c of cases.css) out.css.push(doc.themeCSS(c.name, c.opts));
  nodeFS.writeFileSync(outPath, JSON.stringify(out));
  process.exit(0);
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
