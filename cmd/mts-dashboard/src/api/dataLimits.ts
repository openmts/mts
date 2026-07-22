import { apiGet } from '@/api/client'
import type { DataLimitsResponse } from '@/api/types'
import { parseAdminOpStatusPayload } from '@/utils/adminOpBusy'

export async function fetchDataLimits(): Promise<{
  limits: DataLimitsResponse
  adminOp?: ReturnType<typeof parseAdminOpStatusPayload>
}> {
  const data = await apiGet<DataLimitsResponse>('/api/v1/data/limits')
  return {
    limits: data,
    adminOp: parseAdminOpStatusPayload({
      admin_op_busy: data.admin_op_busy,
      op: data.op,
      started_at_unix: data.started_at_unix,
      last: data.last,
    }),
  }
}
