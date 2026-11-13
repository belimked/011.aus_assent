import {seed} from '../data/seed';
import {commitDK,duplicatesOf,mergeContacts} from './dk';

test('DK approval links an existing owner and creates follow-up and audit',()=>{
 const state=seed();const n=state.contacts.length;
 const c=commitDK(state,'p6',{first:'Mary',last:'Chen',mobile:'0412 555 019',email:'mary.chen@example.com',relation:'owner',note:'Market update requested',tags:['Warm'],reason:'Downsizing',followup:'2027-01-15'},'c1');
 expect(c.id).toBe('c1');expect(state.contacts).toHaveLength(n);expect(state.properties.find(p=>p.id==='p6')?.ownerIds).toContain('c1');expect(c.tags).toContain('Warm');expect(state.todos.some(t=>t.cid==='c1'&&t.d==='2027-01-15')).toBe(true);expect(state.audit[0].a).toBe('DK');
});
test('ambiguous matches require explicit selection',()=>{
 const s=seed();expect(()=>commitDK(s,'p6',{first:'Mary',last:'Chen',mobile:'0412 555 019',email:'',relation:'owner',note:'',tags:[],reason:''})).toThrow('Choose');
});
test('merge transfers property links, notes, and removes duplicate',()=>{
 const s=seed();const a=s.contacts.find(c=>c.id==='c1')!,b=s.contacts.find(c=>c.id==='c9')!;expect(duplicatesOf(s,a)).toContain(b);
 const c=mergeContacts(s,'c1','c9',{mobile:'b'});expect(c.mobile).toBe(b.mobile);expect(s.contacts.some(x=>x.id==='c9')).toBe(false);expect(s.properties.every(p=>!p.ownerIds.includes('c9'))).toBe(true);
});
