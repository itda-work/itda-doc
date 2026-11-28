package web

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 장 배율·장 맞춤(itda-hyve#1116): slides.js 가 슬라이드를 여는 제목의 data-scale·class fit 으로 그 장의 쪽에만 --slide-body-scale 을 걸고, .fit 이면
// 한 장·잘림 0 이 되는 가장 큰 배율(하한 0.6, 하한에서도 안 들면 하한 배율로 「(계속)」, 나눌 수 없는 블록은 잘린 채)을 고르며, 배율 속성이 바뀐
// 장만 이어 만들기에서 다시 만드는지 가짜 DOM 위 실제 build 를 node 로 돌려 본다(testdata/slide_scale.cjs — 쪽 수·잘림은 블록 높이 × 배율의
// 대역이다. 실제 줄 바꿈·글자 크기는 브라우저·hyve selftest 가 본다). 소스는 askSource 로 읽는다(ITDA_DOC_TEST_OVERLAY 뮤테이션이 닿게).
const slideScaleCases = 32

func TestSlideBodyScale(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다 — ITDA_DOC_STRICT=1(just test·CI)은 node 가 필요하다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	runner, _ := filepath.Abs(filepath.Join("testdata", "slide_scale.cjs"))
	out, err := exec.Command(node, runner, askSource(t, "slides.js"), askSource(t, "dashboard.js"), askSource(t, "incr.js")).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(out)), fmt.Sprintf("SLIDESCALE DONE cases=%d failed=0", slideScaleCases)) {
		t.Fatalf("사례 %d 개를 모두 마치지 않았다\n%s", slideScaleCases, out)
	}
}

// 장 배율은 본문만(itda-hyve#1116 결정 1): base.css 의 .page.slide 본문 글자 토큰은 모두 본문 배율 꼴(장 배율이 있으면 그것, 없으면 문서 배율)을
// 곱하고, 슬라이드 머리(.slide-title)·구역 제목·표지 제목은 문서 배율(--slide-scale)만 곱한다. 코드 글자는 블록 배율(--code-scale)을 곱한다.
func TestBaseCSSBodyScaleTokens(t *testing.T) {
	b, err := os.ReadFile(askSource(t, "base.css"))
	if err != nil {
		t.Fatal(err)
	}
	css := string(b)
	rule := func(sel string) string {
		i := strings.Index(css, "\n"+sel+" {")
		if i < 0 {
			t.Fatalf("base.css 에 %s 규칙이 없다", sel)
		}
		j := strings.Index(css[i:], "}")
		return css[i : i+j]
	}
	const body = "var(--slide-body-scale, var(--slide-scale))"
	slide := rule(".page.slide")
	for _, tok := range []string{"--text-size", "--h1-size", "--h2-size", "--h3-size", "--h4-size", "--table-size", "--code-size", "--lead-size", "--caption-size"} {
		m := regexp.MustCompile(regexp.QuoteMeta(tok) + `:\s*calc\(([^;]*)\);`).FindStringSubmatch(slide)
		if m == nil || !strings.Contains(m[1], body) || strings.Count(m[1], "--slide-scale") != 1 {
			t.Errorf(".page.slide %s 가 본문 배율 꼴 하나가 아니다: %v", tok, m)
		}
	}
	for _, sel := range []string{".slide-title", ".page-body .ss-title", ".page-body .sc-title"} {
		r := rule(sel)
		if strings.Contains(r, "--slide-body-scale") || !strings.Contains(r, "var(--slide-scale)") {
			t.Errorf("%s 는 문서 배율만 곱해야 한다(장 배율과 무관): %s", sel, r)
		}
	}
	if pre := rule(".page-body pre"); !strings.Contains(pre, "font-size: calc(var(--code-size) * var(--code-scale, 1))") {
		t.Errorf(".page-body pre 글자 크기가 코드 블록 배율을 곱하지 않는다: %s", pre)
	}
}
