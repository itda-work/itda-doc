package page

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// BuiltinThemeProblems 는 내장(또는 내장 꼴로 불러온) 테마 묶음 ts 를 이름 순으로 검사해 문제 문장
// "<테마>: <무엇>" 을 모은다. 빈 목록이면 통과다. 보는 것은 다섯이다 — 옛 엔진과 새 엔진의 PDF 가 같도록
// (1) derived 식의 값이 같은 이름의 정적 값(tokens)과 같고 (2) tokens 에 상대 색 식(from )이 없으며,
// 테마 규칙이 쪽 밖을 고르지 않도록 (3) 테마 CSS 가 @scope (.page) 안에 있고, 슬라이드 배율이 먹도록
// (4) 슬라이드 전용 규칙의 font-size 가 --slide-scale 허용 꼴이다(slideGuardProblems). 또 (5) theme.json 의 group 이
// 어휘 밖이면(GroupWarning) 문제다 — 사용자 테마에서는 경고일 뿐이지만 내장 테마는 고쳐서 낸다. group 이 없는 것은 문제가 아니다.
// 앱은 자기 내장 테마를 더한 묶음으로 부를 수 있다(itda-hyve#577).
func BuiltinThemeProblems(ts *Themes) []string {
	var out []string
	for _, p := range builtinThemeProblems(ts) {
		out = append(out, p.msg)
	}
	return out
}

// themeCheck 는 검사의 갈래다 — 시험이 갈래별로 나눠 보려고 둔다.
type themeCheck int

const (
	checkTheme   themeCheck = iota // 테마를 쓸 수 없음
	checkDerived                   // (1)·(2) 파생 색·from 금지
	checkScope                     // (3) @scope
	checkSlide                     // (4) 슬라이드 글자 크기
	checkGroup                     // (5) 어휘 밖 group
)

type themeProblem struct {
	kind themeCheck
	msg  string
}

func builtinThemeProblems(ts *Themes) []themeProblem {
	var names []string
	for _, i := range ts.List() {
		names = append(names, i.Name)
	}
	slices.Sort(names)
	var out []themeProblem
	add := func(k themeCheck, msg string) { out = append(out, themeProblem{k, msg}) }
	for _, name := range names {
		th, ok := ts.Get(name)
		if !ok {
			why, _ := ts.Broken(name)
			add(checkTheme, fmt.Sprintf("%s: 쓸 수 없는 테마 — %s", name, why))
			continue
		}
		if w := th.GroupWarning(); w != "" {
			add(checkGroup, name+": "+w)
		}
		for _, p := range derivedProblems(th) {
			add(checkDerived, p)
		}
		if !strings.Contains(th.StyleSheet(CSSMode{}), "@scope (.page) {\n/* ---- ") {
			add(checkScope, name+": 테마 CSS 가 @scope 안에 있지 않음")
		}
		// th.CSS 는 불러올 때 거른 꼴이고 물려받은 테마의 규칙도 든다 — 줄 번호는 th.CSS 안의 위치다.
		if th.CSS != "" { // theme.css 가 없는 테마는 슬라이드 규칙도 없다
			for _, p := range slideGuardProblems(th.CSS) {
				add(checkSlide, name+": "+p)
			}
		}
	}
	return out
}

// derivedProblems — 옛 WebKit 은 상대 색을 모른다. 내장 테마는 정적 값(tokens)을 쓰고, 지원하는 엔진만 식(derived)으로
// 덮는다. 내장 테마의 강조색은 고정이므로 정적 값은 그 강조색으로 계산한 식의 값과 같아야 한다(어느 엔진에서나 같은 PDF).
// 계산은 CSS Color 4 의 OKLab(sRGB 로 자르기)이다 — WebKit 이 그린 값과 8개 모두 같았다(실측).
// tokens 에는 상대 색 식이 없어야 한다(옛 엔진에서 무효가 된다).
func derivedProblems(th *Theme) []string {
	var out []string
	vals := map[string]string{}
	for _, kv := range th.Tokens {
		vals[kv[0]] = kv[1]
	}
	for _, kv := range th.Derived {
		got, err := evalRelColor(kv[1], vals)
		if err != nil {
			out = append(out, fmt.Sprintf("%s: %s: %v", th.Name, kv[0], err))
			continue
		}
		if !strings.EqualFold(got, vals[kv[0]]) {
			out = append(out, fmt.Sprintf("%s: %s 의 정적 값 %s ≠ 식의 값 %s (%s)", th.Name, kv[0], vals[kv[0]], got, kv[1]))
		}
	}
	for _, kv := range th.Tokens {
		if strings.Contains(kv[1], "from ") {
			out = append(out, fmt.Sprintf("%s: tokens 의 %s 에 상대 색 식 — derived 로 옮겨라", th.Name, kv[0]))
		}
	}
	return out
}

