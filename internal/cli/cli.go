// Package cli implements the `capybari` command.
package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/capybari/capybari-core/analyzer"
	"github.com/capybari/capybari-core/cache"
	"github.com/capybari/capybari-core/engine"
	"github.com/capybari/capybari-core/finding"
	"github.com/capybari/capybari-core/report"
	"github.com/capybari/capybari-core/target"
	"github.com/capybari/capybari-schemas"

	"github.com/capybari/capybari-cli/registry"
)

// Exit codes.
const (
	ExitOK     = 0
	ExitError  = 1
	ExitUsage  = 2
	ExitPolicy = 3 // --fail-on threshold reached
)

// Build information, set by main via ldflags.
type Build struct {
	Version string
	Commit  string
}

// App holds I/O for a CLI invocation.
type App struct {
	Build  Build
	Stdout io.Writer
	Stderr io.Writer
	// Registry overrides the bundled capabilities (tests).
	Registry *engine.Registry
	// Env returns environment variables (tests).
	Env func(string) string
}

func (a *App) registry() *engine.Registry {
	if a.Registry != nil {
		return a.Registry
	}
	return registry.New()
}

func (a *App) env(k string) string {
	if a.Env != nil {
		return a.Env(k)
	}
	return os.Getenv(k)
}

const usage = `Capybari Source Intelligence: a free technical X-ray of your software.

Usage:
  capybari analyze <target> [flags]   Analyze a folder, archive, repository URL or website URL
  capybari plan <target> [flags]      Show which capabilities would run, without running them
  capybari capabilities [--json]      List the bundled capabilities
  capybari validate <report.json>     Check a report against the published JSON Schema
  capybari cache clear                Delete cached results
  capybari version                    Print version information

Run 'capybari analyze -h' for analysis flags.
`

// Run executes the CLI and returns an exit code.
func (a *App) Run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(a.Stderr, usage)
		return ExitUsage
	}
	switch args[0] {
	case "analyze", "scan":
		return a.analyze(ctx, args[1:], false)
	case "plan":
		return a.analyze(ctx, args[1:], true)
	case "capabilities", "caps":
		return a.capabilities(args[1:])
	case "validate":
		return a.validate(args[1:])
	case "cache":
		return a.cache(args[1:])
	case "version", "--version", "-v":
		fmt.Fprintf(a.Stdout, "capybari %s (commit %s, core API %s, report schema %s)\n", a.Build.Version, orDash(a.Build.Commit), analyzer.APIVersion, report.SchemaVersion)
		return ExitOK
	case "help", "-h", "--help":
		fmt.Fprint(a.Stdout, usage)
		return ExitOK
	}
	fmt.Fprintf(a.Stderr, "unknown command %q\n\n%s", args[0], usage)
	return ExitUsage
}

type analyzeFlags struct {
	only, skip, formats, out, as, question, baseline, failOn string
	offline, active, noCache, quiet, stdout, verbose         bool
	concurrency                                              int
	timeout                                                  time.Duration
}

