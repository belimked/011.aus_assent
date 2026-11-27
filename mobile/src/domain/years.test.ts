import {yearsSince,yearsText} from './index';
const en=(e:string)=>e,zh=(_:string,z:string)=>z;
test('yearsSince counts whole years from the given date, not a fixed year',()=>{
  const now=new Date(2027,2,15);
  expect(yearsSince('2026-03',now)).toBe(1);
  expect(yearsSince('2026-04',now)).toBe(0);
  expect(yearsSince('2020-03-16',now)).toBe(6);
  expect(yearsSince('2020-03-15',now)).toBe(7);
  expect(yearsSince('2009',now)).toBe(18);
  expect(yearsSince('',now)).toBe('');
  expect(yearsSince('n/a',now)).toBe('');
  expect(yearsSince(`${new Date().getFullYear()-3}-01-01`)).toBe(3);
});
test('yearsText uses the singular for one year in English only',()=>{
  expect([0,1,2].map(n=>yearsText(n,en))).toEqual(['0 years','1 year','2 years']);
  expect(yearsText(1,zh)).toBe('1 年');
  expect(yearsText('',en)).toBe('');
});
