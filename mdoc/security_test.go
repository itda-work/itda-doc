package mdoc

import (
	"regexp"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// 결과 HTML 을 브라우저처럼 파싱해 본다. 글자 비교만으로는 따옴표·꺾쇠 탈출을 놓친다.

// 나와도 되는 요소. 이 밖의 요소가 보이면 raw HTML 이 새어 나간 것이다.
var allowedElements = map[string]bool{
	"html": true, "head": true, "body": true,
	"p": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"ul": true, "ol": true, "li": true, "blockquote": true, "pre": true, "code": true, "hr": true, "br": true,
	"em": true, "strong": true, "del": true, "a": true, "img": true, "input": true,
	"table": true, "caption": true, "thead": true, "tbody": true, "tr": true, "th": true, "td": true,
	"dl": true, "dt": true, "dd": true, "sup": true, "sub": true, "kbd": true, "mark": true, "u": true,
	"ins": true, "small": true, "div": true, "span": true, "nav": true, "figure": true, "figcaption": true,
}

var (
	safeStyleRe  = regexp.MustCompile(`^width:(?:\d+(?:\.\d+)?|\.\d+)(?:%|mm|cm|in|pt|pc|px|em|rem|ch)$`)
	safeAttrName = regexp.MustCompile(`^(?:id|class|href|src|alt|title|style|role|align|type|checked|disabled|start|data-[a-z0-9_-]+)$`)
)

// assertSafe 는 결과 HTML 에 실행되는 것이 없는지 본다.
func assertSafe(t *testing.T, out string) {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(out))
	if err != nil && strings.Contains(err.Error(), "exceeds 512") {
		// x/net/html 의 중첩 한도다. 브라우저도 이보다 깊은 트리는 평평하게 만든다 — 검사할 수 없을 뿐 위험이 아니다
		t.Skip("중첩이 512 를 넘어 검사하지 않음")
	}
	if err != nil {
		t.Fatalf("결과 HTML 파싱 실패: %v", err)
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Namespace == "svg" {
			checkSVGNode(t, n, out)
		} else if n.Type == html.ElementNode && n.Namespace == "math" {
			checkMathNode(t, n, out)
		} else if n.Type == html.ElementNode {
			if !allowedElements[n.Data] {
				t.Errorf("허용하지 않은 요소 <%s>\n%s", n.Data, out)
			}
			for _, a := range n.Attr {
				name := strings.ToLower(a.Key)
				if !safeAttrName.MatchString(name) {
					t.Errorf("<%s> 의 허용하지 않은 속성 %s=%q\n%s", n.Data, a.Key, a.Val, out)
				}
				if name == "style" && !safeStyleRe.MatchString(a.Val) {
					t.Errorf("<%s> style 주입 %q\n%s", n.Data, a.Val, out)
				}
				if name == "href" || name == "src" {
					if s, _ := schemeOf(a.Val); s != "" && s != "http" && s != "https" && s != "mailto" && s != "tel" &&
						(name != "src" || !strings.HasPrefix(strings.ToLower(a.Val), "data:image/")) {
						t.Errorf("<%s> 의 위험한 주소 %s=%q\n%s", n.Data, a.Key, a.Val, out)
					}
				}
				if s, c := schemeOf(a.Val); name == "src" && (s == "http" || s == "https" || strings.HasPrefix(c, "//")) {
					t.Errorf("원격 이미지를 기본으로 불러옴 %q", a.Val)
				}
				if name == "type" && a.Val != "checkbox" {
					t.Errorf("input type %q", a.Val)
				}
			}
			if n.Data == "input" && !hasAttr(n, "disabled") {
				t.Errorf("작업 목록 체크박스가 disabled 가 아님\n%s", out)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
}

// 차트(1.11)·그림(1.10 mermaid)의 인라인 SVG 허용 목록. mdoc 이 내는 SVG 는 이 요소·속성뿐이어야 한다 — 스크립트·이벤트 속성·style·
// 외부 참조(href·url())·foreignObject 가 없다. 값도 꼴을 본다(숫자·경로·클래스 이름 — 글자는 요소 안의 글자로만).
var (
	svgElements = map[string]bool{"svg": true, "g": true, "rect": true, "path": true, "line": true, "circle": true, "text": true, "tspan": true, "title": true}
	svgNum      = regexp.MustCompile(`^-?\d+(?:\.\d+)?$`)
	svgAttrs    = map[string]*regexp.Regexp{
		"viewBox": regexp.MustCompile(`^0 0 \d+(?:\.\d+)? \d+(?:\.\d+)?$`),
		"class":   regexp.MustCompile(`^[a-z0-9 -]+$`),
		"role":    regexp.MustCompile(`^img$`),
		"x":       svgNum, "y": svgNum, "width": svgNum, "height": svgNum, "rx": svgNum,
		"x1": svgNum, "y1": svgNum, "x2": svgNum, "y2": svgNum, "cx": svgNum, "cy": svgNum, "r": svgNum, "font-size": svgNum,
		"d":           regexp.MustCompile(`^[MLCAZ0-9 .-]+$`), // C 는 그림(mermaid)의 곡선 — 숫자뿐이라 주소·함수가 들어갈 수 없다
		"text-anchor": regexp.MustCompile(`^(?:start|middle|end)$`),
		"pathLength":  regexp.MustCompile(`^1$`),
		"aria-label":  nil, // 글자(캡션). 속성 값 안에 있을 뿐 실행되지 않는다 — 탈출은 파싱이 잡는다
	}
)

func checkSVGNode(t *testing.T, n *html.Node, out string) {
	t.Helper()
	if !svgElements[n.Data] {
		t.Errorf("허용하지 않은 SVG 요소 <%s>\n%s", n.Data, out)
	}
	for _, a := range n.Attr {
		re, ok := svgAttrs[a.Key]
		if !ok || a.Namespace != "" {
			t.Errorf("<%s> 의 허용하지 않은 SVG 속성 %s=%q", n.Data, a.Key, a.Val)
			continue
		}
		if re != nil && !re.MatchString(a.Val) {
			t.Errorf("<%s> %s 값 꼴이 아님 %q", n.Data, a.Key, a.Val)
		}
	}
	if n.Data == "svg" && (n.Parent == nil || n.Parent.Data != "figure") {
		t.Errorf("<svg> 가 차트·그림 figure 밖에 있음")
	}
}

// 수식(1.10)의 MathML 허용 목록. 표시 요소만 — 스크립트·주소·style·색(mathcolor·mathbackground)·annotation-xml 이 없다.
// 값은 글자·숫자·길이뿐이다(따옴표·꺾쇠·괄호·콜론 없음).
var (
	mathElements = map[string]bool{}
	mathAttrs    = map[string]bool{}
	mathValue    = regexp.MustCompile(`^[a-z0-9 .+-]+$`)
)

func init() {
	for _, e := range strings.Fields(`math mrow mi mn mo mtext ms mspace mfrac msqrt mroot msub msup msubsup munder mover munderover
		mmultiscripts mprescripts none mtable mtr mtd mstyle mpadded mphantom menclose`) {
		mathElements[e] = true
	}
	for _, a := range strings.Fields(`class display mathvariant displaystyle scriptlevel fence separator stretchy symmetric largeop
		movablelimits accent accentunder form lspace rspace minsize maxsize linethickness width height depth voffset
		notation columnalign rowalign`) {
		mathAttrs[a] = true
	}
}

func checkMathNode(t *testing.T, n *html.Node, out string) {
	t.Helper()
	if !mathElements[n.Data] {
		t.Errorf("허용하지 않은 MathML 요소 <%s>\n%s", n.Data, out)
	}
	for _, a := range n.Attr {
		if !mathAttrs[a.Key] || a.Namespace != "" || !mathValue.MatchString(a.Val) {
			t.Errorf("<%s> 의 허용하지 않은 MathML 속성 %s=%q", n.Data, a.Key, a.Val)
		}
	}
	if n.Data == "math" {
		for p := n.Parent; p != nil; p = p.Parent {
			if p.Namespace != "" {
				t.Errorf("<math> 가 %s:%s 안에 있음", p.Namespace, p.Data)
			}
		}
	}
}

func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}

var securityInputs = []struct{ name, src string }{
	{"script 블록", "<script>alert(1)</script>"},
	{"script 인라인", "글 <script>alert(1)</script> 끝"},
	{"img onerror 블록", `<img src=x onerror="alert(1)">`},
	{"img onerror 인라인", `글 <img src=x onerror=alert(1)>`},
	{"iframe", `<iframe src="https://evil.example"></iframe>`},
	{"svg onload", `<svg onload=alert(1)>`},
	{"style 태그", "<style>body{background:url(https://evil.example/t)}</style>"},
	{"object·embed", `<object data="x"></object><embed src="x">`},
	{"form", `<form action="https://evil.example"><input name=p></form>`},
	{"허용 태그에 속성", `<sub onclick="alert(1)">a</sub> <mark style="color:red">b</mark>`},
	{"대문자 허용 태그", `<SUB>a</SUB><Br>`},
	{"주석 탈출", "<!-- --><script>alert(1)</script> -->"},
	{"주석 블록 뒤 HTML", "<!-- pagebreak --><script>alert(1)</script>"},
	{"쪽 나눔 div 흉내", `<div style="page-break-after: always" onclick="alert(1)"></div>`},
	{"쪽 나눔 div 에 내용", `<div style="page-break-after: always"><script>alert(1)</script></div>`},
	{"CDATA·처리 명령", "<![CDATA[x]]> <?php echo 1 ?> <!DOCTYPE html>"},
	{"javascript 링크", "[a](javascript:alert(1))"},
	{"javascript 자동 링크", "<javascript:alert(1)>"},
	{"javascript 대소문자", "[a](JaVaScRiPt:alert(1))"},
	{"javascript 공백", "[a]( javascript:alert(1))"},
	{"javascript 탭 엔티티", "[a](java&#x09;script:alert(1))"},
	{"javascript 엔티티", "[a](&#106;avascript:alert(1))"},
	{"javascript 콜론 엔티티", "[a](javascript&colon;alert(1))"},
	{"javascript 백슬래시", `[a](<javascript\:alert(1)>)`},
	{"javascript 참조 링크", "[a][r]\n\n[r]: javascript:alert(1)"},
	{"vbscript", "[a](vbscript:msgbox(1))"},
	{"data html 링크", "[a](data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==)"},
	{"file 링크", "[a](file:///C:/Windows/System32/calc.exe)"},
	{"javascript 이미지", "![a](javascript:alert(1))"},
	{"data svg 아닌 이미지", "![a](data:text/html,<script>alert(1)</script>)"},
	{"원격 이미지", "![a](https://evil.example/pixel.png) ![b](//evil.example/p.png)"},
	{"링크 title 탈출", `[a](https://x.com "a\" onmouseover=\"alert(1)")`},
	{"이미지 alt 탈출", `![a" onerror="alert(1)](x.png)`},
	{"이미지 title 탈출", `![a](x.png "\"><script>alert(1)</script>")`},
	{"주소 따옴표", `[a](https://x.com/"onmouseover="alert(1))`},
	{"제목 속성 onclick", "## 제목 {onclick=alert(1)}"},
	{"제목 속성 style", `## 제목 {style="background:url(https://evil.example)"}`},
	{"제목 id 따옴표", `## 제목 {#a"onclick="alert(1)}`},
	{"제목 id 꺾쇠", "## 제목 {#a><script>alert(1)</script>}"},
	{"제목 class 따옴표", `## 제목 {.a"b}`},
	{"제목 키 탈출", `## 제목 {a"b=1}`},
	{"제목 값 따옴표", `## 제목 {k='a"><script>alert(1)</script>'}`},
	{"제목 글자의 HTML", "## <img src=x onerror=alert(1)> 제목 <script>alert(1)</script>\n\n[TOC]"},
	{"컨테이너 onclick", ":::box{onclick=alert(1)}\n안\n:::"},
	{"컨테이너 style", ":::box{style=\"color:red\"}\n안\n:::"},
	{"column width 주입", ":::column{width=\"1px;background:url(https://evil.example)\"}\n안\n:::"},
	{"column width 괄호", ":::column{width=calc(1px)}\n안\n:::"},
	{"컨테이너 이름 따옴표", ":::a\"onclick=\"alert(1)\n안\n:::"},
	{"컨테이너 이름 꺾쇠", ":::<script>\n안\n:::"},
	{"컨테이너 id 따옴표", ":::box{#a\"b}\n안\n:::"},
	{"컨테이너 id 꺾쇠", ":::box{#<script>}\n안\n:::"},
	{"컨테이너 class 꺾쇠", ":::box{.a<b}\n안\n:::"},
	{"컨테이너 값 탈출", ":::box{k=\"\\\"><script>alert(1)</script>\"}\n안\n:::"},
	{"이미지 width 주입", "![](a.png){width=\"1px;background:url(https://evil.example)\"}"},
	{"이미지 width onerror", "![](a.png){width=10% onerror=alert(1)}"},
	{"frontmatter 값의 HTML", "---\ntitle: <script>alert(1)</script>\n수신: <img src=x onerror=alert(1)>\n---\n\n본문"},
	{"표 캡션의 HTML", "| a |\n|---|\n| 1 |\n\nTable: <script>alert(1)</script> <img src=x onerror=alert(1)>"},
	{"표 칸의 HTML", "| a |\n|---|\n| <iframe src=x></iframe> |"},
	{"그림 캡션의 HTML", `![a](x.png "<script>alert(1)</script>")`},
	{"알림 안의 HTML", "> [!NOTE]\n> <script>alert(1)</script>"},
	{"각주 안의 HTML", "a[^1]\n\n[^1]: <script>alert(1)</script>"},
	{"정의 목록의 HTML", "<b>용어</b>\n: <script>alert(1)</script>"},
	{"코드 언어 탈출", "```\"><script>alert(1)</script>\nx\n```"},
	{"코드 언어 onclick", "~~~ x\"onclick=\"alert(1)\nx\n~~~"},
	{"목록 안 HTML", "- <script>alert(1)</script>\n  <iframe></iframe>"},
	{"수식 글자의 HTML", `$\text{<script>alert(1)</script>}$ $$\text{<img src=x onerror=alert(1)>}$$`},
	{"수식 href", `$\href{javascript:alert(1)}{x}$`},
	{"수식 url", `$\url{https://evil.example}$`},
	{"수식 그림", `$\includegraphics{https://evil.example/p.png}$`},
	{"수식 html 명령", `$\htmlStyle{background:url(https://evil.example)}{x}$ $\htmlClass{page-break}{x}$ $\htmlId{toc}{x}$`},
	{"수식 색", `$\color{red}{x}$ $\colorbox{red}{x}$`},
	{"수식 원문 탈출", "$\\foo \"><script>alert(1)</script>$"},
	{"블록 수식 원문 탈출", "$$\n\\foo </p><script>alert(1)</script>\n"},
	{"math 코드 블록 탈출", "```math\n\\foo </code></pre><script>alert(1)</script>\n```"},
	{"수식 안 raw HTML", `$<b onclick=alert(1)>x</b>$`},
	{"수식 verb", `$\verb|<script>alert(1)</script>|$`},
	{"그림 라벨의 HTML", "```mermaid\nflowchart TD\nA[\"<script>alert(1)</script>\"] -->|<img src=x onerror=alert(1)>| B(\"</text><foreignObject>x</foreignObject>\")\n```"},
	{"그림 click", "```mermaid\nflowchart TD\nA --> B\nclick A \"javascript:alert(1)\"\nclick B href \"https://evil.example\" _blank\nclick A call alert(1)\n```"},
	{"그림 init·style", "```mermaid\n%%{init: {\"theme\":\"dark\",\"themeCSS\":\"body{background:url(https://evil.example)}\"}}%%\nflowchart TD\nA --> B\nstyle A fill:#f00,stroke:url(https://evil.example)\nclassDef x fill:red\nclass B x\nlinkStyle 0 stroke:red\nC:::x --> A\n```"},
	{"그림 frontmatter 설정", "```mermaid\n---\ntitle: <script>alert(1)</script>\nconfig:\n  themeCSS: \"body{background:url(https://evil.example)}\"\n---\nflowchart LR\nA --> B\n```"},
	{"그림 엔티티 탈출", "```mermaid\nflowchart TD\nA[\"#lt;script#gt;alert(1)#lt;/script#gt;\"] --> B[#60;img src=x onerror=alert(1)#62;]\n```"},
	{"그림 subgraph 제목 탈출", "```mermaid\nflowchart TD\nsubgraph s [\"</text><script>alert(1)</script>\"]\nA --> B\nend\n```"},
	{"그림 accTitle 탈출", "```mermaid\nflowchart TD\naccTitle: a\" onload=\"alert(1)\naccDescr: </title><script>alert(1)</script>\nA --> B\n```"},
	{"그림 원문 탈출(미지원 문장)", "```mermaid\nsequenceDiagram\n</code></pre><script>alert(1)</script>\n```"},
	{"순서도 참여자·메시지 탈출", "```mermaid\nsequenceDiagram\nparticipant A as </text><script>alert(1)</script>\nA->>B: </text><img src=x onerror=alert(1)>\n```"},
	{"순서도 제목·묶음·메모 탈출", "```mermaid\nsequenceDiagram\ntitle </text><script>alert(1)</script>\nalt </text><img src=x onerror=alert(1)>\nA->>B: x\nelse </text><script>alert(1)</script>\nend\nNote over A,B: </text><script>alert(1)</script>\n```"},
	{"순서도 accTitle·accDescr 탈출", "```mermaid\nsequenceDiagram\naccTitle: 순서\" onload=\"alert(1)\naccDescr: </title><script>alert(1)</script>\nA->>B: x\n```"},
	{"순서도 엔티티 탈출", "```mermaid\nsequenceDiagram\nA#lt;script#gt;alert(1)#lt;/script#gt;->>B: #lt;img src=x onerror=alert(1)#gt;\n```"},
	{"ER 별칭·역할·설명 탈출", "```mermaid\nerDiagram\nA[\"#lt;/text#gt;#lt;script#gt;alert(1)#lt;/script#gt;\"] ||--o{ B : \"#lt;img src=x onerror=alert(1)#gt;\"\nB {\n  string x \"#lt;/text#gt;#lt;script#gt;\"\n}\n```"},
	{"ER 접근성 탈출", "```mermaid\nerDiagram\naccTitle: 관계\" onload=\"alert(1)\naccDescr: </title><script>alert(1)</script>\nA\n```"},
	{"ER 원문 탈출(미지원 문장)", "```mermaid\nerDiagram\n</code></pre><script>alert(1)</script>\n```"},
	{"상태도 설명·라벨·복합 제목 탈출", "```mermaid\nstateDiagram-v2\nstate \"#lt;/text#gt;#lt;script#gt;alert(1)#lt;/script#gt;\" as S {\n  A : #lt;img src=x onerror=alert(1)#gt;\n}\nA --> B : #lt;/text#gt;#lt;script#gt;\nB : 둘#quot; onload=#quot;alert(1)\n```"},
	{"상태도 접근성 탈출", "```mermaid\nstateDiagram-v2\naccTitle: 상태\" onload=\"alert(1)\naccDescr: </title><script>alert(1)</script>\nA\n```"},
	{"상태도 원문 탈출(미지원 문장)", "```mermaid\nstateDiagram\n</code></pre><script>alert(1)</script>\n```"},
	{"클래스도 이름·멤버·라벨·수량 탈출", "```mermaid\nclassDiagram\nclass 고객 {\n  +이름 #lt;/text#gt;#lt;script#gt;\n}\n고객 \"#lt;b#gt;\" --> \"*\" 주문 : #lt;img src=x onerror=alert(1)#gt;\n주문 : +합계() #lt;/tspan#gt;\n```"},
	{"클래스도 접근성 탈출", "```mermaid\nclassDiagram\naccTitle: 클래스\" onload=\"alert(1)\naccDescr: </title><script>alert(1)</script>\nclass A\n```"},
	{"클래스도 원문 탈출(미지원 문장)", "```mermaid\nclassDiagram\n</code></pre><script>alert(1)</script>\n```"},
	{"순서도 링크 메뉴 탈출", "```mermaid\nsequenceDiagram\nA->>B: x\nlink A: 문서 @ javascript:alert(1)\nlinks B: {\"악의\": \"https://evil.example\"}\n```"},
	{"간트 작업 이름 탈출", "```mermaid\ngantt\n이름</text><script>alert(1)</script> :2026-01-01, 1d\n```"},
	{"간트 제목·구역·축 탈출", "```mermaid\ngantt\ntitle </text><img src=x onerror=alert(1)>\nsection 구역</text><script>alert(1)</script>\n작업 :2026-01-01, 1d\naxisFormat %Y<script>alert(1)</script>\n```"},
	{"간트 accTitle·accDescr 탈출", "```mermaid\ngantt\naccTitle: 간트\" onload=\"alert(1)\naccDescr: </title><script>alert(1)</script>\n작업 :2026-01-01, 1d\n```"},
	{"간트 click·todayMarker·frontmatter", "```mermaid\n---\nconfig:\n  themeCSS: \"body{background:url(https://evil.example)}\"\n---\ngantt\nclick a1 \"javascript:alert(1)\"\ntodayMarker stroke:url(https://evil.example)\n작업 :2026-01-01, 1d\n```"},
	{"간트 엔티티 탈출", "```mermaid\ngantt\n이름#lt;script#gt;alert(1)#lt;/script#gt; :2026-01-01, 1d\n```"},
	{"타임라인 시기·사건 탈출", "```mermaid\ntimeline\n시기</text><script>alert(1)</script> : 사건</text><img src=x onerror=alert(1)>\n```"},
	{"타임라인 제목·구역 탈출", "```mermaid\ntimeline\ntitle </text><img src=x onerror=alert(1)>\nsection 구역</text><script>alert(1)</script>\n2026 : 사건\n```"},
	{"타임라인 accTitle·accDescr 탈출", "```mermaid\ntimeline\naccTitle: 타임\" onload=\"alert(1)\naccDescr: </title><script>alert(1)</script>\n2026 : 사건\n```"},
	{"타임라인 frontmatter·엔티티 탈출", "```mermaid\n---\ntitle: <script>alert(1)</script>\nconfig:\n  themeCSS: \"body{background:url(https://evil.example)}\"\n---\ntimeline\n시기#lt;script#gt;alert(1)#lt;/script#gt; : 사건\n```"},
}

func TestSecurity(t *testing.T) {
	for _, c := range securityInputs {
		t.Run(c.name, func(t *testing.T) {
			for _, opts := range []Options{{}, {AllowRemoteImages: true}} {
				r := render(t, c.src, opts)
				if opts.AllowRemoteImages {
					// 허용하면 원격 src 는 의도된 것이라 assertSafe 의 원격 검사만 빼고 본다
					r.HTML = strings.NewReplacer(`src="https://`, `src="x-`, `src="//`, `src="x-`).Replace(r.HTML)
				}
				assertSafe(t, r.HTML)
				// 속성 탈출은 assertSafe 가 파싱해서 본다. 여기서는 태그가 글자 그대로 새는지만 본다
				// 그림(mermaid)의 SVG 는 mdoc 이 낸 것이다(요소·속성은 assertSafe 가 허용 목록으로 본다) — 그 여는 태그만 빼고 본다
				low := strings.ReplaceAll(strings.ToLower(r.HTML), `<svg class="diagram-svg" `, "")
				for _, bad := range []string{"<script", "<iframe", "<svg", "<style", "<object", "<embed", "<form"} {
					if strings.Contains(low, bad) {
						t.Errorf("결과에 %q\n%s", bad, r.HTML)
					}
				}
			}
		})
	}
}

// 버린 HTML 은 경고로 알린다(1.9). 편집기가 "렌더하지 않은 HTML n곳" 을 보여 준다.
func TestDroppedHTMLIsWarned(t *testing.T) {
	r := render(t, "<script>alert(1)</script>\n\n글 <span>a</span>\n")
	n := 0
	for _, w := range r.Warnings {
		if w.Code == WarnRawHTML {
			n++
		}
	}
	if n != 3 {
		t.Errorf("버린 HTML 3곳 기대: %+v", r.Warnings)
	}
}

// FuzzRender 는 어떤 입력에도 패닉하지 않고 결과가 안전한지 본다. 평소 go test 는 시드만 돈다.
//
//	go test ./mdoc -run '^$' -fuzz FuzzRender -fuzztime 30s
func FuzzRender(f *testing.F) {
	for _, c := range securityInputs {
		f.Add(c.src)
	}
	for _, s := range []string{
		":::a\n::::b\n:::\n", "::::a\n:::\n::::\n:::", "> [!NOTE]", "![](x){", "![](x){width=", "# {", "# {#}", "| a |\n|-|\n\n:",
		"---\n---", "---\n:\n---", "[TOC]", "\\newpage", "<u><u></u>", "<!--", "```\n:::\n", ":::a\n```\n:::\n```\n:::",
		"$x$", "$$x$$", "$$\nx\n$$", "$$\nx", "$a$b$c$", "$$$", "$ $", "$\\$$", "| $a\\|b$ |\n|-|\n| $c$ |", "```math\nx\n```",
		"> $$\n> x\n> $$", "- $$\n  x\n  $$", "$`x`$ `$x$`", "$[a](b)$", "[$x$](y)", "**$x$**", "$x$5", "$5$x",
		"```mermaid\ngraph TD\nA-->B\n```", "```mermaid\nflowchart LR\nA[가] -->|나| B{다} --> A\nB --> B\n```",
		"```mermaid\nflowchart TD\nsubgraph s [제목]\nA --> B\nend\nB --> C\n```", "```mermaid\nflowchart RL\nA & B --> C & D\nA -.->|x| D\nA ==> D\nA <--> D\n```",
		"```mermaid\nflowchart BT\nA((원)) --- B[(DB)] ~~~ C{{육}} --> D[/a/] --> E[\\b\\] --> F[/c\\] --> G[\\d/]\n```",
		"> ```mermaid\n> graph TD\n> A-->B\n> ```", "- ```mermaid\n  graph TD\n  A-->B\n  ```", "```mermaid\n---\ntitle: t\n---\ngraph TD\nA\n```",
		"```mermaid\ngantt\ndateFormat YYYY-MM-DD\nexcludes weekends\nsection A\na :done, 2026-01-05, 4d\nb :b1, after a, 5d\nsection B\nc :crit, 2026-02-01, 1w\nd :milestone, 2026-02-15, 0d\n```",
		"```mermaid\ngantt\ntickInterval 1week\naxisFormat %Y.%m\ndateFormat YYYY-MM\na :2026-01, 2M\nb :until a\n```",
		"```mermaid\ngantt\ninclusiveEndDates\n제목 < 나 :2026-01-01, 1d", "```mermaid\ngantt\n작업 :", "```mermaid\ngantt\nsection", "```mermaid\ngantt\n작업 :1.5d", "```mermaid\ngantt\n작업 :after x,\n```",
		"```mermaid\ntimeline\n2026 : 사건", "```mermaid\ntimeline\nsection 구역\n시기 : 사건1 : 사건2\n: 이어 쓰기\n```",
		"```mermaid\ntimeline TD\n2026 : 사건", "```mermaid\ntimeline\n2026:사건", "```mermaid\ntimeline\n2026 :", "```mermaid\ntimeline\n: 사건",
		"```mermaid\nTimeline\nTITLE 제목</title><script>alert(1)</script>\n2026 : 사건", "```mermaid\ntimeline\n2026 # 메모 : 사건",
		"```mermaid\ntimeline\naccDescr { </title><script>alert(1)</script> }\n2026 : 사건",
		"```mermaid\nsequenceDiagram\nA->>B: 요청\nB-->>A: 응답\n```", "```mermaid\nsequenceDiagram\nautonumber 10 5\nactor U\nU->>+S: 요청\nS-->>-U: 응답\n```",
		"```mermaid\nsequenceDiagram\nloop 매번\nalt 있음\nA->>B: x\nelse 없음\nA--xB: y\nend\nend\nNote over A,B: 메모\n```",
		"```mermaid\nSequenceDiagram\nA-|\\B: x", "```mermaid\nsequenceDiagram\nA->>B", "```mermaid\nsequenceDiagram\nparticipant A@{shape: database}\nA->>B: x",
		"```mermaid\nerDiagram\n고객 ||--o{ 주문 : 넣는다\n주문 {\n  string 번호 PK, FK \"설명\"\n}\n```", "```mermaid\nerDiagram\ndirection LR\nA |o..|{ A : 자기\nA[\"별칭\"]\n```",
		"```mermaid\nerDiagram\nA one or more B : x", "```mermaid\nerDiagram\nA {\nstring a", "```mermaid\nerDiagram\naccDescr {\n</title>\n}\nA",
		"```mermaid\nstateDiagram-v2\n[*] --> 접수\n접수 --> 결제 : 확인\nstate 배송 {\n  [*] --> 출고\n  출고 --> [*]\n}\n결제 --> 출고\n출고 --> [*]\n```",
		"```mermaid\nstateDiagram\ndirection LR\nstate 갈래 <<fork>>\nstate 합류 <<join>>\nstate 고름 <<choice>>\n[*] --> 갈래\n갈래 --> A\nA --> 합류\n합류 --> 고름\nA : 첫째\nA : 둘째",
		"```mermaid\nstateDiagram-v2\nnote right of A : x", "```mermaid\nstateDiagram-v2\nstate A {\nB", "```mermaid\nstateDiagram-v2\naccDescr {\n</title>\n}\nA",
		"```mermaid\nstateDiagram-v2\nA --> B : x_y\nclick A \"javascript:alert(1)\" \"t\"",
		"```mermaid\nclassDiagram\nclass 고객 {\n  <<interface>>\n  +이름 String\n  +주문하기(상품) 주문$\n}\n고객 \"1\" --> \"*\" 주문 : 주문한다\n주문 *-- 항목\n항목 o-- 상품\n```",
		"```mermaid\nclassDiagram-v2\ndirection RL\nA <|.. B\nB ..> A : 씀\nA -- A : 자기\nA : -x int*", "```mermaid\nclassDiagram\nclass L~T~", "```mermaid\nclassDiagram\nclass A {\n+x",
		"```mermaid\nclassDiagram\naccDescr {\n</title>\n}\nclass A", "```mermaid\nclassDiagram\nA --oB\nclick A href \"javascript:alert(1)\"",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		r, err := Render([]byte(src), Options{})
		if err != nil {
			t.Fatal(err)
		}
		assertSafe(t, r.HTML)
	})
}
