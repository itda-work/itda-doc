package web

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 보드 배치(itda-work/itda-hyve#362): dashboard.js 의 pack·tileWidth 를 node 로 돌린다(testdata/board_pack.cjs) — 같은 입력에 같은 배치,
// 읽는 차례 보존(빈 칸 남김)·2×1 이 줄 끝에서 다음 줄로·판 없음(줄만 는다)·cols=2·줄 나눔·구역 줄·구조 없는 kpi 는 글 타일·크게 보기 자리(zoomBox),
// 자리·크기 속성(itda-work/itda-hyve#1038 — at·span 의 고정·자동 섞임, 구역 상대 행, 높이 2 이상, 찬 칸 건너뛰기, 겹침, 격자 밖, tileAttrs), 자동 타일의 격자 배율(3단계 A — autoScale 경계값,
// tileItem 의 at 만·span 만 제외, 4열·3행~24열·12행, 높이 2 이상 자동 타일의 커서). 타일 안 넘침(「… n행 더」·
// KPI 하한·「…」)은 화면 실측이다(명세 dashboard.md 8장 「미검증」).
func TestBoardPack(t *testing.T) { runBoardNode(t, "board_pack.cjs", "BOARD") }

// 크게 보기(itda-work/itda-hyve#362): dashboard.js 의 zoomHit·zoomStep·zoomKey 와 가짜 DOM 위 zoomer 를 node 로 돌린다(testdata/board_zoom.cjs) —
// 여는 곳(머리·빈 곳·단추)과 안 여는 곳(글·링크·표 머리 칸·차트·글자 고름·끌기), Esc·닫기 단추·바깥 누르기(다른 타일을 열지 않음)·초점 복귀,
// 보이는 영역 크기·굴림·창 크기 따라감·바깥 굴림 막기, ←·→ 구역 줄 건너기·끝 멈춤, 굴림 키 막기·Tab 가둠, 발표 중(blocked) 안 열림·열린 것 닫기, 다시 그린 뒤 버림. 실제 화면(WebKit)은 문서 창 selftest 가 본다.
func TestBoardZoom(t *testing.T) { runBoardNode(t, "board_zoom.cjs", "ZOOM") }

// 타일 상태·움직임(itda-work/itda-hyve#1038 3단계 B·C): 가짜 DOM 위에서 build 를 끝까지 돌린다(testdata/board_motion.cjs) —
// 상태 class 하나씩·둘 이상이면 무거운 쪽(bad > warn > ok)·구역 줄과 타일 안 낮은 제목은 무시·머리 맨 앞 표식 자리와 읽는 프로그램용 글·slots 그대로,
// 상태 없는 문서의 보드 마크업이 이 변경 전(main 26d80f8)과 같음, 처음 그림에만 등장(읽는 차례 늦춤)·다시 그림에 등장 없음·본문이 바뀐 타일만 강조(새 타일·
// 자리만 바뀐 타일·같은 본문 없음, 같은 제목 둘의 차례 짝), motion: false·motionScope, 끝나면(animationend·animationcancel·늦은 끝) 표지를 뗌,
// enter(발표 복제본), 늦춤 상한(타일 100개 — 0.3초 안). 실제 움직임은 CSS 라 TestBoardMotionCSS 가 규칙 꼴을, 화면은 Chrome 캡처가 봤다.
func TestBoardMotion(t *testing.T) { runBoardNode(t, "board_motion.cjs", "MOTION") }

// 보드 움직임의 CSS(3단계 C): 등장·강조 규칙은 @media screen and (prefers-reduced-motion: no-preference) 안에만 있다 — 인쇄·PDF·움직임 줄이기에는
// animation 이 없다. 끝 모양이 기본 모양이다 — keyframes 는 처음 모양(from)만 적고, 등장·강조 class 규칙은 opacity·transform·visibility 를 직접 적지 않는다
// (애니메이션이 돌지 않아도 타일이 다 보인다). 발표의 축소 복제본(.pr-thumb)과 움직임 줄이기는 present.css 가 모든 animation 을 끈다.
// present.js startBoard 는 발표 DOM 을 document.body 에 연결한 뒤 같은 동기 흐름에서 ItdaBoard.enter 를 부른다. CSS 를 해석하지 않는 꼴 검사다 — 계산 스타일은 Chrome 캡처로 봤다.
func TestBoardMotionCSS(t *testing.T) {
	read := func(n string) string {
		b, err := FS.ReadFile(n) // 내장 FS 로 읽어 go test -overlay 변이가 닿는다
		if err != nil {
			t.Fatal(err)
		}
		return regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(b), "")
	}
	css := read("base.css")
	const gate = "@media screen and (prefers-reduced-motion: no-preference) {"
	// 등장·강조 규칙을 고르는 블록: gate 안의 tile-enter·tile-changed 규칙
	var inGate, outGate []string
	for i := 0; i < len(css); {
		j := strings.Index(css[i:], gate)
		if j < 0 {
			outGate = append(outGate, css[i:])
			break
		}
		outGate = append(outGate, css[i:i+j])
		start := i + j + len(gate)
		depth, k := 1, start
		for ; k < len(css) && depth > 0; k++ {
			switch css[k] {
			case '{':
				depth++
			case '}':
				depth--
			}
		}
		inGate = append(inGate, css[start:k-1])
		i = k
	}
	rule := regexp.MustCompile(`(?s)([^{}]*)\{([^{}]*)\}`)
	found := map[string]bool{}
	for _, blk := range inGate {
		for _, m := range rule.FindAllStringSubmatch(blk, -1) {
			for _, c := range []string{"tile-enter", "tile-changed"} {
				if strings.Contains(m[1], "."+c) {
					found[c] = true
					if !strings.Contains(m[2], "animation:") {
						t.Errorf("%s 규칙에 animation 이 없음: %s", c, strings.TrimSpace(m[1]))
					}
					if regexp.MustCompile(`(^|[;\s])(opacity|transform|visibility)\s*:`).MatchString(m[2]) {
						t.Errorf("%s 규칙이 opacity·transform·visibility 를 직접 적음 — 끝 모양이 기본 모양이어야 한다: %s", c, strings.TrimSpace(m[2]))
					}
				}
			}
		}
	}
	if !found["tile-enter"] || !found["tile-changed"] {
		t.Errorf("움직임 줄이기를 묻는 화면 블록 안에 등장·강조 규칙이 없음: %v", found)
	}
	// 블록 밖(인쇄·화면 일반·keyframes 밖)에는 등장·강조 class 를 고르는 규칙이 없다
	for _, s := range outGate {
		for _, m := range rule.FindAllStringSubmatch(s, -1) {
			if strings.Contains(m[1], ".tile-enter") || strings.Contains(m[1], ".tile-changed") {
				t.Errorf("움직임 블록 밖의 등장·강조 규칙: %s", strings.TrimSpace(m[1]))
			}
		}
	}
	for _, kf := range []string{"itda-tile-enter", "itda-tile-flash"} {
		m := regexp.MustCompile(`(?s)@keyframes ` + kf + ` \{(.*?\})\s*\}`).FindStringSubmatch(css)
		if m == nil {
			t.Errorf("@keyframes %s 가 없음", kf)
			continue
		}
		if regexp.MustCompile(`(^|[\s}])(to|100%)\s*\{`).MatchString(m[1]) || !strings.Contains(m[1], "from {") {
			t.Errorf("@keyframes %s 는 처음 모양(from)만 적는다 — 끝 모양은 기본 모양: %s", kf, m[1])
		}
	}
	if !regexp.MustCompile(`@keyframes itda-tile-enter \{\s*from \{[^}]*opacity: 0;[^}]*transform: translateY`).MatchString(css) {
		t.Error("등장은 opacity·transform 만 움직인다(투명·조금 아래에서)")
	}
	pc := read("present.css")
	for _, want := range []string{".pr-thumb .page * { animation: none !important; }", "@media (prefers-reduced-motion: reduce) {\n  #present *, #present .page { animation: none !important;"} {
		if !strings.Contains(pc, want) {
			t.Errorf("present.css 에 %q 없음 — 축소 복제본·움직임 줄이기에서 타일 움직임이 샌다", want)
		}
	}
	js := read("present.js")
	a, b := strings.Index(js, "function startBoard("), strings.Index(js, "function boardFrac(")
	if a < 0 || b < a {
		t.Fatal("present.js 에서 startBoard 를 찾지 못함")
	}
	sb := js[a:b]
	// 차례: 남은 표지 지우기 → 덱에 넣기 → 발표 뿌리를 document.body 에 연결 → enter(c). enter 의 늦은 끝(lifeMs)은 계산 스타일을 읽으므로
	// 문서에 붙은 뒤라야 테마가 늘린 등장 시간을 안다(리뷰 2회차 [1]). 연결과 enter 사이에 await·타이머가 없다(같은 동기 흐름 — 칠하기 전).
	strip, d := strings.Index(sb, `c.querySelectorAll(".tile-enter, .tile-changed")`), strings.Index(sb, "deck.appendChild(c)")
	body, e := strings.Index(sb, "document.body.appendChild(root)"), strings.Index(sb, "window.ItdaBoard.enter(c)")
	switch {
	case strings.Count(sb, "window.ItdaBoard.enter(") != 1:
		t.Errorf("present.js startBoard 의 ItdaBoard.enter 호출이 %d 개 — 하나여야 함", strings.Count(sb, "window.ItdaBoard.enter("))
	case strip < 0 || d < strip || body < d || e < body:
		t.Error("present.js startBoard 의 차례가 「표지 지우기 → deck.appendChild(c) → document.body.appendChild(root) → ItdaBoard.enter(c)」 가 아님")
	case !strings.Contains(sb[body:e+len("window.ItdaBoard.enter(c)")], "o.motion !== false"):
		t.Error("present.js startBoard 의 enter 가 o.motion !== false 로 막히지 않음")
	case strings.Contains(sb[body:e], "await") || strings.Contains(sb[body:e], "setTimeout") || strings.Contains(sb[body:e], "requestAnimationFrame"):
		t.Error("present.js startBoard 의 연결과 enter 사이에 비동기 틈이 있음 — 표지 없는 그림이 한 번 칠해질 수 있다")
	}
}

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
