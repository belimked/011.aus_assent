import type {Pull} from './sync-logic';

// A new native build (version + CFBundleVersion) triggers one full pull on restore; see docs/backend-design.md §6.1.
let mockBuild='67';
let mockDisk:Map<string,string>;
let mockToken:string|null='tok';
jest.mock('expo-constants',()=>({__esModule:true,default:{expoConfig:{version:'0.1.0',extra:{}},get platform(){return {ios:{buildNumber:mockBuild}}}}}));
jest.mock('../lib/notifications',()=>({reconcileReminders:jest.fn(),registerDevice:jest.fn(),unregisterDevice:jest.fn()}));
jest.mock('../lib/photos',()=>({uploadPendingPhotos:jest.fn(async()=>{})}));
jest.mock('expo-secure-store',()=>({setItemAsync:jest.fn(async(_k:string,v:string)=>{mockToken=v;}),getItemAsync:jest.fn(async()=>mockToken),deleteItemAsync:jest.fn(async()=>{mockToken=null;})}));
jest.mock('@react-native-async-storage/async-storage',()=>({__esModule:true,default:{
  setItem:jest.fn(async(k:string,v:string)=>{mockDisk.set(k,v);}),
  getItem:jest.fn(async(k:string)=>mockDisk.get(k)??null),
  removeItem:jest.fn(async(k:string)=>{mockDisk.delete(k);}),
}}));

type Handler=(path:string,query:string,body:any)=>any;
const user={id:'u',name:'Mary Lee',email:'mary@axleo.local',role:'agent',company:'Place Sunnybank'};
const oldProperty={id:'p1',no:'1',street:'A St',suburb:'Sunnybank',ownerIds:[],flags:{},version:1,market:{}};
const newProperty={...oldProperty,market:{hasOwners:true}};
const fullPull=():Pull=>({cursor:'full-1',full:true,deleted:{},properties:[newProperty],contacts:[]});
const incremental=():Pull=>({cursor:'inc-2',full:false,deleted:{}});

let store:typeof import('./index');
let sync:typeof import('./sync');
let calls:{path:string;query:string}[];
let handler:Handler;
const saved=()=>JSON.parse(mockDisk.get('ax_sync')||'null');
const pulls=()=>calls.filter(c=>c.path==='/sync');

function load(){
  jest.resetModules();
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  store=require('./index');
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  sync=require('./sync');
}
/** Simulates the cache an older build left behind: old-format records, a cursor, settings. */
function oldInstall(extra:Record<string,unknown>={}){
  mockDisk.set('ax_sync',JSON.stringify({cursor:'old-cursor',queue:[],lastSync:'2026-11-20T00:00:00.000Z',...extra}));
  mockDisk.set('ax_lang','zh');
  store.replaceState({...store.getState(),auth:{...user,company:'',team:'Axleo',rex:'',since:'',device:''},properties:[oldProperty as any],patch:{suburbs:['Runcorn']},partners:{broker:['Ann']},rexDown:true});
}
async function restore(){await sync.restoreSync();await sync.syncNow();}

beforeEach(()=>{
  jest.useFakeTimers();
  mockBuild='67';mockDisk=new Map();mockToken='tok';calls=[];
  handler=(path,query)=>{
    if(path==='/me')return user;
    if(path==='/team')return [];
    if(path==='/sync/push')throw new Error('unexpected push');
    if(path==='/sync')return query.includes('since=')?incremental():fullPull();
    throw new Error(path);
  };
  globalThis.fetch=jest.fn(async(url:string,options?:RequestInit)=>{
    const [path,query='']=url.split('/api/v1')[1].split('?');
    calls.push({path,query});
    const value=handler(path,query,options?.body?JSON.parse(options.body as string):undefined);
    if(value instanceof Error)throw value;
    return {ok:true,status:200,json:async()=>value};
  }) as any;
  load();
});
afterEach(()=>{jest.clearAllTimers();jest.useRealTimers();});

test('new build, signed in, online: one full pull replaces old-format records and records the build',async()=>{
  oldInstall({build:'0.1.0(66)'});
  await restore();
  expect(pulls()).toEqual([{path:'/sync',query:''}]);
  expect(store.getState().properties).toEqual([expect.objectContaining({id:'p1',market:{hasOwners:true}})]);
  expect(store.getState().auth?.company).toBe('Place Sunnybank');
  expect(saved()).toMatchObject({cursor:'full-1',build:'0.1.0(67)'});
  // The replaced tables were written before the build was recorded.
  expect(JSON.parse(mockDisk.get('ax_state')!).properties[0].market).toEqual({hasOwners:true});
  await sync.syncNow();
  expect(pulls()[1].query).toBe('since=full-1');
});

