// Adapted W02 A12 guard; W04 generation is an unbounded decimal string.
import type { Graph } from './types';
export function newer(shown:Graph|undefined,next:Graph):boolean {
 if(!/^(0|[1-9][0-9]*)$/.test(next.rev??''))return false;
 if(!shown || shown.plan!==next.plan || shown.version!==next.version)return true;
 if(!/^(0|[1-9][0-9]*)$/.test(shown.rev??''))return false;
 return BigInt(next.rev!)>=BigInt(shown.rev!);
}
