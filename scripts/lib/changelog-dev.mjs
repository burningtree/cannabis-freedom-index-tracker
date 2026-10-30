import { execFile } from "node:child_process";
import { promisify } from "node:util";

const exec = promisify(execFile);

// Git updates are ignored by Vite's normal watcher. Poll only the resolved HEAD,
// which also covers packed refs, linked worktrees, branch switches and empty commits.
export function changelogDev({ interval = 1000 } = {}) {
  let stop;
  return {
    name: "tracker:changelog-dev",
    apply: "serve",
    async configureServer(server) {
      stop?.();
      let previous, busy = false, closed = false;
      const revision = async () => {
        try {
          const { stdout } = await exec("git", ["rev-parse", "--verify", "HEAD"], { cwd: server.config.root, timeout: 5000 });
          return stdout.trim();
        } catch { return null; } // e.g. a new repository before its first commit
      };
      previous = await revision();
      const check = async () => {
        if (busy || closed) return;
        busy = true;
        try {
          const next = await revision();
          if (closed || !next || next === previous) return;
          previous = next;
          // Reports and their indexes are computed once when their module loads.
          // Clearing only getStaticPaths leaves those snapshots (and already open
          // map/list pages) alive. Invalidate each environment and use Vite's full
          // reload so its module runner and every browser load fresh report data.
          for (const environment of Object.values(server.environments)) {
            environment.moduleGraph.invalidateAll();
            environment.hot.send("astro:content-changed", {});
            environment.hot.send({ type: "full-reload", path: "*" });
          }
          // Retain the event for existing dev clients; current pages use full-reload.
          server.ws.send({ type: "custom", event: "tracker:git-changed", data: {} });
        } finally { busy = false; }
      };
      const timer = setInterval(check, interval);
      timer.unref();
      stop = () => { closed = true; clearInterval(timer); };
      server.httpServer?.once("close", stop);
    },
    closeBundle() { stop?.(); },
  };
}
