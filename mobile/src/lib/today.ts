// Calendar dates are local to the device; toISOString() would convert to UTC.
export function today(date = new Date()): string {
  return `${date.getFullYear()}-${String(date.getMonth()+1).padStart(2,'0')}-${String(date.getDate()).padStart(2,'0')}`;
}

export function nowMinutes(date = new Date()): number {
  return date.getHours()*60+date.getMinutes();
}

export function addLocalDays(date: Date, days: number): string {
  const next = new Date(date);
  next.setDate(next.getDate()+days);
  return today(next);
}
