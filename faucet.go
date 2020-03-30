package main

import (
	"encoding/hex"
	"fmt"
	"html/template"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	macaroon "gopkg.in/macaroon.v2"

	"github.com/davecgh/go-spew/spew"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrutil/v2"
	"github.com/decred/dcrd/wire"
	"github.com/decred/dcrlnd/lnrpc"
	"github.com/decred/dcrlnd/macaroons"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	// maxChannelSize is the larget channel that the faucet will create to
	// another peer.
	maxChannelSize int64 = (1 << 30)

	// minChannelSize is the smallest channel that the faucet will extend
	// to a peer.
	minChannelSize int64 = 50000

	// maxPaymentAtoms is the larget payment amount in atoms that the faucet
	// will pay to an invoice
	maxPaymentAtoms int64 = 1000
)

var (
	// GenerateInvoiceAction represents an action to generate invoice on post forms
	GenerateInvoiceAction = "generateinvoice"

	// PayInvoiceAction represents an action to pay invoice on post forms
	PayInvoiceAction = "payinvoice"

	// OpenChannelAction represents an action to open channel on post forms
	OpenChannelAction = "openchannel"

	// requestIPs stores the last time an ip did an action,
	// and is protected by a mutex that must be held for reads/writes.
	rateLimitMtx sync.RWMutex
	requestIPs   map[string]time.Time
)

// lightningFaucet is a Decred Channel Faucet. The faucet itself is a web app
// that is capable of programmatically opening channels with users with the
// size of the channel parametrized by the user. The faucet required a
// connection to a local lnd node in order to operate properly. The faucet
// implements the constrains on the channel size, and also will only open a
// single channel to a particular node. Finally, the faucet will periodically
// close channels based on their age as the faucet will only open up 100
// channels total at any given time.
type lightningFaucet struct {
	lnd lnrpc.LightningClient

	templates *template.Template

	openChannels map[wire.OutPoint]time.Time
	cfg          *config

	//Network info
	network string
}

