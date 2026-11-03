package page

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// 인수 조건 1: 내장 중립 다섯은 묶음 basic 이고 경고가 없다.
func TestBuiltinThemeGroups(t *testing.T) {
	ts := builtin(t)
	for _, n := range []string{"classic", "bold", "compact", "mono", "spine"} {
		th, ok := ts.Get(n)
		if !ok {
			t.Fatalf("내장 테마 %s 가 없음", n)
		}
		if th.Group != "basic" || th.GroupWarning() != "" {
			t.Errorf("%s: Group %q, 경고 %q — basic 이고 경고가 없어야 함", n, th.Group, th.GroupWarning())
		}
	}
}

// 인수 조건 2·3: group 은 extends 로 물려받지 않고, 어휘 밖 값은 거부하지 않고 "" + 경고다.
func TestThemeGroupLoad(t *testing.T) {
	ts := mustLoad(t, themeFS(map[string]string{
		"base/theme.json":   `{"group":"basic"}`,
		"child/theme.json":  `{"extends":"base"}`,
		"own/theme.json":    `{"extends":"base","group":"vivid"}`,
		"fun/theme.json":    `{"extends":"base","group":"fun"}`,
		"funkid/theme.json": `{"extends":"fun"}`,
		"empty/theme.json":  `{"extends":"base","group":""}`,
		"caps/theme.json":   `{"extends":"base","group":"Vivid"}`,
		"space/theme.json":  `{"extends":"base","group":" vivid"}`,
	}))
	// 값 → 바란 Group, 경고가 있어야 하는지. 대소문자·공백은 너그럽게 맞추지 않는다(어휘 밖 → 경고), 빈 값은 묶음 없음·경고 없음.
	for name, want := range map[string]struct {
		group string
		warn  bool
	}{
		"base": {"basic", false}, "child": {"", false}, "own": {"vivid", false}, "fun": {"", true}, "funkid": {"", false},
		"empty": {"", false}, "caps": {"", true}, "space": {"", true},
	} {
		th, ok := ts.Get(name)
		if !ok {
			t.Fatalf("%s 를 쓸 수 없음", name)
		}
		if th.Group != want.group {
			t.Errorf("%s: Group %q, 바란 값 %q", name, th.Group, want.group)
		}
		if w := th.GroupWarning(); want.warn != (w != "") {
			t.Errorf("%s: 경고 %q", name, w)
		}
	}
	if w, _ := ts.Get("fun"); !strings.Contains(w.GroupWarning(), `"fun"`) {
		t.Errorf("경고에 받은 값이 없음: %q", w.GroupWarning())
	}

	// 사용자 테마: 어휘 밖 값이어도 목록에서 고를 수 있다(쓸 수 없음이 아니다)
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"fun/theme.json": `{"extends":"classic","group":"fun"}`,
		"kid/theme.json": `{"extends":"classic"}`, // 내장 classic 의 basic 을 물려받지 않는다
	})
	us, err := LoadThemeSet(builtinFS(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	if th, ok := us.Get("fun"); !ok || th.Group != "" || th.GroupWarning() == "" {
		t.Errorf("사용자 테마 fun: %v %+v", ok, th)
	}
	if th, ok := us.Get("kid"); !ok || th.Group != "" || th.GroupWarning() != "" {
		t.Errorf("사용자 테마 kid(extends classic, group 없음): %v %+v", ok, th)
	}
	if why, broken := us.Broken("fun"); broken {
		t.Errorf("사용자 테마 fun 이 쓸 수 없음이 됨: %s", why)
	}
}

// ThemeGroups 는 고쳐도 어휘가 바뀌지 않는 새 슬라이스다.
func TestThemeGroupsCopy(t *testing.T) {
	g := ThemeGroups()
	g[0].Name = "x"
	if ThemeGroups()[0].Name != "basic" {
		t.Error("ThemeGroups 결과를 고치자 어휘가 바뀜")
	}
}

// 인수 조건 4: 안내서(spec/themes.md 2장)의 묶음 표는 코드의 어휘와 값·한글 이름·차례까지 같다.
func TestThemeGroupTableMatchesSpec(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "spec", "themes.md"))
	if err != nil {
		t.Fatal(err)
	}
	g := string(b)
	i, j := strings.Index(g, "<!-- theme-groups:begin -->"), strings.Index(g, "<!-- theme-groups:end -->")
	if i < 0 || j < i {
		t.Fatal("안내서에 묶음 표 표지가 없음")
	}
	row := regexp.MustCompile("^\\| `([a-z]+)` \\| ([^|]+?) \\|")
	var doc []ThemeGroup
	for line := range strings.SplitSeq(g[i:j], "\n") {
		if m := row.FindStringSubmatch(line); m != nil {
			doc = append(doc, ThemeGroup{Name: m[1], Title: m[2]})
		}
	}
	if !slices.Equal(doc, ThemeGroups()) {
		t.Errorf("묶음 표가 코드와 다름\n안내서 %v\n코드   %v", doc, ThemeGroups())
	}
}

// 테마 목록(ThemeInfo, View.Themes JSON)에도 묶음이 실린다 — 내장 classic 은 basic, 어휘 밖 값의 사용자 테마는 비우고
// JSON 에서 group 키가 빠진다(omitempty).
func TestThemeInfoGroup(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"fun/theme.json": `{"extends":"classic","group":"fun"}`})
	ts, err := LoadThemeSet(builtinFS(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ThemeInfo{}
	for _, i := range ts.List() {
		got[i.Name] = i
	}
	if g := got["classic"].Group; g != "basic" {
		t.Errorf("classic 의 ThemeInfo.Group = %q", g)
	}
	fun, ok := got["fun"]
	if !ok || fun.Error != "" || fun.Group != "" {
		t.Fatalf("사용자 테마 fun: %v %+v", ok, fun)
	}
	b, _ := json.Marshal(fun)
	if strings.Contains(string(b), `"group"`) {
		t.Errorf("빈 묶음인데 JSON 에 group 이 있음: %s", b)
	}
	if b, _ := json.Marshal(got["classic"]); !strings.Contains(string(b), `"group":"basic"`) {
		t.Errorf("classic JSON 에 group 이 없음: %s", b)
	}
}
