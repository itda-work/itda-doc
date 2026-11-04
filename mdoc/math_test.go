package mdoc

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/itda-work/itda-doc/internal/timingtest"
	"github.com/itda-work/itda-doc/texmath"
)

// 1.10 수식 — 표기(Pandoc tex_math_dollars)
func TestMathSyntax(t *testing.T) {
	const frac = `<mfrac><mn>1</mn><mn>2</mn></mfrac>`
	runCases(t, []syntaxCase{
		{name: "글 안 수식", src: `반 $\frac12$ 이다`, want: []string{`<p data-line="1">반 <math>` + frac + `</math> 이다</p>`}},
		{name: "블록 수식 한 줄", src: `$$\frac12$$`, want: []string{`<div class="math-display" data-line="1"><math class="tml-display" display="block">` + frac + `</math></div>`}},
		{name: "블록 수식 여러 줄", src: "$$\n\\frac12\n$$\n\n뒤", want: []string{`<div class="math-display" data-line="1"><math class="tml-display" display="block">`, `<p data-line="5">뒤</p>`}},
		{name: "블록 수식 여는 줄에 내용", src: "$$ \\frac12\n+ 1 $$", want: []string{`<div class="math-display"`, frac + `<mo>+</mo><mn>1</mn>`}},
		{name: "블록 수식이 문단을 끊는다", src: "앞 글\n$$\nx\n$$", want: []string{`<p data-line="1">앞 글</p>`, `<div class="math-display" data-line="2">`}},
		{name: "글 안의 $$ 는 블록 꼴", src: `앞 $$\frac12$$ 뒤`, want: []string{`<p data-line="1">앞 <math class="tml-display" display="block">` + frac + `</math> 뒤</p>`}},
		{name: "줄 머리 $$x$$ 뒤에 글", src: `$$x$$ 는 글 안`, want: []string{`<p data-line="1"><math class="tml-display" display="block"><mi>x</mi></math> 는 글 안</p>`}},
		{name: "줄을 넘는 글 안 수식", src: "가 $a +\nb$ 나", want: []string{`<math><mrow><mi>a</mi><mo>+</mo><mi>b</mi></mrow></math>`}},
		{name: "```math 블록", src: "```math\n\\frac12\n```", want: []string{`<div class="math-display" data-line="1"><math class="tml-display" display="block">` + frac}, not: []string{"<pre"}},

		// 금액(Pandoc 규칙): 여는 $ 뒤·닫는 $ 앞이 공백이면, 닫는 $ 뒤가 숫자면 수식이 아니다
		{name: "금액 둘", src: "$30 와 $40 을 낸다", want: []string{`<p data-line="1">$30 와 $40 을 낸다</p>`}, not: []string{"<math"}},
		{name: "금액 범위", src: "$30–$40", want: []string{"$30–$40"}, not: []string{"<math"}},
		{name: "금액과 쉼표", src: "가격은 $5, $6, $7 이다", not: []string{"<math"}},
		{name: "닫는 $ 뒤 숫자", src: "$x$5", want: []string{"$x$5"}, not: []string{"<math"}},
		{name: "여는 $ 뒤 공백", src: "$ x$", want: []string{"$ x$"}, not: []string{"<math"}},
		{name: "닫는 $ 앞 공백", src: "$x $", want: []string{"$x $"}, not: []string{"<math"}},
		{name: "공백 뒤 $ 는 닫지 못함", src: "$x $ 와 $y $", want: []string{"$x $ 와 $y $"}, not: []string{"<math"}},
		{name: "실패한 $ 뒤의 $ 는 다시 연다", src: "$x $y$", want: []string{"$x <math><mi>y</mi></math>"}}, // Pandoc 과 같다
		{name: "닫히지 않은 $ 는 글자(경고 없음)", src: "값은 $x 이다", want: []string{"값은 $x 이다"}, not: []string{"<math"}},
		{name: "빈 $$", src: "$$ $$ 와 $  $", not: []string{"<math"}},
		{name: "$$$$ 는 Pandoc 처럼 $ + $$…$$", src: "$$$$ 와 $$ $$", want: []string{`$<span class="math-error">$$$ 와 $$</span> $$`}, warn: WarnMath},
		{name: "실패한 $ 다음 자리에서 다시 연다", src: "$x$$y$", want: []string{`<math><mi>x</mi></math><math><mi>y</mi></math>`}},
		{name: "$$ 가 실패하면 둘째 $ 로", src: "앞 $$a+1$", want: []string{`앞 $<math><mrow><mi>a</mi><mo>+</mo><mn>1</mn></mrow></math>`}},
		{name: "글자 안의 $", src: "a$b$c", want: []string{`a<math><mi>b</mi></math>c`}},

		// 이스케이프·코드
		{name: "\\$ 는 글자", src: `\$x\$ 는 수식이 아님`, want: []string{"$x$ 는 수식이 아님"}, not: []string{"<math"}},
		{name: "수식 안의 \\$", src: `$\$5$`, want: []string{`<mi>$</mi><mn>5</mn>`}},
		{name: "수식 안의 \\$ 는 닫지 않음", src: `$a\$ b$`, want: []string{`<mi>a</mi><mi>$</mi><mi>b</mi>`}},
		{name: "\\text 안의 $…$", src: `$\text{가격 $p$ 원}$ 끝`, want: []string{`<mtext>가격`, `<mi>p</mi>`, ` 끝</p>`}},
		{name: "코드 안", src: "`$x$` 와 `$$y$$`", want: []string{"<code>$x$</code>", "<code>$$y$$</code>"}, not: []string{"<math"}},
		{name: "코드 블록 안", src: "```\n$x$\n$$\ny\n$$\n```", want: []string{"$x$\n$$\ny\n$$"}, not: []string{"<math"}},
		{name: "들여쓴 코드 블록 안", src: "    $x$", want: []string{"<pre", "$x$"}, not: []string{"<math"}},
		{name: "수식이 먼저 열면 코드가 아님", src: "$a`b$` c", want: []string{"<math"}, not: []string{"<code>"}},
		{name: "수식 안의 _·* 는 TeX", src: "$a_1 *b* c_2$ 와 *강조*", want: []string{"<msub><mi>a</mi><mn>1</mn></msub><mo>∗</mo>", "<em>강조</em>"}},
		{name: "수식 안의 HTML 은 TeX 글자", src: `$a<b$`, want: []string{`<mi>a</mi><mo>&lt;</mo><mi>b</mi>`}, not: []string{"<b"}},
		{name: "링크 안 수식", src: "[$x$ 보기](#a)", want: []string{`<a href="#a"><math><mi>x</mi></math> 보기</a>`}},
		{name: "강조 안 수식", src: "**$x$ 굵게**", want: []string{`<strong><math><mi>x</mi></math> 굵게</strong>`}},

		// 표 안: \| 는 칸을 나누지 않는 | 다(GFM)
		{name: "표 안 수식", src: "| 식 | 값 |\n|---|--:|\n| $\\frac12$ | $\\lvert x \\| y\\rvert$ |", want: []string{"<td>" + `<math>` + frac + `</math></td>`, `<mi>|</mi>`}},
		{name: "표 안 \\Vert", src: "| a |\n|---|\n| $\\Vert x \\Vert$ |", want: []string{"<td><math>", "‖"}},

		// 블록 수식 안의 블록 문법은 TeX 다
		{name: "블록 수식 안의 - 와 #", src: "$$\n- 1\n+ 2\n> 3\n1. x\n=\n$$", want: []string{`<div class="math-display"`}, not: []string{"<ul", "<ol", "<h1", "<blockquote"}},
		{name: "인용 안 블록 수식", src: "> $$\n> x\n> $$", want: []string{`<blockquote data-line="1"><div class="math-display"><math`}},
		{name: "목록 안 블록 수식", src: "- $$\n  x\n  $$", want: []string{`<li>`, `<div class="math-display"><math`}},
		{name: "제목 안 수식", src: "## 공식 $E=mc^2$", want: []string{`<h2 id="공식-emc2" data-line="1">공식 <math>`}},
		{name: "알림 안 수식", src: "> [!NOTE]\n> $x$", want: []string{`markdown-alert-note`, `<math><mi>x</mi></math>`}},
		{name: "각주 안 수식", src: "a[^1]\n\n[^1]: $x$", want: []string{`<math><mi>x</mi></math>`}},
	})
}

