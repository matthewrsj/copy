package protostream

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

// Configuration contains all the initial configuration for a Stream
type Configuration struct {
	ID          IDConfig     `yaml:"ids"`
	Col1        ColumnConfig `yaml:"column1"`
	Col2        ColumnConfig `yaml:"column2"`
	FixtureList []string     `yaml:"fixture_locations"`

	// constructed fields
	Fixtures  map[string]CANConfig
	TCAUXCol1 CANConfig // listen on both columns, though it will only show up on one
	TCAUXCol2 CANConfig
}

// IDConfig contains information on CAN IDs for ISOTP
type IDConfig struct {
	BaseRX     uint32 `yaml:"base_rx"`
	BaseTX     uint32 `yaml:"base_tx"`
	BaseRXDiag uint32 `yaml:"base_rx_diag"`
	BaseTXDiag uint32 `yaml:"base_tx_diag"`
	TAUXRx     uint32 `yaml:"taux_rx"`
	TAUXTx     uint32 `yaml:"taux_tx"`
}

// ColumnConfig contains the base information for a tower column
// each level above level 1 increments the RX and TX values
type ColumnConfig struct {
	CANBus string `yaml:"bus"`
}

// CANConfig contains the RX, TX and bus information for a single FXR
type CANConfig struct {
	RX          uint32
	TX          uint32
	RXDiag      uint32
	TXDiag      uint32
	Bus         string
	NodeID      string
	RecvTimeout time.Duration
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

	c.TCAUXCol1 = CANConfig{
		RX:          c.ID.TAUXRx,
		TX:          c.ID.TAUXTx,
		Bus:         c.Col1.CANBus,
		RecvTimeout: DefaultRecvTimeout,
		NodeID:      "TCAUX",
	}

	c.TCAUXCol2 = CANConfig{
		RX:          c.ID.TAUXRx,
		TX:          c.ID.TAUXTx,
		Bus:         c.Col2.CANBus,
		RecvTimeout: DefaultRecvTimeout,
		NodeID:      "TCAUX",
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

		rx, tx := c.ID.BaseRX+uint32(lvl), c.ID.BaseTX+uint32(lvl)
		rxDiag, txDiag := c.ID.BaseRXDiag+uint32(lvl), c.ID.BaseTXDiag+uint32(lvl)

		var bus string

		switch col {
		case 1:
			bus = c.Col1.CANBus
		case 2:
			bus = c.Col2.CANBus
		default:
			// this should never happen
			return c, fmt.Errorf("invalid column '%d', must be 1 or 2", col)
		}

		c.Fixtures[fixture] = CANConfig{
			RX:     rx,
			TX:     tx,
			RXDiag: rxDiag,
			TXDiag: txDiag,
			Bus:    bus,
		}
	}

	return c, nil
}
