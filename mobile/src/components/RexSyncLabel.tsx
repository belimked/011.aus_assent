import React from 'react';
import {Text} from 'react-native';
import {useT} from '../i18n';
import {colors,text} from '../theme';
type RexSync={status:string;reason?:string;at?:string};
type L2=(en:string,zh:string)=>string;
export const rexSyncText=(value:RexSync,l2:L2)=>{const {status,reason}=value;const label=status==='sent'?l2('In Rex','已同步至 Rex'):status==='pending'||status==='waiting_auth'?l2('Pending Rex','等待同步 Rex'):status==='failed'?l2('Rex failed','Rex 同步失败'):l2('Rex skipped','未同步 Rex');return `${label}${reason?`: ${reason}`:''}`};
export function RexSyncLabel({value,label}:{value?:RexSync;label?:string}){const {l2}=useT();if(!value)return null;return <Text style={[text,{fontSize:12,color:value.status==='failed'?colors.red:colors.muted}]}>{label?`${label} · `:''}{rexSyncText(value,l2)}</Text>}
// Property owner area: the owner contact and the property's owner link each carry a Rex status.
// Both reading the same → one line; otherwise label each so neither status is lost.
export const ownerRexSyncLines=(owner:boolean,contact:RexSync|undefined,link:RexSync|undefined,l2:L2):{label?:string;value:RexSync}[]=>{if(!owner)return link?[{value:link}]:[];if(contact&&link&&rexSyncText(contact,l2)===rexSyncText(link,l2))return [{label:l2('Contact and owner link','联系人和业主关联'),value:contact}];return [...(contact?[{label:l2('Contact','联系人'),value:contact}]:[]),...(link?[{label:l2('Owner link','业主关联'),value:link}]:[])]};
export function OwnerRexSync({owner,contact,link}:{owner:boolean;contact?:RexSync;link?:RexSync}){const {l2}=useT();return <>{ownerRexSyncLines(owner,contact,link,l2).map(x=><RexSyncLabel key={x.label||''} label={x.label} value={x.value}/>)}</>}
