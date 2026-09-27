# Circumscribe

A lightweight macro system in ~1.000 lines of TS.

## Introduction

Circrumscribe is a macro tool that traverses the directory it's run from, searching for specially named files and substituting all placeholders, as defined in `.circumscribe`. For a file to be considered , its **name** has to begin and end in an angle bracket:

```
|- <foo>.json -> foo.json
|- <bar>.md -> bar.md
```

> [!NOTE]
> Initially, when designing Circumscribe, the idea of having a dedicated file extension was considered. However, I found that by having the extention intact, we keep tooling support at its highest, while providing a convention that's unique enough to not trigger unwillingly.

## `.circumscribe`

Similarly to (hopefully) most macroing tools, Compare ships a dedicated macro language that allows you to achieve quite a lot without being over-engineered. The `.circumscribe` file consists of an arbitrary number of **pattern**. Each pattern describes a name-replacement pair. To define a pattern, begin with `def`, followed by the pattern's name, `:=` and the replacement:

```ccs
def VERSION := |1.0.0|
```

The right-hand side of the definition is an **expression**. There are two types of expressions, so-called **snippets** and other patterns. Being a macro language, a "snippet" is the atom of any Compose expression.

A snippet is marked by two vertical bars (`|`) surrounding it and acts like a _multiline string_ in most programming languages. Anything in-between the bars is taken literally from the source, similarly to the `<pre>` element in HTML.

A snippet may include further placeholders in their usual syntax of `<(expression)>`:

```ccs
def VERSION := |0.1.0|
def DESCRIPTION := |A lightweight macro system, v<VERSION>|
```

> [!NOTE]
> Following the C convention, a backslash (`\`) may be used to escape a vertical bar. Or an angle bracket... Or any other usual escape for that matter!
>
> ```ccs
> def DESCRIPTION := |A lightweight macro system \| v<VERSION>| 
> ```

## Roadmap (Please delete me)

- Comments in .circumscribe
- Errors within expressions
    - Error macro?
- 