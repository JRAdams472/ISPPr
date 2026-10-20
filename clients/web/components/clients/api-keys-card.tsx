"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { BanIcon, CheckIcon, CopyIcon, PlusIcon, RotateIcon } from "@/components/icons";
import { CardSkeleton, EmptyState, ErrorState } from "@/components/states";
import { StatusPill } from "@/components/status-pill";
import { apiKeys } from "@/lib/api/resources";
import type { ApiKey } from "@/lib/api/types";
import { keys, useApiKeys } from "@/lib/queries";

// API keys card on the client detail page. The plaintext credential is
// returned exactly once by the API (create or rotate) and shown in a
// modal; after that only the row metadata survives.

function fmtDate(iso?: string) {
  return iso
    ? new Date(iso).toLocaleDateString(undefined, {
        year: "numeric",
        month: "short",
        day: "numeric",
      })
    : "–";
}

function keyState(k: ApiKey) {
  return k.revoked_at ? "revoked" : "active";
}

function KeySecretDialog({
  secret,
  onClose,
}: {
  secret: { key: string; heading: string } | null;
  onClose: () => void;
}) {
  const [copied, setCopied] = useState(false);
  if (!secret) return null;
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(secret.key);
      setCopied(true);
    } catch {
      // clipboard blocked — the field stays selectable
    }
  };
  return (
    <div
      className="fixed inset-0 z-40 flex items-center justify-center bg-zinc-950/40 px-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="key-secret-title"
    >
      <div className="w-full max-w-md rounded-lg border border-zinc-200 bg-white p-6 shadow-lg">
        <h2
          id="key-secret-title"
          className="text-base font-semibold tracking-tight text-zinc-900"
        >
          {secret.heading}
        </h2>
        <p className="mt-1 text-sm text-zinc-500">
          This is the only time the key is shown. Copy it now — it cannot be
          recovered later.
        </p>
        <div className="mt-4 flex items-center gap-2">
          <code className="min-w-0 flex-1 truncate rounded-md border border-zinc-200 bg-stone-50 px-3 py-2 font-mono text-xs text-zinc-900 select-all">
            {secret.key}
          </code>
          <button
            type="button"
            onClick={copy}
            className="inline-flex items-center gap-1.5 rounded-md bg-zinc-900 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-zinc-800"
          >
            {copied ? <CheckIcon /> : <CopyIcon />}
            {copied ? "Copied" : "Copy"}
          </button>
        </div>
        <div className="mt-5 flex justify-end">
          <button
            type="button"
            onClick={onClose}
            className="rounded-md px-3 py-2 text-sm font-medium text-zinc-600 hover:bg-zinc-100"
          >
            Done
          </button>
        </div>
      </div>
    </div>
  );
}

