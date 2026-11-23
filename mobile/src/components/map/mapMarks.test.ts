import {markerKind} from './mapMarks';
import type {Property} from '../../data/types';
const p=(extra:Partial<Property>):Property=>({id:'1',no:'1',street:'St',suburb:'Test',ownerIds:[],pfOwner:'',lastSale:'',flags:{},...extra});
test('DNC overrides pins and listings; competitor listing differs from blue other-agent owner',()=>{
 expect(markerKind(p({flags:{dnc:true},pinned:true}),true)).toBe('dnc');
 expect(markerKind(p({otherListing:true}),false)).toBe('other');
 expect(markerKind(p({otherAgent:true,ownerIds:['c']}),false)).toBe('dot');
 expect(markerKind(p({pending:true}),false)).toBe('pending');
 expect(markerKind(p({}),false)).toBe('dot');
});
