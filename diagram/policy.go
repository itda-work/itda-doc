package diagram

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// 엔티티·공백 정책 B 와 출력 관문 도구 — 여섯 종류(흐름도·gantt·timeline·sequence·er·state)가 함께 쓴다(itda-hyve#409 ④-2 접근 방식
// 리뷰, 2026-10-31 사용자 결정 — state 에서 정하고 후속 PR 에서 여기로 옮겼다. 설계 spec/diagrams-design.md 2장).
//
// 원본은 전처리 encodeEntities 가 지킨 표식(`#\w+;` → 정수 ﬂ°°N¶ß·이름 ﬂ°N¶ß — EncodeEntities)만 SVG 정리 단계(decodeEntities →
// `&#N;`·`&이름;`)에서 한 번 되돌린다. 이 판은 그 표식 가운데 아래 정책이 받는 것만 글자로 한 번 되돌리고, 받지 않는 표식이 든
// 줄은 원문 + 줄 번호 경고다. 표식 밖의 글자(원본이 엔티티로 읽지 않는 `#+65;` 따위)는 다시 찾거나 풀지 않고, 되돌린 글자도
// 다시 읽지 않는다(`#35;35;` → `#35;`). 옛 공통 DecodeLabel(이름 8개·Fields 접기)은 ParseFrontmatter 의 title 길이 검사만 쓴다 — 캡션은 여섯 종류 모두 Caption 이다.
//
//	이름  quot " · amp & · lt < · gt > · apos ' · num # · semi ; — 7개, 대소문자 구분(nbsp 는 공백이라 뺀다)
//	숫자  십진 1~7자리(앞 0 허용)이고 U+0021~U+10FFFF 의 유효 스칼라. 아래를 뺀다(Go unicode 표 — EntityUnicodeVersion 판본):
//	      Cc(제어)·Cf(서식)·White_Space·Noncharacter_Code_Point(비문자)·서러게이트 D800–DFFF
//
// 그 밖(#copy;·#euro;·#nbsp;·#160;·#8195;·#32;·#x41;·8자리 …)은 받지 않는다 — 원본 오류라는 뜻이 아니라 이 판의 미지원이다.

// EntityUnicodeVersion 은 숫자 정책이 기대는 Go unicode 표의 판본이다(Go 를 올려 판본이 바뀌면 state 의 TestEntityPolicyTable 이
// 알린다).
const EntityUnicodeVersion = "17.0.0"

// reMark 는 EncodeEntities 가 지킨 엔티티 표식이다 — 묶음 1 은 정수 표식의 두 번째 °, 2 는 이름 또는 수. 원문의 표식 문자는 각
// 종류의 parse 가 먼저 막으므로(HasMarker) 여기 오는 표식은 모두 이 판이 만든 것이다.
var reMark = regexp.MustCompile(`ﬂ°(°?)(\w+)¶ß`)

var entityNames = map[string]rune{"quot": '"', "amp": '&', "lt": '<', "gt": '>', "apos": '\'', "num": '#', "semi": ';'}

// EntityNumOK 는 숫자 엔티티의 값 n 을 받는지다(정책 B).
func EntityNumOK(n int) bool {
	if n < 0x21 || n > unicode.MaxRune || n >= 0xd800 && n <= 0xdfff {
		return false
	}
	r := rune(n)
	return !unicode.In(r, unicode.Cc, unicode.Cf, unicode.White_Space, unicode.Noncharacter_Code_Point)
}

// entityRune 은 표식 하나(num 은 정수 표식, name 은 그 안의 글)를 글자로 푼다.
func entityRune(num bool, name string) (rune, bool) {
	if !num {
		r, ok := entityNames[name]
		return r, ok
	}
	if len(name) > 7 { // 정수 표식의 안은 ASCII 숫자뿐이다(EncodeEntities 의 \w+ 가운데 \d+)
		return 0, false
	}
	n, err := strconv.Atoi(name)
	if err != nil || !EntityNumOK(n) {
		return 0, false
	}
	return rune(n), true
}

// RestoreEntities 는 글의 엔티티 표식을 정책 B 로 한 번 되돌린다. 받지 않는 표식이 있으면 bad 에 그 엔티티(`#…;`)를 돌려준다.
func RestoreEntities(s string) (v, bad string) {
	var b strings.Builder
	last := 0
	for _, ix := range reMark.FindAllStringSubmatchIndex(s, -1) {
		num, name := ix[3] > ix[2], s[ix[4]:ix[5]]
		r, ok := entityRune(num, name)
		if !ok {
			return "", "#" + name + ";"
		}
		b.WriteString(s[last:ix[0]])
		b.WriteRune(r)
		last = ix[1]
	}
	b.WriteString(s[last:])
	return b.String(), ""
}

