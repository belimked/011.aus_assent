import {seed} from '../data/seed';
import {approveCapture,confirmMarket} from './review';
import {activityRows,periodRange} from './activity';
import {ensureParcelRecord} from './map';
import {properties as parcels} from '../data/properties';

test('FY begins July and activity filters current and previous periods',()=>{expect(periodRange('fy',new Date(2026,0,10))).toEqual({start:'2025-07-01',end:'2026-07-01',previous:'2024-07-01'});const entries=[{d:'2026-11-07 09:00',a:'DK',t:'a'},{d:'2026-11-06 09:00',a:'DK',t:'b'}];expect(activityRows(entries,'voice','today',new Date(2026,10,7))).toHaveLength(1);expect(activityRows(entries,'voice','today',new Date(2026,10,7),true)).toHaveLength(1)});
test('approving capture files extracted fields',()=>{const s=seed();const p=s.properties[0];s.captures.push({id:'test-capture',text:'Spoke to Sample Person, mobile 0412 999 888, might sell next year.',rec:'today',sync:'today',propId:p.id});approveCapture(s,'test-capture');expect(s.captures.some(c=>c.id==='test-capture')).toBe(false);expect(s.contacts.some(c=>c.mobile.replace(/\D/g,'')==='0412999888')).toBe(true)});
test('background map parcel creates one local record for notes and navigation',()=>{const s=seed();const parcel=parcels[0];const a=ensureParcelRecord(s,parcel);expect(ensureParcelRecord(s,parcel).id).toBe(a.id);expect(a.ll).toEqual([parcel.lat,parcel.lng])});
test('confirming sale retains previous owners and updates seller status',()=>{const s=seed();const p=s.properties.find(x=>x.ownerIds.length)!;const id=p.ownerIds[0];const owner=s.contacts.find(x=>x.id===id)!;owner.tags.push('Seller lead');s.market.push({id:'test-sale',kind:'sale',propId:p.id,addr:p.street,meta:'',sub:'',owner:owner.first});confirmMarket(s,'test-sale');expect(p.ownerIds).toEqual([]);expect(p.previousOwnerIds).toContain(id);expect(owner.sellerLeadStatus).toBe('Sold')});
