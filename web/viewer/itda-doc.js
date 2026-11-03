// itda-doc.js — 브라우저용 렌더 입구(cmd/itda-doc-wasm 의 globalThis.itdaDoc)를 감싸는 얇은 로더(itda-work/itda-hyve#577 2b).
// 페이지는 wasm_exec.js → temml.min.js → 이 파일 순으로 올린 뒤 ItdaDocLoader.load({ wasm: "itda-doc.wasm" }) 를 부른다.
//
//   ItdaDocLoader.load(opts) → Promise<doc>   opts: { wasm(주소 — 기본 "itda-doc.wasm"), fetch(기본 globalThis.fetch), timeoutMs(기본 5000) }
//   ItdaDocLoader.wrap(itdaDoc) → doc         이미 올린 itdaDoc 을 감싼다(node 시험이 쓴다)
//   doc.render(src, opts) → View 객체          opts: { theme(글자), allowRemoteImages(참거짓), dropNotes(참거짓) } — 생략 가능
//   doc.themes() → [ThemeInfo]                 page.Themes.List
//   doc.themeCSS(name, opts) → 글자 | null      opts: { unscoped(참거짓) }. 없는 테마 이름이면 null
//   doc.version → {commit, go}
//
// 옵션 오류는 throw 한다(TypeError — 2a 의 열린 결정 4 (b)): 객체가 아닌 옵션, 모르는 키, 꼴이 틀린 값, 글자가 아닌 원문·테마 이름.
// itdaDoc 자신은 render 의 옵션 오류를 View.error 로, themeCSS 의 옵션 오류를 null 로 돌려준다(Go 콜백은 JS 예외를 던지지 못한다 —
// cmd/itda-doc-wasm/main.go). 그래서 로더가 부르기 전에 본다. 키는 정확히 이 이름이어야 한다(itdaDoc 은 encoding/json 이라 대소문자를
// 가리지 않지만 로더는 가린다 — 더 좁다). 원문을 그리다 생긴 오류(렌더하지 못함 등)는 그대로 View.error 다.
//
// 이 파일은 네트워크로 문서를 보내지 않는다. 쓰는 요청은 wasm 받기(GET) 하나다.
(function (g) {
  "use strict";

  const RENDER_KEYS = { theme: "string", allowRemoteImages: "boolean", dropNotes: "boolean" };
  const CSS_KEYS = { unscoped: "boolean" };
  const LOAD_KEYS = { wasm: "string", fetch: "function", timeoutMs: "number" };

  // checkOpts 는 옵션 객체를 본다 — 없으면(undefined·null) 빈 객체, 틀리면 TypeError.
  function checkOpts(where, opts, keys) {
    if (opts === undefined || opts === null) return {};
    if (typeof opts !== "object" || Array.isArray(opts)) throw new TypeError(where + ": 옵션은 객체다(받은 것: " + (Array.isArray(opts) ? "array" : typeof opts) + ")");
    const out = {};
    for (const k of Object.keys(opts)) {
      if (!Object.prototype.hasOwnProperty.call(keys, k)) throw new TypeError(where + ": 모르는 옵션 " + JSON.stringify(k));
      const v = opts[k];
      if (v === undefined) continue;
      if (typeof v !== keys[k]) throw new TypeError(where + ": 옵션 " + k + " 은 " + keys[k] + " 이다(받은 것: " + typeof v + ")");
      out[k] = v;
    }
    return out;
  }

  function wrap(raw) {
    if (!raw || typeof raw.render !== "function" || typeof raw.themes !== "function" || typeof raw.themeCSS !== "function") {
      throw new TypeError("itdaDoc 이 아님(render·themes·themeCSS 가 없음)");
    }
    return {
      render(src, opts) {
        if (typeof src !== "string") throw new TypeError("render: 원문은 문자열이다(받은 것: " + typeof src + ")");
        const o = checkOpts("render", opts, RENDER_KEYS);
        return JSON.parse(raw.render(src, JSON.stringify(o)));
      },
      themes() { return JSON.parse(raw.themes()); },
      themeCSS(name, opts) {
        if (typeof name !== "string") throw new TypeError("themeCSS: 테마 이름은 문자열이다(받은 것: " + typeof name + ")");
        const o = checkOpts("themeCSS", opts, CSS_KEYS);
        const css = raw.themeCSS(name, JSON.stringify(o));
        return typeof css === "string" ? css : null;
      },
      version: raw.version,
    };
  }

  // load 는 wasm 을 받아 올리고 itdaDoc 이 생기면 감싸 돌려준다. wasm_exec.js(전역 Go)가 먼저 올라 있어야 한다.
  // 서버가 application/wasm 으로 내면 instantiateStreaming, 아니면 바이트를 다 받아 instantiate 한다.
  async function load(opts) {
    const o = checkOpts("load", opts, LOAD_KEYS);
    const url = o.wasm || "itda-doc.wasm";
    const fetchFn = o.fetch || g.fetch;
    if (typeof g.Go !== "function") throw new Error("wasm_exec.js 를 먼저 올려야 한다(전역 Go 가 없음)");
    if (typeof fetchFn !== "function") throw new Error("fetch 가 없음");
    const go = new g.Go();
    const r = await fetchFn(url);
    if (!r || !r.ok) throw new Error("wasm 을 받지 못함: " + url + " (" + (r ? r.status : "응답 없음") + ")");
    const type = r.headers && typeof r.headers.get === "function" ? String(r.headers.get("content-type") || "") : "";
    let inst;
    if (typeof WebAssembly.instantiateStreaming === "function" && /^application\/wasm\b/i.test(type)) {
      inst = (await WebAssembly.instantiateStreaming(r, go.importObject)).instance;
    } else {
      inst = (await WebAssembly.instantiate(await r.arrayBuffer(), go.importObject)).instance;
    }
    go.run(inst); // main 은 itdaDoc 을 두고 select{} 로 머문다 — 돌려준 Promise 는 기다리지 않는다
    const until = Date.now() + (o.timeoutMs || 5000);
    while (!g.itdaDoc) {
      if (Date.now() > until) throw new Error("itdaDoc 이 올라오지 않음");
      await new Promise(res => setTimeout(res, 10));
    }
    return wrap(g.itdaDoc);
  }

  g.ItdaDocLoader = { load, wrap, checkOpts };
})(globalThis);
