package web

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 코드 블록 배율·맞춤(itda-hyve#1116): pager.js 가 scale= 블록에만 --code-scale 을 걸고, fit 블록은 가장 긴 줄이 폭에 들어가게(쪽 보기·슬라이드),
// 슬라이드에서는 놓인 자리의 높이에도 맞게(하한 0.55, 자리에 30% 도 안 남으면 새 장, 하한에서도 넘치면 하한 배율로 나눔) 줄이는지 실제 paginate 를
// 레이아웃 대역 위에서 node 로 돌려 본다(testdata/pager_codefit.cjs — 글자 폭·줄 높이가 배율에 비례하는 대역이다. 실제 글꼴·WebKit 최소 글자 크기는
// 브라우저·hyve selftest 가 본다). 소스는 askSource 로 읽는다(ITDA_DOC_TEST_OVERLAY 뮤테이션이 닿게).
const pagerCodeFitCases = 19

func TestPagerCodeFit(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다 — ITDA_DOC_STRICT=1(just test·CI)은 node 가 필요하다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	runner, _ := filepath.Abs(filepath.Join("testdata", "pager_codefit.cjs"))
	out, err := exec.Command(node, runner, askSource(t, "pager.js")).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(out)), fmt.Sprintf("CODEFIT DONE cases=%d failed=0", pagerCodeFitCases)) {
		t.Fatalf("사례 %d 개를 모두 마치지 않았다\n%s", pagerCodeFitCases, out)
	}
}
