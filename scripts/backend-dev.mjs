import { loadEnvFile } from "node:process";
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";

try { loadEnvFile(new URL("../.env", import.meta.url)); }
catch (error) { if (error.code !== "ENOENT") throw error; }
const env = { ...process.env };
env.HTTP_ADDR ||= "127.0.0.1:8080";
env.PUBLIC_BASE_URL ||= "http://localhost:4321";
env.CORS_ORIGINS ||= "http://localhost:4321,http://127.0.0.1:4321";
env.RESEARCH_RUNNER ||= "local";
env.RESEARCH_MODEL ||= "opencode-go/deepseek-v4.1-flash";
env.AUTO_PUBLISH ||= "true";
env.DEV_CONTROLS ||= "true";
env.RAW_AI_LOGS ||= "true";
const child = spawn("go", ["run", "./cmd/tracker"], { cwd: fileURLToPath(new URL("../backend/", import.meta.url)), env, stdio: "inherit" });
child.on("error", () => { console.error("Could not start the Go backend. Check that Go is installed."); process.exitCode = 1; });
child.on("exit", (code, signal) => { process.exitCode = signal ? 0 : code ?? 1; });
for (const signal of ["SIGINT", "SIGTERM"]) process.on(signal, () => child.kill(signal));
