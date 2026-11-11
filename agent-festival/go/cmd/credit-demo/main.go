// credit-demo shows a dynamic plan (f) made explicit and then executed under
// agent-festival contracts: WorkSwarm's planner proposes a team of registered
// experts, yaojun's orchestration module compiles it into a blueprint, the
// festival checkers gate it before approval, an openJiuwen executor runs it
// step by step against this service's ledger, and the festival replay
// verifies the record afterwards. Demo only — not a production component.
package main

import (
	"log"
	"net/http"
	"strconv"
	"strings"
)

func main() {
	lease, _ := strconv.Atoi(envOr("CREDIT_LEASE_SECONDS", "30"))
	cfg := Config{
		Addr:            envOr("CREDIT_ADDR", ":8091"),
		DataDir:         envOr("CREDIT_DATA", "/data"),
		OrchestratorURL: envOr("CREDIT_ORCHESTRATOR_URL", "http://orchestrator:8195"),
		PublicURL:       strings.TrimRight(envOr("CREDIT_PUBLIC_URL", "/credit"), "/"),
		Origins:         strings.Split(envOr("CREDIT_ORIGINS", "http://localhost:15192"), ","),
		MemberMCPURL:    envOr("CREDIT_MEMBER_MCP_URL", "http://credit:8091/member-mcp"),
		Model:           envOr("CREDIT_MODEL", "qwen-plus"),
		SpecPath:        envOr("CREDIT_SPEC", "judgment-spec-credit.json"),
		LeaseSeconds:    lease,
	}
	s, err := OpenService(cfg)
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler(s.plannerServer()))
	for _, e := range experts {
		if e.Kind == "investigate" || e.Kind == "blackbox" {
			mux.Handle("/member-mcp/"+e.ID, mcpHandler(s.memberServer(e)))
		}
	}
	s.routes(mux)
	log.Printf("credit-demo on %s (planner MCP /mcp, member MCP /member-mcp, UI /ui/)", cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, mux))
}
