package mdoc

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// 번호 목록의 자릿수 class(#575): 가장 큰 번호(시작 번호 + 항목 수 − 1)의 자릿수가 둘 이상이면 <ol> 에 ol-dN 을 단다.
// 한 자리 목록·글머리 목록에는 class 를 달지 않고, style 속성도 내지 않는다.
func TestOrderedListDigitsClass(t *testing.T) {
	items := func(start, n int) string {
		var b strings.Builder
		for i := range n {
			b.WriteString(strconv.Itoa(start+i) + ". 항목\n")
		}
		return b.String()
	}
	olTag := regexp.MustCompile(`<ol[^>]*>`)
	cases := []struct {
		name, src string
		want      []string // 차례대로 나오는 <ol …> 태그
	}{
		{"아홉 항목", items(1, 9), []string{`<ol data-line="1">`}},
		{"열 항목", items(1, 10), []string{`<ol class="ol-d2" data-line="1">`}},
		{"시작 번호 9·두 항목", items(9, 2), []string{`<ol start="9" class="ol-d2" data-line="1">`}},
		{"시작 번호 8·두 항목", items(8, 2), []string{`<ol start="8" data-line="1">`}},
		{"시작 번호 0·열 항목", items(0, 10), []string{`<ol start="0" data-line="1">`}},
		{"세 자리", items(100, 2), []string{`<ol start="100" class="ol-d3" data-line="1">`}},
		{"99 까지", items(98, 2), []string{`<ol start="98" class="ol-d2" data-line="1">`}},
		{"네 자리", items(999, 2), []string{`<ol start="999" class="ol-d4" data-line="1">`}},
		// 시작 번호는 아홉 자리까지라(CommonMark) 열 자리 번호는 항목 수로만 나온다 — 999999998 + 3 − 1 = 1000000000. 열 자리 이상도 ol-d9
		{"열 자리", "999999998. 항목\n2. 항목\n3. 항목\n", []string{`<ol start="999999998" class="ol-d9" data-line="1">`}},
		// 중첩: 바깥(세 항목)과 안(시작 12)을 따로 센다. 1 이 아닌 번호 목록은 문단을 끊지 못해(CommonMark) 빈 줄을 둔다
		{"중첩", "1. 바깥\n2. 바깥\n3. 바깥\n\n   12. 안\n   13. 안\n", []string{`<ol data-line="1">`, `<ol start="12" class="ol-d2">`}},
		{"바깥만 두 자리", items(1, 9) + "10. 열\n    1. 안\n", []string{`<ol class="ol-d2" data-line="1">`, `<ol>`}},
	}
	for _, c := range cases {
		h := render(t, c.src).HTML
		got := olTag.FindAllString(h, -1)
		if strings.Join(got, " ") != strings.Join(c.want, " ") {
			t.Errorf("%s: <ol> 태그 %q, 원함 %q\n%s", c.name, got, c.want, h)
		}
		if strings.Contains(h, "style=") {
			t.Errorf("%s: style 속성을 냄\n%s", c.name, h)
		}
	}
	// 글머리 목록은 항목이 많아도 대상이 아니다
	var b strings.Builder
	for range 12 {
		b.WriteString("- 항목\n")
	}
	if h := render(t, b.String()).HTML; strings.Contains(h, "ol-d") || !strings.Contains(h, `<ul data-line="1">`) {
		t.Errorf("글머리 목록\n%s", h)
	}
}

// 각주 목록도 자릿수 class 를 단다(#575): 각주 아홉 개는 class 없음, 열 개는 ol-d2. 각주는 1 부터 차례라 각주 수가 가장 큰 번호다.
func TestFootnoteListDigitsClass(t *testing.T) {
	notes := func(n int) string {
		var body, defs strings.Builder
		for i := 1; i <= n; i++ {
			body.WriteString("본문[^" + strconv.Itoa(i) + "] ")
			defs.WriteString("[^" + strconv.Itoa(i) + "]: 각주 " + strconv.Itoa(i) + "\n")
		}
		return body.String() + "\n\n" + defs.String()
	}
	fnOl := regexp.MustCompile(`<div class="footnotes"[^>]*>\s*<hr>\s*(<ol[^>]*>)`)
	for _, c := range []struct {
		n    int
		want string
	}{{9, `<ol>`}, {10, `<ol class="ol-d2">`}, {100, `<ol class="ol-d3">`}} {
		h := render(t, notes(c.n)).HTML
		m := fnOl.FindStringSubmatch(h)
		if m == nil || m[1] != c.want {
			t.Errorf("각주 %d개: 각주 목록 태그 %q, 원함 %q\n%s", c.n, m, c.want, h)
		}
		if got := strings.Count(h, `<li id="fn:`); got != c.n {
			t.Errorf("각주 %d개: li %d개", c.n, got)
		}
	}
}
