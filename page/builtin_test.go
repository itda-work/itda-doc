package page

import (
	"testing"
)

// Builtin 을 바꾸면 BuiltinThemes·ThemeSet·LoadCheckThemes·Check(ts=nil) 가 모두 그 묶음을 쓴다(앱이 자기 테마를 더하는 입구).
func TestBuiltinVar(t *testing.T) {
	saved := Builtin
	t.Cleanup(func() { Builtin = saved })
	Builtin = themeFS(map[string]string{"only/theme.json": `{"title": "하나"}`})
	check := func(what string, ts *Themes, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if l := ts.List(); len(l) != 1 || l[0].Name != "only" {
			t.Errorf("%s: %+v", what, l)
		}
	}
	ts, err := BuiltinThemes()
	check("BuiltinThemes", ts, err)
	ts, err = ThemeSet(t.TempDir())
	check("ThemeSet", ts, err)
	ts, err = LoadCheckThemes("")
	check("LoadCheckThemes", ts, err)
	ts, err = LoadCheckThemes(t.TempDir())
	check("LoadCheckThemes(dir)", ts, err)
	res, err := Check([]byte("# a\n"), "", nil)
	if err != nil || res.Meta.Theme != "only" {
		t.Errorf("Check(ts=nil): %v %+v", err, res.Meta)
	}
}
