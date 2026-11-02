package mdoc

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "testdata 의 golden 파일을 지금 결과로 다시 쓴다")

// TestGolden 은 견본 문서(보고서·공문)의 결과를 통째로 고정한다. 바뀌면 diff 를 보고 의도한 것일 때만
//
//	go test ./mdoc -run TestGolden -update
//
// 로 다시 쓴다. golden 은 본문 HTML 과, 경고·목차를 적은 .txt 두 가지다.
func TestGolden(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "*.md"))
	if err != nil || len(files) < 2 {
		t.Fatalf("견본 문서가 둘 이상 있어야 함: %v %v", files, err)
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".md")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			r := render(t, string(src))
			assertSafe(t, r.HTML)
			check(t, filepath.Join("testdata", name+".golden.html"), r.HTML)
			check(t, filepath.Join("testdata", name+".golden.txt"), summary(r))
		})
	}
}

// summary 는 HTML 밖의 결과(문서 정보·목차·경고)를 사람이 읽는 글로 적는다.
func summary(r Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "meta: %+v\n", r.Meta)
	fmt.Fprintf(&b, "toc-placed: %v\n", r.TOCPlaced)
	var toc func(items []TOCItem, depth int)
	toc = func(items []TOCItem, depth int) {
		for _, it := range items {
			fmt.Fprintf(&b, "%stoc: h%d #%s [%s] %s\n", strings.Repeat("  ", depth), it.Level, it.ID, it.Number, it.Text)
			toc(it.Children, depth+1)
		}
	}
	toc(r.TOC, 0)
	for _, w := range r.Warnings {
		fmt.Fprintf(&b, "warning: %d %s %s\n", w.Line, w.Code, w.Message)
	}
	return b.String()
}

func check(t *testing.T, path, got string) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (처음이면 -update 로 만든다)", err)
	}
	if string(want) != got {
		t.Errorf("%s 와 다름 (-update 로 갱신)\n--- got\n%s", path, got)
	}
}
