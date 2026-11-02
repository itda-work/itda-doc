# 마크다운 수식 — `$…$` 와 `$$…$$` 안내서

Itda Doc 문서에서 수식을 쓰는 사람을 위한 안내서다. 그리는 쪽은 Itda Doc 을 그리는 구현(예: Itda Hyve Docs 의 문서 창)이다. 설계는 [DESIGN-markdown-docs.md](https://github.com/itda-work/itda-hyve/blob/main/docs/DESIGN-markdown-docs.md) 1.10,
기능 목록은 [FEATURES.md](https://github.com/itda-work/itda-hyve/blob/main/FEATURES.md) 11장이다. 견본은 [`internal/docview/testdata/math.md`](https://github.com/itda-work/itda-hyve/blob/main/internal/docview/testdata/math.md)다.

<!-- syntax:math:begin -->
수식은 **TeX(LaTeX 수식 문법) 그대로** 쓴다. 문서 창이 MathML 로 그리고(앱 안에서, 인터넷 없이), PDF 에서도 글자로 남아 선택·검색된다.
GitHub·VS Code·Obsidian 도 같은 표기를 수식으로 그린다 — 다른 뷰어에서도 수식이 보인다.

## 1. 한눈에

<!-- math-example -->
```markdown
영업이익률은 $m = \frac{\text{영업이익}}{\text{매출}} \times 100\%$ 다.

$$
x = \frac{-b \pm \sqrt{b^2 - 4ac}}{2a}
$$
```

- 글 안 수식: `$…$`. 문장 속에 작게 들어간다.
- 블록 수식: `$$…$$`. 줄 머리의 `$$` 로 열고 `$$` 로 닫으면 가운데 정렬된 한 블록이다. 글 가운데의 `$$…$$` 도 블록 꼴로 그린다.
- ```` ```math ```` 코드 블록도 블록 수식이다(GitHub 표기).

## 2. 달러 기호 규칙 — 금액은 수식이 아니다

규칙은 Pandoc(`tex_math_dollars`)과 같다.

| 쓴 글 | 결과 | 까닭 |
|---|---|---|
| `$x^2$` | 수식 | |
| `계약금 $30 와 잔금 $40` | 글자 | 닫는 `$` 바로 앞이 공백이다 |
| `$30–$40` | 글자 | 닫는 `$` 바로 뒤가 숫자다 |
| `$ x$` · `$x $` | 글자 | 여는 `$` 뒤·닫는 `$` 앞이 공백이다 |
| `\$5` | 글자 `$5` | `\$` 는 언제나 글자다 |
| `` `$x$` `` | 코드 | 코드·코드 블록 안의 `$` 는 글자다 |
| `$\$5$` | 수식 안의 `$` | 수식 안에서도 `\$` 는 글자이고 수식을 닫지 않는다 |
| `$x$$y$` | 수식 둘 | 수식이 되지 않은 `$` 의 바로 다음 자리에서 다시 본다(Pandoc 과 같다) |

`\text{…}` 안의 `$` 는 구분자가 아니다(`$\text{가격 \$5}$`, `$$\text{a $$ b}$$`) — 글 안·블록 모두 같고 줄을 넘어도 된다.
`%` 부터 줄 끝은 TeX 주석이다(글자 %는 `\%`). 주석은 명령·괄호의 뜻만 없앤다 — 주석 안의 `{`·`\text{` 는 수식을 닫는 데 영향이 없다.
주석 안의 `$` 는 Pandoc 처럼 수식을 닫는다. 다만 이미 열린 `\text{…}` 안이면(`$\text{a % $b` ↵ `c}$`) 주석 안의 `$` 도 글자다.
닫는 `$` 가 없는 `$` 는 그냥 글자다(경고 없음 — 금액에 흔하다). 닫는 `$$` 가 없는 `$$` 블록은 빈 줄에서 끝나고,
원문 그대로 두고 경고한다.

## 3. 표 안의 수식

표 칸에도 쓴다. 칸을 나누는 `|` 와 겹치므로 수식 안의 세로줄은 `\|` 로 쓴다(GFM 규칙 — 칸 안의 `\|` 는 `|` 다).
‖(노름)는 `\Vert` 나 `\lVert … \rVert` 로 쓴다.

```markdown
| 지표 | 식 |
|:--|:--|
| 영업이익률 | $\frac{P}{R} \times 100$ |
| 절댓값 | $\lvert x \| y \rvert$ |
```

## 4. 지원 범위

TeX 수식 문법의 대부분(Temml 이 아는 명령)을 쓴다. 아래는 시험(`texmath/texmath_test.go` 의 `supported`)이 그대로 확인하는 분류다.

<!-- math-support:begin -->
| 분류 | 예 |
|---|---|
| 분수 | `\frac{a}{b}` `\dfrac{1}{2}` `\tfrac12` `a \over b` |
| 근호 | `\sqrt{x^2+y^2}` `\sqrt[3]{x}` |
| 첨자 | `x_i^2` `a_{n+1}` `f'(x)` |
| 큰 연산자 | `\sum_{i=1}^{n}` `\prod_k` `\int_0^\infty` `\oint_C` `\iint` |
| 극한 | `\lim_{x\to 0}` `\max_k` `\operatorname*{argmax}_x` |
| 그리스 문자 | `\alpha` `\beta` `\pi` `\Gamma` `\Delta` `\Omega` |
| 관계·연산 기호 | `\pm` `\times` `\leq` `\neq` `\approx` `\in` `\subset` `\forall` `\Rightarrow` `\infty` |
| 괄호 크기 | `\left( … \right)` `\big(` `\Bigg(` `\langle … \rangle` |
| 행렬 | `pmatrix` `bmatrix` `vmatrix` `matrix` `smallmatrix` |
| 표와 선 | `\begin{array}{lcr} \hline a & b & c \\ \hline \end{array}` (세로선은 표 밖의 블록 수식에서 `{\|l\|c\|}` 처럼) |
| 경우 나누기 | `\begin{cases} x & x \geq 0 \\ -x & \text{그 외} \end{cases}` |
| 정렬 | `aligned`(글 안·블록), `align` `gather` `multline` `equation`(블록 수식에서만, 번호 없이) |
| 한글 섞기 | `\text{매출} = \text{단가} \times \text{수량}` |
| 악센트 | `\vec{v}` `\hat{x}` `\bar{y}` `\tilde{z}` `\widehat{abc}` `\overline{ab}` |
| 위·아래 묶음 | `\overbrace{…}^{n}` `\underbrace{…}_{m}` |
| 화살표 위 글 | `\xrightarrow{f}` `\overset{!}{=}` `\stackrel{def}{=}` |
| 글꼴 | `\mathbb{R}` `\mathcal{L}` `\mathfrak{g}` `\mathbf{x}` `\mathrm{d}` `\boldsymbol{\alpha}` |
| 이항 계수 | `\binom{n}{k}` |
| 간격 | `\,` `\:` `\;` `\quad` `\qquad` `\!` `\hspace{1em}` |
| 줄임표 | `\dots` `\cdots` `\vdots` `\ddots` |
| 취소선·상자 | `\cancel{x}` `\xcancel{z}` `\boxed{E=mc^2}` |
| 매크로 | `\newcommand{\R}{\mathbb{R}}` `\def\abs#1{…}` — 그 수식 안에서만 |
<!-- math-support:end -->

**식 번호는 붙이지 않는다**: `align`·`equation` 등은 번호 없이 그린다(`align*` 와 같다). `\tag`·`\label`·`\ref` 는 쓰지 않는다.

**매크로는 수식 하나 안에서만 산다**: `\newcommand`·`\def`·`\gdef` 로 만든 명령은 그 `$…$` 안에서만 쓴다.
다음 수식·다른 문서에는 남지 않는다.

## 5. 쓰지 않는 명령

문서에는 뜻만 적고 모양은 테마가 정한다(설계 1.1). 그래서 아래 명령은 그리지 않고 원문 그대로 두며, 줄 번호와 함께 경고한다.

| 종류 | 명령 |
|---|---|
| 색 | `\color` `\textcolor` `\colorbox` `\fcolorbox` `\definecolor` |
| 글자 크기 | `\tiny` … `\Huge` `\fontsize` |
| HTML·주소 | `\href` `\url` `\includegraphics` `\htmlClass` `\htmlStyle` `\class` `\style` `\bbox` |
| 배치 | `\rule` `\raisebox` `\llap` `\rlap` `\clap` |
| 식 번호 | `\tag` `\label` `\ref` `\eqref` `\notag` |
| 기타 | `\csname` `\expandafter` · 가환 도표 `CD` 환경 |

모르는 명령(`\ce` 화학식 등)과 문법 오류(`\frac{1}{` 처럼 괄호가 맞지 않는 것)도 같다 — 원문 그대로, 경고에 명령 이름이나 까닭이 나온다.
편집 화면에서 그리지 못한 수식은 물결 밑줄로 보인다.

## 6. 상한

<!-- math-limits:begin -->
| 항목 | 상한 |
|---|---|
| 수식 하나의 길이(바이트) | 4000 |
| 괄호·환경의 중첩 | 32 |
| 매크로 확장 횟수 | 500 |
| 간격·크기(em) | 10 |
| 문서 하나의 수식 수 | 2000 |
<!-- math-limits:end -->

넘으면 그 수식은 원문 그대로 두고 경고한다(간격·크기는 상한까지로 줄여 그린다). 문서의 수식이 상한을 넘으면 뒤의 수식만 원문이다.
닫히지 않는 `$` 가 아주 많은 문단(수십 KB)은 그 문단의 뒤의 `$` 를 글자로 두고 한 번 경고한다 — 편집기가 멈추지 않게.

## 7. 인쇄·PDF·발표

- 블록 수식은 쪽·슬라이드 사이에서 나뉘지 않는다. 글 안 수식이 든 긴 문단이 쪽 경계에 걸려도 수식 하나는 한쪽에 통째로 있다.
- 한 줄보다 넓은 블록 수식은 `aligned` 로 줄을 나눈다(`\\` 에서 줄바꿈). 쪽 너비를 넘는 부분은 잘린다.
- 수식 글꼴은 시스템의 수학 글꼴이다 — macOS 는 STIX Two Math, Windows 는 Cambria Math. 테마의 `--font-math` 로 바꾼다([문서 테마 안내서](themes.md)).
- PDF 에서 수식 글자(𝑥·∑·√ 등)는 선택·복사·검색된다. 분수·첨자 배치는 글자 순서대로 추출된다.
<!-- syntax:math:end -->
