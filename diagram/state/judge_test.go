package state

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/itda-work/itda-doc/diagram"
)

// 독립 판정기(itda-hyve#409 ④-2 — er ④-1 judge_test.go 꼴). 허용 꼴과 원본 lexer·DB 의 함정을 이 파일 안에 따로 옮겨 적은 손
// 스캐너로, 무작위 원문마다 받아들일지와 받아들이면 그 값(상태 차례·종류·설명, 묶음 소속, 전이 양끝(시작·끝 상태 포함)·라벨,
// 묶음 차례·제목, 방향, 접근성 값, 캡션)을 스스로 구해 Parse 와 견준다. 구현의 정규식·상수·도우미(idOK·labelOK·
// keywordPrefixes·reTrans …)는 쓰지 않는다 — 공통 패키지의 전처리(EncodeEntities·DirectiveProblem·OddSpace)도 쓰지 않고 따로 적는다.
//
// 판정기가 옮긴 원본 규칙(mermaid@12.2.0):
//   - 전처리: CRLF → LF(줄 중간 CR 은 줄바꿈 — 받지 않음), 줄 머리 %% 주석 줄 지움(글 없는 맨 %% 는 lexer 주석 규칙이
//     건너뜀 — accDescr { } 안에서는 설명 글이 되어 받지 않음), %%{…}%% 지시 지움, style/classDef …:…#…; 의 끝 ; 자르기,
//     엔티티 #\w+; 지키기(표식의 앞뒤 글자 ﬂ·ß 는 글자라 _ 의 이웃이 될 수 있다)
//   - detector: /^\s*stateDiagram/ 와 lexer "stateDiagram"\s+ · "stateDiagram-v2"\s+ — 머리는 정확히 그 둘
//   - lexer: case-insensitive(ASCII 만), 규칙 차례의 첫 일치 — ID 규칙 앞의 click·href·default(\b)·direction(줄 어디서든)·
//     scale·accTitle·accDescr·classDef·class·style·state·note·stateDiagram·hide empty description 으로 시작하는 이름은
//     갈린다. # 는 줄 끝까지 주석, 이름 안의 %% 는 processId 가 자른다, DESCR 은 ":" 뒤 (?:[^:\n;]|":"[^:\n;])+
//   - DB: [*] 는 범위(root·복합 상태 id) + _start(왼끝)·_end(오른끝), 상태는 처음 나온 차례, 설명은 쌓이고, 묶음 소속은
//     복합 상태 안에서 나오면 정해진다(뿌리에서 다시 나와도 지워지지 않는다), 특수 상태 모양은 처음 만들 때만 정한다,
//     방향은 뿌리의 마지막 direction, root 는 상태로 그리지 않는다
//   - 마크다운: 라벨은 markdown 으로 그려진다 — _ 가 글자 사이가 아니면 기울임, -·+·= 나 「수.」로 시작하면 목록·구분선,
//     *·`·<·>·[·] 등은 서식

type jState struct {
	id, kind string
	descr    []string
	comp     int
	line     int
}

type jTrans struct{ a, b, label string }

type jComp struct {
	id, title string
	line, n   int
}

type jModel struct {
	dir                 string
	caption, accT, accD string
	captionLine         int // frontmatter title 줄(정책 B — 다섯 종류와 같은 diagram.Caption)
	states              []*jState
	idx                 map[string]int
	comps               []*jComp
	cidx                map[string]int
	trans               []jTrans
	header              bool
	cur                 int // 열린 복합 상태, -1
	inAcc               bool
	accLine             int
	accBuf              []string
	raw                 string // 지금 줄(앞 빈칸만 지운 것) — accDescr { … } 의 값은 줄 끝 빈칸까지다
	sawIgnorable        bool
	failLine            int // 거부 줄(받아들이면 -1)
}

// jSpaceRunes 는 원본 JS \s(ECMAScript WhiteSpace·LineTerminator)와 Go unicode.IsSpace 가운데 ASCII 밖의 것 + 폭 없는
// 서식 문자 — 판정기가 따로 적은 목록이다.
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

// jWord 는 s[i:] 가 낱말 w(ASCII, 대소문자 무시)로 시작하면 그 뒤 자리를 돌려준다.
func jWord(s string, i int, w string) (int, bool) {
	if len(s)-i < len(w) || jLower(s[i:i+len(w)]) != w {
		return i, false
	}
	return i + len(w), true
}

// jCutSemi 는 원본 encodeEntities 의 앞 두 줄(style·classDef …:\S*#…; 의 마지막 ; 를 지운다)을 손으로 옮긴 것이다 —
// kw 가 처음 나온 뒤에 `:` 와, 그 콜론에서 빈칸 없이 이어진 `#`, 그 # 뒤의 `;` 가 있으면 줄의 마지막 ; 를 지운다.
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

