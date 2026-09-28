# ltcd

[![Build Status](https://github.com/ltcsuite/ltcd/workflows/Build%20and%20Test/badge.svg)](https://github.com/ltcsuite/ltcd/actions)
[![Coverage Status](https://coveralls.io/repos/github/ltcsuite/ltcd/badge.svg?branch=master)](https://coveralls.io/github/ltcsuite/ltcd?branch=master)
[![ISC License](https://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://pkg.go.dev/github.com/ltcsuite/ltcd)

### Table of Contents

1. [About](#About)
2. [Getting Started](#GettingStarted)
   1. [Installation](#Installation)
      1. [Windows](#WindowsInstallation)
      2. [Linux/BSD/MacOSX/POSIX](#PosixInstallation)
   2. [Configuration](#Configuration)
   3. [Controlling and Querying ltcd via ltcctl](#LtcctlConfig)
   4. [Mining](#Mining)
3. [Help](#Help)
   1. [Startup](#Startup)
      1. [Using bootstrap.dat](#BootstrapDat)
   2. [Network Configuration](#NetworkConfig)
   3. [Wallet](#Wallet)
4. [Contact](#Contact)
   1. [Email](#ContactEmail)
5. [Developer Resources](#DeveloperResources)
   1. [Code Contribution Guidelines](#ContributionGuidelines)
   2. [JSON-RPC Reference](#JSONRPCReference)
   3. [The ltcsuite Litecoin-related Go Packages](#GoPackages)
   4. [Litecoin functional changes](#LtcdDifferences)
   5. [Diffing ltcd and btcd codebases](#Diffing)

<a name="About" />

### 1. About

ltcd is a full node litecoin implementation written in [Go](http://go.dev),
licensed under the [copyfree](http://www.copyfree.org) ISC License.

This project is currently under active development and is in a Beta state. It
is extremely stable and has been in production use since prior December 2018.

It properly downloads, validates, and serves the block chain using the exact
rules (including consensus bugs) for block acceptance as Litecoin Core. We have
taken great care to avoid ltcd causing a fork to the block chain. It includes a
full block validation testing framework which contains all of the 'official'
block acceptance tests (and some additional ones) that is run on every pull
request to help ensure it properly follows consensus. Also, it passes all of
the JSON test data in the Litecoin Core code.

It also properly relays newly mined blocks, maintains a transaction pool, and
relays individual transactions that have not yet made it into a block. It
ensures all individual transactions admitted to the pool follow the rules
required by the block chain and also includes more strict checks which filter
transactions based on miner requirements ("standard" transactions).

One key difference between ltcd and Litecoin Core is that ltcd does _NOT_ include
wallet functionality and this was a very intentional design decision. See the
blog entry [here](https://web.archive.org/web/20171125143919/https://blog.conformal.com/btcd-not-your-moms-bitcoin-daemon)
for more details. This means you can't actually make or receive payments
directly with ltcd. That functionality is provided by the
[ltcwallet](https://github.com/ltcsuite/ltcwallet) which is under active development.

<a name="GettingStarted" />

[Go](http://golang.org) 1.17 or newer.

<a name="Installation" />

**2.1 Installation**

The first step is to install ltcd. See one of the following sections for
details on how to install on the supported operating systems.

<a name="WindowsInstallation" />

**2.1.1 Windows Installation**<br />

- Install the MSI available at: https://github.com/ltcsuite/ltcd/releases
- Launch ltcd from the Start Menu

<a name="PosixInstallation" />

**2.1.2 Linux/BSD/MacOSX/POSIX Installation**

- Install Go according to the installation instructions here:
  http://go.dev/doc/install

- Ensure Go was installed properly and is a supported version:

```bash
$ go version
$ go env GOROOT GOPATH
```

NOTE: The `GOROOT` and `GOPATH` above must not be the same path. It is
recommended that `GOPATH` is set to a directory in your home directory such as
`~/goprojects` to avoid write permission issues. It is also recommended to add
`$GOPATH/bin` to your `PATH` at this point.

- Run the following commands to obtain ltcd, all dependencies, and install it:

```bash
$ git clone https://github.com/ltcsuite/ltcd
$ cd ltcd
$ GO111MODULE=on go install -v . ./cmd/...
```

- ltcd (and utilities) will now be installed in `$GOPATH/bin`. If you did
  not already add the bin directory to your system path during Go installation,
  we recommend you do so now.

**Updating**

- Run the following commands to update ltcd, all dependencies, and install it:

```bash
$ cd ltcd
$ git pull
$ GO111MODULE=on go install -v . ./cmd/...
```

<a name="Configuration" />

**2.2 Configuration**

ltcd has a number of [configuration](http://godoc.org/github.com/ltcsuite/ltcd)
options, which can be viewed by running: `$ ltcd --help`.

<a name="ltcctlConfig" />

**2.3 Controlling and Querying ltcd via ltcctl**

ltcctl is a command line utility that can be used to both control and query ltcd
via [RPC](http://www.wikipedia.org/wiki/Remote_procedure_call). ltcd does
**not** enable its RPC server by default; You must configure at minimum both an
RPC username and password or both an RPC limited username and password:

- ltcd.conf configuration file

```
[Application Options]
rpcuser=myuser
rpcpass=SomeDecentp4ssw0rd
rpclimituser=mylimituser
rpclimitpass=Limitedp4ssw0rd
```

- ltcctl.conf configuration file

```
[Application Options]
rpcuser=myuser
rpcpass=SomeDecentp4ssw0rd
```

OR

```
[Application Options]
rpclimituser=mylimituser
rpclimitpass=Limitedp4ssw0rd
```

For a list of available options, run: `$ ltcctl --help`

<a name="Mining" />

**2.4 Mining**

ltcd supports the `getblocktemplate` RPC.
The limited user cannot access this RPC.

**1. Add the payment addresses with the `miningaddr` option.**

```
[Application Options]
rpcuser=myuser
rpcpass=SomeDecentp4ssw0rd
miningaddr=12c6DSiU4Rq3P4ZxziKxzrL5LmMBrzjrJX
miningaddr=1M83ju3EChKYyysmM2FXtLNftbacagd8FR
```

**2. Add ltcd's RPC TLS certificate to system Certificate Authority list.**

`cgminer` uses [curl](http://curl.haxx.se/) to fetch data from the RPC server.
Since curl validates the certificate by default, we must install the `ltcd` RPC
certificate into the default system Certificate Authority list.

**Ubuntu**

1. Copy rpc.cert to /usr/share/ca-certificates: `# cp /home/user/.ltcd/rpc.cert /usr/share/ca-certificates/ltcd.crt`
2. Add ltcd.crt to /etc/ca-certificates.conf: `# echo ltcd.crt >> /etc/ca-certificates.conf`
3. Update the CA certificate list: `# update-ca-certificates`

**3. Set your mining software url to use https.**

`$ cgminer -o https://127.0.0.1:9334 -u rpcuser -p rpcpassword`

<a name="Help" />

### 3. Help

<a name="Startup" />

**3.1 Startup**

Typically ltcd will run and start downloading the block chain with no extra
configuration necessary, however, there is an optional method to use a
`bootstrap.dat` file that may speed up the initial block chain download process.

<a name="BootstrapDat" />

**3.1.1 bootstrap.dat**

- [Using bootstrap.dat](https://github.com/ltcsuite/ltcd/tree/master/docs/configuration.md#using-bootstrapdat)

<a name="NetworkConfig" />

**3.1.2 Network Configuration**

- [What Ports Are Used by Default?](https://github.com/ltcsuite/ltcd/tree/master/docs/default_ports.md)
- [How To Listen on Specific Interfaces](https://github.com/ltcsuite/ltcd/tree/master/docs/configure_peer_server_listen_interfaces.md)
- [How To Configure RPC Server to Listen on Specific Interfaces](https://github.com/ltcsuite/ltcd/tree/master/docs/configure_rpc_server_listen_interfaces.md)
- [Configuring ltcd with Tor](https://github.com/ltcsuite/ltcd/tree/master/docs/configuring_tor.md)

<a name="Wallet" />

**3.1 Wallet**

ltcd was intentionally developed without an integrated wallet for security
reasons. Please see [ltcwallet](https://github.com/ltcsuite/ltcwallet) for more
information.

<a name="Contact" />

### 4. Contact

<a name="ContactEmail" />

**4.1 Email**

- losh11@litecoin.net - maintainers email.

<a name="DeveloperResources" />

### 5. Developer Resources

<a name="ContributionGuidelines" />

- [Code Contribution Guidelines](https://github.com/ltcsuite/ltcd/tree/master/docs/code_contribution_guidelines.md)

<a name="JSONRPCReference" />

- [JSON-RPC Reference](https://github.com/ltcsuite/ltcd/tree/master/docs/json_rpc_api.md)
  - [RPC Examples](https://github.com/ltcsuite/ltcd/tree/master/docs/json_rpc_api.md#ExampleCode)

<a name="GoPackages" />

- The ltcsuite Litecoin-related Go Packages:
  - [btcrpcclient](https://github.com/ltcsuite/ltcd/tree/master/rpcclient) - Implements a
    robust and easy to use Websocket-enabled Litecoin JSON-RPC client
  - [btcjson](https://github.com/ltcsuite/ltcd/tree/master/btcjson) - Provides an extensive API
    for the underlying JSON-RPC command and return values
  - [wire](https://github.com/ltcsuite/ltcd/tree/master/wire) - Implements the
    Litecoin wire protocol
  - [peer](https://github.com/ltcsuite/ltcd/tree/master/peer) -
    Provides a common base for creating and managing Litecoin network peers.
  - [blockchain](https://github.com/ltcsuite/ltcd/tree/master/blockchain) -
    Implements Litecoin block handling and chain selection rules
  - [blockchain/fullblocktests](https://github.com/ltcsuite/ltcd/tree/master/blockchain/fullblocktests) -
    Provides a set of block tests for testing the consensus validation rules
  - [txscript](https://github.com/ltcsuite/ltcd/tree/master/txscript) -
    Implements the Litecoin transaction scripting language
  - [btcec](https://github.com/ltcsuite/ltcd/tree/master/btcec) - Implements
    support for the elliptic curve cryptographic functions needed for the
    Litecoin scripts
  - [database](https://github.com/ltcsuite/ltcd/tree/master/database) -
    Provides a database interface for the Litecoin block chain
  - [mempool](https://github.com/ltcsuite/ltcd/tree/master/mempool) -
    Package mempool provides a policy-enforced pool of unmined litecoin
    transactions.
  - [ltcutil](https://github.com/ltcsuite/ltcd/ltcutil) - Provides Litecoin-specific
    convenience functions and types
  - [chainhash](https://github.com/ltcsuite/ltcd/tree/master/chaincfg/chainhash) -
    Provides a generic hash type and associated functions that allows the
    specific hash algorithm to be abstracted.
  - [connmgr](https://github.com/ltcsuite/ltcd/tree/master/connmgr) -
    Package connmgr implements a generic Litecoin network connection manager.

<a name="LtcdDifferences" />

- Litecoin functional changes:
  - btcd's litecoin mode often has untested, and unknown errors due to injection implementation. ltcd removed custom parameter injection, and replaces litecoin network constants as default, with several tests.
  - `wire/blockheader.go` - declare PowHash method (for scrypt) used instead of BlockHash
  - `blockchain/difficulty.go` - blocksPerRetarget change for Zeitgeist2 attack
  - `blockchain/validate.go` - use PowHash (for scrypt) used instead of BlockHash
  - `mempool/policy.go` - adjust minDustFee (litecoin v0.21 rate)
  - `mining/cpuminer/cpu_miner.go` - use PowHash (for scrypt) used instead of BlockHash
  - `wire/protocol.go` - ProtocolVersion uses 70016; MainNet magic number is different; TestNet4 is used instead of TestNet3; Remove Bitcoin softfork service flags & replace with `NODE_NETWORK_LIMITED` and `NODE_MWEB` flags.
  - `example_test.go` - litecoin address used instead of bitcoin address
  - `wire/msgtx.go` - litecoin MWEB txs are fully deserialsised and consumed.
  - `wire/msgblock.go` - litecoin MWEB blocks are fully parsed
  - `ltcutil/address.go` - fix address decoding, `ltc1` and `L` prefixes decoded correctly
  - `ltcutil/amount.go` - use Math.round() for rounding amounts - fixing float64 overflows
  - `rpcserver.go` - signed message header is `"Litecoin Signed Message:\n"`.

<a name="Diffing" />

- Diffing ltcd and btcd codebases
  - ltcd and the ltcsuite group of projects are a fork of btcd and btcsuite.
    A lot of the changes involve simple renaming of 'btc' to 'ltc'.  
     If you wish to diff the two projects (ltcd/btcd) without being swamped,
    the following grep commands will revert the naming changes when run in the ltcd directory.
    (There are a few exceptions to these rules, so this kind of change should only be used as an aid to diffing).
    find -name '_.go' -exec sed -i 's/ltcd/btcd/g' {} +
    find -name '_.go' -exec sed -i 's/ltcsuite/btcsuite/g' {} +
    find -name '\*.go' -exec sed -i 's/ltcutil/btcutil/g' {} +
