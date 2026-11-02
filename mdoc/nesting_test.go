package mdoc

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"golang.org/x/net/html"
)

// 블록 중첩 상한(nesting.go — 실제 파서가 65번째 겹을 받아들이면 파싱을 멈추고 통째 안전 모드)의 받아들임 시험.
//   - 판정 = "상한 없는 파서로 잰 실제 AST 최대 깊이 > 64"(realDepth — 이 기준은 시험 안에서만). 깊이 64 는 보통, 65 는 안전 모드
//   - 깊이 64 이하 문서는 상한 없는 렌더와 Result 전체가 같다(CommonMark·무작위·현실 문서·리뷰 재현·문법 동등 변형)
//   - 경고 줄은 65번째 겹을 연 줄, 중단 뒤 다음 렌더는 정상(연속·병렬), 다른 panic 은 그대로 전파
//   - 안전 모드는 원문 글자를 모두 보이고 줄 번호를 붙인다. 병적 입력은 모두 선형 시간이다(limits_test.go 도)

// realDepth 는 src 를 상한 없이 파싱하는 동안 실제로 받아들여진 컨테이너 블록의 가장 깊은 겹 수와, 65번째 겹인 블록 가운데 가장 먼저
// 연 것의 줄(없으면 0)이다 — 판정의 정답. 제품 코드(nestingGate)와 따로, 모든 블록 파서의 Close 를 가로채(원래 Close 보다 먼저 —
// 각주 정의는 Close 에서 문서 끝 각주 목록으로 옮겨진다) 그 블록과 조상을 종류 이름으로 센다. 받아들여진 블록은 모두 닫히고, 받아들여지지
// 않은 후보(RequireParagraph 뒤 다시 고른 것)는 닫히지 않는다.
func realDepth(src string) (depth, overLine int) {
	meta, body, _, _ := parseFrontmatter(normalize([]byte(src)), nil)
	st := newState(body, meta, Options{})
	st.nestingLimit = 1 << 30
	rec := &depthRecorder{firstPos: -1}
	st.md.Parser().AddOptions(rec)
	st.md.Parser().Parse(textReader(body))
	if rec.firstPos >= 0 {
		overLine = st.lines.line(rec.firstPos)
	}
	return rec.max, overLine
}

type depthRecorder struct{ max, firstPos int }

func (r *depthRecorder) SetParserOption(c *parser.Config) {
	for i, v := range c.BlockParsers {
		c.BlockParsers[i].Value = closeRecorder{v.Value.(parser.BlockParser), r}
	}
}

type closeRecorder struct {
	parser.BlockParser
	r *depthRecorder
}

func (c closeRecorder) Close(node ast.Node, reader text.Reader, pc parser.Context) {
	d := 0
	for n := node; n != nil; n = n.Parent() {
		switch n.Kind().String() {
		case "Blockquote", "List", "Container", "Footnote", "DefinitionList":
			d++
		}
	}
	c.r.max = max(c.r.max, d)
	if k := node.Kind().String(); d == MaxNestingDepth+1 && (k == "Blockquote" || k == "List" || k == "Container" || k == "Footnote") &&
		(c.r.firstPos < 0 || node.Pos() < c.r.firstPos) {
		c.r.firstPos = node.Pos()
	}
	if node.Kind().String() == "DefinitionDescription" && d == MaxNestingDepth+1 && (c.r.firstPos < 0 || node.Pos() < c.r.firstPos) {
		c.r.firstPos = node.Pos() // 정의 목록 겹은 그 안의 정의 설명을 연 줄
	}
	c.BlockParser.Close(node, reader, pc)
}