// jTok 는 라벨의 글자 하나 또는 엔티티(#\w+;) 하나다.
type jTok struct {
	r   rune
	ent string // 엔티티면 이름
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

// 엔티티 정책 B(접근 방식 리뷰 2장 수락 규칙 1~6 — 판정기가 따로 적는다. 구현의 entityNames·numOK·unicode 표를 쓰지 않는다).
// 원본은 전처리가 지킨 `#\w+;`(jTokens 의 엔티티 토큰)만 SVG 정리 단계에서 `&#N;`·`&이름;` 으로 한 번 되돌린다 — 토큰 밖의
// 글자(`#+65;`·`#-65;`·`itda-hyve#65`·`# 65;`)는 엔티티가 아니고, 되돌린 글자는 다시 읽지 않는다(`#35;35;` → `#35;`).
//
//	이름 7개(대소문자 구분) — nbsp(U+00A0, 공백)는 뺀다. copy·euro·hellip(원본에서는 ©·€·…)·zzz·AMP·x41(원본 \w+ 라 이름
//	          표식 — `&x41;` 는 숫자 참조가 아니다)는 이 판의 미지원이다.
//	숫자     ASCII 숫자 1~7자리(앞 0 허용), U+0021~U+10FFFF, 서러게이트·Cc·Cf·White_Space·비문자를 뺀다(Unicode 17.0.0 — 아래 목록).
var jNamed = map[string]rune{"quot": 0x22, "amp": 0x26, "lt": 0x3c, "gt": 0x3e, "apos": 0x27, "num": 0x23, "semi": 0x3b}

// jCfRanges 는 Unicode 17.0.0 의 General_Category=Cf(서식) 구간이다(UnicodeData.txt 에서 옮겨 적은 목록).
var jCfRanges = [][2]int{{0xad, 0xad}, {0x600, 0x605}, {0x61c, 0x61c}, {0x6dd, 0x6dd}, {0x70f, 0x70f}, {0x890, 0x891}, {0x8e2, 0x8e2},
	{0x180e, 0x180e}, {0x200b, 0x200f}, {0x202a, 0x202e}, {0x2060, 0x2064}, {0x2066, 0x206f}, {0xfeff, 0xfeff}, {0xfff9, 0xfffb},
	{0x110bd, 0x110bd}, {0x110cd, 0x110cd}, {0x13430, 0x1343f}, {0x1bca0, 0x1bca3}, {0x1d173, 0x1d17a}, {0xe0001, 0xe0001},
	{0xe0020, 0xe007f}}

// jWSRanges 는 Unicode 17.0.0 PropList.txt 의 White_Space 구간이다.
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

// jNumPolicy 는 숫자 엔티티의 값 n 을 받는지다.
func jNumPolicy(n int) bool {
	switch {
	case n < 0x21 || n > 0x10ffff: // 0x20 이하는 제어·공백
		return false
	case n >= 0xd800 && n <= 0xdfff: // 서러게이트
		return false
	case n >= 0x7f && n <= 0x9f: // Cc(C1·DEL)
		return false
	case n >= 0xfdd0 && n <= 0xfdef, n&0xfffe == 0xfffe: // 비문자 — FDD0–FDEF 와 면마다 끝 둘
		return false
	}
	return !jIn(n, jCfRanges) && !jIn(n, jWSRanges)
}

// jXMLChar 는 XML 1.0 Char(#x9 | #xA | #xD | [#x20-#xD7FF] | [#xE000-#xFFFD] | [#x10000-#x10FFFF])다 — 최종 SVG 에 담기는
// 글자. 그 밖의 글자는 SVG 쓰기 경로가 U+FFFD 로 바꾸므로 받은 값에 들면 그 줄을 받지 않는다(리뷰 3회차 [1]).
func jXMLChar(r rune) bool {
	return r == 0x9 || r == 0xa || r == 0xd || r >= 0x20 && r <= 0xd7ff || r >= 0xe000 && r <= 0xfffd || r >= 0x10000 && r <= 0x10ffff
}

func jXMLOK(s string) bool {
	for _, r := range s {
		if !jXMLChar(r) {
			return false
		}
	}
	return true
}

// jEntity 는 엔티티 토큰 하나(이름)를 글자로 푼다. 모두 ASCII 숫자면 원본 정수 표식이다.
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

// jRestore 는 글의 엔티티 토큰만 글자로 한 번 바꾼다(그 밖의 글자는 그대로 — 빈칸을 접지 않는다). 받지 않는 엔티티가 있으면 false.
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

// jAccValue 는 접근성 값(원본처럼 다듬은 것)을 본다 — 탭·줄바꿈 밖의 제어 문자와 맨 &·<·>(원본 HTML 정제가 글을 바꾼다)는
// 받지 않고, 엔티티 토큰만 되돌린다. 200 자 상한.
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

func jAlnum(t jTok) bool { return t.ent != "" || unicode.IsLetter(t.r) || unicode.IsNumber(t.r) }

// jLabel 은 라벨(설명·전이 라벨·복합 상태 설명)을 보고 값을 구한다.
func jLabel(raw string) (string, bool) {
	s := jTrim(raw)
	if s == "" {
		return "", false
	}
	ts := jTokens(s)
	for k, t := range ts {
		if t.ent != "" {
			// 엔티티 이름 안의 _ 도 이름 글자 사이여야 한다(표식 ﬂ°·¶ß 의 °·¶ 는 글자가 아니다)
			for p := 0; p < len(t.ent); p++ {
				if t.ent[p] == '_' && (p == 0 || p == len(t.ent)-1 || t.ent[p-1] == '_' || t.ent[p+1] == '_') {
					return "", false
				}
			}
			continue
		}
		r := t.r
		if r < 0x20 && r != '\t' || r == 0x7f || strings.ContainsRune("#;\"`%\\<>&*~$[]{}", r) {
			return "", false
		}
		// 안쪽 빈칸은 한 칸씩만 — 탭과 연속 빈칸은 받지 않는다(접근 방식 리뷰 5장)
		if r == '\t' || r == ' ' && k+1 < len(ts) && ts[k+1].ent == "" && ts[k+1].r == ' ' {
			return "", false
		}
		if r == ':' && (k == 0 || k == len(ts)-1 || ts[k+1].ent == "" && ts[k+1].r == ':') {
			return "", false
		}
		if r == '_' && (k == 0 || k == len(ts)-1 || !jAlnum(ts[k-1]) || !jAlnum(ts[k+1])) {
			return "", false
		}
	}
	// 마크다운 목록·구분선 꼴의 머리
	if f := ts[0]; f.ent == "" && (f.r == '-' || f.r == '+' || f.r == '=') {
		return "", false
	}
	d := 0
	for d < len(ts) && ts[d].ent == "" && ts[d].r >= '0' && ts[d].r <= '9' {
		d++
	}
	if d >= 1 && d <= 9 && d < len(ts) && ts[d].ent == "" && (ts[d].r == '.' || ts[d].r == ')') &&
		(d+1 == len(ts) || ts[d+1].ent == "" && (ts[d+1].r == ' ' || ts[d+1].r == '\t')) {
		return "", false
	}
	v, ok := jRestore(s)
	if !ok || !jXMLOK(v) || utf8.RuneCountInString(v) > 200 {
		return "", false
	}
	return v, true
}

// jScanID 는 s[i:] 의 이름 꼴(글자·_ 로 시작, 글자·숫자·_)을 읽는다.
func jScanID(s string, i int) (string, int) {
	j := i
	for j < len(s) {
		r, n := utf8.DecodeRuneInString(s[j:])
		if !(r == '_' || unicode.IsLetter(r) || j > i && unicode.IsNumber(r)) {
			break
		}
		j += n
	}
	return s[i:j], j
}

// jKeywords 는 원본 lexer 가 ID 보다 먼저 읽는 규칙의 머리 낱말이다(stateDiagram.jison 의 규칙 차례에서 따로 옮겼다).
var jKeywords = []string{"click", "href", "default", "direction", "scale", "acctitle", "accdescr", "classdef", "class", "style",
	"state", "statediagram", "note", "hide"}

func jIDok(id string) bool {
	if id == "" || utf8.RuneCountInString(id) > 200 {
		return false
	}
	r := []rune(id)
	for k, c := range r {
		if c > 0xffff {
			return false
		}
		if c == '_' && (k == 0 || k == len(r)-1 || !(unicode.IsLetter(r[k-1]) || unicode.IsNumber(r[k-1])) || !(unicode.IsLetter(r[k+1]) || unicode.IsNumber(r[k+1]))) {
			return false
		}
	}
	low := jLower(id)
	if low == "as" || id == "root" || strings.HasSuffix(id, "_start") || strings.HasSuffix(id, "_end") {
		return false
	}
	for _, k := range jKeywords {
		if strings.HasPrefix(low, k) {
			return false
		}
	}
	return true
}

func (m *jModel) reject(line int) bool {
	m.failLine = line
	return false
}

// mention 은 상태를 찾거나 더한다(원본 nodeDb·parentId 규칙을 따로 옮긴 것).
func (m *jModel) mention(id, kind string, line int) bool {
	if _, ok := m.cidx[id]; ok {
		return m.reject(line)
	}
	if k, ok := m.idx[id]; ok {
		st := m.states[k]
		if m.cur >= 0 && st.comp != m.cur {
			if st.comp >= 0 {
				return m.reject(line)
			}
			st.comp = m.cur
			m.comps[m.cur].n++
		}
		return true
	}
	if len(m.states) >= 200 {
		return m.reject(line)
	}
	m.idx[id] = len(m.states)
	m.states = append(m.states, &jState{id: id, kind: kind, comp: m.cur, line: line})
	if m.cur >= 0 {
		m.comps[m.cur].n++
	}
	return true
}

// jEnd 는 전이 끝 하나를 읽는다 — [*] 또는 ID(:::이름). 돌려주는 것: 끝(id 또는 "[*]"), 다음 자리, 맞았는지.
func jEnd(s string, i int) (string, int, bool) {
	if strings.HasPrefix(s[i:], "[*]") {
		return "[*]", i + 3, true
	}
	id, j := jScanID(s, i)
	if id == "" {
		return "", i, false
	}
	if strings.HasPrefix(s[j:], ":::") {
		if cls, k := jScanID(s, j+3); cls != "" {
			if !jIDok(cls) {
				return "", i, false
			}
			return id, k, true
		}
	}
	return id, j, true
}

func (m *jModel) endID(e string, first bool) (string, string) {
	if e != "[*]" {
		return e, "state"
	}
	scope := "root"
	if m.cur >= 0 {
		scope = m.comps[m.cur].id
	}
	if first {
		return scope + "_start", "start"
	}
	return scope + "_end", "end"
}

// jStmt 는 줄 하나(앞뒤 공백을 지우고 엔티티 전처리 뒤)를 읽는다.
func (m *jModel) jStmt(t string, line int) bool {
	low := jLower(t)
	in := m.cur >= 0
	if strings.HasPrefix(t, "%%{") {
		m.sawIgnorable = true
		return true
	}
	if !m.header {
		if t != "stateDiagram" && t != "stateDiagram-v2" {
			return m.reject(line)
		}
		m.header = true
		return true
	}
	if in && t == "}" {
		c := m.comps[m.cur]
		m.cur = -1
		if c.n == 0 {
			return m.reject(c.line)
		}
		return true
	}
	if strings.Contains(low, "direction") {
		i, ok := jWord(low, 0, "direction")
		j := jSkip(low, i)
		if !ok || j == i || in {
			return m.reject(line)
		}
		switch low[j:] {
		case "tb", "bt", "lr", "rl":
			m.dir = strings.ToUpper(low[j:])
			return true
		}
		return m.reject(line)
	}
	if strings.Contains(t, "%%") {
		return m.reject(line)
	}
	// 접근성
	for _, kw := range []string{"acctitle", "accdescr"} {
		i, ok := jWord(low, 0, kw)
		if !ok {
			continue
		}
		j := jSkip(t, i)
		if j < len(t) && t[j] == ':' {
			if in {
				return m.reject(line)
			}
			v := jTrim(t[j+1:])
			if v == "" {
				return m.reject(line)
			}
			d, ok := jAccValue(v)
			if !ok {
				return m.reject(line)
			}
			if kw == "acctitle" {
				m.accT = d
			} else {
				m.accD = d
			}
			return true
		}
		if kw == "accdescr" && j < len(t) && t[j] == '{' {
			if in {
				return m.reject(line)
			}
			m.inAcc, m.accLine, m.accBuf = true, line, nil
			return m.accBody(m.raw[strings.IndexByte(m.raw, '{')+1:], line) // 줄 끝 빈칸까지 값
		}
		return m.reject(line)
	}
	// 쓰지 않는 줄
	if i, ok := jWord(low, 0, "style"); ok && i < len(low) && jIsWS(low[i]) {
		n, j, ok := jASCIIList(low, jSkip(low, i), false)
		if !ok || n > 20 || j == len(low) || !jIsWS(low[j]) || !jStyleText(low[jSkip(low, j):]) {
			return m.reject(line)
		}
		m.sawIgnorable = true
		return true
	}
	if i, ok := jWord(low, 0, "classdef"); ok && i < len(low) && jIsWS(low[i]) {
		k := jSkip(low, i)
		// 원본 CLASSDEF 상태의 첫 규칙 DEFAULT\s+(jison:98)가 이기면 DEFAULT_CLASSDEF_ID — 문법에 받는 생산식이 없어 오류다.
		if e, ok := jWord(low, k, "default"); ok && e < len(low) && jIsWS(low[e]) {
			return m.reject(line)
		}
		n, j, ok := jASCIIList(low, k, false)
		if !ok || n != 1 || j == len(low) || !jIsWS(low[j]) || !jStyleText(low[jSkip(low, j):]) {
			return m.reject(line)
		}
		m.sawIgnorable = true
		return true
	}
	if i, ok := jWord(low, 0, "class"); ok && i < len(low) && jIsWS(low[i]) {
		n, j, ok := jASCIIList(low, jSkip(low, i), true)
		if !ok || n > 20 || j == len(low) || !jIsWS(low[j]) {
			return m.reject(line)
		}
		k, j2, ok := jASCIIList(low, jSkip(low, j), false)
		if !ok || k != 1 || j2 != len(low) {
			return m.reject(line)
		}
		m.sawIgnorable = true
		return true
	}
	if i, ok := jWord(low, 0, "click"); ok && i < len(low) && jIsWS(low[i]) {
		if in {
			return m.reject(line)
		}
		id, j := jScanID(t, jSkip(t, i))
		if id == "" || !jIDok(id) || j == len(t) || !jIsWS(t[j]) {
			return m.reject(line)
		}
		j = jSkip(t, j)
		if k, ok := jWord(low, j, "href"); ok && k < len(t) && jIsWS(t[k]) {
			if e, ok := jQuoted(t, jSkip(t, k)); !ok || e != len(t) {
				return m.reject(line)
			}
		} else {
			e, ok := jQuoted(t, j)
			if !ok || e == len(t) || !jIsWS(t[e]) {
				return m.reject(line)
			}
			if e2, ok := jQuoted(t, jSkip(t, e)); !ok || e2 != len(t) {
				return m.reject(line)
			}
		}
		m.sawIgnorable = true
		return true
	}
	if low == "hide empty description" {
		if in {
			return m.reject(line)
		}
		m.sawIgnorable = true
		return true
	}
	// state …
	if i, ok := jWord(low, 0, "state"); ok && i < len(t) && jIsWS(t[i]) {
		return m.jState(t, low, jSkip(t, i), line)
	}
	// 전이
	if a, i, ok := jEnd(t, 0); ok {
		j := jSkip(t, i)
		if strings.HasPrefix(t[j:], "-->") {
			b, k, ok := jEnd(t, jSkip(t, j+3))
			if !ok {
				return m.reject(line)
			}
			rest := t[k:]
			lbl := ""
			if r := jTrim(rest); r != "" {
				r2 := t[jSkip(t, k):]
				if r2[0] != ':' {
					return m.reject(line)
				}
				v, ok := jLabel(r2[1:])
				if !ok {
					return m.reject(line)
				}
				lbl = v
			}
			if len(m.trans) >= 500 {
				return m.reject(line)
			}
			if (a != "[*]" && !jIDok(a)) || (b != "[*]" && !jIDok(b)) {
				return m.reject(line)
			}
			ia, ka := m.endID(a, true)
			ib, kb := m.endID(b, false)
			if !m.mention(ia, ka, line) || !m.mention(ib, kb, line) {
				return false
			}
			if strings.Contains(t[:k], ":::") {
				m.sawIgnorable = true
			}
			m.trans = append(m.trans, jTrans{ia, ib, lbl})
			return true
		}
	}
	// 상태 선언
	id, i := jScanID(t, 0)
	if id == "" {
		return m.reject(line)
	}
	cls := false
	if strings.HasPrefix(t[i:], ":::") {
		if c, k := jScanID(t, i+3); c != "" && (k == len(t) || jSkip(t, k) < len(t) && t[jSkip(t, k)] == ':') {
			if !jIDok(c) {
				return m.reject(line)
			}
			cls, i = true, k
		}
	}
	hasD, d := false, ""
	if i < len(t) {
		j := jSkip(t, i)
		if j == len(t) || t[j] != ':' {
			return m.reject(line)
		}
		v, ok := jLabel(t[j+1:])
		if !ok {
			return m.reject(line)
		}
		hasD, d = true, v
	}
	return m.declare(id, hasD, d, cls, line)
}

func (m *jModel) declare(id string, hasD bool, d string, cls bool, line int) bool {
	if !jIDok(id) {
		return m.reject(line)
	}
	if k, ok := m.idx[id]; ok && hasD && m.states[k].kind != "state" {
		return m.reject(line)
	}
	if !m.mention(id, "state", line) {
		return false
	}
	if cls {
		m.sawIgnorable = true
	}
	if hasD {
		st := m.states[m.idx[id]]
		if len(st.descr) >= 20 {
			return m.reject(line)
		}
		st.descr = append(st.descr, d)
	}
	return true
}

// jState 는 state 낱말 뒤(자리 i)를 읽는다 — 특수 상태·state "설명" as ID(복합 포함)·state ID {.
func (m *jModel) jState(t, low string, i, line int) bool {
	if t[i] == '"' {
		e := strings.IndexByte(t[i+1:], '"')
		if e < 0 {
			return m.reject(line)
		}
		descr := t[i+1 : i+1+e]
		j := i + 2 + e
		k := jSkip(t, j)
		if k == j {
			return m.reject(line)
		}
		k2, ok := jWord(low, k, "as")
		if !ok || k2 == len(t) || !jIsWS(t[k2]) {
			return m.reject(line)
		}
		id, k3 := jScanID(t, jSkip(t, k2))
		if id == "" {
			return m.reject(line)
		}
		rest := jTrim(t[k3:])
		switch rest {
		case "":
			v, ok := jLabel(descr)
			if !ok || !jIDok(id) {
				return m.reject(line)
			}
			return m.declare(id, true, v, false, line)
		case "{":
			return m.open(id, descr, true, line)
		}
		return m.reject(line)
	}
	id, j := jScanID(t, i)
	if id == "" {
		return m.reject(line)
	}
	k := jSkip(t, j)
	rest := low[k:]
	for _, sp := range []string{"choice", "fork", "join"} {
		if rest == "<<"+sp+">>" {
			if !jIDok(id) {
				return m.reject(line)
			}
			if _, ok := m.idx[id]; ok {
				return m.reject(line)
			}
			return m.mention(id, sp, line)
		}
	}
	if rest == "{" {
		return m.open(id, "", false, line)
	}
	return m.reject(line)
}

func (m *jModel) open(id, descr string, hasD bool, line int) bool {
	if !jIDok(id) {
		return m.reject(line)
	}
	title := id
	if hasD {
		if descr != jTrim(descr) {
			return m.reject(line)
		}
		v, ok := jLabel(descr)
		if !ok {
			return m.reject(line)
		}
		title = v
	}
	if m.cur >= 0 {
		return m.reject(line)
	}
	if _, ok := m.cidx[id]; ok {
		return m.reject(line)
	}
	if k, ok := m.idx[id]; ok {
		return m.reject(m.states[k].line)
	}
	if len(m.comps) >= 50 {
		return m.reject(line)
	}
	m.cidx[id] = len(m.comps)
	m.comps = append(m.comps, &jComp{id: id, title: title, line: line})
	m.cur = len(m.comps) - 1
	return true
}

// jASCIIList 는 ASCII 낱말([a-z0-9_]+) 목록(쉼표, spaced 면 쉼표 뒤 빈칸 허용)을 읽는다.
func jASCIIList(s string, i int, spaced bool) (int, int, bool) {
	n := 0
	for {
		j := i
		for j < len(s) && (s[j] == '_' || '0' <= s[j] && s[j] <= '9' || 'a' <= s[j] && s[j] <= 'z') {
			j++
		}
		if j == i {
			return 0, i, false
		}
		n++
		if j < len(s) && s[j] == ',' {
			i = j + 1
			if spaced {
				i = jSkip(s, i)
			}
			continue
		}
		return n, j, true
	}
}

// jStyleText 는 쓰지 않는 style·classDef 의 값이 영문·숫자·_·:·#·.·- 낱말을 빈칸이나 쉼표로 이은 것인지 본다.
func jStyleText(s string) bool {
	if s == "" {
		return false
	}
	prevTok := false
	comma := false
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '_' || c == ':' || c == '#' || c == '.' || c == '-' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z':
			prevTok, comma = true, false
			i++
		case jIsWS(c) || c == ',':
			if !prevTok && !comma {
				return false
			}
			sawComma := false
			for i < len(s) && (jIsWS(s[i]) || s[i] == ',') {
				if s[i] == ',' {
					if sawComma {
						return false
					}
					sawComma = true
				}
				i++
			}
			prevTok, comma = false, true
		default:
			return false
		}
	}
	return prevTok
}

