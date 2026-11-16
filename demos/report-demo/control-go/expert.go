package main

import (
	"net/http"

	"github.com/jackc/pgx/v5"
)

func (s *Server) expertRoutes(m *http.ServeMux) {
	m.HandleFunc("POST /v1/experts/{agent}/calls/{call}/authorize", s.wrap(nil, func(w http.ResponseWriter, r *http.Request) (any, error) {
		agent := r.PathValue("agent")
		if _, err := s.authorize(r, "expert:"+agent); err != nil {
			return nil, err
		}
		var in struct {
			InputDigest string `json:"input_digest"`
			Attempt     int    `json:"attempt"`
			ContextID   string `json:"context_id"`
		}
		if err := readBody(r, &in); err != nil {
			return nil, err
		}
		var taskID string
		err := s.Store.Pool.QueryRow(r.Context(), "SELECT task_id FROM remote_calls WHERE scope=$1 AND id=$2", s.Store.Scope, r.PathValue("call")).Scan(&taskID)
		if err == pgx.ErrNoRows {
			return nil, fault("NOT_FOUND", "call not found", 404)
		}
		if err != nil {
			return nil, err
		}
		return s.Store.mutate(r.Context(), taskID, "EXPERT_INPUT_AUTHORIZED", func(tx pgx.Tx, t *Task) (any, error) {
			var callRaw []byte
			if err := tx.QueryRow(r.Context(), "SELECT document FROM remote_calls WHERE scope=$1 AND id=$2", s.Store.Scope, r.PathValue("call")).Scan(&callRaw); err != nil {
				return nil, err
			}
			var call Call
			if err := DecodeStrict(callRaw, &call); err != nil {
				return nil, err
			}
			if call.AgentID != agent || call.InputDigest != in.InputDigest || call.Attempt != in.Attempt || call.ContextID != in.ContextID || call.Conflicted {
				return nil, fault("FORBIDDEN", "expert/call/input association mismatch", 403)
			}
			if t.Pause {
				return nil, fault("PAUSED", "new expert admission paused", 409)
			}
			ns := t.Nodes[call.NodeID]
			if ns == nil || ns.Result != nil || ns.Attempt != call.Attempt {
				return nil, fault("FENCED", "remote attempt no longer admitted", 403)
			}
			var active bool
			err := tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM run_leases WHERE scope=$1 AND task_id=$2 AND domain='graph' AND epoch::text=$3 AND control_session=$4 AND status='ACTIVE' AND expires_at>clock_timestamp())", s.Store.Scope, taskID, ns.OwnerEpoch, s.Store.Session).Scan(&active)
			if err != nil {
				return nil, err
			}
			if !active {
				return nil, fault("LEASE_EXPIRED", "expert input permit expired", 403)
			}
			binding, err := s.Store.Binding(r.Context(), t, call.NodeID, s.Catalog, s.Material.Digest)
			if err != nil {
				return nil, err
			}
			h, err := Digest(binding)
			if err != nil {
				return nil, err
			}
			if h != call.InputDigest {
				return nil, fault("INPUT_CONFLICT", "actual bound input changed", 409)
			}
			return map[string]any{"binding": binding, "input_digest": h, "material": s.Material, "call_id": call.ID, "attempt": call.Attempt}, nil
		})
	}))
}
