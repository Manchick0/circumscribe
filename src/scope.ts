import type { Function } from "./expression.js";

export type Scope = {
    readonly parent: Scope | undefined;
    readonly entries: {
        [identifier: string]: Function;
    }
}

export function traverse(scope: Scope, identifier: string): Function | undefined {
    if (identifier in scope.entries)
        return scope.entries[identifier];
    return scope.parent ? traverse(scope.parent, identifier) : undefined;
}

export function root(scope: Scope): Scope {
    if (scope.parent)
        return root(scope.parent);
    return scope;
}