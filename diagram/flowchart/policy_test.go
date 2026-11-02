package flowchart

import (
	"reflect"
	"testing"

	"github.com/itda-work/itda-doc/diagram/policytest"
)

// 정책 B(엔티티·공백)와 출력 관문의 종류별 시험 — 기대는 손으로 적은 표(policytest)이고 결과는 최종 SVG 를 XML 로 읽어 본다.

func render(src string) (Result, error) { return Render(src) }

var (
	labelCtxs = []policytest.Ctx{
		{Name: "노드 라벨", Src: func(v string) string { return "flowchart TD\nA[" + v + "]" }, Line: 2},
		{Name: "따옴표 노드 라벨", Src: func(v string) string { return "flowchart TD\nA[\"" + v + "\"]" }, Line: 2},
		{Name: "둥근 노드 라벨", Src: func(v string) string { return "flowchart TD\nA(" + v + ")" }, Line: 2},
		{Name: "간선 라벨", Src: func(v string) string { return "flowchart TD\nA -->|" + v + "| B" }, Line: 2},
		{Name: "따옴표 간선 라벨", Src: func(v string) string { return "flowchart TD\nA -->|\"" + v + "\"| B" }, Line: 2},
	}
	subgraphCtx = policytest.Ctx{Name: "subgraph 제목", Src: func(v string) string { return "flowchart TD\nsubgraph S [" + v + "]\nA\nend" }, Line: 2}
	accCtxs     = []policytest.Ctx{
		{Name: "accTitle", Src: func(v string) string { return "flowchart TD\naccTitle: " + v + "\nA" }, Line: 2, Acc: true},
		{Name: "accDescr 한 줄", Src: func(v string) string { return "flowchart TD\naccDescr: " + v + "\nA" }, Line: 2, Acc: true},
		{Name: "accDescr 블록", Src: func(v string) string { return "flowchart TD\naccDescr {\n  " + v + "  \n}\nA" }, Line: 4, Acc: true, OddLine: 3},
	}
)

// flowRaw 는 흐름도의 원문 조각 기대다 — 라벨 글(원본 text·STR 상태 [^\[\]\(\)\{\}\|\"]+)은 맨 #·;·& 를 글자로 읽는다(엔티티는
// 라벨 글에서만 지키고 푼다 — 갈래 (b)). 맨 & 는 접근성 값에서 원문(HTML 정제), 안쪽 연속 빈칸·탭은 라벨에서만 원문이다.
var flowRaw = []policytest.Raw{
	{Frag: "#+65;", Lbl: "#+65;", Acc: "#+65;", LblOK: true, AccOK: true},
	{Frag: "#65", Lbl: "#65", Acc: "#65", LblOK: true, AccOK: true},
	{Frag: "#35;35;", Lbl: "#35;", Acc: "#35;", LblOK: true, AccOK: true}, // 한 번만
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
	all := append(append(append([]policytest.Ctx{}, labelCtxs...), subgraphCtx), accCtxs...)
	policytest.EntityPolicy(t, render, all)
	policytest.RawPolicy(t, render, all, flowRaw)
	// <br> — 노드·간선 라벨은 줄바꿈, subgraph 제목은 빈칸 하나, 접근성 값은 맨 < 라 원문
	policytest.RawPolicy(t, render, labelCtxs, []policytest.Raw{{Frag: "<br>", Lbl: "\n", LblOK: true}, {Frag: " <br> ", Lbl: "\n", LblOK: true}})
	policytest.RawPolicy(t, render, []policytest.Ctx{subgraphCtx}, []policytest.Raw{{Frag: "<br>", Lbl: " ", LblOK: true}, {Frag: " <br> ", Lbl: " ", LblOK: true}})
	policytest.RawPolicy(t, render, accCtxs, []policytest.Raw{{Frag: "<br>"}})
	policytest.CaptionLiteral(t, render, "flowchart TD\nA")
}

func TestAriaLabelTab(t *testing.T) {
	policytest.AriaTab(t, render, func(v string) string { return "flowchart TD\naccTitle: " + v + "\nA" })
}

func TestWrapOutput(t *testing.T) {
	policytest.WrapOutput(t, render, append(append([]policytest.Ctx{}, labelCtxs...), subgraphCtx), 1)
}

