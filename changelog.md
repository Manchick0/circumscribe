# circumscribe 0.2.0
> November 21, 2026

Overhauled 'circumscribe infer' and enhance error messages
across the codebase.

- The 'circumscribe infer' subcommand now accepts a function
  by an identifier, instead of a general expression, and reports
  its parameters, return pattern, and purity.

- The pattern pretty-printing was enhanced as a result of
  'circumscribe infer' changes. Patterns now only use parentheses
  when necessary, and a redundant 'e' no longer appears in snippet
  product patterns.

- The ':' interval operator was reverted back to '-', due to its
  previously unnoticed conflict with the body of a macro.

# circumscribe 0.1.1
> November 21, 2026

Resolved the issue with incorrectly outputting the machines
on a pattern mismatch instead of the respective patterns.

# circumscribe 0.1.0
> November 21, 2026

The initial, installable version of circumscribe. It's hard to keep
track of everything that was added up to this point, so we'll mark
0.1.0 as the origin, with virtually to no changes.

- The 'circumscribe.py' script should ease the installation process.
  Though, frankly, I have very little experience with both installation
  and Python, so changes will very likely follow.

- The 'circumscribe.tex' manual isn't yet complete! It contains some
  helpful information, but doesn't cover the whole language. Please
  refer to the implementation for now!

