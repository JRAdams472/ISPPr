import { Node } from "@tiptap/core";

// Conditional region (MPE-53): an editable labelled block mapping to
// {{#if expr}}…{{/if}}. The label renders via CSS ::before on data-expr,
// so the DOM stays faithful to the exported markup.
export const ConditionalRegion = Node.create({
  name: "conditionalRegion",
  group: "block",
  content: "block+",
  defining: true,

  addAttributes() {
    return {
      expr: {
        default: "",
        parseHTML: (el: HTMLElement) => el.getAttribute("data-expr") ?? "",
        renderHTML: (attrs: Record<string, string>) => ({
          "data-expr": attrs.expr,
        }),
      },
    };
  },

  parseHTML() {
    return [{ tag: "div.cond-region" }];
  },

  renderHTML({ HTMLAttributes }) {
    return ["div", { class: "cond-region", ...HTMLAttributes }, 0];
  },
});
