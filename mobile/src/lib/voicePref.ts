import {useSyncExternalStore} from 'react';
import AsyncStorage from '@react-native-async-storage/async-storage';
import type {VoiceLang} from './voice';

/** The language the agent speaks into the mic; shared by the hold-to-speak overlay and the Say it or type it sheet. */
const key='ax_voice_lang';
let current:VoiceLang|null=null;const listeners=new Set<()=>void>();
void AsyncStorage.getItem(key).then(v=>{if(!current&&(v==='en'||v==='zh'||v==='yue')){current=v;listeners.forEach(f=>f())}}).catch(()=>{});
export function getVoiceLang(fallback:VoiceLang):VoiceLang{return current||fallback}
export function setVoiceLang(lang:VoiceLang){current=lang;listeners.forEach(f=>f());void AsyncStorage.setItem(key,lang).catch(()=>{})}
export function useVoiceLang(fallback:VoiceLang):[VoiceLang,(lang:VoiceLang)=>void]{
  const value=useSyncExternalStore(f=>{listeners.add(f);return()=>{listeners.delete(f)}},()=>current,()=>current);
  return [value||fallback,setVoiceLang];
}
export const VOICE_LANG_LABEL:Record<VoiceLang,string>={en:'English',zh:'普通话',yue:'粤语'};
