package towercontroller

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// Configuration contains the configuration parameters for the statemachine.
type Configuration struct {
	RecipeFile      string      `yaml:"recipefile"`
	IngredientsFile string      `yaml:"ingredientsfile"`
	CellAPI         cellAPIConf `yaml:"cell_api"`
	Fixtures        []string    `yaml:"fixtures"`
	CAN             canConf     `yaml:"can"`
}

type canConf struct {
	Device string `yaml:"dev"`
	TXID   uint32 `yaml:"txid"`
	RXID   uint32 `yaml:"rxid"`
}

// LoadConfig loads the configuration file at path.
func LoadConfig(path string) (Configuration, error) {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return Configuration{}, fmt.Errorf("read configuration file %s: %v", path, err)
	}

	var conf Configuration

	return conf, yaml.Unmarshal(contents, &conf)
}
