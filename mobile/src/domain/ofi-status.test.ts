import {openHomeStatus} from './ofi';
import {addLocalDays,today,nowMinutes} from '../lib/today';

describe('local time and open-home status',()=>{
  test('calendar date uses device local date, not UTC',()=>{
    const date=new Date(2026,10,7,23,59);
    expect(today(date)).toBe('2026-11-07');
    expect(nowMinutes(date)).toBe(23*60+59);
    expect(addLocalDays(date,1)).toBe('2026-11-08');
  });
  test('start inclusive and end exclusive, with no 90-minute grace period',()=>{
    const at=(day:number,hour:number,minute:number)=>new Date(2026,10,day,hour,minute);
    expect(openHomeStatus('2026-11-07','10:00–10:30',at(7,9,59))).toBe('later');
    expect(openHomeStatus('2026-11-07','10:00–10:30',at(7,10,0))).toBe('live');
    expect(openHomeStatus('2026-11-07','10:00–10:30',at(7,10,29))).toBe('live');
    expect(openHomeStatus('2026-11-07','10:00–10:30',at(7,10,30))).toBe('finished');
    expect(openHomeStatus('2026-11-07','11:00–11:30',at(7,22,4))).toBe('finished');
    expect(openHomeStatus('2026-11-07','10:00–10:30',at(6,22,4))).toBe('later');
    expect(openHomeStatus('2026-11-07','10:00–10:30',at(8,0,1))).toBe('finished');
  });
  test('open home spanning midnight ends on the following date',()=>{
    const status=(day:number,hour:number,minute:number)=>openHomeStatus('2026-11-07','23:30–00:30',new Date(2026,10,day,hour,minute));
    expect(status(7,23,29)).toBe('later');
    expect(status(8,0,0)).toBe('live');
    expect(status(8,0,30)).toBe('finished');
  });
});
