// incr.js(이어 만들기 공통 부품, itda-hyve#658)를 작은 가짜 DOM 으로 돌린다 — 서명(위치 속성 상대값)·밀기·끼우기·비우기. 레이아웃이 드는
// 시험(이어 만든 슬라이드 = 처음부터 만든 것)은 itda-hyve selftest 가 본다.
const fs = require('fs');
const vm = require('vm');
const assert = require('assert');
const path = require('path');
const window = {};
vm.runInNewContext(fs.readFileSync(path.join(__dirname, '../incr.js'), 'utf8'), { window });
const I = window.ItdaIncr;

// 가짜 요소: 속성·자식·outerHTML·querySelectorAll(위치 속성 고르기만)
function node(tag, attrs = {}, kids = []) {
  const n = {
    nodeType: 1, tagName: tag, attrs: Object.assign({}, attrs), kids, parent: null,
    getAttribute(a) { return a in this.attrs ? this.attrs[a] : null; },
    hasAttribute(a) { return a in this.attrs; },
    setAttribute(a, v) { this.attrs[a] = String(v); },
    get outerHTML() {
      const at = Object.entries(this.attrs).map(([k, v]) => ' ' + k + '="' + v + '"').join('');
      return '<' + tag + at + '>' + this.kids.map(k => k.outerHTML).join('') + '</' + tag + '>';
    },
    querySelectorAll(sel) {
      const want = sel.split(',').map(s => s.replace(/[[\]]/g, ''));
      const out = [];
      const walk = x => { for (const k of x.kids) { if (want.some(a => k.hasAttribute(a))) out.push(k); walk(k); } };
      walk(this);
      return out;
    },
  };
  for (const k of kids) k.parent = n;
  return n;
}
let cases = 0;
const ok = (c, msg) => { cases++; assert.ok(c, msg); };

assert.deepStrictEqual(Array.from(I.POS), ['data-line', 'data-body-line', 'data-table-line', 'data-n'], '위치 속성 목록은 incr.js 한 곳');

// 서명: 위치가 통째로 밀린 묶음은 같은 서명, 묶음 안 줄 간격이 바뀌면 다른 서명, 내용이 바뀌면 다른 서명
const grp = (l0, n0, gap = 2, text = '가') => [node('P', { 'data-line': l0, 'data-n': n0 }, [node('SPAN', { 'data-line': l0 })]),
  node('TABLE', { 'data-line': l0 + gap, 'data-n': n0 + 1, 'data-table-line': l0 + gap + 1 }, [node('TD', { 'data-body-line': l0 + gap + 2, title: text })])];
ok(I.sig(['content', ...grp(10, 3)]) === I.sig(['content', ...grp(40, 9)]), '통째로 밀린 묶음은 같은 서명');
ok(I.sig(['content', ...grp(10, 3)]) !== I.sig(['content', ...grp(10, 3, 3)]), '묶음 안 줄 간격이 바뀌면 다른 서명');
ok(I.sig(['content', ...grp(10, 3)]) !== I.sig(['content', ...grp(10, 3, 2, '나')]), '내용이 바뀌면 다른 서명');
ok(I.sig(['content', ...grp(10, 3)]) !== I.sig(['single', ...grp(10, 3)]), '글 조각(묶음 종류)도 서명에 든다');
assert.deepStrictEqual({ ...I.base(grp(10, 3)) }, { line: 10, n: 3 }, '기준은 가장 작은 줄·블록 번호');
assert.deepStrictEqual({ ...I.base([node('P')]) }, { line: 0, n: 0 }, '위치 속성이 없으면 0');
ok(I.rel('<p data-line="12" data-n="5" title="data-line=\'1\'">', { line: 10, n: 3 }) === '<p data-line="2" data-n="2" title="data-line=\'1\'">', 'rel 은 속성 꼴만 바꾼다');
ok(I.rel('<code data-line="12">글 data-line="12"</code>', { line: 10, n: 3 }) === '<code data-line="2">글 data-line="12"</code>', 'rel 은 글 노드를 건드리지 않는다');
ok(I.rel('<a title="a>b" data-line="12">', { line: 10, n: 3 }) === '<a title="a>b" data-line="2">', '속성 값 안의 > 뒤도 태그 안이다');