// cleanAndExpandPath expands environment variables and leading ~ in the
// passed path, cleans the result, and returns it.
func cleanAndExpandPath(path string) string {
	if path == "" {
		return ""
	}

	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		var homeDir string
		user, err := user.Current()
		if err == nil {
			homeDir = user.HomeDir
		} else {
			homeDir = os.Getenv("HOME")
		}

		path = strings.Replace(path, "~", homeDir, 1)
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%,
	// but the variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}

// verifyTimeLimit will return an error if the given IP address
// has already performed an action within the given time limit.
func verifyTimeLimit(ip string, actionsTimeLimit time.Duration) error {
	rateLimitMtx.RLock()
	lastRequestTime, found := requestIPs[ip]
	rateLimitMtx.RUnlock()
	if found {
		nextAllowedRequest := lastRequestTime.Add(actionsTimeLimit)
		coolDownTime := time.Until(nextAllowedRequest)

		if coolDownTime >= 0 {
			err := fmt.Errorf("client(%v) may only do an action every "+
				"%v. Wait another %v.",
				ip, actionsTimeLimit, coolDownTime)
			return err
		}
	}
	return nil
}

// getRealIP returns the clients IP address. If the useRealIP config is set
// it will use the X-Real-IP or X-Forwarded-For HTTP headers, otherwise it will
// return the request.RemoteAddr field.
//
// You should only use this option if you have correctly configured
// your reverse proxies. If you do not use a reverse proxy or are configured
// to pass on the clients headers without any filters, malicious clients
// may inject fake IPs.
func getRealIP(r *http.Request, useRealIP bool) (string, error) {
	if useRealIP {
		xForwardFor := http.CanonicalHeaderKey("X-Forwarded-For")
		xRealIP := http.CanonicalHeaderKey("X-Real-IP")

		if xrip := r.Header.Get(xRealIP); xrip != "" {
			return xrip, nil
		} else if xff := r.Header.Get(xForwardFor); xff != "" {
			i := strings.Index(xff, ", ")
			if i == -1 {
				i = len(xff)
			}
			return xff[:i], nil
		}
		log.Warn(`"X-Forwarded-For" and "X-Real-IP" headers invalid, ` +
			`using RemoteAddr instead`)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", err
	}
	return host, nil
}

// getChainInfo makes a request to get information about dcrlnd chain.
func getChainInfo(l lnrpc.LightningClient) (*lnrpc.Chain, error) {
	infoReq := &lnrpc.GetInfoRequest{}
	info, err := l.GetInfo(ctxb, infoReq)
	if err != nil {
		return nil, fmt.Errorf("get info: %v", err)
	}

	return info.Chains[0], nil
}

// newLightningFaucet creates a new channel faucet that's bound to a cluster of
// lnd nodes, and uses the passed templates to render the web page.
func newLightningFaucet(cfg *config,
	templates *template.Template) (*lightningFaucet, error) {

	// First attempt to establish a connection to lnd's RPC sever.
	tlsCertPath := cleanAndExpandPath(cfg.TLSCertPath)
	creds, err := credentials.NewClientTLSFromFile(tlsCertPath, "")
	if err != nil {
		return nil, fmt.Errorf("unable to read cert file: %v", err)
	}
	opts := []grpc.DialOption{grpc.WithTransportCredentials(creds)}

	// Load the specified macaroon file.
	macPath := cleanAndExpandPath(cfg.MacaroonPath)
	macBytes, err := ioutil.ReadFile(macPath)
	if err != nil {
		return nil, err
	}
	mac := &macaroon.Macaroon{}
	if err = mac.UnmarshalBinary(macBytes); err != nil {
		return nil, err
	}

	// Now we append the macaroon credentials to the dial options.
	opts = append(
		opts,
		grpc.WithPerRPCCredentials(macaroons.NewMacaroonCredential(mac)),
	)

	conn, err := grpc.Dial(cfg.LndNode, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to dial to lnd's gRPC server: %v", err)
	}

	// If we're able to connect out to the lnd node, then we can start up
	// the faucet safely.
	lnd := lnrpc.NewLightningClient(conn)

	// Get chain info to stop creation if the dcrlnd and dcrlnfaucet
	// are set in different networks.
	chain, err := getChainInfo(lnd)
	if err != nil {
		return nil, err
	}
	netParams := normalizeNetwork(activeNetParams.Name)
	if chain.Network != netParams {
		return nil, fmt.Errorf(
			"dcrlnd and dcrlnfaucet are set in different "+
				"networks <dcrlnd: %v / dcrlnfaucet: %v>",
			chain.Network, netParams)
	}

	return &lightningFaucet{
		lnd:       lnd,
		templates: templates,
		cfg:       cfg,
		network:   chain.Network,
	}, nil
}

// Start launches all the goroutines necessary for routine operation of the
// lightning faucet.
func (l *lightningFaucet) Start(cfg *config) {
	requestIPs = make(map[string]time.Time)

	if !cfg.DisableZombieSweeper {
		go l.zombieChanSweeper()
	}
}

// zombieChanSweeper is a goroutine that is tasked with cleaning up "zombie"
// channels. A zombie channel is a channel in which the peer we have the
// channel open with hasn't been online for greater than 48 hours. We'll
// periodically perform a sweep every hour to close out any lingering zombie
// channels.
//
// NOTE: This MUST be run as a goroutine.
func (l *lightningFaucet) zombieChanSweeper() {
	log.Info("zombie chan sweeper active")

	// Any channel peer that hasn't been online in more than 48 hours past
	// from now will have their channels closed out.
	timeCutOff := time.Now().Add(-time.Hour * 48)

	// Upon initial boot, we'll do a scan to close out any channels that
	// are now considered zombies while we were down.
	l.sweepZombieChans(timeCutOff)

	// Every hour we'll consume a new tick and perform a sweep to close out
	// any zombies channels.
	zombieTicker := time.NewTicker(time.Hour * 1)
	for range zombieTicker.C {
		log.Info("Performing zombie channel sweep!")

		// In order to ensure we close out the proper channels, we also
		// calculate the 48 hour offset from the point of our next
		// tick.
		timeCutOff = time.Now().Add(-time.Hour * 48)

		// With the time cut off calculated, we'll force close any
		// channels that are now considered "zombies".
		l.sweepZombieChans(timeCutOff)
	}
}

// strPointToChanPoint concerts a string outpoint (txid:index) into an lnrpc
// ChannelPoint object.
func strPointToChanPoint(stringPoint string) (*lnrpc.ChannelPoint, error) {
	s := strings.Split(stringPoint, ":")

	txid, err := chainhash.NewHashFromStr(s[0])
	if err != nil {
		return nil, err
	}

	index, err := strconv.Atoi(s[1])
	if err != nil {
		return nil, err
	}

	return &lnrpc.ChannelPoint{
		FundingTxid: &lnrpc.ChannelPoint_FundingTxidBytes{
			FundingTxidBytes: txid[:],
		},
		OutputIndex: uint32(index),
	}, nil
}

// sweepZombieChans performs a sweep of the set of channels that the faucet has
// active to close out any channels that are now considered to be a "zombie". A
// channel is a zombie if the peer with have the channel open is currently
// offline, and we haven't detected them as being online since timeCutOff.
//
// TODO(roasbeef): after removing the node ANN on startup, will need to rely on
// LinkNode information.
func (l *lightningFaucet) sweepZombieChans(timeCutOff time.Time) {
	// Fetch all the facuet's currently open channels.
	openChanReq := &lnrpc.ListChannelsRequest{}
	openChannels, err := l.lnd.ListChannels(ctxb, openChanReq)
	if err != nil {
		log.Errorf("unable to fetch open channels: %v", err)
		return
	}

	for _, channel := range openChannels.Channels {
		// For each channel we'll first fetch the announcement
		// information for the peer that we have the channel open with.
		nodeInfoResp, err := l.lnd.GetNodeInfo(ctxb,
			&lnrpc.NodeInfoRequest{
				PubKey: channel.RemotePubkey,
			})
		if err != nil {
			log.Errorf("unable to get node pubkey: %v", err)
			continue
		}

		// Convert the unix time stamp into a time.Time object.
		lastSeen := time.Unix(int64(nodeInfoResp.Node.LastUpdate), 0)

		// If the last time we saw this peer online was _before_ our
		// time cutoff, and the peer isn't currently online, then we'll
		// force close out the channel.
		if lastSeen.Before(timeCutOff) && !channel.Active {
			log.Infof("ChannelPoint(%v) is a zombie, last seen: %v",
				channel.ChannelPoint, lastSeen)

			chanPoint, err := strPointToChanPoint(channel.ChannelPoint)
			if err != nil {
				log.Errorf("unable to get chan point: %v", err)
				continue
			}
			txid, err := l.closeChannel(chanPoint, true)
			if err != nil {
				log.Errorf("unable to close zombie chan: %v", err)
				continue
			}

			log.Infof("closed zombie chan, txid: %v", txid)
		}
	}
}

// closeChannel closes out a target channel optionally executing a force close.
// This function will block until the closing transaction has been broadcast.
func (l *lightningFaucet) closeChannel(chanPoint *lnrpc.ChannelPoint,
	force bool) (*chainhash.Hash, error) {

	closeReq := &lnrpc.CloseChannelRequest{
		ChannelPoint: chanPoint,
		Force:        force,
	}
	stream, err := l.lnd.CloseChannel(ctxb, closeReq)
	if err != nil {
		log.Errorf("unable to start channel close: %v", err)
	}

	// Consume the first response which'll be sent once the closing
	// transaction has been broadcast.
	resp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("unable to close chan: %v", err)
	}

	update, ok := resp.Update.(*lnrpc.CloseStatusUpdate_ClosePending)
	if !ok {
		return nil, fmt.Errorf("didn't get a pending update")
	}

	// Convert the raw bytes into a new chainhash so we gain access to its
	// utility methods.
	closingHash := update.ClosePending.Txid
	return chainhash.NewHash(closingHash)
}

