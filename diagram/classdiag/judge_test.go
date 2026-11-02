package classdiag

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/itda-work/itda-doc/diagram"
)

// 독립 판정기(#409 ④-3 — er ④-1·state ④-2 judge_test.go 꼴). 허용 꼴과 원본 lexer·DB 의 함정을 이 파일 안에 따로 옮겨 적은
// 손 스캐너로, 무작위 원문마다 받아들일지와 받아들이면 그 값(클래스 차례·이름·주석, 멤버 차례·종류(속성/메서드)·가시성·표시 글·
// 분류자, 관계 양끝·모양 둘·선 종류·라벨·양끝 수량 글, 방향, 접근성 값, 캡션)을 스스로 구해 Parse 와 견준다. 구현의 정규식·
// 상수·도우미(nameOK·member·labelOK·keywords·reRel·unicodeText …)는 쓰지 않는다 — 공통 패키지의 전처리(EncodeEntities·
// DirectiveProblem·OddSpace)도 쓰지 않고 따로 적는다. 이름 글자 표는 원본 jison 의 UNICODE_TEXT 규칙 글을 그대로 옮겨(jUnicodeSrc)
// 이 파일 안에서 푼다(구현의 unicode.go 표와 따로).
//
// 판정기가 옮긴 원본 규칙(mermaid@12.2.0 classDiagram.jison·classDb.ts·classTypes.ts):
//   - 전처리: CRLF → LF(줄 중간 CR 은 받지 않음), 줄 머리 %% 주석 줄 지움(클래스 몸 안에서도 — 글 없는 맨 %% 는 뿌리에서만
//     lexer 주석 규칙이 건너뛰고, accDescr { } 안은 설명 글·클래스 몸 안은 멤버가 되어 받지 않음), %%{…}%% 지시 지움,
//     style/classDef …:…#…; 의 끝 ; 자르기, 엔티티 #\w+; 지키기
//   - detector: /^\s*classDiagram/ — 머리는 정확히 classDiagram·classDiagram-v2
//   - lexer: 대소문자 구분(case-insensitive 옵션 없음), 규칙 차례의 첫 일치. lex-parser 가 낱말 글자로 끝나는 규칙에 \b 를
//     붙인다 — 키워드(accTitle·accDescr·classDiagram·call·style·classDef·namespace·class·cssClass·callback·link·click·note·href·
//     _self·_blank·_parent·_top·\s*o)는 뒤가 ASCII 낱말 글자가 아니면 이름(ALPHA \w+·UNICODE_TEXT 한 글자)보다 먼저 맞는다.
//     UNICODE_TEXT 는 한 글자씩이라 ASCII 밖 글자 뒤마다 새 토큰이 시작된다. .*direction\s+(TB|BT|RL|LR) 은 줄 어디서든.
//     ; 규칙은 없다. LABEL 은 ":"[^:\n;]+, ::: 는 STYLE_SEPARATOR, " 는 어느 상태에서든 글(STR) 시작, ~ 는 제네릭 시작
//   - DB: 클래스는 처음 나온 차례(관계는 왼끝 다음 오른끝, 멤버 줄도 만든다, <<주석>> 이름 은 없는 클래스면 멈춘다), 멤버는
//     원문 차례, 주석은 첫 것만 그린다, 방향은 마지막, 관계 라벨·수량 글은 다듬는다(수량 글 none 은 「없음」)
//   - 멤버(ClassMember): ) 가 첫 글자 뒤에 있으면 메서드, 가시성은 첫 글자 + - # ~, 메서드 ([#+~-])?(.+)\((.*)\)([\s$*])?(.*)([$*])?,
//     속성은 끝 글자 $ * 가 분류자, 이름 앞 빈칸 하나는 남는다(Preserve one space only), 표시 글은 가시성 + 이름 + (인자) + 「 : 반환형」
//   - 제네릭(jison:106–108·251–258, classDb splitClassNameAndType·addClass, common.ts parseGenericTypes): ~ 는 generic 상태를
//     열고 그 상태는 [^~]* 와 닫는 ~ 만 — 다만 그보다 앞의 <*> 규칙(" · cssClass · callback · link · click · note for · note ·
//     << · >> · href)이 인자 머리에서 먼저 맞는다. 이름 끝에 한 번(alphaNumToken GENERICTYPE), 클래스는 split('~')[0], 인자는
//     [1](처음 만든 자리의 것), 제목은 「이름<인자>」. 멤버는 MEMBER·LABEL 이 ~ 를 글자로 담고 이름·인자·반환형마다
//     parseGenericTypes(쉼표 조각, ~ 하나씩인 앞뒤 조각은 다시 이음, 조각마다 맨 앞·맨 뒤 ~ 를 <·> 로 되풀이)
//   - 마크다운: 이름·멤버·관계 라벨은 markdown 으로 그려진다 — _ 가 글자 사이가 아니면 기울임, -·+·= 나 「수.」로 시작하면 목록·
//     구분선(멤버는 가시성 앞에 \ 를 붙여 지킨다), *·`·<·>·[·] 등은 서식. 수량 글은 markdown 이 아니다(* 를 글자로)

// jUnicodeSrc 는 원본 classDiagram.jison 145–205 줄(UNICODE_TEXT 규칙의 글자 모임 — 줄 머리 <*> 만 뺐다)을 글자 그대로 옮긴 것이다.
const jUnicodeSrc = `[\u00AA\u00B5\u00BA\u00C0-\u00D6\u00D8-\u00F6]|
[\u00F8-\u02C1\u02C6-\u02D1\u02E0-\u02E4\u02EC\u02EE\u0370-\u0374\u0376\u0377]|
[\u037A-\u037D\u0386\u0388-\u038A\u038C\u038E-\u03A1\u03A3-\u03F5]|
[\u03F7-\u0481\u048A-\u0527\u0531-\u0556\u0559\u0561-\u0587\u05D0-\u05EA]|
[\u05F0-\u05F2\u0620-\u064A\u066E\u066F\u0671-\u06D3\u06D5\u06E5\u06E6\u06EE]|
[\u06EF\u06FA-\u06FC\u06FF\u0710\u0712-\u072F\u074D-\u07A5\u07B1\u07CA-\u07EA]|
[\u07F4\u07F5\u07FA\u0800-\u0815\u081A\u0824\u0828\u0840-\u0858\u08A0]|
[\u08A2-\u08AC\u0904-\u0939\u093D\u0950\u0958-\u0961\u0971-\u0977]|
[\u0979-\u097F\u0985-\u098C\u098F\u0990\u0993-\u09A8\u09AA-\u09B0\u09B2]|
[\u09B6-\u09B9\u09BD\u09CE\u09DC\u09DD\u09DF-\u09E1\u09F0\u09F1\u0A05-\u0A0A]|
[\u0A0F\u0A10\u0A13-\u0A28\u0A2A-\u0A30\u0A32\u0A33\u0A35\u0A36\u0A38\u0A39]|
[\u0A59-\u0A5C\u0A5E\u0A72-\u0A74\u0A85-\u0A8D\u0A8F-\u0A91\u0A93-\u0AA8]|
[\u0AAA-\u0AB0\u0AB2\u0AB3\u0AB5-\u0AB9\u0ABD\u0AD0\u0AE0\u0AE1\u0B05-\u0B0C]|
[\u0B0F\u0B10\u0B13-\u0B28\u0B2A-\u0B30\u0B32\u0B33\u0B35-\u0B39\u0B3D\u0B5C]|
[\u0B5D\u0B5F-\u0B61\u0B71\u0B83\u0B85-\u0B8A\u0B8E-\u0B90\u0B92-\u0B95\u0B99]|
[\u0B9A\u0B9C\u0B9E\u0B9F\u0BA3\u0BA4\u0BA8-\u0BAA\u0BAE-\u0BB9\u0BD0]|
[\u0C05-\u0C0C\u0C0E-\u0C10\u0C12-\u0C28\u0C2A-\u0C33\u0C35-\u0C39\u0C3D]|
[\u0C58\u0C59\u0C60\u0C61\u0C85-\u0C8C\u0C8E-\u0C90\u0C92-\u0CA8\u0CAA-\u0CB3]|
[\u0CB5-\u0CB9\u0CBD\u0CDE\u0CE0\u0CE1\u0CF1\u0CF2\u0D05-\u0D0C\u0D0E-\u0D10]|
[\u0D12-\u0D3A\u0D3D\u0D4E\u0D60\u0D61\u0D7A-\u0D7F\u0D85-\u0D96\u0D9A-\u0DB1]|
[\u0DB3-\u0DBB\u0DBD\u0DC0-\u0DC6\u0E01-\u0E30\u0E32\u0E33\u0E40-\u0E46\u0E81]|
[\u0E82\u0E84\u0E87\u0E88\u0E8A\u0E8D\u0E94-\u0E97\u0E99-\u0E9F\u0EA1-\u0EA3]|
[\u0EA5\u0EA7\u0EAA\u0EAB\u0EAD-\u0EB0\u0EB2\u0EB3\u0EBD\u0EC0-\u0EC4\u0EC6]|
[\u0EDC-\u0EDF\u0F00\u0F40-\u0F47\u0F49-\u0F6C\u0F88-\u0F8C\u1000-\u102A]|
[\u103F\u1050-\u1055\u105A-\u105D\u1061\u1065\u1066\u106E-\u1070\u1075-\u1081]|
[\u108E\u10A0-\u10C5\u10C7\u10CD\u10D0-\u10FA\u10FC-\u1248\u124A-\u124D]|
[\u1250-\u1256\u1258\u125A-\u125D\u1260-\u1288\u128A-\u128D\u1290-\u12B0]|
[\u12B2-\u12B5\u12B8-\u12BE\u12C0\u12C2-\u12C5\u12C8-\u12D6\u12D8-\u1310]|
[\u1312-\u1315\u1318-\u135A\u1380-\u138F\u13A0-\u13F4\u1401-\u166C]|
[\u166F-\u167F\u1681-\u169A\u16A0-\u16EA\u1700-\u170C\u170E-\u1711]|
[\u1720-\u1731\u1740-\u1751\u1760-\u176C\u176E-\u1770\u1780-\u17B3\u17D7]|
[\u17DC\u1820-\u1877\u1880-\u18A8\u18AA\u18B0-\u18F5\u1900-\u191C]|
[\u1950-\u196D\u1970-\u1974\u1980-\u19AB\u19C1-\u19C7\u1A00-\u1A16]|
[\u1A20-\u1A54\u1AA7\u1B05-\u1B33\u1B45-\u1B4B\u1B83-\u1BA0\u1BAE\u1BAF]|
[\u1BBA-\u1BE5\u1C00-\u1C23\u1C4D-\u1C4F\u1C5A-\u1C7D\u1CE9-\u1CEC]|
[\u1CEE-\u1CF1\u1CF5\u1CF6\u1D00-\u1DBF\u1E00-\u1F15\u1F18-\u1F1D]|
[\u1F20-\u1F45\u1F48-\u1F4D\u1F50-\u1F57\u1F59\u1F5B\u1F5D\u1F5F-\u1F7D]|
[\u1F80-\u1FB4\u1FB6-\u1FBC\u1FBE\u1FC2-\u1FC4\u1FC6-\u1FCC\u1FD0-\u1FD3]|
[\u1FD6-\u1FDB\u1FE0-\u1FEC\u1FF2-\u1FF4\u1FF6-\u1FFC\u2071\u207F]|
[\u2090-\u209C\u2102\u2107\u210A-\u2113\u2115\u2119-\u211D\u2124\u2126\u2128]|
[\u212A-\u212D\u212F-\u2139\u213C-\u213F\u2145-\u2149\u214E\u2183\u2184]|
[\u2C00-\u2C2E\u2C30-\u2C5E\u2C60-\u2CE4\u2CEB-\u2CEE\u2CF2\u2CF3]|
[\u2D00-\u2D25\u2D27\u2D2D\u2D30-\u2D67\u2D6F\u2D80-\u2D96\u2DA0-\u2DA6]|
[\u2DA8-\u2DAE\u2DB0-\u2DB6\u2DB8-\u2DBE\u2DC0-\u2DC6\u2DC8-\u2DCE]|
[\u2DD0-\u2DD6\u2DD8-\u2DDE\u2E2F\u3005\u3006\u3031-\u3035\u303B\u303C]|
[\u3041-\u3096\u309D-\u309F\u30A1-\u30FA\u30FC-\u30FF\u3105-\u312D]|
[\u3131-\u318E\u31A0-\u31BA\u31F0-\u31FF\u3400-\u4DB5\u4E00-\u9FCC]|
[\uA000-\uA48C\uA4D0-\uA4FD\uA500-\uA60C\uA610-\uA61F\uA62A\uA62B]|
[\uA640-\uA66E\uA67F-\uA697\uA6A0-\uA6E5\uA717-\uA71F\uA722-\uA788]|
[\uA78B-\uA78E\uA790-\uA793\uA7A0-\uA7AA\uA7F8-\uA801\uA803-\uA805]|
[\uA807-\uA80A\uA80C-\uA822\uA840-\uA873\uA882-\uA8B3\uA8F2-\uA8F7\uA8FB]|
[\uA90A-\uA925\uA930-\uA946\uA960-\uA97C\uA984-\uA9B2\uA9CF\uAA00-\uAA28]|
[\uAA40-\uAA42\uAA44-\uAA4B\uAA60-\uAA76\uAA7A\uAA80-\uAAAF\uAAB1\uAAB5]|
[\uAAB6\uAAB9-\uAABD\uAAC0\uAAC2\uAADB-\uAADD\uAAE0-\uAAEA\uAAF2-\uAAF4]|
[\uAB01-\uAB06\uAB09-\uAB0E\uAB11-\uAB16\uAB20-\uAB26\uAB28-\uAB2E]|
[\uABC0-\uABE2\uAC00-\uD7A3\uD7B0-\uD7C6\uD7CB-\uD7FB\uF900-\uFA6D]|
[\uFA70-\uFAD9\uFB00-\uFB06\uFB13-\uFB17\uFB1D\uFB1F-\uFB28\uFB2A-\uFB36]|
[\uFB38-\uFB3C\uFB3E\uFB40\uFB41\uFB43\uFB44\uFB46-\uFBB1\uFBD3-\uFD3D]|
[\uFD50-\uFD8F\uFD92-\uFDC7\uFDF0-\uFDFB\uFE70-\uFE74\uFE76-\uFEFC]|
[\uFF21-\uFF3A\uFF41-\uFF5A\uFF66-\uFFBE\uFFC2-\uFFC7\uFFCA-\uFFCF]|
[\uFFD2-\uFFD7\uFFDA-\uFFDC]`

