package er

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/itda-work/itda-doc/diagram"
)

// 독립 판정기(itda-hyve#409 ④ 처음부터 — sequence ③ TestJudgeBoundaryProperties 꼴). 허용 꼴과 원본 lexer 의 함정을 이 파일 안에
// 따로 옮겨 적은 손 스캐너로, 무작위 원문마다 받아들일지와 받아들이면 그 값(엔티티 차례·별칭, 속성 차례·타입·이름·키·설명,
// 관계 양끝·기수 둘·식별 여부·역할, 방향, 접근성 값)을 스스로 구해 Parse 와 견준다. 구현의 정규식·상수·도우미(idOK·
// labelOK·keywordPrefixes·reRel …)는 쓰지 않는다 — 공통 패키지의 전처리(EncodeEntities)도 쓰지 않고 따로 적는다.
//
// 판정기가 옮긴 원본 규칙(mermaid@12.2.0):
//   - 전처리: CRLF → LF(줄 중간 CR 은 줄바꿈 — 받지 않음), 줄 머리 %% 주석 줄 지움(글 없는 맨 %% 는 남아 오류),
//     %%{…}%% 지시 지움, style/classDef …:…#…; 의 끝 ; 자르기, 엔티티 #\w+; 지키기
//   - detector: /^\s*erDiagram/ — 머리는 정확히 erDiagram
//   - lexer: case-insensitive(ASCII 만), 규칙 차례의 첫 일치 — UNICODE_TEXT 앞의 키워드(erDiagram·style·classDef·class·
//     subgraph·end·one…·many…·to·accTitle·accDescr·u(?=[.-|]))로 시작하는 이름은 갈린다. .*direction\s+(TB|BT|RL|LR)
//     은 줄 어디서든 줄 전체를 먹는다. 블록 상태에서는 \b(PK|FK|UK)\b 가 ATTRIBUTE_WORD 보다 먼저다.
//   - 마크다운: 라벨은 markdown 으로 그려진다 — _ 가 글자 사이가 아니면 기울임, -·+·= 나 「수.」로 시작하면 목록·구분선.

type jAttr struct {
	typ, name string
	keys      []string
	cmt       string
}

type jEnt struct {
	id, alias string
	attrs     []jAttr
}

type jRel struct {
	a, b        string
	left, right string // 기수 이름(ONLY_ONE …)
	ident       bool
	role        string
}

type jModel struct {
	dir          string
	ents         []*jEnt
	rels         []jRel
	accT, accD   string
	idx          map[string]int
	header       bool
	inBlock      *jEnt
	blockAttrs   []jAttr
	inAcc        bool
	accBuf       []string
	sawIgnorable bool
}

// jSpaceRunes 는 원본 JS \s(ECMAScript WhiteSpace·LineTerminator)와 Go unicode.IsSpace 가운데 ASCII 밖의 것 + 폭 없는
// 서식 문자 — 판정기가 따로 적은 목록이다.
var jSpaceRunes = []rune{0x85, 0xa0, 0x1680, 0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006, 0x2007, 0x2008, 0x2009,
	0x200a, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff, 0x200b, 0x200c, 0x200d, 0x2060}

func jLower(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b.WriteByte(c)
	}
	return b.String()
}

func jIsWS(c byte) bool { return c == ' ' || c == '\t' }

func jTrim(s string) string {
	for len(s) > 0 && jIsWS(s[0]) {
		s = s[1:]
	}
	for len(s) > 0 && jIsWS(s[len(s)-1]) {
		s = s[:len(s)-1]
	}
	return s
}

// jCutLastSemi 은 원본 전처리의 style.*:\S*#.*; (classDef 도) 를 따로 옮긴 것 — 줄에 style(classDef) 뒤 : 뒤 빈칸 없는
// 글 다음 # 이 있고 그 뒤에 ; 가 있으면 마지막 ; 하나를 지운다.
func jCutLastSemi(s, kw string) string {
	for i := 0; i+len(kw) <= len(s); i++ {
		if s[i:i+len(kw)] != kw {
			continue
		}
		for j := i + len(kw); j < len(s); j++ {
			if s[j] != ':' {
				continue
			}
			for k := j + 1; k < len(s); k++ {
				if s[k] == ' ' || s[k] == '\t' {
					break
				}
				if s[k] == '#' {
					if m := strings.LastIndexByte(s, ';'); m > k {
						return s[:m] + s[m+1:]
					}
				}
			}
		}
	}
	return s
}

