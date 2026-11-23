package mdoc

import (
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
)

// itda-hyve#1024 코드 블록 하나만 줄 번호 끄기 — 펜스 낱말 nonumber 의 가림(hasCodeNoNumber·parseCodeInfo).
func TestCodeNoNumberParse(t *testing.T) {
	cases := []struct {
		info string
		want bool
		lang string // parseCodeInfo 의 언어(정규화 전)
	}{
		{info: "text", want: false, lang: "text"},
		{info: "text nonumber", want: true, lang: "text"},
		{info: "nonumber text", want: true, lang: ""}, // 첫 낱말 nonumber — 언어 없음
		{info: "nonumber", want: true, lang: ""},
		{info: "nonumber{2}", want: true, lang: ""}, // 붙여 쓴 줄 강조를 뗀 뒤 첫 낱말
		{info: `nonumber title="a.py"`, want: true, lang: ""},
		{info: "go {2} nonumber", want: true, lang: "go"},
		{info: "go nonumber {2}", want: true, lang: "go"},
		{info: "python cols=2 nonumber", want: true, lang: "python"},
		{info: `bash title="run.sh" nonumber`, want: true, lang: "bash"},
		// 값 없는 낱말 — 정확히 일치만
		{info: "text nonumbers", want: false, lang: "text"},
		{info: "text nonumber=1", want: false, lang: "text"},
		{info: "text mynonumber", want: false, lang: "text"},
		{info: "text NoNumber", want: false, lang: "text"},
		{info: "nonumber:a.py", want: false, lang: "nonumber"}, // 줄임 언어:파일명 — 낱말이 아니다(기존 가르기 그대로)
		// 다른 속성의 따옴표 값 안은 낱말이 아니다
		{info: `bash title="a nonumber"`, want: false, lang: "bash"},
		{info: `bash title='a nonumber' nonumber`, want: true, lang: "bash"},
		{info: `bash title="a nonumber`, want: false, lang: "bash"}, // 닫히지 않은 따옴표는 줄 끝까지 값
		// 접두가 같은 낱말의 따옴표 값 안도 낱말이 아니다(리뷰 1회차 [1])
		{info: `text nonumbers="a nonumber b"`, want: false, lang: "text"},
		{info: `text nonumber="a nonumber b"`, want: false, lang: "text"},
		{info: `text nonumber='a nonumber b'`, want: false, lang: "text"},
		{info: `text nonumbers="a nonumber`, want: false, lang: "text"}, // 닫히지 않은 따옴표
		{info: `text x="a" nonumber`, want: true, lang: "text"},
		{info: `text title="a b" nonumber`, want: true, lang: "text"}, // 닫는 따옴표 뒤 독립 낱말
		{info: `text nonumbers="a b" nonumber`, want: true, lang: "text"},
		{info: `text a"x y"nonumber`, want: false, lang: "text"}, // 따옴표에 붙은 글자는 같은 낱말
		// 낱말 경계는 unicode.IsSpace — 전각 공백·탭
		{info: "text　nonumber", want: true, lang: "text"},
		{info: "text\tnonumber", want: true, lang: "text"},
	}
	for _, c := range cases {
		ci := parseCodeInfo([]byte(c.info))
		if ci.noNumber != c.want || ci.lang != c.lang {
			t.Errorf("parseCodeInfo(%q) noNumber=%v lang=%q, 기대 %v %q", c.info, ci.noNumber, ci.lang, c.want, c.lang)
		}
	}
}

// 문서 자리 — pre 의 data-ln="off", 다른 펜스 속성과의 공존, 그림·수식.
func TestCodeNoNumberFence(t *testing.T) {
	cases := []syntaxCase{
		{name: "text nonumber", src: "```text nonumber\n$ a\n$ b\n```",
			want: []string{`<pre data-line="1" data-ln="off"><code class="language-text">`}},
		{name: "따옴표 값 안은 낱말 아님", src: "```bash title=\"a nonumber\"\na\nb\n```",
			want: []string{`<pre data-line="1" data-title="a nonumber"><code class="language-bash">`}, not: []string{"data-ln"}},
		{name: "줄 강조와 함께", src: "```go {2} nonumber\na\nb\n```",
			want: []string{`<pre data-line="1" data-ln="off" data-hl="2"><code class="language-go">`}},
		{name: "파일명·단과 함께", src: "```bash title=\"run.sh\" cols=2 nonumber\na\nb\n```",
			want: []string{`<pre data-line="1" data-title="run.sh" data-cols="2" data-ln="off"><code class="language-bash">`}},
		{name: "첫 낱말 nonumber — 언어 없음", src: "```nonumber\na\nb\n```",
			want: []string{`<pre data-line="1" data-ln="off"><code>`}, not: []string{"language-"}},
		{name: "첫 낱말 nonumber + 파일명 — 파일명으로 짐작", src: "```nonumber title=\"a.py\"\na = 1\nb = 2\n```",
			want: []string{`data-title="a.py" data-ln="off"><code class="language-python">`}},
		{name: "접두가 같은 낱말의 따옴표 값 안은 낱말 아님", src: "```text nonumbers=\"a nonumber b\"\na\nb\n```", not: []string{"data-ln"}},
		{name: "닫는 따옴표 뒤 독립 낱말", src: "```text title=\"a b\" nonumber\na\nb\n```",
			want: []string{`<pre data-line="1" data-title="a b" data-ln="off"><code class="language-text">`}},
		{name: "nonumbers 는 낱말 아님", src: "```text nonumbers\na\nb\n```", not: []string{"data-ln"}},
		{name: "1줄 블록에도 속성은 낸다", src: "```text nonumber\na\n```",
			want: []string{`<pre data-line="1" data-ln="off"><code class="language-text">`}},
		{name: "mermaid 그림에는 해당 없음", src: "```mermaid nonumber\ngraph TD\nA-->B\n```",
			want: []string{`<figure class="diagram"`}, not: []string{"data-ln"}},
		{name: "math 수식에는 해당 없음", src: "```math nonumber\nE=mc^2\n```",
			want: []string{`<div class="math-display"`}, not: []string{"data-ln"}},
		{name: "그리지 못한 math 는 코드로 남고 적용", src: "```math nonumber\n\\frac{\n```",
			want: []string{`data-ln="off"><code class="language-math">`}, warn: WarnMath},
	}
	runCases(t, cases)
	for _, c := range cases {
		if c.warn != "" {
			continue
		}
		if r := render(t, c.src); len(r.Warnings) != 0 {
			t.Errorf("%s: 경고가 없어야 함: %+v", c.name, r.Warnings)
		}
	}
	// 코드 글자·강조는 바뀌지 않는다 — 속성 하나만 더해진다
	a := render(t, "```go\nx := 1\ny := 2\n```\n").HTML
	b := render(t, "```go nonumber\nx := 1\ny := 2\n```\n").HTML
	if strings.Replace(b, ` data-ln="off"`, "", 1) != a {
		t.Errorf("nonumber 가 속성 말고 출력을 바꿈:\n%s\n---\n%s", a, b)
	}
	// 편집기용 언어(Doc.Code)도 같은 판정 — 첫 낱말 nonumber 는 언어가 아니다
	d := parseDoc(t, "```nonumber title=\"a.py\"\na = 1\n```\n")
	if fcs := nodes[*ast.FencedCodeBlock](d); len(fcs) != 1 {
		t.Fatalf("펜스 코드 %d 개", len(fcs))
	} else if got := d.Code(fcs[0]); got.Lang != "python" || got.Title != "a.py" {
		t.Errorf("Doc.Code %+v, 기대 python·a.py", got)
	}
}
