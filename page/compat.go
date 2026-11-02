package page

import (
	"fmt"
	"maps"
	"math"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// 옛 WebKit 대응(4단계 E-2). Itda Hyve 문서 창은 macOS 12 이상을 지원하고(그 저장소 scripts/macos-app.sh LSMinimumSystemVersion),
// WKWebView 는 시스템 WebKit 이라 Safari 를 올리지 않은 Mac 은 옛 엔진이다. 테마가 쓰는 새 기능 둘:
//
//   - 상대 색 oklch(from var(--accent) …) — Safari 18 부터. 내장 테마는 정적 값(tokens)을 기본으로 두고 식(derived)은
//     @supports 로 덮는다. 정적 값 = 그 테마의 강조색으로 계산한 식의 값(테스트가 맞춰 본다)이라 내장 테마는 어느 엔진에서나
//     같다. 회사 테마는 옛 엔진에서 바탕 테마의 정적 색이 남는다(강조색과 바탕 색이 섞인다 — 안내서).
//   - @scope — Safari 17.4 부터. 모르는 엔진은 @scope 블록을 통째로 버려 테마 CSS 가 사라진다. 화면(JS)이 CSSScopeRule 로
//     판정해 알리면(App.SetScopeSupport) Go 가 내장 테마는 가두지 않은 CSS 로 내고(내장은 우리 코드이고 @scope 는
//     우선순위를 바꾸지 않으니 모양이 같다), 사용자 테마는 그 엔진에서 쓰지 않는다 — 가두지 못하는 남의 CSS 는 받지 않는다.
//     가두지 않은 꼴을 낼지는 Go 가 테마가 내장인지 보고 정한다. 화면은 그것을 고를 수 없다.

// CSSMode 는 엔진에 맞춘 테마 CSS 의 꼴이다.
type CSSMode struct {
	Unscoped       bool // @scope 를 모르는 엔진: 내장 테마를 가두지 않고 낸다
	FakeNoRelColor bool // 시험: 상대 색을 모르는 엔진처럼 파생 토큰을 끈다
}

const (
	// 상대 색과 채널 calc() 를 함께 가린다 — 상대 색을 일부만 아는 엔진(채널 calc 를 모름)도 정적 값으로 가게
	relColorSupports  = "(color: oklch(from red calc(l + 0.1) calc(c * 0.5) h))"
	fakeFalseSupports = "(color: hyve-old-webkit(0))"
)

// oldWebKitEnv 는 옛 WebKit 흉내 내기다(시험·검사용 숨은 환경변수). "scope"·"relcolor" 를 쉼표로, 또는 "all".
// 제품 동작에는 이 변수를 줄 때만 영향이 있다.
const oldWebKitEnv = "ITDA_HYVE_DOCVIEW_OLD_WEBKIT"

// OldWebKit 은 흉내 내는 옛 WebKit 의 모자란 기능이다(NoScope: @scope 없음, NoRelColor: 상대 색 없음).
type OldWebKit struct{ NoScope, NoRelColor bool }

// FakeOldWebKit 은 이 실행의 옛 WebKit 흉내 값이다(oldWebKitEnv).
func FakeOldWebKit() OldWebKit { return parseOldWebKit(runtime.GOOS, os.Getenv(oldWebKitEnv)) }

// parseOldWebKit 은 흉내 변수를 읽는다. 옛 WebKit 은 macOS 의 일이다 — Windows(WebView2, Chromium)에서는 뜻이 없어 무시한다.
func parseOldWebKit(goos, env string) OldWebKit {
	var o OldWebKit
	if goos != "darwin" {
		return o
	}
	for f := range strings.SplitSeq(env, ",") {
		switch strings.TrimSpace(f) {
		case "scope":
			o.NoScope = true
		case "relcolor":
			o.NoRelColor = true
		case "all":
			o.NoScope, o.NoRelColor = true, true
		}
	}
	return o
}

// NoScopeReason 은 @scope 를 모르는 엔진에서 사용자 테마를 쓰지 않는 까닭이다(목록에 보인다).
const NoScopeReason = "이 macOS 의 WebKit 이 오래돼(@scope 없음) 회사 테마를 쓸 수 없음 — macOS 또는 Safari 를 올리면 된다(Safari 17.4 이상)"

// WithoutUser 는 사용자 테마를 모두 "쓸 수 없음" 으로 돌린 묶음이다(@scope 를 모르는 엔진). 내장 테마는 그대로다.
func (ts *Themes) WithoutUser(why string) *Themes {
	out := &Themes{byName: map[string]*Theme{}, alias: map[string]string{}, broken: map[string]string{}, Rev: ts.Rev}
	maps.Copy(out.broken, ts.broken)
	for _, n := range ts.order {
		t := ts.byName[n]
		if t.User {
			out.broken[n] = why
			continue
		}
		out.byName[n] = t
		out.order = append(out.order, n)
		for _, a := range t.Aliases {
			out.alias[a] = n
		}
	}
	return out
}

// ---- 상대 색 계산(CSS Color 4 의 OKLab, sRGB 로 자르기). WebKit 이 그린 값과 같다(내장 8개 실측).
// "회사 테마 만들기" 가 바탕 테마의 파생 식을 새 강조색으로 미리 계산해 정적 값으로 적을 때와, 내장 테마의 정적 값이
// 식의 값과 같은지 보는 테스트가 쓴다.

var relColorRe = regexp.MustCompile(`^oklch\(from var\((--[a-z0-9-]+)\) (l|[\d.]+|calc\(l ([+-]) ([\d.]+)\)) (c|calc\(c \* ([\d.]+)\)) h\)$`)

// evalRelColor 는 내장 테마가 쓰는 꼴(oklch(from var(--x) L C h), L = l·수·calc(l ± n), C = c·calc(c * n))만 계산한다.
func evalRelColor(expr string, vals map[string]string) (string, error) {
	m := relColorRe.FindStringSubmatch(expr)
	if m == nil {
		return "", fmt.Errorf("계산하지 못하는 식 %q", expr)
	}
	base := vals[m[1]]
	if !accentRe.MatchString(base) {
		return "", fmt.Errorf("%s 의 정적 값 %q 가 #rrggbb 가 아님", m[1], base)
	}
	L, C, H := hexToOKLCH(base)
	switch {
	case m[2] == "l":
	case m[3] != "":
		d, _ := strconv.ParseFloat(m[4], 64)
		if m[3] == "+" {
			L += d
		} else {
			L -= d
		}
	default:
		L, _ = strconv.ParseFloat(m[2], 64)
	}
	if m[6] != "" {
		k, _ := strconv.ParseFloat(m[6], 64)
		C *= k
	}
	return oklchToHex(L, C, H), nil
}

func srgbToLinear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func linearToSRGB(c float64) float64 {
	c = math.Max(0, math.Min(1, c))
	if c <= 0.0031308 {
		return 12.92 * c
	}
	return 1.055*math.Pow(c, 1/2.4) - 0.055
}

func hexToOKLCH(h string) (L, C, H float64) {
	v, _ := strconv.ParseUint(h[1:], 16, 32)
	r, g, b := srgbToLinear(float64(v>>16&255)/255), srgbToLinear(float64(v>>8&255)/255), srgbToLinear(float64(v&255)/255)
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	L = 0.2104542553*l + 0.7936177850*m - 0.0040720468*s
	A := 1.9779984951*l - 2.4285922050*m + 0.4505937099*s
	B := 0.0259040371*l + 0.7827717662*m - 0.8086757660*s
	return L, math.Hypot(A, B), math.Atan2(B, A)
}

func oklchToHex(L, C, H float64) string {
	A, B := C*math.Cos(H), C*math.Sin(H)
	l := math.Pow(L+0.3963377774*A+0.2158037573*B, 3)
	m := math.Pow(L-0.1055613458*A-0.0638541728*B, 3)
	s := math.Pow(L-0.0894841775*A-1.2914855480*B, 3)
	r := 4.0767416621*l - 3.3077115913*m + 0.2309699292*s
	g := -1.2684380046*l + 2.6097574011*m - 0.3413193965*s
	b := -0.0041960863*l - 0.7034186147*m + 1.7076147010*s
	ch := func(x float64) int { return int(math.Round(linearToSRGB(x) * 255)) }
	return fmt.Sprintf("#%02x%02x%02x", ch(r), ch(g), ch(b))
}
