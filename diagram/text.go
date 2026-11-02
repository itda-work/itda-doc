package diagram

import (
	"math"
	"strconv"
	"strings"
	"unicode"
)

// 글자 폭 어림·줄바꿈·라벨 풀기 — 그림 종류가 함께 쓴다(flowchart 의 text.go 에서 꺼냈다).

// FontSize·LineH 는 그림 글자의 기준(라벨 폭·줄 간격을 잰다).
const (
	FontSize = 14.0
	LineH    = 18.0
)

// TextW 는 글자 너비 어림이다(글자 크기 size). 글자 너비는 잴 수 없어 어림한다 — 차트(mdoc/chartsvg.go 의 textW)와
// 같은 표다: 한글·한자·가나 약 1em, 숫자 0.58em, 영문 대문자 0.66em·소문자 0.54em.
func TextW(s string, size float64) float64 {
	w := 0.0
	for _, r := range s {
		switch {
		case r == ' ':
			w += 0.28
		case r >= 0x1100 && (unicode.Is(unicode.Hangul, r) || unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) ||
			unicode.Is(unicode.Katakana, r) || (r >= 0xff00 && r <= 0xffef)):
			w += 0.98
		case r >= '0' && r <= '9':
			w += 0.58
		case r >= 'A' && r <= 'Z', r == 'W', r == 'M', r == '%':
			w += 0.66
		case r >= 'a' && r <= 'z':
			w += 0.54
		case r == '.' || r == ',' || r == ':' || r == ';' || r == '·' || r == '\'' || r == '|':
			w += 0.3
		default:
			w += 0.6
		}
	}
	return w * size
}

// WrapLabel 은 라벨을 줄로 나눈다: <br>(\n) 에서, 그리고 너비 width 를 넘으면 낱말 사이에서. 한 낱말이 너비보다 길면
// 글자에서 자른다. 글자는 버리지 않는다.
func WrapLabel(s string, width float64) []string {
	var out []string
	for _, para := range strings.Split(s, "\n") {
		if TextW(para, FontSize) <= width {
			out = append(out, para)
			continue
		}
		cur := ""
		for _, w := range strings.Fields(para) {
			for TextW(w, FontSize) > width { // 긴 낱말은 글자에서
				if cur != "" {
					out = append(out, cur)
					cur = ""
				}
				r := []rune(w)
				k := 1
				for k < len(r) && TextW(string(r[:k+1]), FontSize) <= width {
					k++
				}
				out = append(out, string(r[:k]))
				w = string(r[k:])
			}
			switch {
			case w == "":
			case cur == "":
				cur = w
			case TextW(cur+" "+w, FontSize) <= width:
				cur += " " + w
			default:
				out = append(out, cur)
				cur = w
			}
		}
		if cur != "" {
			out = append(out, cur)
		}
	}
	if len(out) == 0 {
		out = []string{""}
	}
	return out
}

// LinesW 는 줄 가운데 가장 넓은 것의 폭이다.
func LinesW(lines []string) float64 {
	w := 0.0
	for _, l := range lines {
		w = math.Max(w, TextW(l, FontSize))
	}
	return w
}

var brTags = []string{"<br>", "<br/>", "<br />"}

// DecodeLabel 은 라벨 글을 푼다: <br> 은 줄바꿈(\n), Mermaid 엔티티(#quot; #35;)는 글자. 다른 HTML 은 글자 그대로다
// (실행하지 않는다). 제어 문자는 빈칸으로, 줄마다 앞뒤 빈칸을 지운다.
func DecodeLabel(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		c := s[i]
		if c == '<' {
			done := false
			for _, t := range brTags {
				if len(s)-i >= len(t) && strings.EqualFold(s[i:i+len(t)], t) {
					b.WriteByte('\n')
					i += len(t)
					done = true
					break
				}
			}
			if done {
				continue
			}
		}
		if c == '#' {
			if end := strings.IndexByte(s[i+1:min(len(s), i+12)], ';'); end > 0 {
				if r, ok := entity(s[i+1 : i+1+end]); ok {
					b.WriteRune(r)
					i += end + 2
					continue
				}
			}
		}
		if c < 0x20 || c == 0x7f { // 제어 문자는 빈칸
			b.WriteByte(' ')
			i++
			continue
		}
		b.WriteByte(c)
		i++
	}
	lines := strings.Split(b.String(), "\n")
	for i, l := range lines {
		lines[i] = strings.Join(strings.Fields(l), " ")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

var namedEntities = map[string]rune{"quot": '"', "amp": '&', "lt": '<', "gt": '>', "nbsp": ' ', "apos": '\'', "num": '#', "semi": ';'}

func entity(name string) (rune, bool) {
	if r, ok := namedEntities[name]; ok {
		return r, true
	}
	if name == "" || len(name) > 7 {
		return 0, false
	}
	n, err := strconv.Atoi(name)
	if err != nil || n < 0x20 || n > unicode.MaxRune || (n >= 0xd800 && n <= 0xdfff) || (n >= 0x7f && n < 0xa0) {
		return 0, false
	}
	return rune(n), true
}
