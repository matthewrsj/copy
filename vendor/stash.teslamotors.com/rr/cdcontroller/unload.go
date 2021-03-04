package cdcontroller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"go.uber.org/zap"
	asrsapi "stash.teslamotors.com/cas/asrs/idl/src"
	terminal "stash.teslamotors.com/cas/asrs/terminal/server"
)

// UnloadEndpoint handles incoming POSTs to unload a tray from a fixture
const UnloadEndpoint = "/unload"

type trayComplete struct {
	ID     string `json:"id"`
	Aisle  string `json:"aisle"`
	Column string `json:"column"`
	Level  string `json:"level"`
}

// HandleUnloads handles unloads coming from tower controller
// nolint:gocognit,funlen // TODO: simplify
func HandleUnloads(server *terminal.Server, lg *zap.SugaredLogger, conf Configuration, aisles map[string]*Aisle, inuo chan *asrsapi.UnloadOperation, mockAPI string) http.HandlerFunc {
	c := NewCellAPIClient(
		conf.CellAPIBase,
		WithNextProcessStepFmtEndpoint(conf.CellAPINextProcStepFmt),
		WithCloseProcessFmtEndpoint(conf.CellAPICloseProcessFmt),
	)

	var mx sync.Mutex

	return func(w http.ResponseWriter, r *http.Request) {
		allowCORS(w)

		logger := lg.With("remote", r.RemoteAddr)

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		var tc trayComplete
		if err = json.Unmarshal(b, &tc); err != nil {
			logger.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		logger = logger.With("tray", tc.ID, "aisle", tc.Aisle, "source_tower", tc.Column, "source_column", tc.Column, "source_level", tc.Level)
		logger.Info("tray complete")

		w.WriteHeader(http.StatusOK)

		proc := NextFormationStep{
			Name: mockAPI,
			Step: mockAPI + " - 0",
		}

		logger.Infow("process before call", "proc", proc)

		if mockAPI == "" {
			proc, err = c.GetNextProcessStep(tc.ID[:len(tc.ID)-1])
			if err != nil {
				logger.Errorw("get next process step/recipe", "error", err)
				http.Error(w, err.Error(), http.StatusFailedDependency)

				return
			}
		}

		uo := &asrsapi.UnloadOperation{
			Conversation: server.BuildConversationHeader(asrsapi.MessageIDNone),
			Tray: &asrsapi.Tray{
				TrayId: []string{tc.ID},
			},
			State: &asrsapi.UnloadOperationStateAndStatus{
				StateType: asrsapi.StateType_Current,
				Status: &asrsapi.Status{
					Status: asrsapi.Status_Complete,
				},
			},
			Location: &asrsapi.Location{
				LocationByType: &asrsapi.Location_CmFormat{
					CmFormat: &asrsapi.CMLocation{
						EquipmentId:         fmt.Sprintf("CM2-63%s-%s-%s", tc.Aisle, tc.Column, tc.Level),
						ManufacturingSystem: "CM2",
						Workcenter:          "63",
						Equipment:           tc.Aisle,
						Workstation:         tc.Column,
						SubIdentifier:       tc.Level,
					},
				},
			},
		}

		// remove spaces and _nonProdPrefix then check if it's a commissioning tray
		stepName := strings.TrimPrefix(strings.TrimSpace(strings.Split(proc.Step, " - ")[0]), _nonProdPrefix)
		isCommissionRecipe := strings.HasPrefix(stepName, CommissionSelfTestRecipeName)
		logger.Debugw("step name", "step", stepName)

		switch {
		case isCommissionRecipe:
			mx.Lock() // only handle one reload at a time as this is not serialized and can come in at the exact same time
			defer mx.Unlock()

			logger.Debug("handling reload")

			aisle, ok := aisles[tc.Aisle]
			if !ok {
				logger.Errorw("invalid aisle location", "aisle", tc.Aisle, "error", err)
				http.Error(w, "invalid aisle location", http.StatusBadRequest)

				return
			}

			op, err := getLocation(aisle, logger, []string{tc.ID}, 3 /* timesToTry */, func(t *Tower) (*FXRLayout, *PowerAvailable, error) {
				return t.getAvailabilityForCommissioning()
			})
			if err != nil {
				logger.Infow("no more FXRs need commissioning", "error", err)

				var ver int

				splits := strings.Split(proc.Step, " - ")
				if len(splits) != 2 {
					logger.Warn("unable to parse recipe version from step string, using 0", "step", proc.Step)
				} else {
					ver, err = strconv.Atoi(strings.TrimSpace(splits[1]))
					if err != nil {
						logger.Warnw("unable to parse recipe version from step string, using 0", "step", proc.Step)
					}
				}

				if mockAPI == "" {
					if err = c.CloseProcessStep(tc.ID[:len(tc.ID)-1], proc.Name, ver); err != nil {
						logger.Warnw("unable to close process step", "error", err)
					}
				}

				uo.State.State = asrsapi.UnloadOperationState_Executed

				// do not reload, no more FXRs need commissioning
				break
			}

			uo.State.State = asrsapi.UnloadOperationState_PreparedToReload
			uo.Destination = &asrsapi.Location{
				LocationByType: &asrsapi.Location_CmFormat{
					CmFormat: &asrsapi.CMLocation{
						EquipmentId:         fmt.Sprintf("CM2-63%s-%02d-%02d", tc.Aisle, op.front.fixture.Coord.Col, op.front.fixture.Coord.Lvl),
						ManufacturingSystem: "CM2",
						Workcenter:          "63",
						Equipment:           tc.Aisle,
						Workstation:         fmt.Sprintf("%02d", op.front.fixture.Coord.Col),
						SubIdentifier:       fmt.Sprintf("%02d", op.front.fixture.Coord.Lvl),
					},
				},
			}

			if err = reserveOnTower(op.front.tower, tc.ID, uo.GetDestination().GetCmFormat().GetEquipmentId()); err != nil {
				logger.Warnw("unable to reserve fixture on tower", "error", err)
			}
		default:
			logger.Debug("handling unload")

			uo.State.State = asrsapi.UnloadOperationState_Executed
		}

		logger.Info("informing CND")
		inuo <- uo
	}
}
