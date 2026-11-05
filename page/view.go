package page

import (
	"html"
	"regexp"
	"strings"

	"github.com/itda-work/itda-doc/mdoc"
)

// 문서 조립. mdoc 이 낸 본문 앞에 표지(또는 제목 블록)와 목차를 붙여 "문서 흐름" HTML 하나로 만든다.
// 쪽 나눔은 화면(pager.js)이 이 흐름을 A4 상자에 나눠 담아 한다.
//
// 표지·제목 블록·목차의 HTML 구조는 여기서 정한 한 가지다(설계 3장). 테마는 CSS 로 배치만 바꾼다.
// 메타 값은 모두 글자로 이스케이프한다 — frontmatter 는 메일·웹에서 온 문서에도 있다.

// Part 는 머리글·바닥글의 한 조각이다. Text 는 글자 그대로, Field 는 화면이 채울 자리(page·pages)다.
// HTML 로 넘기지 않는 것은 frontmatter 값이 머리글에 들어가기 때문이다 — 화면은 textContent 로만 넣는다.
type Part struct {
	Text  string `json:"text,omitempty"`
	Field string `json:"field,omitempty"`
}

// Slots 는 왼쪽·가운데·오른쪽 세 자리다.
type Slots [3][]Part

// View 는 화면에 넘기는 문서 하나다.
type View struct {
	Path     string `json:"path"`
	FileName string `json:"file_name"`
	Title    string `json:"title"` // 창 제목·PDF 기본 이름
	Theme    string `json:"theme"`
	// DocTheme 은 문서가 frontmatter 로 고른 테마(없거나 모르면 "")다. 화면에서 테마를 바꿔도 이 값은 그대로다.
	DocTheme string `json:"doc_theme"`
	// DocThemeFrom 은 DocTheme 이 폴더 `_metadata.yml` 에서 왔을 때 그 파일의 문서 기준 상대 경로다(itda-hyve#504 — 테마 고르기가
	// 「(폴더 설정: …)」으로 출처를 보인다). 문서 자신이 theme: 을 정했거나 DocTheme 이 비면 "".
	DocThemeFrom string                 `json:"doc_theme_from,omitempty"`
	Themes       []ThemeInfo            `json:"themes"`
	Paper        string                 `json:"paper"` // a4 · a4-landscape
	Lang         string                 `json:"lang"`
	HTML         string                 `json:"html"`
	EditorTokens []mdoc.EditorCodeBlock `json:"editor_tokens,omitempty"`
	editorTokens []mdoc.EditorCodeBlock // renderView 내부 결과; App.Render에서만 공개(EditorCodeBlocks)
	Header       Slots                  `json:"header"`
	Footer       Slots                  `json:"footer"`
	H1Break      bool                   `json:"h1_break"`
	H1Always     bool                   `json:"h1_always"`
	// ListStyle 은 목록 기호 사다리다(itda-hyve#141): "" · "gaejosik" · "gongmun" · "gaejosik gongmun". 화면이 body 의
	// list-<값> 클래스로 건다. 문서의 list-style, 없으면 테마의 것(plain 은 "").
	ListStyle string `json:"list_style"`
	// TOC 는 사이드바용 제목 트리다(toc 설정과 무관하게 늘 채운다).
	TOC           []mdoc.TOCItem `json:"toc"`
	Warnings      []mdoc.Warning `json:"warnings"`
	RemoteImages  int            `json:"remote_images"`
	RemoteAllowed bool           `json:"remote_allowed"`
	Version       int64          `json:"version"` // 다시 렌더할 때마다 오른다(화면이 바뀐 것을 안다)
	// ThemeRev 는 테마 묶음의 판이다. 사용자 테마 폴더가 바뀌면 오르고, 화면은 테마 CSS 를 다시 받는다.
	ThemeRev int64 `json:"theme_rev"`
	// ThemesDir 는 사용자 테마 폴더가 있는지(회사 테마 만들기·폴더 열기를 보일지)다. 경로는 보내지 않는다.
	UserThemes bool `json:"user_themes"`
	// ScopeOK 는 테마 CSS 를 가둘 수 있는 엔진인지다(옛 WebKit 이면 false — 회사 테마를 쓰지 않는다).
	ScopeOK bool `json:"scope_ok"`
	// Slides 는 발표 설정이다(frontmatter slides: + 문서에서 정한 슬라이드 단계). 늘 채운다 — 화면은 "문서 | 슬라이드" 를 언제나 바꿀 수 있다.
	Slides SlideView `json:"slides"`
	// Board 는 보드 설정이다(frontmatter dashboard: + 문서에서 정한 타일 단계, itda-work/itda-hyve#362). 늘 채운다 — 화면은 보드 꼴로
	// 언제나 바꿀 수 있다.
	Board BoardView `json:"board"`
	// FolderMeta 는 이 문서에 적용된 폴더 `_metadata.yml` 의 문서 기준 상대 경로다(itda-hyve#474 — 가까운 것부터).
	// 화면이 「폴더 설정 적용됨」 표시를 낸다. 경로 없는 렌더(붙여넣은 원문 등)에는 비어 있다.
	FolderMeta []string `json:"folder_meta,omitempty"`
	// Code 는 코드 블록 표시 설정이다(frontmatter code: — itda-hyve#435 줄 번호). 늘 채운다 — 화면이 줄 번호를 감쌀지 결정한다.
	Code CodeView `json:"code"`
	// Logo 는 테마 로고의 앱 주소다(슬라이드 아래의 로고). 없으면 "".
	Logo string `json:"logo"`
	// Info 는 제목 슬라이드에 쓸 문서 정보다(글자 값 — 화면은 textContent 로만 넣는다). 테마의 defaults 를 따른다.
	Info  map[string]string `json:"info"`
	Error string            `json:"error,omitempty"`
}

