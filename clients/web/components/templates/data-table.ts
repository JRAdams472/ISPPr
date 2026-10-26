import { Node } from "@tiptap/core";

export interface TableCol {
  label: string;
  expr: string;
}

// Data-bound table (MPE-53): an atom node mapping to
// <table><thead><tr><th>…</th></tr></thead>
// <tbody>{{#each path}}<tr><td>{{expr}}</td>…</tr>{{/each}}</tbody></table>
//
// Cells hold literal {{expr}} text (merge chips optional). Round-trip:
// parse rebuilds cols from thead labels + first-row td exprs; export
// re-wraps the tbody inner in the each markers (lowerConstructs).
export const DataTable = Node.create({
  name: "dataTable",
  group: "block",
  atom: true,
  selectable: true,

  addAttributes() {
    return {
      path: { default: "items" },
      cols: { default: [] as TableCol[] },
      repeatHeader: { default: true },
    };
  },

  parseHTML() {
    return [
      {
        tag: "table.data-table",
        getAttrs: (el: HTMLElement) => {
          const tbody = el.querySelector("tbody");
          const path = tbody?.getAttribute("data-each") ?? "items";
          const labels = [
            ...el.querySelectorAll("thead th"),
          ].map((th) => th.textContent ?? "");
          const exprs = [
            ...(tbody?.querySelectorAll("tr:first-child td") ?? []),
          ].map((td) =>
            (td.textContent ?? "").replace(/^\{\{\s*|\s*\}\}$/g, ""),
          );
          return {
            path,
            repeatHeader: Boolean(el.querySelector("thead")),
            cols: exprs.map((expr, i) => ({
              label: labels[i] ?? expr,
              expr,
            })),
          };
        },
      },
    ];
  },

  renderHTML({ node }) {
    const { path, cols, repeatHeader } = node.attrs as {
      path: string;
      cols: TableCol[];
      repeatHeader: boolean;
    };
    const cells = cols.map((c) => ["td", {}, `{{${c.expr}}}`] as const);
    const head = repeatHeader
      ? [
          "thead" as const,
          ["tr", ...cols.map((c) => ["th", {}, c.label] as const)],
        ]
      : null;
    return [
      "table",
      { class: "data-table", "data-path": path },
      ...(head ? [head] : []),
      ["tbody", { "data-each": path }, ["tr", ...cells]],
    ];
  },
});
