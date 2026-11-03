package classdiag

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/itda-work/itda-doc/diagram"
)

// 제네릭 ~T~ (itda-hyve#409 ④-3 후속). 원본(mermaid@12.2.0)의 두 자리를 옮긴다.
//
//	이름   classDiagram.jison:106–108 — <*>"~" 가 generic 상태를 열고, 그 상태에서는 [^~]*(GENERICTYPE)·[~](닫기)만 쓴다.
//	       문법 className: alphaNumToken GENERICTYPE { $$=$1+'~'+$2+'~' } — 제네릭은 이름 끝에 한 번만 붙는다. classDb
//	       splitClassNameAndType 는 id.split('~') 의 [0] 을 이름, [1] 을 type 으로 쓰고, addClass 는 이미 있는 이름이면 아무것도
//	       하지 않는다(처음 만든 자리의 type 이 남는다). 상자 제목은 text `이름&lt;type&gt;` — 「이름<type>」으로 그려진다.
//	멤버   클래스 몸(MEMBER [^{}\n]*)·`이름 : 멤버`(LABEL ":"[^:\n;]+)는 ~ 를 글자 그대로 담는다. ClassMember 가 이름·인자·반환형
//	       마다 common.ts parseGenericTypes 를 씌운다(parseGenericTypes 아래 — 글자 그대로 옮김).
//
// 이 판은 원본과 값이 같다고 확인한 좁은 꼴만 받는다 — 이름은 한 겹 `이름~인자~`(인자는 이름 글자 낱말의 쉼표 목록 — genericArg),
// 멤버는 ~ 짝을 괄호처럼 읽은 값(genericText 의 「직관적 짝」)이 원본 parseGenericTypes 결과와 같은 꼴만이다.

// genericKeywords 는 generic 상태에서도 [^~]*(jison:107)보다 먼저 맞는 낱말 규칙이다 — <*> 규칙 가운데 jison:89–94·104 의
// "cssClass"·"callback"·"link"·"click"·"note for"·"note"·"href"(lex-parser 가 \b 를 붙인다). 인자 머리에서만 문제다 — 그 뒤는
// [^~]* 가 한 토큰으로 먹는다. 같은 자리의 <*>["](jison:62)·"<<"·">>"(jison:95·96)는 인자 글자 검사가 막는다.
var genericKeywords = []string{"cssClass", "callback", "link", "click", "note", "href"}

// className 은 줄 t 의 이름 자리 t[s:e](gnamePat — 이름 꼴 뒤에 ~…~ 하나가 붙을 수 있다)를 이름과 제네릭 인자로 나눠 본다.
func (p *parser) className(t string, s, e int) (name, typ string, err error) {
	i := strings.IndexByte(t[s:e], '~')
	if i < 0 {
		return t[s:e], "", p.nameOK(t, s, e, "클래스 이름")
	}
	b := s + i // 여는 ~ — 이름 뒤 글자(키워드 낱말 경계)는 ~ 다
	if err := p.nameOK(t, s, b, "클래스 이름"); err != nil {
		return "", "", err
	}
	name, typ = t[s:b], t[b+1:e-1]
	if msg := genericArg(typ); msg != "" {
		return "", "", p.fail("클래스 %s 의 제네릭 인자 %s %s", diagram.Quote(diagram.Clip(name)), diagram.Quote(diagram.Clip(typ)), msg)
	}
	if utf8.RuneCountInString(name)+utf8.RuneCountInString(typ)+2 > MaxLabel {
		return "", "", p.fail("클래스 제목 %s<…> 가 %d 자를 넘음", diagram.Quote(diagram.Clip(name)), MaxLabel)
	}
	return name, typ, nil
}

// genericArg 는 이름 제네릭의 인자(~ 사이 글)가 받는 꼴인지 본다 — 받지 않으면 그 까닭이다. 받는 꼴은 이름 글자(ASCII 낱말
// 글자·원본 UNICODE_TEXT 글자) 낱말을 쉼표로 이은 것(쉼표 뒤 빈칸 한 칸까지)이다. _ 는 글자 사이에만(상자 제목은 마크다운).
func genericArg(arg string) string {
	if arg == "" {
		// jison:106 <generic>[~] 가 107 [^~]* 보다 앞이라 ~~ 는 토큰 없이 닫힌다 — 원본은 인자 없는 이름으로 읽는다
		return "가 비어 있음 — 빈 ~~ 는 받지 않는다(원본은 인자 없는 이름으로 읽는다)"
	}
	for _, kw := range genericKeywords {
		if strings.HasPrefix(arg, kw) && (len(arg) == len(kw) || !isWordByte(arg[len(kw)])) {
			return "가 키워드 " + diagram.Quote(kw) + " 로 시작함 — 원본 lexer 는 generic 상태에서도 이 낱말을 먼저 읽는다"
		}
	}
	for k, item := range strings.Split(arg, ",") {
		if k > 0 {
			item = strings.TrimPrefix(item, " ")
		}
		if item == "" {
			return "의 꼴이 다름 — 이름 글자 낱말을 쉼표로 잇는다(`T`·`K, V`, 쉼표 뒤 빈칸 한 칸까지)"
		}
		for _, r := range item {
			switch {
			case r < 0x80 && isWordByte(byte(r)):
			case r >= 0x80 && r <= 0xffff && inUnicodeText(r):
			default:
				return "에 받지 않는 글자 " + diagram.Quote(string(r)) + " 가 있음 — 이름 글자 낱말을 쉼표로 잇는다(`T`·`K, V`, 한 겹만)"
			}
		}
		if !underscoreOK(item) {
			return "의 _ 가 글자 사이가 아님 — 원본은 상자 제목을 마크다운으로 그려 기울임이 될 수 있다"
		}
	}
	return ""
}

