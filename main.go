// Copyright (c) 2017 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"net"

	"github.com/decred/dcrd/dcrutil"
	"github.com/decred/dcrd/rpcclient"
)

var (
	cfg *config
)

// Balance and limits
var lastBalance float64
var transactionLimit float64

// Daemon Params to use
var dcrwClient *rpcclient.Client

// Map of received IP requests for funds.
var requestAmounts map[int64]float64
var requestIPs map[string]int64

type jsonResponse struct {
	Txid  string
	Error string
}

// Overall Data structure given to the template to render
type testnetFaucetInfo struct {
	Address          string
	Amount           float64
	BlockHeight      int64
	Balance          float64
	TransactionLimit float64
	Error            error
	TimeLimit        int64
	SentToday        float64
	Success          string
}

func requestFunds(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	fp := filepath.Join("public/views", "design_sketch.html")
	amountSentToday := calculateAmountSentToday()
	testnetFaucetInformation := &testnetFaucetInfo{
		Address:          cfg.WalletAddress,
		Amount:           cfg.WithdrawalAmount,
		Balance:          lastBalance,
		TransactionLimit: transactionLimit,
		TimeLimit:        cfg.WithdrawalTimeLimit,
		SentToday:        amountSentToday,
	}

	tmpl, err := template.New("home").ParseFiles(fp)
	if err != nil {
		panic(err)
	}

	hostIP, err := getClientIP(r)
	if err != nil {
		panic(err)
	}

	if r.Method == "GET" {
		sendReply(w, r, tmpl, testnetFaucetInformation, nil)
		return
	}

	if err := r.ParseForm(); err != nil {
		sendReply(w, r, tmpl, testnetFaucetInformation, err)
		return
	}

	addressInput := r.FormValue("address")
	amount := cfg.WithdrawalAmount
	amountInput := r.FormValue("amount")
	overridetokenInput := r.FormValue("overridetoken")

	// enforce ratelimit unless overridetoken was specified and matches
	if overridetokenInput != cfg.OverrideToken {
		lastRequestTime, found := requestIPs[hostIP]
		if found {
			nextAllowedRequest := lastRequestTime + cfg.WithdrawalTimeLimit
			coolDownTime := nextAllowedRequest - time.Now().Unix()

			if coolDownTime >= 0 {
				err = fmt.Errorf("You may only withdraw %v DCR every "+
					"%v seconds.  Please wait another %v seconds.",
					cfg.WithdrawalAmount, cfg.WithdrawalTimeLimit, coolDownTime)
				sendReply(w, r, tmpl, testnetFaucetInformation, err)
				return
			}
		}
	}

	// check amount if specified
	if amountInput != "" {
		amount, err = strconv.ParseFloat(amountInput, 32)
		if err != nil {
			err = fmt.Errorf("amount invalid: %v", err)
			sendReply(w, r, tmpl, testnetFaucetInformation, err)
			return
		}
	}

	if amount <= 0 {
		err = errors.New("amount must be greater than 0")
		sendReply(w, r, tmpl, testnetFaucetInformation, err)
		return
	}

	// enforce the transaction limit unconditionally
	if amount > transactionLimit {
		err = errors.New("amount exceeds limit")
		sendReply(w, r, tmpl, testnetFaucetInformation, err)
		return
	}

	// Decode address and amount and send transaction.
	address, err := dcrutil.DecodeAddress(addressInput)
	if err != nil {
		log.Errorf("ip %v submitted bad address %v", hostIP, addressInput)
		sendReply(w, r, tmpl, testnetFaucetInformation, err)
		return
	}

	if !address.IsForNet(activeNetParams.Params) {
		err = fmt.Errorf("address "+
			"%s is not for %s", address, activeNetParams.Name)
		sendReply(w, r, tmpl, testnetFaucetInformation, err)
		return
	}

	dcramount, err := dcrutil.NewAmount(amount)
	if err != nil {
		sendReply(w, r, tmpl, testnetFaucetInformation, err)
		return
	}

	resp, err := dcrwClient.SendFromMinConf("default", address, dcramount, 0)
	if err != nil {
		log.Errorf("error sending %v to %v for %v: %v",
			amount, address, hostIP, err)
		sendReply(w, r, tmpl, testnetFaucetInformation, err)
		return
	}

	testnetFaucetInformation.Success = resp.String()
	testnetFaucetInformation.SentToday = testnetFaucetInformation.SentToday + amount
	requestAmounts[time.Now().Unix()] = amount
	requestIPs[hostIP] = time.Now().Unix()
	log.Infof("successfully sent %v to %v for %v",
		amount, address, hostIP)
	updateBalance(dcrwClient)
	testnetFaucetInformation.TransactionLimit = transactionLimit

	sendReply(w, r, tmpl, testnetFaucetInformation, nil)
}

