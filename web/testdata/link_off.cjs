// 열지 않는 문서 안 파일 링크 끄기(itda-hyve#1096 — share.js LINK_OFF_MSG·linkTo·markLinks, 그리고 실제 onClick·Enter 처리) 시험. Go 시험(link_off_test.go)이
// pager.js·slides.js·share.js 경로를 넘긴다. onClick 과 Enter keydown 은 share.js 소스에서 그 함수·처리기를 떼어 가짜 DOM 에 단다 — 같은 linkTo 로 끄고 누르는지 본다.
// 화면 모양(CSS)·실제 브라우저의 가운데 단추·새 탭은 보지 않는다(href 가 없음만 본다).
"use strict";
const fs = require("fs");
const vm = require("vm");
const files = process.argv.slice(2);
const ctx = { console };
ctx.window = ctx;
vm.createContext(ctx);
for (const f of files) vm.runInContext(fs.readFileSync(f, "utf8"), ctx);
const S = ctx.window.ItdaShare;
const shareSrc = fs.readFileSync(files[files.length - 1], "utf8");
let cases = 0, failed = 0;
function ok(name, cond, detail) {
  cases++;
  if (!cond) { failed++; console.log("실패 " + name + " — " + JSON.stringify(detail === undefined ? null : detail)); } else console.log("통과 " + name);
}

// ---- 가짜 DOM(링크·문단만)
class ClassList {
  constructor(el) { this.el = el; }
  get s() { return new Set(String(this.el.attrs.class || "").split(/\s+/).filter(Boolean)); }
  w(s) { if (s.size) this.el.attrs.class = [...s].join(" "); else delete this.el.attrs.class; }
  contains(c) { return this.s.has(c); }
  add(c) { const s = this.s; s.add(c); this.w(s); }
  remove(c) { const s = this.s; s.delete(c); this.w(s); }
}
class El {
  constructor(tag, attrs) {
    this.tagName = tag.toUpperCase(); this.attrs = Object.assign({}, attrs || {}); this.children = []; this.parentNode = null; this.id = this.attrs.id || "";
    this.classList = new ClassList(this); this.clicks = 0;
  }
  append(...cs) { for (const c of cs) { c.parentNode = this; this.children.push(c); } return this; }
  hasAttribute(k) { return k in this.attrs; }
  getAttribute(k) { return k in this.attrs ? this.attrs[k] : null; }
  setAttribute(k, v) { this.attrs[k] = String(v); }
  removeAttribute(k) { delete this.attrs[k]; }
  all() { const out = []; const walk = n => { for (const c of n.children) { out.push(c); walk(c); } }; walk(this); return out; }
  querySelectorAll(sel) {
    if (sel !== "a[href], a[data-local-href]") throw new Error("모르는 선택자 " + sel);
    return this.all().filter(e => e.tagName === "A" && (e.hasAttribute("href") || e.hasAttribute("data-local-href")));
  }
  matches(sel) {
    if (sel === "a[href], a[data-local-href]") return this.tagName === "A" && (this.hasAttribute("href") || this.hasAttribute("data-local-href"));
    if (sel === "a.link-off") return this.tagName === "A" && this.classList.contains("link-off");
    if (sel === "#side-files") return this.id === "side-files";
    throw new Error("모르는 선택자 " + sel);
  }
  closest(sel) { for (let n = this; n; n = n.parentNode) if (n.matches && n.matches(sel)) return n; return null; }
  contains(x) { for (let n = x; n; n = n.parentNode) if (n === this) return true; return false; }
  click() { this.clicks++; }
}
const a = (attrs, text) => new El("a", attrs);
function body(...links) { const root = new El("div", { id: "pages" }); const p = new El("p"); p.append(...links); root.append(p); return root; }

