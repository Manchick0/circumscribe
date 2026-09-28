#!/usr/bin/env node
import fs = require("node:fs");
import { join, relative } from "node:path";
import { readDefinition, readExpression } from "./parser.js";
import { readConfig, type Config } from "./config.js";
import { STANDARD, type Scope } from "./scope.js";
import type { Function } from "./expression.js";
import type { Diagnostic } from "./diagnostic.js";
import { Reader } from "./reader.js";
import { display, pointer } from "./position.js";

const CIRCUMSCRIBED_NAME: RegExp = /^<(.+)>\.(.+)$/
const DOT_CIRCUMSCRIBE: string = join(process.cwd(), ".circumscribe");

const VERSION: string = `circumscribe 0.1.3`
const USAGE: string = `usage: circumscribe [--help | --version] [<command>] [<args>]

Recursively substitute any expressions in all circumscribed files in the current working
directory with their replacements, as defined in '.circumscribe'.
A file is considered circumscribed if its name, that being the part before the last dot,
is surrounded by angle brackets (<example>.txt).

circumscribe            Perform the substitution as described above.

circumscribe init       Create a default .circumscribe file in the current
                        working directory.

circumscribe evaluate   Evaluate the given expression and print the substitution
                        to stdout. The expression is evaluated in the context of
                        the .circumscribe file.

circumscribe --version  Display the current circumscribe version.
                      
circumscribe --help     Print this message.`;

const DEFAULT = `def greet(name): |Hello, <trim(name)>!|

def greeting: greet(|World|)
`

function main(args: string[]): number {
    const config = readConfig();
    if (config[0]) {
        const [flags, options] = splitArgs(args.slice(1));
        if (flags.includes("help")) {
            console.log(USAGE);
            return 0;
        }
        if (flags.includes("version")) {
            console.log(VERSION);
            return 0;
        }
        if (options.length > 0) {
            const option = options[0];
            if (option === 'evaluate') {
                if (options.length > 1) {
                    const scope = loadCircumscribe();
                    if (scope[0]) {
                        const reader = new Reader("--evaluate", options[1]!);
                        const expression = readExpression(reader, 2);
                        if (expression[0]) {
                            const result = expression[0].evaluate(scope[0]);
                            if (result[0] !== undefined) {
                                console.log(result[0]);
                                return 0;
                            }
                            const diagnostic = result[1];
                            if (diagnostic.type === "source") {
                                console.error(display(diagnostic.excerpt));
                                console.error(`${pointer(diagnostic.excerpt)} ${diagnostic.message}`);
                                return 1;
                            }
                            console.error(`${diagnostic.message}`);
                            return 1;
                        }
                        const diagnostic = expression[1];
                        if (diagnostic.type === "source") {
                            console.error(display(diagnostic.excerpt));
                            console.error(`${pointer(diagnostic.excerpt)} ${diagnostic.message}`);
                            return 1;
                        }
                        console.error(`${diagnostic.message}`);
                        return 1;
                    }
                    const diagnostic = scope[1];
                    if (diagnostic.type === "source") {
                        console.error(display(diagnostic.excerpt));
                        console.error(`${pointer(diagnostic.excerpt)} ${diagnostic.message}`);
                        return 1;
                    }
                    console.error(`${diagnostic.message}`);
                    return 1;
                }
                return 0;
            }
            if (option === "init") {
                if (fs.existsSync(DOT_CIRCUMSCRIBE) && !flags.includes("force")) {
                    console.error("circumscribe: .circumscribe already exists in the current working directory. Use '--force' to override it")
                    return 1;
                }
                fs.writeFileSync(DOT_CIRCUMSCRIBE, DEFAULT);
                return 0;
            }
            console.error(`circumscribe: Unknown option '${options[0]}'. See circumscribe --help`);
            return 1;
        }
        if (fs.existsSync(DOT_CIRCUMSCRIBE)) {
            const scope = loadCircumscribe();
            if (scope[0]) {
                const diagnostic = splitSubstitute(config[0].structure.root, config[0], scope[0]);
                if (diagnostic) {
                    if (diagnostic.type === "source") {
                        console.error(display(diagnostic.excerpt));
                        console.error(`${pointer(diagnostic.excerpt)} ${diagnostic.message}`);
                        return 1;
                    }
                    console.error(`${diagnostic.message}`);
                    return 1;
                }
                return 0;
            }
            const diagnostic = scope[1];
            if (diagnostic.type === "source") {
                console.error(display(diagnostic.excerpt));
                console.error(`${pointer(diagnostic.excerpt)} ${diagnostic.message}`);
                return 1;
            }
            console.error(`${diagnostic.message}`);
            return 1;
        }
        console.error("circumscribe: Couldn't find the .circumscribe file in the current working directory. Perhaps circumscribe init?");
        return 1;
    }
    console.error(`circumscribe: Couldn't load 'circumscribe.json': ${config[1]}`);
    return 1;
}

