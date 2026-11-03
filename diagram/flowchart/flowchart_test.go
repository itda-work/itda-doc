package flowchart

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/itda-work/itda-doc/internal/timingtest"
)

func mustParse(t *testing.T, src string) (*Graph, []Issue) {
	t.Helper()
	g, is, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return g, is
}

// edgeStr 는 간선 하나를 시험에서 견주기 쉬운 글로 적는다: "A-->B" 꼴 + 선·길이·글.
func edgeStr(g *Graph, e *Edge) string {
	op := map[Stroke]string{StrokeNormal: "-", StrokeDotted: ".", StrokeThick: "=", StrokeInvisible: "~"}[e.Stroke]
	s := g.Nodes[e.From].ID + op
	if e.Both {
		s = g.Nodes[e.From].ID + "<" + op
	}
	if e.Arrow {
		s += ">"
	}
	s += g.Nodes[e.To].ID
	if e.Length != 1 {
		s += fmt.Sprintf("/%d", e.Length)
	}
	if e.Label != "" {
		s += "|" + e.Label
	}
	return s
}

func edges(g *Graph) string {
	var out []string
	for _, e := range g.Edges {
		out = append(out, edgeStr(g, e))
	}
	return strings.Join(out, " ")
}

// 지원 문법(안내서 2장 표)마다 읽은 결과.
func TestSyntaxEdges(t *testing.T) {
	cases := []struct{ src, want string }{
		{"A-->B", "A->B"},
		{"A --- B", "A-B"},
		{"A-.->B", "A.>B"},
		{"A -.- B", "A.B"},
		{"A==>B", "A=>B"},
		{"A === B", "A=B"},
		{"A ~~~ B", "A~B"},
		{"A--->B", "A->B/2"},
		{"A---->B", "A->B/3"},
		{"A----->B", "A->B/4"},
		{"A------>B", "A->B/4"}, // MaxLength 에서 멈춤
		{"A----B", "A-B/2"},
		{"A-..->B", "A.>B/2"},
		{"A===>B", "A=>B/2"},
		{"A<-->B", "A<->B"},
		{"A<-.->B", "A<.>B"},
		{"A<==>B", "A<=>B"},
		{"A-->|예|B", "A->B|예"},
		{"A -->|\"따옴표 글\"| B", "A->B|따옴표 글"},
		{"A -- 글자 화살표 --> B", "A->B|글자 화살표"},
		{"A -- 열린 글 --- B", "A-B|열린 글"},
		{"A -. 점선 글 .-> B", "A.>B|점선 글"},
		{"A == 굵은 글 ==> B", "A=>B|굵은 글"},
		{"A-->B-->C", "A->B B->C"},
		{"A & B --> C & D", "A->C A->D B->C B->D"},
		{"A --> B & C --> D", "A->B A->C B->D C->D"},
		{"가[시작] --> 나", "가->나"},
		{"my-node --> a.b", "my-node->a.b"},
		{"A-->A", "A->A"},
		{"A-->B;B-->C", "A->B B->C"},
		{"A[\"a; b\"] --> B", "A->B"},
	}
	for _, c := range cases {
		g, _ := mustParse(t, "flowchart TD\n"+c.src)
		if got := edges(g); got != c.want {
			t.Errorf("%q: %q, 기대 %q", c.src, got, c.want)
		}
	}
}

func TestSyntaxShapes(t *testing.T) {
	cases := []struct {
		src   string
		shape Shape
		label string
	}{
		{"A", ShapeRect, "A"},
		{"A[사각]", ShapeRect, "사각"},
		{"A(둥근)", ShapeRound, "둥근"},
		{"A([경기장])", ShapeStadium, "경기장"},
		{"A[[서브루틴]]", ShapeSubroutine, "서브루틴"},
		{"A[(DB)]", ShapeCylinder, "DB"},
		{"A((원))", ShapeCircle, "원"},
		{"A{판단}", ShapeDiamond, "판단"},
		{"A{{육각}}", ShapeHexagon, "육각"},
		{"A[/입력/]", ShapeLean, "입력"},
		{"A[\\역\\]", ShapeLeanAlt, "역"},
		{"A[/위가 좁은\\]", ShapeTrap, "위가 좁은"},
		{"A[\\아래가 좁은/]", ShapeTrapAlt, "아래가 좁은"},
		{"A[\"괄호 [a] (b) {c} 글\"]", ShapeRect, "괄호 [a] (b) {c} 글"},
		{"A[가<br>나<br/>다<BR />라]", ShapeRect, "가\n나\n다\n라"},
		{"A[\"#quot;인용#quot; #35; #lt;b#gt; #9829;\"]", ShapeRect, "\"인용\" # <b> ♥"},
		{"A[<b>굵게</b>]", ShapeRect, "<b>굵게</b>"}, // 다른 HTML 은 글자 그대로
		{"A[  여러 칸  ]", ShapeRect, "여러 칸"},       // 앞뒤 빈칸만 지운다 — 안쪽 연속 빈칸은 원문(정책 B, policy_test.go)
		{"A[]", ShapeRect, ""},
		{"A:::cls", ShapeRect, "A"},
		{"A[x]:::cls", ShapeRect, "x"},
	}
	for _, c := range cases {
		g, _ := mustParse(t, "flowchart TD\n"+c.src)
		n := g.Nodes[0]
		if n.Shape != c.shape || n.Label != c.label {
			t.Errorf("%q: 모양 %d 글 %q, 기대 %d %q", c.src, n.Shape, n.Label, c.shape, c.label)
		}
	}
}

func TestSyntaxStructure(t *testing.T) {
	for src, dir := range map[string]Dir{"graph": TB, "graph TD": TB, "flowchart TB": TB, "flowchart BT": BT, "graph LR": LR, "flowchart RL": RL} {
		g, _ := mustParse(t, src+"\nA")
		if g.Dir != dir {
			t.Errorf("%q: 방향 %d", src, g.Dir)
		}
	}
	// 머리 줄 뒤 ; 와 주석·빈 줄
	g, _ := mustParse(t, "graph LR;A-->B\n\n%% 주석 A-->C\n  B-->C\n")
	if edges(g) != "A->B B->C" || g.Dir != LR {
		t.Errorf("주석·; : %s", edges(g))
	}
	// 나중 정의가 이긴다, 처음 줄이 노드 줄
	g, _ = mustParse(t, "graph TD\nA[하나] --> B\nA[둘]\nB(셋)")
	if g.Nodes[0].Label != "둘" || g.Nodes[1].Shape != ShapeRound || g.Nodes[0].Line != 2 {
		t.Errorf("다시 정의: %+v %+v", g.Nodes[0], g.Nodes[1])
	}
	// subgraph 꼴과 구성원
	g, _ = mustParse(t, `graph TD
X --> A
subgraph s1 [첫 묶음]
  A --> B
end
subgraph 둘째 묶음
  C
  B
end
subgraph "따옴표 제목"
  D
end
subgraph s4["따옴표 괄호"]
  E
end
subgraph
  F
end`)
	wantC := []struct{ id, title string }{{"s1", "첫 묶음"}, {"둘째 묶음", "둘째 묶음"}, {"따옴표 제목", "따옴표 제목"}, {"s4", "따옴표 괄호"}, {"subgraph-5", ""}}
	for i, w := range wantC {
		if c := g.Clusters[i]; c.ID != w.id || c.Title != w.title {
			t.Errorf("subgraph %d: %+v, 기대 %+v", i, c, w)
		}
	}
	member := map[string]int{"X": -1, "A": 0, "B": 0, "C": 1, "D": 2, "E": 3, "F": 4} // B 는 처음 들어간 s1
	for _, n := range g.Nodes {
		if member[n.ID] != n.Cluster {
			t.Errorf("%s 의 subgraph %d, 기대 %d", n.ID, n.Cluster, member[n.ID])
		}
	}
	// frontmatter·접근성
	g, is := mustParse(t, "---\ntitle: \"분기 흐름<br>요약\"\n---\nflowchart LR\naccTitle: 접근성 이름\naccDescr {\n  여러 줄\n  설명\n}\nA")
	if g.Title != "분기 흐름 요약" || g.AccTitle != "접근성 이름" || g.AccDescr != "여러 줄\n설명" || len(is) != 0 {
		t.Errorf("frontmatter·acc: %q %q %q %v", g.Title, g.AccTitle, g.AccDescr, is)
	}
	// 접근성 글은 줄 끝까지 — ; 와 엔티티 #35; 로 나누지 않는다(리뷰 R2 작업 중)
	g, _ = mustParse(t, "graph TD\naccTitle: 매출; 비용 #35;1\naccDescr: 가; 나\nA")
	if g.AccTitle != "매출; 비용 #1" || g.AccDescr != "가; 나" || len(g.Nodes) != 1 {
		t.Errorf("acc 의 ; : %q %q %d", g.AccTitle, g.AccDescr, len(g.Nodes))
	}
	g, _ = mustParse(t, "graph TD\naccDescr: 한 줄 설명\naccDescr { 한 줄 } \nA")
	if g.AccDescr != "한 줄" {
		t.Errorf("accDescr: %q", g.AccDescr)
	}
}

