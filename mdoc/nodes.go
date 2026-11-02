package mdoc

import "github.com/yuin/goldmark/ast"

// 이 패키지가 더하는 AST 노드. 모두 렌더러(render.go)가 그린다.

// Container 는 ::: 로 연 블록이다(1.5). 안은 보통의 마크다운이다.
type Container struct {
	ast.BaseBlock
	Name  string
	Attrs attrs
	Width string // column 의 width — 길이 형식을 통과한 CSS 값
	// closed 는 닫는 줄로 닫혔는지다. 거짓이면 문서(또는 바깥 블록) 끝에서 닫힌 것이다.
	closed bool
	// closeNext 는 넘긴 닫는 줄을 표시로 받아(container.go Open) 다음 줄에서 닫힌다는 것이다.
	closeNext bool
	// endLine 은 닫는 줄의 원문 줄(1부터)이다. 닫는 줄로 닫히지 않았으면 0 — 블록 범위 읽기(TopBlockAt)만 쓴다.
	endLine int
}

var KindContainer = ast.NewNodeKind("Container")

// classes 는 컨테이너 div 에 붙는 class 다 — 이름, 그다음 속성의 class. 렌더(render.go container)와 노트 판정(transform.go
// notes)이 같은 값을 쓴다.
func (n *Container) classes() []string {
	var cs []string
	if n.Name != "" {
		cs = append(cs, n.Name)
	}
	return append(cs, n.Attrs.Classes...)
}

func (n *Container) Kind() ast.NodeKind { return KindContainer }
func (n *Container) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{"Name": n.Name}, nil)
}

// Alert 는 GFM 알림 > [!NOTE] 이다(1.6). 원래 blockquote 의 자식을 그대로 가진다.
type Alert struct {
	ast.BaseBlock
	AlertType string // note · tip · important · warning · caution
}

var KindAlert = ast.NewNodeKind("Alert")

func (n *Alert) Kind() ast.NodeKind { return KindAlert }
func (n *Alert) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{"Type": n.AlertType}, nil)
}

// Figure 는 이미지 하나만 있는 문단이다(1.7). 자식은 원래 문단의 인라인이고, 그림 바로 다음의
// Figure: 문단이 FigureCaption 자식으로 옮겨 온다(아래).
type Figure struct {
	ast.BaseBlock
	Caption string // 이미지 title. FigureCaption 자식이 없을 때만 figcaption 이 된다
}

var KindFigure = ast.NewNodeKind("Figure")

func (n *Figure) Kind() ast.NodeKind         { return KindFigure }
func (n *Figure) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }

// FigureCaption 은 그림 바로 다음의 Figure: 문단이다(1.7, #466). 그림의 마지막 자식으로 옮겨
// <figcaption> 이 된다 — 인라인 서식을 살리는 것은 TableCaption 과 같은 꼴이다.
type FigureCaption struct{ ast.BaseBlock }

var KindFigureCaption = ast.NewNodeKind("FigureCaption")

func (n *FigureCaption) Kind() ast.NodeKind         { return KindFigureCaption }
func (n *FigureCaption) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }

// PageBreak 는 쪽 나눔이다(1.4). 세 가지 표기가 모두 이것이 된다.
type PageBreak struct{ ast.BaseBlock }

var KindPageBreak = ast.NewNodeKind("PageBreak")

func (n *PageBreak) Kind() ast.NodeKind         { return KindPageBreak }
func (n *PageBreak) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }

// TOCNav 는 [TOC] 자리다(1.8). 그릴 때 목차를 채운다.
type TOCNav struct{ ast.BaseBlock }

var KindTOCNav = ast.NewNodeKind("TOCNav")

func (n *TOCNav) Kind() ast.NodeKind         { return KindTOCNav }
func (n *TOCNav) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }

// TableCaption 은 표 바로 다음의 Table: 문단이다(1.7). 표의 첫 자식으로 옮겨 <caption> 이 된다.
type TableCaption struct{ ast.BaseBlock }

var KindTableCaption = ast.NewNodeKind("TableCaption")

func (n *TableCaption) Kind() ast.NodeKind         { return KindTableCaption }
func (n *TableCaption) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }

// Pause 는 한 줄 문단 ". . ." 이다(1.12, Pandoc 의 멈춤). 발표에서는 그 자리에서 멈추고(다음 누름에 뒤가 나타난다),
// 문서(A4)에서는 사라진다(화면이 쪽 나눔 전에 뺀다 — 숨김 클래스만으로는 쪽 맨 위 여백 규칙이 어긋난다).
type Pause struct{ ast.BaseBlock }

var KindPause = ast.NewNodeKind("Pause")

func (n *Pause) Kind() ast.NodeKind         { return KindPause }
func (n *Pause) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }
