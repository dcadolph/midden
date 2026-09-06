package mobile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// defaultBranch is the branch synced when the caller names none.
const defaultBranch = "main"

// remoteName is the only remote midden manages.
const remoteName = "origin"

// Sync commits this device's captures, reconciles with the remote, and pushes,
// returning a short human-readable summary of what happened.
//
// remoteURL is an https git URL and token is a personal access token with write
// access to it; both are supplied per call so the caller can keep the token in
// the platform keychain rather than in the vault. An empty branch means main.
//
// Reconciliation never merges file contents. A device writes only inside its
// own inbox, and the desktop only ever deletes from that inbox once entries are
// folded, so the two sides touch disjoint paths. When history has diverged this
// takes the remote wholesale and replays this device's inbox on top, which is
// safe because those files are the one thing the remote cannot have changed.
func (m *Vault) Sync(remoteURL, token, branch string) (string, error) {
	if strings.TrimSpace(remoteURL) == "" {
		return "", errors.New("remote URL is empty")
	}
	if branch = strings.TrimSpace(branch); branch == "" {
		branch = defaultBranch
	}
	repo, err := m.openRepo(remoteURL, branch)
	if err != nil {
		return "", err
	}
	auth := authFor(token)
	var log []string

	committed, err := m.commitLocal(repo, "sync from "+m.deviceName())
	if err != nil {
		return "", err
	}
	if committed {
		log = append(log, "committed local captures")
	}

	fetched, err := fetch(repo, auth)
	if err != nil {
		return "", err
	}
	if fetched {
		log = append(log, "fetched remote")
	}

	action, err := m.reconcile(repo, branch)
	if err != nil {
		return "", err
	}
	if action != "" {
		log = append(log, action)
	}

	pushed, err := push(repo, auth)
	if err != nil {
		return "", err
	}
	if pushed {
		log = append(log, "pushed")
	}
	if len(log) == 0 {
		return "already up to date", nil
	}
	return strings.Join(log, ", "), nil
}

// openRepo opens the vault as a git repository, initializing it and setting the
// remote on first use so a fresh device needs no manual git setup.
func (m *Vault) openRepo(remoteURL, branch string) (*git.Repository, error) {
	repo, err := git.PlainOpen(m.v.Dir)
	if errors.Is(err, git.ErrRepositoryNotExists) {
		repo, err = git.PlainInit(m.v.Dir, false)
		if err != nil {
			return nil, fmt.Errorf("git init: %w", err)
		}
		// A fresh repository points HEAD at the library default, which is not
		// necessarily the branch being synced.
		head := plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName(branch))
		if err := repo.Storer.SetReference(head); err != nil {
			return nil, fmt.Errorf("set head branch: %w", err)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	if err := ensureRemote(repo, remoteURL); err != nil {
		return nil, err
	}
	return repo, nil
}

// ensureRemote points origin at remoteURL, replacing a stale URL.
func ensureRemote(repo *git.Repository, remoteURL string) error {
	existing, err := repo.Remote(remoteName)
	switch {
	case errors.Is(err, git.ErrRemoteNotFound):
	case err != nil:
		return fmt.Errorf("read remote: %w", err)
	default:
		if len(existing.Config().URLs) > 0 && existing.Config().URLs[0] == remoteURL {
			return nil
		}
		if err := repo.DeleteRemote(remoteName); err != nil {
			return fmt.Errorf("replace remote: %w", err)
		}
	}
	if _, err := repo.CreateRemote(&config.RemoteConfig{Name: remoteName, URLs: []string{remoteURL}}); err != nil {
		return fmt.Errorf("create remote: %w", err)
	}
	return nil
}

// authFor builds the credentials used for fetch and push.
// An empty token yields nil so a public or local remote still works.
func authFor(token string) *http.BasicAuth {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	// Git over HTTPS ignores the username when the password is a token, but it
	// must be non-empty for basic auth to be sent at all.
	return &http.BasicAuth{Username: "midden", Password: token}
}

// commitLocal stages and commits the working tree, reporting whether anything
// was committed.
func (m *Vault) commitLocal(repo *git.Repository, message string) (bool, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("worktree: %w", err)
	}
	status, err := wt.Status()
	if err != nil {
		return false, fmt.Errorf("status: %w", err)
	}
	if status.IsClean() {
		return false, nil
	}
	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return false, fmt.Errorf("stage changes: %w", err)
	}
	if _, err := wt.Commit(message+" "+time.Now().UTC().Format(time.RFC3339), &git.CommitOptions{
		Author: &object.Signature{Name: "midden", Email: "midden@localhost", When: time.Now()},
	}); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}
	return true, nil
}

