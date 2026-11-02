package mdoc

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
)

// 문서 창의 표 편집 그리드(#355). 원문의 GFM 표 하나를 칸 글(마크다운 원문 그대로)·열 정렬·캡션으로 읽고(TableAt),
// 고친 것을 열 너비를 맞춘 표 원문으로 쓴다(FormatTable). 렌더와 같은 파서(컨테이너·CJK 확장)로 읽으므로
// :::chart·:::approval·:::actions·목록·인용 안 표도 찾고, 칸 나누기(\| ·코드 스팬 안 |)가 렌더와 같다.

// Table 은 원문의 표 하나다. 줄 번호는 1부터 센 원문 줄이다(frontmatter 포함 — 편집기 줄과 같다).
type Table struct {
	Start int `json:"start"` // 머리 행 줄
	End   int `json:"end"`   // 마지막 표 줄(본문 행이 없으면 구분 행)
	// Header 는 머리 칸, Align 은 열마다 ""(없음)·"left"·"center"·"right", Rows 는 본문 행이다. 칸 글은 원문 그대로다
	// (앞뒤 공백만 뗀다 — \| ·**굵게**·<br> 는 글자 그대로). 셋의 열 수는 같다(짧은 행은 빈 칸으로 채운다).
	Header []string   `json:"header"`
	Align  []string   `json:"align"`
	Rows   [][]string `json:"rows"`
	// Caption 은 표 바로 다음 캡션 문단의 글(접두 뺌, 여러 줄이면 \n 으로 잇는다 — 줄 끝 공백(강제 줄바꿈)은 둔다), CaptionPrefix 는 원문의 접두
	// ("Table:"·"table:"·":", 캡션이 없으면 ""). CaptionStart·CaptionEnd 는 캡션 문단의 줄 범위(없으면 0).
	Caption       string `json:"caption"`
	CaptionPrefix string `json:"caption_prefix"`
	CaptionStart  int    `json:"caption_start"`
	CaptionEnd    int    `json:"caption_end"`
	// InChart 는 :::chart 바로 안의 표인지다 — 새 캡션의 접두가 ":" 다.
	InChart bool `json:"in_chart"`
	// HeadPrefix 는 머리 행 줄 앞의 컨테이너 표시(인용의 "> "·목록 항목의 "- " 등), Prefix 는 그 밖의 줄(구분 행·본문 행·캡션) 앞의 것이다.
	HeadPrefix string `json:"head_prefix"`
	Prefix     string `json:"prefix"`
	// Notes 는 읽으며 바꿔 보인 것(머리 행보다 긴 행 — 열을 늘림)이다.
	Notes []string `json:"notes"`
	// Tabs 는 GFM 표가 아니라 탭으로 나뉜 줄들을 읽은 것인지다(TabTable·TabBlockAt, #579) — 화면은 그 줄 범위를 표로 바꾸며 앞뒤 빈 줄을 보충한다.
	Tabs bool `json:"tabs"`
}

var alignNames = map[extast.Alignment]string{extast.AlignLeft: "left", extast.AlignCenter: "center", extast.AlignRight: "right"}

// TableAt 은 원문 line 줄이 든 표를 읽는다. 표 줄(머리·구분·본문 행)과 바로 아래 캡션 문단 줄이 그 표의 줄이다. 없으면 false.
func TableAt(src []byte, line int) (Table, bool) {
	src = normalize(src)
	meta, body, _, _ := parseFrontmatter(src, nil)
	st := newState(body, meta, Options{})
	doc, over := st.parseGuarded(body)
	if over > 0 || line < 1 {
		return Table{}, false
	}
	var found *extast.Table
	var out Table
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		t, ok := n.(*extast.Table)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		tb := st.readTable(t)
		last := max(tb.End, tb.CaptionEnd)
		if line >= tb.Start && line <= last {
			found, out = t, tb
			return ast.WalkStop, nil
		}
		return ast.WalkSkipChildren, nil
	})
	return out, found != nil
}

