// Queensland Spatial LandParcelPropertyFramework, layers 0 and 4. CC BY 4.0.
// Usage: node tools/download-qld-parcels.mjs .tmp/qld-raw [SUBURB ...]
// With no suburb arguments, download all supported suburbs.
import {mkdir,writeFile} from 'node:fs/promises';
import {join} from 'node:path';
const dir=process.argv[2];
if(!dir)throw Error('usage: node tools/download-qld-parcels.mjs <raw-dir>');
await mkdir(dir,{recursive:true});
const base='https://spatial-gis.information.qld.gov.au/arcgis/rest/services/PlanningCadastre/LandParcelPropertyFramework/MapServer';
const supported=['SUNNYBANK','SUNNYBANK HILLS','CALAMVALE','PARKINSON','MACGREGOR','ROBERTSON','RUNCORN','KURABY','EIGHT MILE PLAINS'];
const suburbs=process.argv.slice(3).length?process.argv.slice(3).map(s=>s.toUpperCase()):supported;
if(suburbs.some(s=>!supported.includes(s)))throw Error(`suburbs must be one of: ${supported.join(', ')}`);
for(const [prefix,layer] of [['addr',0],['parcel',4]]){
  for(const suburb of suburbs){
    const where=`locality = '${(suburb==='MACGREGOR'&&layer===4?'MacGregor':suburb.toLowerCase().replace(/\b\w/g,c=>c.toUpperCase()))}'`+(layer===4?" AND parcel_typ = 'Lot Type Parcel'":'');
    const tag=suburb.toLowerCase().replaceAll(' ','-');
    for(let offset=0;;offset+=2000){
      const q=new URLSearchParams({where,outFields:'*',outSR:'4326',f:'geojson',resultOffset:String(offset),resultRecordCount:'2000'});
      const res=await fetch(`${base}/${layer}/query?${q}`);
      if(!res.ok)throw Error(`${suburb} ${layer}: HTTP ${res.status}`);
      const data=await res.json();
      if(!Array.isArray(data.features))throw Error(JSON.stringify(data).slice(0,400));
      await writeFile(join(dir,`${prefix}_${tag}_${offset}.json`),JSON.stringify(data));
      console.log(prefix,suburb,offset,data.features.length);
      if(data.features.length<2000)break;
    }
  }
}