// EntityProblem 은 정책 B 가 받지 않는 엔티티(bad — `#…;`)의 경고 문구다(줄 번호 없음).
func EntityProblem(what, bad string) string {
	return fmt.Sprintf("%s의 엔티티 %s 는 이 판이 받지 않음 — 이름은 quot·amp·lt·gt·apos·num·semi, 숫자는 공백·제어·서식이 아닌 글자(#169;)만 쓴다(그 밖의 글자는 그대로 — ©, 특수 공백은 쓰지 못 함)", what, Clip(bad))
}

// XMLProblem 은 v 가 XmlChars(SVG 쓰기 경로 Esc 와 Result.Caption)를 지나며 바뀌면 그 까닭(줄 번호 없는 경고 문구)이다 — XML 1.0
// Char 밖의 글자(원문에 실제로 든 U+FFFE·U+FFFF 등)는 그 경로가 U+FFFD 로 바꾸므로, 받은 글자와 SVG 글자가 같도록 그 줄을
// 그리지 않는다. 종류마다의 개별 검사와 출력 관문(각 종류의 gate)이 같은 문구를 쓴다. XmlChars 는 바꾸지 않고 묻기만 한다.
func XMLProblem(v, what string) string {
	if XmlChars(v) == v {
		return ""
	}
	for _, r := range v {
		if c := string(r); XmlChars(c) != c {
			return fmt.Sprintf("%s에 SVG(XML)가 담지 못하는 글자(U+%04X)가 있음 — 이 판은 그 줄을 그리지 않는다(그 글자를 지운다)", what, r)
		}
	}
	return fmt.Sprintf("%s에 SVG(XML)가 담지 못하는 글자가 있음 — 이 판은 그 줄을 그리지 않는다", what)
}

// spaceProblem 은 보이는 글 안의 연속 빈칸·탭 경고 문구다(state 의 label 과 같은 문구).
func spaceProblem(what string) string {
	return fmt.Sprintf("%s 안에 연속 빈칸이나 탭이 있음 — 이 판은 글 사이 빈칸을 한 칸씩만 그린다", what)
}

// oddProblem 은 글에 ASCII 밖 공백·서식 문자(OddSpace — NBSP·EM SPACE·U+200B 따위)가 있으면 그 까닭이다. 이 파일의 함수들은
// 「공백은 ASCII 빈칸·탭(·acc 의 LF)뿐」을 전제로 원본 JS \s 규칙(줄 머리 정리 \n\s+, 줄바꿈 자리)을 ASCII 로 옮겼다. 각 종류의 줄
// 루프가 그 줄을 먼저 막지만(DESIGN 2장 줄 루프 차례), 호출자 전제에 기대지 않게 함수가 스스로도 거부한다(itda-hyve#409 ④-2 후속 리뷰
// 1회차 [2]·[3] — 심층 방어). 거부 범위는 OddSpace 그대로다 — 공백이 아닌 다른 서식 문자(U+00AD SHY·U+200E 따위)는 Go·JS 어느
// 쪽도 공백으로 보지 않아 줄바꿈·정리에서 바뀌지 않으므로 글자로 받는다(state 가 받던 그대로 — 바이트 불변).
func oddProblem(s, what string) string {
	if r, ok := OddSpace(s); ok {
		return fmt.Sprintf("%s에 ASCII 밖 공백·서식 문자(U+%04X)가 있음 — 이 판은 그 줄을 그리지 않는다(일반 빈칸으로 바꿔 쓴다)", what, r)
	}
	return ""
}

