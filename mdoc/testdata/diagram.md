---
title: 그림 견본 — 업무 흐름도
subtitle: mermaid 흐름도 (설계 1.10)
date: 2026-10-13
lang: ko
theme: classic
organization: 잇다워크 주식회사
doc-no: 그림-2026-01
---

# 결재 흐름

구매 요청은 팀장 검토를 거쳐 금액에 따라 결재선이 갈린다. 반려되면 요청자가 보완해 다시 올린다. DIA_A01

```mermaid
---
title: 구매 결재 흐름 DIA_CAP1
---
flowchart TD
  req([구매 요청 접수]) --> lead{팀장 검토}
  lead -->|반려| fix[요청자 보완]
  fix --> req
  lead -->|승인| amt{금액 500만 원 이상?}
  amt -->|예| cfo[재무 이사 결재]
  amt -->|아니오| buy[구매 진행]
  cfo --> buy
  buy --> done((완료))
```

DIA_A02 위의 그림은 순환(반려 → 보완 → 접수)과 합류(두 결재선이 구매 진행으로)를 담는다.

## 시스템 구성

```mermaid
---
title: 서비스 구성도 DIA_CAP2
---
flowchart LR
  user[사용자 웹 화면] --> api[인증된 웹 API]
  ai[ChatGPT · Claude] -->|HTTPS MCP| api
  subgraph 서버 구역
    api --> auth[권한 검사]
    auth --> job[(작업 대기열)]
    job --> worker[[작업자]]
  end
  worker -.->|결과 알림| user
  worker ==> store[(산출물 저장소)]
```

DIA_B01 묶음은 점선 없이 옅은 상자로, 상자 밖으로 나가는 화살표는 상자 테두리를 지난다.

## 분기 일정

```mermaid
flowchart TB
  subgraph 1분기
    a1[시장 조사] --> a2[제품 기획]
  end
  subgraph 2분기
    b1[설계] --> b2[개발]
    b2 --> b2
  end
  subgraph 3분기
    c1[시범 운영] --> c2[정식 출시]
  end
  a2 --> b1
  b2 -->|일정 지연 시| a2
  b2 --> c1
  c2 -. 고객 의견 .-> a1
```

DIA_C01 자기 간선(개발 → 개발)은 노드 옆의 고리다.

# 모양과 화살표

| 표기 | 모양 | 표기 | 화살표 |
|---|---|---|---|
| `A[글]` | 사각형 | `-->` | 화살표 |
| `A(글)` | 둥근 사각형 | `---` | 선 |
| `A{글}` | 마름모(판단) | `-.->` | 점선 화살표 |
| `A[(글)]` | 원통(DB) | `==>` | 굵은 화살표 |

Table: 자주 쓰는 표기 DIA_T01

```mermaid
flowchart LR
  r1[사각형] --> r2(둥근) --> r3([경기장]) --> r4[[서브루틴]]
  r4 --> r5[(원통)] --> r6((원)) --> r7{마름모}
  r7 --> r8{{육각형}} --> r9[/평행사변형/] --> r10[/사다리꼴\]
  r1 <--> r6
```

DIA_D01 위는 지원하는 모양 열 가지다.

## 아래에서 위로·오른쪽에서 왼쪽으로

```mermaid
graph BT
  base[기초 체력] --> mid[전문 역량]
  mid --> top[리더십]
  base --> top
```

```mermaid
graph RL
  in[입고] --> qc[검수] --> out[출고]
  qc -->|불량| ret[반품]
```

DIA_E01 방향 네 가지(TD·LR·BT·RL)를 모두 그린다.

# 그리지 않는 것

다른 그림 종류는 원문 코드 그대로 둔다(경고 하나). DIA_F01

```mermaid
pie
  "고객 DIA_SEQ" : 1
```

모양을 적는 줄은 쓰지 않고 그림은 그린다(경고). DIA_F02

```mermaid
%%{init: {"theme": "dark"}}%%
flowchart LR
  s1[색 지정 DIA_STYLE] --> s2[링크 지정]
  style s1 fill:#f00
  click s2 "https://example.com"
```

# 긴 흐름

쪽 끝에 걸치는 긴 그림은 나뉘지 않고 다음 쪽으로 넘어간다. DIA_G01

