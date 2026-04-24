package slack

import (
	"context"
	"fmt"
)

func (c *Client) SetStatus(ctx context.Context, text, emoji string) error {
	err := c.api.SetUserCustomStatusContextWithUser(ctx, "", text, emoji, 0)
	if err != nil {
		return fmt.Errorf("set status: %w", err)
	}
	return nil
}

func (c *Client) ClearStatus(ctx context.Context) error {
	err := c.api.SetUserCustomStatusContextWithUser(ctx, "", "", "", 0)
	if err != nil {
		return fmt.Errorf("clear status: %w", err)
	}
	return nil
}
