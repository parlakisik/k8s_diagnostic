# TLS Server Name Indication (SNI) Policies

This directory contains examples of Cilium policies that control TLS traffic based on Server Name Indication (SNI).

## Policy Files

- **basic-sni-policy.yaml**: Controls TLS traffic to a single domain via SNI
- **multi-domain-sni-policy.yaml**: Controls TLS traffic to multiple domains via SNI
- **combined-l4-sni-policy.yaml**: Combines port restrictions with SNI filtering

## Purpose

These policies demonstrate how to:

1. Restrict TLS handshakes to specific domain names
2. Control outbound traffic to external services
3. Implement domain-based filtering for encrypted traffic
4. Combine SNI filtering with port restrictions

## Testing

These policies can be tested using the test script in the parent directory:

```bash
../test-l4-policies.sh tls-sni
```

## Prerequisites

**IMPORTANT:** TLS SNI policy enforcement requires the L7 proxy to be enabled in Cilium. Without the L7 proxy, SNI-based policies will not function correctly.

## How TLS SNI Works

Server Name Indication (SNI) is an extension of the TLS protocol that allows a client to specify the hostname or domain name during the TLS handshake. This is particularly useful when multiple websites are hosted on the same server with a shared IP address.

When a client initiates a connection to a server:
1. The client includes the desired hostname in the ClientHello message during the TLS handshake
2. The server uses this information to select the appropriate certificate
3. The appropriate TLS certificate is used for the secure connection

Cilium can filter these connections by inspecting the SNI field, allowing or blocking connections based on the domain name.
