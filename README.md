# Circumscribe

A general-purpose macro processing tool in ~1.000 lines of TS.

## Introduction

Circrumscribe is a tool that traverses the directory it's run from, searching for “_cicumscribed_“ files and substituting all expressions within them. For a file to be considered “_circumscribed_“, its name has to be surrounded an angle brackets — it has to be written around — it has to be **circumscribed**:

```
|- <foo>.json -> foo.json
|- <bar>.md -> bar.md
```

> [!NOTE]
> When initially designing Circumscribe, the idea of having a dedicated file extension was considered. However, I found that by having the extention intact, we keep tooling support at its highest, while providing a convention that's unique enough to not trigger unwillingly.

## `.circumscribe`

Similarly to (hopefully) most macroing tools, circumscribe ships a dedicated macro language that allows you to achieve quite a lot without being over-engineered. The `.circumscribe` file consists of an arbitrary number of macros. Each macro describes a name-replacement pair. To define a macro, begin with `def`, followed by the pattern's name, `:` and the replacement:

```ccs
def VERSION: |0.1.0|
```

The right-hand side of the definition is an **expression**. The simplest type of an expression is a **snippet**.

A snippet is marked by two vertical bars (`|`) surrounding it and acts like a _multiline string_ in most programming languages. Anything in-between the bars is taken literally from the source, similarly to the `<pre>` element in HTML.

A snippet may include further placeholders in their usual syntax of `<(expression)>`:

```ccs
def VERSION: |0.1.0|
def DESCRIPTION: |A lightweight macro system, v<VERSION>|
```

> [!NOTE]
> Following the C convention, a backslash (`\`) may be used to escape a vertical bar. Or an angle bracket... Or any other usual escape for that matter!
>
> ```ccs
> def DESCRIPTION: |A lightweight macro system \| v<VERSION>| 
> ```
