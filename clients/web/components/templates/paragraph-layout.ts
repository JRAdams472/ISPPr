import { Extension } from "@tiptap/core";

// Paragraph layout controls (MPE-51): line-height, space before/after,
// and indent as attributes on paragraph + heading — the formatting a
// document editor needs that TipTap's StarterKit doesn't carry.
//
// Styles are emitted inline so they survive the visual → source →
// server round trip exactly as authored.

const TYPES = ["paragraph", "heading"];

export const ParagraphLayout = Extension.create({
  name: "paragraphLayout",

  addGlobalAttributes() {
    return [
      {
        types: TYPES,
        attributes: {
          lineHeight: {
            default: null,
            parseHTML: (el: HTMLElement) => el.style.lineHeight || null,
            renderHTML: (attrs: Record<string, string | null>) =>
              attrs.lineHeight ? { style: `line-height:${attrs.lineHeight}` } : {},
          },
          spaceBefore: {
            default: null,
            parseHTML: (el: HTMLElement) => el.style.marginTop || null,
            renderHTML: (attrs: Record<string, string | null>) =>
              attrs.spaceBefore
                ? { style: `margin-top:${attrs.spaceBefore}` }
                : {},
          },
          spaceAfter: {
            default: null,
            parseHTML: (el: HTMLElement) => el.style.marginBottom || null,
            renderHTML: (attrs: Record<string, string | null>) =>
              attrs.spaceAfter
                ? { style: `margin-bottom:${attrs.spaceAfter}` }
                : {},
          },
          indent: {
            default: 0,
            parseHTML: (el: HTMLElement) => {
              const px = parseFloat(el.style.marginLeft || "0");
              return Number.isFinite(px) && px > 0 ? Math.round(px / 24) : 0;
            },
            renderHTML: (attrs: Record<string, number>) =>
              attrs.indent
                ? { style: `margin-left:${attrs.indent * 24}px` }
                : {},
          },
        },
      },
    ];
  },

  addCommands() {
    return {
      setBlockLayout:
        (attrs: {
          lineHeight?: string | null;
          spaceBefore?: string | null;
          spaceAfter?: string | null;
          indent?: number;
        }) =>
        ({ commands }) =>
          TYPES.map((t) => commands.updateAttributes(t, attrs)).some(Boolean),
      indentBlock:
        () =>
        ({ state, commands }) => {
          const cur = firstIndent(state);
          if (cur === null || cur >= 8) return false;
          return TYPES.map((t) =>
            commands.updateAttributes(t, { indent: cur + 1 }),
          ).some(Boolean);
        },
      outdentBlock:
        () =>
        ({ state, commands }) => {
          const cur = firstIndent(state);
          if (cur === null || cur <= 0) return false;
          return TYPES.map((t) =>
            commands.updateAttributes(t, { indent: cur - 1 }),
          ).some(Boolean);
        },
    };
  },
});

// First paragraph/heading indent within the selection — anchors the
// step for every selected block.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function firstIndent(state: any): number | null {
  let cur: number | null = null;
  state.doc.nodesBetween(
    state.selection.from,
    state.selection.to,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (n: any) => {
      if (cur === null && TYPES.includes(n.type.name)) {
        cur = (n.attrs.indent as number) ?? 0;
      }
    },
  );
  return cur;
}

declare module "@tiptap/core" {
  interface Commands<ReturnType> {
    paragraphLayout: {
      setBlockLayout: (attrs: {
        lineHeight?: string | null;
        spaceBefore?: string | null;
        spaceAfter?: string | null;
        indent?: number;
      }) => ReturnType;
      indentBlock: () => ReturnType;
      outdentBlock: () => ReturnType;
    };
  }
}
