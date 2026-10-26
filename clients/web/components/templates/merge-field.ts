import { Node } from "@tiptap/core";

// Merge-field chip (MPE-51): an inline atom holding a Handlebars binding
// expression. Chips delete/select as a unit and serialize back to plain
// {{expr}} text on export (see lib/templates/serialize.ts).
export const MergeField = Node.create({
  name: "mergeField",
  inline: true,
  group: "inline",
  atom: true,
  selectable: true,

  addAttributes() {
    return {
      expr: {
        default: "",
        parseHTML: (el: HTMLElement) =>
          (el.textContent ?? "").trim().replace(/^\{\{\s*|\s*\}\}$/g, ""),
      },
    };
  },

  parseHTML() {
    return [{ tag: "span.merge-field-chip" }];
  },

  renderHTML({ node }) {
    return [
      "span",
      { class: "merge-field-chip", "data-expr": node.attrs.expr },
      `{{${node.attrs.expr}}}`,
    ];
  },

  renderText({ node }) {
    return `{{${node.attrs.expr}}}`;
  },

  addCommands() {
    return {
      insertMergeField:
        (expr: string) =>
        ({ commands }) =>
          commands.insertContent({ type: this.name, attrs: { expr } }),
    };
  },
});

declare module "@tiptap/core" {
  interface Commands<ReturnType> {
    mergeField: {
      insertMergeField: (expr: string) => ReturnType;
    };
  }
}
