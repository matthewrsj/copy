package towercontroller

import (
	"net/http"
	"testing"
	"time"

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
