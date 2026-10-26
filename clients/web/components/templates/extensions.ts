import type { AnyExtension } from "@tiptap/core";
import StarterKit from "@tiptap/starter-kit";
import TextAlign from "@tiptap/extension-text-align";
import TextStyle from "@tiptap/extension-text-style";
import Color from "@tiptap/extension-color";
import FontFamily from "@tiptap/extension-font-family";
import Highlight from "@tiptap/extension-highlight";
import Underline from "@tiptap/extension-underline";
import PaginationExtension, {
  PageNode,
  BodyNode,
} from "tiptap-extension-pagination";
import { ConditionalRegion } from "./cond-region";
import { DataTable } from "./data-table";
import { FontSize } from "./font-size";
import { MergeField } from "./merge-field";
import { PageBreak } from "./page-break";
import { ParagraphLayout } from "./paragraph-layout";

// Editor extension set (MPE-51). `paginate` splits the document into
// visual pages via tiptap-extension-pagination — cosmetic only; the PDF
// layout is owned by defaults.page on the Gotenberg side. Tests run
// unpaginated since jsdom has no layout engine.
export function editorExtensions({ paginate = true } = {}): AnyExtension[] {
  const exts: AnyExtension[] = [
    StarterKit.configure({
      heading: { levels: [1, 2, 3, 4, 5, 6] },
      dropcursor: { color: "#0f766e", width: 2 },
    }),
    Underline,
    TextStyle,
    Color,
    FontFamily,
    FontSize,
    Highlight.configure({ multicolor: true }),
    TextAlign.configure({ types: ["heading", "paragraph"] }),
    ParagraphLayout,
    MergeField,
    ConditionalRegion,
    DataTable,
    PageBreak,
  ];
  if (paginate) {
    exts.push(
      PageNode,
      BodyNode,
      PaginationExtension.configure({
        defaultPaperSize: "A4",
        defaultPaperOrientation: "portrait",
        pageAmendmentOptions: { enableHeader: false, enableFooter: false },
      }),
    );
  }
  return exts;
}
