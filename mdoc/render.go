package mdoc

import (
	"bytes"
	"github.com/alecthomas/chroma/v2"
	"hash/fnv"
	"io"
	"strconv"
	"strings"
	"unicode/utf16"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// htmlRenderer 는 goldmark 기본 렌더러 위에 덮어 쓰는 부분이다.
// 보안에 걸리는 노드(raw HTML·링크·이미지·제목 속성·코드 언어)는 모두 여기서 그린다.
type htmlRenderer struct{ st *state }

func (r *htmlRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHeading, r.heading)
	reg.Register(ast.KindFencedCodeBlock, r.codeBlock)
	reg.Register(ast.KindCodeBlock, r.codeBlock)
	reg.Register(ast.KindHTMLBlock, r.htmlBlock)
	reg.Register(ast.KindRawHTML, r.rawHTML)
	reg.Register(ast.KindLink, r.link)
	reg.Register(ast.KindAutoLink, r.autoLink)
	reg.Register(ast.KindImage, r.image)
	reg.Register(KindContainer, r.container)
	reg.Register(extast.KindFootnoteList, r.footnoteList)
	reg.Register(KindAlert, r.alert)
	reg.Register(KindFigure, r.figure)
	reg.Register(KindFigureCaption, r.figureCaption)
	reg.Register(KindPageBreak, r.pageBreak)
	reg.Register(KindTOCNav, r.tocNav)
	reg.Register(KindTodosNav, r.todosNav)
	reg.Register(KindTableCaption, r.tableCaption)
	reg.Register(KindChart, r.chart)
	reg.Register(KindPause, r.pause)
	reg.Register(KindMathInline, r.mathInline)
	reg.Register(KindMathBlock, r.mathBlock)
	reg.Register(KindKPIList, r.kpiList)
	reg.Register(KindKPIItem, r.kpiItem)
}

// attr 는 이스케이프한 속성 하나를 쓴다.
func attr(w util.BufWriter, name, value string) {
	_, _ = w.WriteString(" " + name + `="`)
	_, _ = w.Write(util.EscapeHTML([]byte(value)))
	_ = w.WriteByte('"')
}

// dataAttrs 는 노드에 붙인 data-* (data-line) 를 쓴다. 이 패키지가 붙인 것만 있다.
func dataAttrs(w util.BufWriter, n ast.Node) {
	html.RenderAttributes(w, n, util.NewBytesFilter())
}

func (r *htmlRenderer) heading(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Heading)
	tag := "h" + string(rune('0'+n.Level))
	info := r.st.headings[n]
	if !entering {
		_, _ = w.WriteString("</" + tag + ">\n")
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString("<" + tag)
	if info != nil {
		attr(w, "id", info.id)
		if len(info.classes) > 0 {
			attr(w, "class", strings.Join(info.classes, " "))
		}
		for _, kv := range info.data {
			attr(w, "data-"+kv[0], kv[1])
		}
	}
	dataAttrs(w, n)
	_ = w.WriteByte('>')
	if info != nil && info.number != "" {
		_, _ = w.WriteString(`<span class="header-section-number">` + info.number + `</span> `)
	}
	return ast.WalkContinue, nil
}

const (
	maxHighlightBlockBytes = 64 << 10
	maxHighlightBlockLines = 1000
	maxHighlightDocBytes   = 256 << 10
)

var highlightLanguages = func() map[string]bool {
	names := make(map[string]bool)
	for _, name := range lexers.Names(true) {
		names[strings.ToLower(name)] = true
	}
	names["yml"] = true // YAML 파일 확장자
	return names
}()

