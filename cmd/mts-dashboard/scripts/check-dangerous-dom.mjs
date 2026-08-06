import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { fileURLToPath, pathToFileURL } from "node:url";

const sourceExtensions = new Set([".js", ".ts", ".vue"]);
const dangerousSinks = [
  ["v-html", /\bv-html\s*=/u],
  ["innerHTML", /\.innerHTML\b/u],
  ["outerHTML", /\.outerHTML\b/u],
  ["insertAdjacentHTML", /\.insertAdjacentHTML\s*\(/u],
  ["document.write", /\bdocument\.write\s*\(/u],
  ["eval", /\beval\s*\(/u],
  ["new Function", /\bnew\s+Function\s*\(/u],
];

export function findDangerousDOMUsages(source, file) {
  const usages = [];
  const lines = source.split(/\r?\n/u);
  for (const [index, line] of lines.entries()) {
    for (const [sink, pattern] of dangerousSinks) {
      if (pattern.test(line)) {
        usages.push({ file, line: index + 1, sink });
      }
    }
  }
  return usages;
}

async function sourceFiles(root) {
  const entries = await readdir(root, { withFileTypes: true });
  const files = [];
  for (const entry of entries) {
    const entryPath = path.join(root, entry.name);
    if (entry.isDirectory()) {
      files.push(...await sourceFiles(entryPath));
    } else if (entry.isFile() && sourceExtensions.has(path.extname(entry.name))) {
      files.push(entryPath);
    }
  }
  return files.sort();
}

export async function scanDangerousDOM(root) {
  const files = await sourceFiles(root);
  const usages = [];
  for (const file of files) {
    const source = await readFile(file, "utf8");
    usages.push(...findDangerousDOMUsages(source, path.relative(process.cwd(), file)));
  }
  return usages;
}

async function main() {
  const root = path.resolve(process.argv[2] ?? "src");
  const usages = await scanDangerousDOM(root);
  for (const usage of usages) {
    console.error(`${usage.file}:${usage.line}: dangerous DOM sink: ${usage.sink}`);
  }
  if (usages.length > 0) {
    process.exitCode = 1;
  }
}

const invokedPath = process.argv[1] ? pathToFileURL(path.resolve(process.argv[1])).href : "";
if (import.meta.url === invokedPath) {
  main().catch((error) => {
    const script = path.basename(fileURLToPath(import.meta.url));
    console.error(`${script}: ${error instanceof Error ? error.message : String(error)}`);
    process.exitCode = 1;
  });
}
