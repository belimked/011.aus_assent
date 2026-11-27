import type {VoiceLang} from '../lib/voice';

/** Press-and-hold recording, shared by every mic in the app: press starts, release (or 120 s) stops and transcribes. */
export type HoldPhase='idle'|'starting'|'recording'|'transcribing';
export type HoldStart='ok'|'offline'|'asked'|'denied'|'error'|'busy';
export type HoldResult={kind:'text';text:string}|{kind:'short'}|{kind:'empty'}|{kind:'error';error:unknown};
/** Shorter than this is treated as nothing said (accidental tap): the file is deleted, never uploaded. */
export const MIN_SPEECH_MS=600;
export const MAX_RECORDING_MS=120000;
export type HoldDeps={
  signedIn:()=>boolean;
  permission:()=>Promise<{granted:boolean;canAskAgain:boolean}>;
  requestPermission:()=>Promise<{granted:boolean}>;
  /** Audio mode on, prepare, record. */
  begin:()=>Promise<void>;
  /** Stop; the recorded file URI, if any. */
  end:()=>Promise<string|null|undefined>;
  discard:(uri:string)=>void;
  /** Uploads and deletes the file. */
  transcribe:(uri:string,lang:VoiceLang)=>Promise<string>;
  /** Audio mode off. */
  idle:()=>void;
  now:()=>number;
};
export type HoldEvents={onPhase:(phase:HoldPhase)=>void;onResult:(result:HoldResult)=>void};

export function createHoldSession(deps:HoldDeps,events:HoldEvents,opts:{minMs?:number;maxMs?:number}={}){
  const minMs=opts.minMs??MIN_SPEECH_MS,maxMs=opts.maxMs??MAX_RECORDING_MS;
  let phase:HoldPhase='idle';let held=false;let startedAt=0;let lang:VoiceLang='en';let gen=0;let limit:ReturnType<typeof setTimeout>|undefined;
  const set=(p:HoldPhase)=>{phase=p;events.onPhase(p)};
  const stopQuietly=()=>{void deps.end().then(uri=>{if(uri)deps.discard(uri)}).catch(()=>{}).finally(deps.idle)};
  const release=()=>{
    held=false;if(phase!=='recording')return; // still starting: press() finishes the release once recording begins
    clearTimeout(limit);const my=gen;const spoke=deps.now()-startedAt;set('transcribing');
    void (async():Promise<HoldResult>=>{
      const uri=await deps.end();if(!uri)return {kind:'empty'};
      if(spoke<minMs){deps.discard(uri);return {kind:'short'}}
      const text=(await deps.transcribe(uri,lang)).trim();return text?{kind:'text',text}:{kind:'empty'};
    })().catch((error:unknown):HoldResult=>({kind:'error',error})).then(result=>{deps.idle();if(my!==gen)return;set('idle');events.onResult(result)});
  };
  const press=async(spoken:VoiceLang):Promise<HoldStart>=>{
    if(phase!=='idle')return 'busy';
    held=true;lang=spoken;
    if(!deps.signedIn())return 'offline';
    const my=gen;set('starting');
    try{
      const permission=await deps.permission();
      if(!permission.granted){
        // The system prompt takes the touch, so this hold cannot record; the next one will.
        const r=permission.canAskAgain?((await deps.requestPermission()).granted?'asked':'denied'):'denied';
        if(my===gen)set('idle');return r;
      }
      await deps.begin();
    }catch{if(my===gen)set('idle');stopQuietly();return 'error'}
    if(my!==gen){stopQuietly();return 'error'} // cancelled (unmounted) while starting
    startedAt=deps.now();set('recording');limit=setTimeout(release,maxMs);
    if(!held)release();
    return 'ok';
  };
  /** Unmount: stop without uploading and drop any pending result. */
  const cancel=()=>{gen++;held=false;clearTimeout(limit);const was=phase;phase='idle';if(was==='recording')stopQuietly()};
  return {press,release,cancel,phase:()=>phase};
}
