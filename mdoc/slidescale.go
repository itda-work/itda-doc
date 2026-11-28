package mdoc

import (
	"fmt"
	"regexp"
	"strconv"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
)

// 슬라이드 장·코드 블록 배율과 맞춤(itda-hyve#1116).
//
//	## 제목 {scale=0.75}      이 장(과 그 「(계속)」 장)의 본문 배율 — slides.font-scale 을 대신하는 절대값, 0.6~1.5
//	## 제목 {.fit}            본문이 한 장에 다 들어가는 가장 큰 배율(상한 = 그 장의 배율, 하한 0.6)
//	```python scale=0.8      그 블록의 코드 글자 배율 — 놓인 자리의 코드 크기에 곱하는 값, 0.55~1.5
//	```python fit            가장 긴 줄이 폭에 들어가게, 슬라이드에서는 높이도 놓인 자리에 맞게 줄인다(하한 0.55, 상한 = scale= 또는 1)
//
// mdoc 은 값 검사만 한다 — 제목의 scale·class fit 은 Pandoc 속성 그대로 data-scale·class="fit" 으로 나가고(틀린 scale 값은 경고하고
// 뗀다), 코드 블록은 pre 에 data-scale(검사를 마친 값)·data-fit="on" 을 단다(render.go codeBlock). 줄이는 것은 화면이다 —
// web/slides.js makeGroup(장 배율·장 맞춤)·web/pager.js(코드 배율·코드 맞춤)·web/base.css(--slide-body-scale·--code-scale).
// 장 속성은 슬라이드를 여는 제목(맨 위 블록이면서 슬라이드 단계인 제목)에만 뜻이 있다 — 그 밖의 제목에 적으면 경고하고 화면이 무시한다.
// 문서(A4)·이어보기·docx·hwpx 는 장 속성을 무시한다(그림 .fit 과 같은 규칙).

const (
	slideScaleMin, slideScaleMax = 0.6, 1.5  // 장 배율 — slides.font-scale 과 같은 범위(meta.go)
	codeScaleMin, codeScaleMax   = 0.55, 1.5 // 코드 배율 — 하한은 WebKit 최소 글자 크기 9px(코드 기본 14pt × 0.9 ≈ 16.8px × 0.55 ≈ 9.2px)
	codeFitWord                  = "fit"
)

// scaleNumRe 는 배율 값의 꼴이다 — 따옴표·단위·부호 없는 십진수(0.75·.8·1).
var scaleNumRe = regexp.MustCompile(`^(?:\d+(?:\.\d+)?|\.\d+)$`)

// parseScaleValue 는 v 가 [lo, hi] 안의 배율 숫자면 그 값을 돌려준다.
func parseScaleValue(v string, lo, hi float64) (float64, bool) {
	if !scaleNumRe.MatchString(v) {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f < lo || f > hi {
		return 0, false
	}
	return f, true
}

// clipValue 는 경고 문구에 넣을 값을 50자로 줄인다.
func clipValue(v string) string {
	if utf8.RuneCountInString(v) > 50 {
		return string([]rune(v)[:50]) + "…"
	}
	return v
}

// slideHeadingAttrs 는 제목 속성의 scale·.fit 을 검사한다(processHeadings 가 속성을 뗀 뒤 부른다). 틀린 scale 값은 경고하고
// data 에서 뗀다. 슬라이드를 여는 제목이 아닌 곳의 scale·.fit 은 경고만 한다(값·class 는 그대로 나가고 화면이 무시한다).
// 발표자 노트 안 제목은 슬라이드가 아니므로 같은 경고다.
func (st *state) slideHeadingAttrs(doc ast.Node, hs []*ast.Heading) {
	lvl := slideLevel(doc)
	if st.meta.Slides != nil && st.meta.Slides.Level > 0 {
		lvl = st.meta.Slides.Level
	}
	for _, h := range hs {
		info := st.headings[h]
		if info == nil {
			continue
		}
		line := st.lines.line(h.Pos())
		v, hasScale := attrs{Data: info.data}.get("scale")
		if hasScale {
			if _, ok := parseScaleValue(v, slideScaleMin, slideScaleMax); !ok {
				st.warn(line, WarnBadAttribute, fmt.Sprintf("장 배율 scale=%s 값이 틀려 무시함(따옴표 없이 0.6~1.5 의 숫자)", clipValue(v)))
				info.data = attrs{Data: info.data}.without("scale")
				hasScale = false
			}
		}
		hasFit := attrs{Classes: info.classes}.has(codeFitWord)
		if !hasScale && !hasFit {
			continue
		}
		if h.Parent() != doc || lvl == 0 || h.Level != lvl {
			st.warn(line, WarnBadAttribute, fmt.Sprintf("장 배율 scale=·.fit 은 슬라이드를 여는 제목(맨 위의 %s 제목)에만 — 이 제목에서는 무시함", slideLevelMark(lvl)))
		}
	}
}

// slideLevelMark 는 슬라이드 단계 제목 표기(## 등)다. 단계가 없으면(제목 없는 문서) "슬라이드 단계" 다.
func slideLevelMark(lvl int) string {
	if lvl < 1 || lvl > 6 {
		return "슬라이드 단계"
	}
	return string([]byte("######")[:lvl])
}

// parseCodeScale 은 펜스 info string 에서 코드 배율(scale=, 없으면 "")과 경고 문구를 돌려준다. 낱말 규칙은 cols=(codecols.go)와
// 같다 — 따옴표 없는 낱말만, 다른 속성의 따옴표 값 안은 후보가 아니고, 같은 낱말이 여럿이면 앞의 것이 이긴다. 범위 밖·틀린 꼴은
// 경고하고 무시한다. 값은 원문 그대로(검사를 마친 십진수) 돌려준다.
func parseCodeScale(info string) (string, []string) {
	words := fenceAttrWords(info, "scale=")
	if len(words) == 0 {
		return "", nil
	}
	v := words[0][len("scale="):]
	if _, ok := parseScaleValue(v, codeScaleMin, codeScaleMax); ok {
		return v, nil
	}
	return "", []string{fmt.Sprintf("코드 블록 배율: scale=%s 값이 틀려 무시함(따옴표 없이 0.55~1.5 의 숫자)", clipValue(v))}
}
