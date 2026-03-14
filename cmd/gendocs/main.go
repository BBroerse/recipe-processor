// Package main generates the configuration reference documentation.
// Run with: go run ./cmd/gendocs
//
//go:generate go run ./cmd/gendocs
package main

import (
	"fmt"
	"os"
	"strings"
	"text/template"
)

// configVar describes a single environment variable used by the application.
type configVar struct {
	Name        string
	Type        string
	Default     string
	Required    bool
	Description string
}

// configVars is the canonical list of all environment variables.
// Update this list whenever you add or change configuration in
// internal/shared/config/config.go.
var configVars = []configVar{
	{
		Name:        "ENV",
		Type:        "string",
		Default:     "development",
		Required:    false,
		Description: "Deployment environment (`development` or `production`). Controls features like Swagger UI availability.",
	},
	{
		Name:        "PORT",
		Type:        "int",
		Default:     "8080",
		Required:    false,
		Description: "HTTP server listen port.",
	},
	{
		Name:        "DB_HOST",
		Type:        "string",
		Default:     "localhost",
		Required:    false,
		Description: "PostgreSQL server hostname.",
	},
	{
		Name:        "DB_PORT",
		Type:        "int",
		Default:     "5432",
		Required:    false,
		Description: "PostgreSQL server port.",
	},
	{
		Name:        "DB_USER",
		Type:        "string",
		Default:     "postgres",
		Required:    false,
		Description: "PostgreSQL username.",
	},
	{
		Name:        "DB_PASSWORD",
		Type:        "string",
		Default:     "",
		Required:    true,
		Description: "PostgreSQL password. **Must be set** — the application will not start without it.",
	},
	{
		Name:        "DB_NAME",
		Type:        "string",
		Default:     "recipes",
		Required:    false,
		Description: "PostgreSQL database name.",
	},
	{
		Name:        "DB_SSLMODE",
		Type:        "string",
		Default:     "require",
		Required:    false,
		Description: "PostgreSQL SSL mode (e.g. `disable`, `require`, `verify-full`).",
	},
	{
		Name:        "OLLAMA_URL",
		Type:        "string",
		Default:     "http://localhost:11434",
		Required:    false,
		Description: "Base URL of the Ollama LLM service.",
	},
	{
		Name:        "OLLAMA_MODEL",
		Type:        "string",
		Default:     "tinyllama",
		Required:    false,
		Description: "Ollama model name used for recipe text processing.",
	},
	{
		Name:        "LOG_LEVEL",
		Type:        "string",
		Default:     "info",
		Required:    false,
		Description: "Logging verbosity level (`debug`, `info`, `warn`, `error`).",
	},
}

const markdownTmpl = `# Configuration Reference

> **Auto-generated** — do not edit manually. Update the source in ` + "`cmd/gendocs/main.go`" + ` and run ` + "`go generate ./cmd/gendocs`" + `.

All configuration is supplied via environment variables.

## Environment Variables

| Variable | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
{{- range .}}
| ` + "`{{.Name}}`" + ` | {{.Type}} | {{formatDefault .Default}} | {{if .Required}}**Yes**{{else}}No{{end}} | {{.Description}} |
{{- end}}

## Quick Start

` + "```" + `bash
# Minimal required configuration
export DB_PASSWORD="your-secure-password"

# Start the server (all other values use defaults)
go run ./cmd/api
` + "```" + `

## Example: Full Configuration

` + "```" + `bash
export ENV=production
export PORT=8080
export DB_HOST=db.example.com
export DB_PORT=5432
export DB_USER=recipe_app
export DB_PASSWORD=super-secret
export DB_NAME=recipes
export DB_SSLMODE=require
export OLLAMA_URL=http://ollama:11434
export OLLAMA_MODEL=tinyllama
export LOG_LEVEL=info
` + "```" + `
`

func main() {
	funcMap := template.FuncMap{
		"formatDefault": func(d string) string {
			if d == "" {
				return "—"
			}
			// Escape pipes in defaults for markdown table safety
			d = strings.ReplaceAll(d, "|", "\\|")
			return "`" + d + "`"
		},
	}

	tmpl, err := template.New("config").Funcs(funcMap).Parse(markdownTmpl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing template: %v\n", err)
		os.Exit(1)
	}

	f, err := os.Create("docs/configuration.md")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating file: %v\n", err)
		os.Exit(1)
	}

	if err := tmpl.Execute(f, configVars); err != nil {
		_ = f.Close()
		fmt.Fprintf(os.Stderr, "error executing template: %v\n", err)
		os.Exit(1)
	}

	if err := f.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "error closing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("docs/configuration.md generated successfully")
}
