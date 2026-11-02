# 제3자 구성 요소

이 저장소가 쓰거나 함께 배포하는 남의 코드·자원과 그 사용권이다. Go 모듈은 `go.mod` 의 판이고, 사용권은 모듈 캐시
(`go mod download -json <모듈>@<판>` 의 `Dir`)의 사용권 파일을 열어 확인했다(2026-11-02).

## Go 모듈 — 직접 의존(`go.mod` 첫 require)

| 모듈 | 판 | 사용권 | 쓰는 곳 |
|---|---|---|---|
| `github.com/alecthomas/chroma/v2` | v2.23.1 | MIT(`COPYING`) | 코드 블록 구문 강조(mdoc) |
| `github.com/dop251/goja` | v0.0.0-20260826204918-8f1c0696a37b | MIT(`LICENSE`) | Temml 을 돌리는 JS 엔진(texmath) |
| `github.com/mattn/go-runewidth` | v0.0.19 | MIT(`LICENSE`) | 글자 너비(mdoc) |
| `github.com/yuin/goldmark` | v1.8.6 | MIT(`LICENSE`) | CommonMark·GFM 파서(mdoc) |
| `golang.org/x/net` | v0.62.0 | BSD-3-Clause(`LICENSE`) | 시험 전용(HTML 파싱 대조 — `mdoc/{textprop,nesting,security}_test.go`), 배포물에 없음 |
| `gopkg.in/yaml.v3` | v3.0.1 | MIT 와 Apache-2.0(`LICENSE` — 파일마다 둘 중 하나) | frontmatter·`_metadata.yml`(mdoc) |

## Go 모듈 — 간접 의존(빌드에 들어가는 것)

| 모듈 | 판 | 사용권 |
|---|---|---|
| `github.com/clipperhouse/uax29/v2` | v2.6.0 | MIT |
| `github.com/dlclark/regexp2` | v1.11.5 | MIT |
| `github.com/dlclark/regexp2/v2` | v2.5.2 | MIT |
| `github.com/go-sourcemap/sourcemap` | v2.1.3+incompatible | BSD-2-Clause |
| `github.com/google/pprof` | v0.0.0-20230207041349-798e818bf904 | Apache-2.0 |
| `golang.org/x/text` | v0.43.0 | BSD-3-Clause |

시험에서만 쓰는 모듈(`go list -m all` 의 나머지 — goldmark 의 `testutil` 이 끌어오는 것 등)은 배포물에 들어가지 않는다.

## 함께 넣은 자원

| 자원 | 자리 | 사용권 |
|---|---|---|
| Temml 0.13.5(`temml.min.js`) | `texmath/temml/` | MIT — `texmath/temml/LICENSE` |
| Pretendard(regular·semibold·bold woff2) | `web/fonts/` | SIL Open Font License 1.1 — `web/fonts/OFL-Pretendard.txt` |
| Lucide 아이콘 모양(공유 화면 `share.html` 의 심볼) | `web/share.html` | ISC(일부 MIT — Feather) — `web/LICENSE-Lucide.txt` |
