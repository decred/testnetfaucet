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
	"strings"
	"time"

	"net"

	"github.com/decred/dcrd/chaincfg"
	"github.com/decred/dcrrpcclient"
	"github.com/decred/dcrutil"
)

// Settings for daemon
var dcrwCertPath = ("/home/user/.dcrwallet/rpc.cert")
var dcrwServer = "127.0.0.1:19110"
var dcrwUser = "USER"
var dcrwPass = "PASSWORD"

// Daemon Params to use
var activeNetParams = &chaincfg.TestNetParams
var dcrwClient *dcrrpcclient.Client

// Map of received IP requests for funds.
var requestedIps map[string]time.Time
var ipTimeoutValue = time.Duration(10 * time.Minute) // 10 minutes

// Webserver settings
var listeningPort = ":8001"

// Overall Data structure given to the template to render
type testnetFaucetInfo struct {
	BlockHeight int64
	Balance     int64
	Error       error
	Success     string
}

var funcMap = template.FuncMap{
	"minus": minus,
}

func minus(a, b int) int {
	return a - b
}

func requestFunds(w http.ResponseWriter, r *http.Request) {
	fp := filepath.Join("public/views", "design_sketch.html")
	tmpl, err := template.New("home").ParseFiles(fp)
	if err != nil {
		panic(err)
	}
	if r.Method == "GET" {
		err = tmpl.Execute(w, nil)
		if err != nil {
			panic(err)
		}
	} else {
		testnetFaucetInformation := &testnetFaucetInfo{}
		incomingIPaddress := getClientIP(r)
		hostIP, _, err := net.SplitHostPort(incomingIPaddress)
		if err != nil && incomingIPaddress != "127.0.0.1" {
			err = fmt.Errorf("Error when parsing incoming IP address, Please try again")
			testnetFaucetInformation.Error = err
			err = tmpl.Execute(w, testnetFaucetInformation)
			if err != nil {
				panic(err)
			}
			return
		} else if incomingIPaddress == "127.0.0.1" {
			hostIP = "127.0.0.1"
		}
		timeOut, ok := requestedIps[hostIP]
		if !ok {
			requestedIps[hostIP] = time.Now()
		} else {
			// If time saved in the requestedIps map is less than
			// ten minutes later than don't allow request
			if time.Now().Sub(timeOut) < ipTimeoutValue {
				err = fmt.Errorf("To ensure everyone has equal access to testnet "+
					"coins, we have a timeout per IP address of %s."+
					" Please try again shortly", ipTimeoutValue.String())
				testnetFaucetInformation.Error = err
				err = tmpl.Execute(w, testnetFaucetInformation)
				if err != nil {
					panic(err)
				}
				return
			}
		}
		r.ParseForm()
		fmt.Println("address:", r.Form["address"])
		addr, err := dcrutil.DecodeAddress(r.Form["address"][0], activeNetParams)
		if err != nil {
			testnetFaucetInformation.Error = err
		} else {
			resp, err := dcrwClient.SendToAddress(addr, 10000000000)
			if err != nil {
				testnetFaucetInformation.Error = err

			} else {
				testnetFaucetInformation.Success = fmt.Sprintf("Success! Txid: %s", resp.String())
			}
		}
		err = tmpl.Execute(w, testnetFaucetInformation)
		if err != nil {
			panic(err)
		}
	}
}

func main() {
	quit := make(chan struct{})

	requestedIps = make(map[string]time.Time)
	var dcrwCerts []byte
	dcrwCerts, err := ioutil.ReadFile(dcrwCertPath)
	if err != nil {
		fmt.Printf("Failed to read dcrd cert file at %s: %s\n", dcrwCertPath,
			err.Error())
		os.Exit(1)
	}
	fmt.Printf("Attempting to connect to dcrd RPC %s as user %s "+
		"using certificate located in %s\n",
		dcrwServer, dcrwUser, dcrwCertPath)
	connCfgDaemon := &dcrrpcclient.ConnConfig{
		Host:         dcrwServer,
		Endpoint:     "ws",
		User:         dcrwUser,
		Pass:         dcrwPass,
		Certificates: dcrwCerts,
		DisableTLS:   false,
	}
	dcrwClient, err = dcrrpcclient.New(connCfgDaemon, nil)
	if err != nil {
		fmt.Printf("Failed to start dcrd rpcclient: %s\n", err.Error())
		os.Exit(1)
	}

	go func() {
		for {
			select {
			case <-quit:
				close(quit)
				dcrwClient.Disconnect()
				fmt.Printf("\nClosing testnet demo.\n")
				os.Exit(1)
				break
			}
		}
	}()
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("public/js/"))))
	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("public/css/"))))
	http.Handle("/fonts/", http.StripPrefix("/fonts/", http.FileServer(http.Dir("public/fonts/"))))
	http.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("public/images/"))))
	http.HandleFunc("/", requestFunds)
	err = http.ListenAndServe(listeningPort, nil)
	if err != nil {
		fmt.Printf("Failed to bind http server: %s\n", err.Error())
	}
}

// Get the client's real IP address using the X-Real-IP header, or if that is
// empty, http.Request.RemoteAddr. See the sample nginx.conf for using the
// real_ip module to correctly set the X-Real-IP header.
func getClientIP(r *http.Request) string {
	getHost := func(ip string) string {
		if strings.Contains(ip, ":") {
			parts := strings.Split(ip, ":")
			return parts[0]
		}
		return ip
	}

	realIP := r.Header.Get("X-Real-IP")
	realIP = getHost(realIP)

	if realIP == "" {
		fmt.Println(`"X-Real-IP" header invalid, using RemoteAddr instead`)
		// If this somehow errors, just go with empty
		realIP = getHost(r.RemoteAddr)
	}

	return realIP
}
