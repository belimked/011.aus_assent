import React,{useState} from 'react';
import {Text,View} from 'react-native';
import {Button,Chip,Field} from '../../components/ui';
import {colors,text} from '../../theme';
import {closeSheet} from '../../store';
import {useT} from '../../i18n';
import type {SheetProps} from '../types';
const samples=['Wants an appraisal, moving interstate','Pre-approved, looking for four bedrooms in Runcorn','Please send the building and pest report'];
export function DictateSheet({params}:SheetProps){const {l2}=useT();const [value,setValue]=useState(String(params.value||''));return <View style={{gap:14}}><Text style={[text,{fontSize:18,fontWeight:'700'}]}>{l2('Dictate note','语音输入备注')}</Text><Text style={[text,{fontSize:12,color:colors.muted}]}>{l2('Speech recognition is coming later. Type or choose a sample.','语音识别稍后接入。请手动输入或选择示例。')}</Text><Field label={l2('TRANSCRIPT','转写内容')} multiline value={value} onChangeText={setValue}/><View style={{flexDirection:'row',flexWrap:'wrap',gap:6}}>{samples.map(s=><Chip key={s} kind="blue" onPress={()=>setValue(s)}>{s}</Chip>)}</View><Button variant="primary" onPress={()=>{params.onResult?.(value);closeSheet()}}>{l2('Use text','使用文字')}</Button></View>}
// TODO: replace sample/text capture with device STT when available.
