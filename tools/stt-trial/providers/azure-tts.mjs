import { writeFile } from 'node:fs/promises';
import { config } from '../config.mjs';
import { checked } from '../util.mjs';
export async function tts(lang, out) {
  const region = process.env.AZURE_SPEECH_REGION || config.azure.region; const voice = config.azure.voices[lang];
  const text = { 'en-AU': 'Hello, this is a fictional open home in Sunnybank.', 'zh-CN': '您好，欢迎来到虚构的周六开放日。', 'zh-HK': '你好，歡迎嚟到星期六開放日。' }[lang];
  const escape = s => s.replaceAll('&', '&amp;').replaceAll('<', '&lt;');
  const start = performance.now(); const res = await checked(await fetch(`https://${region}.tts.speech.microsoft.com${config.azure.ttsPath}`, { method: 'POST', headers: { 'Ocp-Apim-Subscription-Key': process.env.AZURE_SPEECH_KEY, 'Content-Type': 'application/ssml+xml', 'X-Microsoft-OutputFormat': 'riff-16khz-16bit-mono-pcm' }, body: `<speak version="1.0" xml:lang="${lang}"><voice name="${voice}">${escape(text)}</voice></speak>` }));
  const reader = res.body.getReader(); const first = await reader.read(); const firstByteMs = performance.now() - start; const chunks = first.done ? [] : [first.value]; for (;;) { const next = await reader.read(); if (next.done) break; chunks.push(next.value); }
  await writeFile(out, Buffer.concat(chunks.map(c => Buffer.from(c)))); return { lang, voice, firstByteMs, latencyMs: performance.now() - start, file: out };
}