func (st *state) readTable(t *extast.Table) Table {
	tb := Table{Header: []string{}, Align: []string{}, Rows: [][]string{}, Notes: []string{}}
	for _, a := range t.Alignments {
		tb.Align = append(tb.Align, alignNames[a])
	}
	cols := len(t.Alignments)
	long := 0
	for r := t.FirstChild(); r != nil; r = r.NextSibling() {
		var cs []string
		for _, c := range cells(r) {
			cs = append(cs, st.cellSource(c))
		}
		switch r := r.(type) {
		case *extast.TableHeader:
			tb.Header = cs
			tb.Start = st.tableLine(t)
			tb.End = tb.Start + 1 // 구분 행
			tb.HeadPrefix = string(st.src[st.lines[tb.Start-1]:r.Pos()])
			tb.Prefix = delimPrefix(st.lineBytes(tb.Start + 1))
		case *extast.TableRow:
			// goldmark 는 머리 행보다 많은 칸을 버린다 — 남는 칸은 줄을 같은 규칙으로 다시 나눠 살린다(#355 결정 5)
			if extra := splitRow(st.src[r.Pos():st.lineEnd(r.Pos())]); len(extra) > len(cs) {
				cs = append(cs, extra[len(cs):]...)
				long++
			}
			tb.Rows = append(tb.Rows, cs)
			tb.End = st.lines.line(r.Pos())
		}
		cols = max(cols, len(cs))
	}
	if long > 0 {
		tb.Notes = append(tb.Notes, fmt.Sprintf("머리 행보다 긴 행 %d개 — 열을 늘려 보입니다", long))
	}
	tb.Header = pad(tb.Header, cols)
	tb.Align = pad(tb.Align, cols)
	for i := range tb.Rows {
		tb.Rows[i] = pad(tb.Rows[i], cols)
	}
	if c, ok := t.Parent().(*Container); ok && c.Name == "chart" {
		tb.InChart = true
	}
	// 캡션: 다음 형제 문단의 첫 줄이 Table: · : 로 시작하면(transform.go tableCaption 과 같은 판정)
	if p, ok := t.NextSibling().(*ast.Paragraph); ok && p.Lines().Len() > 0 {
		first := p.Lines().At(0)
		v := first.Value(st.src)
		if loc := captionRe.FindIndex(v); loc != nil {
			tb.CaptionPrefix = strings.TrimSpace(string(v[:loc[1]]))
			// 줄 종결자만 뗀다 — 줄 끝 공백 둘은 강제 줄바꿈(<br>)이라 지우면 렌더가 바뀐다(#355 리뷰 [2])
			parts := []string{strings.TrimRight(string(v[loc[1]:]), "\r\n")}
			for i := 1; i < p.Lines().Len(); i++ {
				s := p.Lines().At(i)
				parts = append(parts, strings.TrimRight(string(s.Value(st.src)), "\r\n"))
			}
			tb.Caption = strings.Join(parts, "\n")
			tb.CaptionStart = st.lines.line(first.Start)
			tb.CaptionEnd = st.lines.line(p.Lines().At(p.Lines().Len() - 1).Start)
		}
	}
	return tb
}

// tableLine 은 표의 머리 행 원문 줄(1부터, frontmatter 포함)이다 — TableAt 의 Start 와 렌더의 data-table-line(transform.go)이 함께 쓴다.
// 머리 행이 없으면 0. 캡션을 옮긴 표(transform.go tableCaption)는 첫 자식이 캡션이라 머리 행을 찾아 본다.
func (st *state) tableLine(t *extast.Table) int {
	for c := t.FirstChild(); c != nil; c = c.NextSibling() {
		if h, ok := c.(*extast.TableHeader); ok {
			return st.lines.line(h.Pos())
		}
	}
	return 0
}

// cellSource 는 칸의 원문이다 — goldmark 가 앞뒤 공백을 뗀 세그먼트(\| 는 그대로). 짧은 행을 채운 빈 칸은 세그먼트가 없다.
func (st *state) cellSource(c ast.Node) string {
	if c.Lines().Len() == 0 {
		return ""
	}
	s := c.Lines().At(0)
	return string(s.Value(st.src))
}

func (st *state) lineEnd(pos int) int {
	if i := bytes.IndexByte(st.src[pos:], '\n'); i >= 0 {
		return pos + i
	}
	return len(st.src)
}

