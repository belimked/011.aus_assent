import type {AppState, Contact} from '../data/types';
import {TODAY} from '../data/seed';
import {addTodo, audit, byId, linkOwner, matchContacts, normMobile, noteOn} from './index';

export type DKFields = {first:string;last:string;mobile:string;email:string;relation:string;note:string;reason:string;tags:string[];followup?:string;linked?:string};
export function commitDK(s:AppState, propId:string, r:DKFields, pick?:string) {
  const p=byId(s.properties,propId);
  const matches=matchContacts(s,r);
  if(matches.status==='multiple'&&!pick) throw new Error('Choose a matching contact before approving');
  let c=pick?byId(s.contacts,pick):matches.status==='unique'?matches.list[0].c:undefined;
  if(!c){
    c={id:'c'+Date.now(),first:r.first,last:r.last,mobile:r.mobile,email:r.email,owner:s.auth?.name||'Owen Chen',roles:[],tags:[],flags:{},notes:[],rels:[],updated:{by:s.auth?.name||'Owen Chen',d:TODAY}};
    s.contacts.push(c);
  } else {
    if(r.first)c.first=r.first;
    if(r.last)c.last=r.last;
    if(r.mobile)c.mobile=r.mobile;
    if(r.email)c.email=r.email;
  }
  if(r.relation&&!c.roles.includes(r.relation))c.roles.push(r.relation);
  c.tags=[...new Set([...c.tags,...r.tags])];
  if(r.reason)c.reason=r.reason;
  if(c.tags.includes('Do not contact')){c.flags.dnc=true;c.consent={email:0,sms:0,phone:0,letter:0};}
  if(c.tags.includes('No junk mail'))c.consent={...c.consent,junk:0};
  if(r.linked){c.rels=c.rels||[];c.rels.push({type:'Partner of',name:r.linked});}
  if(r.note)noteOn(s,c,r.note);
  if(p&&r.relation==='owner')linkOwner(s,c,p);
  if(r.followup)addTodo(s,{t:`Follow up with ${c.first} ${c.last}`.trim(),d:r.followup,cid:c.id,propId:p?.id});
  if(/market update/i.test(r.note))addTodo(s,{t:`Send market update to ${c.first} ${c.last}`.trim(),d:TODAY,cid:c.id,propId:p?.id});
  audit(s,'DK',`${c.first} ${c.last} · ${p?.street||'new contact'}`);
  return c;
}
export function duplicatesOf(s:AppState,c:Contact){return s.contacts.filter(x=>x.id!==c.id&&x.owner===c.owner&&((!!c.mobile&&normMobile(c.mobile)===normMobile(x.mobile))||!!c.email&&c.email.toLowerCase()===x.email.toLowerCase()));}
export function mergeContacts(s:AppState,aId:string,bId:string,fields:Record<string,'a'|'b'>={}){
  const a=byId(s.contacts,aId),b=byId(s.contacts,bId);
  if(!a||!b||a.id===b.id||a.owner!==b.owner)throw new Error('Contacts cannot be merged');
  for(const k of ['first','last','mobile','email','owner','reason']){const v=(fields[k]==='b'?b:a)[k]||a[k]||b[k];if(v)a[k]=v;}
  a.tags=[...new Set([...a.tags,...b.tags])];a.roles=[...new Set([...a.roles,...b.roles])];
  a.notes=[...a.notes,...b.notes].sort((x,y)=>y.d.localeCompare(x.d));
  a.rels=[...(a.rels||[]),...(b.rels||[])];
  for(const p of s.properties)p.ownerIds=[...new Set(p.ownerIds.map(id=>id===b.id?a.id:id))];
  for(const t of s.todos)if(t.cid===b.id)t.cid=a.id;
  s.contacts=s.contacts.filter(x=>x.id!==b.id);
  audit(s,'Merge',`${a.first} ${a.last}`);
  return a;
}
