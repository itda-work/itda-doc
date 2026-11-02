package mdoc

import (
	"math/rand/v2"
	"runtime"
	"strings"
	"testing"
)

// 코드 블록 줄 강조(#479, codehl.go): 펜스 info string 의 {…} 낱말과 펜스 바로 앞 Deckset 지시어
// [.code-highlight: …] 를 블록 줄 수에 맞춰 정규화해 pre data-hl 로 낸다. 강조는 속성값일 뿐 코드 글자에 들어가지 않는다.
func TestCodeHLInfo(t *testing.T) {
	cases := []struct {
		info, lang, title, hl string
		has                   bool
	}{
		{"go {1,3-5}", "go", "", "1,3-5", true},
		{"go{1,3-5}", "go", "", "1,3-5", true}, // 붙여 씀 — 예전엔 언어 go135 였다
		{"js{4}", "js", "", "4", true},         // 깨지는 입력(예전엔 언어 js4)
		{"{2}", "", "", "2", true},             // 언어 없음
		{" {2}", "", "", "2", true},            // 언어 없음(앞 공백)
		{`go title="a.go" {2-3}`, "go", "a.go", "2-3", true},
		{"go:a.go {2}", "go", "a.go", "2", true},
		{"go:a.go{2}", "go", "a.go{2}", "", false},                                 // 줄임 파일명에 붙은 {…} 는 파일명 글자(기존 가르기가 고르는 범위)
		{"go {1 3}", "go", "", "1 3", true},                                        // 공백 구분
		{"go {}", "go", "", "", true},                                              // 빈 강조 — 지시어보다 이긴다
		{`go title="x {2}"`, "go", "x {2}", "", false},                             // 따옴표 안은 파일명
		{`go title="part {2} draft.go" {1}`, "go", "part {2} draft.go", "1", true}, // 따옴표 안 {…} 뒤 공백도 파일명, 바깥만 강조
		{`go title='part {2} draft.go' {1}`, "go", "part {2} draft.go", "1", true},
		{`go title="part {2} draft.go"`, "go", "part {2} draft.go", "", false},
		{`go title='part {2} draft.go'`, "go", "part {2} draft.go", "", false},
		{`go {3} title="part {2} draft.go"`, "go", "part {2} draft.go", "3", true}, // 강조가 앞
		{`go title="{2} a.go" {1}`, "go", "{2} a.go", "1", true},                   // 따옴표 바로 뒤 {
		{`go title="part {2} draft.go`, "go", "", "2", true},                       // 닫히지 않은 따옴표 — 기존 titleRe 는 값 없음(title= 만)으로 읽어 {2} 는 바깥 낱말
		// 리뷰 r2 재현 — 다른 = 뒤 따옴표가 앞서도 기존 titleRe 가 고르는 파일명 그대로, 바깥 {1} 만 강조
		{`go foo=' title="part ' {2} draft.go" {1}`, "go", `part ' {2} draft.go`, "1", true},
		{`go foo=" title='part " {2} draft.go' {1}`, "go", `part " {2} draft.go`, "1", true},
		{`go foo="ignored" title='part {2} draft.go' {1}`, "go", "part {2} draft.go", "1", true},
		{`go title="first {2} file.go" title='second {3} file.go' {1}`, "go", "first {2} file.go", "1", true},
		{`go:short.go title="part {2} file.go" {1}`, "go", "short.go", "1", true},
		{`go title='a=" {2} file.go' {1}`, "go", `a=" {2} file.go`, "1", true},
		{`{1} go:a.go`, "go", "a.go", "1", true}, // 맨 앞 {…} 를 떼면 다음 낱말이 첫 낱말
		// 리뷰 r3 재현 — 앞 공백이 유니코드 공백(전각 U+3000·U+00A0·U+2003)이어도 기존 가르기와 같은 글에서 보호 구간을 고른다
		{"\u3000title=\"part {2} draft.go\" {1}", "", "part {2} draft.go", "1", true},
		{"\u3000title=x{2} {1}", "", "x{2}", "1", true}, // 보호 구간 끝의 {…} — 앞 공백 바이트만큼 좌표를 되돌려야 덮인다
		{"\u3000\u3000title=x.go{2}", "", "x.go{2}", "", false},
		{"\u3000title='part {2} draft.go' {1}", "", "part {2} draft.go", "1", true},
		{"\u3000title=\"part {2} draft.go\"", "", "part {2} draft.go", "", false},
		{"\u00a0title=\"part {2} draft.go\" {1}", "", "part {2} draft.go", "1", true},
		{"\u2003 title='part {2} draft.go' {1}\u3000", "", "part {2} draft.go", "1", true},
		{"\u3000go title=\"part {2} draft.go\" {1}", "go", "part {2} draft.go", "1", true},
		{"\t\u3000go:a{2}.go {1}", "go", "a{2}.go", "1", true},
		{"go\u3000{2}", "go", "", "2", true},                          // 낱말 경계는 unicode.IsSpace — 전각 공백 뒤 {…}
		{"go\u3000{2} title='a {3}.go'", "go", "a {3}.go", "2", true}, // 전각 공백 앞 {…}, 파일명 안 {3} 은 그대로
		{"go\u3000{2}\u3000title='a {3}.go'", "go", "", "2", true},    // titleRe 는 \s(ASCII) 뒤 title= 만 읽어 기존 가르기도 파일명 없음

		{"go {a}", "go", "", "", false}, // 숫자가 아닌 {…} 는 줄 강조가 아니다
		{"go", "go", "", "", false},
		{"yaml:docker-compose.yml", "yaml", "docker-compose.yml", "", false},
	}
	for _, c := range cases {
		got := parseCodeInfo([]byte(c.info))
		if got.lang != c.lang || got.title != c.title || got.hl != c.hl || got.hasHL != c.has {
			t.Errorf("%q → %+v, 기대 {lang:%q title:%q hl:%q has:%v}", c.info, got, c.lang, c.title, c.hl, c.has)
		}
	}
}

