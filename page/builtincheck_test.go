package page

import (
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/itda-work/itda-doc/themes"
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
