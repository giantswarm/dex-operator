package manifest

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/giantswarm/dex-operator/pkg/idp/provider"

	"net/http"
	"net/url"

	githubclient "github.com/google/go-github/v84/github"
	"github.com/pkg/browser"
)

type Config struct {
	AppConfig         provider.AppConfig
	Port              int
	Host              string
	ReadHeaderTimeout time.Duration
	Organization      string
}

type Flow struct {
	manifest          Manifest
	createURL         url.URL
	installLink       string
	state             string
	readHeaderTimeout time.Duration
	result            *githubclient.AppConfig
	renderer          *Renderer
	port              int
}

func newFlow(c Config) (*Flow, error) {
	state, err := newState()
	if err != nil {
		return nil, err
	}

	if c.Port == 0 {
		c.Port, err = findAvailablePort()
		if err != nil {
			return nil, err
		}
	}

	return &Flow{
		state:             state,
		createURL:         getCreateURL(c.Host, c.Organization, state),
		installLink:       getInstallLink(c.Host, c.Organization, c.AppConfig.Name),
		manifest:          NewManifest(c.AppConfig),
		renderer:          newRenderer(),
		port:              c.Port,
		readHeaderTimeout: c.ReadHeaderTimeout,
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
	ctx, cancel := context.WithCancel(context.Background())

	var server *http.Server
	{
		mux := http.NewServeMux()
		// Create a form with the app manifest that is to be submitted to github by the user
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// After the manifest is created in github we need to redirect back to complete the flow
			f.manifest.RedirectURL = fmt.Sprintf("http://%s/retrieve", r.Host)

			jsonManifest, err := json.MarshalIndent(f.manifest, "", "  ")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			template := Template{
				URL:     f.createURL.String(),
				Content: string(jsonManifest),
			}
			err = f.renderer.Render("submit.tmpl", w, template)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		})
		// Retrieves the app code after redirecting back to the local server
		mux.HandleFunc("/retrieve", func(w http.ResponseWriter, r *http.Request) {
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
		// The flow is completed, redirect to install
		mux.HandleFunc("/complete", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, f.installLink, http.StatusFound)
			cancel()
		})

		mux.Handle("/static/", http.FileServer(http.FS(f.renderer.fs)))

		server = &http.Server{
			Addr:              fmt.Sprintf("localhost:%d", f.port),
			Handler:           mux,
			ReadHeaderTimeout: f.readHeaderTimeout,
		}
	}
	err := browser.OpenURL(fmt.Sprintf("http://%s", server.Addr))
	if err != nil {
		return err
	}

	go func() {
		err = server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			// All good.
		} else if err != nil {
			cancel()
		}
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

func getCreateURL(host string, org string, state string) url.URL {
	return url.URL{
		Scheme:   "https",
		Host:     host,
		Path:     fmt.Sprintf("/organizations/%s/settings/apps/new", org),
		RawQuery: fmt.Sprintf("state=%s", state),
	}
}

func getInstallLink(host string, organization string, slug string) string {
	return fmt.Sprintf("https://%s/organizations/%s/settings/apps/%s/installations", host, organization, slug)
}

func findAvailablePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return -1, err
	}

	ln, err := net.Listen("tcp", addr.String())
	if err != nil {
		return -1, err
	}

	defer func() {
		if closeErr := ln.Close(); closeErr != nil {
			fmt.Println("Error closing listener:", closeErr)
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port

	return port, nil
}
