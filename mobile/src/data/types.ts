export type Contact = {id:string;first:string;last:string;mobile:string;email:string;owner:string;roles:string[];tags:string[];flags:{dnc?:boolean};notes:ContactNote[];noteRexSync?:Record<string,{status:string;reason?:string;at?:string}>;[key:string]:any};
/** id: stable key of the one-time Rex Notes::create job. sys: app activity log, not written to Rex. propId: property the note was made about. */
export type ContactNote = {id?:string;d:string;a:string;t:string;at?:string;propId?:string;sys?:string};
export type Property = {id:string;rexId?:string;no:string;street:string;suburb:string;ownerIds:string[];pfOwner:string;lastSale:string;flags:{dnc?:boolean};notes?:{id?:string;d:string;a:string;t:string;who?:string;at?:string}[];noteRexSync?:Record<string,{status:string;reason?:string;at?:string}>;pending?:boolean;otherAgent?:boolean;photos?:{id:string;d:string;uri?:string;uploaded?:boolean}[];market?:{lastSalePrice:number|null;lastSaleDate:string;source:string;updatedAt:string;owners?:string};[key:string]:any};
export type Todo = {id:string;d:string;time?:string;t:string;who:string;by?:string;kind:string;done?:boolean;doneAt?:string;cid?:string;propId?:string;assigneeId?:string;createdBy?:string;version?:number};
export type Sale = {id:string;propertyId:string;cat:string;seller:string;buyer:string;price:number;gci:number;settle:string;when?:string;reminders:{type:string;d:string;time?:string}[]};
export type Listing = {id:string;propertyId:string;agent:string;price:string;status:string;ofi:any[];[key:string]:any};
export type Enquiry = {id:string;source:string;d:string;type:string;status:string;raw:string;parsed:Record<string,string>;bpReport?:boolean;marketingConsent?:boolean};
export type Approval = {id:string;kind:string;t?:string;sub?:string;propertyId?:string;status?:string;primaryId?:string;secondaryId?:string;entity?:string;targetId?:string;[key:string]:any};
export type Capture = {id:string;text:string;rec:string;sync:string;propId:string;status?:string;[key:string]:any};
export type MarketItem = {id:string;kind:string;propId:string;addr:string;meta:string;sub:string;owner:string;agency?:string};
export type Zone = {id:string;name:string;pts?:number[][];type?:'suburbs';suburbs?:string[]};
export type MyTag = {id:string;n:string;by:string;d:string;used?:number;[key:string]:any};
export type AuditEntry = {id?:string;version?:number;d:string;a:string;t:string};
export type SyncItem = {id:string;d:string;what:string;status:string;tries:number;err:string};
export type Account = {id?:string;name:string;email:string;role?:string;company?:string;team:string;rex:string;since?:string;device?:string;expires?:string};
export type AppState = {v:number;auth:Account|null;contacts:Contact[];properties:Property[];todos:Todo[];sales:Sale[];listings:Listing[];ofi_attendees:any[];enquiries:Enquiry[];approvals:Approval[];captures:Capture[];market:MarketItem[];zones:Zone[];myTags:MyTag[];audit:AuditEntry[];syncq:SyncItem[];rexDown:boolean;patch:{suburbs:string[]};partners:Record<string,string[]>;referrals:any[];leads:any[];contracts:any[];queue:number};