// Label 은 보이는 라벨 하나(엔티티 표식이 든 글 — 줄 단위 EncodeEntities 를 지난 것, 흐름도·간트는 라벨 글에 EncodeEntities 를
// 먼저 씌워 넘긴다)를 정책 B 로 읽는다. 문법상 앞뒤 빈칸·탭을 지운 뒤 안쪽에 제어 문자(탭 밖의 C0·DEL)·탭·연속 빈칸이 있으면
// 받지 않는다(원본 화면의 배치를 확인하지 못한 꼴 — 접근 방식 리뷰 5장). 이 제어 문자 검사는 C0·DEL 만 보고 C1(U+0080~U+009F)은
// 보지 않는다 — C1 이 든 입력의 수락·표시는 보장 밖이다(지금 동작은 DESIGN 2장 관찰 표). br 이면 <br>(<br/>·<br />, 대소문자 무시)을 줄바꿈(\n)
// 으로 받는다 — 그 앞뒤 빈칸 한 칸은 줄바꿈이 대신한다(옛 DecodeLabel 과 같다, 축소하지 않는다). 엔티티 표식은 줄마다
// RestoreEntities 로 한 번 — 되돌린 `#lt;br#gt;` 는 <br> 글자일 뿐 다시 줄바꿈으로 읽지 않는다. 되돌린 글자는 공백이 아니므로
// (White_Space 제외) 값의 줄에는 앞뒤·연속 빈칸이 없다. 맨 앞·끝의 빈 줄은 지운다(옛 DecodeLabel 과 같다). msg 가 비어 있지 않으면
// 받지 않는 까닭이다(줄 번호 없음 — 부르는 쪽이 그 줄로 알린다).
func Label(s, what string, br bool) (v, msg string) {
	s = strings.Trim(s, " \t")
	for _, r := range s {
		if r < 0x20 && r != '\t' || r == 0x7f {
			return "", what + "에 제어 문자가 있음"
		}
	}
	if m := oddProblem(s, what); m != "" {
		return "", m
	}
	if strings.Contains(s, "\t") || strings.Contains(s, "  ") {
		return "", spaceProblem(what)
	}
	segs := []string{s}
	if br {
		segs = splitBR(s)
	}
	for i, seg := range segs {
		d, bad := RestoreEntities(strings.Trim(seg, " "))
		if bad != "" {
			return "", EntityProblem(what, bad)
		}
		segs[i] = d
	}
	v = strings.Trim(strings.Join(segs, "\n"), "\n")
	if m := XMLProblem(v, what); m != "" {
		return "", m
	}
	if utf8.RuneCountInString(v) > MaxLabel {
		return "", fmt.Sprintf("%s이 %d 자를 넘음", what, MaxLabel)
	}
	return v, ""
}

// splitBR 은 글을 <br>(<br/>·<br />, 대소문자 무시)에서 나눈다.
func splitBR(s string) []string {
	var out []string
	from := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '<' {
			continue
		}
		for _, t := range brTags {
			if len(s)-i >= len(t) && strings.EqualFold(s[i:i+len(t)], t) {
				out = append(out, s[from:i])
				i += len(t) - 1
				from = i + 1
				break
			}
		}
	}
	return append(out, s[from:])
}

// OneLine 은 한 줄로 그리는 자리(gantt 라벨·제목 줄·흐름도 subgraph 제목·캡션)의 값이다 — 줄바꿈(<br>)은 빈칸 하나가 된다.
// 사이가 빈 줄(<br><br>)은 빈칸 둘이 되므로 받지 않는다(Label 의 연속 빈칸과 같은 까닭).
func OneLine(v, what string) (string, string) {
	if m := oddProblem(v, what); m != "" {
		return "", m
	}
	if strings.Contains(v, "\n\n") {
		return "", fmt.Sprintf("%s의 <br> 사이가 비어 있음 — 한 줄로 그리는 자리라 빈칸 둘이 된다", what)
	}
	return strings.ReplaceAll(v, "\n", " "), ""
}

// AccSet 은 접근성 값(원본처럼 다듬은 것 — AccValue·AccBlock)의 엔티티 표식만 RestoreEntities 로 한 번 되돌린다 — 표식 밖의 글
// (`#+65;`)은 그대로고, 되돌린 `#lt;br#gt;` 는 `<br>` 글자일 뿐 줄바꿈으로 다시 해석하지 않는다. 탭·줄바꿈 밖의 제어 문자(unicode.Cc —
// C0·DEL·C1)와 맨 &·<·> 는 받지 않는다 — 원본은 값을 HTML 정제(commonDb setAccTitle·setAccDescription 의 sanitizeText → DOMPurify)에 넘겨 그
// 글자가 &amp; 따위 글로 바뀔 수 있다(글자는 엔티티 #amp;·#lt;·#gt; 로 — 표식은 정제 뒤 SVG 정리 단계에서 풀린다).
func AccSet(v, kw string) (string, string) {
	if m := oddProblem(v, kw); m != "" {
		return "", m
	}
	for _, r := range v {
		switch {
		case r != '\t' && r != '\n' && unicode.Is(unicode.Cc, r):
			return "", fmt.Sprintf("%s에 제어 문자(U+%04X)가 있음", kw, r)
		case r == '&' || r == '<' || r == '>':
			return "", fmt.Sprintf("%s에 %s 를 그대로 쓰지 못 함 — 원본은 접근성 값을 HTML 정제에 넘겨 글이 바뀔 수 있다(글자는 엔티티 #amp;·#lt;·#gt; 로)", kw, Quote(string(r)))
		}
	}
	s, bad := RestoreEntities(v)
	if bad != "" {
		return "", EntityProblem(kw, bad)
	}
	if m := XMLProblem(s, kw); m != "" {
		return "", m
	}
	if utf8.RuneCountInString(s) > MaxLabel {
		return "", fmt.Sprintf("%s 값이 %d 자를 넘음", kw, MaxLabel)
	}
	return s, ""
}

