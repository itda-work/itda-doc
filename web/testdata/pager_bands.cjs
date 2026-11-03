const fs = require('fs');
const vm = require('vm');
const assert = require('assert');
const path = require('path');
const window = {};
vm.runInNewContext(fs.readFileSync(path.join(__dirname, '../pager.js'), 'utf8'), { window });
const el = (tag, line, children = [], continued = false, bodyLine = false, generated = false) => ({
  tagName: tag, children, classList: { contains: x => x === 'cont' && continued },
  getAttribute: x => x === 'data-line' ? line == null ? null : String(line) : x === 'data-body-line' && bodyLine ? String(line) : null,
  closest: () => null,
  matches: () => generated,
});
const page = (n, children, head = null, continuedHead = false) => ({
  dataset: { page: String(n) },
  querySelector: x => x === '.page-body' ? { children } : x === '.slide-head' ? head : x === '.slide-head .slide-cont' && continuedHead ? {} : null,
});
assert.ok(window.ItdaPager && window.HyvePager === window.ItdaPager, '옛 이름 HyvePager 는 ItdaPager 와 같은 객체'); // itda-hyve#625 별칭
const bands = window.ItdaPager.pageBands;
const plain = bands([page(1, [el('P', 1)]), page(2, [el('H1', 20)])], 'doc');
assert.deepStrictEqual(JSON.parse(JSON.stringify(plain)), [{ page: 1, total: 2, line: 1 }, { page: 2, total: 2, line: 20 }]);
const cases = bands([
  page(1, [el('P', 1)]),
  page(2, [el('P', 1, [], true), el('P', 9)]),
  page(3, [el('PRE', 15, [], true, true)]),
  page(4, [el('DIV', null), el('H1', 22)]),
], 'doc');
assert.deepStrictEqual(Array.from(cases, b => b.line), [1, 9, 15, 22]);
// report.md: 표지는 0, 생성 차례가 1쪽, 본문 첫 블록은 2쪽. 머리말은 어느 띠에도 속하지 않는다.
const report = bands([
  page(0, [el('H1', null)]),
  page(1, [el('NAV', 1, [], false, false, true)]),
  page(2, [el('P', 20)]), page(3, [el('H1', 40)]),
], 'doc');
assert.deepStrictEqual(JSON.parse(JSON.stringify(report)), [
  { page: 2, total: 3, line: 20 }, { page: 3, total: 3, line: 40 },
]);
// 머리말 없는 문서는 첫 줄부터, 슬라이드 제목은 본문 밖의 .slide-head 줄을 쓴다.
const slides = bands([page(1, [el('P', 2)], el('DIV', null, [el('H1', 1)])), page(2, [el('P', 8)])], 'slides');
assert.deepStrictEqual(JSON.parse(JSON.stringify(slides)), [
  { page: 1, total: 2, line: 1 }, { page: 2, total: 2, line: 8 },
]);
assert.strictEqual(bands([page(1, [el('P', 1)])], 'flow').length, 0);
// 넘친 슬라이드 제목은 원래 제목 줄을 되풀이한다. 새 블록이 없으면 앞 구간에 포함하고,
// 새 블록이 있으면 반복 제목 대신 그 줄을 쓴다. 역순·같은 줄은 새 띠를 만들지 않는다.
const head = el('DIV', null, [el('H1', 20)]);
const continued = bands([page(1, [el('P', 21)], head),
  page(2, [el('P', 21, [], true)], head, true),
  page(3, [el('P', 21, [], true), el('P', 40)], head, true),
  page(4, [el('P', 40)]), page(5, [el('P', 30)]), page(6, [el('H1', 50)])], 'slides');
assert.deepStrictEqual(Array.from(continued, b => [b.page, b.line]), [[1, 20], [3, 40], [6, 50]]);
console.log('PAGER BANDS DONE');
