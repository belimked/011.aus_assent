import {areaProperties} from './patch';
import type {AppState} from '../data/types';

// A rectangle covering the Carnaby St / Delfin Dr neighbourhood in Macgregor.
const pts = [[-27.570,153.065],[-27.570,153.071],[-27.566,153.071],[-27.566,153.065]];
test('Macgregor QLD addresses are available for drawn patch counts', () => {
  const inside = areaProperties({properties:[]} as unknown as AppState, pts);
  const streets = new Set(inside.filter(p=>p.suburb==='Macgregor').map(p=>p.street));
  expect(streets.has('Carnaby St')).toBe(true);
  expect(streets.has('Delfin Dr')).toBe(true);
  expect(inside).toHaveLength(198);
});
