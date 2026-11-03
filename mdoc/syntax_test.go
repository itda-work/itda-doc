package mdoc

import (
	"reflect"
	"strings"
	"testing"
)

// 1장의 절마다 표 하나. 결과 HTML 의 일부(want)가 있고 not 이 없는지, 기대한 경고가 났는지 본다.
type syntaxCase struct {
	name string
	src  string
	want []string
	not  []string
	warn string // 기대하는 경고 코드("" 이면 경고가 없어야 한다)
}

func runCases(t *testing.T, cases []syntaxCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := render(t, c.src)
			contains(t, r.HTML, c.want...)
			notContains(t, r.HTML, c.not...)
			if c.warn == "" && len(r.Warnings) > 0 {
				t.Errorf("경고가 없어야 함: %+v", r.Warnings)
			}
			if c.warn != "" && !hasWarn(r, c.warn) {
				t.Errorf("경고 %s 가 없음: %+v", c.warn, r.Warnings)
			}
		})
	}
}

// 1.2 frontmatter
func TestFrontmatter(t *testing.T) {
	src := `---
title: 2026년 4분기 사업 계획
subtitle: 영업본부
author: [홍길동, {name: 김철수}]
date: 2026-10-02
lang: ko
toc: true
toc-depth: 2
number-sections: true
papersize: A4-landscape
theme: classic
cover: true
doc-no: 기획-2026-041
version: "1.2"
classification: 대외비
organization: 잇다워크
수신: 각 부서장
참조: [재무팀, 법무팀]
시행일: 2026-10-05
결재:
  담당: 홍길동
---

본문
`
	r := render(t, src)
	m := r.Meta
	if !m.Present || m.Title != "2026년 4분기 사업 계획" || m.Subtitle != "영업본부" ||
		!reflect.DeepEqual(m.Author, []string{"홍길동", "김철수"}) || m.Date != "2026-10-02" ||
		m.Lang != "ko" || !m.TOC || m.TOCDepth != 2 || !m.NumberSections || m.PaperSize != "a4-landscape" ||
		m.Theme != "classic" || !m.Cover || m.DocNo != "기획-2026-041" || m.Version != "1.2" ||
		m.Classification != "대외비" || m.Organization != "잇다워크" {
		t.Errorf("아는 키가 틀림: %+v", m)
	}
	var keys []string
	for _, f := range m.Extra {
		keys = append(keys, f.Key)
	}
	if !reflect.DeepEqual(keys, []string{"수신", "참조", "시행일", "결재"}) {
		t.Errorf("모르는 키의 순서가 보존되지 않음: %v", keys)
	}
	if f, _ := m.Field("참조"); f.Value != "재무팀, 법무팀" || len(f.List) != 2 {
		t.Errorf("목록 값: %+v", f)
	}
	if f, _ := m.Field("결재"); !strings.Contains(f.Value, "담당: 홍길동") {
		t.Errorf("구조 값은 YAML 글자로: %+v", f)
	}
	if len(r.Warnings) > 0 {
		t.Errorf("경고 없어야 함: %+v", r.Warnings)
	}
	// frontmatter 자리를 빈 줄로 채워 본문 줄 번호가 원문과 같다
	contains(t, r.HTML, `<p data-line="24">본문</p>`)

	t.Run("기본값", func(t *testing.T) {
		m := render(t, "본문").Meta
		if m.Present || m.TOCDepth != 3 || m.PaperSize != "a4" {
			t.Errorf("기본값: %+v", m)
		}
	})
	t.Run("깨진 YAML 은 본문으로", func(t *testing.T) {
		r := render(t, "---\ntitle: [닫히지 않음\n---\n\n본문\n")
		if r.Meta.Present || r.Meta.Title != "" || !hasWarn(r, WarnFrontmatterInvalid) {
			t.Errorf("깨진 frontmatter: %+v %+v", r.Meta, r.Warnings)
		}
		contains(t, r.HTML, "<hr", "닫히지 않음", "본문")
	})
	t.Run("매핑이 아닌 YAML 도 본문으로", func(t *testing.T) {
		r := render(t, "---\n그냥 글\n---\n")
		if r.Meta.Present || !hasWarn(r, WarnFrontmatterInvalid) {
			t.Errorf("%+v %+v", r.Meta, r.Warnings)
		}
	})
	t.Run("닫는 줄이 없으면 frontmatter 가 아님", func(t *testing.T) {
		r := render(t, "---\n\n본문\n")
		if r.Meta.Present || len(r.Warnings) > 0 {
			t.Errorf("%+v %+v", r.Meta, r.Warnings)
		}
		contains(t, r.HTML, "<hr")
	})
	t.Run("값 형식이 틀리면 경고하고 기본값", func(t *testing.T) {
		r := render(t, "---\ntoc: 네\ntoc-depth: 9\npapersize: letter\ntitle: [a, b]\n---\n")
		if r.Meta.TOC || r.Meta.TOCDepth != 3 || r.Meta.PaperSize != "a4" || r.Meta.Title != "" {
			t.Errorf("%+v", r.Meta)
		}
		n := 0
		for _, w := range r.Warnings {
			if w.Code == WarnFrontmatterValue {
				n++
				if w.Line < 2 || w.Line > 5 {
					t.Errorf("경고 줄이 원문 줄이 아님: %+v", w)
				}
			}
		}
		if n != 4 {
			t.Errorf("경고 4개 기대: %+v", r.Warnings)
		}
	})
	t.Run("CRLF·BOM", func(t *testing.T) {
		r := render(t, "\ufeff---\r\ntitle: 제목\r\n---\r\n\r\n본문\r\n")
		if r.Meta.Title != "제목" {
			t.Errorf("%+v", r.Meta)
		}
		contains(t, r.HTML, `<p data-line="5">본문</p>`)
	})
	t.Run("값은 HTML 로 해석하지 않음", func(t *testing.T) {
		r := render(t, "---\ntitle: <script>alert(1)</script>\n---\n\n본문\n")
		if r.Meta.Title != "<script>alert(1)</script>" {
			t.Errorf("값은 글자 그대로: %q", r.Meta.Title)
		}
		notContains(t, r.HTML, "<script", "alert")
	})
}