// codeBlock 은 <pre><code class="language-…"> 다. 언어 이름은 글자·숫자·+-_. 만 남긴다.
func (r *htmlRenderer) codeBlock(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	blockIndex := -1
	var info codeInfo // 펜스 info string — 언어 낱말과 파일명 머리 띠 표기(codetitle.go)
	if fc, ok := node.(*ast.FencedCodeBlock); ok {
		info = parseCodeInfo(fenceInfo(fc, src))
		if entering {
			// 그림·수식·강조 상한과 무관하게 파서가 받아들인 모든 펜스를 기록한다.
			blockIndex = len(r.st.editorTokens)
			r.st.editorTokens = append(r.st.editorTokens, editorCodeLocation(fc, src, r.st.lines, r.st.fenceCloses[fc]))
		}
	}
	if fc, ok := node.(*ast.FencedCodeBlock); ok && info.lang == "math" {
		// ```math 는 GitHub 처럼 블록 수식이다. 그리지 못하면 보통의 코드 블록으로 둔다(원문이 보인다 — 파일명 띠도 그린다)
		if !entering && r.st.mathDone[fc] {
			return ast.WalkContinue, nil
		}
		if entering {
			if out, ok := r.st.math(rawLines(fc, src), true, r.st.lines.line(fc.Pos())); ok {
				r.st.mathDone[fc] = true
				r.st.mathOut[fc] = out
				r.mathDiv(w, fc, out)
				return ast.WalkSkipChildren, nil
			}
		}
	}
	if fc, ok := node.(*ast.FencedCodeBlock); ok && info.lang == "mermaid" {
		// ```mermaid 의 흐름도는 그림이다(diagram.go). 그리지 못하면 보통의 코드 블록으로 둔다(원문이 보인다 — 파일명 띠도 그린다)
		if !entering && r.st.diagramDone[fc] {
			return ast.WalkContinue, nil
		}
		if entering {
			if out, ok := r.st.diagram(fc, src); ok {
				r.st.diagramDone[fc] = true
				_, _ = w.WriteString(`<figure class="diagram"`)
				dataAttrs(w, fc)
				// 그림 자리(itda-hyve#475): 펜스 줄 ratio·width·height — 그림이 성공했을 때만 값과 경고를 낸다(diagramsize.go)
				sz, swarns := parseDiagramSize(string(fenceInfo(fc, src)))
				sz.attrs(w)
				for _, msg := range swarns {
					r.st.warn(r.st.lines.line(fc.Pos()), WarnDiagram, msg)
				}
				_, _ = w.WriteString(">\n" + out + "\n</figure>\n")
				return ast.WalkSkipChildren, nil
			}
		}
	}
	if !entering {
		_, _ = w.WriteString("</code></pre>\n")
		return ast.WalkContinue, nil
	}
	// 언어가 없고 파일명이 있으면 확장자·이름으로 짐작한다(guessed). 짐작 포함 언어는 아래 강조 허용 목록·상한을 그대로 통과한다.
	lang := cleanLang([]byte(info.lang))
	if lang == "" && info.title != "" {
		lang = guessCodeLang(info.title)
	}
	_, _ = w.WriteString("<pre")
	dataAttrs(w, node)
	if info.title != "" {
		attr(w, "data-title", info.title) // 머리 띠 파일명 — 화면(base.css ::before)이 그리는 표식이다
	}
	if fc, ok := node.(*ast.FencedCodeBlock); ok {
		// 단 나누기(itda-hyve#487): 펜스 줄 cols=2|3 — 화면(codelines.js·base.css)이 N단으로 흘린다(codecols.go)
		cols, cwarns := parseCodeCols(string(fenceInfo(fc, src)))
		if cols != 0 {
			attr(w, "data-cols", strconv.Itoa(cols))
		}
		for _, msg := range cwarns {
			r.st.warn(r.st.lines.line(fc.Pos()), WarnBadAttribute, msg)
		}
	}
	if hl := r.codeHL(node, info); hl != "" {
		attr(w, "data-hl", hl) // 줄 강조(itda-hyve#479) — 화면(codelines.js)이 줄 span 에 hl 클래스를 단다
	}
	_, _ = w.WriteString("><code")
	if lang != "" {
		attr(w, "class", "language-"+lang)
	}
	_ = w.WriteByte('>')
	lines := node.Lines()
	if fc, ok := node.(*ast.FencedCodeBlock); ok && lang != "" {
		if lang != "math" && lang != "mermaid" && len(lang) <= 64 && highlightLanguages[strings.ToLower(lang)] && lines.Len() <= maxHighlightBlockLines && r.st.highlightBytes < maxHighlightDocBytes {
			var code bytes.Buffer
			for i := 0; i < lines.Len() && code.Len() <= maxHighlightBlockBytes; i++ {
				_, _ = code.Write(segValue(lines.At(i), src))
			}
			if code.Len() <= maxHighlightBlockBytes && code.Len() <= maxHighlightDocBytes-r.st.highlightBytes {
				if lexer := lexers.Get(lang); lexer != nil {
					iter, err := lexer.Tokenise(nil, code.String())
					if err == nil {
						var out bytes.Buffer
						formatter := chromahtml.New(chromahtml.WithClasses(true), chromahtml.ClassPrefix("hl-"), chromahtml.PreventSurroundingPre(true))
						tokens := iter.Tokens()
						if formatter.Format(&out, styles.Fallback, chroma.Literator(tokens...)) == nil {
							if mapped, ok := editorRawTokens(fc, src, r.st.lines, tokens); ok {
								r.st.editorTokens[blockIndex].Tokens = mapped
							}
							r.st.highlightBytes += code.Len()
							_, _ = io.Copy(w, &out)
							return ast.WalkContinue, nil
						}
					}
				}
			}
		}
	}
	for i := 0; i < lines.Len(); i++ {
		_, _ = w.Write(util.EscapeHTML(segValue(lines.At(i), src)))
	}
	return ast.WalkContinue, nil
}

