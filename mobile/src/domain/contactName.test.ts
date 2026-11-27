import {seed} from '../data/seed';
import type {AppState} from '../data/types';
import {hasContactName,nameRequiredMessage,isNameRequiredError,CONTACT_NAME_REQUIRED} from './contactName';
import {commitDK,confirmNeedsName,dkNeedsName,saveConfirm,type DKFields} from './dk';
import {approveCapture,captureNeedsName} from './review';
import {savePropertyTranscript} from './propertyNotes';
import {commitOH} from './ofi';
import {createLead,emptyEnq,enquiryNeedsName,fileEnquiry,leadNeedsName} from './enq';

const NEW_MOBILE='0499 000 111';
const dk=(o:Partial<DKFields>={}):DKFields=>({first:'',last:'',mobile:'',email:'',relation:'owner',note:'',reason:'',tags:[],...o});
/** A seed contact whose mobile no other contact shares, so a mobile match is unique. */
const uniqueContact=(s:AppState)=>s.contacts.find(c=>c.mobile&&s.contacts.filter(x=>x.mobile.replace(/\D/g,'')===c.mobile.replace(/\D/g,'')).length===1)!;
const caught=(fn:()=>unknown)=>{try{fn()}catch(e){return e}return undefined};

test('one rule and one message for every entry point',()=>{
  expect(hasContactName({first:'',last:''})).toBe(false);
  expect(hasContactName({first:'  ',last:' '})).toBe(false);
  expect(hasContactName({first:'',last:'Lee'})).toBe(true);
  expect(hasContactName({first:'Tom'})).toBe(true);
  const en=(a:string)=>a,zh=(_:string,b:string)=>b;
  expect(nameRequiredMessage(en)).toBe('Enter a first or last name before saving a new contact');
  expect(nameRequiredMessage(zh)).toBe('保存新联系人前请填写名字或姓氏');
  expect(isNameRequiredError(new Error(CONTACT_NAME_REQUIRED))).toBe(true);
  expect(isNameRequiredError(new Error('other'))).toBe(false);
});

describe('commitDK (QuickAdd "Add or link owner", DK confirm, capture approval)',()=>{
  test('a new contact with only a mobile is refused and nothing changes',()=>{
    const s=seed(),p=s.properties[0];const count=s.contacts.length;const owners=[...p.ownerIds];
    expect(dkNeedsName(s,dk({mobile:NEW_MOBILE}))).toBe(true);
    expect(isNameRequiredError(caught(()=>commitDK(s,p.id,dk({mobile:NEW_MOBILE,note:'x'}))))).toBe(true);
    expect(s.contacts).toHaveLength(count);expect(p.ownerIds).toEqual(owners);
  });
  test('QuickAdd "Not this person" (pick new) with no name is refused, with a name it creates',()=>{
    const s=seed(),p=s.properties[0],c=s.contacts[0];const count=s.contacts.length;
    expect(dkNeedsName(s,dk({mobile:c.mobile}),'new')).toBe(true);
    expect(confirmNeedsName(s,{propId:p.id,pick:'new',fields:dk({mobile:c.mobile})})).toBe(true);
    expect(()=>commitDK(s,p.id,dk({mobile:c.mobile}),'new')).toThrow(CONTACT_NAME_REQUIRED);
    expect(s.contacts).toHaveLength(count);
    const made=commitDK(s,p.id,dk({last:'Nguyen',mobile:c.mobile}),'new');
    expect(made.id).not.toBe(c.id);expect(made.last).toBe('Nguyen');expect(s.contacts).toHaveLength(count+1);
  });
  test('a named new contact saves',()=>{
    const s=seed(),p=s.properties[0];const count=s.contacts.length;
    expect(dkNeedsName(s,dk({first:'Tom',mobile:NEW_MOBILE}))).toBe(false);
    const c=commitDK(s,p.id,dk({first:'Tom',mobile:NEW_MOBILE}));
    expect(s.contacts).toHaveLength(count+1);expect(c.first).toBe('Tom');expect(p.ownerIds).toContain(c.id);
  });
  test('matching or picking an existing contact saves without a name',()=>{
    const s=seed(),p=s.properties[0],c=uniqueContact(s);const count=s.contacts.length;const first=c.first;
    expect(dkNeedsName(s,dk({mobile:c.mobile}))).toBe(false);
    expect(commitDK(s,p.id,dk({mobile:c.mobile,note:'Matched by mobile'})).id).toBe(c.id);
    expect(dkNeedsName(s,dk(),c.id)).toBe(false);
    expect(commitDK(s,p.id,dk({note:'Picked'}),c.id).id).toBe(c.id);
    expect(s.contacts).toHaveLength(count);expect(c.first).toBe(first);
  });
  test('saveConfirm still refuses a nameless new contact and saves on a contact page',()=>{
    const s=seed(),c=s.contacts[0];
    expect(()=>saveConfirm(s,{fields:dk({mobile:NEW_MOBILE})})).toThrow(CONTACT_NAME_REQUIRED);
    expect(saveConfirm(s,{contactId:c.id,fields:dk({relation:'',note:'n'})}).id).toBe(c.id);
  });
});

