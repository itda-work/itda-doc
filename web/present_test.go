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

// 발표·판서(present.js·ink.js·inkshape.js): 판서 모형(획 추가·되돌리기·다시 실행·상한·탭 수명)·기하·바탕 밝기와 발표의 넘김·차례로 나타내기,
// 호스트 훅 계약(present.js 머리 주석 — 전체 화면·호스트 전용 앵커·키 양보 ownsKey·겹침 층, 크게 보기·코드 복사 부품이 없을 때),
// 두 화면 발표(itda-doc#95 — onState·audience 열기·show 덮어쓰기)를, 펜만 판서(itda-doc#100 — penOnly 입력 나눔·inkHeld·발표 중 setPenOnly)를,
// 판서 받아 그리기(itda-doc#108 — ink.js view 연산·멱등·reset·레이저, 청중 열기의 붙이기, 발표자·청중의 같은 경로)를
// 가짜 DOM 위에서 node 로 돌린다(testdata/present_ink.cjs). 겹침 층의 CSS 는 TestPresentOverlayCSS 가, 실제 창 안 동작은 호스트 앱의 시험이 본다.
const presentInkCases = 595 // itda-hyve#979 표지 id 가 덱 사본·미리보기·썸네일·보드 사본에 남지 않음 5, itda-hyve-ios#24 발표자 보조(다음 장 미리보기·현재 시각·onState next) 26·미리보기 그림·표 번호 6·배치된 캡션만·장별 캐시 4, itda-doc#95 두 화면 발표 32·끝 콜백 재진입 8·청중 덱 입력 7, itda-doc#100 펜만 판서 28·발표 중 바꾸기(setPenOnly) 26(떼기 놓친 같은 id 의 새 누르기 회수 포함), itda-doc#99 장 넘김 전환 101, itda-doc#105 전환 중 판서 입력 173, itda-doc#108 판서 받아 그리기 97(받는 경계·입력 예산·되풀이 입력·레이저 넘김 포함)

func TestPresentInk(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다 — ITDA_DOC_STRICT=1(just test·CI)은 node 가 필요하다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	dir := presentSources(t)
	runner, _ := filepath.Abs(filepath.Join("testdata", "present_ink.cjs"))
	out, err := exec.Command(node, runner, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("실행 검사 실패: %v\n%s", err, out)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(out)), fmt.Sprintf("PRESENT INK DONE cases=%d failed=0", presentInkCases)) {
		t.Fatalf("사례 %d 개를 모두 마치지 않았다\n%s", presentInkCases, out)
	}
}