func cleanLang(b []byte) string {
	return strings.Map(func(r rune) rune {
		if r < 0x80 && (isASCIILetter(byte(r)) || (r >= '0' && r <= '9') || strings.ContainsRune("+-_.#", r)) {
			return r
		}
		return -1
	}, string(b))
}

// htmlBlock 은 변환에서 걸러지고 남은 것(<br> 만 있는 블록)만 쓴다. 나머지는 이미 지웠지만 혹시 남아도 쓰지 않는다.
func (r *htmlRenderer) htmlBlock(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		if v, ok := r.st.brBlocks[node.(*ast.HTMLBlock)]; ok {
			_, _ = w.WriteString(v + "\n")
		}
	}
	return ast.WalkSkipChildren, nil
}

func (r *htmlRenderer) rawHTML(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		if v, ok := r.st.rawOK[node.(*ast.RawHTML)]; ok {
			_, _ = w.WriteString(v)
		}
	}
	return ast.WalkSkipChildren, nil
}

// link 는 막은 주소면 <a> 없이 글자만 쓴다.
func (r *htmlRenderer) link(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Link)
	info := r.st.links[n]
	if info.kind == urlBlocked {
		return ast.WalkContinue, nil
	}
	if !entering {
		_, _ = w.WriteString("</a>")
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString(`<a href="`)
	_, _ = w.Write(util.EscapeHTML(util.URLEscape([]byte(info.href), true)))
	_ = w.WriteByte('"')
	if n.Title != nil {
		attr(w, "title", inlineValue(n.Title))
	}
	_ = w.WriteByte('>')
	return ast.WalkContinue, nil
}

func (r *htmlRenderer) autoLink(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.AutoLink)
	info := r.st.links[n]
	label := util.EscapeHTML(n.Label(src))
	if info.kind == urlBlocked {
		_, _ = w.Write(label)
		return ast.WalkSkipChildren, nil
	}
	_, _ = w.WriteString(`<a href="`)
	_, _ = w.Write(util.EscapeHTML(util.URLEscape([]byte(info.href), false)))
	_, _ = w.WriteString(`">`)
	_, _ = w.Write(label)
	_, _ = w.WriteString("</a>")
	return ast.WalkSkipChildren, nil
}

// classes 는 슬라이드 그림 배치(itda-hyve#550)의 class 다 — img-fit·img-left·img-right, 그리고 img-fill. 문서(A4·이어보기)에는 이 class 의
// 규칙이 없어 보통 그림이다(base.css 는 .page.slide 안에서만 고른다). style 은 여전히 width 에서만 나온다.
func (info *imageInfo) classes() string {
	if info.place == "" {
		return ""
	}
	if info.fill {
		return "img-" + info.place + " img-fill"
	}
	return "img-" + info.place
}

