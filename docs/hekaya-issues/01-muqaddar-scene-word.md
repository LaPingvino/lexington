# `مقدر` is a scene heading keyword on the compatibility page only

*Draft for github.com/michaelkmalak/hekaya — not filed. For Joop to review.*

**The difference.** The Fountain Compatibility page lists `مقدر` among the Arabic scene heading keywords ("`داخلي`, `خارجي`, `مقدر`, `داخلي/خارجي`, `د/خ`"), but the spec's keyword table and `SCENE_HEADING_KEYWORDS_AR` in `keywords.ts` do not have it.

**The case that shows it.**

```
مقدر له أن يعود إلى البيت.
```

As a keyword, this action line (after a blank line) would read as a scene heading.

**Proposed fix.** Remove `مقدر` from the compatibility page, or, if it is meant (a term for a kind of scene?), add it to the spec with its meaning and to `keywords.ts`.

**What Lexington does now.** Lexington (Fountain tooling with a Hekaya reading, `.hekaya` files read with Hekaya's rules) reads it this way: Not a scene word: the line above is action. If Hekaya decides otherwise, Lexington will follow.
