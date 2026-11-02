package er

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

// summary 는 읽은 그림을 한 줄 글로 줄인다 — 엔티티(ID[별칭]{타입 이름 키 "설명"; …}) · 관계(A 왼 선 오른 B "역할") · 방향.
func summary(c *chart) string {
	var b strings.Builder
	fmt.Fprintf(&b, "dir=%d", c.Dir)
	for _, e := range c.Entities {
		b.WriteString(" " + e.ID)
		if e.Alias != "" {
			b.WriteString("[" + e.Alias + "]")
		}
		if len(e.Attrs) > 0 {
			b.WriteString("{")
			for i, a := range e.Attrs {
				if i > 0 {
					b.WriteString("; ")
				}
				b.WriteString(a.Type + " " + a.Name)
				if len(a.Keys) > 0 {
					b.WriteString(" " + strings.Join(a.Keys, ","))
				}
				if a.Comment != "" {
					b.WriteString(` "` + a.Comment + `"`)
				}
			}
			b.WriteString("}")
		}
	}
	for _, r := range c.Relations {
		line := "--"
		if !r.Identifying {
			line = ".."
		}
		fmt.Fprintf(&b, " | %s %s%s%s %s %q", c.Entities[r.A].ID, r.Left, line, r.Right, c.Entities[r.B].ID, r.Role)
	}
	if c.AccTitle != "" || c.AccDescr != "" {
		fmt.Fprintf(&b, " acc=%q/%q", c.AccTitle, c.AccDescr)
	}
	return b.String()
}

