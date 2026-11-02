package mdoc

import (
	"strings"
	"testing"
)

// 문서 창은 상대 경로 그림을 앱 화면과 다른 주소 공간(/_doc/)으로 받는다. data:·원격·막은 주소에는 붙지 않는다.
func TestLocalImagePrefix(t *testing.T) {
	src := "![a](images/org.png)\n\n![b](data:image/png;base64,AAAA)\n\n![c](https://example.com/x.png)\n\n![d](javascript:alert(1))\n\n![e](../up.png \"캡션\")\n"
	r := render(t, src, Options{LocalImagePrefix: "/_doc/"})
	for _, want := range []string{
		`<img src="/_doc/images/org.png"`,
		`<img src="data:image/png;base64,AAAA"`,
		`<img data-remote-src="https://example.com/x.png"`,
		`<img src="/_doc/../up.png"`, // 폴더 탈출은 문서 창의 핸들러가 거부한다(렌더러는 주소 공간만 나눈다)
	} {
		if !strings.Contains(r.HTML, want) {
			t.Errorf("없음: %s\n%s", want, r.HTML)
		}
	}
	if strings.Contains(r.HTML, "/_doc/javascript") || strings.Contains(r.HTML, "/_doc/https") || strings.Contains(r.HTML, "/_doc/data") {
		t.Errorf("상대 경로가 아닌 주소에 접두가 붙음:\n%s", r.HTML)
	}
	// 기본(접두 없음)은 원문 그대로 — golden 과 같다
	if r0 := render(t, src); !strings.Contains(r0.HTML, `<img src="images/org.png"`) {
		t.Errorf("기본 동작이 바뀜:\n%s", r0.HTML)
	}
}
