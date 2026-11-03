package page

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"sync"

	"github.com/itda-work/itda-doc/web"
)

// 테마 CSS 검사와 가두기. 테마 CSS 는 남이 쓴 파일이다(사용자 테마 폴더). 막는 것:
//   - 밖으로 나가는 요청: @import, assets/ 밖·원격 url(), 글자로 주소를 받는 함수(image-set("…")·src("…") 등).
//     문서 창의 CSP 가 2차로 막지만, 거부해야 테마를 쓴 사람이 고친다.
//   - 스크립트 흔적: expression(·javascript:·behavior:·-moz-binding.
//   - 창 틀 꾸미기: 테마 규칙은 @scope (.page) 안에 들어가 쪽 상자 안의 요소에만 맞는다. 그 밖을 노리는 선택자
//     (html·body·:root·:host)와 가두기를 벗어나는 표기(:scope·맨 위의 &)는 아무것도 바꾸지 못하지만, 거부해서
//     "창 틀은 꾸밀 수 없다" 를 알린다. 쪽 상자 밖에 둘 수밖에 없는 @font-face 만 꺼내 따로 낸다.
//   - 전역 이름을 만드는 at 규칙(@page·@layer·@keyframes·@property·@counter-style·@namespace 등). @media·@supports 만 받는다.
//
// 해석은 CSS 를 문법대로 다 읽지 않는다. 글자열·주석·괄호를 따라 맨 위 블록을 가르는 데까지만 한다 — 가두기의 근거는
// 이 해석이 아니라 @scope 이고, 해석은 거부 이유를 알려 주는 몫이다.

const maxThemeCSS = 256 << 10

