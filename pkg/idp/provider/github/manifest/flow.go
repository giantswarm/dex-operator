package manifest

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"giantswarm/dex-operator/pkg/idp/provider"

	"net"
	"net/http"
	"net/url"

	githubclient "github.com/google/go-github/v50/github"
	"github.com/pkg/browser"
)

type Config struct {
	AppConfig    provider.AppConfig
	Port         int
	Host         string
	Organization string
}

type Flow struct {
	manifest Manifest
	url      url.URL
	state    string
	result   *githubclient.AppConfig
	renderer *Renderer
	port     int
}

func newFlow(c Config) (*Flow, error) {
	state, err := newState()
	if err != nil {
		return nil, err
	}

	return &Flow{
		state:    state,
		url:      getURL(c.Host, c.Organization, state),
		manifest: NewManifest(c.AppConfig),
		renderer: newRenderer(),
		port:     c.Port,
	}, nil
}

func CreateGithubApp(c Config) (*githubclient.AppConfig, error) {
	f, err := newFlow(c)
	if err != nil {
		return nil, err
	}
	err = f.run()
	if err != nil {
		return nil, err
	}
	return f.result, err
}

func (f *Flow) run() error {
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", f.port))
	if err != nil {
		return err
	}
	serverURL := "http://" + listener.Addr().String()
	ctx, cancel := context.WithCancel(context.Background())

	// Create a form with the app manifest that is to be submitted to github by the user
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// After the manifest is created in github we need to redirect back to complete the flow
		f.manifest.RedirectURL = fmt.Sprintf("http://%s/retrieve", r.Host)

		jsonManifest, err := json.MarshalIndent(f.manifest, "", "  ")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		template := Template{
			URL:     f.url.String(),
			Content: string(jsonManifest),
		}
		err = f.renderer.Render("submit.tmpl", w, template)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	// Retrieves the app code after redirecting back to the local server
	http.HandleFunc("/retrieve", func(w http.ResponseWriter, r *http.Request) {
		if f.state != r.URL.Query().Get("state") {
			http.Error(w, "state does not match", http.StatusInternalServerError)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "code was not found", http.StatusInternalServerError)
			return
		}
		client := githubclient.NewClient(nil)
		app, resp, err := client.Apps.CompleteAppManifest(ctx, code)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to complete github app manifest: %v", err.Error()), http.StatusInternalServerError)
		}
		if resp.StatusCode != http.StatusCreated {
			http.Error(w, fmt.Sprintf("failed to complete github app manifest. Got response %v", resp), http.StatusInternalServerError)
		}
		f.result = app
		http.Redirect(w, r, "/complete", http.StatusFound)
	})
	// The flow is completed, show the user some sort of success and cancel the context
	http.HandleFunc("/complete", func(w http.ResponseWriter, r *http.Request) {
		template := Template{
			Content: f.manifest.Name,
		}
		if err := f.renderer.Render("complete.tmpl", w, template); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		cancel()
	})

	err = browser.OpenURL(serverURL)
	if err != nil {
		return err
	}

	go func() {
		err = http.Serve(listener, nil)
	}()

	<-ctx.Done()
	return err
}

func newState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func getURL(host string, org string, state string) url.URL {
	return url.URL{
		Scheme:   "https",
		Host:     host,
		Path:     fmt.Sprintf("/organizations/%s/settings/apps/new", org),
		RawQuery: fmt.Sprintf("state=%s", state),
	}
}
