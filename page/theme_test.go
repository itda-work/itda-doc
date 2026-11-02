package page

import (
	"fmt"
	"github.com/itda-work/itda-doc/themes"
	"github.com/itda-work/itda-doc/web"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func TestBuiltinThemes(t *testing.T) {
	ts, err := BuiltinThemes()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, i := range ts.List() {
		names = append(names, i.Name)
	}
	// 표준 다섯이 있어야 한다. 다른 조직이 themes/ 에 자기 테마를 더해 빌드해도(안내서 11장) 이 검사는 깨지지 않는다.
	if !containsAll(names, "spine", "compact", "mono", "bold", "classic") {
		t.Fatalf("내장 테마 = %s", strings.Join(names, ","))
	}
	if th, _ := ts.Get("컴팩트"); th == nil || th.Name != "compact" || !th.NoCover {
		t.Errorf("compact: 별칭 컴팩트·표지 없음이어야 함: %+v", th)
	}
	if th, _ := ts.Get("spine"); !th.H1Break || !th.H1Always || th.NoCover || th.Header[0] != "{title}" {
		t.Errorf("spine: 장마다 새 쪽(always)·표지·머리글에 문서 제목이어야 함: %+v", th)
	}
	if th, ok := ts.Get("모노"); !ok || th.Name != "mono" {
		t.Fatalf("모노 별칭이 mono 가 아님: %v", th)
	}
	if th, _ := ts.Get("mono"); !th.NoCover || th.Footer[0] != "{doc-no}" {
		t.Errorf("mono: 표지 없음·바닥글 문서번호여야 함: %+v", th)
	}
	// 모든 내장 테마는 쪽번호를 "{page} / {pages}" 로 어딘가에 낸다(검증 스크립트가 표지를 뺀 모든 쪽에서 찾는다)
	for _, i := range ts.List() {
		th, _ := ts.Get(i.Name)
		all := strings.Join(th.Header[:], "|") + strings.Join(th.Footer[:], "|")
		if !strings.Contains(all, "{page} / {pages}") {
			t.Errorf("%s: 머리글·바닥글에 {page} / {pages} 가 없음", i.Name)
		}
		if css := th.StyleSheet(cssMode{}); !strings.Contains(css, "--page-margin-top:") {
			t.Errorf("%s: 여백 토큰이 없음", i.Name)
		}
	}
}

// 테마는 모양일 뿐 문서의 목적을 정하지 않는다 — 같은 테마가 제안서·강의·사내 공지 어디에 쓰일지 모른다.
// 그래서 내장 테마는 문서 종류를 나타내는 장식 글자(자기 이름·제목·별칭, 「PROPOSAL」·「회의록」 같은 종류 낱말)를
// CSS 글자(content)나 머리글·바닥글의 고정 글자로 쪽·슬라이드에 찍지 않는다(#549). 구조 글자(「제 n 장」·「결과」)와
// meta 이름표는 이 검사의 대상이 아니다.
// themeContentStr 는 CSS content 의 문자열 값(큰·작은따옴표, 콜론 앞뒤 공백)을 잡는다.
var themeContentStr = regexp.MustCompile(`content\s*:\s*((?:(?:"[^"]*"|'[^']*')\s*)+)`)

func TestThemeContentStrForms(t *testing.T) {
	for _, in := range []string{`content: "PROPOSAL"`, `content:'PROPOSAL'`, `content : "PRO" "POSAL"`} {
		m := themeContentStr.FindStringSubmatch(in)
		if m == nil || !strings.Contains(contentText(m[1]), "PROPOSAL") {
			t.Errorf("content 문자열을 못 잡음: %s → %v", in, m)
		}
	}
}

// contentText 는 content 값의 문자열들을 CSS 처럼 이어 붙인다("PRO" "POSAL" → PROPOSAL).
func contentText(v string) string {
	var b strings.Builder
	for _, m := range regexp.MustCompile(`"([^"]*)"|'([^']*)'`).FindAllStringSubmatch(v, -1) {
		b.WriteString(m[1] + m[2])
	}
	return b.String()
}

func TestBuiltinThemesNoPurposeText(t *testing.T) {
	ts := builtin(t)
	comment := regexp.MustCompile(`(?s)/\*.*?\*/`)
	contentStr := themeContentStr
	placeholder := regexp.MustCompile(`\{[^}]*\}`)
	for _, i := range ts.List() {
		th, _ := ts.Get(i.Name)
		// 자기 이름·제목·별칭에 더해 문서 종류 낱말(옛 테마 이름 포함, #549)도 금지어로 본다
		words := append([]string{th.Name, th.Title}, th.Aliases...)
		words = append(words, "report", "proposal", "official", "minutes", "manual",
			"보고서", "제안서", "공문", "기안", "회의록", "매뉴얼", "절차서")
		var texts []string
		for _, m := range contentStr.FindAllStringSubmatch(comment.ReplaceAllString(th.CSS, ""), -1) {
			texts = append(texts, contentText(m[1]))
		}
		for _, slot := range append(th.Header[:], th.Footer[:]...) {
			texts = append(texts, placeholder.ReplaceAllString(slot, ""))
		}
		for _, txt := range texts {
			low := strings.ToLower(txt)
			for _, w := range words {
				if w != "" && strings.Contains(low, strings.ToLower(w)) {
					t.Errorf("%s: 테마 이름·별칭 %q 이 고정 글자 %q 에 있음", i.Name, w, strings.TrimSpace(txt))
				}
			}
		}
	}
}

// 기본 내장 다섯은 모양 이름(classic·bold·mono·compact·spine)과 한글 제목 별칭만 받는다. 옛 목적 이름과 그 별칭은
// 받지 않는다(#549 — 숨은 별칭으로 남기지 않기로 함). 옛 이름을 적은 문서는 같은 이름의 사용자 테마가 없으면 모르는 테마처럼 기본 테마로 열린다.
func TestBuiltinThemesNoLegacyNames(t *testing.T) {
	ts := builtin(t)
	legacy := []string{"report", "proposal", "official", "minutes", "manual", "gongmun", "default",
		"보고서", "제안서", "공문", "기안", "기안문", "회의록", "minute", "meeting", "매뉴얼", "절차서", "업무매뉴얼", "guide"}
	for _, n := range legacy {
		if th, ok := ts.Get(n); ok {
			t.Errorf("옛 이름 %q 이 내장 테마 %s 로 찾힘", n, th.Name)
		}
	}
	for name, alias := range map[string]string{"classic": "클래식", "bold": "볼드", "mono": "모노", "compact": "컴팩트", "spine": "책등"} {
		th, ok := ts.Get(name)
		if !ok || th.Title != alias || !slices.Equal(th.Aliases, []string{alias}) {
			t.Errorf("%s: 제목·별칭이 %q 하나여야 함: %+v", name, alias, th)
		}
	}
}

func themeFS(files map[string]string) fstest.MapFS {
	m := fstest.MapFS{}
	for k, v := range files {
		m[k] = &fstest.MapFile{Data: []byte(v)}
	}
	return m
}

func TestThemeExtends(t *testing.T) {
	fsys := themeFS(map[string]string{
		"base/theme.json": `{"title":"기본","tokens":{"--accent":"#111111","--ink":"#222"},"margin":{"top":"20mm"},
			"footer":{"center":"{page} / {pages}","right":"{doc-no}"},"meta":["doc-no","date"]}`,
		"base/theme.css":         `.cover { background: url("assets/bg.png"); }`,
		"base/assets/bg.png":     "png",
		"acme/theme.json":        `{"extends":"base","tokens":{"--accent":"#c8102e"},"logo":"assets/logo.svg","footer":{"right":""}}`,
		"acme/theme.css":         `.cover-title { color: var(--accent); }`,
		"acme/assets/logo.svg":   "<svg/>",
		"acme/assets/.hidden.js": "nope",
	})
	ts, err := LoadThemes(fsys)
	if err != nil {
		t.Fatal(err)
	}
	acme, _ := ts.Get("acme")
	css := acme.StyleSheet(cssMode{})
	for _, want := range []string{"--accent: #c8102e;", "--ink: #222;", "--page-margin-top: 20mm;",
		`url("/_theme/base/assets/bg.png")`, ".cover-title { color: var(--accent); }"} {
		if !strings.Contains(css, want) {
			t.Errorf("acme CSS 에 %q 없음:\n%s", want, css)
		}
	}
	// 물려받은 CSS 가 먼저, 자기 CSS 가 뒤(덮어쓰기)
	if strings.Index(css, "---- base") > strings.Index(css, "---- acme") {
		t.Error("부모 CSS 가 자식보다 뒤에 있음")
	}
	if acme.Footer[1] != "{page} / {pages}" || acme.Footer[2] != "" {
		t.Errorf("바닥글 물려받기/비우기: %q", acme.Footer)
	}
	if acme.LogoURL != "/_theme/acme/assets/logo.svg" {
		t.Errorf("logo = %q", acme.LogoURL)
	}
	if _, _, ok := ts.Asset("/_theme/acme/assets/.hidden.js"); ok {
		t.Error("숨김 파일을 내줌")
	}
	if _, _, ok := ts.Asset("/_theme/base/assets/bg.png"); !ok {
		t.Error("물려받은 테마의 파일을 못 찾음")
	}
	if strings.Join(acme.Meta, ",") != "doc-no,date" {
		t.Errorf("meta 물려받기: %v", acme.Meta)
	}
}

func TestThemeRejects(t *testing.T) {
	cases := map[string]map[string]string{
		"원격 url":     {"x/theme.json": `{}`, "x/theme.css": `body{background:url(https://evil.example/p.png)}`},
		"@import":    {"x/theme.json": `{}`, "x/theme.css": `@import "https://evil.example/a.css";`},
		"폴더 탈출 url":  {"x/theme.json": `{}`, "x/theme.css": `body{background:url(../y/assets/a.png)}`},
		"없는 파일 url":  {"x/theme.json": `{}`, "x/theme.css": `body{background:url(assets/none.png)}`},
		"토큰 값 탈출":    {"x/theme.json": `{"tokens":{"--accent":"red; } body { display:none"}}`},
		"토큰 url":     {"x/theme.json": `{"tokens":{"--bg":"url(https://evil.example/p.png)"}}`},
		"토큰 이름":      {"x/theme.json": `{"tokens":{"color":"red"}}`},
		"여백 형식":      {"x/theme.json": `{"margin":{"top":"calc(1px+2px)"}}`},
		"모르는 키":      {"x/theme.json": `{"foter":{}}`},
		"없는 부모":      {"x/theme.json": `{"extends":"nope"}`},
		"돌고 도는 상속":   {"x/theme.json": `{"extends":"y"}`, "y/theme.json": `{"extends":"x"}`},
		"자기 상속":      {"x/theme.json": `{"extends":"x"}`},
		"이름 불일치":     {"x/theme.json": `{"name":"y"}`},
		"폴더 이름":      {"X Y/theme.json": `{}`},
		"logo 밖":     {"x/theme.json": `{"logo":"../logo.svg"}`},
		"logo 없음":    {"x/theme.json": `{"logo":"assets/logo.svg"}`},
		"cover 값":    {"x/theme.json": `{"cover":"sometimes"}`},
		"theme.json": {"x/theme.css": `body{}`},
		"expression": {"x/theme.json": `{}`, "x/theme.css": `body{width:expression(alert(1))}`},
	}
	for name, files := range cases {
		if _, err := LoadThemes(themeFS(files)); err == nil {
			t.Errorf("%s: 거부해야 함", name)
		}
	}
}

// 문서 CSS 는 OpenType 숫자 기능(tnum 등)을 쓰지 않는다 — WKWebView PDF 에서 그 숫자 글리프가 유니코드 대응 없이
// 들어가 표의 숫자를 복사·검색할 수 없었다(scripts/docview_check.py 가 잡은 결함).
func TestNoNumericFontFeatures(t *testing.T) {
	ts := builtin(t)
	base, err := web.FS.ReadFile("base.css")
	if err != nil {
		t.Fatal(err)
	}
	sheets := map[string]string{"base.css": string(base)}
	for _, i := range ts.List() {
		th, _ := ts.Get(i.Name)
		sheets[i.Name] = th.StyleSheet(cssMode{})
	}
	comment := regexp.MustCompile(`(?s)/\*.*?\*/`)
	for name, css := range sheets {
		css = comment.ReplaceAllString(css, "")
		for _, bad := range []string{"tabular-nums", "oldstyle-nums", "lining-nums", "font-feature-settings"} {
			if strings.Contains(css, bad) {
				t.Errorf("%s 에 %s", name, bad)
			}
		}
	}
}

func containsAll(have []string, want ...string) bool {
	set := map[string]bool{}
	for _, h := range have {
		set[h] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}

// 안내서(spec/themes.md)의 토큰 표는 base.css 의 :root 와 같아야 한다 — 이름·기본값 모두.
// base.css 가 var() 로 쓰는 토큰은 모두 :root 에 기본값이 있다(규칙 안 지역 변수와 화면이 재어 거는 값은 빼고 — 아래 local).
func TestTokenTableMatchesBaseCSS(t *testing.T) {
	base, err := web.FS.ReadFile("base.css")
	if err != nil {
		t.Fatal(err)
	}
	css := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(base), "")
	root := regexp.MustCompile(`(?s):root\s*\{(.*?)\n\}`).FindStringSubmatch(css)
	if root == nil {
		t.Fatal("base.css 에 :root 가 없음")
	}
	code := map[string]string{}
	for _, m := range regexp.MustCompile(`(--[a-z0-9-]+)\s*:\s*([^;]+);`).FindAllStringSubmatch(root[1], -1) {
		code[m[1]] = strings.TrimSpace(m[2])
	}
	for _, m := range regexp.MustCompile(`var\((--[a-z0-9-]+)`).FindAllStringSubmatch(css, -1) {
		local := map[string]bool{"--c": true, "--d": true, "--panel-w": true, "--panel-gap": true, "--img-max-h": true, "--slide-head-h": true, "--body-h": true}
		if _, ok := code[m[1]]; !ok && !local[m[1]] {
			// --c·--d·--panel-w·--panel-gap 은 규칙 안의 지역 변수(알림 색·차트 항목 차례·분할 그림 패널 폭과 틈 #550),
			// --img-max-h·--slide-head-h·--body-h 는 화면(쪽 나눔기·슬라이드 나눔기)이 재어 거는 값
			t.Errorf("base.css 가 쓰는 %s 에 기본값이 없음", m[1])
		}
	}
	guide, err := os.ReadFile(filepath.Join("..", "spec", "themes.md"))
	if err != nil {
		t.Fatal(err)
	}
	g := string(guide)
	i, j := strings.Index(g, "<!-- tokens:begin -->"), strings.Index(g, "<!-- tokens:end -->")
	if i < 0 || j < i {
		t.Fatal("안내서에 토큰 표 표지가 없음")
	}
	doc := map[string]string{}
	row := regexp.MustCompile("^\\| `(--[a-z0-9-]+)` \\| [^|]+ \\| `(.*)` \\|$")
	for line := range strings.SplitSeq(g[i:j], "\n") {
		if m := row.FindStringSubmatch(line); m != nil {
			doc[m[1]] = m[2]
		}
	}
	for k, v := range code {
		if dv, ok := doc[k]; !ok {
			t.Errorf("안내서 표에 %s 가 없음", k)
		} else if dv != v {
			t.Errorf("%s 기본값: 안내서 %q ≠ base.css %q", k, dv, v)
		}
	}
	for k := range doc {
		if _, ok := code[k]; !ok {
			t.Errorf("안내서 표의 %s 가 base.css 에 없음", k)
		}
	}
	if len(doc) < 60 {
		t.Errorf("표의 토큰이 %d 개뿐", len(doc))
	}
}

// ---- 슬라이드 글자 배율 가드(#430 2단계, 리뷰 1~3회차) ----
// base.css 와 내장 테마의 슬라이드 전용 요소(표지 .sc-*·구역 .ss-*·슬라이드 머리/바닥 .slide-*)는
// font-size 가 --slide-scale 배율을 받아야 한다. 테마의 고정 pt 선언이 calc 를 덮쳐 누락이 1·2회차 리뷰에
// 연속 나왔으므로(grep 은 여러 줄 선언을 놓쳤다) 규칙 단위로 읽어 막는다. 3회차부터는 해석을 줄이고
// **허용 꼴만 통과**시킨다(거짓 통과를 없애고 거짓 실패를 감수 — 대상은 우리가 관리하는 내장 CSS 다).
// 쪽 문서와 함께 쓰는 본문 서식(:::result·절차 번호·결재란·서명 — 선택자에 슬라이드 전용 표식이 없다)는
// 배율 약속 밖이다.

// slideScaleFixedSel — 배율 검사에서 빼는 관리 표시. 조각 전체가 정확히 일치해야 예외다(부분 일치로 두면
// :not(.sc-class) 같은 반례가 새어 들어간다 — 리뷰 3회차 [3]). 지금 내장 CSS 가 실제로 고르는 선택자:
//
//	.sc-class   표지 분류 표시(대외비 등) — 문서 관리 표시
//	.slide-foot 슬라이드 바닥글 — 조직명·쪽번호(관리 표시)
//	.sf-num     바닥글 쪽번호
//	.sf-left    바닥글 왼쪽 칸(조직명·로고)
//	.sf-logo    바닥글 로고
var slideScaleFixedSel = []string{".sc-class", ".slide-foot", ".sf-num", ".sf-left", ".sf-logo"}

// slideSelMark — 슬라이드에만 있는 요소 표식. 이 조각의 font-size 는 배율과 곱해져야 한다.
var slideSelMark = []string{".sc-", ".ss-", ".slide-", ".page.slide"}

// nestAtRuleName — 안으로 들어가는 @묶음 이름(atName 이 뽑은 것을 소문자로). 안에 규칙을 담는 @묶음 중
// 이 목록에 없는 이름은 파싱 실패로 본다(조용히 건너뛰지 않는다 — 리뷰 4회차 [1]). 리프 @규칙
// (@font-face·@counter-style 등)는 이 목록과 무관하게 선언 블록으로 검사한다.
var nestAtRuleName = []string{"media", "supports", "scope", "layer", "container", "keyframes"}

// slideCSSRules 는 리프 규칙(선언 블록)만 남긴다. 주석은 문자열을 보존하는 stripCSSComments 로 지우고
// themecss splitCSSBlocks 로 가른다(content:"{" 같은 문자열은 splitCSSBlocks 가 이미 건너뛴다).
// @묶음은 atName 이 뽑은 이름(소문자)으로 식별한다 — " @media\nprint"·"@MEDIA"·"@supports(…" 같은
// 표기 변형을 ASCII 공백 자르기로는 못 얻는다(리뷰 4회차 [1]). 이 검사는 내장 테마·base.css 에서 흔히
// 쓰는 꼴의 회귀를 잡는다(완전한 CSS 해석기가 아니다 — 거짓 실패를 감수한 허용 목록).
func slideCSSRules(css string) ([]cssBlock, error) {
	return slideCSSRulesInner(stripCSSComments(css))
}

func slideCSSRulesInner(css string) ([]cssBlock, error) {
	bs, err := splitCSSBlocks(css)
	if err != nil {
		return nil, err
	}
	var out []cssBlock
	for _, b := range bs {
		if b.statement {
			continue
		}
		pre := strings.TrimSpace(b.prelude)
		if strings.HasPrefix(pre, "@") {
			name := strings.ToLower(atName(pre))
			if slices.Contains(nestAtRuleName, name) {
				inner, err := slideCSSRulesInner(b.body)
				if err != nil {
					return nil, err
				}
				out = append(out, inner...)
				continue
			}
			if hasRuleBlocks(b.body) { // 안에 규칙 블록이 있는데 지원하지 않는 묶음 — 실패
				return nil, fmt.Errorf("지원하지 않는 @묶음 @%s", name)
			}
		}
		out = append(out, b)
	}
	return out, nil
}

// hasRuleBlocks 는 css 안에 리프가 아닌 규칙 블록이 하나라도 있는지(@font-face 같은 리프 선언과 갈라).
func hasRuleBlocks(css string) bool {
	bs, err := splitCSSBlocks(css)
	if err != nil {
		return true // 못 가르는 것도 규칙이 있는 것으로 본다 — 조용히 통과시키지 않는다
	}
	for _, b := range bs {
		if !b.statement {
			return true
		}
	}
	return false
}

// slideSelectorPieces 는 선택자 목록을 괄호 바깥(깊이 0)의 쉼표로만 나눈다 — :is(a, b)·[a=b, c] 안의
// 쉼표는 조각을 가르지 않는다(리뷰 3회차 [3]).
func slideSelectorPieces(sel string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(sel); i++ {
		switch sel[i] {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(sel[start:i]))
				start = i + 1
			}
		}
	}
	out = append(out, strings.TrimSpace(sel[start:]))
	return out
}

