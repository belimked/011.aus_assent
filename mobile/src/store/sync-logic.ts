import type {AppState} from '../data/types';
export const kinds=['contacts','properties','todos','captures','enquiries','leads','listings','ofi_attendees','sales','contracts','referrals','market_items','zones','tags'] as const;
export type Kind=typeof kinds[number];
export const field=(kind:Kind):keyof AppState=>kind==='market_items'?'market':kind==='tags'?'myTags':kind;
export type RecordData={id:string;version?:number;[key:string]:any};
export type Op={opId:string;kind:Kind;op:'upsert'|'delete';id:string;baseVersion?:number;data?:RecordData};
export type Pull={cursor:string;full:boolean;deleted:Record<string,string[]>;[key:string]:any};
export function diff(before:AppState,after:AppState,makeId:()=>string):Op[]{
 const ops:Op[]=[];
 for(const kind of kinds){const key=field(kind);const old=new Map((before[key] as RecordData[]).map(x=>[x.id,x]));const next=new Map((after[key] as RecordData[]).map(x=>[x.id,x]));
 for(const [id,item] of next){const previous=old.get(id);const clean=(v:RecordData)=>kind==='listings'?{...v,ofi:(v.ofi||[]).map((event:any)=>({...event,attendees:[]}))}:v;const data=clean(item);if(!previous || JSON.stringify(clean(previous))!==JSON.stringify(data))ops.push({opId:makeId(),kind,op:'upsert',id,baseVersion:previous?.version,data});}
 for(const [id,item] of old)if(!next.has(id))ops.push({opId:makeId(),kind,op:'delete',id,baseVersion:item.version});
 }return ops;
}
export function attendeeOps(before:AppState,after:AppState,makeId:()=>string):Op[]{
 const flattened=(s:AppState)=>s.listings.flatMap(l=>(l.ofi||[]).flatMap((event:any,index:number)=>(event.attendees||[]).map((a:any,i:number)=>({...a,id:a.id||`ofi-${l.id}-${index}-${a.contactId||i}`,listingId:l.id,eventIndex:index}))));
 const old=new Map(flattened(before).map(a=>[a.id,a]));const next=new Map(flattened(after).map(a=>[a.id,a]));const ops:Op[]=[];
 for(const [id,a] of next)if(JSON.stringify(a)!==JSON.stringify(old.get(id)))ops.push({kind:'ofi_attendees',op:'upsert',opId:makeId(),id,baseVersion:old.get(id)?.version,data:a});
 for(const [id,a] of old)if(!next.has(id))ops.push({kind:'ofi_attendees',op:'delete',opId:makeId(),id,baseVersion:a.version});return ops;
}
export function enqueue(queue:Op[],changes:Op[]):Op[]{const out=[...queue];for(const op of changes){const index=out.findIndex(x=>x.kind===op.kind&&x.id===op.id);if(index<0)out.push(op);else {const previous=out[index];if(!previous.baseVersion&&op.op==='delete')out.splice(index,1);else out[index]={...op,opId:previous.opId,baseVersion:previous.baseVersion};}}return out;}
export function mapRecord(record:RecordData,kind:Kind,names:Record<string,string>,me:string):RecordData{
 const r={...record};delete r.updated_at;delete r.deleted_at;
 if(kind==='contacts')r.owner=names[r.ownerId]||r.owner||me;
 if(kind==='todos'){r.who=names[r.assigneeId]||r.who||me;r.by=names[r.createdBy]||r.by||me;}
 if(kind==='listings')r.agent=names[r.ownerId]||r.agent||me;
 return r;
}
export function merge(state:AppState,pull:Pull,names:Record<string,string>,pending:Op[]):AppState{
 const next={...state};for(const kind of kinds){const key=field(kind);const blocked=new Set(pending.filter(x=>x.kind===kind).map(x=>x.id));const records=(pull[kind]||[]).map((r:RecordData)=>mapRecord(r,kind,names,state.auth?.name||''));const existing=pull.full?[]:state[key] as RecordData[];const mapped=new Map<string,RecordData>(existing.map(x=>[x.id,x]));for(const id of pull.deleted?.[kind]||[])if(!blocked.has(id))mapped.delete(id);for(const r of records)if(!blocked.has(r.id))mapped.set(r.id,r);(next as any)[key]=[...mapped.values()];}
 next.approvals=pull.full?pull.approvals||[]:[...new Map([...state.approvals,...(pull.approvals||[])].map(x=>[x.id,x])).values()].filter(x=>!pull.deleted?.approvals?.includes(x.id));
 // Attendee PII is kept separately; only attach records visible to this agent.
 const attendees=next.ofi_attendees;next.listings=next.listings.map(listing=>({...listing,ofi:(listing.ofi||[]).map((event:any,i:number)=>({...event,attendees:attendees.filter(a=>a.listingId===listing.id&&a.eventIndex===i)}))}));
 return next;
}
