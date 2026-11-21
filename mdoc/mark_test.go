package mdoc

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

// 형광펜 ==글== (cjk.go cjkMarkParser, itda-hyve#992).

var firstParaRe = regexp.MustCompile(`(?s)<p[^>]*>(.*?)</p>`)

// paraInner 는 본문 첫 문단의 안쪽 HTML 이다.
func paraInner(t *testing.T, src string) string {
	t.Helper()
	r := render(t, src)
	m := firstParaRe.FindStringSubmatch(r.HTML)
	if m == nil {
		t.Fatalf("%q: 문단이 없음\n%s", src, r.HTML)
	}
	return m[1]
}

func TestMarkInline(t *testing.T) {
	for src, want := range map[string]string{
		"이것은 ==강조== 다":       "이것은 <mark>강조</mark> 다",
		"==강조==는 중요":         "<mark>강조</mark>는 중요",
		"값은==강조==이다":         "값은<mark>강조</mark>이다",
		"(==강조==)":           "(<mark>강조</mark>)",
		"==**굵게** 와 *기울임*==": "<mark><strong>굵게</strong> 와 <em>기울임</em></mark>",
		"**==안==**":          "<strong><mark>안</mark></strong>",
		"==Go== 언어":          "<mark>Go</mark> 언어",
		"x==1 이고 y==2":       "x==1 이고 y==2",
		"a==b==c":            "a==b==c",
		"a == b 그리고 c == d":  "a == b 그리고 c == d",
		"===강조===":           "===강조===",
		"==== 넷":             "==== 넷",
		"\\==강조==":           "==강조==",
		"`==코드==`":           "<code>==코드==</code>",
		"==닫지 않음":            "==닫지 않음",
		"<mark>글</mark> 그대로": "<mark>글</mark> 그대로", // raw <mark> 는 바뀌지 않는다
		"==가== =나= ==다==":    "<mark>가</mark> =나= <mark>다</mark>",
		"==~~취소~~==":         "<mark><del>취소</del></mark>",
		"~~==안==~~":          "<del><mark>안</mark></del>",
		"==가 *기울임== 밖*":      "<mark>가 *기울임</mark> 밖*", // 짝이 엇갈리면 먼저 닫힌 쪽이 이긴다(강조와 같은 규칙)
		"==가\n나==":           "<mark>가\n나</mark>",
		"\\===가==":           "===가==",   // \= 바로 뒤의 == 는 글자(취소선의 ~ 뒤 ~ 와 같다)
		"==가==a 다":           "==가==a 다", // 닫는 == 뒤가 ASCII 영문자 — 닫지 못한다
		"a==가== 다":           "a==가== 다", // 여는 == 앞이 ASCII 영문자 — 열지 못한다
		"1==가==2":            "1==가==2",
		"=":                  "=",
		"====":               "====",
		// 자동 링크: 맨 주소는 경로가 없을 때만 칠해진다 — 경로가 있으면 GFM 자동 링크가 닫는 == 까지 주소로 읽는다
		"==https://a.example==":           `<mark><a href="https://a.example">https://a.example</a></mark>`,
		"==https://a.example/path==":      `==<a href="https://a.example/path==">https://a.example/path==</a>`,
		"==<https://a.example/path>==":    `<mark><a href="https://a.example/path">https://a.example/path</a></mark>`,
		"==[글](https://a.example/path)==": `<mark><a href="https://a.example/path">글</a></mark>`,
	} {
		if got := paraInner(t, src); got != want {
			t.Errorf("%q\n얻음 %q\n기대 %q", src, got, want)
		}
	}
	// 형광펜이 아닌 = 뒤는 낱말 머리가 아니다 — 주소 글자가 자동 링크가 되지 않는다(= 를 파서가 보기 전과 같다)
	for _, src := range []string{"키=https://x.example 값", "x==https://x.example 값", "가===https://x.example 값", "{poster=https://x.example/p.jpg}"} {
		if got := paraInner(t, src); strings.Contains(got, "<a ") {
			t.Errorf("%q: 자동 링크가 걸림 %q", src, got)
		}
	}
	// 링크 안
	contains(t, paraInner(t, "[==링크==](https://example.com)"), `<mark>링크</mark></a>`)
	// 수식 안은 TeX 다 — 형광펜이 끼지 않는다
	if got := render(t, "$a==b$ 와 $c==d$").HTML; strings.Contains(got, "<mark>") || strings.Count(got, "<math>") != 2 {
		t.Errorf("수식 안 ==: %s", got)
	}
	// 표 칸 안
	r := render(t, "| 가 |\n|---|\n| ==칸== |\n")
	contains(t, r.HTML, "<td><mark>칸</mark></td>")
	// setext 제목은 그대로
	r = render(t, "제목\n===\n")
	contains(t, r.HTML, ">제목</h1>")
	notContains(t, r.HTML, "<mark>")
	// 보통 문서에는 경고가 없다
	if r := render(t, "이것은 ==강조== 다. x==1 이고 a == b, ===셋===."); len(r.Warnings) > 0 {
		t.Errorf("경고: %+v", r.Warnings)
	}
}