// lineBytes 는 n 줄(1부터)의 글이다(줄 끝 \n 뺌).
func (st *state) lineBytes(n int) []byte {
	if n < 1 || n > len(st.lines) {
		return nil
	}
	s := st.lines[n-1]
	return st.src[s:st.lineEnd(s)]
}

// delimPrefix 는 구분 행 앞의 컨테이너 표시(공백·탭·>)다. 구분 행 글에는 > 가 없다.
func delimPrefix(b []byte) string {
	i := 0
	for i < len(b) && (b[i] == ' ' || b[i] == '\t' || b[i] == '>') {
		i++
	}
	return string(b[:i])
}

// splitRow 는 표 한 줄을 goldmark 표 확장(parseRow)과 같은 규칙으로 칸 원문으로 나눈다: 앞뒤 공백·앞뒤 | 를 떼고, 앞 글자가 \ 가
// 아닌 | 에서 나눈다(코드 스팬 안의 | 도 나눈다 — GFM). 칸마다 앞뒤 공백을 뗀다.
func splitRow(line []byte) []string {
	line = bytes.TrimSpace(line)
	if len(line) > 0 && line[0] == '|' {
		line = line[1:]
	}
	if len(line) > 0 && line[len(line)-1] == '|' {
		line = line[:len(line)-1]
	}
	var out []string
	start := 0
	for i := 0; i <= len(line); i++ {
		if i == len(line) || (line[i] == '|' && (i == 0 || line[i-1] != '\\')) {
			out = append(out, string(bytes.TrimSpace(line[start:i])))
			start = i + 1
		}
	}
	if len(line) == 0 {
		return nil
	}
	return out
}

func pad(s []string, n int) []string {
	for len(s) < n {
		s = append(s, "")
	}
	return s
}

// tableWidth 는 칸 글의 표시 폭이다. East Asian ambiguous(·, ①, ─ …)는 좁게(1) 센다 — 고정폭 편집기 글꼴이 대개 그렇다.
var tableWidth = &runewidth.Condition{EastAsianWidth: false}

// FormatCell 은 그리드 칸의 글을 표 칸 원문으로 바꾼다. 바꾸는 것은 셋뿐이다(#355 결정 3): 양끝 공백 제거, 칸 안 줄바꿈 → <br>,
// 앞 글자가 \ 가 아닌 | → \|. 다른 글자(**·[·< …)는 원문이므로 막지 않는다.
func FormatCell(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n"))
	if !strings.ContainsAny(s, "\n|") {
		return s
	}
	var b strings.Builder
	prev := rune(0)
	for _, r := range s {
		switch {
		case r == '\n':
			b.WriteString("<br>")
		case r == '|' && prev != '\\':
			b.WriteString(`\|`)
		default:
			b.WriteRune(r)
		}
		prev = r
	}
	return b.String()
}

// 표 쓰기 예산(#355 리뷰 [3]) — WriteTable 이 쓰기 전에 센다. 가장 긴 칸 폭으로 모든 행을 채우므로 칸 하나가 길고 행이 많으면
// 맞춘 출력이 입력보다 훨씬 커진다. 시험이 작게 바꾼다.
var (
	tableMaxCells = 200_000 // 열 수 × (본문 행 + 머리 행)
	tableMaxIn    = 8 << 20 // 칸·캡션·접두 글 바이트 합
	tableMaxOut   = 8 << 20 // 쓴 표 원문 바이트(문서 창의 문서 상한과 같다)
)

// ErrTableTooBig 은 표가 예산을 넘어 쓰지 못할 때다.
var ErrTableTooBig = errors.New("표가 너무 커서 쓰지 못함")

// TableText 는 WriteTable 의 결과다. Note 는 열 너비를 맞추지 않았을 때의 알림이다(맞췄으면 "").
type TableText struct {
	Markdown string `json:"markdown"`
	Note     string `json:"note"`
}

