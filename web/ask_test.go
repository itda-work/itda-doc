package web

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 원문 제목의 섹션 식별자와 서비스별 새 대화 주소를 확인한다.
const askCases = 54

func TestAsk(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다 — ITDA_DOC_STRICT=1(just test·CI)은 node 가 필요하다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	src := askSource(t, "ask.js")
	runner, _ := filepath.Abs(filepath.Join("testdata", "ask.cjs"))
	out, err := exec.Command(node, runner, src, askSource(t, "slides.js")).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(out)), fmt.Sprintf("R965 ASK DONE cases=%d failed=0", askCases)) {
		t.Fatalf("사례 %d 개를 모두 마치지 않았다\n%s", askCases, out)
	}
}

// askSource 는 node에서 읽을 스크립트 경로다. 뮤테이션 시험에서는 overlay의 사본을 읽는다.
func askSource(t *testing.T, name string) string {
	t.Helper()
	src, _ := filepath.Abs(name)
	p := os.Getenv("ITDA_DOC_TEST_OVERLAY")
	if p == "" {
		return src
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("ITDA_DOC_TEST_OVERLAY=%s 를 읽지 못함: %v", p, err)
	}
	var ov struct {
		Replace map[string]string `json:"Replace"`
	}
	if json.Unmarshal(b, &ov) != nil {
		t.Fatalf("ITDA_DOC_TEST_OVERLAY=%s 가 overlay JSON 이 아님", p)
	}
	if r, ok := ov.Replace[src]; ok {
		return r
	}
	if r, ok := ov.Replace[name]; ok {
		return r
	}
	return src
}

// 공유 화면의 스크립트·스타일과 CSP에 필요한 제한을 확인한다.
func TestAskFS(t *testing.T) {
	b, err := fs.ReadFile(FS, "ask.js")
	if err != nil {
		t.Fatalf("FS 에 ask.js 가 없다: %v", err)
	}
	s := string(b)
	// 툴팁은 브라우저 기본 title 이다 — 자리를 재는 문서 리스너·aria-description 은 두지 않는다(itda-hyve#988 접근 방식 변경).
	for _, bad := range []string{"clipboard", "armSlides", "codeButtons", "itda-asknote", "slideText", ".style.", "setAttribute(\"style\"", "eval(", "new Function", "window.go", "window.runtime", "aria-description", "pointerover", "focusin", "data-tip"} {
		if strings.Contains(s, bad) {
			t.Errorf("ask.js 에 %q", bad)
		}
	}
	if !strings.Contains(s, `root.open(u, "_blank", "noopener")`) {
		t.Error("ask.js 의 기본 새 탭이 noopener 가 아니다")
	}
	h, _ := fs.ReadFile(FS, "share.html")
	if !strings.Contains(string(h), "/_/ask.js") {
		t.Error("공유 화면에서 ask.js를 싣지 않는다")
	}
	css, _ := fs.ReadFile(FS, "base.css")
	// 아이콘(::before 의 mask-image)·자리(float)·인쇄 숨김. 툴팁은 CSS 로 그리지 않는다(::after·data-tip 규칙 없음).
	for _, bad := range []string{".itda-section-ask::after", ".itda-section-ask[data-tip", "attr(aria-description)"} {
		if strings.Contains(string(css), bad) {
			t.Errorf("base.css 에 CSS 툴팁 규칙 %q 이 남았다", bad)
		}
	}
	for _, want := range []string{
		".itda-section-ask {\n  float: right;",
		".itda-section-ask[data-service=\"claude\"]::before { mask-image: url(\"data:image/svg+xml,",
		".itda-section-ask[data-service=\"chatgpt\"]::before { mask-image: url(\"data:image/svg+xml,",
		"@media print {\n  .itda-section-ask { display: none !important; }",
	} {
		if !strings.Contains(string(css), want) {
			t.Errorf("섹션 단추의 아이콘·자리·인쇄 숨김 규칙이 없다: %q", want)
		}
	}
	// 상표 아이콘의 출처·표기 파일이 묶음에 든다.
	lic, err := fs.ReadFile(FS, "LICENSE-ask-icons.txt")
	if err != nil || !strings.Contains(string(lic), "상표") {
		t.Errorf("FS 에 아이콘 출처·상표 표기(LICENSE-ask-icons.txt)가 없다: %v", err)
	}
}
