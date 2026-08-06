import type { UserGrantBundle } from './grantsSummary.ts'

export const ACCESS_GRANTS_PAGE_LIMIT = 100
const ACCESS_GRANTS_PAGE_MAX = 200
const ACCESS_GRANTS_PATH = '/api/v1/users/access-grants'

export interface AccessGrantItem {
  user: {
    name: string
    display_name?: string
    role?: string
    disabled?: boolean
    metadata?: Record<string, string>
  }
  grants: Array<{ database: string; permission: string }>
}

export interface AccessGrantsPageResponse {
  items: AccessGrantItem[]
  total_users: number
  next_cursor?: string
  path?: string
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: unknown
}

export interface AccessGrantsCursorNavigation {
  cursor: string
  history: string[]
}

export function buildAccessGrantsPagePath(
  cursor = '',
  limit = ACCESS_GRANTS_PAGE_LIMIT,
): string {
  if (!Number.isInteger(limit) || limit < 1 || limit > ACCESS_GRANTS_PAGE_MAX) {
    throw new Error('access grants page limit must be between 1 and 200')
  }
  const query = new URLSearchParams({ limit: String(limit) })
  if (cursor) query.set('cursor', cursor)
  return `${ACCESS_GRANTS_PATH}?${query.toString()}`
}

export function accessGrantItemsToBundles(items: AccessGrantItem[]): UserGrantBundle[] {
  return items.map((item) => ({
    user: item.user.name,
    role: item.user.role,
    disabled: item.user.disabled,
    grants: item.grants.map((grant) => ({ ...grant })),
  }))
}

export function advanceAccessGrantsCursor(
  navigation: AccessGrantsCursorNavigation,
  nextCursor: string,
): AccessGrantsCursorNavigation {
  if (!nextCursor) return navigation
  return {
    cursor: nextCursor,
    history: [...navigation.history, navigation.cursor],
  }
}

export function retreatAccessGrantsCursor(
  navigation: AccessGrantsCursorNavigation,
): AccessGrantsCursorNavigation {
  if (!navigation.history.length) return navigation
  return {
    cursor: navigation.history.at(-1) || '',
    history: navigation.history.slice(0, -1),
  }
}
