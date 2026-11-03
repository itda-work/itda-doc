#!/usr/bin/env bash
# 설치 없는 웹 뷰어의 정적 번들을 만든다 — just web 이 부른다(itda-work/itda-hyve#577 2b).
#   scripts/build-web.sh
# 먼저 scripts/build-wasm.sh 로 build/web/ 의 wasm 산출물을 만들고, build/web/viewer/ 에 다음을 모은다(커밋하지 않음 — .gitignore 의 build/):
#   index.html·viewer.js·viewer.css·itda-doc.js   web/viewer/ 그대로
#   base.css·pager.js·slides.js·share.js·share.css web/ 그대로(공유 화면과 같은 파일 — 뷰어는 share.js 의 그리기를 쓴다)
#   fonts/*.woff2·fonts/OFL-Pretendard.txt·LICENSE-Lucide.txt
#   LICENSE-Go.txt(wasm_exec.js — Go 의 LICENSE, BSD)·LICENSE-Temml.txt(temml.min.js — texmath/temml/LICENSE, MIT)
#   itda-doc.wasm·.wasm.gz·.wasm.br·wasm_exec.js·temml.min.js   build/web/ 의 2a 산출물(.br 은 여기서 brotli -q 11 로)
#   samples/*.md·samples/index.json   저장소의 견본 셋(아래 SAMPLES — 사용자 데이터·비공개 이슈 링크가 없는 것)
# 같은 입력이면 같은 바이트다(gzip 은 같은 구현일 때 — build-wasm.sh 머리 주석, brotli 는 같은 판일 때). brotli 가 없으면 .br 없이 만들고 알린다.
# 띄워 보기: python3 -m http.server -d build/web/viewer 8000 → http://localhost:8000/ (그 서버는 .br·.gz 를 Content-Encoding 으로 내지 않는다 — 원본 .wasm 을 받는다)
set -euo pipefail
cd "$(dirname "$0")/.."
scripts/build-wasm.sh
OUT=build/web/viewer
# 지난 번들의 남은 파일(이름이 바뀐 견본 등)이 섞이지 않게 비우고 다시 만든다 — build/ 아래만 지운다
rm -rf "$OUT"
mkdir -p "$OUT/fonts" "$OUT/samples"
cp web/viewer/index.html web/viewer/viewer.js web/viewer/viewer.css web/viewer/itda-doc.js "$OUT/"
cp web/base.css web/pager.js web/slides.js web/share.js web/share.css web/LICENSE-Lucide.txt "$OUT/"
cp web/fonts/*.woff2 web/fonts/OFL-Pretendard.txt "$OUT/fonts/"
cp build/web/itda-doc.wasm build/web/itda-doc.wasm.gz build/web/wasm_exec.js build/web/temml.min.js "$OUT/"
GOROOT_DIR=$(go env GOROOT)
# Windows(Git Bash)는 GOROOT 가 C:\… 꼴이다 — cygpath 가 있으면 셸 경로로 바꾼다(build-wasm.sh 와 같다)
if command -v cygpath >/dev/null 2>&1; then GOROOT_DIR=$(cygpath -u "$GOROOT_DIR"); fi
# Go 의 LICENSE 는 보통 GOROOT 에 있지만 Homebrew 는 한 칸 위(Cellar/go/<판>/LICENSE)로 옮긴다 — 둘 다 없으면 멈춘다(고지 없이 싣지 않는다)
GO_LICENSE=""
for f in "$GOROOT_DIR/LICENSE" "$GOROOT_DIR/../LICENSE"; do if [ -f "$f" ]; then GO_LICENSE=$f; break; fi; done
if [ -z "$GO_LICENSE" ]; then echo "Go 의 LICENSE 를 찾지 못함($GOROOT_DIR) — wasm_exec.js 의 고지를 실을 수 없다" >&2; exit 1; fi
cp "$GO_LICENSE" "$OUT/LICENSE-Go.txt"
cp texmath/temml/LICENSE "$OUT/LICENSE-Temml.txt"
if command -v brotli >/dev/null 2>&1; then
  brotli -q 11 -c "$OUT/itda-doc.wasm" > "$OUT/itda-doc.wasm.br"
else
  echo "알림: brotli 가 없어 itda-doc.wasm.br 없이 만든다" >&2
fi
# 견본: <저장소 경로>|<번들 이름>|<고르기 글>. 고르기 글에 | 와 " 를 쓰지 않는다(아래 JSON 을 손으로 짓는다)
SAMPLES=(
  "mdoc/testdata/report.md|report.md|보고서 — 표·그림·목차"
  "mdoc/testdata/math.md|math.md|수식 — 재무 지표와 통계"
  "spec/samples/charts-guide.md|charts-guide.md|차트 — 안내서 예제"
)
{
  printf '['
  sep=""
  for s in "${SAMPLES[@]}"; do
    IFS='|' read -r from name label <<<"$s"
    cp "$from" "$OUT/samples/$name"
    printf '%s\n  {"file": "%s", "label": "%s"}' "$sep" "$name" "$label"
    sep=","
  done
  printf '\n]\n'
} > "$OUT/samples/index.json"
echo "==> $OUT ($(find "$OUT" -type f | wc -l | tr -d ' ') 파일)"
for f in itda-doc.wasm itda-doc.wasm.gz itda-doc.wasm.br; do
  if [ -f "$OUT/$f" ]; then echo "    $f $(wc -c < "$OUT/$f" | tr -d ' ') bytes"; fi
done
