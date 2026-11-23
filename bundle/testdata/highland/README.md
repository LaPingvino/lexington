# Highland samples

Five `.highland` files from [eugenefischer/highwater-too](https://github.com/eugenefischer/highwater-too)
(a VS Code extension for Highland 2's files), `fixtures/generated/` at commit
370ba0ea7df3c9f738450451fd04b188a56f8be1, unchanged. MIT licence, © 2026 eugenefischer:
see [LICENSE](LICENSE).

| File | Upstream | What it is |
|---|---|---|
| MarkerTest.highland | `MarkerTest.highland` | Saved by Highland 2 itself (2026-10-12), with markers `{{%m: …}}` |
| ScreenplayEdgeCases.highland | `ScreenplayEdgeCases.highland` | Screenplay template, `{{HEADER}}`, `{{FOOTER}}`, `{{INCLUDE}}`, dual dialogue |
| StagePlayProbe.highland | `templates/StagePlayProbe.highland` | Stage Play template (Fountain) |
| ManuscriptStart2.highland | `ManuscriptStart2.highland` | Manuscript template: `text.md`, `net.daringfireball.markdown` |
| PlainTextProbe.highland | `templates/PlainTextProbe.highland` | Plain Text template: `text.txt`, `public.plain.text`, no sidecars |

What they show is this. Only MarkerTest was saved by Highland 2. Highwater-too's
scripts wrote the others the way Highland 2 writes a new document, and Highland 2
then opened and printed them (upstream's `fixtures/README.md`). Each file is a zip
holding one `Name.textbundle/` folder, with no entries for the folders themselves.
Inside the folder are `text.fountain`, `text.md` or `text.txt`, then `info.json`
(compact; `version` 2, `type` the text's UTI, and Highland's settings under
`com.quoteunquoteapps.highland2`, `templateName` among them), and
`characters.json`, `navigatorFilters.json` and `sprints.json` (not for plain
text).
