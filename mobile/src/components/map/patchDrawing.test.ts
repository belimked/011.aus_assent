import {areaPoints,canFinish,validAreaName} from './patchDrawing';

test('area names require words, not empty or all digits',()=>{
  expect(validAreaName('  ')).toBe(false);
  expect(validAreaName(' 8 ')).toBe(false);
  expect(validAreaName('2026')).toBe(false);
  expect(validAreaName('Zone 8')).toBe(true);
});
test('polygon needs three corners and undo removes last point',()=>{
  const corners=[[1,1],[2,2],[3,3]];
  expect(canFinish(corners,null)).toBe(true);
  expect(canFinish(corners.slice(0,-1),null)).toBe(false);
});
test('radius uses selected centre and metres rather than corner count',()=>{
  const centre:[number,number]=[-27.6,153];
  expect(canFinish([],centre)).toBe(true);
  expect(areaPoints([],centre,300)).toHaveLength(36);
  expect(areaPoints([],centre,500)[0][1]).toBeGreaterThan(areaPoints([],centre,300)[0][1]);
});
