package diagram

import (
	"regexp"
	"strings"
)

// 원본 엔티티 전처리(mermaid@12.2.0 utils.ts encodeEntities · decodeEntities — Diagram.fromText 가 파싱 전에 지나친다).
// itda-hyve#409 ② timeline 에서 처음 재현했고 ③ sequence 도 같은 규칙을 쓴다(한 곳에 둔다).
//
// 원본은 원문 전체를 encodeEntities 로 지나친다: `style…:#…;`·`classDef…:#…;` 줄의 끝 `;` 를 자르고, 엔티티 `#\w+;`
// (JS \w = ASCII [A-Za-z0-9_] — Go regexp \w 와 같다)를 # 이 아닌 내부 표식으로 바꾼다(정수 `ﬂ°°N¶ß`, 이름 `ﬂ°N¶ß`
// — 원본 표식 문자를 그대로 쓴다). 그래서 각 그림 종류의 lexer 가 # 을 주석으로 잘라도 엔티티의 # 은 버려지지 않는다.
// decodeEntities 는 표식을 다시 `#…;` 로 되돌린다(공통 DecodeLabel 이 글자로 푼다) — 되돌린 글자가 문법 글자(: 등)여도
// 문법은 이미 지쳤으므로 다시 읽히지 않는다(원본과 같다).

var (
	reStyleHash = regexp.MustCompile(`style.*:\S*#.*;`)
	reClassHash = regexp.MustCompile(`classDef.*:\S*#.*;`)
	reEntity    = regexp.MustCompile(`#\w+;`)
	reEntMark   = regexp.MustCompile(`ﬂ°°?(\w+)¶ß`)
	reEntInt    = regexp.MustCompile(`^\+?\d+$`)
)

// EncodeEntities 는 원본 전처리를 한 줄에 재현한다(줄 안에서만 작동한다 — 원본 두 정규식의 점은 줄바꿈을 건너지 않는다).
func EncodeEntities(line string) string {
	cut := func(s string) string { return s[:len(s)-1] }
	line = reStyleHash.ReplaceAllStringFunc(line, cut)
	line = reClassHash.ReplaceAllStringFunc(line, cut)
	return reEntity.ReplaceAllStringFunc(line, func(s string) string {
		inner := s[1 : len(s)-1]
		if reEntInt.MatchString(inner) {
			return "ﬂ°°" + inner + "¶ß"
		}
		return "ﬂ°" + inner + "¶ß"
	})
}

// StyleCut 은 원본 전처리의 첫 두 정규식(`style.*:\S*#.*;`·`classDef.*:\S*#.*;`)이 줄에 맞는지다 — 맞으면 원본은 그 줄의 마지막
// `;` 하나를 지운 뒤 엔티티를 지킨다. 라벨 글에만 전처리를 씌우는 종류(흐름도·gantt)는 이 줄에서 원본과 뜻이 갈릴 수 있다(itda-hyve#409 ④-2
// 후속 리뷰 1회차 [1] — `A["style x:#35;"];` 는 원본이 줄 끝 ; 를, 라벨 전처리는 엔티티의 ; 를 지운다).
func StyleCut(line string) bool {
	return reStyleHash.MatchString(line) || reClassHash.MatchString(line)
}

// EntityTokens 는 글 안의 엔티티 토큰(`#\w+;`) 수다. 라벨 글만 전처리하는 종류가 「줄의 모든 토큰이 라벨 안에 통째로 있다」를 확인
// 하는 데 쓴다 — 라벨은 줄의 부분 문자열이고 #·; 는 \w 가 아니므로 라벨 안의 토큰은 줄의 토큰이기도 하다(부분집합). 수가 같으면 두
// 전처리가 같은 토큰을 지킨다.
func EntityTokens(s string) int { return len(reEntity.FindAllStringIndex(s, -1)) }

// DecodeEntities 는 표식을 엔티티 글 `#…;` 로 되돌린다.
func DecodeEntities(s string) string {
	return reEntMark.ReplaceAllString(s, "#$1;")
}

// HasMarker 는 원문 줄에 Mermaid 내부 표식 문자(encodeEntities 가 만드는 ﬂ°·¶ß — ﬂ U+FB02, ° U+00B0, ¶ U+00B6,
// ß U+00DF)가 이미 들어 있는지 본다. 들어 있으면 이 판은 그 줄을 그리지 않는다(2026-10-30 사용자 결정 — 원본은 그리지만,
// 이 판의 복원은 이 판이 만든 표식만 되돌리므로 원문 표식과 구별한다. itda-hyve#409 ② 리뷰 3회차 [2] 와 같은 결정).
func HasMarker(line string) bool {
	return strings.Contains(line, "ﬂ°") || strings.Contains(line, "¶ß")
}
