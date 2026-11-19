package main

import (
	"errors"
	"net/http"
)

// Typed control-protocol errors (AR-07, #27). Every /exec and /agent endpoint
// answers an error as {"error": message, "code": CODE}. The code, not the
// HTTP status, tells the executor what happened: losing the lease, an input
// conflict, a storage failure and a rejected judgment all need different
// handling, and several of them share 409.
var errJudgmentRejected = errors.New("POLICY_REJECTED")
var errAdoptionRejected = errors.New("CONTROL_REJECTED")

// errorCode maps an error to its HTTP status and protocol code.
func errorCode(err error) (int, string) {
	switch {
	case errors.Is(err, errStorage):
		return http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE" // persistent outcome may be partial/uncertain; repair and reconcile before retry
	case errors.Is(err, errFenced):
		return http.StatusConflict, "FENCED" // stop writing: the lease belongs to another executor or the run stopped
	case errors.Is(err, errCallConflict):
		return http.StatusConflict, "INPUT_CONFLICT" // same identity, different input or binding: never resend
	case errors.Is(err, errModelBinding):
		return http.StatusConflict, "MODEL_BINDING_MISMATCH" // run blocked before the model call
	case errors.Is(err, errAdoptionRejected):
		return http.StatusConflict, "CONTROL_REJECTED" // framework/schema/state rejection, not revisable model content
	case errors.Is(err, errJudgmentRejected):
		return http.StatusUnprocessableEntity, "POLICY_REJECTED" // content rejected: a revised judgment may be submitted
	default:
		return http.StatusBadRequest, "INVALID_REQUEST"
	}
}
