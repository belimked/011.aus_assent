import type {Property} from '../../data/types';
import type {MapMark} from './mapPage';
import {competitorAgency} from '../../domain/listing';

/** listed: this agency's own Rex listing (status listed), which wins over a competitor listing. */
export function markerKind(p:Property,listed:boolean,company?:string):MapMark['kind']{
 if(p.flags?.dnc)return 'dnc';
 if(p.pinned)return 'pin';
 if(listed)return 'listed';
 if(p.otherListing||competitorAgency(p,company))return 'other';
 if(p.pending)return 'pending';
 return 'dot';
}
