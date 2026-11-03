package mdoc

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// 입구 안내서(spec/markdown.md)·발표 안내서(slides.md)를
// 코드와 맞춘다(itda-hyve#134) — 경고 코드 표는 코드의 경고 상수와, frontmatter 키 표는 meta.go 가 읽는 키와, slides 키 표는 parseSlides 와
// 양방향으로. ```markdown 예제는 모두 그려 경고 0(경고 예제는 적은 코드 그대로). 상대 링크는 파일·제목이 있다.
// 코드에 경고·키를 더하고 안내서를 안 고치면 여기서 실패한다.

var entryGuides = []string{"markdown.md", "slides.md"}

func readDoc(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(guideDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// tableNames 는 표지 구간 안 표의 첫 칸 `이름` 들이다(머리 줄은 백틱이 없어 빠진다).
func tableNames(t *testing.T, g, mark string) []string {
	t.Helper()
	re := regexp.MustCompile("(?m)^\\| `([^`]+)` \\|")
	var out []string
	for _, m := range re.FindAllStringSubmatch(marked(t, g, mark), -1) {
		out = append(out, m[1])
	}
	return out
}

func sameSet(t *testing.T, what string, doc, code []string) {
	t.Helper()
	d, c := slices.Clone(doc), slices.Clone(code)
	sort.Strings(d)
	sort.Strings(c)
	if dup := slices.Compact(slices.Clone(d)); len(dup) != len(d) {
		t.Errorf("%s: 안내서 표에 같은 이름이 둘 %v", what, d)
	}
	for _, x := range c {
		if !slices.Contains(d, x) {
			t.Errorf("%s: 코드의 %q 가 안내서 표에 없음", what, x)
		}
	}
	for _, x := range d {
		if !slices.Contains(c, x) {
			t.Errorf("%s: 안내서 표의 %q 를 코드가 모름", what, x)
		}
	}
}

// pkgFiles 는 이 패키지의 시험 아닌 Go 파일을 읽는다. go test -overlay 로 바꿔 끼운 파일은
// ITDA_DOC_TEST_OVERLAY 가 가리키는 overlay JSON 의 Replace 를 따른다(컴파일과 같은 소스를 본다 —
// 뮤테이션 시험이 디스크의 옛 원문을 읽지 않게).
func pkgFiles(t *testing.T) []*ast.File {
	t.Helper()
	rep := overlayReplace(t)
	fset := token.NewFileSet()
	names, _ := filepath.Glob("*.go")
	var out []*ast.File
	for _, n := range names {
		if strings.HasSuffix(n, "_test.go") {
			continue
		}
		var src any // nil 이면 ParseFile 이 파일을 읽는다([]byte nil 을 넘기면 빈 소스로 본다)
		p, ok := rep[n]
		if !ok {
			if abs, err := filepath.Abs(n); err == nil {
				p, ok = rep[abs] // overlay JSON 은 절대 경로 키로 쓴다
			}
		}
		if ok {
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			src = b
		}
		f, err := parser.ParseFile(fset, n, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, f)
	}
	return out
}

// overlayReplace 는 go test -overlay 의 Replace 맵이다(환경변수로 overlay JSON 경로를 받는다). 비면 nil.
// 환경변수가 있는데 파일을 읽거나 풀지 못하면 죽인다 — 조용히 nil 을 돌려주면 컴파일 소스와 이 검사가
// 읽는 소스가 어긋나 뮤티이션이 잡히지 않는다(리뷰 R1 검증 범위).
func overlayReplace(t *testing.T) map[string]string {
	p := os.Getenv("ITDA_DOC_TEST_OVERLAY")
	if p == "" {
		return nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("ITDA_DOC_TEST_OVERLAY=%s 를 읽지 못함: %v", p, err)
	}
	var ov struct {
		Replace map[string]string `json:"Replace"`
	}
	if json.Unmarshal(b, &ov) != nil {
		t.Fatalf("ITDA_DOC_TEST_OVERLAY=%s 가 overlay JSON 이 아님", p)
	}
	return ov.Replace
}

// warnCodes 는 패키지의 Warn* 글자 상수 값이다 — 경고 상수를 어느 파일에 더해도 잡힌다.
func warnCodes(t *testing.T) []string {
	var out []string
	for _, f := range pkgFiles(t) {
		for _, d := range f.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.CONST {
				continue
			}
			for _, s := range g.Specs {
				vs := s.(*ast.ValueSpec)
				for i, n := range vs.Names {
					if !strings.HasPrefix(n.Name, "Warn") || i >= len(vs.Values) {
						continue
					}
					if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						v, _ := strconv.Unquote(lit.Value)
						out = append(out, v)
					}
				}
			}
		}
	}
	return out
}

