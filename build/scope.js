export const STANDARD = {
    parent: undefined,
    entries: {
        identity: (env, args) => {
            if (args.length === 1)
                return args[0];
            throw new Error();
        },
        env: (env, args) => {
            if (args.length === 1 || args.length === 2) {
                const value = process.env[args[0]];
                if (value)
                    return value;
                if (args.length === 2)
                    return args[1];
                return "";
            }
            throw new Error();
        },
        len: (env, args) => {
            if (args.length === 1)
                return `${args[0].length}`;
            throw new Error();
        },
        trim: (env, args) => {
            if (args.length === 1)
                return args[0].trim();
            throw new Error();
        },
        meaningful: (env, args) => {
            if (args.length === 1)
                return args[0].length > 0 ? "true" : "false";
            throw new Error();
        }
    }
};
export function traverse(scope, identifier) {
    if (identifier in scope.entries)
        return scope.entries[identifier];
    return scope.parent ? traverse(scope.parent, identifier) : undefined;
}
export function root(scope) {
    if (scope.parent && scope.parent !== STANDARD)
        return root(scope.parent);
    return scope;
}