// EditorCodeBlocks 는 렌더가 낸 편집기 코드 블록 토큰이다 — json 에 실리지 않는 값이라 앱이 필요할 때 EditorTokens 로 옮긴다.
func (v View) EditorCodeBlocks() []mdoc.EditorCodeBlock { return v.editorTokens }

// SlideView 는 화면이 슬라이드를 나눌 때 쓰는 값이다(설계 1.12·5장).
type SlideView struct {
	Present     bool   `json:"present"`     // frontmatter 에 slides: 가 있다 — 슬라이드 미리보기로 연다
	Ratio       string `json:"ratio"`       // 16:9 · 4:3
	Level       int    `json:"level"`       // 슬라이드를 여는 제목 단계(slides.level 또는 문서에서 정한 것). 제목이 없으면 0
	Transition  string `json:"transition"`  // fade · slide · none
	Incremental bool   `json:"incremental"` // 모든 목록을 차례로
	// FontScale 은 slides.font-scale 이다. 0 이면 화면이 base.css 의 기본(--slide-scale 0.9)을 쓴다
	FontScale float64 `json:"font_scale,omitempty"`
}

// CodeView 는 화면이 코드 블록을 어떻게 보이는지다(itda-hyve#435). LineNumbers 는 줄 번호다(기본 켬 — code.line-numbers: false 면 끔).
type CodeView struct {
	LineNumbers bool `json:"line_numbers"`
}

// slideView 는 frontmatter 와 렌더 결과로 발표 설정을 채운다. 빈 값은 기본(16:9·none·문서에서 정한 단계)이다.
func slideView(r mdoc.Result) SlideView {
	sv := SlideView{Ratio: "16:9", Level: r.SlideLevel, Transition: "none"}
	if m := r.Meta.Slides; m != nil {
		sv.Present = true
		if m.Ratio != "" {
			sv.Ratio = m.Ratio
		}
		if m.Level > 0 {
			sv.Level = m.Level
		}
		if m.Transition != "" {
			sv.Transition = m.Transition
		}
		sv.Incremental = m.Incremental
		sv.FontScale = m.FontScale
	}
	return sv
}

// BoardView 는 화면이 보드(격자 한 장)를 놓을 때 쓰는 값이다(itda-work/itda-hyve#362 — web/dashboard.js).
type BoardView struct {
	Present bool   `json:"present"` // frontmatter 에 dashboard: 가 있다 — 보드로 연다(slides: 보다 앞선다)
	Cols    int    `json:"cols"`    // 격자 열 수(2~6, 기본 4)
	Rows    int    `json:"rows"`    // 기준 상자에 들어가는 타일 행 수(2~4, 기본 3) — 타일 높이를 정한다
	Ratio   string `json:"ratio"`   // 비율 16:9 · 4:3 — 폭과 기준 상자는 슬라이드와 같다(web/slides.js SIZE)
	Level   int    `json:"level"`   // 타일을 여는 제목 단계(dashboard.level 또는 문서에서 정한 것 — 슬라이드와 같은 규칙). 제목이 없으면 0
}

