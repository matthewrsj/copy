//+build !test

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"

	"google.golang.org/grpc/benchmark/flags"
	pb "stash.teslamotors.com/rr/towerproto"
	"stash.teslamotors.com/rr/traycontrollers"
)

func main() {
	avail := flags.IntSlice("with-avail", []int{2}, "availability for each tower")

	flag.Parse()

	log.Printf("%d available", *avail)

	for i, a := range *avail {
		go func(i, a int) {
			addr := fmt.Sprintf("localhost:%d", 13180+i)

			sm := http.NewServeMux()
			sm.HandleFunc("/avail", func(w http.ResponseWriter, r *http.Request) {
				avail := make(traycontrollers.Availability, a)
				for j := 0; j < a; j++ {
					avail[fmt.Sprintf("CM2-63010-%02d-%02d", i*2+1+(j%2), (j-(j/2)-(j%2))+1)] = traycontrollers.FXRAvailable{
						Status:   pb.FixtureStatus_FIXTURE_STATUS_IDLE.String(),
						Reserved: false,
					}
				}

				jb, err := json.Marshal(avail)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				if _, err := w.Write(jb); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				log.Printf("responded with %d trays available from %s", a, addr)
			})

			sm.HandleFunc("/preparedForDelivery", func(w http.ResponseWriter, r *http.Request) {
				log.Printf("received fixture reservation at %s", addr)
				w.WriteHeader(http.StatusOK)
			})

			log.Printf("serving on %s", addr)

			log.Fatal(http.ListenAndServe(addr, sm))
		}(i, a)
	}

	select {}
}
