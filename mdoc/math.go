package mdoc

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/itda-work/itda-doc/texmath"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// 수식(1.10). 표기는 Pandoc tex_math_dollars 다:
//
//   - $…$ 글 안 수식: 여는 $ 바로 뒤가 공백이 아니고, 닫는 $ 바로 앞이 공백이 아니며, 닫는 $ 바로 뒤가 숫자가 아니다.
//     그래서 "$30 와 $40" 같은 금액은 수식이 아니다. 줄을 넘어도 되지만(빈 줄은 문단을 끝낸다) 공백 바로 뒤의 $ 는 닫지 못한다.
//   - $$…$$ 블록 수식: 글 안에 있으면 그 자리에서 블록 꼴로, 줄 머리의 $$ 로 시작하면 블록 하나다(빈 줄 전까지 닫는 $$ 를 찾는다).
//   - \$ 는 글자 $ 다(goldmark 의 이스케이프 — 이 파서가 불리지 않는다). 수식 안의 \$ 는 닫지 않는다.
//   - 코드·코드 블록 안의 $ 는 글자다(코드가 먼저 읽힌다). ```math 코드 블록은 GitHub 처럼 블록 수식이다.
//   - 표 칸 안에서는 \| 가 | 다(GFM 규칙 — 칸을 나누지 않는 세로줄). 칸 안에서 ‖ 는 \Vert 로 쓴다.
//
// 그리기는 texmath(Temml → 허용 목록 MathML). 그리지 못한 수식은 원문 글자 그대로 두고 줄 번호 경고를 낸다.

// MaxMathPerDoc 는 문서 하나에서 그리는 수식 수의 상한이다. 넘는 수식은 원문 글자로 둔다(경고 한 번).
const MaxMathPerDoc = 2000

// MathInline 은 글 안의 $…$ 또는 $$…$$ 다.
type MathInline struct {
	ast.BaseInline
	Tex     string
	Raw     string // 구분자를 포함한 원문 — 그리지 못하면 이 글자를 낸다
	Display bool
	Line    int
}

var KindMathInline = ast.NewNodeKind("MathInline")

func (n *MathInline) Kind() ast.NodeKind { return KindMathInline }
func (n *MathInline) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{"Tex": n.Tex}, nil)
}

// MathBlock 은 줄 머리의 $$ 로 연 블록 수식이다. closed 가 거짓이면 닫는 $$ 없이 끝난 것이다(원문 + 경고).
type MathBlock struct {
	ast.BaseBlock
	closed bool
	raw    []text.Segment // 읽은 줄(여는 $$·닫는 $$ 포함, 인용·목록 표지 뺌) — 그리지 못하면 이 원문을 그대로 낸다(리뷰 R1 P2)
	sc     texScanner
}

var KindMathBlock = ast.NewNodeKind("MathBlock")

func (n *MathBlock) Kind() ast.NodeKind         { return KindMathBlock }
func (n *MathBlock) IsRaw() bool                { return true }
func (n *MathBlock) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }

// ---- 글 안 수식

type mathInlineParser struct{ st *state }

func (p *mathInlineParser) Trigger() []byte { return []byte{'$'} }

func (p *mathInlineParser) node(parent ast.Node, block text.Reader, start, stop int, tex string, display bool) ast.Node {
	src := block.Source()
	tex = strings.TrimSpace(tex)
	if inTableCell(parent) {
		tex = strings.ReplaceAll(tex, `\|`, `|`)
	}
	return &MathInline{Tex: tex, Raw: string(src[start:stop]), Display: display, Line: p.st.lines.line(start)}
}

func inTableCell(n ast.Node) bool {
	for ; n != nil; n = n.Parent() {
		if n.Kind() == extast.KindTableCell {
			return true
		}
	}
	return false
}

// ---- 블록 수식: 줄 머리의 $$

type mathBlockParser struct{ st *state }

func (p *mathBlockParser) Trigger() []byte { return []byte{'$'} }

func (p *mathBlockParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, seg := reader.PeekLine()
	pos := pc.BlockOffset()
	if pos < 0 || !bytes.HasPrefix(line[pos:], []byte("$$")) || bytes.HasPrefix(line[pos:], []byte("$$$")) {
		return nil, parser.NoChildren
	}
	n := &MathBlock{raw: []text.Segment{text.NewSegment(seg.Start+pos, seg.Stop)}}
	rest := line[pos+2:]
	k, trailing := n.scanLine(rest)
	if k >= 0 {
		if trailing || len(bytes.TrimSpace(rest[:k])) == 0 {
			// $$x$$ 뒤에 글이 있으면 문단 안의 수식이다(글 안 파서가 읽는다). 빈 $$ $$ 도 문단(글자)이다
			return nil, parser.NoChildren
		}
		n.Lines().Append(text.NewSegment(seg.Start+pos+2, seg.Start+pos+2+k))
		n.closed = true
		reader.AdvanceToEOL()
		return n, parser.Close
	}
	if len(bytes.TrimSpace(rest)) > 0 {
		n.Lines().Append(text.NewSegment(seg.Start+pos+2, seg.Stop))
	}
	reader.AdvanceToEOL()
	return n, parser.NoChildren
}

