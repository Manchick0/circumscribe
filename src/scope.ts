import { type Macro } from "./expression.js";
import { ANY } from "./pattern.js";

///
/// The standard library
/// ---
/// A scope containing the standard macros of
/// circumscribe: 9 in-total.
///
export const STANDARD: Scope = bootstrap({
    parent: undefined,
    entries: {
        ["env"]: {
            type: "native",
            identifier: "env",
            parameters: [ANY],
            body: (args) => {
                const env = process.env[args[0]];
                if (env !== undefined) {
                    return [env, undefined];
                }
                return ["", undefined];
            }
        },
        ["reverse"]: {
            type: "native",
            identifier: "reverse",
            parameters: [ANY],
            body: (args) => [[...args[0]].toReversed().join(""), undefined]
        },
        ["uppercase"]: {
            type: "native",
            identifier: "uppercase",
            parameters: [ANY],
            body: (args) => [args[0].toUpperCase(), undefined]
        },
        ["lowercase"]: {
            type: "native",
            identifier: "lowercase",
            parameters: [ANY],
            body: (args) => [args[0].toLowerCase(), undefined]
        },
        ["trim"]: {
            type: "native",
            identifier: "trim",
            parameters: [ANY],
            body: (args) => [args[0].trim(), undefined]
        },
        ["length"]: {
            type: "native",
            identifier: "length",
            parameters: [ANY],
            body: (args) => [[...args[0]].length.toString(), undefined]
        },
        ["head"]: {
            type: "native",
            identifier: "head",
            parameters: [ANY],
            body: (args) => {
                const source = args[0];
                if (source.length > 0) {
                    return [source[0], undefined];
                }
                return ["", undefined];
            }
        },
        ["tail"]: {
            type: "native",
            identifier: "tail",
            parameters: [ANY],
            body: (args) => {
                const source = args[0];
                if (source.length > 1) {
                    return [source.substring(1), undefined];
                }
                return ["", undefined];
            }
        },
        ["meaningful"]: {
            type: "native",
            identifier: "meaningful",
            parameters: [ANY],
            body: (args) => [args[0] !== "" ? "true" : "false", undefined]
        }
    }
});

export type Scope = {
    readonly root: Scope;
    readonly parent: Scope | undefined;
    readonly entries: {
        [identifier: string]: Macro;
    };
};

export function bootstrap(scope: Omit<Scope, "root">): Scope {
    const self: any = {
        parent: scope.parent,
        entries: scope.entries
    };
    return (self["root"] = self);
}

export function traverse(scope: Scope, identifier: string): Macro | undefined {
    if (Object.hasOwn(scope.entries, identifier)) {
        return scope.entries[identifier];
    }
    return scope.parent ? traverse(scope.parent, identifier) : undefined;
}
