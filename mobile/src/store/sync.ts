import AsyncStorage from '@react-native-async-storage/async-storage';
import Constants from 'expo-constants';
import * as SecureStore from 'expo-secure-store';
import {AppState as RNAppState,Platform} from 'react-native';
import {cancelStateWrite,flushState,getState,onLocalWrite,replaceState,showToast} from './index';
import {diff,attendeeOps,enqueue,merge,mapRecord,field,type Op,type Pull} from './sync-logic';
import type {Account,AppState} from '../data/types';
import type {MergePreview,MergeResult} from '../domain/mergePreview';
import {l2} from '../i18n';
import {uploadPendingPhotos} from '../lib/photos';
import {registerDevice,unregisterDevice} from '../lib/notifications';
export const apiUrl=process.env.EXPO_PUBLIC_API_URL || Constants.expoConfig?.extra?.apiUrl || 'https://axleo-api.saulxj.ggff.net/api/v1';
const tokenKey='ax_token', syncKey='ax_sync';
let token:string|null=null, cursor='', queue:Op[]=[], names:Record<string,string>={}, lastSync='';
let running=false, timer:ReturnType<typeof setTimeout>|undefined, delay=2000;
let inFlight=new Set<string>();
const save=()=>AsyncStorage.setItem(syncKey,JSON.stringify({cursor,queue,lastSync})).catch(()=>{});
const tokenGet=()=>Platform.OS==='web'?Promise.resolve(localStorage.getItem(tokenKey)):SecureStore.getItemAsync(tokenKey);
const tokenSet=(v:string|null)=>Platform.OS==='web'?Promise.resolve(v?localStorage.setItem(tokenKey,v):localStorage.removeItem(tokenKey)):v?SecureStore.setItemAsync(tokenKey,v):SecureStore.deleteItemAsync(tokenKey);
export const isOnline=()=>!!token;
export const authHeader=()=>token ? {Authorization:`Bearer ${token}`} : null;
export const syncInfo=()=>({lastSync,pending:queue.length});
export const formatLastSync=(value:string)=>{const d=new Date(value);return value&&!Number.isNaN(d.getTime())?d.toLocaleTimeString('en-AU',{hour:'2-digit',minute:'2-digit'}):'—'};
/** Team member id for a display name (from the last /team fetch); undefined offline. */
/** Team from the last sync (id+name), so pickers render instantly. */
export const teamList=()=>Object.entries(names).map(([id,name])=>({id,name}));
export const teamIdOf=(name:string)=>Object.entries(names).find(([,n])=>n===name)?.[0];
const label=(op:Op,status='waiting',err='')=>({id:op.opId,d:new Date().toISOString(),what:`${op.kind} · ${op.id}`,status,tries:0,err});
function reflect(errors:AppState['syncq']=[]){const s=getState();replaceState({...s,syncq:[...queue.map(op=>label(op)),...errors],queue:queue.length});}
const uuid=()=>`${Date.now()}-${Math.random().toString(36).slice(2)}`;
onLocalWrite((before,after)=>{if(!token)return;const changes=[...diff(before,after,uuid),...attendeeOps(before,after,uuid)];for(const op of changes)if(op.kind==='listings'&&op.data)op.data={...op.data,ofi:(op.data.ofi||[]).map((e:any)=>({...e,attendees:[]}))};if(!changes.length)return;for(const op of changes){const sending=queue.find(x=>x.kind===op.kind&&x.id===op.id&&inFlight.has(x.opId));if(sending){const pending=queue.filter(x=>!inFlight.has(x.opId));queue=[...queue.filter(x=>inFlight.has(x.opId)),...enqueue(pending,[{...op,baseVersion:pending.find(x=>x.kind===op.kind&&x.id===op.id)?.baseVersion??sending.baseVersion}])];}else queue=enqueue(queue,[op]);}void save();reflect(after.syncq.filter(q=>q.status==='error'));schedule();});
function schedule(ms=2000){clearTimeout(timer);timer=setTimeout(()=>void syncNow(),ms);}
async function api(path:string,options:RequestInit={}){const response=await fetch(`${apiUrl}${path}`,{...options,headers:{'Content-Type':'application/json',Authorization:`Bearer ${token}`,...options.headers}});if(!response.ok)throw new Error(`HTTP ${response.status}`);return response.json();}
export async function restoreSync(){token=await tokenGet();if(!token){if(getState().auth && getState().auth?.email.endsWith('@axleo.local'))replaceState({...getState(),auth:null});return;}try{const me=await api('/me');const raw=await AsyncStorage.getItem(syncKey);const saved=raw&&JSON.parse(raw);cursor=saved?.cursor||'';queue=saved?.queue||[];lastSync=saved?.lastSync||'';replaceState({...getState(),auth:account(me)});void registerDevice(token!,apiUrl);reflect(getState().syncq.filter(q=>q.status==='error'));schedule(0);}catch{ // Keep offline cache and token for retry; revoked sessions require a fresh login.
 schedule(60000);}}
