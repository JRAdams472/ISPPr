"use client";

import { useRef } from "react";
import type { PageMargins, PageSetup } from "@/lib/templates/page-setup";
import { SIZE_GROUPS } from "@/lib/templates/page-setup";

// Page-setup panel (MPE-52): authors defaults.page — size, orientation,
// margins (in), and the header/footer HTML strips sent to Gotenberg.
// Token chips insert Chromium's pageNumber/totalPages spans, which are
// the only way to get incrementing numbers in print headers.
//
// Chromium print headers ignore external CSS — the strips must be
// inline-styled; the panel says so next to the fields.

const inputCls =
  "rounded-md border border-zinc-300 bg-white px-2 py-1 text-xs text-zinc-900 focus:border-teal-700 focus:outline-none focus:ring-2 focus:ring-teal-700/20";

const MARGIN_SIDES: { key: keyof PageMargins; label: string }[] = [
  { key: "top", label: "T" },
  { key: "right", label: "R" },
  { key: "bottom", label: "B" },
  { key: "left", label: "L" },
];

const TOKENS = [
  { label: "[page #]", html: '<span class="pageNumber"></span>' },
  { label: "[total]", html: '<span class="totalPages"></span>' },
] as const;

function StripEditor({
  id,
  label,
  value,
  readOnly,
  onChange,
}: {
  id: string;
  label: string;
  value: string;
  readOnly: boolean;
  onChange: (v: string) => void;
}) {
  const ref = useRef<HTMLTextAreaElement>(null);
  const insert = (html: string) => {
    const el = ref.current;
    if (!el) return;
    const { selectionStart: s, selectionEnd: e } = el;
    const next = value.slice(0, s) + html + value.slice(e);
    onChange(next);
    requestAnimationFrame(() => {
      el.focus();
      el.setSelectionRange(s + html.length, s + html.length);
    });
  };
  return (
    <div>
      <div className="mb-1 flex items-center justify-between">
        <label htmlFor={id} className="text-xs font-medium text-zinc-600">
          {label}
        </label>
        <div className="flex gap-1">
          {TOKENS.map((t) => (
            <button
              key={t.label}
              type="button"
              disabled={readOnly}
              title={`Insert ${t.html}`}
              onClick={() => insert(t.html)}
              className="rounded border border-zinc-300 px-1.5 py-0.5 font-mono text-[10px] text-zinc-600 hover:border-teal-700 hover:text-teal-800 disabled:opacity-50"
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>
      <textarea
        id={id}
        ref={ref}
        rows={3}
        value={value}
        readOnly={readOnly}
        onChange={(e) => onChange(e.target.value)}
        placeholder="<div style='font-size:9px;text-align:center;width:100%'>…</div>"
        className={`${inputCls} w-full font-mono leading-4`}
      />
    </div>
  );
}

export function PageSetupPanel({
  value,
  readOnly,
  error,
  onChange,
}: {
  value: PageSetup;
  readOnly: boolean;
  error?: string;
  onChange: (p: PageSetup) => void;
}) {
  const set = (patch: Partial<PageSetup>) => onChange({ ...value, ...patch });
  const setMargin = (side: keyof PageMargins, v: string) => {
    const n = parseFloat(v);
    set({
      margins: {
        ...value.margins,
        [side]: v === "" || !Number.isFinite(n) ? undefined : Math.max(0, n),
      },
    });
  };

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label
            htmlFor="page-size"
            className="mb-1 block text-xs font-medium text-zinc-600"
          >
            Paper size
          </label>
          <select
            id="page-size"
            value={value.size}
            disabled={readOnly}
            onChange={(e) => set({ size: e.target.value })}
            className={`${inputCls} w-full`}
          >
            {SIZE_GROUPS.map((g) => (
              <optgroup key={g.label} label={g.label}>
                {g.sizes.map((s) => (
                  <option key={s.value} value={s.value}>
                    {s.label}
                  </option>
                ))}
              </optgroup>
            ))}
          </select>
        </div>
        <div>
          <p className="mb-1 text-xs font-medium text-zinc-600">Orientation</p>
          <div
            role="radiogroup"
            aria-label="Orientation"
            className="flex rounded-md border border-zinc-200 bg-white p-0.5 text-xs"
          >
            {(["portrait", "landscape"] as const).map((o) => (
              <button
                key={o}
                role="radio"
                aria-checked={value.orientation === o}
                disabled={readOnly}
                onClick={() => set({ orientation: o })}
                className={`flex-1 rounded px-2 py-1 font-medium capitalize transition-colors disabled:opacity-60 ${
                  value.orientation === o
                    ? "bg-zinc-900 text-white"
                    : "text-zinc-500 hover:text-zinc-800"
                }`}
              >
                {o}
              </button>
            ))}
          </div>
        </div>
      </div>

      <div>
        <p className="mb-1 text-xs font-medium text-zinc-600">
          Margins (inches — blank = Chromium default)
        </p>
        <div className="grid grid-cols-4 gap-2">
          {MARGIN_SIDES.map(({ key, label }) => (
            <label key={key} className="block">
              <span className="mb-0.5 block text-[10px] font-medium uppercase tracking-wide text-zinc-500">
                {label}
              </span>
              <input
                type="number"
                min={0}
                step={0.1}
                aria-label={`${key} margin`}
                value={value.margins?.[key] ?? ""}
                disabled={readOnly}
                onChange={(e) => setMargin(key, e.target.value)}
                className={`${inputCls} w-full text-right`}
              />
            </label>
          ))}
        </div>
      </div>

      <StripEditor
        id="page-header"
        label="Header HTML"
        value={value.header_html ?? ""}
        readOnly={readOnly}
        onChange={(v) => set({ header_html: v })}
      />
      <StripEditor
        id="page-footer"
        label="Footer HTML"
        value={value.footer_html ?? ""}
        readOnly={readOnly}
        onChange={(v) => set({ footer_html: v })}
      />

      <p className="text-[11px] leading-4 text-zinc-400">
        Print headers/footers ignore external CSS — use inline styles, and
        leave margin room (top/bottom) for them.{" {{…}} "}bindings resolve
        with the same data as the body.
      </p>

      {error ? (
        <p role="alert" className="rounded-md border border-red-200 bg-red-50 px-2.5 py-1.5 text-xs text-red-800">
          {error}
        </p>
      ) : null}
    </div>
  );
}
