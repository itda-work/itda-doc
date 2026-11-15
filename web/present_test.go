package web

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 발표·판서(present.js·ink.js·inkshape.js): 판서 모형(획 추가·되돌리기·다시 실행·상한·탭 수명)·기하·바탕 밝기와 발표의 넘김·차례로 나타내기,
// 호스트 훅 계약(present.js 머리 주석 — 전체 화면·호스트 전용 앵커, 크게 보기·코드 복사 부품이 없을 때)을 가짜 DOM 위에서 node 로 돌린다
// (testdata/present_ink.cjs). 실제 창 안 동작은 호스트 앱의 시험이 본다.
const presentInkCases = 49

func TestPresentInk(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다 — ITDA_DOC_STRICT=1(just test·CI)은 node 가 필요하다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	dir, _ := filepath.Abs(".")
	runner, _ := filepath.Abs(filepath.Join("testdata", "present_ink.cjs"))
	out, err := exec.Command(node, runner, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(out)), fmt.Sprintf("PRESENT INK DONE cases=%d failed=0", presentInkCases)) {
		t.Fatalf("사례 %d 개를 모두 마치지 않았다\n%s", presentInkCases, out)
	}
}

// 발표·판서는 FS 에 들고, 특정 앱의 창 API·이름을 부르지 않는다 — 창 틀은 호스트 훅(window.ItdaPresentHost)으로만 만난다.
func TestPresentInkFS(t *testing.T) {
	bad := regexp.MustCompile(`window\.go\b|window\.runtime\b|(?i:hyve)|(?i:wails)`)
	for _, name := range []string{"present.js", "ink.js", "inkshape.js", "present.css"} {
		b, err := fs.ReadFile(FS, name)
		if err != nil {
			t.Fatalf("FS 에 %s 가 없다: %v", name, err)
		}
		if m := bad.FindString(string(b)); m != "" {
			t.Errorf("%s 에 %q — 창 틀은 호스트 훅으로만 만난다", name, m)
		}
	}
	src, _ := fs.ReadFile(FS, "present.js")
	if !strings.Contains(string(src), "window.ItdaPresent = {") {
		t.Error("present.js 의 전역이 window.ItdaPresent 가 아니다")
	}
}
