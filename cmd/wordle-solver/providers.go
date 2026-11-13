package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

type provider struct {
	BaseURL string `json:"base_url"`
	KeyEnv  string `json:"api_key_env,omitempty"`
}

var builtins = map[string]provider{
	"ollama":     {"http://localhost:11434/v1", ""},
	"lmstudio":   {"http://localhost:1234/v1", ""},
	"llamacpp":   {"http://localhost:8080/v1", ""},
	"omlx":       {"http://127.0.0.1:8000/v1", ""},
	"omniroute":  {"http://127.0.0.1:20128/v1", "OMNIROUTE_API_KEY"},
	"openai":     {"https://api.openai.com/v1", "OPENAI_API_KEY"},
	"openrouter": {"https://openrouter.ai/api/v1", "OPENROUTER_API_KEY"},
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
func providerNames(c configFile) []string {
	n := []string{}
	for k := range allProviders(c) {
		n = append(n, k)
	}
	sort.Strings(n)
	return n
}
func providersCommand(args []string, c *configFile) error {
	if len(args) == 0 {
		for _, n := range providerNames(*c) {
			p := allProviders(*c)[n]
			src := "builtin"
			if _, ok := c.Providers[n]; ok {
				src = "config"
			}
			fmt.Printf("%s\t%s\t%s (%s)\t%s\n", n, p.BaseURL, p.KeyEnv, keyStatus(p), src)
		}
		return nil
	}
	if len(args) == 2 && args[0] == "remove" {
		if _, ok := c.Providers[args[1]]; !ok {
			return fmt.Errorf("no saved provider %q", args[1])
		}
		delete(c.Providers, args[1])
		return saveConfig(*c)
	}
	if len(args) == 2 && args[0] == "check" {
		p, e := findProvider(*c, args[1])
		if e != nil {
			return e
		}
		m, e := fetchModels(p)
		if e != nil {
			fmt.Printf("reachable: no; key: %s\n", keyStatus(p))
			return e
		}
		fmt.Printf("reachable: yes; key: %s; models: %d\n", keyStatus(p), len(m))
		return nil
	}
	if len(args) >= 2 && args[0] == "add" {
		n := args[1]
		if !providerName.MatchString(n) {
			return fmt.Errorf("invalid provider name")
		}
		fs := flag.NewFlagSet("providers add", flag.ContinueOnError)
		base := fs.String("base-url", "", "")
		key := fs.String("api-key-env", "", "")
		if e := fs.Parse(args[2:]); e != nil {
			return e
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("unexpected provider arguments")
		}
		if e := validBase(*base); e != nil {
			return e
		}
		if e := validKeyEnv(*key); e != nil {
			return e
		}
		if c.Providers == nil {
			c.Providers = map[string]provider{}
		}
		c.Providers[n] = provider{strings.TrimRight(*base, "/"), *key}
		return saveConfig(*c)
	}
	return fmt.Errorf("usage: providers [add NAME --base-url URL [--api-key-env NAME] | remove NAME | check NAME]")
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
		m = append(m, x.ID)
	}
	sort.Strings(m)
	return m, nil
}
