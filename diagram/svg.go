package diagram

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// SVG 쓰기 도구 — 그림 종류가 함께 쓴다(flowchart 의 svg.go 에서 꺼냈다). 쓰는 요소·속성은 각 그리기가 정하고,
// mdoc/security_test.go 의 허용 목록이 고정한다: style·이벤트 속성·url()·href·id 를 쓰지 않는다.

// Writer 는 SVG 요소를 쌓는다. 사용자 글자는 Text 와 Open 의 속성 값으로만 들어간다(둘 다 Esc 를 지난다).
type Writer struct{ b strings.Builder }

func (s *Writer) Open(tag string, kv ...string) {
	s.b.WriteString("<" + tag)
	for i := 0; i+1 < len(kv); i += 2 {
		s.b.WriteString(" " + kv[i] + `="`)
		s.b.WriteString(Esc(kv[i+1]))
		s.b.WriteByte('"')
	}
	s.b.WriteByte('>')
}

func (s *Writer) Close(tag string) { s.b.WriteString("</" + tag + ">") }

func (s *Writer) Leaf(tag string, kv ...string) {
	s.Open(tag, kv...)
	s.Close(tag)
}

// Text 는 요소 글자다(Esc 를 지난다).
func (s *Writer) Text(t string) { s.b.WriteString(Esc(t)) }

func (s *Writer) String() string { return s.b.String() }

var escaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")

// Esc 는 SVG 에 쓰는 모든 사용자 글자(속성 값·요소 글자)가 지나는 하나뿐인 경계다: XML 1.0 이 허용하지 않는 글자
// (탭·줄바꿈 밖의 제어 문자, 서로게이트, U+FFFE·U+FFFF, 잘못된 UTF-8)를 U+FFFD 로 바꾸고 이스케이프한다. 글자가 어디서 왔든
// (원문·Mermaid 엔티티 #65535;·YAML 이스케이스 "￿"·accDescr 블록) 여기서 한 번에 거른다 — 리뷰 R2.
func Esc(s string) string { return escaper.Replace(XmlChars(s)) }

func xmlChar(r rune) bool {
	return r == 0x9 || r == 0xa || r == 0xd || (r >= 0x20 && r <= 0xd7ff) || (r >= 0xe000 && r <= 0xfffd) || (r >= 0x10000 && r <= 0x10ffff)
}

// XmlChars 는 XML 1.0 의 Char 밖 글자를 U+FFFD 로 바꾼다. 모두 올바르면 그대로 돌려준다(할당 없음).
func XmlChars(s string) string {
	clean := utf8.ValidString(s)
	for _, r := range s {
		if !clean || !xmlChar(r) {
			clean = false
			break
		}
	}
	if clean {
		return s
	}
	return strings.Map(func(r rune) rune {
		if xmlChar(r) {
			return r
		}
		return 0xfffd
	}, strings.ToValidUTF8(s, "�"))
}

// F1 은 SVG 좌표·크기에 쓰는 숫자다(소수 한 자리).
func F1(v float64) string {
	v = math.Round(v*10) / 10
	if v == 0 {
		v = 0 // -0
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// Scale 은 viewBox 단위(글자 14) → 화면 px 이다. 그림의 제 크기에서 글자가 약 12.6px(9.5pt) — 차트 글자와 비슷하다.
const Scale = 0.9
