import {cancelStateWrite,getState,onLocalWrite,replaceState,update} from './index';
import type {AppState} from '../data/types';

jest.mock('../lib/notifications',()=>({reconcileReminders:jest.fn()}));
jest.mock('@react-native-async-storage/async-storage',()=>({__esModule:true,default:{setItem:jest.fn(async()=>{}),getItem:jest.fn(async()=>null)}}));

beforeEach(()=>{jest.useFakeTimers();replaceState({...getState(),myTags:[],contacts:[]} as AppState);cancelStateWrite();onLocalWrite(null);});
afterEach(()=>{cancelStateWrite();jest.useRealTimers();});

// UAT 3: creating a custom tag crashed because the producer returned push()'s length while mutating the draft.
test('an expression-bodied producer that mutates and returns a value does not throw',()=>{
  const write=jest.fn();onLocalWrite(write);
  expect(()=>update(d=>d.myTags.push({id:'tag-1',n:'Pool',by:'Owen Chen',d:'2026-11-26'}))).not.toThrow();
  expect(getState().myTags.map(t=>t.n)).toEqual(['Pool']);
  expect(write).toHaveBeenCalledTimes(1);
  expect(()=>update(d=>d.myTags.unshift({id:'tag-2',n:'Corner',by:'Owen Chen',d:'2026-11-26'}))).not.toThrow();
  expect(()=>update(d=>delete (d as Partial<AppState>).market)).not.toThrow();
  expect(getState().myTags.map(t=>t.n)).toEqual(['Corner','Pool']);
});
