package mdoc

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// transform 은 goldmark 가 만든 AST 를 1장의 문법에 맞게 고친다. 순서가 뜻을 가진다:
// raw HTML 을 먼저 걷어 내야 쪽 나눔·그림 판정이 남은 노드만 본다.
func (st *state) transform(doc *ast.Document) {
	st.notes(doc)
	var (
		htmlBlocks []*ast.HTMLBlock
		rawParents []ast.Node
		paras      []*ast.Paragraph
		quotes     []*ast.Blockquote
		tables     []*extast.Table
		headings   []*ast.Heading
		images     []*ast.Image
		links      []ast.Node
		charts     []*Container
		fences     []*ast.FencedCodeBlock
	)
	seenParent := map[ast.Node]bool{}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.HTMLBlock:
			htmlBlocks = append(htmlBlocks, n)
		case *ast.RawHTML:
			if p := n.Parent(); !seenParent[p] {
				seenParent[p] = true
				rawParents = append(rawParents, p)
			}
		case *ast.Paragraph:
			paras = append(paras, n)
		case *ast.FencedCodeBlock:
			fences = append(fences, n)
		case *ast.Blockquote:
			quotes = append(quotes, n)
		case *extast.Table:
			tables = append(tables, n)
		case *ast.Heading:
			headings = append(headings, n)
		case *ast.Image:
			images = append(images, n)
		case *ast.Link, *ast.AutoLink:
			links = append(links, n)
		case *Container:
			if n.Name == "chart" {
				charts = append(charts, n)
			}
		case *ast.List:
			olDigits(n)
		}
		return ast.WalkContinue, nil
	})

	for _, b := range htmlBlocks {
		st.htmlBlock(b)
	}
	for _, p := range rawParents {
		st.rawInline(p)
	}
	for _, p := range paras {
		st.specialParagraph(p)
	}
	// 펜스 바로 앞 Deckset 줄 강조 지시어 문단은 펜스에 걸고 지운다(itda-hyve#479 — codehl.go). 아래 그림 판정은 지운 문단(부모 nil)을 건너뛴다
	for _, fc := range fences {
		st.codeHLDirectives(fc)
	}
	for _, q := range quotes {
		st.alert(q)
	}
	for _, img := range images {
		st.image(img)
	}
	var figs []*Figure
	for _, p := range paras {
		if p.Parent() != nil {
			if f := st.figure(p); f != nil {
				figs = append(figs, f)
			}
		}
	}
	// 섞인 문단의 마지막 줄 이미지를 그림으로 떼는 것은 다음 문단이 Figure: 캡션일 때만이다(itda-hyve#466) —
	// Figure: 가 없는 섞인 문단(배지 줄 따위)은 그대로 둔다.
	for _, p := range paras {
		if p.Parent() != nil {
			if f := st.splitTrailingImage(p); f != nil {
				figs = append(figs, f)
			}
		}
	}
	for _, f := range figs {
		st.figureCaption(f) // 그림 캡션 문단 — 표 캡션(tableCaption)과 같은 자리에 둔다
	}
	for _, t := range tables {
		st.tableCaption(t)
	}
	for _, c := range charts {
		st.chart(c) // 표 캡션을 옮긴 뒤 — 캡션이 차트 캡션이 된다
	}
	st.processHeadings(headings)
	st.slideSplits(doc) // 제목·그림 판정 뒤 — 슬라이드 경계와 분할 그림이 정해졌다(itda-hyve#550)
	for _, l := range links {
		st.link(l)
	}
	st.pitfalls(doc)
	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		if line := st.blockLine(c); line > 0 {
			c.SetAttributeString("data-line", []byte(strconv.Itoa(line)))
		}
	}
	// 표마다(최상위·컨테이너·목록·인용 안) 머리 행의 원문 줄 — 미리보기에서 표 편집으로 가는 길(itda-hyve#355). 값은 TableAt 의 Start 와 같다
	// (tableLine). 차트로 바뀐 표는 노드가 없어 붙지 않는다. goldmark 표 렌더러는 data- 속성을 필터와 상관없이 낸다.
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if t, ok := n.(*extast.Table); ok && entering {
			if line := st.tableLine(t); line > 0 {
				t.SetAttributeString("data-table-line", []byte(strconv.Itoa(line)))
			}
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
}

// notes 는 발표자 노트 — 이름이나 클래스가 notes 인 컨테이너(render.go container 가 붙이는 class 와 같은 classes() 로 판정한다) —
// 를 DropNotes 와 상관없이 늘 이름 notes 하나로 두고 다른 속성(class·id·data-*·너비)은 경고와 함께 버린다(itda-hyve#344 2단계 리뷰 5회차 —
// {.notes .page-break} 가 호스트에서는 쪽 나눔, 공유에서는 노트라 묶음이 갈렸다). 노트는 노트일 뿐 쪽 나눔 같은 구조 표지를 겸하지
// 않으므로, 렌더 결과의 노트 자리는 늘 `<div class="notes" data-line="N">` 이다. DropNotes 면 자식도 모두 떼어 빈 자리로 남긴다 —
// 태그·속성이 호스트와 바이트 단위로 같으므로 맨 위 블록의 차례·종류·class 가 같고 slideLevel·쪽 나눔·slides.js 묶음이 노트 공개와
// 상관없이 같다(리뷰 4회차). 다른 변환 전에 부르므로 노트 안 제목·그림·차트·링크는 수집되지 않고, 노트의 .chart 같은 class 도 변환이
// 보지 않는다. 제목의 .notes 는 노트가 아니라 processHeadings 가 경고와 함께 버리므로(headings.go dropHeadingNotes) 여기서 보지 않는다.
// 노트 안의 노트도 같게 정리하고(호스트에 그려지므로), DropNotes 면 바깥의 자식이라 함께 빠진다 — 먼저 모은 뒤 고치므로 걷는 중에
// 트리를 바꾸지 않는다. 경고는 DropNotes 와 상관없이 같다.
func (st *state) notes(doc ast.Node) {
	var found []*Container
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if c, ok := n.(*Container); ok && entering && slices.Contains(c.classes(), "notes") {
			found = append(found, c)
		}
		return ast.WalkContinue, nil
	})
	for _, c := range found {
		if !slices.Equal(c.classes(), []string{"notes"}) || c.Attrs.ID != "" || len(c.Attrs.Data) > 0 || c.Attrs.Unnumbered || c.Width != "" {
			st.warn(st.lines.line(c.Pos()), WarnBadAttribute, "발표자 노트에는 다른 class·id·속성을 붙이지 않습니다 — 버림(쪽 나눔은 --- 로 따로 씁니다)")
		}
		c.Name, c.Attrs, c.Width = "notes", attrs{}, ""
		if st.opts.DropNotes {
			c.RemoveChildren(c)
		}
	}
	if st.opts.DropNotes && len(found) > 0 {
		pruneFootnotes(doc)
	}
}

