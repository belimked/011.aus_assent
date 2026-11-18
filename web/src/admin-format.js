import { locale, t } from "./i18n";

// Same date presentation as the prototype, without importing its simulated store.
const dateLocale = () =>
  ({ "zh-CN": "zh-CN", "zh-HK": "zh-HK", en: "en-AU" })[locale.value];
export function fmtDate(value) {
  if (!value) return "—";
  const date = new Date(value);
  return Number.isNaN(date.valueOf())
    ? "—"
    : date.toLocaleDateString(dateLocale(), {
        year: "numeric",
        month: "short",
        day: "numeric",
      });
}
export function fmtDateTime(value) {
  if (!value || Number.isNaN(new Date(value).valueOf())) return "—";
  return `${fmtDate(value)} ${new Date(value).toLocaleTimeString(dateLocale(), { hour: "2-digit", minute: "2-digit" })}`;
}
export function relTime(value) {
  if (!value) return t("从未");
  const date = new Date(value);
  if (Number.isNaN(date.valueOf())) return "—";
  const seconds = (Date.now() - date.valueOf()) / 1000;
  if (seconds < 60) return t("刚刚");
  if (seconds < 3600) return t("{n} 分钟前", { n: Math.floor(seconds / 60) });
  if (seconds < 86400)
    return t("{n} 小时前", { n: Math.floor(seconds / 3600) });
  if (seconds < 86400 * 30)
    return t("{n} 天前", { n: Math.floor(seconds / 86400) });
  return fmtDate(value);
}
export function money(value) {
  if (value == null) return "—";
  return new Intl.NumberFormat(dateLocale(), {
    style: "currency",
    currency: "AUD",
    maximumFractionDigits: 0,
  }).format(value);
}
export const roleLabel = (role) => t(role === "admin" ? "管理员" : "经纪人");
export const approvalKind = (kind) =>
  t({ reassign: "重新分配", merge: "合并记录" }[kind] || kind);
export const parcelStatus = (parcel) =>
  parcel.hasOwner || parcel.ownerNames
    ? "owned"
    : parcel.registered
      ? "registered"
      : "unregistered";