// homePageContext defines the initial context required for rendering home
// page. The home page displays some basic statistics, errors in the case of an
// invalid channel submission, and finally a splash page upon successful
// creation of a channel.
type homePageContext struct {
	// NodeInfo is the response of lnrpc.GetInfo
	NodeInfo *lnrpc.GetInfoResponse

	// FaucetVersion is version of executable of lightning-faucet
	FaucetVersion string

	// FaucetCommit is the commit from the builing of lightning-faucet
	FaucetCommit string
	// NumCoins is the number of coins in Decred that the faucet has available
	// for channel creation.
	NumCoins float64

	// GitCommitHash is the git HEAD's commit hash of
	// $GOPATH/src/github.com/decred/dcrlnd
	GitCommitHash string

	// NodeAddr is the full <pubkey>@host:port where the faucet can be
	// connect to.
	NodeAddr string

	// SubmissionError is a enum that stores if any error took place during
	// the creation of a channel.
	SubmissionError ChanCreationError

	// ChannelTxid is the txid of the created funding channel. If this
	// field is an empty string, then that indicates the channel hasn't yet
	// been created.
	ChannelTxid string

	// NumConfs is the number of confirmations required for the channel to
	// open up.
	NumConfs uint32

	// FormFields contains the values which were submitted through the form.
	FormFields map[string]string

	// PendingChannels contains all of this faucets pending channels.
	PendingChannels []*lnrpc.PendingChannelsResponse_PendingOpenChannel

	// ActiveChannels contains all of this faucets active channels.
	ActiveChannels []*lnrpc.Channel

	// InvoicePaymentRequest the payment request generated by an invoice.
	InvoicePaymentRequest string

	// PayInvoiceRequest the pay request for an invoice.
	PayInvoiceRequest string

	// PayInvoiceAction action
	PayInvoiceAction string

	// OpenChannelAction indicates the form action to open a channel
	OpenChannelAction string

	// GenerateInvoiceAction indicates the form action to generate a new Invoice
	GenerateInvoiceAction string

	// Disable generate invoices form
	DisableGenerateInvoices bool

	// Disable invoices payments form
	DisablePayInvoices bool

	// Payment infos
	PaymentDestination string
	PaymentDescription string
	PaymentAmount      string
	PaymentHash        string
	PaymentPreimage    string
	PaymentHops        []*lnrpc.Hop

	// Network info
	Network string
}

