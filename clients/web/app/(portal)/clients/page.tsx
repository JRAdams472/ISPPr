import { PageHeader } from "@/components/page-header";

export default function Clients() {
  return (
    <>
      <PageHeader
        title="Clients"
        description="Tenants, their customers, and API keys."
      />
      <div className="rounded-lg border border-dashed border-zinc-300 bg-white p-10 text-center text-sm text-zinc-500">
        Clients, customers, and API keys will be managed here.
      </div>
    </>
  );
}
