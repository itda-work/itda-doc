// dashboard.js 의 배치(pack)·타일 크기(tileWidth)를 node 로 돌린다(board_test.go TestBoardPack). 인자: dashboard.js.
// 배치는 DOM 없이 숫자만 다룬다 — 같은 입력이면 같은 배치, 읽는 차례 보존(빈 칸은 남긴다), 2×1 이 줄 끝에 안 들어가면 다음 줄, 판이 없다(줄만 는다), 줄 나눔·구역 줄,
// 자리·크기(at·span — 고정·자동 섞임, 구역 상대 행, 높이 2 이상, 건너뛰기, 겹침, 격자 밖, tileAttrs).
// zoomBox(크게 보기 자리)도 숫자만 다룬다.
"use strict";
const fs = require("fs"), vm = require("vm");

const ctx = { window: {}, document: {}, performance: { now: () => 0 } };
vm.createContext(ctx);
vm.runInContext(fs.readFileSync(process.argv[2], "utf8"), ctx);
const B = ctx.window.ItdaBoard;

let cases = 0, failed = 0;
function eq(name, got, want) {
  cases++;
  const g = JSON.stringify(got), w = JSON.stringify(want);
  if (g !== w) { failed++; console.log("FAIL " + name + "\n  got  " + g + "\n  want " + w); }
}
const T = w => ({ w }), BRK = { brk: true }, SEC = { sec: true };
const at = (r, c, w) => ({ r, c, w });

// 1. 4열에 1·1·1·1 / 2·2 / 2·2 — 명세 dashboard.md 견본의 꼴
eq("견본 꼴", B.pack([T(1), T(1), T(1), T(1), T(2), T(2), T(2), T(2)], 4),
  { place: [at(0, 0, 1), at(0, 1, 1), at(0, 2, 1), at(0, 3, 1), at(1, 0, 2), at(1, 2, 2), at(2, 0, 2), at(2, 2, 2)], rows: 3, sections: [] });

// 2. 읽는 차례 보존: 1·1·1 다음 2×1 은 남은 한 칸에 안 들어가 다음 줄로(빈 칸을 남긴다), 그 뒤 1×1 은 앞의 빈 칸으로 당기지 않는다
eq("줄 끝 2×1", B.pack([T(1), T(1), T(1), T(2), T(1)], 4),
  { place: [at(0, 0, 1), at(0, 1, 1), at(0, 2, 1), at(1, 0, 2), at(1, 2, 1)], rows: 2, sections: [] });

// 3. 판이 없다: 타일이 아무리 많아도 줄만 늘어난다(100개 — 4열이면 25줄)
{
  const r = B.pack(Array.from({ length: 100 }, () => T(1)), 4);
  eq("판 없음: 100개는 25줄", [r.rows, r.place[99]], [25, at(24, 3, 1)]);
}

// 4. cols=2: 2×1 은 한 줄을 다 쓴다, 1×1 다음 2×1 은 다음 줄
eq("cols=2", B.pack([T(2), T(1), T(2), T(1), T(1)], 2),
  { place: [at(0, 0, 2), at(1, 0, 1), at(2, 0, 2), at(3, 0, 1), at(3, 1, 1)], rows: 4, sections: [] });

// 5. 가로줄·쪽 나눔: 다음 줄부터. 이미 줄 머리면 무시(맨 앞·연속·꽉 찬 줄 뒤)
eq("줄 나눔", B.pack([BRK, T(1), T(1), BRK, BRK, T(2)], 4),
  { place: [{ r: 0 }, at(0, 0, 1), at(0, 1, 1), { r: 1 }, { r: 1 }, at(1, 0, 2)], rows: 2, sections: [] });
eq("줄 나눔: 꽉 찬 줄 뒤에는 빈 줄이 없다", B.pack([T(2), T(2), BRK, T(1)], 4),
  { place: [at(0, 0, 2), at(0, 2, 2), { r: 1 }, at(1, 0, 1)], rows: 2, sections: [] });

