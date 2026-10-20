// Shared surface states: loading skeletons, empty states, error states.
// Skeletons match the final layout's shape so the swap doesn't jump.

export function TableSkeleton({ cols = 4, rows = 4 }: { cols?: number; rows?: number }) {
  return (
    <div
      aria-busy="true"
      aria-label="Loading"
      className="divide-y divide-zinc-100 rounded-lg border border-zinc-200 bg-white"
    >
      {Array.from({ length: rows }).map((_, r) => (
        <div key={r} className="flex items-center gap-6 px-4 py-3.5">
          {Array.from({ length: cols }).map((_, c) => (
            <div
              key={c}
              className={`h-3.5 animate-pulse rounded bg-zinc-200 ${
                c === 0 ? "w-40" : "w-20"
              }`}
            />
          ))}
        </div>
      ))}
    </div>
  );
}

export function CardSkeleton({ lines = 3 }: { lines?: number }) {
  return (
    <div
      aria-busy="true"
      aria-label="Loading"
      className="space-y-3 rounded-lg border border-zinc-200 bg-white p-5"
    >
      {Array.from({ length: lines }).map((_, i) => (
        <div
          key={i}
          className={`h-3.5 animate-pulse rounded bg-zinc-200 ${
            i === 0 ? "w-1/3" : "w-2/3"
          }`}
        />
      ))}
    </div>
  );
}

export function EmptyState({
  title,
  hint,
  action,
}: {
  title: string;
  hint?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="rounded-lg border border-dashed border-zinc-300 bg-white px-6 py-10 text-center">
      <p className="text-sm font-medium text-zinc-700">{title}</p>
      {hint ? <p className="mt-1 text-sm text-zinc-500">{hint}</p> : null}
      {action ? <div className="mt-4">{action}</div> : null}
    </div>
  );
}

export function ErrorState({
  message,
  onRetry,
}: {
  message: string;
  onRetry?: () => void;
}) {
  return (
    <div
      role="alert"
      className="rounded-lg border border-red-200 bg-red-50 px-5 py-4"
    >
      <p className="text-sm font-medium text-red-800">
        Something went wrong
      </p>
      <p className="mt-0.5 text-sm text-red-700">{message}</p>
      {onRetry ? (
        <button
          type="button"
          onClick={onRetry}
          className="mt-2 text-sm font-medium text-red-800 underline underline-offset-2 hover:text-red-900"
        >
          Try again
        </button>
      ) : null}
    </div>
  );
}
