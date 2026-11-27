import {ownerRexSyncLines,rexSyncText} from './RexSyncLabel';
jest.mock('@react-native-async-storage/async-storage',()=>({__esModule:true,default:{setItem:jest.fn(async()=>{}),getItem:jest.fn(async()=>null)}}));
const en=(e:string)=>e;
const show=(lines:ReturnType<typeof ownerRexSyncLines>)=>lines.map(x=>`${x.label?x.label+' · ':''}${rexSyncText(x.value,en)}`);
const sent={status:'sent'},skipped={status:'skipped',reason:'not in Rex'};

test('contact and owner link in the same state show one line, not "In Rex" twice',()=>{
  expect(show(ownerRexSyncLines(true,sent,sent,en))).toEqual(['Contact and owner link · In Rex']);
});
test('different states keep both, each labelled',()=>{
  expect(show(ownerRexSyncLines(true,sent,skipped,en))).toEqual(['Contact · In Rex','Owner link · Rex skipped: not in Rex']);
  expect(show(ownerRexSyncLines(true,{status:'pending'},{status:'waiting_auth'},en))).toEqual(['Contact and owner link · Pending Rex']);
  expect(show(ownerRexSyncLines(true,undefined,skipped,en))).toEqual(['Owner link · Rex skipped: not in Rex']);
  expect(show(ownerRexSyncLines(true,sent,undefined,en))).toEqual(['Contact · In Rex']);
});
test('no owner contact keeps the property status unlabelled',()=>{
  expect(show(ownerRexSyncLines(false,undefined,sent,en))).toEqual(['In Rex']);
  expect(ownerRexSyncLines(false,undefined,undefined,en)).toEqual([]);
});
