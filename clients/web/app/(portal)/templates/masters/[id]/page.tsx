"use client";

import dynamic from "next/dynamic";
import { useParams } from "next/navigation";
import { PageHeader } from "@/components/page-header";
import { CardSkeleton, ErrorState } from "@/components/states";
import { useMaster, useMe } from "@/lib/queries";

const TemplateEditor = dynamic(
  () =>
    import("@/components/templates/template-editor").then(
      (m) => m.TemplateEditor,
    ),
  { ssr: false, loading: () => <CardSkeleton lines={6} /> },
);

// Edit a master (MPE-51): the editor loads the latest content and
// publishes the next version on save — masters are append-only.
export default function MasterEditorPage() {
  const { id } = useParams<{ id: string }>();
  const master = useMaster(id);
  const me = useMe();

  return (
    <>
      <PageHeader
        title="Edit template"
        description="Publish a new version of this master — existing versions stay immutable."
      />
      {master.isLoading || !me.isFetched ? (
        <CardSkeleton lines={6} />
      ) : master.isError || !master.data ? (
        <ErrorState
          message={master.error?.message ?? "Template master not found."}
          onRetry={() => void master.refetch()}
        />
      ) : (
        <TemplateEditor
          clientId={master.data.client_id}
          master={master.data}
          isAdmin={me.data?.role === "admin"}
        />
      )}
    </>
  );
}
