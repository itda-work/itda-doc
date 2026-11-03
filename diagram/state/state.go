// Package state 는 mermaid 상태도(stateDiagram·stateDiagram-v2)의 부분집합을 SVG 로 그린다(itda-hyve#409 ④-2, 설계 1.10).
//
// 문법은 실제 Mermaid(mermaid@12.2.0 — diagrams/state/parser/stateDiagram.jison·stateDb.ts·dataFetcher.ts)의 부분집합이다
// — 새 문법을 만들지 않는다(안내서 spec/diagrams.md 12장 표가 정본). 줄마다 허용 꼴(양성 목록) 하나에 통째로
// 맞는 줄만 그리고, 어느 꼴에도 맞지 않으면 *Error(줄 번호)를 돌려준다 — 부르는 쪽(mdoc)이 원문 코드 블록을 그대로 두고
// 경고한다(itda-hyve#208 결정 「허용 목록 문법」, sequence ③·er ④-1 과 같은 틀). 모양을 문서에 적는 것(%%{init: …}%%·frontmatter 의
// title 밖 키·style·classDef·class·:::·click·hide empty description)은 쓰지 않고 그림은 그린다(Issue 경고).
//
// 두 머리(stateDiagram·stateDiagram-v2)는 원본에서 같은 렌더러(stateRenderer-v3-unified — stateDetector-V2 가 둘 다
// 맡는다)로 같게 그려진다. 배치는 흐름도·ER 과 같은 계층 배치(diagram/layered)다: 상태가 노드, 전이가 간선, 복합 상태가
// 묶음(한 단계). 모양은 원본 v3 렌더의 꼴 — 상태는 둥근 사각형(설명이 둘 이상이면 머리 + 구분선 + 설명 줄, rectWithTitle),
// 시작은 채운 작은 원, 끝은 이중 원, 선택은 마름모, 갈래·합류는 채운 막대(가로 그림에서는 세로), 복합 상태는 제목 칸이 있는
// 묶음 상자다. 전이는 실선 + 끝 화살촉(marker 없이 path)이다.
//
// 결과 SVG 는 허용 목록의 요소·속성만 쓰고(흐름도·간트·타임라인·순서도·ER 과 같은 목록) 글자는 문서 글꼴의 <text> 라 PDF 에서
// 선택·검색된다. 같은 입력은 같은 출력이다(map 순회 없음, 숫자는 소수 한 자리).
package state

import (
	"fmt"

	"github.com/itda-work/itda-doc/diagram"
)

// 상한. 넘으면 그리지 않고 *Error 다(원문 + 경고). 원문·라벨 상한은 공통(diagram)이고 배치 점 수는 공통 배치의
// 상한(layered.MaxPoints)이다. 상태 수에는 시작·끝·선택·갈래·합류 상태가 든다.
const (
	MaxSource        = diagram.MaxSource // 그림 하나의 원문(바이트)
	MaxLabel         = diagram.MaxLabel  // 라벨 하나의 글자 수(룬)
	MaxStates        = 200               // 상태
	MaxTransitions   = 500               // 전이
	MaxComposites    = 50                // 복합 상태
	MaxDescrs        = 20                // 상태 하나의 설명 줄
	maxClassNameList = 20                // class·style 문장의 이름 수(쓰지 않는 줄이라 짧게 묶는다)
)

// Error·Issue·Result 는 그림 종류가 함께 쓰는 공개 꼴이다(diagram).
type (
	Error  = diagram.Error
	Issue  = diagram.Issue
	Result = diagram.Result
)

// Kind 는 상태의 종류다(원본 dataFetcher 의 shape — start·end 는 [*], 나머지는 state … <<…>>).
type Kind int

const (
	Normal Kind = iota
	Start
	End
	Choice
	Fork
	Join
)

var kindName = [...]string{"state", "start", "end", "choice", "fork", "join"}

func (k Kind) String() string { return kindName[k] }

// Dir 는 그리는 방향이다(direction TB·BT·LR·RL — 원본 기본 TB).
type Dir int

const (
	TB Dir = iota
	BT
	LR
	RL
)

// node 는 상태 하나다 — 처음 나온 차례(원본 dataFetcher 가 문서 차례로 nodes 에 넣는다). 시작·끝의 ID 는 원본이 만드는 id
// (`root_start`·`복합_end` — stateDb docTranslator)다. Descr 는 원본 description 배열(설명 문장마다 하나씩 쌓인다): 비면 ID 를,
// 하나면 그 설명을 이름 자리에, 둘 이상이면 첫 설명을 머리로 나머지를 아래 줄로 그린다(rectWithTitle).
type node struct {
	ID      string
	Kind    Kind
	Descr   []string
	Cluster int // 들어 있는 복합 상태(chart.Comps 의 차례), 없으면 -1
	Line    int
	// DescrLines 는 Descr 마다 그 설명이 나온 줄이다(출력 관문 gate 의 줄 번호).
	DescrLines []int
}

// transition 은 전이 하나다(원문 차례).
type transition struct {
	From, To int // chart.States 의 차례
	Label    string
	Line     int
}

// composite 는 복합 상태 하나다(여는 차례). Title 은 묶음 머리 글(ID 를 푼 글 또는 state "설명" as ID { 의 설명).
type composite struct {
	ID    string
	Title string
	Line  int
	n     int // 구성원 수
}

// chart 는 읽은 그림이다.
type chart struct {
	Dir       Dir
	FMCaption string // frontmatter title — 그림 캡션
	AccTitle  string
	AccDescr  string
	States    []*node
	Trans     []transition
	Comps     []*composite
	byID      map[string]int
	compByID  map[string]int

	// 글 필드가 나온 줄(출력 관문 gate 의 줄 번호). FMLine 은 frontmatter 의 title 줄이다(다섯 종류와 같다).
	FMLine, AccTitleLine, AccDescrLine int
}

// Render 는 mermaid stateDiagram 원문 하나(코드 블록 안)를 그린다. 그리지 못하면 *Error 다.
func Render(src string) (Result, error) {
	if len(src) > MaxSource {
		return Result{}, &Error{Msg: fmt.Sprintf("그림 원문이 %dKB 를 넘음", MaxSource>>10)}
	}
	c, issues, err := Parse(src)
	if err != nil {
		return Result{}, err
	}
	l, err := layout(c)
	if err != nil {
		return Result{}, err
	}
	return Result{SVG: drawSVG(c, l), Caption: diagram.XmlChars(c.FMCaption), Issues: issues}, nil
}