// {…} 가 없으면 언어·파일명 결과는 줄 강조 이전 가르기(parseCodeLangTitle)와 바이트까지 같다.
func TestCodeHLInfoUnchanged(t *testing.T) {
	for _, info := range []string{`go title="part {2} x.go"`, `go title='a {1} b'`, "", "go", " yaml ", "yaml:a.yml", `title="a b.go"`, "x=y", `go title="a{1}.go"`, "go{a}", "{x}", "c++", "go:main.go:1"} {
		got := parseCodeInfo([]byte(info))
		want := parseCodeLangTitle(info)
		if got.lang != want.lang || got.title != want.title || got.hasHL {
			t.Errorf("%q → %+v, 기대 %+v(강조 없음)", info, got, want)
		}
	}
}

func TestNormalizeHL(t *testing.T) {
	cases := []struct {
		spec      string
		n         int
		directive bool
		want      string
		bad       int
	}{
		{"1,3-5", 10, false, "1,3-5", 0},
		{"5,1,3-4,2", 10, false, "1-5", 0},          // 정렬·합치기
		{"1 3", 5, false, "1,3", 0},                 // 공백 구분
		{"3-99", 5, false, "3-5", 0},                // 줄 수 밖은 자른다
		{"9", 5, false, "", 0},                      // 줄 수 밖 번호는 버린다(경고 없음)
		{"0,5-3,x,2", 5, false, "2", 3},             // 0·거꾸로·글자는 bad
		{"1-99999999999999999999", 3, false, "", 1}, // 넘침은 bad
		{"1-1000000", 2000, false, "1-1000", 0},     // 상한 maxCodeHLLine
		{"all", 3, true, "1-3", 0},
		{"ALL", 3, true, "1-3", 0},
		{"none", 3, true, "", 0},
		{"all", 3, false, "", 1}, // info 에서는 all 을 받지 않는다(정규식이 막지만 함수도)
		{"1", 0, false, "", 0},   // 빈 블록
	}
	for _, c := range cases {
		got, bad := normalizeHL(c.spec, c.n, c.directive)
		if got != c.want || len(bad) != c.bad {
			t.Errorf("normalizeHL(%q,%d,%v) = %q bad=%v, 기대 %q bad %d개", c.spec, c.n, c.directive, got, bad, c.want, c.bad)
		}
	}
}

