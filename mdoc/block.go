package mdoc

import (
	"bytes"
	"regexp"
	"slices"
	"strings"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
)

// 자료 표 발행기(#365 ③ — internal/datareport)가 쓰는 읽기 전용 도우미. 렌더 경로는 쓰지 않는다 — 칸 이스케이프는 CSV 표(#115)
// 와 발행기의 한 정본이고, 최상위 블록 범위는 표·:::chart 를 그 앞 기록 주석과 묶어 읽는 데 쓴다.

// cellEscapes 는 칸에서 백슬래시로 막는 글자다. GFM 표 칸의 `|`, 강조·코드·링크·이미지·raw HTML·엔티티·취소선·수식(#116)·각주.
// CommonMark 는 ASCII 문장부호 어느 것이든 앞에 \ 를 붙이면 그 글자 그대로다 — GitHub·VS Code 에서도 같다.
const cellEscapes = "\\`*_[]<>!&~|$"

// EscapeCell 은 칸 원값(신뢰하지 않는 글)을 표 칸의 원문으로 바꾼다. 앞뒤 공백은 GFM 이 어차피 버린다. 칸 안의 줄바꿈은 <br>(설계
// 1.9 가 허용하는 태그 — GitHub 도 표 칸에서 줄을 바꾼다), 탭은 공백, 그 밖의 제어 글자는 뺀다. brs 는 <br> 로 바꾼 곳, dropped 는
// 뺀 제어 글자의 수다. 원래 글의 <br> 은 < 를 막아 글자 그대로 남는다.
func EscapeCell(s string) (out string, brs, dropped int) {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\n':
			b.WriteString("<br>")
			brs++
		case r == '\t':
			b.WriteByte(' ')
		case r == '\r' || r < 0x20 || r == 0x7f:
			dropped++
		case strings.ContainsRune(cellEscapes, r):
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String(), brs, dropped
}

// TopBlock 은 문서 최상위(목록·인용·컨테이너 밖)의 표 또는 :::chart 하나다. 줄 번호는 TableAt 과 같다(1부터, frontmatter 포함).
type TopBlock struct {
	// Kind 는 "table"(표)·"chart"(:::chart 컨테이너)·"html"(HTML 블록 — 자료 표 앞의 기록 주석 줄이 이것이다)이다.
	Kind string
	// Start·End 는 블록의 줄 범위다. 표는 머리 행부터 캡션 끝(캡션이 없으면 마지막 표 줄)까지, 차트는 여는 줄부터 닫는 줄까지,
	// HTML 블록은 첫 줄부터 끝 줄까지다. 닫히지 않은 차트는 Closed 가 거짓이고 End 가 0 이다.
	Start, End int
	Closed     bool
	// HasTable 은 Table 이 있는지다 — 표는 늘 참이고, 차트는 안에 표가 있을 때만(첫 표 — 렌더가 그리는 표와 같다) 참이다.
	HasTable bool
	Table    Table
	// ChartAttrs 는 차트 여는 줄의 key=value 속성(적힌 순서, 키는 소문자)이다.
	ChartAttrs [][2]string
}

// TopBlockAt 은 원문 line 줄이 든 최상위 표·:::chart·HTML 블록을 읽는다. 그 줄이 최상위의 다른 블록(문단·목록·코드 등)이거나
// 표·차트가 다른 블록 안에 있으면 false 다. 렌더와 같은 파서로 읽는다.
func TopBlockAt(src []byte, line int) (TopBlock, bool) {
	src = normalize(src)
	meta, body, _, _ := parseFrontmatter(src, nil)
	st := newState(body, meta, Options{})
	doc, over := st.parseGuarded(body)
	if over > 0 || line < 1 || line > len(st.lines) {
		return TopBlock{}, false
	}
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		switch n := n.(type) {
		case *extast.Table:
			t := st.readTable(n)
			if line >= t.Start && line <= max(t.End, t.CaptionEnd) {
				return TopBlock{Kind: "table", Start: t.Start, End: max(t.End, t.CaptionEnd), Closed: true, HasTable: true, Table: t}, true
			}
		case *ast.HTMLBlock:
			if n.Lines().Len() == 0 {
				continue
			}
			start := st.lines.line(n.Lines().At(0).Start)
			end := st.lines.line(n.Lines().At(n.Lines().Len() - 1).Start)
			if n.HasClosure() {
				end = st.lines.line(n.ClosureLine.Start)
			}
			if line >= start && line <= end {
				return TopBlock{Kind: "html", Start: start, End: end, Closed: true}, true
			}
		case *Container:
			if n.Name != "chart" {
				continue
			}
			start := st.lines.line(n.Pos())
			if line < start || (n.endLine > 0 && line > n.endLine) || (n.endLine == 0 && !lastTop(n)) {
				continue
			}
			b := TopBlock{Kind: "chart", Start: start, End: n.endLine, Closed: n.closed && n.endLine > 0, ChartAttrs: n.Attrs.Data}
			for c := n.FirstChild(); c != nil; c = c.NextSibling() {
				if t, ok := c.(*extast.Table); ok {
					b.Table, b.HasTable = st.readTable(t), true
					break
				}
			}
			return b, true
		}
	}
	return TopBlock{}, false
}

