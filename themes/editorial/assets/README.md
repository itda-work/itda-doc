# 글꼴

- `noto-serif-kr-bold.woff2` — Noto Serif KR Bold, 판 2.003-H1. TrueType 윤곽이다.
  원본은 https://github.com/google/fonts 커밋 `8b0a1d0f5983c89bc2b93f1b5fb55f9e252744b5` 의 `ofl/notoserifkr/NotoSerifKR[wght].ttf`(가변, wght 200~900) —
  그쪽 `METADATA.pb` 는 이 파일이 notofonts/noto-cjk 커밋 `985fa52c81c1d6692ccdd82bc3656e8fb932fd89` 의 `google-fonts/NotoSerifKR[wght].ttf` 라고 적는다.
  글꼴 이름표 0번 `(c) 2017-2024 Adobe (http://www.adobe.com/).`. SIL Open Font License 1.1. 원문은 `OFL-NotoSerifKR.md`(위 커밋의 `ofl/notoserifkr/OFL.txt` 를 그대로 옮겼다).
  이 라이선스 원문에는 예약 글꼴 이름(Reserved Font Name)이 적혀 있지 않다. 「Noto」 는 Google 의 상표다(글꼴 이름표 7번).
- CFF(OTF 기반) 판을 쓰지 않는다 — macOS PDF 에서 글자층(ToUnicode)이 비어 제목의 검색·복사가 깨졌다(itda-hyve#666 실측, itda-doc#50). 내장 테마의 woff2 가 TrueType 인지는 `TestBuiltinWoff2TrueType`(`themes/themes_test.go`)이 막는다.
- editorial 테마의 제목·큰 인용에만 쓴다(`--font-heading`). 본문은 바탕의 Pretendard 그대로다.
  테마 자산을 제공하는 호스트에서 사용하도록 묶었다(OFL 은 문서 임베드를 허용한다). 앱 화면 로딩·PDF 임베드는 미검증이다. 글꼴을 따로 팔지 않는다.
- 원본에서 굵기를 700 으로 고정하고 아래 글자만 남겨 woff2 로 바꿨다(fontTools 4.68.0·brotli 1.2.1, 같은 입력이면 같은 바이트):

  ```
  fonttools varLib.instancer 'NotoSerifKR[wght].ttf' wght=700 --update-name-table --no-recalc-timestamp -o NotoSerifKR-Bold.ttf
  pyftsubset NotoSerifKR-Bold.ttf --output-file=noto-serif-kr-bold.woff2 --flavor=woff2 \
    --layout-features=aalt,dlig,fwid,hwid,pwid,vert,vrt2,vjmo,chws,halt,kern,palt,vchw,vhal,vpal --name-IDs='*' --name-legacy --name-languages='*' --notdef-outline \
    --unicodes=U+0020-007E,U+00A0-00FF,U+2010-2027,U+2030-203A,U+2103,U+2122,U+2190-2193,U+2460-2473,U+3000-3003,U+3008-3011,U+3131-318E,U+AC00-D7A3
  ```

  한글 음절 11172자 전부, 호환 자모, 라틴 기본·보충, 문장 부호·화살표·①~⑳·꺾쇠 괄호. 한자와 그 밖의 글자는 넣지 않았다 —
  `--font-heading` 의 다음 글꼴을 순서대로 시도하며, 내장 Pretendard 에도 없는 한자는 시스템 serif·시스템 대체 글꼴에 의존한다.
  플랫폼별 모습은 미검증이다.
- `--layout-features` 에서 `calt`·`locl`·`liga`·`ccmp` 만 뺐다. 이 넷은 기본으로 켜지면서 글자를 cmap 에 없는 글리프로 바꾼다 —
  한글 사이 공백(`calt`), `lang="ko"` 의 숫자·문장 부호·공백(`locl`), `fi`·`ff`(`liga`), `——`(`ccmp`). macOS PDF 경로의 ToUnicode 에는 그런 글리프가 빠져
  뷰어가 코드 값을 글자로 읽는다(한글 사이 공백이 `"` 로 나왔다 — 저장소 밖 WKWebView `createPDF` 실측, 시험으로 막지는 않았다).
  화면에서는 옛 CFF 판보다 숫자·문장 부호 폭이 조금 달라지고 줄 높이는 같다.
- 원본 sha256 `11f8d5de6f1b79195efba3828aaa2ec95c1178f5ae976fb23c8d53250a9938f3`(23,795,420 바이트) →
  700 고정 TTF sha256 `23967d12dbcf05ab454f2e552da013d1e2680f6f61ddc537acf9ac639221c7e2`(14,126,400 바이트) →
  woff2 sha256 `26f094290c4aed145be7c4718f75a6c7da67525f694e31cde2f57968a4fcbd61`(1,057,184 바이트).
- 대시·공백류 일부는 원래 코드 포인트로 왕복하지 않는다 — cmap 에서 U+0020·U+00A0, U+002D·U+00AD·U+2011, U+2012·U+2013 이 각각 글리프를 공유해 저장소 밖 실측에서 U+2013 이 U+2012 로, U+2011 이 `-` 로 추출됐다. itda-hyve 앱 경로는 미검증이다.
