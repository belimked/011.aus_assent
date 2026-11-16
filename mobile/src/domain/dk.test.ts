import {matchesAddress,houseNumberOrder,isLongHold} from './dk';
describe('DK map and hold logic',()=>{
 test('address search ignores case and trims input',()=>{expect(matchesAddress('12 Gager St, Sunnybank',' gAgEr ')).toBe(true);expect(matchesAddress('12 Gager St','Main')).toBe(false)});
 test('sorts by numeric house number',()=>{expect([{no:'20'},{no:'3'},{no:'11'}].sort(houseNumberOrder).map(p=>p.no)).toEqual(['3','11','20'])});
 test('260ms is the long press boundary',()=>{expect(isLongHold(259)).toBe(false);expect(isLongHold(260)).toBe(true)});
});
