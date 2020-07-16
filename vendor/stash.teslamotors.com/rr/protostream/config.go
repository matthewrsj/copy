package protostream

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

// Configuration contains all the initial configuration for a Stream
type Configuration struct {
	Col1        ColumnConfig `yaml:"column1"`
	Col2        ColumnConfig `yaml:"column2"`
	FixtureList []string     `yaml:"fixture_locations"`

	Fixtures map[string]CANConfig // constructed field
}

// ColumnConfig contains the base information for a tower column
// each level above level 1 increments the RX and TX values
type ColumnConfig struct {
	BaseRX uint32 `yaml:"base_rx"`
	BaseTX uint32 `yaml:"base_tx"`
	CANBus string `yaml:"bus"`
}

// CANConfig contains the RX, TX and bus information for a single FXR
type CANConfig struct {
	RX, TX uint32
	Bus    string
}

// LoadConfig loads a protostream configuration file
func LoadConfig(path string) (Configuration, error) {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return Configuration{}, fmt.Errorf("read file: %v", err)
	}

	var c Configuration
	if err = yaml.Unmarshal(buf, &c); err != nil {
		return c, fmt.Errorf("unmarshal YAML file: %v", err)
	}

	c.Fixtures = make(map[string]CANConfig)

	for _, fixture := range c.FixtureList {
		fields := strings.Split(fixture, "-")
		if len(fields) != 2 {
			return c, fmt.Errorf("invalid fixture format '%s', expect 'CC-LL` (column-level)", fixture)
		}

		colS, lvlS := fields[0], fields[1]

		col, err := strconv.ParseUint(colS, 10, 32)
		if err != nil {
			return c, fmt.Errorf("invalid column format '%s', expect numeric field", colS)
		}

		// coerce the column to be 1 or 2 for configuration purposes
		if col%2 == 0 {
			col = 2
		} else {
			col = 1
		}

		lvl, err := strconv.ParseUint(lvlS, 10, 32)
		if err != nil {
			return c, fmt.Errorf("invalid level format '%s', expect numeric field", lvlS)
		}

		if lvl < 1 || lvl > 12 {
			return c, fmt.Errorf("invalid level '%d', must be 1-12, inclusive", lvl)
		}

		var (
			rx, tx uint32
			bus    string
		)

		switch col {
		case 1:
			rx = c.Col1.BaseRX + uint32(lvl)
			tx = c.Col1.BaseTX + uint32(lvl)
			bus = c.Col1.CANBus
		case 2:
			rx = c.Col2.BaseRX + uint32(lvl)
			tx = c.Col2.BaseTX + uint32(lvl)
			bus = c.Col2.CANBus
		default:
			// this should never happen
			return c, fmt.Errorf("invalid column '%d', must be 1 or 2", col)
		}

		c.Fixtures[fixture] = CANConfig{
			RX:  rx,
			TX:  tx,
			Bus: bus,
		}
	}

	return c, nil
}
