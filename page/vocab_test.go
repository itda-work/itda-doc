package page

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/itda-work/itda-doc/mdsyntax"
	"github.com/itda-work/itda-doc/web"
)

// 회의록·절차 어휘 넷(itda-doc#28)은 표준 어휘다 — 모든 테마에서 기본 모양이 있다.
var standardVocab = []string{"decisions", "actions", "steps", "result"}

const vocabDoc = `---
title: 어휘 견본
---

:::decisions
- 정한 것
- 정한 것 둘
:::

:::actions
| 할 일 | 담당 | 기한 |
|---|---|---|
| 보고 | 김 | 10/9 |
:::

:::steps
1. 첫째 단계

   :::result
   보이는 화면
   :::

2. 둘째 단계

   1. 하위 항목
:::
`

// 경고 0 회귀 방지: 내장 다섯 테마 각각에서 넷을 쓴 문서가 경고 없이 그려지고, 넷이 제 클래스의 블록으로 나간다.
// 모르는 이름도 원래 <div class> 로 경고 없이 나가므로 이 시험은 표준 어휘가 된 것을 묶지 않는다 — 그 묶음은
// TestStandardVocabBaseCSS(기본 모양)와 아래 완성 후보 확인(컨테이너 표)의 몫이다.
func TestStandardVocabRendersInEveryBuiltinTheme(t *testing.T) {
	names := map[string]bool{}
	for _, c := range mdsyntax.Completions() {
		if c.Kind == "container" {
			names[c.Name] = true
		}
	}
	for _, c := range standardVocab {
		if !names[c] {
			t.Errorf("문법 완성 후보(컨테이너 표)에 %s 가 없음", c)
		}
	}
	ts := builtin(t)
	for _, name := range []string{"classic", "bold", "compact", "mono", "spine"} {
		v := RenderView([]byte(vocabDoc), "/docs/a.md", "/_doc/", ts, name, false)
		if v.Theme != name {
			t.Errorf("%s: 고른 테마가 %s", name, v.Theme)
		}
		if len(v.Warnings) != 0 {
			t.Errorf("%s: 경고 %+v", name, v.Warnings)
		}
		for _, c := range standardVocab {
			if !strings.Contains(v.HTML, `<div class="`+c+`"`) {
				t.Errorf("%s: %s 블록이 없음", name, c)
			}
		}
	}
}

