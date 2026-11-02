package page

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/itda-work/itda-doc/mdoc"
)

func builtin(t *testing.T) *Themes {
	t.Helper()
	ts, err := BuiltinThemes()
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

func render(t *testing.T, src, theme string) View {
	t.Helper()
	return RenderView([]byte(src), "/docs/a.md", "/_doc/", builtin(t), theme, false)
}

func TestAssembleCoverAndTOC(t *testing.T) {
	v := render(t, "---\ntitle: 사업 계획\nsubtitle: 영업본부\ncover: true\ntoc: true\norganization: 잇다\ndoc-no: A-1\ndate: 2026-10-02\nclassification: 대외비\n---\n\n# 하나\n\n본문\n\n## 둘\n", "classic")
	h := v.HTML
	order := []string{`<section class="cover">`, `<h1 class="cover-title">사업 계획</h1>`, `<p class="cover-subtitle">영업본부</p>`,
		`<dl class="cover-meta">`, `data-key="doc-no"><dt>문서번호</dt><dd>A-1</dd>`, `<section class="toc-page">`,
		`<h2 class="toc-title">목차</h2>`, `<nav class="toc"`, `<div class="page-break"></div>`, `<h1 id="하나"`}
	last := -1
	for _, s := range order {
		i := strings.Index(h, s)
		if i < 0 {
			t.Fatalf("없음: %s\n%s", s, h)
		}
		if i < last {
			t.Errorf("순서가 틀림: %s", s)
		}
		last = i
	}
	if v.Theme != "classic" || v.DocTheme != "" || v.Paper != "a4" {
		t.Errorf("theme=%s doc=%s paper=%s", v.Theme, v.DocTheme, v.Paper)
	}
}

func TestAssembleTitleBlockAndPlacedTOC(t *testing.T) {
	// 표지 없음 + title → 제목 블록. [TOC] 로 자리를 정했으면 앞에 또 넣지 않는다.
	v := render(t, "---\ntitle: 공지\ntoc: true\n수신: 각 부서장\n참조: 인사팀\n---\n\n[TOC]\n\n# 가\n", "mono")
	if !strings.Contains(v.HTML, `<header class="title-block"><h1 class="title">공지</h1><dl class="title-meta"><div class="meta-row" data-key="수신"><dt>수신</dt><dd>각 부서장</dd></div><div class="meta-row" data-key="참조"><dt>참조</dt><dd>인사팀</dd></div></dl>`) {
		t.Errorf("제목 블록:\n%s", v.HTML)
	}
	if strings.Count(v.HTML, `<nav class="toc"`) != 1 || strings.Contains(v.HTML, "toc-page") {
		t.Errorf("목차는 [TOC] 자리에 한 번만:\n%s", v.HTML)
	}
	// mono 은 cover: true 여도 표지를 쓰지 않는다
	v = render(t, "---\ntitle: 공지\ncover: true\n---\n본문\n", "mono")
	if strings.Contains(v.HTML, "cover") || !strings.Contains(v.HTML, "title-block") {
		t.Errorf("mono 은 표지 대신 제목 블록:\n%s", v.HTML)
	}
	// 제목이 없으면 둘 다 없다
	if v := render(t, "# 그냥\n", "classic"); strings.Contains(v.HTML, "title-block") || strings.Contains(v.HTML, "cover") {
		t.Errorf("제목 없는 문서에 제목 블록:\n%s", v.HTML)
	}
}

// 메타 값은 글자다 — 표지·제목 블록·머리글 어디에서도 HTML 로 해석되지 않는다.
func TestAssembleEscapesMeta(t *testing.T) {
	evil := `<img src=x onerror=alert(1)>"&'`
	src := "---\ntitle: '" + strings.ReplaceAll(evil, "'", "''") + "'\nsubtitle: '<script>alert(2)</script>'\ncover: true\norganization: '<b>조직</b>'\ndoc-no: '\"><svg onload=alert(3)>'\n수신: '<i>x</i>'\n---\n본문\n"
	for _, theme := range []string{"classic", "mono"} {
		v := render(t, src, theme)
		for _, bad := range []string{"<img src=x", "<script>", "<b>조직", "<svg", "<i>x"} {
			if strings.Contains(v.HTML, bad) {
				t.Errorf("%s: 이스케이프 안 됨 %q:\n%s", theme, bad, v.HTML)
			}
		}
		if !strings.Contains(v.HTML, "&lt;img src=x onerror=alert(1)&gt;&#34;&amp;&#39;") {
			t.Errorf("%s: 제목이 글자로 남아야 함:\n%s", theme, v.HTML)
		}
		// 머리글 조각은 글자로만 넘긴다(화면은 textContent 로 넣는다)
		for _, slots := range []Slots{v.Header, v.Footer} {
			for _, s := range slots {
				for _, p := range s {
					if p.Field != "" && p.Field != "page" && p.Field != "pages" {
						t.Errorf("모르는 조각 종류 %q", p.Field)
					}
				}
			}
		}
	}
}

func TestParseSlot(t *testing.T) {
	m := mdoc.Meta{Title: "계획서", Organization: "잇다", DocNo: "A-7", Extra: []mdoc.MetaField{{Key: "시행일", Value: "2026. 10. 5."}}}
	str := func(ps []Part) string {
		var b strings.Builder
		for _, p := range ps {
			if p.Field != "" {
				b.WriteString("[")
				b.WriteString(p.Field)
				b.WriteString("]")
			} else {
				b.WriteString(p.Text)
			}
		}
		return b.String()
	}
	cases := []struct{ tpl, want string }{
		{"{organization} · {title}", "잇다 · 계획서"},
		{"{page} / {pages}", "[page] / [pages]"},
		{"- {page} -", "- [page] -"},
		{"시행 {시행일}", "시행 2026. 10. 5."},
		{"{doc-no}", "A-7"},
		{"{classification} · {title}", "계획서"}, // 빈 값 옆 구분자를 지운다
		{"{title} · {classification}", "계획서"}, // 뒤쪽도
		{"시행 {없는키}", ""},                      // 값이 하나도 없으면 자리를 비운다
		{"{classification} {page}", "[page]"}, // 쪽번호가 있으면 남긴다(빈 값 옆 공백은 지운다)
		{"고정 글자", "고정 글자"},
		{"", ""},
		{"{<b>}", "{<b>}"}, // 자리표시자 이름에 꺾쇠가 와도 글자 그대로(화면이 textContent 로 넣는다)
	}
	for _, c := range cases {
		if got := str(parseSlot(c.tpl, m, nil)); got != c.want {
			t.Errorf("%q → %q, 원함 %q", c.tpl, got, c.want)
		}
	}
}

func TestRenderViewThemePick(t *testing.T) {
	ts := builtin(t)
	v := RenderView([]byte("---\ntheme: mono\n---\n본문"), "/a/b.md", "/_doc/", ts, "", false)
	if v.Theme != "mono" || v.DocTheme != "mono" {
		t.Errorf("별칭: %s / %s", v.Theme, v.DocTheme)
	}
	v = RenderView([]byte("---\ntheme: mono\n---\n본문"), "/a/b.md", "/_doc/", ts, "bold", false)
	if v.Theme != "bold" || v.DocTheme != "mono" {
		t.Errorf("화면에서 고른 테마가 이긴다: %s / %s", v.Theme, v.DocTheme)
	}
	v = RenderView([]byte("---\ntheme: nope\n---\n본문"), "/a/b.md", "/_doc/", ts, "", false)
	if v.Theme != "classic" || !hasCode(v, "unknown-theme") {
		t.Errorf("모르는 테마는 classic + 경고: %s %+v", v.Theme, v.Warnings)
	}
	v = RenderView([]byte("![a](https://example.com/a.png) ![b](images/b.png)"), "/a/b.md", "/_doc/", ts, "", false)
	if v.RemoteImages != 1 || !strings.Contains(v.HTML, `src="/_doc/images/b.png"`) || strings.Contains(v.HTML, `<img src="https://`) {
		t.Errorf("원격 그림은 막고 상대 경로는 /_doc/: %d\n%s", v.RemoteImages, v.HTML)
	}
	v = RenderView([]byte("![a](https://example.com/a.png)"), "/a/b.md", "/_doc/", ts, "", true)
	if !strings.Contains(v.HTML, `src="https://example.com/a.png"`) || v.RemoteImages != 0 || !v.RemoteAllowed {
		t.Errorf("원격 그림을 켠 문서: %s", v.HTML)
	}
	if v.Title != "b" || v.FileName != "b.md" {
		t.Errorf("제목이 없으면 파일 이름: %q %q", v.Title, v.FileName)
	}
	if v := RenderView([]byte("---\npapersize: a4-landscape\n---\n"), "/a.md", "/_doc/", ts, "", false); v.Paper != "a4-landscape" {
		t.Errorf("paper = %s", v.Paper)
	}
}

func hasCode(v View, code string) bool {
	for _, w := range v.Warnings {
		if w.Code == code {
			return true
		}
	}
	return false
}

func TestEditorTokensOnlyInAppRender(t *testing.T) {
	src := []byte("```go\nfmt.Println(\"hello\")\n```\n")
	v := renderView(src, "/a/b.md", "", builtin(t), "", false)
	if len(v.editorTokens) != 1 || len(v.EditorTokens) != 0 {
		t.Fatalf("View 토큰 경계: %+v", v)
	}
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "editor_tokens") {
		t.Fatal("일반 보기 JSON 에 편집기 토큰이 섞임")
	}
}