// AccValue 는 accTitle:·accDescr: 의 값(콜론 뒤 줄 끝까지)을 읽는다. 원본은 값을 trim 한다(각 종류 jison 의 acc_title_value
// ·acc_descr_value 규칙 — $2.trim()) — 안쪽 빈칸·탭은 그대로다. 빈 값은 받지 않는다(원본 accTitle\s*":"\s* 의 \s* 가 줄바꿈을
// 넘어 다음 줄을 값으로 먹는다).
func AccValue(v, kw string) (string, string) {
	v = strings.Trim(v, " \t")
	if v == "" {
		return "", fmt.Sprintf("%s 뒤에 글이 없음", kw)
	}
	return AccSet(v, kw)
}

// reNLSpace 는 원본 setAccDescription 의 replace(/\n\s+/g, '\n') 이다(commonDb.ts — 줄 머리 빈칸과 빈 줄을 지운다). 원문 줄의
// 제어 문자는 AccSet 이, ASCII 밖 공백(NBSP 따위 — JS \s 에 든다)은 Add 가 막으므로 \s 는 빈칸·탭·줄바꿈이다.
var reNLSpace = regexp.MustCompile("\n[ \t\n]+")

// AccBlock 은 accDescr { … } 의 줄을 모은다(선언마다 새 값 — 마지막 선언이 이긴다).
type AccBlock struct{ lines []string }

// Add 는 블록의 한 줄(여는 줄의 { 뒤 포함 — 엔티티 전처리를 지나 앞 빈칸만 지운 것, 끝 빈칸은 그대로)을 더한다. 닫는 } 는 그 줄
// 끝에만 온다(뒤에 글이 있으면 원본은 } 뒤를 다음 문장으로 읽는다 — 이 판은 받지 않는다). 닫혔으면 done 이 참이고 v 는 원본처럼
// 다듬은 값이다 — 블록 글(acc_descr_multiline_value [^\}]*)을 통째로 trim 하고 줄 머리 빈칸·빈 줄을 지운다(reNLSpace). 줄바꿈과
// 줄 끝 빈칸, 안쪽 빈칸·탭은 그대로다. 빈 블록이면 v 가 "" 이다 — 부르는 쪽이 앞 설명을 그대로 둔다(원본 \s* 가 { 뒤 빈칸·줄바꿈을
// 먹고 } 규칙은 상태만 닫는다). 엔티티 표식은 v 에 그대로다(AccSet 이 되돌린다).
func (b *AccBlock) Add(t string) (done bool, v, msg string) {
	if m := oddProblem(t, "accDescr"); m != "" {
		return true, "", m
	}
	before, after, closed := strings.Cut(t, "}")
	b.lines = append(b.lines, strings.TrimLeft(before, " \t"))
	if !closed {
		return false, "", ""
	}
	if strings.Trim(after, " \t") != "" {
		return true, "", fmt.Sprintf("accDescr { … } 의 닫는 } 뒤에 글이 있음(%s) — 이 판은 } 를 줄 끝에만 둔다", Clip(after))
	}
	v = reNLSpace.ReplaceAllString(strings.Trim(strings.Join(b.lines, "\n"), " \t\n"), "\n")
	b.lines = nil
	return true, v, ""
}

// Reset 은 새 블록을 연다.
func (b *AccBlock) Reset() { b.lines = nil }

