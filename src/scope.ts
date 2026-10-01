import { type Macro } from "./expression.js";

export const STANDARD: Scope = bootstrap({
    parent: undefined,
    entries: {
        ["identity"]: {
            type: "native",
            identifier: "identity",
            parameters: [
                { type: "any" }
            ],
            body: (args) => [args[0]!, undefined]
        },
        ["env"]: {
            type: "native",
            identifier: "env",
            parameters: [
                { type: "any" }
            ],
            body: (args) => {
                const env = process.env[args[0]!];
                if (env !== undefined)
                    return [env, undefined];
                return ["", undefined];
            }
        },
        ["trim"]: {
            type: "native",
            identifier: "trim",
            parameters: [
                { type: "any" }
            ],
            body: (args) => [args[0]!.trim(), undefined]
        },
        ["length"]: {
            type: "native",
            identifier: "length",
            parameters: [
                { type: "any" }
            ],
            body: (args) => [`${args[0]!.length}`, undefined]
        },
        ["meaningful"]: {
            type: "native",
            identifier: "meaningful",
            parameters: [
                { type: "any" }
            ],
            body: (args) => [args[0] !== '' ? "true" : "false", undefined]
        }
    }
})

export type Scope = {
    readonly root: Scope;
    readonly parent: Scope | undefined;
    readonly entries: {
        [identifier: string]: Macro;
    }
}

export function bootstrap(scope: Omit<Scope, "root">): Scope {
    const self: any = {
        parent: scope.parent,
        entries: scope.entries
    }
    return (self["root"] = self);
}

export function traverse(scope: Scope, identifier: string): Macro | undefined {
    if (identifier in scope.entries)
        return scope.entries[identifier];
    return scope.parent ? traverse(scope.parent, identifier) : undefined;
}