function account(user:{id:string;name:string;email:string;role:string;company?:string}):Account{return {id:user.id,name:user.name,email:user.email,role:user.role,company:user.company||'',team:'Axleo',rex:'Not connected',since:new Date().toLocaleString(),device:'This device'};}
export async function login(email:string,password:string){const response=await fetch(`${apiUrl}/auth/login`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({email,password,device:'AxleoCollect'})});if(!response.ok)throw new Error(response.status===401?'credentials':'network');const result=await response.json();token=result.token;await tokenSet(token);cursor='';queue=[];lastSync='';await save();const s=getState();replaceState({...s,auth:account(result.user),contacts:[],properties:[],todos:[],captures:[],enquiries:[],leads:[],listings:[],ofi_attendees:[],sales:[],contracts:[],referrals:[],market:[],zones:[],myTags:[],audit:[],approvals:[],syncq:[],queue:0});await syncNow();void registerDevice(token!,apiUrl);}
/** Contact merge: no approval step. The server computes the preview; executing it with a changed preview returns 409 and a fresh preview. */
export async function contactMergePreview(aId:string,bId:string):Promise<MergePreview>{return api('/contacts/merge-preview',{method:'POST',body:JSON.stringify({aId,bId})});}
export async function contactMerge(primaryId:string,secondaryId:string,previewHash:string):Promise<MergeResult>{const response=await fetch(`${apiUrl}/contacts/${encodeURIComponent(primaryId)}/merge`,{method:'POST',headers:{'Content-Type':'application/json',Authorization:`Bearer ${token}`},body:JSON.stringify({secondaryId,previewHash})});if(response.status===409){const body=await response.json().catch(()=>({}));if(body?.preview)return {stale:body.preview};}if(!response.ok)throw new Error(`HTTP ${response.status}`);const out=await response.json();await syncNow();return out;}
export async function decideApproval(id:string,decision:'approve'|'reject'){await api(`/approvals/${encodeURIComponent(id)}/decide`,{method:'POST',body:JSON.stringify({decision})});await syncNow();}
export async function propertiesWithin(polygon:number[][],limit=100,offset=0):Promise<{items:import('../domain/patch').AreaProperty[];total:number;linked:number}>{return api('/properties/within',{method:'POST',body:JSON.stringify({polygon,limit,offset})});}
/** Office + Rex contact search; only called when the phone has no local results. */
export async function searchContactsServer(q:string,signal?:AbortSignal):Promise<import('../lib/contactSearch').ServerSearch>{return api(`/contacts/search?q=${encodeURIComponent(q)}`,{signal});}
/** Imports a Rex contact (owned by the caller), then syncs so it is on the phone. */
export async function importRexContact(rexId:string):Promise<{imported:boolean;item:import('../lib/contactSearch').ServerContact}>{const r=await api('/contacts/import-rex',{method:'POST',body:JSON.stringify({rexId})});await syncNow();return r;}
export async function teamMembers():Promise<{id:string;name:string}[]>{return api('/team');}
/** Company mailing-list catalogue (server cache of Rex AdminMailingLists), kept in state for offline use; at most every 10 minutes. */
let catalogAt=0;export async function refreshMailingCatalog(force=false){if(!token||(!force&&Date.now()-catalogAt<600000))return;catalogAt=Date.now();try{const r=await api('/mailing-lists') as {items?:{name:string;channel?:string}[];at?:string};replaceState({...getState(),mailingCatalog:{items:r.items||[],at:r.at||''}});}catch{catalogAt=0;}}
export async function directAction(path:string,data:Record<string,unknown>){await api(path,{method:'POST',body:JSON.stringify(data)});await syncNow();}
export async function logout(){const old=token;token=null;clearTimeout(timer);cancelStateWrite();cursor='';queue=[];lastSync='';names={};await tokenSet(null);await AsyncStorage.removeItem(syncKey);const s=getState();replaceState({...s,auth:null,contacts:[],properties:[],todos:[],captures:[],enquiries:[],leads:[],listings:[],ofi_attendees:[],sales:[],contracts:[],referrals:[],market:[],zones:[],myTags:[],audit:[],approvals:[],syncq:[],queue:0});flushState();if(old){await unregisterDevice(old,apiUrl);void fetch(`${apiUrl}/auth/logout`,{method:'POST',headers:{Authorization:`Bearer ${old}`}}).catch(()=>{});}}
export async function syncNow(){if(!token||running)return;running=true;let errors=getState().syncq.filter(q=>q.status==='error');try{
 try{await uploadPendingPhotos()}catch{/* Photo retries must not block record sync. */}
 const team=await api('/team') as {id:string;name:string}[];names=Object.fromEntries(team.map(u=>[u.id,u.name]));
 if(queue.length){const batch=[...queue];inFlight=new Set(batch.map(op=>op.opId));const result=await api('/sync/push',{method:'POST',body:JSON.stringify({ops:batch})}) as {results:{opId:string;status:string;record?:any}[]};
 for(const item of result.results){const op=batch.find(o=>o.opId===item.opId);if(!op)continue;const idx=queue.findIndex(o=>o.opId===item.opId);if(idx<0)continue;queue.splice(idx,1);
 if(item.status==='conflict'){const s=getState();const arr=(s[field(op.kind)] as any[]).filter(x=>x.id!==op.id);if(item.record)arr.push(mapRecord(item.record,op.kind,names,s.auth?.name||'') as never);replaceState({...s,[field(op.kind)]:arr});showToast(l2('Changed on another device. Updated to the latest version.','已被其他设备修改，已更新为最新内容'));}
 if(item.status==='notfound'||item.status==='invalid')errors=[...errors,label(op,'error',item.status)];
 if(item.status==='ok'&&item.record){const later=queue.find(x=>x.kind===op.kind&&x.id===op.id);if(later)later.baseVersion=item.record.version;else {const s=getState();replaceState({...s,[field(op.kind)]:[...(s[field(op.kind)] as any[]).filter(x=>x.id!==op.id),mapRecord(item.record,op.kind,names,s.auth?.name||'')]});}}
 }await save();reflect(errors);}
 const pull=await api(`/sync${cursor?`?since=${encodeURIComponent(cursor)}`:''}`) as Pull;
 const current=getState();const merged=merge(current,pull,names,queue);
 const changed=Object.keys(merged).some(key=>merged[key as keyof AppState]!==current[key as keyof AppState]);
 if(changed || current.queue!==queue.length || current.syncq.length!==queue.length+errors.length)
   replaceState({...merged,syncq:[...queue.map(op=>label(op)),...errors],queue:queue.length});
 cursor=pull.cursor;lastSync=new Date().toISOString();await save();delay=2000;
 }catch{queue=enqueue([],queue);void save();delay=Math.min(Math.max(delay*2,4000),60000);schedule(delay);}finally{inFlight.clear();running=false;if(queue.length&&delay===2000)schedule();}}
RNAppState.addEventListener('change',v=>{if(v==='active'&&token)schedule(0)});
setInterval(()=>{if(token)void syncNow()},60000);
