package web

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 쪽 나눔의 pre 줄 번호(itda-hyve#126 리뷰 R4): 코드 블록·mdoc 안전 모드의 원문 블록(.raw-source)을 거듭 나눠도 조각마다 data-line 이 그 조각의
// 첫 원문 줄이고(나눌 때마다 펜스 몫 1 이 쌓였다), 원문 블록은 조각에서도 줄마다 span[data-line] 이다. pager.js 의 splitPre 를 그대로 떼어
// node 로 돌린다(testdata/pager_split.cjs). 실제 레이아웃은 selftest 가 본다.
func TestPagerSplitPreLines(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("pager.js"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	a := strings.Index(src, "  function splitPre(")
	z := strings.Index(src[max(a, 0):], "\n  // ---- 문단")
	if a < 0 || z < 0 {
		t.Fatal("pager.js 에서 splitPre 를 찾지 못함")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" { // just test·CI
			t.Fatal("node 가 없어 실행 검사를 할 수 없다 — ITDA_DOC_STRICT=1(just test·CI)은 node 가 필요하다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	part := filepath.Join(t.TempDir(), "split.js")
	if err := os.WriteFile(part, []byte(src[a:a+z]), 0o600); err != nil {
		t.Fatal(err)
	}
	runner, _ := filepath.Abs(filepath.Join("testdata", "pager_split.cjs"))
	out, err := exec.Command(node, runner, part).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	m := regexp.MustCompile(`PAGER DONE cases=(\d+) failed=0\s*$`).FindStringSubmatch(string(out))
	if m == nil || m[1] == "0" {
		t.Fatalf("실패가 있거나 끝나지 않음\n%s", out)
	}
}

func TestPagerPageBands(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 경계 검사를 할 수 없다")
		}
		t.Skip("node 가 없어 경계 검사를 건너뜀")
	}
	runner, _ := filepath.Abs(filepath.Join("testdata", "pager_bands.cjs"))
	out, err := exec.Command(node, runner).CombinedOutput()
	if err != nil {
		t.Fatalf("쪽 경계 검사 실패: %v\n%s", err, out)
	}
}

// 쪽 넘김 키와 복사 단추(itda-hyve#413 리뷰 [2]): 단추(button — 코드 복사 단추 등)에 포커스가 있으면 Space 는 단추 누르기의 키다 —
// navKey 는 넘기지 않고, hostNavKey(문서 창 보기 모드)도 단추 위에서는 넘기지 않는다(HOST_BUSY 제외). pager.js 를 그대로
// node 로 돌린다(testdata/pager_nav.cjs). 공유 보기 화면의 keyHandler 는 share_client_test.go(share_conn.cjs)가 본다.
func TestPagerNavKeys(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	src, _ := filepath.Abs(filepath.Join("pager.js"))
	runner, _ := filepath.Abs(filepath.Join("testdata", "pager_nav.cjs"))
	out, err := exec.Command(node, runner, src).CombinedOutput()
	if err != nil {
		t.Fatalf("네김 키 검사 실패: %v\n%s", err, out)
	}
	if m := regexp.MustCompile(`PAGERNAV DONE cases=(\d+) failed=0\s*$`).FindStringSubmatch(string(out)); m == nil || m[1] == "0" {
		t.Fatalf("실패가 있거나 끝나지 않음\n%s", out)
	}
}
