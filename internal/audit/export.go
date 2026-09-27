package audit

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"go.mau.fi/whatsmeow"
)

type Contact struct{ JID, Phone, FirstName, FullName, PushName, BusinessName string }
type Group struct {
	JID, Name, Topic string
	Participants     []string
}
type Snapshot struct {
	ExportedAt time.Time `json:"exported_at"`
	Contacts   []Contact `json:"contacts"`
	Groups     []Group   `json:"groups"`
}

func Collect(ctx context.Context, client *whatsmeow.Client) (Snapshot, error) {
	contacts, err := client.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		return Snapshot{}, fmt.Errorf("listar contatos sincronizados: %w", err)
	}
	groups, err := client.GetJoinedGroups(ctx)
	if err != nil {
		return Snapshot{}, fmt.Errorf("listar grupos: %w", err)
	}
	out := Snapshot{ExportedAt: time.Now().UTC()}
	for jid, c := range contacts {
		out.Contacts = append(out.Contacts, Contact{JID: jid.String(), Phone: jid.User, FirstName: c.FirstName, FullName: c.FullName, PushName: c.PushName, BusinessName: c.BusinessName})
	}
	for _, g := range groups {
		row := Group{JID: g.JID.String(), Name: g.Name, Topic: g.Topic}
		for _, p := range g.Participants {
			row.Participants = append(row.Participants, p.JID.String())
		}
		sort.Strings(row.Participants)
		out.Groups = append(out.Groups, row)
	}
	sort.Slice(out.Contacts, func(i, j int) bool { return out.Contacts[i].JID < out.Contacts[j].JID })
	sort.Slice(out.Groups, func(i, j int) bool { return out.Groups[i].JID < out.Groups[j].JID })
	return out, nil
}

func Write(snapshot Snapshot, dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("criar exportação: %w", err)
	}
	b, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "audit.json"), b, 0o600); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, "contacts.csv"))
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	_ = w.Write([]string{"jid", "phone", "first_name", "full_name", "push_name", "business_name"})
	for _, c := range snapshot.Contacts {
		_ = w.Write([]string{c.JID, c.Phone, c.FirstName, c.FullName, c.PushName, c.BusinessName})
	}
	w.Flush()
	closeErr := f.Close()
	if err := w.Error(); err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	g, err := os.Create(filepath.Join(dir, "group_members.csv"))
	if err != nil {
		return err
	}
	gw := csv.NewWriter(g)
	_ = gw.Write([]string{"group_jid", "group_name", "participant_jid"})
	for _, group := range snapshot.Groups {
		for _, p := range group.Participants {
			_ = gw.Write([]string{group.JID, group.Name, p})
		}
	}
	gw.Flush()
	closeErr = g.Close()
	if err := gw.Error(); err != nil {
		return err
	}
	return closeErr
}
