//go:build !js

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/itda-work/itda-doc/mdoc"
	"github.com/itda-work/itda-doc/page"
)

// 브라우저용 입구와 네이티브의 동일성(itda-work/itda-hyve#577 D8): 이 패키지를 GOOS=js GOARCH=wasm 으로 한 번 빌드해 node 에서 올리고
// (testdata/run.cjs — temml.min.js 를 전역 스크립트로 먼저 올린다), 같은 원문·테마·옵션으로 네이티브 page.RenderViewOpts 가 낸
// View JSON 과 비교한다. 렌더 시각인 version 만 뺀다. 네이티브는 goja 로, wasm 은 JS Temml 로 수식을 그리므로 수식 경로(옵션·sanitize·
// 오류 고르기)가 두 판에서 같은지 여기서 묶인다. 테마 목록·테마 CSS 도 함께 본다.
//
// node 가 없으면 건너뛰고, ITDA_DOC_STRICT=1(just test·CI)이면 실패한다(web/pager_test.go 와 같다). 뮤테이션 시험은
// go test -overlay <json> 과 함께 ITDA_DOC_TEST_OVERLAY=<json> 을 주면 wasm 빌드에도 같은 overlay 를 넘긴다.

type renderCase struct {
	Name   string `json:"name"`
	Src    string `json:"src"`
	Opts   any    `json:"opts,omitempty"`   // 객체나 JSON 문자열(둘 다 받는지 본다)
	RawSrc any    `json:"rawSrc,omitempty"` // 있으면 src 대신 이 값을 첫 인자로(글자가 아닌 인자)
	Temml  string `json:"temml,omitempty"`  // 그 렌더 동안만 globalThis.temml 을 바꾼다(run.cjs temmlModes: none·notFunc·nonString·throw*)
	// 네이티브와 비교하지 않는 사례: wantErr 는 View.error 에 든 글, wantWarn 은 math 경고에 든 글이다(그때 HTML 에 <math 가 없어야 한다)
	wantErr  string
	wantWarn string
	// wantMath: 네이티브와 같고, HTML 에 <math 가 있어야 한다
	wantMath bool
	// internal: Temml 이 ParseError 가 아닌 JS 실행 오류(TypeError)를 던지는 입력이다. 그 문장은 JS 엔진이 지어 판마다 다르지만
	// (goja 「Cannot read property 'type' of undefined」, V8 「Cannot read properties of null (reading 'type')」) 경고는 두 판 모두
	// 정해진 한 문장이다(texmath internalMsg — itda-work/itda-hyve#577 결정 1). 가리지 않고 네이티브와 같게 보고, 그 문장이 든지도 본다.
	internal bool
	theme    string
	allow    bool
	drop     bool
}

type cssCase struct {
	Name string `json:"name"`
	Opts any    `json:"opts,omitempty"`
}

