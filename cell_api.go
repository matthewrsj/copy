package towercontroller

type cellAPIConf struct {
	Base      string       `yaml:"base"`
	Endpoints endpointConf `yaml:"endpoints"`
}

// endpointConf contains the various endpoints used to communicate with the cell API
// Members with the `Fmt` suffix have format directives embedded for various options.
type endpointConf struct {
	// fmt: tray_serial
	CellMapFmt string `yaml:"cell_map"`
	// fmt: tray_serial|process|status
	ProcessStatusFmt string `yaml:"process_status"`
	// fmt: tray_serial
	NextProcStepFmt string `yaml:"next_process_step"`
	// fmt: tray_serial
	CloseProcessFmt string `yaml:"close_process"`

	// no format here
	CellStatus string `yaml:"cell_status"`
}
