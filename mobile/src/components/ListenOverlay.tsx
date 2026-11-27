import React from 'react';
import {ActivityIndicator,Text,View} from 'react-native';
import {useSafeAreaInsets} from 'react-native-safe-area-context';
import {Icon} from './ui';
import {colors,text} from '../theme';
import {useT} from '../i18n';
import {VOICE_LANG_LABEL} from '../lib/voicePref';
import type {VoiceLang} from '../lib/voice';

/** The prototype's "listen" sheet, drawn in-screen (not a Modal) while the microphone is active. */
export function ListenOverlay({phase,lang,durationMs,metering}:{phase:'recording'|'transcribing';lang:VoiceLang;durationMs:number;metering?:number}){
  const {t,l2}=useT();const safe=useSafeAreaInsets();
  const level=metering==null?0.35:Math.max(0.12,Math.min(1,(metering+60)/50));
  const bars=[0.55,0.85,1,0.75,0.95,0.6].map((k,i)=>phase==='recording'?Math.max(8,Math.round(44*level*k*(0.75+0.25*Math.sin(durationMs/140+i*1.3)))):12);
  return <View pointerEvents="none" style={{position:'absolute',left:0,right:0,top:0,bottom:0,justifyContent:'flex-end',backgroundColor:'#13233F55'}}>
    <View style={{backgroundColor:colors.bg,borderTopLeftRadius:18,borderTopRightRadius:18,padding:20,paddingBottom:safe.bottom+150,gap:14}}>
      <View style={{flexDirection:'row',alignItems:'center',gap:8}}><Icon name="mic" size={20} color={colors.mic}/><Text style={[text,{fontSize:18,fontWeight:'700'}]}>{phase==='recording'?t('listening'):l2('Transcribing…','正在转写…')}</Text><View style={{paddingHorizontal:8,paddingVertical:3,borderRadius:10,backgroundColor:colors.surface,borderWidth:1,borderColor:colors.line}}><Text style={[text,{fontSize:12}]}>{VOICE_LANG_LABEL[lang]}</Text></View></View>
      <View style={{height:48,flexDirection:'row',gap:4,alignItems:'center',justifyContent:'center'}}>{phase==='recording'?bars.map((h,i)=><View key={i} style={{width:4,height:h,borderRadius:2,backgroundColor:colors.mic}}/>):<ActivityIndicator color={colors.mic}/>}</View>
      <Text style={[text,{textAlign:'center',color:colors.muted,fontSize:13}]}>{phase==='recording'?`${t('release')} · ${Math.floor(durationMs/1000)} / 120 s`:l2('Turning your words into text…','正在把语音转成文字…')}</Text>
    </View>
  </View>;
}