// 1.3 제목 id·속성·번호
func TestHeadings(t *testing.T) {
	runCases(t, []syntaxCase{
		{name: "GitHub 식 id", src: "## Hello World! 한글 제목?",
			want: []string{`<h2 id="hello-world-한글-제목" data-line="1">Hello World! 한글 제목?</h2>`}},
		{name: "공백을 몰아 쓰지 않음", src: "# a - b",
			want: []string{`id="a---b"`}},
		{name: "꾸밈은 글자만", src: "# **굵게** `code` [링크](x)",
			want: []string{`id="굵게-code-링크"`}},
		{name: "겹치면 -1 -2", src: "# 개요\n\n# 개요\n\n# 개요",
			want: []string{`id="개요"`, `id="개요-1"`, `id="개요-2"`}},
		{name: "직접 적은 id 를 피해 감", src: "# 개요\n\n# 다른 {#개요-1}\n\n# 개요",
			want: []string{`id="개요-1" data-line="3">다른</h1>`, `id="개요-2"`}},
		{name: "속성", src: "## 제목 {#custom .wide key=값}",
			want: []string{`<h2 id="custom" class="wide" data-key="값" data-line="1">제목</h2>`}},
		{name: "닫는 # 뒤 속성도", src: "## 제목 {#c} ##", want: []string{`<h2 id="c" data-line="1">제목</h2>`}},
		{name: "공백 없이 붙은 중괄호는 글자", src: "## 제목{#c}", want: []string{`>제목{#c}</h2>`}},
		{name: "빈 제목", src: "## {#only}", want: []string{`<h2 id="only" data-line="1"></h2>`}},
		{name: "못 읽은 속성은 글자로", src: "## 제목 {#a\"b}", want: []string{`제목 {#a&quot;b}</h2>`}, warn: WarnBadAttribute},
		{name: "겹친 직접 id", src: "# a {#x}\n\n# b {#x}", warn: WarnDuplicateID},
		{name: "제목 글자의 HTML", src: "## 제목 <script>alert(1)</script> <b>굵게</b>",
			not: []string{"<script", "<b>"}, warn: WarnRawHTML},
	})
}

func TestNumberSections(t *testing.T) {
	src := "---\nnumber-sections: true\n---\n\n# 하나\n\n## 하나의 하나\n\n### 깊이\n\n## 건너뜀 {-}\n\n## 하나의 둘\n\n# 둘 {.unnumbered}\n\n# 셋\n\n### 단계 건너뜀\n"
	r := render(t, src)
	contains(t, r.HTML,
		`<span class="header-section-number">1</span> 하나</h1>`,
		`<span class="header-section-number">1.1</span> 하나의 하나`,
		`<span class="header-section-number">1.1.1</span> 깊이`,
		`class="unnumbered" data-line="11">건너뜀</h2>`,
		`<span class="header-section-number">1.2</span> 하나의 둘`,
		`class="unnumbered" data-line="15">둘</h1>`,
		`<span class="header-section-number">2</span> 셋`,
		`<span class="header-section-number">2.0.1</span> 단계 건너뜀`,
	)

	t.Run("h1 이 없으면 h2 가 1", func(t *testing.T) {
		r := render(t, "---\nnumber-sections: true\n---\n\n## 가\n\n### 나\n")
		contains(t, r.HTML, `number">1</span> 가`, `number">1.1</span> 나`)
	})
	t.Run("꺼져 있으면 번호 없음", func(t *testing.T) {
		notContains(t, render(t, "# 가").HTML, "header-section-number")
	})
}

// 1.4 쪽 나눔
func TestPageBreak(t *testing.T) {
	const pb = `<div class="page-break" data-line="3"></div>`
	runCases(t, []syntaxCase{
		{name: "주석", src: "가\n\n<!-- pagebreak -->\n\n나", want: []string{pb}},
		{name: "주석 대소문자·공백", src: "가\n\n<!--   PageBreak -->\n\n나", want: []string{pb}},
		{name: "div page-break-after", src: "가\n\n<div style=\"page-break-after: always\"></div>\n\n나", want: []string{pb}},
		{name: "div break-after page", src: "가\n\n<div style='break-after:page;'></div>\n\n나", want: []string{pb}},
		{name: "newpage", src: "가\n\n\\newpage\n\n나", want: []string{pb}},
		{name: "newpage 는 한 줄 단독일 때만", src: "가\n\\newpage", want: []string{`\newpage`}, not: []string{"page-break"}},
		{name: "다른 style 의 div 는 버림", src: "가\n\n<div style=\"color:red\"></div>\n\n나",
			not: []string{"page-break", "<div"}, warn: WarnRawHTML},
		{name: "내용 있는 div 는 버림", src: "가\n\n<div style=\"page-break-after: always\">x</div>\n\n나",
			not: []string{"page-break"}, warn: WarnRawHTML},
		{name: "따옴표 짝이 틀린 div", src: "가\n\n<div style=\"page-break-after: always'></div>\n\n나",
			not: []string{"page-break"}, warn: WarnRawHTML},
		{name: "다른 주석은 조용히 버림", src: "가\n\n<!-- 메모 -->\n\n나", not: []string{"<!--", "메모"}},
		{name: "컨테이너 안에서도", src: ":::box\n\\newpage\n:::", want: []string{`<div class="box" data-line="1">` + "\n" + `<div class="page-break"></div>`}},
	})
}

