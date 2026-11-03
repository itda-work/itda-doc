// 쪽 넘김 키와 복사 단추(itda-hyve#413 리뷰 [2]): pager.js 의 navKey·typingTarget·hostNavKey 를 그대로 node 로 돌린다(pager_test.go).
// Space 는 단추(button)에 포커스가 있으면 단추 누르기의 키다 — 공유 보기 화면(share.js keyHandler)도 넘기지 않고, 문서 창 보기
// 모드(hostNavKey — HOST_BUSY 제외)도 단추 위에서는 넘기지 않는다. 인자: pager.js 경로.
"use strict";
const fs = require("fs");
const vm = require("vm");

const ctx = { console };
ctx.window = ctx; // pager.js 는 window.ItdaPager(옛 이름 HyvePager 도 같은 객체)에 내보낸다
vm.createContext(ctx);
vm.runInContext(fs.readFileSync(process.argv[2], "utf8"), ctx);
const P = ctx.window.ItdaPager;

let cases = 0, failed = 0;
function ok(name, cond, detail) { cases++; if (!cond) { failed++; console.log("실패 " + name + " — " + JSON.stringify(detail)); } else console.log("통과 " + name); }
const eq = (name, got, want) => ok(name, JSON.stringify(got) === JSON.stringify(want), { got, want });

// closest 대역: 태그 이름으로만 매칭하는 요소 모형(button·input·[contenteditable] 셀렉터에 닿는다)
const el = tag => ({ closest: sel => sel.split(",").some(s => s.trim().startsWith(tag)) ? {} : null });
const key = (k, target, extra) => Object.assign({ key: k, target, altKey: false, ctrlKey: false, metaKey: false, shiftKey: false }, extra || {});

eq("typingTarget: 복사 단추는 글 넣는 곳이 아님(네김 키를 단추에 둔다)", P.typingTarget(el("button")), false);
eq("typingTarget: input 은 글 넣는 곳", P.typingTarget(el("input")), true);
eq("navKey: 단추 위 Space 는 단추 누르기 — 넘기지 않음", P.navKey(key(" ", el("button")), true), "");
eq("navKey: 단추 밖 Space 는 다음(1)", P.navKey(key(" ", null), true), 1);
eq("navKey: Shift+Space 는 이전(-1)", P.navKey(key(" ", null, { shiftKey: true }), true), -1);
eq("navKey: ← 는 이전(-1)", P.navKey(key("ArrowLeft", null), true), -1);
eq("navKey: ⌘← 는 넘기지 않음", P.navKey(key("ArrowLeft", null, { metaKey: true }), true), "");
const view = { mode: "view", presenting: false, hasDoc: true, modal: false, active: el("button") };
eq("hostNavKey: 문서 창 보기 — 단추에 포커스면 → 도 넘기지 않음", P.hostNavKey(key("ArrowRight", view.active), view), "");
const body = { mode: "view", presenting: false, hasDoc: true, modal: false, active: null };
eq("hostNavKey: 본문에 포커스면 → 는 다음(1)", P.hostNavKey(key("ArrowRight", body.active), body), 1);
eq("hostNavKey: 문서 창은 Space 를 넘김 키로 안 씀(브라우저 굴림 그대로)", P.hostNavKey(key(" ", body.active), body), "");
eq("hostNavKey: 발표 중엔 넘기지 않음(발표 키는 present.js)", P.hostNavKey(key("ArrowRight", body.active), Object.assign({}, body, { presenting: true })), "");
eq("hostNavKey: 편집 모드는 커서 이동 몫", P.hostNavKey(key("ArrowRight", body.active), Object.assign({}, body, { mode: "edit" })), "");
console.log("PAGERNAV DONE cases=" + cases + " failed=" + failed);
process.exit(failed ? 1 : 0);
