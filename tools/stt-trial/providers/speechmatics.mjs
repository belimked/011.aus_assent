import { readFile } from 'node:fs/promises';
import { config } from '../config.mjs';
import { checked, pause, pace } from '../util.mjs';
export const variants = Object.keys(config.speechmatics.variants);
const auth = () => ({ Authorization: `Bearer ${process.env.SPEECHMATICS_API_KEY}` });
export async function batch(file, { variant = 'auto' } = {}) {
  const start = performance.now(); const form = new FormData(); form.append('data_file', new Blob([await readFile(file)]), file.split('/').at(-1)); form.append('config', JSON.stringify({ type: 'transcription', transcription_config: config.speechmatics.variants[variant] }));
  const submitted = await (await checked(await fetch(config.speechmatics.batch, { method: 'POST', headers: auth(), body: form }))).json(); const id = submitted.id;
  if (!id) throw new Error('Speechmatics 未返回 job id');
  let job; for (let i = 0; i < 60; i++) { await pause(2000); job = await (await checked(await fetch(`${config.speechmatics.batch}/${encodeURIComponent(id)}`, { headers: auth() }))).json(); if (job.job?.status === 'done') break; if (['rejected','failed'].includes(job.job?.status)) throw new Error(`Speechmatics job ${id}: ${job.job.status}`); }
  if (job.job?.status !== 'done') throw new Error(`Speechmatics job ${id} 轮询超时`);
  const raw = await (await checked(await fetch(`${config.speechmatics.batch}/${encodeURIComponent(id)}/transcript?format=json-v2`, { headers: auth() }))).json();
  return { text: raw.results?.flatMap(r => r.alternatives?.map(a => a.content) || []).join('') || '', segments: raw.results, raw, latencyMs: performance.now() - start };
}
export async function stream(buffer, { variant = 'auto', onPartial = () => {} } = {}) {
  const start = performance.now(); let ended, firstPartialMs = null, text = '', raw = [];
  const temp = await (await checked(await fetch(config.speechmatics.tempKey, { method:'POST', headers: { ...auth(), 'Content-Type':'application/json' }, body:JSON.stringify({ttl:60}) }))).json();
  if (!temp.key_value) throw new Error('Speechmatics 无法签发临时实时 key（联系账号管理员开通）');
  const url = new URL(config.speechmatics.stream); url.searchParams.set('jwt', temp.key_value);
  const ws = new WebSocket(url); return new Promise((resolve, reject) => { let done = false; const timer = setTimeout(() => fail(new Error('Speechmatics stream timeout')), Math.max(30000, buffer.length / 32 + 20000));
    function fail(e) { if (!done) { done = true; clearTimeout(timer); ws.close(); reject(e); } }
    let started; const ready = new Promise(resolve => { started = resolve; });
    ws.onopen = async () => { try { ws.send(JSON.stringify({ message: 'StartRecognition', audio_format: { type: 'raw', encoding: 'pcm_s16le', sample_rate: 16000 }, transcription_config: { ...config.speechmatics.variants[variant], enable_partials: true } })); await ready; let seq = 0; await pace(buffer, c => { ws.send(c); seq++; }); ended = performance.now(); ws.send(JSON.stringify({ message: 'EndOfStream', last_seq_no: seq })); } catch (e) { fail(e); } };
    ws.onmessage = e => { try { const m = JSON.parse(e.data); raw.push(m); if (m.message === 'RecognitionStarted') started(); if (m.message === 'Error') return fail(new Error(JSON.stringify(m))); if (m.message === 'AddPartialTranscript' || m.message === 'AddTranscript') { const t = m.results?.flatMap(r => r.alternatives?.map(a => a.content) || []).join('') || ''; if (t) { if (firstPartialMs === null) firstPartialMs = performance.now() - start; onPartial(t); if (m.message === 'AddTranscript') text += t; } } if (m.message === 'EndOfTranscript' && !done) { done = true; clearTimeout(timer); ws.close(); resolve({ text, firstPartialMs, finalAfterEndMs: performance.now() - (ended || start), raw }); } } catch (err) { fail(err); } };
    ws.onerror = () => fail(new Error('Speechmatics WebSocket 错误')); ws.onclose = () => { if (!done) fail(new Error('Speechmatics WS 提前关闭')); };
  });
}
