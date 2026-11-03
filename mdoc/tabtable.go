package mdoc

import (
	"errors"
	"strings"

	"github.com/yuin/goldmark/ast"
)

// 탭으로 나뉜 표(itda-hyve#579) — 엑셀·시트 복사본(TSV, 웹 표는 미검증)을 표 편집 그리드·표 원문으로 읽는다. 문서 창의 붙여넣기 자동 변환
// (TabPasteSpot + TabTable)과 「표 편집」의 탭 줄 경로(TabBlockAt — TableAt 이 실패했을 때)가 쓴다. 쓰기는 GFM 표와 같이
// WriteTable 이다(열 너비 맞춤, 첫 줄 = 머리 행, 정렬 없음).

// ErrNotTabTable 은 TabTable 이 받은 글이 탭으로 나뉜 표가 아닐 때다.
var ErrNotTabTable = errors.New("탭으로 나뉜 표가 아님")

// TabTable 은 탭으로 나뉜 글 text 를 표로 읽는다. 판정(모두 맞아야 표):
//   - \r\n·\r 은 \n 으로 보고 끝 줄바꿈 하나는 무시한다.
//   - 행은 줄바꿈으로, 칸은 탭으로 나눈다. 다만 따옴표 칸(quotedCell — " 로 시작하고, 짝 따옴표 바로 뒤가 탭·줄바꿈·끝이며, 그 안에
//     탭·줄바꿈·"" 중 하나가 있는 칸)은 짝 따옴표까지 한 칸이다(그 안의 탭·줄바꿈은 칸 글, "" 는 "). 그 밖의 " 는 글자 그대로다.
//   - 행이 2개 이상이고, 행마다 칸이 2개 이상(탭이 있다)이며, 모든 행의 칸 수가 같다. 탭 없는 줄(빈 줄 포함, 따옴표 칸 안의 줄은 빼고)이 있으면 표가 아니다.
//     모든 칸이 빈 행(탭·공백만 있는 줄)도 칸 수가 맞으면 행이다.
//   - 모든 행이 탭으로 시작하지는 않는다 — 그러면 탭 들여쓰기(코드·목록)로 보고 표로 읽지 않는다.
//   - 칸 수(행 × 열)가 tableMaxCells 를 넘으면 ErrTableTooBig 이다(칸을 모으지 않고 판정만 끝까지 한다).
//
// 칸 글은 FormatCell 을 거친다(앞뒤 공백 뗌, 칸 안 줄바꿈 → <br>, 앞 글자가 \ 가 아닌 | → \|). 첫 행이 머리 행, 나머지가 본문
// 행이고 정렬은 없다(""). 줄 범위(Start·End)는 0 이다 — 문서 안 자리는 TabBlockAt 이 정한다. 표가 아니면 ErrNotTabTable.
func TabTable(text string) (Table, error) {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	return tabParse(strings.TrimSuffix(text, "\n"))
}

// tabParse 는 TabTable 의 판정이다(text 는 \n 줄바꿈, 끝 줄바꿈은 뗀 뒤).
func tabParse(text string) (Table, error) {
	n := len(text)
	var rows [][]string
	cols, nrows, indented, cells := -1, 0, 0, 0
	tooBig := false
	i := 0
	for {
		if i < n && text[i] == '\t' {
			indented++
		}
		var rec []string
		fc := 0
		for {
			var f string
			if q, end, ok := quotedCell(text, i); ok { // 엑셀식 따옴표 칸
				f, i = q, end
			} else {
				j := i
				for j < n && text[j] != '\t' && text[j] != '\n' {
					j++
				}
				f, i = text[i:j], j
			}
			fc++
			cells++
			if cells > tableMaxCells {
				tooBig, rows, rec = true, nil, nil
			}
			if !tooBig {
				rec = append(rec, FormatCell(f))
			}
			if i < n && text[i] == '\t' {
				i++
				continue
			}
			break
		}
		if fc < 2 || (cols >= 0 && fc != cols) {
			return Table{}, ErrNotTabTable
		}
		cols = fc
		nrows++
		if !tooBig {
			rows = append(rows, rec)
		}
		if i >= n {
			break
		}
		i++ // \n
	}
	if nrows < 2 || indented == nrows {
		return Table{}, ErrNotTabTable
	}
	if tooBig {
		return Table{}, ErrTableTooBig
	}
	return Table{Header: rows[0], Align: make([]string, cols), Rows: rows[1:], Notes: []string{}, Tabs: true}, nil
}