// 1.5 컨테이너
func TestContainers(t *testing.T) {
	runCases(t, []syntaxCase{
		{name: "이름", src: ":::right\n2026년 10월 2일\n잇다워크 대표 홍길동\n:::",
			want: []string{"<div class=\"right\" data-line=\"1\">\n<p>2026년 10월 2일\n잇다워크 대표 홍길동</p>\n</div>"}},
		{name: "공백 뒤 이름과 끝 콜론", src: "::: box :::\n안\n:::", want: []string{`<div class="box"`}},
		{name: "한글 이름", src: ":::서명란\n안\n:::", want: []string{`<div class="서명란"`}},
		{name: "속성만", src: "::: {#a .b .c k=v}\n안\n:::", want: []string{`<div class="b c" id="a" data-k="v" data-line="1">`}},
		{name: "이름과 속성", src: ":::box{.wide #x n=\"두 단어\"}\n안\n:::", want: []string{`<div class="box wide" id="x" data-n="두 단어" data-line="1">`}},
		{name: "다단", src: "::::columns\n:::column{width=40%}\n왼쪽\n:::\n:::column\n오른쪽\n:::\n::::",
			want: []string{"<div class=\"columns\" data-line=\"1\">\n<div class=\"column\" style=\"width:40%\">\n<p>왼쪽</p>\n</div>\n<div class=\"column\">\n<p>오른쪽</p>\n</div>\n</div>"}},
		{name: "Pandoc 식 클래스 column 의 폭", src: ":::: {.columns}\n::: {.column width=40%}\n왼쪽\n:::\n::::",
			want: []string{"<div class=\"columns\" data-line=\"1\">\n<div class=\"column\" style=\"width:40%\">\n<p>왼쪽</p>"}, not: []string{"data-width"}},
		{name: "클래스 column 의 길이 아닌 폭", src: "::: {.column width=\"1px;color:red\"}\n안\n:::",
			want: []string{`<div class="column" data-line="1">`}, not: []string{"style=", "data-width"}, warn: WarnBadAttribute},
		{name: "이름이 다르고 클래스가 column", src: ":::box{.column width=30mm}\n안\n:::", want: []string{`<div class="box column" style="width:30mm"`}},
		{name: "mm 폭", src: ":::column{width=60mm}\n안\n:::", want: []string{`style="width:60mm"`}},
		{name: "단위 없는 폭은 px", src: ":::column{width=120}\n안\n:::", want: []string{`style="width:120px"`}},
		{name: "column 아닌 width 는 data", src: ":::box{width=40%}\n안\n:::", want: []string{`data-width="40%"`}, not: []string{"style="}},
		{name: "가장 안쪽을 닫음", src: ":::a\n:::b\n안\n:::\n밖\n:::",
			want: []string{"<div class=\"a\" data-line=\"1\">\n<div class=\"b\">\n<p>안</p>\n</div>\n<p>밖</p>\n</div>"}},
		{name: "안은 보통의 마크다운", src: ":::box\n# 제목\n\n- 항목\n\n> 인용\n:::",
			want: []string{"<h1 id=\"제목\">제목</h1>", "<li>항목</li>", "<blockquote>"}},
		{name: "코드 안의 ::: 는 코드", src: ":::box\n```\n:::\n```\n:::\n\n밖",
			want: []string{"<pre><code>:::\n</code></pre>\n</div>", `<p data-line="7">밖</p>`}},
		{name: "닫지 않으면 문서 끝에서", src: ":::box\n안", want: []string{"<p>안</p>\n</div>"}, warn: WarnUnclosedContainer},
		{name: "문단 중간에서는 열리지 않음", src: "문단\n:::box\n안\n:::", want: []string{":::box"}, not: []string{"<div"}},
		{name: "콜론 둘은 컨테이너 아님", src: "::box\n안", not: []string{"<div"}},
		{name: "이름 없는 콜론 줄은 여는 줄 아님", src: ":::\n안", not: []string{"<div"}},
		{name: "목록 안", src: "- 항목\n\n  :::box\n  안\n  :::\n", want: []string{"<li>\n<p>항목</p>\n<div class=\"box\">\n<p>안</p>\n</div>\n</li>"}},
		{name: "인용 안", src: "> :::box\n> 안\n> :::\n", want: []string{"<blockquote data-line=\"1\"><div class=\"box\">\n<p>안</p>\n</div>\n</blockquote>"}},
	})
}

