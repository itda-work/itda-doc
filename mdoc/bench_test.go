package mdoc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// genDoc 은 벤치마크용 한글 문서를 만든다. 제목마다 문단 paras 개와 rows 줄짜리 표 하나를 둔다.
// 문단에는 강조·링크·코드처럼 인라인 파서가 일하는 것을 섞는다.
func genDoc(headings, parasPer, rows int) []byte {
	var b strings.Builder
	b.WriteString("---\ntitle: 벤치마크 문서\ntoc: true\nnumber-sections: true\n---\n\n[TOC]\n\n")
	for h := 0; h < headings; h++ {
		fmt.Fprintf(&b, "%s %d번째 절의 제목\n\n", strings.Repeat("#", 1+h%3), h+1)
		for p := 0; p < parasPer; p++ {
			fmt.Fprintf(&b, "%d번 문단이다. 회사 문서는 **\"결론\"**을 먼저 쓰고, 근거는 [붙임 %d](#붙임)과 `코드 %d` 로 단다. "+
				"매출은 전년 대비 *12%%* 늘었고 비용은 ~~줄었다~~ 그대로다.\n\n", p+1, p+1, p+1)
		}
		b.WriteString("| 구분 | 담당 | 금액 | 비고 |\n|:--|:--|--:|:--|\n")
		for r := 0; r < rows; r++ {
			fmt.Fprintf(&b, "| 항목 %d | 홍길동 | %d,000원 | 검토 중<br>재확인 |\n", r+1, (r+1)*37)
		}
		b.WriteString("\nTable: 세부 항목\n\n")
	}
	return []byte(b.String())
}

func benchRender(b *testing.B, src []byte) {
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Render(src, Options{}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRender 는 문서 크기별 렌더 시간이다. 편집기가 입력마다 다시 렌더하므로 30쪽(약 60KB)이 기준이다.
//
//	go test ./mdoc -run '^$' -bench BenchmarkRender -benchmem
func BenchmarkRender(b *testing.B) {
	report, err := os.ReadFile(filepath.Join("testdata", "report.md"))
	if err != nil {
		b.Fatal(err)
	}
	docs := []struct {
		name string
		src  []byte
	}{
		{"report.md", report},
		{"30쪽", genDoc(26, 6, 20)},  // 약 60KB
		{"1MB", genDoc(400, 6, 20)}, // 제목 400, 문단 2,400, 20줄 표 400
	}
	for _, d := range docs {
		b.Run(fmt.Sprintf("%s_%.0fKB", d.name, float64(len(d.src))/1024), func(b *testing.B) {
			benchRender(b, d.src)
		})
	}
}
