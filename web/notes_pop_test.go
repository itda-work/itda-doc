package web

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// 발표자 노트 판(itda-hyve#1040 2단계): 막대의 「노트」 단추(#notes-btn) 하나와 그 아래 판(#notes-pop — 위치·글자 크기). 여닫기(share.js notesPop)를
// 가짜 DOM 으로 node 에서 돌린다 — 단추 누름 토글, 판 안 단추 누름은 열린 채, 바깥 pointerdown·Esc(열렸을 때만 — 캡처에서 막고 단추로 초점)·초점 이탈은 닫음,
// aria-expanded 가 상태와 같음, 단추의 .on 은 건드리지 않음(노트 켜짐 표시 — paintNotesSwitch), 바깥 누름 사건을 막지 않음(호스트의 다른 메뉴도 닫힌다).
func TestShareNotesPop(t *testing.T) {
	node := needNode(t)
	js := `
  const fs = require("fs"), vm = require("vm");
  const ctx = { console }; ctx.window = ctx; vm.createContext(ctx);
  for (const f of ["pager.js", "slides.js", "share.js"]) vm.runInContext(fs.readFileSync(f, "utf8"), ctx);
  const S = ctx.window.ItdaShare, bad = [];
  const eq = (name, got, want) => { const a = JSON.stringify(got), b = JSON.stringify(want); if (a !== b) bad.push(name + ": " + a + " ≠ " + b); };
  // 가짜 요소: 부모 사슬로 contains, 사건 목록, 속성, class
  const el = (name, parent) => { const ls = {}, attrs = {}, cls = new Set();
    const e = { name, parent, hidden: false, ls, focused: 0,
      contains: t => { for (let x = t; x; x = x.parent) if (x === e) return true; return false; },
      addEventListener: (k, f) => { (ls[k] = ls[k] || []).push(f); },
      setAttribute: (k, v) => { attrs[k] = String(v); }, getAttribute: k => k in attrs ? attrs[k] : null,
      classList: { toggle: (c, on) => { if (on) cls.add(c); else cls.delete(c); }, has: c => cls.has(c) },
      focus: () => { e.focused++; } };
    return e; };
  const doc = { cap: {}, addEventListener: (k, f, c) => { if (c === true) (doc.cap[k] = doc.cap[k] || []).push(f); else bad.push("문서 사건 " + k + " 을 캡처가 아니게 담") } };
  const bar = el("bar"), sw = el("switch", bar), btn = el("btn", sw), icon = el("icon", btn), pop = el("pop", sw), right = el("right", pop), aplus = el("aplus", pop), other = el("other", bar);
  const ev = (target, extra) => Object.assign({ target, stopped: 0, prevented: 0, stopPropagation() { this.stopped++; }, preventDefault() { this.prevented++; } }, extra || {});
  // 누름: 문서 캡처 pointerdown → 단추면 click(target 이 단추 안)
  const press = t => { const p = ev(t); for (const f of doc.cap.pointerdown || []) f(p); if (btn.contains(t)) for (const f of btn.ls.click || []) f(ev(t)); return p; };
  const key = (k, extra) => { const e = ev(other, Object.assign({ key: k }, extra)); for (const f of doc.cap.keydown || []) f(e); return e; };
  // focusout 은 올라간다(bubbles) — 떠나는 요소에서 조상 사슬로
  const blur = (from, to) => { const e = ev(from, { relatedTarget: to }); for (let x = from; x; x = x.parent) for (const f of x.ls.focusout || []) f(e); };
  const st = () => ({ open: p.isOpen(), hidden: pop.hidden, exp: btn.getAttribute("aria-expanded") });
  const OPEN = { open: true, hidden: false, exp: "true" }, SHUT = { open: false, hidden: true, exp: "false" };
  const p = S.notesPop({ doc, btn, pop });
  eq("처음은 닫힘", st(), SHUT);
  press(btn); eq("단추 누름 → 열림", st(), OPEN);
  press(icon); eq("단추 안 아이콘 누름 → 닫힘(토글)", st(), SHUT);
  press(btn);
  press(right); press(aplus); press(right); eq("판 안 위치·글자 단추를 여러 번 눌러도 열린 채", st(), OPEN);
  const out = press(other); eq("바깥 누름 → 닫힘", st(), SHUT);
  eq("바깥 누름 사건을 막지 않음(다른 메뉴도 같은 사건으로 닫힌다)", [out.stopped, out.prevented], [0, 0]);
  const shutEsc = key("Escape"); eq("닫혔을 때 Esc 는 흘려보냄", [shutEsc.stopped, shutEsc.prevented, st()], [0, 0, SHUT]);
  press(btn); const f0 = btn.focused;
  const comp = key("Escape", { isComposing: true }); eq("조합 중 Esc 는 그대로", [comp.stopped, st()], [0, OPEN]);
  const other1 = key("ArrowDown"); eq("다른 키는 흘려보냄", [other1.stopped, st()], [0, OPEN]);
  const esc = key("Escape"); eq("열렸을 때 Esc → 닫고 막고 단추로 초점", [st(), esc.stopped, esc.prevented, btn.focused - f0], [SHUT, 1, 1, 1]);
  for (const [name, k, mods] of [["F5", "F5", {}], ["⌘+Shift+Enter", "Enter", { shiftKey: true, metaKey: true }], ["Ctrl+Shift+Enter", "Enter", { shiftKey: true, ctrlKey: true }]]) {
    press(btn); const f1 = btn.focused;
    const k1 = key(k, mods); eq("발표로 들어가는 키 " + name + " → 닫고 흘려보냄(초점 그대로)", [st(), k1.stopped, k1.prevented, btn.focused - f1], [SHUT, 0, 0, 0]);
  }
  press(btn); const se = key("Enter", { shiftKey: true }); eq("Shift+Enter 만은 그대로", [se.stopped, st()], [0, OPEN]); press(btn);
  press(btn);
  blur(btn, right); eq("초점이 단추 → 판 안: 열린 채", st(), OPEN);
  blur(right, aplus); eq("초점이 판 안에서 옮김: 열린 채", st(), OPEN);
  blur(aplus, null); eq("초점 상대가 없음(창 밖 등): 열린 채 — 바깥 누름이 닫는다", st(), OPEN);
  blur(aplus, other); eq("초점이 판 밖으로 → 닫힘", st(), SHUT);
  press(btn); blur(btn, other); eq("초점이 단추에서 밖으로 → 닫힘", st(), SHUT);
  press(btn); p.set(false); eq("set(false) — 막대를 숨길 때 닫힘(초점은 그대로)", [st(), btn.focused - f0], [SHUT, 1]);
  eq("여닫기는 단추의 .on 을 건드리지 않음", btn.classList.has("on"), false);
  btn.classList.toggle("on", true); press(btn); press(btn); eq("켜진 표시는 여닫아도 남음", btn.classList.has("on"), true);
  process.stdout.write(bad.length ? "FAIL " + bad.join("; ") : "NOTESPOP OK");
`
	out, err := exec.Command(node, "-e", js).CombinedOutput()
	if err != nil || string(out) != "NOTESPOP OK" {
		t.Fatalf("노트 판 여닫기: %v\n%s", err, out)
	}
}

