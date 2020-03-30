package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/decred/dcrd/dcrutil/v2"
	"github.com/jessevdk/go-flags"
)

const (
	defaultTLSCertFilename  = "tls.cert"
	defaultMacaroonFilename = "admin.macaroon"
	defaultLogFilename      = "dcrlnfaucet.log"
	defaultConfigFilename   = "dcrlnfaucet.conf"
	defaultLogLevel         = "info"
	defaultLndNode          = "localhost:10009"
	defaultBindAddr         = ":80"
	defaultUseLeHTTPS       = false
	defaultWipeChannels     = false
	defaultUseRealIP        = false
	defaultActionsTimeLimit = time.Duration(30) * time.Second
)

var (
	defaultLndDir       = dcrutil.AppDataDir("dcrlnd", false)
	defaultTLSCertPath  = filepath.Join(defaultLndDir, defaultTLSCertFilename)
	defaultMacaroonPath = filepath.Join(
		defaultLndDir, "data", "chain", "decred", "testnet",
		defaultMacaroonFilename,
	)
	defaultDataDir = dcrutil.AppDataDir("dcrlnfaucet", false)
	defaultLogPath = filepath.Join(
		defaultDataDir, "logs", "decred", "testnet",
		defaultLogFilename,
	)
	defaultConfigFile = filepath.Join(
		defaultDataDir, defaultConfigFilename,
	)
)

type config struct {
	ShowVersion bool `short:"V" long:"version" description:"Display version information and exit"`

	ConfigFile       string        `short:"C" long:"configfile" description:"Path to configuration file"`
	LndNode          string        `long:"lnd_node" description:"network address of dcrlnd RPC (host:port)"`
	BindAddr         string        `long:"bind_addr" description:"port to listen for http"`
	UseLeHTTPS       bool          `long:"use_le_https" description:"use https via lets encrypt"`
	WipeChannels     bool          `long:"wipe_chans" description:"close all faucet channels and exit"`
	Domain           string        `long:"domain" description:"the domain of the faucet, required for TLS"`
	MacaroonPath     string        `long:"macpath" description:"path to macaroons files"`
	TLSCertPath      string        `long:"tlscertpath" description:"Path to write the TLS certificate for lnd's RPC and REST services"`
	ActionsTimeLimit time.Duration `long:"actions_timelimit" description:"Time to wait before a second request can be made by a single faucet client."`
	UseRealIP        bool          `long:"userealip" description:"Use the RealIP middleware to get the client's real IP from the X-Real-IP or X-Forwarded-For headers, in that order."`

	DisableZombieSweeper bool `long:"disable_zombie_sweeper" description:"disable zombie channels sweeper"`

	// Network
	MainNet bool `long:"mainnet" description:"Use the main network"`
	TestNet bool `long:"testnet" description:"Use the test network"`
	SimNet  bool `long:"simnet" description:"Use the simulation test network"`

	// Invoice features
	DisableGenerateInvoices bool `long:"disablegen" description:"disable generate invoice"`
	DisablePayInvoices      bool `long:"disablepay" description:"disable invoice payment"`
}

// normalizeNetwork returns the common name of a network type used to create
// file paths. This allows differently versioned networks to use the same path.
func normalizeNetwork(network string) string {
	if strings.HasPrefix(network, "testnet") {
		return "testnet"
	}

	return network
}

