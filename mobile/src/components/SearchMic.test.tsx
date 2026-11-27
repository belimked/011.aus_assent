import React,{act} from 'react';
import {createRoot,type TestInstance} from 'test-renderer';
import {SearchMic} from './SearchMic';

const mockRecorder={prepareToRecordAsync:jest.fn(async()=>{}),record:jest.fn(),stop:jest.fn(async()=>{}),uri:'file:///r.wav'};
jest.mock('expo-audio',()=>({useAudioRecorder:()=>mockRecorder,useAudioRecorderState:()=>({durationMillis:0}),getRecordingPermissionsAsync:jest.fn(async()=>({granted:true,canAskAgain:true})),requestRecordingPermissionsAsync:jest.fn(async()=>({granted:true})),setAudioModeAsync:jest.fn(async()=>{}),RecordingPresets:{HIGH_QUALITY:{}},IOSOutputFormat:{}}));
jest.mock('expo-file-system',()=>({File:class{delete(){}}}));
jest.mock('@react-native-async-storage/async-storage',()=>({__esModule:true,default:{setItem:jest.fn(async()=>{}),getItem:jest.fn(async()=>null)}}));
const mockTranscribe=jest.fn(async(..._a:unknown[])=>({text:'  Mary Smith。 '}));
jest.mock('../lib/voice',()=>({transcribe:(...a:unknown[])=>mockTranscribe(...a),voiceErrorText:()=>'error'}));
jest.mock('../store/sync',()=>({authHeader:()=>({Authorization:'x'})}));
const mockToast=jest.fn();
jest.mock('../store',()=>({showToast:(m:string)=>mockToast(m)}));

// act() ships only in React's development build (jest's default NODE_ENV=test); skip rather than fail under NODE_ENV=production.
const it=typeof act==='function'?test:test.skip;
beforeEach(()=>{jest.useFakeTimers();jest.clearAllMocks()});
afterEach(()=>jest.useRealTimers());
const press=(root:{container:TestInstance},k:'onPressIn'|'onPressOut')=>act(async()=>{micProps(root)[k]();for(let i=0;i<10;i++)await Promise.resolve()});
/** The mic Pressable's own props (the host view only carries responder handlers). */
const micProps=(root:{container:TestInstance})=>{const host=root.container.queryAll(i=>i.props.accessibilityRole==='button')[0];let f=host.unstable_fiber;while(f&&!f.memoizedProps?.onPressIn)f=f.return;return f!.memoizedProps as {onPressIn:()=>void;onPressOut:()=>void;accessibilityLabel:string}};

it('hold then release hands the cleaned words to the search callback, replacing the query',async()=>{
  const onSearch=jest.fn();const root=createRoot();
  await act(async()=>root.render(<SearchMic value="old query" onSearch={onSearch}/>));
  expect(micProps(root).accessibilityLabel).toBe('Hold to dictate');
  await press(root,'onPressIn');expect(mockRecorder.record).toHaveBeenCalled();
  expect(micProps(root).accessibilityLabel).toBe('Release to finish dictation');
  jest.setSystemTime(Date.now()+2000);await press(root,'onPressOut');
  expect(mockTranscribe).toHaveBeenCalledWith('file:///r.wav','en');
  expect(onSearch).toHaveBeenCalledTimes(1);expect(onSearch).toHaveBeenCalledWith('Mary Smith');expect(mockToast).not.toHaveBeenCalled();
  await act(async()=>root.unmount());
});
it('a hold shorter than 600 ms only shows the hold hint and leaves the query alone',async()=>{
  const onSearch=jest.fn();const root=createRoot();
  await act(async()=>root.render(<SearchMic value="" onSearch={onSearch}/>));
  await press(root,'onPressIn');jest.setSystemTime(Date.now()+200);await press(root,'onPressOut');
  expect(mockTranscribe).not.toHaveBeenCalled();expect(onSearch).not.toHaveBeenCalled();expect(mockToast).toHaveBeenCalledWith('Hold to speak, release to finish.');
  await act(async()=>root.unmount());
});
it('words that are only punctuation do not clear the query',async()=>{
  mockTranscribe.mockResolvedValueOnce({text:' 。'});
  const onSearch=jest.fn();const root=createRoot();
  await act(async()=>root.render(<SearchMic value="Mary" onSearch={onSearch}/>));
  await press(root,'onPressIn');jest.setSystemTime(Date.now()+2000);await press(root,'onPressOut');
  expect(onSearch).not.toHaveBeenCalled();expect(mockToast).toHaveBeenCalledWith('Could not hear that');
  await act(async()=>root.unmount());
});
