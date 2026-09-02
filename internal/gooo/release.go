package gooo

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"crypto/sha256"
	"encoding/hex"
)

type ReleaseGuardReport struct {
	Schema            string          `json:"schema"`
	Repository        string          `json:"repository"`
	PreviousReleaseID int64           `json:"previous_release_id"`
	PreviousTag       string          `json:"previous_tag"`
	NextTag           string          `json:"next_tag"`
	BurnedTags        []string        `json:"burned_tags,omitempty"`
	MainRunID         int64           `json:"main_run_id"`
	Status            Status          `json:"status"`
	Checks            []GuardCheck    `json:"checks"`
	Unknown           *UnknownRecord  `json:"unknown,omitempty"`
}

type GuardCheck struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type ReleaseLock struct {
	Schema     string             `json:"schema"`
	Immutable  bool               `json:"immutable"`
	Repository string             `json:"repository"`
	Tag        ReleaseLockTag     `json:"tag"`
	Actions    ReleaseLockActions `json:"actions"`
	Assets     []ReleaseLockAsset `json:"assets"`
}

type ReleaseLockTag struct {
	Name           string `json:"name"`
	Annotated      bool   `json:"annotated"`
	TagObjectSHA   string `json:"tag_object_sha"`
	TargetCommitSHA string `json:"target_commit_sha"`
}

type ReleaseLockActions struct {
	RunID          int64  `json:"run_id"`
	URL            string `json:"url"`
	Conclusion     string `json:"conclusion"`
	EvidenceArtifact string `json:"evidence_artifact"`
}

type ReleaseLockAsset struct {
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

func BuildReleaseLock(repo, tag, tagObjectSHA, targetCommitSHA, evidenceArtifact string, runID int64, sourcePath, evidencePath, sumsPath string) (ReleaseLock, error) {
	if repo == "" || tag == "" || tagObjectSHA == "" || targetCommitSHA == "" || evidenceArtifact == "" || runID < 1 {
		return ReleaseLock{}, errors.New("release lock requires repository, tag lineage, evidence artifact, and run id")
	}
	assets := make([]ReleaseLockAsset, 0, 3)
	for _, path := range []string{sourcePath, evidencePath, sumsPath} {
		asset, err := releaseLockAsset(path)
		if err != nil {
			return ReleaseLock{}, err
		}
		assets = append(assets, asset)
	}
	return ReleaseLock{
		Schema: "gooo.release-lock/v1", Immutable: true, Repository: repo,
		Tag: ReleaseLockTag{Name: tag, Annotated: true, TagObjectSHA: tagObjectSHA, TargetCommitSHA: targetCommitSHA},
		Actions: ReleaseLockActions{RunID: runID, URL: fmt.Sprintf("https://github.com/%s/actions/runs/%d", repo, runID), Conclusion: "success", EvidenceArtifact: evidenceArtifact},
		Assets: assets,
	}, nil
}

func releaseLockAsset(path string) (ReleaseLockAsset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ReleaseLockAsset{}, err
	}
	sum := sha256.Sum256(data)
	return ReleaseLockAsset{Name: filepath.Base(path), SizeBytes: int64(len(data)), SHA256: "sha256:" + hex.EncodeToString(sum[:])}, nil
}

type releaseMetadata struct {
	ID              int64  `json:"id"`
	TagName         string `json:"tag_name"`
	Draft           bool   `json:"draft"`
	Prerelease      bool   `json:"prerelease"`
	Immutable       bool   `json:"immutable"`
	TargetCommitish string `json:"target_commitish"`
	Assets          []struct {
		Name      string `json:"name"`
		Size      int64  `json:"size"`
		Digest    string `json:"digest"`
		BrowserURL string `json:"browser_download_url"`
	} `json:"assets"`
}

type releaseLock struct {
	Schema     string `json:"schema"`
	Immutable  bool   `json:"immutable"`
	Repository string `json:"repository"`
	Tag        struct {
		Name           string `json:"name"`
		Annotated      bool   `json:"annotated"`
		TagObjectSHA   string `json:"tag_object_sha"`
		TargetCommitSHA string `json:"target_commit_sha"`
	} `json:"tag"`
	Actions struct {
		RunID       int64  `json:"run_id"`
		Conclusion  string `json:"conclusion"`
		Evidence    string `json:"evidence_artifact"`
	} `json:"actions"`
	Assets []struct {
		Name      string `json:"name"`
		SizeBytes int64  `json:"size_bytes"`
		SHA256    string `json:"sha256"`
	} `json:"assets"`
}

type workflowRunMetadata struct {
	ID         int64  `json:"id"`
	HeadSHA    string `json:"head_sha"`
	Conclusion string `json:"conclusion"`
	Event      string `json:"event"`
}

