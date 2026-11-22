# `مزج إلى` is a transition in the code but not in the spec

*Draft for github.com/michaelkmalak/hekaya — not filed. For Joop to review.*

**The difference.** `TRANSITION_KEYWORDS_AR` has `مزج إلى` (DISSOLVE TO); the spec's transition table has `مزج` but not `مزج إلى`, while it does list the parallel `مسح إلى` and `وميض إلى`.

**The case that shows it.**

```
- مزج إلى -
```

Writers reading the spec cannot know it is a transition; other implementations following the spec read it as action.

**Proposed fix.** Add `مزج إلى` | DISSOLVE TO to the spec's table.

**What Lexington does now.** Lexington (Fountain tooling with a Hekaya reading, `.hekaya` files read with Hekaya's rules) reads it this way: A transition. If Hekaya decides otherwise, Lexington will follow.