// 6. 구역 제목 줄: 한 줄을 다 쓰고, 앞줄의 빈 칸은 그대로, 맨 앞·연속이면 빈 줄 없이 차례로
eq("구역 줄", B.pack([T(1), SEC, T(1), SEC, SEC, T(2)], 4),
  { place: [at(0, 0, 1), { r: 1, sec: true }, at(2, 0, 1), { r: 3, sec: true }, { r: 4, sec: true }, at(5, 0, 2)], rows: 6, sections: [1, 3, 4] });
eq("구역 줄: 맨 앞", B.pack([SEC, T(1)], 4), { place: [{ r: 0, sec: true }, at(1, 0, 1)], rows: 2, sections: [0] });
eq("구역 줄: 맨 끝", B.pack([T(1), SEC], 4), { place: [at(0, 0, 1), { r: 1, sec: true }], rows: 2, sections: [1] });
eq("구역 줄 뒤 가로줄은 무시", B.pack([SEC, BRK, T(1)], 4), { place: [{ r: 0, sec: true }, { r: 1 }, at(1, 0, 1)], rows: 2, sections: [0] });

// 7. 너비는 cols 를 넘지 않는다(cols=1 이면 2×1 도 한 칸), 이상한 값은 1
eq("너비 제한", B.pack([T(2), T(0), T(7)], 1),
  { place: [at(0, 0, 1), at(1, 0, 1), at(2, 0, 1)], rows: 3, sections: [] });

// 8. 같은 입력이면 같은 배치(여러 번 불러도)
const big = Array.from({ length: 40 }, (_, i) => (i % 11 === 3 ? SEC : i % 7 === 3 ? BRK : T(1 + (i * 5) % 2)));
eq("결정적", B.pack(big, 4), B.pack(big, 4));
eq("빈 입력", B.pack([], 4), { place: [], rows: 0, sections: [] });

// 9. 자리·크기(itda-work/itda-hyve#1038): at=[열, 행](1부터, 구역 안 행), span=[너비, 높이]
const F = (w, at, span) => Object.assign({ w }, at ? { at } : {}, span ? { span } : {});
const ath = (r, c, w, h) => ({ r, c, w, h });
// 고정·자동 섞임: 고정 타일을 먼저 놓고, 나머지는 읽는 차례로 찬 칸을 건너뛴다(앞의 빈 칸은 당기지 않는다)
eq("섞임: 고정이 먼저, 자동은 찬 칸을 건너뜀", B.pack([T(1), F(1, [2, 1]), T(1), T(1), T(1)], 4),
  { place: [at(0, 0, 1), at(0, 1, 1), at(0, 2, 1), at(0, 3, 1), at(1, 0, 1)], rows: 2, sections: [] });
eq("섞임: 뒤에 적은 고정 타일이 앞 자리를 먼저 갖는다", B.pack([T(1), T(1), F(1, [1, 1])], 4),
  { place: [at(0, 1, 1), at(0, 2, 1), at(0, 0, 1)], rows: 1, sections: [] });
// 건너뛰기: 2칸 타일이 고정 칸에 걸리면 한 칸씩 오른쪽으로 — 커서 앞의 빈 칸(0열)은 남는다
eq("건너뛰기: 2칸이 고정 칸을 넘어 감", B.pack([F(1, [2, 1]), T(2), T(1)], 4),
  { place: [at(0, 1, 1), at(0, 2, 2), at(1, 0, 1)], rows: 2, sections: [] });
// 높이 2 이상: 아래 줄의 찬 칸도 건너뛴다
eq("높이: 2×2 고정 아래 줄을 건너뜀", B.pack([F(1, [1, 1], [2, 2]), T(2), T(1), T(1), T(1)], 4),
  { place: [ath(0, 0, 2, 2), at(0, 2, 2), at(1, 2, 1), at(1, 3, 1), at(2, 0, 1)], rows: 3, sections: [] });
eq("높이: span 만 적으면 자리는 자동", B.pack([T(1), F(1, null, [2, 3]), T(1)], 4),
  { place: [at(0, 0, 1), ath(0, 1, 2, 3), at(0, 3, 1)], rows: 3, sections: [] });
