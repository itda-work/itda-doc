package mdoc

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
)

// 코드 블록 줄 강조(#479): 펜스 info string 에 강조할 줄을 적으면 pre 에 data-hl 속성(정규화한 목록 "1,3-5")을
// 달고 화면(codelines.js — 줄 span 에 hl 클래스, base.css 띠)이 그 줄을 띠로 강조한다. 표기는 둘이다 —
//
//	```go {1,3-5}   ```go{1,3-5}   ``` {2}   ```go title="a.go" {2-3}   ```go:a.go {2}   (정식 — Docusaurus·VitePress)
//	[.code-highlight: 1, 3-5]  (펜스 바로 앞 문단 — Deckset 호환. all·none 도 받는다)
//
// 둘 다 있으면 info 가 이긴다. 강조는 속성값일 뿐 코드 글자(textContent·복사·검색)에 들어가지 않는다.

// maxCodeHLLine 은 강조할 수 있는 가장 큰 줄 번호다 — 구문 강조 블록 상한(maxHighlightBlockLines)과 같은 값을 쓴다.
// 이 위 번호와 블록 줄 수 밖 번호는 버린다. 강조 가능한 줄 번호의 상한일 뿐 코드 표시·줄 번호나 입력 처리 비용의
// 상한이 아니다(지시어 수집은 문단 수에 선형 — codeHLDirectives).
const maxCodeHLLine = maxHighlightBlockLines

// cutHLWord 는 info string 에서 첫 줄 강조 낱말 {…}(숫자·쉼표·공백·- 만)을 떼어 낸 나머지와 그 안 글자를 돌려준다
// (없으면 found=false, rest 는 s 그대로 — 기존 언어·파일명 가르기가 바이트까지 같다). 낱말의 { 앞은 맨 앞·공백·
// 언어 글자(```js{4} 처럼 붙여 쓴 꼴), } 뒤는 공백·끝이어야 한다. 공백은 기존 가르기의 낱말 가르기와 같은
// unicode.IsSpace(전각 공백 포함)다. 떼어 낸 자리는 공백 하나로 둔다(이웃 글자가 이어 붙지 않게).
//
// 파일명은 따로 가르지 않는다 — 기존 가르기(parseCodeLangTitle)가 파일명으로 고르는 원문 범위(hlProtected)를 그대로
// 보호하고 그 밖에서만 낱말을 찾는다. 그래서 떼어 내도 title·줄임 파일명 글자는 바뀌지 않는다(title="part {2} x.go"
// {1} 은 파일명 그대로에 1줄 강조). 맨 앞 낱말이 {…} 면 떼어 낸 뒤 다음 낱말이 첫 낱말(언어·줄임)이 된다.
func cutHLWord(s string) (rest, spec string, found bool) {
	protected := hlProtected(s)
	for i := 0; i < len(s); i++ {
		if s[i] != '{' || !hlWordBefore(s[:i]) {
			continue
		}
		j := i + 1
		for j < len(s) && strings.IndexByte("0123456789 \t,-", s[j]) >= 0 {
			j++
		}
		if j >= len(s) || s[j] != '}' || !hlWordAfter(s[j+1:]) {
			continue
		}
		if inHLProtected(protected, i, j+1) {
			continue
		}
		return s[:i] + " " + s[j+1:], s[i+1 : j], true
	}
	return s, "", false
}

