import type {AppState} from '../data/types';

/** Suggest a name for a finished polygon; ties follow property order. */
export function suggestAreaName(s:AppState,pts:number[][]):string {
 const inside=propertiesInArea(s,pts);
 const counts=new Map<string,number>();
 for(const p of inside)if(p.suburb)counts.set(p.suburb,(counts.get(p.suburb)||0)+1);
 let suburb=[...counts].sort((a,b)=>b[1]-a[1])[0]?.[0];
 if(!suburb&&pts.length){
  let twiceArea=0,cx=0,cy=0;
  for(let i=0;i<pts.length;i++){const a=pts[i],b=pts[(i+1)%pts.length],cross=a[0]*b[1]-b[0]*a[1];twiceArea+=cross;cx+=(a[0]+b[0])*cross;cy+=(a[1]+b[1])*cross}
  const center=twiceArea? [cx/(3*twiceArea),cy/(3*twiceArea)] : [pts.reduce((v,p)=>v+p[0],0)/pts.length,pts.reduce((v,p)=>v+p[1],0)/pts.length];
  suburb=s.properties.filter(p=>p.ll&&p.suburb).sort((a,b)=>{const dist=(p:typeof a)=>Math.pow(p.ll![0]-center[0],2)+Math.pow(p.ll![1]-center[1],2);return dist(a)-dist(b)})[0]?.suburb;
 }
 suburb=suburb||'Patch';
 return `${suburb} ${1+s.zones.filter(z=>z.name.startsWith(suburb)).length}`;
}

export function pointInPolygon(p:number[],poly:number[][]){let inside=false;for(let i=0,j=poly.length-1;i<poly.length;j=i++){const a=poly[i],b=poly[j];if((a[1]>p[1])!==(b[1]>p[1])&&p[0]<(b[0]-a[0])*(p[1]-a[1])/(b[1]-a[1])+a[0])inside=!inside}return inside}
export function propertiesInArea(s:AppState,pts:number[][]){return s.properties.filter(p=>p.ll&&pointInPolygon(p.ll,pts))}
export function tagArea(s:AppState,name:string,pts:number[][]){const inside=propertiesInArea(s,pts);let tagged=0;for(const p of inside)for(const id of p.ownerIds){const c=s.contacts.find(x=>x.id===id);if(c&&!c.tags.includes('Area: '+name)){c.tags.push('Area: '+name);tagged++}}return tagged}
