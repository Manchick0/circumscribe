import fs = require("node:fs");
import { join, relative } from "node:path";
import { Reader } from "./reader.js";
import { STANDARD, type Scope } from "./scope.js";
import { readPattern, readExpression } from "./parser.js";
import type { Function } from "./function.js";
import { readConfig } from "./config.js";

const CIRCUMSCRIBED_NAME = /^<(.+)>\.(.+)$/
const DOT_CIRCUMSCRIBE: string = join(process.cwd(), ".circumscribe");

function main(args: string[]): number {
    const { structure, mirror } = readConfig();
    const [flags, short, options] = splitArgs(args.slice(1));
    if (fs.existsSync(DOT_CIRCUMSCRIBE)) {
        const content = fs.readFileSync(DOT_CIRCUMSCRIBE, "utf8");
        const reader = new Reader(DOT_CIRCUMSCRIBE, content);
        const entries: Record<string, Function> = {};
        for (; ;) {
            const pattern = readPattern(reader);
            if (pattern) {
                entries[pattern[0]] = pattern[1];
                continue;
            }
            break;
        }
        const scope: Scope = { parent: STANDARD, entries }
        if (flags.includes("expand") || short.includes("e")) {
            if (options.length > 0) {
                const reader = new Reader("--expand", options[0]!);
                const expression = readExpression(reader, 2);
                console.log(expression(scope));
                return 0;
            }
            console.error("Expected an expression");
            return 0;
        }
        splitSubstitute(structure.root, structure.root, structure.build, scope, mirror);
        return 0;
    }
    console.error("Couldn't find the <compare> file in the current working directory");
    return 1;
}

/**
 * Recursively substitute all circumscribed files.
 * 
 * ---
 * 
 * Starting at the provided `root`, all circumscribed files are recursively collected, substituted,
 * and placed in by their normalized name to the _twin-directory_ at `destination`. If `mirror` is set to `true`,
 * any non-circumscribed files are **copied** to the twin-directory.
 * 
 * @param path the path to recursively substitute
 * @param root the root source directory
 * @param destination the root destination directory
 * @param scope the scope used when evaluating expressions
 * @param mirror whether to mirror existing files
 */
function splitSubstitute(path: string, root: string, destination: string, scope: Scope, mirror: boolean) {
    const entries = fs.readdirSync(path, { withFileTypes: true });
    for (const entry of entries) {
        const twin = join(destination, relative(root, path));
        const source = join(path, entry.name);
        fs.mkdirSync(twin, { recursive: true });
        if (entry.isFile()) {
            const name = CIRCUMSCRIBED_NAME.exec(entry.name);
            if (name) {
                substitute(source, join(twin, `${name[1]}.${name[2]}`), scope);
                continue;
            }
            if (mirror)
                fs.copyFileSync(source, join(twin, entry.name));
            continue;
        }
        splitSubstitute(source, root, destination, scope, mirror);
    }
}

/**
 * Substitute the content of the file at the provided `path` and place
 * the replacement at the given `destination`.
 * 
 * ---
 * 
 * @param path 
 * @param destination 
 * @param scope 
 */
function substitute(path: string, destination: string, scope: Scope) {
    const content = fs.readFileSync(path, "utf8");
    const reader = new Reader(path, content);
    const buffer = [];
    while (reader.canRead()) {
        const c = reader.peek();
        if (reader.readOnly('<')) {
            const expression = readExpression(reader, 2);
            if (reader.skipWhitespace()) {
                if (reader.readOnly('>')) {
                    buffer.push(expression(scope));
                    continue;
                }
                throw new SyntaxError(reader.diagnostic("Expected a termination of the substitution"))
            }
            throw new SyntaxError(reader.diagnostic("Encountered an unterminated substitution"))
        }
        buffer.push(c);
        reader.read();
    }
    fs.writeFileSync(destination, buffer.join(''));
}

function splitArgs(args: string[]): [flags: string[], short: string[], options: string[]] {
    const flags: string[] = [];
    const short: string[] = []
    const options: string[] = [];
    for (const arg of args) {
        if (arg.startsWith("--")) {
            flags.push(arg.substring(2));
            continue;
        }
        if (arg.startsWith("-")) {
            short.push(arg.substring(1));
            continue
        }
        options.push(arg);
    }
    return [flags, short, options];
}

process.exit(main(process.argv.slice(1)))