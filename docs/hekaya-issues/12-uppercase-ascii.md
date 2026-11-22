# English character detection is ASCII-only

*Draft for github.com/michaelkmalak/hekaya — not filed. For Joop to review.*

**The difference.** The spec says "Standard Fountain UPPERCASE detection works for English names", but `CHARACTER_ENGLISH` is `[A-Z][A-Z0-9 _\-']+`: names with capitals beyond ASCII are not cues.

**The case that shows it.**

```
JOSÉ
Hola.
```

Read as action and dialogue-less text.

**Proposed fix.** Use Unicode capitals (`\p{Lu}`), as Fountain does.

**What Lexington does now.** Lexington (Fountain tooling with a Hekaya reading, `.hekaya` files read with Hekaya's rules) reads it this way: Any capitals: `JOSÉ` is a cue. If Hekaya decides otherwise, Lexington will follow.
