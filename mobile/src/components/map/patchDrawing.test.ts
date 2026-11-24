import {areaPoints,canFinish,validAreaName} from './patchDrawing';

test('area names need a letter: not empty, digits, emoji or symbols only',()=>{
  for(const bad of ['  ',' 8 ','2026','8️⃣','1️⃣2️⃣','#8','8-12','🏠',' 3.5 '])expect(validAreaName(bad)).toBe(false);
  for(const ok of ['Zone 8','8 Gager St','Gager West','区域8','日光 2','É'])expect(validAreaName(ok)).toBe(true);
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
