package rules

import (
	"github.com/BurntSushi/toml"
	"os"
	"log"
)

type TOMLConf struct{
	Elements map[string]Set
	Scenes map[string][]string
	metadata toml.MetaData
}

func ReadFile(file string) (TOMLConf, error) {
	var r TOMLConf
	m, err := toml.DecodeFile(file, &r)
	r.metadata = m
	return r, err
}

func GetConf(file string) TOMLConf {
	c, err := ReadFile(file)
	if err != nil {
		log.Println("Error loading file, loading default configuration: ", err)
		c = DefaultConf()
	}
	log.Println("Configuration set")
	return c
}

func DefaultConf() TOMLConf {
	return TOMLConf{
		Elements: map[string]Set{
			"default": Default,
		},
		Scenes: map[string][]string{
			"en": []string{"INT", "EXT", "EST", "INT./EXT", "INT/EXT", "EXT/INT", "EXT./INT", "I/E"},
			"it": []string{"INT", "EST", , "INT./EST", "INT/EST", "EST/INT", "EST./INT", "I/E"},
			"nl": []string{"BIN", "BUI", "BI", "BU", "OPEN", "BIN./BUI", "BUI./BIN", "BIN/BUI", "BI/BU"},
			"de": []string{"INT", "EXT", "ETABL", "INT./EXT", "INT/EXT", "EXT/INT", "EXT./INT", "I/E"},
			"fr": []string{"INT", "EXT", "INT./EXT", "INT/EXT", "EXT/INT", "EXT./INT", "I/E"},
			"eo": []string{"EN.", "ENE", "EKST", "EK", "EN/EKST", "EKST/EN", "EKST./EN", "EN./EKST"},
			"ru": []string{"ИНТ", "НАТ", "ИНТ/НАТ", "ИНТ./НАТ", "НАТ/ИНТ", "НАТ./ИНТ", "ЭКСТ", "И/Н", "Н/И"},
		},
	}
}

func Dump(file string) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	return toml.NewEncoder(f).Encode(DefaultConf())
}
