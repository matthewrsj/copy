# sniffer

The sniffer sniffs proto over ISOTP on a CANFD bus.

## Build

```bash
go build .
```

## Usage

```bash
$ ./sniffer -h
Usage of sniffer:
  -ids value
        CAN IDs in hex to sniff
  -ifname string
        CAN interface name (default "can0")
  -txid uint
        CAN TXID in hex (default 577)

```

## Example output

```bash
$ ./sniffer -ids 1c1 -ifname vcan0
2020/07/01 12:23:15 ID: 1c1; Message: {
	"Content": {
		"Op": {
			"status": 1
		}
	},
	"traybarcode": "11223344B",
	"fixturebarcode": "CM2-63010-02-04",
	"process_step": "FORM_CYCLE"
}
2020/07/01 12:23:15 ID: 1c1; Message: {
	"recipe": {}
}
2020/07/01 12:23:16 ID: 1c1; Message: {
	"Content": {
		"Op": {
			"status": 1
		}
	},
	"traybarcode": "11223344B",
	"fixturebarcode": "CM2-63010-02-04",
	"process_step": "FORM_CYCLE"
}
2020/07/01 12:23:16 ID: 1c1; Message: {
	"recipe": {}
}
2020/07/01 12:23:17 ID: 1c1; Message: {
	"Content": {
		"Op": {
			"status": 1
		}
	},
	"traybarcode": "11223344B",
	"fixturebarcode": "CM2-63010-02-04",
	"process_step": "FORM_CYCLE"
}
2020/07/01 12:23:17 ID: 1c1; Message: {
	"recipe": {}
}
2020/07/01 12:23:21 CTRL-C received, shutting down... (CTRL-C again to force)
```