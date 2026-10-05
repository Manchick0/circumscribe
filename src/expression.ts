import { attach, type Diagnostic } from "./diagnostic.js";
import { complain, satisfies, type Pattern } from "./pattern.js";
import { Excerpt } from "./source/position.js";
import { traverse, type Scope } from "./scope.js";

///
/// A macro function.
/// ---
/// A macro function may either be a `native` one, implemented in TypeScript,
/// or a `source` one, defined within the `.circumscribe` file, and implemented
/// in terms of an `Expression`.
///
/// Despite the differences in implementation, the `source` functions
/// are annotated with their corresponding excerpt.
///
export type Macro =
    | {
          readonly type: "native";
          readonly identifier: string;
          readonly parameters: Pattern[];
          readonly body: (args: string[]) => [string, undefined] | [undefined, Diagnostic];
      }
    | {
          readonly type: "source";
          readonly identifier: string;
          readonly excerpt: Excerpt;
          readonly parameters: {
              readonly name: string;
              readonly pattern: Pattern;
          }[];
          readonly expression: Expression;
      };

///
/// A single expression.
/// ---
/// An expression represents a piece of code that may be evaluated
/// within a given `Scope`. An expression either successfully evaluates
/// to a string, or returns a `Diagnostic` describing the error.
///
/// Since all expression inevitably come from a circumscribe source,
/// every expression must always carry the `Excerpt` it originates from.
///
export type Expression = {
    readonly excerpt: Excerpt;
    readonly evaluate: (scope: Scope) => [string, undefined] | [undefined, Diagnostic];
};

///
/// Compute an expression representing an application of a macro.
/// ---
/// Computes an expression that, when evaluated, finds and applies
/// the macro function represented by the given `identifier` within
/// the received `Scope`.
///
/// When applying a native function, any native diagnostic returned
/// by the macro will be attached to the `excerpt` of the application,
/// effectively turning the diagnostic into a source one.
///
export function application(identifier: string, args: Expression[], excerpt: Excerpt): Expression {
    return {
        excerpt: excerpt,
        evaluate: (scope) => {
            const callee = traverse(scope, identifier);
            if (callee) {
                const { type, identifier, parameters } = callee;
                if (parameters.length === args.length) {
                    if (type === "source") {
                        const buffer: Record<string, Macro> = {};
                        for (let i = 0; i < parameters.length; i++) {
                            const { name, pattern } = parameters[i]!;
                            const argument = args[i]!;
                            const snippet = argument.evaluate(scope);
                            if (snippet[0] !== undefined) {
                                if (satisfies(snippet[0], pattern)) {
                                    buffer[name] = {
                                        type: "native",
                                        identifier: name,
                                        parameters: [],
                                        body: () => snippet
                                    };
                                    continue;
                                }
                                return [
                                    undefined,
                                    {
                                        type: "source",
                                        excerpt: argument.excerpt,
                                        message: complain(snippet[0], pattern)
                                    }
                                ];
                            }
                            return snippet;
                        }
                        return callee.expression.evaluate({ root: scope.root, parent: scope.root, entries: buffer });
                    }
                    const buffer: string[] = [];
                    for (let i = 0; i < parameters.length; i++) {
                        const pattern = parameters[i]!;
                        const argument = args[i]!;
                        const snippet = argument.evaluate(scope);
                        if (snippet[0] !== undefined) {
                            if (satisfies(snippet[0], pattern)) {
                                buffer.push(snippet[0]);
                                continue;
                            }
                            return [
                                undefined,
                                {
                                    type: "source",
                                    excerpt: argument.excerpt,
                                    message: complain(snippet[0], pattern)
                                }
                            ];
                        }
                        return snippet;
                    }
                    const result = callee.body(buffer);
                    if (result[0] !== undefined) return result;
                    return [undefined, attach(result[1], excerpt)];
                }
                return [
                    undefined,
                    {
                        type: "source",
                        excerpt: excerpt,
                        message: `'${identifier}' expects ${parameters.length} argument(s), but ${args.length} were provided`
                    }
                ];
            }
            return [undefined, { type: "source", excerpt: excerpt, message: `${identifier} is not defined` }];
        }
    };
}