// parseInterspersed parses flags that may appear before or after positionals.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (a *App) analyze(ctx context.Context, args []string, planOnly bool) int {
	var f analyzeFlags
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	fs.StringVar(&f.only, "only", "", "run only these capabilities (comma-separated) and what they depend on")
	fs.StringVar(&f.skip, "skip", "", "skip these capabilities (comma-separated)")
	fs.StringVar(&f.formats, "format", "json,md,html", "report formats to write: json, md, html, sarif")
	fs.StringVar(&f.out, "out", "capybari-report", "directory for report files")
	fs.BoolVar(&f.stdout, "stdout", false, "write the first --format to stdout instead of files")
	fs.StringVar(&f.as, "as", "", "force the target kind: repository or website")
	fs.StringVar(&f.question, "question", "", "optional: what do you want to know? (recorded in the report)")
	fs.BoolVar(&f.offline, "offline", false, "forbid all network access; capabilities that need it are skipped")
	fs.BoolVar(&f.active, "active", false, "allow active website probes (only for sites you own or are authorised to test)")
	fs.BoolVar(&f.noCache, "no-cache", false, "do not read or write the local result cache")
	fs.StringVar(&f.baseline, "baseline", "", "previous JSON report; findings are marked new or existing")
	fs.StringVar(&f.failOn, "fail-on", "", "exit with code 3 if findings at or above this severity exist (new findings only with --baseline)")
	fs.IntVar(&f.concurrency, "concurrency", 4, "capabilities run in parallel")
	fs.DurationVar(&f.timeout, "timeout", 10*time.Minute, "maximum time per capability")
	fs.BoolVar(&f.quiet, "quiet", false, "no progress or summary output")
	fs.BoolVar(&f.verbose, "verbose", false, "log diagnostic details")
	fs.Usage = func() {
		fmt.Fprintln(a.Stderr, "Usage: capybari analyze <folder | archive | repository URL | website URL> [flags]")
		fs.PrintDefaults()
	}
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitOK
		}
		return ExitUsage
	}
	if len(pos) != 1 {
		fs.Usage()
		return ExitUsage
	}
	formats := splitList(f.formats)
	for _, fm := range formats {
		if !slices.Contains(report.Formats, fm) {
			fmt.Fprintf(a.Stderr, "unknown format %q (want %s)\n", fm, strings.Join(report.Formats, ", "))
			return ExitUsage
		}
	}
	var failOn finding.Severity
	if f.failOn != "" {
		if failOn, err = finding.ParseSeverity(f.failOn); err != nil {
			fmt.Fprintln(a.Stderr, err)
			return ExitUsage
		}
	}
	var base *report.Report
	if f.baseline != "" {
		fh, err := os.Open(f.baseline)
		if err != nil {
			fmt.Fprintln(a.Stderr, "baseline:", err)
			return ExitUsage
		}
		base, err = report.Read(fh)
		fh.Close()
		if err != nil {
			fmt.Fprintln(a.Stderr, "baseline:", err)
			return ExitUsage
		}
	}
	if a.env("CAPYBARI_OFFLINE") == "1" {
		f.offline = true
	}

	res, err := target.Resolve(ctx, pos[0], target.Options{As: analyzer.TargetKind(f.as), AllowClone: !f.offline})
	if err != nil {
		fmt.Fprintln(a.Stderr, "error:", err)
		return ExitError
	}
	defer res.Cleanup()

	logLevel := slog.LevelWarn
	if f.verbose {
		logLevel = slog.LevelDebug
	}
	cfg := engine.Config{
		Registry:          a.registry(),
		Tool:              report.Tool{Name: "capybari", Version: a.Build.Version, Commit: a.Build.Commit},
		Offline:           f.offline,
		Concurrency:       f.concurrency,
		CapabilityTimeout: f.timeout,
		Log:               slog.New(slog.NewTextHandler(a.Stderr, &slog.HandlerOptions{Level: logLevel})),
	}
	if !f.noCache && !planOnly {
		if p, err := cache.DefaultPath(); err == nil {
			if c, err := cache.Open(p, 0); err == nil {
				defer c.Close()
				cfg.Cache = c
				cfg.CacheSalt = executableHash()
			} else if f.verbose {
				fmt.Fprintln(a.Stderr, "cache disabled:", err)
			}
		}
	}
	ui := newUI(a.Stderr, f.quiet || (f.stdout && !isTerminal(a.Stderr)), a.env("NO_COLOR") == "")
	cfg.Progress = ui.progress
	e := engine.New(cfg)
	sel := engine.Selection{Only: splitList(f.only), Skip: splitList(f.skip)}

	if planOnly {
		levels, err := e.Plan(res.Target.Kind, sel)
		if err != nil {
			fmt.Fprintln(a.Stderr, "error:", err)
			return ExitUsage
		}
		fmt.Fprintf(a.Stdout, "Target: %s (%s)\n", res.Target.Display, res.Target.Kind)
		for i, level := range levels {
			fmt.Fprintf(a.Stdout, "Stage %d:\n", i+1)
			for _, id := range level {
				an, _ := e.Registry().Get(id)
				c := an.Capability()
				note := ""
				if c.Execution.Network == analyzer.RequireRequired {
					note = " [network: " + strings.Join(c.Execution.NetworkHosts, ", ") + "]"
					if f.offline {
						note += " (will be skipped: offline)"
					}
				}
				fmt.Fprintf(a.Stdout, "  - %-14s %s%s\n", c.ID, c.Name, note)
			}
		}
		return ExitOK
	}

	opts := analyzer.Options{}
	if f.active {
		opts[analyzer.OptionActive] = "true"
	}
	if f.question != "" {
		opts[analyzer.OptionQuestion] = f.question
	}
	ui.start(res.Target)
	st := e.NewState(res.Target, opts)
	if err := e.Run(ctx, st, sel); err != nil {
		fmt.Fprintln(a.Stderr, "error:", err)
		return ExitError
	}
	r := e.Report(st)
	if base != nil {
		r.ApplyBaseline(base)
	}

	if f.stdout {
		if err := report.Write(a.Stdout, r, formats[0]); err != nil {
			fmt.Fprintln(a.Stderr, "error:", err)
			return ExitError
		}
	} else {
		written, err := writeOutputs(f.out, r, st, formats)
		if err != nil {
			fmt.Fprintln(a.Stderr, "error:", err)
			return ExitError
		}
		ui.summary(r, written)
	}
	if failOn != "" {
		n := 0
		for _, fd := range r.Findings {
			if fd.Severity.Rank() >= failOn.Rank() && (base == nil || fd.BaselineState == finding.BaselineNew) {
				n++
			}
		}
		if n > 0 {
			fmt.Fprintf(a.Stderr, "policy: %d finding(s) at or above %s\n", n, failOn)
			return ExitPolicy
		}
	}
	return ExitOK
}

