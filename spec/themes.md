# 문서 테마 만들기 — 회사 테마 안내서

Itda Doc 문서의 테마를 만드는 사람을 위한 안내서다. 테마를 읽고 그리는 쪽은 Itda Doc 을 그리는 구현(예: Itda Hyve Docs 의 문서 창)이다. 설계는 [DESIGN-markdown-docs.md](https://github.com/itda-work/itda-hyve/blob/main/docs/DESIGN-markdown-docs.md) 3장, 문법은 1장이다.
견본 회사 테마: [`themes/samples/acme/`](../themes/samples/acme/).

테마는 **폴더 하나**다. 문서에는 뜻만 적고(제목·표·`:::box`), 모양은 테마가 정한다. 테마는 CSS 로 모양만 바꾸고,
HTML 구조·스크립트·원격 파일은 담지 못한다.

## 1. 가장 작은 회사 테마 — 색과 로고만

구현에서 만들기(예: Itda Hyve Docs 문서 창): 테마 고르기 옆 **⋯ → 회사 테마 만들기** 에 이름·바탕 테마·강조색·로고·조직 이름을 넣으면 아래 폴더를 만들고 바로 그 테마로 보여 준다.
**⋯ → 테마 폴더 열기** 로 Finder 에서 연다.

손으로 만들기: 사용자 테마 폴더(Itda Hyve Docs 는 `~/.itda/hyve/themes/`, `itda-doc` 명령은 `-themes DIR`)의 `acme/theme.json` 하나면 된다.

```json
{
  "title": "에이크미",
  "extends": "classic",
  "tokens": { "--accent": "#b3122e" },
  "logo": "assets/logo.svg",
  "defaults": { "organization": "에이크미 주식회사" }
}
```

```
~/.itda/hyve/themes/
  acme/
    theme.json
    assets/logo.svg
```

- 폴더 이름이 테마 이름이다. 문서는 frontmatter 에 `theme: acme` 로 고른다. 이름은 **소문자 영문·숫자·한글(완성형)·`-`·`_`**, 40자까지.
  대문자는 받지 않는다(대소문자를 가리지 않는 파일 시스템에서 `Acme` 와 `acme` 가 한 폴더가 된다).
- 내장 테마 다섯(`classic`·`bold`·`mono`·`compact`·`spine`)은 색이 모두 `--accent` 에서 나온다. `--accent` 하나만 바꿔도
  제목·표 머리·띠·요약문이 그 색 계열로 바뀐다(`mono` 는 흑백이라 강조색을 쓰지 않는다). 옅은 면·제목 번호 같은 파생 색은
  상대 색을 아는 엔진에서만 따라 바뀐다 — 옛 WebKit 에서는 바탕 테마의 색이 남는다(12장). 창의 "회사 테마 만들기" 는 파생 색을
  새 강조색으로 미리 계산해 `tokens` 에 함께 적으므로 어느 엔진에서나 같다.
- `defaults.organization` 은 문서에 `organization` 이 없을 때 표지·제목 블록·머리글의 `{organization}` 에 쓰인다.
- 폴더를 고치면 창이 1초 안에 다시 불러온다(창을 다시 띄우지 않아도 된다).
- 내장 테마와 이름(또는 별칭)이 겹치면 내장이 이기고, 사용자 테마는 "쓸 수 없음 — 이름이 겹침" 으로 보인다.

## 2. 폴더와 theme.json

```
<이름>/
  theme.json   (꼭) 아래 키들
  theme.css    (선택) 토큰으로 안 되는 모양
  template.md  (선택) "새 문서" 서식
  assets/      (선택) 로고·그림·글꼴
```

| 키 | 뜻 | 예 |
|---|---|---|
| `name` | 폴더 이름과 같아야 한다(적지 않아도 된다) | `"acme"` |
| `title` | 테마 고르기에 보일 이름(옆에 식별자가 함께 보인다 — `에이크미 — acme`) | `"에이크미"` |
| `description` | 한 줄 설명 | |
| `extends` | 물려받을 테마. 토큰·머리글·로고·CSS 를 물려받고 적은 것만 바꾼다(4단까지) | `"classic"` |
| `aliases` | 다른 이름(`theme:` 에 써도 되는 이름) | `["에이크미"]` |
| `tokens` | CSS 변수 값(3장). 어느 엔진에서나 쓰이는 정적 값 | `{"--accent": "#b3122e"}` |
| `derived` | 상대 색을 아는 엔진에서만 `tokens` 를 덮는 식(12장). 같은 이름의 정적 값이 `tokens`(또는 물려받은 테마)에 꼭 있어야 한다 | `{"--accent-soft": "oklch(from var(--accent) 0.955 calc(c * 0.14) h)"}` |
| `margin` | 쪽 여백 `top`·`right`·`bottom`·`left`(길이: `mm` `cm` `in` `pt` `pc` `px` `em` `rem`) | `{"top": "25mm"}` |
| `header`·`footer` | 머리글·바닥글의 `left`·`center`·`right` 틀(7장). 빈 글자 `""` 는 그 자리를 비운다 | `{"right": "{title}"}` |
| `cover` | `"auto"`(문서의 `cover: true` 를 따름) · `"never"`(표지 없이 제목 블록) | |
| `meta` | 표지·제목 블록의 정보 표에 보일 frontmatter 키(순서대로, 값 없는 키는 건너뜀) | `["doc-no","version","date"]` |
| `labels` | 정보 표의 이름표(키 → 글자). 모르는 키는 키 자체가 이름표 | `{"date": "작성일"}` |
| `logo` | 표지·제목 블록의 로고(`assets/` 아래 svg·png·jpg·webp) | `"assets/logo.svg"` |
| `defaults` | 문서에 없을 때 쓸 frontmatter 값(글자로만, `{ }` 없이) | `{"organization": "…"}` |
| `h1-break` | 장(h1) 앞에서 새 쪽. `true` 는 쪽이 1/3 넘게 찼을 때만, `"always"` 는 늘 | `"always"` |
| `toc-break` | 목차 뒤에서 새 쪽 | `true` |
| `list-style` | 문서가 `list-style` 을 적지 않았을 때의 목록 기호 사다리(4장 "목록 기호 사다리"). `"gaejosik"`·`"gongmun"`·`"plain"` 또는 둘 다 `["gaejosik", "gongmun"]`. 적지 않으면 물려받거나 보통 목록 | `"gongmun"` |

모르는 키는 거부한다(오타 `"foter"` 를 조용히 넘기지 않는다). 쪽번호는 표지를 빼고 센다.

## 3. 토큰 — 전체 표

토큰은 CSS 변수다. `theme.json` 의 `tokens` 로 값을 주고, `theme.css` 에서 `var(--이름)` 으로 쓴다. 아래는 공통 `base.css` 가 쓰는 토큰과
기본값이다(테마가 하나도 주지 않아도 문서가 된다). 값에는 `;` `{` `}` `<` `>` `\`·`url(`·`@`·주석을 쓸 수 없다 — 그림은 `logo`·`theme.css` 로.
`var()`·`color-mix()` 는 쓸 수 있다. 상대 색 `oklch(from var(--accent) …)` 은 옛 WebKit 이 몰라 토큰이 통째로 무효가 되므로
`tokens` 가 아니라 `derived` 에 두고 정적 값을 `tokens` 에 함께 적는다(12장). 이 표는 테스트(`TestTokenTableMatchesBaseCSS`)가 `base.css` 와 맞춰 본다.

<!-- tokens:begin -->
| 토큰 | 뜻 | 기본값 |
|---|---|---|
| `--font-body` | 본문 글꼴 | `"Pretendard", "Apple SD Gothic Neo", "Malgun Gothic", sans-serif` |
| `--font-heading` | 제목 글꼴 | `var(--font-body)` |
| `--font-mono` | 코드 글꼴 | `ui-monospace, "SF Mono", Menlo, Consolas, monospace` |
| `--font-math` | 수식 글꼴(OpenType MATH 표가 있는 수학 글꼴이어야 한다 — 큰 괄호·∑ 를 그린다). 이 글꼴에 없는 글자(`\text{…}` 의 한글 등)는 본문 글꼴로. 수식 쓰는 법은 [수식 안내서](math.md) | `"STIX Two Math", "Cambria Math"` |
| `--text-size` | 본문 글자 크기 | `10.5pt` |
| `--line-height` | 줄 간격 | `1.7` |
| `--paragraph-gap` | 문단 아래 간격 | `6pt` |
| `--list-indent` | 목록 한 단계의 들여쓰기 폭(기호는 이 여백 안에 글자 머리에 붙어 선다) | `1.6em` |
| `--list-indent-gongmun` | 공문 사다리(`list-style: gongmun`)의 순서 목록 한 단계 폭 — `(가)`·`(10)` 이 들어가게 넓다 | `2.2em` |
| `--list-ul-1` | 개조식 사다리(`list-style: gaejosik`) 1단계 기호(CSS 글자열 — 뒤 빈칸까지) | `"□ "` |
| `--list-ul-2` | 개조식 2단계 | `"○ "` |
| `--list-ul-3` | 개조식 3단계 | `"- "` |
| `--list-ul-4` | 개조식 4단계(더 깊으면 되풀이) | `"· "` |
| `--list-ol-1` | 공문 사다리(`list-style: gongmun`) 1단계 번호 꼴(`list-style-type` 값) — `1.` | `decimal` |
| `--list-ol-2` | 공문 2단계 — `가.` | `hyve-ganada-dot` |
| `--list-ol-3` | 공문 3단계 — `1)` | `hyve-decimal-paren` |
| `--list-ol-4` | 공문 4단계 — `가)` | `hyve-ganada-paren` |
| `--list-ol-5` | 공문 5단계 — `(1)` | `hyve-decimal-parens` |
| `--list-ol-6` | 공문 6단계 — `(가)` | `hyve-ganada-parens` |
| `--list-ol-7` | 공문 7단계 — `①` | `hyve-circled` |
| `--list-ol-8` | 공문 8단계 — `㉮`(더 깊으면 되풀이) | `hyve-circled-ganada` |
| `--ink` | 글자색 | `#23272f` |
| `--muted` | 옅은 글자색(이름표·캡션) | `#6b7280` |
| `--accent` | 강조색 — 제목·표 머리·띠가 여기서 나온다 | `#2b4a7e` |
| `--accent-ink` | 강조색 면 위의 글자색 | `#ffffff` |
| `--accent-soft` | 옅은 강조색 면 | `#eef2f8` |
| `--rule` | 가는 선 | `#cfd5de` |
| `--rule-strong` | 굵은 선(결재란 칸) | `#23272f` |
| `--paper` | 쪽 바탕 | `#ffffff` |
| `--link` | 링크 색 | `var(--accent)` |
| `--link-hover-ink` | 화면에서 링크를 가리킬 때 글자색(hover 는 color-mix 를 아는 엔진에서만. 평소 밑줄은 링크 색을 옅게, PDF 에도) | `color-mix(in srgb, var(--accent) 75%, black)` |
| `--link-hover-bg` | 가리킬 때 링크 뒤 면 | `var(--accent-soft)` |
| `--link-hover-line` | 가리킬 때 밑줄 색 | `var(--accent)` |
| `--page-margin-top` | 쪽 위 여백(`margin.top` 과 같다) | `25mm` |
| `--page-margin-right` | 쪽 오른쪽 여백 | `20mm` |
| `--page-margin-bottom` | 쪽 아래 여백 | `22mm` |
| `--page-margin-left` | 쪽 왼쪽 여백 | `20mm` |
| `--header-top` | 머리글의 쪽 위 끝에서 거리 | `11mm` |
| `--footer-bottom` | 바닥글의 쪽 아래 끝에서 거리 | `10mm` |
| `--header-size` | 머리글·바닥글 글자 크기 | `7.5pt` |
| `--header-color` | 머리글·바닥글 글자색 | `var(--muted)` |
| `--header-rule` | 머리글 아래 선 | `0.5pt solid var(--rule)` |
| `--footer-rule` | 바닥글 위 선 | `none` |
| `--h1-size` | 장 제목 크기 | `17pt` |
| `--h1-color` | 장 제목 색 | `var(--accent)` |
| `--h1-weight` | 장 제목 굵기 | `700` |
| `--h1-gap-top` | 장 제목 위 간격 | `4pt` |
| `--h1-gap-bottom` | 장 제목 아래 간격 | `10pt` |
| `--h2-size` | 절 제목 크기 | `13pt` |
| `--h2-color` | 절 제목 색 | `var(--ink)` |
| `--h2-weight` | 절 제목 굵기 | `700` |
| `--h2-gap-top` | 절 제목 위 간격 | `16pt` |
| `--h3-size` | 항 제목 크기 | `11.5pt` |
| `--h3-color` | 항 제목 색 | `var(--ink)` |
| `--h3-weight` | 항 제목 굵기 | `600` |
| `--h3-gap-top` | 항 제목 위 간격 | `12pt` |
| `--h4-size` | h4~h6 크기 | `10.5pt` |
| `--table-size` | 표 글자 크기 | `9.5pt` |
| `--table-head-bg` | 표 머리 바탕 · 화면의 정렬 머리 칸 포커스·올림 표시는 이 바탕과 창 모양에 상관없이 남색 `#1e3a8a`·흰색 두 겹이라 한 겹이 이 바탕과 3:1 이상이다(비텍스트 색 대비 1.4.11 식, #528 — `dist/app.css` `th.sortable`, 테마가 `th` 의 `outline`·`box-shadow` 를 덮으면 미검증) | `#eef2f8` |
| `--table-head-ink` | 표 머리 글자색 | `var(--ink)` |
| `--table-border` | 표 칸 선 | `var(--rule)` |
| `--table-top-rule` | 표 맨 위 선 | `1.2pt solid var(--accent)` |
| `--table-cell-pad` | 칸 안 여백 | `3.5pt 6pt` |
| `--table-stripe` | 짝수 줄 바탕 | `transparent` |
| `--code-bg` | 코드 바탕 | `#f5f6f8` |
| `--code-size` | 코드 블록 글자 크기 | `8.6pt` |
| `--code-kw` | 키워드 글자색 | `#7135a6` |
| `--code-str` | 문자열 글자색 | `#0b6847` |
| `--code-com` | 주석 글자색 | `#596579` |
| `--code-num` | 숫자 글자색 | `#9a4100` |
| `--code-fn` | 함수 글자색 | `#174b9b` |
| `--code-type` | 타입 글자색 | `#185e85` |
| `--code-op` | 연산자 글자색 | `#7b3434` |
| `--code-diff-add` | diff 추가 바탕 | `#d9f2e3` |
| `--code-diff-del` | diff 삭제 바탕 | `#fce1df` |
| `--code-title-fg` | 코드 파일명 머리 띠 글자색 · 화면의 코드 복사 단추 아이콘색(#525) | `#596579` |
| `--code-title-line` | 코드 파일명 머리 띠 아래 구분선 | `#dfe3e9` |
| `--code-title-size` | 코드 파일명 머리 띠 글자 크기 | `0.8em` |
| `--code-hl-bg` | 코드 줄 강조 띠 배경(발표 노트 칸은 어두운 값 `#4a4020`) | `#fdf0c4` |
| `--code-hl-bar` | 코드 줄 강조 띠 왼쪽 막대(2pt, 노트 칸 `#e6c04a`) | `#d4a017` |
| `--ln-w` | 코드 줄 번호 칸 너비(자릿수만큼 — `pre[data-ln-digits]`가 1·3~5자리를 바꾼다) | `2ch` |
| `--quote-bar` | 인용 왼쪽 막대 | `var(--rule)` |
| `--quote-ink` | 인용 글자색 | `var(--ink)` |
| `--quote-size` | 인용 글자 크기(본문 대비) | `0.9em` |
| `--quote-border` | 인용 상자 테두리(왼쪽 띠 말고 세 변) | `var(--rule)` |
| `--quote-bg` | 인용 상자 배경 | `color-mix(in srgb, var(--accent) 4%, var(--paper))` |
| `--box-border` | `:::box` 테두리 | `0.75pt solid var(--rule)` |
| `--box-bg` | `:::box` 바탕 | `transparent` |
| `--box-radius` | `:::box` 모서리 | `3pt` |
| `--img-border` | 본문 그림 테두리(그림만 있는 문단의 그림 — 글 줄 속 아이콘·로고 제외). 없애려면 `none` | `0.75pt solid color-mix(in srgb, var(--rule) 60%, var(--paper))` |
| `--img-radius` | 본문 그림 모서리 | `2pt` |
| `--lead-size` | `:::lead` 글자 크기 | `11.5pt` |
| `--lead-bar` | `:::lead` 왼쪽 막대 | `3pt solid var(--accent)` |
| `--lead-bg` | `:::lead` 바탕 | `transparent` |
| `--alert-note` | 알림 `[!NOTE]` 색 | `#2f6fdb` |
| `--alert-tip` | 알림 `[!TIP]` 색 | `#1f8a4c` |
| `--alert-important` | 알림 `[!IMPORTANT]` 색 | `#7c4dcc` |
| `--alert-warning` | 알림 `[!WARNING]` 색 | `#b7791f` |
| `--alert-caution` | 알림 `[!CAUTION]` 색 | `#c53030` |
| `--alert-bg-alpha` | 알림 바탕의 진하기 | `8%` |
| `--chart-1` | 차트 계열 1의 색(막대·선·조각) — 기본은 강조색. 차트 쓰는 법은 [차트 안내서](charts.md) | `var(--accent)` |
| `--chart-2` | 차트 계열 2 | `#eb6834` |
| `--chart-3` | 차트 계열 3 | `#1baf7a` |
| `--chart-4` | 차트 계열 4 | `#eda100` |
| `--chart-5` | 차트 계열 5 | `#008300` |
| `--chart-6` | 차트 계열 6 | `#e87ba4` |
| `--chart-7` | 차트 계열 7 | `#4a3aa7` |
| `--chart-8` | 차트 계열 8 | `#e34948` |
| `--chart-text` | 차트 글자(항목·값·범례) | `var(--ink)` |
| `--chart-muted` | 차트 눈금·단위 글자 | `var(--muted)` |
| `--chart-grid` | 차트 격자선 | `#e3e6eb` |
| `--chart-axis` | 차트 0 기준선 | `#8d95a1` |
| `--chart-on` | 색 면 위의 값 글자(쌓은 막대·원 조각 안) | `#ffffff` |
| `--chart-area-opacity` | 면 차트(area)의 채움 불투명도 | `0.16` |
| `--diagram-node-bg` | 그림(mermaid 흐름도·간트) 노드·막대 바탕. 쓰는 법은 [그림 안내서](diagrams.md) | `var(--accent-soft)` |
| `--diagram-node-border` | 그림 노드 테두리 | `var(--accent)` |
| `--diagram-edge` | 그림 화살표 선·화살촉 | `var(--muted)` |
| `--diagram-text` | 그림 글자 | `var(--ink)` |
| `--diagram-muted` | 그림 묶음(subgraph) 제목 | `var(--muted)` |
| `--diagram-label-bg` | 그림 화살표 글의 바탕 | `var(--paper)` |
| `--diagram-cluster-bg` | 그림 묶음 바탕 | `var(--code-bg)` |
| `--diagram-cluster-border` | 그림 묶음 테두리 | `var(--rule)` |
| `--diagram-grid` | 간트 눈금 격자선(#409) | `var(--rule)` |
| `--diagram-exclude-bg` | 간트 제외 날짜 띠 | `var(--rule)` |
| `--diagram-bar-done` | 간트 `done` 막대 | `var(--muted)` |
| `--diagram-bar-active` | 간트 `active` 막대·마일스톤 | `var(--accent)` |
| `--diagram-bar-crit` | 간트 `crit` 막대·마일스톤 | `var(--alert-caution)` |
| `--diagram-bar-crit-border` | 간트 `crit` 테두리 | `var(--rule-strong)` |
| `--slide-pad-x` | 슬라이드 좌우 여백 | `64pt` |
| `--slide-pad-top` | 슬라이드 위 여백 | `44pt` |
| `--slide-pad-bottom` | 슬라이드 아래 여백(번호 자리 포함) | `52pt` |
| `--slide-scale` | 슬라이드 글자 배율 — 아래 `--slide-*-size` 에 곱한다. frontmatter `slides.font-scale`(0.6~1.5)이 문서마다 바꾼다 | `0.9` |
| `--slide-text-size` | 슬라이드 본문 글자 크기(문서의 `--text-size` 대신) | `23pt` |
| `--slide-line-height` | 슬라이드 줄 간격 | `1.45` |
| `--slide-title-size` | 슬라이드 제목 크기 | `33pt` |
| `--slide-title-color` | 슬라이드 제목 색 | `var(--ink)` |
| `--slide-section-size` | 구역 슬라이드 제목 크기 | `48pt` |
| `--slide-cover-size` | 제목 슬라이드의 문서 제목 크기 | `50pt` |
| `--slide-table-size` | 슬라이드의 표 글자 | `16pt` |
| `--slide-code-size` | 슬라이드의 코드 글자 | `14pt` |
| `--slide-lead-size` | 슬라이드의 요약문(lead) 글자 | `27pt` |
| `--slide-bg` | 슬라이드 바탕 | `var(--paper)` |
| `--slide-scrim` | 배경 그림 `{.full .fill}` 위에 덮는 스크림 바탕(#569) — 글이 읽히게 그림을 어둡게. 색 하나로 둔다(반투명 `linear-gradient` 는 슬라이드 PDF 에서 불투명 검정으로 찍힌다 — 2026-11-02 macOS 슬라이드 PDF 실측) | `rgba(0, 0, 0, 0.4)` |
| `--slide-scrim-shade` | 스크림의 위·아래를 더 어둡게 하는 안쪽 그림자(`box-shadow` 값, 없애려면 `none`) | `inset 0 -260px 220px -60px rgba(0, 0, 0, 0.6), inset 0 160px 140px -60px rgba(0, 0, 0, 0.35)` |
| `--slide-scrim-ink` | 스크림 위 바탕 없는 글 색 — 슬라이드 제목, 본문 맨 위(`.page-body` 직접 자식) 제목·문단·목록·정의 목록·구역 제목과 그 안의 굵은 글·링크. 인용·알림·상자·표·코드는 맨 위든 목록 항목 안이든 테마 색 그대로(목록 안 블록은 base.css 가 `--ink` 로 되돌린다 — 특이도 0,1,0 이라 테마가 정한 글자 색이 이김) — 그래서 그 블록은 바탕이 있어야 한다: 맨 위·목록 안 표는 base.css 가 종이색 판을 깔고(테마 바탕이 이김), `--box-bg`·`--lead-bg` 가 투명인 테마는 `.slide-bg.fill ~ .page-body { --box-bg: …; --lead-bg: … }` 로 덮는다(내장 compact·mono·bold) | `#ffffff` |
| `--slide-scrim-muted` | 스크림 위 바닥글·「(계속)」·캡션, 맨 위 목록의 기호·맨 위 글 링크 밑줄 색 | `rgba(255, 255, 255, 0.82)` |
| `--caption-size` | 그림·표 캡션 크기 | `9pt` |
| `--caption-color` | 캡션 색 | `var(--muted)` |
| `--figure-label` | 그림 번호 앞 글자(CSS 글자열) | `"그림"` |
| `--table-label` | 표 번호 앞 글자 | `"표"` |
| `--page-w` | 쪽 너비 — 앱이 용지(`papersize`)로 정한다, 테마가 줄 수 없음 | `210mm` |
| `--page-h` | 쪽 높이 — 앱이 정한다, 테마가 줄 수 없음 | `297mm` |
<!-- tokens:end -->

테마는 자기 토큰을 더 만들어 `theme.css` 에서 써도 된다(내장 `classic` 의 `--accent-2`·`--accent-3`, `spine` 의 `--highlight`).
창 틀이 쓰는 이름(`--ui-…`·`--wails-…`·`--editor-w` 등 `app.css` 의 변수)과 `--page-w`·`--page-h`·`--img-max-h` 는 줄 수 없다.

## 4. HTML 구조와 클래스

구조는 앱이 정한 한 가지다. 테마는 이 클래스에 CSS 를 건다.

**쪽 상자** — 화면이 A4 상자로 직접 나눈다(인쇄·PDF 도 같다).

```html
<div class="page">               <!-- 표지 쪽은 class="page page-cover" (머리글·바닥글 없음) -->
  <div class="page-header"><div class="slot left">…</div><div class="slot center">…</div><div class="slot right">…</div></div>
  <div class="page-body"> …문서 흐름… </div>
  <div class="page-footer"> …같은 세 자리… </div>
</div>
```

**표지**(`cover: true` 이고 테마가 `cover: "never"` 가 아닐 때)

```html
<section class="cover">
  <div class="cover-top"><img class="cover-logo"> <div class="cover-org">조직</div> <div class="cover-classification">대외비</div></div>
  <div class="cover-main"><h1 class="cover-title">제목</h1><p class="cover-subtitle">부제</p></div>
  <dl class="cover-meta"><div class="meta-row" data-key="doc-no"><dt>문서번호</dt><dd>…</dd></div>…</dl>
  <div class="cover-bottom"><div class="cover-date">날짜</div><div class="cover-org">조직</div></div>
</section>
```

**제목 블록**(표지가 없고 `title` 이 있을 때)

```html
<header class="title-block"><img class="title-logo"><div class="title-org">조직</div><h1 class="title">제목</h1>
  <p class="subtitle">부제</p><dl class="title-meta">…meta-row 들…</dl></header>
```

**목차**: `<section class="toc-page"><h2 class="toc-title">목차</h2><nav class="toc"><ul><li><a><span class="toc-section-number">1</span> 제목 <span class="toc-pno">3</span></a>…`

**본문**: 제목 번호 `<span class="header-section-number">1.2</span>`(끝의 점은 `h1 > .header-section-number::after` 로 붙인다), 그림
`<figure><img><figcaption>`(번호는 `--figure-label` + CSS 카운터), 표 `<table><caption>`, 알림 `<div class="markdown-alert markdown-alert-note">
<p class="markdown-alert-title">참고</p>…`, 각주 `<section class="footnotes">`, 쪽을 넘어 이어지는 조각은 `.cont`(목록 항목은 `li.cont`).
메타 행은 `data-key` 로 고를 수 있다: `.meta-row[data-key="classification"] dd { color: #b42318 }`.
코드 블록(2줄 이상)은 `<pre class="has-ln" data-ln-digits="N"><code><span class="ln" data-n="1">…</span>…` 이고 줄 번호 칸은 `code` 의 왼쪽 안쪽(`padding-left`, 번호는 `.ln::before`)이다 — 테마는 `pre` 의 `padding`·`border` 를 마음대로 바꿔도 되지만 `pre code` 의 `padding`·`display` 는 덮지 않는다(번호가 코드 글자와 겹친다). 파일명을 적은 코드 블록(여는 펜스에 ```` ```yaml:docker-compose.yml ```` 같이)은 `<pre data-title="…">` 이고 띠는 `pre[data-title]::before`(`content: attr(data-title)`)가 pre 흐름 안 첫 줄로 그린다(#441) — 띠 색·구분선·글자 크기는 `--code-title-*` 토큰, 띠의 오른쪽 패딩은 화면의 복사 단추 자리이니 `pre[data-title]::before` 의 `padding-right` 를 줄이지 않는다. 화면(미리보기·발표·공유 보기)에서는 이 띠를 누르면 파일명을 복사하는 단추 `pre > button.hyve-codetitle`(#516 — `::before` 로 파일명을 그린다)가 `::before` 띠 대신 같은 자리에 그린다 — `--code-title-*` 토큰은 두 쪽에 같이 걸리지만, 테마가 `pre[data-title]::before` 를 직접 덮은 규칙은 화면의 단추 띠에 걸리지 않는다(인쇄·PDF·사이드바 축소본·발표 한눈에 보기처럼 단추를 숨기고 `::before` 띠를 그대로 그리는 곳에만 걸린다). 화면의 두 단추(복사·파일명 띠) 색은 창 모양(밝게·어둡게)을 따르지 않고 밝은 코드 바탕에 맞춘다(#525 — `dist/app.css` `button.hyve-codecopy`·`button.hyve-codetitle`): 복사 아이콘은 `--code-title-fg`, 올리면 `--ink`, 띠를 올렸을 때·「✓ 복사됨」·포커스 테두리는 고정 `#2563eb` 이다(본문·밝은 노트 칸). 어두운 발표 노트 칸(`.pr-notes-body`)은 예외로 고정 어두운 배경용 색(아이콘 `#9aa3b0`, 올리면 `#e6e9ee`, 띠 올림 `#e6e9ee`, 「✓ 복사됨」 `#8fdfb2`, 포커스 `#8db4ff`)을 쓴다 — 테마가 `--code-bg` 를 어둡게 바꾸거나 `--ink` 를 밝게 바꾸면 이 색들의 대비는 미검증이다. 강조할 줄을 적은 코드 블록(```` ```go {2-3} ````·Deckset `[.code-highlight: 2-3]`)은 `<pre data-hl="2-3" class="… has-hl">` 이고 그 줄이 `<span class="ln hl">` 다(#479 — 줄 번호를 끈 문서·1줄 블록에서도 감싼다). 강조 줄은 `display: block` 으로 code 가로 전체(번호 칸 포함)와 양옆 4pt 에 띠를 깐다 — 테마는 띠 색을 `--code-hl-bg`·`--code-hl-bar` 로 바꾸고, `.ln.hl` 의 `display`·`margin`·`padding` 은 덮지 않는다(띠가 번호 칸과 어긋난다).

### 목록 기호 사다리 — 개조식·공문

한국 보고서·공문의 단계별 항목 기호를 문서에 글자로 적지 않고 테마가 그린다(#141). 문서는 보통 `-`·`1.` 목록을 들여 써서
단계를 나누고, frontmatter `list-style`(없으면 `theme.json` 의 `list-style`)이 사다리를 켠다. 원문·HTML 은 보통 목록 그대로라
다른 뷰어에서는 점·숫자 목록으로 보인다.

| 값 | 목록 | 단계별 기호(토큰) |
|---|---|---|
| `gaejosik` | 비순서 목록(`-`) | `□` → `○` → `-` → `·` (`--list-ul-1…4`) |
| `gongmun` | 순서 목록(`1.`) | `1.` → `가.` → `1)` → `가)` → `(1)` → `(가)` → `①` → `㉮` (`--list-ol-1…8`) |
| `plain` | 둘 다 끔 | 테마 기본을 끄고 보통 목록(• ◦ ▪ / 1. 2.) |

- **단계**는 같은 종류 목록의 중첩 깊이다(`ul ul`·`ol ol ol`). 사다리보다 깊으면 마지막 단계를 되풀이한다. 두 값은 서로 다른 목록에 걸리므로 함께 켤 수 있다(`[gaejosik, gongmun]`). 문서의 값은 테마 기본을 통째로 바꾼다.
- **들여쓰기**는 단계마다 같은 폭(`--list-indent`, 공문은 `--list-indent-gongmun`)이고 기호는 그 여백 안에서 글자 머리에 붙어 선다(`::marker`). 줄이 넘치면 기호 뒤 글자 머리에 맞춰 들어간다(내어쓰기). 가장 큰 번호(시작 번호 + 항목 수 − 1)가 두 자리 이상인 순서 목록은 `<ol class="ol-d2">`…`ol-d9`(아홉 자리 이상은 `ol-d9`)이고, 그 목록만 들여쓰기를 `max(들여쓰기 토큰, 자릿수 × 0.66em + 0.6em)`(공문 사다리는 `+ 1.1em`)으로 넓혀 번호 앞자리가 쪽 본문 밖으로 나가 잘리지 않게 한다(#575 — `web/base.css`, `mdoc/transform.go` `olDigits`). 한 자리 목록에는 class 가 없어 위 폭 그대로다. 각주 목록도 각주가 열 개 이상이면 같은 class 를 달고(`mdoc/render.go` `footnoteList`) 기본 1.4em 을 같은 공식(`+ 0.6em`)으로 넓힌다. 본문 목록 규칙은 특이도 0 이라 테마가 순서 목록의 들여쓰기나 기호 꼴(`--list-ol-N`)을 바꾸면 테마가 이긴다 — 그때 10번 이상 번호가 잘리지 않는지는 테마가 맡는다.
- **번호**는 목록의 시작 번호(`3.` 으로 시작하면 `다.`)와 쪽을 넘어 이어지는 조각의 번호를 따른다. 가나다는 행정업무운영 편람대로 `가`\~`하` 다음 `거`\~`허`(28)이고 넘으면 숫자(`29.`), 원문자는 `①`\~`⑳` 다음 `(21)`\~`(50)`, `㉮`\~`㉻` 다음 `(거)`\~`(허)` 이고 그 너머는 숫자.
- 내장 테마 중 **`mono`(모노)만 기본으로 둘 다 켠다** — 흑백 격식 꼴이 주로 담는 공공 문서는 개조식 보고(`□ ○`)와 공문 항목 번호(`1. 가.`)가 거의 늘 쓰이고, 공문 항목 구분은 규정(행정업무운영 편람)이다. `classic`·`bold`·`compact`·`spine` 은 보통 목록이 기본이고, 개조식으로 쓸 문서가 `list-style: gaejosik` 으로 고른다.
- **회사 테마**는 토큰으로 기호를 바꾼다: `"--list-ul-4": "\"▸ \""`(P02 꼴), `"--list-ol-1": "upper-roman"`. 꼴 이름은 CSS 기본 꼴(`decimal`·`upper-roman`·`hangul`…)이나 위의 `hyve-…` 다(`theme.css` 에는 `@counter-style` 을 둘 수 없다). 기호 글자는 Pretendard 에 있는 것이 좋다 — `□ ○ · ① … ⑳` 은 있고 `㉮ … ㉻` 는 없어 시스템 글꼴(macOS Apple SD Gothic Neo, Windows 맑은 고딕)로 그린다.
- 기호를 글자로 적은 목록(`- ○ 내용`, `1. 가. 내용`, `2. 본문` 아래 줄 `가. 내용`)은 알아보지 않고 경고(`list-marker`)만 낸다 — 점과 기호가 겹쳐 `• ○ 내용` 이 된다.
- 테마 CSS 가 목록 모양을 정하면 테마가 이긴다: 사다리와 보통 목록의 기본 꼴·들여쓰기는 특이도 0(`:where()`)이라 `.decisions ol { list-style: none }` 같은 규칙이 중첩 단계까지 그대로 맞는다. 목차·각주 안의 목록에는 사다리가 걸리지 않는다.
- 옛 WebKit(Safari 17 미만)은 `@counter-style` 을 몰라 공문 사다리의 2단계 이하가 숫자로 보인다(개조식 기호는 보인다).

### 차트·슬라이드·이어보기

- **차트**(`:::chart`): `<figure class="chart chart-<종류>">` 안의 `<svg class="chart-svg t-<종류>">`. 색은 클래스 `s1`…`s8` 이 토큰 `--chart-1…8` 로 칠한다 — 테마는 토큰만 주면 된다(1번은 강조색이 좋다. 이웃한 색끼리 색약에도 구분되게). 글자는 `--chart-text`·`--chart-muted`, 선은 `--chart-grid`·`--chart-axis`. SVG 안 요소의 클래스: 막대 `col`(음수 `neg`)·쌓은 조각 `seg`·선 `line`·점 `dot`·면 `area`·원 조각 `slice`·값 `val`(색 면 위는 `val in`)·눈금 `tick`·항목 `cat`·범례 `legend`/`sw`/`lg`. 움직임은 앱이 하므로 테마가 `@keyframes` 를 둘 필요가 없다(둘 수도 없다).
- **슬라이드**: 같은 쪽 상자(`.page`)가 슬라이드가 된다(1280×720 — 인쇄 960×540pt, 4:3 은 960×720). 테마 CSS 는 쪽 상자 자신을 고르지 못하므로 슬라이드 종류는 안쪽 요소의 클래스로 고른다: `k-title`(제목)·`k-section`(구역 — 뒤에 내용이 없고 배경 그림 장이 아니면 `bare` 도)·`k-content`(내용)·`k-single`(그림·차트·표·요약문 하나). 구조:

  ```
  .page.slide
    .slide-deco.k-…          장식 자리(테마가 ::before·::after 로 띠·면을 칠한다)
    .slide-head.k-…          제목(.slide-title — 제목 번호 .header-section-number, 이어진 슬라이드는 .slide-cont "(계속)")
    .page-body.k-…           본문. 제목 슬라이드는 .slide-cover(.sc-top·.sc-logo·.sc-org·.sc-main·.sc-title·.sc-sub·.sc-meta),
                             구역 슬라이드는 .slide-section(.ss-num — 번호가 없는 문서는 구역 차례 — · .ss-title)
    .slide-foot.k-…          아래(.sf-logo·.sf-text 조직·.sf-num 번호)
  ```

  배경 그림 장(`{.full}`·`{.full .fill}`, #569)은 쪽의 첫 자식이 `.slide-bg`(cover 면 `.slide-bg.fill`)이고 **`.slide-deco` 가 없다**(`web/slides.js`) — `.slide-deco.k-section ~ .page-body` 같은 형제 선택자는 배경 장에 걸리지 않는다. 또 배경 장은 본문이 없는 구역 장이라도 **`bare` 가 아니다** — `bare` 는 장식 면 위 큰 제목을 뜻하므로 장식 없는 배경 장에는 붙이지 않는다(spine 「제 n 장」 꼬리표 같은 `bare` 전용 모양은 배경 장에 나오지 않는다). 스크림 위 모양은 `.slide-bg.fill ~ …` 로 고른다(내장 mono·spine 의 `.ss-num` 예, spine 은 번호 목록 원도).

  분할 그림 패널 장(`{.left}`·`{.right}`, itda-work/itda-hyve#550)은 쪽 직접 자식 `.slide-panel` 이 한쪽을 차지하고, `.slide-deco`·머리·본문·바닥은 반대쪽으로 좁아진다(`web/base.css` `panel-left`·`panel-right`). 왼쪽 패널 장에서 본문은 `left` 가 아니라 `margin-left` 로 밀린다 — 테마가 `.page-body` 의 `left` 를 정하면(구역 장을 장식 옆으로 미는 `.slide-deco.k-section ~ .page-body { left: … }` 꼴) 그 값은 그대로 남고, 패널 폭과 바깥 여백(`--slide-pad-x`)을 패널 간격(`--panel-gap`)으로 바꾼 차가 더해진다(itda-work/itda-hyve#581). 장식 상자 폭의 %로 그린 장식(classic 의 24% 띠)에 맞춘 본문 `left` 의 %는 쪽 폭 기준이라 패널만큼 넓게 잡히므로, 왼쪽 패널 장에서만 정해지는 `--slide-deco-x`(장식 상자 왼쪽 끝 = 패널 폭)로 `calc(24% - var(--slide-deco-x, 0%) * 0.24 + 60pt)` 처럼 뺀다(내장 classic). 패널 없는 장·오른쪽 패널 장에는 `--slide-deco-x` 가 없어 대체값(`0%`)이 쓰인다. 이 식은 테마 `left` 가 바깥 여백(`--slide-pad-x`)을 품는다고 보고 그 몫을 빼므로, `left` 가 `--slide-pad-x` 와 무관한 테마(classic 의 `24% + 60pt`)는 왼쪽 패널 장에서 들여쓰기가 `--slide-pad-x` 와 `--panel-gap` 의 차만큼 줄어든다 — 내장 classic(64pt)은 제목과 띠 선 사이가 25pt 로 남지만, 바깥 여백을 크게 잡은 회사 테마는 제목이 띠 선에 닿을 수 있다(계산, 미렌더). 이 보정이 없는 회사 테마의 왼쪽 패널 구역 장은 본문이 장식 상자 기준 자리보다 그 %×패널 폭만큼 더 오른쪽에서 시작한다(회사 테마로는 미검증). 오른쪽 패널 장은 본문 `left` 를 건드리지 않는다.

  크기·여백은 `--slide-*` 토큰이다. 슬라이드에서는 문서의 `--text-size`·`--h1-size`… 대신 `--slide-text-size`·`--slide-title-size` 에서 나온 값이 쓰인다. `--slide-scale`(기본 0.9)이 모든 `--slide-*-size` 글자에 곱해진다 — 테마가 이 토큰들을 덮어도 배율은 그대로 얹힌다. 슬라이드 전용 선택자(표지 `.sc-*`·구역 `.ss-*`·슬라이드 머리 `.slide-*`)의 `font-size` 는 `calc(Npt * var(--slide-scale))`·`calc(var(--slide-…-size) * 수 * var(--slide-scale))`·em·%·inherit 꼴만 쓴다(`font` 단축 속성은 금지) — 시험(`TestSlideFontSizeScaleGuard`, page/theme_test.go)이 이 허용 꼴만 통과시키고 다른 꼴(고정 pt·rem·var 참조 등)은 실패시킨다 — 내장 테마·base.css 에서 흔히 쓰는 꼴의 회귀를 잡는 검사다(완전한 CSS 해석기는 아니다). 관리 표시(선택자가 정확히 `.sc-class`·`.slide-foot`·`.sf-num`·`.sf-left`·`.sf-logo`)만 고정 크기를 쓸 수 있다. 본문 서식(`spine` 의 결과 상자·절차 번호처럼 쪽 문서와 함께 쓰는 것)의 고정 크기는 슬라이드에서도 그대로다. 내장 테마의 `theme.css` 끝에 슬라이드 절이 있다 — 표지의 띠·면, 장 머리의 꼴을 슬라이드로 옮긴 예다.
- **이어보기**: 쪽 나눔 없이 보는 꼴이다. 같은 쪽 상자 하나(`.page.page-flow`)에 문서 흐름이 통째로 들어가고 높이는 내용만큼이다 — 테마에게는 "아주 긴 쪽 하나" 라 같은 선택자가 그대로 맞는다. 머리글·바닥글은 없다. 표지(`cover: true`)는 종이 한 장이 아니라 맨 위의 제목 영역이 되고, 이어보기 전용 모양은 본문 상자의 클래스 **`.page-body.flow-view`** 로 고른다(예: `.page-body.flow-view .cover { … }`). 내장 `classic`·`bold`·`spine` 의 `theme.css` 끝에 그 예가 있다. 회사 테마는 바탕 테마의 것을 물려받는다. 출력(PDF·인쇄)은 이어보기에서도 쪽으로 나오므로 이어보기 모양은 화면에만 쓰인다.

## 5. 표준 어휘와 자기 어휘

문서는 `:::이름` 컨테이너로 뜻을 적는다(설계 1.5). 모든 테마가 그려야 하는 **표준 어휘**는 `base.css` 가 기본 모양을 준다:
`center`·`right`(정렬), `columns`·`column`(다단), `box`(테두리 상자), `lead`(요약문), `keep`(쪽 사이에서 쪼개지 않음), `signature`(서명란),
`approval`(결재란 — 안에 표). 테마는 이것을 다시 그려도 된다.

**자기 어휘**: 모르는 이름도 `<div class="이름">` 으로 나간다. 테마가 클래스에 모양을 주면 그 테마의 어휘가 된다.
다른 테마에서는 보통 문단으로 보이므로 뜻이 깨지지 않는다. 예(견본 `acme`):

```markdown
:::confidential
이 문서는 에이크미 임직원만 볼 수 있다.
:::
```

```css
.confidential { border: 1.2pt solid var(--accent); border-left-width: 6pt; padding: 6pt 11pt; color: var(--accent); font-weight: 700; }
.confidential::before { content: "배포 제한"; background: var(--accent); color: #fff; padding: 1pt 5pt; margin-right: 7pt; }
```

내장 테마의 자기 어휘: `compact` 의 `decisions`(결정 사항 상자, 항목마다 ✔)·`actions`(할 일 표), `spine` 의 `steps`(절차 단계)·`result`(단계의 결과).
쪽 나눔기는 `.keep`·그림·알림·`box`·결재란을 들어갈 쪽으로 통째로 옮긴다. 자기 어휘를 쪼개지 않으려면 문서에서 `:::keep` 으로 감싼다.

## 6. theme.css 에서 할 수 있는 것과 없는 것

테마 CSS 는 **쪽 상자 안에 갇힌다**: 앱이 `@scope (.page) { … }` 로 감싸 낸다. 선택자는 쪽 상자 안의 요소에만 맞고, 쪽 상자는
`contain: paint` 라 `position: fixed` 로도 쪽 밖을 덮지 못하며, 보기 영역은 확인창·메뉴 위로 올라오지 못한다.

| 할 수 있음 | 할 수 없음(불러올 때 거부하고 까닭을 보인다) |
|---|---|
| 클래스·요소 선택자, `::before`·`::after`, `:has()`·`:lang()` | 창 틀 꾸미기: `html`·`body`·`:root`·`:host`, 가두기를 벗어나는 `:scope`·맨 위의 `&` |
| `@media`(예: `print`)·`@supports` | `@import`·`@page`·`@layer`·`@keyframes`·`@property`·`@namespace`·`@counter-style` 등 다른 at 규칙 |
| `url(assets/…)` 로 테마 폴더의 그림·글꼴 | 원격·폴더 밖 `url()`, 글자로 주소를 받는 `image-set("…")`·`src("…")`·`image()` |
| `@font-face`(맨 위, `assets/*.woff2`) | 스크립트 흔적: `expression(`·`javascript:`·`behavior:`·`-moz-binding` |
| 중첩 규칙(`.a { &:hover {} }`) | 앱·창 틀의 글꼴 이름으로 `@font-face`(`Pretendard`·`-apple-system`·`sans-serif` 등) |

`.page` 자신(쪽 크기·바탕)은 고를 수 없다 — 바탕은 `--paper`, 크기는 용지가 정한다. 표지 쪽만 꾸미려면 `.cover` 를 쓴다.

**테마 폴더의 파일**: `css`·`json`·`md`·`svg`·`png`·`jpg`·`webp`·`woff2` 만, 파일 하나 8MB·테마 전체 32MB·200개까지, 폴더 4단까지.
링크(심볼릭 링크)는 따라가지 않는다(테마가 쓸 수 없음 — 테마 폴더 자체가 링크여도). 숨김 파일(`.DS_Store` 등)과 편집기 백업(`theme.css~`·`#theme.css#`)은 건너뛴다.
`~/.itda/hyve/themes` 자체는 링크여도 된다(팀 공유 폴더). 테마 SVG 는 문서 그림과 같이 스크립트를 막는 헤더(CSP `sandbox`)를 붙여 낸다.
사용자 테마는 불러올 때 메모리에 읽어 두고, 창이 그림·글꼴을 달라고 하면 그것을 준다(고른 뒤 파일을 바꿔치기해 다른 파일을 내주게 하는 길이 없다).

## 7. 머리글·바닥글 자리표시자

세 자리(`left`·`center`·`right`)에 글자와 자리표시자를 섞어 쓴다: `"{organization} · {title}"`.

- `{page}`·`{pages}` — 쪽번호와 본문 쪽 수(표지를 빼고 센다)
- `{title}`·`{subtitle}`·`{author}`·`{date}`·`{doc-no}`·`{version}`·`{classification}`·`{organization}`·`{lang}`
- frontmatter 의 다른 키도 그대로: `{시행일}`·`{고객사}`
- 값은 글자로만 들어간다(HTML 로 해석하지 않는다). 값이 없으면 빈 글자가 되고 옆의 구분자(` · `·` | `·` / ` 등)를 지운다.
  자리의 자리표시자가 모두 비면(쪽번호가 없을 때) 그 자리를 통째로 비운다 — `"시행 {시행일}"` 이 `"시행"` 만 남지 않는다.
- `defaults` 의 값은 문서에 없을 때 쓰인다.

## 8. template.md — 새 문서 서식

있으면 **새 문서 ▾** 메뉴에 테마 이름으로 보인다. frontmatter 에 `theme: <이 테마>` 와 채울 자리를, 본문에 그 문서의 흔한 구성을 둔다.
64KB 까지. 서식은 경고 없이 렌더돼야 한다(내장 테마는 `TestTemplatesRenderClean` 이 확인한다).

## 9. 글꼴 넣기

```css
@font-face { font-family: "Acme Sans"; src: url(assets/acme-sans-regular.woff2) format("woff2"); font-weight: 400; font-display: block; }
@font-face { font-family: "Acme Sans"; src: url(assets/acme-sans-bold.woff2) format("woff2"); font-weight: 700; font-display: block; }
```

```json
"tokens": { "--font-body": "\"Acme Sans\", \"Pretendard\", sans-serif" }
```

- woff2 만 받는다. `@font-face` 는 theme.css 맨 위에(`@media` 안이 아니라) 둔다. 서술자는 `font-family`·`src`·`font-weight`·`font-style`·
  `font-display`·`unicode-range`·`font-stretch`·`size-adjust`·`*-override` 만.
- `font-display: block` 을 권한다 — 화면이 글꼴을 다 받은 뒤 쪽을 나누므로(줄 수가 바뀌지 않게) PDF 와 화면이 같다.
- PDF 에 글꼴이 임베드된다(`pdffonts` 로 확인). 글꼴의 사용 허락(OFL 등)을 확인하라 — PDF 에 들어가 배포된다.
- 뒤에 `"Pretendard"` 를 두면 테마 글꼴에 없는 글자(한자 등)를 기본 글꼴로 채운다.

## 10. 확인하는 법

```sh
itda-hyve md --theme acme --export-pdf /tmp/acme.pdf 견본.md    # 창 없이 PDF. 테마가 틀리면 종료 코드 1 과 까닭
itda-hyve md --theme acme 견본.md                                 # 창으로 보며 고치기 — 폴더를 고치면 1초 안에 따라온다
pdffonts /tmp/acme.pdf                                            # 글꼴 임베드
```

- 창의 **⋯** 메뉴에 "쓸 수 없는 회사 테마" 와 까닭이 보인다(테마 고르기에서는 고를 수 없게 흐리게).
- `--export-pdf` 는 stdout 에 한 줄 JSON(`ok`·`pages`·`warnings`·`overflow`)을 쓴다. `overflow` 가 0 이 아니면 쪽에서 잘린 블록이 있다.
- 작업 폴더를 바로 보려면(숨은 옵션) `--themes-dir <폴더>`: `~/.itda/hyve/themes` 대신 그 폴더를 사용자 테마 폴더로 쓴다.
- 저장소에서: `just docview-check` 가 견본 × 테마(폴더에서 읽은 내장·견본 전부)의 PDF 를 쪽수·A4·쪽번호·머리글·표 머리 반복·글자 누락·글꼴 임베드로 센다.

## 11. 빌드 때 넣기(내장 테마로)

내장 테마는 렌더러 구현이 바이너리에 넣는다. 이 저장소의 내장 테마는 `themes/` 의 중립 다섯이다.

### 스타일 테마 — 남의 디자인 시스템에서 테마 만들기

한국 서비스 디자인 시스템(토스 TDS·카카오·네이버…)에서 온 스타일 테마를 만드는 레시피다(#366). 문법·폴더 구조·검사는 위와 같고, 색 토큰·타이포·슬라이드 장식만 남의 것이다.

**상표와 서체(#366 정책 C안)** — 폴더 이름은 회사명 소문자(`toss`), `aliases` 에 한글 별칭(`토스`)을 둔다. 로고·심볼 SVG·전용 서체 파일은 넣지 않는다. `description` 첫머리에 「비공식 — OO 디자인 시스템에서 영감」 을 적는다. 서체는 오픈 라이선스 글꼴(LINE Seed·Wanted Sans 등 — 테마마다 라이선스를 실측한다)만 9장 방식으로 임베드하고, 그 외는 Pretendard 기본 글꼴로 무드를 낸다.

**팔레트 → 토큰** — 색 값은 공식 디자인 문서에서 읽어 고정한다(기억으로 적지 않는다). 바탕 테마(`extends`)는 내장 구조 테마 5종 중에서 고른다 — 표지·머리글·목록 기호 같은 문서 구조는 바탕이 정한다.

| 토큰 | 어디서 정하나 |
|---|---|
| `--accent` | 대표 브랜드색(예: 카카오 옐로 `#FEE500` — 값은 테마 이슈에서 공식 문서로 실측) |
| `--accent-ink` | 강조색 면 위 글자색 — 밝은 브랜드색(옐로 등)이면 어두운 글자색으로 대비를 맞춘다 |
| `--chart-1..8` | 1번은 강조색. 디자인 시스템에 차트 팔레트가 있으면 그것을, 없으면 이웃한 색끼리 색약에도 구분되게 고른다(4장 "차트·슬라이드·이어보기") |
| `--slide-*` | 슬라이드 장식이 바탕과 다를 때만(같은 절) |

문서용 토큰(`--ink`·`--paper`·`--rule` 등)은 바탕을 물려받는 게 보통이다.

**`derived` 정적값 계산** — 강조색을 바꾼 테마는 바탕 테마의 파생 식을 새 색으로 미리 계산해 `tokens` 에 정적 값으로 함께 적는다 — 옛 WebKit 에서 바탕 색이 섞이지 않고, 지원하는 엔진에서도 같은 값이다(12장). 파생 식은 바탕마다 다르니 물려받은 `derived` 그대로를 계산한다(`classic` 의 넷 `--accent-2`·`--accent-3`·`--accent-soft`·`--lead-bg` 는 모두 `var(--accent)` 기반이고, `spine` 의 `--highlight-soft` 는 `--highlight` 기반이다). 창의 "회사 테마 만들기" 는 `var(--accent)` 식만 골라 계산한다(`page/usertheme.go` `createUserTheme`).

값은 `page/compat.go` 의 `evalRelColor`(OKLab 계산 — WebKit 이 그린 값과 같다, 내장 8개 실측)로 구한다. 패키지 안에서만 부를 수 있어 일회용 시험 파일을 잠깐 넣어 돌리고, 확인 뒤 지운다.

```go
// page/relcolor_scratch_test.go
package page

import "testing"

func TestScratchDerived(t *testing.T) {
    vals := map[string]string{"--accent": "#3182f6"} // 새 강조색
    for _, kv := range [][2]string{
        {"--accent-2", "oklch(from var(--accent) calc(l + 0.18) c h)"},
        {"--accent-3", "oklch(from var(--accent) calc(l + 0.37) calc(c * 0.65) h)"},
        {"--accent-soft", "oklch(from var(--accent) 0.955 calc(c * 0.14) h)"},
        {"--lead-bg", "oklch(from var(--accent) 0.972 calc(c * 0.09) h)"},
    } {
        v, err := evalRelColor(kv[1], vals)
        t.Logf("%s = %s (err %v)", kv[0], v, err)
    }
}
```

```sh
go test ./page/ -run TestScratchDerived -v
```

식은 `evalRelColor` 가 아는 꼴만 계산한다 — `oklch(from var(--x) L C h)` 에서 L 은 `l`·수·`calc(l ± n)`, C 는 `c`·`calc(c * n)`. 바탕 테마의 식을 그대로 베껴 쓴다.

**`theme.css`** — 슬라이드 장식 절을 중심으로 쓴다(4장의 슬라이드 구조 — `.slide-deco`·`.slide-cover`·`.slide-section`·`--slide-*`). 문서 절은 최소로 바탕을 물려받는다. `template.md`(선택)은 그 회사 무드의 발표 서식으로 둔다.

**문서·시험 연동** — 테마 하나를 만들 때 함께 손대는 곳:

| 어디 | 무엇 |
|---|---|
| `spec/markdown.md` 3장 테마 표(`themes:begin`~`end` 사이) | 이름·별칭 한 줄 — `TestGuideThemes`(`guidethemes_test.go`)가 안내서 표와 theme.json 의 이름·별칭을 맞춰 본다(빠지면 `go test ./page/` 가 실패한다) |
| `page/usertheme_test.go` | `TestUserThemeFolder` 의 목록 접두사(내장 순서·이름)와 내장 수 기대값을 새 테마에 맞춘다(빠지면 `go test ./page/` 가 실패한다) |
| `FEATURES.md` 내장 테마 소개 절 | 테마 소개 덧붙이기 |
| 이 문서 1장 | 내장 테마를 나열하는 문장에 이름 넣기 |
| `template.md` | 경고 없이 렌더된다 — `TestTemplatesRenderClean`(`save_test.go`)이 확인한다 |

`scripts/docview_check.py` 는 검사 목록을 바이너리에게서 받는다(#367) — 숨은 옵션 `itda-hyve md --theme-meta` 가 Go 로더가 푼 테마 메타데이터(이름·별칭·최종 cover·머리글·바닥글 슬롯, 쓸 수 없는 테마와 까닭)를 한 줄 JSON 으로 낸다. 쓸 수 없는 테마는 그 까닭과 함께 따로 보고하고 나머지 검사는 계속한다. 내장·견본 폴더의 테마 전부 × 견본 전부로 PDF 를 잰다. 검증은 #366 뼈대 그대로: `go test ./internal/docview/` · `just docview-check`(슬라이드는 `SLIDES=1`)·`ITDA_HYVE_DOCVIEW_OLD_WEBKIT=all` 정합성 · 발표 견본 눈 검수. 눈 검수·PIL 검사는 `just`(빌드 포함) 뒤의 렌더로 — `theme.css` 는 go:embed 라 바이너리 재빌드 없이 재렌더하면 낡은 꼴이 나온다(#386 사건).

## 12. 지원 엔진과 옛 WebKit

문서 창은 시스템 WebKit(WKWebView)으로 그린다. Itda Hyve 는 macOS 12 이상을 지원하는데, Safari 를 올리지 않은 Mac 은 옛 엔진이다.
테마가 쓰는 새 기능 둘을 엔진마다 이렇게 다룬다.

| 기능 | 필요한 엔진 | 모르는 엔진에서 |
|---|---|---|
| `@scope`(테마 CSS 를 쪽 상자에 가둔다) | Safari 17.4 이상(macOS 14.4·또는 macOS 12~13 에 Safari 17.4 이상), Chromium 118 이상 | **내장 테마**는 가두지 않은 CSS 로 낸다(우리 코드이고 `@scope` 는 우선순위를 바꾸지 않아 모양이 같다). **회사 테마는 쓰지 않는다** — 목록에 "이 macOS 의 WebKit 이 오래돼 회사 테마를 쓸 수 없음 — macOS 또는 Safari 를 올리면 된다" 로 보이고, 그 테마를 고른 문서는 경고와 함께 기본 테마로 보인다. 가두지 못하는 남의 CSS 는 받지 않는다 |
| 상대 색 `oklch(from var(--x) …)` 과 채널 `calc()` | Safari 18 이상, Chromium 119 이상 | `derived` 의 식을 쓰지 않고 `tokens` 의 정적 값을 쓴다(`@supports` 로 가른다). 내장 테마는 정적 값 = 그 강조색으로 계산한 식의 값이라 같다 |

- 판정은 화면이 한다(`CSSScopeRule` 이 있는지, CSS `@supports (color: oklch(from red calc(l + 0.1) calc(c * 0.5) h))`).
  가두지 않은 CSS 를 어느 테마에 낼지는 Go 가 테마가 내장인지 보고 정한다 — 화면은 고르지 못한다.
- **손으로 만든 회사 테마**가 `--accent` 만 바꾸면, `@scope` 는 알지만 상대 색은 모르는 엔진(Safari 17.4~17.x)에서는 진한 강조색 자리(제목·표 위 선·띠)는
  새 색이 되고 옅은 면·제목 번호(표 머리 바탕·요약문 바탕·표지의 옅은 띠·절 번호)는 바탕 테마의 색(클래식이면 옅은 남색)이 남아 두 색이 섞인다.
  섞이지 않게 하려면 바탕의 `derived` 토큰(예: classic 의 `--accent-2`·`--accent-3`·`--accent-soft`·`--lead-bg`)에 새 강조색으로 계산한 값을 `tokens` 에 함께 적는다
  (정적 값을 직접 준 토큰은 물려받은 식이 덮지 않는다). "회사 테마 만들기" 는 이것을 저절로 한다.
- 저장소에서 옛 엔진을 흉내 내 볼 수 있다(숨은 환경변수 — 줄 때만 영향): `ITDA_HYVE_DOCVIEW_OLD_WEBKIT=scope`(@scope 모름) · `relcolor`(상대 색 모름) · `all`.
  예: `ITDA_HYVE_DOCVIEW_OLD_WEBKIT=all THEMES="classic bold mono compact spine" just docview-check`.