// 그리지 못하는 문법은 *Error 와 그 줄이다(원문 + 경고는 mdoc 이).
func TestUnsupported(t *testing.T) {
	cases := []struct {
		src  string
		line int
		msg  string
	}{
		{"", 0, "비어 있음"},
		{"%% 주석만", 0, "비어 있음"},
		{"pie\n\"a\": 1", 1, "pie 그림은 아직"},
		{"flowchart-elk TD\nA-->B", 1, "flowchart-elk"},
		{"그래프 TD", 1, "알 수 없음"},
		{"graph XY\nA", 1, "방향"},
		{"graph TD A-->B", 1, "머리 줄 뒤"},
		{"graph TD", 1, "노드가 없음"},
		{"graph TD\nsubgraph a\nsubgraph b\nX\nend\nend", 3, "subgraph 안의 subgraph"},
		{"graph TD\nA\nsubgraph a\nX", 3, "end 로 닫히지 않음"},
		{"graph TD\nA\nend", 3, "닫을 subgraph 가 없는"},
		{"graph TD\nsubgraph s1\nX\nend\nA --> s1", 5, "subgraph “s1” 를 노드로"},
		{"graph TD\nA --> s1\nsubgraph s1\nX\nend", 3, "subgraph “s1” 를 노드로"},
		{"graph TD\nsubgraph s\nA\nend\nsubgraph s\nB\nend", 5, "id “s” 가 겹침"},
		{"graph TD\nA(((겹원)))", 2, "겹원"},
		{"graph TD\nA>깃발]", 2, "깃발"},
		{"graph TD\nA@{ shape: rect }", 2, "@{"},
		{"graph TD\nA[\"`**굵게**`\"]", 2, "마크다운"},
		{"graph TD\nA -->|\"`**굵게**`\"| B", 2, "마크다운"},
		{"graph TD\nA --o B", 2, "o·x"},
		{"graph TD\nA --x B", 2, "o·x"},
		{"graph TD\nA -- 글 --o B", 2, "o·x"},
		{"graph TD\nA[열고 안 닫음", 2, "닫히지 않음"},
		{"graph TD\nA[\"따옴표", 2, "따옴표가 닫히지"},
		{"graph TD\nA[\"따옴표\" 뒤 ]", 2, "라벨 뒤에"},
		{"graph TD\nA -> B", 2, "화살표가 와야"},
		{"graph TD\nA - B", 2, "화살표가 와야"},
		{"graph TD\nA <-- B", 2, "닫히지 않음"},
		{"graph TD\nA ~~ B", 2, "화살표가 와야"},
		{"graph TD\nA -- 글만 B", 2, "닫히지 않음"},
		{"graph TD\nA -. 글만 B", 2, "닫히지 않음"},
		{"graph TD\nA -->|안 닫음 B", 2, "|…| 이 닫히지"},
		{"graph TD\nA -- 글 -->|또 글| B", 2, "글이 둘"},
		{"graph TD\nA --> end", 2, "노드 이름으로 쓸 수 없음"},
		{"graph TD\nA -->", 2, "노드 이름이 와야"},
		{"graph TD\n--> B", 2, "노드 이름이 와야"},
		{"graph TD\nend x", 2, "end 뒤에"},
		{"---\ntitle: x\nflowchart TD\nA", 1, "frontmatter"},
		{"graph TD\naccDescr {\n안 닫음", 2, "accDescr {"},
	}
	for _, c := range cases {
		_, _, err := Parse(c.src)
		var fe *Error
		if !errors.As(err, &fe) {
			t.Errorf("%q: 오류가 아님(%v)", c.src, err)
			continue
		}
		if fe.Line != c.line || !strings.Contains(fe.Msg, c.msg) {
			t.Errorf("%q: %d줄 %q, 기대 %d줄 …%s…", c.src, fe.Line, fe.Msg, c.line, c.msg)
		}
		if _, err := Render(c.src); err == nil {
			t.Errorf("%q: Render 가 그림", c.src)
		}
	}
}

// 모양을 문서에 적는 줄·click 은 쓰지 않고 그림은 그린다 — 종류마다 한 번, 그 줄 번호로.
func TestIgnoredLines(t *testing.T) {
	src := `%%{init: {"theme": "dark"}}%%
flowchart TD
  A --> B
  style A fill:#f00
  style B fill:#0f0
  classDef red fill:#f00
  class B red
  linkStyle 0 stroke:red
  click A "https://example.com"
  click B call alert()
  C:::red --> A
  subgraph s
    direction LR
    D
  end
  subgraph 빈
  end`
	r, err := Render(src)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, is := range r.Issues {
		got = append(got, fmt.Sprintf("%d:%s", is.Line, strings.SplitN(is.Msg, " ", 2)[0]))
	}
	want := "1:설정 4:style 6:classDef 7:class 8:linkStyle 9:click 11:노드 13:subgraph 16:빈"
	if strings.Join(got, " ") != want {
		t.Errorf("경고 %q\n기대 %q", strings.Join(got, " "), want)
	}
	for _, bad := range []string{"#f00", "red", "example.com", "alert", "fill", "href"} {
		if strings.Contains(r.SVG, bad) {
			t.Errorf("SVG 에 %q", bad)
		}
	}
	// frontmatter 의 설정 키
	r, err = Render("---\ntitle: 제목\nconfig:\n  theme: dark\n  themeCSS: x\ndisplayMode: compact\n---\ngraph TD\nA")
	if err != nil || r.Caption != "제목" || len(r.Issues) != 2 || r.Issues[0].Line != 3 || r.Issues[1].Line != 6 {
		t.Errorf("frontmatter 설정: %v %+v", err, r)
	}
}

