import { traverse } from "../scope.js";
import { BOOLEAN, NUMBER, SNIPPET } from "../type.js";
// general
function literal(literal, type) {
    return { type, evaluate: () => literal };
}
function application(identifier, type, args) {
    return {
        type: type,
        evaluate: (scope) => {
            const callee = traverse(scope, identifier);
            if (callee) {
                return callee.apply(args.map(argument => argument.evaluate(scope)));
            }
            throw new TypeError(`'${identifier}' is not defined`);
        }
    };
}
function condition(type, condition, then, orElse) {
    return {
        type: type,
        evaluate: (scope) => {
            if (condition.evaluate(scope))
                return then.evaluate(scope);
            return orElse.evaluate(scope);
        }
    };
}
function forLoop(identifier, type, range, expression) {
    return {
        type: SNIPPET,
        evaluate: (scope) => {
            const entries = range.evaluate(scope);
            const buffer = [];
            for (const entry of entries) {
                const child = {
                    parent: scope,
                    entries: {
                        [identifier]: {
                            identifier: identifier,
                            parameters: [],
                            returns: type,
                            apply: () => entry
                        }
                    }
                };
                buffer.push(expression.evaluate(child));
            }
            return buffer.join('');
        }
    };
}
// snippets
function concat(left, right) {
    return { type: SNIPPET, evaluate: (scope) => left.evaluate(scope).concat(right.evaluate(scope)) };
}
function join(children) {
    return { type: SNIPPET, evaluate: (scope) => children.map(child => child.evaluate(scope)).join('') };
}
// numbers
function add(left, right) {
    return { type: NUMBER, evaluate: (scope) => left.evaluate(scope) + right.evaluate(scope) };
}
function sub(left, right) {
    return { type: NUMBER, evaluate: (scope) => left.evaluate(scope) - right.evaluate(scope) };
}
function mul(left, right) {
    return { type: NUMBER, evaluate: (scope) => left.evaluate(scope) * right.evaluate(scope) };
}
function div(left, right) {
    return { type: NUMBER, evaluate: (scope) => left.evaluate(scope) / right.evaluate(scope) };
}
function mod(left, right) {
    return { type: NUMBER, evaluate: (scope) => left.evaluate(scope) % right.evaluate(scope) };
}
// booleans
function equals(left, right) {
    return { type: BOOLEAN, evaluate: (scope) => left.evaluate(scope) === right.evaluate(scope) };
}
function lessThan(left, right) {
    return { type: BOOLEAN, evaluate: (scope) => left.evaluate(scope) < right.evaluate(scope) };
}
function lessThanOrEqual(left, right) {
    return { type: BOOLEAN, evaluate: (scope) => left.evaluate(scope) <= right.evaluate(scope) };
}
function greaterThanOrEqual(left, right) {
    return { type: BOOLEAN, evaluate: (scope) => left.evaluate(scope) >= right.evaluate(scope) };
}
