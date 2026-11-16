import type {AppState,Property} from '../data/types';
import type {Property as Parcel} from '../data/properties';
export function ensureParcelRecord(s:AppState,parcel:Parcel):Property{const existing=s.properties.find(p=>p.parcelId===parcel.id);if(existing)return existing;const p:Property={id:'parcel-'+parcel.id,parcelId:parcel.id,no:parcel.number,street:parcel.street,suburb:parcel.suburb,ownerIds:[],pfOwner:'',lastSale:'',flags:{},ll:[parcel.lat,parcel.lng],lot:parcel.lotplan,land:parcel.area};s.properties.push(p);return p}
