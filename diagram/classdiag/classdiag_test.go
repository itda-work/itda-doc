package classdiag

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/itda-work/itda-doc/diagram"
	"github.com/itda-work/itda-doc/diagram/layered"
)

// mermaid 그대로의 예제 문법 — 모르는 것은 *Error(줄 번호), 쓰지 않는 것은 Issue(경고)다.

// errAt 는 원문을 그려 *Error 의 줄·메시지 일부를 견준다(Render — 원문 상한 검사까지 지나간다).
func errAt(t *testing.T, src string, line int, msg string) {
	t.Helper()
	_, err := Render(src)
	var de *diagram.Error
	if !errors.As(err, &de) {
		t.Fatalf("Render(%q) 오류가 *Error 가 아님: %v", src, err)
	}
	if de.Line != line || !strings.Contains(de.Msg, msg) {
		t.Errorf("Render(%q) 오류 %d:%q, 기대 %d 에 %q", src, de.Line, de.Msg, line, msg)
	}
}

// drawn 은 원문을 그려 SVG·경고를 얻는다.
func drawn(t *testing.T, src string) (string, []diagram.Issue) {
	t.Helper()
	r, err := Render(src)
	if err != nil {
		t.Fatalf("Render(%q) 실패: %v", src, err)
	}
	return r.SVG, r.Issues
}

// memSummary 는 멤버 하나를 「가시성 분류자:글」로 적는다(가시성이 없으면 .).
func memSummary(vis rune, cls Classifier, text string) string {
	if vis == 0 {
		vis = '.'
	}
	return fmt.Sprintf("%c%d:%s", vis, cls, text)
}

// summary 는 읽은 그림을 한 줄 글로 줄인다 — 클래스(이름<제네릭 인자>«주석»{속성;…|메서드;…}) · 관계(왼>오른:모양1:모양2:선"라벨"[수량1,수량2])
// · 방향 · 접근성. 판정기(judge_test.go jSummary)도 같은 꼴로 적는다.
func summary(c *chart) string {
	var b strings.Builder
	fmt.Fprintf(&b, "dir=%d", c.Dir)
	for _, cl := range c.Classes {
		b.WriteString(" " + cl.Name)
		if cl.Type != "" {
			b.WriteString("<" + cl.Type + ">")
		}
		if cl.Annot != "" {
			b.WriteString("«" + cl.Annot + "»")
		}
		b.WriteString("{")
		for i, ms := range [][]member{cl.Attrs, cl.Methods} {
			if i == 1 {
				b.WriteString("|")
			}
			for k, m := range ms {
				if k > 0 {
					b.WriteString(";")
				}
				if m.Method != (i == 1) {
					b.WriteString("!종류")
				}
				b.WriteString(memSummary(m.Vis, m.Cls, m.Text))
			}
		}
		b.WriteString("}")
	}
	if len(c.Rels) > 0 {
		b.WriteString(" |")
	}
	for _, r := range c.Rels {
		line := "--"
		if r.Dotted {
			line = ".."
		}
		fmt.Fprintf(&b, " %s>%s:%s:%s:%s", c.Classes[r.From].Name, c.Classes[r.To].Name, r.Type1, r.Type2, line)
		if r.Label != "" {
			b.WriteString(`"` + r.Label + `"`)
		}
		if r.Card1 != "" || r.Card2 != "" {
			b.WriteString("[" + r.Card1 + "," + r.Card2 + "]")
		}
	}
	if c.AccTitle != "" || c.AccDescr != "" {
		b.WriteString(" | acc=" + c.AccTitle + "/" + c.AccDescr)
	}
	return b.String()
}

func parsed(t *testing.T, src string) string {
	t.Helper()
	c, _, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return summary(c)
}

