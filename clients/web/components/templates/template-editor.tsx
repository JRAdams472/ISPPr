"use client";

import { css as cssLang } from "@codemirror/lang-css";
import { html as htmlLang } from "@codemirror/lang-html";
import { json } from "@codemirror/lang-json";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import dynamic from "next/dynamic";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { EditorContent, useEditor } from "@tiptap/react";
import { ApiError } from "@/lib/api/client";
import { masters } from "@/lib/api/resources";
import type { TemplateMaster } from "@/lib/api/types";
import { keys } from "@/lib/queries";
import {
  MM_PER_INCH,
  extPaperSize,
  pagePayload,
  printPayload,
  readPage,
  type PageSetup,
} from "@/lib/templates/page-setup";
import { exportHTML, prepareForEditor, residualAdvanced, togglesFromHtml } from "@/lib/templates/serialize";
import { editorExtensions } from "./extensions";
import { FieldPicker } from "./field-picker";
import { InsertMenu } from "./insert-menu";
import { PageSetupPanel } from "./page-setup";
import { EditorToolbar } from "./toolbar";

// Template master editor (MPE-51): dual-mode canvas — a paged TipTap
// visual editor and a CodeMirror source view — plus a live Gotenberg
// preview of the unsaved draft via /admin/template-masters/preview.
// Visual mode handles the representable subset of Handlebars (inline
// {{expr}} bindings become chips); block helpers pin the document to
// source mode rather than being mangled.

const CodeMirror = dynamic(() => import("@uiw/react-codemirror"), {
  ssr: false,
});

const NAME_RE = /^[a-z0-9][a-z0-9_-]*$/;

const editorBox =
  "overflow-hidden rounded-md border border-zinc-300 bg-white text-xs [&_.cm-editor]:text-xs [&_.cm-editor]:bg-white";

type Mode = "visual" | "source";

