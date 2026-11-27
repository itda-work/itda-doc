package web

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 게시 강의 용어사전(glossary.js — itda-doc#135)의 맞춤 규칙·긋는 곳·팝업·여는 곳·팝업 안 키(실제 share.js keyHandler 까지) 사례 수(testdata/glossary.cjs).
const glossaryCases = 200

func TestGlossary(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다 — ITDA_DOC_STRICT=1(just test·CI)은 node 가 필요하다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	runner, _ := filepath.Abs(filepath.Join("testdata", "glossary.cjs"))
	out, err := exec.Command(node, runner, askSource(t, "glossary.js"), askSource(t, "ask.js"), askSource(t, "pager.js"), askSource(t, "share.js")).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(out)), fmt.Sprintf("R135 GLOSSARY DONE cases=%d failed=0", glossaryCases)) {
		t.Fatalf("사례 %d 개를 모두 마치지 않았다\n%s", glossaryCases, out)
	}
}

// 용어사전 파일이 모듈 FS 에 들고, 공유 화면(share.html)은 싣지 않으며, CSP(인라인 스크립트·style 속성 없음)를 지키고 받아 오거나 보내지 않는다.
func TestGlossaryFS(t *testing.T) {
	js, err := fs.ReadFile(FS, "glossary.js")
	if err != nil {
		t.Fatalf("FS 에 glossary.js 가 없다: %v", err)
	}
	css, err := fs.ReadFile(FS, "glossary.css")
	if err != nil {
		t.Fatalf("FS 에 glossary.css 가 없다: %v", err)
	}
	s := string(js)
	for _, bad := range []string{"fetch(", "XMLHttpRequest", "sendBeacon", "WebSocket", "EventSource", "innerHTML", "setAttribute(\"style\"", "eval(", "new Function", "sessionStorage"} {
		if strings.Contains(s, bad) {
			t.Errorf("glossary.js 에 %q", bad)
		}
	}
	for _, want := range []string{`"https://claude.ai/new?q="`, `"claude://claude.ai/new?q="`, `"noopener noreferrer"`, `PREF_KEY = "itda-glossary-open"`} {
		if !strings.Contains(s, want) {
			t.Errorf("glossary.js 에 %q 가 없다", want)
		}
	}
	h, _ := fs.ReadFile(FS, "share.html")
	if strings.Contains(string(h), "glossary") {
		t.Error("공유 화면(share.html)이 용어사전을 싣는다 — 문서 창·공유 보기는 이번 범위가 아니다(itda-hyve#1084 열린 질문)")
	}
	c := string(css)
	for _, want := range []string{"#present .itda-term { text-decoration: none;", "@media print {\n  .itda-term { text-decoration: none !important;", "  .itda-gloss-pop { display: none !important; }"} {
		if !strings.Contains(c, want) {
			t.Errorf("glossary.css 에 발표·인쇄 규칙 %q 가 없다", want)
		}
	}
}