// ---- 슬라이드 글자 배율 가드(itda-hyve#430 2단계, 리뷰 1~3회차) ----
// base.css 와 내장 테마의 슬라이드 전용 요소(표지 .sc-*·구역 .ss-*·슬라이드 머리/바닥 .slide-*)는
// font-size 가 --slide-scale 배율을 받아야 한다. 테마의 고정 pt 선언이 calc 를 덮쳐 누락이 1·2회차 리뷰에
// 연속 나왔으므로(grep 은 여러 줄 선언을 놓쳤다) 규칙 단위로 읽어 막는다. 3회차부터는 해석을 줄이고
// **허용 꼴만 통과**시킨다(거짓 통과를 없애고 거짓 실패를 감수 — 대상은 우리가 관리하는 내장 CSS 다).
// 쪽 문서와 함께 쓰는 본문 서식(:::result·절차 번호·결재란·서명 — 선택자에 슬라이드 전용 표식이 없다)는
// 배율 약속 밖이다.

// slideGuardProblems 는 CSS 한 벌의 가드 결과다. 파싱 실패는 곧 문제, 검사한 슬라이드 규칙이 0개여도 문제다
// (파서가 규칙을 통째로 놓쳤는지 보인다). 문장은 "<줄>행 <선택자> { … } — …" 꼴이다(줄은 css 안의 근사 위치).
// 이 검사는 내장 테마·base.css 에서 흔히 쓰는 꼴의 회귀를 잡는다(완전한 CSS 해석기가 아니다).
func slideGuardProblems(css string) []string {
	rules, err := slideCSSRules(css)
	if err != nil {
		return []string{fmt.Sprintf("CSS 파싱 실패: %v", err)}
	}
	var out []string
	n := 0
	for _, r := range rules {
		if msg := slideGuardRuleErr(r); msg != "" {
			at := strings.Index(css, strings.TrimSpace(strings.Split(r.Prelude, ",")[0])) // 원본 줄 근사
			line := 1
			if at >= 0 {
				line += strings.Count(css[:at], "\n")
			}
			out = append(out, fmt.Sprintf("%d행 %s", line, msg))
		}
		if slices.ContainsFunc(slideSelectorPieces(r.Prelude), slideGuardChecked) {
			n++
		}
	}
	if n == 0 {
		out = append(out, "검사한 슬라이드 규칙이 0개 — 파서가 규칙을 놓쳤을 수 있다")
	}
	return out
}

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
		if b.Statement {
			continue
		}
		pre := strings.TrimSpace(b.Prelude)
		if strings.HasPrefix(pre, "@") {
			name := strings.ToLower(atName(pre))
			if slices.Contains(nestAtRuleName, name) {
				inner, err := slideCSSRulesInner(b.Body)
				if err != nil {
					return nil, err
				}
				out = append(out, inner...)
				continue
			}
			if hasRuleBlocks(b.Body) { // 안에 규칙 블록이 있는데 지원하지 않는 묶음 — 실패
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
		if !b.Statement {
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
	for _, piece := range slideSelectorPieces(r.Prelude) {
		if !slideGuardChecked(piece) {
			continue
		}
		for decl := range strings.SplitSeq(r.Body, ";") {
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

// pageRootSel — 선택자 조각이 쪽 상자(.page) 자신으로 시작하는가(.page-body 같은 다른 이름은 아니다).
var pageRootSel = regexp.MustCompile(`^\.page(?:$|[^-_a-zA-Z0-9])`)

// pageRootProblems 는 테마 CSS 한 벌에서 .page 로 시작하는 규칙을 찾는다(itda-work/itda-hyve#362 리뷰 1회차 [1]).
// 테마 CSS 는 @scope (.page) 안에 들어가고(StyleSheet), 그 안의 선택자는 :scope 뿌리의 자손으로 읽히므로
// .page.board .tile 은 「.page 안의 또 다른 .page.board」 를 찾아 아무것도 고르지 못한다(Chrome 실측).
// 판 꼴마다 다른 모양은 판 안 요소(.board-deco·.board-head·.page-body.board-grid …)로 시작해 고른다.
// BuiltinThemeProblems 에는 넣지 않았다 — 앱이 더한 테마(hyve 브랜드)에 같은 꼴(.page.slide …)이 있어 그 판단은 따로 한다.
func pageRootProblems(css string) []string {
	rules, err := slideCSSRules(css)
	if err != nil {
		return []string{fmt.Sprintf("CSS 파싱 실패: %v", err)}
	}
	var out []string
	for _, r := range rules {
		if strings.HasPrefix(strings.TrimSpace(r.Prelude), "@") {
			continue // @font-face 같은 리프 @규칙
		}
		for _, p := range slideSelectorPieces(r.Prelude) {
			if pageRootSel.MatchString(p) {
				out = append(out, p+" — @scope (.page) 안에서 .page 로 시작하는 선택자는 아무것도 고르지 못한다")
			}
		}
	}
	return out
}