// 그리지 못한 수식은 원문 글자 그대로(구분자 포함) 두고, 원문 줄 번호와 까닭을 경고한다.
func TestMathFallback(t *testing.T) {
	cases := []struct {
		name, src, html, msg string
		line                 int
	}{
		{"미지원 명령", "가\n\n나 $\\foo{x}$ 다", `<span class="math-error">$\foo{x}$</span>`, `지원하지 않는 명령 \foo`, 3},
		{"색", `$\color{red}{x}$`, `<span class="math-error">$\color{red}{x}$</span>`, `쓰지 않는 명령 \color — 색은 테마가 정한다`, 1},
		{"크기", `$\Huge x$`, `<span class="math-error">$\Huge x$</span>`, `쓰지 않는 명령 \Huge`, 1},
		{"주소", `$\href{https://a.example}{x}$`, `<span class="math-error">$\href{https://a.example}{x}$</span>`, `쓰지 않는 명령 \href`, 1},
		{"HTML", `$\htmlClass{a}{x}$`, `<span class="math-error">`, `쓰지 않는 명령 \htmlClass`, 1},
		{"식 번호", "$$\nx \\tag{1}\n$$", `<p class="math-error" data-line="1">$$
x \tag{1}
$$</p>`, `쓰지 않는 명령 \tag`, 1},
		{"문법 오류", "1\n2\n$\\frac{1}{$", `<span class="math-error">$\frac{1}{$</span>`, "수식 문법 오류", 3},
		{"닫히지 않은 $$", "$$\nx + y\n\n뒤", `<p class="math-error" data-line="1">$$
x + y</p>`, "닫는 $$ 가 없어", 1},
		{"문서 끝까지 닫히지 않은 $$", "글\n\n$$\nx", `<p class="math-error" data-line="3">$$
x</p>`, "닫는 $$ 가 없어", 3},
		{"블록 수식에서만 쓰는 환경", `$\begin{align}a&=b\end{align}$`, `<span class="math-error">`, "블록 수식에서만", 1},
		{"```math 의 오류는 코드 블록", "```math\n\\foo\n```", `<pre data-line="1"><code class="language-math">\foo`, `지원하지 않는 명령 \foo`, 1},
		{"표 안", "| a |\n|---|\n| $\\foo$ |", `<td><span class="math-error">$\foo$</span></td>`, `\foo`, 3},
		{"원문 탈출", `$\foo "><script>$`, `<span class="math-error">$\foo &quot;&gt;&lt;script&gt;$</span>`, `\foo`, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := render(t, c.src)
			contains(t, r.HTML, c.html)
			notContains(t, r.HTML, "<math")
			if len(r.Warnings) != 1 || r.Warnings[0].Code != WarnMath || r.Warnings[0].Line != c.line || !strings.Contains(r.Warnings[0].Message, c.msg) {
				t.Errorf("경고 하나(줄 %d, %q) 기대: %+v", c.line, c.msg, r.Warnings)
			}
			assertSafe(t, r.HTML)
		})
	}
}