// pruneFootnotes 는 노트를 뗀 뒤의 각주를 고친다. goldmark 은 파싱 때 각주 정의를 노트 안에 적었어도 문서 끝 목록(FootnoteList)으로
// 옮겨 두므로, 노트를 떼도 노트에서만 가리킨 각주의 글이 남는다. 남길 각주는 남은 본문의 참조에서 출발해 남긴 각주 안의 참조를
// 따라 전이적으로 모은다(방문한 번호는 다시 보지 않으므로 순환해도 끝난다). 모으지 못한 각주(노트에서만 닿는 것)와 남기지 않은
// 참조로 돌아가는 되돌림 링크를 지우고, 남은 각주의 번호를 모은 차례(본문 참조가 나오는 차례, 그다음 각주 안 참조)로 1, 2, …
// 다시 매긴 뒤 목록도 그 차례로 놓는다. 목록이 비어도 지우지 않는다 — 노트처럼 빈 자리로 남겨 맨 위 블록 구조가 호스트와 같다
// (render.go footnoteList 가 빈 목록은 빈 div 로 그린다).
func pruneFootnotes(doc ast.Node) {
	var list *extast.FootnoteList
	var body []*extast.FootnoteLink
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *extast.FootnoteList:
			list = n
			return ast.WalkSkipChildren, nil
		case *extast.FootnoteLink:
			body = append(body, n)
		}
		return ast.WalkContinue, nil
	})
	if list == nil {
		return
	}
	defs := map[int]*extast.Footnote{}
	for c := list.FirstChild(); c != nil; c = c.NextSibling() {
		if fn, ok := c.(*extast.Footnote); ok {
			defs[fn.Index] = fn
		}
	}
	renum := map[int]int{}
	var kept []*extast.FootnoteLink // 남는 참조(본문 + 남긴 각주 안)
	var queue []int                 // 남길 각주 번호(모은 차례)
	visit := func(l *extast.FootnoteLink) {
		kept = append(kept, l)
		if _, ok := renum[l.Index]; !ok && defs[l.Index] != nil {
			renum[l.Index] = len(renum) + 1
			queue = append(queue, l.Index)
		}
	}
	for _, l := range body {
		visit(l)
	}
	for i := 0; i < len(queue); i++ {
		_ = ast.Walk(defs[queue[i]], func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if l, ok := n.(*extast.FootnoteLink); ok && entering {
				visit(l)
			}
			return ast.WalkContinue, nil
		})
	}
	refs := map[[2]int]bool{} // 남는 참조의 (각주 번호, 같은 각주 안 참조 순번)
	for _, l := range kept {
		refs[[2]int{l.Index, l.RefIndex}] = true
	}
	for idx, fn := range defs {
		if _, ok := renum[idx]; !ok {
			list.RemoveChild(list, fn)
			continue
		}
		var back []*extast.FootnoteBacklink
		_ = ast.Walk(fn, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if b, ok := n.(*extast.FootnoteBacklink); ok && entering {
				back = append(back, b)
			}
			return ast.WalkContinue, nil
		})
		for _, b := range back {
			if !refs[[2]int{b.Index, b.RefIndex}] {
				b.Parent().RemoveChild(b.Parent(), b)
				continue
			}
			b.Index = renum[b.Index]
		}
	}
	for _, l := range kept {
		if v, ok := renum[l.Index]; ok {
			l.Index = v
		}
	}
	for _, idx := range queue {
		fn := defs[idx]
		fn.Index = renum[idx]
		list.RemoveChild(list, fn)
		list.AppendChild(list, fn) // 새 번호 차례로
	}
}

// blockLine 은 블록이 시작하는 원문 줄이다. 정의 목록처럼 노드 위치가 첫 자식보다 뒤인 경우가 있어 자식까지 본다.
func (st *state) blockLine(n ast.Node) int {
	best := -1
	for c := n; c != nil && c.Type() == ast.TypeBlock; c = c.FirstChild() {
		if p := c.Pos(); p >= 0 && (best < 0 || p < best) {
			best = p
		}
	}
	return st.lines.line(best)
}

// ---- raw HTML (1.9) 과 쪽 나눔 (1.4)

var (
	pagebreakCommentRe = regexp.MustCompile(`(?i)^<!--\s*pagebreak\s*-->$`)
	// 내용 없는 div 하나, style 은 page-break-after·break-after 의 always·page 한 가지만
	pagebreakDivRe = regexp.MustCompile(`(?i)^<div\s+style\s*=\s*("|')\s*(?:page-break-after|break-after)\s*:\s*(?:always|page)\s*;?\s*("|')\s*>\s*</div>$`)
	brBlockRe      = regexp.MustCompile(`(?i)^(?:<br\s*/?>\s*)+$`)
	brRe           = regexp.MustCompile(`(?i)<br\s*/?>`)
	// 속성 없는 허용 태그. br 은 <br/> 도 받는다.
	allowedTagRe = regexp.MustCompile(`(?i)^<(/?)(br|sub|sup|kbd|mark|u|ins|small)\s*(/?)>$`)
)

