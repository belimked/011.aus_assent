import {seed, TODAY} from '../data/seed';
import type {AppState} from '../data/types';
function addDays(n:number){const d=new Date(TODAY);d.setDate(d.getDate()+n);return d.toISOString().slice(0,10)}
export function extract(text: string, ctx: string, state: AppState = seed()){
  const r: any = {first:"",last:"",mobile:"",email:"",relation:ctx==="oh"?"buyer":"owner",note:text.trim(),intent:"",timing:"",followup:"",tags:[],req:{},ps:false,address:"",reason:"",unsure:[],type:"",source:"",need:""};
  const m = text.match(/0[45]\d{2}[\s-]?\d{3}[\s-]?\d{3}|0[45]\d{8}/); if(m) r.mobile = m[0].replace(/\D/g,"").replace(/(\d{4})(\d{3})(\d{3})/,"$1 $2 $3");
  const e = text.match(/[\w.+-]+@[\w-]+\.[\w.]+/); if(e) r.email = e[0];
  const notName=/^(St|Street|Rd|Road|Ave|Avenue|Ct|Court|Dr|Drive|Pl|Place|Sunnybank|Runcorn|Calamvale|Domain|Mobile|Lead|Referral|From)$/;
  let n = text.match(/(?:[Oo]wner is|[Ss]poke to|[Ss]poke with|[Tt]alked to|[Nn]ame is|[Tt]his is|with|业主是|叫|和)\s*([A-Z][a-z]+)\s+([A-Z][a-z]+)/) || text.match(/([A-Z][a-z]+)\s+([A-Z][a-z]+),?\s*(?:on |at |mobile |mob |ph )?0[45]\d/) || text.match(/^([A-Z][a-z]+)\s+([A-Z][a-z]+)[\s,]/);
  if(n && (notName.test(n[1])||notName.test(n[2]))) n=null;
  if(n){ r.first=n[1]; r.last=n[2]; } else { const cn=text.match(/(?:业主是|业主叫|名叫|我是|这位是|叫)\s*([一-龥]{2,3})(?=[，,。\s]|$)/); const c = cn?[cn[1]]:text.match(/[一-龥]{2,3}(?=[，,\s]|$)/); if(c){ r.first=c[0]; r.last=""; if(!cn) r.unsure.push("name"); } else r.unsure.push("name"); }
  if(/tenant|租客|renting/i.test(text)){ r.relation="tenant"; r.type="tenant"; }
  if(/landlord|房东/i.test(text)){ r.relation="landlord"; r.type="landlord"; }
  if(/might sell|may sell|thinking of selling|could sell|可能卖|想卖|考虑卖|sell next|apprais|估价/i.test(text)){ r.intent="Seller lead"; r.tags.push("Seller lead"); r.ps=true; r.type="seller"; }
  if(/wants? a|looking for|buyer|想买|bed in/i.test(text) && !r.type) r.type="buyer";
  if(/not interested|不感兴趣|no plans/i.test(text)){ r.intent="Not interested"; r.tags.push("Cold"); }
  if(/\bhot\b|urgent|this month|asap/i.test(text)) r.tags.push("Hot"); else if(r.ps) r.tags.push("Warm");
  const tm = text.match(/next (spring|summer|autumn|winter|year|month)|in (\d+) months?|early (\d{4})|(明年|春天|三月|下个月|年底)/i); if(tm) r.timing = tm[0];
  if(/march|三月/i.test(text)) r.followup = "2027-03-01"; else if(/next year|明年/i.test(text)) r.followup = "2027-01-15"; else if(/next month|下个月/i.test(text)) r.followup = addDays(30); else if(r.ps) r.followup = addDays(14);
  if(/building and pest|building & pest|b&p|验房/i.test(text)) r.req.bp=true;
  if(/form 2|disclosure/i.test(text)) r.req.form2=true;
  if(/private (inspection|viewing)/i.test(text)) r.req.private=true;
  if(/broker|finance|贷款/i.test(text)) r.req.broker=true;
  if(/downsiz|kids (have )?moved out|孩子.*搬/i.test(text)){ r.reason="Downsizing"; r.tags.push("Downsizing"); }
  if(/upsiz|bigger|growing family/i.test(text)){ r.reason="Upsizing"; r.tags.push("Upsizing"); }
  if(/school/i.test(text)){ r.reason="Moving for school"; r.tags.push("Moving for school"); }
  if(/relocat|interstate|overseas/i.test(text)){ r.reason="Relocating"; r.tags.push("Relocating"); }
  if(/investor|投资/i.test(text)){ r.tags.push("Investor"); if(!r.reason) r.reason="Investor"; }
  if(/market update/i.test(text)) r.tags.push("Market update");
  if(/don'?t (contact|call|knock)|stop knocking|别再上门|别来敲门|别联系/i.test(text)) r.tags.push("Do not contact");
  if(/friendly|nice chat|lovely|友好|人挺好|人不错|很热情/i.test(text)) r.tags.push("Friendly");
  if(/no ?one (was )?home|not (at )?home|nobody (was )?(home|in)|没人在家|不在家|没人应门/i.test(text)) r.tags.push("Not at home");
  if(/left (a |my )?card|留了?(名片|卡片|卡)/i.test(text)) r.tags.push("Left card");
  if(/no junk|junk mail|don'?t (send|email).*(junk|stuff|newsletter|email)|乱七八糟|别.*(邮件|广告)|不要.*(广告|邮件)/i.test(text)) r.tags.push("No junk mail");
  if(/don'?t bother|leave (him|her|them) alone|别去烦|别烦|不要打扰|别打扰/i.test(text)) r.tags.push("Do not contact");
  if(/market (update|report)|市场(报告|更新|分析)/i.test(text) && !r.tags.includes("Market update")) r.tags.push("Market update");
  if(/换学校|转学|学区/i.test(text) && !r.tags.includes("Moving for school")){ r.tags.push("Moving for school"); if(!r.reason) r.reason="Moving for school"; }
  if(/投资房|出租房|rental property/i.test(text) && !r.tags.includes("Investor")) r.tags.push("Investor");
  if(/急|尽快|马上/.test(text) && !r.tags.includes("Hot")) r.tags.push("Hot");
  if(r.mobile) r.tags.push("Phone provided"); if(r.email) r.tags.push("Email provided");
  try{ state.myTags.filter(m=>m.by===(state.auth?.name||"Owen Chen")).forEach(m=>{ if(text.toLowerCase().includes(m.n.toLowerCase()) && !r.tags.includes(m.n)) r.tags.push(m.n); }); }catch{}
  const a = text.match(/(\d+[a-z]?\/?\d*\s+[A-Z][a-z]+\s+(?:St|Street|Rd|Road|Ave|Avenue|Ct|Dr)\b[^,.]*)/); if(a) r.address=a[1];
  else { const h = text.match(/(?:own|owns|house|home|房子?)\s+(?:in|at|在)\s+([A-Z][a-z]+)/); if(h) r.address=h[1]; }
  const src = text.match(/from (domain|rea|realestate(?:\.com\.au)?|eplace|referral|phone|walk[- ]?in)/i); if(src) r.source=src[1];
  const nd = text.match(/(wants?|looking for|needs?)\s+(.+?)(?:\.|$)/i); if(nd) r.need=nd[2];
  if(!r.mobile) r.unsure.push("mobile");
  r.tags=[...new Set(r.tags)]; r.ai=[...r.tags];
  return r;
}

export const SAMPLES = {
  search:["who owns 4 Gager St","0433","Mary"],
  dk:["Spoke to Mary Chen at 12 Smith St, she's John's wife. Kids moved out, might sell next year. Mobile 0412 555 019. Send her a market update.",
      "Spoke to Peter Kowalski, 0418 332 760, not interested right now, retired, happy with a market update once a year.",
      "业主是王丽, 手机 0421 338 902, 住了八年, 明年可能卖, 三月再联系。"],
  oh:["John Lee, 0433 221 877, john.lee@example.com, wants the building and pest report and Form 2, owns in Runcorn, might sell next year.",
      "Sarah Lim 0433 908 112 sarah.lim@example.com, first home buyer, pre-approved, looking at 4 bed in Sunnybank, wants a private inspection."],
  enq:["Jenny Lam, 0412 778 201, wants a 3 bed in Runcorn up to 1.1m",
       "Lead from Domain, Alex Park, 0407 118 450, wants an appraisal for 16 Gager St Sunnybank, thinking of selling early 2027."],
  enqsearch:["Buyers looking for 4 bed in Runcorn or Sunnybank, up to 1.2 million","investor","pre-approved"]
};