type gitRefMetadata struct {
	Object struct {
		Type string `json:"type"`
		SHA  string `json:"sha"`
	} `json:"object"`
}

type annotatedTagMetadata struct {
	Object struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

func VerifyReleaseLineage(ctx context.Context, repo string, previousReleaseID, mainRunID int64, nextTag, currentSHA, contractAuthority, contractSchema string, burnedTags []string) (ReleaseGuardReport, error) {
	guard := ReleaseGuardReport{
		Schema:            "gooo.release-guard/v1",
		Repository:        repo,
		PreviousReleaseID: previousReleaseID,
		NextTag:           nextTag,
		BurnedTags:        append([]string{}, burnedTags...),
		MainRunID:         mainRunID,
		Status:            StatusUnknown,
		Checks:            make([]GuardCheck, 0, 12),
	}
	if repo == "" || previousReleaseID < 1 || mainRunID < 1 || nextTag == "" || currentSHA == "" {
		return guard, errors.New("release guard requires repository, release id, main run id, next tag, and current SHA")
	}
	if contractAuthority != "metacode" || contractSchema != "gooo.origin-resolver/v2" {
		return guard, errors.New("release guard requires authoritative v2 metacode without aggregate reporting")
	}
	observer := githubObserver{baseURL: defaultGitHubAPI, token: os.Getenv("GITHUB_TOKEN"), client: http.DefaultClient}
	if observer.token == "" {
		return guard, errors.New("release guard requires GITHUB_TOKEN")
	}
	var release releaseMetadata
	if err := observer.get(ctx, fmt.Sprintf("/repos/%s/releases/%d", repo, previousReleaseID), &release); err != nil {
		return guard, err
	}
	guard.PreviousTag = release.TagName
	addGuardCheck(&guard, "previous-release-immutable", release.Immutable && !release.Draft && !release.Prerelease, fmt.Sprintf("tag=%s immutable=%t draft=%t prerelease=%t", release.TagName, release.Immutable, release.Draft, release.Prerelease))
	if !release.Immutable || release.Draft || release.Prerelease {
		return guard, errors.New("previous release is not a published immutable release")
	}
	lockAsset, ok := findReleaseAsset(release, "release-lock-"+release.TagName+".json")
	if !ok {
		return guard, errors.New("previous immutable release lock asset is missing")
	}
	lockBytes, err := observer.getBytes(ctx, lockAsset.BrowserURL)
	if err != nil {
		return guard, err
	}
	var lock releaseLock
	if err := json.Unmarshal(lockBytes, &lock); err != nil {
		return guard, fmt.Errorf("parse previous release lock: %w", err)
	}
	addGuardCheck(&guard, "release-lock-integrity", lock.Immutable && lock.Repository == repo && lock.Tag.Name == release.TagName, fmt.Sprintf("repository=%s tag=%s immutable=%t", lock.Repository, lock.Tag.Name, lock.Immutable))
	if !lock.Immutable || lock.Repository != repo || lock.Tag.Name != release.TagName {
		return guard, errors.New("previous release lock identity is invalid")
	}
	for _, expected := range lock.Assets {
		asset, exists := findReleaseAsset(release, expected.Name)
		matches := exists && asset.Size == expected.SizeBytes && asset.Digest == expected.SHA256
		addGuardCheck(&guard, "asset-digest:"+expected.Name, matches, fmt.Sprintf("size=%d digest=%s", expected.SizeBytes, expected.SHA256))
		if !matches {
			return guard, fmt.Errorf("previous asset %q does not match immutable lock", expected.Name)
		}
	}
	if lock.Tag.Annotated && lock.Tag.TagObjectSHA != "" && lock.Tag.TargetCommitSHA != "" {
		var ref gitRefMetadata
		if err := observer.get(ctx, fmt.Sprintf("/repos/%s/git/ref/tags/%s", repo, release.TagName), &ref); err != nil {
			return guard, err
		}
		var tag annotatedTagMetadata
		if err := observer.get(ctx, fmt.Sprintf("/repos/%s/git/tags/%s", repo, ref.Object.SHA), &tag); err != nil {
			return guard, err
		}
		matches := ref.Object.Type == "tag" && ref.Object.SHA == lock.Tag.TagObjectSHA && tag.Object.SHA == lock.Tag.TargetCommitSHA
		addGuardCheck(&guard, "annotated-tag-lineage", matches, fmt.Sprintf("tag-object=%s target=%s", ref.Object.SHA, tag.Object.SHA))
		if !matches {
			return guard, errors.New("previous annotated tag lineage does not match release lock")
		}
	} else {
		return guard, errors.New("previous release lock does not prove annotated tag lineage")
	}
	if lock.Actions.Conclusion != "success" || lock.Actions.RunID < 1 || lock.Actions.Evidence == "" {
		return guard, errors.New("previous release lock does not prove successful main evidence")
	}
	addGuardCheck(&guard, "previous-main-evidence", true, fmt.Sprintf("run=%d conclusion=%s artifact=%s", lock.Actions.RunID, lock.Actions.Conclusion, lock.Actions.Evidence))
	var run workflowRunMetadata
	if err := observer.get(ctx, fmt.Sprintf("/repos/%s/actions/runs/%d", repo, mainRunID), &run); err != nil {
		return guard, err
	}
	currentRunMatches := run.ID == mainRunID && run.HeadSHA == currentSHA && run.Conclusion == "success" && run.Event == "push"
	addGuardCheck(&guard, "current-main-evidence", currentRunMatches, fmt.Sprintf("run=%d sha=%s conclusion=%s event=%s", run.ID, run.HeadSHA, run.Conclusion, run.Event))
	if !currentRunMatches {
		return guard, errors.New("current main run is not a successful push for the requested SHA")
	}
	expectedNext, err := nextPatchTag(release.TagName)
	if err != nil {
		return guard, err
	}
	for containsString(burnedTags, expectedNext) {
		releaseStatus, releaseErr := observer.getStatus(ctx, fmt.Sprintf("/repos/%s/releases/tags/%s", repo, expectedNext))
		if releaseErr != nil {
			return guard, releaseErr
		}
		refStatus, refErr := observer.getStatus(ctx, fmt.Sprintf("/repos/%s/git/ref/tags/%s", repo, expectedNext))
		if refErr != nil {
			return guard, refErr
		}
		absent := releaseStatus == http.StatusNotFound && refStatus == http.StatusNotFound
		addGuardCheck(&guard, "burned-tag-absent:"+expectedNext, absent, fmt.Sprintf("release_status=%d ref_status=%d", releaseStatus, refStatus))
		if !absent {
			return guard, fmt.Errorf("burned tag %s is present and cannot be reused", expectedNext)
		}
		expectedNext, err = nextPatchTag(expectedNext)
		if err != nil {
			return guard, err
		}
	}
	addGuardCheck(&guard, "next-patch-lineage", nextTag == expectedNext, fmt.Sprintf("previous=%s next=%s", release.TagName, nextTag))
	if nextTag != expectedNext {
		return guard, errors.New("next release must be the next available patch tag")
	}
	if status, err := observer.getStatus(ctx, fmt.Sprintf("/repos/%s/releases/tags/%s", repo, nextTag)); err != nil {
		return guard, err
	} else if status != http.StatusNotFound {
		return guard, fmt.Errorf("next release tag %s already exists", nextTag)
	}
	if status, err := observer.getStatus(ctx, fmt.Sprintf("/repos/%s/git/ref/tags/%s", repo, nextTag)); err != nil {
		return guard, err
	} else if status != http.StatusNotFound {
		return guard, fmt.Errorf("next git tag %s already exists", nextTag)
	}
	addGuardCheck(&guard, "next-tag-available", true, nextTag+" is absent")
	guard.Status = StatusClosed
	return guard, nil
}

func addGuardCheck(guard *ReleaseGuardReport, id string, closed bool, detail string) {
	status := "CLOSED"
	if !closed {
		status = "REFUTED"
	}
	guard.Checks = append(guard.Checks, GuardCheck{ID: id, Status: status, Detail: detail})
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func findReleaseAsset(release releaseMetadata, name string) (struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Digest    string `json:"digest"`
	BrowserURL string `json:"browser_download_url"`
}, bool) {
	for _, asset := range release.Assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return struct {
		Name      string `json:"name"`
		Size      int64  `json:"size"`
		Digest    string `json:"digest"`
		BrowserURL string `json:"browser_download_url"`
	}{}, false
}

func nextPatchTag(tag string) (string, error) {
	parts := strings.Split(strings.TrimPrefix(tag, "v"), ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("tag %q is not a semantic three-part version", tag)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", err
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", err
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("v%d.%d.%d", major, minor, patch+1), nil
}

func (o githubObserver) getBytes(ctx context.Context, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("Authorization", "Bearer "+o.token)
	response, err := o.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub GET %s returned %s", url, response.Status)
	}
	var data []byte
	data, err = io.ReadAll(response.Body)
	return data, err
}

func (o githubObserver) getStatus(ctx context.Context, path string) (int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(o.baseURL, "/")+path, nil)
	if err != nil {
		return 0, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("Authorization", "Bearer "+o.token)
	response, err := o.client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	return response.StatusCode, nil
}