```mermaid
flowchart TD
  g1[1단계 요구사항 수집] --> g2[2단계 범위 확정]
  g2 --> g3[3단계 설계 문서 작성]
  g3 --> g4{4단계 설계 검토}
  g4 -->|보완| g3
  g4 -->|승인| g5[5단계 구현 계획]
  g5 --> g6[6단계 백엔드 개발]
  g5 --> g7[7단계 프론트엔드 개발]
  g6 --> g8[8단계 통합]
  g7 --> g8
  g8 --> g9[9단계 단위 시험]
  g9 --> g10[10단계 통합 시험]
  g10 --> g11[11단계 사용자 시험]
  g11 --> g12{12단계 출시 판정}
  g12 -->|결함| g6
  g12 -->|통과| g13([13단계 출시 DIA_END])
```

DIA_H01 문서 끝.

# 간트 차트

mermaid 간트 블록도 그림이 된다(#409 ①). 정상 견본 DIA_I01

```mermaid
---
title: 출시 추진 일정 DIA_CAP3
---
gantt
  title 2026년 상반기 추진
  dateFormat YYYY-MM-DD
  excludes weekends
  section 준비
  요구사항 조사 :done, 2026-01-05, 4d
  계획 수립 :p1, 2026-01-12, 5d
  section 개발
  설계 :active, after p1, 2w
  구현 :crit, 2026-02-02, 3w
  출시 확정 :milestone, 2026-03-02, 0d
```

DIA_I02 모양을 적는 줄(todayMarker·topAxis)은 쓰지 않고 그림은 그린다(경고).

```mermaid
gantt
  dateFormat YYYY-MM-DD
  todayMarker stroke:red
  topAxis
  작업 하나 :2026-04-01, 10d
```

DIA_I03 vert 태그는 이 판이 그리지 않는다 — 원문 코드 블록 + 경고.

```mermaid
gantt
  dateFormat YYYY-MM-DD
  마감 표식 :vert, 2026-05-01, 1d
```

DIA_I04 월 단위 로드맵도 그린다.

```mermaid
gantt
  dateFormat YYYY-MM
  axisFormat %Y.%m
  section 하반기
  파일럿 :2026-07, 2M
  확산 :2026-09, 4M
```

DIA_I05 문서 끝.

# 타임라인

mermaid 타임라인 블록도 그림이 된다(#409 ②). DIA_J01

```mermaid
---
title: 회사 연혁 DIA_CAP4
---
timeline
  title 창업 10년
  section 창업기
  2016 : 잇다워크 설립
  2018 : 시리즈 A 유치
  section 성장기
  2020 : 매출 100억 돌파
  2022 : 해외 법인 설립
```

DIA_J02 구역(창업기·성장기)과 한 줄에 사건 하나, 캡션을 담는다.

## 사건 여럿과 이어 쓰기

```mermaid
timeline
  로드맵
  1분기 : 파일럿 : 내부 검증
  : 고객 시범 적용
  2분기 : 정식 출시
  3분기
```

DIA_J03 한 줄에 사건 여럿(콜론 구분), 다음 줄 이어 쓰기, 사건 없는 시기(3분기)를 담는다.

# 순서도

mermaid 순서도(sequenceDiagram) 블록도 그림이 된다(#409 ③). DIA_K01

```mermaid
---
title: 주문 결제 흐름 DIA_CAP5
---
sequenceDiagram
    title 주문 흐름
    actor U as 고객
    participant S as 상점
    participant P as 결제사
    autonumber
    U->>+S: 주문 요청
    S->>+P: 결제 승인
    P-->>-S: 승인 완료
    S-->>-U: 주문 확인
    Note over U,P: 주문 접수
    alt 재고 있음
        S->>U: 배송 시작
    else 품절
        S--xU: 품절 알림
    end
```

DIA_K02 사람 모양 참여자(actor)·활성 막대(+·-)·메모·alt/else 구획·자동 번호·캡션을 담는다.

## 묶음과 무리

```mermaid
sequenceDiagram
    box 서버
        participant App as 앱
        participant Api as API 서버
    end
    participant Db as DB
    activate Api
    loop 매번 확인
        App->>Api: 상태 조회
        Api->>Db: SELECT
        Db-->>Api: 결과
        Api-->>App: 응답
    end
    deactivate Api
    par 동시 진행
        App->>App: 로그 정리
    and
        App->>App: 캐시 갱신
    end
    opt 지연이 크면
        App--)App: 재시도 표시
    end
```

DIA_K03 무리(box) 띠·loop·par/and·opt·가는 화살촉(-))·activate/deactivate 문장을 담는다.

## 그리지 않는 것

```mermaid
sequenceDiagram
    link U: 문서 @ https://example.com/doc
    U->>S: 주문
    rect rgb(240,240,255)
        S->>S: 재고 확인
    end
```

DIA_K04 링크 메뉴(link)와 rect 의 색은 쓰지 않고 그림은 그린다(경고 각 한 번).

```mermaid
sequenceDiagram
    U-|\S: 반쪽 화살표는 이 판이 그리지 않는다
```

DIA_K05 반쪽 화살표는 원문 코드 블록 + 경고.

## ER 다이어그램

```mermaid
---
title: 주문 모델
---
erDiagram
    direction LR
    고객 ||--o{ 주문 : 넣는다
    주문 ||--|{ 주문_항목 : 담는다
    상품 |o..o{ 주문_항목 : "팔린다"
    고객 |o--o| 고객 : 추천
    고객 {
        string 고객_번호 PK
        string 이름 "실명"
    }
    주문 {
        int 주문_번호 PK
        string 고객_번호 FK
        date 주문일
    }
    주문_항목["주문 항목"] {
        int 주문_번호 PK, FK
        int 수량 "1 이상"
    }
```

DIA_L01 엔티티 상자(머리 + 속성 표 — 타입·이름·키·설명)·까마귀발 기수 네 종·점선 비식별 관계·자기 관계·별칭을 담는다.

```mermaid
erDiagram
    A ||--o{ B : x
    classDef 강조 fill:#f9f
    A one or more B : y
```

DIA_L02 낱말 기수 줄 하나 때문에 그림 전체가 원문 코드 블록 + 경고 하나(그리지 않으니 classDef 경고는 따로 내지 않는다).

## 상태도

```mermaid
---
title: 주문 처리
---
stateDiagram-v2
    state 확인 <<choice>>
    [*] --> 접수
    접수 --> 확인 : 결제 요청
    확인 --> 취소 : 실패
    확인 --> 포장 : 성공
    state "배송 중" as 배송 {
        state 갈래 <<fork>>
        state 합류 <<join>>
        [*] --> 포장
        포장 --> 갈래
        갈래 --> 출고
        갈래 --> 안내
        출고 --> 합류
        안내 --> 합류
        합류 --> [*]
    }
    포장 : 상자에 담기
    포장 : 송장 붙이기
    합류 --> 완료
    완료 --> [*]
    취소 --> [*]
    classDef done fill:#eee
```

DIA_M01 시작·끝 원·선택 마름모·갈래·합류 막대·복합 상태(제목 칸)·전이 라벨·설명 둘(머리 + 구분선 + 설명)을 담는다. classDef 줄은 쓰지 않은 줄 경고 하나.

```mermaid
stateDiagram-v2
    A --> B
    note right of A : 메모
```

DIA_M02 메모 줄 하나 때문에 그림 전체가 원문 코드 블록 + 경고 하나.

## 클래스도

```mermaid
---
title: 주문 클래스
---
classDiagram-v2
    direction LR
    accTitle: 주문 모델
    class 고객 {
        +이름 String
        -전화$
        +주문하기(상품) 주문
    }
    class 결제수단 {
        <<interface>>
        +결제(금액) bool*
    }
    class 빈칸
    고객 "1" --> "*" 주문 : 주문한다
    주문 *-- 주문항목
    주문항목 o-- 상품
    결제수단 <|.. 카드결제
    주문 ..> 결제수단 : 사용
    주문 -- 주문 : 묶음
    주문 : +합계() decimal
    class 저장소~T~ {
        +찾기(키 List~K~) Map~K, T~
    }
    저장소~주문~ <-- 주문 : 보관
    classDef hot fill:#f9f
```

DIA_N01 이름 칸(주석 «interface» + 굵은 이름)·속성 칸·메서드 칸·빈 칸·가시성·정적 밑줄·추상 기울임·상속·실현·합성·집합·의존·자기 관계·양끝 수량 글·관계 라벨·제네릭(이름 `저장소<T>`·멤버 `List<K>`·`Map<K, T>`)을 담는다. classDef 줄은 쓰지 않은 줄 경고 하나.

```mermaid
classDiagram
    class 목록~T~
    목록 : +쌍 a~T~ b~U~
```

DIA_N02 받지 않는 제네릭(원본이 바깥 짝으로 묶는 형제 제네릭) 줄 하나 때문에 그림 전체가 원문 코드 블록 + 경고 하나. 문서 끝.
