// 설치 없는 웹 뷰어(itda-work/itda-hyve#577 2b). 그리기는 공유 화면과 한 코드다 — share.js 의 화면(render·draw·보기 꼴·목차·노트·
// 쪽 넘김 키)에 View 원천만 바꿔 끼운다: 이 파일이 share.js 보다 먼저 window.ItdaViewSource 를 두고(body 에 data-base 가 없다 —
// share.js pickSource), 원천은 파일 열기(고르기·끌어놓기)·붙여넣기·견본 글을 wasm(itda-doc.js — ItdaDocLoader)으로 그린 View 다.
//
// 문서는 브라우저를 떠나지 않는다: 원문은 FileReader(file.text)·붙여넣기 칸에서만 읽고, 이 파일이 쓰는 요청은 견본 목록·견본 글
// (samples/ — 같은 출처의 정적 파일, GET)뿐이다(wasm 받기는 itda-doc.js). index.html 의 CSP 는 제3자 출처로 가는 요청(connect·그림·
// 글꼴·스크립트)을 막고, 그림은 data:·blob: 만 열어 같은 출처 그림 요청도 막는다 — 같은 출처(뷰어를 낸 서버)로 가는 다른 요청은 막지
// 않으므로, 같은 출처에 원문을 보내지 않는 것은 이 코드가 지킨다.
//  - 탭 제목은 늘 「itda-doc 뷰어」다(원천의 tabTitle — share.js docTitle). 문서 제목·파일 이름은 탭 제목·브라우저 기록에 넣지 않는다.
//  - 테마 CSS 는 doc.themeCSS 를 blob: URL 로 만들어 <link id="viewer-theme"> 에 건다(공유 화면은 서버가 낸 <link> — CSP 는 둘 다
//    style-src 에 인라인을 열지 않는다). @scope 를 모르는 엔진은 가두지 않은 CSS(unscoped)를 받는다(CSSScopeRule 이 없으면).
//  - 상대 경로 그림은 폴더가 없어 그릴 수 없다 — src 를 data-local-src 로 옮겨 요청하지 않고(파일 이름이 서버로 가지 않게) 그 수를
//    경고 칸에 한 줄로 알린다(localImages). 원격 그림은 늘 끈다 — mdoc 이 data-remote-src 로 옮기고(경고는 View.warnings),
//    그래도 남은 원격 src 는 localImages 가 옮기며, CSP img-src(data:·blob: 만)가 원격·같은 출처 그림 요청을 막는다.
//  - 문서 안 상대 링크(#조각·http(s)·mailto·tel 이 아닌 href — 다른 md·// 주소 등)는 href 를 data-local-href 로 옮겨(localLinks)
//    같은 출처로 이동하지 않는다 — 가운데 단추·새 탭으로 열어도 마찬가지다. 누르면 share.js onClick 이 linkMsg 안내를 낸다.
//  - 경고 목록은 View.warnings(줄 번호와 글)에 뷰어의 줄(그리지 않은 그림)을 더한 것이다.
//  - 인쇄는 브라우저 인쇄다. 쪽 크기(@page)는 시작할 때 꼴마다 blob: 스타일시트를 media="not all" 로 만들어 두고, 인쇄 직전
//    (beforeprint)에 보기 꼴·용지에 맞는 하나만 media="print" 로 켠다(viewer.css 의 @media print 가 틀을 숨긴다).
//    인쇄 단추는 이어보기면 쪽 보기로 바꿔 그린 뒤 인쇄한다(브라우저 메뉴의 인쇄는 지금 꼴 그대로).
// node 시험(cmd/itda-doc-wasm/testdata/viewer.cjs)이 document 없이 이 파일을 올려 window.ItdaViewer 의 부품(localImages·localLinks·warningLines·
// viewerSource)을 본다 — document 가 없으면 화면을 시작하지 않는다.
(function () {
  "use strict";

  const TAB_TITLE = "itda-doc 뷰어";

  // localImages 는 View.html 의 <img src> 가운데 그릴 수 없는 것을 요청하지 않게 바꾼다: data:·blob: 은 그대로, 그 밖 — 상대 경로
  // (local)와 원격 주소(remote — 스킴이 있거나 //) — 은 src 를 data-local-src 로 옮긴다.
  // mdoc 의 HTML 은 속성 값을 늘 큰따옴표로 감싸고 그 안의 " 를 이스케이프하므로 정규식으로 충분하다(이 HTML 에만 쓴다).
  // 바꾼 것이 없으면 html 을 그대로 돌려준다.
  const IMG_SRC = /(<img\b[^>]*?\s)src="([^"]*)"/gi;
  function localImages(html) {
    let local = 0, remote = 0;
    const out = String(html == null ? "" : html).replace(IMG_SRC, (m, head, src) => {
      if (/^(data|blob):/i.test(src)) return m;
      if (/^([a-z][a-z0-9+.-]*:|\/\/)/i.test(src)) remote++; else local++;
      return head + 'data-local-src="' + src + '"';
    });
    return { html: local || remote ? out : html, local, remote };
  }

  // localLinks 는 View.html 의 <a href> 가운데 #조각·http(s)·mailto·tel 이 아닌 것(상대 경로·// — 뷰어를 낸 서버로 가는 주소 — 와
  // 그 밖의 스킴)의 href 를 data-local-href 로 옮긴다: href 가 남지 않으니 왼쪽·가운데 단추·새 탭으로 열어도 어디로도 가지 않는다
  // (누르면 share.js onClick 이 linkMsg 를 낸다). mdoc 은 이미 목록 밖 스킴(javascript: 등)의 링크를 글자만 남기지만(mdoc/urls.go
  // classifyLink), 여기서도 허용 목록 밖은 모두 옮긴다. 정규식으로 충분한 까닭은 localImages 와 같다.
  const A_HREF = /(<a\b[^>]*?\s)href="([^"]*)"/gi;
  function localLinks(html) {
    return String(html == null ? "" : html).replace(A_HREF, (m, head, href) =>
      /^(#|https?:|mailto:|tel:)/i.test(href) ? m : head + 'data-local-href="' + href + '"');
  }

  // warningLines 는 경고 목록이다: View.warnings({Code, Line, Message}) 차례 그대로, 뒤에 뷰어의 줄(줄 번호 0).
  function warningLines(v, imgs) {
    const out = [];
    for (const w of (v && Array.isArray(v.warnings) ? v.warnings : [])) out.push({ line: Number(w.Line) || 0, code: String(w.Code || ""), text: String(w.Message || "") });
    if (imgs && imgs.local) out.push({ line: 0, code: "viewer-local-image", text: "상대 경로 그림 " + imgs.local + "개는 그리지 않음 — 뷰어는 문서 폴더를 열지 않는다" });
    if (imgs && imgs.remote) out.push({ line: 0, code: "viewer-remote-image", text: "원격 그림 " + imgs.remote + "개는 그리지 않음 — 뷰어는 원격 그림을 불러오지 않는다" });
    return out;
  }

  // viewerSource 는 뷰어의 View 원천이다(share.js hostSource 머리 주석의 꼴). deps:
  //   load() → Promise<doc>(ItdaDocLoader.load), setThemeCSS(css) → Promise(테마 CSS 를 건다 — 걸린 뒤 풀린다),
  //   unscoped()(@scope 를 모르는 엔진이면 참), onReady(doc)(wasm 이 올라옴), onOpen()(새 문서를 그리기 직전 — 굴린 자리·경고 칸 되돌리기),
  //   onView(v, lines)(그리기 직전 — 경고 목록 칠하기), onError(msg)(wasm 을 올리지 못함).
  // 원천 밖에서 부르는 것: open(name, src)(새 문서 — wasm 이 아직이면 올라온 뒤 그린다), setTheme(name)("" 이면 문서의 테마),
  // api()(attach 가 받은 화면 부품), doc()(올라온 doc 또는 null), current()({name, src} 또는 null).
  function viewerSource(deps) {
    let api = null, doc = null, cur = null, theme = "", cssKey = "", fresh = false;
    function draw() {
      if (!api || !doc || !cur) return Promise.resolve();
      const o = {};
      if (theme) o.theme = theme;
      const v = doc.render(cur.src, o);
      if (!v.error && !v.title) v.title = cur.name; // 화면 머리(#doc-title)에만 — 탭 제목은 tabTitle 로 고정
      const imgs = localImages(v.html);
      v.html = localLinks(imgs.html);
      if (fresh) { fresh = false; if (deps.onOpen) deps.onOpen(); }
      if (deps.onView) deps.onView(v, warningLines(v, imgs));
      return api.show(v);
    }
    const src = {
      base: "",
      tabTitle: TAB_TITLE,
      linkMsg: "문서 안의 파일 링크는 뷰어에서 열지 않습니다",
      attach(a) {
        api = a;
        return {
          applyTheme(v) {
            const un = !!(deps.unscoped && deps.unscoped());
            const key = v.theme + (un ? "@unscoped" : "");
            if (key === cssKey) return true;
            cssKey = key;
            const css = doc ? doc.themeCSS(String(v.theme || ""), { unscoped: un }) : null;
            return Promise.resolve(deps.setThemeCSS(css || "")).then(() => true, () => true);
          },
          start() {
            Promise.resolve().then(() => deps.load()).then(d => {
              doc = d;
              if (deps.onReady) deps.onReady(d);
              return draw();
            }).catch(e => {
              const msg = "뷰어를 올리지 못했습니다 — " + String(e && e.message || e);
              if (deps.onError) deps.onError(msg);
              api.failed(msg);
            });
          },
          stop() {},
        };
      },
      open(name, text) { cur = { name: String(name || "문서"), src: String(text) }; fresh = true; return draw(); },
      setTheme(name) { theme = String(name || ""); return draw(); },
      api: () => api,
      doc: () => doc,
      current: () => cur,
    };
    return src;
  }

  window.ItdaViewer = { localImages, localLinks, warningLines, viewerSource, TAB_TITLE };
  if (typeof document === "undefined" || !document.body || !document.body.classList.contains("itda-viewer")) return;

  // ---- 화면(뷰어의 막대·대화상자). 그리기는 share.js 가 한다.

  const $ = id => document.getElementById(id);
  const MAX_BYTES = 8 << 20; // 열 원문의 상한(바이트) — 큰 파일을 실수로 끌어 놓아 탭이 멈추지 않게

  // blobSheet 는 link 에 css 를 blob: URL 로 건다 — 걸리면(load·error) 풀리고, 3초가 넘으면 기다리지 않는다. 앞 URL 은 다 건 뒤 거둔다.
  function blobSheet(link, css) {
    const old = link.dataset.url || "";
    const url = URL.createObjectURL(new Blob([css], { type: "text/css" }));
    link.dataset.url = url;
    return new Promise(res => {
      let done = false;
      const fin = () => { if (done) return; done = true; if (old) URL.revokeObjectURL(old); res(); };
      link.addEventListener("load", fin, { once: true });
      link.addEventListener("error", fin, { once: true });
      setTimeout(fin, 3000);
      link.href = url;
    });
  }

  const source = viewerSource({
    load: () => window.ItdaDocLoader.load({ wasm: "itda-doc.wasm" }),
    setThemeCSS: css => blobSheet($("viewer-theme"), css),
    unscoped: () => typeof window.CSSScopeRule !== "function",
    onReady: d => { paintThemes(d.themes()); loadSamples(); $("doc-title").textContent = "문서를 여세요 — 열기·끌어놓기·붙여넣기·견본"; setBusy(false); },
    onOpen: () => { $("viewer").scrollTop = 0; closeWarnings(); }, // 새 문서는 맨 위부터(render 는 읽던 자리를 둔다)
    onView: (v, lines) => { $("viewer-empty").hidden = true; paintWarnings(lines); },
    onError: () => { $("doc-title").textContent = "뷰어를 올리지 못했습니다"; }, // 단추는 막아 둔 채로 — 그릴 수 없다
  });
  window.ItdaViewSource = source;

  function setBusy(on) {
    for (const id of ["btn-open", "btn-paste", "sample", "theme", "btn-print"]) $(id).disabled = on;
  }
  setBusy(true);

  // ---- 테마 고르기: "" 는 문서가 고른 테마(없으면 기본)
  function paintThemes(list) {
    const sel = $("theme");
    for (const t of list) {
      if (t.error) continue;
      const o = document.createElement("option");
      o.value = t.name;
      o.textContent = t.title || t.name;
      sel.appendChild(o);
    }
  }
  $("theme").addEventListener("change", e => { source.setTheme(e.target.value); });

  // ---- 견본(samples/index.json — 빌드가 저장소의 견본을 싣는다)
  let samples = [];
  function loadSamples() {
    fetch("samples/index.json", { cache: "no-cache" }).then(r => r.ok ? r.json() : []).then(list => {
      samples = Array.isArray(list) ? list : [];
      const sel = $("sample");
      for (const s of samples) {
        const o = document.createElement("option");
        o.value = s.file;
        o.textContent = s.label || s.file;
        sel.appendChild(o);
      }
    }, () => { /* 견본 없이 */ });
  }
  $("sample").addEventListener("change", e => {
    const s = samples.find(x => x.file === e.target.value);
    e.target.value = "";
    if (!s) return;
    fetch("samples/" + encodeURIComponent(s.file), { cache: "no-cache" }).then(r => {
      if (!r.ok) throw new Error(String(r.status));
      return r.text();
    }).then(t => source.open(s.label || s.file, t), err => toastMsg("견본을 받지 못했습니다 — " + err.message));
  });

  // ---- 파일 열기(고르기·끌어놓기) — 브라우저 안에서만 읽는다
  function openFile(f) {
    if (!f) return;
    if (f.size > MAX_BYTES) { toastMsg("파일이 너무 큽니다(" + Math.round(f.size / 1048576) + " MB, 상한 8 MB)"); return; }
    f.text().then(t => source.open(f.name.replace(/\.(md|markdown|txt)$/i, ""), t), () => toastMsg("파일을 읽지 못했습니다"));
  }
  $("btn-open").addEventListener("click", () => $("file").click());
  $("file").addEventListener("change", e => { openFile(e.target.files && e.target.files[0]); e.target.value = ""; });
  let dragDepth = 0;
  const hasFiles = e => !!(e.dataTransfer && [...e.dataTransfer.types].includes("Files"));
  document.addEventListener("dragenter", e => { if (!hasFiles(e)) return; e.preventDefault(); dragDepth++; document.body.classList.add("dropping"); });
  document.addEventListener("dragover", e => { if (hasFiles(e)) e.preventDefault(); });
  document.addEventListener("dragleave", () => { if (dragDepth && --dragDepth === 0) document.body.classList.remove("dropping"); });
  document.addEventListener("drop", e => {
    if (!hasFiles(e)) return;
    e.preventDefault();
    dragDepth = 0;
    document.body.classList.remove("dropping");
    if (!source.doc()) { toastMsg("뷰어를 올리는 중입니다 — 잠시 뒤 다시 놓아 주세요"); return; }
    openFile(e.dataTransfer.files[0]);
  });

  // ---- 붙여넣기
  $("btn-paste").addEventListener("click", () => { $("paste-text").value = ""; $("paste-dlg").showModal(); $("paste-text").focus(); });
  $("paste-cancel").addEventListener("click", () => $("paste-dlg").close());
  $("paste-ok").addEventListener("click", () => {
    const t = $("paste-text").value;
    $("paste-dlg").close();
    if (t.trim()) source.open("붙여넣은 문서", t);
  });

  // ---- 경고 목록(칸은 단추·×·Esc 로 닫고, 새 문서를 열면 닫는다)
  function closeWarnings() {
    $("warn-panel").hidden = true;
    $("btn-warn").setAttribute("aria-expanded", "false");
  }
  function paintWarnings(lines) {
    const b = $("btn-warn"), list = $("warn-list");
    b.hidden = !lines.length;
    $("warn-count").textContent = String(lines.length);
    list.textContent = "";
    for (const l of lines) {
      const li = document.createElement("li");
      const no = document.createElement("span");
      no.className = "warn-line";
      no.textContent = l.line ? "줄 " + l.line : "뷰어";
      const tx = document.createElement("span");
      tx.textContent = l.text;
      li.append(no, tx);
      list.appendChild(li);
    }
    if (!lines.length) closeWarnings();
  }
  $("btn-warn").addEventListener("click", () => {
    const p = $("warn-panel");
    p.hidden = !p.hidden;
    $("btn-warn").setAttribute("aria-expanded", String(!p.hidden));
  });
  $("warn-close").addEventListener("click", closeWarnings);
  document.addEventListener("keydown", e => { if (e.key === "Escape" && !$("warn-panel").hidden) closeWarnings(); });

  // ---- 인쇄: 쪽 크기 스타일시트(꼴마다 하나 — media 로 켜고 끈다). 슬라이드는 슬라이드 크기, 쪽은 A4 세로·가로, 이어보기는 A4 + 여백
  const PAGE_RULES = {
    a4: "@page { size: A4; margin: 0; }",
    "a4-landscape": "@page { size: A4 landscape; margin: 0; }",
    flow: "@page { size: A4; margin: 15mm; }",
    "flow-landscape": "@page { size: A4 landscape; margin: 15mm; }",
  };
  for (const r of Object.keys(window.ItdaSlides.SIZE)) {
    const [w, h] = window.ItdaSlides.SIZE[r];
    PAGE_RULES["slides-" + r] = "@page { size: " + w + "px " + h + "px; margin: 0; }";
  }
  const pageSheets = {};
  const pagesReady = Promise.all(Object.keys(PAGE_RULES).map(k => {
    const link = document.createElement("link");
    link.rel = "stylesheet";
    link.media = "not all";
    link.dataset.page = k;
    document.head.appendChild(link);
    pageSheets[k] = link;
    return blobSheet(link, PAGE_RULES[k]);
  }));
  function pageKey() {
    const b = document.body;
    if (b.classList.contains("look-slides")) return "slides-" + ($("pages").classList.contains("r43") ? "4:3" : "16:9");
    const land = b.classList.contains("paper-a4-landscape");
    if (b.classList.contains("look-flow")) return land ? "flow-landscape" : "flow";
    return land ? "a4-landscape" : "a4";
  }
  function pickPageSheet() {
    const k = pageKey();
    for (const [name, link] of Object.entries(pageSheets)) link.media = name === k ? "print" : "not all";
  }
  window.addEventListener("beforeprint", pickPageSheet);
  $("btn-print").addEventListener("click", async () => {
    const api = source.api();
    if (!api || !source.current()) { toastMsg("인쇄할 문서가 없습니다"); return; }
    if (document.body.classList.contains("look-flow")) await api.look("doc");
    await pagesReady;
    pickPageSheet();
    window.print();
  });

  function toastMsg(msg) { const a = source.api(); if (a) a.toast(msg); }
})();
