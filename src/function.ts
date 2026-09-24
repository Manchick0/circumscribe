import type { Expression } from "./expression.js";
import type { Scope } from "./scope.js";

export type Function = (env: Scope, args: string[]) => string;

export function pattern(parameters: string[], expression: Expression): Function {
    return (env, args) => {
        const entries: Record<string, Function> = {};
        for (let i = 0; i < args.length; i++) {
            const parameter = parameters[i]!;
            const argument = args[i]!;
            entries[parameter] = () => argument;
        }
        return expression({
            parent: env,
            entries: entries
        });
    }
}