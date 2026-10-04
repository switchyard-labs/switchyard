package actions

import "context"

// Pages storage shares the authenticated Actions Worker protocol. Runner upload
// grants do not confer any of these control-plane capabilities.
func (c *Client) PublishPages(ctx context.Context, input map[string]any) (map[string]any, error) {
	out := map[string]any{}
	err := c.Request(ctx, "POST", "/pages/publish", input, &out)
	return out, err
}
func (c *Client) PagesMapping(ctx context.Context, owner string) (map[string]any, error) {
	out := map[string]any{}
	err := c.Request(ctx, "POST", "/pages/mapping", map[string]any{"owner": owner}, &out)
	return out, err
}
func (c *Client) PromotePages(ctx context.Context, input map[string]any) (map[string]any, error) {
	out := map[string]any{}
	err := c.Request(ctx, "POST", "/pages/promote", input, &out)
	return out, err
}
