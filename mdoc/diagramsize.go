package mdoc

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark/util"
)

// 그림 자리 지정(#475): ```mermaid 펜스 줄의 ratio=W:H·width=N%·height=N% 으로 그림이 차지할 상자를 정한다.
// 그림(SVG)은 상자 안에 비율을 지켜 최대한 크게·가운데 그린다(contain) — 배치를 다시 짜지는 않는다.
// 다른 뷰어(GitHub 등)는 이 속성을 무시하고 그림을 그대로 그리므로 원문 호환은 지켜진다.
//
// mdoc 은 검사를 마친 값만 data-diagram-* 속성으로 내고, 화면(pager.js sizeDiagrams)이 값을 다시 검사해
// CSSOM 으로 상자를 건다 — 문서 창의 CSP(style-src 'self') 가 style 속성을 막기 때문에(run.go csp).
// column 의 style="width:…" 를 화면이 data-w 로 옮겨 다시 거는 것과 같은 선례다.
// 틀린 값은 그 펜스 줄 번호로 WarnDiagram 경고를 남기고 그 값만 무시한다. 그림을 그리지 못해 코드 블록으로
// 남으면 속성도 경고도 쓰지 않는다 — parseDiagramSize 는 그림이 성공한 뒤에만 부른다(render.go codeBlock).

// diagramSize 는 펜스 줄에서 가린 그림 상자 값이다. 0 은 없음.
type diagramSize struct {
	ratioW, ratioH int // ratio=W:H — 상자의 가로:세로 비율
	width, height  int // width — 그림이 놓인 자리 너비의, height — 본문 높이의 백분율(10~100. 기준은 화면이 정한다)
}

// 크기 속성은 따옴표 없는 낱말만 받는다(#475 결정, 2026-10-31 — 리뷰 2회 연속 같은 종류 발견으로 단순화, #208).
// info string 을 낱말로 훑는데 낱말 경계는 맨 앞과 unicode.IsSpace 공백 뒤뿐이다:
//   - 경계에서 ratio=·width=·height= 로 시작하는 낱말은 다음 공백까지 통째로 후보로 삼는다 — 값에 따옴표가
//     든 낱말(ratio="16:9"·width=50%"junk")은 아래 엄격한 값 정규식에 걸려 경고 후 무시된다.
//   - 그 밖의 낱말은 건너뛴다 — 낱말 안의 따옴표 구간("…"·'…')은 닫는 따옴표까지(없으면 줄 끝까지) 한 덩어리로
//     지나가므로 다른 속성의 따옴표 값 안(title="a width=50%")의 크기 꼴은 후보가 되지 않는다.
// title= 파일명 띠(codetitle.go)와는 나누기·계약을 섞지 않는다.

var (
	diagramRatioRe = regexp.MustCompile(`^([1-9][0-9]{0,1}):([1-9][0-9]{0,1})$`) // 1~99 정수 둘
	diagramPctRe   = regexp.MustCompile(`^(100|[1-9][0-9])%$`)                   // 10~100 정수 + %
)

// diagramWords 는 info string 에서 그림 자리 속성 후보 낱말(경계에서 ratio=·width=·height= 로 시작하는
// 낱말 전체)을 차례로 낸다.
func diagramWords(s string) []string {
	return fenceAttrWords(s, "ratio=", "width=", "height=")
}

// fenceAttrWords 는 info string 에서 경계에서 prefixes 중 하나로 시작하는 낱말 전체를 차례로 낸다(위 낱말 규칙).
// 코드 블록 단 나누기 cols=(codecols.go)도 같은 규칙을 쓴다.
func fenceAttrWords(s string, prefixes ...string) []string {
	hasPrefix := func(t string) bool {
		for _, p := range prefixes {
			if strings.HasPrefix(t, p) {
				return true
			}
		}
		return false
	}
	var words []string
	i := 0
	for i < len(s) {
		r, sz := utf8.DecodeRuneInString(s[i:])
		if unicode.IsSpace(r) {
			i += sz
			continue
		}
		if !hasPrefix(s[i:]) {
			for i < len(s) { // 그 밖의 낱말 — 따옴표 구간을 통째로 지나간다
				if c := s[i]; c == '"' || c == '\'' {
					if j := strings.IndexByte(s[i+1:], c); j >= 0 {
						i += j + 2
						continue
					}
					return words // 닫히지 않은 따옴표 — 줄 끝까지가 그 속성의 값이라 뒤는 안 읽는다
				}
				r2, sz2 := utf8.DecodeRuneInString(s[i:])
				if unicode.IsSpace(r2) {
					break
				}
				i += sz2
			}
			continue
		}
		j := i // 크기 낱말 — 다음 공백까지 통째로(따옴표가 든 값은 값 검사에서 틀린 값이 된다)
		for j < len(s) {
			r2, sz2 := utf8.DecodeRuneInString(s[j:])
			if unicode.IsSpace(r2) {
				break
			}
			j += sz2
		}
		words = append(words, s[i:j])
		i = j
	}
	return words
}

