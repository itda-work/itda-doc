package mdoc

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// 블록 중첩 깊이 상한(설계 1.10, #126) — 넘는 문서는 통째 안전 모드.
//
// goldmark 은 줄마다 열린 블록을 모두 돌고(줄당 O(깊이)) 한 줄에 n 겹을 열 때도 O(n²) 이다 — `>>>>…` 한 줄 128KB 3.8초,
// `:::a` 줄 반복 17초(순수 goldmark 도 같다). 실제 파서가 컨테이너 블록(인용·목록·:::·각주 정의·정의 목록 — 목록 항목·정의 설명은
// 목록·정의 목록과 한 겹)을 MaxNestingDepth+1 번째 겹으로 받아들이는 순간 그 파싱 호출을 통째로 버리고(nestingAbort), 문서 전체를
// 원문 글자로 보인다(renderRaw, 경고 too-complex 는 그 겹을 연 줄에). 넘지 않으면 파싱을 끝까지 하고 결과는 상한이 없을 때와 같다 —
// 막거나 건너뛰는 블록이 없다. 중단 전까지 열린 블록은 64겹 이하라 줄당 비용이 상수다.
//
// 깊이는 원문을 따로 훑어 추정하지 않고(#126 R3~R5 — 블록 문맥을 다시 구현하는 추정은 어긋나는 자리가 계속 나왔다) goldmark 이 실제로
// 받아들인 블록의 조상으로 잰다. 그래서 코드·HTML·수식 안의 글자, 인용 뒤 공백·탭 들여쓰기 같은 문법 차이는 판정에 들어오지 않는다.
//
// 받아들임의 근거(goldmark v1.8.6 parser/parser.go openBlocks 995~1038): bp.Open 이 돌려준 노드를 버리고 다시 고르는 길은
// state&RequireParagraph 분기(1012~1026 — 앞 문단이 표 등으로 바뀌면 이 파서는 실패로 본다) 하나뿐이고, 그 밖에는 바로 AppendChild 한다.
// 깊이를 세는 파서 가운데 RequireParagraph 를 돌려주는 것은 정의 목록의 첫 항목(extension/definition_list.go 62)뿐이다
// (인용·목록·각주 정의·우리 ::: 는 HasChildren/NoChildren 만 — TestNestingGoldmarkContract 가 소스를 다시 확인한다). 그래서 정의 목록은
// 그 목록이 받아들여진 뒤 같은 줄에서 바로 이어 여는 정의 설명(HasChildren 만)의 Open 에서 센다 — 정의 목록 Open 은 줄을 소비하지
// 않고, 받아들여지면 goldmark 이 그 안에서 같은 : 로 정의 설명을 연다(definition_list.go 의 두 Open).

// MaxNestingDepth 는 컨테이너 블록 중첩의 상한이다. 목록 항목·정의 설명은 목록·정의 목록과 한 겹으로 센다
// (`> - ` 반복은 인용과 목록을 합해 64겹). 사람이 쓰는 문서의 중첩은 몇 겹이다.
const MaxNestingDepth = 64

// nestingAbort 는 상한을 넘는 겹을 받아들인 순간 파싱을 멈추는 panic 값이다. Render 의 파싱 경계(parseGuarded)만 이 값을 잡는다.
type nestingAbort struct{ line int }

// counted 는 깊이에 들어가는 블록인지다.
func counted(n ast.Node) bool {
	switch n.(type) {
	case *ast.Blockquote, *ast.List, *Container, *extast.Footnote, *extast.DefinitionList:
		return true
	}
	return false
}

// nestingDepth 는 n 과 그 조상 가운데 깊이에 들어가는 블록의 수다. 상한에서 멈추므로 걷는 길이도 상한 남짓이다.
func nestingDepth(n ast.Node) int {
	d := 0
	for ; n != nil; n = n.Parent() {
		if counted(n) {
			d++
		}
	}
	return d
}

// nestingGate 는 깊이에 들어가는 블록을 여는 goldmark 블록 파서를 감싼다. 원래 Open 을 그대로 부르고, 받아들여질 노드가 상한을 넘는
// 겹이면 파싱을 멈춘다. 나머지(Continue·Close·Trigger …)는 그대로다.
type nestingGate struct {
	parser.BlockParser
	st *state
	// self 면 Open 이 돌려준 노드가 한 겹이다(인용·목록·:::·각주 정의). 거짓이면 노드는 세지 않고 부모(정의 목록)까지 센다 — 정의 설명
	self bool
}

func (g *nestingGate) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	node, state := g.BlockParser.Open(parent, reader, pc)
	if _, mark := node.(*closeMark); node == nil || mark || state&parser.RequireParagraph != 0 { // 닫는 줄 표시는 겹이 아니다
		return node, state
	}
	d := nestingDepth(parent)
	if g.self {
		d++
	}
	if d > g.st.nestingLimit {
		_, seg := reader.PeekLine()
		panic(nestingAbort{line: g.st.lines.line(seg.Start)})
	}
	return node, state
}

