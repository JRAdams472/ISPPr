import { PageHeader } from "@/components/page-header";

export default function Usage() {
  return (
    <>
      <PageHeader
        title="Usage"
        description="Request volume and render latency by tenant."
      />
      <div className="rounded-lg border border-dashed border-zinc-300 bg-white p-10 text-center text-sm text-zinc-500">
        Request volume and render latency will be reported here.
      </div>
    </>
  );
}
