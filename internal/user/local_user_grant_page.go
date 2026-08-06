package user

import (
	"context"
	"fmt"
	"sort"
)

func (m *Manager) ListUserGrantPage(
	ctx context.Context,
	cursor string,
	limit int,
) (UserGrantPage, error) {
	if err := ctx.Err(); err != nil {
		return UserGrantPage{}, err
	}
	if limit < 1 || limit > MaxGrantPageLimit {
		return UserGrantPage{}, fmt.Errorf("%w: %d", ErrInvalidPageLimit, limit)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return UserGrantPage{}, err
	}

	names := sortedUserNames(m.store.users)
	start := sort.Search(len(names), func(index int) bool {
		return names[index] > cursor
	})
	end := min(start+limit, len(names))
	items := make([]UserGrantBundle, end-start)
	for index, name := range names[start:end] {
		items[index] = UserGrantBundle{
			User:   cloneUser(m.store.users[name]),
			Grants: sortedGrants(m.store.grants[name]),
		}
	}

	page := UserGrantPage{Items: items, TotalUsers: len(names)}
	if end < len(names) {
		page.NextCursor = names[end-1]
	}
	return page, nil
}
