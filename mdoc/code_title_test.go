package mdoc

import (
	"strings"
	"testing"
)

// 코드 블록 파일명 머리 띠(#441, codetitle.go): 펜스 info string 에 파일명을 적으면 pre 에 data-title 속성을
// 달고(띠 글자는 화면이 CSS ::before 로 그린다), 언어를 비우면 파일명 확장자·이름으로 강조 언어를 짐작한다.
// 파일명은 속성값일 뿐 코드 글자(code textContent — 복사·줄 번호·검색)에 들어가지 않는다.
// 표기: 정식 `title="…"`(·작은따옴표·따옴표 없는 값)과 줄임 `언어:파일명` 둘 다 받는다.
func TestCodeTitleInfo(t *testing.T) {
	long := strings.Repeat("가", maxCodeTitle+5)
	cases := []struct {
		info string
		lang string
		want string
	}{
		{"", "", ""},
		{"   ", "", ""},
		{"go", "go", ""},
		{" yaml ", "yaml", ""},
		{`yaml title="docker-compose.yml"`, "yaml", "docker-compose.yml"}, // 정식
		{"yaml:docker-compose.yml", "yaml", "docker-compose.yml"},         // 줄임 — 깨지는 입력(예전엔 언어가 yamldocker-compose.yml 로 망가졌다)
		{`title=".env"`, "", ".env"},                                      // 언어 없이 — 첫 낱말에 = 가 있으면 언어 없음
		{":.env", "", ".env"},                                             // 줄임 + 언어 없이
		{"title='.env'", "", ".env"},                                      // 작은따옴표
		{"title=.env", "", ".env"},                                        // 따옴표 없는 값(공백 없음)
		{`title="my compose.yml"`, "", "my compose.yml"},                  // 따옴표 안 공백은 값이다
		{`yaml title='a.yml' x=1`, "yaml", "a.yml"},                       // 나머지 낱말 사이에도 있다
		{`yaml title=""`, "yaml", ""},                                     // 빈 값은 띠 없음
		{`yaml title="`, "yaml", ""},                                      // 닫는 따옴표가 없으면 값이 없다
		{`yaml mytitle="x.yml"`, "yaml", ""},                              // 낱말 경계 — mytitle= 은 title= 이 아니다
		{"go:main.go", "go", "main.go"},
		{"go:main.go:1", "go", "main.go:1"},                                  // 첫 : 로 가른다(파일명에 : 이 있어도 그대로)
		{`go:main.go title="other.md"`, "go", "main.go"},                     // 둘 다 있으면 줄임이 이긴다
		{"yaml:", "yaml", ""},                                                // 줄임 파일명이 비면 띠 없음(언어는 그대로)
		{"x=y", "", ""},                                                      // 첫 낱말에 = — 언어 없음 + title 낱말 없음
		{`go title="a` + "\x01" + `.go"`, "go", "a.go"},                      // 제어 문자는 파일명에서 지운다
		{`go title="` + long + `"`, "go", strings.Repeat("가", maxCodeTitle)}, // 200자 상한
	}
	for _, c := range cases {
		got := parseCodeInfo([]byte(c.info))
		if got.lang != c.lang || got.title != c.want {
			t.Errorf("%q → {%q %q}, 기대 {%q %q}", c.info, got.lang, got.title, c.lang, c.want)
		}
	}
}

