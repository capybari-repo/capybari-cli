# capybari-cli

**Capybari Source Intelligence: a free technical X-ray of your software.**

One binary. Point it at a folder, an archive, a repository URL or a website, and it decides which analyses apply, runs them, and gives you one report:

```text
$ capybari analyze ./legacy-shop-api
Capybari X-Ray: legacy-shop-api (repository)
  ✓ Repository Inventory             0 finding(s), 29ms
  ✓ Dependency Inventory & SBOM      0 finding(s), 8ms
  ✓ Code Health                      0 finding(s), 11ms
  ✓ Project Fingerprint              2 finding(s), 16ms
  ✓ Architecture Mapper              1 finding(s), 23ms
  ✓ Secret Scanner                   0 finding(s), 286ms
  ✓ Technology & Version Detector    2 finding(s), 15ms
  ✓ Known Vulnerability Scanner      6 finding(s), 791ms

legacy-shop-api: 6 critical/high finding(s) in Security, Technology Currency. Start there.

  Dependency Hygiene   100/100  high confidence
  Technology Currency   75/100  high confidence
  Maintainability       91/100  high confidence
  Operability           98/100  high confidence
  Security              38/100  high confidence
  Structure             98/100  high confidence

  Top findings:
  high     Node.js 12 is end-of-life .nvmrc
  high     body-parser 1.18.2 has 2 known vulnerabilities package-lock.json
  high     jsonwebtoken 8.5.1 has 3 known vulnerabilities package-lock.json
  high     lodash 4.17.15 has 6 known vulnerabilities package-lock.json
  high     moment 2.29.1 has 2 known vulnerabilities package-lock.json

  What else can we tell you?
  → Add the deployed URL: The repository shows the code. The deployed URL adds external exposure, TLS and security-header checks of what users actually reach.

  Network requests were made to api.osv.dev by vulns. Source files were not uploaded; each disclosure states exactly what was sent.

  Reports: capybari-report/report.html, capybari-report/report.json, capybari-report/report.md, capybari-report/sbom.cdx.json
```

_Real output for the `node-express-legacy` fixture in [capybari-fixtures](https://github.com/capybari-repo/capybari-fixtures)._

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/capybari-repo/capybari-cli/main/scripts/install.sh | sh
brew install capybari-repo/tap/capybari
go install github.com/capybari-repo/capybari-cli/cmd/capybari@latest
docker run --rm -v "$PWD:/src" ghcr.io/capybari-repo/capybari analyze /src
```

Or download a binary for Linux, macOS or Windows (amd64/arm64) from the releases page. Builds are reproducible, static (no cgo) and about 30 MB.

## Use

```bash
capybari analyze .                              # a folder
capybari analyze project.zip                    # an archive (.zip, .tar, .tar.gz)
capybari analyze https://github.com/org/repo    # a remote repository (needs git)
capybari analyze https://example.com            # a website: passive checks
capybari analyze . --offline                    # guarantee nothing leaves the machine
capybari analyze https://app.example.com        # JavaScript-built sites are rendered if headless Chromium is installed
capybari analyze https://app.example.com --no-render
capybari analyze . --only vulns                 # one capability (plus what it needs)
capybari analyze . --format sarif --stdout      # SARIF for code scanning
capybari analyze . --baseline old/report.json --fail-on high   # CI gate on new findings
capybari plan .                                 # show what would run
capybari capabilities                           # list capabilities
capybari validate capybari-report/report.json   # check a report against the schema
```

Exit codes: `0` ok, `1` error, `2` usage, `3` `--fail-on` threshold reached.

## What runs

| Capability | Repository | Network |
|---|---|---|
| Repository Inventory | ✓ | none |
| Project Fingerprint | ✓ | none |
| Technology & Version Detector (with offline end-of-life data) | ✓ | none |
| Dependency Inventory & SBOM (CycloneDX) | ✓ | none |
| Known Vulnerability Scanner (+ hallucinated-package check) | ✓ | `api.osv.dev`: package names and versions; `api.deps.dev`: direct dependency names |
| Secret Scanner (Gitleaks engine) | ✓ | none |
| Code Health | ✓ | none |
| Architecture Mapper | ✓ | none |
| Website Snapshot | website | the site itself |

The **AI Slop Score** combines AI-generation signs, unfinished code, security shortcuts, dependency hygiene and organization into one meter: **0 = clean, 100 = pure slop** (higher is worse). Every other score runs 100 = best.

Every report ends with a **data boundary** section that lists every host contacted, which capability contacted it, and what was sent.

## Build from source

```bash
make build     # ./capybari
make test
make dist      # reproducible binaries + checksums for 6 platforms
```

Pre-release: `go.mod` uses `replace` directives pointing at sibling checkouts of `capybari-core`, `capybari-schemas` and the `capybari-analyzer-*` repositories. `scripts/release-deps.sh` swaps them for tagged versions.

## License

Apache-2.0. See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for bundled components.
