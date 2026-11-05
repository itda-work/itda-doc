package web

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// View 원천 나누기(itda-work/itda-hyve#577 2b)의 경계를 지키는 시험. share.js 는 받은 View 를 그리는 「화면」과 View 를 받는 「원천」으로
// 나뉜다 — 공유 화면의 원천(hostSource)만 호스트와 이어지고, 화면은 원천 api 로만 view 를 받는다. 뷰어(web/viewer)는 같은 화면에
// wasm 원천을 끼운다.

func readText(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func needNode(t *testing.T) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 실행 검사를 할 수 없다")
		}
		t.Skip("node 가 없어 실행 검사를 건너뜀")
	}
	return node
}

// stripComments 는 // 주석 줄과 줄 끝의 「 // …」 주석을 뺀다(경계 검사가 주석의 낱말에 걸리지 않게 — share.js 의 글자에는 「 // 」 가 없다).
func stripComments(s string) string {
	var out []string
	tail := regexp.MustCompile(`\s// .*$`)
	for _, l := range strings.Split(s, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "//") {
			continue
		}
		out = append(out, tail.ReplaceAllString(l, ""))
	}
	return strings.Join(out, "\n")
}

// 화면(share.js 의 「// ---- 화면」 뒤)은 호스트와 직접 잇지 않는다 — 요청·스트림·저장소·다시 불러오기·서버 주소는 hostSource 안에만 있다.
func TestShareScreenHasNoHostIO(t *testing.T) {
	src := readText(t, "share.js")
	i := strings.Index(src, "\n  // ---- 화면\n")
	h := strings.Index(src, "\n  function hostSource(base, w) {\n")
	if i < 0 || h < 0 || h > i {
		t.Fatal("share.js 에서 「---- 화면」 절이나 hostSource 를 찾지 못함")
	}
	screen := stripComments(src[i:])
	for _, bad := range []string{"fetch(", "EventSource", "view.json", "/events", "sessionStorage", "location.", "_/_theme", "XMLHttpRequest", "WebSocket", "sendBeacon"} {
		if strings.Contains(screen, bad) {
			t.Errorf("share.js 화면에 호스트 입출력 %q 가 있음 — hostSource 로 옮긴다", bad)
		}
	}
	// 화면이 원천을 쓰는 길은 attach·applyTheme·start·stop 뿐이다
	if n := strings.Count(screen, "linked."); n != 3 {
		t.Errorf("화면이 원천(linked)을 부르는 곳이 %d 곳(applyTheme·stop·start 의 3 이어야 함)", n)
	}
	// 탭 제목 두 곳(render·showEnded)은 원천의 tabTitle 을 넘긴다(뷰어는 고정 — 문서 제목이 탭·기록에 남지 않게)
	if n := strings.Count(screen, ", app, source.tabTitle);"); n != 2 {
		t.Errorf("docTitle 에 source.tabTitle 을 넘기는 곳이 %d 곳(2 이어야 함)", n)
	}
	// 탭 제목을 쓰는 곳은 셋뿐이다: 문서를 받기 전(앱 이름)과 위 두 곳. 다른 곳에서 문서 제목을 탭에 쓰면 뷰어의 고정이 깨진다
	if n := strings.Count(screen, "document.title ="); n != 3 {
		t.Errorf("화면 절의 document.title = 이 %d 곳(3 이어야 함)", n)
	}
	// 순서(나누기 전과 같게): attach 는 첫 그리기 전(render 정의보다 앞), start 는 확장 시작(extInit(·ResizeObserver 뒤)의 맨 끝 문장,
	// stop 은 끝난 화면(showEnded)의 state.ended = true 바로 뒤. 이 글 위치 검사 밖의 화면 배선(타이밍 등)은 시험이 없다 —
	// hyve 핀을 올릴 때 hyve selftest(브라우저)가 지킨다.
	at, rd := strings.Index(screen, "source.attach("), strings.Index(screen, "async function render(")
	ex, ro := strings.Index(screen, "extInit("), strings.LastIndex(screen, "new ResizeObserver(")
	st := strings.Index(screen, "  linked.start();\n})();")
	if at < 0 || rd < 0 || at > rd {
		t.Error("source.attach 가 render 정의보다 앞에 없음")
	}
	if ex < 0 || ro < 0 || st < 0 || st < ex || st < ro || strings.TrimSpace(screen[st:]) != "linked.start();\n})();" {
		t.Error("linked.start() 가 확장 시작·ResizeObserver 뒤 화면 절의 맨 끝 문장이 아님")
	}
	se := strings.Index(screen, "function showEnded(")
	ne := -1
	if se >= 0 {
		ne = strings.Index(screen[se:], "\n  }\n")
	}
	if se < 0 || ne < 0 || !strings.Contains(screen[se:se+ne], "state.ended = true;\n    linked.stop();") {
		t.Error("linked.stop() 이 showEnded 의 state.ended = true 바로 뒤에 없음")
	}
}

