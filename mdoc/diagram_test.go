package mdoc

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/itda-work/itda-doc/internal/timingtest"
)

// 1.10 그림(mermaid 흐름도) — 문서 안의 자리: figure·캡션·data-line, 그리지 못하면 코드 블록 그대로와 줄 번호 경고.
func TestDiagramBlock(t *testing.T) {
	runCases(t, []syntaxCase{
		{name: "그림", src: "앞\n\n```mermaid\nflowchart LR\n  A[시작] --> B[끝]\n```\n\n뒤",
			want: []string{"<p data-line=\"1\">앞</p>\n<figure class=\"diagram\" data-line=\"3\">\n<svg class=\"diagram-svg\"", `aria-label="흐름도"`, ">시작</text>", ">끝</text>",
				"</svg>\n</figure>\n<p data-line=\"8\">뒤</p>"},
			not: []string{"<pre", "language-mermaid"}},
		{name: "frontmatter title 은 캡션", src: "```mermaid\n---\ntitle: 결재 <흐름>\n---\ngraph TD\nA-->B\n```",
			want: []string{`aria-label="결재 &lt;흐름&gt;"`, "</svg>\n<figcaption>결재 &lt;흐름&gt;</figcaption>\n</figure>"}},
		{name: "~~~ 울타리", src: "~~~mermaid\ngraph TD\nA-->B\n~~~", want: []string{`<figure class="diagram"`}},
		{name: "인용 안", src: "> ```mermaid\n> graph TD\n> A-->B\n> ```", want: []string{"<blockquote data-line=\"1\"><figure class=\"diagram\">"}},
		{name: "목록 안", src: "- 항목\n\n  ```mermaid\n  graph TD\n  A-->B\n  ```", want: []string{"<li>", `<figure class="diagram">`}},
		{name: "언어 이름이 다르면 코드", src: "```Mermaid\ngraph TD\nA-->B\n```\n\n```mermaid-js\ngraph TD\n```",
			want: []string{`<code class="language-Mermaid">`, `<code class="language-mermaid-js">`}, not: []string{"<svg"}},
		{name: "다른 그림 종류는 코드 그대로", src: "```mermaid\npie\n  \"a\" : 1\n```",
			want: []string{"<pre data-line=\"1\"><code class=\"language-mermaid\">pie\n  &quot;a&quot; : 1\n</code></pre>"}, warn: WarnDiagram},
		{name: "쓰지 않은 줄이 있어도 그림", src: "```mermaid\ngraph TD\nA-->B\nstyle A fill:#f00\n```", want: []string{"<svg"}, warn: WarnDiagram},
		{name: "간트 차트", src: "앞\n\n```mermaid\ngantt\ndateFormat YYYY-MM-DD\nsection 준비\n조사 :done, 2026-01-05, 4d\n계획 :2026-01-12, 5d\n```\n\n뒤",
			want: []string{"<figure class=\"diagram\" data-line=\"3\">\n<svg class=\"diagram-svg\"", `aria-label="간트 차트"`, ">조사</text>", ">준비<", ">2026-01-11</text>",
				"</svg>\n</figure>\n<p data-line=\"11\">뒤</p>"},
			not: []string{"<pre", "language-mermaid"}},
		{name: "간트 frontmatter 캡션·무시 줄", src: "```mermaid\n---\ntitle: 출시 일정\n---\ngantt\ntodayMarker stroke:red\n작업 :2026-01-01, 5d\n```",
			want: []string{"</svg>\n<figcaption>출시 일정</figcaption>"}, warn: WarnDiagram},
		{name: "타임라인", src: "앞\n\n```mermaid\ntimeline\nsection 창업기\n2016 : 설립\n2018 : 투자\nsection 성장기\n2020 : 매출 100억\n```\n\n뒤",
			want: []string{"<figure class=\"diagram\" data-line=\"3\">\n<svg class=\"diagram-svg\"", `aria-label="타임라인"`, ">2016<", ">설립<", ">창업기<",
				"</svg>\n</figure>\n<p data-line=\"12\">뒤</p>"},
			not: []string{"<pre", "language-mermaid"}},
		{name: "타임라인 이어 쓰기·사건 여럿", src: "```mermaid\ntimeline\n2026\n: 사건1\n2027 : 사건2 : 사건3\n```",
			want: []string{">사건1<", ">사건2<", ">사건3<"}},
		{name: "타임라인 frontmatter 캡션·무시 줄", src: "```mermaid\n---\ntitle: 연혁\n---\n%%{init: {\"theme\":\"dark\"}}%%\ntimeline\n2026 : 사건\n```",
			want: []string{"</svg>\n<figcaption>연혁</figcaption>"}, warn: WarnDiagram},
		{name: "타임라인 대소문자(머리·지시자 — 리뷰 [3])", src: "```mermaid\nTimeline\nTITLE 연혁\nSECTION 성장기\n2016 : 설립\n```",
			want: []string{`aria-label="연혁"`, ">연혁<", ">성장기<", ">설립<", ">2016<"},
			not:  []string{"<pre", "language-mermaid"}},
		{name: "타임라인 시기 엔티티(리뷰 2회차 [1] — 원본 전처리 재현)", src: "```mermaid\ntimeline\n시기#lt;script#gt;alert(1)#lt;/script#gt; : 사건\n```",
			want: []string{"<svg", "&lt;script&gt;"},
			not:  []string{"<pre", "<script"}},
		{name: "순서도", src: "앞\n\n```mermaid\nsequenceDiagram\nactor U as 고객\nparticipant S as 상점\nU->>+S: 주문 요청\nS-->>-U: 주문 확인\n```\n\n뒤",
			want: []string{"<figure class=\"diagram\" data-line=\"3\">\n<svg class=\"diagram-svg\"", `aria-label="순서도"`, ">고객<", ">상점<", ">주문 요청<", ">주문 확인<", `class="sq-act"`,
				"</svg>\n</figure>\n<p data-line=\"11\">뒤</p>"},
			not: []string{"<pre", "language-mermaid"}},
		{name: "순서도 묶음·메모·번호", src: "```mermaid\nsequenceDiagram\nautonumber\nloop 재고 확인\nU->>S: 조회\nNote over U,S: 확인 중\nend\n```",
			want: []string{">loop<", ">[재고 확인]<", ">조회<", ">확인 중<", `class="sq-num"`, ">1<"}},
		{name: "순서도 frontmatter 캡션·무시 줄", src: "```mermaid\n---\ntitle: 주문 흐름\n---\n%%{init: {\"theme\":\"dark\"}}%%\nsequenceDiagram\nU->>S: 주문\n```",
			want: []string{"</svg>\n<figcaption>주문 흐름</figcaption>"}, warn: WarnDiagram},
		{name: "순서도 대소문자(머리·지시자 — 원본 lexer case-insensitive)", src: "```mermaid\nSequenceDiagram\nActor U\nLoop 매번\nU->>U: 반복\nEnd\n```",
			want: []string{`aria-label="순서도"`, ">U<", ">loop<", ">[매번]<", ">반복<"},
			not:  []string{"<pre", "language-mermaid"}},
		{name: "ER", src: "앞\n\n```mermaid\nerDiagram\n고객 ||--o{ 주문 : 넣는다\n주문 {\n  string 번호 PK\n  int 금액 \"원 단위\"\n}\n```\n\n뒤",
			want: []string{"<figure class=\"diagram\" data-line=\"3\">\n<svg class=\"diagram-svg\"", `aria-label="ER 다이어그램"`, ">고객<", ">주문<", ">넣는다<",
				">번호<", ">PK<", ">원 단위<", `class="er-mk"`, `class="er-mko"`, "</svg>\n</figure>\n<p data-line=\"12\">뒤</p>"},
			not: []string{"<pre", "language-mermaid"}},
		{name: "ER frontmatter 캡션·별칭·비식별", src: "```mermaid\n---\ntitle: 주문 모델\n---\nerDiagram\ndirection LR\nC[\"고객 정보\"]\nC |o..|{ O : 가진다\n```",
			want: []string{"</svg>\n<figcaption>주문 모델</figcaption>", ">고객 정보<", `class="er-rel er-dash"`}},
		{name: "상태도", src: "앞\n\n```mermaid\nstateDiagram-v2\n[*] --> 접수\n접수 --> 완료 : 확인\n완료 --> [*]\n```\n\n뒤",
			want: []string{"<figure class=\"diagram\" data-line=\"3\">\n<svg class=\"diagram-svg\"", `aria-label="상태도"`, ">접수<", ">완료<", ">확인<",
				`class="st-start"`, `class="st-end"`, `class="st-tr"`, "</svg>\n</figure>\n<p data-line=\"10\">뒤</p>"},
			not: []string{"<pre", "language-mermaid"}},
		{name: "상태도 frontmatter 캡션·복합·특수 상태", src: "```mermaid\n---\ntitle: 주문 상태\n---\nstateDiagram\ndirection LR\nstate \"배송 중\" as 배송 {\n  [*] --> 출고\n}\nstate 갈림 <<choice>>\n출고 --> 갈림\n```",
			want: []string{"</svg>\n<figcaption>주문 상태</figcaption>", ">배송 중<", `class="st-cl"`, `class="st-choice"`}},
		{name: "클래스도", src: "앞\n\n```mermaid\nclassDiagram\nclass 고객 {\n  +이름 String\n  +주문하기() 주문\n}\n고객 \"1\" --> \"*\" 주문 : 주문한다\n```\n\n뒤",
			want: []string{"<figure class=\"diagram\" data-line=\"3\">\n<svg class=\"diagram-svg\"", `aria-label="클래스도"`, ">고객<", ">+이름 String<",
				">+주문하기() : 주문<", ">주문한다<", `class="cls-card"`, `class="cls-dep"`, "</svg>\n</figure>\n<p data-line=\"12\">뒤</p>"},
			not: []string{"<pre", "language-mermaid"}},
		{name: "클래스도 v2 frontmatter 캡션·주석·상속", src: "```mermaid\n---\ntitle: 결제 모델\n---\nclassDiagram-v2\ndirection LR\nclass 결제수단 {\n<<interface>>\n}\n결제수단 <|.. 카드결제\n```",
			want: []string{"</svg>\n<figcaption>결제 모델</figcaption>", ">«interface»<", `class="cls-rel cls-dot"`, `class="cls-ext"`}},
	})
}