func TestParseStatements(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		// 관계 — 모양 여섯 가지 양쪽, 실선·점선, 라벨, 양끝 수량 글. 클래스는 처음 나온 차례(왼끝 다음 오른끝)
		{"A <|-- B", "dir=0 A{|} B{|} | A>B:extension:none:--"},
		{"A --|> B", "dir=0 A{|} B{|} | A>B:none:extension:--"},
		{"A *-- B\nC o-- D\nE <.. F\nG ..> H", "dir=0 A{|} B{|} C{|} D{|} E{|} F{|} G{|} H{|} | A>B:composition:none:-- C>D:aggregation:none:-- " +
			"E>F:dependency:none:.. G>H:none:dependency:.."},
		{"A <|--|> B\nA *--o B\nA <--> B", "dir=0 A{|} B{|} | A>B:extension:extension:-- A>B:composition:aggregation:-- A>B:dependency:dependency:--"},
		{"A -- B : 잇는다\nA .. B", "dir=0 A{|} B{|} | A>B:none:none:--\"잇는다\" A>B:none:none:.."},
		{"고객 \"1\" --> \"*\" 주문 : 주문한다", "dir=0 고객{|} 주문{|} | 고객>주문:none:dependency:--\"주문한다\"[1,*]"},
		{"A \"0..1\" -- B\nA -- \" 1..* \" B", "dir=0 A{|} B{|} | A>B:none:none:--[0..1,] A>B:none:none:--[,1..*]"},
		{"Ao--B\nA o--B\nA --o B", "dir=0 Ao{|} B{|} A{|} | Ao>B:none:none:-- A>B:aggregation:none:-- A>B:none:aggregation:--"},
		{"A --> A : 자기", "dir=0 A{|} | A>A:none:dependency:--\"자기\""},
		// 클래스 문장·몸·한 줄 몸·주석
		{"class 고객", "dir=0 고객{|}"},
		{"class 고객 {\n  +이름 String\n  -전화$\n  +주문하기(상품) 주문\n  +합계()* \n}", "dir=0 고객{+0:+이름 String;-1:-전화|+0:+주문하기(상품) : 주문;+2:+합계()}"},
		{"class A {}\nclass B { }\nclass C { +x }", "dir=0 A{|} B{|} C{+0:+x|}"},
		{"class A {\n<<interface>>\n}\nclass B <<service>>\n<<abstract>> A2\nclass A2", ""},
		{"class A2\n<<abstract>> A2\nclass B <<service>>\nclass C {\n<<interface>>\n+f()\n}", "dir=0 A2«abstract»{|} B«service»{|} C«interface»{|+0:+f()}"},
		// 이름 : 멤버 — 가시성 넷, 분류자, 메서드 판정(괄호 짝), 「Preserve one space only」
		{"A : +a\nA : -b\nA : #c\nA : ~d\nA : e$\nA : f*\nA : + g", "dir=0 A{+0:+a;-0:-b;#0:#c;~0:~d;.1:e;.2:f;+0:+ g|}"},
		{"A : f() int\nA : g(a, b) List$\nA : h()$\nA : + i( x )", "dir=0 A{|.0:f() : int;.1:g(a, b) : List;.1:h();+0:+ i(x)}"},
		{"A : 값 = 1\nA : 경로/이름\nA : #35;태그\nA : x#59;y", "dir=0 A{.0:값 = 1;.0:경로/이름;.0:#태그;.0:x;y|}"},
		// 방향(마지막이 이긴다), 접근성, 머리 v2
		{"direction LR\nA\ndirection RL", ""},
		{"direction LR\nclass A\ndirection  RL", "dir=3 A{|}"},
		{"accTitle: 제목\naccDescr {\n  둘째\n}\nclass A", "dir=0 A{|} | acc=제목/둘째"},
		// 제네릭 — 이름(한 겹, 인자 쉼표 목록). `A~T~` 와 `A` 는 같은 클래스, 인자는 처음 만든 자리의 것(원본 addClass)
		{"class 목록~T~", "dir=0 목록<T>{|}"},
		{"class 목록~T~\n목록~T~ <|-- 구현\n목록 : +크기() int", "dir=0 목록<T>{|+0:+크기() : int} 구현{|} | 목록>구현:extension:none:--"},
		{"class A\nA~T~ --> B~U~\nclass B~V~\nB~W~ : +x", "dir=0 A{|} B<U>{+0:+x|} | A>B:none:dependency:--"},
		{"A~T~ \"1\" --> \"*\" B~K, V~ : 담는다\nA~X~ .. B", "dir=0 A<T>{|} B<K, V>{|} | A>B:none:dependency:--\"담는다\"[1,*] A>B:none:none:.."},
		{"A~T~o--B\nA~T~--o B1", "dir=0 A<T>{|} B{|} B1{|} | A>B:aggregation:none:-- A>B1:none:aggregation:--"},
		{"class Map~K,V~ {\n+목록 List~int~\n+찾기(키 List~K~) Map~K, V~\n}", "dir=0 Map<K,V>{+0:+목록 List<int>|+0:+찾기(키 List<K>) : Map<K, V>}"},
		{"class A~T~ <<interface>>\nclass B~가나_다~:::hot\nclass C~T1, T2~ { +x }\n<<service>> B~Z~", "dir=0 A<T>«interface»{|} B<가나_다>«service»{|} C<T1, T2>{+0:+x|}"},
		{"class 고객~linker~\nclass 상품~notes, hrefs~", "dir=0 고객<linker>{|} 상품<notes, hrefs>{|}"},
		// 제네릭 — 멤버(이름·인자·반환형마다 원본 parseGenericTypes, 가시성 ~ 와 함께, 중첩, 분류자)
		{"A : ~목록 List~int~\nA : 행렬 Array~Array~int~~$\nA : +f(a List~T~, b Map~K, V~) Map~K, V~*\nA : ~g~T~(x) T",
			"dir=0 A{~0:~목록 List<int>;.1:행렬 Array<Array<int>>|+2:+f(a List<T>, b Map<K, V>) : Map<K, V>;~0:~g<T>(x) : T}"},
		{"A : 값 List~#35;~\nA : #35;~T~ x", "dir=0 A{.0:값 List<#>;.0:#<T> x|}"},
	} {
		if c.want == "" {
			continue
		}
		if got := parsed(t, "classDiagram\n"+c.src); got != c.want {
			t.Errorf("%q\n얻음 %s\n기대 %s", c.src, got, c.want)
		}
	}
	if got := parsed(t, "classDiagram-v2\nclass A"); got != "dir=0 A{|}" {
		t.Errorf("v2 머리: %s", got)
	}
	// 거부 — 줄 번호와 까닭
	for _, c := range []struct {
		src  string
		line int
		msg  string
	}{
		{"class A\n<<x>> B", 3, "아직 없음"},
		{"class A\nclass A <<x>>\nclass A <<y>>", 4, "두 번"},
		// 제네릭 — 받지 않는 꼴
		{"class 목록~List~int~~", 2, "class 문장은"},
		{"class A~T", 2, "class 문장은"},
		{"class A~T~B", 2, "class 문장은"},
		{"class A~~", 2, "비어 있음"},
		{"class A~link~", 2, "키워드"},
		{"class A~note, T~", 2, "키워드"},
		{"class A~ T~", 2, "받지 않는 글자"},
		{"class A~K,,V~", 2, "꼴이 다름"},
		{"class A~K, ~", 2, "꼴이 다름"},
		{"class A~K,  V~", 2, "받지 않는 글자"},
		{"class A~T-1~", 2, "받지 않는 글자"},
		{"class A~T_~", 2, "_ 가"},
		{"class A~𝔸~", 2, "받지 않는 글자"},
		{"o~T~ --> B", 2, "키워드"},
		{"A~T~ <|-- B~", 2, "이 자리의 제네릭"},
		{"click A~T~ href \"u\"", 2, "이 자리의 제네릭"},
		{"cssClass \"A~T~\" hot", 2, "classDef"},
		{"A : a~T~ b~U~", 2, "parseGenericTypes"},
		{"A : Map~K, List~V~ x~", 2, "parseGenericTypes"},
		{"A : List~int~~", 2, "parseGenericTypes"},
		{"A : m Map~K, V, W~", 2, "parseGenericTypes"},    // 원본은 쉼표 조각을 ~ 가 하나씩인 앞뒤 둘만 다시 잇는다 — 셋이면 ~ 가 남는다
		{"A : m Map~K, List~V~~", 2, "parseGenericTypes"}, // 원본은 Map~K, List<V~> 로 읽는다
		{"A : m List~Map~K, V~~", 2, "parseGenericTypes"}, // 원본은 List<Map>K, V~~ 쪽으로 읽는다
		{"A : List~~", 2, "parseGenericTypes"},
		{"A : x~T~y", 2, "parseGenericTypes"},
		{"A : ~T~ x", 2, "parseGenericTypes"},
		{"A : f(x~) T", 2, "parseGenericTypes"},
		{"A : a~b", 2, "parseGenericTypes"},
		{"A : List~T~.x", 2, "parseGenericTypes"},
		{"namespace N {\nclass A\n}", 2, "namespace"},
		{"note \"메모\"", 2, "메모"},
		{"note for A \"메모\"", 2, "메모"},
		{"class A[\"라벨\"]", 2, "class 문장은"},
		{"A --() B", 2, "lollipop"},
		{"A --oB", 2, "o 바로 뒤"},
		{"가o--B", 2, "키워드"},
		{"click --> B", 2, "키워드"},
		{"A --> link", 2, "키워드"},
		{"accTitleX --> B", 2, "accTitle"},
		{"A_ --> B", 2, "_ 가"},
		{"A --> B : x : y", 2, ": 를"},
		{"A --> B : *강조*", 2, "*"},
		{"A --> B : - 목록", 2, "목록"},
		{"A \"none\" --> B", 2, "none"},
		{"A \"" + strings.Repeat("1", 21) + "\" --> B", 2, "20 자"},
		{"A \"1\" --> A", 2, "자기 관계"},
		{"A : foo() : int", 2, ": 를"},
		{"class A {\nfoo() : int\n}", 3, ": 를"},
		{"A : foo()$ int", 2, ") 뒤"},
		{"A : f(()", 2, "한 쌍"},
		{"A : (x)", 2, "비어 있음"},
		{"A : +(x)", 2, "비어 있음"},
		{"A : x$y", 2, "분류자"},
		{"A : a+b", 2, "첫 글자"},
		{"A : 1. 첫째", 2, "「수.」"},
		{"A : = 선", 2, "="},
		{"A : a  b", 2, "연속 빈칸"},
		{"A :", 2, "비어 있음"},
		{"class A {\n<<가>>\n}", 3, "한 낱말"},
		{"class A {\nx }", 3, "{ }"},
		{"class A {\ndirection LR\n}", 3, "direction"},
		{"class A {\n%%\n}", 3, "%%"},
		{"class A {", 2, "닫히지"},
		{"direction lr", 2, "direction"},
		{"Direction LR", 2, "direction"},
		{"A --> B %% 주석", 2, "줄 중간"},
		{"A --> B; B --> C", 2, ";"},
		{"}", 2, "짝 없는"},
		{"A", 2, "알 수 없는"},
		{"classDef 강조 fill:#f9f", 2, "classDef"},
		{"style A fill:#f9f;stroke:#333", 2, "classDef"},
		{"A <| -- B", 2, "관계 꼴"},
		{"accTitle:", 2, "글이 없음"},
	} {
		errAt(t, "classDiagram\n"+c.src, c.line, c.msg)
	}
	errAt(t, "classDiagramX\nclass A", 1, "그림 종류")
	errAt(t, "classdiagram\nclass A", 1, "그림 종류")
	errAt(t, "classDiagram\n%% 주석", 0, "그릴 클래스")
	errAt(t, "%% 주석", 0, "비어 있음")
}