eq("높이: 자동 2×2 다음 넓은 타일은 비는 줄까지 내려감", B.pack([F(1, null, [2, 2]), T(2), T(4)], 4),
  { place: [ath(0, 0, 2, 2), at(0, 2, 2), at(2, 0, 4)], rows: 3, sections: [] });
// 빈 줄: at 의 행이 멀면 그 사이 줄은 빈 채로 남고, 구역 높이는 마지막 행까지
eq("빈 줄: 3행에 놓으면 구역이 3줄", B.pack([F(1, [4, 3])], 4), { place: [at(2, 3, 1)], rows: 3, sections: [] });
// 구역 상대 행: 구역 줄·가로줄 뒤의 행 1은 그 구역의 첫 줄이다
eq("구역 상대 행", B.pack([T(1), SEC, F(1, [2, 2]), T(1), BRK, F(2, [3, 1])], 4),
  { place: [at(0, 0, 1), { r: 1, sec: true }, at(3, 1, 1), at(2, 0, 1), { r: 4 }, at(4, 2, 2)], rows: 5, sections: [1] });
eq("구역 상대 행: 앞 구역의 키 큰 타일 아래로 쌓인다", B.pack([F(1, [1, 1], [1, 3]), BRK, F(1, [1, 1])], 4),
  { place: [ath(0, 0, 1, 3), { r: 3 }, at(3, 0, 1)], rows: 4, sections: [] });
// 겹침: 뒤에 적은 고정 타일이 자동 배치로(moved 에 번호), 크기(span)는 지킨다
eq("겹침: 뒤의 것이 자동", B.pack([F(1, [1, 1], [2, 1]), F(1, [2, 1], [2, 2])], 4),
  { place: [at(0, 0, 2), ath(0, 2, 2, 2)], rows: 2, sections: [], moved: [1] });
// 내용 너비(span 없음)로만 드러나는 겹침도 같다 — 표(2칸)를 1열에, 다음 타일을 2열에
eq("겹침: 내용 너비로 겹침", B.pack([F(2, [1, 1]), F(1, [2, 1])], 4),
  { place: [at(0, 0, 2), at(0, 2, 1)], rows: 1, sections: [], moved: [1] });
// 격자 밖: span 이 격자를 넘으면 자리를 버리고 자동, span 없는 내용 너비는 그 열에서 격자 끝까지로 줄인다
eq("격자 밖: span 이 넘침", B.pack([F(1, [4, 1], [2, 1])], 4), { place: [at(0, 0, 2)], rows: 1, sections: [], moved: [0] });
eq("격자 밖: 열이 넘침", B.pack([F(1, [5, 1])], 4), { place: [at(0, 0, 1)], rows: 1, sections: [], moved: [0] });
eq("격자 끝: 내용 너비 2 를 4열에 두면 1칸", B.pack([F(2, [4, 1])], 4), { place: [at(0, 3, 1)], rows: 1, sections: [] });
eq("틀린 span 은 없는 것: 너비가 cols 보다 큼", B.pack([F(1, null, [5, 1])], 4), { place: [at(0, 0, 1)], rows: 1, sections: [] });
eq("틀린 span 은 없는 것: 높이 0", B.pack([F(2, null, [2, 0])], 4), { place: [at(0, 0, 2)], rows: 1, sections: [] });
eq("행 상한: 99행을 넘는 자리는 자동", B.pack([F(1, [1, 99], [1, 2])], 4), { place: [ath(0, 0, 1, 2)], rows: 2, sections: [], moved: [0] });
// 촘촘한 격자(12열): 자리·크기를 섞은 보드 — 마지막 고정 타일은 둘째 타일과 겹쳐 자동(커서가 2행 9열)
eq("12열 보드", B.pack([F(1, [1, 1], [3, 2]), F(1, [4, 1], [9, 1]), F(1, null, [3, 1]), T(2), F(1, [5, 1], [3, 4])], 12),
  { place: [ath(0, 0, 3, 2), at(0, 3, 9), at(1, 3, 3), at(1, 6, 2), ath(1, 8, 3, 4)], rows: 5, sections: [], moved: [4] });