// lastTop 은 n 이 문서의 마지막 최상위 블록인지다 — 닫히지 않은 컨테이너는 문서 끝까지 이어지므로 그 뒤 줄도 그것의 것이다.
func lastTop(n ast.Node) bool { return n.NextSibling() == nil }

// ChartNumber 는 차트 렌더러가 칸 글 s 를 unit 속성 아래에서 읽는 값이다(렌더러의 parseChartNumber 그대로 — 끝의 단위·% 떼기,
// 쉼표 무시). 읽지 못하거나 빈 값이면 ok 가 거짓이다. 발행기가 unit 속성이 값 읽기를 바꾸는지 미리 보는 데 쓴다.
func ChartNumber(s, unit string) (v float64, ok bool) {
	v, _, _, ok, _ = parseChartNumber(s, unit)
	return v, ok
}

// ChartMaxSeries·ChartMaxSlices 는 차트가 그리는 계열 수·원 차트 조각 수의 상한이다(넘으면 렌더러가 자르고 경고한다).
func ChartMaxSeries() int { return chartMaxSeries }
func ChartMaxSlices() int { return chartMaxSlices }

// InsertSpot 은 커서 줄에 최상위 블록 하나(자료 표 — #365 ③ 조각 C)를 넣을 자리다. Line 은 넣을 줄이다 — 그 줄 머리에 넣는다
// (줄 수 + 1 이면 문서 끝). Kind 는 넣지 못할 때 커서가 든 블록의 꼴이다: "list"(목록·정의 목록)·"quote"(인용·알림)·
// "container"(::: 컨테이너 — 다단·차트 포함)·"footnote"(각주 정의 — 참조 여부·적은 자리와 무관)·"frontmatter"·"boundary"(넣어 보면 기존 글이 달리 읽히는 자리)·"complex"(중첩 상한을 넘어 읽지 못한 문서).
type InsertSpot struct {
	Line int
	Kind string
}