func TestIgnoredLines(t *testing.T) {
	for _, c := range []struct {
		src  string
		line int
		msg  string
	}{
		{"%%{init: {\"theme\": \"dark\"}}%%\nclassDiagram\nclass A", 1, "설정 지시"},
		{"classDiagram\nclass A\nclassDef hot fill:#f9f,stroke:#333", 3, "classDef"},
		{"classDiagram\nclass A\nstyle A fill:#f9f", 3, "style"},
		{"classDiagram\nclass A\ncssClass \"A,B\" hot", 3, "cssClass"},
		{"classDiagram\nclass A:::hot", 2, "cssClass"},
		{"classDiagram\nclass A\nclick A href \"https://example.com\"", 3, "click"},
		{"classDiagram\nclass A\nclick A call 함수()", 0, ""},
		{"classDiagram\nclass A\nclick A call f()", 3, "click"},
		{"classDiagram\nclass A\nlink A \"https://example.com\"", 3, "click"},
		{"classDiagram\nclass A\ncallback A \"f\"", 3, "click"},
		{"---\nconfig:\n  theme: dark\n---\nclassDiagram\nclass A", 2, "frontmatter"},
	} {
		r, err := Render(c.src)
		if c.line == 0 {
			if err == nil {
				t.Errorf("%q: 거부를 기대", c.src)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: %v", c.src, err)
			continue
		}
		if len(r.Issues) != 1 || r.Issues[0].Line != c.line || !strings.Contains(r.Issues[0].Msg, c.msg) {
			t.Errorf("%q: 경고 %+v, 기대 %d:%s", c.src, r.Issues, c.line, c.msg)
		}
	}
	// 같은 종류의 경고는 한 번
	r, err := Render("classDiagram\nclass A\nstyle A fill:#f00\nstyle A fill:#0f0")
	if err != nil || len(r.Issues) != 1 {
		t.Errorf("같은 경고 한 번: %+v %v", r.Issues, err)
	}
}

func TestDirectiveLines(t *testing.T) {
	for _, c := range []struct {
		src  string
		line int
	}{
		{"classDiagram\nclass A %%{init: {}}%%", 2},
		{"classDiagram\nclass A {\n%%{init: {}}%%\n}", 3},
		{"classDiagram\naccDescr {\n%%{init: {}}%%\n}\nclass A", 3},
		{"classDiagram\naccTitle: %%{init: {}}%%\nclass A", 2},
		{"classDiagram\n%%{a}%% %%{b}%%\nclass A", 2},
	} {
		_, err := Render(c.src)
		var de *diagram.Error
		if !errors.As(err, &de) || de.Line != c.line {
			t.Errorf("%q: %d 줄 거부를 기대, %v", c.src, c.line, err)
		}
	}
	// 줄 머리 주석은 클래스 몸 안에서도 지운다(원본 전처리 cleanupComments)
	if got := parsed(t, "classDiagram\nclass A {\n  %% 메모\n  +x\n}"); got != "dir=0 A{+0:+x|}" {
		t.Errorf("몸 안 주석: %s", got)
	}
}

func TestLimits(t *testing.T) {
	var b strings.Builder
	b.WriteString("classDiagram\n")
	for i := 0; i <= MaxClasses; i++ {
		fmt.Fprintf(&b, "class C%d\n", i)
	}
	errAt(t, b.String(), MaxClasses+2, "클래스가")
	b.Reset()
	b.WriteString("classDiagram\n")
	for i := 0; i <= MaxRelations; i++ {
		fmt.Fprintf(&b, "A%d --> B%d\n", i%10, i%7)
	}
	errAt(t, b.String(), MaxRelations+2, "관계가")
	b.Reset()
	b.WriteString("classDiagram\nclass A {\n")
	for i := 0; i <= MaxMembers; i++ {
		fmt.Fprintf(&b, "+m%d\n", i)
	}
	b.WriteString("}")
	errAt(t, b.String(), MaxMembers+3, "멤버가")
	errAt(t, "classDiagram\nclass "+strings.Repeat("가", MaxLabel+1), 2, "넘음")
	if _, err := Render("classDiagram\nclass " + strings.Repeat("가", MaxLabel)); err != nil {
		t.Errorf("200 자 이름: %v", err)
	}
	// 제네릭 이름 — 상자 제목 「이름<인자>」가 200 자까지
	if _, err := Render("classDiagram\nclass " + strings.Repeat("가", MaxLabel-6) + "~T, U~"); err != nil {
		t.Errorf("200 자 제네릭 제목: %v", err)
	}
	errAt(t, "classDiagram\nclass "+strings.Repeat("가", MaxLabel-5)+"~T, U~", 2, "넘음")
	errAt(t, "classDiagram\nA : "+strings.Repeat("가", MaxLabel+1), 2, "넘음")
	errAt(t, "classDiagram\nA --> B : "+strings.Repeat("가", MaxLabel+1), 2, "넘음")
	if _, err := Render("classDiagram\nA \"" + strings.Repeat("가", MaxCard) + "\" --> B"); err != nil {
		t.Errorf("20 자 수량 글: %v", err)
	}
	_, err := Render("classDiagram\n" + strings.Repeat("%% "+strings.Repeat("x", 100)+"\n", MaxSource/100))
	if err == nil || !strings.Contains(err.Error(), "KB") {
		t.Errorf("원문 상한: %v", err)
	}
}

func TestDeterminism(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for k := 0; k < 30; k++ {
		src := randChart(rng, 3+rng.Intn(25), rng.Intn(40))
		a, err := Render(src)
		if err != nil {
			t.Fatalf("%v\n%s", err, src)
		}
		for i := 0; i < 3; i++ {
			b, _ := Render(src)
			if b.SVG != a.SVG {
				t.Fatalf("같은 입력의 출력이 다름\n%s", src)
			}
		}
	}
}

var (
	rNames   = []string{"고객", "주문", "주문항목", "결제수단", "카드결제", "상품", "A", "B", "Pay1", "x_y", "Đơn", "검토"}
	rMembers = []string{"+이름 String", "-전화$", "#보호 int", "~패키지", "+주문하기(상품) 주문", "+합계() decimal", "+결제(금액) bool*", "생성()$", "x"}
	rShapes  = []string{"", "", "<|", "|>", "*", "o", "<", ">"}
	rLabels  = []string{"", "", "", "주문한다", "담는다", "1단계 확인", "아주 긴 관계 라벨이 여러 줄로 나뉘어 그려진다 정말로 길다"}
	rCards   = []string{"", "", "", "1", "*", "0..1", "1..*", "많음"}
)

// randChart 는 n 개 클래스(멤버 몇 개씩)와 m 개 관계의 원문이다 — 자기 관계·같은 짝 여러 관계·양끝 모양·라벨·수량 글을 섞는다.
func randChart(rng *rand.Rand, n, m int) string {
	var b strings.Builder
	b.WriteString("classDiagram\n")
	b.WriteString([]string{"", "direction LR\n", "direction BT\n", "direction RL\n"}[rng.Intn(4)])
	name := func(i int) string { return fmt.Sprintf("%s%d", rNames[i%len(rNames)], i) }
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "class %s {\n", name(i))
		if rng.Intn(5) == 0 {
			b.WriteString("<<interface>>\n")
		}
		for k := rng.Intn(5); k > 0; k-- {
			b.WriteString(rMembers[rng.Intn(len(rMembers))] + "\n")
		}
		b.WriteString("}\n")
	}
	for k := 0; k < m; k++ {
		a, c := rng.Intn(n), rng.Intn(n)
		line := "--"
		if rng.Intn(3) == 0 {
			line = ".."
		}
		c1, c2 := rCards[rng.Intn(len(rCards))], rCards[rng.Intn(len(rCards))]
		if a == c {
			c1, c2 = "", ""
		}
		s := name(a)
		if c1 != "" {
			s += " \"" + c1 + "\""
		}
		s += " " + rShapes[rng.Intn(len(rShapes))] + line + rShapes[rng.Intn(len(rShapes))] + " "
		if c2 != "" {
			s += "\"" + c2 + "\" "
		}
		s += name(c)
		if l := rLabels[rng.Intn(len(rLabels))]; l != "" {
			s += " : " + l
		}
		b.WriteString(s + "\n")
	}
	return b.String()
}