// parseDiagramSize 는 펜스 info string 에서 그림 상자 값을 가린다. 같은 속성이 여럿이면 앞의 것이 이긴다.
// 돌려주는 경고 문구는 펜스 줄에 달 것이다(줄 번호는 부르는 쪽이 안다).
// ratio·width·height 를 모두 적으면 ratio 를 쓰지 않는다(경고 한 번).
func parseDiagramSize(info string) (diagramSize, []string) {
	vals := map[string]string{} // 속성 이름 → 낱말 전체(첫 낱말이 이긴다 — 틀린 값도 자리를 잡아 뒤의 것을 밀어낸다)
	for _, w := range diagramWords(info) {
		name := w[:strings.IndexByte(w, '=')] // diagramWords 의 낱말은 셋 중 하나로 시작한다
		if _, seen := vals[name]; !seen {
			vals[name] = w
		}
	}
	var sz diagramSize
	var warns []string
	bad := func(name, val, want string) {
		if utf8.RuneCountInString(val) > 50 { // 경고 문구에 너무 긴 값을 그대로 두지 않는다
			val = string([]rune(val)[:50]) + "…"
		}
		warns = append(warns, fmt.Sprintf("mermaid 그림 자리: %s=%s 값이 틀려 무시함(%s)", name, val, want))
	}
	if w, ok := vals["ratio"]; ok {
		v := w[len("ratio="):]
		if m := diagramRatioRe.FindStringSubmatch(v); m == nil {
			bad("ratio", v, "W:H — 따옴표 없이 정수 1~99")
		} else {
			w, _ := strconv.Atoi(m[1])
			h, _ := strconv.Atoi(m[2])
			if w*10 >= h && h*10 >= w {
				sz.ratioW, sz.ratioH = w, h
			} else {
				bad("ratio", v, "비율 1/10~10")
			}
		}
	}
	for _, name := range []string{"width", "height"} {
		w, ok := vals[name]
		if !ok {
			continue
		}
		v := w[len(name)+1:]
		m := diagramPctRe.FindStringSubmatch(v)
		if m == nil {
			bad(name, v, "따옴표 없이 퍼센트 정수 10~100 — % 필수")
			continue
		}
		n, _ := strconv.Atoi(m[1])
		if name == "width" {
			sz.width = n
		} else {
			sz.height = n
		}
	}
	if sz.ratioW != 0 && sz.width != 0 && sz.height != 0 {
		sz.ratioW, sz.ratioH = 0, 0
		warns = append(warns, "mermaid 그림 자리: ratio·width·height 를 모두 적어 ratio 를 안 씀(width·height 상자를 그대로 씀)")
	}
	return sz, warns
}

// attrs 는 검사한 값을 data-diagram-* 속성으로 쓴다(값이 있는 것만 — 속성이 없으면 출력이 지금과 같다).
func (sz diagramSize) attrs(w util.BufWriter) {
	if sz.ratioW != 0 {
		attr(w, "data-diagram-ratio", strconv.Itoa(sz.ratioW)+":"+strconv.Itoa(sz.ratioH))
	}
	if sz.width != 0 {
		attr(w, "data-diagram-width", strconv.Itoa(sz.width)+"%")
	}
	if sz.height != 0 {
		attr(w, "data-diagram-height", strconv.Itoa(sz.height)+"%")
	}
}
