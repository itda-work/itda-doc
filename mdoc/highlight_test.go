package mdoc

import (
	"github.com/yuin/goldmark/ast"
	"hash/fnv"
	"html"
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"
)

var highlightTag = regexp.MustCompile(`<[^>]*>`)

func TestHighlightLanguagesAndText(t *testing.T) {
	samples := map[string]string{
		"go": `fmt.Println("<script>&")`, "python": `print("<script>&")`, "py": `print("<script>&")`,
		"js": `const x = "<script>&";`, "ts": `const x: string = "<script>&";`,
		"bash": `echo '<script>&'`, "sh": `echo '<script>&'`, "json": `{"x":"<script>&"}`,
		"yaml": `x: "<script>&"`, "yml": `x: "<script>&"`, "sql": `SELECT '<script>&';`,
		"diff": "+<script>&", "powershell": `$x = '<script>&'`, "ps1": `$x = '<script>&'`,
	}
	for lang, code := range samples {
		t.Run(lang, func(t *testing.T) {
			got := render(t, "```"+lang+"\n"+code+"\n```\n").HTML
			if !strings.Contains(got, `class="hl-`) {
				t.Fatalf("토큰 span 없음: %s", got)
			}
			if strings.Contains(got, "<script>") || strings.Contains(got, " style=") {
				t.Fatalf("안전하지 않은 HTML: %s", got)
			}
			start := strings.Index(got, "><code")
			if start < 0 {
				t.Fatal(got)
			}
			start = strings.Index(got[start:], ">") + start + 1
			end := strings.Index(got[start:], "</code>") + start
			plain := html.UnescapeString(highlightTag.ReplaceAllString(got[start:end], ""))
			if plain != code+"\n" {
				t.Fatalf("글자 변경: %q", plain)
			}
		})
	}
}

func TestEditorFenceRangesFollowGoldmarkAST(t *testing.T) {
	var sources []string
	for indent := 0; indent <= 4; indent++ {
		for _, mark := range []string{"```go", "~~~~go", "````go"} {
			pad := strings.Repeat(" ", indent)
			sources = append(sources, pad+mark+"\n"+pad+"x\n"+pad+mark[:strings.Index(mark, "go")]+"\n# outside\n")
		}
	}
	sources = append(sources,
		"  ```go\n\tvar x = 1\n  ```\n", "> ```go\n> x\n> ```\n",
		"- item\n\n    ```go\n    x\n    ```\n", "- ```\n  x\n  ```\n",
		":::box\n```go\nx\n```\n:::\n", "> - item\n>\n>   ~~~go\n>   x\n>   ~~~\n",
		"```go\nx", "```go\r\nx\r\n```\r\n", "```go`bad\nx\n```\n",
		"> ```go\n> var x = 1\n```\nplain\n```\n",
		"> ```go\n> x\nplain\n```go\ny\n```\n",
		"- ```go\n  x\noutside\n```go\ny\n```\n",
		":::box\n> ```go\n> x\n::: \n```go\ny\n```\n",
	)
	for i, source := range sources {
		src := normalize([]byte(source))
		meta, body, _, _ := parseFrontmatter(src, nil)
		st := newState(body, meta, Options{})
		doc, over := st.parseGuarded(body)
		if over != 0 {
			t.Fatalf("%d: 깊이 초과", i)
		}
		var want []struct {
			line    int
			content []int
			close   int
		}
		_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}
			if fc, ok := n.(*ast.FencedCodeBlock); ok {
				item := struct {
					line    int
					content []int
					close   int
				}{line: st.lines.line(fc.Pos()), close: st.fenceCloses[fc]}
				for j := 0; j < fc.Lines().Len(); j++ {
					item.content = append(item.content, st.lines.line(fc.Lines().At(j).Start))
				}
				want = append(want, item)
			}
			return ast.WalkContinue, nil
		})
		r := render(t, source)
		if len(r.EditorTokens) != len(want) {
			t.Fatalf("%d: AST %v, 편집기 %+v", i, want, r.EditorTokens)
		}
		for j, b := range r.EditorTokens {
			if b.Kind != "fence" || b.Line != want[j].line || len(b.Content) != len(want[j].content) {
				t.Fatalf("%d: AST %v, 범위 %+v", i, want[j], b)
			}
			last := want[j].line
			if len(want[j].content) > 0 {
				last = want[j].content[len(want[j].content)-1]
			}
			end := last
			if want[j].close > 0 {
				end = want[j].close
			}
			if b.Close != want[j].close || b.End != end {
				t.Fatalf("%d: 파서 닫는 줄 %d, 범위 %+v", i, want[j].close, b)
			}
			if j > 0 && r.EditorTokens[j-1].End >= b.Line {
				t.Fatalf("%d: 펜스 범위 겹침: %+v", i, r.EditorTokens)
			}
			for k, c := range b.Content {
				if c.Line != want[j].content[k] {
					t.Fatalf("%d: 내용 줄 %+v, AST %v", i, b, want[j])
				}
			}
		}
	}
}