// 문서 하나의 수식 수 상한. 넘는 수식은 원문이고 경고는 한 번이다.
func TestMathPerDocLimit(t *testing.T) {
	var b strings.Builder
	for i := range MaxMathPerDoc + 5 {
		fmt.Fprintf(&b, "$x_{%d}$\n", i)
	}
	r := render(t, b.String())
	if n := strings.Count(r.HTML, "<math"); n != MaxMathPerDoc {
		t.Errorf("그린 수식 %d, 상한 %d", n, MaxMathPerDoc)
	}
	if n := strings.Count(r.HTML, `<span class="math-error">`); n != 5 {
		t.Errorf("원문으로 둔 수식 %d", n)
	}
	if len(r.Warnings) != 1 || r.Warnings[0].Code != WarnMath || r.Warnings[0].Line != MaxMathPerDoc+1 {
		t.Errorf("경고 한 번(줄 %d): %+v", MaxMathPerDoc+1, r.Warnings)
	}
}

// 매크로는 수식 하나 안에서만 산다 — 같은 문서의 다음 수식에도, 다른 문서에도 남지 않는다.
func TestMathMacroIsolation(t *testing.T) {
	a := render(t, `$\gdef\myR{\mathbb{R}}\def\myS{x}\myR \myS$ 그리고 $\myR$`)
	if strings.Count(a.HTML, "<math") != 1 || len(a.Warnings) != 1 || !strings.Contains(a.Warnings[0].Message, `\myR`) {
		t.Errorf("같은 문서의 다음 수식에 매크로가 남음: %s %+v", a.HTML, a.Warnings)
	}
	b := render(t, `$\myS$`)
	if strings.Contains(b.HTML, "<math") || len(b.Warnings) != 1 {
		t.Errorf("다른 문서에 매크로가 남음: %s %+v", b.HTML, b.Warnings)
	}
}

