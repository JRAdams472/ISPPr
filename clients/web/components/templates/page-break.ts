import { Node } from "@tiptap/core";

// Print breaks (MPE-53): block atoms that serialize as inline
// break-before styles — `page` for a new page, `recto` for a new sheet
// (right-hand side when duplex printing). The rendered div is already
// valid print markup, so export needs no transform.
export const PageBreak = Node.create({
  name: "pageBreak",
  group: "block",
  atom: true,
  selectable: true,

  addAttributes() {
    return {
      kind: {
        default: "page",
        parseHTML: (el: HTMLElement) =>
          (el.getAttribute("data-break") as "page" | "recto") ?? "page",
      },
    };
  },

  parseHTML() {
    return [{ tag: "div[data-break]" }];
  },

  renderHTML({ node }) {
    const kind = node.attrs.kind === "recto" ? "recto" : "page";
    return [
      "div",
      {
        class: `doc-break doc-break-${kind}`,
        "data-break": kind,
        style: `break-before:${kind}`,
      },
    ];
  },
});
