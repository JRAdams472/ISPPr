"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { ApiError, getToken, setToken } from "@/lib/api/client";
import { useMe } from "@/lib/queries";

// Client-side auth gate for the (portal) group: no stored token goes straight
// to /login; a stored-but-dead session (401 from /me) is cleared first.

export function RequireAuth({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const hasToken = getToken() !== "";
  const me = useMe(hasToken);

  useEffect(() => {
    if (!hasToken) router.replace("/login");
  }, [hasToken, router]);

  useEffect(() => {
    if (me.error instanceof ApiError && me.error.status === 401) {
      setToken("");
      router.replace("/login");
    }
  }, [me.error, router]);

  if (!hasToken) return null;
  if (me.isPending) {
    return (
      <div className="flex min-h-[100dvh] items-center justify-center bg-stone-50">
        <p className="text-sm text-zinc-500">Checking your session…</p>
      </div>
    );
  }
  if (me.isError && !(me.error instanceof ApiError && me.error.status === 401)) {
    return (
      <div className="flex min-h-[100dvh] items-center justify-center bg-stone-50">
        <p className="text-sm text-zinc-600">
          The admin API is unreachable. Try again shortly.
        </p>
      </div>
    );
  }
  if (me.isError) return null;
  return <>{children}</>;
}