describe('review queue: approveCapture',()=>{
  const add=(s:AppState,text:string)=>{const p=s.properties.find(x=>!x.ownerIds.length)||s.properties[0];s.captures.push({id:'cap',text,rec:'today',sync:'today',propId:p.id});return p};
  test('a capture with only a mobile is refused and stays in the queue',()=>{
    const s=seed();add(s,`Call back on ${NEW_MOBILE} about selling`);const count=s.contacts.length;
    expect(captureNeedsName(s,'cap')).toBe(true);
    expect(()=>approveCapture(s,'cap')).toThrow(CONTACT_NAME_REQUIRED);
    expect(s.captures.some(c=>c.id==='cap')).toBe(true);expect(s.contacts).toHaveLength(count);
    // keepApproved (online) path also keeps the pending capture
    expect(()=>approveCapture(s,'cap',undefined,true)).toThrow(CONTACT_NAME_REQUIRED);
    expect(s.captures.filter(c=>c.id==='cap')).toEqual([expect.objectContaining({id:'cap'})]);
    expect(s.captures.find(c=>c.id==='cap')?.status).toBeUndefined();
  });
  test('a name typed on the review sheet lets it file',()=>{
    const s=seed();add(s,`Call back on ${NEW_MOBILE} about selling`);const count=s.contacts.length;
    expect(captureNeedsName(s,'cap',undefined,{first:'Ann'})).toBe(false);
    approveCapture(s,'cap',undefined,false,{first:'Ann'});
    expect(s.captures.some(c=>c.id==='cap')).toBe(false);expect(s.contacts).toHaveLength(count+1);expect(s.contacts.at(-1)).toMatchObject({first:'Ann'});
  });
  test('a capture matching an existing contact by mobile files without a name',()=>{
    const s=seed(),c=uniqueContact(s);add(s,`Call back on ${c.mobile} about selling`);const count=s.contacts.length;
    expect(captureNeedsName(s,'cap')).toBe(false);
    approveCapture(s,'cap');
    expect(s.captures.some(x=>x.id==='cap')).toBe(false);expect(s.contacts).toHaveLength(count);
  });
  test('"None of these · create new" without a name is refused',()=>{
    const s=seed(),c=s.contacts[0];add(s,`Call back on ${c.mobile} about selling`);
    expect(captureNeedsName(s,'cap',undefined,undefined,'new')).toBe(true);
    expect(()=>approveCapture(s,'cap',undefined,false,undefined,'new')).toThrow(CONTACT_NAME_REQUIRED);
    expect(s.captures.some(x=>x.id==='cap')).toBe(true);
  });
  test('a visit with no identity still files to the property',()=>{
    const s=seed();const p=add(s,'Not home, left card');const count=s.contacts.length;
    expect(captureNeedsName(s,'cap')).toBe(false);
    approveCapture(s,'cap');
    expect(s.captures.some(x=>x.id==='cap')).toBe(false);expect(s.contacts).toHaveLength(count);expect(p.visits?.length).toBeGreaterThan(0);
  });
});

