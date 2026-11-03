// Package gantt 는 mermaid 간트 차트(gantt)의 부분집합을 SVG 로 그린다(itda-hyve#409 ①, 설계 1.10).
//
// 문법은 실제 Mermaid 의 부분집합이다 — 새 문법을 만들지 않는다(안내서 spec/diagrams.md 의 표가 정본).
// 모르는 문장·값은 그리지 않고 *Error(줄 번호)를 돌려준다 — 부르는 쪽(mdoc)이 원문 코드 블록을 그대로 두고 경고한다.
// 모양을 문서에 적는 줄(todayMarker·topAxis·weekday·click·%%{init}%%·frontmatter 설정)은 쓰지 않고 그림은 그린다(Issue 경고).
//
// 배치는 시간 축(아래) + 작업 행이다: 왼쪽에 구역 이름 열, 시간 영역에 구역마다 번갈이 띠·제외 날짜의 옅은 세로 띠·눈금 격자선,
// 작업은 둥근 사각형 막대(이름은 폭이 되면 막대 안, 아니면 오른쪽 — Mermaid 와 같게), milestone 은 마름모(작업 시작·끝의 가운데).
// 오늘 선(todayMarker)은 그리지 않는다 — PDF 는 기록이고 오늘 날짜에 의존하지 않는다(itda-hyve#409 공통 규칙 4).
//
// 결과 SVG 는 허용 목록의 요소·속성만 쓰고(흐름도·차트와 같은 목록) 색·선은 클래스 → 테마 토큰(--diagram-*),
// 글자는 문서 글꼴의 <text> 라 PDF 에서 선택·검색된다. 같은 입력은 같은 출력이다(map 순회 없음, 숫자는 소수 한 자리,
// 시각은 UTC 로만 다룬다 — 로캘·시간대·오늘 날짜 의존 없음).
package gantt

import (
	"fmt"
	"time"

	"github.com/itda-work/itda-doc/diagram"
)

// 상한. 넘으면 그리지 않고 *Error 다(원문 + 경고). 원문·라벨 상한은 공통(diagram)이다.
const (
	MaxSource   = diagram.MaxSource // 그림 하나의 원문(바이트)
	MaxLabel    = diagram.MaxLabel  // 라벨 하나의 글자 수(룬)
	MaxTasks    = 150               // 작업
	MaxSections = 30                // 구역
	MaxYears    = 100               // 날짜 범위(년)
	MaxTicks    = 400               // 눈금 — 자동 눈금은 사다리의 다음 칸으로, tickInterval 이 넘으면 경고하고 자동 눈금
)

// Error·Issue·Result 는 그림 종류가 함께 쓰는 공개 꼴이다(diagram — flowchart 와 같다).
type (
	Error  = diagram.Error
	Issue  = diagram.Issue
	Result = diagram.Result
)

// task 는 작업 하나다. End 는 제외 날짜를 밀어낸 뒤의 값이다(Mermaid 와 같게).
type task struct {
	Name      string
	ID        string
	Done      bool
	Active    bool
	Crit      bool
	Milestone bool
	Start     time.Time
	End       time.Time
	Section   int // 들어 있는 구역(chart.Sections 의 차례), 없으면 -1
	Line      int
}

// section 은 구역(띠) 하나다.
type section struct {
	Name  string
	First int // 첫 작업(chart.Tasks 의 차례)
	Line  int
}

// chart 는 읽은 그림이다.
type chart struct {
	Title     string // title 줄 — 그림 안 위쪽 제목(Mermaid 와 같게)
	FMCaption string // frontmatter title — 그림 캡션
	AccTitle  string // accTitle — 그림의 접근성 이름
	AccDescr  string // accDescr — 그림 설명
	DF        *dateFmt
	AF        *axisFmt
	Tick      rung // tickInterval(HasTick 일 때)
	HasTick   bool
	TickLine  int // tickInterval 줄 — 눈금을 바꿔 그릴 때 알리는 줄
	AxisLine  int // axisFormat 줄(없으면 머리 줄) — 축 라벨이 어느 칸에서도 겹쳐 그리지 못할 때 알리는 줄
	Excl      *excludes
	Tasks     []*task
	Sections  []*section
	ticks     []time.Time // drawSVG 가 그린 눈금(격자선 = 라벨) — 시험이 독립 계산으로 정렬을 본다

	// 글 필드가 나온 줄(출력 관문의 줄 번호). FMLine 은 frontmatter 의 title 줄이다.
	FMLine, TitleLine, AccTitleLine, AccDescrLine int
}

// Render 는 mermaid gantt 원문 하나(코드 블록 안)를 그린다. 그리지 못하면 *Error 다.
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
