import React,{useState} from 'react';
import {Text,View} from 'react-native';
import {Button,Chip,Field,Seg} from '../../components/ui';
import {text} from '../../theme';
import {closeSheet} from '../../store';
import {useT} from '../../i18n';
import NativeRecorder from '../../components/Recorder';
import type {VoiceLang} from '../../lib/voice';
import type {SheetProps} from '../types';
const samples=['Wants an appraisal, moving interstate','Pre-approved, looking for four bedrooms in Runcorn','Please send the building and pest report'];
let lastLang:VoiceLang|undefined;
export function DictateSheet({params}:SheetProps){const {l2,lang:appLang}=useT();const [lang,setLang]=useState<VoiceLang>(lastLang||appLang);const [value,setValue]=useState(String(params.value||''));return <View style={{gap:14}}><Text style={[text,{fontSize:18,fontWeight:'700'}]}>{l2('Dictate note','语音输入备注')}</Text><Seg items={[["en","English"],["zh","普通话"],["yue","粤语"]]} value={lang} onChange={v=>{lastLang=v as VoiceLang;setLang(v as VoiceLang)}}/><NativeRecorder lang={lang} onText={transcript=>setValue(v=>v.trim()?`${v.trim()} ${transcript}`:transcript)}/><Field autoFocus label={l2('TRANSCRIPT','转写内容')} multiline value={value} onChangeText={setValue}/><View style={{flexDirection:'row',flexWrap:'wrap',gap:6}}>{samples.map(s=><Chip key={s} kind="blue" onPress={()=>setValue(s)}>{s}</Chip>)}</View><Button variant="primary" onPress={()=>{params.onResult?.(value);closeSheet()}}>{l2('Use text','使用文字')}</Button></View>}
