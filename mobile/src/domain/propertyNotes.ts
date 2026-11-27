import type {AppState,Contact} from '../data/types';
import {filePropertyNote} from './dk';
import {byId,linkOwner,normMobile} from './index';
import {requireNewContactName} from './contactName';
import {today} from '../lib/today';
import {extract} from './extract';
import {mergeAI,aiText} from './aiMerge';
import type {AIResult} from '../lib/voice';

export type NoteIdentity={first:string;last:string;mobile:string};
/** A name part is kept only when it has letters/CJK and is not a model placeholder. */
export const noteName=(value:unknown)=>{const t=aiText(value);return /[\p{L}]/u.test(t)?t:'';};
/** Owner phone must be a full Australian number: mobile 04xxxxxxxx, or a landline with its area code 0[2378]xxxxxxxx.
 * Shorter digit runs (e.g. STT turning 0467 076 507 into 67076507) stay in the note text only. */
export const plausibleNotePhone=(value:unknown)=>{const t=aiText(value);if(!/^[\d\s()+.-]+$/.test(t))return false;return /^0[2-478]\d{8}$/.test(canonicalMobile(t));};
export function noteIdentity(transcript:string,ai?:AIResult):NoteIdentity{
  const result=extract(transcript,'dk');
  const fields=ai?mergeAI(result,ai):result;
  // Like mergeAI, an unusable AI value falls back to what the local parser heard.
  const name={first:noteName(fields.first)||noteName(result.first),last:noteName(fields.last)||noteName(result.last)};
  const mobile=[fields.mobile,result.mobile].find(plausibleNotePhone)||'';
  return {...name,mobile:mobile.trim()};
}
/** Confirmation-card label: only the parts that were heard, no stray separators. */
export const noteIdentityLabel=(identity:NoteIdentity)=>[[identity.first,identity.last].filter(Boolean).join(' '),identity.mobile].filter(Boolean).join(' · ');
export const hasNoteIdentity=(identity:NoteIdentity)=>!!(identity.first||identity.last||identity.mobile);
export function canonicalMobile(value:string){const digits=normMobile(value);return digits.startsWith('61')&&digits.length===11?'0'+digits.slice(2):digits;}
export function matchingNoteContacts(s:AppState,mobile:string){const number=canonicalMobile(mobile);return number.length===10?s.contacts.filter(c=>canonicalMobile(c.mobile)===number):[];}
/** Save the transcript even if no identity was heard; only explicit confirmation links an owner. */
export function savePropertyTranscript(s:AppState,propId:string,transcript:string,identity?:NoteIdentity,pick?:string){
  const matches=identity?matchingNoteContacts(s,identity.mobile):[];
  if(identity&&matches.length>1&&!pick)throw new Error('Choose a matching contact');
  let c:Contact|undefined=pick?byId(s.contacts,pick):matches.length===1?matches[0]:undefined;
  if(pick&&!c)throw new Error('Contact not found');
  // A heard mobile with no name and no matching contact cannot become a new owner (Rex rejects it); refuse before filing the note.
  if(identity&&!c)requireNewContactName(identity);
  const p=filePropertyNote(s,propId,transcript.trim());
  if(!identity)return p;
  if(!c){
    c={id:'c'+Date.now()+Math.random().toString(36).slice(2),first:identity.first.trim(),last:identity.last.trim(),mobile:identity.mobile.trim(),email:'',owner:s.auth?.name||'Owen Chen',roles:[],tags:[],flags:{},notes:[],rels:[],updated:{by:s.auth?.name||'Owen Chen',d:today()}};
    s.contacts.push(c);
  }
  if(!c.roles.includes('owner'))c.roles.push('owner');
  linkOwner(s,c,p);
  return p;
}
