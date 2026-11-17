import { readFile } from 'node:fs/promises';
export async function env() { try { for (const line of (await readFile(new URL('./.env', import.meta.url), 'utf8')).split(/\r?\n/)) { const m = line.match(/^\s*([A-Z][A-Z0-9_]*)\s*=\s*(.*?)\s*$/); if (m && process.env[m[1]] === undefined) process.env[m[1]] = m[2].replace(/^(['"])(.*)\1$/, '$2'); } } catch (e) { if (e.code !== 'ENOENT') throw e; } }
export const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
export async function checked(response) { if (!response.ok) { const error = new Error(`HTTP ${response.status}: ${(await response.text()).slice(0, 600)}`); error.status = response.status; throw error; } return response; }
export async function retry(fn) { try { return await fn(); } catch (e) { await pause(e.status === 429 ? 2000 : 300); return fn(); } }
export async function pace(buffer, send) { for (let i = 0; i < buffer.length; i += 3200) { await send(buffer.subarray(i, i + 3200)); await pause(100); } }
export function pcm(wav) { if (wav.toString('ascii', 0, 4) !== 'RIFF' || wav.readUInt32LE(24) !== 16000 || wav.readUInt16LE(22) !== 1 || wav.readUInt16LE(34) !== 16) throw new Error('需要 prep.mjs 生成的 16kHz mono PCM16 WAV'); let i = 12; while (i + 8 <= wav.length) { const size = wav.readUInt32LE(i + 4); if (wav.toString('ascii', i, i + 4) === 'data') return wav.subarray(i + 8, i + 8 + size); i += 8 + size + (size % 2); } throw new Error('WAV data 缺失'); }
export function json(text) { return JSON.parse(text.replace(/^```(?:json)?\s*|\s*```$/g, '')); }