// 닫는 줄은 가장 안쪽에 열린 컨테이너의 것이다 — 사이에 목록·인용·각주·정의 목록이 있어도(itda-hyve#139)
func TestContainerCloseInnermost(t *testing.T) {
	steps := "<div class=\"steps\" data-line=\"1\">\n<ol>\n<li>\n<p>첫째</p>\n<div class=\"result\">\n<p>결과</p>\n</div>\n</li>\n<li>\n<p>둘째</p>\n</li>\n</ol>\n</div>\n"
	for _, c := range []struct {
		name, src, want string
		warns           []Warning
	}{
		{"목록 항목 안", ":::steps\n1. 첫째\n\n   :::result\n   결과\n   :::\n\n2. 둘째\n:::\n", steps, nil},
		{"바깥 콜론이 더 많아도", "::::steps\n1. 첫째\n\n   :::result\n   결과\n   :::\n\n2. 둘째\n::::\n", steps, nil},
		{"목록 두 겹 안", ":::a\n- x\n  - :::b\n    안\n    :::\n:::\n",
			"<div class=\"a\" data-line=\"1\">\n<ul>\n<li>x\n<ul>\n<li>\n<div class=\"b\">\n<p>안</p>\n</div>\n</li>\n</ul>\n</li>\n</ul>\n</div>\n", nil},
		{"인용 안의 목록 안", ":::a\n> - :::b\n>   안\n>   :::\n:::\n",
			"<div class=\"a\" data-line=\"1\">\n<blockquote>\n<ul>\n<li>\n<div class=\"b\">\n<p>안</p>\n</div>\n</li>\n</ul>\n</blockquote>\n</div>\n", nil},
		{"정의 설명 안", ":::a\n용어\n: :::b\n  안\n  :::\n:::\n",
			"<div class=\"a\" data-line=\"1\">\n<dl>\n<dt>용어</dt>\n<dd><div class=\"b\">\n<p>안</p>\n</div>\n</dd>\n</dl>\n</div>\n", nil},
		{"각주 정의 안", ":::a\n[^1]: 각주\n\n    :::b\n    안\n    :::\n:::\n\n본문[^1]\n",
			"<div class=\"a\" data-line=\"1\">\n</div>\n<p data-line=\"9\">본문<sup id=\"fnref:1\"><a href=\"#fn:1\" class=\"footnote-ref\" role=\"doc-noteref\">1</a></sup></p>\n" +
				"<div class=\"footnotes\" role=\"doc-endnotes\" data-line=\"2\">\n<hr>\n<ol>\n<li id=\"fn:1\">\n<p>각주</p>\n<div class=\"b\">\n<p>안</p>\n</div>\n" +
				"&#160;<a href=\"#fnref:1\" class=\"footnote-backref\" role=\"doc-backlink\">&#x21a9;&#xfe0e;</a></li>\n</ol>\n</div>\n", nil},
		{"목록 안 펜스 코드의 ::: 는 코드", ":::a\n- ```\n  :::\n  ```\n:::\n",
			"<div class=\"a\" data-line=\"1\">\n<ul>\n<li>\n<pre><code>:::\n</code></pre>\n</li>\n</ul>\n</div>\n", nil},
		// 닫는 줄이 안쪽까지 닿지 못하면(목록 항목·인용이 끝남) 아직 열린 가장 안쪽 컨테이너를 닫는다 — 문단에 글자로 붙지 않는다
		{"목록이 끝나는 닫는 줄", ":::a\n- :::b\n  안\n:::\n뒤\n",
			"<div class=\"a\" data-line=\"1\">\n<ul>\n<li>\n<div class=\"b\">\n<p>안</p>\n</div>\n</li>\n</ul>\n</div>\n<p data-line=\"5\">뒤</p>\n",
			[]Warning{{Line: 2, Code: WarnUnclosedContainer, Message: ":::b 가 닫히지 않아 감싼 블록 끝에서 닫음"}}},
		{"인용이 끝나는 닫는 줄", "::::a\n:::b\n> :::c\n> 안\n:::\n밖\n::::\n",
			"<div class=\"a\" data-line=\"1\">\n<div class=\"b\">\n<blockquote>\n<div class=\"c\">\n<p>안</p>\n</div>\n</blockquote>\n</div>\n<p>밖</p>\n</div>\n",
			[]Warning{{Line: 3, Code: WarnUnclosedContainer, Message: ":::c 가 닫히지 않아 감싼 블록 끝에서 닫음"}}},
		{"안쪽이 없으면 목록 안 닫는 줄도 바깥 것", ":::a\n- 항목\n\n  :::\n뒤\n",
			"<div class=\"a\" data-line=\"1\">\n<ul>\n<li>항목</li>\n</ul>\n</div>\n<p data-line=\"5\">뒤</p>\n", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := render(t, c.src)
			if r.HTML != c.want {
				t.Errorf("HTML\n got %q\nwant %q", r.HTML, c.want)
			}
			if !reflect.DeepEqual(r.Warnings, c.warns) {
				t.Errorf("경고 %+v, 기대 %+v", r.Warnings, c.warns)
			}
		})
	}
}

