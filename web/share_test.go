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

// 처음 보기 꼴(itda-hyve#673): 필드가 없으면 chosen 은 비어 지금처럼 폭으로 고르고, look 은 처음 값일 뿐이라 이미 고른 꼴(따라가기·이 탭에서 사람이 고른 꼴)이 이긴다.
// 폴더 공유: 상대 링크 풀기(docLink)·목록 그리기(fileTreeHTML)·목록이 없으면 사이드바를 건드리지 않음(mountFiles).
func TestShareFolderParts(t *testing.T) {
	node := needNode(t)
	js := `
  const fs = require("fs"), vm = require("vm");
  const ctx = { console }; ctx.window = ctx; vm.createContext(ctx);
  for (const f of ["pager.js", "slides.js", "share.js"]) vm.runInContext(fs.readFileSync(f, "utf8"), ctx);
  const S = ctx.window.ItdaShare, bad = [];
  const eq = (name, got, want) => { const a = JSON.stringify(got), b = JSON.stringify(want); if (a !== b) bad.push(name + ": " + a + " ≠ " + b); };
  eq("필드 없음", S.initialLook("", "", undefined), "");
  eq("서버 값", S.initialLook("", "", "flow"), "flow");
  eq("사람이 고른 꼴", S.initialLook("", "doc", "flow"), "doc");
  eq("이미 정해짐", S.initialLook("slides", "doc", "flow"), "slides");
  eq("모르는 값", S.initialLook("", "x", "y"), "");
  eq("폭으로 고름은 그대로", S.lookFor({}, 500, S.initialLook("", "", undefined)), "flow");
  eq("look 이 폭을 이김", S.lookFor({}, 500, S.initialLook("", "", "doc")), "doc");
  const docs = ["README.md", "a/b.md", "a/다른 문서.md", "c.md", "\u1112\u1161\u11ab.md"];
  eq("같은 폴더", S.docLink("b.md", "a/x.md", docs), { doc: "a/b.md", hash: "" });
  eq("위 폴더", S.docLink("../c.md#머리", "a/x.md", docs), { doc: "c.md", hash: "#머리" });
  eq("퍼센트 꼴", S.docLink("%EB%8B%A4%EB%A5%B8%20%EB%AC%B8%EC%84%9C.md", "a/x.md", docs), { doc: "a/다른 문서.md", hash: "" });
  eq("NFC 링크 → NFD 파일", S.docLink("한.md", "README.md", docs), { doc: "\u1112\u1161\u11ab.md", hash: "" });
  eq("폴더 밖", S.docLink("../../c.md", "a/x.md", docs), null);
  eq("뿌리에서 위로", S.docLink("../c.md", "README.md", docs), null);
  eq("절대", S.docLink("/c.md", "a/x.md", docs), null);
  eq("스킴", S.docLink("file:///c.md", "a/x.md", docs), null);
  eq("목록에 없음", S.docLink("없음.md", "a/x.md", docs), null);
  eq("md 가 아닌 파일", S.docLink("c.pdf", "README.md", docs), null);
  eq("풀 수 없는 꼴", S.docLink("%E0%A4%A.md", "README.md", docs), null);
  eq("숨김 경로는 목록에 없다", S.docLink(".git/x.md", "README.md", docs), null);
  const html = S.fileTreeHTML(["z.md", "d/<b>.md"], "d/<b>.md");
  eq("이스케이프·현재 문서", html, '<ul><li><a href="?doc=z.md">z.md</a></li><li><span class="dir">d</span><ul><li><a href="?doc=d%2F%3Cb%3E.md" class="on" aria-current="page">&lt;b&gt;.md</a></li></ul></li></ul>');
  // 목록이 없으면 요소를 만들지도 넣지도 않는다
  let made = 0, put = 0, gone = 0;
  const doc = { createElement() { made++; return { setAttribute() {} }; } };
  const side = { firstChild: null, insertBefore() { put++; }, querySelector() { return null; } };
  eq("없음", S.mountFiles(doc, side, null), null);
  eq("빈 응답", S.mountFiles(doc, side, {}), null);
  eq("DOM 그대로", [made, put], [0, 0]);
  S.mountFiles(doc, { firstChild: null, insertBefore() { put++; }, querySelector() { return { remove() { gone++; } }; } }, null);
  eq("끝나면 뗀다", gone, 1);
  eq("있으면 넣는다", S.mountFiles(doc, side, { docs: ["a.md"] }, "a.md").id, "side-files");
  eq("하나", [made, put], [1, 1]);
  // 목록 요청은 view 에 doc 이 있을 때만 — 단일 파일·문서 창은 files.json 을 부르지 않는다
  const run = async (view) => {
    const urls = []; let files = null, shown = 0, handlers = {}, nav = 0;
    const w = { sessionStorage: {}, location: {}, setTimeout: () => 0, clearTimeout() {}, addEventListener() {}, document: { addEventListener() {} },
      fetch: async (u) => { urls.push(u); return { ok: true, status: 200, json: async () => u.endsWith("files.json") ? { docs: ["a.md"], rev: 1 } : view }; },
      EventSource: class { constructor() {} addEventListener(n, f) { handlers[n] = f; } close() {} } };
    const link = { addEventListener() {}, setAttribute() {} };
    const api = { $: () => link, ended: () => false, current: () => null, show: async () => { shown++; }, failed() {}, themeFailed() {}, end() {}, hello() {}, slide() {}, pos() {}, redraw() {}, files: d => { files = d; }, nav: () => { nav++; } };
    S.hostSource("/b", w).attach(api).start();
    await new Promise(r => setImmediate(r));
    await new Promise(r => setImmediate(r));
    return { urls, files, shown, handlers, nav };
  };
  (async () => {
    const one = await run({ version: 1, theme: "t", html: "" });
    eq("단일: 요청", one.urls, ["/b/view.json"]);
    eq("단일: 목록 없음", [one.files, one.nav], [null, 0]);
    const many = await run({ version: 1, theme: "t", html: "", doc: "a.md" });
    eq("폴더: 요청", many.urls, ["/b/view.json", "/b/files.json"]);
    eq("폴더: 목록", [many.files && many.files.docs, many.nav], [["a.md"], 1]);
    many.handlers.files({});
    await new Promise(r => setImmediate(r));
    await new Promise(r => setImmediate(r));
    eq("files 알림이면 다시 받음", many.urls.length, 3);
    // 문서 부재(표시 머리가 든 404)는 공유 끝이 아니다 — 오류 view 를 그리고 목록을 받는다. 표시가 없는 404 는 지금처럼 공유 끝
    const gone = async (head) => {
      const urls = []; let ended = 0, shown = null, files = null, timers = 0, es = null;
      const reply = { ok: false, status: 404, headers: { get: n => n === "X-Share-Doc" ? head : null }, json: async () => ({ version: 2, theme: "", error: "문서를 읽지 못했습니다", doc: "a.md" }) };
      const w = { sessionStorage: {}, location: {}, setTimeout: () => { timers++; return 1; }, clearTimeout() {}, addEventListener() {}, document: { addEventListener() {} },
        fetch: async (u) => { urls.push(u); return u.endsWith("files.json") ? { ok: true, status: 200, json: async () => ({ docs: ["b.md"], rev: 1 }) } : reply; },
        EventSource: class { constructor() { es = this; } addEventListener() {} close() {} } };
      const link = { addEventListener() {}, setAttribute() {} };
      const api = { $: () => link, ended: () => ended > 0, current: () => null, show: async v => { shown = v; }, failed() {}, themeFailed() {}, end() { ended++; }, hello() {}, slide() {}, pos() {}, redraw() {}, files: d => { files = d; }, nav() {} };
      S.hostSource("/b", w).attach(api).start();
      for (let i = 0; i < 6; i++) await new Promise(r => setImmediate(r));
      const first = ended;
      if (es && es.onerror) es.onerror(); // 스트림이 끊겨 살아 있나 묻는다
      for (let i = 0; i < 6; i++) await new Promise(r => setImmediate(r));
      return { first, ended, shown, files, timers };
    };
    const miss = await gone("missing");
    eq("부재: 끝나지 않음", [miss.first, miss.ended], [0, 0]);
    eq("부재: 오류 view", miss.shown && miss.shown.error, "문서를 읽지 못했습니다");
    eq("부재: 목록 남음", miss.files && miss.files.docs, ["b.md"]);
    eq("부재: 다시 붙기", miss.timers, 1);
    const over = await gone(null);
    eq("표시 없는 404 는 공유 끝", [over.first > 0, over.shown, over.files], [true, null, null]);
    eq("머리 없는 응답", S.docMissing({ status: 404 }), false);
    eq("200 은 부재가 아님", S.docMissing({ status: 200, headers: { get: () => "missing" } }), false);
    process.stdout.write(bad.length ? "FAIL " + bad.join("; ") : "FOLDER OK");
  })();
`
	out, err := exec.Command(node, "-e", js).CombinedOutput()
	if err != nil || string(out) != "FOLDER OK" {
		t.Fatalf("폴더 공유 부품: %v\n%s", err, out)
	}
}

