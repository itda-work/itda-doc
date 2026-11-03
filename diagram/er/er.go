// Package er 는 mermaid ER 다이어그램(erDiagram)의 부분집합을 SVG 로 그린다(itda-hyve#409 ④-1, 설계 1.10).
//
// 문법은 실제 Mermaid(mermaid@12.2.0 — diagrams/er/parser/erDiagram.jison·erDb.ts)의 부분집합이다 — 새 문법을 만들지
// 않는다(안내서 spec/diagrams.md 11장 표가 정본). 줄마다 허용 꼴(양성 목록) 하나에 통째로 맞는 줄만
// 그리고, 어느 꼴에도 맞지 않으면 *Error(줄 번호)를 돌려준다 — 부르는 쪽(mdoc)이 원문 코드 블록을 그대로 두고 경고한다
// (itda-hyve#208 결정 「허용 목록 문법」, sequence ③ 과 같은 틀). 모양을 문서에 적는 것(%%{init: …}%%·frontmatter 의 title 밖 키·
// style·classDef·class·:::)은 쓰지 않고 그림은 그린다(Issue 경고).
//
// 배치는 흐름도와 같은 계층 배치(diagram/layered)다: 엔티티 상자가 노드, 관계가 간선(역할 글은 가운데 더미). 엔티티 상자는
// 원본 erBox 의 꼴 — 머리 칸(이름 또는 별칭) + 속성 표(열: 타입 · 이름 · 키 · 설명, 키·설명이 하나도 없으면 그 열을 접는다).
// 관계 선 양끝의 기수 기호(까마귀발)는 marker 없이 path·circle 로 선이 상자에 닿는 자리에 선 방향으로 돌려 그린다.
//
// 결과 SVG 는 허용 목록의 요소·속성만 쓰고(흐름도·간트·타임라인·순서도와 같은 목록) 글자는 문서 글꼴의 <text> 라 PDF 에서
// 선택·검색된다. 같은 입력은 같은 출력이다(map 순회 없음, 숫자는 소수 한 자리).
package er

import (
	"fmt"

	"github.com/itda-work/itda-doc/diagram"
)

// 상한. 넘으면 그리지 않고 *Error 다(원문 + 경고). 원문·라벨 상한은 공통(diagram)이고 배치 점 수는 공통 배치의
// 상한(layered.MaxPoints)이다.
const (
	MaxSource        = diagram.MaxSource // 그림 하나의 원문(바이트)
	MaxLabel         = diagram.MaxLabel  // 라벨 하나의 글자 수(룬)
	MaxEntities      = 100               // 엔티티
	MaxRelations     = 300               // 관계
	MaxAttrsPerEnt   = 50                // 엔티티 하나의 속성
	MaxAttrsTotal    = 1000              // 속성 합계
	maxClassNameList = 20                // `:::`·class 문장의 이름 수(쓰지 않는 줄이라 짧게 묶는다)
)

// Error·Issue·Result 는 그림 종류가 함께 쓰는 공개 꼴이다(diagram).
type (
	Error  = diagram.Error
	Issue  = diagram.Issue
	Result = diagram.Result
)

// Card 는 관계 한쪽 끝의 기수다(원본 erDb Cardinality — MD_PARENT 는 이 판이 받지 않는다).
type Card int

const (
	ZeroOrOne  Card = iota // |o · o|
	ExactlyOne             // ||
	ZeroOrMore             // }o · o{
	OneOrMore              // }| · |{
)

var cardName = [...]string{"ZERO_OR_ONE", "ONLY_ONE", "ZERO_OR_MORE", "ONE_OR_MORE"}

func (c Card) String() string { return cardName[c] }

// Dir 는 그리는 방향이다(direction TB·BT·LR·RL — 원본 기본 TB).
type Dir int

const (
	TB Dir = iota
	BT
	LR
	RL
)

// attribute 는 속성 한 줄이다(원본 Attribute — 키는 적은 차례, 설명은 따옴표를 뺀 글).
type attribute struct {
	Type    string
	Name    string
	Keys    []string
	Comment string
	Line    int
}

// entity 는 엔티티 하나다(원본 EntityNode — 처음 나온 차례, 별칭은 처음 적은 것이 이긴다).
type entity struct {
	ID    string // 원문 이름(참조에 쓴다)
	Name  string // 상자 머리에 적는 이름(ID 를 푼 글)
	Alias string // 별칭(없으면 "") — 있으면 머리에 별칭을 적는다(원본 erBox)
	Attrs []attribute
	Line  int
	// AliasLine 은 Alias 를 정한 줄이다(뒤 문장이 비어 있던 별칭을 채운다 — 출력 관문의 줄 번호).
	AliasLine int
}

// relation 은 관계 하나다. Left 는 왼기호(엔티티 A 쪽 끝에 그린다), Right 는 오른기호(B 쪽 끝) — 원본 relSpec 의
// cardB·cardA 와 렌더의 arrowTypeStart = cardB(시작 = A)·arrowTypeEnd = cardA(끝 = B).
type relation struct {
	A, B        int // chart.Entities 의 차례
	Left, Right Card
	Identifying bool   // -- 실선(식별), .. 점선(비식별)
	Role        string // 역할 글(없으면 "")
	Line        int
}

// chart 는 읽은 그림이다.
type chart struct {
	Dir       Dir
	FMCaption string // frontmatter title — 그림 캡션
	AccTitle  string
	AccDescr  string
	Entities  []*entity
	Relations []relation
	byID      map[string]int

	// 글 필드가 나온 줄(출력 관문의 줄 번호). FMLine 은 frontmatter 의 title 줄이다.
	FMLine, AccTitleLine, AccDescrLine int
}

// Render 는 mermaid erDiagram 원문 하나(코드 블록 안)를 그린다. 그리지 못하면 *Error 다.
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
