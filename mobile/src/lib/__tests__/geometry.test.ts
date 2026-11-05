import { contains, maskMobile, nearest, pointInPolygon, ringInBounds, tenureColor } from '../geometry'
const bounds = { south: 0, north: 2, west: 0, east: 2 }
test('polygon containment and viewport clipping', () => {
  const ring = [{ lat: -1, lng: -1 }, { lat: -1, lng: 3 }, { lat: 3, lng: 3 }, { lat: 3, lng: -1 }]
  expect(pointInPolygon({ lat: 1, lng: 1 }, ring)).toBe(true)
  expect(pointInPolygon({ lat: 4, lng: 1 }, ring)).toBe(false)
  expect(ringInBounds(ring, bounds)).toBe(true)
  expect(contains({ lat: 1, lng: 1 }, bounds)).toBe(true)
  expect(ringInBounds([{ lat: 4, lng: 4 }], bounds)).toBe(false)
})
test('nearest, tenure and mask', () => {
  expect(nearest([{ lat: 0, lng: 0 }, { lat: 1, lng: 1 }], { lat: 0.9, lng: 1 })?.lat).toBe(1)
  expect([0, 3, 4, 7, 8, 14, 15].map(tenureColor)).toEqual(['#e5e7eb', '#e5e7eb', '#9ca3af', '#9ca3af', '#4b5563', '#4b5563', '#0b5d8a'])
  expect(maskMobile('0412345678')).toBe('0412 345 ••••')
})
