package texmath

import (
	"fmt"
	"strings"
)

// 두 엔진 판(temml.go — goja, temml_js.go — 브라우저 JS)이 함께 쓰는 것. 옵션과 오류 고르기를 여기 한 곳에 둔다 —
// 판마다 따로 적으면 같은 수식이 판에 따라 다르게 그려지거나 다른 경고가 된다(동일성 시험: cmd/itda-doc-wasm/wasm_test.go).

// temmlOptions 는 Temml renderToString 의 옵션이다.
// macros 를 주지 않는다 — 렌더마다 새 매크로 표라 \gdef 도 그 수식 밖으로 나가지 않는다.
// trust 거짓: \href·\url·\includegraphics·\html* 를 막는다. maxSize 는 [em, pt] 상한(\rule 등),
// maxExpand 는 매크로 확장 횟수 상한(재귀 매크로를 멈춘다).
func temmlOptions(display bool) map[string]any {
	return map[string]any{
		"displayMode":  display,
		"throwOnError": true,
		"trust":        false,
		"strict":       false,
		"xml":          true,
		"annotate":     false,
		"maxSize":      []any{MaxLengthEm, MaxLengthEm * 10},
		"maxExpand":    MaxExpand,
	}
}

// internalMsg 는 Temml 이 ParseError 가 아닌 것을 던졌을 때의 경고 한 문장이다 — Temml 안의 TypeError(`$x^$`·
// `\operatorname{__proto__}`), Temml 자신의 단언 `new Error(…)`(name 이 "Error" — temml.min.js 의 「Expected node of type …」 등.
// 어느 입력이 닿는지는 찾지 않았다), Temml 이 아닌 받침의 Error, Error 가 아닌 값(글자·숫자 등) 모두. 거꾸로 name 만 "ParseError" 인
// 가짜(Temml 이 아닌 받침 — js 판에서만 생길 수 있다)는 문법 오류로 골라 기억된다. 그 문장은 JS 엔진(goja·V8)이
// 지어 판마다 다르므로 내보이지 않는다(itda-work/itda-hyve#577 결정 1). CodeEngine 이라 기억(cache)하지 않는다(texmath.go Render) —
// 엔진을 바꾸면(진짜 Temml 을 올리면) 다음 렌더에 다시 부른다.
const internalMsg = "수식 엔진 내부 오류(Temml 이 문법 오류가 아닌 예외를 냄)"

// classifyThrown 은 Temml 이 던진 값을 고른다. parse 는 그 값이 Temml 의 ParseError 인지(name 이 "ParseError" 이고 message 가
// 글자인 객체), msg 는 그 message 다. 두 판(temml.go parseError, temml_js.go jsError)이 이 함수 하나로 고른다.
func classifyThrown(parse bool, msg string) error {
	if !parse {
		return &Error{Code: CodeEngine, Msg: internalMsg}
	}
	return classifyError(msg)
}

// classifyError 는 Temml 의 영어 오류 문장(ParseError 의 message)을 고른다. 모르는 명령은 따로 알린다(경고가 명령 이름을 보여 준다).
func classifyError(msg string) error {
	msg = strings.TrimSpace(strings.SplitN(msg, "\n", 2)[0])
	msg = strings.TrimPrefix(msg, "ParseError:")
	msg = strings.TrimSpace(msg)
	for _, p := range []string{"Unsupported function name: ", "Undefined control sequence: "} {
		if rest, ok := strings.CutPrefix(msg, p); ok {
			name, _, _ := strings.Cut(rest, " ")
			return &Error{Code: CodeUnsupported, Msg: "지원하지 않는 명령 " + name, Command: name}
		}
	}
	if rest, ok := strings.CutPrefix(msg, "No such environment: "); ok {
		name, _, _ := strings.Cut(rest, " ")
		return &Error{Code: CodeUnsupported, Msg: "지원하지 않는 환경 " + name, Command: name}
	}
	if env, ok := strings.CutSuffix(msg, " can be used only in display mode."); ok {
		return &Error{Code: CodeSyntax, Msg: "환경 " + strings.TrimSuffix(strings.Trim(env, "{}"), "*") + " 은 $$…$$ 블록 수식에서만 쓴다"}
	}
	if strings.HasPrefix(msg, "Too many expansions") {
		return &Error{Code: CodeLimit, Msg: fmt.Sprintf("매크로 확장이 %d 번을 넘음(재귀 매크로?)", MaxExpand)}
	}
	return &Error{Code: CodeSyntax, Msg: "수식 문법 오류: " + clip(msg, 120)}
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
