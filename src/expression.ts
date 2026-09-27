import { attach, type Diagnostic } from "./diagnostic.js";
import type { Range } from "./reader.js";
import { complain, satisfies, type Pattern } from "./pattern.js";
import { traverse, type Scope } from "./scope.js";

export type Function = {
    readonly type: "native",
    readonly identifier: string;
    readonly parameters: Pattern[],
    readonly body: (args: string[]) => [string, undefined] | [undefined, Diagnostic]
} | {
    readonly type: "source"
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
                const { type, identifier, parameters } = callee;
                if (parameters.length === args.length) {
                    if (type === "source") {
                        const buffer: Record<string, Function> = {};
                        for (let i = 0; i < parameters.length; i++) {
                            const { name, pattern, range } = parameters[i]!;
                            const argument = args[i]!;
                            const snippet = argument.evaluate(scope);
                            if (snippet[0] !== undefined) {
                                if (satisfies(snippet[0], pattern)) {
                                    buffer[name] = {
                                        type: "source",
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
                        return callee.expression.evaluate({ root: scope.root, parent: scope.root, entries: buffer })
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
                            return [undefined, {
                                type: "source",
                                range: argument.range,
                                message: complain(snippet[0], pattern)
                            }]
                        }
                        return snippet;
                    }
                    const result = callee.body(buffer);
                    if (result[0] !== undefined)
                        return result;
                    return [undefined, attach(result[1], range)];
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