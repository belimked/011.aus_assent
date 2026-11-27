import {useEffect,useRef,useState} from 'react';
import {AppState} from 'react-native';
import {getRecordingPermissionsAsync,requestRecordingPermissionsAsync,setAudioModeAsync,useAudioRecorder,useAudioRecorderState} from 'expo-audio';
import {File} from 'expo-file-system';
import {recordingOptions} from './recordingOptions';
import {createHoldSession,type HoldPhase,type HoldResult,type HoldStart} from './holdSession';
import {transcribe,type VoiceLang} from '../lib/voice';
import {authHeader} from '../store/sync';
export {MIN_SPEECH_MS,MAX_RECORDING_MS,type HoldResult,type HoldStart} from './holdSession';

type L2=(en:string,zh:string)=>string;
/** Toast text for a hold that could not record; '' when it did (or was a repeat press). */
export const holdStartText=(r:HoldStart,l2:L2)=>r==='ok'||r==='busy'?'':r==='asked'?l2('Microphone ready. Hold to speak, release to finish.','麦克风已开启。按住说话，松开结束。'):r==='offline'?l2('Sign in with your email to use voice. Type instead.','请用邮箱登录后使用语音，或直接输入。'):r==='denied'?l2('Microphone access is off. Enable it in Settings, or type instead.','麦克风权限已关闭，请在设置中开启，或直接输入。'):l2('Could not start recording. Try again, or type instead.','无法开始录音，请重试或直接输入。');
/** A press too short to be speech. */
export const holdShortText=(l2:L2)=>l2('Hold to speak, release to finish.','按住说话，松开结束。');

/**
 * Press-and-hold recording for the caller's own Pressable (no Modal while holding:
 * presenting a Modal cancels the touch on iOS). Spread `pressProps` on the Pressable.
 * Releasing, sliding off, a parent taking the touch, the app leaving the foreground or 120 s all stop it;
 * unmounting stops it without uploading.
 */
export function useHoldRecorder({lang,onResult,onStart}:{lang:()=>VoiceLang;onResult:(r:HoldResult)=>void;onStart?:(r:HoldStart)=>void}){
  const recorder=useAudioRecorder(recordingOptions);const status=useAudioRecorderState(recorder,120);
  const [phase,setPhase]=useState<HoldPhase>('idle');
  const handlers=useRef({lang,onResult,onStart});useEffect(()=>{handlers.current={lang,onResult,onStart}});
  const live=useRef(recorder);useEffect(()=>{live.current=recorder},[recorder]);
  // The session only reads the refs inside its callbacks (touch, timer and audio events), never during render.
  // eslint-disable-next-line react-hooks/refs
  const [s]=useState(()=>createHoldSession({
    signedIn:()=>!!authHeader(),
    permission:getRecordingPermissionsAsync,
    requestPermission:requestRecordingPermissionsAsync,
    begin:async()=>{await setAudioModeAsync({allowsRecording:true,playsInSilentMode:true});await live.current.prepareToRecordAsync();live.current.record()},
    end:async()=>{await live.current.stop();return live.current.uri},
    discard:uri=>{try{new File(uri).delete()}catch{}},
    transcribe:async(uri,l)=>(await transcribe(uri,l)).text, // transcribe() deletes the file
    idle:()=>{void setAudioModeAsync({allowsRecording:false}).catch(()=>{})},
    now:Date.now,
  },{onPhase:setPhase,onResult:r=>handlers.current.onResult(r)}));
  useEffect(()=>{const sub=AppState.addEventListener('change',st=>{if(st!=='active')s.release()});return()=>{sub.remove();s.cancel()}},[s]);
  const press=async()=>{const r=await s.press(handlers.current.lang());handlers.current.onStart?.(r)};
  // cancelable={false}: a parent ScrollView must not steal the touch mid-sentence.
  const pressProps={onPressIn:()=>{void press()},onPressOut:()=>s.release(),cancelable:false};
  // 'starting' reads as idle: the mic is not live yet.
  const shown:'idle'|'recording'|'transcribing'=phase==='starting'?'idle':phase;
  return {phase:shown,pressProps,durationMs:status.durationMillis||0,metering:status.metering};
}
