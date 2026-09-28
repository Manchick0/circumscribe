export type Position = {
    readonly absolute: number;
    readonly column: number;
    readonly line: number;
}

export type Excerpt = {
    readonly source: {
        readonly name: string;
        readonly content: string;
    }
    readonly range: [Position, Position]
};

export function pointer(excerpt: Excerpt): string {
    return `(${excerpt.source.name}:${excerpt.range[0].line}-${excerpt.range[1].line})`
}

export function display(excerpt: Excerpt): string {
    const { source: { content: source }, range: [left, right] } = excerpt;
    const begin = Math.max(source.lastIndexOf('\n', left.absolute) + 1, 0);
    const end = (() => {
        const position = source.indexOf('\n', right.absolute);
        if (position === -1)
            return source.length;
        return position;
    })();
    const width = (() => {
        const n = Math.max(left.line, right.line);
        if (n === 0)
            return 1;
        return Math.floor(Math.log10(n)) + 1;
    })();
    const gutter = (line: number, width: number): [string[], string[]] => [
        [`${line.toString().padStart(width)} | `],
        [`${' '.repeat(width)} | `]
    ];
    const buffer: [string[], string[]][] = [gutter(left.line, width)];
    for (let position = begin, line = left.line; position < end;) {
        const c = String.fromCodePoint(source.codePointAt(position)!);
        if (c === "\n" || c === "\r") {
            buffer.push(gutter(++line, width));
            if (c === "\r")
                if (source[position + 1] == '\n')
                    position++;
            position++;
            continue;
        }
        buffer[buffer.length - 1]![0].push(c);
        buffer[buffer.length - 1]![1].push(position >= left.absolute && position < right.absolute ? '^' : ' ');
        position += c.length;
    }
    return buffer.map(line => line.map(segment => segment.join('')).join('\n')).join('\n')
}

export function combine(first: Excerpt, second: Excerpt): Excerpt | undefined {
    function min(first: Position, second: Position): Position {
        if (first.absolute < second.absolute)
            return first;
        return second;
    }
    function max(first: Position, second: Position): Position {
        if (first.absolute >= second.absolute)
            return first;
        return second;
    }
    if (first.source !== second.source)
        return undefined;
    return {
        source: first.source,
        range: [
            min(first.range[0], second.range[0]),
            max(first.range[1], second.range[1])
        ]
    }
}