func renderUnlimited(t *testing.T, src string) Result {
	t.Helper()
	r, err := renderLimit([]byte(src), Options{}, 1<<30)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func isRaw(r Result) bool { return strings.HasPrefix(r.HTML, `<pre class="raw-source"`) }

// assertDecision 은 판정이 실제 깊이와 같은지 본다: 깊이 64 이하면 상한 없는 렌더와 Result 전체가 같고, 넘으면 안전 모드이며 경고
// too-complex 는 65번째 겹을 연 줄에 하나다.
func assertDecision(t *testing.T, name, src string) (raw bool) {
	t.Helper()
	real, overLine := realDepth(src)
	r := render(t, src)
	raw = isRaw(r)
	switch {
	case raw != (real > MaxNestingDepth):
		t.Errorf("%s: 실제 깊이 %d 인데 안전 모드 %v\n원문 %q", name, real, raw, shortSrc(src))
	case !raw:
		if u := renderUnlimited(t, src); !reflect.DeepEqual(r, u) {
			t.Errorf("%s: 상한 없는 렌더와 다름", name)
		}
	default:
		w := r.Warnings[len(r.Warnings)-1]
		if w.Code != WarnTooComplex || w.Line != overLine {
			t.Errorf("%s: 경고 %+v, 기대 줄 %d", name, w, overLine)
		}
	}
	return raw
}

// 깊이 정확히 64 는 보통 렌더, 65 는 안전 모드 — 컨테이너 종류마다, 섞어서.
func TestNestingBoundary(t *testing.T) {
	for name, gen := range map[string]func(n int) string{
		"인용 한 줄":    func(n int) string { return strings.Repeat(">", n) + " 끝\n" },
		"인용 여러 줄":   func(n int) string { return strings.Repeat(strings.Repeat("> ", n)+"줄\n", 3) },
		"번호 목록":     func(n int) string { return strings.Repeat("1. ", n) + "끝\n" },
		"들여쓴 목록":    func(n int) string { return nestedIndentList(n) },
		"컨테이너":      func(n int) string { return strings.Repeat(":::box\n", n) + "안\n" + strings.Repeat(":::\n", n) },
		"인용 안 컨테이너": func(n int) string { return strings.Repeat("> :::box\n", n-1) + "> 안\n" },
		"각주 정의":     func(n int) string { return "본문[^a]\n\n" + strings.Repeat("[^a]: ", n) + "끝\n" },
		"정의 목록":     func(n int) string { return defListNest(n) },
		"인용·목록 섞음":  func(n int) string { return strings.Repeat("> - ", n/2) + strings.Repeat("> ", n%2) + "끝\n" },
	} {
		for _, n := range []int{MaxNestingDepth - 1, MaxNestingDepth, MaxNestingDepth + 1, MaxNestingDepth + 5} {
			src := gen(n)
			if d, _ := realDepth(src); d != n {
				t.Fatalf("%s %d: 시험 입력의 실제 깊이 %d", name, n, d)
			}
			if raw := assertDecision(t, fmt.Sprintf("%s %d", name, n), src); raw != (n > MaxNestingDepth) {
				t.Errorf("%s %d: 안전 모드 %v", name, n, raw)
			}
		}
	}
}

func nestedIndentList(n int) string {
	var b strings.Builder
	for k := range n {
		b.WriteString(strings.Repeat("  ", k) + "- 단계\n")
	}
	return b.String()
}

// defListNest 는 정의 목록 n 겹이다(정의 설명 안에 용어·정의를 들여 쓴다).
func defListNest(n int) string {
	var b strings.Builder
	for k := range n {
		ind := strings.Repeat("  ", k)
		b.WriteString(ind + "용어\n" + ind + ": 설명\n\n")
	}
	return b.String()
}

// 문법 동등 변형(리뷰 #126 R5): 인용 뒤 공백 유무, 탭 들여쓰기·탭 목록 표지, 닫는 줄의 공백 — 판정이 각 꼴의 실제 깊이와 같다.
// 얕은 문서(형제 콜아웃 64~200개)는 모두 보통 렌더다.
func TestNestingSyntaxVariants(t *testing.T) {
	shallow := map[string]string{
		"인용 닫는 줄 공백 생략":  "> :::note\n> 안내\n>:::\n>\n",
		"인용 모두 공백 생략":    ">:::note\n>안내\n>:::\n>\n",
		"인용 뒤 탭":         ">\t:::note\n>\t안내\n>\t:::\n>\n",
		"탭 목록":           "-\t:::note\n\t안내\n\t:::\n\n",
		"탭 번호 목록":        "1.\t:::note\n\t안내\n\t:::\n\n",
		"목록 항목마다 콜아웃":    "- :::note\n  안내\n  :::\n\n",
		"인용 안 콜아웃":       "> :::note\n> 안내\n> :::\n>\n",
		"인용 안 목록 안 콜아웃":  "> - :::tip\n>   안내\n>   :::\n",
		"들여쓴 콜아웃 0열 닫기":  "  :::note\n  안내\n:::\n\n",
		"탭 들여쓴 콜아웃 안 코드": "- a\n\n\t:::note\n\t```\n\t:::\n\t```\n\t:::\n\n",
		"정의 목록 콜아웃":      "용어\n: :::note\n  안내\n  :::\n\n",
		"번호 목록 콜아웃과 코드":  "1. 단계\n\n   :::note\n   안내\n\n   ```sh\n   x\n   ```\n   :::\n",
	}
	for name, unit := range shallow {
		for _, n := range []int{64, 65, 200} {
			src := strings.Repeat(unit, n)
			if assertDecision(t, fmt.Sprintf("%s ×%d", name, n), src) {
				t.Errorf("%s ×%d: 얕은 문서가 안전 모드", name, n)
			}
		}
	}
	// 같은 문서의 동등한 꼴들: 깊은 것도 얕은 것도 판정이 각자의 실제 깊이와 같다
	variants := []func(string) string{
		func(s string) string { return s },
		func(s string) string { return strings.ReplaceAll(s, "> ", ">") },
		func(s string) string { return strings.ReplaceAll(s, "> ", ">\t") },
		func(s string) string { return regexp.MustCompile(`(?m)^    `).ReplaceAllString(s, "\t") },
		func(s string) string { return strings.ReplaceAll(s, "- ", "-\t") },
	}
	rng := rand.New(rand.NewSource(1265))
	for i := range 400 {
		g := &realDocGen{rng: rng, tight: i%2 == 1}
		g.blocks(5+rng.Intn(30), 0, "", "")
		src := g.b.String()
		if i%4 == 0 { // 깊은 조각을 섞는다
			src += strings.Repeat([]string{"> ", "- ", "> :::box\n", "  - "}[rng.Intn(4)], 60+rng.Intn(10)) + "끝\n"
		}
		for k, v := range variants {
			assertDecision(t, fmt.Sprintf("#%d 꼴 %d", i, k), v(src))
		}
		if t.Failed() {
			t.Fatalf("#%d 원문 %q", i, src)
		}
	}
}

// RequireParagraph 후보 폐기 경계(리뷰 #126 R6): 64겹 ::: 안에서 정의 목록 후보가 앞 문단의 표 변환으로 버려지면 그 겹은 받아들여지지
// 않아 보통 렌더다. setext 후보가 표 변환으로 버려진 뒤 다시 고른 실제 목록이 65번째 겹이면 안전 모드다.
func TestNestingRequireParagraphDiscard(t *testing.T) {
	deep := strings.Repeat(":::note\n", MaxNestingDepth)
	if raw := assertDecision(t, "정의 목록 후보 폐기", deep+"a | b\n---|---\n: 설명\n"); raw {
		t.Errorf("정의 목록 후보 폐기: 안전 모드")
	}
	if raw := assertDecision(t, "setext 후보 폐기 뒤 목록", deep+"0\n-:\n-\n"); !raw {
		t.Errorf("setext 후보 폐기 뒤 목록: 보통 렌더")
	}
}

// 중단은 그 Render 호출 하나만 버린다 — 연속 호출·병렬 호출에서 경고·메타가 새지 않는다.
func TestNestingAbortIsolation(t *testing.T) {
	deep := "---\ntitle: 깊음\n---\n[x](javascript:alert(1))\n\n" + strings.Repeat(":::box\n", MaxNestingDepth+1)
	shallow := "# 제목\n\n본문 **굵게**\n"
	want := render(t, shallow)
	for range 3 {
		if r := render(t, deep); !isRaw(r) || r.Meta.Title != "깊음" || len(r.Warnings) != 1 {
			t.Fatalf("깊은 문서: %+v", r.Warnings)
		}
		if r := render(t, shallow); !reflect.DeepEqual(r, want) {
			t.Fatalf("중단 뒤 렌더가 다름: %+v", r)
		}
	}
	var wg sync.WaitGroup
	for i := range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			src := shallow
			if i%2 == 0 {
				src = deep
			}
			r, err := Render([]byte(src), Options{})
			if err != nil || isRaw(r) != (i%2 == 0) || (i%2 == 1 && !reflect.DeepEqual(r, want)) {
				t.Errorf("병렬 #%d: %v %+v", i, err, r.Warnings)
			}
		}()
	}
	wg.Wait()
}

