// Renders a Markdown file to a standalone, mobile-friendly HTML page.
// Usage: node tools/build-answers-page.mjs <in.md> <marked-package-dir> <out.html>
import { readFileSync, writeFileSync } from 'node:fs'
import { pathToFileURL } from 'node:url'
import { join } from 'node:path'

const [input, markedDir, output] = process.argv.slice(2)
if (!input || !markedDir || !output) {
  console.error('Usage: node tools/build-answers-page.mjs <in.md> <marked-package-dir> <out.html>')
  process.exit(1)
}
const { marked } = await import(pathToFileURL(join(markedDir, 'lib/marked.esm.js')).href)
const md = readFileSync(input, 'utf8')
const title = (md.match(/^#\s+(.+)$/m) || [, 'agentCollect'])[1]
const body = marked.parse(md, { gfm: true, breaks: true })
const built = new Date().toISOString().slice(0, 10)

writeFileSync(output, `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex,nofollow">
<title>${title}</title>
<style>
:root{--b:#0b5d8a}
*{box-sizing:border-box}
body{margin:0;background:#f3f5f8;color:#1f2937;font:15px/1.7 -apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif}
header{background:#0f172a;color:#fff;padding:12px 20px;display:flex;gap:16px;align-items:center;flex-wrap:wrap}
header b{font-size:16px} header a{color:#93c5fd;text-decoration:none;font-size:14px}
main{max-width:920px;margin:20px auto;background:#fff;border-radius:14px;padding:24px 28px;box-shadow:0 1px 3px #0001}
h1{font-size:24px;color:var(--b);margin-top:0} h2{font-size:20px;border-bottom:2px solid #e5e7eb;padding-bottom:6px;margin-top:2em}
h3{font-size:16.5px;color:var(--b);margin-top:1.6em}
table{border-collapse:collapse;width:100%;display:block;overflow-x:auto;font-size:14px;margin:1em 0}
th,td{border:1px solid #e5e7eb;padding:6px 10px;vertical-align:top;text-align:left} th{background:#f1f5f9}
code{background:#f1f5f9;border-radius:4px;padding:1px 5px;font-size:13px;word-break:break-word}
pre{background:#f8fafc;border:1px solid #e5e7eb;border-radius:8px;padding:12px;overflow:auto} pre code{background:none;padding:0}
blockquote{margin:1em 0;padding:8px 14px;border-left:4px solid var(--b);background:#eff6ff;color:#334155;border-radius:0 8px 8px 0}
hr{border:0;border-top:1px solid #e5e7eb;margin:2em 0}
.foot{color:#64748b;font-size:12px;text-align:center;margin:0 0 30px}
@media(max-width:600px){main{margin:0;border-radius:0;padding:18px 16px}}
</style></head><body>
<header><b>agentCollect · 反馈问题回答</b><a href="./">← 返回原型</a></header>
<main>${body}</main>
<p class="foot">临时页面，仅供讨论 · 生成于 ${built}</p>
</body></html>
`)
console.log(`wrote ${output} (${(md.length / 1024).toFixed(1)} KB markdown)`)
