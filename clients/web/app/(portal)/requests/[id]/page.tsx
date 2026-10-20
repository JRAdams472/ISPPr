"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { BackIcon, DownloadIcon } from "@/components/icons";
import { PageHeader } from "@/components/page-header";
import { CardSkeleton, ErrorState } from "@/components/states";
import { StatusPill } from "@/components/status-pill";
import type { RequestMetrics } from "@/lib/api/types";
import { useRequestView } from "@/lib/queries";

// /requests/[id] — the end-to-end trace for a single generation request:
// lifecycle, render-phase waterfall, output re-download, webhook attempts.

function fmtTime(iso: string) {
  return new Date(iso).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function fmtMs(v: number | undefined) {
  if (v === undefined) return "–";
  return v >= 1000 ? `${(v / 1000).toFixed(2)}s` : `${v}ms`;
}

// Segmented duration bar: each render phase sized proportionally to the
// request's total, with queue/other time shown as the zinc remainder.
function Waterfall({ m }: { m: RequestMetrics }) {
  const total = m.total_duration_ms || 1;
  const phases = [
    { label: "Parsing", ms: m.parsing_duration_ms ?? 0, cls: "bg-teal-300" },
    { label: "Rendering", ms: m.rendering_duration_ms ?? 0, cls: "bg-teal-600" },
    { label: "Compression", ms: m.compression_duration_ms ?? 0, cls: "bg-teal-800" },
  ];
  const accounted = phases.reduce((n, p) => n + p.ms, 0);
  const rest = Math.max(total - accounted, 0);

  return (
    <section aria-labelledby="waterfall-heading">
      <h2 id="waterfall-heading" className="mb-3 text-sm font-semibold text-zinc-900">
        Render phases
      </h2>
      <div className="rounded-lg border border-zinc-200 bg-white p-5">
        <div
          className="flex h-6 w-full overflow-hidden rounded-md"
          role="img"
          aria-label={`Total ${fmtMs(m.total_duration_ms)}: parsing ${fmtMs(m.parsing_duration_ms)}, rendering ${fmtMs(m.rendering_duration_ms)}, compression ${fmtMs(m.compression_duration_ms)}`}
        >
          {phases.map((p) =>
            p.ms > 0 ? (
              <div
                key={p.label}
                className={p.cls}
                style={{ width: `${(p.ms / total) * 100}%` }}
              />
            ) : null,
          )}
          {rest > 0 ? (
            <div
              className="bg-zinc-200"
              style={{ width: `${(rest / total) * 100}%` }}
            />
          ) : null}
        </div>
        <dl className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-5">
          {phases.map((p) => (
            <div key={p.label}>
              <dt className="flex items-center gap-1.5 text-xs text-zinc-500">
                <span className={`inline-block h-2.5 w-2.5 rounded-sm ${p.cls}`} />
                {p.label}
              </dt>
              <dd className="mt-0.5 font-mono text-sm text-zinc-900">
                {fmtMs(p.ms)}
              </dd>
            </div>
          ))}
          <div>
            <dt className="flex items-center gap-1.5 text-xs text-zinc-500">
              <span className="inline-block h-2.5 w-2.5 rounded-sm bg-zinc-200" />
              Queue / other
            </dt>
            <dd className="mt-0.5 font-mono text-sm text-zinc-900">
              {fmtMs(rest)}
            </dd>
          </div>
          <div>
            <dt className="text-xs text-zinc-500">Total</dt>
            <dd className="mt-0.5 font-mono text-sm font-semibold text-zinc-900">
              {fmtMs(m.total_duration_ms)}
            </dd>
          </div>
        </dl>
        <p className="mt-3 text-xs text-zinc-500">
          {m.total_documents} document{m.total_documents === 1 ? "" : "s"} in
          this package
        </p>
      </div>
    </section>
  );
}

export default function RequestDetail() {
  const params = useParams();
  const id = typeof params.id === "string" ? params.id : "";
  const req = useRequestView(id);

  if (req.isLoading) {
    return (
      <>
        <PageHeader title="Request" />
        <div className="space-y-6">
          <CardSkeleton lines={4} />
          <CardSkeleton lines={4} />
        </div>
      </>
    );
  }
  if (req.isError) {
    return (
      <>
        <PageHeader title="Request" />
        <ErrorState
          message={req.error.message}
          onRetry={() => void req.refetch()}
        />
      </>
    );
  }
  const r = req.data;
  if (!r) return null;

  return (
    <>
      <Link
        href="/requests"
        className="mb-3 inline-flex items-center gap-1.5 text-sm font-medium text-zinc-500 hover:text-zinc-800"
      >
        <BackIcon />
        All requests
      </Link>
      <PageHeader
        title={r.package_name}
        description={r.id}
        actions={<StatusPill value={r.status} />}
      />

      <div className="grid gap-6 lg:grid-cols-3">
        <section className="rounded-lg border border-zinc-200 bg-white p-5">
          <h2 className="mb-3 text-sm font-semibold text-zinc-900">Detail</h2>
          <dl className="space-y-2.5 text-sm">
            <div className="flex justify-between gap-4">
              <dt className="text-zinc-500">Client</dt>
              <dd>
                <Link
                  href={`/clients/${r.client_id}`}
                  className="font-medium text-teal-700 hover:underline"
                >
                  {r.client_id}
                </Link>
              </dd>
            </div>
            <div className="flex justify-between gap-4">
              <dt className="text-zinc-500">Customer</dt>
              <dd className="font-medium text-zinc-900">{r.customer_id}</dd>
            </div>
            <div className="flex justify-between gap-4">
              <dt className="text-zinc-500">Created</dt>
              <dd className="text-zinc-900">{fmtTime(r.created_at)}</dd>
            </div>
          </dl>

          {r.error ? (
            <p className="mt-4 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-800">
              {r.error}
            </p>
          ) : null}

          {r.results ? (
            <div className="mt-4 border-t border-zinc-100 pt-4">
              <p className="mb-2 text-xs text-zinc-500">
                {r.results.filename} · {r.results.output_type}
              </p>
              <a
                href={r.results.download_url}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-1.5 rounded-md bg-teal-700 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-teal-800"
              >
                <DownloadIcon />
                Download output
              </a>
            </div>
          ) : null}
        </section>

        <div className="space-y-6 lg:col-span-2">
          {r.metrics?.total_duration_ms != null ? (
            <Waterfall m={r.metrics} />
          ) : null}

          <section aria-labelledby="webhook-heading">
            <h2
              id="webhook-heading"
              className="mb-3 flex items-center gap-2 text-sm font-semibold text-zinc-900"
            >
              Webhook delivery
              <StatusPill value={r.webhook_status} />
            </h2>
            <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white">
              {r.webhook_log.length === 0 ? (
                <p className="px-4 py-6 text-center text-sm text-zinc-500">
                  No delivery attempts — the customer has no webhook URL
                  configured, or the request is still running.
                </p>
              ) : (
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-zinc-200 bg-zinc-50 text-left text-xs font-medium uppercase tracking-wide text-zinc-500">
                      <th className="px-4 py-2.5">Attempt</th>
                      <th className="px-4 py-2.5">At</th>
                      <th className="px-4 py-2.5">Result</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-zinc-100">
                    {r.webhook_log.map((a) => (
                      <tr key={a.attempt}>
                        <td className="px-4 py-3 font-mono text-xs text-zinc-600">
                          #{a.attempt}
                        </td>
                        <td className="px-4 py-3 whitespace-nowrap text-zinc-500">
                          {fmtTime(a.at)}
                        </td>
                        <td className="px-4 py-3">
                          {a.code !== undefined ? (
                            <span
                              className={`font-mono text-xs ${
                                a.code < 300 ? "text-teal-700" : "text-red-700"
                              }`}
                            >
                              HTTP {a.code}
                            </span>
                          ) : null}
                          {a.error ? (
                            <span className="ml-2 text-red-700">{a.error}</span>
                          ) : null}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          </section>
        </div>
      </div>
    </>
  );
}
