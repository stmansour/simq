package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// SvcStatus200 is a simple status message return
type SvcStatus200 struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// SvcStatus201 is a simple status message for use when a new resource is created
type SvcStatus201 struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	ID      int    `json:"id"`
}

// SvcWrite is a general write routine for service calls... it is a bottleneck
// where we can place debug statements as needed.
func SvcWrite(w http.ResponseWriter, b []byte) {
	w.Write(b)
}

// SvcErrorReturn formats an error return to the grid widget and sends it
// func SvcErrorReturn(w http.ResponseWriter, err error, funcname string) {
// 	// rlib.Console("<Function>: %s | <Error Message>: <<begin>>\n%s\n<<end>>\n", funcname, err.Error())
// 	var e SvcStatus200
// 	e.Status = "error"
// 	e.Message = err.Error()
// 	w.Header().Set("Content-Type", "application/json	")
// 	b, _ := json.Marshal(e)
// 	SvcWrite(w, b)
// }

// SvcWriteResponse finishes the transaction with the W2UI client
func SvcWriteResponse(w http.ResponseWriter, g interface{}) {
	w.Header().Set("Content-Type", "application/json") // we're marshaling the data as json
	b, err := json.Marshal(g)
	if err != nil {
		LogAndErrorReturn(w, fmt.Errorf("error marshaling json data: %s", err.Error()))
		return
	}
	SvcWrite(w, b)
}
