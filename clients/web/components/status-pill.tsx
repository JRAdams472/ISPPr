// Status pill shared across portal tables. One accent family: teal means
// healthy/live, amber means in-flight, red means failed, zinc means off.

const TONES: Record<string, string> = {
  active: "bg-teal-50 text-teal-800 ring-teal-600/25",
  enabled: "bg-teal-50 text-teal-800 ring-teal-600/25",
  COMPLETED: "bg-teal-50 text-teal-800 ring-teal-600/25",
  DELIVERED: "bg-teal-50 text-teal-800 ring-teal-600/25",
  QUEUED: "bg-amber-50 text-amber-800 ring-amber-600/25",
  RUNNING: "bg-amber-50 text-amber-800 ring-amber-600/25",
  CLAIMED: "bg-amber-50 text-amber-800 ring-amber-600/25",
  PENDING: "bg-amber-50 text-amber-800 ring-amber-600/25",
  FAILED: "bg-red-50 text-red-700 ring-red-600/20",
  EXHAUSTED: "bg-red-50 text-red-700 ring-red-600/20",
};

export function StatusPill({ value }: { value: string }) {
  const tone =
    TONES[value] ?? "bg-zinc-100 text-zinc-600 ring-zinc-500/20";
  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium ring-1 ring-inset ${tone}`}
    >
      {value.toLowerCase()}
    </span>
  );
}
