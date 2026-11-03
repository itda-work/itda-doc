#!/usr/bin/env bash
# 브라우저용 렌더 입구(cmd/itda-doc-wasm)를 만든다 — just wasm 이 부른다.
#   scripts/build-wasm.sh
# 결과(커밋하지 않음 — .gitignore 의 build/): build/web/ 아래
#   itda-doc.wasm     GOOS=js GOARCH=wasm, -trimpath -ldflags "-s -w -X main.commit=<커밋>"
#   itda-doc.wasm.gz  gzip -9 -n — 같은 소스·같은 Go 판·같은 커밋에 같은 gzip 구현이면 같은 바이트
#                     (.wasm 은 gzip 과 상관없이 같다. macOS gzip 과 GNU gzip 의 결과가 같은지는 비교하지 않았다)
#   wasm_exec.js      이 Go 판의 런타임 받침($(go env GOROOT)/lib/wasm/wasm_exec.js, BSD — Go 저장소 LICENSE)
#   temml.min.js      texmath/temml/temml.min.js 그대로(정본은 그 한 벌 — 판이 어긋나지 않게 복사만 한다, MIT)
# 페이지는 wasm_exec.js → temml.min.js → itda-doc.wasm 순으로 올린다. temml 이 없으면 수식이 원문 + 경고로 남는다(texmath/temml_js.go).
set -euo pipefail
cd "$(dirname "$0")/.."
OUT=build/web
mkdir -p "$OUT"
# 중간에 실패해도 반쯤 쓴 임시 파일이 남지 않게 끝날 때 늘 지운다
trap 'rm -f "$OUT"/*.tmp' EXIT
# 커밋: 작업 트리가 깨끗하지 않으면 -dirty 를 붙인다(서로 다른 dirty 트리도 같은 이름이 된다). git 이 없으면 비운다(itdaDoc.version.commit 이 "")
COMMIT=""
if git rev-parse --git-dir >/dev/null 2>&1; then
  COMMIT=$(git rev-parse --short=12 HEAD)
  if [ -n "$(git status --porcelain)" ]; then COMMIT="$COMMIT-dirty"; fi
fi
# 임시 이름에 다 쓴 뒤 옮긴다 — 중간에 끊기거나 디스크가 차도 잘린 파일이 남지 않게
GOOS=js GOARCH=wasm go build -trimpath -buildvcs=false -ldflags "-s -w -X main.commit=$COMMIT" -o "$OUT/itda-doc.wasm.tmp" ./cmd/itda-doc-wasm
gzip -9 -n -c "$OUT/itda-doc.wasm.tmp" > "$OUT/itda-doc.wasm.gz.tmp"
GOROOT_DIR=$(go env GOROOT)
# Windows(Git Bash)는 GOROOT 가 C:\… 꼴이다 — cygpath 가 있으면 셸 경로로 바꾼다
if command -v cygpath >/dev/null 2>&1; then GOROOT_DIR=$(cygpath -u "$GOROOT_DIR"); fi
cp "$GOROOT_DIR/lib/wasm/wasm_exec.js" "$OUT/wasm_exec.js.tmp"
cp texmath/temml/temml.min.js "$OUT/temml.min.js.tmp"
# 옮기기 도중 실패하면(set -e 로 멈춘다) 새 파일과 옛 파일이 섞여 남을 수 있다 — 다시 돌리면 맞춰진다
for f in itda-doc.wasm itda-doc.wasm.gz wasm_exec.js temml.min.js; do mv -f "$OUT/$f.tmp" "$OUT/$f"; done
echo "==> $OUT/itda-doc.wasm $(wc -c < "$OUT/itda-doc.wasm" | tr -d ' ') bytes, .gz $(wc -c < "$OUT/itda-doc.wasm.gz" | tr -d ' ') bytes ($(go env GOVERSION), commit ${COMMIT:-없음})"
