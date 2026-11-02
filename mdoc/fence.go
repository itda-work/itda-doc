package mdoc

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// fenceCloser 는 Goldmark 가 실제 닫는 펜스를 소비한 줄만 기록한다.
// 컨테이너나 문서 끝에서 암묵적으로 닫히면 Continue 가 Close 를 돌려주지 않는다.
type fenceCloser struct {
	parser.BlockParser
	st *state
}

func (p *fenceCloser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	_, seg := reader.PeekLine()
	result := p.BlockParser.Continue(node, reader, pc)
	if result&parser.Close != 0 {
		p.st.fenceCloses[node.(*ast.FencedCodeBlock)] = p.st.lines.line(seg.Start)
	}
	return result
}

// 기본 fenced 파서를 그 자리에서 감싼다. 우선순위 700과 다른 파서의 순서는 그대로다.
type fenceCloserOption struct{ st *state }

func (o fenceCloserOption) SetParserOption(c *parser.Config) {
	for i, v := range c.BlockParsers {
		if v.Value == parser.NewFencedCodeBlockParser() {
			c.BlockParsers[i].Value = &fenceCloser{BlockParser: v.Value.(parser.BlockParser), st: o.st}
			return
		}
	}
}
