//+build !test

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/ctet/go-socketcan/pkg/socketcan"
	tower "stash.teslamotors.com/rr/towerproto"
)

type canIDs struct {
	ids *[]uint32
}

func (c canIDs) String() string {
	if c.ids == nil || len(*c.ids) == 0 {
		return ""
	}

	s := fmt.Sprint((*c.ids)[0])

	for _, id := range *c.ids {
		s += "," + fmt.Sprint(id)
	}

	return s
}

func (c canIDs) Set(s string) error {
	fields := strings.Split(s, ",")
	if len(fields) == 0 {
		return fmt.Errorf("invalid can ID list: %v", c)
	}

	for _, id := range fields {
		id64, err := strconv.ParseUint(id, 16, 32)
		if err != nil {
			return err
		}

		(*c.ids) = append(*c.ids, uint32(id64))
	}

	return nil
}

// nolint:gocognit // just a script
func main() {
	var cs canIDs
	cs.ids = &[]uint32{}

	fs := flag.NewFlagSet("sniffer", flag.ExitOnError)
	fs.Var(&cs, "ids", "CAN IDs in hex to sniff")
	tx64 := fs.Uint64("txid", 0x241, "CAN TXID in hex")
	ifName := fs.String("ifname", "can0", "CAN interface name")

	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}

	type devID struct {
		dev socketcan.Interface
		id  uint32
	}

	devIDs := make([]devID, len(*cs.ids))

	for i, id := range *cs.ids {
		dev, err := socketcan.NewIsotpInterface(*ifName, id, uint32(*tx64))
		if err != nil {
			log.Printf("create ISOTP interface: %v", err)
			return
		}

		defer func() {
			_ = dev.Close()
		}()

		if err := dev.SetCANFD(); err != nil {
			log.Printf("set CANFD: %v", err)
			return
		}

		if err := dev.SetRecvTimeout(time.Millisecond * 500); err != nil {
			log.Printf("SetRecvTimeout: %v", err)
			return
		}

		devIDs[i] = devID{
			dev: dev,
			id:  id,
		}
	}

	c := make(chan os.Signal, 1)
	done := make(chan struct{})

	signal.Notify(c, os.Interrupt, syscall.SIGHUP)

	go func() {
		<-c
		log.Println("CTRL-C received, shutting down... (CTRL-C again to force)")
		close(done)
		<-c
		os.Exit(1)
	}()

	var wg sync.WaitGroup

	wg.Add(len(devIDs))

	// main sniff loop
	for _, devInfo := range devIDs {
		go func(dev socketcan.Interface, id uint32) {
			defer wg.Done()

			for {
				select {
				case <-done:
					return
				default:
					buf, err := dev.RecvBuf()
					if err != nil {
						continue
					}

					var foundMsg bool

					for _, msg := range []proto.Message{&tower.FixtureToTower{}, &tower.TowerToFixture{}} {
						if err := proto.Unmarshal(buf, msg); err != nil {
							continue
						}

						jb, err := json.MarshalIndent(msg, "", "\t")
						if err != nil {
							log.Printf("%x marshal proto into JSON: %v", id, err)
							continue
						}

						foundMsg = true

						log.Printf("ID: %x; Message: %v", id, string(jb))
					}

					if !foundMsg {
						log.Printf("%x sent unrecognized message", id)
					}
				}
			}
		}(devInfo.dev, devInfo.id)
	}

	wg.Wait()
}