func TestParseStatements(t *testing.T) {
	ok := []struct{ src, want string }{
		{"erDiagram\nA", "dir=0 A"},
		{"erDiagram\nA[별칭]", "dir=0 A[별칭]"},
		{"erDiagram\nA[\"고객 계정\"]", "dir=0 A[고객 계정]"},
		{"erDiagram\nA {\n  string id PK \"번호\"\n  int? qty\n}", `dir=0 A{string id PK "번호"; int? qty}`},
		{"erDiagram\nA {}\nB { }\nC[c] {\n}", "dir=0 A B C[c]"},
		// 속성 차례는 원문 차례(원본 attributes 재귀 + addAttributes 역순 = 원문 차례), 같은 엔티티 블록 둘은 이어 붙는다
		{"erDiagram\nA {\n t1 a\n t2 b\n t3 c\n}\nA {\n t4 d\n}", "dir=0 A{t1 a; t2 b; t3 c; t4 d}"},
		// 별칭은 처음 적은 것이 이긴다(원본 addEntity — 비어 있을 때만 채운다)
		{"erDiagram\nA\nA[첫째]\nA[둘째]", "dir=0 A[첫째]"},
		{"erDiagram\nA ||--o{ B : places", `dir=0 A B | A ONLY_ONE--ZERO_OR_MORE B "places"`},
		{"erDiagram\nA |o..o| B : \"선택 관계\"", `dir=0 A B | A ZERO_OR_ONE..ZERO_OR_ONE B "선택 관계"`},
		{"erDiagram\nA }|--|{ B : x\nA }o..|| B : \"\"", `dir=0 A B | A ONE_OR_MORE--ONE_OR_MORE B "x" | A ZERO_OR_MORE..ONLY_ONE B ""`},
		{"erDiagram\nA ||--o{ A : self", `dir=0 A | A ONLY_ONE--ZERO_OR_MORE A "self"`},
		// 관계가 엔티티를 A·B 차례로 더한다 — 뒤의 선언은 별칭만 채운다
		{"erDiagram\nB ||--|| A : r\nA[에이]", `dir=0 B A[에이] | B ONLY_ONE--ONLY_ONE A "r"`},
		{"erDiagram\ndirection LR\nA\ndirection rl", "dir=3 A"},
		{"erDiagram\nDirection Bt\nA", "dir=1 A"},
		{"erDiagram\nA:::c1,c2\nB[b]:::c {\n s x\n}\nA:::c ||--o{ B:::d : r", `dir=0 A B[b]{s x} | A ONLY_ONE--ZERO_OR_MORE B "r"`},
		{"erDiagram\naccTitle: 이름\naccDescr: 설명 #35; ; #\nA", `dir=0 A acc="이름"/"설명 # ; #"`},
		{"erDiagram\naccDescr: 앞\naccDescr {\n}\nA", `dir=0 A acc=""/"앞"`},
		{"erDiagram\naccDescr {\n  첫 줄\n  둘째 줄\n}\nA", `dir=0 A acc=""/"첫 줄\n둘째 줄"`},
		{"erDiagram\n%% 주석\nA {\n  %% 블록 안 주석\n  string a\n}", "dir=0 A{string a}"},
		{"erDiagram\nA[\"비율 #35; #quot;x#quot;\"]", `dir=0 A[비율 # "x"]`},
		{"erDiagram\n고객_정보 {\n  문자열 이름_1 FK \"설명 a_b\"\n}", `dir=0 고객_정보{문자열 이름_1 FK "설명 a_b"}`},
		{"erDiagram\nA {\n  decimal(10,2) price PK,FK , UK\n  string[] tags\n}", "dir=0 A{decimal(10,2) price PK,FK,UK; string[] tags}"},
		{"erDiagram\nA {\n  string pkey\n  string fk_x\n}", "dir=0 A{string pkey; string fk_x}"},
		{"%%{init: {\"theme\": \"dark\"}}%%\nerDiagram\nA", "dir=0 A"},
		{"erDiagram\n  A  \r\n\tB\t", "dir=0 A B"},
	}
	for _, c := range ok {
		ch, _, err := Parse(c.src)
		if err != nil {
			t.Errorf("Parse(%q): %v", c.src, err)
			continue
		}
		if got := summary(ch); got != c.want {
			t.Errorf("Parse(%q)\n 결과 %s\n 기대 %s", c.src, got, c.want)
		}
	}

	bad := []struct {
		src  string
		line int
		msg  string
	}{
		{"erdiagram\nA", 1, "erDiagram 이 아님"},
		{"erDiagram x\nA", 1, "erDiagram 이 아님"},
		{"erDiagram", 0, "엔티티가 없음"},
		{"", 0, "비어 있음"},
		{"erDiagram\nA one or more B : x", 2, "낱말 기수"},
		{"erDiagram\nA ||--o{ B", 2, "콜론"},
		{"erDiagram\nA ||.-o{ B : x", 2, ".-"},
		{"erDiagram\nA || -- o{ B : x", 2, "관계 꼴"},
		{"erDiagram\nA ||--o{B : x", 2, "관계 꼴"},
		{"erDiagram\nA o|--|| B : x", 2, "관계 꼴"},
		{"erDiagram\nA |O--o{ B : x", 2, "관계 꼴"},
		{"erDiagram\n\"Customer Account\" ||--o{ B : x", 2, "따옴표 엔티티"},
		{"erDiagram\nA B", 2, "알 수 없는 문장"},
		{"erDiagram\nA { string a }", 2, "한 줄 블록"},
		{"erDiagram\nA {\n  string a\n  int b PK }", 4, "} 는 한 줄"},
		{"erDiagram\nA {\n  a~T~ x\n}", 3, "제네릭"},
		{"erDiagram\nA {\n  *int x\n}", 3, "~·`·*"},
		{"erDiagram\nA {\n  string a pk\n}", 3, "속성은"},
		{"erDiagram\nA {\n  PKey x\n}", 3, ""}, // PKey 는 원본도 ATTRIBUTE_WORD(PK 뒤가 낱말 글자) — 받는다
		{"erDiagram\nA {\n  PK-x y\n}", 3, "키 낱말 PK"},
		{"erDiagram\nA {\n  string Fk\n}", 3, "키 낱말 FK"},
		{"erDiagram\nA {\n  ªx y\n}", 3, "U+00AA"},
		{"erDiagram\nA {\n  int_ x\n}", 3, "_ 가 글자 사이"},
		{"erDiagram\nA {\n  string a \"x*y\"\n}", 3, "*"},
		{"erDiagram\nA {\n string a\n", 2, "닫히지 않음"},
		{"erDiagram\nsubgraph s\nA\nend", 2, "subgraph"},
		{"erDiagram\ntitle 제목\nA", 2, "title 문장"},
		{"erDiagram\nTOKEN", 2, "키워드 to"},
		{"erDiagram\nONEROUS ||--o{ B : x", 2, "키워드 one"},
		{"erDiagram\nA ||--o{ classroom : x", 2, "키워드 class"},
		{"erDiagram\nA ||--o{ B : endless", 2, "키워드 end"},
		{"erDiagram\nu-x", 2, "u-"},
		{"erDiagram\nA[many]", 2, "키워드 many"},
		{"erDiagram\nPK", 2, "PK·FK·UK"},
		{"erDiagram\na--b", 2, "--"},
		{"erDiagram\na-", 2, "--"},
		{"erDiagram\n_a", 2, "_ 가 글자 사이"},
		{"erDiagram\n𝔸", 2, "BMP 밖"},
		{"erDiagram\nA ||--o{ B : direction", 2, "direction 낱말"},
		{"erDiagram\ndirection TD", 2, "direction 낱말"},
		{"erDiagram\ndirection LR x", 2, "direction 낱말"},
		{"erDiagram\nA[\"\"]", 2, "빈 별칭"},
		{"erDiagram\nA[\"a#b\"]", 2, "#"},
		{"erDiagram\nA ||--o{ B : \"a;b\"", 2, ";"},
		{"erDiagram\nA ||--o{ B : \"a%b\"", 2, "%"},
		{"erDiagram\nA ||--o{ B : \"- 목록\"", 2, "-·+·="},
		{"erDiagram\nA ||--o{ B : \"1. 목록\"", 2, "-·+·="},
		{"erDiagram\nA ||--o{ B : \"_기울임_\"", 2, "_ 가 글자 사이"},
		{"erDiagram\nA #메모", 2, "#"},
		{"erDiagram\nA; B", 2, "#·;"},
		{"erDiagram\nA\n%%", 3, "글 없는"},
		{"erDiagram\n%%{init: {", 2, "한 줄로"},
		{"erDiagram\naccTitle:", 2, "글이 없음"},
		{"erDiagram\naccDescr {\n x } A", 3, "} 뒤에 글"},
		{"erDiagram\naccDescr {\n x", 2, "닫히지 않음"},
		{"erDiagram\nA\u00a0B", 2, "U+00A0"},
		{"erDiagram\nA\uFEFF", 2, "U+FEFF"},
		{"erDiagram\nA\fB", 2, "CR·FF·VT"},
		{"erDiagram\nA\rB", 2, "CR·FF·VT"},
		{"erDiagram\nAﬂ°", 2, "내부 표식"},
		{"erDiagram\nstyle A fill:rgb(1,2,3)", 2, "확신할 수 없음"},
		{"erDiagram\nclass A", 2, "알 수 없는"},
		{"erDiagram\n}", 2, "짝 없는 }"},
		{"erDiagram\nA {\n  %%{init: {}}%%\n}", 3, "설정 지시"},
	}
	for _, c := range bad {
		if c.msg == "" {
			if _, _, err := Parse(c.src); err != nil {
				t.Errorf("Parse(%q) 는 받아야 함: %v", c.src, err)
			}
			continue
		}
		errAt(t, c.src, c.line, c.msg)
	}
}

