package towercontroller

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// Configuration contains the configuration parameters for the statemachine.
type Configuration struct {
	RecipeFile      string                 `yaml:"recipefile"`
	IngredientsFile string                 `yaml:"ingredientsfile"`
	Remote          string                 `yaml:"cdcontroller_remote"`
	CellAPI         cellAPIConf            `yaml:"cell_api"`
	Loc             location               `yaml:"location"`
	Fixtures        map[string]fixtureConf `yaml:"fixture_ids"`
	CellMap         map[string][]string    `yaml:"cell_map"`
}

type fixtureConf struct {
	Bus string `yaml:"bus"`
	RX  uint32 `yaml:"rx"`
	TX  uint32 `yaml:"tx"`
}

type location struct {
	Line    string `yaml:"line"`
	Process string `yaml:"process"`
	Aisle   string `yaml:"aisle"`
}

// LoadConfig loads the configuration file at path.
func LoadConfig(path string) (Configuration, error) {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return Configuration{}, fmt.Errorf("read configuration file %s: %v", path, err)
	}

	var conf Configuration

	if err = yaml.Unmarshal(contents, &conf); err != nil {
		return Configuration{}, fmt.Errorf("unmarshal configuration contents: %v", err)
	}

	return conf, nil
}
