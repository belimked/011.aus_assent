/**
 * Rex rejects a new contact with neither first nor last name (server/rexworker.go: "contact name required").
 * Every phone entry point that creates a contact uses these, so the rule and the wording live in one place.
 * Existing contacts (matched or picked) are never checked here.
 */
export const CONTACT_NAME_REQUIRED='contact name required';
export const NAME_REQUIRED_TEXT=['Enter a first or last name before saving a new contact','保存新联系人前请填写名字或姓氏'] as const;
export const nameRequiredMessage=(l2:(english:string,chinese:string)=>string)=>l2(NAME_REQUIRED_TEXT[0],NAME_REQUIRED_TEXT[1]);
export const hasContactName=(f:{first?:string;last?:string})=>!!(f.first?.trim()||f.last?.trim());
/** Throws CONTACT_NAME_REQUIRED when a contact about to be created has no name. */
export function requireNewContactName(f:{first?:string;last?:string}){if(!hasContactName(f))throw new Error(CONTACT_NAME_REQUIRED);}
export const isNameRequiredError=(e:unknown)=>e instanceof Error&&e.message===CONTACT_NAME_REQUIRED;