// image 는 원격 주소를 기본으로 data-remote-src 로 옮기고, 막은 주소는 src 없이 쓴다.
func (r *htmlRenderer) image(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Image)
	info := r.st.images[n]
	if info == nil {
		info = &imageInfo{kind: urlBlocked}
	}
	dest := util.EscapeHTML(util.URLEscape(n.Destination, true))
	if info.yt != nil {
		r.ytOpen(w, info)
	}
	_, _ = w.WriteString("<img")
	switch {
	case info.yt != nil:
		r.ytPoster(w, info.yt)
	case info.kind == urlOK, info.kind == urlRemote && r.st.opts.AllowRemoteImages:
		_, _ = w.WriteString(` src="`)
		if info.kind == urlOK && r.st.opts.LocalImagePrefix != "" && !isDataURL(string(n.Destination)) {
			_, _ = w.Write(util.EscapeHTML([]byte(r.st.opts.LocalImagePrefix)))
		}
		_, _ = w.Write(dest)
		_ = w.WriteByte('"')
	case info.kind == urlRemote:
		_, _ = w.WriteString(` data-remote-src="`)
		_, _ = w.Write(dest)
		_ = w.WriteByte('"')
	}
	if info.noAlt {
		attr(w, "alt", "") // Deckset 배치 지시(itda-hyve#550) — 대체 글이 아니다
	} else {
		attr(w, "alt", plainText(n, src))
	}
	if cls := info.classes(); cls != "" {
		attr(w, "class", cls)
	}
	if info.split != "" {
		attr(w, "data-split", info.split)
	}
	if _, inFigure := n.Parent().(*Figure); n.Title != nil && !inFigure {
		attr(w, "title", inlineValue(n.Title))
	}
	if info.width != "" {
		// cssLength 를 통과한 숫자+단위뿐이다
		attr(w, "style", "width:"+info.width)
	}
	_ = w.WriteByte('>')
	if info.yt != nil {
		_, _ = w.WriteString("</a>")
	}
	return ast.WalkSkipChildren, nil
}

// ytOpen 은 YouTube 블록(youtube.go)의 여는 링크다: <a class="yt" href="정규화한 watch 주소" data-yt-id data-yt-start>. 대표 그림이
// 불러오지 않는 원격 그림이면 class 에 yt-ph(자리표시 — base.css 가 ▶·글자·주소를 그린다)를 더한다. 배치 class·data-split·width 는
// 안의 img 에 보통 그림과 똑같이 단다(슬라이드 배치 itda-hyve#550 의 규칙이 그대로 고른다). 스크립트·iframe 은 내지 않는다 — 플레이어는 화면 JS 몫이다.
//
// 자리표시는 안의 img 를 그리지 않고 링크가 상자를 그리므로, 배치 class 가 없는 그림의 width 를 링크에도 같은 값으로 단다(style 은 여전히
// width 에서만 — 슬라이드 배치 그림(.fit·.left·.right)은 원래 width 를 쓰지 않는다).
func (r *htmlRenderer) ytOpen(w util.BufWriter, info *imageInfo) {
	v := info.yt
	ph := v.poster == "" && !r.st.opts.AllowRemoteImages
	cls := "yt"
	if ph {
		cls = "yt yt-ph"
	}
	_, _ = w.WriteString(`<a class="` + cls + `"`)
	attr(w, "href", v.watchURL())
	attr(w, "data-yt-id", v.id)
	if v.start > 0 {
		attr(w, "data-yt-start", strconv.Itoa(v.start))
	}
	if ph && info.width != "" && info.place == "" {
		attr(w, "style", "width:"+info.width) // cssLength 를 통과한 숫자+단위뿐이다
	}
	_ = w.WriteByte('>')
}

// ytPoster 는 YouTube 블록 대표 그림의 src 다: poster= 가 있으면 로컬 그림 규칙(LocalImagePrefix — data: 는 그대로), 없으면 원격 그림을
// 켠 문서에서만 i.ytimg.com 대표 그림, 끈 문서에서는 data-remote-src(불러오지 않음)다.
func (r *htmlRenderer) ytPoster(w util.BufWriter, v *ytVideo) {
	switch {
	case v.poster != "":
		_, _ = w.WriteString(` src="`)
		if r.st.opts.LocalImagePrefix != "" && !isDataURL(v.poster) {
			_, _ = w.Write(util.EscapeHTML([]byte(r.st.opts.LocalImagePrefix)))
		}
		_, _ = w.Write(util.EscapeHTML(util.URLEscape([]byte(v.poster), true)))
		_ = w.WriteByte('"')
	case r.st.opts.AllowRemoteImages:
		attr(w, "src", v.thumbURL())
	default:
		attr(w, "data-remote-src", v.thumbURL())
	}
}

