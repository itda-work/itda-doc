package mdoc

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/itda-work/itda-doc/internal/timingtest"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// assertLinear 는 gen(size) 를 크기 두 배씩(64KB~512KB) 렌더해 512KB 의 비용이 64KB 의 27배(3³, +3×5ms)를 넘지 않는지 본다(이차면 64배).
// 비용은 CPU 시간이고 두 번 재어 작은 쪽, 실패로 보이면 모든 크기를 다시 잰다(timingtest — 부하에 흔들리지 않게, #201). 크기마다 limit 안이어야 한다.
func assertLinear(t *testing.T, name string, gen func(size int) string, limit time.Duration) {
	t.Helper()
	timingtest.AssertDoubling(t, name, []int{64_000, 128_000, 256_000, 512_000}, renderWork(t, gen),
		timingtest.Doubling{Reps: 2, Slack: 5 * time.Millisecond, Limit: limit})
}

// renderWork 는 gen(size) 의 원문을 만들어 두고 그것을 렌더하는 일을 돌려준다(timingtest 가 잰다 — 원문 만들기는 재지 않는다).
func renderWork(t *testing.T, gen func(size int) string) func(int) func() {
	return func(size int) func() {
		src := []byte(gen(size))
		return func() {
			if _, err := Render(src, Options{}); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func repeatTo(unit string) func(int) string {
	return func(size int) string { return strings.Repeat(unit, size/len(unit)+1) }
}

// 강조 구분자 수 상한(cjk.go). 고치기 전: 짝이 될 수 없는 구분자가 번갈아 쌓인 `_a*` 반복 128KB 가 9초(순수 goldmark 도 같다).
func TestDelimiterLimit(t *testing.T) {
	for _, unit := range []string{"_a*", "*a_", "~# *", "#_1*", "*a", "_a", "~a*"} {
		assertLinear(t, unit, repeatTo(unit), 600*time.Millisecond)
	}
	// 상한까지는 그대로 강조, 넘는 것은 글자(원문 그대로) + 경고 한 번
	r := render(t, strings.Repeat("*a* ", MaxDelimitersPerBlock))
	if n := strings.Count(r.HTML, "<em>a</em>"); n != MaxDelimitersPerBlock/2 {
		t.Errorf("상한 안의 강조 %d 개, 기대 %d", n, MaxDelimitersPerBlock/2)
	}
	if !strings.Contains(r.HTML, "*a* *a*") || len(r.Warnings) != 1 || r.Warnings[0].Code != WarnTooComplex {
		t.Errorf("상한 뒤는 글자 + 경고 하나: %+v", r.Warnings)
	}
	// 문단마다 센다 — 다음 문단은 다시 강조
	r = render(t, strings.Repeat("*a* ", MaxDelimitersPerBlock)+"\n\n*b*")
	contains(t, r.HTML, "<em>b</em>")
	// 보통 문서에는 경고가 없다
	if r := render(t, strings.Repeat("**굵게** 와 *기울임* 과 ~~취소~~ ", 100)); len(r.Warnings) > 0 {
		t.Errorf("보통 문단에 경고: %+v", r.Warnings)
	}
}

// 중첩 상한(nesting.go, #126). 상한 없이: `>>>>…` 한 줄 128KB 3.8초, `1. 1. …` 1.3초(둘 다 순수 goldmark 도 같다), `:::a` 줄 반복 17초.
// 넘을 수 있는 문서는 통째 원문 글자다(안전 모드).
func TestQuoteDepthLimit(t *testing.T) {
	for _, unit := range []string{">", "> ", ">>x\n", "1. ", "- ", "* ", "+ ", ":::a\n", "> - ", "- > ", "[^a]: ", "> :::a\n"} {
		assertLinear(t, unit, repeatTo(unit), 300*time.Millisecond)
	}
	r := render(t, strings.Repeat(">", MaxNestingDepth+10)+" 끝")
	if !isRaw(r) || strings.Contains(r.HTML, "<blockquote") {
		t.Errorf("안전 모드가 아님: %.200s", r.HTML)
	}
	contains(t, r.HTML, strings.Repeat("&gt;", MaxNestingDepth+10)+" 끝</span>")
	if len(r.Warnings) != 1 || r.Warnings[0].Code != WarnTooComplex || r.Warnings[0].Line != 1 {
		t.Errorf("경고 하나 기대: %+v", r.Warnings)
	}
	if r := render(t, "> 가\n>> 나\n>>> 다"); strings.Count(r.HTML, "<blockquote") != 3 || len(r.Warnings) > 0 {
		t.Errorf("보통 인용: %s %+v", r.HTML, r.Warnings)
	}
}

// 컨테이너·목록도 같은 중첩 상한이다 — 인용·목록·::: 를 합해 센다. 64겹까지는 보통 렌더다.
func TestNestingLimit(t *testing.T) {
	for _, src := range []string{strings.Repeat(":::a\n", MaxNestingDepth+5) + "끝\n", strings.Repeat("1. ", MaxNestingDepth+5) + "끝",
		strings.Repeat("> - ", MaxNestingDepth/2+1) + "끝", strings.Repeat("[^a]: ", MaxNestingDepth+1) + "끝"} {
		r := render(t, src)
		if !isRaw(r) || !hasWarn(r, WarnTooComplex) || !strings.Contains(r.HTML, "끝") {
			t.Errorf("%q: 안전 모드가 아님 %+v", shortSrc(src), r.Warnings)
		}
	}
	r := render(t, strings.Repeat(":::a\n", MaxNestingDepth)+"끝\n")
	if n := strings.Count(r.HTML, `<div class="a"`); n != MaxNestingDepth || hasWarn(r, WarnTooComplex) {
		t.Errorf("컨테이너 64겹: %d 겹, %+v", n, r.Warnings)
	}
	r = render(t, strings.Repeat("> - ", MaxNestingDepth/2))
	if n := strings.Count(r.HTML, "<blockquote") + strings.Count(r.HTML, "<ul"); n != MaxNestingDepth || len(r.Warnings) > 0 {
		t.Errorf("인용 + 목록 %d 겹, 기대 %d", n, MaxNestingDepth)
	}
	// 보통 문서는 그대로
	if r := render(t, "- 가\n  - 나\n    - 다\n\n:::box\n:::note\n안\n:::\n:::\n\n> 1. 인용 안 목록"); len(r.Warnings) > 0 {
		t.Errorf("보통 중첩에 경고: %+v", r.Warnings)
	}
}

// 같은 제목이 많아도 id 를 붙이는 시간은 선형이다(고치기 전: 같은 제목 16000개 4.4초 — 겹칠 때마다 -1 부터 다시 찾았다).
func TestHeadingIDsLinear(t *testing.T) {
	// 상한 600ms: 512KB 가 GitHub 러너에서 410ms(256KB 179ms — 선형).
	assertLinear(t, "같은 제목", repeatTo("# 제목\n"), 600*time.Millisecond)
	r := render(t, "# a\n# a\n# a-1\n# a\n# a {#a-3}\n# a\n")
	for _, id := range []string{`id="a"`, `id="a-1"`, `id="a-1-1"`, `id="a-2"`, `id="a-3"`, `id="a-4"`} {
		contains(t, r.HTML, id)
	}
}

// 링크 닫기 시도 상한(linkclose.go). 고치기 전: `[a](` 반복 128KB 5초 — 링크 파서가 ]( 마다 주소를 줄 끝까지 읽었다.
func TestLinkCloserLimit(t *testing.T) {
	for _, unit := range []string{"[a](", "[a]", "![a](", "[", "]("} {
		// 상한 1s: 옛 상한 300ms(256KB)는 Mac 의 `[a](` 256KB 170ms 에 1.76배 여유였다. GitHub 러너는 Mac 의 약 2배이고
		// 512KB 는 그 두 배라 같은 여유면 약 1.05s — 러너에서 `[a](` 512KB 594ms.
		assertLinear(t, unit, repeatTo(unit), time.Second)
	}
	r := render(t, strings.Repeat("[x](#u) ", MaxLinkClosersPerBlock+50))
	if n := strings.Count(r.HTML, `<a href="#u">x</a>`); n != MaxLinkClosersPerBlock {
		t.Errorf("링크 %d 개, 기대 %d", n, MaxLinkClosersPerBlock)
	}
	if !strings.Contains(r.HTML, "[x](#u) [x](#u)") || len(r.Warnings) != 1 || r.Warnings[0].Code != WarnTooComplex {
		t.Errorf("상한 뒤는 글자 + 경고 하나: %+v", r.Warnings)
	}
	if r := render(t, strings.Repeat("[링크](https://a.example) 와 ![그림](a.png) ", 40)); len(r.Warnings) > 0 {
		t.Errorf("보통 문단에 경고: %+v", r.Warnings)
	}
}

// 리뷰 R4: 취소선이 될 수 없는 긴 ~ 묶음은 통째로 글자다(고치기 전 `~` 128KB 3.9초 — 다음 ~ 마다 묶음을 다시 읽었다).
// 짧은 ~ 조합의 HTML 은 goldmark GFM 과 같다.
func TestTildeRuns(t *testing.T) {
	for name, gen := range map[string]func(int) string{
		"긴 ~ 묶음 하나":   func(n int) string { return "a " + strings.Repeat("~", n) + " b" },
		"~~~ 반복":      repeatTo("~~~a"),
		"~ 붙은 ~~ 반복":  repeatTo("~~~~ "),
		"글자 사이 긴 묶음":  func(n int) string { return "가" + strings.Repeat("~", n) + "나" },
		"*·~ 긴 묶음 섞음": func(n int) string { return strings.Repeat("~", n/2) + strings.Repeat("*", n/2) },
	} {
		assertLinear(t, name, gen, 100*time.Millisecond)
	}
	gfm := goldmark.New(goldmark.WithExtensions(extension.GFM))
	dataLine := regexp.MustCompile(` data-line="\d+"`)
	for _, src := range []string{"a~b~", "~a~", "~~a~~", "x ~~~a~~~", "a ~~~ b", "~~a~", "~a~~", "x~~~~y", "~~ a ~~", "a~~b~~c", "~~~", "~", "~~", "a ~ b ~ c", "~~a~~~b~~", "(~~a~~)"} {
		var want bytes.Buffer
		_ = gfm.Convert([]byte(src), &want)
		got := dataLine.ReplaceAllString(render(t, src).HTML, "")
		if got != want.String() {
			t.Errorf("%q: GFM %q · mdoc %q", src, want.String(), got)
		}
	}
}
