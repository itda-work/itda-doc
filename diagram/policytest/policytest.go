// Package policytest 는 엔티티·공백 정책 B 와 출력 관문을 종류마다 같은 방법으로 시험하는 도구다(시험에서만 부른다 — #409 ④-2
// 후속). 기대값은 사람이 적은 표이고 구현의 함수·표(diagram.RestoreEntities·EntityNumOK·WrapLabel 따위)를 부르지 않는다. 결과는
// 최종 SVG 를 실제 XML 파서(encoding/xml)로 읽어 본다. state 의 judge_test.go 표·도구를 옮긴 것이다(state 는 제 것을 그대로 쓴다).
package policytest

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/itda-work/itda-doc/diagram"
)

// EntityTable 은 모든 문맥(라벨·접근성)에서 같은 기대를 갖는 엔티티 조각이다(Want 가 nil 이면 그 줄은 원문 + 경고). 캡션
// (frontmatter title)은 원본이 엔티티 전처리 전에 떼어 내므로 이 표를 쓰지 않는다 — 글자 그대로다(CaptionLiteral).
var EntityTable = []struct {
	Frag string
	Want []rune
}{
	{"#35;", []rune{0x23}}, {"#59;", []rune{0x3b}}, {"#quot;", []rune{0x22}},
	{"#amp;", []rune{0x26}}, {"#lt;", []rune{0x3c}}, {"#gt;", []rune{0x3e}}, {"#apos;", []rune{0x27}},
	{"#num;", []rune{0x23}}, {"#semi;", []rune{0x3b}}, {"#lt;br#gt;", []rune{0x3c, 0x62, 0x72, 0x3e}},
	{"#169;", []rune{0xa9}}, {"#8364;", []rune{0x20ac}}, {"#0000065;", []rune{0x41}},
	// 미지원 이름 7개 밖·대소문자 변형·이름 표식인 #x41;·8자리
	{"#copy;", nil}, {"#euro;", nil}, {"#hellip;", nil}, {"#zzz;", nil}, {"#nbsp;", nil}, {"#AMP;", nil}, {"#Quot;", nil},
	{"#LT;", nil}, {"#x41;", nil}, {"#00000065;", nil},
	// 공백·제어·서식
	{"#160;", nil}, {"#160;#160;", nil}, {"#8195;", nil}, {"#32;", nil}, {"#0032;", nil}, {"#9;", nil}, {"#10;", nil},
	{"#8203;", nil}, {"#65279;", nil},
	// 숫자 경계
	{"#31;", nil}, {"#33;", []rune{0x21}}, {"#126;", []rune{0x7e}}, {"#127;", nil}, {"#159;", nil}, {"#161;", []rune{0xa1}},
	// 서러게이트 양끝과 이웃, 비문자
	{"#55295;", []rune{0xd7ff}}, {"#55296;", nil}, {"#57343;", nil}, {"#57344;", []rune{0xe000}},
	{"#1114109;", []rune{0x10fffd}}, {"#1114110;", nil}, {"#1114111;", nil}, {"#1114112;", nil},
	{"#64975;", []rune{0xfdcf}}, {"#64976;", nil}, {"#65533;", []rune{0xfffd}}, {"#65534;", nil}, {"#65535;", nil},
	{"#65536;", []rune{0x10000}},
}

// Ctx 는 값 하나를 넣을 문맥이다 — Src 가 값 v 를 넣은 원문을 만들고, 거부면 Line 번 줄의 원문 + 경고다.
type Ctx struct {
	Name string
	Src  func(v string) string
	Line int
	Acc  bool // 접근성 문맥(accTitle·accDescr) — 안쪽 빈칸·탭을 그대로 둔다
	// OddLine 은 accDescr { … } 블록 문맥에서 값 줄의 번호다(Line 은 닫는 줄 — 값 검사가 거부하는 줄, 줄 단위 특수 공백 검사는
	// 값 줄에서 거부한다). 0 이면 블록 문맥이 아니다.
	OddLine int
	// Show 는 받은 값이 SVG 에 보이는 꼴이다(nil 이면 값 그대로) — sequence 의 loop 조건은 [값] 으로 그린다.
	Show func(v string) string
}

