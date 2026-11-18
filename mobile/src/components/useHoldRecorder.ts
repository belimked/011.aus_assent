import {useEffect,useRef,useState} from 'react';
import {getRecordingPermissionsAsync,requestRecordingPermissionsAsync,setAudioModeAsync,useAudioRecorder,useAudioRecorderState} from 'expo-audio';
import {File} from 'expo-file-system';
import {recordingOptions} from './Recorder';
import {transcribe,type VoiceLang} from '../lib/voice';
import {authHeader} from '../store/sync';

export type HoldStart='ok'|'offline'|'asked'|'denied'|'error';
/** Shorter than this is treated as nothing said (accidental long-press). */
export const MIN_SPEECH_MS=600;

/**
 * Press-and-hold recording driven by the caller's own Pressable (no Modal while holding:
 * presenting a Modal cancels the touch on iOS). start() on hold, finish() on release.
 */
export function useHoldRecorder(){
  const recorder=useAudioRecorder(recordingOptions);const status=useAudioRecorderState(recorder,120);
  const [phase,setPhase]=useState<'idle'|'recording'|'transcribing'>('idle');
  const active=useRef(false);const startedAt=useRef(0);const mounted=useRef(true);
  useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;if(active.current){active.current=false;void recorder.stop().then(()=>{if(recorder.uri)try{new File(recorder.uri).delete()}catch{}}).catch(()=>{})}}},[recorder]);
  const start=async():Promise<HoldStart>=>{
    if(!authHeader())return 'offline';
    try{
      const permission=await getRecordingPermissionsAsync();
      if(!permission.granted){
        if(!permission.canAskAgain)return 'denied';
        // The system prompt steals the touch, so this hold cannot record; the next one will.
        return (await requestRecordingPermissionsAsync()).granted?'asked':'denied';
      }
      await setAudioModeAsync({allowsRecording:true,playsInSilentMode:true});
      await recorder.prepareToRecordAsync();recorder.record();
      active.current=true;startedAt.current=Date.now();if(mounted.current)setPhase('recording');return 'ok';
    }catch{return 'error'}
  };
  /** Stops and transcribes. null = not recording (already finished); '' = nothing usable said. Throws VoiceError from the API. */
  const finish=async(lang:VoiceLang):Promise<string|null>=>{
    if(!active.current)return null;
    active.current=false;const spoke=Date.now()-startedAt.current;if(mounted.current)setPhase('transcribing');
    try{
      await recorder.stop();const uri=recorder.uri;if(!uri)return '';
      if(spoke<MIN_SPEECH_MS){try{new File(uri).delete()}catch{};return ''}
      return (await transcribe(uri,lang)).text.trim(); // transcribe() deletes the file
    }finally{if(mounted.current)setPhase('idle');void setAudioModeAsync({allowsRecording:false}).catch(()=>{})}
  };
  return {phase,start,finish,durationMs:status.durationMillis||0,metering:status.metering};
}
