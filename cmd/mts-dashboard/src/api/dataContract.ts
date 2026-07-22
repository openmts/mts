import { apiGet } from '@/api/client'
import type { DataContractResponse } from '@/api/types'
import { parseAdminOpStatusPayload } from '@/utils/adminOpBusy'

export async function fetchDataContract(): Promise<{
  contract: DataContractResponse
  adminOp?: ReturnType<typeof parseAdminOpStatusPayload>
}> {
  const data = await apiGet<DataContractResponse>('/api/v1/data/contract')
  return {
    contract: data,
    adminOp: parseAdminOpStatusPayload({
      admin_op_busy: data.admin_op_busy,
      op: data.op,
      started_at_unix: data.started_at_unix,
      last: data.last,
    }),
  }
}
