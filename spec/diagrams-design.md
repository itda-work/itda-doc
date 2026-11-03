# 마크다운 그림 — 새 mermaid 종류 구현 가이드

` ```mermaid ` 에 새 그림 종류(④-2 상태도·④-3 클래스도·⑤ 이후)를 넣는 개발자·에이전트가 읽는 정본이다.
쓰는 사람용 문법은 [diagrams.md](diagrams.md)다. 이 문서는 「~한다(코드 위치)」 또는
「미검증」으로 쓴다 — 코드가 하지 않는 것을 보장으로 적지 않는다.

## 1. 구조 지도

```
mdoc ```mermaid 블록 ── mdoc/diagram.go  renderDiagram: diagram.Kind(src) 로 가른다
   ├─ gantt              → diagram/gantt
   ├─ timeline (EqualFold)→ diagram/timeline
   ├─ sequenceDiagram (EqualFold) → diagram/sequence
   ├─ erDiagram          → diagram/er
   ├─ stateDiagram…(HasPrefix) → diagram/state  (stateDiagram·stateDiagram-v2, 머리가 정확히 그 둘이 아니면 state 가 거부)
   ├─ classDiagram…(HasPrefix) → diagram/classdiag  (classDiagram·classDiagram-v2, 머리가 정확히 그 둘이 아니면 classdiag 가 거부)
   └─ 그 밖               → diagram/flowchart  (흐름도가 아닌 이름은 otherKinds 로 원문 + 경고)
종류 패키지: parse.go → (layout) → svg.go,  Render(src) (diagram.Result, error)
             오류 *diagram.Error{Line, Msg} → 원문 코드 블록 + 줄 번호 경고, 쓰지 않은 줄 diagram.Issue → 경고만
공통 diagram:  frontmatter·acc·종류 판별·상한·글자 폭·줄바꿈·SVG 이스케이프·엔티티 전처리·lexer 도구
공통 배치 diagram/layered:  계층 배치(Sugiyama) — 흐름도·ER·상태도·클래스도가 쓴다
```

- **종류 분기**: `mdoc/diagram.go` `renderDiagram`. 새 종류는 여기 `case` 한 줄과 import 를 더하고,
  `diagram/flowchart/parse.go` `otherKinds` 에서 그 머리 낱말을 지운다(남겨 두면 흐름도가 「다른 종류」로 원문 처리한다).
  머리 대소문자는 원본 detector 를 따른다 — `gantt`·`erDiagram` 은 구분, `timeline`·`sequenceDiagram` 은 `EqualFold`,
  `stateDiagram` 은 앞머리 `strings.HasPrefix`(원본 detector `/^\s*stateDiagram/` — 대소문자 구분), `classDiagram` 도 앞머리
  `strings.HasPrefix`(원본 classDetector-V2 `/^\s*classDiagram(-v2)?/` — 대소문자 구분. 패키지 이름은 `classdiag` — `classdiag.go` 머리 주석).
- **공개 꼴**(`diagram/diagram.go`): `Result{SVG, Caption, Issues}`, `Error{Line, Msg}`, `Issue{Line, Msg}`,
  상한 `MaxSource`(16KB)·`MaxLabel`(200). 종류 패키지는 이 꼴을 별칭으로 다시 내보낸다(예: `diagram/er/er.go`).
- **공통 함수**(`diagram`) — 복제하지 않고 부른다:

| 파일 | 함수 | 하는 일 |
|---|---|---|
| `parse.go` | `SplitFrontmatter`·`ParseFrontmatter`·`AccLine` | `---` YAML(글인 `title` 은 캡션, 나머지 키는 `Others`)·`accTitle:`/`accDescr` 꼴 |
| `diagram.go` | `Kind`·`Clip`·`Quote` | 머리 낱말(frontmatter·`%%`·빈 줄을 건너뜀)·경고에 넣을 조각 |
| `text.go` | `TextW`·`WrapLabel`·`LinesW`·`DecodeLabel` | 글자 폭(문서 글꼴 근사)·줄바꿈·`<br>`·엔티티 풀기 |
| `svg.go` | `Esc`·`XmlChars`·`F1` | SVG 이스케이프·XML 금지 글자 거르기·소수 한 자리 |
| `entity.go` | `EncodeEntities`·`DecodeEntities`·`HasMarker` | 원본 `encodeEntities` 재현(엔티티 `#…;` 를 내부 표식으로)·원문 표식 문자 검사 |
| `lex.go` | `OddSpace`·`ASCIILower`·`DirectiveProblem` | ASCII 밖 공백·서식 문자, ASCII 만 접는 소문자, 설정 지시 줄 판정 |

- **공통 배치**(`diagram/layered/layered.go`): 입력 `Graph{Dir, Nodes{W,H,Cluster}, Edges{From,To,Length,Label,Hidden},
  Clusters{Title}, NodeSep, RankSep, LoopOut, Clip}`, 출력 `Result{W, H, Nodes[]Box, Edges[]EdgePath{Pts, Self, LC, LW, LH},
  Clusters[]ClusterBox}`. 모양을 모른다 — 간선 끝 자르기는 `Clip` 콜백(없으면 `ClipRect`), 곡선은 `Basis`. 점(노드 + 더미)이
  `MaxPoints` 를 넘으면 `*diagram.Error`. 흐름도(`diagram/flowchart/layout.go` `layout`)·ER(`diagram/er/layout.go`)·상태도
  (`diagram/state/layout.go` — 복합 상태는 `Clusters`, 모양별 끝 자르기는 `Clip` 콜백 `clipShape`)가 쓴다.
  성질은 `diagram/layered/layered_test.go` 가 직접 본다(아래 4장).

## 2. 문법 정책 — 허용 목록 (③에서 정함)

차단 목록(막을 꼴을 하나씩 더하기)은 ③ 리뷰 1~3회차가 연이어 새 lexer 경계를 찾아 버렸다(2026-10-30). 새 종류는 처음부터
**허용 목록 문법**으로 짓는다(`diagram/sequence/parse.go`·`diagram/er/parse.go`·`diagram/state/parse.go` 머리 주석이 본보기):

- **줄마다 보수적인 양성 꼴 하나**에 통째로 맞는 줄만 그린다. 맞으면 원본과 같은 뜻, 아니면 그 줄 번호로 `*Error`.
  원본이 그리는 구석 꼴이 원문으로 가는 것은 의도된 축소다 — 안내서 「원본과 다르게 정한 점」에 적는다.
- **한 줄에 문장 하나** — `;` 로 이은 문장은 원문(원본에서 `;` 가 문장 구분인 종류).
- **지시자 대소문자는 ASCII 만 접는다**: 원본 jison `case-insensitive` 는 JS `/i`(u 깃발 없음)다. `diagram.ASCIILower` 로
  바꾼 줄에 소문자 정규식을 맞추고 값은 원래 줄의 같은 자리를 잘라 온다(`match` — sequence·er). Go `(?i)` 는 쓰지 않는다
  (`ſ`·`K`(U+212A) 가 s·k 로 접힌다).
- **ASCII 밖 공백·서식 문자가 든 줄은 원문**(`diagram.OddSpace` — 원본 JS `\s` 와 Go `unicode.IsSpace` 가 갈린다). 줄 중간의
  CR·FF·VT 도 원문(`diagram/er/parse.go`).
