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
  eq("고정 제목(뷰어)", S.docTitle("보고서", "앱", "itda-doc 뷰어"), "itda-doc 뷰어");
  eq("고정 제목 없음", S.docTitle("보고서", "앱", undefined), "보고서 — 앱");
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

// YouTube 블록(mdoc youtube.go)의 공유 화면 쪽: 따라가기 중 정지 문지기 ytFollowGate 는 발표 슬라이드(kind "slide")의 번호가 직전에
// 따라간 번호와 다를 때만 참이다 — 같은 번호 재적용(조각 넘김·다시 맞춤)·내용 위치(kind "pos")는 멈추지 않고, pos 는 기억한 번호도
// 바꾸지 않는다. 화면 배선은 글로 본다: 재생은 호스트 부품 YT(pick — ItdaYT·HyveYT)를 쓰고, 틀 주소는 youtube-nocookie·referrerpolicy
// strict-origin, 따라가기(scrollToFollow)는 문지기가 참일 때만, 다시 그리기(render)·끝난 화면(showEnded)은 늘 멈춘다.
func TestShareYouTubeFollowGate(t *testing.T) {
	node := needNode(t)
	js := `
  const fs = require("fs"), vm = require("vm");
  const ctx = { console }; ctx.window = ctx; vm.createContext(ctx);
  for (const f of ["pager.js", "slides.js", "share.js"]) vm.runInContext(fs.readFileSync(f, "utf8"), ctx);
  const g = ctx.window.ItdaShare.ytFollowGate();
  const got = [g(3, "slide"), g(3, "slide"), g(7, "pos"), g(3, "slide"), g(4, "slide"), g(3, "slide"), g(3, "pos"), g(3, "slide")];
  const want = [true, false, false, false, true, true, false, false];
  process.stdout.write(JSON.stringify(got) === JSON.stringify(want) ? "GATE OK" : "GATE FAIL " + JSON.stringify(got));
`
	out, err := exec.Command(node, "-e", js).CombinedOutput()
	if err != nil || string(out) != "GATE OK" {
		t.Fatalf("ytFollowGate: %v\n%s", err, out)
	}
	b, err := os.ReadFile("share.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		`const YT = hostPart("YT");`,
		`const YT_EMBED = "https://www.youtube-nocookie.com/embed/";`,
		`referrerPolicy: "strict-origin",`,
		`inScope: a => $("viewer").contains(a),`,
		"    if (ytFollowChanged(i, kind)) ytStop();",
		"    if (state.ended) return;\n    ytStop();",
		"    state.ended = true;\n    linked.stop();\n    ytStop();",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("share.js 에 %q 없음", want)
		}
	}
	html, _ := os.ReadFile("share.html")
	if !strings.Contains(string(html), `<script src="{{BASE}}/_/yt.js"></script>`) {
		t.Error("share.html 이 호스트 부품 yt.js 자리를 싣지 않음")
	}
}

