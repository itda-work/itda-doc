package web

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 목차 접기(itda-hyve#985): share.js 의 접기 꼴 tocHTML·tocTree·tocFold*(따라 펼치기·보이는 강조 조상·일괄 단추)·기억 맵 고르기·저장 글(문서 수
// 상한·오래된 것부터 버림·다른 문서와 섞이지 않음)·localStorage 기억(던지면 메모리)·tocFold·DOM 고치기를 node 로 돌린다(testdata/toc_fold.cjs).
// 소스는 askSource 로 읽는다(ITDA_DOC_TEST_OVERLAY 뮤테이션이 닿게). 문서 창 app.js 가 같은 규칙인지는 hyve 시험이 본다. 화면 안 누르기·굴림은 미검증(브라우저로 본다).
const tocFoldCases = 67

func TestShareTocFold(t *testing.T) {
	node := needNode(t)
	runner, _ := filepath.Abs(filepath.Join("testdata", "toc_fold.cjs"))
	out, err := exec.Command(node, runner, askSource(t, "pager.js"), askSource(t, "slides.js"), askSource(t, "share.js")).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(out)), fmt.Sprintf("TOCFOLD DONE cases=%d failed=0", tocFoldCases)) {
		t.Fatalf("사례 %d 개를 모두 마치지 않았다\n%s", tocFoldCases, out)
	}
}
