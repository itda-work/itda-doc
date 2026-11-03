package mdoc

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// state 는 Render 한 번의 작업 공간이다. 파서·변환·렌더러가 경고와 판정을 여기 모은다.
// goldmark 인스턴스도 호출마다 만든다 — 파서·렌더러가 이 state 를 쥐어야 해서다(문서 창의 디바운스 렌더에 충분히 싸다).
type state struct {
	src   []byte
	lines lineIndex
	meta  Meta
	opts  Options
	md    goldmark.Markdown

	warns     []Warning
	toc       []TOCItem
	tocPlaced bool

	headings map[*ast.Heading]*headingInfo
	images   map[*ast.Image]*imageInfo
	links    map[ast.Node]linkInfo     // Link·AutoLink
	rawOK    map[*ast.RawHTML]string   // 남긴 인라인 HTML → 정규화한 태그
	brBlocks map[*ast.HTMLBlock]string // <br> 만 있는 HTML 블록

	editorTokens   []EditorCodeBlock
	fenceCloses    map[*ast.FencedCodeBlock]int // Goldmark 가 소비한 닫는 줄만
	highlightBytes int                          // 이번 렌더에서 구문 강조한 코드 원문 바이트 합
	mathCount      int                          // 그린(그리려 한) 수식 수 — MaxMathPerDoc
	mathDone       map[ast.Node]bool            // 블록 수식으로 그린 ```math 코드 블록
	// 그린 수식의 MathML(MathInline·MathBlock·```math 코드 블록 → 결과)과 그린 그림(```mermaid → SVG·캡션) — HTML 에는 쓰지 않고
	// 문서 모델(doc.go Doc.Math·Doc.Diagram)이 내준다
	mathOut     map[ast.Node]string
	diagramPics map[ast.Node]DiagramPic
	// 펜스 바로 앞 Deckset 줄 강조 지시어(codehl.go) — 펜스 info 의 {…} 가 이긴다
	codeHL map[*ast.FencedCodeBlock]codeHLDirective
	// 그린(그리려 한) mermaid 그림 수 — MaxDiagramsPerDoc, 그림으로 그린 ```mermaid 코드 블록
	diagramCount int
	diagramBytes int  // 그린 SVG 의 합 — MaxDiagramBytesPerDoc
	diagramFull  bool // 합 상한에 닿음(경고는 한 번)
	diagramDone  map[ast.Node]bool
	// 글 안 수식 탐색 예산의 남은 양(문단마다) — math.go
	mathBudget map[ast.Node]int
	delims     map[ast.Node]int // 블록마다 읽은 강조·취소선 구분자 수 — cjk.go 의 상한
	closers    map[ast.Node]int // 블록마다 링크로 닫아 본 ] 의 수 — linkclose.go
	// 블록 중첩 상한 — nesting.go. nestingLimit 은 시험만 바꾼다(상한 없는 파서로 실제 깊이를 잴 때)
	nestingLimit int
	nestingGated int // 상한을 건 블록 파서 수(시험이 본다)
	// 열린 ::: 컨테이너(바깥부터)와, 닫는 줄을 안쪽에 넘긴 가장 안쪽 컨테이너·그 줄 — container.go
	containers []*Container
	passedBy   *Container
	passedLine int
}

func newState(src []byte, meta Meta, opts Options) *state {
	st := &state{
		src:         src,
		lines:       newLineIndex(src),
		meta:        meta,
		opts:        opts,
		headings:    map[*ast.Heading]*headingInfo{},
		images:      map[*ast.Image]*imageInfo{},
		links:       map[ast.Node]linkInfo{},
		rawOK:       map[*ast.RawHTML]string{},
		brBlocks:    map[*ast.HTMLBlock]string{},
		fenceCloses: map[*ast.FencedCodeBlock]int{},
		mathDone:    map[ast.Node]bool{},
		mathOut:     map[ast.Node]string{},
		diagramPics: map[ast.Node]DiagramPic{},
		codeHL:      map[*ast.FencedCodeBlock]codeHLDirective{},
		diagramDone: map[ast.Node]bool{},
		mathBudget:  map[ast.Node]int{},
		delims:      map[ast.Node]int{},
		closers:     map[ast.Node]int{},
		// 상한을 넘는 겹을 받아들이면 파싱을 멈춘다 — nesting.go
		nestingLimit: MaxNestingDepth,
	}
	st.md = goldmark.New(
		goldmark.WithExtensions(
			// 정렬은 style 대신 align 속성으로 낸다 — 결과 HTML 에 style 이 나오는 곳을 width 하나로 좁힌다
			extension.NewTable(extension.WithTableCellAlignMethod(extension.TableCellAlignAttribute)),
			extension.Strikethrough,
			extension.TaskList,
			extension.DefinitionList,
			extension.Footnote,
		),
		// 제목 id·속성은 goldmark 의 WithAutoHeadingID·WithHeadingAttribute 를 쓰지 않고 직접 한다
		// (GitHub 규칙의 id, {-}, 속성의 data-* 제한).
		goldmark.WithParserOptions(
			parser.WithBlockParsers(
				util.Prioritized(&containerParser{st: st}, 99),
				// fenced 코드(700)보다 앞 — 줄 머리의 $$ 는 코드 표기와 겹치지 않는다
				util.Prioritized(&mathBlockParser{st: st}, 98),
			),
			// 기본 강조·취소선 파서(500)보다 먼저 돈다 — CJK-friendly 판정(cjk.go)
			parser.WithInlineParsers(
				util.Prioritized(&linkCloserGuard{st: st}, 199), // 링크(200)보다 앞 — linkclose.go
				util.Prioritized(cjkEmphasisParser{st}, 499),
				util.Prioritized(cjkStrikeParser{st}, 499),
				// GFM 자동 링크. extension.Linkify 와 같은 파서·우선순위를 이차 시간을 막는 덮개로 감싼다(linkify.go)
				util.Prioritized(newLinkifyGuard(), 999),
				// 수식은 코드(`)·링크보다 뒤에 돈다 — 먼저 시작한 것이 이긴다. 강조 구분자보다 앞이라 수식 안의 _·* 는 TeX 다
				util.Prioritized(&mathInlineParser{st: st}, 498),
			),
		),
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(util.Prioritized(&htmlRenderer{st: st}, 100)),
		),
	)
	st.md.Parser().AddOptions(fenceCloserOption{st})
	// 확장이 넣은 것까지 블록 파서에 중첩 상한을 건다(nesting.go) — 확장은 goldmark.New 끝에서 파서를 넣는다
	st.md.Parser().AddOptions(nestingGates{st})
	return st
}

func textReader(src []byte) text.Reader { return text.NewReader(src) }

// innerContainer 는 가장 안쪽에 열린 ::: 컨테이너다.
func (st *state) innerContainer() *Container {
	if len(st.containers) == 0 {
		return nil
	}
	return st.containers[len(st.containers)-1]
}

// popContainer 는 닫힌 컨테이너를 스택에서 뺀다. goldmark 은 안쪽부터 닫으므로 늘 맨 위다.
func (st *state) popContainer(n *Container) {
	for i := len(st.containers) - 1; i >= 0; i-- {
		if st.containers[i] == n {
			st.containers = append(st.containers[:i], st.containers[i+1:]...)
			return
		}
	}
}

func (st *state) warn(line int, code, msg string) {
	st.warns = append(st.warns, Warning{Line: line, Code: code, Message: msg})
}
