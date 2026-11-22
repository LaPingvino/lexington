# Lexington and Accolade read and write .hekaya; a monospaced font for both scripts

*Draft note for github.com/michaelkmalak/hekaya — not filed. For Joop to review.*

Hello! Thank you for Hekaya. Lexington (a Go Fountain toolkit:
PDF, HTML, FDX…) and Accolade (a screenwriting app built on it) now read and
write `.hekaya`: a `.hekaya` file is read with Hekaya's rules — the Arabic
scene words, the keyword transitions with their dashes, the extensions, the
Arabic title keys, the Character Registry, `Direction:` — and written back
as it was typed. While implementing it we wrote down the places where the
spec, the code and the compatibility page differ, each as its own issue.

A note on fonts: `@hekaya/pdf` pairs Courier Prime (English) with Cascadia
Mono (Arabic). Courier Badi (https://github.com/LaPingvino/courier-badi,
OFL) is Courier Prime with Arabic (and pointed Hebrew) added in the same
monospaced cell, with generated Arabic shaping (GSUB joining forms,
lam-alef): one screenplay font for both scripts, so a mixed line keeps one
rhythm. Lexington's PDF shapes Arabic through Courier Badi's own OpenType
features. It might be useful for Hekaya's PDF too.
