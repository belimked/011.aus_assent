// Registered owner names (UAT 3 item 6). The server never syncs them: a property
// only carries market.hasOwners, and the name is read on demand from
// GET /properties/{id}/owner-names, which logs the view. Names live only in this
// module's memory (never in the persisted store or a sync push) and are dropped
// on logout/login (clearOwnerNames) or after OWNER_NAMES_TTL_MS.
export type OwnerNames={owners:string;absenteeOwner?:boolean;source?:string;updatedAt?:string;notice?:string};
export type OwnerNamesView={status:'loading'}|{status:'ok';data:OwnerNames}|{status:'unavailable'};
export type OwnerNamesFetch=(propertyId:string)=>Promise<unknown>;

export const OWNER_NAMES_TTL_MS=10*60*1000;
const cache=new Map<string,{data:OwnerNames;at:number}>();
const inFlight=new Map<string,Promise<OwnerNames>>();

export function clearOwnerNames(){cache.clear();inFlight.clear();}

/** A fresh cached result, or undefined. */
export function cachedOwnerNames(propertyId:string,now=Date.now()):OwnerNames|undefined{
 const hit=cache.get(propertyId);if(!hit)return undefined;
 if(now-hit.at>OWNER_NAMES_TTL_MS){cache.delete(propertyId);return undefined;}
 return hit.data;
}

function parse(raw:unknown):OwnerNames{
 const r=(raw&&typeof raw==='object'?raw:{}) as Record<string,unknown>;
 return {owners:typeof r.owners==='string'?r.owners.trim():'',absenteeOwner:r.absenteeOwner===true,source:typeof r.source==='string'?r.source:'',updatedAt:typeof r.updatedAt==='string'?r.updatedAt:'',notice:typeof r.notice==='string'?r.notice:''};
}

/** Cached names, else one request per property at a time. Failures reject and are not cached. */
export function loadOwnerNames(propertyId:string,fetcher:OwnerNamesFetch,now=Date.now()):Promise<OwnerNames>{
 const hit=cachedOwnerNames(propertyId,now);if(hit)return Promise.resolve(hit);
 const pending=inFlight.get(propertyId);if(pending)return pending;
 const p=fetcher(propertyId).then(raw=>{const data=parse(raw);if(inFlight.get(propertyId)===p)cache.set(propertyId,{data,at:Date.now()});return data;}).finally(()=>{if(inFlight.get(propertyId)===p)inFlight.delete(propertyId);});
 inFlight.set(propertyId,p);return p;
}

/** The view state for a property page: names are only requested when the server says they exist. */
export async function ownerNamesView(propertyId:string,hasOwners:boolean|undefined,fetcher:OwnerNamesFetch):Promise<OwnerNamesView|null>{
 if(!hasOwners)return null;
 try{return {status:'ok',data:await loadOwnerNames(propertyId,fetcher)};}catch{return {status:'unavailable'};}
}
