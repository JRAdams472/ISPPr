"use client";

import { CardSkeleton, EmptyState, ErrorState } from "@/components/states";
import { useRequests } from "@/lib/queries";

// Failed-request inspector on the client detail page — the recent
// failures an operator most likely needs to chase. The global monitor
// with all statuses lands in MPE-43.

function fmtTime(iso: string) {
  return new Date(iso).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function FailedRequests({ clientId }: { clientId: string }) {
  const list = useRequests({ client_id: clientId, status: "FAILED", limit: 10 });

  return (
    <section aria-labelledby="failed-heading">
      <h2
        id="failed-heading"
        className="mb-3 text-sm font-semibold text-zinc-900"
      >
        Failed requests
      </h2>

      {list.isLoading ? <CardSkeleton /> : null}
      {list.isError ? (
        <ErrorState
          message={list.error.message}
          onRetry={() => void list.refetch()}
        />
      ) : null}

      {list.data && list.data.length === 0 ? (
        <EmptyState title="No failed requests" hint="Nothing has failed recently." />
      ) : null}

      {list.data && list.data.length > 0 ? (
        <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-zinc-200 bg-zinc-50 text-left text-xs font-medium uppercase tracking-wide text-zinc-500">
                <th className="px-4 py-2.5">Request</th>
                <th className="px-4 py-2.5">Package</th>
                <th className="px-4 py-2.5">Error</th>
                <th className="px-4 py-2.5">When</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-100">
              {list.data.map((r) => (
                <tr key={r.id}>
                  <td className="px-4 py-3 font-mono text-xs text-zinc-600">
                    {r.id}
                  </td>
                  <td className="px-4 py-3 font-medium text-zinc-900">
                    {r.package_name}
                  </td>
                  <td className="max-w-0 truncate px-4 py-3 text-red-700" title={r.error}>
                    {r.error || "–"}
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
    </section>
  );
}
