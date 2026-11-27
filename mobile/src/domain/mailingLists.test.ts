import {addMailingList,mailingReady,mailingSync,pickerOptions,removeMailingList,unsubscribePrompt} from './mailingLists';
import type {AppState,Contact} from '../data/types';
const contact=(x:Partial<Contact>={}):Contact=>({id:'c1',first:'Jane',last:'Doe',mobile:'',email:'',owner:'',roles:[],tags:[],flags:{},notes:[],consent:{email:1,sms:1,phone:1,letter:1},...x});
const state=(c:Contact)=>({auth:{name:'Mary'},audit:[],contacts:[c]} as unknown as AppState);
describe('mailing lists',()=>{
 test('picker hides joined lists (any case/spacing), filters, dedupes and sorts',()=>{
  const cat=[{name:'Sellers'},{name:'buyers  newsletter'},{name:'Buyers Newsletter'},{name:'Auction alerts'},{name:''}];
  expect(pickerOptions(cat,['Buyers newsletter'],'').map(l=>l.name)).toEqual(['Auction alerts','Sellers']);
  expect(pickerOptions(cat,[],' SELL ').map(l=>l.name)).toEqual(['Sellers']);
  expect(pickerOptions([],[],'')).toEqual([]);
 });
 test('known channel prompts only when that channel is unsubscribed',()=>{
  const c=contact({consent:{email:1,sms:0,phone:1,letter:1}});
  expect(unsubscribePrompt(c,{name:'News',channel:'email'})).toBeNull();
  expect(unsubscribePrompt(c,{name:'Texts',channel:'sms'})).toEqual({channels:['sms'],dnc:false});
 });
 test('unknown channel prompts when any channel is off and lists them',()=>{
  expect(unsubscribePrompt(contact(),{name:'News'})).toBeNull();
  expect(unsubscribePrompt(contact({consent:{email:0,sms:1,phone:1,letter:0}}),{name:'News'})).toEqual({channels:['email','letter'],dnc:false});
 });
 test('Do not contact counts as every channel off, by flag or tag',()=>{
  const all=['email','sms','phone','letter'];
  expect(unsubscribePrompt(contact({flags:{dnc:true}}),{name:'News',channel:'email'})).toEqual({channels:['email'],dnc:true});
  expect(unsubscribePrompt(contact({tags:['Do not contact']}),{name:'News'})).toEqual({channels:all,dnc:true});
 });
 test('add/remove edit only mailingLists, case-insensitively, with audit',()=>{
  const c=contact({mailingLists:['Buyers'],mailingListsSeen:['Buyers']});const s=state(c);
  addMailingList(s,c,' Sellers ');addMailingList(s,c,'buyers');
  expect(c.mailingLists).toEqual(['Buyers','Sellers']);
  removeMailingList(s,c,'BUYERS');
  expect(c.mailingLists).toEqual(['Sellers']);expect(c.mailingListsSeen).toEqual(['Buyers']);
  expect(s.audit[0].t).toBe('Jane Doe · removed BUYERS');expect(c.updated.by).toBe('Mary');
 });
 test('sync label and Rex readiness',()=>{
  const c=contact({source:'rex_import',mailingLists:['A','B','C'],mailingListsSeen:['A','B'],mailingListSync:{b:{status:'failed',reason:'unsubscribed in Rex'}}});
  expect(mailingSync(c,'A')).toEqual({status:'sent'});
  expect(mailingSync(c,'b')).toEqual({status:'failed',reason:'unsubscribed in Rex'});
  expect(mailingSync(c,'C')).toEqual({status:'pending'});
  expect(mailingReady(c)).toBe(true);expect(mailingReady({...c,rexLinked:false})).toBe(false);expect(mailingReady(contact())).toBe(false);
 });
});
