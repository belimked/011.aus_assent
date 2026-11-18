import {Image,Platform} from 'react-native';
import {File,Paths} from 'expo-file-system';
import {manipulateAsync,SaveFormat} from 'expo-image-manipulator';
import {apiUrl,authHeader} from '../store/sync';
import {getState,update} from '../store';
import {VoiceError,type AIResult,type VoiceLang} from './voice';

/** Keep the prepared file in documents so an offline upload survives cache eviction. */
export async function preparePhoto(uri:string,persist=false):Promise<string>{
  const {width,height}=await new Promise<{width:number;height:number}>((resolve,reject)=>Image.getSize(uri,(width,height)=>resolve({width,height}),reject));
  const scale=Math.min(1,1600/Math.max(width,height));
  const resized=await manipulateAsync(uri,scale<1?[{resize:{width:Math.round(width*scale),height:Math.round(height*scale)}}]:[],{compress:0.7,format:SaveFormat.JPEG});
  if(!persist||Platform.OS==='web')return resized.uri;
  const target=new File(Paths.document,`photo-${Date.now()}-${Math.random().toString(36).slice(2)}.jpg`);
  new File(resized.uri).copy(target);
  return target.uri;
}
function multipart(file:Uint8Array,fields:Record<string,string>){
  const boundary=`----AxleoBoundary${Date.now().toString(36)}${Math.random().toString(36).slice(2)}`;
  const enc=new TextEncoder();let head='';
  for(const [key,value] of Object.entries(fields))head+=`--${boundary}\r\nContent-Disposition: form-data; name="${key}"\r\n\r\n${value}\r\n`;
  head+=`--${boundary}\r\nContent-Disposition: form-data; name="file"; filename="photo.jpg"\r\nContent-Type: image/jpeg\r\n\r\n`;
  const a=enc.encode(head),b=enc.encode(`\r\n--${boundary}--\r\n`),body=new Uint8Array(a.length+file.length+b.length);
  body.set(a);body.set(file,a.length);body.set(b,a.length+file.length);
  return {body,contentType:`multipart/form-data; boundary=${boundary}`};
}
async function send(path:string,uri:string,fields:Record<string,string>,limit:number){
  const auth=authHeader();if(!auth)throw new VoiceError('offline','Sign in required');
  const bytes=Platform.OS==='web'?new Uint8Array(await (await fetch(uri)).arrayBuffer()):await new File(uri).bytes();
  if(bytes.length>limit)throw new Error('Photo is too large');
  const {body,contentType}=multipart(bytes,fields);
  try{
    const response=await fetch(`${apiUrl}${path}`,{method:'POST',headers:{...auth,'Content-Type':contentType},body:body as unknown as BodyInit});
    if(!response.ok)throw new VoiceError(response.status===401?'offline':response.status===503?'unavailable':response.status===429?'rateLimit':'upstream',`HTTP ${response.status}`);
    return response.json();
  }catch(e){if(e instanceof VoiceError)throw e;throw new VoiceError('network',String(e));}
}
export function uploadPhoto(uri:string,propertyId?:string,id?:string):Promise<{id:string;propertyId:string;mime:string;size:number;createdAt:string}>{
  return send('/photos',uri,{...(propertyId?{propertyId}:{}),...(id?{id}:{})},8*1024*1024);
}
export function aiImage(uri:string,ctx:'card'|'note'|'enq',lang?:VoiceLang):Promise<AIResult>{
  return send('/ai/image',uri,{ctx,...(lang?{lang}:{})},5*1024*1024);
}
const inProgress=new Set<string>();
export async function uploadPendingPhotos(){
  if(!authHeader())return;
  const pending=getState().properties.flatMap(p=>(p.photos||[]).filter(photo=>!photo.uploaded&&photo.uri?.startsWith('file://')).map(photo=>({propertyId:p.id,...photo})));
  for(const photo of pending){
    if(inProgress.has(photo.id))continue;
    inProgress.add(photo.id);
    try{
      if(!new File(photo.uri!).exists)continue;
      await uploadPhoto(photo.uri!,photo.propertyId,photo.id);
      update(d=>{const item=d.properties.find(p=>p.id===photo.propertyId)?.photos?.find(x=>x.id===photo.id);if(item)item.uploaded=true});
    }catch{/* Keep pending for the next sync. */}finally{inProgress.delete(photo.id)}
  }
}
