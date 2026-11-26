package web

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 열지 않는 문서 안 파일 링크 끄기(itda-hyve#1096): share.js 의 linkTo(누를 때와 끌 때가 같이 쓰는 판정)·markLinks(href 를 data-local-href 로 옮기고
// class link-off·aria-disabled·title·tabindex 를 단다, 목록이 늦게 오면 되살린다)·원천의 liveLink·실제 onClick 과 Enter 처리기를 node 로 돌린다
// (testdata/link_off.cjs). 소스는 askSource 로 읽는다(ITDA_DOC_TEST_OVERLAY 뮤테이션이 닿게). 화면 모양(share.css)·실제 브라우저의 가운데 단추는 미검증.
const linkOffCases = 32

func TestShareLinkOff(t *testing.T) {
	node := needNode(t)
	runner, _ := filepath.Abs(filepath.Join("testdata", "link_off.cjs"))
	out, err := exec.Command(node, runner, askSource(t, "pager.js"), askSource(t, "slides.js"), askSource(t, "share.js")).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(out)), fmt.Sprintf("LINKOFF DONE cases=%d failed=0", linkOffCases)) {
		t.Fatalf("사례 %d 개를 모두 마치지 않았다\n%s", linkOffCases, out)
	}
}
