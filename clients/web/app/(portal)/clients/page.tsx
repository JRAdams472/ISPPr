"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { PageHeader } from "@/components/page-header";
import { EmptyState, ErrorState, TableSkeleton } from "@/components/states";
import { StatusPill } from "@/components/status-pill";
import { PlusIcon } from "@/components/icons";
import { clients } from "@/lib/api/resources";
import type { Client } from "@/lib/api/types";
import { keys, useClients, useMe } from "@/lib/queries";

// /clients — tenant roster. Reads are open to viewers; create and
// enable/disable are admin-only (server enforces too, the UI just doesn't
// offer the affordance).

function fmtDate(iso: string) {
  return new Date(iso).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

function NewClientDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (c: Client) => void;
}) {
  const qc = useQueryClient();
  const [name, setName] = useState("");
  const create = useMutation({
    mutationFn: () => clients.create({ name: name.trim() }),
    onSuccess: (c) => {
      void qc.invalidateQueries({ queryKey: keys.clients });
      setName("");
      onClose();
      onCreated(c);
    },
  });

  if (!open) return null;
  return (
    <div
      className="fixed inset-0 z-40 flex items-center justify-center bg-zinc-950/40 px-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="new-client-title"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="w-full max-w-sm rounded-lg border border-zinc-200 bg-white p-6 shadow-lg">
        <h2
          id="new-client-title"
          className="text-base font-semibold tracking-tight text-zinc-900"
        >
          New client
        </h2>
        <p className="mt-1 text-sm text-zinc-500">
          A client owns customers, templates, and API keys.
        </p>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (name.trim()) create.mutate();
          }}
          className="mt-5 space-y-4"
        >
          <div className="space-y-2">
            <label
              htmlFor="client-name"
              className="block text-sm font-medium text-zinc-700"
            >
              Name
            </label>
            <input
              id="client-name"
              autoFocus
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="w-full rounded-md border border-zinc-300 px-3 py-2 text-sm text-zinc-900 outline-none focus:border-teal-600 focus:ring-1 focus:ring-teal-600"
            />
          </div>
          {create.error ? (
            <p role="alert" className="text-sm text-red-700">
              {create.error.message}
            </p>
          ) : null}
          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={onClose}
              className="rounded-md px-3 py-2 text-sm font-medium text-zinc-600 hover:bg-zinc-100"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={create.isPending || !name.trim()}
              className="rounded-md bg-teal-700 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-teal-800 disabled:opacity-60"
            >
              {create.isPending ? "Creating…" : "Create client"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

export default function ClientsPage() {
  const router = useRouter();
  const qc = useQueryClient();
  const me = useMe();
  const list = useClients();
  const isAdmin = me.data?.role === "admin";
  const [dialogOpen, setDialogOpen] = useState(false);

  const setStatus = useMutation({
    mutationFn: ({ id, status }: { id: string; status: string }) =>
      clients.update(id, { status }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: keys.clients }),
  });

  return (
    <>
      <PageHeader
        title="Clients"
        description="Tenants, their customers, and API keys."
        actions={
          isAdmin ? (
            <button
              type="button"
              onClick={() => setDialogOpen(true)}
              className="inline-flex items-center gap-1.5 rounded-md bg-teal-700 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-teal-800"
            >
              <PlusIcon />
              New client
            </button>
          ) : null
        }
      />

      {list.isLoading ? <TableSkeleton cols={5} /> : null}

      {list.isError ? (
        <ErrorState
          message={list.error.message}
          onRetry={() => void list.refetch()}
        />
      ) : null}

      {list.data && list.data.length === 0 ? (
        <EmptyState
          title="No clients yet"
          hint="Create a client to issue API keys and start generating documents."
          action={
            isAdmin ? (
              <button
                type="button"
                onClick={() => setDialogOpen(true)}
                className="inline-flex items-center gap-1.5 rounded-md bg-teal-700 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-teal-800"
              >
                <PlusIcon />
                New client
              </button>
            ) : null
          }
        />
      ) : null}

      {list.data && list.data.length > 0 ? (
        <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-zinc-200 bg-zinc-50 text-left text-xs font-medium uppercase tracking-wide text-zinc-500">
                <th className="px-4 py-2.5">Client</th>
                <th className="px-4 py-2.5">Status</th>
                <th className="px-4 py-2.5 text-right">Customers</th>
                <th className="px-4 py-2.5 text-right">Active keys</th>
                <th className="px-4 py-2.5">Created</th>
                {isAdmin ? <th className="px-4 py-2.5" /> : null}
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-100">
              {list.data.map((c) => (
                <tr
                  key={c.id}
                  onClick={() => router.push(`/clients/${c.id}`)}
                  className="cursor-pointer transition-colors hover:bg-stone-50"
                >
                  <td className="px-4 py-3">
                    <p className="font-medium text-zinc-900">{c.name}</p>
                    <p className="font-mono text-[11px] text-zinc-400">
                      {c.id}
                    </p>
                  </td>
                  <td className="px-4 py-3">
                    <StatusPill value={c.status} />
                  </td>
                  <td className="px-4 py-3 text-right tabular-nums text-zinc-700">
                    {c.customers ?? "–"}
                  </td>
                  <td className="px-4 py-3 text-right tabular-nums text-zinc-700">
                    {c.active_keys ?? "–"}
                  </td>
                  <td className="px-4 py-3 text-zinc-500">
                    {fmtDate(c.created_at)}
                  </td>
                  {isAdmin ? (
                    <td className="px-4 py-3 text-right">
                      <button
                        type="button"
                        onClick={(e) => {
                          e.stopPropagation();
                          setStatus.mutate({
                            id: c.id,
                            status:
                              c.status === "active" ? "disabled" : "active",
                          });
                        }}
                        className="rounded-md px-2.5 py-1 text-xs font-medium text-zinc-600 ring-1 ring-inset ring-zinc-300 transition-colors hover:bg-zinc-100"
                      >
                        {c.status === "active" ? "Disable" : "Enable"}
                      </button>
                    </td>
                  ) : null}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}

      <NewClientDialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        onCreated={(c) => router.push(`/clients/${c.id}`)}
      />
    </>
  );
}
