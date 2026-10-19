import { PageHeader } from "@/components/page-header";

export default function Templates() {
  return (
    <>
      <PageHeader
        title="Templates"
        description="Template masters, versions, and customer variants."
      />
      <div className="rounded-lg border border-dashed border-zinc-300 bg-white p-10 text-center text-sm text-zinc-500">
        Template masters, versions, and variants will be managed here.
      </div>
    </>
  );
}
