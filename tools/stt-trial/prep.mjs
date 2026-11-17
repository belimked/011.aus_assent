import { readFile, readdir, mkdir, writeFile } from 'node:fs/promises';
import { spawn } from 'node:child_process';
import { join } from 'node:path';
const root = new URL('./', import.meta.url), audio = new URL('./audio/', root), wav = new URL('./audio/.wav/', root);
const manifest = JSON.parse(await readFile(new URL('./manifest.json', root))); await mkdir(wav, { recursive:true });
const files = await readdir(audio); const missing = [], durations = {};
function exec(command, args) { return new Promise((resolve, reject) => { const child = spawn(command,args); let out='',err=''; child.stdout.on('data',d => out+=d); child.stderr.on('data',d => err+=d); child.on('error',reject); child.on('close',code => code ? reject(new Error(`${command} (${code}): ${err.slice(-600)}`)) : resolve(out)); }); }
for (const m of manifest) { const found = files.find(f => f.match(new RegExp(`^${m.id}\\.(m4a|wav|mp3|aac)$`, 'i'))); if (!found) { missing.push(m.id); continue; } const src = join(audio.pathname, found), dst = join(wav.pathname,`${m.id}.wav`); await exec('/usr/local/bin/ffmpeg',['-y','-v','error','-i',src,'-ar','16000','-ac','1','-c:a','pcm_s16le',dst]); durations[m.id] = Number(await exec('/usr/local/bin/ffprobe',['-v','error','-show_entries','format=duration','-of','default=noprint_wrappers=1:nokey=1',dst])); console.log(`${m.id}: ${durations[m.id].toFixed(1)}s`); }
await writeFile(new URL('./audio/.wav/durations.json',root),JSON.stringify(durations,null,2)); console.log(`缺失 ${missing.length}: ${missing.join(', ') || '无'}`);
