// Package cdcontroller implements the cdcontroller for RR Formation
package cdcontroller

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// Configuration holds the common options for running this controller
type Configuration struct {
	Loc                    Location `yaml:"location"`
	TestOnlyAisles         []string `yaml:"test_only_aisles"`
	ProductionOnlyAisles   []string `yaml:"production_only_aisles"`
	CellAPIBase            string   `yaml:"cell_api_base"`
	CellAPINextProcStepFmt string   `yaml:"cell_api_next_process_step"`
	CellAPICloseProcessFmt string   `yaml:"cell_api_close_process"`
}

// Location describes the physical location of the controller
// and the aisles it manages.
type Location struct {
	Line    string                 `yaml:"line"`
	Station string                 `yaml:"station"`
	Aisles  map[string]AisleConfig `yaml:"aisles"`
}

// AisleConfig contains a map of towers to their corresponding tower config
type AisleConfig map[int]TowerConfig

// TowerConfig describes the fixture configuration of a tower
type TowerConfig struct {
	Fixtures map[int][]int `yaml:"fixtures"`
	Address  string        `yaml:"address"`
}

// LoadConfig loads a configuration file
func LoadConfig(fName string) (Configuration, error) {
	b, err := ioutil.ReadFile(fName)
	if err != nil {
		return Configuration{}, fmt.Errorf("read file: %v", err)
	}

	var c Configuration
	if err := yaml.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("unmarshal: %v", err)
	}

	return c, nil
}
