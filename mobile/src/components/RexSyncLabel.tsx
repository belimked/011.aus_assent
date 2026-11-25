import React from 'react';
import {Text} from 'react-native';
import {useT} from '../i18n';
import {colors,text} from '../theme';
export function RexSyncLabel({value}:{value?:{status:string;reason?:string;at?:string}}){const {l2}=useT();if(!value)return null;const {status,reason}=value;const label=status==='sent'?l2('In Rex','已同步至 Rex'):status==='pending'||status==='waiting_auth'?l2('Pending Rex','等待同步 Rex'):status==='failed'?l2('Rex failed','Rex 同步失败'):l2('Rex skipped','未同步 Rex');return <Text style={[text,{fontSize:12,color:status==='failed'?colors.red:colors.muted}]}>{label}{reason?`: ${reason}`:''}</Text>}
