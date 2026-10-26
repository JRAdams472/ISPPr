"use client";

import type { Editor } from "@tiptap/react";
import type { ReactNode } from "react";

// Editor toolbar (MPE-51) — grouped controls over the document canvas:
// history, block type, font, marks, color, alignment, paragraph layout,
// lists, blocks, and merge-field insertion. Glyphs follow the portal's
// authored 1.5px-stroke convention; letterforms for marks per editor
// convention.

const FONT_FAMILIES = [
  ["", "Default"],
  ["Georgia, 'Times New Roman', serif", "Serif"],
  ["Arial, Helvetica, sans-serif", "Sans"],
  ["'Courier New', monospace", "Mono"],
] as const;

const FONT_SIZES = ["", "9", "10", "11", "12", "14", "18", "24", "32"] as const;
const LINE_HEIGHTS = [["", "Auto"], ["1", "1.0"], ["1.15", "1.15"], ["1.5", "1.5"], ["2", "2.0"]] as const;
const SPACING = [["", "None"], ["0.25em", "S"], ["0.5em", "M"], ["1em", "L"], ["2em", "XL"]] as const;

function I({ d }: { d: string }) {
  return (
    <svg
      viewBox="0 0 16 16"
      width="13"
      height="13"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden
    >
      <path d={d} />
    </svg>
  );
}

const GLYPHS = {
  undo: "M6.5 4.5 3.5 7.5l3 3M3.8 7.5h6a3.2 3.2 0 0 1 0 6.4H8.5",
  redo: "M9.5 4.5l3 3-3 3M12.2 7.5h-6a3.2 3.2 0 0 0 0 6.4H7.5",
  alignLeft: "M2.5 3.5h11M2.5 6.5h7M2.5 9.5h11M2.5 12.5h7",
  alignCenter: "M2.5 3.5h11M4.5 6.5h7M2.5 9.5h11M4.5 12.5h7",
  alignRight: "M2.5 3.5h11M6.5 6.5h7M2.5 9.5h11M6.5 12.5h7",
  alignJustify: "M2.5 3.5h11M2.5 6.5h11M2.5 9.5h11M2.5 12.5h11",
  bulletList: "M6.5 4h7M6.5 8h7M6.5 12h7",
  orderedList: "M7 4h6.5M7 8h6.5M7 12h6.5",
  outdent: "M9.5 4.5h4M9.5 8h4M9.5 11.5h4M6.5 5.5l-3 3 3 3",
  indent: "M2.5 4.5h4M2.5 8h4M2.5 11.5h4M9.5 5.5l3 3-3 3",
  quote: "M3 11.5V7a3 3 0 0 1 3-3M9 11.5V7a3 3 0 0 1 3-3",
  hr: "M2.5 8h11",
  field: "M6 3.5C4.9 3.5 4.5 4.2 4.5 5v1.6c0 .8-.5 1.4-1.2 1.4.7 0 1.2.6 1.2 1.4V11c0 .8.4 1.5 1.5 1.5M10 3.5c1.1 0 1.5.7 1.5 1.5v1.6c0 .8.5 1.4 1.2 1.4-.7 0-1.2.6-1.2 1.4V11c0 .8-.4 1.5-1.5 1.5",
} as const;

function Btn({
  onClick,
  active,
  disabled,
  title,
  children,
}: {
  onClick: () => void;
  active?: boolean;
  disabled?: boolean;
  title: string;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      title={title}
      aria-label={title}
      aria-pressed={active}
      disabled={disabled}
      onMouseDown={(e) => e.preventDefault()}
      onClick={onClick}
      className={`flex h-7 min-w-7 items-center justify-center rounded px-1 text-xs transition-colors disabled:opacity-35 ${
        active
          ? "bg-teal-800/10 text-teal-800"
          : "text-zinc-600 hover:bg-zinc-200/70 hover:text-zinc-900"
      }`}
    >
      {children}
    </button>
  );
}