// 경고는 원문 줄을 가리킨다 — 그리지 못한 까닭의 줄(인용·목록 안에서도), 쓰지 않은 줄마다.
func TestDiagramWarnLines(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string // "줄:메시지 일부"
		drawn     bool
	}{
		{"미지원 줄", "# 제목\n\n```mermaid\nflowchart TD\n  A --> B\n  A --o C\n```", []string{"6:끝이 o·x"}, false},
		{"그림 종류", "```mermaid\npie\n```", []string{"2:pie 그림은 아직"}, false},
		{"빈 블록", "본문\n\n```mermaid\n```", []string{"3:비어 있음"}, false},
		{"인용 안", "> 인용\n>\n> ```mermaid\n> graph TD\n> A --> end\n> ```", []string{"5:노드 이름으로"}, false},
		{"목록 안", "- 가\n- 나\n\n  ```mermaid\n  graph TD\n  subgraph a\n  subgraph b\n  ```", []string{"7:subgraph 안의 subgraph"}, false},
		{"닫히지 않은 subgraph", "```mermaid\ngraph TD\nA\nsubgraph s\nB\n```", []string{"4:end 로 닫히지"}, false},
		{"쓰지 않은 줄", "```mermaid\n%%{init: {}}%%\ngraph LR\nA --> B\nclick A \"https://x.example\"\nstyle B fill:red\n```",
			[]string{"2:설정 지시", "5:click", "6:style"}, true},
		{"깨진 frontmatter YAML(리뷰 R1)", "```mermaid\n---\ntitle: [가\n---\ngraph TD\nA\n```", []string{"2:YAML 을 읽지 못함"}, false},
		{"글이 아닌 title", "```mermaid\n---\nconfig: {}\ntitle:\n  a: b\n---\ngraph TD\nA\n```", []string{"4:title 이 글이 아님"}, false},
		{"원문 상한", "```mermaid\ngraph TD\nA\n%%" + strings.Repeat("x", 17000) + "\n```", []string{"1:16KB"}, false},
		{"간트 미지원(vert)", "```mermaid\ngantt\n표식 :vert, 2026-01-01, 1d\n```", []string{"3:vert"}, false},
		{"간트 쓰지 않는 줄", "```mermaid\ngantt\ntopAxis\n작업 :2026-01-01, 1d\n```", []string{"3:topAxis"}, true},
		{"간트 인용 안", "> ```mermaid\n> gantt\n> 잘못 :2026-01-01\n> ```", []string{"3:첫 작업의 시작"}, false},
		{"타임라인 미지원(세로)", "```mermaid\ntimeline TD\n2026 : 사건\n```", []string{"2:세로 방향"}, false},
		{"타임라인 쓰지 않는 줄", "```mermaid\ntimeline\n%%{init: {\"theme\":\"dark\"}}%%\n2026 : 사건\n```", []string{"3:설정 지시"}, true},
		{"타임라인 인용 안", "> ```mermaid\n> timeline\n> 2026:사건\n> ```", []string{"3:콜론 뒤에 공백"}, false},
		{"순서도 미지원(반쪽 화살표)", "```mermaid\nsequenceDiagram\nA-|\\B: x\n```", []string{"3:반쪽 화살표"}, false},
		{"순서도 쓰지 않는 줄", "```mermaid\nsequenceDiagram\nlink A: 문서 @ https://x.example\nA->>B: 주문\n```", []string{"3:링크 메뉴"}, true},
		{"순서도 인용 안", "> ```mermaid\n> sequenceDiagram\n> A->>B\n> ```", []string{"3:콜론"}, false},
		{"ER 미지원(낱말 기수)", "```mermaid\nerDiagram\nA ||--o{ B : x\nA one or more to many B : y\n```", []string{"4:낱말 기수"}, false},
		{"ER 쓰지 않는 줄", "```mermaid\nerDiagram\nA ||--o{ B : x\nstyle A fill:#f9f\n```", []string{"4:style"}, true},
		{"ER 블록 안 줄", "- 목록\n\n  ```mermaid\n  erDiagram\n  A {\n    string a pk\n  }\n  ```", []string{"6:속성은"}, false},
		{"상태도 미지원(메모)", "```mermaid\nstateDiagram-v2\nA --> B\nnote right of A : 메모\n```", []string{"4:note"}, false},
		{"상태도 쓰지 않는 줄", "```mermaid\nstateDiagram-v2\nA --> B\nclassDef bad fill:#f00\n```", []string{"4:classDef"}, true},
		{"상태도 복합 상태 안 줄", "- 목록\n\n  ```mermaid\n  stateDiagram-v2\n  state 배송 {\n    direction LR\n    A\n  }\n  ```", []string{"6:direction"}, false},
		{"클래스도 미지원(형제 제네릭)", "```mermaid\nclassDiagram\nA <|-- B\nA : +쌍 a~T~ b~U~\n```", []string{"4:제네릭"}, false},
		{"클래스도 제네릭", "```mermaid\nclassDiagram\nclass 목록~T~\n목록 : +항목 List~T~\n```", nil, true},
		{"클래스도 쓰지 않는 줄", "```mermaid\nclassDiagram\nA <|-- B\nstyle A fill:#f9f\n```", []string{"4:style"}, true},
		{"클래스도 몸 안 줄", "- 목록\n\n  ```mermaid\n  classDiagram\n  class 고객 {\n    direction LR\n  }\n  ```", []string{"6:direction"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := render(t, c.src)
			var got []string
			for _, w := range r.Warnings {
				if w.Code != WarnDiagram {
					t.Errorf("다른 경고 %+v", w)
				}
				got = append(got, fmt.Sprintf("%d:%s", w.Line, w.Message))
			}
			if len(got) != len(c.want) {
				t.Fatalf("경고 %q, 기대 %q", got, c.want)
			}
			for i, w := range c.want {
				line, frag, _ := strings.Cut(w, ":")
				if !strings.HasPrefix(got[i], line+":") || !strings.Contains(got[i], frag) {
					t.Errorf("경고 %q, 기대 %q", got[i], w)
				}
			}
			if drawn := strings.Contains(r.HTML, "<svg"); drawn != c.drawn {
				t.Errorf("그림 %v, 기대 %v\n%s", drawn, c.drawn, r.HTML)
			}
			if !c.drawn && !strings.Contains(r.HTML, `class="language-mermaid"`) {
				t.Errorf("원문 코드 블록이 아님\n%s", r.HTML)
			}
		})
	}
}

