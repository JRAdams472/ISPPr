#!/usr/bin/env node
// e2e — one-command pipeline integration suite (MPE-38).
//
//   node tools/e2e.mjs          # bring the stack up, wait, run go test ./e2e
//   node tools/e2e.mjs --down   # also tear the stack down afterwards
//
// The suite (e2e/pipeline_test.go) itself only needs a running stack —
// `go test ./e2e/...` works directly when one is already up.

import { execSync, spawn } from "node:child_process";

const API = process.env.API_URL ?? "http://localhost:8090";

function sh(cmd, opts = {}) {
	return execSync(cmd, { stdio: "inherit", ...opts });
}

async function waitHealthy(timeoutMs = 120_000) {
	const deadline = Date.now() + timeoutMs;
	while (Date.now() < deadline) {
		try {
			const r = await fetch(`${API}/healthz`);
			if (r.ok) return;
		} catch { /* still booting */ }
		await new Promise((r) => setTimeout(r, 1500));
	}
	throw new Error(`api never became healthy at ${API}`);
}

console.log("==> docker compose up -d --build");
sh("docker compose up -d --build");

console.log(`==> waiting for ${API}/healthz`);
await waitHealthy();

console.log("==> go test -count=1 -v ./e2e/...");
const code = await new Promise((resolve) => {
	const p = spawn("go", ["test", "-count=1", "-v", "./e2e/..."], { stdio: "inherit" });
	p.on("close", resolve);
});

if (process.argv.includes("--down")) {
	console.log("==> docker compose down");
	sh("docker compose down");
}
process.exit(code);
