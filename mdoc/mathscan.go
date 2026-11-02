package mdoc

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// 수식 구분자 찾기(1.10). 규칙은 이 파일 하나에 있다 — 글 안 수식($…$·$$…$$)과 줄 머리 블록($$ 줄)이 같은 texScanner 를 쓴다.
//
// texScanner 의 상태는 셋뿐이다: \text{…} 의 괄호 깊이(text), TeX 주석 안인지(comment), 그리고 글자를 짝으로 먹는 이스케이프.
// 글자 하나(또는 짝)를 읽으면 그 뜻을 tokChar·tokSpace·tokDollar 하나로 돌려준다:
//
//	읽은 것                  상태 바꿈                         뜻(text = 0)       뜻(text > 0)
//	\ + 글자 하나             \text{ 면 text = 1(주석 밖일 때만)   글자               글자
//	% (이스케이프 아님)        comment = 참(줄 끝까지)              글자               글자
//	{ · }                    text 안이고 주석 밖이면 깊이 ±1      글자               글자
//	공백·탭·줄바꿈             —                                 공백               글자
//	$                        —                                 구분자 후보         글자
//
// 주석은 명령·괄호의 뜻만 없앤다: 주석 안의 \text{ 는 깊이를 바꾸지 않고, 이미 열린 \text{…} 안이면 주석 안의 $ 도 글자다.
// \text 밖에서는 주석 안의 $ 도 구분자 후보다 — Pandoc 은 TeX 주석을 모르고 $ 로 닫는다(주석은 Temml 이 읽는다).
// 그 밖의 Pandoc 규칙(여는 $ 뒤·닫는 $ 앞 공백, 닫는 $ 뒤 숫자, \text{…} 안의 $)은 파서가 이 뜻을 보고 정한다.
//
// 읽기는 mathCursor 하나가 한다: 줄 넘김과 전진이 모두 next 를 지나고, 지난 바이트는 모두 read 에 더해진다 —
// 실패한 시도의 비용(예산)을 빠뜨리는 길이 구조상 없다(리뷰 R3: 공백을 훑은 뒤의 실패가 비용에서 빠졌다).