func TestEditorImplicitFenceEndAllowsNextFence(t *testing.T) {
	for _, tc := range []struct {
		source              string
		lines, ends, closes []int
	}{
		{"> ```go\n> var x = 1\n```\nplain\n```\n", []int{1, 3}, []int{2, 5}, []int{0, 5}},
		{"> ```go\n> x\nplain\n```go\ny\n```\n", []int{1, 4}, []int{2, 6}, []int{0, 6}},
		{"- ```go\n  x\noutside\n```go\ny\n```\n", []int{1, 4}, []int{2, 6}, []int{0, 6}},
	} {
		r := render(t, tc.source)
		if len(r.EditorTokens) != len(tc.lines) {
			t.Fatalf("블록 수: %+v", r.EditorTokens)
		}
		for i, b := range r.EditorTokens {
			if b.Line != tc.lines[i] || b.End != tc.ends[i] || b.Close != tc.closes[i] {
				t.Fatalf("블록 %d: %+v", i, b)
			}
		}
	}
}

func TestHighlightFallbackAndLimits(t *testing.T) {
	for _, lang := range []string{"", "unknownlanguage123", "math", "mermaid"} {
		src := "```" + lang + "\nplain <script>&\n```\n"
		got := render(t, src).HTML
		want := `<pre data-line="1"><code`
		if lang != "" {
			want += ` class="language-` + lang + `"`
		}
		want += ">plain &lt;script&gt;&amp;\n</code></pre>\n"
		if got != want {
			t.Fatalf("%s 기존 출력 변경: got %q want %q", lang, got, want)
		}
	}
	for _, code := range []string{strings.Repeat("x", maxHighlightBlockBytes+1), strings.Repeat("x\n", maxHighlightBlockLines+1)} {
		got := render(t, "```go\n"+code+"\n```\n").HTML
		if strings.Contains(got, `class="hl-`) {
			t.Fatal("상한 초과 강조됨")
		}
	}
	var doc strings.Builder
	for i := 0; i < 5; i++ {
		doc.WriteString("```go\n")
		doc.WriteString(strings.Repeat("x", maxHighlightBlockBytes-2))
		doc.WriteString("\n```\n\n")
	}
	got := render(t, doc.String()).HTML
	if strings.Count(got, `class="hl-`) == 0 || strings.Count(got, `<code class="language-go">`) != 5 {
		t.Fatal("문서 예산 적용 오류")
	}
	if strings.Count(got, `<code class="language-go"><span`) >= 5 {
		t.Fatal("문서 예산 초과 강조됨")
	}
}

func BenchmarkHighlightDocument(b *testing.B) {
	var doc strings.Builder
	for i := 0; i < 40; i++ {
		doc.WriteString("```go\nfunc main() { println(\"hello\") }\n```\n\n")
	}
	src := []byte(doc.String())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Render(src, Options{}); err != nil {
			b.Fatal(err)
		}
	}
}

func TestEditorTokensMatchCode(t *testing.T) {
	code := "fmt.Println(\"한글 😀 & <x>\")\n"
	r := render(t, "```go\n"+code+"```\n")
	if len(r.EditorTokens) != 1 || r.EditorTokens[0].Line != 1 {
		t.Fatalf("블록 위치: %+v", r.EditorTokens)
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte("```go\n" + code + "```\n"))
	if r.EditorTokens[0].Hash != h.Sum32() {
		t.Fatal("원문 해시 불일치")
	}
	n := 0
	for _, tok := range r.EditorTokens[0].Tokens {
		n += tok.Length
	}
	if n != len(utf16.Encode([]rune(code))) {
		t.Fatalf("UTF-16 토큰 길이 %d", n)
	}
}

