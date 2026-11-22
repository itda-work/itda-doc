package web

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 그림 설명 툴팁·크게 보기(itda-doc#121): share.js 의 대상 고르기(figTarget·figText·figFocus — 렌더된 alt 만, 로고·숨은 단계·무대 제외)·툴팁 자리(figTipPlace)와
// 가짜 DOM 위 figTips(링크·YouTube 그림 누르기는 크게 보기가 아님, 빈 alt 툴팁 없음, 글자로만 넣음, 본문 교체 뒤 중복·유령 없음, 열린 동안 Esc·Space 우선순위,
// 부품이 없으면 툴팁만)를 node 로 돌린다(testdata/fig_tips.cjs). 소스는 askSource 로 읽는다(ITDA_DOC_TEST_OVERLAY 뮤테이션이 닿게).
// 실제 배치·좌표·인쇄·화면 읽기 프로그램은 브라우저로 본다.
const figTipCases = 84

func TestShareFigTips(t *testing.T) {
	node := needNode(t)
	runner, _ := filepath.Abs(filepath.Join("testdata", "fig_tips.cjs"))
	out, err := exec.Command(node, runner, askSource(t, "pager.js"), askSource(t, "slides.js"), askSource(t, "share.js")).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(out)), fmt.Sprintf("FIGTIP DONE cases=%d failed=0", figTipCases)) {
		t.Fatalf("사례 %d 개를 모두 마치지 않았다\n%s", figTipCases, out)
	}
}

// 그림 설명 툴팁·크게 보기는 화면에만 있다: 인쇄에서 숨기고(share.css @media print), share.js 는 render 가 쪽을 바꿔 끼운 뒤에만 표지를 단다
// (나누기 전 흐름에 붙이면 pager.js 측정·slides.js 패널 사본에 끼어든다).
func TestShareFigTipsWiring(t *testing.T) {
	css, err := os.ReadFile("share.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(css), "@media print { .fig-tip, .fig-lb-close, body > .fig-lb { display: none !important; } }") {
		t.Error("share.css 에 그림 설명·크게 보기의 인쇄 숨김이 없다")
	}
	b, err := os.ReadFile("share.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	a := strings.Index(src, "  async function render(")
	if a < 0 {
		t.Fatal("share.js 에서 render 를 찾지 못함")
	}
	body := src[a:]
	rep, arm := strings.Index(body, "pagesEl.replaceChildren(frag)"), strings.Index(body, "figs.arm()")
	if rep < 0 || arm < 0 || arm < rep {
		t.Errorf("figs.arm() 이 render 의 쪽 바꿔 끼우기 뒤에 있지 않다(replaceChildren %d, arm %d)", rep, arm)
	}
	if n := strings.Count(src, "figs.arm()"); n != 1 {
		t.Errorf("figs.arm() 은 render 한 곳에서만 부른다: %d", n)
	}
}
