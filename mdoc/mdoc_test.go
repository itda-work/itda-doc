package mdoc

import (
	"strings"
	"testing"
)

// render 는 테스트용 줄임이다. 렌더 오류는 곧 실패다.
func render(t *testing.T, src string, opts ...Options) Result {
	t.Helper()
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	r, err := Render([]byte(src), o)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// TestShow 는 MDOC_SHOW 에 담은 원문의 렌더 결과를 찍는다. 눈으로 볼 때만 쓴다:
//
//	MDOC_SHOW="$(cat a.md)" go test ./mdoc -run TestShow -v
func TestShow(t *testing.T) {
	src := getenv("MDOC_SHOW")
	if src == "" {
		t.Skip("MDOC_SHOW 가 비어 있음")
	}
	r := render(t, src)
	t.Log("\n" + r.HTML)
	for _, w := range r.Warnings {
		t.Logf("경고 %d %s %s", w.Line, w.Code, w.Message)
	}
	t.Logf("TOC %+v placed=%v", r.TOC, r.TOCPlaced)
	t.Logf("Meta %+v", r.Meta)
}

func hasWarn(r Result, code string) bool {
	for _, w := range r.Warnings {
		if w.Code == code {
			return true
		}
	}
	return false
}

func contains(t *testing.T, html string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(html, w) {
			t.Errorf("결과에 %q 가 없음\n%s", w, html)
		}
	}
}

func notContains(t *testing.T, html string, bads ...string) {
	t.Helper()
	for _, b := range bads {
		if strings.Contains(html, b) {
			t.Errorf("결과에 %q 가 있으면 안 됨\n%s", b, html)
		}
	}
}