// boardView 는 frontmatter 와 렌더 결과로 보드 설정을 채운다. 빈 값은 기본(4×3·16:9·문서에서 정한 단계)이다.
func boardView(r mdoc.Result) BoardView {
	bv := BoardView{Cols: 4, Rows: 3, Ratio: "16:9", Level: r.SlideLevel}
	if m := r.Meta.Dashboard; m != nil {
		bv.Present = true
		if m.Cols > 0 {
			bv.Cols = m.Cols
		}
		if m.Rows > 0 {
			bv.Rows = m.Rows
		}
		if m.Ratio != "" {
			bv.Ratio = m.Ratio
		}
		if m.Level > 0 {
			bv.Level = m.Level
		}
	}
	return bv
}

// codeView 는 frontmatter 로 코드 블록 표시를 채운다. line-numbers 를 정하지 않았거나 code: 가 없으면 켬이다(itda-hyve#435).
func codeView(r mdoc.Result) CodeView {
	v := CodeView{LineNumbers: true}
	if m := r.Meta.Code; m != nil && m.LineNumbers != nil {
		v.LineNumbers = *m.LineNumbers
	}
	return v
}

// 메타 키의 기본 이름표. 테마가 labels 로 바꾼다. 모르는 키(공문의 수신 등)는 키 자체가 이름표다.
var defaultLabels = map[string]string{
	"doc-no": "문서번호", "version": "판", "date": "작성일", "author": "작성자", "classification": "보안 등급",
	"organization": "기관", "subtitle": "부제", "title": "제목",
}

var enLabels = map[string]string{
	"doc-no": "Document No.", "version": "Version", "date": "Date", "author": "Author", "classification": "Classification",
	"organization": "Organization", "subtitle": "Subtitle", "title": "Title",
}

// metaValue 는 자리표시자·메타 키의 글자 값이다. 없으면 "", false.
func metaValue(m mdoc.Meta, key string) (string, bool) {
	var v string
	switch key {
	case "title":
		v = m.Title
	case "subtitle":
		v = m.Subtitle
	case "author":
		v = strings.Join(m.Author, ", ")
	case "date":
		v = m.Date
	case "doc-no":
		v = m.DocNo
	case "version":
		v = m.Version
	case "classification":
		v = m.Classification
	case "organization":
		v = m.Organization
	case "lang":
		v = m.Lang
	default:
		f, ok := m.Field(key)
		if !ok {
			return "", false
		}
		v = f.Value
	}
	v = strings.TrimSpace(v)
	return v, v != ""
}

// listStyle 은 문서의 list-style, 없으면 테마의 것이다. plain(보통 목록)은 "" 로 낸다.
func listStyle(m mdoc.Meta, t *Theme) string {
	ls := m.ListStyle
	if ls == "" {
		ls = t.ListStyle
	}
	if ls == mdoc.ListPlain {
		return ""
	}
	return ls
}

// themedValue 는 문서의 값, 없으면 테마의 defaults(회사 테마의 organization 등)다.
func themedValue(m mdoc.Meta, t *Theme, key string) (string, bool) {
	if v, ok := metaValue(m, key); ok {
		return v, true
	}
	if t != nil {
		if v := strings.TrimSpace(t.Defaults[key]); v != "" {
			return v, true
		}
	}
	return "", false
}

func label(t *Theme, lang, key string) string {
	if l, ok := t.Labels[key]; ok {
		return l
	}
	labels := defaultLabels
	if lang != "" && !strings.HasPrefix(lang, "ko") {
		labels = enLabels
	}
	if l, ok := labels[key]; ok {
		return l
	}
	return key
}

// 자리표시자 이름은 frontmatter 키 꼴(글자·숫자·-·_, 한글 가능)만. 그 밖의 {…} 는 글자 그대로 둔다.
var placeholderRe = regexp.MustCompile(`\{([\p{L}\p{N}_-]{1,40})\}`)

