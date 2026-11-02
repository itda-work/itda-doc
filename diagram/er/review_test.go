package er

import (
	"strings"
	"testing"
)

// TestReviewR1 은 리뷰 1회차 재현이다 — 설정 지시 %%{…}%% 는 줄 전체가 지시 하나(블록 밖)일 때만 무시하고 그린다. 지시
// 앞뒤·사이의 글, acc 값·라벨·주석 안의 지시, 블록 안의 지시 줄은 원문 + 줄 번호 경고(원본 removeDirectives 는 지시만
// 지우고 남은 글을 읽는다 — 이 판은 그 갈래를 그리지 않는다).
func TestReviewR1(t *testing.T) {
	const d = "%%{init: {}}%%"
	const one = "한 줄에 하나만"
	for _, c := range []struct {
		src  string
		line int
		msg  string
	}{
		{"erDiagram\n" + d + " A " + d + "\nB", 2, one},   // [1] 지시 사이 엔티티 A 가 사라지던 것
		{"erDiagram\n" + d + " ??? " + d + "\nB", 2, one}, // [1] 지시 사이 문법 오류가 사라지던 것
		{"erDiagram\naccTitle: 앞 " + d + " 뒤\nB", 2, one}, // [2] aria-label 에 지시가 남던 것
		{"erDiagram\naccDescr: 앞 " + d + " 뒤\nB", 2, one}, // [2] <title> 에 지시가 남던 것
		{"erDiagram\n" + d + " A", 2, one},
		{"erDiagram\nA " + d, 2, one},
		{"erDiagram\n%% 메모 " + d + "\nA", 2, one},
		{"erDiagram\nA {\n  " + d + "\n}", 3, "블록 안"},
		{"erDiagram\nA {\n  string a \"" + d + "\"\n}", 3, one},
		{"erDiagram\naccDescr {\n" + d + "\n}\nA", 3, "블록 안"},
		{"erDiagram\naccDescr {\n설명 " + d + "\n}\nA", 3, one},
		{"erDiagram\nA[\"" + d + "\"]", 2, one},
		{"erDiagram\nA ||--o{ B : \"" + d + "\"", 2, one},
		{"erDiagram\n%%{init: {\"a\": \"%%\"}}%%\nA", 2, one}, // 지시 안의 %% — 지시 하나로 확신할 수 없다
		{"erDiagram\n%%{init: {\nA", 2, "한 줄로 닫아야"},
	} {
		errAt(t, c.src, c.line, c.msg)
	}
	// 줄 전체가 지시 하나면 여전히 무시하고 그린다(머리 앞·뒤, 앞뒤 공백).
	for _, src := range []string{d + "\nerDiagram\nA", "erDiagram\n  " + d + "  \nA", "erDiagram\n%%{}%%\nA ||--o{ B : x"} {
		svg, iss := drawn(t, src)
		if len(iss) != 1 || !strings.Contains(iss[0].Msg, "설정 지시") || !strings.Contains(svg, ">A<") || strings.Contains(svg, "%%{") {
			t.Errorf("%q: 경고 %+v", src, iss)
		}
	}
}

// TestReviewR2 는 리뷰 2회차 재현이다 — 판정기가 accDescr { 여는 줄의 { 뒤를 원문 줄로 다시 읽어(전처리 두 번) 정상 입력을
// 거부하던 것. 제품은 그 본문을 바로 accBody 로 보낸다(parse.go statement). 원본 근거: 전처리 cleanupComments
// (/^\s*%%(?!{)[^\n]+\n?/gm)는 실제 줄 머리 %% 만 지우므로 본문 가운데의 %% 는 글이고, erDiagram.jison:16–18 은
// accDescr\s*"{"\s* 뒤부터 } 앞까지를 설명 값(acc_descr_multiline_value)으로 읽는다. 엔티티 #…; 는 encodeEntities 가
// 지켜 두었다가 글자로 풀린다.
func TestReviewR2(t *testing.T) {
	for _, c := range []struct{ src, descr string }{
		{"erDiagram\naccDescr { #35; }\nA", "#"},
		{"erDiagram\naccDescr { %% 메모 }\nA", "%% 메모"},
		{"erDiagram\naccDescr { #quot;x }\nA", `"x`},
		{"erDiagram\naccDescr { 앞 #35;\n뒤 #quot;x }\nA", "앞 #\n뒤 \"x"}, // 정책 B — 줄바꿈은 그대로(원본 \n\s+ → \n)
		{"erDiagram\naccDescr { #lt;b#gt;\n#59; }\nA", "<b>\n;"},
	} {
		ch, _, err := Parse(c.src)
		if err != nil {
			t.Errorf("%q: Parse 오류 %v", c.src, err)
			continue
		}
		if ch.AccDescr != c.descr {
			t.Errorf("%q: 설명 %q, 기대 %q", c.src, ch.AccDescr, c.descr)
		}
		if _, err := Render(c.src); err != nil {
			t.Errorf("%q: Render 오류 %v", c.src, err)
		}
		want, bad := jJudge(c.src)
		if want == nil {
			t.Errorf("%q: 판정기가 거부(줄 %d)", c.src, bad)
		} else if got, exp := summary(ch), jSummary(want); got != exp {
			t.Errorf("%q: 값이 다름\n Parse  %s\n 판정기 %s", c.src, got, exp)
		}
	}
}
