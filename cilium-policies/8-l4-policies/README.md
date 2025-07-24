# Cilium L4 Network Policies

This directory contains examples and tests for Cilium Layer 4 (L4) network policies, which control traffic based on ports, protocols, and other L4 attributes.

## Categories (based on official Cilium documentation)

According to the [official Cilium documentation](https://docs.cilium.io/en/latest/network/kubernetes/policy/), L4 policies fall into three main categories:

1. **Limit ingress/egress ports** - Controls traffic based on port numbers and protocols
   - Found in: `basic-port-policies/`

2. **Limit ICMP/ICMPv6 types** - Controls ICMP traffic based on ICMP types
   - Found in: `icmp-policies/`

3. **Limit TLS Server Name Indication (SNI)** - Controls TLS traffic based on SNI
   - Found in: `tls-sni-policies/`

## Additional Categories

4. **HTTP/API Policies (L7)** - Advanced HTTP traffic filtering (technically L7, but built on L4)
   - Found in: `http-api-policies/`

5. **Advanced L4 Policies** - More complex policies combining various L4 features
   - Found in: `advanced-l4-policies/`

## Running Tests

To run all L4 policy tests with cleanup between test categories:

```bash
./test-l4-policies.sh
```

To run a specific test category:

```bash
./test-l4-policies.sh [category]
```

Available test categories:
- basic-ports
- icmp
- tls-sni
- http-api
- advanced-l4
- all (default)
