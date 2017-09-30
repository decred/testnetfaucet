# testnetfaucet

testnetfaucet is a simple web app that connects to dcrd and displays
information about the tesnet hardfork voting.

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

Start testnetfaucet

```bash
testnetfaucet
```

## Contact

If you have any further questions you can find us at:

- irc.freenode.net (channel #decred)
- [webchat](https://webchat.freenode.net/?channels=decred)
- forum.decred.org
- decred.slack.com

## Issue Tracker

The
[integrated github issue tracker](https://github.com/decred/testnetfaucet/issues)
is used for this project.

## License

testnetfaucet is licensed under the [copyfree](http://copyfree.org) ISC License.