- **설정 지시**: 줄 전체가 닫힌 지시 하나(블록 밖)일 때만 무시하고 그린다. 그 밖에 `%%{` 가 든 줄은 원문 —
  `diagram.DirectiveProblem(t, 블록 안인가)` 를 줄 머리 주석 처리·엔티티 전처리·상태 분기보다 **앞**에서 부른다(일곱 종류 모두
  — er·sequence ④-1 리뷰 1회차, flowchart·gantt·timeline ④-1 후속, state ④-2 — 복합 상태 안도 「블록 안」, classdiag ④-3 — 클래스 몸 안도 「블록 안」).
- **줄 머리 `%%` 주석**: 원본 `cleanupComments`(`/^\s*%%(?!{)[^\n]+\n?/gm`)는 lexer 상태와 관계없이 먼저 지운다 — 이 판도 지시
  검사 바로 뒤, 상태 분기(`accDescr { … }` 블록·ER 속성 블록) 앞에서 지운다(블록 안의 `%% 메모` 는 설명에 들어가지 않는다).
  뒤에 글이 없는 맨 `%%` 줄은 그 정규식에 걸리지 않아 lexer 까지 간다 — 그래서 종류마다 원본 lexer 대로 가른다(④-1 후속 수정 2):

  | 종류 | 블록 밖 맨 `%%` | `accDescr { … }` 블록 안 맨 `%%` |
  |---|---|---|
  | sequence | 경고 없이 건너뜀 — 원본 주석 규칙 `\%%(?!\{)[^\n]*`(`sequenceDiagram.jison:28`) | 원문 + 경고 |
  | timeline | 경고 없이 건너뜀 — `\%%(?!\{)[^\n]*`(`timeline.jison:14`) | 원문 + 경고 |
  | gantt | 경고 없이 건너뜀 — `\%%[^\n]*`(`gantt.jison:34`) | 원문 + 경고 |
  | flowchart | 원문 + 경고 — `flow.jison` 에 주석 규칙이 없어 원본은 문법 오류 | 원문 + 경고 |
  | er | 원문 + 경고 — `erDiagram.jison` 에 주석 규칙이 없어 원본은 문법 오류 | 원문 + 경고(속성 블록 안도) |
  | state | 경고 없이 건너뜀 — `\%\%(?!\{)[^\n]*`(`stateDiagram.jison:84`, 복합 상태 안도) | 원문 + 경고 |
  | classdiag | 경고 없이 건너뜀 — `\%\%(?!\{)*[^\n]*(\r?\n?)+`(`classDiagram.jison:28`). 클래스 몸 안의 맨 `%%` 는 원본이 멤버(`jison:87` `[^{}\n]*`)로 읽어 원문 + 경고 | 원문 + 경고 |

  블록 안 맨 `%%` 는 원본 acc_descr_multiline 상태(`[^\}]*`)가 설명 글로 넣지만, 이 판은 보수적으로 그리지 않는다
  (sequence·timeline·gantt·state·classdiag 는 줄 머리 `%%` 처리의 `p.inAcc` 조건, flowchart·er 는 블록 안팎 같은 분기 — 각 `parse.go` `parse`).
- **지금 줄 루프의 차례**(새 종류는 er·sequence 꼴을 따른다):

| 종류 | 줄 루프 차례(코드 위치) |
|---|---|
| er | CR·FF·VT·`OddSpace` → `HasMarker` → `DirectiveProblem` → 줄 머리 `%%` → `EncodeEntities` → 상태 분기(acc·속성 블록·문장) — `diagram/er/parse.go` `parse` |
| sequence | `OddSpace` → `HasMarker` → `DirectiveProblem` → 줄 머리 `%%` → `EncodeEntities` → 상태 분기(acc·문장) — `diagram/sequence/parse.go` `parse` |
| state | CR·FF·VT·`OddSpace` → `HasMarker` → `DirectiveProblem`(acc·복합 상태 안은 블록 안) → 줄 머리 `%%` → `EncodeEntities` → 상태 분기(acc·문장 — 복합 상태 안팎은 `statement` 한 곳) — `diagram/state/parse.go` `parse` |
| classdiag | CR·FF·VT·`OddSpace` → `HasMarker` → `DirectiveProblem`(acc·클래스 몸 안은 블록 안) → 줄 머리 `%%` → `EncodeEntities` → 상태 분기(acc·문장 — 클래스 몸 안팎은 `statement` 한 곳, 몸 안은 `bodyLineMember`) — `diagram/classdiag/parse.go` `parse` |
| timeline | CR·FF·VT·`OddSpace` → `HasMarker` 꼴 검사 → `DirectiveProblem` → 줄 머리 `%%` → `encodeEntities` → 상태 분기 — `diagram/timeline/parse.go` `parse` |
| flowchart | CR·FF·VT·`OddSpace` → `HasMarker` → `DirectiveProblem` → 줄 머리 `%%` → 줄 전처리 경계(`StyleCut` — 쓰지 않는 문장만 있는 줄·접근성 줄 밖) → 상태 분기(라벨 글만 `EncodeEntities`) → 줄의 엔티티 토큰 수 = 라벨 안 토큰 수(`EntityTokens`) — `diagram/flowchart/parse.go` `parse` |
| gantt | CR·FF·VT·`OddSpace` → `HasMarker` → `DirectiveProblem` → 줄 머리 `%%` → 상태 분기(라벨 글만 `EncodeEntities` — 작업 줄은 `StyleCut`·`EntityTokens` 검사 뒤) — `diagram/gantt/parse.go` `parse`·`taskLine` |

- **엔티티 전처리 뒤에 남은 `#`** 는 원본이 주석·다른 토큰으로 읽는 자리면 원문(timeline 시기·sequence 줄 —
  `diagram.EncodeEntities` 로 지킨 엔티티는 글자).
- **원문에 이미 든 내부 표식 문자(`ﬂ°`·`¶ß`)는 원문**(`diagram.HasMarker` — 2026-10-30 결정).
- **무시하고 그리는 줄과 원문으로 돌리는 줄의 경계**: 모양을 문서에 적는 줄(`style`·`classDef`·`class`·`:::`·`linkStyle`·
  한 줄 설정 지시·frontmatter `config` 등·`click`)만 무시하고 그린다(종류마다 첫 줄에 `Issue` 하나 — `note(key, …)`).
  뜻을 바꾸는 줄(방향·구조·라벨)을 모르면 무시하지 않고 원문이다.
