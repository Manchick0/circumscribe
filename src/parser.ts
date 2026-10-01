import { type Diagnostic } from "./diagnostic.js";
import { application, type Expression, type Macro } from "./expression.js";
import { BOOLEAN, complain, readPattern, satisfies } from "./pattern.js";
import { combine, Excerpt, expand, Position } from "./position.js";
import { Reader } from "./reader.js";

type Precedence = 0 /* POSTFIX */ | 1 /* AND */ | 2 /*  */;

/**
 * Read a single expression from the given {@linkcode Reader}.
 * 
 * ---
 * Once an expression is read, any subsequent operators are processed based on the provided `precedence`.
 * 
 * ```ts
 * const reader = new Reader("|true| and |false|");
 * const expr   = readExpression(reader, 2);
 * ```
 */
export function readExpression(reader: Reader, precedence: Precedence): [Expression, undefined] | [undefined, Diagnostic] {
    if (reader.skipWhitespace()) {
        const position = reader.position();
        if (reader.readOnly('(')) {
            const expression = readExpression(reader, 2);
            if (expression[0] !== undefined) {
                if (reader.skipWhitespace()) {
                    if (reader.readOnly(')'))
                        return readOperations(reader, { ...expression[0], excerpt: expand(expression[0].excerpt, position) }, precedence);
                    return [undefined, { type: "source", excerpt: reader.pointExcerpt(), message: "Expected a ')'" }];
                }
                return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Encountered an incomplete parenthesized expression" }];
            }
            return expression;
        }
        if (reader.readOnly('not')) {
            const expression = readExpression(reader, 0);
            if (expression[0] !== undefined) {
                return readOperations(reader, {
                    excerpt: expand(expression[0].excerpt, position),
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
                                excerpt: expression[0].excerpt,
                            }];
                        }
                        return snippet;
                    }
                }, precedence)
            }
            return expression;
        }
        if (reader.readOnly('if')) {
            const condition = readExpression(reader, 2);
            if (condition[0] !== undefined) {
                if (reader.skipWhitespace()) {
                    if (reader.readOnly(':')) {
                        const expr1 = readExpression(reader, 2);
                        if (expr1[0] !== undefined) {
                            if (reader.skipWhitespace()) {
                                if (reader.readOnly("else")) {
                                    const expr2 = readExpression(reader, 2);
                                    if (expr2[0] !== undefined) {
                                        return [{
                                            excerpt: expand(expr2[0].excerpt, position),
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
                                                        excerpt: condition[0].excerpt,
                                                    }]
                                                }
                                                return snippet;
                                            }
                                        }, undefined]
                                    }
                                    return expr2;
                                }
                                return [undefined, { type: "source", excerpt: reader.wordExcerpt(), message: "Expected an 'else' clause" }]
                            }
                            return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Encountered an incomplete 'if' clause" }]
                        }
                        return expr1;
                    }
                    return [undefined, { type: "source", excerpt: reader.pointExcerpt(), message: "Expected ':'" }]
                }
                return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Encountered an incomplete 'if' clause" }]
            }
            return condition;
        }
        if (reader.readOnly('with')) {
            const expression = readExpression(reader, 2);
            if (expression[0] !== undefined) {
                if (reader.skipWhitespace()) {
                    if (reader.readOnly('as')) {
                        if (reader.skipWhitespace()) {
                            const identifier = readIdentifier(reader);
                            if (identifier[0] !== undefined) {
                                if (reader.readOnly(':')) {
                                    const body = readExpression(reader, 2);
                                    if (body[0] !== undefined) {
                                        return [{
                                            excerpt: expand(expression[0].excerpt, position),
                                            evaluate: (scope) => {
                                                const snippet = expression[0].evaluate(scope);
                                                if (snippet[0] !== undefined) {
                                                    return body[0].evaluate({
                                                        root: scope.root,
                                                        entries: {
                                                            [identifier[0][0]]: {
                                                                type: "source",
                                                                identifier: identifier[0][0],
                                                                parameters: [],
                                                                expression: {
                                                                    excerpt: identifier[0][1],
                                                                    evaluate: () => [snippet[0], undefined]
                                                                }
                                                            }
                                                        },
                                                        parent: scope
                                                    })
                                                }
                                                return snippet;
                                            }
                                        }, undefined];
                                    }
                                    return body;
                                }
                                return [undefined, { type: "source", excerpt: reader.wordExcerpt(), message: "Expected ':'" }]
                            }
                            return identifier;
                        }
                        return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Encountered an incomplete 'with' clause" }]
                    }
                    return [undefined, { type: "source", excerpt: reader.wordExcerpt(), message: "Expected 'as'" }]
                }
                return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Encountered an incomplete 'with' clause" }]
            }
            return expression;
        }
        if (reader.readOnly('match')) {
            const pattern = readRegular(reader);
            if (pattern[0] !== undefined) {
                if (reader.skipWhitespace()) {
                    const groups: [string, Excerpt][] = [];
                    if (reader.readOnly('as')) {
                        while (reader.skipWhitespace()) {
                            const identifier = readIdentifier(reader);
                            if (identifier[0] !== undefined) {
                                groups.push(identifier[0])
                                if (reader.readOnly(','))
                                    continue;
                                break
                            }
                            return identifier;
                        }
                    }
                    if (reader.skipWhitespace()) {
                        if (reader.readOnly('in')) {
                            const source = readExpression(reader, 2);
                            if (source[0] !== undefined) {
                                if (reader.skipWhitespace()) {
                                    if (reader.readOnly(':')) {
                                        const expression = readExpression(reader, 2);
                                        if (expression[0] !== undefined) {
                                            if (reader.skipWhitespace()) {
                                                if (reader.readOnly('else')) {
                                                    const other = readExpression(reader, 2);
                                                    if (other[0] !== undefined) {
                                                        return [{
                                                            excerpt: expand(expression[0].excerpt, position),
                                                            evaluate: (scope) => {
                                                                const snippet = source[0].evaluate(scope);
                                                                if (snippet[0] !== undefined) {
                                                                    const match = pattern[0].exec(snippet[0]);
                                                                    if (match) {
                                                                        const child: Record<string, Macro> = {};
                                                                        for (let i = 0; i < groups.length; i++) {
                                                                            const [name, range] = groups[i]!;
                                                                            const group = match[i + 1] ?? '';
                                                                            child[name] = {
                                                                                type: "source",
                                                                                identifier: name,
                                                                                parameters: [],
                                                                                expression: {
                                                                                    excerpt: range,
                                                                                    evaluate: () => [group, undefined]
                                                                                }
                                                                            }
                                                                        }
                                                                        return expression[0].evaluate({ root: scope.root, parent: scope, entries: child });
                                                                    }
                                                                    return other[0].evaluate(scope);
                                                                }
                                                                return snippet;
                                                            }
                                                        }, undefined];
                                                    }
                                                    return other;
                                                }
                                                return [undefined, { type: "source", excerpt: reader.wordExcerpt(), message: "Expected 'else'" }]
                                            }
                                            return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Encountered an incomplete 'match' clause" }]
                                        }
                                        return expression;
                                    }
                                    return [undefined, { type: "source", excerpt: reader.wordExcerpt(), message: "Expected ':'" }]
                                }
                                return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Encountered an incomplete 'match' clause" }]
                            }
                            return source;
                        }
                        return [undefined, { type: "source", excerpt: reader.wordExcerpt(), message: "Expected 'in'" }]
                    }
                    return [undefined, { type: "source", excerpt: reader.wordExcerpt(), message: "Expected 'as'" }]
                }
                return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Encountered an incomplete 'match' clause" }]
            }
            return pattern;
        }
        if (reader.readOnly('switch')) {
            const operand = readExpression(reader, 2);
            if (operand[0] !== undefined) {
                if (reader.skipWhitespace()) {
                    if (reader.readOnly(':')) {
                        const options: {
                            readonly condition: Expression,
                            readonly body: Expression
                        }[] = [];
                        while (reader.skipWhitespace()) {
                            if (reader.readOnly('case')) {
                                const condition = readExpression(reader, 2);
                                if (condition[0] !== undefined) {
                                    if (reader.skipWhitespace()) {
                                        if (reader.readOnly(':')) {
                                            const body = readExpression(reader, 2);
                                            if (body[0] !== undefined) {
                                                options.push({
                                                    condition: condition[0],
                                                    body: body[0]
                                                });
                                                continue;
                                            }
                                            return body;
                                        }
                                        return [undefined, { type: "source", excerpt: reader.wordExcerpt(), message: "Expected ':'" }]
                                    }
                                    return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Encountered an incomplete 'switch' clause" }]
                                }
                                return condition;
                            }
                            if (reader.readOnly('default')) {
                                if (reader.skipWhitespace()) {
                                    if (reader.readOnly(':')) {
                                        const body = readExpression(reader, 2);
                                        if (body[0] !== undefined) {
                                            return [{
                                                excerpt: expand(body[0].excerpt, position),
                                                evaluate: (scope) => {
                                                    const upon = operand[0].evaluate(scope);
                                                    if (upon[0] !== undefined) {
                                                        for (const option of options) {
                                                            const snippet = option.condition.evaluate(scope);
                                                            if (snippet[0] !== undefined) {
                                                                if (snippet[0] === upon[0])
                                                                    return option.body.evaluate(scope);
                                                                continue;
                                                            }
                                                            return snippet;
                                                        }
                                                        return body[0].evaluate(scope);
                                                    }
                                                    return upon;
                                                }
                                            }, undefined]
                                        }
                                        return body;
                                    }
                                    return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Expected ':'" }]
                                }
                                return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Encountered an incomplete 'switch' clause" }]
                            }
                            return [undefined, { type: "source", excerpt: reader.wordExcerpt(), message: "Expected 'default'" }]
                        }
                        return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Encountered an incomplete 'switch' clause" }]
                    }
                    return [undefined, { type: "source", excerpt: reader.wordExcerpt(), message: "Expected ':'" }]
                }
                return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Encountered an incomplete 'switch' clause" }]
            }
            return operand;
        }
        if (reader.readOnly('raise')) {
            const expression = readExpression(reader, 2);
            if (expression[0] !== undefined) {
                const excerpt = expand(expression[0].excerpt, position);
                return [{
                    excerpt: excerpt,
                    evaluate: (scope) => {
                        const message = expression[0].evaluate(scope);
                        if (message[0] !== undefined) {
                            return [undefined, {
                                type: "source",
                                excerpt: excerpt,
                                message: message[0]
                            }];
                        }
                        return message;
                    }
                }, undefined];
            }
            return expression;
        }
        if (reader.readOnly('attempt')) {
            const happy = readExpression(reader, 2);
            if (happy[0] !== undefined) {
                if (reader.skipWhitespace()) {
                    if (reader.readOnly('else')) {
                        const sad = readExpression(reader, 2);
                        if (sad[0] !== undefined) {
                            return [{
                                excerpt: expand(sad[0].excerpt, position),
                                evaluate: (scope) => {
                                    const snippet = happy[0].evaluate(scope);
                                    if (snippet[0] !== undefined)
                                        return snippet;
                                    return sad[0].evaluate(scope);
                                }
                            }, undefined]
                        }
                        return sad;
                    }
                    return [undefined, { type: "source", excerpt: reader.wordExcerpt(), message: "Expected 'else'" }]
                }
                return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Encountered an incomplete 'attempt' clause" }]
            }
            return happy;
        }
        if (reader.readOnly('|')) {
            const snippet = readSnippet(reader, position);
            if (snippet[0] !== undefined)
                return readOperations(reader, snippet[0], precedence);
            return snippet;
        }
        const identifier = readIdentifier(reader);
        if (identifier[0] !== undefined) {
            if (reader.skipWhitespace()) {
                if (reader.readOnly('(')) {
                    if (reader.skipWhitespace()) {
                        if (reader.readOnly(')'))
                            return readOperations(reader, application(identifier[0][0], [], reader.excerpt(position)), precedence);
                        const args: Expression[] = [];
                        while (reader.skipWhitespace()) {
                            const expression = readExpression(reader, precedence);
                            if (expression[0] !== undefined) {
                                if (reader.skipWhitespace()) {
                                    args.push(expression[0]);
                                    if (reader.readOnly(','))
                                        continue;
                                    if (reader.readOnly(')'))
                                        return readOperations(reader, application(identifier[0][0], args, reader.excerpt(position)), precedence);
                                    return [undefined, { type: "source", excerpt: reader.pointExcerpt(), message: "Expected ')'" }]
                                }
                                return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Encountered an incomplete argument list'" }]
                            }
                            return expression;
                        }
                        return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Encountered an incomplete argument list'" }]
                    }
                    return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Encountered an incomplete argument list'" }]
                }
                return readOperations(reader, application(identifier[0][0], [], reader.excerpt(position)), precedence);
            }
            return readOperations(reader, application(identifier[0][0], [], reader.excerpt(position)), precedence);
        }
        return identifier;
    }
    return [undefined, { type: "source", excerpt: reader.pointExcerpt(), message: "Expected an expression" }]
}

