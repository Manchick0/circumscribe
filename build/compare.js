import { createRequire as _createRequire } from "module";
const __require = _createRequire(import.meta.url);
const fs = __require("node:fs");
import { dirname, join, relative } from "node:path";
import { Reader } from "./reader.js";
import { STANDARD } from "./scope.js";
import { readPattern, readExpression } from "./parser.js";
const COMPARE = "<compare>";
const ESCAPED_NAME = /^<(.+?)>(\..+)$/;
function main(args) {
    const [flags, short, options] = splitArgs(args.slice(1));
    const compare = join(process.cwd(), COMPARE);
    if (fs.existsSync(compare)) {
        const content = fs.readFileSync(compare, "utf8");
        const reader = new Reader(COMPARE, content);
        const entries = {};
        for (;;) {
            const pattern = readPattern(reader);
            if (pattern) {
                entries[pattern[0]] = pattern[1];
                continue;
            }
            break;
        }
        const scope = { parent: STANDARD, entries };
        if (flags.includes("expand") || short.includes("e")) {
            if (options.length > 0) {
                const reader = new Reader("--expand", options[0]);
                const expression = readExpression(reader, 2);
                console.log(expression(scope));
                return 0;
            }
            console.error("Expected an expression");
            return 0;
        }
        return 0;
    }
    console.error("Couldn't find the <compare> file in the current working directory");
    return 1;
}
// function inplaceSubstitute() {
// }
// function splitSubstitute(path: string, root: string, destination: string) {
//     const entries = fs.readdirSync(path, { withFileTypes: true });
//     for (const entry of entries) {
//         const file = join(path, entry.name);
//         const dest = join(destination, relative(root, path));
//         fs.mkdirSync(dest, { recursive: true });
//         if (entry.isFile()) {
//             const name = ESCAPED_NAME.exec(entry.name);
//             if (name) {
//                 subsitute(file, `${name[1]}${name[2]}`, root);
//                 continue;
//             }
//             fs.copyFileSync(file, dest);
//             continue;
//         }
//         splitSubstitute();
//     }
// }
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
function subsitute(path, destination, scope) {
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
                throw new SyntaxError(reader.diagnostic("Expected a termination of the substitution"));
            }
            throw new SyntaxError(reader.diagnostic("Encountered an unterminated substitution"));
        }
        buffer.push(c);
        reader.read();
    }
    fs.writeFileSync(destination, buffer.join(''));
}
function splitArgs(args) {
    const flags = [];
    const short = [];
    const options = [];
    for (const arg of args) {
        if (arg.startsWith("--")) {
            flags.push(arg.substring(2));
            continue;
        }
        if (arg.startsWith("-")) {
            short.push(arg.substring(1));
            continue;
        }
        options.push(arg);
    }
    return [flags, short, options];
}
process.exit(main(process.argv.slice(1)));
