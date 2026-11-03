package mdoc

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/itda-work/itda-doc/internal/timingtest"
	"github.com/yuin/goldmark/ast"
	"golang.org/x/net/html"
)

// 수식 구분자 찾기의 리뷰 재현(R1~R3)을 한곳에 모은다. 각 사례는 그 라운드의 리뷰가 적은 입력 그대로이고,
// want 는 결과 HTML 에 있어야 할 조각, not 은 없어야 할 조각, warns 는 경고 수다. R2·R3 의 주석 사례는 주석이 같은 줄 뒤의 글을
// 지우므로 주석 경고(percentLoss) 하나를 낸다(itda-hyve#180) — 구분자 판정(want·not)은 그대로다.
func TestMathReviewRepros(t *testing.T) {
	cases := []struct {
		round, src string
		want, not  []string
		warns      int
	}{
		// R1 P2: 빈·미완성 블록의 원문
		{"R1", "$$", []string{`<p class="math-error" data-line="1">$$</p>`}, nil, 1},
		{"R1", "앞\n\n$$\n\n뒤", []string{`<p class="math-error" data-line="3">$$</p>`, `<p data-line="5">뒤</p>`}, nil, 1},
		{"R1", "$$\n$$", []string{"<p class=\"math-error\" data-line=\"1\">$$\n$$</p>"}, nil, 1},
		// R1 P2: \text 보호(여러 줄·블록 꼴)
		{"R1", "가 $\\text{한\n글}$ 나", []string{"<mtext>한\u00a0글</mtext>"}, []string{"math-error"}, 0},
		{"R1", `$$\text{a $$ b}$$`, []string{"<mtext>a\u00a0\u00a0b</mtext>"}, []string{"math-error"}, 0},
		{"R1", "$$\n\\text{a $$ b}\n$$", []string{"<mtext>a\u00a0\u00a0b</mtext>"}, []string{"math-error"}, 0},
		// R1 P2: 짝수 역슬래시 뒤 닫는 $$
		{"R1", "$$\na\\\\$$\n뒤", []string{`<div class="math-display" data-line="1">`, `<p data-line="3">뒤</p>`}, nil, 0},
		// R2 P2: 주석 안 \text{ 가 닫는 $$ 를 가림
		{"R2", "$$\nx % \\text{\n$$\n뒤", []string{`<mi>x</mi>`, `<p data-line="4">뒤</p>`}, []string{"math-error"}, 1},
		{"R2", "$$\n\\text{a % }\nb}\n$$\n뒤", []string{"<mtext>a\u00a0b</mtext>", `<p data-line="5">뒤</p>`}, []string{"math-error"}, 1},
		{"R2", "앞 $x % \\text{\n+y$ 뒤", []string{"<mi>x</mi><mo>+</mo><mi>y</mi>"}, []string{"math-error"}, 1},
		// R2 P2: 닫힌 긴 수식은 원문 + 길이 경고(안의 *·_ 는 강조가 아님)
		{"R2", "$" + strings.Repeat("x", 4020) + `\text{*강조*}$`, []string{`\text{*강조*}$</span>`}, []string{"<em>"}, 1},
		// R3 P2: 열린 \text 안의 주석 속 $ 는 글자
		{"R3", "$\\text{a % $literal\nb}$", []string{"<math><mtext>a\u00a0b</mtext></math>"}, []string{"math-error"}, 1},
		{"R3", "$$\n\\text{a % $$\nb}\n$$\n뒤", []string{"<mtext>a\u00a0b</mtext>", `<p data-line="5">뒤</p>`}, []string{"math-error"}, 1},
		{"R3", "$\\text{한%foo\n글}$", []string{"<mtext>한글</mtext>"}, []string{"math-error"}, 1},
		{"R3", "$x %foo \\text{\n+y$", []string{"<mi>x</mi><mo>+</mo><mi>y</mi>"}, []string{"math-error"}, 1},
	}
	for _, c := range cases {
		r := render(t, c.src)
		contains(t, r.HTML, c.want...)
		notContains(t, r.HTML, c.not...)
		if len(r.Warnings) != c.warns {
			t.Errorf("%s %q: 경고 %d 기대, %+v", c.round, shortSrc(c.src), c.warns, r.Warnings)
		}
	}
}

