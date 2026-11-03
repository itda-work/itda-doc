package texmath

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// 결과 MathML 을 브라우저처럼 파싱해 허용 목록으로 본다. sanitize.go 의 표와 따로 적는다 — 구현이 틀려도 시험이 잡게.
var (
	mathElems = setOf(`math mrow mi mn mo mtext ms mspace mfrac msqrt mroot msub msup msubsup munder mover munderover
		mmultiscripts mprescripts none mtable mtr mtd mstyle mpadded mphantom menclose`)
	mathAttrs = setOf(`class display mathvariant displaystyle scriptlevel fence separator stretchy symmetric largeop
		movablelimits accent accentunder form lspace rspace minsize maxsize linethickness width height depth voffset
		notation columnalign rowalign`)
	classRe     = regexp.MustCompile(`^[a-z0-9-]+(?: [a-z0-9-]+)*$`)
	valueRe     = regexp.MustCompile(`^[a-z0-9 .+-]+$`)
	styleAttrRe = regexp.MustCompile(`\sstyle=`)
	lenAttrs    = setOf("lspace rspace minsize maxsize linethickness width height depth voffset")
	lenValRe    = regexp.MustCompile(`^[+-]?(\d+(?:\.\d*)?|\.\d+)(em|ex|pt|px|mu)?$`)
)

// assertMathML 은 out 이 <math> 하나이고 그 안이 허용 목록뿐인지 본다. style·이벤트 속성·주소·색이 없다.
func assertMathML(t *testing.T, tex, out string) {
	t.Helper()
	if styleAttrRe.MatchString(out) || strings.Contains(strings.ToLower(out), "href") {
		t.Errorf("%q: style·href 가 나옴\n%s", tex, out)
	}
	ctx := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(out), ctx)
	if err != nil {
		t.Fatalf("%q: 파싱 실패 %v", tex, err)
	}
	if len(nodes) != 1 || nodes[0].Data != "math" || nodes[0].Namespace != "math" {
		t.Fatalf("%q: <math> 하나가 아님\n%s", tex, out)
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.ElementNode:
			if n.Namespace != "math" || !mathElems[n.Data] {
				t.Errorf("%q: 허용하지 않은 요소 %s:%s\n%s", tex, n.Namespace, n.Data, out)
			}
			for _, a := range n.Attr {
				switch {
				case a.Namespace != "" || !mathAttrs[a.Key]:
					t.Errorf("%q: <%s> 의 허용하지 않은 속성 %s=%q", tex, n.Data, a.Key, a.Val)
				case a.Key == "class" && !classRe.MatchString(a.Val):
					t.Errorf("%q: class %q", tex, a.Val)
				case a.Key != "class" && !valueRe.MatchString(a.Val):
					t.Errorf("%q: <%s> %s 값 %q", tex, n.Data, a.Key, a.Val)
				}
				if lenAttrs[a.Key] && (!lenValRe.MatchString(a.Val) || !length(a.Val)) {
					t.Errorf("%q: 길이 상한을 넘음 %s=%q", tex, a.Key, a.Val)
				}
			}
		case html.CommentNode, html.DoctypeNode:
			t.Errorf("%q: 주석·선언이 나옴", tex)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(nodes[0])
}

