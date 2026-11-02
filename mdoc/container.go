package mdoc

import (
	"fmt"
	"regexp"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// ::: 컨테이너 파서(1.5, Pandoc fenced_divs).
//
// 여는 줄: 콜론 3개 이상 + 이름 그리고/또는 {속성}. 줄 끝 콜론은 무시한다.
// 닫는 줄: 콜론만 있는 줄. 가장 안쪽에 열린 컨테이너를 닫는다.
// 문단 중간에서는 열리지 않는다(Open 이 막는다).

var (
	openRe = regexp.MustCompile(`^(:{3,})[ \t]*([\p{L}\p{N}_-]+)?[ \t]*(?:\{([^{}\n]*)\})?[ \t]*:*[ \t]*\n?$`)
)

// isCloseLine 은 닫는 줄(`^:{3,}[ \t]*\n?$`)인지다. 열린 컨테이너마다 줄마다 부르므로 정규식을 쓰지 않는다.
func isCloseLine(b []byte) bool {
	i := 0
	for i < len(b) && b[i] == ':' {
		i++
	}
	if i < 3 {
		return false
	}
	for i < len(b) && (b[i] == ' ' || b[i] == '\t') {
		i++
	}
	return i == len(b) || (i == len(b)-1 && b[i] == '\n')
}

type containerParser struct{ st *state }

func (p *containerParser) Trigger() []byte { return []byte{':'} }

func (p *containerParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, seg := reader.PeekLine()
	pos := pc.BlockOffset()
	if pos < 0 || pos >= len(line) {
		return nil, parser.NoChildren
	}
	// 바깥 컨테이너가 안쪽에 넘긴 닫는 줄이 안쪽까지 닿지 못하고(사이의 목록 항목·인용이 끝나서) 블록을 새로 여는 자리로 왔다 —
	// 이 줄은 아직 열린 가장 안쪽 컨테이너(넘긴 컨테이너 가운데 마지막)의 것이다. 닫는 줄을 표시로 받고 그 컨테이너는 다음 줄에서 닫는다.
	if ln, _ := reader.Position(); p.st.passedBy != nil && ln == p.st.passedLine && isCloseLine(line[pos:]) {
		c := p.st.passedBy
		p.st.passedBy = nil
		c.closed, c.closeNext = true, true
		c.endLine = p.st.lines.line(seg.Start)
		reader.AdvanceToEOL()
		return &closeMark{}, parser.NoChildren
	}
	// 문단 중간에서는 열리지 않는다. 위의 닫는 줄은 문단(게으른 이어짐)도 끊어야 해서 CanInterruptParagraph 대신 여기서 막는다
	if ast.IsParagraph(pc.LastOpenedBlock().Node) {
		return nil, parser.NoChildren
	}
	m := openRe.FindSubmatch(line[pos:])
	if m == nil || (m[2] == nil && m[3] == nil) {
		return nil, parser.NoChildren
	}
	lineNo := p.st.lines.line(seg.Start)
	n := &Container{Name: string(m[2])}
	if m[3] != nil {
		a, err := parseAttrs(string(m[3]))
		if err != nil {
			// 속성을 못 읽은 여는 줄은 컨테이너로 보지 않는다 — 글자로 남아 틀린 곳이 보인다
			p.st.warn(lineNo, WarnBadAttribute, "컨테이너 속성을 읽지 못함: "+err.Error())
			return nil, parser.NoChildren
		}
		n.Attrs = a
		if w, ok := a.get("width"); ok {
			css, ok := cssLength(w)
			switch {
			case n.Name != "column" && !a.has("column"):
				// column(이름 :::column 또는 Pandoc 식 클래스 {.column}) 말고는 width 도 data-width 로만 나간다
			case ok:
				n.Width = css
				n.Attrs.Data = a.without("width")
			default:
				p.st.warn(lineNo, WarnBadAttribute, fmt.Sprintf("column width %q 는 길이 형식(40%%·60mm 등)이 아니라 버림", w))
				n.Attrs.Data = a.without("width")
			}
		}
	}
	reader.AdvanceToEOL()
	p.st.containers = append(p.st.containers, n)
	return n, parser.HasChildren
}

func (p *containerParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	n, ok := node.(*Container)
	if !ok || n.closeNext { // 닫는 줄 표시, 또는 앞 줄의 닫는 줄을 표시로 받은 컨테이너
		return parser.Close
	}
	line, seg := reader.PeekLine()
	w, pos := util.IndentWidth(line, reader.LineOffset())
	if w >= 4 || pos >= len(line) || !isCloseLine(line[pos:]) {
		return parser.Continue | parser.HasChildren
	}
	// 닫는 줄은 가장 안쪽에 열린 컨테이너의 것이다. goldmark 은 열린 블록을 바깥부터 묻기 때문에 바깥이 먼저 이 줄을 본다 —
	// 이 노드 안쪽에 열린 컨테이너가 있으면(사이에 목록·인용·각주·정의 목록이 있어도) 가져가지 않고 안쪽 블록들에 넘긴다.
	// 가장 안쪽 열린 블록이 fenced 코드여도 넘긴다(코드 안의 ::: 는 코드다). 넘긴 줄은 goldmark 의 실제 이어가기를 따라
	// 안쪽 컨테이너가 받아 닫거나, 사이 블록이 끝나 블록을 여는 자리로 오면 Open 이 아직 열린 가장 안쪽 넘긴 컨테이너를 닫는다.
	// 열린 컨테이너는 state 의 스택으로 알고 열린 블록 목록을 훑지 않는다(줄마다 깊이의 제곱이 된다).
	if p.st.innerContainer() != n || isFencedCode(pc.LastOpenedBlock().Node) {
		ln, _ := reader.Position()
		p.st.passedBy, p.st.passedLine = n, ln
		return parser.Continue | parser.HasChildren
	}
	p.st.passedBy = nil
	reader.AdvanceToEOL()
	n.closed = true
	n.endLine = p.st.lines.line(seg.Start)
	return parser.Close
}

func isFencedCode(n ast.Node) bool {
	_, ok := n.(*ast.FencedCodeBlock)
	return ok
}

func (p *containerParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {
	n, ok := node.(*Container)
	if !ok { // 닫는 줄 표시는 트리에 남기지 않는다
		node.Parent().RemoveChild(node.Parent(), node)
		return
	}
	p.st.popContainer(n)
	if !n.closed && !p.st.indentedClose(n) {
		name := n.Name
		if name == "" {
			name = "{…}"
		}
		p.st.warn(p.st.lines.line(n.Pos()), WarnUnclosedContainer,
			fmt.Sprintf(":::%s 가 닫히지 않아 감싼 블록 끝에서 닫음", name))
	}
}

// closeMark 는 넘긴 닫는 줄을 받은 자리다(Open). 그 줄을 다른 블록이 갖지 않게 하고, 닫힐 때 트리에서 빠진다.
type closeMark struct{ ast.BaseBlock }

var kindCloseMark = ast.NewNodeKind("closeMark")

func (n *closeMark) Kind() ast.NodeKind         { return kindCloseMark }
func (n *closeMark) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }

func (p *containerParser) CanInterruptParagraph() bool { return true } // 막는 것은 Open
func (p *containerParser) CanAcceptIndentedLine() bool { return false }
