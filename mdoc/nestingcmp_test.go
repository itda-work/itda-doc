package mdoc

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// 블록 중첩 상한의 리뷰 가족(#116 R4~R6, #126 R1~R3)을 안전 모드의 계약으로 본다.
//
// 예전 계약 "상한은 깊은 부분의 넘는 표지만 글자로 바꾸고 깊은 부분 밖은 상한 없는 렌더와 바이트까지 같다" 는 파싱 중에 겹을 막던 방식의
// 것이다. 그 방식은 막은 겹의 닫는 줄 짝·이어쓰기·안쪽 블록의 수명을 흉내 내야 했고 리뷰마다 새 어긋남이 나왔다. 지금은 문서를 쪼개지
// 않는다 — 결과는 통째 둘 가운데 하나다(assertDecision): 실제 깊이가 상한 안이면 상한 없는 렌더와 바이트까지 같고, 넘으면 문서 전체가
// 원문 글자다(실제 파서가 65번째 겹을 받아들이는 순간 파싱을 멈춘다). 그래서 "깊은 부분 밖" 이 따로 없고, 같은 입력들로 다음을 본다:
//   - 판정 = 실제 깊이 > 64, 경고 줄은 65번째 겹을 연 줄
//   - 안전 모드면 원문의 표식이 모두 보인다(리뷰 R2 — 본문이 사라지면 안 된다)
//   - 상한에 닿기만 한 문서(64겹)는 보통 렌더다

// assertFamily 는 assertDecision 에 더해, 안전 모드일 때 원문의 표식이 결과 글자에 남는지 본다.
func assertFamily(t *testing.T, name, src string, marks ...string) {
	t.Helper()
	if !assertDecision(t, name, src) {
		return
	}
	text := renderedText(t, render(t, src).HTML)
	for _, m := range marks {
		if strings.Contains(src, m) && !strings.Contains(text, m) {
			t.Errorf("%s: 안전 모드에서 %s 가 사라짐", name, m)
		}
	}
}

// 리뷰가 적은 경계 사례들(깊은 부분 앞뒤에 보통 블록과 표식).
func TestNestingReviewCases(t *testing.T) {
	q63 := strings.Repeat("> ", MaxNestingDepth-1)
	cases := []struct{ name, deep string }{
		{"R5 인용 63 + 형제 목록", q63 + "- A\n" + q63 + "- B"},
		{"R5 인용 63 + 형제 번호 목록", q63 + "1. A\n" + q63 + "2. B"},
		{"인용 64 + 목록", strings.Repeat("> ", MaxNestingDepth) + "- A\n" + strings.Repeat("> ", MaxNestingDepth) + "- B"},
		{"번호 목록 64", strings.Repeat("1. ", MaxNestingDepth) + "KEEP"},
		{"번호 목록 65", strings.Repeat("1. ", MaxNestingDepth+1) + "KEEP"},
		{"R4 컨테이너 64 + 넘는 줄", strings.Repeat(":::box\n", MaxNestingDepth) + ":::overflow\nKEEP_ONE\nKEEP_TWO\n" + strings.Repeat(":::\n", MaxNestingDepth)},
		{"R4 컨테이너 64 + 숫자 문장", strings.Repeat(":::box\n", MaxNestingDepth) + "123 first\nKEEP_ONE\nKEEP_TWO\n" + strings.Repeat(":::\n", MaxNestingDepth)},
		{"R4 컨테이너 64 + 목록", strings.Repeat(":::box\n", MaxNestingDepth) + "- a\n- b\n- c\n" + strings.Repeat(":::\n", MaxNestingDepth)},
		{"컨테이너 70 균형", strings.Repeat(":::box\n", 70) + "안\n" + strings.Repeat(":::\n", 70)},
		{"컨테이너 70 닫는 줄 하나 더", strings.Repeat(":::box\n", 70) + "안\n" + strings.Repeat(":::\n", 71)},
		{"컨테이너 안 펜스 코드의 :::", strings.Repeat(":::box\n", 70) + "```\n:::\n:::x\n```\n" + strings.Repeat(":::\n", 70)},
		{"인용 70 여러 줄", strings.Repeat(strings.Repeat("> ", 70)+"줄\n", 3)},
		{"한 줄 섞음 > - 1.", strings.Repeat("> - 1. ", 30) + "끝"},
	}
	for _, c := range cases {
		src := "앞 문단 HEAD\n\n- 앞 목록\n\n앞 경계\n\n" + c.deep + "\n\n경계 문단\n\n- OUT\n- OUT2\n\n1. NUM\n\n> 인용 OUT\n\n:::box\n상자 OUT\n:::\n\nEND_MARK\n"
		assertFamily(t, c.name, src, "HEAD", "KEEP", "OUT", "NUM", "END_MARK")
	}
	// 상한에 닿기만 하고 넘지 않으면 보통 렌더이고 경고가 없다(R5: 64겹 목록에 초과 경고가 났다)
	for _, src := range []string{strings.Repeat("1. ", MaxNestingDepth) + "KEEP", strings.Repeat("> ", MaxNestingDepth) + "x",
		strings.Repeat(":::box\n", MaxNestingDepth) + "x\n" + strings.Repeat(":::\n", MaxNestingDepth)} {
		if r := render(t, src); isRaw(r) || len(r.Warnings) > 0 {
			t.Errorf("상한 그대로(%d겹)가 안전 모드이거나 경고: %q %+v", MaxNestingDepth, shortSrc(src), r.Warnings)
		}
	}
}