// quotedCell 은 text 의 i 에서 시작하는 칸이 따옴표 칸인지다(엑셀 규칙 — 탭·줄바꿈·" 가 든 칸만 따옴표로 감싸 복사한다). 칸이 " 로
// 시작하고, 짝 따옴표("" 는 글자 ")가 있으며 그 바로 뒤가 탭·줄바꿈·끝이고, 그 안에 탭·줄바꿈·"" 중 하나가 있을 때만 참이다 — 그러면
// 칸 글(바깥 따옴표를 떼고 "" → ")과 닫는 따옴표 다음 자리를 준다. 아니면 거짓이고 칸은 글자 그대로(따옴표 포함) 다음 탭·줄바꿈까지다
// — 짝이 없는 " 로 시작한 칸이 뒤 줄을 삼키지 않는다.
func quotedCell(text string, i int) (string, int, bool) {
	n := len(text)
	if i >= n || text[i] != '"' {
		return "", 0, false
	}
	special := false
	j := i + 1
	for j < n {
		switch text[j] {
		case '"':
			if j+1 < n && text[j+1] == '"' {
				special = true
				j += 2
				continue
			}
			if !special || (j+1 < n && text[j+1] != '\t' && text[j+1] != '\n') {
				return "", 0, false
			}
			return strings.ReplaceAll(text[i+1:j], `""`, `"`), j + 1, true
		case '\t', '\n':
			special = true
		}
		j++
	}
	return "", 0, false // 짝 따옴표 없음
}

// TabBlockAt 은 원문 line 줄이 든 탭 표 줄들을 읽는다(「표 편집」 — TableAt 이 실패했을 때). 그 줄이 든 문단 하나가 대상이다:
// 문단의 조상이 문서·::: 컨테이너뿐이고(목록·인용 안은 아님), 문단의 원문 줄 전체(줄바꿈으로 이은 글)가 TabTable 의 판정을 지난다.
// 문단 바로 앞·뒤 줄이 탭을 품으면(모든 칸이 빈 행이 문단을 나눈 경우, 머리 행이 탭으로 시작해 들여 쓴 코드가 된 경우 따위 — 더 큰
// 탭 덩어리의 일부) 읽지 않는다. 판정은 문서 그대로의 파싱이라 #·-·1.·> 따위로 시작하는 탭 줄이 제목·목록·인용이 되면 그 줄은
// 문단이 아니어서 읽지 않는다(붙여넣기 판정 TabPasteSpot 과 다르다). Start·End 는 문단의 첫·끝 줄이다. 없으면 false.
func TabBlockAt(src []byte, line int) (Table, bool) {
	src = normalize(src)
	meta, body, _, _ := parseFrontmatter(src, nil)
	st := newState(body, meta, Options{})
	doc, over := st.parseGuarded(body)
	if over > 0 || line < 1 || line > len(st.lines) {
		return Table{}, false
	}
	var out Table
	found := false
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if found {
			return ast.WalkStop, nil
		}
		p, ok := n.(*ast.Paragraph)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		first, last, ok := st.blockLines(p)
		if !ok || line < first || line > last {
			return ast.WalkSkipChildren, nil
		}
		found = true
		if !plainAncestry(p) || st.tabLine(first-1) || st.tabLine(last+1) {
			return ast.WalkStop, nil
		}
		var lines []string
		for l := first; l <= last; l++ {
			lines = append(lines, string(st.lineBytes(l)))
		}
		if t, err := tabParse(strings.Join(lines, "\n")); err == nil {
			t.Start, t.End = first, last
			out = t
		}
		return ast.WalkStop, nil
	})
	return out, out.Tabs
}

