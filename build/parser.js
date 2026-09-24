import { application, forJoin } from "./expression.js";
import { pattern } from "./function.js";
import { Reader } from "./reader.js";
import { STANDARD } from "./scope.js";
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
export function readExpression(reader, precedence) {
    if (reader.skipWhitespace()) {
        if (reader.readOnly('(')) {
            const expression = readExpression(reader, 2);
            if (reader.skipWhitespace()) {
                if (reader.readOnly(')')) {
                    return readOps(reader, expression, precedence);
                }
                throw new SyntaxError(reader.diagnostic("Expected ')'"));
            }
            throw new SyntaxError(reader.diagnostic("Encountered an unterminated parameterized expression"));
        }
        if (reader.readOnly('not')) {
            const expression = readExpression(reader, 0);
            return readOps(reader, (scope) => {
                const value = expression(scope);
                if (value === "true")
                    return "false";
                if (value === "false")
                    return "true";
                throw new Error();
            }, precedence);
        }
        if (reader.readOnly('if')) {
            const condition = readExpression(reader, 0);
            if (reader.skipWhitespace()) {
                if (reader.readOnly(':')) {
                    const expr1 = readExpression(reader, 2);
                    if (reader.skipWhitespace()) {
                        if (reader.readOnly("else")) {
                            const expr2 = readExpression(reader, 0);
                            return readOps(reader, (scope) => {
                                const value = condition(scope);
                                if (value === "true")
                                    return expr1(scope);
                                if (value === "false")
                                    return expr2(scope);
                                throw new Error();
                            }, precedence);
                        }
                        throw new SyntaxError(reader.diagnostic("Expected an else clause"));
                    }
                    throw new SyntaxError(reader.diagnostic("Encountered an unterminated if clause"));
                }
                throw new SyntaxError(reader.diagnostic("Expected ':'"));
            }
            throw new SyntaxError(reader.diagnostic("Encountered an unterminated if clause"));
        }
        if (reader.readOnly("for")) {
            const identifier = readIdentifier(reader);
            if (reader.skipWhitespace()) {
                if (reader.readOnly("between")) {
                    const delimiter = readExpression(reader, 0);
                    if (reader.skipWhitespace()) {
                        if (reader.readOnly("in")) {
                            const source = readExpression(reader, 0);
                            if (reader.skipWhitespace()) {
                                if (reader.readOnly(':')) {
                                    const expression = readExpression(reader, 0);
                                    if (reader.skipWhitespace() && reader.readOnly("join")) {
                                        const join = readExpression(reader, 0);
                                        return forJoin(identifier, delimiter, source, expression, join);
                                    }
                                    return forJoin(identifier, delimiter, source, expression, () => "");
                                }
                                throw new SyntaxError(reader.diagnostic("Expected ':'"));
                            }
                            throw new SyntaxError(reader.diagnostic("Encountered an unterminated for clause"));
                        }
                        throw new SyntaxError(reader.diagnostic("Expected 'in'"));
                    }
                    throw new SyntaxError(reader.diagnostic("Encountered an unterminated for clause"));
                }
                if (reader.readOnly('in')) {
                    const source = readExpression(reader, 0);
                    if (reader.skipWhitespace()) {
                        if (reader.readOnly(':')) {
                            const expression = readExpression(reader, 0);
                            if (reader.skipWhitespace() && reader.readOnly("join")) {
                                const join = readExpression(reader, 0);
                                return forJoin(identifier, () => "", source, expression, join);
                            }
                            return forJoin(identifier, () => "", source, expression, () => "");
                        }
                        throw new SyntaxError(reader.diagnostic("Expected ':'"));
                    }
                    throw new SyntaxError(reader.diagnostic("Encountered an unterminated for clause"));
                }
                throw new SyntaxError(reader.diagnostic("Expected 'in'"));
            }
            throw new SyntaxError(reader.diagnostic("Encountered an unterminated for clause"));
        }
        if (reader.readOnly('match')) {
            const identifier = readIdentifier(reader);
            if (reader.skipWhitespace()) {
                if (reader.readOnly('of')) {
                    const pattern = readExpression(reader, 2);
                    if (reader.skipWhitespace()) {
                        if (reader.readOnly('in')) {
                            const source = readExpression(reader, 2);
                            if (reader.canRead()) {
                                if (reader.readOnly(':')) {
                                    const expression = readExpression(reader, 2);
                                    return readOps(reader, (scope) => {
                                        const regex = new RegExp(pattern(scope), "g");
                                        const src = source(scope);
                                        return src.replaceAll(regex, (substring) => expression({
                                            parent: scope,
                                            entries: {
                                                [identifier]: () => substring
                                            }
                                        }));
                                    }, precedence);
                                }
                                throw new SyntaxError(reader.diagnostic("Expected ':'"));
                            }
                            throw new SyntaxError(reader.diagnostic("Encountered an uncomplete 'match' clause"));
                        }
                        throw new SyntaxError(reader.diagnostic("Expected 'in'"));
                    }
                    throw new SyntaxError(reader.diagnostic("Encountered an uncomplete 'match' clause"));
                }
                throw new SyntaxError(reader.diagnostic("Expected 'of'"));
            }
            throw new SyntaxError(reader.diagnostic("Encountered an uncomplete 'match' clause"));
        }
        if (reader.readOnly('with')) {
            const buffer = [];
            while (reader.skipWhitespace()) {
                const name = readIdentifier(reader);
                if (reader.skipWhitespace()) {
                    if (reader.readOnly(':=')) {
                        const expression = readExpression(reader, 2);
                        buffer.push([name, expression]);
                        if (reader.skipWhitespace()) {
                            if (reader.readOnly(','))
                                continue;
                            if (reader.readOnly(':')) {
                                const expression = readExpression(reader, 2);
                                return (scope) => {
                                    const entries = {};
                                    for (const [name, expression] of buffer) {
                                        const snippet = expression(scope);
                                        entries[name] = () => snippet;
                                    }
                                    return expression({
                                        parent: scope,
                                        entries: entries
                                    });
                                };
                            }
                            throw new SyntaxError(reader.diagnostic("Expected ':'"));
                        }
                        throw new SyntaxError(reader.diagnostic("Encountered an uncomplete 'with' clause"));
                    }
                    throw new SyntaxError(reader.diagnostic("Expected ':='"));
                }
                throw new SyntaxError(reader.diagnostic("Encountered an uncomplete 'with' clause"));
            }
            throw new SyntaxError(reader.diagnostic("Encountered an uncomplete 'with' clause"));
        }
        if (reader.readOnly('|'))
            return readOps(reader, readSnippet(reader), precedence);
        const identifier = readIdentifier(reader);
        if (reader.skipWhitespace()) {
            if (reader.readOnly('(')) {
                if (reader.skipWhitespace()) {
                    if (reader.readOnly(')'))
                        return application(identifier, []);
                    const buffer = [];
                    while (reader.skipWhitespace()) {
                        const expression = readExpression(reader, precedence);
                        if (expression) {
                            if (reader.skipWhitespace()) {
                                buffer.push(expression);
                                if (reader.readOnly(','))
                                    continue;
                                if (reader.readOnly(')'))
                                    return readOps(reader, application(identifier, buffer), precedence);
                                throw new SyntaxError(reader.diagnostic("Expected either a continuation or a termination of the argument list"));
                            }
                            throw new SyntaxError(reader.diagnostic("Encountered an unterminated argument list"));
                        }
                        throw new SyntaxError(reader.diagnostic("Expected an expression"));
                    }
                    throw new SyntaxError(reader.diagnostic("Encountered an unterminated argument list"));
                }
                throw new SyntaxError(reader.diagnostic("Encountered an unterminated argument list"));
            }
            return readOps(reader, application(identifier, []), precedence);
        }
        return readOps(reader, application(identifier, []), precedence);
    }
    throw new SyntaxError(reader.diagnostic("Expected an expression"));
}
export function readOps(reader, expression, precedence) {
    if (reader.skipWhitespace()) {
        if (reader.readOnly('==')) {
            const right = readExpression(reader, 0);
            return readOps(reader, (scope) => expression(scope) === right(scope) ? "true" : "false", precedence);
        }
        if (reader.readOnly('~')) {
            const right = readExpression(reader, 0);
            return readOps(reader, (scope) => expression(scope).concat(right(scope)), precedence);
        }
        if (reader.readOnly('*')) {
            const right = readExpression(reader, 0);
            return readOps(reader, (scope) => expression(scope).concat(right(scope)), precedence);
        }
        if (precedence > 0) {
            if (reader.readOnly('and')) {
                const right = readExpression(reader, 0);
                return readOps(reader, (scope) => {
                    const lhs = expression(scope);
                    const rhs = right(scope);
                    if (lhs === "true" && rhs === "true")
                        return "true";
                    return "false";
                }, precedence);
            }
        }
        if (precedence > 1) {
            if (reader.readOnly('or')) {
                const right = readExpression(reader, 1);
                return readOps(reader, (scope) => {
                    const lhs = expression(scope);
                    const rhs = right(scope);
                    if (lhs === "true" || rhs === "true")
                        return "true";
                    return "false";
                }, precedence);
            }
        }
        return expression;
    }
    return expression;
}
export function readPattern(reader) {
    if (reader.skipWhitespace()) {
        if (reader.readOnly('def')) {
            if (reader.skipWhitespace()) {
                const identifier = readIdentifier(reader);
                if (reader.skipWhitespace()) {
                    const parameters = [];
                    if (reader.readOnly('('))
                        parameters.push(...readParameters(reader));
                    if (reader.skipWhitespace()) {
                        if (reader.readOnly(':=')) {
                            const expression = readExpression(reader, 2);
                            return [identifier, pattern(parameters, expression)];
                        }
                        throw new SyntaxError(reader.diagnostic("Expected '::'"));
                    }
                    throw new SyntaxError(reader.diagnostic("Encountered an unterminated definition"));
                }
                throw new SyntaxError(reader.diagnostic("Encountered an unterminated definition"));
            }
            throw new SyntaxError(reader.diagnostic("Encountered an unterminated definition"));
        }
        throw new SyntaxError(reader.diagnostic("Expected a definition"));
    }
    return undefined;
}
export function readParameters(reader) {
    const buffer = [];
    if (reader.skipWhitespace()) {
        if (reader.readOnly(')'))
            return [];
        while (reader.skipWhitespace()) {
            const identifier = readIdentifier(reader);
            buffer.push(identifier);
            if (reader.skipWhitespace()) {
                if (reader.readOnly(','))
                    continue;
                if (reader.readOnly(')'))
                    return buffer;
                throw new SyntaxError(reader.diagnostic("Expected either a continuation or a termination of the parameter list"));
            }
            throw new SyntaxError(reader.diagnostic("Encountered an unterminated parameter list"));
        }
        throw new SyntaxError(reader.diagnostic("Encountered an unterminated parameter list"));
    }
    throw new SyntaxError(reader.diagnostic("Encountered an unterminated parameter list"));
}
export function readIdentifier(reader) {
    if (reader.skipWhitespace()) {
        const buffer = [];
        while (reader.canRead()) {
            const c = reader.peek();
            if (c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c === '_' || c === '-') {
                buffer.push(c);
                reader.read();
                continue;
            }
            break;
        }
        if (buffer.length == 0)
            throw new Error(reader.diagnostic("Here"));
        return buffer.join('');
    }
    throw new Error();
}
export function readSnippet(reader) {
    const children = [];
    const buffer = [];
    while (reader.canRead()) {
        const c = reader.peek();
        if (reader.readOnly('<')) {
            const expression = readExpression(reader, 2);
            if (reader.skipWhitespace()) {
                if (reader.readOnly('>')) {
                    if (buffer.length > 0) {
                        const content = buffer.join('');
                        children.push(() => content);
                    }
                    children.push(expression);
                    buffer.length = 0;
                    continue;
                }
                throw new SyntaxError(reader.diagnostic("Expected '>' to terminate a substitution clause"));
            }
            throw new SyntaxError(reader.diagnostic("Encountered an unterminated snippet"));
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
            throw new SyntaxError(reader.diagnostic(`Unrecognized escape sequence.`));
        }
        if (reader.readOnly('|')) {
            if (buffer.length > 0 || children.length === 0) {
                const content = buffer.join('');
                children.push(() => content);
            }
            if (children.length > 1)
                return (scope) => children.map(child => child(scope)).join('');
            return children[0];
        }
        buffer.push(c);
        reader.read();
    }
    throw new SyntaxError(reader.diagnostic("Encountered an unterminated snippet"));
}