// jEncode 는 엔티티 #\w+; 를 원본 표식으로 바꾼다(\w 는 ASCII 낱말 글자).
func jEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '#' {
			j := i + 1
			for j < len(s) && (s[j] == '_' || s[j] >= '0' && s[j] <= '9' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z') {
				j++
			}
			if j > i+1 && j < len(s) && s[j] == ';' {
				inner := s[i+1 : j]
				digits := true
				for k := 0; k < len(inner); k++ {
					if inner[k] < '0' || inner[k] > '9' {
						digits = false
					}
				}
				if digits {
					b.WriteString("ﬂ°°" + inner + "¶ß")
				} else {
					b.WriteString("ﬂ°" + inner + "¶ß")
				}
				i = j
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// jDecode 는 표식을 정책 B 로 한 번 푼다 — 이름 7개(quot·amp·lt·gt·apos·num·semi, 대소문자 구분)와 십진 7자리 이하의 수 가운데
// 빈칸·제어·nbsp 가 아닌 글자(판정기 생성 낱말 #35;·#59;·#9731;·#160;·#32;·#copy;·#Quot; 에 맞춘 손 규칙 — 구현의 표를 쓰지
// 않는다). 받지 않는 표식이 있으면 false. 빈칸은 접지 않는다.
func jDecode(s string) (string, bool) {
	var b strings.Builder
	for {
		i := strings.Index(s, "ﬂ°")
		if i < 0 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:i])
		rest := s[i+len("ﬂ°"):]
		num := strings.HasPrefix(rest, "°")
		rest = strings.TrimPrefix(rest, "°")
		j := strings.Index(rest, "¶ß")
		name := rest[:j]
		s = rest[j+len("¶ß"):]
		if !num {
			v, ok := map[string]string{"quot": `"`, "amp": "&", "lt": "<", "gt": ">", "apos": "'", "num": "#", "semi": ";"}[name]
			if !ok {
				return "", false
			}
			b.WriteString(v)
			continue
		}
		n, _ := strconv.Atoi(name)
		if len(name) > 7 || n < 0x21 || n >= 0x7f && n <= 0xa0 || n == 0x2003 {
			return "", false
		}
		b.WriteRune(rune(n))
	}
	return b.String(), true
}

// jLabel 은 따옴표 라벨(전처리 뒤)의 값이다(정책 B) — 앞뒤 빈칸·탭을 지우고, 안쪽에 탭·연속 빈칸이 있으면 false.
func jLabel(c string) (string, bool) {
	c = jTrim(c)
	if strings.Contains(c, "\t") || strings.Contains(c, "  ") {
		return "", false
	}
	return jDecode(c)
}

// jAcc 는 접근성 값(전처리 뒤, 앞뒤를 지운 것)이다(정책 B) — 안쪽 빈칸·탭·줄바꿈은 그대로, 맨 &·<·> 는 false.
func jAcc(v string) (string, bool) {
	if strings.ContainsAny(v, "&<>") {
		return "", false
	}
	return jDecode(v)
}

func jAlnum(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }

func jUnderscoreOK(s string) bool {
	r := []rune(s)
	for i := range r {
		if r[i] == '_' && (i == 0 || i == len(r)-1 || !jAlnum(r[i-1]) || !jAlnum(r[i+1])) {
			return false
		}
	}
	return true
}

// jScanID 는 s 앞의 이름 꼴(글자·_ 로 시작, 글자·숫자·_·- 가 이어짐)을 잘라 돌려준다.
func jScanID(s string) (string, string) {
	i := 0
	for i < len(s) {
		r, n := rune(s[i]), 1
		if r >= 0x80 {
			rs := []rune(s[i:])
			r = rs[0]
			n = len(string(r))
		}
		ok := unicode.IsLetter(r) || r == '_' || i > 0 && (unicode.IsNumber(r) || r == '-')
		if !ok {
			break
		}
		i += n
	}
	return s[:i], s[i:]
}

// jIDok 는 판정기의 이름 규칙이다.
func jIDok(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if r > 0xffff {
			return false
		}
	}
	if strings.HasSuffix(id, "-") || strings.Contains(id, "--") || !jUnderscoreOK(id) {
		return false
	}
	low := jLower(id)
	switch low {
	case "title", "pk", "fk", "uk":
		return false
	}
	if len(low) >= 2 && low[0] == 'u' && low[1] == '-' {
		return false
	}
	// jison 규칙 차례에서 UNICODE_TEXT(93번째 줄)보다 앞이고 이름 글자로 시작하는 낱말
	for _, kw := range []string{"acctitle", "accdescr", "direction", "erdiagram", "style", "classdef", "class",
		"subgraph", "end", "one", "many", "to"} {
		if strings.HasPrefix(low, kw) {
			return false
		}
	}
	return true
}

// jLabelOK 는 따옴표 라벨(전처리 뒤) 규칙이다.
func jLabelOK(s string) bool {
	for _, r := range s {
		if r < 0x20 && r != '\t' || r == 0x7f {
			return false
		}
		switch r {
		case '#', ';', '"', '`', '%', '\\', '<', '>', '&', '*', '~', '$':
			return false
		}
	}
	if !jUnderscoreOK(s) {
		return false
	}
	t := s
	for len(t) > 0 && jIsWS(t[0]) {
		t = t[1:]
	}
	if t != "" && (t[0] == '-' || t[0] == '+' || t[0] == '=') {
		return false
	}
	d := 0
	for d < len(t) && d < 10 && t[d] >= '0' && t[d] <= '9' {
		d++
	}
	if d >= 1 && d <= 9 && d < len(t) && (t[d] == '.' || t[d] == ')') && (d+1 == len(t) || jIsWS(t[d+1])) {
		return false
	}
	return true
}

func jSkipWS(s string) string {
	for len(s) > 0 && jIsWS(s[0]) {
		s = s[1:]
	}
	return s
}

// jIDList 는 이름[,이름…](빈칸 없음)을 읽는다.
func jIDList(s string) (string, bool) {
	for {
		id, rest := jScanID(s)
		if !jIDok(id) {
			return s, false
		}
		s = rest
		if len(s) > 0 && s[0] == ',' {
			s = s[1:]
			continue
		}
		return s, true
	}
}

