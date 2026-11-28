package mdoc

import (
	"strings"
	"testing"
)

// itda-hyve#1116 장 배율·장 맞춤 — 제목 {scale=}·{.fit} 의 값 검사와 자리 경고(slidescale.go slideHeadingAttrs).
func TestSlideHeadingScale(t *testing.T) {
	cases := []syntaxCase{
		{name: "scale=0.75", src: "## 제목 {scale=0.75}\n\n글\n",
			want: []string{`<h2 id="제목" data-scale="0.75" data-line="1">제목</h2>`}},
		{name: "하한·상한 값", src: "## 가 {scale=0.6}\n\n글\n\n## 나 {scale=1.5}\n\n글\n\n## 다 {scale=.8}\n\n글\n\n## 라 {scale=1}\n\n글\n",
			want: []string{`data-scale="0.6"`, `data-scale="1.5"`, `data-scale=".8"`, `data-scale="1"`}},
		{name: ".fit", src: "## 제목 {.fit}\n\n글\n", want: []string{`<h2 id="제목" class="fit" data-line="1">`}},
		{name: ".fit + scale", src: "## 제목 {.fit scale=0.8}\n\n글\n", want: []string{`class="fit" data-scale="0.8"`}},
		{name: "범위 밖 값은 경고하고 뗀다", src: "## 제목 {scale=1.6}\n\n글\n", not: []string{"data-scale"}, warn: WarnBadAttribute},
		{name: "하한 밑은 경고", src: "## 제목 {scale=0.5}\n\n글\n", not: []string{"data-scale"}, warn: WarnBadAttribute},
		{name: "숫자가 아니면 경고", src: "## 제목 {scale=big}\n\n글\n", not: []string{"data-scale"}, warn: WarnBadAttribute},
		{name: "단위·퍼센트는 경고", src: "## 제목 {scale=80%}\n\n글\n", not: []string{"data-scale"}, warn: WarnBadAttribute},
		{name: "따옴표 값도 꼴은 같다", src: "## 제목 {scale=\"0.8\"}\n\n글\n", want: []string{`data-scale="0.8"`}},
		{name: "다른 속성은 그대로", src: "## 제목 {#a .b scale=2 k=v}\n\n글\n", want: []string{`id="a"`, `class="b"`, `data-k="v"`}, not: []string{"data-scale"}, warn: WarnBadAttribute},
		{name: "구역 제목(슬라이드 단계 위)은 자리 경고", src: "# 구역 {scale=0.8}\n\n## 장\n\n글\n", want: []string{`data-scale="0.8"`}, warn: WarnBadAttribute},
		{name: "슬라이드 단계 아래 제목은 자리 경고", src: "## 장\n\n글\n\n### 작은 {.fit}\n\n글\n", warn: WarnBadAttribute},
		{name: "상자 안 제목은 자리 경고", src: ":::box\n## 안 {.fit}\n\n글\n:::\n\n## 장\n\n글\n", warn: WarnBadAttribute},
		{name: "slides.level 을 따른다", src: "---\nslides:\n  level: 3\n---\n\n## 위\n\n### 장 {scale=0.8}\n\n글\n", want: []string{`data-scale="0.8"`}},
	}
	runCases(t, cases)
	// 자리 경고 문구와 줄
	r := render(t, "## 장\n\n글\n\n### 작은 {.fit}\n\n글\n")
	if len(r.Warnings) != 1 || r.Warnings[0].Line != 5 || !strings.Contains(r.Warnings[0].Message, "## 제목") {
		t.Errorf("자리 경고: %+v", r.Warnings)
	}
	// 값 경고는 하나 — 자리 경고와 겹치지 않는다(뗀 값은 자리를 보지 않는다)
	r = render(t, "# 구역 {scale=9}\n\n## 장\n\n글\n")
	if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0].Message, "scale=9") {
		t.Errorf("값 경고: %+v", r.Warnings)
	}
}

// itda-hyve#1116 코드 배율·맞춤 — 펜스 낱말 scale=·fit(parseCodeScale·parseCodeInfo).
func TestCodeScaleFit(t *testing.T) {
	cases := []syntaxCase{
		{name: "scale=0.8", src: "```python scale=0.8\na\nb\n```", want: []string{`<pre data-line="1" data-scale="0.8"><code class="language-python">`}},
		{name: "fit", src: "```python fit\na\nb\n```", want: []string{`<pre data-line="1" data-fit="on"><code class="language-python">`}},
		{name: "scale + fit + 다른 낱말", src: "```bash title=\"run.sh\" cols=2 scale=0.7 fit nonumber {2}\na\nb\n```",
			want: []string{`<pre data-line="1" data-title="run.sh" data-cols="2" data-scale="0.7" data-fit="on" data-ln="off" data-hl="2"><code class="language-bash">`}},
		{name: "하한·상한", src: "```text scale=0.55\na\n```\n\n```text scale=1.5\na\n```", want: []string{`data-scale="0.55"`, `data-scale="1.5"`}},
		{name: "첫 낱말 fit — 언어 없음", src: "```fit\na\nb\n```", want: []string{`<pre data-line="1" data-fit="on"><code>`}, not: []string{"language-"}},
		{name: "범위 밖은 경고하고 무시", src: "```text scale=0.5\na\n```", not: []string{"data-scale"}, warn: WarnBadAttribute},
		{name: "따옴표 값은 틀린 값", src: "```text scale=\"0.8\"\na\n```", not: []string{"data-scale"}, warn: WarnBadAttribute},
		{name: "fit 은 정확히 그 낱말만", src: "```text fits\na\n```\n\n```text fit=1\na\n```\n\n```text myfit\na\n```", not: []string{"data-fit"}},
		{name: "다른 속성의 따옴표 값 안은 낱말 아님", src: "```text title=\"a fit scale=0.8\"\na\n```", not: []string{"data-fit", "data-scale"}},
		{name: "같은 낱말이 여럿이면 앞의 것", src: "```text scale=0.7 scale=0.9\na\n```", want: []string{`data-scale="0.7"`}, not: []string{`data-scale="0.9"`}},
		{name: "mermaid 그림에는 해당 없음", src: "```mermaid fit scale=0.8\ngraph TD\nA-->B\n```", want: []string{`<figure class="diagram"`}, not: []string{"data-fit", "data-scale"}},
	}
	runCases(t, cases)
	// 코드 글자·강조는 바뀌지 않는다 — 속성만 더해진다
	a := render(t, "```go\nx := 1\ny := 2\n```\n").HTML
	b := render(t, "```go scale=0.8 fit\nx := 1\ny := 2\n```\n").HTML
	if strings.Replace(b, ` data-scale="0.8" data-fit="on"`, "", 1) != a {
		t.Errorf("scale·fit 이 속성 말고 출력을 바꿈:\n%s\n---\n%s", a, b)
	}
	// 경고는 펜스 줄
	r := render(t, "글\n\n```text scale=9\na\n```\n")
	if len(r.Warnings) != 1 || r.Warnings[0].Line != 3 || r.Warnings[0].Code != WarnBadAttribute {
		t.Errorf("경고 줄: %+v", r.Warnings)
	}
}
