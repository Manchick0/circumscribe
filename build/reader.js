export class Reader {
    name;
    source;
    position;
    column;
    line;
    constructor(name, source) {
        this.name = name;
        this.source = source;
        this.position = 0;
        this.column = 1;
        this.line = 1;
    }
    read(n = 1) {
        const c = this.source[this.position];
        while (n--) {
            const c = this.source[this.position];
            this.position++;
            if (c === '\n') {
                this.column = 1;
                this.line++;
                continue;
            }
            this.column++;
        }
        return c;
    }
    readOnly(sequence) {
        if (this.isAt(sequence)) {
            this.read(sequence.length);
            return true;
        }
        return false;
    }
    peek() {
        return this.source[this.position];
    }
    isAt(sequence) {
        if (this.canRead()) {
            for (let i = 0; i < sequence.length; i++) {
                if (this.source[this.position + i] === sequence[i])
                    continue;
                return false;
            }
            return true;
        }
        return false;
    }
    skipWhitespace() {
        while (this.canRead()) {
            const c = this.peek();
            if (isspace(c)) {
                this.read();
                continue;
            }
            return true;
        }
        return false;
    }
    canRead() {
        return this.position < this.source.length;
    }
    diagnostic(message) {
        return `(${this.name}:${this.line}:${this.column}) ${message}`;
    }
}
function isspace(c) {
    return c === ' ' || c === '\n' || c === '\t' || c === '\r';
}
