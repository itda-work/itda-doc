# itda-doc 에이전트 지침

공개 저장소다. 여기 쓰는 모든 글·코드·커밋은 누구나 읽는다고 보고 쓴다.

## 언어
- 응답·코드 주석·커밋 메시지는 한국어로 쓴다. 식별자·명령·기술 용어는 원래 꼴 그대로 둔다.

## 문법 — 새 기호를 만들지 않는다
- 새 문법이 필요하면 이 차례로 이미 있는 것에서 찾는다: CommonMark → GFM → Pandoc 부분집합 → 컨테이너(`:::이름`)·frontmatter 키 어휘.
  앞 단계에 있는 것을 두고 뒤 단계로 가지 않고, 어디에도 없는 기호는 만들지 않는다.
- 문법의 정본은 `spec/` 이다. 코드와 표는 시험이 묶는다(`mdoc/*guide*_test.go`·`page/guidethemes_test.go`·`mdsyntax`).
  `spec/*.md` 의 `<!-- syntax:주제:begin/end -->` 구간을 고치면 `just syntax` 로 `mdsyntax/topics`·`completions.json` 을 다시 만든다.

## 명세 변경 절차(itda-work/itda-hyve#577 §3-4)
- 어휘 추가(새 컨테이너·키·값)는 minor 판으로 낸다. 옛 판은 모르는 키로 두고 그린다.
- 이미 있는 키의 뜻은 바꾸지 않는다. 다른 뜻이 필요하면 새 키를 만든다.
- 폐기는 한 판 동안 경고를 낸 뒤에 한다.

## 시험
- `just check`(gofmt·vet)와 `just test`(`ITDA_DOC_STRICT=1 go test ./...`)가 통과한다(CI `.github/workflows/ci.yml`).
  `ITDA_DOC_STRICT=1` 이면 node(web 의 쪽 나눔 시험)나 CommonMark 예제(`spec.json`)가 없을 때 건너뛰지 않고 실패한다. 값이 없으면 건너뛴다.
- 뮤테이션 시험은 원본 파일을 고치지 않고 `go test -overlay <json>` 으로 바꿔 끼운 소스를 얹어 돌린다. 이때 `ITDA_DOC_TEST_OVERLAY=<json>` 도 함께 주면 소스를 읽는 시험(`mdoc/guidemd_test.go`)이 디스크 대신 바꿔 끼운 파일을 읽는다.
- 외부 네트워크가 필요한 시험은 넣지 않는다.

## 금지선
- 비밀값·토큰·개인 정보·내부 주소를 파일·커밋·이슈에 적지 않는다.
- 코드가 하지 않는 것을 보장처럼 적지 않는다 — 「~한다(코드 위치)」 또는 「미검증」 으로 쓴다.