// 중단 신호가 아닌 panic 은 파싱 경계에서 삼키지 않는다.
func TestNestingOtherPanic(t *testing.T) {
	st := newState([]byte("x"), Meta{}, Options{})
	st.md.Parser().AddOptions(panicGate{})
	defer func() {
		if r := recover(); r != "다른 panic" {
			t.Errorf("다른 panic 이 전파되지 않음: %v", r)
		}
	}()
	st.parseGuarded([]byte("x\n"))
	t.Error("panic 없이 돌아옴")
}

type panicGate struct{}

func (panicGate) SetParserOption(c *parser.Config) {
	for i, v := range c.BlockParsers {
		c.BlockParsers[i].Value = panicParser{v.Value.(parser.BlockParser)}
	}
}

type panicParser struct{ parser.BlockParser }

func (panicParser) Open(ast.Node, text.Reader, parser.Context) (ast.Node, parser.State) {
	panic("다른 panic")
}

// goldmark 계약: 감싼 블록 파서는 인용·목록·각주 정의·정의 설명·::: 다섯이고, 깊이를 세는 파서 가운데 RequireParagraph 를 돌려줄 수
// 있는 것은 정의 목록뿐이다(그래서 정의 설명에서 센다). 판을 올리면 이 시험이 실패한다 — nesting.go 머리말의 근거를 다시 확인할 것.
func TestNestingGoldmarkContract(t *testing.T) {
	dir := goldmarkDir(t)
	if !strings.HasSuffix(dir, "@v1.8.6") {
		t.Fatalf("goldmark 판이 바뀜(%s) — nesting.go 의 근거를 다시 확인할 것", filepath.Base(dir))
	}
	st := newState(nil, Meta{}, Options{})
	st.md.Parser().Parse(text.NewReader([]byte("x")))
	if st.nestingGated != 5 {
		t.Errorf("상한을 건 블록 파서 %d 개, 기대 5", st.nestingGated)
	}
	var users []string
	for _, f := range []string{"parser/blockquote.go", "parser/list.go", "extension/footnote.go", "extension/definition_list.go", "parser/setext_headings.go"} {
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "RequireParagraph") {
			users = append(users, f)
		}
	}
	if !reflect.DeepEqual(users, []string{"extension/definition_list.go", "parser/setext_headings.go"}) {
		t.Errorf("RequireParagraph 를 쓰는 파일 %v", users)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "parser", "parser.go"))
	if !strings.Contains(string(b), "if state&RequireParagraph != 0 {") || strings.Count(string(b), "goto retry") != 3 {
		t.Errorf("parser.go openBlocks 의 받아들임 길이 바뀜 — 다시 확인할 것")
	}
	if b, _ := os.ReadFile("container.go"); strings.Contains(string(b), "RequireParagraph") {
		t.Errorf("::: 파서가 RequireParagraph 를 씀")
	}
}

