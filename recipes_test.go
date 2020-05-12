package towercontroller

import (
	"io/ioutil"
	"os"
	"testing"
)

const (
	_ingContents = `
PRECHARGE:
  mode: FORM_REQ_CC
  charge_current: 2.6  # Amps
  max_current: 3       # Amps limited to this value chg/dchg
  cut_off_voltage: 3.2 # Voltage where cells are dropped out
  cut_off_current: 0   # NA
  cell_drop_out_v: 0   # NA
  step_timeout: 3600   # 1 hour - Seconds before step timeout

FIRST_CHARGE_CC:
  mode: FORM_REQ_CC
  charge_current: 8.67
  max_current: 9
  cut_off_voltage: 3.7
  cut_off_current: 0
  cell_drop_out_v: 0
  step_timeout: 7200

FIRST_CHARGE_CV:
  mode: FORM_REQ_CV
  charge_current: 8.67
  max_current: 9
  cut_off_voltage: 3.7
  cut_off_current: 1.3 # A
  cell_drop_out_v: 0
  step_timeout: 9000 # 2.5 hours
`
	_recContents = `
FORM_PRECHARGE:
  - PRECHARGE

FORM_FIRST_CHARGE:
  - FIRST_CHARGE_CC
  - FIRST_CHARGE_CV
`
)

func Test_loadRecipe(t *testing.T) {
	itf, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = os.Remove(itf.Name())
	}()

	if _, err = itf.Write([]byte(_ingContents)); err != nil {
		// don't fatal so the defer will be called
		t.Error(err)
		return
	}

	_ = itf.Close()

	rtf, err := ioutil.TempFile("", "")
	if err != nil {
		// don't fatal so the defer will be called
		t.Error(err)
		return
	}

	defer func() {
		_ = os.Remove(rtf.Name())
	}()

	if _, err = rtf.Write([]byte(_recContents)); err != nil {
		// don't fatal so the defer will be called
		t.Error(err)
		return
	}

	_ = rtf.Close()

	rName := "FORM_FIRST_CHARGE"

	ings, err := loadRecipe(rtf.Name(), itf.Name(), rName)
	if err != nil {
		// don't fatal so the defer will be called
		t.Error(err)
		return
	}

	if len(ings) != 2 {
		// don't fatal so the defer will be called
		t.Errorf("expected two steps, got %d", len(ings))
		return
	}

	expIngFormReqs := []string{"FORM_REQ_CC", "FORM_REQ_CV"}
	for i, ing := range ings {
		if ing.Mode != expIngFormReqs[i] {
			t.Errorf("step %d Mode got %s, expect %s", i, ing.Mode, expIngFormReqs[i])
		}
	}
}