// ---- 무작위 입력 속성 시험

// nestingProne 은 조각을 되풀이하면 블록 중첩(인용·목록)이 깊어질 수 있는지다 — > 가 있거나 목록 표지 뒤에 공백이 온다.
func nestingProne(unit string) bool {
	return strings.Contains(unit, ">") || regexp.MustCompile(`(^|[^\S\n]|[-+*])[-+*][ \t]|\d[.)][ \t]`).MatchString(unit+unit)
}

// 무작위 글의 조각. 수식 구분자 찾기에 걸리는 것(구분자·이스케이프·\text·괄호·주석·공백·줄바꿈)과 마크다운 글자를 섞는다.
// 빈 줄은 넣지 않는다(문단을 나누면 시간 시험이 문단 길이를 재지 못한다).
var mathAtoms = []string{"$", "$$", `\$`, `\\`, `\text{`, "{", "}", "%", `\%`, " ", "  ", "\n", "x", "1", "+", "*", "_", "|", "`",
	`\frac{a}{b}`, "가", `\foo`, `\color{red}`, "\t", "#", ">", "-", "a@b.c", "www.", "~"}

func randMath(rng *rand.Rand, n int) string {
	var b strings.Builder
	for range n {
		b.WriteString(mathAtoms[rng.Intn(len(mathAtoms))])
	}
	return b.String()
}

