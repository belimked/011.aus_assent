import React,{useEffect,useRef,useState} from 'react';
import {Platform,Pressable,Text,View} from 'react-native';
import {RecordingPresets,IOSOutputFormat,requestRecordingPermissionsAsync,setAudioModeAsync,useAudioRecorder,useAudioRecorderState} from 'expo-audio';
import {Icon} from './ui';
import {colors,text} from '../theme';
import {useT} from '../i18n';
import {transcribe,type VoiceLang} from '../lib/voice';

const options={...RecordingPresets.HIGH_QUALITY,isMeteringEnabled:true,...(Platform.OS==='ios'?{extension:'.wav',sampleRate:16000,numberOfChannels:1,ios:{...RecordingPresets.HIGH_QUALITY.ios,outputFormat:IOSOutputFormat.LINEARPCM,sampleRate:16000,linearPCMBitDepth:16,linearPCMIsBigEndian:false,linearPCMIsFloat:false}}:{})};
function RecorderControl({lang,onText}:{lang:VoiceLang;onText:(text:string)=>void}){
  const {l2}=useT();const recorder=useAudioRecorder(options);const status=useAudioRecorderState(recorder,250);
  const [phase,setPhase]=useState<'idle'|'recording'|'uploading'|'error'>('idle');const [error,setError]=useState('');
  const busy=useRef(false);const mounted=useRef(true);const limit=useRef<ReturnType<typeof setTimeout>|undefined>(undefined);
  useEffect(()=>()=>{mounted.current=false;clearTimeout(limit.current);if(busy.current)void recorder.stop().catch(()=>{});},[recorder]);
  const stop=async()=>{if(!busy.current)return;busy.current=false;clearTimeout(limit.current);setPhase('uploading');try{await recorder.stop();const uri=recorder.uri;if(!uri)throw new Error('No recording');const result=await transcribe(uri,lang);if(mounted.current){onText(result.text);setPhase('idle');}}catch(e){if(mounted.current){setError(String(e));setPhase('error');}}finally{void setAudioModeAsync({allowsRecording:false}).catch(()=>{});}};
  const toggle=async()=>{if(busy.current){await stop();return}if(phase==='uploading')return;
    try{const permission=await requestRecordingPermissionsAsync();if(!permission.granted){setError(l2('Microphone access is off. Enable it in device settings to record, or type instead.','麦克风权限已关闭。请在设备设置中启用，或直接输入文字。'));setPhase('error');return}
      await setAudioModeAsync({allowsRecording:true,playsInSilentMode:true});await recorder.prepareToRecordAsync();recorder.record();busy.current=true;setPhase('recording');limit.current=setTimeout(()=>void stop(),120000);
    }catch(e){setError(String(e));setPhase('error');}
  };
  return <View style={{alignItems:'center',gap:8,paddingVertical:12}}><Pressable accessibilityRole="button" accessibilityLabel={phase==='recording'?l2('Stop recording','停止录音'):l2('Start recording','开始录音')} disabled={phase==='uploading'} onPress={()=>void toggle()} style={{width:76,height:76,borderRadius:38,backgroundColor:phase==='recording'?colors.mic:colors.fg,alignItems:'center',justifyContent:'center',borderWidth:phase==='recording'?4:0,borderColor:colors.line}}><Icon name="mic" size={32} color="#fff"/></Pressable><Text style={[text,{fontSize:13,color:colors.muted}]}>{phase==='recording'?`${l2('Tap to stop','点击停止')} · ${Math.floor(status.durationMillis/1000)} / 120 s  ${status.metering!=null?'▮'.repeat(Math.max(1,Math.min(5,Math.round((status.metering+60)/12)))):''}`:phase==='uploading'?l2('Transcribing…','正在转写…'):phase==='error'?error:l2('Tap to record (up to 2 minutes)','点击录音（最长两分钟）')}</Text></View>;
}
export default function NativeRecorder(props:{lang:VoiceLang;onText:(text:string)=>void}){return Platform.OS==='web'?null:<RecorderControl {...props}/>;}
