import {LEAFLET_CSS,LEAFLET_JS} from './leafletAsset';

/** One marker as drawn in the WebView. kind follows the prototype legend. */
export type MapMark={id:string;ll:[number,number];no:string;color:string;kind:'dot'|'pending'|'listed'|'other'|'pin'|'dnc'};
export type MapState={sat:boolean;selected?:string;center:[number,number];centerKey:string;locate:number;zoom:number;marks:MapMark[];bg:[number,number][];parcels:[number,number][][];me:[number,number]|null;labels:boolean};
export type MapMsg={t:'ready'}|{t:'move';s:number;n:number;w:number;e:number;z:number}|{t:'sel';id:string}|{t:'tap';lat:number;lng:number};

// Runs inside the WebView. Mirrors the prototype's Leaflet map (prototype/index.html propMarker/drawMap).
const page=`
const post=m=>window.ReactNativeWebView.postMessage(JSON.stringify(m));
const map=L.map('m',{zoomControl:false,attributionControl:true,preferCanvas:true,tap:false});
map.attributionControl.setPrefix(false);
const osm=L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png',{maxZoom:19,attribution:'© OpenStreetMap'});
const sat=L.tileLayer('https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}',{maxZoom:19,attribution:'© Esri'});
const parcels=L.layerGroup().addTo(map),bg=L.layerGroup().addTo(map),marks=L.layerGroup().addTo(map),me=L.layerGroup().addTo(map);
const PATH={ban:'<circle cx="12" cy="12" r="9"/><path d="M5.6 5.6l12.8 12.8" fill="none"/>',star:'<path d="M12 2.8l2.8 5.8 6.3.9-4.6 4.4 1.1 6.3L12 17.2l-5.6 3 1.1-6.3-4.6-4.4 6.3-.9z"/>'};
const icon=(html,size)=>L.divIcon({className:'',html,iconSize:[size,size],iconAnchor:[size/2,size/2]});
let cur=null,lastKey=null,lastLocate=null,quiet=false;
function setTiles(isSat){const want=isSat?sat:osm,other=isSat?osm:sat;if(map.hasLayer(other))map.removeLayer(other);if(!map.hasLayer(want))want.addTo(map);}
function draw(s){
  parcels.clearLayers();bg.clearLayers();marks.clearLayers();me.clearLayers();
  s.parcels.forEach(r=>L.polygon(r,{color:'#0b5d8a',weight:1,opacity:.45,fillColor:'#0b5d8a',fillOpacity:.05,interactive:false}).addTo(parcels));
  s.bg.forEach(ll=>L.circleMarker(ll,{radius:4,color:'#fff',weight:1,fillColor:'#8A94A6',fillOpacity:.9,interactive:false}).addTo(bg));
  s.marks.forEach(p=>{const at=p.id===s.selected;let mk;
    if(p.kind==='dnc')mk=L.marker(p.ll,{icon:icon('<div class="mk"><svg viewBox="0 0 24 24" style="fill:#D7263D">'+PATH.ban+'</svg></div>',22)});
    else if(p.kind==='pin')mk=L.marker(p.ll,{icon:icon('<div class="mk"><svg viewBox="0 0 24 24" style="fill:#F5A300">'+PATH.star+'</svg></div>',22)});
    else if(p.kind==='listed')mk=L.marker(p.ll,{icon:icon('<div class="pmark"><i></i></div>',22)});
    else if(p.kind==='other')mk=L.marker(p.ll,{icon:icon('<div class="pmark other"></div>',22)});
    else mk=L.circleMarker(p.ll,{radius:at?10:7,color:p.kind==='pending'?'#8A94A6':at?'#13233F':'#fff',weight:at?3:2,fillColor:p.kind==='pending'?'#fff':p.color,fillOpacity:1,dashArray:p.kind==='pending'?'2 2':null});
    if(at&&mk instanceof L.Marker)L.circleMarker(p.ll,{radius:15,color:'#13233F',weight:3,fill:false,interactive:false}).addTo(marks);
    mk.on('click',e=>{L.DomEvent.stopPropagation(e);post({t:'sel',id:p.id});});
    if(s.labels||at)mk.bindTooltip(p.no,{permanent:true,direction:'top',offset:[0,-10],className:'lbl'});
    mk.addTo(marks);});
  if(s.me)L.marker(s.me,{interactive:false,zIndexOffset:1000,icon:icon('<div class="me2"><svg viewBox="0 0 24 24"><circle cx="13" cy="4.5" r="2"/><path d="M13 8l-3 2.5-1 3.5M13 8l1 5 3 2M14 13l-1.5 4-3 3.5M14 13l2.5 3.5.5 4" fill="none"/></svg></div>',36)}).addTo(me);
}
function report(){if(quiet)return;const b=map.getBounds();post({t:'move',s:b.getSouth(),n:b.getNorth(),w:b.getWest(),e:b.getEast(),z:map.getZoom()});}
window.axUpdate=function(s){cur=s;setTiles(s.sat);
  // Closing a selection (key falls back to init / me#n with the same locate count) keeps the current view.
  const wasSel=lastKey!==null&&lastKey!=='init'&&!lastKey.startsWith('me');
  const keep=wasSel&&(s.centerKey==='init'||s.centerKey.startsWith('me#'))&&s.locate===lastLocate;lastLocate=s.locate;
  if(s.centerKey!==lastKey){lastKey=s.centerKey;if(!keep){quiet=true;map.setView(s.center,s.zoom,{animate:false});quiet=false;setTimeout(report,0);}}
  document.body.classList.toggle('z18',map.getZoom()>=18);draw(s);};
map.on('moveend',()=>{document.body.classList.toggle('z18',map.getZoom()>=18);report();});
map.on('click',e=>post({t:'tap',lat:e.latlng.lat,lng:e.latlng.lng}));
post({t:'ready'});
`;

const css=`html,body,#m{margin:0;height:100%;width:100%;background:#E9EEF3;-webkit-tap-highlight-color:transparent}
.leaflet-control-attribution{font-size:9px}
.lbl{background:transparent;border:0;box-shadow:none;padding:0;font:600 11px -apple-system,sans-serif;color:#13233F;text-shadow:0 0 3px #fff,0 0 3px #fff;display:none}.lbl:before{display:none}.z18 .lbl{display:block}
.pmark{width:22px;height:22px;box-sizing:border-box;border-radius:50%;background:#13233F;border:2px solid #fff;display:grid;place-items:center;box-shadow:0 1px 3px rgba(0,0,0,.3)}.pmark i{width:7px;height:7px;border-radius:50%;background:#E8622C}
.pmark.other{background:#FF9F1A}
.mk{display:grid;place-items:center;width:22px;height:22px;filter:drop-shadow(0 1px 2px rgba(0,0,0,.35))}.mk svg{width:20px;height:20px;stroke:#fff;stroke-width:1.5;stroke-linejoin:round}
.me2{width:36px;height:36px;box-sizing:border-box;border-radius:50%;background:#13233F;border:2px solid #fff;display:grid;place-items:center;box-shadow:0 0 0 8px rgba(43,149,245,.28),0 1px 4px rgba(0,0,0,.4)}
.me2 svg{width:20px;height:20px;fill:#fff;stroke:#fff;stroke-width:1.8;stroke-linecap:round;stroke-linejoin:round}`;

export const MAP_HTML=`<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1,maximum-scale=1,user-scalable=no"><style>${LEAFLET_CSS}\n${css}</style></head><body><div id="m"></div><script>${LEAFLET_JS}</script><script>${page}</script></body></html>`;
