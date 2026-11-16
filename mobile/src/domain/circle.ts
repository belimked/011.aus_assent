// 36 vertices, matching the prototype's metre-to-degree approximation.
export function circlePts(center:[number,number],metres:number):number[][]{const dLat=metres/111320,dLng=metres/(111320*Math.cos(center[0]*Math.PI/180));return Array.from({length:36},(_,i)=>{const a=i*Math.PI/18;return [center[0]+dLat*Math.sin(a),center[1]+dLng*Math.cos(a)]})}
