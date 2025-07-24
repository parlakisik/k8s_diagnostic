# Layer 7 Cilium Network Policies

This directory contains examples and tests for Cilium's Layer 7 (application layer) network policies. L7 policies allow for fine-grained control over network traffic based on application-specific attributes such as HTTP paths, methods, headers, DNS queries, and more.

## Policy Categories

The L7 policies are organized into the following categories:

### 1. HTTP Policies
Located in the `http-policies` directory, these policies demonstrate how to:
- Filter HTTP requests based on paths (`/path1`, `/public`, etc.)
- Control access based on HTTP methods (GET, POST, PUT, etc.)
- Require specific HTTP headers to be present
- Combine multiple HTTP rules

### 2. DNS Policies
Located in the `dns-policies` directory, these policies demonstrate:
- Controlling DNS queries with exact name matching (`matchName`)
- Pattern-based filtering with wildcards (`matchPattern`)
- Using DNS lookups to populate IP allow-lists (`toFQDNs`)

### 3. Deny Policies
Located in the `deny-policies` directory, these examples show:
- How deny policies take precedence over allow policies
- Combining allow and deny rules for complex policy requirements

## Important Notes on L7 Policy Behavior

1. **Proxy Requirement**: L7 policies cause traffic to be proxied through a node-local Envoy instance, which may impact performance and availability.

2. **Violation Handling**: Unlike L3/L4 policies, L7 policy violations do not result in packet drops; instead, application-specific rejection responses are returned (e.g., HTTP 403, DNS REFUSED).

3. **Port Range Support**: L7 rules support port ranges, except for DNS rules.

4. **Host Policy Limitations**: In host policies (using Node Selector), only DNS L7 rules are currently supported.

5. **IPv6 Requirements**: L7 policies for SNATed IPv6 traffic require specific kernel versions (see limitations section).

6. **Default Deny Behavior**: `EnableDefaultDeny` does not apply to L7 rules.

## Test Script Usage

The `test-l7-policies.sh` script tests various L7 policy types:

```bash
./test-l7-policies.sh [subtest-name]
```

Available subtests organized by category:

### DNS Policy Tests
- `dns-matchname` - Test DNS matchName policy (exact matching)
- `dns-matchpattern` - Test DNS matchPattern policy (wildcard matching)
- `dns-fqdn` - Test DNS FQDN policy with IP discovery
- `dns` - Run all DNS policy tests

### Deny Policy Tests
- `deny-ingress` - Test deny ingress policy
- `deny-clusterwide` - Test clusterwide deny policy
- `deny` - Run all deny policy tests

### Other Options
- `categories` - Test all categories with cleanup between each (default)
- `cleanup` - Only clean up the test environment
- `check-dns-config` - Check DNS proxy configuration in Cilium
- `fix-dns-config` - Fix DNS proxy configuration in Cilium
- `list` - List all available subtests
- `help` - Show usage information

Examples:
```bash
./test-l7-policies.sh dns
./test-l7-policies.sh deny-ingress
./test-l7-policies.sh categories
```

### DNS Configuration

For DNS policies to work correctly, the Cilium DNS proxy must be enabled:

```bash
# Check current DNS proxy configuration
./test-l7-policies.sh check-dns-config

# Enable DNS proxy if needed
./test-l7-policies.sh fix-dns-config
```

## Limitations

- L7 policies are dependent on the Cilium agent pod when Envoy is embedded.
- L7 policies for SNATed IPv6 traffic require kernel versions: 6.14.1, 6.12.22, 6.6.86, 6.1.133, 5.15.180, 5.10.236, 5.4.292.
- EnableDefaultDeny does not apply to layer-7 rules.
- DNS policies have specific requirements regarding cilium-agent configuration.

## Testing Notes

When running these tests, you may see failures in tests where traffic should be allowed. This is a known issue with Cilium in many environments, where policies can be enforced more strictly than documented. The tests are structured correctly according to Cilium documentation, but your environment may have different policy enforcement behavior.
