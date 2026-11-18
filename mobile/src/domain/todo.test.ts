import {seed} from '../data/seed';
import {reassignTodos,setTodosDone,revertTodos} from './todo';
import {diff,merge,type Pull} from '../store/sync-logic';

const withContact=()=>{const s=seed();const c=s.contacts[0];const t=s.todos.find(x=>!x.done)!;t.cid=c.id;t.who=s.auth?.name||'Owen Chen';return {s,c,t};};

test('reassign applies immediately, notes the contact, keeps it under Assigned by me, and undoes',()=>{
  const {s,c,t}=withContact();const before=structuredClone(s);const notes=c.notes.length;const originalWho=t.who;
  expect(reassignTodos(s,[t.id],'Natalie Wong','u-nat')).toBe(1);
  const x=s.todos.find(y=>y.id===t.id)!;
  expect(x).toMatchObject({who:'Natalie Wong',assigneeId:'u-nat',by:s.auth?.name||'Owen Chen'});
  expect(s.contacts.find(y=>y.id===c.id)!.notes.length).toBe(notes+1);
  expect(reassignTodos(s,[t.id],'Natalie Wong')).toBe(0);
  revertTodos(before,[t.id])(s);
  expect(s.todos.find(y=>y.id===t.id)!.who).toBe(originalWho);
  expect(s.contacts.find(y=>y.id===c.id)!.notes.length).toBe(notes);
});

test('done sets doneAt, notes the contact and reopens',()=>{
  const {s,c,t}=withContact();const notes=c.notes.length;
  expect(setTodosDone(s,[t.id],true)).toBe(1);
  expect(s.todos.find(y=>y.id===t.id)).toMatchObject({done:true});
  expect(s.todos.find(y=>y.id===t.id)!.doneAt).toBeTruthy();
  expect(s.contacts.find(y=>y.id===c.id)!.notes[0].t).toContain('To-do done');
  expect(s.contacts.find(y=>y.id===c.id)!.notes.length).toBe(notes+1);
  setTodosDone(s,[t.id],false);
  expect(s.todos.find(y=>y.id===t.id)!.doneAt).toBeUndefined();
});

test('activity entries sync as their own kind and survive a fresh pull',()=>{
  const a=seed();const b=structuredClone(a);
  b.audit.unshift({id:'a1',d:'2026-11-07 10:00',a:'To-do done',t:'x'},{d:'legacy',a:'old',t:'no id'});
  const ops=diff(a,b,()=>'op').filter(o=>o.kind==='activity');
  expect(ops).toMatchObject([{kind:'activity',op:'upsert',id:'a1'}]);
  const pull:Pull={cursor:'1',full:true,deleted:{},activity:[{id:'a2',d:'2026-11-06 09:00',a:'Note',t:'y',version:1},{id:'a1',d:'2026-11-07 10:00',a:'To-do done',t:'x',version:1}]};
  expect(merge(b,pull,{},[]).audit.map(x=>x.id)).toEqual(['a1','a2']);
});
