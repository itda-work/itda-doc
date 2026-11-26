// 문서 창 공유의 보기 화면(itda-hyve#339 ①·②·③·④·⑥·⑦·itda-hyve#346 ⑧·⑨). 같은 네트워크의 브라우저(폰·노트북)에서 돈다 — Wails 바인딩이 없고, 문서는
// GET <base>/view.json 으로 받아 문서 창과 같은 부품(pager.js 쪽 나눔·slides.js 슬라이드·base.css·테마 CSS)으로 그린다.
// 갱신 스트림(<base>/events, SSE)은 hello·reload·version·kick·stop 을 받는다: hello·version(파일 저장)은 지금 판·테마를 싣고, 제 view 와
// 다르면 view.json 을 다시 받아 테마가 같으면 제자리에서 다시 그리고 다르면 화면을 다시 불러온다(syncer). reload(호스트가 테마를 바꿈·
// 다른 이름으로 저장)면 화면을 처음부터 다시 불러오고, stop 이면 「공유가 끝났습니다」, kick(이 화면을 끊음)이면 「이 화면의 공유가
// 끝났습니다」 로 바꾸고 더 요청하지 않는다. 스트림이 끊기면(폰 Safari 가 뒤로 갔다 오면 끊긴다) 지금 문서(와 폴더면 목록)를 다시 받아 상태를 본 뒤
// 1·2·4…30초 간격으로 다시 붙는다. 테마 CSS 는 화면을 불러올 때 한 번 받는다(정책 D — themeApplier).
// 편집·저장·테마 바꾸기·링크 열기 바인딩은 없다. 보기 꼴은 슬라이드 문서면 슬라이드, 아니면 좁은 화면(768px 이하)은 이어보기, 넓으면 쪽. 호스트의 꼴 전환은 아래 「호스트 꼴 적용」.
// ③ 발표자 따라가기: slide 이벤트(hello 에도 slide·slide_total·follow)로 호스트의 발표 슬라이드를 받아, 따라가기가 켜져 있고 이 화면이
// 잠시 풀리지 않았으면 보기 꼴을 슬라이드로 바꾸고 그 슬라이드를 가운데로 굴린다(follower). 보는 사람이 직접 움직이면(휠·터치·키·
// 굴림 막대·문서 안 #제목 링크) 이 화면에서만 잠시 풀리고 「발표자 위치로」 단추로 다시 맞춘다. 크기가 바뀌면(폰 회전·창 너비) 따라가는
// 중이면 새 배율로 다시 맞춘다. 원래 슬라이드 문서가 아니면 발표가 끝나거나 따라가기가 꺼진 뒤 쪽 | 이어보기 단추를 다시 보인다. 풀림은 이 화면의 메모리에만 있다(새로 고치면 다시 따라감).
// ④ 목차: view.json 의 toc(제목 트리)를 왼쪽 사이드바에 그린다(문서 창과 같은 꼴 — 번호·글자). 화면 폭이 1024px 보다 넓으면 펼치고
// 아니면 접어 두며 목차 단추로 여는 서랍(본문 위에 겹치고 바깥을 누르면 닫힘)이 된다. 폭이 경계를 넘나들면 그 쪽의 기본값으로 되돌리고,
// 같은 쪽 안에서는 사람이 열고 닫은 대로 둔다(tocPanel — 메모리에만). 항목을 누르면 문서 안 #제목 링크와 같은 길(tocJumper →
// hashJump → followLink)로 가 따라가기 중이면 이 화면만 풀린다. 제목은 그린 본문(#pages) 안에서만 찾는다(headingFinder — UI 와 같은
// id 의 제목). 보기 영역을 굴리면 맨 위에서 120px 안쪽에 든 마지막 제목의 항목을 강조한다(currentHeading — #제목으로 옮긴 제목은
// 보기 영역 안에 있는 동안 그것). 제목이 없는 문서·공유가 끝난 화면이면 목차 단추·사이드바를 숨긴다.
// 목차 접기(itda-hyve#985): 하위가 있는 항목마다 ▸/▾ 단추(button.toc-tg — 접고 펼 뿐 옮기지 않는다), 「목차」 머리 옆에 「모두 접기·모두 펼치기」(tocBulkHead).
// 처음에는 모두 접고 강조 항목의 조상만 펼치며, 강조 항목이 바뀌면 그 조상을 더 펼친다(앞서 펼친 것은 접지 않는다 — tocFold). 사람이 단추로 고른 것만
// 문서마다 이 브라우저의 localStorage 에 기억하고(tocFoldPrefs — 열쇠는 원천의 foldKey, 문서 200개까지 — 뷰어는 기억하지 않는다) 자동 규칙보다 앞선다. 강조 항목이 접힌 가지 안이면
// 보이는 가장 가까운 조상에 .on-within 을 단다.
// ⑧ 내용 위치 따라가기(⑤의 쪽 따라가기를 대체): pos 이벤트(hello 에도 pos·pos_frac·pos_total)로 호스트가 발표가 아닌 보기로 보고 있는
// 내용 위치 — 최상위 블록 번호 n(pager.js numberBlocks 의 data-n — 호스트와 같은 규칙)과 그 블록 안 비율 f — 를 받는다. follower 는 위치
// 하나를 따라간다 — 발표 중(slide ≥ 0)이면 슬라이드(보기 꼴을 슬라이드로 바꾼다), 아니고 pos ≥ 0 이면 내용 위치(보기 꼴은 바꾸지 않는다 —
// 쪽·이어보기는 블록 n 의 조각들 가운데 f 가 닿는 지점을 보기 영역 맨 위에, 슬라이드 보기는 블록 n 이 든 슬라이드를 가운데에). 풀림·「발표자
// 위치로」·hello·version·크기 변경 뒤 재정렬은 슬라이드와 같은 길이다. 꼴을 붙들지 않으므로 내용 위치를 따라가는 동안에도 쪽 | 이어보기
// 단추를 보인다(발표 슬라이드를 따라가는 동안만 숨김). 둘 다 -1 이면 놓는다(보기 꼴은 그대로 둔다).
// ⑥ 슬라이드 보기는 너비와 높이에 맞춰 축소해 한 장이 다 보이게 한다(slideZoom — 최소 0.2 라 아주 작은 화면에서는 일부가 잘릴 수 있다). 쪽 보기는 너비만 맞춘다.
//   보기 영역이 낮으면(SHORT_VIEW_H 미만 — 폰 가로) 높이를 보지 않고 너비를 여백 없이 채운다(slideFit) — 한 장이 세로로 다 보이지 않을 수 있고 굴려 본다.
// ⑦ 목차 강조의 id → 제목 Map 은 본문을 바꿀 때 한 번 만들고 스크롤 프레임은 그것을 쓴다(headingFinder 의 refresh·index).
// ⑨ 쪽 넘김 키: ←/→·PageUp/PageDown·Space/Shift+Space·Home/End 로 쪽 보기는 이전·다음 쪽 맨 위, 슬라이드 보기는 이전·다음 장 가운데,
// 이어보기는 보기 영역 높이 − 40px 만큼(pager.js pageNav — 문서 창 보기 모드와 같은 규칙). 키로 넘기면 따라가기가 이 화면에서 잠시 풀린다(keyHandler).
// 발표자 노트(itda-hyve#344 결정 7): 호스트가 공유의 「발표자 노트 함께 보이기」 를 켰을 때만 view.json 에 노트가 온다(꺼져 있으면 서버가 뺀다).
// 노트가 온 슬라이드 보기에서만 「노트: 끔 | 오른쪽 | 아래」(기본 아래 — 폰)와 글자 A−/A+ 를 보인다. 노트 칸은 문서 창 미리보기와 같은
// 부품이다(slides.js placeNotes·호스트 앱 창 틀 CSS 의 .slide-note). 쪽·이어보기에는 노트가 없다. 오른쪽 칸은 왼쪽 손잡이를 끌어 240~640px 로
// 넓히고 좁힌다(bindNoteGrip — 두 번 누르면 320). 고른 배치·글자 크기·칸 너비는 이 브라우저의 localStorage 에만 둔다(못 쓰면 메모리 — notePrefs). 발표 슬라이드를 따라가는 중 노트가 아래면 그 슬라이드를 위에 붙여 아래 노트까지 보이게 한다(followSpot).
// 확장(itda-hyve#617): share.js 보다 먼저 실행되는 스크립트가 window.ItdaShareExt(api)(옛 이름 window.HyveShareExt 도 본다) 를 두면, 시작할 때 한 번 불러 화면 부품(api — 아래
// 「확장을 시작한다」 절)을 넘기고 돌려받은 훅을 그리기 길에서 부른다 — flow(흐름 — 나누기 전, numberBlocks 뒤)·drawn(새 본문을 다 그린 뒤)·tick(목차
// 강조와 같은 때, 오류 view 포함)·ended(끝난 화면)·wide(목차 서랍 경계를 넘나듦)·resized(보기 영역 크기 바뀜)·files(폴더 목록이 바뀌어 본문·노트 칸 링크를 다시 건 뒤 —
// itda-hyve#1096, 확장은 api.markLinks 로 제 자리를 다시 건다). 없으면 아무것도 하지 않는다.
// 시작이나 훅이 던지면 콘솔에만 남기고 확장 없이처럼 이어 간다.
// 호스트 꼴 적용(itda-hyve#748): 확장이 api.setLook(name) 을 부르면 호스트가 고른 꼴로 다시 그린다 — 사람이 고른 것이 아니라 폴더 공유의 기억한 꼴(nav.setLook)에 적지 않는다.
// 쪽·이어보기면 state.forced 가 서서 슬라이드·보드 문서에서도 그 꼴로 그린다(lookFor). 사람이 꼴 단추를 누르거나, 발표 꼴로 바꾸거나(follower align), 따라가기가 꺼지면(release) 내려간다.
// 따라가기가 꺼질 때 슬라이드·보드 문서는 꼴 단추가 숨어 있어 chosen 도 비워 문서의 꼴로 돌아가고, 그 밖의 문서는 꼴을 그대로 둔다. 따라가기가 켜진 채 위치만 없어지면(호스트가 다른 탭으로 감) 꼴을 두고 다시 돌아오길 기다린다.
// 확장이 쓸 화면 자리는 share.html 의 {{EXT_*}} 다(서버가 채우거나 비운다).
// 같은 주소 공유(itda-hyve#705 3절): stop 의 까닭이 paused 이거나 hello 에 keep 이 있던 공유의 연결이 끊기면 끝난 화면으로 가지 않는다 — 문서를 두고 띠(#pause-band)를 보이고 다시 붙기를
// 시도한다(connect — 1·2·4·5·5…초, pauseBackoff). 띠의 「지금 다시 잇기」·망 복귀(online)·탭이 다시 보임은 기다리지 않고 바로 붙어 본다(retryNow·wake). 다시 붙으면 hello → sync.check 가 바뀐 문서를 받고 띠를 지운다. 새로 고치지 않으므로 읽던 자리가 남는다(다만 테마의 내용이 바뀌었으면 기존 reloadGate 대로 새로 고친다).
// 쿠키를 잃은 브라우저는 다시 붙지 못하므로 재시도가 지금 주소를 살펴 공유가 열렸으면 새로 고쳐 입장 화면으로 간다(hostSource resync). 같은 주소 공유의 자동 새로 고침은 모두 reloadGate 의 hold 를 지나고, 메모를 새로 고침이 지울 때는 확장이 막는다(holdReload — 없으면 막지 않는다).
// 앱 이름(itda-hyve#625): 탭 제목 접미 「 — <앱 이름>」 은 모듈에 박지 않는다 — 서버가 share.html 의 <meta name="application-name"> 자리를
// 채우면 그 이름을 붙이고, 비우거나 채우지 않으면(자리 글자 그대로) 접미 없이 문서 제목만 쓴다(appName·docTitle).
// 호스트 부품: 표 복사·표 정렬·코드 복사·코드 줄·YouTube 재생(share.html 이 share.js 앞에 싣는 호스트의 script)은 정식 이름 window.Itda*(ItdaTableCopy·
// ItdaTableSort·ItdaCodeCopy·ItdaCodeLines·ItdaYT)로 찾고, 없으면 옛 이름 window.Hyve* 로 물러선다(hostPart·extInit — 둘 다 pick). 이 파일은 window.ItdaShare 로
// 내보내고 옛 이름 window.HyveShare 도 같은 객체다. 호스트 부품이 없으면(설치 없는 뷰어 — web/viewer) 그 기능만 빠지고 그리기는 이어 간다.
// 강의 MCP와 번들 문서 id를 받은 경우 ask.js가 원문 제목에 서비스별 단추를 단다.
// YouTube 재생 부품(ItdaYT)의 꼴: init(o)(o = { inScope(a), src(id, start), referrerPolicy, noSrc() } — 누른 블록 a 를 그 자리 틀로 바꾼다)·
// stop()(지금 틀을 떼고 대표 그림을 되살린다). 부품이 없으면 YouTube 블록은 보통 링크다.
// View 원천(itda-hyve#577 2b): 받은 View 를 그리는 것(아래 「화면」 — render·draw·보기 꼴·목차·노트·표 정렬·쪽 넘김 키)과 View 를 어디서
// 받는지(원천)를 가른다. body 에 data-base 가 있으면(공유 화면 — share.html) 원천은 hostSource(view.json·SSE syncer·follower 에 넘길 이벤트·
// 끝남·kick·테마 CSS link·다시 불러오기) 하나로 정해지고 다른 것으로 바꿀 수 없다. data-base 가 없으면 share.js 보다 먼저 실행된 스크립트가 둔
// window.ItdaViewSource 를 쓴다(web/viewer 의 wasm 원천). 둘 다 없으면 화면을 시작하지 않는다(pickSource). 원천의 꼴은 hostSource 머리 주석.
// 처음 보기 꼴(itda-hyve#673): view.json 에 look(doc·flow·slides)이 오면 처음 한 번 chosen 으로 쓴다(initialLook). 보는 사람이 단추로 바꾸면 그쪽이 이긴다.
// 필드가 없으면 지금처럼 폭으로 고른다.
// 폴더 공유(itda-hyve#673): view.json 에 doc(폴더 안 상대 경로)이 올 때만 <base>/files.json 을 받아 사이드바 목차 위에 파일 목록을 그린다(mountFiles — share.html 에
// 자리가 없고 여기서 만든다). 필드가 없으면(단일 파일·문서 창·뷰어) 요청도 DOM 도 지금과 같다. 문서를 고르면 ?doc= 주소로 화면을 처음부터 불러온다 — 뒤로 가기·새 로고침이
// 통하고, 문서별 테마 CSS·메모 식별값이 그 화면의 것이 된다. 고른 보기 꼴만 이 탭의 sessionStorage 로 옮긴다. 문서 안 상대 링크는 목록에 있으면 그 문서로(docLink), 없으면
// 지금처럼 열지 않는다. 열지 않는 링크는 그린 뒤 링크 모양 없이 끄고 title 에 까닭을 단다(markLinks — itda-hyve#1096, 목록이 늦게 오면 다시 건다). 갱신 스트림의 files(hello 에는 files_rev)가 오면 목록을 다시 받는다. 세션이 있는데 문서만 없으면 서버가 표시 머리(docMissing)를 단 404 에 오류 view 를
// 담아 준다 — 공유 끝이 아니므로 오류 view 를 그리고 파일 목록을 남긴다. 표시가 없는 404 는 지금처럼 공유 끝이다. 그 문서의 스트림은 404 라 files 가 오지 않으므로, 다시 붙기를
// 기다리는 동안의 재동기화(viewState·resync)가 목록을 잇고 문서가 돌아오면 본문을 되살린다.
(function () {
  "use strict";

  const NARROW = 768;
  const PX_PER_MM = 96 / 25.4;

  // boardOK 는 이 화면이 보드 꼴을 그릴 수 있는가다(itda-work/itda-hyve#362). 화면이 시작할 때 window.ItdaBoard(web/dashboard.js)가
  // 있는지로 한 번 정한다(setBoardOK — 아래 「화면」). 공유 화면(호스트가 dashboard.js 를 싣는다)은 참, 웹 뷰어(싣지 않는다 —
  // scripts/build-web.sh)는 거짓이라 보드 문서를 쪽 보기로 그린다. 화면을 시작하지 않는 쓰임(시험)은 기본값 참이다.
  let boardOK = true;
  function setBoardOK(ok) { boardOK = !!ok; }
  // boardDoc 은 보드로 열 문서인가다 — 문서에 dashboard: 가 있고 이 화면이 보드를 그릴 수 있다(lookFor·lookSwitchHidden 이 같이 쓴다).
  function boardDoc(v) { return boardOK && !!(v && v.board && v.board.present); }

  // lookFor 는 그릴 보기 꼴이다(호스트 앱과 같은 이름 — "doc" 쪽·"flow" 이어보기·"slides"·"dashboard" 보드).
  // chosen 이 "slides"·"dashboard" 면(발표자 따라가기가 호스트의 발표 꼴로 바꿈 — itda-work/itda-hyve#362) 그 꼴이 문서의 꼴보다 앞선다.
  // 그 밖에는 보드 문서(board.present)는 보드(호스트의 처음 꼴과 같이 slides: 보다 앞선다), 슬라이드 문서는 슬라이드다.
  // 쪽 따라가기는 chosen 을 "doc" 으로 바꾼다. 보드를 그릴 수 없는 화면(boardOK 거짓 — dashboard.js 를 싣지 않은 웹 뷰어)에서는 보드를
  // 고르지 않는다 — board.present 와 chosen "dashboard" 를 없는 것으로 보고 아래 규칙으로 고른다(쪽 문서와 같이).
  // forced 가 참이면(호스트의 꼴 전환 — setLook, itda-hyve#748) chosen 의 쪽·이어보기도 문서의 꼴보다 앞선다. 발표 꼴은 그보다 앞이다. forced 를 넘기지 않으면 chosen 의 쪽·이어보기는 슬라이드·보드 문서를 바꾸지 않는다 —
  // 폴더 공유의 기억한 꼴·view.json 의 look·좁은 화면의 처음 꼴이 이 길이다.
  function lookFor(v, width, chosen, forced) {
    if (chosen === "slides" || (chosen === "dashboard" && boardOK)) return chosen;
    if (forced && (chosen === "doc" || chosen === "flow")) return chosen;
    if (boardDoc(v)) return "dashboard";
    if (v && v.slides && v.slides.present) return "slides";
    if (chosen === "doc" || chosen === "flow") return chosen;
    return width <= NARROW ? "flow" : "doc";
  }

  // initialLook 은 첫 view 로 정하는 chosen 이다: 이미 정해졌으면(발표자 따라가기) 그대로, 아니면 이 탭에서 사람이 고른 꼴(stored), 없으면 서버의 look. 아는 꼴이
  // 아니면 ""(폭으로 고른다 — lookFor).
  function initialLook(chosen, stored, look) {
    if (chosen) return chosen;
    for (const x of [stored, look]) if (x === "doc" || x === "flow" || x === "slides") return x;
    return "";
  }

  // docLink 는 문서 cur(폴더 안 상대 경로) 안의 상대 링크 href 를 푼다: 목록 docs 에 있는 문서면 {doc, hash}, 아니면 null(폴더 밖·절대·스킴·풀 수 없는 % 꼴·목록에 없는 파일).
  // 글자 꼴(NFC·NFD — macOS 파일 이름)이 달라도 같은 문서로 본다. doc 은 목록의 글자 그대로다.
  function docLink(href, cur, docs) {
    const m = /^([^?#]*)(?:\?[^#]*)?(#.*)?$/.exec(String(href));
    if (!m || !m[1] || m[1][0] === "/" || /^[a-z][a-z0-9+.-]*:/i.test(m[1])) return null;
    let p;
    try { p = decodeURIComponent(m[1]); } catch (e) { return null; }
    const parts = String(cur || "").split("/").slice(0, -1);
    for (const seg of p.split("/")) {
      if (seg === "" || seg === ".") continue;
      if (seg === "..") { if (!parts.length) return null; parts.pop(); continue; }
      parts.push(seg);
    }
    const want = parts.join("/").normalize("NFC");
    const doc = (docs || []).find(d => d.normalize("NFC") === want);
    return doc === undefined ? null : { doc, hash: m[2] || "" };
  }

  // LINK_OFF_MSG 는 열지 않는 문서 안 파일 링크의 까닭이다(원천이 linkMsg 를 주지 않을 때 — itda-hyve#1096). 알림(onClick)과 꺼진 링크의 title(markLinks)이 같은 글이다.
  const LINK_OFF_MSG = "공유 화면은 문서와 그 그림만 보입니다 — 이 파일은 받을 수 없습니다";

  // linkTo 는 문서 안 링크 href 가 이 화면에서 어디로 가는가다 — "hash"(#제목 — 이 문서 안)·"out"(바깥 http(s)·mailto·tel — 새 탭)·{doc, hash}(폴더 안 문서 —
  // docLink, docs 가 목록일 때만)·null(열지 않는 링크). onClick(누를 때)과 markLinks(그린 뒤 끄기)가 같이 쓴다 — 끄는 링크와 눌러서 안내만 하는 링크가 같은 집합이다.
  function linkTo(href, cur, docs) {
    const h = String(href == null ? "" : href);
    if (h.startsWith("#")) return "hash";
    if (/^(https?:|mailto:|tel:)/i.test(h)) return "out";
    return docs ? docLink(h, cur, docs) : null;
  }

  // markLinks 는 root 안 링크 가운데 열지 않는 것(linkTo 가 null 이고 원천이 살려 두지 않는 것 — o.live(a))을 링크 모양 없이 끈다(itda-hyve#1096):
  // href 를 data-local-href 로 옮기고(왼쪽·가운데 단추·새 탭으로도 어디로도 가지 않는다 — 뷰어의 localLinks 와 같은 꼴) class link-off·aria-disabled="true"·
  // title(o.msg — 저자가 단 title 이 있으면 그 뒤에 「 — 까닭」)을 단다. 키보드로 닿도록 tabindex="0"·role="link" 를 둔다(Enter 는 화면의 keydown 이 누르기로 바꾼다 —
  // 누르면 onClick 이 같은 글로 알린다). 이미 끈 링크가 다시 열리는 것이 되면(폴더 공유의 목록이 늦게 옴 — files) 되살린다. o: { cur, docs, live(a), msg }.
  function markLinks(root, o) {
    if (!root || !root.querySelectorAll) return;
    o = o || {};
    const msg = o.msg || LINK_OFF_MSG;
    for (const a of root.querySelectorAll("a[href], a[data-local-href]")) {
      const has = a.hasAttribute("href");
      const href = has ? a.getAttribute("href") : a.getAttribute("data-local-href");
      const off = !(o.live && o.live(a)) && linkTo(href, o.cur, o.docs) === null;
      const marked = a.classList.contains("link-off");
      if (off) {
        if (has) { a.setAttribute("data-local-href", href); a.removeAttribute("href"); }
        if (!marked) {
          const t = a.getAttribute("title");
          if (t !== null) a.setAttribute("data-link-title", t);
          a.classList.add("link-off");
        }
        const own = a.getAttribute("data-link-title");
        a.setAttribute("title", own ? own + " — " + msg : msg);
        a.setAttribute("aria-disabled", "true");
        a.setAttribute("role", "link");
        a.setAttribute("tabindex", "0");
      } else if (marked) {
        if (!has) { a.setAttribute("href", href); a.removeAttribute("data-local-href"); }
        const own = a.getAttribute("data-link-title");
        if (own !== null) a.setAttribute("title", own); else a.removeAttribute("title");
        for (const k of ["data-link-title", "aria-disabled", "role", "tabindex"]) a.removeAttribute(k);
        a.classList.remove("link-off");
      }
    }
  }

  // fileTreeHTML 은 문서 목록(슬래시 상대 경로)을 폴더별로 묶은 HTML 이다 — ul > li > a[href="?doc=…"], 폴더는 li > span.dir + ul. 글자·경로는 이스케이프한다.
  // cur 인 문서에 class on·aria-current 를 단다.
  function fileTreeHTML(docs, cur) {
    const root = { dirs: new Map(), files: [] };
    for (const d of docs) {
      const parts = d.split("/");
      let n = root;
      for (const p of parts.slice(0, -1)) {
        if (!n.dirs.has(p)) n.dirs.set(p, { dirs: new Map(), files: [] });
        n = n.dirs.get(p);
      }
      n.files.push({ name: parts[parts.length - 1], doc: d });
    }
    const list = n => {
      let h = "<ul>";
      for (const f of n.files) {
        const on = f.doc === cur ? ' class="on" aria-current="page"' : "";
        h += '<li><a href="?doc=' + esc(encodeURIComponent(f.doc)) + '"' + on + ">" + esc(f.name) + "</a></li>";
      }
      for (const [name, sub] of n.dirs) h += '<li><span class="dir">' + esc(name) + "</span>" + list(sub) + "</li>";
      return h + "</ul>";
    };
    return list(root);
  }

  // mountFiles 는 사이드바 맨 위의 파일 목록을 data({docs, truncated})로 그린다. data 가 없으면 있던 목록을 떼고 아무것도 만들지 않는다 — 단일 파일·문서 창의 사이드바는 그대로다.
  function mountFiles(doc, sidebar, data, cur) {
    const old = sidebar.querySelector("#side-files");
    if (!data || !Array.isArray(data.docs)) { if (old) old.remove(); return null; }
    const box = old || doc.createElement("section");
    if (!old) {
      box.id = "side-files";
      box.setAttribute("aria-label", "문서 목록");
      sidebar.insertBefore(box, sidebar.firstChild);
    }
    box.innerHTML = '<div class="side-title">문서</div><nav>' + fileTreeHTML(data.docs, cur) + "</nav>" +
      (data.truncated ? '<p class="files-more">폴더가 커서 목록 일부만 보입니다</p>' : "");
    return box;
  }

  // docMissing 은 폴더 공유의 문서 부재 응답인가다(공유·세션은 살아 있다). 표시가 없는 404 는 공유 끝이다.
  function docMissing(r) {
    return r.status === 404 && !!r.headers && r.headers.get("X-Share-Doc") === "missing";
  }

  // viewState 는 view 응답 하나({status, missing})가 말하는 상태다 — 공유 생존과 문서 가용성을 가른다: "alive"(살아 있음)·"missing"(문서 없음 — 공유는 살아 있다)·
  // "ended"(끝남)·"kicked"(끝남 — 이 화면을 끊음). 그 밖의 실패(429·5xx)는 공유가 끝났다는 말이 아니라 "alive" 다.
  function viewState(r) {
    if (r.missing) return "missing";
    return r.status === 403 ? "kicked" : r.status === 404 ? "ended" : "alive";
  }

  // backoff 는 n 번째(0부터) 다시 붙기 전의 대기다(ms) — 1·2·4·8·16·30·30…초.
  function backoff(n) { return Math.min(30000, 1000 * Math.pow(2, n)); }

  // pauseBackoff 는 잠시 닫힘(connect 의 c.paused) 동안 n 번째 다시 붙기 전의 대기다(ms) — 1·2·4·5·5…초. 자동 재시도 사이의 대기 상한이 5초다(요청 처리 시간은 따로 — itda-hyve#728 후속).
  function pauseBackoff(n) { return Math.min(5000, 1000 * Math.pow(2, n)); }

  // PAUSE_MAX 는 잠시 닫힘이 시작된 뒤 다시 붙기를 시도하는 시간이다(ms) — 지나면 멈추고 새로 고침을 안내한다.
  const PAUSE_MAX = 24 * 3600 * 1000;

  // parseData 는 이벤트 data(JSON)다 — 틀리면 {}.
  function parseData(e) { try { return JSON.parse(e && e.data) || {}; } catch (err) { return {}; } }

  // connect 는 갱신 스트림의 상태 기계다. 상태: connecting → open(hello) → waiting(끊김, 다시 붙기 전) → … / ended(stop·kick·공유 없음) / idle(잠시 닫힘이 PAUSE_MAX 를 넘겨 멈춤).
  // 잠시 닫힘(같은 주소 공유): stop 의 까닭이 paused 이거나 hello 에 keep 이 있던 공유의 스트림이 끊기면 c.paused(시작 시각)를 두고 onPaused("closed") — 끝내지 않는다. 이때 probe 의
  // "ended"(404)는 재시도이고 "kicked"(403)만 끝낸다. 망 실패는 syncer 가 "alive" 로 돌려주므로 재시도다. 다시 붙은 hello 가 onPaused("") 로 거둔다. PAUSE_MAX 가 지나면 onPaused("stale") 로
  // 바꾸고 다시 붙지 않는다. keep 은 hello 마다 고친다 — 같은 주소가 아닌 공유는 이 길을 타지 않는다. 잠시 닫힘의 재시도 간격은 pauseBackoff(상한 5초), 그 밖의 끊김은 backoff(상한 30초)다.
  // c.retryNow() 는 잠시 닫힘에서 기다리지 않고 바로 붙어 본다(띠의 「지금 다시 잇기」): 기다리던 타이머를 지우고 tries 를 0 으로 open. 멈춤(idle — stale)이면 c.paused 를 지금으로 두고
  // onPaused("closed") 뒤 다시 PAUSE_MAX 동안 시도한다. 붙는 중(connecting)이면 새로 열지 않는다(스트림 하나). 상태를 묻는 중(probe)이면 바로 열고 늦게 온 그 답은 버린다(gen). 이번 시도가 끝나면 풀리는 Promise 를 돌려준다 — hello 면 true,
  // 실패해 다음 시도를 걸었거나·멈췄거나·끝났으면 false. 잠시 닫힘이 아니거나 끝났으면 아무것도 하지 않고 false.
  // deps: EventSource, probe()(끊긴 뒤 부른다 — viewState 의 값을 돌려준다. "alive"·"missing" 이면 다시 붙고 "ended"·"kicked" 면 끝낸다), setTimeout·clearTimeout, now()(없으면 Date.now),
  // onHello(first, data), onReload(), onVersion(data), onSlide(data — {index, total, look, follow}), onPos(data — {block, frac, total, follow}, ⑧),
  // onFiles()(폴더의 문서 목록이 바뀜 — 없어도 된다), onEnded(reason — kick·403 은 "kicked"), onPaused(state — "closed"·"stale"·"", 같은 주소 공유만).
  // gone(reason)은 view.json 이 공유 없음("")·끊김("kicked")을 말할 때다(syncer) — 같은 주소 공유의 공유 없음은 끝이 아니라 잠시 닫힘이다.
  function connect(base, deps) {
    const c = { state: "connecting", tries: 0, es: null, timer: 0, hellos: 0, keep: false, paused: 0 };
    const now = () => deps.now ? deps.now() : Date.now();
    // waiters 는 retryNow 가 돌려준 Promise 의 resolve 들이다 — 이번 시도의 끝(settle)에서 한 번에 푼다.
    let waiters = [];
    function settle(ok) { const ws = waiters; waiters = []; for (const f of ws) f(ok); }
    // gen 은 시도 세대다 — open·lost·end 가 올린다. lost 의 probe 는 건 때의 세대가 아직 지금 세대일 때만 결과를 쓴다(그 사이 retryNow·wake 가 새로 열었으면 버린다 —
    // waiters 는 새 시도의 끝이 푼다). 연결 시도도 다음 타이머도 늘 하나다(arm 은 기존 타이머를 지우고 건다, open 은 남은 스트림을 닫고 연다).
    let gen = 0;
    function arm(ms) { deps.clearTimeout(c.timer); c.timer = deps.setTimeout(open, ms); }
    function pause() {
      c.keep = true;
      if (c.paused) return;
      c.paused = now();
      deps.onPaused("closed");
    }
    // lost 는 스트림을 잃었을 때다 — 지금 스트림을 닫고 상태를 물어 끝내거나 다시 붙는다.
    function lost() {
      if (c.es) { c.es.close(); c.es = null; }
      c.state = "waiting";
      const g = ++gen;
      Promise.resolve(deps.probe()).then(st => {
        if (g !== gen || c.state !== "waiting" || c.es) return; // 지난 세대의 probe — 그 뒤에 새 시도가 있었다
        if (st === "kicked") { end("kicked"); return; }
        if (st === "ended" && !c.paused) { end(""); return; }
        if (c.paused && now() - c.paused >= PAUSE_MAX) {
          c.state = "idle";
          deps.onPaused("stale");
          settle(false);
          return;
        }
        arm((c.paused ? pauseBackoff : backoff)(c.tries++));
        settle(false);
      });
    }
    function end(reason) {
      if (c.state === "ended") return;
      c.state = "ended";
      gen++;
      if (c.es) { c.es.close(); c.es = null; }
      deps.clearTimeout(c.timer);
      deps.onEnded(reason || "");
      settle(false);
    }
    function open() {
      if (c.state === "ended") return;
      deps.clearTimeout(c.timer);
      gen++;
      if (c.es) { c.es.close(); c.es = null; } // 스트림은 늘 하나 — 남은 것을 닫고 연다(닫힌 것의 이벤트는 c.es !== es 로 버린다)
      c.state = "connecting";
      const es = new deps.EventSource(base + "/events");
      c.es = es;
      es.addEventListener("hello", e => {
        if (c.es !== es) return;
        c.state = "open";
        c.tries = 0;
        const data = parseData(e);
        c.keep = !!data.keep;
        if (c.paused) { c.paused = 0; deps.onPaused(""); }
        settle(true);
        deps.onHello(c.hellos++ === 0, data);
      });
      es.addEventListener("reload", () => {
        if (c.es !== es || c.state === "ended") return;
        deps.onReload();
      });
      es.addEventListener("version", e => {
        if (c.es !== es || c.state === "ended") return;
        deps.onVersion(parseData(e));
      });
      es.addEventListener("slide", e => {
        if (c.es !== es || c.state === "ended") return;
        deps.onSlide(parseData(e));
      });
      es.addEventListener("pos", e => {
        if (c.es !== es || c.state === "ended" || !deps.onPos) return;
        deps.onPos(parseData(e));
      });
      es.addEventListener("files", () => {
        if (c.es !== es || c.state === "ended" || !deps.onFiles) return;
        deps.onFiles();
      });
      es.addEventListener("kick", () => {
        if (c.es !== es) return;
        end("kicked");
      });
      es.addEventListener("stop", e => {
        if (c.es !== es) return;
        let reason = "";
        try { reason = JSON.parse(e.data).reason || ""; } catch (err) { /* 까닭 없음 */ }
        if (reason === "paused") { pause(); lost(); return; }
        end(reason);
      });
      es.onerror = () => {
        if (c.es !== es || c.state === "ended") return;
        if (c.keep) pause();
        lost();
      };
    }
    // wake 는 화면이 다시 보일 때(뒤로 갔다 옴·탭 전환) 기다리지 않고 바로 다시 붙는다.
    c.wake = () => { if (c.state === "waiting" && !c.es) { c.tries = 0; open(); } };
    c.retryNow = () => {
      if (c.state === "ended" || !c.paused) return Promise.resolve(false);
      const p = new Promise(r => waiters.push(r));
      if (c.state === "idle") { // 멈춘 뒤의 단추: 다시 PAUSE_MAX 동안 시도한다
        c.paused = now();
        c.state = "waiting";
        deps.onPaused("closed");
      }
      c.wake(); // 붙는 중이면 그 시도를 기다린다
      return p;
    };
    c.gone = reason => {
      if (reason === "kicked" || !c.keep) { end(reason); return; }
      if (c.state === "ended" || c.state === "idle") return;
      pause();
      if (c.state !== "waiting") lost(); // 기다리는 중이면 이미 다음 시도가 있다
    };
    c.end = end;
    open();
    return c;
  }

  // reloadGate 는 화면을 다시 불러오는 두 길이다. deps: storage()(sessionStorage — 던질 수 있다), now(), reload()(location.reload),
  // ended()(공유가 끝났으면 참), failed()(「테마를 불러오지 못했습니다」 안내), setTimeout·clearTimeout.
  //  - once(why): 테마 CSS 를 받지 못했을 때(error)만. 10초 안에 이미 다시 불렀으면(sessionStorage
  //    "hs-reload:<base>" — 다시 불러와도 남는다) 다시 부르지 않고 안내한다(되풀이 방지). sessionStorage 를 못 쓰면 안내만 한다.
  //  - server(): 서버가 알린 변경 — reload 이벤트, 그리고 hello·version 으로 받은 view 의 테마가 이 페이지의 테마와 다를 때(테마를
  //    바꾼 저장 — syncer·themeApplier). 호스트가 테마를 여러 번 바꿔도 따라가되 2초에 한 번만 다시 부른다. 마지막 시각은
  //    sessionStorage "hs-reload-srv:<base>"(once 의 키와 따로)에 두어 다시 불러온 페이지에도 남는다. 2초 안에 또 오면 버리지 않고
  //    남은 시간 뒤에 한 번 다시 부른다(지연 타이머는 하나만 — 마지막 변경을 놓치지 않게). 저장소를 못 쓰면 페이지 안에서만 2초 보호.
  //  - cancel(): 공유가 끝났을 때 — 걸어 둔 지연 타이머를 거둔다.
  // 공유가 끝났으면 아무것도 하지 않는다(지연 타이머가 울려도). 바로 다시 불렀으면 true.
  function reloadGate(base, deps) {
    const key = "hs-reload:" + base, srvKey = "hs-reload-srv:" + base;
    let lastServer = -Infinity, timer = 0;
    function lastSrv() {
      try { const at = Number(deps.storage().getItem(srvKey)); if (at) return Math.max(at, lastServer); } catch (e) { /* 메모리 시각 */ }
      return lastServer;
    }
    const held = () => !!deps.hold && deps.hold();
    function fire(now) {
      if (held()) return false;
      lastServer = now;
      try { deps.storage().setItem(srvKey, String(now)); } catch (e) { /* 페이지 안에서만 보호 */ }
      deps.reload();
      return true;
    }
    return {
      once(why) {
        if (deps.ended() || held()) return false;
        const now = deps.now();
        try {
          const st = deps.storage();
          const at = Number(st.getItem(key)) || 0;
          if (at && now - at >= 0 && now - at < 10000) { deps.failed(why); return false; }
          st.setItem(key, String(now));
        } catch (e) { deps.failed(why); return false; }
        deps.reload();
        return true;
      },
      server() {
        if (deps.ended() || timer) return false;
        const now = deps.now(), since = now - lastSrv();
        if (since >= 0 && since < 2000) {
          timer = deps.setTimeout(() => { timer = 0; if (!deps.ended()) fire(deps.now()); }, 2000 - since);
          return false;
        }
        return fire(now);
      },
      cancel() { if (timer) { deps.clearTimeout(timer); timer = 0; } },
    };
  }

  // themeKey 는 view 의 테마 판이다(이름@테마 판).
  // theme_hash 는 내용 지문이라 서버를 다시 띄워도 같은 테마면 같고, 내용이 바뀌면 판 번호가 같아도 다르다(itda-hyve#705 — theme_rev 는 프로세스마다 처음부터 센다). 지문이 없는 서버는 theme_rev 다.
  function themeStamp(v) { return v.theme_hash || v.theme_rev || 0; }
  function themeKey(v) { return v.theme + "@" + themeStamp(v); }

  // themeApplier 는 테마 CSS 를 <link>(link)에 한 번 넣는 함수를 만든다(정책 D — 화면을 불러올 때 한 번). 돌려준 apply(v) 는:
  //  - 처음이면 v 의 테마를 넣고 load·error 를 기다린다. 3초 넘으면 기다리지 않는다(느린 망에서 그리기를 막지 않는다). 리스너는 떼지
  //    않는다 — 늦은 load 면 deps.late()(쪽 나눔 다시), error 는 언제 오든 deps.reload("theme").
  //  - 처음과 같은 테마면 그대로, 다른 테마면 CSS 를 바꾸지 않고 그리지 않은 채 deps.changed()(gate.server — 서버가 알린 변경과 같은 길).
  // 돌려줌(Promise): 이 view 로 그려도 되면 true, 다시 불러오는 중이면 false.
  function themeApplier(link, base, deps) {
    let key = "";
    const apply = v => {
      const k = themeKey(v);
      if (key) {
        if (k === key) return Promise.resolve(true);
        deps.changed();
        return Promise.resolve(false);
      }
      key = k;
      return new Promise(resolve => {
        let done = false;
        const finish = ok => { if (!done) { done = true; resolve(ok); } };
        link.addEventListener("load", () => { if (done) deps.late(); else finish(true); });
        link.addEventListener("error", () => { finish(!deps.reload("theme")); });
        link.setAttribute("href", base + "/_/_theme/" + encodeURIComponent(v.theme) + "/theme.css?v=" + themeStamp(v));
        deps.setTimer(() => finish(true), 3000);
      });
    };
    apply.key = () => key; // 이 페이지가 넣은 테마의 키(아직 없으면 "")
    return apply;
  }

  // fontGate 는 쪽·슬라이드를 나누기 전의 글꼴 기다림을 만든다(itda-doc#62). fonts 는 document.fonts, deps 는 setTimer(f, ms)·late()다. fonts.ready 만으로는
  // 모자란다 — 본문을 넣기 전에는 받는 글꼴이 없어 곧바로 풀린다. 그래서 선언된 face 를 load 한다. 끝남은 성공과 실패를 함께 뜻한다. 돌려주는 것:
  //  - wait(): 끝나지 않은 face 를 받기 시작하고, 끝나면(남은 face 가 없으면 fonts.ready 까지) 풀린다. ms 를 넘기면 그대로 풀린다. 이미 끝났으면 타이머 없이 풀린다.
  //    behind() 가 적어 둔 face 는 기다리지 않는다 — 끝나지 않는 글꼴이 다시 그릴 때마다 ms 씩 늦추지 않게. 새 face 가 섞이면 새 face 만 기다린다.
  //  - behind(): 지금 끝나지 않은 face 가 있으면 true 다. 그 face 를 적어 두고, 하나씩 도착할 때마다 late() 를 예약한다. 몰린 도착은 FONT_GATHER 안에서 한 번으로 합친다.
  //    실패한 face 는 나눔을 바꾸지 않으므로 late() 를 부르지 않는다.
  const FONT_GATHER = 100;
  function fontGate(fonts, ms, deps) {
    const open = () => [...fonts].filter(f => f.status !== "loaded" && f.status !== "error");
    const overdue = new WeakSet();
    let gathering = false;
    const arrived = () => {
      if (gathering) return;
      gathering = true;
      deps.setTimer(() => { gathering = false; deps.late(); }, FONT_GATHER);
    };
    return {
      wait() {
        const fresh = open().filter(f => !overdue.has(f));
        // 적어 둔 face 가 남아 있으면 fonts.ready 는 풀리지 않는다 — 그때는 ready 를 보지 않는다
        const all = Promise.allSettled(fresh.map(f => f.load())).then(() => open().length ? null : fonts.ready).catch(() => {});
        return fresh.length ? Promise.race([all, new Promise(r => deps.setTimer(r, ms))]) : all;
      },
      behind() {
        const left = open();
        for (const f of left) {
          if (overdue.has(f)) continue;
          overdue.add(f);
          f.load().then(arrived, () => {});
        }
        return left.length > 0;
      },
    };
  }

  // syncer 는 view.json 다시 받기다(refresh — 처음·hello·version·연결 복구). deps:
  //   fetch() → Promise<{status, view, missing}>(view 는 200·문서 부재일 때만, missing 은 문서 부재), current()(지금 그린 view 또는 null), pageKey()(이 페이지가 넣은 테마
  //   CSS 의 키 — 아직 없으면 ""), apply(v)(view 를 바꾸고 그린다 — Promise), gate(reloadGate), ended(), end(reason)(공유 끝),
  //   failed()(「문서를 받지 못했습니다」), setTimeout·clearTimeout.
  //  - 세대 번호: refresh 마다 하나씩 올리고, 응답이 왔을 때 더 뒤에 시작한 refresh 가 있으면 그 응답을 버린다 — 늦게 온 옛 응답이
  //    최신 view 를 덮지 않는다(응답 역전).
  //  - 받은 view 의 판·테마 키가 지금 것과 같으면(force 가 아니면) 그리지 않는다. 테마 키가 이 페이지의 테마와 다르면 그리지 않고
  //    다시 불러온다(gate.server() — 서버가 알린 변경이라 reload 이벤트와 같은 길: 2초 보호 + 지연 한 번. CSS 실패용 once 의 10초
  //    보호는 쓰지 않는다 — 10초 안에 테마를 두 번 바꿔 저장해도 멈추지 않게), 같으면 제자리에서 다시 그린다.
  //  - 404 면 공유 끝, 403 이면 이 화면을 끊음, 429 면 1·2·4…30초 뒤 다시(backoff), 그 밖의 실패는 안내. 문서 부재는 받은 오류 view 를 그린다.
  //  - refresh 는 받은 응답의 상태(viewState)를 돌려준다. 망 실패는 "alive" 다.
  //  - recovering(연결 복구 — hostSource resync): 받은 view 를 그리는 것은 같되 안내·끝내기·429 재시도는 하지 않고 상태만 돌려준다 — 끝내기와 다음 시도는 connect 의 몫이다.
  //  - check(data): hello·version 의 data({version, theme, theme_rev, theme_hash})가 지금 view 와 다르면 refresh. 같으면 아무것도 하지 않는다.
  function syncer(deps) {
    let gen = 0, tries = 0, timer = 0;
    function differs(d) {
      const v = deps.current();
      return !v || !d || v.version !== d.version || themeKey(v) !== themeKey(d);
    }
    async function refresh(force, recovering) {
      if (deps.ended()) return "ended";
      const my = ++gen;
      if (timer) { deps.clearTimeout(timer); timer = 0; }
      let r;
      try { r = await deps.fetch(); } catch (e) {
        if (!recovering && my === gen && !deps.ended()) deps.failed();
        return "alive";
      }
      if (deps.ended()) return "ended";
      const st = viewState(r);
      if (my !== gen) return st; // 더 뒤의 refresh 가 있다 — 이 응답은 그리지 않는다
      if (st === "ended" || st === "kicked") { if (!recovering) deps.end(st === "kicked" ? "kicked" : ""); return st; }
      if (r.status === 429) { if (!recovering) timer = deps.setTimeout(() => { timer = 0; refresh(force); }, backoff(tries++)); return st; }
      if (!r.view) { if (!recovering) deps.failed(); return st; }
      tries = 0;
      const v = r.view;
      if (!force && !differs(v)) return st;
      const key = deps.pageKey();
      if (key && !v.error && themeKey(v) !== key) { deps.gate.server(); return st; }
      await deps.apply(v);
      return st;
    }
    return {
      refresh,
      differs,
      check(d) { if (differs(d)) refresh(false); },
      cancel() { if (timer) { deps.clearTimeout(timer); timer = 0; } gen++; },
    };
  }

  // follower 는 발표자 따라가기의 상태 기계다. 상태: follow(서버 값 — 슬라이드·내용 위치가 함께 쓴다)·slide·slideTotal(호스트의 발표
  // 슬라이드, -1 = 발표 아님)·pos·posFrac·posTotal(호스트가 발표가 아닌 보기로 보는 최상위 블록·그 안 비율·블록 수, -1 = 아님 — ⑧)·
  // paused(보는 사람이 직접 움직여 이 화면에서만 잠시 풀림).
  // 따라갈 위치는 하나다(kind·index·frac·total): slide ≥ 0 이면 "slide", 아니고 pos ≥ 0 이면 "pos", 둘 다 -1 이면 없음(kind "").
  // 보드 발표(slideLook "dashboard")는 번호가 없는 굴림이라 slide(0 / 1)는 꼴만 붙들고, pos 가 있으면 "pos" 로 따라간다.
  // deps: align(i, kind, frac, look)(맞춘다 — look 은 호스트의 발표 보기 꼴(slides·dashboard) — 슬라이드는 보기 꼴을 그 꼴로 바꿔 가운데,
  // 내용 위치는 꼴을 그대로 둔 채 — 다만 보드 발표면 look 이 "dashboard" 라 보드로 바꿔 맞춘다 — followAction),
  // release()(따라가는 표시를 뗀다 — 보기 꼴은 되돌리지 않는다), button(label)(「발표자 위치로」 단추를 그 글로 보임 — "" 이면 숨김, backLabel),
  // changed()(상태가 바뀔 때마다 맨 끝에 한 번 — 화면은 쪽 | 이어보기 단추의 숨김을 고친다).
  //  - follow && !paused && 위치가 있으면 맞춘다(active). 단추는 숨긴다.
  //  - follow && paused && 위치가 있으면 맞추지 않고 단추를 보인다.
  //  - follow 가 꺼지거나 위치가 없어지면 단추를 숨기고 놓는다. 그때 풀림도 거둔다 — 다음 발표·읽기·다시 켬은 처음부터 따라간다.
  //    위치의 꼴이 바뀌어도(읽기 → 발표 시작, 발표 끝 → 읽기) 풀림을 거둔다 — 새 꼴은 처음부터 따라간다.
  //  - onSlide(d): {index, total, look, follow}. onPos(d): {block, frac, total, follow} — 슬라이드 발표 중(presentLook "slides")에 온 pos 는 저장만 한다(follow 가
  //    그대로면 맞추지 않는다).
  //  - moved(): 맞추는 중에 사람이 움직이면 풀린다. resume(): 단추 — 풀림을 거두고 바로 맞춘다.
  //  - hello(data): hello 의 slide·slide_total·slide_look·pos·pos_frac·pos_total·follow 로 맞춘다(늦게 들어온 화면·다시 붙음). 값이 없으면
  //    아무것도 하지 않는다.
  // presentLook 은 「발표가 붙든 꼴」 이다(itda-work/itda-hyve#362 — "slides"·"dashboard"·null). 보기 꼴의 복귀는 이벤트 종류가 아니라 이
  // 상태의 전이로만 가린다(slide·pos 는 호스트가 따로 합쳐 보내 순서가 보장되지 않는다):
  //   1. slide{index ≥ 0, look}·hello 의 slide_look(따라가기 켜짐) → presentLook = look. align(번호, "slide", 0, look) 이 꼴이 다르면 그
  //      꼴로 다시 그린 뒤 번호로 맞춘다(followAction "redraw").
  //   2. slide{index: -1}(발표 끝) → presentLook = null. release() 가 문서 자신의 꼴로 돌아가고(화면 — restoreOwnLook), 유효한 pos 가
  //      있으면 이어서 align(pos) — 화면은 다시 그린 뒤 render 끝에서 그 앵커로 맞춘다(fol.active()).
  //   3. 따라가기 끔 → 2 와 같다(release). 다시 켬 → 저장된 발표(slide ≥ 0)가 있으면 1, 없으면 pos.
  //   4. pos → presentLook 이 "slides" 면 저장만(발표가 붙들고 있다), null·"dashboard" 면 앵커로 맞춘다(보드 발표는 내용 위치가 곧 발표 위치다).
  //   꼴이 없는 slide(옛 호스트)는 "slides" 로 읽는다(lookName).
  function follower(deps) {
    const f = { follow: false, slide: -1, slideTotal: 0, slideLook: "", pos: -1, posFrac: 0, posTotal: 0, paused: false, kind: "", index: -1, frac: 0, total: 0,
      presentLook: null };
    function update() {
      const kind = f.slide >= 0 && !(f.slideLook === "dashboard" && f.pos >= 0) ? "slide" : f.pos >= 0 ? "pos" : "";
      const held = f.follow && f.slide >= 0 ? f.slideLook : null; // presentLook 규칙 1·2·3
      if (kind !== f.kind || held !== f.presentLook) f.paused = false; // 위치의 꼴이나 발표가 붙든 꼴이 바뀌면 처음부터 따라간다
      f.kind = kind;
      f.index = kind === "slide" ? f.slide : kind === "pos" ? f.pos : -1;
      f.frac = kind === "pos" ? f.posFrac : 0;
      f.total = kind === "slide" ? f.slideTotal : kind === "pos" ? f.posTotal : 0;
      const freed = f.presentLook !== null && held === null;
      f.presentLook = held;
      const on = f.follow && !!kind;
      if (freed && on) deps.release(); // 규칙 2: 발표가 놓은 꼴을 먼저 되돌리고 아래에서 pos 로 맞춘다
      if (!on) { f.paused = false; deps.button(""); deps.release(); }
      else if (f.paused) deps.button(backLabel(kind, f.index, f.total));
      else { deps.button(""); deps.align(f.index, kind, f.frac, f.presentLook || ""); }
      if (deps.changed) deps.changed();
    }
    f.active = () => f.follow && !f.paused && !!f.kind;
    f.following = () => f.follow && !!f.kind; // 따라갈 위치가 있고 따라가기가 켜짐(이 화면이 잠시 풀렸어도)
    f.holdsLook = () => f.presentLook !== null; // 보기 꼴을 붙드는 따라가기(발표가 붙든 꼴 — 풀린 동안 포함)
    f.onSlide = d => {
      if (!d || typeof d.index !== "number") return;
      f.follow = !!d.follow;
      f.slide = d.index;
      f.slideTotal = Number(d.total) || 0;
      f.slideLook = lookName(d.look);
      update();
    };
    f.onPos = d => {
      if (!d || typeof d.block !== "number") return;
      const was = f.follow;
      f.follow = !!d.follow;
      f.pos = d.block;
      f.posFrac = clampFrac(d.frac);
      f.posTotal = Number(d.total) || 0;
      if (f.presentLook === "slides" && f.follow === was) return; // 규칙 4: 슬라이드 발표가 꼴을 붙들고 있으면 저장만
      update();
    };
    f.hello = d => {
      if (!d || typeof d.slide !== "number") return;
      f.follow = !!d.follow;
      f.slide = d.slide;
      f.slideTotal = Number(d.slide_total) || 0;
      f.slideLook = lookName(d.slide_look);
      if (typeof d.pos === "number") { f.pos = d.pos; f.posFrac = clampFrac(d.pos_frac); f.posTotal = Number(d.pos_total) || 0; }
      update();
    };
    f.moved = () => { if (f.active()) { f.paused = true; update(); } };
    f.resume = () => { if (f.following()) { f.paused = false; update(); } };
    return f;
  }
  // lookName 은 발표 보기 꼴 값이다 — "dashboard" 가 아니면 "slides"(옛 호스트·빈 값은 슬라이드).
  function lookName(x) { return x === "dashboard" ? "dashboard" : "slides"; }
  function clampFrac(x) { const v = Number(x); return v > 0 ? Math.min(1, v) : 0; }

  // backLabel 은 풀린 동안의 「발표자 위치로」 단추 글이다. 발표 슬라이드는 (n/N), 내용 위치는 숫자 없이 — 블록 번호는 사람에게 뜻이
  // 없고 쪽 수는 기기마다 다르다(⑧).
  function backLabel(kind, index, total) {
    return kind === "slide" ? "발표자 위치로 (" + (index + 1) + "/" + total + ")" : "발표자 위치로";
  }

  // followAction 은 맞출 때 할 일이다: 발표 슬라이드는 지금 꼴이 호스트의 발표 꼴(want — "slides"·"dashboard", 없으면 "slides")이
  // 아니면 그 꼴로 바꿔 다시 그린 뒤 맞추고("redraw" — 그 꼴로 그릴 수 없으면 "none"), 같으면 번호로 맞춘다("scroll").
  // 내용 위치는 보기 꼴을 바꾸지 않고 지금 꼴에서 맞춘다("scroll" — ⑤처럼 쪽으로 바꾸지 않는다). 다만 보드 발표의 내용 위치(want "dashboard")는
  // 발표가 꼴을 붙들고 있어 슬라이드와 같이 그 꼴로 바꾼다 — 그릴 수 없는 화면(웹 뷰어)은 지금 꼴에서 맞춘다.
  function followAction(kind, look, v, width, want) {
    if (kind !== "slide" && want !== "dashboard") return "scroll";
    want = want === "dashboard" ? "dashboard" : "slides";
    if (look === want) return "scroll";
    if (lookFor(v, width, want) === want) return "redraw";
    return kind === "slide" ? "none" : "scroll";
  }

  // ytFollowGate 는 따라가기로 맞출 때 재생 중인 YouTube 를 멈출지 정하는 문지기를 만든다. 발표 슬라이드(kind "slide")는 직전에 따라간
  // 슬라이드 번호와 다를 때만 참 — 같은 슬라이드 안 조각 넘김·같은 번호 재적용(그리기 뒤 다시 맞춤 등)은 멈추지 않는다. 내용 위치(kind "pos")는
  // 늘 거짓이고 기억한 번호도 바꾸지 않는다 — 그 이동으로 플레이어가 화면 밖으로 나가면 호스트 부품 YT 가 정리한다.
  function ytFollowGate() {
    let last = null;
    return (i, kind) => {
      if (kind !== "slide") return false;
      const changed = i !== last;
      last = i;
      return changed;
    };
  }

  // pickBlock 은 블록 n 이 그려지지 않았을 때(가로줄·노트 등 그 꼴에 없는 블록) 쓸 번호다: ns 는 그려진 블록 번호(오름차순). n 이 있으면
  // n, 없으면 n 보다 작은 것 가운데 가장 큰 것, 그것도 없으면 가장 작은 것. 하나도 없으면 -1.
  function pickBlock(ns, n) {
    let below = -1;
    for (const x of ns) { if (x === n) return n; if (x < n) below = x; }
    return below >= 0 ? below : ns.length ? ns[0] : -1;
  }

  // fragAt 은 블록 하나의 조각 높이들(heights — 차례대로)에서 비율 f 가 닿는 조각과 그 안 깊이다({i, off}). 조각 사이 경계는 앞 조각의
  // 끝이 아니라 다음 조각의 머리로(쪽 사이 여백이 아니라 내용이 맨 위에 오게). 조각이 없으면 null.
  function fragAt(heights, f) {
    if (!heights.length) return null;
    const total = heights.reduce((a, h) => a + Math.max(0, h), 0);
    let target = clampFrac(f) * total, i = 0;
    for (; i < heights.length - 1; i++) {
      const h = Math.max(0, heights[i]);
      if (target < h) break;
      target -= h;
    }
    return { i, off: Math.min(target, Math.max(0, heights[i])) };
  }

  // lookSwitchHidden 은 쪽 | 이어보기 단추를 숨길지다. 원래 슬라이드 문서(slides.present)는 늘 숨기고, 그 밖의 문서는 발표자 따라가기가
  // 보기 꼴을 붙들고 있는 동안(holding — 발표 슬라이드를 따라가는 동안, 풀린 동안 포함)만 숨긴다. 내용 위치 따라가기(⑧)는 꼴을 붙들지
  // 않으므로 보인다. 발표가 끝나거나 따라가기가 꺼지면 보기 꼴은 그대로 두되 단추를 다시 보여 사람이 쪽·이어보기로 돌아갈 수 있다.
  // look 은 견주지 않는다(따라가는 꼴로 다시 그리는 중이어도 숨김).
  function lookSwitchHidden(v, look, holding) {
    if ((v && v.slides && v.slides.present) || boardDoc(v)) return true;
    return !!holding;
  }

  // restoreOwnLook 은 follower 의 presentLook 규칙 2·3(발표 끝·따라가기 끔 — release)에서 문서 자신의 꼴로 돌아갈지다(itda-work/itda-hyve#362):
  // 슬라이드·보드 문서는 꼴 단추가 숨어 있어 호스트가 다른 꼴로 발표한 뒤 그 꼴에 갇히지 않게, 따라가기가 고른 꼴(chosen)을 거두고
  // 문서의 꼴로 다시 그린다. 그 밖의 문서는 종전처럼 꼴을 그대로 둔다(단추가 보인다).
  function restoreOwnLook(v, chosen, width) {
    if (!lookSwitchHidden(v, "", false) || (chosen !== "slides" && chosen !== "dashboard")) return false;
    return lookFor(v, width, "") !== chosen;
  }

  // afterResize 는 보기 영역 크기가 바뀐 뒤다: 배율을 다시 적용하고(applyZoom), 따라가는 중이면(active — 풀린 화면은 아니다) 새 배율·폭의
  // 자리로 지금 위치에 다시 맞춘다(scrollTo(i, kind, frac) — 슬라이드든 내용 위치든). 배율·폭이 바뀌면 굴린 자리(scrollTop)가 다른 내용을
  // 가리키게 되기 때문이다.
  function afterResize(fol, applyZoom, scrollTo) {
    applyZoom();
    if (fol.active()) scrollTo(fol.index, fol.kind, fol.frac);
  }

  // slideGlider 는 슬라이드 보기에서 발표자를 따라가는 굴림이다(itda-hyve#728 후속). to(viewer, el, delta, move): 슬라이드 el 을 보기 영역의 자리로 옮긴다 — delta 는 지금 굴린
  // 자리에서 el 의 자리까지(요소의 화면 자리로 잰 값이라 굴리는 도중이어도 목표는 늘 요소의 자리다). move(발표자가 옮김 — follower align)이고 지난번에 맞춘 것과 다른 슬라이드 요소이고
  // reduced()(prefers-reduced-motion: reduce)가 아니면 부드럽게(scrollTo behavior "smooth" — 도중에 다음 슬라이드가 오면 새 목표로 다시 굴린다), 아니면 바로 간다(scrollTop 대입 —
  // 첫 맞추기·같은 슬라이드 다시 맞추기·그리기·창 크기·배율 뒤의 재정렬. 굴리는 중이던 부드러운 굴림을 덮는다). 따라가기 해제는 wheel·touchmove·mousedown 이라 굴림의 scroll 이벤트는 풀지 않는다.
  function slideGlider(reduced) {
    let last = null;
    return {
      to(viewer, el, delta, move) {
        const smooth = move && last !== null && el !== last && !reduced();
        last = el;
        const top = viewer.scrollTop + delta;
        if (smooth) viewer.scrollTo({ top, behavior: "smooth" });
        else viewer.scrollTop = top;
      },
    };
  }

  // followLink 는 문서 안 #제목 링크로 이동한다: 대상(el)이 있으면 사람이 직접 움직인 것이므로 먼저 따라가기를 풀고(fol.moved) 굴린다
  // (scroll(el)). 대상이 없으면 아무것도 하지 않고 거짓.
  function followLink(fol, el, scroll) {
    if (!el) return false;
    fol.moved();
    scroll(el);
    return true;
  }

  // hashId 는 #제목 href 의 id 다(퍼센트 꼴을 푼다). 풀 수 없는 href(잘못된 % 꼴)·빈 id 면 "".
  function hashId(href) {
    try { return decodeURIComponent(String(href).slice(1)); } catch (e) { return ""; }
  }

  // hashJump 는 #제목 href 로 이동한다(문서 안 링크·목차 항목이 함께 쓴다): id 를 풀어(find(id) — headingFinder) followLink 로 넘긴다.
  // 풀 수 없는 href 면 대상이 없는 것으로 친다.
  function hashJump(fol, href, find, scroll) {
    const id = hashId(href);
    return followLink(fol, id ? find(id) : null, scroll);
  }

  // headingFinder 는 root(#pages — 그린 본문) 안에서만 id 로 요소를 찾는 함수다. 보기 화면의 UI(sidebar·side-toc·btn-toc·toc-back 등)와
  // 같은 id 의 제목(# Sidebar → id="sidebar")이 있어도 제목을 찾는다 — 전역 getElementById 는 앞에 있는 UI 를 돌려준다. 선택자를 만들지
  // 않고 id 를 그대로 견주므로 특수문자(점·한글·%) id 도 escape 없이 찾는다. 같은 id 가 여럿이면 문서 차례의 첫 것.
  // find.index() 는 id → 요소 Map 이다(강조 계산이 한 번에 쓴다 — ⑦). 본문을 바꿀 때(render 의 교체·오류·공유 끝) find.refresh() 로
  // 한 번 다시 만들고 그 사이의 스크롤 프레임은 같은 Map 을 쓴다(큰 문서에서 프레임마다 [id] 를 훑지 않게). 위치값은 담지 않는다 —
  // 강조 계산이 매번 잰다. 아직 만든 적이 없으면 처음 부를 때 만든다.
  function headingFinder(root) {
    let cache = null;
    const find = id => {
      for (const el of root.querySelectorAll("[id]")) if (el.id === id) return el;
      return null;
    };
    find.refresh = () => {
      const m = new Map();
      for (const el of root.querySelectorAll("[id]")) if (!m.has(el.id)) m.set(el.id, el);
      cache = m;
      return m;
    };
    find.index = () => cache || find.refresh();
    return find;
  }

  // tocJumper 는 #제목 이동과 강조 고정(pin)이다. deps: fol(follower), find(id), scroll(el), closeDrawer(). jump(href, fromToc): hashJump 로
  // 옮기고, 실제로 옮겼으면 목차 항목이든 문서 안 링크든 pin 을 그 제목으로 바꾼다(이전 pin 이 남아 다른 제목을 강조하지 않게). 목차에서
  // 눌렀으면 서랍을 닫는다. resume(): 「발표자 위치로」 — pin 을 거두고(발표자 위치가 기준) 따라가기로 돌아간다. clear(): 사람이 직접
  // 움직임·공유 끝·그 id 가 목차에서 사라짐.
  function tocJumper(deps) {
    const j = { pin: "" };
    j.jump = (href, fromToc) => {
      const went = hashJump(deps.fol, href, deps.find, deps.scroll);
      if (went) j.pin = hashId(href);
      if (fromToc) deps.closeDrawer();
      return went;
    };
    j.resume = () => { j.pin = ""; deps.fol.resume(); };
    j.clear = () => { j.pin = ""; };
    return j;
  }

  // esc 는 HTML 글자·속성 값으로 넣을 글자다.
  function esc(s) {
    return String(s == null ? "" : s).replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);
  }

  // tocHTML 은 view.toc(mdoc.TOCItem{Level, ID, Number, Text, Children} 의 트리)를 목차 목록 HTML 로 만든다 — 문서 창 tocList 와 같은
  // 꼴(ul > li > a[href="#id"][data-id] > span.num·span, 하위는 li 안의 ul). 제목이 없으면 "".
  // open(id → 참·거짓)을 주면 접기 꼴(itda-hyve#985)이다: li 안에 div.toc-row(접기 단추 button.toc-tg 또는 빈 자리 span.toc-tg-pad + a)를
  // 두고, 하위가 있고 id 가 있는 항목만 단추를 단다 — 접힌 하위 ul 은 hidden. 단추는 이동하지 않는다(누르기는 화면이 받는다). open 이 없으면 옛 꼴.
  function tocHTML(items, open) {
    if (!Array.isArray(items) || !items.length) return "";
    let h = "<ul>";
    for (const it of items) {
      const id = String(it.ID || "");
      let a = '<a href="#' + esc(encodeURIComponent(id)) + '" data-id="' + esc(id) + '">';
      if (it.Number) a += '<span class="num">' + esc(it.Number) + "</span>";
      a += "<span>" + esc(it.Text) + "</span></a>";
      if (typeof open !== "function") { h += "<li>" + a + tocHTML(it.Children) + "</li>"; continue; }
      const kids = Array.isArray(it.Children) && it.Children.length > 0, fold = kids && id !== "", o = !fold || !!open(id);
      const sub = kids ? tocHTML(it.Children, open) : "";
      h += '<li><div class="toc-row">' + (fold ? tocToggleHTML(id, o) : '<span class="toc-tg-pad"></span>') + a + "</div>" +
        (o ? sub : sub.replace(/^<ul>/, "<ul hidden>")) + "</li>";
    }
    return h + "</ul>";
  }
  // tocToggleHTML 은 접기 단추 하나다 — 펼쳐 있으면 ▾(누르면 접기), 접혀 있으면 ▸(누르면 펼치기). 문서 창 tocList 와 같은 꼴·이름.
  function tocToggleHTML(id, open) {
    const label = open ? "하위 항목 접기" : "하위 항목 펼치기";
    return '<button type="button" class="toc-tg" data-fold="' + esc(id) + '" aria-expanded="' + open + '" aria-label="' + label + '" title="' + label + '">' + (open ? "▾" : "▸") + "</button>";
  }

  // ---- 목차 접기(itda-hyve#985). 아래 순수 함수와 tocFold 는 문서 창 app.js 의 같은 이름 함수와 규칙이 같다(hyve 시험 toc_fold.cjs 가 둘을 같은
  // 사례로 돌린다). 기억 맵(memo)은 {id: "open"|"closed"} — 사람이 단추로 고른 것만 담는다. 자동으로 펼친 것(auto, Set)은 담지 않는다.

  // tocTree 는 목차 트리의 부모 표(parent: id → 가장 가까운 id 있는 조상, 최상위는 "")와 하위가 있는 항목(branch — Set, order — 문서 차례)이다.
  // id 가 빈 항목은 건너뛴다(그 하위는 그 위 조상에 붙는다). 같은 id 가 다시 나오면 앞의 것만 센다.
  function tocTree(items) {
    const parent = new Map(), order = [];
    const walk = (list, up) => {
      if (!Array.isArray(list)) return;
      for (const it of list) {
        if (!it) continue;
        const id = String(it.ID || ""), kids = Array.isArray(it.Children) && it.Children.length > 0;
        if (id && !parent.has(id)) { parent.set(id, up); if (kids) order.push(id); }
        walk(it.Children, id || up);
      }
    };
    walk(items, "");
    return { parent, branch: new Set(order), order };
  }
  // tocAncestors 는 id 의 조상 id 들이다(가까운 것부터). 목차에 없는 id 면 [].
  function tocAncestors(id, tree) {
    const up = [], seen = new Set([id]);
    let p = tree.parent.get(id);
    while (p && !seen.has(p)) { seen.add(p); up.push(p); p = tree.parent.get(p); }
    return up;
  }
  // tocFoldIsOpen 은 항목이 펼쳐 있는가다 — 기억(memo)이 자동(auto)보다 앞선다.
  function tocFoldIsOpen(id, memo, auto) {
    const m = Object.prototype.hasOwnProperty.call(memo, id) ? memo[id] : "";
    return m === "open" || (m !== "closed" && auto.has(id));
  }
  // tocFoldFollow 는 지금 항목 cur 의 조상 가운데 사람이 접지 않은 것(memo 가 "closed" 가 아님)을 auto 에 더한다. 앞서 펼친 것은 빼지 않는다.
  function tocFoldFollow(cur, tree, memo, auto) {
    for (const a of tocAncestors(cur, tree)) if (!Object.prototype.hasOwnProperty.call(memo, a) || memo[a] !== "closed") auto.add(a);
    return auto;
  }
  // tocFoldShown 은 cur 를 강조할 때 화면에 보이는 항목이다 — 맨 위 조상부터 내려가며 처음 만나는 접힌 조상, 접힌 조상이 없으면 cur 그대로.
  function tocFoldShown(cur, tree, memo, auto) {
    const up = tocAncestors(cur, tree);
    for (let i = up.length - 1; i >= 0; i--) if (!tocFoldIsOpen(up[i], memo, auto)) return up[i];
    return cur;
  }
  // tocFoldBulk 은 「모두 접기·모두 펼치기」 단추가 다음에 할 일이다 — 하위가 있는 항목이 모두 펼쳐 있으면 "collapse", 아니면 "expand",
  // 하위가 있는 항목이 없으면 ""(단추를 숨긴다).
  function tocFoldBulk(tree, memo, auto) {
    if (!tree.order.length) return "";
    return tree.order.every(id => tocFoldIsOpen(id, memo, auto)) ? "collapse" : "expand";
  }
  // TOC_FOLD_DOCS 는 기억하는 문서 수 상한(가장 최근에 쓴 것부터 남긴다), TOC_FOLD_IDS 는 문서 하나의 항목 수 상한, TOC_FOLD_KEY_MAX 는
  // 문서 열쇠 길이 상한이다. 문서 창(prefs.go tocFoldDocs·tocFoldIDs)과 같다.
  const TOC_FOLD_DOCS = 200, TOC_FOLD_IDS = 1000, TOC_FOLD_KEY_MAX = 2048;
  // tocFoldClean 은 기억 맵을 고른다 — 값이 "open"·"closed" 인 것만, tree 를 주면 그 목차의 하위 있는 항목만(지금 목차에 없는 id 는 버린다),
  // 앞에서부터 TOC_FOLD_IDS 개까지. 원형 없는 객체(__proto__ 같은 id 도 그냥 열쇠)를 돌려준다.
  function tocFoldClean(m, tree) {
    const out = Object.create(null);
    if (!m || typeof m !== "object" || Array.isArray(m)) return out;
    let n = 0;
    for (const id of Object.keys(m)) {
      if (n >= TOC_FOLD_IDS) break;
      const v = m[id];
      if ((v === "open" || v === "closed") && (!tree || tree.branch.has(id))) { out[id] = v; n++; }
    }
    return out;
  }
  // tocFoldList 는 저장 글(JSON [[열쇠, 맵], …] — 오래 전에 쓴 것부터)을 읽는다. 깨졌으면 [], 틀린 항목은 버리고 같은 열쇠는 뒤의 것만 둔다.
  function tocFoldList(raw) {
    let a = null;
    try { a = JSON.parse(raw); } catch (e) { a = null; }
    if (!Array.isArray(a)) return [];
    const m = new Map();
    for (const e of a) {
      if (!Array.isArray(e) || typeof e[0] !== "string" || !e[0] || e[0].length > TOC_FOLD_KEY_MAX) continue;
      const memo = tocFoldClean(e[1]);
      m.delete(e[0]);
      if (Object.keys(memo).length) m.set(e[0], memo);
    }
    return [...m].slice(-TOC_FOLD_DOCS);
  }
  // tocFoldGet 은 저장 글에서 열쇠 key 문서의 기억 맵이다(없으면 빈 맵).
  function tocFoldGet(raw, key) {
    const e = tocFoldList(raw).find(x => x[0] === key);
    return e ? e[1] : Object.create(null);
  }
  // tocFoldPut 은 key 문서의 맵을 memo 로 바꾼 저장 글이다 — 그 문서를 맨 뒤(가장 최근)로 옮기고, 빈 맵이면 지운다. cap(기본 TOC_FOLD_DOCS)을
  // 넘으면 오래 전에 쓴 것부터 버린다. 열쇠가 비었거나 너무 길면 그대로 둔다.
  function tocFoldPut(raw, key, memo, cap) {
    const list = tocFoldList(raw);
    if (typeof key !== "string" || !key || key.length > TOC_FOLD_KEY_MAX) return JSON.stringify(list);
    const out = list.filter(e => e[0] !== key), m = tocFoldClean(memo);
    if (Object.keys(m).length) out.push([key, m]);
    return JSON.stringify(out.slice(-(cap > 0 ? cap : TOC_FOLD_DOCS)));
  }
  // tocFoldKey 는 보는 화면의 문서 열쇠다: 주소의 경로(공유 화면은 /s/<토큰>/ — 같은 주소 공유면 다음 공유에도 같다, 묶은 교안은
  // <판>/<문서 id>/ — index.html 은 뗀다) + 폴더 공유의 문서(view.doc — ?doc= 와 같은 꼴). 쿼리·#제목은 넣지 않는다.
  function tocFoldKey(pathname, doc) {
    const p = String(pathname || "/").replace(/index\.html$/, "");
    return doc ? p + "?doc=" + doc : p;
  }
  // TOC_FOLD_STORE 는 공유·게시 화면의 기억 자리(localStorage 열쇠 하나 — 문서마다 [열쇠, 맵])다.
  const TOC_FOLD_STORE = "hs-toc-fold";
  // tocFoldPrefs 는 이 브라우저의 목차 접기 기억이다(localStorage — 읽기·쓰기가 던지면 이 화면의 메모리만, notePrefs 와 같다). 서버에 보내지 않는다.
  // load(key) 는 그 문서의 맵, save(key, memo) 는 지금 저장 글을 다시 읽어(다른 탭이 쓴 것을 지우지 않게) 그 문서만 바꿔 쓴다.
  function tocFoldPrefs(storage) {
    let mem = null; // 쓰기가 막혔을 때의 저장 글
    const get = () => {
      if (mem !== null) return mem;
      try { return storage().getItem(TOC_FOLD_STORE); } catch (e) { return null; }
    };
    return {
      load: key => tocFoldGet(get(), key),
      save: (key, memo) => {
        const raw = tocFoldPut(get(), key, memo);
        try { storage().setItem(TOC_FOLD_STORE, raw); mem = null; } catch (e) { mem = raw; }
      },
    };
  }
  // tocFold 는 문서 하나의 목차 접기 상태다(문서 창 app.js tocFold 와 같다). memo 는 처음 기억 맵, save(memo) 는 사람이 고를 때마다 부른다
  // (지금 목차의 하위 있는 항목만 고른 맵을 넘긴다).
  //  - setTree(items): 새 목차(다시 그림). 기억·자동 펼침은 그대로 둔다 — 없는 id 는 쓰이지 않는다.
  //  - open(id): 펼쳐 있는가. follow(cur): 지금 항목이 바뀌었으면 그 조상을 자동으로 펼친다 — 펼침이 바뀌었으면 참.
  //  - shown(cur): 강조가 보일 항목(tocFoldShown). toggle(id): 단추 하나 — 그 항목을 지금과 반대로 기억. all(open): 모든 하위 있는 항목을 그 값으로.
  //  - bulk(): 일괄 단추가 다음에 할 일(tocFoldBulk).
  function tocFold(memo, save) {
    const f = { tree: tocTree([]), memo: tocFoldClean(memo), auto: new Set(), cur: "" };
    const store = () => { if (typeof save === "function") save(tocFoldClean(f.memo, f.tree)); };
    // 목차가 바뀌면(편집·저장 내용 갱신) 다음 follow 는 같은 id 라도 새 조상을 다시 본다(리뷰 r1 [2] — 부모가 바뀐 지금 항목이 숨지 않게). 사람이 접은 것은 그대로 앞선다
    f.setTree = items => { f.tree = tocTree(items); f.cur = null; };
    f.open = id => tocFoldIsOpen(id, f.memo, f.auto);
    f.follow = cur => {
      if (cur === f.cur) return false;
      f.cur = cur;
      const before = f.auto.size;
      tocFoldFollow(cur, f.tree, f.memo, f.auto);
      return f.auto.size !== before;
    };
    f.shown = cur => tocFoldShown(cur, f.tree, f.memo, f.auto);
    f.toggle = id => {
      if (!f.tree.branch.has(id)) return false;
      const m = tocFoldClean(f.memo);
      m[id] = f.open(id) ? "closed" : "open";
      f.memo = m;
      store();
      return true;
    };
    f.all = open => {
      const m = Object.create(null);
      for (const id of f.tree.order) m[id] = open ? "open" : "closed";
      f.memo = m;
      store();
    };
    f.bulk = () => tocFoldBulk(f.tree, f.memo, f.auto);
    return f;
  }
  // paintTocFold 는 그린 목차(nav)의 접기 단추·하위 ul 을 f 대로 고친다(다시 그리지 않는다 — 누른 단추의 포커스가 남는다).
  function paintTocFold(nav, f) {
    for (const b of nav.querySelectorAll("button.toc-tg")) {
      const o = f.open(b.dataset.fold), label = o ? "하위 항목 접기" : "하위 항목 펼치기";
      b.setAttribute("aria-expanded", String(o));
      b.setAttribute("aria-label", label);
      b.title = label;
      b.textContent = o ? "▾" : "▸";
      const ul = b.parentNode && b.parentNode.nextElementSibling;
      if (ul && ul.tagName === "UL") ul.hidden = !o;
    }
  }
  // TOC_BULK_ICON 은 일괄 단추의 그림이다(Lucide chevrons-down-up·chevrons-up-down — ISC). 글자가 없어 머리 글(side-title)의 textContent 에 섞이지 않는다.
  const TOC_BULK_ICON = {
    collapse: '<svg class="ic" viewBox="0 0 24 24" aria-hidden="true"><path d="m7 20 5-5 5 5M7 4l5 5 5-5"/></svg>',
    expand: '<svg class="ic" viewBox="0 0 24 24" aria-hidden="true"><path d="m7 15 5 5 5-5M7 9l5-5 5 5"/></svg>',
  };
  // tocBulkHead 는 목차 머리(title — .side-title)를 div.toc-head 로 싸고 그 끝에 일괄 단추(button.toc-fold-all)를 단다(이미 쌌으면 그 단추).
  // 머리 글은 그대로 title 에 있다 — 글을 바꾸는 곳(문서 창 썸네일 제목)이 단추를 지우지 않는다. 문서 창 app.js 와 같다.
  function tocBulkHead(doc, title) {
    if (!title || !title.parentNode) return null;
    const had = title.parentNode.classList && title.parentNode.classList.contains("toc-head") ? title.parentNode.querySelector(".toc-fold-all") : null;
    if (had) return had;
    const head = doc.createElement("div"), b = doc.createElement("button");
    head.className = "toc-head";
    b.type = "button";
    b.className = "toc-fold-all";
    b.hidden = true;
    title.parentNode.insertBefore(head, title);
    head.appendChild(title);
    head.appendChild(b);
    return b;
  }
  // paintTocBulk 은 일괄 단추를 f.bulk() 대로 둔다 — "" 면 숨기고, 이름(title·aria-label)은 다음에 할 일.
  function paintTocBulk(b, f, hide) {
    if (!b) return;
    const next = hide ? "" : f.bulk();
    b.hidden = !next;
    if (!next || b.dataset.next === next) return;
    const label = next === "collapse" ? "모두 접기" : "모두 펼치기";
    b.dataset.next = next;
    b.title = label;
    b.setAttribute("aria-label", label);
    b.innerHTML = TOC_BULK_ICON[next];
  }

  // tocIDs 는 목차 트리의 id 를 문서 차례(앞 순회)로 늘어놓는다.
  function tocIDs(items, out) {
    out = out || [];
    if (Array.isArray(items)) for (const it of items) { if (it.ID) out.push(String(it.ID)); tocIDs(it.Children, out); }
    return out;
  }

  // currentHeading 은 강조할 목차 항목의 id 다. positions 는 [{id, top}](문서 차례, top 은 보기 영역 내용 맨 위에서 잰 제목의 위치 px),
  // scrollTop 은 보기 영역을 굴린 양. 보기 영역 맨 위에서 TOC_AHEAD(120px) 아래보다 위에 있는 제목 가운데 마지막 것이다(문서 창
  // updatePageNo 와 같은 계산). 첫 제목도 아직 거기 닿지 않았으면 "".
  // pin·height: 목차 항목을 눌러 간 제목(pin)이 보기 영역(scrollTop ~ scrollTop+height) 안에 있으면 그것 — 문서 끝 가까운 제목은 더
  // 굴릴 수 없어 맨 위에 닿지 못하기 때문이다(화면은 사람이 직접 움직이면 pin 을 거둔다).
  const TOC_AHEAD = 120;
  function currentHeading(positions, scrollTop, pin, height) {
    if (pin) {
      const p = (positions || []).find(x => x.id === pin);
      if (p && p.top >= scrollTop && p.top < scrollTop + (height || 0)) return pin;
    }
    let cur = "";
    for (const p of positions || []) {
      if (p.top < scrollTop + TOC_AHEAD) cur = p.id; else break;
    }
    return cur;
  }

  // tocPanel 은 목차 사이드바의 열림 상태다(메모리에만). width(w): 폭 w 가 TOC_WIDE(1024px)보다 넓은지가 바뀌면(처음 포함) 그 쪽의
  // 기본값 — 넓으면 펼침, 좁으면 접힘 — 으로 되돌린다. 같은 쪽 안이면 그대로 둔다. toggle(): 사람이 단추로 바꿈. close(): 서랍 닫기
  // (바깥을 누름·항목을 누름·Esc). wide 가 거짓이면 사이드바는 서랍이다.
  const TOC_WIDE = 1024;
  function tocPanel() {
    const p = { open: false, wide: null };
    p.width = w => {
      const wide = w > TOC_WIDE;
      if (wide !== p.wide) { p.wide = wide; p.open = wide; }
      return p.open;
    };
    p.toggle = () => { p.open = !p.open; return p.open; };
    p.close = () => { p.open = false; return p.open; };
    return p;
  }

  // tocLayout 은 목차 부품의 모습이다: has(그릴 목차가 있고 공유가 살아 있음)·panel(tocPanel) → 단추·사이드바 숨김, 펼침(open),
  // 서랍인지(drawer), 서랍 바깥 가림막(back — 서랍이 열렸을 때만).
  function tocLayout(has, panel) {
    const open = !!has && !!panel.open;
    return { hidden: !has, open, drawer: !panel.wide, back: open && !panel.wide };
  }

  // ---- 목차 따라 굴리기(itda-hyve#991). 지금 항목이 바뀌면 그 항목이 목차를 굴리는 상자(tocScroller) 안에 보이게 그 상자의 scrollTop 만
  // 바꾼다 — scrollIntoView 는 조상(창·본문 보기 영역)까지 굴릴 수 있어 쓰지 않는다. 문서 창 app.js tocReveal 과 규칙이 같고, 다른 것은
  // 상자 안에 sticky 로 붙은 것(묶은 교안의 저작권 줄 .pack-side-foot — #979)이 가린 곳을 보이는 영역에서 뺀다는 것뿐이다(문서 창에는 없다).
  const TOC_REVEAL_PAD = 24;
  // tocCover 는 상자 rect(box — top·bottom)에서 sticky 로 붙은 것들(rects — 그 사각형들)이 가린 위·아래 높이다 — { top, bottom }.
  // 가운데가 상자 아래쪽 반에 있으면 아래를, 위쪽 반이면 위를 가린 것으로 본다. 상자 밖이거나 높이가 없는 것은 세지 않는다.
  function tocCover(box, rects) {
    const c = { top: 0, bottom: 0 }, mid = (box.top + box.bottom) / 2;
    for (const r of rects || []) {
      if (!r || !(r.bottom > r.top) || r.bottom <= box.top || r.top >= box.bottom) continue;
      if ((r.top + r.bottom) / 2 >= mid) c.bottom = Math.max(c.bottom, box.bottom - Math.max(r.top, box.top));
      else c.top = Math.max(c.top, Math.min(r.bottom, box.bottom) - box.top);
    }
    return c;
  }
  // tocRevealTop 은 항목 rect(item)가 상자 rect(box) 안 보이는 영역 — 가린 높이(cover — tocCover)를 뺀 곳 — 에 여유 pad(기본 24px)를 두고
  // 들게 하는 상자의 새 scrollTop 이다. 이미 들면 st 그대로. 위로 넘치면 위에, 아래로 넘치면 아래에 맞추되 항목이 영역보다 높으면 위에 맞춘다.
  // 0 아래로는 내리지 않는다(위 끝은 브라우저가 scrollHeight 로 자른다).
  function tocRevealTop(item, box, cover, st, pad) {
    const p = pad === undefined ? TOC_REVEAL_PAD : pad, c = cover || { top: 0, bottom: 0 };
    const top = box.top + c.top + p, bottom = box.bottom - c.bottom - p;
    let d = 0;
    if (item.top < top) d = item.top - top;
    else if (item.bottom > bottom) d = Math.min(item.bottom - bottom, item.top - top);
    return d ? Math.max(0, st + d) : st;
  }
  // tocScroller 는 목차(nav — #side-toc)를 굴리는 상자다: 목차 자신이 굴러가는 꼴(묶은 교안의 「첨부」 가 있는 사이드바 — #987, #side-toc 이
  // overflow auto)이면 목차, 아니면 사이드바(side — #sidebar). style(el) 은 계산된 스타일(getComputedStyle)이다.
  function tocScroller(nav, side, style) {
    const o = nav && style ? style(nav).overflowY : "";
    return o === "auto" || o === "scroll" ? nav : side;
  }
  // tocReveal 은 nav 의 강조 항목(접힌 가지 안이면 보이는 조상 a.on-within, 아니면 a.on)이 보이게 굴리는 상자(tocScroller)만 굴린다.
  // 사이드바가 숨었거나(hidden·서랍이 닫힘 — no-toc 로 display none) 항목이 보이지 않으면(getClientRects 가 빔) 아무것도 하지 않는다.
  // 가림은 상자의 자식 가운데 position sticky 인 것만 잰다. win 은 getComputedStyle 을 가진 창이다. 굴렸으면 참.
  function tocReveal(nav, side, win) {
    const on = nav && (nav.querySelector("a.on-within") || nav.querySelector("a.on"));
    if (!on || !side || side.hidden || nav.hidden || !on.getClientRects().length) return false;
    const style = el => win.getComputedStyle(el), box = tocScroller(nav, side, style);
    const sticky = [];
    for (const el of box.children) if (el !== on && style(el).position === "sticky") sticky.push(el.getBoundingClientRect());
    const b = box.getBoundingClientRect(), st = box.scrollTop;
    const next = tocRevealTop(on.getBoundingClientRect(), b, tocCover(b, sticky), st);
    if (next === st) return false;
    box.scrollTop = next;
    return true;
  }
  // tocRevealer 는 언제 굴릴지(tocReveal 을 언제 부를지)를 쥔다. raf(fn)·caf(id) 는 requestAnimationFrame·cancelAnimationFrame 이고,
  // run() 은 굴리기 하나다. 예약은 늘 하나이고 다음 그림 차례에 한 번 돈다 — 본문 scroll·그리기 도중에 다른 상자를 굴리지 않게.
  //  - see(cur): 지금 항목. 앞 항목과 다르고 비지 않았을 때만 예약한다 — 사람이 목차를 굴려 보는 동안 본문이 조금 움직였다고 되돌리지 않는다.
  //  - doc(key): 그리는 문서의 열쇠. 앞 문서와 다르면 앞 항목을 잊는다 — 새 문서의 첫 see 가 같은 id 여도 그 문서 목차로 맞춘다.
  //    같은 문서를 다시 그리면(같은 열쇠) 그대로다.
  //  - open(): 목차를 열었다 — 닫힌 동안 굴리지 못한 것을 한 번 맞춘다.
  //  - cancel(): 사람이 목차 상자를 만졌다(휠·터치·누름·키) — 남은 예약을 버린다. 그 뒤 지금 항목이 다시 바뀌면 또 예약한다.
  function tocRevealer(raf, caf, run) {
    const r = { cur: "", key: undefined, frame: 0 };
    const fire = () => { r.frame = 0; run(); };
    const schedule = () => { if (!r.frame) r.frame = raf(fire) || -1; };
    r.see = cur => { if (cur && cur !== r.cur) schedule(); r.cur = cur; };
    r.doc = key => { if (key !== r.key) { r.key = key; r.cur = ""; } };
    r.open = schedule;
    r.cancel = () => { if (r.frame) { if (r.frame !== -1) caf(r.frame); r.frame = 0; } };
    r.pending = () => r.frame !== 0;
    return r;
  }
  // tocDocKey 는 tocRevealer.doc 에 넘기는, 그리는 문서의 열쇠다: 원천의 foldKey(v)(공유·게시 화면 — 주소 경로 + 폴더 공유의 문서)가 있으면 그것,
  // 없으면 원천의 current()(웹 뷰어 — 문서를 열 때마다 새 객체, 같은 문서를 다시 그리면 같은 객체), 그것도 없으면 v.doc.
  function tocDocKey(source, v) {
    if (source && typeof source.foldKey === "function") return String(source.foldKey(v) || "");
    if (source && typeof source.current === "function") return source.current();
    return v && v.doc !== undefined ? v.doc : undefined;
  }
  // TOC_TOUCH 는 목차 상자에서 사람이 굴리거나 만졌다고 보는 이벤트다(scroll 은 넣지 않는다 — tocReveal 이 바꾼 scrollTop 도 scroll 을 낸다).
  const TOC_TOUCH = ["wheel", "touchstart", "pointerdown", "keydown"];

  // slideZoom 은 보는 화면 슬라이드 보기의 배율이다(⑥): 보기 영역(viewerW×viewerH)에서 여백(gutter — 가로, gutterV — 세로, 없으면 gutter)을
  // 뺀 자리에 슬라이드(slideW×slideH) 한 장이 들도록 너비·높이 가운데 작은 쪽에 맞춘다. 상한 1.6·하한 0.2 — 하한에 걸리는 아주 작은 보기 영역에서는
  // 일부가 잘린다(쪽 보기는 너비만 — applyZoom).
  // 문서 창 발표 모드의 fit(present.js)과 같은 「둘 다 맞춤」 이되 위아래로 이어진 슬라이드를 굴려 보므로 여백을 둔다.
  function slideZoom(viewerW, viewerH, slideW, slideH, gutter, gutterV) {
    const gv = gutterV === undefined ? gutter : gutterV;
    return Math.min(1.6, Math.max(0.2, Math.min((viewerW - gutter) / slideW, (viewerH - gv) / slideH)));
  }

  // slideFit 은 applyZoom 의 슬라이드 맞춤이다 — { z: 배율, gutter: 가로 여백 }. 보기 영역 높이가 SHORT_VIEW_H 미만이면(폰 가로 —
  // 위 막대를 빼면 300px 안팎) 높이까지 맞추면 슬라이드가 가운데에 작게 남으므로 너비만 여백 없이 채운다. 그 밖은 slideZoom(⑥)이다.
  const SHORT_VIEW_H = 480;
  function slideFit(viewerW, viewerH, slideW, slideH, gutter) {
    if (viewerH < SHORT_VIEW_H) return { z: Math.min(1.6, Math.max(0.2, viewerW / slideW)), gutter: 0 };
    return { z: slideZoom(viewerW, viewerH, slideW, slideH, gutter), gutter };
  }

  // notesLayout 은 보는 화면의 노트 배치다: 노트가 없거나 슬라이드 보기가 아니면 "off", 오른쪽을 골랐어도 보기 영역 너비에서 여백(gutter)·
  // 칸(noteW)·사이(gap)를 뺀 슬라이드 자리가 narrow 보다 좁으면 "below"(문서 창 applyZoom 과 같은 규칙).
  function notesLayout(mode, hasNotes, look, viewerW, gutter, noteW, gap, narrow) {
    if (!hasNotes || look !== "slides" || (mode !== "right" && mode !== "below")) return "off";
    return mode === "right" && !notesRightFits(viewerW, gutter, noteW, gap, narrow) ? "below" : mode;
  }
  // notesRightFits 는 오른쪽 배치가 들어가는가다 — 보기 영역 너비에서 여백·칸·사이를 뺀 슬라이드 자리가 narrow 이상이면 참(notesLayout 의 문턱과 같은 식).
  function notesRightFits(viewerW, gutter, noteW, gap, narrow) {
    return viewerW - gutter - noteW - gap >= narrow;
  }
  // notesSwitchState 는 노트 단추의 칠하기다 — { on: 켜진 단추, rightOff: 「오른쪽」 을 비활성으로, reason: 비활성일 때의 title(아니면 "") }.
  // 끔이면 「끔」, 그 밖은 실제 배치(layout — 오른쪽을 골랐어도 좁아 아래로 그렸으면 「아래」)가 켜진다. 배치가 "off"(노트 없음·슬라이드 아님)면 고른 값.
  // 고른 값(mode)은 바꾸지 않는다 — 오른쪽을 골라 둔 경우에만 넓어지면 오른쪽으로 돌아오므로, 그때만 이유에 그 말을 붙인다.
  function notesSwitchState(mode, layout, fits) {
    const on = mode === "off" ? "off" : (layout === "right" || layout === "below" ? layout : mode);
    const reason = fits ? "" : mode === "right" ? "화면이 좁아 노트를 아래에 둔다 — 넓히면 오른쪽으로 돌아온다" : "화면이 좁아 오른쪽에 둘 수 없다";
    return { on, rightOff: !fits, reason };
  }
  // paintNotesButtons 는 노트 모드 단추들(data-notes)에 notesSwitchState 의 st 를 칠한다: 켜진 표시(.on), 「오른쪽」 의 disabled·title(원래 title 은
  // 처음 칠할 때 data-title 에 두고 들어가면 되돌린다).
  function paintNotesButtons(buttons, st) {
    for (const b of buttons) {
      b.classList.toggle("on", b.dataset.notes === st.on);
      if (b.dataset.notes !== "right") continue;
      if (b.dataset.title === undefined) b.dataset.title = b.title;
      b.disabled = st.rightOff;
      b.title = st.rightOff ? st.reason : b.dataset.title;
    }
  }

  // 오른쪽 노트 칸 너비(화면 px — itda-hyve#728 실기 후속): 기본·최소·최대. 호스트 앱 문서 창의 끌기 손잡이와 같은 값이다(app.js NOTE_W_*).
  const NOTE_W_DEF = 320, NOTE_W_MIN = 240, NOTE_W_MAX = 640;
  // noteWidth 는 칸 너비 px 를 [NOTE_W_MIN, NOTE_W_MAX] 로 자르고 반올림한다. 수가 아니면(NaN·무한) 기본 320.
  function noteWidth(px) {
    const n = Number(px);
    return Number.isFinite(n) ? Math.round(Math.min(NOTE_W_MAX, Math.max(NOTE_W_MIN, n))) : NOTE_W_DEF;
  }
  // noteWidthCap 은 끄는 동안 받을 칸 너비의 상한이다 — 보기 영역 너비에서 여백(gutter)·사이(gap)·슬라이드 최소 자리(narrow)를 뺀 값까지만
  // (그보다 넓으면 notesLayout 이 아래로 바꿔 손잡이가 사라진다). [NOTE_W_MIN, NOTE_W_MAX] 안으로 자른다.
  function noteWidthCap(viewerW, gutter, gap, narrow) {
    return Math.max(NOTE_W_MIN, Math.min(NOTE_W_MAX, viewerW - gutter - gap - narrow));
  }
  // noteResizer 는 노트 칸 왼쪽 손잡이 끌기의 계산이다(DOM 없음). o = { width() — 지금 너비, apply(px, save) — 너비를 걸고 save 면 기억,
  // cap() — 지금 상한(없으면 NOTE_W_MAX) }. begin(x) 로 잡고 move(x) 마다 잡은 자리에서 왼쪽으로 간 만큼 넓힌다(상한은 움직일 때마다 cap() 을 다시
  // 묻는다 — 끄는 중 창 폭이 바뀌어도 오른쪽이 유지되는 너비까지만. noteWidth 로 자름 — 기억은 하지 않는다).
  // end() 는 끌기가 끝날 때다 — 너비가 바뀌었으면 한 번 기억한다. reset() 은 두 번 누름 — 기본 320 으로 걸고 기억한다. 잡지 않았으면 move·end 는 아무것도 하지 않는다.
  function noteResizer(o) {
    let d = null;
    const cap = () => (o.cap ? o.cap() : NOTE_W_MAX);
    return {
      begin(x) { d = { x0: x, w0: o.width() }; },
      dragging() { return d !== null; },
      move(x) {
        if (!d) return;
        const w = noteWidth(Math.min(cap(), d.w0 + (d.x0 - x)));
        if (w !== o.width()) o.apply(w, false);
      },
      end() {
        if (!d) return;
        const w0 = d.w0;
        d = null;
        if (o.width() !== w0) o.apply(o.width(), true);
      },
      reset() { d = null; o.apply(NOTE_W_DEF, true); },
    };
  }

  // noteGripDrag 는 노트 칸 손잡이(.sn-grip) 끌기의 이벤트 수명이다(DOM 은 넘겨받는다 — 시험이 가짜로 돌린다). o = { notes(pointerdown·dblclick 을
  // 받는 칸 상자), doc(document — 움직임·놓음·캡처 상실·숨김), win(window — blur), body(끄는 동안 dragging-note 를 단다), resizer(noteResizer),
  // canGrip()(지금 오른쪽 배치인가), onStart()(잡을 때·두 번 누를 때 — 보던 자리를 기억) }.
  // 끌기는 놓기(pointerup)·취소(pointercancel)·캡처 상실(lostpointercapture — 손잡이가 빠지면 document 로 온다)·버튼 없는 마우스 움직임(놓음을 놓침)·
  // 창 blur·탭 숨김·check()(붙든 손잡이가 문서에서 빠졌거나 오른쪽 배치가 아님) 가운데 무엇으로든 finish 한 곳에서 끝난다 — 리스너·클래스·캡처를 거두고,
  // 바뀐 너비가 있으면 놓기와 같게 한 번 기억한다(resizer.end). 두 번 불러도 된다. 돌려주는 것 = { finish, check, dragging }.
  function noteGripDrag(o) {
    let pid = null, held = null;
    const gripOf = e => (e.target && e.target.closest ? e.target.closest(".sn-grip") : null);
    const mine = e => pid !== null && e.pointerId === pid;
    const onMove = e => {
      if (!mine(e)) return;
      if (e.pointerType === "mouse" && e.buttons === 0) { finish(); return; } // 창 밖에서 놓아 pointerup 을 놓쳤다
      o.resizer.move(e.clientX);
    };
    const onEnd = e => { if (mine(e)) finish(); };
    const onVis = () => { if (o.doc.visibilityState === "hidden") finish(); };
    const onBlur = () => finish();
    const DOC = [["pointermove", onMove], ["pointerup", onEnd], ["pointercancel", onEnd], ["lostpointercapture", onEnd], ["visibilitychange", onVis]];
    function finish() {
      if (pid === null) return;
      const id = pid, g = held;
      pid = null; held = null; // 먼저 비운다 — 아래 캡처 해제가 부르는 lostpointercapture·기억이 부르는 check 가 다시 들어와도 아무것도 하지 않게
      for (const [t, f] of DOC) o.doc.removeEventListener(t, f);
      o.win.removeEventListener("blur", onBlur);
      o.body.classList.remove("dragging-note");
      try { if (g && g.hasPointerCapture && g.hasPointerCapture(id)) g.releasePointerCapture(id); } catch (err) { /* 이미 풀렸다 */ }
      o.resizer.end();
    }
    // 펜·터치도 받는다(pointer). 잡은 손잡이에 포인터를 붙들어(setPointerCapture) YouTube 틀 위로 지나가도 움직임이 오게 하고, 움직임·놓음은 document 에서 받는다.
    o.notes.addEventListener("pointerdown", e => {
      const g = gripOf(e);
      if (!g || pid !== null || (e.pointerType === "mouse" && e.button !== 0) || !o.canGrip()) return;
      e.preventDefault();
      pid = e.pointerId; held = g;
      o.onStart();
      o.resizer.begin(e.clientX);
      try { g.setPointerCapture(pid); } catch (err) { /* 붙들 수 없으면 document 로만 */ }
      o.body.classList.add("dragging-note");
      for (const [t, f] of DOC) o.doc.addEventListener(t, f);
      o.win.addEventListener("blur", onBlur);
    });
    o.notes.addEventListener("dblclick", e => {
      if (!gripOf(e) || !o.canGrip()) return;
      e.preventDefault();
      finish();
      o.onStart();
      o.resizer.reset();
    });
    return {
      finish,
      check() { if (pid !== null && (!held || held.isConnected === false || !o.canGrip())) finish(); },
      dragging: () => pid !== null,
    };
  }

  // slideAnchor 는 보기 영역 세로 가운데 mid 에 걸린(없으면 가장 가까운) 슬라이드 번호 i 와 그 슬라이드 안의 자리 frac(위 0 · 아래 1 — 사이에 걸리면
  // 0 미만·1 초과)이다. rs 는 슬라이드들의 화면 사각형({ top, bottom, height }). 슬라이드가 없으면 null.
  function slideAnchor(rs, mid) {
    let best = -1, bd = Infinity;
    rs.forEach((r, i) => { const d = r.top > mid ? r.top - mid : r.bottom < mid ? mid - r.bottom : 0; if (d < bd) { bd = d; best = i; } });
    if (best < 0) return null;
    const r = rs[best];
    return { i: best, frac: r.height > 0 ? (mid - r.top) / r.height : 0 };
  }
  // anchorDelta 는 슬라이드 사각형 r 의 frac 자리를 다시 mid 에 두기까지 굴릴 양(px)이다 — 배율이 바뀌어 슬라이드 크기가 달라져도 슬라이드 안의 같은 자리.
  function anchorDelta(r, frac, mid) { return r.top + frac * r.height - mid; }

  // notePrefs 는 보는 사람의 노트 배치·글자 크기·오른쪽 칸 너비 기억이다(이 브라우저의 localStorage — 읽기·쓰기가 던지면 기억 없이 메모리만).
  // 배치는 "off"·"right"·"below"(그 밖·없음이면 "below"), 글자는 NOTE_FONTS 의 px(그 밖·없음이면 0 — 기본 14), 너비는
  // [NOTE_W_MIN, NOTE_W_MAX] 안의 px(그 밖·수가 아님·없음이면 320). 호스트에 보내지 않는다.
  function notePrefs(storage, fonts) {
    const get = k => { try { return storage().getItem(k); } catch (e) { return null; } };
    const set = (k, v) => { try { storage().setItem(k, v); } catch (e) { /* 메모리만 */ } };
    const m = get("hs-notes");
    const f = Number(get("hs-note-font"));
    const ws = get("hs-note-w"), w = ws === null || ws.trim() === "" ? NaN : Number(ws);
    const p = { mode: ["off", "right", "below"].includes(m) ? m : "below", font: fonts.includes(f) ? f : 0,
      width: Number.isFinite(w) && w >= NOTE_W_MIN && w <= NOTE_W_MAX ? Math.round(w) : NOTE_W_DEF };
    p.setMode = v => { p.mode = v; set("hs-notes", v); };
    p.setFont = v => { p.font = v; set("hs-note-font", String(v)); };
    p.setWidth = v => { p.width = noteWidth(v); set("hs-note-w", String(p.width)); };
    return p;
  }

  // followSpot 은 따라가는 슬라이드를 둘 자리다: 노트가 아래에 붙어 보이면 "top"(슬라이드를 보기 영역 위에 — 아래 노트까지 보이게),
  // 아니면 "center".
  function followSpot(layout, noteShown) { return layout === "below" && noteShown ? "top" : "center"; }

  // keyHandler 는 보는 화면의 keydown 이다(⑨). deps: pager(ItdaPager — navKey·typingTarget)·fol·jumper·closeDrawer()·ended()·turn(dir).
  // Esc 는 목차 서랍을 닫는다(전과 같다). 입력란에 포커스가 있으면 나머지는 건드리지 않는다. 쪽 넘김 키(navKey — ←/→·PageUp/PageDown·
  // Space/Shift+Space·Home/End)면 브라우저 굴림을 막고 ③ 규칙대로 이 화면의 따라가기를 먼저 풀고(jumper.clear·fol.moved) turn(dir) 로
  // 넘긴다(공유가 끝난 화면은 넘기지 않는다). 단추·링크에 포커스가 있을 때의 Space 는 그것을 누르는 키라 넘기지도, 따라가기를 풀지도
  // 않는다(코드 복사 단추 itda-hyve#413 등 — navKey 와 같은 제외 셀렉터). 그 밖의 넘기는 키(↑↓, ⌘ 를 곁든 화살표 등 — 브라우저가 굴린다)는 풀기만 한다.
  function keyHandler(deps) {
    const onBtn = el => !!(el && el.closest && el.closest("button, a[href], summary"));
    return e => {
      if (e.key === "Escape") { deps.closeDrawer(); return; }
      if (deps.pager.typingTarget(e.target)) return;
      const dir = deps.ended() || e.defaultPrevented || e.isComposing ? "" : deps.pager.navKey(e, true);
      if (dir) { e.preventDefault(); deps.jumper.clear(); deps.fol.moved(); deps.turn(dir); return; }
      if (e.key === " " && onBtn(e.target)) return;
      if (/^(Arrow|Page|Home|End| $)/.test(e.key)) { deps.jumper.clear(); deps.fol.moved(); }
    };
  }

  // notesPop 은 머리 막대 「노트」 단추(#notes-btn)와 그 아래 판(#notes-pop — 위치·글자 크기, itda-hyve#1040)의 여닫기다. o = { doc(사건을 받는 문서),
  // btn, pop } — DOM 은 넘겨받는다(시험이 가짜로 돌린다). { isOpen, set(on, focus) } 를 돌려주고 처음에는 닫는다.
  //  - 상태는 판의 hidden 과 단추의 aria-expanded 가 늘 같이 말한다(set). 단추의 .on 은 여기서 건드리지 않는다 — 노트가 켜졌음의 표시다(paintNotesSwitch).
  //  - 단추 누름은 여닫기다. 판 안의 위치·글자 단추를 눌러도 닫지 않는다(여러 번 누르는 컨트롤이다).
  //  - 열렸을 때만: Esc 는 닫고 단추로 초점을 돌린다(캡처 단계에서 받아 문서로 내려보내지 않는다 — 보기 화면의 Esc 처리까지 가지 않게),
  //    판·단추 밖 누름(pointerdown — 캡처, 막지 않는다. 호스트의 다른 메뉴도 같은 사건으로 닫힌다)과 초점이 판·단추 밖의 요소로 옮겨감(relatedTarget 이 있을 때 —
  //    창 밖으로 나가 상대가 없으면 그대로)은 닫는다. 발표로 들어가는 키(F5 · ⌘/Ctrl+Shift+Enter)는 닫기만 하고 막지 않는다.
  function notesPop(o) {
    const { doc, btn, pop } = o;
    let open = false;
    const inside = t => !!t && (btn.contains(t) || pop.contains(t));
    function set(on, focus) {
      open = !!on;
      pop.hidden = !open;
      btn.setAttribute("aria-expanded", String(open));
      if (!open && focus && btn.focus) btn.focus({ preventScroll: true });
    }
    set(false);
    btn.addEventListener("click", () => set(!open));
    doc.addEventListener("pointerdown", e => { if (open && !inside(e && e.target)) set(false); }, true);
    doc.addEventListener("keydown", e => {
      if (!open || !e || e.isComposing) return;
      // 발표로 들어가는 키(F5 · ⌘/Ctrl+Shift+Enter — 호스트가 그 키로 발표를 연다)면 판을 닫고 키는 그대로 흘려보낸다 — 발표 뒤에 열린 판이 남지 않는다
      if (e.key === "F5" || (e.key === "Enter" && e.shiftKey && (e.metaKey || e.ctrlKey))) { set(false); return; }
      if (e.key !== "Escape") return;
      e.stopPropagation();
      e.preventDefault();
      set(false, true);
    }, true);
    const blur = e => { if (open && e && e.relatedTarget && !inside(e.relatedTarget)) set(false); };
    pop.addEventListener("focusout", blur);
    btn.addEventListener("focusout", blur);
    return { isOpen: () => open, set };
  }

  // tableMenu 는 표 복사 메뉴(#table-ctx — itda-hyve#349)의 열고 닫기와 메뉴 안의 키다(itda-hyve#376). deps: menu(메뉴 요소)·active()(document.activeElement)·
  // view()({w, h} 창 크기)·now()(ms)·onClose()(닫힐 때 — 고른 표를 버린다).
  //  - open(x, y, how): (x, y)에 두고(창 밖으로 나가지 않게) 그때의 포커스를 돌아갈 자리로 쥔 뒤 첫 항목에 포커스를 준다(preventScroll —
  //    메뉴는 fixed 라 굴리지 않는다). how 가 "key"(Shift+F10·ContextMenu 키)면 그 시각을 적는다 — fromKey().
  //    마우스로 열어도 포커스를 옮긴다(문서 창 미리보기 메뉴는 마우스로 열면 옮기지 않는다 — 여기는 ↑↓ 가 메뉴 밖이면 보기 영역을 굴려
  //    메뉴가 닫히므로 옮긴다). 마우스로 열면 메뉴에 class by-mouse 를 달아 포커스 테두리를 숨기고(share.css — Chrome 은 오른쪽 클릭 뒤
  //    옮긴 포커스에도 :focus-visible 을 그린다) 메뉴 안에서 키를 누르면 뗀다.
  //  - close(): 열려 있으면 숨기고 onClose. 포커스가 메뉴 안이었으면 쥔 자리로 돌려준다(preventScroll — 따라가기 중 굴림이 바뀌지 않게).
  //    쥔 자리가 없거나 본문을 다시 그려 떨어졌으면(isConnected 아님) 메뉴 안 포커스만 놓는다(blur).
  //  - key(e): 메뉴 안의 keydown. ↑↓ 는 항목 사이를 돌고(끝에서 처음으로), Home/End 는 처음·끝, Esc 는 닫고 포커스를 돌려준다, Tab 은
  //    닫고 포커스를 돌려준 뒤 기본 동작(돌려준 자리에서 다음·앞으로)을 둔다, Enter·Space 는 단추 그대로. 모든 키는 문서로 올려보내지
  //    않는다(stopPropagation) — 보는 화면의 keydown(keyHandler)이 ↑↓·Home/End·Space 로 따라가기를 풀거나 쪽을 넘기지 않게.
  //  - fromKey(): 방금(500ms 안) 키로 열었고 아직 열려 있음 — 같은 키로 브라우저가 이어 보내는 contextmenu 로 두 번 열지 않게.
  function tableMenu(deps) {
    const m = deps.menu;
    let back = null, keyAt = -Infinity;
    const items = () => [...m.querySelectorAll("[role=menuitem]")].filter(b => !b.disabled && !b.hidden);
    const focus = el => el.focus({ preventScroll: true });
    const t = {};
    t.isOpen = () => !m.hidden;
    t.open = (x, y, how) => {
      const a = deps.active();
      back = a && !m.contains(a) ? a : back;
      keyAt = how === "key" ? deps.now() : -Infinity;
      m.classList.toggle("by-mouse", how !== "key");
      m.hidden = false;
      const v = deps.view();
      m.style.left = Math.max(4, Math.min(x, v.w - m.offsetWidth - 8)) + "px";
      m.style.top = Math.max(4, Math.min(y, v.h - m.offsetHeight - 8)) + "px";
      const b = items()[0];
      if (b) focus(b);
    };
    t.close = () => {
      if (m.hidden) return;
      const a = deps.active(), inside = !!a && m.contains(a);
      m.hidden = true;
      deps.onClose();
      const to = back;
      back = null;
      if (!inside) return;
      if (to && to.isConnected && to.focus) focus(to);
      else a.blur();
    };
    t.fromKey = () => !m.hidden && deps.now() - keyAt < 500;
    t.key = e => {
      if (m.hidden) return;
      e.stopPropagation();
      m.classList.remove("by-mouse");
      if (e.isComposing || e.metaKey || e.ctrlKey || e.altKey) return;
      const list = items();
      if (!list.length) return;
      const i = list.indexOf(deps.active());
      let n = -1;
      if (e.key === "ArrowDown") n = i < 0 ? 0 : (i + 1) % list.length;
      else if (e.key === "ArrowUp") n = i < 0 ? list.length - 1 : (i + list.length - 1) % list.length;
      else if (e.key === "Home") n = 0;
      else if (e.key === "End") n = list.length - 1;
      else if (e.key === "Escape") { e.preventDefault(); t.close(); return; }
      else if (e.key === "Tab") { t.close(); return; }
      if (n < 0) return;
      e.preventDefault();
      focus(list[n]);
    };
    return t;
  }

  // tableCopyClick 은 표 메뉴 항목(복사 단추)의 click 이다(itda-hyve#349·itda-hyve#376). deps: tables()(고른 표 조각 — 닫으면 버린다)·close()(메뉴 닫기 —
  // 포커스가 메뉴 안이면 연 자리로 돌려준다)·copy(kind, tables)(ItdaTableCopy.copy — http 면 이 안에서 바로 execCopy 가 돌고, 그 임시
  // textarea 가 가져간 포커스는 tablecopy.js 가 돌려준다)·toast(글). 닫기를 먼저 한다 — 복사가 먼저면 포커스가 textarea 로 가 있어
  // close 가 「메뉴 안」을 못 알아본다. 복사가 끝난 뒤(비동기)에는 포커스를 건드리지 않는다 — 그 사이 사용자가 옮긴 자리를 빼앗지 않게.
  function tableCopyClick(deps) {
    return kind => {
      const tables = deps.tables();
      deps.close();
      if (!tables) return Promise.resolve();
      let p;
      try { p = deps.copy(kind, tables); } catch (e) { p = Promise.reject(e); }
      return p.then(deps.toast, e => deps.toast("표를 복사하지 못했습니다 — " + String(e && e.message || e)));
    };
  }

  // appName 은 <meta name="application-name"> 의 값에서 고른 앱 이름이다 — 비었거나 서버가 채우지 않은 자리 글자({{…}})면 "".
  function appName(raw) {
    const s = String(raw == null ? "" : raw).trim();
    return /^\{\{.*\}\}$/.test(s) ? "" : s;
  }

  // docTitle 은 탭 제목이다 — fixed(원천의 tabTitle)가 있으면 늘 그 글(뷰어 — 문서 제목·파일 이름을 탭 제목·브라우저 기록에 넣지 않는다),
  // 아니면 앱 이름이 있으면 「<name> — <app>」, 없으면 name 그대로.
  function docTitle(name, app, fixed) { if (fixed) return String(fixed); return app ? name + " — " + app : name; }
  // pick 은 호스트가 둔 전역 하나다 — 정식 이름 w["Itda" + name] 을 먼저, 없으면 옛 이름 w["Hyve" + name](호환용 별칭)을 본다. 둘 다 없으면 undefined.
  function pick(w, name) { return w["Itda" + name] || w["Hyve" + name]; }

  // hostSource 는 공유 화면의 View 원천이다 — 호스트(공유 서버)와 이어진 것 전부: <base>/view.json 받기(syncer)·갱신 스트림(<base>/events —
  // connect, hello·slide·pos 는 화면의 follower 로)·테마 CSS(<base>/_/_theme/… 를 <link id="theme-css"> 에 — themeApplier)·다시 불러오기
  // (reloadGate)·끝남(stop·kick·404·403). w 는 window(시험은 가짜 — EventSource·fetch·sessionStorage·location·setTimeout·clearTimeout·
  // addEventListener·document).
  // 원천의 꼴(뷰어의 원천도 같다): { base, linkMsg?, attach(api) → { applyTheme(v), start(), stop() } }.
  //  - base: 확장(ItdaShareExt)에 넘기는 base. linkMsg: 열지 않는 문서 안 파일 링크의 까닭 — 누를 때의 알림이자 꺼진 링크의 title(없으면 LINK_OFF_MSG).
  //    liveLink(a): 있으면 이 원천이 살려 두는 링크인가(참이면 화면이 끄지 않고 onClick 도 건드리지 않는다 — 브라우저 기본 동작, itda-hyve#1096). 묶은 교안(hyve
  //    packview.js)이 첨부·교안 링크에 참을 준다. hostSource·뷰어는 두지 않는다.
  //    tabTitle: 있으면 탭 제목을 늘 이 글로 둔다(docTitle — 뷰어). 공유 화면(hostSource)은 두지 않는다 — 문서 제목 그대로.
  //    foldKey(v): 있으면 view v 의 목차 접기 기억 열쇠(itda-hyve#985 — 문서마다 다른 글, ""·없음이면 기억하지 않고 이 화면 메모리만). hostSource 는
  //    주소 경로 + 폴더 공유의 문서(tocFoldKey), 묶은 교안(hyve packview.js)은 문서 주소. 뷰어는 두지 않는다(파일마다 주소가 같다).
  //  - attach(api) 는 화면이 시작할 때 한 번(첫 그리기 전) 부른다. api 는 화면 부품이다: $(id), ended()(공유가 끝남), current()(지금
  //    view 또는 null), show(v)(view 를 바꾸고 그린다 — Promise), redraw()(끝나지 않았고 view 가 있으면 다시 그림), look(name)(보기 꼴을
  //    골라 다시 그림 — Promise), failed(msg)(안내 줄), themeFailed()(「테마를 불러오지 못했습니다」), end(reason)(끝난 화면),
  //    paused(st, retry?)(같은 주소 공유의 잠시 닫힘 띠 — "closed"·"stale"·"held"·"". retry 는 "closed"·"stale" 에만 온다 — 바로 다시 붙어 보는 함수(connect 의 retryNow, Promise)), holdReload()(새로 고치면 잃을 것이 있으면 참 — 확장이 답한다),
  //    hello(d)·slide(d)·pos(d)(발표자 따라가기 — follower), toast(msg), files(d)(폴더의 문서 목록 {docs, rev, truncated})·
  //    nav(n)(폴더 공유의 주소 쪽 일 { look(), setLook(look), hash(), open(doc, hash) } — 첫 show 전에 한 번). 뒤의 둘은 hostSource 만 부른다.
  //  - applyTheme(v): render 가 그리기 전에 부른다(오류 view 는 부르지 않는다). true(또는 Promise<true>)면 그리고, false 면 그리지 않는다.
  //  - start(): 화면 준비가 끝난 뒤 한 번 — 첫 view 를 받아 show 한다. stop(): 끝난 화면(showEnded)이 부른다 — 타이머·요청을 거둔다.
  function hostSource(base, w) {
    return {
      base,
      foldKey: v => tocFoldKey(w.location.pathname, v && v.doc),
      attach(api) {
        const gate = reloadGate(base, {
          storage: () => w.sessionStorage,
          now: () => Date.now(),
          reload: () => w.location.reload(),
          ended: () => api.ended(),
          setTimeout: (f, ms) => w.setTimeout(f, ms),
          clearTimeout: t => w.clearTimeout(t),
          failed: () => api.themeFailed(),
          // 자동 새로 고침의 한 관문(#705): 쿠키 유실·테마 변경·reload·CSS 실패 모두 여기를 지난다. 같은 주소 공유만 묻는다 — 그 밖의 공유는 지금처럼 새로 고친다
          hold: () => {
            if (!conn || !conn.keep || !api.holdReload()) return false;
            api.paused("held");
            return true;
          },
        });
        const applyTheme = themeApplier(api.$("theme-css"), base, {
          setTimer: (f, ms) => w.setTimeout(f, ms),
          late: () => api.redraw(),
          reload: why => gate.once(why), // CSS error 만
          changed: () => gate.server(),
        });
        // fetchView 는 view.json 의 {status, view, missing} 이다. view 는 200 이거나 문서 부재(docMissing — 서버가 오류 view 를 담아 준다)일 때만 있다.
        async function fetchView() {
          const r = await w.fetch(base + "/view.json", { cache: "no-store", credentials: "same-origin" });
          const missing = docMissing(r);
          return { status: r.status, missing, view: r.ok || missing ? await r.json() : null };
        }
        // 폴더 공유: view 에 doc 이 올 때만 목록을 받는다 — 그 밖의 공유는 files.json 요청 자체가 없다
        let filesOn = false, filesRev;
        const loadFiles = () => w.fetch(base + "/files.json", { cache: "no-store", credentials: "same-origin" })
          .then(r => r.ok ? r.json() : null)
          .then(d => { if (d && !api.ended()) { filesRev = d.rev; api.files(d); } }, () => { /* 다음 알림·다시 붙기에서 다시 받는다 */ });
        let conn = null;
        // sync 는 view.json 다시 받기다(syncer — 처음·hello·version·다시 붙음). 공유가 끝난 뒤에는 요청하지 않는다.
        const sync = syncer({
          fetch: fetchView,
          current: () => api.current(),
          pageKey: () => applyTheme.key(),
          apply: v => {
            if (v.doc && !filesOn) {
              filesOn = true;
              // 고른 보기 꼴은 이 탭의 sessionStorage 로 문서를 옮겨도 잇는다(경로에 토큰이 있어 공유마다 따로)
              const lookKey = "hs-look:" + w.location.pathname;
              api.nav({
                look() { try { return w.sessionStorage.getItem(lookKey) || ""; } catch (e) { return ""; } },
                setLook(look) { try { w.sessionStorage.setItem(lookKey, look); } catch (e) { /* 이 화면에서만 */ } },
                hash: () => w.location.hash,
                open: (doc, hash) => w.location.assign("?doc=" + encodeURIComponent(doc) + hash),
              });
              loadFiles();
            }
            return api.show(v);
          },
          gate,
          ended: () => api.ended(),
          end: reason => conn.gone(reason),
          failed: () => api.failed("문서를 받지 못했습니다 — 네트워크를 확인하세요."),
          setTimeout: (f, ms) => w.setTimeout(f, ms),
          clearTimeout: t => w.clearTimeout(t),
        });
        // pageStatus 는 지금 주소(폴더 공유는 ?doc= 까지)의 상태 코드다 — 쓰기 없는 GET, 망 실패는 0.
        const pageStatus = () => w.fetch(w.location.href, { cache: "no-store", credentials: "same-origin" }).then(r => r.status, () => 0);
        // resync 는 연결 복구의 한 절차다 — 스트림이 끊길 때마다(문서 없음이면 backoff 재시도마다) 지금 문서의 view 와 폴더면 목록을 다시 받아 맞추고 상태를 돌려준다. 받은 view 가
        // 곧 생존 판정이라 따로 묻지 않는다. 폴더 공유가 아니면 filesOn 이 거짓이라 목록 요청이 없다. 잠시 닫힘의 view 404 만 지금 주소를 함께 살핀다: 세션이 없어도 토큰이 살아 있으면
        // 입장 화면이 200 이므로, 200 은 공유는 열렸는데 이 브라우저의 쿠키만 모른다는 뜻이다 — 새로 고쳐 입장 화면이나 문서를 띄운다. 공유가 없는 동안은 같은 404 라 재시도이고 403 은 끝이다.
        const resync = async () => {
          const st = await sync.refresh(false, true);
          if (st === "ended" && conn.paused) {
            const code = await pageStatus();
            if (code === 403) return "kicked";
            if (code === 200) gate.server();
            return st;
          }
          if (filesOn && (st === "alive" || st === "missing")) await loadFiles();
          return st;
        };
        return {
          applyTheme,
          start() {
            conn = connect(base, {
              EventSource: w.EventSource,
              probe: resync,
              setTimeout: (f, ms) => w.setTimeout(f, ms),
              clearTimeout: t => w.clearTimeout(t),
              // 처음이든 다시 붙었든 hello 의 판·테마가 제 view 와 다르면 다시 받는다 — 첫 구독 전·끊긴 사이의 저장을 따라잡는다
              onHello: (first, data) => {
                api.hello(data);
                sync.check(data);
                if (filesOn && data.files_rev !== undefined && data.files_rev !== filesRev) loadFiles(); // 목록을 받은 뒤·복구의 목록 요청이 실패한 뒤의 변경
              },
              onFiles: () => { if (filesOn) loadFiles(); },
              onReload: () => gate.server(),
              onVersion: data => sync.check(data),
              onSlide: data => api.slide(data),
              onPos: data => api.pos(data),
              onEnded: reason => api.end(reason),
              onPaused: st => api.paused(st, st ? () => conn.retryNow() : undefined), // 띠의 「지금 다시 잇기」
            });
            // 폰 Safari 는 뒤로 갔다 오면(bfcache) 스트림이 끊겨 있다 — 보이면 바로 다시 붙는다(잠시 닫힘의 기다림도 지운다)
            w.addEventListener("pageshow", () => conn.wake());
            w.document.addEventListener("visibilitychange", () => { if (w.document.visibilityState === "visible") conn.wake(); });
            // 망이 돌아오면 잠시 닫힘은 기다리지 않고 한 번 붙어 본다(멈춤 — stale — 은 그대로, 잠시 닫힘이 아닌 끊김은 backoff 그대로)
            w.addEventListener("online", () => { if (conn.paused) conn.wake(); });
            sync.refresh(true);
          },
          stop() { gate.cancel(); sync.cancel(); },
        };
      },
    };
  }

  // pickSource 는 화면의 View 원천이다: body 에 data-base 가 있으면 공유 화면(hostSource — 다른 원천으로 바꿀 수 없다), 없으면
  // w.ItdaViewSource(페이지가 share.js 앞에서 둔 것 — 뷰어), 그것도 없으면 null(화면을 시작하지 않는다).
  function pickSource(w, body) {
    const base = body && body.dataset ? body.dataset.base : "";
    if (base) return hostSource(base, w);
    const s = w.ItdaViewSource;
    return s && typeof s.attach === "function" ? s : null;
  }

  // ---- 그림 설명·크게 보기(itda-doc#121) — 쪽·이어보기·슬라이드 보기(보드·전체 화면 발표·청중은 아니다)의 쪽 콘텐츠 영역(#pages 안 .page-body 와
  // 슬라이드 분할 패널 .page > .slide-panel·배경 .page > .slide-bg — 넘쳐 생긴 장의 사본 포함, figArea) 그림.
  // 배경 그림은 머리·본문 밑(base.css z-index 0)이라 누르기는 그 위를 덮는 블록이 없는 자리에서만 그림에 닿는다 — 본문 글자·빈 본문 자리를 눌러도 열리지 않는다.
  // 키보드로는 다른 그림과 같이 Tab(tabindex 0)으로 닿아 Enter·Space 로 연다.
  // 대상은 렌더된 img 그대로다(배치 낱말 판정을 다시 하지 않는다 — mdoc 이 배치 낱말뿐인 alt 를 빈 alt 로 낸다). 로고(.sc-logo·.sf-logo)·숨은 단계(.pr-pending)는
  // 빼고, 측정 무대(#stage)·노트 칸(#notes)은 범위(#pages) 밖이라 빠진다. 그림의 DOM(부모·형제·class·크기)은 바꾸지 않는다 — 본문을 다 그린 뒤
  // tabindex 와 data-fig 만 단다(figTips arm).
  //  - 설명 툴팁: alt 가 비지 않은(공백뿐 아님) 그림에 마우스·펜을 올리거나(터치는 아니다 — 탭은 크게 보기) 키보드 포커스가 닿으면(링크 그림은 그 링크의 포커스)
  //    body 직계의 .fig-tip(aria-hidden — alt 를 다시 낭독하지 않게, textContent 로만)에 보인다. 쪽 배율(transform)과 상관없는 글자 크기, 자리는 그림 아래 →
  //    위 → 겹침(figTipPlace — 늘 창 안). 높이는 창 높이 - 16px 와 FIG_TIP_MAX_H(약 12줄) 중 작은 쪽까지이고, 넘치면 끝을 「…」 로 줄인다(figClip).
  //    전체 설명은 링크 밖 그림이면 크게 보기 캡션으로 읽는다(링크 그림의 긴 설명 전체를 화면에 보이는 길은 없다 — alt 는 화면 읽기 프로그램에 그대로 간다).
  //    포인터·포커스가 떠나거나 Esc·굴림·창 크기 바뀜·다시 그리기에 닫는다. Esc 로 닫은 그림은 떠났다 올 때까지 다시 열지 않는다.
  //  - 크게 보기: 링크(a) 안이 아닌 그림을 누르면(클릭·탭) 또는 포커스한 그림에서 Enter·Space 면 호스트의 크게 보기 부품(window.ItdaLightbox — 없으면
  //    크게 보기 없이 툴팁만, 누르기는 지금처럼 아무 일도 없다)이 body 에 열고 설명을 아래 캡션으로 보인다. 같은 img 의 얕은 복제(같은 src)다.
  //    닫기는 Esc·바탕 누르기·닫기 단추(.fig-lb-close), 닫으면 원래 그림으로 포커스를 돌린다. 열린 동안 키는 창(window) 붙잡기 단계에서 이것이 먼저
  //    받는다 — 쪽 넘김(Space 등)·목차 서랍 Esc·발표 시작 키로 새지 않는다. 링크·YouTube 대표 그림의 누르기는 지금처럼 링크 동작이다.
  // 인쇄에는 툴팁·크게 보기가 찍히지 않는다(share.css·호스트 app.css 의 @media print).

  const FIG_SKIP = ".sc-logo, .sf-logo, .pr-pending";
  const FIG_TIP_MAX_H = 250; // 툴팁 높이 상한(px) — 13px·줄 높이 1.5 로 약 12줄 + 안쪽 여백(share.css .fig-tip)
  // figArea 는 t 가 든 쪽 콘텐츠 영역이다 — 본문(.page-body) 또는 쪽의 직접 자식인 분할 패널(.page > .slide-panel)·배경(.page > .slide-bg).
  // 본문 안 사용자 컨테이너(:::slide-panel 등)는 본문으로 본다. 없으면 null.
  function figArea(t) {
    const pb = t.closest(".page-body");
    if (pb) return pb;
    const sp = t.closest(".slide-panel, .slide-bg"), pg = sp && sp.parentElement;
    return pg && pg.classList && pg.classList.contains("page") ? sp : null;
  }
  // figTarget 은 t 가 scope 안 쪽 콘텐츠 영역(figArea)의 대상 그림이면 그 img, 아니면 null 이다.
  function figTarget(t, scope) {
    if (!t || !scope || typeof t.closest !== "function" || String(t.tagName).toLowerCase() !== "img") return null;
    if (!scope.contains(t) || !figArea(t) || t.closest(FIG_SKIP)) return null;
    return t;
  }
  // figClip 은 글 text 가 fits(글) 에 들면 그대로, 아니면 끝을 줄이고 「…」 를 단 가장 긴 글이다(글자 단위 — 서로게이트 쌍을 가르지 않는다).
  // 「…」 하나도 들지 않으면 「…」.
  function figClip(text, fits) {
    if (fits(text)) return text;
    const cs = Array.from(text);
    const cut = n => cs.slice(0, n).join("").replace(/\s+$/, "") + "…";
    let lo = 0, hi = cs.length - 1;
    while (lo < hi) {
      const mid = Math.ceil((lo + hi) / 2);
      if (fits(cut(mid))) lo = mid; else hi = mid - 1;
    }
    return cut(lo);
  }
  // figText 는 그림의 설명(렌더된 alt 앞뒤 공백을 뺀 것 — 공백뿐이면 "")이다.
  function figText(img) { return String((img && img.getAttribute && img.getAttribute("alt")) || "").trim(); }
  // figLinked 는 그림이 링크(a — YouTube 대표 그림 포함) 안인가다 — 누르기는 링크의 것이다.
  function figLinked(img) { return !!(img && img.closest && img.closest("a")); }
  // figFocus 는 포커스를 받은 t 의 설명 그림이다 — 대상 그림 자신, 또는 링크면 그 안 첫 설명 그림. 없으면 null.
  function figFocus(t, scope) {
    if (!t || typeof t.closest !== "function") return null;
    const img = figTarget(t, scope);
    if (img) return figText(img) ? img : null;
    if (String(t.tagName).toLowerCase() !== "a" || !scope || !scope.contains(t)) return null;
    for (const x of t.querySelectorAll("img")) if (figTarget(x, scope) && figText(x)) return x;
    return null;
  }
  // figTipPlace 는 툴팁(w×h)을 그림 상자 r(화면 좌표 — getBoundingClientRect, 쪽 배율이 이미 든 값)에 맞춰 놓을 자리다(창 vw×vh, 가장자리 여백 pad).
  // 가로는 그림 가운데(창 안으로 당김), 세로는 아래에 들면 아래, 아니면 위, 둘 다 안 되면 그림 아래쪽 안에 겹친다(창 안으로 당김).
  function figTipPlace(r, w, h, vw, vh, gap, pad) {
    gap = gap === undefined ? 8 : gap;
    pad = pad === undefined ? 8 : pad;
    const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));
    const left = clamp(r.left + r.width / 2 - w / 2, pad, Math.max(pad, vw - w - pad));
    let top;
    if (r.bottom + gap + h <= vh - pad) top = r.bottom + gap;
    else if (r.top - gap - h >= pad) top = r.top - gap - h;
    else top = clamp(Math.min(r.bottom, vh) - h - gap, pad, Math.max(pad, vh - h - pad));
    return { left: Math.round(left), top: Math.round(top) };
  }

  // figTips 는 위 두 기능의 화면 쪽이다. d = { doc, win, scope(#pages), viewer(굴림 상자 — 굴리면 툴팁을 닫는다), active()(지금 보기 꼴이 대상인가),
  // lightbox()(크게 보기 부품 — 없으면 null), scale()(쪽 배율) }. 듣기는 처음 한 번 위임으로만 단다(본문을 바꿔도 다시 달지 않는다).
  // 돌려주는 것: arm()(본문을 바꾼 뒤 — 열린 툴팁·크게 보기를 닫고 새 그림에 tabindex·data-fig 를 단다, 몇 번 불러도 같다)·reset()(본문을 비우거나 끝난 화면)·
  // tipOn()·lbOn()·tip()(시험용 — 툴팁 요소).
  function figTips(d) {
    const doc = d.doc, scope = d.scope;
    // quiet 은 다시 열지 않을 그림이다 — 연 길(hover·focus)마다 따로: Esc 로 닫으면 그 길의 그림, 크게 보기를 닫고 포커스를 돌려주면 focus 의 그림.
    // hover 는 그 그림에서 포인터가 떠날 때, focus 는 포커스가 떠날 때 푼다(포커스가 남아 있어도 마우스를 다시 올리면 연다)
    let tip = null, cur = null, how = "";
    const quiet = { hover: null, focus: null };
    let lb = null, lbBtn = null, lbFrom = null;
    const lbOn = () => !!(lb && lb.kind());

    function tipEl() {
      if (tip) return tip;
      tip = doc.createElement("div");
      tip.className = "fig-tip";
      tip.setAttribute("aria-hidden", "true");
      tip.hidden = true;
      doc.body.appendChild(tip);
      return tip;
    }
    function show(img, src) {
      if (!img || img === quiet[src] || lbOn() || !d.active()) return;
      const text = figText(img);
      if (!text) return;
      const t = tipEl();
      t.style.left = "0px";
      t.style.top = "0px";
      t.style.maxHeight = Math.max(20, Math.min(d.win.innerHeight - 16, FIG_TIP_MAX_H)) + "px";
      t.hidden = false;
      // 높이 상한에 넘치면 끝을 줄인다(share.css .fig-tip overflow:hidden — scrollHeight 가 내용 전체 높이)
      t.textContent = figClip(text, s => { t.textContent = s; return t.scrollHeight <= t.clientHeight + 1; });
      cur = img;
      how = src;
      const p = figTipPlace(img.getBoundingClientRect(), t.offsetWidth, t.offsetHeight, d.win.innerWidth, d.win.innerHeight);
      t.style.left = p.left + "px";
      t.style.top = p.top + "px";
    }
    function hide() {
      if (tip) { tip.hidden = true; tip.textContent = ""; }
      cur = null;
      how = "";
    }

    function lbMake() {
      if (lb !== null) return lb;
      const L = d.lightbox();
      if (!L || typeof L.create !== "function") return null; // 부품이 나중에 오면 다음에 다시 본다
      lb = L.create(doc.body, { scale: d.scale, card: h => h.closest(".page"), caption: figText, cls: "fig-lb" });
      // aria-modal 은 달지 않는다 — 닫기 단추가 대화상자(.lb — 열 때마다 내용을 비운다) 밖 body 직계라, 달면 보조기술이 단추를 닫힌 바깥으로 본다.
      // 열린 동안 Tab 은 닫기 단추에 머문다(아래 창 keydown)
      lb.el.setAttribute("role", "dialog");
      lb.el.setAttribute("aria-label", "그림 크게 보기");
      lb.el.tabIndex = -1;
      lb.el.addEventListener("click", e => { if (e.target === lb.el && !lb.onScrollbar(e)) lbClose(true); });
      lbBtn = doc.createElement("button");
      lbBtn.type = "button";
      lbBtn.className = "lb-close fig-lb-close";
      lbBtn.title = "닫기(Esc)";
      lbBtn.setAttribute("aria-label", "크게 보기 닫기");
      lbBtn.textContent = "✕";
      lbBtn.hidden = true;
      lbBtn.addEventListener("click", () => lbClose(true));
      doc.body.appendChild(lbBtn);
      return lb;
    }
    function lbOpen(img) {
      const L = lbMake();
      if (!L) return false;
      hide();
      lbFrom = img;
      L.open(img);
      lbBtn.hidden = false;
      if (L.el.focus) L.el.focus({ preventScroll: true });
      return true;
    }
    // lbClose 는 크게 보기를 닫는다. back 이면 연 그림으로 포커스를 돌린다(그 포커스로 툴팁이 다시 열리지 않게 quiet.focus). 열려 있지 않으면 거짓.
    function lbClose(back) {
      if (!lbOn()) return false;
      lb.close();
      lbBtn.hidden = true;
      const f = lbFrom;
      lbFrom = null;
      if (back && f && f.isConnected && f.focus) { quiet.focus = f; f.focus({ preventScroll: true }); }
      return true;
    }

    scope.addEventListener("pointerover", e => {
      if (e.pointerType === "touch") return;
      const img = figTarget(e.target, scope);
      if (img && img !== cur) show(img, "hover");
    });
    scope.addEventListener("pointerout", e => {
      const img = figTarget(e.target, scope);
      if (!img) return;
      if (img === quiet.hover) quiet.hover = null;
      if (img === cur && how === "hover") hide();
    });
    scope.addEventListener("focusin", e => { const img = figFocus(e.target, scope); if (img) show(img, "focus"); });
    scope.addEventListener("focusout", e => {
      const img = figFocus(e.target, scope) || figTarget(e.target, scope);
      if (img && img === quiet.focus) quiet.focus = null;
      if (how === "focus") hide();
    });
    scope.addEventListener("click", e => {
      if (e.button !== 0 || e.defaultPrevented || e.shiftKey || e.metaKey || e.ctrlKey || e.altKey) return;
      const img = figTarget(e.target, scope);
      if (!img || figLinked(img) || !d.active()) return;
      if (lbOpen(img)) e.preventDefault();
    });
    scope.addEventListener("keydown", e => {
      if ((e.key !== "Enter" && e.key !== " ") || e.isComposing || e.shiftKey || e.metaKey || e.ctrlKey || e.altKey) return;
      const img = figTarget(e.target, scope);
      if (!img || img !== e.target || figLinked(img) || !d.active() || !d.lightbox()) return;
      e.preventDefault();
      e.stopPropagation(); // 쪽 넘김 키(Space)가 아니다
      lbOpen(img);
    });
    // capScroll 은 크게 보기가 열린 동안의 굴림 키(↑↓·PageUp/PageDown·Space/Shift+Space·Home/End)로 넘치는 설명 캡션(.lb-cap — lightbox.js 가 높이 상한을 건다)을
    // 굴린다. 캡션이 없거나 넘치지 않으면 아무것도 하지 않는다(뒤 화면도 굴리지 않는다). ⌘·Ctrl 조합은 아니다.
    function capScroll(e) {
      if (e.metaKey || e.ctrlKey || e.altKey) return;
      const c = lb && lb.el.querySelector && lb.el.querySelector(".lb-cap");
      if (!c || c.scrollHeight <= c.clientHeight) return;
      const page = Math.max(20, c.clientHeight - 24);
      const by = { ArrowDown: 40, ArrowUp: -40, PageDown: page, PageUp: -page, " ": e.shiftKey ? -page : page }[e.key];
      if (by) c.scrollTop += by;
      else if (e.key === "Home") c.scrollTop = 0;
      else if (e.key === "End") c.scrollTop = c.scrollHeight;
    }
    // 크게 보기가 열린 동안의 키는 이것이 먼저다(창 붙잡기 — 문서의 keydown 들보다 앞). Esc 는 닫기, Tab 은 닫기 단추로, 닫기 단추 위 Enter·Space 는 단추 그대로,
    // 그 밖은 뒤 화면으로 보내지 않는다(⌘·Ctrl 조합은 브라우저 몫으로 둔다). 닫혀 있으면 Esc 는 툴팁만 닫는다(목차 서랍 닫기 등 다른 Esc 도 그대로 간다).
    d.win.addEventListener("keydown", e => {
      if (lbOn()) {
        e.stopPropagation();
        if (e.key === "Escape") { e.preventDefault(); lbClose(true); return; }
        if (e.key === "Tab") { e.preventDefault(); lbBtn.focus(); return; }
        if ((e.key === "Enter" || e.key === " ") && e.target === lbBtn) return;
        if (!e.metaKey && !e.ctrlKey) e.preventDefault();
        capScroll(e); // 넘치는 설명 캡션은 굴림 키로 굴린다(캡션은 포커스를 받지 않는다 — aria-hidden)
        return;
      }
      if (e.key === "Escape" && cur) { quiet[how] = cur; hide(); }
    }, true);
    if (d.viewer) d.viewer.addEventListener("scroll", hide, { passive: true });
    d.win.addEventListener("resize", () => { hide(); if (lbOn()) lb.relayout(); });

    function reset() {
      hide();
      quiet.hover = quiet.focus = null;
      lbClose(false);
    }
    function arm() {
      reset();
      if (!d.active()) return;
      const zoom = !!d.lightbox();
      for (const img of scope.querySelectorAll("img")) { // 대상(본문·분할 패널·배경 — 사본 포함)은 figTarget 이 가른다
        if (!figTarget(img, scope) || img.hasAttribute("data-fig")) continue;
        const linked = figLinked(img), kind = !linked && zoom ? "zoom" : figText(img) ? "tip" : "";
        if (!kind) continue;
        // 링크 그림은 링크가 포커스를 받는다 — 표지만(크게 보기 커서 없음). 링크 밖 그림은 Tab 으로 닿게(이미 tabindex 가 있으면 둔다)
        img.setAttribute("data-fig", kind);
        if (!linked && !img.hasAttribute("tabindex")) img.tabIndex = 0;
      }
    }
    return { arm, reset, tipOn: () => !!cur, lbOn, tip: () => tip, close: () => lbClose(true) };
  }

  // 정식 이름은 window.ItdaShare 다. window.HyveShare 는 옛 이름(호환용 별칭) — 같은 객체를 한 번에 대입한다.
  window.ItdaShare = window.HyveShare = { lookFor, slideZoom, slideFit, SHORT_VIEW_H, backoff, connect, reloadGate, themeKey, themeApplier, fontGate, syncer, follower, backLabel, followAction, pickBlock, fragAt,
    restoreOwnLook, setBoardOK, pauseBackoff, slideGlider,
    lookSwitchHidden, afterResize, followLink, notesLayout, notesRightFits, notesSwitchState, paintNotesButtons, notePrefs, noteWidth, noteWidthCap, noteResizer, noteGripDrag, slideAnchor, anchorDelta, followSpot, ytFollowGate, initialLook, docLink, LINK_OFF_MSG, linkTo, markLinks, fileTreeHTML, mountFiles, docMissing, viewState,
    hashJump, hashId, headingFinder, tocJumper, tocHTML, tocIDs,
    tocTree, tocAncestors, tocFoldIsOpen, tocFoldFollow, tocFoldShown, tocFoldBulk, tocFoldClean, tocFoldList, tocFoldGet, tocFoldPut, tocFoldKey, tocFoldPrefs, tocFold, paintTocFold, tocBulkHead, paintTocBulk, TOC_FOLD_DOCS, TOC_FOLD_IDS, currentHeading, tocPanel, tocLayout, TOC_REVEAL_PAD, tocCover, tocRevealTop, tocScroller, tocReveal, tocRevealer, tocDocKey, TOC_TOUCH, keyHandler, notesPop, tableMenu, tableCopyClick, appName, docTitle, pick,
    hostSource, pickSource, figArea, figTarget, figClip, figText, figLinked, figFocus, figTipPlace, figTips };
  if (typeof document === "undefined" || !document.body) return;
  const source = pickSource(window, document.body);
  if (!source) return;
  setBoardOK(!!window.ItdaBoard); // 보드를 그릴 부품(dashboard.js)이 없으면 보드 꼴을 고르지 않는다(boardOK — 웹 뷰어)

  // ---- 화면

  const $ = id => document.getElementById(id);
  const base = source.base || "";
  // 호스트 부품은 정식 이름 window.Itda* 를 먼저, 없으면 옛 이름 window.Hyve* 를 본다(머리 주석 「호스트 부품」).
  const hostPart = name => pick(window, name);
  const TableCopy = hostPart("TableCopy"), TableSort = hostPart("TableSort"), CodeCopy = hostPart("CodeCopy"), CodeLines = hostPart("CodeLines");
  const Ask = window.ItdaAsk && typeof window.ItdaAsk.set === "function" ? window.ItdaAsk : null;
  const appMeta = document.querySelector('meta[name="application-name"]');
  const app = appName(appMeta && appMeta.getAttribute("content"));
  if (app) document.title = app; // 문서를 받기 전의 탭 제목 — 앱 이름이 없으면 share.html 의 title 그대로
  const state = { view: null, look: "", chosen: "", forced: false, themeFailed: false, ratio: "16:9", landscape: false, ended: false, rendering: null, again: false,
    // 발표자 노트(itda-hyve#344): 받은 view 의 슬라이드마다 노트(slideNotes — 원문 자리의 장, slides.js build itda-hyve#518)·노트가 하나라도 있는지(hasNotes)·지금 그린 배치(notesLayout)
    slideNotes: [], hasNotes: false, notesLayout: "off",
    // 오른쪽 배치가 지금 보기 영역 너비에 들어가는가(notesRightFits — applyZoom 이 배치를 정할 때 잰다). 아니면 「오른쪽」 단추를 비활성으로(paintNotesSwitch).
    notesRightFits: true,
    // 지금 화면에 걸린 표 정렬 지정(itda-hyve#349 2단계 — 이 화면만). 없으면 null
    tableSort: null,
    // 첫 view 를 받았는가(firstView)·폴더의 문서 목록({docs, rev, truncated}, 폴더 공유가 아니면 null)·첫 그리기 뒤 옮겨 갈 #제목
    seen: false, files: null, hash: "",
    // 지금 화면이 끝나지 않은 글꼴을 빼고 나뉘었는가(fontsLate 가 글꼴이 온 뒤 다시 그린다)
    fontStale: false };
  const NOTE_GAP = 20, NOTE_NARROW = 520; // 오른쪽 칸 너비는 보는 사람이 끌어 고른다(np.width — notePrefs)
  // gripDrag 는 노트 칸 너비 끌기(bindNoteGrip 이 단다 — 그 전엔 null), gripAnchor 는 끌기를 잡을 때 보던 자리(slideAnchor — 따라가는 중이면 null).
  // applyZoom·placeSlideNotes 가 배치를 바꿀 때 gripDrag.check() 로 끌기를 끝낼지 본다 — 그래서 여기(먼저 실행되는 자리)에 둔다.
  let gripDrag = null, gripAnchor = null;
  // nav 는 폴더 공유의 주소 쪽 일이다(원천이 api.nav 로 넘긴다 — 폴더 공유가 아니면 null). firstView 는 첫 view 로 한 번 정하는 것이다 — 처음 보기 꼴, 폴더 공유면 주소의 #제목.
  let nav = null;
  function firstView(v) {
    if (state.seen) return;
    state.seen = true;
    state.chosen = initialLook(state.chosen, nav ? nav.look() : "", v.look);
    if (nav) state.hash = nav.hash();
  }
  const np = notePrefs(() => window.localStorage, window.ItdaSlides.NOTE_FONTS);

  // ext 는 확장이 돌려준 훅이다(시작할 때 window.ItdaShareExt — 옛 이름 HyveShareExt — 가 돌려준 것 — 그 전에는 비어 있다). 훅이 던지면 콘솔에만 남기고 그리기 길을 이어 간다 — 확장 탓에
  // 본문이 안 붙거나 목차 강조가 멈추지 않게(itda-hyve#617 리뷰 3회차 [2]).
  let ext = {};
  const hook = (name, ...args) => {
    if (typeof ext[name] !== "function") return undefined;
    try { return ext[name](...args); } catch (e) { console.error("share ext:", name, e); return undefined; }
  };

  let toastTimer = 0;
  function toast(msg) {
    const t = $("toast");
    t.textContent = msg;
    t.hidden = false;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => { t.hidden = true; }, 3200);
  }

  const THEME_FAILED = "테마를 불러오지 못했습니다 — 화면을 새로 고쳐 주세요.";
  const PAUSE_MSG = { closed: "공유가 잠시 닫혔습니다 — 같은 주소로 다시 열리면 이어집니다", stale: "공유가 오래 닫혀 있어 다시 잇기를 멈췄습니다 — 공유가 다시 열렸으면 새로 고쳐 주세요",
    held: "화면을 새로 고쳐야 하는데 메모가 이 화면에만 있습니다 — 메모를 복사한 뒤 새로 고쳐 주세요" };
  // pauseCtl 은 잠시 닫힘 띠의 단추 상태다 — retry(지금 띠에 온 다시 잇기 함수, 없으면 null)·busy(누른 시도가 아직 안 끝남).
  const pauseCtl = { retry: null, busy: false };
  function paintRetry() {
    const band = document.getElementById("pause-band"), btn = band && band.querySelector(".pause-retry"); // 띠는 처음 쓸 때 만드는 자리라 화면 html 에 없다
    if (!btn) return;
    btn.hidden = !pauseCtl.retry;
    btn.disabled = pauseCtl.busy;
    btn.textContent = pauseCtl.busy ? "잇는 중…" : "지금 다시 잇기";
  }
  // 원천을 화면에 잇는다(hostSource 머리 주석의 api). fol 은 아래에서 만든다 — 원천은 start() 뒤에만 부른다.
  const linked = source.attach({
    $,
    ended: () => state.ended,
    current: () => state.view,
    show: v => { firstView(v); state.view = v; return draw(); },
    redraw: () => { if (!state.ended && state.view) draw(); },
    look: name => { state.chosen = name; return draw(); },
    failed: msg => { $("load-err").hidden = false; $("load-err").textContent = msg; },
    themeFailed: () => {
      if (state.ended) return;
      state.themeFailed = true;
      $("load-err").hidden = false;
      $("load-err").textContent = THEME_FAILED;
    },
    end: reason => showEnded(reason),
    holdReload: () => !!hook("holdReload"), // 확장이 없거나 던지면 막지 않는다
    // 띠는 문서 위에 떠 있어 쪽 나눔·굴림 자리를 건드리지 않는다. 끊기기 직전의 「문서를 받지 못했습니다」 는 띠가 대신한다(테마 실패는 그대로 둔다)
    // 「지금 다시 잇기」 단추는 retry 가 온 띠("closed"·"stale")에만 보인다 — 누르면 붙는 중(retry 의 Promise 가 풀리기 전)에는 막고 「잇는 중…」. 알림(role="status")은 글 칸에만 두어 단추 글자와 섞여 읽히지 않는다
    paused: (st, retry) => {
      let band = document.getElementById("pause-band");
      if (!band) { // 공유 화면만 쓰는 자리라 처음 쓸 때 만든다 — 뷰어 화면(web/viewer)은 이 길을 타지 않는다
        band = document.createElement("div");
        band.id = "pause-band";
        band.className = "pause-band";
        const msg = document.createElement("span");
        msg.className = "pause-msg";
        msg.setAttribute("role", "status");
        const btn = document.createElement("button");
        btn.type = "button";
        btn.className = "pause-retry";
        btn.addEventListener("click", () => {
          if (pauseCtl.busy || !pauseCtl.retry) return;
          pauseCtl.busy = true;
          paintRetry();
          Promise.resolve(pauseCtl.retry()).catch(() => false).then(() => { pauseCtl.busy = false; paintRetry(); });
        });
        band.append(msg, btn);
        document.querySelector(".main").appendChild(band);
      }
      band.hidden = !st;
      const text = PAUSE_MSG[st] || "", msg = band.querySelector(".pause-msg");
      if (msg.textContent !== text) msg.textContent = text; // 같은 글을 다시 넣어 알림이 되풀이되지 않게
      pauseCtl.retry = (st === "closed" || st === "stale") && typeof retry === "function" ? retry : null;
      paintRetry();
      if (!state.themeFailed && !(state.view && state.view.error)) $("load-err").hidden = true;
    },
    hello: d => { fol.hello(d); zoomCheck(); },
    slide: d => { fol.onSlide(d); zoomCheck(); },
    pos: d => fol.onPos(d),
    toast: msg => toast(msg),
    // 목록이 늦게 오거나 바뀌면 폴더 안 문서 링크를 되살리고 목록에서 빠진 문서 링크를 끈다 — 확장(files 훅)도 제 자리(hyve 노트 창)를 다시 건다
    files: d => { state.files = d; paintFiles(); markAll(); hook("files"); },
    nav: n => { nav = n; },
  });
  const applyTheme = v => linked.applyTheme(v);

  function imagesSettled(root, ms) {
    const imgs = [...root.querySelectorAll("img[src]")];
    return Promise.race([
      Promise.all(imgs.map(im => im.complete ? null : new Promise(r => { im.addEventListener("load", r, { once: true }); im.addEventListener("error", r, { once: true }); }))),
      new Promise(r => setTimeout(r, ms)),
    ]);
  }

  const fontWait = fontGate(document.fonts, 5000, { setTimer: (f, ms) => setTimeout(f, ms), late: () => fontsLate() });
  // fontsLate 는 지금 화면이 도착한 글꼴을 빼고 나뉘었으면 다시 나눈다. render 는 시작할 때 fontStale 을 내리고 나누기 직전에야 올리므로 도는 중에는 할 일이 없다 —
  // 그 글꼴은 도는 render 가 나눌 때 쓴다(draw 의 「한 번 더」 로 표 정렬이 풀리지 않는다). 공유가 끝났으면 draw 가 그리지 않는다. 내용이 바뀐 그리기가 아니므로 표 정렬은
  // 잇고, viewer.scrollTop 수치는 render 가 되돌린다 — 다시 나눈 뒤 같은 내용이 보인다는 뜻은 아니다.
  function fontsLate() {
    if (state.fontStale) draw(state.tableSort);
  }

  function lookClasses(el, look) {
    const slides = look === "slides", sized = slides || look === "dashboard"; // 보드의 폭·기준 상자는 슬라이드와 같다
    el.classList.toggle("slides", slides);
    el.classList.toggle("boards", look === "dashboard");
    el.classList.toggle("flow-look", look === "flow");
    el.classList.toggle("r43", sized && state.ratio === "4:3");
    el.classList.toggle("r169", sized && state.ratio !== "4:3");
    if (sized) { const [w, h] = window.ItdaSlides.SIZE[state.ratio]; el.style.setProperty("--page-w", w + "px"); el.style.setProperty("--page-h", h + "px"); }
    else { el.style.removeProperty("--page-w"); el.style.removeProperty("--page-h"); }
  }

  // gutterOf 는 보기 영역 너비 w 의 가로 여백이다(좁은 화면 16, 그 밖 56) — 맞춤 배율과 노트 칸 끌기의 상한이 함께 쓴다.
  function gutterOf(w) { return w < 600 ? 16 : 56; }
  // 쪽은 보기 영역 너비에 맞춰 그린 그림을 줄인다(호스트 앱의 「너비 맞춤」 과 같은 길 — transform). 슬라이드는 너비와 높이를
  // 모두 맞춰 축소한다(slideZoom — ⑥, 하한 0.2 에 걸리면 일부가 잘릴 수 있다). 이어보기는 너비가 화면을 따라 다시 흐르고 확대는 브라우저 것을 쓴다.
  function applyZoom() {
    const viewer = $("viewer"), pg = $("pages"), box = $("pages-box");
    if (state.look === "flow" || !pg.childElementCount) {
      box.classList.remove("scaled");
      box.style.width = box.style.height = pg.style.transform = pg.style.width = "";
      pg.classList.remove("notes-right", "notes-below");
      state.notesLayout = "off";
      if (gripDrag) gripDrag.check(); // 오른쪽이 아니게 됐다 — 끌던 것이 있으면 끝낸다
      window.ItdaSlides.layoutNotes(pg, $("notes"), { layout: "off" });
      paintNotesSwitch();
      return;
    }
    if (viewer.clientWidth === 0) return;
    const gutter = gutterOf(viewer.clientWidth);
    // 노트 칸(itda-hyve#344): 오른쪽이면 슬라이드 맞춤의 너비에서 칸(np.width + NOTE_GAP)을 뺀다. 칸은 #pages 밖(#notes)이라 크기·글자가 화면 픽셀이다(slides.js layoutNotes).
    // 아래 전환 문턱도 고른 너비로 잰다 — 아래 배치의 칸은 슬라이드 너비라 고른 너비를 쓰지 않는다.
    const noteW = np.width;
    const layout = notesLayout(np.mode, state.hasNotes, state.look, viewer.clientWidth, gutter, noteW, NOTE_GAP, NOTE_NARROW);
    const side = layout === "right" ? noteW + NOTE_GAP : 0;
    state.notesLayout = layout;
    if (gripDrag) gripDrag.check(); // 아래·끔으로 바뀌었으면 끌던 것을 끝낸다
    state.notesRightFits = notesRightFits(viewer.clientWidth, gutter, noteW, NOTE_GAP, NOTE_NARROW);
    paintNotesSwitch(); // 배치가 정해질 때마다(회전·창 크기·칸 너비 끌기·보기 꼴 바꿈 — 모두 applyZoom 을 지난다) 단추를 실제 배치로
    pg.classList.toggle("notes-right", layout === "right");
    pg.classList.toggle("notes-below", layout === "below");
    let w, z;
    if (state.look === "slides") {
      const [sw, sh] = window.ItdaSlides.SIZE[state.ratio];
      w = sw;
      z = slideFit(viewer.clientWidth - side, viewer.clientHeight, sw, sh, gutter).z;
    } else if (state.look === "dashboard") {
      w = window.ItdaSlides.SIZE[state.ratio][0]; // 보드는 긴 한 장이다 — 쪽처럼 너비에 맞춘다
      z = Math.min(1.6, Math.max(0.2, (viewer.clientWidth - gutter) / w));
    } else {
      w = (state.landscape ? 297 : 210) * PX_PER_MM;
      z = Math.min(1.6, Math.max(0.2, (viewer.clientWidth - gutter) / w));
    }
    const outW = Math.max(viewer.clientWidth, Math.ceil(w * z + side));
    pg.style.setProperty("--nz", String(z)); // 공유 메모 표지의 화면 픽셀 역산(호스트 sharememo-view.css)
    box.classList.add("scaled");
    pg.style.width = outW / z + "px";
    pg.style.transform = "scale(" + z + ")";
    box.style.width = outW + "px";
    window.ItdaSlides.layoutNotes(pg, $("notes"), { layout, zoom: z, noteW, gap: NOTE_GAP }); // 아래 배치의 여백이 #pages 높이에 들어가므로 높이를 재기 전에
    box.style.height = Math.ceil(pg.offsetHeight * z) + "px";
    if (boardZoom) boardZoom.fit(); // 크게 보기 타일은 보이는 영역을 채운다
  }

  // 표 복사(itda-hyve#349) 메뉴가 고른 표의 조각들. render 가 본문을 바꾸거나 비우면 메뉴를 닫고 버린다 — 열린 메뉴가 화면에서 떨어진 옛 표를 복사하지 않게.
  // 메뉴의 열고 닫기·키·포커스 돌려주기는 tableMenu(itda-hyve#376)가 쥔다.
  let ctxTables = null;
  const tmenu = tableMenu({ menu: $("table-ctx"), active: () => document.activeElement, view: () => ({ w: window.innerWidth, h: window.innerHeight }),
    now: () => Date.now(), onClose: () => { ctxTables = null; } });
  function closeTableMenu() { tmenu.close(); ctxTables = null; }
  // 발표자 노트 판(itda-hyve#1040): 「노트」 단추를 누르면 위치·글자 크기 판이 열린다 — 여닫기는 notesPop 이 쥔다. 공유 화면(share.html)과 웹 뷰어
  // (viewer/index.html)가 같은 단추·판을 둔다. 둘이 없는 화면(옛 html)은 여닫을 것이 없다(빈 짝)
  const npop = $("notes-btn") && $("notes-pop") ? notesPop({ doc: document, btn: $("notes-btn"), pop: $("notes-pop") }) : { isOpen: () => false, set() {} };

  // 그림 설명·크게 보기(itda-doc#121 — figTips). 크게 보기 부품은 호스트의 ItdaLightbox(공유 화면·묶은 교안이 싣는다 — 뷰어에는 없다)
  const figs = figTips({ doc: document, win: window, scope: $("pages"), viewer: $("viewer"),
    active: () => !!state.view && !state.ended && (state.look === "doc" || state.look === "flow" || state.look === "slides"),
    lightbox: () => { const L = hostPart("Lightbox"); return L && typeof L.create === "function" ? L : null; },
    scale: () => parseFloat($("pages").style.getPropertyValue("--nz")) || 1 });

  // 보드 크게 보기(itda-work/itda-hyve#362 — dashboard.js zoomer). 발표를 따라가는 동안(fol.holdsLook — 발표가 꼴을 붙듦, 잠시 풀린 동안 포함)은
  // 열지 않고, 열려 있으면 다음 이벤트·발표 메시지(slide·hello)에서 닫는다. 보드를 그릴 수 없는 화면(웹 뷰어 — ItdaBoard 없음)에는 없다.
  const boardZoom = window.ItdaBoard && window.ItdaBoard.zoomer ? window.ItdaBoard.zoomer($("pages"), { blocked: () => fol.holdsLook(), scroller: $("viewer") }) : null;
  const zoomCheck = () => { if (boardZoom) boardZoom.current(); }; // current() 가 막힘(발표 따라가기)을 보고 닫는다

  // render 는 호스트 앱 보기 그리기의 보기 부분이다: 무대(화면 밖)에서 나눈 뒤 쪽을 한 번에 바꾼다.
  // 공유가 끝나면(state.ended) 각 await 뒤·DOM 쓰기 전에 멈춘다 — 늦게 끝난 그리기가 「공유가 끝났습니다」 를 덮지 않게.
  // sort 는 표 정렬 지정(tablesort.js — 이 화면만, 서버·발표자에게 보내지 않는다)이다. 머리 칸을 누른 그리기(sortByHead)만 넘기고, 호스트
  // 갱신(version)·꼴 바꾸기·화면 폭 변경으로 다시 그리면 넘기지 않으므로 풀린다.
  async function render(v, sort) {
    if (state.ended) return;
    ytStop(); // 다시 그리면 플레이어가 놓인 쪽도 새로 만든다
    state.fontStale = false; // 이 그리기가 글꼴을 다시 본다 — 그동안 도착한 글꼴로 다시 그릴 일이 없다(fontsLate)
    if (!v.error && !(await applyTheme(v))) return; // 다시 불러오는 중
    if (state.ended) return;
    $("load-err").hidden = !v.error && !state.themeFailed;
    $("load-err").textContent = v.error || (state.themeFailed ? THEME_FAILED : "");
    document.title = docTitle(v.title || "문서", app, source.tabTitle);
    $("doc-title").textContent = v.title || "";
    paintTOC(v);
    if (v.error) state.tableSort = null; // 본문을 비운다 — 아래 줄(시험이 글로 보는 자리)
    if (v.error) hook("tick"); // 확장: 오류 view 는 본문을 다시 만들지 않는다 — 지난 본문 기준 그대로
    if (v.error) { closeTableMenu(); figs.reset(); $("pages").textContent = $("notes").textContent = ""; findHeading.refresh(); return; }
    state.landscape = v.paper === "a4-landscape";
    document.body.classList.toggle("paper-a4-landscape", state.landscape);
    document.body.classList.toggle("paper-a4", !state.landscape);
    const lists = (v.list_style || "").split(" ");
    document.body.classList.toggle("list-gaejosik", lists.includes("gaejosik"));
    document.body.classList.toggle("list-gongmun", lists.includes("gongmun"));
    document.documentElement.lang = v.lang || "ko";
    await fontWait.wait(); // 테마 CSS(applyTheme)가 선언한 face 까지 본다
    if (state.ended) return;
    const sv = v.slides || {}, bv = v.board || {};
    state.look = lookFor(v, window.innerWidth, state.chosen, state.forced);
    state.ratio = (state.look === "dashboard" ? bv.ratio : sv.ratio) === "4:3" ? "4:3" : "16:9";
    paintLookSwitch();
    for (const b of document.querySelectorAll("[data-look]")) b.classList.toggle("on", b.dataset.look === state.look);
    const stage = $("stage");
    stage.textContent = "";
    const flow = document.createElement("div");
    flow.className = "page-body doc-flow";
    stage.appendChild(flow);
    flow.innerHTML = v.html; // mdoc 이 만든 HTML(스크립트 없음). CSP 가 2차로 막는다
    window.ItdaPager.numberBlocks(flow); // 내용 앵커(⑧) — 호스트와 같은 규칙으로 나누기 전에
    hook("flow", flow); // 확장: 나누기 전 흐름(쪽·슬라이드 나눔이 쪼개거나 옮기기 전)
    await imagesSettled(flow, 5000);
    if (state.ended) { stage.textContent = ""; return; }
    state.fontStale = fontWait.behind(); // 여기부터 render 끝까지 await 가 없다 — 이 판단과 나누기 사이에 글꼴이 끼어들지 않는다
    const target = document.createElement("div");
    target.className = "pages-inner";
    stage.appendChild(target);
    // 표 정렬(itda-hyve#349 2단계): 쪽·이어보기에서만, 나누기 전 흐름에(문서 창과 같다 — 쪽 나눔이 정렬된 표 전체를 다시 나눈다)
    let sorted = null;
    if (state.look !== "slides" && state.look !== "dashboard" && TableSort) {
      TableSort.mark(flow);
      if (sort && TableSort.apply(flow, sort)) sorted = sort;
    }
    // 코드 줄 번호(itda-hyve#435 — 호스트 부품 CodeLines): 쪽 나눔·슬라이드 나눔 전에 감싼다(호스트 앱 창과 같은 자리).
    // frontmatter code.line-numbers: false 면 줄 강조(itda-hyve#479 data-hl)·단 나누기(itda-hyve#487 data-cols) 블록만 번호 없이 감싼다. 스크립트가 404 여도(허용 목록 누락 — 리뷰 R1) 렌더는 깨지 않는다.
    if (CodeLines) CodeLines.wrap(flow, { numbers: (v.code || {}).line_numbers !== false });
    if (Ask) {
      Ask.set(v.ask, { title: v.title });
      Ask.armSections(flow);
    }
    if (state.look === "dashboard") {
      // 움직임(dashboard.js motion): target 은 그릴 때마다 새것이라 앞 그림의 기억은 늘 남는 #pages 에 묶고(motionScope), 문서 열쇠(tocDocKey — 목차 따라가기와 같은 열쇠)가
      // 바뀌면 처음 그림으로 다룬다(motionDoc — 다른 문서의 같은 제목 타일을 강조하지 않는다). 같은 문서의 호스트 갱신(version)은 등장 없이 바뀐 타일만 강조하고,
      // 보기 꼴을 쪽·슬라이드로 바꿨다 돌아와도 같은 문서면 다시 그림이다(그 사이 바뀐 타일만 강조). 발표를 따라가는 동안(fol.holdsLook)은 등장을 끈다 —
      // 그린 직후 scrollToFollow 가 타일 안 블록의 getBoundingClientRect 로 굴리는데 등장 중 translateY 만큼 어긋난다(강조는 자리를 바꾸지 않아 그대로 둔다)
      window.ItdaBoard.build(flow, target, { ratio: state.ratio, cols: bv.cols, rows: bv.rows, level: bv.level, logo: v.logo, info: v.info,
        motionScope: $("pages"), motionDoc: tocDocKey(source, v), enter: !fol.holdsLook() });
    } else if (state.look === "slides") {
      const org = (flow.querySelector(".cover-org, .title-org") || {}).textContent || "";
      const r = window.ItdaSlides.build(flow, target, { ratio: state.ratio, level: sv.level, incremental: sv.incremental, fontScale: sv.font_scale, logo: v.logo, footer: org.trim(), info: v.info });
      // 노트는 호스트가 「노트 함께 보이기」 를 켰을 때만 온다 — 오지 않았으면 컨트롤을 숨기고 칸도 없다
      state.slideNotes = r.notes || [];
      state.hasNotes = state.slideNotes.some(Boolean);
    } else if (state.look === "flow") {
      for (const x of flow.querySelectorAll(".pause, .notes")) x.remove();
      window.ItdaPager.adoptWidths(flow);
      const pg = document.createElement("div");
      pg.className = "page page-flow";
      const body = document.createElement("div");
      body.className = "page-body flow-view";
      while (flow.firstChild) body.appendChild(flow.firstChild);
      pg.appendChild(body);
      target.appendChild(pg);
    } else {
      window.ItdaPager.prepareTOC(flow);
      for (const x of flow.querySelectorAll(".pause, .notes")) x.remove();
      window.ItdaPager.paginate(flow, target, { header: v.header, footer: v.footer, h1Break: v.h1_break, h1Always: v.h1_always });
    }
    flow.remove();
    if (state.look !== "slides") { state.slideNotes = []; state.hasNotes = false; }
    paintNotesSwitch();
    const pagesEl = $("pages");
    lookClasses(pagesEl, state.look);
    document.body.classList.toggle("look-slides", state.look === "slides");
    document.body.classList.toggle("look-board", state.look === "dashboard");
    document.body.classList.toggle("look-flow", state.look === "flow");
    const frag = document.createDocumentFragment();
    while (target.firstChild) frag.appendChild(target.firstChild);
    const viewer = $("viewer"), top = viewer.scrollTop; // 제자리 갱신(version)은 읽던 자리를 둔다
    closeTableMenu();
    pagesEl.replaceChildren(frag);
    placeSlideNotes(pagesEl); // 노트 칸(슬라이드 보기 밖이면 지우기만)은 쪽을 바꿔 끼운 뒤에 — 칸은 #notes 에 있어 쪽과 함께 갈리지 않는다
    // 코드 블록 복사 단추(itda-hyve#413 — codecopy.js): 쪽 나눔이 끝난 뒤에 붙인다(나누기 전에 붙으면 splitPre 가 단추를 줄로 센다).
    if (CodeCopy) { CodeCopy.arm(pagesEl); CodeCopy.arm($("notes")); }
    figs.arm(); // 그림 설명·크게 보기(itda-doc#121): 쪽 나눔이 끝난 새 본문에 — 열린 툴팁·크게 보기는 옛 그림과 함께 닫는다

    if (boardZoom && state.look === "dashboard") boardZoom.arm(); // 크게 보기 단추 — 새 보드의 타일마다(옛 보드는 떨어져 열린 상태도 버려진다)
    state.tableSort = sorted;
    findHeading.refresh(); // 본문이 바뀌었다 — 강조가 쓰는 id Map 을 새 본문으로(⑦)
    target.remove();
    applyZoom();
    if (fol.active()) scrollToFollow(fol.index, fol.kind, fol.frac); // 따라가는 중이면 읽던 자리 대신 발표자의 슬라이드·내용 위치로
    else viewer.scrollTop = top;
    markAll(); // 열지 않는 파일 링크를 링크 모양 없이 끈다(itda-hyve#1096) — 확장(drawn)이 보기 전에
    hook("drawn"); // 확장: 새 본문을 다 그렸다
    if (state.hash) { const h = state.hash; state.hash = ""; jumper.jump(h, false); } // 다른 문서의 #제목 링크로 왔다
    paintTocOn();
  }

  // ---- 발표자 노트(itda-hyve#344)

  let noteWatch = null;
  function placeSlideNotes(box) {
    // grip: 오른쪽 배치에서 칸 너비를 끄는 손잡이(.sn-grip — 보이기는 호스트 앱 창 틀 CSS 의 #pages.notes-right ~ #notes .sn-grip, 끌기는 bindNoteGrip)
    window.ItdaSlides.placeNotes(box, { notes: state.slideNotes, on: state.look === "slides" && state.hasNotes && np.mode !== "off", into: $("notes"), grip: true });
    if (gripDrag) gripDrag.check(); // 칸을 다시 만들어 붙든 손잡이가 빠졌으면 끌기를 끝낸다
    if (!noteWatch) noteWatch = window.ItdaSlides.watchNotes($("notes"), applyZoom); // 아래 칸의 그림이 늦게 와 높이가 바뀌면 다시 놓는다
    noteWatch();
  }
  // applyNoteFont 는 고른 글자 크기를 문서 뿌리의 --note-fs 로 건다(0 이면 기본 14 — 호스트 앱 창 틀 CSS 의 .slide-note).
  function applyNoteFont() {
    const r = document.documentElement.style;
    if (np.font) r.setProperty("--note-fs", String(np.font)); else r.removeProperty("--note-fs");
    $("note-fs-val").textContent = String(np.font || 14);
  }
  // paintNotesSwitch 는 노트 컨트롤을 노트가 온 슬라이드 보기에서만 보인다(공유가 끝났으면 숨김 — 숨길 때 판도 닫는다). 켜진 표시는 실제 배치에 붙고, 오른쪽이 들어가지
  // 않는 너비에서는 「오른쪽」 을 비활성(disabled)으로 두고 title 을 이유로 바꾼다(notesSwitchState·paintNotesButtons). 막대의 「노트」 단추(#notes-btn)는
  // 고른 값이 끔이 아니면 .on 이다(실제 배치가 아니라 고른 값 — 좁아 아래로 그려도 켜져 있다).
  function paintNotesSwitch() {
    $("notes-switch").hidden = state.ended || state.look !== "slides" || !state.hasNotes;
    if ($("notes-switch").hidden) npop.set(false);
    if ($("notes-btn")) $("notes-btn").classList.toggle("on", np.mode !== "off");
    paintNotesButtons(document.querySelectorAll("[data-notes]"), notesSwitchState(np.mode, state.notesLayout, state.notesRightFits));
    for (const b of document.querySelectorAll("[data-note-font]")) b.disabled = np.mode === "off";
  }
  // slideAtMiddle 은 보기 영역 가운데에 걸린 슬라이드 번호다(배치를 바꾼 뒤 그 슬라이드로 돌아오려고). 없으면 -1.
  function slideAtMiddle() {
    const a = anchorAtMiddle();
    return a ? a.i : -1;
  }
  // viewerMid 는 보기 영역 세로 가운데(화면 y), slidePages 는 슬라이드들이다.
  function viewerMid() { const vr = $("viewer").getBoundingClientRect(); return vr.top + vr.height / 2; }
  function slidePages() { return [...$("pages").querySelectorAll(":scope > .page")]; }
  // anchorAtMiddle 은 보기 영역 가운데에 걸린 슬라이드와 그 안의 자리다(slideAnchor). 없으면 null.
  function anchorAtMiddle() { return slideAnchor(slidePages().map(pg => pg.getBoundingClientRect()), viewerMid()); }
  // relayoutNotes 는 노트 배치·글자를 바꾼 뒤다: 칸을 다시 붙이고(배치 바꿈만) 배율을 다시 건 뒤, 따라가는 중이면 발표자 위치로, 아니면
  // 보던 슬라이드로 돌아온다.
  function relayoutNotes(replace) {
    const keep = fol.active() ? -1 : slideAtMiddle();
    if (replace) {
      placeSlideNotes($("pages"));
      markLinks($("notes"), linkOpts()); // 다시 만든 노트 칸의 링크도 같은 규칙으로 끈다(itda-hyve#1096)
      // 노트 칸을 다시 만들었으니 복사 단추(itda-hyve#413)도 다시 붙인다 — 노트 코드의 조각 범위는 그 노트 안(codecopy.js noteOf)
      if (CodeCopy) CodeCopy.arm($("notes"));
    }
    applyZoom();
    realignNotes(keep);
    paintNotesSwitch();
  }
  // realignNotes 는 노트 배치·너비를 바꿔 배율을 다시 건 뒤의 자리 맞추기다: 따라가는 중이면 발표자 위치로(move 없이 — 바로), 아니면
  // keep 번째 슬라이드로(placeSlide). keep 이 -1 이면 그대로 둔다.
  function realignNotes(keep) {
    if (fol.active()) scrollToFollow(fol.index, fol.kind, fol.frac);
    else if (keep >= 0) placeSlide($("pages").querySelectorAll(":scope > .page")[keep]);
  }
  // 노트 칸 너비 끌기(itda-hyve#728 실기 후속): 오른쪽 칸 왼쪽 손잡이(.sn-grip)를 끌면 너비가 바뀌고(왼쪽으로 끌면 넓어짐), 맞춤 배율·아래 전환 문턱이
  // 그 너비로 다시 계산된다(applyZoom). 끄는 동안은 지금 화면 폭에서 오른쪽이 유지되는 너비까지만(noteWidthCap — 움직일 때마다 다시 잰다), 잡을 때
  // 보던 슬라이드 안의 자리(gripAnchor — 따라가는 중이면 발표자 위치)를 지킨다. 끌기가 끝날 때 한 번 이 브라우저에 기억한다(np.setWidth — 호스트에
  // 보내지 않는다). 두 번 누르면 기본 320. 끝나는 길(놓기·취소·캡처 상실·창 이탈·탭 숨김·손잡이가 빠지는 재그리기·배치 전환)은 noteGripDrag 의 finish 하나다.
  const resizer = noteResizer({
    width: () => np.width,
    cap: () => { const w = $("viewer").clientWidth; return noteWidthCap(w, gutterOf(w), NOTE_GAP, NOTE_NARROW); },
    apply: (px, save) => {
      if (save) np.setWidth(px); else np.width = noteWidth(px);
      applyZoom();
      keepGripAnchor();
    },
  });
  // keepGripAnchor 는 너비를 바꿔 배율을 다시 건 뒤의 자리 맞추기다: 따라가는 중이면 발표자 위치로(move 없이 — 바로), 아니면 잡을 때 보던
  // 슬라이드 안의 같은 자리를 보기 영역 가운데에 다시 둔다(슬라이드 가운데로 옮기지 않는다).
  function keepGripAnchor() {
    if (fol.active()) { realignNotes(-1); return; }
    const a = gripAnchor, pg = a && slidePages()[a.i];
    if (pg) $("viewer").scrollTop += anchorDelta(pg.getBoundingClientRect(), a.frac, viewerMid());
  }
  // 보기 영역의 따라가기 해제(mousedown 은 viewer 자신만·touchmove 는 손잡이 제외)에 걸리지 않는다.
  function bindNoteGrip() {
    gripDrag = noteGripDrag({
      notes: $("notes"), doc: document, win: window, body: document.body, resizer,
      canGrip: () => state.notesLayout === "right",
      onStart: () => { gripAnchor = fol.active() ? null : anchorAtMiddle(); },
    });
  }
  function setNotesMode(mode) {
    if (mode === np.mode || !["off", "right", "below"].includes(mode)) return;
    np.setMode(mode);
    relayoutNotes(true);
  }
  function stepNoteFont(dir) {
    const px = window.ItdaSlides.noteFontStep(np.font || 14, dir);
    if (px === (np.font || 14)) return;
    np.setFont(px);
    applyNoteFont();
    relayoutNotes(false);
  }
  // placeSlide 는 슬라이드 하나를 보기 영역에 둔다 — 아래에 노트 칸이 붙어 보이면 위에 붙여(노트까지 보이게), 아니면 가운데(followSpot).
  function placeSlide(pg) {
    if (pg) $("viewer").scrollTop += slideDelta(pg);
  }
  // slideDelta 는 지금 굴린 자리에서 슬라이드 pg 를 placeSlide 의 자리에 두기까지 굴릴 양이다(px).
  function slideDelta(pg) {
    const vr = $("viewer").getBoundingClientRect(), r = pg.getBoundingClientRect();
    const nt = $("notes").querySelector('[data-slide="' + ([...$("pages").querySelectorAll(":scope > .page")].indexOf(pg) + 1) + '"]');
    const shown = !!(nt && nt.offsetParent !== null);
    if (followSpot(state.notesLayout, shown) === "top") return r.top - vr.top - 8;
    return (r.top + r.height / 2) - (vr.top + vr.height / 2);
  }
  // glider 는 슬라이드 보기 따라가기의 굴림이다(slideGlider) — 옮김만 부드럽게, 줄인 움직임을 고른 사람에게는 늘 바로.
  const reducedMotion = window.matchMedia ? window.matchMedia("(prefers-reduced-motion: reduce)") : null;
  const glider = slideGlider(() => !!(reducedMotion && reducedMotion.matches));

  // ---- 목차(④)

  const toc = tocPanel();
  let tocIds = [], tocFrame = 0, tocKeep = -1;
  // 목차 따라 굴리기(itda-hyve#991 — tocRevealer): 굴리는 것은 tocReveal(사이드바나 목차 상자만), 공유가 끝났으면 굴리지 않는다
  const revealer = tocRevealer(f => requestAnimationFrame(f), id => cancelAnimationFrame(id), () => { if (!state.ended) tocReveal($("side-toc"), $("sidebar"), window); });
  const findHeading = headingFinder($("pages")); // 제목은 그린 본문(#pages) 안에서만 찾는다
  // 목차 접기(itda-hyve#985): 문서마다 기억(foldPrefs — 이 브라우저의 localStorage, 열쇠는 원천의 foldKey(v) — 없거나 "" 면 기억 없이 이 화면 메모리만)하고,
  // 열쇠가 바뀌면(폴더 공유의 다른 문서) 새 상태를 만든다.
  // 일괄 단추는 「목차」 머리(.side-title) 옆에 단다(tocBulkHead — 머리를 div.toc-head 로 싼다). 머리가 없는 화면이면 단추도 없다.
  const foldPrefs = tocFoldPrefs(() => window.localStorage);
  let fold = tocFold(null, null), foldKey = null;
  const tocTitle = $("sidebar").querySelector(":scope > .side-title");
  const tocBulk = tocBulkHead(document, tocTitle);

  // paintTOC 는 view 의 목차를 사이드바에 그린다(오류 view·제목 없는 문서면 비움 — 단추·사이드바를 숨긴다).
  function paintTOC(v) {
    const has = v && !v.error && Array.isArray(v.toc) && v.toc.length > 0;
    revealer.doc(tocDocKey(source, v)); // 다른 문서면 앞 지금 항목을 잊는다 — 첫 paintTocOn 이 새 목차로 맞춘다(같은 문서 다시 그리기는 그대로)
    if (has) {
      const key = typeof source.foldKey === "function" ? String(source.foldKey(v) || "") : "";
      if (key !== foldKey) { foldKey = key; fold = key ? tocFold(foldPrefs.load(key), m => foldPrefs.save(key, m)) : tocFold(null, null); }
      fold.setTree(v.toc);
    }
    const html = has ? tocHTML(v.toc, id => fold.open(id)) : "";
    $("side-toc").innerHTML = html; // 글자는 tocHTML 이 이스케이프한다
    tocIds = html ? tocIDs(v.toc) : [];
    if (!tocIds.includes(jumper.pin)) jumper.clear();
    paintTocBulk(tocBulk, fold, !html);
    paintFiles();
  }
  // foldTo 는 접기 상태가 바뀐 뒤 목차를 고친다 — 단추·하위 ul·일괄 단추, 그리고 강조(접힌 가지 안이면 보이는 조상에 — paintTocOn).
  function foldTo() {
    paintTocFold($("side-toc"), fold);
    paintTocBulk(tocBulk, fold, !tocIds.length);
    paintTocOn();
  }

  // paintFiles 는 폴더의 파일 목록을 그린다(폴더 공유가 아니면 아무것도 만들지 않는다). 제목이 없는 문서면 「목차」 머리만 숨긴다.
  function paintFiles() {
    mountFiles(document, $("sidebar"), state.files, state.view && state.view.doc);
    const head = tocBulk ? tocBulk.parentNode : tocTitle;
    if (state.files && head) head.hidden = !tocIds.length;
    paintTocPanel();
  }

  // paintTocPanel 은 목차 단추·사이드바·서랍 가림막을 tocLayout 대로 둔다. 공유가 끝났으면 숨긴다.
  function paintTocPanel() {
    const l = tocLayout((tocIds.length > 0 || !!state.files) && !state.ended, toc);
    $("btn-toc").hidden = l.hidden;
    $("sidebar").hidden = l.hidden;
    $("toc-back").hidden = !l.back;
    document.body.classList.toggle("toc-drawer", l.drawer);
    document.body.classList.toggle("no-toc", !l.open);
    $("btn-toc").classList.toggle("on", l.open);
    $("btn-toc").setAttribute("aria-expanded", String(l.open));
  }

  // headingNow 는 지금 보이는 가장 위 제목의 id 다(currentHeading — 목차가 없으면 ""). 제목 위치는 배율(transform)이 걸린 화면에서 잰다.
  function headingNow() {
    if (!tocIds.length) return "";
    const viewer = $("viewer"), vtop = viewer.getBoundingClientRect().top, st = viewer.scrollTop;
    const ix = findHeading.index(), positions = [];
    for (const id of tocIds) {
      const el = ix.get(id);
      if (el) positions.push({ id, top: el.getBoundingClientRect().top - vtop + st });
    }
    return currentHeading(positions, st, jumper.pin, viewer.clientHeight);
  }
  // paintTocOn 은 지금 제목(headingNow)의 항목에 .on 을 달고 확장의 tick 을 부른다. 지금 제목이 바뀌면 그 조상을 펼친다(사람이 접은 것은
  // 그대로 — fold.follow). 그 항목이 접힌 가지 안이라 보이지 않으면 보이는 가장 가까운 조상에 .on-within 을 단다(.on 은 그대로 하나 — 확장·시험이 본다).
  // 공유가 끝났으면 아무것도 하지 않는다(끝나기 직전에 예약된 프레임 포함).
  function paintTocOn() {
    if (state.ended) return;
    if (tocIds.length) {
      const cur = headingNow();
      if (fold.follow(cur)) { paintTocFold($("side-toc"), fold); paintTocBulk(tocBulk, fold, false); }
      const shown = cur ? fold.shown(cur) : "";
      for (const a of $("side-toc").querySelectorAll("a")) {
        a.classList.toggle("on", a.dataset.id === cur);
        a.classList.toggle("on-within", shown !== cur && a.dataset.id === shown);
      }
      // 지금 항목이 바뀌었을 때만 굴린다 — 사람이 목차를 굴려 보는 동안 본문이 조금 움직였다고 되돌리지 않는다(itda-hyve#991)
      revealer.see(cur);
    }
    hook("tick");
  }
  function scheduleTocOn() {
    if (tocFrame || state.ended) return;
    tocFrame = requestAnimationFrame(() => { tocFrame = 0; paintTocOn(); });
  }

  // closeTocDrawer 는 서랍이면 닫는다(넓은 화면의 펼친 사이드바는 그대로).
  function closeTocDrawer() {
    if (toc.wide || !toc.open) return;
    toc.close();
    paintTocPanel();
  }

  // scrollToFollow 는 따라가는 위치를 맞추고 따라가는 표시(.following)를 단다. 아직 맞출 수 없으면(그리는 중·그 꼴이 아님) 아무것도 하지
  // 않는다 — 그리기가 끝나면 render 가 다시 부른다.
  //  - slide: 슬라이드 보기에서 i 번째 슬라이드를 보기 영역 가운데로.
  //  - pos: 보기 꼴 그대로. 슬라이드 보기면 블록 i 가 든 슬라이드를 가운데로, 쪽·이어보기면 블록 i 의 조각들(쪽 나눔이 쪼갠 것) 가운데
  //    비율 frac 이 닿는 지점(fragAt)을 보기 영역 맨 위 + 1px(호스트의 기준선과 같다)에. 그 꼴에 없는 블록이면 가까운 앞 블록(pickBlock).
  const ytFollowChanged = ytFollowGate();
  // move 는 발표자가 옮겨서 맞추는 때다(fol 의 align) — 슬라이드 보기에서 다른 슬라이드로 옮기면 부드럽게 굴린다(glider). 그리기·노트 배치·창 크기 뒤의 재정렬은 move 없이 바로 간다.
  function scrollToFollow(i, kind, frac, move) {
    const root = $("pages"), viewer = $("viewer"), vr = viewer.getBoundingClientRect();
    if (ytFollowChanged(i, kind)) ytStop(); // 따라간 슬라이드 번호가 바뀌면 소리가 남지 않게(같은 슬라이드 안 조각·내용 위치는 그대로)
    for (const x of root.querySelectorAll(".following")) x.classList.remove("following");
    const center = el => glider.to(viewer, el, slideDelta(el), !!move); // 아래 노트가 붙어 보이면 위에 붙인다(itda-hyve#344 — placeSlide 의 자리)
    if (kind === "slide") {
      if (state.look !== "slides") return; // 보드 발표의 번호(0 / 1)는 꼴만 붙든다 — 위치는 내용 위치(pos)로 온다
      const pg = root.querySelectorAll(".page")[i];
      if (!pg) return;
      pg.classList.add("following");
      center(pg);
      return;
    }
    if (kind !== "pos") return;
    const all = [...root.querySelectorAll("[data-n]")];
    const ns = [...new Set(all.map(el => Number(el.dataset.n)))].sort((a, b) => a - b);
    const n = pickBlock(ns, i);
    if (n < 0) return;
    const els = all.filter(el => Number(el.dataset.n) === n);
    if (state.look === "slides") {
      const pg = els[0].closest(".page");
      if (!pg) return;
      pg.classList.add("following");
      center(pg);
      return;
    }
    for (const el of els) el.classList.add("following");
    const rs = els.map(el => el.getBoundingClientRect());
    const at = fragAt(rs.map(r => r.height), n === i ? frac : 0);
    viewer.scrollTop += rs[at.i].top + at.off - (vr.top + 1);
  }

  // draw 는 그리기를 하나씩 돌린다(그리는 중에 또 부르면 끝난 뒤 한 번 더). sort(표 정렬 지정)는 첫 그리기에만 건다 — 그 뒤의 한 번 더는
  // 다른 까닭(호스트 갱신 등)의 그리기라 정렬이 풀린다.
  async function draw(sort) {
    if (state.rendering) { state.again = true; return state.rendering; }
    state.rendering = (async () => {
      let s = sort || null;
      do {
        state.again = false;
        if (state.ended || !state.view) break;
        await render(state.view, s);
        s = null;
      } while (state.again);
    })();
    try { await state.rendering; } finally { state.rendering = null; }
  }

  // setLook 은 호스트가 고른 꼴로 바꾼다(확장 api — 머리 주석 「호스트 꼴 적용」). 그리는 중이면 draw 가 한 번 더 그린다. 이미 그 꼴이면 그리지 않는다 — 꼴 단추와 같다.
  function setLook(name) {
    state.chosen = name;
    state.forced = name === "doc" || name === "flow";
    return state.look !== name ? draw() : (state.rendering || Promise.resolve());
  }

  // fol 은 발표자 따라가기다(follower). 맞출 때 할 일은 followAction 이 정한다 — 발표 슬라이드는 보기 꼴이 호스트의 발표 꼴(want —
  // 슬라이드·보드)이 아니면 그 꼴로 바꿔 다시 그리고(render 가 끝에 맞춘다), 내용 위치는 보기 꼴을 그대로 둔 채 지금 꼴에서 맞춘다
  // (⑧ — 원래 슬라이드 문서는 슬라이드로. 보드 발표의 내용 위치는 예외로 보기 꼴을 보드로 바꿔 맞춘다). 발표가 끝나거나 따라가기를 끄면 슬라이드·보드 문서는 자기 꼴로 돌아간다(release — restoreOwnLook).
  const fol = follower({
    align: (i, kind, frac, want) => {
      if (state.ended) return;
      const act = followAction(kind, state.look, state.view, window.innerWidth, want);
      if (act === "redraw") { state.chosen = want === "dashboard" ? "dashboard" : "slides"; state.forced = false; draw(); return; }
      if (act === "scroll") scrollToFollow(i, kind, frac, true);
    },
    release: () => {
      for (const x of $("pages").querySelectorAll(".following")) x.classList.remove("following");
      if (state.ended) return;
      if (state.forced && !fol.follow) { // 호스트의 꼴을 거둔다 — 머리 주석 「호스트 꼴 적용」
        state.forced = false;
        if (lookSwitchHidden(state.view, "", false)) { state.chosen = ""; draw(); }
        return;
      }
      if (restoreOwnLook(state.view, state.chosen, window.innerWidth)) { state.chosen = ""; draw(); }
    },
    // 따라가기 상태가 바뀔 때마다(발표·읽기 시작·끝, 켬·끔, 풀림·복귀, hello) 쪽 | 이어보기 단추를 고친다 — 발표가 끝나면 보이고,
    // 보기 꼴 그대로 다음 발표·다시 켬이 오면 다시 숨긴다(내용 위치 따라가기는 숨기지 않는다)
    changed: () => paintLookSwitch(),
    button: label => {
      const b = $("follow-back");
      b.hidden = !label || state.ended;
      if (label) b.textContent = label;
    },
  });

  // jumper 는 #제목 이동(문서 안 링크·목차 항목)과 강조 고정이다(tocJumper).
  const jumper = tocJumper({ fol, find: findHeading, scroll: x => jumpScroll(x), closeDrawer: () => closeTocDrawer() });

  // paintLookSwitch 는 쪽 | 이어보기 단추의 숨김을 고친다(lookSwitchHidden). 공유가 끝났으면 늘 숨긴다.
  function paintLookSwitch() {
    $("look-switch").hidden = state.ended || !state.view || lookSwitchHidden(state.view, state.look, fol.holdsLook());
  }

  // showEnded 는 「공유가 끝났습니다」 화면이다. 이 화면만 끊겼으면(kicked) 「이 화면의 공유가 끝났습니다」.
  function showEnded(reason) {
    state.ended = true;
    linked.stop();
    ytStop();
    if (reason === "kicked") {
      $("ended-title").textContent = "이 화면의 공유가 끝났습니다";
      $("ended-msg").textContent = "공유한 사람이 이 화면의 연결을 끊었습니다.";
    }
    closeTableMenu();
    figs.reset();
    $("pages").textContent = $("notes").textContent = "";
    findHeading.refresh();
    $("stage").textContent = "";
    $("pages-box").style.width = $("pages-box").style.height = "";
    $("look-switch").hidden = true;
    $("notes-switch").hidden = true;
    npop.set(false);
    $("follow-back").hidden = true;
    // 목차: 예약된 강조 프레임을 거두고 링크·id·pin 을 비운 뒤 숨긴다 — 끝난 뒤 목차 콜백이 DOM 을 쓰지 않게
    if (tocFrame) { cancelAnimationFrame(tocFrame); tocFrame = 0; }
    tocIds = [];
    jumper.clear();
    $("side-toc").textContent = "";
    paintTocBulk(tocBulk, fold, true);
    state.files = null;
    mountFiles(document, $("sidebar"), null);
    paintTocPanel();
    $("load-err").hidden = true;
    hook("ended"); // 확장: 끝난 화면(자기 칸을 숨기고 끝난 카드의 자리를 채운다)
    $("ended").hidden = false;
    document.title = docTitle($("ended-title").textContent, app, source.tabTitle);
  }

  // pageTurn 은 쪽 넘김 키 한 번이다(⑨): 쪽(슬라이드)들의 자리를 scrollTop 좌표로 재어 pageNav 가 고른 자리로 굴린다.
  function pageTurn(dir) {
    const viewer = $("viewer"), vt = viewer.getBoundingClientRect().top;
    const pages = [...$("pages").querySelectorAll(".page")].map(pg => {
      const r = pg.getBoundingClientRect();
      return { top: r.top - vt + viewer.scrollTop, height: r.height };
    });
    viewer.scrollTop = window.ItdaPager.pageNav(dir, state.look === "dashboard" ? "flow" : state.look, viewer.scrollTop, viewer.clientHeight, pages, viewer.scrollHeight - viewer.clientHeight);
  }

  // jumpScroll 은 #제목 이동의 굴림이다: 슬라이드 보기면 그 제목이 든 슬라이드, 보드면 그 타일·구역 줄을 가운데로, 그 밖은 제목을 맨 위로.
  function jumpScroll(x) {
    const pg = state.look === "slides" ? x.closest(".page") : state.look === "dashboard" ? x.closest(".tile, .board-section") : null;
    if (pg) pg.scrollIntoView({ block: "center" });
    else x.scrollIntoView({ block: "start" });
  }

  // YouTube 블록(mdoc youtube.go — <a class="yt" data-yt-id …><img 대표 그림></a>). 재생은 호스트 부품 YT(window.ItdaYT, 옛 이름 HyveYT —
  // 머리 주석 「호스트 부품」)가 한다. 이 화면이 정하는 것은 셋이다: 누를 수 있는 자리(보기 영역 #viewer 안), 틀 주소
  // https://www.youtube-nocookie.com/embed/<ID>, referrerpolicy="strict-origin"(이 화면 주소의 경로 없이 출처만 간다 — 문서 Referrer-Policy
  // no-referrer 는 이 틀 요청만 덮인다). 누르기 전에는 YouTube 로 아무 요청도 가지 않는다(틀은 누를 때만 부품이 만든다). 노트 칸 안 블록은
  // 부품이 빼므로 보통 링크(새 탭)다. 되돌리는 때: 다시 그리기(render — 갱신·꼴 바꾸기·폭 변경·발표자 따라가기의 꼴 바꾸기)·끝난 화면
  // (showEnded)·따라간 슬라이드 번호가 바뀔 때(scrollToFollow — ytFollowGate), 화면 밖 이동(부품). 부품이 없으면(설치 없는 뷰어) 보통 링크다.
  const YT_EMBED = "https://www.youtube-nocookie.com/embed/";
  const YT = hostPart("YT");
  if (YT) {
    YT.init({
      inScope: a => $("viewer").contains(a),
      src: (id, start) => YT_EMBED + id + "?autoplay=1&rel=0&playsinline=1" + (start > 0 ? "&start=" + start : ""),
      referrerPolicy: "strict-origin",
      noSrc: () => false,
    });
  }
  // ytStop 은 함수 선언이다 — 위의 render 가 이 줄보다 먼저 불려도 쓸 수 있게 부품을 부를 때 찾는다.
  function ytStop() { const p = pick(window, "YT"); if (p) p.stop(); }

  // 문서 안 링크·목차 항목: #제목은 그 자리로, 바깥 http(s)·mailto·tel 은 새 탭(토큰 주소가 Referer 로 가지 않게 noreferrer), 폴더 안 문서는 그 문서로, 그 밖(문서 폴더의
  // 파일)은 열지 않는다 — 서버도 404 다. 어디로 가는지는 linkTo 가 정하고(그린 뒤 끄는 markLinks 와 같은 함수), 원천이 살려 두는 링크(source.liveLink — 묶은 교안의
  // 첨부·교안 링크)는 건드리지 않는다. 목차 서랍에서 누르면 이동 뒤 서랍을 닫는다. 끈 링크(markLinks — href 를 data-local-href 로 옮김)와 뷰어 원천이 옮겨 둔
  // <a data-local-href>(viewer.js localLinks)는 href 가 없어 열지 않는 쪽으로 가고 같은 까닭(linkMsg)을 알린다(itda-hyve#1096).
  function onClick(e) {
    const a = e.target.closest && e.target.closest("a[href], a[data-local-href]");
    if (!a) return;
    if (a.closest("#side-files")) return; // 파일 목록 — 브라우저가 ?doc= 주소로 옮긴다
    const inToc = $("sidebar").contains(a);
    if (!inToc && !$("viewer").contains(a)) return;
    if (source.liveLink && source.liveLink(a)) return; // 원천이 살려 두는 링크 — 브라우저 기본 동작
    const href = a.getAttribute("href") || "";
    const to = linkTo(href, state.view && state.view.doc, state.files && state.view ? state.files.docs : null);
    if (to === "hash") {
      e.preventDefault();
      jumper.jump(href, inToc); // 직접 옮긴 것 — 따라가기를 먼저 풀고, 옮겼으면 그 제목을 강조한다
      scheduleTocOn();
      return;
    }
    if (to === "out") {
      a.target = "_blank";
      a.rel = "noopener noreferrer";
      return;
    }
    e.preventDefault();
    if (to) { // 폴더 안 문서 링크
      if (to.doc !== state.view.doc) nav.open(to.doc, to.hash);
      else if (to.hash) { jumper.jump(to.hash, false); scheduleTocOn(); }
      return;
    }
    toast(source.linkMsg || LINK_OFF_MSG);
  }

  // linkOpts 는 지금 화면의 링크 끄기 기준이다(markLinks 의 o) — 지금 문서·폴더 목록(있을 때만)·원천이 살려 두는 링크·까닭 글. markAll 은 본문(#pages)과 노트 칸(#notes)에
  // 건다. 확장은 api.markLinks(root) 로 제 자리(hyve 노트 창)에 같은 기준을 건다.
  function linkOpts() {
    return { cur: state.view && state.view.doc, docs: state.files && state.view ? state.files.docs : null,
      live: source.liveLink ? a => !!source.liveLink(a) : null, msg: source.linkMsg || LINK_OFF_MSG };
  }
  function markAll() {
    const o = linkOpts();
    markLinks($("pages"), o);
    markLinks($("notes"), o);
  }
  // 끈 링크(tabindex 0 — markLinks) 위의 Enter 는 누르기로 바꾼다 — href 가 없는 a 는 브라우저가 Enter 로 click 을 내지 않는다. 누르기는 onClick(본문·노트 칸)이나
  // 확장의 누르기(hyve 노트 창)가 받아 같은 까닭을 알린다.
  document.addEventListener("keydown", e => {
    if (e.key !== "Enter" || e.isComposing || e.defaultPrevented || e.shiftKey || e.metaKey || e.ctrlKey || e.altKey) return; // ⌘/Ctrl+Shift+Enter(발표)는 그대로
    const a = e.target && e.target.closest ? e.target.closest("a.link-off") : null;
    if (!a) return;
    e.preventDefault();
    a.click();
  });

  for (const b of document.querySelectorAll("[data-notes]")) b.addEventListener("click", () => setNotesMode(b.dataset.notes));
  for (const b of document.querySelectorAll("[data-note-font]")) b.addEventListener("click", () => stepNoteFont(Number(b.dataset.noteFont)));
  applyNoteFont();
  for (const b of document.querySelectorAll("[data-look]")) {
    b.addEventListener("click", () => {
      state.chosen = b.dataset.look;
      state.forced = false; // 사람이 고른 꼴이다
      if (nav) nav.setLook(state.chosen); // 폴더 공유 — 다음 문서에도 이 꼴로
      if (state.look !== state.chosen) draw();
    });
  }
  document.addEventListener("click", onClick);
  // 보는 사람이 직접 움직이면 따라가기가 이 화면에서 잠시 풀린다 — 휠·터치·넘기는 키·굴림 막대(보기 영역 자체를 누름). 따라가기가 굴린
  // 것은 굴림(scroll) 이벤트라 여기 걸리지 않는다.
  // #제목 이동으로 고정한 강조(jumper.pin)도 이때 거둔다.
  $("viewer").addEventListener("wheel", () => { jumper.clear(); fol.moved(); }, { passive: true });
  // 노트 칸 손잡이(.sn-grip)를 터치로 끄는 것은 굴림이 아니다 — 풀지 않는다(bindNoteGrip)
  $("viewer").addEventListener("touchmove", e => {
    if (e.target && e.target.closest && e.target.closest(".sn-grip")) return;
    jumper.clear(); fol.moved();
  }, { passive: true });
  $("viewer").addEventListener("mousedown", e => { if (e.target === $("viewer")) { jumper.clear(); fol.moved(); } });
  bindNoteGrip();
  // 쪽 넘김 키(⑨ — keyHandler): 쪽·슬라이드·이어보기 규칙은 pager.js pageNav(문서 창 보기 모드와 같다)
  document.addEventListener("keydown", keyHandler({ pager: window.ItdaPager, fol, jumper, closeDrawer: () => closeTocDrawer(),
    ended: () => state.ended || !state.view, turn: dir => pageTurn(dir) }));
  $("follow-back").addEventListener("click", () => { jumper.resume(); scheduleTocOn(); });

  // ---- 표 복사(itda-hyve#349): 표 위 오른쪽 클릭에만 우리 메뉴(tablecopy.js — 쪽 나눔으로 잘린 표는 조각을 모아 전체). 표 밖·글자를 고른 상태는
  // 브라우저 기본 메뉴 그대로. 이 화면은 http(비보안 컨텍스트)라 navigator.clipboard 가 없다 — 누른 처리 안에서 바로 execCommand 로 쓴다.
  // 고른 표는 ctxTables(render 위) — 본문을 다시 그리면 render 가 메뉴를 닫고, 복사 직전에도 조각이 #pages 에 붙어 있는지 본다.
  // 키보드(itda-hyve#376): 표 안에 포커스(정렬 머리 칸 — Tab 으로 닿는다)가 있을 때 Shift+F10·ContextMenu 키로 그 칸 아래에 연다. 브라우저가 같은
  // 키로 contextmenu 를 이어 보내면(fromKey) 기본 메뉴만 막고 다시 열지 않는다.
  // openTableMenu 는 target 이 표 안이고 고른 글이 없으면 메뉴를 (x, y)에 열고 true 다. 아니면 메뉴를 닫고 false(브라우저 기본 메뉴).
  function openTableMenu(target, x, y, how) {
    if (!TableCopy) return false; // 호스트 부품이 없으면(뷰어) 브라우저 기본 메뉴
    const table = TableCopy.tableAt(target, $("pages-box")); // 본문과 노트 칸(#notes)의 표 모두
    const sel = window.getSelection && String(window.getSelection() || "");
    if (!table || (sel && sel.trim())) { closeTableMenu(); return false; }
    ctxTables = TableCopy.pieces(table, $("pages-box"));
    tmenu.open(x, y, how);
    return true;
  }
  $("pages-box").addEventListener("contextmenu", e => {
    if (tmenu.fromKey()) { e.preventDefault(); return; }
    if (openTableMenu(e.target, e.clientX, e.clientY, "mouse")) e.preventDefault();
  });
  $("pages-box").addEventListener("keydown", e => {
    if (!((e.key === "F10" && e.shiftKey) || e.key === "ContextMenu") || e.isComposing || e.metaKey || e.ctrlKey || e.altKey) return;
    const r = e.target.getBoundingClientRect();
    if (!openTableMenu(e.target, r.left, r.bottom, "key")) return;
    e.preventDefault();
    e.stopPropagation();
  });
  $("table-ctx").addEventListener("keydown", e => tmenu.key(e));
  const copyFromMenu = tableCopyClick({ tables: () => ctxTables, close: closeTableMenu, toast,
    copy: (kind, tables) => TableCopy.copy(kind, tables, $("pages-box")) });
  for (const b of document.querySelectorAll("[data-table-copy]")) b.addEventListener("click", () => { copyFromMenu(b.dataset.tableCopy); });
  // ---- 코드 블록 복사 단추(itda-hyve#413 — codecopy.js): 위임 클릭 하나를 문서(document)에 단다 — 본문을 다시 그려도 살아 있다.
  // 이 화면은 http(비보안 컨텍스트)라 navigator.clipboard 가 없다 — 누른 처리 안에서 바로 execCommand 로 쓴다(tablecopy.js write).
  if (CodeCopy) document.addEventListener("click", CodeCopy.handler({ toast }));

  if (Ask) document.addEventListener("click", Ask.handler());
  // ---- 표 열 정렬(itda-hyve#349 2단계 — tablesort.js): 머리 칸 왼쪽 클릭(키보드는 Enter·Space)으로 오름 → 내림 → 해제. 보는 사람 각자(이 화면의
  // 메모리에만 — 서버에 보내지 않는다). 슬라이드 보기에는 없다(표시를 달지 않는다). 다시 그린 뒤 누른 조각의 머리가 같은 높이에 남게 굴린다 —
  // 발표자를 따라가는 중이면 render 가 따라가는 자리로 굴리므로 굴리지 않는다(블록 번호 data-n 은 정렬과 상관없이 같다). 키보드로 눌렀으면
  // 따라가는 중에도 새 머리 칸에 포커스를 돌려준다(settle — preventScroll).
  async function sortByHead(th) {
    if (!TableSort || !TableCopy || state.ended || !state.view || state.look === "slides" || state.look === "dashboard" || state.rendering) return;
    const S = TableSort, pagesEl = $("pages"), viewer = $("viewer");
    const spec = S.specFor(th), table = th.closest("table");
    const k = Math.max(0, TableCopy.pieces(table, pagesEl).indexOf(table));
    const off = table.getBoundingClientRect().top - viewer.getBoundingClientRect().top;
    const focused = document.activeElement === th;
    await draw(spec.dir ? spec : null);
    if (state.ended) return;
    S.settle({ scope: pagesEl, viewer, spec, k, off, focused, scroll: !fol.active() });
  }
  $("pages").addEventListener("click", e => {
    if (e.button !== 0 || e.detail > 1 || e.shiftKey || e.metaKey || e.ctrlKey || e.altKey) return;
    const th = TableSort && TableSort.headAt(e.target, $("pages"));
    if (!th) return;
    const sel = window.getSelection && window.getSelection();
    if (sel && !sel.isCollapsed && String(sel).trim()) return;
    sortByHead(th);
  });
  $("pages").addEventListener("keydown", e => {
    if ((e.key !== "Enter" && e.key !== " ") || e.isComposing || e.shiftKey || e.metaKey || e.ctrlKey || e.altKey) return;
    const th = e.target.closest && e.target.closest("th.sortable");
    if (!th || th !== e.target) return;
    e.preventDefault();
    e.stopPropagation(); // 쪽 넘김 키(Space)가 아니다
    sortByHead(th);
  });
  document.addEventListener("mousedown", e => { if (!e.target.closest || !e.target.closest("#table-ctx")) closeTableMenu(); });
  document.addEventListener("keydown", e => { if (e.key === "Escape") closeTableMenu(); });
  $("viewer").addEventListener("scroll", closeTableMenu, { passive: true });
  window.addEventListener("resize", closeTableMenu);
  // 넓은 화면에서 펼치고 접으면 보기 영역 폭이 바뀌어 배율이 다시 걸린다 — 읽던 자리(굴린 비율)를 크기 변경 뒤에 되돌린다(tocKeep)
  $("btn-toc").addEventListener("click", () => {
    const v = $("viewer");
    tocKeep = toc.wide && v.scrollHeight > 0 ? v.scrollTop / v.scrollHeight : -1;
    // 닫혀 있는 동안 바뀐 지금 항목은 굴리지 못했다 — 열 때 한 번 맞춘다(사람이 연 것이라 목차를 굴려 보던 중이 아니다)
    if (toc.toggle()) revealer.open();
    paintTocPanel();
  });
  $("toc-back").addEventListener("click", () => closeTocDrawer());
  // 접기 단추는 접거나 펼 뿐이다 — 옮기지 않고, 서랍을 닫지 않고, 따라가기를 풀지 않는다(itda-hyve#985)
  // 사람이 목차 상자(사이드바 — 목차 상자 #side-toc 은 그 안이라 이벤트가 올라온다)를 굴리거나 만지면 남은 굴림 예약을 버린다(tocRevealer.cancel)
  for (const t of TOC_TOUCH) $("sidebar").addEventListener(t, () => revealer.cancel(), { passive: true, capture: true });
  $("side-toc").addEventListener("click", e => {
    const b = e.target.closest && e.target.closest("button.toc-tg");
    if (!b) return;
    e.preventDefault();
    if (fold.toggle(b.dataset.fold)) foldTo();
  });
  if (tocBulk) tocBulk.addEventListener("click", () => {
    const next = fold.bulk();
    if (!next) return;
    fold.all(next === "expand");
    foldTo();
  });
  $("viewer").addEventListener("scroll", scheduleTocOn, { passive: true });
  // 목차의 펼침 기본값은 화면(본문) 폭으로 고른다 — 경계(1024px)를 넘나들 때만 기본값으로 되돌린다(tocPanel)
  toc.width(document.body.clientWidth);
  paintTocPanel();
  // 확장(itda-hyve#617)을 시작한다 — 목차 폭을 정한 뒤·첫 그리기(linked.start — 공유 화면은 sync.refresh) 전. 넘기는 것은 이 화면의 부품 그대로다(같은 객체 — 확장이
  // 읽는 state·toc 은 늘 지금 값). 확장이 던지면 확장 없이 계속한다.
  const extInit = pick(window, "ShareExt");
  if (typeof extInit === "function") {
    try {
      ext = extInit({ $, base, state, toc, fol, jumper, findHeading, headingNow, slideAtMiddle, placeSlide, jumpScroll, scheduleTocOn, toast, setLook,
        markLinks: root => markLinks(root, linkOpts()) }) || {};
    } catch (e) { ext = {}; console.error("share ext:", e); }
  }
  new ResizeObserver(() => {
    const was = toc.wide;
    toc.width(document.body.clientWidth);
    if (toc.wide !== was) { paintTocPanel(); hook("wide"); }
  }).observe(document.body);
  let lastWide = window.innerWidth > NARROW;
  new ResizeObserver(() => {
    const wide = window.innerWidth > NARROW;
    // 고르지 않았으면 좁음·넓음이 바뀔 때 꼴도 바꾼다(폰을 가로로 돌림 등)
    if (wide !== lastWide && !state.chosen && state.view) { lastWide = wide; draw(); return; }
    lastWide = wide;
    afterResize(fol, applyZoom, scrollToFollow); // 따라가는 중이면 새 배율의 자리로 다시 맞춘다(목차를 펼치고 접어도 여기로 온다)
    hook("resized"); // 확장: 이어보기는 폭이 바뀌면 글이 다시 흐른다
    if (tocKeep >= 0 && !fol.active()) $("viewer").scrollTop = tocKeep * $("viewer").scrollHeight;
    tocKeep = -1;
    scheduleTocOn();
  }).observe($("viewer"));

  // 원천을 시작한다 — 공유 화면은 갱신 스트림을 열고 첫 view.json 을 받는다(hostSource start)
  linked.start();
})();