// 발표 꼴 따라가기의 화면 연결(itda-work/itda-hyve#362 리뷰 1회차 [2]): share.js 의 fol 선언(align·release)을 그대로 떼어 node 로
// 돌린다 — follower 는 받은 선택을 돌려주는 가짜이고 followAction·restoreOwnLook 은 같은 파일의 진짜다. 보는 것 셋:
// align 이 호스트의 발표 꼴(want)을 followAction 에 넘기는지(이미 보드면 다시 그리지 않고 맞춘다), redraw 가 그 꼴을 고르는지
// (state.chosen = "dashboard"), release 가 슬라이드·보드 문서를 제 꼴로 되돌리는지(restoreOwnLook).
func TestShareFollowerWiring(t *testing.T) {
	b, err := FS.ReadFile("share.js") // 내장 FS 로 읽어 go test -overlay 변이가 닿는다
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	a := strings.Index(src, "  const fol = follower({\n")
	z := strings.Index(src[max(a, 0):], "\n  });\n")
	if a < 0 || z < 0 {
		t.Fatal("share.js 에서 fol 선언을 찾지 못함")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	decl := src[a : a+z+len("\n  });\n")]
	js := `
  const vm = require("vm");
  const ctx = { console }; ctx.window = ctx; vm.createContext(ctx);
  vm.runInContext(process.env.SRC, ctx);
  const S = ctx.window.ItdaShare;
  const make = new Function("follower", "followAction", "restoreOwnLook", "state", "$", "window", "draw", "scrollToFollow", "paintLookSwitch",
    ` + "`" + `${process.env.DECL}; return fol;` + "`" + `);
  function rig(view, look, chosen) {
    const r = { draws: 0, scrolls: 0 };
    r.state = { ended: false, look, view, chosen };
    const el = { hidden: true, textContent: "", classList: { remove() {} }, querySelectorAll: () => [] };
    r.fol = make(o => o, S.followAction, S.restoreOwnLook, r.state, () => el, { innerWidth: 1200 },
      () => { r.draws++; }, () => { r.scrolls++; }, () => {});
    return r;
  }
  const bad = [];
  const doc = {}, board = { board: { present: true } };
  // 쪽 문서를 보드 발표로 따라가면 보드로 다시 그린다
  let r = rig(doc, "doc", "");
  r.fol.align(0, "slide", 0, "dashboard");
  if (r.state.chosen !== "dashboard" || r.draws !== 1) bad.push("보드 발표 → chosen " + r.state.chosen + " · 그리기 " + r.draws);
  // 이미 보드로 보고 있으면 다시 그리지 않고 번호로 맞춘다(want 가 빠지면 슬라이드로 다시 그린다)
  r = rig(board, "dashboard", "");
  r.fol.align(0, "slide", 0, "dashboard");
  if (r.draws !== 0 || r.scrolls !== 1) bad.push("보드 위 보드 발표 → 그리기 " + r.draws + " · 맞춤 " + r.scrolls);
  // 보드 문서를 슬라이드로 따라갔다가 풀리면 제 꼴(보드)로 되돌린다
  r = rig(board, "slides", "slides");
  r.fol.release();
  if (r.state.chosen !== "" || r.draws !== 1) bad.push("풀림 → chosen " + JSON.stringify(r.state.chosen) + " · 그리기 " + r.draws);
  process.stdout.write(bad.length ? "FOL FAIL " + bad.join(" / ") : "FOL OK");
`
	cmd := exec.Command(node, "-e", js)
	cmd.Env = append(os.Environ(), "DECL="+decl, "SRC="+src)
	out, err := cmd.CombinedOutput()
	if err != nil || string(out) != "FOL OK" {
		t.Fatalf("발표 꼴 따라가기 연결: %v\n%s", err, out)
	}
}

// 보드를 그릴 수 없는 화면(itda-work/itda-hyve#362 — 웹 뷰어는 dashboard.js 를 싣지 않는다, scripts/build-web.sh): boardOK 가 거짓이면
// 보드 문서도 발표 꼴 "dashboard" 도 보드를 고르지 않고 쪽 문서와 같이 고른다(lookFor). 따라가기는 보드로 다시 그리지 않고(followAction
// "none"), 꼴 단추도 숨기지 않는다(lookSwitchHidden). 참이면(공유 화면 — 기본값) 종전대로 보드다. 화면 연결은 share.js 가 화면을 시작할 때
// window.ItdaBoard 로 setBoardOK 를 부르는지를 글로 본다 — 첫 그리기(linked.start) 전에. 실제 화면은 Chrome 헤드리스 실측(처리 보고).
func TestShareBoardUnavailable(t *testing.T) {
	b, err := FS.ReadFile("share.js") // 내장 FS 로 읽어 go test -overlay 변이가 닿는다
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	at := strings.Index(src, "\n  setBoardOK(!!window.ItdaBoard);")
	if at < 0 || at < strings.Index(src, "\n  if (!source) return;\n") || at > strings.Index(src, "\n  linked.start();") {
		t.Error("share.js 가 화면 시작(원천을 고른 뒤·첫 그리기 전)에 setBoardOK(!!window.ItdaBoard) 를 부르지 않음")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	js := `
  const vm = require("vm");
  const ctx = { console }; ctx.window = ctx; vm.createContext(ctx);
  vm.runInContext(process.env.SRC, ctx);
  const S = ctx.window.ItdaShare, bad = [];
  const eq = (name, got, want) => { if (got !== want) bad.push(name + ": " + JSON.stringify(got) + " ≠ " + JSON.stringify(want)); };
  const doc = {}, board = { board: { present: true } }, both = { board: { present: true }, slides: { present: true } };
  // 기본값(참) — 공유 화면
  eq("참: 보드 문서", S.lookFor(board, 1400, ""), "dashboard");
  eq("참: 발표 꼴 dashboard", S.lookFor(doc, 1400, "dashboard"), "dashboard");
  eq("참: 보드 발표로 다시 그림", S.followAction("slide", "doc", doc, 1400, "dashboard"), "redraw");
  eq("참: 꼴 단추 숨김", S.lookSwitchHidden(board, "dashboard", false), true);
  S.setBoardOK(false);
  eq("거짓: 보드 문서(넓음) → 쪽", S.lookFor(board, 1400, ""), "doc");
  eq("거짓: 보드 문서(좁음) → 이어보기", S.lookFor(board, 390, ""), "flow");
  eq("거짓: 보드+슬라이드 문서 → 슬라이드", S.lookFor(both, 1400, ""), "slides");
  eq("거짓: 발표 꼴 dashboard → 쪽", S.lookFor(doc, 1400, "dashboard"), "doc");
  eq("거짓: 보드 발표 → 다시 그리지 않음", S.followAction("slide", "doc", board, 1400, "dashboard"), "none");
  eq("거짓: 슬라이드 발표는 그대로", S.followAction("slide", "doc", board, 1400, "slides"), "redraw");
  eq("거짓: 꼴 단추 보임", S.lookSwitchHidden(board, "doc", false), false);
  eq("거짓: 고른 꼴(쪽)", S.lookFor(board, 1400, "flow"), "flow");
  S.setBoardOK(true);
  eq("다시 참: 보드 문서", S.lookFor(board, 1400, ""), "dashboard");
  process.stdout.write(bad.length ? "BOARD FAIL " + bad.join("; ") : "BOARD OK");
`
	cmd := exec.Command(node, "-e", js)
	cmd.Env = append(os.Environ(), "SRC="+src)
	out, err := cmd.CombinedOutput()
	if err != nil || string(out) != "BOARD OK" {
		t.Fatalf("보드를 그릴 수 없는 화면: %v\n%s", err, out)
	}
}