// 지원 범위(안내서의 표와 같은 분류). 모두 그려지고 허용 목록을 지나야 한다.
var supported = []struct {
	name, tex string
	block     bool // 블록 수식($$)에서만 쓰는 환경
}{
	{"분수", `\frac{a}{b} + \dfrac{1}{2} + \tfrac12 + a \over b`, false},
	{"근호", `\sqrt{x^2+y^2} + \sqrt[3]{x}`, false},
	{"첨자", `x_i^2 + a_{n+1} + {}_nC_r + f'(x) + f''(x)`, false},
	{"큰 연산자", `\sum_{i=1}^{n} i = \frac{n(n+1)}{2},\ \prod_k a_k,\ \int_0^\infty e^{-x}\,dx,\ \oint_C,\ \iint`, false},
	{"극한", `\lim_{x\to 0}\frac{\sin x}{x} = 1,\ \max_k,\ \operatorname*{argmax}_x f(x)`, false},
	{"그리스 문자", `\alpha\beta\gamma\delta\epsilon\varepsilon\theta\lambda\mu\pi\sigma\phi\omega\Gamma\Delta\Sigma\Omega`, false},
	{"관계·연산 기호", `\pm\mp\times\div\cdot\leq\geq\neq\approx\equiv\sim\propto\in\notin\subset\subseteq\cup\cap\forall\exists\to\Rightarrow\iff\infty\partial\nabla`, false},
	{"괄호 크기", `\left(\frac{a}{b}\right) \left[x\right] \left\{y\right\} \left|z\right| \left\langle w\right\rangle \big( \Big( \bigg( \Bigg(`, false},
	{"행렬", `\begin{pmatrix}1&2\\3&4\end{pmatrix} \begin{bmatrix}a&b\\c&d\end{bmatrix} \begin{vmatrix}a&b\\c&d\end{vmatrix} \begin{matrix}a&b\end{matrix}`, false},
	{"array 와 선", `\begin{array}{|l|c|r|}\hline a&b&c\\\hline d&e&f\\\hline\end{array}`, false},
	{"경우 나누기", `f(x)=\begin{cases}x & x \geq 0\\ -x & \text{그 외}\end{cases}`, false},
	{"정렬 aligned", `\begin{aligned}a &= b + c\\ &= d\end{aligned}`, false},
	{"한글 섞기", `\text{매출} = \text{단가} \times \text{수량},\ \text{이익률} = \frac{\text{영업이익}}{\text{매출}} \times 100\%`, false},
	{"악센트", `\vec{v}\hat{x}\bar{y}\tilde{z}\dot{a}\ddot{a}\widehat{abc}\overline{ab}\underline{ab}`, false},
	{"위·아래 묶음", `\overbrace{a+\cdots+a}^{n}\ \underbrace{b+\cdots+b}_{m}`, false},
	{"화살표 위 글", `\xrightarrow{f} \xleftarrow[a]{b} \overset{!}{=} \underset{x}{\arg\min} \stackrel{def}{=}`, false},
	{"글꼴", `\mathbb{R}\mathcal{L}\mathfrak{g}\mathbf{x}\mathit{x}\mathrm{d}\mathsf{x}\mathtt{x}\boldsymbol{\alpha}`, false},
	{"이항 계수", `\binom{n}{k} \tbinom{n}{k} \dbinom{n}{k}`, false},
	{"간격", `a\,b\:c\;d\quad e\qquad f\!g\ h \hspace{1em} i`, false},
	{"줄임표", `1, 2, \dots, n \quad a_1 + \cdots + a_n \quad \vdots \quad \ddots`, false},
	{"나머지", `a \equiv b \pmod{n},\ a \bmod b`, false},
	{"글자", `\text{if } x > 0,\ \textbf{굵게} \textit{기울임} \operatorname{sgn}(x)`, false},
	{"취소선", `\cancel{x} \bcancel{y} \xcancel{z}`, false},
	{"상자", `\boxed{E = mc^2}`, false},
	{"매크로", `\newcommand{\R}{\mathbb{R}} f: \R \to \R,\ \def\abs#1{\left|#1\right|}\abs{x}`, false},
	{"번호 환경(번호 없이)", `\begin{align}a&=b\\c&=d\end{align}`, true},
	{"gather", `\begin{gather}a=b\\c=d\end{gather}`, true},
	{"split·multline", `\begin{multline}a+b\\+c\end{multline}`, true},
	{"equation", `\begin{equation}E = mc^2\end{equation}`, true},
	{"특수 글자", `\% \$ \& \# \_ \{ \}`, false},
	{"부등식", `a < b > c \le d \ge e \ll f \gg g`, false},
	{"작은 행렬", `\begin{smallmatrix}a&b\\c&d\end{smallmatrix}`, false},
	{"부분 첨자 sideset", `\sideset{_a^b}{_c^d}\sum`, false},
}

func TestSupported(t *testing.T) {
	for _, c := range supported {
		for _, display := range []bool{false, true} {
			if c.block && !display {
				continue // TestDisplayOnly
			}
			out, err := Render(c.tex, display)
			if err != nil {
				t.Errorf("%s (display=%v): %v", c.name, display, err)
				continue
			}
			assertMathML(t, c.tex, out)
			if display != strings.HasPrefix(out, `<math class="tml-display" display="block">`) {
				t.Errorf("%s: display=%v 인데 %s", c.name, display, out[:min(60, len(out))])
			}
		}
	}
}

