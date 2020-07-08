package towercontroller

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// Configuration contains the configuration parameters for the statemachine.
type Configuration struct {
	RecipeFile      string              `yaml:"recipefile"`
	IngredientsFile string              `yaml:"ingredientsfile"`
	Remote          string              `yaml:"cdcontroller_remote"`
	CellAPI         cellAPIConf         `yaml:"cell_api"`
	Loc             location            `yaml:"location"`
	CAN             canConf             `yaml:"can"`
	Fixtures        map[string]uint32   `yaml:"fixture_ids"`
	CellMap         map[string][]string `yaml:"cell_map"`
}

type canConf struct {
	Col1Device string `yaml:"col1_dev"`
	Col2Device string `yaml:"col2_dev"`
	TXID       uint32 `yaml:"txid"`
}

const (
	// nolint:deadcode,varcheck // _colOneID is for documentation, but is not currently used
	_colOneID = "01"
	_colTwoID = "02"
)

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
