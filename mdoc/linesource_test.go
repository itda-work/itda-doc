package mdoc

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// 원문 줄 출처(itda-work/itda-hyve#1038 2단계 — 접근 방식 리뷰 권장안 A): data-line 은 렌더러가 계산한 원문 줄만이다. 사용자 속성 line=(키는 대소문자
// 무관)은 HTML 로 나가지 않는다(render.go userData) — 제목의 단계·ATX/setext·닫는 #·중첩 여부와 상관없다. 원문 위치가 있는 맨 위 블록만 data-line 을
// 한 번 낸다. 문서 창의 목차 이동·편집기에서 이 줄로·고쳐 달라기·보드 끌기가 이 값을 원문 줄로 쓴다.

var startTagRe = regexp.MustCompile(`<(h[1-6]|div)\b[^>]*>`)

// lineSource 는 렌더된 HTML 의 맨 위 제목마다 data-line 을 차례로, 중첩 제목에 붙은 data-line 을 따로 돌려준다.
// HTML 파서는 브라우저처럼 같은 이름 속성의 앞 것만 남긴다 — 중복 여부는 시작 태그 글자로 따로 센다.
func lineSource(t *testing.T, out string) (top []string, nested []string, dups []string) {
	t.Helper()
	for _, tag := range startTagRe.FindAllString(out, -1) {
		if strings.Count(tag, ` data-line="`) > 1 {
			dups = append(dups, tag)
		}
	}
	nodes, err := html.ParseFragment(strings.NewReader(out), &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body})
	if err != nil {
		t.Fatal(err)
	}
	isH := func(n *html.Node) bool {
		return n.Type == html.ElementNode && len(n.Data) == 2 && n.Data[0] == 'h' && n.Data[1] >= '1' && n.Data[1] <= '6'
	}
	get := func(n *html.Node) (string, bool) {
		for _, a := range n.Attr {
			if a.Key == "data-line" {
				return a.Val, true
			}
		}
		return "", false
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if isH(n) {
			if v, ok := get(n); ok {
				nested = append(nested, n.Data+"@"+v)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, n := range nodes {
		if isH(n) {
			v, _ := get(n)
			top = append(top, v)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	return top, nested, dups
}

func TestLineSource(t *testing.T) {
	const fm = "---\ntitle: 시험\n---\n\n" // 본문은 5줄부터
	cases := []struct {
		name string
		src  string
		top  []int    // 맨 위 제목의 원문 줄(차례대로)
		has  []string // HTML 에 그대로 있어야 하는 조각(line 이외 속성)
	}{
		{"ATX", "# 가 {line=9}\n\n글\n", []int{1}, []string{`id="가"`}},
		{"닫는 #", "글\n\n## 가 {line=9} ##\n\n## 나\n", []int{3, 5}, nil},
		{"setext =·-", "가 {line=9}\n===\n\n나 {LINE=1}\n---\n", []int{1, 4}, nil},
		{"대소문자·따옴표·반복", "## 가 {LINE=3}\n\n## 나 {Line=\"3\"}\n\n## 다 {line=1 line=2}\n", []int{1, 3, 5}, nil},
		{"문서 밖 값·숫자 아닌 값", "## 가 {line=999}\n\n## 나 {line=abc}\n\n## 다 {line=-1}\n", []int{1, 3, 5}, nil},
		{"line 이외 속성은 그대로", "## 가 {#g .c k=v line=3 lines=4 t=\"line=5\"}\n", []int{1},
			[]string{`<h2 id="g" class="c" data-k="v" data-lines="4" data-t="line=5" data-line="1">`}},
		{"frontmatter 뒤", fm + "## 가 {line=1}\n\n## 나\n", []int{5, 7}, nil},
		{"앞의 코드 펜스 안 ##", "```text\n## 펜스 {line=9}\n```\n\n## 가 {line=9}\n", []int{5}, nil},
		{"중첩 제목(인용·목록·상자)", "> ## 인용 {line=9}\n\n- ## 목록 {line=9}\n\n:::box\n## 상자 {line=9}\n:::\n\n## 가\n", []int{9}, nil},
		{"컨테이너 line=", "글\n\n::: {line=9 k=v}\n안\n:::\n", nil, []string{`<div data-k="v" data-line="3">`}},
		{"코드 src 파일 안 제목", "```text src=\"a.txt\"\n```\n\n## 가 {line=1}\n", []int{4}, nil},
	}
	opts := Options{ReadInclude: func(rel string) ([]byte, error) { return []byte("## 안 {line=1}\n"), nil }}
	for _, c := range cases {
		for _, crlf := range []bool{false, true} {
			name, src := c.name, c.src
			if crlf {
				name += " (CRLF)"
				src = strings.ReplaceAll(src, "\n", "\r\n")
			}
			r := render(t, src, opts)
			if len(r.Warnings) > 0 { // 사용자 line= 은 조용히 빠진다 — 새 경고가 없다
				t.Errorf("%s: 경고 %v", name, r.Warnings)
			}
			top, nested, dups := lineSource(t, r.HTML)
			want := make([]string, len(c.top))
			for i, l := range c.top {
				want[i] = fmt.Sprint(l)
			}
			if strings.Join(top, ",") != strings.Join(want, ",") {
				t.Errorf("%s: 맨 위 제목 data-line %v, 기대 %v\n%s", name, top, want, r.HTML)
			}
			if len(nested) > 0 {
				t.Errorf("%s: 중첩 제목에 data-line %v", name, nested)
			}
			if len(dups) > 0 {
				t.Errorf("%s: data-line 이 둘 이상 %v", name, dups)
			}
			for _, h := range c.has {
				if !strings.Contains(r.HTML, h) {
					t.Errorf("%s: %q 가 없음\n%s", name, h, r.HTML)
				}
			}
			if strings.Contains(r.HTML, `<h2 id="안"`) { // 불러온 글자는 코드 글자로만 — 제목이 되지 않는다
				t.Errorf("%s: 코드 src 파일 안 글자가 제목이 됨\n%s", name, r.HTML)
			}
		}
	}
}