var jCards = map[string]string{"|o": "ZERO_OR_ONE", "||": "ONLY_ONE", "}o": "ZERO_OR_MORE", "}|": "ONE_OR_MORE"}
var jCardsR = map[string]string{"o|": "ZERO_OR_ONE", "||": "ONLY_ONE", "o{": "ZERO_OR_MORE", "|{": "ONE_OR_MORE"}

func (m *jModel) add(id, alias string) {
	if k, ok := m.idx[id]; ok {
		if m.ents[k].alias == "" {
			m.ents[k].alias = alias
		}
		return
	}
	m.idx[id] = len(m.ents)
	m.ents = append(m.ents, &jEnt{id: id, alias: alias})
}

// jWord 는 속성 타입·이름 한 토큰을 읽는다(원본 ATTRIBUTE_WORD 의 부분집합).
func jWord(s string) (string, string) {
	i := 0
	for i < len(s) {
		rs := []rune(s[i:])
		r := rs[0]
		ok := r < 0x80 && (r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r == '_') ||
			r >= 0xc0 && r <= 0xffff && unicode.IsLetter(r) ||
			i > 0 && r < 0x80 && (r >= '0' && r <= '9' || strings.ContainsRune("()[].,-", r))
		if !ok {
			break
		}
		i += len(string(r))
	}
	return s[:i], s[i:]
}

func jWordOK(w string) bool {
	if w == "" {
		return false
	}
	low := jLower(w)
	for _, k := range []string{"pk", "fk", "uk"} {
		if strings.HasPrefix(low, k) {
			if len(low) == 2 {
				return false
			}
			c := low[2]
			if !(c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z') {
				return false
			}
		}
	}
	return jUnderscoreOK(w) && !strings.Contains(w, "](")
}

// jAttrLine 은 속성 한 줄을 읽는다.
func jAttrLine(t string) (jAttr, bool) {
	var a jAttr
	typ, rest := jWord(t)
	if !jWordOK(typ) {
		return a, false
	}
	if strings.HasPrefix(rest, "?") {
		typ += "?"
		rest = rest[1:]
	}
	if rest == "" || !jIsWS(rest[0]) {
		return a, false
	}
	rest = jSkipWS(rest)
	name, rest := jWord(rest)
	if !jWordOK(name) {
		return a, false
	}
	a.typ, a.name = typ, name
	if rest == "" {
		return a, true
	}
	if !jIsWS(rest[0]) {
		return a, false
	}
	rest = jSkipWS(rest)
	for len(rest) >= 2 && (rest[:2] == "PK" || rest[:2] == "FK" || rest[:2] == "UK") {
		a.keys = append(a.keys, rest[:2])
		rest = rest[2:]
		after := jSkipWS(rest)
		if strings.HasPrefix(after, ",") {
			rest = jSkipWS(after[1:])
			if len(rest) < 2 || !(rest[:2] == "PK" || rest[:2] == "FK" || rest[:2] == "UK") {
				return a, false
			}
			continue
		}
		if rest != "" && !jIsWS(rest[0]) {
			return a, false
		}
		rest = after
		break
	}
	if rest == "" {
		return a, true
	}
	if len(a.keys) == 0 && !jIsWS(t[len(t)-len(rest)-1]) {
		return a, false
	}
	if rest[0] != '"' || len(rest) < 2 || rest[len(rest)-1] != '"' || strings.Contains(rest[1:len(rest)-1], `"`) {
		return a, false
	}
	c := rest[1 : len(rest)-1]
	if !jLabelOK(c) {
		return a, false
	}
	v, ok := jLabel(c)
	a.cmt = v
	return a, ok
}

// jAccBody 는 accDescr { … } 본문 조각(이미 원문 전처리 — 지시·주석 판정·엔티티 지키기 — 를 한 번 거친 글)을 모은다.
// 여는 줄의 { 뒤와 다음 원문 줄들이 같은 길로 온다. 본문 안의 %% 는 줄 머리가 아니면 주석이 아니고(원본 cleanupComments 는
// 줄 머리 ^\s*%% 만), 엔티티 표식은 원문 표식이 아니라 전처리가 만든 것이다 — 원문 줄로 다시 판정하지 않는다.
func (m *jModel) jAccBody(t string) bool {
	before, after, closed := strings.Cut(t, "}")
	m.accBuf = append(m.accBuf, strings.TrimLeft(before, " \t"))
	if closed {
		if jTrim(after) != "" {
			return false
		}
		m.inAcc = false
		// 정책 B — 블록 글 전체를 trim 하고 줄 머리 빈칸·빈 줄을 지운다(원본 setAccDescription 의 \n\s+ → \n). 줄 끝 빈칸·안쪽
		// 빈칸은 그대로. 빈 블록은 앞 설명을 그대로 둔다.
		v := strings.Trim(strings.Join(m.accBuf, "\n"), " \t\n")
		for strings.Contains(v, "\n ") || strings.Contains(v, "\n\t") || strings.Contains(v, "\n\n") {
			v = strings.NewReplacer("\n ", "\n", "\n\t", "\n", "\n\n", "\n").Replace(v)
		}
		if v != "" {
			d, ok := jAcc(v)
			if !ok {
				return false
			}
			m.accD = d
		}
	}
	return true
}

