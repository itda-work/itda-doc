package mdoc

import (
	"strings"
	"testing"
)

// itda-hyve#487 코드 블록 단 나누기 — 펜스 줄 cols= 의 가림(parseCodeCols).
func TestCodeColsParse(t *testing.T) {
	cases := []struct {
		info string
		want int
		warn string // 경고 문구에 들어갈 글자("" 이면 경고 없음)
	}{
		{info: "python", want: 0},
		{info: "python cols=2", want: 2},
		{info: "python cols=3", want: 3},
		{info: "cols=2", want: 2}, // 언어 없이
		{info: "python cols=1", want: 0},
		{info: "python cols=4", want: 0, warn: "cols=4"},
		{info: "python cols=0", want: 0, warn: "cols=0"},
		{info: "python cols=x", want: 0, warn: "cols=x"},
		{info: "python cols=", want: 0, warn: "cols= "},
		{info: "python cols=02", want: 0, warn: "cols=02"},
		{info: `python cols="2"`, want: 0, warn: `cols="2"`},
		{info: "python cols=2x", want: 0, warn: "cols=2x"},
		// 낱말 경계 — mycols= 는 잡지 않는다
		{info: "python mycols=2", want: 0},
		// 같은 낱말이 여럿이면 앞의 것이 이긴다(틀린 값도 자리를 잡는다)
		{info: "python cols=2 cols=3", want: 2},
		{info: "python cols=x cols=3", want: 0, warn: "cols=x"},
		// title=·줄임 파일명과 함께
		{info: `python title="app.py" cols=2`, want: 2},
		{info: `python cols=3 title="app.py"`, want: 3},
		{info: "python:app.py cols=2", want: 2},
		// 다른 속성의 따옴표 값 안은 낱말이 아니다
		{info: `python title="a cols=2"`, want: 0},
		{info: `python title='a cols=2' cols=3`, want: 3},
		// 닫히지 않은 따옴표는 줄 끝까지가 그 값이다
		{info: `python title="a cols=2`, want: 0},
		// 낱말 경계는 unicode.IsSpace — 전각 공백·탭
		{info: "python　cols=2", want: 2},
		{info: "python\tcols=3", want: 3},
		// mermaid 의 그림 자리 낱말과 섞여도 각자 읽는다
		{info: "mermaid width=50% cols=2", want: 2},
	}
	for _, c := range cases {
		got, warns := parseCodeCols(c.info)
		if got != c.want {
			t.Errorf("parseCodeCols(%q) = %d, 기대 %d", c.info, got, c.want)
		}
		if c.warn == "" {
			if len(warns) != 0 {
				t.Errorf("parseCodeCols(%q) 경고가 없어야 함: %v", c.info, warns)
			}
			continue
		}
		if len(warns) != 1 || !strings.Contains(warns[0]+" ", c.warn) {
			t.Errorf("parseCodeCols(%q) 경고 %v 에 %q 가 없음", c.info, warns, c.warn)
		}
	}
}

