import type {AppState} from '../data/types';
export function pointInPolygon(p:number[],poly:number[][]){let inside=false;for(let i=0,j=poly.length-1;i<poly.length;j=i++){const a=poly[i],b=poly[j];if((a[1]>p[1])!==(b[1]>p[1])&&p[0]<(b[0]-a[0])*(p[1]-a[1])/(b[1]-a[1])+a[0])inside=!inside}return inside}
export function propertiesInArea(s:AppState,pts:number[][]){return s.properties.filter(p=>p.ll&&pointInPolygon(p.ll,pts))}
export function tagArea(s:AppState,name:string,pts:number[][]){const inside=propertiesInArea(s,pts);let tagged=0;for(const p of inside)for(const id of p.ownerIds){const c=s.contacts.find(x=>x.id===id);if(c&&!c.tags.includes('Area: '+name)){c.tags.push('Area: '+name);tagged++}}return tagged}
