import {Platform} from 'react-native';
import {File,Paths} from 'expo-file-system';
import {createAudioPlayer,setAudioModeAsync} from 'expo-audio';
import {apiUrl,authHeader} from '../store/sync';

export type VoiceLang='en'|'zh'|'yue';
export type SheetLang='en-AU'|'zh-CN'|'yue-HK';
export const voiceLang=(lang:SheetLang):VoiceLang=>({'en-AU':'en','zh-CN':'zh','yue-HK':'yue'}[lang] as VoiceLang);
export type VoiceCtx='dk'|'oh'|'enq'|'note';
export type AIFields={contactName:string|null;firstName:string|null;lastName:string|null;phone:string|null;email:string|null;address:string|null;suburb:string|null;intent:'buy'|'sell'|'rent'|'appraisal'|'other'|null;priceMin:number|null;priceMax:number|null;timeframe:string|null;notes:string|null;language:string|null};
export type AIResult={fields:AIFields;confidence:Partial<Record<keyof AIFields,number>>;model:string;latencyMs:number};
export class VoiceError extends Error {
  constructor(public kind:'offline'|'timeout'|'unavailable'|'rateLimit'|'upstream'|'network',message:string){super(message);this.name='VoiceError';}
}
async function requestResponse(path:string,body:BodyInit,timeout:number,contentType?:string){
  const auth=authHeader();
  if(!auth)throw new VoiceError('offline','Sign in to use voice services');
  const controller=new AbortController();const timer=setTimeout(()=>controller.abort(),timeout);
  try {
    const response=await fetch(`${apiUrl}${path}`,{method:'POST',headers:{...auth,...(contentType?{'Content-Type':contentType}:{})},body,signal:controller.signal});
    if(!response.ok)throw new VoiceError(response.status===401?'offline':response.status===503?'unavailable':response.status===429?'rateLimit':'upstream',`HTTP ${response.status}`);
    return response;
  }catch(e){if(e instanceof VoiceError)throw e;throw new VoiceError(controller.signal.aborted?'timeout':'network',String(e));}
  finally{clearTimeout(timer);}
}
async function request(path:string,body:BodyInit,timeout:number,contentType?:string){return (await requestResponse(path,body,timeout,contentType)).json()}
export async function transcribe(uri:string,lang:VoiceLang):Promise<{text:string;lang:string;durationMs:number;latencyMs:number}>{
  try{
    const data=new FormData();const ext=Platform.OS==='ios'?'wav':uri.split('?')[0].split('.').pop()||'m4a';
    // RN requires a file descriptor; fetch must set the multipart boundary itself.
    data.append('audio',{uri,name:`recording.${ext}`,type:ext==='wav'?'audio/wav':ext==='webm'?'audio/webm':'audio/mp4'} as unknown as Blob);
    data.append('lang',lang);
    return await request('/stt/transcribe',data,30000);
  }finally{if(Platform.OS!=='web')try{new File(uri).delete()}catch{}}
}
export async function aiExtract(text:string,ctx:VoiceCtx,lang?:VoiceLang):Promise<AIResult>{
  return request('/ai/extract',JSON.stringify({text,ctx,...(lang?{lang}:{})}),20000,'application/json');
}
export type Translation={text:string;from:VoiceLang;to:VoiceLang;model:string;latencyMs:number};
export async function translate(text:string,from:VoiceLang,to:VoiceLang):Promise<Translation>{
  return request('/ai/translate',JSON.stringify({text,from,to}),20000,'application/json');
}
let playback:undefined|(()=>void);
let playbackToken=0;
export function stopSpeaking(){playbackToken++;playback?.();playback=undefined;}
export async function speak(text:string,lang:VoiceLang):Promise<void>{
  stopSpeaking();const token=playbackToken;
  const response=await requestResponse('/tts',JSON.stringify({text,lang}),20000,'application/json');
  const bytes=await response.arrayBuffer();
  if(token!==playbackToken)return;
  if(Platform.OS==='web'){
    const url=URL.createObjectURL(new Blob([bytes],{type:'audio/mpeg'}));
    const audio=new Audio(url);
    const cleanup=()=>{audio.pause();audio.onended=null;audio.onerror=null;URL.revokeObjectURL(url);if(playback===cleanup)playback=undefined};
    playback=cleanup;audio.onended=cleanup;audio.onerror=cleanup;
    try{await audio.play()}catch(e){cleanup();throw new VoiceError('unavailable',String(e))}
    return;
  }
  const file=new File(Paths.cache,`tts-${Date.now()}-${Math.random().toString(36).slice(2)}.mp3`);
  try{
    await file.write(new Uint8Array(bytes));
    if(token!==playbackToken){file.delete();return}
    // After recording, iOS stays in record mode (earpiece / quiet); switch to loud playback that ignores the silent switch.
    await setAudioModeAsync({allowsRecording:false,playsInSilentMode:true});
    if(token!==playbackToken){file.delete();return}
    const player=createAudioPlayer({uri:file.uri});
    const subscription=player.addListener('playbackStatusUpdate',status=>{if(status.didJustFinish)cleanup()});
    const cleanup=()=>{subscription.remove();player.remove();try{file.delete()}catch{};if(playback===cleanup)playback=undefined};
    playback=cleanup;player.play();
  }catch(e){try{file.delete()}catch{};if(e instanceof VoiceError)throw e;throw new VoiceError('unavailable',String(e))}
}