// 리뷰 R1: frontmatter 는 YAML 이다 — 블록 스칼라·따옴표·주석이 캡션을 바꾸지 않고, 읽을 수 없거나 글이 아닌 title 은 원문(*Error).
func TestFrontmatterYAML(t *testing.T) {
	for fm, want := range map[string]string{
		"title: |\n  구매 결재 흐름":            "구매 결재 흐름",
		"title: >\n  구매 결재\n  흐름":         "구매 결재 흐름",
		"title: |\n  첫 줄\n  둘째 줄":         "첫 줄 둘째 줄",
		"title: 구매 결재 # 메모":               "구매 결재",
		"title: '구매 결재'":                  "구매 결재",
		"title: 'It''s 흐름'":               "It's 흐름",
		"title: \"따옴표 \\\"안\\\" 글\"":      "따옴표 \"안\" 글",
		"title: \"#quot;엔티티#quot;<br>줄\"": "#quot;엔티티#quot; 줄", // 정책 B — 원본은 frontmatter 를 엔티티 전처리 전에 뗀다(글자 그대로)
		"title: 2026":                     "2026",
		"title:":                          "",
		"# 주석만":                           "",
		"title: 구매 결재 흐름 #117":            "구매 결재 흐름", // YAML 의 공백 뒤 # 는 주석
		"title: \"구매 결재 흐름 #117\"":        "구매 결재 흐름 #117",
		"base: &t 흐름\ntitle: *t":          "흐름",
	} {
		r, err := Render("---\n" + fm + "\n---\nflowchart TD\nA --> B")
		if err != nil || r.Caption != want {
			t.Errorf("%q: 캡션 %q 오류 %v, 기대 %q", fm, r.Caption, err, want)
		}
	}
	for fm, line := range map[string]int{
		"title: [가, 나]":      2,
		"title:\n  a: b":     2,
		"title: \"닫히지 않음":    0, // yaml 오류는 줄을 모른다 — frontmatter 첫 줄
		"- 목록":               2,
		"title: a\n  b: : c": 0,
		"title: " + strings.Repeat("가", MaxLabel+1): 2,
		// 리뷰 R4(P3): 여러 줄 제목·별칭의 길이 초과 오류 줄은 title 키 줄이다(값 줄이 아니다)
		"title:\n  >-\n    " + strings.Repeat("가", MaxLabel+1):        2, // 값(가 201자)은 4행 — 키 줄 보고
		"base: &t " + strings.Repeat("가", MaxLabel+1) + "\ntitle: *t": 3, // 별칭 대상 정의는 2행 — 키 줄 보고
	} {
		_, err := Render("---\n" + fm + "\n---\nflowchart TD\nA --> B")
		var fe *Error
		if !errors.As(err, &fe) {
			t.Errorf("%q: 오류가 아님", fm)
			continue
		}
		if line > 0 && fe.Line != line {
			t.Errorf("%q: %d줄 %q, 기대 %d줄", fm, fe.Line, fe.Msg, line)
		}
	}
	// 설정 키는 최상위 키마다 경고 하나(들여 쓴 값은 따로 세지 않는다), 그림은 그린다
	r, err := Render("---\ntitle: t\nconfig:\n  theme: dark\n  flowchart:\n    curve: linear\ndisplayMode: compact\n---\ngraph TD\nA")
	if err != nil || r.Caption != "t" || len(r.Issues) != 2 || r.Issues[0].Line != 3 || r.Issues[1].Line != 7 {
		t.Errorf("설정 키: %v %+v", err, r.Issues)
	}
}

// 상한 = 그림, 상한 + 1 = 오류.
func TestLimits(t *testing.T) {
	nodes := func(n int) string {
		var b strings.Builder
		b.WriteString("graph LR\n")
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, "n%d\n", i)
		}
		return b.String()
	}
	edgesN := func(n int) string {
		var b strings.Builder
		b.WriteString("graph LR\n")
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, "n%d --> n%d\n", i%20, (i*7+1)%20)
		}
		return b.String()
	}
	subs := func(n int) string {
		var b strings.Builder
		b.WriteString("graph LR\n")
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, "subgraph s%d\nn%d\nend\n", i, i)
		}
		return b.String()
	}
	label := func(n int) string { return "graph LR\nA[" + strings.Repeat("가", n) + "]" }
	edgeLabel := func(n int) string { return "graph LR\nA -->|" + strings.Repeat("a", n) + "| B" }
	for _, c := range []struct {
		name string
		gen  func(int) string
		max  int
	}{{"노드", nodes, MaxNodes}, {"화살표", edgesN, MaxEdges}, {"subgraph", subs, MaxSubgraphs}, {"라벨", label, MaxLabel}, {"화살표 글", edgeLabel, MaxLabel}} {
		if _, err := Render(c.gen(c.max)); err != nil {
			t.Errorf("%s %d: %v", c.name, c.max, err)
		}
		if _, err := Render(c.gen(c.max + 1)); err == nil {
			t.Errorf("%s %d: 오류가 아님", c.name, c.max+1)
		}
	}
	src := "graph LR\nA-->B\n%%" + strings.Repeat("x", MaxSource)
	if _, err := Render(src[:MaxSource]); err != nil {
		t.Errorf("원문 %d: %v", MaxSource, err)
	}
	if _, err := Render(src[:MaxSource+1]); err == nil {
		t.Errorf("원문 %d: 오류가 아님", MaxSource+1)
	}
	// A & B & … 의 곱은 화살표 상한에 걸린다
	var b strings.Builder
	b.WriteString("graph LR\n")
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&b, "a%d & ", i)
	}
	b.WriteString("a --> ")
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&b, "b%d & ", i)
	}
	b.WriteString("b")
	if _, err := Render(b.String()); err == nil || !strings.Contains(err.Error(), "화살표가") {
		t.Errorf("& 곱: %v", err)
	}
}

// ---- 무작위 그림

func randLabel(rng *rand.Rand) string {
	words := []string{"매출", "보고", "승인 요청", "검토", "ERP", "data", "123", "아주긴낱말이라줄을바꿔야하는경우의라벨", "A/B", "x", "고객 응대 및 후속 조치"}
	n := 1 + rng.Intn(4)
	var w []string
	for i := 0; i < n; i++ {
		w = append(w, words[rng.Intn(len(words))])
	}
	return strings.Join(w, " ")
}

// randGraph 는 문법 전체를 섞은 무작위 흐름도다.
func randGraph(rng *rand.Rand, nNodes, nEdges int, withSub bool) string {
	dirs := []string{"TD", "BT", "LR", "RL"}
	opens := []string{"[", "(", "([", "[[", "[(", "((", "{", "{{", "[/", "[\\", "[/", "[\\"}
	closes := []string{"]", ")", "])", "]]", ")]", "))", "}", "}}", "/]", "\\]", "\\]", "/]"}
	arrows := []string{"-->", "---", "-.->", "-.-", "==>", "===", "--->", "<-->", "~~~", "---->"}
	var b strings.Builder
	fmt.Fprintf(&b, "flowchart %s\n", dirs[rng.Intn(4)])
	def := func(i int) string {
		k := rng.Intn(len(opens))
		return fmt.Sprintf("n%d%s\"%s\"%s", i, opens[k], randLabel(rng), closes[k])
	}
	for i := 0; i < nNodes; i++ {
		b.WriteString(def(i) + "\n")
	}
	if withSub {
		at := 0
		for s := 0; at < nNodes && s < 4; s++ {
			fmt.Fprintf(&b, "subgraph s%d [%s]\n", s, randLabel(rng))
			for k := 0; k < 1+rng.Intn(4) && at < nNodes; k++ {
				fmt.Fprintf(&b, "  n%d\n", at)
				at += 1 + rng.Intn(2)
			}
			b.WriteString("end\n")
		}
	}
	for i := 0; i < nEdges; i++ {
		a, c := rng.Intn(nNodes), rng.Intn(nNodes)
		ar := arrows[rng.Intn(len(arrows))]
		lab := ""
		if rng.Intn(3) == 0 && ar != "~~~" {
			lab = "|" + randLabel(rng) + "|"
		}
		fmt.Fprintf(&b, "n%d %s%s n%d\n", a, ar, lab, c)
	}
	return b.String()
}

