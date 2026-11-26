import {normMobile} from '../domain';

/** Server search result (GET /contacts/search). Limited items only say that a contact exists and who owns it. */
export type ServerContact={source:'local'|'rex';id?:string;rexId?:string;full:boolean;name:string;first?:string;last?:string;mobile?:string;email?:string;mobileMasked?:string;ownerName?:string;ownerId?:string;inRex?:boolean;mine?:boolean};
export type ServerSearch={items:ServerContact[];rexTimedOut?:boolean;rexSkipped?:boolean;rexError?:boolean;rexReason?:string};
export type SearchPlan='idle'|'local'|'offline'|'short'|'server';

export const SEARCH_DEBOUNCE_MS=500;

/** Mirrors the server rule: ≥2 characters; a number needs ≥3 digits. */
export function serverQueryOK(term:string){const q=term.trim();if([...q].length<2)return false;if(/^[0-9+()\-. ]+$/.test(q))return q.replace(/\D/g,'').length>=3;return true;}

/** Local results win: the office/Rex search runs only when the phone has nothing. */
export function searchPlan(term:string,localTotal:number,online:boolean):SearchPlan{
 if(!term.trim())return 'idle';
 if(localTotal>0)return 'local';
 if(!serverQueryOK(term))return 'short';
 return online?'server':'offline';
}

/** Drops results already on the phone (same local id, or a Rex id the phone already links). */
export function serverRows(resp:ServerSearch|null,phone:{id:string;rexId?:string}[]){
 if(!resp)return [];const ids=new Set(phone.map(c=>c.id));const rex=new Set(phone.map(c=>c.rexId).filter(Boolean));
 return resp.items.filter(x=>x.source==='rex'?!rex.has(x.rexId):!ids.has(x.id||''));
}

/** Subtitle for one server row. */
export function serverSubtitle(x:ServerContact,l2:(en:string,zh:string)=>string){
 if(x.source==='rex')return [l2('In Rex · tap to import','在 Rex 中 · 点按导入'),x.mobileMasked,x.ownerName].filter(Boolean).join(' · ');
 if(!x.full)return x.ownerName?l2(`Owned by ${x.ownerName}`,`归属：${x.ownerName}`):l2('Unassigned','未分配');
 return [x.mobile?maskShort(x.mobile):'',x.ownerName].filter(Boolean).join(' · ');
}
const maskShort=(m:string)=>{const d=normMobile(m);return d.length<8?m:d.slice(0,4)+' '+d.slice(4,7)+' ••••';};

/** One quiet line about Rex; never an error banner. */
export function rexNotice(resp:ServerSearch|null,l2:(en:string,zh:string)=>string){
 if(!resp)return '';
 if(resp.rexTimedOut)return l2("Rex didn't respond in time",'Rex 未及时响应');
 if(resp.rexError)return l2('Rex unavailable; showing office results','Rex 暂不可用，仅显示后台结果');
 if(resp.rexSkipped&&resp.rexReason==='rate limited')return l2('Rex search paused for a minute; showing office results','Rex 搜索稍后可用，仅显示后台结果');
 return '';
}
