package texmath

import (
	"encoding/xml"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// 결과에 남기는 MathML 요소. MathML Core 의 표시 요소 가운데 Temml 이 내는 것이다.
// semantics·annotation 은 내지 않는다(annotate 끔). merror 는 throwOnError 라 나오지 않는다.
var elements = map[string]bool{
	"math": true, "mrow": true, "mi": true, "mn": true, "mo": true, "mtext": true, "ms": true, "mspace": true,
	"mfrac": true, "msqrt": true, "mroot": true, "msub": true, "msup": true, "msubsup": true,
	"munder": true, "mover": true, "munderover": true, "mmultiscripts": true, "mprescripts": true, "none": true,
	"mtable": true, "mtr": true, "mtd": true, "mstyle": true, "mpadded": true, "mphantom": true, "menclose": true,
}

// 글자를 담는 요소. 다른 요소 안의 글자는 공백만 허용한다.
var tokenElements = map[string]bool{"mi": true, "mn": true, "mo": true, "mtext": true, "ms": true}

type validator func(string) bool

var (
	lengthRe   = regexp.MustCompile(`^[+-]?(\d+(?:\.\d*)?|\.\d+)(em|ex|pt|px|mu)?$`)
	tokensRe   = regexp.MustCompile(`^[a-z]+(?: [a-z]+)*$`)
	variantRe  = regexp.MustCompile(`^[a-z]+(?:-[a-z]+)*$`)
	scriptRe   = regexp.MustCompile(`^[+-]?[0-7]$`)
	notationOK = setOf("top bottom left right box roundedbox circle updiagonalstrike downdiagonalstrike horizontalstrike verticalstrike longdiv actuarial madruwb phasorangle updiagonalarrow")
	alignOK    = setOf("left center right top bottom baseline axis")
)

func setOf(s string) map[string]bool {
	m := map[string]bool{}
	for _, f := range strings.Fields(s) {
		m[f] = true
	}
	return m
}

func oneOf(vals ...string) validator {
	m := setOf(strings.Join(vals, " "))
	return func(v string) bool { return m[v] }
}

func tokensIn(ok map[string]bool) validator {
	return func(v string) bool {
		if !tokensRe.MatchString(v) {
			return false
		}
		for _, f := range strings.Fields(v) {
			if !ok[f] {
				return false
			}
		}
		return true
	}
}

// length 는 길이 값이다. 절댓값이 MaxLengthEm 을 넘으면 받지 않는다(간격으로 쪽을 밀어내지 못하게).
func length(v string) bool {
	m := lengthRe.FindStringSubmatch(v)
	if m == nil {
		return false
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return false
	}
	em := n
	switch m[2] {
	case "pt":
		em = n / 10
	case "px":
		em = n / 13.33
	case "ex":
		em = n / 2
	case "mu":
		em = n / 18
	case "":
		if n != 0 {
			return false // 단위 없는 길이는 0 만
		}
	}
	return math.Abs(em) <= MaxLengthEm
}

var (
	boolean = oneOf("true", "false")
	// 요소마다 허용하는 속성과 값의 꼴. "*" 는 모든 요소. class 와 style 은 따로 다룬다.
	attrRules = map[string]map[string]validator{
		"*": {
			"mathvariant":  variantRe.MatchString,
			"displaystyle": boolean,
			"scriptlevel":  scriptRe.MatchString,
		},
		"math": {"display": oneOf("block")},
		"mo": {
			"fence": boolean, "separator": boolean, "stretchy": boolean, "symmetric": boolean, "largeop": boolean,
			"movablelimits": boolean, "accent": boolean, "form": oneOf("prefix", "infix", "postfix"),
			"lspace": length, "rspace": length, "minsize": length, "maxsize": length,
		},
		"mover":      {"accent": boolean},
		"munder":     {"accentunder": boolean},
		"munderover": {"accent": boolean, "accentunder": boolean},
		"mfrac":      {"linethickness": length},
		"mspace":     {"width": length, "height": length, "depth": length},
		"mpadded":    {"width": length, "height": length, "depth": length, "lspace": length, "voffset": length},
		"menclose":   {"notation": tokensIn(notationOK)},
		"mtable":     {"columnalign": tokensIn(alignOK), "rowalign": tokensIn(alignOK)},
		"mtr":        {"columnalign": tokensIn(alignOK), "rowalign": tokensIn(alignOK)},
		"mtd":        {"columnalign": tokensIn(alignOK), "rowalign": tokensIn(alignOK)},
	}
)

// Temml 의 class. 테마(base.css 의 수식 절)가 아는 것만 남기고 나머지 낱말은 버린다 —
// 문서의 다른 class(page-break 등)와 겹치는 이름이 수식에서 나오지 않게.
var classOK = setOf(`tml-display tml-jot tml-small tml-left tml-right tml-sml-pad tml-med-pad tml-lrg-pad tml-prime
	tml-vec tml-overline tml-underline tml-cancel upstrike downstrike sout tml-xcancel
	tml-hat-1 tml-hat-2 tml-hat-3 tml-hat-4 tml-tilde-2 tml-tilde-3 tml-tilde-4
	chr-sml chr-med chr-lrg chr-sml-vec chr-med-vec chr-lrg-vec
	wbk-acc wbk-sml wbk-sml-acc wbk-sml-vec wbk-med wbk-med-acc wbk-med-vec wbk-lrg wbk-lrg-acc wbk-lrg-vec
	mathcal mathscr`)

// Temml 이 style 로 적는 배치를 class 로 옮긴다(CSP 가 style 속성을 막는다 — 설계 1.9). 값이 정해진 것만 옮기고,
// 값이 표에 없는 안쪽 여백은 버린다(브라우저 기본 여백). 그 밖의 선언이 하나라도 있으면 수식을 거부한다.
var padCodes = map[string]string{"0": "0", "0em": "0", "0pt": "0", "0px": "0", "0.1389em": "t", "0.25em": "q", "0.4em": "d", "0.5em": "h", "1em": "1", "5.9776pt": "a"}

var marginCodes = map[string]string{"-0.1667em": "1667", "-0.125em": "125", "-0.15em": "15", "-0.35em": "35"}

var borderCodes = map[string]string{"0.06em solid": "", "0.06em dashed": "-d", "0.15em double": "-2"}

// styleClasses 는 style 값 하나를 class 낱말들로 바꾼다. 옮기지 못하는 선언이면 ok 가 거짓이다.
func styleClasses(elem, style string) (classes []string, ok bool) {
	for _, decl := range strings.Split(style, ";") {
		prop, val, found := strings.Cut(decl, ":")
		prop, val = strings.TrimSpace(prop), strings.Join(strings.Fields(val), " ")
		if !found {
			if prop == "" {
				continue
			}
			return nil, false
		}
		switch {
		case elem == "math" && prop == "display" && val == "block math":
			// Chromium 이 display="block" 을 쓰게 하는 선언이다 — base.css 가 math[display=block] 에 같은 것을 준다
		case prop == "math-depth" && len(val) == 1 && val[0] >= '0' && val[0] <= '7':
			classes = append(classes, "tml-md"+val) // 악센트(0)·겹친 분수의 첨자 단계
		case elem == "mtd" && strings.HasPrefix(prop, "padding-"):
			side := map[string]string{"padding-left": "l", "padding-right": "r", "padding-top": "t", "padding-bottom": "b"}[prop]
			if side == "" {
				return nil, false
			}
			if code, known := padCodes[val]; known {
				classes = append(classes, "tml-p"+side+"-"+code)
			}
		case elem == "mtd" && strings.HasPrefix(prop, "border-"):
			side := map[string]string{"border-left": "l", "border-right": "r", "border-top": "t", "border-bottom": "b"}[prop]
			code, known := borderCodes[val]
			if side == "" || !known {
				return nil, false
			}
			classes = append(classes, "tml-b"+side+code)
		case prop == "text-align" && (val == "left" || val == "right" || val == "center"):
			classes = append(classes, "tml-"+val)
		case elem == "mrow" && prop == "margin-left" && marginCodes[val] != "":
			classes = append(classes, "tml-ml-"+marginCodes[val]) // \! 와 \TeX 의 글자 겹침
		case elem == "mtable" && prop == "width" && val == "100%":
			classes = append(classes, "tml-full") // multline
		case elem == "mrow" && (prop == "padding" && val == "3pt" || prop == "border" && val == "1px solid"):
			classes = append(classes, "tml-fbox") // \boxed
		default:
			return nil, false
		}
	}
	return classes, true
}

const maxNesting = 200

// sanitize 는 Temml 의 XML 결과를 다시 읽어 허용 목록대로 다시 쓴다. 속성은 이름 차례로 쓴다.
func sanitize(raw string, display bool) (string, error) {
	reject := func(what string) (string, error) {
		return "", &Error{Code: CodeOutput, Msg: "이 수식은 그릴 수 없음(" + what + ")"}
	}
	dec := xml.NewDecoder(strings.NewReader(raw))
	dec.Strict = true
	dec.Entity = xml.HTMLEntity
	var b strings.Builder
	var stack []string
	roots := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return reject("XML 아님")
		}
		switch t := tok.(type) {
		case xml.StartElement:
			name := t.Name.Local
			if !elements[name] || (t.Name.Space != "" && t.Name.Space != "http://www.w3.org/1998/Math/MathML") {
				return reject("<" + name + ">")
			}
			if (len(stack) == 0) != (name == "math") {
				return reject("<math> 밖")
			}
			if len(stack) == 0 {
				roots++
				if roots > 1 {
					return reject("<math> 둘")
				}
			}
			if len(stack) >= maxNesting {
				return "", &Error{Code: CodeLimit, Msg: "수식의 중첩이 너무 깊음"}
			}
			attrs := map[string]string{}
			var classes []string
			for _, a := range t.Attr {
				k := a.Name.Local
				if a.Name.Space == "xmlns" || (a.Name.Space == "" && k == "xmlns") {
					continue // 이름공간 선언. HTML 은 <math> 로 이름공간을 정한다
				}
				if a.Name.Space != "" {
					return reject(a.Name.Space + ":" + k)
				}
				switch k {
				case "class":
					for _, c := range strings.Fields(a.Value) {
						if classOK[c] {
							classes = append(classes, c)
						}
					}
					continue
				case "style":
					cs, ok := styleClasses(name, a.Value)
					if !ok {
						return reject("style " + a.Value)
					}
					classes = append(classes, cs...)
					continue
				}
				v, ok := attrRules[name][k]
				if !ok {
					v, ok = attrRules["*"][k]
				}
				if !ok {
					return reject(name + "@" + k)
				}
				if !v(a.Value) {
					if lengthRe.MatchString(a.Value) {
						return "", &Error{Code: CodeLimit, Msg: "수식의 간격·크기가 상한(" + strconv.Itoa(MaxLengthEm) + "em)을 넘음"}
					}
					return reject(name + "@" + k + "=" + a.Value)
				}
				attrs[k] = a.Value
			}
			if name == "math" {
				// display 는 렌더 꼴로 정한다(Temml 의 것을 믿지 않는다). class tml-display 는 테마가 블록 수식에 쓴다
				delete(attrs, "display")
				classes = filterOut(classes, "tml-display")
				if display {
					attrs["display"] = "block"
					classes = append(classes, "tml-display")
				}
			}
			if cs := uniq(classes); len(cs) > 0 {
				attrs["class"] = strings.Join(cs, " ")
			}
			b.WriteString("<" + name)
			keys := make([]string, 0, len(attrs))
			for k := range attrs {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				b.WriteString(" " + k + `="`)
				xmlEscape(&b, attrs[k])
				b.WriteByte('"')
			}
			b.WriteByte('>')
			stack = append(stack, name)
		case xml.EndElement:
			name := stack[len(stack)-1] // Strict 디코더가 짝을 맞춘다
			stack = stack[:len(stack)-1]
			b.WriteString("</" + name + ">")
		case xml.CharData:
			if len(stack) == 0 || !tokenElements[stack[len(stack)-1]] {
				if strings.TrimSpace(string(t)) != "" {
					return reject("글자 위치")
				}
				continue
			}
			xmlEscape(&b, string(t))
		default:
			// 주석·처리 명령·선언
			return reject("XML 지시")
		}
	}
	if roots != 1 || len(stack) != 0 {
		return reject("<math> 없음")
	}
	return b.String(), nil
}

func xmlEscape(b *strings.Builder, s string) {
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&#39;")
		default:
			b.WriteRune(r)
		}
	}
}

func uniq(s []string) []string {
	sort.Strings(s)
	out := s[:0]
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}

func filterOut(s []string, drop string) []string {
	out := s[:0]
	for _, v := range s {
		if v != drop {
			out = append(out, v)
		}
	}
	return out
}
