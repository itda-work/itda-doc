package page

import (
	"strings"
	"testing"
)

// 옛 WebKit 은 상대 색을 모른다 — 내장 테마는 정적 값(tokens)을 쓰고, 지원하는 엔진만 식(derived)으로 덮는다.
// 내장 테마의 강조색은 고정이므로 정적 값은 그 강조색으로 계산한 식의 값과 같아야 한다(어느 엔진에서나 같은 PDF).
// 계산은 CSS Color 4 의 OKLab(sRGB 로 자르기)이다 — WebKit 이 그린 값과 8개 모두 같았다(실측).
func TestDerivedTokensMatchStatic(t *testing.T) {
	ts := builtin(t)
	n := 0
	for _, i := range ts.List() {
		th, _ := ts.Get(i.Name)
		vals := map[string]string{}
		for _, kv := range th.Tokens {
			vals[kv[0]] = kv[1]
		}
		for _, kv := range th.Derived {
			n++
			got, err := evalRelColor(kv[1], vals)
			if err != nil {
				t.Errorf("%s %s: %v", i.Name, kv[0], err)
				continue
			}
			if !strings.EqualFold(got, vals[kv[0]]) {
				t.Errorf("%s %s: 정적 값 %s ≠ 식의 값 %s (%s)", i.Name, kv[0], vals[kv[0]], got, kv[1])
			}
		}
	}
	if n < 8 {
		t.Errorf("파생 토큰이 %d 개뿐", n)
	}
	// 내장 테마의 tokens 에는 상대 색 식이 없다(옛 엔진에서 무효가 된다)
	for _, i := range ts.List() {
		th, _ := ts.Get(i.Name)
		for _, kv := range th.Tokens {
			if strings.Contains(kv[1], "from ") {
				t.Errorf("%s: tokens 의 %s 에 상대 색 식 — derived 로 옮겨라", i.Name, kv[0])
			}
		}
	}
}

// 엔진에 맞춘 CSS 꼴: 파생 토큰은 @supports 안(시험 흉내면 늘 거짓인 조건), @scope 를 모르는 엔진이면 내장만 가두지 않는다.
func TestStyleSheetModes(t *testing.T) {
	ts, err := LoadThemeSet(builtinFS(t), "../themes/samples")
	if err != nil {
		t.Fatal(err)
	}
	classic, _ := ts.Get("classic")
	css := classic.StyleSheet(cssMode{})
	sup := strings.Index(css, "@supports "+relColorSupports+" {\n:root {\n  --accent-2: oklch(from")
	root := strings.Index(css, "--accent-2: #4d6da2;")
	if root < 0 || sup < root || !strings.Contains(css, "@scope (.page) {") {
		t.Fatalf("정적 값 → @supports 파생 → @scope 순이어야 함\n%s", css[:min(len(css), 1500)])
	}
	if c := classic.StyleSheet(cssMode{fakeNoRelColor: true}); !strings.Contains(c, "@supports "+fakeFalseSupports) || strings.Contains(c, relColorSupports) {
		t.Error("상대 색 끄기 흉내가 조건을 바꾸지 않음")
	}
	un := classic.StyleSheet(cssMode{unscoped: true})
	if strings.Contains(un, "@scope") || !strings.Contains(un, "/* ---- classic ---- */") {
		t.Error("@scope 를 모르는 엔진에서 내장 테마는 가두지 않고 내야 함")
	}
	if un == css {
		t.Error("꼴이 같음")
	}
	acme, _ := ts.Get("acme")
	if c := acme.StyleSheet(cssMode{unscoped: true}); !strings.Contains(c, "@scope (.page) {") {
		t.Error("사용자 테마는 어떤 꼴로도 가두지 않은 CSS 로 내지 않는다")
	}
	// acme 는 --accent 만 준다 → 물려받은 파생 식이 지원 엔진에서 붉은 계열을 만든다(정적 값은 남색 그대로)
	if c := acme.StyleSheet(cssMode{}); !strings.Contains(c, "--accent-soft: #ebf0f9;") || !strings.Contains(c, "--accent-soft: oklch(from var(--accent)") {
		t.Error("회사 테마가 바탕의 정적 값·파생 식을 함께 물려받아야 함")
	}
}

func TestDerivedRules(t *testing.T) {
	// 정적 값 없는 파생 식은 거부
	if _, err := LoadThemes(themeFS(map[string]string{"x/theme.json": `{"derived":{"--a":"oklch(from var(--accent) l c h)"}}`})); err == nil {
		t.Error("정적 값 없는 derived 를 받음")
	}
	// 파생 값도 토큰처럼 검사
	if _, err := LoadThemes(themeFS(map[string]string{"x/theme.json": `{"tokens":{"--a":"red"},"derived":{"--a":"url(x)"}}`})); err == nil {
		t.Error("derived 의 url( 을 받음")
	}
	// 자식이 정적 값을 직접 주면 물려받은 식이 덮지 않는다
	ts, err := LoadThemes(themeFS(map[string]string{
		"p/theme.json": `{"tokens":{"--accent":"#123456","--soft":"#eeeeee"},"derived":{"--soft":"oklch(from var(--accent) 0.95 c h)"}}`,
		"c/theme.json": `{"extends":"p","tokens":{"--soft":"#ffeeee"}}`,
		"d/theme.json": `{"extends":"p","tokens":{"--accent":"#c8102e"}}`,
	}))
	if err != nil {
		t.Fatal(err)
	}
	c, _ := ts.Get("c")
	d, _ := ts.Get("d")
	if len(c.Derived) != 0 || len(d.Derived) != 1 {
		t.Errorf("물려받기: c %v d %v", c.Derived, d.Derived)
	}
}