// pickSource: data-base 가 있으면(공유 화면) 늘 hostSource — 페이지의 ItdaViewSource 가 있어도 바뀌지 않는다. 없으면 ItdaViewSource,
// 그것도 없으면(또는 attach 가 없으면) null. hostSource 는 가짜 window 로 돌려 본다: start 하면 <base>/events 를 열고 <base>/view.json 을
// 받아 api.show 로 넘기고, 테마 CSS link 를 <base>/_/_theme/… 로 걸며, stop 이벤트면 api.end(""), hello·slide·pos 는 api 로 넘긴다.
func TestShareSourcePickAndHost(t *testing.T) {
	node := needNode(t)
	js := `
  const fs = require("fs"), vm = require("vm");
  const ctx = { console, setTimeout, clearTimeout, Promise }; ctx.window = ctx; vm.createContext(ctx);
  for (const f of ["pager.js", "slides.js", "share.js"]) vm.runInContext(fs.readFileSync(f, "utf8"), ctx);
  const S = ctx.window.ItdaShare, bad = [];
  const ok = (name, c) => { if (!c) bad.push(name); };
  const mine = { attach() { return {}; } };
  const h = S.pickSource({ ItdaViewSource: mine }, { dataset: { base: "/s/tok" } });
  ok("data-base 면 hostSource", h && h !== mine && h.base === "/s/tok" && typeof h.attach === "function");
  ok("data-base 없으면 ItdaViewSource", S.pickSource({ ItdaViewSource: mine }, { dataset: {} }) === mine);
  ok("둘 다 없으면 null", S.pickSource({}, { dataset: {} }) === null);
  ok("attach 없는 원천은 null", S.pickSource({ ItdaViewSource: {} }, { dataset: {} }) === null);
  ok("hostSource 는 탭 제목을 고정하지 않음", h.tabTitle === undefined);

  const log = [];
  class ES {
    constructor(url) { this.url = url; this.on = {}; ES.last = this; log.push("es " + url); }
    addEventListener(n, f) { this.on[n] = f; }
    close() { log.push("close"); }
  }
  const view = { version: 1, theme: "classic", theme_rev: 0, html: "<p>x</p>" };
  let status = 200, tid = 0;
  const timers = new Map(); // 원천의 타이머는 손으로 돌린다(flush)
  const flush = () => { for (const [id, f] of [...timers]) { timers.delete(id); f(); } };
  const w = {
    EventSource: ES,
    fetch: (url, o) => { log.push("fetch " + url + " " + ((o && o.method) || "GET")); return Promise.resolve({ status, ok: status === 200, json: () => Promise.resolve(view) }); },
    sessionStorage: { getItem: () => null, setItem() {} },
    location: { reload: () => log.push("reload") },
    setTimeout: f => { timers.set(++tid, f); return tid; }, clearTimeout: id => timers.delete(id),
    addEventListener: n => log.push("w.on " + n),
    document: { addEventListener: n => log.push("doc.on " + n), visibilityState: "visible" },
  };
  const link = { href: "", addEventListener(n, f) { if (n === "load") this.load = f; }, setAttribute(k, v) { this.href = v; if (this.load) setTimeout(this.load, 0); } };
  const shown = [], ended = [], fol = [];
  let cur = null;
  const api = { $: id => id === "theme-css" ? link : null, ended: () => ended.length > 0, current: () => cur,
    show: v => { cur = v; shown.push(v); return Promise.resolve(); }, redraw() {}, failed: m => log.push("failed " + m), themeFailed() {},
    end: r => ended.push(r), hello: d => fol.push("hello"), slide: d => fol.push("slide"), pos: d => fol.push("pos"), toast() {} };
  const linked = S.hostSource("/s/tok", w).attach(api);
  ok("attach 는 아무것도 요청하지 않음", log.length === 0);
  linked.start();
  (async () => {
    for (let i = 0; i < 50 && !shown.length; i++) await new Promise(r => setTimeout(r, 2));
    ok("events 를 연다", log.includes("es /s/tok/events"));
    ok("view.json 을 받는다", log.includes("fetch /s/tok/view.json GET"));
    ok("받은 view 를 show", shown[0] === view);
    ok("pageshow·visibilitychange", log.includes("w.on pageshow") && log.includes("doc.on visibilitychange"));
    const th = await linked.applyTheme(view);
    ok("테마 link", th === true && link.href === "/s/tok/_/_theme/classic/theme.css?v=0");
    ES.last.on.hello({ data: JSON.stringify({ version: 1, theme: "classic", theme_rev: 0, slide: -1 }) });
    ES.last.on.slide({ data: JSON.stringify({ index: 0, total: 1, follow: true }) });
    ES.last.on.pos({ data: JSON.stringify({ block: 1, frac: 0, total: 2, follow: true }) });
    ok("hello·slide·pos 는 api 로", fol.join(",") === "hello,slide,pos");
    // stop() 은 걸어 둔 다시 불러오기(reloadGate — 2초 안 두 번째 reload)와 view.json 다시 받기(syncer — 429 뒤 backoff)를 거둔다
    ES.last.on.reload({});
    ES.last.on.reload({});
    status = 429;
    ES.last.on.version({ data: JSON.stringify({ version: 2, theme: "classic", theme_rev: 0 }) });
    for (let i = 0; i < 10; i++) await new Promise(r => setTimeout(r, 1));
    const reloads = () => log.filter(x => x === "reload").length, fetches = () => log.filter(x => x.startsWith("fetch ")).length;
    const r0 = reloads(), f0 = fetches();
    ok("stop 전: reload 1 번·지연 reload 와 429 다시 받기가 걸림", r0 === 1 && timers.size >= 2);
    linked.stop();
    flush();
    for (let i = 0; i < 10; i++) await new Promise(r => setTimeout(r, 1));
    ok("stop 뒤: 지연 reload·다시 받기 없음", reloads() === r0 && fetches() === f0);
    ES.last.on.stop({ data: "{}" });
    ok("stop 이면 end('')", ended.length === 1 && ended[0] === "" && log.includes("close"));
    process.stdout.write(bad.length ? "FAIL " + bad.join("; ") + " " + JSON.stringify(log) : "SOURCE OK");
  })();
`
	cmd := exec.Command(node, "-e", js)
	out, err := cmd.CombinedOutput()
	if err != nil || string(out) != "SOURCE OK" {
		t.Fatalf("원천 고르기·hostSource: %v\n%s", err, out)
	}
}

