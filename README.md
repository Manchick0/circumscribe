# Circumscribe

A general-purpose macro processing tool in ~1.000 lines of TS.

## Introduction

Circrumscribe is a tool that traverses the directory it's run from, searching for “_cicumscribed_“ files, and substitutes all expressions within them. For a file to be considered “_circumscribed_“, its name has to be surrounded an angle brackets — it has to be written around — it has to be **circumscribed**:

```
|- <foo>.json -> foo.json
|- <bar>.md -> bar.md
```

> [!NOTE]
> When initially designing Circumscribe, the idea of having a dedicated file extension was considered. However, I found that by having the extention intact, we keep tooling support at its highest, while providing a convention that's unique enough to not trigger unwillingly.
> ...And gain a whole bunch of personality!

## `.circumscribe`

Similarly to (hopefully) most macroing tools, circumscribe ships a dedicated macro language that allows you to achieve quite a lot despite its simplicity. At its core lies a philosophy of "everything is a snippet". Indeed, the only type present in circumscribe are snippets.

The `.circumscribe` file consists of an arbitrary number of macro definitions. Each definition describes a name-replacement pair. To define a macro, begin with `def`, followed by a name, `:` and the replacement:

```circumscribe
def version: |0.1.0|
```

The right-hand side of the definition is an **expression**. The most basic type of an expression is a so-called **snippet**.

A snippet is marked by two vertical bars (`|`) surrounding it and acts as a _multiline string_ in most programming languages. Anything in-between the bars is taken literally from the source, similarly to the `<pre>` element in HTML.

A snippet may include further placeholders in their usual syntax of `<(expression)>`:

```circumscribe
def version: |0.1.0|
def description: |A lightweight macro system, v<version>|
```

> [!NOTE]
> Following the C convention, a backslash (`\`) may be used to escape a whole bunch of characters within snippet literals. Outside of snippets, however, such as within circumscribed files, special backslash handling could introduce incompatibilites with the rest of the source.
> 
> Since `<` is the only specially-handled sequence within circumscribed sources, one may wish to escape it. To do, given the lack of backslash-escapes, we recommend: Not escaping it at all. Instead, embrace the replacement logic:
>
> ```circumscribe
> # <math-homework>.txt
> 5 <|<|> 7
> ```