// InsertSpotAt 은 원문 line 줄(커서 줄)에 최상위 블록을 넣을 자리를 정한다. 커서 줄이 최상위 블록 사이(빈 줄·문서 끝)면 그 줄이고,
// 최상위 문단·제목·표(캡션 포함)·코드·HTML 블록 따위 안이면 그 블록 끝 다음 줄이다 — 문단 중간을 가르지 않는다. 블록 끝은 다음
// 최상위 블록 앞의 빈 줄을 뺀 마지막 줄이다. 목록·인용·컨테이너·각주 정의·frontmatter 안이면 ok 가 거짓이다(옮겨 넣지 않는다 — 예측 못 한
// 자리에 들어가지 않게). 렌더와 같은 파서로 읽는다. 읽기 전용 — 렌더 경로는 쓰지 않는다.
//
// 넣은 결과가 정말 최상위 블록이 되는지(닫히지 않은 코드·HTML 블록이 삼키지 않는지)는 이 함수가 보증하지 않는다 — 호출자가 넣은
// 글을 다시 읽어 확인한다.
func InsertSpotAt(src []byte, line int) (spot InsertSpot, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			spot, ok = InsertSpot{Kind: "complex"}, false // 읽기 전용 판정이 예기치 못하게 멈추면 넣지 않는다(바인딩이 부른다 — 퍼징이 찾은 panic 뒤)
		}
	}()
	p, ok := parseSpots(src)
	if !ok {
		return InsertSpot{Kind: "complex"}, false // 중첩 상한을 넘었거나 각주 정의 구간을 다 읽지 못했다 — 넣지 않는다
	}
	st, doc, n, blank := p.st, p.doc, len(p.st.lines), p.blank
	if line < 1 || line > n {
		return InsertSpot{}, false
	}
	if p.inFrontmatter(line) {
		return InsertSpot{Kind: "frontmatter"}, false
	}
	// 각주 정의는 파서가 받아들인 구간으로 판정한다(footnote_spans.go — 접근 리뷰 ⓐ). 최종 AST 는 정의를 문서 끝 목록으로 옮기고
	// 참조되지 않거나 같은 이름에서 뽑히지 않은 정의는 지워서, 원래 자리·소속을 알 수 없다. 정의 안의 커서는 어디에 적었든 먼저 거부한다.
	spans := p.spans
	for _, sp := range spans {
		if sp.start <= line && line <= sp.end {
			return InsertSpot{Kind: "footnote"}, false
		}
	}
	type top struct {
		n     ast.Node
		start int
		end   int // 각주 정의만 — 파서가 정한 끝. 0 이면 다음 블록 시작 앞까지
	}
	var tops []top
	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		if _, ok := c.(*extast.FootnoteList); ok {
			continue // 정의의 원래 자리는 위 구간이 맡는다
		}
		if s := st.blockLine(c); s > 0 {
			tops = append(tops, top{n: c, start: s})
		}
	}
	// 문서 바로 아래에서 연 정의는 최상위 블록이다 — 앞 블록(문단 등)의 끝을 그 앞에서 끊는다. 다른 블록 안에서 연 정의는 그 블록의 구간에 든다
	for _, sp := range spans {
		if sp.topLevel {
			tops = append(tops, top{n: nil, start: sp.start, end: sp.end})
		}
	}
	slices.SortStableFunc(tops, func(a, b top) int { return a.start - b.start })
	// span 은 tops[i] 의 원문 구간 끝 줄과 다음 블록 번호다 — 다음 블록 시작 직전에서 뒤쪽 빈 줄을 뺀 곳(표는 캡션까지)
	span := func(i int) (end, next int) {
		for {
			t := tops[i]
			next = i + 1
			if t.n == nil {
				return t.end, next
			}
			capEnd := 0
			if tb, ok := t.n.(*extast.Table); ok {
				capEnd = st.readTable(tb).CaptionEnd // 캡션 문단은 다음 형제다 — 표와 한 블록으로 본다
				for next < len(tops) && tops[next].start <= capEnd {
					next++
				}
			}
			end = n
			if next < len(tops) {
				end = tops[next].start - 1
			}
			for end > t.start && blank(end) {
				end--
			}
			end = max(end, capEnd)
			// 빈 줄 없이 바로 붙은 다음 블록이 네 칸 넘게 들여 쓴 줄로 시작하면 원문에서는 이 블록의 이어짐이다(링크 참조 정의를 떼고 남은
			// 문단 — `[a]: /u\n    글`). 사이에 넣으면 그 줄이 빈 줄 뒤의 들여 쓴 코드가 된다 — 한 블록으로 묶는다(퍼징 발견)
			if next < len(tops) && tops[next].n != nil && tops[next].start == end+1 && indentCols(p.line(end+1)) >= 4 {
				i = next
				continue
			}
			return end, next
		}
	}
	at := line
	for i := 0; i < len(tops); i++ {
		t := tops[i]
		if line < t.start {
			break // 앞 블록과 이 블록 사이 — 아래에서 그 줄
		}
		end, j := span(i)
		if line > end {
			i = j - 1
			continue
		}
		switch t.n.(type) {
		case nil: // 각주 정의 — 위에서 거부했다
			return InsertSpot{Kind: "footnote"}, false
		case *ast.List, *extast.DefinitionList:
			return InsertSpot{Kind: "list"}, false
		case *ast.Blockquote:
			return InsertSpot{Kind: "quote"}, false
		case *Container:
			return InsertSpot{Kind: "container"}, false
		}
		at = end + 1
		break
	}
	// 넣는 세 층과 꼴이 같은 글(spotProbe)로 그 자리의 경계를 본다. 실제로 넣을 때는 호출자가 마지막 자리·실제 글로 InsertFitsAt 을 다시 부른다
	if kind := p.fitsAt(at, spotProbe); kind != "" {
		return InsertSpot{Kind: kind}, false
	}
	return InsertSpot{Line: at}, true
}

// InsertFitsAt 은 원문 line 줄 머리(줄 수 + 1 이면 글 끝)에 block(datareport.Block.Text() 꼴 — 기록 주석 한 줄·표 또는 :::chart·
// 빈 줄·캡션)을 그대로 넣어도 되는지다. 자리를 옮기지 않는다 — 받으면 spot.Line 이 line 이고, 아니면 Kind 가 거부 꼴이다(InsertSpot).
// InsertSpotAt 이 정한 자리를 호출자가 더 옮겼거나(기록 주석 줄 커서 → 그 블록 뒤) 넣을 실제 글이 탐침과 다를 때, 넣기 직전에 부른다
// (리뷰 3회차 [1]). 읽기 전용.
func InsertFitsAt(src []byte, line int, block string) (spot InsertSpot, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			spot, ok = InsertSpot{Kind: "complex"}, false
		}
	}()
	p, ok := parseSpots(src)
	if !ok {
		return InsertSpot{Kind: "complex"}, false
	}
	if line < 1 || line > len(p.st.lines)+1 {
		return InsertSpot{}, false
	}
	if p.inFrontmatter(line) {
		return InsertSpot{Kind: "frontmatter"}, false
	}
	if kind := p.fitsAt(line, block); kind != "" {
		return InsertSpot{Kind: kind}, false
	}
	return InsertSpot{Line: line}, true
}

