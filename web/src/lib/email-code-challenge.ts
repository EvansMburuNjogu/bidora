const referencePattern=/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
export function emailCodeReference(value:string|null):string|null{return value&&referencePattern.test(value)?value:null;}
export function emailCodeQuery(response:Response):string{const ref=emailCodeReference(response.headers.get("X-Bidora-Code-Challenge"));return ref?"?challenge_ref="+encodeURIComponent(ref):"";}
