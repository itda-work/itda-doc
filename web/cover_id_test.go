package web

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 표지 id 옮기기(itda-hyve#979): 원본 표지(section.cover·header.title-block)에 id 가 있으면 슬라이드 꼴의 새 표지 상자(div.slide-cover)와
// 보드 꼴의 머리(board-head)가 같은 id 를 갖고, 없으면 id 가 없다. 가짜 DOM 위 실제 build 를 node 로 돌린다(testdata/cover_id.cjs).
// 소스는 askSource 로 읽는다(ITDA_DOC_TEST_OVERLAY 뮤테이션이 닿게). 그 꼴의 장이 발표 사본에서 id 를 잃는지는 TestPresentInk(present_ink.cjs 14절)가
// 실제 present.js 로 본다. 실제 화면의 #이동은 미검증(브라우저로 본다).
const coverIDCases = 21

func TestCoverID(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다 — ITDA_DOC_STRICT=1(just test·CI)은 node 가 필요하다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	runner, _ := filepath.Abs(filepath.Join("testdata", "cover_id.cjs"))
	out, err := exec.Command(node, runner, askSource(t, "slides.js"), askSource(t, "dashboard.js")).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(out)), fmt.Sprintf("COVERID DONE cases=%d failed=0", coverIDCases)) {
		t.Fatalf("사례 %d 개를 모두 마치지 않았다\n%s", coverIDCases, out)
	}
}
