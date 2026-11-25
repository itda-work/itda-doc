package web

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 캡션 있는 배경 장(itda-doc#128): slides.js 가 배경 블록에 figcaption 이 있는 장에만 쪽 class bg-cap 을 달고, 그 class 가 쪽 나눔의
// pageClass 에 들어가는지 가짜 DOM 위 실제 build 를 node 로 돌려 본다(testdata/slide_bg_cap.cjs). bg-cap 의 본문 bottom 은 base.css 가 준다 —
// 높이·겹침은 이 시험이 보지 않는다. 소스는 askSource 로 읽는다(ITDA_DOC_TEST_OVERLAY 뮤테이션이 닿게).
const slideBgCapCases = 6

func TestSlideBgCaption(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다 — ITDA_DOC_STRICT=1(just test·CI)은 node 가 필요하다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	runner, _ := filepath.Abs(filepath.Join("testdata", "slide_bg_cap.cjs"))
	out, err := exec.Command(node, runner, askSource(t, "slides.js"), askSource(t, "dashboard.js")).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(out)), fmt.Sprintf("BGCAP DONE cases=%d failed=0", slideBgCapCases)) {
		t.Fatalf("사례 %d 개를 모두 마치지 않았다\n%s", slideBgCapCases, out)
	}
}
