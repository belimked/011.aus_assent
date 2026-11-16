import type {Zone} from '../data/types';

export function mergeSuburbsZone(zones:Zone[],userId:string,suburbs:string[]):Zone[]{
 const existing=zones.find(z=>z.type==='suburbs');
 const record:Zone={id:existing?.id||`zone-suburbs-${userId}`,type:'suburbs',name:'Monitored suburbs',suburbs:[...new Set(suburbs)]};
 return [...zones.filter(z=>z.type!=='suburbs'),record];
}
export function monitoredSuburbs(zones:Zone[],fallback:string[]):string[]{
 return zones.find(z=>z.type==='suburbs')?.suburbs||fallback;
}
