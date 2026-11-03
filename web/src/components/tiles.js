import L from 'leaflet'

// 与 wasteid（blk_wasteid/web-prototype）一致：OpenStreetMap 标准底图
export const osmLayer = () =>
  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', { maxZoom: 19, attribution: '© OpenStreetMap' })

// 驾驶舱深色大屏：CARTO Dark Matter（OSM 数据）
export const darkLayer = () =>
  L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
    maxZoom: 19, subdomains: 'abcd', attribution: '© OpenStreetMap © CARTO',
  })
