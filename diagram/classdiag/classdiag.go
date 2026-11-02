// Package classdiag 는 mermaid 클래스도(classDiagram·classDiagram-v2)의 부분집합을 SVG 로 그린다(#409 ④-3, 설계 1.10).
//
// 문법은 실제 Mermaid(mermaid@12.2.0 — diagrams/class/parser/classDiagram.jison·classDb.ts·classTypes.ts)의 부분집합이다
// — 새 문법을 만들지 않는다(안내서 spec/diagrams.md 13장 표가 정본). 줄마다 허용 꼴(양성 목록) 하나에 통째로
// 맞는 줄만 그리고, 어느 꼴에도 맞지 않으면 *Error(줄 번호)를 돌려준다 — 부르는 쪽(mdoc)이 원문 코드 블록을 그대로 두고
// 경고한다(#208 결정 「허용 목록 문법」, sequence ③·er ④-1·state ④-2 와 같은 틀). 모양을 문서에 적는 것(%%{init: …}%%·
// frontmatter 의 title 밖 키·classDef·cssClass·style·:::·click·link·callback)은 쓰지 않고 그림은 그린다(Issue 경고).
//
// 패키지 이름은 class 가 아니라 classdiag 다 — class 는 Go 예약어가 아니어서 쓸 수는 있지만, CSS 클래스를 다루는 코드에서
// 흔한 지역 이름이라 import 한 패키지를 가리기 쉽다(mdoc 의 state 를 statediagram 으로 고쳐 부르는 것과 같은 까닭).
//
// 두 머리(classDiagram·classDiagram-v2)는 원본에서 같은 렌더러(classRenderer-v3-unified — classDetector-V2 가 둘 다 맡는다)로
// 같게 그려진다. 배치는 흐름도·ER·상태도와 같은 계층 배치(internal/layered)다: 클래스가 노드, 관계가 간선. 모양은 원본 v3
// 렌더(classBox·shapeUtil·markers)의 꼴 — 클래스는 이름 칸(주석 «이름» + 굵은 이름)·속성 칸·메서드 칸을 쌓은 사각형(빈 칸도
// 그린다 — 원본 hideEmptyMembersBox 기본값 거짓), 정적 멤버는 밑줄·추상 멤버는 기울임, 관계는 실선·점선 + 양끝 모양(상속은
// 속이 빈 삼각형, 합성은 채운 마름모, 집합은 속이 빈 마름모, 의존은 원본 dependency 표식의 채운 갈고리 화살촉 — marker 없이
// path), 관계 라벨은 선 가운데의 바탕 상자, 양끝 수량 글은 끝 가까이에 바탕 없이다.
//
// 결과 SVG 는 허용 목록의 요소·속성만 쓰고(흐름도·간트·타임라인·순서도·ER·상태도와 같은 목록) 글자는 문서 글꼴의 <text> 라
// PDF 에서 선택·검색된다. 같은 입력은 같은 출력이다(map 순회 없음, 숫자는 소수 한 자리).
package classdiag

import (
	"fmt"

	"github.com/itda-work/itda-doc/diagram"
)

// 상한. 넘으면 그리지 않고 *Error 다(원문 + 경고). 원문·라벨 상한은 공통(internal/diagram)이고 배치 점 수는 공통 배치의
// 상한(layered.MaxPoints)이다.
const (
	MaxSource    = diagram.MaxSource // 그림 하나의 원문(바이트)
	MaxLabel     = diagram.MaxLabel  // 이름·멤버·라벨 하나의 글자 수(룬)
	MaxClasses   = 200               // 클래스
	MaxRelations = 500               // 관계
	MaxMembers   = 50                // 클래스 하나의 멤버(속성 + 메서드)
	MaxCard      = 20                // 양끝 수량 글 하나의 글자 수(룬)
	maxNameList  = 20                // classDef·cssClass 의 이름 수(쓰지 않는 줄이라 짧게 묶는다)
)

