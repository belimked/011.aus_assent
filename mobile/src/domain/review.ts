import type {AppState} from '../data/types';
import {today} from '../lib/today';
import {extract} from './extract';
import {commitDK,dkNeedsName} from './dk';
import {audit,byId,matchContacts,normMobile,noteOn,ownersOf,propAddr} from './index';

export function captureIdentity(text:string, first='',last=''){
  if(first)return {first,last};
  const named=text.match(/\b(?:spoke (?:to|with)|talked to|met|name is|this is|with)\s+([A-Z][a-z]+)(?:\s+([A-Z][a-z]+))?\b/i);
  if(named)return {first:named[1],last:named[2]||''};
  const leading=text.match(/^([A-Z][a-z]+)(?:\s+([A-Z][a-z]+))?\s*[,.:]?\s*(?:mobile|phone|0[45]\d|$)/);
  return leading?{first:leading[1],last:leading[2]||''}:{first,last};
}
export function captureCandidates(s:AppState,identity:{first:string;last:string;mobile:string;email:string}){
  const mobile=normMobile(identity.mobile),email=identity.email.trim().toLowerCase();
  return s.contacts.flatMap(c=>{const by:string[]=[];
    if(mobile.length>=10&&normMobile(c.mobile)===mobile)by.push('phone');
    if(email&&c.email.toLowerCase()===email)by.push('email');
    if(identity.first&&c.first.toLowerCase()===identity.first.toLowerCase()&&(!identity.last||c.last.toLowerCase()===identity.last.toLowerCase()))by.push('name');
    return by.length?[{c,by,duplicate:s.contacts.some(other=>other.id!==c.id&&other.first.toLowerCase()===c.first.toLowerCase()&&other.last.toLowerCase()===c.last.toLowerCase()&&!!normMobile(c.mobile)&&normMobile(other.mobile)===normMobile(c.mobile))}]:[];
  });
}

/** What approveCapture would file: null for a visit (no identity → property owner), else the commitDK fields and pick. */
function captureFiling(s:AppState,id:string,transcript?:string,fields?:Partial<ReturnType<typeof extract>>,pick?:string){
  const capture=byId(s.captures,id);if(!capture)return undefined;
  const t=transcript??capture.text;
  const r={...extract(t,'dk',s),...fields};
  const matches=matchContacts(s,r);
  if(!pick&&matches.status==='none'&&!r.mobile&&!r.email&&!r.first)return {capture,t,r,visit:true as const};
  const existing=pick?pick==='new'?undefined:byId(s.contacts,pick):matches.status==='unique'?matches.list[0].c:undefined;
  const dk={first:r.first||existing?.first||'',last:r.last||existing?.last||'',mobile:r.mobile||existing?.mobile||'',email:r.email||existing?.email||'',relation:r.relation,note:r.note,reason:r.reason,tags:r.tags,followup:r.followup,linked:r.linked};
  return {capture,t,r,visit:false as const,matches,dk};
}
/** True when approving would create a contact with no first or last name; approveCapture refuses and keeps the capture. */
export function captureNeedsName(s:AppState,id:string,transcript?:string,fields?:Partial<ReturnType<typeof extract>>,pick?:string){
  const f=captureFiling(s,id,transcript,fields,pick);
  return !!f&&!f.visit&&dkNeedsName(s,f.dk,pick);
}

export function approveCapture(s:AppState,id:string,transcript?:string,keepApproved=false,fields?:Partial<ReturnType<typeof extract>>,pick?:string){
  const f=captureFiling(s,id,transcript,fields,pick);if(!f)return;
  const {capture,t,r}=f;
  const p=byId(s.properties,capture.propId);
  // A visit has no contact identity; it belongs to the property's owner, not a new contact.
  if(f.visit){
    if(p){p.visits=[...(p.visits||[]),{d:today(),o:r.tags.includes('Not at home')?'not_home':r.tags.includes('Left card')?'left_card':'spoke'}];const owner=ownersOf(s,p)[0];if(owner){owner.tags=[...new Set([...owner.tags,...r.tags])];noteOn(s,owner,t,{propId:p.id})}}
    audit(s,'DK',`${p?propAddr(p):capture.propId} · visit logged`);
  }else{
    if(f.matches.status==='multiple'&&!pick)throw new Error('Choose a matching contact before approving');
    // commitDK throws 'contact name required' before any change, so the capture stays in the queue.
    commitDK(s,capture.propId,f.dk,pick);
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
      for(const c of owners){c.roles=[...new Set([...c.roles.filter(role=>role!=='owner'),'previous owner'])];c.tags=c.tags.filter(tag=>tag!=='Seller lead');c.sellerLeadStatus='Sold';c.sellerStatus='Sold';c.rels=[...(c.rels||[]),{type:'Previous owner of',name:m.addr}];noteOn(s,c,`Market monitor: ${propAddr(p)} sold ${m.meta}. Moved to previous owner, seller lead status Sold.`,{sys:'market',propId:p.id})}
      p.ownerIds=[];p.pfOwner='';p.flags.dnc=false;p.pending=true;
      const date=m.meta.match(/\b(\d{1,2})\s+(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)(?:\s+(\d{4}))?/i);
      const month=date?String(['jan','feb','mar','apr','may','jun','jul','aug','sep','oct','nov','dec'].indexOf(date[2].toLowerCase())+1).padStart(2,'0'):'';
      const year=date?.[3]||today().slice(0,4);
      const soldDate=date?`${year}-${month}-${date[1].padStart(2,'0')}`:'';
      const priceMatch=m.meta.match(/\$\s*([\d,.]+)\s*([mk])?/i);
      const price=priceMatch?Math.round(Number(priceMatch[1].replace(/,/g,''))*(priceMatch[2]?.toLowerCase()==='m'?1e6:priceMatch[2]?.toLowerCase()==='k'?1e3:1)):null;
      if(soldDate)p.lastSale=soldDate.slice(0,7);
      p.market={...p.market,lastSalePrice:price??p.market?.lastSalePrice??null,lastSaleDate:soldDate||p.market?.lastSaleDate||'',source:p.market?.source||'RP Data',updatedAt:today()};
    }else if(m.kind==='listing'){
      p.otherListing=true;
      for(const c of ownersOf(s,p)){c.sellerLeadStatus='Listed with competitor';c.sellerStatus='Listed with competitor';noteOn(s,c,`Market monitor: ${propAddr(p)} listed with ${m.agency||'a competitor'}. Seller lead status: Listed with competitor.`,{sys:'market',propId:p.id})}
    }
  }
  audit(s,'Market monitor',`${m.addr} · ${m.kind==='sale'?'ownership changed, Sold':'Listed with competitor'}`);
  s.market=s.market.filter(x=>x.id!==id);
}
