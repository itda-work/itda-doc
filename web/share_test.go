package web

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// 보기 화면의 확장 훅(itda-hyve#617 리뷰 3회차 [2]): 훅이 던져도 hook 은 던지지 않고(콘솔 오류 한 줄) 그리기 길이 이어진다. share.js 의
// ext·hook 정의를 그대로 떼어 node 로 돌린다 — 던지는 훅·없는 훅·인자 전달.
func TestShareHookContained(t *testing.T) {
	b, err := os.ReadFile("share.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	a := strings.Index(src, "  let ext = {};\n")
	z := strings.Index(src[max(a, 0):], "\n  };\n")
	if a < 0 || z < 0 {
		t.Fatal("share.js 에서 ext·hook 정의를 찾지 못함")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	js := src[a:a+z+len("\n  };\n")] + `
  const errs = []; console.error = (...a) => errs.push(a.map(String).join(" "));
  let got = 0;
  ext = { drawn: () => { throw new Error("boom"); }, flow: f => { got = f; } };
  hook("drawn"); hook("missing"); hook("flow", 7);
  const ok = errs.length === 1 && errs[0].includes("drawn") && errs[0].includes("boom") && got === 7;
  process.stdout.write(ok ? "HOOK OK" : "HOOK FAIL " + JSON.stringify({ errs, got }));
`
	out, err := exec.Command(node, "-e", js).CombinedOutput()
	if err != nil || string(out) != "HOOK OK" {
		t.Fatalf("훅이 던지면 그리기가 이어져야 함: %v\n%s", err, out)
	}
}