// isGenName 은 멤버 제네릭의 짝을 가르는 「이름 글자」다(글자·숫자·_·엔티티 자리표).
func isGenName(r rune) bool {
	return r == entPH || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsNumber(r)
}

// genericText 는 멤버의 한 조각(이름·인자·반환형 — 원본이 parseGenericTypes 를 따로 씌우는 단위)의 ~ 를 <·> 로 바꾼다. 받는
// 꼴은 ~ 마다 다음 둘 가운데 하나인 것이다.
//
//	여는 ~  앞이 이름 글자, 뒤가 이름 글자          → <
//	닫는 ~  앞이 이름 글자나 ~, 뒤가 끝·~·쉼표·빈칸 → >(열린 짝이 있어야 한다)
//
// 끝에서 열린 짝이 없어야 하고, 이렇게 괄호처럼 짝지은 값(직관적 짝)이 원본 parseGenericTypes 의 값과 같아야 한다 — 원본은
// 쉼표 덩어리마다 「맨 앞 ~ 와 맨 뒤 ~」를 짝짓기 때문에 `a~T~ b~U~` 를 a<T< b>U> 로 읽는다. 그런 꼴은 받지 않는다(ok 거짓).
func genericText(part []rune) ([]rune, bool) {
	if indexRune(part, '~') < 0 {
		return part, true
	}
	out := append([]rune(nil), part...)
	depth, n := 0, len(part)
	for i, c := range part {
		if c != '~' {
			continue
		}
		prevName := i > 0 && isGenName(part[i-1])
		nextName := i+1 < n && isGenName(part[i+1])
		nextEnd := i+1 == n || part[i+1] == '~' || part[i+1] == ',' || part[i+1] == ' '
		switch {
		case prevName && nextName:
			out[i] = '<'
			depth++
		case (prevName || i > 0 && part[i-1] == '~') && nextEnd && depth > 0:
			out[i] = '>'
			depth--
		default:
			return nil, false
		}
	}
	if depth != 0 || parseGenericTypes(string(part)) != string(out) {
		return nil, false
	}
	return out, true
}

// parseGenericTypes 는 원본 common.ts parseGenericTypes·shouldCombineSets·processSet·countOccurrence 를 글자 그대로 옮긴
// 것이다 — 입력을 쉼표에서 나누되(split(/(,)/) — 쉼표도 조각으로 남는다) ~ 가 하나씩인 앞뒤 조각은 다시 잇고, 조각마다 ~ 가 둘
// 이상이면 맨 앞 ~ 를 <, 맨 뒤 ~ 를 > 로 바꾸기를 둘이 만날 때까지 되풀이한다(~ 수가 홀수이고 맨 앞이 ~ 면 그 ~ 는 빼 두었다
// 되붙인다). 원본은 JS 문자열(코드 포인트 배열 [...input])을 다루지만 ~·쉼표는 BMP 한 글자라 룬으로 옮겨도 자리가 같다.
func parseGenericTypes(input string) string {
	parts := strings.Split(input, ",")
	sets := make([]string, 0, 2*len(parts))
	for k, s := range parts {
		if k > 0 {
			sets = append(sets, ",")
		}
		sets = append(sets, s)
	}
	var out []string
	for i := 0; i < len(sets); i++ {
		this := sets[i]
		if this == "," && i > 0 && i+1 < len(sets) {
			prev, next := sets[i-1], sets[i+1]
			if strings.Count(prev, "~") == 1 && strings.Count(next, "~") == 1 { // shouldCombineSets
				this = prev + "," + next
				i++
				out = out[:len(out)-1]
			}
		}
		out = append(out, processSet(this))
	}
	return strings.Join(out, "")
}

func processSet(input string) string {
	tildes := strings.Count(input, "~")
	if tildes <= 1 {
		return input
	}
	starting := false
	if tildes%2 != 0 && strings.HasPrefix(input, "~") {
		input, starting = input[1:], true
	}
	chars := []rune(input)
	first, last := indexRune(chars, '~'), lastIndexRune(chars, '~')
	for first != -1 && last != -1 && first != last {
		chars[first], chars[last] = '<', '>'
		first, last = indexRune(chars, '~'), lastIndexRune(chars, '~')
	}
	if starting {
		return "~" + string(chars)
	}
	return string(chars)
}

func lastIndexRune(r []rune, c rune) int {
	for i := len(r) - 1; i >= 0; i-- {
		if r[i] == c {
			return i
		}
	}
	return -1
}
