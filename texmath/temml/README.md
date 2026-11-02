# Temml (내장 사본)

`temml.min.js` 는 [Temml](https://temml.org/) 0.13.5 의 배포 파일 그대로다(MIT, `LICENSE`). 고치지 않는다.

- 출처: `https://registry.npmjs.org/temml/-/temml-0.13.5.tgz` 의 `package/dist/temml.min.js` (2026-08-28 배포)
- tarball sha1 `96c53c4ece59ae719859e2a5074b6d4251cee423` (npm `dist.shasum` 과 같음)
- 파일 sha256 `1ab19148afe0dbc836b7a74f344071b3f3a70a7a8259ae4049e76894fcdf8d95`

판을 올릴 때는 같은 자리의 파일을 바꾸고 위 세 줄을 고친 뒤 `go test ./internal/texmath` 의 허용 목록·지원 범위 시험을
다시 본다 — Temml 이 새 요소·속성·style 을 내면 시험이 그 수식을 거부한 것으로 알려 준다.
