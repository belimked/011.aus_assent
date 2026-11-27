import type {AppState,Contact} from '../data/types';
import {audit,cname} from './index';
import {today} from '../lib/today';

// Contact mailing lists (UAT 3 item 9). The server owns membership: a pull sets
// c.mailingLists (current names), c.mailingListsSeen (the same list, the baseline the
// server diffs a push against), c.mailingListSync (latest Rex job per lower-cased name)
// and c.rexLinked. The phone only edits c.mailingLists; offline edits queue like tags.
export type MailingList={name:string;channel?:string};
export type MailingChannel='email'|'sms'|'phone'|'letter';
export const MAILING_CHANNELS:MailingChannel[]=['email','sms','phone','letter'];
export const mailingKey=(name:string)=>name.trim().replace(/\s+/g,' ').toLowerCase();
export const contactLists=(c:Contact):string[]=>Array.isArray(c.mailingLists)?c.mailingLists:[];
/** Rex jobs need a linked Rex contact; older pulls without rexLinked fall back to the Rex import marker. */
export const mailingReady=(c:Contact)=>c.rexLinked===true||(c.rexLinked===undefined&&c.source==='rex_import');
/** Catalogue lists the contact is not on, matching the search text, by name. */
export function pickerOptions(catalog:MailingList[],joined:string[],query:string){const have=new Set(joined.map(mailingKey));const q=mailingKey(query);const seen=new Set<string>();return catalog.filter(l=>{const k=mailingKey(l.name||'');if(!k||have.has(k)||seen.has(k)||(q&&!k.includes(q)))return false;seen.add(k);return true}).sort((a,b)=>a.name.localeCompare(b.name));}
/** Status for one list: its latest job, else "In Rex" when the server already reported it, else waiting to sync. */
export function mailingSync(c:Contact,name:string){const k=mailingKey(name);const job=c.mailingListSync?.[k];if(job)return job;return (c.mailingListsSeen||[]).some((n:string)=>mailingKey(n)===k)?{status:'sent'}:{status:'pending'};}
// Do not contact is handled as elsewhere in the app (applyTags, merge): every channel is off.
// It is a warning, not a block: the app never hard-blocks edits on DNC contacts (tags, notes stay editable).
export const isDNC=(c:Contact)=>!!c.flags?.dnc||c.tags.includes('Do not contact');
export const offChannels=(c:Contact):MailingChannel[]=>isDNC(c)?[...MAILING_CHANNELS]:MAILING_CHANNELS.filter(k=>c.consent?.[k]===0);
/** null = add without asking. A known list channel only prompts when that channel is off; otherwise any off channel prompts, listing them. */
export function unsubscribePrompt(c:Contact,list:MailingList):{channels:MailingChannel[];dnc:boolean}|null{
  const off=offChannels(c);const dnc=isDNC(c);const channel=list.channel as MailingChannel;
  if(channel&&MAILING_CHANNELS.includes(channel))return off.includes(channel)?{channels:[channel],dnc}:null;
  return off.length?{channels:off,dnc}:null;
}
export function setMailingLists(s:AppState,c:Contact,names:string[],what:string){
  const seen=new Set<string>();c.mailingLists=names.filter(n=>{const k=mailingKey(n);if(!k||seen.has(k))return false;seen.add(k);return true});
  c.updated={by:s.auth?.name||'Owen Chen',d:today()};audit(s,'Mailing lists',`${cname(c)} · ${what}`);
}
export const addMailingList=(s:AppState,c:Contact,name:string)=>setMailingLists(s,c,[...contactLists(c),name.trim()],'added '+name.trim());
export const removeMailingList=(s:AppState,c:Contact,name:string)=>setMailingLists(s,c,contactLists(c).filter(n=>mailingKey(n)!==mailingKey(name)),'removed '+name);
export function channelLabel(k:MailingChannel|string,l2:(en:string,zh:string)=>string){return ({email:l2('Email','邮件'),sms:l2('Text','短信'),phone:l2('Call','电话'),letter:l2('Letter','信件')} as Record<string,string>)[k]||k;}