type rect struct{ x0, y0, x1, y1 float64 }

func (a rect) overlaps(b rect, tol float64) bool {
	return a.x0 < b.x1-tol && b.x0 < a.x1-tol && a.y0 < b.y1-tol && b.y0 < a.y1-tol
}

func (a rect) contains(b rect, tol float64) bool {
	return b.x0 >= a.x0-tol && b.y0 >= a.y0-tol && b.x1 <= a.x1+tol && b.y1 <= a.y1+tol
}

func nodeRect(n nodeBox) rect {
	return rect{n.c.X - n.w/2, n.c.Y - n.h/2, n.c.X + n.w/2, n.c.Y + n.h/2}
}

// 배치 속성(무작위 그림 400개 × 방향 넷): 노드끼리·노드와 간선 라벨이 겹치지 않고, subgraph 상자는 서로 겹치지 않으며
// 구성원을 담고 밖의 노드와 겹치지 않는다. 화살표 끝은 그 노드의 테두리에 있고, 모든 것이 그림 안이다. 라벨 글자는 줄을
// 바꿔도 하나도 빠지지 않는다.
func TestLayoutProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(117))
	for it := 0; it < 400; it++ {
		src := randGraph(rng, 2+rng.Intn(18), rng.Intn(30), rng.Intn(2) == 0)
		g, _, err := Parse(src)
		if err != nil {
			t.Fatalf("무작위 그림 %d 을 읽지 못함: %v\n%s", it, err, src)
		}
		l, err := layout(g)
		if err != nil {
			t.Fatalf("%d: %v", it, err)
		}
		fail := func(f string, a ...any) { t.Errorf("그림 %d: %s\n%s", it, fmt.Sprintf(f, a...), src) }
		all := rect{0, 0, l.W, l.H}
		for i, n := range l.nodes {
			r := nodeRect(n)
			if !all.contains(r, 0.01) {
				fail("노드 %s 가 그림 밖 %+v (%v×%v)", g.Nodes[i].ID, r, l.W, l.H)
			}
			for j := i + 1; j < len(l.nodes); j++ {
				if r.overlaps(nodeRect(l.nodes[j]), 0.5) {
					fail("노드 %s·%s 겹침", g.Nodes[i].ID, g.Nodes[j].ID)
				}
			}
			for k, e := range l.edges {
				if len(e.label) > 0 && g.Edges[k].Stroke != StrokeInvisible { // 자기 간선의 라벨도(리뷰 R1)
					lr := rect{e.lc.X - e.lw/2, e.lc.Y - e.lh/2, e.lc.X + e.lw/2, e.lc.Y + e.lh/2}
					if r.overlaps(lr, 0.5) {
						fail("노드 %s 와 간선 %d 라벨 겹침", g.Nodes[i].ID, k)
					}
				}
			}
			if joined, want := strings.ReplaceAll(strings.Join(n.lines, ""), " ", ""), strings.ReplaceAll(strings.ReplaceAll(g.Nodes[i].Label, "\n", ""), " ", ""); joined != want {
				fail("라벨 글자가 빠짐 %q → %q", g.Nodes[i].Label, n.lines)
			}
		}
		for c, cb := range l.clusters {
			if !cb.show {
				continue
			}
			box := rect{cb.x, cb.y, cb.x + cb.w, cb.y + cb.h}
			if !all.contains(box, 0.01) {
				fail("subgraph %d 상자가 그림 밖", c)
			}
			for d := c + 1; d < len(l.clusters); d++ {
				o := l.clusters[d]
				if o.show && box.overlaps(rect{o.x, o.y, o.x + o.w, o.y + o.h}, 0.5) {
					fail("subgraph %d·%d 상자 겹침", c, d)
				}
			}
			for i, n := range l.nodes {
				in := g.Nodes[i].Cluster == c
				if in && !box.contains(nodeRect(n), 0.5) {
					fail("subgraph %d 상자가 구성원 %s 를 담지 않음", c, g.Nodes[i].ID)
				}
				if !in && box.overlaps(nodeRect(n), 0.5) {
					fail("subgraph %d 상자가 밖의 노드 %s 와 겹침", c, g.Nodes[i].ID)
				}
			}
		}
		for k, e := range l.edges {
			for _, p := range e.pts {
				if p.X < 0 || p.Y < 0 || p.X > l.W || p.Y > l.H {
					fail("간선 %d 의 점이 그림 밖 %v", k, p)
				}
			}
			if len(e.pts) == 0 { // 보이지 않는 자기 간선(~~~)은 그리지 않는다
				continue
			}
			from, to := l.nodes[g.Edges[k].From], l.nodes[g.Edges[k].To]
			for _, end := range []struct {
				p pt
				n nodeBox
			}{{e.pts[0], from}, {e.pts[len(e.pts)-1], to}} {
				if !nodeRect(end.n).contains(rect{end.p.X, end.p.Y, end.p.X, end.p.Y}, 0.6) {
					fail("간선 %d 끝 %v 가 노드 밖", k, end.p)
				}
				if d := boundaryDist(end.n, g.Nodes[nodeOf(l, end.n)].Shape, end.p); d > 0.1 {
					fail("간선 %d 끝 %v 가 노드 테두리에서 %.2f 떨어짐", k, end.p, d)
				}
			}
		}
		// 간선 라벨끼리 겹치지 않는다(자기 간선 포함 — 리뷰 R1: 같은 노드의 고리 둘이 같은 자리였다)
		var labels []rect
		var lk []int
		for k, e := range l.edges {
			if len(e.label) > 0 && len(e.pts) > 0 {
				labels = append(labels, rect{e.lc.X - e.lw/2, e.lc.Y - e.lh/2, e.lc.X + e.lw/2, e.lc.Y + e.lh/2})
				lk = append(lk, k)
			}
		}
		for a := range labels {
			for b := a + 1; b < len(labels); b++ {
				if labels[a].overlaps(labels[b], 0.5) {
					fail("간선 %d·%d 라벨 겹침", lk[a], lk[b])
				}
			}
		}
		if t.Failed() {
			return
		}
	}
}

func nodeOf(l *laid, n nodeBox) int {
	for i, m := range l.nodes {
		if m.c == n.c && m.w == n.w {
			return i
		}
	}
	return -1
}

// boundaryDist 는 점 p 와 모양 테두리 사이의 거리다. 테두리는 svg.go 가 그리는 그대로다 — 둥근 사각형·경기장형의 모서리
// 원호, 원통의 위·아래 타원호는 촘촘한 꺾은선으로 잰다(리뷰 R2: 사각형으로 근사하면 구현과 같은 근사를 반복할 뿐이다).
func boundaryDist(n nodeBox, sh Shape, p pt) float64 {
	if sh == ShapeCircle {
		return math.Abs(math.Hypot(p.X-n.c.X, p.Y-n.c.Y) - n.w/2)
	}
	poly := outline(n, sh)
	best := math.Inf(1)
	for i := range poly {
		a, b := poly[i], poly[(i+1)%len(poly)]
		dx, dy := b.X-a.X, b.Y-a.Y
		u := 0.0
		if l := dx*dx + dy*dy; l > 0 {
			u = math.Max(0, math.Min(1, ((p.X-a.X)*dx+(p.Y-a.Y)*dy)/l))
		}
		best = math.Min(best, math.Hypot(a.X+u*dx-p.X, a.Y+u*dy-p.Y))
	}
	return best
}