// 연결 복구는 공유 생존과 문서 가용성을 가른다: 끊길 때마다 지금 문서의 view 와 폴더면 목록을 다시 받고(resync), 문서 없음이면 오류 view 와 최신 목록을, 표시 없는 404·403 이면 끝남을
// 보인다. 문서 없음이면 스트림이 404 라 알림이 오지 않으므로 backoff 재시도가 목록과 본문을 잇는다. 단일 파일은 같은 절차에서 목록을 건드리지 않는다.
func TestShareFolderRecovery(t *testing.T) {
	node := needNode(t)
	js := `
  const fs = require("fs"), vm = require("vm");
  const ctx = { console }; ctx.window = ctx; vm.createContext(ctx);
  for (const f of ["pager.js", "slides.js", "share.js"]) vm.runInContext(fs.readFileSync(f, "utf8"), ctx);
  const S = ctx.window.ItdaShare, bad = [];
  const eq = (name, got, want) => { const a = JSON.stringify(got), b = JSON.stringify(want); if (a !== b) bad.push(name + ": " + a + " ≠ " + b); };
  const settle = async () => { for (let i = 0; i < 8; i++) await new Promise(r => setImmediate(r)); };
  // page 는 가짜 서버(srv)에 이어진 한 화면이다. srv.mode: ok·missing·gone(표시 없는 404)·kicked(403)·down(망 실패)
  const page = async (srv) => {
    const p = { reqs: [], streams: [], timers: [], ended: [], fails: 0, cur: null, shows: 0, files: null, nav: null, assigned: [] };
    const reply = (status, head, body) => ({ ok: status === 200, status, headers: { get: n => n === "X-Share-Doc" ? head : null }, json: async () => body });
    const w = { sessionStorage: {}, addEventListener() {}, document: { addEventListener() {} },
      location: { pathname: "/b", hash: "", assign: u => p.assigned.push(u) },
      setTimeout: f => { p.timers.push({ f, off: false }); return p.timers.length; },
      clearTimeout: id => { if (id && p.timers[id - 1]) p.timers[id - 1].off = true; },
      fetch: async (u, o) => {
        p.reqs.push(((o && o.method) || "GET") + " " + u);
        if (srv.mode === "down") throw new Error("망");
        if (srv.mode === "gone") return reply(404, null, null);
        if (srv.mode === "kicked") return reply(403, null, null);
        if (u.endsWith("files.json")) return reply(200, null, { docs: srv.docs, rev: srv.rev });
        if (srv.mode === "missing") return reply(404, "missing", { version: ++srv.stamp, theme: "", error: "문서를 읽지 못했습니다", doc: srv.doc });
        return reply(200, null, { version: srv.version, theme: "t", html: srv.html, doc: srv.doc });
      },
      EventSource: class { constructor(u) { this.u = u; this.on = {}; p.streams.push(this); } addEventListener(n, f) { this.on[n] = f; } close() { this.closed = true; } } };
    const link = { addEventListener() {}, setAttribute() {} };
    const api = { $: () => link, ended: () => p.ended.length > 0, current: () => p.cur, show: async v => { p.cur = v; p.shows++; }, failed() { p.fails++; }, themeFailed() {},
      end(r) { p.ended.push(r); }, hello() {}, slide() {}, pos() {}, redraw() {}, files: d => { p.files = d.docs; }, nav: n => { p.nav = n; } };
    S.hostSource("/b", w).attach(api).start();
    await settle();
    p.drop = async () => { p.reqs.length = 0; p.streams[p.streams.length - 1].onerror(); await settle(); };
    p.retry = async () => { const t = p.timers.filter(x => !x.off).pop(); t.off = true; p.reqs.length = 0; t.f(); await settle(); };
    p.hello = async d => { p.streams[p.streams.length - 1].on.hello({ data: JSON.stringify(d) }); await settle(); };
    return p;
  };
  const folder = () => ({ mode: "ok", version: 1, stamp: 100, html: "OLD CONTENT", doc: "a.md", docs: ["a.md", "b.md"], rev: 1 });
  const VIEW = "GET /b/view.json", FILES = "GET /b/files.json";
  (async () => {
    eq("상태: 표시", S.viewState({ status: 404, missing: true }), "missing");
    eq("상태: 표시 없는 404", S.viewState({ status: 404, missing: false }), "ended");
    eq("상태: 403", S.viewState({ status: 403 }), "kicked");
    eq("상태: 429 는 끝이 아니다", S.viewState({ status: 429 }), "alive");

    // 정상 → 끊김 → 문서 없음 복구
    let srv = folder(), p = await page(srv);
    eq("정상: 본문", p.cur.html, "OLD CONTENT");
    srv.mode = "missing"; srv.docs = ["b.md"]; srv.rev = 2;
    await p.drop();
    eq("복구: 요청", p.reqs, [VIEW, FILES]);
    eq("복구: 옛 본문이 남지 않음", [p.cur.error, p.cur.html], ["문서를 읽지 못했습니다", undefined]);
    eq("복구: 최신 목록", p.files, ["b.md"]);
    eq("복구: 끝나지 않고 다시 붙기를 기다림", [p.ended, p.timers.filter(x => !x.off).length, p.fails], [[], 1, 0]);

    // 문서 없음 → 목록 변경 반영(스트림은 404 — 재시도가 잇는다)
    srv.docs = ["b.md", "c.md"]; srv.rev = 3;
    await p.retry();
    eq("문서 없음: 스트림을 다시 엶", p.streams[p.streams.length - 1].u, "/b/events");
    await p.drop();
    eq("문서 없음: 재시도의 요청", p.reqs, [VIEW, FILES]);
    eq("문서 없음: 목록 따라옴", p.files, ["b.md", "c.md"]);

    // 문서 없음 → 문서 복귀
    srv.mode = "ok"; srv.version = 5; srv.html = "NEW CONTENT"; srv.docs = ["a.md", "b.md", "c.md"]; srv.rev = 4;
    await p.retry();
    await p.drop();
    eq("복귀: 본문", [p.cur.error, p.cur.html], [undefined, "NEW CONTENT"]);
    eq("복귀: 목록", p.files, ["a.md", "b.md", "c.md"]);
    await p.retry();
    await p.hello({ version: 5, theme: "t", files_rev: 4 });
    eq("복귀: 스트림이 붙은 뒤 더 묻지 않음", [p.reqs, p.ended], [[], []]);

    // 판이 같은 복구는 다시 그리지 않는다
    const shows = p.shows;
    await p.drop();
    eq("같은 판: 그리지 않음", p.shows, shows);
    eq("같은 판: 동기화는 함", p.reqs, [VIEW, FILES]);

    // 문서 없음 → 다른 문서 선택: 화면을 새로 불러 정상 스트림으로 돌아간다
    srv = folder(); p = await page(srv);
    srv.mode = "missing"; await p.drop();
    p.nav.open("b.md", "");
    eq("다른 문서: 주소", p.assigned, ["?doc=b.md"]);
    srv = folder(); srv.doc = "b.md"; p = await page(srv);
    await p.hello({ version: 1, theme: "t", files_rev: 1 });
    eq("다른 문서: 정상 스트림", [p.cur.doc, p.cur.error, p.reqs, p.timers.length], ["b.md", undefined, [VIEW, FILES], 0]);

    // 끊김 뒤 표시 없는 404·403 은 끝남 — 그 뒤 목록 요청도, 다시 붙기도 없다
    srv = folder(); p = await page(srv);
    srv.mode = "gone"; await p.drop();
    eq("표시 없는 404: 끝남", [p.ended, p.reqs, p.timers.length], [[""], [VIEW], 0]);
    srv = folder(); p = await page(srv);
    srv.mode = "kicked"; await p.drop();
    eq("403: 끊김", [p.ended, p.reqs, p.timers.length], [["kicked"], [VIEW], 0]);

    // 망 실패는 끝이 아니다 — 안내 없이 다시 붙는다(본문을 지우지 않는다)
    srv = folder(); p = await page(srv);
    srv.mode = "down"; await p.drop();
    eq("망 실패", [p.ended, p.fails, p.timers.filter(x => !x.off).length, p.cur.html], [[], 0, 1, "OLD CONTENT"]);

    // 단일 파일: 같은 절차, 목록 요청 없음. 끝남도 그대로
    srv = folder(); srv.doc = undefined; p = await page(srv);
    eq("단일: 처음", p.reqs, [VIEW]);
    srv.version = 2; srv.html = "SAVED";
    await p.drop();
    eq("단일: 복구 요청", p.reqs, [VIEW]);
    eq("단일: 본문·목록", [p.cur.html, p.files, p.nav], ["SAVED", null, null]);
    srv.mode = "gone"; await p.retry(); await p.drop();
    eq("단일: 표시 없는 404 는 끝남", [p.ended, p.reqs], [[""], [VIEW]]);

    process.stdout.write(bad.length ? "FAIL " + bad.join("; ") : "RECOVERY OK");
  })().catch(e => process.stdout.write("FAIL " + e.stack));
`
	out, err := exec.Command(node, "-e", js).CombinedOutput()
	if err != nil || string(out) != "RECOVERY OK" {
		t.Fatalf("연결 복구: %v\n%s", err, out)
	}
}