func TestIgnoredLines(t *testing.T) {
	src := "---\ntitle: 캡션\nconfig:\n  theme: dark\n---\n%%{init: {}}%%\nerDiagram\nstyle A fill:#f9f,stroke:#333\nstyle B color:red\nclassDef c1,c2 fill:#f00 , stroke-width:2px\nclass A,B c1,c2\nA:::c ||--o{ B : x"
	r, err := Render(src)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, is := range r.Issues {
		got = append(got, fmt.Sprintf("%d:%s", is.Line, is.Msg))
	}
	want := []string{"3:frontmatter 의 설정(config)", "6:설정 지시", "8:style 줄", "10:classDef 줄", "11:class·:::"}
	if len(got) != len(want) {
		t.Fatalf("경고 %v, 기대 %v", got, want)
	}
	for i := range want {
		if !strings.HasPrefix(got[i], want[i]) {
			t.Errorf("경고 %d: %q, 기대 %q…", i, got[i], want[i])
		}
	}
	if r.Caption != "캡션" {
		t.Errorf("캡션 %q", r.Caption)
	}
}

func TestLimits(t *testing.T) {
	var b strings.Builder
	b.WriteString("erDiagram\n")
	for i := 0; i <= MaxEntities; i++ {
		fmt.Fprintf(&b, "E%d\n", i)
	}
	errAt(t, b.String(), MaxEntities+2, "엔티티가")

	b.Reset()
	b.WriteString("erDiagram\n")
	for i := 0; i <= MaxRelations; i++ {
		fmt.Fprintf(&b, "A ||--o{ B : r%d\n", i)
	}
	errAt(t, b.String(), MaxRelations+2, "관계가")

	b.Reset()
	b.WriteString("erDiagram\nA {\n")
	for i := 0; i <= MaxAttrsPerEnt; i++ {
		fmt.Fprintf(&b, "  int a%d\n", i)
	}
	b.WriteString("}\n")
	errAt(t, b.String(), MaxAttrsPerEnt+3, "속성이")

	b.Reset()
	b.WriteString("erDiagram\n")
	for e := 0; e*MaxAttrsPerEnt <= MaxAttrsTotal; e++ {
		fmt.Fprintf(&b, "E%d {\n", e)
		for i := 0; i < MaxAttrsPerEnt; i++ {
			fmt.Fprintf(&b, "  t a\n")
		}
		b.WriteString("}\n")
	}
	if len(b.String()) <= MaxSource {
		_, err := Render(b.String())
		if err == nil || !strings.Contains(err.Error(), "속성 합계") {
			t.Errorf("속성 합계 상한: %v", err)
		}
	}

	errAt(t, "erDiagram\nA[\""+strings.Repeat("가", MaxLabel+1)+"\"]", 2, "라벨이")
	errAt(t, "erDiagram\n"+strings.Repeat("%% 긴 주석\n", MaxSource/10), 0, "KB 를 넘음")
	if _, err := Render("erDiagram\nA[\"" + strings.Repeat("가", MaxLabel) + "\"]"); err != nil {
		t.Errorf("라벨 %d 자는 받아야 함: %v", MaxLabel, err)
	}
}

