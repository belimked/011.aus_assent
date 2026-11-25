import type {AppState} from '../data/types';
import {properties as mapProperties} from '../data/properties';

export type AreaProperty={id:string;no:string;street:string;suburb:string;ll:[number,number];lotplan?:string;hasContact:boolean;source:string;inRex:boolean};
const addressKey=(no:string,street:string,suburb:string)=>`${no} ${street.replace(/\b(street|st)\b/gi,'st').replace(/\b(drive|dr)\b/gi,'dr')} ${suburb}`.toLowerCase().replace(/[^a-z0-9]/g,'');
export function areaProperties(s:AppState,pts:number[][]):AreaProperty[]{
 if(pts.length<3)return [];
 const lat=pts.map(p=>p[0]),lng=pts.map(p=>p[1]);const minLat=Math.min(...lat),maxLat=Math.max(...lat),minLng=Math.min(...lng),maxLng=Math.max(...lng);
 const within=(ll:number[])=>ll[0]>=minLat&&ll[0]<=maxLat&&ll[1]>=minLng&&ll[1]<=maxLng&&pointInPolygon(ll,pts);
 const found=new Map<string,AreaProperty>();const byLot=new Map<string,string>();const byAddress=new Map<string,string>();
 const add=(item:AreaProperty)=>{const lot=item.lotplan?.trim().toLowerCase();const address=addressKey(item.no,item.street,item.suburb);const k=(lot&&byLot.get(lot))||byAddress.get(address)||lot||address;
  found.set(k,item);if(lot)byLot.set(lot,k);byAddress.set(address,k)};
 for(const p of mapProperties){const ll:[number,number]=[p.lat,p.lng];if(within(ll))add({id:p.id,no:p.number,street:p.street,suburb:p.suburb,ll,lotplan:p.lotplan,hasContact:false,source:'qld',inRex:false})}
 for(const p of s.properties)if(p.ll?.length===2&&within(p.ll))add({id:p.id,no:p.no,street:p.street,suburb:p.suburb,ll:p.ll as [number,number],lotplan:p.lotplan,hasContact:p.ownerIds?.length>0,source:p.source||'',inRex:!!p.rexId});
 return [...found.values()];
}


/** Suggest a name for a finished polygon; ties follow property order. */
export function suggestAreaName(s:AppState,pts:number[][]):string {
 const inside=areaProperties(s,pts);
 const counts=new Map<string,number>();
 for(const p of inside)if(p.suburb)counts.set(p.suburb,(counts.get(p.suburb)||0)+1);
 let suburb:string|undefined=[...counts].sort((a,b)=>b[1]-a[1])[0]?.[0];
 if(!suburb&&pts.length){
  let twiceArea=0,cx=0,cy=0;
  for(let i=0;i<pts.length;i++){const a=pts[i],b=pts[(i+1)%pts.length],cross=a[0]*b[1]-b[0]*a[1];twiceArea+=cross;cx+=(a[0]+b[0])*cross;cy+=(a[1]+b[1])*cross}
  const center=twiceArea? [cx/(3*twiceArea),cy/(3*twiceArea)] : [pts.reduce((v,p)=>v+p[0],0)/pts.length,pts.reduce((v,p)=>v+p[1],0)/pts.length];
  suburb=mapProperties.reduce<{suburb:string;distance:number}|null>((best,p)=>{const distance=(p.lat-center[0])**2+(p.lng-center[1])**2;return !best||distance<best.distance?{suburb:p.suburb,distance}:best},null)?.suburb||undefined;
 }
 suburb=suburb||'Patch';
 return `${suburb} ${1+s.zones.filter(z=>z.name.startsWith(suburb)).length}`;
}

export function pointInPolygon(p:number[],poly:number[][]){let inside=false;for(let i=0,j=poly.length-1;i<poly.length;j=i++){const a=poly[i],b=poly[j];if((a[1]>p[1])!==(b[1]>p[1])&&p[0]<(b[0]-a[0])*(p[1]-a[1])/(b[1]-a[1])+a[0])inside=!inside}return inside}
export function propertiesInArea(s:AppState,pts:number[][]){return s.properties.filter(p=>p.ll&&pointInPolygon(p.ll,pts))}
export function tagArea(s:AppState,name:string,pts:number[][]){const inside=propertiesInArea(s,pts);let tagged=0;for(const p of inside)for(const id of p.ownerIds){const c=s.contacts.find(x=>x.id===id);if(c&&!c.tags.includes('Area: '+name)){c.tags.push('Area: '+name);tagged++}}return tagged}
