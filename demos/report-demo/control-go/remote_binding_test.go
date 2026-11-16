package main

import (
	"encoding/json"
	"testing"
)

func routeFixture(t *testing.T, cat Catalog, expert string) json.RawMessage {
	t.Helper()
	cap, ok := cat.Get(expert)
	if !ok {
		t.Fatal("missing fixture capability")
	}
	b := cap.RemoteBinding
	if b == nil {
		b = &RemoteBinding{"1.0.0", "1.0.0", "1.0.0", ""}
	}
	card, _ := json.Marshal(map[string]string{"name": cap.AgentID, "version": b.CardVersion})
	raw, _ := json.Marshal(map[string]any{"agent_id": cap.AgentID, "card": map[string]string{"name": cap.AgentID, "rawJson": string(card)}, "instances": map[string]any{"instances": []any{map[string]any{"health": "ONLINE", "contractVersion": b.ContractVersion, "capabilityVersion": b.CapabilityVersion, "card": map[string]string{"name": cap.AgentID}}}}})
	return raw
}

func TestDiscoveryUsesAdmittedVersionsAndCardDigest(t *testing.T) {
	cat, _ := sample(t)
	cap, _ := cat.Get("extractor@1")
	cap.RemoteBinding = &RemoteBinding{"2.3.0", "3.1.0", "4.0.0", ""}
	cat.Capabilities[0] = cap
	raw := routeFixture(t, cat, cap.ID)
	if err := validateRoute(cap, raw); err != nil {
		t.Fatal(err)
	}
	var route map[string]any
	json.Unmarshal(raw, &route)
	card := route["card"].(map[string]any)["rawJson"].(string)
	cap.RemoteBinding.CardDigest = "sha256:" + tokenDigest(card)
	if err := validateRoute(cap, raw); err != nil {
		t.Fatal(err)
	}
	cap.RemoteBinding.CardDigest = "sha256:" + tokenDigest("other card")
	requireCode(t, validateRoute(cap, raw), "DISCOVERY_BINDING_CHANGED")
	cap.RemoteBinding.CardDigest = ""
	cap.RemoteBinding.CardVersion = "different"
	requireCode(t, validateRoute(cap, raw), "DISCOVERY_BINDING_CHANGED")
	cap.RemoteBinding.CardVersion = "2.3.0"
	cap.RemoteBinding.ContractVersion = "different"
	requireCode(t, validateRoute(cap, raw), "DISCOVERY_UNAVAILABLE")
	cap.RemoteBinding.ContractVersion = ""
	requireCode(t, validateRoute(cap, raw), "POLICY_REJECTED")
}
