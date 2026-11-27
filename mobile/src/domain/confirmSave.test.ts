import {seed} from '../data/seed';
import {saveConfirm,confirmNeedsName,confirmConsent,type DKFields} from './dk';
import type {AppState} from '../data/types';

const fields=(o:Partial<DKFields>={}):DKFields=>({first:'',last:'',mobile:'',email:'',relation:'',note:'',reason:'',tags:[],...o});
const owners=(s:AppState)=>s.properties.map(p=>[p.id,[...p.ownerIds]]);
const setup=()=>{const s=seed();const c=s.contacts[0];const p=s.properties[0];p.ownerIds=[c.id];return {s,c,p}};

test('Update record from a contact page: the note goes to that contact, no new contact, no owner change',()=>{
  const {s,c,p}=setup();const count=s.contacts.length;const before=owners(s);const notes=c.notes.length;const{first,last,mobile}=c;
  // Even with the default-looking relation and a property in context, nothing is linked.
  const out=saveConfirm(s,{propId:p.id,contactId:c.id,fields:fields({note:'Thinking of selling in spring',tags:['Friendly']})});
  expect(out.id).toBe(c.id);
  expect(s.contacts).toHaveLength(count);
  expect(owners(s)).toEqual(before);
  expect(c.notes).toHaveLength(notes+1);
  expect(c.notes[0]).toMatchObject({t:'Thinking of selling in spring'});
  expect(c.notes[0].id).toMatch(/^cn[a-z0-9]+$/);expect(c.notes[0].propId).toBeUndefined();expect(c.notes[0].sys).toBeUndefined();
  expect(c.tags).toContain('Friendly');
  // Empty name / mobile fields keep the current details.
  expect(c).toMatchObject({first,last,mobile});
});

test('Update record: extracted identity is kept out unless replaced; consent only changes when edited',()=>{
  const {s,c,p}=setup();c.consent={sms:0,email:1};const before=owners(s);const{first,email}=c;
  const consent=confirmConsent(c,[]);expect(consent).toEqual({email:1,sms:0,phone:1,letter:1});
  saveConfirm(s,{propId:p.id,contactId:c.id,fields:fields({first:'Teddie',email:'teddie@example.com',consent,followup:'2030-01-02'})});
  expect(c.first).toBe(first);expect(c.email).toBe(email);
  saveConfirm(s,{propId:p.id,contactId:c.id,replace:{first:'Teddie',email:'teddie@example.com'},fields:fields({first:'Teddie',email:'teddie@example.com'})});
  expect(c.first).toBe('Teddie');expect(c.email).toBe('teddie@example.com');
  expect(c.consent).toEqual({sms:0,email:1});expect(c.consentRec).toBeUndefined();
  expect(s.todos.some(t=>t.cid===c.id&&t.d==='2030-01-02')).toBe(true);
  expect(owners(s)).toEqual(before);
  saveConfirm(s,{propId:p.id,contactId:c.id,fields:fields({consent:{...consent,letter:0}})});
  expect(c.consent).toEqual({sms:0,email:1,letter:0});
});

test('Update record links a property only when the relation is explicitly owner',()=>{
  const {s,c}=setup();const other=s.properties[1];other.ownerIds=other.ownerIds.filter(id=>id!==c.id);
  saveConfirm(s,{propId:other.id,contactId:c.id,fields:fields({relation:'tenant'})});
  expect(other.ownerIds).not.toContain(c.id);
  saveConfirm(s,{propId:other.id,contactId:c.id,fields:fields({relation:'owner'})});
  expect(other.ownerIds).toContain(c.id);
});

test('Update record never silently creates a contact for a missing target',()=>{
  const {s}=setup();const count=s.contacts.length;
  expect(()=>saveConfirm(s,{contactId:'nope',fields:fields({note:'x'})})).toThrow('Contact not found');
  expect(s.contacts).toHaveLength(count);
});

test('door knock without contactId and no match: unchanged, creates the contact and links the owner',()=>{
  const {s,p}=setup();const count=s.contacts.length;
  const a={propId:p.id,fields:fields({first:'Nina',last:'Park',mobile:'0499 123 456',relation:'owner',note:'Kids moved out'})};
  expect(confirmNeedsName(s,a)).toBe(false);
  const c=saveConfirm(s,a);
  expect(s.contacts).toHaveLength(count+1);expect(p.ownerIds).toContain(c.id);
  expect(c.notes[0]).toMatchObject({t:'Kids moved out',propId:p.id});
});

test('a new contact with no first or last name is refused; an existing match still saves',()=>{
  const {s,c,p}=setup();const count=s.contacts.length;const before=owners(s);
  const a={propId:p.id,fields:fields({relation:'owner',note:'Left a card'})};
  expect(confirmNeedsName(s,a)).toBe(true);
  expect(()=>saveConfirm(s,a)).toThrow('contact name required');
  expect(s.contacts).toHaveLength(count);expect(owners(s)).toEqual(before);
  expect(confirmNeedsName(s,{...a,fields:fields({first:'  ',last:'Lee'})})).toBe(false);
  // A unique mobile match updates that contact, so no name is needed.
  c.mobile='0411 222 333';
  const m={propId:p.id,fields:fields({mobile:'0411 222 333',note:'Called back'})};
  expect(confirmNeedsName(s,m)).toBe(false);expect(saveConfirm(s,m).id).toBe(c.id);expect(s.contacts).toHaveLength(count);
  expect(confirmNeedsName(s,{...a,pick:c.id})).toBe(false);
});