describe('property voice note: savePropertyTranscript',()=>{
  const noOwner=(s:AppState)=>{const p=s.properties[0];p.ownerIds=[];return p};
  test('a heard mobile with no name and no match does not create an owner (or file the note twice)',()=>{
    const s=seed(),p=noOwner(s);const count=s.contacts.length;const notes=(p.notes||[]).length;
    expect(()=>savePropertyTranscript(s,p.id,'call 0499 000 111',{first:'',last:'',mobile:NEW_MOBILE})).toThrow(CONTACT_NAME_REQUIRED);
    expect(s.contacts).toHaveLength(count);expect(p.ownerIds).toEqual([]);expect((p.notes||[]).length).toBe(notes);
  });
  test('"Save note only" keeps working',()=>{
    const s=seed(),p=noOwner(s);const count=s.contacts.length;
    savePropertyTranscript(s,p.id,'call 0499 000 111');
    expect(p.notes?.[0].t).toBe('call 0499 000 111');expect(s.contacts).toHaveLength(count);
  });
  test('a named owner is created and a matching mobile links the existing contact',()=>{
    const s=seed(),p=noOwner(s);const count=s.contacts.length;
    savePropertyTranscript(s,p.id,'Owner Tom',{first:'Tom',last:'',mobile:NEW_MOBILE});
    expect(s.contacts).toHaveLength(count+1);expect(p.ownerIds).toEqual([s.contacts.at(-1)!.id]);
    const c=uniqueContact(s);const q=s.properties[1];q.ownerIds=[];
    savePropertyTranscript(s,q.id,'number only',{first:'',last:'',mobile:c.mobile});
    expect(q.ownerIds).toEqual([c.id]);expect(s.contacts).toHaveLength(count+1);
  });
});

describe('OFI check-in: commitOH',()=>{
  test('a new visitor with no name is refused; named and returning visitors check in',()=>{
    const s=seed();const count=s.contacts.length;
    expect(()=>commitOH(s,'l1',{first:'',last:'',mobile:NEW_MOBILE,email:''})).toThrow(CONTACT_NAME_REQUIRED);
    expect(s.contacts).toHaveLength(count);
    expect(commitOH(s,'l1',{first:'Zoe',last:'',mobile:NEW_MOBILE,email:''}).first).toBe('Zoe');
    expect(s.contacts).toHaveLength(count+1);
    const c=s.contacts[0];const name=c.first;
    expect(commitOH(s,'l1',{first:'',last:'',mobile:c.mobile,email:''}).id).toBe(c.id);
    expect(c.first).toBe(name);expect(s.contacts).toHaveLength(count+1);
  });
});

describe('enquiries',()=>{
  test('createLead: nameless new contact refused; named or matched saves',()=>{
    const s=seed();const count=s.contacts.length;const d={...emptyEnq(),type:'seller',mobile:NEW_MOBILE,note:'Appraisal'};
    expect(leadNeedsName(s,d)).toBe(true);
    expect(()=>createLead(s,d)).toThrow(CONTACT_NAME_REQUIRED);
    expect(s.contacts).toHaveLength(count);expect(s.leads.length).toBe(seed().leads.length);
    expect(leadNeedsName(s,{...d,last:'Wu'})).toBe(false);
    createLead(s,{...d,last:'Wu'});expect(s.contacts).toHaveLength(count+1);
    const c=s.contacts[0];
    expect(leadNeedsName(s,{...d,mobile:c.mobile})).toBe(false);
    expect(createLead(s,{...d,mobile:c.mobile}).contactId).toBe(c.id);
    expect(leadNeedsName(s,{...d,mobile:'',email:c.email})).toBe(!c.email);
  });
  test('fileEnquiry: nameless new contact refused and the enquiry stays unfiled; named or matched files',()=>{
    const s=seed();const e=s.enquiries[0];e.parsed={...e.parsed,first:'',last:'',mobile:NEW_MOBILE,email:''};const count=s.contacts.length;
    expect(enquiryNeedsName(s,e)).toBe(true);
    expect(()=>fileEnquiry(s,e)).toThrow(CONTACT_NAME_REQUIRED);
    expect(e.status).toBe('new');expect(s.contacts).toHaveLength(count);
    e.parsed.first='Kim';expect(enquiryNeedsName(s,e)).toBe(false);fileEnquiry(s,e);
    expect(e.status).toBe('filed');expect(s.contacts).toHaveLength(count+1);
    const f=s.enquiries[1];const c=s.contacts[0];f.parsed={...f.parsed,first:'',last:'',mobile:c.mobile,email:''};
    expect(enquiryNeedsName(s,f)).toBe(false);fileEnquiry(s,f);expect(f.status).toBe('filed');expect(s.contacts).toHaveLength(count+1);
  });
});