// fetchHomeState is helper functions that populates the homePageContext with
// the latest state from the local lnd node.
func (l *lightningFaucet) fetchHomeState() (*homePageContext, error) {
	// First query for the general information from the lnd node, this'll
	// be used to populate the number of active channel as well as the
	// identity of the node.
	infoReq := &lnrpc.GetInfoRequest{}
	nodeInfo, err := l.lnd.GetInfo(ctxb, infoReq)
	if err != nil {
		log.Errorf("rpc GetInfoRequest failed: %v", err)
		return nil, err
	}

	activeChanReq := &lnrpc.ListChannelsRequest{}
	activeChannels, err := l.lnd.ListChannels(ctxb, activeChanReq)
	if err != nil {
		log.Errorf("rpc ListChannels failed: %v", err)
		return nil, err
	}

	pendingChanReq := &lnrpc.PendingChannelsRequest{}
	pendingChannels, err := l.lnd.PendingChannels(ctxb, pendingChanReq)
	if err != nil {
		log.Errorf("rpc PendingChannels failed: %v", err)
		return nil, err
	}

	// Next obtain the wallet's available balance which indicates how much
	// we can allocate towards channels.
	balReq := &lnrpc.WalletBalanceRequest{}
	walletBalance, err := l.lnd.WalletBalance(ctxb, balReq)
	if err != nil {
		log.Errorf("rpc WalletBalance failed: %v", err)
		return nil, err
	}

	// Parse the git commit used to build the node, assuming it was built
	// with `make install`.
	gitHash := ""
	if p := strings.LastIndex(nodeInfo.Version, "commit="); p > -1 && len(nodeInfo.Version)-p-7 >= 40 {
		// Read the right-most 40 chars and assume it's the git hash.
		gitHash = nodeInfo.Version[len(nodeInfo.Version)-40:]
	}

	nodeAddr := ""
	if len(nodeInfo.Uris) == 0 {
		log.Warn("nodeInfo did not include a URI. external_ip config of dcrlnd is probably not set")
	} else {
		nodeAddr = nodeInfo.Uris[0]
	}

	return &homePageContext{
		NodeInfo:                nodeInfo,
		FaucetVersion:           Version(),
		FaucetCommit:            SourceCommit(),
		NumCoins:                dcrutil.Amount(walletBalance.ConfirmedBalance).ToCoin(),
		GitCommitHash:           strings.Replace(gitHash, "'", "", -1),
		NodeAddr:                nodeAddr,
		NumConfs:                6,
		FormFields:              make(map[string]string),
		ActiveChannels:          activeChannels.Channels,
		PendingChannels:         pendingChannels.PendingOpenChannels,
		OpenChannelAction:       OpenChannelAction,
		GenerateInvoiceAction:   GenerateInvoiceAction,
		PayInvoiceAction:        PayInvoiceAction,
		DisableGenerateInvoices: l.cfg.DisableGenerateInvoices,
		DisablePayInvoices:      l.cfg.DisablePayInvoices,
		Network:                 l.network,
	}, nil
}

