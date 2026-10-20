"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { PageHeader } from "@/components/page-header";
import { EmptyState, ErrorState, TableSkeleton } from "@/components/states";
import { StatusPill } from "@/components/status-pill";
import { useClients, useRequests } from "@/lib/queries";

// The operator monitor: every generation request across tenants,
// newest-first, filterable by lifecycle status and client.

const STATUSES = [
  "QUEUED",
  "RENDERING",
  "COMPRESSING",
  "COMPLETED",
  "FAILED",
] as const;

function fmtTime(iso: string) {
  return new Date(iso).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function fmtMs(m?: { total_duration_ms?: number } | null) {
  const ms = m?.total_duration_ms;
  if (ms == null) return "–";
  return ms >= 1000 ? `${(ms / 1000).toFixed(1)}s` : `${ms}ms`;
}

export default function Requests() {
  const router = useRouter();
  const [status, setStatus] = useState("");
  const [clientId, setClientId] = useState("");
  const list = useRequests({
    status: status || undefined,
    client_id: clientId || undefined,
    limit: 100,
  });
  const clients = useClients();

  return (
    <>
      <PageHeader
        title="Requests"
        description="Every generation request, end to end — lifecycle, render metrics, delivery."
      />

      <div className="mb-4 flex flex-wrap items-center gap-3">
        <label className="flex items-center gap-2 text-sm text-zinc-600">
          Status
          <select
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            className="rounded-md border border-zinc-300 bg-white px-2.5 py-1.5 text-sm text-zinc-900 focus:border-teal-600 focus:outline-none focus:ring-2 focus:ring-teal-600/20"
          >
            <option value="">All</option>
            {STATUSES.map((s) => (
              <option key={s} value={s}>
                {s.toLowerCase()}
              </option>
            ))}
          </select>
        </label>
        <label className="flex items-center gap-2 text-sm text-zinc-600">
          Client
          <select
            value={clientId}
            onChange={(e) => setClientId(e.target.value)}
            className="rounded-md border border-zinc-300 bg-white px-2.5 py-1.5 text-sm text-zinc-900 focus:border-teal-600 focus:outline-none focus:ring-2 focus:ring-teal-600/20"
          >
            <option value="">All clients</option>
            {(clients.data ?? []).map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
        </label>
      </div>

      {list.isLoading ? <TableSkeleton cols={7} rows={6} /> : null}
      {list.isError ? (
        <ErrorState
          message={list.error.message}
          onRetry={() => void list.refetch()}
        />
      ) : null}

      {list.data && list.data.length === 0 ? (
        <EmptyState
          title="No requests match"
          hint="Loosen the filters or wait for traffic — generation requests appear here as they're submitted."
        />
      ) : null}

      {list.data && list.data.length > 0 ? (
        <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-zinc-200 bg-zinc-50 text-left text-xs font-medium uppercase tracking-wide text-zinc-500">
                <th className="px-4 py-2.5">Request</th>
                <th className="px-4 py-2.5">Package</th>
                <th className="px-4 py-2.5">Customer</th>
                <th className="px-4 py-2.5">Status</th>
                <th className="px-4 py-2.5">Duration</th>
                <th className="px-4 py-2.5">Webhook</th>
                <th className="px-4 py-2.5">Created</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-100">
              {list.data.map((r) => (
                <tr
                  key={r.id}
                  onClick={() => router.push(`/requests/${r.id}`)}
                  className="cursor-pointer transition-colors hover:bg-stone-50"
                >
                  <td className="px-4 py-3 font-mono text-xs text-zinc-600">
                    {r.id}
                  </td>
                  <td className="px-4 py-3 font-medium text-zinc-900">
                    {r.package_name}
                  </td>
                  <td className="px-4 py-3 text-zinc-600">{r.customer_id}</td>
                  <td className="px-4 py-3">
                    <StatusPill value={r.status} />
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap font-mono text-xs text-zinc-600">
                    {fmtMs(r.metrics)}
                  </td>
                  <td className="px-4 py-3">
                    <StatusPill value={r.webhook_status} />
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap text-zinc-500">
                    {fmtTime(r.created_at)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
    </>
  );
}