// jQuoted 는 s[i:] 의 "…"(안에 " 없음)을 읽고 끝 자리를 돌려준다.
func jQuoted(s string, i int) (int, bool) {
	if i >= len(s) || s[i] != '"' {
		return i, false
	}
	e := strings.IndexByte(s[i+1:], '"')
	if e < 0 {
		return i, false
	}
	return i + e + 2, true
}

// accBody 는 accDescr { … } 의 줄 하나(앞 빈칸을 지운 것)를 모은다. 원본은 블록 글 전체를 trim 하고(jison:270)
// setAccDescription 이 replace(/\n\s+/g, '\n') 한다(commonDb.ts) — 줄 머리 빈칸을 이미 지웠으니 빈 줄이 빠지고, 줄바꿈·줄 끝
// 빈칸·안쪽 빈칸과 탭은 그대로다(마지막 줄 끝만 trim).
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

// jYAMLTitle 은 생성기가 내는 title 값 꼴만 손으로 푼다 — 큰따옴표 글이면 \uXXXX·\t·\"·\\ 이스케이프를 풀고, 아니면 글 그대로
// (YAML 일반 스칼라). 그 밖의 YAML 꼴은 생성하지 않는다.
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
		default: // \" · \\
			b.WriteByte(in[i+1])
			i++
		}
	}
	return b.String()
}

