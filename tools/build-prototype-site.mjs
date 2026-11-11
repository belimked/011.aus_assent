#!/usr/bin/env node
// 构建带版本切换的原型站点（部署到 Cloudflare Pages: aus-assent-mobile）
//
//   /                 当前 main 的 prototype/（最新，可能含未发版改动）
//   /v/proto-vN/      git 标签 proto-vN 时的 prototype/ 快照
//   /versions/        版本列表页
//   /_versions/switcher.js  页面右侧的版本切换浮标（注入到每个 index.html）
//
// 发版：node tools/proto-release.mjs "本版说明"   （= git tag -a proto-vN + push）
// 用法：node tools/build-prototype-site.mjs [输出目录，默认 .tmp/proto-site]
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'

const git = (args, enc = 'utf8') =>
  execFileSync('git', args, { cwd: ROOT, encoding: enc, maxBuffer: 1 << 30 })
const ROOT = execFileSync('git', ['rev-parse', '--show-toplevel'], { encoding: 'utf8' }).trim()
const OUT = path.resolve(process.argv[2] || path.join(ROOT, '.tmp/proto-site'))
const SRC = 'prototype'

fs.rmSync(OUT, { recursive: true, force: true })
fs.mkdirSync(OUT, { recursive: true })

// ---- 1. 收集版本（标签 proto-vN，按 N 倒序）----
const tags = git(['tag', '-l', 'proto-v*'])
  .split('\n').map(s => s.trim()).filter(t => /^proto-v\d+$/.test(t))
  .sort((a, b) => +b.slice(7) - +a.slice(7))

const versions = tags.map(tag => {
  const commit = git(['rev-list', '-n', '1', tag]).trim()
  const date = git(['log', '-1', '--format=%cs', commit]).trim()
  // 注释标签用标签说明，轻量标签退回提交标题
  let note = git(['tag', '-l', '--format=%(contents:subject)', tag]).trim()
  if (!note) note = git(['log', '-1', '--format=%s', commit]).trim()
  return { id: tag, label: 'v' + tag.slice(7), date, commit: commit.slice(0, 7), note, path: `v/${tag}/` }
})

const headCommit = git(['rev-parse', 'HEAD']).trim()
const headTreeOf = rev => git(['rev-parse', `${rev}:${SRC}`]).trim()
const headTree = headTreeOf('HEAD')
// 工作区 prototype/ 和最新标签完全一致时，“最新”就等于该版本
const latestSame = versions.length && headTreeOf(versions[0].id) === headTree && !dirty()
function dirty() {
  return git(['status', '--porcelain', '--', SRC]).trim() !== ''
}
const manifest = {
  builtAt: new Date().toISOString(),
  latest: {
    id: 'latest', label: latestSame ? versions[0].label : '最新',
    date: git(['log', '-1', '--format=%cs', 'HEAD']).trim(),
    commit: headCommit.slice(0, 7),
    note: latestSame ? versions[0].note : 'main 分支当前内容（尚未发版）',
    sameAs: latestSame ? versions[0].id : null, path: '',
  },
  versions,
}

// ---- 2. 输出文件 ----
// 最新：直接拷贝工作区 prototype/
fs.cpSync(path.join(ROOT, SRC), OUT, { recursive: true })
// 历史版本：从 git 标签取出 prototype/ 下全部文件
for (const v of versions) {
  const dir = path.join(OUT, v.path)
  const files = git(['ls-tree', '-r', '--name-only', v.id, '--', SRC + '/']).split('\n').filter(Boolean)
  for (const f of files) {
    const dest = path.join(dir, path.relative(SRC, f))
    fs.mkdirSync(path.dirname(dest), { recursive: true })
    fs.writeFileSync(dest, git(['show', `${v.id}:${f}`], 'buffer'))
  }
}

// 切换器脚本（版本数据内联，file:// 打开也能用）
fs.mkdirSync(path.join(OUT, '_versions'), { recursive: true })
fs.writeFileSync(path.join(OUT, '_versions/versions.json'), JSON.stringify(manifest, null, 2))
fs.writeFileSync(path.join(OUT, '_versions/switcher.js'),
  `window.__PROTO_VERSIONS__=${JSON.stringify(manifest)};\n` +
  fs.readFileSync(path.join(ROOT, 'tools/proto-switcher.js'), 'utf8'))

// 注入到每个版本的 index.html
const inject = (file, id, rel) => {
  if (!fs.existsSync(file)) return
  let html = fs.readFileSync(file, 'utf8')
  const tag = `<script src="${rel}_versions/switcher.js" data-ver="${id}" data-root="${rel}" defer></script>`
  const i = html.lastIndexOf('</body>')
  html = i >= 0 ? html.slice(0, i) + tag + '\n' + html.slice(i) : html + '\n' + tag + '\n'
  fs.writeFileSync(file, html)
}
inject(path.join(OUT, 'index.html'), 'latest', './')
for (const v of versions) inject(path.join(OUT, v.path, 'index.html'), v.id, '../../')

// 版本列表页
const esc = s => String(s).replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c])
const row = (v, href) => `<a class="row" href="${href}"><b>${esc(v.label)}</b><span class="d">${esc(v.date)} · ${esc(v.commit)}</span><span class="n">${esc(v.note)}</span></a>`
fs.mkdirSync(path.join(OUT, 'versions'), { recursive: true })
fs.writeFileSync(path.join(OUT, 'versions/index.html'), `<!doctype html><html lang="zh"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1"><title>原型版本</title>
<style>body{font:15px/1.5 system-ui,sans-serif;margin:0;background:#f5f6f8;color:#13233f}
main{max-width:640px;margin:0 auto;padding:24px 16px}h1{font-size:20px;margin:0 0 4px}p{color:#667;margin:0 0 16px;font-size:13px}
.row{display:grid;grid-template-columns:auto 1fr;gap:2px 12px;background:#fff;border:1px solid #e3e6eb;border-radius:8px;padding:12px 14px;margin-bottom:8px;color:inherit;text-decoration:none}
.row:hover{border-color:#13233f}.row b{font-size:16px}.d{color:#667;font-size:13px;align-self:center}.n{grid-column:1/-1;font-size:14px}</style></head>
<body><main><h1>原型版本</h1><p>构建于 ${esc(manifest.builtAt.slice(0, 16).replace('T', ' '))} UTC，共 ${versions.length} 个版本</p>
${latestSame ? '' : row({ ...manifest.latest, label: '最新（未发版）' }, '../')}
${versions.map((v, i) => row(i === 0 && latestSame ? { ...v, label: v.label + '（最新）' } : v, i === 0 && latestSame ? '../' : '../' + v.path)).join('\n')}
</main></body></html>`)

console.log(`built ${OUT}: latest(${manifest.latest.label}) + ${versions.length} versions`)
