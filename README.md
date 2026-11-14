# Itda Doc

마크다운으로 문서를 쓰는 방법의 정의다. 새 문법이 아니라 **프로파일**(CommonMark → GFM → Pandoc 부분집합)과 그 위의 **어휘**(컨테이너·frontmatter 키·차트·그림·수식·슬라이드), 같게 그리는 **참조 렌더러**(Go · WebAssembly), 모양을 정하는 **테마 계약**으로 이루어진다.

- 파일은 `.md` 그대로 쓴다. 이 프로파일을 따름을 밝히려면 `.itda.md`.
- 새 기호를 만들지 않는다. GitHub · VS Code · Obsidian 에서 그대로 읽힌다.
- 문서에는 모양을 적지 않는다. 모양은 테마가 정한다.
- 원격 그림은 기본 차단, raw HTML 은 그리지 않는다.

[Itda Hyve Docs](https://itda.work/hyve) 는 Itda Doc 문서를 쓰고 그리는 창이다. 이 저장소는 그 렌더러와 명세를 꺼내 공개하는 자리다.

## 상태

1단계: 렌더러·테마·쪽 모델 자원·명세를 Itda Hyve 에서 옮겨 왔고(a), Itda Hyve 가 이 모듈을 쓴다(b). 남은 Itda Hyve 이름·이슈 번호·옛 경로를 정리했다(c). 다음은 명세의 새 절과 웹 뷰어(2단계).

## 라이선스

코드 · 테마 · 웹 자원은 [MIT](LICENSE), 명세(`spec/`)는 [CC BY 4.0](spec/LICENSE).

## 구조

| 자리 | 내용 |
|---|---|
| [`spec/`](spec/) | 명세(CC BY 4.0) — 입구 [markdown.md](spec/markdown.md), 차트 [charts.md](spec/charts.md), 수식 [math.md](spec/math.md), 그림 [diagrams.md](spec/diagrams.md), 발표 [slides.md](spec/slides.md), 테마 [themes.md](spec/themes.md), 그림 설계 [diagrams-design.md](spec/diagrams-design.md) |
| `mdoc/` | 참조 렌더러 — `mdoc.Render(src, Options)` 가 원문을 HTML 조각·목차·경고로 바꾼다. `testdata/` 에 골든 5벌 |
| `texmath/` | 수식(TeX → MathML, Temml) |
| `diagram/` | mermaid 부분집합 그림 — 공통 `diagram` 과 `flowchart`·`sequence`·`classdiag`·`er`·`state`·`gantt`·`timeline`·`layered` |
| `mdsyntax/` | 명세의 syntax 구간에서 모은 문법 단일본(`topics/`)과 편집기 자동 완성 후보(`completions.json`) |
| `page/` | 쪽 모델 조립 — 테마 고르기·표지·차례·머리글(`RenderView`), 테마 묶음(`LoadThemes`·`LoadThemeSet`), 검사(`Check`), 폴더 기본값(`_metadata.yml`) |
| `themes/` | 내장 테마 — 구조 테마 다섯(`classic`·`bold`·`compact`·`mono`·`spine`)과 중립 스타일 테마(`slate`·`swiss`·`contrast`·`verdant`·`editorial`), 어두운 슬라이드 테마 `hyrule` — 와 견본 회사 테마 `samples/acme`. `GOOS=js` 판 묶음(`themes_js.go`)에는 글꼴 자산을 넣지 않는다 |
| `web/` | 쪽 모델 웹 자원 — `base.css`·`pager.js`·`slides.js`·보기 전용 공유 화면(`share.*`)·글꼴. `share.js` 의 그리기는 View 원천(공유 서버 `hostSource` · 뷰어의 `window.ItdaViewSource`)과 나뉜다 |
| `web/viewer/` | 설치 없는 웹 뷰어 — 파일 열기·폴더 열기·붙여넣기·견본을 wasm 으로 그려 `share.js` 와 같은 화면에 그린다(문서를 네트워크로 보내지 않음). 폴더로 열면(「폴더」 단추·폴더 끌어놓기) 문서 폴더부터 연 폴더까지의 `_metadata.yml` 을 네이티브와 같은 규칙으로 얹고(render 옵션 `folderMeta`) 상대 경로 그림을 blob: URL 로 그린다 — 연 폴더보다 위의 폴더는 보지 못하고, 파일 하나·붙여넣기로 열면 둘 다 적용하지 않는다. `editorial` 의 제목 글꼴은 wasm 에 없어 제목·큰 인용이 세리프가 아니라 대체 글꼴(Pretendard)로 나온다. 파일 변경을 감시하지 않아 고친 뒤에는 다시 연다(열어 둔 폴더의 문서나 그림을 읽지 못하면 다시 열라는 안내가 나온다). `itda-doc.js` 는 `itdaDoc` 을 감싸는 로더(옵션 오류는 throw). `just web` 이 `build/web/viewer/` 에 정적 번들을 만든다. 탭 제목은 늘 「itda-doc 뷰어」라 브라우저 인쇄의 「PDF 로 저장」 기본 파일 이름도 문서 제목이 아니라 「itda-doc 뷰어」다 |
| `cmd/itda-doc/` | 명령줄 도구 |
| `cmd/itda-doc-wasm/` | 브라우저용 렌더 입구(WebAssembly) — 전역 `itdaDoc.render`·`themes`·`themeCSS`. 수식은 페이지가 올린 `temml.min.js` 로 그린다. `just wasm` 이 `build/web/` 에 만든다 |

## 빌드·시험

Go 1.27 과 node 가 필요하다(node 는 `web/` 의 쪽 나눔 시험). `just test` 와 CI 는 `ITDA_DOC_STRICT=1` 로 돈다 — node 나 CommonMark 예제(`spec.json`)가 없으면 건너뛰지 않고 실패한다. 값 없이 `go test ./...` 하면 그 시험만 건너뛴다.

```sh
just check     # gofmt · go vet
just test      # ITDA_DOC_STRICT=1 go test ./...
just syntax    # spec/ 을 고친 뒤 mdsyntax/topics·completions.json 다시 만들기
GOOS=js GOARCH=wasm go build ./mdoc/ ./page/ ./texmath/   # WebAssembly 컴파일 확인
just wasm      # build/web/ 에 itda-doc.wasm·.gz·wasm_exec.js·temml.min.js
just web       # build/web/viewer/ 에 웹 뷰어 정적 번들(.wasm.br 은 brotli 가 있을 때)
python3 -m http.server -d build/web/viewer 8000   # http://localhost:8000/ 로 뷰어 열기
```

## CLI

```sh
go install github.com/itda-work/itda-doc/cmd/itda-doc@latest

itda-doc render 보고서.md                    # HTML 조각(mdoc.Render)
itda-doc render -page -theme mono 보고서.md  # 표지·차례·base.css·테마 CSS 가 든 단일 HTML(JS 없음)
itda-doc check 보고서.md 발표.md             # "파일:줄: 코드 메시지", 경고가 있으면 종료 코드 1
itda-doc syntax charts                       # 문법 안내 한 주제(인자 없으면 주제 목록)
itda-doc themes -themes ~/회사테마            # 내장·사용자 테마 목록
```

## English

Itda Doc is a way of writing documents in Markdown. It is not a new syntax but a **profile** (CommonMark → GFM → a subset of Pandoc) plus a **vocabulary** on top of it (containers, frontmatter keys, charts, diagrams, math, slides), a **reference renderer** (Go, compiles to WebAssembly) that draws it the same way everywhere, and a **theme contract** that decides how it looks. Files stay plain `.md`; `.itda.md` states that a file follows the profile (a naming convention — renderers treat it as `.md`, and no code here gives it special handling). No new symbols are invented, so documents read fine on GitHub, VS Code and Obsidian. Documents carry meaning, themes carry looks. Remote images are blocked by default and raw HTML is not rendered.

- **License**: code, themes and web assets are MIT; the specification in `spec/` is CC BY 4.0.
- **Packages**: `mdoc` (Markdown → HTML fragment, TOC, warnings), `texmath` (TeX → MathML), `diagram/…` (a Mermaid subset: flowchart, gantt, timeline, sequence, ER, state, class), `mdsyntax` (syntax digest generated from `spec/`), `page` (page model: themes, cover, TOC, header/footer, `Check`), `themes` (five neutral built-in themes), `web` (page-model CSS/JS and the read-only share view), `cmd/itda-doc` (CLI: `render`, `check`, `syntax`, `themes`).
- **Relation to Itda Hyve**: [Itda Hyve Docs](https://itda.work/hyve) is a desktop app that uses this module as its document renderer and adds its own brand themes and app features on top.

The specification is written in Korean; an English translation will follow the 1.0 release of the specification.
