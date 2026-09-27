import { attach, type Diagnostic } from "./diagnostic.js";
import { application, type Expression, type Function } from "./expression.js";
import { BOOLEAN, complain, readPattern, satisfies, type Pattern } from "./pattern.js";
import { combine, Reader, type Position, type Range } from "./reader.js";
import { traverse, type Scope } from "./scope.js";

type Precedence = 0 /* POSTFIX */ | 1 /* AND */ | 2 /*  */;

/**
 * Read a single `<>` expression from the given {@linkcode Reader}.
 * 
 * ---
 * Once an expression is read, any subsequent operators are processed based on the provided `precedence`.
 * 
 * ```ts
 * const reader = new Reader("|true| and |false|");
 * const expr = readExpression(reader, 2);
 * ```
 * 
 * @param reader 
 * @param precedence 
 * @returns 
 */
export function readExpression(reader: Reader, precedence: Precedence): [Expression, undefined] | [undefined, Diagnostic] {
    if (reader.skipWhitespace()) {
        const position = reader.position();
        if (reader.readOnly('(')) {
            const expression = readExpression(reader, 2);
            if (expression[0]) {
                if (reader.skipWhitespace()) {
                    if (reader.readOnly(')'))
                        return readOps(reader, { ...expression[0], range: reader.range(position) }, precedence);
                    return [undefined, { type: "source", range: reader.pointRange(), message: "Expected a ')'" }];
                }
                return [undefined, { type: "source", range: reader.fullRange(), message: "Encountered an incomplete parenthesized expression" }];
            }
            return expression;
        }
        if (reader.readOnly('not')) {
            const expression = readExpression(reader, 0);
            if (expression[0]) {
                return readOps(reader, {
                    range: reader.range(position),
                    evaluate: (scope) => {
                        const snippet = expression[0].evaluate(scope);
                        if (snippet[0] !== undefined) {
                            if (satisfies(snippet[0], BOOLEAN)) {
                                if (snippet[0] === "true")
                                    return ["false", undefined];
                                return ["true", undefined]
                            }
                            return [undefined, {
                                type: "source",
                                message: complain(snippet[0], BOOLEAN),
                                range: expression[0].range,
                            }];
                        }
                        return snippet;
                    }
                }, precedence)
            }
            return expression;
        }
        if (reader.readOnly('if')) {
            const condition = readExpression(reader, 0);
            if (condition[0]) {
                if (reader.skipWhitespace()) {
                    if (reader.readOnly(':')) {
                        const expr1 = readExpression(reader, 2);
                        if (expr1[0]) {
                            if (reader.skipWhitespace()) {
                                if (reader.readOnly("else")) {
                                    const expr2 = readExpression(reader, 0);
                                    if (expr2[0]) {
                                        return readOps(reader, {
                                            range: reader.range(position),
                                            evaluate: (scope) => {
                                                const snippet = condition[0].evaluate(scope);
                                                if (snippet[0] !== undefined) {
                                                    if (satisfies(snippet[0], BOOLEAN)) {
                                                        if (snippet[0] === "true")
                                                            return expr1[0].evaluate(scope);
                                                        return expr2[0].evaluate(scope);
                                                    }
                                                    return [undefined, {
                                                        type: "source",
                                                        message: complain(snippet[0], BOOLEAN),
                                                        range: condition[0].range,
                                                    }]
                                                }
                                                return snippet;
                                            }
                                        }, precedence)
                                    }
                                    return expr2;
                                }
                                return [undefined, { type: "source", range: reader.wordRange(), message: "Expected an 'else' clause" }]
                            }
                            return [undefined, { type: "source", range: reader.fullRange(), message: "Encountered an incomplete 'if' clause" }]
                        }
                        return expr1;
                    }
                    return [undefined, { type: "source", range: reader.pointRange(), message: "Expected ':'" }]
                }
                return [undefined, { type: "source", range: reader.fullRange(), message: "Encountered an incomplete 'if' clause" }]
            }
            return condition;
        }
        if (reader.readOnly('with')) {
            const buffer: [{ name: string, range: Range }, Expression][] = [];
            while (reader.skipWhitespace()) {
                const [name, range] = readIdentifier(reader);
                if (reader.skipWhitespace()) {
                    if (reader.readOnly(':=')) {
                        const expression = readExpression(reader, 2);
                        if (expression[0]) {
                            buffer.push([{ name, range }, expression[0]]);
                            if (reader.skipWhitespace()) {
                                if (reader.readOnly(','))
                                    continue;
                                if (reader.readOnly(':')) {
                                    const expression = readExpression(reader, 2);
                                    if (expression[0]) {
                                        return [{
                                            range: reader.range(position),
                                            evaluate: (scope) => {
                                                const entries: Record<string, Function> = {};
                                                for (const [{ name, range }, expression] of buffer) {
                                                    const snippet = expression.evaluate(scope);
                                                    if (snippet[0] !== undefined) {
                                                        entries[name] = {
                                                            identifier: name,
                                                            parameters: [],
                                                            expression: {
                                                                range: range,
                                                                evaluate: () => [snippet[0], undefined]
                                                            }
                                                        };
                                                        continue;
                                                    }
                                                    return snippet;
                                                }
                                                return expression[0].evaluate({
                                                    parent: scope,
                                                    entries: entries
                                                })
                                            }
                                        }, undefined]
                                    }
                                    return expression;
                                }
                                return [undefined, { type: "source", range: reader.pointRange(), message: "Expected ':'" }]
                            }
                            return [undefined, { type: "source", range: reader.fullRange(), message: "Encountered an incomplete 'with' clause" }]
                        }
                        return expression;
                    }
                    return [undefined, { type: "source", range: reader.wordRange(), message: "Expected ':='" }]
                }
                return [undefined, { type: "source", range: reader.fullRange(), message: "Encountered an incomplete 'with' clause" }]
            }
            return [undefined, { type: "source", range: reader.fullRange(), message: "Encountered an incomplete 'with' clause" }]
        }
        if (reader.readOnly('|')) {
            const snippet = readSnippet(reader, position);
            if (snippet[0])
                return readOps(reader, snippet[0], precedence);
            return snippet;
        }
        const [identifier] = readIdentifier(reader);
        if (reader.skipWhitespace()) {
            if (reader.readOnly('(')) {
                if (reader.skipWhitespace()) {
                    if (reader.readOnly(')'))
                        return readOps(reader, application(identifier, [], reader.range(position)), precedence);
                    const args: Expression[] = [];
                    while (reader.skipWhitespace()) {
                        const expression = readExpression(reader, precedence);
                        if (expression[0]) {
                            if (reader.skipWhitespace()) {
                                args.push(expression[0]);
                                if (reader.readOnly(','))
                                    continue;
                                if (reader.readOnly(')'))
                                    return readOps(reader, application(identifier, args, reader.range(position)), precedence);
                                return [undefined, { type: "source", range: reader.pointRange(), message: "Expected ')'" }]
                            }
                            return [undefined, { type: "source", range: reader.fullRange(), message: "Encountered an incomplete argument list'" }]
                        }
                        return expression;
                    }
                    return [undefined, { type: "source", range: reader.fullRange(), message: "Encountered an incomplete argument list'" }]
                }
                return [undefined, { type: "source", range: reader.fullRange(), message: "Encountered an incomplete argument list'" }]
            }
            return readOps(reader, application(identifier, [], reader.range(position)), precedence);
        }
        return readOps(reader, application(identifier, [], reader.range(position)), precedence);
    }
    return [undefined, { type: "source", range: reader.pointRange(), message: "Expected an expression" }]
}