// outline 은 모양의 외곽선(닫힌 꺾은선)이다.
func outline(n nodeBox, sh Shape) []pt {
	x0, y0, x1, y1 := n.c.X-n.w/2, n.c.Y-n.h/2, n.c.X+n.w/2, n.c.Y+n.h/2
	arc := func(cx, cy, a, b, from, to float64) []pt { // 각도 from → to(라디안, y 아래가 +)
		var out []pt
		for k := 0; k <= 64; k++ {
			th := from + (to-from)*float64(k)/64
			out = append(out, pt{cx + a*math.Cos(th), cy + b*math.Sin(th)})
		}
		return out
	}
	switch sh {
	case ShapeRound, ShapeStadium:
		// svg.go 가 쓰는 rx 속성 값에서 SVG 2 rect 규칙(ry 를 안 적으면 ry = rx, 각각 너비·높이의 반까지)으로 — 구현의
		// cornerRadii 를 부르지 않고 따로 적는다(같은 근사를 되풀이하지 않게)
		attr := roundRx
		if sh == ShapeStadium {
			attr = n.h / 2
		}
		rx, ry := math.Min(attr, n.w/2), math.Min(attr, n.h/2)
		var out []pt
		out = append(out, arc(x1-rx, y0+ry, rx, ry, -math.Pi/2, 0)...)
		out = append(out, arc(x1-rx, y1-ry, rx, ry, 0, math.Pi/2)...)
		out = append(out, arc(x0+rx, y1-ry, rx, ry, math.Pi/2, math.Pi)...)
		out = append(out, arc(x0+rx, y0+ry, rx, ry, math.Pi, 3*math.Pi/2)...)
		return out
	case ShapeCylinder:
		ry := cylRy(n.w)
		var out []pt
		out = append(out, arc(n.c.X, y0+ry, n.w/2, ry, math.Pi, 2*math.Pi)...) // 위 타원의 위쪽 반
		out = append(out, arc(n.c.X, y1-ry, n.w/2, ry, 0, math.Pi)...)         // 아래 타원의 아래쪽 반
		return out
	case ShapeDiamond, ShapeHexagon, ShapeLean, ShapeLeanAlt, ShapeTrap, ShapeTrapAlt:
		return shapePoly(n, sh)
	}
	return []pt{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}} // 사각형·서브루틴
}

// 리뷰 R2: 둥근 모양의 끝점은 실제 원호 위 — 경기장형 고리(리뷰 사례)·둥근 사각형·원통, 방향마다 여러 각도.
func TestRoundClip(t *testing.T) {
	g, _ := mustParse(t, "flowchart TD\nA([가]) --> A")
	l, _ := layout(g)
	n, p := l.nodes[0], l.edges[0].pts[0]
	r := n.h / 2
	cx := n.c.X + n.w/2 - r
	if d := math.Abs(math.Hypot(p.X-cx, p.Y-n.c.Y) - r); d > 0.05 {
		t.Errorf("경기장형 고리 시작점이 끝 원에서 %.3f 떨어짐", d)
	}
	// 리뷰 R3: 높이 > 너비인 경기장형(여러 줄 라벨) — 모서리는 rx = 너비/2, ry = 높이/2 인 타원이다
	g, _ = mustParse(t, "flowchart TD\nA([가<br>가<br>가<br>가<br>가<br>가<br>가<br>가<br>가<br>가]) --> A")
	l, _ = layout(g)
	n = l.nodes[0]
	if n.h <= n.w {
		t.Fatalf("세로로 긴 경기장형이 아님 %v×%v", n.w, n.h)
	}
	for _, q := range []pt{l.edges[0].pts[0], l.edges[0].pts[len(l.edges[0].pts)-1]} {
		if d := boundaryDist(n, ShapeStadium, q); d > 0.05 {
			t.Errorf("세로로 긴 경기장형 고리 끝점이 테두리에서 %.3f", d)
		}
	}
	for _, sh := range []Shape{ShapeStadium, ShapeRound, ShapeCylinder, ShapeRect, ShapeSubroutine} {
		for _, wh := range [][2]float64{{120, 40}, {60, 44}, {200, 80}, {40, 120}, {141, 198}, {10, 10}} { // 세로로 긴 것(리뷰 R3)
			nb := nodeBox{c: pt{300, 200}, w: wh[0], h: wh[1]}
			for k := 0; k < 72; k++ {
				th := float64(k) * math.Pi / 36
				q := clipShape(nb, sh, pt{300 + 1000*math.Cos(th), 200 + 1000*math.Sin(th)})
				if d := boundaryDist(nb, sh, q); d > 0.05 {
					t.Errorf("모양 %d %v 각도 %d: 끝점이 테두리에서 %.3f", sh, wh, k*5, d)
				}
			}
		}
	}
}

// 리뷰 R1: 자기 간선마다 따로 고리·라벨 자리 — 같은 노드의 고리 여럿, 이웃 노드의 긴 자기 간선 라벨(방향 넷).
func TestSelfLoops(t *testing.T) {
	for _, dir := range []string{"TD", "BT", "LR", "RL"} {
		for _, body := range []string{
			"A -->|승인| A\nA -->|반려| A",
			"A --> A\nA -.-> A\nA ==>|굵게| A\nA -->|여러<br>줄<br>라벨| A\nA --> B",
			"A --> B\nA -->|가나다라마바사아자차| A\nB -->|타파하가나다라마바사| B",
			"subgraph s [묶음]\nA -->|안| A\nend\nB -->|밖| B\nA --> B",
		} {
			src := "flowchart " + dir + "\n" + body
			g, _ := mustParse(t, src)
			l, err := layout(g)
			if err != nil {
				t.Fatal(err)
			}
			for i := range l.edges {
				for j := i + 1; j < len(l.edges); j++ {
					a, b := l.edges[i], l.edges[j]
					if a.self && b.self && g.Edges[i].From == g.Edges[j].From && reflect.DeepEqual(a.pts, b.pts) {
						t.Errorf("%s: 고리 %d·%d 의 경로가 같음", src, i, j)
					}
					if len(a.label) > 0 && len(b.label) > 0 {
						ra := rect{a.lc.X - a.lw/2, a.lc.Y - a.lh/2, a.lc.X + a.lw/2, a.lc.Y + a.lh/2}
						rb := rect{b.lc.X - b.lw/2, b.lc.Y - b.lh/2, b.lc.X + b.lw/2, b.lc.Y + b.lh/2}
						if ra.overlaps(rb, 0.5) {
							t.Errorf("%s: 라벨 %q·%q 겹침", src, g.Edges[i].Label, g.Edges[j].Label)
						}
					}
				}
				e := l.edges[i]
				if !e.self || len(e.label) == 0 {
					continue
				}
				lr := rect{e.lc.X - e.lw/2, e.lc.Y - e.lh/2, e.lc.X + e.lw/2, e.lc.Y + e.lh/2}
				for _, n := range l.nodes {
					if lr.overlaps(nodeRect(n), 0.5) {
						t.Errorf("%s: 고리 라벨 %q 가 노드와 겹침", src, g.Edges[i].Label)
					}
				}
			}
		}
	}
}

// 같은 입력은 같은 출력이다 — 여러 고루틴에서 여러 번.
func TestDeterministic(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	var srcs []string
	for i := 0; i < 30; i++ {
		srcs = append(srcs, randGraph(rng, 5+rng.Intn(30), 10+rng.Intn(40), i%2 == 0))
	}
	want := make([]string, len(srcs))
	for i, s := range srcs {
		r, err := Render(s)
		if err != nil {
			t.Fatal(err)
		}
		want[i] = r.SVG
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for rep := 0; rep < 3; rep++ {
				for i, s := range srcs {
					r, _ := Render(s)
					if r.SVG != want[i] {
						mu.Lock()
						t.Errorf("그림 %d 의 결과가 다름", i)
						mu.Unlock()
						return
					}
				}
			}
		}()
	}
	wg.Wait()
}