/**
 * Read any remaining operations within the given precedence.
 */
export function readOperations(reader: Reader, expression: Expression, precedence: Precedence): [Expression, undefined] | [undefined, Diagnostic] {
    if (reader.skipWhitespace()) {
        if (reader.readOnly('==')) {
            const right = readExpression(reader, 0);
            if (right[0] !== undefined)
                return readOperations(reader, {
                    excerpt: combine(expression.excerpt, right[0].excerpt.range),
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
            if (right[0] !== undefined)
                return readOperations(reader, {
                    excerpt: combine(expression.excerpt, right[0].excerpt.range),
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
        if (precedence > 1) {
            if (reader.readOnly('or')) {
                const right = readExpression(reader, 1);
                if (right[0] !== undefined)
                    return readOperations(reader, {
                        excerpt: combine(expression.excerpt, right[0].excerpt.range),
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
        if (precedence > 0) {
            if (reader.readOnly('and')) {
                const right = readExpression(reader, 0);
                if (right[0] !== undefined)
                    return readOperations(reader, {
                        excerpt: combine(expression.excerpt, right[0].excerpt.range),
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
        return [expression, undefined];
    }
    return [expression, undefined];
}

export function readDefinition(reader: Reader): [Macro | undefined, undefined] | [undefined, Diagnostic] {
    if (reader.skipWhitespace()) {
        const position = reader.position()
        if (reader.readOnly('def')) {
            if (reader.skipWhitespace()) {
                const identifier = readIdentifier(reader);
                if (identifier[0] !== undefined) {
                    if (reader.skipWhitespace()) {
                        const parameters: (Macro & { type: "source" })["parameters"] = [];
                        if (reader.readOnly('(')) {
                            const params = readParameters(reader);
                            if (params[1])
                                return params;
                            parameters.push(...params[0]);
                        }
                        if (reader.skipWhitespace()) {
                            if (reader.readOnly(':')) {
                                const expression = readExpression(reader, 2);
                                if (expression[0] !== undefined)
                                    return [{
                                        type: "source",
                                        identifier: identifier[0][0],
                                        parameters: parameters,
                                        expression: expression[0]
                                    }, undefined];
                                return expression;
                            }
                            return [undefined, { type: "source", excerpt: reader.wordExcerpt(), message: "Expected ':'" }]
                        }
                        return [undefined, { type: "source", excerpt: reader.excerpt(position), message: "Encountered an incomplete definition" }]
                    }
                    return [undefined, { type: "source", excerpt: reader.excerpt(position), message: "Encountered an incomplete definition" }]
                }
                return identifier;
            }
            return [undefined, { type: "source", excerpt: reader.excerpt(position), message: "Encountered an incomplete definition" }]
        }
        return [undefined, { type: "source", excerpt: reader.pointExcerpt(), message: "Expected a pattern" }]
    }
    return [undefined, undefined];
}

export function readParameters(reader: Reader): [(Macro & { type: "source" })["parameters"], undefined] | [undefined, Diagnostic] {
    const buffer: (Macro & { type: "source" })["parameters"] = [];
    if (reader.skipWhitespace()) {
        const position = reader.position();
        if (reader.readOnly(')'))
            return [[], undefined];
        while (reader.skipWhitespace()) {
            const identifier = readIdentifier(reader);
            if (identifier[0] !== undefined) {
                if (reader.skipWhitespace()) {
                    if (reader.readOnly(':')) {
                        if (reader.skipWhitespace()) {
                            const pattern = readPattern(reader);
                            if (pattern[0] === undefined)
                                return pattern;
                            buffer.push({
                                name: identifier[0][0],
                                excerpt: identifier[0][1],
                                pattern: pattern[0]
                            });
                        }
                    } else {
                        buffer.push({
                            name: identifier[0][0],
                            excerpt: identifier[0][1],
                            pattern: { type: "any" }
                        });
                    }
                    if (reader.skipWhitespace()) {
                        if (reader.readOnly(','))
                            continue;
                        if (reader.readOnly(')'))
                            return [buffer, undefined];
                        return [undefined, { type: "source", excerpt: reader.excerpt(position), message: "Encountered an incomplete parameter list" }]
                    }
                    return [undefined, { type: "source", excerpt: reader.excerpt(position), message: "Encountered an incomplete parameter list" }]
                }
                return [undefined, { type: "source", excerpt: reader.excerpt(position), message: "Encountered an incomplete parameter list" }]
            }
            return identifier;
        }
        return [undefined, { type: "source", excerpt: reader.excerpt(position), message: "Encountered an incomplete parameter list" }]
    }
    return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Encountered an incomplete parameter list" }]
}

export function readIdentifier(reader: Reader): [[string, Excerpt], undefined] | [undefined, Diagnostic] {
    if (reader.skipWhitespace()) {
        const c = reader.peek()!;
        if (c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c === '_' || c === '-') {
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
            return [[buffer.join(''), reader.excerpt(position)], undefined];
        }
        return [undefined, { type: "source", excerpt: reader.wordExcerpt(), message: "Expected an identifier" }]
    }
    return [undefined, { type: "source", excerpt: reader.fullExcerpt(), message: "Expected an identifier" }]
}

export function readRegular(reader: Reader): [RegExp, undefined] | [undefined, Diagnostic] {
    if (reader.skipWhitespace()) {
        if (reader.readOnly('/')) {
            const buffer: string[] = [];
            while (reader.canRead()) {
                const c = reader.peek()!;
                if (reader.readOnly('/'))
                    return [new RegExp(buffer.join('')), undefined]
                if (reader.readOnly('\/')) {
                    buffer.push('/');
                    continue;
                }
                buffer.push(c);
                reader.read();
            }
        }
        return [undefined, {
            type: "source",
            message: "Expected a regular expression",
            excerpt: reader.pointExcerpt()
        }]
    }
    return [undefined, {
        type: "source",
        message: "Encountered an incomplete regular expression",
        excerpt: reader.fullExcerpt()
    }]
}

export function readSnippet(reader: Reader, position: Position): [Expression, undefined] | [undefined, Diagnostic] {
    const children: (string | Expression)[] = [];
    const buffer: string[] = [];
    while (reader.canRead()) {
        const c = reader.peek()!;
        if (reader.readOnly('<')) {
            const expression = readExpression(reader, 2);
            if (expression[0] !== undefined) {
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
                    return [undefined, { type: "source", excerpt: reader.pointExcerpt(), message: "Expected '>' to terminate a substitution clause" }]
                }
                return [undefined, { type: "source", excerpt: reader.excerpt(position), message: "Encountered an incomplete snippet" }]
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
            return [undefined, { type: "source", excerpt: reader.pointExcerpt(), message: "Encountered an unknown escape sequence" }]
        }
        if (reader.readOnly('|')) {
            if (buffer.length > 0 || children.length === 0) {
                const content = buffer.join('');
                children.push(content);
            }
            return [{
                excerpt: reader.excerpt(position),
                evaluate: (scope) => {
                    const buffer = [];
                    for (const child of children) {
                        if (typeof child === "string") {
                            buffer.push(child);
                            continue;
                        }
                        const result = child.evaluate(scope);
                        if (result[0] !== undefined) {
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
    return [undefined, { type: "source", excerpt: reader.excerpt(position), message: "Encountered an incomplete snippet" }]
}