export function TemplateEditor({
  clientId,
  master,
  isAdmin,
  paginate = true,
}: {
  clientId: string;
  master: TemplateMaster | null; // null → create a new master
  isAdmin: boolean;
  /** Paged canvas — off in jsdom tests (no layout engine). */
  paginate?: boolean;
}) {
  const router = useRouter();
  const qc = useQueryClient();
  const editing = master !== null;

  const initialAdvanced = useMemo(
    () => residualAdvanced(master?.html ?? ""),
    [master],
  );
  const [mode, setMode] = useState<Mode>(
    initialAdvanced.length ? "source" : "visual",
  );
  const [pinned, setPinned] = useState(initialAdvanced);
  const [name, setName] = useState(master?.name ?? "");
  const [cssValue, setCssValue] = useState(master?.css ?? "");
  const [sourceHtml, setSourceHtml] = useState(master?.html ?? "");
  const [sourceCss, setSourceCss] = useState(master?.css ?? "");
  const [sampleJson, setSampleJson] = useState(
    '{\n  "doc_title": "Sample document",\n  "due": "2026-03-01"\n}',
  );
  const [dataErr, setDataErr] = useState("");
  const [pdfUrl, setPdfUrl] = useState<string | null>(null);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const [pane, setPane] = useState<"preview" | "setup">("preview");
  const [page, setPage] = useState<PageSetup>(() =>
    readPage(master?.defaults),
  );

  const editor = useEditor({
    extensions: editorExtensions({ paginate }),
    content: prepareForEditor(master?.html ?? "<p></p>", { wrap: paginate }),
    editable: isAdmin,
    immediatelyRender: false,
  });

  // Mirror page setup onto the paged canvas (visual only — the PDF
  // layout is owned by defaults.page on the Gotenberg side).
  useEffect(() => {
    if (!editor || !paginate) return;
    const m = page.margins ?? {};
    editor.commands.setDocumentPaperSize(
      extPaperSize(page.size) as never,
    );
    editor.commands.setDocumentPaperOrientation(
      page.orientation ?? "portrait",
    );
    editor.commands.setDocumentPageMargins({
      top: (m.top ?? 1) * MM_PER_INCH,
      right: (m.right ?? 1) * MM_PER_INCH,
      bottom: (m.bottom ?? 1) * MM_PER_INCH,
      left: (m.left ?? 1) * MM_PER_INCH,
    });
  }, [editor, paginate, page]);

  const currentHtml = () => {
    if (mode === "source") return sourceHtml;
    return editor ? exportHTML(editor.getHTML()) : "";
  };

  const defaultsPayload = () => {
    const p = pagePayload(page);
    const base = { ...(master?.defaults ?? {}) };
    if (Object.keys(p).length) base.page = p;
    // defaults.print owns the Ghostscript post-process flag; dropping the
    // key when off beats persisting a stale enabled:true.
    const pr = printPayload(page.print);
    if (Object.keys(pr).length) base.print = pr;
    else delete base.print;
    // {{#if blocks.X}} regions flow into defaults.toggles so customer
    // variants can flip them; existing values are preserved.
    const tg = togglesFromHtml(currentHtml());
    if (tg.length) {
      const existing = (base.toggles ?? {}) as Record<string, boolean>;
      base.toggles = Object.fromEntries(
        tg.map((t) => [t, existing[t] ?? true]),
      );
    }
    return base;
  };

  const toSource = () => {
    if (editor) setSourceHtml(exportHTML(editor.getHTML()));
    setSourceCss(cssValue);
    setMode("source");
  };

  const toVisual = () => {
    const bad = residualAdvanced(sourceHtml);
    if (bad.length) {
      setPinned(bad);
      return;
    }
    setPinned([]);
    setCssValue(sourceCss);
    editor?.commands.setContent(prepareForEditor(sourceHtml));
    setMode("visual");
  };

  const save = useMutation({
    mutationFn: () =>
      editing
        ? masters.publish(clientId, master.name, {
            html: currentHtml(),
            css: mode === "source" ? sourceCss : cssValue,
            defaults: defaultsPayload(),
          })
        : masters.create(clientId, {
            name: name.trim(),
            html: currentHtml(),
            css: mode === "source" ? sourceCss : cssValue,
            defaults: defaultsPayload(),
          }),
    onSuccess: (m) => {
      void qc.invalidateQueries({ queryKey: keys.masters(clientId) });
      void qc.invalidateQueries({ queryKey: keys.master(m.id) });
      if (!editing) router.push(`/templates/masters/${m.id}`);
    },
  });

  const preview = useMutation({
    mutationFn: () =>
      masters.preview({
        html: currentHtml(),
        css: mode === "source" ? sourceCss : cssValue,
        defaults: defaultsPayload(),
        data: JSON.parse(sampleJson) as Record<string, unknown>,
      }),
    onSuccess: (blob) => {
      setPdfUrl((old) => {
        if (old) URL.revokeObjectURL(old);
        return URL.createObjectURL(blob);
      });
    },
  });

  const nameValid = editing || NAME_RE.test(name.trim());
  const canSave = isAdmin && nameValid && !save.isPending;

  // Surface page-block problems (bad size, uncompilable header/footer)
  // inside the panel, next to the fields that caused them.
  const pageErr = useMemo(() => {
    const err = save.error ?? preview.error;
    const detail = err instanceof ApiError ? (err.problem.detail ?? err.message) : err?.message;
    return detail && /page|margin|size|orientation|header|footer/i.test(detail)
      ? detail
      : undefined;
  }, [save.error, preview.error]);

  return (
    <div className="grid grid-cols-1 gap-6 xl:grid-cols-[1fr_380px]">
      <div className="min-w-0 space-y-4">
        <div className="flex flex-wrap items-center gap-3">
          {editing ? (
            <h2 className="font-mono text-sm font-semibold text-zinc-900">
              {master.name}
              <span className="ml-2 rounded bg-zinc-100 px-1.5 py-0.5 text-[11px] font-medium text-zinc-500">
                v{master.version} → v{master.version + 1}
              </span>
            </h2>
          ) : (
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="template_name"
              aria-label="Template name"
              disabled={!isAdmin}
              className="w-56 rounded-md border border-zinc-300 bg-white px-2.5 py-1.5 font-mono text-xs text-zinc-900 placeholder:text-zinc-400 focus:border-teal-700 focus:outline-none focus:ring-2 focus:ring-teal-700/20"
            />
          )}

          <div
            role="tablist"
            aria-label="Editor mode"
            className="flex rounded-md border border-zinc-200 bg-white p-0.5 text-xs"
          >
            {(["visual", "source"] as const).map((m) => (
              <button
                key={m}
                role="tab"
                aria-selected={mode === m}
                onClick={() => (m === "source" ? toSource() : toVisual())}
                className={`rounded px-2.5 py-1 font-medium capitalize transition-colors ${
                  mode === m
                    ? "bg-zinc-900 text-white"
                    : "text-zinc-500 hover:text-zinc-800"
                }`}
              >
                {m}
              </button>
            ))}
          </div>

          <div className="ml-auto flex items-center gap-3">
            {save.isError ? (
              <span role="alert" className="text-xs text-red-700">
                {save.error.message}
              </span>
            ) : null}
            {save.isSuccess && editing ? (
              <span className="text-xs font-medium text-teal-800">
                Published v{save.data.version}
              </span>
            ) : null}
            {isAdmin ? (
              <button
                type="button"
                onClick={() => save.mutate()}
                disabled={!canSave}
                title={!nameValid ? "Lowercase letters, digits, _ and -" : undefined}
                className="rounded-md bg-zinc-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-zinc-800 disabled:opacity-60"
              >
                {save.isPending
                  ? "Saving…"
                  : editing
                    ? "Publish new version"
                    : "Create template"}
              </button>
            ) : null}
          </div>
        </div>

        {pinned.length ? (
          <div
            role="alert"
            className="rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-900"
          >
            This template uses Handlebars the visual editor can’t
            represent ({pinned.slice(0, 4).join(", ")}
            {pinned.length > 4 ? `, +${pinned.length - 4} more` : ""}) — pinned
            to source mode.
          </div>
        ) : null}

        {mode === "visual" ? (
          <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white">
            {isAdmin && editor ? (
              <div className="relative">
                <EditorToolbar
                  editor={editor}
                  onInsertField={() => setMenuOpen((o) => !o)}
                />
                {pickerOpen ? (
                  <div className="absolute right-2 top-9">
                    <FieldPicker
                      editor={editor}
                      sampleJson={sampleJson}
                      onClose={() => setPickerOpen(false)}
                    />
                  </div>
                ) : null}
                {menuOpen ? (
                  <div className="absolute right-2 top-9">
                    <InsertMenu
                      editor={editor}
                      brandingKeys={Object.keys(
                        (master?.defaults?.branding ?? {}) as Record<
                          string,
                          string
                        >,
                      )}
                      onPickField={() => {
                        setMenuOpen(false);
                        setPickerOpen(true);
                      }}
                      onClose={() => setMenuOpen(false)}
                    />
                  </div>
                ) : null}
              </div>
            ) : null}
            <div className="isppr-canvas bg-zinc-100 px-4 py-6">
              <EditorContent editor={editor} />
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            <div>
              <p className="mb-1.5 text-xs font-medium text-zinc-600">
                HTML (Handlebars)
              </p>
              <div className={editorBox}>
                <CodeMirror
                  value={sourceHtml}
                  height="380px"
                  extensions={[htmlLang()]}
                  editable={isAdmin}
                  onChange={setSourceHtml}
                />
              </div>
            </div>
            <div>
              <p className="mb-1.5 text-xs font-medium text-zinc-600">CSS</p>
              <div className={editorBox}>
                <CodeMirror
                  value={sourceCss}
                  height="140px"
                  extensions={[cssLang()]}
                  editable={isAdmin}
                  onChange={setSourceCss}
                />
              </div>
            </div>
          </div>
        )}

        {mode === "visual" ? (
          <div>
            <p className="mb-1.5 text-xs font-medium text-zinc-600">
              Document CSS
            </p>
            <div className={editorBox}>
              <CodeMirror
                value={cssValue}
                height="110px"
                extensions={[cssLang()]}
                editable={isAdmin}
                onChange={setCssValue}
              />
            </div>
          </div>
        ) : null}
      </div>

      <section aria-label="Editor side panel" className="min-w-0">
        <div
          role="tablist"
          aria-label="Side panel"
          className="mb-2 flex gap-1 text-xs"
        >
          {(
            [
              ["preview", "Preview"],
              ["setup", "Page setup"],
            ] as const
          ).map(([k, label]) => (
            <button
              key={k}
              role="tab"
              aria-selected={pane === k}
              onClick={() => setPane(k)}
              className={`rounded-md px-2.5 py-1 font-medium transition-colors ${
                pane === k
                  ? "bg-zinc-900 text-white"
                  : "text-zinc-500 hover:text-zinc-800"
              }`}
            >
              {label}
            </button>
          ))}
        </div>

        {pane === "setup" ? (
          <div className="rounded-lg border border-zinc-200 bg-white p-4">
            <PageSetupPanel
              value={page}
              readOnly={!isAdmin}
              error={pageErr}
              onChange={setPage}
            />
          </div>
        ) : (
        <div className="space-y-3 rounded-lg border border-zinc-200 bg-white p-4">
          <p className="text-xs font-medium text-zinc-600">Sample data (JSON)</p>
          <div className={editorBox}>
            <CodeMirror
              value={sampleJson}
              height="110px"
              extensions={[json()]}
              onChange={setSampleJson}
              basicSetup={{ lineNumbers: false, foldGutter: false }}
            />
          </div>
          {dataErr ? (
            <p role="alert" className="text-xs text-red-700">
              {dataErr}
            </p>
          ) : null}
          <button
            type="button"
            onClick={() => {
              try {
                JSON.parse(sampleJson);
                setDataErr("");
                preview.mutate();
              } catch {
                setDataErr("Sample data must be valid JSON.");
              }
            }}
            className="rounded-md px-3 py-2 text-sm font-medium text-zinc-700 ring-1 ring-inset ring-zinc-300 transition-colors hover:bg-zinc-100"
          >
            Render preview
          </button>
          {preview.isPending ? (
            <p className="text-xs text-zinc-500">Rendering through Gotenberg…</p>
          ) : null}
          {preview.isError ? (
            <p role="alert" className="text-xs text-red-700">
              {preview.error.message}
            </p>
          ) : null}
          {pdfUrl ? (
            <iframe
              title="Template preview PDF"
              src={pdfUrl}
              className="h-[480px] w-full rounded-md border border-zinc-200 bg-zinc-100"
            />
          ) : (
            <div className="flex h-[480px] items-center justify-center rounded-md border border-dashed border-zinc-300 bg-stone-50">
              <p className="max-w-56 text-center text-xs text-zinc-400">
                Render a preview to see this draft as a PDF.
              </p>
            </div>
          )}
        </div>
        )}
      </section>
    </div>
  );
}
