package page

import (
	"regexp"
	"strings"
	"testing"

	"github.com/itda-work/itda-doc/web"
)

// 할 일 표 머리(강조색 면) 안 인라인(itda-doc#41). 이 시험은 문자열과 토큰 값만 묶는다 — 규칙이 실제로 그 요소에 걸리는지, 각주 번호가 상자 안에
// 놓이는지 같은 배치·계산값은 저장소 시험에 브라우저가 없어 묶지 못하고 렌더한 계산값·PNG 로 확인한다.
var actionsHeadTokens = []string{
	"--link: currentColor", "--link-hover-ink: currentColor", "--link-hover-bg: transparent", "--link-hover-line: currentColor",
	"--mark-ink: var(--ink)", "--del-ink: currentColor", "--kbd-line: currentColor",
	"--head-code-bg: transparent", "--head-code-line: inset 0 0 0 0.75pt currentColor",
}

func baseCSSNoComments(t *testing.T) string {
	t.Helper()
	b, err := web.FS.ReadFile("base.css")
	if err != nil {
		t.Fatal(err)
	}
	return regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(b), "")
}

// 토큰은 할 일 표 머리에서만 값을 얻고(:root 나 다른 자리에 두면 본문·다른 표 머리로 샌다), 기존 규칙이 값이 없을 때의 옛 값으로 떨어져야
// 어휘를 쓰지 않는 문서가 그대로다. 결재란 머리는 투명 바탕이라 뺀다.
func TestActionsHeadInlineBaseCSS(t *testing.T) {
	css := baseCSSNoComments(t)
	m := regexp.MustCompile(`:where\(\.actions th:not\(\.approval th\)\) \{([^}]*)\}`).FindStringSubmatch(css)
	if m == nil {
		t.Fatal("base.css 에 할 일 표 머리 규칙(:where(.actions th:not(.approval th)))이 없음")
	}
	for _, want := range actionsHeadTokens {
		if !strings.Contains(m[1], want) {
			t.Errorf("할 일 표 머리 규칙에 %q 가 없음", want)
		}
	}
	for _, want := range []string{
		`.page-body mark { background: #fff1a8; color: var(--mark-ink, inherit);`,
		`border: 0.75pt solid var(--kbd-line, var(--rule));`,
		`.page-body del { color: var(--del-ink, var(--muted)); }`,
		`.page-body th code { color: inherit; background: var(--head-code-bg, transparent); }`,
		`.page-body th code { background: var(--head-code-bg, color-mix(in srgb, currentColor 16%, transparent)); }`,
		`.page-body th pre code { background: none; }`,
		`:where(.actions th:not(.approval th) code:not(pre code)) { box-shadow: var(--head-code-line); }`,
		`.footnote-ref { font-size: 0.75em; vertical-align: var(--footnote-align, super);`,
		`:where(.page-body th :is(kbd, mark)) { --footnote-align: baseline; }`,
		`:where(.page-body th :is(kbd, mark) sup) { vertical-align: 0.3em; line-height: 0; }`,
		`:where(.decisions li) { position: relative; padding-left: max(15pt, 1.35em);`,
	} {
		if !strings.Contains(css, want) {
			t.Errorf("base.css 에 %q 가 없음", want)
		}
	}
	for _, tok := range []string{"--mark-ink", "--del-ink", "--kbd-line", "--head-code-bg", "--head-code-line", "--footnote-align"} {
		if n := len(regexp.MustCompile(tok+`\s*:`).FindAllString(css, -1)); n != 1 {
			t.Errorf("%s 를 정하는 자리가 %d 곳 — 머리 규칙 한 곳이어야 함", tok, n)
		}
	}
	// code 의 box-shadow 는 base 에서 할 일 표 머리 규칙 한 곳만 선언한다 — 다른 자리에 선언이 생기면(none 이어도) 특이도로 회사 테마의
	// .page-body code 장식을 지운다(일반 표·결재란 머리, itda-doc#41 리뷰 1회차 [1])
	// 선택자 전체를 견주고 규칙 수를 센다 — 접두사만 보면 같은 머리로 시작해 목록으로 다른 자리까지 넓힌 규칙을 놓친다(리뷰 2회차 [3])
	const shadowSel = ":where(.actions th:not(.approval th) code:not(pre code))"
	shadowRules := 0
	for _, m := range regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`).FindAllStringSubmatch(css, -1) {
		sel := strings.TrimSpace(m[1])
		if !regexp.MustCompile(`\bcode\b`).MatchString(sel) || !strings.Contains(m[2], "box-shadow") {
			continue
		}
		shadowRules++
		if sel != shadowSel {
			t.Errorf("할 일 표 머리 밖 code 규칙에 box-shadow 선언: %s", sel)
		}
	}
	if shadowRules != 1 {
		t.Errorf("code 의 box-shadow 규칙이 %d 개 — 할 일 표 머리 규칙 하나여야 함", shadowRules)
	}
}

// base 가 맡은 자리의 임시 규칙이 새 테마에 되살아나지 않고, 진한 일반 표 머리는 바탕 테마가 맡으며, 그것을 물려받되 표 머리가 투명한 swiss 는 되돌린다.
func TestActionsHeadThemeCSS(t *testing.T) {
	ts := builtin(t)
	css := func(name string) string {
		th, ok := ts.Get(name)
		if !ok {
			t.Fatalf("%s 가 없음", name)
		}
		return th.CSS
	}
	// 주석을 빼고, 허용한 되돌리기 선택자(swiss)만 지운 나머지에서 찾는다 — 그 한 줄이 있다고 파일 전체를 넘기면 옛 규칙이 되살아나도 모른다
	comment := regexp.MustCompile(`(?s)/\*.*?\*/`)
	actionsHead := regexp.MustCompile(`\.actions\b[^{},]*\bth\b`)
	for _, name := range []string{"swiss", "contrast", "editorial", "verdant"} {
		s := strings.ReplaceAll(comment.ReplaceAllString(css(name), ""), ":where(.page-body th:not(.actions th))", "")
		if m := actionsHead.FindString(s); m != "" {
			t.Errorf("%s: base 가 맡은 %q 규칙이 테마에 남음", name, m)
		}
	}
	for _, name := range []string{"contrast", "editorial", "verdant"} {
		if strings.Contains(css(name), ".decisions li { padding-left: 1.4em") {
			t.Errorf("%s: base 가 맡은 decisions 들여쓰기 규칙이 테마에 남음", name)
		}
	}
	if !strings.Contains(css("compact"), "padding-left: max(15pt, 1.35em)") {
		t.Error("compact: 자기 decisions 규칙이 글자 크기를 따르지 않음")
	}
	const head = ":where(.page-body th:not(.approval th)) { --link: currentColor;"
	for _, name := range []string{"bold", "spine", "verdant"} {
		if !strings.Contains(css(name), head) {
			t.Errorf("%s: 진한 표 머리 규칙이 없음", name)
		}
	}
	// 같은 특이도(0)라 뒤에 와야 되돌린다
	s := css("swiss")
	back := strings.Index(s, ":where(.page-body th:not(.actions th)) { --link: inherit;")
	if back < 0 || back < strings.Index(s, head) {
		t.Error("swiss: bold 의 진한 표 머리 규칙을 그 뒤에서 되돌리지 않음")
	}
}

// 전 내장 테마의 할 일 표 머리 글자 / 면 대비. base 가 .actions 안에서 --table-head-* 를 --accent-ink·--accent 로 다시 정해(web/base.css),
// 이 값이 곧 머리 글자·링크·각주·code·del·kbd 가 면 위에 놓이는 대비다. compact 는 같은 두 토큰을 color·background 로 직접 정한다.
// bold 는 #41 전부터 4.30:1 로 기준 아래다 — 면 색을 바꾸면 표지·상자 모양이 바뀌어 #41 범위 밖이라 지금 값을 바닥으로 묶고 그대로 둔다.
func TestActionsHeadContrastEveryBuiltinTheme(t *testing.T) {
	below := map[string]float64{"bold": 4.3}
	ts := builtin(t)
	seen := 0
	for _, info := range ts.List() {
		th, ok := ts.Get(info.Name)
		if !ok {
			t.Fatalf("%s 를 찾지 못함", info.Name)
		}
		seen++
		vals := map[string]string{"--accent": "#2b4a7e", "--accent-ink": "#ffffff"} // base.css 기본값
		for _, kv := range th.Tokens {
			vals[kv[0]] = kv[1]
		}
		ink, face := vals["--accent-ink"], vals["--accent"]
		if len(ink) != 7 || len(face) != 7 || ink[0] != '#' || face[0] != '#' {
			t.Fatalf("%s: 색 값을 풀지 못함 %q / %q", info.Name, ink, face)
		}
		r := contrastRatio(strings.ToLower(ink), strings.ToLower(face))
		min := 4.5
		if b, ok := below[info.Name]; ok {
			min = b
		}
		if r < min-0.005 {
			t.Errorf("%s: 할 일 표 머리 %s on %s = %.2f:1 — %.2f:1 이상이어야 함", info.Name, ink, face, r, min)
		}
	}
	if seen < 10 {
		t.Errorf("내장 테마를 %d 개만 봄", seen)
	}
}
