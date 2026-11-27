import {OWNER_NAMES_TTL_MS,cachedOwnerNames,clearOwnerNames,loadOwnerNames,ownerNamesView} from './ownerNames';

beforeEach(()=>clearOwnerNames());

const reply={owners:'Jane Citizen & John Citizen',absenteeOwner:true,source:'rpdata',updatedAt:'2026-11-25T01:00:00Z',notice:'RP Data · not for direct marketing'};

test('success returns the names and caches them by property',async()=>{
  const fetcher=jest.fn(async()=>reply);
  expect(await ownerNamesView('p1',true,fetcher)).toEqual({status:'ok',data:reply});
  expect(fetcher).toHaveBeenCalledWith('p1');
  expect(cachedOwnerNames('p1')?.owners).toBe('Jane Citizen & John Citizen');
  expect(cachedOwnerNames('p2')).toBeUndefined();
});

test('empty names are a valid answer, not a failure',async()=>{
  const fetcher=jest.fn(async()=>({owners:'',absenteeOwner:false,source:'',updatedAt:'',notice:'x'}));
  const v=await ownerNamesView('p1',true,fetcher);
  expect(v).toEqual({status:'ok',data:expect.objectContaining({owners:''})});
});

test('no request when the property has no owner names',async()=>{
  const fetcher=jest.fn(async()=>reply);
  expect(await ownerNamesView('p1',false,fetcher)).toBeNull();
  expect(await ownerNamesView('p1',undefined,fetcher)).toBeNull();
  expect(fetcher).not.toHaveBeenCalled();
});

test('failure (offline / HTTP error) is unavailable and not cached',async()=>{
  const fetcher=jest.fn(async():Promise<unknown>=>{throw new Error('HTTP 503')});
  expect(await ownerNamesView('p1',true,fetcher)).toEqual({status:'unavailable'});
  expect(cachedOwnerNames('p1')).toBeUndefined();
  const ok=jest.fn(async()=>reply);
  expect((await ownerNamesView('p1',true,ok))?.status).toBe('ok');
  expect(ok).toHaveBeenCalledTimes(1);
});

test('cache hit skips the request; concurrent loads share one request',async()=>{
  const fetcher=jest.fn(async()=>reply);
  const [a,b]=await Promise.all([loadOwnerNames('p1',fetcher),loadOwnerNames('p1',fetcher)]);
  expect(a).toEqual(b);
  await loadOwnerNames('p1',fetcher);
  expect(fetcher).toHaveBeenCalledTimes(1);
});

test('cache expires after the TTL and on logout',async()=>{
  const fetcher=jest.fn(async()=>reply);
  await loadOwnerNames('p1',fetcher);
  expect(cachedOwnerNames('p1',Date.now()+OWNER_NAMES_TTL_MS+1)).toBeUndefined();
  await loadOwnerNames('p1',fetcher);
  expect(fetcher).toHaveBeenCalledTimes(2);
  clearOwnerNames();
  expect(cachedOwnerNames('p1')).toBeUndefined();
});

test('a reply that arrives after logout is not cached',async()=>{
  let release:(v:unknown)=>void=()=>{};
  const fetcher=jest.fn(()=>new Promise<unknown>(r=>{release=r}));
  const pending=loadOwnerNames('p1',fetcher);
  clearOwnerNames();
  release(reply);
  await pending;
  expect(cachedOwnerNames('p1')).toBeUndefined();
});
