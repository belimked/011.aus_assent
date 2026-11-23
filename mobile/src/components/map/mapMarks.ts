import type {Property} from '../../data/types';
import type {MapMark} from './mapPage';

export function markerKind(p:Property,listed:boolean):MapMark['kind']{
 if(p.flags?.dnc)return 'dnc';
 if(p.pinned)return 'pin';
 if(listed)return 'listed';
 if(p.otherListing)return 'other';
 if(p.pending)return 'pending';
 return 'dot';
}
