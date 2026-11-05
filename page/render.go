package page

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/itda-work/itda-doc/mdoc"
)

// 원문 → View 조립. 파일 읽기·감시·그림 내주기는 앱 몫이라 여기 없다.

// RenderView 는 원문을 테마로 조립해 화면에 넘길 View 를 만든다. imgPrefix 는 상대 경로 그림 앞에 붙는 주소 머리다
// (mdoc.Options.LocalImagePrefix — 문서 창은 "/_doc/"·탭별 주소, 비면 상대 경로 그대로).
func RenderView(src []byte, path, imgPrefix string, ts *Themes, themeOverride string, allowRemote bool) View {
	return renderView(src, path, imgPrefix, ts, themeOverride, allowRemote)
}

// RenderViewOpts 는 RenderView 와 같되 렌더 옵션을 통째로 받는다(renderViewOpts).
func RenderViewOpts(src []byte, path string, ts *Themes, themeOverride string, opts mdoc.Options) View {
	return renderViewOpts(src, path, ts, themeOverride, opts)
}

// WarnUnknownTheme 은 문서의 theme: 를 쓸 수 없어 다른 테마로 보일 때의 경고 코드다. 렌더러(mdoc)가 아니라 테마를 고르는
// 여기서 더한다 — 입구 안내서의 경고 코드 표에 함께 있다(TestGuideThemes).
const WarnUnknownTheme = "unknown-theme"

// renderView 는 RenderView 와 같되 그림 주소 머리(imgPrefix — 창은 탭별 tabImagePrefix)를 정한다.
func renderView(src []byte, path, imgPrefix string, ts *Themes, themeOverride string, allowRemote bool) View {
	return renderViewOpts(src, path, ts, themeOverride, mdoc.Options{AllowRemoteImages: allowRemote, LocalImagePrefix: imgPrefix})
}

// renderViewOpts 는 renderView 와 같되 렌더 옵션을 통째로 받는다 — 공유 화면이 발표자 노트를 뺄 때(opts.DropNotes, share_http.go renderShare).
// path 가 있으면 문서 위 폴더의 `_metadata.yml` 을 읽어 frontmatter 기본값으로 넘긴다(itda-hyve#474 — 모든 렌더
// 입구가 이 함수를 지나므로 미리보기·발표·PDF·공유·고침 요청·md_check 가 같은 합친 값을 쓴다). 파일 읽기는
// 이쪽(page — 호스트 앱이 부르는 입구)에서, 합치기는 mdoc 이 한다. 호출자가 opts.FolderMeta 를 이미 쥐고 들어오면 그 값을 그대로
// 쓴다(폴더 설정 편집 화면 itda-hyve#474 ② 의 미리보기 — 디스크 대신 폼이 만들 내용으로 렌더한다).
func renderViewOpts(src []byte, path string, ts *Themes, themeOverride string, opts mdoc.Options) View {
	allowRemote := opts.AllowRemoteImages
	if path != "" && opts.FolderMeta == nil {
		opts.FolderMeta = findFolderMetas(path)
	}
	v := View{Path: path, FileName: filepath.Base(path), Themes: ts.List(), RemoteAllowed: allowRemote, Version: time.Now().UnixNano(),
		ThemeRev: ts.Rev}
	r, err := mdoc.Render(src, opts)
	if err != nil {
		v.Error = err.Error()
		return v
	}
	v.editorTokens = r.EditorTokens
	v.FolderMeta = r.FolderMeta
	t := PickTheme(ts, r.Meta.Theme, themeOverride)
	if _, ok := ts.Get(r.Meta.Theme); ok {
		dt, _ := ts.Get(r.Meta.Theme)
		v.DocTheme = dt.Name
		v.DocThemeFrom = docThemeFrom(src, r.FolderMeta, opts.FolderMeta)
	} else if why, broken := ts.Broken(r.Meta.Theme); broken {
		r.Warnings = append(r.Warnings, mdoc.Warning{Code: WarnUnknownTheme, Message: fmt.Sprintf("테마 %q 를 쓸 수 없음(%s) — %s 로 보인다", r.Meta.Theme, why, t.Name)})
	} else if r.Meta.Theme != "" {
		r.Warnings = append(r.Warnings, mdoc.Warning{Code: WarnUnknownTheme, Message: fmt.Sprintf("모르는 테마 %q — %s 로 보인다", r.Meta.Theme, t.Name)})
	}
	v.Theme = t.Name
	v.HTML, v.Header, v.Footer = Assemble(r, t)
	v.H1Break, v.H1Always = t.H1Break, t.H1Always
	v.ListStyle = listStyle(r.Meta, t)
	v.TOC = r.TOC
	v.Warnings = r.Warnings
	if v.Warnings == nil {
		v.Warnings = []mdoc.Warning{}
	}
	for _, w := range r.Warnings {
		if w.Code == mdoc.WarnRemoteImage {
			v.RemoteImages++
		}
	}
	v.Paper = r.Meta.PaperSize
	if v.Paper == "" {
		v.Paper = "a4"
	}
	v.Lang = r.Meta.Lang
	v.Slides = slideView(r)
	v.Board = boardView(r)
	v.Code = codeView(r)
	v.Logo = t.LogoURL
	v.Info = map[string]string{}
	for _, k := range []string{"title", "subtitle", "author", "date", "organization", "doc-no", "classification"} {
		if val, ok := themedValue(r.Meta, t, k); ok {
			v.Info[k] = val
		}
	}
	v.Title = r.Meta.Title
	if v.Title == "" {
		v.Title = strings.TrimSuffix(v.FileName, filepath.Ext(v.FileName))
	}
	return v
}