func (st *state) htmlBlock(b *ast.HTMLBlock) {
	var buf bytes.Buffer
	for i := 0; i < b.Lines().Len(); i++ {
		buf.Write(segValue(b.Lines().At(i), st.src))
	}
	if b.HasClosure() {
		buf.Write(b.ClosureLine.Value(st.src))
	}
	v := strings.TrimSpace(buf.String())
	line := st.lines.line(b.Pos())
	parent := b.Parent()

	switch {
	case pagebreakCommentRe.MatchString(v), pagebreakDivRe.MatchString(v) && sameQuote(v):
		pb := &PageBreak{}
		pb.SetPos(b.Pos())
		parent.ReplaceChild(parent, b, pb)
		return
	case brBlockRe.MatchString(v):
		st.brBlocks[b] = strings.Repeat("<br>", len(brRe.FindAllString(v, -1)))
		return
	case strings.HasPrefix(v, "<!--"):
		// 주석은 조용히 버린다. 주석 뒤에 다른 HTML 이 붙어 있으면 그것은 알린다.
		if end := strings.Index(v, "-->"); end >= 0 && strings.TrimSpace(v[end+3:]) != "" {
			st.warn(line, WarnRawHTML, "렌더하지 않은 HTML: "+clip(strings.TrimSpace(v[end+3:])))
		}
	default:
		st.warn(line, WarnRawHTML, "렌더하지 않은 HTML: "+clip(v))
	}
	parent.RemoveChild(parent, b)
}

func sameQuote(v string) bool {
	m := pagebreakDivRe.FindStringSubmatch(v)
	return m != nil && m[1] == m[2]
}

// rawInline 은 한 부모 아래의 인라인 HTML 을 거른다. 허용 태그도 같은 부모 안에서 짝이 맞을 때만 남긴다 —
// 닫히지 않은 <u> 가 뒤 문단까지 번지거나 </small> 하나가 바깥 구조를 깨지 않게 한다.
func (st *state) rawInline(parent ast.Node) {
	type open struct {
		node *ast.RawHTML
		tag  string
	}
	var stack []open
	drop := func(n *ast.RawHTML, v string, quiet bool) {
		if !quiet {
			st.warn(st.rawLine(n), WarnRawHTML, "렌더하지 않은 HTML: "+clip(v))
		}
		delete(st.rawOK, n)
		parent.RemoveChild(parent, n)
	}
	for c := parent.FirstChild(); c != nil; {
		next := c.NextSibling()
		n, ok := c.(*ast.RawHTML)
		if !ok {
			c = next
			continue
		}
		v := st.rawValue(n)
		m := allowedTagRe.FindStringSubmatch(v)
		switch {
		case strings.HasPrefix(v, "<!--"):
			drop(n, v, true)
		case m == nil:
			drop(n, v, false)
		case strings.EqualFold(m[2], "br"):
			if m[1] != "" {
				drop(n, v, false) // </br>
			} else {
				st.rawOK[n] = "<br>"
			}
		case m[3] != "":
			drop(n, v, false) // <sub/>
		case m[1] == "":
			tag := strings.ToLower(m[2])
			st.rawOK[n] = "<" + tag + ">"
			stack = append(stack, open{n, tag})
		default:
			tag := strings.ToLower(m[2])
			if len(stack) > 0 && stack[len(stack)-1].tag == tag {
				stack = stack[:len(stack)-1]
				st.rawOK[n] = "</" + tag + ">"
			} else {
				drop(n, v, false)
			}
		}
		c = next
	}
	for _, o := range stack {
		drop(o.node, "<"+o.tag+">", false)
	}
}

func (st *state) rawValue(n *ast.RawHTML) string {
	var b strings.Builder
	for i := 0; i < n.Segments.Len(); i++ {
		b.Write(segValue(n.Segments.At(i), st.src))
	}
	return b.String()
}

func (st *state) rawLine(n *ast.RawHTML) int {
	if n.Segments.Len() == 0 {
		return 0
	}
	return st.lines.line(n.Segments.At(0).Start)
}

// clip 은 경고에 담을 HTML 을 한 줄 40자로 줄인다.
func clip(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= 40 {
		return s
	}
	r := []rune(s)
	return string(r[:40]) + "…"
}

// ---- 한 줄 문단: \newpage (1.4) 와 [TOC] (1.8)

func (st *state) specialParagraph(p *ast.Paragraph) {
	if p.Lines().Len() != 1 {
		return
	}
	v := strings.TrimSpace(string(segValue(p.Lines().At(0), st.src)))
	var repl ast.Node
	switch {
	case v == `\newpage`:
		repl = &PageBreak{}
	case strings.EqualFold(v, "[TOC]") && !hasChild[*ast.Link](p):
		repl = &TOCNav{}
		st.tocPlaced = true
	case v == ". . .":
		repl = &Pause{}
	default:
		return
	}
	repl.SetPos(p.Pos())
	p.Parent().ReplaceChild(p.Parent(), p, repl)
}

func hasChild[T ast.Node](n ast.Node) bool {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if _, ok := c.(T); ok {
			return true
		}
	}
	return false
}

// ---- 알림 (1.6)

var alertRe = regexp.MustCompile(`(?i)^\[!(note|tip|important|warning|caution)\]$`)

