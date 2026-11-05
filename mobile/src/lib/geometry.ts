export type Point = { lat: number; lng: number }
export type Bounds = { south: number; north: number; west: number; east: number }

export function contains(p: Point, b: Bounds): boolean {
  return p.lat >= b.south && p.lat <= b.north && p.lng >= b.west && p.lng <= b.east
}

export function pointInPolygon(p: Point, ring: Point[]): boolean {
  let inside = false
  for (let i = 0, j = ring.length - 1; i < ring.length; j = i++) {
    const a = ring[i], b = ring[j]
    if ((a.lat > p.lat) !== (b.lat > p.lat) &&
        p.lng < ((b.lng - a.lng) * (p.lat - a.lat)) / (b.lat - a.lat) + a.lng) inside = !inside
  }
  return inside
}

// Segment/bounding-box overlap so parcels whose centroid is outside the screen aren't dropped.
export function ringInBounds(ring: Point[], b: Bounds): boolean {
  if (!ring.length) return false
  const lats = ring.map(p => p.lat), lngs = ring.map(p => p.lng)
  return Math.max(...lats) >= b.south && Math.min(...lats) <= b.north &&
    Math.max(...lngs) >= b.west && Math.min(...lngs) <= b.east
}

export function nearest<T extends Point>(properties: T[], p: Point): T | undefined {
  let best: T | undefined, distance = Infinity
  for (const property of properties) {
    const d = (property.lat - p.lat) ** 2 + ((property.lng - p.lng) * Math.cos(p.lat * Math.PI / 180)) ** 2
    if (d < distance) { best = property; distance = d }
  }
  return best
}

export function tenureColor(years: number): string {
  return years <= 3 ? '#e5e7eb' : years <= 7 ? '#9ca3af' : years <= 14 ? '#4b5563' : '#0b5d8a'
}

export function maskMobile(_mobile?: string): string { return '0412 345 ••••' }
