import { Excerpt, Position } from "./position.js";

export class Reader {

    private readonly name: string;
    private readonly source: string;
    private absolute: number;
    private column: number;
    private line: number;

    constructor(name: string, source: string, position: Position = { absolute: 0, column: 1, line: 1 }) {
        this.name = name;
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
            if (c === '#') {
                while (this.canRead()) {
                    const c = this.peek()!;
                    if (c === '\n' || c === '\r')
                        return this.skipWhitespace();
                    this.read();
                }
                return false;
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

    excerpt(position: Position): Excerpt {
        return {
            source: {
                name: this.name,
                content: this.source
            },
            range: [position, this.position()]
        }
    }

    pointExcerpt(): Excerpt {
        const source = { name: this.name, content: this.source };
        const point = this.source.codePointAt(this.absolute);
        const position = this.position();
        if (point) {
            if (point === 0x0A || point === 0x0D)
                return { source: source, range: [position, { absolute: this.absolute + 1, column: 1, line: this.line + 1 }] };
            return { source: source, range: [position, { absolute: this.absolute + length(point), column: this.column, line: this.line }] };
        }
        return { source: source, range: [position, { absolute: this.absolute + 1, column: this.column, line: this.line }] };
    }

    wordExcerpt(): Excerpt {
        const c = this.peek()!;
        if (c >= 'a' && c <= 'b' || c >= 'A' && c <= 'Z' || c === '_') {
            const branch = this.branch();
            while (branch.canRead()) {
                const c = branch.peek()!;
                if (c >= 'a' && c <= 'b' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c === '_') {
                    branch.read();
                    continue;
                }
                break;
            }
            return branch.excerpt(this.position());
        }
        return this.pointExcerpt();
    }

    fullExcerpt(): Excerpt {
        return {
            source: {
                name: this.name,
                content: this.source
            },
            range: [{ absolute: 0, column: 1, line: 1 }, this.position()]
        }
    }

    branch(): Reader {
        return new Reader(this.name, this.source, this.position());
    }
}

function length(point: number): 1 | 2 {
    if (point > 0xFFFF)
        return 2;
    return 1;
}

function isspace(c: string): boolean {
    return c === ' ' || c === '\n' || c === '\t' || c === '\r';
}