// 번호 붙는 AMS 환경은 블록 수식에서만 되고, 번호를 붙이지 않는다(\tag 를 막는 것과 같은 까닭).
func TestDisplayOnly(t *testing.T) {
	for _, env := range []string{"align", "align*", "gather", "equation", "multline", "alignat"} {
		tex := `\begin{` + env + `}a&=b\end{` + env + `}`
		switch env {
		case "alignat":
			tex = `\begin{alignat}{1}a&=b\end{alignat}`
		case "equation", "multline":
			tex = `\begin{` + env + `}a=b\end{` + env + `}`
		}
		out, err := Render(tex, true)
		if err != nil {
			t.Errorf("%s 블록: %v", env, err)
			continue
		}
		if strings.Contains(out, "(1)") || strings.Contains(out, "tml-eqn") || strings.Contains(out, "mlabeledtr") {
			t.Errorf("%s: 식 번호가 나옴\n%s", env, out)
		}
		if _, err := Render(tex, false); !isCode(err, CodeSyntax) || !strings.Contains(err.Error(), "블록 수식에서만") {
			t.Errorf("%s 글 안: 블록에서만 쓴다는 오류여야 함: %v", env, err)
		}
	}
}

func isCode(err error, code string) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == code
}

// 문서에 모양을 적는 명령(색·크기·HTML·주소·배치·식 번호)은 막고, 걸린 명령을 알린다.
func TestBlocked(t *testing.T) {
	for _, tex := range []string{
		`\color{red}{x}`, `\textcolor{red}{x}`, `\colorbox{yellow}{x}`, `\fcolorbox{red}{blue}{x}`, `\definecolor{a}{RGB}{1,2,3}`,
		`\Huge x`, `\tiny x`, `\large x`, `\small x`, `\fontsize{50}{60}x`,
		`\href{javascript:alert(1)}{x}`, `\url{https://evil.example}`, `\includegraphics{a.png}`, `\htmlStyle{color:red}{x}`,
		`\htmlClass{page-break}{x}`, `\htmlId{a}{x}`, `\htmlData{a=b}{x}`, `\class{a}{x}`, `\cssId{a}{x}`, `\style{color:red}{x}`, `\bbox[red]{x}`,
		`\rule{100em}{100em}`, `\raisebox{50em}{x}`, `\llap{x}`, `\rlap{x}`, `\clap{x}`, `\mathrlap{x}`,
		`a \tag{1}`, `a \label{eq}`, `\eqref{eq}`, `\ref{eq}`, `a \notag`, `a \nonumber`,
		`\csname color\endcsname{red}x`, `\expandafter\a\b`,
		`\def\x{\color}\x{red}{y}`, `\newcommand{\c}{\textcolor}\c{red}{y}`,
		`x \textcolor {red}{y}`, `\Large{x}`,
	} {
		_, err := Render(tex, false)
		var e *Error
		if !errors.As(err, &e) || e.Code != CodeBlocked || e.Command == "" || !strings.Contains(e.Msg, e.Command) {
			t.Errorf("%q: 막혀야 함, %v", tex, err)
		}
	}
	// 환경
	if _, err := Render(`\begin{CD}A @>a>> B\end{CD}`, true); !isCode(err, CodeBlocked) {
		t.Errorf("CD: %v", err)
	}
	// 주석 안의 명령은 Temml 도 읽지 않는다 — 막지 않는다
	if _, err := Render("x % \\color{red}\n+ y", true); err != nil {
		t.Errorf("주석 안의 명령: %v", err)
	}
}

