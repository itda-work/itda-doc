// Package texmath 는 TeX 수식 하나를 MathML 로 바꾼다(설계 1.10).
//
// 변환은 Temml(temml/README.md)이 한다. 네이티브 판은 내장한 Temml 을 순수 Go JS 실행기 goja 에서 돌린다 — cgo·외부 프로그램·
// 네트워크 없음(temml.go). js/wasm 판은 goja 를 싣지 않고 페이지가 올린 globalThis.temml 을 직접 부른다(temml_js.go). 두 판의
// 옵션·오류 고르기는 temmlcommon.go 한 곳이다.
// Temml 의 결과는 그대로 내지 않는다. 앞뒤로 이 패키지가 지킨다:
//
//   - 앞(check.go): 길이·중첩 상한, 문서에 모양을 적는 명령(색·크기·HTML·주소·식 번호)을 막는다.
//   - 뒤(sanitize.go): 결과 XML 을 다시 읽어 허용 목록의 요소·속성·값만 정해진 순서로 다시 쓴다.
//     style 은 내지 않는다(문서 창의 CSP 가 막는다 — 설계 1.9) — Temml 이 style 로 적는 배치는 정해진 class 로 바꾸고,
//     모르는 요소·속성·style 이 하나라도 있으면 그 수식을 그리지 않는다(원문 + 경고).
//
// 같은 입력은 같은 출력이다. 결과를 기억해 두므로(cache) 편집 중 다시 렌더할 때 바뀐 수식만 변환한다.
// 수식마다 Temml 의 매크로 표가 새로 만들어진다 — \def·\newcommand 는 그 수식 안에서만 산다(문서 사이·수식 사이 격리).
package texmath

import (
	"fmt"
	"sync"
)

// 상한. 넘으면 그 수식을 그리지 않고 CodeLimit 오류다.
const (
	MaxInputBytes = 4000 // 수식 하나의 TeX 길이(바이트). 큰 행렬도 1~2KB 다
	MaxDepth      = 32   // { … }·\begin…\end·\left…\right 의 중첩
	MaxExpand     = 500  // 매크로 확장 횟수(Temml maxExpand)
	MaxLengthEm   = 10   // 길이 값(간격·\rule 등)의 절댓값 상한(em). 결과의 길이 속성도 이것으로 본다
	MaxOutput     = 256 << 10
	// 시간 상한(renderTimeout)은 네이티브 판에만 있다 — temml.go. js 판은 끊지 못한다(temml_js.go).
)

// 오류 코드.
const (
	CodeUnsupported = "unsupported" // Temml 이 모르는 명령·환경
	CodeBlocked     = "blocked"     // 알지만 막은 명령(색·크기·HTML·주소·식 번호·배치)
	CodeSyntax      = "syntax"      // 괄호가 맞지 않는 등 TeX 문법 오류
	CodeLimit       = "limit"       // 길이·중첩·확장·시간·크기 상한
	CodeOutput      = "output"      // 결과에 허용 목록 밖의 요소·속성·style 이 있음
	CodeEngine      = "engine"      // 엔진을 올리지 못함(내부 오류)
)

// Error 는 수식을 그리지 못한 까닭이다. Msg 는 사람에게 보이는 한국어 문장이다.
type Error struct {
	Code    string
	Msg     string
	Command string // 걸린 명령(\color 등). 모르면 ""
}

func (e *Error) Error() string { return e.Msg }

type key struct {
	tex     string
	display bool
}

type result struct {
	html string
	err  error
}

// 결과 기억. 넘치면 통째로 비운다(순서 관리 없이 메모리만 묶는다). 수식 수와 글자 수 둘로 묶는다 —
// 수식 하나의 결과가 MaxOutput 까지 클 수 있다.
const (
	cacheMax      = 4096
	cacheMaxBytes = 16 << 20
)

var (
	cacheMu    sync.Mutex
	cache      = map[key]result{}
	cacheBytes int
)

// Render 는 TeX 수식(구분자 $ 없이)을 <math> 하나로 바꾼다. display 면 <math display="block">.
// 오류는 *Error 다 — 화면은 원문을 글자로 두고 Msg 를 경고로 보인다.
func Render(tex string, display bool) (string, error) {
	if len(tex) > MaxInputBytes {
		// 기억하기 전에 거른다 — 긴 원문을 열쇠로 쥐지 않는다
		return "", &Error{Code: CodeLimit, Msg: fmt.Sprintf("수식이 너무 김(%d 바이트, 상한 %d)", len(tex), MaxInputBytes)}
	}
	k := key{tex, display}
	cacheMu.Lock()
	r, ok := cache[k]
	cacheMu.Unlock()
	if ok {
		return r.html, r.err
	}
	html, err := render(tex, display)
	if e, ok := err.(*Error); ok && e.Code == CodeEngine {
		return html, err // 내부 오류는 기억하지 않는다
	}
	cacheMu.Lock()
	if len(cache) >= cacheMax || cacheBytes+len(tex)+len(html) > cacheMaxBytes {
		cache, cacheBytes = map[key]result{}, 0
	}
	cache[k] = result{html, err}
	cacheBytes += len(tex) + len(html)
	cacheMu.Unlock()
	return html, err
}

func render(tex string, display bool) (string, error) {
	src, err := check(tex)
	if err != nil {
		return "", err
	}
	raw, err := temml(src, display)
	if err != nil {
		return "", err
	}
	if len(raw) > MaxOutput {
		return "", &Error{Code: CodeLimit, Msg: "수식의 결과가 너무 큼"}
	}
	return sanitize(raw, display)
}

// resetCache 는 기억을 비운다(시험용).
func resetCache() {
	cacheMu.Lock()
	cache, cacheBytes = map[key]result{}, 0
	cacheMu.Unlock()
}