// genMathDoc 은 수식이 많은 긴 문서다: 절마다 글 안 수식 6개 + 블록 수식(행렬·정렬·경우) 3개 + 수식이 든 표(2개).
// seed 를 모든 수식 앞에 첨자로 넣는다 — seed 가 다르면 수식 원문이 모두 달라 기억(cache)을 지나지 않는다.
func genMathDoc(sections, seed int) []byte {
	var b strings.Builder
	m := func(tex string, args ...any) string { return fmt.Sprintf("{}^{%d}", seed) + fmt.Sprintf(tex, args...) }
	for s := range sections {
		fmt.Fprintf(&b, "## %d. 수식 절\n\n", s+1)
		fmt.Fprintf(&b, "매출 $%s$ 와 이익률 $%s$, 성장률 $%s$ 이다. 합 $%s$ 과 근 $%s$, 극한 $%s$.\n\n",
			m(`R_{%d} = p_{%d} \times q_{%d}`, s, s, s), m(`\frac{\text{영업이익}}{\text{매출}} \times 100\%%`), m(`g = \frac{R_t - R_{t-1}}{R_{t-1}}`),
			m(`\sum_{i=1}^{%d} i = \frac{n(n+1)}{2}`, s+3), m(`x = \frac{-b \pm \sqrt{b^2-4ac}}{2a}`), m(`\lim_{n\to\infty} (1+\frac1n)^n = e`))
		fmt.Fprintf(&b, "$$\n%s\n$$\n\n", m(`\begin{pmatrix} %d & 2 \\ 3 & 4 \end{pmatrix} \begin{pmatrix} x \\ y \end{pmatrix} = \begin{pmatrix} 5 \\ 6 \end{pmatrix}`, s))
		fmt.Fprintf(&b, "$$\n%s\n$$\n\n", m(`\begin{aligned} f(x) &= (x+%d)^2 \\ &= x^2 + %dx + %d \end{aligned}`, s, 2*s, s*s))
		fmt.Fprintf(&b, "$$\n%s\n$$\n\n", m(`|x| = \begin{cases} x & x \geq 0 \\ -x & \text{그 외} \end{cases}`))
		fmt.Fprintf(&b, "| 지표 | 식 |\n|:--|:--|\n| 이익률 | $%s$ |\n| 증가율 | $%s$ |\n\n", m(`\frac{P}{R}`), m(`\Delta R / R`))
	}
	return []byte(b.String())
}

// 렌더 결정성: 수식이 든 긴 문서를 여러 번 렌더해도(기억을 지나든 아니든) 바이트까지 같다.
func TestMathDeterministic(t *testing.T) {
	src := genMathDoc(30, 0)
	first := render(t, string(src)).HTML
	if n := strings.Count(first, "<math"); n != 30*11 {
		t.Fatalf("수식 %d 개 기대 %d", n, 30*11)
	}
	for range 5 {
		if got := render(t, string(src)).HTML; got != first {
			t.Fatal("다시 렌더한 결과가 다름")
		}
	}
}

// 긴 수식 문서의 렌더 시간(p50·p95). 처음(엔진을 올리고 모든 수식을 변환)과 편집 중(기억) 둘을 잰다.
// 보고만 하고 실패하지 않는다.
//
//	MDOC_TIMING=1 go test ./mdoc -run TestMathRenderTime -v
func TestMathRenderTime(t *testing.T) {
	if os.Getenv("MDOC_TIMING") == "" {
		t.Skip("MDOC_TIMING=1 로 잰다")
	}
	measure := func(name string, src func(i int) []byte, n int) {
		var ds []time.Duration
		for i := range n {
			s := src(i)
			t0 := time.Now()
			if _, err := Render(s, Options{}); err != nil {
				t.Fatal(err)
			}
			ds = append(ds, time.Since(t0))
		}
		sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
		t.Logf("%s: n=%d p50=%v p95=%v max=%v", name, n, ds[n/2].Round(time.Microsecond), ds[n*95/100].Round(time.Microsecond), ds[n-1].Round(time.Microsecond))
	}
	doc := genMathDoc(30, 0) // 수식 330 개
	t.Logf("문서: %d 바이트, 수식 %d 개", len(doc), 30*11)
	measure("처음 렌더(수식 330 개 모두 새로 변환)", func(i int) []byte { return genMathDoc(30, 1000+i) }, 20)
	measure("편집 중 다시 렌더(수식 하나 바뀜)", func(i int) []byte { return append(append([]byte{}, doc...), fmt.Sprintf("\n끝 $y_{%d}$\n", i)...) }, 50)
	measure("수식 없는 같은 크기 문서", func(i int) []byte {
		return []byte(regexp.MustCompile(`\$\$?[^$]+\$\$?`).ReplaceAllString(string(genMathDoc(30, i)), "x"))
	}, 50)
}

