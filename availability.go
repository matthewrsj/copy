package towercontroller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"go.uber.org/zap"
	"stash.teslamotors.com/rr/cdcontroller"
	tower "stash.teslamotors.com/rr/towerproto"
)

// AvailabilityEndpoint is the endpoint that handles request for fixture availability
const AvailabilityEndpoint = "/avail"

const _allowedQueryKey = "allowed"

// HandleAvailable is the handler for the endpoint reporting availability of fixtures
// nolint:gocognit,funlen // ignore
func HandleAvailable(configPath string, logger *zap.SugaredLogger, registry map[string]*FixtureInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger = logger.With("endpoint", AvailabilityEndpoint, "remote", r.RemoteAddr)

		logger.Info("got request to endpoint")

		var (
			conf Configuration
			err  error
		)

		if _globalConfiguration != nil {
			conf = *_globalConfiguration
		} else if conf, err = LoadConfig(configPath); err != nil {
			logger.Errorw("read configuration file", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		respondAvailable := conf.AllFixtures

		values := r.URL.Query()

		allowedOnly := values.Get(_allowedQueryKey)
		if allowedOnly == "true" {
			respondAvailable = conf.AllowedFixtures
		}

		type namedAvail struct {
			name  string
			avail cdcontroller.FXRAvailable
		}

		avail := make(chan namedAvail)
		done := make(chan struct{})
		as := make(cdcontroller.Availability)

		go func() {
			for a := range avail {
				as[a.name] = a.avail
			}

			close(done)
		}()

		var wg sync.WaitGroup

		wg.Add(len(respondAvailable))

		for _, n := range respondAvailable {
			go func(n string) {
				defer wg.Done()

				location := fmt.Sprintf("%s-%s%s-%s", conf.Loc.Line, conf.Loc.Process, conf.Loc.Aisle, n)
				cl := logger.With("fixture", location)
				cl.Debug("checking availability on fixture")

				zeroAvail := namedAvail{
					name: location,
					avail: cdcontroller.FXRAvailable{
						Status:          tower.FixtureStatus_FIXTURE_STATUS_UNKNOWN_UNSPECIFIED.String(),
						EquipmentStatus: tower.EquipmentStatus_EQUIPMENT_STATUS_UNKNOWN_UNSPECIFIED.String(),
						Allowed:         fixtureIsAllowed(n, conf.AllowedFixtures),
					},
				}

				fxrInfo, ok := registry[n]
				if !ok {
					cl.Warn("fixture not in registry")
					avail <- zeroAvail

					return
				}

				// nolint:govet // don't care about shadowing above errors, especially when we aren't dealing with concurrency
				msg, err := fxrInfo.FixtureState.GetOp()
				if err != nil {
					cl.Debugw("get fixture operational status", "error", err)
					// wait a second for it to update
					avail <- zeroAvail

					return
				}

				cl.Debugw("fixture status rxd, checking if available", "status", msg.GetOp().GetStatus().String())

				var reserved bool

				switch fxrInfo.Avail.Status() {
				case StatusWaitingForReservation, StatusUnknown:
					reserved = false
				default:
					reserved = true
				}

				avail <- namedAvail{
					name: location,
					avail: cdcontroller.FXRAvailable{
						Status:          msg.GetOp().GetStatus().String(),
						EquipmentStatus: msg.GetOp().GetEquipmentStatus().String(),
						Reserved:        reserved,
						Allowed:         fixtureIsAllowed(n, conf.AllowedFixtures),
					},
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

		w.Header().Set("Content-Type", "application/json")

		if _, err = w.Write(body); err != nil {
			logger.Errorw("write body to responsewriter", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		logger.Info("sent response to request to /avail")
	}
}
