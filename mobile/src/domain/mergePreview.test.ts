import {confirmLabel,consentLines,dncLine,fieldReason,fieldValue,offlineMessage,primaryReason,relationLines,rexLine,side,IRREVERSIBLE,type MergePreview} from './mergePreview';

const en=(english:string)=>english;
const zh=(_:string,chinese:string)=>chinese;
const preview=(over:Partial<MergePreview>={}):MergePreview=>({
  primaryId:'a',secondaryId:'b',primaryReason:'older',
  contacts:[{id:'a',name:'Ann',mobile:'',email:'',createdAt:'',updatedAt:'',rexLinked:false},{id:'b',name:'Ann',mobile:'',email:'',createdAt:'',updatedAt:'',rexLinked:false}],
  fields:[],dnc:{value:false,a:false,b:false},consent:[],notes:[],tags:[],roles:[],properties:[],relations:[],logNote:'',previewHash:'h',rex:'none',...over,
});

test('field lines mask contact details and explain which side won',()=>{
  const p=preview();
  expect(fieldValue('mobile','0412345678')).toBe('0412 345 ••••');
  expect(fieldValue('email','jane@example.com')).toBe('ja••••@example.com');
  expect(fieldValue('first','')).toBe('—');
  expect(fieldReason({key:'first',value:'Janet',from:'b',a:'Jane',b:'Janet',reason:'newer'},p,en)).toBe('B is newer');
  expect(fieldReason({key:'last',value:'Doe',from:'a',a:'Doe',b:'',reason:'only'},p,zh)).toBe('仅 A 有值');
  expect(side(p,'x')).toBe('A+B');
});

test('primary reason, compliance and relation lines',()=>{
  expect(primaryReason(preview({primaryReason:'rex'}),en)).toMatch(/linked to Rex/);
  expect(primaryReason(preview(),zh)).toMatch(/创建得更早/);
  expect(dncLine(preview({dnc:{value:true,a:false,b:true}}),en)).toBe('Do not contact: yes (from B; either one wins)');
  expect(dncLine(preview(),en)).toBe('Do not contact: no');
  expect(consentLines(preview({consent:[{channel:'sms',value:0,a:1,b:0},{channel:'email',value:1,a:1,b:1}]}),en)).toEqual(['Text: opted out','Email: allowed']);
  expect(relationLines(preview({properties:[
    {propertyId:'p1',address:'3 Feltwell St',relation:'owner',label:'Owner',at:'',from:'b',changed:true,replaced:[{relation:'tenant',label:'Tenant',from:'a'}]},
    {propertyId:'p2',address:'',relation:'tenant',label:'Tenant',at:'',from:'a',changed:false},
  ]}),en)).toEqual(['3 Feltwell St · Owner (newest, replaces Tenant)','p2 · Tenant']);
});

test('irreversible wording, offline guard and Rex outcome are bilingual',()=>{
  expect(IRREVERSIBLE).toEqual(['This cannot be undone.','合并后无法撤销。']);
  expect(confirmLabel(false,en)).toBe('Merge · cannot be undone');
  expect(confirmLabel(true,zh)).toBe('正在合并…');
  expect(offlineMessage(zh)).toMatch(/联网/);
  expect(rexLine(preview({rex:'both'}),en)).toMatch(/combines them into A/);
  expect(rexLine(preview({rex:'secondary'}),en)).toMatch(/takes over/);
  expect(rexLine(preview(),zh)).toMatch(/不会写入 Rex/);
});
