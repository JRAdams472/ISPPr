"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { NAV, navActive } from "@/lib/nav";

// Portal app shell: dark rail sidebar with brand + primary nav, light
// content area. Operate-mode surface — scannability first, decoration last.

export function Shell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();

  return (
    <div className="flex min-h-[100dvh]">
      <aside className="flex w-56 shrink-0 flex-col bg-zinc-950 text-zinc-300">
        <Link href="/" className="flex items-center gap-2.5 px-5 pt-6 pb-7">
          <span className="flex h-7 w-7 items-center justify-center rounded-md bg-teal-600 text-[11px] font-bold text-white">
            IS
          </span>
          <span className="leading-tight">
            <span className="block text-sm font-semibold text-white">
              ISPPr
            </span>
            <span className="block text-[11px] text-zinc-500">
              document engine
            </span>
          </span>
        </Link>

        <nav aria-label="Primary" className="flex-1 px-3">
          <ul className="space-y-0.5">
            {NAV.map((item) => {
              const active = navActive(pathname, item.href);
              return (
                <li key={item.href}>
                  <Link
                    href={item.href}
                    aria-current={active ? "page" : undefined}
                    className={`flex items-center rounded-md px-3 py-2 text-sm transition-colors ${
                      active
                        ? "bg-zinc-800 font-medium text-white shadow-[inset_2px_0_0_0_var(--color-teal-500)]"
                        : "text-zinc-400 hover:bg-zinc-900 hover:text-zinc-100"
                    }`}
                  >
                    {item.label}
                  </Link>
                </li>
              );
            })}
          </ul>
        </nav>

        <footer className="border-t border-zinc-900 px-5 py-4 text-[11px] text-zinc-500">
          Admin portal
        </footer>
      </aside>

      <main className="min-w-0 flex-1 bg-stone-50 text-zinc-900">
        <div className="mx-auto max-w-6xl px-8 py-8">{children}</div>
      </main>
    </div>
  );
}
