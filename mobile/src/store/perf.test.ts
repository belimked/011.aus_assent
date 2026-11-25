import AsyncStorage from '@react-native-async-storage/async-storage';
import {diff} from './sync-logic';
import {cancelStateWrite,getState,onLocalWrite,replaceState,update} from './index';
import type {AppState} from '../data/types';

jest.mock('../lib/notifications',()=>({reconcileReminders:jest.fn()}));
jest.mock('@react-native-async-storage/async-storage',()=>({__esModule:true,default:{setItem:jest.fn(async()=>{}),getItem:jest.fn(async()=>null)}}));
const storage=AsyncStorage.setItem as jest.Mock;
const base=()=>({...getState(),properties:[{id:'p1',street:'Main',version:3,notes:[]}],contacts:[],todos:[]}) as unknown as AppState;

beforeEach(()=>{jest.useFakeTimers();cancelStateWrite();storage.mockClear();replaceState(base());cancelStateWrite();onLocalWrite(null);});
afterEach(()=>{cancelStateWrite();onLocalWrite(null);jest.useRealTimers();});

test('unchanged collections share identity, changed property is a sync upsert',()=>{
  const before=getState();const write=jest.fn((a:AppState,b:AppState)=>diff(a,b,()=> 'op1'));onLocalWrite(write);
  update(d=>{d.properties[0].street='New';});
  const after=getState();
  expect(after.properties).not.toBe(before.properties);
  expect(after.properties[0]).not.toBe(before.properties[0]);
  expect(after.contacts).toBe(before.contacts);
  expect(after.todos).toBe(before.todos);
  expect(before.properties[0].street).toBe('Main');
  expect(write.mock.results[0].value).toEqual([{opId:'op1',kind:'properties',op:'upsert',id:'p1',baseVersion:3,data:expect.objectContaining({street:'New'})}]);
});

test('persistence coalesces edits and writes latest snapshot',async()=>{
  update(d=>{d.properties[0].street='First';});
  jest.advanceTimersByTime(400);
  update(d=>{d.properties[0].street='Second';});
  jest.advanceTimersByTime(799);
  expect(storage).not.toHaveBeenCalled();
  jest.advanceTimersByTime(1);
  await Promise.resolve();await Promise.resolve();
  expect(storage).toHaveBeenCalledTimes(1);
  expect(JSON.parse(storage.mock.calls[0][1]).properties[0].street).toBe('Second');
});

test('40k property benchmark (informational; no timing assertion)',()=>{
  jest.useRealTimers();
  const properties=Array.from({length:40000},(_,i)=>({id:`p${i}`,street:`Street ${i}`,version:1,notes:[]}));
  replaceState({...getState(),properties} as unknown as AppState);cancelStateWrite();
  const before=getState();
  const start=process.hrtime.bigint();const old=structuredClone(before);old.properties[20000].street='Changed';
  const cloneMs=Number(process.hrtime.bigint()-start)/1e6;
  const stringifyStart=process.hrtime.bigint();JSON.stringify(old);const stringifyMs=Number(process.hrtime.bigint()-stringifyStart)/1e6;
  const nextStart=process.hrtime.bigint();update(d=>{d.properties[20000].street='Changed';});const updateMs=Number(process.hrtime.bigint()-nextStart)/1e6;
  console.log(`40k properties: old clone ${cloneMs.toFixed(1)}ms + stringify ${stringifyMs.toFixed(1)}ms; COW update ${updateMs.toFixed(1)}ms (persistence deferred)`);
});
