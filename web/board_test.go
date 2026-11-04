package web

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
)

// 보드 배치(itda-work/itda-hyve#362): dashboard.js 의 pack·tileWidth 를 node 로 돌린다(testdata/board_pack.cjs) — 같은 입력에 같은 배치,
// 읽는 차례 보존(빈 칸 남김)·2×1 이 줄 끝에서 다음 줄로·판 넘김·cols=2·판 나눔·구조 없는 kpi 는 글 타일. 타일 안 넘침(「… n행 더」·
// KPI 하한·「…」)은 화면 실측이다(명세 dashboard.md 8장 「미검증」).
func TestBoardPack(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" { // just test·CI
			t.Fatal("node 가 없어 실행 검사를 할 수 없다 — ITDA_DOC_STRICT=1(just test·CI)은 node 가 필요하다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	js, _ := filepath.Abs("dashboard.js")
	runner, _ := filepath.Abs(filepath.Join("testdata", "board_pack.cjs"))
	out, err := exec.Command(node, runner, js).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	m := regexp.MustCompile(`BOARD DONE cases=(\d+) failed=0\s*$`).FindStringSubmatch(string(out))
	if m == nil || m[1] == "0" {
		t.Fatalf("실패가 있거나 끝나지 않음\n%s", out)
	}
}