func (r *htmlRenderer) container(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*Container)
	if !entering {
		_, _ = w.WriteString("</div>\n")
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString("<div")
	if classes := n.classes(); len(classes) > 0 {
		attr(w, "class", strings.Join(classes, " "))
	}
	if n.Attrs.ID != "" {
		attr(w, "id", n.Attrs.ID)
	}
	for _, kv := range n.Attrs.Data {
		if kv[0] == "line" {
			continue // data-line 은 원문 줄(아래 dataAttrs)이다 — 차트의 line="계열" 이 표로 남을 때 겹치지 않게
		}
		attr(w, "data-"+kv[0], kv[1])
	}
	if n.Width != "" {
		attr(w, "style", "width:"+n.Width)
	}
	dataAttrs(w, n)
	_, _ = w.WriteString(">\n")
	return ast.WalkContinue, nil
}

// footnoteList 는 goldmark 각주 목록과 같게 그리되, 비었으면(DropNotes 가 노트 전용 각주를 다 뺀 자리 — transform.go
// pruneFootnotes) 가로줄·목록 없이 빈 div 만 그린다.
func (r *htmlRenderer) footnoteList(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	empty := node.FirstChild() == nil
	if !entering {
		if !empty {
			_, _ = w.WriteString("</ol>\n")
		}
		_, _ = w.WriteString("</div>\n")
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString(`<div class="footnotes" role="doc-endnotes"`)
	html.RenderAttributes(w, node, html.GlobalAttributeFilter)
	_ = w.WriteByte('>')
	if !empty {
		_, _ = w.WriteString("\n<hr>\n<ol")
		// 각주 번호도 본문 순서 목록처럼 자릿수 class 를 단다(itda-hyve#575 — transform.go olDigits 와 같은 규칙, 각주는 1 부터 차례다).
		// 각주 수가 아홉 이하면 class 가 없어 모양이 그대로다. 들여쓰기는 base.css .footnotes ol.ol-dN
		n := 0
		for c := node.FirstChild(); c != nil; c = c.NextSibling() {
			if _, ok := c.(*extast.Footnote); ok {
				n++
			}
		}
		if d := len(strconv.Itoa(n)); d >= 2 {
			attr(w, "class", "ol-d"+strconv.Itoa(min(d, 9)))
		}
		_, _ = w.WriteString(">\n")
	}
	return ast.WalkContinue, nil
}

// 알림 제목. GitHub 은 영어 고정이고, 여기서는 lang 에 따른다. lang 이 없으면 한국어다(이 앱의 기본 독자).
var alertTitles = map[string][2]string{
	"note":      {"참고", "Note"},
	"tip":       {"팁", "Tip"},
	"important": {"중요", "Important"},
	"warning":   {"주의", "Warning"},
	"caution":   {"경고", "Caution"},
	"todo":      {"할 일", "Todo"},
}

func (r *htmlRenderer) alertTitle(typ string) string {
	t := alertTitles[typ]
	if r.korean() {
		return t[0]
	}
	return t[1]
}

// korean 은 문서 lang 이 한국어(또는 없음)인지다 — 알림 제목·할 일 모음의 빈 줄 글이 따른다.
func (r *htmlRenderer) korean() bool {
	lang := strings.ToLower(r.st.meta.Lang)
	return lang == "" || lang == "ko" || strings.HasPrefix(lang, "ko-")
}

func (r *htmlRenderer) alert(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*Alert)
	if !entering {
		_, _ = w.WriteString("</div>\n")
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString(`<div class="markdown-alert markdown-alert-` + n.AlertType + `"`)
	if id := r.st.todoID(n); id != "" {
		attr(w, "id", id) // 할 일 모음이 가리키는 자리(todos.go)
	}
	dataAttrs(w, n)
	_, _ = w.WriteString(">\n" + `<p class="markdown-alert-title">` + r.alertTitle(n.AlertType) + "</p>\n")
	return ast.WalkContinue, nil
}

func (r *htmlRenderer) figure(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*Figure)
	if entering {
		_, _ = w.WriteString("<figure")
		dataAttrs(w, n)
		_, _ = w.WriteString(">\n")
		return ast.WalkContinue, nil
	}
	if n.Caption != "" && !hasChild[*FigureCaption](n) {
		// Figure: 문단 캡션이 있으면 그쪽만 낸다(figureCaption 이 이미 그렸다)
		_, _ = w.WriteString("\n<figcaption>")
		_, _ = w.Write(util.EscapeHTML([]byte(n.Caption)))
		_, _ = w.WriteString("</figcaption>")
	}
	_, _ = w.WriteString("\n</figure>\n")
	return ast.WalkContinue, nil
}

