package web

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
)

// 제목 앞 머리표(:::eyebrow, itda-doc#92): 제목 바로 앞 형제인 .eyebrow 는 슬라이드에서 그 제목의 장으로 넘어가고(slides.js takeLeadIn — 내용 장·구역 장),
// 쪽 나눔에서 제목과 함께 다닌다(pager.js leadIn·keepWithNext). groups 와 leadIn 을 node 로 돌린다(testdata/leadin.cjs) — 머리 높이·쪽 경계의 실제 배치는 브라우저로 본다.
func TestEyebrowLeadIn(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	slides, _ := filepath.Abs("slides.js")
	pager, _ := filepath.Abs("pager.js")
	runner, _ := filepath.Abs(filepath.Join("testdata", "leadin.cjs"))
	out, err := exec.Command(node, runner, slides, pager).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	if m := regexp.MustCompile(`LEADIN DONE cases=(\d+) failed=0\s*$`).FindStringSubmatch(string(out)); m == nil || m[1] == "0" {
		t.Fatalf("실패가 있거나 끝나지 않음\n%s", out)
	}
}

// 제목 앞 머리표의 쪽 나눔을 실제 paginate 로 돌린다(testdata/pager_leadin.cjs): 머리표는 나누지 않고(atomic) 통째로 제목과 함께 다음 쪽으로 간다 —
// 두 문단 머리표의 첫 문단만 들어갈 자리, 뒤 H1 + h1Always, 뒤 H2 + 본문, 한 쪽보다 긴 머리표(넘침 경로·빈 쪽 없음), 제목 앞이 아닌 머리표(보통 나눔).
// pager.js 는 pagerSource 로 읽는다(ITDA_DOC_TEST_OVERLAY 뮤테이션이 닿게).
func TestEyebrowPaginate(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	src := filepath.Join(t.TempDir(), "pager.js")
	if err := os.WriteFile(src, []byte(pagerSource(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	runner, _ := filepath.Abs(filepath.Join("testdata", "pager_leadin.cjs"))
	out, err := exec.Command(node, runner, src).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	if m := regexp.MustCompile(`PAGERLEADIN DONE cases=(\d+) failed=0\s*$`).FindStringSubmatch(string(out)); m == nil || m[1] == "0" {
		t.Fatalf("실패가 있거나 끝나지 않음\n%s", out)
	}
}