func isSpaceByte(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

type texTok uint8

const (
	tokChar   texTok = iota // 수식 글자
	tokSpace                // \text 밖의 공백 — "공백 바로 뒤의 $ 는 닫지 못한다"
	tokDollar               // \text 밖의 이스케이프하지 않은 $ — 구분자 후보
)

type texScanner struct {
	text    int  // \text{ 안의 { 깊이. 0 이면 수식 안
	comment bool // % 주석 안(줄 끝까지)
}

// newline 은 줄이 바뀔 때 부른다 — 주석은 줄 끝에서 끝난다.
func (s *texScanner) newline() { s.comment = false }

// step 은 line[i] 에서 글자 하나(이스케이프면 짝)를 읽고 다음 자리와 뜻을 돌려준다. 위 표가 전부다.
func (s *texScanner) step(line []byte, i int) (next int, tok texTok) {
	c := line[i]
	switch {
	case c == '\\':
		if !s.comment && s.text == 0 && bytes.HasPrefix(line[i+1:], []byte("text{")) {
			s.text = 1
			return i + 6, tokChar
		}
		return min(i+2, len(line)), tokChar
	case c == '%' && !s.comment:
		s.comment = true
		return i + 1, tokChar
	case s.text > 0:
		if !s.comment {
			switch c {
			case '{':
				s.text++
			case '}':
				s.text--
			}
		}
		return i + 1, tokChar
	case isSpaceByte(c):
		return i + 1, tokSpace
	case c == '$':
		return i + 1, tokDollar
	}
	return i + 1, tokChar
}

// mathCursor 는 글 안 수식 하나를 찾는 동안 문단의 줄을 읽는다. 모든 전진이 take 를 지나 read 가 실제로 읽은 바이트다.
type mathCursor struct {
	block text.Reader
	line  []byte
	seg   text.Segment
	i     int // line 안의 자리
	read  int
	sc    texScanner
}

// take 는 n 바이트를 지난다(같은 줄 안).
func (c *mathCursor) take(n int) { c.i += n; c.read += n }

// next 는 토큰 하나를 읽는다. 줄이 끝나면 다음 줄로 넘어가고, 문단이 끝나면 eof.
func (c *mathCursor) next() (tok texTok, b []byte, eof bool) {
	for c.i >= len(c.line) {
		c.block.AdvanceLine()
		c.line, c.seg = c.block.PeekLine()
		if c.line == nil {
			return 0, nil, true
		}
		c.i = 0
		c.sc.newline()
	}
	n, tok := c.sc.step(c.line, c.i)
	b = c.line[c.i:n]
	c.take(n - c.i)
	return tok, b, false
}

// peek 는 다음 바이트를 읽지 않고 본다(같은 줄 안만 — 닫는 $$ 의 둘째 $, 닫는 $ 뒤의 숫자). 없으면 0.
func (c *mathCursor) peek() byte {
	if c.i < len(c.line) {
		return c.line[c.i]
	}
	return 0
}

// 탐색 예산(리뷰 R1 P1·R3). 닫히지 않는 $ 는 문단 끝까지 읽고 실패한다 — $ 가 많은 문단에서 매번 그러면 이차 시간이다.
// 실패한 시도가 읽은 바이트(mathCursor.read — 공백도, 줄 넘김도)를 문단마다 모아 예산(64KB + 문단 길이의 4배)을 넘으면
// 그 문단의 뒤의 $ 는 글자로 둔다(경고 한 번). 시도 하나도 남은 예산을 넘으면 그 자리에서 멈춘다(긴 공백 한가운데라도).
// 시도를 수식 길이 상한(4000바이트)에서 끊지는 않는다 — 닫힌 긴 수식은 수식 노드가 되어 그릴 때 원문 + 길이 경고다(리뷰 R2).
// 성공한 시도는 읽은 글을 소비하므로 세지 않는다 — 모두 합해 문단 길이에 비례한다.
const (
	mathScanBase    = 64 << 10
	mathScanPerByte = 4
)

func (p *mathInlineParser) budget(parent ast.Node) int {
	if rem, ok := p.st.mathBudget[parent]; ok {
		return rem
	}
	n := 0
	for i := 0; i < parent.Lines().Len(); i++ {
		seg := parent.Lines().At(i)
		n += seg.Len()
	}
	rem := mathScanBase + mathScanPerByte*n
	p.st.mathBudget[parent] = rem
	return rem
}

func (p *mathInlineParser) spend(parent ast.Node, n, line int) {
	rem := p.st.mathBudget[parent]
	if rem > 0 && rem-n <= 0 {
		p.st.warn(line, WarnMath, "닫히지 않은 $ 가 너무 많아 이 문단의 뒤의 $ 는 수식으로 읽지 않고 글자로 둠")
	}
	p.st.mathBudget[parent] = rem - n
}

func (p *mathInlineParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	// 앞 글자가 $ 여도 연다 — Pandoc 은 실패한 $ 다음 자리에서 다시 본다($x$$y$ 는 수식 둘, $$}1$ 은 $ + 수식).
	// Pandoc 차등 시험(TestMathPropPandoc)이 찾았다
	line, seg := block.PeekLine()
	display := len(line) > 1 && line[1] == '$'
	opener := 1
	if display {
		opener = 2
	} else if len(line) < 2 || isSpaceByte(line[1]) {
		return nil // 여는 $ 뒤가 공백이면 수식이 아니다
	}
	rem := p.budget(parent)
	if rem <= 0 {
		return nil
	}
	savedLine, savedPos := block.Position()
	start := seg.Start
	c := &mathCursor{block: block, line: line, seg: seg}
	c.take(opener)
	fail := func() ast.Node {
		p.spend(parent, c.read, p.st.lines.line(start))
		block.SetPosition(savedLine, savedPos)
		return nil
	}
	var tex bytes.Buffer
	afterSpace := false
	for c.read <= rem {
		tok, b, eof := c.next()
		if eof {
			return fail()
		}
		if tok == tokDollar {
			switch {
			case display && c.peek() == '$':
				if strings.TrimSpace(tex.String()) == "" {
					return fail()
				}
				c.take(1)
				block.Advance(c.i)
				return p.node(parent, block, start, c.seg.Start+c.i, tex.String(), true)
			case !display && afterSpace:
				return fail() // 공백 바로 뒤의 $ 는 닫지 못한다 — Pandoc 은 그 자리에서 수식이 아니라고 본다
			case !display && c.peek() >= '0' && c.peek() <= '9':
				return fail() // 닫는 $ 뒤의 숫자 — "$5 와 $6" 의 앞 $ 는 수식이 아니다
			case !display:
				block.Advance(c.i)
				return p.node(parent, block, start, c.seg.Start+c.i, tex.String(), false)
			}
		}
		afterSpace = tok == tokSpace
		tex.Write(b)
	}
	return fail() // 남은 예산을 다 읽음
}