// 뷰어 화면(web/viewer/index.html)은 share.js 화면이 찾는 자리(id)를 모두 갖고, data-base 가 없으며(공유 서버 원천을 쓰지 않는다),
// share.js 보다 먼저 viewer.js 를 싣는다(ItdaViewSource). CSP 는 공유 화면처럼 default-src 'none' 에 쓰는 지시어만 열고(지시어 목록이
// 그대로 같아야 한다), 그림은 data:·blob: 만 열며(같은 출처·https: 그림 요청도 막음) 인라인 스타일·unsafe-eval 을 열지 않는다. DNS 미리 찾기를 끈다.
// 공유 화면(share.html)도 같은 id 를 모두 갖는다(나누면서 자리를 잃지 않았는지).
func TestViewerPageMatchesScreen(t *testing.T) {
	js := readText(t, "share.js")
	screen := js[strings.Index(js, "\n  // ---- 화면\n"):]
	ids := map[string]bool{}
	for _, m := range regexp.MustCompile(`[^.]\$\("([a-z-]+)"\)`).FindAllStringSubmatch(screen, -1) {
		ids[m[1]] = true
	}
	if len(ids) < 15 {
		t.Fatalf("share.js 화면의 id 를 덜 찾음: %v", ids)
	}
	page := readText(t, "viewer/index.html")
	share := readText(t, "share.html")
	for id := range ids {
		if !strings.Contains(page, `id="`+id+`"`) {
			t.Errorf("viewer/index.html 에 id=%q 가 없음", id)
		}
		if !strings.Contains(share, `id="`+id+`"`) {
			t.Errorf("share.html 에 id=%q 가 없음", id)
		}
	}
	if body := regexp.MustCompile(`<body[^>]*>`).FindString(page); body == "" || strings.Contains(body, "data-base") {
		t.Errorf("viewer/index.html 의 body 에 data-base 가 있음(또는 body 가 없음) — share.js 가 공유 서버 원천을 쓰게 된다: %q", body)
	}
	vi, si := strings.Index(page, `<script src="viewer.js"`), strings.Index(page, `<script src="share.js"`)
	if vi < 0 || si < 0 || vi > si {
		t.Error("viewer.js 를 share.js 보다 먼저 실어야 한다")
	}
	csp := regexp.MustCompile(`http-equiv="Content-Security-Policy" content="([^"]*)"`).FindStringSubmatch(page)
	if csp == nil {
		t.Fatal("viewer/index.html 에 meta CSP 가 없음")
	}
	want := []string{"default-src 'none'", "script-src 'self' 'wasm-unsafe-eval'", "style-src 'self' blob:", "img-src data: blob:",
		"font-src 'self'", "connect-src 'self'", "base-uri 'none'", "form-action 'none'"}
	var got []string
	for _, d := range strings.Split(csp[1], ";") {
		if d = strings.TrimSpace(d); d != "" {
			got = append(got, d)
		}
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("CSP 지시어가 다름\n받음 %q\n기대 %q", got, want)
	}
	if !strings.Contains(page, `<meta http-equiv="x-dns-prefetch-control" content="off">`) {
		t.Error("x-dns-prefetch-control off 가 없음")
	}
	if strings.Contains(page, "<style") {
		t.Error("viewer/index.html 에 <style> 이 있음 — CSP 가 인라인 스타일을 열지 않는다")
	}
}

