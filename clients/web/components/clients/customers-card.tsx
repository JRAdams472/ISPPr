"use client";

import { CardSkeleton, EmptyState, ErrorState } from "@/components/states";
import { useCustomers } from "@/lib/queries";

// Read-only customer roster on the client detail page. Customer
// create/edit lands in MPE-42; this card just shows who exists.

export function CustomersCard({ clientId }: { clientId: string }) {
  const list = useCustomers(clientId);

  return (
    <section aria-labelledby="customers-heading">
      <h2
        id="customers-heading"
        className="mb-3 text-sm font-semibold text-zinc-900"
      >
        Customers
      </h2>

      {list.isLoading ? <CardSkeleton /> : null}
      {list.isError ? (
        <ErrorState
          message={list.error.message}
          onRetry={() => void list.refetch()}
        />
      ) : null}

      {list.data && list.data.length === 0 ? (
        <EmptyState
          title="No customers"
          hint="Customers carry webhook targets and template overrides."
        />
      ) : null}

      {list.data && list.data.length > 0 ? (
        <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-zinc-200 bg-zinc-50 text-left text-xs font-medium uppercase tracking-wide text-zinc-500">
                <th className="px-4 py-2.5">Customer</th>
                <th className="px-4 py-2.5">External ref</th>
                <th className="px-4 py-2.5">Webhook</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-100">
              {list.data.map((cu) => (
                <tr key={cu.id}>
                  <td className="px-4 py-3">
                    <p className="font-medium text-zinc-900">{cu.name}</p>
                    <p className="font-mono text-[11px] text-zinc-400">
                      {cu.id}
                    </p>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-zinc-600">
                    {cu.external_ref || "–"}
                  </td>
                  <td className="px-4 py-3 text-zinc-500">
                    {cu.webhook_url ? (
                      <span
                        className="block max-w-56 truncate font-mono text-xs"
                        title={cu.webhook_url}
                      >
                        {cu.webhook_url}
                      </span>
                    ) : (
                      "–"
                    )}
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
