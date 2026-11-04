//go:build js && wasm

// itda-doc-wasm 은 브라우저용 렌더 입구다(itda-work/itda-hyve#577 2단계). GOOS=js GOARCH=wasm 으로 만들고(scripts/build-wasm.sh —
// just wasm) 페이지가 wasm_exec.js·temml.min.js 와 함께 올린다. 전역 itdaDoc 에 다음을 둔다:
//
//	itdaDoc.render(src, opts)       → View JSON 문자열. 네이티브 page.RenderViewOpts(src, "", 내장 테마, opts.theme,
//	                                  mdoc.Options{AllowRemoteImages, DropNotes, FolderMeta}) 와 같다
//	                                  (version — 렌더 시각 — 만 다르다. 동일성 시험: wasm_test.go)
//	itdaDoc.themes()                → 테마 목록 JSON 문자열(page.Themes.List — View.themes 와 같다)
//	itdaDoc.themeCSS(name, opts)    → 그 테마의 CSS 문자열(page.Theme.StyleSheet — 호스트 앱이 /_theme/<이름>/theme.css 로 내는 것).
//	                                  없는 이름이면 null. 옵션을 읽지 못해도 지금은 null 이다 — Go 콜백은 JS 예외를 던질 수 없다
//	                                  (syscall/js 에 그 수단이 없고, wasm_exec.js 의 함수 감싸기는 결과만 돌려준다)
//	itdaDoc.version                 → {commit, go}. commit 은 빌드가 -ldflags -X 로 넣은 itda-doc 커밋(없으면 "")
//
// opts 는 JS 객체나 JSON 문자열이고 생략할 수 있다. render: {theme, allowRemoteImages, dropNotes, folderMeta}, themeCSS: {unscoped}.
// folderMeta 는 [{rel, yaml, err}] — 폴더 `_metadata.yml` 을 먼 폴더 → 가까운 폴더 순으로(mdoc.FolderMeta 의 Rel·YAML·Err). 비거나
// 없으면 폴더 기본값 없이 그린다. 파일을 찾아 읽는 것은 부르는 쪽(웹 뷰어 viewer.js — 사용자가 연 폴더)이다.
// 모르는 키와 JSON 값 뒤에 남은 글자는 오류다(오타가 조용히 무시되지 않게). 키의 대소문자는 가리지 않는다(encoding/json —
// "THEME" 도 theme 이다). 같은 키가 두 번 있으면 뒤 값이 이긴다. render 의 오류는 View 의 error 칸으로 돌려준다.
//
// 경로는 늘 빈 값으로 넘긴다 — wasm 쪽의 파일 접근(폴더 기본값 찾기 등)은 일어나지 않는다(page/render.go renderViewOpts 는 path 가
// 비면 찾지 않는다). 폴더 기본값은 opts.folderMeta 로 받은 내용만 쓴다.
// 테마는 내장 다섯뿐이다(page.BuiltinThemes). 수식은 페이지가 올린 globalThis.temml 로 그린다(texmath/temml_js.go) —
// 올리지 않으면 수식이 원문 + math 경고(「수식 엔진을 올리지 못함」)로 남는다. globalThis.temml 과 그 renderToString 은
// 평범한 값이어야 한다 — 읽기(getter·Proxy)에서 던지면 syscall/js 의 Get 이 잡지 않아 render 가 JS 예외를 던진다
// (wasm_exec.js valueGet, texmath/temml_js.go). 수식 하나의 시간 상한은 이 판에 없다(texmath/temml_js.go).
//
// 함수가 Go panic 하면 받아서 오류로 돌려준다(JS 예외는 위 경우처럼 받지 못한다) — 콜백 밖으로 나가면 Go 프로그램이 끝나 새로 고칠 때까지 아무것도 못 그린다.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"
	"syscall/js"

	"github.com/itda-work/itda-doc/mdoc"
	"github.com/itda-work/itda-doc/page"
)

// commit 은 빌드가 넣는다: -ldflags "-X main.commit=<커밋>"(scripts/build-wasm.sh).
var commit string

var (
	themesOnce sync.Once
	themes     *page.Themes
	themesErr  error
)

func builtin() (*page.Themes, error) {
	themesOnce.Do(func() { themes, themesErr = page.BuiltinThemes() })
	return themes, themesErr
}