// 1.6 알림
func TestAlerts(t *testing.T) {
	runCases(t, []syntaxCase{
		{name: "다섯 가지", src: "> [!NOTE]\n> a\n\n> [!TIP]\n> b\n\n> [!IMPORTANT]\n> c\n\n> [!WARNING]\n> d\n\n> [!CAUTION]\n> e",
			want: []string{
				"<div class=\"markdown-alert markdown-alert-note\" data-line=\"1\">\n<p class=\"markdown-alert-title\">참고</p>\n<p>a</p>\n</div>",
				`markdown-alert-tip" data-line="4">` + "\n" + `<p class="markdown-alert-title">팁</p>`,
				`<p class="markdown-alert-title">중요</p>`,
				`<p class="markdown-alert-title">주의</p>`,
				`<p class="markdown-alert-title">경고</p>`,
			}, not: []string{"[!", "<blockquote"}},
		{name: "소문자 표시", src: "> [!note]\n> a", want: []string{"markdown-alert-note"}},
		{name: "본문 여러 블록", src: "> [!WARNING]\n> 첫 문단\n>\n> - 목록",
			want: []string{"<p>첫 문단</p>\n<ul>\n<li>목록</li>\n</ul>\n</div>"}},
		{name: "표시만 있는 알림", src: "> [!TIP]", want: []string{"<p class=\"markdown-alert-title\">팁</p>\n</div>"}},
		{name: "모르는 종류는 인용", src: "> [!DANGER]\n> a", want: []string{"<blockquote", "[!DANGER]"}},
		{name: "표시 뒤에 글이 붙으면 인용", src: "> [!NOTE] 글\n> a", want: []string{"<blockquote"}},
		{name: "보통 인용", src: "> 인용", want: []string{"<blockquote"}, not: []string{"markdown-alert"}},
	})
	t.Run("영어 lang", func(t *testing.T) {
		r := render(t, "---\nlang: en-US\n---\n\n> [!CAUTION]\n> a")
		contains(t, r.HTML, `<p class="markdown-alert-title">Caution</p>`)
	})
}

// 1.7 그림·표 캡션·이미지 width
func TestFiguresAndCaptions(t *testing.T) {
	runCases(t, []syntaxCase{
		{name: "title 이 캡션", src: "![대체 글](a.png \"그림 캡션\")",
			want: []string{"<figure data-line=\"1\">\n<img src=\"a.png\" alt=\"대체 글\">\n<figcaption>그림 캡션</figcaption>\n</figure>"}},
		{name: "title 없으면 캡션 없음", src: "![대체 글](a.png)",
			want: []string{"<figure data-line=\"1\">\n<img src=\"a.png\" alt=\"대체 글\">\n</figure>"}, not: []string{"figcaption"}},
		{name: "글과 함께면 그림 아님", src: "글 ![a](a.png \"t\")",
			want: []string{`<p data-line="1">글 <img src="a.png" alt="a" title="t"></p>`}, not: []string{"<figure"}},
		{name: "width", src: "![](a.png){width=50%}", want: []string{`<img src="a.png" alt="" style="width:50%">`}, not: []string{"{"}},
		{name: "width 따옴표", src: "![](a.png){width=\"30mm\"}", want: []string{`style="width:30mm"`}},
		{name: "글 안의 width", src: "앞 ![](a.png){width=10%} 뒤", want: []string{`style="width:10%"> 뒤`}},
		{name: "width 말고는 버림", src: "![](a.png){width=10% .big}", want: []string{`style="width:10%"`}, not: []string{"big"}, warn: WarnBadAttribute},
		{name: "길이 아닌 width", src: "![](a.png){width=\"1px;background:red\"}", not: []string{"style=", "background"}, warn: WarnBadAttribute},
		{name: "띄어 쓴 중괄호는 글자", src: "![](a.png) {width=50%}", want: []string{"{width=50%}"}, not: []string{"style="}},
		{name: "표 캡션 Table:", src: "| a |\n|---|\n| 1 |\n\nTable: 표 *캡션*",
			want: []string{"<table data-line=\"1\" data-table-line=\"1\">\n<caption>표 <em>캡션</em></caption>\n<thead>"}, not: []string{"Table:"}},
		{name: "Figure: 문단 캡션", src: "![조직도](a.png)\n\nFigure: 2026년 **조직도**",
			want: []string{"<figure data-line=\"1\">\n<img src=\"a.png\" alt=\"조직도\">\n<figcaption>2026년 <strong>조직도</strong></figcaption>\n</figure>"}, not: []string{"Figure:"}},
		{name: "소문자 figure:", src: "![](a.png)\n\nfigure: 캡션", want: []string{"<figcaption>캡션</figcaption>"}},
		{name: "Figure: 캡션 여러 줄", src: "![](a.png)\n\nFigure: 첫 줄\n둘째 줄", want: []string{"<figcaption>첫 줄\n둘째 줄</figcaption>"}},
		{name: "섞인 문단의 마지막 줄 이미지", src: "설명 문장입니다.\n![조직도](a.png)\n\nFigure: 캡션",
			want: []string{"<p data-line=\"1\">설명 문장입니다.</p>",
				"<figure data-line=\"2\">\n<img src=\"a.png\" alt=\"조직도\">\n<figcaption>캡션</figcaption>\n</figure>"}},
		{name: "섞인 문단의 강제 줄바꿈 꼬리도 지움", src: "설명 문장입니다.  \n![조직도](a.png)\n\nFigure: 캡션",
			want: []string{"<p data-line=\"1\">설명 문장입니다.</p>", "<figure data-line=\"2\">"}, not: []string{"<br"}},
		{name: "title 과 Figure: 가 같이 있으면 Figure: 하나", src: "![](a.png \"title 캡션\")\n\nFigure: 문단 캡션",
			want: []string{"<figcaption>문단 캡션</figcaption>"}, not: []string{"title 캡션", "title="}},
		{name: "Figure: 없는 섞인 문단은 그대로", src: "설명 문장입니다.\n![조직도](a.png \"캡션\")",
			want: []string{"<p data-line=\"1\">설명 문장입니다.\n<img src=\"a.png\" alt=\"조직도\" title=\"캡션\"></p>"}, not: []string{"<figure"}},
		{name: "마지막 줄에 긘이 섞이면 분리 안 함", src: "글 ![a](a.png)\n\nFigure: x",
			want: []string{"<p data-line=\"1\">글 <img src=\"a.png\" alt=\"a\"></p>", "<p data-line=\"3\">Figure: x</p>"}},
		{name: "그림 뒤 : 문단은 정의 목록", src: "![](a.png)\n\n: x",
			want: []string{"<dl data-line=\"1\">\n<dt><img src=\"a.png\" alt=\"\"></dt>\n<dd>\n<p>x</p>\n</dd>\n</dl>"}, not: []string{"figcaption"}},
		{name: "그림 앞 Figure: 문단은 글자", src: "Figure: 앞\n\n![](a.png)", want: []string{"<p data-line=\"1\">Figure: 앞</p>"}, not: []string{"<figcaption"}},
		{name: "목록 안 그림 캡션", src: "- ![](a.png)\n\n  Figure: 캡션", want: []string{"<li>", "<figcaption>캡션</figcaption>"}},
		{name: "표 캡션 :", src: "| a |\n|---|\n| 1 |\n\n: 캡션", want: []string{"<caption>캡션</caption>"}},
		{name: "표 앞 캡션은 받지 않음", src: "Table: 앞\n\n| a |\n|---|\n| 1 |", want: []string{"<p data-line=\"1\">Table: 앞</p>"}, not: []string{"<caption"}},
		{name: "표 다음 보통 문단", src: "| a |\n|---|\n| 1 |\n\n보통", want: []string{"<p data-line=\"5\">보통</p>"}, not: []string{"<caption"}},
		{name: "표 정렬은 align 속성", src: "| a | b |\n|:-:|--:|\n| 1 | 2 |", want: []string{`<th align="center">a</th>`}, not: []string{"style="}},
	})
}

