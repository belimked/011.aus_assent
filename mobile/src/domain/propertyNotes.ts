import type {AppState,Contact} from '../data/types';
import {filePropertyNote} from './dk';
import {byId,linkOwner,normMobile} from './index';
import {today} from '../lib/today';
import {extract} from './extract';
import {mergeAI} from './aiMerge';
import type {AIResult} from '../lib/voice';

export type NoteIdentity={first:string;last:string;mobile:string};
export function noteIdentity(transcript:string,ai?:AIResult):NoteIdentity{
  const result=extract(transcript,'dk');
  const fields=ai?mergeAI(result,ai):result;
  return {first:fields.first||'',last:fields.last||'',mobile:fields.mobile||''};
}
export function canonicalMobile(value:string){const digits=normMobile(value);return digits.startsWith('61')&&digits.length===11?'0'+digits.slice(2):digits;}
export function matchingNoteContacts(s:AppState,mobile:string){const number=canonicalMobile(mobile);return number.length===10?s.contacts.filter(c=>canonicalMobile(c.mobile)===number):[];}
/** Save the transcript even if no identity was heard; only explicit confirmation links an owner. */
export function savePropertyTranscript(s:AppState,propId:string,transcript:string,identity?:NoteIdentity,pick?:string){
  const matches=identity?matchingNoteContacts(s,identity.mobile):[];
  if(identity&&matches.length>1&&!pick)throw new Error('Choose a matching contact');
  let c:Contact|undefined=pick?byId(s.contacts,pick):matches.length===1?matches[0]:undefined;
  if(pick&&!c)throw new Error('Contact not found');
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