// 계산 시간은 상한 안에서 묶인다: 상한 근처의 큰 그림(무작위·긴 사슬·subgraph 30개)도 각 1초 안,
// 배치 점 상한을 넘는 그림은 배치 전에 멈춘다. 원문을 한 번만 훑는다 — 최대 크기의 이상한 원문도 바로 끝난다.
func TestBounded(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	gen := map[string]func(*strings.Builder){
		"무작위 150·300": func(b *strings.Builder) {
			for i := 0; i < 300; i++ {
				fmt.Fprintf(b, "n%d[노드 %d] -->|간선 %d| n%d\n", rng.Intn(150), i, i, rng.Intn(150))
			}
		},
		"subgraph 30": func(b *strings.Builder) {
			for c := 0; c < 30; c++ {
				fmt.Fprintf(b, "subgraph s%d [묶음 %d]\n", c, c)
				for k := 0; k < 5; k++ {
					fmt.Fprintf(b, "c%d_%d --> c%d_%d\n", c, k, c, (k+1)%5)
				}
				b.WriteString("end\n")
			}
			for i := 0; i < 150; i++ {
				fmt.Fprintf(b, "c%d_%d -->|x| c%d_%d\n", rng.Intn(30), rng.Intn(5), rng.Intn(30), rng.Intn(5))
			}
		},
		"점 상한 바로 아래": func(b *strings.Builder) { // 노드 150 + 사슬 더미 149 + 긴 간선 46개의 더미 11546 = 11845
			for i := 0; i < 149; i++ {
				fmt.Fprintf(b, "n%d --> n%d\n", i, i+1)
			}
			for i := 0; i < 46; i++ {
				fmt.Fprintf(b, "n0 -->|라벨 %d| n%d\n", i, 149-i)
			}
		},
		"긴 사슬(점 상한 넘음)": func(b *strings.Builder) {
			for i := 0; i < 149; i++ {
				fmt.Fprintf(b, "n%d ----> n%d\n", i, i+1)
			}
			for i := 0; i < 151; i++ {
				fmt.Fprintf(b, "n0 -->|라벨| n%d\n", 149-i%150)
			}
		},
	}
	for name, f := range gen {
		var b strings.Builder
		b.WriteString("flowchart TD\n")
		f(&b)
		var r Result
		var err error
		el, ok := timingtest.Within(time.Second, func() { r, err = Render(b.String()) }) // CPU 시간, 넘으면 다시 잰다(itda-hyve#201)
		t.Logf("%s: %v, SVG %dKB, 오류 %v", name, el, len(r.SVG)>>10, err)
		if !ok {
			t.Errorf("%s: %v", name, el)
		}
		if (err != nil) != strings.Contains(name, "넘음") {
			t.Errorf("%s: 오류 %v", name, err)
		}
		if err == nil && len(r.SVG) > 1<<20 {
			t.Errorf("%s: SVG %d 바이트", name, len(r.SVG))
		}
	}
	for name, src := range map[string]string{
		"열린 [":     "graph TD\nA[" + strings.Repeat("[(", MaxSource/2-20),
		"따옴표":      "graph TD\nA[\"" + strings.Repeat("a", MaxSource-20),
		"화살표 글":    "graph TD\nA -- " + strings.Repeat("- ", MaxSource/2-20),
		";만":       "graph TD\n" + strings.Repeat(";", MaxSource-20),
		"<br 반복":   "graph TD\nA[" + strings.Repeat("<br", MaxSource/3-20) + "]",
		"엔티티 반복":   "graph TD\nA[" + strings.Repeat("#123", MaxSource/4-20) + "]",
		"주석 줄":     "graph TD\n" + strings.Repeat("%%\n", MaxSource/3-20) + "A",
		"subgraph": "graph TD\n" + strings.Repeat("subgraph\nend\n", MaxSource/13-2),
	} {
		if el, ok := timingtest.Within(100*time.Millisecond, func() { _, _ = Render(src) }); !ok {
			t.Errorf("%s: %v", name, el)
		}
	}
}

// ---- 결과 SVG 의 허용 목록(구현과 따로 적는다 — mdoc/security_test.go 의 목록과 같은 뜻)

var (
	okElems = map[string]bool{"svg": true, "g": true, "rect": true, "path": true, "line": true, "circle": true, "text": true, "tspan": true, "title": true}
	numRe   = regexp.MustCompile(`^-?\d+(?:\.\d)?$`)
	okAttrs = map[string]*regexp.Regexp{
		"viewBox": regexp.MustCompile(`^0 0 \d+(?:\.\d)? \d+(?:\.\d)?$`), "width": numRe, "height": numRe,
		"class": regexp.MustCompile(`^(?:diagram-svg|clusters|edges|heads|elabels|nodes|cl-box|cl-title|edge|edge dotted|edge thick|head|el-bg|el|node|node rim|node-in|nl)$`),
		"role":  regexp.MustCompile(`^img$`), "aria-label": nil,
		"x": numRe, "y": numRe, "rx": numRe, "x1": numRe, "y1": numRe, "x2": numRe, "y2": numRe, "cx": numRe, "cy": numRe, "r": numRe,
		"d":           regexp.MustCompile(`^[MLCAZ0-9 .-]+$`),
		"text-anchor": regexp.MustCompile(`^middle$`), "font-size": numRe,
	}
)

func checkSVG(t *testing.T, svg string) {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(svg))
	depth := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("SVG 가 XML 이 아님: %v\n%s", err, svg)
		}
		switch tk := tok.(type) {
		case xml.StartElement:
			depth++
			if !okElems[tk.Name.Local] || tk.Name.Space != "" {
				t.Errorf("허용하지 않은 요소 %s", tk.Name.Local)
			}
			for _, a := range tk.Attr {
				re, ok := okAttrs[a.Name.Local]
				if !ok || a.Name.Space != "" {
					t.Errorf("<%s> 의 허용하지 않은 속성 %s=%q", tk.Name.Local, a.Name.Local, a.Value)
					continue
				}
				if re != nil && !re.MatchString(a.Value) {
					t.Errorf("<%s> %s 값 꼴이 아님 %q", tk.Name.Local, a.Name.Local, a.Value)
				}
			}
		case xml.EndElement:
			depth--
		case xml.ProcInst, xml.Directive:
			t.Errorf("처리 명령·지시 %v", tk)
		}
	}
	if depth != 0 {
		t.Errorf("요소가 닫히지 않음")
	}
}

func TestSVGAllowlist(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	for i := 0; i < 100; i++ {
		r, err := Render(randGraph(rng, 2+rng.Intn(25), rng.Intn(40), i%2 == 0))
		if err != nil {
			t.Fatal(err)
		}
		checkSVG(t, r.SVG)
	}
	// 접근성 값의 맨 <·>·& 는 원문이다(정책 B — 원본 HTML 정제가 글을 바꿀 수 있다) — 엔티티로 적은 같은 글자가 이스케이프되는지 본다.
	r, err := Render("---\ntitle: <t>\n---\ngraph TD\naccTitle: a\"#lt;b#gt;\naccDescr: #lt;/title#gt;#lt;script#gt;x#lt;/script#gt;\nA[\"<script>&amp;</script>\"] -->|\"<img src=x>\"| B[#lt;i#gt;]\nsubgraph s [\"<s>\"]\nC\nend")
	if err != nil {
		t.Fatal(err)
	}
	checkSVG(t, r.SVG)
	for _, bad := range []string{"<script", "<img", "<i>", "<s>", "<b>", "</title><script"} {
		if strings.Contains(r.SVG, bad) {
			t.Errorf("SVG 에 %q", bad)
		}
	}
}

