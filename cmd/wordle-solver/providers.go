package main

import (
	"encoding/json"

	"fmt"
	modeleval "github.com/notzenco/wordle-solver/internal/eval"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

type provider struct {
	BaseURL string `json:"base_url"`
	KeyEnv  string `json:"api_key_env,omitempty"`
	Kind    string `json:"kind,omitempty"`
}

var builtins = map[string]provider{
	"ollama":     {"http://localhost:11434/v1", "", ""},
	"lmstudio":   {"http://localhost:1234/v1", "", ""},
	"llamacpp":   {"http://localhost:8080/v1", "", ""},
	"omlx":       {"http://127.0.0.1:8000/v1", "", ""},
	"omniroute":  {"http://127.0.0.1:20128/v1", "OMNIROUTE_API_KEY", ""},
	"openai":     {"https://api.openai.com/v1", "OPENAI_API_KEY", ""},
	"openrouter": {"https://openrouter.ai/api/v1", "OPENROUTER_API_KEY", ""},
}
var providerName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
var envName = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

func validKeyEnv(s string) error {
	if s != "" && (!envName.MatchString(s) || strings.HasPrefix(s, "SK_") || len(s) > 128) {
		return fmt.Errorf("API key must be an environment variable name (for example OPENAI_API_KEY)")
	}
	return nil
}
func validBase(s string) error {
	u, e := url.Parse(s)
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("base URL must use http or https without credentials, query or fragment")
	}
	return nil
}
func allProviders(c configFile) map[string]provider {
	m := map[string]provider{}
	for k, v := range builtins {
		m[k] = v
	}
	for k, v := range c.Providers {
		m[k] = v
	}
	return m
}
func keyStatus(p provider) string {
	if p.KeyEnv != "" && os.Getenv(p.KeyEnv) != "" {
		return "set"
	}
	return "unset"
}

// redact hides the provider's API key in text a server sent back, which a
// misbehaving server could echo.
func redact(p provider, s string) string {
	if p.KeyEnv == "" {
		return s
	}
	if key := os.Getenv(p.KeyEnv); key != "" {
		return strings.ReplaceAll(s, key, "[redacted]")
	}
	return s
}

