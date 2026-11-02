package mdoc

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// attrs 는 Pandoc 속성 {#id .class key=value} 하나다(1.3·1.5·1.7).
// key=value 는 어디서든 data-key 로만 나간다 — style·onclick 같은 실제 속성이 될 수 없다.
type attrs struct {
	ID         string
	Classes    []string
	Data       [][2]string // 적힌 순서. 키는 소문자
	Unnumbered bool        // {-} 또는 {.unnumbered}
}

func (a attrs) has(class string) bool {
	for _, c := range a.Classes {
		if c == class {
			return true
		}
	}
	return false
}

// get 은 key=value 의 값을 찾는다.
func (a attrs) get(key string) (string, bool) {
	for _, kv := range a.Data {
		if kv[0] == key {
			return kv[1], true
		}
	}
	return "", false
}

// without 은 key 를 뺀 data 목록이다. width 처럼 따로 쓰는 키를 data-* 로 두 번 내지 않으려고 쓴다.
func (a attrs) without(key string) [][2]string {
	var out [][2]string
	for _, kv := range a.Data {
		if kv[0] != key {
			out = append(out, kv)
		}
	}
	return out
}

// parseAttrs 는 중괄호 안쪽 글자를 읽는다. 하나라도 알아보지 못하면 통째로 실패한다 —
// 반쯤 읽은 속성을 내보내는 것보다 글자로 보여 주는 편이 틀린 곳을 찾기 쉽다.
func parseAttrs(s string) (attrs, error) {
	var a attrs
	i := 0
	for {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i >= len(s) {
			return a, nil
		}
		switch c := s[i]; {
		case c == '#':
			name, n := readName(s[i+1:], isIDRune)
			if name == "" {
				return a, fmt.Errorf("id 가 비었거나 쓸 수 없는 글자가 있음")
			}
			a.ID = name
			i += 1 + n
		case c == '.':
			name, n := readName(s[i+1:], isClassRune)
			if name == "" {
				return a, fmt.Errorf("class 가 비었거나 쓸 수 없는 글자가 있음")
			}
			if name == "unnumbered" {
				a.Unnumbered = true
			}
			a.Classes = append(a.Classes, name)
			i += 1 + n
		case c == '-' && (i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\t'):
			a.Unnumbered = true
			a.Classes = append(a.Classes, "unnumbered")
			i++
		default:
			key, n := readName(s[i:], isKeyRune)
			if key == "" || !isASCIILetter(key[0]) || i+n >= len(s) || s[i+n] != '=' {
				return a, fmt.Errorf("알아보지 못한 속성 %q", tokenAt(s[i:]))
			}
			i += n + 1
			val, m, err := readValue(s[i:])
			if err != nil {
				return a, err
			}
			a.Data = append(a.Data, [2]string{strings.ToLower(key), val})
			i += m
		}
		// 다음 토큰과는 공백으로 떨어져야 한다(#a.b 같은 붙여 쓰기는 받지 않는다)
		if i < len(s) && s[i] != ' ' && s[i] != '\t' {
			return a, fmt.Errorf("알아보지 못한 속성 %q", tokenAt(s[i:]))
		}
	}
}

func readName(s string, ok func(rune) bool) (string, int) {
	n := 0
	for _, r := range s {
		if !ok(r) {
			break
		}
		n += len(string(r))
	}
	return s[:n], n
}

func readValue(s string) (string, int, error) {
	if s == "" {
		return "", 0, nil
	}
	if q := s[0]; q == '"' || q == '\'' {
		end := strings.IndexByte(s[1:], q)
		if end < 0 {
			return "", 0, fmt.Errorf("따옴표가 닫히지 않음")
		}
		return s[1 : 1+end], end + 2, nil
	}
	end := strings.IndexAny(s, " \t")
	if end < 0 {
		end = len(s)
	}
	return s[:end], end, nil
}

func tokenAt(s string) string {
	if end := strings.IndexAny(s, " \t"); end >= 0 {
		return s[:end]
	}
	return s
}

// id 는 글자·숫자와 - _ . : 만 받는다. 따옴표·꺾쇠가 섞인 id 는 속성 전체를 버린다.
func isIDRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' || r == '_' || r == '.' || r == ':'
}

func isClassRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' || r == '_'
}

// 키는 data-* 이름이 되므로 ASCII 로 좁힌다.
func isKeyRune(r rune) bool {
	return r < 0x80 && (isASCIILetter(byte(r)) || (r >= '0' && r <= '9') || r == '-' || r == '_')
}

func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// lengthRe 는 width 로 받는 길이 형식이다. 숫자 + 단위 하나만 — 세미콜론·괄호가 들어갈 틈이 없다.
var lengthRe = regexp.MustCompile(`^(?:\d+(?:\.\d+)?|\.\d+)(?:%|mm|cm|in|pt|pc|px|em|rem|ch)?$`)

// cssLength 는 길이 형식이면 CSS 값을 돌려준다. 단위 없는 숫자는 Pandoc 처럼 px 로 본다.
func cssLength(v string) (string, bool) {
	v = strings.TrimSpace(v)
	if !lengthRe.MatchString(v) {
		return "", false
	}
	if last := v[len(v)-1]; last >= '0' && last <= '9' {
		v += "px"
	}
	return v, true
}
