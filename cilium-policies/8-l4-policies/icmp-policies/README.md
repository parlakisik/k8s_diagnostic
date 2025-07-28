# ICMP/ICMPv6 Type Policies

This directory contains examples of Cilium policies that control ICMP/ICMPv6 traffic based on ICMP types.

## Policy Files

- **icmp-type-policy.yaml**: Controls ICMP traffic by type number
- **icmpv6-type-policy.yaml**: Controls ICMPv6 traffic by named type (using CamelCase names)
- **mixed-icmp-policy.yaml**: Controls both ICMP and ICMPv6 traffic in the same policy

## Purpose

These policies demonstrate how to:

1. Control ICMP traffic based on specific ICMP types
2. Specify ICMP types using both numeric values and CamelCase names
3. Allow specific ICMP operations while blocking others
4. Combine ICMP rules with other L4 port rules

## Testing

These policies can be tested using the test script in the parent directory:

```bash
../test-l4-policies.sh icmp
```

## Supported ICMP Types

### IPv4 ICMP Types
- EchoReply
- DestinationUnreachable
- Redirect
- Echo / EchoRequest
- RouterAdvertisement
- RouterSelection
- TimeExceeded
- ParameterProblem
- Timestamp
- TimestampReply
- Photuris
- ExtendedEchoRequest
- ExtendedEchoReply

### IPv6 ICMP Types
- DestinationUnreachable
- PacketTooBig
- TimeExceeded
- ParameterProblem
- EchoRequest
- EchoReply
- MulticastListenerQuery
- MulticastListenerReport
- MulticastListenerDone
- RouterSolicitation
- RouterAdvertisement
- NeighborSolicitation
- NeighborAdvertisement
- RedirectMessage
- RouterRenumbering
- And many others documented in the Cilium API
