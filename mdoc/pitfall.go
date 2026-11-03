package mdoc

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
)

// 한국어 업무 문서의 문법 함정(itda-hyve#140). 표준(CommonMark·GFM)대로의 파싱이라 결과는 바꾸지 않고 경고만 낸다 —
// 다른 뷰어(GitHub 등)와 결과가 같아야 한다(1.1). 판정은 goldmark 가 실제로 만든 노드로 해서 코드·수식·HTML·
// 이스케이프는 저절로 빠진다. 표 칸에는 목록이 없어 W2 만 빠지고, W1 은 칸 안에서도 실제 취소선이라 경고한다.
//
//	W1 tilde-range: 06시~22시 → 06시~다음 날 01시 — 단일 ~ 둘이 짝이 되어 사이가 취소선
//	W2 date-list  : 2026. 10. 14.(수) — 줄 머리 날짜가 "2026 번 목록 안 10 번 목록"
//	list-marker   : - ○ 내용, 1. 가. 내용 — 항목 기호를 글자로 적음(itda-hyve#141, 기호는 list-style 로 테마가 그린다)

// pitfalls 는 transform 의 마지막에 돈다 — 다른 변환이 버린 노드에는 경고하지 않는다.
func (st *state) pitfalls(doc *ast.Document) {
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *extast.Strikethrough:
			st.tildeRange(n)
		case *ast.List:
			st.dateList(n)
			st.listMarker(n)
		}
		return ast.WalkContinue, nil
	})
}

// tildeRange 는 ~ 하나로 연 취소선의 여는 ~ 가 숫자·한글 사이(낱말 안 — 3~5명, 06시~22시)면 경고한다.
// 한글 낱말 사이에 일부러 쓴 단일 ~ 취소선에도 경고한다(알려진 오탐) — 그래서 "범위 표기면" 으로 조건을 단다.
func (st *state) tildeRange(n *extast.Strikethrough) {
	pos := n.Pos() // 여는 구분자 묶음의 첫 바이트(goldmark ProcessDelimiters)
	src := st.src
	if pos <= 0 || pos+1 >= len(src) || src[pos] != '~' || src[pos+1] == '~' || src[pos-1] == '~' {
		return
	}
	before, _ := utf8.DecodeLastRune(src[:pos])
	after, _ := utf8.DecodeRune(src[pos+1:])
	if !rangeChar(before) || !rangeChar(after) {
		return
	}
	st.warn(st.lines.line(pos), WarnTildeRange, "~ 하나로 감싼 사이가 취소선이 됨 — 범위 표기(3~5명)면 \\~ 로 적는다")
}

// rangeChar 는 범위 표기의 ~ 앞뒤에 오는 글자다: 아라비아 숫자, 한글.
func rangeChar(r rune) bool {
	return r >= '0' && r <= '9' || unicode.Is(unicode.Hangul, r)
}

// dateList 는 시작 번호 1900~2099 인 `.` 순서 목록의 첫 항목이 시작 번호 1~12 인 `.` 순서 목록으로 바로 시작하면 경고한다
// (공문 날짜 2026. 10. 14. 가 만드는 "연 목록 안 월 목록"). `)` 는 같은 중첩을 만들지만 날짜 표기가 아니다.
// 날짜가 아닌 "큰 번호 안 작은 번호" 중첩에도 경고한다(알려진 오탐) — 그래서 "날짜면" 으로 조건을 단다.
func (st *state) dateList(year *ast.List) {
	if !year.IsOrdered() || year.Marker != '.' || year.Start < 1900 || year.Start > 2099 {
		return
	}
	item := year.FirstChild()
	if item == nil {
		return
	}
	month, ok := item.FirstChild().(*ast.List)
	if !ok || !month.IsOrdered() || month.Marker != '.' || month.Start < 1 || month.Start > 12 {
		return
	}
	st.warn(st.blockLine(year), WarnDateList,
		fmt.Sprintf("줄 머리의 %d. %d. 가 번호 목록이 됨 — 날짜면 %d\\. 로 적는다", year.Start, month.Start, year.Start))
}

