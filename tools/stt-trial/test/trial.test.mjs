import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { join } from 'node:path';
import { mer, normalize, tokens, fieldAccuracy } from '../score.mjs';
import { signingKey, validateModel, sign } from '../providers/bedrock.mjs';
import { endpoint } from '../providers/deepgram.mjs';
import { run } from '../run.mjs';
test('MER 中英混合及归一化', () => { assert.equal(mer('你好 Sunnybank','你好 sunnybank'),0); assert.equal(mer('你好 world','你 world'),1/3); assert.deepEqual(tokens('ＡＢＣ，萬'),['abc','万']); });
test('中英数字归一化（有限词汇）', () => { assert.equal(normalize('1.2 million'), '1200000'); assert.equal(normalize('one point five million'),'1500000'); assert.equal(normalize('一百五十萬'),'1500000'); assert.equal(normalize('一百二十万'),'1200000'); assert.equal(fieldAccuracy({phone:'0400 000 123',priceMin:1200000},{phone:'0400000123',priceMin:1200000}),1); });
test('AWS 官方 SigV4 示例密钥派生向量 (20150830/us-east-1/iam)', () => { // AWS IAM Signature Version 4 examples: AWS4 + wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY
 const key=signingKey('wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY','20150830','us-east-1','iam'); assert.equal(key.toString('hex'),'c4afb1cc5771d871763a393e44b703571b55cc28424d1a5e86da6ed3c154a4b9'); const h=sign({path:'/model/au.test/converse',body:'{}',region:'ap-southeast-2',accessKey:'AKIDEXAMPLE',secretKey:'secret',date:new Date('2025-01-01T00:00:00Z')}); assert.match(h.Authorization,/SignedHeaders=content-type;host;x-amz-date/); });
test('Deepgram 所有端点强制 MIP opt-out', () => { for (const mode of ['batch','stream']) assert.equal(new URL(endpoint(mode)).searchParams.get('mip_opt_out'),'true'); });
test('拒绝 global. 和其他非 au. 推理配置', () => { assert.throws(()=>validateModel('global.anthropic.foo'),/禁止 global/); assert.throws(()=>validateModel('us.foo'),/au/); assert.equal(validateModel('au.anthropic.foo'),'au.anthropic.foo'); });
test('mock batch + stream + extract + score', async () => { const out=await run(['all','--mock','--only','B01,C01']); const lines=(await readFile(join(out,'results.jsonl'),'utf8')).trim().split('\n').map(JSON.parse); assert.equal(lines.filter(r=>r.mode==='batch').length,2); assert.equal(lines.filter(r=>r.mode==='stream').length,2); assert.ok(lines.some(r=>r.mode==='extract' && r.provider==='reference')); assert.match(await readFile(join(out,'report.md'),'utf8'),/关键词命中/); });
