import { expand } from "./expression.js";
export function apply({ identifier, expression, parameters }, root, args) {
    if (parameters.length === args.length) {
        const scope = {
            parent: root,
            entries: args.reduce((acc, string, i) => {
                const parameter = parameters[i];
                acc[parameter] = {
                    identifier: string,
                    parameters: [],
                    expression: {
                        type: "literal",
                        content: string
                    }
                };
                return acc;
            }, {})
        };
        return expand(expression, scope);
    }
    throw new TypeError(`Arity mismatch when applying function ${identifier} with ${parameters} => ${args}`);
}
