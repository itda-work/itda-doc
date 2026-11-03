//go:build !js

package texmath

import (
	_ "embed"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dop251/goja"
)

// 네이티브 판의 엔진: Temml 을 순수 Go JS 실행기 goja 에서 돌린다. js/wasm 판은 temml_js.go — 브라우저의 Temml 을 직접 부른다.
// 두 판이 같은 옵션(temmlOptions)과 같은 오류 고르기(classifyError)를 쓴다 — temmlcommon.go.

// renderTimeout 은 수식 하나의 시간 상한이다. goja 를 멈출 수 있는 이 판에만 있다(js 판은 동기 JS 호출이라 끊지 못한다).
const renderTimeout = 2 * time.Second

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
		e.opts[i] = vm.ToValue(temmlOptions(display))
	}
	return e, nil
}

// temml 은 TeX 를 Temml 로 MathML 문자열로 바꾼다. 오류는 Temml 의 ParseError 문장(첫 줄)이다.
// 한 수식이 renderTimeout 을 넘으면 멈춘다(goja Interrupt).
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

// parseError 는 goja 가 돌려준 오류에서 Temml 의 문장을 꺼내 고른다(classifyError).
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
	return classifyError(msg)
}
