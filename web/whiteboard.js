// whiteboard.js — 화이트보드 표면. 전역은 window.ItdaWhiteboard 다. 순수 함수·파일 형식은 wbcore.js(window.ItdaWbCore — 먼저 싣는다).
// 어느 화면(문서·발표) 위에서든 화면 전체를 덮는 끝없는 판을 열고 닫는다. 외부 라이브러리·네트워크 요청이 없다 — 판은 호스트 저장 계약으로만 나간다.
//
// 공개 함수
//   ItdaWhiteboard.open({ host?, id? }) → Promise<id|null>  표면(<div class="itda-wb" data-pr-overlay>)을 host.mount(없으면 document.body)에 붙이고
//       판을 읽는다. host 가 없으면 window.ItdaWhiteboardHost 다 — 그것도 없거나 store 가 없으면 거부한다(표면을 만들지 않는다).
//       id 가 없으면 이 페이지에서 그 store 로 마지막에 쓴 판, 그것이 목록에 없으면 고친 때가 가장 늦은 판, 목록이 비었으면 새 판을 만든다(store.create).
//       이미 열려 있으면 연 때의 Promise 를 그대로 돌려준다. data-pr-overlay 는 발표(present.js 「겹침 층」) 중에도 보이게 하는 표지다.
//       열기를 요청한 때부터 STORE_WAIT(8초) 안에 첫 판이 다 뜨지 않으면(list·load·create·새 판의 첫 save·목록 다시 읽기를 합쳐) 표면을 떼고
//       Promise 를 거부한다 — onClose({ unsaved, failed: true }) 를 부르고 안내는 호스트 몫이다(unsaved 는 아래 onClose 와 같다 — 보통 false,
//       새 판의 빈 본문 save 가 기한을 넘겼거나 앞 세션이 남긴 미저장 본문이 있으면 true). 기한 안에 온 list·load 실패는 예전처럼 읽기 전용으로 연다.
//   ItdaWhiteboard.close() → Promise<boolean>  진행 중 획을 확정하고 잠근 뒤 그 자리에서 지금 판을 기록기에 넣고, 저장이 확인되면 표면을 뗀다(true).
//       저장이 거부되면 처음 한 번은 닫지 않고 안내만 한다(false) — 한 번 더 닫으면 다시 저장해 보고, 또 거부되면 본문을 기록기(이 페이지의 메모리)에
//       남긴 채 닫는다(같은 store 객체로 같은 판을 다시 열면 그 본문으로 열고 다시 저장한다). 「처음 한 번」 은 저장이 한 번 되거나 판을 바꾸면 다시 센다.
//       닫기를 요청한 때부터 STORE_WAIT(8초) 안에 확인되지 않으면 첫 닫기라도 본문을 기록기에 남긴 채 닫는다(앞 일 — 전환 등 — 은 기다리지 않는다).
//       판에 좌표 범위 밖 점이 있어 본문을 만들지 못하면 닫지 않는다(false — 닫으면 판을 잃는다).
//   ItdaWhiteboard.isOpen() · ItdaWhiteboard.toggle(opts)(열렸으면 close, 아니면 open(opts))
//   ItdaWhiteboard.unsaved(store) → string[]  그 store 객체에서 저장 성공을 아직 확인하지 못한 기록기 본문이 있는 판 id 의 새 배열(차례에 뜻 없음).
//       동기·읽기 전용이다 — 저장을 시작하지도 기록기·저장소별 상태를 만들거나 바꾸지도 않는다. 표면이 열려 있는지와 상관없이 그 store 의 기록기를 모두 본다.
//       판별은 아래 「기록기」 의 미저장(가장 새 번호 > done — 도는 save 만 있어도, 그 뒤 더 새 스냅샷이 있어도 든다)이고, 삭제가 된 판(gone)은 뺀다.
//       「디스크에 없음」 이 아니라 「이 store 의 save 성공을 확인하지 못함」 이다. 엔진에만 있는 변경(아직 기록기에 넣지 않은 획 — 자동 저장을 기다리거나
//       본문을 만들지 못한 것)은 들지 않는다 — 열린 표면의 마지막 획까지 지키려면 close() 가 true 인 것을 먼저 확인한다. 이 페이지에서 쓴 적 없는
//       store 는 빈 배열이고, store 가 객체(함수 포함)가 아니면 TypeError 를 던진다.
//   ItdaWhiteboard.memoryStore()  메모리 저장소(아래 store 계약을 그대로 따른다 — 시험·데모용, 페이지를 닫으면 사라진다).
//   ItdaWhiteboard.debug()  열린 동안의 엔진 손잡이(시험·진단용 — 계약 아님). 닫혀 있으면 null.
//
// 저장 계약(정본) — 호스트는 window.ItdaWhiteboardHost = { store, mount?, onOpen?, onClose? } 를 준다.
//   store 의 함수는 모두 Promise 를 돌려준다(값을 바로 돌려줘도 Promise.resolve 로 받는다). 던지거나 거부하면 실패로 본다.
//     list()            → [{ id, name, modified(ms), bytes }]   차례는 상관없다 — 표면이 고친 때 내림차순으로 정렬한다.
//     load(id)          → 파일 본문 문자열(wbcore.js 「파일 형식 v1」). 빈 문자열이면 빈 새 판으로 연다(create 가 빈 파일을 만드는 호스트를 위해).
//     save(id, text)    → { modified }                            text 는 serialize 결과다. 표면은 이 id 의 본문을 통째로 바꿔 쓴다고 본다.
//     create(name)      → { id }                                  표면은 만든 곧바로 빈 판 본문을 save 한다.
//     rename(id, name)  · duplicate(id) → { id } · trash(id)      trash 는 목록에서 빼는 것이다(휴지통 등 되살리기는 호스트 몫).
//   id 는 호스트가 정하는 불투명 문자열이다. 표면은 경로를 만들거나 해석하지 않는다. 이름·크기·숫자 범위의 최종 검사는 호스트 몫이다.
//   mount  표면을 붙일 요소(없으면 document.body). onOpen()·onClose({ unsaved, failed? })  표면을 붙인 뒤·뗀 뒤 한 번씩 부른다(예외는 삼킨다).
//       failed 가 true 면 처음 열기가 기한을 넘겨 표면을 뗐다는 뜻이다(open 의 Promise 도 거부한다).
//       unsaved 가 true 면 저장하지 못한 판 본문을 이 페이지의 메모리(기록기)에 남긴 채 닫았다는 뜻이다(호스트가 안내한다 — 페이지를 닫으면 사라진다).
//   호스트가 지킬 것(표면은 이것을 믿는다 — 어기면 아래 보장이 없다):
//     · 페이지가 사는 동안 같은 저장소에는 같은 store 객체를 준다. 저장 상태는 store 객체마다 따로라, 같은 저장소를 새 객체로 감싸 주면 앞 객체의
//       미저장 본문·도는 save 를 잇지 않는다(새 객체는 저장소의 본문으로 열고, 앞 객체의 늦은 save 가 그 뒤에 끝나면 덮을 수 있다).
//     · store 함수의 Promise 가 이루어지거나 거부되면 그 요청(쓰기)은 정말 끝났다 — 거부한 뒤 뒤에서 쓰기를 계속하지 않는다.
//     · 한 판은 한 실행 환경에서 한 편집자다 — 두 탭·두 페이지가 같은 판을 함께 고치는 것은 지키지 않는다(늦게 쓴 쪽이 이긴다).
//
// 기록기(저장 요청의 수명): store 객체마다, 판 id 마다 하나 — 세션(열린 표면)을 닫고 다시 열어도 이어진다. 세션은 스냅샷을 넣고(put) 지켜볼(watch) 뿐이다.
//   · put 은 기록기가 내는 단조 증가 번호를 돌려준다. 아직 내지 않은 스냅샷(want)은 가장 새 것 하나로 합친다. 같은 본문이면 번호를 새로 내지 않는다.
//   · 한 판의 store.save 는 늘 하나만 돈다(running — 부르기 전에 잡는다). 끝나면 done 을 그 스냅샷의 번호까지 올리고 더 새 want 를 곧바로 낸다.
//     번호 n 의 성공은 done ≥ n 이다(합친 중간 번호는 따로 쓰이지 않는다 — 「n 뒤의 가장 새 본문이 저장됐다」 는 뜻).
//   · 거부(동기 throw 포함)면 want 를 둔 채 멈춘다 — 스스로 되풀이하지 않고, 다음 변경·떠나기·다시 열기 때 retry 로 다시 낸다.
//   · 시간 넘김·세션 닫힘은 도는 save 를 건드리지 않는다 — 그 요청이 끝날 때까지 그 판의 다음 save 를 내지 않는다(앞지르면 늦은 save 가 덮는다).
//   · 미저장의 정본은 기록기다(가장 새 번호 > done). 그런 판을 다시 열면 그 본문으로 열고 다시 저장한다. 깨끗한 기록기는 도는 save·지켜보는 쪽·
//     연 세션·관리 작업이 없을 때 치운다. 삭제가 된 판의 기록기는 폐기한다(남은 본문을 버리고 옛 put·retry 를 받지 않는다).
//   · 폐기(gone)된 기록기를 든 표면은 잠겨 있다(불변식 — unlock 이 풀지 않는다). 삭제가 언제 되든(요청한 세션이 닫혔어도, 기한을 넘겨 늦게 되어도)
//     그 판을 지금 보고 있는 세션이 곧바로 받아(bury → buried) 진행 중 획을 확정하고 잠근 뒤 「이 화이트보드는 삭제되었습니다」 를 보이고, 도는 시도가
//     없으면 다음 판(남은 판 중 고친 때가 가장 늦은 판, 없으면 새 판)을 새 시도로 연다 — 시도가 있으면 그 시도가 끝난 뒤(end) 옮긴다.
//     삭제가 된 판에 삭제 전까지 그은 미저장 획(막혀 있어 저장하지 못한 것)은 삭제와 함께 버려진다 — 사용자가 그 판을 지우라고 했다.
// 떠나기·기다림: 떠나는 모든 길(닫기·전환·새 판·복제·삭제·숨김)은 그 자리에서 진행 중 그리기·지우기를 확정(commit)하고 — 떠나는 길이면 잠그고 —
//   본문을 기록기에 넣은(put) 뒤 기다림만 뒤로 보낸다. 자동 저장 타이머도 put 만 한다(앞 저장이 도는 동안이면 끝난 뒤 기록기가 낸다).
//   처음 열기·전환·새 판·삭제 뒤 다음 판 열기·닫기·관리 작업은 시도 하나다(한 번에 하나 — 닫기는 앞 시도를 밀어낸다). 시도마다 요청한 때부터
//   STORE_WAIT(8초) 하나의 기한이 있다(앞 저장 확인·list·save·load·create 와 새 판의 첫 save·목록 다시 읽기를 합쳐 — 새 판 만들기(newBoard)는 판을 보인
//   뒤의 첫 save·목록 읽기를 시도 밖에서 따로 기다린다). 처음 열기가 기한을 넘기면 표면을 닫고 open 을 거부한다(open). 삭제 뒤 다음 판 열기가 넘기면
//   잠근 채 「판을 열지 못했습니다」 와 목록을 보여 고르게 한다(늦은 load·create 는 버린다). 전환·새 판은 떠나는 판의 저장이 확인된 뒤에만 대상을 읽거나 만든다 — 저장이 거부되거나, load·create 가
//   실패하거나, 기한을 넘기면 원래 판에 남고(잠금을 풀고 안내) 그 시도의 늦은 load·show·unlock 은 버린다(시도 토큰). 닫힌 세션의 늦은 완료는 표면의
//   DOM·상태를 건드리지 않는다(세션 토큰 alive) — 저장 결과는 기록기가 받는다.
// 관리 작업(이름·복제·삭제): 시작 전에 관련 판(지금 판이면) 편집을 잠근다. 요청이 도는 동안 그 판의 기록기를 막아 새 save 를 내지 않는다 —
//   기한을 넘겨도(「결과를 확인하지 못했습니다」 안내) 그 요청이 끝날 때까지 막는다.
//   · 복제는 그 판의 기록기가 깨끗해진 뒤에만 부른다 — 미저장이면 다시 저장해 보고, 기한 안에 깨끗해지지 않으면 철회한다(뒤늦게 부르지 않는다).
//   · 삭제는 요청한 그 자리에서 기록기를 막고, 도는 save 가 끝난 뒤에만 부른다 — 기한 안에 시작하지 못하면 철회한다. 되면 기록기를 폐기하고
//     그 판을 보는 세션에 알린다(위 「기록기」 불변식).
//   · 이름 바꾸기가 되면 그 판의 본문 이름도 맞춘다(열린 판이면 엔진 상태로 put, 닫힌 판에 미저장 본문이 있으면 그 본문의 name). 판을 열 때는
//     목록(호스트)의 이름을 먼저 쓴다.
// 한계: 끝나지 않는 save 가 있으면 그 판의 새 본문은 저장되지 않고 기록기(메모리)에만 있다(안내한다). 페이지를 닫으면 미저장 본문은 사라진다.
//   두 탭 동시 편집·store 객체 교체는 지키지 않는다(위 「호스트가 지킬 것」). 기한을 넘긴 create 가 늦게 이루어지면 호스트에 빈 판이 남을 수 있다.
//   숨겨져 멈춘 페이지·막힌 메인 스레드에서 8초 기한이 벽시계대로 지켜진다고 보장하지 않는다.
// 잠금: 판을 바꾸는 동안(떠나기 시작부터 새 판이 다 뜰 때까지 — 전환·새 판·지금 판 삭제·처음 열기), 지금 판의 관리 작업 동안, 닫는 동안,
// 지금 판이 삭제된 뒤(다음 판이 뜰 때까지 — 다음 판 열기가 기한을 넘기면 목록에서 고를 때까지) 표면은
// 「여는 중」·「닫는 중」 등을 보이고 그리기·지우기·되돌리기·배경 바꾸기·그림 넣기·옮기기·지우기를 받지 않는다(이동·확대는 된다 — 펜·마우스 끌기도 이동.
// 잠글 때 진행 중 그림 조작은 확정하고 고르기를 푼다).
//
// 저장 때(save): 판이 바뀐 뒤 SAVE_QUIET(2초) 조용하면, 그리고 판 전환·새 판·복제·닫기·visibilitychange(hidden)·pagehide 때(숨김은 처리기 안에서
// 곧바로 commit·put 한다). 저장이 확인됐을 때 자동 저장을 기다리는 변경이 있으면 2초를 기다리지 않고 곧바로 한 번 더 한다.
// 자동 저장(2초 타이머)과 그 「곧바로 한 번 더」 는 포인터를 쥔 그리기·지우기·그림 조작 중(엔진 busy)이면 부르지 않고 미룬다 — 쓰는 중 획을
// 끊지 않는다(뗀 뒤의 변경이 타이머를 새로 건다). 떠나는 길·숨김은 미루지 않고 그 자리에서 진행 중 획을 확정한다. 실패하면 표면 위 한 줄 안내(.itda-wb-note)를 보이고 판은 화면과 기록기에 그대로 두며, 다음 변경 때 다시 한다.
// 판에 좌표 범위 밖 점이 있어 본문을 만들지 못하면(wbcore serialize — 내부 불변식이 깨졌을 때) 저장을 멈추고 판은 엔진에 둔 채 안내한다.
// 저장소 구분: 기록기·마지막 판·미리보기 캐시는 store 객체마다 따로다 — 같은 id 를 쓰는 두 store 가 섞이지 않는다.
// 미리보기: 다른 판은 본문을 읽어 작은 canvas 에 그리고 그 canvas 만 캐시한다(store 마다 최대 40개, 오래 쓰지 않은 것부터 버림, 같은 판 요청은 하나로).
//   그림은 하나씩 디코드해 그리고 곧바로 close 한다 — 디코드가 안 되는 그림은 옅은 사각형이다.
// 상한: 판 하나는 wbcore LIMIT(획·점·그림 수·그림 바이트) 안, 진행 중 획 하나는 STROKE_PTS(2만) 점까지 — 닿으면 그 획을 끝내고 안내한다(떼고 다시 그어야 새 획).
//   든 bitmap 의 화소 합 상한은 없다 — 판의 그림 200개(LIMIT.images)와 되돌리기·다시 하기 더미에서 닿는 지운 그림까지, 그림 하나는 긴 변 IMG_SIDE(2000px)
//   까지다(파일에서 읽은 큰 그림도 디코드할 때 줄여 든다 — 줄이지 못하면 원래 크기). 화면 밖 그림의 bitmap 도 놓지 않는다.
// CSP: style 속성 문자열·HTML 문자열을 쓰지 않는다(CSSOM 대입 el.style.x = … 과 클래스만 — style-src 'self' 에서 된다).
//   그림은 data URL 을 fetch·<img src> 로 읽지 않고 blob URL 도 만들지 않는다 — base64 를 바이트로 풀어 Blob → createImageBitmap(아래 「그림」).
// 「바뀜」 은 획·배경·이름·그림(넣기·옮기기·크기·지우기)이다. 보기(이동·확대)만 바뀐 것은 저장을 부르지 않고 다음 저장·전환·닫기 때 함께 싣는다.
// 읽기 전용 — 파일의 v 가 이 판보다 크거나, 본문을 읽지 못했으면(형식·검증 실패, load 실패) 보기만 하고 그리기·배경 바꾸기·그림 넣기·옮기기·저장을 하지 않는다.
//
// 입력(엔진 — 화이트보드 기획 ⓪ 스파이크 시제품 wb.js 그대로): Apple Pencil(pen) = 그리기(필압 굵기), 손가락 하나 = 이동, 둘 = 오므려 확대·이동,
// 펜으로 쓰는 중 닿은 손가락(손바닥)과 펜이 닿은 순간 이미 닿아 있던 손가락은 뗄 때까지 무시, 마우스 끌기 = 그리기(속도 굵기), 휠 = 이동,
// ⌃·⌘+휠(트랙패드 오므리기)·Safari gesture = 커서 기준 확대, Space+끌기·가운데 단추 = 이동. 좌표 지킴이: coalesced 점이 앞 점과 이 사건의 자리에서
// 크게 벗어나면, 예측(predicted) 점이 마지막 점에서 PRED_MAX_PX 넘게 떨어지면 버린다. 예측 꼬리는 펜에만 쓴다(마우스·손가락은 쓴 자리만).
// 점은 쓴 그대로 저장하고(평활 없음), 그릴 때만 curve(centripetal Catmull-Rom — 받은 점을 그대로 지난다)로 잇는다.
// 좌표 범위: 보기는 화면 전체가 ±COORD_MAX 안이도록 묶는다(wbcore boundView — 이동·확대·크기 바꾸기·판 읽기). 그래도 범위 밖 점(화면 밖으로 붙잡은
// 포인터 등)이 오면 — 쓴 점·coalesced·뗀 자리 어느 길이든 — 그 획을 거기서 끝내고 안내한다(다시 들어와도 잇지 않는다). 첫 점부터 밖이면 획을 만들지 않는다.
// 범위 밖 예측 꼬리는 그리지 않는다.
// 그림: 판에 그림을 넣는다 — ⌘V(paste 사건 — 입력 칸에 초점이 없을 때 클립보드의 첫 image/* 하나, 그림이 없으면 아무 일도 하지 않는다)와
//   도구 막대 「그림」(<input type="file" accept="image/*"> — iPad 에는 ⌘V 가 없다). 넣을 때 디코드(createImageBitmap) → 긴 변이 IMG_SIDE(2000px)를
//   넘으면 줄임 → 다시 인코드(투명 화소가 있으면 PNG, 없으면 JPEG 0.85) → 바이트가 IMG_BYTES(1.5MB)를 넘으면 긴 변을 0.75 배씩 줄여 다시(4번까지 —
//   그래도 넘으면 넣지 않고 안내). 판 하나의 그림 수·바이트 합 상한(wbcore LIMIT.images 200·imageBytes 24MB)을 넘으면 넣지 않고 안내한다.
//   놓는 자리는 화면 가운데, 처음 크기는 화면 px 로 긴 변 IMG_SHOW_PX(480 — 원래 그림이 더 작으면 원래 크기)이고 왼쪽 위와 폭을 격자(GRID 32 판 단위)에
//   맞춘다(높이는 비율대로). 넣은 뒤 고르기 도구로 바꾸고 그 그림을 고른다. 읽기 전용·잠금이면 넣지 않는다. 그림은 판 파일 안에 들어간다(wbcore
//   「images」 — 저장·복제는 본문을 따른다). 붙여넣은 그림은 판 본문으로만 나간다(네트워크 요청이 없다). 그림은 늘 획 아래에 그린다(그 위에 쓴다 —
//   그림 층이 획 층 아래다. 옮기기·크기 조절 중에도 그림끼리·획과의 차례는 같다).
// 고르기 도구(V): 그림을 누르면 고른다(테두리 + 네 귀 손잡이 — 화면 px 크기). 빈 곳을 누르면 풀고, 펜·마우스로 빈 곳을 끌면 판 이동이다.
//   겹친 그림은 나중에 넣은 것이 잡힌다. 몸통 끌기 = 옮기기(왼쪽 위가 격자에 붙는다), 귀 끌기 = 크기(비율을 지키고 맞은편 귀는 고정, 폭이 격자 배수 —
//   최소 격자 한 칸). 결과가 저장할 수 있는 자리(x·y·x+w·y+h 가 좌표 범위 안, w·h 가 IMG_MIN 이상)인 것이 격자보다 먼저다 — 고정한 귀를 둔 채
//   그런 결과가 없으면(좌표 끝의 작은 그림 등) 그 움직임은 적용하지 않고, 뗄 때도 한 번 더 보아 아니면 제자리로 둔다. 옮기기·크기 조절 동안은 바탕이 격자가 아니어도 옅은 격자를 보인다. 고른 그림 옆 「지우기」·Delete·Backspace 로 지운다.
//   그림 조작은 펜·마우스다 — 손가락 하나는 고르기 도구에서도 이동이고, 고르기 도구일 때에 한해 그림(손잡이) 위에서 시작한 손가락 하나만 그림 조작이다
//   (펜이 없는 기기를 위해 — 빈 곳을 톡 치면 고르기를 푼다). 두 손가락은 확대(하던 그림 조작은 제자리로). 지우개는 그림을 지우지 않는다.
//   넣기·옮기기·크기·지우기는 저마다 되돌리기 한 단계다. 끌기 동안은 저장하지 않는다(뗀 뒤 자동 저장 타이머로).
// 키(열린 동안 — 창 붙잡기 단계에서 표면이 먼저 받고 아래 화면으로 보내지 않는다): P 펜 · H 형광펜 · E 지우개 · V 고르기 · Delete·Backspace 고른 그림 지우기 ·
// ⌘Z(Windows Ctrl) 되돌리기 · ⇧⌘Z·⌘Y 다시 하기 · ⌘V 붙여넣기(이 키만 기본 동작을 막지 않는다 — paste 사건이 와야 한다) · [ ] 이전·다음 판(목록 차례) · W 닫기 ·
// Esc 한 겹씩(고른 그림 풀기 → 이름 고치기·삭제 확인 → 목록 → 팝오버 → 닫기) · Space 누른 채 끌기 = 이동.
// 포인터·휠·터치·클릭도 표면 안의 것은 창 붙잡기 단계에서 표면이 처리하고 전파를 멈춘다(아래 화면의 처리기는 받지 않는다).
(function () {
  "use strict";
  const C = window.ItdaWbCore;
  const TILE = 512;              // 타일 한 변(장치 px)
  const MAX_TILES = 140;         // 두 단계 합쳐 캐시 상한(512² × 4B = 1MB 씩)
  const BAKE_BUDGET = 8;         // 프레임마다 타일 굽기 시간 예산(ms)
  const ZOOM_SETTLE = 160;       // 배율이 이만큼(ms) 멈추면 새 단계로 다시 굽는다
  const MIN_STEP_PX = 1;         // 앞 점과 화면 px 로 이보다 가까운 점은 거른다(확대와 무관)
  const ERASE_PX = 10;           // 지우개 반지름(화면 px)
  const PRED_MAX_PX = 40;        // 예측 꼬리가 마지막 점에서 뻗을 수 있는 화면 px
  const FINGER_DRAW = false;     // 손가락 하나로 그리기(시제품의 토글 — 제품은 꺼 둔다: 손가락 하나는 이동)
  const WIDTHS = { pen: [2, 4, 8], hl: [14, 24, 40] }, WIDTH_RANGE = { pen: [1, 20], hl: [6, 80] }; // 굵기 3단·막대 범위(화면 px — 그릴 때 배율로 판 단위가 된다)
  const COLORS = ["ink", "#ff453a", "#ff9f0a", "#ffd60a", "#30d158", "#0a84ff", "#bf5af2", "#ff4fa3"]; // 0 「잉크」 는 바탕을 따른다
  const GRID = 32;               // 격자 칸(판 단위)
  const SAVE_QUIET = 2000;       // 바뀐 뒤 이만큼(ms) 조용하면 저장한다
  const THUMB_W = 160, THUMB_H = 100, THUMB_MAX = 20000; // 목록 미리보기 크기(CSS px)·그리는 획 상한
  const THUMB_CACHE = 40;        // 미리보기 캐시 상한(store 마다 — 넘으면 오래 쓰지 않은 것부터 버린다)
  const STORE_WAIT = 8000;       // store 요청 하나를 기다리는 끝(ms) — 넘으면 실패로 본다(머리 주석 「기다림」)
  const STROKE_PTS = 20000;      // 진행 중 획 하나의 점 상한 — 닿으면 그 획을 끝낸다(떼고 다시 그어야 새 획)
  const IMG_SIDE = 2000;         // 넣는 그림의 긴 변 상한(px) — 넘으면 줄여 다시 인코드한다
  const IMG_BYTES = 1.5 * 1024 * 1024; // 그림 하나의 인코드 바이트 상한 — 넘으면 긴 변을 IMG_SHRINK 배씩 줄여 다시(IMG_RETRY 번까지)
  const IMG_SHRINK = 0.75, IMG_RETRY = 4, IMG_JPEG_Q = 0.85;
  const IMG_SHOW_PX = 480;       // 넣은 그림의 처음 크기 — 화면 px 로 긴 변 최대(원래 크기가 더 작으면 원래 크기)
  const HANDLE_PX = 5;           // 고른 그림 귀 손잡이 반 변(화면 px — 확대와 무관)
  const HIT_PX = { mouse: 10, pen: 14, touch: 24 }; // 손잡이를 잡는 반지름(화면 px)
  const BG_NAMES = { dark: "검정", light: "흰색", grid: "격자" };
  const SAVE_FAIL = "저장하지 못했습니다 — 판은 이 화면에 그대로 있고, 다음에 바꾸면 다시 저장합니다.";
  const RANGE_FAIL = "판에 좌표 범위 밖의 점이 있어 저장하지 못했습니다 — 판은 이 화면에 그대로 있습니다(마지막 획을 되돌린 뒤 다시 해 보세요).";
  const IMG_READ_FAIL = "그림을 읽지 못해 넣지 않았습니다.";
  const IMG_BIG_FAIL = "그림을 줄여도 " + IMG_BYTES / 1024 / 1024 + "MB 를 넘어 넣지 않았습니다.";

  // 도구·색·굵기는 판이 아니라 사람의 것이다 — 페이지가 살아 있는 동안 판을 바꾸거나 닫았다 열어도 기억한다(도구마다 따로).
  const prefs = { tool: "pen", color: { pen: 0, hl: 3 }, width: { pen: 4, hl: 24 } };
  let S = null;                  // 열린 표면(세션)

  // ---- 기록기(머리 주석 「기록기」) — 저장 요청의 수명은 세션이 아니라 여기 있다. 세션은 넣고(put·retry) 지켜볼(watch) 뿐이다.
  // 저장소마다 따로 두는 것(store 객체가 열쇠 — 같은 id 를 쓰는 두 저장소가 섞이지 않는다. 객체가 바뀌면 잇지 않는다):
  //   lastId  이 페이지에서 그 저장소로 마지막에 쓴 판 · recs  id → 기록기 · thumbs  id → { modified, canvas } 목록 미리보기(그린 결과만)
  //   seq  기록기 번호(receipt)를 내는 단조 증가 수 — 저장소 하나에서 줄지 않는다(세션을 닫고 다시 열어도 이어 센다)
  const bags = new WeakMap();
  function bagOf(store) {
    let b = bags.get(store);
    if (!b) bags.set(store, (b = { store, lastId: null, recs: new Map(), thumbs: new Map(), seq: 0 }));
    return b;
  }
  // 기록기 r(판 하나): want  { text, n } 아직 내지 않은 가장 새 스냅샷(새 put 이 덮어 합친다) · running  { text, n } 지금 도는 store.save(하나뿐)
  //   done  끝난 스냅샷의 번호 중 가장 큰 것 · saved  저장소에 있다고 아는 본문(같은 본문 put 은 번호를 새로 내지 않는다) · halted  거부 뒤 멈춤
  //   blocked  관리 작업(이름·복제·삭제)이 막은 수 — 0 이 아니면 새 save 를 내지 않는다 · editors  이 판을 연 세션 수 · gone  삭제됨(put·retry 를 받지 않는다)
  //   waiters  지켜보는 쪽(watch·idle)
  function recOf(B, id) {
    let r = B.recs.get(id);
    if (!r) B.recs.set(id, (r = { B, id, want: null, running: null, done: 0, saved: null, modified: NaN, bytes: 0, halted: false, blocked: 0, editors: 0, gone: false, waiters: new Set() }));
    return r;
  }
  const latest = r => (r.want ? r.want.n : r.running ? r.running.n : r.done);
  // dirty 는 저장하지 못한 본문이 있는가다(미저장의 정본 — 가장 새 번호가 done 보다 크다). latestText 는 그 본문이다.
  const dirty = r => !!r && !r.gone && latest(r) > r.done;
  const latestText = r => (r.want ? r.want.text : r.running ? r.running.text : null);
  // prune 은 깨끗하고 쉬는 기록기(도는 save·지켜보는 쪽·연 세션·관리 작업이 없다)를 치운다. 미저장 기록기는 치우지 않는다.
  function prune(r) {
    if (!dirty(r) && !r.running && !r.waiters.size && !r.editors && !r.blocked && r.B.recs.get(r.id) === r) r.B.recs.delete(r.id);
  }
  // put 은 스냅샷 text 를 넣고 번호를 돌려준다 — 같은 본문이면 새 번호를 내지 않는다(이미 저장됐으면 done). 삭제된 판이면 0.
  // 멈춘(거부 뒤) 기록기는 put 만으로 다시 움직이지 않는다 — retry 가 따로다.
  function put(r, text) {
    if (r.gone) return 0;
    let n;
    if (r.want && r.want.text === text) n = r.want.n;
    else if (!r.want && (r.running ? r.running.text === text : r.saved === text)) return latest(r);
    else r.want = { text, n: (n = ++r.B.seq) };
    pump(r);
    return n;
  }
  function retry(r) { if (r.gone) return; r.halted = false; pump(r); }
  // pump 는 want 를 store.save 로 낸다 — running 은 부르기 전에 동기로 잡는다(한 판에 save 하나). 끝나면 done 을 그 스냅샷 번호까지 올리고,
  // 더 새 want 가 있으면 곧바로 다음을 낸다. 거부(동기 throw 포함)면 want 를 둔 채(더 새 것이 없으면 그 스냅샷을 다시 want 로) 멈춘다.
  // 시간 넘김·세션 닫힘은 running 을 건드리지 않는다 — 끝날 때까지 다음 save 를 내지 않는다.
  function pump(r) {
    if (r.running || !r.want || r.halted || r.blocked || r.gone) return;
    const job = r.running = r.want;
    r.want = null;
    let p;
    try { p = r.B.store.save(r.id, job.text); } catch (e) { p = Promise.reject(e); }
    Promise.resolve(p).then(res => {
      r.running = null;
      if (job.n > r.done) r.done = job.n;
      r.saved = job.text; r.bytes = job.text.length;
      r.modified = res && Number.isFinite(res.modified) ? res.modified : Date.now();
      for (const w of [...r.waiters]) if (w.idle || r.done >= w.n) w.fin("ok");
      pump(r);
      prune(r);
    }, () => {
      r.running = null;
      if (!r.want && !r.gone) r.want = job;
      r.halted = true;
      for (const w of [...r.waiters]) w.fin(w.idle || r.done >= w.n ? "ok" : "fail"); // 멈췄으니 기다리던 번호는 모두 실패다(다음 retry 의 관찰자는 따로 본다)
    });
  }
  // watch 는 번호 n 이 저장됐는가를 at(nowMs 기준 시각)까지 본다 → "ok"(done ≥ n)·"fail"(거부·삭제)·"timeout". idle 은 도는 save 가 끝나기를 본다.
  // 지켜보기를 그만둬도(시간 넘김) 기록기는 그대로 돈다.
  function watch(r, n, at) { return r.done >= n ? Promise.resolve("ok") : r.gone ? Promise.resolve("fail") : waitRec(r, { n }, at); }
  function idle(r, at) { return r.running ? waitRec(r, { idle: true }, at) : Promise.resolve("ok"); }
  function waitRec(r, w, at) {
    return new Promise(res => {
      const t = setTimeout(() => w.fin("timeout"), Math.max(0, at - nowMs()));
      w.fin = v => { if (!r.waiters.delete(w)) return; clearTimeout(t); res(v); prune(r); };
      r.waiters.add(w);
    });
  }
  // bury 는 삭제된 판의 기록기를 폐기한다 — 남은 본문을 버리고 옛 put·retry 를 받지 않는다(같은 id 가 다시 쓰이면 새 기록기다).
  // 그 판을 지금 보고 있는 세션이 있으면 곧바로 알린다(buried — 잠그고 안내하고 다음 판으로 옮긴다). 삭제를 낸 세션의 생존과 상관없다.
  function bury(r) {
    r.gone = true; r.want = null;
    for (const w of [...r.waiters]) w.fin(w.idle ? "ok" : "fail");
    if (r.B.recs.get(r.id) === r) r.B.recs.delete(r.id);
    r.B.thumbs.delete(r.id);
    if (S && S.state.rec === r) S.buried(); // 그 판을 지금 보고 있는 세션(요청한 세션이 아니어도 — renamed 와 같은 수명 구분)
  }
  // renamed 는 이름 바꾸기가 된 판의 본문 이름을 맞춘다(그 요청을 낸 세션이 닫혔어도) — 그 판을 연 세션이 있으면 그 세션의 이름과 본문(put),
  // 없고 저장하지 못한 본문이 있으면 그 본문의 name 을 바꿔 다시 넣는다. 깨끗한 닫힌 판은 다음에 열 때 목록의 이름을 쓴다(show).
  function renamed(r, name) {
    if (r.gone) return;
    if (S && S.state.rec === r) { S.applyName(name); return; }
    const text = latestText(r);
    if (text == null) return;
    const p = C.parse(text);
    if (!p.ok || p.readOnly) return;
    p.board.name = name;
    try { put(r, C.serialize(p.board)); } catch (e) { /* 범위 밖 — 본문은 그대로 둔다 */ }
  }
  // lateErr 는 기한을 넘겼다는 오류다(timeout 표지). within 은 p 를 시각 at 까지만 기다린다 — 넘으면 timeout 표지를 단 오류로 거부한다. p 가 늦게 끝나도 결과는 버린다(요청 자체는 취소하지 못한다).
  function lateErr() { const e = new Error("저장소가 " + STORE_WAIT / 1000 + "초 안에 답하지 않았습니다"); e.timeout = true; return e; }
  function within(p, at) {
    return new Promise((res, rej) => {
      const t = setTimeout(() => rej(lateErr()), Math.max(0, at - nowMs()));
      p.then(v => { clearTimeout(t); res(v); }, e => { clearTimeout(t); rej(e); });
    });
  }

  function el(tag, cls, text) {
    const e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  }
  function button(cls, text, act, title) {
    const b = el("button", cls, text);
    b.type = "button";
    if (act) b.dataset.act = act;
    if (title) b.title = title;
    return b;
  }
  const call = (f, ...a) => { try { return f && f(...a); } catch (e) { return undefined; } };
  const nowMs = () => (window.performance || performance).now();
  const raf = f => window.requestAnimationFrame(f);
  const snapG = v => Math.round(v / GRID) * GRID; // 격자에 붙이기(판 단위)

  // ---- 그림 디코드·인코드(머리 주석 「그림」). data URL 을 fetch·<img src> 로 읽지 않는다 — base64 를 바이트로 풀어(wbcore b64Decode)
  // Blob → createImageBitmap 으로 디코드한다(CSP img-src·connect-src 에 기대지 않는다). blob URL 도 만들지 않는다.
  // 긴 변이 IMG_SIDE 를 넘는 그림(파일에서 읽은 것 — 넣는 길은 이미 줄였다)은 줄인 bitmap 을 돌려주고 큰 것은 close 한다(fitBitmap — 본문의 data·w·h 는 그대로).
  function decodeImage(mime, data) {
    const cib = window.createImageBitmap, Bl = window.Blob;
    if (typeof cib !== "function" || typeof Bl !== "function") return Promise.reject(new Error("createImageBitmap 이 없다"));
    return Promise.resolve().then(() => cib.call(window, new Bl([C.b64Decode(data)], { type: mime }))).then(fitBitmap);
  }
  // fitBitmap 은 bitmap 의 긴 변이 IMG_SIDE 이하이면 그대로, 넘으면 canvas 에 줄여 그린 뒤 createImageBitmap(canvas) 로 만든 작은 bitmap 을 돌려주고
  // 큰 것은 close 한다. 줄이지 못하면(canvas·createImageBitmap(canvas) 실패) 큰 것을 그대로 돌려준다(그림은 보인다 — 그때는 그 그림의 화소를 줄이지 못한다).
  function fitBitmap(bmp) {
    const iw = bmp ? bmp.width : 0, ih = bmp ? bmp.height : 0, long = Math.max(iw, ih);
    if (!(long > IMG_SIDE)) return bmp;
    const s = IMG_SIDE / long, w = Math.max(1, Math.round(iw * s)), h = Math.max(1, Math.round(ih * s));
    let cv = null;
    const free = () => { if (cv) { cv.width = 0; cv.height = 0; } };
    try {
      cv = document.createElement("canvas");
      cv.width = w; cv.height = h;
      const ctx = cv.getContext && cv.getContext("2d");
      if (!ctx) { free(); return bmp; }
      ctx.drawImage(bmp, 0, 0, w, h);
    } catch (e) { free(); return bmp; }
    return Promise.resolve().then(() => window.createImageBitmap(cv)).then(small => {
      free();
      if (!small) return bmp;
      if (bmp.close) bmp.close();
      return small;
    }, () => { free(); return bmp; });
  }
  // hasAlpha 는 canvas 에 투명한 화소가 있는가다(알파 훑기). 읽지 못하면 있다고 본다(PNG 로 — 투명을 잃지 않게).
  function hasAlpha(ctx, w, h) {
    let d;
    try { d = ctx.getImageData(0, 0, w, h).data; } catch (e) { return true; }
    for (let i = 3; i < d.length; i += 4) if (d[i] < 255) return true;
    return false;
  }
  // prepareImage 는 넣을 그림(Blob·File)을 판에 넣을 꼴 { mime, data, bytes, nat, iw, ih }(iw·ih 는 인코드한 크기 px)로 만든다 — 디코드 → 긴 변이 IMG_SIDE 를 넘으면 줄임 →
  // 다시 인코드(투명이 있으면 PNG, 없으면 JPEG IMG_JPEG_Q). 바이트가 IMG_BYTES 를 넘으면 긴 변을 IMG_SHRINK 배씩 줄여 다시(IMG_RETRY 번까지) —
  // 그래도 넘으면 거부한다. nat 은 원래 그림의 긴 변(px — 처음 크기를 정할 때 쓴다). 거부 오류의 message 는 사람에게 보일 안내다.
  function prepareImage(blob) {
    const cib = window.createImageBitmap;
    if (typeof cib !== "function") return Promise.reject(new Error(IMG_READ_FAIL));
    return Promise.resolve().then(() => cib.call(window, blob)).then(bmp => {
      try { return encodeImage(bmp); } finally { if (bmp && bmp.close) bmp.close(); }
    }, () => { throw new Error(IMG_READ_FAIL); });
  }
  function encodeImage(bmp) {
    const iw = bmp.width, ih = bmp.height, long = Math.max(iw, ih);
    if (!(iw > 0 && ih > 0)) throw new Error(IMG_READ_FAIL);
    let side = Math.min(long, IMG_SIDE), alpha = null;
    for (let k = 0; k <= IMG_RETRY; k++) {
      const s = side / long, w = Math.max(1, Math.round(iw * s)), h = Math.max(1, Math.round(ih * s));
      const cv = document.createElement("canvas");
      cv.width = w; cv.height = h;
      const ctx = cv.getContext && cv.getContext("2d");
      if (!ctx || !cv.toDataURL) throw new Error(IMG_READ_FAIL);
      ctx.drawImage(bmp, 0, 0, w, h);
      if (alpha === null) alpha = hasAlpha(ctx, w, h);
      let url = "";
      try { url = cv.toDataURL(alpha ? "image/png" : "image/jpeg", IMG_JPEG_Q); } catch (e) { /* 아래에서 거부 */ }
      cv.width = 0; cv.height = 0;
      const m = /^data:(image\/(?:png|jpeg));base64,/.exec(url);
      if (!m) throw new Error(IMG_READ_FAIL);
      const data = url.slice(m[0].length);
      if (!C.b64Ok(data)) throw new Error(IMG_READ_FAIL);
      const bytes = C.b64Bytes(data);
      if (bytes <= IMG_BYTES) return { mime: m[1], data, bytes, nat: long, iw: w, ih: h };
      side *= IMG_SHRINK;
    }
    throw new Error(IMG_BIG_FAIL);
  }
  // clipImage 는 클립보드(DataTransfer)의 첫 image/* 파일이다(없으면 null).
  function clipImage(dt) {
    if (!dt) return null;
    const items = dt.items || [];
    for (let i = 0; i < items.length; i++) {
      const it = items[i];
      if (it && it.kind === "file" && /^image\//.test(it.type || "")) { const f = it.getAsFile && it.getAsFile(); if (f) return f; }
    }
    const files = dt.files || [];
    for (let i = 0; i < files.length; i++) if (files[i] && /^image\//.test(files[i].type || "")) return files[i];
    return null;
  }

  // ---- 엔진: 판 하나를 그리고 입력을 받는다(시제품 wb.js 를 표면 안으로 옮긴 것 — 측정·HUD·토글을 뺐다).
  // 층(아래부터): stage 의 CSS 바탕(검정·흰색·격자) ← 그림 canvas(타일 없이 보기·그림이 바뀔 때마다 화면에 걸친 그림을 넣은 차례로 다시 그린다)
  //   ← base canvas(끝난 획 타일을 옮겨 그린다 — 타일에 그림은 굽지 않는다) ← live canvas(진행 중 획·예측 꼬리·지우개 표시·고른 그림의 테두리와 손잡이).
  // 보기 view = {x, y, z}: 화면 s = (판 - (x, y)) * z. 입력은 맞추지 않은 view, 그리기는 장치 픽셀에 맞춘 snap(view).
  // 획 s = { id, tool: "pen"|"hl", color(팔레트 번호), w(판 단위 기준 굵기 = 화면 굵기 / 그릴 때 배율), pts[x, y, p, …](판 좌표, 평활 없음),
  //          ox, oy(경로 원점 — 큰 좌표에서 float32 정밀도를 지키려고 경로는 이 점 기준), box, path(Path2D), joins(Path2D|null) }.
  // 타일: 512 장치 px. 배율 단계(Level)마다 따로 굽는다. 이동은 구운 타일을 옮겨 그리기만, 배율이 바뀌면 멈춘 뒤(ZOOM_SETTLE) 보이는 타일만 다시 굽는다
  // — 그동안은 이전 단계 타일을 늘려 그린다(잠깐 흐리다).
  // 그림 im = { id(넣은 차례 — 아래부터), x, y, w, h(판 단위), mime, data(base64), bytes, box, bmp(ImageBitmap|null), state: "wait"|"ok"|"bad", live }.
  //   그림 canvas 에 id 차례로 그린다(drawImages — 공간 색인 igrid 로 화면에 걸친 것만, rAF 한 번에 묶는다) — 층 차례로 늘 획 아래다. 그림이 바뀌어도
  //   획 타일은 버리지 않는다. 옮기기·크기 조절 중인 그림도 그 층의 제 차례에 지금 조작 중인 자리·크기(mode.g)로 그린다 — 조작 중에도 차례가 같다.
  //   디코드는 비동기라 준비되면 그림 층을 다시 그리고, 그때까지(또는 디코드가 안 되면) 옅은 자리 표시를 그린다.
  //   bmp 는 그 그림이 지금 판이나 되돌리기·다시 하기 더미에서 닿는 동안 든다 — 새 작업(push)이 다시 하기 더미를 비워 닿지 않게 되면 곧바로
  //   close 하고(reap), 늦게 끝난 디코드도 닿지 않는 그림이면 곧바로 close 한다. 판을 떠나면(load·destroy) 모두 close 한다.
  // cb = { change()(획·배경·그림이 바뀜), view()(보기가 바뀜), bar()(도구 막대를 다시 칠함), down()(판을 누름 — 팝오버 닫기), full()(판 상한),
  //        long()(한 획의 점 상한), edge()(좌표 범위 밖 점 — 획을 끝냄), imgFull()(그림 상한), sel(rect|null)(고른 그림의 화면 상자 — 지우기 단추 자리) }
  function engine(stage, imgC, baseC, liveC, cb) {
    const ictx = imgC.getContext("2d"), bctx = baseC.getContext("2d"), lctx = liveC.getContext("2d");
    let W = 0, H = 0, dpr = 1;
    let view = { x: 0, y: 0, z: 1 };
    let lastZoomAt = 0, bg = "dark", readOnly = false, locked = false, dead = false;
    const strokes = new Map();     // id → 획(지금 판에 있는 것)
    const grid = new C.Grid(512);
    const images = new Map();      // id → 그림(지금 판에 있는 것)
    const igrid = new C.Grid(512);
    const held = new Set();        // bmp 를 든 그림(판에 있든 되돌리기·다시 하기 더미에서 닿든 — 닿지 않게 되면 reap 이 close, 판을 떠나면 모두 close)
    let nextId = 1, undoStack = [], redoStack = [], totalPts = 0;
    let nextImg = 1, imgBytes = 0, sel = null, gen = 0, snapping = false; // sel: 고른 그림 · gen: 판을 열 때마다 — 늦은 디코드를 버린다
    const guard = { coal: 0, pred: 0 }; // 좌표 지킴이가 버린 수(진단용)

    // ---- 그리기 도구
    const dark = () => bg === "dark";
    const colorOf = c => (c === 0 ? (dark() ? "#f2f2f2" : "#141414") : COLORS[c]);
    const hlAlpha = () => (dark() ? 0.5 : 0.35);
    const drawPts = (pts, w) => C.curve(pts, w * 0.25);

    function geom(s) {
      s.ox = s.pts[0]; s.oy = s.pts[1];
      if (s.tool === "hl") {
        s.box = C.strokeBox(s.pts, s.w / 2);
        s.path = linePath(drawPts(s.pts, s.w), s.ox, s.oy);
        s.joins = null;
      } else {
        let rmax = 0;
        for (let i = 2; i < s.pts.length; i += 3) rmax = Math.max(rmax, C.penRadius(s.w, s.pts[i]));
        s.box = C.strokeBox(s.pts, rmax * 2);
        const o = C.outline(drawPts(s.pts, s.w), s.w);
        s.path = polyPath(o.poly, s.ox, s.oy);
        s.joins = o.joins.length ? circlesPath(o.joins, s.ox, s.oy) : null;
      }
    }
    function polyPath(poly, ox, oy) {
      const p = new window.Path2D();
      if (!poly.length) return p;
      p.moveTo(poly[0] - ox, poly[1] - oy);
      for (let i = 2; i < poly.length; i += 2) p.lineTo(poly[i] - ox, poly[i + 1] - oy);
      p.closePath();
      return p;
    }
    function circlesPath(c, ox, oy) {
      const p = new window.Path2D();
      for (let i = 0; i < c.length; i += 3) { p.moveTo(c[i] - ox + c[i + 2], c[i + 1] - oy); p.arc(c[i] - ox, c[i + 1] - oy, c[i + 2], 0, Math.PI * 2); }
      return p;
    }
    function linePath(pts, ox, oy) {
      const p = new window.Path2D();
      p.moveTo(pts[0] - ox, pts[1] - oy);
      if (pts.length === 3) p.lineTo(pts[0] - ox + 0.001, pts[1] - oy);
      for (let i = 3; i < pts.length; i += 3) p.lineTo(pts[i] - ox, pts[i + 1] - oy);
      return p;
    }
    // paintImage 는 그림 im 을 판 상자 g(없으면 im 자리)에 그린다 — tx·ty 는 paint 와 같다. 준비되지 않은(디코드 전·실패) 그림은 옅은 자리 표시다.
    function paintImage(ctx, im, k, tx, ty, g) {
      g = g || im;
      ctx.setTransform(k, 0, 0, k, g.x * k + tx, g.y * k + ty);
      if (im.bmp) { ctx.drawImage(im.bmp, 0, 0, g.w, g.h); return; }
      ctx.fillStyle = dark() ? "rgba(255,255,255,.08)" : "rgba(0,0,0,.06)";
      ctx.fillRect(0, 0, g.w, g.h);
      ctx.strokeStyle = dark() ? "rgba(255,255,255,.25)" : "rgba(0,0,0,.2)";
      ctx.lineWidth = 1 / k;
      ctx.strokeRect(0, 0, g.w, g.h);
    }
    // paint 는 획을 ctx 에 그린다 — 호출자가 setTransform(k, 0, 0, k, tx, ty) 의 tx·ty 를 준다(판 원점 기준 평행 이동, 장치 px).
    function paint(ctx, s, k, tx, ty, path, joins) {
      ctx.setTransform(k, 0, 0, k, s.ox * k + tx, s.oy * k + ty);
      if (s.tool === "hl") {
        ctx.globalAlpha = hlAlpha();
        ctx.strokeStyle = colorOf(s.color);
        ctx.lineWidth = s.w; ctx.lineCap = "round"; ctx.lineJoin = "round";
        ctx.stroke(path || s.path);
        ctx.globalAlpha = 1;
      } else {
        ctx.fillStyle = colorOf(s.color);
        ctx.fill(path || s.path);
        const j = path ? joins : s.joins;
        if (j) ctx.fill(j);
      }
    }

    // ---- 타일 단계
    const pool = [];
    function newCanvas() {
      const c = pool.pop() || document.createElement("canvas");
      if (c.width !== TILE) { c.width = TILE; c.height = TILE; }
      return c;
    }
    function freeCanvas(c) { if (pool.length < 24) pool.push(c); else { c.width = 0; c.height = 0; } }
    class Level {
      constructor(z) { this.z = z; this.dpr = dpr; this.T = C.tileWorld(z, dpr, TILE); this.tiles = new Map(); }
      drop(key) { const t = this.tiles.get(key); if (t) { freeCanvas(t.c); this.tiles.delete(key); } }
      dropAll() { for (const t of this.tiles.values()) freeCanvas(t.c); this.tiles.clear(); }
      // dropBox 는 판 상자 b 에 걸친 타일을 버린다(지우기·되돌리기 — 보이면 다시 굽는다).
      dropBox(b) {
        for (const [k, t] of this.tiles) if (C.boxHit(C.tileBox(t.i, t.j, this.T), b)) { freeCanvas(t.c); this.tiles.delete(k); }
      }
    }
    let cur = null, prev = null, frameNo = 0, baseDirty = true;
    function bake(L, i, j) {
      const t0 = nowMs();
      const c = newCanvas(), ctx = c.getContext("2d");
      ctx.setTransform(1, 0, 0, 1, 0, 0);
      ctx.clearRect(0, 0, TILE, TILE);
      const k = L.z * L.dpr, tb = C.tileBox(i, j, L.T);
      for (const s of grid.query(tb)) paint(ctx, s, k, -i * TILE, -j * TILE);
      L.tiles.set(i + "," + j, { c, i, j, used: frameNo });
      return nowMs() - t0;
    }
    // addToTiles 는 막 끝난 획을 이미 구운 타일에 덧그린다(새 획이 맨 위라 다시 굽지 않는다). 이전 단계 타일은 버린다.
    function addToTiles(s) {
      if (cur) {
        const k = cur.z * cur.dpr;
        for (const t of cur.tiles.values()) if (C.boxHit(C.tileBox(t.i, t.j, cur.T), s.box)) paint(t.c.getContext("2d"), s, k, -t.i * TILE, -t.j * TILE);
      }
      if (prev) prev.dropBox(s.box);
      baseDirty = true;
    }
    function invalidate(list) {
      if (list === "all" || list.length > 60) { if (cur) cur.dropAll(); if (prev) { prev.dropAll(); prev = null; } }
      else for (const s of list) { if (cur) cur.dropBox(s.box); if (prev) prev.dropBox(s.box); }
      baseDirty = true;
      kick();
    }

    // ---- 획 넣고 빼기·되돌리기
    function addStroke(s) { strokes.set(s.id, s); grid.insert(s); totalPts += s.pts.length / 3; }
    function removeStroke(s) { if (strokes.delete(s.id)) { grid.remove(s); totalPts -= s.pts.length / 3; } }
    // room 은 획 n 개·점 pts 개를 더해도 판 상한(wbcore LIMIT) 안인가다 — 넘는 판은 파일로 다시 읽지 못하므로 넘게 그리지 않는다.
    const room = (n, pts) => strokes.size + n <= C.LIMIT.strokes && totalPts + pts <= C.LIMIT.points;
    const ptsOf = list => list.reduce((a, s) => a + s.pts.length / 3, 0);
    // ---- 그림 넣고 빼기·옮기기(머리 주석 「그림」)
    // imgRoom 은 그림 n 개·bytes 바이트를 더해도 판 상한(wbcore LIMIT.images·imageBytes) 안인가다.
    const imgRoom = (n, bytes) => images.size + n <= C.LIMIT.images && imgBytes + bytes <= C.LIMIT.imageBytes;
    function addImg(im) { images.set(im.id, im); im.box = C.imageBox(im); igrid.insert(im); imgBytes += im.bytes; }
    function removeImg(im) { if (images.delete(im.id)) { igrid.remove(im); imgBytes -= im.bytes; if (sel === im) select(null); } }
    // imgChanged 는 그림 층을 다음 프레임에 다시 그리게 한다(획 타일은 그대로).
    function imgChanged() { imgDirty = true; kick(); }
    // setGeom 은 그림을 상자 g 로 옮긴다 — 색인을 바꾸고 그림 층을 다시 그린다.
    function setGeom(im, g) {
      if (images.has(im.id)) igrid.remove(im);
      im.x = g.x; im.y = g.y; im.w = g.w; im.h = g.h; im.box = C.imageBox(im);
      if (images.has(im.id)) igrid.insert(im);
      imgChanged();
    }
    // imgOk 는 상자 g 가 저장할 수 있는 그림 자리인가다 — 유한, w·h 가 IMG_MIN~COORD_MAX, x·y·x+w·y+h 가 ±COORD_MAX 안(옮기기·크기 조절이 지킨다).
    const imgOk = g => [g.x, g.y, g.w, g.h].every(Number.isFinite) && g.w >= C.IMG_MIN && g.h >= C.IMG_MIN && g.w <= C.COORD_MAX && g.h <= C.COORD_MAX
      && g.x >= -C.COORD_MAX && g.y >= -C.COORD_MAX && g.x + g.w <= C.COORD_MAX && g.y + g.h <= C.COORD_MAX;
    // reach 는 그림 im 이 지금 판이나 되돌리기·다시 하기 더미에서 닿는가다. reap 은 닿지 않는 그림의 bmp 를 close 하고 held 에서 뺀다.
    const reach = im => images.get(im.id) === im || undoStack.some(o => o.im === im) || redoStack.some(o => o.im === im);
    function dropBmp(im) { if (im.bmp && im.bmp.close) im.bmp.close(); im.bmp = null; held.delete(im); }
    function reap() {
      const r = new Set(images.values());
      for (const list of [undoStack, redoStack]) for (const o of list) if (o.im) r.add(o.im);
      for (const im of [...held]) if (!r.has(im)) dropBmp(im);
    }
    // decode 는 그림 im 을 비동기로 디코드한다 — 준비되면 그림 층을 다시 그린다. 그사이 판을 떠났거나(gen) 그 그림이 판·더미 어디에서도 닿지 않게
    // 됐으면(reach) 곧바로 close 한다. 실패하면 자리 표시로 남긴다(본문의 data 는 그대로 — 다시 저장해도 잃지 않는다).
    function decode(im) {
      const g = gen;
      im.state = "wait";
      decodeImage(im.mime, im.data).then(bmp => {
        if (dead || g !== gen || !reach(im)) { if (bmp && bmp.close) bmp.close(); return; }
        im.bmp = bmp; im.state = "ok"; held.add(im);
        imgChanged();
      }, () => { if (!dead && g === gen) im.state = "bad"; });
    }
    function closeHeld() { for (const im of [...held]) dropBmp(im); }
    // insertImage 는 준비된 그림 p = { mime, data, bytes, nat }(prepareImage)을 화면 가운데에 넣는다 — 처음 크기는 화면 px 로 긴 변 IMG_SHOW_PX
    // (원래 긴 변 nat 이 더 작으면 그것), 왼쪽 위와 폭은 격자에 맞춘다(높이는 비율대로). 넣은 뒤 고르기 도구로 바꾸고 그 그림을 고른다.
    // 읽기 전용·잠금·상한·그리기 중이면 넣지 않는다(false). iw·ih 는 인코드한 그림의 크기다(비율).
    function insertImage(p, iw, ih) {
      if (readOnly || locked || (mode && mode.kind !== "pan" && mode.kind !== "pinch")) return false; // 그리기·지우기·그림 조작 중이면 넣지 않는다
      if (!imgRoom(1, p.bytes)) { cb.imgFull(); return false; }
      const px = Math.min(IMG_SHOW_PX, p.nat || IMG_SHOW_PX), ratio = ih / iw;
      const wpx = iw >= ih ? px : px / ratio;
      const w = C.clamp(Math.max(GRID, snapG(wpx / view.z)), GRID, Math.floor(C.COORD_MAX / GRID) * GRID), h = w * ratio;
      const c = C.toWorld(view, W / 2, H / 2);
      const x = C.clamp(snapG(c.x - w / 2), -C.COORD_MAX, C.COORD_MAX - w), y = C.clamp(snapG(c.y - h / 2), -C.COORD_MAX, C.COORD_MAX - h);
      const im = { id: nextImg++, x, y, w, h, mime: p.mime, data: p.data, bytes: p.bytes, bmp: null, state: "wait" };
      addImg(im);
      decode(im);
      imgChanged();
      setTool("select");
      select(im);
      push({ t: "iadd", im });
      return true;
    }
    function select(im) { if (sel === im) return; sel = im; liveDirty = true; kick(); }
    // deleteSelected 는 고른 그림을 지운다(되돌리기 한 단계).
    function deleteSelected() {
      if (readOnly || locked || mode || !sel) return false;
      const im = sel;
      removeImg(im);
      imgChanged();
      push({ t: "idel", im });
      return true;
    }
    function apply(op, dir) { // dir 1 = 하기, -1 = 되돌리기
      if (op.t === "imove") { setGeom(op.im, dir === 1 ? op.to : op.from); return true; }
      if (op.t === "iadd" || op.t === "idel") {
        const add = (op.t === "iadd") === (dir === 1);
        if (add) { if (!imgRoom(1, op.im.bytes)) { cb.imgFull(); return false; } addImg(op.im); } else removeImg(op.im);
        imgChanged();
        return true;
      }
      const adding = (op.t === "add") === (dir === 1);
      if (adding && !room(op.list.length, ptsOf(op.list))) { cb.full(); return false; }
      for (const s of op.list) adding ? addStroke(s) : removeStroke(s);
      // 다시 하기로 돌아온 새 획은 맨 위라 덧그리고, 되돌리기로 돌아온 지운 획(옛 획)은 차례를 지키려고 그 자리 타일을 다시 굽는다
      if (adding && op.t === "add" && op.list.length <= 60) { for (const s of op.list) addToTiles(s); kick(); }
      else invalidate(op.list);
      return true;
    }
    // push 는 새 작업이다 — 다시 하기 더미를 비운다. 버린 단계에 그림이 있었으면 닿지 않게 된 그림의 bmp 를 놓는다(reap). 되돌리기 더미에는 상한이 없다.
    function push(op) { const dropped = redoStack; undoStack.push(op); redoStack = []; if (dropped.some(o => o.im)) reap(); cb.bar(); cb.change(); }
    function undo() { if (readOnly || locked || mode) return; const op = undoStack.pop(); if (!op) return; if (!apply(op, -1)) { undoStack.push(op); return; } redoStack.push(op); cb.bar(); cb.change(); }
    function redo() { if (readOnly || locked || mode) return; const op = redoStack.pop(); if (!op) return; if (!apply(op, 1)) { redoStack.push(op); return; } undoStack.push(op); cb.bar(); cb.change(); }

    // ---- 입력
    const ptrs = new Map();        // pointerId → { type, x, y, ignored }
    let mode = null;               // { kind: "draw"|"erase"|"pan"|"pinch"|"imove"|"isize", … }
    let space = false, mouse = { x: 0, y: 0 }, hover = null;
    const touches = () => [...ptrs.entries()].filter(([, p]) => p.type === "touch" && !p.ignored);
    const drawTool = () => !readOnly && !locked && (prefs.tool === "pen" || prefs.tool === "hl" || prefs.tool === "erase");
    const selTool = () => !readOnly && !locked && prefs.tool === "select";
    const imgMode = () => !!mode && (mode.kind === "imove" || mode.kind === "isize");

    // ---- 고르기 도구(그림 — 머리 주석 「그림」)
    // hitImage 는 화면 점 (cx, cy) 아래의 맨 위 그림(나중에 넣은 것)이다.
    function hitImage(cx, cy) {
      const w = C.toWorld(view, cx, cy);
      let top = null;
      for (const im of igrid.query([w.x, w.y, w.x, w.y])) if (w.x >= im.x && w.x <= im.x + im.w && w.y >= im.y && w.y <= im.y + im.h && (!top || im.id > top.id)) top = im;
      return top;
    }
    // hitHandle 은 고른 그림의 귀 손잡이 번호다(0 왼쪽 위·1 오른쪽 위·2 오른쪽 아래·3 왼쪽 아래, 없으면 -1). 화면 px 로 잰다(확대와 무관).
    function hitHandle(cx, cy, type) {
      if (!sel) return -1;
      const a = C.toScreen(view, sel.x, sel.y), b = C.toScreen(view, sel.x + sel.w, sel.y + sel.h), r = HIT_PX[type] || HIT_PX.mouse;
      const cs = [[a.x, a.y], [b.x, a.y], [b.x, b.y], [a.x, b.y]];
      let best = -1, bd = Infinity;
      cs.forEach(([x, y], i) => { const d = Math.hypot(cx - x, cy - y); if (d <= r && d < bd) { bd = d; best = i; } });
      return best;
    }
    // onImage 는 고르기 도구에서 이 누름이 그림 조작으로 시작하는가다(손잡이나 그림 위).
    const onImage = (cx, cy, type) => hitHandle(cx, cy, type) >= 0 || !!hitImage(cx, cy);
    // startSelect 는 고르기 도구의 누름이다 — 손잡이면 크기, 그림이면 고르고 옮기기, 빈 곳이면 고르기를 풀고 판 이동.
    function startSelect(e, type) {
      const c = hitHandle(e.clientX, e.clientY, type), im = c >= 0 ? sel : hitImage(e.clientX, e.clientY);
      if (!im) { select(null); startPan(e.pointerId, e.clientX, e.clientY, type); return; }
      select(im);
      mode = { kind: c >= 0 ? "isize" : "imove", id: e.pointerId, type, im, c, w0: C.toWorld(view, e.clientX, e.clientY), o: { x: im.x, y: im.y, w: im.w, h: im.h }, g: null };
    }
    // imageTo 는 옮기기·크기 조절 중의 포인터 자리다. 옮기기는 왼쪽 위가 격자(GRID)에 붙는다. 크기는 비율을 지키고 맞은편 귀를 고정한 채
    // 폭이 격자 배수가 되게 한다(맞은편 귀가 격자에 있으면 움직이는 귀의 x 가 격자에 붙는다) — 폭은 격자 한 칸 이상, 그림이 좌표 범위 안에 들 만큼까지.
    // 유효성(imgOk — 좌표·크기 범위)이 격자 최소 한 칸보다 먼저다: 고정한 귀를 둔 채 유효한 결과가 없으면(좌표 끝의 작은 그림 등) 그 움직임을
    // 적용하지 않고 직전 유효 상태(d.g, 없으면 처음 자리)를 둔다. 높이가 IMG_MIN 아래로 내려가지 않을 만큼의 폭도 하한이다(아주 납작한 그림).
    function imageTo(cx, cy) {
      const d = mode, o = d.o, w = C.toWorld(view, cx, cy), M = C.COORD_MAX;
      let g;
      if (d.kind === "imove") {
        g = { x: C.clamp(snapG(o.x + w.x - d.w0.x), -M, M - o.w), y: C.clamp(snapG(o.y + w.y - d.w0.y), -M, M - o.h), w: o.w, h: o.h };
      } else {
        const ratio = o.h / o.w, right = d.c === 1 || d.c === 2, bottom = d.c === 2 || d.c === 3;
        const ax = right ? o.x : o.x + o.w, ay = bottom ? o.y : o.y + o.h;
        const room = Math.min(right ? M - ax : ax + M, (bottom ? M - ay : ay + M) / ratio, M, M / ratio);
        // 폭은 격자 배수로 [minW, maxW] — maxW < minW 면(고정한 귀를 둔 채 들 자리가 없다) 아래 imgOk 가 그 움직임을 버린다
        let minW = Math.max(GRID, Math.ceil(C.IMG_MIN / ratio / GRID) * GRID);
        if (minW * ratio < C.IMG_MIN) minW += GRID; // 부동소수로 높이가 하한에 조금 못 미치면 한 칸 더(예: 9344×0.02 → 4672 이면 0.00999…)
        const maxW = Math.floor(room / GRID) * GRID;
        const nw = Math.min(Math.max(snapG(right ? w.x - ax : ax - w.x), minW), maxW), nh = nw * ratio;
        g = { x: right ? ax : ax - nw, y: bottom ? ay : ay - nh, w: nw, h: nh };
      }
      if (!imgOk(g)) return; // 유효하지 않은 움직임은 적용하지 않는다(직전 유효 상태를 둔다)
      const prev = d.g || o;
      if (g.x === prev.x && g.y === prev.y && g.w === prev.w && g.h === prev.h) return;
      if (!d.g) setSnapping(true); // 처음 움직일 때 옅은 격자
      d.g = g;
      imgDirty = true; liveDirty = true; kick(); // 그림 층의 제 차례에 지금 자리로, 테두리·손잡이는 live 층에
    }
    // endImage 는 옮기기·크기 조절을 끝낸다 — 바뀌었으면 되돌리기 한 단계. keep 이 거짓이거나 확정할 자리가 유효하지 않으면(imgOk) 버린다(제자리로).
    function endImage(keep) {
      const d = mode;
      mode = null;
      setSnapping(false);
      const im = d.im, g = d.g;
      imgDirty = true; liveDirty = true; kick();
      if (!keep || !g || !imgOk(g)) return;
      const from = { x: im.x, y: im.y, w: im.w, h: im.h };
      setGeom(im, g);
      push({ t: "imove", im, from, to: { x: g.x, y: g.y, w: g.w, h: g.h } });
    }
    function setSnapping(on) {
      if (snapping === !!on) return;
      snapping = !!on;
      stageClass();
      if (!snapping && bg !== "grid") { stage.style.backgroundSize = ""; stage.style.backgroundPosition = ""; }
      baseDirty = true; kick();
    }

    function startTool(e, type) {
      const w = C.toWorld(view, e.clientX, e.clientY);
      if (prefs.tool === "erase") {
        mode = { kind: "erase", id: e.pointerId, type, removed: [], lx: w.x, ly: w.y };
        eraseTo(w.x, w.y);
        return;
      }
      const tool = prefs.tool;
      const s = { id: nextId++, tool, color: prefs.color[tool], w: prefs.width[tool] / view.z, pts: [] };
      mode = { kind: "draw", id: e.pointerId, type, s, lp: null, lt: 0, skipped: null, pred: null, cx: e.clientX, cy: e.clientY };
      addPoint(e.clientX, e.clientY, e.pressure, e.timeStamp, type);
      kick();
    }
    function addPoint(cx, cy, pressure, ts, type) {
      const d = mode, s = d.s, w = C.toWorld(view, cx, cy), n = s.pts.length;
      // 좌표 범위 밖(화면 밖으로 붙잡은 포인터 등) — 이 획은 여기서 끝낸다(다시 들어와도 잇지 않는다 — 떼고 다시 그어야 새 획). 첫 점이면 획을 만들지 않는다
      if (!C.inRange(w.x, w.y)) {
        if (n) finishDraw(); else { mode = null; liveDirty = true; kick(); }
        cb.edge();
        return;
      }
      if (n) {
        const lx = s.pts[n - 3], ly = s.pts[n - 2], dist = Math.hypot(w.x - lx, w.y - ly) * view.z;
        if (dist < MIN_STEP_PX) { d.skipped = { cx, cy, pressure, ts }; return; }
        d.speed = dist / Math.max(1, ts - d.lt);
      }
      let p;
      if (type === "pen" && pressure > 0) p = pressure;
      else p = C.speedPressure(d.lp, n ? d.speed : 0.3);
      s.pts.push(w.x, w.y, Math.round(p * 1000) / 1000);
      d.lp = p; d.lt = ts; d.skipped = null;
      if (s.pts.length / 3 >= STROKE_PTS) { finishDraw(); cb.long(); } // 한 획의 점 상한 — 여기서 끝낸다(다음 점부터 새 획으로 잇지 않는다)
    }
    // 좌표 지킴이: 일부 브라우저가 coalesced·predicted 사건에 엉뚱한 좌표(0,0 이나 화면 밖)를 실어 획이 화면 끝으로 튄다.
    // coalesced 점은 앞 점과 이 사건의 좌표 사이 근처에 있어야 하고, 예측 꼬리는 마지막 점에서 짧게만 뻗는다.
    function takeEvents(e) {
      const list = e.getCoalescedEvents ? e.getCoalescedEvents() : null;
      const evs = list && list.length ? list : [e];
      const px = mode.cx == null ? e.clientX : mode.cx, py = mode.cy == null ? e.clientY : mode.cy;
      const span = Math.hypot(e.clientX - px, e.clientY - py) + 24;
      let endOk = false;
      const d = mode, cut = () => mode !== d; // 점 상한으로 획이 끝났으면(addPoint) 이 사건의 나머지 점은 버린다
      for (const ev of evs) {
        const x = ev.clientX, y = ev.clientY;
        if (ev !== e && (Math.hypot(x - px, y - py) > span || Math.hypot(x - e.clientX, y - e.clientY) > span)) { guard.coal++; continue; }
        addPoint(x, y, ev.pressure, ev.timeStamp, d.type);
        if (cut()) return;
        endOk = x === e.clientX && y === e.clientY;
      }
      if (!endOk) addPoint(e.clientX, e.clientY, e.pressure, e.timeStamp, d.type); // 묶음이 이 사건의 자리에서 끝나지 않으면 그 자리를 넣는다
      if (cut()) return;
      mode.cx = e.clientX; mode.cy = e.clientY;
      mode.pred = null;
      if (mode.type === "pen" && e.getPredictedEvents) { // 예측은 펜에만 — 마우스·손가락은 쓴 자리만 그린다
        const pe = e.getPredictedEvents();
        if (pe && pe.length) {
          const pr = [], lastP = mode.lp == null ? 0.5 : mode.lp;
          for (const ev of pe) {
            if (Math.hypot(ev.clientX - e.clientX, ev.clientY - e.clientY) > PRED_MAX_PX) { guard.pred++; break; }
            const w = C.toWorld(view, ev.clientX, ev.clientY);
            if (!C.inRange(w.x, w.y)) break; // 범위 밖 예측 꼬리는 그리지 않는다(그 점은 쓸 수 없다)
            pr.push(w.x, w.y, lastP);
          }
          mode.pred = pr.length ? pr : null;
        }
      }
      kick();
    }
    function finishDraw() {
      const d = mode, s = d.s;
      mode = null;
      if (d.skipped) { // 뗀 자리까지 — 거른 마지막 점을 넣는다(쓴 그대로 끝나게)
        const n = s.pts.length, w = C.toWorld(view, d.skipped.cx, d.skipped.cy);
        if (n && C.inRange(w.x, w.y) && Math.hypot(w.x - s.pts[n - 3], w.y - s.pts[n - 2]) * view.z > 0.05) s.pts.push(w.x, w.y, s.pts[n - 1]);
      }
      liveDirty = true;
      kick();
      if (!s.pts.length) return;
      if (!room(1, s.pts.length / 3)) { cb.full(); return; }
      geom(s);
      addStroke(s);
      addToTiles(s);
      push({ t: "add", list: [s] });
    }
    function cancelMode() { // 버린다(오므리기로 바뀐 한 손가락 획 등)
      if (!mode) return;
      if (mode.kind === "erase") endErase();
      if (imgMode()) { endImage(false); return; } // 그림 조작은 제자리로(오므리기·펜이 끼어듦)
      mode = null; liveDirty = true; kick();
    }
    function eraseTo(x, y) {
      const d = mode, r = ERASE_PX / view.z, dx = x - d.lx, dy = y - d.ly, len = Math.hypot(dx, dy), steps = Math.max(1, Math.ceil(len / (r * 0.7)));
      for (let k = 1; k <= steps || k === 1; k++) {
        const px = d.lx + (dx * k) / steps, py = d.ly + (dy * k) / steps;
        for (const s of grid.query([px - r, py - r, px + r, py + r])) if (C.hitStroke(s, px, py, r)) { removeStroke(s); d.removed.push(s); invalidate([s]); }
        if (!len) break;
      }
      d.lx = x; d.ly = y; d.ex = x; d.ey = y;
      liveDirty = true; kick();
    }
    function endErase() { const d = mode; mode = null; if (d.removed.length) push({ t: "erase", list: d.removed }); liveDirty = true; kick(); }
    // tap 이 참이면 움직이지 않고 뗐을 때 고르기를 푼다(고르기 도구의 손가락 — 빈 곳을 톡 치기).
    function startPan(id, x, y, type, tap) { mode = { kind: "pan", id, type, lx: x, ly: y, sx: x, sy: y, tap: !!tap }; stage.classList.add("grabbing"); }
    function startPinch() {
      const t = touches().slice(0, 2);
      if (t.length < 2) return;
      const [a, b] = t.map(([, p]) => p), cx = (a.x + b.x) / 2, cy = (a.y + b.y) / 2;
      mode = { kind: "pinch", ids: t.map(([id]) => id), w0: C.toWorld(view, cx, cy), d0: Math.max(1, Math.hypot(a.x - b.x, a.y - b.y)), z0: view.z };
    }
    function setView(v) {
      v = C.boundView(v, W, H); // 화면 전체가 좌표 범위 안이다(wbcore boundView)
      if (v.z !== view.z) lastZoomAt = nowMs();
      view = v; baseDirty = true; liveDirty = true; imgDirty = true; kick();
      cb.view();
    }
    const capture = id => { try { stage.setPointerCapture(id); } catch (err) { /* 합성 입력 */ } };

    function onDown(e) {
      e.preventDefault();
      cb.down();
      const type = e.pointerType || "mouse";
      ptrs.set(e.pointerId, { type, x: e.clientX, y: e.clientY, ignored: false });
      if (type === "pen") {
        if (mode && mode.type === "pen") return;
        if (mode && mode.kind === "draw" && mode.type === "mouse") finishDraw();
        else if (mode) cancelMode();
        for (const [, p] of ptrs) if (p.type === "touch") p.ignored = true; // 펜이 닿으면 이미 닿은 손가락(손바닥)은 뗄 때까지 무시
        stage.classList.remove("grabbing");
        if (drawTool()) startTool(e, type); else if (selTool()) startSelect(e, type); else startPan(e.pointerId, e.clientX, e.clientY, type);
        capture(e.pointerId);
        return;
      }
      if (type === "touch") {
        if (mode && mode.type === "pen") { ptrs.get(e.pointerId).ignored = true; return; } // 펜으로 쓰는 중의 손바닥
        const t = touches();
        if (t.length === 1) {
          // 손가락 하나는 이동 — 고르기 도구에서 그림(손잡이) 위에서 시작했을 때만 그림 조작(펜이 없는 기기를 위해)
          if (FINGER_DRAW && drawTool()) startTool(e, "touch");
          else if (selTool() && onImage(e.clientX, e.clientY, "touch")) startSelect(e, "touch");
          else startPan(e.pointerId, e.clientX, e.clientY, "touch", selTool());
        } else if (t.length === 2) {
          if (mode && mode.kind === "draw") cancelMode(); // 한 손가락 획 → 오므리기로 바뀌면 그 획은 버린다
          else if (mode && mode.kind === "erase") endErase();
          else if (imgMode()) cancelMode(); // 한 손가락 그림 조작 → 오므리기로 바뀌면 제자리로
          startPinch();
        } else { ptrs.get(e.pointerId).ignored = true; return; }
        capture(e.pointerId);
        return;
      }
      // mouse
      if (mode) return;
      if (e.button === 1 || (e.button === 0 && space)) startPan(e.pointerId, e.clientX, e.clientY, "mouse");
      else if (e.button === 0 && drawTool()) startTool(e, "mouse");
      else if (e.button === 0 && selTool()) startSelect(e, "mouse");
      else if (e.button === 0) startPan(e.pointerId, e.clientX, e.clientY, "mouse");
      else return;
      capture(e.pointerId);
    }
    function onMove(e) {
      if (e.pointerType === "mouse") { mouse.x = e.clientX; mouse.y = e.clientY; }
      const p = ptrs.get(e.pointerId);
      if (prefs.tool === "erase" && (!mode || mode.kind === "erase")) { hover = { x: e.clientX, y: e.clientY }; liveDirty = true; kick(); }
      if (!p || p.ignored || !mode) { if (p) { p.x = e.clientX; p.y = e.clientY; } return; }
      const dx = e.clientX - p.x, dy = e.clientY - p.y;
      p.x = e.clientX; p.y = e.clientY;
      if (mode.kind === "draw" && mode.id === e.pointerId) takeEvents(e);
      else if (mode.kind === "erase" && mode.id === e.pointerId) { const w = C.toWorld(view, e.clientX, e.clientY); eraseTo(w.x, w.y); }
      else if (mode.kind === "pan" && mode.id === e.pointerId) setView(C.panBy(view, dx, dy));
      else if (imgMode() && mode.id === e.pointerId) imageTo(e.clientX, e.clientY);
      else if (mode.kind === "pinch" && mode.ids.includes(e.pointerId)) {
        const a = ptrs.get(mode.ids[0]), b = ptrs.get(mode.ids[1]);
        if (!a || !b) return;
        const cx = (a.x + b.x) / 2, cy = (a.y + b.y) / 2, d = Math.max(1, Math.hypot(a.x - b.x, a.y - b.y));
        setView(C.anchor(mode.w0, cx, cy, mode.z0 * (d / mode.d0)));
      }
    }
    function onUp(e) {
      const p = ptrs.get(e.pointerId);
      if (!p) return;
      ptrs.delete(e.pointerId);
      if (!mode || p.ignored) return;
      if (mode.kind === "draw" && mode.id === e.pointerId) finishDraw();
      else if (mode.kind === "erase" && mode.id === e.pointerId) endErase();
      else if (mode.kind === "pan" && mode.id === e.pointerId) {
        const d = mode;
        mode = null; stage.classList.remove("grabbing");
        if (d.tap && Math.hypot(e.clientX - d.sx, e.clientY - d.sy) < 6) select(null);
      }
      else if (imgMode() && mode.id === e.pointerId) endImage(e.type !== "pointercancel");
      else if (mode.kind === "pinch" && mode.ids.includes(e.pointerId)) {
        mode = null;
        const rest = touches(); // 두 손가락 중 하나를 떼면 남은 손가락으로 이동을 잇는다(그리기는 새로 닿을 때만)
        if (rest.length === 1) startPan(rest[0][0], rest[0][1].x, rest[0][1].y, "touch");
        else if (rest.length >= 2) startPinch();
      }
    }
    function pointer(type, e) {
      if (type === "pointerdown") onDown(e);
      else if (type === "pointermove") onMove(e);
      else if (type === "pointerup" || type === "pointercancel" || type === "lostpointercapture") onUp(e);
    }
    function leave() { if (hover) { hover = null; liveDirty = true; kick(); } }
    function wheel(e) {
      e.preventDefault();
      let dx = e.deltaX, dy = e.deltaY;
      if (e.deltaMode === 1) { dx *= 16; dy *= 16; } else if (e.deltaMode === 2) { dx *= H; dy *= H; }
      if (e.ctrlKey || e.metaKey) { if (!gest) setView(C.zoomAt(view, e.clientX, e.clientY, view.z * Math.exp(-dy * 0.01))); }
      else setView(C.panBy(view, -dx, -dy));
    }
    // Safari(맥 트랙패드 오므리기)의 gesture 이벤트 — 손가락이 판에 닿아 있으면(iPad 오므리기) pointer 쪽이 맡는다.
    let gest = null;
    function gesture(type, e) {
      e.preventDefault();
      if (type === "gesturestart") {
        if (touches().length || [...ptrs.values()].some(p => p.type === "touch")) { gest = null; return; }
        const x = e.clientX != null ? e.clientX : mouse.x, y = e.clientY != null ? e.clientY : mouse.y;
        gest = { z0: view.z, w0: C.toWorld(view, x, y), x, y };
      } else if (type === "gesturechange") { if (gest) setView(C.anchor(gest.w0, gest.x, gest.y, gest.z0 * e.scale)); }
      else gest = null;
    }
    function setSpace(on) { space = !!on; stage.classList.toggle("grab", space); }

    // ---- 프레임
    let rafId = 0, liveDirty = true, imgDirty = true;
    function kick() { if (!rafId && !dead) rafId = raf(frame); }
    // drawImages 는 그림 층을 다시 그린다 — 화면에 걸친 그림(공간 색인)을 넣은 차례(id)로. 옮기기·크기 조절 중인 그림은 제 차례에 지금 자리(mode.g)로.
    // 자리 맞춤은 획 타일과 같다(판 원점의 평행 이동을 장치 px 로 반올림 — composite).
    function drawImages(v) {
      const k = v.z * dpr, tx = Math.round(-v.x * k), ty = Math.round(-v.y * k);
      ictx.setTransform(1, 0, 0, 1, 0, 0);
      ictx.clearRect(0, 0, imgC.width, imgC.height);
      if (!images.size) return;
      const vb = [v.x, v.y, v.x + W / v.z, v.y + H / v.z], d = imgMode() && mode.g ? mode : null;
      const list = igrid.query(vb);
      if (d && images.get(d.im.id) === d.im && !list.includes(d.im)) { list.push(d.im); list.sort((a, b) => a.id - b.id); }
      for (const im of list) {
        const g = d && d.im === im ? d.g : im;
        if (C.boxHit(C.imageBox(g), vb)) paintImage(ictx, im, k, tx, ty, g);
      }
    }
    function visibleTiles(L, v) { return C.tilesFor(v, W, H, L.T); }
    function composite(v) {
      const k = v.z * dpr;
      bctx.setTransform(1, 0, 0, 1, 0, 0);
      bctx.clearRect(0, 0, baseC.width, baseC.height);
      if (cur.z !== v.z) { drawScaled(cur, v); return; }
      const vis = visibleTiles(cur, v);
      let anyMissing = false;
      for (const [i, j] of vis) if (!cur.tiles.has(i + "," + j)) { anyMissing = true; break; }
      if (anyMissing && prev) drawScaled(prev, v);
      const ox = Math.round(-v.x * k), oy = Math.round(-v.y * k);
      for (const [i, j] of vis) {
        const t = cur.tiles.get(i + "," + j);
        if (!t) continue;
        t.used = frameNo;
        const x = i * TILE + ox, y = j * TILE + oy;
        if (anyMissing && prev) bctx.clearRect(x, y, TILE, TILE);
        bctx.drawImage(t.c, x, y);
      }
      if (!anyMissing && prev) { prev.dropAll(); prev = null; }
    }
    function drawScaled(L, v) {
      const k = v.z * dpr, size = TILE * k / (L.z * L.dpr);
      for (const [i, j] of visibleTiles(L, v)) {
        const t = L.tiles.get(i + "," + j);
        if (!t) continue;
        t.used = frameNo;
        bctx.drawImage(t.c, (i * L.T - v.x) * k, (j * L.T - v.y) * k, size, size);
      }
    }
    function evict() {
      const n = (cur ? cur.tiles.size : 0) + (prev ? prev.tiles.size : 0);
      if (n <= MAX_TILES) return;
      if (prev) { prev.dropAll(); prev = null; }
      if (cur.tiles.size <= MAX_TILES) return;
      const old = [...cur.tiles.entries()].sort((a, b) => a[1].used - b[1].used);
      for (let q = 0; q < old.length - MAX_TILES; q++) cur.drop(old[q][0]);
    }
    function frame() {
      rafId = 0;
      if (dead) return;
      frameNo++;
      const t0 = nowMs();
      let again = false;
      const v = C.snap(view, dpr);
      // 단계: 배율이 멈추면 새 단계
      if (!cur) cur = new Level(v.z);
      if (cur.z !== v.z) {
        if (t0 - lastZoomAt >= ZOOM_SETTLE) {
          if (prev) prev.dropAll();
          prev = cur.tiles.size ? cur : null;
          if (!prev) cur.dropAll();
          cur = new Level(v.z);
          baseDirty = true;
        } else again = true;
      }
      // 굽기
      if (cur.z === v.z) {
        const todo = visibleTiles(cur, v).filter(([i, j]) => !cur.tiles.has(i + "," + j));
        const tb = nowMs();
        let baked = 0;
        for (const [i, j] of todo) {
          bake(cur, i, j); baked++;
          if (nowMs() - tb > BAKE_BUDGET) break;
        }
        if (baked) baseDirty = true;
        if (baked < todo.length) again = true;
      }
      if (imgDirty) {
        imgDirty = false;
        drawImages(v);
      }
      if (baseDirty) {
        baseDirty = false;
        composite(v);
        evict();
        paintGrid(v);
      }
      // 진행 중 획
      if (liveDirty || (mode && mode.kind === "draw")) {
        liveDirty = false;
        drawLive(v);
      }
      if (again) kick();
    }
    function drawLive(v) {
      const k = v.z * dpr;
      lctx.setTransform(1, 0, 0, 1, 0, 0);
      lctx.clearRect(0, 0, liveC.width, liveC.height);
      if (mode && mode.kind === "draw" && mode.s.pts.length) {
        const s = mode.s, pred = mode.pred;
        s.ox = s.pts[0]; s.oy = s.pts[1];
        let path, joins = null;
        if (s.tool === "hl") path = linePath(drawPts(pred ? s.pts.concat(pred) : s.pts, s.w), s.ox, s.oy);
        else {
          const o = C.outline(drawPts(pred ? s.pts.concat(pred) : s.pts, s.w), s.w);
          path = polyPath(o.poly, s.ox, s.oy);
          if (o.joins.length) joins = circlesPath(o.joins, s.ox, s.oy);
        }
        paint(lctx, s, k, -v.x * k, -v.y * k, path, joins);
      }
      drawSel(v); // 고른 그림의 테두리·손잡이만(그림 자체는 그림 층에 — drawImages)
      const eh = mode && mode.kind === "erase" ? C.toScreen(view, mode.ex, mode.ey) : hover && prefs.tool === "erase" ? hover : null;
      if (eh) {
        lctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        lctx.strokeStyle = dark() ? "rgba(255,255,255,.8)" : "rgba(0,0,0,.7)";
        lctx.lineWidth = 1.5;
        lctx.beginPath(); lctx.arc(eh.x, eh.y, ERASE_PX, 0, Math.PI * 2); lctx.stroke();
      }
    }
    // drawSel 은 고른 그림의 테두리와 네 귀 손잡이다(화면 px 크기 — 확대와 무관). cb.sel 에 화면 상자를 알린다(지우기 단추 자리).
    let selKey = "";
    function drawSel(v) {
      let r = null;
      if (sel && images.has(sel.id)) {
        const g = imgMode() && mode.im === sel && mode.g ? mode.g : sel, a = C.toScreen(v, g.x, g.y);
        r = { x: a.x, y: a.y, w: g.w * v.z, h: g.h * v.z };
        lctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        lctx.strokeStyle = "#2f7cf6"; lctx.lineWidth = 1.5;
        lctx.strokeRect(r.x, r.y, r.w, r.h);
        lctx.fillStyle = "#fff";
        for (const [x, y] of [[r.x, r.y], [r.x + r.w, r.y], [r.x + r.w, r.y + r.h], [r.x, r.y + r.h]]) {
          lctx.fillRect(x - HANDLE_PX, y - HANDLE_PX, 2 * HANDLE_PX, 2 * HANDLE_PX);
          lctx.strokeRect(x - HANDLE_PX, y - HANDLE_PX, 2 * HANDLE_PX, 2 * HANDLE_PX);
        }
      }
      const key = r ? [r.x, r.y, r.w, r.h].map(Math.round).join() : "";
      if (key !== selKey) { selKey = key; cb.sel(r); }
    }
    function paintGrid(v) {
      if (bg !== "grid" && !snapping) return; // 옮기기·크기 조절 동안은 바탕이 격자가 아니어도 옅은 격자(CSS .snapping)
      let g = GRID * v.z;
      while (g < 12) g *= 4;
      while (g > 160) g /= 4;
      stage.style.backgroundSize = g + "px " + g + "px";
      stage.style.backgroundPosition = (((-v.x * v.z) % g) + g) % g + "px " + ((((-v.y * v.z) % g) + g) % g) + "px";
    }

    // ---- 크기·회전
    function resize() {
      const nw = stage.clientWidth, nh = stage.clientHeight, nd = window.devicePixelRatio || 1;
      if (nw === W && nh === H && nd === dpr) return;
      const c = W ? C.toWorld(view, W / 2, H / 2) : null; // 가운데를 지킨다
      W = nw; H = nh;
      if (nd !== dpr) { dpr = nd; if (cur) cur.dropAll(); if (prev) prev.dropAll(); cur = prev = null; }
      for (const cv of [imgC, baseC, liveC]) {
        cv.width = Math.round(W * dpr); cv.height = Math.round(H * dpr);
        cv.style.width = W + "px"; cv.style.height = H + "px";
      }
      if (c) view = C.anchor(c, W / 2, H / 2, view.z);
      view = C.boundView(view, W, H);
      baseDirty = liveDirty = imgDirty = true;
      kick();
    }

    // ---- 도구·바탕
    function stageClass() {
      stage.className = "itda-wb-stage bg-" + bg + (space ? " grab" : "") + (prefs.tool === "erase" ? " erasing" : "") + (prefs.tool === "select" ? " selecting" : "")
        + (snapping ? " snapping" : "") + (readOnly ? " readonly" : "") + (locked ? " locked" : "");
    }
    function setTool(t) {
      if (mode && (mode.kind === "draw" || mode.kind === "erase" || imgMode())) return;
      prefs.tool = t; hover = null; liveDirty = true; kick();
      if (t !== "select") select(null);
      stageClass();
      cb.bar();
    }
    function applyBg(b) {
      bg = C.BGS.includes(b) ? b : "dark";
      stageClass();
      if (bg !== "grid") { stage.style.backgroundSize = ""; stage.style.backgroundPosition = ""; }
      imgDirty = true; // 그림 자리 표시의 색도 바탕을 따른다
      invalidate("all"); // 기본 펜 색·형광펜 투명도가 바탕을 따른다
    }
    function setBg(b) {
      if (readOnly || locked || b === bg) return;
      applyBg(b);
      cb.bar();
      cb.change();
    }

    // load 는 판 하나를 연다 — 획·보기·바탕을 바꾸고 되돌리기를 비운다. ro 면 읽기 전용(그리기·지우기·바탕 바꾸기 없음 — 끌기는 이동).
    // 그림은 디코드를 시작해 두고(비동기 — 준비되면 그림 층을 다시 그린다) 앞 판의 bmp 는 모두 close 한다.
    function load(b, ro) {
      mode = null; ptrs.clear(); hover = null;
      strokes.clear(); grid.clear(); totalPts = 0; nextId = 1; undoStack = []; redoStack = [];
      images.clear(); igrid.clear(); imgBytes = 0; nextImg = 1; sel = null; gen++; closeHeld(); snapping = false;
      if (cur) cur.dropAll(); if (prev) prev.dropAll(); cur = prev = null;
      readOnly = !!ro;
      for (const g of b.strokes || []) {
        const s = { id: nextId++, tool: g.tool, color: g.color, w: g.w, pts: g.pts };
        geom(s);
        addStroke(s);
      }
      for (const g of b.images || []) {
        const im = { id: nextImg++, x: g.x, y: g.y, w: g.w, h: g.h, mime: g.mime, data: g.data, bytes: C.b64Bytes(g.data), bmp: null, state: "wait" };
        addImg(im);
        decode(im);
      }
      view = C.boundView(b.view ? { x: b.view.x, y: b.view.y, z: C.clamp(b.view.z, C.ZMIN, C.ZMAX) } : { x: 0, y: 0, z: 1 }, W, H);
      lastZoomAt = 0;
      applyBg(b.bg);
      liveDirty = true;
      cb.bar();
      kick();
    }
    // commit 은 진행 중 그리기·지우기를 거기까지 확정해 판에 넣는다(저장하고 떠나는 모든 길이 먼저 부른다 — 머리 주석 「저장 때」).
    // 이동·오므리기는 그대로 둔다. 펜이 아직 닿아 있어도 이 획은 끝났다 — 떼고 다시 대야 새 획이다.
    // 그림 옮기기·크기 조절도 거기까지 확정한다.
    function commit() {
      if (!mode) return;
      if (mode.kind === "draw") finishDraw();
      else if (mode.kind === "erase") endErase();
      else if (imgMode()) endImage(true);
    }
    // lock 은 판을 바꾸거나 닫는 동안의 잠금이다 — 진행 중 획을 확정하고, 그리기·지우기·되돌리기·배경 바꾸기를 받지 않는다(이동·확대는 된다).
    function lock(on) {
      if (on) { commit(); select(null); }
      locked = !!on; hover = null; liveDirty = true; kick();
      stageClass();
      cb.bar();
    }
    // board 는 지금 판의 획·그림·보기·바탕이다(쓴 차례·넣은 차례 — serialize 에 넘긴다). 그림의 bmp 는 미리보기용이다(serialize 는 쓰지 않는다).
    function board() {
      const list = [...strokes.values()].sort((a, b) => a.id - b.id).map(s => ({ tool: s.tool, color: s.color, w: s.w, pts: s.pts }));
      const imgs = [...images.values()].sort((a, b) => a.id - b.id).map(im => ({ x: im.x, y: im.y, w: im.w, h: im.h, mime: im.mime, data: im.data, bmp: im.bmp }));
      return { bg, view: { x: view.x, y: view.y, z: view.z }, strokes: list, images: imgs };
    }
    function destroy() {
      dead = true;
      gen++; closeHeld();
      if (rafId && window.cancelAnimationFrame) window.cancelAnimationFrame(rafId);
      rafId = 0;
      if (cur) cur.dropAll(); if (prev) prev.dropAll(); cur = prev = null;
      pool.length = 0;
    }

    return {
      load, board, destroy, resize, pointer, leave, wheel, gesture, setSpace, setTool, setBg, undo, redo, colorOf, hlAlpha, kick, frame, commit, lock,
      // busy 는 포인터를 쥔 채 판을 바꾸는 중인가다(그리기·지우기·그림 옮기기·크기 — 이동·오므리기는 아니다). 자동 저장은 이 동안 미룬다(scheduleSave·follow).
      busy: () => !!mode && (mode.kind === "draw" || mode.kind === "erase" || imgMode()), canUndo: () => !readOnly && !locked && undoStack.length > 0, canRedo: () => !readOnly && !locked && redoStack.length > 0,
      readOnly: () => readOnly, locked: () => locked, bg: () => bg, strokes, guard,
      images, insertImage, deleteSelected, imgRoom, gen: () => gen, selected: () => sel, select: im => select(im || null), snapping: () => snapping,
      get view() { return view; }, set view(v) { setView(v); }, get mode() { return mode; }, get size() { return { W, H, dpr }; },
      stats: () => ({ strokes: strokes.size, points: totalPts, images: images.size, imageBytes: imgBytes, undo: undoStack.length, redo: redoStack.length, tiles: (cur ? cur.tiles.size : 0) + (prev ? prev.tiles.size : 0) }),
    };
  }

  // ---- 미리보기: 판 b(parse 의 board 꼴)를 canvas 에 맞춰 그린다(목록 — 획 THUMB_MAX 개까지). 그림은 획 아래에 — layer(thumbLayer 가 그린
  // canvas)가 있으면 그것을, 없으면 그림마다 b.images[i].bmp(지금 판 — 엔진이 든 것)나 옅은 사각형을 그린다.
  // thumbFit 은 판 b 를 W × H 장치 px 에 맞추는 변환 { k, tx, ty } 다(그릴 것이 없으면 null).
  function thumbFit(b, Wd, Hd, d) {
    const list = (b.strokes || []).slice(0, THUMB_MAX), imgs = b.images || [];
    if (!list.length && !imgs.length) return null;
    let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity;
    const take = bx => { x0 = Math.min(x0, bx[0]); y0 = Math.min(y0, bx[1]); x1 = Math.max(x1, bx[2]); y1 = Math.max(y1, bx[3]); };
    for (const s of list) take(C.strokeBox(s.pts, s.w));
    for (const im of imgs) take(C.imageBox(im));
    const pad = 6 * d, k = Math.min((Wd - 2 * pad) / Math.max(1e-6, x1 - x0), (Hd - 2 * pad) / Math.max(1e-6, y1 - y0), 2 * d);
    return { k, tx: (Wd - (x1 - x0) * k) / 2 - x0 * k, ty: (Hd - (y1 - y0) * k) / 2 - y0 * k };
  }
  function thumbImage(ctx, im, bmp, f, darkBg) {
    ctx.setTransform(f.k, 0, 0, f.k, f.tx, f.ty);
    if (bmp) { ctx.drawImage(bmp, im.x, im.y, im.w, im.h); return; }
    ctx.fillStyle = darkBg ? "rgba(255,255,255,.14)" : "rgba(0,0,0,.1)";
    ctx.fillRect(im.x, im.y, im.w, im.h);
  }
  // thumbLayer 는 다른 판의 그림을 하나씩 디코드해 미리보기 크기 canvas 에 그린 것이다(그린 곧바로 close — bmp 를 하나만 든다).
  // 디코드가 안 되는 그림은 옅은 사각형이다. 그림이 없으면 null.
  function thumbLayer(b) {
    const imgs = b.images || [];
    if (!imgs.length) return Promise.resolve(null);
    const d = window.devicePixelRatio || 1, layer = document.createElement("canvas");
    layer.width = Math.round(THUMB_W * d); layer.height = Math.round(THUMB_H * d);
    const ctx = layer.getContext && layer.getContext("2d"), f = thumbFit(b, layer.width, layer.height, d);
    if (!ctx || !f) return Promise.resolve(null);
    let chain = Promise.resolve();
    for (const im of imgs) {
      chain = chain.then(() => decodeImage(im.mime, im.data).then(bmp => {
        try { thumbImage(ctx, im, bmp, f, b.bg === "dark"); } finally { if (bmp && bmp.close) bmp.close(); }
      }, () => thumbImage(ctx, im, null, f, b.bg === "dark")));
    }
    return chain.then(() => layer);
  }
  function drawThumb(cv, b, layer) {
    const d = window.devicePixelRatio || 1, ctx = cv.getContext && cv.getContext("2d");
    cv.width = Math.round(THUMB_W * d); cv.height = Math.round(THUMB_H * d);
    if (!ctx) return;
    const darkBg = b.bg === "dark";
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.fillStyle = darkBg ? "#000" : b.bg === "grid" ? "#fbfbf7" : "#fff";
    ctx.fillRect(0, 0, cv.width, cv.height);
    const f = thumbFit(b, cv.width, cv.height, d);
    if (!f) return;
    if (layer) ctx.drawImage(layer, 0, 0);
    else for (const im of b.images || []) thumbImage(ctx, im, im.bmp || null, f, darkBg);
    const list = (b.strokes || []).slice(0, THUMB_MAX);
    const color = c => (c === 0 ? (darkBg ? "#f2f2f2" : "#141414") : COLORS[c]);
    ctx.setTransform(f.k, 0, 0, f.k, f.tx, f.ty);
    for (const s of list) {
      const pts = C.curve(s.pts, s.w * 0.25);
      if (s.tool === "hl") {
        ctx.globalAlpha = darkBg ? 0.5 : 0.35;
        ctx.strokeStyle = color(s.color); ctx.lineWidth = s.w; ctx.lineCap = "round"; ctx.lineJoin = "round";
        ctx.beginPath(); ctx.moveTo(pts[0], pts[1]);
        for (let i = 3; i < pts.length; i += 3) ctx.lineTo(pts[i], pts[i + 1]);
        if (pts.length === 3) ctx.lineTo(pts[0] + 0.001, pts[1]);
        ctx.stroke();
        ctx.globalAlpha = 1;
      } else {
        const o = C.outline(pts, s.w, 6);
        ctx.fillStyle = color(s.color);
        ctx.beginPath(); ctx.moveTo(o.poly[0], o.poly[1]);
        for (let i = 2; i < o.poly.length; i += 2) ctx.lineTo(o.poly[i], o.poly[i + 1]);
        ctx.closePath(); ctx.fill();
        for (let i = 0; i < o.joins.length; i += 3) { ctx.beginPath(); ctx.arc(o.joins[i], o.joins[i + 1], o.joins[i + 2], 0, Math.PI * 2); ctx.fill(); }
      }
    }
  }

  function fmtTime(ms) {
    if (!Number.isFinite(ms)) return "";
    const d = new Date(ms), n = new Date(), p = x => (x < 10 ? "0" : "") + x;
    return (d.getFullYear() !== n.getFullYear() ? d.getFullYear() + "년 " : "") + (d.getMonth() + 1) + "월 " + d.getDate() + "일 " + p(d.getHours()) + ":" + p(d.getMinutes());
  }

  // ---- 표면
  // 창 붙잡기 단계에서 받는 사건 — 표면 안의 것은 표면이 처리하고 전파를 멈춘다(머리 주석). 키는 어디서 왔든 표면 몫이다.
  const KEY_TYPES = ["keydown", "keyup", "keypress"];
  const CAP_TYPES = ["pointerdown", "pointermove", "pointerup", "pointercancel", "lostpointercapture", "gotpointercapture", "pointerover", "pointerout",
    "pointerenter", "pointerleave", "pointerrawupdate", "mousedown", "mouseup", "mousemove", "mouseover", "mouseout", "click", "dblclick", "auxclick",
    "contextmenu", "wheel", "touchstart", "touchmove", "touchend", "touchcancel", "gesturestart", "gesturechange", "gestureend", "selectstart", "dragstart",
    "input", "change"];
  const NOT_PASSIVE = ["wheel", "touchstart", "touchmove", "gesturestart", "gesturechange", "gestureend"];

  function open(o) {
    o = o || {};
    if (S) return S.ready;
    const host = o.host || window.ItdaWhiteboardHost;
    if (!C) return Promise.reject(new Error("wbcore.js(window.ItdaWbCore)가 먼저 실려야 한다"));
    if (!host || !host.store) return Promise.reject(new Error("저장 계약(ItdaWhiteboardHost.store)이 없다"));
    const s = S = surface(host);
    s.ready = s.start(o.id);
    return s.ready;
  }
  function close() { return S ? S.close() : Promise.resolve(true); }
  function isOpen() { return !!S; }
  function toggle(opts) { return S ? S.close() : open(opts); }

  function surface(host) {
    const store = host.store, B = bagOf(store);
    // st.rec 은 지금 판의 기록기다(st.id 와 함께 바뀐다). st.task 는 지금 도는 시도(판 바꾸기·새 판·관리 작업·처음 열기·닫기 — 한 번에 하나).
    const st = {
      host, id: null, rec: null, name: "", extra: {}, readOnly: true, list: [], timer: 0, task: null,
      closing: null, closeFailed: false, confirm: null, renaming: null, pop: "", listOpen: false, noteKind: "",
      locked: "", gone: false, thumbWait: new Map(), // gone: 지금 판이 남의 삭제(또는 늦은 삭제)로 사라졌다 — 다음 판을 연 뒤 안내한다
    };
    const sp = x => Promise.resolve().then(x); // store 함수가 값을 바로 돌려주거나 던져도 Promise 로 받는다
    // 세션 토큰: alive() 는 이 표면이 아직 열려 있는가다. 늦게 끝난 store 요청은 닫힌 세션이면 DOM·상태를 건드리지 않는다
    // (저장 결과는 기록기가 받는다). 시도 토큰: mine(t) 는 시도 t 가 아직 지금 시도인가다 — 밀려난(닫기)·기한을 넘긴 시도의 늦은 load·show·unlock 은 버린다.
    const alive = () => S === api; // teardown 이 S 를 비운다
    const mine = t => alive() && st.task === t;
    // askBy 는 store 함수 하나를 시각 at 까지 기다려 부른다(넘으면 timeout 오류로 거부). ask 는 지금부터 STORE_WAIT 까지다.
    // 기한이 이미 지났으면 부르지 않고 곧바로 거부한다(시도의 남은 단계가 기한 뒤에 요청을 내지 않게).
    const askBy = (at, name, ...a) => (nowMs() >= at ? Promise.reject(lateErr()) : within(sp(() => store[name](...a)), at));
    const ask = (name, ...a) => askBy(nowMs() + STORE_WAIT, name, ...a);
    // begin 은 시도 하나를 시작한다 — 기한은 요청한 때부터 STORE_WAIT 하나다(앞 저장 대기·save·load·create 를 합쳐). 앞 시도가 있으면 하지 않는다.
    function begin(kind) {
      if (st.task) { showNote("앞 일이 끝나지 않았습니다 — 끝난 뒤 다시 해 보세요.", "op"); return null; }
      return (st.task = { kind, at: nowMs() + STORE_WAIT });
    }
    // end 는 시도 t 를 끝낸다. 지금 판이 그사이 삭제됐으면(buried) 그 시도의 나머지 동기 처리가 끝난 뒤 다음 판으로 옮긴다(moveOn — 그때도 그 판이면).
    function end(t) {
      if (st.task !== t) return;
      st.task = null;
      if (st.rec && st.rec.gone) Promise.resolve().then(maybeMove);
    }

    // ---- DOM
    const root = el("div", "itda-wb");
    root.setAttribute("data-pr-overlay", "");
    root.setAttribute("role", "application");
    root.setAttribute("aria-label", "화이트보드");
    root.tabIndex = -1;
    const stage = root.appendChild(el("div", "itda-wb-stage bg-dark"));
    const imgC = stage.appendChild(el("canvas", "itda-wb-img"));   // 층 차례(아래부터): 그림 → 획 타일 → 진행 중·고르기 표시(엔진 머리 주석 「층」)
    const baseC = stage.appendChild(el("canvas", "itda-wb-base"));
    const liveC = stage.appendChild(el("canvas", "itda-wb-live"));
    const title = root.appendChild(el("div", "itda-wb-title"));
    const note = root.appendChild(el("div", "itda-wb-note"));
    note.hidden = true;
    note.setAttribute("role", "status");
    const busyEl = root.appendChild(el("div", "itda-wb-busy")); // 「여는 중」·「닫는 중」 — 잠근 동안(lock)
    busyEl.hidden = true;
    busyEl.setAttribute("role", "status");

    const bar = root.appendChild(el("div", "itda-wb-bar"));
    const tools = { pen: button("itda-wb-tool", "펜", "tool", "펜 (P)"), hl: button("itda-wb-tool", "형광", "tool", "형광펜 (H)"), erase: button("itda-wb-tool", "지우개", "tool", "지우개 (E)"),
      select: button("itda-wb-tool", "고르기", "tool", "고르기 — 그림 옮기기·크기 (V)") };
    for (const t of Object.keys(tools)) { tools[t].dataset.tool = t; bar.appendChild(tools[t]); }
    const imageBtn = bar.appendChild(button("", "그림", "image", "그림 넣기 (⌘V 로 붙여넣기도 된다)"));
    const fileIn = root.appendChild(el("input", "itda-wb-file")); // 그림 고르기(iPad 에는 ⌘V 가 없다) — 보이지 않고 「그림」 단추가 연다
    fileIn.type = "file"; fileIn.accept = "image/*"; fileIn.hidden = true; fileIn.tabIndex = -1;
    fileIn.setAttribute("aria-hidden", "true");
    const colorBtn = bar.appendChild(button("itda-wb-swbtn", null, "pop", "색"));
    colorBtn.dataset.pop = "color";
    const colorDot = colorBtn.appendChild(el("i", "itda-wb-dot"));
    const widthBtn = bar.appendChild(button("itda-wb-swbtn", null, "pop", "굵기"));
    widthBtn.dataset.pop = "width";
    const widthDot = widthBtn.appendChild(el("i", "itda-wb-wdot"));
    const undoBtn = bar.appendChild(button("", "↶", "undo", "되돌리기 (⌘Z)"));
    const redoBtn = bar.appendChild(button("", "↷", "redo", "다시 하기 (⇧⌘Z)"));
    const bgBtn = bar.appendChild(button("", "배경", "pop", "배경"));
    bgBtn.dataset.pop = "bg";
    bar.appendChild(button("", "판", "list", "판 목록 ([ ] 이전·다음 판)"));
    bar.appendChild(button("", "닫기", "close", "닫기 (W·Esc)"));

    const pops = {};
    for (const k of ["color", "width", "bg"]) { pops[k] = root.appendChild(el("div", "itda-wb-pop itda-wb-pop-" + k)); pops[k].hidden = true; }
    const swatchRow = pops.color.appendChild(el("div", "itda-wb-row"));
    const swatches = COLORS.map((c, i) => { const b = swatchRow.appendChild(button("itda-wb-sw", null, "color", "색 " + (i + 1))); b.dataset.i = String(i); b.appendChild(el("i")); return b; });
    const stepRow = pops.width.appendChild(el("div", "itda-wb-row"));
    const steps = [0, 1, 2].map(i => { const b = stepRow.appendChild(button("itda-wb-sw itda-wb-step", null, "width", "굵기 " + (i + 1))); b.dataset.i = String(i); b.appendChild(el("i")); return b; });
    const sliderRow = pops.width.appendChild(el("div", "itda-wb-row itda-wb-sliderrow"));
    const slider = sliderRow.appendChild(el("input", "itda-wb-slider"));
    slider.type = "range"; slider.step = "1";
    slider.setAttribute("aria-label", "굵기");
    const widthLab = sliderRow.appendChild(el("span", "itda-wb-wlab"));
    const bgRow = pops.bg.appendChild(el("div", "itda-wb-row"));
    const bgBtns = C.BGS.map(b => { const x = bgRow.appendChild(button("", BG_NAMES[b], "bg")); x.dataset.bg = b; return x; });

    const imgDel = root.appendChild(button("itda-wb-imgdel itda-wb-danger", "지우기", "imgdel", "그림 지우기 (Delete)")); // 고른 그림 옆
    imgDel.hidden = true;

    const listEl = root.appendChild(el("div", "itda-wb-list"));
    listEl.hidden = true;
    listEl.setAttribute("role", "dialog");
    listEl.setAttribute("aria-label", "화이트보드 목록");
    const listHead = listEl.appendChild(el("div", "itda-wb-listhead"));
    listHead.appendChild(el("span", "itda-wb-listtitle", "화이트보드"));
    listHead.appendChild(button("", "새 판", "new", "새 판 만들기"));
    listHead.appendChild(button("", "닫기", "listclose", "목록 닫기 (Esc)"));
    const items = listEl.appendChild(el("div", "itda-wb-items"));

    const eng = engine(stage, imgC, baseC, liveC, {
      change, view: () => {}, bar: paintBar, down: closePops, // 보기만 바뀐 것은 저장을 부르지 않는다 — 다음 스냅샷에 함께 실린다
      edge: () => showNote("판의 끝(좌표 범위)에 닿아 획을 끝냈습니다 — 안쪽에서 다시 그으세요.", "full"),
      full: () => showNote("판이 가득 찼습니다(획 " + C.LIMIT.strokes + "·점 " + C.LIMIT.points + " 상한) — 이 획은 넣지 않았습니다.", "full"),
      long: () => showNote("한 획의 점이 " + STROKE_PTS + "개에 닿아 획을 끝냈습니다 — 떼고 다시 그으세요.", "full"),
      imgFull: () => showNote("그림이 가득 찼습니다(판 하나에 " + C.LIMIT.images + "개·" + Math.round(C.LIMIT.imageBytes / 1024 / 1024) + "MB 상한) — 이 그림은 넣지 않았습니다.", "full"),
      sel: placeImgDel,
    });
    // placeImgDel 은 고른 그림(화면 상자 r) 오른쪽 위 옆에 「지우기」 단추를 둔다(화면 안으로 끌어 맞춘다). 고친 수 없는 판이면 숨긴다.
    function placeImgDel(r) {
      if (!r || eng.readOnly() || eng.locked()) { imgDel.hidden = true; return; }
      const { W, H } = eng.size;
      imgDel.hidden = false;
      imgDel.style.left = Math.round(C.clamp(r.x + r.w + 8, 4, Math.max(4, W - 90))) + "px";
      imgDel.style.top = Math.round(C.clamp(r.y, 4, Math.max(4, H - 110))) + "px";
    }
    // insertImage 는 그림(Blob·File)을 줄이고 인코드해(prepareImage) 지금 판의 화면 가운데에 넣는다(머리 주석 「그림」). 읽기 전용·잠금이면 넣지 않는다.
    // 준비하는 사이 판이 바뀌었거나(gen) 잠겼으면 버린다. 넣었으면 true.
    function insertImage(blob) {
      if (!blob || !alive()) return Promise.resolve(false);
      if (eng.readOnly()) { showNote("읽기 전용 판에는 그림을 넣지 않습니다.", "full"); return Promise.resolve(false); }
      if (eng.locked()) return Promise.resolve(false);
      if (!eng.imgRoom(1, 0)) { showNote("그림이 가득 찼습니다(판 하나에 " + C.LIMIT.images + "개 상한) — 이 그림은 넣지 않았습니다.", "full"); return Promise.resolve(false); }
      const g = eng.gen();
      return prepareImage(blob).then(p => {
        if (!alive() || eng.gen() !== g || eng.readOnly() || eng.locked()) return false;
        return eng.insertImage(p, p.iw, p.ih);
      }).catch(err => {
        if (alive() && eng.gen() === g) showNote(err && err.message ? err.message : IMG_READ_FAIL, "full");
        return false;
      });
    }

    // ---- 잠금: 판을 바꾸거나 닫는 동안(머리 주석 「잠금」)
    function lock(text) { st.locked = text; busyEl.textContent = text; busyEl.hidden = false; root.classList.add("busy"); eng.lock(true); }
    // unlock 은 폐기된 판을 든 동안 풀지 않는다(머리 주석 「기록기」 불변식 — 그 판에 그은 것은 저장되지 않는다).
    function unlock() { if (!st.locked || (st.rec && st.rec.gone)) return; st.locked = ""; busyEl.textContent = ""; busyEl.hidden = true; root.classList.remove("busy"); eng.lock(false); }

    // ---- 안내 한 줄
    function showNote(msg, kind) { note.textContent = msg; note.hidden = false; st.noteKind = kind || ""; }
    function clearNote(kind) { if (!kind || st.noteKind === kind) { note.textContent = ""; note.hidden = true; st.noteKind = ""; } }

    // ---- 도구 막대
    const drawKind = () => (prefs.tool === "hl" ? "hl" : "pen");
    function paintBar() {
      for (const t of Object.keys(tools)) { tools[t].classList.toggle("on", prefs.tool === t); tools[t].disabled = eng.readOnly(); }
      undoBtn.disabled = !eng.canUndo();
      redoBtn.disabled = !eng.canRedo();
      bgBtn.disabled = eng.readOnly() || eng.locked();
      imageBtn.disabled = eng.readOnly() || eng.locked();
      const t = drawKind();
      colorDot.style.background = eng.colorOf(prefs.color[t]);
      const wd = Math.max(4, Math.min(28, t === "hl" ? prefs.width[t] * 0.6 : prefs.width[t] * 1.6));
      widthDot.style.width = widthDot.style.height = wd + "px";
      widthDot.classList.toggle("hl", t === "hl");
      swatches.forEach((b, i) => {
        b.classList.toggle("on", prefs.color[t] === i);
        const dot = b.firstElementChild;
        dot.style.background = eng.colorOf(i);
        dot.style.opacity = t === "hl" ? String(eng.hlAlpha() + 0.2) : "";
      });
      steps.forEach((b, i) => {
        const px = WIDTHS[t][i], dot = b.firstElementChild, d = Math.min(30, t === "hl" ? px * 0.7 : px * 2.2);
        b.classList.toggle("on", prefs.width[t] === px);
        dot.style.width = dot.style.height = d + "px";
      });
      slider.min = String(WIDTH_RANGE[t][0]); slider.max = String(WIDTH_RANGE[t][1]); slider.value = String(prefs.width[t]);
      widthLab.textContent = (t === "hl" ? "형광펜 " : "펜 ") + prefs.width[t] + "px";
      for (const b of bgBtns) b.classList.toggle("on", b.dataset.bg === eng.bg());
      title.textContent = st.name + (eng.readOnly() ? " · 읽기 전용" : "");
    }
    function closePops() { for (const k of Object.keys(pops)) pops[k].hidden = true; st.pop = ""; }
    function togglePop(k) { const was = st.pop === k; closePops(); if (!was) { pops[k].hidden = false; st.pop = k; paintBar(); } }

    // ---- 저장
    function change() {
      if (st.noteKind === "full") clearNote("full");
      scheduleSave();
    }
    function scheduleSave() {
      clearTimeout(st.timer);
      // 쓰는·지우는·그림을 끄는 중(eng.busy)이면 넣지 않는다 — save 의 commit 이 진행 중 획을 끊지 않게 다시 2초 뒤로 미룬다
      // (뗀 뒤의 변경이 타이머를 새로 건다). 떠나는 길(닫기·전환·새 판·복제·삭제·숨김)은 미루지 않고 그 자리에서 확정한다.
      st.timer = setTimeout(() => { st.timer = 0; if (!alive()) return; if (eng.busy()) scheduleSave(); else save(); }, SAVE_QUIET);
    }
    const textNow = () => C.serialize(Object.assign(eng.board(), { name: st.name, extra: st.extra }));
    // snapshot 은 진행 중 그리기·지우기를 확정하고 지금 판의 본문을 그 자리에서 기록기에 넣는다(put — 다른 대기 뒤로 미루지 않는다).
    // 돌려주는 값은 기록기 번호(0 = 넣을 것이 없다 — 읽기 전용·판 없음·이미 저장됨·삭제됨), 본문을 만들지 못하면(좌표 범위 — wbcore serialize) -1 이다
    // (그때는 저장을 멈추고 판은 엔진에 그대로 둔 채 안내한다).
    function snapshot() {
      eng.commit();
      if (!st.rec || st.readOnly || st.rec.gone) return 0;
      let text;
      try { text = textNow(); } catch (e) { showNote(RANGE_FAIL, "save"); return -1; }
      return put(st.rec, text);
    }
    // save 는 지금 판을 기록기에 넣고 멈춘 기록기를 다시 움직인다(자동 저장 타이머·숨김). 돌려주는 Promise 는 저장이 확인되면 true.
    // at 을 주면 그 시각까지 지켜본다(시도의 기한 — 없으면 지금부터 STORE_WAIT).
    function save(at) {
      clearTimeout(st.timer); st.timer = 0;
      const n = snapshot();
      if (n <= 0) return Promise.resolve(n === 0);
      retry(st.rec);
      return follow(st.rec, n, at || nowMs() + STORE_WAIT);
    }
    // follow 는 번호 n 의 저장을 at 까지 지켜보고 표면에 알린다 — 성공이면 안내를 거두고 목록의 고친 때를 맞춘다, 실패·시간 넘김이면 안내한다.
    // 닫힌 세션·떠난 판이면 표면을 건드리지 않는다. 저장이 확인되면 true.
    function follow(r, n, at) {
      return watch(r, n, at).then(v => {
        if (!alive() || st.rec !== r) return v === "ok";
        if (v === "ok") {
          const it = st.list.find(x => x.id === r.id);
          if (it && Number.isFinite(r.modified)) { it.modified = r.modified; it.bytes = r.bytes; }
          st.closeFailed = false; // 저장이 한 번 되면 닫기 첫 실패 안내를 다시 센다
          clearNote("save");
          // 저장 중 또 바뀌었다(자동 저장 대기) — 2초를 기다리지 않고 곧바로 한 번 더. 쓰는 중(eng.busy)이면 타이머에 맡긴다(획을 끊지 않게)
          if (st.timer && !st.task && !eng.busy()) save();
        } else if (dirty(r)) showNote(SAVE_FAIL, "save");
        return v === "ok";
      });
    }
    // leaveBy 는 떠나는 길(전환·새 판)의 저장이다 — 잠근 뒤 그 자리에서 본문을 넣고, 시도 t 의 기한까지 그 저장이 확인되기를 기다린다(확인되면 true).
    function leaveBy(t) {
      clearTimeout(st.timer); st.timer = 0;
      const n = snapshot();
      if (n <= 0) return Promise.resolve(n === 0);
      retry(st.rec);
      return follow(st.rec, n, t.at);
    }

    // ---- 목록
    function refreshList(at) {
      return (at ? askBy(at, "list") : ask("list")).then(l => {
        if (!alive()) return st.list;
        st.list = (Array.isArray(l) ? l : []).filter(x => x && x.id != null).map(x => ({ id: x.id, name: String(x.name || ""), modified: Number(x.modified) || 0, bytes: Number(x.bytes) || 0 }));
        st.list.sort((a, b) => b.modified - a.modified);
        if (st.listOpen) paintList();
        return st.list;
      });
    }
    function paintList() {
      items.textContent = "";
      for (const it of st.list) {
        const row = items.appendChild(el("div", "itda-wb-item" + (it.id === st.id ? " on" : "")));
        row.dataset.id = String(it.id);
        const cv = row.appendChild(el("canvas", "itda-wb-thumb"));
        const meta = row.appendChild(el("div", "itda-wb-meta"));
        if (st.renaming === it.id) {
          const inp = meta.appendChild(el("input", "itda-wb-rename"));
          inp.type = "text"; inp.value = it.name; inp.setAttribute("aria-label", "판 이름");
          st.renameEl = inp;
        } else meta.appendChild(el("span", "itda-wb-iname", it.name || "(이름 없음)"));
        meta.appendChild(el("span", "itda-wb-itime", fmtTime(it.modified)));
        const acts = row.appendChild(el("div", "itda-wb-acts"));
        acts.appendChild(button("", "이름", "rename", "이름 바꾸기"));
        acts.appendChild(button("", "복제", "dup", "복제"));
        const del = acts.appendChild(button("itda-wb-danger", st.confirm === it.id ? "정말 삭제" : "삭제", "trash", "삭제"));
        del.classList.toggle("confirm", st.confirm === it.id);
        thumbFor(it, cv);
      }
      if (st.renameEl && st.renameEl.focus) { st.renameEl.focus(); if (st.renameEl.select) st.renameEl.select(); }
    }
    // 미리보기: 지금 판은 엔진의 판으로 바로 그린다. 다른 판은 본문을 읽어(저장하지 못한 본문이 있으면 그것) 작은 canvas 에 그리고, 그 canvas 만
    // 이 저장소의 캐시(B.thumbs — id+modified, 최대 THUMB_CACHE 개, 오래 쓰지 않은 것부터 버림)에 둔다 — parse 한 본문은 들지 않는다.
    // 같은 판을 기다리는 요청은 하나로 합치고(st.thumbWait), 읽기는 한 번에 하나씩 한다(thumbChain).
    let thumbChain = Promise.resolve();
    function blit(cv, src) {
      cv.width = src.width; cv.height = src.height;
      const ctx = cv.getContext && cv.getContext("2d");
      if (ctx) { ctx.setTransform(1, 0, 0, 1, 0, 0); ctx.drawImage(src, 0, 0); }
    }
    function thumbFor(it, cv) {
      if (it.id === st.id) { drawThumb(cv, eng.board()); return; }
      const c = B.thumbs.get(it.id);
      if (c && c.modified === it.modified) { B.thumbs.delete(it.id); B.thumbs.set(it.id, c); blit(cv, c.canvas); return; }
      let w = st.thumbWait.get(it.id);
      if (!w || w.modified !== it.modified) {
        const id = it.id, modified = it.modified;
        const job = thumbChain.then(() => {
          if (!alive() || !st.listOpen) return null;
          const r = B.recs.get(id), mem = dirty(r) ? latestText(r) : null;
          return (mem != null ? Promise.resolve(mem) : ask("load", id)).then(t => {
            const r = typeof t === "string" && t.trim() ? C.parse(t) : null;
            if (!r || !r.ok || !alive()) return null;
            return thumbLayer(r.board).then(layer => ({ r, layer }), () => ({ r, layer: null }));
          }, () => null).then(x => {
            if (!x || !alive()) return null;
            const pic = el("canvas");
            drawThumb(pic, x.r.board, x.layer);
            if (x.layer) { x.layer.width = 0; x.layer.height = 0; }
            if (mem == null) { // 저장소의 본문만 캐시한다(저장하지 못한 본문은 modified 와 맞지 않는다)
              B.thumbs.delete(id); B.thumbs.set(id, { modified, canvas: pic });
              while (B.thumbs.size > THUMB_CACHE) B.thumbs.delete(B.thumbs.keys().next().value);
            }
            return pic;
          }, () => null);
        });
        thumbChain = job.catch(() => null);
        w = { modified, job };
        st.thumbWait.set(id, w);
        const mine = w;
        job.then(() => { if (st.thumbWait.get(id) === mine) st.thumbWait.delete(id); }, () => {});
      }
      w.job.then(pic => { if (pic && alive()) blit(cv, pic); }, () => {});
    }
    function openList() { closePops(); st.listOpen = true; listEl.hidden = false; refreshList().then(() => paintList(), () => paintList()); paintList(); }
    function closeList() { st.listOpen = false; st.confirm = null; st.renaming = null; st.renameEl = null; listEl.hidden = true; items.textContent = ""; }

    // ---- 판 읽기·바꾸기
    function emptyBoard(name) { return { name: name || "", bg: "dark", view: { x: 0, y: 0, z: 1 }, strokes: [], images: [], extra: {} }; }
    const why = err => (err && err.message ? " — " + err.message : "");
    // show 는 판을 보인다 — 지금 판의 기록기를 바꾸고(연 세션 수), 이름은 목록(호스트)의 것을 먼저 쓴다(rename 뒤 본문 이름이 낡았어도 다음 저장에서 맞춘다).
    // 깨끗한 기록기면 지금 엔진의 본문을 「저장소에 있는 본문」 으로 둔다(바뀐 것 없이 떠나면 저장하지 않게). fresh 면 두지 않는다(새 판 — 곧바로 쓴다).
    function show(id, board, ro, msg, fresh) {
      if (st.rec) { st.rec.editors--; prune(st.rec); }
      st.rec = id != null ? recOf(B, id) : null;
      if (st.rec) st.rec.editors++;
      const it = st.list.find(x => x.id === id);
      st.id = id; st.name = (it && it.name) || board.name || ""; st.extra = board.extra || {}; st.readOnly = !!ro;
      st.closeFailed = false;
      if (id != null) B.lastId = id;
      eng.load(board, ro);
      const r = st.rec;
      if (r && !ro && !fresh && !dirty(r) && !r.running) { try { r.saved = textNow(); } catch (e) { r.saved = null; } }
      clearNote();
      if (msg) showNote(msg, "ro");
      paintBar();
    }
    // fetchBoard 는 판 id 의 본문(기록기에 저장하지 못한 본문이 있으면 그것)을 시각 at 까지 읽어 열 꼴 { board, ro, note, mem } 로 만든다.
    // load 실패·시간 넘김은 거부.
    function fetchBoard(id, at) {
      const r = B.recs.get(id), mem = dirty(r) ? latestText(r) : null;
      return (mem != null ? Promise.resolve(mem) : askBy(at, "load", id)).then(text => {
        if (typeof text === "string" && !text.trim()) { const it = st.list.find(x => x.id === id); return { board: emptyBoard(it && it.name), ro: false }; }
        const r = C.parse(typeof text === "string" ? text : "");
        if (!r.ok) return { board: emptyBoard(), ro: true, note: "이 판을 읽지 못해 읽기 전용으로 열었습니다 — " + r.error };
        return { board: r.board, ro: r.readOnly, note: r.readOnly ? "새 판 형식(v" + r.v + ")이라 읽기만 합니다." : "", mem: mem != null && !r.readOnly };
      });
    }
    // showFetched 는 읽은 판을 보인다 — 기록기에 남은 본문으로 열었으면 곧바로 다시 저장해 본다(retry — 도는 save 가 있으면 그 뒤에).
    function showFetched(id, f) {
      show(id, f.board, f.ro, f.note);
      if (f.mem && st.rec) { retry(st.rec); follow(st.rec, latest(st.rec), nowMs() + STORE_WAIT); }
    }
    // loadBoard 는 돌아갈 판이 없는 길(열기·지금 판 삭제)의 읽기다(시도 t — 그 기한까지) — 읽지 못하면 빈 판을 읽기 전용으로 보인다.
    // 기한을 넘긴 것은 그대로 거부한다(시도가 기한 정책을 따른다 — start·moveOn).
    function loadBoard(id, t) {
      return fetchBoard(id, t.at).then(f => { if (mine(t)) showFetched(id, f); },
        err => { if (err && err.timeout) throw err; if (mine(t)) show(id, emptyBoard(), true, "이 판을 불러오지 못했습니다" + why(err) + "."); });
    }
    // makeBoard 는 store.create 로 새 판을 시각 at 까지 만든다(보이지는 않는다 — showNew).
    function makeBoard(at) {
      const name = "화이트보드 " + (st.list.length + 1);
      return askBy(at, "create", name).then(r => {
        if (!r || r.id == null) throw new Error("create 가 id 를 주지 않았다");
        return { id: r.id, name };
      });
    }
    // showNew 는 만든 판을 보이고 곧바로 빈 판 본문을 쓴다(머리 주석 「저장 계약」 create). 저장·목록 읽기를 기다리는 Promise 를 돌려준다.
    // at 을 주면 그 저장·목록 읽기를 그 시각까지 기다린다(시도의 기한 — 없으면 저마다 STORE_WAIT).
    function showNew(n, at) {
      st.list.unshift({ id: n.id, name: n.name, modified: Date.now(), bytes: 0 });
      show(n.id, emptyBoard(n.name), false, "", true);
      return save(at).then(() => (alive() ? refreshList(at) : null)).catch(() => {}).then(() => { if (alive() && st.listOpen) paintList(); });
    }
    // createBoard 는 돌아갈 판이 없는 길(열기·지금 판 삭제)의 새 판이다 — create·첫 save·목록 읽기 모두 시도 t 의 기한까지.
    function createBoard(t) { return makeBoard(t.at).then(n => (mine(t) ? showNew(n, t.at) : undefined)); }
    // stay 는 판 바꾸기가 실패해 원래 판에 남을 때다 — 잠금을 풀고 안내한다(떠날 때 넣은 본문은 기록기에 있다 — 다음 변경 때 다시 저장한다).
    // 그사이 지금 판이 삭제됐으면(buried) 잠금은 풀리지 않고 실패 까닭 대신 삭제를 안내한다(end 가 다음 판으로 옮긴다).
    function stay(t, msg) {
      end(t);
      unlock();
      showNote(st.rec && st.rec.gone ? GONE + " — 다음 판을 엽니다." : msg, "op");
    }
    // switchTo 는 판 바꾸기다(머리 주석 「떠나기」) — 잠그고 그 자리에서 지금 판을 넣은 뒤, 그 저장이 확인돼야 대상을 읽는다.
    // 저장이 실패하거나, load 가 실패하거나, 요청부터 STORE_WAIT 를 넘기면 원래 판에 남는다(그 시도의 늦은 load 는 버린다).
    function switchTo(id) {
      if (id === st.id) return Promise.resolve();
      const t = begin("switch");
      if (!t) return Promise.resolve();
      lock("여는 중");
      const from = st.name;
      return leaveBy(t).then(ok => {
        if (!mine(t)) return;
        if (!ok) { stay(t, "‘" + from + "’ 을 저장하지 못해 판을 바꾸지 않았습니다 — 판은 이 화면에 그대로 있습니다."); return; }
        return fetchBoard(id, t.at).then(f => {
          if (!mine(t)) return;
          end(t);
          showFetched(id, f);
          unlock();
        }, err => { if (mine(t)) stay(t, "판을 열지 못해 원래 판으로 돌아왔습니다" + why(err) + "."); });
      });
    }
    function stepBoard(dir) {
      if (!st.list.length) return;
      const i = st.list.findIndex(x => x.id === st.id), n = st.list.length;
      const j = i < 0 ? 0 : (i + dir + n) % n;
      if (st.list[j].id !== st.id) switchTo(st.list[j].id);
    }
    // newBoard 는 새 판 만들기다 — switchTo 처럼 잠그고 지금 판의 저장을 확인한 뒤 만든다. 저장·create 가 실패하거나 요청부터 STORE_WAIT 를 넘기면
    // 원래 판에 남는다(기한 뒤 늦게 된 create 는 버린다 — 호스트에 빈 판이 남을 수 있다).
    function newBoard() {
      const t = begin("new");
      if (!t) return Promise.resolve();
      lock("여는 중");
      const from = st.name;
      return leaveBy(t).then(ok => {
        if (!mine(t)) return;
        if (!ok) { stay(t, "‘" + from + "’ 을 저장하지 못해 새 판을 만들지 않았습니다 — 판은 이 화면에 그대로 있습니다."); return; }
        return refreshList(t.at).catch(() => {}) // 새 판 이름의 번호를 맞추려는 것 — 실패해도 만든다
          .then(() => (mine(t) ? makeBoard(t.at) : null))
          .then(n => {
            if (!n || !mine(t)) return;
            end(t);
            const saved = showNew(n);
            unlock();
            return saved;
          }, err => { if (mine(t)) stay(t, "새 판을 만들지 못해 원래 판으로 돌아왔습니다" + why(err) + "."); });
      });
    }
    // manage 는 시작한 관리 요청(store.rename·duplicate·trash) 하나다 — 부르는 쪽이 판 r 의 기록기를 막아 둔 채(r.blocked) 부르고,
    // 요청이 끝나면(기한을 넘겼어도 끝날 때까지 기다려) 푼다 — 그동안 그 판의 새 save 를 내지 않는다. h.settle(ok, v) 는 요청이 끝날 때 늘(세션이 닫혔어도)
    // 막기를 풀기 전에 부른다(삭제 폐기·이름 맞추기). 시도 t 의 기한 안에 끝나면 h.ok(v)·h.fail(e), 넘기면 h.unsure() — 늦게 끝나면 h.late(ok).
    function manage(t, r, name, args, h) {
      let late = false;
      const p = sp(() => store[name](...args));
      const fin = (ok, v) => { call(h.settle, ok, v); r.blocked--; pump(r); prune(r); if (late) call(h.late, ok); };
      p.then(v => fin(true, v), e => fin(false, e));
      return within(p, t.at).then(v => h.ok(v), e => { if (e && e.timeout) { late = true; return h.unsure(); } return h.fail(e); });
    }
    const paintIfOpen = () => { if (alive() && st.listOpen) paintList(); };
    const UNSURE = "결과를 확인하지 못했습니다 — 끝날 때까지 그 판은 저장하지 않습니다(목록을 다시 열어 확인하세요).";
    // duplicate 는 복제다 — 지금 판이면 잠그고 그 자리에서 본문을 넣는다. 그 판의 기록기가 깨끗해진 뒤에만 store.duplicate 를 부른다(미저장이면 다시
    // 저장해 보고, 기한 안에 깨끗해지지 않으면 철회한다 — 뒤늦게 부르지 않는다). 옛 본문으로 사본을 만들지 않게.
    function duplicate(id) {
      const t = begin("dup");
      if (!t) return Promise.resolve();
      const current = id === st.id, it = st.list.find(x => x.id === id), name = it ? it.name : "";
      let n = 0;
      if (current) { lock("복제하는 중"); clearTimeout(st.timer); st.timer = 0; n = snapshot(); }
      const r = recOf(B, id);
      let ready = Promise.resolve(n < 0 ? "fail" : "ok");
      if (n >= 0 && dirty(r)) { retry(r); ready = watch(r, latest(r), t.at); }
      return ready.then(v => {
        if (!mine(t)) { prune(r); return; } // 닫기가 밀어냈다 — 철회
        if (v !== "ok" || dirty(r)) {
          end(t);
          if (current) unlock();
          showNote(current ? "지금 판을 저장하지 못해 복제하지 않았습니다 — 저장된 뒤 다시 해 보세요."
            : "‘" + name + "’ 의 저장하지 못한 본문을 저장하지 못해 복제하지 않았습니다.", "op");
          prune(r);
          return;
        }
        r.blocked++; // 깨끗함을 확인한 그 자리에서 막는다 — 요청이 끝날 때까지 이 판의 save 가 끼지 않는다
        return manage(t, r, "duplicate", [id], {
          ok: () => { if (!mine(t)) return; end(t); if (current) unlock(); return refreshList().catch(() => {}).then(paintIfOpen); },
          fail: () => { if (!mine(t)) return; end(t); if (current) unlock(); showNote("복제하지 못했습니다.", "op"); },
          unsure: () => { if (!mine(t)) return; end(t); if (current) unlock(); showNote("복제 " + UNSURE, "op"); },
        });
      });
    }
    // trash 는 삭제다 — 지금 판이면 잠근다(지울 판에 그리지 않게 — 진행 중 획은 확정되지만 이 판은 저장하지 않는다). 요청한 그 자리에서 그 판의 기록기를
    // 막고(새 save 를 내지 않는다), 도는 save 가 끝난 뒤에만 store.trash 를 부른다 — 기한 안에 시작하지 못하면 철회한다(뒤늦게 지우지 않는다).
    // 앞지르지 않는 까닭: 늦게 끝난 save 가 지운 판을 되살릴 수 있다. 성공하면 기록기를 폐기한다(bury — 옛 put·retry 를 받지 않는다).
    function trash(id) {
      const t = begin("trash");
      if (!t) return Promise.resolve();
      const current = id === st.id, it = st.list.find(x => x.id === id), name = it ? it.name : "";
      if (current) { lock("지우는 중"); clearTimeout(st.timer); st.timer = 0; }
      const r = recOf(B, id);
      t.trash = r; // 기한 안에 된 내 삭제는 안내 없이 다음 판으로 옮긴다(buried)
      r.blocked++;
      const back = () => { // 지우지 않았다 — 판은 그대로, 지금 판의 바뀐 것은 다시 저장한다
        end(t); st.confirm = null;
        if (current) { unlock(); scheduleSave(); }
        paintIfOpen();
      };
      return idle(r, t.at).then(v => {
        if (v !== "ok" || !mine(t)) { // 철회 — 막기를 풀고 부르지 않는다
          r.blocked--; pump(r); prune(r);
          if (mine(t)) { back(); showNote("앞 저장이 끝나지 않아 ‘" + name + "’ 을 삭제하지 않았습니다.", "op"); }
          return;
        }
        return manage(t, r, "trash", [id], {
          settle: ok => { if (ok) bury(r); },
          // 지금 판이었으면 bury 가 이미 잠그고(buried) end 가 옮긴다 — 여기서는 옮김(moveOn)을 곧바로 시작해 그 Promise 를 돌려줄 뿐이다
          ok: () => { if (!mine(t)) return; end(t); st.confirm = null; return st.rec === r ? maybeMove() : refreshList().catch(() => {}).then(paintIfOpen); },
          fail: () => { if (!mine(t)) return; back(); showNote("삭제하지 못했습니다.", "op"); },
          unsure: () => { if (!mine(t)) return; back(); showNote("‘" + name + "’ 삭제 " + UNSURE, "op"); },
          // 늦게 된 삭제 — 그 판을 보는 세션(이 세션이 아니어도)은 bury 가 알린다. 이 세션이 살아 있고 다른 판을 보고 있으면 목록만 다시 읽는다
          late: ok => { if (ok && alive() && st.rec !== r) refreshList().catch(() => {}).then(paintIfOpen); },
        });
      });
    }
    // buried 는 지금 판이 삭제됐다는 알림이다(bury — 누가 낸 삭제든, 언제 되든). 그 자리에서 진행 중 획을 확정하고 잠근다(이 판은 다시 풀지 않는다 —
    // unlock). 기한 안에 된 내 삭제(시도 t.trash)가 아니면 「삭제되었습니다」 를 안내한다. 도는 시도가 없으면 곧바로 다음 판으로 옮기고, 있으면 그 시도가
    // 끝날 때(end) 옮긴다.
    const GONE = "이 화이트보드는 삭제되었습니다";
    function buried() {
      const mineTrash = !!(st.task && st.task.trash === st.rec);
      lock(mineTrash ? "지우는 중" : GONE);
      clearTimeout(st.timer); st.timer = 0; // 잠그며 확정한 획의 자동 저장 — 폐기된 판이라 넣을 곳이 없다
      if (!mineTrash) { st.gone = true; showNote(GONE + " — 다음 판을 엽니다.", "op"); }
      if (!st.task) moveOn();
    }
    function maybeMove() { return alive() && !st.task && st.rec && st.rec.gone ? moveOn() : undefined; }
    // moveOn 은 지금 판이 삭제된 뒤다 — 남은 판 중 가장 늦은 판(없으면 새 판)을 연다(잠근 채 — 새 시도, 기한은 그때부터 STORE_WAIT 하나:
    // list·load·create·새 판의 첫 save·목록 읽기를 합쳐). 기한을 넘기면 잠근 채 「판을 열지 못했습니다」 와 목록을 보여 고르게 한다
    // (그 시도의 늦은 load·create 는 버린다). 기한 안의 list·load 실패는 예전처럼 빈 판·읽기 전용으로 보인다.
    function moveOn() {
      const t = (st.task = { kind: "next", at: nowMs() + STORE_WAIT });
      lock("여는 중");
      if (st.rec) { st.rec.editors--; prune(st.rec); }
      st.rec = null; st.id = null; // 이제부터 저장은 이 판으로 가지 않는다
      const told = !!st.gone; st.gone = false;
      return refreshList(t.at).then(() => {
        if (!mine(t)) return;
        return st.list.length ? loadBoard(st.list[0].id, t) : createBoard(t);
      }).then(() => { if (mine(t) && nowMs() >= t.at) throw lateErr(); }) // 새 판의 첫 save·목록 읽기가 기한까지 끝나지 않았다
        .catch(err => {
          if (!mine(t)) return;
          if (err && err.timeout) { // 기한을 넘겼다 — 잠근 채 목록에서 고르게 한다(고른 판이 뜨면 switchTo·newBoard 가 푼다)
            end(t);
            show(null, emptyBoard(), true, "");
            lock("판을 열지 못했습니다");
            showNote("판을 열지 못했습니다" + why(err) + " — 목록에서 열 판을 고르세요.", "op");
            if (!st.listOpen) openList();
            return;
          }
          if (st.id == null) show(null, emptyBoard(), true, "다음 판을 열지 못했습니다" + why(err) + ".");
        })
        .then(() => {
          if (!mine(t)) return paintIfOpen();
          end(t); unlock();
          if (told && !st.noteKind) showNote(GONE + " — 다음 판을 열었습니다.", "op");
          paintIfOpen();
        });
    }
    // rename 은 이름 바꾸기다 — 지금 판이면 잠근다. 요청이 끝날 때까지 그 판의 save 를 내지 않고, 되면 본문의 이름도 맞춘다(renamed — 세션이 닫혔어도).
    function rename(id, name) {
      name = String(name || "").trim();
      st.renaming = null; st.renameEl = null;
      const it = st.list.find(x => x.id === id);
      if (!name || !it || it.name === name) { paintIfOpen(); return Promise.resolve(); }
      const t = begin("rename");
      if (!t) return Promise.resolve();
      const current = id === st.id, r = recOf(B, id);
      if (current) lock("이름 바꾸는 중");
      r.blocked++;
      const done = () => { end(t); if (current) unlock(); };
      return manage(t, r, "rename", [id, name], {
        settle: ok => { if (ok) renamed(r, name); },
        ok: () => { if (!mine(t)) return; done(); it.name = name; return refreshList().catch(() => {}).then(paintIfOpen); },
        fail: () => { if (!mine(t)) return; done(); showNote("이름을 바꾸지 못했습니다.", "op"); paintIfOpen(); },
        unsure: () => { if (!mine(t)) return; done(); showNote("이름 바꾸기 " + UNSURE, "op"); paintIfOpen(); },
      });
    }

    // ---- 사건(창 붙잡기 단계 — 머리 주석)
    const isText = t => !!(t && t.tagName === "INPUT" && t.type === "text");
    const inside = t => !!(t && t !== window && t !== document && root.contains(t));
    function onKey(e) {
      const t = e.target;
      if (e.type !== "keydown") {
        if (e.type === "keyup" && (e.code === "Space" || e.key === " ")) { eng.setSpace(false); if (!isText(t)) e.preventDefault(); }
        return;
      }
      if (isText(t) && inside(t)) { // 이름 고치기 — 글자는 기본 동작 그대로, Enter 확정·Esc 취소
        if (e.key === "Enter" && !e.isComposing) { e.preventDefault(); const id = st.renaming; rename(id, t.value); }
        else if (e.key === "Escape") { e.preventDefault(); st.renaming = null; st.renameEl = null; paintList(); }
        return;
      }
      const code = e.code || "", k = e.key || "", mod = e.metaKey || e.ctrlKey;
      const onButton = t && t.tagName === "BUTTON" && inside(t);
      if (k === "Tab" || (onButton && (k === "Enter" || k === " "))) return; // 포커스 이동·단추 누르기는 기본 동작
      if (mod && !e.altKey && !e.shiftKey && (code === "KeyV" || k.toLowerCase() === "v")) return; // ⌘V — 기본 동작을 막으면 paste 사건이 오지 않는다(그림 넣기는 onPaste)
      e.preventDefault();
      const is = (c, ch) => code === c || k.toLowerCase() === ch;
      if (k === "Escape") { escape(); return; }
      if (code === "Space" || k === " ") { eng.setSpace(true); return; }
      if (mod && is("KeyZ", "z")) { e.shiftKey ? eng.redo() : eng.undo(); return; }
      if (mod && is("KeyY", "y")) { eng.redo(); return; }
      if (mod || e.altKey) return;
      if (is("KeyW", "w")) { close(); return; }
      if (st.listOpen) return; // 목록이 열린 동안 도구 키는 쉰다
      if (is("KeyP", "p")) eng.setTool("pen");
      else if (is("KeyH", "h")) eng.setTool("hl");
      else if (is("KeyE", "e")) eng.setTool("erase");
      else if (is("KeyV", "v")) eng.setTool("select");
      else if (k === "Delete" || k === "Backspace") eng.deleteSelected();
      else if (is("BracketLeft", "[")) stepBoard(-1);
      else if (is("BracketRight", "]")) stepBoard(1);
    }
    function escape() {
      if (eng.selected()) { eng.select(null); return; } // 고르기 풀기가 먼저
      if (st.renaming != null) { st.renaming = null; st.renameEl = null; paintList(); return; }
      if (st.confirm != null) { st.confirm = null; paintList(); return; }
      if (st.listOpen) { closeList(); return; }
      if (st.pop) { closePops(); return; }
      close();
    }
    function onClick(e, t) {
      const b = t.closest && t.closest("button");
      if (!b || !inside(b)) {
        const row = t.closest && t.closest(".itda-wb-item");
        if (row && !(t.closest && t.closest("input"))) { const id = idOf(row); if (id !== undefined) { closeList(); switchTo(id); } }
        return;
      }
      if (b.disabled) return;
      const a = b.dataset.act, row = b.closest(".itda-wb-item"), id = row ? idOf(row) : undefined;
      if (a !== "trash") st.confirm = null;
      if (a === "tool") { eng.setTool(b.dataset.tool); closePops(); }
      else if (a === "pop") togglePop(b.dataset.pop);
      else if (a === "color") { prefs.color[drawKind()] = Number(b.dataset.i); paintBar(); }
      else if (a === "width") { prefs.width[drawKind()] = WIDTHS[drawKind()][Number(b.dataset.i)]; paintBar(); }
      else if (a === "bg") eng.setBg(b.dataset.bg);
      else if (a === "undo") eng.undo();
      else if (a === "redo") eng.redo();
      else if (a === "image") { closePops(); fileIn.value = ""; if (fileIn.click) fileIn.click(); } // 고르면 change(route)
      else if (a === "imgdel") eng.deleteSelected();
      else if (a === "list") { if (st.listOpen) closeList(); else openList(); }
      else if (a === "close") close();
      else if (a === "listclose") closeList();
      else if (a === "new") newBoard();
      else if (a === "rename" && id !== undefined) { st.renaming = id; paintList(); }
      else if (a === "dup" && id !== undefined) duplicate(id);
      else if (a === "trash" && id !== undefined) { if (st.confirm === id) trash(id); else { st.confirm = id; paintList(); } }
      if (b.blur) b.blur();
    }
    const idOf = row => { const it = st.list.find(x => String(x.id) === row.dataset.id); return it ? it.id : undefined; };
    function route(e, t) {
      const type = e.type, onStage = stage.contains(t), inList = listEl.contains(t);
      if (/^pointer(down|move|up|cancel)$|^lostpointercapture$/.test(type)) { if (onStage) eng.pointer(type, e); }
      else if (type === "pointerleave") { if (t === stage) eng.leave(); }
      else if (type === "wheel") { if (onStage) eng.wheel(e); else if (!inList) e.preventDefault(); }
      else if (type === "touchstart") { if (onStage) e.preventDefault(); }
      else if (type === "touchmove") { if (!inList) e.preventDefault(); }
      else if (/^gesture/.test(type)) eng.gesture(type, e);
      else if (type === "contextmenu" || type === "dblclick" || type === "dragstart" || (type === "selectstart" && !isText(t))) e.preventDefault();
      else if (type === "click") onClick(e, t);
      else if (type === "change" && t === fileIn) { const f = fileIn.files && fileIn.files[0]; fileIn.value = ""; if (f) insertImage(f); }
      else if (type === "input" && t === slider) {
        const k = drawKind();
        prefs.width[k] = Number(slider.value);
        widthLab.textContent = (k === "hl" ? "형광펜 " : "펜 ") + prefs.width[k] + "px";
        for (const b of steps) b.classList.toggle("on", WIDTHS[k][Number(b.dataset.i)] === prefs.width[k]);
      }
    }
    function onCap(e) {
      if (S !== api) return;
      if (KEY_TYPES.includes(e.type)) { onKey(e); e.stopPropagation(); return; }
      const t = e.target;
      if (!inside(t) && !/^gesture/.test(e.type)) return; // 표면 밖(호스트가 더 위에 띄운 층 등)은 건드리지 않는다
      route(e, t);
      e.stopPropagation();
    }
    // 붙여넣기(⌘V) — 입력 칸(표면 안의 이름 칸이든 호스트 층의 칸이든)에 초점이 있으면 건드리지 않는다(글자 붙여넣기는 기본 동작).
    // 그 밖에는 표면 몫이다(아래 화면으로 보내지 않는다) — 클립보드의 첫 image/* 를 넣고, 그림이 없으면 아무 일도 하지 않는다.
    const isField = t => !!t && (t.tagName === "TEXTAREA" || t.isContentEditable === true
      || (t.tagName === "INPUT" && !/^(range|button|checkbox|radio|file|submit|reset|color|image)$/i.test(t.type || "")));
    function onPaste(e) {
      if (S !== api || isField(e.target) || isField(document.activeElement)) return;
      e.stopPropagation();
      const f = clipImage(e.clipboardData);
      if (!f) return;
      e.preventDefault();
      insertImage(f);
    }
    // 숨김 저장 — 처리기 안에서 곧바로 진행 중 획을 확정하고 기록기에 넣는다(다른 대기 뒤로 미루지 않는다 — 머리 주석 「저장 때」)
    const hideSave = () => { save(); };
    const onHidden = () => { if (document.visibilityState === "hidden") hideSave(); };
    const onPageHide = () => { hideSave(); };
    const onResize = () => eng.resize();
    const onOrient = () => setTimeout(onResize, 50);
    const onBlur = () => eng.setSpace(false);
    const listeners = [];
    function listen(target, type, f, opts) { if (!target || !target.addEventListener) return; target.addEventListener(type, f, opts); listeners.push([target, type, f, opts]); }

    // ---- 열기·닫기
    let prevFocus = null;
    function start(id) {
      prevFocus = document.activeElement || null;
      for (const t of KEY_TYPES) listen(window, t, onCap, true);
      listen(window, "paste", onPaste, true);
      for (const t of CAP_TYPES) listen(window, t, onCap, NOT_PASSIVE.includes(t) ? { capture: true, passive: false } : true);
      listen(document, "visibilitychange", onHidden);
      listen(window, "pagehide", onPageHide);
      listen(window, "resize", onResize);
      listen(window, "orientationchange", onOrient);
      listen(window, "blur", onBlur);
      if (window.visualViewport) listen(window.visualViewport, "resize", onResize);
      (host.mount || document.body).appendChild(root);
      if (root.focus) root.focus();
      eng.resize();
      paintBar();
      call(host.onOpen);
      lock("여는 중"); // 첫 판이 뜰 때까지
      // 닫기가 밀어내면 늦은 load·create 는 판을 열지 않는다. 기한(요청부터 STORE_WAIT 하나 — list·load·create·새 판의 첫 save·목록 읽기를 합쳐)을
      // 넘기면 표면을 떼고 거부한다(머리 주석 open — 안내는 호스트 몫).
      const t = (st.task = { kind: "open", at: nowMs() + STORE_WAIT });
      return refreshList(t.at).then(() => {
        if (!mine(t)) return;
        const has = x => x != null && st.list.some(i => i.id === x);
        const pick = has(id) ? id : has(B.lastId) ? B.lastId : st.list.length ? st.list[0].id : null;
        return pick != null ? loadBoard(pick, t) : createBoard(t);
      }).then(() => { if (mine(t) && nowMs() >= t.at) throw lateErr(); })
        .catch(err => {
          if (err && err.timeout) throw err;
          if (mine(t) && st.id == null) show(null, emptyBoard(), true, "판 목록을 읽지 못했습니다" + why(err) + ".");
        }).then(() => { if (mine(t)) { end(t); unlock(); } return st.id; }, err => {
          if (!mine(t)) return st.id; // 닫기가 밀어냈다 — 그 닫기가 표면을 뗀다
          teardown(true);
          throw err;
        });
    }
    function teardown(failed) {
      const kept = dirty(st.rec); // 저장하지 못한 본문이 기록기에 남았다(같은 store 로 다시 열면 그것으로 연다)
      if (S === api) S = null; // 세션 토큰(alive) — 이 뒤에 끝나는 store 요청은 DOM·상태를 건드리지 않는다
      st.task = null;
      if (st.rec) { st.rec.editors--; prune(st.rec); }
      clearTimeout(st.timer); st.timer = 0;
      for (const [t, type, f, opts] of listeners) t.removeEventListener(type, f, opts);
      listeners.length = 0;
      eng.destroy();
      root.remove();
      if (prevFocus && prevFocus.focus) { try { prevFocus.focus(); } catch (e) { /* 떨어진 요소 */ } }
      call(host.onClose, failed ? { unsaved: !!kept, failed: true } : { unsaved: !!kept });
    }
    // closeSurface 는 닫기다(머리 주석 close) — 앞 시도(전환 등)를 밀어내고(그 늦은 결과는 버린다) 잠근 뒤, 그 자리에서 지금 판을 기록기에 넣는다.
    // 기다리는 것은 그 저장의 확인 하나이고, 기한은 닫기를 요청한 때부터 STORE_WAIT 다(앞 일을 기다리지 않는다).
    function closeSurface() {
      if (st.closing) return st.closing;
      const t = (st.task = { kind: "close", at: nowMs() + STORE_WAIT });
      lock("닫는 중");
      const r = st.rec, n = snapshot();
      if (n < 0) { // 본문을 만들지 못한다 — 닫으면 판을 잃으므로 닫지 않는다(엔진에 그대로 둔다)
        end(t); unlock();
        showNote(RANGE_FAIL, "save");
        return Promise.resolve(false);
      }
      if (n > 0) retry(r);
      st.closing = (n > 0 ? watch(r, n, t.at) : Promise.resolve("ok")).then(v => {
        st.closing = null;
        if (!alive()) return true;
        // 저장됨·지운 판·시간 넘김(첫 닫기라도 — 본문은 기록기에 남는다)·두 번째 실패면 닫는다
        if (v === "ok" || r.gone || v === "timeout" || st.closeFailed) { teardown(); return true; }
        st.closeFailed = true; // 처음 거부는 닫지 않는다 — 안내를 보이고, 한 번 더 닫으면 다시 저장해 보고 또 실패하면 기록기에 남긴 채 닫는다
        end(t); unlock();
        showNote("저장하지 못해 닫지 않았습니다 — 한 번 더 닫으면 판을 이 페이지의 메모리에 남긴 채 닫습니다.", "save");
        return false;
      });
      return st.closing;
    }
    // applyName 은 다른 곳(닫힌 세션이 낸 rename 등)에서 된 이름 바꾸기를 지금 판에 맞춘다(renamed) — 진행 중 획은 건드리지 않고 끝난 획으로 넣는다.
    function applyName(name) {
      st.name = name;
      const it = st.list.find(x => x.id === st.id);
      if (it) it.name = name;
      paintBar();
      if (st.readOnly || !st.rec) return;
      let text;
      try { text = textNow(); } catch (e) { return; }
      const n = put(st.rec, text);
      if (n > 0) { retry(st.rec); follow(st.rec, n, nowMs() + STORE_WAIT); }
    }

    const api = { start, close: closeSurface, root, engine: eng, state: st, save, bag: B, applyName, buried, insertImage };
    return api;
  }

  // unsaved 는 store 의 미저장 판 id 다(머리 주석 「unsaved」) — bags.get 으로만 본다(조회가 bag·기록기를 만들지 않는다). 판별은 dirty 그대로다.
  function unsaved(store) {
    if (store === null || (typeof store !== "object" && typeof store !== "function")) throw new TypeError("ItdaWhiteboard.unsaved: store 객체가 필요합니다");
    const B = bags.get(store), out = [];
    if (B) for (const [id, r] of B.recs) if (dirty(r)) out.push(id);
    return out;
  }

  // memoryStore 는 저장 계약(머리 주석)을 메모리로 따르는 저장소다 — 시험·데모용.
  function memoryStore() {
    const files = new Map();
    let n = 0, clock = 0;
    const tick = () => (clock = Math.max(clock + 1, Date.now()));
    const get = id => { const f = files.get(id); if (!f) throw new Error("없는 판: " + id); return f; };
    const bytes = t => (typeof window.TextEncoder === "function" ? new window.TextEncoder().encode(t).length : t.length);
    return {
      list: () => Promise.resolve([...files].map(([id, f]) => ({ id, name: f.name, modified: f.modified, bytes: bytes(f.text) }))),
      load: id => sp0(() => get(id).text),
      save: (id, text) => sp0(() => { const f = get(id); f.text = String(text); f.modified = tick(); return { modified: f.modified }; }),
      create: name => sp0(() => { const id = "wb" + ++n; files.set(id, { name: String(name || ""), text: "", modified: tick() }); return { id }; }),
      rename: (id, name) => sp0(() => { get(id).name = String(name); }),
      duplicate: id => sp0(() => { const f = get(id), nid = "wb" + ++n; files.set(nid, { name: f.name + " 사본", text: f.text, modified: tick() }); return { id: nid }; }),
      trash: id => sp0(() => { get(id); files.delete(id); }),
    };
  }
  const sp0 = f => Promise.resolve().then(f);

  window.ItdaWhiteboard = {
    open, close, isOpen, toggle, unsaved, memoryStore,
    // debug 의 rec·dirty·latest·put·watch 는 지금 store 의 기록기를 들여다보는 손잡이다(시험용 — put 은 그 판의 기록기에 본문을 넣는다).
    debug: () => {
      if (!S) return null;
      const B = S.bag;
      return { engine: S.engine, state: S.state, root: S.root, save: S.save, recs: B.recs, thumbs: B.thumbs, prefs,
        rec: id => B.recs.get(id), dirty: id => dirty(B.recs.get(id)), latest: id => { const r = B.recs.get(id); return r ? latestText(r) : null; },
        put: (id, text) => put(recOf(B, id), text), watch: (id, n, ms) => watch(recOf(B, id), n, nowMs() + ms) };
    },
  };
})();
