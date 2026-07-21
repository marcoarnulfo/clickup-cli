package clickup

import "context"

// User è l'utente autenticato.
type User struct {
	ID       int
	Username string
}

// CurrentUser ritorna l'utente proprietario del token.
func (c *Client) CurrentUser(ctx context.Context) (User, error) {
	var resp struct {
		User struct {
			ID       int    `json:"id"`
			Username string `json:"username"`
		} `json:"user"`
	}
	if err := c.get(ctx, "/user", nil, &resp); err != nil {
		return User{}, err
	}
	return User{ID: resp.User.ID, Username: resp.User.Username}, nil
}