// jLine 은 한 줄을 판정한다(거짓이면 원문 + 경고).
func (m *jModel) jLine(raw string) bool {
	raw = strings.TrimSuffix(raw, "\r")
	if strings.ContainsAny(raw, "\r\f\v") {
		return false
	}
	for _, r := range raw {
		for _, s := range jSpaceRunes {
			if r == s {
				return false
			}
		}
	}
	t := jTrim(raw)
	if strings.Contains(t, "ﬂ°") || strings.Contains(t, "¶ß") {
		return false
	}
	// 설정 지시: 판정기 규칙 — %%{ 가 든 줄은 줄 전체가 「%%{ … }%%」 하나(사이에 %% 없음)이고 블록·acc 블록 밖일 때만
	// 받는다(무시하는 지시). 원본 removeDirectives 는 지시만 지우고 남은 글을 읽지만 이 판은 그 갈래를 원문으로 돌린다.
	if strings.Contains(t, "%%{") {
		n := len(t)
		single := n >= 6 && t[:3] == "%%{" && t[n-3:] == "}%%" && !strings.Contains(t[2:n-2], "%%")
		return single && !m.inAcc && m.inBlock == nil
	}
	if strings.HasPrefix(t, "%%") {
		return jSkipWS(raw) != "%%"
	}
	// 블록형 accDescr 의 글은 줄 끝 빈칸까지 값이다 — 앞만 지운 줄(tl)로 읽는다.
	tl := jEncode(jCutLastSemi(jCutLastSemi(strings.TrimLeft(raw, " \t"), "style"), "classDef"))
	t = jEncode(jCutLastSemi(jCutLastSemi(t, "style"), "classDef"))
	if m.inAcc {
		return m.jAccBody(tl)
	}
	if m.inBlock != nil {
		if t == "" {
			return true
		}
		if t == "}" {
			m.inBlock.attrs = append(m.inBlock.attrs, m.blockAttrs...)
			m.inBlock, m.blockAttrs = nil, nil
			return true
		}
		a, ok := jAttrLine(t)
		if !ok {
			return false
		}
		m.blockAttrs = append(m.blockAttrs, a)
		return true
	}
	if t == "" {
		return true
	}
	if !m.header {
		m.header = t == "erDiagram"
		return m.header
	}
	low := jLower(t)
	for _, kw := range []string{"acctitle", "accdescr"} {
		if !strings.HasPrefix(low, kw) {
			continue
		}
		r := jSkipWS(t[len(kw):])
		if r != "" && r[0] == ':' {
			v := jSkipWS(r[1:])
			if v == "" {
				return false
			}
			d, ok := jAcc(v)
			if kw == "acctitle" {
				m.accT = d
			} else {
				m.accD = d
			}
			return ok
		}
		if kw == "accdescr" && r != "" && r[0] == '{' {
			m.inAcc, m.accBuf = true, nil
			_, open, _ := strings.Cut(tl, "{")
			return m.jAccBody(open) // 여는 줄의 { 뒤(줄 끝 빈칸 포함) — 이 줄은 위에서 이미 전처리를 한 번 거쳤다(리뷰 2회차)
		}
	}
	if strings.Contains(low, "direction") {
		if !strings.HasPrefix(low, "direction") || len(low) <= len("direction") || !jIsWS(low[len("direction")]) {
			return false
		}
		v := jSkipWS(low[len("direction"):])
		switch v {
		case "tb", "bt", "lr", "rl":
			m.dir = strings.ToUpper(v)
			return true
		}
		return false
	}
	for _, kw := range []string{"style", "classdef"} {
		if !strings.HasPrefix(low, kw) || len(t) == len(kw) || !jIsWS(t[len(kw)]) {
			continue
		}
		rest, ok := jIDList(jSkipWS(t[len(kw):]))
		if !ok || rest == "" || !jIsWS(rest[0]) {
			return false
		}
		rest = jSkipWS(rest)
		// 값: , 로 나뉜 조각마다 [A-Za-z0-9_:#-] 낱말이 빈칸으로 이어진 것(빈 조각 없음)
		for _, piece := range strings.Split(rest, ",") {
			piece = jTrim(piece)
			if piece == "" {
				return false
			}
			for _, w := range strings.FieldsFunc(piece, func(r rune) bool { return r == ' ' || r == '\t' }) {
				for _, r := range w {
					if !(r < 0x80 && (r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || strings.ContainsRune("_:#-", r))) {
						return false
					}
				}
			}
		}
		m.sawIgnorable = true
		return true
	}
	if strings.HasPrefix(low, "class") && len(t) > 5 && jIsWS(t[5]) {
		rest, ok := jIDList(jSkipWS(t[5:]))
		if ok && rest != "" && jIsWS(rest[0]) {
			if rest, ok = jIDList(jSkipWS(rest)); ok && rest == "" {
				m.sawIgnorable = true
				return true
			}
		}
		return false
	}
	// 엔티티 또는 관계 — 이름 [:::목록]
	id, rest := jScanID(t)
	if !jIDok(id) {
		return false
	}
	if strings.HasPrefix(rest, ":::") {
		var ok bool
		if rest, ok = jIDList(rest[3:]); !ok {
			return false
		}
	}
	// 관계: 빈칸 + 왼기호 선 오른기호(여섯 글자) + 빈칸 + 이름 [:::목록] 빈칸* : 빈칸* 역할
	if len(rest) > 0 && jIsWS(rest[0]) {
		r2 := jSkipWS(rest)
		if len(r2) >= 6 {
			l, ln, rr := r2[:2], r2[2:4], r2[4:6]
			lc, okL := jCards[l]
			rc, okR := jCardsR[rr]
			if okL && okR && (ln == "--" || ln == "..") {
				r3 := r2[6:]
				if r3 == "" || !jIsWS(r3[0]) {
					return false
				}
				b, r4 := jScanID(jSkipWS(r3))
				if !jIDok(b) {
					return false
				}
				if strings.HasPrefix(r4, ":::") {
					var ok bool
					if r4, ok = jIDList(r4[3:]); !ok {
						return false
					}
				}
				r4 = jSkipWS(r4)
				if !strings.HasPrefix(r4, ":") {
					return false
				}
				r4 = jSkipWS(r4[1:])
				role := ""
				if strings.HasPrefix(r4, `"`) {
					if len(r4) < 2 || r4[len(r4)-1] != '"' || strings.Contains(r4[1:len(r4)-1], `"`) || !jLabelOK(r4[1:len(r4)-1]) {
						return false
					}
					var ok bool
					if role, ok = jLabel(r4[1 : len(r4)-1]); !ok {
						return false
					}
				} else {
					var after string
					role, after = jScanID(r4)
					if after != "" || !jIDok(role) {
						return false
					}
				}
				m.add(id, "")
				m.add(b, "")
				m.rels = append(m.rels, jRel{a: id, b: b, left: lc, right: rc, ident: ln == "--", role: role})
				return true
			}
		}
	}
	// 엔티티: [별칭] [:::목록] [빈칸* { [빈칸* }]]
	alias := ""
	if strings.HasPrefix(rest, "[") {
		body := rest[1:]
		if strings.HasPrefix(body, `"`) {
			end := strings.Index(body[1:], `"`)
			if end < 0 {
				return false
			}
			c := body[1 : 1+end]
			if c == "" || !jLabelOK(c) || !strings.HasPrefix(body[2+end:], "]") {
				return false
			}
			var ok bool
			alias, ok = jLabel(c)
			if !ok || alias == "" {
				return false
			}
			rest = body[3+end:]
		} else {
			a, r := jScanID(body)
			if !jIDok(a) || !strings.HasPrefix(r, "]") {
				return false
			}
			alias, rest = a, r[1:]
		}
	}
	if strings.HasPrefix(rest, ":::") {
		var ok bool
		if rest, ok = jIDList(rest[3:]); !ok {
			return false
		}
	}
	rest = jSkipWS(rest)
	open := false
	if strings.HasPrefix(rest, "{") {
		r := jSkipWS(rest[1:])
		switch r {
		case "":
			open = true
		case "}":
		default:
			return false
		}
		rest = ""
	}
	if rest != "" {
		return false
	}
	m.add(id, alias)
	if open {
		m.inBlock = m.ents[m.idx[id]]
	}
	return true
}

