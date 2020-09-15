package towercontroller

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestMonitorConfig(t *testing.T) {
	tf, err := ioutil.TempFile(".", "")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = os.Remove(tf.Name())
	}()

	initial := _globalConfiguration

	defer func() {
		_globalConfiguration = initial
	}()

	go MonitorConfig(zap.NewExample().Sugar(), tf.Name(), &Configuration{})

	time.Sleep(time.Millisecond * 200)

	if _, err = tf.Write([]byte("cell_api:\n base: foo\n")); err != nil {
		t.Fatal(err)
	}

	if err = tf.Close(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond * 200)
	assert.Equal(t, "foo", _globalConfiguration.CellAPI.Base)
}
