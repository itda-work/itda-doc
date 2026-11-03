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

// 정식 이름과 옛 이름(itda-hyve#625): pager.js·slides.js·share.js 는 window.ItdaPager·ItdaSlides·ItdaShare 로 내보내고 옛 이름
// window.HyvePager·HyveSlides·HyveShare 는 같은 객체다(한 번에 대입 — 갈라지지 않는다). share.js 는 document 가 없으면 화면을
// 시작하지 않으므로 node 에서 셋을 차례로 실어 본다. 탭 제목: 앱 이름 자리가 채워지면 「제목 — 앱」, 비었거나 자리 글자 그대로면 제목만.
func TestGlobalNamesAndTitle(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	js := `
  const fs = require("fs"), vm = require("vm");
  const ctx = { console }; ctx.window = ctx; vm.createContext(ctx);
  for (const f of ["pager.js", "slides.js", "share.js"]) vm.runInContext(fs.readFileSync(f, "utf8"), ctx);
  const w = ctx.window, bad = [];
  for (const n of ["Pager", "Slides", "Share"]) {
    if (!w["Itda" + n] || typeof w["Itda" + n] !== "object") bad.push("Itda" + n + " 없음");
    if (w["Hyve" + n] !== w["Itda" + n]) bad.push("Hyve" + n + " 가 Itda" + n + " 와 다른 객체");
  }
  const S = w.ItdaShare;
  const eq = (name, got, want) => { if (got !== want) bad.push(name + ": " + JSON.stringify(got) + " ≠ " + JSON.stringify(want)); };
  eq("앱 이름", S.appName("Itda Hyve Docs"), "Itda Hyve Docs");
  eq("앞뒤 빈칸", S.appName("  앱  "), "앱");
  eq("채우지 않은 자리", S.appName("{{APP_NAME}}"), "");
  eq("빈 값", S.appName(""), "");
  eq("없음", S.appName(null), "");
  eq("제목 + 앱", S.docTitle("보고서", "Itda Hyve Docs"), "보고서 — Itda Hyve Docs");
  eq("제목만", S.docTitle("보고서", ""), "보고서");
  // 호스트 전역 물러서기(pick — hostPart·extInit 가 쓴다): 정식 이름 Itda* 먼저, 없으면 옛 이름 Hyve*
  const a = {}, b = {};
  eq("둘 다 → Itda", S.pick({ ItdaCodeCopy: a, HyveCodeCopy: b }, "CodeCopy"), a);
  eq("Hyve 만 → Hyve", S.pick({ HyveCodeCopy: b }, "CodeCopy"), b);
  eq("없음 → undefined", S.pick({}, "CodeCopy"), undefined);
  process.stdout.write(bad.length ? "FAIL " + bad.join("; ") : "NAMES OK");
`
	out, err := exec.Command(node, "-e", js).CombinedOutput()
	if err != nil || string(out) != "NAMES OK" {
		t.Fatalf("전역 이름·탭 제목: %v\n%s", err, out)
	}
}

// 공유 화면 틀(itda-hyve#625): 모듈 글에 앱 이름을 박지 않는다 — share.html 은 앱 이름 자리(<meta name="application-name">)를 두고
// share.js 는 탭 제목을 docTitle 로만 만든다(「 — <앱 이름>」 글자를 직접 쓰지 않는다).
func TestShareNoAppName(t *testing.T) {
	for _, f := range []string{"share.html", "share.js"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "Itda Hyve") {
			t.Errorf("%s 에 앱 이름이 박혀 있음", f)
		}
	}
	html, _ := os.ReadFile("share.html")
	if !strings.Contains(string(html), `<meta name="application-name" content="{{APP_NAME}}">`) {
		t.Error("share.html 에 앱 이름 자리가 없음")
	}
	js, _ := os.ReadFile("share.js")
	if n := strings.Count(string(js), "document.title = docTitle("); n != 2 {
		t.Errorf("share.js 의 탭 제목이 docTitle 을 지나지 않음(%d 곳)", n)
	}
}