// presentSources 는 node 실행 검사가 읽을 폴더다 — 보통은 web/ 그대로다. ITDA_DOC_TEST_OVERLAY 가 가리키는 overlay JSON 의 Replace 에
// present.js·ink.js·inkshape.js 가 있으면 셋을 임시 폴더에 바꿔 끼운 채 모아 그 폴더를 준다(뮤테이션 시험이 디스크의 원문 대신 바꿔 끼운
// 소스를 보게 — pagerSource 와 같은 약속).
func presentSources(t *testing.T) string {
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
	for _, name := range []string{"present.js", "ink.js", "inkshape.js", "ask.js"} {
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

// 발표·판서는 FS 에 들고, 특정 앱의 창 API·이름을 부르지 않는다 — 창 틀은 호스트 훅(window.ItdaPresentHost)으로만 만난다.
func TestPresentInkFS(t *testing.T) {
	bad := regexp.MustCompile(`window\.go\b|window\.runtime\b|(?i:hyve)|(?i:wails)`)
	for _, name := range []string{"present.js", "ink.js", "inkshape.js", "present.css"} {
		b, err := fs.ReadFile(FS, name)
		if err != nil {
			t.Fatalf("FS 에 %s 가 없다: %v", name, err)
		}
		if m := bad.FindString(string(b)); m != "" {
			t.Errorf("%s 에 %q — 창 틀은 호스트 훅으로만 만난다", name, m)
		}
	}
	src, _ := fs.ReadFile(FS, "present.js")
	if !strings.Contains(string(src), "window.ItdaPresent = {") {
		t.Error("present.js 의 전역이 window.ItdaPresent 가 아니다")
	}
}

// 장 넘김 전환(itda-doc#99)의 CSS 글: present.js TRANSITIONS 의 값마다(none 밖) .t-* 들어오기·나가기 규칙이 있고, 방향이 있는 전환은
// 뒤로(pr-back) 규칙도 있다. 전환 keyframes 는 transform·opacity 만 움직이고(합성기 — clip-path·filter·크기·자리 속성 없음), 나가는 장의
// 움직임은 지연 포함 OUT_MS 안에 끝난다(그 뒤 pr-out 이 떨어져 장이 숨는다 — 끝나기 전에 숨으면 중간 모습에서 툭 사라진다).
// 가짜 DOM 은 CSS 를 그리지 않아 규칙 글을 본다. 실제 그리기(120Hz·저사양)는 미검증이다.
func TestPresentTransitionsCSS(t *testing.T) {
	cssB, err := fs.ReadFile(FS, "present.css")
	if err != nil {
		t.Fatal(err)
	}
	jsB, err := fs.ReadFile(FS, "present.js")
	if err != nil {
		t.Fatal(err)
	}
	css, js := string(cssB), string(jsB)
	m := regexp.MustCompile(`const TRANSITIONS = \[([^\]]*)\], OUT_MS = (\d+);`).FindStringSubmatch(js)
	if m == nil {
		t.Fatal("present.js 에 TRANSITIONS·OUT_MS 가 없다")
	}
	var names []string
	for q := range strings.SplitSeq(m[1], ",") {
		names = append(names, strings.Trim(strings.TrimSpace(q), `"`))
	}
	if strings.Join(names, "·") != "none·fade·slide·zoom·push·cover" {
		t.Errorf("TRANSITIONS %v — mdoc 스키마(slides.transition)와 같은 값·차례여야 한다", names)
	}
	var outMs float64
	fmt.Sscan(m[2], &outMs)
	if !strings.Contains(js, "}, OUT_MS));") {
		t.Error("나가는 장 정리 타이머가 OUT_MS 를 쓰지 않는다")
	}
	directional := map[string]bool{"slide": true, "zoom": true, "push": true}
	for _, n := range names[1:] {
		pre := ".t-" + n + " .pr-deck .page."
		if !strings.Contains(css, pre+"pr-in {") {
			t.Errorf("%s: 들어오는 장 규칙이 없다", n)
		}
		if directional[n] {
			for _, r := range []string{"pr-in.pr-back {", "pr-out {", "pr-out.pr-back {"} {
				if !strings.Contains(css, pre+r) {
					t.Errorf("%s: %s 규칙이 없다", n, r)
				}
			}
		}
	}
	// cover: 앞으로는 새 장이 덮고(옛 장 제자리), 뒤로는 옛 장을 위 층에 올려 걷어낸다
	for _, want := range []string{
		".t-cover .pr-deck .page.pr-in.pr-back { animation: none; }",
		".t-cover .pr-deck .page.pr-out.pr-back { z-index: 3; animation: pr-cover-out-back ",
		".t-push .pr-deck, .t-cover .pr-deck { overflow: hidden; }",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("present.css 에 %q 가 없다", want)
		}
	}
	// 동작 줄이기(리뷰 1회차 [1]): 발표 중에 켜면 루트에 t-cover 등이 남는다 — 미디어 규칙이 움직임을 끄고 나가는 장을 곧바로 숨겨야
	// cover 뒤로(z-index 3)의 나가는 장이 OUT_MS 동안 새 장을 가리지 않는다. 이 숨김은 pr-out 의 visible 규칙(!important 아님)을 이긴다
	rm := regexp.MustCompile(`(?s)@media \(prefers-reduced-motion: reduce\) \{(.*?)\n\}`).FindStringSubmatch(css)
	if rm == nil {
		t.Fatal("present.css 에 여러 줄 prefers-reduced-motion 규칙이 없다")
	}
	for _, want := range []string{
		"#present *, #present .page { animation: none !important; transition: none !important; }",
		"#present .pr-deck .page.pr-out { visibility: hidden !important; }",
	} {
		if !strings.Contains(rm[1], want) {
			t.Errorf("동작 줄이기 규칙에 %q 가 없다", want)
		}
	}
	if regexp.MustCompile(`\.pr-out[^{]*\{[^}]*visibility: visible !important`).MatchString(css) {
		t.Error("나가는 장의 visible 이 !important 라 동작 줄이기 숨김을 이긴다")
	}
	// keyframes: transform·opacity 만
	kf := regexp.MustCompile(`@keyframes (pr-(?:fade|slide|zoom|push|cover)[\w-]*) \{(.*)\}\n`)
	prop := regexp.MustCompile(`([a-z-]+)\s*:`)
	seen := 0
	for _, k := range kf.FindAllStringSubmatch(css, -1) {
		seen++
		for _, p := range prop.FindAllStringSubmatch(k[2], -1) {
			if p[1] != "transform" && p[1] != "opacity" {
				t.Errorf("@keyframes %s 가 %s 를 움직인다 — transform·opacity 만", k[1], p[1])
			}
		}
	}
	if seen < 14 {
		t.Errorf("전환 keyframes %d 개만 찾았다", seen)
	}
	// 나가는 장의 움직임 길이(지연 포함) ≤ OUT_MS
	out := regexp.MustCompile(`\.t-(\w+) \.pr-deck \.page\.pr-out(?:\.pr-back)? \{[^}]*animation: [\w-]+ ([\d.]+)s[^;]*?(?: ([\d.]+)s)? both;`)
	outs := out.FindAllStringSubmatch(css, -1)
	if len(outs) < 5 { // fade·slide·zoom·push 의 pr-out, cover 의 pr-out.pr-back(뒤로 규칙은 animation-name 만 바꿔 길이를 물려받는다)
		t.Errorf("나가는 장 규칙 %d 개만 찾았다", len(outs))
	}
	for _, o := range outs {
		var d, delay float64
		fmt.Sscan(o[2], &d)
		if o[3] != "" {
			fmt.Sscan(o[3], &delay)
		}
		if (d+delay)*1000 > outMs {
			t.Errorf("%s 나가는 장 %gs+%gs 가 OUT_MS %gms 를 넘는다: %s", o[1], d, delay, outMs, o[0])
		}
	}
}

// 다음 장 미리보기의 카운터(present.js NEXT_COUNTERS — 1회차 리뷰 [1])는 base.css 에서 장을 넘어 이어지는 카운터와 짝이 맞는다: 쪽 상자 조상(.pages)의
// counter-reset 이름 모두가 NEXT_COUNTERS 에 있고, 이름마다 선택자가 base.css 의 `<선택자>::before { counter-increment: <이름> }` 선택자와 글자 그대로 같다.
// base.css 에 장을 넘는 카운터를 더하거나 증가 선택자를 바꾸면 여기서 실패한다 — present.js 상수도 함께 고친다.
func TestPresentCountersCSS(t *testing.T) {
	css, err := fs.ReadFile(FS, "base.css")
	if err != nil {
		t.Fatal(err)
	}
	js, err := fs.ReadFile(FS, "present.js")
	if err != nil {
		t.Fatal(err)
	}
	reset := regexp.MustCompile(`(?m)^\.pages \{ counter-reset: ([^;]+); \}`).FindSubmatch(css)
	if reset == nil {
		t.Fatal("base.css 에 .pages { counter-reset: … } 가 없다")
	}
	inc := map[string]string{}
	for _, m := range regexp.MustCompile(`(?m)^([^{}\n]+)::before \{[^}]*counter-increment: (\w+)`).FindAllSubmatch(css, -1) {
		inc[string(m[2])] = strings.TrimSpace(string(m[1]))
	}
	decl := regexp.MustCompile(`const NEXT_COUNTERS = \[(.*)\];`).FindSubmatch(js)
	if decl == nil {
		t.Fatal("present.js 에 NEXT_COUNTERS 가 없다")
	}
	got := map[string]string{}
	for _, m := range regexp.MustCompile(`\["(\w+)", "([^"]+)"\]`).FindAllSubmatch(decl[1], -1) {
		got[string(m[1])] = string(m[2])
	}
	names := strings.Fields(string(reset[1]))
	if len(got) != len(names) {
		t.Errorf("NEXT_COUNTERS %v 와 .pages 의 카운터 %v 의 수가 다르다", got, names)
	}
	for _, n := range names {
		if inc[n] == "" {
			t.Errorf("base.css 에 %s 의 증가 규칙이 없다", n)
		} else if got[n] != inc[n] {
			t.Errorf("카운터 %s: NEXT_COUNTERS 선택자 %q ≠ base.css 증가 선택자 %q", n, got[n], inc[n])
		}
	}
}

// 겹침 층([data-pr-overlay] — present.js 머리 주석 「호스트 계약」)의 CSS: body.presenting 숨김에서 빠지고 #present(z-index 100) 위에 놓이며,
// 가림 클래스(pr-overlay-hidden — present.js syncOverlay) 동안 층과 그 자손이 모두 가려지고(display 는 바꾸지 않는다), #present 의 커서 숨김을 물려받지 않는다. 가짜 DOM 은 CSS 를 그리지 않아 규칙 글을 본다.
func TestPresentOverlayCSS(t *testing.T) {
	b, err := fs.ReadFile(FS, "present.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(b)
	for _, want := range []string{
		"body.presenting > :not(#present):not([data-pr-overlay]) { visibility: hidden !important; }",
		"body.presenting > [data-pr-overlay] { z-index: 101; }",
		"body.presenting.pr-overlay-hidden [data-pr-overlay], body.presenting.pr-overlay-hidden [data-pr-overlay] * { visibility: hidden !important; }",
		"#present.pr-idle [data-pr-overlay] { cursor: auto; }",
		"#present { position: fixed; inset: 0; z-index: 100;",
		// 청중 열기(present.js 「두 화면 발표」)는 커서 깨우기가 없어 늘 숨기고, 겹침 층은 되살린다
		"#present.pr-audience { cursor: none; }",
		"#present.pr-audience [data-pr-overlay] { cursor: auto; }",
		// 청중 열기의 덱 사본 입력 부품(섹션 묻기 단추·코드 복사 단추)은 마우스 올림·포커스로도 드러나지 않는다
		"#present .itda-section-ask, #present.pr-audience .pr-deck .itda-codecopy { display: none !important; }",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("present.css 에 %q 가 없다", want)
		}
	}
	if strings.Contains(css, "body.presenting > :not(#present) {") {
		t.Error("present.css 의 발표 숨김이 겹침 층까지 가린다")
	}
}
