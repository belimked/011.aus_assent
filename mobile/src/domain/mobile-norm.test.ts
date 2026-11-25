import {seed} from '../data/seed';
import {matchContacts,normMobile} from './index';

test('AU mobile formats resolve to the same contact',()=>{
 const formats=['0401 999 088','+61 401 999 088','61401999088','0401999088'];
 const state=seed();state.contacts[0].mobile=formats[0];
 for(const mobile of formats){expect(normMobile(mobile)).toBe('0401999088');expect(matchContacts(state,{mobile}).list[0]?.c.id).toBe(state.contacts[0].id)}
});
