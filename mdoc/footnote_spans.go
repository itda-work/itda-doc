package mdoc

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// 각주 정의의 원문 구간(#365 ③ 조각 C 접근 리뷰 ⓐ) — 넣을 자리 판정(InsertSpotAt)의 읽기 전용 파싱에만 붙인다. 렌더(Render)는 붙이지 않는다.
//
// goldmark v1.8.6 의 각주 파서는 Close(extension/footnote.go 95)에서 정의를 원래 부모에서 떼어 문서 끝 목록으로 옮기고, AST 변환
// (footnoteASTTransformer)이 참조되지 않거나 같은 이름에서 뽑히지 않은 정의를 지운다. 그래서 최종 AST 로는 정의의 원래 자리·소속을
// 알 수 없다(리뷰 1·2회차 — 참조 없는 정의 안 목록, 같은 이름 둘째 정의 안 커서가 빈 자리로 읽혔다). 원문을 따로 훑어 정의를 추정하지
// 않고(nesting.go 와 같은 까닭) 실제 파서가 받아들인 정의마다 Open 때의 부모와 원래 Close 를 부르기 전의 구간을 적는다(fence.go 꼴의
// 덮개 — 원래 파서에 다 맡긴다). 이름으로 합치거나 참조 여부로 거르지 않는다.

// footnoteSpan 은 파서가 받아들인 각주 정의 하나의 원문 구간(줄, 1부터)이다. end 는 정의가 Continue 로 받은 마지막 줄(또는 자손 블록
// 원문의 마지막 줄)이라 정의 뒤 빈 줄을 포함할 수 있다 — 호출자가 뺀다. topLevel 은 문서 바로 아래에서 열렸는지(Open 때의 부모)다.
type footnoteSpan struct {
	start, end int
	topLevel   bool
	parent     ast.NodeKind // Open 때의 원래 부모(시험이 본다)
}

type footnoteWatch struct {
	st        *state
	installed int
	open      map[ast.Node]footnoteSpan // 열렸으나 아직 닫히지 않은 정의
	spans     []footnoteSpan
}

// complete 는 관찰기가 각주 파서에 붙었고 연 정의가 모두 닫혔는지다. 거짓이면 구간을 믿을 수 없다 — 호출자는 넣지 않는다.
func (w *footnoteWatch) complete() bool { return w.installed == 1 && len(w.open) == 0 }

type footnoteObserver struct {
	parser.BlockParser
	w *footnoteWatch
}

func (p *footnoteObserver) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	node, state := p.BlockParser.Open(parent, reader, pc) // 중첩 상한(nestingGate)도 이 안에서 돈다
	if node != nil {
		p.w.open[node] = footnoteSpan{topLevel: parent.Kind() == ast.KindDocument, parent: parent.Kind()} // 소속은 지금(Close 가 옮기기 전)만 안다
	}
	return node, state
}

// Continue 는 정의가 받아들인 줄을 적는다 — 닫힐 때의 PeekLine 으로 끝을 추정하지 않는다(리뷰 3회차 [2]: 끝 줄바꿈 없는 마지막
// 줄을 새 블록이 먼저 먹으면 Close 때 PeekLine 이 nil 이라 그 줄이 정의 안으로 읽혔다 — `[^a]: note\n\nbody`).
func (p *footnoteObserver) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	_, seg := reader.PeekLine()
	state := p.BlockParser.Continue(node, reader, pc)
	if sp, ok := p.w.open[node]; ok && state&parser.Continue != 0 {
		sp.end = max(sp.end, p.w.st.lines.line(seg.Start))
		p.w.open[node] = sp
	}
	return state
}

func (p *footnoteObserver) Close(node ast.Node, reader text.Reader, pc parser.Context) {
	if sp, ok := p.w.open[node]; ok {
		delete(p.w.open, node)
		sp.start = p.w.st.lines.line(node.Pos()) // 정의 표식([^이름]:)의 자리 — 파서가 Open 뒤에 정한다
		// 끝 = 정의가 Continue 로 받은 마지막 줄과 자손 블록 원문의 마지막 줄 중 큰 것 — 게으른 이어짐(`[^a]: note\nlazy`)은 정의의
		// Continue 를 거치지 않고 문단이 줄을 받는다.
		_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if entering && n.Type() == ast.TypeBlock {
				for i := 0; i < n.Lines().Len(); i++ {
					if s := n.Lines().At(i); s.Start >= 0 && s.Start <= s.Stop && s.Stop <= len(p.w.st.src) {
						sp.end = max(sp.end, p.w.st.lines.line(max(s.Start, s.Stop-1)))
					}
				}
			}
			return ast.WalkContinue, nil
		})
		sp.end = max(sp.end, sp.start)
		p.w.spans = append(p.w.spans, sp)
	}
	p.BlockParser.Close(node, reader, pc) // 원래 Close — 여기서 정의가 문서 끝 목록으로 옮겨진다
}

