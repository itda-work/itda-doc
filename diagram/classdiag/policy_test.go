package classdiag

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/itda-work/itda-doc/diagram"
	"github.com/itda-work/itda-doc/diagram/policytest"
)

// 정책 B(엔티티·공백)와 출력 관문의 종류별 시험 — 기대는 손으로 적은 표(policytest)이고 결과는 최종 SVG 를 XML 로 읽어 본다.

func render(src string) (Result, error) { return Render(src) }

// 멤버 문맥(클래스 몸·이름 : 멤버)은 라벨보다 좁다 — 글자·숫자·빈칸·, . - = ? / _ ( ) $ * 와 첫 글자 가시성만 받고(parse.go member),
// 그 밖의 글자(U+FFFD·비문자·C1 제어 문자 포함)는 엔티티 #…; 로만 쓴다. 그래서 라벨 문맥과 원문 조각 표·C1 표를 따로 둔다.
var (
	memberCtxs = []policytest.Ctx{
		{Name: "멤버(몸)", Src: func(v string) string { return "classDiagram\nclass A {\n  " + v + "\n}" }, Line: 3},
		{Name: "멤버(이름 :)", Src: func(v string) string { return "classDiagram\nA : " + v }, Line: 2},
	}
	// genericCtxs 는 멤버 안 제네릭 ~…~ 의 안쪽 글이다 — 보이는 꼴은 「<값>」(원본 parseGenericTypes). 값은 x…y 로 감싸 넣으므로
	// 여는 ~ 앞뒤가 늘 이름 글자다.
	genericCtxs = []policytest.Ctx{
		{Name: "멤버 제네릭(몸)", Src: func(v string) string { return "classDiagram\nclass A {\n  +목록 List~" + v + "~\n}" }, Line: 3,
			Show: func(v string) string { return "+목록 List<" + v + ">" }},
		{Name: "멤버 제네릭 반환형(이름 :)", Src: func(v string) string { return "classDiagram\nA : +f() Map~" + v + "~" }, Line: 2,
			Show: func(v string) string { return "+f() : Map<" + v + ">" }},
	}
	labelCtxs = []policytest.Ctx{
		{Name: "관계 라벨", Src: func(v string) string { return "classDiagram\nA --> B : " + v }, Line: 2},
		{Name: "수량 글", Src: func(v string) string { return "classDiagram\nA \"" + v + "\" --> B" }, Line: 2},
	}
	accCtxs = []policytest.Ctx{
		{Name: "accTitle", Src: func(v string) string { return "classDiagram\naccTitle: " + v + "\nclass A" }, Line: 2, Acc: true},
		{Name: "accDescr 한 줄", Src: func(v string) string { return "classDiagram\naccDescr: " + v + "\nclass A" }, Line: 2, Acc: true},
		{Name: "accDescr 블록", Src: func(v string) string { return "classDiagram\naccDescr {\n  " + v + "  \n}\nclass A" }, Line: 4, Acc: true, OddLine: 3},
	}
)

// classRaw 는 라벨 문맥(관계 라벨·수량 글)과 접근성 문맥의 원문 조각 기대다 — 라벨은 맨 # ; < > & 와 연속 빈칸·탭을 받지 않는다
// (원본 lexer 가 갈리거나 마크다운·HTML 로 그려지는 글자 — labelOK). 접근성 값은 줄 끝까지 글이라 #·; 를 받고 맨 &<> 만 받지 않는다.
var classRaw = []policytest.Raw{
	{Frag: "#+65;", Acc: "#+65;", AccOK: true},
	{Frag: "#65", Acc: "#65", AccOK: true},
	{Frag: "#35;35;", Acc: "#35;", AccOK: true}, // 한 번만 — 되돌린 #35; 를 다시 풀지 않는다
	{Frag: "  ", Acc: "  ", AccOK: true},
	{Frag: "\t", Acc: "\t", AccOK: true},
	{Frag: " \t ", Acc: " \t ", AccOK: true},
	{Frag: " ", Lbl: " ", Acc: " ", LblOK: true, AccOK: true},
	{Frag: "<br>"},
	{Frag: "&"},
	{Frag: "#65;", Lbl: "A", Acc: "A", LblOK: true, AccOK: true},
	{Frag: "￾"},
	{Frag: "￿"},
	{Frag: "�", Lbl: "�", Acc: "�", LblOK: true, AccOK: true},
	{Frag: "﷐", Lbl: "﷐", Acc: "﷐", LblOK: true, AccOK: true},
}

