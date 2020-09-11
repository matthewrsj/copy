package towercontroller

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// Configuration contains the configuration parameters for the statemachine.
type Configuration struct {
	Remote          string              `yaml:"cdcontroller_remote"`
	RouterAddress   string              `yaml:"router_address"`
	CellAPI         cellAPIConf         `yaml:"cell_api"`
	Loc             location            `yaml:"location"`
	AllFixtures     []string            `yaml:"all_fixtures"`
	AllowedFixtures []string            `yaml:"allowed_fixtures"`
	CellMap         map[string][]string `yaml:"cell_map"`
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