func calculateAmountSentToday() float64 {
	amountToday := float64(0)

	for then, amount := range requestAmounts {
		now := time.Now().Unix()
		if now-then >= 24*60*60 {
			continue
		}

		amountToday = amountToday + amount
	}

	return amountToday
}

func main() {
	// Load configuration and parse command line.  This function also
	// initializes logging and configures it accordingly.
	loadedCfg, _, err := loadConfig()
	if err != nil {
		return
	}

	cfg = loadedCfg

	quit := make(chan struct{})
	requestAmounts = make(map[int64]float64)
	requestIPs = make(map[string]int64)

	dcrwCerts, err := ioutil.ReadFile(cfg.WalletCert)
	if err != nil {
		log.Errorf("Failed to read dcrd cert file at %s: %s", cfg.WalletCert,
			err.Error())
		os.Exit(1)
	}
	log.Infof("Attempting to connect to dcrd RPC %s as user %s "+
		"using certificate located in %s",
		cfg.WalletHost, cfg.WalletUser, cfg.WalletCert)
	connCfgDaemon := &rpcclient.ConnConfig{
		Host:         cfg.WalletHost,
		Endpoint:     "ws",
		User:         cfg.WalletUser,
		Pass:         cfg.WalletPassword,
		Certificates: dcrwCerts,
		DisableTLS:   false,
	}
	dcrwClient, err = rpcclient.New(connCfgDaemon, nil)
	if err != nil {
		log.Errorf("Failed to start dcrd rpcclient: %s", err.Error())
		os.Exit(1)
	}
	updateBalance(dcrwClient)

	go func() {
		<-quit
		log.Info("Closing testnet demo.")
		dcrwClient.Disconnect()
		os.Exit(1)
	}()
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("public/js/"))))
	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("public/css/"))))
	http.Handle("/fonts/", http.StripPrefix("/fonts/", http.FileServer(http.Dir("public/fonts/"))))
	http.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("public/images/"))))
	http.HandleFunc("/", requestFunds)
	err = http.ListenAndServe(cfg.Listen, nil)
	if err != nil {
		log.Errorf("Failed to bind http server: %s", err.Error())
	}
}

func sendReply(w http.ResponseWriter, r *http.Request, tmpl *template.Template, info *testnetFaucetInfo, err error) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	jsonResp := &jsonResponse{}
	if err != nil {
		info.Error = err
		jsonResp.Error = err.Error()
		jsonResp.Txid = ""
	} else {
		jsonResp.Error = ""
		jsonResp.Txid = info.Success
	}
	json, _ := json.Marshal(jsonResp)

	// Return only raw JSON, if specified.
	if r.FormValue("json") != "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(json)
		return
	}

	w.Header().Set("X-Json-Reply", string(json))
	err = tmpl.Execute(w, info)
	if err != nil {
		panic(err)
	}
}

// Get the client's real IP address using the X-Real-IP header, or if that is
// empty, http.Request.RemoteAddr. See the sample nginx.conf for using the
// real_ip module to correctly set the X-Real-IP header.
func getClientIP(r *http.Request) (string, error) {
	xRealIP := r.Header.Get("X-Real-IP")
	if len(xRealIP) == 0 {
		log.Warn(`"X-Real-IP" header invalid, using RemoteAddr instead`)
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return "", err
		}
		return host, nil
	}

	return xRealIP, nil
}

func updateBalance(c *rpcclient.Client) {
	// calculate balance
	gbr, err := c.GetBalanceMinConf("default", 0)
	if err != nil {
		log.Warnf("unable to update balance: %v", err)
		return
	}

	spendable := float64(0)
	for _, v := range gbr.Balances {
		spendable = v.Spendable + spendable
	}

	log.Infof("updating balance from %v to %v", lastBalance, spendable)
	lastBalance = spendable
	transactionLimit = spendable / 100
	log.Infof("updating transaction limit to %v", transactionLimit)
}