func TestCodeHLRender(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
		not  []string
		warn string
	}{
		{name: "앞 전각 공백 + title 안 {…}(리뷰 r3 재현)", src: "```\u3000title=\"part {2} draft.go\" {1}\na\nb\n```\n",
			want: []string{`data-title="part {2} draft.go" data-hl="1">`}},
		{name: "앞 전각 공백 + 작은따옴표 title(리뷰 r3 재현)", src: "```\u3000title='part {2} draft.go' {1}\na\nb\n```\n",
			want: []string{`data-title="part {2} draft.go" data-hl="1">`}},
		{name: "앞 전각 공백 + title 안 {…} 만", src: "```\u3000title=\"part {2} draft.go\"\na\nb\n```\n",
			want: []string{`data-title="part {2} draft.go">`}, not: []string{"data-hl"}},
		{name: "info 공백", src: "```go {1,3-5}\na\nb\nc\nd\ne\nf\n```\n",
			want: []string{`<pre data-line="1" data-hl="1,3-5"><code class="language-go">`}},
		{name: "info 붙임 — 언어가 망가지지 않는다", src: "```js{2}\na\nb\n```\n",
			want: []string{`data-hl="2"><code class="language-js">`}},
		{name: "언어 없음", src: "``` {2}\na\nb\n```\n", want: []string{`data-hl="2"><code>a`}},
		{name: "파일명과 함께", src: "```go title=\"a.go\" {2-3}\na\nb\nc\n```\n",
			want: []string{`data-title="a.go" data-hl="2-3"><code class="language-go">`}},
		{name: "줄 수 밖은 버림", src: "```go {2,9}\na\nb\n```\n", want: []string{`data-hl="2"`}},
		{name: "모두 밖이면 속성 없음", src: "```go {9}\na\nb\n```\n", not: []string{"data-hl"}},
		{name: "거꾸로 범위는 경고", src: "```go {3-1,2}\na\nb\nc\n```\n", want: []string{`data-hl="2"`}, warn: "3-1"},
		{name: "Deckset 지시어 — 문단을 지운다", src: "[.code-highlight: 1, 3-4]\n\n```go\na\nb\nc\nd\n```\n",
			want: []string{`data-hl="1,3-4"`}, not: []string{"code-highlight"}},
		{name: "Deckset 지시어 — 빈 줄 없이 바로 앞", src: "[.code-highlight: 2]\n```go\na\nb\n```\n",
			want: []string{`data-hl="2"`}, not: []string{"code-highlight", "<p>"}},
		{name: "Deckset all", src: "[.code-highlight: all]\n\n```go\na\nb\nc\n```\n", want: []string{`data-hl="1-3"`}},
		{name: "Deckset none", src: "[.code-highlight: none]\n\n```go\na\nb\n```\n", not: []string{"data-hl", "code-highlight"}},
		{name: "Deckset 연속은 첫 것만 + 경고", src: "[.code-highlight: 1]\n[.code-highlight: 2]\n\n[.code-highlight: 3]\n\n```go\na\nb\nc\n```\n",
			want: []string{`data-hl="1"`}, not: []string{"code-highlight"}, warn: "단계별 강조"},
		{name: "info 가 지시어를 이긴다", src: "[.code-highlight: 1]\n\n```go {2}\na\nb\n```\n",
			want: []string{`data-hl="2"`}, not: []string{"code-highlight"}},
		{name: "info 빈 {} 도 지시어를 이긴다", src: "[.code-highlight: 1]\n\n```go {}\na\nb\n```\n",
			not: []string{"data-hl", "code-highlight"}},
		{name: "펜스가 뒤따르지 않으면 글자로 남는다", src: "[.code-highlight: 1]\n\n문단\n\n```go\na\n```\n",
			want: []string{"[.code-highlight: 1]"}, not: []string{"data-hl"}},
		{name: "지시어와 다른 줄이 섞인 문단은 지시어가 아니다", src: "[.code-highlight: 1]\n글\n```go\na\n```\n",
			want: []string{"[.code-highlight: 1]"}, not: []string{"data-hl"}},
		{name: "mermaid 그림에는 없음", src: "```mermaid {1}\ngraph TD\nA-->B\n```\n",
			want: []string{`<figure class="diagram"`}, not: []string{"data-hl"}},
		{name: "촘촘한 목록 — 지시어가 항목 전체(TextBlock)", src: "- [.code-highlight: 2]\n  ```go\n  a\n  b\n  ```\n",
			want: []string{`data-hl="2"`}, not: []string{"code-highlight"}},
		{name: "촘촘한 목록 — 다른 항목과 이어진 목록", src: "- 앞 항목\n- [.code-highlight: 1]\n  ```go\n  a\n  b\n  ```\n- 뒤 항목\n",
			want: []string{`data-hl="1"`, "앞 항목", "뒤 항목"}, not: []string{"code-highlight"}},
		{name: "촘촘한 목록 — 지시어가 다른 글과 한 문단이면 글자로 남는다", src: "- 글\n  [.code-highlight: 2]\n  ```go\n  a\n  b\n  ```\n",
			want: []string{"[.code-highlight: 2]"}, not: []string{"data-hl"}},
		{name: "느슨한 목록 — 지시어가 다른 글과 한 문단이면 글자로 남는다", src: "- 글\n  [.code-highlight: 2]\n\n  ```go\n  a\n  b\n  ```\n\n- 뒤\n",
			want: []string{"[.code-highlight: 2]"}, not: []string{"data-hl"}},
		{name: "목록 안 펜스와 지시어", src: "- 항목\n\n  [.code-highlight: 2]\n  ```go\n  a\n  b\n  ```\n",
			want: []string{`data-hl="2"`}, not: []string{"code-highlight"}},
	}
	for _, c := range cases {
		r := render(t, c.src)
		for _, w := range c.want {
			if !strings.Contains(r.HTML, w) {
				t.Errorf("%s: 출력에 %q 없음:\n%s", c.name, w, r.HTML)
			}
		}
		for _, n := range c.not {
			if strings.Contains(r.HTML, n) {
				t.Errorf("%s: 출력에 %q 있음:\n%s", c.name, n, r.HTML)
			}
		}
		found := false
		for _, w := range r.Warnings {
			if c.warn != "" && w.Code == WarnBadAttribute && strings.Contains(w.Message, c.warn) {
				found = true
			}
		}
		if c.warn != "" && !found {
			t.Errorf("%s: 경고 %q 없음: %+v", c.name, c.warn, r.Warnings)
		}
		if c.warn == "" && len(r.Warnings) > 0 {
			t.Errorf("%s: 뜻밖의 경고 %+v", c.name, r.Warnings)
		}
	}
}