// Caption 은 frontmatter title(YAML 이 읽은 글 — fm.Raw)을 정책 B 로 캡션 한 줄로 읽는다. 원본은 frontmatter 를 엔티티 전처리
// (Diagram.fromText 의 encodeEntities) 전에 떼어 내므로 title 의 `#…;` 는 표식이 아니다 — 되돌릴 표식이 없어 글자 그대로다.
// 줄바꿈(YAML 블록 스칼라의 줄)과 <br> 은 빈칸 하나가 된다(옛 DecodeLabel 과 같다). 앞뒤 빈칸·탭·줄바꿈을 지운 뒤 안쪽에 제어
// 문자(C0·DEL — C1 은 이 검사가 보지 않는다)·탭·연속 빈칸·빈 줄이 있으면 받지 않는다(보이는 라벨과 같은 공백 정책). XML 이 담지 못하는 글자는 여기서 보지 않는다 —
// 본문을 다 읽은 뒤 각 종류의 출력 관문(gate)이 title 줄로 알린다(본문 오류가 먼저). state 도 이 함수를 쓴다(2026-10-31 사용자
// 결정 「여섯 종류 통일」).
func Caption(raw string) (string, string) {
	const what = "frontmatter title"
	s := strings.Trim(raw, " \t\n")
	for _, r := range s {
		if r < 0x20 && r != '\t' && r != '\n' || r == 0x7f {
			return "", what + "에 제어 문자가 있음"
		}
	}
	if m := oddProblem(s, what); m != "" {
		return "", m
	}
	if strings.Contains(s, "\t") || strings.Contains(s, "  ") {
		return "", spaceProblem(what)
	}
	var segs []string
	for ln := range strings.SplitSeq(s, "\n") {
		for _, seg := range splitBR(ln) {
			segs = append(segs, strings.Trim(seg, " "))
		}
	}
	v := strings.Trim(strings.Join(segs, "\n"), "\n")
	v, m := OneLine(v, what)
	if m != "" {
		return "", m
	}
	if utf8.RuneCountInString(v) > MaxLabel {
		return "", fmt.Sprintf("라벨이 %d 자를 넘음", MaxLabel)
	}
	return v, ""
}

// AriaRefs 는 SVG 첫 요소(svg)의 aria-label 값 안의 탭·LF·CR 을 문자 참조(&#9;·&#10;·&#13;)로 바꾼다 — XML 은 속성값의 리터럴
// 탭·줄바꿈을 빈칸으로 정규화하므로(XML 1.0 3.3.3) 받은 accTitle 의 탭이 XML 로 읽어도 탭이게 한다(④-2 리뷰 5회차 [2]). 여섯
// 종류 모두 사용자 글자가 드는 속성은 aria-label 하나다(나머지 속성은 고정 글·수). Esc 는 바꾸지 않는다(요소 글자의 바이트 불변).
// 값은 Esc 를 지나 「"」가 &quot; 이므로 첫 ` aria-label="` 뒤 첫 「"」까지가 그 값이다(앞의 속성은 고정 글·수).
func AriaRefs(svg string) string {
	const key = ` aria-label="`
	i := strings.Index(svg, key)
	if i < 0 {
		return svg
	}
	i += len(key)
	j := i + strings.IndexByte(svg[i:], '"')
	return svg[:i] + attrWS.Replace(svg[i:j]) + svg[j:]
}

var attrWS = strings.NewReplacer("\t", "&#9;", "\n", "&#10;", "\r", "&#13;")

// TextFunc 는 출력 관문이 사용자 글 하나마다 부르는 함수다 — field 는 「형.필드」(TestGateCoversAllText 가 모델의 글 필드와
// 견준다), what 은 경고 문구의 이름, line 은 그 값이 나온 줄이다.
type TextFunc func(field, what, v string, line int) error

// Gate 는 출력 관문이다 — each(종류마다의 필드 목록 한곳 eachText)가 넘기는 내보낼 사용자 글자가 모두 XmlChars(SVG 쓰기 경로 Esc 와
// Result.Caption)를 지나도 그대로인지 보고, 바뀌는 값이 있으면 그 값이 나온 줄의 *Error 다(받은 글자 = 출력 글자 — ④-2 리뷰
// 3·4회차, itda-hyve#208 규칙으로 구조로 닫음). 각 종류의 Parse 가 본문을 다 읽은 뒤 한 번 부른다(본문 오류가 먼저다).
func Gate(each func(TextFunc) error) error {
	return each(func(_, what, v string, line int) error {
		if msg := XMLProblem(v, what); msg != "" {
			return &Error{Line: line, Msg: msg}
		}
		return nil
	})
}