// Error·Issue·Result 는 그림 종류가 함께 쓰는 공개 꼴이다(internal/diagram).
type (
	Error  = diagram.Error
	Issue  = diagram.Issue
	Result = diagram.Result
)

// Dir 는 그리는 방향이다(direction TB·BT·LR·RL — 원본 기본 TB, 마지막이 이긴다 — classDb setDirection).
type Dir int

const (
	TB Dir = iota
	BT
	LR
	RL
)

// End 는 관계 한 끝의 모양이다(원본 relationType — 없으면 None).
type End int

const (
	None        End = iota
	Extension       // <|·|> 상속 — 속이 빈 삼각형
	Composition     // * 합성 — 채운 마름모
	Aggregation     // o 집합 — 속이 빈 마름모
	Dependency      // <·> 의존 — 화살촉
)

var endName = [...]string{"none", "extension", "composition", "aggregation", "dependency"}

func (e End) String() string { return endName[e] }

// Classifier 는 멤버의 분류자다(원본 classifier — $ 정적은 밑줄, * 추상은 기울임).
type Classifier int

const (
	Plain Classifier = iota
	Static
	Abstract
)

// member 는 속성·메서드 하나다(원본 ClassMember). Text 는 원본 classBox 가 그리는 글(가시성 + 이름 + 메서드면 (인자) 와
// 「 : 반환형」 — 분류자 글자는 빠진다)을 엔티티 정책 B 로 되돌린 값이고, Vis 는 가시성 글자(+ - # ~, 없으면 0)다.
type member struct {
	Method bool
	Vis    rune
	Text   string
	Cls    Classifier
	Line   int
}

// classNode 는 클래스 하나다 — 처음 나온 차례(원본 classDb addClass 의 Map 차례). Annot 은 주석(«이름» — 원본은 첫 주석만
// 그린다 — 이 판은 클래스마다 하나만 받는다). Attrs·Methods 는 원문 차례(원본 addMembers 가 뒤집힌 목록을 되돌린다).
// Type 은 이름의 제네릭 인자(`이름~T~` 의 T — 원본 splitClassNameAndType 의 split('~')[1])다. 클래스를 처음 만든 자리의 값만
// 남는다(원본 addClass 는 이미 있는 이름이면 아무것도 하지 않는다). 이름(Name)에는 ~…~ 가 붙지 않는다 — `A~T~` 와 `A` 는
// 같은 클래스다.
type classNode struct {
	Name      string
	Type      string
	Annot     string
	Attrs     []member
	Methods   []member
	Line      int
	AnnotLine int
}

// Title 은 상자 이름 칸의 글이다 — 원본 addClass 의 text `이름&lt;인자&gt;`(그려지면 「이름<인자>」), 인자가 없으면 이름.
func (c *classNode) Title() string {
	if c.Type == "" {
		return c.Name
	}
	return c.Name + "<" + c.Type + ">"
}

// relation 은 관계 하나다(원문 차례). Type1 은 왼끝(From) 쪽, Type2 는 오른끝(To) 쪽 모양이다(원본 type1·type2 —
// arrowTypeStart·arrowTypeEnd). Card1·Card2 는 양끝 수량 글(원본 relationTitle1·2 — startLabelRight·endLabelLeft).
type relation struct {
	From, To     int // chart.Classes 의 차례
	Type1, Type2 End
	Dotted       bool
	Label        string
	Card1, Card2 string
	Line         int
}

// chart 는 읽은 그림이다.
type chart struct {
	Dir       Dir
	FMCaption string // frontmatter title — 그림 캡션
	AccTitle  string
	AccDescr  string
	Classes   []*classNode
	Rels      []relation
	byName    map[string]int

	// 글 필드가 나온 줄(출력 관문 gate 의 줄 번호). FMLine 은 frontmatter 의 title 줄이다(여섯 종류와 같다).
	FMLine, AccTitleLine, AccDescrLine int
}

// Render 는 mermaid classDiagram 원문 하나(코드 블록 안)를 그린다. 그리지 못하면 *Error 다.
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