// 리뷰 R2: XML 이 허용하지 않는 글자는 어디서 생기든(원문·Mermaid 엔티티·YAML 이스케이프·accDescr 블록) SVG 에 나가지 않는다 —
// 거르는 곳은 SVG 를 쓰는 경계(esc) 하나다. 정책 B(받은 글자 = 출력 글자) 뒤로는 그런 글자가 든 그림은 그리지 않고 그 줄의
// 원문 + 경고다 — 엔티티는 정책(Label·AccSet)이, 원문·YAML 글자는 개별 검사·출력 관문(gate)이 막는다.
func TestXMLChars(t *testing.T) {
	for in, want := range map[string]string{
		"보통 글 ABC 😀 \U0010FFFF": "보통 글 ABC 😀 \U0010FFFF",
		"탭\t줄\n복귀\r":            "탭\t줄\n복귀\r",
		"\x01\x08\x0b\x1f\x7f":  "\uFFFD\uFFFD\uFFFD\uFFFD\x7f", // 0x7f 는 XML 1.0 Char 다
		"\uFFFE\uFFFF\uFFFD":    "\uFFFD\uFFFD\uFFFD",
		"잘못된\xff바이트":            "잘못된\uFFFD바이트",
		"\xed\xa0\x80":          "\uFFFD", // 서로게이트의 UTF-8(잘못된 바이트 묶음은 하나로)
	} {
		if got := xmlChars(in); got != want {
			t.Errorf("xmlChars(%q) = %q, 기대 %q", in, got, want)
		}
	}
	for src, line := range map[string]int{
		"flowchart TD\nA[\"#65535;\"] --> B":                                                 2,
		"flowchart TD\nA -->|#65534;| B":                                                     2,
		"---\ntitle: \"\\uFFFF\"\n---\nflowchart TD\nA --> B":                                2,
		"flowchart TD\naccDescr {\n\x01 설명\x02\n}\nA --> B":                                  4,
		"---\ntitle: \"a\\x01b\\uFFFEc\"\n---\nflowchart TD\naccTitle: \uFFFF\nA --> B":      2,
		"flowchart TD\nsubgraph s [\"#65535; 묶음\"]\nA[\"\uFFFE\"] -- \"#65534;\" --> B\nend": 2,
		"flowchart TD\naccDescr: #65535;\nA":                                                 2,
		"flowchart TD\nA[\"x\uFFFFy\"]":                                                      2,
		"flowchart TD\nA\naccTitle: x\uFFFEy":                                                3,
	} {
		_, err := Render(src)
		var de *Error
		if !errors.As(err, &de) || de.Line != line {
			t.Errorf("%q: %d 번째 줄 원문을 기대, %v", src, line, err)
		}
	}
}

// 무작위 속성(리뷰 R2): 위험한 코드 포인트를 원문 글자·Mermaid 엔티티·YAML 이스케이프로 라벨·화살표 글·subgraph 제목·
// title·accTitle·accDescr(한 줄·블록) 어디에나 섞은 그림 3000개 — 그리면 SVG 는 늘 XML 로 파싱되고 캡션도 깨끗하다.
func TestRandomXMLValid(t *testing.T) {
	rng := rand.New(rand.NewSource(1172))
	risky := []rune{0x0, 0x1, 0x8, 0xb, 0xc, 0x1f, 0x7f, 0x85, 0xd7ff, 0xe000, 0xfffd, 0xfffe, 0xffff, 0x10000, 0x10ffff, '<', '&', '"', '\'', 'a', '가'}
	piece := func(yaml bool) string {
		var b strings.Builder
		for k := 0; k < 1+rng.Intn(4); k++ {
			r := risky[rng.Intn(len(risky))]
			switch rng.Intn(3) {
			case 0: // 엔티티
				fmt.Fprintf(&b, "#%d;", r)
			case 1: // YAML 이스케이프(title 안에서만 뜻이 있다 — 그 밖에서는 글자 그대로)
				if yaml {
					fmt.Fprintf(&b, "\\U%08X", r)
				} else {
					b.WriteString("x")
				}
			default: // 원문 글자(줄바꿈·따옴표처럼 문법을 깨는 것은 빼고)
				if r >= 0x20 && r != '"' {
					b.WriteRune(r)
				} else {
					b.WriteString("y")
				}
			}
		}
		return b.String()
	}
	drawn := 0
	for it := 0; it < 3000; it++ {
		var b strings.Builder
		if rng.Intn(2) == 0 {
			fmt.Fprintf(&b, "---\ntitle: \"%s\"\n---\n", piece(true))
		}
		b.WriteString("flowchart LR\n")
		if rng.Intn(2) == 0 {
			fmt.Fprintf(&b, "accTitle: %s\n", piece(false))
		}
		if rng.Intn(2) == 0 {
			fmt.Fprintf(&b, "accDescr {\n%s\n%s\n}\n", piece(false), string(risky[rng.Intn(len(risky))]))
		}
		fmt.Fprintf(&b, "subgraph s [\"%s\"]\nA[\"%s\"] -->|%s| B(%s)\nend\nB -->|%s| B\n", piece(false), piece(false), piece(false), piece(false), piece(false))
		src := b.String()
		r, err := Render(src)
		if err != nil {
			continue // 문법을 깬 조각(예: 엔티티가 만든 | ) — 원문 + 경고 쪽이다
		}
		drawn++
		checkSVG(t, r.SVG)
		if xmlChars(r.Caption) != r.Caption {
			t.Fatalf("캡션에 XML 금지 글자: %q", src)
		}
		if t.Failed() {
			t.Fatalf("원문: %q", src)
		}
	}
	// 정책 B(받은 글자 = 출력 글자) 뒤로는 위험 코드 포인트(엔티티·원문 U+FFFE·제어 문자)가 든 그림 대부분이 원문이다 — 이
	// 씨앗에서 3000판 가운데 55판을 그린다(원문 U+0085 는 ASCII 밖 공백이라 줄째 원문 — itda-hyve#409 ④-2 후속 수정 1). 그린 것은 모두 XML
	// 로 읽혀야 한다.
	if drawn < 40 {
		t.Errorf("그린 그림이 %d 개뿐 — 생성기가 문법을 너무 자주 깸", drawn)
	}
	if rejected := 3000 - drawn; rejected == 0 {
		t.Errorf("원문으로 돌린 그림이 없음 — 정책 B 가 위험 글자를 막지 않음")
	}
}