// caseKeys 는 함수 fn 안에서 `switch x.Value` 의 case 글자들이다(frontmatter 키를 읽는 자리).
func caseKeys(t *testing.T, fn string) []string {
	t.Helper()
	var out []string
	for _, f := range pkgFiles(t) {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Name.Name != fn {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				sw, ok := n.(*ast.SwitchStmt)
				if !ok {
					return true
				}
				if sel, ok := sw.Tag.(*ast.SelectorExpr); !ok || sel.Sel.Name != "Value" {
					return true
				}
				for _, st := range sw.Body.List {
					for _, e := range st.(*ast.CaseClause).List {
						if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
							v, _ := strconv.Unquote(lit.Value)
							out = append(out, v)
						}
					}
				}
				return true
			})
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s 에서 키를 찾지 못함", fn)
	}
	return out
}

// literalCodes 는 dir 의 시험 아닌 Go 파일에서 경고 코드를 글자로 직접 쓴 자리다 — st.warn(줄, "코드", …) 와 Warning{Code: "코드"}.
// 코드는 Warn* 상수로만 써야 경고 코드 표 시험(상수 ↔ 안내서)이 새 코드를 놓치지 않는다(itda-hyve#134 리뷰 R1 P2-1).
func literalCodes(t *testing.T, dir string) []string {
	t.Helper()
	fset := token.NewFileSet()
	names, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	var out []string
	for _, n := range names {
		if strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, n, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(node ast.Node) bool {
			switch x := node.(type) {
			case *ast.CallExpr:
				if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "warn" && len(x.Args) >= 2 {
					if lit, ok := x.Args[1].(*ast.BasicLit); ok {
						out = append(out, fset.Position(lit.Pos()).String())
					}
				}
			case *ast.CompositeLit:
				typ := ""
				switch tt := x.Type.(type) {
				case *ast.Ident:
					typ = tt.Name
				case *ast.SelectorExpr:
					typ = tt.Sel.Name
				}
				if typ != "Warning" {
					return true
				}
				for _, e := range x.Elts {
					if kv, ok := e.(*ast.KeyValueExpr); ok {
						if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "Code" {
							if lit, ok := kv.Value.(*ast.BasicLit); ok {
								out = append(out, fset.Position(lit.Pos()).String())
							}
						}
					}
				}
			}
			return true
		})
	}
	return out
}

func TestWarnCodesAreConstants(t *testing.T) {
	for _, dir := range []string{".", filepath.Join("..", "docview")} {
		for _, at := range literalCodes(t, dir) {
			t.Errorf("%s: 경고 코드를 글자로 씀 — Warn* 상수로(안내서 경고 코드 표와 묶인다)", at)
		}
	}
}

func TestEntryGuideWarnCodes(t *testing.T) {
	// unknown-theme 은 문서 창(docview)이 테마를 고르며 더한다 — docview 의 시험이 그 상수를 이 표와 맞춘다
	sameSet(t, "경고 코드", tableNames(t, readDoc(t, "markdown.md"), "warn-codes"), append(warnCodes(t), "unknown-theme"))
}

func TestEntryGuideFrontmatterKeys(t *testing.T) {
	sameSet(t, "frontmatter 키", tableNames(t, readDoc(t, "markdown.md"), "fm-keys"), caseKeys(t, "fill"))
}