// 같은 주소 공유의 잠시 닫힘(itda-hyve#705 3절): stop 의 까닭 paused·hello 의 keep 뒤 끊김은 끝이 아니다. 404·망 실패는 재시도, 403 만 끝, 다시 붙으면 새로 고침 없이 띠가 사라지고
// 바뀐 문서를 받는다. 24시간 뒤 멈춘다. keep 이 아닌 공유는 그대로다(끊김 뒤 404 는 끝, TestShareFolderRecovery 도 본다).
func TestSharePausedResume(t *testing.T) {
	node := needNode(t)
	js := `
  const fs = require("fs"), vm = require("vm");
  const ctx = { console }; ctx.window = ctx; vm.createContext(ctx);
  for (const f of ["pager.js", "slides.js", "share.js"]) vm.runInContext(fs.readFileSync(f, "utf8"), ctx);
  const S = ctx.window.ItdaShare, bad = [];
  const eq = (name, got, want) => { const a = JSON.stringify(got), b = JSON.stringify(want); if (a !== b) bad.push(name + ": " + a + " ≠ " + b); };
  const settle = async () => { for (let i = 0; i < 8; i++) await new Promise(r => setImmediate(r)); };
  const DAY = 24 * 3600 * 1000;
  // page 는 가짜 서버(srv)에 이어진 한 화면이다. srv.mode: ok·gone(404)·kicked(403)·down(망 실패)·stranger(쿠키를 모름 — view.json 404, 지금 주소 200)·page403(지금 주소만 403)
  // themed 면 show 가 진짜 themeApplier 를 지나 이 페이지의 테마 키를 잡는다(hold 는 함수로 줘도 된다 — 시험 중간에 바뀐다)
  const page = async (srv, hold, themed) => {
    const p = { streams: [], timers: [], ended: [], fails: 0, cur: null, shows: 0, bands: [], reloads: 0, wakes: [], views: 0, pages: [], base: "/b", hrefs: [], link: {} };
    const reply = (status, body) => ({ ok: status === 200, status, headers: { get: () => null }, json: async () => body });
    const on = (o, n, f) => { if (n === "pageshow" || n === "visibilitychange") p.wakes.push(f); if (n === "online") p.online = f; };
    const kept = {};
    const w = { sessionStorage: { getItem: k => kept[k] || null, setItem: (k, v) => { kept[k] = v; } }, addEventListener(n, f) { on(w, n, f); }, document: { addEventListener(n, f) { on(w, n, f); }, visibilityState: "visible" },
      location: { pathname: "/b", hash: "", href: "http://h/b?doc=a.md", reload: () => { p.reloads++; } },
      setTimeout: f => { p.timers.push({ f, off: false }); return p.timers.length; },
      clearTimeout: id => { if (id && p.timers[id - 1]) p.timers[id - 1].off = true; },
      fetch: async (u, o) => {
        if (srv.mode === "down") throw new Error("망");
        if (!u.endsWith("view.json")) {
          p.pages.push(((o && o.method) || "GET") + " " + u);
          return reply(srv.mode === "stranger" ? 200 : srv.mode === "page403" ? 403 : 404, null);
        }
        p.views++;
        if (srv.mode === "gone" || srv.mode === "stranger" || srv.mode === "page403") return reply(404, null);
        if (srv.mode === "kicked") return reply(403, null);
        return reply(200, { version: srv.version, theme: "t", theme_rev: srv.rev, theme_hash: srv.hash, html: srv.html });
      },
      EventSource: class { constructor(u) { this.u = u; this.on = {}; p.streams.push(this); } addEventListener(n, f) { this.on[n] = f; } close() { this.closed = true; } } };
    const link = { addEventListener(n, f) { p.link[n] = f; }, setAttribute(k, v) { p.hrefs.push(v); } };
    let src;
    const api = { $: () => link, ended: () => p.ended.length > 0, current: () => p.cur, show: async v => { if (themed) src.applyTheme(v); p.cur = v; p.shows++; }, failed() { p.fails++; }, themeFailed() {},
      end(r) { p.ended.push(r); }, hello() {}, slide() {}, pos() {}, redraw() {}, files() {}, nav() {}, paused: (st, r) => { p.bands.push(st); p.retryFn = r; }, holdReload: () => !!(typeof hold === "function" ? hold() : hold) };
    src = S.hostSource("/b", w).attach(api);
    src.start();
    await settle();
    p.timers.forEach(t => { t.off = true; }); // 테마 CSS 를 기다리는 시계 — 이 시험의 재시도가 아니다
    p.last = () => p.streams[p.streams.length - 1];
    p.hello = async d => { p.last().on.hello({ data: JSON.stringify(d) }); await settle(); };
    p.drop = async () => { p.last().onerror(); await settle(); };
    p.stop = async reason => { p.last().on.stop({ data: JSON.stringify({ reason }) }); await settle(); };
    p.retry = async () => { const t = p.timers.filter(x => !x.off).pop(); t.off = true; t.f(); await settle(); };
    p.live = () => p.timers.filter(x => !x.off).length;
    return p;
  };
  const server = () => ({ mode: "ok", version: 1, html: "읽던 문서" });
  const hello = { version: 1, theme: "t", keep: true };
  (async () => {
    // paused 로 닫힘 → 404 동안 재시도 → 다시 떠서 바뀐 문서를 받고 띠가 사라짐, 새로 고침 없음
    let srv = server(), p = await page(srv);
    await p.hello(hello);
    srv.mode = "gone";
    await p.stop("paused");
    eq("paused: 띠·끝나지 않음·문서 남음", [p.bands, p.ended, p.cur.html, p.live()], [["closed"], [], "읽던 문서", 1]);
    const streams = p.streams.length;
    await p.retry();
    eq("재시도: 스트림을 다시 엶", p.streams.length, streams + 1);
    await p.drop();
    eq("404 는 끝이 아니다", [p.ended, p.bands, p.live()], [[], ["closed"], 1]);
    srv.mode = "down";
    await p.retry(); await p.drop();
    eq("망 실패도 재시도, 안내 없음", [p.ended, p.fails, p.live()], [[], 0, 1]);
    srv.mode = "ok"; srv.version = 2; srv.html = "고친 문서";
    await p.retry();
    await p.hello({ version: 2, theme: "t", keep: true });
    eq("복귀: 바뀐 문서·띠 지움·새로 고침 없음", [p.cur.html, p.bands, p.reloads, p.ended], ["고친 문서", ["closed", ""], 0, []]);
    await p.stop("paused");
    eq("다시 닫혀도 같다", [p.bands.slice(2), p.ended], [["closed"], []]);

    // 쿠키를 잃음: 재시도가 지금 주소를 살펴 200 이면 새로 고친다. 공유가 없으면(404) 새로 고치지 않는다 — 꺼진 서버로 새로 고치면 브라우저 오류 화면이다
    srv = server(); p = await page(srv);
    await p.hello(hello);
    srv.mode = "gone"; await p.stop("paused");
    eq("쿠키: 공유가 없으면 재시도", [p.pages, p.reloads, p.ended, p.live()], [["GET http://h/b?doc=a.md"], 0, [], 1]);
    srv.mode = "stranger";
    await p.retry(); await p.drop();
    eq("쿠키: 200 이면 새로 고침", [p.reloads, p.ended, p.pages.length], [1, [], 2]);
    srv = server(); p = await page(srv);
    await p.hello(hello);
    srv.mode = "gone"; await p.stop("paused");
    srv.mode = "page403"; await p.retry(); await p.drop();
    eq("쿠키: 지금 주소 403 은 끝", [p.ended, p.reloads, p.live()], [["kicked"], 0, 0]);
    srv = server(); p = await page(srv);
    await p.hello(hello);
    srv.mode = "down"; await p.drop();
    eq("쿠키: 망 실패는 살피지 않음", [p.pages, p.reloads], [[], 0]);
    srv = server(); p = await page(srv);
    await p.hello(hello);
    await p.drop();
    srv.mode = "ok"; await p.retry();
    eq("쿠키: view.json 이 되면 살피지 않음", [p.pages, p.reloads], [[], 0]);

    // 새로 고침이 메모를 지울 때는 띠로 안내한다
    srv = server(); p = await page(srv, true);
    await p.hello(hello);
    srv.mode = "stranger"; await p.stop("paused");
    eq("메모가 화면에만: 새로 고치지 않고 안내", [p.reloads, p.bands, p.ended, p.live()], [0, ["closed", "held"], [], 1]);

    // 테마는 내용 지문으로 견준다: 서버를 다시 띄워 판 번호가 달라져도 지문이 같으면 새로 고치지 않고 문서만 바꾼다
    const themeHello = (hash, rev) => ({ version: 2, theme: "t", keep: true, theme_hash: hash, theme_rev: rev });
    const themedPage = async (hold, hash) => {
      const s = server(); s.hash = hash || "aaa"; s.rev = 5;
      const q = await page(s, hold, true);
      await q.hello({ version: 1, theme: "t", keep: true, theme_hash: s.hash, theme_rev: 5 });
      s.mode = "gone"; await q.stop("paused");
      return [s, q];
    };
    [srv, p] = await themedPage();
    eq("CSS 주소는 지문", p.hrefs, ["/b/_/_theme/t/theme.css?v=aaa"]);
    srv.mode = "ok"; srv.rev = 1; srv.version = 2; srv.html = "고친 문서";
    await p.retry(); await p.hello(themeHello("aaa", 1));
    eq("같은 지문·다른 판: 새로 고침 없음", [p.reloads, p.cur.html, p.hrefs.length, p.bands], [0, "고친 문서", 1, ["closed", ""]]);

    // 지문이 다르면 판 번호가 같아도(재시작한 프로세스의 같은 번호) 새로 고친다
    [srv, p] = await themedPage();
    srv.mode = "ok"; srv.hash = "bbb";
    await p.retry(); await p.hello(themeHello("bbb", 5));
    eq("다른 지문·같은 판: 새로 고침", [p.reloads, p.cur.html], [1, "읽던 문서"]);

    // 쿠키 정상 + 저장 불가 메모 + 닫힌 사이 테마 변경: 새로 고치지 않고 문서를 두고 안내한다
    [srv, p] = await themedPage(true);
    srv.mode = "ok"; srv.hash = "bbb"; srv.version = 2;
    await p.retry(); await p.hello(themeHello("bbb", 5));
    eq("메모 보호: hello", [p.reloads, p.bands, p.cur.html, p.ended], [0, ["closed", "", "held"], "읽던 문서", []]);
    p.last().on.version({ data: JSON.stringify({ version: 3, theme: "t", theme_hash: "ccc" }) }); await settle();
    p.last().on.reload({}); await settle();
    eq("메모 보호: version·reload", [p.reloads, p.cur.html], [0, "읽던 문서"]);
    p.link.error(); await settle();
    eq("메모 보호: CSS 실패", p.reloads, 0);
    [srv, p] = await themedPage(true);
    srv.mode = "ok"; srv.hash = "bbb";
    await p.retry(); await p.drop(); // hello 전에 다시 끊겨 복구의 view 가 먼저 온다
    eq("메모 보호: 복구의 view", [p.reloads, p.bands], [0, ["closed", "held"]]);

    // 2초 보호로 미룬 새로 고침도 그때 다시 묻는다
    let held = false;
    [srv, p] = await themedPage(() => held);
    srv.mode = "ok"; srv.hash = "bbb";
    await p.retry(); await p.hello(themeHello("bbb", 5));
    eq("첫 새로 고침", p.reloads, 1);
    p.last().on.reload({}); await settle();
    held = true;
    p.timers.filter(t => !t.off).pop().f(); await settle();
    eq("미룬 새로 고침도 막힘", [p.reloads, p.bands.includes("held")], [1, true]);

    // 같은 주소가 아닌 공유는 holdReload 를 묻지 않는다 — 지금처럼 새로 고친다
    srv = server(); srv.hash = "aaa"; p = await page(srv, true, true);
    await p.hello({ version: 1, theme: "t", theme_hash: "aaa" });
    p.last().on.reload({}); await settle();
    eq("keep 아님: 관문 없음", [p.reloads, p.bands], [1, []]);

    // 강제 종료: stop 없이 끊겨도 keep 공유면 띠
    srv = server(); p = await page(srv);
    await p.hello(hello);
    srv.mode = "down";
    await p.drop();
    eq("강제 종료: 띠", [p.bands, p.ended, p.fails, p.live()], [["closed"], [], 0, 1]);

    // 403 은 끝낸다
    srv = server(); p = await page(srv);
    await p.hello(hello);
    srv.mode = "gone"; await p.stop("paused");
    srv.mode = "kicked"; await p.retry(); await p.drop();
    eq("403: 끝남", [p.ended, p.live()], [["kicked"], 0]);

    // 끝남 stop·kick 은 keep 공유에서도 끝이다(주소 버리기)
    srv = server(); p = await page(srv);
    await p.hello(hello);
    await p.stop("address_dropped");
    eq("주소 버리기: 끝남", [p.ended, p.bands], [["address_dropped"], []]);
    srv = server(); p = await page(srv);
    await p.hello(hello);
    p.last().on.kick({}); await settle();
    eq("kick: 끝남", [p.ended, p.bands], [["kicked"], []]);

    // 붙어 있는 동안 version 의 view.json 이 404 면 끝이 아니라 잠시 닫힘이다
    srv = server(); p = await page(srv);
    await p.hello(hello);
    srv.mode = "gone";
    p.last().on.version({ data: JSON.stringify({ version: 9, theme: "t" }) }); await settle();
    eq("연결 중 404: 잠시 닫힘", [p.ended, p.bands, p.last().closed, p.live()], [[], ["closed"], true, 1]);

    // 붙어 있는 동안의 403 은 끝이다
    srv = server(); p = await page(srv);
    await p.hello(hello);
    srv.mode = "kicked";
    p.last().on.version({ data: JSON.stringify({ version: 9, theme: "t" }) }); await settle();
    eq("연결 중 403: 끝남", [p.ended, p.bands], [["kicked"], []]);

    // 화면이 다시 보이면 바로 붙는다
    srv = server(); p = await page(srv);
    await p.hello(hello);
    srv.mode = "gone"; await p.stop("paused");
    const n = p.streams.length;
    p.wakes[0](); await settle();
    eq("wake: 바로 붙음", p.streams.length, n + 1);

    // 띠의 「지금 다시 잇기」(itda-hyve#728 후속): 기다리던 타이머를 지우고 바로 붙는다. 붙는 중에 또 눌러도 스트림 하나, 이번 시도가 끝나면 풀린다
    srv = server(); p = await page(srv);
    await p.hello(hello);
    srv.mode = "gone"; await p.stop("paused");
    let m = p.streams.length;
    eq("단추: closed 띠에 다시 잇기 함수", [typeof p.retryFn, p.live()], ["function", 1]);
    let first = p.retryFn(), again = p.retryFn();
    eq("단추: 타이머를 지우고 바로 붙음·두 번 눌러도 스트림 하나", [p.streams.length, p.live()], [m + 1, 0]);
    await p.drop();
    eq("단추: 실패하면 false 로 풀리고 다음 시도", [await first, await again, p.live(), p.ended], [false, false, 1, []]);
    // online·visible 도 잠시 닫힘이면 기다리지 않고 한 번 붙어 본다
    m = p.streams.length;
    p.online(); await settle();
    eq("online: 타이머를 지우고 바로 붙음", [p.streams.length, p.live()], [m + 1, 0]);
    await p.drop();
    m = p.streams.length;
    p.wakes[1](); await settle();
    eq("visible: 타이머를 지우고 바로 붙음", [p.streams.length, p.live()], [m + 1, 0]);
    await p.drop();
    srv.mode = "ok";
    first = p.retryFn();
    await p.hello(hello);
    eq("단추: 붙으면 true·띠 지움", [await first, p.bands[p.bands.length - 1], p.retryFn], [true, "", undefined]);
    // 잠시 닫힘이 아닌 끊김: online 은 backoff 를 앞당기지 않는다
    srv = server(); p = await page(srv);
    await p.hello({ version: 1, theme: "t" });
    srv.mode = "down"; await p.drop();
    m = p.streams.length;
    p.online(); await settle();
    eq("keep 아님: online 은 그대로", [p.streams.length, p.live()], [m, 1]);
    // 끝난 뒤에는 아무것도 하지 않는다
    srv = server(); p = await page(srv);
    await p.hello(hello);
    srv.mode = "gone"; await p.stop("paused");
    const retry = p.retryFn;
    srv.mode = "kicked"; await p.retry(); await p.drop();
    m = p.streams.length;
    const late403 = retry(); p.online(); p.wakes[1](); await settle();
    eq("끝난 뒤: 붙지 않음", [await late403, p.streams.length, p.ended], [false, m, ["kicked"]]);

    // 24시간: 재시도를 멈추고 안내로 바꾼다
    let shift = 0, c0 = null;
    const t0 = 1000, late = [], ended = [], seen = [];
    const c = S.connect("/b", { EventSource: class { constructor() { c0 = this; this.on = {}; } addEventListener(n, f) { this.on[n] = f; } close() {} },
      probe: async () => "ended", setTimeout: f => { late.push(f); return late.length; }, clearTimeout() {}, now: () => t0 + shift,
      onHello() {}, onEnded: r => ended.push(r), onPaused: s => seen.push(s) });
    await settle();
    c0.on.hello({ data: JSON.stringify({ keep: true }) });
    c0.on.stop({ data: JSON.stringify({ reason: "paused" }) }); await settle();
    eq("24시간 전: 재시도", [seen, late.length], [["closed"], 1]);
    shift = DAY - 1; late.shift()(); c0.onerror(); await settle();
    eq("24시간 직전: 재시도", [seen, late.length], [["closed"], 1]);
    shift = DAY; late.shift()(); c0.onerror(); await settle();
    eq("24시간: 멈추고 안내", [seen, late.length, ended], [["closed", "stale"], 0, []]);
    const before = c.state;
    c.wake(); await settle();
    eq("멈춘 뒤 wake 는 붙지 않음", [before, c.state], ["idle", "idle"]);
    // 멈춘 뒤의 단추: 지금부터 다시 PAUSE_MAX 동안 시도한다
    const restart = c.retryNow(); await settle();
    eq("stale 에서 다시 시작", [seen, c.state, c.paused], [["closed", "stale", "closed"], "connecting", t0 + DAY]);
    shift = 2 * DAY - 1; c0.onerror(); await settle();
    eq("다시 시작한 뒤 재시도", [await restart, late.length, seen.length], [false, 1, 3]);

    // keep 이 아닌 공유는 그대로다: 끊김 뒤 404 는 끝, 띠 없음, stop 은 끝
    srv = server(); p = await page(srv);
    await p.hello({ version: 1, theme: "t" });
    srv.mode = "gone"; await p.drop();
    eq("keep 아님: 끊김 뒤 404 는 끝", [p.ended, p.bands, p.live(), p.pages], [[""], [], 0, []]);
    srv = server(); p = await page(srv);
    await p.hello({ version: 1, theme: "t" });
    await p.stop("user");
    eq("keep 아님: stop 은 끝", [p.ended, p.bands], [["user"], []]);
    srv = server(); p = await page(srv);
    await p.hello({ version: 1, theme: "t" });
    srv.mode = "down"; await p.drop();
    eq("keep 아님: 망 실패는 띠 없이 다시 붙기", [p.ended, p.bands, p.live()], [[], [], 1]);

    process.stdout.write(bad.length ? "FAIL " + bad.join("; ") : "PAUSED OK");
  })().catch(e => process.stdout.write("FAIL " + e.stack));
`
	out, err := exec.Command(node, "-e", js).CombinedOutput()
	if err != nil || string(out) != "PAUSED OK" {
		t.Fatalf("잠시 닫힘: %v\n%s", err, out)
	}
}