// 제목 안 형광펜: 제목 id·목차 글은 같은 글의 굵게(**)와 같다 — == 가 남지 않는다.
func TestMarkHeading(t *testing.T) {
	a, b := render(t, "# ==중요== 공지\n"), render(t, "# **중요** 공지\n")
	contains(t, a.HTML, "<mark>중요</mark> 공지</h1>")
	if len(a.TOC) != 1 || len(b.TOC) != 1 {
		t.Fatalf("목차: %+v / %+v", a.TOC, b.TOC)
	}
	if a.TOC[0].ID != b.TOC[0].ID || a.TOC[0].Text != b.TOC[0].Text || strings.Contains(a.TOC[0].Text, "=") {
		t.Errorf("목차 %+v ≠ %+v", a.TOC[0], b.TOC[0])
	}
	idRe := regexp.MustCompile(`<h1 id="([^"]*)"`)
	if ia, ib := idRe.FindStringSubmatch(a.HTML), idRe.FindStringSubmatch(b.HTML); ia == nil || ib == nil || ia[1] != ib[1] {
		t.Errorf("제목 id: %v / %v", ia, ib)
	}
}

// 문단 구분자 상한에 = 묶음도 센다(cjk.go tooManyDelims). 상한 뒤는 글자 + 경고 한 번.
func TestMarkDelimiterLimit(t *testing.T) {
	r := render(t, strings.Repeat("==a== ", MaxDelimitersPerBlock))
	if n := strings.Count(r.HTML, "<mark>a</mark>"); n != MaxDelimitersPerBlock/2 {
		t.Errorf("상한 안의 형광펜 %d 개, 기대 %d", n, MaxDelimitersPerBlock/2)
	}
	if !strings.Contains(r.HTML, "==a== ==a==") || len(r.Warnings) != 1 || r.Warnings[0].Code != WarnTooComplex ||
		!strings.Contains(r.Warnings[0].Message, "(* _ ~ =)") {
		t.Errorf("상한 뒤는 글자 + 경고 하나: %+v", r.Warnings)
	}
	// * 와 = 를 합해 센다
	r = render(t, strings.Repeat("*a* ==b== ", MaxDelimitersPerBlock/4+10))
	if !hasWarn(r, WarnTooComplex) {
		t.Errorf("* 와 = 합이 상한을 넘어도 경고가 없음")
	}
}

// 긴 = 묶음·== 반복이 선형이다(limits_test.go 의 방식).
func TestMarkRunsLinear(t *testing.T) {
	for name, gen := range map[string]func(int) string{
		"긴 = 묶음 하나":    func(n int) string { return "a " + strings.Repeat("=", n) + " b" },
		"글자 사이 긴 = 묶음": func(n int) string { return "가" + strings.Repeat("=", n) + "나" },
		"==a 반복":       repeatTo("==a"),
		"==a 띄어 반복":    repeatTo("==a "),
		"===a 반복":      repeatTo("===a"),
		"_a== 반복":      repeatTo("_a=="),
		"=·* 긴 묶음 섞음":  func(n int) string { return strings.Repeat("=", n/2) + strings.Repeat("*", n/2) },
	} {
		assertLinear(t, name, gen, 600*time.Millisecond)
	}
}
