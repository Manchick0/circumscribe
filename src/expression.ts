import { root, traverse, type Scope } from "./scope.js";

export type Expression = (scope: Scope) => string;

export function application(identifier: string, args: Expression[]): Expression {
    return (scope) => {
        const callee = traverse(scope, identifier);
        if (callee) {
            const buffer: string[] = [];
            for (const argument of args)
                buffer.push(argument(scope));
            return callee(root(scope), buffer);
        }
        throw new ReferenceError(`${identifier} is not defined`);
    }
}

export function forJoin(identifier: string, delimiter: Expression, source: Expression, expression: Expression, join: Expression): Expression {
    return (scope) => {
        const src = source(scope);
        const segments = src.split(delimiter(scope));
        const buffer = [];
        for (const segment of segments) {
            const child: Scope = { parent: scope, entries: { [identifier]: () => segment } }
            buffer.push(expression(child));
        }
        return buffer.join(join(scope));
    }
}