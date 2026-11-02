package state

import (
	"testing"

	"github.com/itda-work/itda-doc/diagram/policytest"
)

// TestCaptionPolicy 는 state 캡션도 다섯 종류와 같은 diagram.Caption 경로인지 CaptionLiteral 의 사례로 본다(2026-10-31 사용자 결정
// 「여섯 종류 통일」). 기대는 policytest 에 손으로 적었다.
func TestCaptionPolicy(t *testing.T) {
	policytest.CaptionLiteral(t, Render, "stateDiagram-v2\nA")
}

// TestC1Policy 는 C1 제어 문자의 지금 동작을 DESIGN 2장 관찰 표의 행대로 본다 — **지원 약속이 아닌 회귀 감지**(policytest.C1Policy).
// state 라벨(상태 설명·전이 라벨·복합 상태 제목)은 parse.go labelOK 를 지난다. 리뷰 2회차 [1] 재현 title "x\x80y" 도 남긴다.
func TestC1Policy(t *testing.T) {
	labels := []policytest.Ctx{
		{Name: "상태 설명", Src: func(v string) string { return "stateDiagram-v2\nA : " + v }, Line: 2},
		{Name: "전이 라벨", Src: func(v string) string { return "stateDiagram-v2\nA --> B : " + v }, Line: 2},
		{Name: "복합 상태 제목", Src: func(v string) string { return "stateDiagram-v2\nstate \"" + v + "\" as C {\nA\n}" }, Line: 2},
	}
	accs := []policytest.Ctx{
		{Name: "accTitle", Src: func(v string) string { return "stateDiagram-v2\naccTitle: " + v + "\nA" }, Line: 2},
		{Name: "accDescr", Src: func(v string) string { return "stateDiagram-v2\naccDescr: " + v + "\nA" }, Line: 2},
	}
	policytest.C1Policy(t, Render, labels, accs, "stateDiagram-v2\nA")
	// 리뷰어 재현 그대로 — YAML 이스케이프 \x80 으로 든 C1 캡션(관찰 행 C1-캡션-이스케이프, 회귀 감지)
	r, err := Render("---\ntitle: \"x\\x80y\"\n---\nstateDiagram-v2\nA")
	if err != nil || r.Caption != "x\u0080y" {
		t.Errorf("title \"x\\x80y\": Caption %q, %v", r.Caption, err)
	}
}