type wasmOut struct {
	Renders []struct {
		Name string  `json:"name"`
		View string  `json:"view"`
		MS   float64 `json:"ms"`
	} `json:"renders"`
	CSS     []*string         `json:"css"`
	Themes  string            `json:"themes"`
	Version map[string]string `json:"version"`
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func cases(t *testing.T) []renderCase {
	var cs []renderCase
	for _, f := range []string{"report", "charts", "diagram", "math", "gongmun"} {
		cs = append(cs, renderCase{Name: f, Src: readFile(t, filepath.Join("..", "..", "mdoc", "testdata", f+".md"))})
	}
	cs = append(cs, renderCase{Name: "charts-guide", Src: readFile(t, filepath.Join("..", "..", "spec", "samples", "charts-guide.md"))})
	report := cs[0].Src
	cs = append(cs,
		renderCase{Name: "report+mono(객체 옵션)", Src: report, Opts: map[string]any{"theme": "mono"}, theme: "mono"},
		renderCase{Name: "math+spine(JSON 옵션)", Src: cs[3].Src, Opts: `{"theme":"spine"}`, theme: "spine"},
		renderCase{Name: "원격 그림 허용", Src: "# 제목\n\n![그림](https://example.com/a.png)\n", Opts: map[string]any{"allowRemoteImages": true}, allow: true},
		renderCase{Name: "원격 그림 기본", Src: "# 제목\n\n![그림](https://example.com/a.png)\n"},
		renderCase{Name: "노트 빼기", Src: "---\nslides: true\n---\n\n# 하나\n\n본문 $a+b$\n\n:::notes\n노트 $c^2$\n:::\n", Opts: map[string]any{"dropNotes": true}, drop: true},
		renderCase{Name: "모르는 테마", Src: "---\ntheme: nope\n---\n\n본문\n"},
		// 수식 오류 사례 — 엔진(Temml)이 던지는 것과 앞(check.go)에서 막는 것
		renderCase{Name: "Temml 실행 오류(엔진 문장)", Src: "위첨자 없음 $x^$ 와 $\\operatorname{__proto__}$ 와 $y_$\n", internal: true},
		renderCase{Name: "수식 오류", Src: strings.Join([]string{
			`모르는 명령 $\foo{x}$`,
			`모르는 정의 $\undefinedthing$`,
			`문법 $\frac{a}$ 와 $\left( x$ 와 $a}$`,
			`환경 $\begin{foo}x\end{foo}$`,
			`블록 전용 $\begin{align}a&=b\end{align}$`,
			`재귀 $\def\a{\a}\a$`,
			`막힘 $\href{https://x.y}{z}$ $\url{https://x.y}$ $\color{red}{x}$ $\includegraphics{a.png}$ $\htmlClass{a}{b}$ $\tag{1}$`,
			`큰 길이 $\rule{100em}{1em}$ $\hspace{50em}$`,
			`누수 $\gdef\leakw{Q}\leakw$ 다음 $\leakw$`,
			"",
			`$$`,
			`\begin{aligned} a &= \sum_{i=1}^{n} \mathbb{R}_i \\ b &= \text{한글} \end{aligned}`,
			`$$`,
			"",
			"```math",
			`\begin{pmatrix} 1 & 2 \\ 3 & 4 \end{pmatrix}`,
			"```",
		}, "\n\n")},
	)
	return cs
}

func nativeView(t *testing.T, ts *page.Themes, c renderCase) string {
	t.Helper()
	v := page.RenderViewOpts([]byte(c.Src), "", ts, c.theme, mdoc.Options{AllowRemoteImages: c.allow, DropNotes: c.drop})
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// internalWarn 은 Temml 이 ParseError 가 아닌 것을 던졌을 때의 경고 글이다(texmath/temmlcommon.go internalMsg 와 같아야 한다 —
// 따로 적어 문장이 바뀌면 이 시험이 알린다).
const internalWarn = "수식을 그리지 않고 원문 그대로 둠: 수식 엔진 내부 오류(Temml 이 문법 오류가 아닌 예외를 냄)"

// normalize 는 View JSON 에서 version(렌더 시각)을 빼고 키 순서를 고정해 다시 쓴다.
func normalize(t *testing.T, s string) (string, map[string]any) {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("View JSON 이 아님: %v\n%.300s", err, s)
	}
	delete(m, "version")
	b, _ := json.Marshal(m)
	return string(b), m
}

// firstDiff 는 어긋난 맨 위 키들과 첫 차이 부근이다.
func firstDiff(a, b map[string]any) string {
	var keys []string
	for k := range a {
		keys = append(keys, k)
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var out []string
	for _, k := range keys {
		x, _ := json.Marshal(a[k])
		y, _ := json.Marshal(b[k])
		if bytes.Equal(x, y) {
			continue
		}
		i := 0
		for i < len(x) && i < len(y) && x[i] == y[i] {
			i++
		}
		lo := max(i-80, 0)
		out = append(out, fmt.Sprintf("%s:\n  네이티브 …%.240s\n  wasm     …%.240s", k, x[min(lo, len(x)):], y[min(lo, len(y)):]))
	}
	return strings.Join(out, "\n")
}

func goEnv(t *testing.T, gobin, key string) string {
	t.Helper()
	out, err := exec.Command(gobin, "env", key).Output()
	if err != nil {
		t.Fatalf("go env %s: %v", key, err)
	}
	return strings.TrimSpace(string(out))
}

func TestWasmMatchesNative(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" { // just test·CI
			t.Fatal("node 가 없어 wasm 동일성 검사를 할 수 없다 — ITDA_DOC_STRICT=1(just test·CI)은 node 가 필요하다")
		}
		t.Skip("node 가 없어 wasm 동일성 검사를 건너뜀")
	}
	if testing.Short() {
		t.Skip("-short: wasm 빌드를 건너뜀")
	}
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Fatal("go 명령을 찾지 못함")
	}
	dir := t.TempDir()
	wasm := filepath.Join(dir, "itda-doc.wasm")
	args := []string{"build", "-o", wasm}
	if ov := os.Getenv("ITDA_DOC_TEST_OVERLAY"); ov != "" {
		args = append(args, "-overlay", ov)
	}
	build := exec.Command(gobin, append(args, ".")...)
	build.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("wasm 빌드 실패: %v\n%s", err, out)
	}

	ts, err := page.BuiltinThemes()
	if err != nil {
		t.Fatal(err)
	}
	rcs := cases(t)
	var ccs []cssCase
	var cssWant []*string
	for _, ti := range ts.List() {
		th, _ := ts.Get(ti.Name)
		for _, un := range []bool{false, true} {
			ccs = append(ccs, cssCase{Name: ti.Name, Opts: map[string]any{"unscoped": un}})
			s := th.StyleSheet(page.CSSMode{Unscoped: un})
			cssWant = append(cssWant, &s)
		}
	}
	ccs = append(ccs, cssCase{Name: "nope"})
	cssWant = append(cssWant, nil)
	// 깨지는 입력: temml 을 올리지 않으면 수식은 math 경고(엔진을 올리지 못함)가 된다(네이티브와 다르다 — 따로 본다).
	// 바로 뒤에 temml 을 되돌리고 같은 원문을 다시 그리면 그려진다 — 엔진 오류는 기억(cache)하지 않는다(texmath.go Render).
	noTemml := "수식 $y^{17}+\\omega$\n"
	rcs = append(rcs,
		renderCase{Name: "temml 없음", Src: noTemml, Temml: "none", wantWarn: "globalThis.temml 이 없음"},
		renderCase{Name: "temml 뒤에 올림", Src: noTemml, wantMath: true},
		// 엔진 거부 경로(temml_js.go)
		renderCase{Name: "renderToString 이 함수가 아님", Src: "$q_{1}$\n", Temml: "notFunc", wantWarn: "temml.renderToString 이 함수가 아님"},
		renderCase{Name: "renderToString 이 글자를 돌려주지 않음", Src: "$q_{2}$\n", Temml: "nonString", wantWarn: "renderToString 이 글자를 돌려주지 않음"},
		// ParseError 가 아닌 것을 던지면 — Error 가 아닌 값(숫자·원형 없는 객체·글자)도, Temml 이 아닌 받침의 Error 도 — 한 문장
		// (CodeEngine)이고 기억하지 않아, Temml 을 되돌린 뒤 같은 원문이 그려진다(짝 「…뒤 복구」)
		renderCase{Name: "숫자를 던짐", Src: "$q_{4}$\n", Temml: "throwNumber", wantWarn: internalWarn},
		renderCase{Name: "숫자를 던진 뒤 복구", Src: "$q_{4}$\n", wantMath: true},
		renderCase{Name: "원형 없는 객체를 던짐", Src: "$q_{5}$\n", Temml: "throwBare", wantWarn: internalWarn},
		renderCase{Name: "원형 없는 객체를 던진 뒤 복구", Src: "$q_{5}$\n", wantMath: true},
		renderCase{Name: "글자를 던짐", Src: "$q_{3}$\n", Temml: "throwString", wantWarn: internalWarn},
		renderCase{Name: "글자를 던진 뒤 복구", Src: "$q_{3}$\n", wantMath: true},
		renderCase{Name: "Temml 아닌 Error", Src: "$q_{6}$\n", Temml: "throwError", wantWarn: internalWarn},
		renderCase{Name: "Temml 아닌 Error 뒤 복구", Src: "$q_{6}$\n", wantMath: true},
		// name 만 ParseError 인 객체는 Temml 의 것처럼 고른다(두 판이 name·message 만 본다)
		renderCase{Name: "ParseError 꼴", Src: "$q_{7}$\n", Temml: "throwParse", wantWarn: "지원하지 않는 명령 \\zzparse"},
		// render 인자·옵션 오류는 View.error
		renderCase{Name: "모르는 옵션 키", Src: "본문\n", Opts: map[string]any{"thme": "mono"}, wantErr: "옵션을 읽지 못함"},
		renderCase{Name: "깨진 JSON 옵션", Src: "본문\n", Opts: "{", wantErr: "옵션을 읽지 못함"},
		renderCase{Name: "JSON 뒤 글자", Src: "본문\n", Opts: `{"theme":"mono"} 쓰레기`, wantErr: "JSON 값 뒤에 글자가 남음"},
		renderCase{Name: "숫자 옵션", Src: "본문\n", Opts: 5, wantErr: "옵션은 객체나 JSON 문자열이다"},
		renderCase{Name: "글자가 아닌 원문", RawSrc: 1, wantErr: "첫 인자는 원문 문자열이다"},
	)
	// themeCSS 의 옵션 오류는 지금 null 이다(main.go — Go 콜백은 throw 를 못 한다)
	ccs = append(ccs, cssCase{Name: "classic", Opts: map[string]any{"bad": 1}})
	cssWant = append(cssWant, nil)

	in, _ := json.Marshal(map[string]any{"renders": rcs, "css": ccs})
	casesPath, outPath := filepath.Join(dir, "cases.json"), filepath.Join(dir, "out.json")
	if err := os.WriteFile(casesPath, in, 0o600); err != nil {
		t.Fatal(err)
	}
	wasmExec := filepath.Join(goEnv(t, gobin, "GOROOT"), "lib", "wasm", "wasm_exec.js")
	temmlJS, _ := filepath.Abs(filepath.Join("..", "..", "texmath", "temml", "temml.min.js"))
	runner, _ := filepath.Abs(filepath.Join("testdata", "run.cjs"))
	if out, err := exec.Command(node, runner, wasmExec, temmlJS, wasm, casesPath, outPath).CombinedOutput(); err != nil {
		t.Fatalf("node 실행 실패: %v\n%s", err, out)
	}
	var got wasmOut
	if err := json.Unmarshal([]byte(readFile(t, outPath)), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Renders) != len(rcs) || len(got.CSS) != len(ccs) {
		t.Fatalf("결과 수가 다름: 렌더 %d/%d, CSS %d/%d", len(got.Renders), len(rcs), len(got.CSS), len(ccs))
	}

	for i, c := range rcs {
		g := got.Renders[i]
		gs, gm := normalize(t, g.View)
		if c.wantErr != "" {
			if e := fmt.Sprint(gm["error"]); !strings.Contains(e, c.wantErr) {
				t.Errorf("%s: error 칸에 %q 가 없음: %q", c.Name, c.wantErr, e)
			}
			continue
		}
		if c.wantWarn != "" {
			ws, _ := gm["warnings"].([]any)
			found := false
			for _, w := range ws {
				if m, _ := w.(map[string]any); m["Code"] == mdoc.WarnMath && strings.Contains(fmt.Sprint(m["Message"]), c.wantWarn) {
					found = true
				}
			}
			if !found || strings.Contains(fmt.Sprint(gm["html"]), "<math") {
				t.Errorf("%s: math 경고 %q 가 없거나 수식이 그려짐: %v\n%.300s", c.Name, c.wantWarn, gm["warnings"], gm["html"])
			}
			continue
		}
		if c.wantMath && !strings.Contains(fmt.Sprint(gm["html"]), "<math") {
			t.Errorf("%s: wasm 결과에 <math 가 없음: %v", c.Name, gm["warnings"])
		}
		ns, nm := normalize(t, nativeView(t, ts, c))
		if c.internal {
			// 세 수식 모두 그 한 문장이어야 한다 — 다른 경고로 바뀌면 사례가 낡았다
			if n := strings.Count(gs, internalWarn); n != 3 {
				t.Errorf("%s: 내부 오류 문장이 %d 번(3 이어야 함)\n%.600s", c.Name, n, gm["warnings"])
			}
		}
		if gs != ns {
			t.Errorf("%s: wasm View 가 네이티브와 다름\n%s", c.Name, firstDiff(nm, gm))
		}
		t.Logf("%s: %d 바이트, wasm %.1f ms", c.Name, len(gs), g.MS)
	}
	// 수식 경로를 실제로 지났는지(둘 다 수식을 그리지 못해 같아지는 것을 막는다)
	if _, m := normalize(t, got.Renders[3].View); !strings.Contains(fmt.Sprint(m["html"]), "<math") {
		t.Error("math.md 의 wasm 결과에 <math> 가 없음")
	}
	themesWant, _ := json.Marshal(ts.List())
	if got.Themes != string(themesWant) {
		t.Errorf("테마 목록이 다름\n네이티브 %s\nwasm     %s", themesWant, got.Themes)
	}
	for i, c := range ccs {
		w, g := cssWant[i], got.CSS[i]
		if (w == nil) != (g == nil) || (w != nil && *w != *g) {
			t.Errorf("테마 CSS %s %v 가 다름", c.Name, c.Opts)
		}
	}
	if got.Version["go"] == "" {
		t.Errorf("version.go 가 비었음: %v", got.Version)
	}
}