type rect struct{ x0, y0, x1, y1 float64 }

func (r rect) overlaps(o rect, eps float64) bool {
	return r.x0 < o.x1-eps && o.x0 < r.x1-eps && r.y0 < o.y1-eps && o.y0 < r.y1-eps
}

func boxRect(c layered.Pt, w, h float64) rect {
	return rect{c.X - w/2, c.Y - h/2, c.X + w/2, c.Y + h/2}
}

// onBorder 는 점 p 가 상자 b(가운데 c)의 테두리 위인지다.
func onBorder(b box, c layered.Pt, p layered.Pt) bool {
	x0, y0, x1, y1 := c.X-b.w/2, c.Y-b.h/2, c.X+b.w/2, c.Y+b.h/2
	in := p.X >= x0-0.5 && p.X <= x1+0.5 && p.Y >= y0-0.5 && p.Y <= y1+0.5
	edge := math.Abs(p.X-x0) < 0.5 || math.Abs(p.X-x1) < 0.5 || math.Abs(p.Y-y0) < 0.5 || math.Abs(p.Y-y1) < 0.5
	return in && edge
}

// checkLayout 은 배치의 기하 성질을 본다 — viewBox 안, 상자끼리 겹치지 않음, 라벨·수량 글이 서로·상자와 겹치지 않음, 관계 양 끝이
// 끝 클래스 상자의 테두리(끝 모양의 꼭짓점이 거기 닿는다), 그리는 차례(관계 → 끝 모양 → 라벨 → 수량 글 → 클래스).
func checkLayout(t *testing.T, src string) {
	t.Helper()
	c, _, err := Parse(src)
	if err != nil {
		t.Fatalf("%v\n%s", err, src)
	}
	l, err := layout(c)
	if err != nil {
		t.Fatalf("배치 %v\n%s", err, src)
	}
	in := func(r rect) bool { return r.x0 >= -0.01 && r.y0 >= -0.01 && r.x1 <= l.W+0.01 && r.y1 <= l.H+0.01 }
	var boxes, labels []rect
	for i, b := range l.boxes {
		r := boxRect(l.at[i], b.w, b.h)
		if !in(r) {
			t.Fatalf("상자 %d 가 viewBox 밖 %+v (%v×%v)\n%s", i, r, l.W, l.H, src)
		}
		boxes = append(boxes, r)
	}
	for i := range boxes {
		for j := i + 1; j < len(boxes); j++ {
			if boxes[i].overlaps(boxes[j], 0.01) {
				t.Fatalf("상자 %d·%d 가 겹침\n%s", i, j, src)
			}
		}
	}
	for i, e := range l.edges {
		r := c.Rels[i]
		if len(e.Pts) < 2 {
			t.Fatalf("관계 %d 의 점이 %d 개", i, len(e.Pts))
		}
		if !onBorder(l.boxes[r.From], l.at[r.From], e.Pts[0]) || !onBorder(l.boxes[r.To], l.at[r.To], e.Pts[len(e.Pts)-1]) {
			t.Fatalf("관계 %d 의 끝이 상자 테두리가 아님 %v … %v\n%s", i, e.Pts[0], e.Pts[len(e.Pts)-1], src)
		}
		for _, p := range e.Pts {
			if p.X < 0 || p.Y < 0 || p.X > l.W || p.Y > l.H {
				t.Fatalf("관계 %d 의 점 %v 가 viewBox 밖\n%s", i, p, src)
			}
		}
		if len(e.Label) > 0 {
			lr := boxRect(e.LC, e.LW, e.LH)
			if !in(lr) {
				t.Fatalf("라벨 %d 가 viewBox 밖\n%s", i, src)
			}
			labels = append(labels, lr)
		}
	}
	for _, cd := range l.cards {
		cr := boxRect(cd.C, cd.W, cd.H)
		if !in(cr) {
			t.Fatalf("수량 글 %q 가 viewBox 밖 %+v (%v×%v)\n%s", cd.text, cr, l.W, l.H, src)
		}
		labels = append(labels, cr)
	}
	for i := range labels {
		for j := i + 1; j < len(labels); j++ {
			if labels[i].overlaps(labels[j], 0.01) {
				t.Fatalf("라벨·수량 글 %d·%d 가 겹침 %+v %+v\n%s", i, j, labels[i], labels[j], src)
			}
		}
		for k, b := range boxes {
			if labels[i].overlaps(b, 0.01) {
				t.Fatalf("라벨·수량 글 %d 가 상자 %d 와 겹침 %+v %+v\n%s", i, k, labels[i], b, src)
			}
		}
	}
	// 끝 모양의 꼭짓점(path 의 첫 점)이 관계 끝(상자 테두리)이다
	svg := drawSVG(c, l)
	var tips []string
	for i, r := range c.Rels {
		pts := l.edges[i].Pts
		if r.Type1 != None {
			tips = append(tips, "M"+p2(pts[0])+" ")
		}
		if r.Type2 != None {
			tips = append(tips, "M"+p2(pts[len(pts)-1])+" ")
		}
	}
	ends := regexp.MustCompile(`class="cls-(?:ext|comp|agg|dep)" d="([^"]*)"`).FindAllStringSubmatch(svg, -1)
	if len(ends) != len(tips) {
		t.Fatalf("끝 모양 %d 개, 기대 %d\n%s", len(ends), len(tips), src)
	}
	for i, m := range ends {
		if !strings.HasPrefix(m[1], tips[i]) {
			t.Fatalf("끝 모양 %d 가 관계 끝에서 시작하지 않음 %q, 기대 %q\n%s", i, m[1], tips[i], src)
		}
	}
	// 그리는 차례
	last := -1
	for _, g := range []string{`class="cls-rels"`, `class="cls-ends"`, `class="cls-lbls"`, `class="cls-cards"`, `class="cls-nodes"`} {
		at := strings.Index(svg, g)
		if at < 0 {
			continue
		}
		if at < last {
			t.Fatalf("그리는 차례가 틀림: %s\n%s", g, src)
		}
		last = at
	}
}

func TestLayoutProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(20261031))
	for k := 0; k < 150; k++ {
		checkLayout(t, randChart(rng, 1+rng.Intn(14), rng.Intn(22)))
	}
	long := strings.Repeat("가", 200)
	var many strings.Builder
	many.WriteString("classDiagram\nclass 넓은 {\n")
	for i := 0; i < MaxMembers; i++ {
		fmt.Fprintf(&many, "+멤버%d 아주 긴 형식 이름이 붙은 속성 %s\n", i, strings.Repeat("나", i%30))
	}
	many.WriteString("}\n넓은 --> 좁은\n")
	var big strings.Builder
	big.WriteString("classDiagram\n")
	for i := 0; i < MaxClasses; i++ {
		fmt.Fprintf(&big, "class C%d\n", i)
		if i > 0 {
			fmt.Fprintf(&big, "C%d \"1\" --> \"*\" C%d\n", i/2, i)
		}
	}
	for _, dir := range []string{"", "direction LR\n", "direction BT\n", "direction RL\n"} {
		for _, src := range []string{
			"classDiagram\n" + dir + "class A",
			"classDiagram\n" + dir + "class " + long + " {\n+" + long[:300] + "\n}\n" + long + " --> B : " + long,
			"classDiagram\n" + dir + "A --> A : 자기\nA ..|> A\nA *-- A",
			"classDiagram\n" + dir + "A \"1\" --> \"*\" B : 하나\nA \"0..1\" --> \"1..*\" B : 둘\nA <|--|> B\nB o--* A : 셋",
			"classDiagram\n" + dir + "A \"" + strings.Repeat("가", 20) + "\" -- \"" + strings.Repeat("나", 20) + "\" B\nA \"x\" -- C\nA \"y\" -- D\nA \"z\" -- E",
			strings.Replace(many.String(), "classDiagram\n", "classDiagram\n"+dir, 1),
			strings.Replace(big.String(), "classDiagram\n", "classDiagram\n"+dir, 1),
			"classDiagram\n" + dir + "class 빈 {\n}\nclass 몸 {\n<<interface>>\n}\n빈 --> 몸",
			// 제네릭 — 200 자 제목 「이름<인자>」, 긴 제네릭 멤버·중첩, 제네릭 이름끼리의 관계·수량 글
			"classDiagram\n" + dir + "class " + strings.Repeat("가", 190) + "~T1, T2~ {\n<<interface>>\n+목록 List~" + strings.Repeat("항목 ", 30) +
				"끝~\n+깊이 A~B~C~D~E~F~~~~~$\n+찾기(키 List~K~) Map~K, " + strings.Repeat("값", 120) + "~*\n}\n" +
				strings.Repeat("가", 190) + "~X~ \"1\" --> \"*\" 사전~" + strings.Repeat("키, ", 40) + "값~ : 담는다\n사전 --> 사전",
		} {
			checkLayout(t, src)
		}
	}
}