// 한 문서에 그림 여럿 — 겹치는 id 가 없고(그림은 id 를 쓰지 않는다), 문서 상한(수)을 넘는 그림은 코드로(경고 한 번).
func TestDiagramPerDoc(t *testing.T) {
	var b strings.Builder
	for i := 0; i < MaxDiagramsPerDoc+5; i++ {
		fmt.Fprintf(&b, "```mermaid\ngraph LR\nA%d[그림 %d] --> B%d\n```\n\n", i, i, i)
	}
	r := render(t, b.String())
	if n := strings.Count(r.HTML, "<svg"); n != MaxDiagramsPerDoc {
		t.Errorf("그림 %d 개, 기대 %d", n, MaxDiagramsPerDoc)
	}
	if n := strings.Count(r.HTML, `class="language-mermaid"`); n != 5 {
		t.Errorf("코드로 둔 그림 %d 개", n)
	}
	if len(r.Warnings) != 1 || r.Warnings[0].Line != MaxDiagramsPerDoc*5+1 {
		t.Errorf("경고 %+v", r.Warnings)
	}
	if strings.Contains(r.HTML, " id=") {
		t.Error("그림이 id 를 씀")
	}
	// 바이트 합 상한: 큰 그림(약 300KB)을 여럿
	var big strings.Builder
	big.WriteString("```mermaid\ngraph TD\n")
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&big, "n%d[노드 %d] -->|간선| n%d\n", (i*37)%150, i, (i*91+7)%150)
	}
	big.WriteString("```\n\n")
	one := render(t, big.String())
	size := len(one.HTML)
	var doc strings.Builder
	n := MaxDiagramBytesPerDoc/size + 3
	for i := 0; i < n; i++ {
		doc.WriteString(strings.Replace(big.String(), "graph TD", fmt.Sprintf("graph TD\n%%%% %d", i), 1))
	}
	r = render(t, doc.String())
	if got := strings.Count(r.HTML, "<svg"); got >= n || got < n-4 {
		t.Errorf("바이트 상한: 그림 %d/%d (하나 %dKB)", got, n, size>>10)
	}
	if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0].Message, "MB") {
		t.Errorf("바이트 상한 경고 %+v", r.Warnings)
	}
}

