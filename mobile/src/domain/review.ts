import type {AppState} from '../data/types';
import {TODAY} from '../data/seed';
import {extract} from './extract';
import {commitDK} from './dk';
import {audit,byId,matchContacts,noteOn,ownersOf,propAddr} from './index';

export function approveCapture(s:AppState,id:string,transcript?:string,keepApproved=false,fields?:Partial<ReturnType<typeof extract>>,pick?:string){
  const capture=byId(s.captures,id);if(!capture)return;
  const t=transcript??capture.text;
  const r={...extract(t,'dk',s),...fields};
  const p=byId(s.properties,capture.propId);
  const matches=matchContacts(s,r);
  // A visit has no contact identity; it belongs to the property's owner, not a new contact.
  if(matches.status==='none'&&!r.mobile&&!r.email&&(!r.first||!r.last)){
    if(p){p.visits=[...(p.visits||[]),{d:TODAY,o:r.tags.includes('Not at home')?'not_home':r.tags.includes('Left card')?'left_card':'spoke'}];const owner=ownersOf(s,p)[0];if(owner){owner.tags=[...new Set([...owner.tags,...r.tags])];noteOn(s,owner,t)}}
    audit(s,'DK',`${p?propAddr(p):capture.propId} · visit logged`);
  }else{
    if(matches.status==='multiple'&&!pick)throw new Error('Choose a matching contact before approving');
    const existing=pick&&pick!=='new'?byId(s.contacts,pick):matches.status==='unique'?matches.list[0].c:undefined;
    commitDK(s,capture.propId,{first:r.first||existing?.first||'',last:r.last||existing?.last||'',mobile:r.mobile||existing?.mobile||'',email:r.email||existing?.email||'',relation:r.relation,note:r.note,reason:r.reason,tags:r.tags,followup:r.followup,linked:r.linked},pick);
  }
  s.captures=s.captures.filter(c=>c.id!==id);
  if(keepApproved)s.captures.push({...capture,text:t,status:'approved'});
}

export function confirmMarket(s:AppState,id:string){
  const m=byId(s.market,id);if(!m)return;
  const p=byId(s.properties,m.propId);
  if(p){
    if(m.kind==='sale'){
      const owners=ownersOf(s,p);
      p.previousOwnerIds=[...new Set([...(p.previousOwnerIds||[]),...p.ownerIds])];
      for(const c of owners){c.roles=[...new Set([...c.roles.filter(role=>role!=='owner'),'previous owner'])];c.tags=c.tags.filter(tag=>tag!=='Seller lead');c.sellerLeadStatus='Sold';c.sellerStatus='Sold';c.rels=[...(c.rels||[]),{type:'Previous owner of',name:m.addr}];noteOn(s,c,`Market monitor: ${propAddr(p)} sold ${m.meta}. Moved to previous owner, seller lead status Sold.`)}
      p.ownerIds=[];p.pfOwner='';p.flags.dnc=false;p.pending=true;
      const date=m.meta.match(/\b(\d{1,2})\s+(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)(?:\s+(\d{4}))?/i);
      const month=date?String(['jan','feb','mar','apr','may','jun','jul','aug','sep','oct','nov','dec'].indexOf(date[2].toLowerCase())+1).padStart(2,'0'):'';
      const year=date?.[3]||TODAY.slice(0,4);
      const soldDate=date?`${year}-${month}-${date[1].padStart(2,'0')}`:'';
      const priceMatch=m.meta.match(/\$\s*([\d,.]+)\s*([mk])?/i);
      const price=priceMatch?Math.round(Number(priceMatch[1].replace(/,/g,''))*(priceMatch[2]?.toLowerCase()==='m'?1e6:priceMatch[2]?.toLowerCase()==='k'?1e3:1)):null;
      if(soldDate)p.lastSale=soldDate.slice(0,7);
      p.market={...p.market,lastSalePrice:price??p.market?.lastSalePrice??null,lastSaleDate:soldDate||p.market?.lastSaleDate||'',source:'Pricefinder',updatedAt:TODAY};
    }else if(m.kind==='listing'){
      p.otherListing=true;
      for(const c of ownersOf(s,p)){c.sellerLeadStatus='Listed with competitor';c.sellerStatus='Listed with competitor';noteOn(s,c,`Market monitor: ${propAddr(p)} listed with ${m.agency||'a competitor'}. Seller lead status: Listed with competitor.`)}
    }
  }
  audit(s,'Market monitor',`${m.addr} · ${m.kind==='sale'?'ownership changed, Sold':'Listed with competitor'}`);
  s.market=s.market.filter(x=>x.id!==id);
}