// FuzzRender 는 어떤 원문에도 패닉하지 않고, 그리면 허용 목록의 SVG 이며 두 번 그려도 같은지 본다.
//
//	go test ./diagram/flowchart -run '^$' -fuzz FuzzRender -fuzztime 60s
func FuzzRender(f *testing.F) {
	rng := rand.New(rand.NewSource(11))
	for i := 0; i < 20; i++ {
		f.Add(randGraph(rng, 2+rng.Intn(8), rng.Intn(12), i%2 == 0))
	}
	for _, s := range []string{"graph TD\nA-->B", "graph LR\nA[가<br>나] -.->|x| B((원))\nB --> B", "flowchart RL\nsubgraph s\nA\nend\nA --- C",
		"graph\nA & B --> C", "---\ntitle: t\n---\ngraph BT\nA", "graph TD\nA -- t --> B -. u .-> C == v ==> D", "graph TD\nA ~~~ B",
		// 리뷰 R2: 디코딩 뒤에 생기는 XML 금지 글자
		"flowchart TD\nA[\"#65535;\"] --> B", "flowchart TD\nA -->|#65534;| B", "---\ntitle: \"\\uFFFF\"\n---\nflowchart TD\nA --> B",
		"flowchart TD\naccDescr {\n\x01\n}\nA --> B", "---\ntitle: \"\\x01\"\n---\ngraph TD\naccTitle: #65535;\nA"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		r, err := Render(src)
		if err != nil {
			var fe *Error
			if !errors.As(err, &fe) {
				t.Fatalf("*Error 가 아닌 오류 %T", err)
			}
			return
		}
		checkSVG(t, r.SVG)
		r2, _ := Render(src)
		if r2.SVG != r.SVG {
			t.Fatal("두 번 그린 결과가 다름")
		}
	})
}

// 무작위 그림 300개의 SVG 묶음의 해시를 testdata 에 고정한다 — 다른 아키텍처(GOARCH=amd64, arm64 의 곱셈-덧셈 융합 여부)와
// 다른 기계에서도 같은 출력인지 본다. 출력을 일부러 바꾸면 FC_UPDATE=1 로 다시 쓴다.
func TestCorpusHash(t *testing.T) {
	rng := rand.New(rand.NewSource(2026))
	h := sha256.New()
	for i := 0; i < 300; i++ {
		r, err := Render(randGraph(rng, 2+rng.Intn(25), rng.Intn(40), i%2 == 0))
		if err != nil {
			t.Fatal(err)
		}
		io.WriteString(h, r.SVG)
	}
	got := hex.EncodeToString(h.Sum(nil))
	path := filepath.Join("testdata", "corpus.sha256")
	if os.Getenv("FC_UPDATE") != "" {
		_ = os.MkdirAll("testdata", 0o755)
		if err := os.WriteFile(path, []byte(got+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(want)) != got {
		t.Errorf("그림 묶음 해시 %s, 고정 %s (%s)", got, strings.TrimSpace(string(want)), runtime.GOARCH)
	}
}

// 설정 지시 — 줄 전체가 지시 하나인 줄만 무시하고 그린다. 그 밖에 %%{ 가 든 줄(지시 앞뒤·사이의 글, acc 값·노드 글·
// 간선 글·주석 안, accDescr 블록 안, subgraph 안의 그런 줄)은 원문 + 그 줄 번호(er·sequence 와 같은 공통 규칙 — itda-hyve#409 ④-1 후속).
func TestDirectiveLines(t *testing.T) {
	const d = "%%{init: {}}%%"
	const one = "한 줄에 하나만"
	for _, c := range []struct {
		src  string
		line int
		msg  string
	}{
		{"flowchart TD\n" + d + " A --> X " + d + "\nB", 2, one},
		{"flowchart TD\n" + d + " ??? " + d + "\nB", 2, one},
		{"flowchart TD\n" + d + d + "\nB", 2, one},
		{"flowchart TD\naccTitle: 앞 " + d + " 뒤\nB", 2, one},
		{"flowchart TD\naccDescr: 앞 " + d + " 뒤\nB", 2, one},
		{"flowchart TD\nA[글 " + d + "]", 2, one},
		{"flowchart TD\nA -->|글 " + d + "| B", 2, one},
		{"flowchart TD\nA -- 글 " + d + " --> B", 2, one},
		{"flowchart TD\nA --> B " + d, 2, one},
		{"flowchart TD\nA --> B; " + d, 2, one},
		{"flowchart TD\n" + d + "; A --> B", 2, one},
		{"flowchart TD\n%% 메모 " + d + "\nB", 2, one},
		{"flowchart TD\nsubgraph s [제목 " + d + "]\nA\nend", 2, one},
		{"flowchart TD\nsubgraph s\n" + d + " A\nend", 3, one},
		{"flowchart TD\naccDescr {\n" + d + "\n}\nB", 3, "블록 안"},
		{"flowchart TD\naccDescr {\n글 " + d + "\n}\nB", 3, one},
		{"flowchart TD\n%%{init: {\nB", 2, "한 줄로 닫아야"},
	} {
		_, err := Render(c.src)
		var fe *Error
		if !errors.As(err, &fe) {
			t.Errorf("%q: 오류가 아님(%v)", c.src, err)
			continue
		}
		if fe.Line != c.line || !strings.Contains(fe.Msg, c.msg) {
			t.Errorf("%q: %d줄 %q, 기대 %d줄 …%s…", c.src, fe.Line, fe.Msg, c.line, c.msg)
		}
	}
	for _, src := range []string{d + "\nflowchart TD\nA", "flowchart TD\n  " + d + "  \nA", "flowchart TD\n%%{}%%\nA",
		"flowchart TD\nsubgraph s\n" + d + "\nA\nend", "flowchart TD\n" + d + "\n" + d + "\nA"} {
		r, err := Render(src)
		if err != nil {
			t.Errorf("%q: %v", src, err)
			continue
		}
		if len(r.Issues) != 1 || !strings.Contains(r.Issues[0].Msg, "설정 지시") || strings.Contains(r.SVG, "%%{") {
			t.Errorf("%q: 경고 %+v", src, r.Issues)
		}
	}
}

// 줄 머리 %% 주석 — 원본 cleanupComments 처럼 accDescr { … } 블록 안에서도 지운다(설명에 넣지 않는다). 블록 밖 주석은 그대로
// 건너뛰고, 뒤에 글이 없는 맨 %% 줄은 원문 + 그 줄 번호(er 와 같은 차례·규칙 — itda-hyve#409 ④-1 후속 리뷰 1회차 [2]).
func TestCommentLines(t *testing.T) {
	const head, tail = "flowchart TD\n", "\nA"
	for _, c := range []struct{ src, desc string }{
		{head + "accDescr {\n설명\n%% 메모\n}" + tail, "설명"},
		{head + "accDescr {\n  %% 메모 }\n설명\n}" + tail, "설명"}, // 주석 줄은 줄째 지운다 — 안의 } 로 닫지 않는다
		{head + "accDescr { 앞\n%%  \n뒤 }" + tail, "앞\n뒤"},    // %% 뒤 공백만 있어도 원본 정규식은 지운다(블록 줄바꿈은 그대로 — 정책 B)
	} {
		r, err := Render(c.src)
		if err != nil {
			t.Errorf("%q: %v", c.src, err)
			continue
		}
		if !strings.Contains(r.SVG, "<title>"+c.desc+"</title>") || strings.Contains(r.SVG, "메모") || strings.Contains(r.SVG, "%%") {
			t.Errorf("%q: 설명이 %q 가 아니거나 주석이 남음\n%s", c.src, c.desc, r.SVG)
		}
	}
	if r, err := Render(head + "%% 밖 주석" + tail); err != nil || len(r.Issues) != 0 {
		t.Errorf("블록 밖 주석: %v %+v", err, r.Issues)
	}
	for _, c := range []struct {
		src  string
		line int
	}{{head + "%%" + tail, 2}, {head + "  %%\r" + tail, 2}, {head + "accDescr {\n설명\n%%\n}" + tail, 4}} {
		_, err := Render(c.src)
		var de *Error
		if !errors.As(err, &de) || de.Line != c.line || !strings.Contains(de.Msg, "글 없는") {
			t.Errorf("%q: 오류 %v, 기대 %d줄 글 없는 %%%% 줄", c.src, err, c.line)
		}
	}
}
