# Scene numbers in Arabic digits and renumbering

*Draft for github.com/michaelkmalak/hekaya — not filed. For Joop to review.*

**The difference.** The spec accepts `#١#` and `#1#` but says nothing of which digits a tool writes when it numbers or renumbers scenes.

**The case that shows it.**

```
داخلي - قهوة بلدي - نهار #١٢#
```

A tool renumbering a script mixes digits.

**Proposed fix.** Say which digits numbering writes (perhaps those the script already uses).

**What Lexington does now.** Lexington (Fountain tooling with a Hekaya reading, `.hekaya` files read with Hekaya's rules) reads it this way: Kept as written; its renumbering writes Western digits. If Hekaya decides otherwise, Lexington will follow.
