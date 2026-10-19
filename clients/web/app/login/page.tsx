"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { ApiError, setToken } from "@/lib/api/client";
import { useLogin } from "@/lib/queries";

export default function Login() {
  const router = useRouter();
  const login = useLogin();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const res = await login.mutateAsync({ email, password });
      setToken(res.token);
      router.replace("/");
    } catch {
      // surfaced via login.error below
    }
  };

  const error =
    login.error instanceof ApiError
      ? login.error.message
      : login.error
        ? "Sign in failed. Try again."
        : null;

  return (
    <main className="flex min-h-[100dvh] items-center justify-center bg-stone-50 px-4">
      <div className="w-full max-w-sm rounded-lg border border-zinc-200 bg-white p-8 shadow-sm">
        <div className="mb-6 flex items-center gap-2.5">
          <span className="flex h-8 w-8 items-center justify-center rounded-md bg-teal-600 text-xs font-bold text-white">
            IS
          </span>
          <div className="leading-tight">
            <p className="text-sm font-semibold text-zinc-900">ISPPr</p>
            <p className="text-xs text-zinc-500">admin portal</p>
          </div>
        </div>

        <h1 className="text-lg font-semibold tracking-tight text-zinc-900">
          Sign in
        </h1>
        <p className="mt-1 text-sm text-zinc-500">
          Use your admin account to manage the engine.
        </p>

        <form onSubmit={onSubmit} className="mt-6 space-y-4">
          <div className="space-y-2">
            <label
              htmlFor="email"
              className="block text-sm font-medium text-zinc-700"
            >
              Email
            </label>
            <input
              id="email"
              type="email"
              required
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="w-full rounded-md border border-zinc-300 px-3 py-2 text-sm text-zinc-900 outline-none focus:border-teal-600 focus:ring-1 focus:ring-teal-600"
            />
          </div>

          <div className="space-y-2">
            <label
              htmlFor="password"
              className="block text-sm font-medium text-zinc-700"
            >
              Password
            </label>
            <input
              id="password"
              type="password"
              required
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full rounded-md border border-zinc-300 px-3 py-2 text-sm text-zinc-900 outline-none focus:border-teal-600 focus:ring-1 focus:ring-teal-600"
            />
          </div>

          {error ? (
            <p role="alert" className="text-sm text-red-700">
              {error}
            </p>
          ) : null}

          <button
            type="submit"
            disabled={login.isPending}
            className="w-full rounded-md bg-teal-700 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-teal-800 disabled:opacity-60"
          >
            {login.isPending ? "Signing in…" : "Sign in"}
          </button>
        </form>
      </div>
    </main>
  );
}