// @scope 를 모르는 엔진: 사용자 테마는 까닭과 함께 쓸 수 없고 내장은 그대로.
func TestWithoutUserThemes(t *testing.T) {
	ts, err := LoadThemeSet(builtinFS(t), "../themes/samples")
	if err != nil {
		t.Fatal(err)
	}
	old := ts.withoutUser(noScopeReason)
	if _, ok := old.Get("acme"); ok {
		t.Error("옛 엔진에서 사용자 테마를 고를 수 있음")
	}
	if _, ok := old.Get("에이크미"); ok {
		t.Error("사용자 테마의 별칭이 남음")
	}
	if why, ok := old.Broken("acme"); !ok || !strings.Contains(why, "WebKit") {
		t.Errorf("까닭: %q", why)
	}
	for _, n := range []string{"classic", "bold", "mono", "compact", "spine", "클래식"} {
		if _, ok := old.Get(n); !ok {
			t.Errorf("내장 %s 가 빠짐", n)
		}
	}
	if _, ok := ts.Get("acme"); !ok {
		t.Error("원래 묶음이 바뀜")
	}
	// 문서가 고른 회사 테마를 쓸 수 없으면 경고에 까닭이 실린다
	v := RenderView([]byte("---\ntheme: acme\n---\n본문\n"), "", "/_doc/", old, "", false)
	if v.Theme != "classic" || len(v.Warnings) != 1 || !strings.Contains(v.Warnings[0].Message, "WebKit") {
		t.Errorf("경고: %s %+v", v.Theme, v.Warnings)
	}
	// 앱 쪽(SetScopeSupport·CreateTheme·옛 WebKit 흉내)은 App 이 있는 Itda Hyve 의 문서 창 시험이 본다.
}

// 회사 테마 만들기는 바탕의 파생 색을 새 강조색으로 계산해 정적 값으로 적는다 — 옛 엔진에서도 색이 섞이지 않는다.
func TestCreatedThemeHasStaticDerived(t *testing.T) {
	dir := t.TempDir()
	for _, base := range []string{"classic", "compact", "spine"} {
		ts, err := LoadThemeSet(builtinFS(t), dir)
		if err != nil {
			t.Fatal(err)
		}
		id, err := createUserTheme(dir, ts, NewTheme{Name: "c-" + base, Base: base, Accent: "#c8102e"}, "")
		if err != nil {
			t.Fatal(err)
		}
		ts, _ = LoadThemeSet(builtinFS(t), dir)
		th, ok := ts.Get(id)
		if !ok {
			t.Fatalf("%s 를 불러오지 못함", id)
		}
		vals := map[string]string{}
		for _, kv := range th.Tokens {
			vals[kv[0]] = kv[1]
		}
		bt, _ := ts.Get(base)
		for _, kv := range bt.Derived {
			want, _ := evalRelColor(kv[1], vals)
			left := false
			for _, d := range th.Derived {
				left = left || d[0] == kv[0]
			}
			switch {
			case strings.Contains(kv[1], "var(--accent)") && (left || vals[kv[0]] != want):
				t.Errorf("%s %s: 정적 값 %s (바란 것 %s), 식이 남음 %v", id, kv[0], vals[kv[0]], want, left)
			case !strings.Contains(kv[1], "var(--accent)") && !left:
				t.Errorf("%s %s: 강조색과 무관한 식은 물려받아야 함", id, kv[0])
			}
		}
	}
	if got, _ := evalRelColor("oklch(from var(--accent) 0.955 calc(c * 0.14) h)", map[string]string{"--accent": "#1d3a6b"}); got != "#ebf0f9" {
		t.Errorf("계산 %s", got)
	}
}

// 옛 WebKit 흉내 변수는 macOS 에서만 뜻이 있다(Windows 는 WebView2 — Chromium).
func TestOldWebKitEnvOnlyOnMac(t *testing.T) {
	if o := parseOldWebKit("darwin", "scope,relcolor"); !o.noScope || !o.noRelColor {
		t.Errorf("macOS: %+v", o)
	}
	for _, goos := range []string{"windows", "linux"} {
		if o := parseOldWebKit(goos, "all"); o.noScope || o.noRelColor {
			t.Errorf("%s 에서 흉내가 켜졌다: %+v", goos, o)
		}
	}
}
