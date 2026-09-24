export function pattern(parameters, expression) {
    return (env, args) => {
        const entries = {};
        for (let i = 0; i < args.length; i++) {
            const parameter = parameters[i];
            const argument = args[i];
            entries[parameter] = () => argument;
        }
        return expression({
            parent: env,
            entries: entries
        });
    };
}
