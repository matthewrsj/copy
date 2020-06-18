package towercontroller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/ctet/go-socketcan/pkg/socketcan"
	pb "stash.teslamotors.com/rr/towerproto"
)

const _availabilityEndpoint = "/avail"

type availability struct {
	Location string           `json:"location"`
	Status   pb.FixtureStatus `json:"status"`
}

// HandleAvailable is the handler for the endpoint reporting availability of fixtures
func HandleAvailable(conf Configuration, logger *zap.SugaredLogger) {
	http.HandleFunc(_availabilityEndpoint, func(w http.ResponseWriter, r *http.Request) {
		logger.Infow("got request to /avail", "remote", r.RemoteAddr)

		avail := make(chan availability)
		done := make(chan struct{})
		var wg sync.WaitGroup

		var as []availability
		go func() {
			for a := range avail {
				as = append(as, a)
			}
			close(done)
		}()

		wg.Add(len(conf.Fixtures))

		for n, id := range conf.Fixtures {
			go func(n string, id uint32) {
				// TODO don't write an error immediately
				defer func() {
					wg.Done()
				}()

				dev, err := socketcan.NewIsotpInterface(conf.CAN.Device, id, conf.CAN.TXID)
				if err != nil {
					logger.Errorw("create new ISOTP interface", "FXR", n, "error", err)
					avail <- availability{
						Location: fmt.Sprintf("%s-%s%s-%s", conf.Loc.Line, conf.Loc.Process, conf.Loc.Aisle, n),
						Status:   pb.FixtureStatus_FIXTURE_STATUS_UNKNOWN_UNSPECIFIED,
					}
					return
				}

				logger.Debugw("created ISOTP interface", "FXR", n)

				if err = dev.SetCANFD(); err != nil {
					logger.Errorw("set CANFD on ISTOP interface", "FXR", n, "error", err)
					avail <- availability{
						Location: fmt.Sprintf("%s-%s%s-%s", conf.Loc.Line, conf.Loc.Process, conf.Loc.Aisle, n),
						Status:   pb.FixtureStatus_FIXTURE_STATUS_UNKNOWN_UNSPECIFIED,
					}
					return
				}

				logger.Debugw("set CANFD flags", "FXR", n)

				if err = dev.SetRecvTimeout(time.Second * 3); err != nil {
					logger.Errorw("set recv timeout on ISOTP interface", "FXR", n, "error", err)
					avail <- availability{
						Location: fmt.Sprintf("%s-%s%s-%s", conf.Loc.Line, conf.Loc.Process, conf.Loc.Aisle, n),
						Status:   pb.FixtureStatus_FIXTURE_STATUS_UNKNOWN_UNSPECIFIED,
					}
					return
				}

				logger.Debugw("set recv timeout", "FXR", n)

				buf, err := dev.RecvBuf()
				if err != nil {
					// only a warn because a timeout could have occurred which isn't as drastic
					logger.Warnw("receive buffer", "FXR", n, "error", err)
					avail <- availability{
						Location: fmt.Sprintf("%s-%s%s-%s", conf.Loc.Line, conf.Loc.Process, conf.Loc.Aisle, n),
						Status:   pb.FixtureStatus_FIXTURE_STATUS_UNKNOWN_UNSPECIFIED,
					}
					return
				}

				logger.Debugw("received message from FXR", "FXR", n)

				var msg pb.FixtureToTower
				if err = proto.Unmarshal(buf, &msg); err != nil {
					avail <- availability{
						Location: fmt.Sprintf("%s-%s%s-%s", conf.Loc.Line, conf.Loc.Process, conf.Loc.Aisle, n),
						Status:   pb.FixtureStatus_FIXTURE_STATUS_UNKNOWN_UNSPECIFIED,
					}
					return
				}

				logger.Debugw("fixture status", "FXR", n, "status", msg.GetOp().GetStatus().String())

				avail <- availability{
					Location: msg.GetFixturebarcode(),
					Status:   msg.GetOp().GetStatus(),
				}
			}(n, id)
		}

		wg.Wait()
		close(avail)

		<-done

		body, err := json.Marshal(as)
		if err != nil {
			logger.Errorw("marshal json body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		if _, err = w.Write(body); err != nil {
			logger.Errorw("write body to responsewriter", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}