type renderOpts struct {
	Theme             string `json:"theme"`
	AllowRemoteImages bool   `json:"allowRemoteImages"`
	DropNotes         bool   `json:"dropNotes"`
	// FolderMeta 는 mdoc.FolderMeta 의 JSON 꼴이다(YAML 을 글자로 받는다 — []byte 는 encoding/json 이 base64 로 읽는다).
	FolderMeta []folderMetaOpt `json:"folderMeta"`
}

type folderMetaOpt struct {
	Rel  string `json:"rel"`
	YAML string `json:"yaml"`
	Err  string `json:"err"`
}

// mdocOptions 는 render 옵션을 mdoc.Options 로 옮긴다. folderMeta 가 비면 nil 이다(폴더 기본값 없음 — 네이티브의 경로 없는 렌더와 같다).
func (o renderOpts) mdocOptions() mdoc.Options {
	opts := mdoc.Options{AllowRemoteImages: o.AllowRemoteImages, DropNotes: o.DropNotes}
	for _, f := range o.FolderMeta {
		opts.FolderMeta = append(opts.FolderMeta, mdoc.FolderMeta{Rel: f.Rel, YAML: []byte(f.YAML), Err: f.Err})
	}
	return opts
}

type cssOpts struct {
	Unscoped bool `json:"unscoped"` // @scope 를 모르는 엔진(옛 WebKit): 내장 테마를 가두지 않고 낸다(page.CSSMode)
}

// readOpts 는 인자 i 를 dst 로 읽는다. 없음·undefined·null 이면 그대로 둔다.
func readOpts(a []js.Value, i int, dst any) error {
	if len(a) <= i || a[i].IsUndefined() || a[i].IsNull() {
		return nil
	}
	var raw string
	switch a[i].Type() {
	case js.TypeString:
		raw = a[i].String()
	case js.TypeObject:
		raw = js.Global().Get("JSON").Call("stringify", a[i]).String()
	default:
		return fmt.Errorf("옵션은 객체나 JSON 문자열이다(받은 것: %s)", a[i].Type())
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("옵션을 읽지 못함: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("옵션을 읽지 못함: JSON 값 뒤에 글자가 남음")
	}
	return nil
}

func errorView(msg string) string {
	b, _ := json.Marshal(page.View{Error: msg, Warnings: []mdoc.Warning{}})
	return string(b)
}

func render(_ js.Value, a []js.Value) (out any) {
	defer func() {
		if r := recover(); r != nil {
			out = errorView(fmt.Sprint("렌더하지 못함: ", r))
		}
	}()
	if len(a) < 1 || a[0].Type() != js.TypeString {
		return errorView("첫 인자는 원문 문자열이다")
	}
	var o renderOpts
	if err := readOpts(a, 1, &o); err != nil {
		return errorView(err.Error())
	}
	ts, err := builtin()
	if err != nil {
		return errorView("테마를 불러오지 못함: " + err.Error())
	}
	v := page.RenderViewOpts([]byte(a[0].String()), "", ts, o.Theme, o.mdocOptions())
	b, err := json.Marshal(v)
	if err != nil {
		return errorView(err.Error())
	}
	return string(b)
}

func themeList(_ js.Value, _ []js.Value) (out any) {
	defer func() {
		if r := recover(); r != nil {
			out = "[]"
		}
	}()
	ts, err := builtin()
	if err != nil {
		return "[]"
	}
	b, _ := json.Marshal(ts.List())
	return string(b)
}

func themeCSS(_ js.Value, a []js.Value) (out any) {
	defer func() {
		if r := recover(); r != nil {
			out = js.Null()
		}
	}()
	if len(a) < 1 || a[0].Type() != js.TypeString {
		return js.Null()
	}
	var o cssOpts
	if err := readOpts(a, 1, &o); err != nil {
		return js.Null()
	}
	ts, err := builtin()
	if err != nil {
		return js.Null()
	}
	t, ok := ts.Get(a[0].String())
	if !ok {
		return js.Null()
	}
	return t.StyleSheet(page.CSSMode{Unscoped: o.Unscoped})
}

func main() {
	api := js.Global().Get("Object").New()
	api.Set("render", js.FuncOf(render))
	api.Set("themes", js.FuncOf(themeList))
	api.Set("themeCSS", js.FuncOf(themeCSS))
	api.Set("version", map[string]any{"commit": commit, "go": runtime.Version()})
	js.Global().Set("itdaDoc", api)
	select {}
}
