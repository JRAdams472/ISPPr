"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";
import { PlusIcon } from "@/components/icons";
import { CardSkeleton, EmptyState, ErrorState } from "@/components/states";
import { customers } from "@/lib/api/resources";
import { keys, useCustomers } from "@/lib/queries";

// Customer roster on the client detail page: admins can create customers
// inline, and each row links to the customer's template-override
// workspace (MPE-42).

function NewCustomerDialog({
  clientId,
  open,
  onClose,
}: {
  clientId: string;
  open: boolean;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const [name, setName] = useState("");
  const [externalRef, setExternalRef] = useState("");
  const [webhookUrl, setWebhookUrl] = useState("");
  const create = useMutation({
    mutationFn: () =>
      customers.create(clientId, {
        name: name.trim(),
        external_ref: externalRef.trim() || undefined,
        webhook_url: webhookUrl.trim() || undefined,
      }),
    onSuccess: () => {
      setName("");
      setExternalRef("");
      setWebhookUrl("");
      void qc.invalidateQueries({ queryKey: keys.customers(clientId) });
      onClose();
    },
  });

  if (!open) return null;
  const field =
    "w-full rounded-md border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 placeholder:text-zinc-400 focus:border-teal-700 focus:outline-none focus:ring-2 focus:ring-teal-700/20";

  return (
    <div
      className="fixed inset-0 z-40 flex items-center justify-center bg-zinc-950/40 px-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="new-customer-title"
    >
      <form
        className="w-full max-w-md rounded-lg border border-zinc-200 bg-white p-6 shadow-lg"
        onSubmit={(e) => {
          e.preventDefault();
          if (name.trim()) create.mutate();
        }}
      >
        <h2
          id="new-customer-title"
          className="text-base font-semibold tracking-tight text-zinc-900"
        >
          New customer
        </h2>
        <p className="mt-1 text-sm text-zinc-500">
          Customers own webhook callbacks and per-template overrides.
        </p>

        <div className="mt-4 space-y-3">
          <label className="block">
            <span className="mb-1 block text-xs font-medium text-zinc-600">
              Name
            </span>
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              className={field}
              placeholder="Acme Corp"
              autoFocus
              required
            />
          </label>
          <label className="block">
            <span className="mb-1 block text-xs font-medium text-zinc-600">
              External ref <span className="text-zinc-400">(optional)</span>
            </span>
            <input
              value={externalRef}
              onChange={(e) => setExternalRef(e.target.value)}
              className={field}
              placeholder="your-crm-id"
            />
          </label>
          <label className="block">
            <span className="mb-1 block text-xs font-medium text-zinc-600">
              Webhook URL <span className="text-zinc-400">(optional)</span>
            </span>
            <input
              value={webhookUrl}
              onChange={(e) => setWebhookUrl(e.target.value)}
              className={field}
              placeholder="https://…"
              type="url"
            />
          </label>
        </div>

        {create.isError ? (
          <p role="alert" className="mt-3 text-sm text-red-700">
            {create.error.message}
          </p>
        ) : null}

        <div className="mt-5 flex justify-end gap-2">
          <button
            type="button"
            onClick={onClose}
            className="rounded-md px-3 py-2 text-sm font-medium text-zinc-600 hover:bg-zinc-100"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={!name.trim() || create.isPending}
            className="rounded-md bg-zinc-900 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-zinc-800 disabled:opacity-60"
          >
            {create.isPending ? "Creating…" : "Create customer"}
          </button>
        </div>
      </form>
    </div>
  );
}

export function CustomersCard({
  clientId,
  isAdmin,
}: {
  clientId: string;
  isAdmin?: boolean;
}) {
  const list = useCustomers(clientId);
  const [showNew, setShowNew] = useState(false);

  return (
    <section aria-labelledby="customers-heading">
      <div className="mb-3 flex items-center justify-between">
        <h2
          id="customers-heading"
          className="text-sm font-semibold text-zinc-900"
        >
          Customers
        </h2>
        {isAdmin ? (
          <button
            type="button"
            onClick={() => setShowNew(true)}
            className="inline-flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-xs font-medium text-zinc-700 ring-1 ring-inset ring-zinc-300 transition-colors hover:bg-zinc-100"
          >
            <PlusIcon /> New customer
          </button>
        ) : null}
      </div>

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
                <th className="px-4 py-2.5" aria-label="Actions" />
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-100">
              {list.data.map((cu) => (
                <tr key={cu.id} className="group">
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
                  <td className="px-4 py-3 text-right">
                    <Link
                      href={`/clients/${clientId}/customers/${cu.id}`}
                      className="text-xs font-medium text-teal-800 opacity-0 transition-opacity hover:underline focus:opacity-100 group-hover:opacity-100"
                    >
                      Template overrides
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}

      <NewCustomerDialog
        clientId={clientId}
        open={showNew}
        onClose={() => setShowNew(false)}
      />
    </section>
  );
}