// 1.8 목차
func TestTOC(t *testing.T) {
	src := "---\ntoc: true\ntoc-depth: 2\nnumber-sections: true\n---\n\n# 가\n\n## 가-1\n\n### 깊어서 빠짐\n\n## 숨김 {.unlisted}\n\n# 나 {-}\n"
	r := render(t, src)
	want := []TOCItem{
		{Level: 1, ID: "가", Number: "1", Text: "가", Children: []TOCItem{
			{Level: 2, ID: "가-1", Number: "1.1", Text: "가-1"},
		}},
		{Level: 1, ID: "나", Text: "나"},
	}
	if !reflect.DeepEqual(r.TOC, want) {
		t.Errorf("TOC\n got %+v\nwant %+v", r.TOC, want)
	}
	if r.TOCPlaced {
		t.Error("[TOC] 가 없으면 HTML 에 넣지 않음")
	}
	notContains(t, r.HTML, "<nav")
	// 숨긴 제목도 번호는 받는다(목차에서만 빠진다)
	contains(t, r.HTML, `<span class="header-section-number">1.2</span> 숨김`)

	nav := RenderTOC(r.TOC)
	contains(t, nav, "<nav class=\"toc\" role=\"doc-toc\">\n<ul>\n<li><a href=\"#가\"><span class=\"toc-section-number\">1</span> 가</a>\n<ul>\n<li><a href=\"#가-1\">")
	if RenderTOC(nil) != "" {
		t.Error("빈 목차는 빈 글자")
	}

	t.Run("[TOC] 자리", func(t *testing.T) {
		r := render(t, "# 가\n\n[toc]\n\n## 나\n")
		if !r.TOCPlaced {
			t.Error("TOCPlaced 가 거짓")
		}
		contains(t, r.HTML, "<nav class=\"toc\" role=\"doc-toc\" data-line=\"3\">\n<ul>\n<li><a href=\"#가\">가</a>\n<ul>\n<li><a href=\"#나\">나</a></li>")
		notContains(t, r.HTML, "[toc]")
	})
	t.Run("[TOC] 가 링크 정의면 링크", func(t *testing.T) {
		r := render(t, "[TOC]\n\n[toc]: https://example.com\n")
		if r.TOCPlaced {
			t.Error("링크인데 목차로 봄")
		}
		contains(t, r.HTML, `<a href="https://example.com">TOC</a>`)
	})
	t.Run("기본 깊이 3", func(t *testing.T) {
		r := render(t, "# a\n## b\n### c\n#### d\n")
		if len(r.TOC) != 1 || len(r.TOC[0].Children[0].Children) != 1 || len(r.TOC[0].Children[0].Children[0].Children) != 0 {
			t.Errorf("%+v", r.TOC)
		}
	})
	t.Run("제목 글자의 HTML 은 목차에서 이스케이프", func(t *testing.T) {
		r := render(t, "# a &lt;b&gt; `<i>`\n\n[TOC]")
		contains(t, r.HTML, `<a href="#a-b-i">a &lt;b&gt; &lt;i&gt;</a>`)
	})
}

