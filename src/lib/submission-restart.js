export const RESTARTABLE = new Set(["budget_wait", "failed", "insufficient_evidence", "source_unavailable", "needs_review"]);
const localHost = () => ["localhost", "127.0.0.1", "[::1]", "::1"].includes(location.hostname);
export function restartControl(submission, { api, enabled, onRestart }) {
  if (!enabled || !localHost() || !RESTARTABLE.has(submission.status)) return null;
  const box = document.createElement("div"); box.className = "sx-restart";
  const button = document.createElement("button"); button.type = "button"; button.className = "sx-btn"; button.textContent = "Restart submission";
  const message = document.createElement("span"); message.setAttribute("role", "status");
  button.addEventListener("click", async () => {
    button.disabled = true; button.textContent = "Restarting…"; message.textContent = "";
    try {
      const response = await fetch(`${api}/v1/submissions/${encodeURIComponent(submission.id)}/restart`, { method: "POST", credentials: "omit", headers: { "Content-Type": "application/json" }, body: "{}", signal: AbortSignal.timeout(15000) });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || "Restart failed.");
      button.textContent = "Queued again"; onRestart?.();
    } catch (error) { message.textContent = error.message; button.textContent = "Restart submission"; button.disabled = false; }
  });
  box.append(button, message); return box;
}
