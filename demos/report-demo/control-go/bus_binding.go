package main

import "strings"

// BusBinding identifies a logical deployment and trust domain, not a network
// route. Proxies and rotating credentials do not change an approved task.
type BusBinding struct {
	EnvironmentID string `json:"environment_id"`
	Tenant        string `json:"tenant"`
	Issuer        string `json:"issuer"`
	Audience      string `json:"audience"`
	CallerSubject string `json:"caller_subject"`
}

func (b *BusBinding) valid() bool {
	return b != nil && strings.TrimSpace(b.EnvironmentID) != "" && strings.TrimSpace(b.Tenant) != "" &&
		strings.TrimSpace(b.Issuer) != "" && strings.TrimSpace(b.Audience) != "" && strings.TrimSpace(b.CallerSubject) != ""
}

func sameBusBinding(a, b *BusBinding) bool {
	if a == nil || b == nil {
		return a == nil && b == nil // Unpublished, model-free control fixtures only.
	}
	return a.valid() && b.valid() && *a == *b
}

func (s *Store) checkBusBinding(t *Task) error {
	if !sameBusBinding(t.BusBinding, s.BusBinding) {
		return fault("BUS_BINDING_CHANGED", "Bus environment differs or historical binding is missing; review a new candidate", 409)
	}
	return nil
}
