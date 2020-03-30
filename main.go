package main

import (
	"context"
	"crypto/tls"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/acme/autocert"
)

// equal reports whether the first argument is equal to any of the remaining
// arguments. This function is used as a custom function within templates to do
// richer equality tests.
func equal(x, y interface{}) bool {
	return reflect.DeepEqual(x, y)
}

var (
	// templateGlobPattern is the pattern than matches all the HTML
	// templates in the static directory
	templateGlobPattern = filepath.Join(staticDirName, "*.html")

	// customFuncs is a registry of custom functions we use from within the
	// templates.
	customFuncs = template.FuncMap{
		"equal": equal,
	}

	// ctxb is a global context with no timeouts that's used within the
	// gRPC requests to lnd.
	ctxb = context.Background()
)

const (
	staticDirName = "static"
)

func main() {
	// Load configuration and parse command line.  This function also
	// initializes logging and configures it accordingly.
	cfg, _, err := loadConfig()
	if err != nil {
		return
	}

	// Pre-compile the list of templates so we'll catch any errors in the
	// templates as soon as the binary is run.
	faucetTemplates := template.Must(template.New("faucet").
		Funcs(customFuncs).
		ParseGlob(templateGlobPattern))

	// With the templates loaded, create the faucet itself.
	faucet, err := newLightningFaucet(cfg, faucetTemplates)
	if err != nil {
		log.Criticalf("unable to create faucet: %v", err)
		os.Exit(1)
		return
	}

	// If the wipe channels bool is set, then we'll attempt to close ALL
	// the faucet's channels by any means and exit in the case of a success
	// or failure.
	if cfg.WipeChannels {
		log.Info("Attempting to wipe all faucet channels")
		if err := faucet.CloseAllChannels(); err != nil {
			log.Criticalf("unable to close all the faucet's channels: %v", err)
			os.Exit(1)
			return
		}

		return
	}

	// If we're not wiping all the channels, then we'll launch the set of
	// goroutines required for the faucet to function.
	faucet.Start(cfg)

	// Create a new mux in order to route a request based on its path to a
	// dedicated http.Handler.
	r := mux.NewRouter()
	r.HandleFunc("/", faucet.faucetHome).Methods("POST", "GET")
	r.HandleFunc("/info", faucet.infoPage).Methods("GET")

	// Next create a static file server which will dispatch our static
	// files. We rap the file sever http.Handler is a handler that strips
	// out the absolute file path since it'll dispatch based on solely the
	// file name.
	staticFileServer := http.FileServer(http.Dir(staticDirName))
	staticHandler := http.StripPrefix("/static/", staticFileServer)
	r.PathPrefix("/static/").Handler(staticHandler)

	// With all of our paths registered we'll register our mux as part of
	// the global http handler.
	http.Handle("/", r)

	if !cfg.UseLeHTTPS {
		log.Infof("Listening on %s", cfg.BindAddr)
		go http.ListenAndServe(cfg.BindAddr, r)
	} else {
		// Create a directory cache so the certs we get from Let's
		// Encrypt are cached locally. This avoids running into their
		// rate-limiting by requesting too many certs.
		certCache := autocert.DirCache("certs")

		// Create the auto-cert manager which will automatically obtain a
		// certificate provided by Let's Encrypt.
		m := autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Cache:      certCache,
			HostPolicy: autocert.HostWhitelist(cfg.Domain),
		}

		// As we'd like all requests to default to https, redirect all regular
		// http requests to the https version of the faucet.
		log.Infof("Listening on %s", cfg.BindAddr)
		go http.ListenAndServe(cfg.BindAddr, m.HTTPHandler(nil))

		// Finally, create the http server, passing in our TLS configuration.
		httpServer := &http.Server{
			Handler:      r,
			WriteTimeout: 30 * time.Second,
			ReadTimeout:  30 * time.Second,
			Addr:         ":https",
			TLSConfig: &tls.Config{
				GetCertificate: m.GetCertificate,
				MinVersion:     tls.VersionTLS12,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				},
			},
		}
		if err := httpServer.ListenAndServeTLS("", ""); err != nil {
			log.Critical(err)
			os.Exit(1)
		}
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

func init() {
	// Support TLS 1.3.
	os.Setenv("GODEBUG", os.Getenv("GODEBUG")+",tls13=1")

}
