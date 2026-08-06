package mts

import "github.com/openmts/mts/internal/runtime"

func runtimePermission(permission DatabasePermission) runtime.DatabasePermission {
	return runtime.DatabasePermission(permission)
}

func fromRuntimeGrant(grant runtime.DatabaseGrant) DatabaseGrant {
	return DatabaseGrant{
		Database:   grant.Database,
		Permission: DatabasePermission(grant.Permission),
	}
}

func toRuntimeCredentials(credentials Credentials) runtime.Credentials {
	return runtime.Credentials{
		UserName: credentials.UserName,
		Password: credentials.Password,
	}
}

func toRuntimeUser(user User) runtime.User {
	return runtime.User{
		Name:        user.Name,
		DisplayName: user.DisplayName,
		Role:        runtime.UserRole(user.Role),
		Disabled:    user.Disabled,
		Metadata:    user.Metadata,
	}
}

func fromRuntimeUser(user runtime.User) User {
	return User{
		Name:        user.Name,
		DisplayName: user.DisplayName,
		Role:        UserRole(user.Role),
		Disabled:    user.Disabled,
		Metadata:    user.Metadata,
	}
}

func fromRuntimeUserGrantPage(page runtime.UserGrantPage) UserGrantPage {
	items := make([]UserGrantBundle, len(page.Items))
	for index, item := range page.Items {
		grants := make([]DatabaseGrant, len(item.Grants))
		for grantIndex, grant := range item.Grants {
			grants[grantIndex] = fromRuntimeGrant(grant)
		}
		items[index] = UserGrantBundle{
			User:   fromRuntimeUser(item.User),
			Grants: grants,
		}
	}
	return UserGrantPage{
		Items:      items,
		TotalUsers: page.TotalUsers,
		NextCursor: page.NextCursor,
	}
}