// 라벨 글자는 모두 SVG 글자로 남는다(이스케이프만). 같은 그림은 결과 기억과 무관하게 같은 출력이고, 그리지 못한 것을
// 고치면 바로 그림이 된다(편집 중 회복).
func TestDiagramTextAndCache(t *testing.T) {
	src := "```mermaid\nflowchart TD\n  A[\"매출 & 비용 <합계>\"] -->|\"승인 '예'\"| B{검토}\n  subgraph 본사 영업팀\n    B\n  end\n```"
	r1 := render(t, src)
	for _, s := range []string{"매출 &amp; 비용 &lt;합계&gt;", "승인 &#39;예&#39;", ">검토<", ">본사 영업팀<"} {
		if !strings.Contains(r1.HTML, s) {
			t.Errorf("글자 %q 가 없음\n%s", s, r1.HTML)
		}
	}
	diagramCache = newLRU(512, 16<<20) // 기억을 비우고 다시
	r2 := render(t, src)
	r3 := render(t, src)
	if r1.HTML != r2.HTML || r2.HTML != r3.HTML {
		t.Error("결과 기억과 무관하게 같은 출력이어야 함")
	}
	// 쓰지 않은 줄의 경고는 기억에서 꺼내도 다시 난다
	w := "```mermaid\ngraph TD\nA-->B\nstyle A fill:red\n```"
	for i := 0; i < 2; i++ {
		if r := render(t, w); len(r.Warnings) != 1 || r.Warnings[0].Line != 4 {
			t.Errorf("%d번째 경고 %+v", i, r.Warnings)
		}
	}
	bad := render(t, "```mermaid\ngraph TD\nA --o B\n```")
	good := render(t, "```mermaid\ngraph TD\nA --> B\n```")
	if strings.Contains(bad.HTML, "<svg") || !strings.Contains(good.HTML, "<svg") || len(good.Warnings) != 0 {
		t.Errorf("편집 회복: %s / %s", bad.HTML, good.HTML)
	}
}

