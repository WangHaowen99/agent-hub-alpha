package main

import (
	"fmt"
	"sync"
	"time"
)

// Provider simulates the business API behind the effect boundary. It owns
// its own outcome (Go only stores observations of it) and is idempotent on
// the business request id: a second submit of the same request returns the
// existing application instead of creating another one.
type Provider struct {
	mu      sync.Mutex
	apps    map[string]*providerApp
	observe func(requestID, status, eventID string)
	// Delays are fields so tests can shrink them.
	AcceptDelay   time.Duration
	TerminalDelay time.Duration
	CallTimeout   time.Duration
}

type providerApp struct {
	status   string // pending, approved, declined, cancelled
	scenario string
}

func NewProvider(observe func(requestID, status, eventID string)) *Provider {
	return &Provider{
		apps:          map[string]*providerApp{},
		observe:       observe,
		AcceptDelay:   300 * time.Millisecond,
		TerminalDelay: 8 * time.Second,
		CallTimeout:   2 * time.Second,
	}
}

// Submit is one dispatch attempt. It returns the synchronous receipt, or an
// error when the caller cannot know the outcome (timeout, lost response):
// the application may still have been accepted.
func (p *Provider) Submit(requestID, scenario string) (string, error) {
	p.mu.Lock()
	app, exists := p.apps[requestID]
	if !exists {
		app = &providerApp{status: "pending", scenario: scenario}
		p.apps[requestID] = app
		p.scheduleTerminal(requestID, app)
	}
	status := app.status
	p.mu.Unlock()
	if exists {
		return status, nil // idempotent replay of the current receipt
	}
	switch scenario {
	case "timeout":
		time.Sleep(p.CallTimeout)
		return "", fmt.Errorf("provider call timed out after %s", p.CallTimeout)
	case "lost_receipt":
		return "", fmt.Errorf("connection reset before the receipt arrived")
	}
	time.Sleep(p.AcceptDelay)
	return status, nil
}

func (p *Provider) scheduleTerminal(requestID string, app *providerApp) {
	time.AfterFunc(p.TerminalDelay, func() {
		p.mu.Lock()
		if app.status != "pending" { // cancelled meanwhile
			p.mu.Unlock()
			return
		}
		app.status = "approved"
		if app.scenario == "decline" {
			app.status = "declined"
		}
		final, scenario := app.status, app.scenario
		p.mu.Unlock()
		// Timeout and lost-receipt deliver no callback: Go has to query.
		// duplicate_receipt repeats the terminal event and then sends a
		// stale "pending" that must not regress the observed state.
		switch scenario {
		case "timeout", "lost_receipt":
		case "duplicate_receipt":
			p.observe(requestID, final, requestID+"-terminal")
			p.observe(requestID, final, requestID+"-terminal")
			p.observe(requestID, "pending", requestID+"-late")
		default:
			p.observe(requestID, final, requestID+"-terminal")
		}
	})
}

// Query reads the provider's current status; "" means it has no record.
func (p *Provider) Query(requestID string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if app, ok := p.apps[requestID]; ok {
		return app.status
	}
	return ""
}

// Cancel is a separate request. Only a pending application can be
// cancelled; the answer is the provider's, not the caller's.
func (p *Provider) Cancel(requestID string) (bool, string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	app, ok := p.apps[requestID]
	if !ok {
		return false, "unknown request"
	}
	if app.status == "pending" {
		app.status = "cancelled"
		return true, "cancelled"
	}
	return false, "already " + app.status
}