// 잠시 닫힘 띠: 문서를 지우는 끝난 화면을 부르지 않는다 — 띠는 share.css 에서 흐름 밖에 떠 있어 쪽 나눔·굴림 자리를 건드리지 않는다. 자리는 share.js 가 처음 쓸 때 .main 에 만든다.
func TestSharePauseBandParts(t *testing.T) {
	read := func(n string) string {
		b, err := os.ReadFile(n)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	css := read("share.css")
	if !strings.Contains(css, ".pause-band { position: absolute;") || !strings.Contains(css, ".pause-band[hidden] { display: none; }") {
		t.Error("띠가 흐름 밖에 있어야 함")
	}
	src := read("share.js")
	a := strings.Index(src, "    paused: (st, retry) => {\n")
	z := strings.Index(src[max(a, 0):], "\n    },\n")
	if a < 0 || z < 0 {
		t.Fatal("share.js 에서 paused 를 찾지 못함")
	}
	body := src[a : a+z]
	if strings.Contains(body, "showEnded") || strings.Contains(body, `"pages"`) {
		t.Errorf("띠가 문서를 건드림:\n%s", body)
	}
	// 「지금 다시 잇기」(itda-hyve#728 후속): 키보드로 닿는 button, 알림(role="status")은 글 칸에만, 띠가 누름을 흘려보내도 단추는 받는다
	if !strings.Contains(body, `createElement("button")`) || !strings.Contains(body, `msg.setAttribute("role", "status")`) || strings.Contains(body, `band.setAttribute("role"`) {
		t.Errorf("단추·알림 자리가 틀림:\n%s", body)
	}
	if !strings.Contains(css, ".pause-retry { pointer-events: auto;") {
		t.Error("단추가 누름을 받아야 함")
	}
}

// 글꼴을 받은 뒤 나누기(itda-doc#62): share.js 의 render·draw·fontWait·fontsLate 정의를 그대로 떼어 node 로 돌린다. 화면 부품은 가짜(이름을 모르면 빈 함수)이고
// 나누기는 ItdaSlides.build 호출로 센다. 글꼴·타이머는 시험이 손으로 끝낸다.
func TestShareFontsBeforeSplit(t *testing.T) {
	b, err := FS.ReadFile("share.js") // 내장 FS 로 읽어 go test -overlay 변이가 닿는다
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	cut := func(head, tail string) string {
		a := strings.Index(src, head)
		if a < 0 {
			t.Fatalf("share.js 에서 %q 를 찾지 못함", head)
		}
		z := strings.Index(src[a:], tail)
		if z < 0 {
			t.Fatalf("share.js 에서 %q 의 끝을 찾지 못함", head)
		}
		return src[a : a+z+len(tail)]
	}
	decl := cut("  const fontWait = fontGate(", ";\n") + cut("  function fontsLate() {", "\n  }\n") +
		cut("  async function render(v, sort) {", "\n  }\n") + cut("  async function draw(sort) {", "\n  }\n")
	node := needNode(t)
	js := `
  const vm = require("vm");
  const ctx = { console }; ctx.window = ctx; vm.createContext(ctx);
  vm.runInContext(process.env.SRC, ctx);
  const S = ctx.window.ItdaShare, bad = [];
  const eq = (name, got, want) => { const a = JSON.stringify(got), b = JSON.stringify(want); if (a !== b) bad.push(name + ": " + a + " ≠ " + b); };
  const settle = async () => { for (let i = 0; i < 8; i++) await new Promise(r => setImmediate(r)); };
  const noop = () => {};
  // face 는 끝나지 않은 글꼴이다. arrive·fail 로 끝낸다. load 를 부르기 전에는 끝낼 수 없다 — load 를 안 하면 글꼴은 오지 않는다
  const face = (log, name, status) => {
    const f = { status: status || "unloaded", loads: 0 };
    const p = new Promise((ok, no) => { f.arrive = () => { f.status = "loaded"; ok(f); }; f.fail = () => { f.status = "error"; no(new Error("404")); }; });
    p.catch(noop);
    f.load = () => { f.loads++; log.push("load " + name); return p; };
    return f;
  };
  // el 은 어떤 메서드를 불러도 되는 가짜 요소다
  const element = () => new Proxy({ querySelectorAll: () => [], querySelector: () => null, firstChild: null, childElementCount: 0, scrollTop: 0, textContent: "", style: {}, dataset: {} }, {
    get: (t, k) => k in t ? t[k] : k === "classList" || k === "style" ? { toggle: noop, add: noop, remove: noop, setProperty: noop, removeProperty: noop } : noop,
    set: (t, k, val) => { t[k] = val; return true; },
  });
  const load = new Function("env", "with (env) { " + process.env.DECL + " return { render, draw, fontsLate }; }");
  function rig(faces) {
    const log = [], r = { log, timers: [], builds: [], sorts: [], ticks: 0, imgs: null };
    r.faces = faces(log);
    const set = r.faces; set.ready = Promise.resolve();
    const el = element();
    r.state = { view: null, look: "", chosen: "", ratio: "16:9", ended: false, rendering: null, again: false, slideNotes: [], hasNotes: false, tableSort: null, fontStale: false };
    const env = { state: r.state, fontGate: S.fontGate, applyTheme: async () => true, $: () => el, docTitle: () => "", app: "", source: {}, fol: { active: () => false },
      findHeading: { refresh: noop }, jumper: { jump: noop }, TableSort: { mark: noop, apply: (f, sort) => { r.sorts.push(sort); return true; } }, CodeLines: null, CodeCopy: null, boardZoom: null, hook: name => { if (name === "tick") r.ticks++; }, imagesSettled: () => r.imgs ? r.imgs.done : null,
      lookFor: (v, w, chosen) => v.look || chosen || "slides",
      document: { fonts: set, title: "", body: el, documentElement: el, createElement: () => element(), createDocumentFragment: () => element(), querySelectorAll: () => [] },
      setTimeout: (f, ms) => { r.timers.push({ f, ms }); return r.timers.length; },
      window: { innerWidth: 1200, ItdaPager: { numberBlocks: noop, prepareTOC: noop, adoptWidths: () => { r.builds.push("flow"); log.push("build"); }, paginate: (f, t, o) => { r.builds.push("page"); log.push("build"); } },
        ItdaSlides: { SIZE: { "16:9": [960, 540], "4:3": [960, 720] }, placeNotes: noop, build: () => { r.builds.push("slides"); log.push("build"); return {}; } },
        ItdaBoard: { build: () => { r.builds.push("board"); log.push("build"); } } } };
    const scope = new Proxy(env, { has: () => true, get: (t, k) => k === Symbol.unscopables ? undefined : k in t ? t[k] : k in globalThis ? globalThis[k] : noop });
    Object.assign(r, load(scope));
    r.show = v => { r.state.view = v; return r.draw(); };
    r.expire = async () => { r.timers.splice(0).forEach(t => t.f()); await settle(); };
    r.holdImages = () => { let open; r.imgs = { done: new Promise(ok => { open = ok; }) }; r.imgs.open = () => { open(); r.imgs = null; }; };
    return r;
  }
  const v = { html: "" };
  (async () => {
    // build 전에 모든 face 의 load 가 끝난다 — 안 쓰는 face 까지
    let r = rig(l => [face(l, "본문"), face(l, "테마")]);
    r.show(v); await settle();
    eq("끝나기 전에는 나누지 않음", r.builds, []);
    eq("모든 face 를 받기 시작", r.faces.map(f => f.loads), [1, 1]);
    eq("상한", r.timers.map(t => t.ms), [5000]);
    r.faces[0].arrive(); await settle();
    eq("하나만 왔을 때", r.builds, []);
    r.faces[1].arrive(); await settle();
    eq("다 오면 나눔", r.log, ["load 본문", "load 테마", "build"]);
    await r.expire();
    eq("끝난 뒤 지난 타이머는 할 일이 없음", r.builds.length, 1);
    r.fontsLate(); await settle();
    eq("끝난 뒤에는 다시 그리지 않음", r.builds.length, 1);

    // 쪽·보드·이어보기도 같다 — 기다린 뒤 그리고, 상한을 넘기면 도착 뒤 다시 그린다(이어보기는 adoptWidths 로 센다)
    for (const [look, kind] of [["doc", "page"], ["dashboard", "board"], ["flow", "flow"]]) {
      r = rig(l => [face(l, "본문")]);
      r.show({ html: "", look }); await settle();
      eq(look + ": 기다림", r.builds, []);
      r.faces[0].arrive(); await settle();
      eq(look + ": 나눔", r.builds, [kind]);
      r = rig(l => [face(l, "본문")]);
      r.show({ html: "", look }); await settle(); await r.expire();
      eq(look + ": 상한 뒤 먼저 그림", r.builds, [kind]);
      r.faces[0].arrive(); await settle(); await r.expire();
      eq(look + ": 도착 뒤 다시 그림", r.builds, [kind, kind]);
    }

    // fonts.ready 가 남아 있으면 끝날 때까지 기다린다
    r = rig(l => [face(l, "본문")]);
    let readyOK;
    r.faces.ready = new Promise(ok => { readyOK = ok; });
    r.show(v); await settle();
    r.faces[0].arrive(); await settle();
    eq("ready 전", r.builds, []);
    readyOK(); await settle();
    eq("ready 뒤", r.builds, ["slides"]);

    // 실패한 face 는 막지 않는다
    r = rig(l => [face(l, "깨짐"), face(l, "본문")]);
    r.show(v); await settle();
    r.faces[0].fail(); r.faces[1].arrive(); await settle();
    eq("reject 된 face 가 있어도 진행", r.builds, ["slides"]);
    eq("reject 뒤 다시 그리지 않음", [r.state.fontStale, r.timers.filter(t => t.ms !== 5000)], [false, []]);

    // 이미 끝난 글꼴은 지연이 없다 — 타이머도 없다
    r = rig(l => [face(l, "본문", "loaded"), face(l, "깨짐", "error")]);
    r.faces[0].arrive(); r.faces[1].fail();
    await r.show(v);
    eq("끝난 글꼴: 바로 나눔", [r.builds, r.timers.length], [["slides"], 0]);

    // 상한을 넘기면 먼저 그리고, 도착하면 한 번만 다시 그린다
    r = rig(l => [face(l, "본문")]);
    r.show(v); await settle();
    await r.expire();
    eq("상한 뒤 먼저 그림", r.builds.length, 1);
    r.faces[0].arrive(); await settle(); await r.expire();
    eq("도착 뒤 한 번 다시 그림", r.builds.length, 2);
    eq("다시 그린 뒤 표시 없음", [r.state.fontStale, r.timers.length], [false, 0]);
    r.fontsLate(); await settle();
    eq("두 번 다시 그리지 않음", r.builds.length, 2);

    // 하나가 끝나지 않아도 도착한 face 마다 다시 나눈다
    r = rig(l => [face(l, "본문"), face(l, "제목"), face(l, "안 끝남")]);
    r.show(v); await settle(); await r.expire();
    r.faces[0].arrive(); await settle(); await r.expire();
    eq("끝나지 않는 face 가 있어도 다시 나눔", r.builds.length, 2);
    eq("다시 나눈 뒤 남은 face 를 기다리지 않음", r.timers.length, 0);
    r.faces[1].arrive(); await settle(); await r.expire();
    eq("다시 그린 뒤 도착한 face 로 또 나눔", r.builds.length, 3);
    eq("끝나지 않는 face 를 또 받지 않음", r.faces[2].loads, 2);

    // 몰려 도착하면 합친다
    r = rig(l => [face(l, "가"), face(l, "나"), face(l, "다"), face(l, "안 끝남")]);
    r.show(v); await settle(); await r.expire();
    r.faces[0].arrive(); r.faces[1].arrive(); await settle();
    r.faces[2].arrive(); await settle();
    eq("합치는 동안 그리지 않음", [r.builds.length, r.timers.length], [1, 1]);
    await r.expire();
    eq("몰린 도착은 한 번", r.builds.length, 2);

    // 도착이 실패면 나눔이 바뀌지 않는다
    r = rig(l => [face(l, "깨짐")]);
    r.show(v); await settle(); await r.expire();
    r.faces[0].fail(); await settle(); await r.expire();
    eq("상한 뒤 실패는 다시 그리지 않음", r.builds.length, 1);

    // 상한을 넘긴 글꼴은 다음 그리기를 또 늦추지 않고, 여러 번 그려도 도착 뒤 한 번만 다시 그린다
    r = rig(l => [face(l, "본문")]);
    r.show(v); await settle(); await r.expire();
    await r.draw();
    eq("상한을 넘긴 face 는 또 기다리지 않음", [r.builds.length, r.timers.length], [2, 0]);
    r.faces[0].arrive(); await settle(); await r.expire();
    eq("여러 번 그렸어도 도착 뒤 한 번", r.builds.length, 3);

    // 새 face(늦은 테마 CSS)는 그 face 만 기다린다
    r = rig(l => [face(l, "안 끝남")]);
    r.faces.ready = new Promise(noop); // 끝나지 않는 face 가 있는 동안 브라우저의 ready 는 풀리지 않는다
    r.show(v); await settle(); await r.expire();
    r.faces.push(face(r.log, "테마"));
    r.draw(); await settle();
    eq("새 face 를 기다림", [r.builds.length, r.timers.map(t => t.ms)], [1, [5000]]);
    eq("새 face 를 받기 시작", r.faces[1].loads, 1);
    r.faces[1].arrive(); await settle();
    eq("새 face 가 오면 상한을 또 쓰지 않고 나눔", r.builds.length, 2);
    await r.expire();
    eq("새 face 로 나눴으니 다시 그리지 않음", r.builds.length, 2);

    // 나누는 동안 처음 나타난 face 도 도착하면 다시 나눈다
    r = rig(l => []);
    r.holdImages();
    r.show(v); await settle();
    r.faces.push(face(r.log, "늦은 선언"));
    r.imgs.open(); await settle();
    eq("늦은 선언: 먼저 그림", [r.builds.length, r.faces[0].loads], [1, 1]);
    r.faces[0].arrive(); await settle(); await r.expire();
    eq("늦은 선언: 도착 뒤 다시 그림", r.builds.length, 2);

    // 렌더 도중 도착: 나누기 전이면 그 렌더가 글꼴을 쓴다 — 추가 렌더도, 정렬 풀림도 없다
    r = rig(l => [face(l, "본문")]);
    r.show({ html: "", look: "doc" }); await settle(); await r.expire();
    r.holdImages();
    const sorting = r.draw({ col: 1 }); await settle();
    r.faces[0].arrive(); await settle(); await r.expire();
    r.imgs.open(); await sorting; await settle(); await r.expire();
    eq("렌더 도중 도착: 정렬", [r.sorts, r.state.tableSort], [[{ col: 1 }], { col: 1 }]);
    eq("렌더 도중 도착: 추가 렌더 없음", [r.builds.length, r.state.again, r.state.fontStale], [2, false, false]);

    // 렌더 도중 일부만 왔으면 다음 도착이 정렬을 이어 다시 나눈다
    r = rig(l => [face(l, "본문"), face(l, "제목")]);
    r.show({ html: "", look: "doc" }); await settle(); await r.expire();
    r.holdImages();
    const part = r.draw({ col: 2 }); await settle();
    r.faces[0].arrive(); await settle(); await r.expire();
    r.imgs.open(); await part; await settle();
    eq("일부 도착: 표시 남음", [r.builds.length, r.state.fontStale], [2, true]);
    r.faces[1].arrive(); await settle(); await r.expire();
    eq("일부 도착: 정렬 이음", [r.builds.length, r.sorts, r.state.tableSort], [3, [{ col: 2 }, { col: 2 }], { col: 2 }]);

    // 공유가 끝났으면 다시 그리지 않는다
    r = rig(l => [face(l, "본문")]);
    r.show(v); await settle(); await r.expire();
    r.state.ended = true;
    r.faces[0].arrive(); await settle(); await r.expire();
    eq("끝난 공유", r.builds.length, 1);

    // 문서가 바뀌어 새 그리기가 글꼴을 받고 나눴으면 다시 그리지 않는다
    r = rig(l => [face(l, "본문")]);
    r.show(v); await settle(); await r.expire();
    r.faces.push(face(r.log, "새 문서"));
    r.show({ html: "새 판" }); await settle();
    r.faces[1].arrive(); r.faces[0].arrive(); await settle(); await r.expire();
    eq("바뀐 문서", r.builds.length, 2);

    // 오류 view 로 바뀌었으면 글꼴이 와도 그 화면을 다시 그리지 않는다
    r = rig(l => [face(l, "본문")]);
    r.show(v); await settle(); await r.expire();
    r.show({ error: "문서 없음" }); await settle();
    eq("오류 view 그림", [r.builds.length, r.ticks], [1, 1]);
    r.faces[0].arrive(); await settle(); await r.expire();
    eq("오류 view 를 다시 그리지 않음", [r.builds.length, r.ticks], [1, 1]);

    // 바뀐 문서도 대체 글꼴로 나눴으면 그 문서를 다시 나눈다
    r = rig(l => [face(l, "본문")]);
    r.show(v); await settle(); await r.expire();
    r.show({ html: "새 판" }); await settle();
    eq("바뀐 문서 먼저 그림", r.builds.length, 2);
    r.faces[0].arrive(); await settle(); await r.expire();
    eq("바뀐 문서를 한 번 다시 나눔", r.builds.length, 3);

    // 다시 그려도 사람이 고른 표 정렬은 잇는다
    r = rig(l => [face(l, "본문")]);
    r.show({ html: "", look: "doc" }); await settle(); await r.expire();
    r.state.tableSort = { col: 1 };
    r.faces[0].arrive(); await settle(); await r.expire();
    eq("다시 그린 뒤 표 정렬", [r.sorts, r.state.tableSort], [[{ col: 1 }], { col: 1 }]);

    process.stdout.write(bad.length ? "FAIL " + bad.join("; ") : "FONTS OK");
  })().catch(e => process.stdout.write("FAIL " + e.stack));
`
	cmd := exec.Command(node, "-e", js)
	cmd.Env = append(os.Environ(), "DECL="+decl, "SRC="+src)
	out, err := cmd.CombinedOutput()
	if err != nil || string(out) != "FONTS OK" {
		t.Fatalf("글꼴을 받은 뒤 나누기: %v\n%s", err, out)
	}
}

// 호스트 꼴 적용(itda-hyve#748): lookFor 표, setLook·꼴 단추·release 의 forced 수명, 발표 흐름. share.js 의 fol 선언·setLook·꼴 단추 처리를 소스 그대로 떼어 진짜 follower·followAction·restoreOwnLook·lookSwitchHidden 위에서 돌린다.
// draw 는 render 의 꼴 정하기만 한다(소스에서 render 가 state.forced 를 넘기는지는 글로 본다). 실제 화면은 미검증.
func TestShareForcedLook(t *testing.T) {
	b, err := FS.ReadFile("share.js") // 내장 FS 로 읽어 go test -overlay 변이가 닿는다
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	cut := func(from, to string) string {
		t.Helper()
		a := strings.Index(src, from)
		z := strings.Index(src[max(a, 0):], to)
		if a < 0 || z < 0 {
			t.Fatalf("share.js 에서 %q 를 찾지 못함", from)
		}
		return src[a : a+z+len(to)]
	}
	for _, m := range []string{
		`state.look = lookFor(v, window.innerWidth, state.chosen, state.forced);`,
		`chosen: "", forced: false,`,
	} {
		if !strings.Contains(src, m) {
			t.Errorf("share.js 에 %s 가 없다", m)
		}
	}
	if !strings.Contains(cut("ext = extInit({", "})"), "setLook") {
		t.Error("확장 api 에 setLook 이 없다")
	}
	node := needNode(t)
	js := `
  const vm = require("vm");
  const ctx = { console }; ctx.window = ctx; vm.createContext(ctx);
  vm.runInContext(process.env.SRC, ctx);
  const S = ctx.window.ItdaShare, bad = [];
  const eq = (name, got, want) => { const a = JSON.stringify(got), b = JSON.stringify(want); if (a !== b) bad.push(name + ": " + a + " ≠ " + b); };
  const doc = {}, slide = { slides: { present: true } }, board = { board: { present: true } }, both = { board: { present: true }, slides: { present: true } };

  // ---- lookFor: forced 없음은 기존 값 그대로
  const table = [
    [doc, 1200, "", "doc"], [doc, 500, "", "flow"], [doc, 1200, "flow", "flow"], [doc, 500, "doc", "doc"], [doc, 1200, "slides", "slides"], [doc, 1200, "dashboard", "dashboard"],
    [slide, 1200, "", "slides"], [slide, 500, "", "slides"], [slide, 1200, "doc", "slides"], [slide, 1200, "flow", "slides"], [slide, 500, "flow", "slides"], [slide, 1200, "slides", "slides"], [slide, 1200, "dashboard", "dashboard"],
    [board, 1200, "", "dashboard"], [board, 500, "", "dashboard"], [board, 1200, "doc", "dashboard"], [board, 1200, "flow", "dashboard"], [board, 1200, "slides", "slides"], [board, 1200, "dashboard", "dashboard"],
    [both, 1200, "", "dashboard"], [both, 1200, "flow", "dashboard"], [both, 1200, "slides", "slides"],
  ];
  for (const [v, w, c, want] of table) {
    const name = JSON.stringify(v) + " " + w + " " + JSON.stringify(c);
    eq("forced 없음 " + name, S.lookFor(v, w, c), want);
    eq("forced 거짓 " + name, S.lookFor(v, w, c, false), want);
    // forced 는 쪽·이어보기 chosen 만 바꾼다
    eq("forced 참 " + name, S.lookFor(v, w, c, true), c === "doc" || c === "flow" ? c : want);
  }
  for (const v of [slide, board, both]) for (const c of ["doc", "flow"]) for (const w of [1200, 500]) eq("forced 가 문서 꼴을 이김 " + JSON.stringify(v) + c + w, S.lookFor(v, w, c, true), c);
  S.setBoardOK(false);
  eq("boardOK 거짓: forced 이어보기", S.lookFor(board, 1200, "flow", true), "flow");
  eq("boardOK 거짓: forced 쪽(슬라이드 문서)", S.lookFor(both, 1200, "doc", true), "doc");
  eq("boardOK 거짓: forced 없는 보드 문서", S.lookFor(board, 1200, ""), "doc");
  eq("boardOK 거짓: 발표 꼴 dashboard 는 forced 여도 고르지 않음", S.lookFor(both, 1200, "dashboard", true), "slides");
  S.setBoardOK(true);

  // ---- 호스트 꼴이 아닌 길은 슬라이드 문서를 바꾸지 않는다: view.json 의 look·폴더의 기억한 꼴·좁은 화면의 처음 꼴
  eq("view.json look", S.lookFor(slide, 1200, S.initialLook("", "", "flow")), "slides");
  eq("기억한 꼴", S.lookFor(board, 1200, S.initialLook("", "doc", "flow")), "dashboard");
  eq("좁은 화면 처음 꼴", S.lookFor(slide, 500, "flow"), "slides");
  eq("일반 문서 view.json look", S.lookFor(doc, 500, S.initialLook("", "", "doc")), "doc");
  eq("일반 문서 기억한 꼴", S.lookFor(doc, 1200, S.initialLook("", "flow", "doc")), "flow");

  // ---- 화면: 소스 그대로
  const make = new Function("follower", "followAction", "restoreOwnLook", "lookSwitchHidden", "state", "$", "window", "draw", "scrollToFollow", "paintLookSwitch", "nav", "b",
    process.env.DECL + "\n" + process.env.SET + "\nconst click = []; const on = (n, f) => click.push(f); const btn = { dataset: b.dataset, addEventListener: on };\n" +
    "(b => { " + process.env.CLICK + " })(btn);\nreturn { fol, setLook, click: () => click[0]() };");
  function rig(view, chosen) {
    const r = { draws: 0, scrolls: 0, saved: [], label: { look: "doc" } };
    r.state = { ended: false, view, look: "", chosen: chosen || "", forced: false, rendering: null, again: false };
    const el = { hidden: true, textContent: "", classList: { remove() {} }, querySelectorAll: () => [] };
    const draw = () => { r.draws++; r.state.look = S.lookFor(r.state.view, 1200, r.state.chosen, r.state.forced); return Promise.resolve(); };
    const m = make(S.follower, S.followAction, S.restoreOwnLook, S.lookSwitchHidden, r.state, () => el, { innerWidth: 1200 }, draw, () => { r.scrolls++; }, () => {},
      { setLook: x => r.saved.push(x) }, { dataset: r.label });
    r.fol = m.fol; r.setLook = m.setLook; r.click = m.click;
    draw(); r.draws = 0;
    r.at = () => [r.state.look, r.state.chosen, r.state.forced];
    return r;
  }
  const off = { index: -1, total: 0, follow: false }, end = { index: -1, total: 0, follow: true };

  // setLook
  let r = rig(slide);
  r.setLook("flow");
  eq("setLook 이어보기", [r.at(), r.draws], [["flow", "flow", true], 1]);
  eq("setLook 은 기억한 꼴에 적지 않는다", r.saved, []);
  r.setLook("flow");
  eq("같은 꼴은 다시 그리지 않는다", [r.at(), r.draws], [["flow", "flow", true], 1]);
  r.setLook("doc");
  eq("setLook 쪽", [r.at(), r.draws], [["doc", "doc", true], 2]);
  r.setLook("slides");
  eq("setLook 슬라이드는 forced 를 내린다", r.at(), ["slides", "slides", false]);
  r.setLook("flow");
  r.setLook("dashboard");
  eq("setLook 보드도", r.at(), ["dashboard", "dashboard", false]);
  r = rig(doc);
  r.setLook("flow");
  eq("일반 문서 setLook", r.at(), ["flow", "flow", true]);
  r.state.rendering = Promise.resolve("그리는 중");
  eq("이미 그 꼴이면 그리는 중인 그림을 돌려준다", r.setLook("flow"), r.state.rendering);

  // 꼴 단추: 사람이 고르면 forced 를 내리고 기억한다
  r = rig(doc);
  r.setLook("flow");
  r.click();
  eq("단추", [r.at(), r.saved], [["doc", "doc", false], ["doc"]]);
  r = rig(doc);
  r.setLook("doc");
  r.click();
  eq("단추 — 그리지 않아도 forced 를 내린다", [r.at(), r.draws], [["doc", "doc", false], 0]);

  // release: 따라가기 끔
  for (const [name, v, own] of [["슬라이드", slide, "slides"], ["보드", board, "dashboard"]]) {
    r = rig(v);
    r.fol.onSlide(end);
    r.setLook("flow");
    r.fol.onSlide(off);
    eq(name + " 문서 따라가기 끔 → 문서 꼴", r.at(), [own, "", false]);
    eq(name + " 문서 끔은 한 번 그린다", r.draws, 2);
  }
  r = rig(doc);
  r.fol.onSlide(end);
  r.setLook("flow");
  const was = r.draws;
  r.fol.onSlide(off);
  eq("일반 문서 끔 → 꼴은 그대로, 표시만", [r.at(), r.draws], [["flow", "flow", false], was]);
  r = rig(slide);
  r.fol.onSlide(end);
  r.setLook("flow");
  r.state.ended = true;
  r.fol.onSlide(off);
  eq("끝난 화면은 건드리지 않는다", r.at(), ["flow", "flow", true]);

  // release: 따라가기가 켜진 채 위치만 없어짐 — 호스트의 꼴을 거두지 않는다(번호가 같아 다시 맞출 길이 없다)
  r = rig(slide);
  r.fol.onPos({ block: 3, frac: 0, total: 9, follow: true });
  r.setLook("flow");
  r.fol.onPos({ block: -1, frac: 0, total: 0, follow: true });
  eq("호스트가 다른 탭으로 감", r.at(), ["flow", "flow", true]);
  r.fol.hello({ slide: -1, slide_total: 0, pos: -1, pos_frac: 0, pos_total: 0, follow: true });
  eq("재접속 hello 에 위치가 없음", r.at(), ["flow", "flow", true]);

  // 발표 흐름: 슬라이드 문서를 이어보기로 바꾼 뒤 F5 → 발표 끝 → 호스트의 지금 꼴
  r = rig(slide);
  r.fol.onPos({ block: 3, frac: 0, total: 9, follow: true });
  r.setLook("flow");
  r.setLook("slides"); // 호스트가 슬라이드로 다시 그려 먼저 알린다(app.js presentNow)
  r.fol.onSlide({ index: 2, total: 5, look: "slides", follow: true });
  eq("F5: 슬라이드", r.at(), ["slides", "slides", false]);
  r.fol.onSlide(end);
  eq("발표 끝: 문서 꼴이 그 꼴이라 그대로", r.at(), ["slides", "slides", false]);
  r.setLook("flow");
  eq("발표 끝 뒤 호스트가 이어보기", r.at(), ["flow", "flow", true]);

  // 알림 차례가 달라 slide 가 먼저 와도 forced 가 남지 않는다
  r = rig(slide);
  r.fol.onPos({ block: 3, frac: 0, total: 9, follow: true });
  r.setLook("flow");
  r.fol.onSlide({ index: 0, total: 5, look: "slides", follow: true });
  eq("F5 가 먼저: align 이 forced 를 내린다", r.at(), ["slides", "slides", false]);
  r.fol.onSlide(off);
  eq("발표 중 끔: forced 가 없어 restoreOwnLook 길", r.at(), ["slides", "slides", false]);

  // 보드 문서를 슬라이드로 발표 → 끝 → 문서 꼴, 이어서 호스트의 지금 꼴
  r = rig(board);
  r.fol.onPos({ block: 3, frac: 0, total: 9, follow: true });
  r.setLook("flow");
  r.fol.onSlide({ index: 0, total: 5, look: "slides", follow: true });
  eq("보드 문서 F5", r.at(), ["slides", "slides", false]);
  r.fol.onSlide(end);
  eq("보드 문서 발표 끝 → 문서 꼴", r.at(), ["dashboard", "", false]);
  r.setLook("slides");
  eq("이어서 호스트의 지금 꼴", r.at(), ["slides", "slides", false]);

  // 보드 발표의 내용 위치도 forced 를 내린다
  r = rig(board);
  r.fol.onPos({ block: 3, frac: 0, total: 9, follow: true });
  r.setLook("flow");
  r.fol.onSlide({ index: 0, total: 1, look: "dashboard", follow: true });
  eq("보드 발표", r.at(), ["dashboard", "dashboard", false]);

  // 따라가기를 끈 뒤 다시 켜면 확장이 setLook 을 다시 부른다 — 그 길이 다시 forced 를 세운다
  r = rig(slide);
  r.fol.onSlide(end);
  r.setLook("doc");
  r.fol.onSlide(off);
  r.fol.onSlide(end);
  eq("다시 켬: 확장이 부르기 전에는 문서 꼴", r.at(), ["slides", "", false]);
  r.setLook("doc");
  eq("다시 켬: 호스트의 꼴", r.at(), ["doc", "doc", true]);

  process.stdout.write(bad.length ? "FAIL " + bad.join("\n") : "FORCED OK");
`
	cmd := exec.Command(node, "-e", js)
	cmd.Env = append(os.Environ(), "SRC="+src,
		"DECL="+cut("  const fol = follower({\n", "\n  });\n"),
		"SET="+cut("  function setLook(name) {\n", "\n  }\n"),
		"CLICK="+cut("b.addEventListener(\"click\", () => {\n      state.chosen = b.dataset.look;", "\n    });"))
	out, err := cmd.CombinedOutput()
	if err != nil || string(out) != "FORCED OK" {
		t.Fatalf("호스트 꼴 적용: %v\n%s", err, out)
	}
}

// 다시 붙기 간격(itda-hyve#728 후속): 잠시 닫힘 동안은 1·2·4·5·5초(pauseBackoff), 잠시 닫힘이 아닌 끊김은 1·2·4·8·16·30·30초(backoff) 그대로다.
// retryNow 는 잠시 닫힘이 아니면(붙어 있음·잠시 닫힘 아닌 끊김·끝남) 아무것도 하지 않는다.
func TestSharePauseRetryDelay(t *testing.T) {
	node := needNode(t)
	js := `
  const fs = require("fs"), vm = require("vm");
  const ctx = { console }; ctx.window = ctx; vm.createContext(ctx);
  for (const f of ["pager.js", "slides.js", "share.js"]) vm.runInContext(fs.readFileSync(f, "utf8"), ctx);
  const S = ctx.window.ItdaShare, bad = [];
  const eq = (name, got, want) => { const a = JSON.stringify(got), b = JSON.stringify(want); if (a !== b) bad.push(name + ": " + a + " ≠ " + b); };
  const settle = async () => { for (let i = 0; i < 8; i++) await new Promise(r => setImmediate(r)); };
  // run 은 가짜 스트림에 이은 connect 다 — ms 는 setTimeout 에 건 대기, opens 는 연 스트림 수
  const run = probe => {
    const r = { ms: [], fns: [], opens: 0, ended: [], seen: [] };
    r.c = S.connect("/b", { EventSource: class { constructor() { r.es = this; r.opens++; this.on = {}; } addEventListener(n, f) { this.on[n] = f; } close() {} },
      probe: async () => probe, setTimeout: (f, d) => { r.fns.push(f); r.ms.push(d); return r.fns.length; }, clearTimeout() {},
      onHello() {}, onEnded: x => r.ended.push(x), onPaused: s => r.seen.push(s) });
    r.next = async () => { r.fns[r.fns.length - 1](); r.es.onerror(); await settle(); };
    return r;
  };
  (async () => {
    eq("pauseBackoff", [0, 1, 2, 3, 4, 9].map(S.pauseBackoff), [1000, 2000, 4000, 5000, 5000, 5000]);
    eq("backoff 그대로", [0, 1, 2, 3, 4, 5, 6].map(S.backoff), [1000, 2000, 4000, 8000, 16000, 30000, 30000]);

    let r = run("ended");
    r.es.on.hello({ data: JSON.stringify({ keep: true }) });
    r.es.on.stop({ data: JSON.stringify({ reason: "paused" }) }); await settle();
    for (let k = 0; k < 4; k++) await r.next();
    eq("잠시 닫힘 간격", [r.ms, r.seen, r.ended], [[1000, 2000, 4000, 5000, 5000], ["closed"], []]);

    r = run("alive");
    r.es.on.hello({ data: JSON.stringify({}) });
    r.es.onerror(); await settle();
    for (let k = 0; k < 6; k++) await r.next();
    eq("잠시 닫힘 아님: 30초 상한", [r.ms, r.seen], [[1000, 2000, 4000, 8000, 16000, 30000, 30000], []]);
    const opens = r.opens;
    eq("잠시 닫힘 아님: retryNow 는 아무것도 안 함", [await r.c.retryNow(), r.opens], [false, opens]);

    r = run("ended");
    r.es.on.hello({ data: JSON.stringify({ keep: true }) });
    eq("붙어 있음: retryNow 는 아무것도 안 함", [await r.c.retryNow(), r.opens], [false, 1]);
    r.es.on.kick({}); await settle();
    eq("끝남: retryNow 는 아무것도 안 함", [await r.c.retryNow(), r.opens, r.ended], [false, 1, ["kicked"]]);

    // 붙는 중에 끝나면(kick) 기다리던 단추도 false 로 풀린다
    r = run("ended");
    r.es.on.hello({ data: JSON.stringify({ keep: true }) });
    r.es.on.stop({ data: JSON.stringify({ reason: "paused" }) }); await settle();
    const p = r.c.retryNow();
    r.es.on.kick({}); await settle();
    eq("붙는 중 끝남", [await p, r.ended, r.opens], [false, ["kicked"], 2]);

    process.stdout.write(bad.length ? "FAIL " + bad.join("; ") : "DELAY OK");
  })().catch(e => process.stdout.write("FAIL " + e.stack));
`
	out, err := exec.Command(node, "-e", js).CombinedOutput()
	if err != nil || string(out) != "DELAY OK" {
		t.Fatalf("다시 붙기 간격: %v\n%s", err, out)
	}
}

// 재시도 경합(itda-hyve#728 리뷰 1회차 [1]): 늦은 probe A 도중 retryNow → 새 스트림도 실패해 probe B → A·B 가 차례로 돌아옴 → 다시 retryNow → 남은 타이머 실행 → end.
// 지난 세대의 probe(A)는 버려 타이머가 하나만 남고, 스트림은 늘 하나이며, end 뒤 열린 스트림은 0 이다. 첫 단추를 망 복귀(online — hostSource 의
// `if (conn.paused) conn.wake()`)가 대신해도 같다. A 의 답이 retryNow 의 Promise 를 엉뚱하게 풀지 않고, B 의 끝에서 false 로 풀린다.
func TestSharePauseRetryRace(t *testing.T) {
	node := needNode(t)
	js := `
  const fs = require("fs"), vm = require("vm");
  const ctx = { console }; ctx.window = ctx; vm.createContext(ctx);
  for (const f of ["pager.js", "slides.js", "share.js"]) vm.runInContext(fs.readFileSync(f, "utf8"), ctx);
  const S = ctx.window.ItdaShare, bad = [];
  const eq = (name, got, want) => { const a = JSON.stringify(got), b = JSON.stringify(want); if (a !== b) bad.push(name + ": " + a + " ≠ " + b); };
  const settle = async () => { for (let i = 0; i < 8; i++) await new Promise(r => setImmediate(r)); };
  // run 은 probe 의 답을 손으로 푸는 connect 다 — probes 는 미완료 probe 의 resolve, timers 는 clearTimeout 이 지우는 Map, streams 는 연 스트림
  const run = () => {
    const r = { streams: [], probes: [], timers: new Map(), id: 0, ended: [] };
    r.c = S.connect("/b", { EventSource: class { constructor() { r.streams.push(this); this.on = {}; } addEventListener(n, f) { this.on[n] = f; } close() { this.closed = true; } },
      probe: () => new Promise(res => r.probes.push(res)),
      setTimeout: (f, d) => { r.timers.set(++r.id, { f, d }); return r.id; }, clearTimeout: id => { r.timers.delete(id); },
      onHello() {}, onEnded: x => r.ended.push(x), onPaused() {} });
    r.last = () => r.streams[r.streams.length - 1];
    r.open = () => r.streams.filter(s => !s.closed).length;
    r.fire = async () => { const all = [...r.timers]; r.timers.clear(); for (const [, t] of all) t.f(); await settle(); };
    return r;
  };
  const race = async (name, first) => {
    const r = run();
    r.last().on.hello({ data: JSON.stringify({ keep: true }) });
    r.last().on.stop({ data: JSON.stringify({ reason: "paused" }) }); await settle(); // 1. probe A 미완료
    const A = r.probes.shift();
    let done = "pending";
    const p1 = first(r.c); if (p1) p1.then(v => { done = v; });          // 2. 새 스트림을 엶 — A 는 아직
    await settle();
    eq(name + ": 첫 시도 뒤 열린 스트림", [r.open(), r.streams.length], [1, 2]);
    r.last().onerror(); await settle();                                    // 3. probe B
    const B = r.probes.shift();
    A("ended"); await settle();                                            // 4. A 가 늦게 돌아옴 — 버린다
    eq(name + ": 지난 probe 는 타이머를 걸지 않고 Promise 를 풀지 않음", [r.timers.size, done], [0, "pending"]);
    B("ended"); await settle();
    eq(name + ": 타이머 하나·Promise false", [[...r.timers.values()].map(t => t.d), p1 ? done : false], [[1000], false]);
    const p2 = r.c.retryNow(); await settle();                             // 5. 다시 단추
    eq(name + ": 두 번째 단추 뒤 열린 스트림·남은 타이머", [r.open(), r.timers.size], [1, 0]);
    await r.fire();                                                        // 6. 남은 타이머(없어야 한다)
    eq(name + ": 남은 타이머 실행 뒤 열린 스트림", r.open(), 1);
    r.c.end();                                                             // 7.
    eq(name + ": end 뒤 열린 스트림·Promise", [r.open(), await p2, r.ended], [0, false, [""]]);
  };
  (async () => {
    await race("단추", c => c.retryNow());
    await race("online", c => { if (c.paused) c.wake(); return null; });

    // open 은 남은 스트림이 있으면 닫고 연다 — 붙는 중의 wake·단추는 새로 열지 않고, 지난 스트림의 이벤트는 버린다
    const r = run();
    r.last().on.hello({ data: JSON.stringify({ keep: true }) });
    r.last().on.stop({ data: JSON.stringify({ reason: "paused" }) }); await settle();
    r.probes.shift()("ended"); await settle();
    const old = r.c.retryNow(); r.c.wake(); await settle();
    eq("붙는 중: 스트림 하나", [r.open(), r.streams.length], [1, 2]);
    r.streams[0].onerror(); await settle();
    eq("닫힌 스트림의 오류는 버림", [r.probes.length, r.open()], [0, 1]);
    r.last().on.hello({ data: JSON.stringify({ keep: true }) }); await settle();
    eq("붙으면 true", [await old, r.c.state], [true, "open"]);

    process.stdout.write(bad.length ? "FAIL " + bad.join("; ") : "RACE OK");
  })().catch(e => process.stdout.write("FAIL " + e.stack));
`
	out, err := exec.Command(node, "-e", js).CombinedOutput()
	if err != nil || string(out) != "RACE OK" {
		t.Fatalf("재시도 경합: %v\n%s", err, out)
	}
}

// 슬라이드 이동 굴림(itda-hyve#728 후속): 발표자가 다른 슬라이드로 옮기면 scrollTo smooth(목표는 요소의 자리), 첫 맞추기·같은 슬라이드·재정렬·
// prefers-reduced-motion 이면 scrollTop 을 바로 둔다. 화면은 fol 의 align 만 move 로 부른다 — 그리기·노트 배치·창 크기의 재정렬은 바로 간다.
func TestShareSlideGlide(t *testing.T) {
	node := needNode(t)
	js := `
  const fs = require("fs"), vm = require("vm");
  const ctx = { console }; ctx.window = ctx; vm.createContext(ctx);
  for (const f of ["pager.js", "slides.js", "share.js"]) vm.runInContext(fs.readFileSync(f, "utf8"), ctx);
  const S = ctx.window.ItdaShare, bad = [];
  const eq = (name, got, want) => { const a = JSON.stringify(got), b = JSON.stringify(want); if (a !== b) bad.push(name + ": " + a + " ≠ " + b); };
  const v = { scrollTop: 100, calls: [], scrollTo(o) { this.calls.push(o); } };
  let reduced = false;
  const g = S.slideGlider(() => reduced), a = {}, b = {}, c = {};
  g.to(v, a, 50, true);
  eq("첫 맞추기: 바로", [v.calls, v.scrollTop], [[], 150]);
  g.to(v, a, 10, true);
  eq("같은 슬라이드: 바로", [v.calls, v.scrollTop], [[], 160]);
  g.to(v, b, 200, true);
  eq("다른 슬라이드: smooth", [v.calls, v.scrollTop], [[{ top: 360, behavior: "smooth" }], 160]);
  v.scrollTop = 250; // 굴리는 도중 — delta 는 지금 자리에서 잰 요소의 자리
  g.to(v, c, 300, true);
  eq("굴리는 도중 다음 슬라이드: 새 목표", v.calls[1], { top: 550, behavior: "smooth" });
  v.scrollTop = 400;
  g.to(v, c, 150, false);
  eq("재정렬: 바로 덮음", [v.calls.length, v.scrollTop], [2, 550]);
  g.to(v, a, -100, false);
  eq("재정렬(다른 요소): 바로", [v.calls.length, v.scrollTop], [2, 450]);
  reduced = true;
  g.to(v, b, 40, true);
  eq("줄인 움직임: 바로", [v.calls.length, v.scrollTop], [2, 490]);
  process.stdout.write(bad.length ? "FAIL " + bad.join("; ") : "GLIDE OK");
`
	out, err := exec.Command(node, "-e", js).CombinedOutput()
	if err != nil || string(out) != "GLIDE OK" {
		t.Fatalf("슬라이드 굴림: %v\n%s", err, out)
	}
	b, err := os.ReadFile("share.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	// move 를 넘기는 곳은 fol 의 align 하나다 — 재정렬(render·relayoutNotes·afterResize)은 세 인자로 부른다
	if n := strings.Count(src, "scrollToFollow(i, kind, frac, true)"); n != 1 || !strings.Contains(src, `if (act === "scroll") scrollToFollow(i, kind, frac, true);`) {
		t.Errorf("align 만 move 로 불러야 함 (%d)", n)
	}
	if n := strings.Count(src, "scrollToFollow(fol.index, fol.kind, fol.frac)"); n != 2 || !strings.Contains(src, "afterResize(fol, applyZoom, scrollToFollow)") {
		t.Errorf("재정렬은 move 없이 불러야 함 (%d)", n)
	}
	if !strings.Contains(src, `"(prefers-reduced-motion: reduce)"`) {
		t.Error("prefers-reduced-motion 을 봐야 함")
	}
}

// 노트 칸 너비 끌기(itda-hyve#728 실기 후속): 너비는 240~640 으로 자르고, 끄는 동안의 상한은 오른쪽 배치가 유지되는 너비까지,
// 기억은 이 브라우저의 localStorage(hs-note-w — 틀린 값·없는 값이면 320), 두 번 누르면 기본 320, 아래 전환 문턱은 고른 너비로 잰다.
func TestShareNoteWidth(t *testing.T) {
	node := needNode(t)
	js := `
  const fs = require("fs"), vm = require("vm");
  const ctx = { console }; ctx.window = ctx; vm.createContext(ctx);
  for (const f of ["pager.js", "slides.js", "share.js"]) vm.runInContext(fs.readFileSync(f, "utf8"), ctx);
  const S = ctx.window.ItdaShare, bad = [];
  const eq = (name, got, want) => { const a = JSON.stringify(got), b = JSON.stringify(want); if (a !== b) bad.push(name + ": " + a + " ≠ " + b); };
  eq("자름", [S.noteWidth(100), S.noteWidth(240), S.noteWidth(400.6), S.noteWidth(640), S.noteWidth(900), S.noteWidth(NaN), S.noteWidth("x")], [240, 240, 401, 640, 640, 320, 320]);
  // 상한: 보기 1400·여백 56·사이 20·최소 슬라이드 520 → 804 → 640, 1100 → 504, 좁으면 240
  eq("상한", [S.noteWidthCap(1400, 56, 20, 520), S.noteWidthCap(1100, 56, 20, 520), S.noteWidthCap(700, 56, 20, 520)], [640, 504, 240]);
  // 기억 읽기
  const store = init => { const m = Object.assign({}, init); return { m, getItem: k => k in m ? m[k] : null, setItem: (k, v) => { m[k] = String(v); } }; };
  const fonts = [12, 14, 17];
  const w = init => S.notePrefs(() => store(init), fonts).width;
  eq("없음·틀림·범위 밖이면 320", [w({}), w({ "hs-note-w": "" }), w({ "hs-note-w": "abc" }), w({ "hs-note-w": "100" }), w({ "hs-note-w": "700" }), w({ "hs-note-w": "Infinity" })], [320, 320, 320, 320, 320, 320]);
  eq("맞는 값", [w({ "hs-note-w": "240" }), w({ "hs-note-w": "480" }), w({ "hs-note-w": "640" })], [240, 480, 640]);
  eq("못 읽으면 320", S.notePrefs(() => { throw new Error("막힘"); }, fonts).width, 320);
  const st = store({}), p = S.notePrefs(() => st, fonts);
  p.setWidth(500);
  eq("기억 쓰기", [p.width, st.m["hs-note-w"]], [500, "500"]);
  p.setWidth(9999);
  eq("기억 쓰기도 자름", [p.width, st.m["hs-note-w"]], [640, "640"]);
  eq("다시 읽기", S.notePrefs(() => st, fonts).width, 640);
  // 끌기: 왼쪽으로 끌면 넓어지고 상한·하한에서 멈추며 놓을 때 한 번만 기억, 두 번 누르면 320 기억
  let cur = 320, cap = 504; const log = [];
  const r = S.noteResizer({ width: () => cur, cap: () => cap, apply: (px, save) => { cur = px; log.push([px, save]); } });
  r.move(0); r.end();
  eq("잡지 않으면 아무것도", log, []);
  r.begin(1000);
  eq("잡음", r.dragging(), true);
  r.move(900); r.move(700); r.move(1300);
  eq("끄는 중(상한 504·하한 240)", log, [[420, false], [504, false], [240, false]]);
  r.move(1050); r.end();
  eq("놓을 때 한 번 기억", log.slice(3), [[270, false], [270, true]]);
  eq("놓은 뒤", r.dragging(), false);
  log.length = 0; cap = 640; r.begin(500); r.move(500); r.end();
  eq("안 바뀌면 기억하지 않음", log, []);
  r.reset();
  eq("두 번 누름: 기본 320 기억", [cur, log], [320, [[320, true]]]);
  // 상한은 움직일 때마다 다시 묻는다(리뷰 [2]): 보기 1400 에서 잡고(상한 640) 끄는 중 보기가 1000 으로 줄면 상한 404
  log.length = 0; cur = 320; cap = S.noteWidthCap(1400, 56, 20, 520); r.begin(1000);
  r.move(900); cap = S.noteWidthCap(1000, 56, 20, 520); r.move(800);
  eq("끄는 중 상한이 줄어듦", [cap, log], [404, [[420, false], [404, false]]]);
  r.end();
  eq("상한 없으면 640", (() => { let c = 320; const q = S.noteResizer({ width: () => c, apply: px => { c = px; } }); q.begin(1000); q.move(0); return c; })(), 640);

  // 끌기 이벤트 수명(리뷰 [1]) — 가짜 document·window·칸·손잡이로 noteGripDrag 를 실제로 돌린다.
  const target = () => { const ls = {}; return { ls,
    addEventListener(t, f) { (ls[t] = ls[t] || []).push(f); },
    removeEventListener(t, f) { ls[t] = (ls[t] || []).filter(g => g !== f); },
    fire(t, e) { for (const f of (ls[t] || []).slice()) f(Object.assign({ preventDefault() {} }, e || {})); },
    count() { return Object.values(ls).reduce((n, a) => n + a.length, 0); } }; };
  const rig = () => {
    const doc = target(), win = target(), notes = target(), cls = new Set();
    doc.visibilityState = "visible";
    const body = { classList: { add: c => cls.add(c), remove: c => cls.delete(c) } };
    const caps = new Set(), released = [];
    const mkGrip = () => ({ isConnected: true, closest: s => null,
      setPointerCapture: id => caps.add(id), hasPointerCapture: id => caps.has(id),
      releasePointerCapture: id => { caps.delete(id); released.push(id); doc.fire("lostpointercapture", { pointerId: id }); } });
    let grip = mkGrip(); grip.closest = s => s === ".sn-grip" ? grip : null;
    const o = { w: 320, saves: [], right: true, starts: 0 };
    const res = S.noteResizer({ width: () => o.w, cap: () => 640, apply: (px, save) => { o.w = px; if (save) o.saves.push(px); } });
    const drag = S.noteGripDrag({ notes, doc, win, body, resizer: res, canGrip: () => o.right, onStart: () => { o.starts++; } });
    const docBase = doc.count();
    o.down = (id, x, extra) => notes.fire("pointerdown", Object.assign({ target: grip, pointerId: id, pointerType: "mouse", button: 0, clientX: x }, extra || {}));
    o.move = (id, x, buttons) => doc.fire("pointermove", { pointerId: id, pointerType: "mouse", buttons: buttons === undefined ? 1 : buttons, clientX: x });
    o.regrip = () => { grip.isConnected = false; grip = mkGrip(); grip.closest = s => s === ".sn-grip" ? grip : null; };
    // 끝난 상태: 끌기 아님·document·window 리스너 0·dragging-note 없음·캡처 없음
    o.clean = () => !drag.dragging() && doc.count() === docBase && win.count() === 0 && !cls.has("dragging-note") && caps.size === 0;
    return Object.assign(o, { doc, win, notes, cls, caps, released, drag });
  };
  { // 놓기
    const g = rig(); g.down(1, 1000);
    eq("잡으면 끄는 중", [g.drag.dragging(), g.cls.has("dragging-note"), g.caps.has(1), g.starts, g.win.count() > 0], [true, true, true, 1, true]);
    g.move(1, 900); g.move(2, 100); // 다른 포인터는 무시
    g.doc.fire("pointerup", { pointerId: 2 });
    eq("다른 포인터의 놓음은 무시", [g.drag.dragging(), g.w], [true, 420]);
    g.doc.fire("pointerup", { pointerId: 1 });
    eq("놓기: 정리·한 번 기억", [g.clean(), g.saves], [true, [420]]);
    g.drag.finish(); g.doc.fire("pointerup", { pointerId: 1 }); g.move(1, 0);
    eq("두 번 끝내도 그대로", [g.clean(), g.saves, g.w], [true, [420], 420]);
  }
  { // 취소
    const g = rig(); g.down(1, 1000); g.move(1, 950); g.doc.fire("pointercancel", { pointerId: 1 });
    eq("취소: 정리·바뀐 너비 기억", [g.clean(), g.saves], [true, [370]]);
  }
  { // 캡처 상실 → 버튼 없이 움직임 → 새로 잡기
    const g = rig(); g.down(1, 1000); g.move(1, 900);
    g.caps.delete(1); g.doc.fire("lostpointercapture", { pointerId: 1 });
    eq("캡처 상실: 정리·기억", [g.clean(), g.saves], [true, [420]]);
    g.move(1, 500, 0);
    eq("상실 뒤 움직임은 너비를 안 바꿈", g.w, 420);
    g.down(7, 800);
    eq("새 포인터로 다시 잡힘", [g.drag.dragging(), g.caps.has(7)], [true, true]);
    g.doc.fire("pointerup", { pointerId: 7 });
    eq("다시 놓음", g.clean(), true);
  }
  { // 창 밖에서 놓아 pointerup 을 놓침 → 버튼 없는 마우스 움직임
    const g = rig(); g.down(1, 1000); g.move(1, 900); g.move(1, 600, 0);
    eq("버튼 없는 움직임: 끝냄·그 움직임은 안 씀", [g.clean(), g.w, g.saves], [true, 420, [420]]);
  }
  { // 창 blur
    const g = rig(); g.down(1, 1000); g.move(1, 900); g.win.fire("blur");
    eq("blur: 정리·기억·캡처 풂", [g.clean(), g.saves, g.released], [true, [420], [1]]);
  }
  { // 탭 숨김 — 보임은 끝내지 않는다
    const g = rig(); g.down(1, 1000);
    g.doc.fire("visibilitychange"); eq("보이는 동안은 이어짐", g.drag.dragging(), true);
    g.doc.visibilityState = "hidden"; g.doc.fire("visibilitychange");
    eq("숨김: 정리·안 바뀌면 기억 없음", [g.clean(), g.saves], [true, []]);
  }
  { // 손잡이를 없애는 재그리기
    const g = rig(); g.down(1, 1000); g.move(1, 900);
    g.drag.check(); eq("손잡이가 그대로면 이어짐", g.drag.dragging(), true);
    g.regrip(); g.drag.check();
    eq("재그리기로 손잡이가 빠짐: 정리·기억", [g.clean(), g.saves], [true, [420]]);
  }
  { // 배치 전환(오른쪽이 아니게 됨)
    const g = rig(); g.down(1, 1000); g.move(1, 900); g.right = false; g.drag.check();
    eq("배치 전환: 정리·기억", [g.clean(), g.saves], [true, [420]]);
    g.down(2, 1000); eq("오른쪽이 아니면 잡지 않음", g.drag.dragging(), false);
  }
  { // 잡지 않는 경우와 두 번 누름
    const g = rig();
    g.down(1, 1000, { button: 2 }); g.notes.fire("pointerdown", { target: { closest: () => null }, pointerId: 1, pointerType: "mouse", button: 0, clientX: 0 });
    eq("오른쪽 단추·손잡이 밖은 잡지 않음", [g.drag.dragging(), g.starts], [false, 0]);
    g.down(1, 1000); g.move(1, 900);
    g.notes.fire("dblclick", { target: { closest: s => s === ".sn-grip" ? {} : null } });
    eq("두 번 누름: 끝내고 320 기억", [g.clean(), g.w, g.saves, g.starts], [true, 320, [420, 320], 2]);
  }
  // 보던 자리(리뷰 [3]): 가운데 mid=500, 슬라이드 0 은 [100,400], 1 은 [420,720] → 1 번 안 (500-420)/300
  const a = S.slideAnchor([{ top: 100, bottom: 400, height: 300 }, { top: 420, bottom: 720, height: 300 }], 500);
  eq("자리 잡기", [a.i, Math.round(a.frac * 1000)], [1, 267]);
  eq("슬라이드 없음", S.slideAnchor([], 500), null);
  eq("사이에 걸리면 가까운 슬라이드 밖 자리", S.slideAnchor([{ top: 100, bottom: 400, height: 300 }, { top: 440, bottom: 740, height: 300 }], 410).i, 0);
  // 배율이 줄어 슬라이드 1 이 [380,620](높이 240)이 되면 같은 자리(380+0.267×240=444)를 가운데 500 으로 — 56 위로 굴림(가운데 정렬이면 0)
  eq("자리 지키기", Math.round(S.anchorDelta({ top: 380, height: 240 }, a.frac, 500)), -56);
  eq("자리 그대로면 굴리지 않음", S.anchorDelta({ top: 420, height: 300 }, a.frac, 500), 0);
  // 아래 전환 문턱은 고른 너비로: 보기 1000·여백 56·사이 20·최소 520 → 너비 404 까지 오른쪽, 405 부터 아래
  eq("문턱", [S.notesLayout("right", true, "slides", 1000, 56, 404, 20, 520), S.notesLayout("right", true, "slides", 1000, 56, 405, 20, 520),
    S.notesLayout("below", true, "slides", 1000, 56, 640, 20, 520), S.notesLayout("off", true, "slides", 1000, 56, 240, 20, 520)], ["right", "below", "below", "off"]);
  // 폰 폭(< 600, 여백 16)은 가장 좁은 칸이어도 늘 아래
  eq("폰은 아래", S.notesLayout("right", true, "slides", 599, 16, 240, 20, 520), "below");
  process.stdout.write(bad.length ? "FAIL " + bad.join("; ") : "NOTEW OK");
`
	out, err := exec.Command(node, "-e", js).CombinedOutput()
	if err != nil || string(out) != "NOTEW OK" {
		t.Fatalf("노트 칸 너비: %v\n%s", err, out)
	}
	b, err := os.ReadFile("share.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	// 화면 연결: 배율·칸 배치가 고른 너비를 쓰고, 손잡이를 붙이고, 터치 끌기가 따라가기를 풀지 않는다
	for _, want := range []string{
		"notesLayout(np.mode, state.hasNotes, state.look, viewer.clientWidth, gutter, noteW, NOTE_GAP, NOTE_NARROW)",
		"{ layout, zoom: z, noteW, gap: NOTE_GAP }",
		`into: $("notes"), grip: true }`,
		`canGrip: () => state.notesLayout === "right",`,
		`if (e.target && e.target.closest && e.target.closest(".sn-grip")) return;`,
		"bindNoteGrip();",
		// 끌기를 끝낼지: 배치를 정할 때(두 갈래 모두)와 칸을 다시 만들 때 gripDrag.check()
		"state.notesLayout = \"off\";\n      if (gripDrag) gripDrag.check();",
		"state.notesLayout = layout;\n    if (gripDrag) gripDrag.check();",
		"grip: true });\n    if (gripDrag) gripDrag.check();",
		// 상한은 지금 보기 폭으로, 자리는 슬라이드 안의 자리로
		"cap: () => { const w = $(\"viewer\").clientWidth; return noteWidthCap(w, gutterOf(w), NOTE_GAP, NOTE_NARROW); },",
		"scrollTop += anchorDelta(pg.getBoundingClientRect(), a.frac, viewerMid());",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("share.js 에 %q 가 없음", want)
		}
	}
	if strings.Contains(src, "NOTE_W,") || strings.Contains(src, "noteW: NOTE_W") {
		t.Error("고정 너비 NOTE_W 가 남아 있음")
	}
}