// 무작위: 보통 부분 A + 상한 근처(±)의 깊은 부분 D + 보통 부분 B(인용·목록·한 줄 섞음·컨테이너, 넘는 컨테이너를 닫자마자 다시 여는 줄 —
// 리뷰 #126 R1).
func TestNestingRestUnchanged(t *testing.T) {
	rng := rand.New(rand.NewSource(1167))
	normal := []string{"글 ", "- 항목\n", "1. 번호\n", "> 인용\n", "\n", "**굵게** ", "`코드` ", ":::box\n안\n:::\n", "| a | b |\n|---|---|\n| 1 | 2 |\n", "## 제목\n", "    들여쓴 코드\n", "[링크](#a) ", "$x^2$ "}
	part := func(n int) string {
		var b strings.Builder
		for range n {
			b.WriteString(normal[rng.Intn(len(normal))])
		}
		return b.String()
	}
	markers := []string{"> ", "- ", "* ", "1. ", "2) ", "+ "}
	deep := func() string {
		L := MaxNestingDepth - 3 + rng.Intn(10)
		switch rng.Intn(6) {
		case 0: // 인용 여러 줄 + 형제 목록
			p := strings.Repeat("> ", L)
			return p + "- A\n" + p + "- B\n" + p + "본문"
		case 1: // 한 줄에 섞인 표지
			var b strings.Builder
			for range L {
				b.WriteString(markers[rng.Intn(len(markers))])
			}
			return b.String() + "끝\n" + "  이어진 줄"
		case 2: // 컨테이너, 닫는 줄은 같거나 하나 더
			return strings.Repeat(":::box\n", L) + part(3) + "\n" + strings.Repeat(":::\n", L+rng.Intn(2))
		case 3: // 들여쓰기로 깊어지는 목록
			var b strings.Builder
			for k := range L {
				b.WriteString(strings.Repeat("  ", k) + "- 단계\n")
			}
			return b.String()
		case 5: // 넘는 컨테이너를 닫자마자 다시 연다(리뷰 #126 R1)
			var b strings.Builder
			b.WriteString(strings.Repeat(":::box\n", L))
			for range 1 + rng.Intn(4) {
				b.WriteString([]string{"IN\n", "- a\n", "> q\n", ""}[rng.Intn(4)] + ":::\n:::note\n")
			}
			b.WriteString("안\n" + strings.Repeat(":::\n", L-1+rng.Intn(4)))
			return b.String()
		default: // 인용 안 컨테이너
			p := strings.Repeat("> ", L/2)
			return p + ":::box\n" + p + "안\n" + p + ":::"
		}
	}
	for i := range 400 {
		src := part(1+rng.Intn(6)) + "\n\n앞 경계\n\n" + deep() + "\n\n경계 문단\n\n" + part(1+rng.Intn(6)) + "\nEND_MARK\n"
		assertFamily(t, fmt.Sprintf("#%d", i), src, "END_MARK", "앞 경계", "경계 문단")
		if t.Failed() {
			t.Fatalf("#%d 원문 %q", i, src)
		}
	}
}

