// Explicit, reproducible CFI snapshot; research never needs a network glossary request.
import fs from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";
const source = process.argv[2] || "../cannabis-freedom-index";
let glossary;
if (/^https?:\/\//.test(source)) {
  const response = await fetch(new URL("api/glossary.json", source.endsWith("/") ? source : source + "/"), { signal: AbortSignal.timeout(30000) });
  if (!response.ok) throw new Error(`CFI glossary: HTTP ${response.status}`);
  glossary = await response.json();
} else {
  const root = path.resolve(source);
  const { glossaryAPI } = await import(pathToFileURL(path.join(root, "src/lib/glossary-api.js")));
  glossary = glossaryAPI(root);
}
if (glossary.version !== 1 || !glossary.recognition?.words?.length || !glossary.recognition?.prefixes?.length) throw new Error("Invalid CFI glossary");
const output = { version: glossary.version, source: "https://cannabisfreedom.fyi/api/glossary.json", recognition: glossary.recognition };
await fs.writeFile("backend/internal/safeurl/cannabis-glossary.json", JSON.stringify(output, null, 2) + "\n");
console.log(`CFI glossary: ${output.recognition.words.length} words and ${output.recognition.prefixes.length} inflection prefixes.`);