// 리뷰 R1 P1: 닫히지 않는 $ 가 많은 문단도 선형 시간이다. 고치기 전에는 `$\text{` 반복 448KB 가 8.8초(이차)였다.
// 크기를 두 배씩 늘려 시간이 거의 두 배로만 늘고(이차면 네 배), 가장 큰 것도 짧으며, 원문 글자가 그대로 남는지 본다.
func TestMathScanTime(t *testing.T) {
	patterns := map[string]string{
		"닫히지 않은 \\text{": `$\text{`,
		"\\text 안의 $":    `$\text{$ `,     // \text 가 닫히지 않아 그 안의 $ 로 닫지 못함
		"닫히지 않은 $$":      `$$\text{a $$ `, // 블록 꼴도 같은 스캐너
		"공백 뒤 $":         `$a $ `,         // 실패가 빠른 꼴(대조)
		"금액":             `$30 와 `,
	}
	for name, unit := range patterns {
		t.Run(name, func(t *testing.T) {
			gen := func(size int) string { return strings.Repeat(unit, size/len(unit)) }
			sizes := []int{112_000, 224_000, 448_000}
			for _, size := range sizes {
				src := gen(size)
				r := render(t, src)
				if len(r.Warnings) > 1 {
					t.Errorf("%d 바이트: 경고 %d 개(한 번이어야)", len(src), len(r.Warnings))
				}
				if !strings.Contains(r.HTML, strings.Repeat(htmlEsc(unit), 10)) {
					t.Errorf("%d 바이트: 원문 글자가 남지 않음", len(src))
				}
			}
			// 크기 셋이라 쌍마다 판정한다(timingtest judge): 크기가 두 배가 될 때마다 비용(CPU 시간)이 세 배(+10ms)를 넘지 않는다 — 이차 시간이면 네 배다
			timingtest.AssertDoubling(t, name, sizes, renderWork(t, gen),
				timingtest.Doubling{Reps: 1, Slack: 10 * time.Millisecond, Limit: 500 * time.Millisecond})
		})
	}
	// 예산은 보통 문서의 수식을 막지 않는다: 금액 $ 가 많은 긴 문단의 끝 수식도 그린다
	var b strings.Builder
	for i := range 300 {
		fmt.Fprintf(&b, "항목 %d 은 $%d 와 ", i, i+10)
	}
	b.WriteString("끝으로 $x^2$ 이다.")
	r := render(t, b.String())
	if strings.Count(r.HTML, "<math") != 1 || len(r.Warnings) > 0 {
		t.Errorf("금액 300개 문단 끝의 수식: math %d, %+v", strings.Count(r.HTML, "<math"), r.Warnings)
	}
}