// alert 는 첫 줄이 [!NOTE] 뿐인 인용을 알림으로 바꾼다. 표시 줄은 지운다.
func (st *state) alert(q *ast.Blockquote) {
	p, ok := q.FirstChild().(*ast.Paragraph)
	if !ok || p.Lines().Len() == 0 {
		return
	}
	first := p.Lines().At(0)
	m := alertRe.FindStringSubmatch(strings.TrimSpace(string(first.Value(st.src))))
	if m == nil {
		return
	}
	// 첫 줄의 인라인이 모두 글자일 때만(링크 정의로 [!note] 가 링크가 된 경우는 알림이 아니다)
	for c := p.FirstChild(); c != nil; c = c.NextSibling() {
		t, ok := c.(*ast.Text)
		if !ok {
			if c.Pos() < first.Stop {
				return
			}
			break
		}
		if t.Segment.Start >= first.Stop {
			break
		}
	}
	for c := p.FirstChild(); c != nil; {
		next := c.NextSibling()
		t, ok := c.(*ast.Text)
		if !ok || t.Segment.Start >= first.Stop {
			break
		}
		p.RemoveChild(p, c)
		c = next
	}
	if p.ChildCount() == 0 {
		q.RemoveChild(q, p)
	}
	a := &Alert{AlertType: strings.ToLower(m[1])}
	a.SetPos(q.Pos())
	for c := q.FirstChild(); c != nil; {
		next := c.NextSibling()
		a.AppendChild(a, c)
		c = next
	}
	q.Parent().ReplaceChild(q.Parent(), q, a)
}

// ---- 이미지 width·슬라이드 배치·그림 (1.7, itda-hyve#550)

type imageInfo struct {
	kind  urlKind
	width string
	// 슬라이드 그림 배치(itda-hyve#550·itda-hyve#569): place 는 "" · "fit" · "left" · "right" · "full"(배경), fill 은 .fill(분할 패널·배경을 cover 로),
	// split 은 그림:글 비율(splitRatios 의 값, ""이면 1:1). noAlt 는 alt 가 Deckset 배치 키워드였다는 것 — 대체 글 대신 배치 지시라
	// alt 를 비운다.
	place string
	fill  bool
	split string
	noAlt bool
}

// imagePlaceClasses 는 이미지 속성으로 받는 배치 클래스다(itda-hyve#550, .full 은 itda-hyve#569). 그 밖의 클래스·id 는 경고하고 버린다.
var imagePlaceClasses = map[string]bool{"fit": true, "fill": true, "left": true, "right": true, "full": true}

// splitRatios 는 split= 으로 받는 그림:글 비율이다(itda-hyve#550).
var splitRatios = map[string]bool{"1:1": true, "1:2": true, "1:3": true, "2:1": true, "3:1": true}

// decksetWordRe 는 Deckset 그림 지시 낱말이다(itda-hyve#550). alt 전체가 이 낱말로만 이루어졌을 때만 배치 지시로 읽는다.
var decksetWordRe = regexp.MustCompile(`^(?:left|right|fit|inline|filtered|original|[0-9]+%)$`)

// decksetAlt 는 alt 글자가 Deckset 지시(낱말만)인지 보고 배치를 정한다. ok 가 거짓이면 지시가 아니다(보통 대체 글).
// 받는 꼴: left·right → 분할 패널 cover, left fit·right fit(차례 무관) → 분할 패널 contain, fit 단독 → 배경 contain(.full, itda-hyve#569),
// inline → 보통 그림. 그 밖의 지시 낱말 조합(filtered·original·N% 등)은 known 이 거짓 — 경고하고 보통 그림으로 둔다.
func decksetAlt(alt string) (place string, fill, known, ok bool) {
	words := strings.Fields(alt)
	if len(words) == 0 {
		return "", false, false, false
	}
	for _, w := range words {
		if !decksetWordRe.MatchString(w) {
			return "", false, false, false
		}
	}
	slices.Sort(words)
	switch strings.Join(words, " ") {
	case "left", "right":
		return words[0], true, true, true
	case "fit left":
		return "left", false, true, true
	case "fit right":
		return "right", false, true, true
	case "fit":
		return "full", false, true, true
	case "inline":
		return "", false, true, true
	}
	return "", false, false, true
}

func (st *state) image(img *ast.Image) {
	line := st.lines.line(img.Pos())
	info := &imageInfo{kind: classifyImage(string(img.Destination))}
	st.images[img] = info
	switch info.kind {
	case urlBlocked:
		st.warn(line, WarnUnsafeURL, "막은 이미지 주소: "+clip(string(img.Destination)))
	case urlRemote:
		if !st.opts.AllowRemoteImages {
			st.warn(line, WarnRemoteImage, "불러오지 않은 원격 이미지: "+clip(string(img.Destination)))
		}
	}
	// Deckset 호환(itda-hyve#550): alt 가 지시 낱말로만 이루어졌으면 배치 지시다. 아래 {…} 의 배치 클래스가 있으면 그쪽이 이긴다.
	if alt := plainText(img, st.src); alt != "" {
		if place, fill, known, ok := decksetAlt(alt); ok {
			info.noAlt = true
			if known {
				info.place, info.fill = place, fill
			} else {
				st.warn(line, WarnBadAttribute, fmt.Sprintf("Deckset 그림 지시 %q 는 받지 않아 보통 그림으로 둠(받는 것: left·right·left fit·right fit·fit·inline)", alt))
			}
		}
	}

	// 바로 뒤의 {…} (Pandoc link_attributes). 글자 노드가 쪼개져 있을 수 있어 이어 붙여 본다.
	t, ok := img.NextSibling().(*ast.Text)
	if !ok || t.Segment.Len() == 0 || st.src[t.Segment.Start] != '{' {
		return
	}
	start := t.Segment.Start
	var acc []byte
	end := -1
	for c := ast.Node(t); c != nil; c = c.NextSibling() {
		ct, ok := c.(*ast.Text)
		if !ok || ct.Segment.Start != start+len(acc) {
			break
		}
		acc = append(acc, ct.Segment.Value(st.src)...)
		if i := bytes.IndexByte(acc, '}'); i >= 0 {
			end = i
			break
		}
		if ct.SoftLineBreak() || ct.HardLineBreak() {
			break
		}
	}
	if end < 0 {
		return
	}
	a, err := parseAttrs(string(acc[1:end]))
	if err != nil {
		st.warn(line, WarnBadAttribute, "이미지 속성을 읽지 못해 글자로 둠: "+err.Error())
		return
	}
	if w, ok := a.get("width"); ok {
		if css, ok := cssLength(w); ok {
			info.width = css
		} else {
			st.warn(line, WarnBadAttribute, fmt.Sprintf("이미지 width %q 는 길이 형식이 아니라 버림", w))
		}
	}
	st.imagePlace(line, info, a)
	unknown := a.ID != "" || a.Unnumbered
	for _, c := range a.Classes {
		if !imagePlaceClasses[c] {
			unknown = true
		}
	}
	for _, kv := range a.Data {
		if kv[0] != "width" && kv[0] != "split" {
			unknown = true
		}
	}
	if unknown {
		st.warn(line, WarnBadAttribute, "이미지 속성은 width·split 과 클래스 .fit·.fill·.left·.right·.full 만 받아 나머지는 버림")
	}
	cutFront(img.Parent(), t, start+end+1)
}