// fetch updates the remote-tracking refs, reporting whether anything new arrived.
func fetch(repo *git.Repository, auth *http.BasicAuth) (bool, error) {
	err := repo.Fetch(&git.FetchOptions{RemoteName: remoteName, Auth: auth, Force: true})
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, git.NoErrAlreadyUpToDate):
		return false, nil
	case errors.Is(err, transport.ErrEmptyRemoteRepository):
		return false, nil
	default:
		return false, fmt.Errorf("fetch: %w", err)
	}
}

// reconcile brings the local branch in line with the remote, returning a short
// description of what it did.
func (m *Vault) reconcile(repo *git.Repository, branch string) (string, error) {
	remoteRef, err := repo.Reference(plumbing.NewRemoteReferenceName(remoteName, branch), true)
	if err != nil {
		// No remote branch yet: this device's history is the starting point.
		return "", nil
	}
	head, err := repo.Head()
	if err != nil {
		// No local commits yet: adopt the remote wholesale.
		return "adopted remote history", m.checkoutRemote(repo, branch, remoteRef.Hash())
	}
	if head.Hash() == remoteRef.Hash() {
		return "", nil
	}
	ahead, err := isAncestor(repo, remoteRef.Hash(), head.Hash())
	if err != nil {
		return "", err
	}
	if ahead {
		// Local already contains the remote; the push carries it forward.
		return "", nil
	}
	behind, err := isAncestor(repo, head.Hash(), remoteRef.Hash())
	if err != nil {
		return "", err
	}
	if behind {
		return "fast-forwarded", m.checkoutRemote(repo, branch, remoteRef.Hash())
	}
	return "replayed local captures onto remote", m.replayOntoRemote(repo, branch, remoteRef.Hash())
}

// isAncestor reports whether the commit at ancestor is reachable from descendant.
func isAncestor(repo *git.Repository, ancestor, descendant plumbing.Hash) (bool, error) {
	a, err := repo.CommitObject(ancestor)
	if err != nil {
		return false, fmt.Errorf("read commit %s: %w", ancestor.String(), err)
	}
	d, err := repo.CommitObject(descendant)
	if err != nil {
		return false, fmt.Errorf("read commit %s: %w", descendant.String(), err)
	}
	ok, err := a.IsAncestor(d)
	if err != nil {
		return false, fmt.Errorf("compare history: %w", err)
	}
	return ok, nil
}

// checkoutRemote moves the branch and the working tree to the given commit,
// discarding local history. Callers use it only when local holds nothing the
// remote lacks.
//
// The branch reference is moved rather than the commit merely checked out,
// because a detached HEAD would leave nothing for the push to advance.
func (m *Vault) checkoutRemote(repo *git.Repository, branch string, target plumbing.Hash) error {
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}
	name := plumbing.NewBranchReferenceName(branch)
	if err := repo.Storer.SetReference(plumbing.NewHashReference(name, target)); err != nil {
		return fmt.Errorf("move branch: %w", err)
	}
	if err := wt.Checkout(&git.CheckoutOptions{Branch: name, Force: true}); err != nil {
		return fmt.Errorf("checkout remote: %w", err)
	}
	return nil
}