- **라벨**: 원본이 마크다운·HTML 로 그리는 글이면 그렇게 될 글자를 원문으로 돌린다(ER·state·classdiag `labelOK` — classdiag 는 멤버를 `member` 가 더 좁게 본다). 200자 상한.
- **엔티티·공백 정책 B — 일곱 종류 공통**(state ④-2 접근 방식 리뷰, 2026-10-31 결정. 흐름도·gantt·timeline·sequence·er
  는 ④-2 후속에서 옮겼다). 받은 라벨·설명·제목·접근성 값의 글자는 원본 전처리가 지킨 엔티티 표식만 한 번 되돌린 값이고, 그 밖의
  글자는 바꾸지 않는다(공통 `diagram/policy.go` `RestoreEntities` — 표식 → 검증된 글자 또는 오류. 표식 밖 글을 다시 찾지
  않고 되돌린 글자를 다시 읽지 않는다, `#35;35;` → `#35;`. state 의 `entity.go` `restore`·`numOK`·`unicodeVersion` 은 이 공통 구현의
  별칭이다).
  - 공통 함수 자리(`diagram/policy.go`): 엔티티 `RestoreEntities`·`EntityNumOK`·`EntityProblem`, 보이는 라벨 `Label`(여섯
    종류 — state 는 제 `label`)·한 줄 라벨 `OneLine`, 접근성 `AccValue`·`AccSet`·`AccBlock`, 캡션 `Caption`(일곱 종류), 속성
    `AriaRefs`, 출력 관문 `Gate`·`TextFunc`, XML 검사 `XMLProblem`. 종류마다의 시험은 공통 도구 `diagram/policytest`(손으로
    적은 엔티티 표·`encoding/xml` 로 SVG 읽기·`WrapOK`·`TextFields`)를 쓴다 — 기대값이 구현의 함수·표를 부르지 않는다.
  - 이름 7개(`quot`·`amp`·`lt`·`gt`·`apos`·`num`·`semi`, 대소문자 구분 — `entityNames`). `nbsp` 는 공백이라 뺀다.
  - 숫자는 십진 1\~7자리, U+0021\~U+10FFFF 의 유효 스칼라에서 Cc·Cf·White_Space·Noncharacter_Code_Point·서러게이트를 뺀다(`EntityNumOK` —
    Go `unicode` 표, 판본 `EntityUnicodeVersion` = 17.0.0. 판본이 바뀌면 state 의 `TestEntityPolicyTable` 이 멈춘다). 비문자 제외는 ④-2 1회차
    (공통 디코더 `entity` 의 숫자 규칙 — 비문자를 받는다)보다 더 줄인 것이다.
  - 표 밖의 엔티티(`#copy;`·`#nbsp;`·`#160;`·`#8195;`·`#32;`·`#x41;`·8자리)가 든 줄은 원문 + 줄 번호 경고 — 원본 오류가 아니라 미지원이다.
  - 보이는 라벨은 문법상 앞뒤 빈칸을 지운 뒤 안쪽 연속 빈칸·탭이 있으면 원문이다(`label`). 한 칸 빈칸만 남아(되돌린 글자에는
    `unicode.IsSpace` 글자가 없다 — White_Space 제외) `WrapLabel` 의 줄은 **보이는 글자까지** 값 그대로다(④-2 리뷰 5회차, 2026-10-31
    사용자 결정): 줄을 차례로 이으면 값이고, 줄 경계에서는 값의 빈칸 하나가 줄바꿈이 되거나(빠진다) 긴 낱말을 강제로 잘라 아무 글자도
    더하거나 빼지 않는다. 줄에는 앞뒤·연속 빈칸이 없다. **SVG 의 `<text>`·`<tspan>` 글을 이어 붙인 문자열이 원문과 같다고는 약속하지
    않는다**(줄바꿈된 빈칸은 SVG 글에 없다). 시험은 최종 SVG 를 `encoding/xml` 로 읽어 이 규칙을 손으로 적은 판정(`jWrapOK` —
    `WrapLabel` 을 부르지 않는다)에 건다(`TestWrapOutput` — 상태 설명 위·아래 칸, 복합 상태 제목, 전이 라벨).
  - `aria-label`(사용자 글자가 드는 유일한 속성 — 값은 `accTitle` → 캡션 → 종류 이름 「상태도」·「클래스도」 …)은 탭·LF·CR 을 `&#9;`·`&#10;`·`&#13;` 로
    쓴다(`svg.go` `ariaRefs`). 리터럴 탭은 XML 속성값 정규화(XML 1.0 3.3.3)로 빈칸이 되기 때문이다. 공통 `Esc` 는 바꾸지 않는다.
    캡션은 공통 `Caption` 이 탭을 원문으로 막고 줄바꿈을 빈칸으로 바꾸므로 실제로 닿는 것은
    `accTitle` 의 탭이다(`TestAriaLabelTab` — XML 로 읽은 값 = `AccTitle`).
  - 접근성 값은 원본처럼 앞뒤만 trim 하고 안쪽 빈칸·탭을 둔다(`accValue`·`accSet`). `accDescr { … }` 는 블록 전체 trim 뒤 원본
    `setAccDescription` 의 `replace(/\n\s+/g, '\n')` 를 옮긴다(`accBody`·`reNLSpace` — 줄바꿈·줄 끝 빈칸 보존, 줄 머리 빈칸·빈 줄 제거).
    표식 밖의 `#+65;` 는 글자 그대로, 복원한 `#lt;br#gt;` 는 `<br>` 글자일 뿐 줄바꿈으로 다시 해석하지 않는다. 맨 `&`·`<`·`>` 와 제어 문자(`AccSet` 이 탭·줄바꿈 밖의 `unicode.Cc` 를 검사한다 — C1 은 아래 관찰 표)는 원문 — 원본은 값을 `sanitizeText`(DOMPurify)에 넘겨 글이 바뀔
    수 있다(브라우저로 확인하지 않음 — 보수적).
  - **받은 글자 = 출력 글자 — 출력 관문 하나로 닫는다**(④-2 리뷰 3·4회차 — 같은 종류 발견이 두 회차라 문맥별 검사 대신 구조로).
    `Parse` 가 본문을 다 읽은 뒤 `chart.gate` 가 SVG·`Result` 로 나가는 사용자 글자를 모두 `diagram.XmlChars` 에 묻고, 바뀌는 값이
    있으면 그 값이 나온 줄의 원문 + 경고다(원문에 실제로 든 U+FFFE·U+FFFF, YAML 이스케이프 `"\uFFFE"` 로 든 캡션 등 XML 1.0 Char 밖
    글자). 범위는 필드 목록 한곳(`chart.eachText`): frontmatter title 캡션(`FMCaption` — title 줄 `fm.TitleLine`), `accTitle`·`accDescr`, 상태 이름·설명, 전이 라벨, 복합 상태 이름·제목. `TestGateCoversAllText`
    가 모델의 내보낸 string·[]string 필드(reflect)와 그 목록을 견주어 새 글 필드가 빠지면 멈춘다 — 이 시험은 목록의 덮음만 본다.
    출력 보존(최종 SVG 에 보이는 글자 그대로)은 SVG 를 XML 파서로 읽는 시험(`TestEntityPolicyTable`·`TestWrapOutput`·
    `TestAriaLabelTab`·`TestCaptionOutput`)이 본다. 라벨·acc 의 개별 `xmlOK` 는 그 줄에서
    먼저 같은 문구로 막는다(`TestGateMatchesFieldChecks`). 공통 `XmlChars` 는 묻기만 한다. **새 그림 종류도 이 관문 꼴(필드 목록 한곳 +
    빠짐 검사 + 본문 뒤 한 번)을 따른다.**
  - **다섯 종류(흐름도·gantt·timeline·sequence·er)에 옮긴 것**(④-2 후속). 보이는 라벨은 공통 `Label`(앞뒤 빈칸·탭을 지운 뒤
    안쪽 탭·연속 빈칸, `Label` 이 검사하는 제어 문자(탭 밖의 C0·DEL)는 원문 — C1 은 보장 밖이고 지금 동작은 아래 관찰 표, `<br>` 은
    줄바꿈 — er 는 `<br>` 을 줄바꿈으로 읽지 않는다), gantt 라벨과 흐름도 subgraph·
    sequence·timeline 의 제목은 `OneLine`(한 줄 — `<br>` 은 빈칸 하나). 접근성 값은 state 와 같은 `AccValue`·`AccBlock`·`AccSet`(원본
    `commonDb` 의 `setAccTitle`·`setAccDescription` 과 각 jison 의 `$2.trim()` 이 일곱 종류에 같다). 출력 관문은 종류마다 `eachText` +
    `diagram.Gate`(`parse.go`)이고, 종류마다 `TestGateCoversAllText`(내보내지 않은 필드까지 reflect — gantt 의 `dateFmt.fmt`·
    `datePart.tok` 는 날짜를 읽는 데만 써 뺀다)가 덮음을 본다. `aria-label` 의 탭·LF·CR 은 `AriaRefs` 가 문자 참조로 쓴다(각
    `svg.go`).
  - 엔티티 표식을 지키는 자리 — er·sequence·timeline 은 원본처럼 줄 전체를 `EncodeEntities` 로 지킨 뒤 `Label`·`AccSet` 이 한 번
    되돌린다. 흐름도·gantt 는 라벨 글에만 `EncodeEntities` 를 씌운다(갈래 (b)): 원본 lexer 가 라벨 글 상태에서는 `#`·`;` 를 글자로
    읽고(flow.jison text·STR·EDGE_TEXT `[^\[\]\(\)\{\}\|\"]+`, gantt.jison `taskTxt` `[^:\n]+`·`title`·`section` `[^\n]+`) 설정
    줄(`dateFormat`·`axisFormat` `[^#\n;]+`, flow 의 INITIAL 상태 `#`·`;`)에서만 갈라 읽으므로, 줄 단위로 바꾸면 이 판의 설정 줄
    읽기가 바뀐다. **라벨 글만 전처리한 값이 원본(줄 전체 전처리)과 같은 것은 두 조건에서뿐이다**(④-2 후속 리뷰 1회차):
    ① 줄에 원본 전처리의 `style.*:\S*#.*;`·`classDef.*:\S*#.*;` 가 맞지 않는다 — 맞으면 원본은 **줄의** 마지막 `;` 를 지우는데
    라벨 전처리는 라벨 안의 `;` 를 지운다(`A["style x:#35;"];` → 원본 라벨 `style x:#`, 라벨 전처리 `style x:#35`; `A[style] -->|x:#35;| B`
    는 반대). 그런 줄은 원문이다(`diagram.StyleCut`). 쓰지 않는 문장(`style`·`classDef`·`class`·`linkStyle`·`click`·`direction`)만
    있는 줄은 끝 `;` 를 잘라도 그리지 않는 그대로라 받고, 접근성 줄(gantt 는 `title`·`section` 도)은 라벨이 줄 끝까지라 두 전처리가 같다.
    ② 줄의 엔티티 토큰(`#\w+;`)이 모두 라벨 글 안에 통째로 있다 — 라벨은 줄의 부분 문자열이고 `#`·`;` 는 `\w` 가 아니어서 라벨 안의
    토큰은 줄의 토큰이기도 하므로, 수가 같으면 같은 토큰이다(`diagram.EntityTokens`, 흐름도 `labTok`·gantt `taskLine`). 라벨 밖·
    경계에 걸친 토큰은 원본이 표식으로 지켜 문법의 뜻이 갈린다 — 원문. 문법이 받아 주는 반례: 흐름도 `subgraph s#35;`(검사를 끄면
    id 를 그대로 쓴 제목 `s#35` 로 그린다), gantt `작업 : t#35;, 2026-01-01, 1d`(콜론 뒤 작업 id). 두 검사 모두 `TestPreprocessBoundary`
    의 뮤턴트가 잡는다. gantt 의 ①은 콜론 뒤에 `#\w+;` 가 아닌 `#…;` 가 올 때만 ② 와 따로 걸린다(`style :a#;, …`). 원문 줄의 표식 문자(ﬂ°·¶ß)는 다섯 종류 모두 줄째 원문이다(`HasMarker`).
  - **특수 공백은 줄째 원문 + 공통 함수의 심층 방어**(리뷰 1회차 [2]·[3]): 다섯 종류 모두 줄 루프 첫머리에서 `OddSpace`(NBSP·EM
    SPACE·U+200B 따위)와 줄 중간 CR·FF·VT 를 막는다. 받으면 긴 라벨의 `WrapLabel`(`strings.Fields`)이 NBSP 를 빈칸으로 바꾸고,
    `accDescr { … }` 의 줄 머리 정리(ASCII `reNLSpace`)가 원본 JS `\n\s+` 와 갈린다. 공통 `Label`·`OneLine`·`AccValue`·`AccSet`·
    `AccBlock.Add`·`Caption` 도 호출자 전제에 기대지 않고 같은 범위(`OddSpace`)를 스스로 거부한다(`oddProblem`). 공백이 아닌 서식
    문자(U+00AD 따위)는 Go·JS 어느 쪽도 공백으로 보지 않아 바뀌지 않으므로 받는다(state 가 받던 그대로 — state 는 줄에서 이미 막아
    이 방어에 닿지 않는다).
  - 캡션(frontmatter title)은 일곱 종류가 공통 `Caption`(`fm.Raw` — YAML 이 읽은 글)으로 읽는다. 원본 `Diagram.fromText` 가
    frontmatter 를 `encodeEntities` 전에 떼어 내므로 title 의 `#…;` 는 표식이 아니라 **글자 그대로**다. 앞뒤 빈칸·줄바꿈을 지운 뒤
    안쪽 탭·연속 빈칸과 `Caption` 이 검사하는 제어 문자(탭·줄바꿈 밖의 C0·DEL — C1 은 아래 관찰 표)는 title 줄(`fm.TitleLine`)의 원문.
    `Caption` 이 받는 것은 YAML 이 읽은 값(`fm.Raw`)이라 YAML 의 이스케이프·줄 접기가 먼저다. `<br>`·줄바꿈은 빈칸 하나, XML 이 담지 못하는 글자는
    관문이 title 줄로 알린다. state 도 같다(2026-10-31 사용자 결정 「여섯 종류 통일」 — 옛 `fm.Title`(`DecodeLabel`)·frontmatter 시작 줄 1 에서
    옮겼다. `diagram/state/parse.go` frontmatter 처리, `TestCaptionPolicy`·`TestCaptionOutput`).
  - 공통 `DecodeLabel`·`namedEntities` 는 지우지 않았다 — `ParseFrontmatter` 의 title 길이 검사(`fm.Title`)만 쓴다. `WrapLabel` 은 바꾸지 않았다.
  - **C1 제어 문자(U+0080~U+009F) — 현재 동작, 지원 약속 아님**(2026-10-31 결정 「보장에서 제외」, ④-2 후속 접근 방식
    리뷰). C1 이 든 입력의 수락 여부와 표시 형태는 호환성 보장 대상이 아니다(안내서 1장). 아래는 구현을 지금 이 경로로 지나는
    결과의 관찰 기록이고, 각 행은 `policytest.C1Policy` 의 같은 이름 갈래가 일곱 종류에서 C1 32개로 회귀 감지한다(classdiag 는 라벨·수량 글 문맥만 — 멤버는 C1 을 받지 않고 `TestC1Policy` 가 따로 본다)(행 이름은
    `diagram` `TestC1ObserveTable` 이 대조한다). 결과가 바뀌면 의도한 변경인지 검토하고 이 표를 고친다. U+0085(NEL)는
    C1 이면서 공백이라 따로 둔다(시험의 기대도 손으로 나눈다). 캡션 행은 YAML 원문 → YAML 이 읽은 값 → `Caption` 의 차례다.

