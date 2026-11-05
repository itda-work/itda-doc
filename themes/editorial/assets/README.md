# 글꼴

- `noto-serif-kr-bold.woff2` — Noto Serif KR Bold, 판 2.003 (https://github.com/notofonts/noto-cjk 의 `Serif/SubsetOTF/KR/NotoSerifKR-Bold.otf`).
  Copyright © 2017-2024 Adobe. SIL Open Font License 1.1. 원문은 `OFL-NotoSerifKR.md`(같은 저장소의 `Serif/LICENSE` 를 그대로 옮겼다).
  이 라이선스 원문에는 예약 글꼴 이름(Reserved Font Name)이 적혀 있지 않다. 「Noto」 는 Google 의 상표다(글꼴 이름표 7번).
- editorial 테마의 제목·큰 인용에만 쓴다(`--font-heading`). 본문은 바탕의 Pretendard 그대로다.
  테마 자산을 제공하는 호스트에서 사용하도록 묶었다(OFL 은 문서 임베드를 허용한다). 앱 화면 로딩·PDF 임베드는 미검증이다. 글꼴을 따로 팔지 않는다.
- 원본에서 아래 글자만 남겨 woff2 로 바꿨다(fontTools 4.68.0 `pyftsubset`, 같은 입력이면 같은 바이트):

  ```
  pyftsubset NotoSerifKR-Bold.otf --flavor=woff2 --layout-features='*' --name-IDs='*' --name-legacy --name-languages='*' --notdef-outline \
    --unicodes=U+0020-007E,U+00A0-00FF,U+2010-2027,U+2030-203A,U+2103,U+2122,U+2190-2193,U+2460-2473,U+3000-3003,U+3008-3011,U+3131-318E,U+AC00-D7A3
  ```

  한글 음절 11172자 전부, 호환 자모, 라틴 기본·보충, 문장 부호·화살표·①~⑳·꺾쇠 괄호. 한자와 그 밖의 글자는 넣지 않았다 —
  `--font-heading` 의 다음 글꼴을 순서대로 시도하며, 내장 Pretendard 에도 없는 한자는 시스템 serif·시스템 대체 글꼴에 의존한다.
  플랫폼별 모습은 미검증이다.
- 원본 sha256 `263408609ef416d597718ea91055379867d3e7086fa32f049996995d4e715e55`(7,910,236 바이트) →
  woff2 sha256 `e5a820424fb4aa3f2bd57e73655ef97182a03f1c8e1a4c85b07050b51b06f1bb`(1,851,468 바이트).
