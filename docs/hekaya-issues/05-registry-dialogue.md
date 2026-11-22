# The Character Registry: "followed by dialogue" is not what the code checks

*Draft for github.com/michaelkmalak/hekaya — not filed. For Joop to review.*

**The difference.** The spec: once registered, "subsequent standalone lines matching that name (followed by dialogue) are auto-detected". `lexer.ts` checks only that some later line is not blank (`findNextNonBlankLine`), so a registered name followed by a blank line becomes a cue with no dialogue. Fountain requires the dialogue right under the name.

**The case that shows it.**

```
@سمير
أهلا.

سمير

سمير يمشي في الشارع.
```

The second `سمير` (a one-word action paragraph) is read as a cue without speech.

**Proposed fix.** Require the next line itself to be non-blank (as Fountain does for a character), and say so in the spec.

**What Lexington does now.** Lexington (Fountain tooling with a Hekaya reading, `.hekaya` files read with Hekaya's rules) reads it this way: The speech right under it; otherwise action. If Hekaya decides otherwise, Lexington will follow.
