import {seed} from '../data/seed';
import {audit,noteOn} from './index';
import {setTodosDone} from './todo';
import {captureIdentity} from './review';

test('writes use the local calendar day after midnight without reloading modules',()=>{
  jest.useFakeTimers();
  try {
    const s=seed(); const c=s.contacts[0];
    const t=s.todos[0]; t.done=false;
    jest.setSystemTime(new Date(2026,10,23,23,59));
    audit(s,'Before','midnight');
    expect(s.audit[0].d.slice(0,10)).toBe('2026-11-23');
    jest.setSystemTime(new Date(2026,10,24,0,1));
    noteOn(s,c,'After midnight');
    audit(s,'After','midnight');
    setTodosDone(s,[t.id],true);
    expect(c.notes[0].d).toBe('2026-11-24');
    expect(s.audit[0].d.slice(0,10)).toBe('2026-11-24');
    expect(t.doneAt).toBe('2026-11-24');
  } finally {jest.useRealTimers()}
});

test('single spoken first name is used for capture edit',()=>{
  expect(captureIdentity('Mary')).toEqual({first:'Mary',last:''});
});
