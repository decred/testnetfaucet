// Copyright (c) 2017 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"net"

	"github.com/decred/dcrd/dcrutil"
	"github.com/decred/dcrd/rpcclient"
)

var (
	cfg *config
)

// Daemon Params to use
var dcrwClient *rpcclient.Client

// Map of received IP requests for funds.
var requestIPs map[string]int64

// Overall Data structure given to the template to render
type testnetFaucetInfo struct {
	Address     string
	Amount      float64
	BlockHeight int64
	Balance     float64
	Limit       int64
	Error       error
	Success     string
}

func requestFunds(w http.ResponseWriter, r *http.Request) {
	fp := filepath.Join("public/views", "design_sketch.html")
	testnetFaucetInformation := &testnetFaucetInfo{
		Address: cfg.WalletAddress,
		Amount:  cfg.WithdrawalAmount,
		Limit:   cfg.WithdrawalTimeLimit,
	}

	tmpl, err := template.New("home").ParseFiles(fp)

	if err != nil {
		panic(err)
	}
	if r.Method == "GET" {
		err = tmpl.Execute(w, testnetFaucetInformation)
		if err != nil {
			panic(err)
		}
	} else {
		// calculate balance
		gbr, err := dcrwClient.GetBalance("default")
		if err != nil {
			testnetFaucetInformation.Error = err
			err = tmpl.Execute(w, testnetFaucetInformation)
			if err != nil {
				panic(err)
			}
			return
		}

		spendable := float64(0)
		for _, v := range gbr.Balances {
			spendable = v.Spendable + spendable
		}
		testnetFaucetInformation.Balance = spendable

		r.ParseForm()
		address := r.FormValue("address")
		overrideToken := r.FormValue("overrideToken")

		// enforce ratelimit unless overridetoken was specified and matches
		hostIP, err := getClientIP(r)
		if err != nil {
			panic(err)
		}

		if overrideToken != cfg.OverrideToken {
			lastRequestTime, found := requestIPs[hostIP]
			if found {
				nextAllowedRequest := lastRequestTime + cfg.WithdrawalTimeLimit
				coolDownTime := nextAllowedRequest - time.Now().Unix()

				if coolDownTime >= 0 {
					err = fmt.Errorf("You may only withdraw %v DCR every "+
						"%v seconds.  Please wait another %v seconds.",
						cfg.WithdrawalAmount, cfg.WithdrawalTimeLimit, coolDownTime)
					testnetFaucetInformation.Error = err
					err = tmpl.Execute(w, testnetFaucetInformation)
					if err != nil {
						panic(err)
					}
					return
				}
			}
		}

		// Try to send the tx if we can.
		addr, err := dcrutil.DecodeAddress(address)
		if err != nil {
			testnetFaucetInformation.Error = err
		} else if addr.IsForNet(activeNetParams.Params) {
			amount, err := dcrutil.NewAmount(cfg.WithdrawalAmount)
			if err != nil {
				testnetFaucetInformation.Error = err
				err = tmpl.Execute(w, testnetFaucetInformation)
				if err != nil {
					panic(err)
				}
			}
			resp, err := dcrwClient.SendToAddress(addr, amount)
			if err != nil {
				log.Errorf("error sending %v to %v for %v: %v",
					amount, addr, hostIP, err)
				testnetFaucetInformation.Error = err
			} else {
				testnetFaucetInformation.Success = resp.String()
				requestIPs[hostIP] = time.Now().Unix()
				log.Infof("successfully sent %v to %v for %v",
					amount, addr, hostIP)
			}
		} else {
			testnetFaucetInformation.Error = fmt.Errorf("address "+
				"%s is not for %s", addr, activeNetParams.Name)
		}
		err = tmpl.Execute(w, testnetFaucetInformation)
		if err != nil {
			panic(err)
		}
	}
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
	requestIPs = make(map[string]int64)

	dcrwCerts, err := ioutil.ReadFile(cfg.WalletCert)
	if err != nil {
		log.Errorf("Failed to read dcrd cert file at %s: %s\n", cfg.WalletCert,
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
		log.Errorf("Failed to start dcrd rpcclient: %s\n", err.Error())
		os.Exit(1)
	}

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
		log.Errorf("Failed to bind http server: %s\n", err.Error())
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
