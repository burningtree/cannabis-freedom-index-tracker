export const RESTARTABLE = new Set(["budget_wait", "failed", "insufficient_evidence", "source_unavailable", "needs_review"]);
const localHost = () => ["localhost", "127.0.0.1", "[::1]", "::1"].includes(location.hostname);
export function restartControl(submission, { api, enabled, onRestart }) {
  if (!enabled || !localHost() || !RESTARTABLE.has(submission.status)) return null;
  const box = document.createElement("div"); box.className = "sx-restart";
  const message = document.createElement("span"); message.setAttribute("role", "status");
  const buttons = [];
  // Retry resumes after the saved AI research (checks, build and commit again, no new AI run);
  // Restart starts over from opening the source
  const control = (label, busy, resume, primary) => {
    const button = document.createElement("button"); button.type = "button"; button.className = primary ? "sx-btn sx-btn-primary" : "sx-btn"; button.textContent = label;
    button.title = resume ? "Check, build and commit the saved AI research again, without a new AI run" : "Start again from opening the source, with a new AI run";
    button.addEventListener("click", async () => {
      for (const b of buttons) b.disabled = true;
      button.textContent = busy; message.textContent = "";
      try {
        const response = await fetch(`${api}/v1/submissions/${encodeURIComponent(submission.id)}/restart`, { method: "POST", credentials: "omit", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ resume }), signal: AbortSignal.timeout(15000) });
        const data = await response.json();
        if (!response.ok) throw new Error(data.error || "Restart failed.");
        button.textContent = "Queued again"; onRestart?.();
      } catch (error) { message.textContent = error.message; button.textContent = label; for (const b of buttons) b.disabled = false; }
    });
    buttons.push(button);
    return button;
  };
  if (submission.resumable) box.append(control("Retry from last step", "Retrying…", true, true));
  box.append(control(submission.resumable ? "Restart from scratch" : "Restart submission", "Restarting…", false, false), message);
  return box;
}
