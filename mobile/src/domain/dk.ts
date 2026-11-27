import type {AppState, Contact, Property} from '../data/types';
import {today} from '../lib/today';
import {addTodo, audit, byId, linkOwner, matchContacts, normMobile, noteOn} from './index';
import {hasContactName, requireNewContactName} from './contactName';

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
  noteOn(s,c,`Preferences · ${label} ${val?'opted in':'opted out'}${reason?' · '+reason:''}`,{sys:'preference'});audit(s,'Preferences',`${c.first} ${c.last}`.trim()+` · ${label}=${val}`);
}
/** Stable note id: the server keys its one-time Rex Notes::create job on it. */
export const propertyNoteId=()=>'pn'+Date.now().toString(36)+Math.random().toString(36).slice(2,10);
export function filePropertyNote(s:AppState,propId:string,txt:string){
  const p=byId(s.properties,propId);if(!p)throw new Error('Property not found');
  p.notes=[{id:propertyNoteId(),d:today(),at:new Date().toISOString(),a:s.auth?.name||'Owen Chen',t:txt},...(p.notes||[])];
  p.visits=[...(p.visits||[]),{d:today(),o:'spoke'}];audit(s,'Note',`${p.no} ${p.street}`);return p;
}
/** True when commitDK would create a new contact (no usable pick, no unique match). Multiple matches without a pick fail in commitDK itself. */
export function commitDKCreates(s:AppState,r:{mobile:string;email:string},pick?:string){
  if(pick)return pick==='new'||!byId(s.contacts,pick);
  return matchContacts(s,r).status==='none';
}
/** True when commitDK would create a contact with neither first nor last name. */
export const dkNeedsName=(s:AppState,r:Pick<DKFields,'first'|'last'|'mobile'|'email'>,pick?:string)=>commitDKCreates(s,r,pick)&&!hasContactName(r);
export function commitDK(s:AppState, propId:string, r:DKFields, pick?:string) {
  const p=byId(s.properties,propId);
  const matches=matchContacts(s,r);
  if(matches.status==='multiple'&&!pick) throw new Error('Choose a matching contact before approving');
  let c=pick==='new'?undefined:pick?byId(s.contacts,pick):matches.status==='unique'?matches.list[0].c:undefined;
  if(!c){
    requireNewContactName(r);
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
  if(r.note)noteOn(s,c,r.note,{propId:p?.id});
  if(r.voiceAudit) c.voiceAudit=[...(c.voiceAudit||[]),{...r.voiceAudit,d:today()}];
  if(p&&['owner','landlord'].includes(r.relation))linkOwner(s,c,p);
  if(p)p.visits=[...(p.visits||[]),{d:today(),o:'spoke'}];
  if(r.followup)addTodo(s,{t:`Follow up with ${c.first} ${c.last}`.trim(),d:r.followup,cid:c.id,propId:p?.id});
  if(/market update/i.test(r.note))addTodo(s,{t:`Send market update to ${c.first} ${c.last}`.trim(),d:today(),cid:c.id,propId:p?.id});
  audit(s,'DK',`${c.first} ${c.last} · ${p?.street||'new contact'}`);
  return c;
}
export type IdentityField='first'|'last'|'mobile'|'email';
export const IDENTITY_FIELDS:IdentityField[]=['first','last','mobile','email'];
export type IdentityReplace=Partial<Record<IdentityField,string>>;
/**
 * replace: identity fields the user explicitly chose to overwrite on an existing contact (contactId, a picked or a
 * unique match). Only these are written; fields.first/last/mobile/email never overwrite an existing contact.
 */
export type ConfirmSave = {propId?:string;contactId?:string;pick?:string;fields:DKFields;replace?:IdentityReplace};
export type IdentitySuggestion={field:IdentityField;from:string;to:string;unsure:boolean};
const sameIdentity=(f:IdentityField,a:string,b:string)=>f==='mobile'?normMobile(a)===normMobile(b):(a||'').trim().toLowerCase()===(b||'').trim().toLowerCase();
/**
 * Extracted identity values that differ from the existing contact (mobile compared with normMobile, name/email
 * ignoring case and surrounding space). Empty extracted values are never suggested; an empty current value is.
 */
export function identitySuggestions(c:Pick<Contact,IdentityField>,extracted:Partial<Record<IdentityField,string>>,unsure:string[]=[]):IdentitySuggestion[]{
  return IDENTITY_FIELDS.flatMap(field=>{
    const to=(extracted[field]||'').trim();const from=c[field]||'';
    if(!to||sameIdentity(field,from,to))return [];
    return [{field,from,to,unsure:unsure.includes(field)||((field==='first'||field==='last')&&unsure.includes('name'))}];
  });
}
/** The replace map for the suggestions the user switched to Replace. */
export const chosenReplace=(suggestions:IdentitySuggestion[],chosen:string[]):IdentityReplace=>Object.fromEntries(suggestions.filter(x=>chosen.includes(x.field)).map(x=>[x.field,x.to]));
/** The existing contact the confirm sheet will update, or undefined when it creates one (or must ask to choose). */
export function confirmTarget(s:AppState,a:{contactId?:string;pick?:string;fields:{mobile:string;email:string}}):Contact|undefined{
  if(a.contactId)return byId(s.contacts,a.contactId);
  if(a.pick)return a.pick==='new'?undefined:byId(s.contacts,a.pick);
  const m=matchContacts(s,a.fields);return m.status==='unique'?m.list[0].c:undefined;
}
/** Fields for commitDK on an existing contact: identity comes only from replace. */
const keepIdentity=(f:DKFields,replace:IdentityReplace={}):DKFields=>({...f,first:replace.first||'',last:replace.last||'',mobile:replace.mobile||'',email:replace.email||''});
const CONSENT_KEYS=['email','sms','phone','letter'];
/** Consent shown on the confirm sheet: a DNC tag turns everything off, otherwise the contact's current value (default on). */
export function confirmConsent(c:Contact|undefined,tags:string[]){return Object.fromEntries(CONSENT_KEYS.map(k=>[k,tags.includes('Do not contact')?0:c?.consent?.[k]??1]))}
/** True when saving the confirm sheet would create a contact with neither first nor last name (Rex rejects those). */
export function confirmNeedsName(s:AppState,a:ConfirmSave){
  if(a.contactId)return false;
  return dkNeedsName(s,a.fields,a.pick);
}
/**
 * Save of the DK "Check before saving" sheet.
 * With contactId (Update record on a contact page) that contact is the target: no new contact, and no property's
 * ownerIds change unless the relation was explicitly set to owner/landlord; the note is a plain contact note.
 * On any existing contact, name/mobile/email change only via a.replace.
 */
export function saveConfirm(s:AppState,a:ConfirmSave){
  if(a.contactId){
    const c=byId(s.contacts,a.contactId);if(!c)throw new Error('Contact not found');
    const link=['owner','landlord'].includes(a.fields.relation);
    let consent:Record<string,number>|undefined;
    if(a.fields.consent){const now=confirmConsent(c,[]);const diff=Object.entries(a.fields.consent).filter(([k,v])=>now[k]!==v);consent=diff.length?Object.fromEntries(diff):undefined}
    const out=commitDK(s,link?a.propId||'':'',{...keepIdentity(a.fields,a.replace),consent},c.id);
    // Same DNC sync as the contact page's tag field, without the door-knock visit side effects.
    const owned=s.properties.find(p=>p.ownerIds.includes(c.id));if(owned&&!link&&a.fields.tags.length)owned.flags.dnc=!!c.flags.dnc;
    return out;
  }
  // A picked or uniquely matched contact: the typed mobile/email did the matching but are not written back.
  const target=confirmTarget(s,a);
  if(target)return commitDK(s,a.propId||'',keepIdentity(a.fields,a.replace),target.id);
  return commitDK(s,a.propId||'',a.fields,a.pick||undefined);
}
// possibleDuplicateOf is set by the server when an app contact and a Rex contact share a mobile (问题02).
export function duplicatesOf(s:AppState,c:Contact){const flagged=(x:Contact)=>(c.possibleDuplicateOf||[]).includes(x.id)||(x.possibleDuplicateOf||[]).includes(c.id);return s.contacts.filter(x=>x.id!==c.id&&((!!c.mobile&&normMobile(c.mobile)===normMobile(x.mobile))||!!c.email&&c.email.toLowerCase()===(x.email||'').toLowerCase()||flagged(x)));}