function NewKeyDialog({
  clientId,
  open,
  onClose,
  onSecret,
}: {
  clientId: string;
  open: boolean;
  onClose: () => void;
  onSecret: (key: string) => void;
}) {
  const [label, setLabel] = useState("");
  const create = useMutation({
    mutationFn: () =>
      apiKeys.create(clientId, {
        label: label.trim() || undefined,
        scopes: ["generate"],
      }),
    onSuccess: (k) => {
      setLabel("");
      onClose();
      if (k.key) onSecret(k.key);
    },
  });

  if (!open) return null;
  return (
    <div
      className="fixed inset-0 z-40 flex items-center justify-center bg-zinc-950/40 px-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="new-key-title"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="w-full max-w-sm rounded-lg border border-zinc-200 bg-white p-6 shadow-lg">
        <h2
          id="new-key-title"
          className="text-base font-semibold tracking-tight text-zinc-900"
        >
          New API key
        </h2>
        <p className="mt-1 text-sm text-zinc-500">
          Keys carry the <code className="font-mono text-xs">generate</code>{" "}
          scope and authenticate tenant requests.
        </p>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            create.mutate();
          }}
          className="mt-5 space-y-4"
        >
          <div className="space-y-2">
            <label
              htmlFor="key-label"
              className="block text-sm font-medium text-zinc-700"
            >
              Label <span className="font-normal text-zinc-400">(optional)</span>
            </label>
            <input
              id="key-label"
              autoFocus
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              placeholder="production webapp"
              className="w-full rounded-md border border-zinc-300 px-3 py-2 text-sm text-zinc-900 outline-none placeholder:text-zinc-400 focus:border-teal-600 focus:ring-1 focus:ring-teal-600"
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
              disabled={create.isPending}
              className="rounded-md bg-teal-700 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-teal-800 disabled:opacity-60"
            >
              {create.isPending ? "Creating…" : "Create key"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

export function ApiKeysCard({
  clientId,
  isAdmin,
}: {
  clientId: string;
  isAdmin: boolean;
}) {
  const qc = useQueryClient();
  const list = useApiKeys(clientId);
  const [newOpen, setNewOpen] = useState(false);
  const [secret, setSecret] = useState<{ key: string; heading: string } | null>(
    null,
  );
  const invalidate = () =>
    void qc.invalidateQueries({ queryKey: keys.apiKeys(clientId) });

  const rotate = useMutation({
    mutationFn: (id: string) => apiKeys.rotate(id),
    onSuccess: (k) => {
      invalidate();
      if (k.key) setSecret({ key: k.key, heading: "Rotated key" });
    },
  });
  const revoke = useMutation({
    mutationFn: (id: string) => apiKeys.revoke(id),
    onSuccess: invalidate,
  });
  const actionError = rotate.error ?? revoke.error;

  return (
    <section aria-labelledby="api-keys-heading">
      <div className="mb-3 flex items-center justify-between">
        <h2
          id="api-keys-heading"
          className="text-sm font-semibold text-zinc-900"
        >
          API keys
        </h2>
        {isAdmin ? (
          <button
            type="button"
            onClick={() => setNewOpen(true)}
            className="inline-flex items-center gap-1 rounded-md px-2.5 py-1.5 text-xs font-medium text-teal-800 ring-1 ring-inset ring-teal-600/30 transition-colors hover:bg-teal-50"
          >
            <PlusIcon />
            New key
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
          title="No API keys"
          hint="Create a key so this client can call the generate endpoint."
        />
      ) : null}

      {list.data && list.data.length > 0 ? (
        <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-zinc-200 bg-zinc-50 text-left text-xs font-medium uppercase tracking-wide text-zinc-500">
                <th className="px-4 py-2.5">Label</th>
                <th className="px-4 py-2.5">Scopes</th>
                <th className="px-4 py-2.5">Created</th>
                <th className="px-4 py-2.5">Last used</th>
                <th className="px-4 py-2.5">State</th>
                {isAdmin ? <th className="px-4 py-2.5" /> : null}
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-100">
              {list.data.map((k) => (
                <tr key={k.id}>
                  <td className="px-4 py-3">
                    <p className="font-medium text-zinc-900">
                      {k.label || "Unnamed"}
                    </p>
                    <p className="font-mono text-[11px] text-zinc-400">
                      {k.id}
                    </p>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-zinc-600">
                    {k.scopes?.join(", ") || "–"}
                  </td>
                  <td className="px-4 py-3 text-zinc-500">
                    {fmtDate(k.created_at)}
                  </td>
                  <td className="px-4 py-3 text-zinc-500">
                    {fmtDate(k.last_used_at)}
                  </td>
                  <td className="px-4 py-3">
                    <StatusPill value={keyState(k)} />
                  </td>
                  {isAdmin ? (
                    <td className="px-4 py-3">
                      <div className="flex justify-end gap-1.5">
                        <button
                          type="button"
                          title="Rotate key"
                          disabled={Boolean(k.revoked_at) || rotate.isPending}
                          onClick={() => rotate.mutate(k.id)}
                          className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-zinc-600 ring-1 ring-inset ring-zinc-300 transition-colors hover:bg-zinc-100 disabled:opacity-40"
                        >
                          <RotateIcon />
                          Rotate
                        </button>
                        <button
                          type="button"
                          title="Revoke key"
                          disabled={Boolean(k.revoked_at) || revoke.isPending}
                          onClick={() => revoke.mutate(k.id)}
                          className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-red-700 ring-1 ring-inset ring-red-600/25 transition-colors hover:bg-red-50 disabled:opacity-40"
                        >
                          <BanIcon />
                          Revoke
                        </button>
                      </div>
                    </td>
                  ) : null}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}

      {actionError ? (
        <p role="alert" className="mt-2 text-sm text-red-700">
          {actionError.message}
        </p>
      ) : null}

      <NewKeyDialog
        clientId={clientId}
        open={newOpen}
        onClose={() => setNewOpen(false)}
        onSecret={(key) => {
          invalidate();
          setSecret({ key, heading: "New API key" });
        }}
      />
      <KeySecretDialog secret={secret} onClose={() => setSecret(null)} />
    </section>
  );
}