// Render 는 종류의 Render 다.
type Render func(src string) (diagram.Result, error)

// Out 은 최종 SVG 를 XML 로 읽은 결과다 — 뿌리 svg 의 aria-label 값, <title> 글, <text> 마다의 줄 목록(tspan 이 있으면 tspan
// 마다 한 줄), 그리고 모든 <text> 줄을 차례로 늘어놓은 Flat(한 줄마다 <text> 하나를 쓰는 종류의 여러 줄 라벨을 보려고).
type Out struct {
	Aria    string
	Title   []string
	Texts   [][]string
	Flat    []string
	Caption string
}

// ParseSVG 는 SVG 를 XML 로 읽는다. 읽히지 않으면 시험 실패다.
func ParseSVG(t *testing.T, svg string) Out {
	t.Helper()
	var out Out
	d := xml.NewDecoder(strings.NewReader(svg))
	depth := 0
	var lines []string
	var buf strings.Builder
	for {
		start := d.InputOffset()
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("SVG 가 XML 로 읽히지 않음: %v\n%s", err, svg)
		}
		switch e := tok.(type) {
		case xml.StartElement:
			depth++
			buf.Reset()
			if depth == 1 && e.Name.Local == "svg" {
				for _, at := range Attrs(t, svg[start:d.InputOffset()]) {
					if at.Name.Local == "aria-label" {
						out.Aria = at.Value
					}
				}
			}
			if e.Name.Local == "text" {
				lines = nil
			}
		case xml.CharData:
			buf.Write(e)
		case xml.EndElement:
			depth--
			switch e.Name.Local {
			case "title":
				out.Title = append(out.Title, buf.String())
			case "tspan":
				lines = append(lines, buf.String())
			case "text":
				if lines == nil {
					lines = []string{buf.String()}
				}
				out.Texts = append(out.Texts, lines)
				out.Flat = append(out.Flat, lines...)
			}
			buf.Reset()
		}
	}
	return out
}

// Attrs 는 시작 태그 원문 하나의 속성을 XML 속성값 정규화(XML 1.0 3.3.3 — 줄끝을 LF 로 모은 뒤 리터럴 탭·LF 를 빈칸으로, 문자
// 참조는 그대로)를 거쳐 돌려준다. encoding/xml 은 이 정규화를 하지 않아 손으로 먼저 한다.
func Attrs(t *testing.T, tag string) []xml.Attr {
	t.Helper()
	tag = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ", "\t", " ").Replace(tag)
	if !strings.HasSuffix(tag, "/>") {
		tag = strings.TrimSuffix(tag, ">") + "/>"
	}
	tok, err := xml.NewDecoder(strings.NewReader(tag)).Token()
	if err != nil {
		t.Fatalf("시작 태그를 읽지 못함: %v\n%s", err, tag)
	}
	return tok.(xml.StartElement).Attr
}

// WrapOK 는 줄 목록이 값 v 를 보이는 글자 그대로 나눈 것인지 본다(손 규칙 — WrapLabel 을 부르지 않는다). 줄은 비지 않고 앞뒤·
// 연속 빈칸이 없으며, 차례로 이으면 v 다. 줄 경계에서 v 의 그 자리 글자가 빈칸 하나면 그 빈칸이 줄바꿈이 된다(빠진다). 아니면
// 긴 낱말을 강제로 자른 것이라 아무 글자도 더하거나 빼지 않는다.
func WrapOK(v string, lines []string) bool {
	if len(lines) == 0 {
		return false
	}
	pos := 0
	for i, ln := range lines {
		if ln == "" || ln[0] == ' ' || ln[len(ln)-1] == ' ' || strings.Contains(ln, "  ") {
			return false
		}
		if i > 0 && pos < len(v) && v[pos] == ' ' {
			pos++
		}
		if !strings.HasPrefix(v[pos:], ln) {
			return false
		}
		pos += len(ln)
	}
	return pos == len(v)
}

// TextOf 는 v 를 WrapOK 로 담은 첫 <text> 의 줄 목록이다(없으면 nil).
func (o Out) TextOf(v string) []string {
	for _, ls := range o.Texts {
		if WrapOK(v, ls) {
			return ls
		}
	}
	return nil
}