type footnoteWatchOption struct{ w *footnoteWatch }

func (o footnoteWatchOption) SetParserOption(c *parser.Config) {
	for i, v := range c.BlockParsers {
		bp, _ := v.Value.(parser.BlockParser)
		if g, ok := bp.(*nestingGate); ok && g.BlockParser == extension.NewFootnoteBlockParser() {
			c.BlockParsers[i].Value = &footnoteObserver{BlockParser: bp, w: o.w}
			o.w.installed++
		}
	}
}

// spotParse 는 넣을 자리 판정의 읽기 전용 파싱 결과다. spans 는 정의 뒤 빈 줄을 뺀 각주 정의 구간이다(정의 안의 빈 줄은 뒤에 정의
// 내용이 있어 남는다).
type spotParse struct {
	src   []byte // 정규화한 원문(frontmatter 포함)
	meta  Meta
	st    *state
	doc   ast.Node
	spans []footnoteSpan
}

// blank 는 l 줄(1부터)이 빈 줄인지다.
func (p spotParse) blank(l int) bool {
	from, to := p.st.lines[l-1], len(p.st.src)
	if l < len(p.st.lines) {
		to = p.st.lines[l]
	}
	return util.IsBlank(p.st.src[from:to]) // 파서와 같은 빈 줄(공백·탭만 — \v·\f 는 글자다, 퍼징 발견)
}

// line 은 l 줄(1부터)의 글이다(줄바꿈 빼고).
func (p spotParse) line(l int) string {
	from, to := p.st.lines[l-1], len(p.st.src)
	if l < len(p.st.lines) {
		to = p.st.lines[l] - 1
	}
	return string(p.st.src[from:to])
}

// indentCols 는 줄 머리 들여쓰기 칸 수다(탭은 다음 4의 배수까지).
func indentCols(s string) int {
	c := 0
	for _, r := range s {
		switch r {
		case ' ':
			c++
		case '\t':
			c += 4 - c%4
		default:
			return c
		}
	}
	return c
}

// parseSpots 는 src 를 렌더와 같은 파서(+ 각주 관찰기)로 읽는다. 중첩 상한을 넘거나 관찰이 끝나지 않으면 ok 가 거짓이다.
func parseSpots(src []byte) (p spotParse, ok bool) {
	p.src = normalize(src)
	var body []byte
	p.meta, body, _, _ = parseFrontmatter(p.src, nil)
	p.st = newState(body, p.meta, Options{})
	w := watchSpots(p.st)
	doc, over := p.st.parseGuarded(body)
	if over > 0 || !w.complete() {
		return p, false
	}
	p.doc = doc
	for _, sp := range w.spans {
		for sp.end > sp.start && p.blank(sp.end) {
			sp.end--
		}
		p.spans = append(p.spans, sp)
	}
	return p, true
}

// watchSpots 는 parseSpots 가 관찰기를 붙이는 함수다 — 시험이 관찰이 끝나지 않은 상태(미설치·두 번 설치·닫히지 않은 정의)를 만들어
// complete 가 거짓일 때 거부하는지 본다.
var watchSpots = watchFootnotes

// watchFootnotes 는 st 의 파서에 각주 정의 관찰기를 붙인다(newState 뒤, 파싱 전에). 중첩 상한 덮개(nestingGate)는 그 안에 그대로 둔다.
func watchFootnotes(st *state) *footnoteWatch {
	w := &footnoteWatch{st: st, open: map[ast.Node]footnoteSpan{}}
	st.md.Parser().AddOptions(footnoteWatchOption{w})
	return w
}

// spotProbe 는 넣는 세 층(datareport.Block — 기록 주석 한 줄·표 또는 :::chart·빈 줄·캡션 문단)과 경계 꼴이 같은 글이다.
const spotProbe = "<!-- itda-data -->\n| x |\n|---|\n| 1 |\n\nTable: x\n"

