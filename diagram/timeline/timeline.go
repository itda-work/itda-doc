// Package timeline 은 mermaid 타임라인(timeline)의 부분집합을 SVG 로 그린다(#409 ②, 설계 1.10).
//
// 문법은 실제 Mermaid 의 부분집합이다 — 새 문법을 만들지 않는다(안내서 spec/diagrams.md 의 표가 정본).
// 모르는 문장·값은 그리지 않고 *Error(줄 번호)를 돌려준다 — 부르는 쪽(mdoc)이 원문 코드 블록을 그대로 두고 경고한다.
// 원본에 문법이 없는 것(classDef·style·click·dateFormat 같은 다른 종류의 지시자)도 알 수 없는 문장이다(timeline 의
// jison 문법은 title·section·시기·사건·접근성만 안다). 모양을 문서에 적는 줄(%%{init: …}%%·frontmatter 의 title 밖 키)은
// 쓰지 않고 그림은 그린다(Issue 경고).
//
// 배치는 가로 시기 열이다: 구역 머리 상자(그 구역 시기들의 폭만큼 걸침) → 시기 상자 한 줄 → 가로 시간축 선(오른쪽 화살촉) →
// 시기마다 세로 점선에 매달린 사건 상자(원본 Mermaid renderer 와 같은 뼈대 — 원본은 활동선을 맨 아래에 그리지만 이 판은
// 시기와 사건 사이에 둔다). 상자 폭은 고정이고 긴 글은 줄을 바꿔 상자 높이가 는다(시기 행의 상자끼리는 높이를 맞춘다).
// 색은 차트 팔레트 --chart-1…8 순환(구역마다, 구역이 없으면 시기마다 — 원본 cScale 순환과 같은 뜻), 진한 면 위 글자는
// --chart-on, 사건 상자는 같은 색을 반투명으로(classes → base.css, 원본 eventWrapper 의 밝게 와 같은 구분).
//
// 결과 SVG 는 허용 목록의 요소·속성만 쓰고(흐름도·간트와 같은 목록 — 화살촉은 marker 가 아니라 삼각형 path) 글자는
// 문서 글꼴의 <text> 라 PDF 에서 선택·검색된다. 같은 입력은 같은 출력이다(map 순회 없음, 숫자는 소수 한 자리).
package timeline

import (
	"fmt"

	"github.com/itda-work/itda-doc/diagram"
)

// 상한. 넘으면 그리지 않고 *Error 다(원문 + 경고). 원문·라벨 상한은 공통(internal/diagram)이다.
const (
	MaxSource          = diagram.MaxSource // 그림 하나의 원문(바이트)
	MaxLabel           = diagram.MaxLabel  // 라벨 하나의 글자 수(룬)
	MaxPeriods         = 60                // 시기
	MaxEvents          = 300               // 사건 합계
	MaxEventsPerPeriod = 20                // 시기 하나의 사건
	MaxSections        = 20                // 구역
)

// Error·Issue·Result 는 그림 종류가 함께 쓰는 공개 꼴이다(internal/diagram — flowchart·gantt 와 같다).
type (
	Error  = diagram.Error
	Issue  = diagram.Issue
	Result = diagram.Result
)

// period 는 시기 하나다(원본 timelineDb 의 task — 시기 이름과 그에 딸린 사건들).
type period struct {
	Name    string
	Events  []string
	Section int // 들어 있는 구역(chart.Sections 의 차례), 없으면 -1
	Line    int
	// EventLines 는 Events 마다 그 사건이 나온 줄이다(이어 쓰기 줄 포함 — 출력 관문의 줄 번호).
	EventLines []int
}

// section 은 구역 하나다(뒤따르는 시기를 묶는 머리 상자).
type section struct {
	Name  string
	First int // 첫 시기(chart.Periods 의 차례)
	Line  int
}

// tl 은 읽은 그림이다.
type tl struct {
	Title     string // title 줄 — 그림 안 위쪽 제목(Mermaid 와 같게)
	FMCaption string // frontmatter title — 그림 캡션
	AccTitle  string // accTitle — 그림의 접근성 이름
	AccDescr  string // accDescr — 그림 설명
	Periods   []*period
	Sections  []*section

	// 글 필드가 나온 줄(출력 관문의 줄 번호). FMLine 은 frontmatter 의 title 줄이다.
	FMLine, TitleLine, AccTitleLine, AccDescrLine int
}

// Render 는 mermaid timeline 원문 하나(코드 블록 안)를 그린다. 그리지 못하면 *Error 다.
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
