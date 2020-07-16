package towercontroller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/rr/protostream"
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

		for n := range conf.Fixtures {
			go func(n string) {
				defer wg.Done()

				cl := logger.With("fixture", n)
				cl.Debug("checking availability on fixture")

				// nolint:govet // allow shadow of err declaration for go routine scope
				var (
					err error
					msg pb.FixtureToTower
				)

				fxrInfo, ok := registry[n]
				if !ok {
					cl.Warn("fixture not in registry")
					avail <- traycontrollers.FXRAvailable{
						Location: fmt.Sprintf("%s-%s%s-%s", conf.Loc.Line, conf.Loc.Process, conf.Loc.Aisle, n),
					}

					return
				}

				for lMsg := range fxrInfo.SC {
					cl.Debugw("got message, checking if fixture is available", "message", lMsg.Msg)

					var event protostream.ProtoMessage
					if err = json.Unmarshal(lMsg.Msg.Body, &event); err != nil {
						cl.Debugw("unmarshal JSON frame", "error", err, "bytes", string(lMsg.Msg.Body))
						continue
					}

					cl.Debug("received message from FXR, unmarshalling to check if it is available")

					if err = proto.Unmarshal(event.Body, &msg); err != nil {
						cl.Debugw("not the message we were expecting", "error", err)
						continue
					}

					if msg.GetOp() == nil {
						cl.Debugw("look for the next message, this is diagnostic", "msg", msg.String())
						continue
					}

					break
				}

				cl.Debugw("fixture status, checking if available", "status", msg.GetOp().GetStatus().String())

				avail <- traycontrollers.FXRAvailable{
					Location: fmt.Sprintf("%s-%s%s-%s", conf.Loc.Line, conf.Loc.Process, conf.Loc.Aisle, n),
					Status:   msg.GetOp().GetStatus(),
					Reserved: fxrInfo.Avail.Status() == StatusWaitingForLoad,
				}
			}(n)
		}

		logger.Debug("waiting for all routines to finish getting status")
		wg.Wait()
		close(avail)

		logger.Debug("waiting for all data to be consumed")
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

		logger.Info("sent response to request to /avail")
	})
}
