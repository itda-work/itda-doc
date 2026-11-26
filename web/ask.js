// 호스트가 set(cfg, host)로 강의 MCP와 번들 문서를 지정하면 armSections(flow)가 원문 직계 제목에
// 서비스별 단추를 단다. 쪽·장으로 나누기 전에 같은 id의 순번을 매긴다. 누르면 섹션 참조와 설명 요청을
// 담은 새 대화를 연다. 단추는 제목의 첫 자식으로 넣어 CSS(base.css .itda-section-ask)가 제목 오른쪽 끝에 띄운다.
// 아이콘은 CSS가 그리고, 읽는 이름은 aria-label, 툴팁은 브라우저 기본 툴팁(title — 첫 줄 이름, 둘째 줄 안내)이다.
// 단추 안에 글자 노드가 없어 제목 글자의 복사·검색에 섞이지 않는다.
(function (root) {
  "use strict";

  const services = {
    claude: { name: "Claude", url: "https://claude.ai/new?q=" },
    chatgpt: { name: "ChatGPT", url: "https://chatgpt.com/?q=" },
  };
  const notice = "질문은 AI 앱의 새 대화로 열립니다.";
  const selector = "button.itda-section-ask";
  const str = v => typeof v === "string" ? v.trim() : "";
  const displayLimit = 80;
  let current = null;

  // 지침의 표시 이름만 줄인다. 코드 포인트 단위로 세어 이모지의 서로게이트 쌍을 보존한다.
  function displayName(value) {
    const chars = Array.from(str(value).replace(/\s+/g, " "));
    return chars.length <= displayLimit ? chars.join("") : chars.slice(0, displayLimit - 1).join("") + "…";
  }

  function headingName(h) {
    if (!h.querySelector(".notes")) return displayName(h.textContent);
    const copy = h.cloneNode(true);
    for (const note of copy.querySelectorAll(".notes")) note.remove();
    return displayName(copy.textContent);
  }

  function normalize(cfg) {
    if (!cfg || !Array.isArray(cfg.services)) return null;
    const enabled = Object.keys(services).filter(s => cfg.services.includes(s));
    const mcp = str(cfg.mcp), doc_id = str(cfg.doc_id);
    if (!enabled.length || !doc_id) return null;
    try {
      if (!/^https:\/\//i.test(mcp) || new URL(mcp).protocol !== "https:") return null;
    } catch { return null; }
    return { services: enabled, mcp, doc_id, course: displayName(cfg.course), doc: displayName(cfg.doc), guide: str(cfg.guide) };
  }

  function set(cfg, host) {
    current = normalize(cfg);
    if (current && !current.doc) current.doc = displayName(host && host.title);
    return current;
  }

  function armSections(box) {
    const counts = new Map();
    for (const h of box.children) {
      if (!/^H[1-6]$/.test(h.tagName)) continue;
      for (const b of h.querySelectorAll(":scope > " + selector)) b.remove();
      if (!current || !str(h.id) || h.closest(".notes")) continue;
      const nth = (counts.get(h.id) || 0) + 1;
      counts.set(h.id, nth);
      const title = headingName(h), at = h.firstChild;
      for (const service of current.services) {
        const b = root.document.createElement("button");
        b.type = "button";
        b.className = "itda-section-ask";
        const label = services[service].name + "에 묻기";
        b.dataset.label = label;
        b.setAttribute("aria-label", label);
        b.title = label + "\n" + notice;
        b.dataset.service = service;
        b.dataset.section = h.id;
        b.dataset.nth = String(nth);
        b.dataset.title = title;
        if (at) h.insertBefore(b, at); else h.appendChild(b);
      }
    }
  }

  function question(cfg, ref) {
    const doc = [cfg.course, cfg.doc].filter(Boolean).join(" · ");
    return [
      (doc ? "「" + doc + "」의 " : "") + "「" + ref.title + "」 섹션과 하위 섹션 전부를 강의 MCP(" + cfg.mcp + ")로 읽어 주세요.",
      "read_section(doc=" + JSON.stringify(cfg.doc_id) + ", section=" + JSON.stringify(ref.section) + ", nth=" + ref.nth + ", subtree=true)로 읽고, 여러 조각이면 끝까지 읽어 주세요.",
      "내 생각을 먼저 물어보고, 교안을 그대로 옮기지 말고 처음 배우는 사람에게 설명해 주세요.",
      connectLine(cfg),
    ].join("\n");
  }

  // connectLine 은 커넥터가 없을 때의 연결 안내 줄이다 — 섹션 묻기(question)와 용어 묻기(termQuestion)가 같이 쓴다.
  function connectLine(cfg) {
    return cfg.guide ? "커넥터가 연결돼 있지 않으면 이 안내를 보여 주세요. " + cfg.guide : "커넥터가 연결돼 있지 않으면 연결 방법을 알려 주세요.";
  }

  // termQuestion 은 게시 화면 용어사전(glossary.js — itda-hyve#1084)의 「Claude 로 추가 설명받기」 지침이다. cfg 는 normalize 결과다.
  // 싣는 것은 강의 이름·용어·강의 MCP 주소·연결 안내뿐이다 — 풀이(def)·교안 본문은 싣지 않는다. 용어 바로 뒤에는 받침에 따라 바뀌는 조사를 두지 않는다(「'<용어>'의 뜻」).
  function termQuestion(cfg, term) {
    return [
      (cfg.course ? "「" + cfg.course + "」 " : "") + "강의 MCP(" + cfg.mcp + ")로 용어 '" + displayName(term) + "'의 뜻을 처음 배우는 사람에게 비유와 함께 설명하고, 이해했는지 질문 하나로 확인해줘.",
      connectLine(cfg),
    ].join("\n");
  }

  function handler(deps) {
    const open = deps && deps.open || (u => root.open(u, "_blank", "noopener"));
    return e => {
      const b = e.target.closest && e.target.closest(selector);
      if (!current || !b || b.closest(".present") || !current.services.includes(b.dataset.service)) return;
      e.preventDefault();
      e.stopPropagation();
      open(services[b.dataset.service].url + encodeURIComponent(question(current, b.dataset)));
    };
  }

  root.ItdaAsk = { normalize, set, armSections, handler, termQuestion };
})(typeof window !== "undefined" ? window : globalThis);
