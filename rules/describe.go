package rules

// What every setting is, for programs that show the configuration to
// people (Accolade's settings and format editors, a help page): each
// field's key as the TOML writes it, its kind, what it does and the
// values it takes. The defaults are the configuration's own
// (DefaultConf, Default, Language, DefaultExtensions, the presets).
// A test checks that every field of TOMLConf, Language, Shape, Format
// and a format file, every element key and every cue rule is described.

// A Field is one setting.
type Field struct {
	Key string // as the TOML writes it
	// Kind is what it holds: "text", "number" (a decimal), "integer",
	// "bool", "list" (of text), "choice" (one of Choices), "choices"
	// (a list of Choices), "table" (keys to text), "tables" (named
	// tables of Fields of their own).
	Kind    string
	Doc     string
	Choices []string
}

// ConfFields are the parts of a configuration file (lexington.toml).
// Legacy ones are read and never written.
func ConfFields() []Field {
	return []Field{
		{Key: "Elements", Kind: "tables", Doc: "Element sets: how each kind of line is laid out on the page, by set (-e on the command line). " +
			"A set named like a preset takes its place; a set is whole, so an element it leaves out is not printed. " +
			"Formats of their own are also files in a folder of format files (-formats)."},
		{Key: "Languages", Kind: "tables", Doc: "How scripts in each language are written: scene words, heading patterns, cue and transition rules, " +
			"title page keys, direction. A field a language leaves out is the default's (English's rules for a language Lexington does not know). " +
			"A file named script.eo.fountain is read with Esperanto's."},
		{Key: "Standard", Kind: "text", Doc: "The language whose scene words are standard Fountain's, what other Fountain apps read " +
			"(en if none). A script in another language is a dialect of Fountain, written for them with -force."},
		{Key: "Extensions", Kind: "table", Doc: "File extensions (without the dot) whose files are in a language: hekaya = \"hekaya\" " +
			"reads film.hekaya with Hekaya's rules. The default's are known in every configuration; one of one's own adds to them or changes one."},
		{Key: "TitlePages", Kind: "tables", Doc: "Title page templates, by name: where each title page key goes (centred under the title, " +
			"at the top, at the foot left or right, or anywhere), in what size and style, and a picture. A key a template does not name is " +
			"placed by the language's TitleCentred. Standard, bold and poster are known without being written; one of one's own named so takes its place."},
		{Key: "Scenes", Kind: "table", Doc: "Legacy: scene words by language, from before [Languages]. Read into Languages, never written."},
		{Key: "Matchups", Kind: "table", Doc: "Legacy: scene words by role, by language, from before [Languages]. Read into Languages, never written."},
		{Key: "Directions", Kind: "table", Doc: "Legacy: the direction by language, from before [Languages]. Read into Languages, never written."},
	}
}

// LanguageFields are the fields of a language ([Languages.xx]), in the
// order of Language.
func LanguageFields() []Field {
	return []Field{
		{Key: "Scenes", Kind: "list", Doc: "The words a scene heading starts with (INT, EXT), followed by a dot or a space " +
			"(or anything, for a word that ends in a sign)."},
		{Key: "Headings", Kind: "list", Doc: "Scene heading patterns for headings that do not start with a scene word, " +
			"with the slots {n} (a scene number), {io} (one of Settings or Scenes), {time} (one of Times), {place} and {any} (any text): " +
			"\"{n}. {place} {io}/{time}\"."},
		{Key: "NewHeading", Kind: "text", Doc: "The pattern a new scene heading is written by (Accolade's Insert > New Scene…), with the slots of Headings: " +
			"one of Headings, or one that starts with a scene word (\"{io} {place} - {time}\", Fountain's INT. KITCHEN - DAY; \"○{place}（{time}）\"). " +
			"Empty: the first of Headings, else Fountain's. Only for writing: Scenes and Headings are what a heading is read by."},
		{Key: "Settings", Kind: "list", Doc: "The words for inside and outside within a heading (INNEN, INT), for the {io} slot."},
		{Key: "Times", Kind: "list", Doc: "The words for the time of day (TAG, JOUR), for the {time} slot."},
		{Key: "Matchup", Kind: "list", Doc: "The scene words by role, in this order: inside, outside, both, both the other way, establishing " +
			"(\"\" for one it has none for): what translates to what, and what the statistics count."},
		{Key: "Direction", Kind: "choice", Choices: []string{"ltr", "rtl"}, Doc: "The way its text runs: what a line with no direction of its own takes."},
		{Key: "Cues", Kind: "choices", Choices: CueRules, Doc: "The rules that make a line a character's cue (forced, @NAME, always counts)."},
		{Key: "Extensions", Kind: "list", Doc: "The words for a cue's extensions (V.O., O.S., CONT'D), as the extension rule reads them."},
		{Key: "Shape", Kind: "tables", Doc: "What a name looks like to the shape rule."},
		{Key: "Repeat", Kind: "integer", Doc: "How often a name must pass shape for the repeat rule."},
		{Key: "TransitionEnds", Kind: "list", Doc: "What a transition line ends in (\" TO:\")."},
		{Key: "Transitions", Kind: "list", Doc: "Transitions as a whole line (\"FADE OUT.\")."},
		{Key: "TransitionMarks", Kind: "text", Doc: "Signs that may stand around one of Transitions (\"-\" for \"- CUT -\")."},
		{Key: "TitleKeys", Kind: "table", Doc: "The language's title page keys and the standard key each one is (TITEL = Title). " +
			"The standard keys themselves are known in every language."},
		{Key: "TitleCentred", Kind: "list", Doc: "The standard title page keys printed centred under the title, in this order; the rest go to the page's foot. " +
			"Empty: Fountain's (Title, Credit, Author, Source). What places a key when no title page template is chosen, or one the template does not name."},
	}
}

