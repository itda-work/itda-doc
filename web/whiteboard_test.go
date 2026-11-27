package web

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 화이트보드(wbcore.js·whiteboard.js — 화이트보드 기획 ④-1a): 순수 함수(좌표·외곽선·curve·색인·타일), 파일 형식(왕복·반올림·검증 거부·모르는 키 보존·
// 큰 v 읽기 전용), 좌표 지킴이(엉뚱한 coalesced·predicted 버리기, 마우스는 예측 없음), 입력 규칙(펜·손가락 이동·오므리기·손바닥·지우개·휠·Space),
// 자동 저장(2초 조용·저장 중 재변경·실패 뒤 재시도·숨김·닫기 실패), 판 목록의 store 호출 차례(전환·새 판·복제·이름·삭제), 열린 동안 아래 화면이
// 키·포인터를 받지 않음, 발표의 W(계약 있음·없음·청중·보드 발표)를 가짜 DOM 위에서 node 로 돌린다(testdata/whiteboard.cjs).
// 리뷰 1회차 반영분: 판을 바꾸거나 닫는 동안의 잠금, 떠나는 길마다 진행 중 획 확정, store 별 메모리, 복제 전 저장, store 요청 8초 끝(지연·거부·
// 끝나지 않는 Promise)과 한 판에 save 하나, 수치 범위(좌표·굵기·보기·색인·타일), 굵기 왕복, 발표 whiteboard() 조건, 미리보기 캐시, 획 점 상한.
// 접근 방식 리뷰 반영분: 판별 기록기(번호·합치기·거부 뒤 멈춤과 retry·동기 throw), 떠나기(그 자리 put·시도 토큰·전체 기한), 관리 작업(복제·삭제 철회,
// 결과 미확정, 삭제 뒤 폐기, 이름 맞추기), 저장소 경계(store 객체 교체는 잇지 않음), 숨김 즉시 put, 좌표 범위(화면 전체 묶기·범위 밖 점에서 획 끝내기·
// serialize 던짐) — 손으로 푸는 store 로 실제 호출 차례·같은 판 save 최대 동시 수·최종 저장 본문을 본다.
// 실제 그리기(canvas 픽셀)·CSS 배치·실기 Pencil 입력은 보지 않는다(미검증 — 데모로 본다).
const whiteboardCases = 359

func TestWhiteboard(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다 — ITDA_DOC_STRICT=1(just test·CI)은 node 가 필요하다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	dir := whiteboardSources(t)
	runner, _ := filepath.Abs(filepath.Join("testdata", "whiteboard.cjs"))
	out, err := exec.Command(node, runner, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(out)), fmt.Sprintf("WHITEBOARD DONE cases=%d failed=0", whiteboardCases)) {
		t.Fatalf("사례 %d 개를 모두 마치지 않았다\n%s", whiteboardCases, out)
	}
}

// whiteboardSources 는 node 실행 검사가 읽을 폴더다 — 보통은 web/ 그대로다. ITDA_DOC_TEST_OVERLAY 가 가리키는 overlay JSON 의 Replace 에
// 아래 파일이 있으면 바꿔 끼운 채 임시 폴더에 모아 그 폴더를 준다(presentSources 와 같은 약속 — 뮤테이션 시험이 디스크 대신 바꿔 끼운 소스를 보게).
func whiteboardSources(t *testing.T) string {
	t.Helper()
	dir, _ := filepath.Abs(".")
	p := os.Getenv("ITDA_DOC_TEST_OVERLAY")
	if p == "" {
		return dir
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
	tmp := t.TempDir()
	for _, name := range []string{"wbcore.js", "whiteboard.js", "present.js", "ink.js", "inkshape.js"} {
		src := filepath.Join(dir, name)
		if r, ok := ov.Replace[src]; ok {
			src = r
		} else if r, ok := ov.Replace[name]; ok {
			src = r
		}
		c, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmp, name), c, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return tmp
}

// 화이트보드 자원은 FS 에 있고, 특정 앱의 창 API·이름을 부르지 않으며, 네트워크 요청을 하지 않는다(핵심 가치 — 저장은 호스트 계약으로만).
// CSP style-src 'self' 에서 돌도록 style 속성·innerHTML·인라인 이벤트 속성을 쓰지 않는다(CSSOM el.style.x = … 은 된다).
func TestWhiteboardFS(t *testing.T) {
	bad := regexp.MustCompile(`window\.go\b|window\.runtime\b|(?i:hyve)|(?i:wails)`)
	net := regexp.MustCompile(`\bfetch\s*\(|XMLHttpRequest|WebSocket|EventSource|sendBeacon|navigator\.serviceWorker|\bimport\s*\(|indexedDB|localStorage|sessionStorage`)
	inline := regexp.MustCompile(`setAttribute\(\s*["']style["']|style\s*=\s*["']|\.innerHTML\s*=|\.outerHTML|insertAdjacentHTML|setAttribute\(\s*["']on[a-z]+["']`)
	for _, name := range []string{"wbcore.js", "whiteboard.js", "whiteboard.css"} {
		b, err := fs.ReadFile(FS, name)
		if err != nil {
			t.Fatalf("FS 에 %s 가 없다: %v", name, err)
		}
		src := string(b)
		if m := bad.FindString(src); m != "" {
			t.Errorf("%s 에 %q — 창 틀·저장은 호스트 계약으로만 만난다", name, m)
		}
		if strings.HasSuffix(name, ".js") {
			if m := net.FindString(src); m != "" {
				t.Errorf("%s 에 %q — 화이트보드는 네트워크·브라우저 저장소를 쓰지 않는다", name, m)
			}
			if m := inline.FindString(src); m != "" {
				t.Errorf("%s 에 %q — CSP 에서 막히는 인라인 스타일·HTML 문자열", name, m)
			}
		}
	}
	js, _ := fs.ReadFile(FS, "whiteboard.js")
	for _, want := range []string{"window.ItdaWhiteboard = {", "window.ItdaWhiteboardHost", "data-pr-overlay"} {
		if !strings.Contains(string(js), want) {
			t.Errorf("whiteboard.js 에 %q 가 없다", want)
		}
	}
	core, _ := fs.ReadFile(FS, "wbcore.js")
	if !strings.Contains(string(core), "root.ItdaWbCore = m") {
		t.Error("wbcore.js 의 전역이 window.ItdaWbCore 가 아니다")
	}
}

// whiteboard.css 의 규칙은 모두 .itda-wb 아래다 — 표면이 붙는 문서·발표 화면의 모양을 건드리지 않게. @규칙은 쓰지 않는다.
func TestWhiteboardCSSScoped(t *testing.T) {
	b, err := fs.ReadFile(FS, "whiteboard.css")
	if err != nil {
		t.Fatal(err)
	}
	css := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(b), "")
	if strings.Contains(css, "@") {
		t.Error("whiteboard.css 에 @규칙이 있다")
	}
	rules := regexp.MustCompile(`([^{}]+)\{[^{}]*\}`).FindAllStringSubmatch(css, -1)
	if len(rules) < 20 {
		t.Fatalf("규칙이 너무 적다: %d", len(rules))
	}
	for _, r := range rules {
		for sel := range strings.SplitSeq(r[1], ",") {
			sel = strings.TrimSpace(sel)
			if !strings.HasPrefix(sel, ".itda-wb") {
				t.Errorf("whiteboard.css 선택자 %q 가 .itda-wb 아래가 아니다", sel)
			}
		}
	}
}
