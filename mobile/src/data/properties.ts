import raw from './sunnybank.json'
import { type Point, type Bounds, contains, ringInBounds } from '@/lib/geometry'

export const ATTRIBUTION = 'Addresses & parcels © State of Queensland (CC BY 4.0)'
type AddressTuple = [string, string, string, string, string, number, number, number]
type ParcelTuple = { lp: string; area: number; r: number[][] }
const data = raw as unknown as { origin: number[]; addresses: AddressTuple[]; parcels: ParcelTuple[] }
const decode = (x: number, y: number): Point => ({ lng: data.origin[0] + x / 1e6, lat: data.origin[1] + y / 1e6 })

export type Property = Point & {
  id: string; unit: string; number: string; street: string; suburb: string; lotplan: string
  area?: number; ring?: Point[]; units: number; status: 'green' | 'red' | 'blue'; owner: string; yearsHeld: number
}

function seeded(key: string): number {
  let h = 2166136261
  for (let i = 0; i < key.length; i++) h = Math.imul(h ^ key.charCodeAt(i), 16777619)
  return (h >>> 0) / 4294967296
}

const parcels = new Map(data.parcels.map(p => [p.lp, p]))
const groups = new Map<string, AddressTuple[]>()
for (const a of data.addresses) {
  const key = `${a[1]}|${a[2]}|${a[3]}|${a[4]}`
  const group = groups.get(key) ?? []
  group.push(a)
  groups.set(key, group)
}

// One marker per complex, with units recorded; no fake owner/status is presented as CRM data.
export const properties: Property[] = [...groups.values()].map(group => {
  const a = group[0], parcel = parcels.get(a[4]), seed = seeded(`${a[1]}|${a[2]}|${a[4]}`)
  const status = seed < 0.46 ? 'green' : seed < 0.78 ? 'red' : 'blue'
  const first = ['Alex', 'Jordan', 'Taylor', 'Morgan', 'Sam', 'Casey', 'Jamie']
  const last = ['Chen', 'Nguyen', 'Patel', 'Lee', 'Smith', 'Wong', 'Brown']
  return {
    id: `${a[1]}-${a[2]}-${a[4]}`, unit: group.length > 1 ? '' : a[0],
    number: a[1], street: a[2], suburb: a[3], lotplan: a[4],
    ...decode(a[5], a[6]), area: parcel?.area,
    ring: parcel?.r.map(p => decode(p[0], p[1])), units: group.length,
    status, owner: `${first[Math.floor(seeded(a[4] + 'first') * first.length)]} ${last[Math.floor(seeded(a[4] + 'last') * last.length)]}`,
    yearsHeld: Math.floor(seeded(a[4] + 'years') * 27),
  }
})

export const DEMO_POSITION: Point = (() => {
  const a = data.addresses.find(a => a[1] === '11' && a[2] === 'Gager St')!
  return decode(a[5], a[6])
})()
export const DATA_BOUNDS: Bounds = { south: -27.65, north: -27.45, west: 152.98, east: 153.14 }
export function inDataBounds(p: Point): boolean { return contains(p, DATA_BOUNDS) }
export function visibleProperties(b: Bounds, cap = 400): Property[] {
  return properties.filter(p => contains(p, b)).slice(0, cap)
}
export const parcelShapes = data.parcels.map(p => ({
  id: p.lp, area: p.area, ring: p.r.map(pair => decode(pair[0], pair[1])),
}))
export function visibleParcels(b: Bounds, cap = 600) {
  return parcelShapes.filter(p => ringInBounds(p.ring, b)).slice(0, cap)
}