// parseSlot 은 머리글 틀(예: "{organization} · {title}")을 조각으로 나눈다. {page}·{pages} 는 화면이 채운다.
// 값이 없는 자리표시자는 빈 글자가 되고, 그 때문에 앞뒤에 남는 구분자(" · ")는 지운다 — 조직이 없는 문서의
// 머리글이 "· 제목" 으로 보이지 않게.
func parseSlot(tpl string, m mdoc.Meta, t *Theme) []Part {
	if tpl == "" {
		return nil
	}
	var parts []Part
	last := 0
	metaSeen, metaFilled := false, false
	for _, loc := range placeholderRe.FindAllStringSubmatchIndex(tpl, -1) {
		if loc[0] > last {
			parts = append(parts, Part{Text: tpl[last:loc[0]]})
		}
		key := tpl[loc[2]:loc[3]]
		switch key {
		case "page", "pages":
			parts = append(parts, Part{Field: key})
		default:
			v, ok := themedValue(m, t, key)
			metaSeen = true
			metaFilled = metaFilled || ok
			parts = append(parts, Part{Field: "meta", Text: v})
		}
		last = loc[1]
	}
	if last < len(tpl) {
		parts = append(parts, Part{Text: tpl[last:]})
	}
	// 메타 자리표시자가 있는데 하나도 값이 없으면 자리를 비운다 — "시행 {시행일}" 이 "시행" 만 남지 않게
	if metaSeen && !metaFilled {
		hasPage := false
		for _, p := range parts {
			hasPage = hasPage || p.Field == "page" || p.Field == "pages"
		}
		if !hasPage {
			return nil
		}
	}
	return tidySeparators(parts)
}

// tidySeparators 는 빈 메타 값 옆의 구분자 글자를 정리하고, 메타 조각을 일반 글자로 바꾼다.
func tidySeparators(parts []Part) []Part {
	out := make([]Part, 0, len(parts))
	for i, p := range parts {
		if p.Field != "meta" {
			out = append(out, p)
			continue
		}
		if p.Text != "" {
			out = append(out, Part{Text: p.Text})
			continue
		}
		// 빈 값: 바로 뒤의 구분자 글자를 하나 지운다(없으면 바로 앞의 것)
		if i+1 < len(parts) && parts[i+1].Field == "" && isSeparator(parts[i+1].Text) {
			parts[i+1].Text = ""
		} else if len(out) > 0 && out[len(out)-1].Field == "" && isSeparator(out[len(out)-1].Text) {
			out = out[:len(out)-1]
		}
	}
	// 빈 글자 조각을 버리고, 전부 공백이면 자리를 비운다
	clean := out[:0]
	visible := false
	for _, p := range out {
		if p.Field == "" && p.Text == "" {
			continue
		}
		if p.Field != "" || strings.TrimSpace(p.Text) != "" {
			visible = true
		}
		clean = append(clean, p)
	}
	if !visible {
		return nil
	}
	return clean
}

func isSeparator(s string) bool {
	t := strings.TrimSpace(s)
	switch t {
	case "·", "|", "-", "–", "—", "/", ",", "", "•":
		return true
	}
	return false
}

// Assemble 은 렌더 결과와 테마로 문서 흐름 HTML 과 머리글·바닥글을 만든다.
func Assemble(r mdoc.Result, t *Theme) (html string, header, footer Slots) {
	var b strings.Builder
	m := r.Meta
	lang := m.Lang
	switch {
	case m.Cover && !t.NoCover:
		writeCover(&b, m, t, lang)
	case m.Title != "":
		writeTitleBlock(&b, m, t, lang)
	}
	if m.TOC && !r.TOCPlaced {
		if toc := mdoc.RenderTOC(r.TOC); toc != "" {
			b.WriteString(`<section class="toc-page">` + "\n")
			writeEl(&b, `<h2 class="toc-title">`, tocTitle(lang), "</h2>\n")
			b.WriteString(toc)
			b.WriteString("</section>\n")
			if t.TOCBreak {
				b.WriteString(`<div class="page-break"></div>` + "\n")
			}
		}
	}
	b.WriteString(r.HTML)
	for i := range 3 {
		header[i] = parseSlot(t.Header[i], m, t)
		footer[i] = parseSlot(t.Footer[i], m, t)
	}
	return b.String(), header, footer
}

func tocTitle(lang string) string {
	if lang != "" && !strings.HasPrefix(lang, "ko") {
		return "Contents"
	}
	return "목차"
}

func esc(s string) string { return html.EscapeString(s) }

