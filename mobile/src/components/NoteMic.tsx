import React,{useEffect,useRef} from 'react';
import {ActivityIndicator,Platform,Pressable,Text,View} from 'react-native';
import {Icon} from './ui/Icon';
import {colors} from '../theme';
import {useT} from '../i18n';
import {showToast} from '../store';
import {useHoldRecorder,holdStartText,holdShortText} from './useHoldRecorder';
import {voiceErrorText} from '../lib/voice';
import {getVoiceLang} from '../lib/voicePref';

/** Inline dictation mic: hold to dictate into the field beside it, release to finish; the words are appended. */
export function NoteMic({value,onChangeText,onTranscript,size=40}:{value:string;onChangeText:(v:string)=>void;onTranscript?:(v:string)=>void;size?:number}){
  const {l2,lang:appLang}=useT();const latest=useRef(value);useEffect(()=>{latest.current=value},[value]);
  const rec=useHoldRecorder({lang:()=>getVoiceLang(appLang),
    onStart:r=>{const message=holdStartText(r,l2);if(message)showToast(message)},
    onResult:r=>{if(r.kind==='text'){const v=latest.current.trim();(onTranscript||onChangeText)(v?`${v} ${r.text}`:r.text)}else showToast(r.kind==='short'?holdShortText(l2):r.kind==='empty'?l2('Could not hear that','没有听清'):voiceErrorText(r.error,l2))}});
  const live=rec.phase==='recording';
  const web=()=>showToast(l2('Dictation works in the iPhone app. Type instead.','听写请在 iPhone App 里使用，这里请直接输入。'));
  return <View style={{alignItems:'center'}}><Pressable accessibilityRole="button" accessibilityLabel={live?l2('Release to finish dictation','松开结束听写'):l2('Hold to dictate','按住听写')} accessibilityHint={l2('Press and hold while you speak, release to finish','说话时按住，松开结束')} disabled={rec.phase==='transcribing'} {...(Platform.OS==='web'?{onPress:web}:rec.pressProps)} hitSlop={6} style={{width:size,height:size,borderRadius:size/2,backgroundColor:colors.mic,alignItems:'center',justifyContent:'center'}}>
    {rec.phase==='transcribing'?<ActivityIndicator color="#fff"/>:<Icon name="mic" size={18} color="#fff"/>}
    {live&&<View pointerEvents="none" style={{position:'absolute',top:-5,left:-5,right:-5,bottom:-5,borderRadius:size/2+5,borderWidth:2,borderColor:colors.mic,opacity:.5}}/>}
  </Pressable>{live&&<Text style={{fontSize:10,color:colors.mic,textAlign:'center'}}>{Math.floor(rec.durationMs/1000)}s · {l2('Release to finish','松开结束')}</Text>}</View>;
}
