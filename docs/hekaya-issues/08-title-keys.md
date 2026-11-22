# The title page takes only listed keys

*Draft for github.com/michaelkmalak/hekaya — not filed. For Joop to review.*

**The difference.** `TITLE_KEY` in `rules.ts` matches only the keys in `TITLE_KEYS_AR`/`TITLE_KEYS_EN`, and the title page is dropped if its first line is another key. Fountain accepts any `Key: value` (Producer, Revision, Episode…).

**The case that shows it.**

```
Producer: نادية
العنوان: فيلم
```

The whole title page is read as script body (action).

**Proposed fix.** Accept any key (as Fountain), mapping the known ones.

**What Lexington does now.** Lexington (Fountain tooling with a Hekaya reading, `.hekaya` files read with Hekaya's rules) reads it this way: Any key. If Hekaya decides otherwise, Lexington will follow.
