// Package sequence 는 mermaid 순서도(sequenceDiagram)의 부분집합을 SVG 로 그린다(itda-hyve#409 ③, 설계 1.10).
//
// 문법은 실제 Mermaid(mermaid@12.2.0 — parser/sequenceDiagram.jison 이 대소문자를 가리지 않는다)의 부분집합이다 —
// 새 문법을 만들지 않는다(안내서 spec/diagrams.md 10장 표가 정본). 모르는 문장·값은 그리지 않고
// *Error(줄 번호)를 돌려준다 — 부르는 쪽(mdoc)이 원문 코드 블록을 그대로 두고 경고한다. 모양을 문서에 적는 것
// (%%{init: …}%%·frontmatter 의 title 밖 키·rect/box 의 색·링크 메뉴)은 쓰지 않고 그림은 그린다(Issue 경고).
//
// 배치는 원본 렌더러(sequenceRenderer.ts)의 뼈대를 테마 토큰으로 옮긴 것이다: 맨 위 참여자 상자 한 줄(사람 actor 는
// 사람 모양), 아래로 생명선(점선), 맨 아래 참여자 상자 되풀이(원본 기본 mirrorActors=true 를 확인했다). 메시지는
// 위→아래 순서의 가로 화살표 + 위 글, 참여자 사이 간격은 그 사이를 지나는 메시지·메모 글 폭으로 넓힌다(원본
// calculateActorMargins 꼴 — 이웃 참여자 사이의 것만, 원본과 같다). 묶음(loop·alt·…)은 걸친 참여자 범위의 테두리 +
// 왼쪽 위 머리 칸 + 구획 점선이고 중첩은 안쪽으로 여백이 늘어난다(원본 updateBounds 의 깊이별 여백과 같다).
//
// 결과 SVG 는 허용 목록의 요소·속성만 쓰고(흐름도·간트·타임라인과 같은 목록 — 화살촉은 marker 가 아니라 path) 글자는
// 문서 글꼴의 <text> 라 PDF 에서 선택·검색된다. 같은 입력은 같은 출력이다(map 순회 없음, 숫자는 소수 한 자리).
package sequence

import (
	"fmt"

	"github.com/itda-work/itda-doc/diagram"
)

// 상한. 넘으면 그리지 않고 *Error 다(원문 + 경고). 원문·라벨 상한은 공통(diagram)이다.
const (
	MaxSource = diagram.MaxSource // 그림 하나의 원문(바이트)
	MaxLabel  = diagram.MaxLabel  // 라벨 하나의 글자 수(룬)
	MaxActors = 50                // 참여자
	MaxEvents = 500               // 메시지·메모 합계
	MaxBlocks = 100               // 묶음(loop·alt·opt·par·critical·break·rect)
	MaxDepth  = 10                // 묶음 깊이
	MaxBoxes  = 20                // 참여자 무리(box)

	MaxNumDigits = 6 // autonumber 수·번호의 표시 자릿수(넘으면 원문 + 경고 — 리뷰 1회차 [8])
)

// Error·Issue·Result 는 그림 종류가 함께 쓰는 공개 꼴이다(diagram — flowchart·gantt·timeline 과 같다).
type (
	Error  = diagram.Error
	Issue  = diagram.Issue
	Result = diagram.Result
)

// arrowKind 는 메시지 화살표의 끝 모양이다(원본 고전 10종 — 실선/점선 × 끝 모양).
type arrowKind struct {
	Dotted bool // 점선
	Head   byte // 'n' 없음(-> · -->), 'f' 채운 화살촉(->> · -->>), 'x' 가위표, 'o' 열린 화살촉(-) · --))
	Both   bool // 양쪽 화살촉(<<->> · <<-->>)
}