// listMarker 는 목록 항목 문단의 줄 머리가 글자로 적은 항목 기호(□ ○ ● ▸ …)나 항목 번호(가. 가) (가) (1))면
// 그 목록에 한 번(첫 자리의 줄) 경고한다(itda-hyve#141). 기호를 알아보고 점을 숨기지는 않는다 — 결과는 그대로다(결정).
// 줄 머리는 항목 문단의 첫 줄과, 기호·가나다 번호에 한해 이어 붙은 줄(게으른 이어쓰기 — `2. 본문` 아래 `가. 내용`)이다.
// 이어 붙은 줄의 (1)·(2) 는 문장 안 열거로도 흔해서 보지 않는다. 원문자(① ㉮)는 "③ 뒤" 처럼 단계를 가리키는 글로 흔해
// 어디서도 보지 않는다. 판정은 줄 머리의 글자 노드로 하므로 코드·수식·강조 안이나 이스케이프(`\(1)`·`가\.`)는 빠진다.
// ※ 보충은 목록 단계가 아니라 경고하지 않는다.
func (st *state) listMarker(list *ast.List) {
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		first := true // 항목의 첫 문단 첫 줄
		for c := item.FirstChild(); c != nil; c = c.NextSibling() {
			if c.Kind() != ast.KindParagraph && c.Kind() != ast.KindTextBlock {
				first = false
				continue
			}
			for in := c.FirstChild(); in != nil; in = in.NextSibling() {
				t, ok := in.(*ast.Text)
				if !ok || !atLineStart(in) {
					continue
				}
				head := in.PreviousSibling() == nil
				pos := t.Segment.Start
				if pos > 0 && st.src[pos-1] == '\\' {
					continue
				}
				// 원문 줄로 본다 — 글자 노드는 빈칸에서 나뉘기도 한다(ㅇ 뒤 빈칸). 이스케이프(가\.)는 원문에 남아 걸리지 않는다
				line := st.src[pos:]
				if i := bytes.IndexByte(line, '\n'); i >= 0 {
					line = line[:i]
				}
				mark, kind := leadingMarker(line)
				if kind == markNone || kind == markParen && !(first && head) {
					continue
				}
				msg := fmt.Sprintf("목록 항목이 글자 기호 %s 로 시작함 — 기호를 빼고 frontmatter list-style: gaejosik 로(단계별 기호는 테마가 그린다)", mark)
				if kind != markSymbol {
					msg = fmt.Sprintf("목록 항목이 항목 번호 %s 로 시작함 — 번호를 빼고 순서 목록(1.)을 단계별로 들여 쓰고 frontmatter list-style: gongmun 으로", mark)
				}
				st.warn(st.lines.line(pos), WarnListMarker, msg)
				return
			}
			first = false
		}
	}
}

// atLineStart 는 글 안 노드가 문단의 줄 머리인지다: 첫 노드이거나 앞 노드가 줄바꿈으로 끝난 글자다.
func atLineStart(n ast.Node) bool {
	p := n.PreviousSibling()
	if p == nil {
		return true
	}
	t, ok := p.(*ast.Text)
	return ok && (t.SoftLineBreak() || t.HardLineBreak())
}

// markerSymbols 는 개조식에서 항목 기호로 쓰는 글자다. ㅇ(○ 대신 흔히 쓰는 자모)은 뒤에 빈칸이 있을 때만 본다.
// ※ 는 보충 문단 표지라 넣지 않는다. - * + 는 그 자체로 목록 표지라 문단 머리에 오지 않는다.
const markerSymbols = "□■◻◼▢○●◦◎◇◆▪▫▸▹▶▷►•∙·"

// ganada 는 공문 항목 번호의 한글 차례다(가~하, 넘으면 거~허 — 행정업무운영 편람). base.css 의 itda-ganada-* 와 같다.
const ganada = "가나다라마바사아자차카타파하거너더러머버서어저처커터퍼허"

const (
	markNone   = iota
	markSymbol // □ ○ …
	markGanada // 가. 가) (가)
	markParen  // (1) — 항목 첫 줄에서만
)

// leadingMarker 는 줄 머리의 항목 기호·번호와 그 꼴을 돌려준다. 번호 꼴은 뒤에 빈칸이 있어야 한다 — "가.나다" 같은
// 글을 잡지 않는다. 괄호 숫자는 두 자리까지(사다리의 (1)…(99)).
func leadingMarker(b []byte) (string, int) {
	r, n := utf8.DecodeRune(b)
	if n == 0 {
		return "", markNone
	}
	spaced := func(k int) bool { return k == len(b) || b[k] == ' ' || b[k] == '\t' }
	switch {
	case strings.ContainsRune(markerSymbols, r):
		return string(r), markSymbol
	case r == 'ㅇ' && n < len(b) && spaced(n):
		return string(r), markSymbol
	case strings.ContainsRune(ganada, r):
		if n < len(b) && (b[n] == '.' || b[n] == ')') && spaced(n+1) {
			return string(b[:n+1]), markGanada
		}
	case r == '(':
		r2, n2 := utf8.DecodeRune(b[n:])
		k, kind := n+n2, markGanada
		if r2 >= '0' && r2 <= '9' {
			kind = markParen
			if k < len(b) && b[k] >= '0' && b[k] <= '9' {
				k++
			}
		} else if n2 == 0 || !strings.ContainsRune(ganada, r2) {
			return "", markNone
		}
		if k < len(b) && b[k] == ')' && spaced(k+1) {
			return string(b[:k+1]), kind
		}
	}
	return "", markNone
}