// jJudge 는 원문 하나를 판정한다 — 받아들이면 모델과 -1, 아니면 nil 과 거부 줄(0 = 줄 없음).
func jJudge(src string) (*jModel, int) {
	m := &jModel{idx: map[string]int{}, cidx: map[string]int{}, cur: -1, failLine: -1, dir: "TB"}
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
				// 정책 B 캡션 — 생성기가 내는 꼴만 손으로 본다: 안쪽 연속 빈칸·탭은 그 줄에서 바로 원문(본문보다 먼저),
				// 엔티티 #…; 는 글자 그대로(원본은 frontmatter 를 엔티티 전처리 전에 떼어 낸다).
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
			if !whole || m.inAcc || m.cur >= 0 {
				return nil, ln
			}
		}
		if strings.HasPrefix(t, "%%") && !strings.HasPrefix(t, "%%{") {
			if m.inAcc && strings.TrimLeft(raw, " \t") == "%%" {
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
	case m.cur >= 0:
		return nil, m.comps[m.cur].line
	case !m.header, len(m.states) == 0:
		return nil, 0
	case !jXMLOK(m.caption): // 출력 관문 — 본문을 다 읽은 뒤, 캡션은 title 줄
		return nil, m.captionLine
	}
	return m, -1
}

// ---- 생성기: 경계 토큰을 명시적으로 조합한다

func pick(rng *rand.Rand, xs []string) string { return xs[rng.Intn(len(xs))] }

// gTrapOn 은 이번 원문에 함정을 섞는지다 — 판의 반은 함정 없이 만들어 받는 판의 몫을 지킨다.
var gTrapOn bool

// jTrap 은 함정 토큰을 고를지(함정 판에서 약 8분의 1)다.
func jTrap(rng *rand.Rand) bool { return gTrapOn && rng.Intn(8) == 0 }

// pickN 은 함정 판이면 xs 전체에서, 아니면 앞의 n 개(받는 꼴)에서 고른다.
func pickN(rng *rand.Rand, xs []string, n int) string {
	if gTrapOn {
		return pick(rng, xs)
	}
	return pick(rng, xs[:n])
}

var (
	gSafeIDs = []string{"접수", "결제", "배송", "출고", "완료", "취소", "A", "B", "C", "가", "나", "Pay1", "x_y", "상태2", "ſtate", "Kick",
		"Đơn", "검토", "보류"}
	gTrapIDs = []string{"click", "Clicked", "click가", "HREF", "hrefs", "Default", "defaults", "Direction", "scale", "Scale1", "accTitleX",
		"accdescr", "classA", "Classroom", "style1", "State", "stateful", "Note", "note2", "hide", "Hidex", "as", "AS", "root", "Root",
		"root_start", "x_start", "a_end", "_a", "a_", "a__b", "𝔸", "a𝔸", "a-b", "a#b", "a%b", "a%%b", "1a", "a;b", "a:b", "a\"b", "end",
		strings.Repeat("가", 200), strings.Repeat("나", 201), "A" + strings.Repeat("b", 199), "A" + strings.Repeat("b", 200)}
	// 라벨 — 받는 꼴. 숫자 경계(33·126·161·172·174·8208·8240·8293 …)는 받는 쪽 이웃이다.
	gSafeLabels = []string{"#169;", "#0000065;", "#33;", "#126;", "#161;", "#172;", "#174;", "#8208;", "#8240;", "#8293;", "#12289;",
		"#55295;", "#57344;", "#64975;", "#65008;", "#65533;", "#65536;", "#131069;", "#917632;", "#1114109;", "#8364;", "#lt;br#gt;",
		"다음", "확인 완료", "a : b", "결제 #35;3", "x#59;y", "a_b", "1단계", "Pay #quot;ok#quot;", "비율 50", "x", "#lt;태그", "a#amp;b",
		"#apos;#num;#semi;", "끝.", "x\ufffdy", "\ufdd0"}
	// 라벨 — 거부 꼴. 공백·제어·서식·비문자·서러게이트·범위 밖 숫자와 그 경계, 미지원 이름·대소문자 변형, 토큰 경계, 연속 빈칸·탭.
	gTrapLabels = []string{"#copy;", "a #euro; b", "#hellip;", "#zzz;", "#AMP;", "#Quot;", "#LT;", "#nbsp;사이", "#x41;", "#12345678;",
		"#00000065;", "#0032;", "#32;", "#31;", "#9;", "#127;", "#133;", "#159;", "#160;", "x#160;#160;y", "x#8195;y", "#173;", "#5760;",
		"#6158;", "#8202;", "#8203;", "#8207;", "#8232;", "#8233;", "#8234;", "#8239;", "#8287;", "#8288;", "#8292;", "#8294;", "#8303;",
		"#12288;", "#65279;", "#55296;", "#57343;", "#64976;", "#65007;", "#65534;", "#65535;", "#131070;", "#917505;", "#917536;",
		"#917631;", "#1114111;", "#1114112;", "x\ufffey", "\uffff", "#+65;", "#-65;", "#65", "# 65;", "#35;35;", "두  칸", "x\ty", "a \tb",
		"#", "#x", ";", "%", "%%", ":", "a:", ":a", "a::b", "\"", "`", "*강조*", "_x_", "x_", "- 목록", "+ 하나", "= 선",
		"1. 첫째", "2) 둘째", "<br>", "a<b", "direction tb", "style x:#a;", "style x:#a; y", "[x]", "{", "}", "~", "$", "&", "", " ", "𝔸",
		"a\u00a0b", "a\ufeffb", "ﬂ°", "#_a;", "a %%{init: {}}%% b", strings.Repeat("가", 201), strings.Repeat("나", 200)}
	// 접근성 값 — 앞 12개는 받는 꼴(표식 밖 #…; 는 글자 그대로, 안쪽 빈칸·탭 보존, 재귀 없음), 나머지는 거부 꼴.
	gAccVals = []string{"#+65;", "#35;35;", "#-65;", "#65", "# 65;", "x  y", "x\ty", "a\t\tb  c", "#lt;br#gt;", "#amp; #169;", "제목 ; # 끝",
		"#0000065;", "<br>", "x\ufffey", "\uffff끝", "a & b", "a > b", "#copy;", "#160;", "#32;", "#9;", "#8203;", "#nbsp;", "\x01", strings.Repeat("가", 201)}
	gSeps = []string{" ", " ", " ", "", "\t", "  ", " \t"}
)

func gID(rng *rand.Rand) string {
	if jTrap(rng) {
		return pick(rng, gTrapIDs)
	}
	return pick(rng, gSafeIDs)
}

func gLabel(rng *rand.Rand) string {
	if jTrap(rng) {
		return pick(rng, gTrapLabels)
	}
	return pick(rng, gSafeLabels)
}

func gSep(rng *rand.Rand) string { return pick(rng, gSeps) }

func gEnd(rng *rand.Rand) string {
	switch rng.Intn(10) {
	case 0, 1:
		return "[*]"
	case 2:
		if jTrap(rng) {
			return "[*]:::x"
		}
		return gID(rng) + ":::" + pick(rng, []string{"강조", "bad", "Default", "a-b"})
	}
	return gID(rng)
}

func gColon(rng *rand.Rand) string {
	if jTrap(rng) {
		return pick(rng, []string{"::", ":::", " : :", ";"})
	}
	return gSep(rng) + ":" + gSep(rng)
}

