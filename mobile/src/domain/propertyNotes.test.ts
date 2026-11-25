import {seed} from '../data/seed';
import {noteIdentity,savePropertyTranscript,noteIdentityLabel,hasNoteIdentity,plausibleNotePhone} from './propertyNotes';
import {mergeAI,aiText} from './aiMerge';
import {extract} from './extract';
import type {AIResult} from '../lib/voice';

test('note without contact works on an unowned property',()=>{
 const s=seed(),p=s.properties[0];p.ownerIds=[];const count=s.contacts.length;
 savePropertyTranscript(s,p.id,'No one home');
 expect(s.contacts).toHaveLength(count);expect(p.notes?.[0].t).toBe('No one home');
});
test('voice finds existing contact by normalized Australian mobile',()=>{
 const s=seed(),p=s.properties[0];p.ownerIds=[];
 const c=s.contacts[0];c.mobile='+61 499 987 654';
 const words='Spoke to Mary Chen, mobile 0499 987 654';
 savePropertyTranscript(s,p.id,words,noteIdentity(words));
 expect(p.ownerIds).toContain(c.id);expect(p.notes?.[0].t).toBe(words);
});
test('voice with new identity creates and links owner, but note stays on property',()=>{
 const s=seed(),p=s.properties[0];p.ownerIds=[];
 const words='Spoke to Mary Chen, mobile 0499 888 777';
 const before=s.contacts.length;
 savePropertyTranscript(s,p.id,words,noteIdentity(words));
 expect(s.contacts).toHaveLength(before+1);expect(p.ownerIds).toContain(s.contacts.at(-1)?.id);
 expect(s.contacts.at(-1)?.notes).toEqual([]);expect(p.notes?.[0].t).toBe(words);
});
test('voice without contact information saves only the note',()=>{
 const s=seed(),p=s.properties[0];p.ownerIds=[];
 const words='Left a card at the door';const identity=noteIdentity(words);
 expect(identity.first||identity.last||identity.mobile).toBeFalsy();
 savePropertyTranscript(s,p.id,words);expect(p.notes?.[0].t).toBe(words);expect(p.ownerIds).toEqual([]);
});
const ai=(fields:Record<string,unknown>):AIResult=>({fields:{contactName:null,firstName:null,lastName:null,phone:null,email:null,address:null,suburb:null,intent:null,priceMin:null,priceMax:null,timeframe:null,notes:null,language:null,...fields} as any,confidence:{},model:'m',latencyMs:1});
test('AI placeholders are treated as empty',()=>{
 for(const v of ['<UNKNOWN>','<UNKNOWN><UNKNOWN>.<UNKNOWN>','UNKNOWN','unknown','N/A','n/a','null','none','-','?','<name>',' <Unknown> '])expect(aiText(v)).toBe('');
 expect(aiText('Tom <UNKNOWN>')).toBe('Tom');expect(aiText('+61 412 345 678')).toBe('+61 412 345 678');
 const merged=mergeAI(extract('Left a card at the door','dk'),ai({contactName:'<UNKNOWN>',firstName:'<UNKNOWN>',lastName:'<UNKNOWN>',phone:'<UNKNOWN><UNKNOWN>.<UNKNOWN>',email:'N/A',notes:'none'}));
 expect(merged).toMatchObject({first:'',last:'',mobile:'',email:''});expect(merged.tags).not.toContain('Phone provided');
 const identity=noteIdentity('Left a card at the door',ai({firstName:'<UNKNOWN>',lastName:'<UNKNOWN>',phone:'<UNKNOWN>'}));
 expect(identity).toEqual({first:'',last:'',mobile:''});expect(hasNoteIdentity(identity)).toBe(false);
});
test('UAT transcript: single owner name kept, misheard 8-digit number is not the owner mobile',()=>{
 const words='Owner is Sawley, mobile is 67076507. Keep to sale next month.';
 const offline=noteIdentity(words);expect(offline).toEqual({first:'Sawley',last:'',mobile:''});
 const online=noteIdentity(words,ai({contactName:'Sawley',firstName:'Sawley',lastName:'<UNKNOWN>',phone:'67076507'}));
 expect(online).toEqual({first:'Sawley',last:'',mobile:''});expect(noteIdentityLabel(online)).toBe('Sawley');
 const s=seed(),p=s.properties[0];p.ownerIds=[];savePropertyTranscript(s,p.id,words,online);
 expect(p.notes?.[0].t).toBe(words);expect(s.contacts.at(-1)).toMatchObject({first:'Sawley',mobile:''});
});
test('normal transcript is unchanged: Tom Lee 0412 345 678',()=>{
 const words='Owner is Tom Lee, mobile 0412 345 678';
 expect(noteIdentity(words)).toEqual({first:'Tom',last:'Lee',mobile:'0412 345 678'});
 const online=noteIdentity(words,ai({contactName:'Tom Lee',firstName:'Tom',lastName:'Lee',phone:'0412 345 678'}));
 expect(online).toEqual({first:'Tom',last:'Lee',mobile:'0412 345 678'});expect(noteIdentityLabel(online)).toBe('Tom Lee · 0412 345 678');
});
test('identity label has no stray separators and phone rule is Australian',()=>{
 expect(noteIdentityLabel({first:'',last:'',mobile:'0412 345 678'})).toBe('0412 345 678');expect(noteIdentityLabel({first:'Mary',last:'',mobile:''})).toBe('Mary');expect(noteIdentityLabel({first:'',last:'',mobile:''})).toBe('');
 for(const ok of ['0412 345 678','+61 412 345 678','61412345678','07 3123 4567'])expect(plausibleNotePhone(ok)).toBe(true);
 for(const bad of ['67076507','0412 345','12345678901','<UNKNOWN>','0912 345 678','call me'])expect(plausibleNotePhone(bad)).toBe(false);
 expect(noteIdentity('Spoke to Mary Chen',ai({firstName:'1234',lastName:'?'}))).toEqual({first:'Mary',last:'Chen',mobile:''});
 expect(noteIdentity('x',ai({firstName:'Sam',phone:'0412345678'})).mobile).toBe('0412 345 678');
});
