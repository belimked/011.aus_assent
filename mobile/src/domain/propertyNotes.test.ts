import {seed} from '../data/seed';
import {noteIdentity,savePropertyTranscript} from './propertyNotes';

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
