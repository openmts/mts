import assert from "node:assert/strict";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import process from "node:process";
import { spawnSync } from "node:child_process";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { findDangerousDOMUsages } from "./check-dangerous-dom.mjs";

test("报告全部危险 DOM sink 并保留行号", () => {
  const source = `<div v-html="html"></div>
element.innerHTML = html;
element.outerHTML = html;
element.insertAdjacentHTML("beforeend", html);
document.write(html);
eval(code);
const compile = new Function(code);`;

  assert.deepEqual(findDangerousDOMUsages(source, "src/example.vue"), [
    { file: "src/example.vue", line: 1, sink: "v-html" },
    { file: "src/example.vue", line: 2, sink: "innerHTML" },
    { file: "src/example.vue", line: 3, sink: "outerHTML" },
    { file: "src/example.vue", line: 4, sink: "insertAdjacentHTML" },
    { file: "src/example.vue", line: 5, sink: "document.write" },
    { file: "src/example.vue", line: 6, sink: "eval" },
    { file: "src/example.vue", line: 7, sink: "new Function" },
  ]);
});

test("忽略安全的文本写入和 Vue interpolation", () => {
  const source = `<p>{{ message }}</p>
element.textContent = message;`;

  assert.deepEqual(findDangerousDOMUsages(source, "src/safe.vue"), []);
});

test("CLI 在源码包含危险 sink 时阻塞", async (t) => {
  const root = await mkdtemp(path.join(os.tmpdir(), "mts-dashboard-dom-"));
  await writeFile(path.join(root, "unsafe.ts"), "element.innerHTML = html;\n", { mode: 0o600 });
  t.after(async () => {
    await rm(root, { recursive: true });
  });

  const script = fileURLToPath(new URL("./check-dangerous-dom.mjs", import.meta.url));
  const result = spawnSync(process.execPath, [script, root], { encoding: "utf8" });

  assert.equal(result.error, undefined);
  assert.equal(result.status, 1);
  assert.match(result.stderr, /unsafe\.ts:1: dangerous DOM sink: innerHTML/u);
});