// imagePlace 는 이미지 속성의 배치 클래스·split 을 읽는다(itda-hyve#550·itda-hyve#569). 배치 클래스(.fit·.left·.right·.full)가 있으면 Deckset alt
// 지시를 통째로 덮는다(.fill 도 속성에 적힌 것만). 배치 클래스 없이 .fill 만 있으면 alt 지시의 분할·배경 그림에 더한다. .left·.right·
// .full 을 둘 이상 적으면 먼저 적힌 쪽 + 경고, 그 셋 가운데 하나가 있으면 .fit 은 겹친 뜻이라 경고 없이 버린다.
// .fill 은 분할 그림(.left·.right)과 배경(.full)에만, split 은 분할 그림에만 뜻이 있어 그 밖에서는 경고하고 버린다. split 이 받는
// 비율이 아니면 경고하고 1:1.
func (st *state) imagePlace(line int, info *imageInfo, a attrs) {
	side := ""
	for _, c := range a.Classes {
		if c == "left" || c == "right" || c == "full" {
			if side == "" {
				side = c
			} else if c != side {
				st.warn(line, WarnBadAttribute, "이미지 배치 .left·.right·.full 은 하나만 — 먼저 적은 ."+side+" 를 씀")
			}
		}
	}
	if side != "" {
		info.place, info.fill = side, a.has("fill")
	} else if a.has("fit") {
		info.place, info.fill = "fit", false
	} else if a.has("fill") && (info.place == "left" || info.place == "right" || info.place == "full") {
		info.fill = true
	}
	split, hasSplit := a.get("split")
	isSide := info.place == "left" || info.place == "right"
	if a.has("fill") && !isSide && info.place != "full" {
		st.warn(line, WarnBadAttribute, "이미지 .fill 은 .left·.right·.full 과 함께만 받아 버림")
	}
	if !hasSplit {
		return
	}
	switch {
	case !isSide:
		st.warn(line, WarnBadAttribute, "이미지 split 은 .left·.right 와 함께만 받아 버림")
	case !splitRatios[split]:
		st.warn(line, WarnBadAttribute, fmt.Sprintf("이미지 split %q 는 1:1·1:2·1:3·2:1·3:1 이 아니라 1:1 로 둠", split))
	case split != "1:1":
		info.split = split
	}
}

