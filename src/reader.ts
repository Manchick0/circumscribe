export class Reader {

    private readonly source: string;
    private absolute: number;
    private column: number;
    private line: number;

    constructor(source: string, position: Position = { absolute: 0, column: 1, line: 1 }) {
        this.source = source;
        this.absolute = position.absolute;
        this.column = position.column;
        this.line = position.line;
    }

    read(n: number = 1): string | undefined {
        const c = this.source.codePointAt(this.absolute);
        if (c === undefined) return undefined;
        while (n--) {
            const c = this.source.codePointAt(this.absolute);
            if (c === undefined) break;
            this.absolute += length(c);
            if (c === 0xA) {
                this.column = 1;
                this.line++;
                continue;
            }
            this.column++;
        }
        return String.fromCharCode(c);
    }

    chainRead(n: number = 1): this {
        this.read(n);
        return this;
    }

    readOnly(sequence: string): boolean {
        if (this.isAt(sequence)) {
            this.read(sequence.length);
            return true;
        }
        return false;
    }

    peek(): string | undefined {
        const point = this.source.codePointAt(this.absolute);
        if (point)
            return String.fromCodePoint(point);
        return undefined;
    }

    isAt(sequence: string): boolean {
        if (this.canRead()) {
            for (let i = 0; i < sequence.length; i++) {
                // Relying on UTF-16 since we only care about comparing
                if (this.source[this.absolute + i] === sequence[i])
                    continue;
                return false;
            }
            return true;
        }
        return false;
    }

    skipWhitespace(): boolean {
        while (this.canRead()) {
            const c = this.peek()!;
            if (isspace(c)) {
                this.read();
                continue;
            }
            return true;
        }
        return false;
    }

    canRead(): boolean {
        return this.absolute < this.source.length;
    }

    position(): Position {
        return { absolute: this.absolute, column: this.column, line: this.line }
    }

    range(position: Position): Range {
        return [position, this.position()];
    }

    pointRange(): Range {
        const c = this.source.codePointAt(this.absolute);
        if (c) {
            if (c === 0x0A || c === 0x0D)
                return [this.position(), { absolute: this.absolute + 1, column: 1, line: this.line + 1 }]
            return [this.position(), { absolute: this.absolute + length(c), column: this.column, line: this.line }];
        }
        return [this.position(), { absolute: this.absolute + 1, column: this.column, line: this.line }];
    }

    branch(): Reader {
        return new Reader(this.source, this.position());
    }

    wordRange(): Range {
        const branch = this.branch();
        while (branch.canRead()) {
            const c = branch.peek()!;
            if (isspace(c)) {
                branch.read();
                continue;
            }
            break;
        }
        return [this.position(), branch.position()];
    }

    fullRange(): Range {
        return [{ absolute: 0, column: 1, line: 1 }, this.position()];
    }
}

export type Position = {
    absolute: number;
    column: number;
    line: number;
}

export type Range = [Position, Position];

export function combine(first: Range, second: Range): Range {
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
    return [min(first[0], second[0]), max(first[1], second[1])];
}

export function excerpt(source: string, [left, right]: Range): string {
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
        const c = source.codePointAt(position)!;
        if (c === 0x0A || c === 0x0D) {
            buffer.push(gutter(++line, width));
            if (c === 0x0D)
                if (source[position + 1] == '\n')
                    position++;
            position++;
            continue;
        }
        buffer[buffer.length - 1]![0].push(String.fromCodePoint(c));
        buffer[buffer.length - 1]![1].push(position >= left.absolute && position < right.absolute ? '^' : ' ');
        position += length(c);
    }
    return buffer.map(line => line.map(segment => segment.join('')).join('\n')).join('\n')
}

function length(point: number): 1 | 2 {
    if (point > 0xFFFF)
        return 2;
    return 1;
}

function isspace(c: string): boolean {
    return c === ' ' || c === '\n' || c === '\t' || c === '\r';
}