func htmlEsc(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

// 리뷰 R1 P2: 그리지 못한 블록 수식은 내용이 없어도 구분자까지 원문 그대로 남는다(인용·목록 표지는 빼고).
func TestMathBlockRaw(t *testing.T) {
	for _, c := range []struct {
		src, html, msg string
		line           int
	}{
		{"$$", `<p class="math-error" data-line="1">$$</p>`, "닫는 $$ 가 없어", 1},
		{"앞\n\n$$\n\n뒤", `<p class="math-error" data-line="3">$$</p>`, "닫는 $$ 가 없어", 3},
		{"$$\n$$", "<p class=\"math-error\" data-line=\"1\">$$\n$$</p>", "빈 수식", 1},
		{"$$  \n  $$", "<p class=\"math-error\" data-line=\"1\">$$  \n  $$</p>", "빈 수식", 1},
		{"> $$\n> \\foo\n> $$", "<p class=\"math-error\">$$\n\\foo\n$$</p>", `\foo`, 1},
		{"- 가\n\n  $$\n  \\foo\n  $$", "<p class=\"math-error\">$$\n\\foo\n$$</p>", `\foo`, 3},
	} {
		r := render(t, c.src)
		contains(t, r.HTML, c.html)
		if len(r.Warnings) != 1 || r.Warnings[0].Line != c.line || !strings.Contains(r.Warnings[0].Message, c.msg) {
			t.Errorf("%q: 경고(줄 %d, %q) 기대: %+v", c.src, c.line, c.msg, r.Warnings)
		}
	}
}

// 리뷰 R1 P2: \text{…} 안의 $ 와 줄 넘김은 $…$·한 줄 $$…$$·여러 줄 블록에서 같은 규칙이다.
// 닫는 $$ 앞의 역슬래시는 홀짝으로 본다(짝수면 서로 이스케이프 — 닫는다).
func TestMathDelimiterRules(t *testing.T) {
	for _, tex := range []string{"\\text{한\n글}", `\text{a $x$ b}`, `\text{a $$ b}`, `\text{가격 \$5}`, `\text{a {b} $c$ d}`} {
		forms := map[string]string{
			"글 안":     "앞 $" + tex + "$ 뒤",
			"한 줄 $$":  "앞 $$" + tex + "$$ 뒤",
			"여러 줄 블록": "$$\n" + tex + "\n$$\n\n뒤",
		}
		if strings.Contains(tex, "$$") {
			delete(forms, "글 안") // 글 안 수식의 $$ 는 \text 안이라도 Temml 이 수식 전환으로 읽는다(그 수식의 문제 — 구분자 규칙과 무관)
		}
		for form, src := range forms {
			r := render(t, src)
			if strings.Count(r.HTML, "<math") != 1 || len(r.Warnings) > 0 || strings.Contains(r.HTML, "math-error") {
				t.Errorf("%s %q: 수식 하나를 그려야 함\n%s\n%+v", form, tex, r.HTML, r.Warnings)
			}
			if !strings.Contains(r.HTML, "뒤") {
				t.Errorf("%s %q: 뒤의 글이 사라짐", form, tex)
			}
		}
	}
	// 짝수 역슬래시: 닫는 $$ — 다음 줄은 따로 문단
	r := render(t, "$$\na\\\\$$\n뒤")
	contains(t, r.HTML, `<div class="math-display" data-line="1">`, `<p data-line="3">뒤</p>`)
	if len(r.Warnings) > 0 {
		t.Errorf("짝수 역슬래시: %+v", r.Warnings)
	}
	// 홀수 역슬래시: \$ 뒤의 $ 하나는 닫지 않는다 — 다음 줄의 $$ 가 닫고, 그 뒤 문장은 따로
	r = render(t, "$$\na\\$\n$$\n뒤")
	contains(t, r.HTML, `<div class="math-display" data-line="1">`, `<mi>$</mi>`, `<p data-line="4">뒤</p>`)
	if len(r.Warnings) > 0 {
		t.Errorf("홀수 역슬래시: %+v", r.Warnings)
	}
	r = render(t, "$$\na\\$$\n뒤")
	if strings.Contains(r.HTML, "<math") || !hasWarn(r, WarnMath) {
		t.Errorf("이스케이프한 $ 뒤의 $ 는 블록을 닫지 않는다(닫히지 않음 경고): %s %+v", r.HTML, r.Warnings)
	}
	// 글 안: \\$ 의 $ 는 닫는다(그 수식 a\\ 는 글 안에서 Temml 오류 — 원문 그대로), \$ 의 $ 는 닫지 않는다
	r = render(t, `$a\\$ 와 $b\$c$`)
	contains(t, r.HTML, `<span class="math-error">$a\\$</span> 와 <math><mrow><mi>b</mi><mi>$</mi><mi>c</mi></mrow></math>`)
}

// 리뷰 R2: TeX 주석(% 부터 줄 끝) 안의 명령·괄호는 스캐너 상태를 바꾸지 않는다 — 다음 줄의 닫는 $$ 를 놓치지 않는다.
// 주석 안의 $ 는 여전히 구분자다(Pandoc 은 TeX 주석을 모른다). \% 는 글자 %.
// 주석이 같은 줄 뒤의 글을 지우는 예(pct)는 그 수식에 주석 경고 하나만 낸다(itda-hyve#180) — 다른 경고는 없다.
func TestMathComment(t *testing.T) {
	for _, c := range []struct {
		src, want, not string
		pct            bool
	}{
		{"$$\nx % \\text{\n$$\n뒤", `<p data-line="4">뒤</p>`, "math-error", true},
		{"$$\nx % {\n$$\n뒤", `<p data-line="4">뒤</p>`, "math-error", true},
		{"$$\n\\text{a % }\nb}\n$$\n뒤", `<p data-line="5">뒤</p>`, "math-error", true},
		{"$$\nx % \\text{ $$\n뒤", `<p data-line="3">뒤</p>`, "math-error", true}, // 주석 안의 $$ 로 닫는다
		{"앞 $x % \\text{\n+y$ 뒤", `<math><mrow><mi>x</mi><mo>+</mo><mi>y</mi></mrow></math> 뒤`, "math-error", true},
		{"앞 $x % } { \\text{$ 뒤", `앞 <math><mi>x</mi></math> 뒤`, "math-error", true},
		{"$$\n50\\% \\text{a $$ b}\n$$\n뒤", `<p data-line="4">뒤</p>`, "math-error", false}, // \% 는 주석이 아니다 — \text 가 $$ 를 가린다
		{"$50\\%$ 와 $x$", `<mn>50</mn><mi>%</mi>`, "math-error", false},
	} {
		r := render(t, c.src)
		want := 0
		if c.pct {
			want = 1
		}
		if !strings.Contains(r.HTML, "<math") || !strings.Contains(r.HTML, c.want) || strings.Contains(r.HTML, c.not) || len(r.Warnings) != want ||
			(want == 1 && (r.Warnings[0].Code != WarnMath || !strings.Contains(r.Warnings[0].Message, "TeX 주석"))) {
			t.Errorf("%q\n%s\n%+v", c.src, r.HTML, r.Warnings)
		}
	}
}

// itda-hyve#180: 이스케이프하지 않은 % 가 보일 글을 지우면(숫자 뒤의 %, 같은 줄 앞뒤에 글이 있는 %) 수식은 그대로 그리고 math 경고를 낸다.
// 줄 머리의 주석 줄·줄 끝에 홀로 남은 %·\% 는 경고하지 않는다. 경고 줄은 그 수식이 시작한 줄이다.
func TestMathPercentWarning(t *testing.T) {
	for _, c := range []struct {
		src  string
		line int
		lost string // 경고 메시지에 실린 사라진 글. "" 이면 경고 없음
	}{
		{"이익률 $50%$ 이다", 1, "「%」"},
		{"앞\n\n$x = 3%$ 와 $y$", 3, "「%」"},
		{"$50 %$", 0, ""}, // 숫자 바로 뒤가 아니고 뒤에 글도 없다
		{"$a % b$", 1, "「% b」"},
		{"$10\\text{원} + 5%$", 1, "「%」"},
		{"$50\\%$", 0, ""},
		{"```math\na \\\\% b\n```", 1, "「% b」"}, // \\ 뒤의 % 는 주석이다
		{"$$\n% 설명 줄\nx^2\n$$", 0, ""},
		{"$$\nx^2 %\n$$", 0, ""},
		{"$$\nx^2 % 제곱\n$$", 1, "「% 제곱」"},
		{"$$\na \\\\\nb = 5%\n$$", 1, "「%」"},
		{"```math\n0.5% + x\n```", 1, "「% + x」"},
		{"$a % 아주아주아주아주아주아주아주아주 긴 주석$", 1, "「% 아주아주아주아주아주아주아주아주 긴…」"},
	} {
		r := render(t, c.src)
		if !strings.Contains(r.HTML, "<math") {
			t.Errorf("%q: 수식을 그리지 않음\n%s", c.src, r.HTML)
		}
		if c.lost == "" {
			if len(r.Warnings) > 0 {
				t.Errorf("%q: 경고가 없어야\n%+v", c.src, r.Warnings)
			}
			continue
		}
		if len(r.Warnings) != 1 || r.Warnings[0].Code != WarnMath || r.Warnings[0].Line != c.line ||
			!strings.Contains(r.Warnings[0].Message, c.lost) || !strings.Contains(r.Warnings[0].Message, `\%`) {
			t.Errorf("%q: 경고 = %+v (줄 %d, %s 기대)", c.src, r.Warnings, c.line, c.lost)
		}
	}
}

// 리뷰 R2: 닫힌 긴 글 안 수식은 길이에 상관없이 수식 노드다 — 원문 그대로(안의 *·_ 가 강조가 되지 않고) 길이 경고.
func TestMathLongInline(t *testing.T) {
	for _, n := range []int{4001, 4017, 4020, 10_000, 100_000} {
		for _, form := range []string{"$", "$$"} {
			body := strings.Repeat("x", n-len(`\text{*강조*}`)) + `\text{*강조*}`
			src := "앞 " + form + body + form + " 뒤 *기울임*"
			r := render(t, src)
			want := `<span class="math-error">` + form + body + form + `</span> 뒤 <em>기울임</em>`
			if !strings.Contains(r.HTML, want) || strings.Contains(r.HTML, "<em>강조</em>") {
				t.Errorf("%s %d 바이트: 원문 보존이 아님\n%.300s", form, n, r.HTML[max(0, len(r.HTML)-300):])
			}
			if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0].Message, "수식이 너무 김") {
				t.Errorf("%s %d 바이트: 길이 경고 하나 기대: %+v", form, n, r.Warnings)
			}
		}
	}
	if r := render(t, "$"+strings.Repeat("x", texmath.MaxInputBytes)+"$"); strings.Count(r.HTML, "<math") != 1 || len(r.Warnings) > 0 {
		t.Errorf("상한 길이의 수식: %+v", r.Warnings)
	}
}

