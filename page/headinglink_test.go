package page

import (
	"regexp"
	"strings"
	"testing"

	"github.com/itda-work/itda-doc/web"
)

// 제목 안 링크(itda-work/itda-hyve#621): base.css 는 쪽 본문 제목(h1~h6)과 슬라이드 제목 띠(.slide-title) 안 링크의 글자색을 제목 색으로
// 물려받게 하고(color: inherit), 밑줄 색은 글자색(currentColor)을 옅게 섞는다. 특이도는 .page-body a 와 같은 (0,1,1)이라 그 뒤에 있어야
// base 의 --link 를 이기고, 테마 CSS 의 같은 특이도 규칙이 덮을 수 있다. 실제 엔진의 계산값은 이 저장소에 브라우저가 없어 보지 않는다(미검증).
func TestHeadingLinkInheritsColor(t *testing.T) {
	b, err := web.FS.ReadFile("base.css")
	if err != nil {
		t.Fatal(err)
	}
	css := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(b), "")
	type rule struct {
		at         int
		sel, decls string
	}
	var rules []rule
	for _, m := range regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`).FindAllStringSubmatchIndex(css, -1) {
		rules = append(rules, rule{m[0], strings.TrimSpace(css[m[2]:m[3]]), css[m[4]:m[5]]})
	}
	find := func(sel, decl string) int {
		for _, r := range rules {
			for _, s := range strings.Split(r.sel, ",") {
				if strings.TrimSpace(s) == sel && strings.Contains(r.decls, decl) {
					return r.at
				}
			}
		}
		return -1
	}
	// 선택자를 쉼표로 나누면 :where(h1, h2, …) 안도 갈라진다 — 같은 규칙에서 머리 조각과 꼬리 조각을 함께 찾는다.
	findHeading := func(tail, decl string) int {
		for _, r := range rules {
			if strings.Contains(r.sel, ".page-body :where(h1, h2, h3, h4, h5, h6) a"+tail) && strings.Contains(r.decls, decl) {
				return r.at
			}
		}
		return -1
	}
	link := find(".page-body a", "color: var(--link)")
	if link < 0 {
		t.Fatal("base.css 에 .page-body a 링크 색 규칙이 없음")
	}
	head := findHeading(":where(:not(.footnote-ref))", "color: inherit")
	if head < 0 {
		t.Fatal("제목 안 링크가 제목 색을 물려받는 규칙이 없음")
	}
	if head < link {
		t.Error("제목 안 링크 규칙이 .page-body a 보다 앞에 있어 같은 특이도의 --link 에 진다")
	}
	if find(".slide-title a:where(:not(.footnote-ref))", "color: inherit") < 0 {
		t.Error("슬라이드 제목 띠 안 링크가 제목 색을 물려받는 규칙이 없음")
	}
	if find(".slide-title a:where([href]:not(.footnote-ref))", "text-decoration: underline") < 0 {
		t.Error("슬라이드 제목 띠 안 링크의 옅은 밑줄 규칙이 없음(.page-body 밖이라 본문 밑줄 규칙이 닿지 않는다)")
	}
	if find(".slide-title a:where(.footnote-ref)", "color: var(--link)") < 0 {
		t.Error("슬라이드 제목 띠 안 각주 번호가 본문처럼 --link 색이 아님(엔진 기본 링크 색이 된다)")
	}
	mix := "color-mix(in srgb, currentColor 40%, transparent)"
	if findHeading(":where([href]:not(.footnote-ref))", mix) < 0 || find(".slide-title a:where([href]:not(.footnote-ref))", mix) < 0 {
		t.Error("제목 안 링크의 밑줄 색이 글자색(currentColor) 기반이 아님")
	}
}

// 본문 링크 안 인라인 코드는 본문 글자색(--ink)이고, 제목 안 링크의 코드는 제목 색(inherit)이다. 둘 다 특이도 (0,1,2)라 제목 규칙이
// 뒤에 있어야 이긴다. 실제 엔진의 계산값은 보지 않는다(미검증).
func TestLinkCodeColor(t *testing.T) {
	b, err := web.FS.ReadFile("base.css")
	if err != nil {
		t.Fatal(err)
	}
	css := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(b), "")
	body := strings.Index(css, ".page-body a code { color: var(--ink); }")
	head := strings.Index(css, ".page-body :where(h1, h2, h3, h4, h5, h6) a code { color: inherit; }")
	if body < 0 {
		t.Fatal("본문 링크 안 인라인 코드가 본문 글자색(--ink)인 규칙이 없음")
	}
	if head < 0 {
		t.Fatal("제목 안 링크의 인라인 코드가 제목 색을 따르는 규칙이 없음")
	}
	if head < body {
		t.Error("제목 안 링크 코드 규칙이 본문 규칙보다 앞에 있어 --ink 에 진다")
	}
}

// 표 머리 안 인라인 코드는 머리 글자색을 따르고 상자는 그 글자색을 옅게 깐다(itda-doc#19) — 진한 머리(bold·spine)에서 옅은
// --code-bg 위 흰 글자가 되지 않게. 본문 링크 안 코드 규칙(같은 특이도)보다 뒤에 있어야 머리 안 링크 코드도 따른다.
// 실제 엔진의 계산값은 보지 않는다(미검증).
func TestTableHeadCodeColor(t *testing.T) {
	b, err := web.FS.ReadFile("base.css")
	if err != nil {
		t.Fatal(err)
	}
	css := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(b), "")
	head := strings.Index(css, ".page-body th code { color: inherit; background: transparent; }")
	if head < 0 {
		t.Fatal("표 머리 안 인라인 코드가 머리 글자색을 따르는 규칙이 없음")
	}
	if !strings.Contains(css, ".page-body th code { background: color-mix(in srgb, currentColor 16%, transparent); }") {
		t.Error("표 머리 안 인라인 코드 상자가 글자색(currentColor) 기반이 아님")
	}
	if link := strings.Index(css, ".page-body a code { color: var(--ink); }"); link < 0 || head < link {
		t.Error("표 머리 코드 규칙이 본문 링크 안 코드 규칙보다 앞에 있어 머리 안 링크 코드가 --ink 가 된다")
	}
	if code := strings.Index(css, ".page-body code {"); code < 0 || head < code {
		t.Error("표 머리 코드 규칙이 인라인 코드 기본 규칙보다 앞에 있음")
	}
}

// 표지·장 슬라이드 큰 제목 안 인라인 코드도 제목 글자색을 따른다 — bold 장 슬라이드의 강조색 면 위 흰 제목에서
// 옅은 --code-bg 위 흰 글자가 되지 않게. 실제 엔진의 계산값은 보지 않는다(미검증).
func TestDisplayTitleCodeColor(t *testing.T) {
	b, err := web.FS.ReadFile("base.css")
	if err != nil {
		t.Fatal(err)
	}
	css := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(b), "")
	sel := ".page-body :is(.cover-title, .sc-title, .ss-title) code"
	rule := strings.Index(css, sel+" { color: inherit; background: transparent; }")
	if rule < 0 {
		t.Fatal("큰 제목 안 인라인 코드가 제목 글자색을 따르는 규칙이 없음")
	}
	if !strings.Contains(css, sel+" { background: color-mix(in srgb, currentColor 16%, transparent); }") {
		t.Error("큰 제목 안 인라인 코드 상자가 글자색(currentColor) 기반이 아님")
	}
	if code := strings.Index(css, ".page-body code {"); code < 0 || rule < code {
		t.Error("큰 제목 코드 규칙이 인라인 코드 기본 규칙보다 앞에 있음")
	}
}
