import {MAP_HTML} from './mapPage';

test('Pricefinder parcel markers are interactive and post selection instead of swallowing clicks',()=>{
 expect(MAP_HTML).toContain("mk.on('click',e=>select(p.id,e))");
 expect(MAP_HTML).toContain("mk.on('click',e=>select(p.id,e));");
 expect(MAP_HTML).toContain("post({t:'sel',id})");
 expect(MAP_HTML).toContain("post({t:'tap',lat:e.latlng.lat,lng:e.latlng.lng})");
});
test('drawing shows open corners, dashed closing edge, filled finished polygon and radius',()=>{
 expect(MAP_HTML).toContain("L.polyline([d.points[d.points.length-1],d.points[0]]");
 expect(MAP_HTML).toContain("dashArray:'6 6'");
 expect(MAP_HTML).toContain('L.polygon(d.points');
 expect(MAP_HTML).toContain('L.circle(d.radius.center');
});