// 알림 표(itda-hyve#434 — 편집기 자동 완성 후보가 이 표에서 나온다): 표시·제목 두 가지가 렌더러의 알림 제목과 같다.
func TestEntryGuideAlerts(t *testing.T) {
	row := regexp.MustCompile("(?m)^\\| `([A-Z]+)` \\| ([^|]+) \\| ([^|]+) \\|")
	var doc, code []string
	for _, m := range row.FindAllStringSubmatch(marked(t, readDoc(t, "markdown.md"), "alerts"), -1) {
		k := strings.ToLower(m[1])
		doc = append(doc, k)
		if want, ok := alertTitles[k]; ok && (strings.TrimSpace(m[2]) != want[0] || strings.TrimSpace(m[3]) != want[1]) {
			t.Errorf("알림 %s 의 제목 %q·%q, 렌더러는 %v", m[1], m[2], m[3], want)
		}
		if !alertRe.MatchString("[!" + m[1] + "]") {
			t.Errorf("알림 %s 를 알림 표시로 읽지 않음", m[1])
		}
	}
	for k := range alertTitles {
		code = append(code, k)
	}
	sameSet(t, "알림", doc, code)
}

// 발표 안내서의 slides 키 표: 첫 칸 키, 둘째 칸 별칭(`a`·`b` 또는 —)
func TestSlidesGuideKeys(t *testing.T) {
	row := regexp.MustCompile("(?m)^\\| `([^`]+)` \\| ([^|]+) \\|")
	alias := regexp.MustCompile("`([^`]+)`")
	var doc []string
	for _, m := range row.FindAllStringSubmatch(marked(t, readDoc(t, "slides.md"), "slides-keys"), -1) {
		doc = append(doc, m[1])
		for _, a := range alias.FindAllStringSubmatch(m[2], -1) {
			doc = append(doc, a[1])
		}
	}
	sameSet(t, "slides 키·별칭", doc, caseKeys(t, "parseSlides"))
}

// mdExample 은 ```markdown 예제 하나다. warn 이 있으면 정확히 그 경고 코드들만 난다(없으면 경고 0).
type mdExample struct {
	line int
	src  string
	warn []string
}

// mdExamples 는 안내서의 ```markdown(또는 ````markdown) 블록 전부다. 바로 앞 줄의 <!-- md-example warn="a|b" --> 가 경고 예제를 표시한다.
func mdExamples(t *testing.T, g string) []mdExample {
	t.Helper()
	lines := strings.Split(g, "\n")
	warnRe := regexp.MustCompile(`^<!-- md-example warn="([^"]+)" -->$`)
	var out []mdExample
	for i := 0; i < len(lines); i++ {
		fence, ok := strings.CutSuffix(lines[i], "markdown")
		if !ok || (fence != "```" && fence != "````") {
			continue
		}
		ex := mdExample{line: i + 1}
		if i > 0 {
			if m := warnRe.FindStringSubmatch(lines[i-1]); m != nil {
				ex.warn = strings.Split(m[1], "|")
			}
		}
		var body []string
		j := i + 1
		for ; j < len(lines) && lines[j] != fence; j++ {
			body = append(body, lines[j])
		}
		if j == len(lines) {
			t.Fatalf("%d줄의 예제가 닫히지 않음", i+1)
		}
		ex.src = strings.Join(body, "\n") + "\n"
		out = append(out, ex)
		i = j
	}
	if n := strings.Count(g, "<!-- md-example"); n != countWarnExamples(out) {
		t.Fatalf("md-example 표지 %d 개 중 %d 개만 ```markdown 블록이 바로 뒤따름", n, countWarnExamples(out))
	}
	return out
}

func countWarnExamples(exs []mdExample) int {
	n := 0
	for _, e := range exs {
		if e.warn != nil {
			n++
		}
	}
	return n
}

