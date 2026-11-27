import React,{useEffect,useRef,useState} from 'react';
import {Platform,Pressable,Text,View} from 'react-native';
import {Icon} from './ui/Icon';
import {colors,text} from '../theme';
import {useT} from '../i18n';
import {voiceErrorText,VoiceError,type VoiceLang} from '../lib/voice';
import {useHoldRecorder,holdStartText,holdShortText} from './useHoldRecorder';
export {recordingOptions} from './recordingOptions';

export type RecorderPhase='idle'|'recording'|'uploading'|'error';
type RecorderProps={lang:VoiceLang;onText:(text:string)=>void;onError?:(error:unknown)=>void;onBusy?:(busy:boolean)=>void;/** Parent shows its own status; the recorder then only shows errors under the button. */onPhase?:(phase:RecorderPhase)=>void;label?:string;activeLabel?:string;color?:string;wide?:boolean;disabled?:boolean};
/** Hold to speak, release to finish (the only voice gesture in the app). */
function RecorderControl({lang,onText,onError,onBusy,onPhase,label,activeLabel,color,wide,disabled}:RecorderProps){
  const {l2}=useT();const [error,setError]=useState('');const latestLang=useRef(lang);useEffect(()=>{latestLang.current=lang},[lang]);
  const rec=useHoldRecorder({lang:()=>latestLang.current,
    onStart:r=>{if(r==='ok'){setError('');return}if(r==='busy')return;if(r==='offline'){const e=new VoiceError('offline','not signed in');setError(voiceErrorText(e,l2));onError?.(e);return}const message=holdStartText(r,l2);setError(message);if(r!=='asked')onError?.(message)},
    onResult:r=>{if(r.kind==='text'){setError('');onText(r.text)}else if(r.kind==='short')setError(holdShortText(l2));else if(r.kind==='empty')setError(l2('Nothing heard. Hold and speak, or type instead.','没有听清。请按住说话，或直接输入。'));else{onError?.(r.error);setError(voiceErrorText(r.error,l2))}}});
  const phase:RecorderPhase=rec.phase==='recording'?'recording':rec.phase==='transcribing'?'uploading':error?'error':'idle';
  useEffect(()=>{onPhase?.(phase)},[phase,onPhase]);
  const uploading=phase==='uploading';const wasUploading=useRef(false);useEffect(()=>{if(uploading!==wasUploading.current){wasUploading.current=uploading;onBusy?.(uploading)}},[uploading,onBusy]);
  const release=l2('Release to finish','松开结束'),hold=l2('Hold to speak','按住说话');
  return <View style={{alignItems:'center',gap:8,paddingVertical:wide?0:12,width:wide?'100%':undefined}}><Pressable accessibilityRole="button" accessibilityLabel={phase==='recording'?release:hold} accessibilityHint={l2('Press and hold while you speak, release to finish','说话时按住，松开结束')} disabled={uploading||disabled} {...rec.pressProps} style={{width:wide?'100%':76,height:wide?58:76,borderRadius:wide?12:38,backgroundColor:phase==='recording'?colors.mic:color||colors.fg,alignItems:'center',justifyContent:'center',flexDirection:'row',gap:10,borderWidth:phase==='recording'?4:0,borderColor:colors.line}}><Icon name="mic" size={wide?24:32} color="#fff"/>{wide&&<Text style={{color:'#fff',fontWeight:'700',fontSize:18}}>{phase==='recording'?activeLabel||release:label||hold}</Text>}</Pressable>{!(wide&&phase==='idle')&&!(onPhase&&phase!=='error')&&<Text style={[text,{fontSize:13,color:colors.muted}]}>{phase==='recording'?`${release} · ${Math.floor(rec.durationMs/1000)} / 120 s  ${rec.metering!=null?'▮'.repeat(Math.max(1,Math.min(5,Math.round((rec.metering+60)/12)))):''}`:uploading?l2('Transcribing…','正在转写…'):phase==='error'?error:l2('Hold to speak, release to finish (up to 2 minutes)','按住说话，松开结束（最长两分钟）')}</Text>}</View>;
}
export default function NativeRecorder(props:RecorderProps){return Platform.OS==='web'?null:<RecorderControl {...props}/>;}