// memberRaw 는 멤버 문맥의 원문 조각 기대다 — 라벨과 같되 U+FFFD·비문자 U+FDD0 도 받지 않는다(글자·숫자가 아니다 — 엔티티로 쓴다).
var memberRaw = []policytest.Raw{
	{Frag: "#+65;"}, {Frag: "#65"}, {Frag: "#35;35;"}, {Frag: "  "}, {Frag: "\t"}, {Frag: " \t "},
	{Frag: " ", Lbl: " ", LblOK: true},
	{Frag: "<br>"}, {Frag: "&"},
	{Frag: "#65;", Lbl: "A", LblOK: true},
	{Frag: "￾"}, {Frag: "￿"}, {Frag: "�"}, {Frag: "﷐"},
	{Frag: "#65533;", Lbl: "�", LblOK: true}, // 엔티티로는 받는다
}

func TestEntityPolicyTable(t *testing.T) {
	all := append(append(append([]policytest.Ctx{}, memberCtxs...), labelCtxs...), accCtxs...)
	policytest.EntityPolicy(t, render, all)
	policytest.RawPolicy(t, render, append(append([]policytest.Ctx{}, labelCtxs...), accCtxs...), classRaw)
	policytest.RawPolicy(t, render, memberCtxs, memberRaw)
	policytest.EntityPolicy(t, render, genericCtxs)
	policytest.RawPolicy(t, render, genericCtxs, memberRaw)
	policytest.CaptionLiteral(t, render, "classDiagram\nclass A")
}

func TestAriaLabelTab(t *testing.T) {
	policytest.AriaTab(t, render, func(v string) string { return "classDiagram\naccTitle: " + v + "\nclass A" })
}

// TestWrapOutput 은 줄바꿈되는 문맥(멤버 — 폭 320, 관계 라벨 — 폭 140)의 최종 SVG 줄이 값을 보이는 글자 그대로 나눈 것인지 본다.
// 클래스 이름은 빈칸이 없는 한 낱말이고 수량 글은 20 자까지라 여기 넣지 않는다.
func TestWrapOutput(t *testing.T) {
	policytest.WrapOutput(t, render, memberCtxs, 2) // 폭 320 — 빈칸 없는 영문 스물여섯 자와 낱말 여덟은 한 줄에 든다
	// 제네릭 표시 글 — 긴 멤버(<·> 가 든 글)와 긴 제목 「이름<인자>」도 보이는 글자를 줄 경계에서만 나눈다
	for _, c := range []struct{ src, show string }{
		{"classDiagram\nclass A {\n+목록 List~" + strings.Repeat("항목 ", 30) + "끝~\n}", "+목록 List<" + strings.Repeat("항목 ", 30) + "끝>"},
		{"classDiagram\nclass " + strings.Repeat("가나다라", 15) + "~K, V~", strings.Repeat("가나다라", 15) + "<K, V>"},
		{"classDiagram\nclass 사전~" + strings.Repeat("키, ", 40) + "값~", "사전<" + strings.Repeat("키, ", 40) + "값>"},
	} {
		r, err := Render(c.src)
		if err != nil {
			t.Fatalf("%q: %v", c.src, err)
		}
		if ls := policytest.ParseSVG(t, r.SVG).LinesOf(c.show); len(ls) < 2 {
			t.Errorf("긴 제네릭 글 %q 이 규칙대로 나뉘지 않음: %q", c.show, ls)
		}
	}
	policytest.WrapOutput(t, render, labelCtxs[:1], 3)
	// 긴 클래스 이름(빈칸 없음)은 글자에서 나뉘고 글자를 더하거나 빼지 않는다
	name := strings.Repeat("가나다라마바사아자차카타파하", 4)
	r, err := Render("classDiagram\nclass " + name)
	if err != nil {
		t.Fatal(err)
	}
	if ls := policytest.ParseSVG(t, r.SVG).LinesOf(name); len(ls) < 2 {
		t.Errorf("긴 이름이 나뉘지 않음: %q", ls)
	}
}

// TestGateCoversAllText 는 출력 관문의 필드 목록(eachText)이 모델의 글 필드를 빠짐없이 덮는지 본다(reflect — 내보내지 않은 필드
// 도). 모델에 글 필드를 더하고 관문에 넣지 않으면 여기서 멈춘다. 그 값이 SVG 에 그대로 나가는지는 위 시험들이 본다.
func TestGateCoversAllText(t *testing.T) {
	c, _, err := Parse(gateSample)
	if err != nil {
		t.Fatal(err)
	}
	want := policytest.TextFields(chart{})
	if got := policytest.GateFields(c.eachText); !reflect.DeepEqual(got, want) {
		t.Errorf("출력 관문 필드 %v\n모델의 글 필드 %v", got, want)
	}
}

// gateSample 은 출력 글자 필드를 모두 채우는 원문이다.
const gateSample = "---\ntitle: 캡션\n---\nclassDiagram\naccTitle: 제목\naccDescr: 설명\nclass A~T~ {\n<<interface>>\n+x\n+f()\n}\nA \"1\" --> \"*\" B : 라벨"

