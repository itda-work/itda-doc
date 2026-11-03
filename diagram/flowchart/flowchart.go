// Package flowchart 는 mermaid 흐름도(flowchart·graph)의 부분집합을 SVG 로 그린다(설계 1.10, itda-hyve#117).
//
// 문법은 실제 Mermaid 의 부분집합이다 — 새 문법을 만들지 않는다(안내서 spec/diagrams.md 의 표가 정본).
// 모르는 문법은 그리지 않고 *Error(줄 번호)를 돌려준다 — 부르는 쪽(mdoc)이 원문 코드 블록을 그대로 두고 경고한다.
// 모양을 문서에 적는 줄(style·classDef·class·linkStyle·:::·%%{init}%%·frontmatter 설정)과 click 은 쓰지 않고
// 그림은 그린다(Issue 경고) — 색·선·글꼴은 테마가 정하고, 링크·스크립트는 내지 않는다.
//
// 그리기는 이 패키지가 처음부터 한다(parse.go → layout.go → svg.go). 외부 렌더러(go-mermaid)의 출력은 그대로 쓸 수
// 없었다 — 기술 검증은 itda-hyve#117 보고서. 배치는 계층 배치(Sugiyama, 공통 diagram/layered): 순환 끊기 → 계층 → 더미 노드(간선 라벨도 더미 하나) →
// 교차 줄이기 → 좌표(가중 등위 회귀) → subgraph 상자 겹침 풀기.
//
// 결과 SVG 는 허용 목록의 요소·속성만 쓴다(차트와 같은 목록 + 곡선 C): style·이벤트·url()·href·id 가 없고, 색·선은
// 클래스로 테마 토큰(--diagram-*)을 따른다. 글자는 문서 글꼴의 <text> 라 PDF 에서 선택·검색된다.
// 같은 입력은 같은 출력이다(map 순회 없음, 숫자는 소수 한 자리).
//
// 흐름도와 간트가 함께 쓰는 부분(공개 꼴·상한·글자 폭·SVG 쓰기·frontmatter·접근성 문장)은 diagram 패키지다.
package flowchart

import (
	"fmt"

	"github.com/itda-work/itda-doc/diagram"
	"github.com/itda-work/itda-doc/diagram/layered"
)

// 상한. 넘으면 그리지 않고 *Error 다(원문 + 경고). 계산 시간은 이 상한 안에서 묶인다.
// 원문·라벨 상한은 공통(diagram)이다.
const (
	MaxSource    = diagram.MaxSource // 그림 하나의 원문(바이트)
	MaxNodes     = 150               // 노드
	MaxEdges     = 300               // 화살표(A & B --> C 는 둘)
	MaxSubgraphs = 30                // subgraph
	MaxLabel     = diagram.MaxLabel  // 라벨 하나의 글자 수(룬)
	MaxLength    = 4                 // 화살표 길이(--->·---->: 2·3 …). 더 긴 화살표는 이 길이로 그린다
	// 배치에 쓰는 점(노드 + 간선이 지나는 더미)의 상한 — 긴 간선이 많은 그림도 계산 시간이 묶이게(공통 배치의 상한)
	maxLayoutNodes = layered.MaxPoints
)

// Error·Issue·Result 는 그림 종류가 함께 쓰는 공개 꼴이다(diagram — gantt 와 같다).
type (
	Error  = diagram.Error
	Issue  = diagram.Issue
	Result = diagram.Result
)

// Render 는 mermaid 원문 하나(코드 블록 안)를 그린다. 그리지 못하면 *Error 다.
func Render(src string) (Result, error) {
	if len(src) > MaxSource {
		return Result{}, &Error{Msg: fmt.Sprintf("그림 원문이 %dKB 를 넘음", MaxSource>>10)}
	}
	g, issues, err := Parse(src)
	if err != nil {
		return Result{}, err
	}
	l, err := layout(g)
	if err != nil {
		return Result{}, err
	}
	return Result{SVG: drawSVG(g, l), Caption: diagram.XmlChars(g.Title), Issues: issues}, nil
}

// Dir 는 그리는 방향이다.
type Dir int

const (
	TB Dir = iota // 위 → 아래(TD 와 같음)
	BT
	LR
	RL
)

// Shape 는 노드 모양이다(Mermaid 표기는 parse.go 의 shapes).
type Shape int

const (
	ShapeRect       Shape = iota // A[글]
	ShapeRound                   // A(글)
	ShapeStadium                 // A([글])
	ShapeSubroutine              // A[[글]]
	ShapeCylinder                // A[(글)]
	ShapeCircle                  // A((글))
	ShapeDiamond                 // A{글}
	ShapeHexagon                 // A{{글}}
	ShapeLean                    // A[/글/]
	ShapeLeanAlt                 // A[\글\]
	ShapeTrap                    // A[/글\]
	ShapeTrapAlt                 // A[\글/]
)

// Stroke 는 화살표 선이다.
type Stroke int

const (
	StrokeNormal    Stroke = iota // -->  ---
	StrokeDotted                  // -.-> -.-
	StrokeThick                   // ==>  ===
	StrokeInvisible               // ~~~ (배치에만 쓰고 그리지 않음)
)

// Node 는 노드 하나다. Label 의 줄바꿈(<br>)은 \n 이다.
type Node struct {
	ID      string
	Label   string
	Shape   Shape
	Cluster int // 들어 있는 subgraph(Graph.Clusters 의 차례), 없으면 -1
	Line    int
	// LabelLine 은 Label 을 정한 줄이다(나중 정의가 이긴다 — 출력 관문의 줄 번호).
	LabelLine int
}

// Edge 는 화살표 하나다.
type Edge struct {
	From, To int // Graph.Nodes 의 차례
	Label    string
	Stroke   Stroke
	Arrow    bool // 끝(To)에 화살촉
	Both     bool // 양쪽 화살촉(<-->)
	Length   int  // 1 + 덧붙인 - 수(MaxLength 까지)
	Line     int
}

// Cluster 는 subgraph 하나다.
type Cluster struct {
	ID, Title string
	Line      int
}

// Graph 는 읽은 그림이다.
type Graph struct {
	Dir      Dir
	Nodes    []*Node
	Edges    []*Edge
	Clusters []*Cluster
	Title    string // frontmatter title
	AccTitle string // accTitle — 그림의 접근성 이름
	AccDescr string // accDescr — 그림 설명
	byID     map[string]int

	// 글 필드가 나온 줄(출력 관문의 줄 번호). TitleLine 은 frontmatter 의 title 줄이다.
	TitleLine, AccTitleLine, AccDescrLine int
}