var (
	cssURLRe = regexp.MustCompile(`(?i)url\(\s*(?:"([^"]*)"|'([^']*)'|([^)\s]*))\s*\)`)
	// 글자로 주소를 받는 함수: image-set("a.png" 1x), src("…"), image("…"), cross-fade(…) 등. url() 처럼 바꿔 줄 수 없어 거부한다.
	urlLikeFuncRe = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_-])(?:-webkit-)?(?:image-set|image|src|cross-fade|element|paint)\(`)
	// 창 틀을 노리는 선택자(쪽 상자 안에서는 맞는 것이 없다)
	chromeSelRe = regexp.MustCompile(`(?i)(?:^|[\s,>+~(])(?:html|body)(?:$|[\s,>+~.#:\[),])|:root\b|:host\b`)
)

// 받는 at 규칙(@scope 안에 그대로 둔다)과 꺼내는 at 규칙
var keptAtRules = map[string]bool{"media": true, "supports": true}

func checkThemeCSS(css, theme string, files map[string]themeAsset, rev int64) (scoped, fonts string, err error) {
	if len(css) > maxThemeCSS {
		return "", "", fmt.Errorf("너무 큼(%dKB 넘음)", maxThemeCSS>>10)
	}
	if strings.ContainsRune(css, 0) {
		return "", "", errors.New("NUL 글자가 있음")
	}
	css = stripCSSComments(css)
	low := strings.ToLower(css)
	for _, bad := range []string{"@import", "expression(", "javascript:", "behavior:", "-moz-binding", "</style", ":scope"} {
		if strings.Contains(low, bad) {
			if bad == ":scope" {
				return "", "", errors.New(":scope 는 쓸 수 없음 — 테마 규칙은 이미 쪽(.page) 안에서만 맞는다")
			}
			return "", "", fmt.Errorf("%s 는 쓸 수 없음", bad)
		}
	}
	if m := urlLikeFuncRe.FindString(low); m != "" {
		return "", "", fmt.Errorf("%s 는 쓸 수 없음 — 그림·글꼴은 url(assets/…) 로만", strings.TrimLeft(m, " \t\n,:;("))
	}
	var bad error
	css = cssURLRe.ReplaceAllStringFunc(css, func(m string) string {
		g := cssURLRe.FindStringSubmatch(m)
		u := g[1] + g[2] + g[3]
		p, err := themeRelPath(u)
		if err != nil {
			bad = fmt.Errorf("url(%s): %w", u, err)
			return m
		}
		full := ThemeURL(theme, p)
		if _, ok := files[full]; !ok {
			bad = fmt.Errorf("url(%s): assets 에 없는 파일", u)
			return m
		}
		return `url("` + withRev(full, rev) + `")`
	})
	if bad != nil {
		return "", "", bad
	}
	blocks, err := splitCSSBlocks(css)
	if err != nil {
		return "", "", err
	}
	var sb, fb strings.Builder
	if err := walkCSSBlocks(blocks, 0, &sb, &fb); err != nil {
		return "", "", err
	}
	return sb.String(), fb.String(), nil
}

func walkCSSBlocks(blocks []cssBlock, depth int, sb, fb *strings.Builder) error {
	for _, b := range blocks {
		pre := strings.TrimSpace(b.Prelude)
		if strings.HasPrefix(pre, "@") {
			name := strings.ToLower(atName(pre))
			switch {
			case name == "font-face" && depth == 0:
				ff, err := checkFontFace(b.Body)
				if err != nil {
					return fmt.Errorf("@font-face: %w", err)
				}
				fb.WriteString("@font-face {")
				fb.WriteString(ff)
				fb.WriteString("}\n")
			case keptAtRules[name] && !b.Statement:
				inner, err := splitCSSBlocks(b.Body)
				if err != nil {
					return err
				}
				sb.WriteString(pre)
				sb.WriteString(" {\n")
				if err := walkCSSBlocks(inner, depth+1, sb, fb); err != nil {
					return err
				}
				sb.WriteString("}\n")
			case name == "font-face":
				return errors.New("@font-face 는 맨 위에만 쓴다")
			default:
				return fmt.Errorf("@%s 는 쓸 수 없음(@media·@supports·@font-face 만)", name)
			}
			continue
		}
		if b.Statement {
			return fmt.Errorf("규칙 %q 에 { } 가 없음", shorten(pre))
		}
		if m := chromeSelRe.FindString(pre); m != "" {
			return fmt.Errorf("선택자 %q: %s 는 창 틀이라 테마가 꾸밀 수 없음 — 테마 규칙은 쪽(.page) 안에만 걸린다",
				shorten(pre), strings.Trim(m, " \t\n,>+~().#:[]"))
		}
		if strings.Contains(pre, "&") {
			return fmt.Errorf("선택자 %q: 맨 위 규칙에는 & 를 쓰지 않는다(쪽 상자 자신을 고르게 된다)", shorten(pre))
		}
		sb.WriteString(pre)
		sb.WriteString(" {")
		sb.WriteString(b.Body)
		sb.WriteString("}\n")
	}
	return nil
}

// @font-face 에 받는 서술자. src 는 assets 의 woff2 만(url 은 이미 앱 주소로 바뀌어 있다).
var fontDescriptors = map[string]bool{
	"font-family": true, "src": true, "font-weight": true, "font-style": true, "font-display": true, "unicode-range": true,
	"font-stretch": true, "size-adjust": true, "ascent-override": true, "descent-override": true, "line-gap-override": true,
}

func checkFontFace(body string) (string, error) {
	var out []string
	family, hasSrc := "", false
	for _, decl := range splitDecls(body) {
		k, v, ok := strings.Cut(decl, ":")
		if !ok {
			return "", fmt.Errorf("서술 %q 를 읽지 못함", shorten(decl))
		}
		k = strings.ToLower(strings.TrimSpace(k))
		v = strings.TrimSpace(v)
		if !fontDescriptors[k] {
			return "", fmt.Errorf("%s 는 쓸 수 없음", k)
		}
		switch k {
		case "font-family":
			family = strings.Trim(v, `"' `)
			if reservedFontFamilies()[strings.ToLower(family)] {
				return "", fmt.Errorf("글꼴 이름 %q 는 앱(창 틀·기본 글꼴)이 쓰는 이름이라 쓸 수 없음 — 다른 이름을 지어라", family)
			}
		case "src":
			hasSrc = true
			urls := cssURLRe.FindAllStringSubmatch(v, -1)
			if len(urls) == 0 {
				return "", errors.New("src 에 url(assets/….woff2) 가 없음")
			}
			for _, g := range urls {
				u, _, _ := strings.Cut(g[1]+g[2]+g[3], "?")
				if strings.ToLower(path.Ext(u)) != ".woff2" {
					return "", fmt.Errorf("글꼴 %s 는 woff2 여야 함", path.Base(u))
				}
			}
		}
		out = append(out, k+": "+v)
	}
	if family == "" || !hasSrc {
		return "", errors.New("font-family 와 src 가 있어야 함")
	}
	return " " + strings.Join(out, "; ") + "; ", nil
}

// cssBlock 은 맨 위의 규칙 하나다: Prelude { Body } 또는 Prelude; (Statement).
type cssBlock struct {
	Prelude, Body string
	Statement     bool
}

// splitCSSBlocks 는 CSS 를 맨 위 규칙으로 가른다. 글자열("…"·'…')·괄호 안의 { } ; 는 세지 않는다.
func splitCSSBlocks(css string) ([]cssBlock, error) {
	var out []cssBlock
	i, n := 0, len(css)
	start := 0
	for i < n {
		c := css[i]
		switch c {
		case '"', '\'':
			j, err := skipString(css, i)
			if err != nil {
				return nil, err
			}
			i = j
			continue
		case '(':
			j, err := skipParens(css, i)
			if err != nil {
				return nil, err
			}
			i = j
			continue
		case ';':
			if p := strings.TrimSpace(css[start:i]); p != "" {
				out = append(out, cssBlock{Prelude: p, Statement: true})
			}
			i++
			start = i
			continue
		case '{':
			end, err := matchBrace(css, i)
			if err != nil {
				return nil, err
			}
			out = append(out, cssBlock{Prelude: css[start:i], Body: css[i+1 : end]})
			i = end + 1
			start = i
			continue
		case '}':
			return nil, errors.New("짝이 없는 } 가 있음")
		}
		i++
	}
	if p := strings.TrimSpace(css[start:]); p != "" {
		out = append(out, cssBlock{Prelude: p, Statement: true})
	}
	return out, nil
}

func matchBrace(s string, open int) (int, error) {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '"', '\'':
			j, err := skipString(s, i)
			if err != nil {
				return 0, err
			}
			i = j - 1
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return 0, errors.New("닫히지 않은 { 가 있음")
}

func skipString(s string, i int) (int, error) {
	q := s[i]
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case q:
			return j + 1, nil
		case '\n':
			return 0, errors.New("닫히지 않은 글자열이 있음")
		}
	}
	return 0, errors.New("닫히지 않은 글자열이 있음")
}

func skipParens(s string, i int) (int, error) {
	depth := 0
	for j := i; j < len(s); j++ {
		switch s[j] {
		case '"', '\'':
			k, err := skipString(s, j)
			if err != nil {
				return 0, err
			}
			j = k - 1
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return j + 1, nil
			}
		case '{', '}', ';':
			return 0, errors.New("괄호 ( ) 가 닫히지 않음")
		}
	}
	return 0, errors.New("괄호 ( ) 가 닫히지 않음")
}

