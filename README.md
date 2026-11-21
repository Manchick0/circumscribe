# circumscribe

```bash
curl https://github.com/Manchick0/circumscribe/blob/master/circumscribe.py?raw=true | python3
```

A general-purpose macro tool. When run against a directory, circumscribe recursively
locates all ”circumscribed“ files and substitutes all placeholders within them. In
order for a file to be considered circumscribed, its name has to be surrounded with
a set of angle brackets, visually written around, hence the name:

```bash
<foo>.c
```

A circumscribed file may contain placeholders in the form of `<expression>`.
When processed, all placeholders are substituted and concatenated to the
rest of the source:

```c
// <foo>.c
#include <stdio.h>

int main(void) {
    printf("<greeting>");
    return 0;
}
```

The so-called “context files” are responsible for all definitions that may
subsequently appear in circumscribed files. Unless others are specified with
the `—c` flag, the default context file is `.circumscribe` in the current
working directory.

```circumscribe
# .circumscribe
def greeting: greet(|Marie|)

def greet(name):
    |Bonjour, <name>!|
```

## Language

Circumscribe features a functional, domain-specific, statically-typed macro
language, dually named circumscribe. With circumscribe being a macro tool,
the language was designed with an interesting constraint in-mind: _Everything
is a string_.

In order to compensate for the lack of types, circumscribe has a notion of
patterns. A pattern represents a single formal language and serves a duty
identical to that of a type. With patterns, advanced inference and context
sensitive type narrowing, circumscribe achieves complete type-safety at
“compile time”:

```circumscribe
def greet(name: any) -> |Hello, <name>!|

def who(greting: |Hello| * any * |!|):
    match |Hello, <name: any>!|:
        name
    else raise |...|
```
