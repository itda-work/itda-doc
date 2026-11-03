package gantt

import (
	"reflect"
	"testing"

	"github.com/itda-work/itda-doc/diagram/policytest"
)

// 정책 B(엔티티·공백)와 출력 관문의 종류별 시험 — 기대는 손으로 적은 표(policytest)이고 결과는 최종 SVG 를 XML 로 읽어 본다.

func render(src string) (Result, error) { return Render(src) }

const head = "gantt\ndateFormat YYYY-MM-DD\n"

var (
	taskCtx    = policytest.Ctx{Name: "작업 이름", Src: func(v string) string { return head + v + " : 2026-01-01, 1d" }, Line: 3}
	sectionCtx = policytest.Ctx{Name: "section 이름", Src: func(v string) string { return head + "section " + v + "\n작업 : 2026-01-01, 1d" }, Line: 3}
	titleCtx   = policytest.Ctx{Name: "제목", Src: func(v string) string { return head + "title " + v + "\n작업 : 2026-01-01, 1d" }, Line: 3}
	labelCtxs  = []policytest.Ctx{taskCtx, sectionCtx, titleCtx}
	accCtxs    = []policytest.Ctx{
		{Name: "accTitle", Src: func(v string) string { return head + "accTitle: " + v + "\n작업 : 2026-01-01, 1d" }, Line: 3, Acc: true},
		{Name: "accDescr 한 줄", Src: func(v string) string { return head + "accDescr: " + v + "\n작업 : 2026-01-01, 1d" }, Line: 3, Acc: true},
		{Name: "accDescr 블록", Src: func(v string) string { return head + "accDescr {\n  " + v + "  \n}\n작업 : 2026-01-01, 1d" }, Line: 5, Acc: true, OddLine: 4},
	}
)

// ganttRaw 는 gantt 의 원문 조각 기대다 — 작업 이름([^:\n]+)·section·title([^\n]+)은 원본 lexer 가 맨 #·; 를 글자로 읽는다
// (엔티티는 라벨 글에서만 지키고 푼다 — 갈래 (b)). 간트 라벨은 한 줄이라 <br> 은 빈칸 하나다. 맨 & 는 라벨에서 글자·접근성
// 값에서 원문(HTML 정제), 안쪽 연속 빈칸·탭은 라벨에서만 원문이다.
var ganttRaw = []policytest.Raw{
	{Frag: "#+65;", Lbl: "#+65;", Acc: "#+65;", LblOK: true, AccOK: true},
	{Frag: "#65", Lbl: "#65", Acc: "#65", LblOK: true, AccOK: true},
	{Frag: "#35;35;", Lbl: "#35;", Acc: "#35;", LblOK: true, AccOK: true}, // 한 번만
	{Frag: "  ", Acc: "  ", AccOK: true},
	{Frag: "\t", Acc: "\t", AccOK: true},
	{Frag: " \t ", Acc: " \t ", AccOK: true},
	{Frag: " ", Lbl: " ", Acc: " ", LblOK: true, AccOK: true},
	{Frag: "<br>", Lbl: " ", LblOK: true},
	{Frag: " <br> ", Lbl: " ", LblOK: true},
	{Frag: "&", Lbl: "&", LblOK: true},
	{Frag: "#65;", Lbl: "A", Acc: "A", LblOK: true, AccOK: true},
	{Frag: "￾"},
	{Frag: "￿"},
	{Frag: "�", Lbl: "�", Acc: "�", LblOK: true, AccOK: true},
	{Frag: "﷐", Lbl: "﷐", Acc: "﷐", LblOK: true, AccOK: true},
}

func TestEntityPolicyTable(t *testing.T) {
	all := append(append([]policytest.Ctx{}, labelCtxs...), accCtxs...)
	policytest.EntityPolicy(t, render, all)
	policytest.RawPolicy(t, render, all, ganttRaw)
	policytest.CaptionLiteral(t, render, "gantt\ndateFormat YYYY-MM-DD\n작업 : 2026-01-01, 1d")
}

func TestAriaLabelTab(t *testing.T) {
	policytest.AriaTab(t, render, func(v string) string { return head + "accTitle: " + v + "\n작업 : 2026-01-01, 1d" })
}

func TestWrapOutput(t *testing.T) {
	policytest.WrapOutput(t, render, []policytest.Ctx{taskCtx, sectionCtx}, 1)
}

