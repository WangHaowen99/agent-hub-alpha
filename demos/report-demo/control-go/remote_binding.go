package main

import (
	"encoding/json"
	"strings"
)

// This governed catalog entry is part of the admitted catalog/binding digest.
// CardDigest, when set, hashes the exact UTF-8 bytes of RDC card.rawJson.
type RemoteBinding struct {
	CardVersion       string `json:"card_version"`
	ContractVersion   string `json:"contract_version"`
	CapabilityVersion string `json:"capability_version"`
	CardDigest        string `json:"card_digest,omitempty"`
}

func (b RemoteBinding) validate() error {
	if strings.TrimSpace(b.CardVersion) == "" || strings.TrimSpace(b.ContractVersion) == "" || strings.TrimSpace(b.CapabilityVersion) == "" {
		return fault("POLICY_REJECTED", "remote binding requires three explicit versions", 422)
	}
	if b.CardDigest != "" && (!strings.HasPrefix(b.CardDigest, "sha256:") || !blobID.MatchString(strings.TrimPrefix(b.CardDigest, "sha256:"))) {
		return fault("POLICY_REJECTED", "remote card digest must be sha256", 422)
	}
	return nil
}

func validateRoute(cap Capability, raw json.RawMessage) error {
	if err := cap.RemoteBinding.validate(); err != nil {
		return err
	}
	var route struct {
		AgentID string `json:"agent_id"`
		Card    struct {
			Name string `json:"name"`
			Raw  string `json:"rawJson"`
		} `json:"card"`
		Instances struct {
			Instances []struct {
				Health     string `json:"health"`
				Contract   string `json:"contractVersion"`
				Capability string `json:"capabilityVersion"`
				Card       struct {
					Name string `json:"name"`
				} `json:"card"`
			} `json:"instances"`
		} `json:"instances"`
	}
	// RDC snapshots have provider metadata beyond these fields. Decode strict
	// JSON first, then project the fields used for binding validation.
	var doc any
	if err := DecodeStrict(raw, &doc); err != nil {
		return fault("DISCOVERY_BINDING_CHANGED", "invalid discovery JSON", 409)
	}
	if err := json.Unmarshal(raw, &route); err != nil {
		return fault("DISCOVERY_BINDING_CHANGED", "invalid discovery shape", 409)
	}
	var card struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	var cardDoc any
	if err := DecodeStrict([]byte(route.Card.Raw), &cardDoc); err != nil {
		return fault("DISCOVERY_BINDING_CHANGED", "invalid AgentCard", 409)
	}
	if err := json.Unmarshal([]byte(route.Card.Raw), &card); err != nil {
		return fault("DISCOVERY_BINDING_CHANGED", "invalid AgentCard fields", 409)
	}
	b := cap.RemoteBinding
	if route.AgentID != cap.AgentID || route.Card.Name != cap.AgentID || card.Name != cap.AgentID || card.Version != b.CardVersion ||
		(b.CardDigest != "" && "sha256:"+tokenDigest(route.Card.Raw) != b.CardDigest) {
		return fault("DISCOVERY_BINDING_CHANGED", "discovery differs from admitted capability", 409)
	}
	for _, instance := range route.Instances.Instances {
		if instance.Health == "ONLINE" && instance.Contract == b.ContractVersion && instance.Capability == b.CapabilityVersion && instance.Card.Name == cap.AgentID {
			return nil
		}
	}
	return fault("DISCOVERY_UNAVAILABLE", "no instance matches admitted capability versions", 503)
}
