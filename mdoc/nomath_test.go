package mdoc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yuin/goldmark/ast"
)

// countTeX 는 수식 렌더러(renderTeX)를 세는 것으로 바꿔 끼운다. 시험이 끝나면 되돌린다.
// 패키지 변수를 바꾸므로 병렬 시험(t.Parallel)에서 쓰지 않는다.
func countTeX(t *testing.T) *int {
	t.Helper()
	orig := renderTeX
	n := new(int)
	renderTeX = func(tex string, display bool) (string, error) {
		*n++
		return orig(tex, display)
	}
	t.Cleanup(func() { renderTeX = orig })
	return n
}

// Options.NoMath(itda-hyve#617): 수식이 어디에 있든 렌더러를 한 번도 부르지 않고, 원문을 이스케이프한 글자로 낸다.
// 같은 원문을 NoMath 없이 그리면 렌더러를 부른다 — 사례가 정말 수식 길을 지나는지 함께 본다.
func TestNoMathNeverRenders(t *testing.T) {
	calls := countTeX(t)
	for _, c := range []struct {
		name, src string
		want      []string
	}{
		{"글 안 수식", `반 $\frac12$ 이다`, []string{`반 <code>$\frac12$</code> 이다`}},
		{"글 안의 $$", `앞 $$x^2$$ 뒤`, []string{`앞 <code>$$x^2$$</code> 뒤`}},
		{"블록 수식", "$$\n\\frac12\n$$", []string{`<pre data-line="1"><code class="language-math">$$` + "\n\\frac12\n$$\n</code></pre>"}},
		{"블록 수식 한 줄", `$$\frac12$$`, []string{`<pre data-line="1"><code class="language-math">$$\frac12$$` + "\n</code></pre>"}},
		{"```math", "```math\n\\frac12\n```", []string{`<pre data-line="1"><code class="language-math">\frac12` + "\n</code></pre>"}},
		{"목록 안", "- $x$\n- $$\n  y\n  $$", []string{`<li><code>$x$</code></li>`, `<code class="language-math">$$` + "\ny\n$$"}},
		{"인용 안", "> $x$\n>\n> $$\n> y\n> $$", []string{`<code>$x$</code>`, `<code class="language-math">$$` + "\ny\n$$"}},
		{"표 칸 안", "| 식 |\n|---|\n| $\\lvert x \\| y\\rvert$ |", []string{`<td><code>$\lvert x | y\rvert$</code></td>`}},
		{"각주 안", "a[^1]\n\n[^1]: $x$ 와 $$y$$", []string{`<code>$x$</code> 와 <code>$$y$$</code>`}},
		{"제목·목차 안", "[TOC]\n\n## 공식 $E=mc^2$", []string{`공식 <code>$E=mc^2$</code></h2>`}},
		{"알림 안", "> [!NOTE]\n> $x$", []string{`<code>$x$</code>`}},
		{"링크·강조 안", "[$x$](#a) **$y$**", []string{`<a href="#a"><code>$x$</code></a>`, `<strong><code>$y$</code></strong>`}},
		{"노트 안", ":::notes\n$x$\n\n$$\ny\n$$\n\n```math\nz\n```\n:::", []string{`<div class="notes"`, `<code>$x$</code>`, `<code class="language-math">z`}},
		{"슬라이드 문서", "---\nslides: true\n---\n\n# 첫 장\n\n$a$\n\n# 둘째 장\n\n$$\nb\n$$\n\n:::notes\n$c$\n:::\n", []string{`<code>$a$</code>`, `<code class="language-math">$$` + "\nb\n$$", `<code>$c$</code>`}},
	} {
		t.Run(c.name, func(t *testing.T) {
			*calls = 0
			r := render(t, c.src, Options{NoMath: true})
			if *calls != 0 {
				t.Errorf("NoMath 인데 수식 렌더러를 %d 번 부름", *calls)
			}
			contains(t, r.HTML, c.want...)
			notContains(t, r.HTML, "<math", "math-error", "math-display")
			if hasWarn(r, WarnMath) {
				t.Errorf("NoMath 는 수식 경고를 내지 않음: %+v", r.Warnings)
			}
			assertSafe(t, r.HTML)

			*calls = 0
			render(t, c.src)
			if *calls == 0 {
				t.Errorf("NoMath 없이 그려도 렌더러를 부르지 않음 — 수식 길을 지나지 않는 사례")
			}
		})
	}
}

