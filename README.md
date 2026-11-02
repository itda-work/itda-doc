# Itda Doc

마크다운으로 문서를 쓰는 방법의 정의다. 새 문법이 아니라 **프로파일**(CommonMark → GFM → Pandoc 부분집합)과 그 위의 **어휘**(컨테이너·frontmatter 키·차트·그림·수식·슬라이드), 같게 그리는 **참조 렌더러**(Go · WebAssembly), 모양을 정하는 **테마 계약**으로 이루어진다.

- 파일은 `.md` 그대로 쓴다. 이 프로파일을 따름을 밝히려면 `.itda.md`.
- 새 기호를 만들지 않는다. GitHub · VS Code · Obsidian 에서 그대로 읽힌다.
- 문서에는 모양을 적지 않는다. 모양은 테마가 정한다.
- 원격 그림은 기본 차단, raw HTML 은 그리지 않는다.

[Itda Hyve Docs](https://itda.work/hyve) 는 Itda Doc 문서를 쓰고 그리는 창이다. 이 저장소는 그 렌더러와 명세를 꺼내 공개하는 자리다.

## 상태

준비 중. 명세·렌더러·테마·웹 뷰어를 차례로 옮긴다.

## 라이선스

코드 · 테마 · 웹 자원은 [MIT](LICENSE), 명세(`spec/`)는 CC BY 4.0.
