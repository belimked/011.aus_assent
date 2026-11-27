import type {Listing, Property} from '../data/types';

/** This agency's current listing of a property: Rex current listings (drafts included) map to status 'listed'; sold, leased, withdrawn, archived and unknown Rex states do not. */
export const placeListing=(listings:Listing[],propertyId:string)=>listings.find(l=>l.propertyId===propertyId&&l.status==='listed');
/** Property status line. The company name comes from the server (admin setting company_name / AXLEO_COMPANY_NAME). */
export function listingStatusText(listing:Listing|undefined,p:Property,company:string|undefined,t:(key:string)=>string){
 if(!listing)return p.otherListing?'Listed with competitor':'Not listed';
 const label=company?t('listed_with').replace('{company}',company):t('listed_with_us');
 return listing.price?`${label} · ${listing.price}`:label;
}
