package page

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// 입구 안내서(spec/markdown.md)의 테마 표는 내장 테마의 이름·별칭과 같고, 경고 코드 표에는 이 패키지가 더하는
// unknown-theme 이 있다(#134). 테마를 더하거나 별칭을 바꾸고 안내서를 안 고치면 실패한다.
func TestGuideThemes(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "spec", "markdown.md"))
	if err != nil {
		t.Fatal(err)
	}
	g := string(b)
	section := func(name string) string {
		i, j := strings.Index(g, "<!-- "+name+":begin -->"), strings.Index(g, "<!-- "+name+":end -->")
		if i < 0 || j < i {
			t.Fatalf("안내서에 %s 표지가 없음", name)
		}
		return g[i:j]
	}
	if !strings.Contains(section("warn-codes"), "| `"+WarnUnknownTheme+"` |") {
		t.Errorf("경고 코드 표에 %s 가 없음", WarnUnknownTheme)
	}
	ts, err := BuiltinThemes()
	if err != nil {
		t.Fatal(err)
	}
	row := regexp.MustCompile("(?m)^\\| `([a-z0-9-]+)` \\| ([^|]+) \\|")
	doc := map[string][]string{}
	for _, m := range row.FindAllStringSubmatch(section("themes"), -1) {
		var aliases []string
		for a := range strings.SplitSeq(m[2], "·") {
			aliases = append(aliases, strings.TrimSpace(a))
		}
		doc[m[1]] = aliases
	}
	if len(doc) != len(ts.List()) {
		t.Errorf("안내서 테마 %d 개, 내장 테마 %d 개", len(doc), len(ts.List()))
	}
	for _, info := range ts.List() {
		th, _ := ts.Get(info.Name)
		got, ok := doc[info.Name]
		if !ok {
			t.Errorf("안내서 테마 표에 %s 가 없음", info.Name)
			continue
		}
		want := slices.Clone(th.Aliases)
		slices.Sort(want)
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("%s 의 별칭: 안내서 %v, theme.json %v", info.Name, got, want)
		}
	}
	if pickTheme(ts, "", "").Name != "classic" || !strings.Contains(g, "없으면 `classic` 다") {
		t.Error("기본 테마가 classic 가 아니거나 안내서에 적힌 것과 다름")
	}
}