// TabPasteSpot 은 붙여넣기 뒤의 원문 src 에서, first 줄부터 붙여 넣은 글 text 가 든 자리가 표로 바꿔도 되는 자리인지다(붙여넣기
// 자동 변환). last 는 text 의 마지막 글자가 든 줄이다(text 가 줄바꿈으로 끝나면 그 앞 줄 — 화면의 lineOf(to-1)). src 의 그 줄들이
// text 와 맞지 않으면 거짓이다. 판정은 두 번 한다:
//   - 붙여 넣은 글 자체의 마크다운(#·-·1.·>·= 로 시작하는 행, 탭으로 시작해 들여 쓴 코드가 되는 머리 행, 모든 칸이 빈 행 따위)에
//     휘둘리지 않게, 붙여 넣은 글의 줄마다 같은 줄 수의 중립 자리표시(x)로 바꾼 원문을 파싱한다 — first 줄의 붙여 넣기 앞 글과
//     마지막 줄의 뒤 글은 그대로 둔다. first..last 줄마다 조상이 문서·::: 컨테이너뿐인 문단의 줄이어야 한다(펜스 코드·들여 쓴 코드·
//     기존 표·HTML 블록·제목·목록·인용·frontmatter 안이면 거짓).
//   - 붙여 넣기 전 원문(그 줄 = 앞 글 + 뒤 글)에서 first 줄이 들여 쓴 코드 블록 안(블록 사이 빈 줄 포함)이면 거짓이다 — 자리표시는
//     코드 블록 사이 빈 줄을 문단으로 바꿔 놓기 때문이다.
func TabPasteSpot(src []byte, text string, first, last int) bool {
	src = normalize(src)
	text = string(normalize([]byte(text)))
	lines := strings.Split(string(src), "\n")
	tl := strings.Split(text, "\n")
	span := len(tl)
	want := first + span - 1
	if tl[span-1] == "" {
		want--
	}
	if span < 2 || first < 1 || last != want || first+span-1 > len(lines) {
		return false
	}
	end := first + span - 1 // text 가 닿은 마지막 줄(줄바꿈으로 끝나면 뒤 글만 남은 줄)
	head, tail := lines[first-1], lines[end-1]
	if !strings.HasSuffix(head, tl[0]) || !strings.HasPrefix(tail, tl[span-1]) {
		return false
	}
	for k := 1; k < span-1; k++ {
		if lines[first-1+k] != tl[k] {
			return false
		}
	}
	prefix, suffix := head[:len(head)-len(tl[0])], tail[len(tl[span-1]):]
	// 붙여 넣기 전 원문의 그 줄이 들여 쓴 코드 블록 안이면 거짓
	pre := strings.Join(append(append(append([]string{}, lines[:first-1]...), prefix+suffix), lines[end:]...), "\n")
	if inIndentedCode([]byte(pre), first) {
		return false
	}
	neutral := append([]string{}, lines...)
	for k := 0; k < span; k++ {
		l := ""
		if k < span-1 || tl[k] != "" {
			l = "x"
		}
		if k == 0 {
			l = prefix + l
		}
		if k == span-1 {
			l += suffix
		}
		neutral[first-1+k] = l
	}
	meta, body, _, _ := parseFrontmatter([]byte(strings.Join(neutral, "\n")), nil)
	st := newState(body, meta, Options{})
	doc, over := st.parseGuarded(body)
	if over > 0 || last > len(st.lines) {
		return false
	}
	ok := make([]bool, last-first+1) // 받는 문단이 덮은 줄
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || n.Type() != ast.TypeBlock {
			return ast.WalkContinue, nil
		}
		if _, para := n.(*ast.Paragraph); !para {
			return ast.WalkContinue, nil
		}
		if s, e, has := st.blockLines(n); has && plainAncestry(n) {
			for l := max(s, first); l <= min(e, last); l++ {
				ok[l-first] = true
			}
		}
		return ast.WalkSkipChildren, nil
	})
	for _, b := range ok {
		if !b {
			return false
		}
	}
	return true
}

// inIndentedCode 는 src 의 line 줄이 들여 쓴 코드 블록의 첫·끝 줄 사이(블록 안 빈 줄 포함)인지다.
func inIndentedCode(src []byte, line int) bool {
	meta, body, _, _ := parseFrontmatter(src, nil)
	st := newState(body, meta, Options{})
	doc, over := st.parseGuarded(body)
	if over > 0 {
		return true
	}
	in := false
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || in {
			return ast.WalkContinue, nil
		}
		if _, code := n.(*ast.CodeBlock); code {
			if s, e, has := st.blockLines(n); has && s <= line && line <= e {
				in = true
				return ast.WalkStop, nil
			}
		}
		return ast.WalkContinue, nil
	})
	return in
}

// blockLines 는 줄 블록(문단·코드)의 첫·끝 원문 줄이다. 줄이 없으면 false.
func (st *state) blockLines(n ast.Node) (int, int, bool) {
	ls := n.Lines()
	if ls == nil || ls.Len() == 0 {
		return 0, 0, false
	}
	return st.lines.line(ls.At(0).Start), st.lines.line(ls.At(ls.Len() - 1).Start), true
}

// tabLine 은 n 줄이 있고 탭을 품는지다(탭·공백만 있는 줄 — 모든 칸이 빈 행 — 도 참).
func (st *state) tabLine(n int) bool {
	return strings.Contains(string(st.lineBytes(n)), "\t")
}

// plainAncestry 는 n 의 조상이 문서·::: 컨테이너뿐인지다 — 그 안의 줄은 앞에 컨테이너 표시(목록 들여쓰기·인용 >)가 없다.
func plainAncestry(n ast.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		switch p.(type) {
		case *ast.Document:
			return true
		case *Container:
		default:
			return false
		}
	}
	return false
}