// jJudge 는 원문 전체를 판정한다 — 받아들이면 모델, 아니면 거부 줄(1부터).
func jJudge(src string) (*jModel, int) {
	m := &jModel{dir: "TB", idx: map[string]int{}}
	lines := strings.Split(src, "\n")
	for i, l := range lines {
		if !m.jLine(l) {
			return nil, i + 1
		}
	}
	switch {
	case m.inAcc, m.inBlock != nil:
		return nil, -1 // 닫히지 않음(줄은 연 줄)
	case !m.header || len(m.ents) == 0:
		return nil, 0
	}
	return m, 0
}

// ---- 생성기: 경계 토큰을 명시적으로 조합한다

var (
	jWS     = []string{"", " ", "\t", "  "}
	jIDs    = []string{"A", "B", "고객", "주문_항목", "LINE-ITEM", "x9", "Ab"}
	jTrapID = []string{"to", "TOKEN", "tO", "One", "ONEROUS", "one_x", "many", "MANYx", "Class", "classroom", "CLASSDEF", "end", "End_x",
		"ENDPOINT", "u-x", "U-1", "ux", "style", "Styles", "subgraph", "erDiagram", "ERDIAGRAMx", "title", "Title", "pk", "Fk", "UK",
		"pkey", "accTitle", "accDescrX", "direction", "Redirection", "a--b", "a-", "-a", "_a", "a_", "a_b", "a__b", "𝔸", "a𝔸",
		"ª", "µx", "zero", "only", "optionally", "o", "u", "가end", "Atitle", "직원to"}
	jLabels = []string{"", "x", "한 글", "a b c", "a#b", "a;b", "a%%b", "a%b", "a~b", "a*b", "a`b", "#35;", "x #59; y", "#quot;인용#quot;",
		"#lt;b#gt;", "#9731;", "a_b", "_a_", "a _b", "- x", "+x", "=x", "1. x", "1.5", "12) y", "1.x", "  sp  ", "a\\b", "a$b",
		"a<b", "a>b", "a&b", "a[1](b)", "a{b}", "a:b", "x\u00a0y", "x\ufeffy", "x\ty", "style x:#35;", "가end",
		// 정책 B 경계 — 안쪽 연속 빈칸, 받지 않는 엔티티(이름·공백 수·대소문자), 한 번만 푸는 엔티티
		"x  y", "#copy; 2026", "x#160;y", "x#32;y", "#Quot;", "#35;35;", "#apos;"}
	jTypes = []string{"string", "int?", "decimal(10,2)", "varchar[]", "a~T~", "*int", "PKey", "pk", "PK-x", "Fk", "uk1", "UK_x",
		"타입", "ªx", "x_", "_x", "a](b)", "a.b", "a,b", "날짜?", "𝔸", "x`y"}
	jKeys  = []string{"", "PK", "PK, FK", "PK,FK", "FK , UK", "pk", "PK FK", "PK,", "PKX", "UK"}
	jSymsL = []string{"|o", "||", "}o", "}|", "o|", "|{", "|O", "}O"}
	jSymsR = []string{"o|", "||", "o{", "|{", "|o", "}o", "O|", "o}"}
	jLns   = []string{"--", "..", ".-", "-.", "-", "---"}
	jOdd   = []string{"\u00a0", "\ufeff", "\u2003", "\u200b", "\r", "\f"}
)

