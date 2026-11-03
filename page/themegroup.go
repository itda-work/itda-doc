package page

import "strings"

// ThemeGroup 은 테마의 성격 묶음 하나다(theme.json 의 group, itda-doc#24). 묶음 기준은 쓰임새가 아니라 성격·무드다.
type ThemeGroup struct {
	Name  string `json:"name"`  // theme.json 에 적는 값
	Title string `json:"title"` // 테마 고르기에 보일 한글 이름
}

// themeGroups 는 어휘이자 표시 순서다. spec/themes.md 2장의 묶음 표와 같다(TestThemeGroupTableMatchesSpec).
var themeGroups = []ThemeGroup{
	{"basic", "기본"},
	{"formal", "단정·격식"},
	{"vivid", "선명·브랜드"},
	{"editorial", "감성·에디토리얼"},
	{"dark", "다크"},
}

// ThemeGroups 는 성격 묶음의 어휘를 표시 순서대로 돌려준다(새 슬라이스 — 고쳐도 어휘는 바뀌지 않는다).
// 앱은 이것으로 테마 고르기를 묶고 한글 이름을 보인다. 묶음이 없는 테마(Theme.Group == "")는 앱이 따로 둔다.
func ThemeGroups() []ThemeGroup {
	return append([]ThemeGroup(nil), themeGroups...)
}

func validThemeGroup(name string) bool {
	for _, g := range themeGroups {
		if g.Name == name {
			return true
		}
	}
	return false
}

func themeGroupNames() string {
	var names []string
	for _, g := range themeGroups {
		names = append(names, g.Name)
	}
	return strings.Join(names, "·")
}

// GroupWarning 은 theme.json 의 group 이 어휘 밖일 때의 경고 문장이다(없으면 ""). 그 테마는 쓸 수 있고 Group 은 "" 다.
// 내장 테마에서는 BuiltinThemeProblems 가 문제로 잡는다.
func (t *Theme) GroupWarning() string { return t.groupWarn }