// 상한 아래의 보통 문서는 안전 모드로 가지 않고 상한 없는 렌더와 같다(리뷰 #126 R4 재현 포함).
func TestNestingNormalDocsNotFlagged(t *testing.T) {
	rep := strings.Repeat
	docs := map[string]string{
		"콜아웃 200개":           rep(":::note\n글\n\n```js\nx\n```\n\n- a\n- b\n:::\n\n", 200),
		"탭 안 탭 100개":         rep(":::tabs\n:::tab\n## 제목\n```\n:::\n```\n:::\n:::tab\n> 인용\n\n:::\n:::\n\n", 100),
		"목록 안 콜아웃 뒤 제목 100개": rep("1. 단계\n\n   :::note\n   글\n   :::\n\n## 다음\n\n", 100),
		"인용 안 콜아웃 100개":      rep("> :::note\n> 글\n> :::\n\n", 100),
		"코드 안 콜아웃 문법 30개":    rep(":::note\n```markdown\n:::tip\n안\n:::\n```\n:::\n\n# 절\n\n", 30),
		"깊은 들여쓰기 코드":         "- a\n\n```json\n" + rep(rep(" ", 140)+"{\n", 50) + "```\n",
		"제목·표·가로줄":           rep("# 제목\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n---\n\n", 200),
		"날짜 문단":              rep("2024. 10. 14. 오늘\n", 100),
		// 리뷰 #126 R4: 목록 항목·인용마다 안내 상자(형제) — 실제 깊이 2, 들여쓴 콜아웃을 0열에서 닫음 — 실제 깊이 1
		"목록 항목마다 콜아웃 200개":    rep("- :::note\n  안내\n  :::\n\n", 200),
		"번호 목록 항목마다 콜아웃 200개": rep("1. 단계\n\n   :::note\n   안내\n\n   ```sh\n   x\n   ```\n   :::\n", 200),
		"인용 안 콜아웃 형제 200개":    rep("> :::note\n> 안내\n> :::\n>\n", 200),
		"인용 안 목록 안 콜아웃 200개":  rep("> - :::tip\n>   안내\n>   :::\n", 200),
		"들여쓴 콜아웃 0열 닫기 200개":  rep("  :::note\n  안내\n:::\n\n", 200),
		"정의 목록 콜아웃 200개":      rep("용어\n: :::note\n  안내\n  :::\n\n", 200),
	}
	for name, src := range docs {
		if assertDecision(t, name, src) {
			t.Errorf("%s: 안전 모드", name)
		}
	}
	// golden 시험의 원문과 저장소의 문서들
	var files []string
	for _, pat := range []string{"testdata/*.md", "../*.md", "../spec/*.md"} {
		m, _ := filepath.Glob(pat)
		files = append(files, m...)
	}
	if len(files) < 10 {
		t.Fatalf("문서 %d 개", len(files))
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if assertDecision(t, f, string(b)) {
			t.Errorf("%s: 안전 모드", f)
		}
	}
}

