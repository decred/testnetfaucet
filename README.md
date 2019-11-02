testnetfaucet
=============

[![Build Status](https://github.com/decred/testnetfaucet/workflows/Build%20and%20Test/badge.svg)](https://github.com/decred/testnetfaucet/actions)
[![ISC License](https://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)

## Overview

testnetfaucet is a simple web app that sends a configurable amount of testnet
Decred via an rpcclient connection to an instance of dcrwallet.

## Installation

## Developing

``` bash
git clone https://github.com/decred/testnetfaucet.git
cd testnetfaucet
dep ensure
go install
```

Start dcrwallet with the following options.  

```bash
dcrwallet --testnet -u USER -P PASSWORD --rpclisten=127.0.0.1:19111 --rpccert=$HOME/.dcrwallet/rpc.cert
```

Configure and start testnetfaucet

```bash
mkdir ~/.testnetfaucet
cp sample-testnetfaucet.conf ~/.testnetfaucet/testnetfaucet.conf (and edit appropriately)
testnetfaucet
```

## Contact

Check with the [community](https://decred.org/community/).

## Issue Tracker

The
[integrated github issue tracker](https://github.com/decred/testnetfaucet/issues)
is used for this project.

## License

testnetfaucet is licensed under the [copyfree](http://copyfree.org) ISC License.

