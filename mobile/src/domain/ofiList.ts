import type {Listing} from '../data/types';
import {addLocalDays,today} from '../lib/today';

export type OfiFilter='all'|'mine'|'listings';
export type Me={id?:string;name:string};
export type OfiItem={l:Listing;o:any;index:number;key:string};
export type OfiGroup={d:string;day:'today'|'tomorrow'|'date';items:OfiItem[]};

const rex=(l:Listing)=>String(l.id).startsWith('rex-listings-');
/** Agent running this open home: the one picked on the Rex event (agentFrom 'listing' when Rex left it empty); local events fall back to the listing agent. */
export const ofiAgent=(l:Listing,o:any):string=>o.agent??l.agent??'';
/** "My open homes": the Rex event's agent is me. A shared Rex login (e.g. Marketing Team) has no app user and is nobody's. */
export function isMyOpenHome(l:Listing,o:any,me:Me){if('agentId' in o||rex(l))return me.id?o.agentId===me.id:!!o.agent&&o.agent===me.name;return l.agent===me.name;}
/** "My listings": I am a listing agent in Rex (listing_agent_1/2), or the agent of a local listing. */
export function isMyListing(l:Listing,me:Me){if(Array.isArray(l.agentIds)||rex(l))return me.id?(l.agentIds||[]).includes(me.id):!!l.agent&&l.agent===me.name;return l.agent===me.name;}
/** All open homes from today on, filtered and grouped Today / Tomorrow / by date, in time order. */
export function upcomingOpenHomes(listings:Listing[],now:Date,filter:OfiFilter,me:Me):OfiGroup[]{
 const day=today(now),next=addLocalDays(now,1);
 const items=listings.flatMap(l=>(filter==='listings'&&!isMyListing(l,me))?[]:(l.ofi||[]).map((o:any,index:number)=>({l,o,index,key:`${l.id}-${o.rexEventId||index}`}))).filter(x=>x.o?.d>=day&&(filter!=='mine'||isMyOpenHome(x.l,x.o,me))).sort((a,b)=>`${a.o.d} ${a.o.time}`.localeCompare(`${b.o.d} ${b.o.time}`));
 const groups:OfiGroup[]=[];for(const x of items){const last=groups[groups.length-1];if(last?.d===x.o.d)last.items.push(x);else groups.push({d:x.o.d,day:x.o.d===day?'today':x.o.d===next?'tomorrow':'date',items:[x]});}
 return groups;
}