// TestBoxShape 는 클래스 상자의 칸(이름·속성·메서드 — 빈 칸 포함)과 정적·추상 클래스를 본다.
func TestBoxShape(t *testing.T) {
	svg, _ := drawn(t, "classDiagram\nclass A {\n<<interface>>\n+x$\n+f()*\n}\nclass B")
	if n := strings.Count(svg, `class="cls-div"`); n != 4 {
		t.Errorf("구분선 %d 개, 기대 4(클래스마다 둘 — 빈 칸도 그린다)", n)
	}
	for _, want := range []string{`class="cls-ann"`, ">«interface»<", `class="cls-mem cls-static"`, `class="cls-mem cls-abstract"`, ">+x<", ">+f()<", `class="cls-name"`} {
		if !strings.Contains(svg, want) {
			t.Errorf("SVG 에 %s 가 없음\n%s", want, svg)
		}
	}
}

func TestClassCollisions(t *testing.T) {
	svg, _ := drawn(t, "classDiagram\naccDescr: 설명\nclass A {\n<<interface>>\n+x$\n+f()*\n}\nA <|-- B : 라벨\nA \"1\" *-- \"*\" C\nA o.. D\nA ..> E\nB -- B")
	for _, m := range regexp.MustCompile(`class="([^"]*)"`).FindAllStringSubmatch(svg, -1) {
		for _, w := range strings.Fields(m[1]) {
			if w != "diagram-svg" && !strings.HasPrefix(w, "cls-") {
				t.Errorf("클래스도 클래스 %q 에 cls- 접두사가 없음", w)
			}
		}
	}
	files, err := filepath.Glob("../../web/*.css")
	if err != nil || len(files) == 0 {
		t.Fatalf("dist css 를 못 찾음: %v", err)
	}
	used := map[string]bool{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if regexp.MustCompile(`\.cls-[a-z]`).MatchString(line) && !strings.Contains(line, ".diagram-svg") {
				t.Errorf("%s:%d — .cls- 선택자가 .diagram-svg 문맥 밖", f, i+1)
			}
			for _, m := range regexp.MustCompile(`\.(cls-[a-z-]+)`).FindAllStringSubmatch(line, -1) {
				used[m[1]] = true
			}
			// 다른 그림 종류의 접두사(cl-·st-·er-·sq-·tl-·gt-)가 cls- 로 시작하지 않는다
			for _, m := range regexp.MustCompile(`\.((?:cl|st|er|sq|tl|gt)-[a-z-]+)`).FindAllStringSubmatch(line, -1) {
				if strings.HasPrefix(m[1], "cls-") {
					t.Errorf("%s:%d — 다른 종류의 클래스 %q 가 cls- 와 겹침", f, i+1, m[1])
				}
			}
		}
	}
	groups := map[string]bool{"cls-rels": true, "cls-ends": true, "cls-lbls": true, "cls-cards": true, "cls-nodes": true}
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(`class="([^"]*)"`).FindAllStringSubmatch(svg, -1) {
		for _, w := range strings.Fields(m[1]) {
			seen[w] = true
			if strings.HasPrefix(w, "cls-") && !used[w] && !groups[w] {
				t.Errorf("클래스 %q 의 CSS 규칙이 없음(색은 테마 토큰)", w)
			}
		}
	}
	for _, w := range []string{"cls-rel", "cls-dot", "cls-ext", "cls-comp", "cls-agg", "cls-dep", "cls-lbg", "cls-lbl", "cls-card", "cls-box",
		"cls-div", "cls-ann", "cls-name", "cls-mem", "cls-static", "cls-abstract"} {
		if !seen[w] {
			t.Errorf("견본 그림에 클래스 %q 가 없음", w)
		}
	}
}

