package cli

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/AndrewMast/rig/internal/selfupdate"
	"github.com/spf13/cobra"
)

const ghOwner = "AndrewMast"
const ghRepo = "rig"

// newSelfCmd groups rig's self-management commands.
func newSelfCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "self",
		Short: "Manage the rig binary itself (update, uninstall, version)",
	}
	cmd.AddCommand(newSelfVersionCmd(), newSelfUpdateCmd(), newSelfUninstallCmd())
	return cmd
}

func newSelfVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the rig version",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			c.Printf("rig %s\n", version)
			return nil
		},
	}
}

func newSelfUpdateCmd() *cobra.Command {
	var check, without, require bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update rig to the latest release (verified)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app := appFrom(cmd)
			latest, err := latestTag(context.Background())
			if err != nil {
				return err
			}
			if !selfupdate.NeedsUpdate(version, latest) {
				cmd.Printf("rig is up to date (%s)\n", version)
				return nil
			}
			cmd.Printf("update available: %s -> %s\n", version, latest)
			if check {
				return nil
			}
			return app.runSelfUpdate(cmd, latest, without, require)
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "report whether an update is available, without installing")
	cmd.Flags().BoolVar(&without, "without-attestation", false, "skip attestation and its prompt (checksum only)")
	cmd.Flags().BoolVar(&require, "require-attestation", false, "hard-fail without verified provenance")
	return cmd
}

func newSelfUninstallCmd() *cobra.Command {
	var purge bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the rig binary (and, with --purge, its config)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app := appFrom(cmd)
			exe, err := os.Executable()
			if err != nil {
				return err
			}
			ok, err := app.UI.Confirm(fmt.Sprintf("Remove %s?", exe), false)
			if err != nil || !ok {
				return err
			}
			if err := os.Remove(exe); err != nil {
				return fmt.Errorf("remove binary: %w", err)
			}
			cmd.Printf("removed %s\n", exe)
			if purge {
				if err := os.RemoveAll(app.Paths.ConfigDir); err != nil {
					return fmt.Errorf("purge config: %w", err)
				}
				cmd.Printf("purged %s\n", app.Paths.ConfigDir)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&purge, "purge", false, "also remove the rig config directory")
	return cmd
}

// latestTag resolves the latest release tag via the GitHub API.
func latestTag(ctx context.Context) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", ghOwner, ghRepo)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("query latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("latest release: HTTP %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	// Avoid a JSON dependency for one field.
	const key = `"tag_name":`
	s := string(body)
	i := strings.Index(s, key)
	if i < 0 {
		return "", fmt.Errorf("no tag_name in release response")
	}
	rest := s[i+len(key):]
	start := strings.Index(rest, `"`)
	rest = rest[start+1:]
	end := strings.Index(rest, `"`)
	return rest[:end], nil
}

// runSelfUpdate downloads, verifies (checksum + attestation ladder), and
// atomically swaps the running binary.
func (a *App) runSelfUpdate(cmd *cobra.Command, version string, without, require bool) error {
	ctx := context.Background()
	asset := selfupdate.AssetName(version, runtime.GOOS, runtime.GOARCH)
	base := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s", ghOwner, ghRepo, version)

	tmp, err := os.MkdirTemp("", "rig-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	archivePath := filepath.Join(tmp, asset)
	if err := download(ctx, base+"/"+asset, archivePath); err != nil {
		return fmt.Errorf("download %s: %w", asset, err)
	}
	checksPath := filepath.Join(tmp, "checksums.txt")
	if err := download(ctx, base+"/checksums.txt", checksPath); err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}

	// Ladder step 1: checksum (always).
	data, err := os.ReadFile(checksPath)
	if err != nil {
		return err
	}
	if err := selfupdate.VerifyChecksum(archivePath, asset, selfupdate.ParseChecksums(string(data))); err != nil {
		return err
	}
	cmd.Println("checksum verified")

	// Ladder steps 2-5: attestation.
	if err := a.verifyAttestation(cmd, tmp, base, archivePath, without, require); err != nil {
		return err
	}

	// Extract and swap.
	newBin := filepath.Join(tmp, "rig")
	if err := extractBinary(archivePath, "rig", newBin); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := atomicSwap(newBin, exe); err != nil {
		return err
	}
	cmd.Printf("updated rig to %s\n", version)
	return nil
}

func (a *App) verifyAttestation(cmd *cobra.Command, tmp, base, archivePath string, without, require bool) error {
	_, ghErr := exec.LookPath("gh")
	if ghErr == nil {
		bundle := filepath.Join(tmp, "rig.attestation.jsonl")
		if err := download(context.Background(), base+"/rig.attestation.jsonl", bundle); err == nil {
			c := exec.Command("gh", "attestation", "verify", archivePath, "--bundle", bundle, "--owner", ghOwner)
			if c.Run() == nil {
				cmd.Println("attestation verified")
				return nil
			}
		}
		if require {
			return fmt.Errorf("attestation verification failed and --require-attestation is set")
		}
		cmd.Println("attestation could not be verified; continuing on checksum only")
		return nil
	}
	if require {
		return fmt.Errorf("--require-attestation set but gh is not installed")
	}
	if without {
		cmd.Println("skipping attestation (--without-attestation)")
		return nil
	}
	ok, err := a.UI.Confirm("gh not found; install with checksum verification only?", false)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("aborted (no attestation)")
	}
	return nil
}

func download(ctx context.Context, url, dest string) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// extractBinary pulls a single named file out of a .tar.gz into dest.
func extractBinary(archive, name, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("%s not found in archive", name)
		}
		if err != nil {
			return err
		}
		if filepath.Base(hdr.Name) == name {
			out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				return err
			}
			defer out.Close()
			_, err = io.Copy(out, tr)
			return err
		}
	}
}

// atomicSwap replaces the destination executable with newBin via a same-dir
// rename (atomic on the same filesystem).
func atomicSwap(newBin, dest string) error {
	staged := dest + ".new"
	in, err := os.Open(newBin)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(staged, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := os.Rename(staged, dest); err != nil {
		return fmt.Errorf("swap binary: %w", err)
	}
	return nil
}