// participant 는 참여자 하나다(원본 sequenceDb 의 actor).
type participant struct {
	ID    string // 원문 식별자(참조에 쓰는 이름)
	Name  string // 상자에 적는 이름(alias 없으면 ID)
	Actor bool   // 사람 모양(actor 선언)
	Box   int    // 들어 있는 box(chart.Boxes 의 차례), 없으면 -1
	Line  int
	// NameLine 은 Name 을 정한 줄이다(별명 있는 다시 적기가 바꾼다 — 출력 관문의 줄 번호).
	NameLine int
}

// box 는 참여자 무리 하나다(box 이름 … end).
type box struct {
	Name  string
	Line  int
	First int // 첫 참여자(chart.Actors 의 차례)
	Count int
}

// placement 는 메모의 자리다(0 = 아직 정해지지 않음).
const (
	leftOf  = 1
	rightOf = 2
	over    = 3
)

// 묶음 종류(loop·opt·alt·par·critical·break·rect).
const (
	bNone = iota
	bLoop
	bOpt
	bAlt
	bPar
	bCritical
	bBreak
	bRect
)

var blockName = map[int]string{bLoop: "loop", bOpt: "opt", bAlt: "alt", bPar: "par", bCritical: "critical", bBreak: "break", bRect: "rect"}

// evKind 는 사건 하나의 종류이다(원본 messages 배열의 type — 메시지·메모·활성·번호·묶음 경계가 줄을 잇는다).
type evKind int

const (
	evMessage     evKind = iota // 참여자 → 참여자 화살표(+ 활성 표식)
	evNote                      // 메모(left of·right of·over)
	evActiveStart               // activate · 메시지 끝 +
	evActiveEnd                 // deactivate · 메시지 끝 -
	evAutonumber                // autonumber [시작 [단계]] · autonumber off
	evBlockStart                // loop·opt·alt·par·critical·break·rect …
	evBlockElse                 // else·and·option(구획 나눔)
	evBlockEnd                  // end
)

// event 는 원문의 사건 하나다(메시지·메모 합계 상한은 evMessage·evNote 로 센다).
type event struct {
	kind evKind
	line int

	from, to string // 메시지·활성·메모의 참여자
	ar       arrowKind
	text     string // 메시지·메모 글·묶음 머리 글
	activate int    // 메시지 끝 활성 표식(+1/-1)

	placement byte   // 메모 자리(leftOf·rightOf·over)
	over2     string // Note over A,B 의 둘째 참여자(없으면 "")

	block   int  // 묶음 종류(bLoop …)
	section bool // evBlockElse — else·and·option

	numStart, numStep   float64 // autonumber 시작·단계(적은 것만)
	haveStart, haveStep bool    // 시작·단계를 원문에 적었는가(안 적은 `autonumber` 는 세던 수를 이어 간다 — 원본과 같다)
	numOff              bool    // autonumber off
}

// chart 는 읽은 그림이다.
type chart struct {
	Title     string // title 줄 — 그림 안 위쪽 제목(Mermaid 와 같게)
	FMCaption string // frontmatter title — 그림 캡션
	AccTitle  string // accTitle — 그림의 접근성 이름
	AccDescr  string // accDescr — 그림 설명
	Actors    []*participant
	Boxes     []*box
	Events    []event
	byID      map[string]int // 참여자 ID → Actors 차례

	// 글 필드가 나온 줄(출력 관문의 줄 번호). FMLine 은 frontmatter 의 title 줄이다.
	FMLine, TitleLine, AccTitleLine, AccDescrLine int
}

// Render 는 mermaid sequenceDiagram 원문 하나(코드 블록 안)를 그린다. 그리지 못하면 *Error 다.
func Render(src string) (Result, error) {
	if len(src) > MaxSource {
		return Result{}, &Error{Msg: fmt.Sprintf("그림 원문이 %dKB 를 넘음", MaxSource>>10)}
	}
	c, issues, err := Parse(src)
	if err != nil {
		return Result{}, err
	}
	svg, issues2, err := drawSVG(c)
	if err != nil {
		return Result{}, err
	}
	return Result{SVG: svg, Caption: diagram.XmlChars(c.FMCaption), Issues: append(issues, issues2...)}, nil
}
