import {circlePts} from '../../domain/circle';

// A name needs at least one letter or CJK character; digits, emoji (e.g. keycap 8️⃣) and symbols alone are rejected.
export function validAreaName(value:string){const name=value.trim();return name.toLowerCase()!==name.toUpperCase()||/[\u3400-\u9FFF\uF900-\uFAFF\u3040-\u30FF\uAC00-\uD7AF]/.test(name)}
export function areaPoints(corners:number[][],center:[number,number]|null,radius:number){return center?circlePts(center,radius):corners}
export function canFinish(corners:number[][],center:[number,number]|null){return center!==null||corners.length>=3}
