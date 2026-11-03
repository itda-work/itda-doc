package mdoc

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// 속성 — 원문 글자 보존(리뷰 R4). 무작위 문서에 고유한 표식 낱말(K0, K1, …)을 마크다운·수식 문법 조각과 섞어 렌더하고,
// 결과의 글자(본문 글자와 속성 값 — 코드 블록 언어·컨테이너 이름은 속성으로 남는다)에 모든 표식이 남는지 본다.
// 렌더 시간 상한(강조 구분자 1000·링크 닫기 200·수식 예산)에 걸리는 조각과 깊은 중첩(65겹 남짓 — nesting.go 의 상한, 문서 전체가 원문 글자가 된다)도 섞는다.
//
// 이 시험이 보는 것은 "표식이 남는다" 뿐이다 — 원문 전체의 글자·차례·횟수나 렌더 구조의 동등성까지 증명하지 않는다. 블록 중첩 상한의
// 결정(보통 렌더 그대로 또는 통째 원문 글자)은 nesting_test.go·nestingcmp_test.go 가 본다. 생성기에서 뺀 자리와 까닭:
//   - 표의 머리 행보다 많은 칸: GFM 사양이 버린다(Example 204) — 그래서 표 구분 행 |---| 은 넣지 않고 | 는 글자로 섞는다.
//   - 펜스 코드 여는 줄의 info string 둘째 낱말부터: 이 렌더러가 첫 낱말만 언어 class 로 쓰는 처리(사양이 강제하지는 않는다) —
//     그래서 ~·` 조각은 앞에 글자를 붙여 줄 머리에서 펜스가 되지 않게 한다.
//   - 쓰이지 않은 링크 정의 `[x]: y`: 정의는 본문에 나오지 않는다.
//   - 링크 주소 `(…)`: 주소는 href 로 남지만 주소 정규화·보안 처리(1.9)를 이 속성의 범위에서 빼려는 보수적인 선택이다.
//   - raw HTML·frontmatter·각주·작업 목록: 태그·메타데이터(Meta 로 옮겨 간다)·참조 표지·체크박스는 본문 글자로 남을 의무가 없다.
//     그 안의 본문(각주·작업 목록의 글)은 보존 대상이지만 이 시험은 그 문법을 통째로 빼 범위를 좁혔다.
//   - TeX 주석 %: 수식 주석은 수식 출력의 보존 대상이 아니다(마크다운 본문의 % 는 글자로 남는다).
//
// 수식 안의 표식은 MathML 의 글자로 남는다(<mi>K</mi><mn>03</mn> 의 글자는 "K03").
var textAtoms = []string{
	"*", "**", "_", "x~", "x~~", "x`", "> ", "- ", "1. ", "#", "## ", "\n", "\n\n", "    ", "| ", " | ", "[", "]", "!",
	`\`, "$", "$$", `\text{`, "{", "}", "&", "\n:::box\n", "\n:::\n", "\n```\n", "=", "\n---\n", " ", "가", ". . .",
}

// 상한을 넘기는 큰 조각(가끔 넣는다)
var heavyAtoms = []func(rng *rand.Rand) string{
	func(*rand.Rand) string { return strings.Repeat("> ", MaxNestingDepth+3) },
	func(*rand.Rand) string { return strings.Repeat(":::box\n", MaxNestingDepth+2) },
	func(*rand.Rand) string { return strings.Repeat("1. ", MaxNestingDepth+2) },
	func(*rand.Rand) string { return "x " + strings.Repeat("~", 3+500) },
	func(*rand.Rand) string { return strings.Repeat("*a_", MaxDelimitersPerBlock/2+10) },
	func(*rand.Rand) string { return strings.Repeat("[a]", MaxLinkClosersPerBlock+10) },
	func(*rand.Rand) string { return strings.Repeat(`$\text{`, 50) },
}

func TestTextPreserved(t *testing.T) {
	rng := rand.New(rand.NewSource(1166))
	for i := range 3000 {
		var b strings.Builder
		b.WriteString("시작 ")
		var marks []string
		for j := range 1 + rng.Intn(30) {
			switch r := rng.Intn(10); {
			case r < 4:
				// 길이가 같은 표식(K00~K99) — 길이가 다르면 K1 이 K11 의 부분 문자열이라 K1 이 사라져도 검사가 통과했다(리뷰 R5)
				m := fmt.Sprintf("K%02d", j)
				marks = append(marks, m)
				b.WriteString(m)
			case r == 4 && rng.Intn(8) == 0:
				b.WriteString(heavyAtoms[rng.Intn(len(heavyAtoms))](rng))
			default:
				b.WriteString(textAtoms[rng.Intn(len(textAtoms))])
			}
		}
		src := b.String()
		r := render(t, src)
		got := renderedText(t, r.HTML)
		if miss := missingMarks(got, marks); len(miss) > 0 {
			t.Fatalf("#%d 표식 %v 가 렌더 결과에 없음\n원문 %q\n글자 %q\nHTML %s", i, miss, src, got, r.HTML)
		}
	}
}

// missingMarks 는 글자 got 에 없는 표식이다. 표식은 길이가 같아 하나가 다른 것의 부분 문자열이 되지 않는다 —
// 표식 뒤에 숫자가 이어 붙어도(K01 + 2 → "K012") 그 자리의 K01 은 있는 것이다.
func missingMarks(got string, marks []string) []string {
	var miss []string
	for _, m := range marks {
		if !strings.Contains(got, m) {
			miss = append(miss, m)
		}
	}
	return miss
}

// 판정 자체의 시험(리뷰 R5 P3): 실제 렌더 글자에서 표식 하나를 지우면 missingMarks 가 그 표식을 찾아낸다 — 리뷰의 itda-hyve#5 꼴
// (K1 과 K11·K14 가 함께 있던 문서)을 길이가 같은 표식으로.
func TestTextPreservedOracle(t *testing.T) {
	src := "시작 *K01K02K03- . . .K06가K08K09}K11**&K14\n:::box\nK16"
	marks := []string{"K01", "K02", "K03", "K06", "K08", "K09", "K11", "K14", "K16"}
	got := renderedText(t, render(t, src).HTML)
	if miss := missingMarks(got, marks); len(miss) > 0 {
		t.Fatalf("렌더 글자에 없음 %v: %q", miss, got)
	}
	for _, m := range marks {
		cut := strings.Replace(got, m, "", 1)
		if miss := missingMarks(cut, marks); len(miss) != 1 || miss[0] != m {
			t.Errorf("%s 하나를 지웠는데 판정 %v", m, miss)
		}
	}
}

// renderedText 는 결과 HTML 의 본문 글자와 모든 속성 값을 이어 붙인다(요소 사이는 띄운다 — 표식이 이웃 글자와 붙지 않게).
// 수식(<math>) 안은 글자를 띄우지 않고 붙인다 — <mi>K</mi><mn>3</mn> 는 "K3" 이다.
func renderedText(t *testing.T, out string) string {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	var walk func(n *html.Node, inMath bool)
	walk = func(n *html.Node, inMath bool) {
		switch n.Type {
		case html.TextNode:
			b.WriteString(n.Data)
		case html.ElementNode:
			if n.Data == "math" {
				inMath = true
			}
			if !inMath {
				b.WriteByte(' ')
			}
			for _, a := range n.Attr {
				b.WriteString(" " + a.Val + " ")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, inMath)
		}
		if n.Type == html.ElementNode && !inMath {
			b.WriteByte(' ')
		}
	}
	walk(doc, false)
	return b.String()
}