// LinesOf 는 v 를 WrapOK 로 담은 줄 목록이다 — 한 <text> 의 tspan 줄이거나, 한 줄마다 <text> 하나를 쓰는 종류(흐름도 subgraph
// 제목)의 잇단 <text> 줄(Flat)이다. 없으면 nil.
func (o Out) LinesOf(v string) []string {
	if ls := o.TextOf(v); ls != nil {
		return ls
	}
	for i := range o.Flat {
		for j := i + 1; j <= len(o.Flat); j++ {
			if WrapOK(v, o.Flat[i:j]) {
				return o.Flat[i:j]
			}
		}
	}
	return nil
}

// Shows 는 v 가 SVG(또는 캡션)에 보이는지다 — aria-label·<title>·캡션은 값 그대로, <text> 는 WrapOK 로. 줄바꿈이 든 v 는 줄마다
// 차례로 이어진 <text> 줄(Flat)이거나 한 <text> 의 tspan 줄들이다.
func (o Out) Shows(v string) bool {
	if o.Aria == v || o.Caption == v || slices.Contains(o.Title, v) || o.TextOf(v) != nil {
		return true
	}
	if !strings.Contains(v, "\n") {
		return false
	}
	want := strings.Split(v, "\n")
	for i := 0; i+len(want) <= len(o.Flat); i++ {
		if slices.Equal(o.Flat[i:i+len(want)], want) {
			return true
		}
	}
	return false
}

// Check 는 원문 하나를 그려 기대(ok·want)와 견준다 — 거부면 ctx.Line 번 줄의 *diagram.Error, 받으면 최종 SVG·캡션에 want 가
// 코드 포인트 그대로 보여야 한다.
func Check(t *testing.T, render Render, ctx Ctx, name string, ok bool, want string) {
	t.Helper()
	src := ctx.Src("x" + name + "y")
	CheckSrc(t, render, ctx, name, src, ok, want)
}

// CheckSrc 는 Check 의 원문을 직접 받는 꼴이다.
func CheckSrc(t *testing.T, render Render, ctx Ctx, name, src string, ok bool, want string) {
	t.Helper()
	r, err := render(src)
	if !ok {
		var de *diagram.Error
		if err == nil || !errors.As(err, &de) || de.Line != ctx.Line {
			t.Errorf("%s/%s: %d 줄 거부를 기대, Render %v\n%q", ctx.Name, name, ctx.Line, err, src)
		}
		return
	}
	if err != nil {
		t.Errorf("%s/%s: 받기를 기대, Render %v\n%q", ctx.Name, name, err, src)
		return
	}
	o := ParseSVG(t, r.SVG)
	o.Caption = r.Caption
	if ctx.Show != nil {
		want = ctx.Show(want)
	}
	if !o.Shows(want) {
		t.Errorf("%s/%s: SVG 에 %U 가 없음\n%q\n%s", ctx.Name, name, []rune(want), src, r.SVG)
	}
}

// EntityPolicy 는 EntityTable 을 문맥마다 교차해 본다. 캡션 문맥(Caption 이 만든 것)은 넣지 않는다 — CaptionLiteral 이 본다.
func EntityPolicy(t *testing.T, render Render, ctxs []Ctx) {
	t.Helper()
	for _, e := range EntityTable {
		for _, ctx := range ctxs {
			CheckSrc(t, render, ctx, e.Frag, ctx.Src("x"+e.Frag+"y"), e.Want != nil, "x"+string(e.Want)+"y")
		}
	}
}

// Raw 는 종류마다 다른 원문 조각의 기대다 — Lbl·LblOK 는 라벨 문맥, Acc·AccOK 는 접근성 문맥(값은 앞뒤에 x·y 를 붙여 본다).
type Raw struct {
	Frag         string
	Lbl, Acc     string
	LblOK, AccOK bool
}