// splitDecls 는 선언 블록을 ; 로 가른다(글자열·괄호 안의 ; 는 빼고).
func splitDecls(body string) []string {
	var out []string
	start, depth := 0, 0
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '"', '\'':
			if j, err := skipString(body, i); err == nil {
				i = j - 1
			}
		case '(':
			depth++
		case ')':
			depth--
		case ';':
			if depth == 0 {
				if d := strings.TrimSpace(body[start:i]); d != "" {
					out = append(out, d)
				}
				start = i + 1
			}
		}
	}
	if d := strings.TrimSpace(body[start:]); d != "" {
		out = append(out, d)
	}
	return out
}

// stripCSSComments 는 주석을 빈칸 하나로 지운다(글자열 안의 /* 는 그대로 둔다).
func stripCSSComments(css string) string {
	var b strings.Builder
	for i := 0; i < len(css); i++ {
		c := css[i]
		if c == '"' || c == '\'' {
			j, err := skipString(css, i)
			if err != nil {
				b.WriteString(css[i:])
				break
			}
			b.WriteString(css[i:j])
			i = j - 1
			continue
		}
		if c == '/' && i+1 < len(css) && css[i+1] == '*' {
			end := strings.Index(css[i+2:], "*/")
			if end < 0 {
				break
			}
			i += end + 3
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// atName 은 @규칙 머리에서 이름(@ 뒤의 영숫자·-)을 뽑는다.
func atName(prelude string) string {
	s := strings.TrimPrefix(prelude, "@")
	for i, r := range s {
		if !(r == '-' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return s[:i]
		}
	}
	return s
}

func shorten(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 60 {
		return string(r[:60]) + "…"
	}
	return s
}

// ---- 창 틀이 쓰는 이름(호스트 앱 창 틀 CSS 에서 모은다 — 목록을 손으로 두면 창 틀을 고칠 때 어긋난다)

// ShellCSS 는 문서를 둘러싼 앱 창 틀의 CSS(호스트 앱이 넣는다)를 돌려준다. 거기서 쓰는 토큰·글꼴 이름은 사용자 테마가
// 쓰지 못한다. 기본은 창 틀이 없는 것(nil). 막는 이름은 reservedOnce 로 처음 한 번만 모으므로 앱은 첫 테마 검사 전에
// 정해야 한다 — 그 뒤에 바꾸면 듣지 않는다. 창 틀과 무관하게 문서 층(web/base.css)의 --font-* 글꼴 이름과 CSS 일반 이름,
// reservedTokenNames 는 늘 막는다(loadReserved).
var ShellCSS = func() []byte { return nil }

var (
	reservedOnce   sync.Once
	reservedTok    map[string]bool
	reservedFamily map[string]bool
)

func loadReserved() {
	reservedTok, reservedFamily = map[string]bool{}, map[string]bool{}
	b := ShellCSS()
	css := stripCSSComments(string(b))
	for _, m := range regexp.MustCompile(`(--[a-zA-Z0-9-]+)\s*:`).FindAllStringSubmatch(css, -1) {
		reservedTok[m[1]] = true
	}
	// 창 틀의 글꼴 이름(--ui-font·font-family). 테마 @font-face 가 같은 이름을 쓰면 창 틀 글꼴이 바뀐다.
	for _, m := range regexp.MustCompile(`(?:font-family|--ui-font)\s*:\s*([^;}]+)`).FindAllStringSubmatch(css, -1) {
		for f := range strings.SplitSeq(m[1], ",") {
			if f = strings.ToLower(strings.Trim(strings.TrimSpace(f), `"'`)); f != "" && !strings.HasPrefix(f, "var(") && f != "inherit" {
				reservedFamily[f] = true
			}
		}
	}
	// 문서 기본 글꼴(base.css 의 --font-* 값)과 CSS 의 일반 이름도 막는다 — 같은 이름으로 덮으면 내장 테마 모양이 몰래 바뀐다.
	// base.css 는 창 틀이 아니라 문서 층이라 ShellCSS 와 무관하게 늘 읽는다.
	base := web.BaseCSS
	for _, m := range regexp.MustCompile(`--font-[a-z0-9-]+\s*:\s*([^;}]+)`).FindAllStringSubmatch(stripCSSComments(string(base)), -1) {
		for f := range strings.SplitSeq(m[1], ",") {
			if f = strings.ToLower(strings.Trim(strings.TrimSpace(f), `"'`)); f != "" && !strings.HasPrefix(f, "var(") && f != "inherit" {
				reservedFamily[f] = true
			}
		}
	}
	for _, f := range []string{"pretendard", "serif", "sans-serif", "monospace", "cursive", "fantasy", "system-ui", "ui-serif",
		"ui-sans-serif", "ui-monospace", "ui-rounded", "math", "emoji", "fangsong"} {
		reservedFamily[f] = true
	}
}

func reservedTokens() map[string]bool {
	reservedOnce.Do(loadReserved)
	return reservedTok
}

func reservedFontFamilies() map[string]bool {
	reservedOnce.Do(loadReserved)
	return reservedFamily
}
