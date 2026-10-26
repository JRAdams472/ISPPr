"use client";

import Link from "next/link";
import { useState } from "react";
import { PageHeader } from "@/components/page-header";
import { EmptyState, ErrorState, TableSkeleton } from "@/components/states";
import type { TemplateMaster } from "@/lib/api/types";
import { useClients, useMasters } from "@/lib/queries";

// Templates home (MPE-51): pick a client, list its masters rolled up to
// the latest version, create new ones.
export default function Templates() {
  const clients = useClients();
  const [clientId, setClientId] = useState("");
  const effective = clientId || clients.data?.[0]?.id || "";
  const masters = useMasters(effective);

  const latest = Object.values(
    (masters.data ?? []).reduce<Record<string, TemplateMaster>>(
      (acc, m) => {
        const cur = acc[m.name];
        if (!cur || m.version > cur.version) acc[m.name] = m;
        return acc;
      },
      {},
    ),
  ).sort((a, b) => a.name.localeCompare(b.name));

  const versionsOf = (name: string) =>
    (masters.data ?? []).filter((m) => m.name === name).length;

  return (
    <>
      <PageHeader
        title="Templates"
        description="Template masters — the canonical document each customer variant refines."
        actions={
          <div className="flex items-center gap-3">
            <select
              aria-label="Client"
              value={effective}
              onChange={(e) => setClientId(e.target.value)}
              className="rounded-md border border-zinc-300 bg-white px-2.5 py-1.5 text-sm text-zinc-800 focus:border-teal-700 focus:outline-none focus:ring-2 focus:ring-teal-700/20"
            >
              {(clients.data ?? []).map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </select>
            <Link
              href={`/templates/new?client=${effective}`}
              className="rounded-md bg-zinc-900 px-3.5 py-2 text-sm font-medium text-white transition-colors hover:bg-zinc-800"
            >
              New template
            </Link>
          </div>
        }
      />

      {masters.isLoading ? (
        <TableSkeleton cols={3} rows={3} />
      ) : masters.isError ? (
        <ErrorState
          message={masters.error.message}
          onRetry={() => void masters.refetch()}
        />
      ) : latest.length === 0 ? (
        <EmptyState
          title="No template masters"
          hint="Create the first template for this client — it becomes the base every customer variant can restyle."
          action={
            <Link
              href={`/templates/new?client=${effective}`}
              className="rounded-md bg-zinc-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-zinc-800"
            >
              New template
            </Link>
          }
        />
      ) : (
        <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-zinc-200 text-xs font-medium uppercase tracking-wide text-zinc-500">
                <th className="px-4 py-2.5">Name</th>
                <th className="px-4 py-2.5">Latest</th>
                <th className="px-4 py-2.5">Versions</th>
                <th className="px-4 py-2.5">Updated</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-100">
              {latest.map((m) => (
                <tr key={m.id} className="transition-colors hover:bg-stone-50">
                  <td className="px-4 py-2.5">
                    <Link
                      href={`/templates/masters/${m.id}`}
                      className="font-mono text-xs font-medium text-zinc-900 hover:text-teal-800"
                    >
                      {m.name}
                    </Link>
                  </td>
                  <td className="px-4 py-2.5 text-xs text-zinc-600">
                    v{m.version}
                  </td>
                  <td className="px-4 py-2.5 text-xs text-zinc-600">
                    {versionsOf(m.name)}
                  </td>
                  <td className="px-4 py-2.5 text-xs text-zinc-500">
                    {new Date(m.created_at).toLocaleString()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}
