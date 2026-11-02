package page

import (
	"encoding/json"
	"github.com/itda-work/itda-doc/web"
	"regexp"
	"strings"
	"testing"
)

// 목록 기호 사다리(#141): 테마의 list-style 이 기본이고 문서의 list-style 이 덮는다. plain 은 끈다.
func TestListStyleThemeAndDoc(t *testing.T) {
	ts, err := BuiltinThemes()
	if err != nil {
		t.Fatal(err)
	}
	// mono 은 공공 문서 — 개조식 보고(□ ○)와 공문 항목(1. 가.)을 모두 켠다. 나머지는 보통 목록(문서가 고른다)
	for name, want := range map[string]string{"mono": "gaejosik gongmun", "classic": "", "bold": "", "compact": "", "spine": ""} {
		if th, _ := ts.Get(name); th.ListStyle != want {
			t.Errorf("%s: list-style %q, 기대 %q", name, th.ListStyle, want)
		}
	}
	cases := []struct{ theme, front, want string }{
		{"classic", "", ""},
		{"classic", "list-style: gaejosik", "gaejosik"},
		{"classic", "list-style: [gongmun, gaejosik]", "gaejosik gongmun"},
		{"mono", "", "gaejosik gongmun"},
		{"mono", "list-style: gongmun", "gongmun"},
		{"mono", "list-style: plain", ""},
		{"mono", "list-style: nope", "gaejosik gongmun"}, // 틀린 값은 경고하고 버린다 — 테마 기본
	}
	for _, c := range cases {
		src := "---\ntheme: " + c.theme + "\n" + c.front + "\n---\n\n- 항목\n"
		v := RenderView([]byte(src), "/tmp/a.md", "/_doc/", ts, "", false)
		if v.ListStyle != c.want {
			t.Errorf("%s + %q: list_style %q, 기대 %q", c.theme, c.front, v.ListStyle, c.want)
		}
		b, _ := json.Marshal(v)
		if !strings.Contains(string(b), `"list_style":"`+c.want+`"`) {
			t.Errorf("%s + %q: JSON 에 list_style 이 없음", c.theme, c.front)
		}
	}
	// 화면에서 테마를 바꿔 보면 바꾼 테마의 기본을 따른다(문서가 정하지 않았을 때)
	if v := RenderView([]byte("- 항목\n"), "/tmp/a.md", "/_doc/", ts, "mono", false); v.ListStyle != "gaejosik gongmun" {
		t.Errorf("테마 바꿔 보기: %q", v.ListStyle)
	}
}

func TestListStyleThemeFile(t *testing.T) {
	ts, err := LoadThemes(themeFS(map[string]string{
		"base/theme.json": `{"list-style": "gongmun"}`,
		"kid/theme.json":  `{"extends": "base"}`,
		"off/theme.json":  `{"extends": "base", "list-style": "plain"}`,
		"both/theme.json": `{"list-style": ["gaejosik", "gongmun", "gaejosik"]}`,
	}))
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"base": "gongmun", "kid": "gongmun", "off": "plain", "both": "gaejosik gongmun"} {
		if th, _ := ts.Get(name); th.ListStyle != want {
			t.Errorf("%s: %q, 기대 %q", name, th.ListStyle, want)
		}
	}
	for name, js := range map[string]string{
		"모르는 값":    `{"list-style": "bullets"}`,
		"plain 섞임": `{"list-style": ["plain", "gongmun"]}`,
		"빈 목록":     `{"list-style": []}`,
		"숫자":       `{"list-style": 1}`,
	} {
		if _, err := LoadThemes(themeFS(map[string]string{"x/theme.json": js})); err == nil {
			t.Errorf("%s: 거부해야 함", name)
		}
	}
}

