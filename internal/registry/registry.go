// Package registry wires every capability into the unified tool. Adding a
// capability to Capybari Source Intelligence means importing its analyzer
// repository here; nothing else in the CLI changes.
package registry

import (
	"github.com/capybari/capybari-core/analyzer"
	"github.com/capybari/capybari-core/builtin"
	"github.com/capybari/capybari-core/engine"
)

// Analyzers returns every bundled capability.
func Analyzers() []analyzer.Analyzer {
	// Analyzer repositories are appended to the built-ins here.
	return builtin.All()
}

// New returns a registry with every bundled capability.
func New() *engine.Registry {
	return engine.NewRegistry().MustRegister(Analyzers()...)
}
