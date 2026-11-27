import {createHoldSession,MAX_RECORDING_MS,type HoldDeps,type HoldPhase,type HoldResult} from './holdSession';

// UAT 3: every mic is hold to speak, release to finish.
function setup(over:Partial<HoldDeps>={}){
  let t=0;const phases:HoldPhase[]=[];const results:HoldResult[]=[];
  const deps={signedIn:jest.fn(()=>true),permission:jest.fn(async()=>({granted:true,canAskAgain:true})),requestPermission:jest.fn(async()=>({granted:true})),
    begin:jest.fn(async()=>{}),end:jest.fn(async()=>'file:///a.wav'),discard:jest.fn(),transcribe:jest.fn(async()=>' hello '),idle:jest.fn(),now:()=>t,...over};
  const s=createHoldSession(deps,{onPhase:p=>phases.push(p),onResult:r=>results.push(r)});
  return {s,deps,phases,results,advance:(ms:number)=>{t+=ms;jest.advanceTimersByTime(ms)}};
}
const flush=async()=>{for(let i=0;i<10;i++)await Promise.resolve()};
beforeEach(()=>jest.useFakeTimers());
afterEach(()=>jest.useRealTimers());

test('hold records, release transcribes and returns the words',async()=>{
  const {s,deps,phases,results,advance}=setup();
  expect(await s.press('en')).toBe('ok');expect(s.phase()).toBe('recording');
  expect(await s.press('en')).toBe('busy');
  advance(3000);s.release();await flush();
  expect(deps.transcribe).toHaveBeenCalledWith('file:///a.wav','en');
  expect(results).toEqual([{kind:'text',text:'hello'}]);
  expect(phases).toEqual(['starting','recording','transcribing','idle']);
  expect(deps.idle).toHaveBeenCalled();
});
test('a short press (accidental tap) is deleted, never uploaded',async()=>{
  const {s,deps,results,advance}=setup();
  await s.press('en');advance(200);s.release();await flush();
  expect(deps.transcribe).not.toHaveBeenCalled();expect(deps.discard).toHaveBeenCalledWith('file:///a.wav');
  expect(results).toEqual([{kind:'short'}]);expect(s.phase()).toBe('idle');
});
test('releasing before recording has started stops it as soon as it starts',async()=>{
  let go=()=>{};const {s,deps,results}=setup({begin:jest.fn(()=>new Promise<void>(r=>{go=r}))});
  const started=s.press('zh');await flush();expect(s.phase()).toBe('starting');
  s.release();go();expect(await started).toBe('ok');await flush();
  expect(deps.transcribe).not.toHaveBeenCalled();expect(results).toEqual([{kind:'short'}]);expect(s.phase()).toBe('idle');
});
test('the permission prompt takes the touch: no recording this time',async()=>{
  const {s,deps,phases}=setup({permission:jest.fn(async()=>({granted:false,canAskAgain:true}))});
  expect(await s.press('en')).toBe('asked');expect(deps.begin).not.toHaveBeenCalled();expect(phases.at(-1)).toBe('idle');
  s.release();await flush();expect(deps.end).not.toHaveBeenCalled();
  const denied=setup({permission:jest.fn(async()=>({granted:false,canAskAgain:false}))});
  expect(await denied.s.press('en')).toBe('denied');expect(denied.deps.requestPermission).not.toHaveBeenCalled();expect(denied.s.phase()).toBe('idle');
  const refused=setup({permission:jest.fn(async()=>({granted:false,canAskAgain:true})),requestPermission:jest.fn(async()=>({granted:false}))});
  expect(await refused.s.press('en')).toBe('denied');expect(refused.s.phase()).toBe('idle');
});
test('signed out and start failures leave the mic idle',async()=>{
  const off=setup({signedIn:()=>false});expect(await off.s.press('en')).toBe('offline');expect(off.deps.permission).not.toHaveBeenCalled();expect(off.s.phase()).toBe('idle');
  const bad=setup({begin:jest.fn(async()=>{throw new Error('busy')})});expect(await bad.s.press('en')).toBe('error');expect(bad.s.phase()).toBe('idle');
  expect(await bad.s.press('en')).toBe('error'); // can try again
});
test('120 s limit stops and transcribes without a release',async()=>{
  const {s,deps,results,advance}=setup();
  await s.press('yue');advance(MAX_RECORDING_MS);await flush();
  expect(deps.transcribe).toHaveBeenCalledWith('file:///a.wav','yue');expect(results).toEqual([{kind:'text',text:'hello'}]);
  s.release();await flush();expect(deps.end).toHaveBeenCalledTimes(1);
});
test('unmount while recording stops and deletes without uploading',async()=>{
  const {s,deps,results,advance}=setup();
  await s.press('en');advance(5000);s.cancel();await flush();
  expect(deps.end).toHaveBeenCalled();expect(deps.discard).toHaveBeenCalled();expect(deps.transcribe).not.toHaveBeenCalled();expect(results).toEqual([]);
  advance(MAX_RECORDING_MS);await flush();expect(deps.transcribe).not.toHaveBeenCalled();
});
test('unmount while starting stops the recorder once it starts',async()=>{
  let go=()=>{};const {s,deps,results}=setup({begin:jest.fn(()=>new Promise<void>(r=>{go=r}))});
  const started=s.press('en');await flush();s.cancel();go();await started;await flush();
  expect(deps.end).toHaveBeenCalled();expect(deps.discard).toHaveBeenCalled();expect(deps.transcribe).not.toHaveBeenCalled();expect(results).toEqual([]);
});
test('unmount during transcription drops the result',async()=>{
  let done=(_:string)=>{};const {s,results,advance}=setup({transcribe:jest.fn(()=>new Promise<string>(r=>{done=r}))});
  await s.press('en');advance(2000);s.release();await flush();s.cancel();done('late');await flush();
  expect(results).toEqual([]);
});
test('transcription errors and silence are reported and the mic returns to idle',async()=>{
  const err=new Error('HTTP 503');const bad=setup({transcribe:jest.fn(async()=>{throw err})});
  await bad.s.press('en');bad.advance(2000);bad.s.release();await flush();
  expect(bad.results).toEqual([{kind:'error',error:err}]);expect(bad.s.phase()).toBe('idle');
  const quiet=setup({transcribe:jest.fn(async()=>'  ')});
  await quiet.s.press('en');quiet.advance(2000);quiet.s.release();await flush();
  expect(quiet.results).toEqual([{kind:'empty'}]);
});