// 리뷰 #126 R1·R2: 상한 근처에서 ::: 여는 줄·닫는 줄·본문과 안쪽 블록(HTML 블록 일곱 시작 조건, 펜스·닫히지 않은 펜스, 수식, 목록,
// 인용, 표, 들여쓴 코드, 내용 없는 표지)이 엇갈린다. 본문 표식(K..)이 안전 모드에서도 남아야 한다.
func TestNestingPhantomFamily(t *testing.T) {
	rng := rand.New(rand.NewSource(1261))
	toks := []string{":::box\n", ":::\n", ":::\n", ":::note\n", ":::{.c}\n", "IN\n", "둘째 줄\n", "\n",
		"<script>\n", "</script>\n", "<!--\n", "-->\n", "<?pi\n", "?>\n", "<!DOCTYPE x\n", ">\n", "<![CDATA[\n", "]]>\n", "<div>\n", "</div>\n", "<my-tag>\n",
		"```\n", "```\n:::\n```\n", "~~~\n", "$$\n", "- a\n", "1. n\n", "  - 안\n", "> q\n", "| a | b |\n|---|---|\n", "| 1 | 2 |\n", "    코드\n",
		">\n", "-\n", "> ```\n", "> # 제목\n", "- ---\n", "> <!--\n"}
	for i := range 2000 {
		var b strings.Builder
		b.WriteString(strings.Repeat(":::box\n", MaxNestingDepth-2+rng.Intn(6)))
		var marks []string
		for j := range 3 + rng.Intn(30) {
			if rng.Intn(4) == 0 {
				m := fmt.Sprintf("K%02d", j)
				marks = append(marks, m)
				b.WriteString(m + "\n")
			} else {
				b.WriteString(toks[rng.Intn(len(toks))])
			}
		}
		b.WriteString(strings.Repeat(":::\n", MaxNestingDepth-3+rng.Intn(10)))
		src := "앞 문단 HEAD\n\n" + b.String() + "\n경계 문단\n\n- OUT\n\n:::box\n상자 OUT\n:::\n\nEND_MARK\n"
		assertFamily(t, fmt.Sprintf("#%d", i), src, append(marks, "HEAD", "OUT", "END_MARK")...)
		if t.Failed() {
			t.Fatalf("#%d 원문 %q", i, src)
		}
	}
}

// 리뷰 #126 R2: 상한 근처의 인용·목록·각주 정의 줄과 그 다음 줄(이어쓰기일 수 있는 줄) — ::: 없이.
func TestNestingRefusedLineFamily(t *testing.T) {
	rng := rand.New(rand.NewSource(1262))
	prefixes := []string{"> ", "- ", "1. ", "> - ", "[^a]: "}
	rests := []string{"", "글", "```", "~~~", "# 제목", "---", "<!--", "<div>", "    코드", "$$", "| a |", "> 글", "- 글", ">", "-", "1.", "[^b]: 글", "<!-- x -->"}
	nexts := []string{"OUT", "- OUT", "1. OUT", "> OUT", "# OUT", ":::box", "OUT ```", "```", "| x |\n|---|", "    OUT"}
	for i := range 3000 {
		p := prefixes[rng.Intn(len(prefixes))]
		n := MaxNestingDepth - 1 + rng.Intn(4)
		if p == "> - " {
			n = n/2 + rng.Intn(2)
		}
		deep := strings.Repeat(p, n) + rests[rng.Intn(len(rests))] + "\n"
		if rng.Intn(3) == 0 {
			deep += strings.Repeat(p, n) + rests[rng.Intn(len(rests))] + "\n"
		}
		src := "앞 문단 HEAD\n\n" + deep + nexts[rng.Intn(len(nexts))] + "\n뒤 줄\n\n경계 문단\n\nEND_MARK\n"
		assertFamily(t, fmt.Sprintf("#%d", i), src, "HEAD", "OUT", "END_MARK")
		if t.Failed() {
			t.Fatalf("#%d 원문 %q", i, src)
		}
	}
}

// 리뷰 #126 R1·R2·R3 이 적은 입력 그대로(65겹 ::: 안에서 닫자마자 다시 열기, 안쪽 블록이 빈 줄로 끝나지 않는 HTML 블록).
func TestNestingPhantomReopen(t *testing.T) {
	for _, in := range []string{"IN\n", "<!--\n", "<script>\n", "<![CDATA[\n", "<?pi\n", "<div>\n", "$$\n", "- a\n", "> q\n", "| a |\n|---|\n"} {
		for _, extra := range []int{1, 2} {
			for _, closes := range []int{MaxNestingDepth + extra - 1, MaxNestingDepth + extra} {
				src := strings.Repeat(":::box\n", MaxNestingDepth+extra) + in + ":::\n:::note\nSECOND\n" + strings.Repeat(":::\n", closes) + "\nTAIL\n"
				name := fmt.Sprintf("%q +%d 닫는 줄 %d", in, extra, closes)
				assertFamily(t, name, src, "SECOND", "TAIL")
				if !isRaw(render(t, src)) {
					t.Errorf("%s: 65겹 이상인데 보통 렌더", name)
				}
			}
		}
	}
}