// 렌더 시간은 문서 길이에 선형이다(크기 두 배마다 시간이 거의 두 배): 서로 다른 그림 n 개(결과 기억에 없게),
// 상한을 넘는 많은 mermaid 블록, 그리지 못하는 큰 블록.
func TestDiagramLinear(t *testing.T) {
	gen := map[string]func(n int) string{
		"서로 다른 그림": func(n int) string {
			var b strings.Builder
			for i := 0; i < n; i++ {
				fmt.Fprintf(&b, "```mermaid\nflowchart TD\n%%%% %d-%d\n", n, i)
				for k := 0; k < 12; k++ {
					fmt.Fprintf(&b, "  n%d[단계 %d] -->|조건 %d| n%d\n", k, k, k, (k*5+3)%12)
				}
				b.WriteString("```\n\n문단 글.\n\n")
			}
			return b.String()
		},
		"상한 넘는 블록": func(n int) string {
			return strings.Repeat("```mermaid\ngraph TD\nA-->B\n```\n\n", n*4)
		},
		"그리지 못하는 블록": func(n int) string {
			return strings.Repeat("```mermaid\ngraph TD\nA[\""+strings.Repeat("가", 300)+"\n```\n\n", n)
		},
	}
	for name, g := range gen {
		for _, n := range []int{12, 24, 48} {
			assertSafe(t, render(t, g(n)).HTML)
		}
		// 비용은 CPU 시간이고 세 번 재어 작은 쪽, 실패로 보이면 모든 크기를 다시 잰다(timingtest, #201). 잴 때마다 결과 기억을 비운다
		timingtest.AssertDoubling(t, name, []int{12, 24, 48}, func(n int) func() {
			src := []byte(g(n))
			return func() {
				diagramCache = newLRU(512, 16<<20)
				if _, err := Render(src, Options{}); err != nil {
					t.Fatal(err)
				}
			}
		}, timingtest.Doubling{Reps: 3, Slack: 10 * time.Millisecond, Limit: 2 * time.Second})
	}
}