// 뷰어 스크립트 둘(viewer.js·itda-doc.js)의 요청 글자 세기: wasm 받기(itda-doc.js load)와 견본 목록·견본 글(viewer.js)의 GET 뿐이고,
// XMLHttpRequest·sendBeacon·WebSocket·EventSource·Image()·method·body·location·window.open·history.·document.title 을 쓰지 않는다
// (폴더 열기·문서 바꾸기도 요청을 내지 않는다 — 폴더 안 파일은 File 로만 읽는다). 글자 세기라 「보내지 않음」의
// 충분한 근거는 아니다 — 그림으로 새는 길은 CSP(img-src 는 data:·blob: 만 — 같은 출처·원격 그림 요청을 막음)와 viewer.cjs 의 「show 가 받은 html 에 원격·상대 src 없음」이
// 함께 막는다. pager.js·slides.js·share.js 는 세지 않는다(share.js 의 요청은 hostSource 안 — TestShareScreenHasNoHostIO).
func TestViewerNoSend(t *testing.T) {
	want := map[string]int{"viewer/itda-doc.js": 0, "viewer/viewer.js": 2}
	for f, n := range want {
		code := stripComments(readText(t, f))
		for _, bad := range []string{"XMLHttpRequest", "sendBeacon", "WebSocket", "EventSource", "Image(", "method:", "body:", "location", "window.open",
			"history.", "document.title"} { // 탭 제목·브라우저 기록에 문서·폴더 이름을 넣지 않는다(탭 제목은 share.js docTitle 이 고정 글로)
			if strings.Contains(code, bad) {
				t.Errorf("%s 에 %q 가 있음", f, bad)
			}
		}
		if got := strings.Count(code, "fetch("); got != n {
			t.Errorf("%s 의 fetch( 가 %d 곳(%d 이어야 함)", f, got, n)
		}
	}
	// itda-doc.js 는 fetch 를 fetchFn(url) 로 부른다(load 의 fetch 옵션) — 그 한 곳뿐
	if got := strings.Count(stripComments(readText(t, "viewer/itda-doc.js")), "fetchFn("); got != 1 {
		t.Errorf("itda-doc.js 의 요청이 %d 곳(wasm 받기 1 이어야 함)", got)
	}
}

// 첫 화면은 뷰어의 한계를 말한다 — 서버로 보내지 않음, 자동 갱신 없음, 연 폴더 위는 못 읽음.
func TestViewerLandingStatesLimits(t *testing.T) {
	s := readText(t, "viewer/index.html")
	for _, want := range []string{"서버로 보내지 않고", "저절로 바뀌지 않습니다", "연 폴더보다 위 폴더의 설정은 읽지 못합니다"} {
		if !strings.Contains(s, want) {
			t.Errorf("첫 화면에 %q 가 없다", want)
		}
	}
}
