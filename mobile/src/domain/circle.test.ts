import {circlePts} from './circle';
test('radius polygon has 36 points at requested distance',()=>{for(const radius of [200,300,500,1000]){const pts=circlePts([-27.58,153.06],radius);expect(pts).toHaveLength(36);expect(pts[0][1]).toBeCloseTo(153.06+radius/(111320*Math.cos(-27.58*Math.PI/180)),8);expect(pts[9][0]).toBeCloseTo(-27.58+radius/111320,8)}});
