import test from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, mkdirSync, writeFileSync, symlinkSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { spawn, execFileSync } from "node:child_process";
import { once } from "node:events";
import { setTimeout as delay } from "node:timers/promises";
import { pathToFileURL } from "node:url";

// Exercise Astro's real dev route cache: commits change no watched source files.
test("dev changelog refreshes cached props and pagination after a commit", { timeout: 60000 }, async () => {
  const root = mkdtempSync(join(tmpdir(), "tracker-changelog-dev-"));
  const git = (...args) => execFileSync("git", args, { cwd: root, stdio: "pipe" });
  let child, socket, logs = "";
  const until = async (action) => {
    for (let i = 0; i < 150; i++) {
      if (child.exitCode !== null) throw new Error(`Astro exited: ${logs}`);
      try { const value = await action(); if (value) return value; } catch { /* startup/reload in progress */ }
      await delay(100);
    }
    throw new Error(`Timed out waiting for dev changelog: ${logs}`);
  };
  try {
    mkdirSync(join(root, "src/pages/changelog"), { recursive: true });
    symlinkSync(resolve("node_modules"), join(root, "node_modules"), "dir");
    writeFileSync(join(root, "package.json"), '{"type":"module"}');
    const plugin = pathToFileURL(resolve("scripts/lib/changelog-dev.mjs")).href;
    writeFileSync(join(root, "astro.config.mjs"), `import { defineConfig } from 'astro/config';\nimport { changelogDev } from ${JSON.stringify(plugin)};\nexport default defineConfig({ devToolbar: { enabled: false }, cacheDir: './.astro-cache', vite: { cacheDir: './.vite-cache', plugins: [changelogDev({ interval: 100 })] } });`);
    writeFileSync(join(root, "src/pages/changelog/[...page].astro"), `---
import {execFileSync} from 'node:child_process';
export function getStaticPaths() {
 const subjects=execFileSync('git',['log','--format=%s'],{encoding:'utf8'}).trim().split('\\n');
 return subjects.map((subject,i)=>({params:{page:i===0?undefined:String(i+1)},props:{subject}}));
}
---
<h1>{Astro.props.subject}</h1>`);
    git("init"); git("config", "user.name", "Test"); git("config", "user.email", "test@example.org");
    git("add", "package.json"); git("commit", "-m", "Initial history");
    child = spawn(process.execPath, [resolve("node_modules/astro/bin/astro.mjs"), "dev", "--ignore-lock", "--root", root, "--host", "127.0.0.1", "--port", "0"], { cwd: root, env: { ...process.env, ASTRO_TELEMETRY_DISABLED: "1", NO_COLOR: "1" }, stdio: ["ignore", "pipe", "pipe"] });
    const record = (chunk) => { logs += chunk.toString(); };
    child.stdout.on("data", record); child.stderr.on("data", record);
    const address = await until(() => logs.match(/http:\/\/127\.0\.0\.1:\d+/)?.[0]);
    const read = async (path) => { const response = await fetch(address + path); return { status: response.status, text: await response.text() }; };
    await until(async () => (await read("/changelog/")).text.includes("<h1>Initial history</h1>"));
    assert.equal((await read("/changelog/2/")).status, 404);
    const client = await (await fetch(address + "/@vite/client")).text();
    const token = client.match(/const wsToken = "([^"]+)"/)?.[1];
    assert.ok(token, "Vite exposes a token for its development WebSocket");
    socket = new WebSocket(address.replace("http:", "ws:") + "/?token=" + token, "vite-hmr");
    const notifications = [];
    socket.addEventListener("message", (event) => notifications.push(JSON.parse(event.data)));
    await once(socket, "open");
    git("commit", "--allow-empty", "-m", "New commit without source edits");
    await until(() => notifications.some((message) => message.event === "tracker:git-changed"));
    await until(async () => (await read("/changelog/")).text.includes("<h1>New commit without source edits</h1>"));
    const older = await read("/changelog/2/");
    assert.equal(older.status, 200);
    assert.ok(older.text.includes("<h1>Initial history</h1>"));
    // Branch/revision changes also invalidate cached paths, including removed pages.
    git("checkout", "--detach", "HEAD~1");
    await until(async () => (await read("/changelog/")).text.includes("<h1>Initial history</h1>"));
    assert.equal((await read("/changelog/2/")).status, 404);
  } finally {
    socket?.close();
    if (child && child.exitCode === null) {
      const exited = once(child, "exit");
      child.kill("SIGTERM");
      await exited;
    }
    rmSync(root, { recursive: true, force: true });
  }
});