// 같은 입력은 같은 출력이다(두 번 그려 바이트가 같다 — map 순회·난수 없음).
func TestDeterminism(t *testing.T) {
	rng := rand.New(rand.NewSource(409))
	for i := 0; i < 60; i++ {
		src := randChart(rng, 1+rng.Intn(12), rng.Intn(16))
		a, err := Render(src)
		if err != nil {
			t.Fatalf("%v\n%s", err, src)
		}
		b, _ := Render(src)
		if a.SVG != b.SVG {
			t.Fatalf("두 번 그린 SVG 가 다름\n%s", src)
		}
	}
}

var symsL = []string{"|o", "||", "}o", "}|"}
var symsR = []string{"o|", "||", "o{", "|{"}

// randChart 는 받아들이는 꼴만으로 된 무작위 그림이다(엔티티 n, 관계 m — 자기 관계·같은 쌍 여러 관계 포함).
func randChart(rng *rand.Rand, n, m int) string {
	names := []string{"고객", "주문", "상품", "A", "B_1", "LINE-ITEM", "배송지", "Payment", "재고", "카테고리", "쿠폰", "리뷰"}
	var b strings.Builder
	b.WriteString("erDiagram\n")
	if rng.Intn(3) == 0 {
		b.WriteString("direction " + []string{"TB", "BT", "LR", "RL"}[rng.Intn(4)] + "\n")
	}
	ent := func(i int) string { return fmt.Sprintf("%s%d", names[i%len(names)], i) }
	for i := 0; i < n; i++ {
		b.WriteString(ent(i))
		if rng.Intn(3) == 0 {
			b.WriteString("[\"보이는 이름 " + strings.Repeat("가나", rng.Intn(8)) + "\"]")
		}
		k := rng.Intn(6)
		if k == 0 {
			b.WriteString("\n")
			continue
		}
		b.WriteString(" {\n")
		for a := 0; a < k; a++ {
			fmt.Fprintf(&b, "  %s %s%d", []string{"string", "int", "decimal(10,2)", "날짜"}[rng.Intn(4)], "col", a)
			switch rng.Intn(4) {
			case 0:
				b.WriteString(" PK")
			case 1:
				b.WriteString(" PK, FK")
			}
			if rng.Intn(3) == 0 {
				b.WriteString(" \"설명 " + strings.Repeat("긴 글 ", rng.Intn(12)) + "\"")
			}
			b.WriteString("\n")
		}
		b.WriteString("}\n")
	}
	for j := 0; j < m; j++ {
		a, c := rng.Intn(n), rng.Intn(n)
		line := []string{"--", ".."}[rng.Intn(2)]
		role := []string{"가진다", "\"\"", "\"여러 낱말 역할\"", "r"}[rng.Intn(4)]
		fmt.Fprintf(&b, "%s %s%s%s %s : %s\n", ent(a), symsL[rng.Intn(4)], line, symsR[rng.Intn(4)], ent(c), role)
	}
	return b.String()
}