// hlProtected 는 기존 가르기(parseCodeLangTitle)가 파일명으로 읽는 원문 범위들([시작, 끝) 바이트)이다. 기존 가르기와
// 같은 전처리(codeInfoText — 앞뒤 공백 지우기)를 거친 같은 글에서 구하고, 앞에서 지운 바이트 수를 더해 원문 좌표로 되돌린다
// (원문에 바로 정규식을 돌리면 전각 공백 같은 앞 공백 때문에 titleRe 의 (^|\s) 경계가 달라져 다른 범위를 고른다 — 리뷰 3회차).
//   - titleRe 매치 전체(앞 공백·title=·따옴표·값). titleWord 와 같은 정규식·같은 글이다. titleWord 는 첫 매치만 쓰지만
//     뒤 title= 값도 파일명 표기이므로 모든 매치를 보호한다(title="a" title='b {3}' {1} 은 1줄 강조).
//   - 첫 낱말에 = 가 없고 : 가 있으면 첫 : 뒤부터 첫 낱말 끝까지(줄임 언어:파일명).
func hlProtected(s string) [][2]int {
	var out [][2]int
	text, lead := codeInfoText(s)
	for _, m := range titleRe.FindAllStringIndex(text, -1) {
		out = append(out, [2]int{lead + m[0], lead + m[1]})
	}
	first := codeInfoFirst(text)
	if !strings.ContainsRune(first, '=') {
		if c := strings.IndexByte(first, ':'); c >= 0 {
			out = append(out, [2]int{lead + c + 1, lead + len(first)})
		}
	}
	return out
}

func inHLProtected(ranges [][2]int, from, to int) bool {
	for _, r := range ranges {
		if from < r[1] && r[0] < to {
			return true
		}
	}
	return false
}

