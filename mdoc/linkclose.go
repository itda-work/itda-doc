package mdoc

import (
	"fmt"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// MaxLinkClosersPerBlock 은 블록 하나에서 링크로 닫아 보는 ] 의 수 상한이다. 넘는 ] 는 글자다(경고 한 번).
// goldmark 링크 파서는 ]( 마다 주소를 줄 끝까지 읽어 `[a](` 반복 128KB 가 5초였다(기존 결함). 사람이 쓰는 문단의 링크는 수십 개다.
const MaxLinkClosersPerBlock = 200

// linkCloserGuard 는 링크 파서(200)보다 먼저 ] 를 세어, 블록의 상한을 넘으면 ] 를 글자로 읽는다.
type linkCloserGuard struct{ st *state }

func (g *linkCloserGuard) Trigger() []byte { return []byte{']'} }

func (g *linkCloserGuard) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	g.st.closers[parent]++
	if g.st.closers[parent] <= MaxLinkClosersPerBlock {
		return nil
	}
	_, seg := block.PeekLine()
	if g.st.closers[parent] == MaxLinkClosersPerBlock+1 {
		g.st.warn(g.st.lines.line(seg.Start), WarnTooComplex, fmt.Sprintf("링크 표시(])가 %d 개를 넘어 이 문단의 뒤의 것은 글자로 둠", MaxLinkClosersPerBlock))
	}
	block.Advance(1)
	return ast.NewTextSegment(seg.WithStop(seg.Start + 1))
}
