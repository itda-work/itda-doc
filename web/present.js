// 발표 — 슬라이드 미리보기(slides.js 가 나눈 쪽)를 전체 화면에서 넘긴다. 전역은 window.ItdaPresent 다.
// 검은 바탕에 슬라이드를 화면에 맞춘다(비율 유지). 움직임은 CSS(present.css)이고 여기서는 클래스만 바꾼다.
//
// 호스트 계약 — 창 틀은 직접 부르지 않고 호스트가 주는 훅으로만 만난다. 훅은 부를 때마다 window.ItdaPresentHost 에서 찾고,
// 없거나 그 함수가 없으면 그 동작만 건너뛴다(발표는 그대로 돈다). 훅이 던지는 예외는 삼킨다.
//   ItdaPresentHost.fullscreen(on)  발표를 열 때 true, 끝낼 때 false 로 부른다 — 창 자체를 전체 화면으로 바꾸는 일(웹뷰 안 JS 전체 화면 API 는
//                                   믿을 수 없는 웹뷰가 있다). state.fsCalls 는 발표를 열 때(start·startBoard)의 전체 화면 진입
//                                   요청 횟수다 — 훅이 없어도 오르고, 끝낼 때의 false 요청은 세지 않는다.
//   ItdaPresentHost.ownLink(href)   참이면 그 href 는 호스트 전용 앵커라 발표가 링크로 다루지 않는다(누르기는 넘김·크게 보기 그대로).
//   ItdaPresentHost.ownsKey(e)      발표의 keydown(capture — onKey) 맨 앞에서 부른다. 참이면 발표는 그 키에 아무것도 하지 않는다(넘김·판서 키·
//                                   preventDefault·stopPropagation 모두 — 키는 호스트 몫이다). 겹침 층을 가린 동안(아래)에도 늘 먼저 부른다.
//                                   가려진 층의 키를 양보할지는 호스트 몫이다 — 호스트는 훅 안에서 body 의 pr-overlay-hidden 을 읽어 정할 수 있다.
//                                   가린 동안 참을 돌려주면 그 키로 검은 화면·한눈에 보기·크게 보기를 닫지 못하므로, 가린 동안은 거짓을 권한다.
// 겹침 층 — 호스트가 발표 위에 띄우는 자기 요소(예: 내장 터미널). body 의 자식에 OVERLAY 속성(data-pr-overlay)을 달면:
//   · body.presenting 의 숨김에서 빠지고 z-index 101(#present 는 100)로 보인다(present.css — 자리 잡기 position 은 호스트가 준다).
//   · 그 안의 mousedown·click·auxclick·pointer 는 넘김·판서·링크·크게 보기로 다루지 않는다(onMouse·eatClick·ink.js BLOCKED). #present 의
//     커서 숨김(pr-idle)은 층에 닿지 않는다(body 의 자식이면 #present 밖이고, 안에 있어도 present.css 가 커서를 되살린다).
//   · 검은 화면·한눈에 보기·크게 보기 동안 body 에 OVERLAY_HIDDEN 클래스(pr-overlay-hidden)를 달아 층과 그 자손을 모두 가리고(visibility —
//     display 는 바꾸지 않아 층 안의 크기 재기가 그대로다), 끝나면 뗀다. 호스트는 이 클래스로 가림을 안다(syncOverlay). 자손의 인라인
//     visibility !important 는 이 가림을 이긴다(CSS 우선순위) — 층 안에서는 쓰지 않는다. 발표가 끝나면 클래스는 떼고 층 요소는 지우지 않는다.
// 두 화면 발표(itda-doc#95) — 발표자 화면과 청중 화면을 따로 띄우는 호스트를 위한 세 가지(슬라이드 발표 start 에만, 보드 발표에는 없다):
//   o.onState(s)        보이는 상태 s = { i, step, steps, total, next, black, grid, lightbox, on } 가 바뀔 때마다 한 번 부른다(emitState) — 장·단계·
//                       검은 화면·한눈에 보기·크게 보기·열림. 직전에 보낸 값과 같으면 부르지 않는다. 열 때 첫 상태를 한 번, 끝낼 때 on:false 를 한 번
//                       보낸다(끝의 black·grid·lightbox 는 false). lightbox 는 크게 보기가 열렸는가(참·거짓)다. onSlide 는 그대로다.
//                       next 는 다음 장 번호(i+1 — 지금 장이 마지막이면 -1)다. 단계가 남아도 다음 「장」 이다(아래 「발표자 보조」 의 미리보기와 같은 장 —
//                       호스트가 그 장의 그림을 뜰 때 쓴다). i 에서 정해지므로 next 만 바뀌어 따로 알리는 일은 없다.
//   o.audience = true   청중(따라가기) 열기 — 덱·전환·fit·검은 화면 층만 만든다. 노트·목차·아래 줄(HUD)·번호 이동·한눈에 보기·크게 보기·판서를 만들지
//                       않고, 키·마우스·커서 깨우기 리스너를 걸지 않는다(커서는 늘 숨긴다 — present.css .pr-audience). 덱 사본에 딸려 온 단추·링크·입력
//                       (섹션 묻기 단추·코드 복사 단추 등)은 입력이 아니다: 덱에 inert 를 달고(WebKit 동작은 미검증), 포커스 차례에서 빼며(tabindex -1),
//                       덮개 안의 click·auxclick 은 창 붙잡기 단계에서 모두 먹는다(eatClick — 문서의 위임 처리기까지 가지 않는다). 섹션 묻기 단추·코드 복사
//                       단추는 보이지 않는다(present.css). 호스트 훅(fullscreen 등)은 그대로 부른다. 넘김은 ItdaPresent.show 로만 한다.
//                       판서 입력·도구 막대는 없지만 발표자의 판서를 받아 그린다(아래 「판서 받기」).
//   ItdaPresent.show({ i, step, black })  보이는 상태를 덮어쓴다(멱등). 범위 밖 값은 잘라낸다(i 는 0~장 수−1, step 은 0~그 장의 단계 수). 없는 키는
//                       그대로 둔다(step 이 없으면 같은 장이면 지금 단계, 다른 장이면 0). 같은 장·같은 단계·같은 검은 화면이면 아무것도 하지 않는다(차트도
//                       다시 돌지 않는다). 장이 바뀌면 go(i, step, dir) 규칙(dir 은 장 번호 차례 — 차트는 앞으로 갈 때만 움직인다), 같은 장의 단계만 바뀌면
//                       next·prev 와 같다. 검은 화면은 setBlack 과 같은 길이다. 바뀐 것이 여럿이어도 onState 는 한 번이다.
// 판서 받기(itda-doc#108) — 발표자 화면의 판서·레이저를 청중 화면에 같은 모양으로 그린다. 둘 사이 길(창·기기 사이 전달)은 호스트 몫이다.
//   발표자 표면: start 에 inkKey 를 주고, 청중 화면이 붙으면 같은 키로 ItdaInk.relay(inkKey, send) 를 부른다. send =
//     · ink(json)            묶음 JSON 문자열 한 줄({"k", "ops"} — ink.js 「중계」). 그대로 청중 표면에 넘기고 ""(받음)·"full"(다음에 전체 상태를 다시
//                            보내라)·"stopped"(그만 보낸다) 또는 그 Promise 를 돌려준다. relay 는 앞 묶음의 값이 오기 전에는 다음 묶음을 보내지 않는다
//                            (그사이 연산은 다음 묶음에 합쳐진다) — 순서를 지키는 길이면 넘기자마자 "" 를 돌려줘도 된다. 던지거나 거부하면 그 묶음이 빠진
//                            것으로 보고 다음에 전체 상태(reset)를 보낸다(reset 묶음까지 거부되면 멈추고 stopped 를 부른다).
//     · laser(i, x, y, line) 레이저의 마지막 값(장·설계 px·끈 선인가) — 꺼지면 i = -1. 그대로 청중 표면에 넘긴다(잃어도 다음 값이 덮는다).
//     · stopped()            (없어도 된다) relay 가 전달을 멈췄다.
//     잇자마자 지금 상태 전체를 reset 묶음으로 보낸다. 돌려받은 close() 로 끊는다. 청중 화면이 새로 붙거나 끊겼다 다시 붙거나, 청중 쪽 apply 가 true 가
//     아닌 값을 돌려주면 relay 를 다시 부른다(앞 길은 저절로 닫히고 새 reset 이 빠진 연산을 메운다).
//   청중 표면: ItdaPresent.inkView() 가 받는 쪽(ink.js view)을 준다(ink.js 가 없으면 null). 받은 것을 그대로 넘긴다 —
//     inkView().apply(json)  묶음(문자열이나 객체) · inkView().laser(i, x, y, line) 레이저 · inkView().reset() 판서를 모두 비운다(다른 문서·발표로 바꿀 때).
//     호스트가 순서와 한 번 전달을 보장한다 — 받는 쪽은 재전송에 멱등이 아니다(전송 꼴에 순번이 없다). 묶음은 relay 가 낸 차례대로 한 번씩 넘기고
//     같은 묶음을 다시 넘기지 않는다(점 추가 a 가 다시 오면 점이 겹치고 apply 는 알아채지 못한다). 빠졌거나 차례가
//     흐트러졌으면 재전송 대신 발표자 쪽 relay 를 다시 불러 reset 을 받는다. apply 가 true 가 아닌 값(false 읽지 못함·"reset" 받는 쪽 상한·모르는 id —
//     ink.js 「다시 맞춤 신호」)을 돌려줘도 같다: 발표자 쪽에 알려 relay 를 다시 부른다. "reset" 은 reset 묶음이 올 때까지 이어진다.
//     받는 쪽은 발표 밖에서도 받는다: 청중 열기(audience) 동안 덱의 장 위에 그리고, 열기 전·닫은 뒤에 온 판서도 모델에 남아 다시 열면 그린다.
//     장 번호는 발표자와 같은 덱 차례다(두 표면이 같은 문서를 연다고 본다). 장 넘김은 show 가 하고, 넘긴 장의 판서는 그 장에 남아 돌아오면 다시 보인다
//     (발표자와 같다). 레이저는 청중 화면의 지금 장이 레이저의 장이고 검은 화면이 아닐 때만 보인다. 그리기는 프레임(rAF)마다 한 번이다.
// 펜만 판서(itda-doc#100) — o.penOnly = true(슬라이드 발표 start 에만, 청중 열기·보드 발표에는 없다):
//   · 판서(펜·형광펜·지우개·레이저)는 pointerType "pen" 입력만 받는다(ink.js notPen) — 손가락·마우스는 획·지우개·레이저를 만들지 않는다.
//   · 판서 도구가 켜진 동안 펜이 아닌 누르기(손가락·마우스, 그 호환 mousedown·click)는 장을 넘기지 않고 먹는다 — 링크·크게 보기도 아니다(펜·형광펜·
//     지우개가 켜진 동안의 기존 누르기와 같다). 호환 mousedown 에는 pointerType 이 없어 출처를 가리지 않는다 — 도구가 켜져 있으면 먹는다(ink.js takes).
//     그래서 손바닥이 닿거나 펜을 먼저 떼도 획이 끊기거나 장이 넘어가지 않는다. 도구를 끄면 지금처럼 누르기 = 다음이다. 쉬는 화면(검은 화면)의 누르기는 지금처럼 다음이다.
//     도구가 켜진 동안의 장 넘김은 키·호스트의 밀기·단추 몫이다.
//   · 덮개에 pr-pen-only 클래스를 단다(호스트 CSS 가 쓸 수 있다 — present.css 에는 이 클래스의 규칙이 없다).
//   · 손가락 밀기(좌우로 넘기기)는 발표가 판정하지 않는다 — 옵션이 없을 때와 같이 호스트 몫이다. 호스트는 ItdaPresent.inkHeld() 가 참인 동안
//     (펜 입력이 진행 중) 밀기를 버릴 수 있다.
//   · 옵션이 없으면 지금 그대로다(마우스·손가락도 그린다). Pencil 이 없는 기기에서 판서 도구를 숨길지는 호스트 몫이다.
//   ItdaPresent.setPenOnly(b)  발표 중에 펜만 판서를 켜고 끈다(호스트의 「손가락으로도 그리기」 토글 같은 것) — 발표를 다시 열지 않으므로 장·단계·판서·
//     도구는 그대로다. P.penOnly·pr-pen-only 클래스·판서(ink.js)의 판정이 함께 바뀐다. 진행 중인 입력(쥔 획·지우개·레이저)은 바뀌기 전 판정으로
//     끝까지 가고, 바뀐 판정은 다음 누르기부터다. 슬라이드 발표 중에만 듣는다 — 발표 밖·보드 발표·청중 열기에는 아무것도 하지 않고 false 를
//     돌려준다(적용하면 true). 다음 발표는 물려받지 않는다(start 의 o.penOnly 가 정한다). onState 의 보이는 상태에는 넣지 않는다 — 청중 화면이 쓰는
//     값이 아니고, 바꾼 것은 부른 호스트가 안다(P.penOnly 로도 읽는다).
//   ItdaPresent.inkHeld()  판서 입력(포인터를 쥔 획·지우개·레이저 누르기)이 진행 중인가 — penOnly 면 펜 입력뿐이다. 발표 밖·판서가 없으면 거짓.
// 발표자 보조 — 두 화면 발표의 발표자 화면을 위한 두 가지(슬라이드 발표 start 에만, 청중 열기·보드 발표에는 없다). 둘 다 기본 꺼짐이다.
//   o.nextPreview = true · ItdaPresent.setNextPreview(b)  다음 장 미리보기 — 노트 칸 머리 아래·노트 위의 「다음」 칸에 다음 장(i+1)의 축소 사본을 둔다.
//                       단계가 남았어도 다음 「단계」 가 아니라 다음 장이고, 사본은 다 나타난 모습이다(한눈에 보기 사본과 같다). 마지막 장이면 「끝」 이다.
//                       노트 칸 안이라 노트를 접으면 함께 숨는다. 사본은 하나만 두고 다음 장 번호가 바뀔 때만 넘김 안에서 곧바로 바꾼다(빠른 연달아 넘김에도
//                       늦게 따라오지 않는다 — 단계만 바뀌면 그대로). 사본은 판서 층·섹션 묻기 단추·코드 복사 단추를 떼고, id 를 지우며, 칸에 aria-hidden·inert 를
//                       달고 그 안의 포커스 가능한 것을 포커스 차례에서 뺀다(tabindex -1). 차트 재생 클래스를 떼고 움직임을 끈다(present.css .pr-thumb).
//                       판서는 넣지 않는다. 노트 칸 너비가 바뀌면 칸 너비에 맞춰 다시 줄인다(relayout). 그림·표 번호(CSS 카운터 fig·tbl)는 상자에 독립
//                       범위를 두고 덱에서 다음 장 앞까지의 캡션 수로 시작해 덱의 번호와 같게 한다(NEXT_COUNTERS — 덱·원문 카운터는 건드리지 않는다).
//                       캡션은 브라우저가 실제로 배치한 것만 센다(display: none 은 빠지고 visibility: hidden 은 남는다 — laidOut).
//   o.clock = true · ItdaPresent.setClock(b)  아래 줄(HUD)의 경과 시간 옆에 현재 시각(HH:mm — 기기 시각, 24시간)을 보인다(.pr-clock — present.css 는
//                       #present.pr-clock-on 일 때만 보인다). 경과 시간과 같은 타이머(tick)로 고친다.
//   setNextPreview·setClock 은 발표를 다시 열지 않고 바꾼다 — 슬라이드 발표 중에만 듣고(발표 밖·보드 발표·청중 열기면 false), 적용하면 true. 다음 발표는
//   물려받지 않는다(start 의 o 가 정한다). P.nextPreview·P.clock 으로 읽는다.
// 있으면 쓰고 없으면 건너뛰는 부품(전역): window.ItdaLightbox(크게 보기 — create·SEL), window.ItdaCodeCopy(노트 코드 복사 단추 — arm),
// window.ItdaInk(판서 — ink.js). 늘 있어야 하는 것: window.ItdaSlides(slides.js — 쪽 크기·노트 글자 단계).
// 조작: → Space PageDown 클릭 = 다음(차례로 나타낼 것이 남았으면 그것부터), ← PageUp Backspace = 이전(이전 슬라이드는 다 나타난 모습),
//       숫자 + Enter = 그 슬라이드, Home·End = 처음·끝, G = 한눈에 보기, B·. = 검은 화면, N = 발표자 노트, Esc = 끝,
//       노트를 편 동안 + = 노트 글자 크게, − = 작게(머리의 A−/A+ 와 같다), D = 노트 붙이기·띄우기(머리 단추와 같다),
//       T = 목차 칸, 목차를 편 동안 Shift+D = 목차 붙이기·띄우기(머리 단추와 같다),
//       슬라이드 안 이미지·표·그림(차트·다이어그램 SVG) 클릭 = 크게 보기(라이트박스 — 열린 동안 클릭·Esc·Space·Enter 는 닫기만, 다른 키는 삼킨다),
//       지금 슬라이드의 외부 링크·# 링크 왼쪽 클릭 = 링크 동작(넘김·크게 보기보다 먼저다, 동작은 부른 쪽 onLink)
// 노트는 기본으로 슬라이드 위에 떠 있는 카드다(청중과 함께 보므로 슬라이드는 제자리·제 크기, fit 결과가 노트 켜기 전과 같다).
// 카드 머리(단추 밖)를 끌면 카드가 옮겨진다(화면 안, 아래 줄 위까지 가둔다 — 위치는 이 발표 동안만). 머리의 「붙이기」 단추나 D 키(노트를 편
// 동안)로 오른쪽에 붙인 칸(도킹 — 슬라이드가 남은 자리로 줄어든다)으로, 「띄우기」·D 로 다시 카드로 바꾼다.
// 노트의 왼쪽 경계(.pr-notes-grip — 카드면 카드를 따라다닌다)를 끌면 너비가 바뀐다(창의 22~60%, 두 번 누르면 34%).
// 너비·글자 크기·도킹은 부른 쪽이 기억한다.
// 목차 칸(T)은 노트 카드와 같은 규칙의 왼쪽 칸이다: 기본은 왼쪽 위 떠 있는 카드(슬라이드 제자리·제 크기), 머리 끌기로 옮기고
// 오른쪽 경계 끌기로 너비(창의 16~40%, 두 번 누르면 22%), 머리 단추·Shift+D 로 왼쪽에 붙인 칸(무대 왼쪽이 칸만큼 준다). 항목은
// 구역 → 슬라이드 2단(넘쳐 생긴 「(계속)」 장은 한 항목), 누르면 그 슬라이드로 간다. 모드·너비는 부른 쪽이 기억한다(켬은 기억하지 않는다).
// 노트가 있는 슬라이드는 목차 항목·진행 막대 눈금·한눈에 보기 썸네일에 점이 붙고, 노트 칸 머리에 「노트 k/n」 이 보인다(노트가 하나도
// 없는 문서면 그리지 않는다).
// 마우스를 가만히 두면 2초 뒤 숨고, 아래 진행 막대·번호·경과 시간도 함께 숨는다(움직이면 다시 보인다).
// 판서(ink.js): L 레이저, P 펜, H 형광펜, E 지우개, 다시 누르면 끈다. 켠 동안 X = 이 장 전부 지우기, C·[·] = 색, ⌘Z·⇧⌘Z(Windows 는 Ctrl) = 되돌리기·다시 실행.
// Esc 는 그리는 중 → 취소, 도구 켜짐 → 도구 끔, 그다음에야 발표 끝. 펜·형광펜·지우개가 켜진 동안 왼쪽 클릭은 넘김·크게 보기가 아니고, 레이저가 켜진 동안
// 넘김은 떼는 때로 미룬다(움직이지 않았으면 클릭, 끌었으면 사라지는 선). 도구가 켜진 동안 커서를 숨기지 않고, 라이트박스·한눈에 보기·검은 화면에서는 쉰다 —
// 누른 채 그 화면으로 들어가면 획·지우개는 거기까지 확정하고 레이저 누르기는 거둔다(ink.js settle).
(function () {
  "use strict";

  const P = { on: false, idx: 0, step: 0, slides: [], notes: [], started: 0, latency: [], switchMs: [], fsCalls: 0, grid: false, black: false, showNotes: false,
    entered: 0, transition: "none" };
  let shareEl = null, shareSummary = ""; // 공유 미션 숫자 요약(setShareSummary)
  let root, deck, gridEl, notesBody, numEl, timeEl, barEl, gotoEl, fsVal, timer = 0, idleT = 0, digits = "", onExit = null, onSlide = null, sentIdx = -1;
  let opts = {}, notesEl = null, dockBtn = null, cardRO = null, dragEnd = null, lb = null, lbEl = null, lbEat = false, linkDown = null;
  let tocEl = null, tocBody = null, tocDockBtn = null, tocItems = [];
  // 발표자 보조(머리 주석) — 다음 장 미리보기 칸·축소 상자·지금 담은 장 번호(-1 끝, -2 비었음)·현재 시각 칸
  let nextEl = null, nextBox = null, nextAt = -2, clockEl = null;
  let ink = null; // 판서(ink.js) — 슬라이드 발표에만 붙는다. 보드 발표에는 없다
  // 받아 그리는 판서(ink.js view — itda-doc#108). 청중 열기에만 붙인다(inkMounted). 모델은 발표를 닫아도 남는다 — 청중 화면을 다시 열면 그대로 그린다
  let inkView = null, inkMounted = false;
  function inkViewOf() {
    if (!inkView && window.ItdaInk && window.ItdaInk.view) inkView = window.ItdaInk.view();
    return inkView;
  }
  let onState = null, sentState = "", hold = 0; // 두 화면 발표의 상태 알림(emitState) — 마지막으로 보낸 값(JSON)·묶음 깊이(batch)
  const NOTE_W_DEF = 34, NOTE_W_MIN = 22, NOTE_W_MAX = 60, NOTE_FS_DEF = 17;
  const TOC_W_DEF = 22, TOC_W_MIN = 16, TOC_W_MAX = 40; // 목차 칸 너비(창의 % — 너비를 기억하는 호스트의 범위와 같게 둔다)
  const CARD_M = 24; // 떠 있는 카드의 기본 자리 — 창 위·오른쪽에서 띄우는 px(present.css 의 max-height 여백과 같다)
  const HUD_H = 40;  // 아래 줄(.pr-hud — 진행 막대·번호·시간) 높이(present.css) — 카드는 그 위까지만 옮긴다
  // 라이트박스(그리기·크기 규칙은 호스트의 크게 보기 부품 ItdaLightbox): 크게 볼 수 있는 것은 슬라이드 본문(.page-body)의 ItdaLightbox.SEL
  // (이미지·표·차트·다이어그램 SVG). 머리·꼬리의 로고(.sc-logo·.sf-logo, slides.js)는 아니다. 호스트의 cursor: zoom-in 규칙과 같은 대상이다
  const OVERLAY = "[data-pr-overlay]", OVERLAY_HIDDEN = "pr-overlay-hidden"; // 겹침 층 표지·가림 클래스(머리 주석 「호스트 계약」)
  const inOverlay = t => !!(t && t.closest && t.closest(OVERLAY));
  // 포커스를 받을 수 있는 것 — 청중 열기에서 덱 사본의 이것들을 포커스 차례에서 뺀다(startAudience)
  const FOCUSABLE = "a[href], area[href], button, input, select, textarea, summary, iframe, audio, video, [tabindex], [contenteditable]";
  const reduce = () => window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  // 장 넘김 전환(present.css .t-*). 모르는 값은 none 이다 — mdoc 이 이미 걸러 주지만 호스트가 다른 값을 넘겨도 클래스가 새지 않게.
  // OUT_MS 는 나가는 장(pr-out)을 보이게 두는 시간 — present.css 의 나가는 움직임(지연 포함)은 이 안에 끝난다(present_test.go 가 묶는다)
  const TRANSITIONS = ["none", "fade", "slide", "zoom", "push", "cover"], OUT_MS = 450;
  const stepsOf = i => Number(P.slides[i].dataset.steps || 0);

  // hostCall 은 호스트 훅 window.ItdaPresentHost[name] 을 부른다(머리 주석 「호스트 계약」). 훅이 없으면 undefined, 훅의 예외는 삼킨다.
  function hostCall(name, ...args) {
    const h = window.ItdaPresentHost;
    if (!h || typeof h[name] !== "function") return undefined;
    try { return h[name](...args); } catch (e) { return undefined; }
  }

  // syncOverlay 는 겹침 층 가림 클래스를 지금 상태(검은 화면·한눈에 보기·크게 보기)에 맞춘다. 발표가 끝났으면 뗀다.
  function overlayHidden() { return !!(P.on && (P.black || P.grid || P.lightbox)); }
  function syncOverlay() { document.body.classList.toggle(OVERLAY_HIDDEN, overlayHidden()); }

  // stateOf 는 보이는 상태(머리 주석 「두 화면 발표」 onState)다.
  function stateOf() {
    const i = P.idx;
    return { i, step: P.step, steps: P.slides[i] ? stepsOf(i) : 0, total: P.slides.length, next: nextIndex(), black: !!P.black, grid: !!P.grid, lightbox: !!P.lightbox, on: !!P.on };
  }
  // nextIndex 는 다음 장 번호다 — 마지막 장이면 -1(머리 주석 「두 화면 발표」 onState 의 next).
  function nextIndex() { return P.idx + 1 < P.slides.length ? P.idx + 1 : -1; }
  // emitState 는 보이는 상태가 지난번 보낸 것과 다르면 onState 로 보낸다. 묶음(batch) 안·첫 장을 보이기 전·보드 발표에는 보내지 않는다.
  function emitState() {
    if (!onState || hold || !P.on || P.board || !P.entered) return;
    const s = stateOf(), j = JSON.stringify(s);
    if (j === sentState) return;
    sentState = j;
    onState(s);
  }
  // batch 는 f 안의 상태 바뀜을 모아 끝에 한 번만 알린다(go 안의 크게 보기 닫기, show 의 장·검은 화면 함께 바꾸기).
  function batch(f) {
    hold++;
    try { f(); } finally { hold--; }
    emitState();
  }

  function el(tag, cls, text) {
    const e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  }

  // start 는 발표를 연다. o = { pages(미리보기의 .page 들), notes(슬라이드마다 노트 HTML), index, transition, ratio, onExit, onSlide,
  //   noteW(N 칸 너비 % — 없으면 34), noteFont()(지금 노트 글자 px — 0 이면 기본 17), onNoteW(pct)(끌기를 놓을 때), onNoteFont(px)(A−/A+·+/−),
  //   noteDock(참이면 오른쪽에 붙인 칸 — 없거나 거짓이면 떠 있는 카드), onNoteDock(dock)(머리 단추로 바꿀 때),
  //   tocDock(참이면 목차를 왼쪽에 붙인 칸 — 없거나 거짓이면 떠 있는 카드), tocW(목차 칸 너비 % — 없으면 22),
  //   onToc(dock, w)(목차 붙이기·띄우기를 바꿀 때, 너비 끌기를 놓을 때), inkKey(판서가 딸린 탭 id — ink.js bookFor),
  //   onLink(href)(지금 슬라이드의 외부 링크·# 링크를 눌렀을 때), onState(s)·audience(머리 주석 「두 화면 발표」), penOnly(머리 주석 「펜만 판서」),
  //   nextPreview·clock(머리 주석 「발표자 보조」) }
  // onSlide(i, total) 는 슬라이드 번호가 바뀔 때마다(첫 슬라이드 포함 — 단계만 바뀌면 부르지 않는다), 끝낼 때 onExit 뒤 (-1, total) 로 부른다.
  function start(o) {
    if (P.on) return;
    P.on = true;
    opts = o;
    P.audience = !!o.audience;
    P.penOnly = !!o.penOnly && !P.audience; // 청중 열기에는 판서가 없다
    P.nextPreview = !!o.nextPreview && !P.audience; // 발표자 보조는 발표자 화면에만(머리 주석)
    P.clock = !!o.clock && !P.audience;
    nextAt = -2;
    capCache = []; // 미리보기 번호용 장별 캡션 수(capsOf) — 새 덱이다
    onState = typeof o.onState === "function" ? o.onState : null;
    sentState = "";
    hold = 0;
    P.slides = [];
    P.entered = 0; // 지난 발표와 같은 번호에서 다시 열어도 첫 슬라이드를 보인다 — 남은 값이면 go 가 「같은 슬라이드」 로 보고 pr-cur 를 달지 않는다
    // 지난 발표가 남긴 화면 상태(한눈에 보기·검은 화면·노트)는 새 발표가 물려받지 않는다 — root 는 새로 만들지만 P 의 깃발은 남는다
    P.grid = false;
    P.black = false;
    P.showNotes = false;
    P.noteDock = !!o.noteDock;
    P.card = null; // 끌어 옮긴 카드 자리 {x, y}(창 기준 px) — null 이면 기본 자리(위·오른쪽 CARD_M). 다음 발표는 물려받지 않는다
    P.lightbox = ""; // 열린 라이트박스의 종류("img"·"table"·"svg" — 닫혔으면 ""). 다음 발표는 물려받지 않는다
    P.lbZoom = 0;
    P.showToc = false; // 목차 칸 켬은 기억하지 않는다(노트 N 과 같다)
    P.tocDock = !!o.tocDock;
    P.tocW = TOC_W_DEF; // 노트 너비를 먼저 놓을 때(applyWidths) 목차 값이 비지 않게 — 아래 setTocW 가 기억한 값으로 바꾼다
    P.tocCard = null; // 끌어 옮긴 목차 카드 자리 — null 이면 기본 자리(위·왼쪽 CARD_M)
    P.notes = o.notes || [];
    P.transition = reduce() || !TRANSITIONS.includes(o.transition) ? "none" : o.transition;
    onExit = o.onExit;
    onSlide = o.onSlide || null;
    sentIdx = -1;
    root = el("div", "present t-" + P.transition + (P.noteDock ? " pr-dock" : "") + (P.tocDock ? " pr-toc-dock" : "") + (P.audience ? " pr-audience" : "") + (P.penOnly ? " pr-pen-only" : "") + (P.nextPreview ? " pr-next-on" : "") + (P.clock ? " pr-clock-on" : ""));
    root.id = "present";
    const stage = root.appendChild(el("div", "pr-stage"));
    deck = stage.appendChild(el("div", "pr-deck pages slides " + (o.ratio === "4:3" ? "r43" : "r169")));
    const [w, h] = window.ItdaSlides.SIZE[o.ratio] || window.ItdaSlides.SIZE["16:9"];
    deck.style.setProperty("--page-w", w + "px");
    deck.style.setProperty("--page-h", h + "px");
    deck.style.width = w + "px";
    deck.style.height = h + "px";
    for (const pg of o.pages) {
      const c = pg.cloneNode(true);
      c.classList.remove("src-active");
      for (const x of c.querySelectorAll(".src-active, .chart-wait, .chart-play")) x.classList.remove("src-active", "chart-wait", "chart-play");
      for (const x of c.querySelectorAll("[id]")) x.removeAttribute("id"); // 미리보기와 id 가 겹치지 않게
      // 미리보기의 읽기 전용 판서 층(ink.js preview)은 따라오지 않는다: 발표는 같은 판서를 자기 층(attachInk)에 그리므로, 따라오면 두 겹이 되고
      // 지우개·되돌리기·전부 지우기가 복제된 층을 지우지 못한다
      for (const x of c.querySelectorAll(".ink-preview")) x.remove();
      c.classList.add("pr-slide");
      deck.appendChild(c);
      P.slides.push(c);
    }
    root.appendChild(el("div", "pr-black"));
    if (P.audience) { startAudience(w, h); return; }
    const notes = notesEl = root.appendChild(el("div", "pr-notes"));
    const head = notes.appendChild(el("div", "pr-notes-head"));
    head.appendChild(el("span", "pr-notes-title", "발표자 노트"));
    // 노트 있는 슬라이드 수/전체 — 노트가 하나도 없으면 그리지 않는다
    const nNoted = notedCount();
    if (nNoted) { const c = head.appendChild(el("span", "pr-notes-count", "노트 " + nNoted + "/" + P.slides.length)); c.title = "노트가 있는 슬라이드 " + nNoted + "장 / 전체 " + P.slides.length + "장"; }
    dockBtn = head.appendChild(el("button", "pr-dock-btn"));
    dockBtn.type = "button";
    dockBtn.addEventListener("click", ev => { ev.stopPropagation(); setDock(!P.noteDock); });
    paintDock();
    head.addEventListener("mousedown", onHeadDrag);
    const fsBtn = (text, dir, title) => {
      const b = head.appendChild(el("button", "pr-fs", text));
      b.type = "button";
      b.title = title;
      b.addEventListener("click", ev => { ev.stopPropagation(); stepFont(dir); });
      return b;
    };
    fsBtn("A−", -1, "노트 글자 작게(−)");
    fsVal = head.appendChild(el("span", "pr-fs-val"));
    fsBtn("A+", 1, "노트 글자 크게(+)");
    paintFont();
    buildNext(notes, w, h); // 머리 아래·노트 위
    notesBody = notes.appendChild(el("div", "pr-notes-body"));
    const grip = root.appendChild(el("div", "pr-notes-grip"));
    grip.title = "끌어 노트 너비 바꾸기 — 두 번 누르면 기본 너비";
    grip.addEventListener("mousedown", onGrip);
    grip.addEventListener("dblclick", ev => { ev.stopPropagation(); resizeNote(NOTE_W_DEF, cardRight()); if (opts.onNoteW) opts.onNoteW(P.noteW); });
    setNoteW(Number(o.noteW) || NOTE_W_DEF);
    buildToc();
    setTocW(Number(o.tocW) || TOC_W_DEF);
    const hud = root.appendChild(el("div", "pr-hud"));
    const bar = hud.appendChild(el("div", "pr-bar"));
    barEl = bar.appendChild(el("i"));
    // 노트 있는 슬라이드 자리의 눈금 — 슬라이드 i 의 막대 구간 [i/N, (i+1)/N] 가운데. 아래 줄과 함께 숨는다
    if (nNoted) P.slides.forEach((_, i) => { if (hasNote(i)) { const t = bar.appendChild(el("b", "pr-tick")); t.style.left = ((i + 0.5) / P.slides.length * 100).toFixed(3) + "%"; } });
    numEl = hud.appendChild(el("span", "pr-num"));
    timeEl = hud.appendChild(el("span", "pr-time", "00:00"));
    clockEl = hud.appendChild(el("span", "pr-clock")); // 현재 시각 — pr-clock-on 일 때만 보인다(머리 주석 「발표자 보조」)
    clockEl.title = "현재 시각";
    shareEl = hud.appendChild(el("span", "pr-share", shareSummary));
    hud.appendChild(el("span", "pr-hint", "← → 넘기기 · 번호 Enter 이동 · G 한눈에 · T 목차(⇧D 붙이기/띄우기) · N 노트(+/− 글자 · D 붙이기/띄우기 · 머리 끌어 옮기기) · 그림·표 클릭 크게 · B 검은 화면 · L 레이저 · P 펜 · H 형광펜 · E 지우개(켠 동안 X 전부 지우기 · C 색 · ⌘Z 되돌리기) · Esc 끝"));
    gotoEl = root.appendChild(el("div", "pr-goto"));
    gridEl = root.appendChild(el("div", "pr-grid"));
    // 한눈에 보기 칸의 복제 쪽도 덱과 같은 쪽 크기로 놓는다 — 칸은 덱 밖(#present 바로 아래)이라 덱에 건 변수를 받지 못해
    // :root 기본값(A4 210mm)으로 놓였다. 사이드바 축소본이 칸 상자에 거는 것과 같은 방식이다
    gridEl.style.setProperty("--page-w", w + "px");
    gridEl.style.setProperty("--page-h", h + "px");
    attachLightbox();
    document.body.appendChild(root);
    document.body.classList.add("presenting");
    // 창 전체 화면(호스트 훅 — 머리 주석 「호스트 계약」). fsCalls 는 진입 요청 횟수 — 훅이 없어도 센다
    P.fsCalls++;
    hostCall("fullscreen", true);
    P.started = Date.now();
    timer = setInterval(tick, 500);
    tick();
    window.addEventListener("resize", onResize);
    window.addEventListener("blur", endDrag); // 끌다가 창이 포커스를 잃으면 mouseup 이 오지 않을 수 있다
    // 라이트박스를 열고 닫은 누르기의 뒤따르는 click·auxclick 은 아무것도 하지 않는다 — 창 붙잡기 단계에서 먼저 먹어
    // 문서의 링크 처리기(호스트 — document 거품 단계)·기본 동작(링크 이동)까지 가지 않게 한다
    window.addEventListener("click", eatClick, true);
    window.addEventListener("auxclick", eatClick, true);
    // 카드 높이는 내용·글자 크기를 따른다 — 바뀌면 화면 안에 다시 가두고 경계(grip)를 맞춘다
    // 목차 카드 높이도 같다
    if (window.ResizeObserver) { cardRO = new ResizeObserver(() => { layoutCard(); layoutToc(); fitNext(); }); cardRO.observe(notes); cardRO.observe(tocEl); }
    document.addEventListener("keydown", onKey, true);
    root.addEventListener("mousemove", wake);
    root.addEventListener("mousedown", onMouse);
    root.addEventListener("contextmenu", e => e.preventDefault());
    fit();
    attachInk(w, h);
    go(Math.max(0, Math.min(o.index || 0, P.slides.length - 1)), 0, 1);
    wake();
  }

  // startAudience 는 청중(따라가기) 열기의 나머지다(start 가 덱·검은 화면 층까지 만든 뒤 — 머리 주석 「두 화면 발표」). 노트·목차·아래 줄·번호 이동·
  // 한눈에 보기·크게 보기·판서 입력은 만들지 않고(받은 판서는 inkView 가 그린다) 키·마우스·커서 깨우기 리스너도 걸지 않는다. 지난 발표가 남긴 부품 참조는 비워 go·stop 이 건너뛰게 한다.
  function startAudience(w, h) {
    notesEl = notesBody = dockBtn = fsVal = numEl = timeEl = barEl = gotoEl = gridEl = tocEl = tocBody = tocDockBtn = shareEl = null;
    nextEl = nextBox = clockEl = null;
    lb = lbEl = null;
    tocItems = [];
    // 덱 사본의 단추·입력·링크(섹션 묻기 단추·코드 복사 단추 등)는 청중 화면에서 입력이 아니다: 덱을 inert 로 두고(지원하는 웹뷰), 지원하지 않아도
    // 포커스 차례에서 빼며(tabindex -1), 누르기·키보드 활성화의 click 은 eatClick 이 붙잡기 단계에서 먹는다(문서의 위임 처리기 — ask.js handler — 까지 가지 않는다)
    deck.setAttribute("inert", "");
    deck.inert = true;
    for (const x of deck.querySelectorAll(FOCUSABLE)) x.setAttribute("tabindex", "-1");
    P.noteW = NOTE_W_DEF;
    P.tocW = TOC_W_DEF;
    document.body.appendChild(root);
    document.body.classList.add("presenting");
    P.fsCalls++;
    hostCall("fullscreen", true);
    P.started = Date.now();
    window.addEventListener("resize", onResize);
    // 덱 안 링크의 click 은 먹는다 — 창이 그 주소로 이동하지 않게(누른 링크 표지 linkDown 이 없으니 열지는 않는다)
    window.addEventListener("click", eatClick, true);
    window.addEventListener("auxclick", eatClick, true);
    root.addEventListener("contextmenu", e => e.preventDefault());
    fit();
    // 발표자 판서를 받아 그린다(머리 주석 「두 화면 발표」 판서 받기). 붙이기 전에 받은 연산도 여기서 그린다
    const v = inkViewOf();
    if (v) {
      v.mount({ slides: () => P.slides, size: [w, h], scale: () => P.scale || 1, idx: () => P.idx, paused: () => !!P.black });
      inkMounted = true;
    }
    go(Math.max(0, Math.min(opts.index || 0, P.slides.length - 1)), 0, 1);
  }

  // ---- 보드 발표: 보드는 나누지 않는 긴 한 장이라 넘기지 않고 전체 화면에서 굴린다. 폭을 화면에 맞추고 휠·↑↓·Space·PageUp/Down·Home/End 로
  // 굴린다(↑↓ 는 BOARD_LINE, Space·Page 는 한 화면 안쪽). 진행 막대는 굴림 비율이다. 장 번호를 전제한 것(번호·한눈에 보기·노트·목차·번호 Enter)은 없다.
  // 링크·표·그림 라이트박스는 그대로다 — 눌러도 다음 장이 없으니 그 밖의 누르기는 아무것도 하지 않는다. 타일 크게 보기는 발표 중에 쓰지 않는다.
  // o = { page(미리보기의 보드), ratio, from(시작 굴림 비율 0~1), onSlide(i, total)(시작 0, 끝 -1 — 공유가 꼴을 붙든다), onScroll()(굴릴 때마다),
  //   onExit(frac)(끝난 굴림 비율), onLink(href)(start 와 같다) }
  const BOARD_LINE = 120; // ↑↓ 한 번의 화면 px
  let boardBox = null, stage = null;
  function startBoard(o) {
    if (P.on) return;
    P.on = true;
    P.board = true;
    P.audience = false; // 두 화면 발표(onState·audience·show)는 슬라이드 발표에만 있다
    P.penOnly = false; // 보드 발표에는 판서가 없다
    P.nextPreview = P.clock = false; // 발표자 보조도 없다
    nextEl = nextBox = clockEl = null;
    onState = null;
    opts = o;
    P.slides = [];
    P.notes = [];
    P.grid = false;
    P.black = false;
    P.showNotes = false;
    P.showToc = false;
    P.lightbox = "";
    P.lbZoom = 0;
    P.idx = 0;
    onExit = o.onExit;
    onSlide = o.onSlide || null;
    root = el("div", "present pr-board");
    root.id = "present";
    stage = root.appendChild(el("div", "pr-stage"));
    boardBox = stage.appendChild(el("div", "pr-board-box"));
    deck = boardBox.appendChild(el("div", "pr-deck pages boards " + (o.ratio === "4:3" ? "r43" : "r169")));
    const [w, h] = window.ItdaSlides.SIZE[o.ratio] || window.ItdaSlides.SIZE["16:9"];
    deck.style.setProperty("--page-w", w + "px");
    deck.style.setProperty("--page-h", h + "px");
    deck.style.width = w + "px";
    const c = o.page.cloneNode(true);
    for (const x of c.querySelectorAll(".src-active, .chart-wait, .chart-play, .toc-focus")) x.classList.remove("src-active", "chart-wait", "chart-play", "toc-focus");
    for (const x of c.querySelectorAll("[id]")) x.removeAttribute("id"); // 미리보기와 id 가 겹치지 않게
    c.classList.add("pr-slide", "pr-cur");
    deck.appendChild(c);
    P.slides.push(c);
    root.appendChild(el("div", "pr-black"));
    const hud = root.appendChild(el("div", "pr-hud"));
    barEl = hud.appendChild(el("div", "pr-bar")).appendChild(el("i"));
    timeEl = hud.appendChild(el("span", "pr-time", "00:00"));
    shareEl = hud.appendChild(el("span", "pr-share", shareSummary));
    hud.appendChild(el("span", "pr-hint", "휠·↑ ↓·Space 굴리기 · Home·End 처음·끝 · 그림·표 클릭 크게 · B 검은 화면 · Esc 끝"));
    attachLightbox();
    document.body.appendChild(root);
    document.body.classList.add("presenting");
    P.fsCalls++;
    hostCall("fullscreen", true);
    P.started = Date.now();
    timer = setInterval(tick, 500);
    tick();
    window.addEventListener("resize", onResize);
    window.addEventListener("click", eatClick, true);
    window.addEventListener("auxclick", eatClick, true);
    document.addEventListener("keydown", onKey, true);
    root.addEventListener("mousemove", wake);
    root.addEventListener("mousedown", onMouse);
    root.addEventListener("contextmenu", e => e.preventDefault());
    stage.addEventListener("scroll", onBoardScroll, { passive: true });
    fit();
    stage.scrollTop = (o.from || 0) * (stage.scrollHeight - stage.clientHeight);
    onBoardScroll();
    wake();
    if (onSlide) onSlide(0, 1);
  }
  function boardFrac() { const max = stage.scrollHeight - stage.clientHeight; return max > 0 ? stage.scrollTop / max : 0; }
  // boardBarWidth 는 진행 막대 % 다 — 굴릴 곳이 없으면(보드가 화면보다 짧다) 끝까지 본 것이다.
  function boardBarWidth(top, max) { return max > 0 ? Math.min(100, Math.max(0, top / max * 100)) : 100; }
  function onBoardScroll() {
    barEl.style.width = boardBarWidth(stage.scrollTop, stage.scrollHeight - stage.clientHeight).toFixed(2) + "%";
    if (opts.onScroll) opts.onScroll();
  }
  // fitBoard 는 폭을 화면에 맞춘다. 상자는 그린 크기를 갖고(굴림 범위가 된다) 보드가 화면보다 짧으면 상자를 세로 가운데로 내린다 — 상자가 overflow: hidden 이라
  // 덱만 내리면 아래가 잘린다.
  function fitBoard() {
    const frac = boardFrac();
    const W = stage.clientWidth, H = stage.clientHeight;
    const w = parseFloat(deck.style.width), s = W / w, h = deck.firstElementChild.offsetHeight;
    P.scale = s;
    deck.style.height = h + "px";
    const drawn = Math.ceil(h * s);
    boardBox.style.height = drawn + "px";
    boardBox.style.marginTop = Math.max(0, Math.floor((H - drawn) / 2)) + "px";
    deck.style.transform = "scale(" + s + ")";
    stage.scrollTop = frac * (stage.scrollHeight - stage.clientHeight);
  }
  // boardKey 는 굴림 키다 — 뜻이 없는 키(번호·G·N·T·D)는 삼킨다. 움직임 줄이기면 즉시 옮긴다.
  function boardKey(k) {
    const page = Math.max(BOARD_LINE, stage.clientHeight - 80), max = stage.scrollHeight - stage.clientHeight;
    const to = { ArrowDown: BOARD_LINE, ArrowUp: -BOARD_LINE, " ": page, PageDown: page, PageUp: -page }[k];
    if (to !== undefined) stage.scrollTo({ top: stage.scrollTop + to, behavior: reduce() ? "auto" : "smooth" });
    else if (k === "Home") stage.scrollTop = 0;
    else if (k === "End") stage.scrollTop = max;
    else if (k === "b" || k === "B" || k === ".") setBlack();
    else if (k === "Escape") stop();
  }

  function stop() {
    if (!P.on) return;
    if (P.board) { stopBoard(); return; }
    P.on = false;
    clearInterval(timer);
    clearTimeout(idleT);
    window.removeEventListener("resize", onResize);
    window.removeEventListener("blur", endDrag);
    window.removeEventListener("click", eatClick, true);
    window.removeEventListener("auxclick", eatClick, true);
    lbEat = false;
    if (ink) { ink.detach(); ink = null; }
    if (inkMounted) { inkView.unmount(); inkMounted = false; } // 층만 뗀다 — 받은 판서(모델)는 남는다
    linkDown = null;
    endDrag(); // 끄는 중에 끝내도 document 리스너가 남아 다음 발표를 건드리지 않게
    if (cardRO) { cardRO.disconnect(); cardRO = null; }
    tocItems = [];
    document.removeEventListener("keydown", onKey, true);
    hostCall("fullscreen", false);
    if (lb) lb.close();
    P.lightbox = "";
    root.remove(); // 겹침 층(호스트 요소)은 root 밖이라 남는다
    document.body.classList.remove("presenting");
    syncOverlay(); // P.on 이 내려가 가림 클래스를 뗀다
    const at = P.idx, total = P.slides.length, last = { i: at, step: P.step, steps: P.slides[at] ? stepsOf(at) : 0, total, next: nextIndex(), black: false, grid: false, lightbox: false, on: false };
    // 끝낼 발표의 콜백을 지역으로 쥐고, 공유 상태 정리를 콜백 앞에 끝낸다 — 콜백(onExit·onSlide·onState) 안에서 새 발표를 열어도
    // 그 발표의 콜백·상태·부품을 이 발표의 끝 처리가 건드리지 않는다. 끝 알림(onState on:false)은 첫 상태를 보낸 발표에만 한 번이다.
    const exitCb = onExit, slideCb = onSlide, stateCb = onState && sentState ? onState : null;
    P.slides = [];
    onExit = onSlide = onState = null;
    sentState = "";
    P.audience = false;
    notesEl = notesBody = dockBtn = fsVal = numEl = timeEl = barEl = gotoEl = gridEl = tocEl = tocBody = tocDockBtn = shareEl = null;
    nextEl = nextBox = clockEl = null;
    nextAt = -2;
    P.nextPreview = P.clock = false;
    lb = lbEl = null;
    if (exitCb) exitCb(at);
    if (slideCb) slideCb(-1, total);
    if (stateCb) stateCb(last);
  }

  function stopBoard() {
    const frac = boardFrac();
    P.on = false;
    P.board = false;
    clearInterval(timer);
    clearTimeout(idleT);
    window.removeEventListener("resize", onResize);
    window.removeEventListener("click", eatClick, true);
    window.removeEventListener("auxclick", eatClick, true);
    lbEat = false;
    linkDown = null;
    document.removeEventListener("keydown", onKey, true);
    hostCall("fullscreen", false);
    if (lb) lb.close();
    P.lightbox = "";
    root.remove(); // 겹침 층(호스트 요소)은 root 밖이라 남는다
    document.body.classList.remove("presenting");
    syncOverlay(); // P.on 이 내려가 가림 클래스를 뗀다
    P.slides = [];
    stage = boardBox = null;
    if (onExit) onExit(frac);
    if (onSlide) onSlide(-1, 1);
  }

  function onResize() { relayout(); if (P.lightbox && lb) lb.relayout(); }

  // ---- 두 칸의 실제 너비: 목차·노트를 둘 다 붙이면 무대(슬라이드)에 창의 STAGE_MIN% 이상을 남긴다.
  // 선호 너비(P.noteW·P.tocW — 끌기·기억한 값)는 그대로 두고, 두 칸 합이 100 − STAGE_MIN 을 넘으면 실제 배치 너비만 두 칸을 선호 너비
  // 비율대로 줄인다(effWidths — 선호값의 순수 함수라 켜기·도킹 전환·시작·창 크기 바꾸기 모두 같은 결과). 그 밖(한 칸만 붙임·카드)은
  // 선호 너비 그대로다. 줄인 값은 저장하지 않는다.
  const STAGE_MIN = 40;
  function bothDocked() { return P.showNotes && P.noteDock && P.showToc && P.tocDock; }
  function effWidths() {
    const n = P.noteW, t = P.tocW, cap = 100 - STAGE_MIN;
    if (!bothDocked() || n + t <= cap) return { n, t };
    const k = cap / (n + t);
    return { n: Math.floor(n * k * 100) / 100, t: Math.floor(t * k * 100) / 100 }; // 내림 — 합이 cap 을 넘지 않게
  }
  // prefFor 는 둘 다 붙인 채 끌 때, 끄는 칸의 실제 너비가 e(%)가 되는 선호 너비다(다른 칸의 선호 너비 other 는 그대로).
  // 합이 넘치지 않으면 e 그대로, 넘치면 effWidths 의 역(e = p × cap/(p+other) → p = e × other/(cap − e)). 칸 범위 [lo, hi] 에 가둔다 —
  // hi 에 닿으면 실제 너비도 거기서 멈춘다(무대는 늘 STAGE_MIN% 이상). 놓을 때 저장하는 값은 이 선호 너비다.
  function prefFor(e, other, lo, hi) {
    const cap = 100 - STAGE_MIN;
    const p = e + other <= cap ? e : e >= cap ? hi : e * other / (cap - e);
    return Math.round(Math.min(hi, Math.max(lo, p)) * 10) / 10;
  }
  function applyWidths() {
    if (!root || P.board) return;
    const e = effWidths();
    root.style.setProperty("--pr-notes-w", e.n + "%");
    root.style.setProperty("--pr-toc-w", e.t + "%");
  }
  // relayout 은 칸 너비·켬·도킹·창 크기가 바뀐 뒤 한 번에 다시 놓는다: 실제 너비 → 슬라이드 맞춤 → 떠 있는 카드 가두기.
  function relayout() { if (P.board) { fitBoard(); return; } applyWidths(); fit(); layoutCard(); layoutToc(); fitNext(); }

  // fit 은 슬라이드를 화면(노트를 오른쪽에·목차를 왼쪽에 붙였으면 남은 자리 — 떠 있는 카드면 화면 전체)에 비율을 지켜 맞춘다.
  function fit() {
    if (!P.on) return;
    if (P.board) { fitBoard(); return; }
    const st = root.querySelector(".pr-stage");
    const W = st.clientWidth, H = st.clientHeight;
    const w = parseFloat(deck.style.width), h = parseFloat(deck.style.height);
    const s = Math.min(W / w, H / h);
    P.scale = s;
    deck.style.transform = "translate(" + Math.round((W - w * s) / 2) + "px," + Math.round((H - h * s) / 2) + "px) scale(" + s + ")";
    if (ink) ink.resize();
    if (inkMounted) inkView.resize();
  }

  function tick() {
    const t = Math.floor((Date.now() - P.started) / 1000);
    const mm = String(Math.floor(t / 60)).padStart(2, "0"), ss = String(t % 60).padStart(2, "0");
    timeEl.textContent = (t >= 3600 ? Math.floor(t / 3600) + ":" : "") + mm + ":" + ss;
    if (clockEl && P.clock) clockEl.textContent = clockText(new Date());
  }
  // clockText 는 현재 시각 글(HH:mm, 24시간 — 머리 주석 「발표자 보조」)이다.
  function clockText(d) { return String(d.getHours()).padStart(2, "0") + ":" + String(d.getMinutes()).padStart(2, "0"); }

  // ---- 다음 장 미리보기(머리 주석 「발표자 보조」). 칸은 노트 칸 안(머리 아래)에 늘 만들고, 보이기는 pr-next-on(present.css)이다.
  // 상자는 한눈에 보기 썸네일(.pr-thumb)과 같은 규칙으로 사본을 그린다 — 덱 밖이라 쪽 크기 변수를 상자에 건다(한눈에 보기 칸과 같다).
  // 사본에서 떼는 것: 판서 층(발표자의 판서를 미리보기에 넣지 않는다)·섹션 묻기 단추·코드 복사 단추(입력 부품)
  const NEXT_DROP = ".ink-hl, .ink-pen, .ink-live, .ink-laser-page, .ink-preview, .itda-section-ask, .itda-codecopy";
  // 장을 넘어 이어지는 CSS 카운터 — 이름과 그 카운터를 올리는 요소의 선택자. base.css 의 증가 규칙과 정확히 같은 선택자다:
  //   fig ← `.page-body figcaption::before { counter-increment: fig }`(base.css 388 부근), tbl ← `.page-body caption::before { counter-increment: tbl }`(678 부근).
  //   둘의 시작은 `.pages { counter-reset: fig tbl }`(384 부근 — 덱 .pr-deck.pages 가 이것을 받는다). base.css 의 다른 카운터(list-item)는 목록마다 새로
  //   세므로 장을 넘지 않는다. base.css 에 장을 넘는 카운터를 더하면 여기에도 더한다(present_test.go TestPresentCountersCSS 가 묶는다).
  // 사본만 따로 두면 앞 장의 캡션 수를 잃어 「그림 1.」 부터 다시 센다 — 미리보기 상자에 독립 범위를 두고 다음 장 앞까지의 수로 시작한다(paintNext).
  const NEXT_COUNTERS = [["fig", ".page-body figcaption"], ["tbl", ".page-body caption"]];
  // 「카운터를 올리는 캡션」 = 증가 선택자에 맞고 브라우저가 실제로 배치한 것(getClientRects 가 하나 이상)이다. CSS 숨김 규칙을 여기서 흉내 내지 않는다:
  //   덱의 장은 visibility: hidden 으로 숨는다(present.css 28 부근 `.pr-deck .page { … visibility: hidden }`) — 배치는 살아 있어 지금 장이 아닌 장·아직
  //   나타나지 않은 단계(visibility hidden)의 캡션은 상자가 있고 카운터도 오른다. 자신이나 조상이 display: none 인 캡션(예: 결재란 캡션 — base.css)은
  //   상자도 ::before 도 없어 카운터가 오르지 않고 여기서도 빠진다. 판정은 브라우저 몫이라 개별 예외를 적지 않는다.
  //   getClientRects 가 없는 환경(시험의 가짜 DOM)은 선택자에 맞는 것을 모두 센다 — 실제 판정은 호스트 앱의 WebKit 시험이 본다.
  const laidOut = x => typeof x.getClientRects !== "function" || x.getClientRects().length > 0;
  // capCache[i] 는 장 i 의 카운터별 수다 — 발표마다 새로(start). 발표 동안 덱은 바뀌지 않는다(문서 갱신은 새 발표의 새 덱이다)
  let capCache = [];
  function capsOf(i) {
    if (!capCache[i]) capCache[i] = NEXT_COUNTERS.map(([, sel]) => { let k = 0; for (const x of P.slides[i].querySelectorAll(sel)) if (laidOut(x)) k++; return k; });
    return capCache[i];
  }
  // countersBefore 는 덱에서 장 n 앞(0~n−1 장)까지 각 카운터를 올리는 캡션 수로 만든 counter-reset 글이다(예: "fig 3 tbl 1").
  function countersBefore(n) {
    const sum = NEXT_COUNTERS.map(() => 0);
    for (let i = 0; i < n; i++) capsOf(i).forEach((k, j) => { sum[j] += k; });
    return NEXT_COUNTERS.map(([name], j) => name + " " + sum[j]).join(" ");
  }
  function buildNext(notes, w, h) {
    nextEl = notes.appendChild(el("div", "pr-next"));
    nextEl.setAttribute("aria-hidden", "true"); // 읽어 주지 않는다 — 미리 보는 그림이다
    nextEl.setAttribute("inert", "");
    nextEl.inert = true;
    nextEl.appendChild(el("div", "pr-next-label", "다음"));
    nextBox = nextEl.appendChild(el("div", "pr-thumb pr-next-box"));
    nextBox.style.setProperty("--page-w", w + "px");
    nextBox.style.setProperty("--page-h", h + "px");
  }
  // paintNext 는 미리보기를 지금 장의 다음 장으로 맞춘다 — 담은 장과 같으면 아무것도 하지 않는다(단계 넘김·같은 장). 꺼져 있으면 비운다.
  // 넘김(goNow) 안에서 곧바로(동기) 부르므로 연달아 넘겨도 마지막 장의 다음 장이 남는다.
  function paintNext() {
    if (!nextBox) return;
    if (!P.nextPreview) { if (nextAt !== -2) { nextBox.textContent = ""; nextAt = -2; } return; }
    const n = nextIndex();
    if (n === nextAt) return;
    nextAt = n;
    nextBox.textContent = "";
    nextBox.classList.toggle("pr-next-end", n < 0);
    if (n < 0) { nextBox.appendChild(el("span", "pr-next-end-t", "끝")); fitNext(); return; }
    const c = P.slides[n].cloneNode(true);
    c.classList.remove("pr-cur", "pr-out", "pr-in", "pr-back");
    c.classList.add("pr-static");
    for (const x of c.querySelectorAll(NEXT_DROP)) x.remove();
    for (const x of c.querySelectorAll(".pr-pending, .pr-shown, .chart-play, .chart-wait")) x.classList.remove("pr-pending", "pr-shown", "chart-play", "chart-wait");
    for (const x of c.querySelectorAll("[id]")) x.removeAttribute("id"); // 덱 사본에서 이미 지웠지만 뒤에 붙은 것이 있어도 겹치지 않게
    for (const x of c.querySelectorAll(FOCUSABLE)) x.setAttribute("tabindex", "-1");
    // 그림·표 번호: 상자가 새 카운터 범위다(덱·원문 카운터는 건드리지 않는다) — 사본의 첫 캡션이 덱의 같은 캡션과 같은 번호가 되게 앞 장까지의 수로 시작한다
    nextBox.style.setProperty("counter-reset", countersBefore(n));
    nextBox.appendChild(c);
    fitNext();
  }
  // fitNext 는 미리보기 상자를 노트 칸 너비에 맞춘다(높이는 쪽 비율, 사본은 같은 배율로 줄인다). 칸이 숨어 너비가 0 이면 다음에 보일 때(relayout) 맞춘다.
  function fitNext() {
    if (!nextBox || !P.nextPreview || !P.showNotes || !deck) return;
    const bw = nextBox.clientWidth;
    if (!bw) return;
    const w = parseFloat(deck.style.width), h = parseFloat(deck.style.height), s = bw / w;
    nextBox.style.height = Math.round(h * s) + "px";
    const c = nextBox.firstElementChild;
    if (c && c.classList.contains("page")) c.style.transform = "scale(" + s + ")";
  }

  // wake 는 마우스·HUD 를 보이고 2초 뒤 숨긴다. 판서 도구가 켜진 동안은 숨기지 않는다.
  function wake() {
    root.classList.remove("pr-idle");
    clearTimeout(idleT);
    if (ink && ink.tool()) return;
    idleT = setTimeout(() => root.classList.add("pr-idle"), 2000);
  }

  // showSteps 는 슬라이드 i 의 단계를 step 까지 보인다(움직임 없이 바로 — present.css). animate 면 새로 보이는 것 안의 차트가 움직인다.
  function showSteps(i, step, animate) {
    const pg = P.slides[i];
    for (const x of pg.querySelectorAll("[data-step]")) {
      const on = Number(x.dataset.step) <= step;
      const was = x.classList.contains("pr-shown");
      x.classList.toggle("pr-shown", on);
      x.classList.toggle("pr-pending", !on);
      if (on && !was && animate) for (const f of (x.matches("figure.chart") ? [x] : x.querySelectorAll("figure.chart"))) playChart(f);
    }
  }

  function playChart(f) {
    if (reduce()) return;
    f.classList.remove("chart-play", "chart-wait");
    void f.offsetWidth;
    f.classList.add("chart-play");
    setTimeout(() => f.classList.remove("chart-play"), 2400);
  }

  // go 는 슬라이드 i 를 step 단계까지 보인다. dir 은 넘기는 방향(1 다음, -1 이전) — 앞으로 갈 때만 차트가 움직인다.
  // 슬라이드 안 블록은 움직이지 않는다 — 확대(transform scale)된 덱에서 움직이는 블록은 WebKit 이 흐리게 그렸다가 끝난 뒤 다시 그려,
  // 넘길 때마다 1~2초 일그러져 보였다. 그래서 블록 떠오름을 뺐고 전환 기본도 none 이다.
  // 상태 알림(onState)은 다 바꾼 뒤 한 번이다(batch — 안의 크게 보기 닫기는 따로 알리지 않는다).
  function go(i, step, dir) { batch(() => goNow(i, step, dir)); }
  const outTimer = new WeakMap(); // 장 → 나가는 장 정리 타이머(goNow)
  function dropOut(s) { const t = outTimer.get(s); if (t !== undefined) { clearTimeout(t); outTimer.delete(s); } }
  // settleTransition 은 진행 중인 장 넘김 전환을 곧바로 끝낸다(itda-doc#105) — 판서 입력(ink.js onDown)이 좌표를 재기 전에 부른다.
  // 판서 좌표(ink.js toDesign)는 멈춘 덱 기준이라, 움직이는 장(pr-in·pr-out 의 transform) 위 획은 펜 끝에서 어긋난다. 그래서 지금 장은
  // pr-in·pr-back 을 떼어 제자리에 두고, 나가는 장은 pr-out·pr-back 을 떼어 숨기며 그 정리 타이머도 지운다 — 보이는 장은 지금 장 하나다.
  function settleTransition() {
    const cur = P.slides[P.idx];
    for (const s of P.slides) {
      if (s === cur) s.classList.remove("pr-in", "pr-back");
      else s.classList.remove("pr-out", "pr-back");
      dropOut(s);
    }
  }
  function goNow(i, step, dir) {
    const t0 = performance.now();
    closeLightbox(); // 슬라이드·단계가 바뀌면 라이트박스는 남지 않는다
    const prev = P.slides[P.idx];
    const next = P.slides[i];
    const same = prev === next && P.entered;
    P.idx = i;
    P.step = step;
    if (ink) ink.slide(i);
    if (inkMounted) inkView.wake(); // 레이저는 지금 장일 때만 보인다
    if (!same) {
      // 빠른 연달아 넘김: 넘길 때마다 지금(next)·바로 앞(prev) 밖의 장은 곧바로 숨긴다 — 보이는 장은 늘 둘 이하다.
      // 나가는 장 정리 타이머는 장마다 하나 — 앞 넘김에서 건 것이 남아 있다가 다시 나가는 그 장의 움직임을 중간에 끊지 않게 새로 걸 때 지운다
      for (const s of P.slides) if (s !== next && s !== prev) { s.classList.remove("pr-cur", "pr-out", "pr-in", "pr-back"); dropOut(s); }
      if (prev && prev !== next) {
        prev.classList.remove("pr-cur", "pr-in");
        prev.classList.add("pr-out");
        prev.classList.toggle("pr-back", dir < 0);
        dropOut(prev);
        outTimer.set(prev, setTimeout(() => { outTimer.delete(prev); if (P.slides[P.idx] !== prev) prev.classList.remove("pr-out", "pr-back"); }, OUT_MS));
      }
      dropOut(next);
      next.classList.remove("pr-out", "pr-in", "pr-back");
      void next.offsetWidth;
      next.classList.add("pr-cur", "pr-in");
      next.classList.toggle("pr-back", dir < 0);
      const forward = dir > 0 && !reduce();
      showSteps(i, step, false);
      for (const f of next.querySelectorAll("figure.chart")) {
        const hidden = f.closest(".pr-pending");
        if (forward && !hidden) playChart(f); else f.classList.remove("chart-play", "chart-wait");
      }
      P.entered = 1;
    } else {
      showSteps(i, step, dir > 0);
    }
    // 청중 열기에는 아래 줄·노트 칸이 없다(startAudience)
    if (numEl) numEl.textContent = (i + 1) + " / " + P.slides.length;
    if (barEl) barEl.style.width = ((i + (stepsOf(i) ? step / (stepsOf(i) + 1) : 0) + 1) / P.slides.length * 100).toFixed(2) + "%";
    if (notesBody) {
      notesBody.innerHTML = P.notes[i] || ""; // mdoc 이 만든 HTML(스크립트 없음, CSP 가 2차로 막는다)
      if (!P.notes[i]) notesBody.appendChild(el("p", "pr-none", "이 슬라이드에는 노트가 없습니다."));
      // 노트 칸을 매 슬라이드 다시 만들므로 복사 단추도 매번 다시 붙인다(ItdaCodeCopy 가 있을 때) — 노트 코드의 조각 범위는 이 칸 안
      if (window.ItdaCodeCopy) window.ItdaCodeCopy.arm(notesBody);
    }
    paintNext(); // 다음 장 미리보기(머리 주석 「발표자 보조」) — 다음 장이 바뀔 때만 사본을 바꾼다
    layoutCard(); // 카드 높이가 노트를 따라 바뀐다(ResizeObserver 가 없는 웹뷰에서도 화면 안에 가둔다)
    // 키 → 화면 전환(DOM·클래스)까지, 그리고 다음 슬라이드가 그려지기까지(다음 그리기 두 번 뒤 — 창이 가려지면 그리기가 미뤄져 재지 않는다)
    P.switchMs.push(Math.round((performance.now() - t0) * 10) / 10);
    if (P.switchMs.length > 50) P.switchMs.shift();
    requestAnimationFrame(() => requestAnimationFrame(() => { P.latency.push(Math.round(performance.now() - t0)); if (P.latency.length > 50) P.latency.shift(); }));
    if (P.grid) markGrid();
    markToc();
    if (onSlide && i !== sentIdx) { sentIdx = i; onSlide(i, P.slides.length); }
  }

  function next() {
    if (P.step < stepsOf(P.idx)) go(P.idx, P.step + 1, 1);
    else if (P.idx < P.slides.length - 1) go(P.idx + 1, 0, 1);
  }
  function prev() {
    if (P.step > 0) go(P.idx, P.step - 1, -1);
    else if (P.idx > 0) go(P.idx - 1, stepsOf(P.idx - 1), -1);
  }
  function jump(i) { if (i >= 0 && i < P.slides.length && i !== P.idx) go(i, i > P.idx ? 0 : stepsOf(i), i > P.idx ? 1 : -1); }

  // show 는 보이는 상태를 덮어쓴다(멱등 — 머리 주석 「두 화면 발표」). 청중 화면이 발표자 화면을 따라갈 때 부른다.
  function show(s) {
    if (!P.on || P.board || !s || !P.slides.length) return;
    const n = P.slides.length;
    const num = v => (v === undefined || v === null || v === "" || !isFinite(Number(v))) ? null : Math.trunc(Number(v));
    const ri = num(s.i), i = ri === null ? P.idx : Math.max(0, Math.min(ri, n - 1));
    const rs = num(s.step), step = Math.max(0, Math.min(rs === null ? (i === P.idx ? P.step : 0) : rs, stepsOf(i)));
    const black = s.black === undefined || s.black === null ? P.black : !!s.black;
    batch(() => {
      if (i !== P.idx) go(i, step, i > P.idx ? 1 : -1);
      else if (step !== P.step) go(i, step, step > P.step ? 1 : -1);
      if (black !== P.black) setBlack(black);
    });
  }

  // ---- 한눈에 보기: 슬라이드 사본을 작게. 누르거나 화살표+Enter 로 간다.
  let gridSel = 0;
  // 청중 열기에는 한눈에 보기 칸이 없다 — 켜지 않는다. 상태 알림은 다 바꾼 뒤 한 번이다(batch).
  function toggleGrid(on) { if (!gridEl) return; batch(() => toggleGridNow(on)); }
  function toggleGridNow(on) {
    P.grid = on === undefined ? !P.grid : on;
    root.classList.toggle("pr-grid-on", P.grid);
    if (P.grid) closeLightbox();
    syncOverlay();
    if (P.grid && ink) ink.pause(); // 그리던 획을 확정한 뒤 복제한다
    if (!P.grid) { gridEl.textContent = ""; return; }
    gridSel = P.idx;
    gridEl.textContent = "";
    const w = parseFloat(deck.style.width), h = parseFloat(deck.style.height);
    const cols = P.slides.length > 20 ? 6 : P.slides.length > 9 ? 5 : 4;
    const tw = Math.floor((root.clientWidth - 48 - (cols - 1) * 16) / cols);
    const s = tw / w;
    gridEl.style.setProperty("--cols", String(cols));
    P.slides.forEach((pg, i) => {
      const cell = el("button", "pr-cell");
      cell.type = "button";
      cell.dataset.i = String(i);
      const box = el("div", "pr-thumb");
      box.style.width = tw + "px";
      box.style.height = Math.round(h * s) + "px";
      const c = pg.cloneNode(true);
      c.classList.remove("pr-cur", "pr-out", "pr-in", "pr-back");
      c.classList.add("pr-static");
      for (const x of c.querySelectorAll(".pr-pending")) x.classList.remove("pr-pending");
      c.style.transform = "scale(" + s + ")";
      box.appendChild(c);
      cell.appendChild(box);
      cell.appendChild(el("span", "pr-cell-n", String(i + 1)));
      if (hasNote(i)) { const d = cell.appendChild(el("span", "pr-cell-note")); d.title = "노트 있음"; }
      cell.addEventListener("click", ev => { ev.stopPropagation(); toggleGrid(false); jump(i); });
      gridEl.appendChild(cell);
    });
    markGrid();
  }
  function markGrid() {
    for (const c of gridEl.children) c.classList.toggle("on", Number(c.dataset.i) === gridSel);
    const c = gridEl.children[gridSel];
    if (c && c.scrollIntoView) c.scrollIntoView({ block: "nearest" });
  }

  // ---- 노트 칸 너비·글자 크기·떠 있는 카드
  function setNoteW(pct) {
    P.noteW = Math.min(NOTE_W_MAX, Math.max(NOTE_W_MIN, pct));
    relayout();
  }
  // cardRight 는 노트 오른쪽 끝(창 왼쪽에서 px)이다 — 도킹이면 창 오른쪽 끝, 카드면 지금 카드의 오른쪽 끝.
  function cardRight() {
    const W = root.clientWidth;
    if (P.noteDock || !P.card) return P.noteDock ? W : W - CARD_M;
    return P.card.x + notesEl.offsetWidth;
  }
  // resizeNote 는 너비를 pct 로 바꾸되 오른쪽 끝(right)을 그대로 둔다 — 카드를 옮긴 뒤에도 경계 끌기가 왼쪽 경계만 움직이게.
  function resizeNote(pct, right) {
    setNoteW(pct);
    if (P.card) { P.card.x = right - notesEl.offsetWidth; layoutCard(); }
  }
  // layoutCard 는 떠 있는 카드를 화면 안에 가두고(끌어 옮긴 자리·기본 자리) 그 자리·높이를 뿌리의 --pr-card-x·y·h 로 건다 — present.css 가
  // 카드와 경계(grip)를 거기에 둔다. 도킹이거나 노트를 접었으면 아무것도 하지 않는다.
  function layoutCard() {
    if (!P.on || !root || !notesEl || P.noteDock || !P.showNotes) return;
    const W = root.clientWidth, H = root.clientHeight, w = notesEl.offsetWidth, h = notesEl.offsetHeight;
    let x = P.card ? P.card.x : W - CARD_M - w, y = P.card ? P.card.y : CARD_M;
    x = Math.round(Math.max(0, Math.min(x, W - w)));
    y = Math.round(Math.max(0, Math.min(y, H - HUD_H - h))); // 아래 줄을 가리지 않게
    if (P.card) P.card = { x, y };
    root.style.setProperty("--pr-card-x", x + "px");
    root.style.setProperty("--pr-card-y", y + "px");
    root.style.setProperty("--pr-card-h", h + "px");
  }
  // onHeadDrag 는 카드 머리(단추 밖) 끌기다 — 카드를 옮긴다(화면 안에 가둔다). 도킹이면 옮기지 않는다. 발표의 누르기(다음 슬라이드)가 아니다.
  function onHeadDrag(e) {
    if (e.button !== 0 || P.noteDock || e.target.closest("button")) return;
    e.preventDefault(); // 머리 글자를 고르지 않게(카드 안 글 고르기는 몸통에서 그대로 된다)
    e.stopPropagation();
    const r = root.getBoundingClientRect(), nr = notesEl.getBoundingClientRect();
    const dx = e.clientX - nr.left, dy = e.clientY - nr.top;
    root.classList.add("pr-moving");
    beginDrag(ev => { P.card = { x: ev.clientX - r.left - dx, y: ev.clientY - r.top - dy }; layoutCard(); },
      () => { if (root) root.classList.remove("pr-moving"); });
  }
  // beginDrag 는 끌기 하나의 document 리스너(mousemove·mouseup)를 건다 — 한 번에 하나만이고, 끝(endDrag)은 mouseup·왼쪽 단추가 놓인
  // mousemove(창 밖에서 놓아 mouseup 을 놓쳤을 때)·창 blur·발표 끝(stop)에서 반드시 온다. 그래서 끝난 발표의 끌기가 다음 발표의 카드를
  // 움직이지 않는다. done 은 끝날 때 한 번 부른다.
  function beginDrag(move, done) {
    endDrag();
    const mv = ev => { if (!(ev.buttons & 1)) { endDrag(); return; } move(ev); };
    const up = () => endDrag();
    dragEnd = () => {
      dragEnd = null;
      document.removeEventListener("mousemove", mv);
      document.removeEventListener("mouseup", up);
      done();
    };
    document.addEventListener("mousemove", mv);
    document.addEventListener("mouseup", up);
  }
  function endDrag() { if (dragEnd) dragEnd(); }
  function paintDock() {
    if (!dockBtn) return;
    dockBtn.textContent = P.noteDock ? "띄우기" : "붙이기";
    dockBtn.title = P.noteDock ? "노트를 슬라이드 위에 떠 있는 카드로(D) — 슬라이드는 제자리·제 크기, 머리를 끌어 옮긴다"
      : "노트를 오른쪽에 붙이기(D) — 슬라이드가 남은 자리로 줄어든다";
  }
  // setDock 은 떠 있는 카드(거짓)·오른쪽에 붙인 칸(참)을 고른다 — 슬라이드를 다시 맞추고 부른 쪽에 알린다(onNoteDock — 기억).
  function setDock(on) {
    on = !!on;
    if (on === P.noteDock) return;
    P.noteDock = on;
    root.classList.toggle("pr-dock", on);
    paintDock();
    relayout();
    if (opts.onNoteDock) opts.onNoteDock(on);
  }
  // onGrip 은 노트 왼쪽 경계 끌기다: 노트 오른쪽 끝(도킹이면 창 끝, 카드면 카드 끝 — 끄는 동안 그대로)에서 마우스까지가 너비다(창의 22~60%).
  // 놓을 때 한 번 알린다(onNoteW).
  function onGrip(e) {
    if (e.button !== 0) return;
    e.preventDefault();
    e.stopPropagation(); // 발표의 누르기(다음 슬라이드)가 아니다
    const w0 = P.noteW, right = cardRight();
    root.classList.add("pr-dragging");
    beginDrag(ev => {
      const r = root.getBoundingClientRect();
      const e = Math.round((r.left + right - ev.clientX) / r.width * 1000) / 10;
      resizeNote(bothDocked() ? prefFor(e, P.tocW, NOTE_W_MIN, NOTE_W_MAX) : e, right); // 둘 다 붙였으면 실제 너비가 마우스를 따른다
    }, () => {
      if (root) root.classList.remove("pr-dragging");
      if (P.noteW !== w0 && opts.onNoteW) opts.onNoteW(P.noteW);
    });
  }
  function curFont() { return (opts.noteFont && opts.noteFont()) || NOTE_FS_DEF; }
  function paintFont() { if (fsVal) fsVal.textContent = String(curFont()); }
  // stepFont 는 노트 글자를 한 단계 바꾼다 — 값은 부른 쪽이 쥐고(미리보기와 같은 값) 문서 뿌리의 --note-fs 로 건다(onNoteFont).
  function stepFont(dir) {
    const px = window.ItdaSlides.noteFontStep(curFont(), dir);
    if (px !== curFont() && opts.onNoteFont) opts.onNoteFont(px);
    paintFont();
    layoutCard();
  }

  // ---- 목차 칸
  // hasNote(i) 는 슬라이드 i 에 노트가 있는가, notedCount 는 노트 있는 슬라이드 수다(P.notes[i] 가 빈 글이면 없음).
  function hasNote(i) { return !!P.notes[i]; }
  function notedCount() { let n = 0; for (let i = 0; i < P.slides.length; i++) if (hasNote(i)) n++; return n; }
  // slideTitle 은 복제된 슬라이드의 머리 글이다 — 제목 슬라이드는 .sc-title, 구역은 .ss-num + .ss-title, 내용은 .slide-title(「(계속 k/n)」
  // 표지 — 이어진 장 .slide-cont·첫 장 .slide-seq — 는 뺀다). cont 는 그 머리에 이어진 장 표지(.slide-cont)가 있는가다. 머리가 없으면 title 이 "" 다.
  function slideTitle(pg) {
    const kind = pg.dataset.kind;
    const text = x => (x && x.textContent || "").replace(/\s+/g, " ").trim();
    if (kind === "title") return { title: text(pg.querySelector(".sc-title")), cont: false };
    if (kind === "section") {
      const sec = pg.querySelector(".slide-section");
      if (!sec) return { title: "", cont: true }; // 넘쳐 생긴 구역 장 — 머리가 없다
      const num = text(sec.querySelector(".ss-num:not(.auto)"));
      const t = text(sec.querySelector(".ss-title"));
      return { title: num && t ? num + " " + t : t, cont: false };
    }
    const h = pg.querySelector(".slide-head .slide-title");
    if (!h) return { title: "", cont: false };
    const c = h.cloneNode(true);
    const cont = c.querySelector(".slide-cont");
    if (cont) cont.remove();
    const seq = c.querySelector(".slide-seq");
    if (seq) seq.remove();
    const num = c.querySelector(".header-section-number");
    let t;
    if (num) { const n = text(num); num.remove(); t = (n + " " + text(c)).trim(); } else t = text(c);
    return { title: t, cont: !!cont };
  }
  // tocEntries 는 슬라이드를 목차 항목으로 묶는다: {start, end(포함), title, kind, sub(구역 아래 슬라이드인가), noted}.
  // 「(계속)」 표지가 있는 내용 장(같은 제목)·머리 없는 구역 장은 앞 항목에 묶는다. 제목이 없으면 「슬라이드 N」.
  function tocEntries() {
    const out = [];
    let inSec = false;
    P.slides.forEach((pg, i) => {
      const kind = pg.dataset.kind || "content";
      const { title, cont } = slideTitle(pg);
      const prev = out[out.length - 1];
      const isContent = kind === "content" || kind === "single";
      if (prev && cont && ((kind === "section" && prev.kind === "section") || (isContent && (prev.kind === "content" || prev.kind === "single") && prev.title === title))) {
        prev.end = i;
        if (hasNote(i)) prev.noted = true;
        return;
      }
      if (kind === "section") inSec = true;
      if (kind === "title") inSec = false;
      out.push({ start: i, end: i, title: title || "슬라이드 " + (i + 1), kind, sub: isContent && inSec, noted: hasNote(i) });
    });
    return out;
  }
  // buildToc 은 목차 칸(머리·항목·오른쪽 경계)을 만든다. 슬라이드는 발표 동안 바뀌지 않으므로 시작할 때 한 번.
  function buildToc() {
    const toc = tocEl = root.appendChild(el("div", "pr-toc"));
    const head = toc.appendChild(el("div", "pr-toc-head"));
    head.appendChild(el("span", "pr-toc-title", "목차"));
    tocDockBtn = head.appendChild(el("button", "pr-toc-dock-btn"));
    tocDockBtn.type = "button";
    tocDockBtn.addEventListener("click", ev => { ev.stopPropagation(); setTocDock(!P.tocDock); tocDockBtn.blur && tocDockBtn.blur(); });
    paintTocDock();
    head.addEventListener("mousedown", onTocHeadDrag);
    tocBody = toc.appendChild(el("div", "pr-toc-body"));
    const showDots = notedCount() > 0;
    tocItems = tocEntries().map(en => {
      const b = tocBody.appendChild(el("button", "pr-toc-item k-" + en.kind + (en.sub ? " sub" : "")));
      b.type = "button";
      b.dataset.start = String(en.start);
      b.dataset.end = String(en.end);
      b.title = en.end > en.start ? "슬라이드 " + (en.start + 1) + "–" + (en.end + 1) : "슬라이드 " + (en.start + 1);
      b.appendChild(el("span", "pr-toc-n", String(en.start + 1)));
      b.appendChild(el("span", "pr-toc-t", en.title));
      if (showDots && en.noted) { const d = b.appendChild(el("span", "pr-toc-dot")); d.title = "노트 있음"; }
      // 누르면 그 슬라이드(묶음이면 첫 장)로 — 포커스를 단추에 남기지 않는다(발표 키는 어차피 발표가 갖는다)
      b.addEventListener("click", ev => { ev.stopPropagation(); jump(en.start); if (b.blur) b.blur(); });
      return { b, start: en.start, end: en.end };
    });
    const grip = root.appendChild(el("div", "pr-toc-grip"));
    grip.title = "끌어 목차 너비 바꾸기 — 두 번 누르면 기본 너비";
    grip.addEventListener("mousedown", onTocGrip);
    grip.addEventListener("dblclick", ev => { ev.stopPropagation(); setTocW(TOC_W_DEF); saveToc(); });
  }
  // markToc 은 지금 슬라이드가 든 항목을 강조하고 칸 안에서 보이게 굴린다.
  function markToc() {
    for (const it of tocItems) {
      const on = P.idx >= it.start && P.idx <= it.end;
      it.b.classList.toggle("on", on);
      if (on && P.showToc && it.b.scrollIntoView) it.b.scrollIntoView({ block: "nearest" });
    }
  }
  function saveToc() { if (opts.onToc) opts.onToc(P.tocDock, P.tocW); }
  function setTocW(pct) {
    P.tocW = Math.min(TOC_W_MAX, Math.max(TOC_W_MIN, pct));
    relayout();
  }
  // layoutToc 은 떠 있는 목차 카드를 화면 안(아래 줄 위)에 가두고 자리·높이를 --pr-toc-x·y·h 로 건다 — layoutCard 와 같은 규칙이다.
  function layoutToc() {
    if (!P.on || !root || !tocEl || P.tocDock || !P.showToc) return;
    const W = root.clientWidth, H = root.clientHeight, w = tocEl.offsetWidth, h = tocEl.offsetHeight;
    let x = P.tocCard ? P.tocCard.x : CARD_M, y = P.tocCard ? P.tocCard.y : CARD_M;
    x = Math.round(Math.max(0, Math.min(x, W - w)));
    y = Math.round(Math.max(0, Math.min(y, H - HUD_H - h)));
    if (P.tocCard) P.tocCard = { x, y };
    root.style.setProperty("--pr-toc-x", x + "px");
    root.style.setProperty("--pr-toc-y", y + "px");
    root.style.setProperty("--pr-toc-h", h + "px");
  }
  // onTocHeadDrag 는 목차 카드 머리(단추 밖) 끌기다 — onHeadDrag 와 같다(도킹이면 옮기지 않는다).
  function onTocHeadDrag(e) {
    if (e.button !== 0 || P.tocDock || e.target.closest("button")) return;
    e.preventDefault();
    e.stopPropagation();
    const r = root.getBoundingClientRect(), tr = tocEl.getBoundingClientRect();
    const dx = e.clientX - tr.left, dy = e.clientY - tr.top;
    root.classList.add("pr-moving");
    beginDrag(ev => { P.tocCard = { x: ev.clientX - r.left - dx, y: ev.clientY - r.top - dy }; layoutToc(); },
      () => { if (root) root.classList.remove("pr-moving"); });
  }
  // onTocGrip 은 목차 오른쪽 경계 끌기다: 목차 왼쪽 끝(도킹이면 창 왼쪽, 카드면 카드 왼쪽 — 끄는 동안 그대로)에서 마우스까지가 너비다
  // (창의 16~40%). 놓을 때 한 번 알린다(onToc).
  function onTocGrip(e) {
    if (e.button !== 0) return;
    e.preventDefault();
    e.stopPropagation();
    const w0 = P.tocW;
    const left = P.tocDock ? 0 : (P.tocCard ? P.tocCard.x : Math.round(Math.max(0, Math.min(CARD_M, root.clientWidth - tocEl.offsetWidth))));
    root.classList.add("pr-toc-dragging");
    beginDrag(ev => {
      const r = root.getBoundingClientRect();
      const e = Math.round((ev.clientX - r.left - left) / r.width * 1000) / 10;
      setTocW(bothDocked() ? prefFor(e, P.noteW, TOC_W_MIN, TOC_W_MAX) : e);
    }, () => {
      if (root) root.classList.remove("pr-toc-dragging");
      if (P.tocW !== w0) saveToc();
    });
  }
  function paintTocDock() {
    if (!tocDockBtn) return;
    tocDockBtn.textContent = P.tocDock ? "띄우기" : "붙이기";
    tocDockBtn.title = P.tocDock ? "목차를 슬라이드 위에 떠 있는 카드로(Shift+D) — 슬라이드는 제자리·제 크기, 머리를 끌어 옮긴다"
      : "목차를 왼쪽에 붙이기(Shift+D) — 슬라이드가 남은 자리로 줄어든다";
  }
  // setTocDock 은 떠 있는 카드(거짓)·왼쪽에 붙인 칸(참)을 고른다 — 슬라이드를 다시 맞추고 부른 쪽에 알린다(onToc — 기억).
  function setTocDock(on) {
    on = !!on;
    if (on === P.tocDock) return;
    P.tocDock = on;
    root.classList.toggle("pr-toc-dock", on);
    paintTocDock();
    relayout();
    saveToc();
  }
  function setToc(on) {
    P.showToc = on === undefined ? !P.showToc : !!on;
    root.classList.toggle("pr-toc-on", P.showToc);
    relayout();
    if (P.showToc) markToc();
  }

  function setBlack(on) { P.black = on === undefined ? !P.black : on; root.classList.toggle("pr-black-on", P.black); syncOverlay(); if (P.black && ink) ink.pause(); if (inkMounted) inkView.wake(); emitState(); }
  function setNotes(on) { P.showNotes = on === undefined ? !P.showNotes : on; root.classList.toggle("pr-notes-on", P.showNotes); relayout(); }

  // ---- 라이트박스: 슬라이드 안 이미지·표·그림을 눌러 크게 본다. 발표자 화면에만 있다 — 공유 따라가기(onSlide)로 보내지 않는다.
  // zoomTarget 은 누른 자리 t 에서 크게 볼 대상을 찾는다: 지금 슬라이드의 본문(.page-body) 안 ItdaLightbox.SEL, 로고가 아니고, 차례로 나타내기로
  // 아직 숨은(흐린 목록 항목 포함 — .pr-pending) 요소 안이 아닐 때만. 없으면 null(지금처럼 다음).
  function zoomTarget(t) {
    if (!t || !t.closest || !deck || !deck.contains(t)) return null;
    if (!window.ItdaLightbox) return null; // 크게 보기 부품이 없는 호스트 — 누르기는 지금처럼 다음
    const hit = t.closest(window.ItdaLightbox.SEL), cur = P.slides[P.idx];
    if (!hit || !cur || !cur.contains(hit) || !hit.closest(".page-body")) return null;
    if (hit.closest(".sc-logo, .sf-logo, .pr-pending")) return null;
    return hit;
  }
  // linkAt 은 누른 자리 t 에서 열 링크를 찾는다: 덱의 지금 슬라이드 안 a[href] 로, 아직 숨은 단계(.pr-pending) 안이 아니고 href 가
  // #·http·https·mailto·tel 일 때만(호스트의 링크 처리와 같은 판정 — 호스트 전용 앵커 ItdaPresentHost.ownLink 는 아니다). 없으면 null(파일 링크도 — 지금처럼 다음).
  function linkAt(t) {
    if (!t || !t.closest || !deck || !deck.contains(t)) return null;
    const a = t.closest("a[href]"), cur = P.slides[P.idx];
    if (!a || !cur || !cur.contains(a) || a.closest(".pr-pending")) return null;
    const href = a.getAttribute("href");
    return /^(#|https?:|mailto:|tel:)/i.test(href) && !hostCall("ownLink", href) ? a : null;
  }
  // attachLightbox 는 이 발표 root 에 라이트박스(호스트의 크게 보기 부품 ItdaLightbox — 그리기·크기 규칙·닫힘 페이드. 없으면 붙이지 않는다)를 붙인다. 쪽 카드는 지금 슬라이드, 배율은 덱 배율,
  // 표 번호는 덱에서 센다. 열고 닫을 때 P.lightbox(종류 "img"·"table"·"svg" — 닫혔으면 "")·P.lbZoom 을 채운다(호스트의 시험이 본다)
  function attachLightbox() {
    if (!window.ItdaLightbox) { lb = lbEl = null; return; }
    lb = window.ItdaLightbox.create(root, {
      scale: () => P.scale || 1,
      card: () => P.slides[P.idx],
      scope: () => deck,
      cls: "pr-lightbox", // 옛 표지 — ink.js BLOCKED(라이트박스 위 판서 입력 제외)·호스트의 YouTube 재생이 이 이름을 본다
      onOpen: (kind, k) => { P.lightbox = kind; P.lbZoom = k; digits = ""; root.classList.remove("pr-goto-on"); syncOverlay(); emitState(); },
      onClose: () => { P.lightbox = ""; syncOverlay(); emitState(); },
    });
    lbEl = lb.el;
  }
  function openLightbox(hit) { lb.open(hit); }
  // closeLightbox 는 라이트박스를 닫는다(페이드 — 크게 보기 부품). 슬라이드 번호·단계는 그대로다. 내용은 사라진 뒤 비운다.
  function closeLightbox() { if (lb) lb.close(); }

  // eatClick 은 라이트박스 안(복제본 — 표 안 링크 포함)에서 온 click·auxclick 과, 라이트박스를 열거나 닫은 누르기의 뒤따르는 click 을 먹는다.
  // 닫는 순간 P.lightbox 가 비므로 상태가 아니라 자리(lbEl 안)·깃발(lbEat — 다음 누르기에서 내린다)로 가른다.
  // 덱 안 링크의 click·auxclick 도 언제나 먹는다 — 창이 그 주소로 이동하지 않게. 링크는 그 링크 위에서 눌렀을 때(linkDown)만 연다:
  // 넘김으로 누른 자리에 늦게 나타난 링크(숨은 단계)는 뒤따르는 click 으로 열리지 않는다.
  // 겹침 층(호스트 요소) 안의 click·auxclick 은 건드리지 않는다 — 깃발(lbEat)도 그대로 둔다(층 밖 다음 click 의 몫이다).
  function eatClick(e) {
    if (!P.on) return;
    const t = e.target;
    if (inOverlay(t)) return;
    // 청중 열기: 덮개 안의 click·auxclick 은 모두 먹는다 — 덱 사본의 단추(섹션 묻기 단추 등)·링크가 아무것도 하지 않게(키보드 활성화도 click 으로 온다)
    if (P.audience) { if (root && root.contains(t)) { e.preventDefault(); e.stopPropagation(); } return; }
    // 크게 보기 부품이 없으면 lbEl 이 없다 — 판서가 건 lbEat·덱 안 링크는 그대로 먹는다
    if (lbEat || (lbEl && t && lbEl.contains(t))) {
      lbEat = false;
      e.preventDefault();
      e.stopPropagation();
      return;
    }
    const a = t && t.closest && deck.contains(t) ? t.closest("a[href]") : null;
    if (!a) return;
    const down = linkDown;
    linkDown = null;
    e.preventDefault();
    e.stopPropagation();
    if (e.type === "click" && a === down && opts.onLink) opts.onLink(a.getAttribute("href"));
  }

  function onMouse(e) {
    if (inOverlay(e.target)) return; // 겹침 층은 호스트 몫 — 넘김·판서·링크·크게 보기·커서 깨우기 모두 아니다
    wake();
    lbEat = false; // 지난 누르기의 깃발은 click 이 오지 않았어도(밖으로 끌어 놓음) 여기서 내린다 — 이 누르기의 click 을 잘못 먹지 않게
    linkDown = null; // 링크 표지도 같다 — 이 누르기의 click 만 그 링크를 연다
    // 라이트박스가 열린 동안 누르기(어느 단추든)는 닫기만 한다 — 넘기지 않는다. 라이트박스의 스크롤 막대를 누른 것은 닫기가 아니다.
    // 닫히는 페이드 동안(P.lightbox 는 이미 비었지만 라이트박스가 아직 보이고 누르기를 받는다 — 크게 보기 부품의 지연 visibility)의 누르기도
    // 라이트박스 안이면 아무것도 하지 않는다(빠르게 두 번 눌러 닫을 때 다음으로 새지 않게). 다 사라진 뒤에는 visibility:hidden
    // 이라 누르기가 라이트박스에 닿지 않는다.
    if (lbEl && lbEl.contains(e.target)) {
      if (lb.onScrollbar(e)) return;
      lbEat = true;
      if (e.preventDefault) e.preventDefault();
      closeLightbox();
      return;
    }
    if (P.lightbox) { lbEat = true; closeLightbox(); return; } // 라이트박스가 창을 덮으므로 밖에서 오는 누르기는 드물다(방어)
    if (P.grid || e.target.closest(".pr-notes, .pr-hud, .pr-notes-grip, .pr-toc, .pr-toc-grip, .pr-ink-bar, .itda-codecopy, .itda-codetitle")) return;
    // 판서가 링크보다 먼저다 — 펜·형광펜·지우개의 왼쪽 누르기는 링크 위에서도 그리기다: 뒤따르는 click 을 먹어 링크가 열리지 않는다.
    // 레이저는 떼는 때 ink.js 가 클릭인지 판단한다(inkClick). 보드 발표에는 판서가 없다(ink 없음)
    const inking = ink ? ink.takes(e) : "";
    if (inking) { if (inking === "draw") lbEat = true; return; }
    // 링크 위 왼쪽 클릭은 넘김·크게 보기가 아니라 링크 동작이다 — 여는 것은 뒤따르는 click(eatClick). 검은 화면에서는 아니다(지금처럼 다음)
    if (e.button === 0 && !P.black) { const a = linkAt(e.target); if (a) { linkDown = a; return; } }
    // 보드 발표: 그림·표는 라이트박스, 그 밖의 누르기는 아무것도 하지 않는다(다음 장이 없다)
    if (P.board) { if (e.button === 0 && !P.black) { const hit = zoomTarget(e.target); if (hit) { lbEat = true; openLightbox(hit); } } return; }
    pressAt(e.target, e.button);
  }
  // pressAt 은 슬라이드 위 누르기 하나다. 이미지·표·그림 위 왼쪽 클릭은 다음이 아니라 크게 보기. 검은 화면에서는 아니다(지금처럼 다음 — 화면이 가려져 있다)
  function pressAt(target, button) {
    if (button === 0 && !P.black) { const hit = zoomTarget(target); if (hit) { lbEat = true; openLightbox(hit); return; } }
    if (button === 0) next(); else if (button === 2) prev();
  }
  // attachInk 는 판서를 붙인다. 판서는 탭에 딸린다(opts.inkKey — ink.js bookFor) — 같은 탭에서 다시 발표해도 남는다.
  function attachInk(w, h) {
    const I = window.ItdaInk;
    if (!I) return;
    ink = I.attach({ root, deck, size: [w, h], book: I.bookFor(opts.inkKey), penOnly: P.penOnly,
      slides: () => P.slides, idx: () => P.idx, scale: () => P.scale, paused: () => !!(P.grid || P.lightbox || P.black),
      click: inkClick, eatClick: () => { lbEat = true; }, wake, settleTransition });
  }
  // inkClick 은 움직이지 않은 레이저 클릭이다 — 도구가 꺼진 클릭과 같다: 링크가 넘김·크게 보기보다 먼저다. 다만 여는 것은 뒤따르는 click(eatClick)이 아니라
  // 여기다: 레이저는 포인터를 잡고 있어 click 이 링크가 아니라 root 로 올 수 있다. 그 click 은 깃발로 먹는다
  function inkClick(target, button) {
    const a = button === 0 ? linkAt(target) : null; // 쉬는 화면의 누르기는 여기 오지 않는다(ink.js onUp)
    if (!a) { pressAt(target, button); return; }
    lbEat = true;
    if (opts.onLink) opts.onLink(a.getAttribute("href"));
  }

  // onKey 는 발표 키를 전부 삼킨다(capture — Tab 도). 발표 중 코드 복사 단추·파일명 띠 단추는 마우스로만 누른다: 발표 키는 단추에 포커스가
  // 있어도 발표가 갖고, 누른 뒤 포커스를 단추에 남기지 않는 건 복사 단추 쪽이 한다.
  // 호스트가 ownsKey 로 가져간 키는 맨 앞에서 놓아준다(머리 주석 「호스트 계약」 — 겹침 층을 가린 동안에도 묻는다).
  function onKey(e) {
    if (!P.on) return;
    if (hostCall("ownsKey", e)) return;
    e.preventDefault();
    e.stopPropagation();
    const k = e.key;
    // 판서 키는 조합 검사 앞이다 — 한글 입력 상태에서도 e.code 로 받는다. 숨겨진 상태(라이트박스·한눈에 보기·검은 화면)와 번호 지우기 Esc 는 아래가 먼저다
    if (ink && !P.lightbox && !P.grid && !P.black && !(digits && k === "Escape") && ink.key(e)) return;
    if (e.isComposing) return;
    // 라이트박스가 열린 동안: Esc·Space·Enter 는 닫기만(발표를 끝내거나 넘기지 않는다), 다른 키는 삼킨다
    if (P.lightbox) { if (k === "Escape" || k === " " || k === "Enter") closeLightbox(); return; }
    if (P.board) { if (P.black && k !== "b" && k !== "B" && k !== ".") setBlack(false); else boardKey(k); return; }
    if (P.grid) {
      const cols = Number(gridEl.style.getPropertyValue("--cols")) || 4;
      if (k === "Escape" || k === "g" || k === "G") toggleGrid(false);
      else if (k === "ArrowRight") gridSel = Math.min(P.slides.length - 1, gridSel + 1);
      else if (k === "ArrowLeft") gridSel = Math.max(0, gridSel - 1);
      else if (k === "ArrowDown") gridSel = Math.min(P.slides.length - 1, gridSel + cols);
      else if (k === "ArrowUp") gridSel = Math.max(0, gridSel - cols);
      else if (k === "Enter" || k === " ") { toggleGrid(false); jump(gridSel); return; }
      markGrid();
      return;
    }
    if (/^[0-9]$/.test(k)) { digits = (digits + k).slice(-4); gotoEl.textContent = digits + " ↵"; root.classList.add("pr-goto-on"); return; }
    if (k === "Enter" && digits) { const n = parseInt(digits, 10); digits = ""; root.classList.remove("pr-goto-on"); jump(n - 1); return; }
    if (digits && (k === "Escape" || k === "Backspace")) { digits = ""; root.classList.remove("pr-goto-on"); return; }
    if (P.black && k !== "b" && k !== "B" && k !== ".") { setBlack(false); return; }
    switch (k) {
      case "ArrowRight": case "ArrowDown": case " ": case "PageDown": case "Enter": next(); break;
      case "ArrowLeft": case "ArrowUp": case "PageUp": case "Backspace": prev(); break;
      case "Home": jump(0); break;
      case "End": jump(P.slides.length - 1); break;
      case "g": case "G": toggleGrid(true); break;
      case "b": case "B": case ".": setBlack(); break;
      case "n": case "N": setNotes(); break;
      case "+": case "=": if (P.showNotes) stepFont(1); break;
      case "-": case "_": if (P.showNotes) stepFont(-1); break;
      case "t": case "T": setToc(); break;
      // Shift+D 는 목차 칸(편 동안), D 는 노트 칸(편 동안) — 목차를 접었으면 Shift+D 는 무시한다(노트로 넘기지 않는다)
      case "d": case "D": if (e.shiftKey) { if (P.showToc) setTocDock(!P.tocDock); } else if (P.showNotes) setDock(!P.noteDock); break;
      case "Escape": stop(); break;
    }
  }

  // boardDeck 은 보드 발표 중이면 덱({root, scroller}), 아니면 null 이다 — 공유 위치를 재는 쪽(호스트)이 쓴다.
  function boardDeck() { return P.on && P.board ? { root: deck, scroller: stage } : null; }

  // current 는 지금 발표의 {on, index, total} 이다(발표 중 공유를 시작했을 때 지금 슬라이드를 한 번 보내려고 — 호스트).
  function current() { return { on: P.on, index: P.on ? P.idx : -1, total: P.slides.length }; }

  // setShareSummary 는 아래 줄의 공유 미션 숫자 요약이다(호스트가 만든 글 — 이름은 싣지 않는다). 발표 밖에서 와도 다음 발표가 쓴다.
  function setShareSummary(text) {
    shareSummary = text || "";
    if (shareEl) shareEl.textContent = shareSummary;
  }
  // setPenOnly 는 발표 중 펜만 판서를 바꾼다(머리 주석 「펜만 판서」 setPenOnly).
  function setPenOnly(b) {
    if (!P.on || P.board || P.audience) return false;
    P.penOnly = !!b;
    if (root) root.classList.toggle("pr-pen-only", P.penOnly);
    if (ink) ink.setPenOnly(P.penOnly);
    return true;
  }
  // setNextPreview·setClock 은 발표 중 발표자 보조를 바꾼다(머리 주석 「발표자 보조」).
  function presenterOn() { return P.on && !P.board && !P.audience && !!root; }
  function setNextPreview(b) {
    if (!presenterOn()) return false;
    P.nextPreview = !!b;
    root.classList.toggle("pr-next-on", P.nextPreview);
    paintNext();
    relayout(); // 카드 높이·미리보기 크기
    return true;
  }
  function setClock(b) {
    if (!presenterOn()) return false;
    P.clock = !!b;
    root.classList.toggle("pr-clock-on", P.clock);
    if (clockEl) clockEl.textContent = P.clock ? clockText(new Date()) : "";
    return true;
  }
  window.ItdaPresent = { setShareSummary, setPenOnly, setNextPreview, setClock, start, startBoard, boardDeck, boardBarWidth, stop, next, prev, jump, show, toggleGrid, setBlack, setNotes, setDock, setToc, setTocDock, closeLightbox, current, ink: () => ink, state: P,
    // inkView 는 청중 화면이 발표자 판서를 받아 그리는 view(ink.js view)다 — 머리 주석 「두 화면 발표」 판서 받기. ink.js 가 없으면 null
    inkView: inkViewOf,
    // inkHeld 는 판서 입력이 진행 중인가다(머리 주석 「펜만 판서」) — 호스트가 손가락 밀기를 버릴지 정할 때 묻는다
    inkHeld: () => !!(P.on && ink && ink.held()) };
})();