func pick(rng *rand.Rand, xs []string) string { return xs[rng.Intn(len(xs))] }

// jClean 이 참인 판은 함정 토큰을 드물게 섞는다 — 받아들이는 판도 충분히 나오게(판마다 jDoc 가 정한다).
var jClean bool

// jTrap 은 함정 토큰을 쓸지다(깨끗한 판 1/40, 아니면 1/3).
func jTrap(rng *rand.Rand) bool {
	if jClean {
		return rng.Intn(40) == 0
	}
	return rng.Intn(3) == 0
}

// jSafe 는 함정 판에서는 xs 전체, 깨끗한 판에서는 앞 n 개(받는 꼴)에서 고른다.
func jSafe(rng *rand.Rand, xs []string, n int) string {
	if jTrap(rng) {
		return pick(rng, xs)
	}
	return pick(rng, xs[:n])
}

func jID(rng *rand.Rand) string {
	if jTrap(rng) {
		return pick(rng, jTrapID)
	}
	return pick(rng, jIDs)
}

func jQuoted(rng *rand.Rand) string {
	s := jSafe(rng, jLabels, 4)
	if rng.Intn(4) == 0 {
		s += jSafe(rng, jLabels, 4)
	}
	return `"` + s + `"`
}

func jRole(rng *rand.Rand) string {
	if rng.Intn(2) == 0 {
		return jQuoted(rng)
	}
	return jID(rng)
}

func jClassList(rng *rand.Rand) string {
	s := jID(rng)
	for rng.Intn(3) == 0 {
		s += "," + jID(rng)
	}
	return s
}