// 속성 1 — 시간 선형성: 무작위 조각을 되풀이한 문단과, 리뷰가 찾은 "긴 구간을 읽고 곧 실패" 꼴의 가족을 크기를 두 배씩 키워
// 렌더한다(64KB~512KB, 크기 넷). 512KB 의 비용(CPU 시간)이 64KB 의 27배(3³, +3×5ms)를 넘지 않고 연속한 두 쌍이 모두 세 배(+5ms)를
// 넘지 않는다(timingtest judge — 이차면 네 배씩). 잡음을 줄이려 두 번 재어 짧은 쪽을 쓰고,
// 실패로 보이면 모든 크기를 다시 잰다(timingtest, itda-hyve#130).
func TestMathPropLinear(t *testing.T) {
	rng := rand.New(rand.NewSource(1163))
	type family struct {
		name string
		gen  func(size int) string
	}
	var fams []family
	for i := range 40 {
		unit := randMath(rng, 2+rng.Intn(6))
		unit = strings.ReplaceAll(unit, "\n", " ") // 한 줄 문단(goldmark 의 줄 처리와 섞이지 않게)
		for nestingProne(unit) {
			// 되풀이하면 블록 중첩이 깊어지는 조각(>·목록 표지 + 공백)은 뺀다 — goldmark 의 깊은 중첩은 이차이고 그 상한은 별도 이슈다
			unit = strings.ReplaceAll(randMath(rng, 2+rng.Intn(6)), "\n", " ")
		}
		fams = append(fams, family{fmt.Sprintf("무작위%02d %q", i, unit), func(size int) string { return strings.Repeat(unit, size/len(unit)+1) }})
	}
	sqrt := func(n int) int {
		k := 1
		for k*k < n {
			k++
		}
		return k
	}
	// k 개의 여는 꼴이 저마다 같은 긴 구간을 읽고 실패한다 — k ∝ √n 이면 예산 없이 k·n 이다(R3)
	for name, parts := range map[string][3]string{
		"text 닫고 긴 공백 뒤 $": {`$\text{`, "}", " "},
		"긴 공백 뒤 $":         {`$x`, "", " "},
		"주석 안의 긴 글":        {`$\text{%`, "", "y"},
		"text 안의 긴 $":      {`$\text{`, "", "$"},
		"블록 꼴 긴 공백":        {`$$\text{`, "}", " "},
	} {
		fams = append(fams, family{name, func(size int) string {
			k := sqrt(size)
			return strings.Repeat(parts[0], k) + strings.Repeat(parts[1], k) + strings.Repeat(parts[2], size) + "$"
		}})
	}
	// 비용 계산 자체: k 개의 여는 꼴이 저마다 긴 구간(n)을 읽고 실패하면 실제로 읽은 양 k·n 이 예산(64KB + 문단 × 4)을 넘어
	// 예산 경고가 꼭 한 번 난다. k = √n/4 로 두어 여는 꼴 자체의 바이트(약 n/2)는 예산 안이다 — 긴 구간(공백·주석·\text 안의 글)을
	// 세지 않으면 경고가 나지 않는다. 그런 누락은 시간이 n^1.5 라 배증 비로는 잡히지 않는다(리뷰 R3)
	for name, parts := range map[string][3]string{
		"text 닫고 긴 공백 뒤 $": {`$\text{`, "}", " "},
		"주석 안의 긴 글":        {`$\text{%`, "", "y"},
		"text 안의 긴 $":      {`$\text{`, "", "$"},
		"text 안의 긴 공백":     {`$\text{`, "", " "},
	} {
		size := 256_000
		k := sqrt(size) / 4
		src := strings.Repeat(parts[0], k) + strings.Repeat(parts[1], k) + strings.Repeat(parts[2], size) + "$"
		r := render(t, src)
		n := 0
		for _, w := range r.Warnings {
			if strings.Contains(w.Message, "닫히지 않은 $ 가 너무 많아") {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%s: 예산 경고 %d 개(한 번이어야 — 읽은 바이트를 빠뜨렸나)", name, n)
		}
	}
	for _, f := range fams {
		timingtest.AssertDoubling(t, f.name, []int{64_000, 128_000, 256_000, 512_000}, renderWork(t, f.gen),
			timingtest.Doubling{Reps: 2, Slack: 5 * time.Millisecond})
	}
}

// 속성 2·3 — 원문 보존과 경고 일관성. 무작위 문서 3천 개마다:
//   - 수식 노드의 원문(Raw)은 원문의 연속 조각이고, 앞의 수식보다 뒤에서 시작하며, 구분자로 열고 닫는다. Tex 는 그 안쪽이다.
//   - 그린 수식(<math>)과 그리지 못한 수식(.math-error)의 수가 수식 노드 수와 같고, 그리지 못한 것은 원문 글자 그대로다.
//   - .math-error 하나마다 "수식을 그리지 않고"·"닫는 $$ 가 없어" 경고가 하나다(경고 없이 원문으로 둔 수식이 없다).
//   - 경고의 줄 번호는 원문 안이다. 같은 입력을 다시 렌더하면 바이트까지 같고, 결과는 안전하다(assertSafe).
func TestMathPropRawAndWarnings(t *testing.T) {
	rng := rand.New(rand.NewSource(1164))
	for i := range 3000 {
		src := randMath(rng, 1+rng.Intn(40))
		r := render(t, src)
		if again := render(t, src); again.HTML != r.HTML {
			t.Fatalf("#%d %q: 다시 렌더한 결과가 다름", i, src)
		}
		assertSafe(t, r.HTML)
		nodes := mathNodes(t, src)
		at := 0
		for _, n := range nodes {
			k := strings.Index(src[at:], n.raw)
			if k < 0 {
				t.Fatalf("#%d %q: 수식 원문 %q 가 원문의 차례대로의 조각이 아님", i, src, n.raw)
			}
			at += k + len(n.raw)
			d := "$"
			if n.display {
				d = "$$"
			}
			// Tex 는 원문의 안쪽과 같다 — 다만 문단의 이어진 줄은 goldmark 가 줄머리 공백을 떼므로 공백을 접어 견준다
			inner := n.raw[len(d) : len(n.raw)-len(d)]
			if !strings.HasPrefix(n.raw, d) || !strings.HasSuffix(n.raw, d) || foldSpace(n.tex) != foldSpace(inner) {
				t.Fatalf("#%d %q: 구분자·Tex 가 맞지 않음 raw=%q tex=%q", i, src, n.raw, n.tex)
			}
		}
		maths, errs := countMath(t, r.HTML)
		blocks := countBlocks(t, src)
		if maths+errs != len(nodes)+blocks {
			t.Fatalf("#%d %q: 수식 노드 %d(+블록 %d) 인데 <math> %d · math-error %d\n%s", i, src, len(nodes), blocks, maths, errs, r.HTML)
		}
		warned := 0
		lines := strings.Count(src, "\n") + 1
		for _, w := range r.Warnings {
			if w.Code != WarnMath {
				continue
			}
			if w.Line < 1 || w.Line > lines {
				t.Fatalf("#%d %q: 경고 줄 %d 이 원문(%d 줄) 밖", i, src, w.Line, lines)
			}
			if strings.HasPrefix(w.Message, "수식을 그리지 않고") || strings.HasPrefix(w.Message, "닫는 $$ 가 없어") {
				warned++
			}
		}
		if warned != errs {
			t.Fatalf("#%d %q: math-error %d 개에 경고 %d 개\n%s\n%+v", i, src, errs, warned, r.HTML, r.Warnings)
		}
	}
}

type mathNode struct {
	raw, tex string
	display  bool
}

// mathNodes 는 렌더와 같은 파서로 원문을 읽어 글 안 수식 노드를 차례대로 모은다(표 칸 안의 것은 \| 를 바꾸므로 뺀다).
func mathNodes(t *testing.T, src string) []mathNode {
	t.Helper()
	body := normalize([]byte(src))
	st := newState(body, Meta{}, Options{})
	doc := st.md.Parser().Parse(textReader(body))
	var out []mathNode
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if m, ok := n.(*MathInline); ok && entering && !inTableCell(m) {
			out = append(out, mathNode{m.Raw, m.Tex, m.Display})
		}
		return ast.WalkContinue, nil
	})
	return out
}