// 그린 SVG 는 차트와 같은 허용 목록(security_test.go)이다 — 견본 그림 전부.
func TestDiagramSafe(t *testing.T) {
	src := "```mermaid\nflowchart LR\n  a[사각] --> b(둥근) --> c([경기장]) --> d[[서브루틴]]\n  d --> e[(DB)] --> f((원)) --> g{판단}\n" +
		"  g --> h{{육각}} --> i[/입력/] --> j[\\역\\] --> k[/사다리\\] --> l[\\역사다리/]\n  a ==> l\n  a <--> f\n  a -.->|점선| g\n  l --> l\n" +
		"  subgraph s [묶음]\n    m --> n\n  end\n```"
	r := render(t, src)
	assertSafe(t, r.HTML)
	if regexp.MustCompile(`(?i)style=|href|url\(|on[a-z]+=`).MatchString(r.HTML) {
		t.Errorf("style·href·url·이벤트\n%s", r.HTML)
	}
}

// genDiagramDoc 은 그림 n 개(노드 12·화살표 16·subgraph 하나)와 문단이 섞인 문서다. seed 가 다르면 모든 그림이 다르다.
func genDiagramDoc(n, seed int) []byte {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "## 절 %d\n\n업무 흐름을 설명하는 문단이다. 담당 부서와 결재선을 적는다.\n\n```mermaid\nflowchart TD\n%%%% %d-%d\n", i, seed, i)
		b.WriteString("  subgraph 검토 단계\n    k1[팀장 검토] --> k2{승인?}\n  end\n")
		for k := 0; k < 10; k++ {
			fmt.Fprintf(&b, "  n%d[단계 %d 처리] -->|조건 %d| n%d\n", k, k, k, (k*3+1)%10)
		}
		b.WriteString("  n0 --> k1\n  k2 -->|예| n5\n  k2 -->|아니오| n0\n  n9 --> n9\n```\n\n")
	}
	return []byte(b.String())
}

