package diagram

import (
	"strings"
	"testing"
)

// TestPolicyOddSpaceDefense 는 공통 정책 함수가 호출자의 줄 검사에 기대지 않고 ASCII 밖 공백·서식 문자(NBSP·EM SPACE·U+200B·
// U+FEFF·NEL)를 스스로 거부하는지 본다(itda-hyve#409 ④-2 후속 리뷰 1회차 [2]·[3] — 심층 방어). 기대는 손으로 적었다. 공백이 아닌 서식
// 문자(U+00AD SHY)는 받는다 — state 가 받던 그대로다.
func TestPolicyOddSpaceDefense(t *testing.T) {
	for _, sp := range []string{"\u00a0", "\u2003", "\u200b", "\ufeff", "\u0085", "\u3000"} {
		long := strings.Repeat("abc"+sp, 30) + "z"
		for _, v := range []string{"x" + sp + "y", long} {
			if _, msg := Label(v, "라벨", true); msg == "" {
				t.Errorf("Label(%q) 를 받음", v)
			}
			if _, msg := OneLine(v, "제목"); msg == "" {
				t.Errorf("OneLine(%q) 를 받음", v)
			}
			if _, msg := AccValue(v, "accTitle"); msg == "" {
				t.Errorf("AccValue(%q) 를 받음", v)
			}
			if _, msg := AccSet(v, "accDescr"); msg == "" {
				t.Errorf("AccSet(%q) 를 받음", v)
			}
			if _, msg := Caption(v); msg == "" {
				t.Errorf("Caption(%q) 를 받음", v)
			}
		}
		// 블록의 줄 머리 — 원본 \n\s+ 는 NBSP 를 지우지만 ASCII 정리(reNLSpace)는 남긴다. 그 줄에서 거부한다.
		var b AccBlock
		if _, _, msg := b.Add("a"); msg != "" {
			t.Fatalf("첫 줄 거부 %s", msg)
		}
		if done, _, msg := b.Add(sp + "b }"); !done || msg == "" {
			t.Errorf("AccBlock 줄 머리 %q 를 받음", sp)
		}
	}
	// 대조 — 공백이 아닌 서식 문자와 일반 빈칸은 받는다.
	if v, msg := Label("x\u00ady z", "라벨", true); msg != "" || v != "x\u00ady z" {
		t.Errorf("Label SHY: %q %s", v, msg)
	}
	if v, msg := AccSet("a\u00ad\tb\nc", "accDescr"); msg != "" || v != "a\u00ad\tb\nc" {
		t.Errorf("AccSet SHY·탭·줄바꿈: %q %s", v, msg)
	}
	var b AccBlock
	b.Add("a")
	if _, v, msg := b.Add("  b }"); msg != "" || v != "a\nb" {
		t.Errorf("AccBlock ASCII 줄 머리: %q %s", v, msg)
	}
}

// TestPolicyC1Range 는 공통 함수의 제어 문자 검사 범위를 본다 — Label·OneLine·Caption 의 검사는 C0·DEL 만 보고(C1 은 통과),
// AccSet 은 unicode.Cc 를 본다. 함수 단위의 회귀 감지이며 C1 의 제품 지원 약속이 아니다(DESIGN 2장 관찰 표). 기대는 손으로 적었다.
func TestPolicyC1Range(t *testing.T) {
	for _, v := range []string{"x\u0080y", "x\u009fy"} {
		if got, msg := Label(v, "라벨", true); msg != "" || got != v {
			t.Errorf("Label(%q) = %q %s", v, got, msg)
		}
		if got, msg := OneLine(v, "제목"); msg != "" || got != v {
			t.Errorf("OneLine(%q) = %q %s", v, got, msg)
		}
		if got, msg := Caption(v); msg != "" || got != v {
			t.Errorf("Caption(%q) = %q %s", v, got, msg)
		}
		if _, msg := AccSet(v, "accTitle"); msg == "" {
			t.Errorf("AccSet(%q) 를 받음", v)
		}
	}
	for _, v := range []string{"x\u0001y", "x\u007fy"} { // C0·DEL 은 어디서나 거부
		if _, msg := Label(v, "라벨", true); msg == "" {
			t.Errorf("Label(%q) 를 받음", v)
		}
		if _, msg := Caption(v); msg == "" {
			t.Errorf("Caption(%q) 를 받음", v)
		}
	}
}