// 1. linkTo — onClick·markLinks 가 같이 쓰는 판정
const docs = ["a/now.md", "a/next.md"];
ok("linkTo: #제목은 hash", S.linkTo("#끝", "a/now.md", docs) === "hash");
ok("linkTo: https·mailto·tel 은 out", ["https://x.org/", "HTTP://x.org/", "mailto:x@x.org", "tel:010"].every(h => S.linkTo(h, "", null) === "out"));
ok("linkTo: 단일 문서의 파일 링크는 null", S.linkTo("x.pdf", "", null) === null && S.linkTo("next.md", "", null) === null);
ok("linkTo: 폴더 목록에 있는 문서는 {doc, hash}", JSON.stringify(S.linkTo("next.md#끝", "a/now.md", docs)) === JSON.stringify({ doc: "a/next.md", hash: "#끝" }));
ok("linkTo: 폴더 목록에 없는 문서는 null", S.linkTo("other.md", "a/now.md", docs) === null);
ok("linkTo: 빈 href·// 주소·다른 스킴은 null", S.linkTo("", "", docs) === null && S.linkTo("//evil.example/", "a/now.md", docs) === null && S.linkTo("ftp://x/", "", docs) === null);
ok("LINK_OFF_MSG: 공유 화면의 기본 까닭", S.LINK_OFF_MSG === "공유 화면은 문서와 그 그림만 보입니다 — 이 파일은 받을 수 없습니다");

// 2. markLinks — 단일 문서: x.pdf 는 꺼지고 #·https·mailto·tel 은 그대로
{
  const pdf = a({ href: "x.pdf" }), hash = a({ href: "#제목" }), out = a({ href: "https://example.org/" }), mail = a({ href: "mailto:x@example.org" }), tel = a({ href: "tel:010" });
  const root = body(pdf, hash, out, mail, tel);
  S.markLinks(root, {});
  ok("단일: x.pdf 는 href 없음·data-local-href 로", !pdf.hasAttribute("href") && pdf.getAttribute("data-local-href") === "x.pdf", pdf.attrs);
  ok("단일: class link-off·aria-disabled·title(기본 까닭)", pdf.classList.contains("link-off") && pdf.getAttribute("aria-disabled") === "true" && pdf.getAttribute("title") === S.LINK_OFF_MSG, pdf.attrs);
  ok("단일: 키보드로 닿는다(tabindex 0·role link)", pdf.getAttribute("tabindex") === "0" && pdf.getAttribute("role") === "link");
  ok("단일: #·https·mailto·tel 은 손대지 않는다", [hash, out, mail, tel].every(x => x.hasAttribute("href") && !x.classList.contains("link-off") && !x.hasAttribute("title") && !x.hasAttribute("aria-disabled")),
    [hash, out, mail, tel].map(x => x.attrs));
  S.markLinks(root, {});
  ok("단일: 두 번 걸어도 같다", pdf.getAttribute("data-local-href") === "x.pdf" && pdf.getAttribute("title") === S.LINK_OFF_MSG && !pdf.hasAttribute("href"));
  // 원천의 까닭 글·저자 title
  const t = a({ href: "y.zip", title: "자료 묶음" });
  S.markLinks(body(t), { msg: "원천의 까닭" });
  ok("원천 linkMsg·저자 title: 「저자 — 까닭」", t.getAttribute("title") === "자료 묶음 — 원천의 까닭" && t.getAttribute("data-link-title") === "자료 묶음", t.attrs);
}