// scanLine 은 블록 수식의 한 줄을 읽어(상태는 줄을 넘어 이어진다) 닫는 $$ 의 자리를 찾는다. 없으면 -1.
// trailing 은 그 $$ 뒤에 글이 더 있는지다. 이스케이프 판정은 texScanner 가 한다 — a\\$$ 의 $$ 는 닫는다(리뷰 R1 P2).
func (n *MathBlock) scanLine(line []byte) (at int, trailing bool) {
	n.sc.newline()
	for i := 0; i < len(line); {
		next, tok := n.sc.step(line, i)
		if tok == tokDollar && next < len(line) && line[next] == '$' {
			return i, len(bytes.TrimSpace(line[i+2:])) > 0
		}
		i = next
	}
	return -1, false
}

func (p *mathBlockParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	line, seg := reader.PeekLine()
	if util.IsBlank(line) {
		return parser.Close // 빈 줄 — 닫는 $$ 없이 끝남(Close 가 경고)
	}
	n := node.(*MathBlock)
	n.raw = append(n.raw, seg)
	if k, trailing := n.scanLine(line); k >= 0 && !trailing {
		if body := line[:k]; len(bytes.TrimSpace(body)) > 0 {
			n.Lines().Append(text.NewSegment(seg.Start, seg.Start+k))
		}
		n.closed = true
		reader.AdvanceToEOL()
		return parser.Close
	}
	// 줄 가운데의 $$(뒤에 글이 있음)는 닫지 않는다 — 그 줄은 내용이고 Temml 이 오류를 알린다
	node.Lines().Append(seg)
	reader.AdvanceToEOL()
	return parser.Continue | parser.NoChildren
}

func (p *mathBlockParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {
	if n := node.(*MathBlock); !n.closed {
		p.st.warn(p.st.lines.line(n.Pos()), WarnMath, "닫는 $$ 가 없어 수식을 그리지 않고 원문 그대로 둠")
	}
}

func (p *mathBlockParser) CanInterruptParagraph() bool { return true }
func (p *mathBlockParser) CanAcceptIndentedLine() bool { return false }

func (n *MathBlock) tex(src []byte) string { return rawLines(n, src) }

// rawLines 는 블록의 줄을 이어 붙인 원문이다(앞뒤 공백을 뗀다).
func rawLines(n ast.Node, src []byte) string {
	var b bytes.Buffer
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		b.Write(seg.Value(src))
	}
	return strings.TrimSpace(b.String())
}

// ---- 그리기

// math 는 수식 하나를 그린다. 그리지 못하면 원문을 글자로 쓰고 경고를 남긴다.
func (st *state) math(tex string, display bool, line int) (string, bool) {
	st.mathCount++
	if st.mathCount > MaxMathPerDoc {
		if st.mathCount == MaxMathPerDoc+1 {
			st.warn(line, WarnMath, fmt.Sprintf("수식이 %d 개를 넘어 뒤의 수식은 원문 그대로 둠", MaxMathPerDoc))
		}
		return "", false
	}
	out, err := texmath.Render(tex, display)
	if err != nil {
		st.warn(line, WarnMath, "수식을 그리지 않고 원문 그대로 둠: "+err.Error())
		return "", false
	}
	return out, true
}

func (r *htmlRenderer) mathInline(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	n := node.(*MathInline)
	if out, ok := r.st.math(n.Tex, n.Display, n.Line); ok {
		_, _ = w.WriteString(out)
	} else {
		_, _ = w.WriteString(`<span class="math-error">`)
		_, _ = w.Write(util.EscapeHTML([]byte(n.Raw)))
		_, _ = w.WriteString(`</span>`)
	}
	return ast.WalkSkipChildren, nil
}

func (r *htmlRenderer) mathBlock(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	n := node.(*MathBlock)
	line := r.st.lines.line(n.Pos())
	if n.closed {
		if out, ok := r.st.math(n.tex(src), true, line); ok {
			r.mathDiv(w, n, out)
			return ast.WalkSkipChildren, nil
		}
	}
	// 그리지 못한 블록: 원문 줄을 그대로(구분자 포함) 한 문단에
	_, _ = w.WriteString(`<p class="math-error"`)
	dataAttrs(w, n)
	_ = w.WriteByte('>')
	var rb strings.Builder
	for _, seg := range n.raw {
		rb.Write(seg.Value(src))
	}
	raw := strings.TrimRight(rb.String(), "\n")
	_, _ = w.Write(util.EscapeHTML([]byte(raw)))
	_, _ = w.WriteString("</p>\n")
	return ast.WalkSkipChildren, nil
}

func (r *htmlRenderer) mathDiv(w util.BufWriter, n ast.Node, out string) {
	_, _ = w.WriteString(`<div class="math-display"`)
	dataAttrs(w, n)
	_ = w.WriteByte('>')
	_, _ = w.WriteString(out)
	_, _ = w.WriteString("</div>\n")
}
