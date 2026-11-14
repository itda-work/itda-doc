//go:build js

package themes

import (
	"embed"
	"io/fs"
	"testing/fstest"
)

// files 는 테마의 theme.json·theme.css 만 담는다 — assets/ 는 넣지 않는다(itda-doc#48 대안 c). 웹 뷰어는 테마 CSS 를 blob: 문서로
// 걸어 /_theme/… 글꼴 주소가 풀리지 않으므로 넣어도 쓰지 못한다(itda-hyve docs/field-test/web-viewer-2026-11-05.md 실측).
// 네이티브 묶음과 파일이 어긋나면 TestJSBundleHasNoFonts(themes_test.go)가 잡는다.
//
//go:embed */theme.json */theme.css
var files embed.FS

// fontSlots 는 내장 테마 theme.css 의 @font-face 가 가리키는 자리다(editorial·hyrule). page 는 url(assets/…) 가 가리키는 파일이 FS 에 없으면 테마를
// 쓸 수 없다고 본다(page/themecss.go checkThemeCSS). 자리를 내용 없이 두어 테마 CSS 를 네이티브와 같은 바이트로 유지한다 —
// TestWasmMatchesNative 가 묶는다. 글꼴 이름을 바꾸거나 새 글꼴을 넣고 이 자리를 맞추지 않으면 같은 시험이 잡는다.
var fontSlots = []string{"editorial/assets/noto-serif-kr-bold.woff2", "hyrule/assets/noto-serif-kr-bold.woff2"}

// FS 는 js 판 내장 테마 묶음이다. 네이티브 판(themes.go)과 달리 글꼴이 없다 — fontSlots 는 0 바이트다.
// 그래서 이 판에서 Themes.Asset 은 그 자리를 찾지만 내용이 비어 있고, editorial·hyrule 의 Theme.Hash 는 네이티브와 다르다.
// fstest.MapFS 는 시험 전용이 아니라 디렉터리를 저절로 지어 주는 메모리 FS 로 쓴다(testing 패키지를 끌어오지 않는다).
// 타입을 fs.FS 로 좁히지 않는다 — 네이티브 판의 ReadFile·ReadDir 호출이 이 판에서도 컴파일되게 한다(TestFSPublicMethods).
var FS = bundle()

func bundle() fstest.MapFS {
	m := fstest.MapFS{}
	for _, p := range fontSlots {
		m[p] = &fstest.MapFile{}
	}
	err := fs.WalkDir(files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := files.ReadFile(p)
		if err == nil {
			m[p] = &fstest.MapFile{Data: b}
		}
		return err
	})
	if err != nil { // embed 묶음을 걷다 나는 오류 — 빌드가 고정한 파일이라 실행 환경에 달리지 않는다
		panic("themes: " + err.Error())
	}
	return m
}