// jUniRanges 는 jUnicodeSrc 의 \uXXXX 와 \uXXXX-\uXXXX 를 푼 구간이다.
var jUniRanges = func() [][2]rune {
	var out [][2]rune
	hex := func(s string) rune {
		n, err := strconv.ParseUint(s, 16, 32)
		if err != nil {
			panic(s)
		}
		return rune(n)
	}
	s := jUnicodeSrc
	for i := 0; i+6 <= len(s); {
		if s[i] != '\\' || s[i+1] != 'u' {
			i++
			continue
		}
		lo := hex(s[i+2 : i+6])
		hi := lo
		i += 6
		if i+7 <= len(s) && s[i] == '-' && s[i+1] == '\\' && s[i+2] == 'u' {
			hi = hex(s[i+3 : i+7])
			i += 7
		}
		out = append(out, [2]rune{lo, hi})
	}
	return out
}()

func jUni(r rune) bool {
	for _, g := range jUniRanges {
		if r >= g[0] && r <= g[1] {
			return true
		}
	}
	return false
}

// ---- 판정기 모델

type jMem struct {
	vis  rune
	cls  int // 0 없음 · 1 $ 정적 · 2 * 추상
	text string
	gen  int // 바꾼 제네릭 짝의 가장 깊은 겹(0 = 제네릭 없음) — 생성기 경계 세기
}

type jClass struct {
	name, typ    string
	annot        string
	hasAnnot     bool
	attrs, meths []jMem
}

type jRel struct {
	a, b, t1, t2, line string
	label, c1, c2      string
}

type jModel struct {
	dir                 int // 0 TB · 1 BT · 2 LR · 3 RL
	caption, accT, accD string
	captionLine         int
	classes             []*jClass
	idx                 map[string]int
	rels                []jRel
	header              bool
	body                int // 열린 클래스 몸, -1
	openLine            int
	inAcc               bool
	accLine             int
	accBuf              []string
	raw                 string // 지금 줄(앞 빈칸만 지운 것) — accDescr { … } 의 값은 줄 끝 빈칸까지다
	sawIgnorable        bool
	failLine            int
	typAgain            int // 이미 있는 클래스를 다른 제네릭 인자(또는 인자 없음)로 다시 부른 수 — 생성기 경계 세기
}

func (m *jModel) reject(line int) bool {
	m.failLine = line
	return false
}

// jSpaceRunes 는 원본 JS \s 와 Go unicode.IsSpace 가운데 ASCII 밖의 것 + 폭 없는 서식 문자 — 판정기가 따로 적은 목록이다.
var jSpaceRunes = []rune{0x85, 0xa0, 0x1680, 0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006, 0x2007, 0x2008, 0x2009,
	0x200a, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff, 0x200b, 0x200c, 0x200d, 0x2060}

func jLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func jIsWS(c byte) bool { return c == ' ' || c == '\t' }

func jTrim(s string) string {
	a, b := 0, len(s)
	for a < b && jIsWS(s[a]) {
		a++
	}
	for b > a && jIsWS(s[b-1]) {
		b--
	}
	return s[a:b]
}

func jSkip(s string, i int) int {
	for i < len(s) && jIsWS(s[i]) {
		i++
	}
	return i
}

// jCutSemi 는 원본 encodeEntities 의 앞 두 줄(style·classDef …:\S*#…; 의 마지막 ; 를 지운다)을 손으로 옮긴 것이다.
func jCutSemi(s, kw string) string {
	i := strings.Index(s, kw)
	if i < 0 {
		return s
	}
	last := strings.LastIndexByte(s, ';')
	for j := i + len(kw); j < len(s); j++ {
		if s[j] != ':' {
			continue
		}
		for k := j + 1; k < len(s) && s[k] != ' ' && s[k] != '\t'; k++ {
			if s[k] == '#' && last > k {
				return s[:last] + s[last+1:]
			}
		}
	}
	return s
}

