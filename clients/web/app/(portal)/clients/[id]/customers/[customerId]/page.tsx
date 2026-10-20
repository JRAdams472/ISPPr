"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useState } from "react";
import { PageHeader } from "@/components/page-header";
import { CardSkeleton, EmptyState, ErrorState } from "@/components/states";
import {
  useCustomer,
  useCustomerVariants,
  useMasters,
  useMe,
} from "@/lib/queries";

// /clients/[id]/customers/[customerId] — the customer's template-override
// workspace: existing saved variants plus a picker to start a new override
// against any of the client's masters.

function OverridePicker({
  clientId,
  customerId,
}: {
  clientId: string;
  customerId: string;
}) {
  const masters = useMasters(clientId);
  const router = useRouter();
  const [open, setOpen] = useState(false);

  if (!open) {
    return (
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="rounded-md bg-zinc-900 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-zinc-800"
      >
        New override
      </button>
    );
  }
  return (
    <div className="flex items-center gap-2">
      <select
        aria-label="Choose a template master"
        className="rounded-md border border-zinc-300 bg-white px-2.5 py-2 text-sm text-zinc-900 focus:border-teal-700 focus:outline-none focus:ring-2 focus:ring-teal-700/20"
        defaultValue=""
        onChange={(e) => {
          const id = e.target.value;
          if (id) {
            router.push(
              `/clients/${clientId}/customers/${customerId}/variants/${id}`,
            );
          }
        }}
      >
        <option value="" disabled>
          {masters.isLoading ? "Loading masters…" : "Choose a template…"}
        </option>
        {(masters.data ?? []).map((m) => (
          <option key={m.id} value={m.id}>
            {m.name} (v{m.version})
          </option>
        ))}
      </select>
      <button
        type="button"
        onClick={() => setOpen(false)}
        className="rounded-md px-2.5 py-2 text-sm font-medium text-zinc-600 hover:bg-zinc-100"
      >
        Cancel
      </button>
    </div>
  );
}

export default function CustomerWorkspace() {
  const params = useParams();
  const clientId = typeof params.id === "string" ? params.id : "";
  const customerId =
    typeof params.customerId === "string" ? params.customerId : "";
  const me = useMe();
  const customer = useCustomer(customerId);
  const variants = useCustomerVariants(customerId);
  const isAdmin = me.data?.role === "admin";

  if (customer.isError || (customer.data && customer.data.client_id !== clientId)) {
    return (
      <>
        <PageHeader title="Customer" />
        <ErrorState
          message={customer.error?.message ?? "Customer not found."}
          onRetry={() => void customer.refetch()}
        />
      </>
    );
  }

  const c = customer.data;

  return (
    <>
      <PageHeader
        title={c ? c.name : "Customer"}
        description=""
        actions={isAdmin && c ? (
          <OverridePicker clientId={clientId} customerId={customerId} />
        ) : null}
      />

      {customer.isLoading ? <CardSkeleton lines={2} /> : null}

      {c ? (
        <div className="mb-6 -mt-4 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-zinc-500">
          <span className="font-mono text-xs">{c.id}</span>
          {c.external_ref ? (
            <span className="font-mono text-xs">ref {c.external_ref}</span>
          ) : null}
          {c.webhook_url ? (
            <span className="max-w-72 truncate font-mono text-xs" title={c.webhook_url}>
              {c.webhook_url}
            </span>
          ) : null}
          <Link
            href={`/clients/${clientId}`}
            className="text-teal-800 hover:underline"
          >
            Back to client
          </Link>
        </div>
      ) : null}

      <section aria-labelledby="overrides-heading">
        <h2
          id="overrides-heading"
          className="mb-3 text-sm font-semibold text-zinc-900"
        >
          Template overrides
        </h2>

        {variants.isLoading ? <CardSkeleton /> : null}
        {variants.isError ? (
          <ErrorState
            message={variants.error.message}
            onRetry={() => void variants.refetch()}
          />
        ) : null}

        {variants.data && variants.data.length === 0 ? (
          <EmptyState
            title="No overrides yet"
            hint={
              isAdmin
                ? "Start with New override — pick a template, tweak blocks, branding, and CSS, preview, then save."
                : "This customer resolves every template straight from the master."
            }
          />
        ) : null}

        {variants.data && variants.data.length > 0 ? (
          <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-zinc-200 bg-zinc-50 text-left text-xs font-medium uppercase tracking-wide text-zinc-500">
                  <th className="px-4 py-2.5">Template</th>
                  <th className="px-4 py-2.5">Overlay</th>
                  <th className="px-4 py-2.5">Updated</th>
                  <th className="px-4 py-2.5" aria-label="Open" />
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-100">
                {variants.data.map((v) => {
                  const parts: string[] = [];
                  if (v.overlay.css_patch) parts.push("css");
                  if (v.overlay.toggles && Object.keys(v.overlay.toggles).length)
                    parts.push(`${Object.keys(v.overlay.toggles).length} toggle(s)`);
                  if (v.overlay.branding && Object.keys(v.overlay.branding).length)
                    parts.push(
                      `${Object.keys(v.overlay.branding).length} branding`,
                    );
                  return (
                    <tr key={v.id} className="group">
                      <td className="px-4 py-3">
                        <p className="font-medium text-zinc-900">
                          {v.master_name ?? v.master_id}
                        </p>
                        <p className="font-mono text-[11px] text-zinc-400">
                          {v.master_id}
                        </p>
                      </td>
                      <td className="px-4 py-3 text-xs text-zinc-600">
                        {parts.length ? parts.join(" · ") : "empty overlay"}
                      </td>
                      <td className="px-4 py-3 text-xs text-zinc-500">
                        {new Date(v.updated_at).toLocaleString(undefined, {
                          month: "short",
                          day: "numeric",
                          hour: "2-digit",
                          minute: "2-digit",
                        })}
                      </td>
                      <td className="px-4 py-3 text-right">
                        <Link
                          href={`/clients/${clientId}/customers/${customerId}/variants/${v.master_id}`}
                          className="text-xs font-medium text-teal-800 opacity-0 transition-opacity hover:underline focus:opacity-100 group-hover:opacity-100"
                        >
                          {isAdmin ? "Edit" : "View"}
                        </Link>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        ) : null}
      </section>
    </>
  );
}
