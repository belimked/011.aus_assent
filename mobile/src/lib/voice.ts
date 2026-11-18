import {Platform} from 'react-native';
import {File} from 'expo-file-system';
import {apiUrl,authHeader} from '../store/sync';

export type VoiceLang='en'|'zh'|'yue';
export type VoiceCtx='dk'|'oh'|'enq'|'note';
export type AIFields={contactName:string|null;firstName:string|null;lastName:string|null;phone:string|null;email:string|null;address:string|null;suburb:string|null;intent:'buy'|'sell'|'rent'|'appraisal'|'other'|null;priceMin:number|null;priceMax:number|null;timeframe:string|null;notes:string|null;language:string|null};
export type AIResult={fields:AIFields;confidence:Partial<Record<keyof AIFields,number>>;model:string;latencyMs:number};
export class VoiceError extends Error {
  constructor(public kind:'offline'|'timeout'|'unavailable'|'rateLimit'|'upstream'|'network',message:string){super(message);this.name='VoiceError';}
}
async function request(path:string,body:BodyInit,timeout:number,contentType?:string){
  const auth=authHeader();
  if(!auth)throw new VoiceError('offline','Sign in to use voice services');
  const controller=new AbortController();const timer=setTimeout(()=>controller.abort(),timeout);
  try {
    const response=await fetch(`${apiUrl}${path}`,{method:'POST',headers:{...auth,...(contentType?{'Content-Type':contentType}:{})},body,signal:controller.signal});
    if(!response.ok)throw new VoiceError(response.status===503?'unavailable':response.status===429?'rateLimit':'upstream',`HTTP ${response.status}`);
    return response.json();
  }catch(e){if(e instanceof VoiceError)throw e;throw new VoiceError(controller.signal.aborted?'timeout':'network',String(e));}
  finally{clearTimeout(timer);}
}
export async function transcribe(uri:string,lang:VoiceLang):Promise<{text:string;lang:string;durationMs:number;latencyMs:number}>{
  const data=new FormData();const ext=Platform.OS==='ios'?'wav':uri.split('?')[0].split('.').pop()||'m4a';
  // RN requires a file descriptor; fetch must set the multipart boundary itself.
  data.append('audio',{uri,name:`recording.${ext}`,type:ext==='wav'?'audio/wav':ext==='webm'?'audio/webm':'audio/mp4'} as unknown as Blob);
  data.append('lang',lang);
  const result=await request('/stt/transcribe',data,30000);
  if(Platform.OS!=='web')try{new File(uri).delete()}catch{} // Only discard after a successful response.
  return result;
}
export async function aiExtract(text:string,ctx:VoiceCtx,lang?:VoiceLang):Promise<AIResult>{
  return request('/ai/extract',JSON.stringify({text,ctx,...(lang?{lang}:{})}),20000,'application/json');
}