func jWordByte(c byte) bool {
	return c == '_' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// jTok 는 글자 하나 또는 엔티티(#\w+;) 하나다.
type jTok struct {
	r   rune
	ent string
}

func jTokens(s string) []jTok {
	var out []jTok
	for i := 0; i < len(s); {
		if s[i] == '#' {
			j := i + 1
			for j < len(s) && jWordByte(s[j]) {
				j++
			}
			if j > i+1 && j < len(s) && s[j] == ';' {
				out = append(out, jTok{ent: s[i+1 : j]})
				i = j + 1
				continue
			}
		}
		r, n := utf8.DecodeRuneInString(s[i:])
		out = append(out, jTok{r: r})
		i += n
	}
	return out
}

func jTokStr(ts []jTok) string {
	var b strings.Builder
	for _, t := range ts {
		if t.ent != "" {
			b.WriteString("#" + t.ent + ";")
		} else {
			b.WriteRune(t.r)
		}
	}
	return b.String()
}

// 엔티티 정책 B(state 판정기와 같은 규칙을 이 파일에 다시 적는다).
var jNamed = map[string]rune{"quot": 0x22, "amp": 0x26, "lt": 0x3c, "gt": 0x3e, "apos": 0x27, "num": 0x23, "semi": 0x3b}

var jCfRanges = [][2]int{{0xad, 0xad}, {0x600, 0x605}, {0x61c, 0x61c}, {0x6dd, 0x6dd}, {0x70f, 0x70f}, {0x890, 0x891}, {0x8e2, 0x8e2},
	{0x180e, 0x180e}, {0x200b, 0x200f}, {0x202a, 0x202e}, {0x2060, 0x2064}, {0x2066, 0x206f}, {0xfeff, 0xfeff}, {0xfff9, 0xfffb},
	{0x110bd, 0x110bd}, {0x110cd, 0x110cd}, {0x13430, 0x1343f}, {0x1bca0, 0x1bca3}, {0x1d173, 0x1d17a}, {0xe0001, 0xe0001},
	{0xe0020, 0xe007f}}

var jWSRanges = [][2]int{{0x9, 0xd}, {0x20, 0x20}, {0x85, 0x85}, {0xa0, 0xa0}, {0x1680, 0x1680}, {0x2000, 0x200a}, {0x2028, 0x2029},
	{0x202f, 0x202f}, {0x205f, 0x205f}, {0x3000, 0x3000}}

func jIn(n int, rs [][2]int) bool {
	for _, r := range rs {
		if n >= r[0] && n <= r[1] {
			return true
		}
	}
	return false
}

func jNumPolicy(n int) bool {
	switch {
	case n < 0x21 || n > 0x10ffff:
		return false
	case n >= 0xd800 && n <= 0xdfff:
		return false
	case n >= 0x7f && n <= 0x9f:
		return false
	case n >= 0xfdd0 && n <= 0xfdef, n&0xfffe == 0xfffe:
		return false
	}
	return !jIn(n, jCfRanges) && !jIn(n, jWSRanges)
}

func jXMLOK(s string) bool {
	for _, r := range s {
		if !(r == 0x9 || r == 0xa || r == 0xd || r >= 0x20 && r <= 0xd7ff || r >= 0xe000 && r <= 0xfffd || r >= 0x10000 && r <= 0x10ffff) {
			return false
		}
	}
	return true
}

func jEntity(name string) (rune, bool) {
	digits := true
	for i := 0; i < len(name); i++ {
		if name[i] < '0' || name[i] > '9' {
			digits = false
		}
	}
	if !digits {
		r, ok := jNamed[name]
		return r, ok
	}
	if len(name) > 7 {
		return 0, false
	}
	n, _ := strconv.Atoi(name)
	if !jNumPolicy(n) {
		return 0, false
	}
	return rune(n), true
}

func jRestore(s string) (string, bool) {
	var b strings.Builder
	for _, t := range jTokens(s) {
		if t.ent == "" {
			b.WriteRune(t.r)
			continue
		}
		r, ok := jEntity(t.ent)
		if !ok {
			return "", false
		}
		b.WriteRune(r)
	}
	return b.String(), true
}

// jAccValue 는 접근성 값(원본처럼 다듬은 것)을 본다 — 탭·줄바꿈 밖의 제어 문자와 맨 &·<·> 는 받지 않고, 엔티티만 되돌린다.
func jAccValue(v string) (string, bool) {
	for _, t := range jTokens(v) {
		if t.ent != "" {
			continue
		}
		if t.r < 0x20 && t.r != '\t' && t.r != '\n' || t.r >= 0x7f && t.r <= 0x9f || t.r == '&' || t.r == '<' || t.r == '>' {
			return "", false
		}
	}
	d, ok := jRestore(v)
	if !ok || !jXMLOK(d) || utf8.RuneCountInString(d) > 200 {
		return "", false
	}
	return d, true
}

func jAlnumR(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }

func jAlnum(t jTok) bool { return t.ent != "" || jAlnumR(t.r) }

// jUnderscoreOK 는 _ 가 모두 글자·숫자(엔티티 포함) 사이인지다.
func jUnderscoreOK(ts []jTok) bool {
	for k, t := range ts {
		if t.ent == "" && t.r == '_' && (k == 0 || k == len(ts)-1 || !jAlnum(ts[k-1]) || !jAlnum(ts[k+1])) {
			return false
		}
	}
	return true
}

// jMDHead 는 토큰 머리가 마크다운 목록·구분선 꼴(「수.」·「수)」 + 빈칸·끝)인지다(=·-·+ 는 부르는 쪽이 본다).
func jMDHead(ts []jTok) bool {
	d := 0
	for d < len(ts) && ts[d].ent == "" && ts[d].r >= '0' && ts[d].r <= '9' {
		d++
	}
	return d >= 1 && d <= 9 && d < len(ts) && ts[d].ent == "" && (ts[d].r == '.' || ts[d].r == ')') &&
		(d+1 == len(ts) || ts[d+1].ent == "" && ts[d+1].r == ' ')
}

// jText 는 관계 라벨(card 거짓)·양끝 수량 글(card 참)을 보고 값을 구한다. 라벨은 markdown 으로, 수량 글은 글 그대로 그려진다.
func jText(raw string, card bool) (string, bool) {
	s := jTrim(raw)
	if s == "" || strings.Contains(s, "\t") || strings.Contains(s, "  ") {
		return "", false
	}
	ts := jTokens(s)
	bad := "#;\"`%\\<>&*~$[]{}:|"
	if card {
		bad = "#;\"`%\\<>&~$[]{}:|"
	}
	for _, t := range ts {
		if t.ent == "" && (t.r < 0x20 || t.r == 0x7f || strings.ContainsRune(bad, t.r)) {
			return "", false
		}
	}
	if !card {
		if !jUnderscoreOK(ts) || ts[0].ent == "" && strings.ContainsRune("-+=", ts[0].r) || jMDHead(ts) {
			return "", false
		}
	}
	v, ok := jRestore(s)
	if !ok || !jXMLOK(v) || utf8.RuneCountInString(v) > 200 {
		return "", false
	}
	if card && (utf8.RuneCountInString(v) > 20 || v == "none") {
		return "", false
	}
	return v, true
}

// jKeywords 는 원본 lexer 가 이름 토큰보다 먼저 읽는 규칙의 낱말이다(classDiagram.jison 규칙에서 따로 옮겼다 — 대소문자 구분).
var jKeywords = []string{"o", "_top", "_parent", "_blank", "_self", "href", "note", "click", "link", "callback", "cssClass", "class",
	"namespace", "classDef", "style", "call", "classDiagram", "accDescr", "accTitle"}

// jNameChar 는 이름 꼴(구조를 읽을 때의 겉 테두리) 글자다 — ASCII 낱말 글자와 Unicode 글자.
func jNameChar(r rune) bool {
	if r < 0x80 {
		return jWordByte(byte(r))
	}
	return unicode.IsLetter(r)
}

// jScanName 은 t[i:] 의 이름 꼴 끝 자리다(없으면 i).
func jScanName(t string, i int) int {
	for i < len(t) {
		r, n := utf8.DecodeRuneInString(t[i:])
		if !jNameChar(r) {
			break
		}
		i += n
	}
	return i
}

// jNameOK 는 t[s:e] 가 원본에서 이름 토큰으로만 읽혀 글자 그대로 클래스 이름이 되는지다(뒤 글자 t[e] 가 키워드의 낱말 경계).
func jNameOK(t string, s, e int) bool {
	name := t[s:e]
	if name == "" {
		return false
	}
	rs := []rune(name)
	if len(rs) > 200 {
		return false
	}
	for k, r := range rs {
		switch {
		case r < 0x80:
			if !jWordByte(byte(r)) {
				return false
			}
		case r > 0xffff || !unicode.IsLetter(r) || !jUni(r):
			return false
		}
		if r == '_' && (k == 0 || k == len(rs)-1 || !jAlnumR(rs[k-1]) || !jAlnumR(rs[k+1])) {
			return false
		}
	}
	if low := jLower(name); strings.HasPrefix(low, "acctitle") || strings.HasPrefix(low, "accdescr") {
		return false
	}
	for i := s; i < e; i++ {
		if t[i] >= 0x80 || i > s && t[i-1] < 0x80 {
			continue
		}
		for _, kw := range jKeywords {
			end := i + len(kw)
			if end <= e && t[i:end] == kw && (end == len(t) || !jWordByte(t[end])) {
				return false
			}
		}
	}
	return true
}

// jWordOK 는 ASCII 낱말 하나(원본 ALPHA)가 이름 토큰으로 읽히는지다.
func jWordOK(w string) bool {
	if w == "" {
		return false
	}
	for i := 0; i < len(w); i++ {
		if !jWordByte(w[i]) {
			return false
		}
	}
	return jNameOK(w, 0, len(w))
}

// jMember 는 멤버 하나(다듬은 글)를 원본 ClassMember 대로 읽어 이 판이 받는 꼴인지와 그 값을 구한다(body 는 클래스 몸 안).
func jMember(s string, body bool) (jMem, bool) {
	if s == "" || strings.Contains(s, "\t") || strings.Contains(s, "  ") {
		return jMem{}, false
	}
	ts := jTokens(s)
	n := len(ts)
	opens, closes, lp, rp, clsN := 0, 0, -1, -1, 0
	for k, t := range ts {
		if t.ent != "" {
			continue
		}
		switch r := t.r; {
		case jAlnumR(r), strings.ContainsRune(" ,.-=?/_", r):
		case r == ':':
			if !body {
				return jMem{}, false
			}
		case r == '(':
			opens++
			lp = k
		case r == ')':
			closes++
			rp = k
		case r == '~':
		case r == '+' || r == '#':
			if k != 0 {
				return jMem{}, false
			}
		case r == '$' || r == '*':
			clsN++
		default:
			return jMem{}, false
		}
	}
	if !jUnderscoreOK(ts) || clsN > 1 {
		return jMem{}, false
	}
	m := jMem{}
	start := 0
	if ts[0].ent == "" && strings.ContainsRune("+-#~", ts[0].r) {
		m.vis, start = ts[0].r, 1
	}
	if m.vis == 0 && (ts[0].ent == "" && ts[0].r == '=' || jMDHead(ts)) {
		return jMem{}, false
	}
	isCls := func(t jTok) bool { return t.ent == "" && (t.r == '$' || t.r == '*') }
	clsOf := func(t jTok) int {
		if t.r == '$' {
			return 1
		}
		return 2
	}
	keep := func(id string) string { // Preserve one space only
		if strings.HasPrefix(id, " ") {
			return " " + strings.Trim(id, " ")
		}
		return strings.Trim(id, " ")
	}
	var disp string
	if closes > 0 {
		if opens != 1 || closes != 1 || lp > rp {
			return jMem{}, false
		}
		for _, t := range ts[:rp] {
			if isCls(t) {
				return jMem{}, false
			}
		}
		name := jTokStr(ts[start:lp])
		if strings.Trim(name, " ") == "" {
			return jMem{}, false
		}
		params := strings.Trim(jTokStr(ts[lp+1:rp]), " ")
		rest := ts[rp+1:]
		ret := ""
		switch {
		case len(rest) == 0:
		case len(rest) == 1 && isCls(rest[0]):
			m.cls = clsOf(rest[0])
		case rest[0].ent == "" && rest[0].r == ' ' && len(rest) > 1:
			rt := rest[1:]
			if isCls(rt[len(rt)-1]) {
				m.cls = clsOf(rt[len(rt)-1])
				rt = rt[:len(rt)-1]
			}
			if len(rt) == 0 || rt[0].ent == "" && (rt[0].r == ' ' || rt[0].r == ':') || rt[len(rt)-1].ent == "" && rt[len(rt)-1].r == ' ' {
				return jMem{}, false
			}
			for _, t := range rt {
				if isCls(t) {
					return jMem{}, false
				}
			}
			ret = jTokStr(rt)
		default:
			return jMem{}, false
		}
		gn, d1, ok1 := jGenStr(keep(name))
		gp, d2, ok2 := jGenStr(params)
		gr, d3, ok3 := jGenStr(ret)
		if !ok1 || !ok2 || !ok3 {
			return jMem{}, false
		}
		m.gen = max(d1, d2, d3)
		disp = jTokStr(ts[:start]) + gn + "(" + gp + ")"
		if ret != "" {
			disp += " : " + gr
		}
	} else {
		if opens > 0 {
			return jMem{}, false
		}
		end := n
		if n > start && isCls(ts[n-1]) {
			m.cls, end = clsOf(ts[n-1]), n-1
		}
		if clsN == 1 && m.cls == 0 {
			return jMem{}, false
		}
		id := ""
		if end > start {
			id = jTokStr(ts[start:end])
		}
		if strings.Trim(id, " ") == "" {
			return jMem{}, false
		}
		gi, d, ok := jGenStr(keep(id))
		if !ok {
			return jMem{}, false
		}
		m.gen = d
		disp = jTokStr(ts[:start]) + gi
	}
	v, ok := jRestore(disp)
	if !ok || !jXMLOK(v) || utf8.RuneCountInString(v) > 200 {
		return jMem{}, false
	}
	m.text = v
	return m, true
}

// addClass 는 원본 addClass 다 — 이름(~ 앞)이 이미 있으면 아무것도 하지 않는다(인자는 처음 것).
func (m *jModel) addClass(name, typ string) int {
	if k, ok := m.idx[name]; ok {
		if m.classes[k].typ != typ {
			m.typAgain++
		}
		return k
	}
	m.idx[name] = len(m.classes)
	m.classes = append(m.classes, &jClass{name: name, typ: typ})
	return len(m.classes) - 1
}

func (m *jModel) annotate(k int, a string, line int) bool {
	c := m.classes[k]
	if c.hasAnnot {
		return m.reject(line)
	}
	c.annot, c.hasAnnot = a, true
	return true
}

func (m *jModel) addMember(k int, s string, body bool, line int) bool {
	c := m.classes[k]
	if len(c.attrs)+len(c.meths) >= 50 {
		return m.reject(line)
	}
	mem, ok := jMember(s, body)
	if !ok {
		return m.reject(line)
	}
	if strings.Contains(s, ")") {
		c.meths = append(c.meths, mem)
	} else {
		c.attrs = append(c.attrs, mem)
	}
	return true
}

// bodyLine 은 클래스 몸의 한 줄(앞뒤 빈칸을 지운 것)이다 — <<주석>> 이거나 멤버 하나.
func (m *jModel) bodyLine(t string, k, line int) bool {
	if t == "" {
		return true
	}
	if strings.ContainsAny(t, "{}") || jHasSemi(t) {
		return m.reject(line)
	}
	if len(t) >= 4 && strings.HasPrefix(t, "<<") && strings.HasSuffix(t, ">>") {
		a := t[2 : len(t)-2]
		if !jWordOK(a) {
			return m.reject(line)
		}
		return m.annotate(k, a, line)
	}
	return m.addMember(k, t, true, line)
}

// jHasSemi 는 엔티티 밖의 ; 가 있는지다.
func jHasSemi(t string) bool {
	for _, tk := range jTokens(t) {
		if tk.ent == "" && tk.r == ';' {
			return true
		}
	}
	return false
}

// jStyleText 는 쓰지 않는 classDef·style 의 값(속성:값 을 쉼표로 이은 것 — 속성은 영문, 값은 영문·숫자·#·%·.)인지다.
func jStyleText(s string) bool {
	for _, part := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(part, ":")
		if !ok || k == "" || v == "" {
			return false
		}
		for i := 0; i < len(k); i++ {
			if !('a' <= k[i] && k[i] <= 'z' || 'A' <= k[i] && k[i] <= 'Z') {
				return false
			}
		}
		for i := 0; i < len(v); i++ {
			c := v[i]
			if !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '#' || c == '%' || c == '.') {
				return false
			}
		}
	}
	return true
}

