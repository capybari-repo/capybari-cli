// Package registry wires every capability into the unified tool. Adding a
// capability to Capybari Source Intelligence means importing its analyzer
// repository here; nothing else changes. It is a public package so the
// hosted service runs exactly the same capability set as the CLI.
package registry

import (
	aisignals "github.com/capybari-repo/capybari-analyzer-ai-signals"
	architecture "github.com/capybari-repo/capybari-analyzer-architecture"
	codehealth "github.com/capybari-repo/capybari-analyzer-code-health"
	commerce "github.com/capybari-repo/capybari-analyzer-commerce"
	dependencies "github.com/capybari-repo/capybari-analyzer-dependencies"
	fingerprint "github.com/capybari-repo/capybari-analyzer-fingerprint"
	secrets "github.com/capybari-repo/capybari-analyzer-secrets"
	techdetect "github.com/capybari-repo/capybari-analyzer-tech-detect"
	vulns "github.com/capybari-repo/capybari-analyzer-vulns"
	websecurity "github.com/capybari-repo/capybari-analyzer-web-security"
	webtech "github.com/capybari-repo/capybari-analyzer-web-tech"
	"github.com/capybari-repo/capybari-core/analyzer"
	"github.com/capybari-repo/capybari-core/builtin"
	"github.com/capybari-repo/capybari-core/engine"
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
		commerce.New(),
		aisignals.New(),
	)
}

// New returns a registry with every bundled capability.
func New() *engine.Registry {
	return engine.NewRegistry().MustRegister(Analyzers()...)
}