<!-- c1-observe:begin -->
| 행 | 입력 경로 | 지금 결과 | 갈리는 곳 |
|---|---|---|---|
| `C1-라벨` | 보이는 라벨 안의 실제 글자(U+0085 밖 31개) | 값 그대로 그림에 나간다 | `Label`·state `labelOK` 의 제어 문자 검사는 C0·DEL 만 본다(`policy.go`, `state/parse.go`) |
| `C1-라벨-NEL` | 보이는 라벨 안의 실제 U+0085 | 그 줄 원문 | 줄 루프·공통 함수의 `OddSpace`(`lex.go`) |
| `C1-캡션-이스케이프` | title 큰따옴표 글의 YAML 이스케이프 `\x80` 꼴(U+0085 밖) | YAML 이 읽은 값에 남아 `Caption`·출력 관문을 지나 캡션 그대로 | `ParseFrontmatter` → `Caption` |
| `C1-캡션-이스케이프-NEL` | title 큰따옴표 글의 `\x85` | title 줄 원문 | `Caption` 의 `oddProblem` |
| `C1-캡션-날글자` | title 큰따옴표 글 안의 실제 글자(U+0085 밖) | YAML 오류 — frontmatter 째 원문(1번 줄) | `ParseFrontmatter` 의 YAML 파서 |
| `C1-캡션-날글자-NEL` | title 큰따옴표 글 안의 실제 U+0085 | YAML 이 줄바꿈으로 접어 캡션 「x y」 | YAML 파서 → `Caption`(줄바꿈은 빈칸) |
| `C1-접근성` | 접근성 값 안의 실제 글자(U+0085 밖) | 값의 줄(`accDescr { … }` 는 닫는 줄) 원문 | `AccSet` 의 `unicode.Cc` 검사 |
| `C1-접근성-NEL` | 접근성 값 안의 실제 U+0085 | 그 글자가 든 줄 원문 | 줄 루프 `OddSpace`·`AccBlock.Add` |
<!-- c1-observe:end -->