// keyColumn describes a provider's key without revealing it.
func keyColumn(p provider) string {
	if p.KeyEnv == "" {
		return "none"
	}
	return fmt.Sprintf("%s (%s)", p.KeyEnv, keyStatus(p))
}
func providerNames(c configFile) []string {
	n := []string{}
	for k := range allProviders(c) {
		n = append(n, k)
	}
	sort.Strings(n)
	return n
}
func providersCommand(c *cmdCtx) error {
	sub, args := "list", c.args
	if len(args) > 0 {
		sub, args = args[0], args[1:]
	}
	if sub != "add" {
		for _, id := range []string{"provider.base-url", "provider.api-key-env", "provider.kind"} {
			if c.set(id) {
				return usageError(fmt.Sprintf("--%s only applies to 'providers add'", optionByID(id).flag))
			}
		}
	}
	switch {
	case sub == "list" && len(args) == 0:
		tw := tabwriter.NewWriter(stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(tw, "NAME\tBASE URL\tKEY\tKIND\tSOURCE")
		for _, n := range providerNames(c.config) {
			p := allProviders(c.config)[n]
			src := "built-in"
			if _, ok := c.config.Providers[n]; ok {
				src = "saved"
				if _, ok := builtins[n]; ok {
					src = "saved (overrides built-in)"
				}
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", n, p.BaseURL, keyColumn(p), kindColumn(p), src)
		}
		return tw.Flush()
	case sub == "remove" && len(args) == 1:
		if _, ok := c.config.Providers[args[0]]; !ok {
			return fmt.Errorf("no saved provider %q", args[0])
		}
		delete(c.config.Providers, args[0])
		if err := saveConfig(c.config); err != nil {
			return err
		}
		if _, ok := builtins[args[0]]; ok {
			fmt.Fprintf(stdout, "Removed the saved %s; the built-in address applies again.\n", args[0])
		} else {
			fmt.Fprintf(stdout, "Removed %s.\n", args[0])
		}
		return nil
	case sub == "check" && len(args) == 1:
		p, err := findProvider(c.config, args[0])
		if err != nil {
			return err
		}
		key := "no key needed"
		if p.KeyEnv != "" {
			key = fmt.Sprintf("key %s", keyColumn(p))
		}
		m, err := fetchModels(p)
		if err != nil {
			fmt.Fprintf(stdout, "%s: not reachable at %s; %s\n", args[0], p.BaseURL, key)
			return err
		}
		fmt.Fprintf(stdout, "%s: reachable at %s; %d models; %s\n", args[0], p.BaseURL, len(m), key)
		return nil
	case sub == "add" && len(args) == 1:
		n := args[0]
		if !providerName.MatchString(n) {
			return usageError("provider names use lowercase letters, digits and single hyphens")
		}
		base, key, kind := c.get("provider.base-url"), c.get("provider.api-key-env"), c.get("provider.kind")
		if base == "" {
			return usageError("providers add needs --base-url URL")
		}
		if err := validBase(base); err != nil {
			return err
		}
		if err := validKeyEnv(key); err != nil {
			return err
		}
		if err := validKind(kind); err != nil {
			return err
		}
		if c.config.Providers == nil {
			c.config.Providers = map[string]provider{}
		}
		c.config.Providers[n] = provider{strings.TrimRight(base, "/"), key, kind}
		if err := saveConfig(c.config); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Saved %s at %s.\n", n, strings.TrimRight(base, "/"))
		return nil
	}
	return usageError("usage: wordle-solver providers [list | add NAME --base-url URL [--api-key-env NAME] [--kind KIND] | remove NAME | check NAME]")
}
func findProvider(c configFile, n string) (provider, error) {
	p, ok := allProviders(c)[n]
	if !ok {
		return p, fmt.Errorf("unknown provider %q; available: %s; use providers add", n, strings.Join(providerNames(c), ", "))
	}
	return p, nil
}
func apiRequest(client *http.Client, p provider, path string, body io.Reader) ([]byte, error) {
	method := "GET"
	if body != nil {
		method = "POST"
	}
	req, e := http.NewRequest(method, strings.TrimRight(p.BaseURL, "/")+path, body)
	if e != nil {
		return nil, fmt.Errorf("invalid API request")
	}
	if p.KeyEnv != "" {
		if key := os.Getenv(p.KeyEnv); key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	req.Header.Set("Content-Type", "application/json")
	// Do not forward credentials through redirects, including same-host redirects.
	safe := *client
	safe.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, e := safe.Do(req)
	if e != nil {
		return nil, fmt.Errorf("API request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 400 && path == "/chat/completions" {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		if strings.Contains(string(b), "not an LLM / chat model") {
			return nil, modeleval.ErrDecisionModel
		}
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned HTTP %d", resp.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if e != nil {
		return nil, fmt.Errorf("reading API response failed")
	}
	return b, nil
}
func fetchModels(p provider) ([]string, error) {
	b, e := apiRequest(&http.Client{Timeout: 10 * time.Second}, p, "/models", nil)
	if e != nil {
		return nil, e
	}
	var r struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if e = json.Unmarshal(b, &r); e != nil {
		return nil, fmt.Errorf("invalid models response")
	}
	m := []string{}
	for _, x := range r.Data {
		id := x.ID
		if key := os.Getenv(p.KeyEnv); key != "" {
			id = strings.ReplaceAll(id, key, "[redacted]")
		}
		m = append(m, id)
	}
	sort.Strings(m)
	return m, nil
}

func validKind(k string) error {
	if k != "" && k != "chat" && k != "systemone" {
		return fmt.Errorf("kind must be chat or systemone")
	}
	return nil
}

// kindColumn shows "auto" for providers whose kind is detected per model.
func kindColumn(p provider) string {
	if p.Kind == "" {
		return "auto"
	}
	return p.Kind
}
func providerKind(p provider) string {
	if p.Kind == "" {
		return "chat"
	}
	return p.Kind
}