// inFrontmatter 는 l 줄이 frontmatter 안(닫는 줄 포함)인지다. body 는 frontmatter 를 줄바꿈만 남겨 줄 번호만 맞춘 글이다 — 바이트
// 위치는 원문에서 센다.
func (p spotParse) inFrontmatter(l int) bool {
	if !p.meta.Present {
		return false
	}
	_, end, ok := splitFrontmatter(p.src)
	return ok && end > 0 && l <= bytes.Count(p.src[:end-1], []byte("\n"))+1
}

// fitsAt 은 at 줄 머리에 block 을 넣는 경계 판정이다 — 받으면 "", 아니면 거부 꼴. 커서에서 자리를 정하는 일(InsertSpotAt)과 떼어,
// 마지막 자리·실제 글로 다시 부를 수 있게 한다.
func (p spotParse) fitsAt(at int, block string) string {
	n := len(p.st.lines)
	// 각주 정의의 첫 줄 앞은 최상위 경계다(`본문\n[^a]: 끊음`). 그 다음 줄부터 끝 줄까지는 정의 안이다
	for _, sp := range p.spans {
		if sp.start < at && at <= sp.end {
			return "footnote"
		}
	}
	// 빈 줄 없이 바로 붙은 앞 줄 뒤의 네 칸 넘게 들여 쓴 줄은 앞 블록의 이어짐이다 — 사이에 넣으면 그 줄이 들여 쓴 코드가 된다(InsertSpotAt 의 span 묶음과 같은 규칙)
	if at > 1 && at <= n && !p.blank(at-1) && !p.blank(at) && indentCols(p.line(at)) >= 4 {
		return "boundary"
	}
	// 넣는 세 층은 캡션 문단으로 끝난다 — 그 뒤 첫 내용 줄이 정의 표지(`: `)면 빈 줄이 몇이든 캡션이 용어가 되어 그 줄을 정의로 삼킨다
	// (goldmark 정의 목록, 퍼징 발견). 옮겨 넣지 않고 거부한다
	for l := at; l <= n; l++ {
		if !p.blank(l) {
			if definitionMarkRe.MatchString(p.line(l)) {
				return "list"
			}
			break
		}
	}
	// 그 글을 그 자리에 넣어 본다 — 새 표·차트가 최상위이고 기존 글(블록 꼴·조상·순서·글, 각주 정의 구간)이 그대로일 때만 받는다.
	// 위 규칙이 못 본 경계 상호작용(빈 줄 없이 붙은 다음 줄이 앞 블록 해석을 바꾸는 따위)은 옮기지 않고 거부한다
	if !p.keepsAt(at, block) {
		return "boundary"
	}
	return ""
}

// InsertText 는 src 의 line 줄 머리(줄 수 + 1 이면 글 끝)에 block(줄바꿈으로 끝나는 블록 글)을 넣을 바이트 위치와 넣을 글(앞뒤 빈 줄
// 포함), 끝에 붙인 줄바꿈 수를 정한다. 앞뒤 빈 줄은 편집기의 blockAt(CSV 표 넣기 — editor-cm6.js)과 같은 규칙이다. src 는 LF 글이다.
func InsertText(src string, line int, block string) (pos int, text string, tail int) {
	lines := strings.Split(src, "\n")
	pos = len(src)
	if line <= len(lines) {
		pos = 0
		for i := 0; i < line-1; i++ {
			pos += len(lines[i]) + 1
		}
		pos = min(pos, len(src))
	}
	before, after := src[:pos], src[pos:]
	pre := "\n\n"
	switch {
	case pos == 0, strings.HasSuffix(before, "\n\n"):
		pre = ""
	case strings.HasSuffix(before, "\n"):
		pre = "\n"
	}
	post := "\n\n"
	switch {
	case strings.HasPrefix(after, "\n\n"):
		post = ""
	case strings.HasPrefix(after, "\n"):
		post = "\n"
	case after == "":
		post = "\n"
	}
	return pos, pre + block + post, len(post)
}

var definitionMarkRe = regexp.MustCompile(`^ {0,3}:(?:[ \t]|$)`)
