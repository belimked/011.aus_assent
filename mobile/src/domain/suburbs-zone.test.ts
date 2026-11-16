import {mergeSuburbsZone,monitoredSuburbs} from './suburbs-zone';

test('creates one stable suburbs record without disturbing drawn zones',()=>{
 const drawn={id:'z1',name:'Area',pts:[[1,2],[3,4],[5,6]]};
 const first=mergeSuburbsZone([drawn],'owen@example.com',['Sunnybank','Sunnybank']);
 expect(first).toEqual([drawn,{id:'zone-suburbs-owen@example.com',type:'suburbs',name:'Monitored suburbs',suburbs:['Sunnybank']}]);
 const next=mergeSuburbsZone(first,'owen@example.com',['Runcorn']);
 expect(next).toHaveLength(2);
 expect(monitoredSuburbs(next,[])).toEqual(['Runcorn']);
 expect(mergeSuburbsZone([{id:'legacy',type:'suburbs',name:'old',suburbs:[]},drawn],'other',['Calamvale'])[1].id).toBe('legacy');
});