// figureCaption 은 Figure: 문단 캡션이다. 자식 인라인을 살려 그리는 것은 tableCaption 과 같다.
// 여는 쪽에서 줄을 바꾼다(이미지와 같은 줄이 되지 않게) — 닫는 쪽의 줄바꿈은 figure 가 낸다.
func (r *htmlRenderer) figureCaption(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString("\n<figcaption>")
	} else {
		_, _ = w.WriteString("</figcaption>")
	}
	return ast.WalkContinue, nil
}

func (r *htmlRenderer) pageBreak(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString(`<div class="page-break"`)
		dataAttrs(w, node)
		_, _ = w.WriteString("></div>\n")
	}
	return ast.WalkSkipChildren, nil
}

func (r *htmlRenderer) pause(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString(`<div class="pause"`)
		dataAttrs(w, node)
		_, _ = w.WriteString("></div>\n")
	}
	return ast.WalkSkipChildren, nil
}

func (r *htmlRenderer) tocNav(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		var attrs bytes.Buffer
		if v, ok := node.AttributeString("data-line"); ok {
			attrs.WriteString(` data-line="`)
			attrs.Write(util.EscapeHTML(v.([]byte)))
			attrs.WriteByte('"')
		}
		_, _ = w.WriteString(renderTOC(r.st.toc, attrs.String()))
	}
	return ast.WalkSkipChildren, nil
}

func (r *htmlRenderer) tableCaption(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString("<caption>")
	} else {
		_, _ = w.WriteString("</caption>\n")
	}
	return ast.WalkContinue, nil
}

// RenderTOC 는 목차를 <nav class="toc" role="doc-toc"> 로 그린다. [TOC] 가 없는 문서의 목차를
// 다음 단계가 표지 뒤 같은 자리에 넣을 때 쓴다. 비어 있으면 "" 다.
func RenderTOC(items []TOCItem) string {
	return renderTOC(items, "")
}

func renderTOC(items []TOCItem, extra string) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<nav class="toc" role="doc-toc"` + extra + ">\n")
	writeTOCList(&b, items)
	b.WriteString("</nav>\n")
	return b.String()
}

func writeTOCList(b *strings.Builder, items []TOCItem) {
	b.WriteString("<ul>\n")
	for _, it := range items {
		b.WriteString(`<li><a href="#`)
		// id 는 slug 나 검사한 글자뿐이다. 한글을 퍼센트 인코딩하지 않아야 원문에서 읽힌다
		b.Write(util.EscapeHTML([]byte(it.ID)))
		b.WriteString(`">`)
		if it.Number != "" {
			b.WriteString(`<span class="toc-section-number">` + it.Number + `</span> `)
		}
		b.Write(util.EscapeHTML([]byte(it.Text)))
		b.WriteString("</a>")
		if len(it.Children) > 0 {
			b.WriteString("\n")
			writeTOCList(b, it.Children)
		}
		b.WriteString("</li>\n")
	}
	b.WriteString("</ul>\n")
}

// inlineValue 는 title 처럼 goldmark 가 원문 그대로 둔 값에서 백슬래시 이스케이프와 엔티티를 푼다
// (기본 렌더러의 Writer.Write 와 같은 처리). 이스케이프는 쓰는 쪽이 한다.
func inlineValue(b []byte) string {
	b = util.UnescapePunctuations(b)
	b = util.ResolveNumericReferences(b)
	b = util.ResolveEntityNames(b)
	return string(b)
}

// EditorCodeBlock 은 원문 줄 범위와 펜스 내용에 대응한다. 길이와 열은 JS 문자열(UTF-16) 단위다.
type EditorCodeBlock struct {
	Kind    string            `json:"kind,omitempty"` // "front" 또는 "fence"; 들여 쓴 코드 블록은 상자가 아니다
	Line    int               `json:"line"`
	End     int               `json:"end"`
	Close   int               `json:"close,omitempty"`
	Hash    uint32            `json:"hash"` // 여는 줄부터 End 줄까지의 원문 해시
	Content []EditorCodeLine  `json:"content"`
	Tokens  []EditorCodeToken `json:"tokens"`
}
type EditorCodeLine struct {
	Line int `json:"line"`
	Col  int `json:"col"`
}
type EditorCodeToken struct {
	Kind   string `json:"kind"`
	Length int    `json:"length"`
}

