package mdoc

import (
	"strings"

	"github.com/yuin/goldmark/ast"
)

// Doc 는 변환(transform)까지 마친 문서 모델이다(itda-work/itda-hyve#608). HTML 말고 다른 출력(docx 등)을 만드는 쪽이 문법을
// 새로 해석하지 않고 Render 와 같은 판정을 쓰도록, goldmark AST 와 변환·그리기가 노드마다 정한 값(제목 id·번호, 그림·링크 주소
// 판정, 남긴 인라인 HTML, 그린 수식의 MathML, 그린 mermaid 그림의 SVG)을 내준다. 안에 든 Result 는 같은 원문·Options 의 Render
// 결과와 같다 — Render 가 이 모델을 만든 뒤 Result 만 돌려준다(mdoc.go parseLimit).
//
// 조회 함수의 노드 인자는 이 Doc 의 Root 에서 얻은 노드여야 한다 — 다른 문서의 노드에 대한 결과는 정하지 않는다.
// AST 는 읽기만 한다. 고치면 내준 값과 어긋난다.
type Doc struct {
	Result

	root *ast.Document
	body []byte // frontmatter 자리를 빈 줄로 채운 원문 — AST 의 Segment 가 가리키는 바이트
	st   *state

	// 블록 중첩 상한을 넘어 AST 없이 원문 글자로 보인 문서(Root 가 nil)의 원문 줄 — rawSkip 은 frontmatter 자리 줄 수,
	// withheld 는 DropNotes 라 원문 대신 안내만 보인 것이다(nesting.go renderRaw·withheldRaw)
	rawSkip  int
	withheld bool
}

// Parse 는 원문을 Render 와 같은 길로 파싱·변환·렌더하고 문서 모델을 돌려준다. 오류는 Render 와 같다.
// HTML 만 필요하면 Render 를 쓴다.
func Parse(src []byte, opts Options) (*Doc, error) {
	return parseLimit(src, opts, MaxNestingDepth)
}

// Root 는 변환된 문서 AST 다. 블록 중첩 상한을 넘어 원문 글자로 보인 문서면 nil 이다(RawLines 를 쓴다).
func (d *Doc) Root() *ast.Document { return d.root }

// Source 는 AST 의 Segment 가 가리키는 원문이다(frontmatter 자리는 빈 줄).
func (d *Doc) Source() []byte { return d.body }

