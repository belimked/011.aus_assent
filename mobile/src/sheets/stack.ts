export type SheetEntry={kind:string;prev?:SheetEntry;[key:string]:any};

/** Preserve the actual parent sheet objects so nested pickers can return to mounted forms. */
export function sheetStack(sheet:SheetEntry|null):SheetEntry[]{
  const entries:SheetEntry[]=[];
  for(let current=sheet;current;current=current.prev??null)entries.unshift(current);
  return entries;
}
