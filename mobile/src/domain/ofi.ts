import type {AppState, Contact, Listing, Property} from '../data/types';
import {today,addLocalDays} from '../lib/today';
import {addTodo, audit, byId, cname, matchContacts, normMobile, noteOn, propAddr, shortAddr} from './index';

export const REQS = [
  ['form2','r_form2','listing agent · today'], ['bp','r_bp','listing agent · today'],
  ['private','r_private','listing agent · 2 h'], ['broker','r_broker','finance team · next business day'],
  ['conv','r_conv','listing agent · today'], ['rental','r_rental','PM team · 2 business days'],
  ['guide','r_guide','listing agent · today'], ['dbyd','r_dbyd','listing agent · 2 business days'],
] as const;
export type Requests = Partial<Record<(typeof REQS)[number][0], boolean>>;
export type Registration = {first:string;last:string;mobile:string;email:string;address?:string;note?:string;req?:Requests;ps?:boolean;nb?:boolean;consent?:boolean;tags?:string[]};
const days = (n:number) => addLocalDays(new Date(),n);
export type OpenHomeStatus = 'later'|'live'|'finished';
export function openHomeStatus(date:string, time:string, now:Date):OpenHomeStatus {
  const [startText,endText] = time.split('–');
  const minutes = (value:string) => {const [hour,minute]=value.split(':').map(Number);return hour*60+minute};
  const start = new Date(`${date}T00:00:00`);
  start.setMinutes(minutes(startText));
  const end = new Date(`${date}T00:00:00`);
  end.setMinutes(minutes(endText)+(minutes(endText)<=minutes(startText)?1440:0));
  return now<start?'later':now>=end?'finished':'live';
}
const id = (prefix:string) => prefix+Date.now()+Math.random().toString(36).slice(2,7);
export function buyerMatches(state:AppState, mobile:string, email:string){
  const number=normMobile(mobile);
  if(number.length>=10){const found=matchContacts(state,{mobile});if(found.status!=='none')return found;}
  return matchContacts(state,{email});
}
export function ohRequests(state:AppState, l:Listing, c:Contact, p:Property, rq:Requests){
  const task=(d:string,t:string,who=l.agent,kind='call')=>addTodo(state,{d,t,who,kind,cid:c.id,propId:p.id});
  if(rq.form2){if(l.form2) noteOn(state,c,`Form 2 emailed ${today()}.`,{propId:p.id});else task(today(),`Send Form 2 to ${cname(c)}`);}
  if(rq.bp){if(l.bpReport) noteOn(state,c,'Building & pest report emailed.',{propId:p.id});else task(today(),`Send B&P report to ${cname(c)}`);}
  if(rq.private) task(today(),`Book private inspection · ${cname(c)} · ${shortAddr(p)} (within 2 h)`,l.agent,'appt');
  for(const type of ['broker','conv','rental'] as const) if(rq[type]){
    const partner=state.partners[type]?.[0]||l.agent;
    state.referrals.unshift({id:id('rf'),type,to:partner,contactId:c.id,propertyId:p.id,d:today(),status:'sent'});
    noteOn(state,c,`${type==='broker'?'Mortgage broker':type==='conv'?'Conveyancer':'Rental appraisal'} referral sent to ${partner}, ${today()} · consent recorded`,{propId:p.id});
    if(type==='broker') task(days(1),`Broker follow-up · ${cname(c)}`,partner);
    if(type==='rental') task(days(2),`Rental appraisal · ${cname(c)}`,partner,'appt');
  }
  if(rq.guide) noteOn(state,c,'Buyer guide emailed.',{propId:p.id});
  if(rq.dbyd) task(days(2),`Send Dial Before You Dig info · ${cname(c)}`);
}
export function commitOH(state:AppState, listingId:string, r:Registration, eventIndex?:number):Contact {
  const l=byId(state.listings,listingId);if(!l) throw new Error('Listing not found');
  const p=byId(state.properties,l.propertyId);if(!p) throw new Error('Property not found');
  const o=eventIndex===undefined?l.ofi.find(x=>x.d===today()):l.ofi[eventIndex];if(!o||o.d!==today()) throw new Error('No open home today');
  const matches=buyerMatches(state,r.mobile,r.email);
  // The prototype falls back to the first match when several contacts match.
  let c=matches.list[0]?.c;
  if(!c){c={id:id('c'),first:r.first,last:r.last,mobile:r.mobile,email:r.email,owner:state.auth?.name||'Owen Chen',roles:['buyer'],tags:[],flags:{},notes:[],history:[]};state.contacts.push(c);}
  else {c.first=r.first||c.first;c.last=r.last||c.last;c.mobile=r.mobile||c.mobile;c.email=r.email||c.email;}
  if(!c.roles.includes('buyer')) c.roles.push('buyer');
  c.history??=[];
  if(!c.history.some((h:{d:string;t:string})=>h.d===today()&&h.t.includes(p.no))) c.history.unshift({d:today(),t:'OFI '+propAddr(p)});
  for(const tag of r.tags||[]) if(!c.tags.includes(tag)) c.tags.push(tag);
  if(r.nb&&!c.tags.includes('Neighbour')) c.tags.push('Neighbour');
  if(r.nb===false) c.tags=c.tags.filter(x=>x!=='Neighbour');
  const existing=o.attendees.find((a:{contactId:string})=>a.contactId===c.id);
  if(!existing){o.attendees.push({contactId:c.id,d:today(),req:r.req||{},ps:!!r.ps,note:r.note||''});ohRequests(state,l,c,p,r.req||{});
    addTodo(state,{d:today(),time:'15:00',t:`Call for feedback · ${cname(c)} · ${shortAddr(p)}`,who:l.agent,kind:'call',cid:c.id,propId:p.id});}
  if(existing&&!state.todos.some(x=>x.cid===c.id&&x.t.startsWith('Call for feedback')&&x.propId===p.id)) addTodo(state,{d:today(),time:'15:00',t:`Call for feedback · ${cname(c)} · ${shortAddr(p)}`,who:l.agent,kind:'call',cid:c.id,propId:p.id});
  if(r.consent) c.consentRec={d:today(),how:'Ticked at open home check-in'};
  if(r.ps){if(!c.tags.includes('Seller lead')) c.tags.push('Seller lead');if(!existing) addTodo(state,{d:days(7),t:`Appraisal follow-up ${cname(c)}`,who:l.agent,kind:'hot',cid:c.id});
    if(r.address){const home=state.properties.find(x=>r.address!.toLowerCase().includes(`${x.no} ${x.street}`.toLowerCase()));if(home&&!home.ownerIds.includes(c.id)) home.ownerIds.push(c.id);}}
  if(r.note) noteOn(state,c,r.note,{propId:p.id});
  audit(state,'OFI',`${propAddr(p)} · ${cname(c)}`);
  return c;
}
export function saveAttendeeProfile(contact:Contact,fields:Record<string,unknown>){Object.assign(contact,fields)}
export function matchingListings(state:AppState){const me=state.auth?.name||'Owen Chen';return state.listings.filter(x=>x.status==='listed').sort((a,b)=>(a.agent===me?0:1)-(b.agent===me?0:1));}
export function saveBuyerProfile(state:AppState,contactId:string,fields:Record<string,unknown>,note=''){
  const c=byId(state.contacts,contactId);if(!c) throw new Error('Contact not found');
  c.buyerProfile={...(c.buyerProfile||{}),...fields};if(!c.roles.includes('buyer')) c.roles.push('buyer');
  if(note) noteOn(state,c,note);audit(state,'Buyer profile',cname(c));
}
