package mdoc

import (
	"bytes"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/itda-work/itda-doc/internal/timingtest"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"
)

// 덮개를 씌운 linkify 는 원래 linkify 와 결과가 같다 — 이메일·URL·문장부호가 섞인 무작위 글 2만 개를 두 goldmark 로 견준다.
func TestLinkifyGuardSame(t *testing.T) {
	plain := goldmark.New(goldmark.WithExtensions(extension.Linkify))
	rng := rand.New(rand.NewSource(116))
	atoms := []string{"a", "b", "1", "_", "*", "~", "(", ")", "-", ".", "@", " ", "x.com", "example.org", "http://e.com/a", "www.e.com", "https:", "ftp://f.io", "!", "+", "%", "\n", "[", "](u)", "가",
		"www.", "www.1", "http://", "ftp:", "http://1", ".c", ":", "/", "#", "?", "=", "`", "\\", "\t", "|"}
	for i := range 20000 {
		var b strings.Builder
		for range 1 + rng.Intn(24) {
			b.WriteString(atoms[rng.Intn(len(atoms))])
		}
		src := b.String()
		guarded := goldmark.New(goldmark.WithParserOptions(parser.WithInlineParsers(util.Prioritized(newLinkifyGuard(), 999))))
		var want, got bytes.Buffer
		if err := plain.Convert([]byte(src), &want); err != nil {
			t.Fatal(err)
		}
		if err := guarded.Convert([]byte(src), &got); err != nil {
			t.Fatal(err)
		}
		if want.String() != got.String() {
			t.Fatalf("#%d %q\n원래 %s\n덮개 %s", i, src, want.String(), got.String())
		}
	}
	// 손으로 고른 경우: 긴 줄 뒤의 이메일, 문장부호로 시작하는 줄 안의 이메일, URL
	for _, src := range []string{"a_b_c_d_x@example.com 끝", "_-a_b@x.com", "a_b_c www.e.com/x", "a_b_http://e.com", "(a_b@c.org)", "~~a_b@c.org~~"} {
		var want bytes.Buffer
		_ = plain.Convert([]byte(src), &want)
		r := render(t, src)
		for _, href := range []string{"mailto:", "http"} {
			if strings.Contains(want.String(), `href="`+href) != strings.Contains(r.HTML, `href="`+href) {
				t.Errorf("%q: 원래 %s / mdoc %s", src, want.String(), r.HTML)
			}
		}
	}
}

// linkify 가 이메일 글자만으로 된 긴 줄에서 이차 시간이 되지 않는다(고치기 전 `a_b` 반복 112KB 0.55초, 448KB 수 초).
// URL 접두사만 맞고 URL 이 아닌 꼴(www.1_ — 리뷰 R2: 0.35 → 1.19 → 4.35초)과 접두사 뒤에 "." + 글자가 있어
// 자리마다 정규식을 도는 꼴(www.a_)도. 크기 셋이라 쌍마다 판정한다(timingtest judge) — 크기가 두 배가 될 때마다 비용(CPU 시간)이
// 세 배(+10ms)를 넘지 않는다(이차면 네 배).
func TestLinkifyLinear(t *testing.T) {
	for _, unit := range []string{"a_b", "a*b", "a~b", "a(b", "a_b.", "$a+b", "a_b@", "www.1_", "www.1*", "www.a_", "http://1_", "http://a.b_", "ftp:1~"} {
		gen := func(size int) string { return strings.Repeat(unit, size/len(unit)) }
		timingtest.AssertDoubling(t, fmt.Sprintf("%q", unit), []int{112_000, 224_000, 448_000}, renderWork(t, gen),
			timingtest.Doubling{Reps: 1, Slack: 10 * time.Millisecond, Limit: 500 * time.Millisecond})
	}
}
