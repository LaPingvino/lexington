package rules

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/BurntSushi/toml"
)

type TOMLConf struct {
	Elements map[string]Set      `toml:"Elements"`
	Scenes   map[string][]string `toml:"Scenes"`
	// Matchups are, by language, its scene heading words in the order of
	// Roles (inside, outside, ...), "" for one it has none for: what
	// translates to what (matchups.go).
	Matchups map[string][]string `toml:"Matchups"`
	metadata toml.MetaData
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
	return r, nil
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
		Scenes: map[string][]string{
			"en": {"INT", "EXT", "EST", "INT./EXT", "INT/EXT", "EXT/INT", "EXT./INT", "I/E"},
			"it": {"INT", "EST", "INT./EST", "INT/EST", "EST/INT", "EST./INT", "I/E"},
			"nl": {"BIN", "BUI", "BI", "BU", "OPEN", "BIN./BUI", "BUI./BIN", "BIN/BUI", "BI/BU"},
			"de": {"INT", "EXT", "ETABL", "INT./EXT", "INT/EXT", "EXT/INT", "EXT./INT", "I/E"},
			"fr": {"INT", "EXT", "INT./EXT", "INT/EXT", "EXT/INT", "EXT./INT", "I/E"},
			"eo": {"EN.", "ENE", "EKST", "EK", "EN/EKST", "EKST/EN", "EKST./EN", "EN./EKST"},
			"ru": {"ИНТ", "НАТ", "ИНТ/НАТ", "ИНТ./НАТ", "НАТ/ИНТ", "НАТ./ИНТ", "ЭКСТ", "И/Н", "Н/И"},
			// after Hekaya, Fountain in Arabic (michaelkmalak.github.io/hekaya)
			"ar": {"داخلي", "خارجي", "لقطة تأسيسية", "داخلي/خارجي", "خارجي/داخلي", "د/خ"},
			"he": {"פנים", "חוץ", "פנים/חוץ", "חוץ/פנים"},
			"zh": {"内景", "外景", "内景/外景", "外景/内景"},
			// Japanese headings (hashira) start with a circle: ○喫茶店・店内（夜）
			"ja":  {"○", "◯"},
			"tok": {"INSA", "KON", "INSA/KON", "I./K", "I/K", "KON/INSA", "K./I", "K/I"},
		},
		Matchups: map[string][]string{
			//     inside, outside, both, both the other way, establishing
			"en":  {"INT.", "EXT.", "INT./EXT.", "EXT./INT.", "EST."},
			"it":  {"INT.", "EST.", "INT./EST.", "EST./INT.", ""}, // EST. is esterno
			"nl":  {"BIN.", "BUI.", "BIN./BUI.", "BUI./BIN.", ""},
			"de":  {"INT.", "EXT.", "INT./EXT.", "EXT./INT.", "ETABL."},
			"fr":  {"INT.", "EXT.", "INT./EXT.", "EXT./INT.", ""},
			"eo":  {"EN.", "EKST.", "EN./EKST.", "EKST./EN.", ""},
			"ru":  {"ИНТ.", "НАТ.", "ИНТ./НАТ.", "НАТ./ИНТ.", ""},
			"ar":  {"داخلي", "خارجي", "داخلي/خارجي", "خارجي/داخلي", "لقطة تأسيسية"},
			"he":  {"פנים", "חוץ", "פנים/חוץ", "חוץ/פנים", ""},
			"zh":  {"内景", "外景", "内景/外景", "外景/内景", ""},
			"tok": {"INSA.", "KON.", "INSA/KON.", "KON/INSA.", ""},
		},
	}
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
