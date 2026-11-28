package mdoc

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// 코드 블록 하나만 줄 번호 끄기(itda-hyve#1024): 펜스 줄에 따옴표 없는 낱말 nonumber 를 적으면 그 블록에 줄 번호를 달지 않는다.
//
//	```text nonumber
//	```bash title="run.sh" nonumber
//
// mdoc 은 pre 에 data-ln="off" 속성만 낸다(render.go codeBlock). 번호를 빼는 것은 화면이다 — codelines.js 가 그 블록을
// 문서 전체를 끈 numbers:false 와 같은 꼴로 감싼다(줄 강조·단·diff 표지는 그대로, has-ln·번호 칸 없음). 다른 뷰어는
// 이 낱말을 무시한다.
//
// 낱말 경계·따옴표 건너뛰기는 cols=(codecols.go)·그림 자리(diagramsize.go fenceAttrWords)와 같다 — 따옴표 없는 낱말만, 다른
// 속성의 따옴표 값 안(title="a nonumber"·nonumbers="a nonumber")은 낱말이 아니다(훑기는 hasCodeNoNumber 가 따로 한다). 값 없는 낱말이라 정확히 nonumber 인 낱말만 받는다(nonumbers·
// nonumber=1·mynonumber 는 아니다 — 경고도 없다). 첫 낱말(언어 자리)이 nonumber 면 언어 없음 + 번호 끔으로 본다
// (parseCodeInfo — 「nonumber」라는 강조 언어는 없다).

// codeNoNumberWord 는 펜스 낱말 이름이다.
const codeNoNumberWord = "nonumber"

// hasCodeNoNumber 는 info string(줄 강조 {…} 를 뗀 나머지)에 nonumber 낱말이 있는지 본다. fenceAttrWords 는 접두 일치 후보를
// 다음 공백까지만 읽어(값 검사가 틀린 값을 가린다) nonumbers="a nonumber b" 처럼 접두가 같은 낱말의 따옴표 값을 쪼갠다 —
// 값 없는 낱말에는 그 규칙이 맞지 않아 따로 훑는다(리뷰 1회차 [1]): 낱말은 맨 앞·unicode.IsSpace 공백 뒤에서 시작해 다음 공백까지,
// 낱말 안의 따옴표 구간("…"·'…')은 닫는 따옴표까지 공백을 품은 채 한 덩어리다. 닫히지 않은 따옴표면 줄 끝까지가 그 낱말이라
// 뒤는 읽지 않는다(fenceAttrWords 와 같다). 낱말 전체가 정확히 nonumber 일 때만 참이다.
func hasCodeNoNumber(info string) bool { return hasCodeWord(info, codeNoNumberWord) }

// hasCodeWord 는 info string 에 값 없는 낱말 word 가 정확히 있는지 본다 — 훑기 규칙은 위 hasCodeNoNumber 와 같다(코드 맞춤 fit 도
// 이 길이다 — slidescale.go).
func hasCodeWord(info, word string) bool {
	i := 0
	for i < len(info) {
		r, sz := utf8.DecodeRuneInString(info[i:])
		if unicode.IsSpace(r) {
			i += sz
			continue
		}
		start := i
		for i < len(info) {
			if c := info[i]; c == '"' || c == '\'' {
				j := strings.IndexByte(info[i+1:], c)
				if j < 0 {
					return false // 닫히지 않은 따옴표 — 줄 끝까지 이 낱말의 값
				}
				i += j + 2
				continue
			}
			r2, sz2 := utf8.DecodeRuneInString(info[i:])
			if unicode.IsSpace(r2) {
				break
			}
			i += sz2
		}
		if info[start:i] == word {
			return true
		}
	}
	return false
}
