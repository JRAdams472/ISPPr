"use client";

import dynamic from "next/dynamic";
import { useSearchParams } from "next/navigation";
import { Suspense, useState } from "react";
import { PageHeader } from "@/components/page-header";
import { CardSkeleton } from "@/components/states";
import { useClients, useMe } from "@/lib/queries";

const TemplateEditor = dynamic(
  () =>
    import("@/components/templates/template-editor").then(
      (m) => m.TemplateEditor,
    ),
  { ssr: false, loading: () => <CardSkeleton lines={6} /> },
);

// New master (MPE-51): pick the owning client (carried via ?client= from
// the templates list), then the editor creates v1 on save.
function NewTemplate() {
  const params = useSearchParams();
  const clients = useClients();
  const me = useMe();
  const [picked, setPicked] = useState(params.get("client") ?? "");
  const clientId = picked || clients.data?.[0]?.id || "";

  return (
    <>
      <PageHeader
        title="New template"
        description="Create a master template — the base every customer variant can restyle."
        actions={
          <select
            aria-label="Client"
            value={clientId}
            onChange={(e) => setPicked(e.target.value)}
            className="rounded-md border border-zinc-300 bg-white px-2.5 py-1.5 text-sm text-zinc-800 focus:border-teal-700 focus:outline-none focus:ring-2 focus:ring-teal-700/20"
          >
            {(clients.data ?? []).map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
        }
      />
      {clientId && me.isFetched ? (
        <TemplateEditor
          clientId={clientId}
          master={null}
          isAdmin={me.data?.role === "admin"}
        />
      ) : (
        <CardSkeleton lines={6} />
      )}
    </>
  );
}

export default function Page() {
  return (
    <Suspense fallback={<CardSkeleton lines={4} />}>
      <NewTemplate />
    </Suspense>
  );
}
