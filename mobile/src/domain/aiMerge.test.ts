import {extract} from './extract';
import {mergeAI} from './aiMerge';
import type {AIResult} from '../lib/voice';
const result=(fields:Partial<AIResult['fields']>,confidence:AIResult['confidence']={}):AIResult=>({fields:fields as AIResult['fields'],confidence,model:'test',latencyMs:1});
test('normalizes phone and maps seller intent while retaining parser tags',()=>{const r=mergeAI(extract('left card','dk'),result({phone:'0400000101',intent:'sell'}));expect(r.mobile).toBe('0400 000 101');expect(r.intent).toBe('Seller lead');expect(r.tags).toContain('Left card');expect(r.tags).toContain('Seller lead');});
test('keeps a Chinese full name in first',()=>{const r=mergeAI(extract('hello','oh'),result({contactName:'王丽'}));expect(r.first).toBe('王丽');expect(r.last).toBe('');});
test('nulls preserve local values and low confidence marks fields unsure',()=>{const r=mergeAI(extract('Spoke to Mary Chen 0412 555 019','dk'),result({firstName:null,phone:'0400000101',email:'test@example.com'},{phone:0.4,email:0.9}));expect(r.first).toBe('Mary');expect(r.mobile).toBe('0400 000 101');expect(r.unsure).toContain('mobile');expect(r.unsure).not.toContain('email');});
