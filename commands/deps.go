package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/neutrino2211/gecko/config"
	"github.com/urfave/cli/v2"
)

var DepsCommand = &cli.Command{
	Name:    "deps",
	Aliases: []string{"d"},
	Usage:   "Manage project dependencies",
	Subcommands: []*cli.Command{
		{
			Name:  "fetch",
			Usage: "Fetch all dependencies defined in gecko.toml",
			Action: func(ctx *cli.Context) error {
				return fetchDependencies(ctx)
			},
		},
		{
			Name:  "update",
			Usage: "Update all dependencies to their latest specified versions",
			Action: func(ctx *cli.Context) error {
				return updateDependencies(ctx)
			},
		},
	},
}

func fetchDependencies(ctx *cli.Context) error {
	// Load project config
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	projectCfg, err := config.LoadProjectConfig(wd)
	if err != nil {
		return fmt.Errorf("failed to load gecko.toml: %w", err)
	}

	if len(projectCfg.Dependencies) == 0 {
		fmt.Println("No dependencies defined in gecko.toml")
		return nil
	}

	depsDir := projectCfg.GetDepsDir()

	// Create deps directory if it doesn't exist
	if err := os.MkdirAll(depsDir, 0755); err != nil {
		return fmt.Errorf("failed to create deps directory: %w", err)
	}

	for name, dep := range projectCfg.Dependencies {
		depPath := filepath.Join(depsDir, name)

		if dep.Path != "" {
			// Local path dependency - create symlink
			if err := handlePathDependency(name, dep, depPath, projectCfg.ProjectRoot); err != nil {
				fmt.Printf("Error handling path dependency '%s': %v\n", name, err)
				continue
			}
		} else if dep.Git != "" {
			// Git dependency - clone or update
			if err := handleGitDependency(name, dep, depPath); err != nil {
				fmt.Printf("Error fetching git dependency '%s': %v\n", name, err)
				continue
			}
		}
	}

	fmt.Println("Dependencies fetched successfully")
	return nil
}

func handlePathDependency(name string, dep *config.Dependency, depPath, projectRoot string) error {
	// Resolve path relative to project root
	sourcePath := dep.Path
	if !filepath.IsAbs(sourcePath) {
		sourcePath = filepath.Join(projectRoot, sourcePath)
	}

	// Check source exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("path '%s' does not exist", dep.Path)
	}

	// Remove existing symlink/directory
	if _, err := os.Lstat(depPath); err == nil {
		if err := os.RemoveAll(depPath); err != nil {
			return fmt.Errorf("failed to remove existing dependency: %w", err)
		}
	}

	// Create symlink
	absSource, _ := filepath.Abs(sourcePath)
	if err := os.Symlink(absSource, depPath); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	fmt.Printf("Linked %s -> %s\n", name, dep.Path)
	return nil
}

func handleGitDependency(name string, dep *config.Dependency, depPath string) error {
	// Check if already cloned
	if _, err := os.Stat(filepath.Join(depPath, ".git")); err == nil {
		// Already exists - update
		return updateGitDependency(name, dep, depPath)
	}

	// Clone the repository
	fmt.Printf("Cloning %s from %s...\n", name, dep.Git)

	args := []string{"clone"}

	// Add branch/tag if specified
	if dep.Tag != "" {
		args = append(args, "--branch", dep.Tag, "--depth", "1")
	} else if dep.Branch != "" {
		args = append(args, "--branch", dep.Branch)
	}

	args = append(args, dep.Git, depPath)

	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	// If specific commit requested, checkout
	if dep.Commit != "" {
		cmd := exec.Command("git", "-C", depPath, "checkout", dep.Commit)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git checkout commit failed: %w", err)
		}
	}

	fmt.Printf("Cloned %s\n", name)
	return nil
}

func updateGitDependency(name string, dep *config.Dependency, depPath string) error {
	fmt.Printf("Updating %s...\n", name)

	// Fetch latest
	cmd := exec.Command("git", "-C", depPath, "fetch", "--all")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git fetch failed: %w", err)
	}

	// Checkout specific ref
	var ref string
	if dep.Tag != "" {
		ref = dep.Tag
	} else if dep.Commit != "" {
		ref = dep.Commit
	} else if dep.Branch != "" {
		ref = "origin/" + dep.Branch
	} else {
		ref = "origin/main"
	}

	cmd = exec.Command("git", "-C", depPath, "checkout", ref)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Try origin/master as fallback
		cmd = exec.Command("git", "-C", depPath, "checkout", "origin/master")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git checkout failed: %w", err)
		}
	}

	fmt.Printf("Updated %s\n", name)
	return nil
}

func updateDependencies(ctx *cli.Context) error {
	// Same as fetch but forces update
	return fetchDependencies(ctx)
}
