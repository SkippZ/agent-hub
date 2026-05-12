package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"agent-hub/internal/types"
)

func ListProjects(dir string) ([]*types.Project, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read projects dir: %w", err)
	}

	var projects []*types.Project
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		gitDir := filepath.Join(dir, e.Name(), ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			projects = append(projects, &types.Project{
				Name: e.Name(),
				Path: filepath.Join(dir, e.Name()),
			})
		}
	}
	return projects, nil
}

func ListBranches(repoPath string) ([]*types.Branch, error) {
	cmd := exec.Command("git", "branch", "--format=%(refname:short)")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var branches []*types.Branch
	for _, line := range lines {
		if line != "" {
			branches = append(branches, &types.Branch{Name: line})
		}
	}
	return branches, nil
}

func BranchNameFromDescription(desc string) string {
	s := strings.ToLower(desc)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	name := strings.Trim(result.String(), "-")
	if len(name) > 80 {
		name = name[:80]
	}
	return "agent/" + name
}

func CreateWorktree(repoPath, baseBranch, featureBranch string) (string, error) {
	worktreeDir := filepath.Join(repoPath, "..", featureBranch)
	absWorktree, err := filepath.Abs(worktreeDir)
	if err != nil {
		return "", fmt.Errorf("worktree abs path: %w", err)
	}

	cmd := exec.Command("git", "worktree", "add", "-b", featureBranch, absWorktree, baseBranch)
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("create worktree: %s: %w", string(out), err)
	}
	return absWorktree, nil
}

func RemoveWorktree(repoPath, worktreePath string) error {
	cmd := exec.Command("git", "worktree", "remove", worktreePath)
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("remove worktree: %s: %w", string(out), err)
	}
	return nil
}

func GetDiff(repoPath, baseRef string) (string, error) {
	cmd := exec.Command("git", "diff", baseRef+"...")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get diff: %w", err)
	}
	return string(out), nil
}

func GetRecentCommits(repoPath, baseRef string, limit int) (string, error) {
	cmd := exec.Command("git", "log", "--oneline", "-n", fmt.Sprintf("%d", limit), baseRef+"..")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get recent commits: %w", err)
	}
	return string(out), nil
}

func GetCommitDiff(repoPath, commitHash string) (string, error) {
	cmd := exec.Command("git", "show", commitHash, "--stat", "--format=format:%h %s%n")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get commit diff: %w", err)
	}
	return string(out), nil
}