// 문서 자리 — pre 의 data-cols 속성, 다른 펜스 속성과의 공존, 경고 줄 번호, 그림·수식.
func TestCodeColsFence(t *testing.T) {
	cases := []syntaxCase{
		{name: "속성 없음 — 출력 불변", src: "```python\na = 1\nb = 2\n```",
			want: []string{`<pre data-line="1"><code class="language-python">`}, not: []string{"data-cols"}},
		{name: "cols=2", src: "```python cols=2\na = 1\nb = 2\n```",
			want: []string{`<pre data-line="1" data-cols="2"><code class="language-python">`}},
		{name: "cols=3 언어 없이", src: "```cols=3\na\nb\n```",
			want: []string{`<pre data-line="1" data-cols="3"><code>`}},
		{name: "title= 과 함께", src: "```python title=\"app.py\" cols=2\na = 1\n```",
			want: []string{`<pre data-line="1" data-title="app.py" data-cols="2"><code class="language-python">`}},
		{name: "줄임 언어:파일명과 함께", src: "```python:app.py cols=3\na = 1\n```",
			want: []string{`data-title="app.py" data-cols="3"><code class="language-python">`}},
		{name: "cols=1 — 속성·경고 없음", src: "```python cols=1\na\nb\n```", not: []string{"data-cols"}},
		{name: "cols=4 — 경고하고 무시", src: "```python cols=4\na\nb\n```",
			want: []string{`<code class="language-python">`}, not: []string{"data-cols"}, warn: WarnBadAttribute},
		{name: "cols= — 경고하고 무시", src: "```python cols=\na\nb\n```", not: []string{"data-cols"}, warn: WarnBadAttribute},
		{name: "따옴표 값 안은 후보 아님", src: "```python title=\"a cols=2\"\na\nb\n```", not: []string{"data-cols"}},
		{name: "mermaid 그림에는 해당 없음", src: "```mermaid cols=2\ngraph TD\nA-->B\n```",
			want: []string{`<figure class="diagram"`}, not: []string{"data-cols"}},
		{name: "math 수식에는 해당 없음", src: "```math cols=2\nE=mc^2\n```",
			want: []string{`<div class="math-display"`}, not: []string{"data-cols"}},
		{name: "그리지 못한 math 는 코드로 남고 단도 적용", src: "```math cols=2\n\\frac{\n```",
			want: []string{`data-cols="2"><code class="language-math">`}, warn: WarnMath},
	}
	runCases(t, cases)

	// 줄 강조(itda-hyve#479)와 함께 — 두 낱말이 서로의 값·언어·파일명을 건드리지 않고, 어느 쪽도 경고하지 않는다
	combos := []syntaxCase{
		{name: "cols + {…}", src: "```python cols=2 {2,5}\na\nb\nc\nd\ne\n```",
			want: []string{`<pre data-line="1" data-cols="2" data-hl="2,5"><code class="language-python">`}},
		{name: "{…} + cols(순서 반대)", src: "```python {2,5} cols=3\na\nb\nc\nd\ne\n```",
			want: []string{`data-cols="3" data-hl="2,5"><code class="language-python">`}},
		{name: "붙인 {…} + cols", src: "```go{1-2} cols=2\na\nb\n```",
			want: []string{`data-cols="2" data-hl="1-2"><code class="language-go">`}},
		{name: "title= + cols + {…}", src: "```go title=\"a.go\" cols=2 {2}\na\nb\n```",
			want: []string{`data-title="a.go" data-cols="2" data-hl="2"><code class="language-go">`}},
		{name: "Deckset 지시어 + cols", src: "[.code-highlight: 1]\n\n```go cols=2\na\nb\n```",
			want: []string{`data-cols="2" data-hl="1"><code class="language-go">`}, not: []string{"code-highlight"}},
	}
	runCases(t, combos)
	for _, c := range combos {
		if r := render(t, c.src); len(r.Warnings) != 0 {
			t.Errorf("%s: 경고가 없어야 함: %+v", c.name, r.Warnings)
		}
	}

	// 경고 줄 번호 — 펜스 줄(문단 뒤 3째 줄)
	r := render(t, "앞\n\n```go cols=x\na\nb\n```\n")
	if len(r.Warnings) != 1 || r.Warnings[0].Line != 3 || r.Warnings[0].Code != WarnBadAttribute ||
		!strings.Contains(r.Warnings[0].Message, "cols=x") {
		t.Errorf("경고가 펜스 줄(3) 에 없음: %+v", r.Warnings)
	}
	// 그림이 된 mermaid 에는 틀린 cols 값 경고도 없다
	r = render(t, "```mermaid cols=x\ngraph TD\nA-->B\n```\n")
	if len(r.Warnings) != 0 {
		t.Errorf("그림에 cols 경고가 없어야 함: %+v", r.Warnings)
	}
	// 코드 글자(textContent)는 바뀌지 않는다
	a := render(t, "```go\nx := 1\ny := 2\n```\n").HTML
	b := render(t, "```go cols=2\nx := 1\ny := 2\n```\n").HTML
	if strings.Replace(b, ` data-cols="2"`, "", 1) != a {
		t.Errorf("cols 가 속성 말고 출력을 바꿈:\n%s\n---\n%s", a, b)
	}
}
