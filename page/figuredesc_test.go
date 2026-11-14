package page

import (
	"testing"

	"github.com/itda-work/itda-doc/mdoc"
)

// RenderViewOpts 로 게시용 검사(mdoc.Options.RequireFigureDesc)를 넘기면 View.Warnings 에 figure-no-desc 가 실린다(itda-work/itda-hyve#845).
func TestRenderViewOptsFigureNoDesc(t *testing.T) {
	ts, err := BuiltinThemes()
	if err != nil {
		t.Fatal(err)
	}
	src := []byte("```mermaid\nflowchart LR\nA --> B\n```\n")
	count := func(v View) (n int) {
		for _, w := range v.Warnings {
			if w.Code == mdoc.WarnFigureNoDesc {
				n++
			}
		}
		return
	}
	if n := count(RenderViewOpts(src, "", ts, "", mdoc.Options{RequireFigureDesc: true})); n != 1 {
		t.Fatalf("켠 렌더의 figure-no-desc = %d, 하나여야 함", n)
	}
	if n := count(RenderViewOpts(src, "", ts, "", mdoc.Options{})); n != 0 {
		t.Fatalf("끈 렌더의 figure-no-desc = %d, 없어야 함", n)
	}
}