func TestEntryGuideExamples(t *testing.T) {
	codes := warnCodes(t)
	total := 0
	for _, name := range entryGuides {
		exs := mdExamples(t, readDoc(t, name))
		total += len(exs)
		for _, ex := range exs {
			res, err := Render([]byte(ex.src), Options{})
			if err != nil {
				t.Fatalf("%s:%d: %v", name, ex.line, err)
			}
			var got []string
			for _, w := range res.Warnings {
				got = append(got, w.Code)
			}
			sort.Strings(got)
			got = slices.Compact(got) // 같은 원인이 여는·닫는 태그처럼 여러 번 날 수 있다 — 코드의 집합을 견준다
			want := slices.Clone(ex.warn)
			sort.Strings(want)
			for _, c := range want {
				if !slices.Contains(codes, c) {
					t.Errorf("%s:%d: 경고 예제의 %q 는 경고 코드가 아님", name, ex.line, c)
				}
			}
			if !slices.Equal(got, want) {
				t.Errorf("%s:%d: 경고 %v, 기대 %v %v\n%s", name, ex.line, got, want, res.Warnings, ex.src)
			}
		}
	}
	if total < 15 {
		t.Errorf("입구·발표 안내서의 예제가 %d 개뿐", total)
	}
}

// 새 안내서 셋의 상대 링크는 파일이 있고 제목 조각이 맞다. 입구에서 주제별 안내서 모두로 가는 길이 있다.
func TestEntryGuideLinks(t *testing.T) {
	link := regexp.MustCompile(`\[[^\]]*\]\(([^)\s]+)\)`)
	for _, name := range entryGuides {
		g := readDoc(t, name)
		prose := regexp.MustCompile("(?s)````.*?````").ReplaceAllString(g, "")
		prose = regexp.MustCompile("(?s)```.*?```").ReplaceAllString(prose, "")
		prose = regexp.MustCompile("`[^`\n]*`").ReplaceAllString(prose, "")
		for _, m := range link.FindAllStringSubmatch(prose, -1) {
			target := m[1]
			if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			path, frag, _ := strings.Cut(target, "#")
			doc := g
			if path != "" {
				b, err := os.ReadFile(filepath.Join(guideDir, filepath.FromSlash(path)))
				if err != nil {
					t.Errorf("%s 의 링크 %s: %v", name, target, err)
					continue
				}
				doc = string(b)
			}
			if frag != "" && !slices.Contains(headingSlugs(dropFences(doc)), frag) {
				t.Errorf("%s 의 링크 %s: 그 제목이 없음", name, target)
			}
		}
	}
	// 설계·기능·README 에서 새 안내서로 가는 제목 조각 링크도 맞아야 한다
	into := regexp.MustCompile(`\]\((?:spec/)?((?:markdown|slides)\.md)#([^)\s]+)\)`)
	for _, f := range []string{"../README.md"} {
		for _, m := range into.FindAllStringSubmatch(readDoc(t, f), -1) {
			if !slices.Contains(headingSlugs(dropFences(readDoc(t, m[1]))), m[2]) {
				t.Errorf("%s 의 링크 %s#%s: 그 제목이 없음", f, m[1], m[2])
			}
		}
	}
	entry := readDoc(t, "markdown.md")
	for _, f := range []string{"charts.md", "math.md", "diagrams.md",
		"slides.md", "themes.md"} {
		if !strings.Contains(entry, "("+f+")") {
			t.Errorf("입구 안내서에 %s 링크가 없음", f)
		}
	}
}

// dropFences 는 줄 머리 펜스로 연 코드 블록을 뺀다. headingSlugs 의 "```…```" 정규식은 글 안의 ```` ```mermaid ```` 와 짝을 지어
// 사이의 제목을 삼키므로 그 앞에서 줄 단위로 먼저 지운다.
func dropFences(md string) string {
	var b strings.Builder
	fence := ""
	for line := range strings.SplitSeq(md, "\n") {
		t := strings.TrimLeft(line, " ")
		if fence == "" && (strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")) {
			fence = t[:len(t)-len(strings.TrimLeft(t, t[:1]))]
			continue
		}
		if fence != "" {
			if strings.TrimSpace(t) != "" && strings.Trim(strings.TrimSpace(t), fence[:1]) == "" && len(strings.TrimSpace(t)) >= len(fence) {
				fence = ""
			}
			continue
		}
		line = strings.ReplaceAll(line, "```", "")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}