// 사용자 글자는 이스케이프되고 SVG 는 허용 목록의 요소만 쓴다(mdoc/security_test.go 가 문서 전체로 다시 본다).
func TestSVGEscapeAndElements(t *testing.T) {
	svg, _ := drawn(t, "classDiagram\naccTitle: a\" onload=\"x\naccDescr: #lt;/title#gt;#lt;script#gt;\nclass 고객 {\n+이름 #lt;script#gt;\n}\n"+
		"고객 \"#lt;b#gt;\" --> 주문 : #lt;img#gt; #amp;\n주문 : +f() #lt;/text#gt;")
	if strings.Contains(svg, "<script") || strings.Contains(svg, "<img") || strings.Contains(svg, "<b>") || strings.Contains(svg, `onload="`) ||
		strings.Contains(svg, "</text><") && strings.Contains(svg, "#lt;") {
		t.Errorf("이스케이프되지 않은 글자:\n%s", svg)
	}
	for _, m := range regexp.MustCompile(`<([a-z]+)`).FindAllStringSubmatch(svg, -1) {
		switch m[1] {
		case "svg", "g", "rect", "path", "line", "text", "tspan", "title":
		default:
			t.Errorf("허용 목록 밖 요소 %q", m[1])
		}
	}
	for _, m := range regexp.MustCompile(` d="([^"]*)"`).FindAllStringSubmatch(svg, -1) {
		if !regexp.MustCompile(`^[MLCZ0-9 .-]+$`).MatchString(m[1]) {
			t.Errorf("경로 d 의 꼴이 허용 목록 밖 %q", m[1])
		}
	}
}