// 강조는 코드 글자에 들어가지 않는다 — code 안 글자는 강조 없는 같은 블록과 같다.
func TestCodeHLNotInCode(t *testing.T) {
	code := func(src string) string {
		h := render(t, src).HTML
		i, j := strings.Index(h, "<code"), strings.Index(h, "</code>")
		if i < 0 || j < i {
			t.Fatalf("코드 요소를 못 찾음:\n%s", h)
		}
		return h[i:j]
	}
	if a, b := code("```go {1-2}\nx := 1\ny := 2\n```\n"), code("```go\nx := 1\ny := 2\n```\n"); a != b {
		t.Errorf("강조가 코드 글자를 바꿨다:\n%s\n---\n%s", a, b)
	}
}

// 연속 지시어 수집은 문단 수에 선형이다(#479 리뷰 [3] — 앞 문단마다 모은 값 전체를 복사하면 누적 할당이 제곱으로
// 는다). 시간 대신 렌더 한 번의 누적 할당(TotalAlloc)을 2000·8000 문단에서 견준다 — 선형이면 약 4배, 제곱이면
// 약 15배(리뷰 실측 37MB → 554MB)라 8배를 경계로 둔다.
func TestCodeHLDirectivesLinear(t *testing.T) {
	alloc := func(n int) uint64 {
		src := strings.Repeat("[.code-highlight: 1]\n\n", n) + "```go\na\n```\n"
		render(t, src) // 데우기
		var a, b runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&a)
		r := render(t, src)
		runtime.ReadMemStats(&b)
		if !strings.Contains(r.HTML, `data-hl="1"`) || strings.Contains(r.HTML, "code-highlight") {
			t.Fatalf("%d 문단: 지시어가 적용되지 않음", n)
		}
		return b.TotalAlloc - a.TotalAlloc
	}
	small, large := alloc(2000), alloc(8000)
	t.Logf("2000: %d B, 8000: %d B", small, large)
	if ratio := float64(large) / float64(small); ratio > 8 {
		t.Errorf("지시어 문단 4배에 누적 할당 %.1f배(2000: %d B, 8000: %d B) — 선형이 아니다", ratio, small, large)
	}
}