// gStmt 는 문장 한 줄이다(in 은 복합 상태 안).
func gStmt(rng *rand.Rand, in bool) string {
	switch rng.Intn(30) {
	case 0, 1, 2, 3:
		return gID(rng)
	case 4, 5, 6:
		return gID(rng) + gColon(rng) + gLabel(rng)
	case 7:
		return "state" + gSep(rng) + "\"" + gLabel(rng) + "\"" + gSep(rng) + pick(rng, []string{"as", "AS", "As"}) + gSep(rng) + gID(rng)
	case 8:
		return "state " + gID(rng) + gSep(rng) + pickN(rng, []string{"<<choice>>", "<<fork>>", "<<join>>", "<<FORK>>", "[[fork]]", "<<fork>> x", "<<choise>>"}, 4)
	case 9, 10, 11, 12, 13, 14, 15:
		s := gEnd(rng) + gSep(rng) + pickN(rng, []string{"-->", "-->", "-->", "-->", "->", "--"}, 4) + gSep(rng) + gEnd(rng)
		if rng.Intn(3) == 0 {
			s += gColon(rng) + gLabel(rng)
		}
		return s
	case 16:
		if in && !jTrap(rng) {
			return gID(rng)
		}
		return pickN(rng, []string{"direction LR", "direction TB", "direction BT", "direction rl", "DIRECTION Lr", "direction  LR", "direction\tTB",
			"direction XY", "direction LR x", "a direction LR", "direction"}, 7)
	case 17:
		if in && !jTrap(rng) {
			return gID(rng)
		}
		v := pickN(rng, gAccVals, 12)
		return pickN(rng, []string{"accTitle: 제목 " + v, "accTitle:" + v + " \t", "ACCTITLE : 큰 제목", "accDescr: 설명 ; # 포함", "accDescr:\t" + v,
			"accDescr { " + v + " }", "accDescr {\n  " + v + "  \n\n\t둘째\t칸  \n  %% 메모\n 끝 }", "accDescr { 앞  \n뒤\t}",
			"accDescr {\n" + v + "\n}", "accTitle: 제목 " + gLabel(rng), "accTitle:", "accDescr {", "accDescr { 앞", "accDescr{"}, 9)
	case 18:
		return pickN(rng, []string{"style A fill:#f9f,stroke:#333", "style A,B fill:#f00;", "STYLE a fill:#f00", "classDef bad fill:#f00,color:white",
			"classDef bad fill:#f00;", "classdef bad fill:#f00;", "class A,B bad", "class A, B bad", "style 가 fill:#f00", "classDef 강조 fill:#f00",
			"class A bad x", "class 가 bad", "classDef default fill:#fff", "classDef DEFAULT x", "classDef Default\tx", "classDef defaults fill:#fff"}, 8)
	case 19:
		if in && !jTrap(rng) {
			return gID(rng)
		}
		return pickN(rng, []string{"click A \"https://example.com\" \"설명\"", "click A href \"https://example.com/#a\"", "hide empty description",
			"HIDE EMPTY DESCRIPTION", "click A \"u\"", "click Default href \"u\"", "hide  empty description"}, 4)
	case 20, 21:
		return pickN(rng, []string{"%% 메모", "%%", "%%  ", "  %% 들여쓴 메모", "%%{init: {}}%%", "%%{init: {", "A %%{init: {}}%%", "%%{a}%% %%{b}%%"}, 5)
	case 22:
		if !gTrapOn {
			return gID(rng)
		}
		return pick(rng, []string{"note right of A : 메모", "note \"떠 있음\" as N", "end note", "--", "state X", "scale 350 width", "}", "A {",
			"[*]", "A --> B --> C", "\"따옴표\" --> A", "A # 주석", "A; B", "A : x : y", "[*] : 설명", "state A B {", "{"})
	case 23:
		return ""
	}
	return gID(rng) + gSep(rng) + "-->" + gSep(rng) + gID(rng)
}