// 3. 폴더 공유: 목록의 next.md 는 살고 other.md 는 꺼진다. 목록이 늦게 오면 되살린다
{
  const next = a({ href: "next.md#끝" }), other = a({ href: "other.md" });
  const root = body(next, other);
  S.markLinks(root, { cur: "a/now.md", docs: null }); // 목록이 오기 전 첫 그리기
  ok("폴더: 목록 전에는 둘 다 꺼진다", next.classList.contains("link-off") && other.classList.contains("link-off"));
  S.markLinks(root, { cur: "a/now.md", docs }); // files 가 옴
  ok("폴더: 목록에 있는 next.md 는 href 를 되찾고 표시를 뗀다", next.getAttribute("href") === "next.md#끝" && !next.hasAttribute("data-local-href") && !next.classList.contains("link-off") &&
    !next.hasAttribute("title") && !next.hasAttribute("aria-disabled") && !next.hasAttribute("tabindex") && !next.hasAttribute("role"), next.attrs);
  ok("폴더: 목록에 없는 other.md 는 꺼진 채", !other.hasAttribute("href") && other.classList.contains("link-off"));
  const n2 = a({ href: "next.md" });
  S.markLinks(body(n2), { cur: "a/now.md", docs });
  ok("폴더: 처음부터 목록이 있으면 next.md 는 손대지 않는다", n2.getAttribute("href") === "next.md" && !n2.classList.contains("link-off") && Object.keys(n2.attrs).length === 1, n2.attrs);
  const t = a({ href: "x.pdf", title: "원래" });
  const r = body(t);
  S.markLinks(r, {});
  S.markLinks(r, { live: () => true });
  ok("되살리면 저자 title 을 돌려준다", t.getAttribute("title") === "원래" && t.getAttribute("href") === "x.pdf" && !t.hasAttribute("data-link-title"), t.attrs);
}

// 4. 원천이 살려 두는 링크(o.live — 묶은 교안의 첨부·교안 링크)
{
  const att = a({ href: "../../_f/" + "0".repeat(64) + "/a.pdf", download: "" }), other = a({ href: "b.pdf" });
  S.markLinks(body(att, other), { live: x => x.hasAttribute("download"), msg: "원천" });
  ok("live: 살려 두는 링크는 그대로", att.hasAttribute("href") && !att.classList.contains("link-off"));
  ok("live: 그 밖 파일 링크는 원천 글로 꺼진다", other.classList.contains("link-off") && other.getAttribute("title") === "원천");
}

// 5. 뷰어 원천이 옮겨 둔 <a data-local-href>(href 없음)도 표시를 단다
{
  const v = a({ "data-local-href": "x.pdf" });
  S.markLinks(body(v), { msg: "뷰어 글" });
  ok("뷰어: data-local-href 링크도 표시", v.classList.contains("link-off") && v.getAttribute("title") === "뷰어 글" && v.getAttribute("data-local-href") === "x.pdf" && !v.hasAttribute("href"));
}

