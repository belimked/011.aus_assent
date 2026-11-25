import type {Op,Pull} from './sync-logic';

jest.mock('../lib/notifications',()=>({reconcileReminders:jest.fn(),registerDevice:jest.fn(),unregisterDevice:jest.fn()}));
jest.mock('../lib/photos',()=>({uploadPendingPhotos:jest.fn(async()=>{})}));
jest.mock('expo-secure-store',()=>({setItemAsync:jest.fn(async()=>{}),getItemAsync:jest.fn(async()=>null),deleteItemAsync:jest.fn(async()=>{})}));
jest.mock('@react-native-async-storage/async-storage',()=>({__esModule:true,default:{setItem:jest.fn(async()=>{}),getItem:jest.fn(async()=>null),removeItem:jest.fn(async()=>{})}}));

type Request={path:string;body?:{ops:Op[]};resolve:(value:any)=>void};
const flush=async()=>{for(let i=0;i<8;i++)await Promise.resolve();};
const response=(value:any)=>({ok:true,json:async()=>value});

describe('edits made while synchronizing',()=>{
  let requests:Request[];
  let server:any;
  let cursor:number;
  let store:typeof import('./index');
  let sync:typeof import('./sync');
  const take=async(path:string)=>{for(let i=0;i<20;i++){const index=requests.findIndex(r=>r.path===path);if(index>=0)return requests.splice(index,1)[0];await flush();}throw Error(`No ${path} request`);};
  const answer=async(path:string,value:any)=>{(await take(path)).resolve(value);await flush();};
  const pull=():Pull=>({cursor:String(++cursor),full:false,deleted:{},todos:server?[{...server}]:[]});
  const push=(req:Request)=>{const results=req.body!.ops.map(op=>{
    if(op.baseVersion!==server?.version)return {opId:op.opId,status:'conflict',record:server&&{...server}};
    server=op.op==='delete'?undefined:{...op.data,version:(server?.version||0)+1};
    return {opId:op.opId,status:'ok',record:server&&{...server}};
  });req.resolve({results});};
  const settle=async()=>{await answer('/team',[]);const req=await take('/sync/push');push(req);await flush();await answer('/sync',pull());};
  beforeEach(async()=>{
    jest.resetModules();jest.useFakeTimers();requests=[];server=undefined;cursor=0;
    globalThis.fetch=jest.fn((url:string,options?:RequestInit)=>new Promise(resolve=>{
      const path=url.split('/api/v1')[1].split('?')[0];
      requests.push({path,body:options?.body?JSON.parse(options.body as string):undefined,resolve:(value:any)=>resolve(response(value))});
    })) as any;
    // Reload the singleton store/sync pair for each interleaving.
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    store=require('./index');
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    sync=require('./sync');
    const login=sync.login('test@example.com','password');
    await answer('/auth/login',{token:'test',user:{id:'u',name:'Owen Chen',email:'test@example.com',role:'agent'}});
    await answer('/team',[]);await answer('/sync',pull());await login;
    store.update(s=>{s.todos=[{id:'new-todo',t:'Follow up',d:'2026-11-12',who:'Owen Chen',done:false} as any];});
  });
  afterEach(()=>{jest.clearAllTimers();jest.useRealTimers();});
  const complete=()=>store.update(s=>{s.todos[0].done=true;});
  const undo=()=>store.update(s=>{s.todos[0].done=false;});
  const check=async()=>{
    expect(store.getState().todos[0]).toMatchObject({id:'new-todo',done:false});
    // A subsequent round trip must not overwrite Undo or create a conflict.
    const next=sync.syncNow();await answer('/team',[]);if(sync.syncInfo().pending){const req=await take('/sync/push');push(req);await flush();}await answer('/sync',pull());await next;
    expect(server).toMatchObject({id:'new-todo',done:false});
    expect(store.getState().todos[0]).toMatchObject({id:'new-todo',done:false});
    expect(store.getState().syncq.filter(q=>q.status==='error')).toEqual([]);
  };
  test.each(['before sync starts','during team fetch','while create push is in flight','after push, during pull'])(
    'Complete and Undo %s',async phase=>{
      if(phase==='before sync starts'){complete();undo();expect(sync.syncInfo().pending).toBeGreaterThan(0);const run=sync.syncNow();await settle();await run;}
      else if(phase==='during team fetch'){const run=sync.syncNow();await take('/team').then(r=>{complete();undo();r.resolve([]);});expect(sync.syncInfo().pending).toBeGreaterThan(0);await flush();const req=await take('/sync/push');push(req);await flush();await answer('/sync',pull());await run;}
      else if(phase==='while create push is in flight'){const run=sync.syncNow();await answer('/team',[]);const req=await take('/sync/push');complete();undo();push(req);await flush();await answer('/sync',pull());await run;}
      else {const run=sync.syncNow();await answer('/team',[]);const req=await take('/sync/push');push(req);await flush();await take('/sync').then(r=>{complete();undo();r.resolve(pull());});await run;}
      await check();
    });
  test.each(['before sync starts','during team fetch','while Complete push is in flight','after push, during pull'])(
    'synced todo: Complete and Undo %s',async phase=>{
      const create=sync.syncNow();await settle();await create;
      complete();
      if(phase==='before sync starts'){undo();const run=sync.syncNow();await settle();await run;}
      else if(phase==='during team fetch'){const run=sync.syncNow();const team=await take('/team');undo();team.resolve([]);await flush();const req=await take('/sync/push');push(req);await flush();await answer('/sync',pull());await run;}
      else if(phase==='while Complete push is in flight'){const run=sync.syncNow();await answer('/team',[]);const req=await take('/sync/push');undo();push(req);await flush();await answer('/sync',pull());await run;}
      else {const run=sync.syncNow();await answer('/team',[]);const req=await take('/sync/push');push(req);await flush();const reqPull=await take('/sync');undo();reqPull.resolve(pull());await run;}
      await check();
    });
  test('empty incremental pull does not emit or replace state',async()=>{
    const create=sync.syncNow();await settle();await create;
    const before=store.getState();const listener=jest.fn();const unsubscribe=store.subscribe(listener);
    const run=sync.syncNow();await answer('/team',[]);await answer('/sync',{cursor:'next',full:false,deleted:{}});await run;
    expect(store.getState()).toBe(before);
    expect(listener).not.toHaveBeenCalled();
    expect(sync.syncInfo().lastSync).toBeTruthy();
    unsubscribe();
  });
  test('Undo while Complete push is in flight retains the latest edit and server version',async()=>{
    const create=sync.syncNow();await settle();await create;
    complete();const run=sync.syncNow();await answer('/team',[]);const req=await take('/sync/push');
    undo();push(req);await flush();await answer('/sync',pull());await run;
    await check();
  });
});

describe('formatLastSync',()=>{
  it('formats ISO timestamps and hides invalid legacy values',()=>{
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const {formatLastSync}=require('./sync');
    expect(formatLastSync('')).toBe('—');
    expect(formatLastSync('25/11/2026, 4:31:00 pm')).toBe('—');
    expect(formatLastSync('2026-11-25T06:31:00.000Z')).toMatch(/\d{1,2}:\d{2}/);
  });
});
