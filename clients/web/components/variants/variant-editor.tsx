"use client";

import { css } from "@codemirror/lang-css";
import { json } from "@codemirror/lang-json";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import dynamic from "next/dynamic";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import { CardSkeleton, ErrorState } from "@/components/states";
import { ApiError } from "@/lib/api/client";
import { variants } from "@/lib/api/resources";
import type { TemplateMaster, TemplateVariant, VariantOverlay } from "@/lib/api/types";
import { keys, useMaster, useMe, useVariant } from "@/lib/queries";

// The customer variant editor (MPE-42): edits the three overlay keys —
// css_patch, toggles, branding — against the master's defaults, previews
// the draft through Gotenberg, and saves via upsert. Masters are never
// touched.

// CodeMirror loads client-side only — it touches DOM APIs at module scope.
const CodeMirror = dynamic(() => import("@uiw/react-codemirror"), {
  ssr: false,
});

type BrandingMap = Record<string, string>;
type ToggleMap = Record<string, boolean>;

function buildOverlay(
  cssPatch: string,
  toggles: ToggleMap,
  branding: BrandingMap,
): VariantOverlay {
  const overlay: VariantOverlay = {};
  if (cssPatch.trim()) overlay.css_patch = cssPatch;
  if (Object.keys(toggles).length) overlay.toggles = toggles;
  if (Object.keys(branding).length) overlay.branding = branding;
  return overlay;
}

function masterDefaults(master: TemplateMaster) {
  const d = master.defaults as { toggles?: ToggleMap; branding?: BrandingMap };
  return { toggles: d.toggles ?? {}, branding: d.branding ?? {} };
}

const inputCls =
  "w-full rounded-md border border-zinc-300 bg-white px-2.5 py-1.5 font-mono text-xs text-zinc-900 placeholder:text-zinc-400 focus:border-teal-700 focus:outline-none focus:ring-2 focus:ring-teal-700/20";
const editorBox =
  "overflow-hidden rounded-md border border-zinc-300 bg-white text-xs [&_.cm-editor]:text-xs [&_.cm-editor]:bg-white";

function ToggleSection({
  defaults,
  overrides,
  readOnly,
  onChange,
}: {
  defaults: ToggleMap;
  overrides: ToggleMap;
  readOnly: boolean;
  onChange: (next: ToggleMap) => void;
}) {
  const names = useMemo(
    () => [...new Set([...Object.keys(defaults), ...Object.keys(overrides)])].sort(),
    [defaults, overrides],
  );
  if (names.length === 0) {
    return (
      <p className="text-xs text-zinc-500">
        This master declares no toggleable blocks.
      </p>
    );
  }
  return (
    <ul className="divide-y divide-zinc-100 rounded-md border border-zinc-200 bg-white">
      {names.map((name) => {
        const overridden = name in overrides;
        const value = overridden ? overrides[name] : defaults[name];
        return (
          <li key={name} className="flex items-center gap-3 px-3 py-2">
            <input
              id={`tgl-${name}`}
              type="checkbox"
              checked={Boolean(value)}
              disabled={readOnly}
              onChange={(e) =>
                onChange({ ...overrides, [name]: e.target.checked })
              }
              className="h-3.5 w-3.5 rounded border-zinc-300 text-teal-800 accent-teal-800"
            />
            <label
              htmlFor={`tgl-${name}`}
              className="flex-1 font-mono text-xs text-zinc-800"
            >
              {name}
            </label>
            {overridden ? (
              <button
                type="button"
                disabled={readOnly}
                onClick={() => {
                  const next = { ...overrides };
                  delete next[name];
                  onChange(next);
                }}
                className="text-[11px] font-medium text-zinc-400 hover:text-zinc-700 disabled:opacity-50"
                title={`Reset to master default (${String(defaults[name])})`}
              >
                overridden · reset
              </button>
            ) : (
              <span className="text-[11px] text-zinc-400">master default</span>
            )}
          </li>
        );
      })}
    </ul>
  );
}