// slideSplits 는 분할 그림(.left·.right)의 패널 여부와 배경 그림(.full, itda-hyve#569)의 배경 여부, 가로 배치(fit-row, itda-hyve#569)를 정한다
// (itda-hyve#550 — 판정은 여기서만 하고 slides.js 는 표시된 블록만 옮긴다).
//  0. 가로 배치: 맨 위 문단이 .fit 그림 2장 이상만으로 되어 있으면 class fit-row 를 단다(fitRow). 아래 1·2 에서 .fit 으로 바뀌기 전의
//     배치로 본다 — 분할·배경 그림이 섞인 문단은 가로 배치가 아니다(경고).
//  1. 패널·배경 후보는 맨 위 블록이 그 그림 하나뿐인 것 — 그림 하나뿐인 그림(Figure, 캡션은 함께)과 그림 하나만 감싼 링크 하나뿐인
//     문단(slotImage). 그 밖(목록·인용·다단·상자·글과 섞인 문단·제목·노트·각주 안)의 분할·배경 그림은 경고하고 제자리 .fit 으로 둔다.
//  2. 그림 자리(패널 또는 배경)는 슬라이드당 하나 — 먼저 적은 후보가 자리를 갖고, 두 번째부터는 종류와 상관없이 경고하고 .fit.
//     자리를 가진 후보의 맨 위 블록에 data-slide-panel(분할) 또는 data-slide-bg(배경)를 단다(slides.js takeSlot 이 그 블록을 옮긴다).
//
// 슬라이드는 slides.js groups 가 렌더된 맨 위 블록을 보는 분기를 같은 차례로 옮겨 가른다(lvl 은 frontmatter slides.level, 없으면
// slideLevel — 화면이 받는 값과 같다): 목차 자리(TOCNav)·class notes 블록은 건너뛰고, 쪽 나눔·가로줄·class page-break 블록
// (:::page-break 컨테이너, {.page-break} 제목)·슬라이드 단계 이하의 제목은 경계, 각주 목록·class footnotes 블록은 앞뒤가 경계인
// 제 슬라이드, 그 밖은 지금 슬라이드의 본문이다.
func (st *state) slideSplits(doc ast.Node) {
	lvl := slideLevel(doc)
	if st.meta.Slides != nil && st.meta.Slides.Level > 0 {
		lvl = st.meta.Slides.Level
	}
	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		if p, ok := c.(*ast.Paragraph); ok {
			st.fitRow(p)
		}
	}
	cands := map[*ast.Image]bool{}
	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		if img := st.slotImage(c); img != nil {
			cands[img] = true
		}
	}
	toFit := func(img *ast.Image, msg string) {
		st.warn(st.lines.line(img.Pos()), WarnBadAttribute, msg)
		info := st.images[img]
		info.place, info.fill, info.split = "fit", false, ""
	}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if img, ok := n.(*ast.Image); ok && entering && !cands[img] {
			switch info := st.images[img]; {
			case info == nil:
			case info.place == "left" || info.place == "right":
				toFit(img, "분할 그림(.left·.right)은 그림 하나뿐인 줄에서만 패널 — 이 그림은 제자리 .fit 으로 둠")
			case info.place == "full":
				toFit(img, "배경 그림(.full)은 그림 하나뿐인 줄에서만 배경 — 이 그림은 제자리 .fit 으로 둠")
			}
		}
		return ast.WalkContinue, nil
	})
	seen := "" // 이 슬라이드의 그림 자리를 가진 종류 — "" · "panel" · "bg"
	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		cls := st.topClasses(c)
		_, isFootnotes := c.(*extast.FootnoteList)
		h, isHeading := c.(*ast.Heading)
		switch {
		case isTOC(c), slices.Contains(cls, "notes"):
		case isBreak(c), slices.Contains(cls, "page-break"), isHeading && lvl > 0 && h.Level <= lvl, isFootnotes, slices.Contains(cls, "footnotes"):
			seen = "" // 각주는 제 슬라이드 — 안에 후보가 없고 뒤는 새 슬라이드다
		default:
			img := st.slotImage(c)
			if img == nil {
				continue
			}
			kind, attr := "panel", "data-slide-panel"
			if st.images[img].place == "full" {
				kind, attr = "bg", "data-slide-bg"
			}
			switch {
			case seen == "panel" && kind == "panel":
				toFit(img, "분할 그림(.left·.right)은 슬라이드당 하나 — 이 그림은 .fit 으로 둠")
			case seen == "bg" && kind == "bg":
				toFit(img, "배경 그림(.full)은 슬라이드당 하나 — 이 그림은 .fit 으로 둠")
			case seen != "":
				toFit(img, "분할 그림과 배경 그림은 합쳐 슬라이드당 하나 — 먼저 적은 쪽을 쓰고 이 그림은 .fit 으로 둠")
			default:
				c.SetAttributeString(attr, []byte("1"))
				seen = kind
			}
		}
	}
}

// fitRow 는 맨 위 문단 p 가 .fit 그림 2장 이상만으로 되어 있으면(그림 하나만 감싼 링크도 그림 한 장, 사이에는 빈 글자·줄바꿈만)
// class fit-row 를 단다(itda-hyve#569 — slides.js·pager.js 가 같은 높이로 가로에 늘어놓는다. 문서에는 규칙이 없어 지금과 같은 문단이다).
// .fit 그림이 2장 이상인데 다른 것(글·서식·다른 배치의 그림·.fit 아닌 그림)이 섞였으면 경고하고 그대로 둔다(각 그림은 제자리 .fit).
func (st *state) fitRow(p *ast.Paragraph) {
	isFit := func(n ast.Node) bool {
		img, ok := n.(*ast.Image)
		return ok && st.images[img] != nil && st.images[img].place == "fit"
	}
	fits := 0
	_ = ast.Walk(p, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && isFit(n) {
			fits++
		}
		return ast.WalkContinue, nil
	})
	if fits < 2 {
		return
	}
	pure := true
	for k := p.FirstChild(); k != nil && pure; k = k.NextSibling() {
		switch k := k.(type) {
		case *ast.Text:
			pure = len(bytes.TrimSpace(k.Segment.Value(st.src))) == 0
		case *ast.Image:
			pure = isFit(k)
		case *ast.Link:
			pure = isFit(st.soleChild(k))
		default:
			pure = false
		}
	}
	if !pure {
		st.warn(st.lines.line(p.Pos()), WarnBadAttribute, "가로 배치는 .fit 그림만 든 문단만 — 글·다른 그림이 섞여 각 그림을 제자리 .fit 으로 둠")
		return
	}
	p.SetAttributeString("class", []byte("fit-row"))
}

// slotImage 는 맨 위 블록 c 가 분할 그림(.left·.right)이나 배경 그림(.full) 하나뿐이면 그 그림을, 아니면 nil 을 돌려준다(itda-hyve#550·itda-hyve#569):
// 그림(Figure)의 자식이 이미지 하나(+ Figure: 캡션)이거나, 문단의 내용이 링크 하나이고 그 링크의 내용이 이미지 하나인 것. 빈 글자
// (속성을 떼고 남은 공백)는 세지 않는다.
func (st *state) slotImage(c ast.Node) *ast.Image {
	var img *ast.Image
	switch c := c.(type) {
	case *Figure:
		for k := c.FirstChild(); k != nil; k = k.NextSibling() {
			if _, ok := k.(*FigureCaption); ok {
				continue
			}
			if i, ok := k.(*ast.Image); ok && img == nil {
				img = i
				continue
			}
			return nil
		}
	case *ast.Paragraph:
		l, ok := st.soleChild(c).(*ast.Link)
		if !ok {
			return nil
		}
		img, _ = st.soleChild(l).(*ast.Image)
	}
	if img == nil {
		return nil
	}
	if info := st.images[img]; info == nil || (info.place != "left" && info.place != "right" && info.place != "full") {
		return nil
	}
	return img
}

