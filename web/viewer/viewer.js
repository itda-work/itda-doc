// 설치 없는 웹 뷰어(itda-work/itda-hyve#577 2b, 폴더 열기 itda-doc#27). 그리기는 공유 화면과 한 코드다 — share.js 의 화면(render·draw·
// 보기 꼴·목차·노트·쪽 넘김 키)에 View 원천만 바꿔 끼운다: 이 파일이 share.js 보다 먼저 window.ItdaViewSource 를 두고(body 에
// data-base 가 없다 — share.js pickSource), 원천은 파일 열기(고르기·끌어놓기)·폴더 열기(고르기·끌어놓기)·붙여넣기·견본 글을
// wasm(itda-doc.js — ItdaDocLoader)으로 그린 View 다.
//
// 문서는 브라우저를 떠나지 않는다: 원문·폴더 안 파일은 File(file.text — 브라우저 안 읽기)·붙여넣기 칸에서만 읽고, 이 파일이 쓰는 요청은
// 견본 목록·견본 글(samples/ — 같은 출처의 정적 파일, GET)뿐이다(wasm 받기는 itda-doc.js). 폴더 열기·문서 바꾸기는 요청을 내지 않는다.
// index.html 의 CSP 는 제3자 출처로 가는 요청(connect·그림·글꼴·스크립트)을 막고, 그림은 data:·blob: 만 열어 같은 출처 그림 요청도
// 막는다 — 같은 출처(뷰어를 낸 서버)로 가는 다른 요청은 막지 않으므로, 같은 출처에 원문을 보내지 않는 것은 이 코드가 지킨다.
//  - 탭 제목은 늘 「itda-doc 뷰어」다(원천의 tabTitle — share.js docTitle). 문서 제목·파일 이름·폴더 이름은 탭 제목·브라우저 기록에 넣지 않는다.
//  - 테마 CSS 는 doc.themeCSS 를 blob: URL 로 만들어 <link id="viewer-theme"> 에 건다(공유 화면은 서버가 낸 <link> — CSP 는 둘 다
//    style-src 에 인라인을 열지 않는다). @scope 를 모르는 엔진은 가두지 않은 CSS(unscoped)를 받는다(CSSScopeRule 이 없으면).
//  - 폴더 열기: 「폴더」 단추(showDirectoryPicker, 없으면 <input webkitdirectory>)나 폴더 끌어놓기(webkitGetAsEntry)로 연 폴더를
//    표 Map<연 폴더 기준 상대 경로(/ 구분, NFC), File> 로 만든다(walkHandle·tableFromList·walkEntries — 이때 내용은 읽지 않는다).
//    . 으로 시작하는 폴더와 node_modules 는 넣지 않고, 항목 상한(LIMITS.entries)·깊이 상한(LIMITS.depth — page.MaxFolderMetaDepth 와 같은 64)을
//    넘으면 멈추고(깊이는 그 갈래만 건너뛰고) 알린다. md 목록(docList)을 <select id="doc-pick"> 에 보이고 처음 문서는 firstDoc 이다.
//  - 폴더 기본값: 문서 폴더에서 연 폴더 뿌리까지 올라가며 _metadata.yml 을 찾아(metaChain — 먼 것부터, rel 은 문서 기준) 문서를 열 때
//    읽어 두고(readFolderMeta — 64KB 상한, 문구는 page.ReadMetaFile 과 같다) render 옵션 folderMeta 로 넘긴다. 테마를 바꿔 다시 그릴 때는
//    읽어 둔 것을 쓴다(동기). 연 폴더보다 위의 폴더는 브라우저가 주지 않아 보지 못한다 — 네이티브(홈·볼륨 루트까지 올라감)와 이 점이 다르다.
//    반대로 홈보다 위(예: /Users)를 연 폴더로 고르면, 네이티브가 홈에서 멈춰 읽지 않는 _metadata.yml 을 뷰어는 얹을 수 있다.
//    _metadata.yml 은 File.text()(UTF-8 디코드 — BOM 은 떼고 깨진 바이트는 U+FFFD)로 읽어 글자로 넘긴다. 네이티브는 원래 바이트를 읽으므로
//    깨진 UTF-8 파일에서 두 판의 경고가 같은지는 미검증이다.
//  - 나중에 연 것이 이긴다: 폴더 열기·폴더 안 문서 고르기·파일 하나·붙여넣기·견본은 viewerOpener 의 세대를 올리고, 늦게 끝난 걷기·읽기는
//    버린다(폴더 상태·고르기 칸·상태 칸·토스트를 건드리지 않는다). 걷는 동안 화면은 잠그지 않는다.
//  - 그림: 폴더로 열면 상대 경로 그림을 표에서 찾아(resolveImgPath — &amp; 풀기·?·# 떼기·퍼센트 풀기·./.. 풀기·NFC) 확장자 허용 목록
//    (IMAGE_TYPES)이면 blob: URL 을 src 로 넣는다(folderImages — 쪽 나누기 전, share.js imagesSettled 가 기다린다). blob URL 은 같은 문서를
//    다시 그릴 때 재사용하고 문서·폴더를 바꾸면(새 문서를 그린 뒤) 거둔다. 표에 없거나 뿌리 밖·/ 로 시작·허용 목록 밖은 src 를
//    data-local-src 로 옮겨 요청하지 않고 경고 칸에 알린다. 파일 하나·붙여넣기로 열면 상대 경로 그림은 그리지 않는다(localImages —
//    「폴더로 열면 그린다」 경고). 표에서 찾은 그림을 브라우저가 읽지 못하면(연 뒤에 고치거나 옮긴 파일) 문서당 한 번 알리고 경고 칸에 한 줄을 더한다
//    (source.imageFailed — 지금 문서의 blob: URL 에만 반응한다. itda-doc#67). 원격 그림은 늘 끈다 — mdoc 이 data-remote-src 로 옮기고(경고는 View.warnings), 그래도 남은 원격 src 는
//    localImages·folderImages 가 옮기며, CSP img-src(data:·blob: 만)가 원격·같은 출처 그림 요청을 막는다.
//  - 문서 안 상대 링크(#조각·http(s)·mailto·tel 이 아닌 href — 다른 md·// 주소 등)는 href 를 data-local-href 로 옮겨(localLinks)
//    같은 출처로 이동하지 않는다 — 가운데 단추·새 탭으로 열어도 마찬가지다. 누르면 share.js onClick 이 linkMsg 안내를 낸다(폴더로 열어도
//    다른 문서로 가지 않는다).
//  - 경고 목록은 View.warnings(줄 번호와 글)에 뷰어의 줄(그리지 않은 그림)을 더한 것이다. 막대의 상태 칸(#open-state)은 폴더 설정
//    적용 여부를 짧게 보인다.
//  - 인쇄는 브라우저 인쇄다. 쪽 크기(@page)는 시작할 때 꼴마다 blob: 스타일시트를 media="not all" 로 만들어 두고, 인쇄 직전
//    (beforeprint)에 보기 꼴·용지에 맞는 하나만 media="print" 로 켠다(viewer.css 의 @media print 가 틀을 숨긴다).
//    인쇄 단추는 이어보기면 쪽 보기로 바꿔 그린 뒤 인쇄한다(브라우저 메뉴의 인쇄는 지금 꼴 그대로).
// node 시험(cmd/itda-doc-wasm/testdata/viewer.cjs)이 document 없이 이 파일을 올려 window.ItdaViewer 의 부품(localImages·localLinks·warningLines·
// viewerSource·viewerOpener·resolveImgPath·folderImages·tableFromList·walkEntries·walkHandle·docList·firstDoc·metaChain·readFolderMeta·
// droppedDir·folderDocError·failedImageURL·watchImageErrors)을 본다 —
// document 가 없으면 화면을 시작하지 않는다.
(function () {
  "use strict";

  const TAB_TITLE = "itda-doc 뷰어";
  const MAX_BYTES = 8 << 20; // 열 원문의 상한(바이트) — 큰 파일을 실수로 끌어 놓아 탭이 멈추지 않게
  // 폴더 표의 상한: entries 는 항목 수(폴더 걷기는 파일과 폴더, <input webkitdirectory> 는 파일), depth 는 연 폴더 아래 폴더 깊이
  // (page.MaxFolderMetaDepth 와 같은 64). metaBytes 는 _metadata.yml 한 파일의 상한(page.MaxFolderMetaBytes).
  const LIMITS = { entries: 20000, depth: 64, metaBytes: 64 << 10 };
  const META_NAME = "_metadata.yml"; // page.FolderMetaName
  // 그리는 그림의 확장자와 형식(Blob type — SVG 는 type 이 없으면 그려지지 않는다). 형식은 확장자로만 정한다.
  const IMAGE_TYPES = { png: "image/png", jpg: "image/jpeg", jpeg: "image/jpeg", gif: "image/gif", webp: "image/webp",
    avif: "image/avif", svg: "image/svg+xml", bmp: "image/bmp", ico: "image/x-icon" };

  // localImages 는 View.html 의 <img src> 가운데 그릴 수 없는 것을 요청하지 않게 바꾼다: data:·blob: 은 그대로, 그 밖 — 상대 경로
  // (local)와 원격 주소(remote — 스킴이 있거나 //) — 은 src 를 data-local-src 로 옮긴다. 파일 하나·붙여넣기·견본으로 열 때 쓴다.
  // mdoc 의 HTML 은 속성 값을 늘 큰따옴표로 감싸고 그 안의 " 를 이스케이프하므로 정규식으로 충분하다(이 HTML 에만 쓴다).
  // 바꾼 것이 없으면 html 을 그대로 돌려준다.
  const IMG_SRC = /(<img\b[^>]*?\s)src="([^"]*)"/gi;
  const REMOTE = /^([a-z][a-z0-9+.-]*:|\/\/)/i;
  function localImages(html) {
    let local = 0, remote = 0;
    const out = String(html == null ? "" : html).replace(IMG_SRC, (m, head, src) => {
      if (/^(data|blob):/i.test(src)) return m;
      if (REMOTE.test(src)) remote++; else local++;
      return head + 'data-local-src="' + src + '"';
    });
    return { html: local || remote ? out : html, local, remote };
  }

  const nfc = s => String(s).normalize("NFC");
  const ATTR_ENT = { amp: "&", lt: "<", gt: ">", quot: '"', "#39": "'", "#x27": "'" };

  // resolveImgPath 는 mdoc 이 낸 <img src> 값(HTML 이스케이프·퍼센트 인코딩 — 한글·공백 이름도 %XX)을 연 폴더 기준 경로로 푼다.
  // docPath 는 문서의 폴더 기준 경로다. &amp; 등을 풀고, ?·# 뒤를 떼고, decodeURIComponent(실패하면 원문)를 한 뒤 문서 폴더에서 ./.. 를
  // 푼다. 돌려주는 것: { path(NFC) } 또는 { why: "remote" | "absolute"(/ 로 시작) | "outside"(뿌리 밖) | "empty" }, 그리고 shown(푼 글 — 경고용).
  function resolveImgPath(src, docPath) {
    let s = String(src).replace(/&(amp|lt|gt|quot|#39|#x27);/g, (m, e) => ATTR_ENT[e]);
    if (REMOTE.test(s)) return { why: "remote", shown: s };
    s = s.replace(/[?#][\s\S]*$/, "");
    try { s = decodeURIComponent(s); } catch (e) { /* 퍼센트 꼴이 깨졌으면 원문 그대로 */ }
    if (s.startsWith("/")) return { why: "absolute", shown: s };
    const out = String(docPath || "").split("/").slice(0, -1);
    for (const seg of s.split("/")) {
      if (seg === "" || seg === ".") continue;
      if (seg === "..") {
        if (!out.length) return { why: "outside", shown: s };
        out.pop();
        continue;
      }
      out.push(seg);
    }
    if (!out.length) return { why: "empty", shown: s };
    return { path: nfc(out.join("/")), shown: s };
  }

  // caseTwin 은 표에 대소문자만 다른 파일이 꼭 하나 있으면 그 경로다(없거나 둘 이상이면 ""). 경고에 실마리로만 쓴다 — 자동으로 맞춰
  // 그리지 않는다(대소문자를 가리는 곳 — 공유 화면·리눅스 — 에서 깨질 문서를 숨기지 않게). 표마다 소문자 색인을 한 번 만든다.
  const lowerIndex = new WeakMap();
  function caseTwin(files, path) {
    let ix = lowerIndex.get(files);
    if (!ix) {
      ix = new Map();
      for (const k of files.keys()) { const l = k.toLowerCase(); ix.set(l, ix.has(l) ? null : k); }
      lowerIndex.set(files, ix);
    }
    const hit = ix.get(String(path).toLowerCase());
    return hit && hit !== path ? hit : "";
  }

  function imageType(path) {
    const m = /\.([^./]+)$/.exec(path);
    return m ? IMAGE_TYPES[m[1].toLowerCase()] || "" : "";
  }

  // folderImages 는 폴더로 연 문서의 <img src> 를 바꾼다: data:·blob: 은 그대로, 표(files)에 있고 형식이 허용 목록이면 urlFor(path, file, type)
  // 가 준 blob: URL 을 src 로, 그 밖(원격·/ 로 시작·뿌리 밖·표에 없음·허용 목록 밖)은 src 를 data-local-src 로 옮긴다.
  // 돌려주는 것: { html, folder: true, drawn, remote, local(그리지 않은 상대 경로 그림 수), missing: [{shown, why}] — why: missing·absolute·
  // outside·empty·type, caseTwin(why 가 missing 이고 표에 대소문자만 다른 파일이 하나 있으면 그 경로) }.
  function folderImages(html, docPath, files, urlFor) {
    const r = { html: "", folder: true, drawn: 0, remote: 0, local: 0, missing: [] };
    r.html = String(html == null ? "" : html).replace(IMG_SRC, (m, head, src) => {
      if (/^(data|blob):/i.test(src)) return m;
      const res = resolveImgPath(src, docPath);
      if (res.why === "remote") { r.remote++; return head + 'data-local-src="' + src + '"'; }
      const f = res.path ? files.get(res.path) : undefined;
      const type = res.path ? imageType(res.path) : "";
      if (f && type) { r.drawn++; return head + 'src="' + urlFor(res.path, f, type) + '"'; }
      r.local++;
      const miss = { shown: res.path || res.shown, why: res.why || (f ? "type" : "missing") };
      if (miss.why === "missing") { const c = caseTwin(files, res.path); if (c) miss.caseTwin = c; }
      r.missing.push(miss);
      return head + 'data-local-src="' + src + '"';
    });
    return r;
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

  // listPaths 는 경고 칸에 보일 앞 몇 개의 경로다.
  function listPaths(items, n) {
    const head = items.slice(0, n).map(x => x.shown + (x.why === "outside" ? "(연 폴더 밖)" : x.why === "absolute" ? "(/ 로 시작)" :
      x.caseTwin ? "(대소문자만 다른 " + x.caseTwin + " 가 있음)" : ""));
    return head.join(", ") + (items.length > n ? " 외 " + (items.length - n) + "개" : "");
  }

  // warningLines 는 경고 목록이다: View.warnings({Code, Line, Message}) 차례 그대로, 뒤에 뷰어의 줄(줄 번호 0).
  // kind 는 연 길(open 의 kind — "file"·"paste"·"sample")이다. 폴더가 아닐 때 상대 경로 그림을 그리지 않은 까닭을 길마다 다르게 말한다:
  // 파일 하나는 폴더로 열면 그리고, 붙여넣은 문서는 폴더가 없고, 견본 글에는 그림 파일이 없다.
  // imgs 는 localImages(파일 하나 — 폴더가 없다) 또는 folderImages(폴더 — 없는 그림의 경로를 보인다)의 결과다.
  const LOCAL_WHY = {
    file: "파일 하나로 열었다(폴더로 열면 그린다)",
    paste: "붙여넣은 문서라 폴더가 없다",
    sample: "견본 글에는 그림 파일이 없다",
  };
  function warningLines(v, imgs, kind) {
    const out = [];
    for (const w of (v && Array.isArray(v.warnings) ? v.warnings : [])) out.push({ line: Number(w.Line) || 0, code: String(w.Code || ""), text: String(w.Message || "") });
    if (imgs && imgs.folder) {
      const miss = (imgs.missing || []).filter(x => x.why !== "type"), types = (imgs.missing || []).filter(x => x.why === "type");
      if (miss.length) out.push({ line: 0, code: "viewer-local-image", text: "폴더에 없는 그림 " + miss.length + "개는 그리지 않음 — " + listPaths(miss, 3) });
      if (types.length) out.push({ line: 0, code: "viewer-image-type", text: "그리지 않는 형식의 그림 " + types.length + "개 — " + listPaths(types, 3) + " (그리는 형식: " + Object.keys(IMAGE_TYPES).join("·") + ")" });
    } else if (imgs && imgs.local) {
      out.push({ line: 0, code: "viewer-local-image", text: "상대 경로 그림 " + imgs.local + "개는 그리지 않음 — " + (LOCAL_WHY[kind] || LOCAL_WHY.file) });
    }
    if (imgs && imgs.remote) out.push({ line: 0, code: "viewer-remote-image", text: "원격 그림 " + imgs.remote + "개는 그리지 않음 — 뷰어는 원격 그림을 불러오지 않는다" });
    return out;
  }

  // ---- 폴더 표: Map<연 폴더 기준 상대 경로(/ 구분, NFC), File>. 세 길(walkEntries·walkHandle·tableFromList)이 같은 표를 만든다.
  // 표를 만드는 동안 파일 내용은 읽지 않는다(File 은 내용을 읽기 전의 손잡이다).

  const skipDir = name => name.startsWith(".") || name === "node_modules";

  // newTable 은 표와 상한 판정이다. stopped: "" | "entries"(항목 상한 — 걷기를 멈춤) | "depth"(깊이 상한 — 그 갈래만 건너뜀).
  function newTable(limits) {
    const lim = Object.assign({}, LIMITS, limits);
    const t = { files: new Map(), stopped: "", count: 0, limits: lim };
    t.full = () => t.stopped === "entries";
    t.take = () => {
      if (t.full()) return false;
      if (++t.count > lim.entries) { t.stopped = "entries"; return false; }
      return true;
    };
    t.deep = depth => {
      if (depth <= lim.depth) return false;
      if (!t.stopped) t.stopped = "depth";
      return true;
    };
    t.add = (rel, file) => {
      if (!t.take()) return false;
      const k = nfc(rel);
      if (!t.files.has(k)) t.files.set(k, file); // NFC 로 같아지는 두 이름은 먼저 본 것
      return true;
    };
    t.result = name => ({ name: String(name || ""), files: t.files, stopped: t.stopped });
    return t;
  }

  // walkEntries 는 끌어놓은 폴더(FileSystemDirectoryEntry — drop 처리기 안에서 webkitGetAsEntry 로 잡은 것)를 걷는다.
  // readEntries 는 한 번에 다 주지 않으므로(브라우저마다 100개 단위 등) 빈 배열이 나올 때까지 되풀이한다. 읽지 못한 폴더·파일은 건너뛴다.
  function walkEntries(root, limits) {
    const t = newTable(limits);
    const readBatch = reader => new Promise((res, rej) => reader.readEntries(res, rej));
    const fileOf = en => new Promise((res, rej) => en.file(res, rej));
    async function walk(dir, prefix, depth) {
      const reader = dir.createReader();
      for (;;) {
        let batch;
        try { batch = await readBatch(reader); } catch (e) { return; }
        if (!batch || !batch.length) return;
        for (const en of batch) {
          if (t.full()) return;
          if (en.isDirectory) {
            if (skipDir(en.name) || t.deep(depth + 1) || !t.take()) continue;
            await walk(en, prefix + en.name + "/", depth + 1);
          } else if (en.isFile) {
            let f;
            try { f = await fileOf(en); } catch (e) { continue; }
            if (!t.add(prefix + en.name, f)) return;
          }
        }
      }
    }
    return walk(root, "", 0).then(() => t.result(root.name));
  }

  // walkHandle 은 showDirectoryPicker 가 준 FileSystemDirectoryHandle 을 걷는다(entries() 의 비동기 되풀이). 읽지 못한 것은 건너뛴다.
  function walkHandle(root, limits) {
    const t = newTable(limits);
    async function walk(dir, prefix, depth) {
      try {
        for await (const [name, h] of dir.entries()) {
          if (t.full()) return;
          if (h.kind === "directory") {
            if (skipDir(name) || t.deep(depth + 1) || !t.take()) continue;
            await walk(h, prefix + name + "/", depth + 1);
          } else {
            let f;
            try { f = await h.getFile(); } catch (e) { continue; }
            if (!t.add(prefix + name, f)) return;
          }
        }
      } catch (e) { /* 읽지 못한 폴더는 건너뛴다 */ }
    }
    return walk(root, "", 0).then(() => t.result(root.name));
  }

  // tableFromList 는 <input webkitdirectory> 가 준 FileList 를 표로 만든다. webkitRelativePath 는 「연 폴더 이름/…」 꼴이라 앞 마디를 뗀다.
  function tableFromList(list, limits) {
    const t = newTable(limits);
    let name = "";
    for (const f of Array.from(list || [])) {
      const segs = String(f.webkitRelativePath || f.name || "").split("/");
      if (segs.length > 1 && !name) name = segs[0];
      const rest = segs.length > 1 ? segs.slice(1) : segs;
      const dirs = rest.slice(0, -1);
      if (dirs.some(skipDir) || t.deep(dirs.length)) continue;
      if (!t.add(rest.join("/"), f)) break;
    }
    return t.result(name);
  }

  // docList 는 표의 .md·.markdown 경로다(경로 순 — 글자 단위 비교).
  function docList(files) {
    return [...files.keys()].filter(p => /\.(md|markdown)$/i.test(p)).sort((a, b) => (a < b ? -1 : a > b ? 1 : 0));
  }

  // firstDoc 은 처음 열 문서다: 뿌리의 index.md → README.md(둘 다 대소문자 무시) → 목록의 첫 항목. 목록이 비면 "".
  function firstDoc(list) {
    const top = list.filter(p => !p.includes("/"));
    return top.find(p => p.toLowerCase() === "index.md") || top.find(p => p.toLowerCase() === "readme.md") || list[0] || "";
  }

  // metaChain 은 문서(docPath)에 얹힐 _metadata.yml 을 먼 폴더 → 가까운 폴더 순으로 돌려준다: [{rel(문서 기준 — _metadata.yml·
  // ../_metadata.yml), path(표의 경로)}]. 문서 폴더에서 연 폴더 뿌리까지 올라가며, 올라가는 단계는 LIMITS.depth 까지다
  // (page.folderMetaDirs 와 같은 상한 — 단계 0..64).
  function metaChain(docPath, files) {
    const dirs = String(docPath).split("/").slice(0, -1);
    const out = [];
    for (let up = 0; up <= Math.min(dirs.length, LIMITS.depth); up++) {
      const dir = dirs.slice(0, dirs.length - up).join("/");
      const path = (dir ? dir + "/" : "") + META_NAME;
      if (files.has(path)) out.unshift({ rel: "../".repeat(up) + META_NAME, path });
    }
    return out;
  }

  // 연 뒤 파일을 고치면 브라우저가 쥔 File 의 읽기가 거부될 수 있다(Chromium 실측: NotReadableError). 까닭을 가리지 않고 할 일을 말한다.
  const DOC_READ_FAILED = "파일을 읽지 못했습니다 — 연 뒤에 고쳤거나 옮겼다면 「폴더」로 다시 열어 주세요";
  const META_READ_FAILED = "읽지 못함(연 뒤에 고쳤다면 「폴더」로 다시 열어 주세요)";
  // 그림은 File 을 읽는 코드가 없이 <img> 가 blob: URL 로 읽으므로 실패가 error 이벤트로만 보인다. 알림은 곧 사라져 경고 칸에도 남긴다.
  const IMAGE_READ_FAILED = "그림을 읽지 못했습니다 — 연 뒤에 고쳤거나 옮겼다면 「폴더」로 다시 열어 주세요";
  const IMAGE_READ_LINE = { line: 0, code: "viewer-image-read", text: "읽지 못한 그림이 있음 — 연 뒤에 고쳤거나 옮겼다면 「폴더」로 다시 열어 주세요" };

  // failedImageURL 은 error 이벤트의 대상이 blob: 그림이면 그 URL 이다(아니면 ""). src 를 걷어낸 그림(data-local-src)·원격·data: 그림은 대상이 아니다.
  function failedImageURL(el) {
    if (!el || el.tagName !== "IMG") return "";
    const src = el.getAttribute("src") || "";
    return /^blob:/i.test(src) ? src : "";
  }

  // watchImageErrors 는 문서 전체의 error 이벤트를 듣고 그림 실패를 source.imageFailed 로 넘긴다. error 는 버블되지 않아 capture 로 듣는다.
  // 문서 영역 밖 그림은 지금 문서의 URL 이 아니어서 imageFailed 가 거른다.
  function watchImageErrors(doc, source) {
    doc.addEventListener("error", e => { source.imageFailed(failedImageURL(e.target)); }, true);
  }

  // readFolderMeta 는 metaChain 의 파일을 읽어 render 옵션 folderMeta 꼴([{rel, yaml, err}])로 돌려준다. 상한(64KB)을 넘거나 읽지 못하면
  // err 만 채운다 — 문구는 page.ReadMetaFile 과 같고(「파일이 너무 큼(64KB 넘음)」·「읽지 못함」), 내용 읽기 실패에만 다시 열라는 안내를 붙인다
  // (브라우저가 쥔 File 은 연 뒤 고치면 읽기가 거부될 수 있다). 순서는 chain 그대로다.
  function readFolderMeta(chain, files) {
    return Promise.all(chain.map(m => {
      const f = files.get(m.path);
      if (!f) return { rel: m.rel, yaml: "", err: "읽지 못함" };
      if (f.size > LIMITS.metaBytes) return { rel: m.rel, yaml: "", err: "파일이 너무 큼(64KB 넘음)" };
      return Promise.resolve().then(() => f.text()).then(t => ({ rel: m.rel, yaml: String(t), err: "" }), () => ({ rel: m.rel, yaml: "", err: META_READ_FAILED }));
    }));
  }

  // folderDocError 는 폴더 표의 문서 path 를 열 수 없는 까닭이다(없음·크기 상한). 열 수 있으면 "".
  function folderDocError(folder, path) {
    const f = folder && folder.files && folder.files.get(path);
    if (!f) return "문서를 찾지 못했습니다";
    if (f.size > MAX_BYTES) return "파일이 너무 큽니다(" + Math.round(f.size / 1048576) + " MB, 상한 8 MB)";
    return "";
  }

  // droppedDir 는 끌어놓은 것(DataTransfer) 가운데 첫 폴더 entry 다. 폴더가 없거나 webkitGetAsEntry 가 없으면 null — 파일 하나로 연다.
  // drop 처리기 안에서 동기로 불러야 한다(처리기를 벗어나면 items 를 읽을 수 없다).
  function droppedDir(dt) {
    for (const it of Array.from((dt && dt.items) || [])) {
      if (!it || it.kind !== "file" || typeof it.webkitGetAsEntry !== "function") continue;
      const en = it.webkitGetAsEntry();
      if (en && en.isDirectory) return en;
    }
    return null;
  }

  // viewerSource 는 뷰어의 View 원천이다(share.js hostSource 머리 주석의 꼴). deps:
  //   load() → Promise<doc>(ItdaDocLoader.load), setThemeCSS(css) → Promise(테마 CSS 를 건다 — 걸린 뒤 풀린다),
  //   unscoped()(@scope 를 모르는 엔진이면 참), onReady(doc)(wasm 이 올라옴), onOpen()(새 문서를 그리기 직전 — 굴린 자리·경고 칸 되돌리기),
  //   onView(v, lines, cur)(그리기 직전 — 경고 목록·상태 칸 칠하기), onWarnings(lines)(그린 뒤 경고 목록이 늘어남), onError(msg)(wasm 을 올리지 못함),
  //   objectURL(file, type)·revokeURL(url)(그림 blob URL — 없으면 URL.createObjectURL·revokeObjectURL. node 시험이 바꿔 낀다).
  // 원천 밖에서 부르는 것: open(name, src, kind)(새 문서 — kind: "file"·"paste"·"sample", 폴더 상태를 비운다. wasm 이 아직이면 올라온 뒤
  // 그린다), openFolder(folder, path) → Promise<{ok, stale?, msg?}>(폴더 표 folder({name, files})의 문서 path 를 연다 — 원문과
  // _metadata.yml 을 읽은 뒤 그린다. 그 사이 다른 open·openFolder 가 오면 이 읽기는 버린다(stale)), setTheme(name)("" 이면 문서의 테마),
  // imageFailed(url)(문서 그림의 읽기 실패 — 지금 문서의 blob: URL 이면 문서당 한 번 알리고 참을 돌려준다),
  // api()(attach 가 받은 화면 부품), doc()(올라온 doc 또는 null), current()({name, src, kind, folder?, path?, meta?} 또는 null).
  function viewerSource(deps) {
    let api = null, doc = null, cur = null, theme = "", cssKey = "", fresh = false, seq = 0;
    // 문서 안 그림의 blob URL(표의 경로 → URL). 같은 문서를 다시 그릴 때(테마 등) 재사용하고, 문서·폴더를 바꾸면 새 문서를 그린 뒤 거둔다.
    let urls = new Map();
    let imageBad = false, lines = []; // imageBad: 이 문서에서 그림 읽기 실패를 이미 알림. lines: 지금 문서의 경고 목록
    const makeURL = deps.objectURL || ((file, type) => URL.createObjectURL(new Blob([file], { type })));
    const dropURL = deps.revokeURL || (u => URL.revokeObjectURL(u));
    function urlFor(path, file, type) {
      let u = urls.get(path);
      if (!u) { u = makeURL(file, type); urls.set(path, u); }
      return u;
    }
    function draw() {
      if (!api || !doc || !cur) return Promise.resolve();
      const o = {};
      if (theme) o.theme = theme;
      if (cur.folder && cur.meta.length) o.folderMeta = cur.meta;
      const v = doc.render(cur.src, o);
      if (!v.error && !v.title) v.title = cur.name; // 화면 머리(#doc-title)에만 — 탭 제목은 tabTitle 로 고정
      const imgs = cur.folder ? folderImages(v.html, cur.path, cur.folder.files, urlFor) : localImages(v.html);
      v.html = localLinks(imgs.html);
      if (fresh) { fresh = false; if (deps.onOpen) deps.onOpen(); }
      lines = warningLines(v, imgs, cur.kind);
      if (imageBad) lines.push(IMAGE_READ_LINE); // 같은 문서를 다시 그려도(테마 등) 경고 칸에서 사라지지 않게
      if (deps.onView) deps.onView(v, lines, cur);
      return api.show(v);
    }
    // begin 은 새 문서를 세워 그린다 — 앞 문서의 blob URL 은 새 문서를 그린 뒤 거둔다(그리는 동안 앞 화면이 깨지지 않게).
    function begin(next) {
      const old = urls;
      urls = new Map();
      imageBad = false;
      cur = next;
      fresh = true;
      const done = () => { for (const u of old.values()) dropURL(u); };
      return Promise.resolve().then(draw).then(done, e => { done(); throw e; });
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
      open(name, text, kind) {
        seq++;
        return begin({ name: String(name || "문서"), src: String(text), kind: String(kind || "file") });
      },
      openFolder(folder, path) {
        // 열 수 없는 문서(없음·너무 큼)는 seq 를 올리기 전에 끝낸다 — 진행 중인 다른 열기를 버리지 않게
        const bad = folderDocError(folder, path);
        if (bad) return Promise.resolve({ ok: false, msg: bad });
        const my = ++seq;
        const f = folder.files.get(path);
        const chain = metaChain(path, folder.files);
        return Promise.all([Promise.resolve().then(() => f.text()), readFolderMeta(chain, folder.files)]).then(([text, meta]) => {
          if (my !== seq) return { ok: false, stale: true }; // 그 사이 다른 문서를 열었다 — 앞선 읽기가 늦게 끝나도 이기지 않는다
          const name = path.split("/").pop().replace(/\.(md|markdown)$/i, "");
          return begin({ name, src: String(text), kind: "folder", folder, path, meta }).then(() => ({ ok: true }));
        }, () => (my !== seq ? { ok: false, stale: true } : { ok: false, msg: DOC_READ_FAILED }));
      },
      setTheme(name) { theme = String(name || ""); return draw(); },
      // 앞 문서의 그림이 새 문서를 그리기 전에 늦게 실패해도 새 문서를 탓하지 않게, 지금 문서의 URL 표에 있는 것에만 반응한다
      imageFailed(url) {
        if (imageBad || !url || ![...urls.values()].includes(url)) return false;
        imageBad = true;
        lines.push(IMAGE_READ_LINE);
        if (deps.onWarnings) deps.onWarnings(lines);
        if (api) api.toast(IMAGE_READ_FAILED);
        return true;
      },
      api: () => api,
      doc: () => doc,
      current: () => cur,
    };
    return src;
  }

  // viewerOpener 는 「지금 여는 일」 의 세대를 쥔다 — 나중에 연 것이 이긴다. 폴더 열기(folder — 표를 만드는 Promise), 폴더 안 문서
  // 고르기(doc), 원문 열기(text — 파일 하나·붙여넣기·견본, 원문 Promise)가 시작할 때 세대를 올리고, 풀릴 때 세대가 다르면 버린다.
  // 버린 것은 폴더 상태·고르기 칸·상태 칸·토스트를 건드리지 않는다(source 도 부르지 않는다). 걷는 동안 화면을 잠그지 않는다.
  // ui: setFolder(list)(고르기 칸을 채워 보임), clearFolder()(고르기 칸을 비워 숨김), pick(path)(고르기 칸의 값), toast(msg).
  // 원문 열기는 원문을 다 읽은 뒤에야 폴더 상태를 비운다(읽기에 실패하면 앞 화면·폴더 그대로).
  function viewerOpener(source, ui) {
    let gen = 0, folder = null;
    // pending 은 아직 끝나지 않은 문서 열기({my, path}) — 그 사이 열 수 없는 문서를 고르면 고르기 칸을 이것으로 되돌린다
    // (세대를 올리지 않으므로 이 열기가 이어서 그려진다 — 화면과 고르기 칸이 어긋나지 않게).
    let pending = null;
    const why = e => String(e && e.message || e);
    // back 은 고르기 칸을 진행 중인 문서 열기의 path 로, 없으면 지금 보이는 문서로 되돌린다(그 문서가 이 폴더의 것이 아니면 빈 고르기).
    const back = () => {
      if (pending) { ui.pick(pending.path); return; }
      const c = source.current();
      ui.pick(c && c.folder === folder ? c.path : "");
    };
    // first 는 폴더의 첫 문서다 — 실패해도 폴더는 열린 채로 두고(고르기 칸을 보임) 다른 문서를 고르라고 알린다.
    const tail = first => (first ? " — 다른 문서를 고르세요" : "");
    function show(my, path, first) {
      pending = { my, path };
      const done = () => { if (pending && pending.my === my) pending = null; };
      return Promise.resolve(source.openFolder(folder, path)).then(r => {
        done();
        if (my !== gen || r.stale) return { ok: false, stale: true };
        if (!r.ok) { back(); ui.toast(r.msg + tail(first)); }
        return r;
      }, e => {
        done();
        if (my !== gen) return { ok: false, stale: true };
        back();
        ui.toast("문서를 그리지 못했습니다 — " + why(e) + tail(first));
        return { ok: false };
      });
    }
    return {
      folder(p) {
        const my = ++gen;
        ui.toast("폴더를 읽는 중…");
        return Promise.resolve(p).then(t => {
          if (my !== gen) return { ok: false, stale: true }; // 걷는 사이 다른 것을 열었다
          const list = docList(t.files);
          const cut = t.stopped === "entries" ? "항목이 " + LIMITS.entries + "개를 넘어 폴더 읽기를 멈췄습니다"
            : t.stopped === "depth" ? "폴더 깊이가 " + LIMITS.depth + "단계를 넘는 곳은 읽지 않았습니다" : "";
          if (!list.length) { ui.toast("폴더에 md 파일이 없습니다" + (cut ? " — " + cut : "")); return { ok: false }; } // 앞 화면 그대로
          if (cut) ui.toast(cut);
          // 폴더는 첫 문서를 열기 전에 세운다 — 첫 문서만 못 열면 폴더는 열린 채 「다른 문서를 고르세요」 다(의도, 리뷰 2회차 [3])
          folder = t;
          pending = null;
          ui.setFolder(list);
          const path = firstDoc(list);
          const bad = folderDocError(folder, path);
          if (bad) { back(); ui.toast(bad + tail(true)); return { ok: false, msg: bad }; }
          ui.pick(path);
          return show(my, path, true);
        }, e => {
          if (my !== gen) return { ok: false, stale: true };
          ui.toast("폴더를 읽지 못했습니다 — " + why(e));
          return { ok: false };
        });
      },
      doc(path) {
        if (!folder) return Promise.resolve({ ok: false });
        const bad = folderDocError(folder, path); // 열 수 없으면 세대를 올리지 않는다 — 진행 중인 열기를 버리지 않게
        if (bad) { back(); ui.toast(bad); return Promise.resolve({ ok: false, msg: bad }); }
        const my = ++gen;
        ui.pick(path);
        return show(my, path);
      },
      text(name, p, kind, failMsg) {
        const my = ++gen;
        return Promise.resolve(p).then(t => {
          if (my !== gen) return { ok: false, stale: true };
          folder = null;
          pending = null;
          ui.clearFolder();
          return Promise.resolve(source.open(name, t, kind)).then(() => ({ ok: true }));
        }, e => {
          if (my !== gen) return { ok: false, stale: true };
          ui.toast(failMsg ? failMsg(e) : "열지 못했습니다 — " + why(e));
          return { ok: false };
        });
      },
      folderTable: () => folder,
    };
  }

  window.ItdaViewer = { localImages, localLinks, warningLines, viewerSource, viewerOpener, TAB_TITLE, LIMITS, IMAGE_TYPES,
    resolveImgPath, folderImages, walkEntries, walkHandle, tableFromList, docList, firstDoc, metaChain, readFolderMeta, droppedDir, folderDocError, failedImageURL, watchImageErrors };
  if (typeof document === "undefined" || !document.body || !document.body.classList.contains("itda-viewer")) return;

  // ---- 화면(뷰어의 막대·대화상자). 그리기는 share.js 가 한다.

  const $ = id => document.getElementById(id);

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
    onReady: d => { paintThemes(d.themes()); loadSamples(); $("doc-title").textContent = "문서를 여세요 — 열기·폴더·끌어놓기·붙여넣기·견본"; setBusy(false); },
    onOpen: () => { $("viewer").scrollTop = 0; closeWarnings(); }, // 새 문서는 맨 위부터(render 는 읽던 자리를 둔다)
    onView: (v, lines, cur) => { $("viewer-empty").hidden = true; paintWarnings(lines); paintState(v, cur); },
    onWarnings: lines => paintWarnings(lines),
    onError: () => { $("doc-title").textContent = "뷰어를 올리지 못했습니다"; }, // 단추는 막아 둔 채로 — 그릴 수 없다
  });
  window.ItdaViewSource = source;
  watchImageErrors(document, source);

  function setBusy(on) {
    for (const id of ["btn-open", "btn-folder", "btn-paste", "sample", "doc-pick", "theme", "btn-print"]) $(id).disabled = on;
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
    opener.text(s.label || s.file, fetch("samples/" + encodeURIComponent(s.file), { cache: "no-cache" }).then(r => {
      if (!r.ok) throw new Error(String(r.status));
      return r.text();
    }), "sample", err => "견본을 받지 못했습니다 — " + String(err && err.message || err));
  });

  // ---- 파일 열기(고르기·끌어놓기) — 브라우저 안에서만 읽는다
  function openFile(f) {
    if (!f) return;
    if (f.size > MAX_BYTES) { toastMsg("파일이 너무 큽니다(" + Math.round(f.size / 1048576) + " MB, 상한 8 MB)"); return; }
    opener.text(f.name.replace(/\.(md|markdown|txt)$/i, ""), Promise.resolve().then(() => f.text()), "file", () => "파일을 읽지 못했습니다");
  }

  // ---- 폴더 열기: 표를 만든 뒤(내용은 읽지 않는다) md 목록을 보이고 처음 문서를 연다. md 가 없으면 알리기만 하고 이전 화면을 둔다.
  // 「지금 여는 일」 의 세대는 opener(viewerOpener)가 쥔다 — 늦게 끝난 걷기·읽기가 그 뒤에 연 것을 덮지 않는다.
  const opener = viewerOpener(source, {
    setFolder(list) {
      const sel = $("doc-pick");
      sel.textContent = "";
      for (const p of list) {
        const o = document.createElement("option");
        o.value = p;
        o.textContent = p;
        sel.appendChild(o);
      }
      sel.hidden = false;
    },
    clearFolder() {
      const sel = $("doc-pick");
      sel.hidden = true;
      sel.textContent = "";
    },
    pick(path) { $("doc-pick").value = path; },
    toast: msg => toastMsg(msg),
  });
  const openFolderTable = p => opener.folder(p);
  $("doc-pick").addEventListener("change", e => opener.doc(e.target.value));
  $("btn-folder").addEventListener("click", () => {
    if (!source.doc()) return;
    if (typeof window.showDirectoryPicker === "function") {
      window.showDirectoryPicker({ mode: "read" }).then(h => openFolderTable(walkHandle(h)), e => {
        if (e && e.name === "AbortError") return; // 사용자가 취소했다
        toastMsg("폴더를 열지 못했습니다 — " + String(e && e.message || e));
      });
    } else {
      $("dir").click();
    }
  });
  $("dir").addEventListener("change", e => {
    const list = e.target.files;
    if (list) openFolderTable(Promise.resolve(tableFromList(list)));
    e.target.value = "";
  });

  // ---- 상태 칸: 폴더 설정이 적용됐는지(폴더로 열면 적용된 수와 경로, 파일 하나·붙여넣기는 미적용, 견본은 비운다)
  function paintState(v, cur) {
    const el = $("open-state");
    const kind = cur ? cur.kind : "";
    if (kind === "folder") {
      const fm = Array.isArray(v.folder_meta) ? v.folder_meta : [];
      el.textContent = fm.length ? "폴더 설정 " + fm.length + "개" : "폴더 설정 없음";
      el.title = (fm.length ? "적용된 폴더 설정(가까운 것부터): " + fm.join(", ") : "문서 폴더부터 연 폴더까지 _metadata.yml 이 없습니다") +
        " — 연 폴더보다 위의 폴더는 브라우저가 주지 않아 보지 못합니다. 고친 내용은 「폴더」로 다시 열어야 보입니다";
      el.hidden = false;
    } else if (kind === "file" || kind === "paste") {
      el.textContent = "폴더 설정·그림 미적용";
      el.title = "파일 하나로 열어 폴더 설정(_metadata.yml)과 상대 경로 그림을 쓰지 않았습니다 — 「폴더」로 열면 적용합니다";
      el.hidden = false;
    } else {
      el.textContent = "";
      el.title = "";
      el.hidden = true;
    }
    // 좁은 막대에서는 상태 칸을 숨긴다(viewer.css) — 같은 정보를 문서 제목의 title 에도 둔다
    $("doc-title").title = el.textContent ? el.textContent + " — " + el.title : "";
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
    const dir = droppedDir(e.dataTransfer); // 처리기 안에서 동기로 — 벗어나면 items 를 읽을 수 없다
    if (dir) { openFolderTable(walkEntries(dir)); return; }
    openFile(e.dataTransfer.files[0]);
  });

  // ---- 붙여넣기
  $("btn-paste").addEventListener("click", () => { $("paste-text").value = ""; $("paste-dlg").showModal(); $("paste-text").focus(); });
  $("paste-cancel").addEventListener("click", () => $("paste-dlg").close());
  $("paste-ok").addEventListener("click", () => {
    const t = $("paste-text").value;
    $("paste-dlg").close();
    if (t.trim()) opener.text("붙여넣은 문서", t, "paste");
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
