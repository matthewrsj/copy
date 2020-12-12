package towercontroller

import "net/http"

func allowCORS(w http.ResponseWriter) {
	// TODO: more fine-grained control
	w.Header().Set("Access-Control-Allow-Origin", "*")
}
