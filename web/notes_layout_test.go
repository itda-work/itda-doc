package web

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
)

// 노트 칸 배치(slides.js placeNotes·notePlacement·noteAfter·layoutNotes·watchNotes, itda-hyve#752): 오른쪽·아래·끔의 좌표와 #pages·쪽 여백, 숨은 쪽·빈 칸, 같은 값 다시 쓰지 않음,
// 칸을 넣는 상자(into), 높이가 바뀌면 다시 놓기. 작은 가짜 DOM 으로 node 에서 돌린다(testdata/notes_layout.cjs). 실제 레이아웃은 itda-hyve selftest 가 본다.
func TestNotesLayout(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	runner, _ := filepath.Abs(filepath.Join("testdata", "notes_layout.cjs"))
	out, err := exec.Command(node, runner).CombinedOutput()
	if err != nil {
		t.Fatalf("노트 칸 배치 검사 실패: %v\n%s", err, out)
	}
	if !regexp.MustCompile(`NOTES DONE cases=\d+ failed=0\s*$`).Match(out) {
		t.Fatalf("끝나지 않음\n%s", out)
	}
}