function BrandingSection({
  defaults,
  overrides,
  readOnly,
  onChange,
}: {
  defaults: BrandingMap;
  overrides: BrandingMap;
  readOnly: boolean;
  onChange: (next: BrandingMap) => void;
}) {
  const names = useMemo(
    () => [...new Set([...Object.keys(defaults), ...Object.keys(overrides)])].sort(),
    [defaults, overrides],
  );
  const [newKey, setNewKey] = useState("");
  return (
    <div className="space-y-2">
      {names.length === 0 ? (
        <p className="text-xs text-zinc-500">
          This master declares no branding paths — add a key below to set one.
        </p>
      ) : (
        <ul className="divide-y divide-zinc-100 rounded-md border border-zinc-200 bg-white">
          {names.map((name) => {
            const overridden = name in overrides;
            return (
              <li key={name} className="flex items-center gap-3 px-3 py-2">
                <span
                  className="w-32 shrink-0 truncate font-mono text-xs text-zinc-800"
                  title={name}
                >
                  {name}
                </span>
                <input
                  aria-label={`Branding path for ${name}`}
                  value={overridden ? overrides[name] : ""}
                  placeholder={defaults[name] ?? "asset path"}
                  disabled={readOnly}
                  onChange={(e) => {
                    const next = { ...overrides };
                    if (e.target.value === "" && name in defaults) delete next[name];
                    else next[name] = e.target.value;
                    onChange(next);
                  }}
                  className={inputCls}
                />
                {overridden ? (
                  <button
                    type="button"
                    disabled={readOnly}
                    onClick={() => {
                      const next = { ...overrides };
                      delete next[name];
                      onChange(next);
                    }}
                    className="shrink-0 text-[11px] font-medium text-zinc-400 hover:text-zinc-700 disabled:opacity-50"
                  >
                    reset
                  </button>
                ) : null}
              </li>
            );
          })}
        </ul>
      )}
      {!readOnly ? (
        <form
          className="flex items-center gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            const k = newKey.trim();
            if (k && !(k in overrides)) {
              onChange({ ...overrides, [k]: "" });
              setNewKey("");
            }
          }}
        >
          <input
            aria-label="New branding key"
            value={newKey}
            onChange={(e) => setNewKey(e.target.value)}
            placeholder="add branding key (e.g. logo)"
            className={`${inputCls} max-w-64`}
          />
          <button
            type="submit"
            disabled={!newKey.trim()}
            className="shrink-0 rounded-md px-2.5 py-1.5 text-xs font-medium text-zinc-700 ring-1 ring-inset ring-zinc-300 hover:bg-zinc-100 disabled:opacity-50"
          >
            Add key
          </button>
        </form>
      ) : null}
    </div>
  );
}

function PreviewPane({
  overlay,
  readOnly,
  preview,
}: {
  overlay: VariantOverlay;
  readOnly: boolean;
  preview: (dataJson: string) => void;
}) {
  const [dataJson, setDataJson] = useState('{\n  "doc_title": "Sample"\n}');
  const [dataErr, setDataErr] = useState("");
  return (
    <div className="space-y-2">
      <p className="text-xs font-medium text-zinc-600">Sample data (JSON)</p>
      <div className={editorBox}>
        <CodeMirror
          value={dataJson}
          height="110px"
          extensions={[json()]}
          onChange={setDataJson}
          basicSetup={{ lineNumbers: false, foldGutter: false }}
        />
      </div>
      {dataErr ? (
        <p role="alert" className="text-xs text-red-700">
          {dataErr}
        </p>
      ) : null}
      {!readOnly ? (
        <button
          type="button"
          onClick={() => {
            try {
              JSON.parse(dataJson);
              setDataErr("");
              preview(dataJson);
            } catch {
              setDataErr("Sample data must be valid JSON.");
            }
          }}
          className="rounded-md px-3 py-2 text-sm font-medium text-zinc-700 ring-1 ring-inset ring-zinc-300 transition-colors hover:bg-zinc-100"
        >
          Render preview
        </button>
      ) : null}
      {/* overlay referenced so the pane rerenders on draft edits */}
      <span className="hidden">{JSON.stringify(overlay)}</span>
    </div>
  );
}

