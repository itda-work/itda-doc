package mdoc

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// 목록 기호 사다리(#141). 문서는 그냥 `-`·`1.` 목록으로 쓰고, 단계(중첩 깊이)마다의 기호는 테마 CSS 가 그린다 —
// 원문·HTML 은 보통 목록 그대로라 다른 뷰어에서도 깨지지 않는다(결정 5: 모양은 문서가 아니라 테마가 정한다).
//
//	gaejosik: 비순서 목록 □ → ○ → - → ·  (개조식 보고서)
//	gongmun : 순서 목록 1. → 가. → 1) → 가) → (1) → (가) → ① → ㉮  (공문 항목 구분 — 행정업무운영 편람)
//	plain   : 둘 다 끈다(테마 기본을 끄고 보통 목록으로)
//
// 둘은 서로 다른 목록 종류에 걸리므로 함께 켤 수 있다. 값은 정규형 글자로 둔다: "" (정하지 않음 — 테마 기본),
// "plain", "gaejosik", "gongmun", "gaejosik gongmun".
const (
	ListGaejosik = "gaejosik"
	ListGongmun  = "gongmun"
	ListPlain    = "plain"
)

// NormalizeListStyle 은 list-style 값 목록을 정규형으로 바꾼다. 같은 값이 거듭되면 하나로 본다.
// plain 은 다른 값과 함께 쓸 수 없다. 빈 목록은 오류다(값을 적지 않으려면 키를 빼면 된다).
func NormalizeListStyle(vals []string) (string, error) {
	var gae, gong, plain bool
	for _, v := range vals {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case ListGaejosik:
			gae = true
		case ListGongmun:
			gong = true
		case ListPlain:
			plain = true
		default:
			return "", fmt.Errorf("모르는 값 %q", v)
		}
	}
	switch {
	case plain && (gae || gong):
		return "", fmt.Errorf("plain 은 다른 값과 함께 쓰지 않는다")
	case plain:
		return ListPlain, nil
	case gae && gong:
		return ListGaejosik + " " + ListGongmun, nil
	case gae:
		return ListGaejosik, nil
	case gong:
		return ListGongmun, nil
	}
	return "", fmt.Errorf("값이 없음")
}

// listStyleWant 는 틀린 값 경고에 쓰는 받는 꼴이다.
const listStyleWant = "gaejosik·gongmun·plain 중 하나 또는 [gaejosik, gongmun]"

// parseListStyle 은 frontmatter list-style: 의 글자 또는 글자 목록을 읽는다.
func parseListStyle(v *yaml.Node) (string, bool) {
	var vals []string
	if s, ok := scalar(v); ok {
		vals = []string{s}
	} else if v.Kind == yaml.SequenceNode {
		for _, it := range v.Content {
			s, ok := scalar(it)
			if !ok {
				return "", false
			}
			vals = append(vals, s)
		}
	} else {
		return "", false
	}
	s, err := NormalizeListStyle(vals)
	return s, err == nil
}