func jDoc(rng *rand.Rand) string {
	var b strings.Builder
	gTrapOn = rng.Intn(2) == 0
	if rng.Intn(6) == 0 {
		titles := []string{"주문 상태", "흐름", "t", `"x\uFFFDy"`, `"따옴표 \"제목\""`, `"x\uFDD0y"`, `"x\uFFFEy"`, `"x\uFFFFy"`,
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
	head := "stateDiagram-v2"
	switch rng.Intn(12) {
	case 0:
		head = "stateDiagram"
	case 1:
		if !gTrapOn {
			break
		}
		head = pick(rng, []string{"stateDiagram-V2", "statediagram", "  stateDiagram-v2  ", "stateDiagram-v2 direction LR", "stateDiagramX", "StateDiagram"})
	}
	b.WriteString(head + "\n")
	n := 1 + rng.Intn(9)
	for k := 0; k < n; k++ {
		if rng.Intn(6) == 0 {
			// 복합 상태
			open := "state " + gID(rng) + gSep(rng) + "{"
			if rng.Intn(3) == 0 {
				open = "state \"" + gLabel(rng) + "\" as " + gID(rng) + gSep(rng) + "{"
			}
			b.WriteString(open + "\n")
			inner := rng.Intn(5)
			if gTrapOn && rng.Intn(8) == 0 {
				inner = 0
			} else if inner == 0 {
				inner = 1
			}
			for j := 0; j < inner; j++ {
				if gTrapOn && rng.Intn(15) == 0 {
					b.WriteString("state " + gID(rng) + " {\n")
				}
				b.WriteString("  " + gStmt(rng, true) + "\n")
			}
			if !gTrapOn || rng.Intn(12) != 0 {
				b.WriteString(pickN(rng, []string{"}", "}", "}", "  }  ", "} x"}, 4) + "\n")
			}
			continue
		}
		line := gStmt(rng, false)
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
	const rounds = 20000
	acc, rej := 0, 0
	// seen 은 받은 판에 나온 정책 B 경계의 수다 — 생성기가 그 꼴을 받는 판에서 실제로 다루는지 본다.
	seen := map[string]int{}
	for k := 0; k < rounds; k++ {
		src := jDoc(rng)
		jm, jl := jJudge(src)
		c, issues, err := Parse(src)
		if jm == nil {
			rej++
			if jl == 2 && (strings.Contains(src, `\uFFFE`) || strings.Contains(src, `\uFFFF`)) {
				seen["캡션 관문 거부"]++
			}
			if jl == 2 && (strings.Contains(src, `"x  y"`) || strings.Contains(src, `"x\ty"`)) {
				seen["캡션 빈칸 거부"]++
			}
			var de *diagram.Error
			if err == nil {
				t.Fatalf("판 %d: 판정기는 %d 줄에서 거부, Parse 는 받음\n%s\n얻음 %s", k, jl, src, summary(c))
			}
			if !errors.As(err, &de) || de.Line != jl {
				t.Fatalf("판 %d: 거부 줄 판정기 %d, Parse %v\n%s", k, jl, err, src)
			}
			continue
		}
		acc++
		if err != nil {
			t.Fatalf("판 %d: 판정기는 받음, Parse 는 거부 %v\n%s\n판정기 %s", k, err, src, jSummary(jm))
		}
		if got, want := summary(c), jSummary(jm); got != want {
			t.Fatalf("판 %d: 값이 다름\n%s\nParse  %s\n판정기 %s", k, src, got, want)
		}
		if c.FMCaption != jm.caption {
			t.Fatalf("판 %d: 캡션 %q, 판정기 %q\n%s", k, c.FMCaption, jm.caption, src)
		}
		if (len(issues) > 0) != jm.sawIgnorable {
			t.Fatalf("판 %d: 경고 %v, 판정기 무시 줄 %v\n%s", k, issues, jm.sawIgnorable, src)
		}
		a := jm.accT + "/" + jm.accD
		for name, hit := range map[string]bool{
			"acc #+65;": strings.Contains(a, "#+65;"), "acc #35; 한 번": strings.Contains(a, "#35;"), "acc 탭": strings.Contains(a, "\t"),
			"acc 연속 빈칸": strings.Contains(a, "  "), "accDescr 줄바꿈": strings.Contains(jm.accD, "\n"),
			"라벨 엔티티":    strings.ContainsAny(jSummary(jm), "©<\u2065\U0010fffd"),
			"캡션 이스케이프":  strings.ContainsAny(jm.caption, "\ufffd\ufdd0\""),
			"캡션 엔티티 글자": strings.Contains(jm.caption, "#35;") || strings.Contains(jm.caption, "#lt;"),
		} {
			if hit {
				seen[name]++
			}
		}
	}
	for _, name := range []string{"acc #+65;", "acc #35; 한 번", "acc 탭", "acc 연속 빈칸", "accDescr 줄바꿈", "라벨 엔티티", "캡션 관문 거부", "캡션 빈칸 거부", "캡션 이스케이프", "캡션 엔티티 글자"} {
		if seen[name] < 5 {
			t.Errorf("받은 판에 %s 가 %d 번 — 생성기가 이 경계를 고루 내지 못한다", name, seen[name])
		}
	}
	t.Logf("경계 %v", seen)
	t.Logf("판 %d — 받음 %d, 거부 %d", rounds, acc, rej)
	if acc < rounds/5 || rej < rounds/5 {
		t.Errorf("받음 %d·거부 %d — 둘 다 판수의 1/5 이상이어야 경계를 고루 본다", acc, rej)
	}
}

// jSummary 는 판정기 모델을 summary 와 같은 꼴로 적는다.
func jSummary(m *jModel) string {
	var b strings.Builder
	fmt.Fprintf(&b, "dir=%d", map[string]int{"TB": 0, "BT": 1, "LR": 2, "RL": 3}[m.dir])
	for _, s := range m.states {
		b.WriteString(" " + s.id)
		if s.kind != "state" {
			b.WriteString("(" + s.kind + ")")
		}
		if len(s.descr) > 0 {
			b.WriteString("[" + strings.Join(s.descr, "/") + "]")
		}
		if s.comp >= 0 {
			b.WriteString("@" + m.comps[s.comp].id)
		}
	}
	b.WriteString(" |")
	for _, tr := range m.trans {
		b.WriteString(" " + tr.a + ">" + tr.b)
		if tr.label != "" {
			b.WriteString(`"` + tr.label + `"`)
		}
	}
	b.WriteString(" |")
	for _, cp := range m.comps {
		b.WriteString(" " + cp.id + `"` + cp.title + `"`)
	}
	if m.accT != "" || m.accD != "" {
		b.WriteString(" | acc=" + m.accT + "/" + m.accD)
	}
	return b.String()
}

// ---- 정책 B 고정 표(접근 방식 리뷰 4장 — 사람이 적은 기대 코드 포인트. 구현의 함수나 unicode 표로 만들지 않는다)

// jEntityTable 은 일곱 문맥 모두에서 같은 기대를 갖는 엔티티 조각이다(want 가 nil 이면 그 줄은 원문 + 경고).
var jEntityTable = []struct {
	frag string
	want []rune
}{
	{"#35;", []rune{0x23}}, {"#59;", []rune{0x3b}}, {"#quot;", []rune{0x22}},
	{"#amp;", []rune{0x26}}, {"#lt;", []rune{0x3c}}, {"#gt;", []rune{0x3e}}, {"#apos;", []rune{0x27}},
	{"#num;", []rune{0x23}}, {"#semi;", []rune{0x3b}}, {"#lt;br#gt;", []rune{0x3c, 0x62, 0x72, 0x3e}},
	{"#169;", []rune{0xa9}}, {"#8364;", []rune{0x20ac}}, {"#0000065;", []rune{0x41}},
	// 미지원 이름·대소문자 변형·이름 표식인 #x41;·8자리
	{"#copy;", nil}, {"#euro;", nil}, {"#hellip;", nil}, {"#zzz;", nil}, {"#nbsp;", nil}, {"#AMP;", nil}, {"#Quot;", nil},
	{"#LT;", nil}, {"#x41;", nil}, {"#00000065;", nil},
	// 공백·제어·서식 — 위치와 무관하게
	{"#160;", nil}, {"#160;#160;", nil}, {"#8195;", nil}, {"#32;", nil}, {"#0032;", nil}, {"#9;", nil}, {"#10;", nil},
	{"#8203;", nil}, {"#65279;", nil},
	// 숫자 경계 — 31/32/33, 126/127/159/160/161
	{"#31;", nil}, {"#33;", []rune{0x21}}, {"#126;", []rune{0x7e}}, {"#127;", nil}, {"#133;", nil}, {"#159;", nil},
	{"#161;", []rune{0xa1}},
	// 서러게이트 양끝과 이웃
	{"#55295;", []rune{0xd7ff}}, {"#55296;", nil}, {"#57343;", nil}, {"#57344;", []rune{0xe000}},
	// U+10FFFF(비문자)와 그 밖, 비문자 구간
	{"#1114109;", []rune{0x10fffd}}, {"#1114110;", nil}, {"#1114111;", nil}, {"#1114112;", nil},
	{"#64975;", []rune{0xfdcf}}, {"#64976;", nil}, {"#65007;", nil}, {"#65008;", []rune{0xfdf0}},
	{"#65533;", []rune{0xfffd}}, {"#65534;", nil}, {"#65535;", nil}, {"#65536;", []rune{0x10000}},
	{"#131069;", []rune{0x1fffd}}, {"#131070;", nil},
	// Cf·공백 구간의 양끝과 이웃(U+2065·U+E0000·U+E001F 는 이 판본에서 Cf 가 아니다 — 받는다)
	{"#172;", []rune{0xac}}, {"#173;", nil}, {"#174;", []rune{0xae}},
	{"#1535;", []rune{0x5ff}}, {"#1536;", nil}, {"#1541;", nil}, {"#1542;", []rune{0x606}},
	{"#6157;", []rune{0x180d}}, {"#6158;", nil}, {"#6159;", []rune{0x180f}},
	{"#5759;", []rune{0x167f}}, {"#5760;", nil}, {"#5761;", []rune{0x1681}},
	{"#8191;", []rune{0x1fff}}, {"#8192;", nil}, {"#8202;", nil}, {"#8207;", nil}, {"#8208;", []rune{0x2010}},
	{"#8231;", []rune{0x2027}}, {"#8232;", nil}, {"#8233;", nil}, {"#8234;", nil}, {"#8238;", nil}, {"#8239;", nil},
	{"#8240;", []rune{0x2030}},
	{"#8286;", []rune{0x205e}}, {"#8287;", nil}, {"#8288;", nil}, {"#8292;", nil}, {"#8293;", []rune{0x2065}}, {"#8294;", nil},
	{"#8303;", nil}, {"#8304;", []rune{0x2070}},
	{"#12287;", []rune{0x2fff}}, {"#12288;", nil}, {"#12289;", []rune{0x3001}},
	{"#65278;", []rune{0xfefe}}, {"#65280;", []rune{0xff00}},
	{"#65528;", []rune{0xfff8}}, {"#65529;", nil}, {"#65531;", nil}, {"#65532;", []rune{0xfffc}},
	{"#69820;", []rune{0x110bc}}, {"#69821;", nil}, {"#69822;", []rune{0x110be}},
	{"#917504;", []rune{0xe0000}}, {"#917505;", nil}, {"#917535;", []rune{0xe001f}}, {"#917536;", nil}, {"#917631;", nil},
	{"#917632;", []rune{0xe0080}},
}

// jRawTable 은 라벨과 접근성 값에서 기대가 갈리는 원문 조각이다 — 원본 전처리의 토큰 범위(#\w+;) 밖은 엔티티가 아니고,
// 라벨은 맨 #·;·&·< 와 연속 빈칸·탭을 받지 않는다. acc 에서 받는 조각이 라벨에서도 받힌다고 추론하지 않는다.
var jRawTable = []struct {
	frag         string
	lbl, acc     string
	lblOK, accOK bool
}{
	{"#+65;", "", "#+65;", false, true},
	{"#-65;", "", "#-65;", false, true},
	{"#65", "", "#65", false, true},
	{"# 65;", "", "# 65;", false, true},
	{"#35;35;", "", "#35;", false, true}, // 한 번만 — 되돌린 #35; 를 다시 풀지 않는다
	{"  ", "", "  ", false, true},
	{"\t", "", "\t", false, true},
	{" \t ", "", " \t ", false, true},
	{" ", " ", " ", true, true},
	{"<br>", "", "", false, false},
	{"&", "", "", false, false},
	{"#65;", "A", "A", true, true},
	// 원문에 실제로 든 글자 — XML 1.0 Char 밖(U+FFFE·U+FFFF)은 SVG 가 바꾸므로 거부, 경계 이웃과 XML 이 담는 비문자·서식 글자는
	// 받고 SVG 에도 그대로(엔티티 정책과 따로 — 원문 글자는 OddSpace 정책만 본다)
	{"\ufffe", "", "", false, false},
	{"\uffff", "", "", false, false},
	{"a\uffffb\ufffe", "", "", false, false},
	{"\ufffd", "\ufffd", "\ufffd", true, true},
	{"\ufffc", "\ufffc", "\ufffc", true, true},
	{"\ud7ff", "\ud7ff", "\ud7ff", true, true},
	{"\ue000", "\ue000", "\ue000", true, true},
	{"\ufdd0", "\ufdd0", "\ufdd0", true, true},
	{"\U0001fffe", "\U0001fffe", "\U0001fffe", true, true},
	{"\U0010ffff", "\U0010ffff", "\U0010ffff", true, true},
	{"\u00ad", "\u00ad", "\u00ad", true, true},
}

type jCtx struct {
	name string
	src  func(v string) string
	line int  // 거부 줄
	acc  bool // 접근성 문맥
}

var jCtxs = []jCtx{
	{"상태 설명", func(v string) string { return "stateDiagram-v2\nA : " + v }, 2, false},
	{"전이 라벨", func(v string) string { return "stateDiagram-v2\nA --> B : " + v }, 2, false},
	{"따옴표 설명", func(v string) string { return "stateDiagram-v2\nstate \"" + v + "\" as A" }, 2, false},
	{"복합 상태 제목", func(v string) string { return "stateDiagram-v2\nstate \"" + v + "\" as M {\nA\n}" }, 2, false},
	{"accTitle", func(v string) string { return "stateDiagram-v2\naccTitle: " + v + "\nA" }, 2, true},
	{"accDescr 한 줄", func(v string) string { return "stateDiagram-v2\naccDescr: " + v + "\nA" }, 2, true},
	{"accDescr 블록", func(v string) string { return "stateDiagram-v2\naccDescr {\n  " + v + "  \n}\nA" }, 4, true},
}

// jValue 는 문맥의 값(Parse 결과)이다.
func jValue(c *chart, ctx string) string {
	switch ctx {
	case "상태 설명", "따옴표 설명":
		return c.States[0].Descr[0]
	case "전이 라벨":
		return c.Trans[0].Label
	case "복합 상태 제목":
		return c.Comps[0].Title
	case "accTitle":
		return c.AccTitle
	}
	return c.AccDescr
}

// jSVGOut 은 최종 SVG 를 실제 XML 파서(encoding/xml)로 읽은 결과다 — 뿌리 svg 의 aria-label 값, <title> 글, <text> 마다의
// 줄 목록(tspan 이 있으면 tspan 마다 한 줄, 없으면 글 하나). 줄을 빈칸으로 잇지 않는다(리뷰 5회차 [1]).
type jSVGOut struct {
	aria  string
	title []string
	texts [][]string
}

// jParseSVG 는 SVG 를 XML 로 읽는다. 읽히지 않으면 시험 실패다.
func jParseSVG(t *testing.T, svg string) jSVGOut {
	t.Helper()
	var out jSVGOut
	d := xml.NewDecoder(strings.NewReader(svg))
	depth := 0
	var lines []string // 지금 <text> 의 tspan 줄
	var buf strings.Builder
	for {
		start := d.InputOffset()
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("SVG 가 XML 로 읽히지 않음: %v\n%s", err, svg)
		}
		switch e := tok.(type) {
		case xml.StartElement:
			depth++
			buf.Reset()
			if depth == 1 && e.Name.Local == "svg" {
				for _, at := range jAttrs(t, svg[start:d.InputOffset()]) {
					if at.Name.Local == "aria-label" {
						out.aria = at.Value
					}
				}
			}
			if e.Name.Local == "text" {
				lines = nil
			}
		case xml.CharData:
			buf.Write(e)
		case xml.EndElement:
			depth--
			switch e.Name.Local {
			case "title":
				out.title = append(out.title, buf.String())
			case "tspan":
				lines = append(lines, buf.String())
			case "text":
				if lines == nil {
					lines = []string{buf.String()}
				}
				out.texts = append(out.texts, lines)
			}
			buf.Reset()
		}
	}
	return out
}

// jAttrs 는 시작 태그 원문 하나의 속성을 XML 속성값 정규화(XML 1.0 3.3.3 — 줄끝을 LF 로 모은 뒤 리터럴 탭·LF 를 빈칸으로,
// 문자 참조는 그대로)를 거쳐 돌려준다. encoding/xml 은 이 정규화를 하지 않아(리터럴 탭이 탭으로 남는다) 손으로 먼저 한다 —
// 그래야 XML 프로세서가 실제로 보는 값과 같다.
func jAttrs(t *testing.T, tag string) []xml.Attr {
	t.Helper()
	tag = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ", "\t", " ").Replace(tag)
	if !strings.HasSuffix(tag, "/>") {
		tag = strings.TrimSuffix(tag, ">") + "/>"
	}
	tok, err := xml.NewDecoder(strings.NewReader(tag)).Token()
	if err != nil {
		t.Fatalf("시작 태그를 읽지 못함: %v\n%s", err, tag)
	}
	return tok.(xml.StartElement).Attr
}

// jWrapOK 는 줄 목록이 값 v 를 보이는 글자 그대로 나눈 것인지 본다(리뷰 5회차 [1] 규칙 — WrapLabel 을 부르지 않고 손으로
// 적었다). 줄은 비지 않고 앞뒤·연속 빈칸이 없으며, 차례로 이으면 v 다. 줄 경계에서 허용하는 것은 둘뿐이다 — v 의 그 자리 글자가
// 빈칸 하나면 그 빈칸이 줄바꿈이 된다(빠진다). 아니면 긴 낱말을 강제로 자른 것이라 아무 글자도 더하거나 빼지 않는다.
func jWrapOK(v string, lines []string) bool {
	if len(lines) == 0 {
		return false
	}
	pos := 0
	for i, ln := range lines {
		if ln == "" || ln[0] == ' ' || ln[len(ln)-1] == ' ' || strings.Contains(ln, "  ") {
			return false
		}
		if i > 0 && pos < len(v) && v[pos] == ' ' {
			pos++
		}
		if !strings.HasPrefix(v[pos:], ln) {
			return false
		}
		pos += len(ln)
	}
	return pos == len(v)
}

// shows 는 v 가 SVG 에 보이는지다 — aria-label·<title> 은 값 그대로, <text> 는 jWrapOK 로.
func (o jSVGOut) shows(v string) bool {
	if o.aria == v || slices.Contains(o.title, v) {
		return true
	}
	return o.textOf(v) != nil
}

// textOf 는 v 를 jWrapOK 로 담은 첫 <text> 의 줄 목록이다(없으면 nil).
func (o jSVGOut) textOf(v string) []string {
	for _, ls := range o.texts {
		if jWrapOK(v, ls) {
			return ls
		}
	}
	return nil
}

// jCheck 는 원문 하나를 Parse·Render·판정기로 보고 기대(ok·want)와 견준다 — 받으면 값과 최종 SVG 의 코드 포인트까지.
func jCheck(t *testing.T, name, src string, ctx jCtx, ok bool, want string) {
	t.Helper()
	c, _, err := Parse(src)
	jm, jl := jJudge(src)
	if !ok {
		var de *diagram.Error
		if err == nil || !errors.As(err, &de) || de.Line != ctx.line {
			t.Errorf("%s/%s: 거부(%d 줄)를 기대, Parse %v\n%q", ctx.name, name, ctx.line, err, src)
		}
		if jm != nil || jl != ctx.line {
			t.Errorf("%s/%s: 판정기 거부 줄 %d, 기대 %d\n%q", ctx.name, name, jl, ctx.line, src)
		}
		return
	}
	if err != nil {
		t.Errorf("%s/%s: 받기를 기대, Parse %v\n%q", ctx.name, name, err, src)
		return
	}
	if jm == nil {
		t.Errorf("%s/%s: 판정기가 %d 줄에서 거부\n%q", ctx.name, name, jl, src)
	}
	if got := jValue(c, ctx.name); got != want {
		t.Errorf("%s/%s: 값 %U, 기대 %U\n%q", ctx.name, name, []rune(got), []rune(want), src)
	}
	r, err := Render(src)
	if err != nil {
		t.Fatalf("%s/%s: Render %v", ctx.name, name, err)
	}
	if !jParseSVG(t, r.SVG).shows(want) {
		t.Errorf("%s/%s: SVG 에 %U 가 없음\n%s", ctx.name, name, []rune(want), r.SVG)
	}
}

// TestEntityPolicyTable 은 정책 B 고정 표를 일곱 문맥에 교차해 Parse 값·판정기·최종 SVG 코드 포인트를 본다. 거부 사례에도
// 상태 A 를 두어 「그릴 상태 없음」으로 우연히 거부되지 않게 한다.
func TestEntityPolicyTable(t *testing.T) {
	if unicode.Version != unicodeVersion {
		t.Fatalf("Go unicode 판본 %s — 숫자 정책은 %s 표에 기대어 적었다(안내서 12장·jCfRanges·jWSRanges 를 새 판본으로 다시 본다)", unicode.Version, unicodeVersion)
	}
	for _, e := range jEntityTable {
		for _, ctx := range jCtxs {
			jCheck(t, e.frag, ctx.src("x"+e.frag+"y"), ctx, e.want != nil, "x"+string(e.want)+"y")
		}
	}
	for _, e := range jRawTable {
		for _, ctx := range jCtxs {
			ok, want := e.lblOK, e.lbl
			if ctx.acc {
				ok, want = e.accOK, e.acc
			}
			jCheck(t, fmt.Sprintf("%q", e.frag), ctx.src("x"+e.frag+"y"), ctx, ok, "x"+want+"y")
		}
	}
}

// TestEntityNumericRange 는 구현의 숫자 정책(numOK — Go unicode 표)과 판정기의 손 목록(jNumPolicy)을 0~U+110000 전 범위에서 견준다.
func TestEntityNumericRange(t *testing.T) {
	for n := 0; n <= 0x110000; n++ {
		if numOK(n) != jNumPolicy(n) {
			t.Fatalf("U+%04X: 구현 %v, 판정기 %v", n, numOK(n), jNumPolicy(n))
		}
	}
}

// TestAccAndWrapSVG 는 접근성 값의 안쪽 빈칸·탭·줄바꿈이 XML 로 읽은 aria-label·<title> 에 값 그대로 들어가고, 긴 라벨은 줄바꿈
// 규칙(jWrapOK)대로 나뉘는지 본다. aria-label 은 탭을 &#9; 로 쓴다 — 리터럴 탭은 XML 속성값 정규화로 빈칸이 된다(리뷰 5회차 [2]).
func TestAccAndWrapSVG(t *testing.T) {
	long := strings.TrimSpace(strings.Repeat("가나다 ", 20) + "#35;끝 " + strings.Repeat("라마 ", 20))
	want := strings.ReplaceAll(long, "#35;", "#")
	src := "stateDiagram-v2\naccTitle:  a\tb  #+65; #35;35; \naccDescr {\n  첫 줄\t끝  \n\n\t  둘째  칸\n  }\nA : " + long + "\nA --> B : " + long
	r, err := Render(src)
	if err != nil {
		t.Fatal(err)
	}
	o := jParseSVG(t, r.SVG)
	if o.aria != "a\tb  #+65; #35;" {
		t.Errorf("aria-label %q", o.aria)
	}
	if !strings.Contains(r.SVG, `aria-label="a&#9;b  #+65; #35;"`) {
		t.Errorf("aria-label 의 탭이 &#9; 가 아님\n%s", r.SVG)
	}
	if !slices.Contains(o.title, "첫 줄\t끝  \n둘째  칸") {
		t.Errorf("<title> %q", o.title)
	}
	n := 0
	for _, ls := range o.texts {
		if jWrapOK(want, ls) {
			n++
			if len(ls) < 2 {
				t.Errorf("긴 라벨이 줄바꿈되지 않음: %q", ls)
			}
		}
	}
	if n != 2 {
		t.Errorf("긴 라벨(설명·전이)을 담은 <text> %d 개, 기대 2\n%q", n, o.texts)
	}
}

// TestAriaLabelTab 은 accTitle 의 탭·연속 빈칸이 XML 로 읽은 aria-label 에 그대로 남는지 본다(리뷰 5회차 [2] 고정 사례).
func TestAriaLabelTab(t *testing.T) {
	for _, v := range []string{"a\tb", "\t", "가\t\t나  다", "x \t y", "끝\t"} {
		src := "stateDiagram-v2\naccTitle: 앞" + v + "뒤\nA"
		c, _, err := Parse(src)
		if err != nil {
			t.Fatalf("%q: %v", v, err)
		}
		r, _ := Render(src)
		if got := jParseSVG(t, r.SVG).aria; got != c.AccTitle || got != "앞"+v+"뒤" {
			t.Errorf("%q: aria-label %q, AccTitle %q", v, got, c.AccTitle)
		}
	}
}

// TestWrapOutput 은 줄바꿈되는 세 문맥(상태 설명 위 칸·아래 칸, 복합 상태 제목, 전이 라벨)의 최종 SVG 줄이 받은 값을 보이는 글자
// 그대로 나눈 것인지 XML 로 읽어 본다(리뷰 5회차 [1] 고정 사례). 줄 목록을 빈칸으로 이은 것이 원문과 같다고는 약속하지 않는다.
func TestWrapOutput(t *testing.T) {
	words := "alpha beta gamma delta epsilon zeta eta theta"
	alpha := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	hangul := strings.Repeat("가나다라마바사아자차카타파하", 3)
	mixed := "짧음 " + alpha + " 끝 " + hangul + " z"
	ctxs := []struct {
		name string
		src  func(v string) string
	}{
		{"설명 위 칸", func(v string) string { return "stateDiagram-v2\nA : " + v }},
		{"설명 아래 칸", func(v string) string { return "stateDiagram-v2\nA : 머리\nA : " + v }},
		{"복합 상태 제목", func(v string) string { return "stateDiagram-v2\nstate \"" + v + "\" as M {\nA\n}" }},
		{"전이 라벨", func(v string) string { return "stateDiagram-v2\nA --> B : " + v }},
	}
	for _, ctx := range ctxs {
		for _, v := range []string{words, alpha, hangul, mixed} {
			src := ctx.src(v)
			r, err := Render(src)
			if err != nil {
				t.Fatalf("%s/%q: %v", ctx.name, v, err)
			}
			o := jParseSVG(t, r.SVG)
			ls := o.textOf(v)
			if ls == nil {
				t.Errorf("%s/%q: 규칙대로 나뉜 <text> 가 없음\n%q", ctx.name, v, o.texts)
				continue
			}
			// 복합 제목 칸은 더 넓어 26 자 낱말 하나는 한 줄에 든다
			if len(ls) < 2 && !(ctx.name == "복합 상태 제목" && v == alpha) {
				t.Errorf("%s/%q: 줄바꿈되지 않음 %q", ctx.name, v, ls)
			}
		}
	}
	// 대조: 긴 상태 이름(ID)은 빈칸 없이 강제로 잘린다 — 줄을 그대로 이으면 원문이다.
	r, _ := Render("stateDiagram-v2\n" + alpha + strings.Repeat("Z", 20))
	o := jParseSVG(t, r.SVG)
	if ls := o.textOf(alpha + strings.Repeat("Z", 20)); len(ls) < 2 || strings.Join(ls, "") != alpha+strings.Repeat("Z", 20) {
		t.Errorf("긴 ID 의 강제 자름 %q", ls)
	}
	// 규칙 자체의 대조 — 경계의 빈칸 더함·글자 빠짐·연속 빈칸은 실패여야 한다(뮤턴트와 같은 꼴).
	for _, c := range []struct {
		v     string
		lines []string
		ok    bool
	}{
		{"ab cd", []string{"ab", "cd"}, true},
		{"abcd", []string{"ab", "cd"}, true},
		{"ab cd", []string{"ab ", "cd"}, false},
		{"ab cd", []string{"ab", " cd"}, false},
		{"abcd", []string{"a", "cd"}, false},
		{"ab cd", []string{"abcd"}, false},
		{"ab  cd", []string{"ab", "cd"}, false},
		{"ab", []string{"ab", ""}, false},
	} {
		if jWrapOK(c.v, c.lines) != c.ok {
			t.Errorf("jWrapOK(%q, %q) 가 %v 가 아님", c.v, c.lines, c.ok)
		}
	}
}

// TestCaptionOutput 은 frontmatter title(YAML 이스케이프를 거친 값)이 출력 관문을 지나 Result.Caption·SVG aria-label 에 같은
// 코드 포인트로 나가는지, XML 이 담지 못하는 글자·안쪽 연속 빈칸·탭이면 title 줄(2)에서 원문인지 본다(정책 B — 다섯 종류와 같은
// diagram.Caption). 엔티티 #…; 는 글자 그대로다(원본은 frontmatter 를 엔티티 전처리 전에 떼어 낸다). 기대값은 손으로 적었다.
func TestCaptionOutput(t *testing.T) {
	for _, c := range []struct {
		yaml string
		want []rune // nil = 거부(2번 줄 — title 줄)
	}{
		{`"비율 #35;"`, []rune("비율 #35;")},
		{`"a#lt;b#gt;"`, []rune("a#lt;b#gt;")},
		{`"#copy;"`, []rune("#copy;")},
		{`"x  y"`, nil},
		{`"x\ty"`, nil},
		{`"x\uFFFEy"`, nil},
		{`"x\uFFFFy"`, nil},
		{`"\uFFFF"`, nil},
		{`"x\uFFFDy"`, []rune{0x78, 0xfffd, 0x79}},
		{`"x\uFFFCy"`, []rune{0x78, 0xfffc, 0x79}},
		{`"x\uFDD0y"`, []rune{0x78, 0xfdd0, 0x79}},
		{`"x\uD7FFy"`, []rune{0x78, 0xd7ff, 0x79}},
		{`"x\uE000y"`, []rune{0x78, 0xe000, 0x79}},
		{`"따옴표 \"제목\""`, []rune("따옴표 \"제목\"")},
		{`주문 상태`, []rune("주문 상태")},
	} {
		for _, acc := range []bool{false, true} {
			src := "---\ntitle: " + c.yaml + "\n---\nstateDiagram-v2\nA"
			if acc {
				src += "\naccTitle: 접근 제목"
			}
			jm, jl := jJudge(src)
			r, err := Render(src)
			if c.want == nil {
				var de *diagram.Error
				if !errors.As(err, &de) || de.Line != 2 || jm != nil || jl != 2 {
					t.Errorf("%s: 2번 줄 거부를 기대, Render %v, 판정기 %d", c.yaml, err, jl)
				}
				continue
			}
			want := string(c.want)
			if err != nil || jm == nil {
				t.Errorf("%s: 받기를 기대, Render %v, 판정기 거부 %d", c.yaml, err, jl)
				continue
			}
			if r.Caption != want || jm.caption != want {
				t.Errorf("%s: Caption %U·판정기 %U, 기대 %U", c.yaml, []rune(r.Caption), []rune(jm.caption), c.want)
			}
			aria := jParseSVG(t, r.SVG).aria
			if wantAria := map[bool]string{false: want, true: "접근 제목"}[acc]; aria != wantAria {
				t.Errorf("%s: aria-label %U, 기대 %U", c.yaml, []rune(aria), []rune(wantAria))
			}
		}
	}
	// 본문 오류가 먼저다 — 관문은 본문을 다 읽은 뒤에 돈다(판정기도 같은 차례).
	src := "---\ntitle: \"x\\uFFFEy\"\n---\nstateDiagram-v2\nA\nnote right of A : 메모"
	_, err := Render(src)
	var de *diagram.Error
	if _, jl := jJudge(src); !errors.As(err, &de) || de.Line != 6 || jl != 6 {
		t.Errorf("본문 오류(6번 줄)가 먼저여야 함: Render %v, 판정기 %d", err, jl)
	}
	// 캡션의 빈칸 정책은 관문이 아니라 frontmatter 를 읽을 때 본다 — 본문 오류보다 먼저다(er·sequence 와 같은 차례).
	src = "---\ntitle: \"x  y\"\n---\nstateDiagram-v2\nA\nnote right of A : 메모"
	_, err = Render(src)
	if _, jl := jJudge(src); !errors.As(err, &de) || de.Line != 2 || jl != 2 {
		t.Errorf("캡션 빈칸 거부(2번 줄)가 먼저여야 함: Render %v, 판정기 %d", err, jl)
	}
}
