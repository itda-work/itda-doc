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

// 묻기 단추(itda-hyve#896, ask.js): 켬 설정 정리·OS 두 출처·질문 글 두 틀·화면 글자(노트 칸·단추 제외)·노트는 실은 렌더일 때만·공개 묶음의
// 슬라이드 주소·주소 넘침 문턱(24,000자 경계)·넘침의 클립보드 길·슬라이드 막대(멱등·끄기·장 수 그대로·글자 없음)·위임 클릭을 가짜 DOM 위에서
// node 로 돌린다(testdata/ask.cjs). 코드 블록 단추(조각 모으기)는 호스트 앱의 codecopy.js 시험이, 실제 화면은 호스트 앱 selftest 가 본다.
const askCases = 64

func TestAsk(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다 — ITDA_DOC_STRICT=1(just test·CI)은 node 가 필요하다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	src := askSource(t)
	runner, _ := filepath.Abs(filepath.Join("testdata", "ask.cjs"))
	out, err := exec.Command(node, runner, src).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(out)), fmt.Sprintf("R896 ASK DONE cases=%d failed=0", askCases)) {
		t.Fatalf("사례 %d 개를 모두 마치지 않았다\n%s", askCases, out)
	}
}

// askSource 는 node 가 읽을 ask.js 경로다 — ITDA_DOC_TEST_OVERLAY 의 Replace 에 ask.js 가 있으면 그 파일(뮤테이션 시험, presentSources 와 같은 약속).
func askSource(t *testing.T) string {
	t.Helper()
	src, _ := filepath.Abs("ask.js")
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
	if r, ok := ov.Replace["ask.js"]; ok {
		return r
	}
	return src
}

// ask.js 는 FS 에 있고 share.html 이 codecopy.js(코드 블록 단추를 다는 호스트 부품)보다 먼저 싣는다. 새 탭은 noopener 로만 연다 —
// 인라인 스타일 속성·eval 류를 쓰지 않는다(공유·묶음 화면의 CSP 를 넓히지 않는다).
func TestAskFS(t *testing.T) {
	b, err := fs.ReadFile(FS, "ask.js")
	if err != nil {
		t.Fatalf("FS 에 ask.js 가 없다: %v", err)
	}
	s := string(b)
	for _, bad := range []string{".style.", "setAttribute(\"style\"", "eval(", "new Function", "window.go", "window.runtime"} {
		if strings.Contains(s, bad) {
			t.Errorf("ask.js 에 %q", bad)
		}
	}
	if !strings.Contains(s, `root.open(u, "_blank", "noopener")`) {
		t.Error("ask.js 의 기본 새 탭이 noopener 가 아니다")
	}
	h, _ := fs.ReadFile(FS, "share.html")
	ai, ci := strings.Index(string(h), "/_/ask.js"), strings.Index(string(h), "/_/codecopy.js")
	if ai < 0 || ci < 0 || ai > ci {
		t.Errorf("share.html 이 ask.js 를 codecopy.js 앞에 싣지 않는다(ask %d, codecopy %d)", ai, ci)
	}
}
