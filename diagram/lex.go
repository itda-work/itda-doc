package diagram

import (
	"strings"
	"unicode"
)

// 원본 lexer 와 Go 의 글자 판정이 갈리는 곳을 막는 도구 — 허용 목록 문법(itda-hyve#208 결정)을 쓰는 그림 종류가 함께 쓴다
// (itda-hyve#409 ③ sequence 에서 처음 썼고 ④ er 가 함께 쓴다).

// OddSpace 는 줄에서 ASCII 밖 공백·서식 문자를 찾는다. 원본 JS 정규식 \s 는 ECMAScript WhiteSpace(U+0009·000B·000C·
// 0020·00A0·FEFF·Zs 범주 — U+1680·2000–200A·202F·205F·3000)와 LineTerminator(U+000A·000D·2028·2029)이고, Go 의
// unicode.IsSpace 는 여기서 U+FEFF 를 빼고 U+0085 를 더한다. 둘 중 하나라도 공백으로 보는 ASCII 밖 글자 전부와,
// 공백처럼 보이지만 어느 쪽도 공백이 아닌 서식 문자 U+200B–200D(폭 없는 빈칸·연결자)·U+2060(단어 이음)을 막는다.
func OddSpace(s string) (rune, bool) {
	for _, r := range s {
		if r > 0x7f && (unicode.IsSpace(r) || r == 0xfeff || 0x200b <= r && r <= 0x200d || r == 0x2060) {
			return r, true
		}
	}
	return 0, false
}

// ASCIILower 는 ASCII 영문자만 소문자로 바꾼다(바이트 길이가 같다). 원본 jison 의 case-insensitive 는 JS 정규식의
// i 깃발(u 깃발 없음)이라 ASCII 만 접는다(ſ U+017F·K U+212A 는 s·k 가 아니다) — Go 의 (?i) 는 유니코드로 접으므로 쓰지 않는다.
func ASCIILower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// DirectiveProblem 은 설정 지시 %%{…}%% 가 든 줄(앞뒤 공백을 지운 것)을 본다 — 돌려준 글이 비어 있지 않으면 그 줄을
// 그리지 않는다(원문 + 줄 번호 경고). 원본 전처리 removeDirectives 는 줄 어디서든 지시만 지우고 나머지 글(지시 사이의 문장·
// accTitle 값 …)을 그대로 읽는다. 이 판은 그 재현을 하지 않고, 줄 전체가 지시 하나(`%%{` 로 시작해 `}%%` 로 끝나고 그
// 사이에 `%%` 가 다시 없음)인 줄만 무시하는 지시로 받는다(itda-hyve#409 ④ 리뷰 1회차 — 2026-10-30 축소 결정). inside 는 블록 안
// (속성 블록·accDescr { … } 안)이라 지시 하나짜리 줄도 받지 않는 자리다. 지시가 없는 줄은 빈 글이다.
func DirectiveProblem(t string, inside bool) string {
	if !strings.Contains(t, "%%{") {
		return ""
	}
	if strings.HasPrefix(t, "%%{") && !strings.HasSuffix(t, "}%%") && strings.Count(t, "%%") == 1 {
		return "설정 지시(%%{…}%%)는 한 줄로 닫아야 함 — 이 판은 여러 줄 지시를 읽지 않는다"
	}
	whole := strings.HasPrefix(t, "%%{") && strings.HasSuffix(t, "}%%") && len(t) >= len("%%{}%%") && !strings.Contains(t[2:len(t)-2], "%%")
	if whole && !inside {
		return ""
	}
	if whole {
		return "설정 지시(%%{…}%%)는 블록 안에 두지 못 함 — 이 판은 그 줄을 그리지 않는다"
	}
	return "설정 지시(%%{…}%%)는 한 줄에 하나만 따로 — 이 판은 그 줄을 그리지 않는다"
}