// jStmt 는 블록 밖의 줄 하나를 만든다.
func jStmt(rng *rand.Rand) string {
	w := func() string { return pick(rng, jWS) }
	if jClean && rng.Intn(5) == 0 {
		return jID(rng) + " {" // 깨끗한 판은 속성 블록을 더 자주 연다
	}
	switch rng.Intn(16) {
	case 0, 1, 2, 3: // 관계
		a := jID(rng)
		if rng.Intn(6) == 0 {
			a += ":::" + jClassList(rng)
		}
		b := jID(rng)
		if rng.Intn(6) == 0 {
			b += ":::" + jClassList(rng)
		}
		sp1, sp2 := " ", " "
		if rng.Intn(6) == 0 {
			sp1 = w()
		}
		if rng.Intn(6) == 0 {
			sp2 = w()
		}
		sym := pick(rng, jSymsL[:4]) + pick(rng, jLns[:2]) + pick(rng, jSymsR[:4])
		if jTrap(rng) {
			sym = pick(rng, jSymsL) + pick(rng, jLns) + pick(rng, jSymsR)
		}
		if jTrap(rng) && rng.Intn(3) == 0 {
			sym = pick(rng, jSymsL) + " " + pick(rng, jLns) + " " + pick(rng, jSymsR)
		}
		colon := w() + ":" + w()
		if jTrap(rng) && rng.Intn(4) == 0 {
			colon = " "
		}
		return a + sp1 + sym + sp2 + b + colon + jRole(rng)
	case 4, 5, 6: // 엔티티 선언
		s := jID(rng)
		switch rng.Intn(4) {
		case 1:
			s += "[" + jID(rng) + "]"
		case 2:
			s += "[" + jQuoted(rng) + "]"
		}
		if rng.Intn(5) == 0 {
			s += ":::" + jClassList(rng)
		}
		switch rng.Intn(5) {
		case 1:
			s += w() + "{}"
		case 2:
			s += " { }"
		}
		return s
	case 7: // 방향 — 줄 머리·줄 중간·줄 끝(다음 줄 LR 은 jDoc 가 붙인다)
		dirs := []string{"TB", "BT", "LR", "RL", "lr", "Tb", "TD", "", "LRX"}
		if !jTrap(rng) {
			return pick(rng, []string{"direction", "Direction", "DIRECTION"}) + pick(rng, []string{" ", "\t", "  "}) + pick(rng, dirs[:6])
		}
		switch rng.Intn(4) {
		case 0:
			return pick(rng, []string{"direction", "Direction", "DIRECTION"}) + pick(rng, []string{" ", "\t", "  ", ""}) + pick(rng, dirs)
		case 1:
			return jID(rng) + " ||--o{ " + jID(rng) + " : direction"
		case 2:
			return jID(rng) + "[\"x direction LR\"]"
		}
		return "direction"
	case 8: // 접근성
		switch rng.Intn(4) {
		case 0:
			return "accTitle" + w() + ":" + w() + pick(rng, jLabels)
		case 1:
			return "accDescr" + w() + ":" + w() + pick(rng, jLabels)
		case 2:
			return "ACCTITLE: " + pick(rng, jLabels)
		}
		// 여는 줄 { 뒤 — 엔티티·%% 일반 글·맨 %%·라벨 함정(리뷰 2회차: 판정기가 이 자리를 원문으로 다시 읽던 것)
		body := pick(rng, []string{"", " 한 줄", " 열기", " #35;", " #quot;x", " #lt;b#gt;", " %% 메모", " %%", " %%%% x", " " + pick(rng, jLabels),
			" a #35;", "#59;"})
		return "accDescr {" + body + pick(rng, []string{"", " }", "}", " }x", " } "})
	case 9: // 무시 줄
		if !jTrap(rng) {
			return pick(rng, []string{"style " + jID(rng) + " fill:#f9f,stroke:#333", "classDef " + jID(rng) + " fill:#f00",
				"class " + jID(rng) + " " + jID(rng), "%% 주석", "%%{init: {}}%%"})
		}
		switch rng.Intn(5) {
		case 0:
			return "style " + jClassList(rng) + " fill:#f9f,stroke:#333" + pick(rng, []string{"", ";", " stroke-width:2px", ",", " rgb(1,2,3)"})
		case 1:
			return "classDef " + jClassList(rng) + " " + pick(rng, []string{"fill:#f00", "color:red , stroke:blue", "fill:#f00;", "", "a b"})
		case 2:
			return "class " + jClassList(rng) + " " + jClassList(rng) + pick(rng, []string{"", " x", ";"})
		case 3:
			return jDirective(rng)
		}
		return pick(rng, []string{"title 제목", "subgraph s", "end", "}", "A one or more B : x", "\"Q A\"", "A B", "A { int a }"})
	case 11: // 설정 지시 경계(리뷰 1회차)
		return jDirective(rng)
	case 10: // 블록 열기 — 속성 줄과 닫기는 jDoc 가 붙인다
		s := jID(rng)
		if rng.Intn(3) == 0 {
			s += "[" + jQuoted(rng) + "]"
		}
		return s + pick(rng, []string{" {", "{", "\t{"})
	}
	return jID(rng) + " ||--o{ " + jID(rng) + " : " + jRole(rng)
}

func jAttrGen(rng *rand.Rand) string {
	if jTrap(rng) && rng.Intn(3) == 0 {
		return pick(rng, []string{"%% 주석", "", "string a }", "int", "%%{init: {}}%%", "direction LR", "string a \"" + jDir + "\"", "string a " + jDir})
	}
	s := jSafe(rng, jTypes, 4) + jSafe(rng, []string{" ", "\t", "  ", ""}, 3) + pick(rng, jTypes[:4]) + pick(rng, []string{"", "", "", "_1", "x"})
	if k := jSafe(rng, jKeys, 4); k != "" {
		s += " " + k
	}
	if rng.Intn(3) == 0 {
		s += " " + jQuoted(rng)
	}
	return s
}

// jDir 는 닫힌 설정 지시 하나다.
const jDir = "%%{init: {}}%%"

// jDirective 는 설정 지시 경계 줄이다 — 지시 하나·앞/뒤/사이 글·지시 둘·지시 안 %%·acc 값 안·라벨 안·주석 안·닫히지 않음.
func jDirective(rng *rand.Rand) string {
	id := jID(rng)
	return pick(rng, []string{jDir, jDir + "  ", jDir + " " + id, id + " " + jDir, jDir + " " + id + " " + jDir, jDir + " ??? " + jDir,
		jDir + jDir, "%%{init: {\"a\": \"%%\"}}%%", "%%{x}%%", "%%{}%%", "%%{init: {", "%%{a %%{b}%%", "%% 주석", "%%", "  %% x",
		"%% 주석 " + jDir, "accTitle: 앞 " + jDir + " 뒤", "accDescr: " + jDir, "accDescr: 앞 " + jDir, "accTitle: %%{init: {",
		id + "[\"x " + jDir + "\"]", id + " ||--o{ " + jID(rng) + " : \"" + jDir + "\"", id + " ||--o{ " + jID(rng) + " : x " + jDir,
		"style " + id + " fill:#f9f " + jDir, "direction LR " + jDir})
}

