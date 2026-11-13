import {useSyncExternalStore} from 'react';
import AsyncStorage from '@react-native-async-storage/async-storage';
import en from './en'; import zh from './zh';
export type Language = 'en'|'zh';
let language: Language = 'en'; const listeners = new Set<()=>void>();
export const setLanguage = (value: Language) => { language=value; listeners.forEach(fn=>fn()); void AsyncStorage.setItem('ax_lang',value).catch(()=>{}); };
export async function hydrateLanguage() { try { const v = await AsyncStorage.getItem('ax_lang'); if(v==='zh') setLanguage('zh'); } catch {} }
export function useT() {const lang=useSyncExternalStore(fn=>{listeners.add(fn);return()=>{listeners.delete(fn)}},()=>language,()=> 'en' as Language); return {lang,t:(key:string)=> (lang==='zh'?zh[key]:en[key]) ?? en[key] ?? key, l2:(english:string,chinese:string)=>lang==='zh'?chinese:english};}
export const l2 = (english:string,chinese:string) => language==='zh'?chinese:english;