// 모르는 명령·환경은 그 이름을 알린다. 문법 오류는 문법 오류로.
func TestUnsupported(t *testing.T) {
	for tex, cmd := range map[string]string{`\foo{x}`: `\foo`, `\ce{H2O}`: `\ce`, `a + \unknownCommand`: `\unknownCommand`, `\begin{foo}x\end{foo}`: "foo"} {
		_, err := Render(tex, false)
		var e *Error
		if !errors.As(err, &e) || e.Code != CodeUnsupported || e.Command != cmd {
			t.Errorf("%q: 미지원 %s 이어야 함, %v", tex, cmd, err)
		}
	}
	for _, tex := range []string{`\frac{1}{`, `\left( x`, `a & b`, `}`, `\sqrt`, "   "} {
		if _, err := Render(tex, false); !isCode(err, CodeSyntax) {
			t.Errorf("%q: 문법 오류여야 함, %v", tex, err)
		}
	}
}

// Temml 이 ParseError 가 아닌 것(안의 TypeError)을 던지는 입력은 JS 엔진 문장 대신 정해진 한 문장이고(itda-work/itda-hyve#577
// 결정 1 — wasm 판과 같은 글, cmd/itda-doc-wasm/wasm_test.go), CodeEngine 이라 기억하지 않는다.
func TestInternalError(t *testing.T) {
	for _, tex := range []string{`x^`, `x_`, `\operatorname{__proto__}`} {
		_, err := Render(tex, false)
		var e *Error
		if !errors.As(err, &e) || e.Code != CodeEngine || e.Msg != internalMsg {
			t.Errorf("%q: 내부 오류 한 문장이어야 함, %#v", tex, err)
		}
		cacheMu.Lock()
		_, cached := cache[key{tex, false}]
		cacheMu.Unlock()
		if cached {
			t.Errorf("%q: 내부 오류가 기억됨", tex)
		}
	}
	// 같은 문서의 다음 수식은 그려진다(예외를 낸 실행기를 버리고 새로 만든다)
	if _, err := Render(`x^2`, false); err != nil {
		t.Fatal(err)
	}
}

// 상한: 길이·중첩·매크로 확장·간격 크기. 상한까지는 그리고 넘으면 CodeLimit.
func TestLimits(t *testing.T) {
	ok := func(tex string) {
		t.Helper()
		if _, err := Render(tex, false); err != nil {
			t.Errorf("상한 안 %q: %v", clip(tex, 40), err)
		}
	}
	limit := func(tex string) {
		t.Helper()
		if _, err := Render(tex, false); !isCode(err, CodeLimit) {
			t.Errorf("상한 밖 %q: %v", clip(tex, 40), err)
		}
	}
	long := strings.Repeat("a+", MaxInputBytes/2)
	ok(long[:MaxInputBytes-1] + "a")
	limit(long[:MaxInputBytes] + "a")

	nest := func(n int) string { return strings.Repeat("{", n) + "x" + strings.Repeat("}", n) }
	ok(nest(MaxDepth))
	limit(nest(MaxDepth + 1))
	frac := func(n int) string { return strings.Repeat(`\frac{1}{`, n) + "x" + strings.Repeat("}", n) }
	ok(frac(MaxDepth))
	limit(frac(MaxDepth + 1))
	lr := func(n int) string { return strings.Repeat(`\left(`, n) + "x" + strings.Repeat(`\right)`, n) }
	ok(lr(MaxDepth))
	limit(lr(MaxDepth + 1))

	// 매크로: 재귀·지수 폭발은 maxExpand 에서 멈춘다
	limit(`\def\a{\a}\a`)
	limit(`\def\a{x\a}\a`)
	limit(`\def\b#1{#1#1}\def\c#1{\b{\b{#1}}}\c{\c{\c{\c{\c{x}}}}}`)
	ok(`\def\b#1{#1#1}\b{\b{\b{x}}}`)

	// 간격: 10em(100pt)까지 — 더 크면 Temml 의 maxSize 가 10em 으로 줄이고, 결과의 길이는 assertMathML 이 본다
	for _, tex := range []string{`a\hspace{10em}b`, `a\hspace{10.5em}b`, `a\hspace{1000em}b`, `a\kern 200pt b`, `a\mkern 2000mu b`, `a\quad\qquad b`} {
		out, err := Render(tex, false)
		if err != nil {
			t.Errorf("%q: %v", tex, err)
			continue
		}
		assertMathML(t, tex, out)
	}
	// 음수 간격은 정해진 것(\! 등)만 — 큰 음수 간격은 그리지 않는다(글자를 겹쳐 덮지 못하게)
	for _, tex := range []string{`a\hspace{-11em}b`, `a\kern-5em b`} {
		if _, err := Render(tex, false); err == nil {
			t.Errorf("%q: 그리지 않아야 함", tex)
		}
	}
}