// TestGateCoversAllText 는 출력 관문의 필드 목록(eachText)이 모델의 글 필드를 빠짐없이 덮는지 본다(reflect — 내보내지 않은 필드
// 도). 출력하지 않는 필드만 까닭과 함께 뺀다.
func TestGateCoversAllText(t *testing.T) {
	c, _, err := Parse("---\ntitle: 캡션\n---\n" + head + "axisFormat %m월\ntitle 제목\naccTitle: 제목\naccDescr: 설명\nsection 묶음\n작업 : a1, 2026-01-01, 1d")
	if err != nil {
		t.Fatal(err)
	}
	// dateFmt.fmt·datePart.tok 는 dateFormat 원문과 그 토큰 — 날짜를 읽는 데만 쓰고 SVG 에 내지 않는다(축 글은 axisFormat 의
	// axisPart.s 와 날짜 숫자로 만든다).
	want := policytest.TextFields(chart{}, "dateFmt.fmt", "datePart.tok")
	if got := policytest.GateFields(c.eachText); !reflect.DeepEqual(got, want) {
		t.Errorf("출력 관문 필드 %v\n모델의 글 필드 %v", got, want)
	}
}

// TestPreprocessBoundary 는 작업 줄에서 원본의 줄 전체 전처리(style…:…#…; 끝 ; 자르기)·라벨 밖 엔티티가 이 판의 라벨 글만
// 전처리와 갈리는 꼴을 원문으로 돌리는지 본다(itda-hyve#409 ④-2 후속 리뷰 1회차 [1] — 흐름도와 같은 검사). 기대는 손으로 적었다.
func TestPreprocessBoundary(t *testing.T) {
	for _, src := range []string{
		head + "style x : 2026-01-01, 1d #35;", // 원본 정규식이 콜론 뒤 #…; 까지 걸친다
		head + "classDef x : 2026-01-01, 1d #35;",
		head + "작업 : 2026-01-01, 1d #35;",   // 라벨(작업 이름) 밖 엔티티
		head + "작업 : t#35;, 2026-01-01, 1d", // 콜론 뒤 작업 id 의 엔티티 — 문법은 받는다
		head + "style :a#;, 2026-01-01, 1d", // 토큰이 아닌 #…; — style 꼴 검사만 잡는다
	} {
		policytest.CheckSrc(t, render, policytest.Ctx{Name: "줄 전처리", Line: 3}, "", src, false, "")
	}
	for _, c := range []struct{ src, want string }{
		{head + "title style a:#35;\n작업 : 2026-01-01, 1d", "style a:#35"}, // 제목은 줄 끝까지 — 원본도 줄 끝 ; 를 자른다
		{head + "style #35; : 2026-01-01, 1d", "style #"},
	} {
		policytest.CheckSrc(t, render, policytest.Ctx{Name: "줄 전처리"}, c.src, c.src, true, c.want)
	}
}

func TestOddSpacePolicy(t *testing.T) {
	policytest.OddSpacePolicy(t, render, labelCtxs, accCtxs)
	policytest.OddSpaceLines(t, render, []policytest.Ctx{
		{Name: "줄 머리", Src: func(sp string) string { return head + sp + "작업 : 2026-01-01, 1d" }, Line: 3},
		{Name: "줄 끝", Src: func(sp string) string { return head + "작업 : 2026-01-01, 1d" + sp }, Line: 3},
		{Name: "값 사이", Src: func(sp string) string { return head + "작업 : 2026-01-01," + sp + "1d" }, Line: 3},
	})
}

// TestAxisFormatHashSemicolon 은 axisFormat 값에 # 나 ; 가 있으면 그 줄(3)의 원문인지 본다 — 엔티티 꼴 #35; 도 포함(2026-10-31
// 사용자 결정). 대조군은 지시자·한글 글자만 든 값이다. 기대는 손으로 적었다.
func TestAxisFormatHashSemicolon(t *testing.T) {
	for _, af := range []string{"%m#35;%d", "%m#%d", "%m;%d", "#", ";", "%m월#lt;"} {
		src := "gantt\ndateFormat YYYY-MM-DD\naxisFormat " + af + "\n작업 : 2026-01-01, 1d"
		policytest.CheckSrc(t, render, policytest.Ctx{Name: "axisFormat", Line: 3}, af, src, false, "")
	}
	for _, af := range []string{"%m-%d", "%m월 %d일", "%Y.%m"} {
		src := "gantt\ndateFormat YYYY-MM-DD\naxisFormat " + af + "\n작업 : 2026-01-01, 1d"
		if _, err := render(src); err != nil {
			t.Errorf("axisFormat %s: %v", af, err)
		}
	}
}

// TestC1Policy 는 C1 제어 문자의 지금 동작을 DESIGN 2장 관찰 표의 행대로 본다 — **지원 약속이 아닌 회귀 감지**(policytest.C1Policy).
func TestC1Policy(t *testing.T) {
	policytest.C1Policy(t, render, labelCtxs, accCtxs, head+"작업 : 2026-01-01, 1d")
}
