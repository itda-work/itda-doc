package web

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
)

// 보드 배치(itda-work/itda-hyve#362): dashboard.js 의 pack·tileWidth 를 node 로 돌린다(testdata/board_pack.cjs) — 같은 입력에 같은 배치,
// 읽는 차례 보존(빈 칸 남김)·2×1 이 줄 끝에서 다음 줄로·판 없음(줄만 는다)·cols=2·줄 나눔·구역 줄·구조 없는 kpi 는 글 타일·크게 보기 자리(zoomBox). 타일 안 넘침(「… n행 더」·
// KPI 하한·「…」)은 화면 실측이다(명세 dashboard.md 8장 「미검증」).
func TestBoardPack(t *testing.T) { runBoardNode(t, "board_pack.cjs", "BOARD") }

// 크게 보기(itda-work/itda-hyve#362): dashboard.js 의 zoomHit·zoomStep·zoomKey 와 가짜 DOM 위 zoomer 를 node 로 돌린다(testdata/board_zoom.cjs) —
// 여는 곳(머리·빈 곳·단추)과 안 여는 곳(글·링크·표 머리 칸·차트·글자 고름·끌기), Esc·닫기 단추·바깥 누르기(다른 타일을 열지 않음)·초점 복귀,
// 보이는 영역 크기·굴림·창 크기 따라감·바깥 굴림 막기, ←·→ 구역 줄 건너기·끝 멈춤, 굴림 키 막기·Tab 가둠, 발표 중(blocked) 안 열림·열린 것 닫기, 다시 그린 뒤 버림. 실제 화면(WebKit)은 문서 창 selftest 가 본다.
func TestBoardZoom(t *testing.T) { runBoardNode(t, "board_zoom.cjs", "ZOOM") }

func runBoardNode(t *testing.T, runnerName, tag string) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" { // just test·CI
			t.Fatal("node 가 없어 실행 검사를 할 수 없다 — ITDA_DOC_STRICT=1(just test·CI)은 node 가 필요하다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	js, _ := filepath.Abs("dashboard.js")
	runner, _ := filepath.Abs(filepath.Join("testdata", runnerName))
	out, err := exec.Command(node, runner, js).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	m := regexp.MustCompile(tag + ` DONE cases=(\d+) failed=0\s*$`).FindStringSubmatch(string(out))
	if m == nil || m[1] == "0" {
		t.Fatalf("실패가 있거나 끝나지 않음\n%s", out)
	}
}