// writeEl 은 open, 이스케이프한 text, end 를 차례로 쓴다.
func writeEl(b *strings.Builder, open, text, end string) {
	b.WriteString(open)
	b.WriteString(esc(text))
	b.WriteString(end)
}

// writeCover 는 표지 쪽이다. 화면은 .cover 를 한 쪽으로 따로 담고 머리글·바닥글을 붙이지 않는다.
//
//	<section class="cover">
//	  <div class="cover-top"><img class="cover-logo"> <div class="cover-org">조직</div> <div class="cover-classification">대외비</div></div>
//	  <div class="cover-main"><h1 class="cover-title">제목</h1><p class="cover-subtitle">부제</p></div>
//	  <dl class="cover-meta"><div class="meta-row" data-key="doc-no"><dt>문서번호</dt><dd>…</dd></div>…</dl>
//	  <div class="cover-bottom"><div class="cover-date">날짜</div><div class="cover-org">조직</div></div>
//	</section>
func writeCover(b *strings.Builder, m mdoc.Meta, t *Theme, lang string) {
	b.WriteString(`<section class="cover">` + "\n")
	b.WriteString(`<div class="cover-top">`)
	if t.LogoURL != "" {
		writeEl(b, `<img class="cover-logo" src="`, t.LogoURL, `" alt="">`)
	}
	if v, ok := themedValue(m, t, "organization"); ok {
		writeEl(b, `<div class="cover-org">`, v, `</div>`)
	}
	if v, ok := themedValue(m, t, "classification"); ok {
		writeEl(b, `<div class="cover-classification">`, v, `</div>`)
	}
	b.WriteString("</div>\n")
	b.WriteString(`<div class="cover-main">`)
	writeEl(b, `<h1 class="cover-title">`, m.Title, `</h1>`)
	if v, ok := themedValue(m, t, "subtitle"); ok {
		writeEl(b, `<p class="cover-subtitle">`, v, `</p>`)
	}
	b.WriteString("</div>\n")
	writeMetaList(b, "cover-meta", m, t, lang)
	b.WriteString(`<div class="cover-bottom">`)
	if v, ok := themedValue(m, t, "date"); ok {
		writeEl(b, `<div class="cover-date">`, v, `</div>`)
	}
	if v, ok := themedValue(m, t, "organization"); ok {
		writeEl(b, `<div class="cover-org">`, v, `</div>`)
	}
	b.WriteString("</div>\n</section>\n")
}

// writeTitleBlock 은 표지가 없을 때 본문 첫머리의 제목 블록이다(공문의 가운데 정렬 제목·수신·참조 줄).
//
//	<header class="title-block"><img class="title-logo"><div class="title-org">조직</div><h1 class="title">제목</h1>
//	  <p class="subtitle">부제</p><dl class="title-meta">…</dl></header>
//
// 로고는 테마에 logo 가 있을 때만 있다(표지 없는 테마의 회사 로고 자리).
func writeTitleBlock(b *strings.Builder, m mdoc.Meta, t *Theme, lang string) {
	b.WriteString(`<header class="title-block">`)
	if t.LogoURL != "" {
		writeEl(b, `<img class="title-logo" src="`, t.LogoURL, `" alt="">`)
	}
	if v, ok := themedValue(m, t, "organization"); ok {
		writeEl(b, `<div class="title-org">`, v, `</div>`)
	}
	writeEl(b, `<h1 class="title">`, m.Title, `</h1>`)
	if v, ok := themedValue(m, t, "subtitle"); ok {
		writeEl(b, `<p class="subtitle">`, v, `</p>`)
	}
	writeMetaList(b, "title-meta", m, t, lang)
	b.WriteString("</header>\n")
}

func writeMetaList(b *strings.Builder, class string, m mdoc.Meta, t *Theme, lang string) {
	var rows strings.Builder
	for _, key := range t.Meta {
		v, ok := themedValue(m, t, key)
		if !ok {
			continue
		}
		writeEl(&rows, `<div class="meta-row" data-key="`, key, `"><dt>`)
		writeEl(&rows, "", label(t, lang, key), `</dt><dd>`)
		writeEl(&rows, "", v, "</dd></div>")
	}
	if rows.Len() > 0 {
		b.WriteString(`<dl class="`)
		b.WriteString(class)
		b.WriteString(`">`)
		b.WriteString(rows.String())
		b.WriteString("</dl>\n")
	}
}