- **원본 DB 의 첫 값·누적 규칙**은 받는 꼴의 뜻을 바꾼다 — 같은 문장이 두 번 나올 때 원본이 무엇을 남기는지(상태도: 특수 상태 모양은
  처음 만들 때만, 설명은 쌓임, 묶음 소속은 지워지지 않음 — `diagram/state/parse.go` `mention`·`special`·`declare`. 클래스도: 주석은 첫 것만 그림·없는 클래스에 `<<주석>> 이름` 은 오류·
  `style`·`cssClass`·`click` 은 있는 클래스에만 — `diagram/classdiag/parse.go` `annotate`·`statement`) 원본 소스로 확인하고,
  뜻이 갈리는 조합은 원문으로 돌린다.
- 안내서 장에는 지원·무시·미지원·**원본과 다르게 정한 점**·상한 표를 둔다. 표는 시험이 실행한다(4장).

## 3. 원본 대조 순서

1. **태그 고정**: 지금 `mermaid@12.2.0`. 의뢰서·보고서·코드 주석에 같은 태그를 적는다.
2. **전처리**(`packages/mermaid/src/preprocess.ts` `preprocessDiagram` → `Diagram.ts` `fromText`): CRLF → LF·HTML 속성 따옴표
   바꾸기(`cleanupText`) → frontmatter → 지시 읽기와 `removeDirectives`(`utils.ts` — 줄 어디서든 지시만 지움) →
   `cleanupComments`(`diagram-api/comments.ts` — 줄 머리 `%%` 주석 지우기) → `encodeEntities`(`utils.ts`, `fromText` 안).
   이 판은 줄마다 지시 → 주석 → (엔티티) → 상태 분기 차례로 판정한다. 엔티티 전처리는 er·sequence·timeline 만 줄 단위로
   하고 flowchart·gantt 는 하지 않는다(2장 「지금 줄 루프의 차례」 표).
3. **detector**(`diagrams/<종류>/*Detector.ts`): 머리 정규식과 대소문자.
4. **jison lexer**(`diagrams/<종류>/parser/*.jison`): 상태(`%x`)·규칙 차례(첫 일치)·`%options case-insensitive`·낱말 규칙 뒤
   `\b`(JS — ASCII 낱말 글자만)·`.*` 로 줄 전체를 먹는 규칙. ③·④-1 에서 나온 함정:
   - 키워드 **앞머리**가 이름 토큰보다 먼저 맞는다 — ER `to`·`one`·`many`·`end`·`class`·`style` … 로 시작하는 이름
     (`Token`·`endpoint`·`Classroom`)을 이 판은 막는다(`keywordPrefixes`, 원본이 낱말 경계로 구하는지는 미검증).
   - `.*direction\s+(TB|BT|RL|LR)` 처럼 **줄 어디서든** 맞는 규칙 — ER 은 `direction` 이 든 줄을 방향 한 줄만 받는다.
   - 문법에 없는 지시자가 **이름이 되는** 것 — ER `title` 은 엔티티가 된다(이 판은 원문).
   - **블록 안 상태의 키 우선** — ER 속성 블록의 `\b(PK|FK|UK)\b` 가 ATTRIBUTE_WORD 보다 먼저.
   - sequence: 지시자 + 비낱말 글자(`end가`)가 `\b` 로 지시자가 됨, 메시지 끝 `-` 뒤 `x` 가 가위표 토큰, `accDescr { … }` 의
     닫는 `}` 뒤 글이 다음 문장, autonumber 수 뒤 `(?=[ \n]+)`(탭이면 원문).
5. **DB**(`*Db.ts`): 값의 뜻(처음 나온 차례·마지막이 이김·별칭 채우기·누적 차례).
6. **렌더러**: 라벨을 마크다운으로 그리는지(`createText` markdown)·배치·끝 기호.

원본 소스는 `curl` 대신 `gh api` 로 받아 scratchpad 에 둔다:

```
gh api 'repos/mermaid-js/mermaid/contents/packages/mermaid/src/diagrams/er/parser/erDiagram.jison?ref=mermaid@12.2.0' \
  -H 'Accept: application/vnd.github.raw'
gh api 'repos/mermaid-js/mermaid/contents/packages/mermaid/src/diagrams/er?ref=mermaid@12.2.0' --jq '.[].name'   # 목록
```

## 4. 시험 틀 (필수)

