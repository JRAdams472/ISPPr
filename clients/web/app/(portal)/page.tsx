import { PageHeader } from "@/components/page-header";

export default function Dashboard() {
  return (
    <>
      <PageHeader
        title="Dashboard"
        description="Generation activity across tenants and packages."
      />
      <div className="rounded-lg border border-dashed border-zinc-300 bg-white p-10 text-center text-sm text-zinc-500">
        Request volume and job status will be charted here.
      </div>
    </>
  );
}
