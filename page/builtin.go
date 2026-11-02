package page

import (
	"io/fs"

	"github.com/itda-work/itda-doc/themes"
)

// Builtin 은 내장 테마 묶음이다(맨 위 폴더마다 테마 하나). 기본은 중립 다섯(themes.FS). BuiltinThemes·ThemeSet·
// LoadCheckThemes·Check(ts=nil) 가 모두 이 변수를 읽는다. 앱은 자기 테마를 더한 묶음(합집합 FS)을 여기 넣을 수 있다 —
// 첫 호출 전에 정한다(ShellCSS 와 같은 꼴).
var Builtin fs.FS = themes.FS

// BuiltinThemes 는 바이너리에 든 테마다.
func BuiltinThemes() (*Themes, error) {
	return LoadThemes(Builtin)
}

// ThemeSet 은 내장 테마와 사용자 테마 폴더(dir)를 함께 불러온다. md --theme-meta 가 이 결과를 내준다(#367).
func ThemeSet(dir string) (*Themes, error) {
	return loadThemeSet(dir)
}

// loadThemeSet 은 내장 테마와 사용자 테마 폴더(dir)를 함께 불러온다.
func loadThemeSet(dir string) (*Themes, error) {
	return LoadThemeSet(Builtin, dir)
}