func TestEditorTokenSourceLocations(t *testing.T) {
	for _, tc := range []struct {
		name, source          string
		line, end, close, col int
	}{
		{"indent", "  ```go\n  var x = 1\n  ```\n", 1, 3, 3, 2},
		{"quote", "> ```go\n> var x = 1\n> ```\n", 1, 3, 3, 2},
		{"list", "- item\n\n    ```go\n    var x = 1\n    ```\n", 3, 5, 5, 4},
		{"open", "```go\nvar x = 1", 1, 2, 0, 0},
		{"horizontal rule", "---\n# Heading\n```go\nvar x = 1\n```\n", 3, 5, 5, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := render(t, tc.source)
			if len(r.EditorTokens) != 1 {
				t.Fatalf("토큰 블록: %+v", r.EditorTokens)
			}
			b := r.EditorTokens[0]
			if b.Line != tc.line || b.End != tc.end || b.Close != tc.close || len(b.Content) != 1 || b.Content[0].Col != tc.col {
				t.Fatalf("원문 위치: %+v", b)
			}
			starts := strings.Split(tc.source, "\n")
			span := strings.Join(starts[b.Line-1:b.End], "\n")
			if b.End < len(starts) {
				span += "\n"
			}
			h := fnv.New32a()
			_, _ = h.Write([]byte(span))
			if b.Hash != h.Sum32() {
				t.Fatalf("원문 해시: %+v", b)
			}
		})
	}
	r := render(t, "```go`bad\nvar x = 1\n```\n")
	if len(r.EditorTokens) != 1 || r.EditorTokens[0].Line != 3 || len(r.EditorTokens[0].Tokens) != 0 {
		t.Fatalf("금지된 info 문자열 뒤의 새 펜스: %+v", r.EditorTokens)
	}
}

func TestEditorAllFenceRangesAndRawTokens(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		lines        []int
		colored      bool
	}{
		{"plain", "- ```\n  plain\n  ```\n# Outside\n", []int{1}, false},
		{"unknown", "~~~unknownlanguage123\nx\n~~~", []int{1}, false},
		{"math", "```math\nx\n```", []int{1}, false},
		{"mermaid", "```mermaid\nx\n```", []int{1}, false},
		{"indent code", "    ```go\nx\n", nil, false},
		{"tab padding", "  ```go\n\tvar x = 1\n  ```\n", []int{1}, true},
		{"open EOF", "```go\nvar x = 1", []int{1}, true},
		{"nested", "> - item\n>\n>   ~~~~go\n>   var x = 1\n>   ~~~~\n", []int{3}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := render(t, tc.source)
			if len(r.EditorTokens) != len(tc.lines) {
				t.Fatalf("범위: %+v", r.EditorTokens)
			}
			for i, line := range tc.lines {
				b := r.EditorTokens[i]
				if b.Kind != "fence" || b.Line != line || (len(b.Tokens) > 0) != tc.colored {
					t.Fatalf("범위·색: %+v", b)
				}
				if tc.colored {
					rows := strings.Split(tc.source, "\n")
					var n int
					for _, c := range b.Content {
						n += len(utf16.Encode([]rune(rows[c.Line-1][c.Col:])))
						if c.Line < len(rows) {
							n++
						}
					}
					var got int
					for _, tok := range b.Tokens {
						got += tok.Length
					}
					if got != n {
						t.Fatalf("원문 토큰 길이 %d, 원문 %d: %+v", got, n, b)
					}
				}
			}
		})
	}
}

func TestEditorFrontmatterRange(t *testing.T) {
	for _, tc := range []struct {
		source string
		front  bool
		end    int
	}{
		{"---\ntitle: A\n...\n# Heading\n", true, 3},
		{"---\ntitle: A\n...", true, 3},
		{"---\n# Heading\n", false, 0},
		{"---\nbroken: [\n---\n", false, 0},
	} {
		r := render(t, tc.source)
		var front *EditorCodeBlock
		for i := range r.EditorTokens {
			if r.EditorTokens[i].Kind == "front" {
				front = &r.EditorTokens[i]
			}
		}
		if (front != nil) != tc.front {
			t.Fatalf("머리말 %q: %+v", tc.source, r.EditorTokens)
		}
		if front != nil && (front.Line != 1 || front.End != tc.end) {
			t.Fatalf("머리말 줄: %+v", front)
		}
	}
}
