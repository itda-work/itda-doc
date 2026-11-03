//go:build js && wasm

package texmath

import (
	"fmt"
	"syscall/js"
)

// js/wasm 판의 엔진: goja 를 싣지 않고(크기 — itda-work/itda-hyve#577 §2·D8) 페이지가 올린 Temml 을 직접 부른다 —
// globalThis.temml.renderToString(tex, temmlOptions(display)). temml.min.js 를 내장하지 않는다(go:embed 는 temml.go 에만 있다).
// 웹 번들이 texmath/temml/temml.min.js 를 따로 싣는다(scripts/build-wasm.sh 가 같은 파일을 복사한다 — 판이 어긋나지 않게).
//
// 결과는 네이티브 판과 똑같이 texmath.go render 에서 check(앞)·MaxOutput·sanitize(뒤)를 지난다. 옵션과 오류 고르기는
// temmlcommon.go 의 같은 함수다.
//
// 네이티브 판과 다른 점(축소):
//   - 한 수식의 시간 상한(renderTimeout)이 없다. 동기 JS 호출은 중간에 끊을 수 없다. 남는 상한은 check.go 의 사전 검사
//     (길이·중첩·막는 명령), Temml 의 maxExpand(매크로 확장 횟수)·maxSize, 결과 크기(MaxOutput)다.
//   - Temml 이 예외를 던진 뒤에도 같은 전역 Temml 을 계속 부른다. 네이티브 판은 예외를 낸 실행기를 버린다(temml.go pool).
//     Temml 이 호출 사이에 상태를 남기지 않는다는 것은 추정이다(던진 뒤 다음 수식이 정상으로 그려지는 것은 node 로 확인).
//
// globalThis.temml 이 없으면(페이지가 temml.min.js 를 올리지 않음) CodeEngine 오류다 — 화면에는 수식이 원문 + 경고로 남고,
// 기억(cache)하지 않으므로 Temml 을 뒤에 올리면 다음 렌더부터 그려진다. 부를 때마다 찾는다.
//
// globalThis.temml 과 그 renderToString 은 평범한 값이어야 한다. 읽기(getter·Proxy)에서 던지면 syscall/js 의 Get 이 잡지 않아
// (wasm_exec.js valueGet 에는 try/catch 가 없다 — valueCall·valueInvoke 에만 있다) Go panic 이 아니라 JS 예외가 wasm 을 풀며
// 나가고, 아래 recover 도 돌지 않아 itdaDoc.render 호출자에게 JS 예외가 던져진다.
func temml(tex string, display bool) (out string, err error) {
	// renderToString 이 던지면 syscall/js 의 Invoke 는 js.Error 로 panic 한다 — 받아서 네이티브 판의 parseError 와 같은 문장
	// 고르기로 돌린다(jsError). 받지 않으면 panic 이 mdoc 을 뚫고 나가 문서 전체가 오류가 된다. 속성 읽기(Get)가 던지는 것은
	// panic 이 아니라 이 recover 로 받지 못한다(위 주석).
	defer func() {
		if r := recover(); r != nil {
			out, err = "", jsError(r)
		}
	}()
	t := js.Global().Get("temml")
	if t.Type() != js.TypeObject && t.Type() != js.TypeFunction {
		return "", &Error{Code: CodeEngine, Msg: "수식 엔진을 올리지 못함: globalThis.temml 이 없음"}
	}
	f := t.Get("renderToString")
	if f.Type() != js.TypeFunction {
		return "", &Error{Code: CodeEngine, Msg: "수식 엔진을 올리지 못함: temml.renderToString 이 함수가 아님"}
	}
	v := f.Invoke(tex, temmlOptions(display))
	if v.Type() != js.TypeString {
		return "", &Error{Code: CodeEngine, Msg: "수식 엔진을 올리지 못함: renderToString 이 글자를 돌려주지 않음"}
	}
	return v.String(), nil
}

// jsError 는 Temml 이 던진 값에서 문장을 꺼낸다.
//   - 글자를 던지면 그 글을, message 가 글자인 객체(Error)를 던지면 message 를 classifyError 로 고른다 — 네이티브 판(temml.go
//     parseError)과 같다. 결과는 기억(cache)된다. 한계: Temml 이 아닌 받침이 Error 를 던져도(예 「Temml loading」) 같은 길이라
//     CodeSyntax 로 기억되어, 진짜 Temml 로 바꾼 뒤에도 같은 수식은 그 경고로 남는다(Temml 0.13.5 자신은 이렇게 던지지 않는다 — 추정).
//   - 그 밖의 값(숫자·undefined·null·message 가 없는 객체 등)은 CodeEngine 이다 — 기억하지 않아 엔진이 고쳐지면 다음 렌더에 그려진다.
//   - 여기서 값을 읽다 panic 하면 한 겹 더 받아 CodeEngine 으로 둔다(지금 부르는 읽기는 panic 하지 않는다 — 앞으로 읽기를 더할 때를 위한 방어). 다만 message 가 던지는 getter 면 Get 이 잡지
//     않아 JS 예외로 나간다(temml 머리 주석과 같은 한계).
func jsError(r any) (err error) {
	je, ok := r.(js.Error)
	if !ok {
		return &Error{Code: CodeEngine, Msg: fmt.Sprintf("수식 엔진 오류: %v", r)}
	}
	defer func() {
		if r2 := recover(); r2 != nil {
			err = &Error{Code: CodeEngine, Msg: "수식 엔진 오류: 던진 값을 읽지 못함"}
		}
	}()
	// je.Error() 는 쓰지 않는다 — 안에서 Value.Get("message") 를 불러 던진 값이 객체가 아니면 panic 한다
	switch je.Value.Type() {
	case js.TypeString:
		return classifyError(je.Value.String())
	case js.TypeObject:
		if m := je.Value.Get("message"); m.Type() == js.TypeString {
			return classifyError(m.String())
		}
	}
	return &Error{Code: CodeEngine, Msg: "수식 엔진 오류: Error 도 글자도 아닌 값(" + je.Value.Type().String() + ")을 던짐"}
}
