import {seed,TODAY} from '../data/seed';
import {applyTags,applyPreference,filePropertyNote,commitDK} from './dk';

test('DNC tags propagate to property and consent, visit tags record outcome',()=>{
 const s=seed(),p=s.properties[0],c=s.contacts[0];p.ownerIds=[c.id];
 applyTags(s,c,p,['Do not contact','Left card']);
 expect(c.flags.dnc).toBe(true);expect(p.flags.dnc).toBe(true);
 expect(c.consent).toMatchObject({email:0,sms:0,phone:0,letter:0});
 expect(p.visits.at(-1)).toEqual({d:TODAY,o:'left_card'});
 applyTags(s,c,p,[]);expect(p.flags.dnc).toBe(false);
});
test('quick add DNC flags property and records a visit',()=>{
 const s=seed(),p=s.properties[0];p.ownerIds=[];
 const c=commitDK(s,p.id,{first:'Test',last:'Owner',mobile:'0499000111',email:'',relation:'owner',note:'',reason:'',tags:['Do not contact']},'new');
 expect(p.ownerIds).toContain(c.id);expect(p.flags.dnc).toBe(true);expect(p.visits.at(-1)?.o).toBe('spoke');
});
test('note on property without owner creates and links a contact',()=>{
 const s=seed(),p=s.properties[0];p.ownerIds=[];p.pfOwner='Sam Smith';
 const c=filePropertyNote(s,p.id,'Spoke at door');
 expect(p.ownerIds).toContain(c.id);expect(c.notes[0].t).toBe('Spoke at door');expect(p.visits.at(-1)?.o).toBe('spoke');expect(p.notes).toBeUndefined();
});
test('preferences log each channel, junk tag, and only SMS determines optedOut',()=>{
 const s=seed(),c=s.contacts[0];
 applyPreference(s,c,'email',0,'asked');expect(c.optedOut).toBe(false);
 expect(c.notes[0].t).toContain('Preferences · Email opted out · asked');
 applyPreference(s,c,'junk',0);expect(c.tags).toContain('No junk mail');
 applyPreference(s,c,'junk',1);expect(c.tags).not.toContain('No junk mail');
 applyPreference(s,c,'sms',0);expect(c.optedOut).toBe(true);
 expect(s.audit.filter(a=>a.a==='Preferences')).toHaveLength(4);
});