// nestingGates 는 깊이에 들어가는 블록 파서에 nestingGate 를 씌운다(확장이 넣은 것까지 — goldmark.New 뒤에 건다).
type nestingGates struct{ st *state }

func (o nestingGates) SetParserOption(c *parser.Config) {
	for i, v := range c.BlockParsers {
		bp, _ := v.Value.(parser.BlockParser)
		g := &nestingGate{BlockParser: bp, st: o.st, self: true}
		switch bp.(type) {
		case *containerParser:
		default:
			switch bp {
			case parser.NewBlockquoteParser(), parser.NewListParser(), extension.NewFootnoteBlockParser():
			case extension.NewDefinitionDescriptionParser():
				g.self = false
			default:
				continue // 목록 항목은 목록 바로 안에서만 열린다(목록이 센다). 나머지는 자식을 갖지 않는 블록이다
			}
		}
		c.BlockParsers[i].Value = g
		o.st.nestingGated++
	}
}

// parseGuarded 는 body 를 파싱한다. 상한을 넘는 겹을 받아들이면 over 에 그 줄(1부터)을 돌려주고 AST 는 버린다. 다른 panic 은 그대로 던진다.
func (st *state) parseGuarded(body []byte) (doc ast.Node, over int) {
	st.containers, st.passedBy = nil, nil
	defer func() {
		if r := recover(); r != nil {
			a, ok := r.(nestingAbort)
			if !ok {
				panic(r)
			}
			doc, over = nil, max(a.line, 1)
		}
	}()
	return st.md.Parser().Parse(textReader(body)), 0
}

// frontmatterLines 는 본문에서 frontmatter 자리(빈 줄로 바꾼 줄)의 수다.
func frontmatterLines(src []byte, meta Meta) int {
	if !meta.Present {
		return 0
	}
	if _, end, ok := splitFrontmatter(src); ok {
		return bytes.Count(src[:end], []byte("\n"))
	}
	return 0
}

// renderRaw 는 안전 모드의 결과다 — 본문 전체를 이스케이프한 원문 글자로 보인다. 줄마다 data-line(원문 줄)을 붙인 span 이고,
// pre 의 data-line 은 첫 내용 줄이다(쪽 나눔 pager.js splitPre 가 조각마다 이어 단다). frontmatter 자리 skip 줄(본문에서는 빈 줄)은 뺀다.
// 경고는 frontmatter 의 것과 too-complex 하나다(중단한 파싱의 경고는 버린다).
func renderRaw(meta Meta, body []byte, warns []Warning, overLine, skip int) Result {
	var b strings.Builder
	lines := bytes.Split(body, []byte("\n"))
	if n := len(lines); n > 0 && len(lines[n-1]) == 0 {
		lines = lines[:n-1]
	}
	first := min(skip, len(lines))
	b.WriteString(`<pre class="raw-source" data-line="` + strconv.Itoa(first+1) + `"><code>`)
	for i := first; i < len(lines); i++ {
		b.WriteString(`<span data-line="` + strconv.Itoa(i+1) + `">`)
		b.Write(util.EscapeHTML(bytes.ReplaceAll(lines[i], []byte{0}, []byte("�"))))
		b.WriteString("</span>\n")
	}
	b.WriteString("</code></pre>\n")
	warns = append(warns[:len(warns):len(warns)], Warning{Line: overLine, Code: WarnTooComplex,
		Message: fmt.Sprintf("블록 중첩(인용·목록·::: 등)이 %d 겹을 넘어 문서 전체를 원문 글자로 보임", MaxNestingDepth)})
	return Result{Meta: meta, HTML: b.String(), Warnings: warns}
}

// WithheldNotice 는 노트를 빼야 하는 렌더(Options.DropNotes)가 블록 중첩 상한을 넘었을 때 본문 대신 내는 안내다.
const WithheldNotice = "이 문서는 구조가 너무 깊어 공유 화면에 보일 수 없습니다."

// withheldRaw 는 DropNotes 의 안전 모드 결과다. renderRaw 는 원문 글자 그대로라 노트를 뺄 수 없으므로 본문 대신 안내 한 줄만
// 낸다. 목차·슬라이드 단계는 비고, 경고는 frontmatter 의 것과 too-complex 하나다(원문 글을 싣지 않는다).
func withheldRaw(meta Meta, warns []Warning, overLine int) Result {
	warns = append(warns[:len(warns):len(warns)], Warning{Line: overLine, Code: WarnTooComplex,
		Message: fmt.Sprintf("블록 중첩(인용·목록·::: 등)이 %d 겹을 넘어 노트를 뺄 수 없음 — 본문 대신 안내만 보임", MaxNestingDepth)})
	return Result{Meta: meta, HTML: `<p class="raw-withheld">` + WithheldNotice + "</p>\n", Warnings: warns}
}
