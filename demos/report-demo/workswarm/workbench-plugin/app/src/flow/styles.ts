import flowCss from "@xyflow/react/dist/style.css";
import cardCss from "./style.css";

export function ensureStyle(doc: Document) {
  if (doc.getElementById("cg-style")) return;
  const st = doc.createElement("style");
  st.id = "cg-style";
  st.textContent = flowCss + cardCss;
  doc.head.appendChild(st);
}