// RawLines 는 Root 가 nil 인 문서의 본문 줄이다(frontmatter 자리 줄은 뺀다 — renderRaw 와 같다). DropNotes 로 원문을 낼 수 없으면
// 안내 한 줄(WithheldNotice)이다.
func (d *Doc) RawLines() []string {
	if d.root != nil {
		return nil
	}
	if d.withheld {
		return []string{WithheldNotice}
	}
	lines := strings.Split(string(d.body), "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines[min(d.rawSkip, len(lines)):]
}

// Line 은 노드가 시작하는 원문 줄(1부터, frontmatter 포함)이다. 모르면 0.
func (d *Doc) Line(n ast.Node) int {
	if d.st == nil || n == nil {
		return 0
	}
	if n.Type() == ast.TypeBlock {
		if l := d.st.blockLine(n); l > 0 {
			return l
		}
	}
	for c := n; c != nil; c = c.Parent() {
		if p := c.Pos(); p >= 0 {
			return d.st.lines.line(p)
		}
		if t, ok := c.(*ast.Text); ok {
			return d.st.lines.line(t.Segment.Start)
		}
	}
	return 0
}

// HeadingInfo 는 제목 하나의 판정이다.
type HeadingInfo struct {
	ID       string // 본문 제목의 id(목차·링크의 #조각)
	Number   string // number-sections 의 번호("1.2"), 없으면 ""
	Unlisted bool   // .unlisted — 목차에 넣지 않는다. 노트 안 제목은 이 값과 상관없이 목차에 없다(headings.go inNotes)
}

// Heading 은 제목의 id·번호다.
func (d *Doc) Heading(h *ast.Heading) HeadingInfo {
	if d.st == nil {
		return HeadingInfo{}
	}
	info := d.st.headings[h]
	if info == nil {
		return HeadingInfo{}
	}
	return HeadingInfo{ID: info.id, Number: info.number, Unlisted: info.unlisted}
}

// ImageKind 는 그림 주소의 판정이다 — HTML 출력이 그 그림을 보이는지와 같은 판정이다(urls.go 주소 규칙).
type ImageKind int

const (
	ImageLocal   ImageKind = iota // 문서 폴더 기준 상대 경로(또는 절대 경로 — 받는 쪽이 가린다)
	ImageData                     // data:image/… 주소
	ImageRemote                   // http(s)·//host — Options.AllowRemoteImages 가 거짓이면 불러오지 않는다
	ImageBlocked                  // 막은 스킴
)

// ImageInfo 는 그림 하나의 판정이다.
type ImageInfo struct {
	Kind  ImageKind
	Dest  string // 원문 주소(이스케이프를 푼 값)
	Alt   string // 대체 글(Deckset 배치 지시면 "")
	Title string // 제목 글(그림 캡션이 되기도 한다 — Figure.Caption)
	Width string // width 속성(cssLength 를 통과한 CSS 길이), 없으면 ""
}

// Image 는 그림의 주소 판정·대체 글·너비다.
func (d *Doc) Image(img *ast.Image) ImageInfo {
	out := ImageInfo{Kind: ImageBlocked, Dest: inlineValue(img.Destination)}
	if img.Title != nil {
		out.Title = inlineValue(img.Title)
	}
	if d.st == nil {
		return out
	}
	info := d.st.images[img]
	if info == nil {
		return out
	}
	switch {
	case info.kind == urlRemote:
		out.Kind = ImageRemote
	case info.kind == urlOK && isDataURL(string(img.Destination)):
		out.Kind = ImageData
	case info.kind == urlOK:
		out.Kind = ImageLocal
	}
	if !info.noAlt {
		out.Alt = plainText(img, d.body)
	}
	out.Width = info.width
	return out
}

// Link 는 링크(Link·AutoLink)의 주소다. 막은 주소면 ok 가 거짓이다(글자만 남긴다 — render.go link).
func (d *Doc) Link(n ast.Node) (href string, ok bool) {
	if d.st == nil {
		return "", false
	}
	info, found := d.st.links[n]
	if !found || info.kind == urlBlocked {
		return "", false
	}
	return info.href, true
}

// AutoLinkLabel 은 자동 링크의 보이는 글자다.
func (d *Doc) AutoLinkLabel(n *ast.AutoLink) string { return string(n.Label(d.body)) }

// RawTag 는 남긴 인라인 HTML 의 정규화한 태그다("<br>"·"<sub>"·"</sub>" …). 버린 것이면 ok 가 거짓이다.
func (d *Doc) RawTag(n *ast.RawHTML) (tag string, ok bool) {
	if d.st == nil {
		return "", false
	}
	tag, ok = d.st.rawOK[n]
	return tag, ok
}

// BreakBlock 은 <br> 만 있는 HTML 블록의 <br> 수다. 다른 HTML 블록이면 0.
func (d *Doc) BreakBlock(n *ast.HTMLBlock) int {
	if d.st == nil {
		return 0
	}
	return strings.Count(d.st.brBlocks[n], "<br>")
}

// Math 는 그린 수식의 MathML 이다 — MathInline·MathBlock·```math 코드 블록. 그리지 못한 수식(원문 글자로 둔 것)이면 ok 가 거짓이다.
func (d *Doc) Math(n ast.Node) (mathml string, ok bool) {
	if d.st == nil {
		return "", false
	}
	mathml, ok = d.st.mathOut[n]
	return mathml, ok
}

// MathSource 는 수식의 원문이다 — MathInline 은 구분자를 포함한 원문, MathBlock 은 읽은 줄 그대로(구분자 포함), 코드 블록은 줄을 이은 것.
func (d *Doc) MathSource(n ast.Node) string {
	switch n := n.(type) {
	case *MathInline:
		return n.Raw
	case *MathBlock:
		var b strings.Builder
		for _, seg := range n.raw {
			b.Write(seg.Value(d.body))
		}
		return strings.TrimRight(b.String(), "\n")
	}
	return rawLines(n, d.body)
}

// DiagramPic 은 그린 mermaid 그림이다 — SVG 와 그림이 정한 캡션(없으면 "").
type DiagramPic struct {
	SVG     string
	Caption string
}

// Diagram 은 그림으로 그린 ```mermaid 코드 블록의 그림이다. 코드 블록으로 둔 것이면 ok 가 거짓이다.
func (d *Doc) Diagram(fc *ast.FencedCodeBlock) (DiagramPic, bool) {
	if d.st == nil {
		return DiagramPic{}, false
	}
	p, ok := d.st.diagramPics[fc]
	return p, ok
}

// CodeInfo 는 코드 블록의 언어·파일명 띠다(render.go codeBlock 과 같은 판정 — 언어가 없으면 파일명에서 짐작).
type CodeInfo struct {
	Lang  string
	Title string
}

// Code 는 펜스 코드 블록의 언어·파일명이다. 들여 쓴 코드 블록 등 다른 노드면 빈 CodeInfo 다.
func (d *Doc) Code(n ast.Node) CodeInfo {
	fc, ok := n.(*ast.FencedCodeBlock)
	if !ok {
		return CodeInfo{}
	}
	info := parseCodeInfo(fenceInfo(fc, d.body))
	lang := cleanLang([]byte(info.lang))
	if lang == "" && info.title != "" {
		lang = guessCodeLang(info.title)
	}
	return CodeInfo{Lang: lang, Title: info.title}
}

// CodeText 는 코드 블록의 글이다(줄 끝 \n 포함, 렌더러가 내는 글 그대로).
func (d *Doc) CodeText(n ast.Node) string {
	var b strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		b.Write(segValue(lines.At(i), d.body))
	}
	return b.String()
}

// Text 는 글자 노드(Text·String)의 보이는 글이다 — goldmark HTML 렌더러처럼 백슬래시 이스케이프·엔티티를 푼다(raw 글자는 그대로).
// 줄바꿈 표시(SoftLineBreak·HardLineBreak)는 붙이지 않는다 — 노드에서 읽는다.
func (d *Doc) Text(n ast.Node) string {
	switch n := n.(type) {
	case *ast.Text:
		v := n.Segment.Value(d.body)
		if n.IsRaw() {
			return string(v)
		}
		return inlineValue(v)
	case *ast.String:
		if n.IsCode() || n.IsRaw() {
			return string(n.Value)
		}
		return inlineValue(n.Value)
	}
	return ""
}

// PlainText 는 인라인의 글자만 모은 것이다(목차·대체 글과 같은 규칙 — headings.go plainText).
func (d *Doc) PlainText(n ast.Node) string { return plainText(n, d.body) }

// AlertTitle 은 알림 종류(note·tip …)의 제목 글이다(문서 lang 에 따른다 — render.go alertTitle 과 같다).
func (d *Doc) AlertTitle(typ string) string {
	return (&htmlRenderer{st: &state{meta: d.Meta}}).alertTitle(typ)
}
