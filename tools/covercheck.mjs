#!/usr/bin/env node
// covercheck — per-area coverage floor for ISPPr.
//
//   node tools/covercheck.mjs <coverage-file> [-floor 80] [-goal 95]
//
// <coverage-file> is either a Go coverprofile (*.out) or an lcov report
// (*.info); the format is detected by extension.
//
// Rules (repo policy):
//   * every "major code area" (directory containing covered source) must
//     meet -floor percent or the check fails;
//   * -goal is the overall target — reported, not enforced;
//   * generated files (banner "Code generated ... DO NOT EDIT" or the
//     *.sql.go convention) and test files are exempt;
//   * cmd/** composition roots are exempt — they're wiring verified by
//     compose/e2e, not unit coverage.

import { readFileSync, existsSync } from "node:fs";
import { dirname, extname, posix } from "node:path";

const args = process.argv.slice(2);
const file = args[0];
const flag = (n, d) => {
  const i = args.indexOf(`-${n}`);
  return i === -1 ? d : Number(args[i + 1]);
};
const FLOOR = flag("floor", 80);
const GOAL = flag("goal", 95);

if (!file || !existsSync(file)) {
  console.error(`usage: covercheck <coverage.out|lcov.info> [-floor 80] [-goal 95]`);
  process.exit(2);
}

const MODULE_PREFIX = "github.com/jradams472/isppr/";
const WEB_PREFIX = "clients/web/";

const SKIP_FILE = [/(_test\.go|\.test\.[jt]sx?|\.spec\.[jt]sx?)$/, /__tests__\//];
const SKIP_AREA = [/^cmd\//, /^\.$/]; // composition roots, repo root (assets.go embed)

function isGenerated(path) {
  if (/\.sql\.go$/.test(path)) return true;
  try {
    const head = readFileSync(path, "utf8").slice(0, 400);
    return head.includes("DO NOT EDIT");
  } catch {
    return false; // source gone (CI may check web paths from Go dir) — don't exempt
  }
}

// Go coverprofile: "pkg/dir/file.go:ls.cs,le.ce nStmt count".
// With -coverpkg each block repeats once per test binary — dedupe on the
// block key (path + range + nStmt) and OR-merge hit counts.
function parseGoCover(text) {
  const blocks = new Map(); // key -> { file, n, covered }
  for (const line of text.split("\n")) {
    const m = line.match(/^(\S+?):(\d+\.\d+,\d+\.\d+)\s+(\d+)\s+(\d+)$/);
    if (!m) continue;
    let path = m[1];
    if (path.startsWith(MODULE_PREFIX)) path = path.slice(MODULE_PREFIX.length);
    const key = `${path}:${m[2]}:${m[3]}`;
    const hit = Number(m[4]) > 0;
    const b = blocks.get(key);
    if (b) b.covered ||= hit;
    else blocks.set(key, { file: path, n: Number(m[3]), covered: hit });
  }
  const files = new Map(); // file -> [coveredStmts, totalStmts]
  for (const { file, n, covered } of blocks.values()) {
    const [c, t] = files.get(file) ?? [0, 0];
    files.set(file, [c + (covered ? n : 0), t + n]);
  }
  return files;
}

// lcov: "SF:<path>" ... "DA:line,hits" ... "end_of_record"
function parseLcov(text) {
  const files = new Map();
  let cur = null;
  for (const raw of text.split("\n")) {
    const line = raw.trim();
    if (line.startsWith("SF:")) {
      let p = line.slice(3).replaceAll("\\", "/");
      const i = p.indexOf(WEB_PREFIX);
      if (i !== -1) p = p.slice(i + WEB_PREFIX.length);
      cur = [0, 0];
      files.set(p, cur);
    } else if (line.startsWith("DA:") && cur) {
      const hits = Number(line.slice(3).split(",")[1]);
      cur[1]++;
      if (hits > 0) cur[0]++;
    }
  }
  return files;
}

const files = extname(file) === ".info" ? parseLcov(readFileSync(file, "utf8")) : parseGoCover(readFileSync(file, "utf8"));

const areas = new Map(); // dir -> [covered, total]
for (const [path, [cov, tot]] of files) {
  if (SKIP_FILE.some((r) => r.test(path)) || isGenerated(path)) continue;
  const dir = posix.dirname(path);
  if (SKIP_AREA.some((r) => r.test(dir))) continue;
  const [c, t] = areas.get(dir) ?? [0, 0];
  areas.set(dir, [c + cov, t + tot]);
}

console.log(`\ncovercheck — floor ${FLOOR}%/area, goal ${GOAL}% overall\n`);
let fail = false;
let sumC = 0, sumT = 0;
for (const [dir, [c, t]] of [...areas].sort()) {
  const pct = t ? (100 * c) / t : 100;
  sumC += c; sumT += t;
  const mark = pct < FLOOR ? "FAIL" : " ok ";
  if (pct < FLOOR) fail = true;
  console.log(` ${mark}  ${dir.padEnd(28)} ${pct.toFixed(1).padStart(5)}%  (${c}/${t} stmts|lines)`);
}
const overall = sumT ? (100 * sumC) / sumT : 100;
console.log(`\n overall: ${overall.toFixed(1)}%  (goal ${GOAL}%)`);
if (overall < GOAL) console.log(` note: overall below ${GOAL}% goal — directional, not a hard gate`);
process.exit(fail ? 1 : 0);
