package towercontroller

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	tower "stash.teslamotors.com/rr/towerproto"
)

const _latestFixtureVar = "fixture"

const (
	// LatestOpEndpoint returns the latest op message for fixture
	LatestOpEndpoint = "/{fixture}/op"
	// LatestDiagEndpoint returns the latest diag message for fixture
	LatestDiagEndpoint = "/{fixture}/diag"
	// LatestAlertEndpoint returns the latest alert message for fixture
	LatestAlertEndpoint = "/{fixture}/alert"
)

// HandleLatestOp handles incoming requests to get the latest op message for a fixture
func HandleLatestOp(logger *zap.SugaredLogger, registry map[string]*FixtureInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allowCORS(w)

		l := logger.With("endpoint", LatestOpEndpoint, "remote", r.RemoteAddr)
		l.Info("got request to endpoint")

		w.Header().Set("Content-Type", "application/json")

		fxr, err := getFixtureFromPath(r, registry)
		if err != nil {
			l.Errorw("get fixture from path", "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		jb, err := getLatestMessageWithGetter(fxr.GetOp)
		if err != nil {
			l.Errorw("get latest message", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		if _, err := w.Write(jb); err != nil {
			l.Errorw("write response body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		l.Info("responded to request")
	}
}

// HandleLatestDiag handles incoming requests to get the latest diag message for a fixture
func HandleLatestDiag(logger *zap.SugaredLogger, registry map[string]*FixtureInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allowCORS(w)

		l := logger.With("endpoint", LatestOpEndpoint, "remote", r.RemoteAddr)
		l.Info("got request to endpoint")

		w.Header().Set("Content-Type", "application/json")

		fxr, err := getFixtureFromPath(r, registry)
		if err != nil {
			l.Errorw("get fixture from path", "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		jb, err := getLatestMessageWithGetter(fxr.GetDiag)
		if err != nil {
			l.Errorw("get latest message", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		if _, err := w.Write(jb); err != nil {
			l.Errorw("write response body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// HandleLatestAlert handles incoming requests to get the latest alert message for a fixture
func HandleLatestAlert(logger *zap.SugaredLogger, registry map[string]*FixtureInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allowCORS(w)

		l := logger.With("endpoint", LatestOpEndpoint, "remote", r.RemoteAddr)
		l.Info("got request to endpoint")

		w.Header().Set("Content-Type", "application/json")

		fxr, err := getFixtureFromPath(r, registry)
		if err != nil {
			l.Errorw("get fixture from path", "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		jb, err := getLatestMessageWithGetter(fxr.GetAlert)
		if err != nil {
			l.Errorw("get latest message", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		if _, err := w.Write(jb); err != nil {
			l.Errorw("write response body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func getFixtureFromPath(r *http.Request, registry map[string]*FixtureInfo) (*FixtureState, error) {
	fxrVar, ok := mux.Vars(r)[_latestFixtureVar]
	if !ok {
		return nil, errors.New("fixture variable not in path")
	}

	fxr, ok := registry[fxrVar]
	if !ok {
		return nil, fmt.Errorf("fixture %s not in registry", fxrVar)
	}

	return fxr.FixtureState, nil
}

func getLatestMessageWithGetter(getter func() (*tower.FixtureToTower, error)) ([]byte, error) {
	msg, err := getter()
	if err != nil {
		msg = &tower.FixtureToTower{}
	}

	mo := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
	}

	jb, err := mo.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal response body: %v", err)
	}

	return jb, err
}
