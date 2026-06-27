package user

// RolePolicy 定义角色级管理策略。
type RolePolicy interface {
	CanManageUser(actor User, target User) bool
	CanSetPassword(actor User, target User) bool
	CanGrantDatabase(actor User, target User) bool
	CanChangeOwnPassword(actor User, targetName string) bool
}

// DefaultRolePolicy 是内置用户模块的默认角色策略。
type DefaultRolePolicy struct{}

func (DefaultRolePolicy) CanManageUser(actor User, target User) bool {
	return isAdmin(actor) && !isAdmin(target)
}

func (p DefaultRolePolicy) CanSetPassword(actor User, target User) bool {
	return p.CanManageUser(actor, target)
}

func (p DefaultRolePolicy) CanGrantDatabase(actor User, target User) bool {
	return p.CanManageUser(actor, target)
}

func (DefaultRolePolicy) CanChangeOwnPassword(actor User, targetName string) bool {
	return actor.Name != "" && actor.Name == targetName && !actor.Disabled
}

func isAdmin(user User) bool {
	return user.Role == RoleAdmin && !user.Disabled
}