// writeOutputs writes artifacts and report files into dir and returns the paths.
func writeOutputs(dir string, r *report.Report, st *engine.State, formats []string) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var written []string
	artifactPath := map[string]string{}
	for id, run := range st.Runs {
		for _, art := range run.Artifacts {
			name := filepath.Base(art.Name)
			p := filepath.Join(dir, name)
			if err := os.WriteFile(p, art.Data, 0o644); err != nil {
				return nil, err
			}
			artifactPath[id+"\x00"+art.Name] = p
			written = append(written, p)
		}
	}
	for i := range r.Artifacts {
		if p, ok := artifactPath[r.Artifacts[i].Capability+"\x00"+r.Artifacts[i].Name]; ok {
			r.Artifacts[i].Path = filepath.Base(p)
		}
	}
	for _, fm := range formats {
		p := filepath.Join(dir, "report"+report.Extension(fm))
		fh, err := os.Create(p)
		if err != nil {
			return nil, err
		}
		err = report.Write(fh, r, fm)
		if cerr := fh.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return nil, err
		}
		written = append(written, p)
	}
	slices.Sort(written)
	return written, nil
}

func (a *App) capabilities(args []string) int {
	fs := flag.NewFlagSet("capabilities", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	asJSON := fs.Bool("json", false, "print capability metadata as JSON")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	caps := a.registry().Capabilities()
	if *asJSON {
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(caps); err != nil {
			return ExitError
		}
		return ExitOK
	}
	fmt.Fprintf(a.Stdout, "%-14s %-9s %-19s %-9s %-8s %s\n", "ID", "VERSION", "TARGETS", "NETWORK", "COST", "NAME")
	for _, c := range caps {
		var ts []string
		for _, t := range c.Targets {
			ts = append(ts, string(t))
		}
		name := c.Name
		if c.Experimental() {
			name += " (experimental)"
		}
		fmt.Fprintf(a.Stdout, "%-14s %-9s %-19s %-9s %-8s %s\n", c.ID, c.Version, strings.Join(ts, ","), c.Execution.Network, c.Cost, name)
	}
	return ExitOK
}

func (a *App) validate(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(a.Stderr, "usage: capybari validate <report.json>")
		return ExitUsage
	}
	b, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(a.Stderr, err)
		return ExitError
	}
	if err := schemas.Validate("report.schema.json", b); err != nil {
		fmt.Fprintf(a.Stderr, "%s is not a valid report:\n%v\n", args[0], err)
		return ExitError
	}
	fmt.Fprintf(a.Stdout, "%s: valid (report schema %s)\n", args[0], schemas.Version)
	return ExitOK
}