// base.css 의 사다리 토큰이 부르는 기호 꼴(@counter-style hyve-…)은 모두 base.css 에 있고, 넘침 대체(fallback)도
// 있는 꼴이다. 단계 규칙은 개조식 넷·공문 여덟이고 목차·각주의 목록은 빠진다.
func TestListLadderCSS(t *testing.T) {
	b, err := web.FS.ReadFile("base.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(b)
	defined := map[string]bool{"decimal": true}
	for _, m := range regexp.MustCompile(`@counter-style (hyve-[a-z-]+) \{([^}]*)\}`).FindAllStringSubmatch(css, -1) {
		defined[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`(?:fallback|extends): *([a-z-]+);`).FindAllStringSubmatch(css, -1) {
		if !defined[m[1]] {
			t.Errorf("기호 꼴 %s 가 base.css 에 없음", m[1])
		}
	}
	for i := 1; i <= 8; i++ {
		m := regexp.MustCompile(`--list-ol-` + string(rune('0'+i)) + `: ([a-z-]+);`).FindStringSubmatch(css)
		if m == nil || !defined[m[1]] {
			t.Errorf("--list-ol-%d 의 꼴이 없음: %v", i, m)
		}
		sel := ":where(body.list-gongmun .page-body ol" + strings.Repeat(" ol", i-1) + ":not(nav ol, .footnotes ol)) { list-style-type: var(--list-ol-"
		if !strings.Contains(css, sel) {
			t.Errorf("공문 %d 단계 규칙이 없음", i)
		}
	}
	for i := 1; i <= 4; i++ {
		sel := ":where(body.list-gaejosik .page-body ul" + strings.Repeat(" ul", i-1) + ":not(nav ul, .footnotes ul)) { list-style-type: var(--list-ul-"
		if !strings.Contains(css, sel) {
			t.Errorf("개조식 %d 단계 규칙이 없음", i)
		}
	}
	// 가나다 차례는 행정업무운영 편람대로 가~하, 거~허 — 경고(mdoc)가 알아보는 차례와 같다
	ganada := regexp.MustCompile(`hyve-ganada-dot \{ system: fixed; symbols: ([^;]+);`).FindStringSubmatch(css)
	if ganada == nil || strings.ReplaceAll(ganada[1], " ", "") != "가나다라마바사아자차카타파하거너더러머버서어저처커터퍼허" {
		t.Errorf("가나다 차례: %v", ganada)
	}
}

// 리뷰 R1-1: base.css 에서 쪽 본문 목록(ul·ol)의 기호 꼴·들여쓰기를 정하는 규칙은 모두 선택자 전체가 :where(…) 라 특이도 0 이다 —
// 단계 깊이(ol ol ol)가 특이도를 올리면 테마의 목록 규칙(.decisions ol 등)을 깊은 단계에서 덮는다. 실제 엔진의 cascade 는
// selftest "목록 사다리" 항목이 본다(compact 결정 목록·spine 3단계 이상·각주 안 목록).
func TestListRulesZeroSpecificity(t *testing.T) {
	b, err := web.FS.ReadFile("base.css")
	if err != nil {
		t.Fatal(err)
	}
	css := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(b), "")
	css = regexp.MustCompile(`@counter-style[^{]*\{[^}]*\}`).ReplaceAllString(css, "")
	n := 0
	for _, m := range regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`).FindAllStringSubmatch(css, -1) {
		sel, decl := strings.TrimSpace(m[1]), m[2]
		if !strings.Contains(sel, ".page-body") || !regexp.MustCompile(`\b(ul|ol)\b`).MatchString(sel) ||
			!regexp.MustCompile(`(^|[;\s])(list-style(-type)?|padding-left)\s*:`).MatchString(decl) {
			continue
		}
		// 목록 항목(li)·각주·목차의 규칙은 대상이 아니다 — 목록 자신(ul·ol)이 끝인 선택자만
		if regexp.MustCompile(`\bli\b|footnotes|nav`).MatchString(regexp.MustCompile(`:not\([^)]*\)`).ReplaceAllString(sel, "")) {
			continue
		}
		n++
		if !wholeWhere(sel) {
			t.Errorf("목록 규칙의 특이도가 0 이 아님(선택자 전체를 :where 로): %s", sel)
		}
	}
	if n < 16 {
		t.Errorf("목록 규칙 %d 개만 찾음 — 시험의 선택자 판별이 틀렸을 수 있다", n)
	}
}

// wholeWhere 는 선택자가 통째로 :where(…) 하나인지다 — 여는 괄호의 짝이 선택자 끝이다.
func wholeWhere(sel string) bool {
	if !strings.HasPrefix(sel, ":where(") {
		return false
	}
	depth := 0
	for i, c := range sel {
		switch c {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i == len(sel)-1
			}
		}
	}
	return false
}
