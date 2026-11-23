package rules

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/BurntSushi/toml"
)

type TOMLConf struct {
	Elements map[string]Set `toml:"Elements,omitempty"`
	// Languages are, by language ("en", "eo", "zh"), how its scripts are
	// written: scene words, cue and transition rules, title keys,
	// direction (language.go). A file named script.eo.fountain is read
	// with Esperanto's.
	Languages map[string]Language `toml:"Languages,omitempty"`
	// Standard is the language whose Scenes are standard Fountain's: what
	// other Fountain apps read. A script in another language is a dialect
	// of Fountain, written for them with fountain.Force.
	Standard string `toml:"Standard,omitempty"`
	// Extensions are file extensions (without the dot) whose files are in
	// a language: [Extensions] hekaya = "hekaya" reads film.hekaya with
	// Hekaya's rules, as film.ar.fountain is read with Arabic's. The
	// default's (DefaultExtensions) are known in every configuration; one
	// of one's own adds to them or changes one.
	Extensions map[string]string `toml:"Extensions,omitempty"`
	// TitlePages are title page templates of one's own, by name
	// (titlepage.go): one named as a preset (standard, bold, poster)
	// takes its place.
	TitlePages map[string]TitlePage `toml:"TitlePages,omitempty"`
	// The tables of configuration files from before Languages: by
	// language its scene words, its matchup and its direction. Reading a
	// file moves them into Languages (normalized); they are never
	// written.
	LegacyScenes     map[string][]string `toml:"Scenes,omitempty"`
	LegacyMatchups   map[string][]string `toml:"Matchups,omitempty"`
	LegacyDirections map[string]string   `toml:"Directions,omitempty"`
	metadata         toml.MetaData
}

func ReadFile(file string) (TOMLConf, error) {
	var r TOMLConf

	// Check if file exists first
	if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
		return r, fmt.Errorf("configuration file %s not found: %w", file, err)
	}

	m, err := toml.DecodeFile(file, &r)
	if err != nil {
		return r, fmt.Errorf("failed to parse TOML file %s: %w", file, err)
	}

	r.metadata = m
	return r.normalized(), nil
}

func GetConf(file string) TOMLConf {
	c, err := ReadFile(file)
	if err != nil {
		log.Printf("Error loading configuration file: %v, using defaults", err)
		return DefaultConf().withPresets()
	}
	log.Println("Configuration loaded successfully")
	return c.withPresets()
}

func DefaultConf() TOMLConf {
	return TOMLConf{
		Elements: map[string]Set{
			"default": Default,
		},
		Languages:  defaultLanguages(),
		Standard:   "en",
		Extensions: DefaultExtensions(),
	}
}

// StandardLanguage is the configured Standard, the default's if none.
func (c TOMLConf) StandardLanguage() string {
	if c.Standard != "" {
		return c.Standard
	}
	return DefaultConf().Standard
}

// RTL tells whether lang's text runs right to left (its Direction is
// "rtl"): what a line of a script in lang with no direction of its own
// takes. A language c does not name has the default's direction, so a
// configuration written before directions existed reads Arabic and
// Hebrew right to left too ("ltr" turns one around).
func (c TOMLConf) RTL(lang string) bool { return c.Language(lang).RTL() }

// Direction is the way lang's text runs as c reads it: "rtl" or "ltr".
func (c TOMLConf) Direction(lang string) string {
	if c.RTL(lang) {
		return "rtl"
	}
	return "ltr"
}

func Dump(file string) error {
	f, err := os.Create(file)
	if err != nil {
		return fmt.Errorf("failed to create configuration file %s: %w", file, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			log.Printf("Error closing configuration file: %v", closeErr)
		}
	}()

	if err := toml.NewEncoder(f).Encode(DefaultConf()); err != nil {
		return fmt.Errorf("failed to encode configuration to %s: %w", file, err)
	}

	return nil
}