// ShapeFields are the fields of a language's Shape.
func ShapeFields() []Field {
	return []Field{
		{Key: "Words", Kind: "integer", Doc: "At most this many words (0: any number)."},
		{Key: "Chars", Kind: "integer", Doc: "At most this many characters, spaces not counted (0: any number)."},
		{Key: "Ends", Kind: "text", Doc: "None of these characters, the sentence ends (\".!?\")."},
	}
}

// CueRuleFields are the cue rules, by name (Language.Cues).
func CueRuleFields() []Field {
	return []Field{
		{Key: CueForced, Kind: "bool", Doc: "@ before the name: always a cue."},
		{Key: CueCaps, Kind: "bool", Doc: "The name in capitals: it has capital letters and no small ones (Fountain's own)."},
		{Key: CueList, Kind: "bool", Doc: "The name is in the script's character list, or was read as a cue before."},
		{Key: CueParenthetical, Kind: "bool", Doc: "A parenthetical follows it directly."},
		{Key: CueExtension, Kind: "bool", Doc: "It ends in one of the language's Extensions in brackets: \"ANNA (V.O.)\"."},
		{Key: CueShape, Kind: "bool", Doc: "It is shaped like a name (Shape)."},
		{Key: CueRepeat, Kind: "bool", Doc: "It passes shape at least Repeat times in the script."},
	}
}

// FormatFields are the fields of an element's format (Format).
func FormatFields() []Field {
	return []Field{
		{Key: "Right", Kind: "number", Doc: "The right margin, in inches from the paper's right edge."},
		{Key: "Left", Kind: "number", Doc: "The left margin, in inches from the paper's left edge."},
		{Key: "Font", Kind: "text", Doc: "The font: CourierPrime (the default), Helvetica, Times, another built-in font's name, or a .ttf/.otf file."},
		{Key: "Style", Kind: "text", Doc: "b bold, i italic, u underlined, joined (\"bu\"). For notecolors, how colors show " +
			"(line, marker, background, text, with +block, +tint, +patterns); for pagination, what is on (all, none, or keep+widows+more)."},
		{Key: "Size", Kind: "number", Doc: "The font size in points (12 if none)."},
		{Key: "Hide", Kind: "bool", Doc: "Not printed. For notecolors and pagination: off."},
		{Key: "Align", Kind: "choice", Choices: []string{"L", "C", "R"}, Doc: "Left (the default), centred or right."},
		{Key: "Prefix", Kind: "text", Doc: "Printed before the line's text."},
		{Key: "Postfix", Kind: "text", Doc: "Printed after the line's text (\":\" after a comic's speaker)."},
	}
}

// ElementKeys are the elements of a set, in the order a script has
// them; notecolors and pagination are switches (IsSwitch), not lines.
func ElementKeys() []Field {
	return []Field{
		{Key: KeyTitle.String(), Kind: "tables", Doc: "The title page's title."},
		{Key: KeyMeta.String(), Kind: "tables", Doc: "The title page's other lines (credit, author, contact)."},
		{Key: KeyScene.String(), Kind: "tables", Doc: "Scene headings."},
		{Key: KeyAction.String(), Kind: "tables", Doc: "Action, and what has no format of its own."},
		{Key: KeySpeaker.String(), Kind: "tables", Doc: "A character's cue, the name above dialogue."},
		{Key: KeyParen.String(), Kind: "tables", Doc: "Parentheticals in dialogue: (quietly)."},
		{Key: KeyDialog.String(), Kind: "tables", Doc: "Dialogue."},
		{Key: KeyDualSpeaker.String(), Kind: "tables", Doc: "A cue in dual dialogue (side by side), relative to its column."},
		{Key: KeyDualParen.String(), Kind: "tables", Doc: "A parenthetical in dual dialogue, relative to its column."},
		{Key: KeyDualDialog.String(), Kind: "tables", Doc: "Dialogue in dual dialogue, relative to its column."},
		{Key: KeyTrans.String(), Kind: "tables", Doc: "Transitions: CUT TO:."},
		{Key: KeyCenter.String(), Kind: "tables", Doc: "Centred text: >THE END<."},
		{Key: KeyLyrics.String(), Kind: "tables", Doc: "Lyrics: lines starting with ~."},
		{Key: KeySection.String(), Kind: "tables", Doc: "Sections (# Act One): printed only with a visible rule."},
		{Key: KeyAllCaps.String(), Kind: "tables", Doc: "Action written all in capitals."},
		{Key: KeyEmpty.String(), Kind: "tables", Doc: "Empty lines."},
		{Key: KeyPrintNotes.String(), Kind: "tables", Doc: "Lines that are only a [[note]]: printed only with a visible rule."},
		{Key: KeyNoteColors.String(), Kind: "tables", Doc: "A switch: scene headings and sections in the color their notes give ([[red]]); its Style is how."},
		{Key: KeyPagination.String(), Kind: "tables", Doc: "A switch: how page breaks keep things together; its Style says what is on " +
			"(keep: headings and cues go over with what follows; widows: two lines of a paragraph on each page; more: (MORE) and CONT'D)."},
	}
}