| 무엇 | 본보기 |
|---|---|
| 파서 단위 — 문장마다 값, 오류 줄 번호·문구, 무시 줄 경고 | `diagram/er/er_test.go` `TestParseStatements`·`TestIgnoredLines` |
| 상한 경계(바로 밑은 그림, 넘으면 원문)·결정성(두 번 같은 SVG) | `TestLimits`·`TestDeterminism` |
| **독립 판정기** — 허용 꼴을 시험 안에 따로 옮긴 손 스캐너(구현의 정규식·상수·도우미 미사용), 생성기는 경계 토큰을 명시적으로 조합(상태 전이·경계 글자 끼우기·함정 이름), 수락 여부·거부 줄·값을 `Render` 결과와 대조, 판수와 받음/거부 비율 하한 | `diagram/er/judge_test.go`·`diagram/state/judge_test.go`·`diagram/classdiag/judge_test.go`·`diagram/sequence/sequence_test.go` `TestJudgeBoundaryProperties` |
| 지시 경계 고정 사례 — 종류마다 지시 사이 문장·acc 값·라벨·주석·블록 안 | 각 패키지 `TestDirectiveLines` |
| 배치 속성 — viewBox 안·겹침 없음·그리기 순서·끝 기호 자리, 네 방향, 무작위 + 극단 | `diagram/er/er_test.go` `TestLayoutProperties` |
| 공통 배치를 쓰면 — 성질은 `diagram/layered` 가 지킨다. 새 종류는 자기 감싸개(크기·Clip·끝 기호)만 본다 | `diagram/layered/layered_test.go` |
| 뮤턴트 — 거부 뒤집기·값 훼손·배치를 `go test -overlay` 로 넣어 새 시험이 잡는지(저장소 파일을 바꿨다 되돌리지 않는다) | ④-1 에서 한 번 했다 |
| 안내서 표 실행 — 지원·무시·미지원 표의 표기를 읽어 실제로 그림/경고/원문인지, 필수 표기 목록 | `mdoc/diagramguide_test.go` `TestERGuide` |
| mdoc golden(`-update` 로만 갱신)·보안(SVG 요소·속성 허용 목록, 공격 사례)·퍼즈 시드·클래스 접두사 | `mdoc/golden_test.go`·`security_test.go`·`TestClassCollisions` |
| 앱 견본·PDF 글자 | 그림을 쓰는 앱 저장소의 몫 |
| 테마 스냅숏 | 사람이 직접 본다 |

**판정기는 구현과 같은 실수를 공유하지 않게 쓴다** — ④-1 2회차는 판정기가 acc 블록 여는 줄 본문을 다시 넣어 전처리를 두 번 한
것을 찾았다. 판정기 안의 전처리·상태 전이도 원본 차례를 따로 따라간다.

## 5. SVG·CSS 규칙

- 클래스는 종류 접두사(`gt-`·`tl-`·`sq-`·`er-`·`st-`·`cls-` — 흐름도는 접두사 없는 옛 클래스 그대로. 클래스도는 흐름도의 `cl-box`·`cl-title`
  과 겹치지 않게 `cls-`)와 `diagram-svg`. `TestClassCollisions` 로 본다.
- 요소는 허용 목록(`mdoc/security_test.go` `svgElements`: svg·g·rect·path·line·circle·text·tspan·title)만. 화살촉·끝
  기호는 `marker` 없이 `path`(·`circle`)로 그린다(고정 id 가 한 문서의 여러 그림에서 겹친다 — 1.10 흐름도).
- 색·선은 클래스 → 기존 `--diagram-*`·`--chart-*`·`--paper` 토큰(`web/base.css`). 새 토큰은 꼭 필요할 때만,
  기본값은 기존 토큰 참조, [themes.md](themes.md) 토큰 표에 적는다.
- 선이 글자를 가로지르지 않게 라벨 바탕 `rect` 와 그리기 순서(선 → 바탕 → 글자)를 둔다(ER `TestLayoutProperties` 가 순서를
  본다). 진한 면 위 글자는 따로 글자색을 준다(`base.css` 의 `.tl-sec-t`·`.tl-per-t`).
- 글자는 문서 글꼴의 `<text>` — PDF 에서 선택·검색된다. 숫자는 `diagram.F1`(소수 한 자리), 곱셈-덧셈은 명시적 `float64(…)`
  변환으로 FMA 합치기를 막는다(흐름도 `TestCorpusHash` 가 `GOARCH=amd64` 와 같은 출력을 본다).
- 그림 자리는 SVG 안이 아니라 감싸는 `figure.diagram` 의 `data-diagram-*` 속성이 운반한다(`mdoc/diagramsize.go`
  가 값 검사·경고 — 크기 속성은 따옴표 없는 낱말만 받는다: `diagramWords` 가 낱말 경계에서 `ratio=`·`width=`·`height=` 로
  시작하는 낱말을 다음 공백까지 통째로 검사하고(따옴표가 든 값은 검사에 걸려 경고 후 무시), 그 밖의 낱말은 따옴표 구간을
  닫는 따옴표까지(없으면 줄 끝까지) 건너뛰니 다른 속성의 따옴표 값 안 `width=…` 는 크기가 아니다. `codetitle.go` 의
  title= 파일명 띠와는 나누기·계약을 섞지 않는다). 화면(`pager.js` `sizeDiagrams`)이 다시 검사해 SVG
  요소의 `width`·`height`·`aspect-ratio` 를 CSSOM 으로 건다 — CSP `style-src 'self'` 로 style 속성은 못 쓴다(column `data-w`
  선례). 너비 백분율은 CSS `%` 가 풀리는 대로 **그림이 놓인 자리**(figure — 최상위면 본문, 목록·인용·다단 안이면 그 칸) 기준이고,
  높이 백분율은 쪽 나눔기가 쪽마다 거는 `--body-h`(본문 높이) 기준이다. 상자 높이 상한은 높이가 정해지는 모든 조합에
  `본문 높이 − 60px`(`--img-max-h` 와 같은 캡션 자리)를 `calc()` 로 계산해 건다. 캡션이 그 60px 자리를 넘어 figure 가
  한 쪽에 못 들어가면 `pager.js` `forceFit` 이 SVG 의 `max-height` 를 줄여 그림과 캡션을 한 쪽에 통째로 넣는다(캡션이 한 쪽보다
  길면 줄여도 못 맞춰 실패 — 캡션이 다음 쪽으로 간다. 축소 뒤 상자의 `aspect-ratio` 는 깨지지만 그림은 contain 으로 비율을 지킨다).
  contain 은 SVG `viewBox` 의 기본 `preserveAspectRatio`(xMidYMid meet)가 한다 — 배치 좌표는 건드리지 않는다.

## 6. 상한과 계산량

- 원문 16KB·라벨 200자(공통, `diagram.MaxSource`·`MaxLabel`) + 종류별 요소 수(예: ER 엔티티 100·관계 300). 상한은 계산 전에 본다.
- 파서는 원문을 한 번 훑는다(선형). 배치를 쓰면 `layered.MaxPoints`(12000) — 넘으면 원문 + 경고.
- 같은 입력은 같은 출력: map 순회로 차례를 정하지 않는다, 오늘 날짜·난수·로캘·환경을 읽지 않는다.