// itda-hyve#591: 열린 \text{…} 안의 주석 % 가 닫는 } 를 가려 수식이 닫히지 않으면, $ 를 글자로 두되(Pandoc 규칙 그대로) 그 % 의 줄에서
// math 경고 하나를 낸다. 같은 % 를 여러 시도가 지나도 한 번이다. 닫힌 수식(R3 꼴)은 itda-hyve#180 경고만, 금액·\% 는 경고 없음.
// $$ 블록이 같은 까닭으로 닫히지 않으면 「닫는 $$ 가 없어」 경고 하나에 까닭을 싣는다.
func TestMathTextPercentUnclosed(t *testing.T) {
	type want struct {
		line  int
		has   []string // 메시지에 든 글
		lacks []string // 메시지에 없어야 할 글
	}
	for _, c := range []struct {
		src   string
		math  bool // 수식이 그려지는가
		warns []want
	}{
		{"앞 $\\text{증가 10%}$ 뒤", false, []want{{1, []string{"「%}$ 뒤」", "닫는 }", `\%`}, nil}}},
		{"첫 줄\n$\\text{10%}$ 와 $\\text{20%}$", false, []want{{2, []string{"「%}$ 와 $\\text{20%}$」"}, nil}, {2, []string{"「%}$」"}, nil}}},
		{"$x\\text{a $\\text{10%}$", false, []want{{1, []string{"「%}$」"}, nil}}},                       // 두 시도가 같은 % 를 지난다 — 한 번
		{"$\\text{a % $b\nc}$", true, []want{{1, []string{"「% $b」(% 부터 줄 끝)는 TeX 주석이라 그리지 않음"}, nil}}}, // 닫힘 — itda-hyve#180
		{"$\\text{10\\%}$", true, nil},
		{"계약금 $30 와 잔금 $40", false, nil},
		{"$50% 증가", false, nil}, // \text 밖의 주석 — 닫히지 않은 $ 는 금액 규칙대로 경고 없음
		{"$$\n\\text{증가 10%}\n$$\n\n뒤", false, []want{{1, []string{"닫는 $$ 가 없어", "「%}」", `\%`}, nil}}},
		{"$$\n\\text{증가 10\\%}\n$$", true, nil},
		{"$\\text{a % b}$", false, []want{{1, []string{"「% b}$」"}, nil}}}, // 명세의 깨지는 입력
		{"$\\text{a % b\nc} $", false, nil},                               // \text 는 뒤 줄에서 닫혔다 — 실패 까닭(공백 뒤 $)은 % 가 아니다
		{"$$\n\\text{a % b\nc}\n\n뒤", false, []want{{1, []string{"닫는 $$ 가 없어"}, []string{"주석"}}}},
		// 앞 줄에서 닫힌 \text 의 % 가 아니라 지금 열린 \text 의 % 를 탓한다
		{"$\\text{a % b\nc} \\text{d % e}$", false, []want{{2, []string{"「% e}$」"}, []string{"% b"}}}},
		{"$$\n\\text{a % b\nc} \\text{d % e}\n\n뒤", false, []want{{1, []string{"닫는 $$ 가 없어", "「% e}」"}, []string{"% b"}}}},
		// 같은 \text 안의 첫 % 가 아니라 실제로 닫는 } 를 가린 % 를 탓한다 — 앞 % 의 줄에는 } 가 없다
		{"$\\text{a % b\nc % d}$", false, []want{{2, []string{"「% d}$」"}, []string{"% b"}}}},
		{"$$\n\\text{a % b\nc % d}\n\n뒤", false, []want{{1, []string{"닫는 $$ 가 없어", "「% d}」", "가림, 퍼센트 기호면 \\%"}, []string{"% b"}}}},
		// 주석 안의 { } 가 짝이 맞으면 \text 의 } 를 가리지 않았다 — 닫힌 꼴은 그려지고, 닫는 } 가 빠진 꼴은 까닭이 % 가 아니다
		{"$\\text{a % {b}\nc}$", true, []want{{1, []string{"「% {b}」"}, []string{"닫는 }"}}}},
		{"$\\text{a % {b}\nc$", false, nil},
		// 중첩 — 안쪽 { 안의 % 줄에 } 가 없다
		{"$\\text{a {b % c\nd} e}$", true, []want{{1, []string{"「% c」"}, []string{"닫는 }"}}}},
		{"$\\text{a {b % c\nd} e$", false, nil},
		// 깊이 2 에서 짝 없는 } 를 가린 % 도 탓한다 — 명세 「짝 없는 } 를 처음 가린 %」
		{"$\\text{a {b % c}\nd} e$", false, []want{{1, []string{"「% c}」"}, nil}}},
	} {
		r := render(t, c.src)
		if got := strings.Contains(r.HTML, "<math"); got != c.math {
			t.Errorf("%q: 수식 %v (기대 %v)\n%s", c.src, got, c.math, r.HTML)
		}
		if len(r.Warnings) != len(c.warns) {
			t.Errorf("%q: 경고 %d (기대 %d)\n%+v", c.src, len(r.Warnings), len(c.warns), r.Warnings)
			continue
		}
		for i, w := range r.Warnings {
			if w.Code != WarnMath || w.Line != c.warns[i].line {
				t.Errorf("%q: 경고 %d = %+v (기대 math·줄 %d)", c.src, i, w, c.warns[i].line)
			}
			for _, s := range c.warns[i].has {
				if !strings.Contains(w.Message, s) {
					t.Errorf("%q: 경고 %d 에 %q 없음: %s", c.src, i, s, w.Message)
				}
			}
			for _, s := range c.warns[i].lacks {
				if strings.Contains(w.Message, s) {
					t.Errorf("%q: 경고 %d 에 %q 가 있음: %s", c.src, i, s, w.Message)
				}
			}
		}
	}
}
