import {SEARCH_DEBOUNCE_MS,rexNotice,searchPlan,serverQueryOK,serverRows,serverSubtitle,type ServerSearch} from './contactSearch';
import {duplicatesOf} from '../domain/dk';
import type {AppState,Contact} from '../data/types';

const l2=(en:string)=>en;

describe('contact search tiers',()=>{
 it('shows local results and never queries the server when the phone has any',()=>{
  expect(searchPlan('teddie',1,true)).toBe('local');
  expect(searchPlan('0401 999 088',3,true)).toBe('local');
 });
 it('queries the office (database + Rex in one call) only when the phone has nothing',()=>{
  expect(searchPlan('teddie',0,true)).toBe('server');
  expect(searchPlan('0401 999 088',0,true)).toBe('server');
  // A partial number still goes to the office; the server decides whether Rex is asked.
  expect(searchPlan('0401',0,true)).toBe('server');
  expect(SEARCH_DEBOUNCE_MS).toBe(500);
 });
 it('needs ≥2 characters, or ≥3 digits for a number',()=>{
  expect(searchPlan('',0,true)).toBe('idle');
  expect(searchPlan('t',0,true)).toBe('short');
  expect(searchPlan('04',0,true)).toBe('short');
  expect(serverQueryOK('+6')).toBe(false);
  expect(serverQueryOK('040')).toBe(true);
  expect(serverQueryOK('李明')).toBe(true);
 });
 it('offline with no local results asks for a connection',()=>{
  expect(searchPlan('teddie',0,false)).toBe('offline');
  expect(searchPlan('teddie',2,false)).toBe('local');
 });
});

const resp:ServerSearch={items:[
 {source:'local',id:'m1',full:true,name:'Teddie Bear',mobile:'0401 999 088',ownerName:'Mary'},
 {source:'local',id:'j1',full:false,name:'T••• O•••',ownerName:'John',inRex:true},
 {source:'rex',rexId:'68313642',full:false,name:'Teddie Bear',mobileMasked:'0401 999 •••',inRex:true},
 {source:'rex',rexId:'555',full:false,name:'Linked Already',inRex:true},
]};

describe('one merged server list',()=>{
 it('drops what the phone already has, by id and by Rex id',()=>{
  const rows=serverRows(resp,[{id:'m1'},{id:'x',rexId:'555'}]);
  expect(rows.map(r=>r.id||r.rexId)).toEqual(['j1','68313642']);
  expect(serverRows(null,[])).toEqual([]);
 });
 it('labels Rex rows as importable and other agents\u2019 contacts by owner only',()=>{
  expect(serverSubtitle(resp.items[2],l2)).toBe('In Rex · tap to import · 0401 999 •••');
  expect(serverSubtitle(resp.items[1],l2)).toBe('Owned by John');
  expect(serverSubtitle(resp.items[0],l2)).toBe('0401 999 •••• · Mary');
 });
 it('reports Rex timeouts, failures and rate limits as a quiet line',()=>{
  expect(rexNotice({items:[],rexTimedOut:true},l2)).toBe("Rex didn't respond in time");
  expect(rexNotice({items:[],rexError:true},l2)).toMatch(/Rex unavailable/);
  expect(rexNotice({items:[],rexSkipped:true,rexReason:'rate limited'},l2)).toMatch(/paused/);
  expect(rexNotice({items:[],rexSkipped:true,rexReason:'disabled'},l2)).toBe('');
  expect(rexNotice({items:[]},l2)).toBe('');
 });
});

describe('server-flagged duplicates',()=>{
 it('duplicatesOf includes possibleDuplicateOf even when the mobiles differ',()=>{
  const c=(id:string,mobile:string,extra:Partial<Contact>={}):Contact=>({id,first:'T',last:'B',mobile,email:'',owner:'Mary',roles:[],tags:[],flags:{},notes:[],...extra});
  const a=c('a','0401999088',{possibleDuplicateOf:['b']}),b=c('b','0499000000'),z=c('z','0411111111');
  const s={contacts:[a,b,z]} as unknown as AppState;
  expect(duplicatesOf(s,a).map(x=>x.id)).toEqual(['b']);
  expect(duplicatesOf(s,b).map(x=>x.id)).toEqual(['a']);
  expect(duplicatesOf(s,z)).toEqual([]);
 });
});
