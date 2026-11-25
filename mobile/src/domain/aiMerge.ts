import type {AIResult,AIFields} from '../lib/voice';
import {extract} from './extract';

const placeholderWords=new Set(["unknown","n/a","na","null","nil","none","-","--","?","??","not provided","not stated","not mentioned","undefined"]);
/** Model stand-ins for "no value" (e.g. "<UNKNOWN>", "N/A", "<UNKNOWN><UNKNOWN>.<UNKNOWN>") become empty; mirrors server aiCleanString. */
export function aiText(value:unknown):string{
  if(typeof value!=="string")return "";let t=value.trim();
  if(t.includes("<"))t=t.replace(/<[^<>]*>/g," ").split(/\s+/).filter(Boolean).join(" ").replace(/^[\s\p{P}]+|[\s\p{P}]+$/gu,"");
  return placeholderWords.has(t.toLowerCase())?"":t;
}
const cleanFields=(f:AIFields)=>Object.fromEntries(Object.entries(f).map(([k,v])=>[k,typeof v==="string"?(aiText(v)||null):v])) as AIFields;
/** Retain the local parser's tags, requests and follow-up; AI overrides only supplied fields. */
export function mergeAI(local:ReturnType<typeof extract>,result:AIResult){
  const r={...local,tags:[...local.tags],unsure:[...local.unsure]};const f=cleanFields(result.fields||{} as AIFields);
  const put=(source:keyof AIFields,target:string,value:unknown=f[source])=>{
    if(value===null||value===undefined||value==='')return;
    r[target]=value;
    if((result.confidence?.[source]??1)<0.6 && !r.unsure.includes(target))r.unsure.push(target);
    else if((result.confidence?.[source]??1)>=0.6)r.unsure=r.unsure.filter((x:string)=>x!==target && !(target==='first'&&x==='name'));
  };
  const chinese=/^[\u3400-\u9fff]+$/;
  if(f.firstName||f.lastName){put('firstName','first');put('lastName','last');}
  else if(f.contactName){const name=f.contactName.trim();if(chinese.test(name)){put('contactName','first',name);put('contactName','last','');r.last='';}else{const parts=name.split(/\s+/);put('contactName','first',parts[0]);if(parts.length>1)put('contactName','last',parts.slice(1).join(' '));}}
  if(f.phone){const digits=f.phone.replace(/\D/g,'');const ok=/^0[45]\d{8}$/.test(digits);put('phone','mobile',ok?digits.replace(/(\d{4})(\d{3})(\d{3})/,'$1 $2 $3'):f.phone);if(!ok&&!r.unsure.includes('mobile'))r.unsure.push('mobile');}
  put('email','email');put('address','address');put('notes','note');put('timeframe','timing');
  if(f.intent){const intent={sell:'Seller lead',appraisal:'Seller lead',buy:'Buyer lead',rent:'Rental lead',other:'Other'}[f.intent];if(intent){put('intent','intent',intent);if(f.intent==='sell'||f.intent==='appraisal'){r.ps=true;r.type='seller';if(!r.tags.includes('Seller lead'))r.tags.push('Seller lead');}else if(f.intent==='buy')r.type='buyer';else if(f.intent==='rent')r.type='tenant';}}
  if(r.mobile&&!r.tags.includes('Phone provided'))r.tags.push('Phone provided');
  if(r.email&&!r.tags.includes('Email provided'))r.tags.push('Email provided');
  r.ai=[...r.tags];return r;
}
