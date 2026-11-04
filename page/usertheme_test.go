package page

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/itda-work/itda-doc/mdoc"
	"github.com/itda-work/itda-doc/themes"
)

func builtinFS(t *testing.T) fs.FS {
	t.Helper()
	sub, err := fs.Sub(themes.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	return sub
}

// writeTree 는 dir 아래에 파일들을 만든다(경로 → 내용).
func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for p, body := range files {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// 사용자 테마 폴더: 정상 테마는 내장과 함께 쓰이고, 잘못된 테마는 조용히 빠지지 않고 이유와 함께 목록에 남는다.
func TestUserThemeFolder(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"good/theme.json":      `{"title":"좋은 회사","extends":"classic","tokens":{"--accent":"#c8102e"},"logo":"assets/logo.svg","defaults":{"organization":"좋은회사"}}`,
		"good/theme.css":       `.cover-title { color: var(--accent); } .page-body .stamp { border: 1pt solid var(--accent); }`,
		"good/assets/logo.svg": `<svg xmlns="http://www.w3.org/2000/svg"/>`,
		"good/.DS_Store":       "finder", // 숨김 파일은 건너뛴다(Finder 가 만든다)
		"good/assets/.x.js":    "nope",
		"좋은회사/theme.json":      `{"extends":"good"}`, // 한글 이름, 사용자 테마끼리 물려받기
		"badjson/theme.json":   `{"title": `,
		"classic/theme.json":   `{}`,                  // 내장과 이름이 겹침
		"alias/theme.json":     `{"aliases":["클래식"]}`, // 내장의 별칭과 겹침
		"script/theme.json":    `{}`,
		"script/assets/a.js":   "alert(1)", // 허용하지 않는 확장자
		"remote/theme.json":    `{}`,
		"remote/theme.css":     `.page-body h1 { background: url(https://evil.example/p.png) }`,
		"orphan/theme.json":    `{"extends":"badjson"}`, // 쓸 수 없는 테마를 물려받음
		"Upper/theme.json":     `{}`,
		"empty/README.md":      "theme.json 없음",
		"chrome/theme.json":    `{}`,
		"chrome/theme.css":     `.bar { display: none } body { display: none }`,
		"uitoken/theme.json":   `{"tokens":{"--ui-bg":"red"}}`,
		"loose-file.md":        "맨 위 낱 파일은 테마가 아니다",
	})
	big := filepath.Join(dir, "big", "assets", "f.woff2")
	writeTree(t, dir, map[string]string{"big/theme.json": `{}`})
	_ = os.MkdirAll(filepath.Dir(big), 0o755)
	f, _ := os.Create(big)
	_ = f.Truncate(maxThemeFile + 1)
	f.Close()
	if runtime.GOOS != "windows" {
		writeTree(t, dir, map[string]string{"link/theme.json": `{}`, "secret.txt": "비밀"})
		if err := os.Symlink(filepath.Join(dir, "secret.txt"), filepath.Join(dir, "link", "assets.css")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(dir, "good"), filepath.Join(dir, "linkdir")); err != nil {
			t.Fatal(err)
		}
		writeTree(t, dir, map[string]string{"fifo/theme.json": `{}`})
		mkfifo(filepath.Join(dir, "fifo", "x.css"))
	}

	ts, err := LoadThemeSet(builtinFS(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	good, ok := ts.Get("good")
	if !ok || !good.User || good.Title != "좋은 회사" || good.Extends != "classic" {
		t.Fatalf("good: %+v %v", good, ok)
	}
	if !strings.HasPrefix(good.LogoURL, "/_theme/good/assets/logo.svg?v=") {
		t.Errorf("사용자 테마 그림 주소에 판이 없음: %q", good.LogoURL)
	}
	if _, _, ok := ts.Asset("/_theme/good/assets/.x.js"); ok {
		t.Error("숨김 파일을 내줌")
	}
	if fsys, p, ok := ts.Asset("/_theme/good/assets/logo.svg"); !ok {
		t.Error("사용자 테마 그림을 못 찾음")
	} else if b, _ := fs.ReadFile(fsys, p); !strings.Contains(string(b), "<svg") {
		t.Errorf("그림 내용 %q", b)
	}
	if th, ok := ts.Get("좋은회사"); !ok || th.Defaults["organization"] != "좋은회사" || th.LogoURL == "" {
		t.Errorf("한글 이름 테마가 good 을 물려받아야 함: %+v", th)
	}
	if th, _ := ts.Get("classic"); th.User {
		t.Error("이름이 겹치면 내장이 이겨야 함")
	}
	if th, _ := ts.Get("클래식"); th.Name != "classic" {
		t.Error("내장 별칭이 사용자 테마에 넘어감")
	}
	want := map[string]string{
		"badjson": "theme.json", "classic": "이름이 겹침", "alias": "별칭", "script": "둘 수 없는 파일", "remote": "url(",
		"orphan": "쓸 수 없음", "Upper": "폴더 이름", "empty": "theme.json", "big": "너무 큼", "chrome": "창 틀", "uitoken": "--ui-",
	}
	if runtime.GOOS != "windows" {
		want["link"], want["linkdir"], want["fifo"] = "링크", "링크", "보통 파일"
	}
	for name, frag := range want {
		why, ok := ts.Broken(name)
		if !ok || !strings.Contains(why, frag) {
			t.Errorf("%s: 쓸 수 없음(%q 포함)이어야 함 — 얻음 %q %v", name, frag, why, ok)
		}
		if _, ok := ts.Get(name); ok && name != "classic" {
			t.Errorf("%s: 쓸 수 없는 테마를 고를 수 있음", name)
		}
	}
	if _, ok := ts.Broken("loose-file.md"); ok {
		t.Error("맨 위 낱 파일을 테마로 봄")
	}
	// 목록: 내장 → 사용자 → 쓸 수 없음(이유와 함께)
	var order []string
	broken := 0
	for _, i := range ts.List() {
		order = append(order, i.Name)
		if i.Error != "" {
			broken++
			if !i.User {
				t.Errorf("쓸 수 없는 테마 %s 가 사용자 테마로 표시되지 않음", i.Name)
			}
		}
	}
	if !strings.HasPrefix(strings.Join(order, ","), "bold,classic,compact,mono,slate,spine,good,좋은회사,") || broken != len(want) {
		t.Errorf("목록 순서·수: %v (쓸 수 없음 %d)", order, broken)
	}
	// 폴더가 없으면 내장만(처음 쓰는 사람)
	ts2, err := LoadThemeSet(builtinFS(t), filepath.Join(dir, "none"))
	if err != nil || len(ts2.List()) != 6 {
		t.Errorf("없는 폴더: %v %d", err, len(ts2.List()))
	}
	// 메모리에 읽어 두었으므로 고른 뒤 디스크가 바뀌어도 내주는 내용은 그대로다(다시 읽을 때까지)
	_ = os.WriteFile(filepath.Join(dir, "good", "assets", "logo.svg"), []byte("바뀜"), 0o644)
	if fsys, p, _ := ts.Asset("/_theme/good/assets/logo.svg"); fsys != nil {
		if b, _ := fs.ReadFile(fsys, p); string(b) == "바뀜" {
			t.Error("내줄 때 디스크를 다시 읽음")
		}
	}
	if UserThemesStamp(dir) == UserThemesStamp(filepath.Join(dir, "none")) {
		t.Error("지문이 폴더를 구별하지 못함")
	}
}

// 폴더가 바뀌면 지문이 바뀐다(감시가 다시 읽는 근거).
func TestUserThemesStamp(t *testing.T) {
	dir := t.TempDir()
	s0 := UserThemesStamp(dir)
	writeTree(t, dir, map[string]string{"a/theme.json": `{}`})
	s1 := UserThemesStamp(dir)
	writeTree(t, dir, map[string]string{"a/theme.css": `.page-body h1 { color: red }`})
	s2 := UserThemesStamp(dir)
	if s0 == s1 || s1 == s2 {
		t.Errorf("지문이 바뀌지 않음: %s %s %s", s0, s1, s2)
	}
	// Windows(NTFS)는 폴더 목록에 적힌 하위 폴더(a)의 수정 시각을 그 안에 파일을 만든 뒤 조금 늦게 고친다 — 쓴 직후 잰
	// 지문이 한 번 더 바뀐다(Windows 11 실측, itda-hyve#61). 감시는 한 번 더 다시 읽을 뿐 바뀐 것을 놓치지 않는다.
	// 그래서 바뀌지 않는 폴더를 연달아 잰 두 지문이 (잠깐 뒤에는) 같은지를 본다.
	same := false
	for range 50 {
		if s := UserThemesStamp(dir); UserThemesStamp(dir) == s {
			same = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !same {
		t.Error("같은 폴더의 지문이 다름")
	}
}

// 창 틀 가두기: 테마 CSS 는 @scope (.page) 안에, @font-face 는 밖에. 가두기를 벗어나거나 창 틀을 노리는 것은 거부한다.
func TestThemeCSSConfinement(t *testing.T) {
	fsys := themeFS(map[string]string{
		"x/theme.json":     `{"tokens":{"--accent":"#123456"}}`,
		"x/assets/f.woff2": "woff2",
		"x/theme.css": `@font-face { font-family: "Acme Sans"; src: url(assets/f.woff2) format("woff2"); font-weight: 400; font-display: block; }
/* 주석 { } 은 무시 */
.page-body h1 { position: fixed; content: "a{b}"; }
@media print { .page-body .stamp { display: none } }
.page-body .box { color: red; &:first-child { color: blue } }`,
	})
	ts, err := LoadThemes(fsys)
	if err != nil {
		t.Fatal(err)
	}
	th, _ := ts.Get("x")
	css := th.StyleSheet(CSSMode{})
	scope := strings.Index(css, "@scope (.page) {")
	font := strings.Index(css, "@font-face")
	if scope < 0 || font < 0 || font > scope || !strings.HasSuffix(strings.TrimSpace(css), "}") {
		t.Fatalf("모양: @font-face 가 @scope 앞(밖)에 있어야 함\n%s", css)
	}
	inner := css[scope:]
	for _, want := range []string{`.page-body h1 { position: fixed; content: "a{b}"; }`, "@media print {", ".page-body .stamp { display: none }",
		"&:first-child { color: blue }"} {
		if !strings.Contains(inner, want) {
			t.Errorf("@scope 안에 %q 가 없음\n%s", want, inner)
		}
	}
	if strings.Contains(inner, "@font-face") || strings.Contains(css, "주석") {
		t.Errorf("@font-face 가 @scope 안에 있거나 주석이 남음\n%s", css)
	}
	if !strings.Contains(css[:scope], `src: url("/_theme/x/assets/f.woff2") format("woff2")`) || !strings.Contains(css[:scope], ":root {\n  --accent: #123456;") {
		t.Errorf("글꼴 주소·토큰\n%s", css[:scope])
	}
	// 괄호 짝: @scope 블록이 맨 끝에서 닫힌다
	if strings.Count(inner, "{") != strings.Count(inner, "}") {
		t.Errorf("괄호 짝이 맞지 않음\n%s", inner)
	}

	rejects := map[string]string{
		"body":          `body { display: none }`,
		"html":          `html .page-body { color: red }`,
		":root":         `:root { --accent: red }`,
		":scope":        `:scope { position: fixed }`,
		"맨 위 &":         `& .x { color: red }`,
		"@page":         `@page { margin: 0 }`,
		"@layer":        `@layer x { .a { color: red } }`,
		"@keyframes":    `@keyframes k { from { opacity: 0 } }`,
		"@property":     `@property --x { syntax: "*"; inherits: false; }`,
		"@namespace":    `@namespace svg url(http://www.w3.org/2000/svg);`,
		"image-set":     `.a { background: image-set("https://evil.example/a.png" 1x) }`,
		"src()":         `.a { background: src("https://evil.example/a.png") }`,
		"@import":       `@import url(assets/a.css);`,
		"ttf 글꼴":        `@font-face { font-family: A; src: url(assets/f.ttf) }`,
		"Pretendard 덮기": `@font-face { font-family: "Pretendard"; src: url(assets/f.woff2) }`,
		"창 틀 글꼴":        `@font-face { font-family: -apple-system; src: url(assets/f.woff2) }`,
		"원격 글꼴":         `@font-face { font-family: A; src: url(https://evil.example/f.woff2) }`,
		"@media 안 글꼴":   `@media print { @font-face { font-family: A; src: url(assets/f.woff2) } }`,
		"@media 안 body": `@media print { body { display: none } }`,
		"글꼴 서술자":        `@font-face { font-family: A; src: url(assets/f.woff2); unknown-thing: 1 }`,
		"닫히지 않음":        `.a { color: red`,
		"짝 없는 }":        `.a { color: red } }`,
		"선택자만":          `.a`,
		"behavior":      `.a { behavior: url(assets/f.woff2) }`,
		"글자열 안 탈출 아님":   `.a { content: "</style>" }`,
	}
	for name, css := range rejects {
		fsys := themeFS(map[string]string{"x/theme.json": `{}`, "x/theme.css": css, "x/assets/f.woff2": "w", "x/assets/f.ttf": "t", "x/assets/a.css": "a"})
		if _, err := LoadThemes(fsys); err == nil {
			t.Errorf("%s: 거부해야 함", name)
		}
	}
	// 창 틀이 쓰는 토큰 이름(ShellCSS 에서 모은 것)은 하나도 받지 않는다
	for name := range reservedTokens() {
		if err := checkToken(name, "red"); err == nil {
			t.Errorf("창 틀 토큰 %s 를 받음", name)
		}
	}
	for _, n := range []string{"--wails-draggable", "--page-w", "--img-max-h", "--body-h"} {
		if checkToken(n, "x") == nil {
			t.Errorf("%s 를 받음", n)
		}
	}
}

// 문서 층 글꼴 보호: base.css 의 --font-* 글꼴 이름과 CSS 일반 이름은 ShellCSS(창 틀)와 무관하게 막는다 — 사용자 테마의
// @font-face 가 같은 이름으로 기본 글꼴을 몰래 바꾸지 못한다. 막는 이름 목록을 여기서 고정한다(처음 이 렌더러를 쓴 호스트 앱은 창 틀 CSS 를 거쳐
// Apple SD Gothic Neo·Malgun Gothic·Menlo 를 막았다 — 그 위에 SF Mono·Consolas·STIX Two Math·Cambria Math 가 더 막힌다).
func TestDocFontFamiliesReserved(t *testing.T) {
	want := []string{"apple sd gothic neo", "cambria math", "consolas", "cursive", "emoji", "fangsong", "fantasy", "malgun gothic",
		"math", "menlo", "monospace", "pretendard", "sans-serif", "serif", "sf mono", "stix two math", "system-ui", "ui-monospace",
		"ui-rounded", "ui-sans-serif", "ui-serif"}
	if ShellCSS() == nil {
		got := slices.Sorted(maps.Keys(reservedFontFamilies()))
		if !slices.Equal(got, want) {
			t.Errorf("막는 글꼴 이름\n받음 %q\n기대 %q", got, want)
		}
	}
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"malgun/theme.json":     `{}`,
		"malgun/theme.css":      `@font-face { font-family: "Malgun Gothic"; src: url(assets/x.woff2) format("woff2"); }`,
		"malgun/assets/x.woff2": "w",
	})
	ts, err := LoadThemeSet(builtinFS(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	if why, ok := ts.Broken("malgun"); !ok || !strings.Contains(strings.ToLower(why), "malgun gothic") {
		t.Errorf("Malgun Gothic 을 덮는 사용자 테마가 쓸 수 없음이 아님: %q %v", why, ok)
	}
}

// 내장 테마의 CSS 도 같은 길(@scope)을 탄다.
func TestBuiltinThemesAreScoped(t *testing.T) {
	builtinProblems(t, checkScope)
}

// 회사 테마 만들기: 이름 검사, 덮어쓰기 거부, 사용자 테마 폴더 밖에 쓰지 못함, 만든 테마가 로더를 통과.
func TestCreateUserTheme(t *testing.T) {
	for in, want := range map[string]string{"ACME 보고서": "acme-보고서", " 잇다워크 ": "잇다워크", "a_b-c": "a_b-c", "Acme  Co": "acme-co", "-x-": "x"} {
		if got, err := themeIDFromName(in); err != nil || got != want {
			t.Errorf("themeIDFromName(%q) = %q, %v — 바란 것 %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "   ", "../x", "a/b", "a.b", "a:b", `a\b`, "ㅎㅎ", "---", strings.Repeat("a", 41), "x\x00"} {
		if id, err := themeIDFromName(bad); err == nil {
			t.Errorf("themeIDFromName(%q) 를 받음 → %q", bad, id)
		}
	}

	parent := t.TempDir()
	dir := filepath.Join(parent, "themes") // 아직 없다 — 만들 때 생긴다
	ts, err := LoadThemeSet(builtinFS(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	logo := filepath.Join(parent, "logo.svg")
	_ = os.WriteFile(logo, []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0o644)
	req := NewTheme{Name: "ACME 보고서", Base: "classic", Accent: "#C8102E", Organization: "에이크미 주식회사", Logo: true}
	id, err := CreateUserTheme(dir, ts, req, logo)
	if err != nil || id != "acme-보고서" {
		t.Fatalf("만들기: %q %v", id, err)
	}
	ts, err = LoadThemeSet(builtinFS(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	th, ok := ts.Get(id)
	if !ok {
		why, _ := ts.Broken(id)
		t.Fatalf("만든 테마가 로더를 통과하지 못함: %s", why)
	}
	if th.Title != "ACME 보고서" || th.Extends != "classic" || th.Defaults["organization"] != "에이크미 주식회사" || th.LogoURL == "" {
		t.Errorf("만든 테마: %+v", th)
	}
	if !strings.Contains(th.StyleSheet(CSSMode{}), "--accent: #c8102e;") {
		t.Error("강조색 토큰이 없음")
	}
	// 조직 이름은 문서에 organization 이 없을 때 표지·머리글에 쓰인다(글자로)
	r, _ := mdoc.Render([]byte("---\ntitle: 시험\ncover: true\n---\n본문\n"), mdoc.Options{})
	html, header, _ := Assemble(r, th)
	if !strings.Contains(html, `<div class="cover-org">에이크미 주식회사</div>`) || !strings.Contains(html, `class="cover-logo"`) ||
		len(header[0]) == 0 || header[0][0].Text != "에이크미 주식회사" {
		t.Errorf("조직 이름·로고가 표지·머리글에 없음: %s %+v", html[:200], header)
	}
	// 같은 이름 다시 → 덮어쓰지 않는다
	before, _ := os.ReadFile(filepath.Join(dir, id, "theme.json"))
	if _, err := CreateUserTheme(dir, ts, NewTheme{Name: "acme 보고서", Base: "mono", Accent: "#000000"}, ""); err == nil {
		t.Error("있는 테마를 덮어씀")
	}
	if after, _ := os.ReadFile(filepath.Join(dir, id, "theme.json")); string(after) != string(before) {
		t.Error("theme.json 이 바뀜")
	}
	// 디스크에는 있지만 아직 읽지 않은 폴더도 덮어쓰지 않는다(경쟁)
	_ = os.MkdirAll(filepath.Join(dir, "race"), 0o755)
	if _, err := CreateUserTheme(dir, ts, NewTheme{Name: "race", Base: "classic", Accent: "#000000"}, ""); err == nil || !strings.Contains(err.Error(), "덮어쓰지") {
		t.Errorf("있는 폴더: %v", err)
	}
	cases := map[string]NewTheme{
		"내장 이름":   {Name: "classic", Base: "classic", Accent: "#000000"},
		"내장 별칭":   {Name: "클래식", Base: "classic", Accent: "#000000"},
		"바탕 없음":   {Name: "a1", Base: "nope", Accent: "#000000"},
		"바탕 사용자":  {Name: "a2", Base: id, Accent: "#000000"},
		"색 형식":    {Name: "a3", Base: "classic", Accent: "red"},
		"색 탈출":    {Name: "a4", Base: "classic", Accent: "#000000; } body {"},
		"조직 줄바꿈":  {Name: "a5", Base: "classic", Accent: "#000000", Organization: "a\nb"},
		"조직 자리표시": {Name: "a6", Base: "classic", Accent: "#000000", Organization: "{title}"},
		"로고 안 고름": {Name: "a7", Base: "classic", Accent: "#000000", Logo: true},
		"폴더 탈출":   {Name: "../escape", Base: "classic", Accent: "#000000"},
	}
	for name, c := range cases {
		if _, err := CreateUserTheme(dir, ts, c, ""); err == nil {
			t.Errorf("%s: 거부해야 함", name)
		}
	}
	if _, err := CreateUserTheme("", ts, NewTheme{Name: "z", Base: "classic", Accent: "#000000"}, ""); err == nil {
		t.Error("폴더 없이 만듦")
	}
	// 쓴 곳은 사용자 테마 폴더 안뿐이다
	var outside []string
	_ = filepath.WalkDir(parent, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !strings.HasPrefix(p, dir+string(filepath.Separator)) && p != logo {
			outside = append(outside, p)
		}
		return nil
	})
	if len(outside) > 0 {
		t.Errorf("테마 폴더 밖에 쓴 파일: %v", outside)
	}
	// 로고: 그림이 아닌 것·링크·너무 큰 것은 받지 않는다
	txt := filepath.Join(parent, "a.txt")
	_ = os.WriteFile(txt, []byte("x"), 0o644)
	if _, _, err := ReadLogo(txt); err == nil {
		t.Error("그림이 아닌 로고를 받음")
	}
	huge := filepath.Join(parent, "huge.png")
	hf, _ := os.Create(huge)
	_ = hf.Truncate(maxLogo + 1)
	hf.Close()
	if _, _, err := ReadLogo(huge); err == nil {
		t.Error("너무 큰 로고를 받음")
	}
	if runtime.GOOS != "windows" {
		ln := filepath.Join(parent, "ln.svg")
		_ = os.Symlink(logo, ln)
		if _, _, err := ReadLogo(ln); err == nil {
			t.Error("링크인 로고를 받음")
		}
	}
}

// 문서와 함께 둔 견본 회사 테마(themes/samples/acme)가 로더를 통과한다 — 안내서의 예가 실제로 돈다.
func TestSampleCompanyTheme(t *testing.T) {
	ts, err := LoadThemeSet(builtinFS(t), filepath.Join("..", "themes", "samples"))
	if err != nil {
		t.Fatal(err)
	}
	acmeInMenu := false
	for _, i := range ts.List() {
		if i.Error != "" {
			t.Errorf("견본 %s: %s", i.Name, i.Error)
		}
		if i.Name == "acme" {
			acmeInMenu = i.HasTemplate // 회사 테마의 서식은 「새 문서」 메뉴에 보인다(내장 중립 테마에는 없다 — itda-doc#28)
		}
	}
	if !acmeInMenu {
		t.Error("견본 acme 의 template.md 가 새 문서 메뉴(HasTemplate)에 없음")
	}
	th, ok := ts.Get("acme")
	if !ok {
		t.Fatal("견본 acme 가 없음")
	}
	if th.Extends != "classic" || th.LogoURL == "" || th.Defaults["organization"] == "" || th.Template == "" || th.Fonts != "" {
		t.Errorf("견본 acme: %+v", th)
	}
	css := th.StyleSheet(CSSMode{})
	if i := strings.Index(css, "@scope (.page) {"); i < 0 || !strings.Contains(css[i:], ".confidential {") {
		t.Errorf("자기 어휘 confidential 이 @scope 안에 없음\n%s", css)
	}
	if t2, _ := ts.Get("에이크미"); t2 != th {
		t.Error("별칭 에이크미")
	}
	v := RenderView([]byte(th.Template), "", "/_doc/", ts, "", false)
	if len(v.Warnings) != 0 || v.DocTheme != "acme" {
		t.Errorf("견본 서식: %+v %s", v.Warnings, v.DocTheme)
	}
}
