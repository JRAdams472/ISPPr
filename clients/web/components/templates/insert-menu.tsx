"use client";

import type { Editor } from "@tiptap/react";
import { useEffect, useRef, useState } from "react";

// Insert menu (MPE-53): every print construct the canvas round-trips —
// conditional region, data-bound table, page/sheet breaks, branding
// slots, merge fields. Forms stay tiny; anything the canvas can't
// represent stays in source mode by design.

const itemCls =
  "block w-full rounded px-2 py-1.5 text-left text-xs text-zinc-700 hover:bg-teal-800/10 hover:text-teal-900";

const inputCls =
  "w-full rounded border border-zinc-300 px-2 py-1 font-mono text-xs focus:border-teal-700 focus:outline-none";

function Panel({
  title,
  children,
  onBack,
}: {
  title: string;
  children: React.ReactNode;
  onBack: () => void;
}) {
  return (
    <div className="p-2">
      <button
        type="button"
        onClick={onBack}
        className="mb-1 text-[11px] font-medium text-zinc-400 hover:text-zinc-700"
      >
        ← {title}
      </button>
      {children}
    </div>
  );
}

export function InsertMenu({
  editor,
  brandingKeys,
  onClose,
  onPickField,
}: {
  editor: Editor;
  brandingKeys: string[];
  onClose: () => void;
  onPickField: () => void;
}) {
  const [view, setView] = useState<"menu" | "cond" | "table" | "brand">("menu");
  const [expr, setExpr] = useState("blocks.");
  const [path, setPath] = useState("items");
  const [cols, setCols] = useState("Item=title\nPrice=price");
  const [repeat, setRepeat] = useState(true);
  const [brand, setBrand] = useState(brandingKeys[0] ?? "");
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const onDoc = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [onClose]);

  const insertBreak = (kind: "page" | "recto") => {
    editor.chain().focus().insertContent({ type: "pageBreak", attrs: { kind } }).run();
    onClose();
  };

  const insertCond = () => {
    const e = expr.trim();
    if (!e) return;
    editor
      .chain()
      .focus()
      .insertContent({
        type: "conditionalRegion",
        attrs: { expr: e },
        content: [{ type: "paragraph" }],
      })
      .run();
    onClose();
  };

  const insertTable = () => {
    const p = path.trim();
    const parsed = cols
      .split("\n")
      .map((l) => l.trim())
      .filter(Boolean)
      .map((l) => {
        const [label, expr] = l.split("=").map((s) => s.trim());
        return { label: label || expr, expr: expr || label };
      });
    if (!p || !parsed.length) return;
    editor
      .chain()
      .focus()
      .insertContent({
        type: "dataTable",
        attrs: { path: p, cols: parsed, repeatHeader: repeat },
      })
      .run();
    onClose();
  };

  return (
    <div
      ref={ref}
      role="dialog"
      aria-label="Insert"
      className="absolute z-20 w-64 rounded-lg border border-zinc-200 bg-white shadow-lg shadow-zinc-900/10"
    >
      {view === "menu" ? (
        <div className="p-2">
          <p className="px-1.5 pb-1 text-[11px] font-medium uppercase tracking-wide text-zinc-500">
            Insert
          </p>
          <button type="button" className={itemCls} onClick={onPickField}>
            Merge field <span className="text-zinc-400">{"{{path}}"}</span>
          </button>
          <button type="button" className={itemCls} onClick={() => setView("brand")}>
            Branding slot{" "}
            <span className="text-zinc-400">{"{{branding.key}}"}</span>
          </button>
          <button type="button" className={itemCls} onClick={() => setView("cond")}>
            Conditional region{" "}
            <span className="text-zinc-400">{"{{#if}}"}</span>
          </button>
          <button type="button" className={itemCls} onClick={() => setView("table")}>
            Data table <span className="text-zinc-400">{"{{#each}}"}</span>
          </button>
          <button type="button" className={itemCls} onClick={() => insertBreak("page")}>
            Page break <span className="text-zinc-400">break-before:page</span>
          </button>
          <button type="button" className={itemCls} onClick={() => insertBreak("recto")}>
            New sheet <span className="text-zinc-400">break-before:recto</span>
          </button>
        </div>
      ) : null}

      {view === "cond" ? (
        <Panel title="Insert" onBack={() => setView("menu")}>
          <form
            className="space-y-2"
            onSubmit={(e) => {
              e.preventDefault();
              insertCond();
            }}
          >
            <p className="text-xs font-medium text-zinc-700">
              Conditional region
            </p>
            <input
              autoFocus
              value={expr}
              onChange={(e) => setExpr(e.target.value)}
              placeholder="blocks.promo or paid"
              aria-label="Condition expression"
              className={inputCls}
            />
            <p className="text-[11px] text-zinc-400">
              `blocks.name` flows into defaults.toggles — variants can flip
              it per customer.
            </p>
            <button
              type="submit"
              disabled={!expr.trim()}
              className="w-full rounded bg-zinc-900 px-2 py-1.5 text-xs font-medium text-white disabled:opacity-50"
            >
              Wrap selection / insert
            </button>
          </form>
        </Panel>
      ) : null}

      {view === "table" ? (
        <Panel title="Insert" onBack={() => setView("menu")}>
          <form
            className="space-y-2"
            onSubmit={(e) => {
              e.preventDefault();
              insertTable();
            }}
          >
            <p className="text-xs font-medium text-zinc-700">Data table</p>
            <input
              value={path}
              onChange={(e) => setPath(e.target.value)}
              placeholder="items"
              aria-label="Array path"
              className={inputCls}
            />
            <textarea
              rows={3}
              value={cols}
              onChange={(e) => setCols(e.target.value)}
              aria-label="Columns (label=expr per line)"
              className={`${inputCls} leading-4`}
            />
            <label className="flex items-center gap-2 text-xs text-zinc-700">
              <input
                type="checkbox"
                checked={repeat}
                onChange={(e) => setRepeat(e.target.checked)}
                className="h-3.5 w-3.5 accent-teal-800"
              />
              Repeat header on each page
            </label>
            <button
              type="submit"
              disabled={!path.trim()}
              className="w-full rounded bg-zinc-900 px-2 py-1.5 text-xs font-medium text-white disabled:opacity-50"
            >
              Insert table
            </button>
          </form>
        </Panel>
      ) : null}

      {view === "brand" ? (
        <Panel title="Insert" onBack={() => setView("menu")}>
          <form
            className="space-y-2"
            onSubmit={(e) => {
              e.preventDefault();
              const k = brand.trim();
              if (!k) return;
              editor
                .chain()
                .focus()
                .insertMergeField(`branding.${k}`)
                .run();
              onClose();
            }}
          >
            <p className="text-xs font-medium text-zinc-700">Branding slot</p>
            {brandingKeys.length ? (
              <select
                value={brand}
                onChange={(e) => setBrand(e.target.value)}
                aria-label="Branding key"
                className={inputCls}
              >
                {brandingKeys.map((k) => (
                  <option key={k} value={k}>
                    {k}
                  </option>
                ))}
              </select>
            ) : null}
            <input
              value={brand}
              onChange={(e) => setBrand(e.target.value)}
              placeholder="logo"
              aria-label="Branding key (manual)"
              className={inputCls}
            />
            <button
              type="submit"
              disabled={!brand.trim()}
              className="w-full rounded bg-zinc-900 px-2 py-1.5 text-xs font-medium text-white disabled:opacity-50"
            >
              Insert {`{{branding.${brand || "key"}}}`}
            </button>
          </form>
        </Panel>
      ) : null}
    </div>
  );
}
