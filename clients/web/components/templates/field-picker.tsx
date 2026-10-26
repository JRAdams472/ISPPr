"use client";

import type { Editor } from "@tiptap/react";
import { useEffect, useMemo, useRef, useState } from "react";
import { fieldPaths } from "@/lib/templates/serialize";

// Merge-field picker (MPE-51): paths flattened from the preview sample
// JSON plus free-form entry for helpers ({{upper name}}) or paths not in
// the sample. Array items list as items[].field.
export function FieldPicker({
  editor,
  sampleJson,
  onClose,
}: {
  editor: Editor;
  sampleJson: string;
  onClose: () => void;
}) {
  const [manual, setManual] = useState("");
  const ref = useRef<HTMLDivElement>(null);

  const paths = useMemo(() => {
    try {
      return fieldPaths(JSON.parse(sampleJson));
    } catch {
      return [];
    }
  }, [sampleJson]);

  useEffect(() => {
    const onDoc = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [onClose]);

  const insert = (expr: string) => {
    const e = expr.trim();
    if (!e) return;
    editor.chain().focus().insertMergeField(e).run();
    onClose();
  };

  return (
    <div
      ref={ref}
      role="dialog"
      aria-label="Insert merge field"
      className="absolute z-20 w-72 rounded-lg border border-zinc-200 bg-white p-2 shadow-lg shadow-zinc-900/10"
    >
      <p className="px-1.5 pb-1 text-[11px] font-medium uppercase tracking-wide text-zinc-500">
        Merge fields
      </p>
      {paths.length ? (
        <ul className="max-h-44 overflow-y-auto">
          {paths.map((p) => (
            <li key={p}>
              <button
                type="button"
                onClick={() => insert(p)}
                className="flex w-full items-center rounded px-1.5 py-1 text-left font-mono text-xs text-zinc-700 hover:bg-teal-800/10 hover:text-teal-900"
              >
                {`{{${p}}}`}
              </button>
            </li>
          ))}
        </ul>
      ) : (
        <p className="px-1.5 py-1 text-xs text-zinc-500">
          No fields in the sample data — enter one below.
        </p>
      )}
      <form
        className="mt-1.5 flex gap-1.5 border-t border-zinc-100 pt-2"
        onSubmit={(e) => {
          e.preventDefault();
          insert(manual);
        }}
      >
        <input
          autoFocus
          value={manual}
          onChange={(e) => setManual(e.target.value)}
          placeholder="doc.total or helper expr"
          aria-label="Field path"
          className="min-w-0 flex-1 rounded border border-zinc-300 px-2 py-1 font-mono text-xs focus:border-teal-700 focus:outline-none"
        />
        <button
          type="submit"
          disabled={!manual.trim()}
          className="rounded bg-zinc-900 px-2 py-1 text-xs font-medium text-white disabled:opacity-50"
        >
          Insert
        </button>
      </form>
    </div>
  );
}
