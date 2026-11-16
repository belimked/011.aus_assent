import fs from 'node:fs';
const source = fs.readFileSync('mobile/src/data/seed.ts','utf8');
const start = source.indexOf('JSON.parse(JSON.stringify(') + 'JSON.parse(JSON.stringify('.length;
const end = source.lastIndexOf(')) as AppState');
if (start < 0 || end < 0) throw Error('seed format changed');
const seed = JSON.parse(source.slice(start,end));
const data = JSON.parse(fs.readFileSync('mobile/src/data/sunnybank.json','utf8'));
const parcels = new Map(data.parcels.map(p => [p.lp,p]));
const groups = new Map();
for (const a of data.addresses) { const key = `${a[1]}|${a[2]}|${a[3]}|${a[4]}`; if (!groups.has(key)) groups.set(key,[]); groups.get(key).push(a); }
const decode = p => ({lng:data.origin[0]+p[0]/1e6,lat:data.origin[1]+p[1]/1e6});
const properties = [...groups.values()].map(g => { const a=g[0], parcel=parcels.get(a[4]); return {id:`${a[1]}-${a[2]}-${a[4]}`,unit:g.length>1?'':a[0],no:a[1],street:a[2],suburb:a[3],lotplan:a[4],...decode([a[5],a[6]]),area:parcel?.area,geometry:parcel?.r?.map(decode),units:g.length,ownerIds:[],flags:{}}; });
fs.mkdirSync('server/seed',{recursive:true});
fs.writeFileSync('server/seed/demo.json',JSON.stringify({contacts:seed.contacts,todos:seed.todos,properties:[...seed.properties,...properties]},null,2)+'\n');
console.log(`exported ${seed.contacts.length} contacts, ${seed.todos.length} todos, ${seed.properties.length+properties.length} properties`);