// NoMath 의 출력은 원문을 이스케이프한 글자다 — <·& 와 태그처럼 보이는 TeX 도 요소가 되지 않는다.
func TestNoMathEscapes(t *testing.T) {
	calls := countTeX(t)
	for _, c := range []struct{ src, want string }{
		{`$a<b & c$`, `<code>$a&lt;b &amp; c$</code>`},
		{`$x</code><script>alert(1)</script>$`, `<code>$x&lt;/code&gt;&lt;script&gt;alert(1)&lt;/script&gt;$</code>`},
		{"$$\na<b & \"c\"\n</code></pre><img src=x onerror=alert(1)>\n$$", "<code class=\"language-math\">$$\na&lt;b &amp; &quot;c&quot;\n&lt;/code&gt;&lt;/pre&gt;&lt;img src=x onerror=alert(1)&gt;\n$$\n</code>"},
		{"```math\na<b & c\n<script>x</script>\n```", "<code class=\"language-math\">a&lt;b &amp; c\n&lt;script&gt;x&lt;/script&gt;\n</code>"},
		{"| a |\n|---|\n| $a<b \\| c&d$ |", `<td><code>$a&lt;b | c&amp;d$</code></td>`},
	} {
		*calls = 0
		r := render(t, c.src, Options{NoMath: true})
		contains(t, r.HTML, c.want)
		notContains(t, r.HTML, "<script", "<img")
		assertSafe(t, r.HTML)
		if *calls != 0 {
			t.Errorf("%q: 수식 렌더러를 %d 번 부름", c.src, *calls)
		}
	}
}

// 닫는 $$ 가 없는 블록은 파싱 경고(수식을 그리지 않은 까닭이 아니라 원문 문제)를 그대로 내고, 원문을 같은 코드 꼴로 둔다.
func TestNoMathUnclosedBlock(t *testing.T) {
	calls := countTeX(t)
	r := render(t, "$$\nx\n\n뒤", Options{NoMath: true})
	contains(t, r.HTML, "<code class=\"language-math\">$$\nx\n</code></pre>", `<p data-line="4">뒤</p>`)
	if !hasWarn(r, WarnMath) || len(r.Warnings) != 1 {
		t.Errorf("닫는 $$ 경고 하나: %+v", r.Warnings)
	}
	if *calls != 0 {
		t.Errorf("수식 렌더러를 %d 번 부름", *calls)
	}
}

// 견본 문서 전부를 NoMath 로 그려도 렌더러를 부르지 않고, 문서 모델의 Doc.Math 는 어느 노드에서도 ok 가 거짓이다
// (수식 결과를 쥐지 않는다 — texmath 의 기억은 Render 안에서만 채워지므로 부르지 않으면 들어가지 않는다).
func TestNoMathGoldenAndDoc(t *testing.T) {
	calls := countTeX(t)
	files, err := filepath.Glob(filepath.Join("testdata", "*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("견본 문서: %v %v", files, err)
	}
	withMath := 0
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		*calls = 0
		d, err := Parse(src, Options{NoMath: true})
		if err != nil {
			t.Fatal(err)
		}
		if *calls != 0 {
			t.Errorf("%s: 수식 렌더러를 %d 번 부름", f, *calls)
		}
		assertSafe(t, d.Result.HTML)
		if d.Root() != nil {
			_ = ast.Walk(d.Root(), func(n ast.Node, entering bool) (ast.WalkStatus, error) {
				if entering {
					switch n.Kind() {
					case KindMathInline, KindMathBlock:
						withMath++
						fallthrough
					case ast.KindFencedCodeBlock:
						if _, ok := d.Math(n); ok {
							t.Errorf("%s: NoMath 인데 Doc.Math 가 결과를 냄", f)
						}
					}
				}
				return ast.WalkContinue, nil
			})
		}
	}
	if withMath == 0 {
		t.Error("견본 문서에 수식이 없음 — 시험이 수식 길을 지나지 않음")
	}
}

// 64KiB 최악 수식 문서(\def 사슬 수백 개·깊은 중첩·상한 넘는 수 등)를 NoMath 로 그리면 수식 비용이 없다.
// 판정은 렌더러 호출 수(0)로 한다 — 시간은 기록만(흔들린다).
func TestNoMathWorstCase(t *testing.T) {
	calls := countTeX(t)
	var b strings.Builder
	for i := 0; b.Len() < 64<<10; i++ {
		// 수식 하나 안에서 \def 사슬: \a0 → \a1 → … 로 확장이 늘어난다
		b.WriteString("$$\n")
		for j := range 40 {
			fmt.Fprintf(&b, `\def\m%c%d{\m%c%d\m%c%d}`, 'a'+rune(j%26), i, 'a'+rune((j+1)%26), i, 'a'+rune((j+1)%26), i)
		}
		b.WriteString("\n$$\n\n")
		fmt.Fprintf(&b, "글 $x_{%d}^{%s}$ 와 $%s$\n\n", i, strings.Repeat("{", 20)+"y"+strings.Repeat("}", 20), strings.Repeat(`\frac{1}{`, 10)+"2"+strings.Repeat("}", 10))
	}
	src := b.String()
	t0 := time.Now()
	r := render(t, src, Options{NoMath: true})
	took := time.Since(t0)
	t.Logf("NoMath: %d 바이트 렌더 %v, 렌더러 호출 %d", len(src), took.Round(time.Microsecond), *calls)
	if *calls != 0 {
		t.Errorf("수식 렌더러를 %d 번 부름", *calls)
	}
	if hasWarn(r, WarnMath) {
		t.Errorf("수식 경고: %d 개", len(r.Warnings))
	}
	notContains(t, r.HTML, "<math")
}
