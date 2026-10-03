# A String-First Language

> [!IMPORTANT]
> This document isn't yet complete. Please refer to src/parser.ts
> for the implementation if some aspects aren't yet discussed well.

When creating circumscribe, the most important aspect of the development process was designing the expression language. Once the first prototype of any language is working, one naturally wants to introduce types, variables, statements, arithmetic, and a dozen other concepts.

A similar story happened to circumscribe. Half-way in creating yet another general-purpose language, I realized that „general-purpose“ wasn‘t the right domain. Circumscribe should be a language **for** strings, not **with** strings.  

## Expressiveness

Similarly to most general-purpose programming languages, circumscribe differentiates between expressions and statements. In circumscribe, however, the only type of a statement is a macro definition, which may only appear at the highest level of a `.circumscribe` file:

```circumscribe
def f(x): ...
```

Similarly to functions in functional programming languages, the body of a macro is a single expression. Since only expressions are evaluated at runtime, circumscribe may well be seen as an expression-first language. The parallel with functional programming, in fact, becomes even more apparent as you consider other parts of circumscribe.

## Expressions

```
if meaningful(env(|foo|)): |bar| else |baz|
```

An expression in circumscribe may be thought of as a single piece of "code" that is evaluated at runtime within a given scope. An expression, however, may become progressively larger due to expression composition: Higher-kinded expressions that accept other ones, as seen above.

### Snippets

The most basic type of an expression in circumscribe is a snippet literal. With strings — snippets — being our one and only type, a snippet literal becomes the **atom** of circumscribe's grammar. Any expression eventually comes down to a snippet literal.

```circumscribe
|Look, mom! I'm a snippet!|
```

A snippet literal is defined as a pair of vertical bars (`|...|`), with anything in-between them taken literally from source, similarly to the `<pre>` element in HTML. Unlike string literals in most programming languages, snippets may include literal newlines.

The choice of vertical bars as delimiters, as opposed to the usual quotes (`"..."`), was made to avoid conflict with any source code commonly included within the snippet literals.

```circumscribe
|{
    "name": "Marie",
    "friends": ...
}|
```

### Applications

The second basic type of expressions in circumscribe are macro applications. An application represents an identifier, followed by an **optional** set of arguments to the macro. Circumscribe does not differentitate between variables and functions.

When referencing what appears to be a "variable", one invokes the macro with no arguments. It is therefore possible to write `x` as `x()`, producing an identical result. This holds for all kinds of variables, including those introduced by higher-kinded expressions.

```circumscribe
foo(|bar|, |baz|)
```

### If-Expressions

The first higher-kinded expression I'd like to introduce (for reasons you'll shortly discover) are **if-expressions**. Similarly to if-statements in any general-purpose programming language, an if-expression evaluates to one of two branches based on some condition. 

Due to the expressive nature of circumscribe, an `else` branch is mandatory, since everything has to evaluate to a snippet.

```circumscribe
if condition: body else fallback
```

It shouldn't take long for any reasonable programmer to question how any boolean algebra could work in a language with snippets being the only type. Indeed, the introduction of if-expressions is the turning point that forces us to introduce **patterns**.

## Patterns (\*_Disrupting your usual reading flow_\*)

Despite circumscribe being a string-first language, it soon became apparent that strings too can be differentiated. One may wish to define an operation on only some chosen literals — such as `true` and `false`. The domain of all possible snippets is, however, infinite.

The intuitive solution is a way to restrict the infinite domain of snippets, creating a subdomain suitable for the operation we're trying to define. Such a restriction upon the domain of all possible snippets is called a **pattern**.

---
### Literals

The most basic type of a pattern is a **literal**. A literal pattern restricts the domain of all possible snippets to only the specific literal. A literal pattern mirrors the syntax of a snippet literal:

```
|foo|
```

### Unions

A single literal pattern isn't much useful on itself. A combination of those, however, may be used to represent any sort of enumeration. A **union pattern** represents a sum-type of an arbitrary amount of patterns.

Since `|` is already long-taken for snippets, and due to the type-theoretical roots of a union pattern, one defines a union by stitching together alternatives with a `+`:

```
|foo| + |bar|
```

### Regular Expressions

When restricting the infinite domain of strings to a specific subset, one naturally considers regular expressions as a way to pattern-match a string to determine whether it should be accepted or not.

It would be a crime to not have a dedicated pattern type for regular expressions. Similarly to literal patterns, regular-expession-patterns mirror the (yet-to-discover) syntax of regular expressions:

```
/\d+/
```
---

Besides being a tool we'll use to define a bunch of expressions, patterns may appear in a macro definition, after each parameter declaration. Circumscribe doesn't perform any compile-time type-checking (consider `foo(env(|xyz|))`), but the patterns **are** reinforced once a call to the macro is made:

```
def foo(x: |foo| + |bar|): ...
```

### Back to If-Expressions...

While it's impossible to define if-expressions for the infinite domain of strings™, we may use our newly acquired tools to restrict the `condition` expression of an if-expressions to a subset of `|true| + |false|`. Similar boolean pattern will be used for other operations later on.

With conditions being restricted to our boolean subset, we define the if-expression to evaluate the "then" branch only when the condition evaluates to `|true|`, and the "else" branch otherwise:

```
if |true|: |What you see...| else |What you don't see...|
```

### Switch-Expressions

Circumscribe too often suffers from long if-else chains. Switch-expressions provide a way to organize those in a more compact form. A switch expression operates upon an operand expression, comparing it to each of the provided cases until one is matched, in-order:

```
switch operand: case |foo|: ... case |bar|: ... default: ...
```

Yet again, given the expression-first constraint, the default branch is mandatory, and serves a double-duty both as a branch and as the gramatical termination of the switch expression.

### With-Expressions

Since expressions may become arbitrary large, and one may wish to reuse the same expression multiple times in a single body, a reasonable programmer's instinct would be to introduce a local variable, capturing and naming the "long expression".

While "variables" aren't a concept in circumscribe, and wouldn't make sense in an expression-first language, the with-expression takes their place. A with-expression evaluates an expression upfront, binds the snippet to a named macro, and finally evaluates the body within the modified scope.

```circumscribe
with |foo| as x: if bar(x): x else |baz|
```

### Match-Expressions

Regular expressions are perhaps the most essential and used tool for text processing. Due to the snippet-first nature of circumscribe, and as hinted by the regular-expression patterns, circumscribe provides a first-class way of dealing with those:

The match-expression attempts to match a source expression against a pattern, binding the capture groups to local macros, as defined in the `as` block, in-order. If successful, the body is evaluated within the modified scope. Otherwise, the else expression is run.

```circumscribe
def reverse_identifier(identifier)
    match /^([a-zA-Z]+)-(\d+)$/ as a, n in identifier:
        |<n>-<a>|
    else |Wrong identifier!| 
```

> In order to demonstrate how much can be achieved with match-statements, I present the standard `trim()` macro, implemented in terms of a match-expression:
>
> ```
> def trim'(x): match /^\s*(.+?)\s*$/ as content in x:
>     match /^\s+$/ in content:
>         ||
>     else content
> else ||
> ```