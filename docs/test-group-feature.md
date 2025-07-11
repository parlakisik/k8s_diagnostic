# Test Group Feature Documentation

## Overview

The `--test-group` feature provides a way to organize and run related diagnostic tests together. This feature allows users to execute predefined collections of tests with a single command, making the tool more flexible and user-friendly.

## Implementation Details

### Core Components

1. **Test Group Registry**

```go
// Test groups for logical organization
var testGroups = map[string][]string{
    "networking": {"pod-to-pod", "service-to-pod", "cross-node", "dns", "nodeport", "loadbalancer"},
    // Future groups will be added here, e.g.:
    // "firewall": {"ingress-policy", "egress-policy"},
    // "storage": {"pv-binding", "pvc-access"},
}
```

2. **Command Line Flag**

```go
testCmd.Flags().String("test-group", "", "run tests by group: networking (more groups coming soon)")
```

3. **Selection Logic**

```go
// Determine which tests to run
testsToRun := defaultTests

// Check for test group first
if testGroup != "" {
    if group, exists := testGroups[testGroup]; exists {
        testsToRun = group
        logger.LogInfo("Running tests in group: %s", testGroup)
    } else {
        fmt.Printf("WARNING: Unknown test group '%s' - using defaults\n", testGroup)
        logger.LogWarning("Unknown test group '%s' - using defaults", testGroup)
    }
} else if len(testList) > 0 {
    // Handle specific test list (omitted for brevity)
}
```

## Usage Examples

### Running the Networking Test Group

```bash
# Run all networking tests
./k8s-diagnostic test --test-group networking
```

### Error Handling

If an invalid group is specified, the tool falls back to the default tests:

```bash
# Invalid group name
./k8s-diagnostic test --test-group invalid
WARNING: Unknown test group 'invalid' - using defaults
```

## Test Group Definitions

Currently, the tool includes the following test groups:

| Group Name  | Tests Included                                                      | Description                         |
|-------------|---------------------------------------------------------------------|-------------------------------------|
| networking  | pod-to-pod, service-to-pod, cross-node, dns, nodeport, loadbalancer | Complete networking stack validation |

## Extending Test Groups

To add a new test group:

1. Add a new entry to the `testGroups` map in `cmd/test.go`:

```go
var testGroups = map[string][]string{
    "networking": {"pod-to-pod", "service-to-pod", "cross-node", "dns", "nodeport", "loadbalancer"},
    "storage": {"volume-mount", "pvc-binding", "storage-class"},
}
```

2. Update the command help text to include the new group

## Design Considerations

1. **Priority Ordering**: Test groups take precedence over individual test selection (`--test-list`) to provide a clear hierarchy of options.

2. **Graceful Fallback**: Invalid group names fall back to default tests rather than failing, enhancing usability.

3. **Forward Compatibility**: The structure supports future test groups without code changes (just map additions).

## Testing

The feature has been tested with:

1. Valid group names: `--test-group networking`
2. Invalid group names: `--test-group invalid`
3. Combination with other flags: `--test-group networking --verbose`
4. Custom namespace: `--test-group networking -n test-namespace`

## Future Enhancements

- Additional predefined groups (security, storage, scaling)
- User-defined groups via configuration file
- Improved metadata for test groups (descriptions, estimated runtime)
