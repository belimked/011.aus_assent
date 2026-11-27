import {seed} from '../data/seed';
import {noteOn} from './index';
import {commitDK,applyPreference} from './dk';
import {setTodosDone} from './todo';

// The server writes each new contact note to Rex once, keyed by its id (rexcontact_notes.go).
test('contact notes carry a stable id; activity logs are marked sys and keep the property',()=>{
  const s=seed();const c=s.contacts[0];const p=s.properties[0];
  noteOn(s,c,'Teddy owns a lot of homes in Brisbane, treated as VIP.');
  noteOn(s,c,'Teddy owns a lot of homes in Brisbane, treated as VIP.');
  const [a,b]=c.notes;
  expect(a.id).toMatch(/^cn[a-z0-9]+$/);expect(a.id).not.toBe(b.id);expect(a.at).toBeTruthy();expect(a.sys).toBeUndefined();expect(a.propId).toBeUndefined();
  applyPreference(s,c,'sms',0);
  expect(c.notes[0]).toMatchObject({sys:'preference'});
  const t=s.todos.find(x=>!x.done)!;t.cid=c.id;setTodosDone(s,[t.id],true);
  expect(c.notes[0]).toMatchObject({a:'AxleoCollect',sys:'todo'});expect(c.notes[0].id).toBeTruthy();
  const dk=commitDK(s,p.id,{first:'Teddie',last:'Bear',mobile:'0412 000 777',email:'',relation:'',note:'Keen to sell next year',reason:'',tags:[]},'new');
  expect(dk.notes[0]).toMatchObject({t:'Keen to sell next year',propId:p.id});expect(dk.notes[0].sys).toBeUndefined();
});
