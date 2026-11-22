# `عودة للمشهد` is BACK TO SCENE in the spec, BACK TO in the code

*Draft for github.com/michaelkmalak/hekaya — not filed. For Joop to review.*

**The difference.** Spec: `عودة للمشهد` = BACK TO SCENE. `TRANSITION_MAP_AR_EN`: `'عودة للمشهد': 'BACK TO'` (the same as `عودة إلى`).

**The case that shows it.**

```
- عودة للمشهد -
```

Converted to English it loses SCENE.

**Proposed fix.** `'عودة للمشهد': 'BACK TO SCENE'` in `TRANSITION_MAP_AR_EN`.

**What Lexington does now.** Lexington (Fountain tooling with a Hekaya reading, `.hekaya` files read with Hekaya's rules) reads it this way: A transition, kept as written. If Hekaya decides otherwise, Lexington will follow.
