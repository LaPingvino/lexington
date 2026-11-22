# `== text` is neither a synopsis nor a page break

*Draft for github.com/michaelkmalak/hekaya — not filed. For Joop to review.*

**The difference.** `SYNOPSIS` is `^=(?!=)` and `PAGE_BREAK` `^={3,}\s*$`, so `== الفصل الثاني` is action. Fountain reads a line starting with `=` (not only `=`s, three or more) as a synopsis.

**The case that shows it.**

```
== الفصل الثاني
```

Printed as action.

**Proposed fix.** Read any line starting with `=` that is not a page break as a synopsis (Fountain), and say which wins in the spec.

**What Lexington does now.** Lexington (Fountain tooling with a Hekaya reading, `.hekaya` files read with Hekaya's rules) reads it this way: A synopsis. If Hekaya decides otherwise, Lexington will follow.