// replayOntoRemote resolves diverged history by taking the remote as the new
// base and re-adding this device's inbox files on top.
//
// This is not a general merge and does not try to be. It is correct only
// because the inbox belongs to one device: the remote cannot have edited these
// files, so re-adding them cannot clobber anyone. Files the remote no longer
// has are left out rather than restored, since their absence means the desktop
// folded them into the day files already.
func (m *Vault) replayOntoRemote(repo *git.Repository, branch string, target plumbing.Hash) error {
	if m.inbox == nil {
		return m.checkoutRemote(repo, branch, target)
	}
	snapshot, err := snapshotDir(m.inbox.Dir)
	if err != nil {
		return err
	}
	keep, err := m.replayable(repo, target, snapshot)
	if err != nil {
		return err
	}
	if err := m.checkoutRemote(repo, branch, target); err != nil {
		return err
	}
	restored := 0
	for _, rel := range keep {
		path := filepath.Join(m.inbox.Dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return fmt.Errorf("restore inbox directory: %w", err)
		}
		if err := os.WriteFile(path, snapshot[rel], 0o600); err != nil {
			return fmt.Errorf("restore %s: %w", rel, err)
		}
		restored++
	}
	if restored == 0 {
		return nil
	}
	if _, err := m.commitLocal(repo, "replay captures from "+m.deviceName()); err != nil {
		return err
	}
	return nil
}

// replayable returns the snapshot paths that should survive the reset onto the
// remote: the captures this device has that the remote has never seen.
//
// A file the remote does not have is only new if it is also absent from the
// point the two histories last agreed. If it was present there and is gone now,
// the desktop folded it into the day files and deleted it, so restoring it
// would bring back entries that have already landed.
func (m *Vault) replayable(repo *git.Repository, target plumbing.Hash, snapshot map[string][]byte) ([]string, error) {
	remoteCommit, err := repo.CommitObject(target)
	if err != nil {
		return nil, fmt.Errorf("read remote commit: %w", err)
	}
	remoteTree, err := remoteCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("read remote tree: %w", err)
	}
	baseTree, err := mergeBaseTree(repo, remoteCommit)
	if err != nil {
		return nil, err
	}
	prefix, err := filepath.Rel(m.v.Dir, m.inbox.Dir)
	if err != nil {
		return nil, fmt.Errorf("locate inbox: %w", err)
	}
	var keep []string
	for rel := range snapshot {
		tracked := filepath.ToSlash(filepath.Join(prefix, rel))
		if _, err := remoteTree.File(tracked); err == nil {
			// The remote carries its own copy; leave that one in place.
			continue
		}
		if baseTree != nil {
			if _, err := baseTree.File(tracked); err == nil {
				// Present when the histories last agreed and absent now, so it
				// was folded away deliberately.
				continue
			}
		}
		keep = append(keep, rel)
	}
	sort.Strings(keep)
	return keep, nil
}

// mergeBaseTree returns the tree of the commit where local history and the
// given remote commit last agreed, or nil when they share no ancestor.
func mergeBaseTree(repo *git.Repository, remoteCommit *object.Commit) (*object.Tree, error) {
	head, err := repo.Head()
	if err != nil {
		return nil, nil
	}
	localCommit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("read local commit: %w", err)
	}
	bases, err := localCommit.MergeBase(remoteCommit)
	if err != nil {
		return nil, fmt.Errorf("find merge base: %w", err)
	}
	if len(bases) == 0 {
		return nil, nil
	}
	tree, err := bases[0].Tree()
	if err != nil {
		return nil, fmt.Errorf("read merge base tree: %w", err)
	}
	return tree, nil
}

// snapshotDir reads every regular file under dir, keyed by path relative to dir.
// A missing directory yields an empty snapshot rather than an error.
func snapshotDir(dir string) (map[string][]byte, error) {
	out := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		data, err := os.ReadFile(path) //nolint:gosec // Path comes from the inbox tree.
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out[rel] = data
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return map[string][]byte{}, nil
		}
		return nil, fmt.Errorf("snapshot inbox: %w", err)
	}
	return out, nil
}

// push sends local commits to the remote, reporting whether anything moved.
func push(repo *git.Repository, auth *http.BasicAuth) (bool, error) {
	err := repo.Push(&git.PushOptions{RemoteName: remoteName, Auth: auth})
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, git.NoErrAlreadyUpToDate):
		return false, nil
	default:
		return false, fmt.Errorf("push: %w", err)
	}
}

// deviceName returns this device's inbox name, or "desktop" when the handle
// writes canonical files directly.
func (m *Vault) deviceName() string {
	if m.inbox == nil {
		return "desktop"
	}
	return filepath.Base(m.inbox.Dir)
}
