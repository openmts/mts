import { apiPost } from './client'

export type DatabasePermission = 'read' | 'write' | 'admin'

export interface AuthzCheckResult {
  allowed: boolean
}

/** 检查当前用户（或 admin 指定用户）对 database 的权限 */
export async function checkDatabasePermission(input: {
  database: string
  permission: DatabasePermission
  user_name?: string
}): Promise<boolean> {
  const data = await apiPost<AuthzCheckResult>('/api/v1/authz/database/check', {
    database: input.database,
    permission: input.permission,
    user_name: input.user_name,
  })
  return !!data.allowed
}