// 현실적인 큰 문서는 안전 모드로 가지 않고 상한 없는 렌더와 같다(리뷰 #126 R4): 목록·번호 목록·인용·콜아웃(목록·인용 안, 서로 안)·표·펜스(안에 콜아웃
// 문법 예시와 ::: 줄, > 줄)·차트·수식·정의 목록·각주·HTML 주석·제목을 실제 깊이 8 이하로 섞은 문서 — 수백~수천 블록 300개(절반은 블록
// 사이 빈 줄을 가끔 뺀다)와 8만 줄 남짓 4개. 안전 모드 0건.
func TestNestingRealisticDocs(t *testing.T) {
	rng := rand.New(rand.NewSource(1264))
	var total, worst int
	for i := range 304 {
		g := &realDocGen{rng: rng, tight: i%2 == 1}
		if rng.Intn(3) == 0 {
			g.b.WriteString("---\ntitle: 문서\n---\n\n")
		}
		top := 50 + rng.Intn(400)
		if i >= 300 {
			top = 3000
		}
		g.blocks(top, 0, "", "")
		src := g.b.String()
		total += g.n
		real, _ := realDepth(src)
		if real > 8 {
			t.Fatalf("#%d: 생성기의 실제 깊이 %d > 8", i, real)
		}
		worst = max(worst, real)
		if assertDecision(t, fmt.Sprintf("#%d", i), src) {
			t.Fatalf("#%d: 현실적인 문서(블록 %d, %d 줄)가 안전 모드", i, g.n, strings.Count(src, "\n"))
		}
	}
	t.Logf("문서 304개, 블록 %d, 가장 깊은 겹 %d", total, worst)
}

// realDocGen 은 사람이 쓰는 모양의 문서를 만든다. 줄 머리는 바깥 컨테이너의 이어지는 줄 머리(cont)를 지킨다.
type realDocGen struct {
	rng   *rand.Rand
	b     strings.Builder
	n     int  // 블록 수
	tight bool // 블록 사이 빈 줄을 가끔 뺀다(목록·제목·펜스가 문단에 바로 붙는 문서)
}

func (g *realDocGen) line(prefix, s string) {
	if s == "" {
		g.b.WriteString(strings.TrimRight(prefix, " ") + "\n")
		return
	}
	g.b.WriteString(prefix + s + "\n")
}

func (g *realDocGen) words() string {
	ws := []string{"문서", "보고", "**굵게**", "`코드`", "[링크](#a)", "$x^2$", "값", "2024년", "합계", "다음", "단계를", "확인한다"}
	var b strings.Builder
	for k := range 2 + g.rng.Intn(8) {
		if k > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(ws[g.rng.Intn(len(ws))])
	}
	return b.String()
}

// blocks 는 블록 n 개를 쓴다 — 첫 줄 머리는 first, 그 뒤는 cont. d 는 지금 겹 수(컨테이너는 d < 4 에서만 연다 — 목록 한 겹이
// 1이라 실제 깊이 ≤ 8).
func (g *realDocGen) blocks(n, d int, first, cont string) {
	for i := range n {
		p := cont
		if i == 0 {
			p = first
		} else if !g.tight || g.rng.Intn(4) != 0 {
			g.line(cont, "")
		}
		g.block(d, p, cont)
	}
}