// jDoc 은 머리 줄 + 무작위 줄들(블록 여닫기 상태 전이 포함)이다. 가끔 줄 하나에 ASCII 밖 공백을 끼운다.
func jDoc(rng *rand.Rand) string {
	jClean = rng.Intn(2) == 0
	lines := []string{"erDiagram"}
	if jTrap(rng) && rng.Intn(6) == 0 {
		lines[0] = pick(rng, []string{"ERDiagram", "erDiagram x", " erDiagram", "%%{init: {}}%%\nerDiagram"})
	}
	n := 1 + rng.Intn(6)
	for i := 0; i < n; i++ {
		st := jStmt(rng)
		lines = append(lines, pick(rng, jWS)+st+pick(rng, jWS))
		if strings.HasSuffix(st, "{") && !strings.HasPrefix(st, "accDescr") {
			for k := rng.Intn(4); k > 0; k-- {
				lines = append(lines, "  "+jAttrGen(rng))
			}
			if !jTrap(rng) || rng.Intn(3) != 0 {
				lines = append(lines, jSafe(rng, []string{"}", "  }", "} x"}, 2))
			}
		}
		if strings.HasPrefix(st, "accDescr {") && !strings.Contains(st, "}") {
			lines = append(lines, pick(rng, jLabels), pick(rng, []string{"}", "  }", "} 뒤", "%%{a}%%\n}", "%%{a\n}"}))
		}
		if strings.HasSuffix(st, "direction") && rng.Intn(2) == 0 {
			lines = append(lines, "LR") // 다음 줄 LR — 원본은 \s+ 가 줄바꿈을 넘어 방향 문장으로 먹는다
		}
	}
	if jTrap(rng) && rng.Intn(2) == 0 {
		k := 1 + rng.Intn(len(lines)-1)
		r := []rune(lines[k])
		at := rng.Intn(len(r) + 1)
		lines[k] = string(r[:at]) + pick(rng, jOdd) + string(r[at:])
	}
	return strings.Join(lines, "\n")
}

func TestJudgeBoundaryProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(4094))
	accepted, rejected := 0, 0
	count := map[string]int{}
	for round := 0; round < 12000; round++ {
		src := jDoc(rng)
		want, badLine := jJudge(src)
		if strings.Contains(src, "%%{") {
			if want != nil {
				count["지시 받음"]++
			} else {
				count["지시 거부"]++
			}
		}
		c, _, err := Parse(src)
		if want == nil {
			rejected++
			if err == nil {
				t.Fatalf("round %d: 판정기는 거부(줄 %d)인데 Parse 가 받음\n%s\n결과 %s", round, badLine, src, summary(c))
			}
			var de *diagram.Error
			if !errors.As(err, &de) {
				t.Fatalf("round %d: 오류가 *Error 가 아님: %v", round, err)
			}
			if badLine > 0 && de.Line != badLine {
				t.Fatalf("round %d: 거부 줄 판정기 %d, Parse %d(%s)\n%s", round, badLine, de.Line, de.Msg, src)
			}
			continue
		}
		if err != nil {
			t.Fatalf("round %d: 판정기는 받는데 Parse 가 거부: %v\n%s", round, err, src)
		}
		accepted++
		if got, exp := summary(c), jSummary(want); got != exp {
			t.Fatalf("round %d: 값이 다름\n Parse  %s\n 판정기 %s\n%s", round, got, exp, src)
		}
		for _, r := range want.rels {
			count["rel"]++
			if r.a == r.b {
				count["self"]++
			}
		}
		for _, e := range want.ents {
			if e.alias != "" {
				count["alias"]++
			}
			count["attr"] += len(e.attrs)
		}
		if want.dir != "TB" {
			count["dir"]++
		}
		if want.accT != "" || want.accD != "" {
			count["acc"]++
		}
		if want.sawIgnorable {
			count["ignored"]++
		}
	}
	t.Logf("받음 %d · 거부 %d · %v", accepted, rejected, count)
	if accepted < 1500 || rejected < 1500 {
		t.Errorf("판수가 한쪽으로 쏠림: 받음 %d · 거부 %d", accepted, rejected)
	}
	for _, k := range []string{"rel", "self", "alias", "attr", "dir", "acc", "ignored", "지시 받음", "지시 거부"} {
		if count[k] < 50 {
			t.Errorf("받아들인 판에 %s 가 %d 번뿐", k, count[k])
		}
	}
}

// jSummary 는 판정기 모델을 summary 와 같은 꼴로 줄인다.
func jSummary(m *jModel) string {
	var b strings.Builder
	fmt.Fprintf(&b, "dir=%d", map[string]int{"TB": 0, "BT": 1, "LR": 2, "RL": 3}[m.dir])
	for _, e := range m.ents {
		b.WriteString(" " + e.id)
		if e.alias != "" {
			b.WriteString("[" + e.alias + "]")
		}
		if len(e.attrs) > 0 {
			b.WriteString("{")
			for i, a := range e.attrs {
				if i > 0 {
					b.WriteString("; ")
				}
				b.WriteString(a.typ + " " + a.name)
				if len(a.keys) > 0 {
					b.WriteString(" " + strings.Join(a.keys, ","))
				}
				if a.cmt != "" {
					b.WriteString(` "` + a.cmt + `"`)
				}
			}
			b.WriteString("}")
		}
	}
	for _, r := range m.rels {
		line := "--"
		if !r.ident {
			line = ".."
		}
		fmt.Fprintf(&b, " | %s %s%s%s %s %q", r.a, r.left, line, r.right, r.b, r.role)
	}
	if m.accT != "" || m.accD != "" {
		fmt.Fprintf(&b, " acc=%q/%q", m.accT, m.accD)
	}
	return b.String()
}