// TestGateLines 는 관문이 필드마다 그 값이 나온 줄을 알리는지 본다 — 받은 모델의 값 하나에 XML 이 담지 못하는 U+FFFE 를 넣어
// 관문을 부른다(그 값을 원문으로 넣으면 개별 검사가 먼저 막으므로 관문만의 갈래를 따로 본다).
func TestGateLines(t *testing.T) {
	const bad = "x￾y"
	for _, c := range []struct {
		name string
		line int
		set  func(c *chart)
	}{
		{"캡션", 2, func(c *chart) { c.FMCaption = bad }},
		{"accTitle", 5, func(c *chart) { c.AccTitle = bad }},
		{"accDescr", 6, func(c *chart) { c.AccDescr = bad }},
		{"클래스 이름", 7, func(c *chart) { c.Classes[0].Name = bad }},
		{"제네릭 인자", 7, func(c *chart) { c.Classes[0].Type = bad }},
		{"주석", 8, func(c *chart) { c.Classes[0].Annot = bad }},
		{"속성", 9, func(c *chart) { c.Classes[0].Attrs[0].Text = bad }},
		{"메서드", 10, func(c *chart) { c.Classes[0].Methods[0].Text = bad }},
		{"관계 라벨", 12, func(c *chart) { c.Rels[0].Label = bad }},
		{"수량 글 1", 12, func(c *chart) { c.Rels[0].Card1 = bad }},
		{"수량 글 2", 12, func(c *chart) { c.Rels[0].Card2 = bad }},
	} {
		m, _, err := Parse(gateSample)
		if err != nil {
			t.Fatal(err)
		}
		c.set(m)
		var ge *diagram.Error
		if err := diagram.Gate(m.eachText); !errors.As(err, &ge) || ge.Line != c.line {
			t.Errorf("%s: 관문 %v, 기대 %d 줄", c.name, err, c.line)
		}
	}
}

// TestGenericTitleOutput 은 이름 제네릭의 상자 제목이 「이름<인자>」 글자 그대로 SVG 에 나가는지 본다(<·> 는 XML 이스케이프 —
// XML 로 읽으면 글자 그대로).
func TestGenericTitleOutput(t *testing.T) {
	for _, c := range []struct{ src, show string }{
		{"classDiagram\nclass 목록~T~", "목록<T>"},
		{"classDiagram\nclass Map~K, V~\nMap --> 값", "Map<K, V>"},
		{"classDiagram\n사전~키,값~ --> 값", "사전<키,값>"},
	} {
		r, err := Render(c.src)
		if err != nil {
			t.Fatalf("%q: %v", c.src, err)
		}
		if !policytest.ParseSVG(t, r.SVG).Shows(c.show) {
			t.Errorf("%q: SVG 에 %q 가 없음\n%s", c.src, c.show, r.SVG)
		}
		if strings.Contains(r.SVG, "~") {
			t.Errorf("%q: SVG 에 ~ 가 남음", c.src)
		}
	}
}

func TestOddSpacePolicy(t *testing.T) {
	policytest.OddSpacePolicy(t, render, append(append([]policytest.Ctx{}, memberCtxs...), labelCtxs...), accCtxs)
	policytest.OddSpaceLines(t, render, []policytest.Ctx{
		{Name: "줄 머리", Src: func(sp string) string { return "classDiagram\n" + sp + "class A" }, Line: 2},
		{Name: "줄 끝", Src: func(sp string) string { return "classDiagram\nclass A" + sp }, Line: 2},
		{Name: "관계 사이", Src: func(sp string) string { return "classDiagram\nA" + sp + "--> B" }, Line: 2},
		{Name: "몸 안 줄 머리", Src: func(sp string) string { return "classDiagram\nclass A {\n" + sp + "+x\n}" }, Line: 3},
	})
}

// TestC1Policy 는 C1 제어 문자의 지금 동작을 DESIGN 2장 관찰 표의 행대로 본다 — **지원 약속이 아닌 회귀 감지**(policytest.C1Policy).
// 라벨 문맥은 관계 라벨·수량 글이다. 멤버 문맥은 C1 을 받지 않는다(글자·숫자가 아니다 — 아래에서 따로 본다).
func TestC1Policy(t *testing.T) {
	policytest.C1Policy(t, render, labelCtxs, accCtxs, "classDiagram\nclass A")
	for r := rune(0x80); r <= 0x9f; r++ {
		for _, ctx := range memberCtxs {
			policytest.CheckSrc(t, render, ctx, string(r), ctx.Src("x"+string(r)+"y"), false, "")
		}
	}
}