// 렌더: data-title 속성(HTML 이스케이프)·언어 class·짐작·코드 글자 불변. 언어만 적은 info string 의 출력은
// 바이트까지 그대로다(golden 이 지킨다 — 여기서는 새 표기만 본다).
func TestCodeTitleRender(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
		not  []string
	}{
		{name: "정식", src: "```yaml title=\"docker-compose.yml\"\na: 1\n```\n",
			want: []string{`<pre data-line="1" data-title="docker-compose.yml"><code class="language-yaml">`}},
		{name: "줄임 — 언어가 망가지지 않는다", src: "```yaml:docker-compose.yml\na: 1\n```\n",
			want: []string{`data-title="docker-compose.yml"><code class="language-yaml"`}},
		{name: "언어 없이 + 확장자로 짐작", src: "```title=\"docker-compose.yml\"\na: 1\n```\n",
			want: []string{`data-title="docker-compose.yml"><code class="language-yaml"`}},
		{name: "줄임 + 언어 없이", src: "```:main.go\nx\n```\n",
			want: []string{`data-title="main.go"><code class="language-go"`}},
		{name: "언어 없이, 짐작하지 못하면 일반 코드", src: "```title=\"해피수요일\"\n글\n```\n",
			want: []string{`data-title="해피수요일"><code>글`}, not: []string{"language-"}},
		{name: "파일명 이스케이프", src: "```go title='a\"b<c.go'\nx\n```\n",
			want: []string{`data-title="a&quot;b&lt;c.go"`}},
		{name: "mermaid 그림에는 띠 없음", src: "```mermaid title=\"f.md\"\ngraph TD\nA-->B\n```\n",
			want: []string{`<figure class="diagram"`}, not: []string{"data-title"}},
		{name: "그리지 못한 mermaid 는 코드로 남고 띠가 있다", src: "```mermaid:f.md\nsequenceDiagram\n```\n",
			want: []string{`data-title="f.md"><code class="language-mermaid"`}, not: []string{`data-title="f.md"><code class="language-mermaidf.md"`}},
		{name: "mermaid 줄임 그림에도 띠 없음", src: "```mermaid:f.md\ngraph TD\nA-->B\n```\n",
			want: []string{`<figure class="diagram"`}, not: []string{"data-title"}},
		{name: "math 정식 수식에는 띠 없음", src: "```math title=\"e.tex\"\nE=mc^2\n```\n",
			want: []string{`<div class="math-display"`}, not: []string{"data-title"}},
		{name: "math 줄임 수식에도 띠 없음", src: "```math:e.tex\nE=mc^2\n```\n",
			want: []string{`<div class="math-display"`}, not: []string{"data-title"}},
		{name: "그리지 못한 math 는 코드로 남고 띠가 있다", src: "```math:e.tex\n\\frac{\n```\n",
			want: []string{`data-title="e.tex"><code class="language-math"`}},
		{name: "언어만 — data-title 없음", src: "```go\nx\n```\n", not: []string{"data-title"}},
		{name: "줄임 파일명이 비면 띠 없음", src: "```yaml:\na: 1\n```\n", not: []string{"data-title"}},
	}
	for _, c := range cases {
		r := render(t, c.src)
		for _, w := range c.want {
			if !strings.Contains(r.HTML, w) {
				t.Errorf("%s: 출력에 %q 없음:\n%s", c.name, w, r.HTML)
			}
		}
		for _, n := range c.not {
			if strings.Contains(r.HTML, n) {
				t.Errorf("%s: 출력에 %q 있음:\n%s", c.name, n, r.HTML)
			}
		}
	}
}

// 파일명은 코드 글자에 들어가지 않는다 — code 안 textContent 와 속성값을 갈라 본다.
func TestCodeTitleNotInCode(t *testing.T) {
	r := render(t, "```yaml:docker-compose.yml\na: 1\nb: 2\n```\n")
	i := strings.Index(r.HTML, "<code")
	j := strings.Index(r.HTML, "</code>")
	if i < 0 || j < 0 || j < i {
		t.Fatalf("코드 요소를 못 찾음:\n%s", r.HTML)
	}
	if strings.Contains(r.HTML[i:j], "docker-compose.yml") {
		t.Errorf("코드 글자에 파일명이 있다:\n%s", r.HTML[i:j])
	}
}

// 언어 짐작(chroma lexers.Match): 확장자·파일명으로 언어를 찾는다. .env 의 구체 언어는 chroma 버전을 따르므로
// "짐작된다"만 고정하고, yaml·go 매칭은 이름까지 고정한다.
func TestGuessCodeLang(t *testing.T) {
	if got := guessCodeLang("docker-compose.yml"); got != "yaml" {
		t.Errorf("docker-compose.yml → %q, 기대 yaml", got)
	}
	if got := guessCodeLang("main.go"); got != "go" {
		t.Errorf("main.go → %q, 기대 go", got)
	}
	if got := guessCodeLang(".env"); got == "" {
		t.Errorf(".env → 짐지 못함(짐작 성공을 기대)")
	}
	if got := guessCodeLang("확장자없는파일"); got != "" {
		t.Errorf("확장자없는파일 → %q, 기대 \"\"", got)
	}
}
