import {seed} from '../data/seed';
import {saveConfirm,identitySuggestions,chosenReplace,confirmTarget,type DKFields} from './dk';
import type {AppState,Contact} from '../data/types';

const fields=(o:Partial<DKFields>={}):DKFields=>({first:'',last:'',mobile:'',email:'',relation:'',note:'',reason:'',tags:[],...o});
const identity=(c:Contact)=>({first:c.first,last:c.last,mobile:c.mobile,email:c.email});
/** UAT3: Teddie Bear, 0401 999 088, on a contact page; the note says "His name is Mr. Wong, phone 107-007-605". */
const teddie=(s:AppState)=>{const c=s.contacts[0];Object.assign(c,{first:'Teddie',last:'Bear',mobile:'0401 999 088',email:'teddie@example.com'});return c};
const wong=fields({first:'Wong',mobile:'107007605',note:"This guy's phone number is 107-007-605. His name is Mr. Wong."});

test('Update record without Replace: name, mobile and email unchanged, the note is added',()=>{
  const s=seed();const c=teddie(s);const before=identity(c);const notes=c.notes.length;
  expect(saveConfirm(s,{contactId:c.id,fields:wong}).id).toBe(c.id);
  expect(identity(c)).toEqual(before);
  expect(c.notes).toHaveLength(notes+1);expect(c.notes[0].t).toBe(wong.note);
  // An empty replace map is the same as none.
  saveConfirm(s,{contactId:c.id,replace:{},fields:{...wong,last:'Li',email:'wong@example.com'}});
  expect(identity(c)).toEqual(before);
});

test('Update record with Replace on mobile only: only the mobile changes',()=>{
  const s=seed();const c=teddie(s);
  saveConfirm(s,{contactId:c.id,replace:{mobile:'107007605'},fields:wong});
  expect(identity(c)).toEqual({first:'Teddie',last:'Bear',mobile:'107007605',email:'teddie@example.com'});
});

test('a contact matched by mobile (no contactId) keeps its name unless replaced',()=>{
  const s=seed();const c=teddie(s);const count=s.contacts.length;const p=s.properties[0];
  const f=fields({first:'Wong',last:'Lee',mobile:'0401999088',email:'WONG@example.com',relation:'owner',note:'Called back'});
  expect(confirmTarget(s,{fields:f})?.id).toBe(c.id);
  expect(saveConfirm(s,{propId:p.id,fields:f}).id).toBe(c.id);
  expect(identity(c)).toEqual({first:'Teddie',last:'Bear',mobile:'0401 999 088',email:'teddie@example.com'});
  expect(s.contacts).toHaveLength(count);expect(c.notes[0].t).toBe('Called back');
  saveConfirm(s,{propId:p.id,fields:f,replace:{last:'Lee'}});
  expect(c.last).toBe('Lee');expect(c.first).toBe('Teddie');
});

test('a contact picked from the match list keeps its name unless replaced',()=>{
  const s=seed();const c=teddie(s);
  saveConfirm(s,{pick:c.id,fields:fields({first:'Wong',email:'other@example.com',note:'Picked'})});
  expect(identity(c)).toEqual({first:'Teddie',last:'Bear',mobile:'0401 999 088',email:'teddie@example.com'});
  expect(confirmTarget(s,{pick:'new',fields:fields({mobile:c.mobile})})).toBeUndefined();
});

test('new contact path still saves the extracted values',()=>{
  const s=seed();const count=s.contacts.length;const p=s.properties[0];
  const f=fields({first:'Nina',last:'Park',mobile:'0499 123 456',email:'nina@example.com',relation:'owner'});
  expect(confirmTarget(s,{fields:f})).toBeUndefined();
  const c=saveConfirm(s,{propId:p.id,fields:f,replace:{first:'Ignored'}});
  expect(s.contacts).toHaveLength(count+1);
  expect(identity(c)).toEqual({first:'Nina',last:'Park',mobile:'0499 123 456',email:'nina@example.com'});
});

describe('identitySuggestions',()=>{
  const cur={first:'Teddie',last:'Bear',mobile:'0401 999 088',email:'Teddie@Example.com'};
  test('normalised equal values give no suggestion',()=>{
    expect(identitySuggestions(cur,{first:' teddie ',last:'BEAR',mobile:'0401999088',email:'teddie@example.COM '})).toEqual([]);
    expect(identitySuggestions(cur,{mobile:'+61 401 999 088'})).toEqual([]);
    expect(identitySuggestions(cur,{first:'',last:'',mobile:'',email:''})).toEqual([]);
  });
  test('different or newly provided values are listed, flagged when unsure',()=>{
    const list=identitySuggestions(cur,{first:'Wong',mobile:'107007605'},['mobile']);
    expect(list).toEqual([{field:'first',from:'Teddie',to:'Wong',unsure:false},{field:'mobile',from:'0401 999 088',to:'107007605',unsure:true}]);
    expect(identitySuggestions({...cur,email:''},{email:'a@b.co'})).toEqual([{field:'email',from:'',to:'a@b.co',unsure:false}]);
    expect(identitySuggestions(cur,{last:'Wong'},['name'])[0]).toMatchObject({field:'last',unsure:true});
  });
  test('chosenReplace keeps only the rows switched to Replace',()=>{
    const list=identitySuggestions(cur,{first:'Wong',mobile:'107007605'});
    expect(chosenReplace(list,[])).toEqual({});
    expect(chosenReplace(list,['mobile'])).toEqual({mobile:'107007605'});
    expect(chosenReplace(list,['email'])).toEqual({});
  });
});
