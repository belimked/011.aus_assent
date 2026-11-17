import { readFile } from 'node:fs/promises';
import { config } from '../config.mjs';
import { checked } from '../util.mjs';
export const variants = Object.keys(config.azure.variants);
export async function batch(file, { langs, variant = 'multi' } = {}) {
  const started = performance.now(); const locales = variant in config.azure.variants ? config.azure.variants[variant] : langs;
  const form = new FormData(); form.append('audio', new Blob([await readFile(file)]), file.split('/').at(-1)); form.append('definition', JSON.stringify(locales ? { locales } : {}));
  const region = process.env.AZURE_SPEECH_REGION || config.azure.region;
  const raw = await (await checked(await fetch(`https://${region}.api.cognitive.microsoft.com${config.azure.fastPath}`, { method: 'POST', headers: { 'Ocp-Apim-Subscription-Key': process.env.AZURE_SPEECH_KEY }, body: form }))).json();
  return { text: raw.combinedPhrases?.map(p => p.text).join(' ') || raw.phrases?.map(p => p.text).join(' ') || '', segments: raw.phrases, detectedLang: raw.phrases?.[0]?.locale, raw, latencyMs: performance.now() - started };
}