// countBlocks 는 줄 머리 $$ 블록과 표 칸 안의 수식 수다(mathNodes 가 세지 않는 것).
func countBlocks(t *testing.T, src string) int {
	t.Helper()
	body := normalize([]byte(src))
	st := newState(body, Meta{}, Options{})
	doc := st.md.Parser().Parse(textReader(body))
	n := 0
	_ = ast.Walk(doc, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch m := c.(type) {
		case *MathBlock:
			n++
		case *MathInline:
			if inTableCell(m) {
				n++
			}
		}
		return ast.WalkContinue, nil
	})
	return n
}

// countMath 는 결과의 <math> 와 .math-error(span·p) 수다.
func countMath(t *testing.T, out string) (maths, errs int) {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "math" {
				maths++
			}
			for _, a := range n.Attr {
				if a.Key == "class" && a.Val == "math-error" {
					errs++
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return
}

// 속성 4 — Pandoc 과의 차등 비교(로컬에 pandoc 이 있을 때만). 글 안 $…$ 의 판정은 Pandoc tex_math_dollars 를 따른다(설계 1.10).
// 무작위 문단 2천 개를 pandoc 한 번에 넘겨 문단마다 InlineMath 목록을 견준다. 비교 범위는 규칙이 같은 곳으로 좁힌다:
// $$ 와 TeX 주석(%)은 넣지 않고(Pandoc 은 주석을 모르고 $$ 안의 이스케이프를 보지 않는다 — 1.10 에 적은 차이), \text{…} 는
// 늘 짝이 맞게 만든다(짝이 없는 \text{ 는 Temml 에서도 오류다). raw_tex 를 꺼 \cmd{…} 가 글을 삼키지 않게 한다.
func TestMathPropPandoc(t *testing.T) {
	if _, err := exec.LookPath("pandoc"); err != nil {
		t.Skip("pandoc 없음")
	}
	rng := rand.New(rand.NewSource(1165))
	// 줄바꿈 뒤에는 늘 글자를 둔다 — 줄머리의 +·1. 이 문단을 끊는 규칙은 CommonMark 와 Pandoc 이 다르다(수식과 무관)
	// 백틱은 넣지 않는다 — 짝 없는 `` 를 Pandoc 은 ` + 코드로, CommonMark 는 글자로 읽는다(수식과 무관한 코드 구간 규칙 차이).
	// 코드 안의 $ 는 TestMathSyntax 가 본다
	atoms := []string{"$", `\$`, `\\`, " ", "  ", "\nx", "x", "y", "1", "+", "가", "{", "}", `\frac{a}{b}`}
	var paras []string
	for range 2000 {
		var b strings.Builder
		for range 1 + rng.Intn(16) {
			if rng.Intn(8) == 0 {
				b.WriteString(`\text{` + []string{"a", "a $ b", "a $x$ b", "가 나", "{b}"}[rng.Intn(5)] + "}")
				continue
			}
			b.WriteString(atoms[rng.Intn(len(atoms))])
		}
		p := strings.TrimSpace(b.String())
		if p == "" || strings.HasPrefix(p, "{") || regexp.MustCompile(`^\d`).MatchString(p) {
			p = "앞 " + p
		}
		paras = append(paras, "P "+p)
	}
	src := strings.Join(paras, "\n\n") + "\n"
	cmd := exec.Command("pandoc", "-f", "markdown-raw_tex-smart+tex_math_dollars", "-t", "json")
	cmd.Stdin = strings.NewReader(src)
	outJSON, err := cmd.Output()
	if err != nil {
		t.Fatalf("pandoc: %v", err)
	}
	want := pandocMath(t, outJSON)
	got := ourMathPerPara(t, src)
	if len(want) != len(got) {
		t.Fatalf("문단 수 pandoc %d · mdoc %d", len(want), len(got))
	}
	ws := regexp.MustCompile(`\s+`)
	norm := func(s []string) string {
		for i := range s {
			s[i] = ws.ReplaceAllString(s[i], " ")
		}
		return strings.Join(s, " ‖ ")
	}
	bad := 0
	for i := range want {
		if w, g := norm(want[i]), norm(got[i]); w != g {
			bad++
			if bad <= 5 {
				t.Errorf("문단 %q\n pandoc %q\n mdoc   %q", paras[i], w, g)
			}
		}
	}
	if bad > 0 {
		t.Errorf("pandoc 과 다른 문단 %d / %d", bad, len(want))
	}
}

// pandocMath 는 pandoc JSON 의 문단마다 InlineMath 내용 목록이다.
func pandocMath(t *testing.T, b []byte) [][]string {
	t.Helper()
	var doc struct {
		Blocks []json.RawMessage `json:"blocks"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	var out [][]string
	for _, blk := range doc.Blocks {
		var v any
		_ = json.Unmarshal(blk, &v)
		var list []string
		var walk func(any)
		walk = func(x any) {
			switch x := x.(type) {
			case map[string]any:
				if x["t"] == "Math" {
					c := x["c"].([]any)
					if c[0].(map[string]any)["t"] == "InlineMath" {
						list = append(list, c[1].(string))
					}
					return
				}
				for _, y := range x {
					walk(y)
				}
			case []any:
				for _, y := range x {
					walk(y)
				}
			}
		}
		walk(v)
		out = append(out, list)
	}
	return out
}

// ourMathPerPara 는 같은 원문을 mdoc 의 파서로 읽어 문단마다 글 안 수식(Display 아님)의 Tex 목록이다.
func ourMathPerPara(t *testing.T, src string) [][]string {
	t.Helper()
	body := normalize([]byte(src))
	st := newState(body, Meta{}, Options{})
	doc := st.md.Parser().Parse(textReader(body))
	var out [][]string
	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		var list []string
		_ = ast.Walk(c, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if m, ok := n.(*MathInline); ok && entering && !m.Display {
				list = append(list, m.Tex)
			}
			return ast.WalkContinue, nil
		})
		out = append(out, list)
	}
	return out
}

var spaceRun = regexp.MustCompile(`\s+`)

func foldSpace(s string) string { return strings.TrimSpace(spaceRun.ReplaceAllString(s, " ")) }

func shortSrc(s string) string {
	if len(s) > 60 {
		return s[:60] + "…"
	}
	return s
}
