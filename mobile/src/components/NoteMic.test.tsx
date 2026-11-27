import React,{act} from 'react';
import {createRoot,type TestInstance} from 'test-renderer';
import {NoteMic} from './NoteMic';

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

it('dictates in the language passed by the sheet, not the app language, and appends the words',async()=>{
  const onChangeText=jest.fn();const root=createRoot();
  await act(async()=>root.render(<NoteMic value="Spoke to owner" onChangeText={onChangeText} lang="yue"/>));
  await press(root,'onPressIn');jest.setSystemTime(Date.now()+2000);await press(root,'onPressOut');
  expect(mockTranscribe).toHaveBeenCalledWith('file:///r.wav','yue');
  expect(onChangeText).toHaveBeenCalledWith('Spoke to owner Mary Smith。');
  await act(async()=>root.unmount());
});
it('without a lang it keeps using the app language',async()=>{
  const root=createRoot();
  await act(async()=>root.render(<NoteMic value="" onChangeText={jest.fn()}/>));
  await press(root,'onPressIn');jest.setSystemTime(Date.now()+2000);await press(root,'onPressOut');
  expect(mockTranscribe).toHaveBeenCalledWith('file:///r.wav','en');
  await act(async()=>root.unmount());
});