func (g *realDocGen) block(d int, p, cont string) {
	g.n++
	kinds := 9
	if d < 4 {
		kinds = 14
	}
	switch g.rng.Intn(kinds) {
	case 0, 1:
		g.line(p, g.words())
		for range g.rng.Intn(3) {
			g.line(cont, g.words())
		}
	case 2:
		g.line(p, strings.Repeat("#", 1+g.rng.Intn(4))+" "+g.words())
	case 3:
		g.line(p, "```"+[]string{"", "js", "markdown", "sh"}[g.rng.Intn(4)])
		for range 1 + g.rng.Intn(6) {
			if g.rng.Intn(6) == 0 { // 콜아웃 문법 예시(여는 줄과 닫는 줄의 짝)
				g.line(cont, ":::note")
				g.line(cont, "예시")
				g.line(cont, ":::")
				continue
			}
			g.line(cont, []string{"x := 1", ":::", "> > > 인용 모양", "- - 목록 모양", "    들여쓴 줄", "<div>", "<!--"}[g.rng.Intn(7)])
		}
		g.line(cont, "```")
	case 4:
		g.line(p, "$$")
		g.line(cont, "\\sum_{i=1}^n i")
		g.line(cont, "$$")
	case 5:
		g.line(p, "| 항목 | 값 |")
		g.line(cont, "|---|---:|")
		for range 1 + g.rng.Intn(5) {
			g.line(cont, "| "+g.words()+" | 12 |")
		}
	case 6:
		g.line(p, ":::chart{type="+[]string{"bar", "line", "pie"}[g.rng.Intn(3)]+"}")
		g.line(cont, "| 월 | 매출 |")
		g.line(cont, "|---|---|")
		g.line(cont, "| 1월 | 10 |")
		g.line(cont, ":::")
	case 7:
		g.line(p, "용어")
		g.line(cont, ": "+g.words())
	case 8:
		g.line(p, []string{"<!-- 메모 -->", "본문[^1] 각주", "---", "[TOC]"}[g.rng.Intn(4)])
	case 9, 10: // 목록
		marker, ind := "- ", "  "
		if g.rng.Intn(2) == 0 {
			marker, ind = "1. ", "   "
		}
		for k := range 1 + g.rng.Intn(5) {
			fp := cont + marker
			if k == 0 {
				fp = p + marker
			}
			g.blocks(1+g.rng.Intn(3), d+1, fp, cont+ind)
		}
	case 11: // 인용
		g.blocks(1+g.rng.Intn(3), d+1, p+"> ", cont+"> ")
	default: // 콜아웃
		g.line(p, []string{":::note", ":::tip", ":::warning", ":::box{.wide}", ":::details"}[g.rng.Intn(5)])
		g.blocks(1+g.rng.Intn(3), d+1, cont, cont)
		g.line(cont, ":::")
	}
}

// 상한 아래 문서는 보통 렌더 그대로다 — CommonMark 0.31.2 예제 전체.
func TestNestingCommonMarkUnchanged(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(goldmarkDir(t), "_test", "spec.json"))
	if err != nil {
		skipOrFatalf(t, "spec.json 없음: %v", err)
	}
	var examples []specExample
	if err := json.Unmarshal(raw, &examples); err != nil {
		t.Fatal(err)
	}
	for _, ex := range examples {
		if raw := assertDecision(t, fmt.Sprintf("예제 %d", ex.Example), ex.Markdown); raw {
			t.Errorf("예제 %d: 안전 모드", ex.Example)
		}
	}
	if len(examples) != 652 {
		t.Errorf("예제 %d 개, 기대 652", len(examples))
	}
}

// 무작위 문서 1만 개: 깊은 조각이 없으면 보통 렌더 그대로, 있으면 실제 깊이가 상한을 넘을 때 반드시 안전 모드.
func TestNestingRandomDocs(t *testing.T) {
	rng := rand.New(rand.NewSource(126))
	atoms := append([]string{
		"글 ", "- 항목\n", "1. 번호\n", "> 인용\n", "\n", ":::box\n", ":::\n", ":::note\n안\n:::\n", "| a | b |\n|---|---|\n| 1 | 2 |\n",
		"## 제목\n", "    들여쓴 코드\n", "[링크](#a) ", "$x^2$ ", "$$\n", "```\n", "<div>\n", "</div>\n", "[^1]: 각주\n", "용어\n: 설명\n",
		"  - 안 항목\n", "> > 두 겹\n", "- [ ] 할 일\n", "* 별\n", "+ 더하기\n", "2) 괄호\n", "---\n", "[TOC]\n", "\\\n",
	}, textAtoms...)
	deep := []func() string{
		func() string { return strings.Repeat("> ", MaxNestingDepth-2+rng.Intn(5)) + "x\n" },
		func() string { return strings.Repeat(":::box\n", MaxNestingDepth-2+rng.Intn(5)) },
		func() string { return strings.Repeat("1. ", MaxNestingDepth-2+rng.Intn(5)) + "x\n" },
		func() string { return strings.Repeat("> - ", MaxNestingDepth/2-1+rng.Intn(3)) + "x\n" },
	}
	var normal, raw, deepRaw int
	for i := range 10_000 {
		var b strings.Builder
		withDeep := i%10 == 9
		for range 1 + rng.Intn(40) {
			if withDeep && rng.Intn(8) == 0 {
				b.WriteString(deep[rng.Intn(len(deep))]())
			} else {
				b.WriteString(atoms[rng.Intn(len(atoms))])
			}
		}
		src := b.String()
		isRawDoc := assertDecision(t, fmt.Sprintf("#%d", i), src)
		switch {
		case !withDeep && isRawDoc:
			t.Errorf("#%d: 깊은 조각 없는 문서가 안전 모드 %q", i, shortSrc(src))
		case isRawDoc:
			raw++
			if d, _ := realDepth(src); d > MaxNestingDepth {
				deepRaw++
			}
		default:
			normal++
		}
		if t.Failed() {
			t.Fatalf("#%d 원문 %q", i, src)
		}
	}
	t.Logf("보통 %d, 안전 모드 %d(실제로 상한을 넘은 것 %d)", normal, raw, deepRaw)
	if deepRaw < 100 {
		t.Errorf("상한을 넘는 문서가 너무 적음: %d", deepRaw)
	}
}