// jStyleLine 은 `kw 이름[,이름…] 값` 꼴이면 이름 목록과 첫 속성을 돌려준다(single 이면 이름 하나).
func jStyleLine(t, kw string, single bool) ([]string, string, bool) {
	rest, ok := strings.CutPrefix(t, kw)
	if !ok || rest == "" || !jIsWS(rest[0]) {
		return nil, "", false
	}
	rest = rest[jSkip(rest, 0):]
	sp := strings.IndexAny(rest, " \t")
	if sp < 0 {
		return nil, "", false
	}
	names := strings.Split(rest[:sp], ",")
	for _, w := range names {
		if w == "" || strings.Trim(w, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_") != "" {
			return nil, "", false
		}
	}
	if single && len(names) != 1 {
		return nil, "", false
	}
	style := rest[jSkip(rest, sp):]
	if !jStyleText(style) {
		return nil, "", false
	}
	key, _, _ := strings.Cut(style, ":")
	return names, key, true
}

// jQuoted 는 t[i:] 가 "…" 로 시작하면 안 글과 그 뒤 자리를 돌려준다. str 은 원본 lexer 가 그 구간에서 STR 토큰을 내는지다 —
// jison:60–62 의 차례 그대로 옮긴다: <*>["] 로 string 상태에 들고, string 상태에서는 ["](상태만 닫고 토큰 없음)가 [^"]*(STR)보다
// 앞이라 여는 따옴표 바로 뒤가 따옴표(빈 "")면 STR 없이 닫힌다. 그 밖이면 [^"]* 가 닫는 따옴표 앞까지를 STR 하나로 낸다.
func jQuoted(t string, i int) (v string, end int, ok bool) {
	v, end, ok, _ = jSTR(t, i)
	return v, end, ok
}

func jSTR(t string, i int) (v string, end int, quoted, str bool) {
	if i >= len(t) || t[i] != '"' {
		return "", i, false, false
	}
	k := i + 1 // string 상태
	if k < len(t) && t[k] == '"' {
		return "", k + 1, true, false // ["] 가 먼저 — STR 없이 닫힘
	}
	e := k
	for e < len(t) && t[e] != '"' {
		e++
	}
	if e >= len(t) {
		return "", i, false, false // 닫는 따옴표 없음(줄은 다른 꼴로 간다)
	}
	return t[k:e], e + 1, true, true
}

// jKwWS 는 t 가 kw + 빈칸(하나 이상)으로 시작하면 빈칸 뒤 자리다.
func jKwWS(t string, i int, kw string) (int, bool) {
	if !strings.HasPrefix(t[i:], kw) {
		return i, false
	}
	j := i + len(kw)
	if j >= len(t) || !jIsWS(t[j]) {
		return i, false
	}
	return jSkip(t, j), true
}