// 1.9 raw HTML·링크·이미지
func TestRawHTMLAndLinks(t *testing.T) {
	runCases(t, []syntaxCase{
		{name: "허용 태그", src: "H<sub>2</sub>O x<sup>2</sup> <kbd>Ctrl</kbd> <mark>표시</mark> <u>밑줄</u> <ins>삽입</ins> <small>작게</small> 줄<br>바꿈<br/>다시<BR />",
			want: []string{"H<sub>2</sub>O x<sup>2</sup> <kbd>Ctrl</kbd> <mark>표시</mark> <u>밑줄</u> <ins>삽입</ins> <small>작게</small> 줄<br>바꿈<br>다시<br>"}},
		{name: "표 칸 안 br", src: "| a |\n|---|\n| 1<br>2 |", want: []string{"<td>1<br>2</td>"}},
		{name: "속성 있는 허용 태그는 버림", src: "<sub class=\"x\">a</sub>", want: []string{">a</p>"}, not: []string{"<sub", "</sub>"}, warn: WarnRawHTML},
		{name: "닫히지 않은 태그는 버림", src: "<u>밑줄\n\n다음 문단", not: []string{"<u>"}, warn: WarnRawHTML},
		{name: "짝 없는 닫는 태그는 버림", src: "글</small>", not: []string{"</small>"}, warn: WarnRawHTML},
		{name: "엇갈린 짝", src: "<u><sub>a</u></sub>", want: []string{">a</sub>"}, not: []string{"<u>", "</u>"}, warn: WarnRawHTML},
		{name: "꾸밈을 넘는 짝은 버림", src: "<u>**a</u>**", not: []string{"<u>", "</u>"}, warn: WarnRawHTML},
		{name: "</br> 은 버림", src: "a</br>b", not: []string{"br"}, warn: WarnRawHTML},
		{name: "br 만 있는 블록", src: "가\n\n<br>\n\n나", want: []string{"<br>\n<p data-line=\"5\">나</p>"}},
		{name: "모르는 태그", src: "<span>글</span>", want: []string{">글</p>"}, not: []string{"<span"}, warn: WarnRawHTML},
		{name: "인라인 주석", src: "가 <!-- 메모 --> 나", not: []string{"메모", "<!--"}},
		{name: "링크 허용", src: "[a](https://x.com) [b](http://x.com) [c](mailto:a@b.c) [d](tel:010) [e](#절) [f](./문서.md) [g](//x.com)",
			want: []string{`href="https://x.com"`, `href="http://x.com"`, `href="mailto:a@b.c"`, `href="tel:010"`, `href="#%EC%A0%88"`, `href="./%EB%AC%B8%EC%84%9C.md"`, `href="//x.com"`}},
		{name: "링크 제목", src: `[a](x "제목 \"따옴표\"")`, want: []string{`title="제목 &quot;따옴표&quot;"`}},
		{name: "막는 스킴", src: "[a](file:///etc/passwd)", want: []string{">a</p>"}, not: []string{"<a"}, warn: WarnUnsafeURL},
		{name: "모르는 스킴도 막음", src: "[a](ms-msdt:/id)", not: []string{"<a"}, warn: WarnUnsafeURL},
		{name: "data 링크", src: "[a](data:text/html,x)", not: []string{"<a"}, warn: WarnUnsafeURL},
		{name: "자동 링크", src: "<https://x.com> www.example.com a@b.co",
			want: []string{`<a href="https://x.com">https://x.com</a>`, `<a href="http://www.example.com">www.example.com</a>`, `<a href="mailto:a@b.co">a@b.co</a>`}},
		{name: "상대 이미지는 그대로", src: "![a](img/그림.png)", want: []string{`src="img/%EA%B7%B8%EB%A6%BC.png"`}},
		{name: "data 이미지", src: "![a](data:image/png;base64,AAAA)", want: []string{`src="data:image/png;base64,AAAA"`}},
		{name: "data 이미지가 아닌 data", src: "![a](data:text/html,x)", not: []string{"src=", "data:text"}, warn: WarnUnsafeURL},
		{name: "원격 이미지는 기본 차단", src: "![a](https://x.com/p.png)",
			want: []string{`<img data-remote-src="https://x.com/p.png" alt="a">`}, not: []string{" src="}, warn: WarnRemoteImage},
		{name: "UNC 이미지는 원격", src: `![a](\\\\server\\share\\p.png)`, not: []string{" src="}, warn: WarnRemoteImage},
	})
	t.Run("원격 이미지 허용", func(t *testing.T) {
		r := render(t, "![a](https://x.com/p.png)", Options{AllowRemoteImages: true})
		contains(t, r.HTML, `<img src="https://x.com/p.png" alt="a">`)
		if len(r.Warnings) > 0 {
			t.Errorf("허용하면 경고 없음: %+v", r.Warnings)
		}
	})
	t.Run("경고 줄 번호", func(t *testing.T) {
		r := render(t, "가\n\n나 <span>x</span>\n\n<iframe src=x></iframe>\n")
		var lines []int
		for _, w := range r.Warnings {
			lines = append(lines, w.Line)
		}
		if !reflect.DeepEqual(lines, []int{3, 3, 5}) {
			t.Errorf("줄 %v (%+v)", lines, r.Warnings)
		}
	})
}