// 코드 줄 span(.ln — codelines.js)의 data-n 은 코드 안 줄 순번 — 서명·기준·밀기에서 빼고 본다
const code = (l0, n0) => [node('PRE', { 'data-line': l0, 'data-n': n0 }, [node('SPAN', { class: 'ln', 'data-n': 1 }), node('SPAN', { class: 'ln hl', 'data-n': 2 })])];
ok(I.sig(code(10, 3)) === I.sig(code(50, 20)), '코드 줄 순번은 위치가 아니다(밀린 코드 묶음은 같은 서명)');
assert.deepStrictEqual({ ...I.base(code(10, 3)) }, { line: 10, n: 3 });
const cb = node('DIV', {}, code(10, 3));
I.shift(cb, 1, 4);
assert.deepStrictEqual(cb.kids[0].kids.map(x => String(x.getAttribute('data-n'))), ['1', '2']);
assert.strictEqual(cb.kids[0].getAttribute('data-n'), '7');
cases += 3;

// 밀기: 줄 속성은 dl, 블록 번호는 dn 만큼. 0 이하 줄 값은 그대로
const g = grp(10, 3);
const box = node('DIV', {}, g);
I.shift(box, 5, 2);
assert.deepStrictEqual(g.map(x => x.getAttribute('data-line')), ['15', '17']);
assert.deepStrictEqual(g.map(x => x.getAttribute('data-n')), ['5', '6']);
assert.strictEqual(g[1].getAttribute('data-table-line'), '18');
assert.strictEqual(g[1].kids[0].getAttribute('data-body-line'), '19');
assert.strictEqual(g[0].kids[0].getAttribute('data-line'), '15');
ok(I.sig(['content', ...g]) === I.sig(['content', ...grp(30, 7)]), '민 뒤에도 서명은 같다');
const zero = node('P', { 'data-line': '0' });
I.shift(zero, 4, 0);
assert.strictEqual(zero.getAttribute('data-line'), '0');
cases += 6;

// 끼우기: 남는 자식은 떼고 새 노드만 끼운다 — 그대로 쓰는 노드는 옮기지 않는다
function parent(kids) {
  const p = {
    childNodes: kids.slice(),
    get firstChild() { return this.childNodes[0] || null; },
    insertBefore(x, ref) {
      const from = this.childNodes.indexOf(x);
      if (from >= 0) this.childNodes.splice(from, 1);
      const i = ref ? this.childNodes.indexOf(ref) : this.childNodes.length;
      this.childNodes.splice(i, 0, x);
      x.parent = this; x.moved = (x.moved || 0) + 1;
    },
  };
  for (const k of p.childNodes) {
    k.parent = p;
    k.remove = function () { const i = p.childNodes.indexOf(this); if (i >= 0) p.childNodes.splice(i, 1); this.parent = null; };
    Object.defineProperty(k, 'nextSibling', { configurable: true, get() { const i = p.childNodes.indexOf(this); return i >= 0 ? p.childNodes[i + 1] || null : null; } });
  }
  return p;
}
const [a, b, c, d] = ['A', 'B', 'C', 'D'].map(t => node(t));
const p = parent([a, b, c, d]);
const x = node('X'), y = node('Y');
const r = I.place(p, [a, x, c, y]);
assert.deepStrictEqual(p.childNodes.map(n => n.tagName), ['A', 'X', 'C', 'Y']);
assert.deepStrictEqual({ ...r }, { removed: 2, added: 2 });
ok(!a.moved && !c.moved, '그대로 쓰는 노드는 옮기지 않는다');

// 비우기: 기억의 모든 값을 놓는다
const memo = { key: 'k', units: [{ pages: [a] }] };
I.clear(memo);
assert.strictEqual(Object.keys(memo).length, 0);
I.clear(null);
cases++;
console.log('INCR DONE cases=' + cases + ' failed=0');