// 안전 모드: 원문 글자를 모두(이스케이프해) 보이고, 줄마다 원문 줄 번호를 붙이고, 경고는 frontmatter 경고와 too-complex 하나다.
func TestNestingSafeMode(t *testing.T) {
	deepLine := strings.Repeat("> ", MaxNestingDepth+3) + "깊음 <b>&amp;</b> \x00"
	src := "---\r\ntitle: 가\r\nslides: x\r\n---\r\n앞 <script>alert(1)</script>\r\n\r\n" + deepLine + "\r\n\r\n# 제목\r\n\t탭\r\n"
	r := render(t, src)
	if !isRaw(r) || r.Meta.Title != "가" || len(r.TOC) != 0 || r.SlideLevel != 0 {
		t.Fatalf("안전 모드 결과: %+v", r)
	}
	_, _, fw, _ := parseFrontmatter(normalize([]byte(src)), nil)
	if want := append(fw, Warning{Line: 7, Code: WarnTooComplex, Message: r.Warnings[len(r.Warnings)-1].Message}); !reflect.DeepEqual(r.Warnings, want) {
		t.Errorf("경고 %+v, 기대 %+v", r.Warnings, want)
	}
	// 원문 글자: 줄마다 span 하나, 글자는 frontmatter 뒤 본문 줄 그대로(NUL 은 U+FFFD — goldmark 과 같다), data-line 은 원문 줄
	doc, err := html.Parse(strings.NewReader(r.HTML))
	if err != nil {
		t.Fatal(err)
	}
	var spans []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "span" {
			spans = append(spans, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	want := strings.Split(strings.ReplaceAll(strings.TrimSuffix(string(normalize([]byte(src))), "\n"), "\x00", "�"), "\n")[4:]
	if len(spans) != len(want) {
		t.Fatalf("줄 %d 개, 기대 %d", len(spans), len(want))
	}
	for i, s := range spans {
		if got := nodeText(s); got != want[i] {
			t.Errorf("%d 줄 글자 %q, 기대 %q", i+5, got, want[i])
		}
		if dl := attrOf(s, "data-line"); dl != fmt.Sprint(i+5) {
			t.Errorf("%d 번째 줄의 data-line %q", i, dl)
		}
	}
	contains(t, r.HTML, `<pre class="raw-source" data-line="5"><code>`, "&lt;script&gt;", "&lt;b&gt;&amp;amp;&lt;/b&gt;")
	if strings.Contains(r.HTML, "<script") || strings.Contains(r.HTML, "<b>") {
		t.Errorf("원문의 HTML 이 나감")
	}
	// frontmatter 가 없으면 1 줄부터. 빈 줄도 줄로 남는다
	r = render(t, strings.Repeat(":::box\n", MaxNestingDepth+1)+"\n\n끝")
	if !isRaw(r) || !strings.Contains(r.HTML, `data-line="1"><code><span data-line="1">:::box</span>`) ||
		!strings.Contains(r.HTML, `<span data-line="67"></span>`+"\n"+`<span data-line="68">끝</span>`) {
		t.Errorf("안전 모드 줄: %s", r.HTML[len(r.HTML)-200:])
	}
	if len(r.Warnings) != 1 || r.Warnings[0].Line != MaxNestingDepth+1 {
		t.Errorf("경고 %+v", r.Warnings)
	}
	// frontmatter 바로 뒤의 본문 빈 줄은 남는다(리뷰 #126 R4 — frontmatter 자리만 뺀다)
	r = render(t, "---\ntitle: 제목\n---\n\n\n"+strings.Repeat(":::note\n", MaxNestingDepth+1))
	if !isRaw(r) || !strings.HasPrefix(r.HTML, `<pre class="raw-source" data-line="4"><code><span data-line="4"></span>`+"\n"+`<span data-line="5"></span>`+"\n"+`<span data-line="6">:::note</span>`) {
		t.Errorf("frontmatter 뒤 빈 줄: %.200s", r.HTML)
	}
	// 깨진 frontmatter 는 본문이다 — 첫 줄부터
	r = render(t, "---\n: [\n---\n"+strings.Repeat(":::note\n", MaxNestingDepth+1))
	if !strings.HasPrefix(r.HTML, `<pre class="raw-source" data-line="1"><code><span data-line="1">---</span>`) {
		t.Errorf("깨진 frontmatter: %.200s", r.HTML)
	}
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func attrOf(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// 병적 입력은 선형 시간이다 — 중단 전까지 열린 블록은 64겹 이하다(각주 괄호, 긴 여는 줄, 펜스·HTML 이 섞인 것, 64겹 바로 아래 큰 문서).
// (인용·목록·::: 반복은 limits_test.go TestQuoteDepthLimit)
func TestNestingLinear(t *testing.T) {
	for _, unit := range []string{
		"- :::box\n# 제목\n", "> :::box\n\n", ":::box\n```\n:::x\n```\n", ":::box\n<!--\n", "[^[^[^", "[^a]: [^a]: [^", ":::{" + strings.Repeat("a", 50) + " ",
		"   :::box\n", "    :::box\n", ":::box\n:::\n", "```\n:::box\n", "- a\n  :::box\n  ", "\t> \t- ", "1. 2. 3) ", ": : ", "~~~~\n",
		":::a\n```\n:::b\n", "<!--\n:::a\n```\n-->\n:::b\n:::c\n", "- $$\n  x\n", "     ```\n", "1. 1. ```\n", "> :::a\n>   $$\n", "- :::a\n  ```\n  :::b\n",
		// 닫는 줄을 안쪽에 넘기는 길(#139): 목록 안 컨테이너를 닫는 줄, 안쪽까지 닿지 못해 넘긴 컨테이너가 받는 줄, 목록 안 펜스 코드
		":::a\n- :::b\n  :::\n", ":::a\n- :::b\n  x\n:::\n", ":::a\n> - ```\n>   :::\n", "1. :::a\n   - :::b\n     :::\n",
	} {
		assertLinear(t, unit, repeatTo(unit), 300*time.Millisecond)
	}
	// 64겹 바로 아래(깊이를 꽉 채운 큰 문서 — 보통 렌더)도 선형이다
	full := func(head, unit string) func(int) string {
		return func(size int) string { return head + strings.Repeat(unit, max(1, (size-len(head))/len(unit))) }
	}
	for name, gen := range map[string]func(int) string{
		"64겹 ::: 안 본문":    full(strings.Repeat(":::a\n", MaxNestingDepth), "본문 글 **굵게**\n\n"),
		"64겹 인용 줄 반복":     full("", strings.Repeat("> ", MaxNestingDepth)+"글\n"),
		"64겹 들여쓴 목록 안 본문": full(nestedIndentList(MaxNestingDepth), strings.Repeat("  ", MaxNestingDepth)+"이어진 글\n"),
	} {
		if r := render(t, gen(64_000)); isRaw(r) {
			t.Fatalf("%s: 64겹인데 안전 모드", name)
		}
		assertLinear(t, name, gen, 300*time.Millisecond)
	}
	// 앞에 무엇이 오든 깊은 ::: 는 안전 모드다(#126 R6 의 접두들 — 실제로 연 겹만 센다)
	for _, pre := range []string{"", "> ```\n\n", "    ```\n\n", "```\n```\n\n", "$$\n$$\n\n", "<div>\n\n", "> :::box\n\n", "- a\n\n  ```\n\n"} {
		gen := func(size int) string {
			n := size / 11
			return pre + strings.Repeat(":::box\n", n) + "KEEP\n" + strings.Repeat(":::\n", n)
		}
		r := render(t, gen(64_000))
		if !isRaw(r) || !strings.Contains(r.HTML, "KEEP") || !hasWarn(r, WarnTooComplex) {
			t.Errorf("접두 %q: 안전 모드가 아님", pre)
		}
		assertLinear(t, "접두 "+pre, gen, 300*time.Millisecond)
	}
}

// isCloseLine 은 예전 정규식과 같은 판정이다.
func TestIsCloseLine(t *testing.T) {
	re := regexp.MustCompile(`^:{3,}[ \t]*\n?$`)
	for _, s := range []string{":::", ":::\n", "::::  \n", ":: ", "::: x", ":::\t", "", ":::\n\n", " :::", "::::::::\t \n", ":::a", "::: \n "} {
		if isCloseLine([]byte(s)) != re.MatchString(s) {
			t.Errorf("%q: %v, 정규식 %v", s, isCloseLine([]byte(s)), re.MatchString(s))
		}
	}
}