func (a *App) cache(args []string) int {
	if len(args) != 1 || args[0] != "clear" {
		fmt.Fprintln(a.Stderr, "usage: capybari cache clear")
		return ExitUsage
	}
	p, err := cache.DefaultPath()
	if err != nil {
		fmt.Fprintln(a.Stderr, err)
		return ExitError
	}
	c, err := cache.Open(p, 0)
	if err != nil {
		fmt.Fprintln(a.Stderr, err)
		return ExitError
	}
	defer c.Close()
	if err := c.Clear(); err != nil {
		fmt.Fprintln(a.Stderr, err)
		return ExitError
	}
	fmt.Fprintln(a.Stdout, "cache cleared:", p)
	return ExitOK
}

// executableHash identifies the running binary so cached results from a
// different build are never reused.
func executableHash() string {
	p, err := os.Executable()
	if err != nil {
		return ""
	}
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// ui renders progress and the terminal summary on stderr.
type ui struct {
	w     io.Writer
	quiet bool
	color bool
	mu    sync.Mutex
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func newUI(w io.Writer, quiet, colorAllowed bool) *ui {
	return &ui{w: w, quiet: quiet, color: colorAllowed && isTerminal(w)}
}

func (u *ui) paint(code, s string) string {
	if !u.color {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func (u *ui) start(t analyzer.Target) {
	if u.quiet {
		return
	}
	fmt.Fprintf(u.w, "%s %s (%s)\n", u.paint("1", "Capybari X-Ray:"), t.Display, t.Kind)
}

func (u *ui) progress(ev engine.Event) {
	if u.quiet || ev.Type != "finish" {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	mark, note := u.paint("32", "✓"), fmt.Sprintf("%d finding(s), %s", ev.Findings, ev.Duration.Round(time.Millisecond))
	switch ev.Status {
	case report.StatusSkipped, report.StatusNotApplicable:
		mark, note = u.paint("90", "–"), ev.Reason
	case report.StatusFailed:
		mark, note = u.paint("31", "✗"), ev.Reason
	}
	if ev.Cached {
		note += ", cached"
	}
	fmt.Fprintf(u.w, "  %s %-32s %s\n", mark, ev.Name, u.paint("90", note))
}

func (u *ui) summary(r *report.Report, written []string) {
	if u.quiet {
		return
	}
	fmt.Fprintf(u.w, "\n%s\n", u.paint("1", r.Summary.Headline))
	if len(r.Scores) > 0 {
		fmt.Fprintln(u.w)
		for _, s := range r.Scores {
			code := map[string]string{"good": "32", "fair": "33", "poor": "31"}[s.Rating]
			fmt.Fprintf(u.w, "  %-20s %s  %s\n", s.Name, u.paint(code, fmt.Sprintf("%3d/100", s.Value)), u.paint("90", string(s.Confidence)+" confidence"))
		}
	}
	shown := 0
	for _, f := range r.Findings {
		if f.Severity.Rank() < finding.Medium.Rank() || shown == 5 {
			break
		}
		if shown == 0 {
			fmt.Fprintf(u.w, "\n  Top findings:\n")
		}
		loc := ""
		if len(f.Evidence) > 0 {
			if f.Evidence[0].Path != "" {
				loc = f.Evidence[0].Path
				if f.Evidence[0].StartLine > 0 {
					loc += fmt.Sprintf(":%d", f.Evidence[0].StartLine)
				}
			} else {
				loc = f.Evidence[0].URL
			}
		}
		code := map[finding.Severity]string{finding.Critical: "31;1", finding.High: "31", finding.Medium: "33"}[f.Severity]
		fmt.Fprintf(u.w, "  %s %s %s\n", u.paint(code, fmt.Sprintf("%-8s", f.Severity)), f.Title, u.paint("90", loc))
		shown++
	}
	if len(r.Recommendations) > 0 {
		fmt.Fprintf(u.w, "\n  What else can we tell you?\n")
		for i, rec := range r.Recommendations {
			if i == 3 {
				break
			}
			fmt.Fprintf(u.w, "  → %s: %s\n", rec.Title, u.paint("90", rec.Reason))
		}
	}
	fmt.Fprintf(u.w, "\n  %s\n", u.paint("90", r.DataBoundary.Statement))
	if len(written) > 0 {
		fmt.Fprintf(u.w, "\n  Reports: %s\n", strings.Join(written, ", "))
	}
}
