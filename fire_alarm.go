package towercontroller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/cenkalti/backoff"
	"go.uber.org/zap"
	"stash.teslamotors.com/rr/cdcontroller"
	pb "stash.teslamotors.com/rr/towerproto"
)

func soundTheAlarm(config Configuration, status pb.FireAlarmStatus, location string, logger *zap.SugaredLogger) error {
	level := cdcontroller.ReasonFireLevel0
	if status == pb.FireAlarmStatus_FIRE_ALARM_LEVEL_1 {
		level = cdcontroller.ReasonFireLevel1
	}

	broadcastReq := cdcontroller.BroadcastRequest{
		Scale:     cdcontroller.ScaleAisle,
		Operation: cdcontroller.OperationPauseFormation, // ignored if fire level is 0
		Reason:    level,
		Originator: cdcontroller.BroadcastOrigin{
			Aisle:    config.Loc.Aisle,
			Location: location,
		},
		ExcludeOrigin: true,
	}

	jb, err := json.Marshal(broadcastReq)
	if err != nil {
		return fmt.Errorf("json marshal broadcast request: %v", err)
	}

	return backoff.Retry(func() error {
		resp, err := http.Post(config.Remote+cdcontroller.BroadcastEndpoint, "application/json", bytes.NewReader(jb))
		if err != nil {
			logger.Warnw("post broadcast request", "error", err)
			return err
		}

		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			rb, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				rb = []byte(fmt.Sprintf("error reading response body: %v", err))
			}

			logger.Warnw("post broadcast request response status code NOT OK",
				"status_code", resp.StatusCode,
				"status", resp.Status,
				"response", string(rb),
			)

			return errors.New("invalid status code")
		}

		return nil
	}, backoff.NewConstantBackOff(time.Second))
}
