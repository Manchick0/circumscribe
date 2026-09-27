import type { Range } from "./reader.js";

export type Diagnostic = { type: "source", message: string, range: Range } | { type: "native", message: string }

export function attach(diagnostic: Diagnostic, range: Range): Diagnostic {
    if (diagnostic.type === "native")
        return { type: "source", message: diagnostic.message, range: range };
    return diagnostic;
}