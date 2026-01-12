//go:build integration

package main

// ---
// Minimal stubs for integration test: always pass, never actually check/kills any instances/servers.
func checkForRunningInstances() error { return nil }
func killExistingBenchmarkServers()   {}

// ---

// Minimal stubs for integration test: always pass, never actually check/kills any instances/servers.
func checkForRunningInstances() error { return nil }
func killExistingBenchmarkServers()   {}

// ---