/**
 * Load the `.circumscribe` file in the current working directory.
 * 
 * ---
 * Attempts to load the patterns defined in the `.circumscribe` file in the current working directory.
 * If an error occurs while parsing the content of the file, `[undefined, [source, diagnostic]]` is returned. Otherwise,
 * `[scope, undefined]` is returned, where `scope` is a properly filled with definitions {@linkcode Scope}. 
 * 
 * @returns A {@linkcode Scope}, or a {@linkcode Diagnostic} if an error occurs.
 */
function loadCircumscribe(): [Scope, undefined] | [undefined, Diagnostic] {
    const content = fs.readFileSync(DOT_CIRCUMSCRIBE, "utf8");
    const reader = new Reader(relative(process.cwd(), DOT_CIRCUMSCRIBE), content);
    const entries: Record<string, Function> = {};
    for (; ;) {
        const pattern = readDefinition(reader);
        if (pattern[1])
            return [undefined, pattern[1]];
        if (pattern[0]) {
            entries[pattern[0].identifier] = pattern[0];
            continue;
        }
        break;
    }
    const buffer: any = {
        parent: STANDARD,
        entries: entries
    };
    buffer["root"] = buffer;
    return [buffer, undefined]
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
function splitSubstitute(path: string, config: Config, scope: Scope): Diagnostic | undefined {
    const entries = fs.readdirSync(path, { withFileTypes: true });
    const { structure, mirror } = config;
    for (const entry of entries) {
        const twin = join(structure.build, relative(structure.root, path));
        const source = join(path, entry.name);
        fs.mkdirSync(twin, { recursive: true });
        if (entry.isFile()) {
            const name = CIRCUMSCRIBED_NAME.exec(entry.name);
            if (name) {
                const diagnostic = substitute(source, join(twin, `${name[1]}.${name[2]}`), scope);
                if (diagnostic)
                    return diagnostic;
                continue;
            }
            if (mirror)
                fs.copyFileSync(source, join(twin, entry.name));
            continue;
        }
        const diagnostic = splitSubstitute(source, config, scope);
        if (diagnostic)
            return diagnostic;
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
function substitute(path: string, destination: string, scope: Scope): Diagnostic | undefined {
    const content = fs.readFileSync(path, "utf8");
    const reader = new Reader(relative(process.cwd(), path), content);
    const buffer = [];
    while (reader.canRead()) {
        const c = reader.peek();
        if (reader.isAt('<')) {
            const position = reader.position();
            const expression = readExpression(reader.chainRead(), 2);
            if (expression[0]) {
                if (reader.skipWhitespace()) {
                    if (reader.readOnly('>')) {
                        const snippet = expression[0].evaluate(scope);
                        if (snippet[0] !== undefined) {
                            buffer.push(snippet[0]);
                            continue;
                        }
                        return snippet[1];
                    }
                    return { type: "source", excerpt: reader.excerpt(position), message: "Expected '>'" }
                }
                return { type: "source", excerpt: reader.excerpt(position), message: "Encountered an incomplete substitution" }
            }
            return expression[1];
        }
        buffer.push(c);
        reader.read();
    }
    fs.writeFileSync(destination, buffer.join(''));
    return undefined;
}

function splitArgs(args: string[]): [flags: string[], options: string[]] {
    const flags: string[] = [];
    const options: string[] = [];
    for (const arg of args) {
        if (arg.startsWith("--")) {
            flags.push(arg.substring(2));
            continue;
        }
        options.push(arg);
    }
    return [flags, options];
}

process.exit(main(process.argv.slice(1)))