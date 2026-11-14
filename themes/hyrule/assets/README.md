# 글꼴

- `noto-serif-kr-bold.woff2` — `themes/editorial/assets/noto-serif-kr-bold.woff2` 와 같은 바이트다. 출처·판·subset 범위·만든 명령은
  [editorial 의 README](../../editorial/assets/README.md) 를 본다. SIL Open Font License 1.1, 원문은 `OFL-NotoSerifKR.md`.
- 테마 자산은 그 테마 이름 아래에서만 풀리므로(`page/themecss.go` — `url(assets/…)` 는 자기 테마 폴더의 파일) editorial 의 파일을 가리키지 못해 복사해 둔다.
- hyrule 테마의 제목(`--font-heading`)에만 쓴다. `GOOS=js` 빌드에는 넣지 않는다(`themes/themes_js.go`).