// WriteTable 은 FormatTable 에 예산을 씌운 것이다(문서 창 TableMarkdown 이 부른다). 칸 수·입력 바이트가 예산을 넘으면
// ErrTableTooBig. 열을 맞춘 출력이 tableMaxOut 을 넘을 것이면 만들기 전에 알고 맞춤 없이(| a | b | 꼴, 구분 행 최소 너비) 쓰고
// Note 로 알린다. 그것도 넘으면 ErrTableTooBig.
func WriteTable(t Table) (TableText, error) {
	cols := tableCols(t)
	if len(t.Rows) >= tableMaxCells/cols { // cols × (행 + 1) > tableMaxCells 를 곱하지 않고 센다
		return TableText{}, ErrTableTooBig
	}
	in := len(t.Caption) + len(t.CaptionPrefix) + len(t.HeadPrefix) + len(t.Prefix)
	for _, r := range append([][]string{t.Header}, t.Rows...) {
		for _, c := range r {
			if in += len(c); in > tableMaxIn {
				return TableText{}, ErrTableTooBig
			}
		}
	}
	g := newTableGrid(t, cols)
	if g.size(true) <= tableMaxOut {
		return TableText{Markdown: g.format(true)}, nil
	}
	if g.size(false) <= tableMaxOut {
		return TableText{Markdown: g.format(false), Note: "표가 커서 열 너비를 맞추지 않음"}, nil
	}
	return TableText{}, ErrTableTooBig
}

// FormatTable 은 t 를 표 원문으로 쓴다 — 표 줄과(캡션이 있으면) 빈 줄 하나 뒤의 캡션 문단. 끝 줄바꿈은 없다.
// 열 너비는 칸의 표시 폭에 맞추고(최소 3), 정렬 행도 그 너비다. 오른쪽 정렬 열은 왼쪽을, 가운데 정렬 열은 양쪽을 채운다.
// 캡션 접두는 원래 것(CaptionPrefix)을 지키고, 없으면 차트 안은 ":", 밖은 "Table:" 이다. 캡션이 공백뿐이면 캡션 문단을 쓰지 않는다.
// 캡션 첫 줄은 왼쪽만 다듬고 줄 끝 공백(강제 줄바꿈)과 뒤 줄은 그대로 둔다. 크기 예산은 보지 않는다 — 화면이 보낸 표는 WriteTable 로.
func FormatTable(t Table) string {
	return newTableGrid(t, tableCols(t)).format(true)
}

func tableCols(t Table) int {
	cols := max(len(t.Header), len(t.Align), 1)
	for _, r := range t.Rows {
		cols = max(cols, len(r))
	}
	return cols
}

// TransposeTable 은 표를 전치한다(#468) — [Header, ...Rows] 행렬을 뒤집는다. 왼쪽 위 칸(Header[0])은 제자리다
// (새 머리 행 = 옛 첫 열, 새 본문 행 = 옛 2열 이후). 열 정렬은 새 첫 열이 옛 첫 열의 정렬을 물려받고, 나머지 새 열은
// 옛 2열 이후의 정렬이 모두 같으면 그 값, 아니면 없음("")이다. 캡션·접두·줄 범위는 그대로 — 화면은 그 표 줄만 바꿔 넣는다.
func TransposeTable(t Table) Table {
	rows := append([][]string{t.Header}, t.Rows...) // 원본 행렬(머리 행이 0행)
	cols := tableCols(t)
	out := t
	out.Header = make([]string, len(rows))
	out.Rows = make([][]string, 0, max(cols-1, 0))
	for c := 0; c < cols; c++ { // 새 머리 행 = 옛 0열, 새 본문 행 = 옛 1열 이후
		col := make([]string, len(rows))
		for r := range rows {
			if c < len(rows[r]) {
				col[r] = rows[r][c]
			}
		}
		if c == 0 {
			out.Header = col
		} else {
			out.Rows = append(out.Rows, col)
		}
	}
	rest, same := "", true // 옛 2열 이후의 정렬이 모두 같은지
	if len(t.Align) > 1 {
		rest = t.Align[1]
		for _, a := range t.Align[2:] {
			if a != rest {
				same = false
				break
			}
		}
	}
	out.Align = make([]string, len(rows))
	if len(t.Align) > 0 {
		out.Align[0] = t.Align[0]
	}
	if same {
		for i := 1; i < len(out.Align); i++ {
			out.Align[i] = rest
		}
	}
	out.Notes = nil // 읽으며 단 알림(머리 행보다 긴 행)은 전치 뒤 성립하지 않는다
	return out
}

