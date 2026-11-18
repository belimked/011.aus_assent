import React,{useEffect,useRef,useState} from 'react';
import {Platform,Pressable,Text,View} from 'react-native';
import {RecordingPresets,IOSOutputFormat,requestRecordingPermissionsAsync,setAudioModeAsync,useAudioRecorder,useAudioRecorderState} from 'expo-audio';
import {File} from 'expo-file-system';
import {Icon} from './ui';
import {colors,text} from '../theme';
import {useT} from '../i18n';
import {transcribe,type VoiceLang} from '../lib/voice';

const options={...RecordingPresets.HIGH_QUALITY,isMeteringEnabled:true,...(Platform.OS==='ios'?{extension:'.wav',sampleRate:16000,numberOfChannels:1,ios:{...RecordingPresets.HIGH_QUALITY.ios,outputFormat:IOSOutputFormat.LINEARPCM,sampleRate:16000,linearPCMBitDepth:16,linearPCMIsBigEndian:false,linearPCMIsFloat:false}}:{})};
type RecorderProps={lang:VoiceLang;onText:(text:string)=>void;onError?:(error:unknown)=>void;onBusy?:(busy:boolean)=>void;holdToTalk?:boolean;color?:string;wide?:boolean;disabled?:boolean};
function RecorderControl({lang,onText,onError,onBusy,holdToTalk,color,wide,disabled}:RecorderProps){
  const {l2}=useT();const recorder=useAudioRecorder(options);const status=useAudioRecorderState(recorder,250);
  const [phase,setPhase]=useState<'idle'|'recording'|'uploading'|'error'>('idle');const [error,setError]=useState('');
  const busy=useRef(false);const released=useRef(false);const starting=useRef(false);const mounted=useRef(true);const limit=useRef<ReturnType<typeof setTimeout>|undefined>(undefined);
  useEffect(()=>()=>{mounted.current=false;clearTimeout(limit.current);if(busy.current)void recorder.stop().then(()=>{if(recorder.uri)try{new File(recorder.uri).delete()}catch{}}).catch(()=>{});},[recorder]);
  const stop=async()=>{if(!busy.current)return;busy.current=false;clearTimeout(limit.current);setPhase('uploading');onBusy?.(true);try{await recorder.stop();const uri=recorder.uri;if(!uri)throw new Error('No recording');const result=await transcribe(uri,lang);if(mounted.current){onText(result.text);setPhase('idle');}}catch(e){if(mounted.current){onError?.(e);setError(String(e));setPhase('error');}}finally{onBusy?.(false);void setAudioModeAsync({allowsRecording:false}).catch(()=>{});}};
  const toggle=async()=>{if(busy.current){await stop();return}if(phase==='uploading')return;
    try{const permission=await requestRecordingPermissionsAsync();if(!permission.granted){const message=l2('Microphone access is off. Enable it in device settings to record, or type instead.','麦克风权限已关闭。请在设备设置中启用，或直接输入文字。');setError(message);onError?.(message);setPhase('error');return}
      await setAudioModeAsync({allowsRecording:true,playsInSilentMode:true});await recorder.prepareToRecordAsync();recorder.record();busy.current=true;setPhase('recording');limit.current=setTimeout(()=>void stop(),120000);if(released.current)void stop();
    }catch(e){onError?.(e);setError(String(e));setPhase('error');}
    finally{starting.current=false}
  };
  const startHold=()=>{released.current=false;starting.current=true;void toggle()};
  const endHold=()=>{released.current=true;if(busy.current)void stop()};
  return <View style={{alignItems:'center',gap:8,paddingVertical:wide?0:12,width:wide?'100%':undefined}}><Pressable accessibilityRole="button" accessibilityLabel={phase==='recording'?l2('Stop recording','停止录音'):l2('Start recording','开始录音')} disabled={phase==='uploading'||disabled} onPress={holdToTalk?undefined:()=>void toggle()} onPressIn={holdToTalk?startHold:undefined} onPressOut={holdToTalk?endHold:undefined} style={{width:wide?'100%':76,height:wide?58:76,borderRadius:wide?12:38,backgroundColor:phase==='recording'?colors.mic:color||colors.fg,alignItems:'center',justifyContent:'center',flexDirection:'row',gap:10,borderWidth:phase==='recording'?4:0,borderColor:colors.line}}><Icon name="mic" size={wide?24:32} color="#fff"/>{wide&&<Text style={{color:'#fff',fontWeight:'700',fontSize:18}}>{l2('Hold to speak','按住说话')}</Text>}</Pressable><Text style={[text,{fontSize:13,color:colors.muted}]}>{phase==='recording'?`${l2('Tap to stop','点击停止')} · ${Math.floor(status.durationMillis/1000)} / 120 s  ${status.metering!=null?'▮'.repeat(Math.max(1,Math.min(5,Math.round((status.metering+60)/12)))):''}`:phase==='uploading'?l2('Transcribing…','正在转写…'):phase==='error'?error:holdToTalk?l2('Hold to speak','按住说话'):l2('Tap to record (up to 2 minutes)','点击录音（最长两分钟）')}</Text></View>;
}
export default function NativeRecorder(props:RecorderProps){return Platform.OS==='web'?null:<RecorderControl {...props}/>;}
