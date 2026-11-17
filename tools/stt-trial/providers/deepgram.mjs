import { readFile } from 'node:fs/promises';
import { config } from '../config.mjs';
import { checked, pace } from '../util.mjs';
export function endpoint(mode) { const u = new URL(mode === 'stream' ? config.deepgram.stream : config.deepgram.batch); for (const [k, v] of Object.entries(config.deepgram.params)) u.searchParams.set(k, v); if (mode === 'stream') for (const [k, v] of Object.entries({ encoding: 'linear16', sample_rate: '16000', interim_results: 'true' })) u.searchParams.set(k, v); return u.toString(); }
export async function batch(file) { const started = performance.now(); const raw = await (await checked(await fetch(endpoint('batch'), { method: 'POST', headers: { Authorization: `Token ${process.env.DEEPGRAM_API_KEY}`, 'Content-Type': 'audio/wav' }, body: await readFile(file) }))).json(); return { text: raw.results?.channels?.[0]?.alternatives?.[0]?.transcript || '', segments: raw.results?.channels?.[0]?.alternatives?.[0]?.words, detectedLang: raw.results?.channels?.[0]?.detected_language, raw, latencyMs: performance.now() - started }; }
export async function stream(buffer, { onPartial = () => {} } = {}) {
  const start = performance.now(); let firstPartialMs = null, ended, text = '', raw = [];
  const ws = new WebSocket(endpoint('stream'), ['token', process.env.DEEPGRAM_API_KEY]);
  return new Promise((resolve, reject) => { let done = false; const fail = e => { if (!done) { done = true; reject(e instanceof Error ? e : new Error(String(e))); ws.close(); } }; const timeout = setTimeout(() => fail(new Error('Deepgram stream timeout')), Math.max(30000, buffer.length / 32 + 20000));
    ws.onopen = async () => { try { await pace(buffer, chunk => ws.send(chunk)); ended = performance.now(); ws.send(JSON.stringify({ type: 'Finalize' })); setTimeout(() => ws.readyState === WebSocket.OPEN && ws.send(JSON.stringify({ type: 'CloseStream' })), 1500); } catch (e) { fail(e); } };
    ws.onmessage = e => { try { const m = JSON.parse(e.data); raw.push(m); const t = m.channel?.alternatives?.[0]?.transcript; if (t) { if (firstPartialMs === null) firstPartialMs = performance.now() - start; onPartial(t); if (m.is_final) text += (text ? ' ' : '') + t; } } catch (e) { fail(e); } };
    ws.onerror = () => fail(new Error('Deepgram WebSocket 错误')); ws.onclose = () => { clearTimeout(timeout); if (!done) { done = true; resolve({ text, firstPartialMs, finalAfterEndMs: performance.now() - (ended || start), raw }); } };
  });
}
