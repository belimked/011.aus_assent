import type {Listing, Property} from '../data/types';

/** This agency's current listing of a property: Rex current listings (drafts included) map to status 'listed'; sold, leased, withdrawn, archived and unknown Rex states do not. */
export const placeListing=(listings:Listing[],propertyId:string)=>listings.find(l=>l.propertyId===propertyId&&l.status==='listed');
const nameKey=(s:string|undefined)=>(s||'').toLowerCase().replace(/[^a-z0-9]/g,'');
/** Agency currently listing the property per imported market data (server field competitorListing,
 *  derived from market_events in server/competitor_listing.go), unless it is this company. */
export function competitorAgency(p:Property,company?:string):string|undefined{
 const agency=typeof p.competitorListing?.agency==='string'?p.competitorListing.agency.trim():'';
 if(!agency||(nameKey(company)&&nameKey(company)===nameKey(agency)))return undefined;
 return agency;
}
/** Property status line. The company name comes from the server (admin setting company_name / AXLEO_COMPANY_NAME).
 *  This agency's own Rex listing always wins over a competitor listing from market data. */
export function listingStatusText(listing:Listing|undefined,p:Property,company:string|undefined,t:(key:string)=>string){
 if(!listing){const other=competitorAgency(p,company);return other?t('listed_with').replace('{company}',other):p.otherListing?'Listed with competitor':'Not listed'}
 const label=company?t('listed_with').replace('{company}',company):t('listed_with_us');
 return listing.price?`${label} · ${listing.price}`:label;
}