// 수식마다 매크로 표가 새로 만들어진다: \gdef 도 다음 수식(같은 문서든 다른 문서든)에 남지 않는다.
func TestMacroIsolation(t *testing.T) {
	resetCache()
	if _, err := Render(`\gdef\leak{X}\global\def\leakb{Y}\leak\leakb`, false); err != nil {
		t.Fatal(err)
	}
	for _, tex := range []string{`\leak`, `\leakb`} {
		_, err := Render(tex, false)
		var e *Error
		if !errors.As(err, &e) || e.Code != CodeUnsupported || e.Command != tex {
			t.Errorf("%s 가 앞 수식에서 새어 나옴: %v", tex, err)
		}
	}
	// 기억(cache)을 지나지 않는 길(엔진)에서도
	for range 3 {
		if _, err := temml(`\gdef\leakc{Z}\leakc`, false); err != nil {
			t.Fatal(err)
		}
		if _, err := temml(`\leakc`, false); err == nil {
			t.Error("엔진에 \\gdef 가 남음")
		}
	}
}

// 반복·동시 렌더 결정성: 기억을 비우고 여러 고루틴이 같은 수식을 그려도 바이트까지 같다.
func TestDeterministic(t *testing.T) {
	want := map[string]string{}
	for _, c := range supported {
		out, err := render(c.tex, true)
		if err != nil {
			continue
		}
		want[c.tex] = out
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var bad []string
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 5 {
				for tex, w := range want {
					got, err := render(tex, true)
					if err != nil || got != w {
						mu.Lock()
						bad = append(bad, fmt.Sprintf("g%d#%d %q", g, i, clip(tex, 30)))
						mu.Unlock()
					}
				}
			}
		}()
	}
	wg.Wait()
	if len(bad) > 0 {
		t.Errorf("결과가 달라짐: %v", bad)
	}
}

// sanitize 는 Temml 이 낼 수 있는(또는 고장 나서 낼) 위험한 꼴을 거부한다.
func TestSanitizeRejects(t *testing.T) {
	for _, raw := range []string{
		`<math><script>alert(1)</script></math>`,
		`<math><mi onclick="alert(1)">x</mi></math>`,
		`<math><mi style="color:red">x</mi></math>`,
		`<math><mi style="background:url(https://evil.example)">x</mi></math>`,
		`<math><mrow mathcolor="red"><mi>x</mi></mrow></math>`,
		`<math><mrow mathbackground="red"><mi>x</mi></mrow></math>`,
		`<math><mi href="javascript:alert(1)">x</mi></math>`,
		`<math xmlns:xlink="http://www.w3.org/1999/xlink"><mi xlink:href="javascript:alert(1)">x</mi></math>`,
		`<math><annotation-xml encoding="text/html"><img src=x onerror=alert(1)></annotation-xml></math>`,
		`<math><semantics><mi>x</mi><annotation>x</annotation></semantics></math>`,
		`<math><mtext><span>x</span></mtext></math>`,
		`<math><mrow>글자</mrow></math>`,
		`<math><mi>x</mi></math><math><mi>y</mi></math>`,
		`<mi>x</mi>`,
		`<math><!-- c --><mi>x</mi></math>`,
		`<math><mi>x</mi>`,
		`<math><mspace width="1000em"></mspace></math>`,
		`<math><mpadded lspace="-0.5width"><mi>x</mi></mpadded></math>`,
		`<math><mo lspace="calc(1em)">x</mo></math>`,
		`<math><mtd style="padding-left:1em;position:absolute"></mtd></math>`,
		`<math><mi mathvariant="bold&quot; onclick=&quot;x">x</mi></math>`,
		`<math><mstyle mathsize="5em"><mi>x</mi></mstyle></math>`,
		`<math><maction actiontype="toggle"><mi>x</mi></maction></math>`,
		`<math><svg:svg xmlns:svg="http://www.w3.org/2000/svg"></svg:svg></math>`,
		`<math><mi dir="rtl">x</mi></math>`,
		`<math><mi id="x">x</mi></math>`,
		`<math><mi tabindex="1">x</mi></math>`,
	} {
		if out, err := sanitize(raw, false); err == nil {
			t.Errorf("거부해야 함 %s\n→ %s", raw, out)
		}
	}
	// 글자의 꺾쇠·따옴표·앰퍼샌드는 이스케이프되고, 모르는 class 낱말은 버리며, display 는 렌더 꼴이 정한다
	out, err := sanitize(`<math display="block" class="page-break tml-display"><mtext class="x toc tml-left">&lt;script&gt;"&amp;'</mtext></math>`, false)
	if err != nil || out != `<math><mtext class="tml-left">&lt;script&gt;&quot;&amp;&#39;</mtext></math>` {
		t.Errorf("정화 결과 %q %v", out, err)
	}
	out, err = sanitize(`<math><mtd style="padding-left:0em;padding-right:5.9776pt;border-top:0.06em dashed;padding-top:3.1em"><mi>x</mi></mtd></math>`, true)
	if err != nil || out != `<math class="tml-display" display="block"><mtd class="tml-bt-d tml-pl-0 tml-pr-a"><mi>x</mi></mtd></math>` {
		t.Errorf("style → class %q %v", out, err)
	}
}

