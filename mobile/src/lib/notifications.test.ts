import {reminderDate} from './notifications';
import type {Todo} from '../data/types';
jest.mock('@react-native-async-storage/async-storage',()=>({__esModule:true,default:{getItem:jest.fn(),setItem:jest.fn()}}));
jest.mock('expo-notifications',()=>({}));
jest.mock('expo-constants',()=>({__esModule:true,default:{expoConfig:{extra:{iosPushEnabled:false}}}}));
const task=(d:string,time?:string)=>({id:'1',d,time,t:'Call',who:'Me',kind:'call'} as Todo);
describe('reminder date',()=>{
  test('requires a real local date and time in the future',()=>{
    expect(reminderDate(task('2099-02-28','09:30'))?.getHours()).toBe(9);
    expect(reminderDate(task('2099-02-30','09:30'))).toBeNull();
    expect(reminderDate(task('2099-02-28','25:00'))).toBeNull();
    expect(reminderDate(task('2099-02-28'))).toBeNull();
    expect(reminderDate(task('2000-01-01','09:30'))).toBeNull();
  });
});
