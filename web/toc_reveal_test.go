package web

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 목차 따라 굴리기(itda-hyve#991): share.js 의 가림 높이(tocCover — sticky 저작권 줄·머리)·새 scrollTop(tocRevealTop — 여유 24px·큰 항목·0 아래 없음)·
// 굴리는 상자 고르기(tocScroller)·가짜 DOM 의 tocReveal(.on-within 먼저·숨김이면 아무것도 안 함·그 상자만)과, 언제 굴리나(tocRevealer 상태 전이 —
// 바뀌었을 때만·문서가 바뀌면 같은 id 여도·사람이 만지면 남은 예약 취소·열 때 한 번)·문서 열쇠(tocDocKey)를 node 로 돌린다(testdata/toc_reveal.cjs).
// 소스는 askSource 로 읽는다(ITDA_DOC_TEST_OVERLAY 뮤테이션이 닿게). 화면 쪽 연결(paintTOC·사이드바 이벤트)과 실제 배치·굴림은 브라우저로 본다.
const tocRevealCases = 47

func TestShareTocReveal(t *testing.T) {
	node := needNode(t)
	runner, _ := filepath.Abs(filepath.Join("testdata", "toc_reveal.cjs"))
	out, err := exec.Command(node, runner, askSource(t, "pager.js"), askSource(t, "slides.js"), askSource(t, "share.js")).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(out)), fmt.Sprintf("TOCREVEAL DONE cases=%d failed=0", tocRevealCases)) {
		t.Fatalf("사례 %d 개를 모두 마치지 않았다\n%s", tocRevealCases, out)
	}
}
