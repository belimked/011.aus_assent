import { createHash, createHmac } from 'node:crypto';
import { config, tokenCost } from '../config.mjs';
import { checked, json } from '../util.mjs';
const hash = s => createHash('sha256').update(s).digest('hex');
const hmac = (key, text) => createHmac('sha256', key).update(text).digest();
export function validateModel(id) { if (id.startsWith('global.')) throw new Error('禁止 global. 推理配置：必须使用 au.'); if (!id.startsWith('au.')) throw new Error('只允许 au. 澳洲推理配置'); return id; }
export function signingKey(secret, day, region, service = 'bedrock') { return hmac(hmac(hmac(hmac(`AWS4${secret}`, day), region), service), 'aws4_request'); }
export function sign({ method = 'POST', path, body, region, service = 'bedrock', accessKey, secretKey, sessionToken, date = new Date() }) {
  const stamp = date.toISOString().replace(/[:-]|\.\d{3}/g, ''); const day = stamp.slice(0, 8); const host = `${service}-runtime.${region}.amazonaws.com`;
  const headers = { 'content-type': 'application/json', host, 'x-amz-date': stamp }; if (sessionToken) headers['x-amz-security-token'] = sessionToken;
  const names = Object.keys(headers).sort(); const canonical = [method, path, '', names.map(n => `${n}:${headers[n].trim()}\n`).join(''), names.join(';'), hash(body)].join('\n');
  const scope = `${day}/${region}/${service}/aws4_request`; const stringToSign = `AWS4-HMAC-SHA256\n${stamp}\n${scope}\n${hash(canonical)}`;
  const signature = hmac(signingKey(secretKey, day, region, service), stringToSign).toString('hex');
  return { ...headers, Authorization: `AWS4-HMAC-SHA256 Credential=${accessKey}/${scope}, SignedHeaders=${names.join(';')}, Signature=${signature}` };
}
export const schema = { type: 'object', properties: Object.fromEntries(['contactName','phone','email','address','suburb','intent','priceMin','priceMax','timeframe','notes','language'].map(k => [k, k.startsWith('price') ? { type: ['number','null'] } : { type: ['string','null'], ...(k === 'intent' ? { enum: ['buy','sell','rent','appraisal','other',null] } : {}) }])), required: ['contactName','phone','email','address','suburb','intent','priceMin','priceMax','timeframe','notes','language','confidence'], additionalProperties: false };
schema.properties.confidence = { type: 'object', properties: Object.fromEntries(Object.keys(schema.properties).map(k => [k, { type: 'number', minimum: 0, maximum: 1 }])), required: Object.keys(schema.properties), additionalProperties: false };
export async function extract({ text = '', image, modelId = process.env.BEDROCK_MODEL_ID || config.bedrock.modelId }) {
  validateModel(modelId); const region = process.env.AWS_REGION || config.bedrock.region;
  const content = [{ text: `从以下中英粤房产中介笔记抽取线索；不确定用 null，不要编造。\n${text}` }]; if (image) content.push({ image: { format: image.format, source: { bytes: image.bytes.toString('base64') } } });
  const body = JSON.stringify({ system: [{ text: 'You extract real estate contacts in English, Mandarin and Cantonese. Return only the tool schema; preserve the speaker’s facts, not guesses.' }], messages: [{ role: 'user', content }], inferenceConfig: { maxTokens: 1024, temperature: 0 }, toolConfig: { tools: [{ toolSpec: { name: 'record_contact', description: 'Extract a lead', inputSchema: { json: schema } } }], toolChoice: { tool: { name: 'record_contact' } } } });
  const path = `/model/${encodeURIComponent(modelId)}/converse`; const url = `https://bedrock-runtime.${region}.amazonaws.com${path}`;
  const headers = process.env.AWS_BEARER_TOKEN_BEDROCK ? { Authorization: `Bearer ${process.env.AWS_BEARER_TOKEN_BEDROCK}`, 'content-type': 'application/json' } : sign({ path, body, region, accessKey: process.env.AWS_ACCESS_KEY_ID, secretKey: process.env.AWS_SECRET_ACCESS_KEY, sessionToken: process.env.AWS_SESSION_TOKEN });
  const start = performance.now(); const raw = await (await checked(await fetch(url, { method: 'POST', headers, body }))).json();
  const value = raw.output?.message?.content?.find(c => c.toolUse)?.toolUse?.input; if (!value) throw new Error('Bedrock 未返回 toolUse JSON');
  return { value: typeof value === 'string' ? json(value) : value, raw, latencyMs: performance.now() - start, inputTokens: raw.usage?.inputTokens || 0, outputTokens: raw.usage?.outputTokens || 0, costAud: tokenCost(raw.usage?.inputTokens || 0, raw.usage?.outputTokens || 0) };
}