// keepsAt 은 at 줄에 block(spotProbe 또는 넣을 실제 세 층)을 넣어 본 결과가 기존 글을 그대로 두는지다 — 넣은 기록 주석 바로 다음이
// 최상위 표·차트이고, 넣은 줄을 뺀 원문 구조(outline)가 같다.
func (p spotParse) keepsAt(at int, block string) bool {
	src := string(p.src)
	pos, text, _ := InsertText(src, at, block)
	out := src[:pos] + text + src[pos:]
	comment := strings.Count(src[:pos]+text[:len(text)-len(strings.TrimLeft(text, "\n"))], "\n") + 1
	if tb, ok := TopBlockAt([]byte(out), comment+1); !ok || (tb.Kind != "table" && tb.Kind != "chart") || tb.Start != comment+1 {
		return false
	}
	q, ok := parseSpots([]byte(out))
	if !ok {
		return false
	}
	before, _ := p.outline(nil)
	after, ok := q.outline(insertedLines(src, pos, text))
	return ok && slices.Equal(before, after)
}

// insertedLines 는 src 의 pos 바이트에 text 를 넣은 글의 줄 번호를 원문 줄 번호로 되돌리는 대응이다. 넣은 바이트 [pos, pos+len(text))
// 안에서 시작하는 줄은 새 글이라 ok 가 거짓이고, 그 앞 줄은 그대로, 그 뒤 줄은 넣은 바이트를 뺀 원문 위치의 줄이다. 줄 이동량과
// 넣은 글이 차지한 줄을 따로 센다 — 끝 줄바꿈 없는 글(실제 표 캡션)을 빈 줄 둘 앞에 넣으면 마지막 부분 줄이 원문 빈 줄의 줄바꿈으로
// 끝나, 넣은 글의 줄바꿈 수로는 그 줄이 새 글인지 알 수 없다(리뷰 4회차 [1] — `# heading\n\n\nbody\n` 2행).
func insertedLines(src string, pos int, text string) func(int) (int, bool) {
	out := src[:pos] + text + src[pos:]
	starts := []int{0}
	for i := 0; i < len(out); i++ {
		if out[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return func(l int) (int, bool) {
		if l < 1 {
			return l, true // 자리를 모르는 블록(빈 칸 따위 — blockLine 0)은 그대로 비교한다
		}
		if l > len(starts) {
			return 0, false
		}
		switch b := starts[l-1]; {
		case b < pos:
			return l, true
		case b < pos+len(text):
			return 0, false
		default:
			return strings.Count(src[:b-len(text)], "\n") + 1, true
		}
	}
}

// outline 은 기존 글 보존을 비교할 원문 구조다 — 최종 AST 의 블록마다 (조상 꼴 길·시작 줄·원문 줄 글), 각주 정의 구간마다
// (시작·끝·최상위·원래 부모). shift 는 줄 번호를 원문 줄 번호로 되돌린다(nil 이면 그대로 — insertedLines). 새 글에서 시작한 블록(새
// 세 층)은 뺀다. 각주 정의 구간이 새 글에 걸치면 ok 가 거짓이다.
func (p spotParse) outline(shift func(int) (int, bool)) (out []string, ok bool) {
	ok = true
	if shift == nil {
		shift = func(l int) (int, bool) { return l, true }
	}
	_ = ast.Walk(p.doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || n.Type() != ast.TypeBlock || n.Kind() == ast.KindDocument {
			return ast.WalkContinue, nil
		}
		l, keep := shift(p.st.blockLine(n))
		if !keep {
			return ast.WalkSkipChildren, nil
		}
		var path []string
		for a := n; a != nil; a = a.Parent() {
			path = append(path, a.Kind().String())
		}
		var text []byte
		for i := 0; i < n.Lines().Len(); i++ {
			seg := n.Lines().At(i)
			if seg.Start < 0 || seg.Start > seg.Stop || seg.Stop > len(p.st.src) {
				text = append(text, 0) // 원문 범위가 아닌 조각(퍼징 발견 — `*\n\t$$0`) — 읽지 않고 자리만 남긴다
				continue
			}
			text = append(text, seg.Value(p.st.src)...)
		}
		// 끝 줄바꿈 없던 마지막 줄 뒤에 넣으면 그 줄 글에 줄바꿈 하나가 붙는다 — 구조 변화가 아니다
		out = append(out, fmt.Sprintf("%s@%d %q", strings.Join(path, "<"), l, bytes.TrimSuffix(text, []byte("\n"))))
		return ast.WalkContinue, nil
	})
	for _, sp := range p.spans {
		s, ok1 := shift(sp.start)
		e, ok2 := shift(sp.end)
		ok = ok && ok1 && ok2
		out = append(out, fmt.Sprintf("각주 %d-%d %v %v", s, e, sp.topLevel, sp.parent))
	}
	return out, ok
}
