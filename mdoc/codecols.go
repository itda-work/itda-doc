package mdoc

import (
	"fmt"
	"unicode/utf8"
)

// 코드 블록 단 나누기(itda-hyve#487): 펜스 줄의 cols=2·cols=3 으로 코드 블록 하나를 N단으로 흘려 그린다.
//
//	```python title="app.py" cols=2
//
// mdoc 은 검사를 마친 값만 pre 의 data-cols 속성으로 낸다(render.go codeBlock). 단 나눔은 화면이 한다 —
// codelines.js 가 줄마다 span.ln 으로 감싸고 base.css 가 pre.has-cols[data-cols] > code 를 CSS 다단으로 흘린다
// (자동 높이 균형, 줄 span 은 단 경계에서 잘리지 않는다). 다른 뷰어(GitHub 등)는 이 낱말을 무시한다.
//
// 낱말 규칙은 그림 자리(diagramsize.go fenceAttrWords)와 같다 — 따옴표 없는 낱말만, 다른 속성의 따옴표 값
// 안(title="a cols=2")은 후보가 아니다. 같은 낱말이 여럿이면 앞의 것이 이긴다(틀린 값도 자리를 잡는다).
// cols=1 은 단 없음과 같아 속성도 경고도 없다. 그 밖의 값은 펜스 줄 번호로 WarnBadAttribute 경고를 남기고 무시한다.
// 그림이 된 mermaid·수식에는 부르지 않는다 — 그리지 못해 코드 블록으로 남으면 보통 코드 블록처럼 적용한다.

// parseCodeCols 는 펜스 info string 에서 단 수(2·3, 없으면 0)와 경고 문구를 돌려준다.
func parseCodeCols(info string) (int, []string) {
	words := fenceAttrWords(info, "cols=")
	if len(words) == 0 {
		return 0, nil
	}
	v := words[0][len("cols="):]
	switch v {
	case "1":
		return 0, nil
	case "2":
		return 2, nil
	case "3":
		return 3, nil
	}
	if utf8.RuneCountInString(v) > 50 { // 경고 문구에 너무 긴 값을 그대로 두지 않는다
		v = string([]rune(v)[:50]) + "…"
	}
	return 0, []string{fmt.Sprintf("코드 블록 단: cols=%s 값이 틀려 무시함(따옴표 없이 2 또는 3)", v)}
}
