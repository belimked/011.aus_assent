import type {AppState} from '../data/types';

export type AreaProperty={id:string;no:string;street:string;suburb:string;ll:[number,number];lotplan?:string;hasContact:boolean;source:string;inRex:boolean};
export function areaProperties(s:AppState,pts:number[][]):AreaProperty[]{
 if(pts.length<3)return [];
 const lat=pts.map(p=>p[0]),lng=pts.map(p=>p[1]);const minLat=Math.min(...lat),maxLat=Math.max(...lat),minLng=Math.min(...lng),maxLng=Math.max(...lng);
 const within=(ll:number[])=>ll[0]>=minLat&&ll[0]<=maxLat&&ll[1]>=minLng&&ll[1]<=maxLng&&pointInPolygon(ll,pts);
 return s.properties.filter(p=>p.ll?.length===2&&within(p.ll)).map(p=>({id:p.id,no:p.no,street:p.street,suburb:p.suburb,ll:p.ll as [number,number],lotplan:p.lotplan,hasContact:p.ownerIds?.length>0,source:p.source||'',inRex:!!p.rexId}));
}


/** Suggest a name from the most common suburb of synced properties inside; ties follow property order. */
export function suggestAreaName(s:AppState,pts:number[][],fallback='Patch'):string {
 const inside=areaProperties(s,pts);
 const counts=new Map<string,number>();
 for(const p of inside)if(p.suburb)counts.set(p.suburb,(counts.get(p.suburb)||0)+1);
 const suburb=[...counts].sort((a,b)=>b[1]-a[1])[0]?.[0]||fallback;
 return `${suburb} ${1+s.zones.filter(z=>z.name.startsWith(suburb)).length}`;
}

export function pointInPolygon(p:number[],poly:number[][]){let inside=false;for(let i=0,j=poly.length-1;i<poly.length;j=i++){const a=poly[i],b=poly[j];if((a[1]>p[1])!==(b[1]>p[1])&&p[0]<(b[0]-a[0])*(p[1]-a[1])/(b[1]-a[1])+a[0])inside=!inside}return inside}
export function propertiesInArea(s:AppState,pts:number[][]){return s.properties.filter(p=>p.ll&&pointInPolygon(p.ll,pts))}
export function tagArea(s:AppState,name:string,pts:number[][]){const inside=propertiesInArea(s,pts);let tagged=0;for(const p of inside)for(const id of p.ownerIds){const c=s.contacts.find(x=>x.id===id);if(c&&!c.tags.includes('Area: '+name)){c.tags.push('Area: '+name);tagged++}}return tagged}