export function readOps(reader: Reader, expression: Expression, precedence: Precedence): [Expression, undefined] | [undefined, Diagnostic] {
    if (reader.skipWhitespace()) {
        const position = reader.position();
        if (reader.readOnly('==')) {
            const right = readExpression(reader, 0);
            if (right[0])
                return readOps(reader, {
                    range: combine(expression.range, reader.range(position)),
                    evaluate: (scope) => {
                        const lhs = expression.evaluate(scope);
                        if (lhs[0] !== undefined) {
                            const rhs = right[0].evaluate(scope);
                            if (rhs[0] !== undefined)
                                return [lhs[0] === rhs[0] ? "true" : "false", undefined]
                            return rhs;
                        }
                        return lhs;
                    }
                }, precedence);
            return right;
        }
        if (reader.readOnly('~')) {
            const right = readExpression(reader, 0);
            if (right[0])
                return readOps(reader, {
                    range: combine(expression.range, reader.range(position)),
                    evaluate: (scope) => {
                        const lhs = expression.evaluate(scope);
                        if (lhs[0] !== undefined) {
                            const rhs = right[0].evaluate(scope);
                            if (rhs[0] !== undefined) {
                                return [lhs[0].concat(rhs[0]), undefined]
                            }
                            return rhs;
                        }
                        return lhs;
                    }
                }, precedence);
            return right;
        }
        if (precedence > 0) {
            if (reader.readOnly('and')) {
                const right = readExpression(reader, 0);
                if (right[0])
                    return readOps(reader, {
                        range: combine(expression.range, reader.range(position)),
                        evaluate: (scope) => {
                            const lhs = expression.evaluate(scope);
                            if (lhs[0] !== undefined) {
                                const rhs = right[0].evaluate(scope);
                                if (rhs[0] !== undefined) {
                                    if (lhs[0] === "true" && rhs[0] === "true")
                                        return ["true", undefined];
                                    return ["false", undefined];
                                }
                                return rhs;
                            }
                            return lhs;
                        }
                    }, precedence);
                return right;
            }
        }
        if (precedence > 1) {
            if (reader.readOnly('or')) {
                const right = readExpression(reader, 1);
                if (right[0])
                    return readOps(reader, {
                        range: combine(expression.range, reader.range(position)),
                        evaluate: (scope) => {
                            const lhs = expression.evaluate(scope);
                            if (lhs[0] !== undefined) {
                                const rhs = right[0].evaluate(scope);
                                if (rhs[0] !== undefined) {
                                    if (lhs[0] === "true" || rhs[0] === "true")
                                        return ["true", undefined];
                                    return ["false", undefined];
                                }
                                return rhs;
                            }
                            return lhs;
                        }
                    }, precedence);
                return right;
            }
        }
        return [expression, undefined];
    }
    return [expression, undefined];
}

