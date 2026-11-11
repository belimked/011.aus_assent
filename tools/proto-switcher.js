// 原型版本切换浮标。由 build-prototype-site.mjs 拼接在版本数据之后，注入每个版本页。
// URL 加 ?nov 可隐藏（截图/演示用），点面板里的“隐藏”也可本次会话隐藏。
;(function () {
  var M = window.__PROTO_VERSIONS__
  var me = document.currentScript
  if (!M || !me) return
  if (/[?&]nov\b/.test(location.search) || sessionStorage.getItem('protoVerHidden')) return
  var cur = me.getAttribute('data-ver')
  var root = me.getAttribute('data-root') || './'
  var sameAs = M.latest.sameAs
  // 当前标签恰好等于最新时，也视为“最新”
  var curInfo = cur === 'latest' ? M.latest : M.versions.filter(function (v) { return v.id === cur })[0]
  if (!curInfo) return
  var isOld = cur !== 'latest' && cur !== sameAs

  var css = document.createElement('style')
  css.textContent =
    '.pv-tab{position:fixed;right:0;top:42%;z-index:2147483000;font:600 12px/1 system-ui,sans-serif;padding:8px 8px 8px 10px;' +
    'border-radius:8px 0 0 8px;border:0;color:#fff;background:rgba(19,35,63,.78);cursor:pointer;-webkit-tap-highlight-color:transparent}' +
    '.pv-tab.old{background:rgba(196,96,0,.92)}' +
    '.pv-mask{position:fixed;inset:0;z-index:2147483001;background:rgba(19,35,63,.45);display:flex;align-items:center;justify-content:center;padding:16px}' +
    '.pv-box{background:#fff;color:#13233f;border-radius:12px;width:100%;max-width:420px;max-height:80vh;display:flex;flex-direction:column;font:14px/1.45 system-ui,sans-serif;overflow:hidden}' +
    '.pv-hd{display:flex;align-items:center;justify-content:space-between;padding:14px 16px;border-bottom:1px solid #e3e6eb;font-weight:600;font-size:15px}' +
    '.pv-hd button{border:0;background:none;color:#667;font:inherit;font-size:13px;cursor:pointer;padding:4px 6px}' +
    '.pv-list{overflow:auto;padding:6px}' +
    '.pv-it{display:block;padding:10px 12px;border-radius:8px;color:inherit;text-decoration:none}' +
    '.pv-it:hover{background:#f2f4f7}.pv-it.on{background:#e8eef9}' +
    '.pv-it b{font-size:15px;margin-right:8px}.pv-it i{font-style:normal;color:#667;font-size:12px}' +
    '.pv-it div{font-size:13px;color:#334;margin-top:2px}' +
    '.pv-ft{padding:10px 16px;border-top:1px solid #e3e6eb;font-size:13px}.pv-ft a{color:#2a5bd7}'
  document.head.appendChild(css)

  function esc(s) { return String(s).replace(/[&<>"]/g, function (c) { return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c] }) }

  var tab = document.createElement('button')
  tab.className = 'pv-tab' + (isOld ? ' old' : '')
  tab.textContent = curInfo.label
  tab.title = '切换原型版本'
  document.body.appendChild(tab)

  function item(v, href, on, tag) {
    return '<a class="pv-it' + (on ? ' on' : '') + '" href="' + href + '"><b>' + esc(v.label) + '</b><i>' +
      esc(v.date) + (tag ? ' · ' + tag : '') + (on ? ' · 当前' : '') + '</i><div>' + esc(v.note) + '</div></a>'
  }
  function open() {
    var h = ''
    if (!sameAs) h += item(M.latest, root, cur === 'latest', '未发版')
    M.versions.forEach(function (v, i) {
      var latest = v.id === sameAs
      h += item(v, latest ? root : root + v.path, cur === v.id || (latest && cur === 'latest'), latest ? '最新' : '')
    })
    var mask = document.createElement('div')
    mask.className = 'pv-mask'
    mask.innerHTML = '<div class="pv-box"><div class="pv-hd">原型版本<span><button data-a="hide">隐藏浮标</button><button data-a="x">关闭</button></span></div>' +
      '<div class="pv-list">' + h + '</div><div class="pv-ft"><a href="' + root + 'versions/">版本列表页</a></div></div>'
    mask.addEventListener('click', function (e) {
      var a = e.target.getAttribute && e.target.getAttribute('data-a')
      if (e.target === mask || a === 'x') mask.remove()
      if (a === 'hide') { sessionStorage.setItem('protoVerHidden', '1'); mask.remove(); tab.remove() }
    })
    document.body.appendChild(mask)
  }
  tab.addEventListener('click', function (e) { e.stopPropagation(); open() })
})()