// TestGateCoversAllText 는 출력 관문의 필드 목록(eachText)이 모델의 글 필드를 빠짐없이 덮는지 본다(reflect — 내보내지 않은 필드
// 도). 출력하지 않는 필드만 까닭과 함께 뺀다.
func TestGateCoversAllText(t *testing.T) {
	g, _, err := Parse("---\ntitle: 캡션\n---\nflowchart TD\naccTitle: 제목\naccDescr: 설명\nsubgraph S [묶음]\nA[라벨] -->|간선| B\nend")
	if err != nil {
		t.Fatal(err)
	}
	want := policytest.TextFields(Graph{})
	if got := policytest.GateFields(g.eachText); !reflect.DeepEqual(got, want) {
		t.Errorf("출력 관문 필드 %v\n모델의 글 필드 %v", got, want)
	}
}

// TestPreprocessBoundary 는 원본의 줄 전체 전처리(encodeEntities 의 style…:…#…;·classDef…:…#…; 끝 ; 자르기)와 이 판의 라벨 글만
// 전처리가 갈리는 줄을 원문으로 돌리는지 본다(#409 ④-2 후속 리뷰 1회차 [1] 재현). 쓰지 않는 style·classDef 줄 하나는 그대로
// 무시하고 그린다. 기대는 손으로 적었다.
func TestPreprocessBoundary(t *testing.T) {
	for _, src := range []string{
		"flowchart TD\nA[\"style x:#35;\"];",    // 원본은 줄 끝 ; 를, 라벨 전처리는 엔티티의 ; 를 자른다
		"flowchart TD\nA[style] -->|x:#35;| B",  // 원본 정규식이 노드 라벨부터 간선 라벨의 ; 까지 걸친다
		"flowchart TD\nA[\"classDef x:#35;\"];", // classDef 변형
		"flowchart TD\nA[classDef] -->|x:#35;| B",
		"flowchart TD\nA --> B; classDef c fill:#f00;", // 그리는 문장과 이은 classDef
		"flowchart TD\nsubgraph s#35;\nA\nend",         // 라벨 밖 엔티티 — 제목이 된 id(검사를 끄면 s#35 로 그린다)
		"flowchart TD\nsubgraph x#35; y\nA\nend",
	} {
		policytest.CheckSrc(t, render, policytest.Ctx{Name: "줄 전처리", Line: 2}, "", src, false, "")
	}
	for _, c := range []struct{ src, want string }{
		{"flowchart TD\nA[\"x:#35;\"];", "x:#"}, // style·classDef 낱말이 없으면 원본도 자르지 않는다
		{"flowchart TD\nA[style] --> B", "style"},
		{"flowchart TD\nstyle A fill:#f00;\nA[\"a#35;\"]", "a#"}, // 쓰지 않는 style 줄 하나(끝 ; 자르기가 뜻을 바꾸지 않는다)
		{"flowchart TD\nclassDef c fill:#f00;\nA[\"a#35;\"]", "a#"},
	} {
		policytest.CheckSrc(t, render, policytest.Ctx{Name: "줄 전처리"}, c.src, c.src, true, c.want)
	}
}

func TestOddSpacePolicy(t *testing.T) {
	policytest.OddSpacePolicy(t, render, append(append([]policytest.Ctx{}, labelCtxs...), subgraphCtx), accCtxs)
	policytest.OddSpaceLines(t, render, []policytest.Ctx{
		{Name: "줄 머리", Src: func(sp string) string { return "flowchart TD\n" + sp + "A[x]" }, Line: 2},
		{Name: "줄 끝", Src: func(sp string) string { return "flowchart TD\nA[x]" + sp }, Line: 2},
		{Name: "화살표 뒤", Src: func(sp string) string { return "flowchart TD\nA -->" + sp + "B" }, Line: 2},
	})
}

// TestC1Policy 는 C1 제어 문자의 지금 동작을 DESIGN 2장 관찰 표의 행대로 본다 — **지원 약속이 아닌 회귀 감지**(policytest.C1Policy).
func TestC1Policy(t *testing.T) {
	policytest.C1Policy(t, render, append(append([]policytest.Ctx{}, labelCtxs...), subgraphCtx), accCtxs, "flowchart TD\nA")
}

// TestC1Reproduction 은 리뷰 2회차 [1] 재현 `A[x<U+0080>y]` 의 지금 동작(받아 SVG 글자에 그대로)을 본다 — 관찰 행 C1-라벨,
// **지원 약속이 아닌 회귀 감지**.
func TestC1Reproduction(t *testing.T) {
	r, err := Render("flowchart TD\nA[x\u0080y]")
	if err != nil {
		t.Fatal(err)
	}
	if o := policytest.ParseSVG(t, r.SVG); !o.Shows("x\u0080y") {
		t.Errorf("SVG 에 x\\u0080y 가 없음\n%s", r.SVG)
	}
}
