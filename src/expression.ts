import type { Diagnostic } from "./diagnostic.js";
import type { Range } from "./reader.js";
import { complain, satisfies, type Pattern } from "./pattern.js";
import { root, traverse, type Scope } from "./scope.js";

export type Function = {
    readonly identifier: string;
    readonly parameters: {
        readonly name: string,
        readonly range: Range,
        readonly pattern: Pattern
    }[];
    readonly expression: Expression;
}

export type Expression = {
    readonly range: Range;
    readonly evaluate: (scope: Scope) => [string, undefined] | [undefined, Diagnostic];
}

export function application(identifier: string, args: Expression[], range: Range): Expression {
    return {
        range: range,
        evaluate: (scope) => {
            const callee = traverse(scope, identifier);
            if (callee) {
                const { identifier, parameters, expression } = callee;
                if (parameters.length === args.length) {
                    const buffer: Record<string, Function> = {};
                    for (let i = 0; i < parameters.length; i++) {
                        const { name, range, pattern } = parameters[i]!;
                        const argument = args[i]!;
                        const snippet = argument.evaluate(scope);
                        if (snippet[0] !== undefined) {
                            if (satisfies(snippet[0], pattern)) {
                                buffer[name] = {
                                    identifier: name,
                                    parameters: [],
                                    expression: {
                                        range: range,
                                        evaluate: () => snippet
                                    }
                                }
                                continue;
                            }
                            return [undefined, {
                                type: "source",
                                range: argument.range,
                                message: complain(snippet[0], pattern)
                            }]
                        }
                        return snippet;
                    }
                    return expression.evaluate({ parent: root(scope), entries: buffer })
                }
                return [undefined, {
                    type: "source",
                    range: range,
                    message: `'${identifier}' expects ${parameters.length} argument(s), but ${args.length} were provided`
                }];
            }
            return [undefined, { type: "source", range: range, message: `${identifier} is not defined` }];
        }
    }
}