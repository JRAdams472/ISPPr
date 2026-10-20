"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { PageHeader } from "@/components/page-header";
import { VariantEditor } from "@/components/variants/variant-editor";
import { useCustomer } from "@/lib/queries";

// /clients/[id]/customers/[customerId]/variants/[masterId] — draft overlay
// editing with live Gotenberg preview (MPE-42).

export default function VariantEditorPage() {
  const params = useParams();
  const clientId = typeof params.id === "string" ? params.id : "";
  const customerId =
    typeof params.customerId === "string" ? params.customerId : "";
  const masterId = typeof params.masterId === "string" ? params.masterId : "";
  const customer = useCustomer(customerId);

  return (
    <>
      <PageHeader
        title={customer.data ? `${customer.data.name} — template override` : "Template override"}
        description=""
      />
      <div className="mb-6 -mt-4 flex items-center gap-3 text-sm text-zinc-500">
        <Link
          href={`/clients/${clientId}/customers/${customerId}`}
          className="text-teal-800 hover:underline"
        >
          {customer.data?.name ?? "Customer"} overrides
        </Link>
      </div>
      <VariantEditor
        clientId={clientId}
        customerId={customerId}
        masterId={masterId}
      />
    </>
  );
}
