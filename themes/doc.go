// Package themes 는 내장 테마다 — 구조 테마 다섯(classic·bold·compact·mono·spine)과 그 위에 extends 로 얹은 중립 스타일 테마(slate·swiss·contrast·verdant·editorial), 어두운 슬라이드 테마 hyrule(extends 없음). 폴더마다 theme.json·theme.css(새 문서 서식 template.md 는 없다 — 앱의 서식 갤러리가 맡는다).
//
// 공개 묶음 FS 는 빌드마다 타입과 내용이 다르다.
//
//   - 네이티브(themes.go): embed.FS. 테마 폴더 전체를 묶어 editorial·hyrule 글꼴이 들어 있다.
//   - GOOS=js(themes_js.go): testing/fstest.MapFS. 폴더마다 theme.json·theme.css 만 묶어 글꼴 바이트가 없고, editorial·hyrule 글꼴 자리가 0 바이트로 있다. 그래서 두 테마의 Theme.Hash 가 네이티브와 다르다.
//
// 두 판에 공통인 호출은 FS.ReadFile·FS.ReadDir·FS.Open 과 fs.FS 를 받는 함수다(TestFSPublicMethods 가 두 판의 컴파일을 본다).
// embed.FS 타입 자체를 요구하는 코드(var x embed.FS = themes.FS 등)는 GOOS=js 에서 컴파일되지 않는다.
package themes