// jIgnored 는 쓰지 않는 줄의 꼴(classDef·style·cssClass·click·link·callback)인지 본다 — matched 가 거짓이면 다음 꼴로 간다.
func (m *jModel) jIgnored(t string, line int) (matched, ok bool) {
	for _, kw := range []string{"classDef", "style"} {
		names, key, hit := jStyleLine(t, kw, kw == "style")
		if !hit {
			continue
		}
		if len(names) > 20 || !jWordOK(key) {
			return true, m.reject(line)
		}
		for _, w := range names {
			if !jWordOK(w) {
				return true, m.reject(line)
			}
		}
		m.sawIgnorable = true
		return true, true
	}
	if i, hit := jKwWS(t, 0, "cssClass"); hit {
		if _, _, q, str := jSTR(t, i); q && !str {
			return true, m.reject(line) // CSSCLASS STR ALPHA 의 STR 이 없다 — 원본 문법 오류
		}
		if list, j, q := jQuoted(t, i); q {
			css := t[jSkip(t, j):]
			good := css != "" && strings.Trim(css, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_") == ""
			items := strings.Split(list, ",")
			for _, it := range items {
				if it == "" || jScanName(it, 0) != len(it) {
					good = false
				}
			}
			if good {
				if len(items) > 20 || !jWordOK(css) {
					return true, m.reject(line)
				}
				m.sawIgnorable = true
				return true, true
			}
		}
	}
	if i, hit := jKwWS(t, 0, "click"); hit {
		e := jScanName(t, i)
		if e > i && e < len(t) && jIsWS(t[e]) {
			k := jSkip(t, e)
			if j, h := jKwWS(t, k, "href"); h {
				if _, end, q, str := jSTR(t, j); q && end == len(t) {
					if !str || !jNameOK(t, i, e) { // CLICK className HREF STR — 빈 "" 는 STR 이 없어 원본 문법 오류
						return true, m.reject(line)
					}
					m.sawIgnorable = true
					return true, true
				}
			}
			if j, h := jKwWS(t, k, "call"); h {
				f := j
				for f < len(t) && (jWordByte(t[f]) && (f > j || t[f] < '0' || t[f] > '9')) {
					f++
				}
				if f > j && t[f:] == "()" {
					if !jNameOK(t, i, e) {
						return true, m.reject(line)
					}
					m.sawIgnorable = true
					return true, true
				}
			}
		}
	}
	for _, kw := range []string{"link", "callback"} {
		if i, hit := jKwWS(t, 0, kw); hit {
			e := jScanName(t, i)
			if e > i && e < len(t) && jIsWS(t[e]) {
				if _, end, q, str := jSTR(t, jSkip(t, e)); q && end == len(t) {
					if !str || !jNameOK(t, i, e) { // LINK·CALLBACK className STR — 빈 "" 는 STR 이 없어 원본 문법 오류
						return true, m.reject(line)
					}
					m.sawIgnorable = true
					return true, true
				}
			}
		}
	}
	return false, false
}

// jClassStmt 는 class 로 시작하는 줄이다.
func (m *jModel) jClassStmt(t string, line int) bool {
	i := jSkip(t, 5)
	ne := jScanName(t, i)
	if ne == i {
		return m.reject(line)
	}
	e, typ, hasGen := jGenName(t, ne)
	k := jSkip(t, e)
	open, empty, annot, css, inner, oneLine := false, false, "", "", "", false
	switch {
	case e == len(t):
	case t[k] == '{':
		k2 := jSkip(t, k+1)
		switch {
		case k2 == len(t):
			open = true
		case t[k2] == '}' && k2+1 == len(t):
			empty = true
		case strings.HasSuffix(t, "}") && len(t) > k+1:
			inner, oneLine = t[k+1:len(t)-1], true
		default:
			return m.reject(line)
		}
	case strings.HasPrefix(t[k:], "<<") && strings.HasSuffix(t, ">>") && len(t)-k >= 4:
		annot = t[k+2 : len(t)-2]
		if annot == "" || strings.Trim(annot, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_") != "" {
			return m.reject(line)
		}
	case strings.HasPrefix(t[k:], ":::"):
		css = t[jSkip(t, k+3):]
		if css == "" || strings.Trim(css, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_") != "" {
			return m.reject(line)
		}
	default:
		return m.reject(line)
	}
	_ = empty
	if !jNameOK(t, i, ne) || hasGen && !jGenArgOK(t[i:ne], typ) || annot != "" && !jWordOK(annot) || css != "" && !jWordOK(css) {
		return m.reject(line)
	}
	if len(m.classes) >= 200 {
		if _, ok := m.idx[t[i:ne]]; !ok {
			return m.reject(line)
		}
	}
	c := m.addClass(t[i:ne], typ)
	switch {
	case annot != "":
		return m.annotate(c, annot, line)
	case css != "":
		m.sawIgnorable = true
	case open:
		m.body, m.openLine = c, line
	case oneLine:
		if strings.ContainsAny(inner, "{}") {
			return m.reject(line)
		}
		return m.bodyLine(jTrim(inner), c, line)
	}
	return true
}

// jShape 는 t[i:] 머리의 관계 끝 모양(원본 규칙 차례 — <|·|>·*·o·<·>)이다.
func jShape(t string, i int) (string, int) {
	for _, s := range []string{"<|", "|>", "*", "o", "<", ">"} {
		if strings.HasPrefix(t[i:], s) {
			return s, i + len(s)
		}
	}
	return "", i
}

var jShapeName = map[string]string{"": "none", "<|": "extension", "|>": "extension", "*": "composition", "o": "aggregation", "<": "dependency", ">": "dependency"}

// jRelation 은 관계 줄이다 — matched 가 거짓이면 관계 꼴이 아니다(다음 꼴로 간다).
func (m *jModel) jRelation(t string, line int) (matched, ok bool) {
	n1 := jScanName(t, 0)
	if n1 == 0 {
		return false, false
	}
	e1, typ1, gen1 := jGenName(t, n1)
	i := jSkip(t, e1)
	c1, j, hasC1, str1 := jSTR(t, i)
	if hasC1 {
		i = jSkip(t, j)
	}
	s1, i := jShape(t, i)
	var ln string
	switch {
	case strings.HasPrefix(t[i:], "--"):
		ln = "--"
	case strings.HasPrefix(t[i:], ".."):
		ln = ".."
	default:
		return false, false
	}
	i += 2
	// 여기부터는 관계이거나 거부다(이름 : 멤버 꼴은 이름 뒤가 : 여야 한다)
	var s2 string
	if i < len(t) && t[i] == 'o' {
		if i+1 < len(t) && jWordByte(t[i+1]) {
			return true, m.reject(line) // --oB: 원본은 o 를 이름 머리로 읽는다
		}
		s2, i = "o", i+1
	} else {
		s2, i = jShape(t, i)
	}
	i = jSkip(t, i)
	c2, j, hasC2, str2 := jSTR(t, i)
	if hasC2 {
		i = jSkip(t, j)
	}
	s := i
	n2 := jScanName(t, s)
	if n2 == s {
		return true, m.reject(line)
	}
	e2, typ2, gen2 := jGenName(t, n2)
	label, hasLabel := "", false
	if e2 < len(t) {
		k := jSkip(t, e2)
		if k >= len(t) || t[k] != ':' {
			return true, m.reject(line)
		}
		label, hasLabel = t[k+1:], true
	}
	if !jNameOK(t, 0, n1) || !jNameOK(t, s, n2) || gen1 && !jGenArgOK(t[:n1], typ1) || gen2 && !jGenArgOK(t[s:n2], typ2) {
		return true, m.reject(line)
	}
	// 빈 수량 글 "" — 원본 lexer 는 STR 을 내지 않아 원본은 수량 없는 관계로 읽는다(relationStatement 의 STR 없는 꼴). 이 판은
	// 보수적으로 받지 않는다(빈 글을 수량 글 자리에 둔 뜻이 없다 — 구현 label 의 빈 값 거부). 같은 판단을 여기서 따로 내린다.
	if hasC1 && !str1 || hasC2 && !str2 {
		return true, m.reject(line)
	}
	r := jRel{a: t[:n1], b: t[s:n2], t1: jShapeName[s1], t2: jShapeName[s2], line: ln}
	if hasC1 {
		v, ok := jText(c1, true)
		if !ok {
			return true, m.reject(line)
		}
		r.c1 = v
	}
	if hasC2 {
		v, ok := jText(c2, true)
		if !ok {
			return true, m.reject(line)
		}
		r.c2 = v
	}
	if hasLabel {
		if strings.Contains(label, ":") {
			return true, m.reject(line)
		}
		v, ok := jText(label, false)
		if !ok {
			return true, m.reject(line)
		}
		r.label = v
	}
	if len(m.rels) >= 500 {
		return true, m.reject(line)
	}
	m.addClass(r.a, typ1)
	m.addClass(r.b, typ2)
	if r.a == r.b && (r.c1 != "" || r.c2 != "") {
		return true, m.reject(line)
	}
	m.rels = append(m.rels, r)
	return true, true
}

func (m *jModel) jStmt(t string, line int) bool {
	if strings.HasPrefix(t, "%%{") {
		m.sawIgnorable = true
		return true
	}
	if !m.header {
		if t != "classDiagram" && t != "classDiagram-v2" {
			return m.reject(line)
		}
		m.header = true
		return true
	}
	dirWord := strings.Contains(jLower(t), "direction")
	if m.body >= 0 {
		if t == "}" {
			m.body = -1
			return true
		}
		if dirWord {
			return m.reject(line)
		}
		return m.bodyLine(t, m.body, line)
	}
	if dirWord {
		rest, ok := strings.CutPrefix(t, "direction")
		if !ok || rest == "" || !jIsWS(rest[0]) {
			return m.reject(line)
		}
		d, ok := map[string]int{"TB": 0, "BT": 1, "LR": 2, "RL": 3}[rest[jSkip(rest, 0):]]
		if !ok {
			return m.reject(line)
		}
		m.dir = d
		return true
	}
	if strings.Contains(t, "%%") {
		return m.reject(line)
	}
	// 접근성 — 원문 대소문자 그대로
	for _, kw := range []string{"accTitle", "accDescr"} {
		rest, ok := strings.CutPrefix(t, kw)
		if !ok {
			continue
		}
		j := jSkip(rest, 0)
		if j < len(rest) && rest[j] == ':' {
			v := jTrim(rest[j+1:])
			if v == "" {
				return m.reject(line)
			}
			d, ok := jAccValue(v)
			if !ok {
				return m.reject(line)
			}
			if kw == "accTitle" {
				m.accT = d
			} else {
				m.accD = d
			}
			return true
		}
		if kw == "accDescr" && j < len(rest) && rest[j] == '{' {
			m.inAcc, m.accLine, m.accBuf = true, line, nil
			return m.accBody(m.raw[strings.IndexByte(m.raw, '{')+1:], line)
		}
	}
	if jHasSemi(t) {
		return m.reject(line)
	}
	if matched, ok := m.jIgnored(t, line); matched {
		return ok
	}
	if len(t) > 5 && strings.HasPrefix(t, "class") && jIsWS(t[5]) {
		return m.jClassStmt(t, line)
	}
	if strings.HasPrefix(t, "<<") {
		if end := strings.Index(t, ">>"); end > 2 {
			a := t[2:end]
			k := jSkip(t, end+2)
			ne := jScanName(t, k)
			e, typ, gen := jGenName(t, ne)
			if strings.Trim(a, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_") == "" && ne > k && e == len(t) {
				if !jWordOK(a) || !jNameOK(t, k, ne) || gen && !jGenArgOK(t[k:ne], typ) {
					return m.reject(line)
				}
				c, ok := m.idx[t[k:ne]] // 원본 addAnnotation 은 클래스를 만들지 않는다 — 인자는 쓰지 않는다
				if !ok {
					return m.reject(line)
				}
				return m.annotate(c, a, line)
			}
		}
	}
	if matched, ok := m.jRelation(t, line); matched {
		return ok
	}
	// 이름 : 멤버
	if ne := jScanName(t, 0); ne > 0 {
		e, typ, gen := jGenName(t, ne)
		k := jSkip(t, e)
		if k < len(t) && t[k] == ':' {
			v := t[k+1:]
			if strings.Contains(v, ":") || !jNameOK(t, 0, ne) || gen && !jGenArgOK(t[:ne], typ) {
				return m.reject(line)
			}
			if len(m.classes) >= 200 {
				if _, ok := m.idx[t[:ne]]; !ok {
					return m.reject(line)
				}
			}
			c := m.addClass(t[:ne], typ)
			return m.addMember(c, jTrim(v), false, line)
		}
	}
	return m.reject(line)
}

// accBody 는 accDescr { … } 의 줄 하나를 모은다(원본 블록 글 trim + setAccDescription 의 \n\s+ → \n).
func (m *jModel) accBody(t string, line int) bool {
	before, after, closed := strings.Cut(t, "}")
	if s := strings.TrimLeft(before, " \t"); s != "" {
		m.accBuf = append(m.accBuf, s)
	}
	if !closed {
		return true
	}
	if jTrim(after) != "" {
		return m.reject(line)
	}
	m.inAcc = false
	if len(m.accBuf) > 0 {
		last := len(m.accBuf) - 1
		m.accBuf[last] = jTrim(m.accBuf[last])
		d, ok := jAccValue(strings.Join(m.accBuf, "\n"))
		if !ok {
			return m.reject(line)
		}
		m.accD = d
	}
	return true
}

// jYAMLTitle 은 생성기가 내는 title 값 꼴만 손으로 푼다.
func jYAMLTitle(v string) string {
	if len(v) < 2 || v[0] != '"' || v[len(v)-1] != '"' {
		return v
	}
	in := v[1 : len(v)-1]
	var b strings.Builder
	for i := 0; i < len(in); i++ {
		if in[i] != '\\' || i+1 >= len(in) {
			b.WriteByte(in[i])
			continue
		}
		switch in[i+1] {
		case 't':
			b.WriteByte('\t')
			i++
		case 'u':
			n, _ := strconv.ParseUint(in[i+2:i+6], 16, 32)
			b.WriteRune(rune(n))
			i += 5
		default:
			b.WriteByte(in[i+1])
			i++
		}
	}
	return b.String()
}

// jJudge 는 원문 하나를 판정한다 — 받아들이면 모델과 -1, 아니면 nil 과 거부 줄(0 = 줄 없음).
func jJudge(src string) (*jModel, int) {
	m := &jModel{idx: map[string]int{}, body: -1, failLine: -1}
	lines := strings.Split(src, "\n")
	start := 0
	if len(lines) > 0 && jTrim(strings.TrimSuffix(lines[0], "\r")) == "---" {
		end := -1
		for i := 1; i < len(lines); i++ {
			if jTrim(strings.TrimSuffix(lines[i], "\r")) == "---" {
				end = i
				break
			}
		}
		if end < 0 {
			return nil, 1
		}
		for k, l := range lines[1:end] {
			if v, ok := strings.CutPrefix(l, "title: "); ok {
				m.caption, m.captionLine = jYAMLTitle(v), k+2
				if strings.Contains(m.caption, "  ") || strings.Contains(m.caption, "\t") {
					return nil, m.captionLine
				}
			} else if strings.HasPrefix(l, "config:") {
				m.sawIgnorable = true
			}
		}
		start = end + 1
	}
	for i := start; i < len(lines); i++ {
		ln := i + 1
		raw := strings.TrimSuffix(lines[i], "\r")
		if strings.ContainsAny(raw, "\r\f\v") {
			return nil, ln
		}
		for _, r := range raw {
			for _, sp := range jSpaceRunes {
				if r == sp {
					return nil, ln
				}
			}
		}
		t := jTrim(raw)
		if strings.Contains(t, "ﬂ°") || strings.Contains(t, "¶ß") {
			return nil, ln
		}
		if strings.Contains(t, "%%{") {
			whole := strings.HasPrefix(t, "%%{") && strings.HasSuffix(t, "}%%") && len(t) >= 6 && !strings.Contains(t[2:len(t)-2], "%%")
			if !whole || m.inAcc || m.body >= 0 {
				return nil, ln
			}
		}
		if strings.HasPrefix(t, "%%") && !strings.HasPrefix(t, "%%{") {
			if (m.inAcc || m.body >= 0) && strings.TrimLeft(raw, " \t") == "%%" {
				return nil, ln
			}
			continue
		}
		t = jCutSemi(jCutSemi(t, "style"), "classDef")
		m.raw = jCutSemi(jCutSemi(strings.TrimLeft(raw, " \t"), "style"), "classDef")
		if m.inAcc {
			if !m.accBody(m.raw, ln) {
				return nil, m.failLine
			}
			continue
		}
		if t == "" {
			continue
		}
		if !m.jStmt(t, ln) {
			return nil, m.failLine
		}
	}
	switch {
	case m.inAcc:
		return nil, m.accLine
	case m.body >= 0:
		return nil, m.openLine
	case !m.header, len(m.classes) == 0:
		return nil, 0
	case !jXMLOK(m.caption):
		return nil, m.captionLine
	}
	return m, -1
}

// jSummary 는 판정기 모델을 summary 와 같은 꼴로 적는다.
func jSummary(m *jModel) string {
	var b strings.Builder
	fmt.Fprintf(&b, "dir=%d", m.dir)
	for _, c := range m.classes {
		b.WriteString(" " + c.name)
		if c.typ != "" {
			b.WriteString("<" + c.typ + ">")
		}
		if c.hasAnnot {
			b.WriteString("«" + c.annot + "»")
		}
		b.WriteString("{")
		for i, ms := range [][]jMem{c.attrs, c.meths} {
			if i == 1 {
				b.WriteString("|")
			}
			for k, x := range ms {
				if k > 0 {
					b.WriteString(";")
				}
				b.WriteString(memSummary(x.vis, Classifier(x.cls), x.text))
			}
		}
		b.WriteString("}")
	}
	if len(m.rels) > 0 {
		b.WriteString(" |")
	}
	for _, r := range m.rels {
		fmt.Fprintf(&b, " %s>%s:%s:%s:%s", r.a, r.b, r.t1, r.t2, r.line)
		if r.label != "" {
			b.WriteString(`"` + r.label + `"`)
		}
		if r.c1 != "" || r.c2 != "" {
			b.WriteString("[" + r.c1 + "," + r.c2 + "]")
		}
	}
	if m.accT != "" || m.accD != "" {
		b.WriteString(" | acc=" + m.accT + "/" + m.accD)
	}
	return b.String()
}

// ---- 판정기: 제네릭(원본 규칙에서 따로 옮김)

// jGenName 은 이름 꼴 끝 ne 바로 뒤가 ~ 면 다음 ~ 까지를 제네릭 인자로 읽는다 — lexer 의 <*>"~" → generic 상태 [^~]* → [~].
// 닫는 ~ 가 줄 안에 없으면 제네릭이 아니다(원본은 다음 줄로 넘어가 먹는다 — 그 줄은 뒤 구조 검사가 받지 않는다).
func jGenName(t string, ne int) (end int, typ string, has bool) {
	if ne >= len(t) || t[ne] != '~' {
		return ne, "", false
	}
	j := strings.IndexByte(t[ne+1:], '~')
	if j < 0 {
		return ne, "", false
	}
	return ne + 1 + j + 1, t[ne+1 : ne+1+j], true
}

// jGenArgOK 는 이름 제네릭의 인자(원본 type)가 이 판이 받는 꼴인지다 — 낱말을 쉼표로 이은 것(쉼표 뒤 빈칸 하나까지). 낱말은
// 이름 글자(ASCII 낱말 글자, BMP 안의 UNICODE_TEXT 글자), _ 는 글자 사이. 머리가 generic 상태에서 먼저 맞는 <*> 키워드 규칙
// (jison:89–94·104 — \b 가 붙는다)이면 받지 않는다. 상자 제목 「이름<인자>」는 200 자까지.
func jGenArgOK(name, typ string) bool {
	if typ == "" {
		return false
	}
	for _, kw := range []string{"cssClass", "callback", "link", "click", "note for", "note", "href"} {
		if rest, ok := strings.CutPrefix(typ, kw); ok && (rest == "" || !jWordByte(rest[0])) {
			return false
		}
	}
	words := strings.Split(typ, ",")
	for k, w := range words {
		if k > 0 && strings.HasPrefix(w, " ") {
			w = w[1:]
		}
		rs := []rune(w)
		if len(rs) == 0 {
			return false
		}
		for x, r := range rs {
			if r < 0x80 && !jWordByte(byte(r)) || r >= 0x80 && (r > 0xffff || !unicode.IsLetter(r) || !jUni(r)) {
				return false
			}
			if r == '_' && (x == 0 || x == len(rs)-1 || !jAlnumR(rs[x-1]) || !jAlnumR(rs[x+1])) {
				return false
			}
		}
	}
	return utf8.RuneCountInString(name)+utf8.RuneCountInString(typ)+2 <= 200
}

// jPGT 는 원본 common.ts parseGenericTypes 를 판정기 쪽에서 다시 적은 것이다(입력은 룬 글 — 엔티티는 글자 하나로 바꿔 넘긴다).
func jPGT(in string) string {
	tilde := func(s string) int { return len(strings.Split(s, "~")) - 1 } // countOccurrence
	process := func(s string) string {                                    // processSet
		n := tilde(s)
		if n < 2 {
			return s
		}
		lead := n%2 == 1 && s[0] == '~'
		if lead {
			s = s[1:]
		}
		cs := []rune(s)
		for {
			a, b := -1, -1
			for k, c := range cs {
				if c == '~' {
					if a < 0 {
						a = k
					}
					b = k
				}
			}
			if a < 0 || a == b {
				break
			}
			cs[a], cs[b] = '<', '>'
		}
		if lead {
			return "~" + string(cs)
		}
		return string(cs)
	}
	// split(/(,)/) — 쉼표 자리마다 [앞, ",", 뒤]
	var sets []string
	cur := ""
	for _, c := range in {
		if c == ',' {
			sets = append(sets, cur, ",")
			cur = ""
			continue
		}
		cur += string(c)
	}
	sets = append(sets, cur)
	var out []string
	for i := 0; i < len(sets); i++ {
		x := sets[i]
		if x == "," && i > 0 && i+1 < len(sets) && tilde(sets[i-1]) == 1 && tilde(sets[i+1]) == 1 {
			x = sets[i-1] + "," + sets[i+1]
			i++
			out = out[:len(out)-1]
		}
		out = append(out, process(x))
	}
	return strings.Join(out, "")
}

// jGenSeq 는 제네릭을 괄호처럼 읽는 되부름 읽기다(이 판이 받는 꼴의 정의 — 구현의 genericText 와 따로 적은 꼴). 이름 글자 낱말
// 바로 뒤의 ~ 는 그 뒤가 이름 글자면 여는 짝이고, 아니면 이 겹을 닫는 짝이다. 닫는 짝 뒤는 끝·~·쉼표·빈칸이어야 하고, 닫는 짝
// 바로 뒤의 ~ 는 바깥 겹을 닫는다. 그 밖의 자리의 ~ 는 받지 않는다. 끝나는 자리(닫는 ~ 의 자리 또는 끝)와 가장 깊은 겹을 준다.
func jGenSeq(g, out []rune, i, depth int) (int, int, bool) {
	name := func(c rune) bool { return c == '\uE001' || c == '_' || unicode.IsLetter(c) || unicode.IsNumber(c) }
	follow := func(k int) bool { return k+1 == len(g) || strings.ContainsRune("~, ", g[k+1]) }
	deep := depth
	for i < len(g) {
		switch c := g[i]; {
		case name(c):
			j := i
			for j < len(g) && name(g[j]) {
				j++
			}
			i = j
			if i == len(g) || g[i] != '~' {
				continue
			}
			if i+1 < len(g) && name(g[i+1]) {
				out[i] = '<'
				k, d, ok := jGenSeq(g, out, i+1, depth+1)
				if !ok || k >= len(g) {
					return 0, 0, false
				}
				out[k] = '>'
				deep = max(deep, d)
				i = k + 1
				continue
			}
			if depth == 0 || !follow(i) {
				return 0, 0, false
			}
			return i, deep, true
		case c == '~':
			if i == 0 || g[i-1] != '~' || depth == 0 || !follow(i) {
				return 0, 0, false
			}
			return i, deep, true
		default:
			i++
		}
	}
	if depth > 0 {
		return 0, 0, false
	}
	return i, deep, true
}

// jGenStr 는 멤버의 한 조각(이름·인자·반환형)의 제네릭을 바꾼다 — 괄호처럼 읽은 값이 원본 parseGenericTypes 와 같을 때만 받는다.
// 엔티티 토큰은 이름 글자 하나(U+E001)로 대신해 읽고 되돌린다. depth 는 가장 깊은 겹이다.
func jGenStr(s string) (string, int, bool) {
	ts := jTokens(s)
	g := make([]rune, len(ts))
	for k, t := range ts {
		if t.ent != "" {
			g[k] = '\uE001'
		} else {
			g[k] = t.r
		}
	}
	if !strings.ContainsRune(string(g), '~') {
		return s, 0, true
	}
	out := append([]rune(nil), g...)
	_, d, ok := jGenSeq(g, out, 0, 0)
	if !ok || jPGT(string(g)) != string(out) {
		return "", 0, false
	}
	for k := range ts {
		if g[k] == '~' {
			ts[k] = jTok{r: out[k]}
		}
	}
	return jTokStr(ts), d, true
}

// ---- 생성기: 경계 토큰을 명시적으로 조합한다

func pick(rng *rand.Rand, xs []string) string { return xs[rng.Intn(len(xs))] }

// gTrapOn 은 이번 원문에 함정을 섞는지다 — 판의 반은 함정 없이 만들어 받는 판의 몫을 지킨다.
var gTrapOn bool

func jTrap(rng *rand.Rand) bool { return gTrapOn && rng.Intn(7) == 0 }

func pickN(rng *rand.Rand, xs []string, n int) string {
	if gTrapOn {
		return pick(rng, xs)
	}
	return pick(rng, xs[:n])
}

var (
	gSafeNames = []string{"고객", "주문", "주문항목", "결제수단", "A", "B", "C", "Pay1", "x_y", "Đơn", "가나", "Ωmega", "ſtate", "class1",
		"Clicker", "linked", "notes", "o1", "가o1", "oA", "styles", "Note", "Link", "callbacks", "가a", "a가b", "ǅx", "ʰa"}
	gTrapNames = []string{"o", "가o", "가_top", "click", "link", "note", "href", "_self", "_blank", "call", "callback", "cssClass",
		"namespace", "classDiagram", "style", "classDef", "class", "accTitleX", "acctitle", "AccDescr2", "_a", "a_", "a__b", "𝔸", "a𝔸",
		"가link", "가note", "가style", "가href", "a가o", "ᏸ", "ꙮ", "ᄀ", "x\u0378", "a-b", "a.b", "ⅷ", "a١", "가class", "가o가",
		strings.Repeat("가", 200), strings.Repeat("나", 201), "A" + strings.Repeat("b", 199), "A" + strings.Repeat("b", 200)}
	gSafeMembers = []string{"+이름 String", "-전화$", "#보호 int", "~패키지", "+주문하기(상품) 주문", "+합계() decimal", "+결제(금액) bool*",
		"생성()$", "x", "a b", "+ 공백", "-총액 decimal", "List of x", "#35;태그", "x#59;y", "a_b", "f(a, b) int", "값 = 1", "경로/이름",
		"질문?", "1단계", "+f() int$", "f() a b", "f( x ) y", "#lt;br#gt;", "#169; 판권", "-x*", "+ f(x)", "f (x)", "a, b. c-d"}
	gTrapMembers = []string{"foo() : int", "foo()$ int", "foo()int", "f(()", "f)(", "(x)", "+(x)", ")x", "x$y", "x$*", "*x", "x~T~",
		"List~int~", "<<interface>>", "a  b", "a\tb", "_x", "x_", "= 선", "1. 첫째", "2) 둘째", "a:b", "a{b", "a\"b", "a`b", "a%b", "a&b",
		"a<b", "a*b", "a[b]", "x#y", "x+y", "x~y", "-", "+", "$", "+$", "f() $", "f() x:y", "f(a)b(c)", "f(x) y z$", "#copy;", "#160;",
		"x\ufffdy", "#65533;", "f() :x", "f()  x", "f() x ", "f() x$*", "#a_b;", strings.Repeat("가", 200), strings.Repeat("나", 199) + "()",
		"a!b", "a@b", "a'b", "a^b", "a|b"}
	gSafeLabels = []string{"주문한다", "담는다", "확인 완료", "결제 #35;3", "x#59;y", "a_b", "1단계", "Pay #quot;ok#quot;", "비율 50",
		"x", "#lt;태그", "a#amp;b", "#169;", "#8364;", "#lt;br#gt;", "끝.", "x\ufffdy", "\ufdd0", "0..*", "많음", "#42;강조#42;"}
	gTrapLabels = []string{"#copy;", "#zzz;", "#AMP;", "#nbsp;사이", "#32;", "#160;", "#8203;", "#65534;", "x\ufffey", "#+65;", "#65",
		"#35;35;", "두  칸", "x\ty", "#", "%", ":", "a:b", "\"", "`", "*강조*", "_x_", "x_", "- 목록", "+ 하나", "= 선", "1. 첫째", "2) 둘째",
		"<br>", "a<b", "[x]", "{", "}", "~", "$", "&", "|", "", " ", "none", "#110;one", strings.Repeat("가", 21), strings.Repeat("나", 201)}
	gSafeCards = []string{"1", "*", "0..1", "1..*", "n", "많음", "0..n", " 1 ", "- 1", "_x", "#42;"}
	gTrapCards = []string{"none", "", " ", "a:b", strings.Repeat("가", 21), "#lt;", "a  b", "#110;one", "<", "x;y", "#", "~"}
	gAccVals   = []string{"#+65;", "#35;35;", "#65", "x  y", "x\ty", "#lt;br#gt;", "#amp; #169;", "제목 ; # 끝", "<br>", "x\ufffey",
		"a & b", "#copy;", "#160;", "\x01", strings.Repeat("가", 201)}
	// 제네릭 경계 — 이름 뒤 ~인자~ 와 멤버 안 ~
	gSafeGens = []string{"~T~", "~K, V~", "~K,V~", "~가~", "~T1, T2~", "~linker~", "~가나_다~", "~Ωx~", "~notes, hrefs~", "~int~"}
	gTrapGens = []string{"~~", "~T", "~ T~", "~T ~", "~link~", "~note, T~", "~note for~", "~href~", "~click~", "~callback~", "~cssClass~",
		"~T_~", "~_T~", "~List~int~~", "~K,,V~", "~K,  V~", "~K, ~", "~,K~", "~\"x\"~", "~<<a>>~", "~T-1~", "~𝔸~", "~T~B", "~T~~U~",
		"~a b~", "~T~ ~U~", "~link2~", "~x\u0378~", "~T;~", "~#35;~", "~" + strings.Repeat("가", 190) + "~", "~" + strings.Repeat("가", 197) + "~"}
	gSafeGenMembers = []string{"+목록 List~int~", "~목록 List~int~", "+찾기(키 List~K~) Map~K, V~", "행렬 Array~Array~int~~$",
		"+f() List~T~$", "~g~T~(x) T", "값 List~#35;~", "#35;~T~ x", "Map~K,V~ 사전", "+f(a List~T~, b Map~K, V~)", "-깊이 A~B~C~D~~~*",
		"+ 앞 List~T~", "f(List~T~) Map~K, V~*", "x~T~, y~U~"}
	gTrapGenMembers = []string{"a~T~ b~U~", "m Map~K, List~V~~", "m List~Map~K, V~~", "List~int~~", "List~~", "x~T~y", "~T~ x", "f(x~) T",
		"List~T~.x", "~~", "~", "a~", "~a~b~", "List~T~ ~", "f~(x)~", "a~b~c~d~", "List~T~$x", "+f() ~T~", "a ~T~", "x~#35;~y~"}
	gSeps   = []string{" ", " ", " ", "", "\t", "  "}
	gShapes = []string{"", "", "<|", "|>", "*", "o", "<", ">"}
)

func gName(rng *rand.Rand) string {
	n := pick(rng, gSafeNames)
	if jTrap(rng) {
		n = pick(rng, gTrapNames)
	}
	if rng.Intn(4) == 0 { // 제네릭 인자 — 같은 이름이 인자 없이·다른 인자로 다시 나오는 판이 생긴다
		if jTrap(rng) {
			return n + pick(rng, gTrapGens)
		}
		return n + pick(rng, gSafeGens)
	}
	return n
}

func gMember(rng *rand.Rand) string {
	switch {
	case jTrap(rng):
		if rng.Intn(2) == 0 {
			return pick(rng, gTrapGenMembers)
		}
		return pick(rng, gTrapMembers)
	case rng.Intn(4) == 0:
		return pick(rng, gSafeGenMembers)
	}
	return pick(rng, gSafeMembers)
}

func gLabel(rng *rand.Rand) string {
	if jTrap(rng) {
		return pick(rng, gTrapLabels)
	}
	return pick(rng, gSafeLabels)
}

func gCard(rng *rand.Rand) string {
	if jTrap(rng) {
		return pick(rng, gTrapCards)
	}
	return pick(rng, gSafeCards)
}

func gSep(rng *rand.Rand) string { return pick(rng, gSeps) }

func gShape(rng *rand.Rand) string {
	if jTrap(rng) {
		return pick(rng, []string{"()", "|", "<|>", "o<", "x", "<<", "**"})
	}
	return pick(rng, gShapes)
}

func gColon(rng *rand.Rand) string {
	if jTrap(rng) {
		return pick(rng, []string{"::", ":::", " : :", ";", ""})
	}
	return gSep(rng) + ":" + gSep(rng)
}

// gRel 은 관계 한 줄이다.
func gRel(rng *rand.Rand) string {
	s := gName(rng) + gSep(rng)
	if rng.Intn(4) == 0 {
		s += "\"" + gCard(rng) + "\"" + gSep(rng)
	}
	line := "--"
	if rng.Intn(3) == 0 {
		line = ".."
	}
	if jTrap(rng) {
		line = pick(rng, []string{"-", "---", ".-", "-.", "...", "—"})
	}
	s += gShape(rng) + line + gShape(rng)
	if jTrap(rng) {
		s += "" // 오른 모양 바로 뒤에 이름(--oB)
	} else {
		s += gSep(rng)
	}
	if rng.Intn(4) == 0 {
		s += "\"" + gCard(rng) + "\"" + gSep(rng)
	}
	s += gName(rng)
	if rng.Intn(3) == 0 {
		s += gColon(rng) + gLabel(rng)
	}
	return s
}

// gStmt 는 뿌리 문장 한 줄이다.
func gStmt(rng *rand.Rand) string {
	switch rng.Intn(32) {
	case 0, 1, 2, 3, 4, 5, 6, 7, 8, 9:
		return gRel(rng)
	case 10, 11, 12:
		return gName(rng) + gColon(rng) + gMember(rng)
	case 13, 14:
		return "class" + pick(rng, []string{" ", " ", "\t", "  "}) + gName(rng) + pickN(rng, []string{"", "", " <<interface>>", "<<service>>",
			":::hot", " ::: hot", " {}", " { }", " { " + gMember(rng) + " }", "{" + gMember(rng) + "}", " { <<abstract>> }",
			"~T~", "[\"라벨\"]", " <<x>> {", ":::x {", " B", " <<가>>", " <<o>>", " <<a b>>", " {x", " { } }", " <<>>", ":::가", "::: "}, 11)
	case 15:
		return pickN(rng, []string{"<<interface>> ", "<<abstract>>", "<<service>>\t"}, 3) + gName(rng)
	case 16:
		return pickN(rng, []string{"direction LR", "direction TB", "direction BT", "direction RL", "direction  LR", "direction\tTB",
			"direction lr", "Direction LR", "DIRECTION RL", "direction XY", "direction LR x", "a direction LR", "direction", "directionLR"}, 6)
	case 17:
		v := pickN(rng, gAccVals, 8)
		return pickN(rng, []string{"accTitle: 제목 " + v, "accTitle:" + v + " \t", "accTitle : 큰 제목", "accDescr: 설명 ; # 포함", "accDescr:\t" + v,
			"accDescr { " + v + " }", "accDescr {\n  " + v + "  \n\n\t둘째\t칸  \n  %% 메모\n 끝 }", "accDescr { 앞  \n뒤\t}", "accDescr {\n" + v + "\n}",
			"accDescr : {", "ACCTITLE: x", "acctitle: x", "accTitle:", "accDescr {", "accDescr { 앞", "accDescr{", "accTitle x"}, 10)
	case 18:
		return pickN(rng, []string{"classDef hot fill:#f9f,stroke:#333", "classDef a,b fill:#f00;", "style A fill:#f00", "style A fill:#f9f,stroke:#333;",
			"cssClass \"A,B\" hot", "cssClass \"고객\" hot", "click A href \"https://example.com\"", "click A call f()", "link A \"https://x.example\"",
			"callback A \"f\"", "classDef 강조 fill:#f00", "classDef hot fill:red;stroke:#333", "classDef o fill:#f00", "classDef hot o:1",
			"style A,B fill:#f00", "style A fill:#f9f;stroke:#333", "cssClass A hot", "cssClass \"A, B\" hot", "cssClass \"A\" 가",
			"click A call 함수()", "click A call f(x)", "click o href \"u\"", "click A \"u\"", "link A u", "callback 가o \"f\"",
			"link A \"\"", "link A \" \"", "link A \"x\"", "callback A \"\"", "callback A \" \"", "callback A \"x\"",
			"click A href \"\"", "click A href \" \"", "click A href \"x\"", "cssClass \"\" hot", "link A \"\"\"\"",
			"classDef " + strings.Repeat("a,", 20) + "a fill:#f00", "STYLE A fill:#f00", "classdef a fill:#f00", "style A fill: #f00",
			"link A \"a~b~\"", "click A href \"x~T~y\"", "callback A \"f~\"", "click A~T~ href \"u\"", "link A~T~ \"u\"", "cssClass \"A~T~\" hot",
			"style A~T~ fill:#f00", "classDef A~T~ fill:#f00"}, 10)
	case 19, 20:
		return pickN(rng, []string{"%% 메모", "%%", "%%  ", "  %% 들여쓴 메모", "%%{init: {}}%%", "%%{init: {", "A %%{init: {}}%%",
			"%%{a}%% %%{b}%%", "A --> B %% 끝 주석"}, 5)
	case 21:
		if !gTrapOn {
			return gRel(rng)
		}
		return pick(rng, []string{"namespace N {", "note \"메모\"", "note for A \"메모\"", "class List~T~", "A~T~ --> B", "class A[\"라벨\"]",
			"A --() B", "}", "A", "A B", "A; B", "A --> B --> C", "[*] --> A", "\"A\" --> B", "A : <<interface>>", "A : x : y", "`A` --> B",
			"A:::hot", "A <| -- B", "A -- > B", "<<a>> B C", "<<a>>", "A :", "A : ", "{", "A --> B : x; y", "A.B --> C"})
	case 22:
		return ""
	}
	return gRel(rng)
}

// jDoc 은 판 하나의 원문이다.
func jDoc(rng *rand.Rand) string {
	var b strings.Builder
	gTrapOn = rng.Intn(2) == 0
	if rng.Intn(6) == 0 {
		titles := []string{"주문 모델", "클래스", "t", `"x\ufffdy"`, `"따옴표 \"제목\""`, `"x\ufdd0y"`, `"x\ufffey"`, `"x\uffffy"`,
			`"비율 #35;"`, `"a#lt;b#gt;"`, `"x  y"`, `"x\ty"`}
		b.WriteString("---\ntitle: " + pickN(rng, titles, 6) + "\n")
		if rng.Intn(2) == 0 {
			b.WriteString("config:\n  theme: dark\n")
		}
		b.WriteString("---\n")
	}
	if rng.Intn(10) == 0 {
		b.WriteString(pick(rng, []string{"%% 머리 앞 주석", "%%{init: {}}%%", ""}) + "\n")
	}
	head := "classDiagram"
	switch rng.Intn(10) {
	case 0, 1:
		head = "classDiagram-v2"
	case 2:
		if gTrapOn {
			head = pick(rng, []string{"classDiagram-V2", "classdiagram", "  classDiagram  ", "classDiagram direction LR", "classDiagramX", "ClassDiagram",
				"classDiagram-v3", "classDiagram;"})
		}
	}
	b.WriteString(head + "\n")
	n := 1 + rng.Intn(9)
	for k := 0; k < n; k++ {
		if rng.Intn(5) == 0 {
			// 클래스 몸
			b.WriteString("class " + gName(rng) + gSep(rng) + "{\n")
			for j := rng.Intn(5); j > 0; j-- {
				switch rng.Intn(9) {
				case 0:
					b.WriteString("  " + pickN(rng, []string{"<<interface>>", "<<abstract>>", "<<가>>", "<<a b>>", "<<o>>", "<<>>", "<< x >>"}, 2) + "\n")
				case 1:
					if gTrapOn {
						b.WriteString("  " + pick(rng, []string{"%%", "%% 메모", "direction LR", "x }", "{", "a;b", "[*]", "%%{init: {}}%%",
							"accTitle: x", "A --> B", "a : b", "class B"}) + "\n")
						continue
					}
					fallthrough
				default:
					b.WriteString(pick(rng, []string{"  ", "\t", "", "    "}) + gMember(rng) + pick(rng, []string{"", " ", "\t"}) + "\n")
				}
			}
			if !gTrapOn || rng.Intn(10) != 0 {
				b.WriteString(pickN(rng, []string{"}", "}", "  }  ", "} x", "}}"}, 3) + "\n")
			}
			continue
		}
		line := gStmt(rng)
		if gTrapOn && rng.Intn(25) == 0 {
			pos := rng.Intn(len(line) + 1)
			for pos > 0 && pos < len(line) && !utf8.RuneStart(line[pos]) {
				pos--
			}
			line = line[:pos] + pick(rng, []string{"\u00a0", "\ufeff", "\r", "\f", "\u200b", "ﬂ°", "\ufffe", "\uffff"}) + line[pos:]
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

// TestJudgeBoundaryProperties 는 생성기의 원문마다 판정기와 Parse 의 수락 여부·거부 줄·값을 견준다.
func TestJudgeBoundaryProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(20261031))
	const rounds = 30000
	acc, rej, genRej := 0, 0, 0
	// seen 은 받은 판에 나온 경계의 수다 — 생성기가 그 꼴을 받는 판에서 실제로 다루는지 본다.
	seen := map[string]int{}
	strSeen := map[string]int{} // 링크·콜백·cssClass 줄의 따옴표 경계(받음·거부 모두) — 생성기가 내는지만 본다
	for k := 0; k < rounds; k++ {
		src := jDoc(rng)
		jm, jl := jJudge(src)
		c, issues, err := Parse(src)
		for _, ln := range strings.Split(src, "\n") {
			ln = strings.TrimSpace(ln)
			if strings.HasPrefix(ln, "link ") || strings.HasPrefix(ln, "callback ") || strings.HasPrefix(ln, "click ") || strings.HasPrefix(ln, "cssClass ") {
				switch {
				case strings.Contains(ln, `""`):
					strSeen["빈 따옴표"]++
				case strings.Contains(ln, `" "`):
					strSeen["빈칸 따옴표"]++
				case strings.Contains(ln, `"x"`):
					strSeen["글 따옴표"]++
				}
			}
		}
		if jm == nil {
			rej++
			var de *diagram.Error
			if err == nil {
				t.Fatalf("판 %d: 판정기는 %d 줄에서 거부, Parse 는 받음\n%s\n얻음 %s", k, jl, src, summary(c))
			}
			if !errors.As(err, &de) || de.Line != jl {
				t.Fatalf("판 %d: 거부 줄 판정기 %d, Parse %v\n%s", k, jl, err, src)
			}
			if lines := strings.Split(src, "\n"); jl >= 1 && jl <= len(lines) && strings.Contains(lines[jl-1], "~") {
				genRej++
			}
			continue
		}
		acc++
		if err != nil {
			t.Fatalf("판 %d: 판정기는 받음, Parse 는 거부 %v\n%s\n판정기 %s", k, err, src, jSummary(jm))
		}
		got, want := summary(c), jSummary(jm)
		if got != want {
			t.Fatalf("판 %d: 값이 다름\n%s\nParse  %s\n판정기 %s", k, src, got, want)
		}
		if c.FMCaption != jm.caption {
			t.Fatalf("판 %d: 캡션 %q, 판정기 %q\n%s", k, c.FMCaption, jm.caption, src)
		}
		if (len(issues) > 0) != jm.sawIgnorable {
			t.Fatalf("판 %d: 경고 %v, 판정기 무시 줄 %v\n%s", k, issues, jm.sawIgnorable, src)
		}
		if jm.typAgain > 0 {
			seen["제네릭 인자 다른 재등장"]++
		}
		for _, cl := range jm.classes {
			if cl.typ != "" {
				seen["제네릭 이름"]++
				if strings.Contains(cl.typ, ",") {
					seen["제네릭 이름 쉼표"]++
				}
			}
			if strings.ContainsFunc(cl.name, func(r rune) bool { return r >= 0x80 && !unicode.Is(unicode.Hangul, r) }) {
				seen["ASCII 밖 이름(한글 밖)"]++
			}
			if cl.hasAnnot {
				seen["주석"]++
			}
			for _, ms := range [][]jMem{cl.attrs, cl.meths} {
				for _, x := range ms {
					if x.cls == 1 {
						seen["정적"]++
					}
					if x.cls == 2 {
						seen["추상"]++
					}
					if strings.Contains(x.text, " : ") {
						seen["반환형"]++
					}
					if len(x.text) > 1 && x.vis != 0 && x.text[1] == ' ' {
						seen["가시성 뒤 빈칸"]++
					}
					if strings.ContainsAny(x.text, "#;<>©") {
						seen["멤버 엔티티"]++
					}
					if x.gen >= 1 {
						seen["멤버 제네릭"]++
					}
					if x.gen >= 2 {
						seen["멤버 제네릭 중첩"]++
					}
					if x.gen >= 1 && x.vis == '~' {
						seen["가시성 ~ + 제네릭"]++
					}
				}
			}
		}
		for _, r := range jm.rels {
			if r.t1 == "aggregation" || r.t2 == "aggregation" {
				seen["집합 o"]++
			}
			if r.t1 == "dependency" || r.t2 == "dependency" {
				seen["의존"]++
			}
			if r.c1 != "" || r.c2 != "" {
				seen["수량 글"]++
			}
			if r.a == r.b {
				seen["자기 관계"]++
			}
			if strings.ContainsAny(r.label, "#;<>©*") {
				seen["라벨 엔티티"]++
			}
		}
		if strings.Contains(src, "%% 메모\n") && strings.Contains(src, "{\n") {
			seen["주석 줄"]++
		}
		if jm.accT != "" || jm.accD != "" {
			seen["접근성"]++
		}
		if jm.dir != 0 {
			seen["방향"]++
		}
	}
	for _, name := range []string{"ASCII 밖 이름(한글 밖)", "주석", "정적", "추상", "반환형", "가시성 뒤 빈칸", "멤버 엔티티", "집합 o", "의존", "수량 글",
		"자기 관계", "라벨 엔티티", "접근성", "방향", "제네릭 이름", "제네릭 이름 쉼표", "제네릭 인자 다른 재등장", "멤버 제네릭", "멤버 제네릭 중첩",
		"가시성 ~ + 제네릭"} {
		if seen[name] < 5 {
			t.Errorf("받은 판에 %s 가 %d 번 — 생성기가 이 경계를 고루 내지 못한다", name, seen[name])
		}
	}
	for _, name := range []string{"빈 따옴표", "빈칸 따옴표", "글 따옴표"} {
		if strSeen[name] < 5 {
			t.Errorf("생성기가 링크·콜백 줄의 %s 경계를 %d 번만 냈다", name, strSeen[name])
		}
	}
	t.Logf("따옴표 경계 %v", strSeen)
	t.Logf("경계 %v", seen)
	t.Logf("판 %d — 받음 %d, 거부 %d(~ 가 든 줄에서 거부 %d)", rounds, acc, rej, genRej)
	if genRej < 100 {
		t.Errorf("~ 가 든 줄에서 거부한 판이 %d 번 — 생성기가 제네릭 거부 경계를 고루 내지 못한다", genRej)
	}
	if acc < rounds/5 || rej < rounds/5 {
		t.Errorf("받음 %d·거부 %d — 둘 다 판수의 1/5 이상이어야 경계를 고루 본다", acc, rej)
	}
}