// TestDiagramRenderTime 은 그림이 많은 문서의 렌더 시간(p50·p95)과 메모리를 잰다(값은 보고용 — 상한은 TestDiagramLinear).
//
//	MDOC_TIMING=1 go test ./mdoc -run TestDiagramRenderTime -v
func TestDiagramRenderTime(t *testing.T) {
	if os.Getenv("MDOC_TIMING") == "" {
		t.Skip("MDOC_TIMING=1 로 잰다")
	}
	measure := func(name string, src func(i int) []byte, n int) {
		var ds []time.Duration
		var ms0, ms1 runtime.MemStats
		runtime.ReadMemStats(&ms0)
		for i := range n {
			s := src(i)
			t0 := time.Now()
			if _, err := Render(s, Options{}); err != nil {
				t.Fatal(err)
			}
			ds = append(ds, time.Since(t0))
		}
		runtime.ReadMemStats(&ms1)
		sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
		t.Logf("%s: n=%d p50=%v p95=%v max=%v, 렌더 하나 할당 %.1fMB", name, n, ds[n/2].Round(time.Microsecond), ds[n*95/100].Round(time.Microsecond),
			ds[n-1].Round(time.Microsecond), float64(ms1.TotalAlloc-ms0.TotalAlloc)/float64(n)/(1<<20))
	}
	doc := genDiagramDoc(20, 0)
	t.Logf("문서: %d 바이트, 그림 20 개", len(doc))
	measure("처음 렌더(그림 20 개 모두 새로 배치)", func(i int) []byte { return genDiagramDoc(20, 1000+i) }, 30)
	_, _ = Render(doc, Options{})
	measure("편집 중 다시 렌더(그림 하나 바뀜)", func(i int) []byte {
		return append(append([]byte{}, doc...), fmt.Sprintf("```mermaid\ngraph TD\nA%d --> B\n```\n", i)...)
	}, 50)
	measure("그림 없는 같은 크기 문서", func(i int) []byte {
		return []byte(strings.ReplaceAll(string(genDiagramDoc(20, i)), "```mermaid", "```text"))
	}, 50)
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	t.Logf("GC 뒤 살아 있는 힙 %.1fMB(결과 기억 포함)", float64(ms.HeapAlloc)/(1<<20))
}
