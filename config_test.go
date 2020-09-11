package towercontroller

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	tf, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = os.Remove(tf.Name())
	}()

	if _, err = tf.Write([]byte("cell_api:\n base: foo")); err != nil {
		// don't fatal so the defer will be called
		t.Error(err)
		return
	}

	_ = tf.Close()

	c, err := LoadConfig(tf.Name())
	if err != nil {
		// don't fatal so the defer will be called
		t.Error(err)
		return
	}

	assert.Equal(t, "foo", c.CellAPI.Base)
}

func TestLoadConfigNoFile(t *testing.T) {
	if _, err := LoadConfig("DNE"); err == nil {
		t.Error("expected error from non-existent file but got none")
	}
}

func TestLoadConfigBadContents(t *testing.T) {
	tf, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = os.Remove(tf.Name())
	}()

	if _, err = tf.Write([]byte("this is not yaml ;;;")); err != nil {
		// don't fatal so the defer will be called
		t.Error(err)
		return
	}

	_ = tf.Close()

	_, err = LoadConfig(tf.Name())
	assert.NotNil(t, err)
}