## 7. 문서·정본 갱신 목록

1. 안내서 [diagrams.md](diagrams.md) 새 장 — 지원·무시·미지원·원본과 다르게·상한 표(표지 주석
   `<!-- 종류-…:begin -->`), 머리 문단의 종류 목록. `just syntax` 로 `mdsyntax/topics/diagrams.md` 재생성.
2. 그림을 쓰는 앱의 기능 목록·설계 문서 — 아래 「문서 역할」대로 한 줄 소개와 안내서 링크만(그 앱 저장소의 몫).
3. mdoc golden 의 `diagram` 견본(`mdoc/testdata/diagram.md`). 앱의 견본·PDF 검사는 그 앱 저장소의 몫.
4. 새 토큰이면 테마 안내서. 이 문서 9장 현황 표.

**문서 역할**(접근 방식 리뷰, 2026-11-01 — ER·상태도·클래스도부터 적용, 흐름도·gantt·timeline·sequence 의 지난 요약은 따로 정리):

- 사용자 문법 경계(지원·무시·미지원·원본과 다르게 정한 점·상한)의 정본은 안내서 한 곳이다. `md_syntax`(`mdsyntax/topics/diagrams.md`)는 그 생성본이라 손으로 고치지 않는다(`just syntax`, `TestBundleFresh`).
- 앱의 기능 목록·설계 문서와 이 문서 9장은 지원 경계와 원본 동작을 다시 쓰지 않는다 — 한 줄 소개, 설계 고유 정보(패키지·배치·토큰·계산량·판정기·검증 범위와 미검증), 안내서 링크만 둔다. 파서 구조·판정 함수 위치·설계 이유는 10장에 종류마다 한 번 쓴다.
- 원본 동작 단정은 확인한 근거 수준까지만 쓴다. ① 버전(mermaid@12.2.0)·완전한 입력·관찰한 출력이나 오류·관찰 단계·근거 위치(파일·함수)를 댈 수 있을 때. ② 원본 함수를 Node 로 돌렸으면 「그 함수가 이 문자열을 돌려준다」까지 — 설정 파싱·전체 문법·화면 렌더 성공으로 넓히지 않는다. ③ 「원본은 그린다」는 그 입력의 렌더 근거가 있을 때만, 소스·함수 대조뿐이면 「원본 소스에서 이렇게 읽는다(소스 판독, 화면 미검증)」로. ④ 실측한 한 꼴을 같은 기호가 든 모든 입력으로 넓히지 않는다. ⑤ 원본 근거가 없어도 「이 판은 이 꼴을 받지 않는다」로 미지원은 안내할 수 있다.

## 8. 작업 방식

이 저장소의 일하는 규칙은 [`AGENTS.md`](../AGENTS.md)다. 그림 종류를 구현할 때 함께 지킬 것:

- 저장소 안에 임시 파일(`.tmp` 포함)을 두지 않는다.
- 뮤테이션·실험 시험은 `go test -overlay` 로만.
- 원본 소스는 `curl` 대신 `gh api … -H 'Accept: application/vnd.github.raw'`.
- 보고서·PR 본문에 자리표시자를 두지 않는다 — 검증 명령은 실제 결과를 적는다.

## 9. 종류별 현황

| 종류 | 패키지 | 안내서 | 배치 | 독립 판정기 | 알려진 미검증 |
|---|---|---|---|---|---|
| 흐름도 | `diagram/flowchart` | 1~7장 | `diagram/layered` | 없음(무작위 그림 속성·말뭉치 해시·`FuzzRender`) | 엔티티·공백은 정책 B(2장 — 라벨 글에만 표식, `policy_test.go` `TestEntityPolicyTable`·`TestWrapOutput`·`TestGateCoversAllText`·`TestAriaLabelTab`). 원본 브라우저에서 접근성 값의 HTML 정제 결과·연속 빈칸의 화면 배치는 미검증(그 꼴은 원문) |
| gantt | `diagram/gantt` | 8장 | 자체(`svg.go` 시간 축) | 없음 | 엔티티·공백은 정책 B(2장 — 라벨 글에만 표식, `policy_test.go`). `axisFormat` 값에 `#`·`;` 가 있으면(엔티티 꼴 `#35;` 포함) 그 줄의 원문 + 경고다(`parse.go` `axisFormat` 갈래, `TestAxisFormatHashSemicolon` — 2026-10-31 사용자 결정 「그 줄은 원문」). 원본은 줄 단위 전처리로 `#…;` 를 표식으로 지킨 뒤 축 글에서 되돌리고, 맨 `#`·`;` 에서는 `[^#\n;]+` 가 멈춘다 — 이 판은 그 갈래를 구현하지 않는다. 원본 브라우저의 HTML 정제·연속 빈칸 배치는 미검증(그 꼴은 원문) |
| timeline | `diagram/timeline` | 9장 | 자체(`svg.go`) | 부분 — 시기 `#` 경계만(`TestHashBoundaryProperties`) | CR·U+2028·U+2029 같은 특이한 줄바꿈 글자에서 엔티티 전처리가 원본과 같은지(안내서 9장). 엔티티·공백은 정책 B(2장, `policy_test.go`, `TestHashBoundaryProperties` 가 연속 빈칸·미지원 엔티티도 손 판정). 원본 브라우저의 HTML 정제·연속 빈칸 배치는 미검증(그 꼴은 원문) |
| sequence | `diagram/sequence` | 10장 | 자체(`svg.go` 생명선·메시지 행) | 있음(`TestJudgeBoundaryProperties`) | 브라우저 고유의 비표준 색 키워드(안내서 10장). 엔티티·공백은 정책 B(2장, `policy_test.go`, 판정기 `jDecode`·`jAcc`). 원본 브라우저의 HTML 정제·연속 빈칸 배치는 미검증(그 꼴은 원문) |
| er | `diagram/er` | 11장 | `diagram/layered` | 있음(`judge_test.go` `TestJudgeBoundaryProperties`) | 문법 경계·원본 대조는 안내서 11장, 파서 구조는 10.1. 미검증: 키워드로 시작하는 이름을 원본이 낱말 경계로 구하는지. 원본 브라우저의 HTML 정제·연속 빈칸 배치. 정책 B 시험은 `policy_test.go`, 판정기 `jDecode`·`jLabel`·`jAcc` |
| state | `diagram/state` | 12장 | `diagram/layered`(복합 상태 = `Clusters`) | 있음(`judge_test.go` `TestJudgeBoundaryProperties`) | 문법 경계·원본 대조는 안내서 12장, 파서 구조는 10.2. 미검증: 빈 복합 상태를 원본이 어떤 모양으로 그리는지. 원본 브라우저에서 접근성 값의 HTML 정제 결과·연속 빈칸의 화면 배치. 정책 B 시험은 `entity.go`·`judge_test.go` `TestEntityPolicyTable`. `aria-label` 의 탭은 `&#9;` 로 써 XML 로 읽어도 탭이다(2장) |
| classdiag | `diagram/classdiag` | 13장 | `diagram/layered`(수량 글은 계층 사이 띠 — `layout.go` `placeCards`) | 있음(`judge_test.go` `TestJudgeBoundaryProperties` — 제네릭은 `jPGT`·`jGenSeq` 로 따로 유도) | 문법 경계·원본 대조는 안내서 13장, 파서 구조는 10.3. 미검증: 멤버·제네릭 표시 글은 원본 소스(`ClassMember.parseMember`·`getDisplayDetails`·`common.ts` `parseGenericTypes`)를 옮긴 것이고 원본 브라우저 화면과 대조하지 않았다. 옮긴 `parseGenericTypes` 는 구현 때 원본 소스를 Node 로 돌린 값과 무작위 20000건을 한 번 대조했다(일회성 — 저장소 시험에는 없다). 원본 브라우저의 HTML 정제·연속 빈칸 배치. 정책 B 시험은 `policy_test.go` |