// 노트 판의 자리(share.html)·칠하기와 닫기(share.js)·꼴(share.css). 호스트(itda-hyve)가 기대는 글 — 숨은 막대 여는 태그, 막대 안의 data-notes 단추, 켜진 단추의
// 셀렉터 #notes-switch [data-notes].on — 도 함께 본다.
func TestShareNotesPopPlace(t *testing.T) {
	html := readText(t, "share.html")
	if !regexp.MustCompile(`id="notes-switch"[^>]* hidden>`).MatchString(html) || !regexp.MustCompile(`id="notes-switch"[^>]*>(?s:.)*data-notes="off"`).MatchString(html) {
		t.Error("share.html 의 #notes-switch 여는 태그가 hidden> 으로 끝나지 않거나 그 뒤에 data-notes 단추가 없음")
	}
	a, b := strings.Index(html, `id="notes-switch"`), strings.Index(html, `id="look-switch"`)
	if a < 0 || b < a {
		t.Fatal("share.html 에 #notes-switch·#look-switch 가 없음")
	}
	sw := html[a:b]
	pop := strings.Index(sw, `<div class="notes-pop" id="notes-pop" role="group"`)
	if pop < 0 || !strings.HasSuffix(sw[:pop], "\n    ") {
		t.Fatal("#notes-switch 안에 판(#notes-pop)이 없음")
	}
	bar := sw[:pop]
	if n := strings.Count(bar, "<button"); n != 1 || !strings.Contains(bar, `id="notes-btn"`) || !strings.Contains(bar, `aria-controls="notes-pop" aria-expanded="false"`) ||
		!strings.Contains(bar, `aria-label="발표자 노트"`) || !strings.Contains(bar, `<use href="#i-notes"/>`) || !strings.Contains(bar, `<span class="notes-k">노트</span>`) {
		t.Errorf("막대에 바로 보이는 노트 요소가 「노트」 단추 하나(아이콘·글·aria)가 아님(단추 %d 개)\n%s", n, bar)
	}
	in := sw[pop:]
	if !strings.HasPrefix(in, `<div class="notes-pop" id="notes-pop" role="group" aria-label="발표자 노트 위치·글자 크기" hidden>`) {
		t.Error("판이 hidden 으로 시작하지 않음")
	}
	for _, want := range []string{`data-notes="off"`, `data-notes="right"`, `data-notes="below"`, `data-note-font="-1"`, `data-note-font="1"`, `id="note-fs-val"`} {
		if strings.Count(in, want) != 1 || strings.Contains(bar, want) {
			t.Errorf("%s 가 판 안에 하나만 있지 않음", want)
		}
	}
	for _, want := range []string{`<symbol id="i-notes"`, `<symbol id="i-chevron-down"`} {
		if !strings.Contains(html, want) {
			t.Errorf("share.html sprite 에 %s 가 없음", want)
		}
	}
	// 인라인 스크립트·style 속성·인라인 사건 처리기를 쓰지 않는다(묶음 화면 packDocCSP·LAN 공유 shareViewCSP)
	if regexp.MustCompile(`<script(?:\s[^>]*)?>[^<]`).MatchString(html) || regexp.MustCompile(`\sstyle=`).MatchString(html) || regexp.MustCompile(`\son[a-z]+=`).MatchString(html) {
		t.Error("share.html 에 인라인 스크립트·style 속성·on* 속성이 있음")
	}

	js := readText(t, "share.js")
	for _, want := range []string{
		// 칠하기: 숨기면 판도 닫고, 단추의 켜진 표시는 고른 값이 끔인가로
		"$(\"notes-switch\").hidden = state.ended || state.look !== \"slides\" || !state.hasNotes;\n    if ($(\"notes-switch\").hidden) npop.set(false);\n    if ($(\"notes-btn\")) $(\"notes-btn\").classList.toggle(\"on\", np.mode !== \"off\");",
		// 끝난 공유: 막대를 숨기고 판도 닫는다
		"$(\"notes-switch\").hidden = true;\n    npop.set(false);",
		// 위치·글자 단추의 처리기는 그대로 — 판 안으로 옮겨도 같은 셀렉터로 단다
		`for (const b of document.querySelectorAll("[data-notes]")) b.addEventListener("click", () => setNotesMode(b.dataset.notes));`,
		`for (const b of document.querySelectorAll("[data-note-font]")) b.addEventListener("click", () => stepNoteFont(Number(b.dataset.noteFont)));`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("share.js 에 %q 가 없음", want)
		}
	}
	if i, j := strings.Index(js, "const npop = "), strings.Index(js, "\n  function paintNotesSwitch()"); i < 0 || j < 0 || i > j {
		t.Error("npop 을 paintNotesSwitch 보다 앞에 만들지 않음")
	}

	css := readText(t, "share.css")
	for _, want := range []string{
		".share-view .notes-switch { position: relative; }",
		".share-view .notes-pop { position: absolute; top: calc(100% + 6px); right: 0; z-index: 60;",
		"background: var(--ui-panel); border: 1px solid var(--ui-line); border-radius: 10px; box-shadow: var(--ui-shadow);",
		// 좁은 폭: 900px 에서 단추 글자를 숨기고(아이콘만), 600px 에서 「오른쪽」 을 숨긴다(폰은 늘 아래)
		"@media (max-width: 900px) { /* 좁은 폭(호스트 막대의 문턱과 같다): 단추는 아이콘만 — 이름은 aria-label 「발표자 노트」 가 남는다 */\n  .share-view .notes-btn .notes-k { display: none; }",
		"@media (max-width: 600px) {\n  .share-view .notes-switch [data-notes=\"right\"] { display: none; }",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("share.css 에 %q 가 없음", want)
		}
	}
}
