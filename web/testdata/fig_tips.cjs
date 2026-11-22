// 그림 설명 툴팁·크게 보기(itda-doc#121 — share.js figTarget·figText·figFocus·figTipPlace·figTips) 시험. Go 시험(figtip_test.go)이
// pager.js·slides.js·share.js 경로를 넘긴다. DOM·크게 보기 부품(ItdaLightbox)은 작은 흉내다 — 상태·이벤트(누가 받고 누가 막는지)만 본다.
// 실제 배치·좌표·인쇄·화면 읽기 프로그램은 브라우저로 본다.
"use strict";
const fs = require("fs");
const vm = require("vm");
const files = process.argv.slice(2);
const ctx = { console };
ctx.window = ctx;
vm.createContext(ctx);
for (const f of files) vm.runInContext(fs.readFileSync(f, "utf8"), ctx);
const S = ctx.window.ItdaShare;
let cases = 0, failed = 0;
function ok(name, cond, detail) {
  cases++;
  if (!cond) { failed++; console.log("실패 " + name + " — " + JSON.stringify(detail === undefined ? null : detail)); } else console.log("통과 " + name);
}
const eq = (name, got, want) => ok(name, JSON.stringify(got) === JSON.stringify(want), { got, want });

// ---- 작은 가짜 DOM: 태그·class·속성 선택자(쉼표·자손 한 단계), 붙잡기·거품, stopPropagation·preventDefault
function simple(e, sel) {
  sel = sel.trim();
  const m = /^([a-z]+)?((?:\.[\w-]+)*)(?:\[([\w-]+)(?:="([^"]*)")?\])?$/i.exec(sel);
  if (!m) throw new Error("흉내가 모르는 선택자: " + sel);
  if (m[1] && e.tagName !== m[1].toUpperCase()) return false;
  if (m[2]) for (const c of m[2].split(".").filter(Boolean)) if (!e.cls().includes(c)) return false;
  if (m[3]) { if (!e.hasAttribute(m[3])) return false; if (m[4] !== undefined && e.getAttribute(m[3]) !== m[4]) return false; }
  return true;
}
function matches(e, sel) {
  return sel.split(",").some(part => {
    const steps = part.trim().split(/\s+/);
    if (!simple(e, steps[steps.length - 1])) return false;
    let x = e.parentElement;
    for (let i = steps.length - 2; i >= 0; i--) {
      while (x && !(x.isEl && simple(x, steps[i]))) x = x.parentElement;
      if (!x) return false;
      x = x.parentElement;
    }
    return true;
  });
}
class El {
  constructor(tag, attrs, kids) {
    this.isEl = true;
    this.tagName = tag.toUpperCase();
    this.attrs = Object.assign({}, attrs || {});
    this.children = [];
    this.parentElement = null;
    this.listeners = [];
    this.style = {};
    this.hidden = false;
    this.text = "";
    this.rect = { left: 100, top: 100, width: 200, height: 100, right: 300, bottom: 200 };
    for (const k of kids || []) this.appendChild(k);
  }
  cls() { return (this.attrs.class || "").split(/\s+/).filter(Boolean); }
  get className() { return this.attrs.class || ""; }
  set className(v) { this.attrs.class = String(v); }
  getAttribute(k) { return Object.prototype.hasOwnProperty.call(this.attrs, k) ? this.attrs[k] : null; }
  setAttribute(k, v) { this.attrs[k] = String(v); }
  hasAttribute(k) { return Object.prototype.hasOwnProperty.call(this.attrs, k); }
  removeAttribute(k) { delete this.attrs[k]; }
  get tabIndex() { return this.hasAttribute("tabindex") ? Number(this.attrs.tabindex) : -1; }
  set tabIndex(v) { this.attrs.tabindex = String(v); }
  appendChild(c) { if (c.parentElement) c.remove(); c.parentElement = this; this.children.push(c); return c; }
  remove() { const p = this.parentElement; if (p) { p.children = p.children.filter(x => x !== this); this.parentElement = null; } }
  replaceChildren(...kids) { for (const k of this.children) k.parentElement = null; this.children = []; for (const k of kids) this.appendChild(k); }
  get textContent() { return this.text + this.children.map(c => c.textContent).join(""); }
  set textContent(v) { for (const k of this.children) k.parentElement = null; this.children = []; this.text = String(v); }
  contains(x) { for (let e = x; e; e = e.parentElement) if (e === this) return true; return false; }
  closest(sel) { for (let e = this; e && e.isEl; e = e.parentElement) if (matches(e, sel)) return e; return null; }
  querySelectorAll(sel) {
    const out = [];
    const walk = e => { for (const c of e.children) { if (matches(c, sel)) out.push(c); walk(c); } };
    walk(this);
    return out;
  }
  addEventListener(t, fn, o) { this.listeners.push({ t, fn, cap: o === true || !!(o && o.capture) }); }
  getBoundingClientRect() { return this.rect; }
  get offsetWidth() { return 120; }
  get offsetHeight() { return 30; }
  get isConnected() { let e = this; while (e.parentElement) e = e.parentElement; return e === doc; }
  focus() { const was = doc.activeElement; if (was === this) return; doc.activeElement = this; if (was && was.isEl) fire(was, "focusout"); fire(this, "focusin"); }
  blur() { if (doc.activeElement === this) { doc.activeElement = doc.body; fire(this, "focusout"); } }
}
const doc = new El("#document");
doc.isEl = false; // closest 가 문서에서 멈추게
doc.body = doc.appendChild(new El("body"));
doc.activeElement = doc.body;
doc.createElement = t => new El(t);
const win = { listeners: [], innerWidth: 1000, innerHeight: 800, addEventListener: El.prototype.addEventListener };
function fire(target, type, props) {
  const ev = Object.assign({ type, target, defaultPrevented: false, stopped: false, button: 0, key: "",
    preventDefault() { this.defaultPrevented = true; }, stopPropagation() { this.stopped = true; } }, props || {});
  const path = [];
  for (let e = target; e; e = e.parentElement) path.push(e);
  path.push(win);
  const call = (node, cap) => { for (const l of node.listeners) if (l.t === type && l.cap === cap) l.fn(ev); };
  for (let i = path.length - 1; i >= 0 && !ev.stopped; i--) call(path[i], true);
  for (let i = 0; i < path.length && !ev.stopped; i++) call(path[i], false);
  return ev;
}

// 가짜 크게 보기 부품 — 연 대상·캡션 글을 적는다
function fakeLightbox() {
  const L = { made: 0, opened: [], caps: [], closed: 0 };
  L.create = (root, opts) => {
    L.made++;
    let kind = "";
    const el = root.appendChild(new El("div", { class: "lb " + (opts.cls || "") }));
    const inner = el.appendChild(new El("div", { class: "lb-fig" }));
    L.inst = { el, inner, open(h) { kind = "img"; L.opened.push(h); L.caps.push(opts.caption ? opts.caption(h) : ""); }, close() { if (!kind) return false; kind = ""; L.closed++; return true; },
      relayout() {}, onScrollbar: () => false, kind: () => kind };
    return L.inst;
  };
  return L;
}

// 본문: 쪽 둘 — 설명 그림, 빈 alt 그림, 공백 alt 그림, 링크 그림(설명), 링크 그림(빈 alt), YouTube 대표 그림, 로고, HTML 처럼 보이는 설명
function page() {
  const imgs = {
    desc: new El("img", { src: "a.png", alt: "고정 IP 생성 및 연결 — 이름 eta-mini-ip" }),
    empty: new El("img", { src: "b.png", alt: "" }),
    blank: new El("img", { src: "c.png", alt: " 　\t" }),
    link: new El("img", { src: "d.png", alt: "링크 그림 설명" }),
    linkEmpty: new El("img", { src: "e.png", alt: "" }),
    yt: new El("img", { src: "f.png", alt: "영상 대표 그림" }),
    logo: new El("img", { src: "logo.png", alt: "로고", class: "sc-logo" }),
    html: new El("img", { src: "g.png", alt: "<b>굵게</b> & <script>x()</script>" }),
  };
  const a = new El("a", { href: "https://example.com/" }, [imgs.link]);
  const a2 = new El("a", { href: "#h" }, [imgs.linkEmpty]);
  const yt = new El("a", { class: "yt", href: "https://youtu.be/x", "data-yt-id": "x" }, [imgs.yt]);
  const p1 = new El("div", { class: "page" }, [new El("div", { class: "page-body" }, [new El("p", {}, [imgs.desc]), imgs.empty, imgs.blank, new El("p", {}, [a]), a2])]);
  const p2 = new El("div", { class: "page" }, [new El("div", { class: "page-body" }, [yt, imgs.html]), imgs.logo]);
  return { imgs, a, nodes: [p1, p2] };
}

function setup(o) {
  o = o || {};
  doc.body.replaceChildren();
  doc.activeElement = doc.body;
  win.listeners = [];
  const scope = doc.body.appendChild(new El("div", { id: "pages" }));
  const viewer = doc.body.appendChild(new El("div", { id: "viewer" }));
  const stage = doc.body.appendChild(new El("div", { id: "stage" }));
  const pg = page();
  scope.replaceChildren(...pg.nodes);
  const L = o.noLb ? null : fakeLightbox();
  const st = { active: o.active !== false };
  // 보는 화면의 다른 keydown(쪽 넘김·목차 서랍 Esc — share.js keyHandler 자리)과 링크 처리(onClick 자리)를 흉내로 단다
  const seen = { key: [], click: [] };
  doc.addEventListener("keydown", e => seen.key.push(e.key));
  doc.addEventListener("click", e => seen.click.push(e.target));
  const F = S.figTips({ doc, win, scope, viewer, active: () => st.active, lightbox: () => L, scale: () => 0.8 });
  F.arm();
  return { F, L, scope, viewer, stage, st, seen, imgs: pg.imgs, a: pg.a };
}

// 1. figText — 앞뒤 공백(유니코드 공백 포함)을 뺀 alt, 공백뿐·없음은 ""
eq("figText: 설명", S.figText(new El("img", { alt: "  지표 탭 — CPU  " })), "지표 탭 — CPU");
eq("figText: 빈 alt·공백뿐(전각 공백·탭·줄바꿈)·alt 없음", [S.figText(new El("img", { alt: "" })), S.figText(new El("img", { alt: "　 \t\n" })), S.figText(new El("img"))], ["", "", ""]);
eq("figText: 가운데 줄바꿈은 둔다", S.figText(new El("img", { alt: "첫 줄\n둘째 줄" })), "첫 줄\n둘째 줄");

// 2. figTarget — 범위 안 본문의 img 만, 로고·숨은 단계·본문 밖·범위 밖·img 아님은 null
{
  const s = setup();
  ok("figTarget: 본문 그림", S.figTarget(s.imgs.desc, s.scope) === s.imgs.desc);
  ok("figTarget: 로고(.sc-logo — 본문 밖)는 아니다", S.figTarget(s.imgs.logo, s.scope) === null);
  const inBodyLogo = new El("img", { alt: "x", class: "sf-logo" });
  s.scope.children[0].children[0].appendChild(inBodyLogo);
  ok("figTarget: 본문 안 로고(.sf-logo)도 아니다", S.figTarget(inBodyLogo, s.scope) === null);
  const pending = new El("img", { alt: "x" });
  s.scope.children[0].children[0].appendChild(new El("li", { class: "pr-pending" }, [pending]));
  ok("figTarget: 숨은 단계(.pr-pending) 안은 아니다", S.figTarget(pending, s.scope) === null);
  const onStage = s.stage.appendChild(new El("div", { class: "page-body" }, [new El("img", { alt: "x" })])).children[0];
  ok("figTarget: 측정 무대(#stage — 범위 밖)는 아니다", S.figTarget(onStage, s.scope) === null);
  ok("figTarget: img 가 아니면 null", S.figTarget(s.a, s.scope) === null && S.figTarget(null, s.scope) === null);
}

// 3. figTipPlace — 아래에 들면 아래, 아니면 위, 둘 다 안 되면 겹침, 가로는 창 안으로
eq("자리: 그림 아래 가운데", S.figTipPlace({ left: 100, top: 100, width: 200, height: 100, bottom: 200 }, 120, 30, 1000, 800), { left: 140, top: 208 });
eq("자리: 아래가 모자라면 위", S.figTipPlace({ left: 100, top: 600, width: 200, height: 180, bottom: 780 }, 120, 30, 1000, 800), { left: 140, top: 562 });
eq("자리: 위아래 다 모자라면(쪽을 채운 그림) 그림 아래쪽 안에 겹친다", S.figTipPlace({ left: 0, top: 0, width: 1000, height: 800, bottom: 800 }, 120, 30, 1000, 800), { left: 440, top: 762 });
eq("자리: 왼쪽·오른쪽 끝은 창 안으로", [S.figTipPlace({ left: -50, top: 100, width: 40, height: 40, bottom: 140 }, 120, 30, 1000, 800).left,
  S.figTipPlace({ left: 980, top: 100, width: 40, height: 40, bottom: 140 }, 120, 30, 1000, 800).left], [8, 872]);
eq("자리: 그림이 창 위아래로 넘치면 창 안 아래쪽에 겹친다", S.figTipPlace({ left: 100, top: 10, width: 200, height: 1190, bottom: 1200 }, 120, 30, 1000, 800).top, 762);
eq("자리: 아래로 넘치는 그림도 위에 자리가 있으면 위", S.figTipPlace({ left: 100, top: 300, width: 200, height: 900, bottom: 1200 }, 120, 30, 1000, 800).top, 262);

// 4. arm — 링크 밖 그림만 tabindex, 표지는 크게 보기(zoom)·설명만(tip), 빈 alt 링크 그림·로고는 손대지 않는다
{
  const s = setup();
  const I = s.imgs;
  eq("arm(부품 있음): data-fig", [I.desc, I.empty, I.blank, I.link, I.linkEmpty, I.yt, I.logo, I.html].map(x => x.getAttribute("data-fig")), ["zoom", "zoom", "zoom", "tip", null, "tip", null, "zoom"]);
  eq("arm(부품 있음): tabindex 는 링크 밖 그림만", [I.desc, I.empty, I.link, I.linkEmpty, I.yt, I.logo].map(x => x.getAttribute("tabindex")), ["0", "0", null, null, null, null]);
  const before = s.scope.listeners.length + win.listeners.length;
  s.F.arm(); s.F.arm();
  ok("arm 을 거듭 불러도 듣기가 늘지 않고 표지가 같다", s.scope.listeners.length + win.listeners.length === before && I.desc.getAttribute("data-fig") === "zoom", { before, after: s.scope.listeners.length + win.listeners.length });
  const t0 = new El("img", { alt: "직접 tabindex", tabindex: "-1" });
  s.scope.children[0].children[0].appendChild(t0);
  s.F.arm();
  ok("arm: 이미 있는 tabindex 는 덮지 않는다", t0.getAttribute("tabindex") === "-1" && t0.getAttribute("data-fig") === "zoom");
}
{
  const s = setup({ noLb: true });
  const I = s.imgs;
  eq("arm(부품 없음 — 뷰어): 설명 그림만 tip, 빈 alt 는 손대지 않는다", [I.desc, I.empty, I.blank, I.link, I.html].map(x => x.getAttribute("data-fig")), ["tip", null, null, "tip", "tip"]);
  eq("arm(부품 없음): tabindex 는 링크 밖 설명 그림만", [I.desc, I.empty, I.link].map(x => x.getAttribute("tabindex")), ["0", null, null]);
}
{
  const s = setup({ active: false });
  ok("arm(대상 꼴 아님 — 보드): 표지를 달지 않는다", !s.imgs.desc.hasAttribute("data-fig") && !s.imgs.desc.hasAttribute("tabindex"));
  fire(s.imgs.desc, "pointerover", { pointerType: "mouse" });
  ok("대상 꼴 아님: 올려도 툴팁 없음", !s.F.tipOn());
  const e = fire(s.imgs.desc, "click");
  ok("대상 꼴 아님: 눌러도 크게 보기 없음", s.L.opened.length === 0 && !e.defaultPrevented);
}

// 5. 마우스 올림 — 설명 그림이면 글자만 넣은 툴팁 하나(aria-hidden), 터치는 아니다, 빈 alt 는 없다, 떠나면 닫는다
{
  const s = setup();
  fire(s.imgs.desc, "pointerover", { pointerType: "mouse" });
  const t = s.F.tip();
  ok("올림: 툴팁이 열린다", s.F.tipOn() && t && !t.hidden && t.textContent === "고정 IP 생성 및 연결 — 이름 eta-mini-ip");
  ok("툴팁은 body 직계·aria-hidden·글자 자식 없음(textContent)", t.parentElement === doc.body && t.getAttribute("aria-hidden") === "true" && t.children.length === 0);
  ok("툴팁 자리는 그림 아래(figTipPlace)", t.style.left === "140px" && t.style.top === "208px", t.style);
  ok("alt·aria 를 그림에 다시 넣지 않는다", s.imgs.desc.getAttribute("alt") === "고정 IP 생성 및 연결 — 이름 eta-mini-ip" && !s.imgs.desc.hasAttribute("aria-label") && !s.imgs.desc.hasAttribute("aria-describedby"));
  fire(s.imgs.desc, "pointerout", { pointerType: "mouse" });
  ok("떠나면 닫힌다", !s.F.tipOn() && t.hidden && t.textContent === "");
  fire(s.imgs.html, "pointerover", { pointerType: "mouse" });
  ok("HTML 처럼 보이는 설명은 글자 그대로(요소를 만들지 않는다)", t.textContent === "<b>굵게</b> & <script>x()</script>" && t.children.length === 0);
  fire(s.imgs.html, "pointerout", { pointerType: "mouse" });
  fire(s.imgs.empty, "pointerover", { pointerType: "mouse" });
  fire(s.imgs.blank, "pointerover", { pointerType: "pen" });
  ok("빈 alt·공백 alt 는 툴팁 없음", !s.F.tipOn() && t.hidden);
  fire(s.imgs.desc, "pointerover", { pointerType: "touch" });
  ok("터치 올림은 툴팁이 아니다(탭은 크게 보기)", !s.F.tipOn());
  fire(s.imgs.link, "pointerover", { pointerType: "mouse" });
  ok("링크 그림도 올리면 설명", s.F.tipOn() && t.textContent === "링크 그림 설명");
  fire(s.imgs.link, "pointerout", { pointerType: "mouse" });
  fire(s.imgs.logo, "pointerover", { pointerType: "mouse" });
  ok("로고는 툴팁 없음", !s.F.tipOn());
  ok("툴팁 요소는 하나", doc.body.querySelectorAll(".fig-tip").length === 1);
  fire(s.imgs.desc, "pointerover", { pointerType: "mouse" });
  fire(s.viewer, "scroll");
  ok("굴리면 닫힌다", !s.F.tipOn());
  fire(s.imgs.desc, "pointerover", { pointerType: "mouse" });
  for (const l of win.listeners) if (l.t === "resize") l.fn({});
  ok("창 크기가 바뀌면 닫힌다", !s.F.tipOn());
}

// 6. 키보드 포커스 — 그림 포커스·링크 포커스(그 안 설명 그림)면 열고, 포커스가 떠나면 닫는다
{
  const s = setup();
  s.imgs.desc.focus();
  ok("포커스: 그림", s.F.tipOn() && s.F.tip().textContent.startsWith("고정 IP"));
  s.imgs.desc.blur();
  ok("포커스 이탈: 닫힌다", !s.F.tipOn());
  s.a.focus();
  ok("포커스: 링크 → 그 안 설명 그림", s.F.tipOn() && s.F.tip().textContent === "링크 그림 설명");
  s.a.blur();
  ok("링크 포커스 이탈: 닫힌다", !s.F.tipOn());
  s.imgs.empty.focus();
  ok("빈 alt 그림 포커스는 툴팁 없음", !s.F.tipOn());
}

// 7. Esc — 툴팁을 닫고(다른 Esc 처리 — 목차 서랍 — 도 그대로 간다), 같은 그림은 떠났다 올 때까지 다시 열지 않는다
{
  const s = setup();
  fire(s.imgs.desc, "pointerover", { pointerType: "mouse" });
  const e = fire(s.imgs.desc, "keydown", { key: "Escape" });
  ok("Esc: 툴팁 닫힘·막지 않음·목차 서랍 Esc 도 받는다", !s.F.tipOn() && !e.defaultPrevented && s.seen.key.includes("Escape"));
  fire(s.imgs.desc, "pointerover", { pointerType: "mouse" });
  ok("Esc 뒤 같은 그림 위에서는 다시 열지 않는다", !s.F.tipOn());
  fire(s.imgs.desc, "pointerout", { pointerType: "mouse" });
  fire(s.imgs.desc, "pointerover", { pointerType: "mouse" });
  ok("떠났다 다시 오면 연다", s.F.tipOn());
}

// 8. 누르기 — 링크 밖 그림은 크게 보기(설명 캡션), 링크·YouTube 그림은 링크 동작 그대로(막지 않음·크게 보기 없음)
{
  const s = setup();
  let e = fire(s.imgs.link, "click");
  ok("링크 그림 누르기: 크게 보기 없음·막지 않음·링크 처리로 간다", s.L.opened.length === 0 && !e.defaultPrevented && s.seen.click.includes(s.imgs.link));
  e = fire(s.imgs.yt, "click");
  ok("YouTube 대표 그림 누르기: 크게 보기 없음·막지 않음", s.L.opened.length === 0 && !e.defaultPrevented);
  e = fire(s.imgs.desc, "click", { ctrlKey: true });
  ok("Ctrl 누르기는 크게 보기가 아니다", s.L.opened.length === 0);
  e = fire(s.imgs.desc, "click", { button: 1 });
  ok("가운데 단추는 크게 보기가 아니다", s.L.opened.length === 0);
  fire(s.imgs.desc, "pointerover", { pointerType: "mouse" });
  e = fire(s.imgs.desc, "click");
  ok("링크 밖 그림 누르기: 크게 보기가 열린다(같은 img)", s.F.lbOn() && s.L.opened[0] === s.imgs.desc && e.defaultPrevented);
  ok("크게 보기 캡션은 렌더된 설명", s.L.caps[0] === "고정 IP 생성 및 연결 — 이름 eta-mini-ip");
  ok("크게 보기가 열리면 툴팁은 닫힌다", !s.F.tipOn());
  ok("크게 보기: 대화상자 표지·닫기 단추 보임·대화상자에 포커스", s.L.inst.el.getAttribute("role") === "dialog" && !s.L.inst.el.hasAttribute("aria-modal")
    && doc.body.querySelectorAll(".fig-lb-close").length === 1 && !doc.body.querySelectorAll(".fig-lb-close")[0].hidden && doc.activeElement === s.L.inst.el);
  fire(s.imgs.empty, "pointerover", { pointerType: "mouse" });
  ok("크게 보기가 열린 동안 툴팁을 열지 않는다", !s.F.tipOn());
  // 바탕(.lb 자신) 누르기는 닫고, 안쪽(그림 상자) 누르기는 닫지 않는다
  fire(s.L.inst.inner, "click");
  ok("크게 보기 안쪽 누르기는 닫지 않는다", s.F.lbOn());
  fire(s.L.inst.el, "click");
  ok("바탕 누르기: 닫고 원래 그림으로 포커스", !s.F.lbOn() && doc.activeElement === s.imgs.desc && doc.body.querySelectorAll(".fig-lb-close")[0].hidden);
  ok("돌아온 포커스로 툴팁이 다시 열리지 않는다", !s.F.tipOn());
  fire(s.imgs.empty, "click");
  ok("빈 alt 그림도 크게 보기(캡션 없음)", s.F.lbOn() && s.L.caps[1] === "");
  fire(doc.body.querySelectorAll(".fig-lb-close")[0], "click");
  ok("닫기 단추: 닫고 포커스 복귀", !s.F.lbOn() && doc.activeElement === s.imgs.empty);
  ok("크게 보기 부품은 한 번만 만든다", s.L.made === 1);
}

// 8b. 크게 보기를 닫아 포커스가 그림에 남아도, 마우스를 떠났다 다시 올리면 툴팁을 연다(포커스 억제와 포인터 억제는 따로)
{
  const s = setup();
  fire(s.imgs.desc, "click");
  fire(s.L.inst.el, "click");
  ok("닫은 뒤: 포커스는 그림·툴팁 없음", doc.activeElement === s.imgs.desc && !s.F.tipOn());
  fire(s.imgs.desc, "pointerout", { pointerType: "mouse" });
  fire(s.imgs.desc, "pointerover", { pointerType: "mouse" });
  ok("포커스가 남아 있어도 다시 올리면 연다", s.F.tipOn());
  fire(s.imgs.desc, "pointerout", { pointerType: "mouse" });
  s.imgs.desc.blur(); s.imgs.desc.focus();
  ok("포커스가 떠났다 돌아오면 포커스로도 연다", s.F.tipOn());
}

// 9. 키보드로 열기·열린 동안 키 우선순위 — Enter·Space 로 열고(쪽 넘김으로 가지 않음), 열린 동안 Space·화살표는 뒤 화면으로 가지 않고, Esc 는 크게 보기만 닫는다
{
  const s = setup();
  s.imgs.desc.focus();
  let e = fire(s.imgs.desc, "keydown", { key: " " });
  ok("Space(포커스한 그림): 크게 보기·쪽 넘김으로 가지 않음", s.F.lbOn() && e.defaultPrevented && !s.seen.key.includes(" "));
  e = fire(doc.activeElement, "keydown", { key: " " });
  ok("열린 동안 Space: 뒤 화면 keydown 으로 가지 않고 막는다", s.F.lbOn() && e.defaultPrevented && !s.seen.key.includes(" "));
  e = fire(doc.activeElement, "keydown", { key: "ArrowDown" });
  ok("열린 동안 화살표: 가지 않고 막는다", e.defaultPrevented && !s.seen.key.includes("ArrowDown"));
  e = fire(doc.activeElement, "keydown", { key: "w", metaKey: true });
  ok("열린 동안 ⌘ 조합은 브라우저 몫(막지 않음)", !e.defaultPrevented);
  e = fire(doc.activeElement, "keydown", { key: "Tab" });
  const btn = doc.body.querySelectorAll(".fig-lb-close")[0];
  ok("열린 동안 Tab: 닫기 단추로", e.defaultPrevented && doc.activeElement === btn);
  e = fire(btn, "keydown", { key: "Enter" });
  ok("닫기 단추 위 Enter 는 단추 그대로(막지 않음)", !e.defaultPrevented && s.F.lbOn());
  e = fire(btn, "keydown", { key: "Escape" });
  ok("열린 동안 Esc: 크게 보기만 닫고 목차 서랍 Esc 로 가지 않는다·포커스 복귀", !s.F.lbOn() && e.defaultPrevented && !s.seen.key.includes("Escape") && doc.activeElement === s.imgs.desc);
  e = fire(s.imgs.desc, "keydown", { key: "Enter" });
  ok("Enter(포커스한 그림): 크게 보기", s.F.lbOn() && e.defaultPrevented);
  fire(doc.activeElement, "keydown", { key: "Escape" });
  e = fire(s.imgs.desc, "keydown", { key: "Enter", isComposing: true });
  ok("한글 조합 중 Enter 는 아니다", !s.F.lbOn());
  e = fire(s.imgs.link, "keydown", { key: "Enter" });
  ok("링크 그림 Enter 는 링크 몫(크게 보기 없음·막지 않음)", !s.F.lbOn() && !e.defaultPrevented);
  e = fire(s.imgs.desc, "keydown", { key: "PageDown" });
  ok("닫힌 뒤 쪽 넘김 키는 다시 뒤 화면으로 간다", s.seen.key.includes("PageDown") && !e.defaultPrevented);
}

// 10. 본문 교체 — 열린 툴팁·크게 보기를 닫고(포커스는 떨어진 그림으로 돌리지 않음), 새 그림만 다룬다, 듣기·툴팁 요소는 그대로 하나
{
  const s = setup();
  const before = s.scope.listeners.length + win.listeners.length;
  fire(s.imgs.desc, "click");
  ok("교체 전: 열림", s.F.lbOn());
  const old = s.imgs.desc;
  const pg = page();
  s.scope.replaceChildren(...pg.nodes);
  s.F.arm();
  ok("교체 뒤: 크게 보기 닫힘·떨어진 그림으로 포커스를 돌리지 않음", !s.F.lbOn() && doc.activeElement !== old);
  ok("교체 뒤: 새 그림에 표지", pg.imgs.desc.getAttribute("data-fig") === "zoom" && pg.imgs.desc.getAttribute("tabindex") === "0");
  fire(pg.imgs.desc, "pointerover", { pointerType: "mouse" });
  s.scope.replaceChildren(...page().nodes);
  s.F.arm();
  ok("교체 뒤: 옛 그림의 툴팁이 남지 않는다(유령 없음)", !s.F.tipOn() && s.F.tip().hidden);
  ok("교체 뒤: 듣기 수 그대로·툴팁 요소 하나·닫기 단추 하나", s.scope.listeners.length + win.listeners.length === before
    && doc.body.querySelectorAll(".fig-tip").length === 1 && doc.body.querySelectorAll(".fig-lb-close").length === 1);
  fire(pg.imgs.desc, "click");
  ok("떨어진 옛 그림 누르기는 크게 보기가 아니다", !s.F.lbOn());
  s.F.reset();
  ok("reset: 닫힌 상태", !s.F.tipOn() && !s.F.lbOn());
}

// 11. 부품 없음(설치 없는 뷰어) — 누르기·Enter 는 지금처럼 아무 일도 없다(막지 않음·쪽 넘김 그대로), 툴팁은 된다
{
  const s = setup({ noLb: true });
  let e = fire(s.imgs.desc, "click");
  ok("부품 없음: 누르기는 막지 않고 아무 일도 없다", !e.defaultPrevented && !s.F.lbOn());
  s.imgs.desc.focus();
  ok("부품 없음: 포커스 툴팁은 된다", s.F.tipOn());
  e = fire(s.imgs.desc, "keydown", { key: " " });
  ok("부품 없음: Space 는 쪽 넘김으로 간다", !e.defaultPrevented && s.seen.key.includes(" "));
}

console.log(`FIGTIP DONE cases=${cases} failed=${failed}`);
