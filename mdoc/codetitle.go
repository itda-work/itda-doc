package mdoc

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/yuin/goldmark/ast"
)

// 코드 블록 파일명 머리 띠(itda-hyve#441): 펜스 info string 에 파일명을 적으면 pre 에 data-title 속성을 달고
// 화면(base.css ::before)이 코드 위에 머리 띠로 그린다. 표기는 둘 다 받는다 — 정식 title= (Docusaurus·
// MkDocs) 과 줄임 언어:파일명(Zenn·Qiita).
//
//	```yaml title="docker-compose.yml"   → 언어 yaml, 파일명 docker-compose.yml
//	```yaml:docker-compose.yml           → 언어 yaml, 파일명 docker-compose.yml
//	```title=".env"                      → 언어 없음(첫 낱말에 = 가 있으면 언어가 없다), 파일명 .env
//	```:.env                             → 줄임 + 언어 없음
//
// 언어가 없으면 파일명 확장자·이름(chroma lexers.Match)으로 강조 언어를 짐작한다 — 짐작한 언어도 기존 강조
// 허용 목록·상한(render.go codeBlock)을 그대로 통과한다. 파일명은 속성값일 뿐 코드 글자(textContent·복사·
// 줄 번호·검색)에 들어가지 않는다.

// maxCodeTitle 은 머리 띠 파일명의 길이 상한이다(글자 수).
const maxCodeTitle = 200

// codeInfo 는 info string 을 가른 결과다. lang 은 원문 첫 낱말에서 콜론 앞까지만(정규화는 cleanLang 이 한다).
type codeInfo struct {
	lang  string
	title string // 머리 띠 파일명(빈 값이면 띠를 그리지 않는다)
	hl    string // 줄 강조 낱말 {…} 안 글자(codehl.go — 정규화 전)
	hasHL bool   // 줄 강조 낱말이 있다({} 처럼 비어도 — 펜스 앞 지시어보다 이긴다)
}

// titleRe 는 title="…"·title='…'·title=값(공백 없음) 낱말을 찾는다. 낱말 경계(맨 앞 또는 공백)를
// 요구해 mytitle= 같은 다른 낱말은 잡지 않는다. 값이 따옴표로 시작하면 닫는 따옴표까지가 값이고(공백을
// 품을 수 있다), 닫는 따옴표가 없으면 그 시도는 값이 없는 것이다(따옴표 없는 값은 따옴표로 시작하지 못한다).
var titleRe = regexp.MustCompile(`(?:^|\s)title=(?:"([^"\n\r]*)"|'([^'\n\r]*)'|([^\s"']*))`)

// titleWord 는 s 에서 title= 낱말의 값을 돌려준다(없으면 ""). 값이 빈 따옴표(title="")면 띠 없음과 같다.
func titleWord(s string) string {
	m := titleRe.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	for _, v := range m[1:] {
		if v != "" {
			return v
		}
	}
	return ""
}

// sanitizeTitle 은 머리 띠 파일명에서 제어 문자를 지우고 maxCodeTitle 글자를 넘지 않게 자른다(글자 수).
func sanitizeTitle(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	r := []rune(s)
	if len(r) > maxCodeTitle {
		r = r[:maxCodeTitle]
	}
	return string(r)
}

// parseCodeInfo 는 펜스 info string(여는 펜스 뒤 첫 줄)을 언어와 파일명으로 가른다.
//   - 첫 낱말에 `=` 가 있으면(`title="…"` 꼴) 언어는 없다.
//   - 첫 낱말의 첫 `:` 는 줄임 `언어:파일명` 구분이다. 파일명이 비면 띠 없음(언어는 그대로).
//   - title= 낱말은 첫 낱말이든 나머지든 찾는다. 줄임 파일명과 title= 이 둘 다 있으면 줄임이 이긴다.
//   - 줄 강조 낱말 `{1,3-5}`(itda-hyve#479 — codehl.go cutHLWord)는 먼저 떼어 내고 나머지를 위 규칙으로 가른다.
//
// 기존 출력 불변: 언어만 적힌 info string(`go`·`math`·`mermaid`)은 lang 이 그 첫 낱말 그대로다. {…} 가 없으면
// 언어·파일명 결과는 줄 강조 이전과 같다.
func parseCodeInfo(info []byte) codeInfo {
	rest, hl, hasHL := cutHLWord(string(info))
	ci := parseCodeLangTitle(rest)
	ci.hl, ci.hasHL = hl, hasHL
	return ci
}

// parseCodeLangTitle 은 줄 강조 낱말을 뗀 info string 을 언어와 파일명으로 가른다(itda-hyve#441).
func parseCodeLangTitle(s string) codeInfo {
	s, _ = codeInfoText(s)
	if s == "" {
		return codeInfo{}
	}
	first := codeInfoFirst(s)
	if strings.ContainsRune(first, '=') {
		return codeInfo{title: sanitizeTitle(titleWord(s))}
	}
	if before, after, found := strings.Cut(first, ":"); found {
		if short := sanitizeTitle(after); short != "" {
			return codeInfo{lang: before, title: short}
		}
		return codeInfo{lang: before, title: sanitizeTitle(titleWord(s))}
	}
	return codeInfo{lang: first, title: sanitizeTitle(titleWord(s))}
}

// codeInfoText 는 info string 가르기의 전처리다 — 앞뒤 공백(unicode.IsSpace)을 지운 글과, 앞에서 지운 바이트 수
// (원문 좌표로 되돌릴 때 더한다). parseCodeLangTitle 과 줄 강조 보호 구간(codehl.go hlProtected)이 같이 쓴다.
func codeInfoText(s string) (text string, lead int) {
	rest := strings.TrimLeftFunc(s, unicode.IsSpace)
	return strings.TrimRightFunc(rest, unicode.IsSpace), len(s) - len(rest)
}

// codeInfoFirst 는 전처리한 info string 의 첫 낱말(첫 공백 앞까지)이다.
func codeInfoFirst(s string) string {
	if i := strings.IndexFunc(s, unicode.IsSpace); i >= 0 {
		return s[:i]
	}
	return s
}

// guessCodeLang 은 파일명(확장자·이름)으로 강조 언어를 짐작해 cleanLang 을 통과한 이름을 돌려준다.
// 짐작하지 못하면 ""(일반 코드). 짐작한 언어는 codeBlock 의 기존 허용 목록·상한을 그대로 통과한다.
func guessCodeLang(title string) string {
	lexer := lexers.Match(title)
	if lexer == nil {
		return ""
	}
	aliases := lexer.Config().Aliases
	if len(aliases) == 0 {
		return ""
	}
	return cleanLang([]byte(aliases[0]))
}

// fenceInfo 는 펜스의 info string 원문(여는 줄에서 펜스 표시를 뺀 나머지)을 돌려준다. info 가 없으면 nil.
// goldmark 의 Language() 는 첫 낱말만 주기 때문에 파일명 표기까지 보려면 info 전체가 필요하다.
func fenceInfo(fc *ast.FencedCodeBlock, src []byte) []byte {
	if fc.Info == nil {
		return nil
	}
	return fc.Info.Segment.Value(src)
}
