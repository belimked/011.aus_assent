import {isMyListing,isMyOpenHome,ofiAgent,upcomingOpenHomes} from './ofiList';
import {listingStatusText,placeListing} from './listing';
import {mapRecord} from '../store/sync-logic';
import {markerKind} from '../components/map/mapMarks';
import type {Listing,Property} from '../data/types';

const ev=(d:string,time:string,extra:Record<string,unknown>={})=>({d,time,attendees:[],...extra});
// Mary's Rex listing: events run by Marketing Team, Owen, and (no agent on the Rex event) Mary as listing agent.
const mary:Listing={id:'rex-listings-2057-1',propertyId:'p1',agent:'Mary Rex',agentIds:['u-mary'],price:'',status:'listed',ofi:[ev('2026-11-19','10:00–10:30',{agent:'Marketing Team',agentId:'',rexEventId:'e0'}),ev('2026-11-20','16:00–16:30',{agent:'Marketing Team',agentId:'',rexEventId:'e1'}),ev('2026-11-20','09:00–09:30',{agent:'Owen Rex',agentId:'u-owen',rexEventId:'e2'}),ev('2026-11-21','10:00–10:30',{agent:'Mary Rex',agentId:'u-mary',agentFrom:'listing',rexEventId:'e3'}),ev('2026-12-18','10:00–10:30',{agent:'Marketing Team',agentId:'',rexEventId:'e4'})]};
const other:Listing={id:'rex-listings-2057-2',propertyId:'p2',agent:'John Rex',agentIds:['u-john'],price:'',status:'listed',ofi:[ev('2026-11-22','11:00–11:30',{agent:'Owen Rex',agentId:'u-owen',rexEventId:'e5'})]};
const now=new Date(2026,10,20,12,0);
const keys=(g:ReturnType<typeof upcomingOpenHomes>)=>g.map(x=>`${x.day}:${x.d}:${x.items.map(i=>i.o.rexEventId).join(',')}`);

test('all upcoming open homes, grouped Today / Tomorrow / date, past days dropped',()=>{
 expect(keys(upcomingOpenHomes([mary,other],now,'all',{id:'u-owen',name:'Owen Chen'}))).toEqual(['today:2026-11-20:e2,e1','tomorrow:2026-11-21:e3','date:2026-11-22:e5','date:2026-12-18:e4']);
});
test('My open homes follows each Rex event agent; Marketing Team is nobody’s',()=>{
 expect(keys(upcomingOpenHomes([mary,other],now,'mine',{id:'u-owen',name:'Owen Chen'}))).toEqual(['today:2026-11-20:e2','date:2026-11-22:e5']);
 expect(keys(upcomingOpenHomes([mary,other],now,'mine',{id:'u-mary',name:'Mary'}))).toEqual(['tomorrow:2026-11-21:e3']);
 expect(isMyOpenHome(mary,mary.ofi[1],{id:'u-mary',name:'Marketing Team'})).toBe(false);
});
test('My listings shows every session on my listings, whoever runs it',()=>{
 expect(keys(upcomingOpenHomes([mary,other],now,'listings',{id:'u-mary',name:'Mary'}))).toEqual(['today:2026-11-20:e2,e1','tomorrow:2026-11-21:e3','date:2026-12-18:e4']);
 expect(isMyListing(other,{id:'u-mary',name:'Mary'})).toBe(false);
});
test('local listings keep name matching; agent shown is the Rex event agent',()=>{
 const local:Listing={id:'l1',propertyId:'p3',agent:'Mary',price:'',status:'listed',ofi:[ev('2026-11-20','10:00–10:30')]};
 expect(isMyOpenHome(local,local.ofi[0],{id:'u-mary',name:'Mary'})).toBe(true);
 expect(isMyListing(local,{id:'u-mary',name:'Mary'})).toBe(true);
 expect(ofiAgent(local,local.ofi[0])).toBe('Mary');
 expect(ofiAgent(mary,mary.ofi[1])).toBe('Marketing Team');
});
test('sync keeps Rex agent names instead of the owner or the signed-in user',()=>{
 expect(mapRecord({id:'rex-listings-2057-9',ownerId:'u1',agent:'Marketing Team'},'listings',{u1:'Mary'},'Owen').agent).toBe('Marketing Team');
 expect(mapRecord({id:'rex-listings-2057-9',ownerId:'u1',agent:''},'listings',{u1:'Mary'},'Owen').agent).toBe('');
 expect(mapRecord({id:'l1',ownerId:'u1',agent:''},'listings',{u1:'Mary'},'Owen').agent).toBe('Mary');
});
test('Listed with <company>; sold or withdrawn Rex listings are not listed; P marker over pending and competitor',()=>{
 const p:Property={id:'p1',no:'16',street:'Gager St',suburb:'Sunnybank',ownerIds:[],pfOwner:'',lastSale:'',flags:{}};
 const t=(k:string)=>({listed_with:'Listed with {company}',listed_with_us:'Listed with our agency'} as Record<string,string>)[k];
 const sold={...mary,id:'s',status:'sold'};
 expect(placeListing([sold],'p1')).toBeUndefined();
 expect(listingStatusText(placeListing([sold,mary],'p1'),p,'Place',t)).toBe('Listed with Place');
 expect(listingStatusText(mary,p,'',t)).toBe('Listed with our agency');
 expect(listingStatusText(undefined,{...p,otherListing:true},'Place',t)).toBe('Listed with competitor');
 expect(listingStatusText(undefined,p,'Place',t)).toBe('Not listed');
 expect(markerKind({...p,pending:true,otherListing:true},true)).toBe('listed');
 expect(markerKind({...p,flags:{dnc:true}},true)).toBe('dnc');
});
test('competitor listing from market data: Listed with <agency> and competitor marker; own Rex listing and own company win',()=>{
 const p:Property={id:'p1',no:'16',street:'Gager St',suburb:'Sunnybank',ownerIds:[],pfOwner:'',lastSale:'',flags:{},competitorListing:{agency:'Harcourts Sunnybank',date:'2025-02-03',provider:'other'}};
 const t=(k:string)=>({listed_with:'Listed with {company}',listed_with_us:'Listed with our agency'} as Record<string,string>)[k];
 expect(listingStatusText(undefined,p,'Place',t)).toBe('Listed with Harcourts Sunnybank');
 expect(markerKind(p,false,'Place')).toBe('other');
 expect(listingStatusText(mary,p,'Place',t)).toBe('Listed with Place');
 expect(markerKind(p,true,'Place')).toBe('listed');
 expect(listingStatusText(undefined,p,'harcourts - sunnybank',t)).toBe('Not listed');
 expect(markerKind(p,false,'Harcourts Sunnybank')).toBe('dot');
 expect(markerKind({...p,competitorListing:{agency:'  '}},false,'Place')).toBe('dot');
});
