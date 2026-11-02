package er

import (
	"reflect"
	"testing"

	"github.com/itda-work/itda-doc/diagram/policytest"
)

// 정책 B(엔티티·공백)와 출력 관문의 종류별 시험 — 기대는 손으로 적은 표(policytest)이고 결과는 최종 SVG 를 XML 로 읽어 본다.

func render(src string) (Result, error) { return Render(src) }

// policyCtxs 는 er 의 대표 라벨 자리(별칭·속성 설명·관계 역할)와 접근성 값 자리다.
var policyCtxs = []policytest.Ctx{
	{Name: "별칭", Src: func(v string) string { return "erDiagram\nA[\"" + v + "\"]" }, Line: 2},
	{Name: "속성 설명", Src: func(v string) string { return "erDiagram\nA {\n  string a \"" + v + "\"\n}" }, Line: 3},
	{Name: "관계 역할", Src: func(v string) string { return "erDiagram\nA ||--o{ B : \"" + v + "\"" }, Line: 2},
	{Name: "accTitle", Src: func(v string) string { return "erDiagram\naccTitle: " + v + "\nA" }, Line: 2, Acc: true},
	{Name: "accDescr 한 줄", Src: func(v string) string { return "erDiagram\naccDescr: " + v + "\nA" }, Line: 2, Acc: true},
	{Name: "accDescr 블록", Src: func(v string) string { return "erDiagram\naccDescr {\n  " + v + "  \n}\nA" }, Line: 4, Acc: true, OddLine: 3},
}

// erRaw 는 er 의 원문 조각 기대다 — 따옴표 라벨은 맨 # ; < > & 를 받지 않는다(원본 lexer 가 갈리는 글자 — jLabelOK). 접근성
// 값은 줄 끝까지 글이라 #·; 를 받고 맨 &<> 만 받지 않는다(HTML 정제). 안쪽 연속 빈칸·탭은 라벨에서만 원문이다.
var erRaw = []policytest.Raw{
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

func TestEntityPolicyTable(t *testing.T) {
	policytest.EntityPolicy(t, render, policyCtxs)
	policytest.RawPolicy(t, render, policyCtxs, erRaw)
	policytest.CaptionLiteral(t, render, "erDiagram\nA")
}

func TestAriaLabelTab(t *testing.T) {
	policytest.AriaTab(t, render, func(v string) string { return "erDiagram\naccTitle: " + v + "\nA" })
}

func TestWrapOutput(t *testing.T) {
	policytest.WrapOutput(t, render, []policytest.Ctx{
		{Name: "별칭", Src: func(v string) string { return "erDiagram\nA[\"" + v + "\"]" }},
		{Name: "속성 설명", Src: func(v string) string { return "erDiagram\nA {\n  string a \"" + v + "\"\n}" }},
		{Name: "관계 역할", Src: func(v string) string { return "erDiagram\nA ||--o{ B : \"" + v + "\"" }},
	}, 3)
}

// TestGateCoversAllText 는 출력 관문의 필드 목록(eachText)이 모델의 글 필드를 빠짐없이 덮는지 본다(reflect — 내보내지 않은 필드
// 도). 모델에 글 필드를 더하고 관문에 넣지 않으면 여기서 멈춘다. 그 값이 SVG 에 그대로 나가는지는 위 시험들이 본다.
func TestGateCoversAllText(t *testing.T) {
	c, _, err := Parse("---\ntitle: 캡션\n---\nerDiagram\naccTitle: 제목\naccDescr: 설명\nA[\"별칭\"] {\n  string a PK \"설명\"\n}\nA ||--o{ B : 역할")
	if err != nil {
		t.Fatal(err)
	}
	want := policytest.TextFields(chart{})
	if got := policytest.GateFields(c.eachText); !reflect.DeepEqual(got, want) {
		t.Errorf("출력 관문 필드 %v\n모델의 글 필드 %v", got, want)
	}
}

func TestOddSpacePolicy(t *testing.T) {
	policytest.OddSpacePolicy(t, render, policyCtxs[:3], policyCtxs[3:])
	policytest.OddSpaceLines(t, render, []policytest.Ctx{
		{Name: "줄 머리", Src: func(sp string) string { return "erDiagram\n" + sp + "A" }, Line: 2},
		{Name: "줄 끝", Src: func(sp string) string { return "erDiagram\nA" + sp }, Line: 2},
	})
}

// TestC1Policy 는 C1 제어 문자의 지금 동작을 DESIGN 2장 관찰 표의 행대로 본다 — **지원 약속이 아닌 회귀 감지**(policytest.C1Policy).
func TestC1Policy(t *testing.T) {
	policytest.C1Policy(t, render, policyCtxs[:3], policyCtxs[3:], "erDiagram\nA")
}
