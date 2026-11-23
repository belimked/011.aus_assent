import {sheetStack} from './stack';

test('a nested tag picker keeps the original contact form in the stack',()=>{
  const parent={kind:'quickadd',propId:'p1'};
  const child={kind:'tagpick',sel:['Buyer'],prev:parent};
  expect(sheetStack(child)).toEqual([parent,child]);
  expect(sheetStack(child)[0]).toBe(parent);
  expect(sheetStack(parent)).toEqual([parent]);
  expect(sheetStack(null)).toEqual([]);
});
