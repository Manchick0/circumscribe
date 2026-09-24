import { root, traverse } from "./scope.js";
export function application(identifier, args) {
    return (scope) => {
        const callee = traverse(scope, identifier);
        if (callee) {
            const buffer = [];
            for (const argument of args)
                buffer.push(argument(scope));
            return callee(root(scope), buffer);
        }
        throw new ReferenceError(`${identifier} is not defined`);
    };
}
export function forJoin(identifier, delimiter, source, expression, join) {
    return (scope) => {
        const src = source(scope);
        const segments = src.split(delimiter(scope));
        const buffer = [];
        for (const segment of segments) {
            const child = { parent: scope, entries: { [identifier]: () => segment } };
            buffer.push(expression(child));
        }
        return buffer.join(join(scope));
    };
}
