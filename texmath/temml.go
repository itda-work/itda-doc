package texmath

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
)

// Temml 0.13.5 (MIT). 출처와 해시는 temml/README.md.
//
//go:embed temml/temml.min.js
var temmlSource string

var (
	programOnce sync.Once
	program     *goja.Program
	programErr  error
)

// engine 은 Temml 을 올린 JS 실행기 하나다. 한 번에 한 수식만 돌린다(goja.Runtime 은 동시에 쓸 수 없다).
type engine struct {
	vm     *goja.Runtime
	render goja.Callable
	opts   [2]goja.Value // [inline, display]
}

// 실행기는 처음 쓸 때 만든다(컴파일 약 9ms + 올리기 약 2ms). 수식이 없는 문서는 값을 치르지 않는다.
// 예외를 낸 실행기는 다시 쓰지 않는다 — 상태가 온전한지 알 수 없다.
var pool = sync.Pool{New: func() any {
	e, err := newEngine()
	if err != nil {
		return err
	}
	return e
}}

func newEngine() (*engine, error) {
	programOnce.Do(func() {
		program, programErr = goja.Compile("temml.min.js", temmlSource, false)
	})
	if programErr != nil {
		return nil, programErr
	}
	vm := goja.New()
	if _, err := vm.RunProgram(program); err != nil {
		return nil, err
	}
	temml := vm.Get("temml")
	if temml == nil {
		return nil, errors.New("temml 이 없음")
	}
	render, ok := goja.AssertFunction(temml.ToObject(vm).Get("renderToString"))
	if !ok {
		return nil, errors.New("temml.renderToString 이 함수가 아님")
	}
	e := &engine{vm: vm, render: render}
	for i, display := range []bool{false, true} {
		// macros 를 주지 않는다 — 렌더마다 새 매크로 표라 \gdef 도 그 수식 밖으로 나가지 않는다.
		// trust 거짓: \href·\url·\includegraphics·\html* 를 막는다. maxSize 는 [em, pt] 상한(\rule 등),
		// maxExpand 는 매크로 확장 횟수 상한(재귀 매크로를 멈춘다).
		e.opts[i] = vm.ToValue(map[string]any{
			"displayMode":  display,
			"throwOnError": true,
			"trust":        false,
			"strict":       false,
			"xml":          true,
			"annotate":     false,
			"maxSize":      []any{MaxLengthEm, MaxLengthEm * 10},
			"maxExpand":    MaxExpand,
		})
	}
	return e, nil
}

// temml 은 TeX 를 Temml 로 MathML 문자열로 바꾼다. 오류는 Temml 의 ParseError 문장(첫 줄)이다.
// 한 수식이 renderTimeout 을 넘으면 멈춘다.
func temml(tex string, display bool) (string, error) {
	got := pool.Get()
	e, ok := got.(*engine)
	if !ok {
		return "", &Error{Code: CodeEngine, Msg: fmt.Sprintf("수식 엔진을 올리지 못함: %v", got)}
	}
	timer := time.AfterFunc(renderTimeout, func() { e.vm.Interrupt("timeout") })
	i := 0
	if display {
		i = 1
	}
	v, err := e.render(goja.Undefined(), e.vm.ToValue(tex), e.opts[i])
	// 시간 제한이 이미 울렸으면(또는 울리는 중이면) 이 실행기에 멈춤 표시가 남을 수 있다 — 다시 쓰지 않는다
	reuse := timer.Stop()
	if err != nil {
		var intr *goja.InterruptedError
		if errors.As(err, &intr) {
			return "", &Error{Code: CodeLimit, Msg: "수식이 너무 오래 걸려 멈춤"}
		}
		return "", parseError(err)
	}
	out := v.String()
	if reuse {
		pool.Put(e)
	}
	return out, nil
}

// parseError 는 Temml 의 영어 오류를 고른다. 모르는 명령은 따로 알린다(경고가 명령 이름을 보여 준다).
func parseError(err error) error {
	msg := err.Error()
	var ex *goja.Exception
	if errors.As(err, &ex) {
		if o, ok := ex.Value().(*goja.Object); ok {
			if m := o.Get("message"); m != nil {
				msg = m.String()
			}
		}
	}
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
