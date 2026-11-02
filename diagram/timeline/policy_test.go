package timeline

import (
	"reflect"
	"testing"

	"github.com/itda-work/itda-doc/diagram/policytest"
)

// 정책 B(엔티티·공백)와 출력 관문의 종류별 시험 — 기대는 손으로 적은 표(policytest)이고 결과는 최종 SVG 를 XML 로 읽어 본다.

func render(src string) (Result, error) { return Render(src) }

var (
	periodCtx  = policytest.Ctx{Name: "시기", Src: func(v string) string { return "timeline\n" + v + " : 사건" }, Line: 2}
	eventCtx   = policytest.Ctx{Name: "사건", Src: func(v string) string { return "timeline\n2026 : " + v }, Line: 2}
	sectionCtx = policytest.Ctx{Name: "section 이름", Src: func(v string) string { return "timeline\nsection " + v + "\n2026 : 사건" }, Line: 2}
	titleCtx   = policytest.Ctx{Name: "제목", Src: func(v string) string { return "timeline\ntitle " + v + "\n2026 : 사건" }, Line: 2}
	accCtxs    = []policytest.Ctx{
		{Name: "accTitle", Src: func(v string) string { return "timeline\naccTitle: " + v + "\n2026 : 사건" }, Line: 2, Acc: true},
		{Name: "accDescr 한 줄", Src: func(v string) string { return "timeline\naccDescr: " + v + "\n2026 : 사건" }, Line: 2, Acc: true},
		{Name: "accDescr 블록", Src: func(v string) string { return "timeline\naccDescr {\n  " + v + "  \n}\n2026 : 사건" }, Line: 4, Acc: true, OddLine: 3},
	}
)

// common 은 모든 문맥에서 같은 원문 조각 기대다 — 안쪽 연속 빈칸·탭은 라벨에서만 원문, 맨 & 는 라벨에서 글자·접근성 값에서
// 원문(HTML 정제), XML 이 담지 못하는 글자는 어디서나 원문.
var common = []policytest.Raw{
	{Frag: "  ", Acc: "  ", AccOK: true},
	{Frag: "\t", Acc: "\t", AccOK: true},
	{Frag: " \t ", Acc: " \t ", AccOK: true},
	{Frag: " ", Lbl: " ", Acc: " ", LblOK: true, AccOK: true},
	{Frag: "&", Lbl: "&", LblOK: true},
	{Frag: "#65;", Lbl: "A", Acc: "A", LblOK: true, AccOK: true},
	{Frag: "#35;35;", Lbl: "#35;", Acc: "#35;", LblOK: true, AccOK: true}, // 한 번만 — 맨 ; 는 timeline 의 글자다
	{Frag: "￾"},
	{Frag: "￿"},
	{Frag: "�", Lbl: "�", Acc: "�", LblOK: true, AccOK: true},
	{Frag: "﷐", Lbl: "﷐", Acc: "﷐", LblOK: true, AccOK: true},
}

// hashOK 는 맨 # 가 글자인 문맥(사건 [^:\n]·section·title [^\n]·접근성 값)의 기대, hashBad 는 시기([^#:\n]+ — # 는 원본 lexer 의
// 주석 시작)의 기대다.
var (
	hashOK = []policytest.Raw{
		{Frag: "#+65;", Lbl: "#+65;", Acc: "#+65;", LblOK: true, AccOK: true},
		{Frag: "#65", Lbl: "#65", Acc: "#65", LblOK: true, AccOK: true},
	}
	hashBad = []policytest.Raw{{Frag: "#+65;"}, {Frag: "#65"}}
)

func TestEntityPolicyTable(t *testing.T) {
	all := append([]policytest.Ctx{periodCtx, eventCtx, sectionCtx, titleCtx}, accCtxs...)
	policytest.EntityPolicy(t, render, all)
	policytest.RawPolicy(t, render, all, common)
	policytest.RawPolicy(t, render, append([]policytest.Ctx{eventCtx, sectionCtx, titleCtx}, accCtxs...), hashOK)
	policytest.RawPolicy(t, render, []policytest.Ctx{periodCtx}, hashBad)
	// <br> — 시기·사건·section 은 줄바꿈, 제목은 빈칸 하나, 접근성 값은 맨 < 라 원문
	br := []policytest.Raw{{Frag: "<br>", Lbl: "\n", LblOK: true}, {Frag: " <br> ", Lbl: "\n", LblOK: true}}
	policytest.RawPolicy(t, render, []policytest.Ctx{periodCtx, eventCtx, sectionCtx}, br)
	policytest.RawPolicy(t, render, []policytest.Ctx{titleCtx}, []policytest.Raw{{Frag: "<br>", Lbl: " ", LblOK: true}, {Frag: " <br> ", Lbl: " ", LblOK: true}})
	policytest.RawPolicy(t, render, accCtxs, []policytest.Raw{{Frag: "<br>"}})
	policytest.CaptionLiteral(t, render, "timeline\n2026 : 사건")
}

func TestAriaLabelTab(t *testing.T) {
	policytest.AriaTab(t, render, func(v string) string { return "timeline\naccTitle: " + v + "\n2026 : 사건" })
}

func TestWrapOutput(t *testing.T) {
	policytest.WrapOutput(t, render, []policytest.Ctx{periodCtx, eventCtx, sectionCtx}, 3)
}

// TestGateCoversAllText 는 출력 관문의 필드 목록(eachText)이 모델의 글 필드를 빠짐없이 덮는지 본다(reflect — 내보내지 않은 필드
// 도). 출력하지 않는 필드만 까닭과 함께 뺀다.
func TestGateCoversAllText(t *testing.T) {
	c, _, err := Parse("---\ntitle: 캡션\n---\ntimeline\ntitle 제목\naccTitle: 제목\naccDescr: 설명\nsection 묶음\n2026 : 사건")
	if err != nil {
		t.Fatal(err)
	}
	want := policytest.TextFields(tl{})
	if got := policytest.GateFields(c.eachText); !reflect.DeepEqual(got, want) {
		t.Errorf("출력 관문 필드 %v\n모델의 글 필드 %v", got, want)
	}
}

func TestOddSpacePolicy(t *testing.T) {
	policytest.OddSpacePolicy(t, render, []policytest.Ctx{periodCtx, eventCtx, sectionCtx, titleCtx}, accCtxs)
	policytest.OddSpaceLines(t, render, []policytest.Ctx{
		{Name: "줄 머리", Src: func(sp string) string { return "timeline\n" + sp + "2026 : 사건" }, Line: 2},
		{Name: "줄 끝", Src: func(sp string) string { return "timeline\n2026 : 사건" + sp }, Line: 2},
		{Name: "section 앞", Src: func(sp string) string { return "timeline\n" + sp + "section 묶음\n2026 : 사건" }, Line: 2},
	})
}

// TestC1Policy 는 C1 제어 문자의 지금 동작을 DESIGN 2장 관찰 표의 행대로 본다 — **지원 약속이 아닌 회귀 감지**(policytest.C1Policy).
func TestC1Policy(t *testing.T) {
	policytest.C1Policy(t, render, []policytest.Ctx{periodCtx, eventCtx, sectionCtx, titleCtx}, accCtxs, "timeline\n2026 : 사건")
}