type rect struct{ x0, y0, x1, y1 float64 }

func (r rect) overlaps(o rect, eps float64) bool {
	return r.x0 < o.x1-eps && o.x0 < r.x1-eps && r.y0 < o.y1-eps && o.y0 < r.y1-eps
}

func (r rect) contains(p layered.Pt, eps float64) bool {
	return p.X >= r.x0-eps && p.X <= r.x1+eps && p.Y >= r.y0-eps && p.Y <= r.y1+eps
}

// borderDist 는 점과 사각형 테두리 사이의 거리다(밖은 +, 안은 −).
func borderDist(r rect, p layered.Pt) float64 {
	dx := math.Max(math.Max(r.x0-p.X, 0), p.X-r.x1)
	dy := math.Max(math.Max(r.y0-p.Y, 0), p.Y-r.y1)
	if dx > 0 || dy > 0 {
		return math.Hypot(dx, dy)
	}
	return -math.Min(math.Min(p.X-r.x0, r.x1-p.X), math.Min(p.Y-r.y0, r.y1-p.Y))
}

// TestLayoutProperties 는 무작위·극단 그림의 배치 속성을 본다: 모든 요소가 viewBox 안, 엔티티 상자끼리 겹침 없음, 역할 글
// 상자끼리·상자와 엔티티 겹침 없음, 끝 기호가 그 관계의 해당 엔티티 테두리에 닿고 상자 밖을 향함, 그리는 차례(선 → 바탕 → 글자 → 엔티티).
func TestLayoutProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(4091))
	var srcs []string
	for i := 0; i < 250; i++ {
		srcs = append(srcs, randChart(rng, 1+rng.Intn(14), rng.Intn(22)))
	}
	// 극단: 엔티티 1·100, 속성 50, 긴 이름·설명, 자기 관계, 같은 쌍 여러 관계, 네 방향, 한글/영문
	one := "erDiagram\nA"
	var hundred strings.Builder
	hundred.WriteString("erDiagram\n")
	for i := 0; i < MaxEntities; i++ {
		fmt.Fprintf(&hundred, "E%d ||--o{ E%d : r\n", i, (i*7+3)%MaxEntities)
	}
	var fifty strings.Builder
	fifty.WriteString("erDiagram\nA[\"" + strings.Repeat("아주 긴 엔티티 이름 ", 8) + "\"] {\n")
	for i := 0; i < MaxAttrsPerEnt; i++ {
		fmt.Fprintf(&fifty, "  varchar(255) column_name_%d PK, FK, UK \"%s\"\n", i, strings.Repeat("긴 설명 글 ", i%15))
	}
	fifty.WriteString("}\nA ||--o{ A : 자기\nA ||--o{ A : 또\nA }|..|{ B : \"" + strings.Repeat("긴 역할 ", 10) + "\"\n")
	multi := "erDiagram\nA ||--o{ B : 하나\nA |o..o| B : 둘\nB }|--|{ A : 셋\nA ||--|| A : 넷\nA ||--|| A : 다섯"
	srcs = append(srcs, one, hundred.String(), fifty.String(), multi)
	for _, d := range []string{"TB", "BT", "LR", "RL"} {
		srcs = append(srcs, strings.Replace(multi, "erDiagram\n", "erDiagram\ndirection "+d+"\n", 1),
			strings.Replace(fifty.String(), "erDiagram\n", "erDiagram\ndirection "+d+"\n", 1))
	}
	for k, src := range srcs {
		c, _, err := Parse(src)
		if err != nil {
			t.Fatalf("%d: %v\n%s", k, err, src)
		}
		l, err := layout(c)
		if err != nil {
			t.Fatalf("%d: %v", k, err)
		}
		fail := func(f string, a ...any) { t.Errorf("그림 %d: %s\n%s", k, fmt.Sprintf(f, a...), src) }
		all := rect{0, 0, l.W, l.H}
		boxes := make([]rect, len(c.Entities))
		for i, b := range l.boxes {
			boxes[i] = rect{l.at[i].X - b.w/2, l.at[i].Y - b.h/2, l.at[i].X + b.w/2, l.at[i].Y + b.h/2}
			if !all.contains(layered.Pt{X: boxes[i].x0, Y: boxes[i].y0}, 0.01) || !all.contains(layered.Pt{X: boxes[i].x1, Y: boxes[i].y1}, 0.01) {
				fail("엔티티 %d 가 그림 밖", i)
			}
			for j := 0; j < i; j++ {
				if boxes[i].overlaps(boxes[j], 0.5) {
					fail("엔티티 %d·%d 겹침", i, j)
				}
			}
		}
		var labels []rect
		for i, e := range l.edges {
			for _, p := range e.Pts {
				if !all.contains(p, 0.01) {
					fail("관계 %d 의 점 %v 가 그림 밖", i, p)
				}
			}
			if len(e.Label) > 0 {
				lr := rect{e.LC.X - e.LW/2, e.LC.Y - e.LH/2, e.LC.X + e.LW/2, e.LC.Y + e.LH/2}
				if !all.contains(layered.Pt{X: lr.x0, Y: lr.y0}, 0.01) || !all.contains(layered.Pt{X: lr.x1, Y: lr.y1}, 0.01) {
					fail("역할 글 %d 가 그림 밖", i)
				}
				for j, b := range boxes {
					if lr.overlaps(b, 0.5) {
						fail("역할 글 %d 와 엔티티 %d 겹침", i, j)
					}
				}
				for j, o := range labels {
					if lr.overlaps(o, 0.5) {
						fail("역할 글 %d 와 앞의 역할 글 %d 겹침", i, j)
					}
				}
				labels = append(labels, lr)
			}
			r := c.Relations[i]
			for side, ent := range []int{r.A, r.B} {
				en := l.ends[i][side]
				if d := borderDist(boxes[ent], en.tip); math.Abs(d) > 0.1 {
					fail("관계 %d 의 %d 쪽 끝 기호가 엔티티 %d 테두리에서 %.2f 떨어짐", i, side, ent, d)
				}
				if out := (layered.Pt{X: en.tip.X + en.ux, Y: en.tip.Y + en.uy}); borderDist(boxes[ent], out) < -0.01 {
					fail("관계 %d 의 %d 쪽 끝 기호가 상자 안을 향함", i, side)
				}
				if want := []Card{r.Left, r.Right}[side]; en.card != want {
					fail("관계 %d 의 %d 쪽 기수 %v, 기대 %v", i, side, en.card, want)
				}
				for _, p := range markExtent(en) {
					if !all.contains(p, 0.01) {
						fail("관계 %d 의 끝 기호가 그림 밖 %v", i, p)
					}
				}
			}
		}
		// 그리는 차례: 모든 관계 선 → 끝 기호 → (바탕 rect → 글자) 짝 → 엔티티
		svg := drawSVG(c, l)
		last := func(s string) int { return strings.LastIndex(svg, s) }
		first := func(s string) int { return strings.Index(svg, s) }
		if len(c.Relations) > 0 {
			if last(`class="er-rel`) > first(`class="er-mk"`) || last(`class="er-mk"`) > first(`class="er-ents"`) {
				fail("관계 선·끝 기호·엔티티의 그리는 차례가 다름")
			}
			if len(labels) > 0 {
				if last(`class="er-mk"`) > first(`class="er-rbg"`) || last(`class="er-role"`) > first(`class="er-ents"`) {
					fail("역할 글의 그리는 차례가 다름")
				}
				for _, m := range regexp.MustCompile(`<rect class="er-rbg"[^>]*></rect>(<text class="er-role")?`).FindAllStringSubmatch(svg, -1) {
					if m[1] == "" {
						fail("역할 바탕 rect 바로 뒤에 글자가 없음")
					}
				}
			}
		}
		if t.Failed() {
			return
		}
	}
}

