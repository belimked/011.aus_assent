import {seed} from '../data/seed';
import {buyerProfileFromText} from './enq';
import {approveCapture,captureCandidates,captureIdentity} from './review';

test('offline buyer profile parses spoken ranges and recognised suburbs',()=>{
  expect(buyerProfileFromText('Budget 1.5 to 2 million in Parkinson, 4 bedrooms', ['Parkinson','Runcorn'])).toEqual({price:'$1.5m–$2m',beds:4,suburbs:'Parkinson'});
  expect(buyerProfileFromText('Looking for 4 beds in Runcorn $900k–1.1m', ['Runcorn'])).toEqual({price:'$900k–$1.1m',beds:4,suburbs:'Runcorn'});
  expect(buyerProfileFromText('Parkinson’s disease, no location given', ['Parkinson']).price).toBe('');
});
test('capture identity and candidates show name/phone/email and duplicates',()=>{
 const s=seed();const c=s.contacts[0];c.first='Mary';c.last='Chen';c.mobile='0412 555 019';c.address='12 Smith St';const copy={...c,id:'other',notes:[]};s.contacts.push(copy);
 const name=captureIdentity('Spoke to Mary Chen at the door');expect(name).toEqual({first:'Mary',last:'Chen'});
 expect(captureIdentity('Mary, mobile 0412 555 019')).toEqual({first:'Mary',last:''});
 const rows=captureCandidates(s,{...name,mobile:c.mobile,email:c.email});expect(rows.length).toBeGreaterThanOrEqual(2);
 expect(rows[0].by).toContain('name');expect(rows[0].by).toContain('phone');expect(rows[0].duplicate).toBe(true);
 expect(captureCandidates(s,{first:'',last:'',mobile:'',email:''})).toHaveLength(0);
 s.captures.push({id:'mary-voice',text:'Mary, mobile 0412 555 019',propId:s.properties[0].id,rec:'today',sync:'today'});
 approveCapture(s,'mary-voice',undefined,false,{...captureIdentity('Mary, mobile 0412 555 019')},c.id);
 expect(s.captures.some(x=>x.id==='mary-voice')).toBe(false);
});
