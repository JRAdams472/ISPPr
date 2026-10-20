"use client";

import { CardSkeleton, EmptyState, ErrorState } from "@/components/states";
import { useUsage } from "@/lib/queries";

// Per-client request volume + render latency. The chart is hand-rolled
// SVG — the portal doesn't need a chart library for daily counts.

const DAYS = 30;
const W = 480;
const H = 120;

interface DayBucket {
  day: string;
  requests: number;
  failed: number;
}

function bucketize(byDay: { day: string; requests: number; failed: number }[]) {
  const map = new Map<string, DayBucket>();
  for (const r of byDay) {
    const b = map.get(r.day) ?? { day: r.day, requests: 0, failed: 0 };
    b.requests += r.requests;
    b.failed += r.failed;
    map.set(r.day, b);
  }
  // Fill the trailing N days so the axis is continuous, oldest → newest.
  const out: DayBucket[] = [];
  const today = new Date();
  for (let i = DAYS - 1; i >= 0; i--) {
    const d = new Date(today.getTime() - i * 86400_000);
    const key = d.toISOString().slice(0, 10);
    out.push(map.get(key) ?? { day: key, requests: 0, failed: 0 });
  }
  return out;
}

function VolumeChart({ days }: { days: DayBucket[] }) {
  const max = Math.max(1, ...days.map((d) => d.requests));
  const bw = W / days.length;
  return (
    <svg
      viewBox={`0 0 ${W} ${H + 18}`}
      className="w-full"
      role="img"
      aria-label="Requests per day"
    >
      {days.map((d, i) => {
        const h = (d.requests / max) * H;
        const fh = (d.failed / max) * H;
        const x = i * bw;
        return (
          <g key={d.day}>
            <title>
              {d.day}: {d.requests} requests
              {d.failed ? `, ${d.failed} failed` : ""}
            </title>
            <rect
              x={x + 1.5}
              y={H - h}
              width={Math.max(1, bw - 3)}
              height={h}
              rx="1.5"
              className="fill-teal-600/80"
            />
            {fh > 0 ? (
              <rect
                x={x + 1.5}
                y={H - fh}
                width={Math.max(1, bw - 3)}
                height={fh}
                rx="1.5"
                className="fill-red-500"
              />
            ) : null}
          </g>
        );
      })}
      <line
        x1="0"
        y1={H + 0.5}
        x2={W}
        y2={H + 0.5}
        className="stroke-zinc-200"
        strokeWidth="1"
      />
      <text x="0" y={H + 14} className="fill-zinc-400" fontSize="9">
        {days[0]?.day.slice(5)}
      </text>
      <text
        x={W}
        y={H + 14}
        textAnchor="end"
        className="fill-zinc-400"
        fontSize="9"
      >
        today
      </text>
    </svg>
  );
}

export function UsagePanel({ clientId }: { clientId: string }) {
  const usage = useUsage(DAYS, clientId);

  if (usage.isLoading) return <CardSkeleton lines={5} />;
  if (usage.isError) {
    return (
      <ErrorState
        message={usage.error.message}
        onRetry={() => void usage.refetch()}
      />
    );
  }
  if (!usage.data) return null;

  const lat = usage.data.latency;
  const days = bucketize(usage.data.by_day);
  const total = days.reduce((s, d) => s + d.requests, 0);

  if (total === 0 && lat.samples === 0) {
    return (
      <EmptyState
        title="No requests in the last 30 days"
        hint="Volume and latency appear once this client generates documents."
      />
    );
  }

  return (
    <div className="rounded-lg border border-zinc-200 bg-white p-5">
      <div className="flex items-baseline justify-between">
        <h2 className="text-sm font-semibold text-zinc-900">
          Requests, last {usage.data.days} days
        </h2>
        <p className="text-sm tabular-nums text-zinc-500">
          {total.toLocaleString()} total
        </p>
      </div>
      <div className="mt-4">
        <VolumeChart days={days} />
      </div>
      <div className="mt-4 grid grid-cols-3 gap-3 border-t border-zinc-100 pt-4">
        <div>
          <p className="text-[11px] font-medium uppercase tracking-wide text-zinc-500">
            p50 latency
          </p>
          <p className="mt-0.5 text-lg font-semibold tabular-nums text-zinc-900">
            {lat.p50_ms.toFixed(0)}
            <span className="ml-0.5 text-xs font-normal text-zinc-500">ms</span>
          </p>
        </div>
        <div>
          <p className="text-[11px] font-medium uppercase tracking-wide text-zinc-500">
            p95 latency
          </p>
          <p className="mt-0.5 text-lg font-semibold tabular-nums text-zinc-900">
            {lat.p95_ms.toFixed(0)}
            <span className="ml-0.5 text-xs font-normal text-zinc-500">ms</span>
          </p>
        </div>
        <div>
          <p className="text-[11px] font-medium uppercase tracking-wide text-zinc-500">
            Samples
          </p>
          <p className="mt-0.5 text-lg font-semibold tabular-nums text-zinc-900">
            {lat.samples.toLocaleString()}
          </p>
        </div>
      </div>
    </div>
  );
}