// TitlePageFields are the fields of a title page template
// ([TitlePages.name]), in the order of TitlePage.
func TitlePageFields() []Field {
	return []Field{
		{Key: "Name", Kind: "text", Doc: "The template's name, as lists show it (its key if none)."},
		{Key: "Description", Kind: "text", Doc: "What the template is for."},
		{Key: "Centre", Kind: "number", Doc: "Where the centred block starts, in inches from the top of a Letter page (0: 4, Fountain's)."},
		{Key: "Keys", Kind: "tables", Doc: "The keys it places, a [[TitlePages.name.Keys]] table each, each place's in this order. " +
			"Key \"*\" is every key it does not name."},
	}
}

// TitleKeyFields are the fields of a template's key
// ([[TitlePages.name.Keys]]), in the order of TitleKey.
func TitleKeyFields() []Field {
	return []Field{
		{Key: "Key", Kind: "text", Doc: "The title page key: a standard one (Title, Credit, Author, Source, Draft date, Contact) or one of " +
			"one's own (Logline, Registration); case does not count. \"*\": every key the template does not name."},
		{Key: "Place", Kind: "choice", Choices: append([]string{""}, Places...), Doc: "Where it goes: centre (the block under the title), " +
			"top, foot-left, foot-right, or free (at X, Y). Empty: the language's (TitleCentred: centre, else the foot on the left)."},
		{Key: "X", Kind: "number", Doc: "A free place's distance from the page's left edge, in inches on Letter (0 and centred: the middle)."},
		{Key: "Y", Kind: "number", Doc: "A free place's distance from the page's top, in inches on Letter; from the foot when negative."},
		{Key: "Align", Kind: "choice", Choices: []string{"", "L", "C", "R"}, Doc: "Left, centred or right; empty: the place's. " +
			"In a free place, how the text stands to X."},
		{Key: "Font", Kind: "text", Doc: "The font, as an element's (empty: the element set's title format in the centre and at the top, its meta format elsewhere)."},
		{Key: "Size", Kind: "number", Doc: "The font size in points (0: the element set's)."},
		{Key: "Style", Kind: "text", Doc: "b bold, i italic, u underlined, joined (\"bi\")."},
		{Key: "Caps", Kind: "bool", Doc: "Printed in capitals."},
		{Key: "Before", Kind: "number", Doc: "Space before it, in lines of 12 pt."},
		{Key: "Picture", Kind: "bool", Doc: "Its value is a picture's file (art.png, or ![Key art](art.png)) beside the script or in its bundle. " +
			"A value written as a Markdown picture is printed as one under any key."},
		{Key: "Height", Kind: "number", Doc: "A picture's height in inches (0: 2); made smaller to fit the page's width."},
		{Key: "Hide", Kind: "bool", Doc: "Left off the page."},
	}
}

// FormatFileFields are what a format file (a preset's file, name.toml
// in a folder of format files) holds.
func FormatFileFields() []Field {
	return []Field{
		{Key: "Name", Kind: "text", Doc: "The format's name, as lists show it (the file's name if none)."},
		{Key: "Description", Kind: "text", Doc: "What kind of script it is for."},
		{Key: "Page", Kind: "choice", Choices: []string{"", "letter", "a4", "a5"}, Doc: "The paper it suggests (\"\": the user's or the default)."},
		{Key: "Base", Kind: "text", Doc: "The format it starts from (the screenplay if none); its name, description and paper too, where this file has none."},
		{Key: "Elements", Kind: "tables", Doc: "The elements that differ from Base, each whole: an element here takes the place of Base's."},
	}
}

// Describe is the field with key among fields, and whether there is one.
func Describe(fields []Field, key string) (Field, bool) {
	for _, f := range fields {
		if f.Key == key {
			return f, true
		}
	}
	return Field{}, false
}
