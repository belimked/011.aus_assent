export function validTodoDate(date:string):boolean {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) return false;
  const [year,month,day]=date.split('-').map(Number);
  if(year===0)return false;
  const parsed=new Date(0);
  parsed.setFullYear(year,month-1,day);
  return parsed.getFullYear()===year&&parsed.getMonth()===month-1&&parsed.getDate()===day;
}

export function validTodoTime(time:string):boolean {
  return /^([01]\d|2[0-3]):[0-5]\d$/.test(time);
}

export function validTodoSchedule(date:string,time:string):boolean {
  return validTodoDate(date)&&validTodoTime(time);
}