export function readDefinition(reader: Reader): [Function | undefined, undefined] | [undefined, Diagnostic] {
    if (reader.skipWhitespace()) {
        const position = reader.position()
        if (reader.readOnly('def')) {
            if (reader.skipWhitespace()) {
                const [identifier] = readIdentifier(reader);
                if (reader.skipWhitespace()) {
                    const parameters: Function["parameters"] = [];
                    if (reader.readOnly('(')) {
                        const params = readParameters(reader);
                        if (params[1])
                            return params;
                        parameters.push(...params[0]);
                    }
                    if (reader.skipWhitespace()) {
                        if (reader.readOnly(':=')) {
                            const expression = readExpression(reader, 2);
                            if (expression[0])
                                return [{
                                    identifier: identifier,
                                    parameters: parameters,
                                    expression: expression[0]
                                }, undefined];
                            return expression;
                        }
                        return [undefined, { type: "source", range: reader.wordRange(), message: "Expected '::'" }]
                    }
                    return [undefined, { type: "source", range: reader.range(position), message: "Encountered an incomplete definition" }]
                }
                return [undefined, { type: "source", range: reader.range(position), message: "Encountered an incomplete definition" }]
            }
            return [undefined, { type: "source", range: reader.range(position), message: "Encountered an incomplete definition" }]
        }
        return [undefined, { type: "source", range: reader.pointRange(), message: "Expected a pattern" }]
    }
    return [undefined, undefined];
}

