const base = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

export async function health(): Promise<{ status: string }> {
  const res = await fetch(`${base}/api/health`, { cache: "no-store" });
  if (!res.ok) throw new Error(`health check failed: ${res.status}`);
  return res.json();
}