// 결과에 style 이 없다(문서 창 CSP 가 막고, 1.9 의 width 불변식). Temml 이 style 로 배치하는 환경도 class 로만 나온다.
func TestNoStyleInOutput(t *testing.T) {
	for _, tex := range []string{
		`\begin{array}{|c|c|}\hline a&b\\\hline\end{array}`, `\begin{array}{c:c}a&b\end{array}`, `\begin{array}{c||c}a&b\end{array}`,
		`\begin{aligned}a&=b\\&=c\end{aligned}`, `\boxed{x}`, `\hat{x}`, `f\!g`, `\begin{cases}a&b\\c&d\end{cases}`,
		`\begin{multline}a\\b\end{multline}`,
	} {
		out, err := Render(tex, true)
		if err != nil {
			t.Errorf("%q: %v", tex, err)
			continue
		}
		assertMathML(t, tex, out)
	}
}

// 엔진이 없거나 고장 나도 패닉하지 않는다. Temml 이 던진 예외 뒤에도 다음 수식이 된다.
func TestEngineRecovers(t *testing.T) {
	for range 3 {
		if _, err := Render(`\frac{`, false); err == nil {
			t.Fatal("오류여야 함")
		}
		resetCache()
		if _, err := Render(`x`, false); err != nil {
			t.Fatal(err)
		}
	}
}

// FuzzRender 는 어떤 TeX 에도 패닉하지 않고, 그린 결과는 늘 허용 목록 안인지 본다.
//
//	go test ./texmath -run '^$' -fuzz FuzzRender -fuzztime 60s
func FuzzRender(f *testing.F) {
	for _, c := range supported {
		f.Add(c.tex, true)
	}
	for _, s := range []string{
		`\color{red}x`, `\href{x}{y}`, `\text{<script>alert(1)</script>}`, `\text{"'&<>}`, `\def\a{\a}\a`, `\verb|<b>|`,
		`\char"3C`, `\mathrm{\&}`, `\operatorname{<img>}`, `\begin{array}{c|c}a&b\end{array}`, `\\[5em]`, `a\\[100em]b`,
		`\begin{matrix}a\\[3em]b\end{matrix}`, `\phantom{x}\vphantom{y}\hphantom{z}`, `\smash{x}`, `\mathchoice{a}{b}{c}{d}`,
		`\not=`, `\mathring{a}`, `\overbrace{x}^{\text{<b>}}`, `\Bigg\langle`, `\sqrt[\text{<}]{x}`,
	} {
		f.Add(s, false)
	}
	f.Fuzz(func(t *testing.T, tex string, display bool) {
		out, err := Render(tex, display)
		if err != nil {
			var e *Error
			if !errors.As(err, &e) || e.Msg == "" {
				t.Fatalf("오류가 *Error 가 아님: %T %v", err, err)
			}
			return
		}
		assertMathML(t, tex, out)
	})
}
