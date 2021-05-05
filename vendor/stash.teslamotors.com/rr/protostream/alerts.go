package protostream

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	tower "stash.teslamotors.com/rr/towerproto"
)

func traceAlert(logger *zap.SugaredLogger, logDir string, nodeID string, msg *tower.FixtureToTower) {
	if _, ok := msg.GetContent().(*tower.FixtureToTower_AlertLog); !ok {
		logger.Debugw("trace alert, message not alert", "type", fmt.Sprintf("%T", msg.GetContent()))
		return
	}

	mo := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
	}

	jb, err := mo.Marshal(msg)
	if err != nil {
		logger.Warnw("unable to log alert", "error", err)
		return
	}

	f, err := os.OpenFile(filepath.Join(logDir, fmt.Sprintf("%s.alert.log", nodeID)), os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModePerm)
	if err != nil {
		logger.Warnw("unable to open alert log file", "error", err)
		return
	}

	defer func() {
		_ = f.Close()
	}()

	header := fmt.Sprintf("\n%s:\t", time.Now().String())

	_, err = f.Write(append([]byte(header), jb...))
	if err != nil {
		logger.Warnw("unable to write to alert log file", "error", err)
	}
}

const (
	_alertsLogEndpoint = "/alerts"
	_alertsFixtureKey  = "fxr"
	_alertsTailKey     = "n"
)

// HandleAlerts serves the alerts for a fixture queried (fxr). Returns the last N lines (n)
func HandleAlerts(mux *http.ServeMux, logDir string) {
	mux.HandleFunc(_alertsLogEndpoint, func(w http.ResponseWriter, r *http.Request) {
		allowCORS(w)
		if r.Method != http.MethodGet {
			http.Error(w, "only GET supported for this endpoint", http.StatusBadRequest)
			return
		}

		id := r.URL.Query().Get(_alertsFixtureKey)
		if id == "" {
			http.Error(w, "must provide fxr query parameter", http.StatusBadRequest)
			return
		}

		n := r.URL.Query().Get(_alertsTailKey)
		nv, _ := strconv.ParseInt(n, 10, 64) // if an error occurs, just use 0 to mean "all"

		fPath := filepath.Join(logDir, fmt.Sprintf("%s.alert.log", id))

		b, err := ioutil.ReadFile(fPath)
		if err != nil {
			http.Error(w, fmt.Errorf("read alerts log file: %v", err).Error(), http.StatusInternalServerError)
			return
		}

		bs := bytes.Split(b, []byte("\n"))

		if nv < 1 || nv > int64(len(bs)) {
			nv = int64(len(bs))
		}

		res := make([][]byte, nv)

		var i int
		for i = 0; i < len(bs) && i < int(nv); i++ {
			t := bs[len(bs)-1-i]
			res[i] = t
		}

		res = res[:i]

		b = bytes.Join(res, []byte("\n"))

		if _, err = w.Write(b); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)
	})
}