test('an install that never stored a build gets the full pull too',async()=>{
  oldInstall();
  await restore();
  expect(pulls()).toEqual([{path:'/sync',query:''}]);
  expect(saved().build).toBe('0.1.0(67)');
});

test('same build: incremental only',async()=>{
  oldInstall({build:'0.1.0(67)'});
  await restore();
  expect(pulls()).toEqual([{path:'/sync',query:'since=old-cursor'}]);
  expect(store.getState().properties[0].market).toEqual({});
});

test('unsent queue and failed push: queue kept, no full replacement, build unchanged',async()=>{
  const op={opId:'o1',kind:'todos',op:'upsert',id:'t1',data:{id:'t1',t:'Call back'}};
  oldInstall({build:'0.1.0(66)',queue:[op]});
  handler=(path,query)=>path==='/me'?user:path==='/team'?[]:path==='/sync/push'?new Error('offline'):query.includes('since=')?incremental():fullPull();
  await restore();
  expect(calls.some(c=>c.path==='/sync/push')).toBe(true);
  expect(pulls()).toEqual([]);
  expect(saved()).toMatchObject({queue:[expect.objectContaining({opId:'o1'})],build:'0.1.0(66)',cursor:'old-cursor'});
  expect(sync.syncInfo().pending).toBe(1);
  expect(store.getState().properties[0].market).toEqual({});
});

test('queue pushed first, then the full pull runs',async()=>{
  const op={opId:'o1',kind:'todos',op:'upsert',id:'t1',data:{id:'t1',t:'Call back'}};
  oldInstall({build:'0.1.0(66)',queue:[op]});
  handler=(path,query)=>path==='/me'?user:path==='/team'?[]:path==='/sync/push'?{results:[{opId:'o1',status:'ok',record:{id:'t1',t:'Call back',version:1}}]}:query.includes('since=')?incremental():{...fullPull(),todos:[{id:'t1',t:'Call back',version:1}]};
  await restore();
  expect(calls.map(c=>c.path).filter(p=>p.startsWith('/sync'))).toEqual(['/sync/push','/sync']);
  expect(pulls()[0].query).toBe('');
  expect(store.getState().todos.map(t=>t.id)).toEqual(['t1']);
  expect(saved()).toMatchObject({queue:[],build:'0.1.0(67)'});
});

test('offline start: no throw, cached data and queue kept, build unchanged',async()=>{
  const op={opId:'o1',kind:'todos',op:'upsert',id:'t1',data:{id:'t1'}};
  oldInstall({build:'0.1.0(66)',queue:[op]});
  globalThis.fetch=jest.fn(async()=>{throw new TypeError('Network request failed');}) as any;
  await expect(restore()).resolves.toBeUndefined();
  expect(store.getState().auth?.email).toBe('mary@axleo.local');
  expect(store.getState().properties[0].market).toEqual({});
  expect(saved()).toMatchObject({build:'0.1.0(66)',cursor:'old-cursor',queue:[expect.objectContaining({opId:'o1'})]});
  expect(sync.syncInfo().pending).toBe(1);
});

test('fresh login stores the build; the next launch does not pull in full again',async()=>{
  handler=(path,query)=>path==='/auth/login'?{token:'tok',user}:path==='/team'?[]:path==='/me'?user:query.includes('since=')?incremental():fullPull();
  await sync.login('mary@axleo.local','pw');
  expect(pulls()).toEqual([{path:'/sync',query:''}]);
  expect(saved()).toMatchObject({cursor:'full-1',build:'0.1.0(67)'});
  load();calls=[];
  await restore();
  expect(pulls()).toEqual([{path:'/sync',query:'since=full-1'}]);
});

test('local-only settings survive the full replacement',async()=>{
  oldInstall({build:'0.1.0(66)'});
  store.replaceState({...store.getState(),mailingCatalog:{items:[{name:'VIP'}],at:'x'}});
  await restore();
  const s=store.getState();
  expect(s.patch).toEqual({suburbs:['Runcorn']});
  expect(s.partners).toEqual({broker:['Ann']});
  expect(s.rexDown).toBe(true);
  expect(s.mailingCatalog?.items).toEqual([{name:'VIP'}]);
  expect(mockDisk.get('ax_lang')).toBe('zh');
  expect(saved().build).toBe('0.1.0(67)');
});

test('not signed in (demo): nothing is fetched',async()=>{
  mockToken=null;
  await restore();
  expect(calls).toEqual([]);
});
