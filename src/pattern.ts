import type { Diagnostic } from "./diagnostic.js";
import { readRegular } from "./parser.js";
import type { Position, Reader } from "./reader.js";

export type Pattern = { type: "any" } | { type: "literal", literal: string } | { type: "regular", expression: RegExp } | { type: "union", options: Pattern[] }

/**
 * The standard boolean pattern: `|true| + |false|`
 */
export const BOOLEAN: Pattern = {
    type: "union",
    options: [
        {
            type: "literal",
            literal: "true"
        },
        {
            type: "literal",
            literal: "false"
        }
    ]
}

export function complain(snippet: string, pattern: Pattern): string {
    return `Snippet '${snippet}' does not satisfy the pattern '${stringify(pattern)}'`;
}

export function satisfies(snippet: string, pattern: Pattern): boolean {
    if (pattern.type === "any")
        return true;
    if (pattern.type === "literal")
        return snippet === pattern.literal;
    if (pattern.type === "union") {
        for (const option of pattern.options) {
            if (satisfies(snippet, option))
                return true;
        }
        return false;
    }
    return pattern.expression.test(snippet);
}

export function stringify(pattern: Pattern): string {
    if (pattern.type === "any")
        return "any";
    if (pattern.type === "literal")
        return `|${pattern.literal.replace("\\", "\\\\")
            .replace("\n", "\\n")
            .replace("\t", "\\t")
            .replace("\b", "\\b")
            .replace("\r", "\\r")
            .replace("|", "\\|")}|`;
    if (pattern.type === "union")
        return pattern.options.map(option => stringify(option)).join(' + ')
    return `/${pattern.expression.source}/`
}

export function readPattern(reader: Reader): [Pattern, undefined] | [undefined, Diagnostic] {
    if (reader.skipWhitespace()) {
        const buffer: Pattern[] = [];
        while (reader.skipWhitespace()) {
            const option = readOption(reader);
            if (option[0] !== undefined) {
                buffer.push(option[0]);
                if (reader.skipWhitespace() && reader.readOnly('+'))
                    continue;
                break;
            }
            return option;
        }
        if (buffer.length == 1)
            return [buffer[0]!, undefined];
        return [{
            type: "union",
            options: buffer
        }, undefined]
    }
    return [undefined, {
        type: "source",
        message: "Expected a pattern",
        range: reader.fullRange()
    }]
}

function readOption(reader: Reader): [Pattern, undefined] | [undefined, Diagnostic] {
    if (reader.skipWhitespace()) {
        const position = reader.position();
        if (reader.readOnly('any'))
            return [{ type: "any" }, undefined]
        if (reader.readOnly('|'))
            return readLiteral(reader, position);
        if (reader.isAt('/')) {
            const expression = readRegular(reader);
            if (expression[0] !== undefined)
                return [{ type: "regular", expression: expression[0] }, undefined]
            return expression;
        }
    }
    return [undefined, {
        type: "source",
        message: "Expected a pattern",
        range: reader.fullRange()
    }]
}

function readLiteral(reader: Reader, position: Position): [Pattern, undefined] | [undefined, Diagnostic] {
    const buffer: string[] = [];
    while (reader.canRead()) {
        const c = reader.peek()!;
        if (reader.readOnly('|'))
            return [{
                type: "literal",
                literal: buffer.join('')
            }, undefined]
        buffer.push(c);
        reader.read();
    }
    return [undefined, {
        type: "source",
        message: "Encountered an incomplete literal pattern",
        range: reader.range(position)
    }]
}