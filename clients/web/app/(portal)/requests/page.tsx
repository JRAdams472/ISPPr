import { PageHeader } from "@/components/page-header";

export default function Requests() {
  return (
    <>
      <PageHeader
        title="Requests"
        description="Generation requests, job lifecycle, and webhook delivery."
      />
      <div className="rounded-lg border border-dashed border-zinc-300 bg-white p-10 text-center text-sm text-zinc-500">
        Generation requests and webhook delivery will be inspected here.
      </div>
    </>
  );
}