// faucetHome renders the main home page for the faucet. This includes the form
// to create channels, the network statistics, and the splash page upon channel
// success.
//
// NOTE: This method implements the http.Handler interface.
func (l *lightningFaucet) faucetHome(w http.ResponseWriter, r *http.Request) {
	// First obtain the home template from our cache of pre-compiled
	// templates.
	homeTemplate := l.templates.Lookup("index.html")

	// In order to render the home template we'll need the necessary
	// context, so we'll grab that from the lnd daemon now in order to get
	// the most up to date state.
	homeInfo, err := l.fetchHomeState()
	if err != nil {
		log.Error("unable to fetch home state")
		http.Error(w, "unable to render home page", http.StatusInternalServerError)
		return
	}

	// If the method is GET, then we'll render the home page with the form
	// itself.
	switch {
	case r.Method == http.MethodGet:
		homeTemplate.Execute(w, homeInfo)

	// Otherwise, if the method is POST, then the user is submitting the
	// form to open a channel, so we'll pass that off to the openChannel
	// handler.
	case r.Method == http.MethodPost:
		actions := r.URL.Query()["action"]
		if len(actions) > 0 {
			action := actions[0]
			// action == 0 is to open channel, action == 1 is to generate, action == 2 is to pay
			switch action {
			case OpenChannelAction:
				l.openChannel(homeTemplate, homeInfo, w, r)
			case GenerateInvoiceAction:
				l.generateInvoice(homeTemplate, homeInfo, w, r)
			case PayInvoiceAction:
				l.payInvoice(homeTemplate, homeInfo, w, r)
			}
		}
	// If the method isn't either of those, then this is an error as we
	// only support the two methods above.
	default:
		http.Error(w, "", http.StatusMethodNotAllowed)
	}
}

// infoHome render information pages
//
// NOTE: This method implements the http.Handler interface.
func (l *lightningFaucet) infoPage(w http.ResponseWriter, r *http.Request) {
	// get info template from our cache of pre-compiled templates.
	infoTemplate := l.templates.Lookup("info.html")

	// In order to render the info template we'll need the necessary
	// context, so we'll grab that from the lnd daemon now in order to get
	// the most up to date state.
	homeInfo, err := l.fetchHomeState()
	if err != nil {
		log.Error("unable to fetch info state")
		http.Error(w, "unable to render info page", http.StatusInternalServerError)
		return
	}

	// If the method is not GET, then we'll render an error.
	if r.Method != http.MethodGet {
		log.Error("method don't allowed in this url")
		http.Error(w, "method don't allowed in this url", http.StatusMethodNotAllowed)
		return
	}

	infoTemplate.Execute(w, homeInfo)
}

func (l *lightningFaucet) pendingChannelExistsWithNode(nodePub string) bool {
	pendingChanReq := &lnrpc.PendingChannelsRequest{}
	resp, err := l.lnd.PendingChannels(ctxb, pendingChanReq)
	if err != nil {
		return false
	}

	for _, channel := range resp.PendingOpenChannels {
		if channel.Channel.RemoteNodePub == nodePub {
			return true
		}
	}

	return false
}

// activeCahannelExistsWithNode return true if the faucet already has a channel open
// with the target node, and false otherwise.
func (l *lightningFaucet) channelExistsWithNode(nodePub string) bool {
	listChanReq := &lnrpc.ListChannelsRequest{}
	resp, err := l.lnd.ListChannels(ctxb, listChanReq)
	if err != nil {
		return false
	}

	for _, channel := range resp.Channels {
		if channel.RemotePubkey == nodePub {
			return true
		}
	}

	return false
}

// connectedToNode returns true if the faucet is connected to the node, and
// false otherwise.
func (l *lightningFaucet) connectedToNode(nodePub string) bool {
	peersReq := &lnrpc.ListPeersRequest{}
	resp, err := l.lnd.ListPeers(ctxb, peersReq)
	if err != nil {
		return false
	}

	for _, peer := range resp.Peers {
		if peer.PubKey == nodePub {
			return true
		}
	}

	return false
}