// 클래스는 모두 er- 접두사(앱 CSS 충돌), base.css 의 .er- 선택자는 .diagram-svg 문맥 안(sequence 와 같은 규칙).
func TestClassCollisions(t *testing.T) {
	svg, _ := drawn(t, "erDiagram\naccDescr: 설명\nA[\"에이\"] {\n string id PK \"번호\"\n int b\n int c\n}\nA ||--o{ B : r\nA |o..|{ C : \"\"\nB }|--|| B : 자기")
	for _, m := range regexp.MustCompile(`class="([^"]*)"`).FindAllStringSubmatch(svg, -1) {
		for _, w := range strings.Fields(m[1]) {
			if w != "diagram-svg" && !strings.HasPrefix(w, "er-") {
				t.Errorf("ER 클래스 %q 에 er- 접두사가 없음", w)
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
			if strings.Contains(line, ".er-") && !strings.Contains(line, ".diagram-svg") {
				t.Errorf("%s:%d — .er- 선택자가 .diagram-svg 문맥 밖", f, i+1)
			}
			for _, m := range regexp.MustCompile(`\.(er-[a-z-]+)`).FindAllStringSubmatch(line, -1) {
				used[m[1]] = true
			}
		}
	}
	for _, m := range regexp.MustCompile(`class="([^"]*)"`).FindAllStringSubmatch(svg, -1) {
		for _, w := range strings.Fields(m[1]) {
			if strings.HasPrefix(w, "er-") && !used[w] && w != "er-rels" && w != "er-marks" && w != "er-roles" && w != "er-ents" {
				t.Errorf("클래스 %q 의 CSS 규칙이 없음(색은 테마 토큰)", w)
			}
		}
	}
}

// 사용자 글자는 이스케이프되고 SVG 는 허용 목록의 요소만 쓴다(mdoc/security_test.go 가 문서 전체로 다시 본다).
func TestSVGEscapeAndElements(t *testing.T) {
	svg, _ := drawn(t, "erDiagram\naccTitle: a\" onload=\"x\naccDescr: #lt;/title#gt;#lt;script#gt;\nA[\"#lt;script#gt;\"] {\n  string a \"#lt;img#gt; #amp;\"\n}\nA ||--o{ B : \"#lt;b#gt;\"")
	if strings.Contains(svg, "<script") || strings.Contains(svg, "<img") || strings.Contains(svg, "<b>") || strings.Contains(svg, `onload="`) {
		t.Errorf("이스케이프되지 않은 글자:\n%s", svg)
	}
	for _, m := range regexp.MustCompile(`<([a-z]+)`).FindAllStringSubmatch(svg, -1) {
		switch m[1] {
		case "svg", "g", "rect", "path", "line", "circle", "text", "tspan", "title":
		default:
			t.Errorf("허용 목록 밖 요소 %q", m[1])
		}
	}
}
