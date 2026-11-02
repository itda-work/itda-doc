package sequence

import (
	"reflect"
	"testing"

	"github.com/itda-work/itda-doc/diagram/policytest"
)

// 정책 B(엔티티·공백)와 출력 관문의 종류별 시험 — 기대는 손으로 적은 표(policytest)이고 결과는 최종 SVG 를 XML 로 읽어 본다.

func render(src string) (Result, error) { return Render(src) }

// labelCtxs 는 sequence 의 대표 라벨 자리(메시지·노트·참여자 별칭·box 이름·loop 조건)다 — <br> 은 줄바꿈이다.
var labelCtxs = []policytest.Ctx{
	{Name: "메시지", Src: func(v string) string { return "sequenceDiagram\nA->>B: " + v }, Line: 2},
	{Name: "노트", Src: func(v string) string { return "sequenceDiagram\nNote over A: " + v }, Line: 2},
	{Name: "참여자 별칭", Src: func(v string) string { return "sequenceDiagram\nparticipant A as " + v }, Line: 2},
	{Name: "box 이름", Src: func(v string) string { return "sequenceDiagram\nbox transparent " + v + "\nparticipant A\nend" }, Line: 2},
	{Name: "loop 조건", Src: func(v string) string { return "sequenceDiagram\nloop " + v + "\nA->>B: m\nend" }, Line: 2,
		Show: func(v string) string { return "[" + v + "]" }},
}

// titleCtx 는 제목(title) 자리다 — 한 줄이라 <br> 은 빈칸 하나다.
var titleCtx = policytest.Ctx{Name: "제목", Src: func(v string) string { return "sequenceDiagram\ntitle " + v + "\nA->>B: m" }, Line: 2}

var accCtxs = []policytest.Ctx{
	{Name: "accTitle", Src: func(v string) string { return "sequenceDiagram\naccTitle: " + v + "\nA->>B: m" }, Line: 2, Acc: true},
	{Name: "accDescr 한 줄", Src: func(v string) string { return "sequenceDiagram\naccDescr: " + v + "\nA->>B: m" }, Line: 2, Acc: true},
	{Name: "accDescr 블록", Src: func(v string) string { return "sequenceDiagram\naccDescr {\n  " + v + "  \n}\nA->>B: m" }, Line: 4, Acc: true, OddLine: 3},
}

// seqRaw 는 sequence 의 원문 조각 기대다 — 라벨의 맨 # 는 원본 lexer 의 주석 시작, 맨 ; 는 문장 끝이라 받지 않는다. 맨 & 는
// 라벨에서 글자이고(SVG 쓰기가 이스케이프), 접근성 값에서는 HTML 정제가 바꿀 수 있어 받지 않는다. 안쪽 연속 빈칸·탭은 라벨
// 에서만 원문이다.
var seqRaw = []policytest.Raw{
	{Frag: "#+65;", Acc: "#+65;", AccOK: true},
	{Frag: "#65", Acc: "#65", AccOK: true},
	{Frag: "#35;35;", Acc: "#35;", AccOK: true}, // 한 번만 — 되돌린 #35; 를 다시 풀지 않는다
	{Frag: "  ", Acc: "  ", AccOK: true},
	{Frag: "\t", Acc: "\t", AccOK: true},
	{Frag: " \t ", Acc: " \t ", AccOK: true},
	{Frag: " ", Lbl: " ", Acc: " ", LblOK: true, AccOK: true},
	{Frag: "&", Lbl: "&", LblOK: true},
	{Frag: "#65;", Lbl: "A", Acc: "A", LblOK: true, AccOK: true},
	{Frag: "￾"},
	{Frag: "￿"},
	{Frag: "�", Lbl: "�", Acc: "�", LblOK: true, AccOK: true},
	{Frag: "﷐", Lbl: "﷐", Acc: "﷐", LblOK: true, AccOK: true},
}

func TestEntityPolicyTable(t *testing.T) {
	all := append(append(append([]policytest.Ctx{}, labelCtxs...), titleCtx), accCtxs...)
	policytest.EntityPolicy(t, render, all)
	policytest.RawPolicy(t, render, all, seqRaw)
	// <br> — 라벨은 줄바꿈, 제목은 빈칸 하나, 접근성 값은 맨 < 라 원문
	policytest.RawPolicy(t, render, labelCtxs, []policytest.Raw{{Frag: "<br>", Lbl: "\n", LblOK: true}, {Frag: " <br> ", Lbl: "\n", LblOK: true}})
	policytest.RawPolicy(t, render, []policytest.Ctx{titleCtx}, []policytest.Raw{{Frag: "<br>", Lbl: " ", LblOK: true}, {Frag: " <br> ", Lbl: " ", LblOK: true}})
	policytest.RawPolicy(t, render, accCtxs, []policytest.Raw{{Frag: "<br>"}})
	policytest.CaptionLiteral(t, render, "sequenceDiagram\nA->>B: m")
}

func TestAriaLabelTab(t *testing.T) {
	policytest.AriaTab(t, render, func(v string) string { return "sequenceDiagram\naccTitle: " + v + "\nA->>B: m" })
}

func TestWrapOutput(t *testing.T) {
	policytest.WrapOutput(t, render, []policytest.Ctx{
		{Name: "메시지", Src: func(v string) string { return "sequenceDiagram\nA->>B: " + v }},
		{Name: "노트", Src: func(v string) string { return "sequenceDiagram\nNote over A: " + v }},
		{Name: "참여자 별칭", Src: func(v string) string { return "sequenceDiagram\nparticipant A as " + v }},
		{Name: "box 이름", Src: func(v string) string { return "sequenceDiagram\nbox transparent " + v + "\nparticipant A\nend" }},
	}, 1)
}

// TestGateCoversAllText 는 출력 관문의 필드 목록(eachText)이 모델의 글 필드를 빠짐없이 덮는지 본다(reflect — 내보내지 않은 필드
// 도). 출력하지 않는 필드만 까닭과 함께 뺀다.
func TestGateCoversAllText(t *testing.T) {
	c, _, err := Parse("---\ntitle: 캡션\n---\nsequenceDiagram\ntitle 제목\naccTitle: 제목\naccDescr: 설명\nbox 묶음\nparticipant A as 앨리스\nend\nA->>B: 안녕\nNote over A,B: 노트")
	if err != nil {
		t.Fatal(err)
	}
	want := policytest.TextFields(chart{})
	if got := policytest.GateFields(c.eachText); !reflect.DeepEqual(got, want) {
		t.Errorf("출력 관문 필드 %v\n모델의 글 필드 %v", got, want)
	}
}

func TestOddSpacePolicy(t *testing.T) {
	policytest.OddSpacePolicy(t, render, append(append([]policytest.Ctx{}, labelCtxs...), titleCtx), accCtxs)
	policytest.OddSpaceLines(t, render, []policytest.Ctx{
		{Name: "줄 머리", Src: func(sp string) string { return "sequenceDiagram\n" + sp + "A->>B: m" }, Line: 2},
		{Name: "줄 끝", Src: func(sp string) string { return "sequenceDiagram\nA->>B: m" + sp }, Line: 2},
	})
}

// TestC1Policy 는 C1 제어 문자의 지금 동작을 DESIGN 2장 관찰 표의 행대로 본다 — **지원 약속이 아닌 회귀 감지**(policytest.C1Policy).
func TestC1Policy(t *testing.T) {
	policytest.C1Policy(t, render, append(append([]policytest.Ctx{}, labelCtxs...), titleCtx), accCtxs, "sequenceDiagram\nA->>B: m")
}
