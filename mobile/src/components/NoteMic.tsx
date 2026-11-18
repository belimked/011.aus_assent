import React,{useEffect,useRef} from 'react';
import {ActivityIndicator,Platform,Pressable,View} from 'react-native';
import {Icon} from './ui/Icon';
import {colors} from '../theme';
import {useT} from '../i18n';
import {showToast} from '../store';
import {useHoldRecorder} from './useHoldRecorder';
import {voiceErrorText} from '../lib/voice';
import {getVoiceLang} from '../lib/voicePref';

/** Prototype "nmic": tap to dictate into the field beside it, tap again to stop; the words are appended. */
export function NoteMic({value,onChangeText,maxMs=60000,size=40}:{value:string;onChangeText:(v:string)=>void;maxMs?:number;size?:number}){
  const {l2,lang:appLang}=useT();const rec=useHoldRecorder();const latest=useRef(value);useEffect(()=>{latest.current=value},[value]);const timer=useRef<ReturnType<typeof setTimeout>|undefined>(undefined);
  useEffect(()=>()=>clearTimeout(timer.current),[]);
  const stop=async()=>{clearTimeout(timer.current);try{const words=await rec.finish(getVoiceLang(appLang));if(words===null)return;if(!words){showToast(l2('Could not hear that','没有听清'));return}const v=latest.current.trim();onChangeText(v?`${v} ${words}`:words)}catch(e){showToast(voiceErrorText(e,l2))}};
  const start=async()=>{
    if(Platform.OS==='web'){showToast(l2('Dictation works in the iPhone app. Type instead.','听写请在 iPhone App 里使用，这里请直接输入。'));return}
    const r=await rec.start();
    if(r==='ok'){timer.current=setTimeout(()=>void stop(),maxMs);return}
    showToast(r==='asked'?l2('Microphone ready. Tap the mic again to dictate.','麦克风已开启，再点一次话筒开始听写。'):r==='offline'?l2('Sign in with your email to use voice. Type instead.','请用邮箱登录后使用语音，或直接输入。'):r==='denied'?l2('Microphone access is off. Enable it in Settings, or type instead.','麦克风权限已关闭，请在设置中开启，或直接输入。'):l2('Could not start recording. Try again, or type instead.','无法开始录音，请重试或直接输入。'));
  };
  const live=rec.phase==='recording';
  return <Pressable accessibilityRole="button" accessibilityLabel={live?l2('Stop dictation','停止听写'):l2('Dictate','听写')} disabled={rec.phase==='transcribing'} onPress={()=>void (live?stop():start())} hitSlop={6} style={{width:size,height:size,borderRadius:size/2,backgroundColor:colors.mic,alignItems:'center',justifyContent:'center'}}>
    {rec.phase==='transcribing'?<ActivityIndicator color="#fff"/>:live?<View style={{width:14,height:14,borderRadius:3,backgroundColor:'#fff'}}/>:<Icon name="mic" size={18} color="#fff"/>}
    {live&&<View pointerEvents="none" style={{position:'absolute',top:-5,left:-5,right:-5,bottom:-5,borderRadius:size/2+5,borderWidth:2,borderColor:colors.mic,opacity:.5}}/>}
  </Pressable>;
}