// base.css 에 넷의 기본 모양이 있고, 그 규칙은 선택자 전체가 :where(…) 라 특이도 0 이다(::before·::after 만 붙을 수 있다) —
// 테마 CSS 가 언제나 이긴다. 색은 base.css 가 늘 정의하는 토큰(var(--…))만 쓰고 색 값을 박지 않는다.
// 번호 목록 들여쓰기(.ol-d*)도 특이도 0 이라 넷의 규칙이 그보다 뒤에 있어야 열 단계 넘는 :::steps 가 들여쓰이지 않는다.
func TestStandardVocabBaseCSS(t *testing.T) {
	b, err := web.FS.ReadFile("base.css")
	if err != nil {
		t.Fatal(err)
	}
	css := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(b), "")
	root := regexp.MustCompile(`(?s):root\s*\{([^}]*)\}`).FindStringSubmatch(css)
	if root == nil {
		t.Fatal("base.css 에 :root 가 없음")
	}
	defined := map[string]bool{}
	for _, m := range regexp.MustCompile(`(--[\w-]+)\s*:`).FindAllStringSubmatch(root[1], -1) {
		defined[m[1]] = true
	}
	defined["--head-code-line"] = true // 할 일 표 머리 규칙이 th 에서 정하고 그 안 code 가 읽는 지역 변수(page/actionshead_test.go)
	vocabSel := regexp.MustCompile(`\.(decisions|actions|steps|result)\b`)
	pseudo := regexp.MustCompile(`::(before|after)$`)
	colorLit := regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b|\brgba?\(|\bhsla?\(|\boklch\(`)
	seen := map[string]bool{}
	firstVocab := -1
	for _, m := range regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`).FindAllStringSubmatchIndex(css, -1) {
		sel, decl := strings.TrimSpace(css[m[2]:m[3]]), css[m[4]:m[5]]
		if !vocabSel.MatchString(sel) {
			continue
		}
		if firstVocab < 0 {
			firstVocab = m[0]
		}
		for _, piece := range splitTopComma(sel) {
			piece = pseudo.ReplaceAllString(strings.TrimSpace(piece), "")
			if !wholeWhere(piece) {
				t.Errorf("어휘 규칙의 특이도가 0 이 아님(선택자 전체를 :where 로): %s", sel)
			}
			for _, c := range vocabSel.FindAllStringSubmatch(piece, -1) {
				seen[c[1]] = true
			}
		}
		if colorLit.MatchString(decl) {
			t.Errorf("어휘 규칙에 박힌 색 값: %s { %s }", sel, strings.TrimSpace(decl))
		}
		for _, v := range regexp.MustCompile(`var\((--[\w-]+)`).FindAllStringSubmatch(decl, -1) {
			if !defined[v[1]] {
				t.Errorf("어휘 규칙이 base.css :root 에 없는 토큰 %s 를 씀: %s", v[1], sel)
			}
		}
	}
	for _, c := range standardVocab {
		if !seen[c] {
			t.Errorf("base.css 에 .%s 규칙이 없음", c)
		}
	}
	if last := strings.LastIndex(css, "ol.ol-d9"); firstVocab < 0 || last > firstVocab {
		t.Errorf("어휘 규칙(%d)이 번호 목록 들여쓰기 규칙(%d)보다 앞에 있음", firstVocab, last)
	}
	// 단계 모양은 :::steps 바로 아래 번호 목록에만 건다 — 후손(.steps ol)·하위 목록(.steps ol ol)에 걸면 단계 안 하위 목록이
	// 기호 사다리(공문 가.)·.ol-d* 들여쓰기를 잃는다(itda-doc#28 리뷰 1회차 [1])
	if m := regexp.MustCompile(`\.steps\s+ol|\.steps\s*>\s*ol[^{,]*\bol\b`).FindString(css); m != "" {
		t.Errorf("base.css 의 steps 규칙이 직계가 아님: %q", m)
	}
	// 쪽 나눔 뒷조각은 번호 원을 다시 그리지 않고, 할 일 표는 머리 칸 색을 토큰으로 바꾸고,
	// 슬라이드에서는 번호 원이 em 으로 본문 글자를 따른다(크기 값 자체는 눈 확인 몫)
	for _, want := range []string{
		`:where(.steps > ol > li.cont)::before { content: none; }`,
		`:where(.page.slide .steps > ol > li)::before {`,
		`:where(.actions) { --table-head-bg: var(--accent);`,
	} {
		if !strings.Contains(css, want) {
			t.Errorf("base.css 에 %q 가 없음", want)
		}
	}
}

// splitTopComma 는 괄호 밖 쉼표로 선택자 목록을 나눈다.
func splitTopComma(sel string) []string {
	var out []string
	depth, start := 0, 0
	for i, c := range sel {
		switch c {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, sel[start:i])
				start = i + 1
			}
		}
	}
	return append(out, sel[start:])
}

// 내장 중립 테마에는 새 문서 서식(template.md)이 없다 — 서식은 앱의 서식 갤러리가 맡는다(itda-doc#28).
// 회사 테마의 template.md 경로는 견본 acme(TestSampleCompanyTheme)가 묶는다.
func TestBuiltinThemesHaveNoTemplate(t *testing.T) {
	ts := builtin(t)
	for _, i := range ts.List() {
		if th, _ := ts.Get(i.Name); i.HasTemplate || th.Template != "" {
			t.Errorf("%s: 내장 테마에 template.md 가 있음", i.Name)
		}
	}
}

// 회사 테마가 template.md 없이 회사 테마(견본 acme)를 물려받으면 그 서식을 물려받아 새 문서 메뉴에 보인다
// (page/theme.go 의 *t = *parent). 바탕이 내장 테마면 물려받을 서식이 없다.
func TestTemplateInheritsFromCompanyTheme(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join("..", "themes", "samples", "acme")
	// 폴더째 복사 — 견본에 자산이 늘어도 물려받기와 무관한 까닭으로 깨지지 않게
	if err := os.CopyFS(filepath.Join(dir, "acme"), os.DirFS(src)); err != nil {
		t.Fatal(err)
	}
	writeTree(t, dir, map[string]string{
		"child/theme.json": `{"extends":"acme"}`,
		"plain/theme.json": `{"extends":"classic"}`,
	})
	ts, err := LoadThemeSet(builtinFS(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, i := range ts.List() {
		if i.Error != "" {
			t.Errorf("%s: %s", i.Name, i.Error)
		}
		got[i.Name] = i.HasTemplate
	}
	if !got["acme"] || !got["child"] {
		t.Errorf("acme·child 는 서식이 있어야 함: %v", got)
	}
	if got["plain"] {
		t.Error("내장 테마(classic)를 물려받은 plain 에 서식이 있음")
	}
}
