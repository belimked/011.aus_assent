import type {AppState} from '../data/types';
export type Kind='contacts'|'properties'|'todos';
export const kinds:Kind[]=['contacts','properties','todos'];
export type RecordData={id:string;version?:number;[key:string]:any};
export type Op={opId:string;kind:Kind;op:'upsert'|'delete';id:string;baseVersion?:number;data?:RecordData};
export type Pull={cursor:string;full:boolean;contacts:RecordData[];properties:RecordData[];todos:RecordData[];deleted:Record<Kind,string[]>};
export function diff(before:AppState,after:AppState,makeId:()=>string):Op[]{
 const ops:Op[]=[];
 for(const kind of kinds){const old=new Map(before[kind].map(x=>[x.id,x]));const next=new Map(after[kind].map(x=>[x.id,x]));
 for(const [id,item] of next){const previous=old.get(id);if(!previous || JSON.stringify(previous)!==JSON.stringify(item))ops.push({opId:makeId(),kind,op:'upsert',id,baseVersion:(previous as RecordData|undefined)?.version,data:item});}
 for(const [id,item] of old)if(!next.has(id))ops.push({opId:makeId(),kind,op:'delete',id,baseVersion:(item as RecordData).version});
 }return ops;
}
export function enqueue(queue:Op[],changes:Op[]):Op[]{const out=[...queue];for(const op of changes){const index=out.findIndex(x=>x.kind===op.kind&&x.id===op.id);if(index<0)out.push(op);else {const previous=out[index];if(!previous.baseVersion&&op.op==='delete')out.splice(index,1);else out[index]={...op,opId:previous.opId,baseVersion:previous.baseVersion};}}return out;}
export function mapRecord(record:RecordData,kind:Kind,names:Record<string,string>,me:string):RecordData{
 const r={...record};delete r.updated_at;delete r.deleted_at;
 if(kind==='contacts')r.owner=names[r.ownerId]||r.owner||me;
 if(kind==='todos'){r.who=names[r.assigneeId]||r.who||me;r.by=names[r.createdBy]||r.by||me;}
 return r;
}
export function merge(state:AppState,pull:Pull,names:Record<string,string>,pending:Op[]):AppState{
 const next={...state};for(const kind of kinds){const blocked=new Set(pending.filter(x=>x.kind===kind).map(x=>x.id));const records=pull[kind].map(r=>mapRecord(r,kind,names,state.auth?.name||''));const existing=pull.full?[]:state[kind];const mapped=new Map<string,RecordData>(existing.map(x=>[x.id,x]));for(const id of pull.deleted[kind]||[])if(!blocked.has(id))mapped.delete(id);for(const r of records)if(!blocked.has(r.id))mapped.set(r.id,r);(next as any)[kind]=[...mapped.values()];}return next;
}
