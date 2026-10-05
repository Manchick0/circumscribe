import type { Diagnostic } from "./diagnostic.js";
import type { Reader } from "./source/reader.js";
import { readRegular } from "./parser.js";
import { combine, Excerpt } from "./source/position.js";

///
/// The singleton 'any' pattern
/// ---
/// Represents a pattern that's satisfied by any given
/// snippet. In theoretical terms, 'any' is the universe
/// of all possible snippets.
///
/// All other patterns may be thought of as
/// predicates over 'any'.
///
export const ANY: unique symbol = Symbol();

///
/// A predicate over a snippet
/// ---
/// A pattern acts similarly to a type in all usual type systems.
/// Circumscribe, however, due to its lack of types altogether,
/// only has to differentiate between snippets.
///
/// The universe of all possible snippets 'any' is infinite. One may,
/// however, wish to define an operation for only a subset of 'any'.
/// A pattern acts as a tool to carve out a set of the desired values
/// out of 'any':
///
/// literal(lit) = { lit }
/// regular(regex) = { x ∈ any | regex matches x }
/// union(a₀, a₁, a₂, ..., aₙ) = a₀ ∪ a₁ ∪ a₂ ∪ ... ∪ aₙ
///
export type Pattern =
    | typeof ANY
    | {
          readonly type: "literal";
          readonly literal: string;
      }
    | {
          readonly type: "regular";
          readonly expression: RegExp;
      }
    | {
          readonly type: "union";
          readonly options: Pattern[];
      };

///
/// The boolean pattern
/// ---
/// Represents a pattern satisfied exclusively by the snippets
/// |true| and |false|. The boolean pattern is essential to
/// circumscribe and is used in order to define `not`, `and`,
/// and `or` operations, alongside if-expressions.
///
/// boolean = { "true", "false" }
///
export const BOOLEAN: Pattern = {
    type: "union",
    options: [
        {
            type: "literal",
            literal: "true"
        },
        {
            type: "literal",
            literal: "false"
        }
    ]
};

///
/// Determine if the given `snippet` satisifies
/// the given `pattern`.
/// ---
/// Returns true if the snippet would be an element
/// of the set represented by the pattern, and `false`
/// otherwise.
///
export function satisfies(snippet: string, pattern: Pattern): boolean {
    if (pattern === ANY) return true;
    if (pattern.type === "literal") return snippet === pattern.literal;
    if (pattern.type === "regular") return pattern.expression.test(snippet);
    return pattern.options.some((pat) => satisfies(snippet, pat));
}

export function complain(snippet: string, pattern: Pattern): string {
    return `Snippet '${snippet}' does not satisfy the pattern '${stringify(pattern)}'`;
}

///
/// Stringify a pattern.
/// ---
/// Computes the string representation of the given
/// pattern.
///
/// It is generally intended for a string returned
/// by this function to produce a logically identical
/// pattern, if parsed.
///
export function stringify(pattern: Pattern): string {
    if (pattern === ANY) return "any";
    if (pattern.type === "literal")
        return `|${pattern.literal
            .replace("\\", "\\\\")
            .replace("\n", "\\n")
            .replace("\t", "\\t")
            .replace("\b", "\\b")
            .replace("\r", "\\r")
            .replace("|", "\\|")}|`;
    if (pattern.type === "union") return pattern.options.map((option) => stringify(option)).join(" + ");
    return `/${pattern.expression.source}/`;
}

///
/// Read a pattern
/// ---
/// Reads a single pattern from the given `reader`.
/// If successful, it is returned together with
/// the corresponding excerpt.
///
/// Otherwise, a Diagnostic describing the error
/// is returned.
///
export function readPattern(reader: Reader): [[Pattern, Excerpt], undefined] | [undefined, Diagnostic] {
    if (reader.skipWhitespace()) {
        const position = reader.position();
        if (reader.readOnly("any")) return [[ANY, reader.excerpt(position)], undefined];
        if (reader.isAt("|")) {
            const literal = readLiteral(reader);
            if (literal[0] !== undefined) {
                return readAlternative(reader, literal[0]);
            }
            return literal;
        }
        if (reader.isAt("/")) {
            const expression = readRegular(reader);
            if (expression[0] !== undefined) {
                const [expr, excerpt] = expression[0];
                return readAlternative(reader, [
                    {
                        type: "regular",
                        expression: expr
                    },
                    excerpt
                ]);
            }
            return expression;
        }
    }
    return [
        undefined,
        {
            type: "source",
            message: "Expected a pattern",
            excerpt: reader.fullExcerpt()
        }
    ];
}

///
/// Read an alternative pattern
/// ---
/// Reads another pattern, separated by a `+`, if such
/// is present. If successful, the two patterns are
/// joined together as a `union`.
///
function readAlternative(reader: Reader, pattern: [Pattern, Excerpt]): [[Pattern, Excerpt], undefined] | [undefined, Diagnostic] {
    if (reader.skipWhitespace()) {
        if (reader.readOnly("+")) {
            const alternative = readPattern(reader);
            if (alternative[0] !== undefined) {
                return [
                    [
                        {
                            type: "union",
                            options: [pattern[0], alternative[0][0]]
                        },
                        combine(pattern[1], alternative[0][1].range)
                    ],
                    undefined
                ];
            }
            return alternative;
        }
        return [pattern, undefined];
    }
    return [pattern, undefined];
}

///
/// Read a literal pattern.
/// ---
/// Reads a single literal pattern from the given `reader`,
/// in the form of `|...|`. If an error occurs, a `Diagnostic`
/// is returned instead.
///
function readLiteral(reader: Reader): [[Pattern, Excerpt], undefined] | [undefined, Diagnostic] {
    if (reader.skipWhitespace()) {
        const position = reader.position();
        if (reader.readOnly("|")) {
            const buffer: string[] = [];
            while (reader.canRead()) {
                if (reader.readOnly("|"))
                    return [
                        [
                            {
                                type: "literal",
                                literal: buffer.join("")
                            },
                            reader.excerpt(position)
                        ],
                        undefined
                    ];
                buffer.push(reader.read()!);
            }
            return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Encountered an incomplete literal pattern" }];
        }
        return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Expected a literal pattern" }];
    }
    return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Expected a literal pattern" }];
}
