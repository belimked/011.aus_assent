import {circlePts} from '../../domain/circle';

export function validAreaName(value:string){const name=value.trim();return !!name&&!/^\d+$/.test(name)}
export function areaPoints(corners:number[][],center:[number,number]|null,radius:number){return center?circlePts(center,radius):corners}
export function canFinish(corners:number[][],center:[number,number]|null){return center!==null||corners.length>=3}
