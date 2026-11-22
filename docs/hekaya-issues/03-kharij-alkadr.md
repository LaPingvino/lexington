# `خارج الكادر` is O.C. in the spec, O.S. in the code

*Draft for github.com/michaelkmalak/hekaya — not filed. For Joop to review.*

**The difference.** The spec's extension table: `خارج الكادر` = O.C. (off camera). `CHARACTER_EXTENSIONS_AR` maps it to `O.S.`.

**The case that shows it.**

```
@نادية (خارج الكادر)
```

Exported to English (or FDX) the extension becomes O.S. in one reading and O.C. in the other.

**Proposed fix.** Map it to `O.C.` in `keywords.ts`, as the spec says (O.S. already has `خارج الشاشة` and `خ.ش`).

**What Lexington does now.** Lexington (Fountain tooling with a Hekaya reading, `.hekaya` files read with Hekaya's rules) reads it this way: Reads it as an extension and keeps it as written; documents it as O.C. If Hekaya decides otherwise, Lexington will follow.