// 6. 실제 onClick: 끈 링크를 누르면 이동 없이 같은 글 알림, 폴더 안 문서는 그 문서로, 원천이 살려 두는 링크는 건드리지 않는다
const m = /\n  function onClick\(e\) \{/.exec(shareSrc);
const z = m ? shareSrc.indexOf("\n  }\n", m.index + 1) : -1;
const onClickSrc = m && z > m.index ? shareSrc.slice(m.index + 1, z + 4) : "";
ok("share.js 에서 onClick 을 뗀다", onClickSrc.includes("linkTo(href") && onClickSrc.includes("source.liveLink"));
function clickWorld(o) {
  const viewer = new El("div", { id: "viewer" }), sidebar = new El("nav", { id: "sidebar" });
  const rec = { toasts: [], opens: [], jumps: [] };
  const state = o.state || { view: { doc: "" }, files: null };
  const lctx = { $: id => (id === "viewer" ? viewer : sidebar), state, linkTo: S.linkTo, LINK_OFF_MSG: S.LINK_OFF_MSG, source: o.source || {},
    jumper: { jump: h => rec.jumps.push(h) }, scheduleTocOn() {}, toast: t => rec.toasts.push(t), nav: { open: (d, h) => rec.opens.push([d, h]) } };
  vm.createContext(lctx);
  vm.runInContext(onClickSrc + "\nthis.onClick = onClick;", lctx);
  const press = el => { const e = { target: el, prevented: false, preventDefault() { this.prevented = true; } }; lctx.onClick(e); return e; };
  return { viewer, rec, press, state };
}
{
  const w = clickWorld({});
  const pdf = a({ href: "x.pdf" });
  w.viewer.append(body(pdf));
  S.markLinks(w.viewer, {});
  const e = w.press(pdf);
  ok("onClick: 끈 x.pdf 는 막고 같은 글 알림", e.prevented && w.rec.toasts.length === 1 && w.rec.toasts[0] === pdf.getAttribute("title") && w.rec.opens.length === 0, w.rec);
  const w2 = clickWorld({ source: { linkMsg: "원천의 안내" } });
  const p2 = a({ href: "x.pdf" });
  w2.viewer.append(body(p2));
  S.markLinks(w2.viewer, { msg: "원천의 안내" });
  w2.press(p2);
  ok("onClick: 원천 linkMsg 가 알림·title 같은 글", w2.rec.toasts[0] === "원천의 안내" && p2.getAttribute("title") === "원천의 안내");
  const folder = { view: { doc: "a/now.md" }, files: { docs } };
  const w3 = clickWorld({ state: folder });
  const nx = a({ href: "next.md#끝" }), ot = a({ href: "other.md" });
  w3.viewer.append(body(nx, ot));
  S.markLinks(w3.viewer, { cur: "a/now.md", docs });
  const e3 = w3.press(nx);
  ok("onClick: 폴더 안 next.md 는 그 문서로", e3.prevented && JSON.stringify(w3.rec.opens) === JSON.stringify([["a/next.md", "#끝"]]) && w3.rec.toasts.length === 0, w3.rec);
  w3.press(ot);
  ok("onClick: 목록 밖 other.md 는 안내", w3.rec.toasts.length === 1 && w3.rec.opens.length === 1);
  const w4 = clickWorld({ source: { liveLink: x => x.hasAttribute("download") } });
  const att = a({ href: "../../_f/x/a.pdf", download: "" });
  w4.viewer.append(body(att));
  const e4 = w4.press(att);
  ok("onClick: 원천이 살려 두는 링크는 막지 않고 알림 없음", !e4.prevented && w4.rec.toasts.length === 0);
  const w5 = clickWorld({});
  const ex = a({ href: "https://example.org/" });
  w5.viewer.append(body(ex));
  const e5 = w5.press(ex);
  ok("onClick: https 는 새 탭(막지 않음)", !e5.prevented && ex.target === "_blank" && ex.rel === "noopener noreferrer");
}

// 7. Enter keydown(끈 링크 위) → 누르기
{
  const km = /\n  document\.addEventListener\("keydown", e => \{\n    if \(e\.key !== "Enter"/.exec(shareSrc);
  const kz = km ? shareSrc.indexOf("\n  });\n", km.index + 1) : -1;
  const kSrc = km && kz > km.index ? shareSrc.slice(km.index + 1, kz + 6) : "";
  ok("share.js 에서 Enter 처리기를 뗀다", kSrc.includes('closest("a.link-off")'));
  let handler = null;
  const kctx = { document: { addEventListener: (t, f) => { if (t === "keydown") handler = f; } } };
  vm.createContext(kctx);
  vm.runInContext(kSrc, kctx);
  const off = a({ href: "x.pdf" });
  S.markLinks(body(off), {});
  const key = (el, k, extra) => { const e = Object.assign({ key: k, target: el, isComposing: false, defaultPrevented: false, prevented: false, preventDefault() { this.prevented = true; } }, extra || {}); handler(e); return e; };
  const e1 = key(off, "Enter");
  ok("Enter: 끈 링크는 누르기로", e1.prevented && off.clicks === 1);
  const live = a({ href: "https://example.org/" });
  key(live, "Enter");
  ok("Enter: 산 링크는 브라우저 그대로", live.clicks === 0);
  key(off, "Enter", { metaKey: true, shiftKey: true });
  key(off, "Enter", { isComposing: true });
  ok("Enter: 조합키(발표)·조합 중은 그대로", off.clicks === 1);
}

console.log("LINKOFF DONE cases=" + cases + " failed=" + failed);
process.exit(failed ? 1 : 0);
