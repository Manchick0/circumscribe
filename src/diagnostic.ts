import { Excerpt } from "./position.js";

export type Diagnostic = {
    readonly type: "source",
    readonly message: string,
    readonly excerpt: Excerpt
} | {
    readonly type: "native",
    readonly message: string
}

export function attach(diagnostic: Diagnostic, excerpt: Excerpt): Diagnostic {
    if (diagnostic.type === "native")
        return { type: "source", message: diagnostic.message, excerpt: excerpt };
    return diagnostic;
}