// openChannel is a hybrid http.Handler that handles: the validation of the
// channel creation form, rendering errors to the form, and finally creating
// channels if all the parameters check out.
func (l *lightningFaucet) openChannel(homeTemplate *template.Template,
	homeState *homePageContext, w http.ResponseWriter, r *http.Request) {
	// Before we can obtain the values the user entered in the form, we
	// need to parse all parameters.  First attempt to establish a
	// connection with the
	if err := r.ParseForm(); err != nil {
		http.Error(w, "unable to parse form", 500)
		return
	}

	nodePubStr := r.FormValue("node")
	amt := r.FormValue("amt")
	bal := r.FormValue("bal")

	homeState.FormFields["Node"] = nodePubStr
	homeState.FormFields["Amt"] = amt
	homeState.FormFields["Bal"] = bal

	// With the forms details parsed, extract out the public key of the
	// target peer.
	nodePub, err := hex.DecodeString(nodePubStr)
	if err != nil {
		homeState.SubmissionError = InvalidAddress
		homeTemplate.Execute(w, homeState)
		return
	}

	// If we already have a channel with this peer, then we'll fail the
	// request as we have a policy of only one channel per node.
	if l.channelExistsWithNode(nodePubStr) {
		homeState.SubmissionError = HaveChannel
		homeTemplate.Execute(w, homeState)
		return
	}

	// If we already have a channel with this peer, then we'll fail the
	// request as we have a policy of only one channel per node.
	if l.pendingChannelExistsWithNode(nodePubStr) {
		homeState.SubmissionError = HavePendingChannel
		homeTemplate.Execute(w, homeState)
		return
	}

	// If we're not connected to the node, then we won't be able to extend
	// a channel to them. So we'll exit early with an error here.
	if !l.connectedToNode(nodePubStr) {
		homeState.SubmissionError = NotConnected
		homeTemplate.Execute(w, homeState)
		return
	}

	// With the connection established (or already present) with the target
	// peer, we'll now parse out the rest of the fields, performing
	// validation and exiting early if any field is invalid.
	chanSizeFloat, err := strconv.ParseFloat(amt, 64)
	if err != nil {
		homeState.SubmissionError = ChanAmountNotNumber
		homeTemplate.Execute(w, homeState)
		return
	}
	pushAmtFloat, err := strconv.ParseFloat(bal, 64)
	if err != nil {
		homeState.SubmissionError = PushIncorrect
		homeTemplate.Execute(w, homeState)
		return
	}

	// Convert from input (dcr) to api (atoms) units.
	chanSize := int64(chanSizeFloat * 1e8)
	pushAmt := int64(pushAmtFloat * 1e8)

	// With the initial validation complete, we'll now ensure the channel
	// size and push amt meet our constraints.
	switch {
	// The target channel can't be below the constant min channel size.
	case chanSize < minChannelSize:
		homeState.SubmissionError = ChannelTooSmall
		homeTemplate.Execute(w, homeState)
		return

	// The target channel can't be above the max channel size.
	case chanSize > maxChannelSize:
		homeState.SubmissionError = ChannelTooLarge
		homeTemplate.Execute(w, homeState)
		return

	// The amount pushed to the other side as part of the channel creation
	// MUST be less than the size of the channel itself.
	case pushAmt >= chanSize:
		homeState.SubmissionError = PushIncorrect
		homeTemplate.Execute(w, homeState)
		return
	}

	// If we were able to connect to the peer successfully, and all the
	// parameters check out, then we'll parse out the remaining channel
	// parameters and initiate the funding workflow.
	openChanReq := &lnrpc.OpenChannelRequest{
		NodePubkey:         nodePub,
		LocalFundingAmount: chanSize,
		PushAtoms:          pushAmt,
	}
	log.Infof("attempting to create channel with params: %v",
		spew.Sdump(openChanReq))

	openChanStream, err := l.lnd.OpenChannel(ctxb, openChanReq)
	if err != nil {
		log.Errorf("Opening channel stream failed: %v", err)
		homeState.SubmissionError = ChannelOpenFail
		homeTemplate.Execute(w, homeState)
		return
	}

	// Consume the first update from the open channel stream which
	// indicates that the channel has been broadcast to the network.
	chanUpdate, err := openChanStream.Recv()
	if err != nil {
		log.Errorf("Channel update failed: %v", err)
		homeState.SubmissionError = ChannelOpenFail
		homeTemplate.Execute(w, homeState)
		return
	}

	pendingUpdate := chanUpdate.Update.(*lnrpc.OpenStatusUpdate_ChanPending).ChanPending
	fundingTXID, _ := chainhash.NewHash(pendingUpdate.Txid)

	log.Infof("channel created with txid: %v", fundingTXID)

	homeState.ChannelTxid = fundingTXID.String()
	if err := homeTemplate.Execute(w, homeState); err != nil {
		log.Errorf("unable to render home page: %v", err)
	}
}