export function readParameters(reader: Reader): [Function["parameters"], undefined] | [undefined, Diagnostic] {
    const buffer: Function["parameters"] = [];
    if (reader.skipWhitespace()) {
        const position = reader.position();
        if (reader.readOnly(')'))
            return [[], undefined];
        while (reader.skipWhitespace()) {
            const [identifier, range] = readIdentifier(reader);
            if (reader.skipWhitespace()) {
                if (reader.readOnly(':')) {
                    if (reader.skipWhitespace()) {
                        const pattern = readPattern(reader);
                        if (pattern[0] === undefined)
                            return pattern;
                        buffer.push({
                            name: identifier,
                            range: range,
                            pattern: pattern[0]
                        });
                    }
                } else {
                    buffer.push({
                        name: identifier,
                        range: range,
                        pattern: { type: "any" }
                    });
                }
                if (reader.skipWhitespace()) {
                    if (reader.readOnly(','))
                        continue;
                    if (reader.readOnly(')'))
                        return [buffer, undefined];
                    return [undefined, { type: "source", range: reader.range(position), message: "Encountered an incomplete parameter list" }]
                }
                return [undefined, { type: "source", range: reader.range(position), message: "Encountered an incomplete parameter list" }]
            }
            return [undefined, { type: "source", range: reader.range(position), message: "Encountered an incomplete parameter list" }]
        }
        return [undefined, { type: "source", range: reader.range(position), message: "Encountered an incomplete parameter list" }]
    }
    return [undefined, { type: "source", range: reader.fullRange(), message: "Encountered an incomplete parameter list" }]
}

export function readIdentifier(reader: Reader): [string, Range] {
    if (reader.skipWhitespace()) {
        const buffer: string[] = [];
        const position = reader.position();
        while (reader.canRead()) {
            const c = reader.peek()!;
            if (c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c === '_' || c === '-') {
                buffer.push(c);
                reader.read();
                continue;
            }
            break;
        }
        return [buffer.join(''), reader.range(position)];
    }
    throw new Error();
}

export function readSnippet(reader: Reader, position: Position): [Expression, undefined] | [undefined, Diagnostic] {
    const children: (string | Expression)[] = [];
    const buffer: string[] = [];
    while (reader.canRead()) {
        const c = reader.peek()!;
        if (reader.readOnly('<')) {
            const expression = readExpression(reader, 2);
            if (expression[0]) {
                if (reader.skipWhitespace()) {
                    if (reader.readOnly('>')) {
                        if (buffer.length > 0) {
                            const content = buffer.join('');
                            children.push(content);
                        }
                        children.push(expression[0]);
                        buffer.length = 0;
                        continue;
                    }
                    return [undefined, { type: "source", range: reader.pointRange(), message: "Expected '>' to terminate a substitution clause" }]
                }
                return [undefined, { type: "source", range: reader.range(position), message: "Encountered an incomplete snippet" }]
            }
            return expression;
        }
        if (reader.readOnly('\\')) {
            if (reader.readOnly('<')) {
                buffer.push('<');
                continue;
            }
            if (reader.readOnly('>')) {
                buffer.push('>');
                continue;
            }
            if (reader.readOnly('|')) {
                buffer.push('|');
                continue;
            }
            if (reader.readOnly('\\')) {
                buffer.push('\\');
                continue;
            }
            if (reader.readOnly('n')) {
                buffer.push('\n');
                continue;
            }
            if (reader.readOnly('r')) {
                buffer.push('\r');
                continue;
            }
            if (reader.readOnly('t')) {
                buffer.push('\t');
                continue;
            }
            if (reader.readOnly('b')) {
                buffer.push('\b');
                continue;
            }
            return [undefined, { type: "source", range: reader.pointRange(), message: "Encountered an unknown escape sequence" }]
        }
        if (reader.readOnly('|')) {
            if (buffer.length > 0 || children.length === 0) {
                const content = buffer.join('');
                children.push(content);
            }
            return [{
                range: reader.range(position),
                evaluate: (scope) => {
                    const buffer = [];
                    for (const child of children) {
                        if (typeof child === "string") {
                            buffer.push(child);
                            continue;
                        }
                        const result = child.evaluate(scope);
                        if (result[0]) {
                            buffer.push(result[0])
                            continue;
                        }
                        return result;
                    }
                    return [buffer.join(''), undefined]
                }
            }, undefined];
        }
        buffer.push(c);
        reader.read();
    }
    return [undefined, { type: "source", range: reader.range(position), message: "Encountered an incomplete snippet" }]
}