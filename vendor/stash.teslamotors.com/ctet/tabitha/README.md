# tabitha

**T**ester **A**utomation **B**oss (**I**s **TH**e **A**nswer)

Tabitha aims to provide a control center for deployed testers. Initial feature set will allow an administrator
to see the current status and version of a tester and exercise supervisory controls such as

- Update firmware used by the tester
- Modify configuration in use by the tester
- Update the tester itself to the latest released version
- View tester githash, version, and up/down status

### Why?

#### Before

- FW:  Hey **Engineer Bob**, your tester needs to be using the latest firmware at (URL).
- Bob: Dammit.
- Also Bob:
    - SSH-es in to tester.
    - Manually downloads firmware.
    - Manually moves it into place on the file system.
    - Manually updates configuration file to point to latest firmware.
    - Cries.

#### After

- FW: Hey **Sustaining Max**, your tester needs to be using the latest firmware at (URL).
- Max: Happy to!
- Also Max:
    - Enters (URL) into tabitha webapp. Presses button.
    - Laughs at the pure ease of it all.

## Project Structure

The tabitha system is client-server, and therefore exists in two packages.

### Client

The base package `tabitha` provides the client interface used by testers. The goal of this package is to provide a
single-statement initiation for the tester software.

```go
package main

import (
  "log"
  "os"
  "os/signal"
  "syscall"

  "stash.teslamotors.com/ctet/tabitha"
)

// Config local configuration for the tester
type Config struct {
  One   string `yaml:"one" json:"one"`
  Two   string `yaml:"two" json:"two"`
  Three int    `yaml:"three" json:"three"`
}

// Global variables for use by govvv
var (
  GitSummary string
  Version    string
)

func main() {
  log.Println("I'm a simple tester, starting up :)")

  /*

     snip tester setup

  */

  configPath := "./conf.json"
  conf := Config{
    One:   "one",
    Two:   "twotwo",
    Three: 333,
  }

  go tabitha.ServeNewHandler(
    tabitha.WithTesterName("wonderful_tester_never_fails"),
    tabitha.WithTesterVersion(Version),
    tabitha.WithTesterGitSummary(GitSummary),
    tabitha.WithLocalConfiguration(configPath, tabitha.ConfigTypeYAML, &conf),
    tabitha.WithLocalFirmwareDirectory("./fw"),
    tabitha.WithFirmwareDownloadWriteFileName("latest_firmware_in_use.hex"),
  )

  /*

    snip tester runtime

   */

  sig := make(chan os.Signal, 1)
  signal.Notify(sig, syscall.SIGTERM)
  <-sig
}
```

### Server

`tabitha/server` contains the web application code for the administrator to manage the testers deployed. It can be run
by simply building and executing.

```shell
cd server/cmd
go build .
./cmd
```

## Web User Interface

- First have server and a development tester running
- Install node.js: `brew install nodejs`
- Switch to node project `cd ui`
- Switch to tesla npm registry `npm config set registry https://artifactory.teslamotors.com/artifactory/api/npm/digital-products-node/`
- Install node dependencies: `npm install`
- To run: `npm start`

Navigate to http://localhost:3000 in browser

## Tabitha Client API Endpoints

The handler on the client spins up a local webserver for the client so the tabitha server can communicate with it.

### /firmware

- METHOD: POST
- BODY
```json
{
  "remote": "http://firmwaredownload.url/fw.hex"
}
```

- RESPONSE
    - 501 NOT IMPLEMENTED (if tester hasn't configured this feature)
    - 500 INTERNAL SERVER ERROR
    - 200 OK

### /configuration

- METHOD: GET
- RESPONSE
  - 501 NOT IMPLEMENTED (if tester hasn't configured this feature)
  - 500 INTERNAL SERVER ERROR
  - 200 OK
```json
{
  "body": "{\"one\":1,\"two\":22}",
  "type": 1
}
```

### /newconfiguration

- METHOD: POST
- BODY: bytes of new configuration file
- RESPONSE
  - 501 NOT IMPLEMENTED (if tester hasn't configured this feature)
  - 500 INTERNAL SERVER ERROR
  - 200 OK

### /update

- METHOD: POST
- BODY: none
- RESPONSE
  - 501 NOT IMPLEMENTED (if tester hasn't configured this feature)
  - 500 INTERNAL SERVER ERROR
  - 200 OK

### /health

- METHOD: GET
- RESPONSE
  - 501 NOT IMPLEMENTED (if tester hasn't configured this feature)
  - 500 INTERNAL SERVER ERROR
  - 200 OK
```json
{
  "name": "tester-name",
  "status": "alive",
  "version": "v0.0.1",
  "gitsummary": "abcdef123-dirty"
}
```

## Example

An example exists in the `examples/simple` directory that can be trivially built and ran.

First run the server locally

```shell
cd server/cmd
go build .
./cmd
```

In another terminal...

```shell
cd examples/simple
# optionally install govvv to get version and git information to tabitha
go get github.com/ahmetb/govvv
govvv build . # if not using govvv just use the regular go build tool
./simple
```

## TODO

- Server needs to host a react app instead of just vanilla HTML.
- Server needs to allow the user to modify the configuration of a tester and POST it to the client.
- Server needs to keep a record of testers in a database and support CRUD commands (current is in-mem).
- Client needs to auto-register with server on startup if not previously registered.
- Server needs to GET current firmware version from tester.
    - Tester firmware management needs to change in this case, as a universal firmware file name is not descriptive.
- Client needs to support multiple update strategies such as fetch binary, fetch new docker image, etc.