// tableGrid 는 쓸 표다 — 칸은 FormatCell 을 거친 글, w 는 칸마다 표시 폭, width 는 열 너비(최소 3), caption 은 캡션 문단 줄(앞 빈 줄 포함).
type tableGrid struct {
	t       Table
	rows    [][]string // 0 은 머리 행
	w       [][]int
	align   []string
	width   []int
	caption []string
}

func newTableGrid(t Table, cols int) *tableGrid {
	g := &tableGrid{t: t, align: make([]string, cols), width: make([]int, cols)}
	copy(g.align, t.Align)
	for i := range g.width {
		g.width[i] = 3
	}
	for _, r := range append([][]string{t.Header}, t.Rows...) {
		out, ws := make([]string, cols), make([]int, cols)
		for i := range out {
			if i < len(r) {
				out[i] = FormatCell(r[i])
			}
			ws[i] = tableWidth.StringWidth(out[i])
			g.width[i] = max(g.width[i], ws[i])
		}
		g.rows, g.w = append(g.rows, out), append(g.w, ws)
	}
	if c := strings.ReplaceAll(strings.ReplaceAll(t.Caption, "\r\n", "\n"), "\r", "\n"); strings.TrimSpace(c) != "" {
		prefix := t.CaptionPrefix
		if prefix == "" {
			prefix = "Table:"
			if t.InChart {
				prefix = ":"
			}
		}
		g.caption = append(g.caption, strings.TrimRight(t.Prefix, " \t"))
		for i, l := range strings.Split(strings.Trim(c, "\n"), "\n") {
			if i == 0 {
				l = prefix + " " + strings.TrimLeft(l, " \t")
			}
			g.caption = append(g.caption, t.Prefix+l)
		}
	}
	return g
}

// size 는 format(aligned) 의 바이트 수다 — 글을 만들지 않고 센다. tableMaxOut 을 넘으면 거기서 멈추고 그보다 큰 값을 돌려준다.
func (g *tableGrid) size(aligned bool) int {
	n := 0
	for r, row := range g.rows {
		p := len(g.t.Prefix)
		if r == 0 {
			p = len(g.t.HeadPrefix)
		}
		n += p + 2 // 접두·첫 | ·줄바꿈
		for i, c := range row {
			n += len(c) + 3
			if aligned {
				n += g.width[i] - g.w[r][i]
			}
		}
		if n > tableMaxOut {
			return n
		}
	}
	n += len(g.t.Prefix) + 2 // 구분 행
	for _, w := range g.width {
		if !aligned {
			w = 3
		}
		n += w + 3
	}
	for _, l := range g.caption {
		n += len(l) + 1
	}
	return n - 1 // 끝 줄바꿈 없음
}

// format 은 표 원문을 쓴다. aligned 가 아니면 칸을 채우지 않고 구분 행은 최소 너비(---·:--·:-:·--:)다.
func (g *tableGrid) format(aligned bool) string {
	width := g.width
	if !aligned {
		width = make([]int, len(g.width))
		for i := range width {
			width[i] = 3
		}
	}
	line := func(r int) string {
		var b strings.Builder
		b.WriteString("|")
		for i, c := range g.rows[r] {
			gap, left := 0, 0
			if aligned {
				gap = width[i] - g.w[r][i]
			}
			switch g.align[i] {
			case "right":
				left = gap
			case "center":
				left = gap / 2
			}
			b.WriteString(" " + strings.Repeat(" ", left) + c + strings.Repeat(" ", gap-left) + " |")
		}
		return b.String()
	}
	var lines []string
	lines = append(lines, g.t.HeadPrefix+line(0))
	var d strings.Builder
	d.WriteString("|")
	for i, w := range width {
		switch g.align[i] {
		case "left":
			d.WriteString(" :" + strings.Repeat("-", w-1) + " |")
		case "center":
			d.WriteString(" :" + strings.Repeat("-", w-2) + ": |")
		case "right":
			d.WriteString(" " + strings.Repeat("-", w-1) + ": |")
		default:
			d.WriteString(" " + strings.Repeat("-", w) + " |")
		}
	}
	lines = append(lines, g.t.Prefix+d.String())
	for r := 1; r < len(g.rows); r++ {
		lines = append(lines, g.t.Prefix+line(r))
	}
	return strings.Join(append(lines, g.caption...), "\n")
}
