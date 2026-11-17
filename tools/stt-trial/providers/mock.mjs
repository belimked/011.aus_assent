import { readFile } from 'node:fs/promises';
const manifest = JSON.parse(await readFile(new URL('../manifest.json',import.meta.url)));
export const variants = ['default'];
const disturb = s => s.replace(/，|。|\.|,/g, '').replace(/Sunnybank/i, 'sunnybank');
export async function batch(_file, { manifest }) { return { text: disturb(manifest.reference), raw: { simulated:true, referenceId:manifest.id }, latencyMs: 15 }; }
export async function stream(_buffer, { manifest, onPartial = () => {} }) { onPartial(manifest.reference.slice(0,10)); return { text: disturb(manifest.reference), raw: { simulated:true, referenceId:manifest.id }, firstPartialMs: 5, finalAfterEndMs: 8 }; }
export function extract({ clip }) { const id=clip.match(/^[A-E]\d\d/)?.[0]; return { value: id ? manifest.find(m=>m.id===id)?.expected || {} : {}, raw:{ simulated:true }, latencyMs:1, inputTokens:10, outputTokens:20, costAud:0 }; }
