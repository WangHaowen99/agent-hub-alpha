// A12: snapshots carry the service's state revision, which only grows. A
// snapshot older than the one on screen (a late reply, another tab, a poll
// that crossed a reconnect) is dropped, so the view never goes back.
import type { Graph } from "./types";

export function newer(shown: Graph | undefined, next: Graph): boolean {
  if (!shown || shown.rev === undefined || next.rev === undefined) return true;
  return next.rev >= shown.rev;
}
