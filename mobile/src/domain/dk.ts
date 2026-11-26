import type {AppState, Contact, Property} from '../data/types';
import {today} from '../lib/today';
import {addTodo, audit, byId, linkOwner, matchContacts, normMobile, noteOn} from './index';

export const isLongHold=(durationMs:number)=>durationMs>=260;
export const matchesAddress=(address:string,query:string)=>address.toLowerCase().includes(query.trim().toLowerCase());
export const houseNumberOrder=(a:{no:string},b:{no:string})=>parseInt(a.no,10)-parseInt(b.no,10);
export type DKFields = {first:string;last:string;mobile:string;email:string;relation:string;note:string;reason:string;tags:string[];followup?:string;linked?:string;consent?:Record<string,number>;voiceAudit?:{raw:string;extracted:Record<string,unknown>}};
export function applyTags(s:AppState,c:Contact,p:Property|undefined,tags:string[]){
  c.tags=[...new Set(tags)];c.flags.dnc=c.tags.includes('Do not contact');
  if(p)p.flags.dnc=c.flags.dnc;
  if(c.flags.dnc)c.consent={...c.consent,email:0,sms:0,phone:0,letter:0};
  if(c.tags.includes('No junk mail'))c.consent={...c.consent,junk:0};
  const reason=c.tags.find(t=>['Downsizing','Upsizing','Moving for school','Relocating','Investor'].includes(t));if(reason)c.reason=reason;
  if(p&&c.tags.some(t=>['Not at home','Left card','Friendly','Not interested'].includes(t))){p.visits=[...(p.visits||[]),{d:today(),o:c.tags.includes('Not at home')?'not_home':c.tags.includes('Left card')?'left_card':c.tags.includes('Not interested')?'not_int':'spoke'}];}
  c.updated={by:s.auth?.name||'Owen Chen',d:today()};audit(s,'Tags',`${c.first} ${c.last}`.trim()+' · '+c.tags.join(', '));
}
export function applyPreference(s:AppState,c:Contact,k:string,val:number,reason=''){
  c.consent={...c.consent,[k]:val};c.optedOut=c.consent.sms===0;
  if(k==='junk')c.tags=val?c.tags.filter(t=>t!=='No junk mail'):[...new Set([...c.tags,'No junk mail'])];
  const label=({sms:'Text',phone:'Call',email:'Email',letter:'Letter',junk:'Junk mail'} as Record<string,string>)[k]||k;
  c.prefLog={d:today(),by:s.auth?.name||'Owen Chen',t:`${label}: ${val?'opted in':'opted out'}${reason?' · “'+reason+'”':''}`};
  noteOn(s,c,`Preferences · ${label} ${val?'opted in':'opted out'}${reason?' · '+reason:''}`);audit(s,'Preferences',`${c.first} ${c.last}`.trim()+` · ${label}=${val}`);
}
/** Stable note id: the server keys its one-time Rex Notes::create job on it. */
export const propertyNoteId=()=>'pn'+Date.now().toString(36)+Math.random().toString(36).slice(2,10);
export function filePropertyNote(s:AppState,propId:string,txt:string){
  const p=byId(s.properties,propId);if(!p)throw new Error('Property not found');
  p.notes=[{id:propertyNoteId(),d:today(),at:new Date().toISOString(),a:s.auth?.name||'Owen Chen',t:txt},...(p.notes||[])];
  p.visits=[...(p.visits||[]),{d:today(),o:'spoke'}];audit(s,'Note',`${p.no} ${p.street}`);return p;
}
export function commitDK(s:AppState, propId:string, r:DKFields, pick?:string) {
  const p=byId(s.properties,propId);
  const matches=matchContacts(s,r);
  if(matches.status==='multiple'&&!pick) throw new Error('Choose a matching contact before approving');
  let c=pick==='new'?undefined:pick?byId(s.contacts,pick):matches.status==='unique'?matches.list[0].c:undefined;
  if(!c){
    c={id:'c'+Date.now(),first:r.first,last:r.last,mobile:r.mobile,email:r.email,owner:s.auth?.name||'Owen Chen',roles:[],tags:[],flags:{},notes:[],rels:[],updated:{by:s.auth?.name||'Owen Chen',d:today()}};
    s.contacts.push(c);
  } else {
    if(r.first)c.first=r.first;
    if(r.last)c.last=r.last;
    if(r.mobile)c.mobile=r.mobile;
    if(r.email)c.email=r.email;
  }
  if(r.relation&&!c.roles.includes(r.relation))c.roles.push(r.relation);
  applyTags(s,c,p,[...c.tags,...r.tags]);
  if(r.reason)c.reason=r.reason;
  if(r.consent)c.consent={...c.consent,...r.consent};
  if(c.flags.dnc)c.consent={...c.consent,email:0,sms:0,phone:0,letter:0};
  c.optedOut=c.consent?.sms===0;
  if(r.consent)c.consentRec={d:today(),how:'At the door'};
  if(r.linked){c.rels=c.rels||[];c.rels.push({type:'Partner of',name:r.linked});}
  if(r.note)noteOn(s,c,r.note);
  if(r.voiceAudit) c.voiceAudit=[...(c.voiceAudit||[]),{...r.voiceAudit,d:today()}];
  if(p&&['owner','landlord'].includes(r.relation))linkOwner(s,c,p);
  if(p)p.visits=[...(p.visits||[]),{d:today(),o:'spoke'}];
  if(r.followup)addTodo(s,{t:`Follow up with ${c.first} ${c.last}`.trim(),d:r.followup,cid:c.id,propId:p?.id});
  if(/market update/i.test(r.note))addTodo(s,{t:`Send market update to ${c.first} ${c.last}`.trim(),d:today(),cid:c.id,propId:p?.id});
  audit(s,'DK',`${c.first} ${c.last} · ${p?.street||'new contact'}`);
  return c;
}
export function duplicatesOf(s:AppState,c:Contact){return s.contacts.filter(x=>x.id!==c.id&&((!!c.mobile&&normMobile(c.mobile)===normMobile(x.mobile))||!!c.email&&c.email.toLowerCase()===x.email.toLowerCase()));}
