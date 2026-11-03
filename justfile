# itda-doc 개발 명령. `just` 또는 `just --list` 로 목록 확인.

set shell := ["bash", "-euo", "pipefail", "-c"]

# 기본: 목록 표시
default:
    @just --list --unsorted

# 정적 검사: gofmt + go vet
check:
    @fmt=$(gofmt -l .); if [ -n "$fmt" ]; then echo "gofmt 필요:"; echo "$fmt"; exit 1; fi
    go vet ./...

# 단위 테스트 전체. ITDA_DOC_STRICT=1 — node(web 의 쪽 나눔 시험)·CommonMark 예제(spec.json)가 없으면 건너뛰지 않고 실패한다
test: node-check
    ITDA_DOC_STRICT=1 go test ./...

# test 의 선행 검사
node-check:
    @command -v node >/dev/null || { echo "node 필요: web 의 pager.js 시험"; exit 1; }

# 문법 단일본 다시 만들기: 명세(spec/*.md)의 syntax 구간 → mdsyntax/topics·completions.json.
# 명세를 고치면 돌린다 — 잊으면 TestBundleFresh 가 실패한다
syntax:
    go generate ./mdsyntax

# 브라우저용 렌더 입구: build/web/ 에 itda-doc.wasm·.wasm.gz·wasm_exec.js·temml.min.js (scripts/build-wasm.sh)
wasm:
    scripts/build-wasm.sh