// RawPolicy 는 종류의 원문 조각 표를 문맥마다 교차해 본다.
func RawPolicy(t *testing.T, render Render, ctxs []Ctx, table []Raw) {
	t.Helper()
	for _, e := range table {
		for _, ctx := range ctxs {
			ok, want := e.LblOK, e.Lbl
			if ctx.Acc {
				ok, want = e.AccOK, e.Acc
			}
			CheckSrc(t, render, ctx, fmt.Sprintf("%q", e.Frag), ctx.Src("x"+e.Frag+"y"), ok, "x"+want+"y")
		}
	}
}

// YAMLQuote 는 v 를 YAML 큰따옴표 글로 쓴다(\·" 와 탭·제어·U+FFFE·U+FFFF 는 이스케이프).
func YAMLQuote(v string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range v {
		switch {
		case r == '\\' || r == '"':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f || r == 0xfffe || r == 0xffff:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// CaptionCtx 는 frontmatter title 문맥이다 — body 는 그 뒤에 올 그림 원문(머리 낱말부터), 거부 줄은 title 줄(2)이다.
func CaptionCtx(body string) Ctx {
	return Ctx{Name: "캡션", Src: func(v string) string { return "---\ntitle: " + YAMLQuote(v) + "\n---\n" + body }, Line: 2}
}

// CaptionLiteral 은 캡션의 정책 B 를 본다 — 엔티티는 표식이 아니라 글자 그대로(원본 Diagram.fromText 가 frontmatter 를
// encodeEntities 전에 뗀다), 앞뒤 빈칸은 지우고 안쪽의 탭·연속 빈칸·C0 제어 문자(U+0001)·특수 공백·XML 이 담지 못하는 글자(U+FFFE)는
// title 줄의 원문 + 경고, <br> 은 빈칸 하나다(아래 사례가 보는 것만 — C1 은 C1Policy). 기대는 손으로 적었다.
func CaptionLiteral(t *testing.T, render Render, body string) {
	t.Helper()
	ctx := CaptionCtx(body)
	for _, e := range EntityTable {
		CheckSrc(t, render, ctx, e.Frag, ctx.Src("x"+e.Frag+"y"), true, "x"+e.Frag+"y")
	}
	for _, c := range []struct {
		v    string
		ok   bool
		want string
	}{
		{"x#35;35;y", true, "x#35;35;y"},
		{"  x y  ", true, "x y"},
		{"x  y", false, ""},
		{"x\ty", false, ""},
		{"x\u0001y", false, ""},
		{"x<br>y", true, "x y"},
		{"x <br> y", true, "x y"},
		{"x&<>y", true, "x&<>y"},
		{"x￾y", false, ""},
		{"x￿y", false, ""},
		{"￾", false, ""},
		{"x�y", true, "x�y"},
		{"x﷐y", true, "x﷐y"},
		{"x\u00a0y", false, ""}, // ASCII 밖 공백·서식 문자(OddSpaces)
		{"x\u2003y", false, ""},
		{"x\u200by", false, ""},
	} {
		CheckSrc(t, render, ctx, fmt.Sprintf("%q", c.v), ctx.Src(c.v), c.ok, c.want)
	}
}

// AriaTab 은 accTitle 의 탭·연속 빈칸이 XML 로 읽은 aria-label 에 그대로 남는지 본다 — 속성의 탭은 &#9; 로 써야 XML 속성값
// 정규화를 지난다. src 는 accTitle 값 v 를 넣은 원문을 만든다.
func AriaTab(t *testing.T, render Render, src func(v string) string) {
	t.Helper()
	for _, v := range []string{"a\tb", "\t", "가\t\t나  다", "x \t y"} {
		r, err := render(src("앞" + v + "뒤"))
		if err != nil {
			t.Fatalf("%q: %v", v, err)
		}
		if got := ParseSVG(t, r.SVG).Aria; got != "앞"+v+"뒤" {
			t.Errorf("%q: aria-label %q\n%s", v, got, r.SVG)
		}
		if !strings.Contains(r.SVG, "&#9;") {
			t.Errorf("%q: aria-label 의 탭이 &#9; 가 아님\n%s", v, r.SVG)
		}
	}
}

// OddSpaces 는 ASCII 밖 공백·서식 문자의 대표다(손 목록 — diagram.OddSpace 를 부르지 않는다): NBSP(U+00A0 — JS \s·Go
// IsSpace 둘 다 공백), EM SPACE(U+2003), 폭 없는 빈칸(U+200B — Cf, 둘 다 공백이 아니지만 빈칸처럼 보인다).
var OddSpaces = []rune{0x00a0, 0x2003, 0x200b}

func hasOdd(v string) bool { return strings.ContainsAny(v, string(OddSpaces)) }

// WrapCases 는 줄바꿈 시험의 긴 값들이다 — 낱말들, 빈칸 없는 영문, 빈칸 없는 한글, 섞음, 그리고 Unicode 공백(OddSpaces)으로
// 낱말을 이은 값(받으면 보이는 글자 그대로여야 하고 — 줄바꿈이 NBSP 를 빈칸으로 바꾸면 안 된다 — 받지 않으면 원문이다).
func WrapCases() []string {
	alpha := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	hangul := strings.Repeat("가나다라마바사아자차카타파하", 3)
	words := "alpha beta gamma delta epsilon zeta eta theta"
	out := []string{words, alpha, hangul, "짧음 " + alpha + " 끝 " + hangul + " z"}
	for _, r := range OddSpaces {
		out = append(out, strings.ReplaceAll(words, " ", string(r)), strings.Replace(words, " ", string(r), 3))
	}
	return out
}

// WrapOutput 은 줄바꿈되는 문맥마다 최종 SVG 의 줄이 값을 보이는 글자 그대로 나눈 것인지(WrapOK) 본다. 한 줄에 다 드는 값도
// 규칙에 맞으면 된다 — 문맥마다 실제로 나뉘는지는 부르는 쪽이 따로 본다(minSplit: 둘 이상 줄로 나뉘어야 하는 값의 수).
func WrapOutput(t *testing.T, render Render, ctxs []Ctx, minSplit int) {
	t.Helper()
	for _, ctx := range ctxs {
		split := 0
		for _, v := range WrapCases() {
			r, err := render(ctx.Src(v))
			if err != nil && hasOdd(v) {
				continue // Unicode 공백이 든 값은 원문이어도 된다(받으면 아래에서 글자 보존을 본다)
			}
			if err != nil {
				t.Fatalf("%s/%q: %v", ctx.Name, v, err)
			}
			ls := ParseSVG(t, r.SVG).LinesOf(v)
			if ls == nil {
				t.Errorf("%s/%q: 규칙대로 나뉜 <text> 가 없음\n%q", ctx.Name, v, ParseSVG(t, r.SVG).Texts)
				continue
			}
			if len(ls) > 1 {
				split++
			}
		}
		if split < minSplit {
			t.Errorf("%s: 줄바꿈된 값 %d 개, 기대 %d 개 이상", ctx.Name, split, minSplit)
		}
	}
	for _, c := range []struct {
		v     string
		lines []string
		ok    bool
	}{
		{"ab cd", []string{"ab", "cd"}, true},
		{"abcd", []string{"ab", "cd"}, true},
		{"ab cd", []string{"ab ", "cd"}, false},
		{"ab cd", []string{"ab", " cd"}, false},
		{"abcd", []string{"a", "cd"}, false},
		{"ab cd", []string{"abcd"}, false},
		{"ab  cd", []string{"ab", "cd"}, false},
		{"ab", []string{"ab", ""}, false},
	} {
		if WrapOK(c.v, c.lines) != c.ok {
			t.Errorf("WrapOK(%q, %q) 가 %v 가 아님", c.v, c.lines, c.ok)
		}
	}
}

// TextFields 는 모델 v 에서 닿는 구조체의 string·[]string 필드를 「형.필드」 이름으로 모은다(내보내지 않은 필드도 — reflect 는
// 형을 볼 수 있다). 맵·포인터·슬라이스 안으로도 내려간다. skip 은 출력하지 않는 필드(부르는 쪽이 까닭을 적는다)다.
func TextFields(v any, skip ...string) map[string]bool {
	out := map[string]bool{}
	seen := map[reflect.Type]bool{}
	var walk func(rt reflect.Type)
	walk = func(rt reflect.Type) {
		for rt.Kind() == reflect.Pointer || rt.Kind() == reflect.Slice || rt.Kind() == reflect.Array || rt.Kind() == reflect.Map {
			if rt.Kind() == reflect.Map {
				walk(rt.Key())
			}
			rt = rt.Elem()
		}
		if rt.Kind() != reflect.Struct || seen[rt] || rt.PkgPath() != "" && !strings.HasPrefix(rt.PkgPath(), "github.com/itda-work/") {
			return
		}
		seen[rt] = true
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			switch {
			case f.Type.Kind() == reflect.String, f.Type.Kind() == reflect.Slice && f.Type.Elem().Kind() == reflect.String:
				out[rt.Name()+"."+f.Name] = true
			default:
				walk(f.Type)
			}
		}
	}
	walk(reflect.TypeOf(v))
	for _, s := range skip {
		if !out[s] {
			panic("policytest.TextFields: 없는 제외 필드 " + s)
		}
		delete(out, s)
	}
	return out
}

// GateFields 는 eachText 가 넘긴 필드 이름이다.
func GateFields(each func(diagram.TextFunc) error) map[string]bool {
	got := map[string]bool{}
	_ = each(func(field, _, _ string, _ int) error { got[field] = true; return nil })
	return got
}

// OddSpacePolicy 는 실제 ASCII 밖 공백·서식 문자(OddSpaces)를 짧은 값·긴 값(줄바꿈되는 길이)·앞뒤에 둔 값으로 라벨·접근성 문맥에
// 넣고, accDescr { … } 블록은 여러 줄 값의 줄 머리에도 넣어 모두 그 줄의 원문 + 경고인지 본다(#409 ④-2 후속 리뷰 1회차 [2]·[3] —
// 받으면 긴 라벨의 줄바꿈이 NBSP 를 빈칸으로 바꾸고, 블록의 줄 머리 정리가 원본 \n\s+ 와 갈린다).
func OddSpacePolicy(t *testing.T, render Render, labels, accs []Ctx) {
	t.Helper()
	long := strings.Repeat("abc ", 30) + "z"
	for _, r := range OddSpaces {
		sp := string(r)
		for _, ctx := range append(append([]Ctx{}, labels...), accs...) {
			line := ctx.Line
			if ctx.OddLine != 0 {
				line = ctx.OddLine
			}
			c := Ctx{Name: ctx.Name, Line: line}
			for _, v := range []string{"x" + sp + "y", strings.ReplaceAll(long, " ", sp), "x" + sp + " y", sp + "x", "x" + sp} {
				CheckSrc(t, render, c, fmt.Sprintf("%U/%q", r, v), ctx.Src(v), false, "")
			}
			if ctx.OddLine != 0 { // 여러 줄 값의 둘째 줄 머리
				c.Line = ctx.OddLine + 1
				CheckSrc(t, render, c, fmt.Sprintf("%U/줄 머리", r), ctx.Src("a\n"+sp+"b"), false, "")
			}
		}
	}
}

// OddSpaceLines 는 특수 공백을 라벨 밖(줄 머리·끝, 문법 낱말 사이)에 둔 줄이 원문인지 본다 — ctx.Src(sp) 가 특수 공백 sp 를 넣은
// 원문이다. Go strings.TrimSpace 는 줄 끝의 NBSP 를 지워 라벨까지 가지 않으므로 줄 루프의 OddSpace 검사만 이 꼴을 막는다(공통
// 함수의 심층 방어와 따로 줄 검사를 시험한다).
func OddSpaceLines(t *testing.T, render Render, ctxs []Ctx) {
	t.Helper()
	for _, r := range OddSpaces {
		for _, ctx := range ctxs {
			CheckSrc(t, render, ctx, fmt.Sprintf("%U", r), ctx.Src(string(r)), false, "")
		}
	}
}

// C1RowIDs 는 C1 제어 문자(U+0080~U+009F) 관찰 행의 식별자다 — diagrams-design.md 2장 「현재 동작 — 지원 약속 아님」
// 표(c1-observe)의 첫 열과 1:1 이다(internal/diagram 의 TestC1ObserveTable 이 대조한다). 행마다 C1Policy 의 같은 이름 갈래가 돈다.
var C1RowIDs = []string{"C1-라벨", "C1-라벨-NEL", "C1-캡션-이스케이프", "C1-캡션-이스케이프-NEL", "C1-캡션-날글자", "C1-캡션-날글자-NEL", "C1-접근성", "C1-접근성-NEL"}

// c1NEL 은 손으로 정한 예외다 — U+0085(NEL)는 C1 이면서 공백(White_Space)이라 다른 갈래를 탄다. 구현의 OddSpace 로 기대를 계산하지
// 않는다.
const c1NEL = 0x85

// C1Policy 는 C1 제어 문자 32개 전부를 C1RowIDs 의 입력 경로마다 넣어 지금 동작을 본다. **지원 약속이 아닌 회귀 감지**다 — C1 이 든
// 입력의 수락 여부와 표시 형태는 호환성 보장 대상이 아니다(2026-10-31 사용자 결정, #409 ④-2 후속). 결과가 바뀌면 의도한 변경인지
// 검토하고 DESIGN 표와 함께 고친다. labels 는 보이는 라벨 문맥, accs 는 접근성 값 문맥, body 는 캡션 뒤에 올 그림 원문이다.
// 기대는 손으로 적었다.
func C1Policy(t *testing.T, render Render, labels, accs []Ctx, body string) {
	t.Helper()
	capRaw := func(v string) string { return "---\ntitle: \"" + v + "\"\n---\n" + body }
	for _, id := range C1RowIDs {
		for r := rune(0x80); r <= 0x9f; r++ {
			name := fmt.Sprintf("%s/%U", id, r)
			v := "x" + string(r) + "y"
			nel := r == c1NEL
			switch id {
			case "C1-라벨": // 라벨의 실제 글자 — NEL 밖은 값 그대로
				if !nel {
					for _, ctx := range labels {
						CheckSrc(t, render, ctx, name, ctx.Src(v), true, v)
					}
				}
			case "C1-라벨-NEL": // 라벨의 실제 NEL — 그 줄의 원문
				if nel {
					for _, ctx := range labels {
						CheckSrc(t, render, ctx, name, ctx.Src(v), false, "")
					}
				}
			case "C1-캡션-이스케이프": // YAML "x\xNNy" — YAML 이 읽은 값에 C1 이 남아 캡션 그대로
				if !nel {
					CheckSrc(t, render, CaptionCtx(body), name, capRaw(fmt.Sprintf("x\\x%02xy", r)), true, v)
				}
			case "C1-캡션-이스케이프-NEL": // YAML "x\x85y" — title 줄의 원문
				if nel {
					CheckSrc(t, render, CaptionCtx(body), name, capRaw("x\\x85y"), false, "")
				}
			case "C1-캡션-날글자": // YAML 따옴표 글 안의 실제 글자 — YAML 오류로 frontmatter 째 원문(1번 줄)
				if !nel {
					CheckSrc(t, render, Ctx{Name: "캡션 날 글자", Line: 1}, name, capRaw(v), false, "")
				}
			case "C1-캡션-날글자-NEL": // YAML 따옴표 글 안의 실제 NEL — YAML 이 줄바꿈으로 접어 「x y」
				if nel {
					CheckSrc(t, render, CaptionCtx(body), name, capRaw(v), true, "x y")
				}
			case "C1-접근성": // 접근성 값의 실제 글자(NEL 밖) — 값의 줄(블록이면 닫는 줄)의 원문
				if !nel {
					for _, ctx := range accs {
						CheckSrc(t, render, ctx, name, ctx.Src(v), false, "")
					}
				}
			case "C1-접근성-NEL": // 접근성 값의 실제 NEL — 그 글자가 든 줄(블록이면 OddLine)의 원문
				if nel {
					for _, ctx := range accs {
						if ctx.OddLine != 0 {
							ctx.Line = ctx.OddLine
						}
						CheckSrc(t, render, ctx, name, ctx.Src(v), false, "")
					}
				}
			default:
				t.Fatalf("C1Policy 가 모르는 행 %s", id)
			}
		}
	}
}
