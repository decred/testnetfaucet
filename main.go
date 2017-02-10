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

	"github.com/decred/dcrd/chaincfg"
	"github.com/decred/dcrrpcclient"
)

// Settings for daemon
var dcrwCertPath = ("/home/user/.dcrd/rpc.cert")
var dcrwServer = "127.0.0.1:19111"
var dcrwUser = "USER"
var dcrwPass = "PASSWORD"

// Daemon Params to use
var activeNetParams = &chaincfg.TestNetParams

// Webserver settings
var listeningPort = ":8001"

// Overall Data structure given to the template to render
type testnetFaucetInfo struct {
	BlockHeight int64
	Balance     int64
}

var testnetInformation = &testnetInfo{}

var funcMap = template.FuncMap{
	"minus": minus,
}

func minus(a, b int) int {
	return a - b
}

func demoPage(w http.ResponseWriter, r *http.Request) {

	fp := filepath.Join("public/views", "design_sketch.html")
	tmpl, err := template.New("home").Funcs(funcMap).ParseFiles(fp)
	if err != nil {
		panic(err)
	}
	err = tmpl.Execute(w, testnetInformation)
	if err != nil {
		panic(err)
	}

}
func updatetestnetInformation(dcrdClient *dcrrpcclient.Client) {
	fmt.Println("updating testnet information")
}

var mux map[string]func(http.ResponseWriter, *http.Request)

func main() {
	mux = make(map[string]func(http.ResponseWriter, *http.Request))
	mux["/"] = demoPage

	connectChan := make(chan int64, 100)
	quit := make(chan struct{})

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
	dcrwClient, err := dcrrpcclient.New(connCfgDaemon, nil)
	if err != nil {
		fmt.Printf("Failed to start dcrd rpcclient: %s\n", err.Error())
		os.Exit(1)
	}

	updatetestnetInformation(dcrwClient)
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
	http.HandleFunc("/", demoPage)
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("public/js/"))))
	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("public/css/"))))
	http.Handle("/fonts/", http.StripPrefix("/fonts/", http.FileServer(http.Dir("public/fonts/"))))
	http.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("public/images/"))))
	err = http.ListenAndServe(listeningPort, nil)
	if err != nil {
		fmt.Printf("Failed to bind http server: %s\n", err.Error())
	}
}