// docThemeFrom 은 합친 theme 값이 폴더 파일에서 왔으면 그 파일의 문서 기준 상대 경로다(itda-hyve#504). 적용된 폴더 파일이
// 없으면(applied 가 비면) 문서가 정한 것이라 "" 다. 출처는 렌더와 한 길(mdoc.ResolveFrontmatter — 같은 정리·합치기)로
// 본다 — 따로 계산하지 않는다(frontmatter 만 다시 푼다, 본문은 렌더하지 않는다).
func docThemeFrom(src []byte, applied []string, folder []mdoc.FolderMeta) string {
	if len(applied) == 0 {
		return ""
	}
	_, keys, _ := mdoc.ResolveFrontmatter(src, folder)
	return keys["theme"].FromRel
}

// PickTheme 은 화면에서 고른 테마 → 문서의 theme: → classic 순으로 고른다.
func PickTheme(ts *Themes, docTheme, override string) *Theme {
	for _, n := range []string{override, docTheme, "classic"} {
		if n == "" {
			continue
		}
		if t, ok := ts.Get(n); ok {
			return t
		}
	}
	for _, n := range ts.order {
		return ts.byName[n]
	}
	return &Theme{Name: "none"}
}

// ThemeStyleSheet 는 엔진을 가리지 않는 기본 꼴(@scope 로 가두고 상대 색은 @supports 로 덮는 것)의 테마 CSS 다 —
// StyleSheet(CSSMode{}) 와 같다. 패키지 밖(CLI)이 쓰는 짧은 입구다.
func (t *Theme) ThemeStyleSheet() string { return t.StyleSheet(CSSMode{}) }

// FindFolderMetas 는 문서(docPath)에 얹힐 폴더 기본값 파일(`_metadata.yml`)을 먼 폴더 → 가까운 폴더 순으로 돌려준다 —
// mdoc.Options.FolderMeta 에 그대로 넘긴다. RenderView·Check 가 쓰는 것과 같다(findFolderMetas). docPath 가 비면 nil.
func FindFolderMetas(docPath string) []mdoc.FolderMeta { return findFolderMetas(docPath) }

// ThemedValue 는 표지·머리글이 보이는 메타 값이다 — 문서 frontmatter 의 값, 없으면 테마 defaults 의 값(Assemble 과 같은 판정 —
// view.go themedValue). HTML 말고 다른 출력(docx 등)이 같은 표지를 만들 때 쓴다(itda-work/itda-hyve#608).
func ThemedValue(m mdoc.Meta, t *Theme, key string) (string, bool) { return themedValue(m, t, key) }

// MetaLabel 은 표지 메타 줄의 이름표다 — 테마 labels, 없으면 문서 lang(ko·그 밖)에 따른 기본 이름, 그것도 없으면 키 그대로
// (Assemble 과 같은 판정 — view.go label).
func MetaLabel(t *Theme, lang, key string) string { return label(t, lang, key) }
