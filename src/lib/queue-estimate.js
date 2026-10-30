// Approximate clearance of the existing backlog at the last hour's commit rate.
// Include active jobs and budget-paused cases; new arrivals are not predicted.
export function queueEstimate(summary, processing) {
  if (!summary || ![summary.queued, summary.running, summary.paused ?? 0].every(n => Number.isFinite(n) && n >= 0)) return "—";
  const remaining = summary.queued + summary.running + (summary.paused ?? 0);
  if (!remaining) return "Queue clear";
  if (processing?.enabled === false) return "Worker paused";
  if (summary.paused > 0) return "Budget paused";
  if (!Number.isFinite(summary.committed_last_hour) || summary.committed_last_hour <= 0) return "Waiting for commits";
  const minutes = Math.ceil(remaining * 60 / summary.committed_last_hour);
  if (minutes < 60) return `~${minutes} min`;
  if (minutes < 1440) return `~${Math.floor(minutes / 60)} h${minutes % 60 ? ` ${minutes % 60} min` : ""}`;
  const hours = Math.ceil(minutes / 60);
  return `~${Math.floor(hours / 24)} d${hours % 24 ? ` ${hours % 24} h` : ""}`;
}
