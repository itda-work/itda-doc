// dashboard.js 의 배치(pack)·타일 크기(tileWidth)를 node 로 돌린다(board_test.go TestBoardPack). 인자: dashboard.js.
// 배치는 DOM 없이 숫자만 다룬다 — 같은 입력이면 같은 배치, 읽는 차례 보존(빈 칸은 남긴다), 2×1 이 줄 끝에 안 들어가면 다음 줄, 판이 차면 다음 판.
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
const T = w => ({ w }), BRK = { brk: true };
const at = (b, r, c, w) => ({ b, r, c, w });

// 1. 4×3 에 1·1·1·1 / 2·2 / 2·2 — 판 하나에 꼭 찬다(명세 dashboard.md 견본의 꼴)
eq("견본 꼴", B.pack([T(1), T(1), T(1), T(1), T(2), T(2), T(2), T(2)], 4, 3),
  { place: [at(0, 0, 0, 1), at(0, 0, 1, 1), at(0, 0, 2, 1), at(0, 0, 3, 1), at(0, 1, 0, 2), at(0, 1, 2, 2), at(0, 2, 0, 2), at(0, 2, 2, 2)], boards: 1 });

// 2. 읽는 차례 보존: 1·1·1 다음 2×1 은 남은 한 칸에 안 들어가 다음 줄로(빈 칸을 남긴다), 그 뒤 1×1 은 앞의 빈 칸으로 당기지 않는다
eq("줄 끝 2×1", B.pack([T(1), T(1), T(1), T(2), T(1)], 4, 3),
  { place: [at(0, 0, 0, 1), at(0, 0, 1, 1), at(0, 0, 2, 1), at(0, 1, 0, 2), at(0, 1, 2, 1)], boards: 1 });

// 3. 판 넘김: 4×2 에 2×1 다섯 — 넷이 한 판, 다섯째는 다음 판 맨 앞
eq("판 넘김", B.pack([T(2), T(2), T(2), T(2), T(2)], 4, 2),
  { place: [at(0, 0, 0, 2), at(0, 0, 2, 2), at(0, 1, 0, 2), at(0, 1, 2, 2), at(1, 0, 0, 2)], boards: 2 });

// 4. 마지막 줄 끝의 2×1 은 다음 판으로(그 판의 남은 칸은 빈 채)
eq("마지막 줄 끝", B.pack([T(2), T(2), T(1), T(1), T(1), T(2)], 4, 2),
  { place: [at(0, 0, 0, 2), at(0, 0, 2, 2), at(0, 1, 0, 1), at(0, 1, 1, 1), at(0, 1, 2, 1), at(1, 0, 0, 2)], boards: 2 });

// 5. cols=2: 2×1 은 한 줄을 다 쓴다, 1×1 다음 2×1 은 다음 줄
eq("cols=2", B.pack([T(2), T(1), T(2), T(1), T(1)], 2, 3),
  { place: [at(0, 0, 0, 2), at(0, 1, 0, 1), at(0, 2, 0, 2), at(1, 0, 0, 1), at(1, 0, 1, 1)], boards: 2 });

// 6. 판 나눔(가로줄·높은 제목): 타일이 있는 판이면 새 판, 빈 판(맨 앞·나눔 연속)이면 그 판
eq("나눔", B.pack([BRK, T(1), T(1), BRK, BRK, T(2)], 4, 3),
  { place: [{ b: 0 }, at(0, 0, 0, 1), at(0, 0, 1, 1), { b: 1 }, { b: 1 }, at(1, 0, 0, 2)], boards: 2 });

// 7. 너비는 cols 를 넘지 않는다(cols=1 이면 2×1 도 한 칸), 이상한 값은 1
eq("너비 제한", B.pack([T(2), T(0), T(7)], 1, 2),
  { place: [at(0, 0, 0, 1), at(0, 1, 0, 1), at(1, 0, 0, 1)], boards: 2 });

// 8. 같은 입력이면 같은 배치(여러 번 불러도)
const big = Array.from({ length: 40 }, (_, i) => (i % 7 === 3 ? BRK : T(1 + (i * 5) % 2)));
eq("결정적", B.pack(big, 4, 3), B.pack(big, 4, 3));
eq("빈 입력", B.pack([], 4, 3), { place: [], boards: 0 });

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
