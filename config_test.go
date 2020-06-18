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

	if _, err = tf.Write([]byte("recipefile: foo\ningredientsfile: bar")); err != nil {
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

	expRF := "foo"
	if c.RecipeFile != expRF {
		t.Errorf("RecipeFile expected: %s, got: %s", expRF, c.RecipeFile)
	}

	expIF := "bar"
	if c.IngredientsFile != expIF {
		t.Errorf("IngredientsFile expected: %s, got: %s", expIF, c.IngredientsFile)
	}
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
