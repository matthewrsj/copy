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
	"stash.teslamotors.com/rr/traycontrollers"
)

const _availabilityEndpoint = "/avail"

// HandleAvailable is the handler for the endpoint reporting availability of fixtures
// nolint:gocognit,funlen // ignore
func HandleAvailable(configPath string, logger *zap.SugaredLogger, registry map[string]*FixtureInfo) {
	http.HandleFunc(_availabilityEndpoint, func(w http.ResponseWriter, r *http.Request) {
		logger.Infow("got request to /avail", "remote", r.RemoteAddr)

		var (
			conf Configuration
			err  error
		)

		if conf, err = LoadConfig(configPath); err != nil {
			logger.Errorw("read configuration file", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		avail := make(chan traycontrollers.FXRAvailable)
		done := make(chan struct{})
		var wg sync.WaitGroup

		var as traycontrollers.Availability
		go func() {
			for a := range avail {
				as = append(as, a)
			}
			close(done)
		}()

		wg.Add(len(conf.Fixtures))

		for n, fConf := range conf.Fixtures {
			go func(n string, fConf fixtureConf) {
				defer func() {
					wg.Done()
				}()

				// nolint:govet // allow shadow of err declaration for go routine scope
				var (
					dev socketcan.Interface
					err error
					msg pb.FixtureToTower
				)

				if dev, err = socketcan.NewIsotpInterface(fConf.Bus, fConf.RX, fConf.TX); err != nil {
					logger.Errorw("create new ISOTP interface", "FXR", n, "error", err)
					avail <- traycontrollers.FXRAvailable{
						Location: fmt.Sprintf("%s-%s%s-%s", conf.Loc.Line, conf.Loc.Process, conf.Loc.Aisle, n),
					}
					return
				}

				defer func() {
					// always try to close it out
					_ = dev.Close()
				}()

				logger.Debugw("created ISOTP interface", "FXR", n)

				if err = dev.SetCANFD(); err != nil {
					logger.Errorw("set CANFD on ISTOP interface", "FXR", n, "error", err)
					avail <- traycontrollers.FXRAvailable{
						Location: fmt.Sprintf("%s-%s%s-%s", conf.Loc.Line, conf.Loc.Process, conf.Loc.Aisle, n),
					}
					return
				}

				logger.Debugw("set CANFD flags", "FXR", n)

				if err = dev.SetRecvTimeout(time.Second * 3); err != nil {
					logger.Errorw("set recv timeout on ISOTP interface", "FXR", n, "error", err)
					avail <- traycontrollers.FXRAvailable{
						Location: fmt.Sprintf("%s-%s%s-%s", conf.Loc.Line, conf.Loc.Process, conf.Loc.Aisle, n),
					}

					return
				}

				logger.Debugw("set recv timeout", "FXR", n)

				for {
					var buf []byte

					if buf, err = dev.RecvBuf(); err != nil {
						// only a warn because a timeout could have occurred which isn't as drastic
						logger.Warnw("receive buffer", "FXR", n, "error", err)
						avail <- traycontrollers.FXRAvailable{
							Location: fmt.Sprintf("%s-%s%s-%s", conf.Loc.Line, conf.Loc.Process, conf.Loc.Aisle, n),
						}

						return
					}

					logger.Debugw("received message from FXR", "FXR", n)

					if err = proto.Unmarshal(buf, &msg); err != nil {
						logger.Debugw("not the message we were expecting", "error", err)
						continue
					}

					break
				}

				logger.Debugw("fixture status", "FXR", n, "status", msg.GetOp().GetStatus().String())

				fxrInfo, ok := registry[n]
				if !ok {
					logger.Warnw("fixture not in registry", "fixture", n)
					avail <- traycontrollers.FXRAvailable{
						Location: fmt.Sprintf("%s-%s%s-%s", conf.Loc.Line, conf.Loc.Process, conf.Loc.Aisle, n),
					}

					return
				}

				avail <- traycontrollers.FXRAvailable{
					Location: fmt.Sprintf("%s-%s%s-%s", conf.Loc.Line, conf.Loc.Process, conf.Loc.Aisle, n),
					Status:   msg.GetOp().GetStatus(),
					Reserved: fxrInfo.Avail.Status() == StatusWaitingForLoad,
				}
			}(n, fConf)
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
