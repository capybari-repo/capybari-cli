// Package registry wires every capability into the unified tool. Adding a
// capability to Capybari Source Intelligence means importing its analyzer
// repository here; nothing else changes. It is a public package so the
// hosted service runs exactly the same capability set as the CLI.
package registry

import (
	aisignals "github.com/capybari/capybari-analyzer-ai-signals"
	architecture "github.com/capybari/capybari-analyzer-architecture"
	codehealth "github.com/capybari/capybari-analyzer-code-health"
	dependencies "github.com/capybari/capybari-analyzer-dependencies"
	fingerprint "github.com/capybari/capybari-analyzer-fingerprint"
	secrets "github.com/capybari/capybari-analyzer-secrets"
	techdetect "github.com/capybari/capybari-analyzer-tech-detect"
	vulns "github.com/capybari/capybari-analyzer-vulns"
	websecurity "github.com/capybari/capybari-analyzer-web-security"
	webtech "github.com/capybari/capybari-analyzer-web-tech"
	"github.com/capybari/capybari-core/analyzer"
	"github.com/capybari/capybari-core/builtin"
	"github.com/capybari/capybari-core/engine"
)

// Analyzers returns every bundled capability.
func Analyzers() []analyzer.Analyzer {
	return append(builtin.All(),
		fingerprint.New(),
		techdetect.New(),
		dependencies.New(),
		vulns.New(),
		secrets.New(),
		codehealth.New(),
		architecture.New(),
		webtech.New(),
		websecurity.New(),
		aisignals.New(),
	)
}

// New returns a registry with every bundled capability.
func New() *engine.Registry {
	return engine.NewRegistry().MustRegister(Analyzers()...)
}