// TestEmptySTR 는 빈 따옴표·빈 값의 경계를 고정한다(itda-hyve#409 ④-3 리뷰 1회차 [1]). 원본 lexer 의 string 상태는 닫는 ["] 가
// [^"]*(STR)보다 먼저라(jison:60–62) "" 는 STR 토큰을 내지 않는다 — STR 을 요구하는 링크·콜백·click href·cssClass 줄은 원본
// 문법 오류라 원문이다. " "·"x" 는 STR 이 있어 무시하고 그린다.
func TestEmptySTR(t *testing.T) {
	const head = "classDiagram\nclass A\n"
	for _, c := range []string{`link A ""`, `callback A ""`, `click A href ""`} { // 리뷰 재현 세 건
		errAt(t, head+c, 3, "빈 따옴표")
	}
	errAt(t, head+`cssClass "" hot`, 3, "classDef·style·cssClass")
	// 리뷰 2회차 [3] — 툴팁 자리의 "" 는 원본에서 토큰 없이 사라져 유효한 링크·콜백 문장이 된다(jison:392·394·398·402). 이 판은
	// 툴팁 꼴을 받지 않아 원문이지만, 「원본도 문법 오류」라고 안내하지 않는다.
	for _, c := range []string{`link A "url" ""`, `callback A "f" ""`, `click A href "url" ""`, `click A call f() ""`} {
		errAt(t, head+c, 3, "이 판이 받지 않는다(지원 꼴 밖)")
		if _, err := Render(head + c); err != nil && strings.Contains(err.Error(), "원본도 문법 오류") {
			t.Errorf("%q: 원본 오류라고 단정함 — %v", c, err)
		}
	}
	// 빈 짝이 없는 따옴표 둘(툴팁 꼴 "x""y")은 빈 따옴표 안내가 아니다
	if _, err := Render(head + `link A "x""y"`); err == nil || strings.Contains(err.Error(), "빈 따옴표") {
		t.Errorf(`link A "x""y": 원문이되 빈 따옴표 안내가 아니어야 함 — %v`, err)
	}
	// 따옴표 안 글자로 안내 갈래를 고르지 않는다(주소의 ~·:::·. 는 제네릭·:::·이름 안내가 아니다)
	for _, c := range []string{`link A "a~b" "t"`, `link A "u:::v" "t"`, `click A href "a.b" "t"`} {
		_, err := Render(head + c)
		if err == nil || !strings.Contains(err.Error(), "click·link·callback 줄은") {
			t.Errorf("%q: click·link·callback 꼴 안내여야 함 — %v", c, err)
		}
	}
	for _, c := range []string{`link A " "`, `link A "x"`, `callback A " "`, `callback A "x"`, `click A href " "`, `click A href "x"`} {
		svg, issues := drawn(t, head+c)
		if svg == "" || len(issues) != 1 || issues[0].Line != 3 {
			t.Errorf("%q: 무시 경고 하나(3줄)와 그림이어야 함 — 경고 %+v", c, issues)
		}
	}
	// 같은 가정(빈 토큰)을 쓰는 다른 자리 — 모두 원문(그 줄)
	for _, c := range []struct{ src, msg string }{
		{"A \"\" --> B", "비어 있음"},        // 원본은 STR 없이 수량 없는 관계로 읽는다 — 이 판은 보수적으로 원문
		{"A --> \"\" B", "비어 있음"},        // 같다
		{"A --> B :", "비어 있음"},           // 원본 LABEL(":"[^:\n;]+)은 한 글자 이상 — : 만이면 COLON 이라 원본 오류
		{"A :", "비어 있음"},                 // 같다(멤버 줄)
		{"class B <<>>", "class 문장은"},    // 원본 ANNOTATION_START ANNOTATION_END 사이에 alphaNumToken 이 없어 오류
		{"<<>> A", "알 수 없는 문장"},          // 같다
		{"class B {\n<<>>\n}", "주석 이름은"}, // 몸 안 <<>> 는 원본이 빈 주석으로 받는다 — 이 판은 보수적으로 원문
	} {
		_, err := Render(head + c.src)
		var de *diagram.Error
		if !errors.As(err, &de) || de.Line < 3 || !strings.Contains(de.Msg, c.msg) {
			t.Errorf("%q: 원문(3줄 이후, %q)이어야 함 — %v", c.src, c.msg, err)
		}
	}
}
