# Scene headings: does a blank line have to come before?

*Draft for github.com/michaelkmalak/hekaya — not filed. For Joop to review.*

**The difference.** Fountain reads a scene heading only after a blank line (a forced `.` heading anywhere). The spec does not say; `lexer.ts` tests `SCENE_HEADING` on every line, also right after action or dialogue.

**The case that shows it.**

```
قهوة بلدي قديمة.
داخلي البيت هادئ.
```

`داخلي البيت هادئ.` ("inside, the house is quiet") right under an action line becomes a heading.

**Proposed fix.** Follow Fountain: a heading keyword starts a heading only after a blank line; say so in the spec.

**What Lexington does now.** Lexington (Fountain tooling with a Hekaya reading, `.hekaya` files read with Hekaya's rules) reads it this way: A blank line before (Fountain). If Hekaya decides otherwise, Lexington will follow.
