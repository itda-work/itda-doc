package page

import (
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/itda-work/itda-doc/themes"
	"github.com/itda-work/itda-doc/web"
)

// builtinProblems 는 내장 테마 묶음을 BuiltinThemeProblems 와 같은 검사로 돌려 갈래 k 의 문장(과 테마를 쓸 수 없음)을
// 시험 실패로 낸다. 갈래를 나눠 두어 시험 셋이 같은 문장을 겹쳐 내지 않는다.
func builtinProblems(t *testing.T, k themeCheck) {
	t.Helper()
	for _, p := range builtinThemeProblems(builtin(t)) {
		if p.kind == k || p.kind == checkTheme {
			t.Error(p.msg)
		}
	}
}

// 공개 함수는 갈래별 결과를 그대로 이은 것이고, 지금 내장 묶음에서는 빈 목록이다.
func TestBuiltinThemeProblemsClean(t *testing.T) {
	if ps := BuiltinThemeProblems(builtin(t)); len(ps) != 0 {
		t.Errorf("내장 테마 문제 %d 건:\n%s", len(ps), strings.Join(ps, "\n"))
	}
}

// copyTheme 는 내장 테마 name 을 메모리 FS 로 복사한다(저장소 파일은 바꾸지 않는다).
func copyTheme(t *testing.T, name string) fstest.MapFS {
	t.Helper()
	m := fstest.MapFS{}
	err := fs.WalkDir(themes.FS, name, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(themes.FS, p)
		m[p] = &fstest.MapFile{Data: b}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func editFile(t *testing.T, m fstest.MapFS, p, old, new string) {
	t.Helper()
	s := string(m[p].Data)
	if !strings.Contains(s, old) {
		t.Fatalf("%s 에 %q 가 없음", p, old)
	}
	m[p] = &fstest.MapFile{Data: []byte(strings.Replace(s, old, new, 1))}
}

// 부정 시험: 내장 테마 하나를 복사해 한 곳씩 망가뜨리면 BuiltinThemeProblems 가 그 갈래로 잡는다.
func TestBuiltinThemeProblemsCatches(t *testing.T) {
	cases := []struct {
		name   string
		break_ func(t *testing.T, m fstest.MapFS)
		want   string // 문장에 들어 있어야 할 글자(느슨하게)
	}{
		{"파생 정적 값", func(t *testing.T, m fstest.MapFS) {
			editFile(t, m, "classic/theme.json", `"--accent-2": "#4d6da2"`, `"--accent-2": "#4d6da3"`)
		}, "--accent-2 의 정적 값 #4d6da3 ≠ 식의 값"},
		{"tokens 의 from 식", func(t *testing.T, m fstest.MapFS) {
			editFile(t, m, "classic/theme.json", `"--accent-3": "#90a7cc"`, `"--accent-3": "oklch(from var(--accent) l c h)"`)
		}, "tokens 의 --accent-3 에 상대 색 식"},
		{"@scope 밖", func(t *testing.T, m fstest.MapFS) {
			// 내장 테마의 CSS 는 늘 @scope 로 감싸진다 — 감쌀 theme.css 가 없으면 감싼 꼴이 나오지 않는다.
			delete(m, "classic/theme.css")
		}, "@scope 안에 있지 않음"},
		{"슬라이드 고정 크기", func(t *testing.T, m fstest.MapFS) {
			p := "classic/theme.css"
			m[p] = &fstest.MapFile{Data: append(m[p].Data, "\n.sc-title { font-size: 12pt }\n"...)}
		}, ".sc-title { font-size: 12pt }"},
		{"어휘 밖 group", func(t *testing.T, m fstest.MapFS) {
			editFile(t, m, "classic/theme.json", `"group": "basic"`, `"group": "fun"`)
		}, `group "fun" 는 모르는 묶음`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := copyTheme(t, "classic")
			if ps := BuiltinThemeProblems(mustLoad(t, m)); len(ps) != 0 {
				t.Fatalf("망가뜨리기 전부터 문제: %v", ps)
			}
			c.break_(t, m)
			ps := BuiltinThemeProblems(mustLoad(t, m))
			if !slices.ContainsFunc(ps, func(p string) bool { return strings.HasPrefix(p, "classic: ") && strings.Contains(p, c.want) }) {
				t.Errorf("못 잡음 — 바란 글자 %q, 받은 것 %q", c.want, ps)
			}
		})
	}
}

func mustLoad(t *testing.T, fsys fs.FS) *Themes {
	t.Helper()
	ts, err := LoadThemes(fsys)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

// 내장 테마의 규칙은 .page 로 시작하지 않는다 — @scope (.page) 안에서는 그런 선택자가 죽는다(itda-work/itda-hyve#362 리뷰 1회차 [1]).
// 보드 절이 실제로 적용되는지(계산된 스타일)는 Chrome 헤드리스로 따로 확인했다 — 이 시험은 그 꼴이 되살아나는 것을 막는다.
func TestBuiltinThemesNoPageRoot(t *testing.T) {
	ts := builtin(t)
	for _, i := range ts.List() {
		th, _ := ts.Get(i.Name)
		for _, p := range pageRootProblems(th.CSS) {
			t.Errorf("%s: %s", i.Name, p)
		}
	}
	// 부정·긍정: 잡을 꼴과 잡지 않을 꼴
	for css, want := range map[string]int{
		".page.board .tile { color: red }":                   1,
		"@media print { .page .x, .y { color: red } }":       1,
		".page { color: red }":                               1,
		".page-body .tile, .board-grid .tile { color: red }": 0,
		".pages .x { color: red } .board-deco::before { }":   0,
		"@font-face { font-family: x; src: url(a.woff2) }":   0,
		".x :is(.page.board, .y) { color: red }":             0, // 맨 앞이 아니면 자손 뜻이 다르다 — 검사 밖
	} {
		if got := len(pageRootProblems(css)); got != want {
			t.Errorf("pageRootProblems(%q) = %d건, 바란 것 %d", css, got, want)
		}
	}
}

// boardBaseSelOK 는 base.css 보드 절의 선택자 조각이 허용 꼴인가다 — 보드 자신, 격자 안(.page-body.board-grid 뿌리), 머리·장식.
func boardBaseSelOK(p string) bool {
	const scope = ":where(.page.board) "
	if p == ".page.board" {
		return true
	}
	rest, ok := strings.CutPrefix(p, scope)
	if !ok {
		return false
	}
	if rest == ".page-body.board-grid" || strings.HasPrefix(rest, ".page-body.board-grid ") {
		return true
	}
	return strings.HasPrefix(rest, ".board-deco") || strings.HasPrefix(rest, ".board-head") || strings.HasPrefix(rest, ".bh-")
}

// boardThemeSelOK 는 테마 선택자 조각이 보드 격자 쪽 어휘(.tile·.tile-*·.board-section·.kpi-item·.board-grid)를 쓰면 .page-body.board-grid 뿌리인가다.
// 그 어휘가 없는 조각(머리·장식 .board-head·.board-deco·.bh-*, 쪽 문서 규칙)은 이 검사와 상관없다.
func boardThemeSelOK(p string) bool {
	if !strings.Contains(p, ".tile") && !strings.Contains(p, ".kpi-item") && !strings.Contains(p, ".board-grid") && !strings.Contains(p, ".board-section") {
		return true
	}
	return p == ".page-body.board-grid" || strings.HasPrefix(p, ".page-body.board-grid ")
}

// 보드 선택자의 꼴(itda-work/itda-hyve#362 접근 방식 리뷰, 리뷰 2회차 [1]·[2]) — base.css 보드 절의 모든 조각이 허용 꼴이다.
// 격자 안이 :where(.page.board) .page-body.board-grid 뿌리라야 테마 보드 절과 우선순위가 같고 쪽 문서용 일반 규칙을 이기며, 가둠이 없으면
// 문서의 {.tile} 이 쪽·슬라이드로 샌다. 이 시험은 꼴 검사다 — 렌더가 아니다. 우선순위 결과(계산 스타일)는 Chrome 헤드리스로만 확인했고 WebKit 은 미검증이다.
func TestBaseBoardSelectorShape(t *testing.T) {
	b, err := web.FS.ReadFile("base.css")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	a := strings.Index(s, "/* ---- 보드(")
	if a < 0 {
		t.Fatal("base.css 에서 보드 절을 찾지 못함")
	}
	sec := s[a:]
	if z := strings.Index(sec[1:], "/* ---- "); z >= 0 {
		sec = sec[:z+1]
	}
	rules, err := slideCSSRules(sec)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, r := range rules {
		for _, p := range slideSelectorPieces(r.Prelude) {
			n++
			if !boardBaseSelOK(p) {
				t.Errorf("base.css 보드 절 %q — .page.board, :where(.page.board) .page-body.board-grid …, :where(.page.board) + .board-deco·.board-head·.bh-* 가운데 하나로 시작한다", p)
			}
		}
	}
	if n < 10 {
		t.Errorf("보드 절 선택자 조각이 %d개 — 보드 절을 못 읽었을 수 있다", n)
	}
	for p, want := range map[string]bool{
		".page.board": true, ":where(.page.board) .page-body.board-grid": true, ":where(.page.board) .page-body.board-grid .tile": true, ":where(.page.board) .bh-title": true,
		".tile": false, ".board-grid .tile": false, ".page.board .tile": false, ":where(.page.board) .board-grid .tile": false, ":where(.page.board) .tile": false,
	} {
		if boardBaseSelOK(p) != want {
			t.Errorf("boardBaseSelOK(%q) != %v", p, want)
		}
	}
}

// 내장 테마의 보드 격자 규칙은(extends 로 물려받은 것 포함) .page-body.board-grid 로 시작한다 — .board-grid .tile 꼴은 base 에 져서 보드 절이 죽는다.
// 이 어휘가 쪽 문서 규칙에 우연히 쓰여도 같은 검사로 드러난다. 이 시험도 꼴 검사다 — 렌더가 아니다(Chrome 실측, WebKit 미검증).
func TestBuiltinThemeBoardSelectorShape(t *testing.T) {
	ts := builtin(t)
	for _, i := range ts.List() {
		th, _ := ts.Get(i.Name)
		rules, err := slideCSSRules(th.CSS)
		if err != nil {
			t.Fatalf("%s: %v", i.Name, err)
		}
		n, sec := 0, 0
		for _, r := range rules {
			for _, p := range slideSelectorPieces(r.Prelude) {
				if strings.HasPrefix(p, ".page-body.board-grid ") {
					n++
				}
				if strings.HasPrefix(p, ".page-body.board-grid .board-section") {
					sec++
				}
				if !boardThemeSelOK(p) {
					t.Errorf("%s: %q — 보드 격자 규칙은 .page-body.board-grid 로 시작한다", i.Name, p)
				}
			}
		}
		if n == 0 {
			t.Errorf("%s: 보드 격자 규칙이 0개 — 보드 절을 못 읽었을 수 있다", i.Name)
		}
		if sec == 0 {
			t.Errorf("%s: 구역 제목 줄(.page-body.board-grid .board-section) 규칙이 없다", i.Name)
		}
	}
	for p, want := range map[string]bool{
		".page-body.board-grid .tile": true, ".board-head": true, ".bh-title": true, ".board-deco::before": true, ".page-body h2": true,
		".board-grid .tile": false, ".tile": false, ".page-body .kpi-item": false, ".board-head .tile": false,
		".page-body.board-grid .board-section": true, ".board-section": false,
	} {
		if boardThemeSelOK(p) != want {
			t.Errorf("boardThemeSelOK(%q) != %v", p, want)
		}
	}
}
