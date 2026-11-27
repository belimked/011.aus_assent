/** Spoken words as a search term: trimmed, with sentence-ending punctuation (English and Chinese . 。 ? ？ ! ！ , ，) dropped from the end. */
export function speechQuery(spoken:string):string{return spoken.replace(/[\s.。?？!！,，]+$/u,'').trim()}
