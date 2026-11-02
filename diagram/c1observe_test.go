package diagram_test

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/itda-work/itda-doc/diagram/policytest"
)

// TestC1ObserveTable 은 diagrams-design.md 2장 「C1 제어 문자 — 현재 동작, 지원 약속 아님」 표(c1-observe)의 행 이름이
// policytest.C1RowIDs 와 1:1 인지 본다 — 표에 행을 더하거나 빼면 C1Policy 의 갈래도 같이 바꿔야 한다(회귀 감지 표, 지원 목록 아님).
func TestC1ObserveTable(t *testing.T) {
	b, err := os.ReadFile("../spec/diagrams-design.md")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i, j := strings.Index(s, "<!-- c1-observe:begin -->"), strings.Index(s, "<!-- c1-observe:end -->")
	if i < 0 || j < i {
		t.Fatal("c1-observe 표 표시가 없음")
	}
	var ids []string
	for _, m := range regexp.MustCompile("(?m)^\\| `(C1-[^`]+)` \\|").FindAllStringSubmatch(s[i:j], -1) {
		ids = append(ids, m[1])
	}
	if !slices.Equal(ids, policytest.C1RowIDs) {
		t.Errorf("DESIGN 관찰 표의 행 %v, C1Policy 의 행 %v", ids, policytest.C1RowIDs)
	}
}
