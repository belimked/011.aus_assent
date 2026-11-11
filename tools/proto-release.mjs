#!/usr/bin/env node
// 发布一个原型版本：给当前 HEAD 打注释标签 proto-vN（N 自动 +1）并推送，CI 会自动部署。
//   node tools/proto-release.mjs "本版说明"            # 打标签并推送
//   node tools/proto-release.mjs "本版说明" --no-push  # 只在本地打标签
import { execFileSync } from 'node:child_process'

const git = (...a) => execFileSync('git', a, { encoding: 'utf8' }).trim()
const args = process.argv.slice(2)
const push = !args.includes('--no-push')
const note = args.filter(a => a !== '--no-push').join(' ').trim()
if (!note) { console.error('用法: node tools/proto-release.mjs "本版说明" [--no-push]'); process.exit(1) }

if (git('status', '--porcelain', '--', 'prototype')) {
  console.error('prototype/ 有未提交的改动，请先 commit 再发版'); process.exit(1)
}
const tags = git('tag', '-l', 'proto-v*').split('\n').filter(t => /^proto-v\d+$/.test(t))
const last = tags.map(t => +t.slice(7)).sort((a, b) => b - a)[0] || 0
if (last) {
  const same = git('rev-parse', `proto-v${last}:prototype`) === git('rev-parse', 'HEAD:prototype')
  if (same) { console.error(`prototype/ 与 proto-v${last} 完全相同，无需发版`); process.exit(1) }
}
const tag = `proto-v${last + 1}`
git('tag', '-a', tag, '-m', note)
console.log(`已创建 ${tag} @ ${git('rev-parse', '--short', 'HEAD')}: ${note}`)
if (push) {
  execFileSync('git', ['push', 'origin', 'HEAD:main', tag], { stdio: 'inherit' })
  console.log('已推送，CI 部署完成后可在页面右侧浮标切换版本')
} else {
  console.log(`推送: git push origin ${tag}`)
}