## 10. 파서 구조 메모 — ER·상태도·클래스도

사용자 문법 경계는 안내서 11·12·13장이 정본이다(7장 「문서 역할」). 여기는 구현을 읽는 사람을 위한 파서 구조·판정 함수
위치·설계 이유만 둔다. 「원본」 근거는 mermaid@12.2.0 소스 판독이며 원본 화면은 실측하지 않았다(9장 미검증 칸). 세 종류 모두
허용 목록 문법이다 — 줄마다 양성 꼴 하나에 통째로 맞는 줄만 그리고, 아니면 원문 + 줄 번호 경고.

### 10.1 ER (`diagram/er`)

- 줄 꼴 `reRel`·`reEnt`·`reAttr`·`reDir`·`reAcc*`·`reStyle`·`reClass`, 판정 `parse.go` `statement`·`blockLine`. 지시자는 ASCII 만 접어 맞춘다(`diagram.ASCIILower` — 순서도에서 공통으로 올렸다, `diagram.OddSpace` 도).
- 이름 앞머리 `keywordPrefixes`: 원본 lexer(`erDiagram.jison`)는 규칙 차례의 첫 일치라 UNICODE_TEXT 앞의 키워드 규칙이 이름 앞머리를 먹을 수 있다 — 이 판은 낱말 경계를 따지지 않고 그 앞머리로 시작하는 이름을 모두 막는다(보수적). 줄 어디서든 줄 전체를 먹는 `.*direction\s+(TB|BT|RL|LR)` 규칙 때문에 `direction` 이 든 줄은 방향 한 줄 꼴만 받는다. 블록 안 타입·이름은 `wordOK`(`\b(PK|FK|UK)\b` 가 ATTRIBUTE_WORD 보다 앞).
- 라벨(별칭·역할·설명)은 `labelOK` — 원본이 `createText` markdown 으로 그리므로 마크다운·HTML 이 될 글자를 막는다.
- 값 쌓기: 엔티티 차례·별칭(`addEntity` — 비어 있을 때만 채운다), 속성은 블록을 닫을 때 원문 차례로 붙인다(원본 attributes 재귀 + 거꾸로 넣기와 같은 차례), 관계 왼기호는 A 쪽 끝(원본 relSpec `cardB:$1` = arrowTypeStart).

### 10.2 상태도 (`diagram/state`)

- 줄 꼴 `reTrans`·`reDecl`·`reSpecial`·`reAsState`·`reComp`·`reDir`·`reAcc*`·`reStyle`·`reClassDef`·`reClass`·`reClick`, 판정 `parse.go` `statement`·`unknown`. 지시자는 `diagram.ASCIILower` + `match`.
- 이름 앞머리 `keywordPrefixes`: 원본 ID 규칙(`stateDiagram.jison` `[^:\n\s\-\{]+`) 앞의 키워드 규칙이 앞머리를 먹을 수 있다. lex-parser 가 낱말 글자로 끝나는 규칙에 `\b` 를 붙이는 것은 확인했지만 이 판은 앞머리로 막는다(보수적). 상태 없는 lexer 규칙은 INITIAL 에서만 쓰이므로 `click`·`hide empty description`·`accTitle`·`accDescr` 는 뿌리에서만 받는다.
- 라벨 `labelOK` — `:` 는 DESCR 토큰과 같은 값으로 읽히는 꼴만. 엔티티 되돌리기 `entity.go` `restore`(정책 B, 2장). 접근성 값 `accSet`·`accBody`.
- 값 쌓기(원본 `stateDb.ts`·`dataFetcher.ts` 소스 판독): `[*]` 는 범위 + `_start`·`_end` 상태(원본 docTranslator), 상태는 처음 나온 차례(nodeDb), 설명은 쌓인다, 복합 상태 소속은 지워지지 않는다(Object.assign), 특수 상태 모양은 처음 만들 때만 정해진다, 방향은 뿌리의 마지막 `direction`(getDir). 원본과 뜻이 갈리는 조합은 `mention`·`special`·`declare`·`openComp`·`closeComp` 에서 원문으로 돌린다.

### 10.3 클래스도 (`diagram/classdiag`)

- 줄 꼴 `reDir`·`reAcc*`·`reClassDef`·`reStyle`·`reCSSClass`·`reClickHref`·`reClickCall`·`reLink`·`reClass`·`reClassLine`·`reAnnot`·`reRel`·`reMember`, 판정 `parse.go` `statement`·`unknown`. 원본 클래스도 lexer 는 대소문자를 접지 않아 지시자도 원문 그대로 맞춘다.
- 이름 `nameOK`: 원본 ALPHA·UNICODE_TEXT 토큰(`unicode.go` — jison 의 글자 표) 이음만 받고, 토큰 머리에서 키워드 규칙이 낱말 경계로 먼저 맞는 이름은 막는다(lex-parser 의 `\b` 덧붙임).
- 멤버 `member`: 원본 `addMember`·`ClassMember.parseMember`·`getDisplayDetails` 를 옮겨 표시 글을 만들고, 원본이 다른 글로 그리는 꼴과 마크다운·HTML 이 될 글자는 막는다. 엔티티 표식은 자리표(U+E000)로 바꿔 글자 검사를 지난 뒤 되돌린다.
- 제네릭 `generic.go`: 이름 자리는 `gnamePat` → `className`(인자 검사 `genericArg`, 클래스 정체는 `~` 앞 이름, 인자 `classNode.Type` 은 처음 만든 자리의 것 — 원본 `splitClassNameAndType`·`addClass`), 상자 제목 `Title()`. 멤버는 이름·인자·반환형마다 `genericText` 가 `~` 짝을 괄호처럼 읽은 값과 원본 `parseGenericTypes` 를 옮긴 값(`parseGenericTypes`)을 비교해 같을 때만 `<…>` 로 바꾼다 — 원본은 쉼표 조각마다 맨 앞·맨 뒤 `~` 를 짝짓기 때문에 두 값이 갈리는 꼴이 있다. 받는 꼴과 받지 않는 꼴의 예는 안내서 13장 멤버 표(`x~T~, y~U~` 는 그림, 한 쉼표 조각 안의 `a~T~ b~U~` 는 원문).
- 관계 라벨·수량 글 `labelOK`. 값 쌓기: 클래스는 처음 나온 차례(`addClass` Map), 주석은 클래스마다 하나, 관계 왼끝 모양·수량 글은 왼 클래스 쪽(원본 `type1`·`relationTitle1`).
