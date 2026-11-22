# Direction of a mixed script: "determined by title page language" is not defined

*Draft for github.com/michaelkmalak/hekaya — not filed. For Joop to review.*

**The difference.** The spec: "Mixed → determined by title page language", but no title key names a language and no rule says how the title page's language is found.

**The case that shows it.**

```
Title: My Film
سيناريو: سمير

...
```

Two implementations can lay the same script out in opposite directions.

**Proposed fix.** Say: each line runs by its own first strong letter (Unicode bidi), and only lines without one take `Direction:`, else the language's default.

**What Lexington does now.** Lexington (Fountain tooling with a Hekaya reading, `.hekaya` files read with Hekaya's rules) reads it this way: Each line by its own letters; a line with none by `Direction:`, else right to left. If Hekaya decides otherwise, Lexington will follow.
