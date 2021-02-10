package towercontroller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"bou.ke/monkey"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type mockResponseWriter struct{}

func (m mockResponseWriter) Header() http.Header {
	return http.Header{}
}

func (m mockResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (m mockResponseWriter) WriteHeader(int) {}

func TestHandleUpdateCancelNotScheduled(t *testing.T) {
	c := make(chan struct{})
	hf := HandleUpdateCancel(zap.NewExample().Sugar(), c)

	go hf(mockResponseWriter{}, &http.Request{})

	time.Sleep(time.Millisecond * 100)

	var hit bool

	select {
	case <-c:
		hit = true
	default:
	}

	assert.False(t, hit)
}

func TestHandleUpdateCancel(t *testing.T) {
	c := make(chan struct{})
	hf := HandleUpdateCancel(zap.NewExample().Sugar(), c)

	_updateScheduled = true

	defer func() {
		_updateScheduled = false
	}()

	go hf(mockResponseWriter{}, &http.Request{})

	time.Sleep(time.Millisecond * 100)

	var hit bool

	select {
	case <-c:
		hit = true
	default:
	}

	assert.True(t, hit)
}

func TestHandleUpdateForce(t *testing.T) {
	cancel := make(chan struct{})
	srv := httptest.NewServer(HandleUpdate(zap.NewExample().Sugar(), cancel, make(map[string]*FixtureInfo)))

	var calledWith int

	exitP := monkey.Patch(os.Exit, func(code int) {
		calledWith = code
	})
	defer exitP.Unpatch()

	r, err := http.Post(fmt.Sprintf("%s?%s=true", srv.URL, _forceQueryKey), "application/json", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = r.Body.Close()
	}()

	time.Sleep(time.Millisecond * 110)

	assert.Equal(t, _exitDueToUpdateRequest, calledWith)
	assert.Equal(t, http.StatusOK, r.StatusCode)
}