// 속성(#479 리뷰 2회차 — 접근 방식): 줄 강조 낱말을 떼어도 기존 가르기가 고르는 파일명은 바뀌지 않는다.
//   - P1: parseCodeInfo(s).title == parseCodeLangTitle(떼어 낸 나머지).title (떼어 낸 뒤 가르기와 이어져 있다)
//   - P2: {…} 낱말이 맨 앞 낱말이 아니면 parseCodeInfo(s).title == parseCodeLangTitle(s).title (보호 구간 밖에서만 뗀다)
//   - P3: s 의 { 가 모두 기존 가르기가 고른 파일명 안에 있으면 강조 없음 + title 그대로
//
// 리뷰 r1·r2 의 재현 입력과 손으로 고른 경계, 그리고 고정 시드로 조각을 이어 붙인 무작위 입력을 쓴다.
func TestCodeHLTitleProperty(t *testing.T) {
	inputs := []string{
		`go title="part {2} draft.go" {1}`, `go title='part {2} draft.go' {1}`,
		`go title="part {2} draft.go"`, `go title='part {2} draft.go'`,
		`go foo=' title="part ' {2} draft.go" {1}`, `go foo=" title='part " {2} draft.go' {1}`,
		`go foo="ignored" title='part {2} draft.go' {1}`,
		`go title="first {2} file.go" title='second {3} file.go' {1}`,
		`go:short.go title="part {2} file.go" {1}`, `go:a{1}.go {2}`, `go:a.go{2}`, `go:a.go {2}`,
		`go title='a=" {2} file.go' {1}`, `go title="a\" {2} draft.go" {1}`,
		`go other="unfinished title='part {2} file.go' {1}`, `go title="part {2} draft.go`,
		`go title=a{1} {2}`, `title="{2} a.go" {1}`, `{1} title="x {2}.go"`, `js{4}`, `{2}`,
		`go {3} title="part {2} draft.go"`, `:{1}.go {2}`, `go:{1} {2}`,
	}
	parts := []string{"go", "go:a.go", "go:{1}.go", ":{2}.go", "go:x{1}", ":y{2}", `title="p {2} q.go"`, `title='p {3} q.go'`,
		`title=x{1}.go`, `title="`, `title='`, `foo='`, `foo="`, `foo="a {1} b"`, "{1}", "{2-3}", "{}", "go{4}",
		"x=1", "'", `"`, `\`, "a{5}", `title="{1}"`}
	// 앞·뒤·낱말 사이 공백은 조각과 따로 고른다 — 기존 가르기의 TrimSpace·IsSpace 가 받지만 titleRe 의 \s(ASCII)와 다른 공백을 섞는다
	spaces := []string{"", " ", "  ", "\t", "\u3000", "\u00a0", "\u2003", "\u3000 ", " \u00a0"}
	pick := func(rng *rand.Rand, nonEmpty bool) string {
		for {
			if sp := spaces[rng.IntN(len(spaces))]; sp != "" || !nonEmpty {
				return sp
			}
		}
	}
	rng := rand.New(rand.NewPCG(479, 2))
	for range 3000 {
		n := 1 + rng.IntN(5)
		w := make([]string, n)
		for k := range w {
			w[k] = parts[rng.IntN(len(parts))]
		}
		inputs = append(inputs, strings.Join(w, " "))
		var b strings.Builder
		b.WriteString(pick(rng, false))
		for k, x := range w {
			if k > 0 {
				b.WriteString(pick(rng, true))
			}
			b.WriteString(x)
		}
		b.WriteString(pick(rng, false))
		inputs = append(inputs, b.String())
	}
	for _, s := range inputs {
		got := parseCodeInfo([]byte(s))
		rest, spec, found := cutHLWord(s)
		if got.title != parseCodeLangTitle(rest).title || got.hl != spec || got.hasHL != found {
			t.Errorf("P1 %q: %+v, 나머지 %q 의 가르기와 다름", s, got, rest)
		}
		orig := parseCodeLangTitle(s)
		if found && !strings.HasPrefix(strings.TrimSpace(s), "{"+spec+"}") && got.title != orig.title {
			t.Errorf("P2 %q: title %q, 떼기 전 %q", s, got.title, orig.title)
		}
		if strings.Count(s, "{") > 0 && strings.Count(s, "{") == strings.Count(orig.title, "{") {
			if got.hasHL || got.title != orig.title {
				t.Errorf("P3 %q: {…} 가 파일명 안에만 있는데 %+v(떼기 전 title %q)", s, got, orig.title)
			}
		}
	}
}
