package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type bootstrapContract struct {
	Schema    string `json:"schema"`
	Authority string `json:"authority"`
	Policy    struct {
		BootstrapCommit struct {
			Exactly   int    `json:"exactly"`
			Exception string `json:"exception"`
		} `json:"bootstrap_commit"`
		PostBootstrapDirectMain struct {
			Exactly        int    `json:"exactly"`
			ViolationStatus string `json:"violation_status"`
		} `json:"post_bootstrap_direct_main"`
		StatusPrecedence []string `json:"status_precedence"`
	} `json:"policy"`
}

type evidence struct {
	Schema                 string `json:"schema"`
	Authority              string `json:"authority"`
	Event                  string `json:"event"`
	BootstrapException     string `json:"bootstrap_exception"`
	PolicyVerifier         string `json:"policy_verifier"`
	TargetRepoWritesBefore string `json:"target_repo_writes_before_apply"`
}

func main() {
	contractPath := flag.String("contract", ".gooo/bootstrap.gooo", "path to the authoritative .gooo contract")
	outputPath := flag.String("output", "", "caller-owned evidence output path")
	event := flag.String("event", os.Getenv("GITHUB_EVENT_NAME"), "CI event name")
	flag.Parse()

	raw, err := os.ReadFile(*contractPath)
	if err != nil {
		fail(err)
	}
	var contract bootstrapContract
	if err := json.Unmarshal(raw, &contract); err != nil {
		fail(fmt.Errorf("parse .gooo contract: %w", err))
	}
	if contract.Schema != "gooo.bootstrap/v1" || contract.Authority != "metacode" {
		fail(errors.New(".gooo contract is not authoritative gooo.bootstrap/v1 metacode"))
	}
	if contract.Policy.BootstrapCommit.Exactly != 1 || contract.Policy.BootstrapCommit.Exception != "BOOTSTRAP_EXCEPTION" {
		fail(errors.New("bootstrap policy must allow exactly one BOOTSTRAP_EXCEPTION"))
	}
	if contract.Policy.PostBootstrapDirectMain.Exactly != 0 || contract.Policy.PostBootstrapDirectMain.ViolationStatus != "REFUTED" {
		fail(errors.New("post-bootstrap direct-main policy must be exactly zero and REFUTED"))
	}
	if len(contract.Policy.StatusPrecedence) != 3 || contract.Policy.StatusPrecedence[0] != "REFUTED" || contract.Policy.StatusPrecedence[1] != "UNKNOWN" || contract.Policy.StatusPrecedence[2] != "CLOSED" {
		fail(errors.New("status precedence must be REFUTED > UNKNOWN > CLOSED"))
	}
	if *event != "" && *event != "pull_request" && *event != "push" {
		fail(fmt.Errorf("unsupported CI event %q for PR-only verifier", *event))
	}
	if *outputPath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(*outputPath), 0o755); err != nil {
		fail(err)
	}
	out := evidence{
		Schema:                 "gooo.bootstrap/evidence/v1",
		Authority:              "metacode",
		Event:                  *event,
		BootstrapException:     "BOOTSTRAP_EXCEPTION",
		PolicyVerifier:         "PASS",
		TargetRepoWritesBefore: "0",
	}
	encoded, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(*outputPath, append(encoded, '\n'), 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
