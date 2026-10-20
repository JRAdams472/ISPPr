"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useParams } from "next/navigation";
import { ApiKeysCard } from "@/components/clients/api-keys-card";
import { CustomersCard } from "@/components/clients/customers-card";
import { FailedRequests } from "@/components/clients/failed-requests";
import { UsagePanel } from "@/components/clients/usage-panel";
import { PageHeader } from "@/components/page-header";
import { CardSkeleton, ErrorState } from "@/components/states";
import { StatusPill } from "@/components/status-pill";
import { clients } from "@/lib/api/resources";
import { keys, useClient, useMe } from "@/lib/queries";

// /clients/[id] — single-tenant operations view: keys lifecycle, customer
// roster, request volume/latency, and recent failures.

export default function ClientDetail() {
  const params = useParams();
  const id = typeof params.id === "string" ? params.id : "";
  const qc = useQueryClient();
  const me = useMe();
  const client = useClient(id);
  const isAdmin = me.data?.role === "admin";

  const setStatus = useMutation({
    mutationFn: (status: string) => clients.update(id, { status }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: keys.client(id) });
      void qc.invalidateQueries({ queryKey: keys.clients });
    },
  });

  if (client.isLoading) {
    return (
      <>
        <PageHeader title="Client" />
        <div className="space-y-6">
          <CardSkeleton lines={4} />
          <CardSkeleton lines={4} />
        </div>
      </>
    );
  }

  if (client.isError || !client.data) {
    return (
      <>
        <PageHeader title="Client" />
        <ErrorState
          message={client.error?.message ?? "Client not found."}
          onRetry={() => void client.refetch()}
        />
      </>
    );
  }

  const c = client.data;
  const active = c.status === "active";

  return (
    <>
      <PageHeader
        title={c.name}
        description=""
        actions={
          isAdmin ? (
            <button
              type="button"
              onClick={() => setStatus.mutate(active ? "disabled" : "active")}
              disabled={setStatus.isPending}
              className="rounded-md px-3 py-2 text-sm font-medium text-zinc-700 ring-1 ring-inset ring-zinc-300 transition-colors hover:bg-zinc-100 disabled:opacity-60"
            >
              {active ? "Disable client" : "Enable client"}
            </button>
          ) : null
        }
      />

      <div className="mb-6 -mt-4 flex items-center gap-3 text-sm text-zinc-500">
        <StatusPill value={c.status} />
        <span className="font-mono text-xs">{c.id}</span>
        <Link href="/clients" className="text-teal-800 hover:underline">
          All clients
        </Link>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-5">
        <div className="space-y-6 lg:col-span-3">
          <ApiKeysCard clientId={id} isAdmin={isAdmin} />
          <CustomersCard clientId={id} isAdmin={isAdmin} />
        </div>
        <div className="lg:col-span-2">
          <UsagePanel clientId={id} />
        </div>
      </div>

      <div className="mt-6">
        <FailedRequests clientId={id} />
      </div>
    </>
  );
}