func loadConfig() (*config, []string, error) {
	// Default config.
	cfg := config{
		BindAddr:         defaultBindAddr,
		UseLeHTTPS:       defaultUseLeHTTPS,
		WipeChannels:     defaultWipeChannels,
		MacaroonPath:     defaultMacaroonPath,
		TLSCertPath:      defaultTLSCertPath,
		ActionsTimeLimit: defaultActionsTimeLimit,
		UseRealIP:        defaultUseRealIP,
	}

	// Pre-parse the command line options to see if an alternative config
	// file was specified.  Any errors aside from the
	// help message error can be ignored here since they will be caught by
	// the final parse below.
	preCfg := cfg
	preParser := flags.NewParser(&preCfg, flags.HelpFlag)
	_, err := preParser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			fmt.Fprintln(os.Stderr, err)
			return nil, nil, err
		}
	}

	// Show the version and exit if the version flag was specified.
	appName := filepath.Base(os.Args[0])
	appName = strings.TrimSuffix(appName, filepath.Ext(appName))
	usageMessage := fmt.Sprintf("Use %s -h to show usage", appName)
	if preCfg.ShowVersion {
		commit := SourceCommit()
		if commit != "" {
			commit = fmt.Sprintf("Commit %s; ", commit)
		}
		fmt.Printf("%s version %s (%sGo version %s %s/%s)\n",
			appName, Version(), commit,
			runtime.Version(), runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	// If the config file path has not been modified by user, then
	// we'll use the default config file path.
	if preCfg.ConfigFile == "" {
		preCfg.ConfigFile = defaultConfigFile
	}

	// Load additional config from file.
	var configFileError error
	parser := flags.NewParser(&cfg, flags.Default)

	err = flags.NewIniParser(parser).ParseFile(preCfg.ConfigFile)
	if err != nil {
		if _, ok := err.(*os.PathError); !ok {
			fmt.Fprintf(os.Stderr, "Error parsing config "+
				"file: %v\n", err)
			fmt.Fprintln(os.Stderr, usageMessage)
			return nil, nil, err
		}
		configFileError = err
	}

	// Parse command line options again to ensure they take precedence.
	remainingArgs, err := parser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); !ok || e.Type != flags.ErrHelp {
			fmt.Fprintln(os.Stderr, usageMessage)
		}
		return nil, nil, err
	}

	// Create the home directory if it doesn't already exist.
	funcName := "loadConfig"
	err = os.MkdirAll(defaultDataDir, 0700)
	if err != nil {
		// Show a nicer error message if it's because a symlink is
		// linked to a directory that does not exist (probably because
		// it's not mounted).
		if e, ok := err.(*os.PathError); ok && os.IsExist(err) {
			if link, lerr := os.Readlink(e.Path); lerr == nil {
				str := "is symlink %s -> %s mounted?"
				err = fmt.Errorf(str, e.Path, link)
			}
		}

		str := "%s: Failed to create home directory: %v"
		err := fmt.Errorf(str, funcName, err)
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, err
	}

	// Multiple networks can't be selected simultaneously.  Count number of
	// network flags passed and assign active network params.
	numNets := 0
	if cfg.MainNet {
		numNets++
		activeNetParams = &decredMainNetParams
	}
	if cfg.TestNet {
		numNets++
		activeNetParams = &decredTestNet3Params
	}
	if cfg.SimNet {
		numNets++
		activeNetParams = &decredSimNetParams
	}
	if numNets > 1 {
		str := "%s: mainnet, testnet and simnet params can't be " +
			"used together -- choose one of the three"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}

	// If user didn't set the LndNode to connect to then use the default
	// port for the active network (localhost:<default_port>).
	if cfg.LndNode == "" {
		cfg.LndNode = "localhost:" + activeNetParams.rpcPort
	}

	// Initialize log rotation.  After log rotation has been initialized, the
	// logger variables may be used.
	defaultLogPath = strings.Replace(defaultLogPath, "testnet", normalizeNetwork(activeNetParams.Name), 1)
	initLogRotator(defaultLogPath)
	setLogLevels(defaultLogLevel)

	if cfg.UseLeHTTPS && cfg.Domain == "" {
		err := fmt.Errorf("%s: domain must be specified to use Let's Encrypt HTTPS", funcName)
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, err
	}

	// Verify actions time limit
	if cfg.ActionsTimeLimit.Seconds() <= 0 {
		str := "%s: ActionsTimeLimit cannot be <= 0"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, err
	}

	// Warn about missing config file only after all other configuration is
	// done.  This prevents the warning on help messages and invalid
	// options.  Note this should go directly before the return.
	if configFileError != nil {
		log.Warnf("%v", configFileError)
	}

	return &cfg, remainingArgs, nil
}
