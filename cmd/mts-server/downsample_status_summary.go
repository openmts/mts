package main

import mts "github.com/openmts/mts"

func summarizeDownsampleStatuses(statuses []mts.DownsamplePolicyStatus) downsampleStatusSummary {
	var out downsampleStatusSummary
	out.Total = len(statuses)
	for _, st := range statuses {
		if st.Enabled {
			out.Enabled++
		}
		if st.Active {
			out.Active++
		}
		if st.LastError != "" {
			out.Error++
		}
		if st.LagSeconds > 0 {
			out.Lagging++
			if st.LagSeconds > out.MaxLagSeconds {
				out.MaxLagSeconds = st.LagSeconds
			}
		}
	}
	return out
}
