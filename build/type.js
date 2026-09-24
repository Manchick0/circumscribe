export const NUMBER = {
    satisfies: (value) => {
        return typeof value === "number";
    },
    stringify(value) {
        return value.toString();
    }
};
export const BOOLEAN = {
    satisfies: (value) => {
        return typeof value === "boolean";
    },
    stringify(value) {
        return value.toString();
    }
};
export const SNIPPET = {
    satisfies: (value) => {
        return typeof value === "string";
    },
    stringify(value) {
        return value;
    }
};