const selCls =
  "h-7 rounded border-0 bg-transparent px-1 text-xs text-zinc-700 hover:bg-zinc-200/70 focus:outline-none focus:ring-1 focus:ring-teal-700/40";

function Sep() {
  return <span className="mx-1 h-4 w-px shrink-0 bg-zinc-300" aria-hidden />;
}

export function EditorToolbar({
  editor,
  onInsertField,
}: {
  editor: Editor;
  onInsertField: () => void;
}) {
  const block = editor.isActive("heading")
    ? `h${editor.getAttributes("heading").level}`
    : editor.isActive("bulletList")
      ? "ul"
      : editor.isActive("orderedList")
        ? "ol"
        : editor.isActive("blockquote")
          ? "quote"
          : "p";
  const ts = editor.getAttributes("textStyle");
  const para = editor.getAttributes(
    editor.isActive("heading") ? "heading" : "paragraph",
  );

  const chain = () => editor.chain().focus();

  return (
    <div
      role="toolbar"
      aria-label="Formatting"
      className="flex flex-wrap items-center gap-0.5 border-b border-zinc-200 bg-white px-2 py-1.5"
    >
      <Btn title="Undo" disabled={!editor.can().undo()} onClick={() => chain().undo().run()}>
        <I d={GLYPHS.undo} />
      </Btn>
      <Btn title="Redo" disabled={!editor.can().redo()} onClick={() => chain().redo().run()}>
        <I d={GLYPHS.redo} />
      </Btn>

      <Sep />

      <select
        aria-label="Block type"
        className={selCls}
        value={block}
        onChange={(e) => {
          const v = e.target.value;
          if (v.startsWith("h"))
            chain().setHeading({ level: Number(v[1]) as 1 | 2 | 3 | 4 | 5 | 6 }).run();
          else if (v === "ul") chain().toggleBulletList().run();
          else if (v === "ol") chain().toggleOrderedList().run();
          else if (v === "quote") chain().toggleBlockquote().run();
          else chain().setParagraph().run();
        }}
      >
        <option value="p">Paragraph</option>
        {[1, 2, 3, 4, 5, 6].map((n) => (
          <option key={n} value={`h${n}`}>{`Heading ${n}`}</option>
        ))}
        <option value="ul">Bulleted list</option>
        <option value="ol">Numbered list</option>
        <option value="quote">Quote</option>
      </select>

      <select
        aria-label="Font family"
        className={selCls}
        value={ts.fontFamily ?? ""}
        onChange={(e) =>
          e.target.value
            ? chain().setFontFamily(e.target.value).run()
            : chain().unsetFontFamily().run()
        }
      >
        {FONT_FAMILIES.map(([v, label]) => (
          <option key={label} value={v}>{label}</option>
        ))}
      </select>

      <select
        aria-label="Font size"
        className={selCls}
        value={ts.fontSize ?? ""}
        onChange={(e) =>
          e.target.value
            ? chain().setFontSize(`${e.target.value}px`).run()
            : chain().unsetFontSize().run()
        }
      >
        {FONT_SIZES.map((v) => (
          <option key={v || "auto"} value={v}>
            {v ? `${v}px` : "Size"}
          </option>
        ))}
      </select>

      <Sep />

      <Btn title="Bold" active={editor.isActive("bold")} onClick={() => chain().toggleBold().run()}>
        <span className="font-bold">B</span>
      </Btn>
      <Btn title="Italic" active={editor.isActive("italic")} onClick={() => chain().toggleItalic().run()}>
        <span className="italic">I</span>
      </Btn>
      <Btn title="Underline" active={editor.isActive("underline")} onClick={() => chain().toggleUnderline().run()}>
        <span className="underline">U</span>
      </Btn>
      <Btn title="Strikethrough" active={editor.isActive("strike")} onClick={() => chain().toggleStrike().run()}>
        <span className="line-through">S</span>
      </Btn>

      <label
        title="Text colour"
        className="relative flex h-7 w-7 cursor-pointer items-center justify-center rounded text-zinc-600 hover:bg-zinc-200/70"
      >
        <span
          className="h-3.5 w-3.5 rounded-sm border border-zinc-400"
          style={{ backgroundColor: ts.color || "#18181b" }}
        />
        <input
          type="color"
          aria-label="Text colour"
          className="absolute inset-0 h-full w-full cursor-pointer opacity-0"
          value={ts.color || "#18181b"}
          onChange={(e) => chain().setColor(e.target.value).run()}
        />
      </label>
      <label
        title="Highlight"
        className="relative flex h-7 w-7 cursor-pointer items-center justify-center rounded text-zinc-600 hover:bg-zinc-200/70"
      >
        <span
          className="h-3.5 w-3.5 rounded-sm border border-zinc-400"
          style={{ backgroundColor: editor.getAttributes("highlight").color || "#ffffff" }}
        />
        <input
          type="color"
          aria-label="Highlight colour"
          className="absolute inset-0 h-full w-full cursor-pointer opacity-0"
          value={editor.getAttributes("highlight").color || "#fef08a"}
          onChange={(e) =>
            chain().setHighlight({ color: e.target.value }).run()
          }
        />
      </label>

      <Sep />

      {(
        [
          ["left", GLYPHS.alignLeft],
          ["center", GLYPHS.alignCenter],
          ["right", GLYPHS.alignRight],
          ["justify", GLYPHS.alignJustify],
        ] as const
      ).map(([dir, d]) => (
        <Btn
          key={dir}
          title={`Align ${dir}`}
          active={editor.isActive({ textAlign: dir })}
          onClick={() => chain().setTextAlign(dir).run()}
        >
          <I d={d} />
        </Btn>
      ))}

      <Sep />

      <select
        aria-label="Line height"
        className={selCls}
        value={para.lineHeight ?? ""}
        onChange={(e) =>
          chain().setBlockLayout({ lineHeight: e.target.value || null }).run()
        }
      >
        {LINE_HEIGHTS.map(([v, label]) => (
          <option key={v || "auto"} value={v}>{`↕ ${label}`}</option>
        ))}
      </select>
      <select
        aria-label="Space before"
        className={selCls}
        value={para.spaceBefore ?? ""}
        onChange={(e) =>
          chain().setBlockLayout({ spaceBefore: e.target.value || null }).run()
        }
      >
        {SPACING.map(([v, label]) => (
          <option key={v || "auto"} value={v}>{`⇡ ${label}`}</option>
        ))}
      </select>
      <select
        aria-label="Space after"
        className={selCls}
        value={para.spaceAfter ?? ""}
        onChange={(e) =>
          chain().setBlockLayout({ spaceAfter: e.target.value || null }).run()
        }
      >
        {SPACING.map(([v, label]) => (
          <option key={v || "auto"} value={v}>{`⇣ ${label}`}</option>
        ))}
      </select>

      <Btn title="Decrease indent" onClick={() => chain().outdentBlock().run()}>
        <I d={GLYPHS.outdent} />
      </Btn>
      <Btn title="Increase indent" onClick={() => chain().indentBlock().run()}>
        <I d={GLYPHS.indent} />
      </Btn>

      <Sep />

      <Btn
        title="Bulleted list"
        active={editor.isActive("bulletList")}
        onClick={() => chain().toggleBulletList().run()}
      >
        <I d={GLYPHS.bulletList} />
      </Btn>
      <Btn
        title="Numbered list"
        active={editor.isActive("orderedList")}
        onClick={() => chain().toggleOrderedList().run()}
      >
        <I d={GLYPHS.orderedList} />
      </Btn>
      <Btn
        title="Blockquote"
        active={editor.isActive("blockquote")}
        onClick={() => chain().toggleBlockquote().run()}
      >
        <I d={GLYPHS.quote} />
      </Btn>
      <Btn title="Horizontal rule" onClick={() => chain().setHorizontalRule().run()}>
        <I d={GLYPHS.hr} />
      </Btn>

      <Sep />

      <Btn title="Insert merge field" onClick={onInsertField}>
        <I d={GLYPHS.field} />
      </Btn>
    </div>
  );
}
