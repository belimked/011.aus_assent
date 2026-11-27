import React,{act} from 'react';
import {AppState} from 'react-native';
import {createRoot} from 'test-renderer';
import {useHoldRecorder,type HoldResult} from './useHoldRecorder';

const mockRecorder={prepareToRecordAsync:jest.fn(async()=>{}),record:jest.fn(),stop:jest.fn(async()=>{}),uri:'file:///r.wav'};
jest.mock('expo-audio',()=>({useAudioRecorder:()=>mockRecorder,useAudioRecorderState:()=>({durationMillis:0}),getRecordingPermissionsAsync:jest.fn(async()=>({granted:true,canAskAgain:true})),requestRecordingPermissionsAsync:jest.fn(async()=>({granted:true})),setAudioModeAsync:jest.fn(async()=>{}),RecordingPresets:{HIGH_QUALITY:{}},IOSOutputFormat:{}}));
const mockDeleted:string[]=[];
jest.mock('expo-file-system',()=>({File:class{mockUri:string;constructor(u:string){this.mockUri=u}delete(){mockDeleted.push(this.mockUri)}}}));
const mockTranscribe=jest.fn(async()=>({text:'hi'}));
jest.mock('../lib/voice',()=>({transcribe:(...a:unknown[])=>mockTranscribe(...(a as []))}));
jest.mock('../store/sync',()=>({authHeader:()=>({Authorization:'x'})}));
let appState:((s:string)=>void)|undefined;
jest.spyOn(AppState,'addEventListener').mockImplementation(((_:string,fn:(s:string)=>void)=>{appState=fn;return {remove:()=>{appState=undefined}}}) as never);

let hook:ReturnType<typeof useHoldRecorder>;const results:HoldResult[]=[];
function Probe({onHook}:{onHook:(h:typeof hook)=>void}){onHook(useHoldRecorder({lang:()=>'en',onResult:r=>results.push(r)}));return null}
// act() ships only in React's development build (jest's default NODE_ENV=test); skip rather than fail under NODE_ENV=production.
const it=typeof act==='function'?test:test.skip;
const flush=()=>act(async()=>{for(let i=0;i<10;i++)await Promise.resolve()});
beforeEach(()=>{jest.useFakeTimers();results.length=0;mockDeleted.length=0;jest.clearAllMocks()});
afterEach(()=>jest.useRealTimers());

it('pressProps: press in records, press out transcribes; parent scroll cannot cancel the hold',async()=>{
  const root=createRoot();await act(async()=>root.render(<Probe onHook={h=>{hook=h}}/>));
  expect(hook.pressProps.cancelable).toBe(false);
  hook.pressProps.onPressIn();await flush();expect(hook.phase).toBe('recording');expect(mockRecorder.record).toHaveBeenCalled();
  jest.setSystemTime(Date.now()+2000);hook.pressProps.onPressOut();await flush();
  expect(mockTranscribe).toHaveBeenCalledWith('file:///r.wav','en');expect(results).toEqual([{kind:'text',text:'hi'}]);expect(hook.phase).toBe('idle');
  await act(async()=>root.unmount());
});
it('leaving the app while holding stops the recording instead of leaving it live',async()=>{
  const root=createRoot();await act(async()=>root.render(<Probe onHook={h=>{hook=h}}/>));
  hook.pressProps.onPressIn();await flush();jest.setSystemTime(Date.now()+2000);
  appState?.('background');await flush();
  expect(mockRecorder.stop).toHaveBeenCalled();expect(results).toEqual([{kind:'text',text:'hi'}]);expect(hook.phase).toBe('idle');
  await act(async()=>root.unmount());
});
it('unmounting mid-hold stops and deletes the file without uploading',async()=>{
  const root=createRoot();await act(async()=>root.render(<Probe onHook={h=>{hook=h}}/>));
  hook.pressProps.onPressIn();await flush();
  await act(async()=>root.unmount());await flush();
  expect(mockRecorder.stop).toHaveBeenCalled();expect(mockDeleted).toEqual(['file:///r.wav']);expect(mockTranscribe).not.toHaveBeenCalled();
});
