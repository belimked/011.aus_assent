// Contact merge preview (server: POST /contacts/merge-preview). The server computes
// every rule; these helpers only turn its result into labelled lines for the sheet.
import {mask, maskEmail} from './index';

export type L2 = (english:string, chinese:string) => string;
export type MergeSummary = {id:string;name:string;mobile:string;email:string;createdAt:string;updatedAt:string;rexLinked:boolean};
export type MergeField = {key:string;value:unknown;from:string;a:unknown;b:unknown;reason:'newer'|'only'|'same'};
export type MergeConsent = {channel:string;value:unknown;a:unknown;b:unknown};
export type MergeNote = {d:string;a:string;t:string;at?:string;from:string;log?:boolean};
export type MergeRelation = {propertyId:string;address:string;relation:string;label:string;at:string;from:string;changed:boolean;replaced?:{relation:string;label:string;from:string}[]};
export type MergePreview = {
  primaryId:string;secondaryId:string;primaryReason:'rex'|'older';contacts:[MergeSummary,MergeSummary];
  fields:MergeField[];dnc:{value:boolean;a:boolean;b:boolean};consent:MergeConsent[];notes:MergeNote[];
  tags:string[];roles:string[];properties:MergeRelation[];relations:unknown[];logNote:string;previewHash:string;
  rex:'both'|'primary'|'secondary'|'none'|'pending';
};
export type MergeResult = {ok:true;primaryId:string;secondaryId:string} | {stale:MergePreview};

const FIELD_LABELS:Record<string,[string,string]>={first:['First name','名'],last:['Last name','姓'],mobile:['Mobile','手机'],email:['Email','邮箱'],address:['Address','住址'],reason:['Reason','动机']};
export function fieldLabel(key:string,l2:L2){const pair=FIELD_LABELS[key];return pair?l2(pair[0],pair[1]):key;}

/** Phone numbers and emails stay masked on the confirmation screen, as elsewhere. */
export function fieldValue(key:string,value:unknown){
  if(value===null||value===undefined||value==='')return '—';
  if(key==='mobile')return mask(String(value));
  if(key==='email')return maskEmail(String(value));
  if(typeof value==='boolean')return value?'✓':'✗';
  return String(value);
}

/** "A" is the record that keeps its id, "B" is removed. */
export function side(p:MergePreview,id:string):'A'|'B'|'A+B'{return id===p.primaryId?'A':id===p.secondaryId?'B':'A+B';}

export function fieldReason(f:MergeField,p:MergePreview,l2:L2){
  if(f.reason==='same')return l2('Same on both','两边相同');
  if(f.reason==='only')return l2(`Only ${side(p,f.from)} has it`,`仅 ${side(p,f.from)} 有值`);
  return l2(`${side(p,f.from)} is newer`,`${side(p,f.from)} 更新`);
}

export function primaryReason(p:MergePreview,l2:L2){
  return p.primaryReason==='rex'
    ?l2('A is kept because it is linked to Rex.','保留 A，因为它已关联 Rex。')
    :l2('A is kept because it was created first.','保留 A，因为它创建得更早。');
}

const CHANNELS:Record<string,[string,string]>={sms:['Text','短信'],phone:['Call','电话'],email:['Email','邮件'],letter:['Letter','信件'],junk:['Junk mail','广告邮件']};
export const refused=(v:unknown)=>v===0||v===false||v==='0';
export function consentLines(p:MergePreview,l2:L2){
  return p.consent.map(c=>{const pair=CHANNELS[c.channel];const name=pair?l2(pair[0],pair[1]):c.channel;
    return `${name}: ${refused(c.value)?l2('opted out','已退订'):c.value===undefined||c.value===null?'—':l2('allowed','允许')}`;});
}
export function dncLine(p:MergePreview,l2:L2){
  if(!p.dnc.value)return l2('Do not contact: no','勿联系：否');
  return p.dnc.a&&p.dnc.b?l2('Do not contact: yes','勿联系：是')
    :l2(`Do not contact: yes (from ${p.dnc.a?'A':'B'}; either one wins)`,`勿联系：是（来自 ${p.dnc.a?'A':'B'}，任一方为是即为是）`);
}

export function relationLines(p:MergePreview,l2:L2){
  return p.properties.map(r=>{const where=r.address||r.propertyId;
    const replaced=(r.replaced||[]).map(x=>x.label).join(', ');
    return replaced
      ?l2(`${where} · ${r.label} (newest, replaces ${replaced})`,`${where} · ${r.label}（最新，取代 ${replaced}）`)
      :`${where} · ${r.label}`;});
}

export function rexLine(p:MergePreview,l2:L2){
  switch(p.rex){
    case 'both':return l2('Both are in Rex: Rex combines them into A, then A is updated.','两条都在 Rex：Rex 中合并到 A，再更新 A。');
    case 'primary':return l2('Only A is in Rex: A is updated in Rex.','只有 A 在 Rex：更新 Rex 中的 A。');
    case 'secondary':return l2('Only B is in Rex: A takes over B\'s Rex record.','只有 B 在 Rex：A 接管 B 的 Rex 记录。');
    case 'pending':return l2('A Rex record is still being created; Rex is updated once it exists.','Rex 记录仍在创建中，创建后再更新 Rex。');
    default:return l2('Neither is in Rex: nothing is sent to Rex.','两条都不在 Rex：不会写入 Rex。');
  }
}

export const IRREVERSIBLE:[string,string]=['This cannot be undone.','合并后无法撤销。'];
export function confirmLabel(busy:boolean,l2:L2){return busy?l2('Merging…','正在合并…'):l2('Merge · cannot be undone','合并 · 无法撤销');}
export function offlineMessage(l2:L2){return l2('Merging needs a connection. Try again when you are online.','合并需要联网，请联网后再试。');}
