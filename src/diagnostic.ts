import { Excerpt } from "./source/position.js";

///
/// A diagnostic.
/// ---
/// A diagnostic of type `"source"` originates from within a circumscribe source,
/// and carries the `Excerpt` associates with it.
///
/// A `"native"` diagnostic, on the other hand, originates from a native function,
/// and therefore may not point to an excerpt within the source. In order to circumvent
/// this, one may wish to `attach()` an excerpt to a native diagnostic once such is present.
///
export type Diagnostic =
    | {
          readonly type: "source";
          readonly message: string;
          readonly excerpt: Excerpt;
      }
    | {
          readonly type: "native";
          readonly message: string;
      };

export function attach(diagnostic: Diagnostic, excerpt: Excerpt): Diagnostic {
    if (diagnostic.type === "native") return { type: "source", message: diagnostic.message, excerpt: excerpt };
    return diagnostic;
}