const mix = Array.from({ length: 30 }, (_, i) => (i % 9 === 4 ? SEC : F(1 + (i % 2), i % 3 ? null : [1 + (i % 4), 1 + (i % 2)], i % 5 ? null : [2, 2])));
eq("결정적: 자리 섞임", B.pack(mix, 4), B.pack(mix, 4));

// tileAttrs: 제목의 data-at·data-span
const ttl = d => ({ dataset: d });
eq("속성: 둘 다", B.tileAttrs(ttl({ at: "5,1", span: "3x4" })), { at: [5, 1], span: [3, 4] });
eq("속성: 공백 허용", B.tileAttrs(ttl({ at: " 2 , 3 " })), { at: [2, 3] });
eq("속성: 틀린 꼴은 없음", B.tileAttrs(ttl({ at: "0,1", span: "3X4" })), {});
eq("속성: 음수·글자·빈 값", B.tileAttrs(ttl({ at: "-1,2", span: "ax2" })), {});
eq("속성: 제목 없음", B.tileAttrs(null), {});

// zoomBox: 격자 좌표(확대 전 px). 굴림 칸 y 100..700, 보드는 위로 굴러 칸을 넘치고 격자는 보드 안 x+30·y+80
{
  const view = { left: 0, right: 1000, top: 100, bottom: 700 };
  const page = { left: 200, right: 800, top: -500, bottom: 2000 }, grid = { left: 230, right: 770, top: -420, bottom: 1970 };
  eq("zoomBox: 확대 1 — 보이는 영역(보드 폭·칸 높이)에서 여백 12", B.zoomBox(view, page, grid, 1, 12), { left: -18, top: 532, width: 576, height: 576 });
  // 화면이 2배면 화면 px 를 절반으로 — 굴림 칸의 같은 자리를 덮는다
  const p2 = { left: 200, right: 1400, top: -500, bottom: 4000 }, g2 = { left: 260, right: 1340, top: -340, bottom: 3940 };
  eq("zoomBox: 확대 2", B.zoomBox({ left: 0, right: 1000, top: 100, bottom: 700 }, p2, g2, 2, 12), { left: -18, top: 232, width: 376, height: 276 });
  // 보드가 칸보다 짧으면 보드 끝까지만
  eq("zoomBox: 짧은 보드", B.zoomBox(view, { left: 0, right: 1000, top: 150, bottom: 400 }, { left: 30, right: 970, top: 200, bottom: 380 }, 1, 12), { left: -18, top: -38, width: 976, height: 226 });
  eq("zoomBox: 이상한 비율은 1", B.zoomBox(view, page, grid, 0, 0).width, 600);
}

// tileWidth: 내용 종류만 본다
// blk 는 선택자 하나(또는 그 목록 — 요소가 맞는 선택자들)에 맞는 가짜 블록이다. 구조 게이트를 지난 KPI 는 .kpi·.kpi-tiles 둘 다 맞는다.
const blk = (sel, items = 0) => ({ matches: s => s.split(",").map(x => x.trim()).some(x => [].concat(sel).includes(x)), querySelectorAll: () => ({ length: items }) });
const KPI = [".kpi", ".kpi-tiles"];
eq("크기: 차트", B.tileWidth([blk("figure")]), 2);
eq("크기: 표", B.tileWidth([blk("table")]), 2);
eq("크기: KPI 둘", B.tileWidth([blk(KPI, 2)]), 1);
eq("크기: KPI 셋", B.tileWidth([blk(KPI, 3)]), 2);
// 구조 없는 회사 어휘 .kpi(표식 없음)는 글 타일과 같다 — 항목처럼 보이는 자식이 있어도
eq("크기: 구조 없는 kpi", B.tileWidth([blk(".kpi", 3)]), 1);
eq("크기: 글", B.tileWidth([blk("p")]), 1);
eq("크기: 여럿", B.tileWidth([blk("figure"), blk("p")]), 1);

console.log("BOARD DONE cases=" + cases + " failed=" + failed);