// 1.10 정의 목록·각주·코드·data-line
func TestRest(t *testing.T) {
	runCases(t, []syntaxCase{
		{name: "정의 목록", src: "수신\n: 각 부서장\n\n제목\n: 4분기 계획",
			want: []string{"<dl data-line=\"1\">\n<dt>수신</dt>\n<dd>각 부서장</dd>\n<dt>제목</dt>\n<dd>4분기 계획</dd>\n</dl>"}},
		{name: "각주는 문서 끝", src: "본문[^a]\n\n[^a]: 각주 *내용*\n\n끝 문단",
			want: []string{`<sup id="fnref:1"><a href="#fn:1" class="footnote-ref" role="doc-noteref">1</a></sup>`, "<p data-line=\"5\">끝 문단</p>\n<div class=\"footnotes\" role=\"doc-endnotes\"", "각주 <em>내용</em>"}},
		{name: "코드 블록", src: "```go\nfmt.Println(\"<b>\")\n```",
			want: []string{`<pre data-line="1"><code class="language-go"><span class="hl-`, `&lt;b&gt;`, `</code></pre>`}},
		{name: "언어 이름 정리", src: "```c++\nx\n```\n\n```x\"onclick=y\nz\n```",
			want: []string{`class="language-c++"`}, not: []string{`language-xonclicky`, `"onclick`}}, // 첫 낱말에 = 가 있으면 언어 없음(itda-hyve#441) — class 가 아예 안 나온다
		{name: "mermaid 흐름도는 그림(자세히는 diagram_test.go)", src: "```mermaid\ngraph TD\nA-->B\n```",
			want: []string{`<figure class="diagram" data-line="1">`, `<svg class="diagram-svg"`}},
		{name: "그리지 못한 mermaid 는 코드", src: "```mermaid\nsequenceDiagram\n```", want: []string{`<code class="language-mermaid">sequenceDiagram`}, warn: WarnDiagram},
		{name: "들여쓴 코드", src: "    코드 <b>", want: []string{"<pre data-line=\"1\"><code>코드 &lt;b&gt;\n</code></pre>"}},
		{name: "수식(자세히는 math_test.go)", src: "$a<b$", want: []string{"<math><mrow><mi>a</mi><mo>&lt;</mo><mi>b</mi></mrow></math>"}},
		{name: "작업 목록·취소선", src: "- [x] 끝\n- [ ] 남음\n\n~~취소~~",
			want: []string{`<input checked="" disabled="" type="checkbox"> 끝`, "<del>취소</del>"}},
		{name: "data-line 은 최상위 블록만", src: "# 제목\n\n- 가\n- 나\n\n> 인용\n> 둘째\n\n---\n\n문단",
			want: []string{`<h1 id="제목" data-line="1">`, `<ul data-line="3">` + "\n<li>가</li>", `<blockquote data-line="6"><p>인용`, `<hr data-line="9">`, `<p data-line="11">문단</p>`}},
		{name: "줄바꿈은 CommonMark 그대로", src: "가  \n나\\\n다\n라", want: []string{"가<br>\n나<br>\n다\n라"}},
	})
}

// 설계 문서 「열린 문제」의 한글 강조 경계. CJK-friendly 보정(cjk.go) 뒤의 동작을 고정한다.
// before 는 보정 전(CommonMark 그대로) 결과다 — 무엇이 바뀌었는지 보이게 남긴다.
func TestKoreanEmphasisBoundary(t *testing.T) {
	cases := []struct {
		src    string
		want   string // <p …> 안쪽
		before string // 보정 전. "" 이면 want 와 같다
	}{
		{`**"인용"**은`, `<strong>&quot;인용&quot;</strong>은`, `**&quot;인용&quot;**은`},
		{`**결론:**입니다`, `<strong>결론:</strong>입니다`, `**결론:**입니다`},
		{`**괄호(안)**를`, `<strong>괄호(안)</strong>를`, `**괄호(안)**를`},
		{`이것은**"인용"**이다`, `이것은<strong>&quot;인용&quot;</strong>이다`, `이것은**&quot;인용&quot;**이다`},
		{`*'따옴표'*를`, `<em>'따옴표'</em>를`, `*'따옴표'*를`},
		{`**50%**를`, `<strong>50%</strong>를`, `**50%**를`},
		{`**「인용」**은`, `<strong>「인용」</strong>은`, `**「인용」**은`},
		{`~~"취소"~~는`, `<del>&quot;취소&quot;</del>는`, `~~&quot;취소&quot;~~는`},
		{`**この文。**この`, `<strong>この文。</strong>この`, `**この文。**この`},
		{`__밑줄__은`, `__밑줄__은`, ""},                   // _ 는 단어 안에서 닫히지 않는다(명세도 그대로)
		{`**"인용"**abc`, `**&quot;인용&quot;**abc`, ""}, // 바깥이 CJK 가 아니면 그대로
		{`**강조**는`, `<strong>강조</strong>는`, ""},
		{`**"인용"** 은`, `<strong>&quot;인용&quot;</strong> 은`, ""},
		{`**결론:** 입니다`, `<strong>결론:</strong> 입니다`, ""},
		{`「**강조**」는`, `「<strong>강조</strong>」는`, ""},
		{"**葛\U000E0100**は", "<strong>葛\U000E0100</strong>は", ""}, // IVS 가 붙은 한자
		{"**\"인용\"**\uFE00은", "**&quot;인용&quot;**\uFE00은", ""},    // 바깥의 SVS 는 앞 글자가 아니다
	}
	for _, c := range cases {
		t.Run(c.src, func(t *testing.T) {
			got := strings.TrimSuffix(strings.TrimPrefix(render(t, c.src).HTML, `<p data-line="1">`), "</p>\n")
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
			before := c.before
			if before == "" {
				before = c.want
			}
			if got := plainGoldmark(t, c.src); got != before {
				t.Errorf("보정 전 got %q, want %q", got, before)
			}
		})
	}
}