func hasMark(s string, marks []string) bool {
	for _, m := range marks {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

// slideGuardChecked 는 선택자 조각이 검사 대상인지 — 슬라이드 전용 표식이 있고 관리 표시 예외(정확
// 일치)가 아니다.
func slideGuardChecked(piece string) bool {
	return hasMark(piece, slideSelMark) && !slices.Contains(slideScaleFixedSel, piece)
}

// slideFontSizeOK 는 font-size 값이 배율 허용 꼴인지. !important 떼고 공백을 없애 정규화한 뒤,
// 이 꼴만 통과한다(rem·var 단독·덧셈 calc 등 알 수 없는 꼴은 전부 실패 — 리뷰 3회차 [1]):
//
//	calc(<수>pt * var(--slide-scale))                        calc(104pt * var(--slide-scale))
//	calc(var(--slide-*-size) * <수…> * var(--slide-scale))   calc(var(--slide-title-size) * 0.86 * var(--slide-scale))
//	<수>em · <수>% · inherit
//
// calc 의 곱셈 인수는 길이(pt 수·size 토큰) 정확히 하나, var(--slide-scale) 정확히 하나, 나머지는
// 무차원 수만 — 길이가 없으면(단위 없는 font-size)·길이가 둘이면·배율이 중복되면(제곱) 실패(리뷰 4회차 [2]).
// 사용자 정의 속성은 대소문자를 가린다.
const fsNum = `-?(?:\d+(?:\.\d+)?|\.\d+)`

var fsSimple = regexp.MustCompile(`^(?:` + fsNum + `em|` + fsNum + `%|inherit)$`)
var fsPt = regexp.MustCompile(`^` + fsNum + `pt$`)
var fsBare = regexp.MustCompile(`^` + fsNum + `$`)
var fsSizeTok = regexp.MustCompile(`^var\(--slide-[a-z0-9-]+-size\)$`)
var fsCalc = regexp.MustCompile(`^calc\((.+)\)$`)

func slideFontSizeOK(v string) bool {
	v = strings.TrimSuffix(strings.Join(strings.Fields(v), ""), "!important")
	if fsSimple.MatchString(v) {
		return true
	}
	m := fsCalc.FindStringSubmatch(v)
	if m == nil {
		return false
	}
	length, scale := 0, 0
	for term := range strings.SplitSeq(m[1], "*") {
		switch {
		case term == "var(--slide-scale)":
			scale++
		case fsPt.MatchString(term), fsSizeTok.MatchString(term):
			length++
		case fsBare.MatchString(term):
			// 무차원 수(0.86 같은 보정) — 개수 제한 없음
		default:
			return false
		}
	}
	return length == 1 && scale == 1
}

// slideGuardRuleErr 은 규칙 하나의 검사 결과(없으면 ""). font 단축 속성은 슬라이드 규칙에서 금지한다.
func slideGuardRuleErr(r cssBlock) string {
	for _, piece := range slideSelectorPieces(r.prelude) {
		if !slideGuardChecked(piece) {
			continue
		}
		for decl := range strings.SplitSeq(r.body, ";") {
			k, v, ok := strings.Cut(decl, ":")
			if !ok {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(k)) {
			case "font":
				return fmt.Sprintf("%s { font: %s } — font 단축 속성은 금지(font-size 를 따로)", strings.TrimSpace(piece), strings.TrimSpace(v))
			case "font-size":
				if !slideFontSizeOK(v) {
					return fmt.Sprintf("%s { font-size: %s } — calc(Npt * var(--slide-scale))·em·퍼센트·inherit 꼴만", strings.TrimSpace(piece), strings.TrimSpace(v))
				}
			}
		}
	}
	return ""
}

// TestSlideFontSizeScaleGuard — 내장 CSS 전부에서 슬라이드 전용 규칙의 글자 크기가 배율 허용 꼴인지.
// 파싱 실패는 곧 실패, 파일마다 검사한 슬라이드 규칙이 0개여도 실패(파서가 규칙을 통째로 놓쳤는지 보인다).
// 이 검사는 내장 테마·base.css 에서 흔히 쓰는 꼴의 회귀를 잡는다(완전한 CSS 해석기가 아니다).
func TestSlideFontSizeScaleGuard(t *testing.T) {
	files := map[string]string{}
	if b, err := web.FS.ReadFile("base.css"); err == nil {
		files["web/base.css"] = string(b)
	} else {
		t.Fatal(err)
	}
	dirs, err := themes.FS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dirs {
		p := "themes/" + d.Name() + "/theme.css"
		b, err := themes.FS.ReadFile(d.Name() + "/theme.css")
		if err != nil {
			continue // 테마마다 theme.css 가 있지만, 없는 디렉터리는 그냥 지나간다
		}
		files[p] = string(b)
	}
	for path, css := range files {
		rules, err := slideCSSRules(css)
		if err != nil {
			t.Fatalf("%s: CSS 파싱 실패: %v", path, err)
		}
		n := 0
		for _, r := range rules {
			if msg := slideGuardRuleErr(r); msg != "" {
				at := strings.Index(css, strings.TrimSpace(strings.Split(r.prelude, ",")[0])) // 원본 줄 근사
				line := 1
				if at >= 0 {
					line += strings.Count(css[:at], "\n")
				}
				t.Errorf("%s:%d %s", path, line, msg)
			}
			if slices.ContainsFunc(slideSelectorPieces(r.prelude), slideGuardChecked) {
				n++
			}
		}
		if n == 0 {
			t.Errorf("%s: 검사한 슬라이드 규칙이 0개 — 파서가 규칙을 놓쳤을 수 있다", path)
		}
	}
}

// TestSlideFontSizeValueForms — 값 판정의 표 시험(리뷰 3회차 [1] 반례 전부).
func TestSlideFontSizeValueForms(t *testing.T) {
	ok := []string{
		"calc(104pt * var(--slide-scale))",
		"calc(13pt*var(--slide-scale))",
		"calc(var(--slide-title-size) * 0.86 * var(--slide-scale))",
		"calc(var(--slide-cover-size)*var(--slide-scale))",
		"calc(var(--slide-section-size) * 1.5 * var(--slide-scale))",
		"0.88em", "1em", ".5em", "50%", "inherit",
		"calc(13pt * var(--slide-scale)) !important",
	}
	for _, v := range ok {
		if !slideFontSizeOK(v) {
			t.Errorf("통과해야 할 값: %q", v)
		}
	}
	bad := []string{
		"44pt", "44PT", "3rem", "1.2rem", "14px",
		"var(--fixed-size)", "var(--slide-section-size)", // var 단독은 참조값이 고정일 수 있다
		"calc(44pt + 0pt * var(--slide-scale))",      // 곱셈이 아니라 덧셈 — 결과는 언제나 44pt
		"calc(44pt * 2)", "calc(var(--slide-scale))", // 배율만 남는 크기는 유효한 CSS 가 아니다
		"calc(44pt * var(--slide-scale) * var(--slide-scale))", // 배율 중복 — 0.9 에서 제곱이 곱해진다(리뷰 4회차 [2])
		"calc(2 * var(--slide-scale))",                         // 길이 인수가 없다(단위 없는 font-size)
		"calc(44pt * 2pt * var(--slide-scale))",                // 길이 인수가 둘
		"44pt !important", "larger", "",
	}
	for _, v := range bad {
		if slideFontSizeOK(v) {
			t.Errorf("거짓 통과: %q", v)
		}
	}
}

// TestSlideGuardSelectorForms — 선택자 조각 판정의 표 시험(리뷰 3회차 [3] 반례).
func TestSlideGuardSelectorForms(t *testing.T) {
	ok := []string{".sc-class", ".slide-foot", ".sf-num", ".sf-left", ".sf-logo"} // 예외(정확 일치만)
	for _, s := range ok {
		if slideGuardChecked(s) {
			t.Errorf("예외여야 할 조각: %q", s)
		}
	}
	bad := []string{
		".ss-num:not(.sc-class)",    // :not 안의 예외 이름은 이 조각을 예외로 만들지 않는다
		".slide-foot + .slide-head", // 최종 대상은 머리 — 예외 아님
		".ss-num:is(.sc-class, .other)",
		".sc-class .sc-title", // 예외 안의 슬라이드 요소
		".ss-num", ".sc-title", ".slide-title",
	}
	for _, s := range bad {
		if !slideGuardChecked(s) {
			t.Errorf("검사 대상이어야 할 조각: %q", s)
		}
	}
	if slideGuardChecked(".result") || slideGuardChecked("p") {
		t.Errorf("슬라이드 전용이 아닌 조각이 검사 대상으로 판정됨")
	}
}

// TestSlideGuardRuleForms — 규칙 단위 표 시험(리뷰 3회차 [1]·[2] 반례: font 단축·대문자 이름·문자열 {).
func TestSlideGuardRuleForms(t *testing.T) {
	bad := []string{
		".ss-num { font: 700 44pt sans-serif; }",
		".ss-num { FONT-SIZE: 44pt; }",
		".ss-num { font-size: 44pt; }",
		`.ss-num::before { content: "{"; font-size: 44pt; }`, // 문자열 { 를 중첩으로 오인하면 통째로 놓친다
		"@media print { .sc-title { font-size: 40pt; } }",
		".ss-num:not(.sc-class) { font-size: 44pt; }",
		".slide-foot + .slide-head { font-size: 44pt; }",
	}
	for _, css := range bad {
		rules, err := slideCSSRules(css)
		if err != nil {
			t.Errorf("%q: 파싱 실패: %v", css, err)
			continue
		}
		var msgs []string
		for _, r := range rules {
			if m := slideGuardRuleErr(r); m != "" {
				msgs = append(msgs, m)
			}
		}
		if len(msgs) == 0 {
			t.Errorf("거짓 통과: %q", css)
		}
	}
	ok := []string{
		".page-body .ss-num { color: var(--ink); font-size: calc(104pt * var(--slide-scale)); }",
		".sc-class { margin-left: auto; font-size: 10pt; }",    // 관리 표시 예외(정확 일치)
		".slide-foot { position: absolute; font-size: 11pt; }", // 바닥글 예외
		".slide-title .slide-cont { font-size: 0.55em; }",      // em
		".result { font-size: 9.6pt; }",                        // 본문 서식 — 약속 밖
		":where(.page.slide .page-body ul ul) { font-size: 0.88em; }",
		// at-rule 이름 변형(리뷰 4회차 [1]) — atName 이 뽑은 이름(소문자)으로 식별한다
		"@MEDIA print { .ss-num { font-size: calc(44pt * var(--slide-scale)); } }",
		"@media\nprint { .ss-num { font-size: calc(44pt * var(--slide-scale)); } }",
		"@supports(display: grid) { .sc-title { font-size: calc(40pt * var(--slide-scale)); } }",
		"@keyframes hyve-x { from { opacity: 0; } }", // 안의 규칙은 슬라이드 표식이 없어 통과
	}
	for _, css := range ok {
		rules, err := slideCSSRules(css)
		if err != nil {
			t.Errorf("%q: 파싱 실패: %v", css, err)
			continue
		}
		for _, r := range rules {
			if m := slideGuardRuleErr(r); m != "" {
				t.Errorf("통과해야 할 규칙: %q — %s", css, m)
			}
		}
	}
	if _, err := slideCSSRules(".ss-num { font-size: 44pt;"); err == nil {
		t.Errorf("깨진 CSS 가 파싱 성공으로 판정됨")
	}
	// 지원하지 않는 @묶음(안에 규칙이 있다)은 건너뛰지 않고 파싱 실패(리뷰 4회차 [1])
	if _, err := slideCSSRules("@document url(x) { .ss-num { font-size: 44pt; } }"); err == nil {
		t.Errorf("지원하지 않는 @묶음이 파싱 성공으로 판정됨")
	}
}