func editorCodeLocation(fc *ast.FencedCodeBlock, src []byte, starts lineIndex, closeLine int) EditorCodeBlock {
	line := starts.line(fc.Pos())
	block := EditorCodeBlock{Kind: "fence", Line: line, End: line}
	for i := 0; i < fc.Lines().Len(); i++ {
		seg := fc.Lines().At(i)
		n := starts.line(seg.Start)
		start := seg.Start
		if seg.Padding > 0 && start > starts[n-1] && src[start-1] == '\t' {
			start-- // Goldmark 가 탭 일부를 가상 공백으로 바꾼 자리
		}
		col := len(utf16.Encode([]rune(string(src[starts[n-1]:start]))))
		block.Content = append(block.Content, EditorCodeLine{Line: n, Col: col})
		block.End = n
	}
	if closeLine > 0 {
		block.Close = closeLine
		block.End = closeLine
	}
	start := starts[line-1]
	stop := len(src)
	if block.End < len(starts) {
		stop = starts[block.End]
	}
	h := fnv.New32a()
	_, _ = h.Write(src[start:stop])
	block.Hash = h.Sum32()
	return block
}

// editorRawTokens 는 Goldmark 의 가상 코드 글자를 textarea 원문 글자에 대응시킨다.
// 일부만 소비된 탭의 가상 공백은 원문 탭 한 글자에 대응하고 EOF 가상 개행은 버린다.
// 대응이 불분명하면 색 토큰을 주지 않아 표시층이 기본 코드색으로 그리게 한다.
func editorRawTokens(fc *ast.FencedCodeBlock, src []byte, starts lineIndex, tokens []chroma.Token) ([]EditorCodeToken, bool) {
	var kinds []string
	for _, token := range tokens {
		kind := editorTokenKind(token.Type)
		for _, r := range token.Value {
			for range utf16.Encode([]rune{r}) {
				kinds = append(kinds, kind)
			}
		}
	}
	var mapped []EditorCodeToken
	add := func(kind string, n int) {
		if n == 0 {
			return
		}
		if len(mapped) > 0 && mapped[len(mapped)-1].Kind == kind {
			mapped[len(mapped)-1].Length += n
		} else {
			mapped = append(mapped, EditorCodeToken{Kind: kind, Length: n})
		}
	}
	at := 0
	for i := 0; i < fc.Lines().Len(); i++ {
		seg := fc.Lines().At(i)
		virtual := segValue(seg, src)
		virtualLen := len(utf16.Encode([]rune(string(virtual))))
		if at+virtualLen > len(kinds) {
			return nil, false
		}
		line := starts.line(seg.Start)
		if line < 1 {
			return nil, false
		}
		end := len(src)
		if line < len(starts) {
			end = starts[line]
		}
		start := seg.Start
		if seg.Padding > 0 {
			if start <= starts[line-1] || src[start-1] != '\t' || seg.Padding > virtualLen {
				return nil, false
			}
			start--
			add(kinds[at], 1)
		}
		if seg.Stop > end || start > seg.Stop {
			return nil, false
		}
		// Segment 는 보통 원문 줄 끝까지다. 다르면 원문 나머지에 토큰을 안전하게 대응할 수 없다.
		if seg.Stop != end && !(seg.ForceNewline && seg.Stop == len(src)) {
			return nil, false
		}
		vpos := at + seg.Padding
		for _, r := range string(src[seg.Start:seg.Stop]) {
			n := len(utf16.Encode([]rune{r}))
			if vpos+n > at+virtualLen {
				return nil, false
			}
			for j := 0; j < n; j++ {
				add(kinds[vpos+j], 1)
			}
			vpos += n
		}
		if vpos != at+virtualLen && !(seg.ForceNewline && vpos+1 == at+virtualLen && end == len(src)) {
			return nil, false
		}
		at += virtualLen
	}
	if at != len(kinds) {
		return nil, false
	}
	return mapped, true
}

func editorTokenKind(t chroma.TokenType) string {
	for t != 0 {
		if cls, ok := chroma.StandardTypes[t]; ok {
			return "hl-" + cls
		}
		t = t.Parent()
	}
	return ""
}