// soleChild 는 n 의 자식 가운데 빈 글자가 아닌 것이 하나뿐이면 그것을, 아니면 nil 을 돌려준다.
func (st *state) soleChild(n ast.Node) ast.Node {
	var one ast.Node
	for k := n.FirstChild(); k != nil; k = k.NextSibling() {
		if t, ok := k.(*ast.Text); ok && len(bytes.TrimSpace(t.Segment.Value(st.src))) == 0 {
			continue
		}
		if one != nil {
			return nil
		}
		one = k
	}
	return one
}

// olDigits 는 번호 목록(<ol>)에 가장 큰 번호의 자릿수 class(ol-d2 … ol-d9)를 단다(itda-hyve#575). 가장 큰 번호 = 시작 번호 + 항목 수 − 1.
// 번호(::marker)는 목록 바깥 들여쓰기 칸에 그려지고 쪽 본문 상자가 그 밖을 잘라 내므로, base.css 가 이 class 에서만 들여쓰기를
// 자릿수만큼 넓힌다. 한 자리(9 이하)는 class 가 없어 모양이 그대로다. 아홉 자리 이상은 ol-d9 하나다(goldmark 의 시작 번호는
// 아홉 자리까지). 글머리 목록은 대상이 아니다. 중첩 목록은 목록마다 따로 센다. 쪽을 넘어 나뉜 뒷조각(pager.js splitList)은
// 원래 목록을 얕게 복제해 class 가 그대로 남는다 — 뒷조각의 번호도 같은 가장 큰 번호 이하다.
func olDigits(l *ast.List) {
	if !l.IsOrdered() {
		return
	}
	n := 0
	for c := l.FirstChild(); c != nil; c = c.NextSibling() {
		if _, ok := c.(*ast.ListItem); ok {
			n++
		}
	}
	d := len(strconv.Itoa(max(l.Start+n-1, 0)))
	if d < 2 {
		return
	}
	cls := "ol-d" + strconv.Itoa(min(d, 9))
	if v, ok := l.AttributeString("class"); ok {
		if b, ok := v.([]byte); ok && len(b) > 0 {
			cls = string(b) + " " + cls
		}
	}
	l.SetAttributeString("class", []byte(cls))
}

func isTOC(n ast.Node) bool { _, ok := n.(*TOCNav); return ok }

func isBreak(n ast.Node) bool {
	switch n.(type) {
	case *ast.ThematicBreak, *PageBreak:
		return true
	}
	return false
}

// topClasses 는 맨 위 블록이 렌더될 때 갖는 class 다 — slides.js groups 가 class 로 고르는 분기(.notes·.page-break·.footnotes)를
// 같게 보려고 쓴다. 컨테이너는 이름과 클래스(render.go container), 제목은 processHeadings 가 남긴 class(render.go heading)다.
func (st *state) topClasses(n ast.Node) []string {
	switch n := n.(type) {
	case *Container:
		return n.classes()
	case *ast.Heading:
		if info := st.headings[n]; info != nil {
			return info.classes
		}
	}
	return nil
}

// cutFront 는 from 부터 이어지는 글자 노드에서 cut 앞부분을 지운다.
func cutFront(parent ast.Node, from *ast.Text, cut int) {
	for c := ast.Node(from); c != nil; {
		next := c.NextSibling()
		t, ok := c.(*ast.Text)
		if !ok {
			return
		}
		if t.Segment.Stop <= cut && !t.SoftLineBreak() && !t.HardLineBreak() {
			parent.RemoveChild(parent, c)
			c = next
			continue
		}
		if t.Segment.Start < cut {
			t.Segment = t.Segment.WithStart(min(cut, t.Segment.Stop))
		}
		return
	}
}

// figure 는 이미지 하나만 있는 문단을 그림으로 바꾼다. 캡션은 title 이다(alt 는 쓰지 않는다).
// 만든 Figure 를 돌려준다(그림이 아니면 nil) — 그림 캡션 문단(figureCaption)이 이어서 본다.
func (st *state) figure(p *ast.Paragraph) *Figure {
	var img *ast.Image
	for c := p.FirstChild(); c != nil; c = c.NextSibling() {
		switch c := c.(type) {
		case *ast.Image:
			if img != nil {
				return nil
			}
			img = c
		case *ast.Text:
			if len(bytes.TrimSpace(c.Segment.Value(st.src))) > 0 {
				return nil
			}
		default:
			return nil
		}
	}
	if img == nil {
		return nil
	}
	f := &Figure{Caption: inlineValue(img.Title)}
	f.SetPos(p.Pos())
	for c := p.FirstChild(); c != nil; {
		next := c.NextSibling()
		if c == ast.Node(img) {
			f.AppendChild(f, c)
		} else {
			p.RemoveChild(p, c)
		}
		c = next
	}
	p.Parent().ReplaceChild(p.Parent(), p, f)
	return f
}

// ---- 그림 캡션 문단 (1.7, itda-hyve#466)

var figureCaptionRe = regexp.MustCompile(`^[ \t]*(?:Figure:|figure:)[ \t]*`)

// figureCaptionCut 은 문단이 그림 캡션 문단(Figure: · figure: 로 시작)이면 접두 다음의 원문 위치를,
// 아니면 -1 을 돌려준다. 표의 captionRe 와는 섞지 않는다 — 표 편집기(TableAt)가 저쪽 정규식을 쓴다.
func (st *state) figureCaptionCut(p *ast.Paragraph) int {
	if p.Lines().Len() == 0 {
		return -1
	}
	first := p.Lines().At(0)
	loc := figureCaptionRe.FindIndex(first.Value(st.src))
	if loc == nil {
		return -1
	}
	return first.Start + loc[1]
}

