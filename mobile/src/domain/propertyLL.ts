/** Map point of a property. Phones map only data.ll ([lat,lng]); legacy server rows may carry lat/lng only. Bounds match server/property_ll.go. */
type Located={ll?:unknown;lat?:unknown;lng?:unknown;[key:string]:any};
const num=(v:unknown)=>typeof v==='number'?v:typeof v==='string'&&v.trim()!==''?Number(v):NaN;
const plausible=(lat:number,lng:number)=>Number.isFinite(lat)&&Number.isFinite(lng)&&lat>=-44&&lat<=-9&&lng>=112&&lng<=154;
export function propertyLL(p:Located|null|undefined):[number,number]|undefined{
 if(!p)return undefined;
 if(Array.isArray(p.ll)&&p.ll.length===2){const lat=num(p.ll[0]),lng=num(p.ll[1]);if(plausible(lat,lng))return [lat,lng];}
 const lat=num(p.lat),lng=num(p.lng);return plausible(lat,lng)?[lat,lng]:undefined;
}
/** Returns the same object when nothing changes, so immer/structural sharing stays intact. */
export function withPropertyLL<T extends Located>(p:T):T{
 const ll=propertyLL(p);if(!ll)return p;
 if(Array.isArray(p.ll)&&p.ll.length===2&&p.ll[0]===ll[0]&&p.ll[1]===ll[1])return p;
 return {...p,ll};
}
