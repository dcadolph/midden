package cmd

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/flock"
	"github.com/dcadolph/midden/internal/jsonutil"
	"github.com/dcadolph/midden/internal/vault"
)

// inboxCmd groups the per-device inbox subcommands.
var inboxCmd = &cobra.Command{
	Use:   "inbox",
	Short: "Inspect and fold in entries captured on other devices.",
	Long: "Inspect and fold in entries captured on other devices.\n\n" +
		"Devices that do not own the canonical day files, such as the phone, append\n" +
		"to their own inbox tree so two devices never write the same file. Folding\n" +
		"merges those entries into the day files and empties the inbox.",
}

// inboxListCmd reports what is waiting in each device inbox.
var inboxListCmd = &cobra.Command{
	Use:   "list",
	Short: "List entries waiting in each device inbox.",
	RunE:  runInboxList,
}

// inboxFoldCmd merges every device inbox into the canonical day files.
var inboxFoldCmd = &cobra.Command{
	Use:   "fold",
	Short: "Merge every device inbox into the day files and empty the inboxes.",
	RunE:  runInboxFold,
}

func init() {
	inboxCmd.AddCommand(inboxListCmd)
	inboxCmd.AddCommand(inboxFoldCmd)
	rootCmd.AddCommand(inboxCmd)
}

// deviceDay pairs a device inbox with one date it holds entries for.
type deviceDay struct {
	// Device is the inbox device name.
	Device string
	// Day is the date the entries fall on.
	Day time.Time
	// Count is the number of entries the inbox holds for that date.
	Count int
}

// runInboxList prints the pending entry count per device and date.
func runInboxList(cmd *cobra.Command, _ []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	pending, err := pendingInbox(v)
	if err != nil {
		return err
	}
	if jsonOutput {
		return jsonutil.Encode(cmd.OutOrStdout(), pending, jsonPretty)
	}
	if len(pending) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No entries waiting.")
		return nil
	}
	total := 0
	for _, p := range pending {
		fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  %d entry(s)\n", p.Device, p.Day.Format(layoutDate), p.Count)
		total += p.Count
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%d entry(s) waiting across %d device inbox(es).\n", total, countDevices(pending))
	return nil
}

// runInboxFold merges every device inbox into the canonical day files.
func runInboxFold(cmd *cobra.Command, _ []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	devices, err := v.InboxDevices()
	if err != nil {
		return errors.Join(ErrVault, err)
	}
	if len(devices) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No entries waiting.")
		return nil
	}
	folded := 0
	for _, device := range devices {
		n, err := foldDevice(v, device)
		folded += n
		if err != nil {
			return err
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Folded %d entry(s) from %d device inbox(es).\n", folded, len(devices))
	return nil
}

// foldDevice moves one device inbox into the canonical day files and returns
// how many entries it folded.
//
// The inbox holds its own lock for the whole read-append-drain cycle, so a
// device appending mid-fold cannot have an entry deleted before it is read.
// That lock is separate from the canonical vault lock AppendAll takes, so the
// two never wait on each other. Entries are written before the inbox file is
// removed, since a crash that leaves an unfolded inbox costs a rerun while the
// opposite order would lose the entries outright.
func foldDevice(v *vault.Vault, device string) (int, error) {
	box, err := v.Inbox(device)
	if err != nil {
		return 0, errors.Join(ErrVault, err)
	}
	lock, err := flock.Acquire(box.LockPath())
	if err != nil {
		return 0, errors.Join(ErrVault, fmt.Errorf("acquire %s inbox lock: %w", device, err))
	}
	defer func() { _ = lock.Close() }()
	days, err := box.ListDays()
	if err != nil {
		return 0, errors.Join(ErrVault, fmt.Errorf("list %s inbox: %w", device, err))
	}
	folded := 0
	for _, day := range days {
		entries, err := box.ReadDay(day)
		if err != nil {
			return folded, errors.Join(ErrVault, fmt.Errorf("read %s inbox %s: %w", device, day.Format(layoutDate), err))
		}
		existing, err := v.ReadDay(day)
		if err != nil {
			return folded, errors.Join(ErrVault, fmt.Errorf("read day %s: %w", day.Format(layoutDate), err))
		}
		entries = newEntries(existing, entries)
		if len(entries) > 0 {
			if err := v.AppendAll(entries); err != nil {
				return folded, errors.Join(ErrVault, fmt.Errorf("fold %s %s: %w", device, day.Format(layoutDate), err))
			}
		}
		if err := box.DrainDay(day); err != nil {
			return folded, errors.Join(ErrVault, fmt.Errorf(
				"folded %s %s but could not clear the inbox, so rerunning would duplicate it: %w",
				device, day.Format(layoutDate), err))
		}
		folded += len(entries)
	}
	// A drained inbox leaves an empty directory behind. Devices that sync in
	// batches create a fresh one each time, so leaving them would grow the
	// vault without bound.
	if err := box.RemoveIfEmpty(); err != nil {
		return folded, errors.Join(ErrVault, fmt.Errorf("prune %s inbox: %w", device, err))
	}
	return folded, nil
}

// newEntries returns the candidates that the day does not already hold.
//
// Folding is content-idempotent so that an inbox file which reappears, whether
// from a restored backup or a sync that resurrected an already-folded file,
// cannot duplicate entries that were folded earlier. Entries match on
// timestamp, tags, and body, which is everything a day file records.
func newEntries(existing, candidates []vault.Entry) []vault.Entry {
	if len(existing) == 0 {
		return candidates
	}
	have := make(map[string]struct{}, len(existing))
	for _, e := range existing {
		have[entryKey(e)] = struct{}{}
	}
	out := make([]vault.Entry, 0, len(candidates))
	for _, c := range candidates {
		key := entryKey(c)
		if _, seen := have[key]; seen {
			continue
		}
		have[key] = struct{}{}
		out = append(out, c)
	}
	return out
}

// entryKey returns a comparison key covering everything a day file stores.
func entryKey(e vault.Entry) string {
	return e.Time.Format(time.RFC3339) + "\x00" + strings.Join(e.Tags, ",") + "\x00" + strings.TrimSpace(e.Body)
}

// pendingInbox returns what each device inbox holds, ordered by device then date.
func pendingInbox(v *vault.Vault) ([]deviceDay, error) {
	devices, err := v.InboxDevices()
	if err != nil {
		return nil, errors.Join(ErrVault, err)
	}
	var out []deviceDay
	for _, device := range devices {
		box, err := v.Inbox(device)
		if err != nil {
			return nil, errors.Join(ErrVault, err)
		}
		days, err := box.ListDays()
		if err != nil {
			return nil, errors.Join(ErrVault, fmt.Errorf("list %s inbox: %w", device, err))
		}
		for _, day := range days {
			entries, err := box.ReadDay(day)
			if err != nil {
				return nil, errors.Join(ErrVault, fmt.Errorf("read %s inbox: %w", device, err))
			}
			if len(entries) == 0 {
				continue
			}
			out = append(out, deviceDay{Device: device, Day: day, Count: len(entries)})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Device != out[j].Device {
			return out[i].Device < out[j].Device
		}
		return out[i].Day.Before(out[j].Day)
	})
	return out, nil
}

// countDevices returns how many distinct devices appear in the pending list.
func countDevices(pending []deviceDay) int {
	seen := make(map[string]struct{}, len(pending))
	for _, p := range pending {
		seen[p.Device] = struct{}{}
	}
	return len(seen)
}