function EditorForm({
  clientId,
  customerId,
  master,
  variant,
  isAdmin,
}: {
  clientId: string;
  customerId: string;
  master: TemplateMaster;
  variant: TemplateVariant | null;
  isAdmin: boolean;
}) {
  const router = useRouter();
  const qc = useQueryClient();
  const defaults = masterDefaults(master);

  const [cssPatch, setCssPatch] = useState(variant?.overlay.css_patch ?? "");
  const [toggles, setToggles] = useState<ToggleMap>(variant?.overlay.toggles ?? {});
  const [branding, setBranding] = useState<BrandingMap>(
    variant?.overlay.branding ?? {},
  );
  const [pdfUrl, setPdfUrl] = useState<string | null>(null);

  const save = useMutation({
    mutationFn: () =>
      variants.upsert({
        master_id: master.id,
        customer_id: customerId,
        overlay: buildOverlay(cssPatch, toggles, branding) as Record<string, unknown>,
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: keys.variant(master.id, customerId) });
      void qc.invalidateQueries({ queryKey: keys.variants(customerId) });
    },
  });

  const preview = useMutation({
    mutationFn: (dataJson: string) =>
      variants.preview({
        master_id: master.id,
        overlay: buildOverlay(cssPatch, toggles, branding) as Record<string, unknown>,
        data: JSON.parse(dataJson) as Record<string, unknown>,
      }),
    onSuccess: (blob) => {
      setPdfUrl((old) => {
        if (old) URL.revokeObjectURL(old);
        return URL.createObjectURL(blob);
      });
    },
  });

  const overlay = buildOverlay(cssPatch, toggles, branding);
  const readOnly = !isAdmin;

  return (
    <div className="grid grid-cols-1 gap-6 xl:grid-cols-2">
      <div className="space-y-6">
        <section aria-labelledby="ov-toggles">
          <h3 id="ov-toggles" className="mb-2 text-sm font-semibold text-zinc-900">
            Blocks
          </h3>
          <ToggleSection
            defaults={defaults.toggles}
            overrides={toggles}
            readOnly={readOnly}
            onChange={setToggles}
          />
        </section>

        <section aria-labelledby="ov-branding">
          <h3 id="ov-branding" className="mb-2 text-sm font-semibold text-zinc-900">
            Branding paths
          </h3>
          <BrandingSection
            defaults={defaults.branding}
            overrides={branding}
            readOnly={readOnly}
            onChange={setBranding}
          />
        </section>

        <section aria-labelledby="ov-css">
          <h3 id="ov-css" className="mb-2 text-sm font-semibold text-zinc-900">
            CSS patch
          </h3>
          <div className={editorBox}>
            <CodeMirror
              value={cssPatch}
              height="180px"
              extensions={[css()]}
              editable={!readOnly}
              onChange={setCssPatch}
            />
          </div>
          <p className="mt-1.5 text-[11px] text-zinc-400">
            Appended after the master CSS — later rules win the cascade.
          </p>
        </section>

        <div className="flex items-center gap-3">
          {isAdmin ? (
            <button
              type="button"
              onClick={() => save.mutate()}
              disabled={save.isPending}
              className="rounded-md bg-zinc-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-zinc-800 disabled:opacity-60"
            >
              {save.isPending ? "Saving…" : variant ? "Save override" : "Create override"}
            </button>
          ) : null}
          {save.isSuccess && !save.isPending ? (
            <span className="text-xs font-medium text-teal-800">Saved</span>
          ) : null}
          {save.isError ? (
            <span role="alert" className="text-xs text-red-700">
              {save.error.message}
            </span>
          ) : null}
          <button
            type="button"
            onClick={() => router.push(`/clients/${clientId}/customers/${customerId}`)}
            className="text-sm font-medium text-zinc-500 hover:text-zinc-800"
          >
            Back to overrides
          </button>
        </div>
      </div>

      <section aria-labelledby="preview-heading" className="min-w-0">
        <h3 id="preview-heading" className="mb-2 text-sm font-semibold text-zinc-900">
          Preview
        </h3>
        <div className="space-y-3 rounded-lg border border-zinc-200 bg-white p-4">
          <PreviewPane overlay={overlay} readOnly={false} preview={(d) => preview.mutate(d)} />
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
              title="Variant preview PDF"
              src={pdfUrl}
              className="h-[480px] w-full rounded-md border border-zinc-200 bg-zinc-100"
            />
          ) : (
            <div className="flex h-[480px] items-center justify-center rounded-md border border-dashed border-zinc-300 bg-stone-50">
              <p className="max-w-56 text-center text-xs text-zinc-400">
                Render a preview to see the draft overlay applied to this
                template.
              </p>
            </div>
          )}
        </div>
      </section>
    </div>
  );
}

export function VariantEditor({
  clientId,
  customerId,
  masterId,
}: {
  clientId: string;
  customerId: string;
  masterId: string;
}) {
  const me = useMe();
  const master = useMaster(masterId);
  const variant = useVariant(masterId, customerId);

  const missing =
    variant.error instanceof ApiError && variant.error.status === 404;

  if (master.isLoading || variant.isLoading) {
    return <CardSkeleton lines={5} />;
  }
  if (master.isError || !master.data) {
    return (
      <ErrorState
        message={master.error?.message ?? "Template master not found."}
        onRetry={() => void master.refetch()}
      />
    );
  }
  if (variant.isError && !missing) {
    return (
      <ErrorState
        message={variant.error.message}
        onRetry={() => void variant.refetch()}
      />
    );
  }
  if (!me.isFetched) {
    return <CardSkeleton lines={2} />;
  }

  return (
    <EditorForm
      clientId={clientId}
      customerId={customerId}
      master={master.data}
      variant={variant.data ?? null}
      isAdmin={me.data?.role === "admin"}
    />
  );
}