// splitTrailingImage 는 문단의 마지막 줄이 이미지 하나뿐이고(앞 줄과 줄바꿈으로 갈린) 다음 문단이
// Figure: 캡션이면, 그 줄을 떼어 문단 뒤에 그림으로 만든다 — 문장 아래 줄에 붙인 그림에도 캡션을 달 수
// 있게 한다. 떼어 낸 그림은 이미지의 원문 줄을 위치로 갖는다(최상위면 data-line 이 된다). 마지막 줄에 글이 섞였거나(글
// ![a](b)) 문단이 한 줄이면 그대로 둔다(nil) — 한 줄 그림 문단은 figure 가 먼저 그림으로 만든다.
func (st *state) splitTrailingImage(p *ast.Paragraph) *Figure {
	next, ok := p.NextSibling().(*ast.Paragraph)
	if !ok || st.figureCaptionCut(next) < 0 {
		return nil
	}
	var ns []ast.Node
	for c := p.FirstChild(); c != nil; c = c.NextSibling() {
		ns = append(ns, c)
	}
	i := len(ns) - 1
	for ; i >= 0 && st.spaceText(ns[i]); i-- { // 이미지 뒤 공백
	}
	if i < 0 {
		return nil
	}
	img, ok := ns[i].(*ast.Image)
	if !ok {
		return nil
	}
	for i--; i >= 0 && st.spaceText(ns[i]); i-- { // 이미지 앞 공백
	}
	if i < 0 {
		return nil
	}
	br, ok := ns[i].(*ast.Text)
	if !ok || (!br.SoftLineBreak() && !br.HardLineBreak()) {
		return nil
	}
	// 끝 줄바꿈(강제 줄바꿈 꼬리 공백 포함)을 지운다 — 앞 문장은 문단으로 남는다.
	br.SetSoftLineBreak(false)
	br.SetHardLineBreak(false)
	v := br.Segment.Value(st.src)
	if n := len(bytes.TrimRight(v, " \t")); n < len(v) {
		br.Segment = br.Segment.WithStop(br.Segment.Stop - (len(v) - n))
	}
	f := &Figure{Caption: inlineValue(img.Title)}
	f.SetPos(img.Pos())
	for _, n := range ns[i+1:] {
		p.RemoveChild(p, n)
		if _, ok := n.(*ast.Image); ok {
			f.AppendChild(f, n)
		}
	}
	p.Parent().InsertAfter(p.Parent(), p, f)
	return f
}

// spaceText 는 줄바꿈이 아니고 공백뿐인 글자 노드다.
func (st *state) spaceText(n ast.Node) bool {
	t, ok := n.(*ast.Text)
	return ok && !t.SoftLineBreak() && !t.HardLineBreak() && len(bytes.TrimSpace(t.Segment.Value(st.src))) == 0
}

// figureCaption 은 그림 바로 다음의 Figure: 문단을 <figcaption> 으로 옮긴다(tableCaption 과 같은 꼴).
// 접두를 떼고 문단의 인라인을 통째로 옮기므로 서식이 살고 여러 줄 캡션도 그대로 담긴다. 캡션 문단이 있으면
// 이미지 title 캡션보다 우선한다(렌더가 FigureCaption 자식이 있으면 title 캡션을 내지 않는다).
func (st *state) figureCaption(f *Figure) {
	p, ok := f.NextSibling().(*ast.Paragraph)
	if !ok {
		return
	}
	cut := st.figureCaptionCut(p)
	if cut < 0 {
		return
	}
	ft, ok := p.FirstChild().(*ast.Text)
	if !ok {
		return
	}
	cutFront(p, ft, cut)
	c := &FigureCaption{}
	for n := p.FirstChild(); n != nil; {
		next := n.NextSibling()
		c.AppendChild(c, n)
		n = next
	}
	p.Parent().RemoveChild(p.Parent(), p)
	f.AppendChild(f, c)
}

// ---- 표 캡션 (1.7)

var captionRe = regexp.MustCompile(`^[ \t]*(?:Table:|table:|:)[ \t]*`)

// tableCaption 은 표 바로 다음의 Table: · : 문단을 <caption> 으로 옮긴다.
func (st *state) tableCaption(t *extast.Table) {
	p, ok := t.NextSibling().(*ast.Paragraph)
	if !ok || p.Lines().Len() == 0 {
		return
	}
	first := p.Lines().At(0)
	loc := captionRe.FindIndex(first.Value(st.src))
	if loc == nil {
		return
	}
	cut := first.Start + loc[1]
	ft, ok := p.FirstChild().(*ast.Text)
	if !ok {
		return
	}
	cutFront(p, ft, cut)
	c := &TableCaption{}
	for n := p.FirstChild(); n != nil; {
		next := n.NextSibling()
		c.AppendChild(c, n)
		n = next
	}
	p.Parent().RemoveChild(p.Parent(), p)
	t.InsertBefore(t, t.FirstChild(), c)
}

// ---- 링크 (1.9)

type linkInfo struct {
	kind urlKind
	href string
}

func (st *state) link(n ast.Node) {
	var dest string
	switch n := n.(type) {
	case *ast.Link:
		dest = string(n.Destination)
	case *ast.AutoLink:
		dest = string(n.URL(st.src))
		if n.AutoLinkType == ast.AutoLinkEmail && !strings.HasPrefix(strings.ToLower(dest), "mailto:") {
			dest = "mailto:" + dest
		}
	}
	info := linkInfo{kind: classifyLink(dest), href: dest}
	if info.kind == urlBlocked {
		st.warn(st.lines.line(n.Pos()), WarnUnsafeURL, "막은 링크 주소: "+clip(dest))
	}
	st.links[n] = info
}

// segValue 는 Segment.Value 가 포인터 메서드라 값으로 받은 Segment 에 쓰려고 둔다.
func segValue(s text.Segment, src []byte) []byte { return s.Value(src) }
