# Itda Doc

마크다운으로 문서를 쓰는 방법의 정의다. 새 문법이 아니라 **프로파일**(CommonMark → GFM → Pandoc 부분집합)과 그 위의 **어휘**(컨테이너·frontmatter 키·차트·그림·수식·슬라이드), 같게 그리는 **참조 렌더러**(Go · WebAssembly), 모양을 정하는 **테마 계약**으로 이루어진다.

- 파일은 `.md` 그대로 쓴다. 이 프로파일을 따름을 밝히려면 `.itda.md`.
- 새 기호를 만들지 않는다. GitHub · VS Code · Obsidian 에서 그대로 읽힌다.
- 문서에는 모양을 적지 않는다. 모양은 테마가 정한다.
- 원격 그림은 기본 차단, raw HTML 은 그리지 않는다.

[Itda Hyve Docs](https://itda.work/hyve) 는 Itda Doc 문서를 쓰고 그리는 창이다. 이 저장소는 그 렌더러와 명세를 꺼내 공개하는 자리다.

## 상태

1단계 a: 렌더러·테마·쪽 모델 자원·명세를 Itda Hyve(69812397)에서 옮겨 왔다. 다음은 Itda Hyve 가 이 모듈을 쓰도록 전환(1b), 명세의 새 절(1c), 웹 뷰어(2단계).

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
| `themes/` | 내장 중립 테마 다섯(`classic`·`bold`·`compact`·`mono`·`spine`)과 견본 회사 테마 `samples/acme` |
| `web/` | 쪽 모델 웹 자원 — `base.css`·`pager.js`·`slides.js`·보기 전용 공유 화면(`share.*`)·글꼴 |
| `cmd/itda-doc/` | 명령줄 도구 |

## 빌드·시험

Go 1.27 과 node 가 필요하다(node 는 `web/` 의 쪽 나눔 시험). `just test` 와 CI 는 `ITDA_DOC_STRICT=1` 로 돈다 — node 나 CommonMark 예제(`spec.json`)가 없으면 건너뛰지 않고 실패한다. 값 없이 `go test ./...` 하면 그 시험만 건너뛴다.

```sh
just check     # gofmt · go vet
just test      # ITDA_DOC_STRICT=1 go test ./...
just syntax    # spec/ 을 고친 뒤 mdsyntax/topics·completions.json 다시 만들기
GOOS=js GOARCH=wasm go build ./mdoc/ ./page/ ./texmath/   # WebAssembly 컴파일 확인
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

Itda Doc is a way of writing documents in Markdown. It is not a new syntax but a **profile** (CommonMark → GFM → a subset of Pandoc) plus a **vocabulary** on top of it (containers, frontmatter keys, charts, diagrams, math, slides), a **reference renderer** (Go, compiles to WebAssembly) that draws it the same way everywhere, and a **theme contract** that decides how it looks. Files stay plain `.md` (`.itda.md` to state that a file follows the profile (a naming convention; renderers treat it as `.md`)); no new symbols are invented, so documents read fine on GitHub, VS Code and Obsidian. Documents carry meaning, themes carry looks. Remote images are blocked by default and raw HTML is not rendered. Code, themes and web assets are MIT-licensed; the specification in `spec/` is CC BY 4.0. The specification is written in Korean; an English translation will follow the 1.0 release of the specification.