// CloseAllChannels attempt unconditionally close ALL of the faucet's currently
// open channels. In the case that a channel is active a cooperative closure
// will be executed, in the case that a channel is inactive, a force close will
// be attempted.
func (l *lightningFaucet) CloseAllChannels() error {
	openChanReq := &lnrpc.ListChannelsRequest{}
	openChannels, err := l.lnd.ListChannels(ctxb, openChanReq)
	if err != nil {
		return fmt.Errorf("unable to fetch open channels: %v", err)
	}

	for _, channel := range openChannels.Channels {
		log.Infof("Attempting to close channel: %s", channel.ChannelPoint)

		chanPoint, err := strPointToChanPoint(channel.ChannelPoint)
		if err != nil {
			log.Errorf("unable to get chan point: %v", err)
			continue
		}

		forceClose := !channel.Active
		if forceClose {
			log.Info("Attempting force close")
		}

		closeTxid, err := l.closeChannel(chanPoint, forceClose)
		if err != nil {
			log.Errorf("unable to close channel: %v", err)
			continue
		}

		log.Infof("closing txid: %v", closeTxid)
	}

	return nil
}

// generateInvoice is a hybrid http.Handler that handles: the validation of the
// generate invoice form, rendering errors to the form, and finally generating
// invoice if all the parameters check out.
func (l *lightningFaucet) generateInvoice(homeTemplate *template.Template,
	homeState *homePageContext, w http.ResponseWriter, r *http.Request) {

	// Disable generate invoice if user set this parameter
	if l.cfg.DisableGenerateInvoices {
		http.Error(w, "generate invoices was disabled", 403)
		return
	}

	// Get the invoice from users form and set the input value again
	// for the users repeat the action or verify
	amt := r.FormValue("amt")
	description := r.FormValue("description")
	homeState.FormFields["Amt"] = amt
	homeState.FormFields["Description"] = description

	// Verify IP before continuing
	clientIP, err := getRealIP(r, l.cfg.UseRealIP)
	if err != nil {
		log.Errorf("Can't get client ip: %v", err)
		homeState.SubmissionError = InternalServerError
		homeTemplate.Execute(w, homeState)
		return
	}

	if err = verifyTimeLimit(clientIP, l.cfg.ActionsTimeLimit); err != nil {
		log.Errorf("%v", err)
		homeState.SubmissionError = TimeLimitError
		homeTemplate.Execute(w, homeState)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "unable to parse form", 500)
		return
	}

	amtDcr, err := strconv.ParseFloat(amt, 64)
	if err != nil {
		homeState.SubmissionError = ChanAmountNotNumber
		homeTemplate.Execute(w, homeState)
		return
	}
	if amtDcr > 0.2 {
		log.Warnf("Attempt to generate high value invoice (%f) from %s",
			amtDcr, r.RemoteAddr)
		homeState.SubmissionError = InvoiceAmountTooHigh
		homeTemplate.Execute(w, homeState)
		return
	}
	amtAtoms := int64(amtDcr * 1e8)

	invoiceReq := &lnrpc.Invoice{
		CreationDate: time.Now().Unix(),
		Value:        amtAtoms,
		Memo:         description,
	}
	invoice, err := l.lnd.AddInvoice(ctxb, invoiceReq)
	if err != nil {
		log.Errorf("Generate invoice failed: %v", err)
		homeState.SubmissionError = ErrorGeneratingInvoice
		homeTemplate.Execute(w, homeState)
		return
	}

	log.Infof("Generated invoice #%d for %s rhash=%064x", invoice.AddIndex,
		dcrutil.Amount(amtAtoms), invoice.RHash)

	homeState.InvoicePaymentRequest = invoice.PaymentRequest

	if err := homeTemplate.Execute(w, homeState); err != nil {
		log.Errorf("unable to render home page: %v", err)
	}

	// Update time for client request
	rateLimitMtx.Lock()
	requestIPs[clientIP] = time.Now()
	rateLimitMtx.Unlock()
}

