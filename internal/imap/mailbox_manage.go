package imap

import "fmt"

// Mailbox create, rename and delete, kept in a separate file from client.go so
// this fork's additions stay easy to merge with upstream.

// CreateMailbox creates a mailbox (folder) with the full path given.
func (c *Client) CreateMailbox(name string) error {
	if c.client == nil {
		return fmt.Errorf("not connected")
	}
	if err := c.client.Create(name, nil).Wait(); err != nil {
		return fmt.Errorf("failed to create %s: %w", name, err)
	}
	c.log.Info().Str("mailbox", name).Msg("Created mailbox")
	return nil
}

// RenameMailbox renames a mailbox. Servers rename its subfolders along with it.
func (c *Client) RenameMailbox(oldName, newName string) error {
	if c.client == nil {
		return fmt.Errorf("not connected")
	}
	if err := c.client.Rename(oldName, newName, nil).Wait(); err != nil {
		return fmt.Errorf("failed to rename %s: %w", oldName, err)
	}
	c.log.Info().Str("from", oldName).Str("to", newName).Msg("Renamed mailbox")
	return nil
}

// DeleteMailbox deletes a mailbox. Callers must check that it's empty first:
// some servers delete the messages in it too.
func (c *Client) DeleteMailbox(name string) error {
	if c.client == nil {
		return fmt.Errorf("not connected")
	}
	if err := c.client.Delete(name).Wait(); err != nil {
		return fmt.Errorf("failed to delete %s: %w", name, err)
	}
	c.log.Info().Str("mailbox", name).Msg("Deleted mailbox")
	return nil
}

// HierarchyDelimiter returns the character the server uses between folder
// levels (usually "/" or "."), or "" if the server reports none.
func (c *Client) HierarchyDelimiter() (string, error) {
	mailboxes, err := c.ListMailboxes()
	if err != nil {
		return "", err
	}
	for _, mb := range mailboxes {
		if mb.Delimiter != "" {
			return mb.Delimiter, nil
		}
	}
	return "", nil
}
