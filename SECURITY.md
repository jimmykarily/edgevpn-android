# Security policy

## Reporting a problem

Please report security problems privately, through GitHub's private vulnerability reporting: open the repository's **Security** tab and choose **Report a vulnerability**. Do not open a public issue or pull request for them.

A useful report says what an attacker can do, what they need for it (for example the network key, a device on the same Wi-Fi, or another app on the phone), and how to reproduce it. Never include a real network key in a report; generate a throwaway one with `edgevpn -g -b`.

This is a project maintained in spare time. Expect a first answer within two weeks.

## Scope

In scope: this app and its Go library in `golib/`, including how it stores the network key and the node key, what it writes to its logs, and which traffic goes through the VPN.

Problems in edgevpn itself (the ledger, discovery, the network protocol) belong to the [edgevpn project](https://github.com/mudler/edgevpn), and problems in libp2p to [go-libp2p](https://github.com/libp2p/go-libp2p). If you are not sure where a problem belongs, report it here.

## What the network key protects

Anyone who has a network's key is a full member of that network: they can join it, see its devices and send them traffic. Keep the key as secret as a password, and generate a new one if it leaks. Every device then needs the new key.

## Supported versions

Only the latest release gets fixes.