// payInvoice is a hybrid http.Handler that handles: the validation of the
// pay invoice form, rendering errors to the form, and finally streaming the
// payment if all the parameters check out.
func (l *lightningFaucet) payInvoice(homeTemplate *template.Template,
	homeState *homePageContext, w http.ResponseWriter, r *http.Request) {

	// Disable pay invoice if user set this parameter
	if l.cfg.DisablePayInvoices {
		http.Error(w, "invoices payment was disabled", 403)
		return
	}

	// Get the invoice from users form and set the input value again
	// for the users repeat the action or verify
	rawPayReq := r.FormValue("payinvoice")
	homeState.FormFields["Payinvoice"] = rawPayReq

	// Verify IP before open a channel
	clientIP, err := getRealIP(r, l.cfg.UseRealIP)
	if err != nil {
		log.Errorf("Can't get client ip: %v", err)
		homeState.SubmissionError = InternalServerError
		homeTemplate.Execute(w, homeState)
		return
	}

	if err = verifyTimeLimit(clientIP, l.cfg.ActionsTimeLimit); err != nil {
		log.Errorf("%v", err)
		homeState.SubmissionError = TimeLimitError
		homeTemplate.Execute(w, homeState)
		return
	}

	// Try to verify and decode the invoice from users form.
	payReq := strings.TrimSpace(rawPayReq)
	payReqString := &lnrpc.PayReqString{PayReq: payReq}
	decodedPayReq, err := l.lnd.DecodePayReq(ctxb, payReqString)
	if err != nil {
		log.Errorf("Error on decode pay_req: %v", err)
		homeState.SubmissionError = ErrorDecodingPayReq
		homeTemplate.Execute(w, homeState)
		return
	}

	decodedAmount := decodedPayReq.GetNumAtoms()

	// Verify invoice amount.
	if decodedAmount > maxPaymentAtoms {
		log.Errorf("Max payout, pay_amount: %v", decodedAmount)
		homeState.SubmissionError = ErrorPaymentAmount
		homeTemplate.Execute(w, homeState)
		return
	}

	// Create the payment request.
	req := &lnrpc.SendRequest{
		PaymentRequest:       payReq,
		Amt:                  decodedAmount,
		IgnoreMaxOutboundAmt: false,
	}

	// Create a payment stream to send your payment request.
	paymentStream, err := l.lnd.SendPayment(ctxb)
	if err != nil {
		log.Errorf("Error on create Payment Stream: %v", err)
		homeState.SubmissionError = PaymentStreamError
		homeTemplate.Execute(w, homeState)
		return
	}

	// Stream the payment request.
	if err := paymentStream.Send(req); err != nil {
		log.Errorf("Error on send pay_req: %v", err)
		homeState.SubmissionError = PaymentStreamError
		homeTemplate.Execute(w, homeState)
		return
	}

	// Receive response from streamed payment request.
	resp, err := paymentStream.Recv()
	if err != nil {
		log.Errorf("Error on receive pay_req response: %v", err)
		homeState.SubmissionError = PaymentStreamError
		homeTemplate.Execute(w, homeState)
		return
	}

	paymentStream.CloseSend()

	// Log response and send to homeState to create the html version.
	log.Infof("Invoice has been paid destination=%v	description=%v amount=%v pay_hash:%v preimage=%v",
		decodedPayReq.Destination, decodedPayReq.Description,
		dcrutil.Amount(resp.PaymentRoute.TotalAmt), hex.EncodeToString(resp.PaymentHash),
		hex.EncodeToString(resp.PaymentPreimage))

	amount := dcrutil.Amount(resp.PaymentRoute.TotalAmt)

	homeState.PaymentDestination = decodedPayReq.Destination
	homeState.PaymentDescription = decodedPayReq.Description
	homeState.PaymentAmount = amount.String()
	homeState.PaymentHash = hex.EncodeToString(resp.PaymentHash)
	homeState.PaymentPreimage = hex.EncodeToString(resp.PaymentPreimage)
	homeState.PaymentHops = resp.PaymentRoute.Hops

	if err := homeTemplate.Execute(w, homeState); err != nil {
		log.Errorf("unable to render home page: %v", err)
	}

	// Update time for client request
	rateLimitMtx.Lock()
	requestIPs[clientIP] = time.Now()
	rateLimitMtx.Unlock()
}
