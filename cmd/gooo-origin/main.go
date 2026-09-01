package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/kimjooyoon/gooo-hygienic-origin-resolver/internal/originresolver"
)

func main() {
	if len(os.Args) < 2 {
		fail(errors.New("command is required: resolve or emit"))
	}
	switch os.Args[1] {
	case "resolve":
		runResolve(os.Args[2:])
	case "emit":
		runEmit(os.Args[2:])
	default:
		fail(fmt.Errorf("unknown command %q", os.Args[1]))
	}
}

func runResolve(args []string) {
	flags := flag.NewFlagSet("resolve", flag.ContinueOnError)
	contractPath := flags.String("contract", ".gooo/origin-resolver.gooo", "authoritative .gooo contract")
	scenarioID := flags.String("scenario", "", "scenario id; empty resolves all scenarios")
	outputPath := flags.String("output", "", "caller-owned report output")
	repoRoot := flags.String("repo-root", ".", "target repository root")
	if err := flags.Parse(args); err != nil {
		fail(err)
	}
	spec, raw, err := originresolver.LoadSpec(*contractPath)
	if err != nil {
		fail(err)
	}
	var value any
	if *scenarioID == "" {
		all, resolveErr := originresolver.ResolveAll(spec)
		if resolveErr != nil {
			fail(resolveErr)
		}
		all.ContractDigest = digest(raw)
		value = all
	} else {
		report, resolveErr := originresolver.ResolveScenario(spec, *scenarioID)
		if resolveErr != nil {
			fail(resolveErr)
		}
		report.ContractDigest = digest(raw)
		value = report
	}
	encoded, err := originresolver.EncodeReport(value)
	if err != nil {
		fail(err)
	}
	if err := originresolver.WriteCallerFile(*outputPath, *repoRoot, append(encoded, '\n')); err != nil {
		fail(err)
	}
}

func runEmit(args []string) {
	flags := flag.NewFlagSet("emit", flag.ContinueOnError)
	contractPath := flags.String("contract", ".gooo/origin-resolver.gooo", "authoritative .gooo contract")
	scenarioID := flags.String("scenario", "normal-nested-expansion", "capture-free example scenario")
	outputPath := flags.String("output", "", "caller-owned generated Go output")
	repoRoot := flags.String("repo-root", ".", "target repository root")
	if err := flags.Parse(args); err != nil {
		fail(err)
	}
	spec, _, err := originresolver.LoadSpec(*contractPath)
	if err != nil {
		fail(err)
	}
	source, err := originresolver.EmitExample(spec, *scenarioID)
	if err != nil {
		fail(err)
	}
	if err := originresolver.WriteCallerFile(*outputPath, *repoRoot, source); err != nil {
		fail(err)
	}
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
