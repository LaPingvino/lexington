# Keyword transitions: blank line before only, Fountain wants them around

*Draft for github.com/michaelkmalak/hekaya — not filed. For Joop to review.*

**The difference.** The spec: transitions are standalone keywords "preceded by a blank line". Fountain's transitions have blank lines before and after. With only the line before checked, a short line of action or dialogue that happens to be a keyword (`انتقال`, `مسح`, `وميض`, `قطع`) is a transition.

**The case that shows it.**

```
قطع
الخبز بالسكين وقدمه لها.
```

The first line of a paragraph that starts with the keyword on its own line becomes a transition.

**Proposed fix.** Require a blank line after as well (as Fountain), in the spec and the lexer.

**What Lexington does now.** Lexington (Fountain tooling with a Hekaya reading, `.hekaya` files read with Hekaya's rules) reads it this way: Blank lines around (Fountain). If Hekaya decides otherwise, Lexington will follow.