// hlWordBefore 는 줄 강조 낱말의 { 앞(before 는 그 앞 원문 전체)이 맨 앞·공백(unicode.IsSpace)·언어 글자인지 본다.
func hlWordBefore(before string) bool {
	if before == "" {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(before)
	return unicode.IsSpace(r) || (r < 0x80 && (isASCIILetter(byte(r)) || (r >= '0' && r <= '9') || strings.ContainsRune("+#._-", r)))
}

// hlWordAfter 는 줄 강조 낱말의 } 뒤(after 는 그 뒤 원문 전체)가 끝·공백(unicode.IsSpace)인지 본다.
func hlWordAfter(after string) bool {
	if after == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(after)
	return unicode.IsSpace(r)
}

// codeHLDirectiveRe 는 Deckset 지시어 한 줄 [.code-highlight: …] 이다(대소문자 무시).
var codeHLDirectiveRe = regexp.MustCompile(`(?i)^\[\.code-highlight:\s*([^\]]*)\]$`)

// codeHLDirective 는 줄 강조 지시어 하나(Deckset)다. 값은 all·none·번호 목록 원문이다.
type codeHLDirective struct {
	spec string
	line int
}

// codeHL 은 펜스 코드 블록의 줄 강조 목록(정규화한 "1,3-5", 없으면 "")이다 — info 의 {…} 가 펜스 앞 Deckset
// 지시어보다 이긴다(codehl.go). 읽지 못한 낱말은 경고하고 버린다.
func (r *htmlRenderer) codeHL(node ast.Node, info codeInfo) string {
	fc, ok := node.(*ast.FencedCodeBlock)
	if !ok {
		return ""
	}
	spec, directive, line := info.hl, false, r.st.lines.line(fc.Pos())
	if !info.hasHL {
		d, ok := r.st.codeHL[fc]
		if !ok {
			return ""
		}
		spec, directive, line = d.spec, true, d.line
	}
	out, bad := normalizeHL(spec, fc.Lines().Len(), directive)
	if len(bad) > 0 {
		r.st.warn(line, WarnBadAttribute, fmt.Sprintf("코드 줄 강조 %s 를 읽지 못해 버림(번호 또는 a-b 범위, a ≤ b)", strings.Join(bad, " ")))
	}
	return out
}

// hlDirective 는 문단(느슨한 목록·본문의 Paragraph, 촘촘한 목록 항목의 TextBlock)의 모든 줄이 지시어면 첫 지시어
// 값과 지시어 수를 돌려준다(아니면 ok=false).
func (st *state) hlDirective(n ast.Node) (first string, count int, ok bool) {
	switch n.(type) {
	case *ast.Paragraph, *ast.TextBlock:
	default:
		return "", 0, false
	}
	lines := n.Lines()
	if lines.Len() == 0 {
		return "", 0, false
	}
	for i := 0; i < lines.Len(); i++ {
		v := strings.TrimSpace(string(segValue(lines.At(i), st.src)))
		m := codeHLDirectiveRe.FindStringSubmatch(v)
		if m == nil {
			return "", 0, false
		}
		if i == 0 {
			first = strings.TrimSpace(m[1])
		}
	}
	return first, lines.Len(), true
}

// codeHLDirectives 는 펜스 바로 앞(같은 부모의 앞 형제)에 이어진 지시어 문단들을 모아 첫 지시어를 그 펜스에
// 걸고 문단은 모두 지운다. 여러 개(Deckset 의 단계별 강조)는 첫 것만 쓰고 경고한다. 펜스가 뒤따르지 않는
// 지시어 문단은 건드리지 않는다(글자로 보인다). 뒤에서 앞으로 걸으며 가장 앞 문단의 첫 값·지시어 수·지울 문단만
// 들고 가므로 비용은 문단 수에 선형이다.
func (st *state) codeHLDirectives(fc *ast.FencedCodeBlock) {
	var paras []ast.Node
	var spec string
	total := 0
	for prev := fc.PreviousSibling(); prev != nil; prev = prev.PreviousSibling() {
		first, n, ok := st.hlDirective(prev)
		if !ok {
			break
		}
		paras = append(paras, prev)
		spec, total = first, total+n // 더 앞 문단의 첫 값이 이긴다
	}
	if len(paras) == 0 {
		return
	}
	line := st.lines.line(paras[len(paras)-1].Lines().At(0).Start)
	st.codeHL[fc] = codeHLDirective{spec: spec, line: line}
	if total > 1 {
		st.warn(line, WarnBadAttribute, fmt.Sprintf("코드 줄 강조 지시어가 %d개 — 단계별 강조는 지원하지 않아 첫 지시어만 씀", total))
	}
	for _, p := range paras {
		p.Parent().RemoveChild(p.Parent(), p)
	}
}

// normalizeHL 은 줄 강조 값을 n 줄 블록에 맞춰 "1,3-5" 꼴로 정규화한다(강조할 줄이 없으면 ""). 쉼표·공백으로
// 가른 낱말마다 번호 하나(k) 또는 범위(a-b). 거꾸로 범위·0·숫자가 아닌 낱말은 bad 로 세어 버리고, 블록 줄 수·
// maxCodeHLLine 밖 번호는 버린다(범위는 안쪽만 남긴다). directive 면 all(모든 줄)·none(강조 없음)을 받는다.
func normalizeHL(spec string, n int, directive bool) (out string, bad []string) {
	limit := min(n, maxCodeHLLine)
	if limit <= 0 {
		return "", nil
	}
	if directive {
		switch strings.ToLower(strings.TrimSpace(spec)) {
		case "all":
			return hlRanges(fullMarks(limit)), nil
		case "none":
			return "", nil
		}
	}
	marks := make([]bool, limit+1)
	for _, w := range strings.FieldsFunc(spec, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
		a, b, ok := hlRange(w)
		if !ok {
			bad = append(bad, w)
			continue
		}
		for k := a; k <= min(b, limit); k++ {
			marks[k] = true
		}
	}
	return hlRanges(marks), bad
}

func fullMarks(limit int) []bool {
	m := make([]bool, limit+1)
	for i := 1; i <= limit; i++ {
		m[i] = true
	}
	return m
}

// hlRange 는 낱말 k 또는 a-b 를 1 이상 a ≤ b 범위로 읽는다.
func hlRange(w string) (a, b int, ok bool) {
	x, y, isRange := strings.Cut(w, "-")
	a, err := strconv.Atoi(x)
	if err != nil || a < 1 {
		return 0, 0, false
	}
	if !isRange {
		return a, a, true
	}
	b, err = strconv.Atoi(y)
	if err != nil || b < a {
		return 0, 0, false
	}
	return a, b, true
}

// hlRanges 는 표시(1부터)를 "1,3-5" 로 묶는다.
func hlRanges(marks []bool) string {
	var parts []string
	for i := 1; i < len(marks); i++ {
		if !marks[i] {
			continue
		}
		j := i
		for j+1 < len(marks) && marks[j+1] {
			j++
		}
		if j == i {
			parts = append(parts, strconv.Itoa(i))
		} else {
			parts = append(parts, strconv.Itoa(i)+"-"+strconv.Itoa(j))
		}
		i = j
	}
	return strings.Join(parts, ",")
}
